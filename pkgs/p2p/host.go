package p2p

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/multiformats/go-multiaddr"
	"github.com/powerloom/snapshot-sequencer-validator/config"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/gossipconfig"
	log "github.com/sirupsen/logrus"
)

type P2PHost struct {
	Host      host.Host
	Pubsub    *pubsub.PubSub
	DHT       *dht.IpfsDHT
	Discovery *routing.RoutingDiscovery
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewP2PHost(ctx context.Context, cfg *config.Settings) (*P2PHost, error) {
	// Create cancelable context
	hostCtx, cancel := context.WithCancel(ctx)

	// Create or load private key
	privKey, err := loadOrCreatePrivateKey(cfg.P2PPrivateKey)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get private key: %w", err)
	}

	p2pPort := strconv.Itoa(cfg.P2PPort)

	// Configure connection manager for subscriber mode
	connMgr, err := connmgr.NewConnManager(
		cfg.ConnManagerLowWater,
		cfg.ConnManagerHighWater,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
	}
	log.Infof("Connection manager configured: LowWater=%d, HighWater=%d", cfg.ConnManagerLowWater, cfg.ConnManagerHighWater)

	// Create RFC1918 connection gater to block reserved IP connections
	// This is required by Hetzner to prevent scanning of internal networks
	reservedIPGater := &RFC1918ConnectionGater{}

	// Build libp2p options (EXACT copy from working implementation)
	opts := []libp2p.Option{
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", p2pPort)),
		libp2p.EnableNATService(),
		libp2p.ConnectionManager(connMgr),
		libp2p.ConnectionGater(reservedIPGater), // Block reserved IP connections at dial/accept level
	}

	// Add public IP address if configured
	if cfg.P2PPublicIP != "" {
		publicAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%s", cfg.P2PPublicIP, p2pPort))
		if err != nil {
			log.Errorf("Failed to create public multiaddr: %v", err)
		} else {
			opts = append(opts, libp2p.AddrsFactory(func(addrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
				// Add the public address to the list
				return append(addrs, publicAddr)
			}))
			log.Infof("Advertising public IP: %s", cfg.P2PPublicIP)
		}
	}

	// Create libp2p host with options
	h, err := libp2p.New(opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create host: %w", err)
	}

	log.Infof("P2P Host started with peer ID: %s", h.ID())

	// Setup DHT
	kademliaDHT, err := dht.New(hostCtx, h, dht.Mode(dht.ModeClient))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	if err = kademliaDHT.Bootstrap(hostCtx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	// Connect to all bootstrap peers for resilience
	if len(cfg.BootstrapPeers) > 0 {
		log.Infof("Attempting to connect to %d bootstrap peers", len(cfg.BootstrapPeers))
		connectedCount := 0
		skippedCount := 0
		for i, bootstrapAddr := range cfg.BootstrapPeers {
			// Filter out bootstrap peers with reserved IP addresses
			maddr, err := multiaddr.NewMultiaddr(bootstrapAddr)
			if err == nil && HasReservedIPAddress(maddr) {
				log.Warnf("Skipping bootstrap peer %d with reserved IP: %s", i+1, bootstrapAddr)
				skippedCount++
				continue
			}

			if err := connectToBootstrap(hostCtx, h, bootstrapAddr); err != nil {
				log.WithError(err).Warnf("Failed to connect to bootstrap peer %d: %s", i+1, bootstrapAddr)
			} else {
				connectedCount++
				log.Infof("Successfully connected to bootstrap peer %d: %s", i+1, bootstrapAddr)
			}
		}
		log.Infof("Connected to %d/%d bootstrap peers (skipped %d with reserved IPs)", connectedCount, len(cfg.BootstrapPeers), skippedCount)
		if connectedCount == 0 {
			log.Warn("Failed to connect to any bootstrap peers - will continue with discovery only")
		}
	}

	// Start discovery on rendezvous point
	rendezvousString := cfg.Rendezvous
	routingDiscovery := routing.NewRoutingDiscovery(kademliaDHT)

	// Advertise and discover peers on rendezvous
	go func() {
		log.Infof("Starting peer discovery on rendezvous: %s", rendezvousString)
		util.Advertise(hostCtx, routingDiscovery, rendezvousString)

		for {
			select {
			case <-hostCtx.Done():
				return
			default:
				peerChan, err := routingDiscovery.FindPeers(hostCtx, rendezvousString)
				if err != nil {
					log.Debugf("Error discovering peers: %v", err)
					time.Sleep(10 * time.Second)
					continue
				}

				for p := range peerChan {
					if p.ID == h.ID() {
						continue
					}
					if h.Network().Connectedness(p.ID) != 2 {
						log.Debugf("Found peer through rendezvous: %s", p.ID)
						if err := h.Connect(hostCtx, p); err != nil {
							log.Debugf("Failed to connect to peer %s: %v", p.ID, err)
						} else {
							log.Infof("Connected to peer via rendezvous: %s", p.ID)
						}
					}
				}
				time.Sleep(30 * time.Second)
			}
		}
	}()

	// Also advertise on the submission topics for discovery
	go func() {
		time.Sleep(5 * time.Second) // Wait a bit for DHT to stabilize
		discoveryTopic, submissionsTopic := cfg.GetSnapshotSubmissionTopics()
		topics := []string{
			discoveryTopic,
			submissionsTopic,
		}
		for _, topic := range topics {
			log.Infof("Advertising on topic: %s", topic)
			util.Advertise(hostCtx, routingDiscovery, topic)
		}
	}()

	// Get standardized gossipsub parameters for snapshot submissions mesh
	discoveryTopic, submissionsTopic := cfg.GetSnapshotSubmissionTopics()
	gossipParams, peerScoreParams, peerScoreThresholds, paramHash := gossipconfig.ConfigureSnapshotSubmissionsMesh(h.ID(), discoveryTopic, submissionsTopic)

	// Create pubsub with standardized parameters
	ps, err := pubsub.NewGossipSub(hostCtx, h,
		pubsub.WithGossipSubParams(*gossipParams),
		pubsub.WithPeerScore(peerScoreParams, peerScoreThresholds),
		pubsub.WithDiscovery(routingDiscovery),
		pubsub.WithFloodPublish(true),
		pubsub.WithMessageSignaturePolicy(pubsub.StrictSign),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}

	log.Infof("🔑 Gossipsub parameter hash: %s (p2p gateway)", paramHash)
	log.Info("Initialized gossipsub with standardized snapshot submissions mesh parameters")

	p2pHost := &P2PHost{
		Host:      h,
		Pubsub:    ps,
		DHT:       kademliaDHT,
		Discovery: routingDiscovery,
		ctx:       hostCtx,
		cancel:    cancel,
	}

	// Periodically tag only mesh peers to protect them from connection manager pruning
	// This prevents DSV nodes from pruning legitimate publishers (local collectors) in the mesh
	// Security: We only tag peers in the gossipsub mesh, not all connections (prevents DDoS)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-hostCtx.Done():
				return
			case <-ticker.C:
				if connMgr := h.ConnManager(); connMgr != nil {
					meshPeers := make(map[peer.ID]bool)

					// Collect mesh peers from both topics
					for _, peerID := range ps.ListPeers(discoveryTopic) {
						meshPeers[peerID] = true
						connMgr.TagPeer(peerID, "mesh-peer", 50) // Medium priority
					}
					for _, peerID := range ps.ListPeers(submissionsTopic) {
						if !meshPeers[peerID] {
							meshPeers[peerID] = true
							connMgr.TagPeer(peerID, "mesh-peer", 50) // Medium priority
						}
					}

					if len(meshPeers) > 0 {
						log.Debugf("Tagged %d mesh peers to protect from pruning", len(meshPeers))
					}
				}
			}
		}
	}()

	return p2pHost, nil
}

