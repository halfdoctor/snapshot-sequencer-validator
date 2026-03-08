package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	redislib "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/utils"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// StateWorker is a background worker that prepares datasets for monitor-api
type StateWorker struct {
	redis      *redis.Client
	pubsub     *redis.PubSub
	keyBuilder *redislib.KeyBuilder

	// In-memory counters for current metrics
	mu          sync.RWMutex
	submissions int64
	validations int64
	epochs      int64
	batches     int64
	lastReset   time.Time

	// Control
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// StateChangeEvent represents a state change from the pipeline
type StateChangeEvent struct {
	Type       string                 `json:"type"`       // submission, validation, epoch, batch
	EntityID   string                 `json:"entity_id"`  // ID of the entity
	Timestamp  int64                  `json:"timestamp"`  // Unix timestamp
	Attributes map[string]interface{} `json:"attributes"` // Additional data
}

// NewStateWorker creates a new state worker instance
func NewStateWorker(redisClient *redis.Client, keyBuilder *redislib.KeyBuilder) *StateWorker {

	return &StateWorker{
		redis:      redisClient,
		keyBuilder: keyBuilder,
		lastReset:  time.Now(),
		shutdown:   make(chan struct{}),
	}
}

// StartStateChangeListener subscribes to state changes and counts them
func (sw *StateWorker) StartStateChangeListener(ctx context.Context) {
	sw.wg.Add(1)
	defer sw.wg.Done()

	// Subscribe to state change channel
	sw.pubsub = sw.redis.Subscribe(ctx, "state:change")
	defer sw.pubsub.Close()

	log.Info("State worker listening for state changes on 'state:change' channel")

	ch := sw.pubsub.Channel()
	for {
		select {
		case <-sw.shutdown:
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}

			// Parse simple event format: "type:action:id"
			// e.g., "submission:received:123", "epoch:open:456"
			parts := strings.Split(msg.Payload, ":")
			if len(parts) < 2 {
				log.WithField("payload", msg.Payload).Debug("Invalid state change format")
				continue
			}

			eventType := parts[0]
			action := ""
			if len(parts) > 1 {
				action = parts[1]
			}
			sw.processSimpleStateChange(eventType, action)
		}
	}
}

// processSimpleStateChange updates counters from simple event format
func (sw *StateWorker) processSimpleStateChange(eventType string, action string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	switch eventType {
	case "submission":
		sw.submissions++
		stateChangesProcessed.WithLabelValues("submission").Inc()
		log.WithField("type", eventType).Debug("Received state change event")
	case "validation":
		sw.validations++
		stateChangesProcessed.WithLabelValues("validation").Inc()
		log.WithField("type", eventType).Debug("Received state change event")
	case "epoch":
		// Only count epoch opens (ignore closes to avoid double-counting)
		if action == "open" {
			sw.epochs++
			stateChangesProcessed.WithLabelValues("epoch").Inc()
			log.WithField("type", eventType).WithField("action", action).Debug("Received state change event")
		}
	case "batch":
		sw.batches++
		stateChangesProcessed.WithLabelValues("batch").Inc()
		log.WithField("type", eventType).Debug("Received state change event")
	case "part":
		// Parts are tracked but don't have separate counter
		stateChangesProcessed.WithLabelValues("part").Inc()
		log.WithField("type", eventType).Debug("Received state change event")
	default:
		log.WithField("type", eventType).Debug("Unknown event type received")
	}
}

// StartMetricsAggregator aggregates metrics every 30 seconds
func (sw *StateWorker) StartMetricsAggregator(ctx context.Context) {
	sw.wg.Add(1)
	defer sw.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Info("Metrics aggregator started - updating every 30 seconds")

	// Initial aggregation
	sw.aggregateCurrentMetrics(ctx)

	for {
		select {
		case <-sw.shutdown:
			return
		case <-ticker.C:
			sw.aggregateCurrentMetrics(ctx)
		}
	}
}

