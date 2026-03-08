package blockpoller

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	rpchelper "github.com/powerloom/go-rpc-helper"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// FilterQuery defines the contract addresses and event topics a consumer wants to watch.
type FilterQuery struct {
	Addresses []common.Address
	Topics    [][]common.Hash
}

// LogHandler is called with matched logs and the current block number for freshness checks.
type LogHandler func(logs []types.Log, currentBlock uint64)

// EventConsumer represents a registered event monitoring subscription.
// Each consumer tracks its own block progress independently so that one
// consumer's failure does not stall another.
type EventConsumer struct {
	Name    string
	Queries []FilterQuery
	Handler LogHandler
	Cadence int // run every N poll cycles (1 = every cycle)

	// Per-consumer block tracking (persisted to Redis)
	lastProcessedBlock uint64
	redisKey           string
	mu                 sync.Mutex
}

// LastProcessedBlock returns the consumer's last successfully processed block.
func (c *EventConsumer) LastProcessedBlock() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastProcessedBlock
}

// BlockPoller polls BlockNumber at a fixed interval and runs registered
// consumers' FilterLogs queries at their configured cadence.
type BlockPoller struct {
	rpcHelper    *rpchelper.RPCHelper
	pollInterval time.Duration
	consumers    []*EventConsumer
	latestBlock  uint64
	redisClient  *redis.Client
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.RWMutex
	cycleCount   uint64

	// Max blocks to scan per FilterLogs call to avoid overwhelming the RPC node.
	maxBlockRange uint64
}

// Config holds configuration for creating a BlockPoller.
type Config struct {
	RPCHelper     *rpchelper.RPCHelper
	RedisClient   *redis.Client
	PollInterval  time.Duration
	MaxBlockRange uint64 // default 1000
}

// New creates a new BlockPoller. Call RegisterConsumer before Start.
func New(cfg *Config) (*BlockPoller, error) {
	if cfg.RPCHelper == nil {
		return nil, fmt.Errorf("RPCHelper is required")
	}
	if cfg.RedisClient == nil {
		return nil, fmt.Errorf("RedisClient is required")
	}

	maxRange := cfg.MaxBlockRange
	if maxRange == 0 {
		maxRange = 1000
	}

	interval := cfg.PollInterval
	if interval == 0 {
		interval = 1 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &BlockPoller{
		rpcHelper:     cfg.RPCHelper,
		pollInterval:  interval,
		consumers:     make([]*EventConsumer, 0),
		redisClient:   cfg.RedisClient,
		ctx:           ctx,
		cancel:        cancel,
		maxBlockRange: maxRange,
	}, nil
}

// RegisterConsumer adds an event consumer. Must be called before Start.
// redisKeyPrefix is used to build the Redis persistence key for block tracking.
func (bp *BlockPoller) RegisterConsumer(name string, queries []FilterQuery, handler LogHandler, cadence int, redisKeyPrefix string) *EventConsumer {
	if cadence < 1 {
		cadence = 1
	}
	consumer := &EventConsumer{
		Name:     name,
		Queries:  queries,
		Handler:  handler,
		Cadence:  cadence,
		redisKey: fmt.Sprintf("%s:BlockPoller.%s.LastBlock", redisKeyPrefix, name),
	}

	// Try to restore persisted block from Redis
	ctx := context.Background()
	val, err := bp.redisClient.Get(ctx, consumer.redisKey).Result()
	if err == nil {
		if block, parseErr := strconv.ParseUint(val, 10, 64); parseErr == nil {
			consumer.lastProcessedBlock = block
			log.Infof("BlockPoller: restored consumer %q last block from Redis: %d", name, block)
		}
	}

	bp.mu.Lock()
	bp.consumers = append(bp.consumers, consumer)
	bp.mu.Unlock()

	log.Infof("BlockPoller: registered consumer %q (cadence=%d, queries=%d, redisKey=%s)",
		name, cadence, len(queries), consumer.redisKey)
	return consumer
}

// InitializeConsumerBlock sets the starting block for a consumer if it has no
// persisted value. Call this after RegisterConsumer and before Start for consumers
// that need a specific start block.
func (bp *BlockPoller) InitializeConsumerBlock(consumer *EventConsumer, startBlock uint64) {
	consumer.mu.Lock()
	defer consumer.mu.Unlock()
	if consumer.lastProcessedBlock == 0 {
		consumer.lastProcessedBlock = startBlock
		log.Infof("BlockPoller: initialized consumer %q start block to %d", consumer.Name, startBlock)
	}
}

// LatestBlock returns the most recently observed block number.
func (bp *BlockPoller) LatestBlock() uint64 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.latestBlock
}

