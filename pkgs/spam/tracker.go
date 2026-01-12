package spam

import (
	"context"
	"fmt"
	"time"

	redislib "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

const (
	// Maximum validation failures per epoch before reporting (immediate)
	MAX_VALIDATION_FAILURES_PER_EPOCH = 5

	// Maximum validation failures per epoch for consecutive tracking
	MAX_VALIDATION_FAILURES_PER_EPOCH_CONSECUTIVE = 2

	// Maximum submissions per epoch for lite nodes (full nodes bypass)
	MAX_SUBMISSIONS_PER_EPOCH_LITE = 2

	// Number of consecutive epochs with violations before reporting
	CONSISTENT_VIOLATIONS_THRESHOLD = 3

	// Minimum validators needed for consensus (hardcoded for consistency)
	SPAM_CONSENSUS_THRESHOLD = 2 // 2 out of 3 validators

	// Default TTL for spam tracking keys (2 hours)
	SPAM_TRACKING_TTL = 2 * time.Hour
)

// SpamTracker tracks validation failures and submission counts per peer/snapshotter per epoch
type SpamTracker struct {
	redisClient *redis.Client
	keyBuilder  *redislib.KeyBuilder
	whitelist   *PeerWhitelist
}

// NewSpamTracker creates a new SpamTracker instance
func NewSpamTracker(redisClient *redis.Client, keyBuilder *redislib.KeyBuilder, whitelist *PeerWhitelist) *SpamTracker {
	return &SpamTracker{
		redisClient: redisClient,
		keyBuilder:  keyBuilder,
		whitelist:   whitelist,
	}
}

// TrackValidationFailure tracks a validation failure for a peer and snapshotter address
func (t *SpamTracker) TrackValidationFailure(ctx context.Context, peerID, snapshotterAddr string, epochID uint64, err error) error {
	// Skip epoch 0 - it's dummy/heartbeat only, never processed
	if epochID == 0 {
		return nil
	}

	// Skip tracking if peer is whitelisted
	if t.whitelist != nil && t.whitelist.IsWhitelisted(peerID) {
		return nil
	}

	// Track by peer ID (primary)
	peerKey := t.getValidationFailureKey(peerID, epochID)
	failureCount, err := t.redisClient.Incr(ctx, peerKey).Result()
	if err != nil {
		return fmt.Errorf("failed to increment peer validation failure count: %w", err)
	}
	if err := t.redisClient.Expire(ctx, peerKey, SPAM_TRACKING_TTL).Err(); err != nil {
		log.Warnf("Failed to set TTL on peer validation failure key: %v", err)
	}

	// Add peer ID to epoch's peer set (for deterministic discovery at epoch boundaries)
	epochPeersKey := t.getEpochPeersKey(epochID)
	if err := t.redisClient.SAdd(ctx, epochPeersKey, peerID).Err(); err != nil {
		log.Warnf("Failed to add peer to epoch peers set: %v", err)
	} else {
		// Set TTL on epoch peers set (same as tracking TTL)
		if err := t.redisClient.Expire(ctx, epochPeersKey, SPAM_TRACKING_TTL).Err(); err != nil {
			log.Warnf("Failed to set TTL on epoch peers set: %v", err)
		}
		log.Debugf("Tracked validation failure for peer %s epoch %d (count: %d)", peerID, epochID, failureCount)
	}

	// Track by snapshotter address (secondary, if available)
	if snapshotterAddr != "" {
		snapshotterKey := t.getSnapshotterValidationFailureKey(snapshotterAddr, epochID)
		if err := t.redisClient.Incr(ctx, snapshotterKey).Err(); err != nil {
			return fmt.Errorf("failed to increment snapshotter validation failure count: %w", err)
		}
		if err := t.redisClient.Expire(ctx, snapshotterKey, SPAM_TRACKING_TTL).Err(); err != nil {
			log.Warnf("Failed to set TTL on snapshotter validation failure key: %v", err)
		}

		// Track association: which addresses this peer uses
		assocKey := t.getPeerSnapshotterMapKey(peerID, epochID)
		if err := t.redisClient.SAdd(ctx, assocKey, snapshotterAddr).Err(); err != nil {
			return fmt.Errorf("failed to add snapshotter to peer map: %w", err)
		}
		if err := t.redisClient.Expire(ctx, assocKey, SPAM_TRACKING_TTL).Err(); err != nil {
			log.Warnf("Failed to set TTL on peer snapshotter map key: %v", err)
		}
	}

	return nil
}

// TrackSubmissionCount tracks submission count for a peer and snapshotter address
// Returns the current count after incrementing
// For bulk service peers: Tracks by snapshotter address only (peer ID remains whitelisted)
// For full node peers: Skips all tracking
// For regular peers: Tracks by both peer ID and snapshotter address
func (t *SpamTracker) TrackSubmissionCount(ctx context.Context, peerID, snapshotterAddr string, epochID uint64) (int, error) {
	// Skip epoch 0 - it's dummy/heartbeat only, never processed
	if epochID == 0 {
		return 0, nil
	}

	// Check if peer is whitelisted
	isWhitelisted := false
	isBulkService := false
	isFullNode := false
	if t.whitelist != nil {
		isWhitelisted = t.whitelist.IsWhitelisted(peerID)
		if isWhitelisted {
			isBulkService = t.whitelist.IsBulkService(peerID)
			isFullNode = t.whitelist.IsFullNode(peerID)
		}
	}

	// Full node peers: Skip all tracking
	if isFullNode {
		return 0, nil
	}

	// Bulk service peers: Skip peer ID tracking but continue snapshotter address tracking
	// Regular peers: Track by both peer ID and snapshotter address
	if !isBulkService {
		// Track by peer ID (primary)
		peerKey := t.getSubmissionCountKey(peerID, epochID)
		count, err := t.redisClient.Incr(ctx, peerKey).Result()
		if err != nil {
			return 0, fmt.Errorf("failed to increment peer submission count: %w", err)
		}
		if err := t.redisClient.Expire(ctx, peerKey, SPAM_TRACKING_TTL).Err(); err != nil {
			log.Warnf("Failed to set TTL on peer submission count key: %v", err)
		}

		// Add peer ID to epoch's peer set (for deterministic discovery at epoch boundaries)
		epochPeersKey := t.getEpochPeersKey(epochID)
		if err := t.redisClient.SAdd(ctx, epochPeersKey, peerID).Err(); err != nil {
			log.Warnf("Failed to add peer to epoch peers set: %v", err)
		} else {
			// Set TTL on epoch peers set (same as tracking TTL)
			if err := t.redisClient.Expire(ctx, epochPeersKey, SPAM_TRACKING_TTL).Err(); err != nil {
				log.Warnf("Failed to set TTL on epoch peers set: %v", err)
			}
			log.Debugf("Tracked submission for peer %s epoch %d (count: %d)", peerID, epochID, count)
		}
	}

	// Track by snapshotter address (secondary, for evidence; primary for bulk service peers)
	if snapshotterAddr != "" {
		snapshotterKey := t.getSnapshotterSubmissionCountKey(snapshotterAddr, epochID)
		snapshotterCount, err := t.redisClient.Incr(ctx, snapshotterKey).Result()
		if err != nil {
			if isBulkService {
				return 0, fmt.Errorf("failed to increment snapshotter submission count: %w", err)
			}
			// For regular peers, return peer count even if snapshotter tracking fails
			peerKey := t.getSubmissionCountKey(peerID, epochID)
			peerCount, _ := t.redisClient.Get(ctx, peerKey).Int64()
			return int(peerCount), fmt.Errorf("failed to increment snapshotter submission count: %w", err)
		}
		if err := t.redisClient.Expire(ctx, snapshotterKey, SPAM_TRACKING_TTL).Err(); err != nil {
			log.Warnf("Failed to set TTL on snapshotter submission count key: %v", err)
		}

		// Track association
		assocKey := t.getPeerSnapshotterMapKey(peerID, epochID)
		if err := t.redisClient.SAdd(ctx, assocKey, snapshotterAddr).Err(); err != nil {
			if isBulkService {
				return int(snapshotterCount), fmt.Errorf("failed to add snapshotter to peer map: %w", err)
			}
			peerKey := t.getSubmissionCountKey(peerID, epochID)
			peerCount, _ := t.redisClient.Get(ctx, peerKey).Int64()
			return int(peerCount), fmt.Errorf("failed to add snapshotter to peer map: %w", err)
		}
		if err := t.redisClient.Expire(ctx, assocKey, SPAM_TRACKING_TTL).Err(); err != nil {
			log.Warnf("Failed to set TTL on peer snapshotter map key: %v", err)
		}

		// For bulk service peers, return snapshotter count; for regular peers, return peer count
		if isBulkService {
			log.Debugf("Tracked submission for bulk service peer %s snapshotter %s epoch %d (snapshotter_count: %d)", peerID, snapshotterAddr, epochID, snapshotterCount)
			return int(snapshotterCount), nil
		}
		peerKey := t.getSubmissionCountKey(peerID, epochID)
		peerCount, _ := t.redisClient.Get(ctx, peerKey).Int64()
		return int(peerCount), nil
	}

	// If no snapshotter address, return peer count (shouldn't happen for regular peers)
	if !isBulkService {
		peerKey := t.getSubmissionCountKey(peerID, epochID)
		peerCount, _ := t.redisClient.Get(ctx, peerKey).Int64()
		return int(peerCount), nil
	}

	return 0, nil
}

// GetValidationFailureCount returns the validation failure count for a peer in an epoch
func (t *SpamTracker) GetValidationFailureCount(ctx context.Context, peerID string, epochID uint64) (int, error) {
	key := t.getValidationFailureKey(peerID, epochID)
	count, err := t.redisClient.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get validation failure count: %w", err)
	}
	return count, nil
}