// aggregateCurrentMetrics prepares the dashboard:summary dataset
func (sw *StateWorker) aggregateCurrentMetrics(ctx context.Context) {
	start := time.Now()
	defer func() {
		aggregationDuration.WithLabelValues("current_metrics").Observe(time.Since(start).Seconds())
	}()

	sw.mu.RLock()

	// Calculate rates from in-memory counters (for recent activity)
	duration := time.Since(sw.lastReset).Seconds()
	submissionRate := float64(sw.submissions) / duration
	epochRate := float64(sw.epochs) / duration
	batchRate := float64(sw.batches) / duration

	// Get actual submission counts from Redis (more accurate)
	// Count submissions in submission queue
	submissionQueueCount, _ := sw.redis.LLen(ctx, sw.keyBuilder.SubmissionQueue()).Result()

	// Count processed submissions using ActiveEpochs set (deterministic aggregation with migration)
	// Also update submission counts in epoch state hashes
	recentProcessedSubmissions := int64(0)
	activeEpochs, err := sw.redis.SMembers(ctx, sw.keyBuilder.ActiveEpochs()).Result()
	if err == nil {
		for _, epochID := range activeEpochs {
			// Use deterministic ZSET for submission counting
			epochSubmissionsKey := sw.keyBuilder.EpochSubmissionsIds(epochID)
			count, err := sw.redis.ZCard(ctx, epochSubmissionsKey).Result()
			if err == nil {
				recentProcessedSubmissions += count
				// Update submission count in epoch state hash
				epochStateKey := sw.keyBuilder.EpochState(epochID)
				sw.redis.HSet(ctx, epochStateKey, "submissions_count", count)
				// Refresh TTL on epoch state (7 days - same as initial creation)
				sw.redis.Expire(ctx, epochStateKey, 7*24*time.Hour)
			}
		}
	}

	// Combine in-memory and Redis counts for total submissions
	totalSubmissions := sw.submissions + recentProcessedSubmissions

	sw.mu.RUnlock()

	// Count active validators using EpochValidators sets (deterministic aggregation)
	activeValidators := make(map[string]bool)
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute).Unix()

	// Get active epochs from the last 5 minutes (with migration support)
	validatorActiveEpochs, err := sw.redis.SMembers(ctx, sw.keyBuilder.ActiveEpochs()).Result()
	if err == nil {
		for _, epochID := range validatorActiveEpochs {
			// Get validators for this epoch from EpochValidators set
			validatorsKey := sw.keyBuilder.EpochValidators(epochID)
			validators, err := sw.redis.SMembers(ctx, validatorsKey).Result()
			if err == nil {
				for _, validator := range validators {
					if validator != "" {
						activeValidators[validator] = true
					}
				}
			}
		}
	}

	// Fallback: Get recent batch epochs from timeline if no active epochs
	if len(activeValidators) == 0 {
		recentBatches, _ := sw.redis.ZRangeByScore(ctx, sw.keyBuilder.MetricsBatchesTimeline(), &redis.ZRangeBy{
			Min: strconv.FormatInt(fiveMinutesAgo, 10),
			Max: "+inf",
		}).Result()

		// For each recent batch, get its validator list
		for _, batchEntry := range recentBatches {
			// Entry format: "aggregated:{epoch}" or "local:{epoch}"
			parts := strings.Split(batchEntry, ":")
			if len(parts) < 2 {
				continue
			}
			batchType := parts[0]
			epochID := parts[1]

			// Only aggregated batches have validator lists (Level 2)
			if batchType != "aggregated" {
				continue
			}

			validatorsKey := sw.keyBuilder.MetricsBatchValidators(epochID)
			validatorList, err := sw.redis.Get(ctx, validatorsKey).Result()
			if err == nil && validatorList != "" {
				// Parse validator list (stored as JSON array)
				var validators []string
				if err := json.Unmarshal([]byte(validatorList), &validators); err == nil {
					for _, v := range validators {
						if v != "" {
							activeValidators[v] = true
						}
					}
				}
			}
		}
	}

	activeCount := len(activeValidators)

	// Prepare summary data with combined metrics
	summary := map[string]interface{}{
		"submissions_total":     totalSubmissions,
		"submissions_queue":     submissionQueueCount,
		"processed_submissions": recentProcessedSubmissions,
		"epochs_total":          sw.epochs,
		"batches_total":         sw.batches,
		"submission_rate":       submissionRate,
		"epoch_rate":            epochRate,
		"batch_rate":            batchRate,
		"active_validators":     activeCount,
		"updated_at":            time.Now().Unix(),
		"measurement_duration":  duration,
	}

	// Get recent activity counts from timelines
	nowTS := time.Now().Unix()
	oneMinuteAgo := nowTS - 60
	fiveMinutesAgo = nowTS - 300

	// Count recent epochs from timeline (both open and close events)
	var recentEpochs, recentEpochs5m int64
	recentEpochs, err = sw.redis.ZCount(ctx, sw.keyBuilder.MetricsEpochsTimeline(),
		strconv.FormatInt(oneMinuteAgo, 10),
		strconv.FormatInt(nowTS, 10)).Result()
	if err != nil {
		recentEpochs = 0
	}
	summary["epochs_1m"] = recentEpochs

	// Count recent epochs in last 5 minutes
	recentEpochs5m, err = sw.redis.ZCount(ctx, sw.keyBuilder.MetricsEpochsTimeline(),
		strconv.FormatInt(fiveMinutesAgo, 10),
		strconv.FormatInt(nowTS, 10)).Result()
	if err != nil {
		recentEpochs5m = 0
	}
	summary["epochs_5m"] = recentEpochs5m

	// Count recent batches from timeline (both local and aggregated)
	var recentBatchesCount, recentBatches5m, recentSubmissions1m, recentSubmissions5m int64
	recentBatchesCount, err = sw.redis.ZCount(ctx, sw.keyBuilder.MetricsBatchesTimeline(),
		strconv.FormatInt(oneMinuteAgo, 10),
		strconv.FormatInt(nowTS, 10)).Result()
	if err != nil {
		recentBatchesCount = 0
	}
	summary["batches_1m"] = recentBatchesCount

	// Count recent batches in last 5 minutes
	recentBatches5m, err = sw.redis.ZCount(ctx, sw.keyBuilder.MetricsBatchesTimeline(),
		strconv.FormatInt(fiveMinutesAgo, 10),
		strconv.FormatInt(nowTS, 10)).Result()
	if err != nil {
		recentBatches5m = 0
	}
	summary["batches_5m"] = recentBatches5m

	// Count recent submissions from timeline
	recentSubmissions1m, err = sw.redis.ZCount(ctx, sw.keyBuilder.MetricsSubmissionsTimeline(),
		strconv.FormatInt(oneMinuteAgo, 10),
		strconv.FormatInt(nowTS, 10)).Result()
	if err != nil {
		recentSubmissions1m = 0
	}
	summary["submissions_1m"] = recentSubmissions1m

	// Count recent submissions in last 5 minutes
	recentSubmissions5m, err = sw.redis.ZCount(ctx, sw.keyBuilder.MetricsSubmissionsTimeline(),
		strconv.FormatInt(fiveMinutesAgo, 10),
		strconv.FormatInt(nowTS, 10)).Result()
	if err != nil {
		recentSubmissions5m = 0
	}
	summary["submissions_5m"] = recentSubmissions5m

	// Aggregate VPA metrics (priority assignments and submissions)
	vpaMetrics := sw.aggregateVPAMetrics(ctx)
	// Add VPA metrics to summary (safe to range over nil map)
	for k, v := range vpaMetrics {
		summary[k] = v
	}

	// Detect epoch gaps
	gapCount := sw.detectEpochGaps(ctx)
	summary["epoch_gaps_count"] = gapCount
	summary["epoch_gaps_rate"] = float64(gapCount) / float64(recentEpochs5m+1) // Avoid division by zero

	// Store summary with TTL
	summaryJSON, _ := json.Marshal(summary)
	sw.redis.Set(ctx, sw.keyBuilder.DashboardSummary(), summaryJSON, 60*time.Second)

	// Also store as hash for easier access
	sw.redis.HMSet(ctx, sw.keyBuilder.StatsCurrent(), summary)
	sw.redis.Expire(ctx, sw.keyBuilder.StatsCurrent(), 60*time.Second)

	datasetsGenerated.WithLabelValues("dashboard_summary").Inc()

	log.WithFields(logrus.Fields{
		"submissions_total": totalSubmissions,
		"queue_submissions": submissionQueueCount,
		"epochs":            sw.epochs,
		"batches":           sw.batches,
		"validators":        activeCount,
		"epochs_1m":         recentEpochs,
		"batches_1m":        recentBatchesCount,
	}).Debug("Updated dashboard summary")

	// Aggregate participation metrics
	sw.aggregateParticipationMetrics(ctx)

	// Aggregate current epoch status
	sw.aggregateCurrentEpochStatus(ctx)

	// Aggregate VPA metrics separately (for dedicated VPA endpoints)
	sw.aggregateVPAMetricsForAPI(ctx)
}

