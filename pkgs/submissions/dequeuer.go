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
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/spam"
	redislib "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
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
	spamTracker   *spam.SpamTracker
	rateLimiter   *spam.RateLimiter
	flagging      *spam.FlaggingService
	spamReporter  *spam.SpamReporter
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
func NewDequeuer(redisClient *redis.Client, keyBuilder *redislib.KeyBuilder, sequencerID string, chainID int64, protocolStateContract string, snapshotterStateAddr common.Address, enableSlotValidation bool, spamComponents *spam.SpamComponents) (*Dequeuer, error) {
	if enableSlotValidation && snapshotterStateAddr == (common.Address{}) {
		return nil, fmt.Errorf("snapshotterStateAddr is required when slot validation is enabled")
	}
	verifier, err := customcrypto.NewEIP712Verifier(chainID, protocolStateContract)
	if err != nil {
		return nil, fmt.Errorf("failed to create EIP-712 verifier: %w", err)
	}

	protocolStateAddr := common.HexToAddress(protocolStateContract)
	slotValidator := NewSlotValidator(redisClient, protocolStateAddr, snapshotterStateAddr)

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
func (d *Dequeuer) ProcessSubmission(submission *SnapshotSubmission, submissionID string, metaData map[string]interface{}) error {
	startTime := time.Now()

	// Check for duplicate
	d.submissionsMutex.RLock()
	if _, exists := d.processedSubmissions[submissionID]; exists {
		d.submissionsMutex.RUnlock()
		log.Debugf("Submission %s already processed", submissionID)
		return nil
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
				return fmt.Errorf("peer is flagged: %s", peerID)
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
		return fmt.Errorf("validation failed: %w", err)
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
		return fmt.Errorf("signature verification failed: %w", err)
	}

	// Spam protection: Check flagged snapshotter address (if enabled)
	if d.enableSpamProtection && d.flagging != nil && snapshotterAddr != (common.Address{}) {
		flagged, err := d.flagging.IsSnapshotterFlagged(ctx, snapshotterAddr.Hex())
		if err != nil {
			log.Warnf("Failed to check if snapshotter is flagged: %v", err)
		} else if flagged {
			d.updateStats(false, time.Since(startTime))
			log.Warnf("Rejected submission from flagged snapshotter: %s", snapshotterAddr.Hex())
			return fmt.Errorf("snapshotter is flagged: %s", snapshotterAddr.Hex())
		}
	}

	// Spam protection: Check rate limit (if enabled)
	if d.enableSpamProtection && d.rateLimiter != nil && peerID != "" {
		exceeded, err := d.rateLimiter.CheckRateLimit(ctx, peerID, submission.Request.EpochId)
		if err != nil {
			log.Warnf("Failed to check rate limit: %v", err)
		} else if exceeded {
			d.updateStats(false, time.Since(startTime))
			log.Warnf("Rejected submission: rate limit exceeded for peer %s (epoch %d)", peerID, submission.Request.EpochId)
			return fmt.Errorf("rate limit exceeded for peer: %s", peerID)
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
			return fmt.Errorf("slot validation failed: %w", err)
		}
		log.Debugf("Slot validation passed: slot %d is registered to %s",
			submission.Request.SlotId, snapshotterAddr.Hex())
	}

	// Spam protection: Track submission count (if enabled)
	if d.enableSpamProtection && d.spamTracker != nil && peerID != "" {
		_, err := d.spamTracker.TrackSubmissionCount(ctx, peerID, snapshotterAddr.Hex(), submission.Request.EpochId)
		if err != nil {
			log.Warnf("Failed to track submission count: %v", err)
		} else {
			// Check if spam should be reported
			if d.spamReporter != nil {
				if err := d.spamReporter.CheckAndReport(ctx, peerID, snapshotterAddr.Hex(), submission.Request.EpochId); err != nil {
					log.Warnf("Failed to check/report spam: %v", err)
				}
			}
		}
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

	// 5. Add to submissions timeline
	timestamp = time.Now().Unix()
	// Note: We don't have access to namespaced keyBuilder here, so we use the non-namespaced timeline key
	pipe.ZAdd(context.Background(), redislib.MetricsSubmissionsTimeline(), redis.Z{
		Score:  float64(timestamp),
		Member: fmt.Sprintf("validated:%s:%d", submissionID, timestamp),
	})

	// 6. Publish state change event
	pipe.Publish(context.Background(), "state:change", fmt.Sprintf("submission:validated:%s", submissionID))

	// Execute pipeline (ignore errors - monitoring is non-critical)
	if _, err := pipe.Exec(context.Background()); err != nil {
		log.Debugf("Failed to write monitoring metrics: %v", err)
	}

	return nil
}

func (d *Dequeuer) validateSubmission(submission *SnapshotSubmission) error {
	// Epoch 0 heartbeat handling: Skip validation for epoch 0 with empty CID
	// These are P2P mesh maintenance messages from Go local collector, not actual submissions
	if submission.Request.EpochId == 0 && submission.Request.SnapshotCid == "" {
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
		log.Debugf("✅ Added and stored submission %s to ZSET %s and HASH %s", submissionID, submissionsIdsKey, submissionsDataKey)
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
