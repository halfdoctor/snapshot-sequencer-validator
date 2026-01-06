package spam

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	redislib "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

const (
	// Default aggregation window size (epochID % windowSize)
	DEFAULT_AGGREGATION_WINDOW_SIZE = 10

	// Default TTL for aggregation keys (2 hours)
	AGGREGATION_TTL = 2 * time.Hour
)

// SpamAggregator receives and aggregates spam reports from other validators
type SpamAggregator struct {
	ctx         context.Context
	redisClient *redis.Client
	keyBuilder  *redislib.KeyBuilder
	whitelist   *PeerWhitelist
	windowSize  int
	sub         *pubsub.Subscription
	flagging    *FlaggingService
}

// AggregatedReport represents aggregated spam reports for a peer in a window
type AggregatedReport struct {
	PeerID           string       `json:"peer_id"`
	Reports          []SpamReport `json:"reports"`
	ValidatorIDs     []string     `json:"validator_ids"`
	ValidatorCount   int          `json:"validator_count"`
	SnapshotterAddrs []string     `json:"snapshotter_addrs"`
	FirstEpoch       uint64       `json:"first_epoch"`
	LastEpoch        uint64       `json:"last_epoch"`
	FirstSeen        int64        `json:"first_seen"`
	LastUpdated      int64        `json:"last_updated"`
}

// NewSpamAggregator creates a new SpamAggregator instance
func NewSpamAggregator(ctx context.Context, redisClient *redis.Client, keyBuilder *redislib.KeyBuilder, whitelist *PeerWhitelist, sub *pubsub.Subscription, flagging *FlaggingService, windowSize int) *SpamAggregator {
	if windowSize <= 0 {
		windowSize = DEFAULT_AGGREGATION_WINDOW_SIZE
	}
	return &SpamAggregator{
		ctx:         ctx,
		redisClient: redisClient,
		keyBuilder:  keyBuilder,
		whitelist:   whitelist,
		windowSize:  windowSize,
		sub:         sub,
		flagging:    flagging,
	}
}

// Start begins listening for spam reports and aggregating them
func (a *SpamAggregator) Start() {
	go a.handleSpamReports()
	go a.periodicConsensusCheck()
}

// handleSpamReports processes incoming spam reports from other validators
func (a *SpamAggregator) handleSpamReports() {
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			msg, err := a.sub.Next(a.ctx)
			if err != nil {
				if a.ctx.Err() != nil {
					return
				}
				log.Errorf("Error reading spam report message: %v", err)
				continue
			}

			// Process spam report
			go a.processSpamReport(msg.Data)
		}
	}
}

