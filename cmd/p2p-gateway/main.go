package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/powerloom/snapshot-sequencer-validator/config"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/events"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/metrics"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/p2p"
	rediskeys "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/utils"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SubmissionMessage represents the structure of incoming submission messages
type SubmissionMessage struct {
	Request struct {
		SlotID      uint64 `json:"slotId"`
		Deadline    uint64 `json:"deadline"`
		SnapshotCid string `json:"snapshotCid"`
		EpochID     uint64 `json:"epochId"`
		ProjectID   string `json:"projectId"`
	} `json:"request"`
	Signature   string  `json:"signature"`
	Header      string  `json:"header"`
	DataMarket  string  `json:"dataMarket"`
	NodeVersion *string `json:"nodeVersion,omitempty"`
}

// SubmissionMetadata holds detailed information about a received submission
type SubmissionMetadata struct {
	EntityID    string `json:"entityId"`
	EpochID     uint64 `json:"epochId"`
	SlotID      uint64 `json:"slotId"`
	ProjectID   string `json:"projectId"`
	PeerID      string `json:"peerId"`
	Timestamp   int64  `json:"timestamp"`
	DataMarket  string `json:"dataMarket"`
	NodeVersion string `json:"nodeVersion"`
	MessageSize int    `json:"messageSize"`
	TopicName   string `json:"topicName"`
}

// extractSubmissionMetadata parses message data and extracts relevant information
func (g *P2PGateway) extractSubmissionMetadata(msgData []byte, peerID peer.ID, timestamp int64, topicName string) (*SubmissionMetadata, error) {
	// Try to parse the message as a SubmissionMessage
	var submissionMsg SubmissionMessage
	if err := json.Unmarshal(msgData, &submissionMsg); err != nil {
		// If parsing fails, return basic metadata without epoch/slot/project info
		log.WithError(err).Debug("Failed to parse submission message, using basic metadata")
		return &SubmissionMetadata{
			PeerID:      peerID.String(),
			Timestamp:   timestamp,
			MessageSize: len(msgData),
			TopicName:   topicName,
		}, nil
	}

	// Extract node version safely
	nodeVersion := ""
	if submissionMsg.NodeVersion != nil {
		nodeVersion = *submissionMsg.NodeVersion
	}

	return &SubmissionMetadata{
		EpochID:     submissionMsg.Request.EpochID,
		SlotID:      submissionMsg.Request.SlotID,
		ProjectID:   submissionMsg.Request.ProjectID,
		PeerID:      peerID.String(),
		Timestamp:   timestamp,
		DataMarket:  submissionMsg.DataMarket,
		NodeVersion: nodeVersion,
		MessageSize: len(msgData),
		TopicName:   topicName,
	}, nil
}

// generateEntityID creates an enhanced entity ID with detailed context
func (g *P2PGateway) generateEntityID(metadata *SubmissionMetadata) (string, string) {
	timestamp := metadata.Timestamp
	peerID := metadata.PeerID

	// Generate enhanced entity ID if we have detailed information
	if metadata.EpochID > 0 && metadata.SlotID > 0 && metadata.ProjectID != "" {
		enhancedID := fmt.Sprintf("received:%d:%d:%s:%d:%s",
			metadata.EpochID, metadata.SlotID, metadata.ProjectID, timestamp, peerID)
		return enhancedID, "enhanced"
	}

	// Fallback to legacy format for backward compatibility
	legacyID := fmt.Sprintf("received:peer-%s-%d:%d",
		peerID[:min(8, len(peerID))], timestamp, timestamp)
	return legacyID, "legacy"
}

// storeSubmissionMetadata saves detailed submission metadata in Redis for later retrieval
func (g *P2PGateway) storeSubmissionMetadata(metadata *SubmissionMetadata) error {
	// Use centralized key builder for namespaced metadata key
	metadataKey := g.keyBuilder.MetricsSubmissionsMetadata(metadata.EntityID)

	metadataMap := map[string]interface{}{
		"entityId":    metadata.EntityID,
		"epochId":     metadata.EpochID,
		"slotId":      metadata.SlotID,
		"projectId":   metadata.ProjectID,
		"peerId":      metadata.PeerID,
		"timestamp":   metadata.Timestamp,
		"dataMarket":  metadata.DataMarket,
		"nodeVersion": metadata.NodeVersion,
		"messageSize": metadata.MessageSize,
		"topicName":   metadata.TopicName,
		"storedAt":    time.Now().Unix(),
	}

	// Store metadata with 24 hour TTL
	if err := g.redisClient.HMSet(g.ctx, metadataKey, metadataMap).Err(); err != nil {
		return fmt.Errorf("failed to store submission metadata: %w", err)
	}

	if err := g.redisClient.Expire(g.ctx, metadataKey, 24*time.Hour).Err(); err != nil {
		log.WithError(err).Warn("Failed to set TTL on submission metadata")
	}

	log.Debugf("Stored submission metadata for entity %s", metadata.EntityID)
	return nil
}