// GetSubmissionCount returns the submission count for a peer in an epoch
func (t *SpamTracker) GetSubmissionCount(ctx context.Context, peerID string, epochID uint64) (int, error) {
	key := t.getSubmissionCountKey(peerID, epochID)
	count, err := t.redisClient.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get submission count: %w", err)
	}
	return count, nil
}

// GetSnapshotterSubmissionCount returns the submission count for a snapshotter address in an epoch
func (t *SpamTracker) GetSnapshotterSubmissionCount(ctx context.Context, snapshotterAddr string, epochID uint64) (int, error) {
	key := t.getSnapshotterSubmissionCountKey(snapshotterAddr, epochID)
	count, err := t.redisClient.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get snapshotter submission count: %w", err)
	}
	return count, nil
}

// ShouldReportSpam checks if spam should be reported based on thresholds
func (t *SpamTracker) ShouldReportSpam(ctx context.Context, peerID string, epochID uint64) (bool, string, error) {
	// Skip reporting if peer is whitelisted (full node or bulk service)
	if t.whitelist != nil && t.whitelist.IsWhitelisted(peerID) {
		return false, "", nil
	}

	// Check validation failures
	failureCount, err := t.GetValidationFailureCount(ctx, peerID, epochID)
	if err != nil {
		return false, "", err
	}

	// Immediate reporting: >= MAX_VALIDATION_FAILURES_PER_EPOCH failures in single epoch
	if failureCount >= MAX_VALIDATION_FAILURES_PER_EPOCH {
		return true, "validation_failure", nil
	}

	// Consecutive reporting: >= MAX_VALIDATION_FAILURES_PER_EPOCH_CONSECUTIVE failures per epoch
	// for >= CONSISTENT_VIOLATIONS_THRESHOLD consecutive epochs
	if failureCount >= MAX_VALIDATION_FAILURES_PER_EPOCH_CONSECUTIVE {
		consecutiveFailures, err := t.CheckConsecutiveValidationFailures(ctx, peerID, epochID)
		if err != nil {
			return false, "", err
		}
		if consecutiveFailures >= CONSISTENT_VIOLATIONS_THRESHOLD {
			return true, "validation_failure", nil
		}
	}

	// Check submission count (requires consecutive epochs with violations for network reports)
	// Local tracking happens every epoch, but network reports only sent when threshold met
	submissionCount, err := t.GetSubmissionCount(ctx, peerID, epochID)
	if err != nil {
		return false, "", err
	}
	if submissionCount > MAX_SUBMISSIONS_PER_EPOCH_LITE {
		// Current epoch has violation - check if previous N-1 epochs also had violations
		consecutiveViolations, err := t.CheckConsecutiveRateLimitViolations(ctx, peerID, epochID)
		if err != nil {
			return false, "", err
		}
		if consecutiveViolations >= CONSISTENT_VIOLATIONS_THRESHOLD {
			return true, "rate_limit", nil
		}
	}

	return false, "", nil
}

