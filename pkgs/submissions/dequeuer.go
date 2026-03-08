package submissions

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	customcrypto "github.com/powerloom/snapshot-sequencer-validator/pkgs/crypto"
	redislib "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/spam"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// Dequeuer handles processing of queued submissions
type Dequeuer struct {
	redisClient           *redis.Client
	keyBuilder            *redislib.KeyBuilder
	sequencerID           string
	eip712Verifier        *customcrypto.EIP712Verifier
	slotValidator         *SlotValidator
	enableSlotValidation  bool
	processedSubmissions  map[string]*ProcessedSubmission
	submissionsMutex      sync.RWMutex
	stats                 DequeuerStats
	statsMutex            sync.RWMutex
	protocolStateContract string // Protocol state contract address for Redis key namespacing

	// Spam protection components
	spamTracker          *spam.SpamTracker
	rateLimiter          *spam.RateLimiter
	flagging             *spam.FlaggingService
	spamReporter         *spam.SpamReporter
	enableSpamProtection bool
}

// DequeuerStats tracks processing metrics
type DequeuerStats struct {
	TotalProcessed  uint64
	SuccessfulCount uint64
	FailedCount     uint64
	LastProcessedAt time.Time
	ProcessingRate  float64 // submissions per second
}

// NewDequeuer creates a new submission dequeuer
// snapshotterStateAddr must be provided if enableSlotValidation is true
// slotManager is optional - if provided, enables on-demand slot fetching during validation
func NewDequeuer(redisClient *redis.Client, keyBuilder *redislib.KeyBuilder, sequencerID string, chainID int64, protocolStateContract string, snapshotterStateAddr common.Address, enableSlotValidation bool, spamComponents *spam.SpamComponents, slotManager *SlotManager) (*Dequeuer, error) {
	if enableSlotValidation && snapshotterStateAddr == (common.Address{}) {
		return nil, fmt.Errorf("snapshotterStateAddr is required when slot validation is enabled")
	}
	verifier, err := customcrypto.NewEIP712Verifier(chainID, protocolStateContract)
	if err != nil {
		return nil, fmt.Errorf("failed to create EIP-712 verifier: %w", err)
	}

	protocolStateAddr := common.HexToAddress(protocolStateContract)
	slotValidator := NewSlotValidator(redisClient, protocolStateAddr, snapshotterStateAddr, slotManager)

	d := &Dequeuer{
		redisClient:           redisClient,
		keyBuilder:            keyBuilder,
		sequencerID:           sequencerID,
		eip712Verifier:        verifier,
		slotValidator:         slotValidator,
		enableSlotValidation:  enableSlotValidation,
		processedSubmissions:  make(map[string]*ProcessedSubmission),
		protocolStateContract: protocolStateContract, // Store for Redis key namespacing
	}

	// Initialize spam protection components if provided
	if spamComponents != nil {
		d.spamTracker = spamComponents.Tracker
		d.rateLimiter = spamComponents.RateLimiter
		d.flagging = spamComponents.Flagging
		d.spamReporter = spamComponents.Reporter
		d.enableSpamProtection = true
	}

	return d, nil
}