// aggregateVPAMetrics aggregates VPA metrics for dashboard summary
func (sw *StateWorker) aggregateVPAMetrics(ctx context.Context) map[string]interface{} {
	vpaMetrics := make(map[string]interface{})

	// Get VPA stats from Redis
	statsKey := sw.keyBuilder.VPAStats()
	stats, err := sw.redis.HGetAll(ctx, statsKey).Result()
	if err != nil {
		log.WithError(err).Debug("Failed to get VPA stats")
		return nil
	}

	// Parse stats and add to metrics
	if len(stats) > 0 {
		// Priority assignment counts
		if totalAssignments, ok := stats["total_priority_assignments"]; ok {
			if val, err := strconv.ParseInt(totalAssignments, 10, 64); err == nil {
				vpaMetrics["vpa_priority_assignments_total"] = val
			}
		}
		if noPriority, ok := stats["no_priority_count"]; ok {
			if val, err := strconv.ParseInt(noPriority, 10, 64); err == nil {
				vpaMetrics["vpa_no_priority_count"] = val
			}
		}

		// Submission counts
		if success, ok := stats["total_submissions_success"]; ok {
			if val, err := strconv.ParseInt(success, 10, 64); err == nil {
				vpaMetrics["vpa_submissions_success"] = val
			}
		}
		if failed, ok := stats["total_submissions_failed"]; ok {
			if val, err := strconv.ParseInt(failed, 10, 64); err == nil {
				vpaMetrics["vpa_submissions_failed"] = val
			}
		}

		// Calculate success rate
		if success, ok := vpaMetrics["vpa_submissions_success"].(int64); ok {
			if failed, ok := vpaMetrics["vpa_submissions_failed"].(int64); ok {
				total := success + failed
				if total > 0 {
					vpaMetrics["vpa_submission_success_rate"] = float64(success) / float64(total) * 100
				}
			}
		}
	}

	// Count recent priority assignments from timeline (last 24 hours)
	nowTS := time.Now().Unix()
	twentyFourHoursAgo := nowTS - (24 * 3600)
	priorityTimelineKey := sw.keyBuilder.VPAPriorityTimeline()
	recentPriorities, err := sw.redis.ZCount(ctx, priorityTimelineKey,
		strconv.FormatInt(twentyFourHoursAgo, 10),
		strconv.FormatInt(nowTS, 10)).Result()
	if err == nil {
		vpaMetrics["vpa_priority_assignments_24h"] = recentPriorities
	}

	// Count recent submissions from timeline (last 24 hours)
	submissionTimelineKey := sw.keyBuilder.VPASubmissionTimeline()
	recentSubmissions, err := sw.redis.ZCount(ctx, submissionTimelineKey,
		strconv.FormatInt(twentyFourHoursAgo, 10),
		strconv.FormatInt(nowTS, 10)).Result()
	if err == nil {
		vpaMetrics["vpa_submissions_24h"] = recentSubmissions
	}

	return vpaMetrics
}

// aggregateVPAMetricsForAPI aggregates VPA metrics for dedicated API endpoints
func (sw *StateWorker) aggregateVPAMetricsForAPI(ctx context.Context) {
	// This function prepares detailed VPA metrics for /vpa/stats endpoint
	// The detailed stats are already stored in Redis by aggregator, so we just ensure TTL
	statsKey := sw.keyBuilder.VPAStats()
	exists, _ := sw.redis.Exists(ctx, statsKey).Result()
	if exists > 0 {
		// Refresh TTL to keep stats available
		sw.redis.Expire(ctx, statsKey, 7*24*time.Hour)
	}
}

// StartHourlyStatsWorker prepares hourly statistics every 5 minutes
func (sw *StateWorker) StartHourlyStatsWorker(ctx context.Context) {
	sw.wg.Add(1)
	defer sw.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Info("Hourly stats worker started - updating every 5 minutes")

	// Initial aggregation
	sw.aggregateHourlyStats(ctx)

	for {
		select {
		case <-sw.shutdown:
			return
		case <-ticker.C:
			sw.aggregateHourlyStats(ctx)
		}
	}
}

// aggregateHourlyStats prepares stats for the current and previous hours
func (sw *StateWorker) aggregateHourlyStats(ctx context.Context) {
	start := time.Now()
	defer func() {
		aggregationDuration.WithLabelValues("hourly_stats").Observe(time.Since(start).Seconds())
	}()

	now := time.Now()
	currentHour := now.Truncate(time.Hour)
	previousHour := currentHour.Add(-time.Hour)

	// Aggregate stats for current hour
	sw.aggregateHourPeriod(ctx, currentHour, now)

	// Aggregate stats for previous hour (complete hour)
	sw.aggregateHourPeriod(ctx, previousHour, currentHour)

	datasetsGenerated.WithLabelValues("hourly_stats").Inc()
}

