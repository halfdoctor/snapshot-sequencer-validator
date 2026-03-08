package protocolstate

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	rpchelper "github.com/powerloom/go-rpc-helper"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/powerloom/snapshot-sequencer-validator/pkgs/protocolstate/contract"
)

// Config holds configuration for the protocol state cacher
type Config struct {
	RPCHelper                *rpchelper.RPCHelper
	ProtocolStateContract    string
	SnapshotterStateContract string
	ContractABIPath          string
	RedisClient              *redis.Client
	SlotSyncInterval         time.Duration // Interval for periodic node count check (was full cold sync)
	SlotSyncBatchSize        int

	// Smart sync configuration
	EventGapThresholdBlocks uint64 // Block gap that triggers full cold sync on startup (default: 5000)
	ForceFullColdSync       bool   // Escape hatch to force old hourly full-sync behavior
}

// Cacher is the main protocol state cacher component
type Cacher struct {
	config                   *Config
	ctx                      context.Context
	cancel                   context.CancelFunc
	slotManager              *SlotManager
	eventProcessor           *EventProcessor
	snapshotterStateContract *contract.SnapshotterStateContract
	protocolStateAddr        common.Address
	snapshotterStateAddr     common.Address
}

// NewCacher creates a new protocol state cacher
func NewCacher(cfg *Config) (*Cacher, error) {
	if cfg.RPCHelper == nil {
		return nil, fmt.Errorf("RPC helper is required")
	}
	if cfg.RedisClient == nil {
		return nil, fmt.Errorf("Redis client is required")
	}
	if cfg.ProtocolStateContract == "" {
		return nil, fmt.Errorf("ProtocolState contract address is required")
	}
	if cfg.SnapshotterStateContract == "" {
		return nil, fmt.Errorf("SnapshotterState contract address is required")
	}

	protocolStateAddr := common.HexToAddress(cfg.ProtocolStateContract)
	snapshotterStateAddr := common.HexToAddress(cfg.SnapshotterStateContract)

	// Initialize RPC helper if not already initialized
	ctx := context.Background()
	if err := cfg.RPCHelper.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize RPC helper: %w", err)
	}

	// Create contract backend from RPC helper
	contractBackend := cfg.RPCHelper.NewContractBackend()

	// Create SnapshotterState contract instance
	snapshotterStateContract, err := contract.NewSnapshotterStateContract(snapshotterStateAddr, contractBackend)
	if err != nil {
		return nil, fmt.Errorf("failed to create SnapshotterState contract instance: %w", err)
	}

	// Create slot manager
	slotManager := NewSlotManager(
		cfg.SlotSyncBatchSize,
		cfg.RedisClient,
		protocolStateAddr,
		snapshotterStateAddr,
		snapshotterStateContract,
	)

	ctx, cancel := context.WithCancel(context.Background())

	cacher := &Cacher{
		config:                   cfg,
		ctx:                      ctx,
		cancel:                   cancel,
		slotManager:              slotManager,
		snapshotterStateContract: snapshotterStateContract,
		protocolStateAddr:        protocolStateAddr,
		snapshotterStateAddr:     snapshotterStateAddr,
	}

	// Get SnapshotterState ABI for event parsing
	snapshotterStateABI, err := GetSnapshotterStateABI()
	if err != nil {
		return nil, fmt.Errorf("failed to get SnapshotterState ABI: %w", err)
	}

	// Create event processor with polling interval (default: 30 seconds)
	pollInterval := 30 * time.Second
	if cfg.SlotSyncInterval > 0 && cfg.SlotSyncInterval < pollInterval {
		// Use a fraction of sync interval for polling, but not less than 10 seconds
		pollInterval = cfg.SlotSyncInterval / 10
		if pollInterval < 10*time.Second {
			pollInterval = 10 * time.Second
		}
	}

	cacher.eventProcessor = NewEventProcessor(
		ctx,
		cfg.RPCHelper,
		snapshotterStateContract,
		snapshotterStateABI,
		slotManager,
		protocolStateAddr,
		snapshotterStateAddr,
		pollInterval,
	)

	return cacher, nil
}

// Start starts the cacher background services. Event processing is handled by
// the BlockPoller consumer; slot freshness is guaranteed by demand-driven
// re-fetch on address mismatch. The only periodic task is the escape-hatch
// full cold sync when ForceFullColdSync is set.
func (c *Cacher) Start(ctx context.Context) {
	log.Info("🚀 Starting protocol state cacher background services...")

	if c.config.ForceFullColdSync {
		go c.periodicColdSync(ctx)
	}

	log.Info("✅ Protocol state cacher background services started")
}

// GetEventProcessor returns the event processor for BlockPoller registration.
func (c *Cacher) GetEventProcessor() *EventProcessor {
	return c.eventProcessor
}

