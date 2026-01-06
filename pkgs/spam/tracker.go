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
	// Maximum validation failures per epoch before reporting
	MAX_VALIDATION_FAILURES_PER_EPOCH = 5

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
	// Skip tracking if peer is whitelisted
	if t.whitelist != nil && t.whitelist.IsWhitelisted(peerID) {
		return nil
	}

	// Track by peer ID (primary)
	peerKey := t.getValidationFailureKey(peerID, epochID)
	if err := t.redisClient.Incr(ctx, peerKey).Err(); err != nil {
		return fmt.Errorf("failed to increment peer validation failure count: %w", err)
	}
	if err := t.redisClient.Expire(ctx, peerKey, SPAM_TRACKING_TTL).Err(); err != nil {
		log.Warnf("Failed to set TTL on peer validation failure key: %v", err)
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
func (t *SpamTracker) TrackSubmissionCount(ctx context.Context, peerID, snapshotterAddr string, epochID uint64) (int, error) {
	// Skip tracking if peer is whitelisted
	if t.whitelist != nil && t.whitelist.IsWhitelisted(peerID) {
		return 0, nil
	}

	// Track by peer ID (primary)
	peerKey := t.getSubmissionCountKey(peerID, epochID)
	count, err := t.redisClient.Incr(ctx, peerKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment peer submission count: %w", err)
	}
	if err := t.redisClient.Expire(ctx, peerKey, SPAM_TRACKING_TTL).Err(); err != nil {
		log.Warnf("Failed to set TTL on peer submission count key: %v", err)
	}

	// Track by snapshotter address (secondary, for evidence)
	if snapshotterAddr != "" {
		snapshotterKey := t.getSnapshotterSubmissionCountKey(snapshotterAddr, epochID)
		if err := t.redisClient.Incr(ctx, snapshotterKey).Err(); err != nil {
			return int(count), fmt.Errorf("failed to increment snapshotter submission count: %w", err)
		}
		if err := t.redisClient.Expire(ctx, snapshotterKey, SPAM_TRACKING_TTL).Err(); err != nil {
			log.Warnf("Failed to set TTL on snapshotter submission count key: %v", err)
		}

		// Track association
		assocKey := t.getPeerSnapshotterMapKey(peerID, epochID)
		if err := t.redisClient.SAdd(ctx, assocKey, snapshotterAddr).Err(); err != nil {
			return int(count), fmt.Errorf("failed to add snapshotter to peer map: %w", err)
		}
		if err := t.redisClient.Expire(ctx, assocKey, SPAM_TRACKING_TTL).Err(); err != nil {
			log.Warnf("Failed to set TTL on peer snapshotter map key: %v", err)
		}
	}

	return int(count), nil
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

// ShouldReportSpam checks if spam should be reported based on thresholds
func (t *SpamTracker) ShouldReportSpam(ctx context.Context, peerID string, epochID uint64) (bool, string, error) {
	// Skip reporting if peer is whitelisted
	if t.whitelist != nil && t.whitelist.IsWhitelisted(peerID) {
		return false, "", nil
	}

	// Check validation failures (immediate - per epoch)
	failureCount, err := t.GetValidationFailureCount(ctx, peerID, epochID)
	if err != nil {
		return false, "", err
	}
	if failureCount >= MAX_VALIDATION_FAILURES_PER_EPOCH {
		return true, "validation_failure", nil
	}

	// Check submission count (requires consecutive epochs with violations)
	// Check if current epoch has violation
	submissionCount, err := t.GetSubmissionCount(ctx, peerID, epochID)
	if err != nil {
		return false, "", err
	}
	if submissionCount > MAX_SUBMISSIONS_PER_EPOCH_LITE {
		// Current epoch has violation - check if previous N-1 epochs also had violations
		consecutiveViolations, err := t.checkConsecutiveRateLimitViolations(ctx, peerID, epochID)
		if err != nil {
			return false, "", err
		}
		if consecutiveViolations >= CONSISTENT_VIOLATIONS_THRESHOLD {
			return true, "rate_limit", nil
		}
	}

	return false, "", nil
}

// checkConsecutiveRateLimitViolations checks how many consecutive epochs (including current) have rate limit violations
// Returns the count of consecutive epochs with violations, starting from current epoch and going backwards
// Stops checking when an epoch without violation is found or when we've checked CONSISTENT_VIOLATIONS_THRESHOLD epochs
func (t *SpamTracker) checkConsecutiveRateLimitViolations(ctx context.Context, peerID string, currentEpochID uint64) (int, error) {
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