// aggregateHourPeriod aggregates stats for a specific hour
func (sw *StateWorker) aggregateHourPeriod(ctx context.Context, hourStart, hourEnd time.Time) {
	startTS := hourStart.Unix()
	endTS := hourEnd.Unix()

	// Count events in this hour (submissions/validations not tracked per market)
	epochs, err := sw.redis.ZCount(ctx, sw.keyBuilder.MetricsEpochsTimeline(),
		strconv.FormatInt(startTS, 10),
		strconv.FormatInt(endTS, 10)).Result()
	if err != nil {
		epochs = 0
	}

	batches, err := sw.redis.ZCount(ctx, sw.keyBuilder.MetricsBatchesTimeline(),
		strconv.FormatInt(startTS, 10),
		strconv.FormatInt(endTS, 10)).Result()
	if err != nil {
		batches = 0
	}

	// Count submissions using ActiveEpochs set (deterministic aggregation with migration)
	submissionCount := int64(0)
	activeEpochs, err := sw.redis.SMembers(ctx, sw.keyBuilder.ActiveEpochs()).Result()
	if err == nil {
		for _, epochID := range activeEpochs {
			// Use deterministic ZSET for submission counting
			epochSubmissionsKey := sw.keyBuilder.EpochSubmissionsIds(epochID)
			count, err := sw.redis.ZCard(ctx, epochSubmissionsKey).Result()
			if err == nil {
				submissionCount += count
			}
		}
	}

	// Count unique validators using EpochValidators sets (deterministic aggregation)
	uniqueValidatorsMap := make(map[string]bool)

	// Get active epochs for this hour period (with migration support)
	hourActiveEpochs, err := sw.redis.SMembers(ctx, sw.keyBuilder.ActiveEpochs()).Result()
	if err == nil {
		for _, epochID := range hourActiveEpochs {
			// Get validators for this epoch from EpochValidators set
			validatorsKey := sw.keyBuilder.EpochValidators(epochID)
			validators, err := sw.redis.SMembers(ctx, validatorsKey).Result()
			if err == nil {
				for _, validator := range validators {
					if validator != "" {
						uniqueValidatorsMap[validator] = true
					}
				}
			}
		}
	}

	// Fallback: Use timeline-based validator detection if no active epochs
	if len(uniqueValidatorsMap) == 0 {
		hourBatches, _ := sw.redis.ZRangeByScore(ctx, sw.keyBuilder.MetricsBatchesTimeline(), &redis.ZRangeBy{
			Min: strconv.FormatInt(startTS, 10),
			Max: strconv.FormatInt(endTS, 10),
		}).Result()

		for _, batchEntry := range hourBatches {
			parts := strings.Split(batchEntry, ":")
			if len(parts) < 2 {
				continue
			}
			batchType := parts[0]
			epochID := parts[1]

			// Only aggregated batches have validator lists (Level 2)
			if batchType != "aggregated" {
				continue
			}

			validatorsKey := sw.keyBuilder.MetricsBatchValidators(epochID)
			validatorList, err := sw.redis.Get(ctx, validatorsKey).Result()
			if err == nil && validatorList != "" {
				// Parse validator list (stored as JSON array)
				var validators []string
				if err := json.Unmarshal([]byte(validatorList), &validators); err == nil {
					for _, v := range validators {
						if v != "" {
							uniqueValidatorsMap[v] = true
						}
					}
				}
			}
		}
	}
	uniqueValidators := len(uniqueValidatorsMap)

	hourStats := map[string]interface{}{
		"hour_start":        hourStart.Format(time.RFC3339),
		"hour_end":          hourEnd.Format(time.RFC3339),
		"epochs":            epochs,
		"batches":           batches,
		"submissions":       submissionCount,
		"unique_validators": uniqueValidators,
		"updated_at":        time.Now().Unix(),
	}

	// Store with hour as key
	hourKey := sw.keyBuilder.StatsHourly(hourStart.Unix())
	statsJSON, _ := json.Marshal(hourStats)
	sw.redis.Set(ctx, hourKey, statsJSON, 2*time.Hour)

	log.WithFields(logrus.Fields{
		"hour":        hourStart.Format("15:04"),
		"epochs":      epochs,
		"batches":     batches,
		"submissions": submissionCount,
		"validators":  uniqueValidators,
	}).Debug("Updated hourly stats")
}

// StartDailyStatsWorker prepares daily statistics every hour
func (sw *StateWorker) StartDailyStatsWorker(ctx context.Context) {
	sw.wg.Add(1)
	defer sw.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	log.Info("Daily stats worker started - updating every hour")

	// Initial aggregation
	sw.aggregateDailyStats(ctx)

	for {
		select {
		case <-sw.shutdown:
			return
		case <-ticker.C:
			sw.aggregateDailyStats(ctx)
		}
	}
}

// aggregateDailyStats prepares stats for the last 24 hours
func (sw *StateWorker) aggregateDailyStats(ctx context.Context) {
	start := time.Now()
	defer func() {
		aggregationDuration.WithLabelValues("daily_stats").Observe(time.Since(start).Seconds())
	}()

	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)

	startTS := dayAgo.Unix()
	endTS := now.Unix()

	// Count events in last 24 hours
	epochs, err := sw.redis.ZCount(ctx, sw.keyBuilder.MetricsEpochsTimeline(),
		strconv.FormatInt(startTS, 10),
		strconv.FormatInt(endTS, 10)).Result()
	if err != nil {
		epochs = 0
	}

	batches, err := sw.redis.ZCount(ctx, sw.keyBuilder.MetricsBatchesTimeline(),
		strconv.FormatInt(startTS, 10),
		strconv.FormatInt(endTS, 10)).Result()
	if err != nil {
		batches = 0
	}

	// Count submissions using ActiveEpochs set (deterministic aggregation with migration)
	submissionCount := int64(0)
	activeEpochs, err := sw.redis.SMembers(ctx, sw.keyBuilder.ActiveEpochs()).Result()
	if err == nil {
		for _, epochID := range activeEpochs {
			// Use deterministic ZSET for submission counting
			epochSubmissionsKey := sw.keyBuilder.EpochSubmissionsIds(epochID)
			count, err := sw.redis.ZCard(ctx, epochSubmissionsKey).Result()
			if err == nil {
				submissionCount += count
			}
		}
	}

	// Get hourly breakdown
	hourlyBreakdown := make([]map[string]interface{}, 0, 24)
	for i := 0; i < 24; i++ {
		hourStart := dayAgo.Add(time.Duration(i) * time.Hour)
		hourEnd := hourStart.Add(time.Hour)

		hourEpochs, _ := sw.redis.ZCount(ctx, sw.keyBuilder.MetricsEpochsTimeline(),
			strconv.FormatInt(hourStart.Unix(), 10),
			strconv.FormatInt(hourEnd.Unix(), 10)).Result()

		hourlyBreakdown = append(hourlyBreakdown, map[string]interface{}{
			"hour":   hourStart.Format("15:04"),
			"epochs": hourEpochs,
		})
	}

	dailyStats := map[string]interface{}{
		"period_start":      dayAgo.Format(time.RFC3339),
		"period_end":        now.Format(time.RFC3339),
		"epochs_total":      epochs,
		"batches_total":     batches,
		"submissions_total": submissionCount,
		"hourly_breakdown":  hourlyBreakdown,
		"updated_at":        time.Now().Unix(),
	}

	// Store daily stats
	statsJSON, _ := json.Marshal(dailyStats)
	sw.redis.Set(ctx, sw.keyBuilder.StatsDaily(), statsJSON, 24*time.Hour)

	datasetsGenerated.WithLabelValues("daily_stats").Inc()

	log.WithFields(logrus.Fields{
		"epochs":      epochs,
		"batches":     batches,
		"submissions": submissionCount,
	}).Info("Updated daily stats")
}

