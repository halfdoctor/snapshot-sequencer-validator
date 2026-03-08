package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	rediskeys "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// WorkerType represents the type of worker
type WorkerType string

const (
	WorkerTypeFinalizer  WorkerType = "finalizer"
	WorkerTypeAggregator WorkerType = "aggregator"
)

// WorkerStatus represents the current state of a worker
type WorkerStatus string

const (
	WorkerStatusIdle       WorkerStatus = "idle"
	WorkerStatusProcessing WorkerStatus = "processing"
	WorkerStatusFailed     WorkerStatus = "failed"
)

// WorkerMonitor handles monitoring and tracking of worker status
type WorkerMonitor struct {
	redisClient   *redis.Client
	workerID      string
	workerType    WorkerType
	protocolState string
}

// NewWorkerMonitor creates a new worker monitor instance
func NewWorkerMonitor(redisClient *redis.Client, workerID string, workerType WorkerType, protocolState string) *WorkerMonitor {
	return &WorkerMonitor{
		redisClient:   redisClient,
		workerID:      workerID,
		workerType:    workerType,
		protocolState: protocolState,
	}
}

// UpdateStatus updates the worker's current status in Redis
func (wm *WorkerMonitor) UpdateStatus(status WorkerStatus) error {
	ctx := context.Background()
	key := rediskeys.WorkerStatus(string(wm.workerType), wm.workerID)

	err := wm.redisClient.Set(ctx, key, string(status), 24*time.Hour).Err()
	if err != nil {
		log.Errorf("Failed to update worker status: %v", err)
		return err
	}

	// Update heartbeat
	wm.UpdateHeartbeat()

	log.Debugf("Worker %s:%s status updated to %s", wm.workerType, wm.workerID, status)
	return nil
}

// UpdateHeartbeat updates the worker's last heartbeat timestamp
func (wm *WorkerMonitor) UpdateHeartbeat() error {
	ctx := context.Background()
	key := rediskeys.WorkerHeartbeat(string(wm.workerType), wm.workerID)

	timestamp := time.Now().Unix()
	err := wm.redisClient.Set(ctx, key, timestamp, 5*time.Minute).Err()
	if err != nil {
		log.Errorf("Failed to update worker heartbeat: %v", err)
		return err
	}

	return nil
}

// SetCurrentBatch sets the current batch/epoch being processed
func (wm *WorkerMonitor) SetCurrentBatch(batchInfo string) error {
	ctx := context.Background()

	var key string
	if wm.workerType == WorkerTypeFinalizer {
		key = rediskeys.WorkerCurrentBatch(string(wm.workerType), wm.workerID)
	} else if wm.workerType == WorkerTypeAggregator {
		key = rediskeys.WorkerCurrentEpoch(string(wm.workerType))
	}

	err := wm.redisClient.Set(ctx, key, batchInfo, 1*time.Hour).Err()
	if err != nil {
		log.Errorf("Failed to set current batch: %v", err)
		return err
	}

	return nil
}

// IncrementProcessedCount increments the count of processed items
func (wm *WorkerMonitor) IncrementProcessedCount() error {
	ctx := context.Background()
	key := rediskeys.WorkerBatchesProcessed(string(wm.workerType), wm.workerID)

	err := wm.redisClient.Incr(ctx, key).Err()
	if err != nil {
		log.Errorf("Failed to increment processed count: %v", err)
		return err
	}

	return nil
}

// TrackBatchPart tracks the status of a batch part
func TrackBatchPart(redisClient *redis.Client, epochID string, batchID int, status string) error {
	ctx := context.Background()
	key := rediskeys.BatchPartStatus(epochID, batchID)

	err := redisClient.Set(ctx, key, status, 2*time.Hour).Err()
	if err != nil {
		log.Errorf("Failed to track batch part status: %v", err)
		return err
	}

	// Update worker assignment if processing
	if status == "processing" {
		workerKey := rediskeys.BatchPartWorker(epochID, batchID)
		// Worker ID should be passed in context or as parameter
		redisClient.Set(ctx, workerKey, "worker-id", 2*time.Hour)
	}

	return nil
}

// UpdateBatchPartsProgress updates the progress of batch parts completion
func UpdateBatchPartsProgress(redisClient *redis.Client, protocolState, dataMarket, epochID string, completed, total int) error {
	ctx := context.Background()

	// Create key builder for namespaced keys
	keyBuilder := rediskeys.NewKeyBuilder(protocolState, dataMarket)
	completedKey := keyBuilder.EpochPartsCompleted(epochID)
	totalKey := keyBuilder.EpochPartsTotal(epochID)

	pipe := redisClient.Pipeline()
	pipe.Set(ctx, completedKey, completed, 2*time.Hour)
	pipe.Set(ctx, totalKey, total, 2*time.Hour)

	// Mark as ready if all parts are complete
	if completed == total && total > 0 {
		readyKey := keyBuilder.EpochPartsReady(epochID)
		pipe.Set(ctx, readyKey, "true", 2*time.Hour)

		// Push to aggregation queue - namespaced
		aggQueueKey := keyBuilder.AggregationQueueLevel1()
		aggData := map[string]interface{}{
			"epoch_id":        epochID,
			"parts_completed": completed,
			"ready_at":        time.Now().Unix(),
			"data_market":     dataMarket, // Include data market so aggregator knows which KeyBuilder to use
		}
		data, _ := json.Marshal(aggData)
		pipe.LPush(ctx, aggQueueKey, data)
		log.WithFields(map[string]interface{}{
			"epoch":     epochID,
			"completed": completed,
			"total":     total,
			"queue":     aggQueueKey,
		}).Info("📤 Pushed to aggregation queue (all parts complete)")
	} else {
		log.Debugf("Not pushing to aggregation queue yet: epoch=%s, completed=%d, total=%d", epochID, completed, total)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Errorf("Failed to update batch parts progress: %v", err)
		return err
	}

	return nil
}

