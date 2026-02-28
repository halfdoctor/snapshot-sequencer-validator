package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
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
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/blockpoller"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/consensus"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/deduplication"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/eventmonitor"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/gossipconfig"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/ipfs"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/p2p"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/protocolstate"
	rediskeys "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/spam"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/submissions"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/workers"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// detectPrimaryComponent identifies the main component role for logging purposes
func detectPrimaryComponent(enableListener, enableDequeuer, enableFinalizer, enableBatchAggregation, enableEventMonitor, enableProtocolStateCacher bool) string {
	// Count enabled components
	enabledCount := 0
	primaryComponent := "unknown"

	if enableListener {
		enabledCount++
		primaryComponent = "listener"
	}
	if enableDequeuer {
		enabledCount++
		primaryComponent = "dequeuer"
	}
	if enableFinalizer {
		enabledCount++
		primaryComponent = "finalizer"
	}
	if enableBatchAggregation {
		enabledCount++
		primaryComponent = "batch-aggregator"
	}
	if enableEventMonitor {
		enabledCount++
		primaryComponent = "event-monitor"
	}
	if enableProtocolStateCacher {
		enabledCount++
		primaryComponent = "protocol-state-cacher"
	}

	// If multiple components are enabled, return "multi-component"
	if enabledCount > 1 {
		return "multi-component"
	}

	return primaryComponent
}

// getComponentEmoji returns appropriate emoji for component identification
func getComponentEmoji(component string) string {
	switch component {
	case "dequeuer":
		return "🚀"
	case "finalizer":
		return "📦"
	case "event-monitor":
		return "🔍"
	case "listener":
		return "📡"
	case "batch-aggregator":
		return "🔄"
	case "protocol-state-cacher":
		return "💾"
	case "multi-component":
		return "🔧"
	default:
		return "⚙️"
	}
}

type UnifiedSequencer struct {
	// Core components
	host        host.Host
	ctx         context.Context
	cancel      context.CancelFunc
	ps          *pubsub.PubSub
	redisClient *redis.Client
	keyBuilder  *rediskeys.KeyBuilder
	dedup       *deduplication.Deduplicator
	ipfsClient  *ipfs.Client

	// Component flags
	enableListener         bool
	enableDequeuer         bool
	enableFinalizer        bool
	enableBatchAggregation bool // P2P exchange and aggregation of finalized batches
	enableEventMonitor     bool

	// Component instances
	dequeuer     *submissions.Dequeuer
	batchGen     *consensus.DummyBatchGenerator
	eventMonitor *eventmonitor.EventMonitor
	p2pConsensus *consensus.P2PConsensus   // P2P consensus handler
	cacher       *protocolstate.Cacher     // Protocol state cacher for accessing SlotManager
	blockPoll    *blockpoller.BlockPoller  // Shared block poller for all event consumers

	// Configuration
	config           *config.Settings
	sequencerID      string
	primaryComponent string
	wg               sync.WaitGroup
}

// parseEpochID parses epoch ID from various formats (string, scientific notation)
func parseEpochID(epochIDStr string) (uint64, error) {
	// Try standard integer parsing first
	epochID, err := strconv.ParseUint(epochIDStr, 10, 64)
	if err == nil {
		return epochID, nil
	}

	// If that fails, try parsing as float64 (for scientific notation)
	floatVal, err := strconv.ParseFloat(epochIDStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse epoch ID '%s' as integer or float: %w", epochIDStr, err)
	}

	// Convert float to uint64, checking for overflow
	if floatVal < 0 || floatVal > float64(^uint64(0)) {
		return 0, fmt.Errorf("epoch ID '%s' is out of valid uint64 range", epochIDStr)
	}

	return uint64(floatVal), nil
}

