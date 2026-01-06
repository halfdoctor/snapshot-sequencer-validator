package spam

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/powerloom/snapshot-sequencer-validator/config"
	redislib "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// InitializeSpamProtection initializes all spam protection components
func InitializeSpamProtection(ctx context.Context, cfg *config.Settings, redisClient *redis.Client, keyBuilder *redislib.KeyBuilder, ps *pubsub.PubSub, sequencerID string) (*SpamComponents, error) {
	if !cfg.EnableSpamProtection {
		log.Info("Spam protection disabled via ENABLE_SPAM_PROTECTION=false")
		return nil, nil
	}

	// Initialize whitelist
	whitelist := NewPeerWhitelist(cfg.FullNodePeerIDs, cfg.BulkServicePeerIDs)
	log.Infof("Initialized peer whitelist: %d full nodes, %d bulk service snapshotters",
		len(cfg.FullNodePeerIDs), len(cfg.BulkServicePeerIDs))

	// Initialize spam tracker
	tracker := NewSpamTracker(redisClient, keyBuilder, whitelist)

	// Initialize rate limiter
	rateLimiter := NewRateLimiter(tracker, whitelist)

	// Initialize flagging service
	flagging := NewFlaggingService(redisClient, keyBuilder, whitelist)

	// Initialize spam reporter (if broadcast enabled)
	var reporter *SpamReporter
	if cfg.EnableSpamReportBroadcast && ps != nil {
		// Get spam report topic (constructed from validator presence prefix + "/spam-reports")
		spamReportTopic := cfg.GetSpamReportTopic()
		spamTopic, err := ps.Join(spamReportTopic)
		if err != nil {
			return nil, fmt.Errorf("failed to join spam reports topic: %w", err)
		}
		reporter = NewSpamReporter(spamTopic, tracker, whitelist, sequencerID)
		log.Infof("Initialized spam reporter for topic: %s", spamReportTopic)
	}

	// Initialize spam aggregator (if broadcast enabled)
	var aggregator *SpamAggregator
	if cfg.EnableSpamReportBroadcast && ps != nil {
		// Get spam report topic (constructed from validator presence prefix + "/spam-reports")
		spamReportTopic := cfg.GetSpamReportTopic()
		spamTopic, err := ps.Join(spamReportTopic)
		if err != nil {
			return nil, fmt.Errorf("failed to join spam reports topic for aggregator: %w", err)
		}
		sub, err := spamTopic.Subscribe()
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to spam reports topic: %w", err)
		}
		// Window size is hardcoded to 10 for consensus consistency across all validators
		aggregator = NewSpamAggregator(ctx, redisClient, keyBuilder, whitelist, sub, flagging, DEFAULT_AGGREGATION_WINDOW_SIZE)
		aggregator.Start()
		log.Infof("Initialized spam aggregator with window size: %d", DEFAULT_AGGREGATION_WINDOW_SIZE)
	}

	// TODO: Initialize state sync service (if enabled)
	// This will sync flagged state from on-chain contract to Redis cache
	if cfg.SpamSyncOnStartup {
		log.Info("State sync on startup enabled (not yet implemented)")
		// TODO: Implement state sync service
		// stateSync := NewStateSync(flagging, time.Duration(cfg.SpamSyncIntervalHours)*time.Hour)
		// if err := stateSync.SyncOnStartup(ctx); err != nil {
		// 	log.Warnf("Failed to sync flagged state on startup: %v", err)
		// }
		// if cfg.SpamSyncIntervalHours > 0 {
		// 	go stateSync.StartPeriodicSync(ctx)
		// 	log.Infof("Started periodic state sync (interval: %d hours)", cfg.SpamSyncIntervalHours)
		// }
	}

	log.Info("✅ Spam protection components initialized")
	return &SpamComponents{
		Tracker:     tracker,
		RateLimiter: rateLimiter,
		Flagging:    flagging,
		Reporter:    reporter,
	}, nil
}

// SpamComponents holds spam protection components for dependency injection
type SpamComponents struct {
	Tracker     *SpamTracker
	RateLimiter *RateLimiter
	Flagging    *FlaggingService
	Reporter    *SpamReporter
}