// ShouldReportSpamForSnapshotter checks if spam should be reported for a snapshotter address
// Used for bulk service peers where we track violations by snapshotter address instead of peer ID
func (t *SpamTracker) ShouldReportSpamForSnapshotter(ctx context.Context, snapshotterAddr string, epochID uint64) (bool, string, error) {
	// Check submission count for snapshotter address
	submissionCount, err := t.GetSnapshotterSubmissionCount(ctx, snapshotterAddr, epochID)
	if err != nil {
		return false, "", err
	}

	if submissionCount > MAX_SUBMISSIONS_PER_EPOCH_LITE {
		// Current epoch has violation - check if previous N-1 epochs also had violations
		consecutiveViolations, err := t.CheckConsecutiveSnapshotterRateLimitViolations(ctx, snapshotterAddr, epochID)
		if err != nil {
			return false, "", err
		}
		if consecutiveViolations >= CONSISTENT_VIOLATIONS_THRESHOLD {
			return true, "rate_limit_snapshotter", nil
		}
	}

	return false, "", nil
}

// CheckConsecutiveRateLimitViolations checks how many consecutive epochs (including current) have rate limit violations
// Returns the count of consecutive epochs with violations, starting from current epoch and going backwards
// Stops checking when an epoch without violation is found or when we've checked CONSISTENT_VIOLATIONS_THRESHOLD epochs
func (t *SpamTracker) CheckConsecutiveRateLimitViolations(ctx context.Context, peerID string, currentEpochID uint64) (int, error) {
	consecutiveCount := 0

	// Check epochs backwards from current epoch
	// We check up to CONSISTENT_VIOLATIONS_THRESHOLD epochs (current + previous N-1)
	for i := uint64(0); i < CONSISTENT_VIOLATIONS_THRESHOLD; i++ {
		epochID := currentEpochID - i

		// If epochID underflows (epochID < i), we've gone past epoch 0
		// In this case, GetSubmissionCount will return 0 (no key exists), breaking the chain
		// This is correct behavior - we can't check epochs before epoch 0

		// Check if this epoch has a rate limit violation
		submissionCount, err := t.GetSubmissionCount(ctx, peerID, epochID)
		if err != nil {
			return consecutiveCount, fmt.Errorf("failed to get submission count for epoch %d: %w", epochID, err)
		}

		if submissionCount > MAX_SUBMISSIONS_PER_EPOCH_LITE {
			consecutiveCount++
		} else {
			// Found an epoch without violation (or epoch doesn't exist) - break the consecutive chain
			break
		}
	}

	return consecutiveCount, nil
}

