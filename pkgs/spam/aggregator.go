package spam

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

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

// Lua script for atomic update of aggregated report
const updateAggregatedReportScript = `
-- KEYS[1]: windowKey (e.g., "{protocol}:{market}:spam:reports:peer:{peerID}:window:{windowID}")
-- ARGV[1]: report JSON (string)
-- ARGV[2]: reporter ID (string)
-- ARGV[3]: snapshotter address (string, can be empty)
-- ARGV[4]: epoch ID (number)
-- ARGV[5]: timestamp (number)
-- ARGV[6]: TTL seconds (number, e.g., 7200 for 2 hours)

local windowKey = KEYS[1]
local reportJson = ARGV[1]
local reporterId = ARGV[2]
local snapshotterAddr = ARGV[3]
local epochId = tonumber(ARGV[4])
local timestamp = tonumber(ARGV[5])
local ttl = tonumber(ARGV[6])

-- Get existing aggregated report or create new one
local existing = redis.call('GET', windowKey)
local aggregated

if existing then
    aggregated = cjson.decode(existing)
else
    aggregated = {
        reports = {},
        validator_ids = {},
        validator_count = 0,
        snapshotter_addrs = {},
        first_epoch = 0,
        last_epoch = 0,
        first_seen = 0,
        last_updated = 0
    }
end

-- Parse new report
local newReport = cjson.decode(reportJson)

-- Append report
table.insert(aggregated.reports, newReport)

-- Add validator ID if not present
local validatorExists = false
for i, vid in ipairs(aggregated.validator_ids) do
    if vid == reporterId then
        validatorExists = true
        break
    end
end
if not validatorExists then
    table.insert(aggregated.validator_ids, reporterId)
    aggregated.validator_count = #aggregated.validator_ids
end

-- Add snapshotter address if not present
if snapshotterAddr ~= '' then
    local addrExists = false
    for i, addr in ipairs(aggregated.snapshotter_addrs) do
        if addr == snapshotterAddr then
            addrExists = true
            break
        end
    end
    if not addrExists then
        table.insert(aggregated.snapshotter_addrs, snapshotterAddr)
    end
end

-- Update epoch range
if aggregated.first_epoch == 0 or epochId < aggregated.first_epoch then
    aggregated.first_epoch = epochId
end
if epochId > aggregated.last_epoch then
    aggregated.last_epoch = epochId
end

-- Update timestamps
if aggregated.first_seen == 0 then
    aggregated.first_seen = timestamp
end
aggregated.last_updated = timestamp

-- Store back with TTL
local updatedJson = cjson.encode(aggregated)
redis.call('SET', windowKey, updatedJson, 'EX', ttl)

-- Return validator count
return aggregated.validator_count
`

