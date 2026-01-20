package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/powerloom/snapshot-sequencer-validator/config"
	abiloader "github.com/powerloom/snapshot-sequencer-validator/pkgs/abi"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/consensus"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/ipfs"
	rediskeys "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/submissions"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/utils"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/vpa"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

type Aggregator struct {
	ctx         context.Context
	cancel      context.CancelFunc
	redisClient *redis.Client
	ipfsClient  *ipfs.Client
	config      *config.Settings
	keyBuilders map[string]*rediskeys.KeyBuilder // dataMarket -> KeyBuilder (for multi-market support)

	// Contract clients for on-chain integration
	vpaClients map[string]*vpa.PriorityCachingClient // dataMarket -> VPA client (one per data market)
	rpcClient  *ethclient.Client                     // Ethereum RPC client for on-chain checks

	// relayer-py integration
	relayerPyEndpoint string       // relayer-py service endpoint
	httpClient        *http.Client // HTTP client for relayer communication

	// Track aggregation state (using composite keys: dataMarket:epochID)
	epochTimers map[string]*time.Timer // "dataMarket:epochID" -> aggregation window timer

	// Track submission state (using composite keys: dataMarket:epochID)
	submissionState map[string]bool // "dataMarket:epochID" -> submitted

	mu sync.RWMutex
}

func NewAggregator(cfg *config.Settings) (*Aggregator, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize Redis
	redisOpts := &redis.Options{
		Addr: fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		DB:   cfg.RedisDB,
	}
	// Only set password if it's not empty (trim spaces first)
	password := strings.TrimSpace(cfg.RedisPassword)
	if password != "" {
		redisOpts.Password = password
	}
	redisClient := redis.NewClient(redisOpts)

	if err := redisClient.Ping(ctx).Err(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize IPFS client (optional)
	var ipfsClient *ipfs.Client
	if cfg.IPFSAPI != "" {
		client, err := ipfs.NewClient(cfg.IPFSAPI)
		if err != nil {
			log.WithError(err).Warn("Failed to connect to IPFS, continuing without it")
		} else {
			ipfsClient = client
		}
	}

	// Initialize VPA clients for priority checking (one per data market)
	vpaClients := make(map[string]*vpa.PriorityCachingClient)

	if cfg.EnableOnChainSubmission {
		log.Info("🔗 Initializing VPA clients for priority checking")

		// Fetch VPA address from ProtocolState contract if not provided
		vpaContractAddr := common.HexToAddress(cfg.VPAContractAddress)
		if vpaContractAddr == (common.Address{}) {
			log.Infof("🔍 Fetching VPA address from ProtocolState contract...")

			// Use shared VPA fetching function
			rpcURL := cfg.RPCNodes[0]
			fetchedVPAAddress, err := vpa.FetchVPAAddress(rpcURL, cfg.ProtocolStateContract)
			if err != nil {
				log.Warnf("⚠️  Failed to fetch VPA address: %v", err)
				vpaContractAddr = common.Address{}
			} else {
				vpaContractAddr = fetchedVPAAddress
				log.Infof("✅ Successfully fetched VPA address: %s", vpaContractAddr.Hex())
			}
		}

		// Initialize VPA caching client for each data market
		if vpaContractAddr != (common.Address{}) && cfg.VPAValidatorAddress != "" {
			rpcURL := cfg.RPCNodes[0]
			for _, dataMarket := range cfg.DataMarketAddresses {
				checksummedMarket := common.HexToAddress(dataMarket).Hex()
				vpaClient, err := vpa.NewPriorityCachingClient(
					rpcURL, vpaContractAddr.Hex(), cfg.VPAValidatorAddress,
					redisClient, cfg.ProtocolStateContract, checksummedMarket, "")
				if err != nil {
					cancel()
					return nil, fmt.Errorf("failed to initialize VPA caching client for data market %s: %w", checksummedMarket, err)
				}
				vpaClients[checksummedMarket] = vpaClient
				log.WithField("data_market", checksummedMarket).Info("✅ VPA caching client initialized")
			}
		} else {
			log.Warn("⚠️  VPA contract address or validator address not available")
		}
	}

	// Initialize Ethereum RPC client for on-chain checks (used to verify if submissions already exist)
	var rpcClient *ethclient.Client
	if cfg.EnableOnChainSubmission && len(cfg.RPCNodes) > 0 {
		client, err := ethclient.Dial(cfg.RPCNodes[0])
		if err != nil {
			log.Warnf("⚠️  Failed to initialize RPC client for on-chain checks: %v", err)
		} else {
			rpcClient = client
			log.Info("✅ RPC client initialized for on-chain submission checks")
		}
	}

	aggregator := &Aggregator{
		ctx:               ctx,
		cancel:            cancel,
		redisClient:       redisClient,
		ipfsClient:        ipfsClient,
		config:            cfg,
		keyBuilders:       make(map[string]*rediskeys.KeyBuilder),
		vpaClients:        vpaClients,
		rpcClient:         rpcClient,
		relayerPyEndpoint: cfg.RelayerPyEndpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		epochTimers:     make(map[string]*time.Timer),
		submissionState: make(map[string]bool),
	}

	// Initialize stream consumers for all data markets (mandatory for deterministic aggregation)
	if err := aggregator.initializeStreamConsumers(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize stream consumers: %w", err)
	}

	return aggregator, nil
}

// initializeStreamConsumers sets up stream consumers for all configured data markets
func (a *Aggregator) initializeStreamConsumers() error {
	groupName := a.config.StreamConsumerGroup
	consumerName := a.config.StreamConsumerName

	// Initialize stream consumer for each data market
	for _, dataMarket := range a.config.DataMarketAddresses {
		// Normalize to checksummed format
		checksummedMarket := common.HexToAddress(dataMarket).Hex()
		kb := a.getKeyBuilder(checksummedMarket)
		streamKey := kb.AggregationStream()

		log.WithFields(logrus.Fields{
			"stream":      streamKey,
			"data_market": checksummedMarket,
			"group":       groupName,
			"consumer":    consumerName,
		}).Info("Initializing Redis stream consumer")

		// Ensure consumer group exists (create with stream if needed)
		err := a.redisClient.XGroupCreateMkStream(a.ctx, streamKey, groupName, "0").Err()
		if err != nil {
			if err.Error() != "BUSYGROUP Consumer Group name already exists" {
				return fmt.Errorf("failed to create consumer group for data market %s: %w", checksummedMarket, err)
			}
			log.WithFields(logrus.Fields{
				"group":       groupName,
				"data_market": checksummedMarket,
			}).Info("Consumer group already exists")
		}

		// Start stream consumer goroutine for this data market
		go a.consumeStreamMessages(checksummedMarket, kb)
	}

	// Start consumer health monitoring
	go a.monitorConsumerHealth()

	return nil
}

// consumeStreamMessages consumes messages from the aggregation stream for a specific data market
func (a *Aggregator) consumeStreamMessages(dataMarket string, kb *rediskeys.KeyBuilder) {
	streamKey := kb.AggregationStream()
	groupName := a.config.StreamConsumerGroup
	consumerName := a.config.StreamConsumerName

	log.WithFields(logrus.Fields{
		"stream":      streamKey,
		"data_market": dataMarket,
		"group":       groupName,
		"consumer":    consumerName,
	}).Info("Starting stream consumer")

	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			// Read messages from stream
			messages, err := a.redisClient.XReadGroup(a.ctx, &redis.XReadGroupArgs{
				Group:    groupName,
				Consumer: consumerName,
				Streams:  []string{streamKey, ">"},
				Count:    int64(a.config.StreamBatchSize),
				Block:    a.config.StreamReadBlock,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					// No messages available, continue
					continue
				}
				if a.ctx.Err() != nil {
					// Context cancelled, exit
					return
				}
				log.WithError(err).WithFields(logrus.Fields{
					"stream":      streamKey,
					"data_market": dataMarket,
				}).Error("Failed to read from stream")
				time.Sleep(5 * time.Second) // Back off on error
				continue
			}

			// Process received messages
			for _, stream := range messages {
				for _, message := range stream.Messages {
					if err := a.processStreamMessage(message); err != nil {
						log.WithError(err).WithFields(logrus.Fields{
							"message_id":  message.ID,
							"stream":      streamKey,
							"data_market": dataMarket,
						}).Error("Failed to process stream message")

						// Move problematic message to dead letter queue
						a.moveToDeadLetterQueue(streamKey, groupName, message, dataMarket)
					}
				}
			}
		}
	}
}

// processStreamMessage processes a single stream message
func (a *Aggregator) processStreamMessage(message redis.XMessage) error {
	// Extract message fields
	epoch, ok := message.Values["epoch"].(string)
	if !ok {
		return fmt.Errorf("missing epoch field in message")
	}

	validator, ok := message.Values["validator"].(string)
	if !ok {
		return fmt.Errorf("missing validator field in message")
	}

	_, ok = message.Values["batch_key"].(string)
	if !ok {
		return fmt.Errorf("missing batch_key field in message")
	}

	timestamp, ok := message.Values["timestamp"].(string)
	if !ok {
		return fmt.Errorf("missing timestamp field in message")
	}

	msgType, ok := message.Values["type"].(string)
	if !ok {
		return fmt.Errorf("missing type field in message")
	}

	// Extract data_market (mandatory field)
	dataMarketRaw, ok := message.Values["data_market"]
	if !ok {
		log.Warnf("Skipping stream message without data_market field - old format not supported: %v", message.ID)
		return nil
	}
	dataMarketStr, ok := dataMarketRaw.(string)
	if !ok || dataMarketStr == "" {
		log.Warnf("Skipping stream message without data_market field - old format not supported: %v", message.ID)
		return nil
	}
	// Normalize to checksummed format
	dataMarket := common.HexToAddress(dataMarketStr).Hex()

	log.WithFields(logrus.Fields{
		"message_id":  message.ID,
		"epoch":       epoch,
		"validator":   validator,
		"type":        msgType,
		"timestamp":   timestamp,
		"data_market": dataMarket,
	}).Debug("Processing stream message")

	// Only process validator batch messages
	if msgType != "validator_batch" {
		log.WithField("type", msgType).Debug("Ignoring non-validator-batch message")
		return nil
	}

	// Get appropriate KeyBuilder for this data market
	kb := a.getKeyBuilder(dataMarket)

	// Check if epoch is already aggregated
	aggregatedKey := kb.BatchAggregated(epoch)
	exists, err := a.redisClient.Exists(a.ctx, aggregatedKey).Result()
	if err != nil {
		return fmt.Errorf("failed to check aggregation status: %w", err)
	}

	if exists > 0 {
		log.WithField("epoch", epoch).Debug("Epoch already aggregated, skipping message")
		return nil
	}

	// Start or extend aggregation window for this epoch (with data market)
	a.startAggregationWindow(epoch, dataMarket)

	return nil
}