// StartPruningWorker removes old data from sorted sets every hour
func (sw *StateWorker) StartPruningWorker(ctx context.Context) {
	sw.wg.Add(1)
	defer sw.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	log.Info("Pruning worker started - cleaning old data every hour")

	for {
		select {
		case <-sw.shutdown:
			return
		case <-ticker.C:
			sw.pruneOldData(ctx)
		}
	}
}

// pruneOldData removes entries older than 24 hours from timeline sorted sets
func (sw *StateWorker) pruneOldData(ctx context.Context) {
	cutoff := time.Now().Add(-24 * time.Hour).Unix()

	// Prune all timeline sorted sets (critical for preventing unbounded growth)
	timelines := []string{
		sw.keyBuilder.MetricsEpochsTimeline(),
		sw.keyBuilder.MetricsBatchesTimeline(),
		sw.keyBuilder.MetricsSubmissionsTimeline(),
		sw.keyBuilder.MetricsValidationsTimeline(),
		// Also prune non-namespaced timeline (legacy code may still write to it)
		"metrics:submissions:timeline",
		"metrics:validations:timeline",
		"metrics:epochs:timeline",
		"metrics:batches:timeline",
	}

	totalRemoved := int64(0)
	for _, timeline := range timelines {
		removed, err := sw.redis.ZRemRangeByScore(ctx, timeline,
			"-inf",
			strconv.FormatInt(cutoff, 10)).Result()
		if err != nil {
			log.WithError(err).WithField("timeline", timeline).Error("Failed to prune old data")
			continue
		}
		if removed > 0 {
			log.WithFields(logrus.Fields{
				"timeline": timeline,
				"removed":  removed,
			}).Info("Pruned old timeline entries")
		}
		totalRemoved += removed
	}

	if totalRemoved > 0 {
		log.WithField("removed_entries", totalRemoved).Info("Pruned old timeline data")
	}

	// Prune epochs:active SET to remove epochs older than 7 days
	// This prevents unbounded growth when TTL keeps getting refreshed
	activeEpochsKey := sw.keyBuilder.ActiveEpochs()
	activeEpochs, err := sw.redis.SMembers(ctx, activeEpochsKey).Result()
	if err == nil && len(activeEpochs) > 0 {
		// Get current epoch from timeline to determine cutoff
		currentEpochStr := ""
		recentEpochs, err := sw.redis.ZRevRangeByScore(ctx, sw.keyBuilder.MetricsEpochsTimeline(),
			&redis.ZRangeBy{
				Min:    strconv.FormatInt(cutoff, 10),
				Max:    "+inf",
				Offset: 0,
				Count:  1,
			}).Result()
		if err == nil && len(recentEpochs) > 0 {
			// Extract epoch ID from timeline entry (format: "open:{epochId}" or "closed:{epochId}")
			entry := recentEpochs[0]
			parts := strings.Split(entry, ":")
			if len(parts) >= 2 {
				currentEpochStr = parts[1]
			}
		}

		if currentEpochStr != "" {
			currentEpoch, err := strconv.ParseInt(currentEpochStr, 10, 64)
			if err == nil {
				cutoffEpoch := currentEpoch - 10080 // Keep last 7 days (assuming ~1 epoch per minute)
				epochsToRemove := []string{}
				for _, epochStr := range activeEpochs {
					epoch, err := strconv.ParseInt(epochStr, 10, 64)
					if err == nil && epoch < cutoffEpoch {
						epochsToRemove = append(epochsToRemove, epochStr)
					}
				}
				if len(epochsToRemove) > 0 {
					removed, err := sw.redis.SRem(ctx, activeEpochsKey, epochsToRemove).Result()
					if err != nil {
						log.WithError(err).Error("Failed to prune old epochs from ActiveEpochs set")
					} else if removed > 0 {
						log.WithFields(logrus.Fields{
							"removed_epochs": removed,
							"remaining":      len(activeEpochs) - int(removed),
						}).Info("Pruned old epochs from ActiveEpochs set")
					}
				}
			}
		}
	}

	// Monitor Redis key sizes and alert if they exceed thresholds
	sw.monitorRedisKeySizes(ctx)

	// Also reset in-memory counters if they've been running for more than 24 hours
	sw.mu.Lock()
	if time.Since(sw.lastReset) > 24*time.Hour {
		sw.submissions = 0
		sw.validations = 0
		sw.epochs = 0
		sw.batches = 0
		sw.lastReset = time.Now()

		log.Info("Reset in-memory counters after 24 hours")
	}
	sw.mu.Unlock()
}

