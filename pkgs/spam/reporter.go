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
	// Skip reporting if peer is whitelisted, EXCEPT for snapshotter address violations (rate_limit_snapshotter)
	// Snapshotter address violations can be reported even if the peer ID is whitelisted (for bulk service peers)
	if violationType != "rate_limit_snapshotter" {
		if r.whitelist != nil && r.whitelist.IsWhitelisted(peerID) {
			return nil
		}
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

	// Log differently for snapshotter address reports vs peer ID reports
	if violationType == "rate_limit_snapshotter" {
		log.WithFields(log.Fields{
			"peer_id":          peerID,
			"snapshotter_addr": snapshotterAddr,
			"violation_type":   violationType,
			"epoch_id":         epochID,
			"count":            count,
			"reporter_id":      r.reporterID,
		}).Infof("📝 Stored snapshotter address spam report for bulk service peer %s snapshotter %s epoch %d (will flag snapshotter address only after consensus)", peerID, snapshotterAddr, epochID)
	} else {
		log.WithFields(log.Fields{
			"peer_id":          peerID,
			"snapshotter_addr": snapshotterAddr,
			"violation_type":   violationType,
			"epoch_id":         epochID,
			"count":            count,
			"reporter_id":      r.reporterID,
		}).Infof("📝 Stored peer ID spam report for peer %s epoch %d (will flag peer ID and associated snapshotter addresses after consensus)", peerID, epochID)
	}

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
	// Check if peer is whitelisted
	isWhitelisted := false
	isBulkService := false
	if r.whitelist != nil {
		isWhitelisted = r.whitelist.IsWhitelisted(peerID)
		if isWhitelisted {
			isBulkService = r.whitelist.IsBulkService(peerID)
		}
	}

	// For bulk service peers: Check snapshotter address violations instead of peer ID violations
	if isBulkService && snapshotterAddr != "" {
		shouldReport, violationType, err := r.tracker.ShouldReportSpamForSnapshotter(ctx, snapshotterAddr, epochID)
		if err != nil {
			return fmt.Errorf("failed to check spam thresholds for snapshotter: %w", err)
		}

		if !shouldReport {
			log.Debugf("Spam check for bulk service peer %s snapshotter %s epoch %d: shouldReport=false (thresholds not met)", peerID, snapshotterAddr, epochID)
			return nil
		}

		log.Infof("⚠️ Spam check for bulk service peer %s snapshotter %s epoch %d: shouldReport=true, violationType=%s (snapshotter address will be flagged after consensus)", peerID, snapshotterAddr, epochID, violationType)

		// Get counts for evidence
		count, err := r.tracker.GetSnapshotterSubmissionCount(ctx, snapshotterAddr, epochID)
		if err != nil {
			return fmt.Errorf("failed to get snapshotter submission count: %w", err)
		}

		// Include consecutive epochs information in evidence
		consecutiveViolations, err := r.tracker.CheckConsecutiveSnapshotterRateLimitViolations(ctx, snapshotterAddr, epochID)
		if err != nil {
			log.Warnf("Failed to get consecutive violations count: %v", err)
			consecutiveViolations = 1 // Fallback to 1 if check fails
		}
		evidence := []string{
			fmt.Sprintf("submissions: %d (limit: %d)", count, MAX_SUBMISSIONS_PER_EPOCH_LITE),
			fmt.Sprintf("consecutive_epochs_with_violations: %d (threshold: %d)", consecutiveViolations, CONSISTENT_VIOLATIONS_THRESHOLD),
		}

		// Report spam (will be stored in Redis and sent after collection window)
		// Note: For bulk service peers, the report is keyed by snapshotter address
		// The peerID is included for context but the violation is tracked per snapshotter address
		return r.ReportSpam(ctx, peerID, snapshotterAddr, epochID, violationType, count, evidence)
	}

	// Skip reporting if peer is whitelisted (full node)
	if isWhitelisted {
		return nil
	}

	// Regular peers: Check peer ID violations
	shouldReport, violationType, err := r.tracker.ShouldReportSpam(ctx, peerID, epochID)
	if err != nil {
		return fmt.Errorf("failed to check spam thresholds: %w", err)
	}

	if !shouldReport {
		log.Debugf("Spam check for peer %s epoch %d: shouldReport=false (thresholds not met)", peerID, epochID)
		return nil
	}

	// Get counts for evidence
	var count int
	var evidence []string
	var reportReason string

	switch violationType {
	case "validation_failure":
		count, err = r.tracker.GetValidationFailureCount(ctx, peerID, epochID)
		if err != nil {
			return fmt.Errorf("failed to get validation failure count: %w", err)
		}
		// Check if it's immediate or consecutive
		isImmediate := count >= MAX_VALIDATION_FAILURES_PER_EPOCH
		if isImmediate {
			reportReason = fmt.Sprintf("immediate (failures: %d >= threshold: %d)", count, MAX_VALIDATION_FAILURES_PER_EPOCH)
			evidence = []string{fmt.Sprintf("validation_failures: %d (immediate threshold: %d)", count, MAX_VALIDATION_FAILURES_PER_EPOCH)}
		} else {
			// Consecutive violations
			consecutiveFailures, err := r.tracker.CheckConsecutiveValidationFailures(ctx, peerID, epochID)
			if err != nil {
				log.Warnf("Failed to get consecutive failures count: %v", err)
				consecutiveFailures = 1 // Fallback to 1 if check fails
			}
			reportReason = fmt.Sprintf("consecutive (failures: %d >= threshold: %d for %d consecutive epochs >= threshold: %d)", count, MAX_VALIDATION_FAILURES_PER_EPOCH_CONSECUTIVE, consecutiveFailures, CONSISTENT_VIOLATIONS_THRESHOLD)
			evidence = []string{
				fmt.Sprintf("validation_failures: %d (consecutive threshold: %d)", count, MAX_VALIDATION_FAILURES_PER_EPOCH_CONSECUTIVE),
				fmt.Sprintf("consecutive_epochs_with_violations: %d (threshold: %d)", consecutiveFailures, CONSISTENT_VIOLATIONS_THRESHOLD),
			}
		}
		log.Infof("⚠️ Spam check for peer %s epoch %d: shouldReport=true, violationType=%s, reason=%s (peer ID and associated snapshotter addresses will be flagged after consensus)", peerID, epochID, violationType, reportReason)
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