// SpamAggregator receives and aggregates spam reports from other validators
type SpamAggregator struct {
	ctx                   context.Context
	redisClient           *redis.Client
	keyBuilder            *redislib.KeyBuilder
	whitelist             *PeerWhitelist
	windowSize            int
	flagging              *FlaggingService
	sequencerID           string        // Validator/sequencer ID for generating local reports
	updateReportScriptSha string        // SHA1 hash of Lua script for atomic updates
	reporter              *SpamReporter // Reporter for storing generated reports (optional)
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
func NewSpamAggregator(ctx context.Context, redisClient *redis.Client, keyBuilder *redislib.KeyBuilder, whitelist *PeerWhitelist, flagging *FlaggingService, windowSize int) *SpamAggregator {
	if windowSize <= 0 {
		windowSize = DEFAULT_AGGREGATION_WINDOW_SIZE
	}
	agg := &SpamAggregator{
		ctx:         ctx,
		redisClient: redisClient,
		keyBuilder:  keyBuilder,
		whitelist:   whitelist,
		windowSize:  windowSize,
		flagging:    flagging,
		sequencerID: "", // Will be set via SetSequencerID if needed
	}

	// Load Lua scripts on initialization
	if err := agg.loadLuaScripts(ctx); err != nil {
		log.Errorf("Failed to load Lua scripts: %v", err)
		// Continue anyway - will use fallback method
	}

	return agg
}

// SetSequencerID sets the sequencer ID for generating local reports
func (a *SpamAggregator) SetSequencerID(sequencerID string) {
	a.sequencerID = sequencerID
}

// SetReporter sets the reporter for storing generated reports
func (a *SpamAggregator) SetReporter(reporter *SpamReporter) {
	a.reporter = reporter
}

// loadLuaScripts loads Lua scripts into Redis and stores their SHA hashes
func (a *SpamAggregator) loadLuaScripts(ctx context.Context) error {
	sha, err := a.redisClient.ScriptLoad(ctx, updateAggregatedReportScript).Result()
	if err != nil {
		return fmt.Errorf("failed to load update aggregated report script: %w", err)
	}
	a.updateReportScriptSha = sha
	log.Infof("Loaded update aggregated report Lua script SHA: %s", sha)
	return nil
}

// Start begins listening for spam reports and aggregating them
func (a *SpamAggregator) Start() {
	// Start Redis queue reader for incoming reports
	go a.handleIncomingSpamReports()
	// Always start periodic consensus check (handles pruning)
	go a.periodicConsensusCheck()
}

// handleIncomingSpamReports reads spam reports from Redis queue
func (a *SpamAggregator) handleIncomingSpamReports() {
	queueKey := a.keyBuilder.IncomingSpamReports()
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			result, err := a.redisClient.BRPop(a.ctx, time.Second, queueKey).Result()
			if err != nil {
				if err == redis.Nil {
					// Timeout - continue
					continue
				}
				if a.ctx.Err() != nil {
					return
				}
				log.Errorf("Error reading from spam reports queue: %v", err)
				continue
			}

			if len(result) >= 2 {
				// Parse report for logging before processing
				var report SpamReport
				if err := json.Unmarshal([]byte(result[1]), &report); err == nil {
					log.WithFields(log.Fields{
						"peer_id":        report.PeerID,
						"epoch_id":       report.EpochID,
						"violation_type": report.ViolationType,
						"count":          report.Count,
						"reporter_id":    report.ReporterID,
					}).Infof("📥 Received spam report from queue: peer=%s epoch=%d violation=%s count=%d reporter=%s", report.PeerID, report.EpochID, report.ViolationType, report.Count, report.ReporterID)
				}

				// Process result[1] as spam report JSON
				go a.processSpamReportDirect([]byte(result[1]))
			}
		}
	}
}

