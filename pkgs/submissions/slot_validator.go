package submissions

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

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
	redisClient            *redis.Client
	protocolStateAddr      common.Address
	snapshotterStateAddr   common.Address
}

// NewSlotValidator creates a new slot validator
// protocolStateAddr and snapshotterStateAddr are used for namespaced Redis keys
func NewSlotValidator(redisClient *redis.Client, protocolStateAddr, snapshotterStateAddr common.Address) *SlotValidator {
	return &SlotValidator{
		redisClient:          redisClient,
		protocolStateAddr:    protocolStateAddr,
		snapshotterStateAddr: snapshotterStateAddr,
	}
}

// ValidateSnapshotterForSlot checks if the recovered EIP-712 signer address matches
// the snapshotter address registered for the given slot ID in the protocol state contract
//
// This validates that:
// 1. The slot exists and is active
// 2. The signer is the authorized snapshotter for this slot
//
// Redis key format (namespaced): {protocolState}:{snapshotterState}:SlotInfo.{slotID}
func (sv *SlotValidator) ValidateSnapshotterForSlot(slotID uint64, signerAddr common.Address) error {
	ctx := context.Background()

	// Get slot info from Redis (populated by protocol-state-cacher)
	// Use namespaced key format: {protocolState}:{snapshotterState}:SlotInfo.{slotID}
	slotKey := fmt.Sprintf("%s:%s:SlotInfo.%d", sv.protocolStateAddr.Hex(), sv.snapshotterStateAddr.Hex(), slotID)
	slotData, err := sv.redisClient.Get(ctx, slotKey).Result()
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
		return fmt.Errorf("snapshotter address mismatch: slot %d is registered to %s, but signature is from %s",
			slotID, slot.SnapshotterAddress.Hex(), signerAddr.Hex())
	}

	log.Debugf("Validated snapshotter %s for slot %d", signerAddr.Hex(), slotID)
	return nil
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