// GetSubmissionMetadata retrieves detailed submission metadata from Redis
func (g *P2PGateway) GetSubmissionMetadata(entityID string) (*SubmissionMetadata, error) {
	metadataKey := g.keyBuilder.MetricsSubmissionsMetadata(entityID)

	result := g.redisClient.HGetAll(g.ctx, metadataKey)
	if result.Err() != nil {
		if result.Err() == redis.Nil {
			return nil, fmt.Errorf("submission metadata not found for entity: %s", entityID)
		}
		return nil, fmt.Errorf("failed to retrieve submission metadata: %w", result.Err())
	}

	data := result.Val()
	if len(data) == 0 {
		return nil, fmt.Errorf("submission metadata not found for entity: %s", entityID)
	}

	metadata := &SubmissionMetadata{}

	// Parse numeric fields safely
	if epochID, err := strconv.ParseUint(data["epochId"], 10, 64); err == nil {
		metadata.EpochID = epochID
	}
	if slotID, err := strconv.ParseUint(data["slotId"], 10, 64); err == nil {
		metadata.SlotID = slotID
	}
	if timestamp, err := strconv.ParseInt(data["timestamp"], 10, 64); err == nil {
		metadata.Timestamp = timestamp
	}
	if messageSize, err := strconv.Atoi(data["messageSize"]); err == nil {
		metadata.MessageSize = messageSize
	}

	// Parse string fields
	metadata.EntityID = data["entityId"]
	metadata.ProjectID = data["projectId"]
	metadata.PeerID = data["peerId"]
	metadata.DataMarket = data["dataMarket"]
	metadata.NodeVersion = data["nodeVersion"]
	metadata.TopicName = data["topicName"]

	return metadata, nil
}

type P2PGateway struct {
	ctx         context.Context
	cancel      context.CancelFunc
	p2pHost     *p2p.P2PHost
	redisClient *redis.Client
	keyBuilder  *rediskeys.KeyBuilder
	config      *config.Settings

	// Topic subscriptions
	submissionSub *pubsub.Subscription
	batchSub      *pubsub.Subscription
	presenceSub   *pubsub.Subscription

	// Topic handlers
	submissionTopic *pubsub.Topic
	batchTopic      *pubsub.Topic
	presenceTopic   *pubsub.Topic

	// Event and metrics
	eventEmitter    *events.Emitter
	eventPublisher  *events.Publisher
	metricsRegistry *metrics.Registry
}

func NewP2PGateway(cfg *config.Settings) (*P2PGateway, error) {
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

	// Initialize P2P host
	p2pHost, err := p2p.NewP2PHost(ctx, cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create P2P host: %w", err)
	}

	// Initialize KeyBuilder
	dataMarket := ""
	if len(cfg.DataMarketAddresses) > 0 {
		dataMarket = cfg.DataMarketAddresses[0]
	}
	keyBuilder := rediskeys.NewKeyBuilder(cfg.ProtocolStateContract, dataMarket)

	// Initialize event emitter
	emitterConfig := events.DefaultConfig()
	emitterConfig.BufferSize = 1000
	emitterConfig.SequencerID = "p2p-gateway"
	emitterConfig.Protocol = cfg.ProtocolStateContract
	emitterConfig.DataMarket = dataMarket
	eventEmitter := events.NewEmitter(emitterConfig)
	if err := eventEmitter.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start event emitter: %w", err)
	}

	// Initialize event publisher for Redis
	publisherConfig := &events.PublisherConfig{
		RedisClient:   redisClient,
		ChannelPrefix: "events",
	}
	eventPublisher, err := events.NewPublisher(publisherConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create event publisher: %w", err)
	}
	if err := eventPublisher.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start event publisher: %w", err)
	}

	// Subscribe emitter events to publisher
	if err := eventEmitter.Subscribe(&events.Subscriber{
		ID: "redis-publisher",
		Handler: func(event *events.Event) {
			if err := eventPublisher.Publish(event); err != nil {
				log.WithError(err).Error("Failed to publish event")
			}
		},
	}); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to subscribe emitter events: %w", err)
	}

	// Initialize metrics registry
	metricsConfig := &metrics.CollectorConfig{
		RedisAddr:          fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		RedisPassword:      cfg.RedisPassword,
		RedisDB:            cfg.RedisDB,
		RedisKeyPrefix:     "metrics",
		CollectionInterval: 10 * time.Second,
		BatchSize:          100,
		FlushInterval:      30 * time.Second,
	}
	metricsRegistry := metrics.NewRegistry(metricsConfig)

	gateway := &P2PGateway{
		ctx:             ctx,
		cancel:          cancel,
		p2pHost:         p2pHost,
		redisClient:     redisClient,
		keyBuilder:      keyBuilder,
		config:          cfg,
		eventEmitter:    eventEmitter,
		eventPublisher:  eventPublisher,
		metricsRegistry: metricsRegistry,
	}

	// Setup topic subscriptions
	if err := gateway.setupTopics(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup topics: %w", err)
	}

	// Initialize stream infrastructure (mandatory for deterministic aggregation)
	if err := gateway.initializeStreams(); err != nil {
		log.WithError(err).Fatal("Failed to initialize stream infrastructure (required for deterministic aggregation)")
	}

	return gateway, nil
}