// monitorRedisKeySizes monitors Redis key sizes and logs warnings if they exceed thresholds
func (sw *StateWorker) monitorRedisKeySizes(ctx context.Context) {
	thresholds := map[string]int64{
		"zset": 1000000, // 1M members
		"set":  100000,  // 100K members
		"list": 10000,   // 10K items
	}

	// Check timeline zsets
	timelineKeys := []string{
		sw.keyBuilder.MetricsSubmissionsTimeline(),
		sw.keyBuilder.MetricsValidationsTimeline(),
		sw.keyBuilder.MetricsEpochsTimeline(),
		sw.keyBuilder.MetricsBatchesTimeline(),
	}

	for _, key := range timelineKeys {
		size, err := sw.redis.ZCard(ctx, key).Result()
		if err == nil && size > thresholds["zset"] {
			log.WithFields(logrus.Fields{
				"key":   key,
				"size":  size,
				"limit": thresholds["zset"],
			}).Warn("Timeline zset exceeds size threshold - pruning may not be working correctly")
		}
	}

	// Check ActiveEpochs set
	activeEpochsKey := sw.keyBuilder.ActiveEpochs()
	size, err := sw.redis.SCard(ctx, activeEpochsKey).Result()
	if err == nil && size > thresholds["set"] {
		log.WithFields(logrus.Fields{
			"key":   activeEpochsKey,
			"size":  size,
			"limit": thresholds["set"],
		}).Warn("ActiveEpochs set exceeds size threshold - pruning may not be working correctly")
	}

	// Check aggregation queue (legacy) - auto-cleanup if exceeds threshold
	aggregationQueueKey := sw.keyBuilder.AggregationQueue()
	size, err = sw.redis.LLen(ctx, aggregationQueueKey).Result()
	if err == nil && size > thresholds["list"] {
		// Legacy queue is not used by active aggregation system (uses streams instead)
		// Auto-cleanup if it exceeds threshold to prevent unbounded growth
		log.WithFields(logrus.Fields{
			"key":   aggregationQueueKey,
			"size":  size,
			"limit": thresholds["list"],
		}).Warn("Legacy aggregation queue exceeds threshold - cleaning up")

		// Delete the entire queue (it's legacy and not used)
		deleted, err := sw.redis.Del(ctx, aggregationQueueKey).Result()
		if err != nil {
			log.WithError(err).WithField("key", aggregationQueueKey).Error("Failed to cleanup legacy aggregation queue")
		} else if deleted > 0 {
			log.WithFields(logrus.Fields{
				"key":     aggregationQueueKey,
				"deleted": deleted,
			}).Info("Cleaned up legacy aggregation queue")
		}
	}

	// Also scan for ALL legacy aggregation:queue keys (multi-market support)
	// Pattern: *:*:aggregation:queue - handles multiple protocol:market combinations
	cursor := uint64(0)
	for {
		var keys []string
		keys, cursor, err = sw.redis.Scan(ctx, cursor, "*:*:aggregation:queue", 100).Result()
		if err != nil {
			log.WithError(err).Error("Failed to scan for legacy aggregation queues")
			break
		}
		for _, key := range keys {
			// Skip the one we already checked above
			if key == aggregationQueueKey {
				continue
			}
			size, err := sw.redis.LLen(ctx, key).Result()
			if err == nil && size > thresholds["list"] {
				log.WithFields(logrus.Fields{
					"key":   key,
					"size":  size,
					"limit": thresholds["list"],
				}).Warn("Legacy aggregation queue exceeds threshold - cleaning up")
				deleted, err := sw.redis.Del(ctx, key).Result()
				if err != nil {
					log.WithError(err).WithField("key", key).Error("Failed to cleanup legacy aggregation queue")
				} else if deleted > 0 {
					log.WithFields(logrus.Fields{
						"key":     key,
						"deleted": deleted,
					}).Info("Cleaned up legacy aggregation queue")
				}
			}
		}
		if cursor == 0 {
			break
		}
	}
}

// aggregateParticipationMetrics calculates participation and inclusion rates
func (sw *StateWorker) aggregateParticipationMetrics(ctx context.Context) {
	now := time.Now().Unix()
	last24h := now - 86400

	// Get sequencer ID - must match what aggregator uses
	// This is critical for accurate participation metrics
	sequencerID := viper.GetString("sequencer_id")
	if sequencerID == "" {
		// Try to get from environment as fallback
		sequencerID = os.Getenv("SEQUENCER_ID")
		if sequencerID == "" {
			sequencerID = "validator1" // final fallback - match monitor-api default
		}
	}

	// Count Level 1 batches (our local finalizations) in last 24h
	ourBatchesKey := sw.keyBuilder.MetricsValidatorBatches(sequencerID)
	level1Batches, err := sw.redis.ZCount(ctx, ourBatchesKey,
		strconv.FormatInt(last24h, 10),
		strconv.FormatInt(now, 10)).Result()
	if err != nil {
		log.WithError(err).WithField("validator_id", sequencerID).Debug("Failed to count validator batches")
		level1Batches = 0
	}

	// Count total Level 2 batches (network aggregations) in last 24h
	allLevel2Batches, err := sw.redis.ZCount(ctx, sw.keyBuilder.MetricsBatchesTimeline(),
		strconv.FormatInt(last24h, 10),
		strconv.FormatInt(now, 10)).Result()
	if err != nil {
		log.WithError(err).Debug("Failed to count aggregated batches")
		allLevel2Batches = 0
	}

	// Count Level 2 batches where we were included
	level2Inclusions := int64(0)

	// Get recent aggregated batches from timeline
	recentAggregated, err := sw.redis.ZRangeByScore(ctx, sw.keyBuilder.MetricsBatchesTimeline(), &redis.ZRangeBy{
		Min: strconv.FormatInt(last24h, 10),
		Max: strconv.FormatInt(now, 10),
	}).Result()
	if err != nil {
		log.WithError(err).Debug("Failed to get recent aggregated batches")
		recentAggregated = []string{}
	}

	// For each aggregated batch, check if we contributed
	for _, entry := range recentAggregated {
		// Entry format: "aggregated:{epoch}" or "local:{epoch}"
		if !strings.HasPrefix(entry, "aggregated:") {
			continue
		}

		epochID := strings.TrimPrefix(entry, "aggregated:")

		// Check if our validator is in the validators list
		validatorsKey := sw.keyBuilder.MetricsBatchValidators(epochID)
		validatorsJSON, err := sw.redis.Get(ctx, validatorsKey).Result()
		if err != nil {
			// Validator list might not exist yet, skip
			continue
		}

		var validators []string
		if json.Unmarshal([]byte(validatorsJSON), &validators) == nil {
			for _, v := range validators {
				if v == sequencerID {
					level2Inclusions++
					break
				}
			}
		}
	}

	// Calculate metrics
	participationRate := 0.0
	if allLevel2Batches > 0 {
		participationRate = float64(level2Inclusions) / float64(allLevel2Batches) * 100
	}

	inclusionRate := 0.0
	if level1Batches > 0 {
		inclusionRate = float64(level2Inclusions) / float64(level1Batches) * 100
		if inclusionRate > 100 {
			inclusionRate = 100 // Cap at 100%
		}
	}

	// Count epochs in last 24h (both open and close events)
	epochsTotal, err := sw.redis.ZCount(ctx, sw.keyBuilder.MetricsEpochsTimeline(),
		strconv.FormatInt(last24h, 10),
		strconv.FormatInt(now, 10)).Result()
	if err != nil {
		log.WithError(err).Debug("Failed to count epochs")
		epochsTotal = 0
	}

	// Store participation metrics
	participationMetrics := map[string]interface{}{
		"epochs_participated_24h": level1Batches,
		"epochs_total_24h":        epochsTotal,
		"participation_rate":      participationRate,
		"level1_batches_24h":      level1Batches,
		"level2_inclusions_24h":   level2Inclusions,
		"inclusion_rate":          inclusionRate,
		"timestamp":               now,
	}

	metricsJSON, _ := json.Marshal(participationMetrics)
	sw.redis.Set(ctx, sw.keyBuilder.MetricsParticipation(), metricsJSON, 60*time.Second)

	log.WithFields(logrus.Fields{
		"participation_rate": fmt.Sprintf("%.1f%%", participationRate),
		"inclusion_rate":     fmt.Sprintf("%.1f%%", inclusionRate),
		"level1_batches":     level1Batches,
		"level2_inclusions":  level2Inclusions,
	}).Debug("Updated participation metrics")
}