// moveToDeadLetterQueue moves problematic messages to a dead letter queue
func (a *Aggregator) moveToDeadLetterQueue(streamKey, groupName string, message redis.XMessage, dataMarket string) {
	deadLetterKey := streamKey + ":dlq"

	// Add message to dead letter queue with metadata
	dlqData := map[string]interface{}{
		"original_id":    message.ID,
		"values":         message.Values,
		"error_time":     time.Now().Unix(),
		"consumer_group": groupName,
		"error_reason":   "processing_failed",
		"data_market":    dataMarket,
	}

	if err := a.redisClient.XAdd(a.ctx, &redis.XAddArgs{
		Stream: deadLetterKey,
		Values: dlqData,
	}).Err(); err != nil {
		log.WithError(err).Error("Failed to add message to dead letter queue")
	}

	// Acknowledge the original message to remove it from the pending list
	a.redisClient.XAck(a.ctx, streamKey, groupName, message.ID)

	log.WithFields(logrus.Fields{
		"message_id":  message.ID,
		"dead_letter": deadLetterKey,
		"data_market": dataMarket,
	}).Warn("Moved problematic message to dead letter queue")
}

// monitorConsumerHealth monitors the health of stream consumers for all data markets
func (a *Aggregator) monitorConsumerHealth() {
	ticker := time.NewTicker(120 * time.Second) // Check every 2 minutes
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			groupName := a.config.StreamConsumerGroup
			consumerName := a.config.StreamConsumerName

			// Monitor health for each data market stream
			for _, dataMarket := range a.config.DataMarketAddresses {
				checksummedMarket := common.HexToAddress(dataMarket).Hex()
				kb := a.getKeyBuilder(checksummedMarket)
				streamKey := kb.AggregationStream()

				// Check consumer info
				consumers, err := a.redisClient.XInfoConsumers(a.ctx, streamKey, groupName).Result()
				if err != nil {
					log.WithError(err).WithField("data_market", checksummedMarket).Error("Failed to get consumer info")
					continue
				}

				// Find our consumer
				var ourConsumer *redis.XInfoConsumer
				for _, consumer := range consumers {
					if consumer.Name == consumerName {
						ourConsumer = &consumer
						break
					}
				}

				if ourConsumer == nil {
					log.WithFields(logrus.Fields{
						"consumer":    consumerName,
						"data_market": checksummedMarket,
					}).Warn("Our consumer not found in group")
					continue
				}

				// Log consumer health
				log.WithFields(logrus.Fields{
					"consumer":    consumerName,
					"data_market": checksummedMarket,
					"pending":     ourConsumer.Pending,
					"idle":        ourConsumer.Idle,
				}).Debug("Consumer health check")

				// Check for long idle time (potential consumer stall)
				if time.Duration(ourConsumer.Idle) > a.config.StreamIdleTimeout {
					log.WithFields(logrus.Fields{
						"consumer":    consumerName,
						"data_market": checksummedMarket,
						"idle":        ourConsumer.Idle,
						"pending":     ourConsumer.Pending,
					}).Warn("Consumer appears stalled")

					// Attempt to claim stalled messages
					a.claimStalledMessages(streamKey, groupName, consumerName)
				}
			}
		}
	}
}

// claimStalledMessages claims messages that have been pending too long
func (a *Aggregator) claimStalledMessages(streamKey, groupName, consumerName string) {
	// Get pending messages for our consumer
	pending, err := a.redisClient.XPendingExt(a.ctx, &redis.XPendingExtArgs{
		Stream:   streamKey,
		Group:    groupName,
		Start:    "-",
		End:      "+",
		Count:    10, // Process in batches
		Consumer: consumerName,
	}).Result()

	if err != nil {
		log.WithError(err).Error("Failed to get pending messages")
		return
	}

	// Claim messages that have been idle longer than the timeout
	minIdleTime := a.config.StreamIdleTimeout
	claimedCount := 0

	for _, pendingMsg := range pending {
		if pendingMsg.Idle >= minIdleTime {
			// Claim the message for ourselves
			messages, err := a.redisClient.XClaim(a.ctx, &redis.XClaimArgs{
				Stream:   streamKey,
				Group:    groupName,
				Consumer: consumerName,
				MinIdle:  minIdleTime,
				Messages: []string{pendingMsg.ID},
			}).Result()

			if err != nil {
				log.WithError(err).WithField("message_id", pendingMsg.ID).Error("Failed to claim message")
				continue
			}

			if len(messages) > 0 {
				claimedCount++
				log.WithField("message_id", messages[0].ID).Info("Claimed stalled message")

				// Process the claimed message
				if err := a.processStreamMessage(messages[0]); err != nil {
					log.WithError(err).WithField("message_id", messages[0].ID).Error("Failed to process claimed message")
				}
			}
		}
	}

	if claimedCount > 0 {
		log.WithField("claimed_count", claimedCount).Info("Processed stalled messages")
	}
}

func (a *Aggregator) processAggregationQueue() {
	// Start a separate goroutine for each data market's queue
	for _, dataMarket := range a.config.DataMarketAddresses {
		checksummedMarket := common.HexToAddress(dataMarket).Hex()
		go a.processAggregationQueueForMarket(checksummedMarket)
	}
}

// processAggregationQueueForMarket processes Level 1 aggregation queue for a specific data market
func (a *Aggregator) processAggregationQueueForMarket(dataMarket string) {
	kb := a.getKeyBuilder(dataMarket)
	queueKey := kb.AggregationQueueLevel1()

	log.WithFields(logrus.Fields{
		"data_market":    dataMarket,
		"queue_key":      queueKey,
		"protocol_state": a.config.ProtocolStateContract,
	}).Info("📋 Started Level 1 aggregation queue processor")

	loopCount := 0
	for {
		loopCount++
		select {
		case <-a.ctx.Done():
			log.WithFields(logrus.Fields{
				"queue":       queueKey,
				"data_market": dataMarket,
				"loop_count":  loopCount,
			}).Info("Queue processor exiting - context cancelled")
			return
		default:
			// BRPop blocks until a message is available or timeout
			if loopCount%60 == 0 {
				// Log every 60 iterations (roughly every minute) to show loop is alive
				log.WithFields(logrus.Fields{
					"queue":       queueKey,
					"data_market": dataMarket,
					"loop_count":  loopCount,
				}).Debug("Queue processor still polling (no messages yet)")
			}
			result, err := a.redisClient.BRPop(a.ctx, time.Second, queueKey).Result()
			if err != nil {
				if err == redis.Nil {
					// Timeout - queue empty, continue polling
					continue
				}
				if a.ctx.Err() != nil {
					log.WithFields(logrus.Fields{
						"queue":       queueKey,
						"data_market": dataMarket,
						"loop_count":  loopCount,
					}).Info("Queue processor exiting - context error")
					return
				}
				log.WithError(err).WithFields(logrus.Fields{
					"queue":       queueKey,
					"data_market": dataMarket,
					"loop_count":  loopCount,
				}).Error("Failed to read from aggregation queue")
				time.Sleep(1 * time.Second)
				continue
			}

			if len(result) < 2 {
				log.WithFields(logrus.Fields{
					"result_length": len(result),
					"result":        result,
					"queue":         queueKey,
				}).Warn("Invalid result from BRPop - missing data")
				continue
			}

			log.WithFields(logrus.Fields{
				"queue": queueKey,
				"data":  result[1],
			}).Info("📨 Received message from aggregation queue")

			// Parse the complex JSON from finalizer workers
			var aggData map[string]interface{}
			if err := json.Unmarshal([]byte(result[1]), &aggData); err != nil {
				log.WithError(err).WithFields(logrus.Fields{
					"queue": queueKey,
					"data":  result[1],
				}).Error("Failed to parse aggregation data")
				continue
			}

			epochIDStr, ok := aggData["epoch_id"].(string)
			if !ok {
				log.WithFields(logrus.Fields{
					"data":  aggData,
					"queue": queueKey,
				}).Error("Missing epoch_id in aggregation data")
				continue
			}

			partsCompletedFloat, ok := aggData["parts_completed"].(float64)
			if !ok {
				log.WithFields(logrus.Fields{
					"data":  aggData,
					"queue": queueKey,
				}).Error("Missing or invalid parts_completed in aggregation data")
				continue
			}
			partsCompleted := int(partsCompletedFloat)

			// Extract data_market from queue message (should match, but verify)
			var dataMarketFromMsg string
			if dataMarketRaw, ok := aggData["data_market"]; ok {
				if dataMarketStr, ok := dataMarketRaw.(string); ok && dataMarketStr != "" {
					dataMarketFromMsg = common.HexToAddress(dataMarketStr).Hex()
				}
			}

			// Use data market from message if available, otherwise use the one we're processing
			finalDataMarket := dataMarket
			if dataMarketFromMsg != "" {
				if dataMarketFromMsg != dataMarket {
					log.WithFields(logrus.Fields{
						"queue_data_market": dataMarketFromMsg,
						"expected":          dataMarket,
						"queue":             queueKey,
					}).Warn("⚠️  Data market mismatch in queue message")
				}
				finalDataMarket = dataMarketFromMsg
			}

			// CRITICAL: Validate finalDataMarket is not empty before processing
			if finalDataMarket == "" {
				log.WithFields(logrus.Fields{
					"epoch":                 epochIDStr,
					"queue_data_market":     dataMarketFromMsg,
					"goroutine_data_market": dataMarket,
					"queue":                 queueKey,
				}).Error("❌ CRITICAL: Cannot determine data market for aggregation - both queue message and goroutine data market are empty")
				continue // Skip this message and continue processing
			}

			log.WithFields(logrus.Fields{
				"epoch":       epochIDStr,
				"parts":       partsCompleted,
				"data_market": finalDataMarket,
			}).Info("📦 LEVEL 1: Aggregating finalizer worker parts into local batch")

			// Aggregate worker parts into complete local batch
			// This will write to stream, which triggers Level 2 aggregation via stream consumer
			// Run in goroutine to prevent blocking the BRPop loop
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.WithFields(logrus.Fields{
							"epoch": epochIDStr,
							"panic": r,
						}).Error("Panic in aggregateWorkerParts - recovered")
					}
				}()

				// Log before calling to track if function returns early
				log.WithFields(logrus.Fields{
					"epoch":       epochIDStr,
					"parts":       partsCompleted,
					"data_market": finalDataMarket,
				}).Debug("Calling aggregateWorkerParts")

				a.aggregateWorkerParts(epochIDStr, partsCompleted, finalDataMarket)

				// Log after completion to track successful processing
				log.WithFields(logrus.Fields{
					"epoch": epochIDStr,
				}).Debug("aggregateWorkerParts completed")
			}()

			// Log after queuing message processing to confirm loop continues immediately
			log.WithFields(logrus.Fields{
				"epoch":      epochIDStr,
				"loop_count": loopCount,
			}).Debug("Queued message for processing, continuing BRPop loop")
		}
	}
}