// TrackFinalizedBatch tracks a fully finalized batch with IPFS and merkle root
func TrackFinalizedBatch(redisClient *redis.Client, epochID string, ipfsCID string, merkleRoot string) error {
	ctx := context.Background()
	key := rediskeys.BatchFinalized(epochID)

	pipe := redisClient.Pipeline()
	pipe.HSet(ctx, key, "ipfs_cid", ipfsCID)
	pipe.HSet(ctx, key, "merkle_root", merkleRoot)
	pipe.HSet(ctx, key, "finalized_at", time.Now().Unix())
	pipe.Expire(ctx, key, 24*time.Hour)

	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Errorf("Failed to track finalized batch: %v", err)
		return err
	}

	// Update metrics
	metricsKey := rediskeys.MetricsTotalProcessed()
	redisClient.Incr(ctx, metricsKey)

	return nil
}

// UpdatePerformanceMetrics updates system-wide performance metrics
func UpdatePerformanceMetrics(redisClient *redis.Client, processingRate float64, avgLatency int64) error {
	ctx := context.Background()

	pipe := redisClient.Pipeline()
	pipe.Set(ctx, rediskeys.MetricsProcessingRate(), fmt.Sprintf("%.2f", processingRate), 5*time.Minute)
	pipe.Set(ctx, rediskeys.MetricsAvgLatency(), avgLatency, 5*time.Minute)

	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Errorf("Failed to update performance metrics: %v", err)
		return err
	}

	return nil
}

// HeartbeatLoop runs a background goroutine to maintain worker heartbeat
func (wm *WorkerMonitor) HeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Infof("Worker %s:%s heartbeat loop stopped", wm.workerType, wm.workerID)
			return
		case <-ticker.C:
			if err := wm.UpdateHeartbeat(); err != nil {
				log.Errorf("Failed to update heartbeat for worker %s:%s: %v",
					wm.workerType, wm.workerID, err)
			}
		}
	}
}

// StartWorker initializes a worker and starts its heartbeat
func (wm *WorkerMonitor) StartWorker(ctx context.Context) {
	// Set initial status
	wm.UpdateStatus(WorkerStatusIdle)

	// Start heartbeat loop in background
	go wm.HeartbeatLoop(ctx)

	log.Infof("Worker %s:%s started with monitoring", wm.workerType, wm.workerID)
}

// ProcessingStarted marks the beginning of processing
func (wm *WorkerMonitor) ProcessingStarted(batchInfo string) {
	wm.UpdateStatus(WorkerStatusProcessing)
	wm.SetCurrentBatch(batchInfo)
}

// ProcessingCompleted marks successful completion of processing
func (wm *WorkerMonitor) ProcessingCompleted() {
	wm.UpdateStatus(WorkerStatusIdle)
	wm.IncrementProcessedCount()

	// Clear current batch
	ctx := context.Background()
	if wm.workerType == WorkerTypeFinalizer {
		key := rediskeys.WorkerCurrentBatch(string(wm.workerType), wm.workerID)
		wm.redisClient.Del(ctx, key)
	}
}

// ProcessingFailed marks a processing failure
func (wm *WorkerMonitor) ProcessingFailed(err error) {
	wm.UpdateStatus(WorkerStatusFailed)

	// Log error details
	ctx := context.Background()
	errorKey := rediskeys.WorkerLastError(string(wm.workerType), wm.workerID)
	errorInfo := map[string]interface{}{
		"error":     err.Error(),
		"timestamp": time.Now().Unix(),
	}
	data, _ := json.Marshal(errorInfo)
	wm.redisClient.Set(ctx, errorKey, data, 1*time.Hour)
}

// CleanupWorker removes worker tracking data on shutdown
func (wm *WorkerMonitor) CleanupWorker() {
	ctx := context.Background()

	// Remove all worker keys
	keys := []string{
		rediskeys.WorkerStatus(string(wm.workerType), wm.workerID),
		rediskeys.WorkerHeartbeat(string(wm.workerType), wm.workerID),
		rediskeys.WorkerCurrentBatch(string(wm.workerType), wm.workerID),
		rediskeys.WorkerBatchesProcessed(string(wm.workerType), wm.workerID),
		rediskeys.WorkerLastError(string(wm.workerType), wm.workerID),
	}

	for _, key := range keys {
		wm.redisClient.Del(ctx, key)
	}

	log.Infof("Worker %s:%s monitoring data cleaned up", wm.workerType, wm.workerID)
}