func (g *P2PGateway) setupTopics() error {
	// Get configurable topics
	discoveryTopic, submissionsTopic := g.config.GetSnapshotSubmissionTopics()
	_, batchAllTopic := g.config.GetFinalizedBatchTopics()

	// Join snapshot submission topics
	topics := []string{
		discoveryTopic,   // Discovery topic
		submissionsTopic, // Main submissions
	}

	for _, topicName := range topics {
		topic, err := g.p2pHost.Pubsub.Join(topicName)
		if err != nil {
			return fmt.Errorf("failed to join topic %s: %w", topicName, err)
		}

		sub, err := topic.Subscribe()
		if err != nil {
			return fmt.Errorf("failed to subscribe to topic %s: %w", topicName, err)
		}

		// Store the main submission topic and subscription
		if topicName == submissionsTopic {
			g.submissionTopic = topic
			g.submissionSub = sub
		}

		log.Infof("📡 Subscribed to topic: %s", topicName)

		// Handle messages for each topic
		go g.handleSubmissionMessages(sub, topicName)
	}

	// Finalized batches topic
	batchTopic, err := g.p2pHost.Pubsub.Join(batchAllTopic)
	if err != nil {
		return fmt.Errorf("failed to join batch topic: %w", err)
	}
	g.batchTopic = batchTopic

	g.batchSub, err = batchTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to batch topic: %w", err)
	}
	log.Infof("📡 Subscribed to topic: %s", batchAllTopic)

	// Validator presence topic
	presenceTopic, err := g.p2pHost.Pubsub.Join(g.config.GossipsubValidatorPresenceTopic)
	if err != nil {
		return fmt.Errorf("failed to join presence topic: %w", err)
	}
	g.presenceTopic = presenceTopic

	g.presenceSub, err = presenceTopic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to presence topic: %w", err)
	}
	log.Infof("📡 Subscribed to topic: %s", g.config.GossipsubValidatorPresenceTopic)

	log.Info("P2P Gateway: Subscribed to all topics")
	return nil
}

// initializeStreams sets up Redis streams for deterministic aggregation
func (g *P2PGateway) initializeStreams() error {
	streamKey := g.keyBuilder.AggregationStream()

	log.WithField("stream", streamKey).Info("Initializing Redis streams infrastructure")

	// Initialize the aggregation stream with proper configuration
	// Use XGROUP CREATE MKSTREAM to atomically create both stream and group
	groupName := g.config.StreamConsumerGroup

	// Try to create consumer group with stream (atomic operation)
	err := g.redisClient.XGroupCreateMkStream(g.ctx, streamKey, groupName, "0").Err()
	if err != nil {
		if err.Error() == "BUSYGROUP Consumer Group name already exists" {
			log.WithFields(logrus.Fields{
				"stream": streamKey,
				"group":  groupName,
			}).Info("Consumer group already exists, verifying stream state")

			// Verify the stream exists and is accessible
			info, err := g.redisClient.XInfoStream(g.ctx, streamKey).Result()
			if err != nil {
				return fmt.Errorf("stream exists but is not accessible: %w", err)
			}

			log.WithFields(logrus.Fields{
				"stream":  streamKey,
				"entries": info.Length,
				"last_id": info.LastGeneratedID,
				"groups":  info.Groups,
			}).Info("Stream verified and ready")
		} else {
			return fmt.Errorf("failed to create consumer group and stream: %w", err)
		}
	} else {
		log.WithFields(logrus.Fields{
			"stream": streamKey,
			"group":  groupName,
		}).Info("Created new stream and consumer group")
	}

	// Set up stream monitoring
	go g.monitorStreamHealth()

	// Set up periodic stream cleanup
	go g.cleanupOldStreamEntries()

	return nil
}

// monitorStreamHealth monitors the health of the aggregation stream
func (g *P2PGateway) monitorStreamHealth() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			streamKey := g.keyBuilder.AggregationStream()
			groupName := g.config.StreamConsumerGroup

			// Check stream info
			info, err := g.redisClient.XInfoStream(g.ctx, streamKey).Result()
			if err != nil {
				log.WithError(err).Error("Failed to get stream info")
				continue
			}

			// Check consumer group info
			groups, err := g.redisClient.XInfoGroups(g.ctx, streamKey).Result()
			if err != nil {
				log.WithError(err).Error("Failed to get consumer groups")
				continue
			}

			// Find our consumer group
			var ourGroup *redis.XInfoGroup
			for _, group := range groups {
				if group.Name == groupName {
					ourGroup = &group
					break
				}
			}

			if ourGroup == nil {
				log.WithField("group", groupName).Error("Our consumer group not found")
				// Try to recreate it
				if err := g.redisClient.XGroupCreateMkStream(g.ctx, streamKey, groupName, "0").Err(); err != nil {
					log.WithError(err).Error("Failed to recreate consumer group")
				}
				continue
			}

			// Log stream health metrics
			log.WithFields(logrus.Fields{
				"stream":    streamKey,
				"entries":   info.Length,
				"pending":   ourGroup.Pending,
				"last_id":   info.LastGeneratedID,
				"consumers": ourGroup.Consumers,
				"group":     groupName,
			}).Debug("Stream health check")

			// Emit stream health event
			payload, _ := json.Marshal(map[string]interface{}{
				"stream_entries":   info.Length,
				"pending_messages": ourGroup.Pending,
				"active_consumers": ourGroup.Consumers,
				"last_id":          info.LastGeneratedID,
			})
			if err := g.eventEmitter.Emit(&events.Event{
				Type:      events.EventStreamHealth,
				Severity:  events.SeverityDebug,
				Component: "p2p-gateway",
				Timestamp: time.Now(),
				Payload:   json.RawMessage(payload),
			}); err != nil {
				log.WithError(err).Error("Failed to emit stream health event")
			}
		}
	}
}