// CheckConsecutiveSnapshotterRateLimitViolations checks how many consecutive epochs (including current) have rate limit violations
// for a snapshotter address (used for bulk service peers)
// Returns the count of consecutive epochs with violations, starting from current epoch and going backwards
func (t *SpamTracker) CheckConsecutiveSnapshotterRateLimitViolations(ctx context.Context, snapshotterAddr string, currentEpochID uint64) (int, error) {
	consecutiveCount := 0

	// Check epochs backwards from current epoch
	// We check up to CONSISTENT_VIOLATIONS_THRESHOLD epochs (current + previous N-1)
	for i := uint64(0); i < CONSISTENT_VIOLATIONS_THRESHOLD; i++ {
		epochID := currentEpochID - i

		// If epochID underflows (epochID < i), we've gone past epoch 0
		// In this case, GetSnapshotterSubmissionCount will return 0 (no key exists), breaking the chain
		// This is correct behavior - we can't check epochs before epoch 0

		// Check if this epoch has a rate limit violation for this snapshotter address
		submissionCount, err := t.GetSnapshotterSubmissionCount(ctx, snapshotterAddr, epochID)
		if err != nil {
			return consecutiveCount, fmt.Errorf("failed to get snapshotter submission count for epoch %d: %w", epochID, err)
		}

		if submissionCount > MAX_SUBMISSIONS_PER_EPOCH_LITE {
			consecutiveCount++
		} else {
			// Found an epoch without violation (or epoch doesn't exist) - break the consecutive chain
			break
		}
	}

	return consecutiveCount, nil
}

