package protocolstate

import (
	"context"
	"fmt"
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
	SlotSyncInterval         time.Duration // Fallback cold sync interval
	SlotSyncBatchSize        int
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

	// Create event processor
	cacher.eventProcessor = NewEventProcessor(
		ctx,
		snapshotterStateContract,
		slotManager,
		protocolStateAddr,
		snapshotterStateAddr,
	)

	return cacher, nil
}

// Start starts the cacher component background services
// Note: Cold sync should be performed synchronously via WaitForColdSync() before calling Start()
func (c *Cacher) Start(ctx context.Context) {
	log.Info("🚀 Starting protocol state cacher background services...")

	// Start event-driven updates
	go func() {
		if err := c.eventProcessor.WatchEvents(); err != nil {
			log.Errorf("Event processor error: %v", err)
		}
	}()

	// Start periodic fallback cold sync
	go c.periodicColdSync(ctx)

	log.Info("✅ Protocol state cacher background services started")
}

// WaitForColdSync checks if cold sync is needed and waits for it to complete
// Returns error if cold sync fails or context is cancelled
func (c *Cacher) WaitForColdSync(ctx context.Context) error {
	// Check if cold sync is needed
	needsSync, err := c.needsColdSync(ctx)
	if err != nil {
		return fmt.Errorf("failed to check cold sync status: %w", err)
	}

	if !needsSync {
		lastSync, _ := c.getLastSyncTimestamp(ctx)
		log.Infof("✅ Cold sync up to date (last sync: %s)", lastSync.Format(time.RFC3339))
		return nil
	}

	log.Info("⏳ Cold sync required - waiting for completion...")

	// Perform synchronous cold sync
	if err := c.coldSyncSync(ctx); err != nil {
		return fmt.Errorf("cold sync failed: %w", err)
	}

	log.Info("✅ Cold sync completed successfully")
	return nil
}

// needsColdSync checks if cold sync is needed based on last sync timestamp
func (c *Cacher) needsColdSync(ctx context.Context) (bool, error) {
	lastSync, err := c.getLastSyncTimestamp(ctx)
	if err != nil {
		// No timestamp found = first time setup
		return true, nil
	}

	// Check if last sync is older than the sync interval
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

// periodicColdSync performs periodic fallback cold sync
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