// cleanupOldStreamEntries removes old entries from the aggregation stream to prevent memory bloat
func (g *P2PGateway) cleanupOldStreamEntries() {
	ticker := time.NewTicker(30 * time.Minute) // Cleanup every 30 minutes
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			streamKey := g.keyBuilder.AggregationStream()

			// Get stream info to check current size
			info, err := g.redisClient.XInfoStream(g.ctx, streamKey).Result()
			if err != nil {
				log.WithError(err).Debug("Failed to get stream info for cleanup")
				continue
			}

			// Only trim if stream has more than 1000 entries
			if info.Length <= 1000 {
				continue
			}

			// Trim to keep only the last 1000 entries
			result, err := g.redisClient.XTrimMaxLenApprox(g.ctx, streamKey, 1000, 0).Result()
			if err != nil {
				log.WithError(err).Error("Failed to trim stream")
				continue
			}

			if result > 0 {
				log.WithFields(logrus.Fields{
					"stream":    streamKey,
					"trimmed":   result,
					"remaining": info.Length - result,
					"previous":  info.Length,
				}).Info("Cleaned up old stream entries")

				// Emit stream cleanup event
				payload, _ := json.Marshal(map[string]interface{}{
					"trimmed_entries":   result,
					"remaining_entries": info.Length - result,
					"stream_key":        streamKey,
				})
				if err := g.eventEmitter.Emit(&events.Event{
					Type:      events.EventStreamCleanup,
					Severity:  events.SeverityInfo,
					Component: "p2p-gateway",
					Timestamp: time.Now(),
					Payload:   json.RawMessage(payload),
				}); err != nil {
					log.WithError(err).Error("Failed to emit stream cleanup event")
				}
			}
		}
	}
}

// ensureStreamExists ensures the aggregation stream exists, recreating it if necessary
func (g *P2PGateway) ensureStreamExists() error {
	streamKey := g.keyBuilder.AggregationStream()
	groupName := g.config.StreamConsumerGroup

	// Check if stream exists
	info, err := g.redisClient.XInfoStream(g.ctx, streamKey).Result()
	if err != nil {
		// Stream doesn't exist, try to create it with consumer group
		log.WithField("stream", streamKey).Info("Stream does not exist, creating it")
		if err := g.redisClient.XGroupCreateMkStream(g.ctx, streamKey, groupName, "0").Err(); err != nil {
			return fmt.Errorf("failed to create stream and consumer group: %w", err)
		}
		log.WithFields(logrus.Fields{
			"stream": streamKey,
			"group":  groupName,
		}).Info("Created new stream and consumer group")
		return nil
	}

	// Stream exists, check if our consumer group exists
	groups, err := g.redisClient.XInfoGroups(g.ctx, streamKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get consumer groups: %w", err)
	}

	// Check if our group exists
	groupExists := false
	for _, group := range groups {
		if group.Name == groupName {
			groupExists = true
			break
		}
	}

	if !groupExists {
		// Consumer group doesn't exist, create it
		if err := g.redisClient.XGroupCreate(g.ctx, streamKey, groupName, "0").Err(); err != nil {
			return fmt.Errorf("failed to create consumer group: %w", err)
		}
		log.WithFields(logrus.Fields{
			"stream": streamKey,
			"group":  groupName,
		}).Info("Created consumer group for existing stream")
	}

	log.WithFields(logrus.Fields{
		"stream":  streamKey,
		"group":   groupName,
		"entries": info.Length,
	}).Debug("Stream verified and ready")

	return nil
}

// addToStreamWithRetry adds a message to the stream with retry logic
func (g *P2PGateway) addToStreamWithRetry(values map[string]interface{}) error {
	streamKey := g.keyBuilder.AggregationStream()
	maxRetries := 3
	retryDelay := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Ensure stream exists before adding
		if err := g.ensureStreamExists(); err != nil {
			log.WithError(err).Error("Failed to ensure stream exists")
			time.Sleep(retryDelay)
			continue
		}

		// Add message to stream
		err := g.redisClient.XAdd(g.ctx, &redis.XAddArgs{
			Stream: streamKey,
			Values: values,
		}).Err()

		if err == nil {
			return nil
		}

		log.WithError(err).WithFields(logrus.Fields{
			"attempt": attempt + 1,
			"max":     maxRetries,
			"stream":  streamKey,
		}).Warn("Failed to add message to stream, retrying")

		if attempt < maxRetries-1 {
			time.Sleep(retryDelay * time.Duration(attempt+1)) // Exponential backoff
		}
	}

	return fmt.Errorf("failed to add message to stream after %d attempts", maxRetries)
}