// parseEpochID parses epoch ID from various formats (string, scientific notation)
func parseEpochID(epochIDStr string) (uint64, error) {
	// Try standard integer parsing first
	epochID, err := strconv.ParseUint(epochIDStr, 10, 64)
	if err == nil {
		return epochID, nil
	}

	// If that fails, try parsing as float64 (for scientific notation)
	floatVal, err := strconv.ParseFloat(epochIDStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse epoch ID '%s' as integer or float: %w", epochIDStr, err)
	}

	// Convert float to uint64, checking for overflow
	if floatVal < 0 || floatVal > float64(^uint64(0)) {
		return 0, fmt.Errorf("epoch ID '%s' is out of valid uint64 range", epochIDStr)
	}

	return uint64(floatVal), nil
}

// getKeyBuilder returns a KeyBuilder for the given data market, creating one if needed.
// The dataMarket address is normalized to checksummed format before use.
func (a *Aggregator) getKeyBuilder(dataMarket string) *rediskeys.KeyBuilder {
	// Normalize to checksummed format
	checksummedMarket := common.HexToAddress(dataMarket).Hex()

	a.mu.RLock()
	kb, exists := a.keyBuilders[checksummedMarket]
	a.mu.RUnlock()

	if exists {
		return kb
	}

	// Create new KeyBuilder for this data market
	kb = rediskeys.NewKeyBuilder(a.config.ProtocolStateContract, checksummedMarket)

	a.mu.Lock()
	a.keyBuilders[checksummedMarket] = kb
	a.mu.Unlock()

	return kb
}

func (a *Aggregator) startAggregationWindow(epochIDStr string, dataMarket string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Convert to uint64 for validation
	epochID, err := parseEpochID(epochIDStr)
	if err != nil {
		log.WithError(err).Error("Failed to parse epoch ID for aggregation window")
		return
	}

	// Use composite key: dataMarket:epochID
	checksummedMarket := common.HexToAddress(dataMarket).Hex()
	compositeKey := fmt.Sprintf("%s:%d", checksummedMarket, epochID)

	// Check if timer already exists
	if _, exists := a.epochTimers[compositeKey]; exists {
		// Window already started - just log that we received another batch
		log.WithFields(logrus.Fields{
			"epoch":       epochID,
			"data_market": checksummedMarket,
		}).Info("⏱️  Additional validator batch received during aggregation window")
		return
	}

	// Get KeyBuilder for this data market
	kb := a.getKeyBuilder(dataMarket)

	// Start new aggregation window timer
	timer := time.AfterFunc(a.config.AggregationWindowDuration, func() {
		log.WithFields(logrus.Fields{
			"epoch":  epochID,
			"window": a.config.AggregationWindowDuration,
		}).Info("⏰ Aggregation window expired - finalizing Level 2 aggregation")

		// Update epoch state - transitioning to aggregating phase
		epochStateKey := kb.EpochState(epochIDStr)
		a.redisClient.HSet(a.ctx, epochStateKey, map[string]interface{}{
			"level2_status": "aggregating",
			"last_updated":  time.Now().Unix(),
		})
		a.redisClient.Expire(a.ctx, epochStateKey, 7*24*time.Hour)

		// Perform aggregation after window expires
		a.aggregateEpoch(epochIDStr, dataMarket)

		// Clean up timer
		a.mu.Lock()
		delete(a.epochTimers, compositeKey)
		a.mu.Unlock()
	})

	a.epochTimers[compositeKey] = timer

	// Update epoch state hash - Level 2 aggregation started (collecting phase)
	// kb is already set above (line 633)
	timestamp := time.Now().Unix()
	epochStateKey := kb.EpochState(epochIDStr)
	a.redisClient.HSet(a.ctx, epochStateKey, map[string]interface{}{
		"level2_status":     "collecting",
		"level2_started_at": timestamp,
		"phase":             "level2_aggregation",
		"last_updated":      timestamp,
	})
	a.redisClient.Expire(a.ctx, epochStateKey, 7*24*time.Hour)

	log.WithFields(logrus.Fields{
		"epoch":  epochID,
		"window": a.config.AggregationWindowDuration,
	}).Info("⏱️  Started Level 2 aggregation window - collecting validator batches")
}