func main() {
	// Load configuration
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	cfg := config.SettingsObj

	// Initialize logger
	log.SetLevel(log.InfoLevel)
	if cfg.DebugMode {
		log.SetLevel(log.DebugLevel)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parse component flags from configuration
	enableListener := cfg.EnableListener
	enableDequeuer := cfg.EnableDequeuer
	enableFinalizer := cfg.EnableFinalizer
	enableBatchAggregation := cfg.EnableBatchAggregation
	enableEventMonitor := cfg.EnableEventMonitor

	// Detect primary component for clear identification
	primaryComponent := detectPrimaryComponent(enableListener, enableDequeuer, enableFinalizer, enableBatchAggregation, enableEventMonitor, cfg.EnableProtocolStateCacher)
	componentEmoji := getComponentEmoji(primaryComponent)

	// Component-specific startup banner
	if primaryComponent == "multi-component" {
		log.Infof("========================================")
		log.Infof("🔧 MULTI-COMPONENT SEQUENCER STARTING")
		log.Infof("========================================")
		log.Infof("Components enabled:")
		if enableListener {
			log.Infof("  - Listener: %v", enableListener)
		}
		if enableDequeuer {
			log.Infof("  - Dequeuer: %v", enableDequeuer)
		}
		if enableFinalizer {
			log.Infof("  - Finalizer: %v", enableFinalizer)
		}
		if enableBatchAggregation {
			log.Infof("  - Batch Aggregation: %v", enableBatchAggregation)
		}
		if enableEventMonitor {
			log.Infof("  - Event Monitor: %v", enableEventMonitor)
		}
		if cfg.EnableProtocolStateCacher {
			log.Infof("  - Protocol State Cacher: %v", cfg.EnableProtocolStateCacher)
		}
	} else {
		componentName := strings.ToUpper(strings.ReplaceAll(primaryComponent, "-", " "))
		log.Infof("========================================")
		log.Infof("%s %s COMPONENT STARTING", componentEmoji, componentName)
		log.Infof("========================================")
		log.Infof("Role: %s only", componentName)
	}

	// Get sequencer ID from configuration
	sequencerID := cfg.SequencerID

	// Initialize Redis if any component needs it
	var redisClient *redis.Client
	if enableListener || enableDequeuer || enableFinalizer || enableEventMonitor || cfg.EnableProtocolStateCacher {
		redisAddr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)
		componentPrefix := strings.ToUpper(primaryComponent)
		log.Infof("[%s] Connecting to Redis at %s (DB: %d)", componentPrefix, redisAddr, cfg.RedisDB)

		redisOpts := &redis.Options{
			Addr: redisAddr,
			DB:   cfg.RedisDB,
		}
		// Only set password if it's not empty (trim spaces first)
		password := strings.TrimSpace(cfg.RedisPassword)
		if password != "" {
			redisOpts.Password = password
		}
		redisClient = redis.NewClient(redisOpts)

		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Fatalf("Failed to connect to Redis: %v", err)
		}
		log.Infof("[%s] Connected to Redis at %s", componentPrefix, redisAddr)
	}

	// Initialize deduplicator if Redis is available
	var dedup *deduplication.Deduplicator
	if redisClient != nil && enableListener {
		// Configure deduplication
		localCacheSize := cfg.DedupLocalCacheSize
		dedupTTL := cfg.DedupTTL

		var err error
		dedup, err = deduplication.NewDeduplicator(redisClient, localCacheSize, dedupTTL)
		if err != nil {
			log.Fatalf("Failed to create deduplicator: %v", err)
		}
		log.Infof("Deduplicator initialized with local cache size %d and TTL %v", localCacheSize, dedupTTL)
	}

	// Initialize P2P if listener or batch aggregation is enabled
	// Note: Spam protection P2P is handled by separate spam-aggregator component
	var h host.Host
	var ps *pubsub.PubSub
	if enableListener || enableBatchAggregation {
		p2pPort := strconv.Itoa(cfg.P2PPort)

		// Create or load private key
		privKey, err := loadOrCreatePrivateKey(cfg.P2PPrivateKey)
		if err != nil {
			log.Fatalf("Failed to get private key: %v", err)
		}

		// Configure connection manager for subscriber mode
		connMgr, err := connmgr.NewConnManager(
			cfg.ConnManagerLowWater,
			cfg.ConnManagerHighWater,
			connmgr.WithGracePeriod(time.Minute),
		)
		if err != nil {
			log.Fatalf("Failed to create connection manager: %v", err)
		}
		log.Infof("Connection manager configured: LowWater=%d, HighWater=%d", cfg.ConnManagerLowWater, cfg.ConnManagerHighWater)

		// Create RFC1918 connection gater to block reserved IP connections
		// This is required by Hetzner to prevent scanning of internal networks
		reservedIPGater := &p2p.RFC1918ConnectionGater{}

		// Build libp2p options
		opts := []libp2p.Option{
			libp2p.Identity(privKey),
			libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", p2pPort)),
			libp2p.EnableNATService(),
			libp2p.ConnectionManager(connMgr),
			libp2p.ConnectionGater(reservedIPGater), // Block reserved IP connections at dial/accept level
		}

		// Add public IP address if configured
		// CRITICAL: Filter out internal Docker IPs - never advertise these to DHT/gossipsub
		if cfg.P2PPublicIP != "" {
			publicAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%s", cfg.P2PPublicIP, p2pPort))
			if err != nil {
				log.Errorf("Failed to create public multiaddr: %v", err)
			} else {
				opts = append(opts, libp2p.AddrsFactory(func(addrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
					// Filter out internal/reserved addresses - Docker IPs must not be advertised
					filtered, _ := p2p.FilterReservedMultiaddrs(addrs)
					return append(filtered, publicAddr)
				}))
				log.Infof("Advertising public IP: %s (internal addresses filtered)", cfg.P2PPublicIP)
			}
		}

		// Create libp2p host with options
		h, err = libp2p.New(opts...)
		if err != nil {
			log.Fatalf("Failed to create host: %v", err)
		}

		log.Infof("P2P Host started with peer ID: %s", h.ID())

		// Setup DHT
		kademliaDHT, err := dht.New(ctx, h, dht.Mode(dht.ModeClient))
		if err != nil {
			log.Fatalf("Failed to create DHT: %v", err)
		}

		if err = kademliaDHT.Bootstrap(ctx); err != nil {
			log.Fatalf("Failed to bootstrap DHT: %v", err)
		}

		// Connect to bootstrap if configured (filter reserved IPs)
		if len(cfg.BootstrapPeers) > 0 {
			for i, bootstrapAddr := range cfg.BootstrapPeers {
				// Filter out bootstrap peers with reserved IP addresses
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
				util.Advertise(ctx, routingDiscovery, topic)
			}
		}()

		// Get standardized gossipsub parameters for snapshot submissions mesh
		discoveryTopic, submissionsTopic := cfg.GetSnapshotSubmissionTopics()
		gossipParams, peerScoreParams, peerScoreThresholds, paramHash := gossipconfig.ConfigureSnapshotSubmissionsMesh(h.ID(), discoveryTopic, submissionsTopic)

		// Create pubsub with standardized parameters
		ps, err = pubsub.NewGossipSub(ctx, h,
			pubsub.WithGossipSubParams(*gossipParams),
			pubsub.WithPeerScore(peerScoreParams, peerScoreThresholds),
			pubsub.WithDiscovery(routingDiscovery),
			pubsub.WithFloodPublish(true),
			pubsub.WithMessageSignaturePolicy(pubsub.StrictSign),
		)
		if err != nil {
			log.Fatalf("Failed to create pubsub: %v", err)
		}

		log.Infof("🔑 Gossipsub parameter hash: %s (unified sequencer)", paramHash)
		log.Info("Initialized gossipsub with standardized snapshot submissions mesh parameters")

		// Periodically tag only mesh peers to protect them from connection manager pruning
		// This prevents DSV nodes from pruning legitimate publishers (local collectors) in the mesh
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if connMgr := h.ConnManager(); connMgr != nil && ps != nil {
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
	}

	// Initialize IPFS client if finalizer is enabled
	var ipfsClient *ipfs.Client
	if enableFinalizer {
		ipfsURL := cfg.IPFSAPI // Expecting multiaddr format
		if ipfsURL == "" {
			ipfsURL = "/ip4/127.0.0.1/tcp/5001" // Default
		}

		var err error
		ipfsClient, err = ipfs.NewClient(ipfsURL)
		if err != nil {
			log.Warnf("Failed to create IPFS client: %v (batches will not be stored in IPFS)", err)
			// Don't fail completely, just disable IPFS storage
		} else {
			if ipfsClient.IsAvailable(ctx) {
				log.Infof("✅ IPFS client connected to %s", ipfsURL)
			} else {
				log.Warnf("⚠️ IPFS node not available at %s", ipfsURL)
				ipfsClient = nil
			}
		}
	}

	// Create key builder for Redis operations
	var keyBuilder *rediskeys.KeyBuilder
	if redisClient != nil {
		protocolState := cfg.ProtocolStateContract
		dataMarket := ""
		if len(cfg.DataMarketAddresses) > 0 {
			dataMarket = cfg.DataMarketAddresses[0]
		}
		keyBuilder = rediskeys.NewKeyBuilder(protocolState, dataMarket)
	}

	// Create unified sequencer
	sequencer := &UnifiedSequencer{
		host:                   h,
		ctx:                    ctx,
		cancel:                 cancel,
		ps:                     ps,
		redisClient:            redisClient,
		keyBuilder:             keyBuilder,
		dedup:                  dedup,
		ipfsClient:             ipfsClient,
		enableListener:         enableListener,
		enableDequeuer:         enableDequeuer,
		enableFinalizer:        enableFinalizer,
		enableBatchAggregation: enableBatchAggregation,
		enableEventMonitor:     enableEventMonitor,
		config:                 cfg,
		sequencerID:            sequencerID,
		primaryComponent:       primaryComponent,
	}

	// Wait for cold sync completion if slot validation is enabled
	// This applies to ALL components, not just dequeuer, since slot validation
	// affects the entire node's ability to process submissions correctly
	var snapshotterStateAddr common.Address
	if cfg.EnableSlotValidation && redisClient != nil {
		if !cfg.EnableProtocolStateCacher {
			log.Fatal("ENABLE_PROTOCOL_STATE_CACHER must be true when ENABLE_SLOT_VALIDATION is true")
		}

		// Get SnapshotterState address from ProtocolState contract (needed for Redis key)
		rpcConfig := cfg.ToRPCConfig()
		if rpcConfig == nil || len(rpcConfig.Nodes) == 0 {
			log.Fatal("POWERLOOM_RPC_NODES must be configured for slot validation")
		}

		rpcHelper := rpchelper.NewRPCHelper(rpcConfig)
		if err := rpcHelper.Initialize(context.Background()); err != nil {
			log.Fatalf("Failed to initialize RPC helper for slot validation: %v", err)
		}

		var err error
		snapshotterStateAddr, err = protocolstate.GetSnapshotterStateAddress(
			context.Background(),
			rpcHelper,
			cfg.ProtocolStateContract,
			cfg.ContractABIPath,
		)
		if err != nil {
			log.Fatalf("Failed to get SnapshotterState address: %v", err)
		}

		// Wait for cacher service to complete cold sync (max 10 minutes)
		if err := protocolstate.WaitForColdSyncCompletion(
			context.Background(),
			redisClient,
			cfg.ProtocolStateContract,
			snapshotterStateAddr.Hex(),
			cfg.SlotSyncInterval,
			10*time.Minute,
		); err != nil {
			log.Fatalf("Failed to wait for cold sync completion: %v", err)
		}
	}

	// --- Shared RPC Helper ---
	// Create a single RPC helper shared by cacher, event monitor, and BlockPoller.
	var sharedRPCHelper *rpchelper.RPCHelper
	needsRPC := (cfg.EnableProtocolStateCacher || enableEventMonitor) && redisClient != nil
	if needsRPC {
		rpcConfig := cfg.ToRPCConfig()
		if rpcConfig == nil || len(rpcConfig.Nodes) == 0 {
			log.Fatal("POWERLOOM_RPC_NODES must be configured for protocol state cacher / event monitor")
		}
		if rpcConfig.RequestTimeout == 0 {
			rpcConfig.RequestTimeout = 30 * time.Second
		}
		if rpcConfig.MaxRetries == 0 {
			rpcConfig.MaxRetries = 3
		}
		sharedRPCHelper = rpchelper.NewRPCHelper(rpcConfig)
		if err := sharedRPCHelper.Initialize(context.Background()); err != nil {
			log.Fatalf("Failed to initialize shared RPC helper: %v", err)
		}
		log.Info("✅ Shared RPC helper initialized")
	}

	// --- BlockPoller ---
	// Create single BlockPoller when either cacher or event monitor is enabled.
	if needsRPC {
		bp, err := blockpoller.New(&blockpoller.Config{
			RPCHelper:     sharedRPCHelper,
			RedisClient:   redisClient,
			PollInterval:  1 * time.Second,
			MaxBlockRange: 1000,
		})
		if err != nil {
			log.Fatalf("Failed to create BlockPoller: %v", err)
		}
		sequencer.blockPoll = bp
		log.Info("✅ BlockPoller created (1s interval)")
	}

	// --- Protocol State Cacher ---
	if cfg.EnableProtocolStateCacher && redisClient != nil {
		var err error
		if snapshotterStateAddr == (common.Address{}) {
			snapshotterStateAddr, err = protocolstate.GetSnapshotterStateAddress(
				context.Background(),
				sharedRPCHelper,
				cfg.ProtocolStateContract,
				cfg.ContractABIPath,
			)
			if err != nil {
				log.Fatalf("Failed to get SnapshotterState address: %v", err)
			}
		}
		log.Infof("✅ SnapshotterState contract address: %s", snapshotterStateAddr.Hex())

		cacherCfg := &protocolstate.Config{
			RPCHelper:                sharedRPCHelper,
			ProtocolStateContract:    cfg.ProtocolStateContract,
			SnapshotterStateContract: snapshotterStateAddr.Hex(),
			ContractABIPath:          cfg.ContractABIPath,
			RedisClient:              redisClient,
			SlotSyncInterval:         cfg.SlotSyncInterval,
			SlotSyncBatchSize:        cfg.SlotSyncBatchSize,
			EventGapThresholdBlocks:  cfg.EventGapThresholdBlocks,
			ForceFullColdSync:        cfg.ForceFullColdSync,
		}

		cacher, err := protocolstate.NewCacher(cacherCfg)
		if err != nil {
			log.Fatalf("Failed to create protocol state cacher: %v", err)
		}
		sequencer.cacher = cacher

		log.Info("🔄 Starting protocol state cacher cold sync...")
		if err := cacher.WaitForColdSync(context.Background()); err != nil {
			log.Fatalf("Cold sync failed: %v", err)
		}

		// Register slot event processor as BlockPoller consumer
		ep := cacher.GetEventProcessor()
		snapshotterChangedSig, nodeMintedSig, nodeBurnedSig := ep.EventSignatures()
		slotQueries := []blockpoller.FilterQuery{
			{Addresses: []common.Address{ep.ContractAddress()}, Topics: [][]common.Hash{{snapshotterChangedSig}}},
			{Addresses: []common.Address{ep.ContractAddress()}, Topics: [][]common.Hash{{nodeMintedSig}}},
			{Addresses: []common.Address{ep.ContractAddress()}, Topics: [][]common.Hash{{nodeBurnedSig}}},
		}
		slotConsumer := sequencer.blockPoll.RegisterConsumer(
			"SlotEvents", slotQueries, ep.HandleBlockPollerLogs, 1,
			cacher.RedisKeyPrefix(),
		)

		// Initialize consumer start block if no persisted value
		if currentBlock, err := sharedRPCHelper.BlockNumber(context.Background()); err == nil && currentBlock > 0 {
			sequencer.blockPoll.InitializeConsumerBlock(slotConsumer, currentBlock-1)
		}

		sequencer.wg.Add(1)
		go func() {
			defer sequencer.wg.Done()
			cacher.Start(sequencer.ctx)
		}()
		log.Info("✅ Protocol state cacher started")
	}

	// Initialize spam protection components (if enabled)
	// Dequeuer only needs local components (tracker, rateLimiter, flagging) - no P2P needed
	// Spam-aggregator component handles all P2P functionality (aggregator, reporter)
	var spamComponents *spam.SpamComponents
	if cfg.EnableSpamProtection && redisClient != nil {
		var err error
		// Pass nil for pubsub - dequeuer doesn't need P2P, spam-aggregator component handles it
		spamComponents, err = spam.InitializeSpamProtection(ctx, cfg, redisClient, keyBuilder, nil, sequencerID)
		if err != nil {
			log.Errorf("Failed to initialize spam protection: %v (continuing without spam protection)", err)
			spamComponents = nil
		} else {
			log.Info("✅ Spam protection components initialized (local tracking only - P2P handled by spam-aggregator component)")
		}
	}

	// Initialize components based on flags
	if enableDequeuer && redisClient != nil {
		// Get SlotManager from cacher if available (for on-demand slot fetching)
		var slotManager *protocolstate.SlotManager
		if sequencer.cacher != nil {
			slotManager = sequencer.cacher.GetSlotManager()
			log.Debug("SlotManager available for on-demand slot fetching")
		}

		dequeuer, err := submissions.NewDequeuer(redisClient, keyBuilder, sequencerID, cfg.ChainID, cfg.ProtocolStateContract, snapshotterStateAddr, cfg.EnableSlotValidation, spamComponents, slotManager)
		if err != nil {
			log.Fatalf("Failed to create dequeuer: %v", err)
		}
		sequencer.dequeuer = dequeuer
	}

	if enableBatchAggregation {
		sequencer.batchGen = consensus.NewDummyBatchGenerator(sequencerID)

		// Initialize P2P consensus if we have P2P and IPFS
		if ps != nil && ipfsClient != nil {
			var err error
			sequencer.p2pConsensus, err = consensus.NewP2PConsensus(
				ctx, h, ps, redisClient, ipfsClient, sequencerID, cfg,
			)
			if err != nil {
				log.Errorf("Failed to initialize P2P consensus: %v", err)
				// Continue without P2P consensus
				sequencer.p2pConsensus = nil
			}
		}
	}

	if enableEventMonitor && redisClient != nil {
		monitorCfg := &eventmonitor.Config{
			RPCHelper:             sharedRPCHelper,
			ContractAddress:       cfg.ProtocolStateContract,
			ContractABIPath:       cfg.ContractABIPath,
			RedisClient:           redisClient,
			WindowDuration:        cfg.Level1FinalizationDelay,
			StartBlock:            cfg.EventStartBlock,
			PollInterval:          cfg.EventPollInterval,
			DataMarkets:           cfg.DataMarketAddresses,
			MaxWindows:            cfg.MaxConcurrentWindows,
			FinalizationBatchSize: cfg.FinalizationBatchSize,

			// VPA Configuration
			VPAContractAddress:   cfg.VPAContractAddress,
			VPAValidatorAddress:  cfg.VPAValidatorAddress,
			VPAValidatorNodeID:   cfg.VPAValidatorNodeID,
			VPARPCURL:            strings.Join(cfg.RPCNodes, ","),
			ProtocolState:       cfg.ProtocolStateContract,

			// Window Config Configuration
			WindowConfigCacheTTL: 5 * time.Minute,
			EstimatedMaxPriority: 10,

			// Spam protection components
			SpamComponents: spamComponents,
		}

		var err error
		sequencer.eventMonitor, err = eventmonitor.NewEventMonitor(monitorCfg)
		if err != nil {
			log.Errorf("Failed to create event monitor: %v", err)
			sequencer.enableEventMonitor = false
		} else if sequencer.blockPoll != nil {
			// Register event monitor as BlockPoller consumer
			bpQueries := sequencer.eventMonitor.GetBlockPollerQueries()
			pollerQueries := make([]blockpoller.FilterQuery, 0, len(bpQueries))
			for _, q := range bpQueries {
				pollerQueries = append(pollerQueries, blockpoller.FilterQuery{
					Addresses: q.Addresses,
					Topics:    q.Topics,
				})
			}

			redisPrefix := cfg.ProtocolStateContract
			emConsumer := sequencer.blockPoll.RegisterConsumer(
				"EventMonitor", pollerQueries, sequencer.eventMonitor.HandleBlockPollerLogs, 1,
				redisPrefix,
			)

			// Initialize consumer start block if no persisted value
			if currentBlock, err := sharedRPCHelper.BlockNumber(context.Background()); err == nil && currentBlock > 0 {
				sequencer.blockPoll.InitializeConsumerBlock(emConsumer, currentBlock-1)
			}
			log.Info("✅ Event monitor registered as BlockPoller consumer")
		}
	}

	// Start components
	sequencer.Start()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	componentPrefix := strings.ToUpper(sequencer.primaryComponent)
	log.Infof("[%s] Shutting down %s component", componentPrefix, componentPrefix)
	if sequencer.blockPoll != nil {
		sequencer.blockPoll.Stop()
	}
	cancel()
	sequencer.wg.Wait()
}