func (g *P2PGateway) handleSubmissionMessages(sub *pubsub.Subscription, topicName string) {
	// Get discovery topic to compare
	discoveryTopic, _ := g.config.GetSnapshotSubmissionTopics()
	isDiscoveryTopic := topicName == discoveryTopic
	topicLabel := "SUBMISSIONS"
	if isDiscoveryTopic {
		topicLabel = "DISCOVERY/TEST"
	}
	log.Infof("🎧 Started listening on %s topic: %s", topicLabel, topicName)

	for {
		msg, err := sub.Next(g.ctx)
		if err != nil {
			if g.ctx.Err() != nil {
				return
			}
			log.WithError(err).Error("Error reading message from", topicName)
			continue
		}

		// Skip own messages
		if g.p2pHost.Host != nil && msg.ReceivedFrom == g.p2pHost.Host.ID() {
			continue
		}

		topicLabel := "SUBMISSION"
		if topicName == discoveryTopic {
			topicLabel = "TEST/DISCOVERY"
		}
		log.Infof("📨 RECEIVED %s on %s from peer %s (size: %d bytes)",
			topicLabel, topicName, msg.ReceivedFrom.ShortString(), len(msg.Data))

		// Emit submission received event
		payload, _ := json.Marshal(map[string]interface{}{
			"peer_id":    msg.ReceivedFrom.String(),
			"topic_name": topicName,
			"size":       len(msg.Data),
		})
		if err := g.eventEmitter.Emit(&events.Event{
			Type:      events.EventSubmissionReceived,
			Severity:  events.SeverityInfo,
			Component: "p2p-gateway",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(payload),
		}); err != nil {
			log.WithError(err).Error("Failed to emit submission received event")
		}

		// Update metrics
		submissionsCounter := g.metricsRegistry.GetOrCreate(metrics.MetricConfig{
			Name:   "submissions.received.total",
			Type:   metrics.MetricTypeCounter,
			Help:   "Total submissions received",
			Labels: metrics.Labels{},
		})
		if counter, ok := submissionsCounter.(*metrics.Counter); ok {
			counter.Inc()
		}

		bytesCounter := g.metricsRegistry.GetOrCreate(metrics.MetricConfig{
			Name:   "submissions.received.bytes",
			Type:   metrics.MetricTypeCounter,
			Help:   "Total bytes received",
			Labels: metrics.Labels{},
		})
		if counter, ok := bytesCounter.(*metrics.Counter); ok {
			counter.Add(float64(len(msg.Data)))
		}

		// Write to submissions timeline with enhanced entity ID generation
		timestamp := time.Now().Unix()
		timelineKey := g.keyBuilder.MetricsSubmissionsTimeline()

		// Extract detailed metadata from the submission message
		metadata, err := g.extractSubmissionMetadata(msg.Data, msg.ReceivedFrom, timestamp, topicName)
		if err != nil {
			log.WithError(err).Warn("Failed to extract submission metadata, using basic info")
		}

		// Generate enhanced entity ID
		entityID, idType := g.generateEntityID(metadata)
		metadata.EntityID = entityID

		// Add entity ID to timeline
		if err := g.redisClient.ZAdd(g.ctx, timelineKey, redis.Z{
			Score:  float64(timestamp),
			Member: entityID,
		}).Err(); err != nil {
			log.WithError(err).Error("Failed to write submission to timeline")
		}

		// Store detailed metadata for enhanced monitoring
		if idType == "enhanced" {
			if err := g.storeSubmissionMetadata(metadata); err != nil {
				log.WithError(err).Warn("Failed to store submission metadata")
			}
			log.Infof("📝 Enhanced submission timeline entry: %s (epoch=%d, slot=%d, project=%s, peer=%s)",
				entityID, metadata.EpochID, metadata.SlotID, metadata.ProjectID, msg.ReceivedFrom.ShortString())
		} else {
			log.Infof("📝 Legacy submission timeline entry: %s (peer=%s)", entityID, msg.ReceivedFrom.ShortString())
		}

		// Wrap submission data with peer ID metadata for dequeuer
		submissionWithMetadata := map[string]interface{}{
			"peer_id": msg.ReceivedFrom.String(),
			"data":    string(msg.Data),
		}
		wrappedData, _ := json.Marshal(submissionWithMetadata)

		// Route to Redis for dequeuer processing (namespaced by protocol:market)
		queueKey := g.keyBuilder.SubmissionQueue()
		queueDepthBefore, _ := g.redisClient.LLen(g.ctx, queueKey).Result()

		if err := g.redisClient.LPush(g.ctx, queueKey, wrappedData).Err(); err != nil {
			log.WithError(err).Error("Failed to push submission to Redis")
			failedCounter := g.metricsRegistry.GetOrCreate(metrics.MetricConfig{
				Name:   "submissions.routing.failed",
				Type:   metrics.MetricTypeCounter,
				Help:   "Failed routing attempts",
				Labels: metrics.Labels{},
			})
			if counter, ok := failedCounter.(*metrics.Counter); ok {
				counter.Inc()
			}
		} else {
			log.Infof("✅ P2P Gateway: Routed %s to Redis queue", topicLabel)
			successCounter := g.metricsRegistry.GetOrCreate(metrics.MetricConfig{
				Name:   "submissions.routing.success",
				Type:   metrics.MetricTypeCounter,
				Help:   "Successful routing attempts",
				Labels: metrics.Labels{},
			})
			if counter, ok := successCounter.(*metrics.Counter); ok {
				counter.Inc()
			}

			// Emit queue depth change event
			queuePayload, _ := json.Marshal(map[string]interface{}{
				"queue_name":     "submission",
				"current_depth":  int(queueDepthBefore) + 1,
				"previous_depth": int(queueDepthBefore),
			})
			if err := g.eventEmitter.Emit(&events.Event{
				Type:      events.EventQueueDepthChanged,
				Severity:  events.SeverityDebug,
				Component: "p2p-gateway",
				Timestamp: time.Now(),
				Payload:   json.RawMessage(queuePayload),
			}); err != nil {
				log.WithError(err).Error("Failed to emit queue depth changed event")
			}
		}
	}
}

