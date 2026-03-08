package spam

import (
	"context"
	"time"

	"github.com/powerloom/snapshot-sequencer-validator/config"
	redislib "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// InitializeSpamProtection initializes spam protection components
// All P2P operations are handled via Redis queues with p2p-gateway
func InitializeSpamProtection(ctx context.Context, cfg *config.Settings, redisClient *redis.Client, keyBuilder *redislib.KeyBuilder, ps interface{}, sequencerID string) (*SpamComponents, error) {
	if !cfg.EnableSpamProtection {
		log.Info("Spam protection disabled via ENABLE_SPAM_PROTECTION=false")
		return nil, nil
	}

	log.WithFields(log.Fields{
		"enable_spam_protection":       cfg.EnableSpamProtection,
		"enable_spam_report_broadcast": cfg.EnableSpamReportBroadcast,
	}).Info("Initializing spam protection components (Redis queue-based P2P)")

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

	// Initialize spam aggregator (always needed for local aggregation and Redis queue processing)
	// Window size is hardcoded to 10 for consensus consistency across all validators
	aggregator := NewSpamAggregator(ctx, redisClient, keyBuilder, whitelist, flagging, DEFAULT_AGGREGATION_WINDOW_SIZE)
	// Set sequencer ID for generating local reports
	aggregator.SetSequencerID(sequencerID)
	// Start aggregator (handles Redis queue reading and periodic pruning)
	aggregator.Start()
	log.Infof("Initialized spam aggregator with window size: %d (Redis queue-based P2P)", DEFAULT_AGGREGATION_WINDOW_SIZE)

	// Initialize spam report window manager (for batching reports per epoch)
	var windowManager *SpamReportWindowManager
	if cfg.EnableSpamReportBroadcast {
		collectionWindowDuration := cfg.SpamReportCollectionWindow
		if collectionWindowDuration == 0 {
			// Default: level1_delay + 10 seconds
			collectionWindowDuration = cfg.Level1FinalizationDelay + 10*time.Second
		}
		consensusDelayDuration := cfg.SpamReportConsensusDelay
		if consensusDelayDuration == 0 {
			// Default: 10 seconds
			consensusDelayDuration = 10 * time.Second
		}
		windowManager = NewSpamReportWindowManager(ctx, redisClient, keyBuilder, collectionWindowDuration, consensusDelayDuration, aggregator)
		log.Infof("Initialized spam report window manager (collection window: %v, consensus delay: %v)", collectionWindowDuration, consensusDelayDuration)
	}

	// Initialize spam reporter (if broadcast enabled)
	// Pass aggregator so it can inject reports directly (Gossipsub doesn't deliver self-messages)
	// Pass window manager for batching reports per epoch
	var reporter *SpamReporter
	if cfg.EnableSpamReportBroadcast {
		reporter = NewSpamReporterWithAggregator(redisClient, keyBuilder, tracker, whitelist, sequencerID, aggregator, windowManager)
		log.Infof("Initialized spam reporter (Redis queue-based broadcasting with epoch batching)")
		// Set reporter in aggregator so it can store generated reports
		aggregator.SetReporter(reporter)
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

	log.WithFields(log.Fields{
		"tracker_initialized":      tracker != nil,
		"rate_limiter_initialized": rateLimiter != nil,
		"flagging_initialized":     flagging != nil,
		"reporter_initialized":     reporter != nil,
		"aggregator_initialized":   aggregator != nil,
	}).Info("✅ Spam protection components initialized")
	return &SpamComponents{
		Tracker:     tracker,
		RateLimiter: rateLimiter,
		Flagging:    flagging,
		Reporter:    reporter,
		Aggregator:  aggregator,
	}, nil
}

// SpamComponents holds spam protection components for dependency injection
type SpamComponents struct {
	Tracker     *SpamTracker
	RateLimiter *RateLimiter
	Flagging    *FlaggingService
	Reporter    *SpamReporter
	Aggregator  *SpamAggregator
}

// GetAggregator returns the spam aggregator (for use in other packages)
func (sc *SpamComponents) GetAggregator() *SpamAggregator {
	return sc.Aggregator
}

// GetTracker returns the spam tracker (for use in other packages)
func (sc *SpamComponents) GetTracker() *SpamTracker {
	return sc.Tracker
}