// ProcessSubmission validates and stores a submission
// Returns error and snapshotter address (empty if signature verification failed)
func (d *Dequeuer) ProcessSubmission(submission *SnapshotSubmission, submissionID string, metaData map[string]interface{}) (string, error) {
	startTime := time.Now()

	// Check for duplicate
	d.submissionsMutex.RLock()
	if processed, exists := d.processedSubmissions[submissionID]; exists {
		d.submissionsMutex.RUnlock()
		log.Debugf("Submission %s already processed", submissionID)
		return processed.SnapshotterAddr, nil
	}
	d.submissionsMutex.RUnlock()

	// Extract peer ID from metadata
	var peerID string
	if metaData != nil {
		if p, ok := metaData["peer_id"].(string); ok {
			peerID = p
		}
	}

	ctx := context.Background()

	// Spam protection: Check flagged peers and snapshotters (if enabled)
	if d.enableSpamProtection && d.flagging != nil {
		// Check if peer is flagged
		if peerID != "" {
			flagged, err := d.flagging.IsPeerFlagged(ctx, peerID)
			if err != nil {
				log.Warnf("Failed to check if peer is flagged: %v", err)
			} else if flagged {
				d.updateStats(false, time.Since(startTime))
				log.Warnf("Rejected submission from flagged peer: %s", peerID)
				return "", fmt.Errorf("peer is flagged: %s", peerID)
			}
		}
	}

	// Validate submission
	if err := d.validateSubmission(submission); err != nil {
		// Track validation failure for spam protection
		if d.enableSpamProtection && d.spamTracker != nil && peerID != "" {
			if err := d.spamTracker.TrackValidationFailure(ctx, peerID, "", submission.Request.EpochId, err); err != nil {
				log.Warnf("Failed to track validation failure: %v", err)
			}
		}
		d.updateStats(false, time.Since(startTime))
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// Verify signature and extract snapshotter address
	snapshotterAddr, err := d.verifySignature(submission)
	if err != nil {
		// Track validation failure for spam protection
		if d.enableSpamProtection && d.spamTracker != nil && peerID != "" {
			if err := d.spamTracker.TrackValidationFailure(ctx, peerID, "", submission.Request.EpochId, err); err != nil {
				log.Warnf("Failed to track validation failure: %v", err)
			}
		}
		d.updateStats(false, time.Since(startTime))
		return "", fmt.Errorf("signature verification failed: %w", err)
	}

	// Detect and cache simulation messages (epoch 0 with real CID)
	// Simulation messages are sent by snapshotters at startup to verify connectivity
	// Unlike heartbeats (epoch 0 + empty CID), simulations contain real data and are EIP-712 signed
	if submission.Request.EpochId == 0 && submission.Request.SnapshotCid != "" {
		log.Infof("📡 Detected simulation message from peer %s, snapshotter %s (epoch=0, cid=%s)",
			peerID, snapshotterAddr.Hex(), submission.Request.SnapshotCid)

		// Cache the simulation for monitoring purposes
		if err := d.CacheSimulation(peerID, snapshotterAddr, submission); err != nil {
			log.Warnf("Failed to cache simulation message: %v", err)
			// Don't fail the submission - caching is non-critical
		}

		// Update stats for simulation
		d.updateStats(true, time.Since(startTime))

		// Return early - simulation messages don't need further processing (slot validation, spam tracking, etc.)
		// They're just for startup connectivity verification
		return snapshotterAddr.Hex(), nil
	}

	// Spam protection: Check flagged snapshotter address (if enabled)
	if d.enableSpamProtection && d.flagging != nil && snapshotterAddr != (common.Address{}) {
		flagged, err := d.flagging.IsSnapshotterFlagged(ctx, snapshotterAddr.Hex())
		if err != nil {
			log.Warnf("Failed to check if snapshotter is flagged: %v", err)
		} else if flagged {
			d.updateStats(false, time.Since(startTime))
			log.Warnf("Rejected submission from flagged snapshotter: %s", snapshotterAddr.Hex())
			return snapshotterAddr.Hex(), fmt.Errorf("snapshotter is flagged: %s", snapshotterAddr.Hex())
		}
	}

	// Validate snapshotter address against slot registration (if enabled)
	if d.enableSlotValidation && snapshotterAddr != (common.Address{}) {
		if err := d.slotValidator.ValidateSnapshotterForSlot(submission.Request.SlotId, snapshotterAddr); err != nil {
			// Track validation failure for spam protection
			if d.enableSpamProtection && d.spamTracker != nil && peerID != "" {
				if err := d.spamTracker.TrackValidationFailure(ctx, peerID, snapshotterAddr.Hex(), submission.Request.EpochId, err); err != nil {
					log.Warnf("Failed to track validation failure: %v", err)
				}
			}
			d.updateStats(false, time.Since(startTime))
			log.Errorf("Slot validation failed for submission (epoch=%d, slot=%d, signer=%s): %v",
				submission.Request.EpochId, submission.Request.SlotId, snapshotterAddr.Hex(), err)
			return snapshotterAddr.Hex(), fmt.Errorf("slot validation failed: %w", err)
		}
		log.Debugf("Slot validation passed: slot %d is registered to %s",
			submission.Request.SlotId, snapshotterAddr.Hex())
	}

	// Spam protection: Track submission count (if enabled)
	// NOTE: We track FIRST, then check if spam should be reported
	// Rate limiting is NOT enforced here - only flagged peers (after consensus) are rejected
	if d.enableSpamProtection {
		if d.spamTracker == nil {
			log.Debugf("Spam tracker is nil (spam protection enabled but tracker not initialized)")
		} else if peerID == "" {
			log.Debugf("Peer ID is empty - skipping spam tracking for epoch %d", submission.Request.EpochId)
		} else {
			_, err := d.spamTracker.TrackSubmissionCount(ctx, peerID, snapshotterAddr.Hex(), submission.Request.EpochId)
			if err != nil {
				log.Warnf("Failed to track submission count: %v", err)
			} else {
				// Check if spam should be reported (checks consecutive violations threshold)
				if d.spamReporter != nil {
					if err := d.spamReporter.CheckAndReport(ctx, peerID, snapshotterAddr.Hex(), submission.Request.EpochId); err != nil {
						log.Warnf("Failed to check/report spam: %v", err)
					}
				}
			}
		}
	} else {
		log.Debugf("Spam protection disabled - skipping tracking for epoch %d", submission.Request.EpochId)
	}

	// Store in local state
	processed := &ProcessedSubmission{
		ID:              submissionID,
		Submission:      submission,
		SnapshotterAddr: snapshotterAddr.Hex(),
		DataMarketAddr:  submission.DataMarket,
		ProcessedAt:     time.Now(),
		ValidatorID:     d.sequencerID,
		MetaData:        metaData,
	}

	d.submissionsMutex.Lock()
	d.processedSubmissions[submissionID] = processed

	// Clean up old submissions (keep last 1000)
	if len(d.processedSubmissions) > 1000 {
		d.cleanupOldSubmissions()
	}
	d.submissionsMutex.Unlock()

	// Store processing result in Redis for coordination
	d.storeProcessingResult(submissionID, processed)

	// Update stats
	d.updateStats(true, time.Since(startTime))

	log.Debugf("Successfully processed submission %s for epoch %d, slot %d",
		submissionID, submission.Request.EpochId, submission.Request.SlotId)

	// Get peer ID from submission metadata if available (already extracted above for spam protection)
	if processed.MetaData != nil && peerID == "" {
		if p, ok := processed.MetaData["peer_id"].(string); ok {
			peerID = p
		}
	}

	// Add enhanced submission timeline entry with detailed context
	timestamp := time.Now().Unix()
	enhancedSubmissionID := fmt.Sprintf("received:%d:%d:%s:%d:%s",
		submission.Request.EpochId,
		submission.Request.SlotId,
		submission.Request.ProjectId,
		timestamp,
		peerID)

	// Pipeline for monitoring metrics
	pipe := d.redisClient.Pipeline()

	// Store in submissions timeline with enhanced format
	pipe.ZAdd(context.Background(), d.keyBuilder.MetricsSubmissionsTimeline(), redis.Z{
		Score:  float64(timestamp),
		Member: enhancedSubmissionID,
	})

	// Store submission metadata for enhanced monitoring
	metadata := map[string]interface{}{
		"epoch_id":   submission.Request.EpochId,
		"slot_id":    submission.Request.SlotId,
		"project_id": submission.Request.ProjectId,
		"timestamp":  timestamp,
		"cid":        submission.Request.SnapshotCid,
		"peer_id":    peerID,
		"entity_id":  enhancedSubmissionID,
	}

	metadataJSON, _ := json.Marshal(metadata)
	metadataKey := d.keyBuilder.MetricsSubmissionsMetadata(enhancedSubmissionID)
	pipe.SetEx(context.Background(), metadataKey, metadataJSON, 24*time.Hour)

	log.Infof("📝 Enhanced submission timeline entry: %s (epoch=%d, slot=%d, project=%s)",
		enhancedSubmissionID, submission.Request.EpochId, submission.Request.SlotId, submission.Request.ProjectId)

	// Add monitoring metrics for validated submission
	hour := time.Now().Format("2006010215") // YYYYMMDDHH format

	// Continue with existing pipeline operations...

	// 1. Add to validations timeline (sorted set, no TTL - pruned daily)
	pipe.ZAdd(context.Background(), d.keyBuilder.MetricsValidationsTimeline(), redis.Z{
		Score:  float64(timestamp),
		Member: submissionID,
	})

	// 2. Store validation details with TTL (1 hour)
	validationData := map[string]interface{}{
		"submission_id": submissionID,
		"epoch_id":      submission.Request.EpochId,
		"project_id":    submission.Request.ProjectId,
		"validator_id":  d.sequencerID,
		"snapshotter":   snapshotterAddr.Hex(),
		"timestamp":     timestamp,
		"data_market":   submission.DataMarket,
	}
	jsonData, _ := json.Marshal(validationData)
	pipe.SetEx(context.Background(), fmt.Sprintf("metrics:validation:%s", submissionID), string(jsonData), time.Hour)

	// 3. Add to epoch validated set with TTL
	epochValidatedKey := fmt.Sprintf("metrics:epoch:%d:validated", submission.Request.EpochId)
	pipe.SAdd(context.Background(), epochValidatedKey, submissionID)
	pipe.Expire(context.Background(), epochValidatedKey, 2*time.Hour)

	// 4. Update hourly counter
	pipe.HIncrBy(context.Background(), fmt.Sprintf("metrics:hourly:%s:validations", hour), "total", 1)
	pipe.Expire(context.Background(), fmt.Sprintf("metrics:hourly:%s:validations", hour), 2*time.Hour)

	// 5. Publish state change event
	pipe.Publish(context.Background(), "state:change", fmt.Sprintf("submission:validated:%s", submissionID))

	// Execute pipeline (ignore errors - monitoring is non-critical)
	if _, err := pipe.Exec(context.Background()); err != nil {
		log.Debugf("Failed to write monitoring metrics: %v", err)
	}

	return snapshotterAddr.Hex(), nil
}

func (d *Dequeuer) validateSubmission(submission *SnapshotSubmission) error {
	// Epoch 0 heartbeat handling: Skip validation for epoch 0 with empty CID
	// These are P2P mesh maintenance messages from Go local collector, not actual submissions
	if submission.Request.EpochId == 0 && submission.Request.SnapshotCid == "" {
		// TODO: [SIGNED-HEARTBEATS] Currently heartbeat messages (epoch 0 + empty CID) are not EIP-712 signed,
		// so we cannot extract the snapshotter address. To enable peer banning based on heartbeat authenticity:
		// 1. Modify local-collector to EIP-712 sign heartbeat messages with snapshotter private key
		// 2. Verify signature here and extract snapshotter address
		// 3. Cache peer ID -> snapshotter address mapping for heartbeats
		// 4. Enable banning peers with invalid/missing EIP-712 signatures
		// See: snapshotter-lite-local-collector/pkgs/service/msg_server.go publishHeartbeats()
		//
		// CURRENT IMPLEMENTATION (peer-ID-only tracking):
		// Heartbeats are detected and cached by peer ID in cmd/unified/main.go cacheHeartbeat()
		// This allows tracking peer activity without snapshotter address correlation.
		// To correlate peer ID with snapshotter address, use:
		//   - /api/v1/simulations/recent (simulation messages have EIP-712 signatures)
		//   - /api/v1/epochs/{epochID}/submissions (regular submissions have EIP-712 signatures)
		// Then query /api/v1/heartbeats/peer/{peerID} for heartbeat activity.
		return fmt.Errorf("epoch 0 heartbeat: skipping")
	}

	// Basic validation for real submissions
	if submission.Request.SnapshotCid == "" {
		return fmt.Errorf("empty snapshot CID")
	}

	if submission.Request.ProjectId == "" {
		return fmt.Errorf("empty project ID")
	}

	if submission.DataMarket == "" {
		return fmt.Errorf("empty data market address")
	}

	return nil
}

func (d *Dequeuer) verifySignature(submission *SnapshotSubmission) (common.Address, error) {
	// Skip signature verification for epoch 0 heartbeats
	if submission.Request.EpochId == 0 && submission.Request.SnapshotCid == "" {
		return common.Address{}, nil
	}

	// Check signature exists
	if submission.Signature == "" {
		log.Errorf("EIP-712 verification failed: empty signature (epoch=%d, project=%s, CID=%s, slot=%d)",
			submission.Request.EpochId, submission.Request.ProjectId, submission.Request.SnapshotCid, submission.Request.SlotId)
		return common.Address{}, fmt.Errorf("empty signature")
	}

	// Create EIP-712 request from submission
	request := &customcrypto.SnapshotRequest{
		SlotId:      submission.Request.SlotId,
		Deadline:    submission.Request.Deadline,
		SnapshotCid: submission.Request.SnapshotCid,
		EpochId:     submission.Request.EpochId,
		ProjectId:   submission.Request.ProjectId,
	}

	// Verify signature and recover signer address
	signerAddr, err := d.eip712Verifier.VerifySignature(request, submission.Signature)
	if err != nil {
		log.Errorf("EIP-712 verification failed: %v (epoch=%d, project=%s, CID=%s, slot=%d, signature=%s)",
			err, submission.Request.EpochId, submission.Request.ProjectId,
			submission.Request.SnapshotCid, submission.Request.SlotId, submission.Signature[:20]+"...")
		return common.Address{}, fmt.Errorf("signature verification failed: %w", err)
	}

	log.Infof("EIP-712 signer extracted: epoch=%d, slot=%d, deadline=%d, project=%s, signer=%s, CID=%s",
		submission.Request.EpochId, submission.Request.SlotId, submission.Request.Deadline,
		submission.Request.ProjectId, signerAddr.Hex(), submission.Request.SnapshotCid)

	return signerAddr, nil
}

func (d *Dequeuer) storeProcessingResult(submissionID string, processed *ProcessedSubmission) {
	ctx := context.Background()

	// Extract market from submission
	dataMarket := processed.Submission.DataMarket

	// Use configured protocol state contract for namespacing (not from submission which may be empty/wrong)
	// This ensures consistency with event-monitor which uses cfg.ContractAddress
	protocolState := d.protocolStateContract
	if protocolState == "" {
		// Fallback to submission's protocolState if configured one is empty (shouldn't happen)
		protocolState = processed.Submission.ProtocolState
		if protocolState == "" {
			log.Warnf("No protocol state contract configured and submission has none, using empty string for key")
		}
	}

	// Namespace keys by protocol:market:epoch
	// Format: {protocol}:{market}:processed:{sequencer_id}:{submission_id}
	key := fmt.Sprintf("%s:%s:processed:%s:%s",
		protocolState, dataMarket, d.sequencerID, submissionID)

	data, err := json.Marshal(processed)
	if err != nil {
		log.Errorf("Failed to marshal processing result: %v", err)
		return
	}

	// Store with TTL - must be at least as long as epoch processed SET (1 hour)
	// to ensure submissions are available when event monitor collects them for finalization
	err = d.redisClient.Set(ctx, key, data, 1*time.Hour).Err()
	if err != nil {
		log.Errorf("Failed to store processing result: %v", err)
	}

	// Update epoch processing set (namespaced by protocol:market:epoch)
	epochKey := fmt.Sprintf("%s:%s:epoch:%d:processed",
		protocolState, dataMarket, processed.Submission.Request.EpochId)
	d.redisClient.SAdd(ctx, epochKey, submissionID)
	d.redisClient.Expire(ctx, epochKey, 1*time.Hour)

	// Store deterministically in epoch-keyed structures (ZSET + HASH)
	// This eliminates the need for SCAN operations when collecting submissions for finalization
	epochIDStr := strconv.FormatUint(processed.Submission.Request.EpochId, 10)

	// Create KeyBuilder with correct protocolState and dataMarket for epoch-specific keys
	epochKeyBuilder := redislib.NewKeyBuilder(protocolState, dataMarket)

	// Add submission ID to ZSET for deterministic ordering (score = timestamp)
	submissionsIdsKey := epochKeyBuilder.EpochSubmissionsIds(epochIDStr)
	zsetErr := d.redisClient.ZAdd(ctx, submissionsIdsKey, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: submissionID,
	}).Err()
	if zsetErr != nil {
		log.Errorf("❌ CRITICAL: Failed to add submission %s to ZSET %s: %v", submissionID, submissionsIdsKey, zsetErr)
		// Continue - don't fail entire operation, but this is a critical error
	}

	// Store submission data in epoch-keyed HASH for deterministic lookup
	submissionsDataKey := epochKeyBuilder.EpochSubmissionsData(epochIDStr)
	hashErr := d.redisClient.HSet(ctx, submissionsDataKey, submissionID, data).Err()
	if hashErr != nil {
		log.Errorf("❌ CRITICAL: Failed to store submission %s in HASH %s: %v", submissionID, submissionsDataKey, hashErr)
		// Continue - don't fail entire operation, but this is a critical error
	}

	// Combined log for both operations (only when both succeed)
	if zsetErr == nil && hashErr == nil {
		log.Infof("✅ Added and stored submission %s to ZSET %s and HASH %s", submissionID, submissionsIdsKey, submissionsDataKey)
	} else {
		// Log errors if either operation failed
		if zsetErr != nil {
			log.Errorf("❌ CRITICAL: Failed to add submission %s to ZSET %s: %v", submissionID, submissionsIdsKey, zsetErr)
		}
		if hashErr != nil {
			log.Errorf("❌ CRITICAL: Failed to store submission %s in HASH %s: %v", submissionID, submissionsDataKey, hashErr)
		}
	}

	// Set TTL on BOTH epoch structures (2 hours covers finalization window + buffer)
	// Refresh TTL on each write to ensure data persists through finalization
	if err := d.redisClient.Expire(ctx, submissionsIdsKey, 2*time.Hour).Err(); err != nil {
		log.Errorf("❌ CRITICAL: Failed to set TTL on ZSET %s: %v", submissionsIdsKey, err)
	}
	if err := d.redisClient.Expire(ctx, submissionsDataKey, 2*time.Hour).Err(); err != nil {
		log.Errorf("❌ CRITICAL: Failed to set TTL on HASH %s: %v", submissionsDataKey, err)
	}

	log.Debugf("Stored submission %s with epochKey: %s (protocolState=%s, market=%s, epoch=%d)",
		submissionID, epochKey, protocolState, dataMarket, processed.Submission.Request.EpochId)
	log.Debugf("Stored submission %s deterministically in epoch-keyed structures (ZSET: %s, HASH: %s)",
		submissionID, submissionsIdsKey, submissionsDataKey)
}