// processSpamReportDirect processes a single spam report (can be called directly or from Redis queue)
// This processes reports IMMEDIATELY when received - there is no collection window delay for incoming reports.
// The collection window (LEVEL1_FINALIZATION_DELAY_SECONDS + 10 seconds) only applies to when WE send OUR reports.
// Incoming reports from other validators are processed immediately into aggregation windows.
func (a *SpamAggregator) processSpamReportDirect(data []byte) {
	var report SpamReport
	if err := json.Unmarshal(data, &report); err != nil {
		log.Errorf("Failed to unmarshal spam report: %v", err)
		return
	}

	// Log receipt of report from another validator (only if reporter ID differs from our own)
	if report.ReporterID != a.sequencerID {
		log.WithFields(log.Fields{
			"peer_id":     report.PeerID,
			"epoch_id":    report.EpochID,
			"reporter_id": report.ReporterID,
			"violation":   report.ViolationType,
		}).Infof("📨 Received spam report from validator %s for peer %s epoch %d", report.ReporterID, report.PeerID, report.EpochID)
	}

	// Skip if peer is whitelisted
	if a.whitelist != nil && a.whitelist.IsWhitelisted(report.PeerID) {
		log.Debugf("Skipping spam report for whitelisted peer: %s", report.PeerID)
		return
	}

	// Calculate aggregation window ID (end epoch of the window)
	windowID := ((int(report.EpochID) + a.windowSize - 1) / a.windowSize) * a.windowSize
	windowKey := a.getAggregationWindowKey(report.PeerID, windowID)

	// Use Lua script for atomic update
	reportJson, err := json.Marshal(report)
	if err != nil {
		log.Errorf("Failed to marshal spam report: %v", err)
		return
	}

	ttlSeconds := int(AGGREGATION_TTL.Seconds())
	result, err := a.redisClient.EvalSha(a.ctx, a.updateReportScriptSha,
		[]string{windowKey},
		reportJson, report.ReporterID, report.SnapshotterAddr,
		report.EpochID, report.Timestamp, ttlSeconds).Result()

	if err != nil {
		// Handle NOSCRIPT error (script not in Redis cache)
		if strings.Contains(err.Error(), "NOSCRIPT") {
			log.Warnf("NOSCRIPT error, reloading Lua script")
			if reloadErr := a.loadLuaScripts(a.ctx); reloadErr != nil {
				log.Errorf("Failed to reload Lua script: %v", reloadErr)
				// Fallback to non-atomic method
				a.processSpamReportDirectFallback(data)
				return
			}
			// Retry with reloaded script
			result, err = a.redisClient.EvalSha(a.ctx, a.updateReportScriptSha,
				[]string{windowKey},
				reportJson, report.ReporterID, report.SnapshotterAddr,
				report.EpochID, report.Timestamp, ttlSeconds).Result()
		}

		if err != nil {
			log.Errorf("Failed to update aggregated report atomically: %v", err)
			// Fallback to non-atomic method
			a.processSpamReportDirectFallback(data)
			return
		}
	}

	validatorCount, _ := result.(int64)

	// Add peer to window peers set
	windowPeersKey := a.getWindowPeersKey(windowID)
	if err := a.redisClient.SAdd(a.ctx, windowPeersKey, report.PeerID).Err(); err != nil {
		log.Errorf("Failed to add peer to window peers set: %v", err)
	} else {
		// Set TTL on the set
		if err := a.redisClient.Expire(a.ctx, windowPeersKey, AGGREGATION_TTL).Err(); err != nil {
			log.Warnf("Failed to set TTL on window peers set: %v", err)
		}
	}

	// Add window ID to master windows set
	windowsSetKey := a.getWindowsSetKey()
	windowIDStr := fmt.Sprintf("%d", windowID)
	if err := a.redisClient.SAdd(a.ctx, windowsSetKey, windowIDStr).Err(); err != nil {
		log.Errorf("Failed to add window to master set: %v", err)
	}

	log.WithFields(log.Fields{
		"peer_id":         report.PeerID,
		"epoch_id":        report.EpochID,
		"window_id":       windowID,
		"validator_count": validatorCount,
		"reporter_id":     report.ReporterID,
	}).Infof("Atomically aggregated spam report for peer %s (window %d, validators: %d)", report.PeerID, windowID, validatorCount)
}