func (a *Aggregator) aggregateWorkerParts(epochIDStr string, totalParts int, dataMarketFromQueue string) {
	// Log function entry to track all calls
	log.WithFields(logrus.Fields{
		"epoch":                  epochIDStr,
		"total_parts":            totalParts,
		"data_market_from_queue": dataMarketFromQueue,
	}).Info("🔍 aggregateWorkerParts called")

	// epochIDStr is already a parameter, no need to redeclare
	// Convert string to uint64
	epochID, err := parseEpochID(epochIDStr)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"epoch": epochIDStr,
		}).Error("❌ Failed to parse epoch ID - aggregateWorkerParts returning early")
		return
	}

	// Collect all batch parts from finalizer workers
	aggregatedResults := make(map[string]interface{})
	var dataMarket string
	var kb *rediskeys.KeyBuilder

	// CRITICAL: dataMarketFromQueue MUST be provided - it comes from the queue message
	// If it's missing, we cannot determine which KeyBuilder to use
	if dataMarketFromQueue == "" {
		log.WithFields(logrus.Fields{
			"epoch": epochIDStr,
		}).Error("❌ CRITICAL: Missing data_market in queue message - cannot aggregate batch parts without knowing which data market - aggregateWorkerParts returning early")
		return
	}

	// Normalize to checksummed format and get KeyBuilder
	dataMarket = common.HexToAddress(dataMarketFromQueue).Hex()
	kb = a.getKeyBuilder(dataMarket)

	log.WithFields(logrus.Fields{
		"epoch":       epochIDStr,
		"data_market": dataMarket,
		"total_parts": totalParts,
	}).Debug("Starting batch parts aggregation")

	for i := 0; i < totalParts; i++ {
		// Use the KeyBuilder we already have (from queue message)
		partKey := kb.BatchPart(strconv.FormatUint(epochID, 10), i)

		// Use timeout context to prevent hanging on expired/stale batch parts
		getCtx, getCancel := context.WithTimeout(a.ctx, 5*time.Second)
		partData, err := a.redisClient.Get(getCtx, partKey).Result()
		getCancel()

		if err != nil {
			if err == redis.Nil {
				log.WithFields(logrus.Fields{
					"epoch":       epochID,
					"part":        i,
					"total_parts": totalParts,
					"data_market": dataMarket,
					"part_key":    partKey,
				}).Warn("⚠️  Batch part not found (likely expired or cleaned up) - skipping old epoch")
				// For old epochs, batch parts may have expired - skip gracefully
				return
			}
			log.WithError(err).WithFields(logrus.Fields{
				"epoch":       epochID,
				"part":        i,
				"total_parts": totalParts,
				"data_market": dataMarket,
				"part_key":    partKey,
			}).Error("❌ CRITICAL: Failed to get batch part - Redis error")
			// Don't continue - if we can't find parts, something is wrong
			return
		}

		var partResults map[string]interface{}
		if err := json.Unmarshal([]byte(partData), &partResults); err != nil {
			log.WithError(err).WithFields(logrus.Fields{
				"epoch": epochID,
				"part":  i,
			}).Error("Failed to parse batch part")
			return
		}

		// Verify data_market matches (should always match since we got it from queue message)
		if partDataMarketRaw, ok := partResults["data_market"]; ok {
			if partDataMarketStr, ok := partDataMarketRaw.(string); ok {
				partDataMarket := common.HexToAddress(partDataMarketStr).Hex()
				if partDataMarket != dataMarket {
					log.WithFields(logrus.Fields{
						"epoch":             epochID,
						"part":              i,
						"queue_data_market": dataMarket,
						"part_data_market":  partDataMarket,
					}).Error("❌ CRITICAL: Data market mismatch between queue message and batch part")
					return
				}
			}
		}

		// Extract projects from partResults
		projects, ok := partResults["projects"].(map[string]interface{})
		if !ok {
			log.Errorf("Failed to extract projects from batch part %d", i)
			continue
		}

		// Merge results from this worker
		for projectID, data := range projects {
			aggregatedResults[projectID] = data
		}

		// Clean up part data
		a.redisClient.Del(a.ctx, partKey)
	}

	// Create finalized batch from aggregated worker results
	finalizedBatch := a.createFinalizedBatchFromParts(epochID, aggregatedResults, dataMarket)
	if finalizedBatch == nil {
		log.Errorf("Failed to create finalized batch for epoch %d", epochID)
		return
	}

	// kb is already set from extracting dataMarket above
	if kb == nil {
		log.Errorf("KeyBuilder not initialized for epoch %d", epochID)
		return
	}

	if a.ipfsClient != nil {
		if cid, err := a.ipfsClient.StoreFinalizedBatch(a.ctx, finalizedBatch); err == nil {
			finalizedBatch.BatchIPFSCID = cid
			log.WithFields(logrus.Fields{
				"epoch": epochID,
				"level": 1,
				"cid":   cid,
			}).Info("✅ LEVEL 1: Stored finalized batch in IPFS")
		} else {
			log.WithFields(logrus.Fields{
				"epoch": epochID,
				"level": 1,
			}).WithError(err).Warn("❌ LEVEL 1: Failed to store finalized batch in IPFS, continuing without CID")
		}
	}

	// Store as our local finalized batch (now with BatchIPFSCID populated)
	finalizedKey := kb.FinalizedBatch(strconv.FormatUint(epochID, 10))
	finalizedData, _ := json.Marshal(finalizedBatch)
	if err := a.redisClient.Set(a.ctx, finalizedKey, finalizedData, 24*time.Hour).Err(); err != nil {
		log.WithError(err).Error("Failed to store finalized batch")
		return
	}

	log.WithFields(logrus.Fields{
		"epoch":    epochID,
		"parts":    totalParts,
		"projects": len(aggregatedResults),
	}).Info("✅ LEVEL 1 COMPLETE: Created local finalized batch from worker parts")

	// Update epoch state hash - Level 1 completed
	timestamp := time.Now().Unix()
	epochStateKey := kb.EpochState(strconv.FormatUint(epochID, 10))
	a.redisClient.HSet(a.ctx, epochStateKey, map[string]interface{}{
		"level1_status":       "completed",
		"level1_completed_at": timestamp,
		"phase":               "level2_aggregation",
		"last_updated":        timestamp,
	})
	a.redisClient.Expire(a.ctx, epochStateKey, 7*24*time.Hour)

	// Add monitoring metrics for Level 1 aggregation

	// Pipeline for monitoring metrics
	pipe := a.redisClient.Pipeline()

	// 1. Add to batches timeline
	pipe.ZAdd(a.ctx, kb.MetricsBatchesTimeline(), redis.Z{
		Score:  float64(timestamp),
		Member: fmt.Sprintf("local:%d", epochID),
	})

	// 2. Store local batch metrics with TTL
	batchMetricsKey := kb.MetricsBatchLocal(strconv.FormatUint(epochID, 10))
	batchMetricsData := map[string]interface{}{
		"epoch_id":      epochID,
		"type":          "local",
		"validator_id":  a.config.SequencerID,
		"ipfs_cid":      finalizedBatch.BatchIPFSCID,
		"merkle_root":   finalizedBatch.MerkleRoot,
		"project_count": len(finalizedBatch.ProjectIds),
		"parts_merged":  totalParts,
		"timestamp":     timestamp,
	}
	jsonData, _ := json.Marshal(batchMetricsData)
	pipe.SetEx(a.ctx, batchMetricsKey, string(jsonData), 24*time.Hour)

	// 3. Add to validator batches timeline
	validatorBatchesKey := kb.MetricsValidatorBatches(a.config.SequencerID)
	pipe.ZAdd(a.ctx, validatorBatchesKey, redis.Z{
		Score:  float64(timestamp),
		Member: epochID,
	})

	// 4. Publish state change
	pipe.Publish(a.ctx, "state:change", fmt.Sprintf("batch:local:%d", epochID))

	// Execute pipeline (ignore errors - monitoring is non-critical)
	if _, err := pipe.Exec(a.ctx); err != nil {
		log.Debugf("Failed to write monitoring metrics: %v", err)
	}

	// CRITICAL: Write to aggregation stream to trigger Level 2 aggregation (single unified path)
	// This ensures our own local batch triggers aggregation, not just batches from other validators
	streamKey := kb.AggregationStream()
	// finalizedKey already declared above (line 631), reuse it

	streamValues := map[string]interface{}{
		"epoch":       epochIDStr,
		"validator":   a.config.SequencerID,
		"batch_key":   finalizedKey,
		"timestamp":   time.Now().Unix(),
		"type":        "validator_batch",
		"data_market": finalizedBatch.DataMarket, // EIP-55 checksummed format
	}

	// Add to stream with retry logic (same as P2P gateway)
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		_, err := a.redisClient.XAdd(a.ctx, &redis.XAddArgs{
			Stream: streamKey,
			Values: streamValues,
		}).Result()
		if err == nil {
			log.WithFields(logrus.Fields{
				"epoch":  epochID,
				"stream": streamKey,
			}).Debug("✅ Wrote local batch to aggregation stream")
			break
		}
		if i == maxRetries-1 {
			log.WithError(err).WithFields(logrus.Fields{
				"epoch":  epochID,
				"stream": streamKey,
			}).Error("Failed to write local batch to aggregation stream after retries")
		} else {
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
	}

	// CRITICAL: Broadcast our local batch to validator network
	broadcastMsg := map[string]interface{}{
		"type":    "finalized_batch",
		"epochId": epochID,
		"data":    finalizedBatch,
	}

	if msgData, err := json.Marshal(broadcastMsg); err == nil {
		// Use namespaced broadcast queue
		broadcastQueue := kb.OutgoingBroadcastBatch()
		if err := a.redisClient.LPush(a.ctx, broadcastQueue, msgData).Err(); err != nil {
			log.WithError(err).Error("Failed to queue batch for validator network broadcast")
		} else {
			log.WithFields(logrus.Fields{
				"epoch":    epochID,
				"projects": len(finalizedBatch.ProjectVotes),
			}).Info("📡 Broadcasting LOCAL finalized batch to validator network")
		}
	}

	// Clean up tracking data (namespaced)
	a.redisClient.Del(a.ctx,
		kb.EpochPartsCompleted(epochIDStr),
		kb.EpochPartsTotal(epochIDStr),
		kb.EpochPartsReady(epochIDStr),
	)
}

func (a *Aggregator) createFinalizedBatchFromParts(epochID uint64, projectSubmissions map[string]interface{}, dataMarket string) *consensus.FinalizedBatch {
	// Validate that dataMarket is not empty
	if dataMarket == "" {
		log.Errorf("dataMarket is required but missing for epoch %d", epochID)
		return nil
	}
	// Ensure data market is checksummed (normalize if needed)
	dataMarket = common.HexToAddress(dataMarket).Hex()
	// Extract project data and create proper finalized batch
	projectIDs := make([]string, 0)
	snapshotCIDs := make([]string, 0)
	projectVotes := make(map[string]uint32)
	submissionDetails := make(map[string][]submissions.SubmissionMetadata)

	for projectID, submissionData := range projectSubmissions {
		if dataMap, ok := submissionData.(map[string]interface{}); ok {
			if cid, ok := dataMap["cid"].(string); ok {
				projectIDs = append(projectIDs, projectID)
				snapshotCIDs = append(snapshotCIDs, cid)

				// Extract vote count
				if votes, ok := dataMap["votes"].(float64); ok {
					projectVotes[projectID] = uint32(votes)
				} else {
					projectVotes[projectID] = 1
				}

				// Extract submission metadata (WHO submitted WHAT for rewards)
				if metadataRaw, exists := dataMap["submission_metadata"]; exists {
					if metadataArray, ok := metadataRaw.([]interface{}); ok {
						// Convert to proper SubmissionMetadata structs
						projectMetadata := make([]submissions.SubmissionMetadata, 0)
						for _, metaItem := range metadataArray {
							if metaMap, ok := metaItem.(map[string]interface{}); ok {
								metadata := submissions.SubmissionMetadata{}
								if submitterID, ok := metaMap["submitter_id"].(string); ok {
									metadata.SubmitterID = submitterID
								}
								if snapshotCID, ok := metaMap["snapshot_cid"].(string); ok {
									metadata.SnapshotCID = snapshotCID
								}
								if timestamp, ok := metaMap["timestamp"].(float64); ok {
									metadata.Timestamp = uint64(timestamp)
								}
								if slotID, ok := metaMap["slot_id"].(float64); ok {
									metadata.SlotID = uint64(slotID)
								}
								if signature, ok := metaMap["signature"].(string); ok {
									metadata.Signature = []byte(signature)
								}
								// Initialize validators_confirming with this validator's ID
								metadata.ValidatorsConfirming = []string{a.config.SequencerID}
								metadata.VoteCount = 1 // Each submission counts as 1 vote
								projectMetadata = append(projectMetadata, metadata)
							}
						}
						submissionDetails[projectID] = projectMetadata
					}
				}
			}
		}
	}

	// Create merkle root (simplified)
	combined := ""
	for i := range projectIDs {
		combined += projectIDs[i] + ":" + snapshotCIDs[i] + ","
	}
	hash := sha256.Sum256([]byte(combined))
	merkleRoot := hash[:]

	finalizedBatch := &consensus.FinalizedBatch{
		EpochId:           epochID,
		ProjectIds:        projectIDs,
		SnapshotCids:      snapshotCIDs,
		MerkleRoot:        merkleRoot,
		SequencerId:       a.config.SequencerID,
		Timestamp:         uint64(time.Now().Unix()),
		ProjectVotes:      projectVotes,
		SubmissionDetails: submissionDetails,
		DataMarket:        dataMarket, // EIP-55 checksummed format
	}

	// Note: IPFS storage is now handled in the calling function (aggregateWorkerParts)
	// to ensure BatchIPFSCID is set before Redis persistence

	return finalizedBatch
}