// Start begins the polling loop. Blocks until context is cancelled.
func (bp *BlockPoller) Start() {
	log.Infof("BlockPoller: starting with %d consumers, poll interval %v", len(bp.consumers), bp.pollInterval)

	ticker := time.NewTicker(bp.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bp.ctx.Done():
			log.Info("BlockPoller: stopped")
			return
		case <-ticker.C:
			bp.poll()
		}
	}
}

// Stop cancels the polling loop.
func (bp *BlockPoller) Stop() {
	bp.cancel()
}

// poll executes one polling cycle: fetch BlockNumber, then run eligible consumers.
func (bp *BlockPoller) poll() {
	bp.cycleCount++

	currentBlock, err := bp.rpcHelper.BlockNumber(bp.ctx)
	if err != nil {
		log.Errorf("BlockPoller: failed to get block number: %v", err)
		return
	}

	bp.mu.Lock()
	bp.latestBlock = currentBlock
	bp.mu.Unlock()

	bp.mu.RLock()
	consumers := bp.consumers
	bp.mu.RUnlock()

	for _, consumer := range consumers {
		// Check cadence
		if bp.cycleCount%uint64(consumer.Cadence) != 0 {
			continue
		}
		bp.runConsumer(consumer, currentBlock)
	}
}

// runConsumer executes FilterLogs for a single consumer and advances its block
// counter only when ALL queries succeed.
func (bp *BlockPoller) runConsumer(consumer *EventConsumer, currentBlock uint64) {
	consumer.mu.Lock()
	lastBlock := consumer.lastProcessedBlock
	consumer.mu.Unlock()

	if lastBlock >= currentBlock {
		return
	}

	toBlock := lastBlock + bp.maxBlockRange
	if toBlock > currentBlock {
		toBlock = currentBlock
	}

	fromBlock := lastBlock + 1

	var allLogs []types.Log
	allSucceeded := true

	for _, q := range consumer.Queries {
		query := ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(fromBlock),
			ToBlock:   new(big.Int).SetUint64(toBlock),
			Addresses: q.Addresses,
			Topics:    q.Topics,
		}

		logs, err := bp.rpcHelper.FilterLogs(bp.ctx, query)
		if err != nil {
			log.Warnf("BlockPoller: consumer %q FilterLogs failed (blocks %d-%d): %v",
				consumer.Name, fromBlock, toBlock, err)
			allSucceeded = false
			break
		}
		allLogs = append(allLogs, logs...)
	}

	if !allSucceeded {
		return
	}

	// Dispatch logs to handler
	if len(allLogs) > 0 {
		consumer.Handler(allLogs, currentBlock)
	}

	// Advance block counter and persist
	consumer.mu.Lock()
	consumer.lastProcessedBlock = toBlock
	consumer.mu.Unlock()

	if err := bp.redisClient.Set(bp.ctx, consumer.redisKey, strconv.FormatUint(toBlock, 10), 0).Err(); err != nil {
		log.Warnf("BlockPoller: failed to persist block %d for consumer %q: %v", toBlock, consumer.Name, err)
	}
}

// GetConsumerStatus returns a map of consumer name -> last processed block for monitoring.
func (bp *BlockPoller) GetConsumerStatus() map[string]uint64 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	status := make(map[string]uint64, len(bp.consumers))
	for _, c := range bp.consumers {
		c.mu.Lock()
		status[c.Name] = c.lastProcessedBlock
		c.mu.Unlock()
	}
	return status
}