// aggregateCurrentEpochStatus detects current epoch and its phase
func (sw *StateWorker) aggregateCurrentEpochStatus(ctx context.Context) {
	// Get current epoch using ActiveEpochs set (deterministic aggregation with migration)
	activeEpochs, err := sw.redis.SMembers(ctx, sw.keyBuilder.ActiveEpochs()).Result()
	if err != nil || len(activeEpochs) == 0 {
		// Fallback: Get latest epoch events from timeline
		sw.aggregateCurrentEpochStatusFromTimeline(ctx)
		return
	}

	// Use the first active epoch as current epoch
	currentEpochID := utils.FormatEpochID(activeEpochs[0])
	now := time.Now().Unix()

	// Get epoch info
	infoKey := sw.keyBuilder.MetricsEpochInfo(currentEpochID)
	epochInfo, _ := sw.redis.HGetAll(ctx, infoKey).Result()

	// Get epoch timestamp from info or use current time
	currentEpochTimestamp := now
	if timestampStr, ok := epochInfo["timestamp"]; ok {
		if timestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
			currentEpochTimestamp = timestamp
		}
	}

	// Determine current phase
	phase := "unknown"
	timeRemaining := int64(0)
	submissionsReceived := int64(0)

	// Check if window is still open using epoch info
	if epochInfo != nil {
		if status, ok := epochInfo["status"]; ok && status == "open" {
			phase = "submission"

			// Calculate time remaining - read from epoch info (set by EventMonitor from contract)
			windowDuration := float64(60) // fallback default (60 seconds)
			if durationStr, ok := epochInfo["duration"]; ok {
				if d, err := strconv.ParseFloat(durationStr, 64); err == nil {
					windowDuration = d
				}
			} else {
				// If duration not found, log warning (should be set by EventMonitor)
				log.WithField("epoch_id", currentEpochID).Debug("Window duration not found in epoch info, using fallback")
			}

			windowEndTime := currentEpochTimestamp + int64(windowDuration)
			timeRemaining = windowEndTime - now
			if timeRemaining < 0 {
				timeRemaining = 0
			}
		} else {
			// Window closed, check for finalization status
			level1Key := sw.keyBuilder.MetricsBatchLocal(currentEpochID)
			level1Exists, _ := sw.redis.Exists(ctx, level1Key).Result()

			if level1Exists > 0 {
				// Check for aggregation
				level2Key := sw.keyBuilder.MetricsBatchAggregated(currentEpochID)
				level2Exists, _ := sw.redis.Exists(ctx, level2Key).Result()

				if level2Exists > 0 {
					phase = "complete"
				} else {
					phase = "aggregation"
				}
			} else {
				phase = "finalization"
			}
		}
	}

	// Count submissions in current epoch (from deterministic ZSET)
	epochSubmissionsKey := sw.keyBuilder.EpochSubmissionsIds(currentEpochID)
	if count, err := sw.redis.ZCard(ctx, epochSubmissionsKey).Result(); err == nil {
		submissionsReceived = count
	}

	currentEpochStatus := map[string]interface{}{
		"epoch_id":               currentEpochID,
		"phase":                  phase,
		"time_remaining_seconds": timeRemaining,
		"submissions_received":   submissionsReceived,
		"timestamp":              now,
	}

	// Add epoch info if available
	if epochInfo != nil {
		if durationStr, ok := epochInfo["duration"]; ok {
			if d, err := strconv.ParseFloat(durationStr, 64); err == nil {
				currentEpochStatus["window_duration"] = d
			}
		}
		if dataMarket, ok := epochInfo["data_market"]; ok {
			currentEpochStatus["data_market"] = dataMarket
		}
	}

	// Add default window duration if not set (fallback - should be set by EventMonitor from contract)
	if _, exists := currentEpochStatus["window_duration"]; !exists {
		// Default to 60 seconds (fallback - actual value should come from contract via EventMonitor)
		currentEpochStatus["window_duration"] = 60.0
	}

	statusJSON, _ := json.Marshal(currentEpochStatus)
	sw.redis.Set(ctx, sw.keyBuilder.MetricsCurrentEpoch(), statusJSON, 30*time.Second)

	log.WithFields(logrus.Fields{
		"epoch_id":       currentEpochID,
		"phase":          phase,
		"time_remaining": timeRemaining,
		"submissions":    submissionsReceived,
	}).Debug("Updated current epoch status")
}