func (a *Aggregator) aggregateEpoch(epochIDStr string, dataMarket string) {
	// Get KeyBuilder for this data market
	kb := a.getKeyBuilder(dataMarket)

	// Check if we've already aggregated this epoch recently (deduplication) - namespaced
	aggregatedKey := kb.BatchAggregated(epochIDStr)
	exists, err := a.redisClient.Exists(a.ctx, aggregatedKey).Result()
	if err != nil {
		log.WithField("epoch", epochIDStr).WithError(err).Error("Failed to check aggregated status")
		return
	}
	if exists > 0 {
		log.WithField("epoch", epochIDStr).Info("✅ Epoch already aggregated, skipping re-aggregation")
		return
	}

	// Get our own finalized batch from the unified sequencer's finalizer
	// Use namespaced key
	ourBatchKey := kb.FinalizedBatch(epochIDStr)
	var ourBatchData string
	ourBatchData, _ = a.redisClient.Get(a.ctx, ourBatchKey).Result()

	if ourBatchData == "" {
		log.WithField("epoch", epochIDStr).Warn("No local finalized batch found for epoch")
		// Continue anyway - we might just aggregate other validators' batches
	}

	var ourBatch *consensus.FinalizedBatch
	if ourBatchData != "" {
		ourBatch = &consensus.FinalizedBatch{}
		if err := json.Unmarshal([]byte(ourBatchData), ourBatch); err != nil {
			log.WithError(err).Error("Failed to parse our batch")
			// Continue with other validators' batches
		}
	}

	// Get all validators for this epoch using deterministic approach
	epochValidatorsKey := kb.EpochValidators(epochIDStr)
	validatorIDs, err := a.redisClient.SMembers(a.ctx, epochValidatorsKey).Result()
	if err != nil {
		log.WithError(err).WithField("epoch", epochIDStr).Error("Failed to get epoch validators")
		// Continue with local batch only if validator set is not available
		validatorIDs = []string{}
	}

	// Construct incoming batch keys deterministically
	incomingKeys := make([]string, 0)
	for _, validatorID := range validatorIDs {
		// Skip our own validator ID - we already have our local batch
		if validatorID == a.config.SequencerID {
			continue
		}
		batchKey := kb.IncomingBatch(epochIDStr, validatorID)
		incomingKeys = append(incomingKeys, batchKey)
	}

	totalValidators := len(incomingKeys)
	if ourBatch != nil {
		totalValidators++ // Include ourselves
	}

	log.WithFields(logrus.Fields{
		"epoch":            epochIDStr,
		"local_batch":      ourBatch != nil,
		"incoming_batches": len(incomingKeys),
		"total_validators": totalValidators,
	}).Info("Starting epoch aggregation")

	// Aggregate all batches (with data market)
	aggregatedBatch := a.createAggregatedBatch(ourBatch, incomingKeys, dataMarket)

	// Store in IPFS before Redis to ensure BatchIPFSCID is populated
	if a.ipfsClient != nil {
		if cid, err := a.ipfsClient.StoreFinalizedBatch(a.ctx, &aggregatedBatch); err == nil {
			aggregatedBatch.BatchIPFSCID = cid
			log.WithFields(logrus.Fields{
				"epoch": epochIDStr,
				"level": 2,
				"cid":   cid,
			}).Info("✅ LEVEL 2: Stored aggregated batch in IPFS")
		} else {
			log.WithFields(logrus.Fields{
				"epoch": epochIDStr,
				"level": 2,
			}).WithError(err).Warn("❌ LEVEL 2: Failed to store aggregated batch in IPFS, continuing without CID")
		}
	}

	// Store aggregated batch (now with BatchIPFSCID populated if IPFS was available)
	aggregatedData, _ := json.Marshal(aggregatedBatch)
	if err := a.redisClient.Set(a.ctx, aggregatedKey, aggregatedData, 24*time.Hour).Err(); err != nil {
		log.WithError(err).Error("Failed to store aggregated batch")
		return
	}

	log.WithFields(logrus.Fields{
		"epoch":            epochIDStr,
		"total_validators": totalValidators,
		"projects":         len(aggregatedBatch.ProjectVotes),
	}).Info("Aggregator: Completed aggregation")

	// Attempt VPA-based contract submission
	epochIDUint, _ := strconv.ParseUint(epochIDStr, 10, 64)
	go a.handleNewContractSubmission(epochIDUint, &aggregatedBatch)

	// Add monitoring metrics for Level 2 aggregation
	timestamp := time.Now().Unix()
	epochID, _ := parseEpochID(epochIDStr)

	// Update epoch state hash - Level 2 completed, transition to onchain_submission phase
	epochStateKey := kb.EpochState(epochIDStr)
	a.redisClient.HSet(a.ctx, epochStateKey, map[string]interface{}{
		"level2_status":       "completed",
		"level2_completed_at": timestamp,
		"phase":               "onchain_submission",
		"last_updated":        timestamp,
	})
	a.redisClient.Expire(a.ctx, epochStateKey, 7*24*time.Hour)

	// Pipeline for monitoring metrics
	pipe := a.redisClient.Pipeline()

	// 1. Add to batches timeline
	pipe.ZAdd(a.ctx, kb.MetricsBatchesTimeline(), redis.Z{
		Score:  float64(timestamp),
		Member: fmt.Sprintf("aggregated:%s", utils.FormatEpochID(epochIDStr)),
	})

	// 2. Store aggregated batch metrics with TTL
	batchMetricsKey := kb.MetricsBatchAggregated(epochIDStr)
	batchMetricsData := map[string]interface{}{
		"epoch_id":         epochID,
		"type":             "aggregated",
		"validators_count": totalValidators,
		"project_count":    len(aggregatedBatch.ProjectVotes),
		"timestamp":        timestamp,
		"validator_ids":    extractValidatorIDs(incomingKeys),
		"ipfs_cid":         aggregatedBatch.BatchIPFSCID,
		"merkle_root":      aggregatedBatch.MerkleRoot,
	}
	jsonData, _ := json.Marshal(batchMetricsData)
	pipe.SetEx(a.ctx, batchMetricsKey, string(jsonData), 24*time.Hour)

	// 3. Store validator list with TTL (include local + remote validators)
	validatorsKey := kb.MetricsBatchValidators(epochIDStr)
	allValidators := extractValidatorIDs(incomingKeys)
	// Add local validator ID
	allValidators = append(allValidators, a.config.SequencerID)
	validatorList, _ := json.Marshal(allValidators)
	pipe.SetEx(a.ctx, validatorsKey, string(validatorList), 24*time.Hour)

	// 4. Publish state change
	pipe.Publish(a.ctx, "state:change", fmt.Sprintf("batch:aggregated:%s", epochIDStr))

	// Execute pipeline (ignore errors - monitoring is non-critical)
	if _, err := pipe.Exec(a.ctx); err != nil {
		log.Debugf("Failed to write monitoring metrics: %v", err)
	}

	// CRITICAL: Clean up source finalized batch to prevent re-aggregation loop
	keysToDelete := []string{ourBatchKey}
	keysToDelete = append(keysToDelete, incomingKeys...)

	if deleted, err := a.redisClient.Del(a.ctx, keysToDelete...).Result(); err != nil {
		log.WithError(err).Warn("Failed to clean up source batches after aggregation")
	} else {
		log.WithFields(logrus.Fields{
			"epoch":        epochIDStr,
			"keys_deleted": deleted,
		}).Info("🗑️  Cleaned up source batches after successful aggregation")
	}
}

