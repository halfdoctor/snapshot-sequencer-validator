package protocolstate

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/powerloom/snapshot-sequencer-validator/pkgs/protocolstate/contract"
)

// SlotManager handles batch fetching and Redis persistence of slot information
type SlotManager struct {
	mu                       sync.RWMutex
	slots                    map[uint64]string // slot ID --> slot information JSON
	batchSize                int
	redisClient              *redis.Client
	protocolStateAddr        common.Address
	snapshotterStateAddr     common.Address
	snapshotterStateContract *contract.SnapshotterStateContract
}

// NewSlotManager creates a new SlotManager
func NewSlotManager(
	batchSize int,
	redisClient *redis.Client,
	protocolStateAddr common.Address,
	snapshotterStateAddr common.Address,
	snapshotterStateContract *contract.SnapshotterStateContract,
) *SlotManager {
	return &SlotManager{
		slots:                    make(map[uint64]string),
		batchSize:                batchSize,
		redisClient:              redisClient,
		protocolStateAddr:        protocolStateAddr,
		snapshotterStateAddr:     snapshotterStateAddr,
		snapshotterStateContract: snapshotterStateContract,
	}
}

// AddSlot adds a slot to the batch and returns true if batch size was reached
func (sm *SlotManager) AddSlot(slotID uint64, slotData string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.slots[slotID] = slotData

	if len(sm.slots) >= sm.batchSize {
		sm.flushSlots(context.Background())
		return true
	}
	return false
}

// flushSlots persists all current slots to Redis with namespaced keys
func (sm *SlotManager) flushSlots(ctx context.Context) {
	if len(sm.slots) == 0 {
		return
	}

	for slotID, slotData := range sm.slots {
		slotKey := sm.getSlotKey(slotID)
		if err := sm.redisClient.Set(ctx, slotKey, slotData, 0).Err(); err != nil {
			log.Errorf("Failed to persist slot %d to Redis: %v", slotID, err)
		} else {
			log.Debugf("Persisted slot %d to Redis with key: %s", slotID, slotKey)
		}
	}

	log.Infof("Flushed batch of %d slots to Redis", len(sm.slots))
	sm.slots = make(map[uint64]string)
}

// ForceFlush forces a flush of current slots even if batch size hasn't been reached
func (sm *SlotManager) ForceFlush(ctx context.Context) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.slots) > 0 {
		sm.flushSlots(ctx)
	}
}

// FetchSlot fetches a single slot from the contract and stores it in Redis
func (sm *SlotManager) FetchSlot(ctx context.Context, slotID uint64) error {
	// Call NodeInfo on SnapshotterState contract
	nodeInfo, err := sm.snapshotterStateContract.NodeInfo(&bind.CallOpts{Context: ctx}, big.NewInt(int64(slotID)))
	if err != nil {
		return fmt.Errorf("failed to fetch nodeInfo for slot %d: %w", slotID, err)
	}

	// Convert to SlotInfo struct (handles both legacy and new contracts)
	slotInfo := SlotInfo{
		SnapshotterAddress: nodeInfo.SnapshotterAddress,
		NodePrice:          nodeInfo.NodePrice,
		AmountSentOnL1:     nodeInfo.AmountSentOnL1,
		MintedOn:           nodeInfo.MintedOn,
		BurnedOn:           nodeInfo.BurnedOn,
		LastUpdated:        nodeInfo.LastUpdated,
		IsLegacy:           nodeInfo.IsLegacy,
		ClaimedTokens:      nodeInfo.ClaimedTokens,
		Active:             nodeInfo.Active,
		IsKyced:            nodeInfo.IsKyced,
		// Note: libP2pAddress is not available in legacy contract NodeInfo return
		// If needed for new contract, we'd need to check contract version or call a different method
	}

	// Marshal to JSON
	slotData, err := json.Marshal(slotInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal slot info for slot %d: %w", slotID, err)
	}

	// Store in Redis with namespaced key
	slotKey := sm.getSlotKey(slotID)
	if err := sm.redisClient.Set(ctx, slotKey, string(slotData), 0).Err(); err != nil {
		return fmt.Errorf("failed to store slot %d in Redis: %w", slotID, err)
	}

	log.Debugf("Fetched and stored slot %d: %s", slotID, slotKey)
	return nil
}

// FetchAllSlots fetches all slots from the contract in batches
func (sm *SlotManager) FetchAllSlots(ctx context.Context, totalNodeCount uint64) error {
	return sm.FetchAllSlotsWithProgress(ctx, totalNodeCount, nil)
}

// FetchAllSlotsWithProgress fetches all slots with progress callback
func (sm *SlotManager) FetchAllSlotsWithProgress(ctx context.Context, totalNodeCount uint64, progressCallback func(current, total uint64)) error {
	log.Infof("Fetching all slot information (total nodes: %d)...", totalNodeCount)

	var fetchedCount uint64
	var mu sync.Mutex

	// Process slots in batches
	for i := uint64(0); i <= totalNodeCount; i += uint64(sm.batchSize) {
		var wg sync.WaitGroup
		batchEnd := i + uint64(sm.batchSize)
		if batchEnd > totalNodeCount+1 {
			batchEnd = totalNodeCount + 1
		}

		// Fetch each slot in the current batch
		for j := i; j < batchEnd; j++ {
			wg.Add(1)
			go func(slotID uint64) {
				defer wg.Done()
				if err := sm.FetchSlot(ctx, slotID); err != nil {
					log.Errorf("Error fetching slot %d: %v", slotID, err)
				} else {
					mu.Lock()
					fetchedCount++
					current := fetchedCount
					mu.Unlock()

					// Call progress callback if provided
					if progressCallback != nil {
						progressCallback(current, totalNodeCount+1)
					}
				}
			}(j)
		}
		wg.Wait()
	}

	log.Infof("✅ Completed fetching all slots (%d total)", fetchedCount)
	return nil
}

// getSlotKey generates the namespaced Redis key for a slot
// Format: {protocolState}:{snapshotterState}:SlotInfo.{slotId}
func (sm *SlotManager) getSlotKey(slotID uint64) string {
	return fmt.Sprintf("%s:%s:SlotInfo.%d",
		sm.protocolStateAddr.Hex(),
		sm.snapshotterStateAddr.Hex(),
		slotID)
}

// GetSlotKey is a helper function to generate slot key (used by SlotValidator)
func GetSlotKey(protocolStateAddr, snapshotterStateAddr common.Address, slotID uint64) string {
	return fmt.Sprintf("%s:%s:SlotInfo.%d",
		protocolStateAddr.Hex(),
		snapshotterStateAddr.Hex(),
		slotID)
}