// processSpamReportDirectFallback is the fallback non-atomic implementation
func (a *SpamAggregator) processSpamReportDirectFallback(data []byte) {
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

	// Add peer to window peers set
	windowPeersKey := a.getWindowPeersKey(windowID)
	if err := a.redisClient.SAdd(a.ctx, windowPeersKey, report.PeerID).Err(); err != nil {
		log.Errorf("Failed to add peer to window peers set: %v", err)
	} else {
		if err := a.redisClient.Expire(a.ctx, windowPeersKey, AGGREGATION_TTL).Err(); err != nil {
			log.Warnf("Failed to set TTL on window peers set: %v", err)
		}
	}

	// Add window ID to master windows set
	windowsSetKey := a.getWindowsSetKey()
	windowIDStr := fmt.Sprintf("%d", windowID)
	if err := a.redisClient.SAdd(a.ctx, windowsSetKey, windowIDStr).Err(); err != nil {
		log.Errorf("Failed to add window to master set: %v", err)
	}

	log.WithFields(log.Fields{
		"peer_id":         report.PeerID,
		"epoch_id":        report.EpochID,
		"window_id":       windowID,
		"validator_count": aggregated.ValidatorCount,
		"reporter_id":     report.ReporterID,
	}).Infof("Aggregated spam report for peer %s (window %d, validators: %d) [fallback]", report.PeerID, windowID, aggregated.ValidatorCount)
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

			// Prune old epoch peer sets (fallback cleanup - they should be deleted after aggregation)
			if err := a.pruneOldEpochPeerSets(a.ctx); err != nil {
				log.Warnf("Failed to prune old epoch peer sets: %v", err)
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

// pruneOldEpochPeerSets removes epoch peer sets that are older than the oldest active window
// This is a fallback cleanup - epoch peer sets should be deleted immediately after aggregation
func (a *SpamAggregator) pruneOldEpochPeerSets(ctx context.Context) error {
	// Get all windows to find the oldest active window
	windowsSetKey := a.getWindowsSetKey()
	windowIDs, err := a.redisClient.SMembers(ctx, windowsSetKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get windows set: %w", err)
	}

	if len(windowIDs) == 0 {
		return nil // No windows, nothing to prune
	}

	// Find the oldest window ID
	oldestWindowID := -1
	for _, windowIDStr := range windowIDs {
		windowID, err := strconv.Atoi(windowIDStr)
		if err != nil {
			continue
		}
		if oldestWindowID == -1 || windowID < oldestWindowID {
			oldestWindowID = windowID
		}
	}

	if oldestWindowID == -1 {
		return nil
	}

	// Calculate the oldest epoch we need to keep (oldest window start - 1 for safety margin)
	oldestEpochToKeep := uint64(oldestWindowID - a.windowSize - 1)

	// Delete epoch peer sets older than oldestEpochToKeep
	// We check a reasonable range (e.g., up to 100 epochs back) to avoid checking too many
	maxEpochsToCheck := uint64(100)
	prunedCount := 0
	for epoch := uint64(1); epoch < oldestEpochToKeep && epoch < maxEpochsToCheck; epoch++ {
		// Construct epoch peers key directly (same format as tracker)
		epochPeersKey := fmt.Sprintf("%s:%s:spam:epoch:%d:peers", a.keyBuilder.ProtocolState, a.keyBuilder.DataMarket, epoch)
		exists, err := a.redisClient.Exists(ctx, epochPeersKey).Result()
		if err != nil {
			continue
		}
		if exists > 0 {
			if err := a.redisClient.Del(ctx, epochPeersKey).Err(); err != nil {
				log.Warnf("Failed to delete old epoch peers set for epoch %d: %v", epoch, err)
			} else {
				prunedCount++
			}
		}
	}

	if prunedCount > 0 {
		log.Debugf("Pruned %d old epoch peer sets (older than epoch %d)", prunedCount, oldestEpochToKeep)
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

// CreateWindowAndAggregateLocalData creates a window at epoch boundary and aggregates all local tracking data
// This is called when epochID % windowSize == 0 (e.g., epochs 10, 20, 30, etc.)
// It scans all local tracking data from the last 10 epochs and aggregates it into the window
func (a *SpamAggregator) CreateWindowAndAggregateLocalData(ctx context.Context, epochID uint64, tracker *SpamTracker) error {
	// Calculate window ID (end epoch of the window)
	windowID := ((int(epochID) + a.windowSize - 1) / a.windowSize) * a.windowSize

	// Window start epoch (inclusive)
	windowStartEpoch := uint64(windowID - a.windowSize + 1)
	// Window end epoch (inclusive)
	windowEndEpoch := uint64(windowID)

	log.WithFields(log.Fields{
		"epoch_id":     epochID,
		"window_id":    windowID,
		"window_start": windowStartEpoch,
		"window_end":   windowEndEpoch,
	}).Infof("Creating spam aggregation window %d for epochs %d-%d", windowID, windowStartEpoch, windowEndEpoch)

	// Ensure window is added to master set
	windowsSetKey := a.getWindowsSetKey()
	windowIDStr := fmt.Sprintf("%d", windowID)
	if err := a.redisClient.SAdd(ctx, windowsSetKey, windowIDStr).Err(); err != nil {
		log.Errorf("Failed to add window to master set: %v", err)
	}

	// Get window peers set key
	windowPeersKey := a.getWindowPeersKey(windowID)

	// Collect all peer IDs from epoch peer sets (deterministic, no SCAN)
	peersInWindow := make(map[string]bool)

	// Get peer IDs from each epoch's peer set
	for epoch := windowStartEpoch; epoch <= windowEndEpoch; epoch++ {
		epochPeersKey := tracker.GetEpochPeersKey(epoch)
		peerIDs, err := a.redisClient.SMembers(ctx, epochPeersKey).Result()
		if err != nil {
			if err != redis.Nil {
				log.Warnf("Failed to get epoch peers set for epoch %d: %v", epoch, err)
			}
			continue
		}
		for _, peerID := range peerIDs {
			peersInWindow[peerID] = true
		}
	}

	// For each peer found, aggregate their data into the window
	for peerID := range peersInWindow {
		// Skip whitelisted peers
		if a.whitelist != nil && a.whitelist.IsWhitelisted(peerID) {
			continue
		}

		// Get or create aggregated report for this peer in this window
		windowKey := a.getAggregationWindowKey(peerID, windowID)
		aggregated, err := a.getOrCreateAggregatedReport(windowKey, peerID)
		if err != nil {
			log.Errorf("Failed to get/create aggregated report for peer %s: %v", peerID, err)
			continue
		}

		// Aggregate local tracking data from all epochs in the window
		firstEpoch := uint64(0)
		lastEpoch := uint64(0)
		totalSubmissions := 0
		totalValidationFailures := 0
		snapshotterAddrs := make(map[string]bool)
		localReportsGenerated := false

		for epoch := windowStartEpoch; epoch <= windowEndEpoch; epoch++ {
			// Get submission count
			submissionCount, err := tracker.GetSubmissionCount(ctx, peerID, epoch)
			if err != nil {
				log.Warnf("Failed to get submission count for peer %s epoch %d: %v", peerID, epoch, err)
			} else if submissionCount > 0 {
				totalSubmissions += submissionCount
				if firstEpoch == 0 {
					firstEpoch = epoch
				}
				lastEpoch = epoch

				// Get snapshotter addresses for this epoch
				assocKey := tracker.GetPeerSnapshotterMapKey(peerID, epoch)
				addrs, err := a.redisClient.SMembers(ctx, assocKey).Result()
				if err == nil {
					for _, addr := range addrs {
						snapshotterAddrs[addr] = true
					}
				}
			}

			// Get validation failure count
			failureCount, err := tracker.GetValidationFailureCount(ctx, peerID, epoch)
			if err != nil {
				log.Warnf("Failed to get validation failure count for peer %s epoch %d: %v", peerID, epoch, err)
			} else if failureCount > 0 {
				totalValidationFailures += failureCount
				if firstEpoch == 0 {
					firstEpoch = epoch
				}
				if epoch > lastEpoch {
					lastEpoch = epoch
				}
			}

			// Generate spam report for this epoch if thresholds are met
			shouldReport, violationType, err := tracker.ShouldReportSpam(ctx, peerID, epoch)
			if err != nil {
				log.Warnf("Failed to check if spam should be reported for peer %s epoch %d: %v", peerID, epoch, err)
				continue
			}

			if shouldReport && a.sequencerID != "" {
				// Get snapshotter address for this epoch (use first one found)
				snapshotterAddr := ""
				if len(snapshotterAddrs) > 0 {
					// Get first snapshotter address
					for addr := range snapshotterAddrs {
						snapshotterAddr = addr
						break
					}
				}

				// Create spam report
				var count int
				var evidence []string
				switch violationType {
				case "validation_failure":
					count = failureCount
					evidence = []string{fmt.Sprintf("validation_failures: %d", count)}
				case "rate_limit":
					count = submissionCount
					consecutiveViolations, err := tracker.CheckConsecutiveRateLimitViolations(ctx, peerID, epoch)
					if err != nil {
						log.Warnf("Failed to get consecutive violations count: %v", err)
						consecutiveViolations = 1
					}
					evidence = []string{
						fmt.Sprintf("submissions: %d (limit: %d)", count, MAX_SUBMISSIONS_PER_EPOCH_LITE),
						fmt.Sprintf("consecutive_epochs_with_violations: %d (threshold: %d)", consecutiveViolations, CONSISTENT_VIOLATIONS_THRESHOLD),
					}
				default:
					count = 0
					evidence = []string{}
				}

				if count > 0 {
					report := SpamReport{
						PeerID:          peerID,
						SnapshotterAddr: snapshotterAddr,
						ViolationType:   violationType,
						EpochID:         epoch,
						Count:           count,
						Evidence:        evidence,
						ReporterID:      a.sequencerID,
						Timestamp:       time.Now().Unix(),
					}

					aggregated.Reports = append(aggregated.Reports, report)
					localReportsGenerated = true

					// Add validator ID if not already present
					validatorExists := false
					for _, vid := range aggregated.ValidatorIDs {
						if vid == a.sequencerID {
							validatorExists = true
							break
						}
					}
					if !validatorExists {
						aggregated.ValidatorIDs = append(aggregated.ValidatorIDs, a.sequencerID)
						aggregated.ValidatorCount = len(aggregated.ValidatorIDs)
					}

					log.WithFields(log.Fields{
						"peer_id":        peerID,
						"epoch_id":       epoch,
						"violation_type": violationType,
						"count":          count,
					}).Debugf("Generated local spam report for peer %s epoch %d (will be sent via collection window)", peerID, epoch)

					// Note: Reports are stored in Redis by SpamReporter and sent after collection window
					// No immediate broadcasting here - batching happens per epoch via SpamReportWindowManager
				}
			}
		}

		// Update aggregated report with local tracking data
		if firstEpoch > 0 {
			if aggregated.FirstEpoch == 0 || firstEpoch < aggregated.FirstEpoch {
				aggregated.FirstEpoch = firstEpoch
			}
			if lastEpoch > aggregated.LastEpoch {
				aggregated.LastEpoch = lastEpoch
			}
		}

		// Add snapshotter addresses
		for addr := range snapshotterAddrs {
			addrExists := false
			for _, existingAddr := range aggregated.SnapshotterAddrs {
				if existingAddr == addr {
					addrExists = true
					break
				}
			}
			if !addrExists {
				aggregated.SnapshotterAddrs = append(aggregated.SnapshotterAddrs, addr)
			}
		}

		// Update timestamps if reports were generated
		if localReportsGenerated {
			if aggregated.FirstSeen == 0 {
				aggregated.FirstSeen = time.Now().Unix()
			}
			aggregated.LastUpdated = time.Now().Unix()
		}

		// Store aggregated report
		if err := a.storeAggregatedReport(windowKey, aggregated); err != nil {
			log.Errorf("Failed to store aggregated report for peer %s: %v", peerID, err)
			continue
		}

		// Add peer to window peers set
		if err := a.redisClient.SAdd(ctx, windowPeersKey, peerID).Err(); err != nil {
			log.Errorf("Failed to add peer to window peers set: %v", err)
		} else {
			// Set TTL on the set
			if err := a.redisClient.Expire(ctx, windowPeersKey, AGGREGATION_TTL).Err(); err != nil {
				log.Warnf("Failed to set TTL on window peers set: %v", err)
			}
		}

		log.WithFields(log.Fields{
			"peer_id":                   peerID,
			"window_id":                 windowID,
			"total_submissions":         totalSubmissions,
			"total_validation_failures": totalValidationFailures,
			"epoch_range":               fmt.Sprintf("%d-%d", firstEpoch, lastEpoch),
		}).Debugf("Aggregated local tracking data for peer %s in window %d", peerID, windowID)
	}

	log.WithFields(log.Fields{
		"window_id":   windowID,
		"peers_count": len(peersInWindow),
		"epoch_range": fmt.Sprintf("%d-%d", windowStartEpoch, windowEndEpoch),
	}).Infof("Created spam aggregation window %d with %d peers", windowID, len(peersInWindow))

	// Clean up epoch peer sets after aggregation (they've been aggregated into the window)
	// This prevents accumulation of old epoch peer sets
	for epoch := windowStartEpoch; epoch <= windowEndEpoch; epoch++ {
		epochPeersKey := tracker.GetEpochPeersKey(epoch)
		if err := a.redisClient.Del(ctx, epochPeersKey).Err(); err != nil {
			log.Warnf("Failed to delete epoch peers set for epoch %d: %v", epoch, err)
		}
	}

	return nil
}