// CheckConsecutiveValidationFailures checks how many consecutive epochs (including current) have validation failures
// Returns the count of consecutive epochs with >= MAX_VALIDATION_FAILURES_PER_EPOCH_CONSECUTIVE failures
func (t *SpamTracker) CheckConsecutiveValidationFailures(ctx context.Context, peerID string, currentEpochID uint64) (int, error) {
	consecutiveCount := 0

	// Check epochs backwards from current epoch
	// We check up to CONSISTENT_VIOLATIONS_THRESHOLD epochs (current + previous N-1)
	for i := uint64(0); i < CONSISTENT_VIOLATIONS_THRESHOLD; i++ {
		epochID := currentEpochID - i

		// If epochID underflows (epochID < i), we've gone past epoch 0
		// In this case, GetValidationFailureCount will return 0 (no key exists), breaking the chain
		// This is correct behavior - we can't check epochs before epoch 0

		// Check if this epoch has >= MAX_VALIDATION_FAILURES_PER_EPOCH_CONSECUTIVE validation failures
		failureCount, err := t.GetValidationFailureCount(ctx, peerID, epochID)
		if err != nil {
			return consecutiveCount, fmt.Errorf("failed to get validation failure count for epoch %d: %w", epochID, err)
		}

		if failureCount >= MAX_VALIDATION_FAILURES_PER_EPOCH_CONSECUTIVE {
			consecutiveCount++
		} else {
			// Found an epoch without enough failures (or epoch doesn't exist) - break the consecutive chain
			break
		}
	}

	return consecutiveCount, nil
}

// Redis key builders

func (t *SpamTracker) getValidationFailureKey(peerID string, epochID uint64) string {
	return fmt.Sprintf("%s:%s:spam:validation_failures:peer:%s:%d", t.keyBuilder.ProtocolState, t.keyBuilder.DataMarket, peerID, epochID)
}

func (t *SpamTracker) getSnapshotterValidationFailureKey(snapshotterAddr string, epochID uint64) string {
	return fmt.Sprintf("%s:%s:spam:validation_failures:snapshotter:%s:%d", t.keyBuilder.ProtocolState, t.keyBuilder.DataMarket, snapshotterAddr, epochID)
}

func (t *SpamTracker) getSubmissionCountKey(peerID string, epochID uint64) string {
	return fmt.Sprintf("%s:%s:spam:submissions:peer:%s:%d", t.keyBuilder.ProtocolState, t.keyBuilder.DataMarket, peerID, epochID)
}

func (t *SpamTracker) getSnapshotterSubmissionCountKey(snapshotterAddr string, epochID uint64) string {
	return fmt.Sprintf("%s:%s:spam:submissions:snapshotter:%s:%d", t.keyBuilder.ProtocolState, t.keyBuilder.DataMarket, snapshotterAddr, epochID)
}

func (t *SpamTracker) getPeerSnapshotterMapKey(peerID string, epochID uint64) string {
	return fmt.Sprintf("%s:%s:spam:peer_snapshotter_map:%s:%d", t.keyBuilder.ProtocolState, t.keyBuilder.DataMarket, peerID, epochID)
}

// GetPeerSnapshotterMapKey returns the Redis key for peer-snapshotter mapping (public access)
func (t *SpamTracker) GetPeerSnapshotterMapKey(peerID string, epochID uint64) string {
	return t.getPeerSnapshotterMapKey(peerID, epochID)
}

func (t *SpamTracker) getEpochPeersKey(epochID uint64) string {
	return fmt.Sprintf("%s:%s:spam:epoch:%d:peers", t.keyBuilder.ProtocolState, t.keyBuilder.DataMarket, epochID)
}

// GetEpochPeersKey returns the Redis key for epoch peers set (public access)
func (t *SpamTracker) GetEpochPeersKey(epochID uint64) string {
	return t.getEpochPeersKey(epochID)
}