// processSpamReport processes a single spam report
func (a *SpamAggregator) processSpamReport(data []byte) {
	var report SpamReport
	if err := json.Unmarshal(data, &report); err != nil {
		log.Errorf("Failed to unmarshal spam report: %v", err)
		return
	}

	// Skip if peer is whitelisted
	if a.whitelist != nil && a.whitelist.IsWhitelisted(report.PeerID) {
		log.Debugf("Skipping spam report for whitelisted peer: %s", report.PeerID)
		return
	}

	// Calculate aggregation window ID (end epoch of the window)
	// Window ID = round up to next multiple of windowSize
	// Note: Epoch 0 is dummy/heartbeat only, so windows start from epoch 1
	// Epochs 1-10 → Window 10, Epochs 11-20 → Window 20, etc.
	// Simple formula: round up epochID to next multiple of windowSize
	windowID := ((int(report.EpochID) + a.windowSize - 1) / a.windowSize) * a.windowSize
	windowKey := a.getAggregationWindowKey(report.PeerID, windowID)

	// Get or create aggregated report
	aggregated, err := a.getOrCreateAggregatedReport(windowKey, report.PeerID)
	if err != nil {
		log.Errorf("Failed to get/create aggregated report: %v", err)
		return
	}

	// Add report to aggregation
	aggregated.Reports = append(aggregated.Reports, report)

	// Add validator ID if not already present
	validatorExists := false
	for _, vid := range aggregated.ValidatorIDs {
		if vid == report.ReporterID {
			validatorExists = true
			break
		}
	}
	if !validatorExists {
		aggregated.ValidatorIDs = append(aggregated.ValidatorIDs, report.ReporterID)
		aggregated.ValidatorCount = len(aggregated.ValidatorIDs)
	}

	// Add snapshotter address if not already present
	if report.SnapshotterAddr != "" {
		addrExists := false
		for _, addr := range aggregated.SnapshotterAddrs {
			if addr == report.SnapshotterAddr {
				addrExists = true
				break
			}
		}
		if !addrExists {
			aggregated.SnapshotterAddrs = append(aggregated.SnapshotterAddrs, report.SnapshotterAddr)
		}
	}

	// Update epoch range
	if aggregated.FirstEpoch == 0 || report.EpochID < aggregated.FirstEpoch {
		aggregated.FirstEpoch = report.EpochID
	}
	if report.EpochID > aggregated.LastEpoch {
		aggregated.LastEpoch = report.EpochID
	}

	// Update timestamps
	if aggregated.FirstSeen == 0 {
		aggregated.FirstSeen = report.Timestamp
	}
	aggregated.LastUpdated = report.Timestamp

	// Store aggregated report
	if err := a.storeAggregatedReport(windowKey, aggregated); err != nil {
		log.Errorf("Failed to store aggregated report: %v", err)
		return
	}

	// Add peer ID to window peers set
	windowPeersKey := a.getWindowPeersKey(windowID)
	if err := a.redisClient.SAdd(a.ctx, windowPeersKey, report.PeerID).Err(); err != nil {
		log.Errorf("Failed to add peer to window peers set: %v", err)
		// Continue - non-critical, but log error
	} else {
		// Set TTL on the set (same as aggregation TTL)
		if err := a.redisClient.Expire(a.ctx, windowPeersKey, AGGREGATION_TTL).Err(); err != nil {
			log.Warnf("Failed to set TTL on window peers set: %v", err)
		}
	}

	// Add window ID to master windows set (for discovery/indexing)
	windowsSetKey := a.getWindowsSetKey()
	windowIDStr := fmt.Sprintf("%d", windowID)
	if err := a.redisClient.SAdd(a.ctx, windowsSetKey, windowIDStr).Err(); err != nil {
		log.Errorf("Failed to add window to master set: %v", err)
		// Continue - non-critical, but log error
	}

	log.WithFields(log.Fields{
		"peer_id":         report.PeerID,
		"epoch_id":        report.EpochID,
		"window_id":       windowID,
		"validator_count": aggregated.ValidatorCount,
		"reporter_id":     report.ReporterID,
	}).Debugf("Aggregated spam report for peer %s (window %d, validators: %d)", report.PeerID, windowID, aggregated.ValidatorCount)
}

// CheckWindowForConsensus checks a specific window for consensus and flags if reached
// CRITICAL: This is ONLY called when epochID % windowSize == 0 (end of window boundary)
// During the window (epochs 1-10, 11-20, etc.), reports are collected but NO consensus checking happens
// Only at the boundary does consensus checking and flagging (blacklisting) occur
func (a *SpamAggregator) CheckWindowForConsensus(ctx context.Context, currentEpochID uint64) error {
	// The window that just closed (window ID = end epoch)
	// Note: Epoch 0 is dummy/heartbeat only, so windows start from epoch 1
	// At epoch 10, window 10 (epochs 1-10) is complete → check consensus → flag if threshold reached
	// At epoch 20, window 20 (epochs 11-20) is complete → check consensus → flag if threshold reached
	windowID := int(currentEpochID)

	// Get all peer IDs with reports in this window
	windowPeersKey := a.getWindowPeersKey(windowID)
	peerIDs, err := a.redisClient.SMembers(ctx, windowPeersKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get window peers set: %w", err)
	}

	if len(peerIDs) == 0 {
		log.Debugf("No peers with reports in window %d", windowID)
		return nil
	}

	// For each peer with reports in the window, check consensus
	for _, peerID := range peerIDs {
		// Get aggregated report for this peer in this window
		windowKey := a.getAggregationWindowKey(peerID, windowID)
		aggregated, err := a.getOrCreateAggregatedReport(windowKey, peerID)
		if err != nil {
			log.Errorf("Failed to get aggregated report for key %s: %v", windowKey, err)
			continue
		}

		// Skip if peer is whitelisted
		if a.whitelist != nil && a.whitelist.IsWhitelisted(peerID) {
			log.Debugf("Skipping consensus check for whitelisted peer: %s", peerID)
			continue
		}

		// Check if consensus threshold reached
		if aggregated.ValidatorCount >= SPAM_CONSENSUS_THRESHOLD {
			// Flag on-chain
			if err := a.flagging.FlagPeer(ctx, peerID, aggregated.SnapshotterAddrs, aggregated.FirstEpoch, aggregated.LastEpoch); err != nil {
				log.Errorf("Failed to flag peer %s: %v", peerID, err)
				continue
			}

			log.WithFields(log.Fields{
				"peer_id":           peerID,
				"window_id":         windowID,
				"validator_count":   aggregated.ValidatorCount,
				"snapshotter_addrs": aggregated.SnapshotterAddrs,
				"first_epoch":       aggregated.FirstEpoch,
				"last_epoch":        aggregated.LastEpoch,
			}).Infof("🚩 Consensus reached for peer %s (validators: %d)", peerID, aggregated.ValidatorCount)
		}
	}

	return nil
}