func (s *UnifiedSequencer) Start() {
	componentPrefix := strings.ToUpper(s.primaryComponent)

	// Start P2P listener component
	if s.enableListener && s.ps != nil {
		log.Infof("[%s] Starting P2P Listener component...", componentPrefix)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runListener()
		}()
	}

	// Start dequeuer component
	if s.enableDequeuer && s.redisClient != nil {
		log.Infof("[%s] Starting Dequeuer component...", componentPrefix)
		workers := s.config.DequeueWorkers
		if workers == 0 {
			workers = 5 // Default fallback
		}
		log.Infof("[%s] Starting %d dequeuer workers", componentPrefix, workers)
		for i := 0; i < workers; i++ {
			s.wg.Add(1)
			go func(workerID int) {
				defer s.wg.Done()
				s.runDequeuerWorker(workerID)
			}(i)
		}

		// Monitor queue depth
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.monitorQueueDepth()
		}()
	}

	// Start finalizer component
	if s.enableFinalizer && s.redisClient != nil {
		log.Infof("[%s] Starting Finalizer component...", componentPrefix)
		if s.ipfsClient != nil {
			log.Infof("[%s] ✅ IPFS client connected", componentPrefix)
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runFinalizer()
		}()
	}

	// Start consensus component
	if s.enableBatchAggregation && s.ps != nil {
		log.Infof("[%s] Starting Batch Aggregation component (P2P finalization exchange)...", componentPrefix)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.runBatchAggregation()
		}()
	}

	// Start BlockPoller (shared by cacher + event monitor)
	if s.blockPoll != nil {
		log.Infof("[%s] Starting BlockPoller...", componentPrefix)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.blockPoll.Start()
		}()
	}

	// Start event monitor component (log dispatch handled by BlockPoller)
	if s.enableEventMonitor && s.eventMonitor != nil {
		log.Infof("[%s] Starting Event Monitor component...", componentPrefix)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if err := s.eventMonitor.Start(); err != nil {
				log.Errorf("Event monitor startup error: %v", err)
			}
			<-s.ctx.Done()
		}()
	}

	// Start monitoring timeline cleanup (removes old entries from simulation/heartbeat timelines)
	if s.redisClient != nil && s.keyBuilder != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.cleanupMonitoringTimelines()
		}()
	}

	log.Infof("[%s] ✅ All enabled components started successfully", componentPrefix)
}

func (s *UnifiedSequencer) runListener() {
	// Get configurable topics
	discoveryTopic, submissionsTopic := s.config.GetSnapshotSubmissionTopics()
	topics := []string{
		discoveryTopic,
		submissionsTopic,
	}

	subs := make(map[string]*pubsub.Subscription)

	for _, topicName := range topics {
		topic, err := s.ps.Join(topicName)
		if err != nil {
			log.Errorf("Failed to join topic %s: %v", topicName, err)
			continue
		}

		sub, err := topic.Subscribe()
		if err != nil {
			log.Errorf("Failed to subscribe to topic %s: %v", topicName, err)
			continue
		}

		subs[topicName] = sub
		log.Infof("📡 Subscribed to topic: %s", topicName)

		// Handle messages for each topic
		go s.handleSubmissionMessages(sub)
	}

	// Periodic stats logging
	go s.logListenerStats(topics)

	// Keep listener running
	<-s.ctx.Done()
}