func (d *Dequeuer) cleanupOldSubmissions() {
	// Keep only the most recent 500 submissions
	if len(d.processedSubmissions) <= 500 {
		return
	}

	// Create slice of submissions sorted by time
	type submissionEntry struct {
		id   string
		time time.Time
	}

	entries := make([]submissionEntry, 0, len(d.processedSubmissions))
	for id, sub := range d.processedSubmissions {
		entries = append(entries, submissionEntry{id: id, time: sub.ProcessedAt})
	}

	// Sort by time (oldest first)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].time.After(entries[j].time) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Remove oldest entries
	toRemove := len(entries) - 500
	for i := 0; i < toRemove; i++ {
		delete(d.processedSubmissions, entries[i].id)
	}
}

func (d *Dequeuer) updateStats(success bool, processingTime time.Duration) {
	d.statsMutex.Lock()
	defer d.statsMutex.Unlock()

	d.stats.TotalProcessed++
	if success {
		d.stats.SuccessfulCount++
	} else {
		d.stats.FailedCount++
	}
	d.stats.LastProcessedAt = time.Now()

	// Calculate rolling average processing rate
	if processingTime > 0 {
		rate := 1.0 / processingTime.Seconds()
		if d.stats.ProcessingRate == 0 {
			d.stats.ProcessingRate = rate
		} else {
			// Exponential moving average
			d.stats.ProcessingRate = 0.9*d.stats.ProcessingRate + 0.1*rate
		}
	}
}

