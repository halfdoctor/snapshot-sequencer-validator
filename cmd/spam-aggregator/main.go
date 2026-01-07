package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
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
	rpchelper "github.com/powerloom/go-rpc-helper"
	"github.com/powerloom/snapshot-sequencer-validator/config"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/eventmonitor"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/p2p"
	rediskeys "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/spam"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Setup logging
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	if os.Getenv("DEBUG_MODE") == "true" {
		log.SetLevel(log.DebugLevel)
	}

	log.Info("========================================")
	log.Info("🛡️  SPAM AGGREGATOR COMPONENT STARTING")
	log.Info("========================================")

	// Load configuration
	if err := config.LoadConfig(); err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}
	cfg := config.SettingsObj

	// Validate spam protection is enabled
	if !cfg.EnableSpamProtection {
		log.Fatal("ENABLE_SPAM_PROTECTION must be true for spam-aggregator component")
	}

	if !cfg.EnableSpamReportBroadcast {
		log.Warn("ENABLE_SPAM_REPORT_BROADCAST is false - spam aggregator will only do local aggregation")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Redis
	redisOpts := &redis.Options{
		Addr: fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		DB:   cfg.RedisDB,
	}
	password := strings.TrimSpace(cfg.RedisPassword)
	if password != "" {
		redisOpts.Password = password
	}
	redisClient := redis.NewClient(redisOpts)

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.WithError(err).Fatal("Failed to connect to Redis")
	}
	log.Info("✅ Connected to Redis")

	// Create key builder
	protocolState := cfg.ProtocolStateContract
	dataMarket := ""
	if len(cfg.DataMarketAddresses) > 0 {
		dataMarket = cfg.DataMarketAddresses[0]
	}
	keyBuilder := rediskeys.NewKeyBuilder(protocolState, dataMarket)

	// Get sequencer ID
	sequencerID := cfg.SequencerID
	if sequencerID == "" {
		log.Fatal("SEQUENCER_ID must be set")
	}

	// Initialize P2P for spam report exchange
	p2pPort := strconv.Itoa(cfg.P2PPort)
	privKey, err := loadOrCreatePrivateKey(cfg.P2PPrivateKey)
	if err != nil {
		log.WithError(err).Fatal("Failed to get private key")
	}

	// Configure connection manager
	connMgr, err := connmgr.NewConnManager(
		cfg.ConnManagerLowWater,
		cfg.ConnManagerHighWater,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to create connection manager")
	}

	// Create RFC1918 connection gater to block reserved IP connections
	reservedIPGater := &p2p.RFC1918ConnectionGater{}

	// Build libp2p options
	opts := []libp2p.Option{
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", p2pPort)),
		libp2p.EnableNATService(),
		libp2p.ConnectionManager(connMgr),
		libp2p.ConnectionGater(reservedIPGater),
	}

	// Add public IP address if configured
	if cfg.P2PPublicIP != "" {
		publicAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%s", cfg.P2PPublicIP, p2pPort))
		if err != nil {
			log.WithError(err).Error("Failed to create public multiaddr")
		} else {
			opts = append(opts, libp2p.AddrsFactory(func(addrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
				return append(addrs, publicAddr)
			}))
			log.Infof("Advertising public IP: %s", cfg.P2PPublicIP)
		}
	}

	// Create libp2p host
	h, err := libp2p.New(opts...)
	if err != nil {
		log.WithError(err).Fatal("Failed to create host")
	}
	log.Infof("P2P Host started with peer ID: %s", h.ID())

	// Setup DHT
	kademliaDHT, err := dht.New(ctx, h, dht.Mode(dht.ModeClient))
	if err != nil {
		log.WithError(err).Fatal("Failed to create DHT")
	}

	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		log.WithError(err).Fatal("Failed to bootstrap DHT")
	}

	// Connect to bootstrap peers
	if len(cfg.BootstrapPeers) > 0 {
		for i, bootstrapAddr := range cfg.BootstrapPeers {
			maddr, err := multiaddr.NewMultiaddr(bootstrapAddr)
			if err == nil && p2p.HasReservedIPAddress(maddr) {
				log.Warnf("Skipping bootstrap peer %d with reserved IP: %s", i+1, bootstrapAddr)
				continue
			}
			connectToBootstrap(ctx, h, bootstrapAddr)
		}
	}

	// Start discovery on rendezvous point
	rendezvousString := cfg.Rendezvous
	routingDiscovery := routing.NewRoutingDiscovery(kademliaDHT)

	// Advertise and discover peers on rendezvous
	go func() {
		log.Infof("Starting peer discovery on rendezvous: %s", rendezvousString)
		util.Advertise(ctx, routingDiscovery, rendezvousString)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				peerChan, err := routingDiscovery.FindPeers(ctx, rendezvousString)
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
						if err := h.Connect(ctx, p); err != nil {
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

	// Create pubsub for spam reports (minimal config - no submission topics)
	ps, err := pubsub.NewGossipSub(ctx, h,
		pubsub.WithDiscovery(routingDiscovery),
		pubsub.WithFloodPublish(true),
		pubsub.WithMessageSignaturePolicy(pubsub.StrictSign),
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to create pubsub")
	}
	log.Info("✅ Initialized gossipsub for spam report exchange")

	// Initialize spam protection components
	spamComponents, err := spam.InitializeSpamProtection(ctx, cfg, redisClient, keyBuilder, ps, sequencerID)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize spam protection")
	}

	if spamComponents.Aggregator == nil {
		log.Fatal("Spam aggregator not initialized")
	}

	log.Info("✅ Spam protection components initialized")

	// Initialize event monitor to detect epoch releases and trigger window aggregation
	if cfg.EnableEventMonitor {
		// Initialize RPC Helper with Powerloom chain config
		rpcConfig := cfg.ToRPCConfig()
		if rpcConfig == nil || len(rpcConfig.Nodes) == 0 {
			log.Fatal("POWERLOOM_RPC_NODES must be configured for event monitoring")
		}

		// Set default timeouts if not configured
		if rpcConfig.RequestTimeout == 0 {
			rpcConfig.RequestTimeout = 30 * time.Second
		}
		if rpcConfig.MaxRetries == 0 {
			rpcConfig.MaxRetries = 3
		}

		rpcHelper := rpchelper.NewRPCHelper(rpcConfig)
		if err := rpcHelper.Initialize(ctx); err != nil {
			log.WithError(err).Fatal("Failed to initialize RPC helper")
		}

		// Create event monitor config
		monitorCfg := &eventmonitor.Config{
			RPCHelper:                rpcHelper,
			ContractAddress:          cfg.ProtocolStateContract,
			ContractABIPath:          cfg.ContractABIPath,
			RedisClient:              redisClient,
			WindowDuration:           cfg.Level1FinalizationDelay,
			StartBlock:               cfg.EventStartBlock,
			PollInterval:             cfg.EventPollInterval,
			DataMarkets:              cfg.DataMarketAddresses,
			MaxWindows:               cfg.MaxConcurrentWindows,
			FinalizationBatchSize:    cfg.FinalizationBatchSize,
			VPAContractAddress:       cfg.VPAContractAddress,
			VPAValidatorAddress:      cfg.VPAValidatorAddress,
			VPARPCURL:                strings.Join(cfg.RPCNodes, ","),
			ProtocolState:            cfg.ProtocolStateContract,
			NewProtocolStateContract: cfg.NewProtocolStateContract,
			WindowConfigCacheTTL:     5 * time.Minute,
			EstimatedMaxPriority:     10,
			NewDataMarketContracts: func() []string {
				if cfg.NewDataMarket != "" {
					return []string{cfg.NewDataMarket}
				}
				return []string{}
			}(),
			SpamComponents: spamComponents,
		}

		eventMonitor, err := eventmonitor.NewEventMonitor(monitorCfg)
		if err != nil {
			log.WithError(err).Fatal("Failed to create event monitor")
		}

		// Start event monitor
		go func() {
			if err := eventMonitor.Start(); err != nil {
				log.WithError(err).Error("Event monitor failed")
			}
		}()

		log.Info("✅ Event monitor started (will trigger window aggregation at epoch boundaries)")
	} else {
		log.Warn("Event monitor disabled - window aggregation will not be triggered automatically")
		log.Warn("Consider enabling ENABLE_EVENT_MONITOR for automatic aggregation")
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down spam aggregator component")
	cancel()

	// Close P2P host
	if err := h.Close(); err != nil {
		log.WithError(err).Error("Error closing P2P host")
	}
}

// loadOrCreatePrivateKey loads or creates a P2P private key
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

// connectToBootstrap connects to a bootstrap peer
func connectToBootstrap(ctx context.Context, h host.Host, bootstrapAddr string) {
	if bootstrapAddr == "" {
		log.Warn("No BOOTSTRAP_MULTIADDR configured, skipping bootstrap connection")
		return
	}

	// Parse bootstrap multiaddr
	maddr, err := multiaddr.NewMultiaddr(bootstrapAddr)
	if err != nil {
		log.Errorf("Invalid bootstrap address %s: %v", bootstrapAddr, err)
		return
	}

	// Extract peer info from multiaddr
	peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		log.Errorf("Failed to parse bootstrap peer info: %v", err)
		return
	}

	// Connect to bootstrap node
	if err := h.Connect(ctx, *peerInfo); err != nil {
		log.Debugf("Failed to connect to bootstrap peer %s: %v", bootstrapAddr, err)
	} else {
		log.Infof("Connected to bootstrap peer: %s", bootstrapAddr)
	}
}