func (a *Aggregator) createAggregatedBatch(ourBatch *consensus.FinalizedBatch, incomingKeys []string, dataMarket string) consensus.FinalizedBatch {
	// Extract data market from ourBatch if available, otherwise use passed parameter
	if ourBatch != nil && ourBatch.DataMarket != "" {
		// Normalize to checksummed format
		dataMarket = common.HexToAddress(ourBatch.DataMarket).Hex()
	} else if dataMarket == "" {
		// This shouldn't happen if called correctly, but log warning
		log.Warnf("createAggregatedBatch called without dataMarket and ourBatch.DataMarket is empty")
	}

	// Initialize aggregated batch
	aggregated := consensus.FinalizedBatch{
		SubmissionDetails: make(map[string][]submissions.SubmissionMetadata),
		ProjectVotes:      make(map[string]uint32),
		Timestamp:         uint64(time.Now().Unix()),
		SequencerId:       a.config.SequencerID, // Set our node's ID
		DataMarket:        dataMarket,           // EIP-55 checksummed format
	}

	// Track all validators' views
	validatorViews := make(map[string]*consensus.FinalizedBatch)

	// Add our batch if we have one
	if ourBatch != nil {
		aggregated.EpochId = ourBatch.EpochId
		validatorViews[ourBatch.SequencerId] = ourBatch

		// Add our submissions
		for projectID, submissions := range ourBatch.SubmissionDetails {
			aggregated.SubmissionDetails[projectID] = append(
				aggregated.SubmissionDetails[projectID],
				submissions...,
			)

			// Add our votes
			aggregated.ProjectVotes[projectID] = ourBatch.ProjectVotes[projectID]
		}
	}

	// Store validator IPFS CIDs
	validatorBatchCIDs := make(map[string]string)

	// Add incoming batches from other validators
	for _, key := range incomingKeys {
		batchData, err := a.redisClient.Get(a.ctx, key).Result()
		if err != nil {
			log.WithError(err).WithField("key", key).Error("Failed to get incoming batch")
			continue
		}

		var batch consensus.FinalizedBatch
		var validatorID string

		// First try to unmarshal as ValidatorBatch (P2P message format)
		var vBatch consensus.ValidatorBatch
		if err := json.Unmarshal([]byte(batchData), &vBatch); err == nil && vBatch.BatchIPFSCID != "" {
			// Store the CID mapping
			validatorBatchCIDs[vBatch.ValidatorID] = vBatch.BatchIPFSCID
			validatorID = vBatch.ValidatorID

			// Try to extract FinalizedBatch data (might be embedded or need IPFS fetch)
			if err := json.Unmarshal([]byte(batchData), &batch); err != nil {
				log.WithField("validator", validatorID).Debug("ValidatorBatch format detected but missing FinalizedBatch data")
				continue
			}
		} else {
			// Fallback: try as FinalizedBatch directly
			if err := json.Unmarshal([]byte(batchData), &batch); err != nil {
				log.WithError(err).Error("Failed to parse incoming batch")
				continue
			}

			// Track this validator's view
			validatorID = batch.SequencerId
			if validatorID == "" {
				// Extract from key as fallback
				parts := strings.Split(key, ":")
				if len(parts) >= 4 {
					validatorID = parts[3]
				}
			}
		}

		if aggregated.EpochId == 0 {
			aggregated.EpochId = batch.EpochId
		}

		// Validate incoming batch has DataMarket field
		if batch.DataMarket == "" {
			log.Warnf("Skipping incoming batch without DataMarket field - old format not supported: epoch=%d, validator=%s", batch.EpochId, validatorID)
			continue
		}
		// Normalize to checksummed format
		batch.DataMarket = common.HexToAddress(batch.DataMarket).Hex()
		// Ensure it matches the expected data market
		if batch.DataMarket != dataMarket {
			log.Warnf("Incoming batch DataMarket mismatch: expected=%s, got=%s, epoch=%d, validator=%s", dataMarket, batch.DataMarket, batch.EpochId, validatorID)
			continue
		}

		validatorViews[validatorID] = &batch

		// Merge submissions
		for projectID, submissions := range batch.SubmissionDetails {
			aggregated.SubmissionDetails[projectID] = append(
				aggregated.SubmissionDetails[projectID],
				submissions...,
			)

			// Merge votes (take max count)
			if currentCount, exists := aggregated.ProjectVotes[projectID]; !exists || batch.ProjectVotes[projectID] > currentCount {
				aggregated.ProjectVotes[projectID] = batch.ProjectVotes[projectID]
			}
		}
	}

	// Merge duplicate submissions: combine validators_confirming for same submitter+CID
	for projectID, subs := range aggregated.SubmissionDetails {
		merged := make(map[string]*submissions.SubmissionMetadata) // key: submitter_id:snapshot_cid

		for i := range subs {
			sub := &subs[i]
			key := sub.SubmitterID + ":" + sub.SnapshotCID

			if existing, found := merged[key]; found {
				// Same submission seen by multiple validators - merge validator lists
				existing.ValidatorsConfirming = append(existing.ValidatorsConfirming, sub.ValidatorsConfirming...)
				existing.VoteCount++
			} else {
				merged[key] = sub
			}
		}

		// Replace with merged submissions
		mergedList := make([]submissions.SubmissionMetadata, 0, len(merged))
		for _, sub := range merged {
			mergedList = append(mergedList, *sub)
		}
		aggregated.SubmissionDetails[projectID] = mergedList
	}

	// Build ProjectIds and SnapshotCids arrays from aggregated data
	// Determine consensus CID for each project (most votes)
	projectIDs := make([]string, 0, len(aggregated.ProjectVotes))
	snapshotCIDs := make([]string, 0, len(aggregated.ProjectVotes))

	for projectID := range aggregated.ProjectVotes {
		projectIDs = append(projectIDs, projectID)

		// Find consensus CID (most submitted)
		cidCounts := make(map[string]int)
		if submissions, exists := aggregated.SubmissionDetails[projectID]; exists {
			for _, sub := range submissions {
				cidCounts[sub.SnapshotCID]++
			}
		}

		// Get CID with highest count
		var consensusCID string
		maxCount := 0
		for cid, count := range cidCounts {
			if count > maxCount {
				maxCount = count
				consensusCID = cid
			}
		}
		snapshotCIDs = append(snapshotCIDs, consensusCID)
	}

	aggregated.ProjectIds = projectIDs
	aggregated.SnapshotCids = snapshotCIDs

	// Calculate merkle root from consensus data
	combined := ""
	for i := range projectIDs {
		combined += projectIDs[i] + ":" + snapshotCIDs[i] + ","
	}
	hash := sha256.Sum256([]byte(combined))
	aggregated.MerkleRoot = hash[:]

	// Store validator batch IPFS CIDs for attribution tracking
	aggregated.ValidatorBatches = validatorBatchCIDs

	// Log aggregation summary
	log.WithFields(logrus.Fields{
		"epoch":          aggregated.EpochId,
		"validators":     len(validatorViews),
		"total_projects": len(aggregated.ProjectVotes),
		"consensus_cids": len(snapshotCIDs),
	}).Info("📊 AGGREGATED FINALIZATION: Combined views from all validators")

	// Log which validators contributed
	for validatorID := range validatorViews {
		log.Debugf("  Validator %s contributed to aggregation", validatorID)
	}

	// Note: IPFS storage is now handled in the calling function (aggregateEpoch)
	// to ensure BatchIPFSCID is set before Redis persistence

	// Set validator count
	aggregated.ValidatorCount = len(validatorViews)

	// NOTE: Broadcasting is now handled by the unified sequencer after Level 1 aggregation
	// The aggregator component only performs Level 2 network-wide aggregation
	// No broadcasting needed here since the unified sequencer already queued it

	return aggregated
}

func (a *Aggregator) reportMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			// Aggregate metrics across all data markets
			totalAggregatedCount := 0
			validatorSet := make(map[string]bool) // Use map to avoid duplicates across all markets

			// Process metrics for each data market
			for _, dataMarket := range a.config.DataMarketAddresses {
				checksummedMarket := common.HexToAddress(dataMarket).Hex()
				kb := a.getKeyBuilder(checksummedMarket)

				// Count aggregated batches using timeline entries (matches monitoring API)
				timelineKey := kb.MetricsBatchesTimeline()
				timelineEntries, err := a.redisClient.ZRange(a.ctx, timelineKey, 0, -1).Result()
				if err == nil {
					for _, entry := range timelineEntries {
						if strings.HasPrefix(entry, "aggregated:") {
							totalAggregatedCount++
						}
					}
				}

				// Get all active epochs for this data market
				activeEpochs, err := a.redisClient.SMembers(a.ctx, kb.ActiveEpochs()).Result()
				if err != nil {
					log.WithError(err).WithField("data_market", checksummedMarket).Debug("Failed to get active epochs for validator counting")
					continue
				}

				// Get validators from each active epoch
				for _, epochID := range activeEpochs {
					epochValidators, err := a.redisClient.SMembers(a.ctx, kb.EpochValidators(epochID)).Result()
					if err != nil {
						log.WithError(err).WithFields(logrus.Fields{
							"epoch":       epochID,
							"data_market": checksummedMarket,
						}).Debug("Failed to get epoch validators")
						continue
					}
					// Add validators to set to avoid duplicates
					for _, validatorID := range epochValidators {
						validatorSet[validatorID] = true
					}
				}
			}

			// Convert map to slice
			validators := make([]string, 0, len(validatorSet))
			for validatorID := range validatorSet {
				validators = append(validators, validatorID)
			}

			log.WithFields(logrus.Fields{
				"aggregated_batches": totalAggregatedCount,
				"active_validators":  len(validators),
				"data_markets":       len(a.config.DataMarketAddresses),
			}).Info("Aggregator metrics")
		}
	}
}

func (a *Aggregator) Start() error {
	log.Info("Starting Aggregator")

	// Start queue-based processing (Level 1 only - Level 2 handled by stream consumer)
	go a.processAggregationQueue()
	go a.reportMetrics()

	return nil
}

func (a *Aggregator) Stop() {
	log.Info("Stopping Aggregator")
	a.cancel()
	if a.redisClient != nil {
		a.redisClient.Close()
	}
}

// extractValidatorIDs extracts validator IDs from incoming batch keys
func extractValidatorIDs(keys []string) []string {
	validators := make([]string, 0)
	for _, key := range keys {
		// Keys are in format: {protocol}:{market}:incoming:batch:{epochId}:{validatorId}
		parts := strings.Split(key, ":")
		if len(parts) >= 6 {
			validators = append(validators, parts[5])
		}
	}
	return validators
}

// handleNewContractSubmission implements new contract submission logic
func (a *Aggregator) handleNewContractSubmission(epochID uint64, aggregatedBatch *consensus.FinalizedBatch) {
	// Convert epochID to string for consistency
	epochIDStr := strconv.FormatUint(epochID, 10)

	log.WithFields(logrus.Fields{
		"epoch":    epochIDStr,
		"projects": len(aggregatedBatch.ProjectIds),
	}).Info("🚀 Starting new contract submission")

	// Get data market address from aggregated batch (mandatory field)
	if aggregatedBatch.DataMarket == "" {
		log.Warnf("Cannot submit batch without DataMarket field - skipping: epoch=%s", epochIDStr)
		return
	}
	newDataMarket := aggregatedBatch.DataMarket // Already checksummed

	// Check if already submitted (using composite key: dataMarket:epochID)
	checksummedMarket := common.HexToAddress(newDataMarket).Hex()
	compositeKey := fmt.Sprintf("%s:%d", checksummedMarket, epochID)
	a.mu.Lock()
	if a.submissionState[compositeKey] {
		a.mu.Unlock()
		log.WithFields(logrus.Fields{
			"epoch":       epochIDStr,
			"data_market": checksummedMarket,
		}).Info("Already submitted for this epoch")
		return
	}
	a.mu.Unlock()

	// Submit to new contracts via relayer-py
	// submitBatchViaRelayer handles all VPA priority and timing checks internally
	if err := a.submitBatchViaRelayer(epochID, aggregatedBatch, newDataMarket); err != nil {
		log.WithError(err).Error("New contract submission failed")
		// Don't return error - new contract submission failure shouldn't affect other processing
	}
}

// Legacy contract submission removed - DSV nodes only submit via relayer-py to new contracts

