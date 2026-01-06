package spam

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
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

// SpamReporter broadcasts spam reports to the validator mesh
type SpamReporter struct {
	topic      *pubsub.Topic
	tracker    *SpamTracker
	whitelist  *PeerWhitelist
	reporterID string
}

// NewSpamReporter creates a new SpamReporter instance
func NewSpamReporter(topic *pubsub.Topic, tracker *SpamTracker, whitelist *PeerWhitelist, reporterID string) *SpamReporter {
	return &SpamReporter{
		topic:      topic,
		tracker:    tracker,
		whitelist:  whitelist,
		reporterID: reporterID,
	}
}

// ReportSpam creates and broadcasts a spam report if thresholds are exceeded
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

	// Marshal report
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal spam report: %w", err)
	}

	// Broadcast to validator mesh
	if err := r.topic.Publish(ctx, data); err != nil {
		return fmt.Errorf("failed to publish spam report: %w", err)
	}

	log.WithFields(log.Fields{
		"peer_id":          peerID,
		"snapshotter_addr": snapshotterAddr,
		"violation_type":   violationType,
		"epoch_id":         epochID,
		"count":            count,
		"reporter_id":      r.reporterID,
	}).Infof("📢 Broadcasted spam report for peer %s (violation: %s, count: %d)", peerID, violationType, count)

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
		return nil
	}

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
		consecutiveViolations, err := r.tracker.checkConsecutiveRateLimitViolations(ctx, peerID, epochID)
		if err != nil {
			log.Warnf("Failed to get consecutive violations count: %v", err)
			consecutiveViolations = 1 // Fallback to 1 if check fails
		}
		evidence = []string{
			fmt.Sprintf("submissions: %d (limit: %d)", count, MAX_SUBMISSIONS_PER_EPOCH_LITE),
			fmt.Sprintf("consecutive_epochs_with_violations: %d (threshold: %d)", consecutiveViolations, CONSISTENT_VIOLATIONS_THRESHOLD),
		}
	}

	// Report spam
	return r.ReportSpam(ctx, peerID, snapshotterAddr, epochID, violationType, count, evidence)
}