func (s *UnifiedSequencer) logListenerStats(topics []string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Info("🔵 Listener component active and monitoring P2P network")

	for {
		select {
		case <-ticker.C:
			log.Info("====== P2P LISTENER STATUS ======")
			log.Infof("Host ID: %s", s.host.ID())
			log.Infof("Connected Peers: %d", len(s.host.Network().Peers()))

			for _, topic := range topics {
				peers := s.ps.ListPeers(topic)
				log.Infof("Topic %s: %d peers", topic, len(peers))
			}
			log.Info("=================================")
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *UnifiedSequencer) handleSubmissionMessages(sub *pubsub.Subscription) {
	topicName := sub.Topic()
	// Get discovery topic to compare
	discoveryTopic, _ := s.config.GetSnapshotSubmissionTopics()
	isDiscoveryTopic := topicName == discoveryTopic
	topicLabel := "SUBMISSIONS"
	if isDiscoveryTopic {
		topicLabel = "DISCOVERY/TEST"
	}
	log.Infof("🎧 Started listening on %s topic: %s", topicLabel, topicName)

	for {
		msg, err := sub.Next(s.ctx)
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			log.Errorf("Error reading message from %s: %v", topicName, err)
			continue
		}

		// Skip own messages
		if s.host != nil && msg.ReceivedFrom == s.host.ID() {
			continue
		}

		topicLabel := "SUBMISSION"
		if topicName == discoveryTopic {
			topicLabel = "TEST/DISCOVERY"
		}
		log.Infof("📨 RECEIVED %s on %s from peer %s (size: %d bytes)",
			topicLabel, topicName, msg.ReceivedFrom.ShortString(), len(msg.Data))

		// Queue the submission for processing
		s.queueSubmissionFromP2P(msg.Data, topicName, msg.ReceivedFrom.String())
	}
}

func (s *UnifiedSequencer) queueSubmissionFromP2P(data []byte, topic string, peerID string) {
	// Try to parse the submission to log details
	var submissionInfo map[string]interface{}
	if err := json.Unmarshal(data, &submissionInfo); err == nil {
		// Successfully parsed - log key details
		var epochID interface{}
		if eid, ok := submissionInfo["epoch_id"]; ok {
			epochID = eid
			log.Infof("📋 Submission Details: Epoch=%v, Topic=%s, Peer=%s",
				epochID, topic, peerID[:16])

			// Check if this is a heartbeat message (Epoch 0 with null submissions)
			// Real Epoch 0 submissions will have non-null submissions array
			if epochID == float64(0) || epochID == 0 || epochID == "0" {
				submissions, hasSubmissions := submissionInfo["submissions"]

				// Check if this is a heartbeat (null submissions) vs real Epoch 0 data
				if hasSubmissions && submissions == nil {
					// This is a heartbeat message - log and skip
					log.Debugf("💓 Heartbeat received from %s (skipping queue)", peerID[:16])
					if signature, ok := submissionInfo["signature"]; ok {
						// Try to decode the signature to see heartbeat info
						if sigStr, ok := signature.(string); ok {
							log.Debugf("   Heartbeat signature: %s", sigStr)
						}
					}
					return // Don't queue heartbeat messages
				} else if hasSubmissions {
					// This is a real Epoch 0 submission with data
					log.Infof("📥 Real Epoch 0 submission received with data from %s", peerID[:16])
				}
			}

			// Enhanced debug logging for ALL submissions
			log.Infof("🔍 DEBUG: Full submission content:")
			log.Infof("   Raw JSON (first 500 chars): %s", truncateString(string(data), 500))

			// Log all top-level keys
			keys := make([]string, 0, len(submissionInfo))
			for k := range submissionInfo {
				keys = append(keys, k)
			}
			log.Infof("   Top-level keys: %v", keys)

			// Log specific fields if present
			if snapshotterId, ok := submissionInfo["snapshotter_id"]; ok {
				log.Infof("   Snapshotter ID: %v", snapshotterId)
			}
			if signature, ok := submissionInfo["signature"]; ok {
				log.Infof("   Signature present: yes (len=%d)", len(fmt.Sprintf("%v", signature)))
			}
			if submissions, ok := submissionInfo["submissions"]; ok {
				log.Infof("   Submissions field present: %T", submissions)
				// If submissions is an array, check the first item for request field
				if subArray, ok := submissions.([]interface{}); ok && len(subArray) > 0 {
					if firstSub, ok := subArray[0].(map[string]interface{}); ok {
						if request, ok := firstSub["request"]; ok {
							if reqMap, ok := request.(map[string]interface{}); ok {
								log.Infof("   First submission request: ProjectID=%v, SnapshotCID=%v, EpochID=%v",
									reqMap["projectId"], reqMap["snapshotCid"], reqMap["epochId"])
							}
						}
					}
				}
			}
			// Check for top-level request (in case of different message format)
			if request, ok := submissionInfo["request"]; ok {
				log.Infof("   Request field present at top level: %T", request)
			}
		}

		// Check if it's a P2P batch submission
		if submissions, ok := submissionInfo["submissions"]; ok {
			if subArray, ok := submissions.([]interface{}); ok {
				log.Infof("   └─ Batch submission with %d items", len(subArray))
			}
		}

		// Extract deduplication keys if dedup is enabled
		var dedupKey string
		if s.dedup != nil {
			// Try to extract project_id and snapshot_cid for deduplication
			if request, ok := submissionInfo["request"].(map[string]interface{}); ok {
				projectID := fmt.Sprintf("%v", request["project_id"])
				snapshotCID := fmt.Sprintf("%v", request["snapshot_cid"])

				// Convert epoch_id to uint64
				var epoch uint64
				switch v := epochID.(type) {
				case float64:
					epoch = uint64(v)
				case int:
					epoch = uint64(v)
				case string:
					if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
						epoch = parsed
					}
				}

				if projectID != "" && snapshotCID != "" {
					dedupKey = s.dedup.GenerateKey(projectID, epoch, snapshotCID)

					// Check if we've seen this submission before
					isNew, err := s.dedup.CheckAndMark(s.ctx, dedupKey)
					if err != nil {
						log.Errorf("Deduplication check failed: %v", err)
						// Continue processing on error to avoid losing submissions
					} else if !isNew {
						log.Debugf("🔁 Duplicate submission detected (key=%s), skipping", dedupKey[:16])
						return // Skip duplicate
					}
				}

				log.Infof("   └─ SlotID=%v, ProjectID=%v, CID=%v",
					request["slot_id"], projectID, snapshotCID)
			}
		} else if request, ok := submissionInfo["request"].(map[string]interface{}); ok {
			// No dedup, just log
			log.Infof("   └─ SlotID=%v, ProjectID=%v, CID=%v",
				request["slot_id"], request["project_id"], request["snapshot_cid"])
		}
	} else {
		log.Infof("📋 Received raw submission (%d bytes) from %s", len(data), peerID[:16])
	}

	if s.redisClient != nil {
		// Enrich submission with protocol state if not already present
		var enrichedData []byte
		if submissionInfo != nil {
			// Add protocol state if missing
			if _, hasProtocol := submissionInfo["protocol_state"]; !hasProtocol {
				submissionInfo["protocol_state"] = s.config.ProtocolStateContract
			}

			// Ensure data market is set (use first configured market as default if missing)
			if _, hasMarket := submissionInfo["data_market"]; !hasMarket && len(s.config.DataMarketAddresses) > 0 {
				submissionInfo["data_market"] = s.config.DataMarketAddresses[0]
			}

			enrichedData, _ = json.Marshal(submissionInfo)
		} else {
			enrichedData = data // Use original if parsing failed
		}

		submissionQueueKey := s.keyBuilder.SubmissionQueue()
		err := s.redisClient.LPush(s.ctx, submissionQueueKey, enrichedData).Err()
		if err != nil {
			log.Errorf("❌ Failed to queue submission: %v", err)
		} else {
			// Extract key info for logging
			epochID := "unknown"
			projectID := "unknown"
			if submissionInfo != nil {
				if ep, ok := submissionInfo["epochId"]; ok {
					epochID = fmt.Sprintf("%v", ep)
				}
				if proj, ok := submissionInfo["projectId"]; ok {
					projectID = fmt.Sprintf("%v", proj)
				}
			}

			log.Infof("✅ Queued submission: Epoch=%s, Project=%s from peer %s",
				epochID, projectID, peerID[:16])

			// Skip monitoring for epoch 0 heartbeats (P2P mesh maintenance)
			if epochID != "0" || (epochID == "0" && projectID != "") {
				// Add monitoring metrics
				submissionID := fmt.Sprintf("%s-%s-%d", epochID, projectID, time.Now().UnixNano())
				timestamp := time.Now().Unix()
				hour := time.Now().Format("2006010215") // YYYYMMDDHH format

				// Pipeline for monitoring metrics
				pipe := s.redisClient.Pipeline()

				// 1. Add to timeline (sorted set, no TTL - pruned daily)
				pipe.ZAdd(s.ctx, s.keyBuilder.MetricsSubmissionsTimeline(), redis.Z{
					Score:  float64(timestamp),
					Member: submissionID,
				})

				// 2. Store submission details with TTL (1 hour)
				submissionData := map[string]interface{}{
					"epoch_id":    epochID,
					"project_id":  projectID,
					"peer_id":     peerID[:16],
					"timestamp":   timestamp,
					"data_market": s.config.DataMarketAddresses[0],
				}
				jsonData, _ := json.Marshal(submissionData)
				pipe.SetEx(s.ctx, fmt.Sprintf("metrics:submission:%s", submissionID), string(jsonData), time.Hour)

				// 3. Update hourly counter
				pipe.HIncrBy(s.ctx, fmt.Sprintf("metrics:hourly:%s:submissions", hour), "total", 1)
				pipe.Expire(s.ctx, fmt.Sprintf("metrics:hourly:%s:submissions", hour), 2*time.Hour)

				// 4. Add to submissions timeline
				timestamp = time.Now().Unix()
				pipe.ZAdd(s.ctx, s.keyBuilder.MetricsSubmissionsTimeline(), redis.Z{
					Score:  float64(timestamp),
					Member: fmt.Sprintf("received:%s:%d", submissionID, timestamp),
				})

				// 5. Publish state change event
				pipe.Publish(s.ctx, "state:change", fmt.Sprintf("submission:received:%s", submissionID))

				// Execute pipeline (ignore errors - monitoring is non-critical)
				if _, err := pipe.Exec(s.ctx); err != nil {
					log.Debugf("Failed to write monitoring metrics: %v", err)
				}
			}

			// Log current queue depth
			submissionQueueKey := s.keyBuilder.SubmissionQueue()
			if length, err := s.redisClient.LLen(s.ctx, submissionQueueKey).Result(); err == nil {
				log.Debugf("   Queue depth now: %d", length)
			}
		}
	} else {
		log.Warnf("⚠️ Redis not available - submission not queued")
	}
}