// submitBatchViaRelayer submits batch to new contracts via relayer-py service
// For DSV nodes, we typically send 1 aggregated batch per epoch
func (a *Aggregator) submitBatchViaRelayer(epochID uint64, aggregatedBatch *consensus.FinalizedBatch, dataMarketAddr string) error {
	epochIDStr := strconv.FormatUint(epochID, 10)

	// Get VPA client for this data market
	checksummedMarket := common.HexToAddress(dataMarketAddr).Hex()
	vpaClient, exists := a.vpaClients[checksummedMarket]
	if !exists {
		log.WithFields(logrus.Fields{
			"epoch":       epochIDStr,
			"data_market": checksummedMarket,
		}).Debug("VPA client not initialized for this data market, skipping new contract submission")
		return nil
	}

	// Check if validator has priority for this epoch
	priority, err := vpaClient.GetMyPriority(a.ctx, dataMarketAddr, epochID)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"epoch":       epochIDStr,
			"data_market": dataMarketAddr,
		}).Warn("⚠️ Failed to get VPA priority, skipping submission")
		// Store failed priority check for monitoring
		a.storePriorityCheck(epochID, dataMarketAddr, 0, "priority_check_failed")
		return nil
	}

	log.WithFields(logrus.Fields{
		"epoch":       epochIDStr,
		"data_market": dataMarketAddr,
		"priority":    priority,
	}).Info("🔍 GetMyPriority returned priority")

	if priority == 0 {
		log.WithFields(logrus.Fields{
			"epoch":       epochIDStr,
			"data_market": dataMarketAddr,
		}).Info("ℹ️  No VPA priority assigned (Priority 0), skipping new contract submission")
		// Store priority check result for monitoring (priority 0 = no priority)
		a.storePriorityCheck(epochID, dataMarketAddr, 0, "no_priority")
		return nil
	}

	// Store priority assignment for monitoring
	log.WithFields(logrus.Fields{
		"epoch":       epochIDStr,
		"priority":    priority,
		"data_market": dataMarketAddr,
	}).Info("🎯 VPA Priority assigned for epoch")
	a.storePriorityCheck(epochID, dataMarketAddr, priority, "assigned")

	// Wait for submission window to open (this checks timing and waits if needed)
	log.WithFields(logrus.Fields{
		"epoch":    epochIDStr,
		"priority": priority,
	}).Info("⏳ Waiting for submission window to open...")

	// Create a context with timeout for waiting (max 10 minutes to prevent indefinite blocking)
	waitCtx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()

	if err := vpaClient.WaitForSubmissionWindow(waitCtx, dataMarketAddr, epochID, priority); err != nil {
		if err == context.DeadlineExceeded {
			log.WithFields(logrus.Fields{
				"epoch":    epochIDStr,
				"priority": priority,
			}).Warn("⏰ Timeout waiting for submission window (10min), skipping submission")
			// Store timeout for monitoring
			a.storeSubmissionMetrics(epochID, dataMarketAddr, priority, false, "", 0)
			a.storePriorityCheck(epochID, dataMarketAddr, priority, "window_timeout")
			return nil
		}
		if err == context.Canceled {
			log.WithFields(logrus.Fields{
				"epoch":    epochIDStr,
				"priority": priority,
			}).Debug("Context canceled while waiting for submission window")
			return nil
		}
		log.WithError(err).WithFields(logrus.Fields{
			"epoch":    epochIDStr,
			"priority": priority,
		}).Warn("⚠️ Error waiting for submission window, skipping submission")
		// Store error for monitoring
		a.storeSubmissionMetrics(epochID, dataMarketAddr, priority, false, "", 0)
		a.storePriorityCheck(epochID, dataMarketAddr, priority, "window_error")
		return nil
	}

	log.WithFields(logrus.Fields{
		"epoch":    epochIDStr,
		"priority": priority,
	}).Info("✅ Submission window is open, checking if submission already exists...")

	// CRITICAL: If priority > 1, check if any lower priority validator already submitted
	// Priority 2+ should only submit if Priority 1 (and all lower priorities) failed to submit
	if priority > 1 {
		hasSubmission, err := a.checkEpochHasSubmission(dataMarketAddr, epochID)
		if err != nil {
			log.WithError(err).WithField("epoch", epochIDStr).Warn("Failed to check if epoch has submission, proceeding anyway")
		} else if hasSubmission {
			log.WithFields(logrus.Fields{
				"epoch":    epochIDStr,
				"priority": priority,
			}).Info("⏭️  Epoch already has a submission from a higher priority validator, skipping submission")
			// Store skipped submission reason
			a.storeSubmissionMetrics(epochID, dataMarketAddr, priority, false, "", 0)
			a.storePriorityCheck(epochID, dataMarketAddr, priority, "skipped_higher_priority_submitted")
			return nil
		}
	}

	log.WithFields(logrus.Fields{
		"epoch":    epochIDStr,
		"priority": priority,
	}).Info("✅ No existing submission found, proceeding with submission")

	if a.relayerPyEndpoint == "" {
		log.WithField("epoch", epochIDStr).Debug("Relayer endpoint not configured, skipping new contract submission")
		return nil
	}

	log.WithFields(logrus.Fields{
		"epoch":       epochIDStr,
		"projects":    len(aggregatedBatch.ProjectIds),
		"data_market": dataMarketAddr,
		"endpoint":    a.relayerPyEndpoint,
	}).Info("🚀 Submitting batch via relayer-py")

	// Step 1: Send batch size first (required by relayer to track when to call endBatchSubmissions)
	// For DSV nodes, we typically send 1 aggregated batch per epoch
	batchSize := 1
	if err := a.sendBatchSizeToRelayer(epochID, dataMarketAddr, batchSize); err != nil {
		log.WithError(err).WithField("epoch", epochIDStr).Warn("Failed to send batch size to relayer, continuing with batch submission")
		// Don't fail completely - relayer might still process batches without size info
	} else {
		log.WithFields(logrus.Fields{
			"epoch":      epochIDStr,
			"batch_size": batchSize,
		}).Debug("✅ Sent batch size to relayer")
	}

	// Prepare payload for relayer-py service (must match BatchSubmissionRequest schema)
	// Expected fields: dataMarketAddress, batchCID, epochID, projectIDs, snapshotCIDs, finalizedCIDsRootHash, authToken
	// Note: For internal setups, relayer-py defaults to empty auth_token, so empty string is acceptable
	// If APIAuthToken is not set, use empty string to match relayer's default
	authToken := a.config.APIAuthToken
	if authToken == "" {
		authToken = "" // Explicitly empty for internal setups
	}

	// Convert MerkleRoot ([]byte) to hex string with 0x prefix for bytes32 compatibility
	// The relayer expects this as a hex string, and web3.py will convert it to bytes32
	finalizedCIDsRootHash := fmt.Sprintf("0x%x", aggregatedBatch.MerkleRoot)

	payload := map[string]interface{}{
		"dataMarketAddress":     dataMarketAddr,
		"batchCID":              aggregatedBatch.BatchIPFSCID,
		"epochID":               int(epochID),
		"projectIDs":            aggregatedBatch.ProjectIds,
		"snapshotCIDs":          aggregatedBatch.SnapshotCids,
		"finalizedCIDsRootHash": finalizedCIDsRootHash,
		"authToken":             authToken,
	}

	log.WithFields(logrus.Fields{
		"epoch":        epochIDStr,
		"payload_keys": len(payload),
	}).Debug("Prepared relayer-py payload")

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal relayer-py submission payload: %w", err)
	}

	// Submit to relayer-py service
	endpoint := a.relayerPyEndpoint + "/submitSubmissionBatch"
	resp, err := a.httpClient.Post(endpoint, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"epoch":       epochIDStr,
			"priority":    priority,
			"endpoint":    endpoint,
			"data_market": dataMarketAddr,
		}).Error("❌ Failed to submit batch to relayer-py")
		// Store failed submission attempt
		a.storeSubmissionMetrics(epochID, dataMarketAddr, priority, false, "", 0)
		return fmt.Errorf("failed to submit batch to relayer-py: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		log.WithFields(logrus.Fields{
			"epoch":       epochIDStr,
			"priority":    priority,
			"status_code": resp.StatusCode,
			"response":    bodyStr,
			"data_market": dataMarketAddr,
		}).Error("❌ VPA relayer returned non-200 status")
		a.storeSubmissionMetrics(epochID, dataMarketAddr, priority, false, "", 0)
		return fmt.Errorf("VPA relayer returned non-200 status: %d, response: %s", resp.StatusCode, bodyStr)
	}

	// Relayer-py returns exactly: {'message': 'Submitted Snapshot to relayer!'}
	var response struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.WithError(err).WithFields(logrus.Fields{
			"epoch":       epochIDStr,
			"priority":    priority,
			"response":    string(bodyBytes),
			"data_market": dataMarketAddr,
		}).Error("❌ Failed to decode VPA relayer response")
		a.storeSubmissionMetrics(epochID, dataMarketAddr, priority, false, "", 0)
		return fmt.Errorf("failed to decode VPA relayer response: %w", err)
	}

	// Success: relayer accepted the request (200 OK + message)
	// Transaction details (tx_hash, block_number) come from relayer logs, not this response
	log.WithFields(logrus.Fields{
		"epoch":       epochIDStr,
		"priority":    priority,
		"message":     response.Message,
		"data_market": dataMarketAddr,
	}).Info("✅ VPA batch submission queued successfully - relayer processing asynchronously")

	// Store submission metrics (queued successfully, tx_hash will be empty since relayer processes async)
	a.storeSubmissionMetrics(epochID, dataMarketAddr, priority, true, "", 0)

	// Mark as submitted for this epoch (using composite key: dataMarket:epochID)
	// checksummedMarket already declared above
	compositeKey := fmt.Sprintf("%s:%d", checksummedMarket, epochID)
	a.mu.Lock()
	a.submissionState[compositeKey] = true
	a.mu.Unlock()

	return nil
}

// sendBatchSizeToRelayer sends the batch size to relayer-py before submitting batches
// This tells the relayer how many batches to expect, so it knows when to call endBatchSubmissions
func (a *Aggregator) sendBatchSizeToRelayer(epochID uint64, dataMarketAddr string, batchSize int) error {
	authToken := a.config.APIAuthToken // Defaults to "" if not set

	payload := map[string]interface{}{
		"dataMarketAddress": dataMarketAddr,
		"batchSize":         batchSize,
		"epochID":           int(epochID),
		"authToken":         authToken,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal batch size payload: %w", err)
	}

	endpoint := a.relayerPyEndpoint + "/submitBatchSize"
	resp, err := a.httpClient.Post(endpoint, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to submit batch size to relayer-py: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		return fmt.Errorf("relayer returned non-200 status for batch size: %d, response: %s", resp.StatusCode, bodyStr)
	}

	return nil
}