// aggregateCurrentEpochStatusFromTimeline fallback method using timeline parsing
func (sw *StateWorker) aggregateCurrentEpochStatusFromTimeline(ctx context.Context) {
	// Get latest epoch events from timeline (could be open or close)
	latestEpochs, err := sw.redis.ZRevRangeWithScores(ctx, sw.keyBuilder.MetricsEpochsTimeline(), 0, 4).Result()
	if err != nil || len(latestEpochs) == 0 {
		log.WithError(err).Debug("No epochs found in timeline")
		return
	}

	// Find the most recent open epoch that hasn't been closed yet
	var currentEpochID string
	var currentEpochTimestamp int64
	var epochInfo map[string]string

	now := time.Now().Unix()

	// Check recent timeline entries to find current epoch
	for _, epochEvent := range latestEpochs {
		entry := epochEvent.Member.(string)
		timestamp := int64(epochEvent.Score)
		parts := strings.Split(entry, ":")
		if len(parts) < 2 {
			continue
		}

		eventType := parts[0]
		epochID := parts[1]

		if eventType == "open" {
			// Get epoch info to check if still open
			infoKey := sw.keyBuilder.MetricsEpochInfo(epochID)
			info, _ := sw.redis.HGetAll(ctx, infoKey).Result()

			// Check if this epoch has a corresponding close event
			hasCloseEvent := false
			for _, checkEvent := range latestEpochs {
				checkEntry := checkEvent.Member.(string)
				if strings.HasPrefix(checkEntry, "close:"+epochID) {
					hasCloseEvent = true
					break
				}
			}

			if !hasCloseEvent {
				currentEpochID = utils.FormatEpochID(epochID)
				currentEpochTimestamp = timestamp
				epochInfo = info
				break
			}
		}
	}

	// If no open epoch found, use the most recent epoch
	if currentEpochID == "" {
		// Use the most recent event
		latestEvent := latestEpochs[0]
		entry := latestEvent.Member.(string)
		parts := strings.Split(entry, ":")
		if len(parts) >= 2 {
			currentEpochID = utils.FormatEpochID(parts[1])
			currentEpochTimestamp = int64(latestEvent.Score)
			infoKey := sw.keyBuilder.MetricsEpochInfo(currentEpochID)
			epochInfo, _ = sw.redis.HGetAll(ctx, infoKey).Result()
		} else {
			return
		}
	}

	// Determine current phase
	phase := "unknown"
	timeRemaining := int64(0)
	submissionsReceived := int64(0)

	// Check if window is still open using epoch info
	if epochInfo != nil {
		if status, ok := epochInfo["status"]; ok && status == "open" {
			phase = "submission"

			// Calculate time remaining - read from epoch info (set by EventMonitor from contract)
			windowDuration := float64(60) // fallback default (60 seconds)
			if durationStr, ok := epochInfo["duration"]; ok {
				if d, err := strconv.ParseFloat(durationStr, 64); err == nil {
					windowDuration = d
				}
			} else {
				// If duration not found, log warning (should be set by EventMonitor)
				log.WithField("epoch_id", currentEpochID).Debug("Window duration not found in epoch info, using fallback")
			}

			windowEndTime := currentEpochTimestamp + int64(windowDuration)
			timeRemaining = windowEndTime - now
			if timeRemaining < 0 {
				timeRemaining = 0
			}
		} else {
			// Window closed, check for finalization status
			level1Key := sw.keyBuilder.MetricsBatchLocal(currentEpochID)
			level1Exists, _ := sw.redis.Exists(ctx, level1Key).Result()

			if level1Exists > 0 {
				// Check for aggregation
				level2Key := sw.keyBuilder.MetricsBatchAggregated(currentEpochID)
				level2Exists, _ := sw.redis.Exists(ctx, level2Key).Result()

				if level2Exists > 0 {
					phase = "complete"
				} else {
					phase = "aggregation"
				}
			} else {
				phase = "finalization"
			}
		}
	}

	// Count submissions in current epoch (from deterministic ZSET)
	epochSubmissionsKey := sw.keyBuilder.EpochSubmissionsIds(currentEpochID)
	if count, err := sw.redis.ZCard(ctx, epochSubmissionsKey).Result(); err == nil {
		submissionsReceived = count
	}

	currentEpochStatus := map[string]interface{}{
		"epoch_id":               currentEpochID,
		"phase":                  phase,
		"time_remaining_seconds": timeRemaining,
		"submissions_received":   submissionsReceived,
		"timestamp":              now,
	}

	// Add epoch info if available
	if epochInfo != nil {
		if durationStr, ok := epochInfo["duration"]; ok {
			if d, err := strconv.ParseFloat(durationStr, 64); err == nil {
				currentEpochStatus["window_duration"] = d
			}
		}
		if dataMarket, ok := epochInfo["data_market"]; ok {
			currentEpochStatus["data_market"] = dataMarket
		}
	}

	// Add default window duration if not set (fallback - should be set by EventMonitor from contract)
	if _, exists := currentEpochStatus["window_duration"]; !exists {
		// Default to 60 seconds (fallback - actual value should come from contract via EventMonitor)
		currentEpochStatus["window_duration"] = 60.0
	}

	statusJSON, _ := json.Marshal(currentEpochStatus)
	sw.redis.Set(ctx, sw.keyBuilder.MetricsCurrentEpoch(), statusJSON, 30*time.Second)

	log.WithFields(logrus.Fields{
		"epoch_id":       currentEpochID,
		"phase":          phase,
		"time_remaining": timeRemaining,
		"submissions":    submissionsReceived,
		"method":         "timeline_fallback",
	}).Debug("Updated current epoch status using timeline fallback")
}

// detectEpochGaps scans active epochs and identifies gaps where finalizations are missing
func (sw *StateWorker) detectEpochGaps(ctx context.Context) int64 {
	// Get active epochs
	activeEpochs, err := sw.redis.SMembers(ctx, sw.keyBuilder.ActiveEpochs()).Result()
	if err != nil {
		return 0
	}

	gapsKey := sw.keyBuilder.EpochsGaps()
	now := time.Now().Unix()
	var gapCount int64

	// Clear old gaps (older than 1 hour)
	oneHourAgo := now - 3600
	sw.redis.ZRemRangeByScore(ctx, gapsKey, "0", strconv.FormatInt(oneHourAgo, 10))

	for _, epochID := range activeEpochs {
		epochStateKey := sw.keyBuilder.EpochState(epochID)
		stateData, err := sw.redis.HGetAll(ctx, epochStateKey).Result()
		if err != nil {
			continue
		}

		windowStatus := stateData["window_status"]
		level1Status := stateData["level1_status"]
		level2Status := stateData["level2_status"]
		onchainStatus := stateData["onchain_status"]

		// Check for gaps
		gapType := ""
		if windowStatus == "closed" && level1Status != "completed" && level1Status != "in_progress" {
			gapType = "missing_level1"
		} else if level1Status == "completed" && level2Status != "completed" && level2Status != "aggregating" && level2Status != "collecting" {
			gapType = "missing_level2"
		} else if level2Status == "completed" && onchainStatus != "confirmed" && onchainStatus != "submitted" && onchainStatus != "queued" {
			gapType = "missing_onchain"
		}

		if gapType != "" {
			// Add to gaps set
			sw.redis.ZAdd(ctx, gapsKey, redis.Z{
				Score:  float64(now),
				Member: fmt.Sprintf("%s:%s", epochID, gapType),
			})
			gapCount++
		}
	}

	// Set TTL on gaps key
	sw.redis.Expire(ctx, gapsKey, 24*time.Hour)

	return gapCount
}

// Shutdown gracefully stops the worker
func (sw *StateWorker) Shutdown() {
	close(sw.shutdown)
	sw.wg.Wait()

	if sw.pubsub != nil {
		sw.pubsub.Close()
	}

	log.Info("State worker shutdown complete")
}