// Helper function for min of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *UnifiedSequencer) runDequeuerWorker(workerID int) {
	log.Infof("Dequeuer worker %d started", workerID)

	for {
		select {
		case <-s.ctx.Done():
			log.Infof("Dequeuer worker %d shutting down", workerID)
			return
		default:
			// Pop from queue with timeout - using keyBuilder
			queueKey := s.keyBuilder.SubmissionQueue()
			result, err := s.redisClient.BRPop(s.ctx, 2*time.Second, queueKey).Result()
			if err != nil {
				if err == redis.Nil {
					continue // Queue empty
				}
				log.Errorf("Worker %d: Error popping from queue: %v", workerID, err)
				continue
			}

			if len(result) < 2 {
				continue
			}

			// Process the submission
			rawSubmissionData := []byte(result[1])

			// Check if submission is wrapped with peer ID metadata
			var wrappedSubmission map[string]interface{}
			var submissionData []byte
			var peerID string

			if err := json.Unmarshal(rawSubmissionData, &wrappedSubmission); err == nil && wrappedSubmission["peer_id"] != nil && wrappedSubmission["data"] != nil {
				// This is wrapped submission with peer ID
				if p, ok := wrappedSubmission["peer_id"].(string); ok {
					peerID = p
				}
				if dataStr, ok := wrappedSubmission["data"].(string); ok {
					submissionData = []byte(dataStr)
				} else if dataMap, ok := wrappedSubmission["data"].(map[string]interface{}); ok {
					var err error
					submissionData, err = json.Marshal(dataMap)
					if err != nil {
						log.Warnf("Worker %d: Failed to marshal data map: %v", workerID, err)
						submissionData = rawSubmissionData
					}
				} else if dataArray, ok := wrappedSubmission["data"].([]interface{}); ok {
					var err error
					submissionData, err = json.Marshal(dataArray)
					if err != nil {
						log.Warnf("Worker %d: Failed to marshal data array: %v", workerID, err)
						submissionData = rawSubmissionData
					}
				} else if data, ok := wrappedSubmission["data"].([]byte); ok {
					submissionData = data
				} else {
					var err error
					submissionData, err = json.Marshal(wrappedSubmission["data"])
					if err != nil {
						log.Warnf("Worker %d: Unexpected data type in wrapped submission, using raw data: %T", workerID, wrappedSubmission["data"])
						submissionData = rawSubmissionData
					}
				}
			} else {
				// Legacy format - no wrapper
				submissionData = rawSubmissionData
			}

			// First try to parse as P2P batch submission
			var p2pSubmission submissions.P2PSnapshotSubmission
			unmarshalErr := json.Unmarshal(submissionData, &p2pSubmission)
			if unmarshalErr == nil && p2pSubmission.Submissions != nil {
				// This is a P2P batch submission
				log.Debugf("Worker %d: Processing P2P batch with %d submissions for epoch %d",
					workerID, len(p2pSubmission.Submissions), p2pSubmission.EpochID)

				// Detect heartbeat BEFORE data market validation
				// Heartbeats are epoch 0 messages with empty CID (not EIP-712 signed, peer ID only)
				if p2pSubmission.EpochID == 0 {
					isHeartbeat := false
					if len(p2pSubmission.Submissions) == 0 {
						isHeartbeat = true
					} else if len(p2pSubmission.Submissions) == 1 && p2pSubmission.Submissions[0].Request.SnapshotCid == "" {
						isHeartbeat = true
					}

					if isHeartbeat {
						s.cacheHeartbeat(peerID)
						log.Debugf("Worker %d: Cached heartbeat from peer %s (epoch=0, batch)", workerID, peerID[:min(16, len(peerID))])
						continue // Skip further processing of this heartbeat
					}
				}

				// Process each submission in the batch
				for _, submission := range p2pSubmission.Submissions {
					// Validate data market address - reject if not configured
					if !s.isValidDataMarket(submission.DataMarket) {
						log.Debugf("Worker %d: Rejected submission for unconfigured data market: Epoch=%d, Market=%s", workerID, submission.Request.EpochId, submission.DataMarket)
						continue
					}

					// Generate submission ID
					submissionID := fmt.Sprintf("%d-%s-%d-%s",
						submission.Request.EpochId,
						submission.Request.ProjectId,
						submission.Request.SlotId,
						submission.Request.SnapshotCid)

					// Prepare metadata for dequeuer
					metaData := map[string]interface{}{
						"peer_id": peerID,
					}

					// Process the submission
					if s.dequeuer != nil {
						snapshotterAddr, err := s.dequeuer.ProcessSubmission(submission, submissionID, metaData)
						if err != nil {
							// Epoch 0 heartbeats are expected to fail validation - log as debug, not error
							if strings.Contains(err.Error(), "epoch 0 heartbeat") {
								log.Debugf("Worker %d: Skipped epoch 0 heartbeat (P2P mesh maintenance)", workerID)
							} else {
								log.Errorf("Worker %d: Failed to process submission: Epoch=%d, Project=%s, Slot=%d, Market=%s, CID=%s, Peer=%s, Snapshotter=%s: %v", workerID, submission.Request.EpochId, submission.Request.ProjectId,
									submission.Request.SlotId, submission.DataMarket, submission.Request.SnapshotCid, peerID, snapshotterAddr, err)
							}
						} else {
							// Log with snapshotter address extracted from EIP-712 signature
							log.Infof("Worker %d processed submission: Epoch=%d, Project=%s, Slot=%d, Market=%s, CID=%s, Peer=%s, Snapshotter=%s",
								workerID, submission.Request.EpochId, submission.Request.ProjectId,
								submission.Request.SlotId, submission.DataMarket, submission.Request.SnapshotCid, peerID, snapshotterAddr)
						}
					}
				}
			} else {
				// P2P batch unmarshalling failed or Submissions was nil - log and try single submission format
				if unmarshalErr != nil {
					log.Warnf("Worker %d: Failed to unmarshal as P2P batch submission: %v", workerID, unmarshalErr)
					log.Debugf("Worker %d: Raw submission data (first 500 bytes): %s", workerID, string(submissionData[:min(500, len(submissionData))]))
				} else if p2pSubmission.Submissions == nil {
					// Check if this is a heartbeat (epoch 0 with nil submissions = discovery heartbeat)
					if p2pSubmission.EpochID == 0 {
						s.cacheHeartbeat(peerID)
						log.Debugf("Worker %d: Cached heartbeat from peer %s (epoch=0, nil submissions)", workerID, peerID[:min(16, len(peerID))])
						continue // Skip further processing
					}
					log.Debugf("Worker %d: P2P batch submission has nil Submissions array, trying single submission format", workerID)
				}

				// Try parsing as single submission (fallback)
				var submission submissions.SnapshotSubmission
				if err := json.Unmarshal(submissionData, &submission); err != nil {
					log.Errorf("Worker %d: Failed to parse submission (both P2P batch and single formats failed): %v", workerID, err)
					log.Debugf("Worker %d: Raw submission data: %s", workerID, string(submissionData[:min(200, len(submissionData))]))
					continue
				}

				// Validate data market address - reject if not configured
				if !s.isValidDataMarket(submission.DataMarket) {
					log.Debugf("Worker %d: Rejected submission for unconfigured data market: Epoch=%d, Market=%s", workerID, submission.Request.EpochId, submission.DataMarket)
					continue
				}

				log.Debugf("Worker %d: Parsed single submission: Epoch=%d, Project=%s, Market=%s", workerID, submission.Request.EpochId, submission.Request.ProjectId, submission.DataMarket)

				// Generate submission ID
				submissionID := fmt.Sprintf("%d-%s-%d-%s",
					submission.Request.EpochId,
					submission.Request.ProjectId,
					submission.Request.SlotId,
					submission.Request.SnapshotCid)

				// Prepare metadata for dequeuer
				metaData := map[string]interface{}{
					"peer_id": peerID,
				}

				// Process the submission
				if s.dequeuer != nil {
					snapshotterAddr, err := s.dequeuer.ProcessSubmission(&submission, submissionID, metaData)
					if err != nil {
						// Epoch 0 heartbeats are expected to fail validation - log as debug, not error
						if strings.Contains(err.Error(), "epoch 0 heartbeat") {
							log.Debugf("Worker %d: Skipped epoch 0 heartbeat (P2P mesh maintenance)", workerID)
						} else {
							log.Errorf("Worker %d: Failed to process submission: Epoch=%d, Project=%s, Slot=%d, Market=%s, CID=%s, Peer=%s, Snapshotter=%s: %v", workerID, submission.Request.EpochId, submission.Request.ProjectId,
								submission.Request.SlotId, submission.DataMarket, submission.Request.SnapshotCid, peerID, snapshotterAddr, err)
						}
					} else {
						// Log with snapshotter address extracted from EIP-712 signature
						log.Infof("Worker %d processed submission: Epoch=%d, Project=%s, Slot=%d, Market=%s, CID=%s, Peer=%s, Snapshotter=%s",
							workerID, submission.Request.EpochId, submission.Request.ProjectId,
							submission.Request.SlotId, submission.DataMarket, submission.Request.SnapshotCid, peerID, snapshotterAddr)
					}
				} else {
					log.Warnf("Worker %d: Dequeuer not initialized, skipping storage", workerID)
				}
			}
		}
	}
}