func (g *P2PGateway) handleIncomingBatches() {
	for {
		msg, err := g.batchSub.Next(g.ctx)
		if err != nil {
			if g.ctx.Err() != nil {
				return
			}
			log.WithError(err).Error("Failed to get next batch message")
			continue
		}

		// Ignore our own messages
		if msg.ReceivedFrom == g.p2pHost.Host.ID() {
			continue
		}

		// Parse to get epoch ID and validator ID
		var batchData map[string]interface{}
		if err := json.Unmarshal(msg.Data, &batchData); err != nil {
			log.WithError(err).Error("Failed to parse batch data")
			continue
		}

		epochID, ok := batchData["epochId"]
		if !ok {
			// Try camelCase
			epochID, ok = batchData["EpochId"]
			if !ok {
				log.Error("Batch missing epochId")
				continue
			}
		}

		// Extract validator ID from batch or use peer ID
		validatorID := msg.ReceivedFrom.String()
		if seqID, ok := batchData["sequencerId"].(string); ok && seqID != "" {
			validatorID = seqID
		} else if seqID, ok := batchData["SequencerId"].(string); ok && seqID != "" {
			validatorID = seqID
		}

		// Emit validator batch received event
		batchPayload, _ := json.Marshal(map[string]interface{}{
			"validator_id": validatorID,
			"epoch_id":     utils.FormatEpochID(epochID),
			"peer_id":      msg.ReceivedFrom.String(),
			"size":         len(msg.Data),
		})
		if err := g.eventEmitter.Emit(&events.Event{
			Type:      events.EventValidatorBatchReceived,
			Severity:  events.SeverityInfo,
			Component: "p2p-gateway",
			Timestamp: time.Now(),
			EpochID:   utils.FormatEpochID(epochID),
			Payload:   json.RawMessage(batchPayload),
		}); err != nil {
			log.WithError(err).Error("Failed to emit validator batch received event")
		}

		// Update metrics
		batchesCounter := g.metricsRegistry.GetOrCreate(metrics.MetricConfig{
			Name:   "batches.received.total",
			Type:   metrics.MetricTypeCounter,
			Help:   "Total batches received",
			Labels: metrics.Labels{},
		})
		if counter, ok := batchesCounter.(*metrics.Counter); ok {
			counter.Inc()
		}

		batchBytesCounter := g.metricsRegistry.GetOrCreate(metrics.MetricConfig{
			Name:   "batches.received.bytes",
			Type:   metrics.MetricTypeCounter,
			Help:   "Total batch bytes received",
			Labels: metrics.Labels{},
		})
		if counter, ok := batchBytesCounter.(*metrics.Counter); ok {
			counter.Add(float64(len(msg.Data)))
		}

		// Route to Redis for aggregator processing with ATOMIC PIPELINE OPERATIONS
		// Include validator ID in the key so we can track who sent what
		epochIDStr := utils.FormatEpochID(epochID)
		key := g.keyBuilder.IncomingBatch(epochIDStr, validatorID)

		// ATOMIC PIPELINE OPERATIONS for deterministic batch processing
		pipe := g.redisClient.Pipeline()

		// Store batch data
		pipe.Set(g.ctx, key, msg.Data, 30*time.Minute)

		// Add to epoch validator set (for deterministic batch discovery)
		pipe.SAdd(g.ctx, g.keyBuilder.EpochValidators(epochIDStr), validatorID)
		pipe.Expire(g.ctx, g.keyBuilder.EpochValidators(epochIDStr), 2*time.Hour)

		// CRITICAL: Add stream notification (mandatory for deterministic aggregation)
		streamValues := map[string]interface{}{
			"epoch":     epochIDStr,
			"validator": validatorID,
			"batch_key": key,
			"timestamp": time.Now().Unix(),
			"type":      "validator_batch",
		}

		// Add to stream with retry logic
		if err := g.addToStreamWithRetry(streamValues); err != nil {
			log.WithError(err).Error("Failed to add stream notification")
			// Stream notifications are mandatory - treat as critical failure
			return
		}

		// Mark epoch as active (use migration utility to ensure correct key type)
		// Note: We can't use pipeline with migration utility, so we'll add it after pipeline execution
		// This ensures the epoch is marked as active even if migration is needed

		// Execute atomic pipeline
		_, err = pipe.Exec(g.ctx)
		if err != nil {
			log.WithError(err).Error("Failed to store incoming batch with atomic operations")
			storageFailedCounter := g.metricsRegistry.GetOrCreate(metrics.MetricConfig{
				Name:   "batches.storage.failed",
				Type:   metrics.MetricTypeCounter,
				Help:   "Failed batch storage attempts",
				Labels: metrics.Labels{},
			})
			if counter, ok := storageFailedCounter.(*metrics.Counter); ok {
				counter.Inc()
			}
		} else {
			storageSuccessCounter := g.metricsRegistry.GetOrCreate(metrics.MetricConfig{
				Name:   "batches.storage.success",
				Type:   metrics.MetricTypeCounter,
				Help:   "Successful batch storage",
				Labels: metrics.Labels{},
			})
			if counter, ok := storageSuccessCounter.(*metrics.Counter); ok {
				counter.Inc()
			}

			// Mark epoch as active (with TTL to prevent unbounded growth)
			// Note: We only set TTL if missing (don't refresh on every add)
			// The set is also pruned periodically by state-tracker to remove old epochs
			activeEpochsKey := g.keyBuilder.ActiveEpochs()
			added, err := g.redisClient.SAdd(g.ctx, activeEpochsKey, epochIDStr).Result()
			if err != nil {
				log.WithError(err).Error("Failed to add epoch to ActiveEpochs set")
			} else if added > 0 {
				// Only set TTL if key doesn't already have one (24 hours - covers epoch lifecycle)
				// This prevents unnecessary TTL refreshes that would prevent expiration
				ttl := g.redisClient.TTL(g.ctx, activeEpochsKey).Val()
				if ttl == -1 { // Key exists but has no TTL
					g.redisClient.Expire(g.ctx, activeEpochsKey, 24*time.Hour)
				}
			}

			// Track validator batch activity for monitoring with timeline entries
			timestamp := time.Now().Unix()

			// Pipeline for monitoring metrics
			monitoringPipe := g.redisClient.Pipeline()

			// 1. Add to batches timeline for validator batch
			monitoringPipe.ZAdd(g.ctx, g.keyBuilder.MetricsBatchesTimeline(), redis.Z{
				Score:  float64(timestamp),
				Member: fmt.Sprintf("validator:%s:%s", validatorID, epochIDStr),
			})

			// 2. Track validator batch activity for monitoring
			validatorBatchesKey := g.keyBuilder.MetricsValidatorBatches(validatorID)
			monitoringPipe.ZAdd(g.ctx, validatorBatchesKey, redis.Z{
				Score:  float64(timestamp),
				Member: epochIDStr,
			})

			// 3. Publish state change
			monitoringPipe.Publish(g.ctx, "state:change", fmt.Sprintf("batch:validator:%s:%s", validatorID, epochIDStr))

			// Execute pipeline (ignore errors - monitoring is non-critical)
			if _, err := monitoringPipe.Exec(g.ctx); err != nil {
				log.Debugf("Failed to write validator batch monitoring metrics: %v", err)
			}

			// Format epoch ID as integer to avoid scientific notation
			epochFormatted := utils.FormatEpochID(epochID)

			// Check if epoch is already aggregated (for logging purposes)
			aggregatedKey := g.keyBuilder.BatchAggregated(epochIDStr)
			exists, _ := g.redisClient.Exists(g.ctx, aggregatedKey).Result()
			if exists != 0 {
				log.WithField("epoch", epochFormatted).Debug("Epoch already aggregated, not processing")
			}

			log.WithFields(logrus.Fields{
				"epoch": epochFormatted,
				"from":  validatorID,
			}).Info("P2P Gateway: Received finalized batch from validator (stream-based aggregation)")
		}
	}
}

