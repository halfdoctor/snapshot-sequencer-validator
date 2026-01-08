package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/powerloom/snapshot-sequencer-validator/config"
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

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down spam aggregator component")
	cancel()
}