// cacheHeartbeat stores heartbeat messages (epoch 0 with empty CID) in Redis for monitoring.
// Heartbeats are P2P mesh maintenance messages from local-collector - NOT EIP-712 signed.
// Only peer ID is available (no snapshotter address). To correlate peer ID with snapshotter,
// use simulation or submission data from other endpoints.
func (s *UnifiedSequencer) cacheHeartbeat(peerID string) {
	if s.redisClient == nil || s.keyBuilder == nil {
		return // Redis not available
	}

	timestamp := time.Now().Unix()
	entityID := fmt.Sprintf("hb:%s:%d", peerID, timestamp)

	pipe := s.redisClient.Pipeline()

	// Add to heartbeats timeline (ZSET sorted by timestamp)
	pipe.ZAdd(s.ctx, s.keyBuilder.HeartbeatsTimeline(), redis.Z{
		Score:  float64(timestamp),
		Member: entityID,
	})

	// Add to peer index with 24h TTL (ZSET to track heartbeat frequency per peer)
	peerKey := s.keyBuilder.HeartbeatsByPeer(peerID)
	pipe.ZAdd(s.ctx, peerKey, redis.Z{
		Score:  float64(timestamp),
		Member: entityID,
	})
	pipe.Expire(s.ctx, peerKey, 24*time.Hour)

	// Execute pipeline (ignore errors - heartbeat caching is non-critical)
	pipe.Exec(s.ctx)
}

// cleanupMonitoringTimelines periodically removes old entries from simulation and heartbeat timelines.
// This prevents unbounded growth of ZSET keys that don't have TTLs.
// Retention periods: simulations=7 days (matches metadata TTL), heartbeats=24 hours (matches per-peer TTL)
func (s *UnifiedSequencer) cleanupMonitoringTimelines() {
	if s.redisClient == nil || s.keyBuilder == nil {
		return
	}

	// Run initial cleanup immediately at startup
	log.Info("Running initial monitoring timeline cleanup...")
	s.doTimelineCleanup()

	ticker := time.NewTicker(1 * time.Hour) // Run cleanup every hour
	defer ticker.Stop()

	log.Info("Started monitoring timeline cleanup goroutine (hourly)")

	for {
		select {
		case <-s.ctx.Done():
			log.Info("Stopping monitoring timeline cleanup")
			return
		case <-ticker.C:
			s.doTimelineCleanup()
		}
	}
}

// doTimelineCleanup removes entries older than retention period from monitoring timelines
func (s *UnifiedSequencer) doTimelineCleanup() {
	ctx := context.Background()

	// Simulation timeline: keep 7 days (matches metadata TTL)
	simulationCutoff := time.Now().Add(-7 * 24 * time.Hour).Unix()
	simRemoved, err := s.redisClient.ZRemRangeByScore(ctx, s.keyBuilder.SimulationsTimeline(),
		"-inf", fmt.Sprintf("%d", simulationCutoff)).Result()
	if err != nil {
		log.Warnf("Failed to cleanup simulations timeline: %v", err)
	} else if simRemoved > 0 {
		log.Infof("Cleaned up %d old entries from simulations timeline (older than 7 days)", simRemoved)
	}

	// Heartbeat timeline: keep 24 hours (matches per-peer TTL)
	heartbeatCutoff := time.Now().Add(-24 * time.Hour).Unix()
	hbRemoved, err := s.redisClient.ZRemRangeByScore(ctx, s.keyBuilder.HeartbeatsTimeline(),
		"-inf", fmt.Sprintf("%d", heartbeatCutoff)).Result()
	if err != nil {
		log.Warnf("Failed to cleanup heartbeats timeline: %v", err)
	} else if hbRemoved > 0 {
		log.Infof("Cleaned up %d old entries from heartbeats timeline (older than 24 hours)", hbRemoved)
	}
}

func (s *UnifiedSequencer) runFinalizer() {
	log.Info("Starting finalizer component with parallel workers")

	// Start multiple parallel workers
	numWorkers := s.config.FinalizerWorkers
	if numWorkers == 0 {
		numWorkers = 5 // Default to 5 workers
	}

	log.Infof("Starting %d parallel finalization workers", numWorkers)

	for i := 0; i < numWorkers; i++ {
		go s.runFinalizationWorker(i)
	}

	// NOTE: Aggregation is now handled by the separate aggregator component
	// The unified sequencer only acts as a finalizer worker that creates batch parts
	// go s.runAggregationWorker() // DISABLED - aggregator component handles this

	// Wait for shutdown
	<-s.ctx.Done()
	log.Info("Finalizer component shutting down")
}

func (s *UnifiedSequencer) runFinalizationWorker(workerID int) {
	log.Infof("Finalization worker %d started", workerID)

	// Create worker monitor
	monitor := workers.NewWorkerMonitor(
		s.redisClient,
		fmt.Sprintf("finalizer-%d", workerID),
		workers.WorkerTypeFinalizer,
		s.config.ProtocolStateContract,
	)
	monitor.StartWorker(s.ctx)
	defer monitor.CleanupWorker()

	// Finalization queue key using keyBuilder
	queueKey := s.keyBuilder.FinalizationQueue()
	log.Debugf("Worker %d listening on finalization queue: %s", workerID, queueKey)

	for {
		select {
		case <-s.ctx.Done():
			log.Infof("Finalization worker %d shutting down", workerID)
			return
		default:
			// Pop from batch parts queue with blocking timeout
			result, err := s.redisClient.BRPop(s.ctx, 5*time.Second, queueKey).Result()
			if err != nil {
				if err == redis.Nil {
					continue // Queue empty, wait
				}
				log.Errorf("Worker %d: Error popping from queue: %v", workerID, err)
				time.Sleep(1 * time.Second)
				continue
			}

			if len(result) < 2 {
				continue
			}

			// Parse batch part data
			var batchPart map[string]interface{}
			if err := json.Unmarshal([]byte(result[1]), &batchPart); err != nil {
				log.Errorf("Worker %d: Failed to parse batch part: %v", workerID, err)
				continue
			}

			// Parse epoch ID (comes as string from event monitor)
			epochIDStr, ok := batchPart["epoch_id"].(string)
			if !ok {
				// Fallback for float format
				if epochFloat, ok := batchPart["epoch_id"].(float64); ok {
					epochIDStr = fmt.Sprintf("%.0f", epochFloat)
				} else {
					log.Errorf("Worker %d: Invalid epoch_id format", workerID)
					continue
				}
			}
			epochID, _ := parseEpochID(epochIDStr)

			batchID := int(batchPart["batch_id"].(float64))
			totalBatches := int(batchPart["total_batches"].(float64))

			// Extract data_market (mandatory field)
			dataMarketRaw, ok := batchPart["data_market"]
			if !ok {
				log.Warnf("Worker %d: Skipping batch part without data_market field - old format not supported: epoch=%d, batch=%d", workerID, epochID, batchID)
				continue
			}
			dataMarketStr, ok := dataMarketRaw.(string)
			if !ok || dataMarketStr == "" {
				log.Warnf("Worker %d: Skipping batch part without data_market field - old format not supported: epoch=%d, batch=%d", workerID, epochID, batchID)
				continue
			}
			// Normalize to checksummed format
			dataMarket := common.HexToAddress(dataMarketStr).Hex()

			// Extract projects map (not project_ids array)
			projects, ok := batchPart["projects"].(map[string]interface{})
			if !ok {
				log.Errorf("Worker %d: Invalid projects format in batch", workerID)
				continue
			}

			log.Infof("Worker %d: Processing batch %d/%d for epoch %d with %d projects",
				workerID, batchID+1, totalBatches, epochID, len(projects))

			// Update monitoring
			batchInfo := fmt.Sprintf("epoch:%d:batch:%d/%d", epochID, batchID, totalBatches)
			monitor.ProcessingStarted(batchInfo)

			// Process this batch part with projects map
			if err := s.processBatchPart(epochID, batchID, totalBatches, projects, dataMarket, monitor); err != nil {
				log.Errorf("Worker %d: Failed to process batch part %d for epoch %d: %v",
					workerID, batchID, epochID, err)
				monitor.ProcessingFailed(err)
			} else {
				monitor.ProcessingCompleted()
				log.Infof("Worker %d: Completed batch part %d/%d for epoch %d",
					workerID, batchID, totalBatches, epochID)
			}
		}
	}
}