func (g *P2PGateway) handleValidatorPresence() {
	for {
		msg, err := g.presenceSub.Next(g.ctx)
		if err != nil {
			if g.ctx.Err() != nil {
				return
			}
			log.WithError(err).Error("Failed to get next presence message")
			continue
		}

		// Track active validators
		validatorID := peer.ID(msg.ReceivedFrom).String()
		key := rediskeys.ValidatorActive(validatorID)
		if err := g.redisClient.Set(g.ctx, key, time.Now().Unix(), 5*time.Minute).Err(); err != nil {
			log.WithError(err).Error("Failed to track validator presence")
		}
	}
}

func (g *P2PGateway) handleOutgoingMessages() {
	// Watch for messages to broadcast from other components
	// Get namespaced queue key
	broadcastQueueKey := g.keyBuilder.OutgoingBroadcastBatch()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
			// Check for outgoing batch broadcasts (namespaced)
			result, err := g.redisClient.BRPop(g.ctx, time.Second, broadcastQueueKey).Result()
			if err != nil {
				if err != redis.Nil {
					log.WithError(err).Debug("No outgoing messages")
				}
				continue
			}

			if len(result) < 2 {
				continue
			}

			// Parse the message
			var msg map[string]interface{}
			if err := json.Unmarshal([]byte(result[1]), &msg); err != nil {
				log.WithError(err).Error("Failed to parse outgoing message")
				continue
			}

			// Determine topic based on message type
			msgType, _ := msg["type"].(string)
			var topic *pubsub.Topic

			switch msgType {
			case "batch", "finalized_batch":
				topic = g.batchTopic
				log.WithField("type", msgType).Info("Broadcasting finalized batch to validator network")
			case "presence":
				topic = g.presenceTopic
			default:
				log.WithField("type", msgType).Error("Unknown message type")
				continue
			}

			// Broadcast the message
			data, _ := json.Marshal(msg["data"])
			if err := topic.Publish(g.ctx, data); err != nil {
				log.WithError(err).Error("Failed to broadcast message")
			} else {
				epochID := msg["epochId"]
				log.WithField("epoch", utils.FormatEpochID(epochID)).Info("P2P Gateway: Broadcast batch to network")
			}
		}
	}
}