// getWindowPeersKey returns the Redis key for the set of peer IDs in a window
// Format: {protocol}:{market}:spam:reports:window:{windowID}:peers
func (a *SpamAggregator) getWindowPeersKey(windowID int) string {
	return fmt.Sprintf("%s:%s:spam:reports:window:%d:peers", a.keyBuilder.ProtocolState, a.keyBuilder.DataMarket, windowID)
}

// getWindowsSetKey returns the Redis key for the master set of all windows with reports
// Format: {protocol}:{market}:spam:reports:windows
func (a *SpamAggregator) getWindowsSetKey() string {
	return fmt.Sprintf("%s:%s:spam:reports:windows", a.keyBuilder.ProtocolState, a.keyBuilder.DataMarket)
}

// periodicConsensusCheck periodically checks for consensus on flagged peers
// This is called periodically, but actual consensus checking should be triggered
// when epochID % windowSize == 0 (handled by the component that tracks epochs)
func (a *SpamAggregator) periodicConsensusCheck() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			// Note: Actual consensus checking should be triggered by epoch transitions
			// This periodic check is a fallback safety mechanism
			log.Debug("Periodic consensus check (fallback - epoch-based checking preferred)")

			// Prune expired windows from master set
			if err := a.pruneExpiredWindows(a.ctx); err != nil {
				log.Warnf("Failed to prune expired windows: %v", err)
			}
		}
	}
}

// pruneExpiredWindows removes expired windows from the master windows set
// Windows expire after AGGREGATION_TTL (2 hours) - check if window peers set still exists
func (a *SpamAggregator) pruneExpiredWindows(ctx context.Context) error {
	windowsSetKey := a.getWindowsSetKey()
	windowIDs, err := a.redisClient.SMembers(ctx, windowsSetKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get windows set: %w", err)
	}

	expiredWindows := make([]string, 0)
	for _, windowIDStr := range windowIDs {
		windowID, err := strconv.Atoi(windowIDStr)
		if err != nil {
			continue
		}

		// Check if window peers set still exists (if not, window has expired)
		windowPeersKey := a.getWindowPeersKey(windowID)
		exists, err := a.redisClient.Exists(ctx, windowPeersKey).Result()
		if err != nil {
			log.Warnf("Failed to check window %s existence: %v", windowIDStr, err)
			continue
		}

		if exists == 0 {
			// Window has expired - remove from master set
			expiredWindows = append(expiredWindows, windowIDStr)
		}
	}

	// Remove expired windows from master set
	if len(expiredWindows) > 0 {
		if err := a.redisClient.SRem(ctx, windowsSetKey, expiredWindows).Err(); err != nil {
			return fmt.Errorf("failed to remove expired windows: %w", err)
		}
		log.Debugf("Pruned %d expired windows from master set", len(expiredWindows))
	}

	return nil
}

// getOrCreateAggregatedReport gets an existing aggregated report or creates a new one
func (a *SpamAggregator) getOrCreateAggregatedReport(windowKey string, peerID string) (*AggregatedReport, error) {
	data, err := a.redisClient.Get(a.ctx, windowKey).Result()
	if err == redis.Nil {
		// Create new aggregated report
		return &AggregatedReport{
			PeerID:           peerID,
			Reports:          []SpamReport{},
			ValidatorIDs:     []string{},
			SnapshotterAddrs: []string{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get aggregated report: %w", err)
	}

	var aggregated AggregatedReport
	if err := json.Unmarshal([]byte(data), &aggregated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal aggregated report: %w", err)
	}

	// Ensure peer ID is set
	if aggregated.PeerID == "" {
		aggregated.PeerID = peerID
	}

	return &aggregated, nil
}

// storeAggregatedReport stores an aggregated report in Redis
func (a *SpamAggregator) storeAggregatedReport(windowKey string, aggregated *AggregatedReport) error {
	data, err := json.Marshal(aggregated)
	if err != nil {
		return fmt.Errorf("failed to marshal aggregated report: %w", err)
	}

	if err := a.redisClient.Set(a.ctx, windowKey, data, AGGREGATION_TTL).Err(); err != nil {
		return fmt.Errorf("failed to store aggregated report: %w", err)
	}

	return nil
}

// getAggregationWindowKey returns the Redis key for an aggregation window
func (a *SpamAggregator) getAggregationWindowKey(peerID string, windowID int) string {
	return fmt.Sprintf("%s:%s:spam:reports:peer:%s:window:%d", a.keyBuilder.ProtocolState, a.keyBuilder.DataMarket, peerID, windowID)
}