func (s *UnifiedSequencer) processBatchPart(epochID uint64, batchID int, totalBatches int, projects map[string]interface{}, dataMarket string, _ *workers.WorkerMonitor) error {
	ctx := context.Background()

	// Track batch part as processing
	epochStr := fmt.Sprintf("%d", epochID)
	if err := workers.TrackBatchPart(s.redisClient, epochStr, batchID, "processing"); err != nil {
		log.WithError(err).Error("Failed to track batch part as processing")
	}

	// Process each project in this batch part
	// Projects now contain ALL CIDs with vote counts, we need to select winners
	partResults := make(map[string]interface{})

	// Process each project's vote data and select consensus CID
	for projectID, submissionData := range projects {
		projectData, ok := submissionData.(map[string]interface{})
		if !ok {
			log.Errorf("Invalid project data format for project %s", projectID)
			continue
		}

		// Extract the CID votes map
		cidVotesRaw, exists := projectData["cid_votes"]
		if !exists {
			log.Errorf("No cid_votes found for project %s", projectID)
			continue
		}

		// Extract submission metadata (WHO submitted WHAT for rewards)
		var submissionMetadata []interface{}
		if metadataRaw, exists := projectData["submission_metadata"]; exists {
			if metadataArray, ok := metadataRaw.([]interface{}); ok {
				submissionMetadata = metadataArray
			}
		}

		// Convert to proper type
		cidVotes, ok := cidVotesRaw.(map[string]int)
		if !ok {
			// Try to convert from map[string]interface{}
			cidVotesInterface, ok := cidVotesRaw.(map[string]interface{})
			if !ok {
				log.Errorf("Invalid cid_votes format for project %s", projectID)
				continue
			}

			// Convert to map[string]int
			cidVotes = make(map[string]int)
			for cid, votesRaw := range cidVotesInterface {
				if votesFloat, ok := votesRaw.(float64); ok {
					cidVotes[cid] = int(votesFloat)
				} else if votesInt, ok := votesRaw.(int); ok {
					cidVotes[cid] = votesInt
				}
			}
		}

		// Select winning CID based on consensus (highest vote count)
		var winningCID string
		maxVotes := 0
		totalVotes := 0

		for cid, votes := range cidVotes {
			totalVotes += votes
			if votes > maxVotes {
				winningCID = cid
				maxVotes = votes
			}
		}

		if winningCID != "" {
			partResults[projectID] = map[string]interface{}{
				"cid":                 winningCID,
				"votes":               maxVotes,
				"total_votes":         totalVotes,
				"unique_cids":         len(cidVotes),
				"submission_metadata": submissionMetadata, // WHO submitted WHAT for rewards
			}
			log.Debugf("Project %s: Selected CID %s with %d/%d votes (from %d unique CIDs, %d submissions)",
				projectID, winningCID, maxVotes, totalVotes, len(cidVotes), len(submissionMetadata))
		}
	}

	// Store batch part results with data_market
	partResultsWithMeta := map[string]interface{}{
		"data_market": dataMarket,
		"projects":    partResults,
	}
	// Use KeyBuilder for the correct data market to ensure correct Redis key namespace
	partKeyBuilder := rediskeys.NewKeyBuilder(s.config.ProtocolStateContract, dataMarket)
	partKey := partKeyBuilder.BatchPart(fmt.Sprintf("%d", epochID), batchID)
	partData, err := json.Marshal(partResultsWithMeta)
	if err != nil {
		return fmt.Errorf("failed to marshal batch part: %w", err)
	}

	if err := s.redisClient.Set(ctx, partKey, partData, 2*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to store batch part: %w", err)
	}

	// Mark batch part as completed
	if err := workers.TrackBatchPart(s.redisClient, epochStr, batchID, "completed"); err != nil {
		log.WithError(err).Error("Failed to track batch part as completed")
	}

	// Add monitoring metrics for part creation
	timestamp := time.Now().Unix()
	partID := fmt.Sprintf("%d:%d", epochID, batchID)

	// Pipeline for monitoring metrics
	pipe := s.redisClient.Pipeline()

	// 1. Increment epoch parts counter with TTL
	epochPartsKey := fmt.Sprintf("metrics:epoch:%d:parts", epochID)
	pipe.HIncrBy(ctx, epochPartsKey, "count", 1)
	pipe.Expire(ctx, epochPartsKey, 2*time.Hour)

	// 2. Store part details with TTL
	partMetricsKey := fmt.Sprintf("metrics:part:%s", partID)
	partMetricsData := map[string]interface{}{
		"epoch_id":      epochID,
		"batch_id":      batchID,
		"total_batches": totalBatches,
		"project_count": len(projects),
		"finalizer_id":  s.config.SequencerID,
		"timestamp":     timestamp,
	}
	jsonData, _ := json.Marshal(partMetricsData)
	pipe.SetEx(ctx, partMetricsKey, string(jsonData), time.Hour)

	// 3. Add to parts timeline
	pipe.ZAdd(ctx, "metrics:parts:timeline", redis.Z{
		Score:  float64(timestamp),
		Member: partID,
	})

	// 4. Publish state change
	pipe.Publish(ctx, "state:change", fmt.Sprintf("part:created:%s", partID))

	// Execute pipeline (ignore errors - monitoring is non-critical)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Debugf("Failed to write monitoring metrics: %v", err)
	}

	// Update progress tracking - use the same KeyBuilder for the correct data market
	completedKey := partKeyBuilder.EpochPartsCompleted(fmt.Sprintf("%d", epochID))
	completed, _ := s.redisClient.Incr(ctx, completedKey).Result()

	// Check if all parts are complete
	// Use the dataMarket from the batch part, not the first configured data market
	if err := workers.UpdateBatchPartsProgress(s.redisClient, s.config.ProtocolStateContract, dataMarket, epochStr, int(completed), totalBatches); err != nil {
		log.WithError(err).Error("Failed to update batch parts progress")
	}

	// Log batch contents for operator visibility
	log.Infof("✅ Processed batch part %d/%d for epoch %d: %d projects finalized",
		batchID, totalBatches, epochID, len(partResults))

	// Show first few projects as examples (limit to avoid log spam)
	count := 0
	for projectID, projectData := range partResults {
		if count >= 3 {
			if len(partResults) > 3 {
				log.Debugf("  ... and %d more projects", len(partResults)-3)
			}
			break
		}
		if pd, ok := projectData.(map[string]interface{}); ok {
			if cid, ok := pd["cid"].(string); ok {
				log.Infof("  • %s → %s", projectID, cid[:16]+"...")
			}
		}
		count++
	}

	return nil
}

// DEPRECATED: Legacy functions below are no longer used - aggregator component handles this now
// Kept for reference only, can be removed in future cleanup

