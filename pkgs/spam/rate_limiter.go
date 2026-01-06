package spam

import (
	"context"
	"fmt"
)

// RateLimiter enforces submission rate limits per peer per epoch
type RateLimiter struct {
	tracker   *SpamTracker
	whitelist *PeerWhitelist
}

// NewRateLimiter creates a new RateLimiter instance
func NewRateLimiter(tracker *SpamTracker, whitelist *PeerWhitelist) *RateLimiter {
	return &RateLimiter{
		tracker:   tracker,
		whitelist: whitelist,
	}
}

// CheckRateLimit checks if a peer has exceeded the submission rate limit
// Returns true if the limit is exceeded, false otherwise
func (r *RateLimiter) CheckRateLimit(ctx context.Context, peerID string, epochID uint64) (bool, error) {
	// Whitelisted peers bypass rate limits
	if r.whitelist != nil && r.whitelist.IsWhitelisted(peerID) {
		return false, nil
	}

	// Get current submission count
	count, err := r.tracker.GetSubmissionCount(ctx, peerID, epochID)
	if err != nil {
		return false, fmt.Errorf("failed to get submission count: %w", err)
	}

	// Check against limit
	return count >= MAX_SUBMISSIONS_PER_EPOCH_LITE, nil
}

// IncrementAndCheck increments the submission count and checks if limit is exceeded
// Returns the new count and whether the limit was exceeded
func (r *RateLimiter) IncrementAndCheck(ctx context.Context, peerID, snapshotterAddr string, epochID uint64) (int, bool, error) {
	// Whitelisted peers bypass rate limits
	if r.whitelist != nil && r.whitelist.IsWhitelisted(peerID) {
		return 0, false, nil
	}

	// Track submission count
	count, err := r.tracker.TrackSubmissionCount(ctx, peerID, snapshotterAddr, epochID)
	if err != nil {
		return 0, false, fmt.Errorf("failed to track submission count: %w", err)
	}

	// Check against limit
	exceeded := count > MAX_SUBMISSIONS_PER_EPOCH_LITE
	return count, exceeded, nil
}

