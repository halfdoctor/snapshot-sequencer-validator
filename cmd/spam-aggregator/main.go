package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	rpchelper "github.com/powerloom/go-rpc-helper"
	"github.com/powerloom/snapshot-sequencer-validator/config"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/eventmonitor"
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

	log.Info("✅ P2P operations handled via p2p-gateway (Redis queue-based)")

	// Initialize spam protection components (no P2P - uses Redis queues)
	spamComponents, err := spam.InitializeSpamProtection(ctx, cfg, redisClient, keyBuilder, nil, sequencerID)
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
}
