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

const (
	// Default TTL for flagged state cache (24 hours)
	FLAGGED_STATE_TTL = 24 * time.Hour
)

// FlaggedPeerInfo represents flagged peer information cached in Redis
type FlaggedPeerInfo struct {
	FlaggedAt      int64    `json:"flagged_at"`
	FirstEpoch     uint64   `json:"first_epoch"`
	LastEpoch      uint64   `json:"last_epoch"`
	SnapshotterAddrs []string `json:"snapshotter_addrs"`
	SyncedAt       int64    `json:"synced_at"`
}

// FlaggingService manages on-chain flagging and Redis cache synchronization
type FlaggingService struct {
	redisClient *redis.Client
	keyBuilder  *redislib.KeyBuilder
	whitelist   *PeerWhitelist
	// TODO: Add on-chain contract client when implementing on-chain flagging
}

// NewFlaggingService creates a new FlaggingService instance
func NewFlaggingService(redisClient *redis.Client, keyBuilder *redislib.KeyBuilder, whitelist *PeerWhitelist) *FlaggingService {
	return &FlaggingService{
		redisClient: redisClient,
		keyBuilder:  keyBuilder,
		whitelist:   whitelist,
	}
}

// FlagPeer flags a peer and associated snapshotter addresses on-chain and updates Redis cache
func (f *FlaggingService) FlagPeer(ctx context.Context, peerID string, snapshotterAddrs []string, firstEpoch, lastEpoch uint64) error {
	// Skip flagging if peer is whitelisted
	if f.whitelist != nil && f.whitelist.IsWhitelisted(peerID) {
		log.Debugf("Skipping flagging for whitelisted peer: %s", peerID)
		return nil
	}

	now := time.Now().Unix()

	// TODO: Flag on-chain via contract method
	// For now, we'll just update Redis cache
	// In production, this would call:
	// contract.flagPeer(peerID, snapshotterAddrs, firstEpoch, lastEpoch)

	// Update Redis cache
	peerInfo := &FlaggedPeerInfo{
		FlaggedAt:        now,
		FirstEpoch:       firstEpoch,
		LastEpoch:        lastEpoch,
		SnapshotterAddrs: snapshotterAddrs,
		SyncedAt:         now,
	}

	// Store peer flagging info
	peerKey := f.getFlaggedPeerKey(peerID)
	peerData, err := json.Marshal(peerInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal flagged peer info: %w", err)
	}
	if err := f.redisClient.Set(ctx, peerKey, peerData, FLAGGED_STATE_TTL).Err(); err != nil {
		return fmt.Errorf("failed to store flagged peer info: %w", err)
	}

	// Store snapshotter address flagging info
	for _, addr := range snapshotterAddrs {
		snapshotterInfo := &FlaggedPeerInfo{
			FlaggedAt:        now,
			FirstEpoch:       firstEpoch,
			LastEpoch:        lastEpoch,
			SnapshotterAddrs: []string{addr},
			SyncedAt:         now,
		}
		snapshotterKey := f.getFlaggedSnapshotterKey(addr)
		snapshotterData, err := json.Marshal(snapshotterInfo)
		if err != nil {
			log.Warnf("Failed to marshal flagged snapshotter info: %v", err)
			continue
		}
		if err := f.redisClient.Set(ctx, snapshotterKey, snapshotterData, FLAGGED_STATE_TTL).Err(); err != nil {
			log.Warnf("Failed to store flagged snapshotter info: %v", err)
			continue
		}
	}

	// Update flagged sets for quick lookup
	flaggedPeersKey := f.getFlaggedPeersSetKey()
	if err := f.redisClient.SAdd(ctx, flaggedPeersKey, peerID).Err(); err != nil {
		log.Warnf("Failed to add peer to flagged set: %v", err)
	}
	if err := f.redisClient.Expire(ctx, flaggedPeersKey, FLAGGED_STATE_TTL).Err(); err != nil {
		log.Warnf("Failed to set TTL on flagged peers set: %v", err)
	}

	flaggedSnapshottersKey := f.getFlaggedSnapshottersSetKey()
	for _, addr := range snapshotterAddrs {
		if err := f.redisClient.SAdd(ctx, flaggedSnapshottersKey, addr).Err(); err != nil {
			log.Warnf("Failed to add snapshotter to flagged set: %v", err)
		}
	}
	if err := f.redisClient.Expire(ctx, flaggedSnapshottersKey, FLAGGED_STATE_TTL).Err(); err != nil {
		log.Warnf("Failed to set TTL on flagged snapshotters set: %v", err)
	}

	log.WithFields(log.Fields{
		"peer_id":           peerID,
		"snapshotter_addrs": snapshotterAddrs,
		"first_epoch":       firstEpoch,
		"last_epoch":        lastEpoch,
	}).Infof("🚩 Flagged peer %s with %d snapshotter addresses", peerID, len(snapshotterAddrs))

	return nil
}

// IsPeerFlagged checks if a peer is flagged (checks Redis cache first, then on-chain)
func (f *FlaggingService) IsPeerFlagged(ctx context.Context, peerID string) (bool, error) {
	// Check Redis cache first
	peerKey := f.getFlaggedPeerKey(peerID)
	exists, err := f.redisClient.Exists(ctx, peerKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check flagged peer: %w", err)
	}
	if exists > 0 {
		return true, nil
	}

	// Check flagged peers set
	flaggedPeersKey := f.getFlaggedPeersSetKey()
	isMember, err := f.redisClient.SIsMember(ctx, flaggedPeersKey, peerID).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check flagged peers set: %w", err)
	}
	if isMember {
		return true, nil
	}

	// TODO: If cache miss, query on-chain contract
	// For now, return false if not in cache

	return false, nil
}

// IsSnapshotterFlagged checks if a snapshotter address is flagged
func (f *FlaggingService) IsSnapshotterFlagged(ctx context.Context, snapshotterAddr string) (bool, error) {
	// Check Redis cache first
	snapshotterKey := f.getFlaggedSnapshotterKey(snapshotterAddr)
	exists, err := f.redisClient.Exists(ctx, snapshotterKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check flagged snapshotter: %w", err)
	}
	if exists > 0 {
		return true, nil
	}

	// Check flagged snapshotters set
	flaggedSnapshottersKey := f.getFlaggedSnapshottersSetKey()
	isMember, err := f.redisClient.SIsMember(ctx, flaggedSnapshottersKey, snapshotterAddr).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check flagged snapshotters set: %w", err)
	}
	if isMember {
		return true, nil
	}

	// TODO: If cache miss, query on-chain contract
	// For now, return false if not in cache

	return false, nil
}

// Redis key builders

func (f *FlaggingService) getFlaggedPeerKey(peerID string) string {
	return fmt.Sprintf("%s:%s:spam:consensus_flagged:peer:%s", f.keyBuilder.ProtocolState, f.keyBuilder.DataMarket, peerID)
}

func (f *FlaggingService) getFlaggedSnapshotterKey(snapshotterAddr string) string {
	return fmt.Sprintf("%s:%s:spam:consensus_flagged:snapshotter:%s", f.keyBuilder.ProtocolState, f.keyBuilder.DataMarket, snapshotterAddr)
}

func (f *FlaggingService) getFlaggedPeersSetKey() string {
	return fmt.Sprintf("flagged_peers:%s", f.keyBuilder.DataMarket)
}

func (f *FlaggingService) getFlaggedSnapshottersSetKey() string {
	return fmt.Sprintf("flagged_snapshotters:%s", f.keyBuilder.DataMarket)
}