/*
func (s *UnifiedSequencer) runAggregationWorker() {
	log.Info("Aggregation worker started")

	// Create worker monitor
	monitor := workers.NewWorkerMonitor(
		s.redisClient,
		"aggregator-main",
		workers.WorkerTypeAggregator,
		s.config.ProtocolStateContract,
	)
	monitor.StartWorker(s.ctx)
	defer monitor.CleanupWorker()

	for {
		select {
		case <-s.ctx.Done():
			log.Info("Aggregation worker shutting down")
			return
		default:
			// Pop from aggregation queue using keyBuilder
			aggregationQueueKey := s.keyBuilder.AggregationQueueLevel1()
			result, err := s.redisClient.BRPop(s.ctx, 5*time.Second, aggregationQueueKey).Result()
			if err != nil {
				if err == redis.Nil {
					continue // Queue empty
				}
				log.Errorf("Aggregation worker: Error popping from queue: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if len(result) < 2 {
				continue
			}

			// Parse aggregation data
			var aggData map[string]interface{}
			if err := json.Unmarshal([]byte(result[1]), &aggData); err != nil {
				log.Errorf("Aggregation worker: Failed to parse data: %v", err)
				continue
			}

			epochIDStr := aggData["epoch_id"].(string)
			epochID, _ := parseEpochID(epochIDStr)
			partsCompleted := int(aggData["parts_completed"].(float64))

			// Update monitoring
			monitor.ProcessingStarted(fmt.Sprintf("epoch:%d", epochID))

			// Aggregate all batch parts
			if err := s.aggregateBatchParts(epochID, partsCompleted); err != nil {
				log.Errorf("Aggregation worker: Failed to aggregate epoch %d: %v", epochID, err)
				monitor.ProcessingFailed(err)
			} else {
				monitor.ProcessingCompleted()
				log.Infof("✅ Aggregation worker: Successfully aggregated epoch %d", epochID)
			}
		}
	}
}

func (s *UnifiedSequencer) aggregateBatchParts(epochID uint64, totalParts int) error {
	ctx := context.Background()

	// Collect all batch parts
	aggregatedResults := make(map[string]interface{})

	for i := 0; i < totalParts; i++ {
		partKey := s.keyBuilder.BatchPart(fmt.Sprintf("%d", epochID), i)
		partData, err := s.redisClient.Get(ctx, partKey).Result()
		if err != nil {
			log.Errorf("Failed to get batch part %d for epoch %d: %v", i, epochID, err)
			continue
		}

		var partResults map[string]interface{}
		if err := json.Unmarshal([]byte(partData), &partResults); err != nil {
			log.Errorf("Failed to parse batch part %d: %v", i, err)
			continue
		}

		// Merge results
		for projectID, data := range partResults {
			aggregatedResults[projectID] = data
		}

		// Clean up part data
		s.redisClient.Del(ctx, partKey)
	}

	// Create finalized batch
	if err := s.createFinalizedBatch(epochID, aggregatedResults); err != nil {
		return fmt.Errorf("failed to create finalized batch: %w", err)
	}

	// Clean up tracking data using keyBuilder
	epochStr := fmt.Sprintf("%d", epochID)
	s.redisClient.Del(ctx,
		s.keyBuilder.EpochPartsCompleted(epochStr),
		s.keyBuilder.EpochPartsTotal(epochStr),
		s.keyBuilder.EpochPartsReady(epochStr),
	)

	log.Infof("📦 Aggregated %d projects from %d batch parts for epoch %d",
		len(aggregatedResults), totalParts, epochID)

	// Log aggregation summary
	if totalParts > 1 {
		log.Debugf("Aggregation details: Combined %d parallel batch parts into single batch", totalParts)
	}

	return nil
}

func (s *UnifiedSequencer) processBatchFinalization(epochID uint64) {
	ctx := context.Background()

	// Get ready batch for this epoch (keeping legacy key format for now)
	protocolState := s.config.ProtocolStateContract
	dataMarket := ""
	if len(s.config.DataMarketAddresses) > 0 {
		dataMarket = s.config.DataMarketAddresses[0]
	}
	batchKey := fmt.Sprintf("%s:%s:batch:ready:%d", protocolState, dataMarket, epochID)
	batchData, err := s.redisClient.Get(ctx, batchKey).Result()
	if err != nil {
		if err == redis.Nil {
			log.Warnf("Finalizer: No ready batch found for epoch %d", epochID)
		} else {
			log.Errorf("Finalizer: Failed to get batch for epoch %d: %v", epochID, err)
		}
		return
	}

	// Parse batch data
	var projectSubmissions map[string]interface{}
	if err := json.Unmarshal([]byte(batchData), &projectSubmissions); err != nil {
		log.Errorf("Finalizer: Failed to parse batch data for epoch %d: %v", epochID, err)
		return
	}

	// Create finalized batch (actual merkle tree and signing)
	if err := s.createFinalizedBatch(epochID, projectSubmissions); err != nil {
		log.Errorf("Finalizer: Failed to create finalized batch for epoch %d: %v", epochID, err)
		return
	}

	// Clean up ready batch
	s.redisClient.Del(ctx, batchKey)

	log.Infof("✅ Finalizer: Successfully finalized batch for epoch %d with %d projects",
		epochID, len(projectSubmissions))
}

func (s *UnifiedSequencer) createFinalizedBatch(epochID uint64, projectSubmissions map[string]interface{}) error {
	ctx := context.Background()

	// Skip empty batches - don't finalize epochs with no actual submissions
	if len(projectSubmissions) == 0 {
		log.Warnf("Skipping finalization for epoch %d - no submissions", epochID)
		return nil
	}

	// Extract project IDs and CIDs
	projectIDs := make([]string, 0)
	snapshotCIDs := make([]string, 0)
	projectVotes := make(map[string]uint32)
	submissionDetails := make(map[string][]submissions.SubmissionMetadata)

	for projectID, submissionData := range projectSubmissions {
		// Handle the new structure with vote metadata and submission details
		if dataMap, ok := submissionData.(map[string]interface{}); ok {
			if cid, ok := dataMap["cid"].(string); ok {
				projectIDs = append(projectIDs, projectID)
				snapshotCIDs = append(snapshotCIDs, cid)

				// Extract vote count
				if votes, ok := dataMap["votes"].(float64); ok {
					projectVotes[projectID] = uint32(votes)
				}

				// Extract submission metadata for challenges/proofs
				if metadata, ok := dataMap["submission_metadata"].([]map[string]interface{}); ok {
					for _, meta := range metadata {
						subMeta := submissions.SubmissionMetadata{
							SubmitterID: meta["submitter_id"].(string),
							SnapshotCID: meta["snapshot_cid"].(string),
							SlotID:      uint64(meta["slot_id"].(float64)),
							Timestamp:   uint64(meta["timestamp"].(float64)),
						}
						if sig, ok := meta["signature"].(string); ok {
							subMeta.Signature = []byte(sig)
						}
						submissionDetails[projectID] = append(submissionDetails[projectID], subMeta)
					}
				}
			}
		} else if cid, ok := submissionData.(string); ok {
			// Fallback for old format (direct CID string)
			projectIDs = append(projectIDs, projectID)
			snapshotCIDs = append(snapshotCIDs, cid)
			projectVotes[projectID] = 1 // Default to 1 vote
		}
	}

	if len(projectIDs) == 0 {
		return fmt.Errorf("no valid projects in batch")
	}

	// TODO: Implement actual merkle tree calculation
	// For now, simple hash concatenation
	combined := ""
	for i := range projectIDs {
		combined += projectIDs[i] + ":" + snapshotCIDs[i] + ","
	}
	hash := sha256.Sum256([]byte(combined))
	merkleRoot := hash[:]

	// TODO: Implement actual BLS signing
	// For now, placeholder signature
	sigData := fmt.Sprintf("%d:%s:%s", epochID, merkleRoot, s.sequencerID)
	sigHash := sha256.Sum256([]byte(sigData))
	blsSignature := sigHash[:]

	finalizedBatch := &consensus.FinalizedBatch{
		EpochId:           epochID,
		ProjectIds:        projectIDs,
		SnapshotCids:      snapshotCIDs,
		MerkleRoot:        merkleRoot,
		BlsSignature:      blsSignature,
		SequencerId:       s.sequencerID,
		Timestamp:         uint64(time.Now().Unix()),
		ProjectVotes:      projectVotes,
		SubmissionDetails: submissionDetails, // Include WHO submitted WHAT
	}

	// Store finalized batch in IPFS if available
	if s.ipfsClient != nil {
		batchCID, err := s.ipfsClient.StoreFinalizedBatch(ctx, finalizedBatch)
		if err != nil {
			log.Errorf("Failed to store batch in IPFS: %v", err)
		} else {
			log.Infof("📦 Stored finalized batch in IPFS: %s", batchCID)

			// Keep legacy P2P consensus for unified mode
			if s.p2pConsensus != nil {
				if err := s.p2pConsensus.BroadcastFinalizedBatch(finalizedBatch, batchCID); err != nil {
					log.Errorf("Failed to broadcast batch to P2P consensus: %v", err)
				} else {
					log.Infof("📡 Broadcasted batch to validator consensus network")
				}
			}
		}
	}

	// Store finalized batch in Redis using keyBuilder
	finalizedKey := s.keyBuilder.FinalizedBatch(fmt.Sprintf("%d", epochID))

	finalizedData, err := json.Marshal(finalizedBatch)
	if err != nil {
		return fmt.Errorf("failed to marshal finalized batch: %w", err)
	}

	// Store with TTL
	if err := s.redisClient.Set(ctx, finalizedKey, finalizedData, 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to store finalized batch: %w", err)
	}

	// Log finalized batch summary for operators
	log.Infof("📦 Created finalized batch for epoch %d: %d projects, merkle=%s",
		epochID, len(projectIDs), hex.EncodeToString(merkleRoot[:8]))

	// Show batch contents summary (limit to first 5 to avoid log spam)
	log.Infof("📊 Finalized Batch Summary - Epoch %d:", epochID)
	for i, projectID := range projectIDs {
		if i >= 5 {
			log.Infof("  ... and %d more projects", len(projectIDs)-5)
			break
		}
		cidPreview := snapshotCIDs[i]
		if len(cidPreview) > 20 {
			cidPreview = cidPreview[:20] + "..."
		}
		voteCount := projectVotes[projectID]
		log.Infof("  • %s → %s (votes: %d)", projectID, cidPreview, voteCount)
	}
	log.Infof("  Full Merkle Root: %s", hex.EncodeToString(merkleRoot))
	log.Infof("  Total Projects: %d", len(projectIDs))

	return nil
}
*/

func (s *UnifiedSequencer) runBatchAggregation() {
	// If P2P batch aggregation is enabled, it handles collection of finalized batches from peers
	if s.p2pConsensus != nil {
		log.Info("P2P Batch Aggregation is running - collecting finalized batches from validators")

		// Monitor consensus status periodically
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check recent epochs for consensus
				currentEpoch := uint64(time.Now().Unix() / 30)
				for i := uint64(0); i < 3; i++ {
					epochID := currentEpoch - i
					status := s.p2pConsensus.GetAggregationStatus(epochID)
					if status != nil && len(status.AggregatedProjects) > 0 {
						log.Infof("✅ Aggregation for epoch %d: %d projects aggregated from %d validators",
							epochID, len(status.AggregatedProjects), status.ReceivedBatches)
					}
				}
			case <-s.ctx.Done():
				log.Info("Batch aggregation component shutting down")
				if s.p2pConsensus != nil {
					s.p2pConsensus.Close()
				}
				return
			}
		}
	} else {
		// Fallback to simple consensus monitoring
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				epoch := uint64(time.Now().Unix() / 30)
				log.Infof("Batch Aggregation: Processing epoch %d locally (P2P broadcast disabled)", epoch)
			case <-s.ctx.Done():
				log.Info("Batch aggregation component shutting down")
				return
			}
		}
	}
}

func (s *UnifiedSequencer) monitorQueueDepth() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			submissionQueueKey := s.keyBuilder.SubmissionQueue()
			length, err := s.redisClient.LLen(s.ctx, submissionQueueKey).Result()
			if err != nil {
				log.Errorf("Failed to get queue length: %v", err)
				continue
			}

			if length > 100 {
				log.Warnf("⚠️ Queue depth high: %d submissions pending", length)
			} else if length > 0 {
				log.Infof("📊 Queue depth: %d submissions", length)
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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
		log.Errorf("Failed to connect to bootstrap node: %v", err)
		return
	}

	log.Infof("✅ Connected to bootstrap node: %s", peerInfo.ID)
}

// isValidDataMarket checks if a data market address is in the configured list
func (s *UnifiedSequencer) isValidDataMarket(dataMarketAddr string) bool {
	if dataMarketAddr == "" {
		return false
	}

	// Normalize to checksummed format for comparison
	checksummedAddr := common.HexToAddress(dataMarketAddr).Hex()

	// Check against configured data markets
	for _, configuredMarket := range s.config.DataMarketAddresses {
		if common.HexToAddress(configuredMarket).Hex() == checksummedAddr {
			return true
		}
	}

	return false
}