// RedisKeyPrefix returns the prefix used for this cacher's Redis keys,
// suitable for registering BlockPoller consumers.
func (c *Cacher) RedisKeyPrefix() string {
	return fmt.Sprintf("%s:%s", c.protocolStateAddr.Hex(), c.snapshotterStateAddr.Hex())
}

// WaitForColdSync performs smart startup sync decision:
//   - If InitialComplete not set: full cold sync (first ever startup)
//   - If InitialComplete set and ForceFullColdSync: full cold sync
//   - If InitialComplete set and block gap is small: skip (BlockPoller will catch up)
//   - If InitialComplete set and block gap is large or unknown: full cold sync
func (c *Cacher) WaitForColdSync(ctx context.Context) error {
	if c.config.ForceFullColdSync {
		log.Info("FORCE_FULL_COLD_SYNC=true - performing full cold sync")
		return c.performFullColdSync(ctx)
	}

	initialComplete := c.isInitialComplete(ctx)

	if !initialComplete {
		log.Info("First startup detected (InitialComplete not set) - performing full cold sync")
		if err := c.performFullColdSync(ctx); err != nil {
			return err
		}
		c.setInitialComplete(ctx)
		return nil
	}

	// InitialComplete is set - check if we can skip cold sync
	lastBlock, err := c.getPersistedSlotConsumerBlock(ctx)
	if err != nil {
		log.Warnf("Cannot read persisted slot consumer block (%v) - performing full cold sync", err)
		return c.performFullColdSync(ctx)
	}

	// Get current block to measure gap
	currentBlock, err := c.config.RPCHelper.BlockNumber(ctx)
	if err != nil {
		log.Warnf("Cannot get current block (%v) - performing full cold sync", err)
		return c.performFullColdSync(ctx)
	}

	threshold := c.config.EventGapThresholdBlocks
	if threshold == 0 {
		threshold = 5000
	}

	gap := uint64(0)
	if currentBlock > lastBlock {
		gap = currentBlock - lastBlock
	}

	if gap > threshold {
		log.Warnf("Block gap too large (%d > threshold %d) - performing full cold sync", gap, threshold)
		return c.performFullColdSync(ctx)
	}

	log.Infof("Block gap small (%d blocks < threshold %d), skipping full cold sync - BlockPoller will catch up", gap, threshold)
	
	// Set timestamp even when skipping cold sync so dependent components (dequeuer) don't wait forever
	if err := c.setLastSyncTimestamp(ctx); err != nil {
		log.Warnf("Failed to set last sync timestamp after skipped sync: %v", err)
	} else {
		log.Debug("Set last sync timestamp (full sync skipped, BlockPoller handling updates)")
	}
	
	return nil
}

// performFullColdSync executes a full cold sync and updates timestamps.
func (c *Cacher) performFullColdSync(ctx context.Context) error {
	log.Info("⏳ Cold sync required - waiting for completion...")
	if err := c.coldSyncSync(ctx); err != nil {
		return fmt.Errorf("cold sync failed: %w", err)
	}
	log.Info("✅ Cold sync completed successfully")
	return nil
}

// isInitialComplete checks if the persistent InitialComplete flag is set.
func (c *Cacher) isInitialComplete(ctx context.Context) bool {
	key := fmt.Sprintf("%s:%s:ColdSync.InitialComplete",
		c.protocolStateAddr.Hex(), c.snapshotterStateAddr.Hex())
	val, err := c.config.RedisClient.Get(ctx, key).Result()
	return err == nil && val == "true"
}

// setInitialComplete sets the persistent InitialComplete flag (no TTL).
func (c *Cacher) setInitialComplete(ctx context.Context) {
	key := fmt.Sprintf("%s:%s:ColdSync.InitialComplete",
		c.protocolStateAddr.Hex(), c.snapshotterStateAddr.Hex())
	if err := c.config.RedisClient.Set(ctx, key, "true", 0).Err(); err != nil {
		log.Warnf("Failed to set InitialComplete flag: %v", err)
	} else {
		log.Info("Set ColdSync.InitialComplete persistent flag")
	}
}

// getPersistedSlotConsumerBlock reads the BlockPoller's persisted block counter
// for the slot event consumer.
func (c *Cacher) getPersistedSlotConsumerBlock(ctx context.Context) (uint64, error) {
	prefix := fmt.Sprintf("%s:%s", c.protocolStateAddr.Hex(), c.snapshotterStateAddr.Hex())
	key := fmt.Sprintf("%s:BlockPoller.SlotEvents.LastBlock", prefix)
	val, err := c.config.RedisClient.Get(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	block, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid persisted block value: %w", err)
	}
	return block, nil
}