// checkEpochHasSubmission checks if batch submissions have been completed for this epoch on-chain
// by querying for BatchSubmissionsCompleted event logs from ProtocolState contract
// Returns true if endBatchSubmissions was called (submissions completed), false otherwise
// This is used to prevent Priority 2+ validators from submitting if Priority 1 already completed submissions
func (a *Aggregator) checkEpochHasSubmission(dataMarketAddr string, epochID uint64) (bool, error) {
	if a.rpcClient == nil || a.config.ProtocolStateContract == "" {
		// Can't check on-chain, rely on contract's duplicate prevention
		log.WithFields(logrus.Fields{
			"epoch":       epochID,
			"data_market": dataMarketAddr,
		}).Debug("Cannot check on-chain submission status (RPC client or ProtocolState not configured)")
		return false, nil
	}

	// Load ProtocolState ABI to get event signature
	protocolStateABI, err := abiloader.LoadABI("PowerloomProtocolState.abi.json")
	if err != nil {
		log.WithError(err).Debug("Failed to load ProtocolState ABI for submission check")
		return false, nil // Allow submission attempt, contract will reject if duplicate
	}

	// Get the BatchSubmissionsCompleted event signature
	// Event: BatchSubmissionsCompleted(address indexed dataMarketAddress, uint256 indexed epochId, uint256 timestamp)
	event, found := protocolStateABI.Events["BatchSubmissionsCompleted"]
	if !found {
		log.Debug("BatchSubmissionsCompleted event not found in ABI")
		return false, nil
	}

	// Calculate event signature hash (first topic)
	eventSig := event.ID

	// Prepare filter query
	protocolStateAddr := common.HexToAddress(a.config.ProtocolStateContract)
	dataMarket := common.HexToAddress(dataMarketAddr)
	epochIDBig := big.NewInt(int64(epochID))

	// Topics:
	// [0] = event signature (BatchSubmissionsCompleted)
	// [1] = dataMarketAddress (indexed, left-padded to 32 bytes)
	// [2] = epochId (indexed, as uint256)
	// Addresses in topics are left-padded with zeros to 32 bytes
	dataMarketHash := common.BytesToHash(dataMarket.Bytes())
	epochIDHash := common.BigToHash(epochIDBig)

	query := ethereum.FilterQuery{
		Addresses: []common.Address{protocolStateAddr},
		Topics: [][]common.Hash{
			{eventSig},       // Event signature
			{dataMarketHash}, // dataMarketAddress (left-padded to 32 bytes)
			{epochIDHash},    // epochId (uint256)
		},
	}

	// Query event logs
	logs, err := a.rpcClient.FilterLogs(a.ctx, query)
	if err != nil {
		log.WithError(err).Debug("Failed to filter BatchSubmissionsCompleted logs")
		return false, nil
	}

	hasSubmission := len(logs) > 0

	log.WithFields(logrus.Fields{
		"epoch":          epochID,
		"data_market":    dataMarketAddr,
		"event_logs":     len(logs),
		"has_submission": hasSubmission,
	}).Debug("Checked BatchSubmissionsCompleted event logs")

	return hasSubmission, nil
}

// storePriorityCheck stores priority assignment information in Redis for monitoring
func (a *Aggregator) storePriorityCheck(epochID uint64, dataMarketAddr string, priority int, status string) {
	if a.redisClient == nil {
		return
	}

	epochIDStr := strconv.FormatUint(epochID, 10)
	timestamp := time.Now().Unix()

	// Use protocol state contract for VPA data
	protocolState := a.config.ProtocolStateContract

	// Create KeyBuilder for this data market (namespaced)
	kb := rediskeys.NewKeyBuilder(protocolState, dataMarketAddr)

	// Store priority assignment per epoch
	priorityKey := kb.VPAPriorityAssignment(epochIDStr)
	priorityData := map[string]interface{}{
		"epoch_id":    epochIDStr,
		"priority":    priority,
		"status":      status,
		"timestamp":   timestamp,
		"data_market": dataMarketAddr,
		"validator":   a.config.VPAValidatorAddress,
	}
	jsonData, _ := json.Marshal(priorityData)
	a.redisClient.SetEx(a.ctx, priorityKey, string(jsonData), 7*24*time.Hour) // Keep for 7 days

	// Add to priority timeline for historical tracking (namespaced by protocol:market)
	timelineKey := kb.VPAPriorityTimeline()
	a.redisClient.ZAdd(a.ctx, timelineKey, redis.Z{
		Score:  float64(timestamp),
		Member: fmt.Sprintf("%s:%d:%s", epochIDStr, priority, status),
	})

	// Update priority statistics (namespaced by protocol:market)
	statsKey := kb.VPAStats()
	if priority > 0 {
		a.redisClient.HIncrBy(a.ctx, statsKey, "total_priority_assignments", 1)
		a.redisClient.HIncrBy(a.ctx, statsKey, fmt.Sprintf("priority_%d_count", priority), 1)
	} else {
		a.redisClient.HIncrBy(a.ctx, statsKey, "no_priority_count", 1)
	}
	a.redisClient.Expire(a.ctx, statsKey, 7*24*time.Hour)

	// Store combined epoch status (priority + submission status)
	epochStatusKey := kb.VPAEpochStatus(epochIDStr)
	epochStatusData := map[string]interface{}{
		"epoch_id":        epochIDStr,
		"priority":        priority,
		"priority_status": status,
		"timestamp":       timestamp,
		"data_market":     dataMarketAddr,
		"validator":       a.config.VPAValidatorAddress,
	}
	statusJsonData, _ := json.Marshal(epochStatusData)
	a.redisClient.SetEx(a.ctx, epochStatusKey, string(statusJsonData), 7*24*time.Hour)
}

// storeSubmissionMetrics stores submission attempt results in Redis for monitoring
func (a *Aggregator) storeSubmissionMetrics(epochID uint64, dataMarketAddr string, priority int, success bool, txHash string, blockNumber uint64) {
	if a.redisClient == nil {
		return
	}

	epochIDStr := strconv.FormatUint(epochID, 10)
	timestamp := time.Now().Unix()

	// Use protocol state contract for VPA data
	protocolState := a.config.ProtocolStateContract

	// Create KeyBuilder for this data market (namespaced)
	kb := rediskeys.NewKeyBuilder(protocolState, dataMarketAddr)

	// Store submission result per epoch
	submissionKey := kb.VPASubmissionResult(epochIDStr)
	submissionData := map[string]interface{}{
		"epoch_id":     epochIDStr,
		"priority":     priority,
		"success":      success,
		"tx_hash":      txHash,
		"block_number": blockNumber,
		"timestamp":    timestamp,
		"data_market":  dataMarketAddr,
		"validator":    a.config.VPAValidatorAddress,
	}
	jsonData, _ := json.Marshal(submissionData)
	a.redisClient.SetEx(a.ctx, submissionKey, string(jsonData), 7*24*time.Hour) // Keep for 7 days

	// Update epoch state hash with submission status
	epochStateKey := kb.EpochState(epochIDStr)
	onchainStatus := "pending"
	if success {
		if txHash != "" {
			onchainStatus = "submitted"
		} else {
			onchainStatus = "queued"
		}
	} else {
		onchainStatus = "failed"
	}

	stateUpdates := map[string]interface{}{
		"onchain_status":           onchainStatus,
		"vpa_submission_attempted": true,
		"priority":                 priority,
		"last_updated":             timestamp,
	}
	if txHash != "" {
		stateUpdates["onchain_tx_hash"] = txHash
		stateUpdates["onchain_submitted_at"] = timestamp
	}
	if blockNumber > 0 {
		stateUpdates["onchain_block_number"] = blockNumber
		if txHash != "" {
			stateUpdates["onchain_status"] = "confirmed"
		}
	}
	a.redisClient.HSet(a.ctx, epochStateKey, stateUpdates)
	// Refresh TTL on epoch state (7 days - same as initial creation)
	a.redisClient.Expire(a.ctx, epochStateKey, 7*24*time.Hour)

	// Add to submission timeline (namespaced by protocol:market)
	timelineKey := kb.VPASubmissionTimeline()
	status := "success"
	if !success {
		status = "failed"
	}
	a.redisClient.ZAdd(a.ctx, timelineKey, redis.Z{
		Score:  float64(timestamp),
		Member: fmt.Sprintf("%s:%d:%s", epochIDStr, priority, status),
	})

	// Update submission statistics (namespaced by protocol:market)
	statsKey := kb.VPAStats()
	if success {
		a.redisClient.HIncrBy(a.ctx, statsKey, "total_submissions_success", 1)
		a.redisClient.HIncrBy(a.ctx, statsKey, fmt.Sprintf("priority_%d_submissions_success", priority), 1)
	} else {
		a.redisClient.HIncrBy(a.ctx, statsKey, "total_submissions_failed", 1)
		a.redisClient.HIncrBy(a.ctx, statsKey, fmt.Sprintf("priority_%d_submissions_failed", priority), 1)
	}
	a.redisClient.Expire(a.ctx, statsKey, 7*24*time.Hour)

	// Update epoch state hash with priority (epochStateKey already declared above)
	if priority > 0 {
		a.redisClient.HSet(a.ctx, epochStateKey, map[string]interface{}{
			"priority":     priority,
			"last_updated": timestamp,
		})
		a.redisClient.Expire(a.ctx, epochStateKey, 7*24*time.Hour)
	}

	// Update combined epoch status (priority + submission status)
	epochStatusKey := kb.VPAEpochStatus(epochIDStr)
	// Read existing status if available
	var epochStatusData map[string]interface{}
	existingStatus, err := a.redisClient.Get(a.ctx, epochStatusKey).Result()
	if err == nil && existingStatus != "" {
		json.Unmarshal([]byte(existingStatus), &epochStatusData)
	} else {
		epochStatusData = make(map[string]interface{})
		epochStatusData["epoch_id"] = epochIDStr
		epochStatusData["data_market"] = dataMarketAddr
		epochStatusData["validator"] = a.config.VPAValidatorAddress
	}
	// Update submission fields
	epochStatusData["submission_success"] = success
	epochStatusData["submission_tx_hash"] = txHash
	epochStatusData["submission_block_number"] = blockNumber
	epochStatusData["submission_timestamp"] = timestamp
	statusJsonData, _ := json.Marshal(epochStatusData)
	a.redisClient.SetEx(a.ctx, epochStatusKey, string(statusJsonData), 7*24*time.Hour)
}

func main() {
	// Setup logging
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	if os.Getenv("DEBUG_MODE") == "true" {
		log.SetLevel(logrus.DebugLevel)
	}

	// Load configuration
	if err := config.LoadConfig(); err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}
	cfg := config.SettingsObj

	// Create and start aggregator
	aggregator, err := NewAggregator(cfg)
	if err != nil {
		log.WithError(err).Fatal("Failed to create Aggregator")
	}

	if err := aggregator.Start(); err != nil {
		log.WithError(err).Fatal("Failed to start Aggregator")
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	aggregator.Stop()
}
