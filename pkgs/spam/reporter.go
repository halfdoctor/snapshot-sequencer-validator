package spam

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	redislib "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// SpamReport represents a spam report sent between validators
// Sent on dedicated spam report topic (constructed from validator presence prefix + "/spam-reports")
type SpamReport struct {
	PeerID          string   `json:"peer_id"`          // PRIMARY: libp2p peer ID
	SnapshotterAddr string   `json:"snapshotter_addr"` // Secondary: Ethereum address (may be empty if signature invalid)
	ViolationType   string   `json:"violation_type"`   // "validation_failure", "rate_limit", "slot_mismatch"
	EpochID         uint64   `json:"epoch_id"`
	Count           int      `json:"count"`
	Evidence        []string `json:"evidence"`    // Submission IDs, error messages
	ReporterID      string   `json:"reporter_id"` // Validator ID reporting this
	Timestamp       int64    `json:"timestamp"`
}

// SpamReporter broadcasts spam reports to the validator mesh via Redis queue
type SpamReporter struct {
	redisClient   *redis.Client
	keyBuilder    *redislib.KeyBuilder
	tracker       *SpamTracker
	whitelist     *PeerWhitelist
	reporterID    string
	aggregator    *SpamAggregator
	windowManager *SpamReportWindowManager
}

// NewSpamReporter creates a new SpamReporter instance
func NewSpamReporter(redisClient *redis.Client, keyBuilder *redislib.KeyBuilder, tracker *SpamTracker, whitelist *PeerWhitelist, reporterID string, windowManager *SpamReportWindowManager) *SpamReporter {
	return &SpamReporter{
		redisClient:   redisClient,
		keyBuilder:    keyBuilder,
		tracker:       tracker,
		whitelist:     whitelist,
		reporterID:    reporterID,
		windowManager: windowManager,
	}
}

// NewSpamReporterWithAggregator creates a new SpamReporter with aggregator for direct injection
func NewSpamReporterWithAggregator(redisClient *redis.Client, keyBuilder *redislib.KeyBuilder, tracker *SpamTracker, whitelist *PeerWhitelist, reporterID string, aggregator *SpamAggregator, windowManager *SpamReportWindowManager) *SpamReporter {
	return &SpamReporter{
		redisClient:   redisClient,
		keyBuilder:    keyBuilder,
		tracker:       tracker,
		whitelist:     whitelist,
		reporterID:    reporterID,
		aggregator:    aggregator,
		windowManager: windowManager,
	}
}

// ReportSpam stores a spam report in Redis for batching (will be sent after collection window)
func (r *SpamReporter) ReportSpam(ctx context.Context, peerID, snapshotterAddr string, epochID uint64, violationType string, count int, evidence []string) error {
	// Skip reporting if peer is whitelisted
	if r.whitelist != nil && r.whitelist.IsWhitelisted(peerID) {
		return nil
	}

	// Create spam report
	report := &SpamReport{
		PeerID:          peerID,
		SnapshotterAddr: snapshotterAddr,
		ViolationType:   violationType,
		EpochID:         epochID,
		Count:           count,
		Evidence:        evidence,
		ReporterID:      r.reporterID,
		Timestamp:       time.Now().Unix(),
	}

	// Store report in Redis for batching (will be sent after collection window)
	if err := r.storePendingReport(ctx, epochID, report); err != nil {
		return fmt.Errorf("failed to store pending spam report: %w", err)
	}

	log.WithFields(log.Fields{
		"peer_id":          peerID,
		"snapshotter_addr": snapshotterAddr,
		"violation_type":   violationType,
		"epoch_id":         epochID,
		"count":            count,
		"reporter_id":      r.reporterID,
	}).Debugf("Stored spam report for peer %s epoch %d (will be sent after collection window)", peerID, epochID)

	return nil
}

// storePendingReport stores a report in Redis LIST for deterministic collection
func (r *SpamReporter) storePendingReport(ctx context.Context, epochID uint64, report *SpamReport) error {
	// Marshal report
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal spam report: %w", err)
	}

	// Get data market from key builder (needed for Redis key)
	// Note: The key builder may have a default data market, but reports are per-epoch
	// We need to get the data market from the key builder's current state
	dataMarket := r.keyBuilder.DataMarket
	if dataMarket == "" {
		// If data market is not set in key builder, we can't store the report
		// This should not happen in normal operation, but log a warning
		log.Warnf("Data market not set in key builder, cannot store spam report for epoch %d", epochID)
		return fmt.Errorf("data market not set in key builder")
	}

	// Store in Redis LIST (deterministic collection, like submissions)
	// Use the same key format as window manager
	pendingKey := fmt.Sprintf("%s:%s:spam:reports:pending:epoch:%d", r.keyBuilder.ProtocolState, dataMarket, epochID)
	if err := r.redisClient.LPush(ctx, pendingKey, data).Err(); err != nil {
		return fmt.Errorf("failed to store pending report in Redis: %w", err)
	}

	// Set TTL on the key (2 hours, matches aggregation window TTL)
	if err := r.redisClient.Expire(ctx, pendingKey, 2*time.Hour).Err(); err != nil {
		log.Debugf("Failed to set TTL on pending reports key: %v", err)
	}

	return nil
}

// CheckAndReport checks thresholds and reports if exceeded
func (r *SpamReporter) CheckAndReport(ctx context.Context, peerID, snapshotterAddr string, epochID uint64) error {
	// Skip reporting if peer is whitelisted
	if r.whitelist != nil && r.whitelist.IsWhitelisted(peerID) {
		return nil
	}

	// Check if spam should be reported
	shouldReport, violationType, err := r.tracker.ShouldReportSpam(ctx, peerID, epochID)
	if err != nil {
		return fmt.Errorf("failed to check spam thresholds: %w", err)
	}

	if !shouldReport {
		log.Debugf("Spam check for peer %s epoch %d: shouldReport=false (thresholds not met)", peerID, epochID)
		return nil
	}

	log.Infof("Spam check for peer %s epoch %d: shouldReport=true, violationType=%s", peerID, epochID, violationType)

	// Get counts for evidence
	var count int
	var evidence []string

	switch violationType {
	case "validation_failure":
		count, err = r.tracker.GetValidationFailureCount(ctx, peerID, epochID)
		if err != nil {
			return fmt.Errorf("failed to get validation failure count: %w", err)
		}
		evidence = []string{fmt.Sprintf("validation_failures: %d", count)}
	case "rate_limit":
		count, err = r.tracker.GetSubmissionCount(ctx, peerID, epochID)
		if err != nil {
			return fmt.Errorf("failed to get submission count: %w", err)
		}
		// Include consecutive epochs information in evidence
		consecutiveViolations, err := r.tracker.CheckConsecutiveRateLimitViolations(ctx, peerID, epochID)
		if err != nil {
			log.Warnf("Failed to get consecutive violations count: %v", err)
			consecutiveViolations = 1 // Fallback to 1 if check fails
		}
		evidence = []string{
			fmt.Sprintf("submissions: %d (limit: %d)", count, MAX_SUBMISSIONS_PER_EPOCH_LITE),
			fmt.Sprintf("consecutive_epochs_with_violations: %d (threshold: %d)", consecutiveViolations, CONSISTENT_VIOLATIONS_THRESHOLD),
		}
	}

	// Report spam (will be stored in Redis and sent after collection window)
	return r.ReportSpam(ctx, peerID, snapshotterAddr, epochID, violationType, count, evidence)
}

// GetWindowManager returns the window manager (for use by event monitor)
func (r *SpamReporter) GetWindowManager() *SpamReportWindowManager {
	return r.windowManager
}