// GetStats returns current dequeuer statistics
func (d *Dequeuer) GetStats() DequeuerStats {
	d.statsMutex.RLock()
	defer d.statsMutex.RUnlock()
	return d.stats
}

// GetProcessedCount returns count of processed submissions for an epoch
func (d *Dequeuer) GetProcessedCount(epochID uint64) int {
	d.submissionsMutex.RLock()
	defer d.submissionsMutex.RUnlock()

	count := 0
	for _, sub := range d.processedSubmissions {
		if sub.Submission.Request.EpochId == epochID {
			count++
		}
	}
	return count
}

// CacheSimulation stores simulation message data (epoch 0 with real CID) in Redis for monitoring.
// Simulation messages are sent by snapshotters at startup to verify connectivity and are EIP-712 signed,
// allowing us to extract the snapshotter address and map it to the peer ID.
// This data is useful for:
// 1. Verifying snapshotter identity at startup
// 2. Mapping peer IDs to snapshotter addresses
// 3. Monitoring which snapshotters have connected and when
func (d *Dequeuer) CacheSimulation(peerID string, snapshotterAddr common.Address, submission *SnapshotSubmission) error {
	ctx := context.Background()
	timestamp := time.Now().Unix()

	// Create entity ID for the simulation
	entityID := fmt.Sprintf("sim:%d:%s:%d:%s",
		submission.Request.SlotId,
		submission.Request.ProjectId,
		timestamp,
		peerID)

	// Create metadata for the simulation
	metadata := map[string]interface{}{
		"peer_id":             peerID,
		"snapshotter_address": snapshotterAddr.Hex(),
		"slot_id":             submission.Request.SlotId,
		"project_id":          submission.Request.ProjectId,
		"snapshot_cid":        submission.Request.SnapshotCid,
		"data_market":         submission.DataMarket,
		"timestamp":           timestamp,
		"entity_id":           entityID,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal simulation metadata: %w", err)
	}

	// Pipeline for atomic writes
	pipe := d.redisClient.Pipeline()

	// 1. Add to simulations timeline (ZSET sorted by timestamp)
	pipe.ZAdd(ctx, d.keyBuilder.SimulationsTimeline(), redis.Z{
		Score:  float64(timestamp),
		Member: entityID,
	})

	// 2. Store simulation metadata (HASH with 7-day TTL)
	pipe.SetEx(ctx, d.keyBuilder.SimulationMetadata(entityID), metadataJSON, 7*24*time.Hour)

	// 3. Index by peer ID (SET with 7-day TTL)
	peerKey := d.keyBuilder.SimulationsByPeer(peerID)
	pipe.SAdd(ctx, peerKey, entityID)
	pipe.Expire(ctx, peerKey, 7*24*time.Hour)

	// 4. Index by snapshotter address (SET with 7-day TTL)
	snapshotterKey := d.keyBuilder.SimulationsBySnapshotter(snapshotterAddr.Hex())
	pipe.SAdd(ctx, snapshotterKey, entityID)
	pipe.Expire(ctx, snapshotterKey, 7*24*time.Hour)

	// 5. Index by slot ID (SET with 7-day TTL)
	slotKey := d.keyBuilder.SimulationsBySlot(fmt.Sprintf("%d", submission.Request.SlotId))
	pipe.SAdd(ctx, slotKey, entityID)
	pipe.Expire(ctx, slotKey, 7*24*time.Hour)

	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to cache simulation: %w", err)
	}

	log.Infof("📡 Cached simulation message: peer=%s, snapshotter=%s, slot=%d, project=%s, cid=%s",
		peerID, snapshotterAddr.Hex(), submission.Request.SlotId,
		submission.Request.ProjectId, submission.Request.SnapshotCid)

	return nil
}