func (p *P2PHost) Close() error {
	if p.cancel != nil {
		p.cancel()
	}
	return p.Host.Close()
}

// connectToBootstrap connects to a bootstrap peer
func connectToBootstrap(ctx context.Context, h host.Host, bootstrapAddr string) error {
	maddr, err := multiaddr.NewMultiaddr(bootstrapAddr)
	if err != nil {
		return fmt.Errorf("invalid bootstrap address: %w", err)
	}

	peerinfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("failed to parse bootstrap peer info: %w", err)
	}

	if err := h.Connect(ctx, *peerinfo); err != nil {
		return fmt.Errorf("failed to connect to bootstrap: %w", err)
	}

	log.Infof("Connected to bootstrap peer: %s", peerinfo.ID)
	return nil
}

func loadOrCreatePrivateKey(privKeyHex string) (crypto.PrivKey, error) {
	if privKeyHex != "" {
		// Decode hex string to bytes
		privKeyBytes, err := hex.DecodeString(privKeyHex)
		if err != nil {
			return nil, fmt.Errorf("failed to decode private key hex: %v", err)
		}
		// Load existing key
		return crypto.UnmarshalEd25519PrivateKey(privKeyBytes)
	}
	// Generate new key
	privKey, _, err := crypto.GenerateEd25519Key(nil)
	return privKey, err
}
