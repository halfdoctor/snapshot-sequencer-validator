package submissions

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/protocolstate"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// Type alias for cleaner API
type SlotManager = protocolstate.SlotManager

// SlotInfo represents the cached slot information from protocol-state-cacher
// This structure matches the data stored by protocol-state-cacher in Redis
type SlotInfo struct {
	SnapshotterAddress common.Address `json:"SnapshotterAddress"`
	NodePrice          *big.Int       `json:"NodePrice"`
	AmountSentOnL1     *big.Int       `json:"AmountSentOnL1"`
	MintedOn           *big.Int       `json:"MintedOn"`
	BurnedOn           *big.Int       `json:"BurnedOn"`
	LastUpdated        *big.Int       `json:"LastUpdated"`
	IsLegacy           bool           `json:"IsLegacy"`
	ClaimedTokens      bool           `json:"ClaimedTokens"`
	Active             bool           `json:"Active"`
	IsKyced            bool           `json:"IsKyced"`
}

// SlotValidator validates snapshotter addresses against cached slot info from smart contracts
type SlotValidator struct {
	redisClient          *redis.Client
	protocolStateAddr    common.Address
	snapshotterStateAddr common.Address
	slotManager          *SlotManager // Optional: for on-demand slot fetching
	verifiedTTL          time.Duration // TTL for the SlotVerified marker (default: 1 hour)
}

// NewSlotValidator creates a new slot validator
// protocolStateAddr and snapshotterStateAddr are used for namespaced Redis keys
// slotManager is optional - if provided, enables on-demand slot fetching when cache misses occur
func NewSlotValidator(redisClient *redis.Client, protocolStateAddr, snapshotterStateAddr common.Address, slotManager *SlotManager) *SlotValidator {
	return &SlotValidator{
		redisClient:          redisClient,
		protocolStateAddr:    protocolStateAddr,
		snapshotterStateAddr: snapshotterStateAddr,
		slotManager:          slotManager,
		verifiedTTL:          1 * time.Hour,
	}
}

// SetVerifiedTTL sets the TTL for demand-driven re-fetch state markers.
// This controls how long after a contract re-fetch the validator will skip
// re-fetching on repeated mismatches for the same slot.
func (sv *SlotValidator) SetVerifiedTTL(ttl time.Duration) {
	sv.verifiedTTL = ttl
}