func (g *P2PGateway) sendPresenceHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			presence := map[string]interface{}{
				"peer_id":   g.p2pHost.Host.ID().String(),
				"timestamp": time.Now().Unix(),
				"version":   "1.0.0",
			}

			data, _ := json.Marshal(presence)
			if err := g.presenceTopic.Publish(g.ctx, data); err != nil {
				log.WithError(err).Error("Failed to send presence heartbeat")
			}
		}
	}
}

func (g *P2PGateway) Start() error {
	log.Info("Starting P2P Gateway")

	// Start all handlers (submission handlers already started in setupTopics)
	go g.handleIncomingBatches()
	go g.handleValidatorPresence()
	go g.handleOutgoingMessages()
	go g.sendPresenceHeartbeat()

	// Log connection info
	addrs := g.p2pHost.Host.Addrs()
	for _, addr := range addrs {
		if !strings.Contains(addr.String(), "127.0.0.1") && !strings.Contains(addr.String(), "::1") {
			multiaddr := fmt.Sprintf("%s/p2p/%s", addr, g.p2pHost.Host.ID())
			log.WithField("multiaddr", multiaddr).Info("P2P Gateway listening")
		}
	}

	// Monitor connected peers with enhanced diagnostics
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-g.ctx.Done():
				return
			case <-ticker.C:
				peers := g.p2pHost.Host.Network().Peers()
				submissionPeers := g.submissionTopic.ListPeers()
				batchPeers := g.batchTopic.ListPeers()
				presencePeers := g.presenceTopic.ListPeers()

				// Enhanced peer diagnostics
				peerIDs := make([]string, len(peers))
				for i, p := range peers {
					peerIDs[i] = p.ShortString()
				}

				log.WithFields(logrus.Fields{
					"connected_peers":  len(peers),
					"peer_ids":         peerIDs,
					"submission_peers": len(submissionPeers),
					"batch_peers":      len(batchPeers),
					"presence_peers":   len(presencePeers),
					"bootstrap_config": len(g.config.BootstrapPeers),
					"dht_ready":        g.p2pHost.DHT != nil,
					"pubsub_ready":     g.p2pHost.Pubsub != nil,
				}).Info("P2P Gateway status - DIAGNOSTIC")

				// Add timeline entries for peer discovery events
				timestamp := time.Now().Unix()
				monitoringPipe := g.redisClient.Pipeline()

				// Add peer discovery event to timeline
				monitoringPipe.ZAdd(g.ctx, g.keyBuilder.MetricsBatchesTimeline(), redis.Z{
					Score:  float64(timestamp),
					Member: fmt.Sprintf("peer_discovery:%d:%d", len(peers), timestamp),
				})

				// Publish state change for peer discovery
				monitoringPipe.Publish(g.ctx, "state:change", fmt.Sprintf("peers:connected:%d", len(peers)))

				// Execute pipeline (ignore errors - monitoring is non-critical)
				if _, err := monitoringPipe.Exec(g.ctx); err != nil {
					log.Debugf("Failed to write peer discovery monitoring metrics: %v", err)
				}

				// Alert if no peers connected but bootstrap configured
				if len(peers) == 0 && len(g.config.BootstrapPeers) > 0 {
					log.WithFields(logrus.Fields{
						"bootstrap_count": len(g.config.BootstrapPeers),
						"bootstrap_peers": g.config.BootstrapPeers,
					}).Error("NO PEERS CONNECTED - Check bootstrap connectivity")
				}
			}
		}
	}()

	return nil
}

func (g *P2PGateway) Stop() {
	log.Info("Stopping P2P Gateway")

	// Stop event emitter and publisher
	if g.eventEmitter != nil {
		if err := g.eventEmitter.Stop(); err != nil {
			log.WithError(err).Error("Failed to stop event emitter")
		}
	}
	if g.eventPublisher != nil {
		if err := g.eventPublisher.Stop(); err != nil {
			log.WithError(err).Error("Failed to stop event publisher")
		}
	}

	g.cancel()
	if g.p2pHost != nil {
		g.p2pHost.Close()
	}
	if g.redisClient != nil {
		g.redisClient.Close()
	}
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

	// Create and start gateway
	gateway, err := NewP2PGateway(cfg)
	if err != nil {
		log.WithError(err).Fatal("Failed to create P2P Gateway")
	}

	if err := gateway.Start(); err != nil {
		log.WithError(err).Fatal("Failed to start P2P Gateway")
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	gateway.Stop()
}