// needsColdSync checks if cold sync is needed based on last sync timestamp.
// Kept for backward compatibility with WaitForColdSyncCompletion.
func (c *Cacher) needsColdSync(ctx context.Context) (bool, error) {
	lastSync, err := c.getLastSyncTimestamp(ctx)
	if err != nil {
		return true, nil
	}
	age := time.Since(lastSync)
	if age > c.config.SlotSyncInterval {
		log.Warnf("Last cold sync was %v ago (threshold: %v) - sync required", age, c.config.SlotSyncInterval)
		return true, nil
	}
	return false, nil
}

// getLastSyncTimestamp retrieves the last cold sync timestamp from Redis
func (c *Cacher) getLastSyncTimestamp(ctx context.Context) (time.Time, error) {
	key := c.getLastSyncKey()
	timestampStr, err := c.config.RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return time.Time{}, fmt.Errorf("no last sync timestamp found")
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last sync timestamp: %w", err)
	}

	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return timestamp, nil
}

// setLastSyncTimestamp stores the last cold sync timestamp in Redis
func (c *Cacher) setLastSyncTimestamp(ctx context.Context) error {
	key := c.getLastSyncKey()
	timestamp := time.Now().Format(time.RFC3339)

	// Store with TTL slightly longer than sync interval to ensure it persists
	ttl := c.config.SlotSyncInterval + 1*time.Hour
	if err := c.config.RedisClient.Set(ctx, key, timestamp, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set last sync timestamp: %w", err)
	}

	return nil
}

// getLastSyncKey returns the Redis key for last sync timestamp
func (c *Cacher) getLastSyncKey() string {
	return fmt.Sprintf("%s:%s:ColdSync.LastSyncTimestamp",
		c.protocolStateAddr.Hex(),
		c.snapshotterStateAddr.Hex())
}

// Stop stops the cacher component
func (c *Cacher) Stop() {
	log.Info("Stopping protocol state cacher component...")
	c.cancel()

	// Force flush any pending slots
	c.slotManager.ForceFlush(context.Background())

	log.Info("Protocol state cacher component stopped")
}

// coldSync performs initial cold sync of all slots (async version)
func (c *Cacher) coldSync(ctx context.Context) {
	if err := c.coldSyncSync(ctx); err != nil {
		log.Errorf("Cold sync failed: %v", err)
	}
}

// coldSyncSync performs synchronous cold sync with progress indicators
func (c *Cacher) coldSyncSync(ctx context.Context) error {
	log.Info("🔄 Starting cold sync of all slots...")

	// Get total node count from SnapshotterState contract
	totalNodeCount, err := c.getTotalNodeCount(ctx)
	if err != nil {
		log.Errorf("Failed to get total node count: %v", err)
		log.Warn("Will attempt to sync slots by iterating until failure")
		// Try to sync up to a reasonable maximum
		totalNodeCount = 10000 // Fallback maximum
	}

	log.Infof("📊 Total node count: %d", totalNodeCount)

	// Fetch all slots with progress tracking
	if err := c.slotManager.FetchAllSlotsWithProgress(ctx, totalNodeCount, func(current, total uint64) {
		percent := float64(current) / float64(total) * 100
		if current%100 == 0 || current == total || current == 1 {
			log.Infof("📈 Cold sync progress: %d/%d slots (%.1f%%)", current, total, percent)
		}
	}); err != nil {
		return fmt.Errorf("failed to fetch all slots: %w", err)
	}

	// Update last sync timestamp
	if err := c.setLastSyncTimestamp(ctx); err != nil {
		log.Warnf("Failed to update last sync timestamp: %v", err)
	} else {
		log.Info("✅ Cold sync completed and timestamp updated")
	}

	return nil
}

// periodicColdSync performs periodic fallback cold sync (only used with ForceFullColdSync).
func (c *Cacher) periodicColdSync(ctx context.Context) {
	ticker := time.NewTicker(c.config.SlotSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Info("Starting periodic fallback cold sync...")
			c.coldSync(ctx)
		case <-ctx.Done():
			return
		case <-c.ctx.Done():
			return
		}
	}
}


// getTotalNodeCount gets the total node count from SnapshotterState contract
func (c *Cacher) getTotalNodeCount(ctx context.Context) (uint64, error) {
	// Use NodeCount() method from contract bindings
	nodeCount, err := c.snapshotterStateContract.NodeCount(&bind.CallOpts{Context: ctx})
	if err != nil {
		return 0, fmt.Errorf("failed to call NodeCount(): %w", err)
	}

	if nodeCount == nil {
		return 0, fmt.Errorf("NodeCount() returned nil")
	}

	return nodeCount.Uint64(), nil
}

// GetSlotManager returns the SlotManager for on-demand slot fetching
// This allows other components (e.g., Dequeuer) to access the SlotManager
// for fetching missing slots from the contract when Redis cache misses occur
func (c *Cacher) GetSlotManager() *SlotManager {
	return c.slotManager
}