// ValidateSnapshotterForSlot checks if the recovered EIP-712 signer address matches
// the snapshotter address registered for the given slot ID in the protocol state contract
//
// This validates that:
// 1. The slot exists and is active
// 2. The signer is the authorized snapshotter for this slot
//
// Redis key format (namespaced): {protocolState}:{snapshotterState}:SlotInfo.{slotID}
//
// If SlotManager is available, automatically fetches missing slots from contract on cache miss
func (sv *SlotValidator) ValidateSnapshotterForSlot(slotID uint64, signerAddr common.Address) error {
	ctx := context.Background()

	// Get slot info from Redis (populated by protocol-state-cacher)
	// Use namespaced key format: {protocolState}:{snapshotterState}:SlotInfo.{slotID}
	slotKey := fmt.Sprintf("%s:%s:SlotInfo.%d", sv.protocolStateAddr.Hex(), sv.snapshotterStateAddr.Hex(), slotID)
	slotData, err := sv.redisClient.Get(ctx, slotKey).Result()
	
	// On cache miss, try on-demand fetch if SlotManager is available
	if err == redis.Nil && sv.slotManager != nil {
		log.Infof("Slot %d not in cache - attempting on-demand fetch from contract", slotID)
		
		// Create context with timeout for contract call
		fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		
		if fetchErr := sv.slotManager.FetchSlot(fetchCtx, slotID); fetchErr != nil {
			return fmt.Errorf("slot %d not found in cache and on-demand fetch failed: %w", slotID, fetchErr)
		}
		
		log.Infof("✅ Successfully fetched slot %d on-demand", slotID)
		
		// Retry cache read after successful fetch
		slotData, err = sv.redisClient.Get(ctx, slotKey).Result()
	}
	
	if err == redis.Nil {
		return fmt.Errorf("slot %d not found in protocol state cache - may not be registered", slotID)
	} else if err != nil {
		return fmt.Errorf("failed to fetch slot info for slot %d: %w", slotID, err)
	}

	// Unmarshal slot info
	var slot SlotInfo
	if err := json.Unmarshal([]byte(slotData), &slot); err != nil {
		return fmt.Errorf("failed to parse slot info for slot %d: %w", slotID, err)
	}

	// Check if slot is active
	if !slot.Active {
		return fmt.Errorf("slot %d is not active (burned or inactive)", slotID)
	}

	// Validate snapshotter address matches
	if slot.SnapshotterAddress != signerAddr {
		// Demand-driven re-fetch: the cache may be stale due to a missed
		// SnapshotterAddressChanged event. Re-fetch from contract if the slot
		// has not been recently verified to avoid permanent rejection.
		if sv.slotManager != nil && !sv.wasRecentlyVerified(ctx, slotID) {
			log.Infof("Address mismatch for slot %d (cached=%s, signer=%s) - re-fetching from contract",
				slotID, slot.SnapshotterAddress.Hex(), signerAddr.Hex())

			fetchCtx, fetchCancel := context.WithTimeout(ctx, 30*time.Second)
			defer fetchCancel()

			if fetchErr := sv.slotManager.FetchSlot(fetchCtx, slotID); fetchErr != nil {
				log.Warnf("Demand-driven re-fetch failed for slot %d: %v", slotID, fetchErr)
			} else {
				sv.markAsVerified(ctx, slotID)

				// Re-read from cache after refresh
				updatedData, readErr := sv.redisClient.Get(ctx, slotKey).Result()
				if readErr == nil {
					var updatedSlot SlotInfo
					if json.Unmarshal([]byte(updatedData), &updatedSlot) == nil {
						if updatedSlot.Active && updatedSlot.SnapshotterAddress == signerAddr {
							log.Infof("Slot %d self-healed via demand-driven re-fetch (new snapshotter=%s)",
								slotID, signerAddr.Hex())
							return nil
						}
					}
				}
			}
		}

		return fmt.Errorf("snapshotter address mismatch: slot %d is registered to %s, but signature is from %s",
			slotID, slot.SnapshotterAddress.Hex(), signerAddr.Hex())
	}

	log.Debugf("Validated snapshotter %s for slot %d", signerAddr.Hex(), slotID)
	return nil
}

// wasRecentlyVerified checks if the slot was recently re-fetched from contract.
func (sv *SlotValidator) wasRecentlyVerified(ctx context.Context, slotID uint64) bool {
	key := fmt.Sprintf("%s:%s:SlotVerified.%d", sv.protocolStateAddr.Hex(), sv.snapshotterStateAddr.Hex(), slotID)
	exists, err := sv.redisClient.Exists(ctx, key).Result()
	return err == nil && exists > 0
}

// markAsVerified sets a state marker indicating the slot was recently verified
// by re-fetching from the contract. TTL prevents repeated contract calls for
// the same invalid slot.
func (sv *SlotValidator) markAsVerified(ctx context.Context, slotID uint64) {
	key := fmt.Sprintf("%s:%s:SlotVerified.%d", sv.protocolStateAddr.Hex(), sv.snapshotterStateAddr.Hex(), slotID)
	if err := sv.redisClient.Set(ctx, key, "1", sv.verifiedTTL).Err(); err != nil {
		log.Warnf("Failed to set SlotVerified marker for slot %d: %v", slotID, err)
	}
}

// GetSlotSnapshotter returns the registered snapshotter address for a slot
func (sv *SlotValidator) GetSlotSnapshotter(slotID uint64) (common.Address, error) {
	ctx := context.Background()

	// Use namespaced key format: {protocolState}:{snapshotterState}:SlotInfo.{slotID}
	slotKey := fmt.Sprintf("%s:%s:SlotInfo.%d", sv.protocolStateAddr.Hex(), sv.snapshotterStateAddr.Hex(), slotID)
	slotData, err := sv.redisClient.Get(ctx, slotKey).Result()
	if err == redis.Nil {
		return common.Address{}, fmt.Errorf("slot %d not found", slotID)
	} else if err != nil {
		return common.Address{}, fmt.Errorf("failed to fetch slot info: %w", err)
	}

	var slot SlotInfo
	if err := json.Unmarshal([]byte(slotData), &slot); err != nil {
		return common.Address{}, fmt.Errorf("failed to parse slot info: %w", err)
	}

	return slot.SnapshotterAddress, nil
}
