package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/powerloom/snapshot-sequencer-validator/docs/swagger"
	keys "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/utils"
)

var log = logrus.New()

// @title DSV Pipeline Monitor API (Direct Redis)
// @version 3.0
// @description Monitoring API that reads pre-aggregated data from state-tracker worker
// @BasePath /api/v1

type MonitorAPI struct {
	redis            *redis.Client
	ctx              context.Context
	keyBuilder       *keys.KeyBuilder
	newDataMarket    string // NEW_DATA_MARKET_CONTRACT for VPA endpoints
	newProtocolState string // NEW_PROTOCOL_STATE_CONTRACT for VPA endpoints
}

// DashboardSummary provides overall system health and metrics
type DashboardSummary struct {
	ValidatorID        string                 `json:"validator_id"`
	SystemMetrics      map[string]interface{} `json:"system_metrics"`        // Rates, totals, current counts (includes VPA metrics)
	ParticipationStats map[string]interface{} `json:"participation_stats"`   // 24h participation/inclusion metrics
	CurrentStatus      map[string]interface{} `json:"current_status"`        // Real-time status (epoch, queues)
	RecentActivity     map[string]interface{} `json:"recent_activity"`       // 1m and 5m activity
	VPAMetrics         map[string]interface{} `json:"vpa_metrics,omitempty"` // VPA-specific metrics (priority assignments, submissions)
	Timestamp          time.Time              `json:"timestamp"`
}

// HourlyStats provides hourly aggregated statistics
type HourlyStats struct {
	Hours     []map[string]interface{} `json:"hours"`
	Timestamp time.Time                `json:"timestamp"`
}

// DailyStats provides 24-hour aggregated statistics
type DailyStats struct {
	Summary         map[string]interface{}   `json:"summary"`
	HourlyBreakdown []map[string]interface{} `json:"hourly_breakdown"`
	Timestamp       time.Time                `json:"timestamp"`
}

// FinalizedBatch represents a Level 1 or Level 2 batch
type FinalizedBatch struct {
	EpochID        string   `json:"epoch_id"`
	Level          int      `json:"level"`                     // 1 for local, 2 for aggregated
	ValidatorID    string   `json:"validator_id,omitempty"`    // Level 1 only
	ValidatorIDs   []string `json:"validator_ids,omitempty"`   // Level 2 only
	ValidatorCount int      `json:"validator_count,omitempty"` // Level 2 only
	ProjectCount   int      `json:"project_count"`
	IPFSCid        string   `json:"ipfs_cid,omitempty"`
	MerkleRoot     string   `json:"merkle_root,omitempty"`
	Timestamp      int64    `json:"timestamp"`
	Type           string   `json:"type"`                     // "local" or "aggregated"
	Phase          string   `json:"phase,omitempty"`          // Current phase of epoch
	OnchainStatus  string   `json:"onchain_status,omitempty"` // On-chain submission status
	HasGap         bool     `json:"has_gap,omitempty"`        // Whether epoch has a gap
}

// EpochInfo represents epoch timeline information
type EpochInfo struct {
	EpochID     string `json:"epoch_id"`
	Status      string `json:"status"` // "open", "closed"
	Phase       string `json:"phase"`  // "submission", "finalization", "aggregation", "complete"
	StartTime   int64  `json:"start_time"`
	Duration    int    `json:"duration"`
	DataMarket  string `json:"data_market,omitempty"`
	Level1Batch bool   `json:"level1_batch_exists"`
	Level2Batch bool   `json:"level2_batch_exists"`
}

// QueueStatus represents queue depth and status
type QueueStatus struct {
	QueueName string `json:"queue_name"`
	Depth     int64  `json:"depth"`
	Status    string `json:"status"` // "empty", "healthy", "moderate", "high", "critical"
}

// SubmissionInfo represents a single submission with detailed metadata
type SubmissionInfo struct {
	EntityID    string `json:"entity_id"` // Enhanced entity ID: received:{epoch}:{slot}:{project}:{timestamp}:{peer}
	EpochID     string `json:"epoch_id"`
	SlotID      string `json:"slot_id"`
	ProjectID   string `json:"project_id"`
	SnapshotCID string `json:"snapshot_cid,omitempty"`
	PeerID      string `json:"peer_id,omitempty"`
	ValidatorID string `json:"validator_id,omitempty"`
	Timestamp   int64  `json:"timestamp"`
	Time        string `json:"time"` // RFC3339 formatted time
}

// TimelineEventMetadata contains enriched metadata for timeline events
type TimelineEventMetadata struct {
	EpochID        string `json:"epoch_id,omitempty"`
	SlotID         string `json:"slot_id,omitempty"`
	ProjectID      string `json:"project_id,omitempty"`
	PeerID         string `json:"peer_id,omitempty"`
	ValidatorID    string `json:"validator_id,omitempty"`
	DataMarket     string `json:"data_market,omitempty"`
	SubmissionType string `json:"submission_type,omitempty"` // "snapshot", "validation"
	EventType      string `json:"event_type,omitempty"`      // "submission", "validation", "batch", "epoch"
	IsEnhanced     bool   `json:"is_enhanced"`               // Whether entity ID uses new enhanced format
}

// EnhancedTimelineEvent represents a timeline event with enriched metadata
type EnhancedTimelineEvent struct {
	EntityID  string                 `json:"entity_id"`
	Timestamp int64                  `json:"timestamp"`
	Time      string                 `json:"time"`
	Metadata  *TimelineEventMetadata `json:"metadata,omitempty"`
}

// EnhancedTimelineResponse is the response structure for the enhanced timeline endpoint
type EnhancedTimelineResponse struct {
	Type      string                  `json:"type"`
	Minutes   int                     `json:"minutes"`
	Count     int                     `json:"count"`
	Events    []EnhancedTimelineEvent `json:"events"`
	Timestamp time.Time               `json:"timestamp"`
}

// VPAEpochStatusResponse represents VPA status for a specific epoch
type VPAEpochStatusResponse struct {
	EpochID   string                 `json:"epoch_id"`
	Status    map[string]interface{} `json:"status"` // Combined priority and submission status
	Timestamp time.Time              `json:"timestamp"`
}

// VPATimelineEntry represents a single timeline entry for priority or submission
type VPATimelineEntry struct {
	EpochID   string `json:"epoch_id"`
	Priority  string `json:"priority"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

// VPATimelineResponse represents the VPA timeline response
type VPATimelineResponse struct {
	Timeline  map[string]interface{} `json:"timeline"` // Contains "priority_timeline" and/or "submission_timeline"
	Timestamp time.Time              `json:"timestamp"`
}

// VPAStatsResponse represents aggregated VPA statistics
type VPAStatsResponse struct {
	Stats     map[string]interface{} `json:"stats"` // Contains priority assignments, submissions, success rates, etc.
	Timestamp time.Time              `json:"timestamp"`
}

func NewMonitorAPI(redisClient *redis.Client, protocol, market string) *MonitorAPI {
	newMarket := getEnv("NEW_DATA_MARKET_CONTRACT", "")
	newProtocol := getEnv("NEW_PROTOCOL_STATE_CONTRACT", "")
	return &MonitorAPI{
		redis:            redisClient,
		ctx:              context.Background(),
		keyBuilder:       keys.NewKeyBuilder(protocol, market),
		newDataMarket:    newMarket,
		newProtocolState: newProtocol,
	}
}

// @Summary Dashboard summary
// @Description Get pre-aggregated dashboard metrics from state-tracker
// @Tags dashboard
// @Produce json
// @Success 200 {object} DashboardSummary
// @Router /dashboard/summary [get]
func (m *MonitorAPI) DashboardSummary(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	// Read pre-aggregated dashboard summary from state-tracker (namespaced)
	summaryKey := fmt.Sprintf("%s:%s:dashboard:summary", kb.ProtocolState, kb.DataMarket)
	summaryJSON, err := m.redis.Get(m.ctx, summaryKey).Result()
	if err != nil && err != redis.Nil {
		log.WithError(err).Error("Failed to fetch dashboard summary")
	}

	var summary map[string]interface{}
	if summaryJSON != "" {
		json.Unmarshal([]byte(summaryJSON), &summary)
	} else {
		summary = make(map[string]interface{})
	}

	// Read current stats hash from state-tracker (namespaced)
	statsKey := fmt.Sprintf("%s:%s:stats:current", kb.ProtocolState, kb.DataMarket)
	currentStats, err := m.redis.HGetAll(m.ctx, statsKey).Result()
	if err != nil && err != redis.Nil {
		log.WithError(err).Error("Failed to fetch current stats")
	}

	// Convert string map to interface map
	statsMap := make(map[string]interface{})
	for k, v := range currentStats {
		// Try to parse as number
		if num, err := strconv.ParseFloat(v, 64); err == nil {
			statsMap[k] = num
		} else {
			statsMap[k] = v
		}
	}

	// Get queue depths for real-time status (use kb not m.keyBuilder)
	submissionQueueDepth, _ := m.redis.LLen(m.ctx, kb.SubmissionQueue()).Result()
	finalizationQueueDepth, _ := m.redis.LLen(m.ctx, kb.FinalizationQueue()).Result()
	aggregationQueueDepth, _ := m.redis.LLen(m.ctx, kb.AggregationQueue()).Result()

	// Add queue depths to current stats
	statsMap["submission_queue_depth"] = submissionQueueDepth
	statsMap["finalization_queue_depth"] = finalizationQueueDepth
	statsMap["aggregation_queue_depth"] = aggregationQueueDepth

	// Get participation metrics from state-tracker (namespaced)
	participationKey := fmt.Sprintf("%s:%s:metrics:participation", kb.ProtocolState, kb.DataMarket)
	participationJSON, _ := m.redis.Get(m.ctx, participationKey).Result()
	var participation map[string]interface{}
	if participationJSON != "" {
		json.Unmarshal([]byte(participationJSON), &participation)
	}

	// Get current epoch status from state-tracker (namespaced)
	currentEpochKey := fmt.Sprintf("%s:%s:metrics:current_epoch", kb.ProtocolState, kb.DataMarket)
	currentEpochJSON, _ := m.redis.Get(m.ctx, currentEpochKey).Result()
	var currentEpoch map[string]interface{}
	if currentEpochJSON != "" {
		json.Unmarshal([]byte(currentEpochJSON), &currentEpoch)
	}

	// System Metrics: rates, totals, and current counts
	systemMetrics := make(map[string]interface{})
	if summary != nil {
		// Core system metrics from state-tracker
		systemMetrics["active_validators"] = summary["active_validators"]
		systemMetrics["batch_rate"] = summary["batch_rate"]
		systemMetrics["batches_total"] = summary["batches_total"]
		systemMetrics["epoch_rate"] = summary["epoch_rate"]
		systemMetrics["epochs_total"] = summary["epochs_total"]
		systemMetrics["processed_submissions"] = summary["processed_submissions"]
		systemMetrics["submission_rate"] = summary["submission_rate"]
		systemMetrics["submissions_total"] = summary["submissions_total"]
		systemMetrics["submissions_queue"] = summary["submissions_queue"]
		systemMetrics["measurement_duration"] = summary["measurement_duration"]
		systemMetrics["updated_at"] = summary["updated_at"]

		// VPA metrics (if available from state-tracker)
		if vpaPriority, ok := summary["vpa_priority_assignments_total"]; ok {
			systemMetrics["vpa_priority_assignments_total"] = vpaPriority
		}
		if vpaNoPriority, ok := summary["vpa_no_priority_count"]; ok {
			systemMetrics["vpa_no_priority_count"] = vpaNoPriority
		}
		if vpaSuccess, ok := summary["vpa_submissions_success"]; ok {
			systemMetrics["vpa_submissions_success"] = vpaSuccess
		}
		if vpaFailed, ok := summary["vpa_submissions_failed"]; ok {
			systemMetrics["vpa_submissions_failed"] = vpaFailed
		}
		if vpaSuccessRate, ok := summary["vpa_submission_success_rate"]; ok {
			systemMetrics["vpa_submission_success_rate"] = vpaSuccessRate
		}
		if vpaPriorities24h, ok := summary["vpa_priority_assignments_24h"]; ok {
			systemMetrics["vpa_priority_assignments_24h"] = vpaPriorities24h
		}
		if vpaSubmissions24h, ok := summary["vpa_submissions_24h"]; ok {
			systemMetrics["vpa_submissions_24h"] = vpaSubmissions24h
		}
	}

	// Current Status: real-time status (epoch, phase, queues)
	currentStatus := make(map[string]interface{})
	if currentEpoch != nil {
		currentStatus["current_epoch_id"] = currentEpoch["epoch_id"]
		currentStatus["current_epoch_phase"] = currentEpoch["phase"]
		currentStatus["epoch_time_remaining"] = currentEpoch["time_remaining_seconds"]
		currentStatus["epoch_window_duration"] = currentEpoch["window_duration"]
	}

	// Add queue depths to current status
	currentStatus["submission_queue_depth"] = submissionQueueDepth
	currentStatus["finalization_queue_depth"] = finalizationQueueDepth
	currentStatus["aggregation_queue_depth"] = aggregationQueueDepth

	// Recent Activity: 1m and 5m activity
	recentActivity := make(map[string]interface{})
	if summary != nil {
		recentActivity["submissions_1m"] = summary["submissions_1m"]
		recentActivity["submissions_5m"] = summary["submissions_5m"]
		recentActivity["epochs_1m"] = summary["epochs_1m"]
		recentActivity["epochs_5m"] = summary["epochs_5m"]
		recentActivity["batches_1m"] = summary["batches_1m"]
		recentActivity["batches_5m"] = summary["batches_5m"]
	}

	// Extract VPA metrics from summary for dedicated VPA section
	vpaMetrics := make(map[string]interface{})
	if summary != nil {
		if vpaPriority, ok := summary["vpa_priority_assignments_total"]; ok {
			vpaMetrics["priority_assignments_total"] = vpaPriority
		}
		if vpaNoPriority, ok := summary["vpa_no_priority_count"]; ok {
			vpaMetrics["no_priority_count"] = vpaNoPriority
		}
		if vpaSuccess, ok := summary["vpa_submissions_success"]; ok {
			vpaMetrics["submissions_success"] = vpaSuccess
		}
		if vpaFailed, ok := summary["vpa_submissions_failed"]; ok {
			vpaMetrics["submissions_failed"] = vpaFailed
		}
		if vpaSuccessRate, ok := summary["vpa_submission_success_rate"]; ok {
			vpaMetrics["submission_success_rate"] = vpaSuccessRate
		}
		if vpaPriorities24h, ok := summary["vpa_priority_assignments_24h"]; ok {
			vpaMetrics["priority_assignments_24h"] = vpaPriorities24h
		}
		if vpaSubmissions24h, ok := summary["vpa_submissions_24h"]; ok {
			vpaMetrics["submissions_24h"] = vpaSubmissions24h
		}
	}

	response := DashboardSummary{
		ValidatorID:        getEnv("SEQUENCER_ID", "validator1"),
		SystemMetrics:      systemMetrics,
		ParticipationStats: participation,
		CurrentStatus:      currentStatus,
		RecentActivity:     recentActivity,
		VPAMetrics:         vpaMetrics,
		Timestamp:          time.Now(),
	}

	// Fallback: ensure participation stats exist if not provided by state-tracker
	if response.ParticipationStats == nil {
		response.ParticipationStats = map[string]interface{}{
			"participation_rate":      0,
			"inclusion_rate":          0,
			"level1_batches_24h":      0,
			"level2_inclusions_24h":   0,
			"epochs_participated_24h": 0,
			"epochs_total_24h":        0,
		}
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Hourly statistics
// @Description Get pre-aggregated hourly statistics
// @Tags stats
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Param hours query int false "Number of hours to retrieve (default 24)"
// @Success 200 {object} HourlyStats
// @Router /stats/hourly [get]
func (m *MonitorAPI) HourlyStats(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	hoursParam := c.DefaultQuery("hours", "24")
	hours, _ := strconv.Atoi(hoursParam)
	if hours <= 0 || hours > 48 {
		hours = 24
	}

	hourlyData := make([]map[string]interface{}, 0, hours)
	now := time.Now()

	// Fetch hourly stats for requested hours (namespaced)
	for i := 0; i < hours; i++ {
		hourTime := now.Add(-time.Duration(i) * time.Hour).Truncate(time.Hour)
		hourKey := fmt.Sprintf("%s:%s:stats:hourly:%d", kb.ProtocolState, kb.DataMarket, hourTime.Unix())

		statsJSON, err := m.redis.Get(m.ctx, hourKey).Result()
		if err == redis.Nil {
			continue // No data for this hour
		} else if err != nil {
			log.WithError(err).WithField("key", hourKey).Debug("Failed to fetch hourly stats")
			continue
		}

		var hourStats map[string]interface{}
		if err := json.Unmarshal([]byte(statsJSON), &hourStats); err == nil {
			hourlyData = append(hourlyData, hourStats)
		}
	}

	response := HourlyStats{
		Hours:     hourlyData,
		Timestamp: time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Daily statistics
// @Description Get pre-aggregated 24-hour statistics
// @Tags stats
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {object} DailyStats
// @Router /stats/daily [get]
func (m *MonitorAPI) DailyStats(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	// Read pre-aggregated daily stats (namespaced)
	dailyKey := fmt.Sprintf("%s:%s:stats:daily", kb.ProtocolState, kb.DataMarket)
	statsJSON, err := m.redis.Get(m.ctx, dailyKey).Result()
	if err == redis.Nil {
		c.JSON(http.StatusOK, DailyStats{
			Summary:   map[string]interface{}{"message": "No daily stats available yet"},
			Timestamp: time.Now(),
		})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch daily stats"})
		return
	}

	var dailyStats map[string]interface{}
	if err := json.Unmarshal([]byte(statsJSON), &dailyStats); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse daily stats"})
		return
	}

	// Extract hourly breakdown if present
	var hourlyBreakdown []map[string]interface{}
	if breakdown, ok := dailyStats["hourly_breakdown"].([]interface{}); ok {
		for _, hour := range breakdown {
			if hourMap, ok := hour.(map[string]interface{}); ok {
				hourlyBreakdown = append(hourlyBreakdown, hourMap)
			}
		}
		delete(dailyStats, "hourly_breakdown") // Remove from summary to avoid duplication
	}

	response := DailyStats{
		Summary:         dailyStats,
		HourlyBreakdown: hourlyBreakdown,
		Timestamp:       time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Pipeline overview (legacy support)
// @Description Get pipeline metrics from pre-aggregated data
// @Tags pipeline
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {object} map[string]interface{}
// @Router /pipeline/overview [get]
func (m *MonitorAPI) PipelineOverview(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	// Read from namespaced dashboard:summary
	summaryKey := fmt.Sprintf("%s:%s:dashboard:summary", kb.ProtocolState, kb.DataMarket)
	summaryJSON, _ := m.redis.Get(m.ctx, summaryKey).Result()

	var summary map[string]interface{}
	if summaryJSON != "" {
		json.Unmarshal([]byte(summaryJSON), &summary)
	} else {
		summary = make(map[string]interface{})
	}

	// Add queue depths if needed (real-time check)
	submissionQueueDepth, _ := m.redis.LLen(m.ctx, kb.SubmissionQueue()).Result()
	finalizationQueueDepth, _ := m.redis.LLen(m.ctx, kb.FinalizationQueue()).Result()
	aggregationQueueDepth, _ := m.redis.LLen(m.ctx, kb.AggregationQueue()).Result()

	overview := map[string]interface{}{
		"submission_queue": map[string]interface{}{
			"depth":  submissionQueueDepth,
			"status": getQueueStatus(int(submissionQueueDepth)),
		},
		"finalization_queue": map[string]interface{}{
			"depth":  finalizationQueueDepth,
			"status": getQueueStatus(int(finalizationQueueDepth)),
		},
		"aggregation_queue": map[string]interface{}{
			"depth":  aggregationQueueDepth,
			"status": getQueueStatus(int(aggregationQueueDepth)),
		},
		"metrics":   summary,
		"timestamp": time.Now(),
	}

	c.JSON(http.StatusOK, overview)
}

// @Summary Recent timeline
// @Description Get recent events from timeline sorted sets with enriched metadata
// @Tags timeline
// @Produce json
// @Param type query string false "Event type (submission, validation, epoch, batch)"
// @Param minutes query int false "Minutes to look back (default 5, max 1440 for 24 hours)"
// @Param include_metadata query bool false "Include detailed metadata from Redis (default true)"
// @Success 200 {object} EnhancedTimelineResponse
// @Router /timeline/recent [get]
func (m *MonitorAPI) RecentTimeline(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")
	includeMetadataParam := c.DefaultQuery("include_metadata", "true")
	includeMetadata, _ := strconv.ParseBool(includeMetadataParam)

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	eventType := c.DefaultQuery("type", "submission")
	minutesParam := c.DefaultQuery("minutes", "5")
	minutes, _ := strconv.Atoi(minutesParam)
	// Allow up to 24 hours (1440 minutes) to match data retention period
	if minutes <= 0 || minutes > 1440 {
		minutes = 5
	}

	// Validate event type and map to correct timeline names
	validTypes := map[string]bool{
		"submission": true,
		"validation": true,
		"epoch":      true,
		"batch":      true,
	}
	if !validTypes[eventType] {
		eventType = "submission"
	}

	// Map event types to actual timeline key names
	timelineType := eventType
	switch eventType {
	case "submission":
		timelineType = "submissions"
	case "validation":
		timelineType = "validations"
	case "epoch":
		timelineType = "epochs"
	case "batch":
		timelineType = "batches"
	}

	// Use namespaced timeline key
	timelineKey := fmt.Sprintf("%s:%s:metrics:%s:timeline", kb.ProtocolState, kb.DataMarket, timelineType)
	now := time.Now().Unix()
	start := now - int64(minutes*60)

	// Get recent events from sorted set
	events, err := m.redis.ZRangeByScoreWithScores(m.ctx, timelineKey, &redis.ZRangeBy{
		Min: strconv.FormatInt(start, 10),
		Max: strconv.FormatInt(now, 10),
	}).Result()

	if err != nil {
		log.WithError(err).WithField("timeline_key", timelineKey).Error("Failed to fetch timeline events")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Format enhanced events
	enhancedEvents := make([]EnhancedTimelineEvent, 0, len(events))
	for _, event := range events {
		entityID := event.Member.(string)
		timestamp := int64(event.Score)

		enhancedEvent := EnhancedTimelineEvent{
			EntityID:  entityID,
			Timestamp: timestamp,
			Time:      time.Unix(timestamp, 0).Format(time.RFC3339),
		}

		// Parse entity ID to extract semantic information
		parsedMetadata := parseEntityID(entityID, eventType)
		if parsedMetadata != nil {
			enhancedEvent.Metadata = parsedMetadata
		}

		// If metadata is requested and this is a submission, try to fetch detailed metadata
		if includeMetadata && eventType == "submission" {
			if detailedMetadata, err := m.fetchSubmissionMetadata(kb, entityID); err == nil && detailedMetadata != nil {
				// Merge detailed metadata with parsed metadata, preferring detailed values
				if enhancedEvent.Metadata == nil {
					enhancedEvent.Metadata = detailedMetadata
				} else {
					// Merge fields, with detailed metadata taking precedence
					if detailedMetadata.EpochID != "" {
						enhancedEvent.Metadata.EpochID = detailedMetadata.EpochID
					}
					if detailedMetadata.SlotID != "" {
						enhancedEvent.Metadata.SlotID = detailedMetadata.SlotID
					}
					if detailedMetadata.ProjectID != "" {
						enhancedEvent.Metadata.ProjectID = detailedMetadata.ProjectID
					}
					if detailedMetadata.PeerID != "" {
						enhancedEvent.Metadata.PeerID = detailedMetadata.PeerID
					}
					if detailedMetadata.ValidatorID != "" {
						enhancedEvent.Metadata.ValidatorID = detailedMetadata.ValidatorID
					}
					if detailedMetadata.DataMarket != "" {
						enhancedEvent.Metadata.DataMarket = detailedMetadata.DataMarket
					}
					if detailedMetadata.SubmissionType != "" {
						enhancedEvent.Metadata.SubmissionType = detailedMetadata.SubmissionType
					}
					enhancedEvent.Metadata.IsEnhanced = detailedMetadata.IsEnhanced
				}
			} else if err != nil {
				// Log error but continue without detailed metadata
				log.WithError(err).WithField("entity_id", entityID).Debug("Failed to fetch detailed metadata")
			}
		}

		enhancedEvents = append(enhancedEvents, enhancedEvent)
	}

	response := EnhancedTimelineResponse{
		Type:      eventType,
		Minutes:   minutes,
		Count:     len(enhancedEvents),
		Events:    enhancedEvents,
		Timestamp: time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

// @Summary Health check
// @Description Check if the API and Redis connection are healthy
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func (m *MonitorAPI) Health(c *gin.Context) {
	if err := m.redis.Ping(m.ctx).Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	// Check if state-tracker data is fresh (use default keyBuilder for health check)
	summaryKey := fmt.Sprintf("%s:%s:dashboard:summary", m.keyBuilder.ProtocolState, m.keyBuilder.DataMarket)
	summaryJSON, _ := m.redis.Get(m.ctx, summaryKey).Result()
	dataFresh := summaryJSON != ""

	c.JSON(http.StatusOK, gin.H{
		"status":     "healthy",
		"redis":      "connected",
		"data_fresh": dataFresh,
		"timestamp":  time.Now(),
	})
}

// @Summary Finalized batches
// @Description Get Level 1 (local) or Level 2 (aggregated) finalized batches. Queries last 24 hours of data (matches retention period).
// @Tags batches
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Param level query int false "Batch level (1 or 2, default both)"
// @Param epoch_id query string false "Specific epoch ID"
// @Param limit query int false "Number of batches to retrieve (default 50, max 100)"
// @Success 200 {array} FinalizedBatch
// @Router /batches/finalized [get]
func (m *MonitorAPI) FinalizedBatches(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	levelParam := c.DefaultQuery("level", "0")
	level, _ := strconv.Atoi(levelParam)
	epochID := c.Query("epoch_id")
	limitParam := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitParam)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var batches []FinalizedBatch

	// If specific epoch requested
	if epochID != "" {
		if level == 1 || level == 0 {
			// Get Level 1 batch
			level1Key := kb.MetricsBatchLocal(epochID)
			level1Data, err := m.redis.Get(m.ctx, level1Key).Result()
			if err == nil {
				var batchData map[string]interface{}
				if json.Unmarshal([]byte(level1Data), &batchData) == nil {
					batch := FinalizedBatch{
						EpochID:      epochID,
						Level:        1,
						Type:         "local",
						ProjectCount: int(batchData["project_count"].(float64)),
						Timestamp:    int64(batchData["timestamp"].(float64)),
					}
					if vid, ok := batchData["validator_id"].(string); ok {
						batch.ValidatorID = vid
					}
					if cid, ok := batchData["ipfs_cid"].(string); ok {
						batch.IPFSCid = cid
					}
					// Get epoch state for phase and onchain status
					epochStateKey := kb.EpochState(epochID)
					stateData, _ := m.redis.HGetAll(m.ctx, epochStateKey).Result()
					if phase, ok := stateData["phase"]; ok {
						batch.Phase = phase
					}
					if onchainStatus, ok := stateData["onchain_status"]; ok {
						batch.OnchainStatus = onchainStatus
					}
					// Check for gaps
					windowStatus := stateData["window_status"]
					level1Status := stateData["level1_status"]
					level2Status := stateData["level2_status"]
					if windowStatus == "closed" && level1Status != "completed" && level1Status != "in_progress" {
						batch.HasGap = true
					} else if level1Status == "completed" && level2Status != "completed" && level2Status != "aggregating" && level2Status != "collecting" {
						batch.HasGap = true
					}
					batches = append(batches, batch)
				}
			}
		}

		if level == 2 || level == 0 {
			// Get Level 2 batch
			level2Key := kb.MetricsBatchAggregated(epochID)
			level2Data, err := m.redis.Get(m.ctx, level2Key).Result()
			if err == nil {
				var batchData map[string]interface{}
				if json.Unmarshal([]byte(level2Data), &batchData) == nil {
					// Get validator list
					validatorsKey := kb.MetricsBatchValidators(epochID)
					validatorsJSON, _ := m.redis.Get(m.ctx, validatorsKey).Result()
					var validators []string
					if validatorsJSON != "" {
						json.Unmarshal([]byte(validatorsJSON), &validators)
					}

					batch := FinalizedBatch{
						EpochID:        epochID,
						Level:          2,
						Type:           "aggregated",
						ValidatorIDs:   validators,
						ValidatorCount: len(validators),
						ProjectCount:   int(batchData["project_count"].(float64)),
						Timestamp:      int64(batchData["timestamp"].(float64)),
					}
					if cid, ok := batchData["ipfs_cid"].(string); ok {
						batch.IPFSCid = cid
					}
					if root, ok := batchData["merkle_root"].(string); ok {
						batch.MerkleRoot = root
					}
					// Get epoch state for phase and onchain status
					epochStateKey := kb.EpochState(epochID)
					stateData, _ := m.redis.HGetAll(m.ctx, epochStateKey).Result()
					if phase, ok := stateData["phase"]; ok {
						batch.Phase = phase
					}
					if onchainStatus, ok := stateData["onchain_status"]; ok {
						batch.OnchainStatus = onchainStatus
					}
					// Check for gaps
					level2Status := stateData["level2_status"]
					onchainStatus := stateData["onchain_status"]
					if level2Status == "completed" && onchainStatus != "confirmed" && onchainStatus != "submitted" && onchainStatus != "queued" {
						batch.HasGap = true
					}
					batches = append(batches, batch)
				}
			}
		}
	} else {
		// Get recent batches from timeline
		// Query full 24-hour window (matches data retention period)
		now := time.Now().Unix()
		start := now - 86400 // Last 24 hours (matches pruneOldData retention)

		// Fetch all entries from timeline in the 24-hour window
		// We'll filter and limit in code to ensure we get the requested number of batches
		// Note: Count parameter may limit results, so we fetch without it and filter manually
		entries, _ := m.redis.ZRevRangeByScore(m.ctx, kb.MetricsBatchesTimeline(), &redis.ZRangeBy{
			Min: strconv.FormatInt(start, 10),
			Max: "+inf",
			// Don't use Count here - fetch all entries and filter in code
		}).Result()

		for _, entry := range entries {
			parts := strings.Split(entry, ":")
			if len(parts) < 2 {
				continue
			}
			batchType := parts[0]
			batchEpoch := parts[1]

			if batchType == "local" && (level == 1 || level == 0) {
				level1Key := kb.MetricsBatchLocal(batchEpoch)
				level1Data, err := m.redis.Get(m.ctx, level1Key).Result()
				if err == nil {
					var batchData map[string]interface{}
					if json.Unmarshal([]byte(level1Data), &batchData) == nil {
						batch := FinalizedBatch{
							EpochID:      batchEpoch,
							Level:        1,
							Type:         "local",
							ProjectCount: int(batchData["project_count"].(float64)),
							Timestamp:    int64(batchData["timestamp"].(float64)),
						}
						if vid, ok := batchData["validator_id"].(string); ok {
							batch.ValidatorID = vid
						}
						if cid, ok := batchData["ipfs_cid"].(string); ok {
							batch.IPFSCid = cid
						}
						// Get epoch state for phase and onchain status
						epochStateKey := kb.EpochState(batchEpoch)
						stateData, _ := m.redis.HGetAll(m.ctx, epochStateKey).Result()
						if phase, ok := stateData["phase"]; ok {
							batch.Phase = phase
						}
						if onchainStatus, ok := stateData["onchain_status"]; ok {
							batch.OnchainStatus = onchainStatus
						}
						batches = append(batches, batch)
					}
				}
			} else if batchType == "aggregated" && (level == 2 || level == 0) {
				level2Key := kb.MetricsBatchAggregated(batchEpoch)
				level2Data, err := m.redis.Get(m.ctx, level2Key).Result()
				if err == nil {
					var batchData map[string]interface{}
					if json.Unmarshal([]byte(level2Data), &batchData) == nil {
						validatorsKey := kb.MetricsBatchValidators(batchEpoch)
						validatorsJSON, _ := m.redis.Get(m.ctx, validatorsKey).Result()
						var validators []string
						if validatorsJSON != "" {
							json.Unmarshal([]byte(validatorsJSON), &validators)
						}

						batch := FinalizedBatch{
							EpochID:        batchEpoch,
							Level:          2,
							Type:           "aggregated",
							ValidatorIDs:   validators,
							ValidatorCount: len(validators),
							ProjectCount:   int(batchData["project_count"].(float64)),
							Timestamp:      int64(batchData["timestamp"].(float64)),
						}
						if cid, ok := batchData["ipfs_cid"].(string); ok {
							batch.IPFSCid = cid
						}
						if root, ok := batchData["merkle_root"].(string); ok {
							batch.MerkleRoot = root
						}
						// Get epoch state for phase and onchain status
						epochStateKey := kb.EpochState(batchEpoch)
						stateData, _ := m.redis.HGetAll(m.ctx, epochStateKey).Result()
						if phase, ok := stateData["phase"]; ok {
							batch.Phase = phase
						}
						if onchainStatus, ok := stateData["onchain_status"]; ok {
							batch.OnchainStatus = onchainStatus
						}
						batches = append(batches, batch)
					}
				}
			}

			// Stop if we've reached the requested limit
			if len(batches) >= limit {
				break
			}
		}
	}

	c.JSON(http.StatusOK, batches)
}

// @Summary Aggregation results
// @Description Get network-wide aggregated batches with validator contributions
// @Tags aggregation
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Param limit query int false "Number of results (default 20)"
// @Success 200 {array} FinalizedBatch
// @Router /aggregation/results [get]
func (m *MonitorAPI) AggregationResults(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	limitParam := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitParam)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Get recent Level 2 aggregated batches from timeline
	now := time.Now().Unix()
	start := now - 86400 // Last 24 hours

	entries, _ := m.redis.ZRevRangeByScore(m.ctx, kb.MetricsBatchesTimeline(), &redis.ZRangeBy{
		Min:   strconv.FormatInt(start, 10),
		Max:   "+inf",
		Count: int64(limit * 2), // Get more to filter only aggregated
	}).Result()

	var batches []FinalizedBatch
	for _, entry := range entries {
		if !strings.HasPrefix(entry, "aggregated:") {
			continue
		}

		epochID := utils.FormatEpochID(strings.TrimPrefix(entry, "aggregated:"))

		// Try to get batch data - first with formatted epoch ID, then with original
		level2Key := kb.MetricsBatchAggregated(epochID)
		level2Data, err := m.redis.Get(m.ctx, level2Key).Result()
		if err != nil {
			// Try with the original epoch ID from timeline (might have scientific notation)
			originalEpochID := strings.TrimPrefix(entry, "aggregated:")
			fallbackKey := kb.MetricsBatchAggregated(originalEpochID)
			level2Data, err = m.redis.Get(m.ctx, fallbackKey).Result()
			if err != nil {
				continue
			}
		}

		var batchData map[string]interface{}
		if json.Unmarshal([]byte(level2Data), &batchData) != nil {
			continue
		}

		// Get validator list - try both formatted and original epoch ID
		validatorsKey := kb.MetricsBatchValidators(epochID)
		validatorsJSON, _ := m.redis.Get(m.ctx, validatorsKey).Result()
		if validatorsJSON == "" {
			// Try with original epoch ID
			originalEpochID := strings.TrimPrefix(entry, "aggregated:")
			fallbackValidatorsKey := kb.MetricsBatchValidators(originalEpochID)
			validatorsJSON, _ = m.redis.Get(m.ctx, fallbackValidatorsKey).Result()
		}
		var validators []string
		if validatorsJSON != "" {
			json.Unmarshal([]byte(validatorsJSON), &validators)
		}

		batch := FinalizedBatch{
			EpochID:        epochID,
			Level:          2,
			Type:           "aggregated",
			ValidatorIDs:   validators,
			ValidatorCount: len(validators),
			ProjectCount:   int(batchData["project_count"].(float64)),
			Timestamp:      int64(batchData["timestamp"].(float64)),
		}

		batches = append(batches, batch)

		if len(batches) >= limit {
			break
		}
	}

	c.JSON(http.StatusOK, batches)
}

// @Summary Epochs timeline
// @Description Get epoch progression with phases and batch status
// @Tags epochs
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Param limit query int false "Number of epochs (default 50)"
// @Success 200 {array} EpochInfo
// @Router /epochs/timeline [get]
func (m *MonitorAPI) EpochsTimeline(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	limitParam := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitParam)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	// Get recent epochs from timeline (entries are "open:{id}" or "close:{id}")
	entries, _ := m.redis.ZRevRange(m.ctx, kb.MetricsEpochsTimeline(), 0, int64(limit*2)).Result()

	epochMap := make(map[string]*EpochInfo)
	var epochOrder []string

	for _, entry := range entries {
		parts := strings.Split(entry, ":")
		if len(parts) < 2 {
			continue
		}
		epochID := parts[1]

		if _, exists := epochMap[epochID]; !exists {
			epochMap[epochID] = &EpochInfo{
				EpochID: epochID,
			}
			epochOrder = append(epochOrder, epochID)
		}
	}

	// Get detailed info for each epoch
	var epochs []EpochInfo
	for _, epochID := range epochOrder {
		if len(epochs) >= limit {
			break
		}

		epochInfo := epochMap[epochID]

		// Get epoch info hash (authoritative source)
		infoKey := kb.MetricsEpochInfo(epochID)
		infoData, err := m.redis.HGetAll(m.ctx, infoKey).Result()
		if err == nil {
			// Get status from Redis hash (authoritative)
			if status, ok := infoData["status"]; ok {
				epochInfo.Status = status
			}
			if startStr, ok := infoData["start"]; ok {
				if startInt, err := strconv.ParseInt(startStr, 10, 64); err == nil {
					epochInfo.StartTime = startInt
				}
			}
			if durStr, ok := infoData["duration"]; ok {
				if durInt, err := strconv.Atoi(durStr); err == nil {
					epochInfo.Duration = durInt
				}
			}
			if market, ok := infoData["data_market"]; ok {
				epochInfo.DataMarket = market
			}
		}

		// Check for Level 1 batch
		level1Key := kb.MetricsBatchLocal(epochID)
		level1Exists, _ := m.redis.Exists(m.ctx, level1Key).Result()
		epochInfo.Level1Batch = level1Exists > 0

		// Check for Level 2 batch
		level2Key := kb.MetricsBatchAggregated(epochID)
		level2Exists, _ := m.redis.Exists(m.ctx, level2Key).Result()
		epochInfo.Level2Batch = level2Exists > 0

		// Get phase from epoch state hash (authoritative source)
		epochStateKey := kb.EpochState(epochID)
		stateData, _ := m.redis.HGetAll(m.ctx, epochStateKey).Result()
		if phase, ok := stateData["phase"]; ok && phase != "" {
			epochInfo.Phase = phase
		} else {
			// Fallback: Determine phase from batch status
			if epochInfo.Status == "open" {
				epochInfo.Phase = "submission"
			} else if epochInfo.Level2Batch {
				epochInfo.Phase = "complete"
			} else if epochInfo.Level1Batch {
				epochInfo.Phase = "aggregation"
			} else {
				epochInfo.Phase = "finalization"
			}
		}

		epochs = append(epochs, *epochInfo)
	}

	// Sort epochs by epoch ID (numeric, descending - most recent first)
	sort.Slice(epochs, func(i, j int) bool {
		epochI, errI := strconv.ParseUint(epochs[i].EpochID, 10, 64)
		epochJ, errJ := strconv.ParseUint(epochs[j].EpochID, 10, 64)
		if errI != nil || errJ != nil {
			// If parsing fails, fall back to string comparison
			return epochs[i].EpochID > epochs[j].EpochID
		}
		return epochI > epochJ
	})

	c.JSON(http.StatusOK, epochs)
}

// @Summary Epoch status
// @Description Get complete epoch state with all phase information
// @Tags epochs
// @Produce json
// @Param epochId path string true "Epoch ID"
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {object} map[string]interface{}
// @Router /epochs/{epochId}/status [get]
func (m *MonitorAPI) EpochStatus(c *gin.Context) {
	epochID := c.Param("epochId")
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	// Get epoch state hash
	epochStateKey := kb.EpochState(epochID)
	stateData, err := m.redis.HGetAll(m.ctx, epochStateKey).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Epoch state not found"})
		return
	}

	// Convert string values to appropriate types
	result := make(map[string]interface{})
	for k, v := range stateData {
		// Try to parse as int64 for timestamps
		if strings.HasSuffix(k, "_at") || strings.HasSuffix(k, "_timestamp") || k == "last_updated" {
			if intVal, err := strconv.ParseInt(v, 10, 64); err == nil {
				result[k] = intVal
			} else {
				result[k] = v
			}
		} else if k == "submissions_count" || k == "priority" || k == "onchain_block_number" {
			if intVal, err := strconv.Atoi(v); err == nil {
				result[k] = intVal
			} else {
				result[k] = v
			}
		} else if k == "vpa_submission_attempted" {
			result[k] = v == "true" || v == "1"
		} else {
			result[k] = v
		}
	}

	// Also include batch status
	level1Key := kb.MetricsBatchLocal(epochID)
	level1Exists, _ := m.redis.Exists(m.ctx, level1Key).Result()
	result["level1_batch_exists"] = level1Exists > 0

	level2Key := kb.MetricsBatchAggregated(epochID)
	level2Exists, _ := m.redis.Exists(m.ctx, level2Key).Result()
	result["level2_batch_exists"] = level2Exists > 0

	c.JSON(http.StatusOK, gin.H{
		"epoch_id":  epochID,
		"state":     result,
		"timestamp": time.Now(),
	})
}

// @Summary Active epochs
// @Description Get epochs currently in progress (window open, level 1/2 in progress). Queries epoch state directly for accuracy.
// @Tags epochs
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {array} EpochInfo
// @Router /epochs/active [get]
func (m *MonitorAPI) ActiveEpochs(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	// Query recent epochs from timeline (more reliable than ActiveEpochs SET)
	// Get last 100 epochs from timeline to check for active ones
	entries, _ := m.redis.ZRevRange(m.ctx, kb.MetricsEpochsTimeline(), 0, 99).Result()

	epochMap := make(map[string]bool)
	for _, entry := range entries {
		parts := strings.Split(entry, ":")
		if len(parts) >= 2 {
			epochMap[parts[1]] = true
		}
	}

	var activeEpochs []EpochInfo
	for epochID := range epochMap {
		// Get epoch state hash (authoritative source)
		epochStateKey := kb.EpochState(epochID)
		stateData, err := m.redis.HGetAll(m.ctx, epochStateKey).Result()

		// If state hash doesn't exist, check if epoch has batches (might be in progress)
		if err != nil || len(stateData) == 0 {
			// Check if epoch has Level 1 or Level 2 batches (might be actively processing)
			level1Key := kb.MetricsBatchLocal(epochID)
			level2Key := kb.MetricsBatchAggregated(epochID)
			level1Exists, _ := m.redis.Exists(m.ctx, level1Key).Result()
			level2Exists, _ := m.redis.Exists(m.ctx, level2Key).Result()

			// If epoch has batches but no state hash, it might be actively processing
			// Include it but mark as needing state initialization
			if level1Exists > 0 || level2Exists > 0 {
				epochInfo := EpochInfo{
					EpochID: epochID,
					Phase:   "unknown", // State hash missing, phase unknown
					Status:  "unknown",
				}
				epochInfo.Level1Batch = level1Exists > 0
				epochInfo.Level2Batch = level2Exists > 0
				activeEpochs = append(activeEpochs, epochInfo)
			}
			continue
		}

		// Check if epoch is actively processing
		windowStatus := stateData["window_status"]
		level1Status := stateData["level1_status"]
		level2Status := stateData["level2_status"]
		phase := stateData["phase"]
		onchainStatus := stateData["onchain_status"]

		isActive := false
		if windowStatus == "open" {
			isActive = true
		} else if level1Status == "in_progress" || level1Status == "pending" {
			// Include pending level1 (window closed but finalization not started yet)
			isActive = true
		} else if level2Status == "collecting" || level2Status == "aggregating" || level2Status == "pending" {
			// Include pending level2 (level1 completed but level2 not started yet)
			isActive = true
		} else if onchainStatus == "queued" || onchainStatus == "submitted" {
			// Include epochs with on-chain submissions in progress
			isActive = true
		}

		if !isActive {
			continue
		}

		epochInfo := EpochInfo{
			EpochID: epochID,
			Phase:   phase,
			Status:  windowStatus,
		}

		// Get epoch info for additional details
		infoKey := kb.MetricsEpochInfo(epochID)
		infoData, _ := m.redis.HGetAll(m.ctx, infoKey).Result()
		if startStr, ok := infoData["start"]; ok {
			if startInt, err := strconv.ParseInt(startStr, 10, 64); err == nil {
				epochInfo.StartTime = startInt
			}
		}
		if durStr, ok := infoData["duration"]; ok {
			if durInt, err := strconv.Atoi(durStr); err == nil {
				epochInfo.Duration = durInt
			}
		}

		// Check batch status
		level1Key := kb.MetricsBatchLocal(epochID)
		level1Exists, _ := m.redis.Exists(m.ctx, level1Key).Result()
		epochInfo.Level1Batch = level1Exists > 0

		level2Key := kb.MetricsBatchAggregated(epochID)
		level2Exists, _ := m.redis.Exists(m.ctx, level2Key).Result()
		epochInfo.Level2Batch = level2Exists > 0

		activeEpochs = append(activeEpochs, epochInfo)
	}

	// Sort by epoch ID descending
	sort.Slice(activeEpochs, func(i, j int) bool {
		epochI, errI := strconv.ParseUint(activeEpochs[i].EpochID, 10, 64)
		epochJ, errJ := strconv.ParseUint(activeEpochs[j].EpochID, 10, 64)
		if errI != nil || errJ != nil {
			return activeEpochs[i].EpochID > activeEpochs[j].EpochID
		}
		return epochI > epochJ
	})

	c.JSON(http.StatusOK, activeEpochs)
}

// @Summary Epoch gaps
// @Description Identify epochs that should have finalizations but don't
// @Tags epochs
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Param window_minutes query int false "Window minutes to check for gaps (default 5)"
// @Success 200 {array} map[string]interface{}
// @Router /epochs/gaps [get]
func (m *MonitorAPI) EpochGaps(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")
	windowMinutesParam := c.DefaultQuery("window_minutes", "5")
	windowMinutes, _ := strconv.Atoi(windowMinutesParam)
	// Allow up to 24 hours (1440 minutes) to match data retention period
	if windowMinutes <= 0 || windowMinutes > 1440 {
		windowMinutes = 5
	}

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	// Get recent epochs from timeline
	now := time.Now().Unix()
	cutoffTime := now - int64(windowMinutes*60)
	entries, _ := m.redis.ZRangeByScore(m.ctx, kb.MetricsEpochsTimeline(), &redis.ZRangeBy{
		Min: strconv.FormatInt(cutoffTime, 10),
		Max: "+inf",
	}).Result()

	epochSet := make(map[string]bool)
	for _, entry := range entries {
		parts := strings.Split(entry, ":")
		if len(parts) >= 2 {
			epochSet[parts[1]] = true
		}
	}

	var gaps []map[string]interface{}
	for epochID := range epochSet {
		epochStateKey := kb.EpochState(epochID)
		stateData, _ := m.redis.HGetAll(m.ctx, epochStateKey).Result()

		windowStatus := stateData["window_status"]
		level1Status := stateData["level1_status"]
		level2Status := stateData["level2_status"]
		onchainStatus := stateData["onchain_status"]

		// Check for gaps
		gapType := ""
		diagnostic := ""

		if windowStatus == "closed" && level1Status != "completed" && level1Status != "in_progress" {
			gapType = "missing_level1"
			diagnostic = "Window closed but Level 1 finalization not started"
		} else if level1Status == "completed" && level2Status != "completed" && level2Status != "aggregating" && level2Status != "collecting" {
			gapType = "missing_level2"
			diagnostic = "Level 1 completed but Level 2 aggregation not started"
		} else if level2Status == "completed" && onchainStatus != "confirmed" && onchainStatus != "submitted" && onchainStatus != "queued" {
			gapType = "missing_onchain"
			diagnostic = "Level 2 completed but on-chain submission not attempted"
		}

		if gapType != "" {
			gaps = append(gaps, map[string]interface{}{
				"epoch_id":   epochID,
				"gap_type":   gapType,
				"diagnostic": diagnostic,
				"state":      stateData,
			})
		}
	}

	// Sort by epoch ID descending
	sort.Slice(gaps, func(i, j int) bool {
		epochI := gaps[i]["epoch_id"].(string)
		epochJ := gaps[j]["epoch_id"].(string)
		epochIVal, errI := strconv.ParseUint(epochI, 10, 64)
		epochJVal, errJ := strconv.ParseUint(epochJ, 10, 64)
		if errI != nil || errJ != nil {
			return epochI > epochJ
		}
		return epochIVal > epochJVal
	})

	c.JSON(http.StatusOK, gin.H{
		"gaps":      gaps,
		"count":     len(gaps),
		"timestamp": time.Now(),
	})
}

// @Summary Epoch submissions
// @Description Get all submissions for a specific epoch with detailed metadata (slot ID, peer ID, project ID, CID)
// @Tags epochs
// @Produce json
// @Param epochId path string true "Epoch ID"
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {object} map[string]interface{}
// @Router /epochs/{epochId}/submissions [get]
func (m *MonitorAPI) EpochSubmissions(c *gin.Context) {
	epochID := c.Param("epochId")
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	// Get submissions from timeline (query recent entries to avoid scanning all)
	// Get last 1000 entries which should cover recent epochs
	timelineKey := kb.MetricsSubmissionsTimeline()
	now := time.Now().Unix()
	// Look back 24 hours for submissions (epochs are typically processed within hours)
	startTime := now - (24 * 3600)

	entries, err := m.redis.ZRangeByScoreWithScores(m.ctx, timelineKey, &redis.ZRangeBy{
		Min: strconv.FormatInt(startTime, 10),
		Max: "+inf",
	}).Result()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"epoch_id":    epochID,
			"submissions": []SubmissionInfo{},
			"count":       0,
			"timestamp":   time.Now(),
		})
		return
	}

	var submissions []SubmissionInfo
	for _, entry := range entries {
		entityID := entry.Member.(string)
		timestamp := int64(entry.Score)

		// Parse entity ID format: received:{epoch}:{slot}:{project}:{timestamp}:{peer}
		// Or legacy format: {epoch}-{project}-{timestamp}
		parts := strings.Split(entityID, ":")

		var submissionEpochID string
		if len(parts) >= 2 && parts[0] == "received" {
			// Enhanced format: received:{epoch}:{slot}:{project}:{timestamp}:{peer}
			submissionEpochID = parts[1]
		} else {
			// Legacy format: try to extract epoch from first part
			legacyParts := strings.Split(entityID, "-")
			if len(legacyParts) >= 1 {
				submissionEpochID = legacyParts[0]
			}
		}

		// Only include submissions for this epoch
		if submissionEpochID != epochID {
			continue
		}

		// Fetch detailed metadata
		metadataKey := kb.MetricsSubmissionsMetadata(entityID)
		metadataJSON, err := m.redis.Get(m.ctx, metadataKey).Result()

		submission := SubmissionInfo{
			EntityID:  entityID,
			EpochID:   epochID,
			Timestamp: timestamp,
			Time:      time.Unix(timestamp, 0).Format(time.RFC3339),
		}

		if err == nil && metadataJSON != "" {
			// Parse metadata JSON
			var metadata map[string]interface{}
			if json.Unmarshal([]byte(metadataJSON), &metadata) == nil {
				if slotID, ok := metadata["slot_id"].(string); ok {
					submission.SlotID = slotID
				} else if slotID, ok := metadata["slot_id"].(float64); ok {
					submission.SlotID = strconv.FormatFloat(slotID, 'f', 0, 64)
				}
				if projectID, ok := metadata["project_id"].(string); ok {
					submission.ProjectID = projectID
				}
				if cid, ok := metadata["cid"].(string); ok {
					submission.SnapshotCID = cid
				} else if cid, ok := metadata["snapshot_cid"].(string); ok {
					submission.SnapshotCID = cid
				}
				if peerID, ok := metadata["peer_id"].(string); ok {
					submission.PeerID = peerID
				}
				if validatorID, ok := metadata["validator_id"].(string); ok {
					submission.ValidatorID = validatorID
				}
			}
		}

		// If metadata not found, try to parse from entity ID
		if submission.SlotID == "" && len(parts) >= 3 && parts[0] == "received" {
			submission.SlotID = parts[2]
		}
		if submission.ProjectID == "" && len(parts) >= 4 && parts[0] == "received" {
			submission.ProjectID = parts[3]
		}
		if submission.PeerID == "" && len(parts) >= 6 && parts[0] == "received" {
			submission.PeerID = parts[5]
		}

		submissions = append(submissions, submission)
	}

	// Sort by timestamp descending (most recent first)
	sort.Slice(submissions, func(i, j int) bool {
		return submissions[i].Timestamp > submissions[j].Timestamp
	})

	c.JSON(http.StatusOK, gin.H{
		"epoch_id":    epochID,
		"submissions": submissions,
		"count":       len(submissions),
		"timestamp":   time.Now(),
	})
}

// @Summary Queue status
// @Description Get real-time queue depths and processing rates
// @Tags queues
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {array} QueueStatus
// @Router /queues/status [get]
func (m *MonitorAPI) QueuesStatus(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to default
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	queues := []QueueStatus{
		{
			QueueName: "submission_queue",
			Depth:     0,
			Status:    "empty",
		},
		{
			QueueName: "finalization_queue",
			Depth:     0,
			Status:    "empty",
		},
		{
			QueueName: "aggregation_queue",
			Depth:     0,
			Status:    "empty",
		},
	}

	// Get actual queue depths
	submissionDepth, _ := m.redis.LLen(m.ctx, kb.SubmissionQueue()).Result()
	queues[0].Depth = submissionDepth
	queues[0].Status = getQueueStatus(int(submissionDepth))

	finalizationDepth, _ := m.redis.LLen(m.ctx, kb.FinalizationQueue()).Result()
	queues[1].Depth = finalizationDepth
	queues[1].Status = getQueueStatus(int(finalizationDepth))

	// Check stream lag for aggregation (actual working system)
	streamKey := kb.AggregationStream()
	streamLag, err := getStreamLag(m.redis, m.ctx, streamKey, "aggregator-group")
	if err != nil {
		// Fallback to list depth if stream check fails
		aggregationDepth, _ := m.redis.LLen(m.ctx, kb.AggregationQueue()).Result()
		queues[2].Depth = aggregationDepth
		queues[2].Status = getQueueStatus(int(aggregationDepth))
	} else {
		queues[2].Depth = streamLag
		queues[2].Status = getQueueStatus(int(streamLag))
	}

	c.JSON(http.StatusOK, queues)
}

// Helper function to get stream consumer group lag
func getStreamLag(redisClient *redis.Client, ctx context.Context, streamKey, groupName string) (int64, error) {
	// Get stream info to find last delivered ID
	info, err := redisClient.XInfoGroups(ctx, streamKey).Result()
	if err != nil {
		return 0, err
	}

	// Find the specific group
	var lastDeliveredID string
	for _, group := range info {
		if group.Name == groupName {
			lastDeliveredID = group.LastDeliveredID
			break
		}
	}

	if lastDeliveredID == "" {
		return 0, fmt.Errorf("group %s not found", groupName)
	}

	// Get stream info to find total entries
	streamInfo, err := redisClient.XInfoStream(ctx, streamKey).Result()
	if err != nil {
		return 0, err
	}

	// Simple approximation: if last delivered is "0-0", lag is total entries
	// This is a simplification - a more accurate approach would be to calculate
	// the difference between stream last ID and last delivered ID
	if lastDeliveredID == "0-0" {
		return streamInfo.Length, nil
	}

	// For most cases, if the system is working, lag should be small
	// We'll use a heuristic based on the Redis XINFO GROUPS data
	return 0, nil // Assume no lag if we can process messages
}

// Helper function to determine queue status
func getQueueStatus(depth int) string {
	switch {
	case depth == 0:
		return "empty"
	case depth < 100:
		return "healthy"
	case depth < 500:
		return "moderate"
	case depth < 1000:
		return "high"
	default:
		return "critical"
	}
}

// parseEntityID extracts semantic information from entity ID formats
// Supports both enhanced format (epoch:slot:project:peer) and legacy formats
func parseEntityID(entityID string, eventType string) *TimelineEventMetadata {
	metadata := &TimelineEventMetadata{
		EventType: eventType,
	}

	// Enhanced format pattern: epoch:slot:project:peer_id
	enhancedPattern := regexp.MustCompile(`^(\d+):(\d+):([a-fA-F0-9]+):([a-fA-F0-9]+)$`)
	if matches := enhancedPattern.FindStringSubmatch(entityID); len(matches) == 5 {
		metadata.IsEnhanced = true
		metadata.EpochID = matches[1]
		metadata.SlotID = matches[2]
		metadata.ProjectID = matches[3]
		metadata.PeerID = matches[4]
		metadata.SubmissionType = "snapshot"
		return metadata
	}

	// Legacy format patterns
	switch eventType {
	case "submission":
		// Pattern: epoch:project_id (legacy)
		legacyPattern := regexp.MustCompile(`^(\d+):([a-fA-F0-9]+)$`)
		if matches := legacyPattern.FindStringSubmatch(entityID); len(matches) == 3 {
			metadata.EpochID = matches[1]
			metadata.ProjectID = matches[2]
			metadata.SubmissionType = "snapshot"
		}
	case "validation":
		// Pattern: epoch:project_id:validator_id (legacy)
		validationPattern := regexp.MustCompile(`^(\d+):([a-fA-F0-9]+):([a-zA-Z0-9]+)$`)
		if matches := validationPattern.FindStringSubmatch(entityID); len(matches) == 4 {
			metadata.EpochID = matches[1]
			metadata.ProjectID = matches[2]
			metadata.ValidatorID = matches[3]
			metadata.SubmissionType = "validation"
		}
	case "batch":
		// Pattern: type:epoch_id (local:123, aggregated:123)
		batchPattern := regexp.MustCompile(`^(local|aggregated):(.+)$`)
		if matches := batchPattern.FindStringSubmatch(entityID); len(matches) == 3 {
			metadata.EpochID = matches[2]
		}
	case "epoch":
		// Pattern: action:epoch_id (open:123, close:123)
		epochPattern := regexp.MustCompile(`^(open|close):(.+)$`)
		if matches := epochPattern.FindStringSubmatch(entityID); len(matches) == 3 {
			metadata.EpochID = matches[2]
		}
	}

	return metadata
}

// fetchSubmissionMetadata retrieves detailed submission metadata from Redis
func (m *MonitorAPI) fetchSubmissionMetadata(kb *keys.KeyBuilder, entityID string) (*TimelineEventMetadata, error) {
	// Get metadata key for this entity
	metadataKey := kb.MetricsSubmissionsMetadata(entityID)

	// Try to get metadata from Redis
	metadataJSON, err := m.redis.Get(m.ctx, metadataKey).Result()
	if err != nil {
		if err == redis.Nil {
			// No metadata found, return nil without error
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch submission metadata: %w", err)
	}

	// Parse metadata JSON
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse submission metadata: %w", err)
	}

	// Convert to TimelineEventMetadata
	result := &TimelineEventMetadata{
		IsEnhanced: true,
	}

	// Extract known fields with type safety
	if epochID, ok := metadata["epoch_id"].(string); ok {
		result.EpochID = epochID
	}
	if slotID, ok := metadata["slot_id"].(string); ok {
		result.SlotID = slotID
	}
	if projectID, ok := metadata["project_id"].(string); ok {
		result.ProjectID = projectID
	}
	if peerID, ok := metadata["peer_id"].(string); ok {
		result.PeerID = peerID
	}
	if validatorID, ok := metadata["validator_id"].(string); ok {
		result.ValidatorID = validatorID
	}
	if dataMarket, ok := metadata["data_market"].(string); ok {
		result.DataMarket = dataMarket
	}
	if submissionType, ok := metadata["submission_type"].(string); ok {
		result.SubmissionType = submissionType
	}
	if eventType, ok := metadata["event_type"].(string); ok {
		result.EventType = eventType
	}

	return result, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// @Summary Get VPA status for specific epoch
// @Description Get priority assignment and submission status for a specific epoch. Returns combined priority and submission information including priority details and submission details if available.
// @Tags vpa
// @Produce json
// @Param epochID path string true "Epoch ID"
// @Param protocol query string false "Protocol state identifier (defaults to configured protocol)"
// @Param market query string false "Data market address (defaults to configured market)"
// @Success 200 {object} VPAEpochStatusResponse "VPA epoch status with priority and submission information"
// @Failure 404 {object} map[string]interface{} "Epoch not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /vpa/epoch/{epochID} [get]
func (m *MonitorAPI) VPAEpochStatus(c *gin.Context) {
	epochID := c.Param("epochID")
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to NEW_DATA_MARKET_CONTRACT (VPA is only for new markets)
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	} else {
		// Default to NEW_DATA_MARKET_CONTRACT for VPA endpoints
		if m.newDataMarket != "" {
			protocolToUse := m.keyBuilder.ProtocolState
			if m.newProtocolState != "" {
				protocolToUse = m.newProtocolState
			}
			kb = keys.NewKeyBuilder(protocolToUse, m.newDataMarket)
		}
	}

	// Get combined epoch status
	statusKey := kb.VPAEpochStatus(epochID)
	statusJSON, err := m.redis.Get(m.ctx, statusKey).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     "epoch not found",
			"epoch_id":  epochID,
			"timestamp": time.Now(),
		})
		return
	}

	var status map[string]interface{}
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to parse status data",
		})
		return
	}

	// Also try to get individual priority and submission records for completeness
	priorityKey := kb.VPAPriorityAssignment(epochID)
	priorityJSON, _ := m.redis.Get(m.ctx, priorityKey).Result()
	if priorityJSON != "" {
		var priorityData map[string]interface{}
		if err := json.Unmarshal([]byte(priorityJSON), &priorityData); err == nil {
			status["priority_details"] = priorityData
		}
	}

	submissionKey := kb.VPASubmissionResult(epochID)
	submissionJSON, _ := m.redis.Get(m.ctx, submissionKey).Result()
	if submissionJSON != "" {
		var submissionData map[string]interface{}
		if err := json.Unmarshal([]byte(submissionJSON), &submissionData); err == nil {
			status["submission_details"] = submissionData
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"epoch_id":  epochID,
		"status":    status,
		"timestamp": time.Now(),
	})
}

// @Summary Get VPA timeline
// @Description Get priority assignment and submission timeline. Returns recent priority assignments and/or submissions ordered by timestamp (most recent first).
// @Tags vpa
// @Produce json
// @Param protocol query string false "Protocol state identifier (defaults to configured protocol)"
// @Param market query string false "Data market address (defaults to configured market)"
// @Param type query string false "Timeline type: 'priority', 'submission', or 'both' (default: 'both')"
// @Param limit query int false "Number of entries to retrieve per timeline type (default: 50, max: 1000)"
// @Success 200 {object} VPATimelineResponse "VPA timeline with priority and/or submission entries"
// @Router /vpa/timeline [get]
func (m *MonitorAPI) VPATimeline(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")
	timelineType := c.DefaultQuery("type", "both")
	limitStr := c.DefaultQuery("limit", "50")

	// Use specified protocol/market or fall back to NEW_DATA_MARKET_CONTRACT (VPA is only for new markets)
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	} else {
		// Default to NEW_DATA_MARKET_CONTRACT for VPA endpoints
		if m.newDataMarket != "" {
			protocolToUse := m.keyBuilder.ProtocolState
			if m.newProtocolState != "" {
				protocolToUse = m.newProtocolState
			}
			kb = keys.NewKeyBuilder(protocolToUse, m.newDataMarket)
		}
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 1000 {
		limit = 50
	}

	result := make(map[string]interface{})

	// Get priority timeline
	if timelineType == "both" || timelineType == "priority" {
		priorityTimelineKey := kb.VPAPriorityTimeline()
		priorityEntries, err := m.redis.ZRevRangeWithScores(m.ctx, priorityTimelineKey, 0, int64(limit-1)).Result()
		if err == nil {
			priorityTimeline := make([]map[string]interface{}, 0)
			for _, entry := range priorityEntries {
				// Parse member: "{epochID}:{priority}:{status}"
				parts := strings.Split(entry.Member.(string), ":")
				if len(parts) >= 3 {
					priorityTimeline = append(priorityTimeline, map[string]interface{}{
						"epoch_id":  parts[0],
						"priority":  parts[1],
						"status":    strings.Join(parts[2:], ":"),
						"timestamp": int64(entry.Score),
					})
				}
			}
			// Sort by epoch_id (descending - most recent first)
			sort.Slice(priorityTimeline, func(i, j int) bool {
				epochI, _ := strconv.Atoi(priorityTimeline[i]["epoch_id"].(string))
				epochJ, _ := strconv.Atoi(priorityTimeline[j]["epoch_id"].(string))
				return epochI > epochJ
			})
			result["priority_timeline"] = priorityTimeline
		}
	}

	// Get submission timeline
	if timelineType == "both" || timelineType == "submission" {
		submissionTimelineKey := kb.VPASubmissionTimeline()
		submissionEntries, err := m.redis.ZRevRangeWithScores(m.ctx, submissionTimelineKey, 0, int64(limit-1)).Result()
		if err == nil {
			submissionTimeline := make([]map[string]interface{}, 0)
			for _, entry := range submissionEntries {
				// Parse member: "{epochID}:{priority}:{status}"
				parts := strings.Split(entry.Member.(string), ":")
				if len(parts) >= 3 {
					submissionTimeline = append(submissionTimeline, map[string]interface{}{
						"epoch_id":  parts[0],
						"priority":  parts[1],
						"status":    strings.Join(parts[2:], ":"),
						"timestamp": int64(entry.Score),
					})
				}
			}
			// Sort by epoch_id (descending - most recent first)
			sort.Slice(submissionTimeline, func(i, j int) bool {
				epochI, _ := strconv.Atoi(submissionTimeline[i]["epoch_id"].(string))
				epochJ, _ := strconv.Atoi(submissionTimeline[j]["epoch_id"].(string))
				return epochI > epochJ
			})
			result["submission_timeline"] = submissionTimeline
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"timeline":  result,
		"timestamp": time.Now(),
	})
}

// @Summary Get VPA statistics
// @Description Get aggregated VPA statistics including total priority assignments, submission counts (success/failed), success rates, and 24-hour activity counts.
// @Tags vpa
// @Produce json
// @Param protocol query string false "Protocol state identifier (defaults to configured protocol)"
// @Param market query string false "Data market address (defaults to configured market)"
// @Success 200 {object} VPAStatsResponse "Aggregated VPA statistics"
// @Router /vpa/stats [get]
func (m *MonitorAPI) VPAStats(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to NEW_DATA_MARKET_CONTRACT (VPA is only for new markets)
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	} else {
		// Default to NEW_DATA_MARKET_CONTRACT for VPA endpoints
		if m.newDataMarket != "" {
			protocolToUse := m.keyBuilder.ProtocolState
			if m.newProtocolState != "" {
				protocolToUse = m.newProtocolState
			}
			kb = keys.NewKeyBuilder(protocolToUse, m.newDataMarket)
		}
	}

	// Get stats from Redis hash
	statsKey := kb.VPAStats()
	stats, err := m.redis.HGetAll(m.ctx, statsKey).Result()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"stats":     make(map[string]interface{}),
			"timestamp": time.Now(),
		})
		return
	}

	// Parse stats and calculate rates
	result := make(map[string]interface{})
	for k, v := range stats {
		if val, err := strconv.ParseInt(v, 10, 64); err == nil {
			result[k] = val
		} else {
			result[k] = v
		}
	}

	// Calculate success rate if we have success/failed counts
	if success, ok := result["total_submissions_success"].(int64); ok {
		if failed, ok := result["total_submissions_failed"].(int64); ok {
			total := success + failed
			if total > 0 {
				result["submission_success_rate"] = float64(success) / float64(total) * 100
			}
		}
	}

	// Count recent activity (last 24 hours)
	nowTS := time.Now().Unix()
	twentyFourHoursAgo := nowTS - (24 * 3600)

	priorityTimelineKey := kb.VPAPriorityTimeline()
	recentPriorities, _ := m.redis.ZCount(m.ctx, priorityTimelineKey,
		strconv.FormatInt(twentyFourHoursAgo, 10),
		strconv.FormatInt(nowTS, 10)).Result()
	result["priority_assignments_24h"] = recentPriorities

	submissionTimelineKey := kb.VPASubmissionTimeline()
	recentSubmissions, _ := m.redis.ZCount(m.ctx, submissionTimelineKey,
		strconv.FormatInt(twentyFourHoursAgo, 10),
		strconv.FormatInt(nowTS, 10)).Result()
	result["submissions_24h"] = recentSubmissions

	c.JSON(http.StatusOK, gin.H{
		"stats":     result,
		"timestamp": time.Now(),
	})
}

// @Summary Get epoch lifecycle tracking
// @Description Track complete lifecycle of an epoch from release to submission, showing all stages and timing
// @Tags vpa
// @Produce json
// @Param epochID path string true "Epoch ID to track"
// @Param protocol query string false "Protocol state identifier (defaults to configured protocol)"
// @Param market query string false "Data market address (defaults to configured market)"
// @Success 200 {object} map[string]interface{} "Epoch lifecycle with all stages and timing"
// @Router /vpa/epoch/{epochID}/lifecycle [get]
func (m *MonitorAPI) EpochLifecycle(c *gin.Context) {
	epochID := c.Param("epochID")
	protocol := c.Query("protocol")
	market := c.Query("market")

	// Use specified protocol/market or fall back to NEW_DATA_MARKET_CONTRACT
	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	} else {
		if m.newDataMarket != "" {
			protocolToUse := m.keyBuilder.ProtocolState
			if m.newProtocolState != "" {
				protocolToUse = m.newProtocolState
			}
			kb = keys.NewKeyBuilder(protocolToUse, m.newDataMarket)
		}
	}

	lifecycle := make(map[string]interface{})
	lifecycle["epoch_id"] = epochID
	lifecycle["stages"] = make(map[string]interface{})

	// Check epoch release (from epoch info)
	epochInfoKey := fmt.Sprintf("epoch:%s:%s:info", kb.DataMarket, epochID)
	info, err := m.redis.HGetAll(m.ctx, epochInfoKey).Result()
	if err == nil && len(info) > 0 {
		if releasedAt, ok := info["released_at"]; ok {
			ts, _ := strconv.ParseInt(releasedAt, 10, 64)
			lifecycle["stages"].(map[string]interface{})["released"] = map[string]interface{}{
				"timestamp": ts,
				"status":    "completed",
				"details":   info,
			}
		}
	} else {
		lifecycle["stages"].(map[string]interface{})["released"] = map[string]interface{}{
			"status": "missing",
		}
	}

	// Check finalized batch
	finalizedKey := kb.FinalizedBatch(epochID)
	finalizedData, err := m.redis.Get(m.ctx, finalizedKey).Result()
	if err == nil {
		var batch map[string]interface{}
		if json.Unmarshal([]byte(finalizedData), &batch) == nil {
			if timestamp, ok := batch["timestamp"].(float64); ok {
				lifecycle["stages"].(map[string]interface{})["finalized"] = map[string]interface{}{
					"timestamp": int64(timestamp),
					"status":    "completed",
					"details":   batch,
				}
			}
		}
	} else {
		lifecycle["stages"].(map[string]interface{})["finalized"] = map[string]interface{}{
			"status": "missing",
		}
	}

	// Check aggregated batch
	aggregatedKey := kb.BatchAggregated(epochID)
	aggregatedData, err := m.redis.Get(m.ctx, aggregatedKey).Result()
	if err == nil {
		var batch map[string]interface{}
		if json.Unmarshal([]byte(aggregatedData), &batch) == nil {
			if timestamp, ok := batch["timestamp"].(float64); ok {
				lifecycle["stages"].(map[string]interface{})["aggregated"] = map[string]interface{}{
					"timestamp": int64(timestamp),
					"status":    "completed",
					"details":   batch,
				}
			}
		}
	} else {
		lifecycle["stages"].(map[string]interface{})["aggregated"] = map[string]interface{}{
			"status": "missing",
		}
	}

	// Check VPA priority
	priorityKey := kb.VPAPriorityAssignment(epochID)
	priorityData, err := m.redis.Get(m.ctx, priorityKey).Result()
	if err == nil {
		var priority map[string]interface{}
		if json.Unmarshal([]byte(priorityData), &priority) == nil {
			if timestamp, ok := priority["timestamp"].(float64); ok {
				lifecycle["stages"].(map[string]interface{})["vpa_priority"] = map[string]interface{}{
					"timestamp": int64(timestamp),
					"status":    priority["status"],
					"details":   priority,
				}
			}
		}
	} else {
		lifecycle["stages"].(map[string]interface{})["vpa_priority"] = map[string]interface{}{
			"status": "missing",
		}
	}

	// Check VPA submission
	submissionKey := kb.VPASubmissionResult(epochID)
	submissionData, err := m.redis.Get(m.ctx, submissionKey).Result()
	if err == nil {
		var submission map[string]interface{}
		if json.Unmarshal([]byte(submissionData), &submission) == nil {
			if timestamp, ok := submission["timestamp"].(float64); ok {
				lifecycle["stages"].(map[string]interface{})["vpa_submission"] = map[string]interface{}{
					"timestamp": int64(timestamp),
					"status":    submission["success"],
					"details":   submission,
				}
			}
		}
	} else {
		lifecycle["stages"].(map[string]interface{})["vpa_submission"] = map[string]interface{}{
			"status": "missing",
		}
	}

	// Calculate timing between stages
	stages := lifecycle["stages"].(map[string]interface{})
	if released, ok := stages["released"].(map[string]interface{}); ok {
		if releasedTS, ok := released["timestamp"].(int64); ok {
			if finalized, ok := stages["finalized"].(map[string]interface{}); ok {
				if finalizedTS, ok := finalized["timestamp"].(int64); ok {
					lifecycle["timing"] = map[string]interface{}{
						"released_to_finalized_seconds": finalizedTS - releasedTS,
					}
				}
			}
			if aggregated, ok := stages["aggregated"].(map[string]interface{}); ok {
				if aggregatedTS, ok := aggregated["timestamp"].(int64); ok {
					if timing, ok := lifecycle["timing"].(map[string]interface{}); ok {
						timing["released_to_aggregated_seconds"] = aggregatedTS - releasedTS
					} else {
						lifecycle["timing"] = map[string]interface{}{
							"released_to_aggregated_seconds": aggregatedTS - releasedTS,
						}
					}
				}
			}
			if submission, ok := stages["vpa_submission"].(map[string]interface{}); ok {
				if submissionTS, ok := submission["timestamp"].(int64); ok {
					if timing, ok := lifecycle["timing"].(map[string]interface{}); ok {
						timing["released_to_submission_seconds"] = submissionTS - releasedTS
					} else {
						lifecycle["timing"] = map[string]interface{}{
							"released_to_submission_seconds": submissionTS - releasedTS,
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, lifecycle)
}

// @Summary Get flagged peers
// @Description Get list of all flagged peer IDs (from Redis cache, synced from on-chain)
// @Tags spam
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {object} map[string]interface{} "List of flagged peers with metadata"
// @Router /spam/flagged/peers [get]
func (m *MonitorAPI) FlaggedPeers(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	// Get flagged peers set
	flaggedSetKey := fmt.Sprintf("flagged_peers:%s", kb.DataMarket)
	peerIDs, err := m.redis.SMembers(m.ctx, flaggedSetKey).Result()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"flagged_peers": []string{},
			"count":         0,
			"timestamp":     time.Now(),
		})
		return
	}

	// Get metadata for each flagged peer
	peers := make([]map[string]interface{}, 0)
	for _, peerID := range peerIDs {
		peerKey := fmt.Sprintf("%s:%s:spam:consensus_flagged:peer:%s", kb.ProtocolState, kb.DataMarket, peerID)
		peerData, err := m.redis.Get(m.ctx, peerKey).Result()
		if err == nil {
			var metadata map[string]interface{}
			if json.Unmarshal([]byte(peerData), &metadata) == nil {
				metadata["peer_id"] = peerID
				peers = append(peers, metadata)
			}
		} else {
			// Peer is in set but no metadata - still include it
			peers = append(peers, map[string]interface{}{
				"peer_id": peerID,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"flagged_peers": peers,
		"count":         len(peers),
		"timestamp":     time.Now(),
	})
}

// @Summary Get flagged snapshotters
// @Description Get list of all flagged snapshotter addresses (from Redis cache, synced from on-chain)
// @Tags spam
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {object} map[string]interface{} "List of flagged snapshotters with metadata"
// @Router /spam/flagged/snapshotters [get]
func (m *MonitorAPI) FlaggedSnapshotters(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	// Get flagged snapshotters set
	flaggedSetKey := fmt.Sprintf("flagged_snapshotters:%s", kb.DataMarket)
	snapshotterAddrs, err := m.redis.SMembers(m.ctx, flaggedSetKey).Result()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"flagged_snapshotters": []string{},
			"count":                0,
			"timestamp":            time.Now(),
		})
		return
	}

	// Get metadata for each flagged snapshotter
	snapshotters := make([]map[string]interface{}, 0)
	for _, addr := range snapshotterAddrs {
		snapshotterKey := fmt.Sprintf("%s:%s:spam:consensus_flagged:snapshotter:%s", kb.ProtocolState, kb.DataMarket, addr)
		snapshotterData, err := m.redis.Get(m.ctx, snapshotterKey).Result()
		if err == nil {
			var metadata map[string]interface{}
			if json.Unmarshal([]byte(snapshotterData), &metadata) == nil {
				metadata["snapshotter_addr"] = addr
				snapshotters = append(snapshotters, metadata)
			}
		} else {
			// Snapshotter is in set but no metadata - still include it
			snapshotters = append(snapshotters, map[string]interface{}{
				"snapshotter_addr": addr,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"flagged_snapshotters": snapshotters,
		"count":                len(snapshotters),
		"timestamp":            time.Now(),
	})
}

// @Summary Get spam tracking info for a peer
// @Description Get validation failures, submission counts, and aggregation info for a specific peer
// @Tags spam
// @Produce json
// @Param peerID path string true "Peer ID (libp2p)"
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Param epochID query int false "Specific epoch ID (optional, defaults to current epoch)"
// @Success 200 {object} map[string]interface{} "Spam tracking information for peer"
// @Router /spam/peer/{peerID} [get]
func (m *MonitorAPI) PeerSpamInfo(c *gin.Context) {
	peerID := c.Param("peerID")
	protocol := c.Query("protocol")
	market := c.Query("market")
	epochIDStr := c.Query("epochID")

	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	result := make(map[string]interface{})
	result["peer_id"] = peerID

	// If epochID specified, get per-epoch tracking
	if epochIDStr != "" {
		epochID, err := strconv.ParseUint(epochIDStr, 10, 64)
		if err == nil {
			// Get validation failures for this epoch
			failureKey := fmt.Sprintf("%s:%s:spam:validation_failures:peer:%s:%d", kb.ProtocolState, kb.DataMarket, peerID, epochID)
			failures, _ := m.redis.Get(m.ctx, failureKey).Int64()

			// Get submission count for this epoch
			submissionKey := fmt.Sprintf("%s:%s:spam:submissions:peer:%s:%d", kb.ProtocolState, kb.DataMarket, peerID, epochID)
			submissions, _ := m.redis.Get(m.ctx, submissionKey).Int64()

			// Get peer-snapshotter associations
			mapKey := fmt.Sprintf("%s:%s:spam:peer_snapshotter_map:%s:%d", kb.ProtocolState, kb.DataMarket, peerID, epochID)
			snapshotterAddrs, _ := m.redis.SMembers(m.ctx, mapKey).Result()

			result["epoch_id"] = epochID
			result["validation_failures"] = failures
			result["submission_count"] = submissions
			result["snapshotter_addresses"] = snapshotterAddrs
		}
	}

	// Get aggregation window info (if available)
	// Window ID = round up to next multiple of 10 (end epoch of the 10-epoch range)
	// Formula: ((epochID + 9) / 10) * 10
	// Epochs 1-10 → Window 10, Epochs 11-20 → Window 20, etc.
	const windowSize = 10
	if epochIDStr != "" {
		epochID, err := strconv.ParseUint(epochIDStr, 10, 64)
		if err == nil {
			windowID := ((epochID + uint64(windowSize) - 1) / uint64(windowSize)) * uint64(windowSize)
			aggKey := fmt.Sprintf("%s:%s:spam:reports:peer:%s:window:%d", kb.ProtocolState, kb.DataMarket, peerID, windowID)
			aggData, err := m.redis.Get(m.ctx, aggKey).Result()
			if err == nil {
				var aggregated map[string]interface{}
				if json.Unmarshal([]byte(aggData), &aggregated) == nil {
					result["aggregation_window"] = windowID
					result["aggregated_reports"] = aggregated
				}
			}
		}
	}

	// Check if peer is flagged
	flaggedKey := fmt.Sprintf("%s:%s:spam:consensus_flagged:peer:%s", kb.ProtocolState, kb.DataMarket, peerID)
	flagged, _ := m.redis.Exists(m.ctx, flaggedKey).Result()
	result["is_flagged"] = flagged > 0

	if flagged > 0 {
		flaggedData, err := m.redis.Get(m.ctx, flaggedKey).Result()
		if err == nil {
			var flagMetadata map[string]interface{}
			if json.Unmarshal([]byte(flaggedData), &flagMetadata) == nil {
				result["flag_metadata"] = flagMetadata
			}
		}
	}

	result["timestamp"] = time.Now()
	c.JSON(http.StatusOK, result)
}

// @Summary List all aggregation windows with reports
// @Description Get list of all windows that have spam reports (for discovery/indexing)
// @Tags spam
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {object} map[string]interface{} "List of window IDs with basic info"
// @Router /spam/windows [get]
func (m *MonitorAPI) SpamWindows(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	windowsSetKey := fmt.Sprintf("%s:%s:spam:reports:windows", kb.ProtocolState, kb.DataMarket)
	windowIDs, err := m.redis.SMembers(m.ctx, windowsSetKey).Result()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"windows": []interface{}{},
			"count":   0,
		})
		return
	}

	// Sort window IDs numerically and get details for each
	windows := make([]map[string]interface{}, 0)
	for _, windowIDStr := range windowIDs {
		windowID, err := strconv.Atoi(windowIDStr)
		if err != nil {
			continue
		}

		// Get peer count in this window
		windowPeersKey := fmt.Sprintf("%s:%s:spam:reports:window:%d:peers", kb.ProtocolState, kb.DataMarket, windowID)
		peerCount, _ := m.redis.SCard(m.ctx, windowPeersKey).Result()

		// Calculate epoch range
		firstEpoch := windowID - 9
		lastEpoch := windowID

		windows = append(windows, map[string]interface{}{
			"window_id":   windowID,
			"epoch_range": fmt.Sprintf("%d-%d", firstEpoch, lastEpoch),
			"first_epoch": firstEpoch,
			"last_epoch":  lastEpoch,
			"peer_count":  peerCount,
		})
	}

	// Sort by window ID descending (latest first)
	sort.Slice(windows, func(i, j int) bool {
		return windows[i]["window_id"].(int) > windows[j]["window_id"].(int)
	})

	c.JSON(http.StatusOK, gin.H{
		"windows":   windows,
		"count":     len(windows),
		"timestamp": time.Now(),
	})
}

// @Summary Get details for a specific aggregation window
// @Description Get all peers and their aggregated reports for a specific window
// @Tags spam
// @Produce json
// @Param windowID path int true "Window ID (end epoch of 10-epoch range)"
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {object} map[string]interface{} "Window details with all peers and reports"
// @Router /spam/windows/{windowID} [get]
func (m *MonitorAPI) SpamWindowDetails(c *gin.Context) {
	windowIDStr := c.Param("windowID")
	protocol := c.Query("protocol")
	market := c.Query("market")

	windowID, err := strconv.Atoi(windowIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid window ID"})
		return
	}

	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	// Calculate epoch range
	firstEpoch := windowID - 9
	lastEpoch := windowID

	// Get all peers in this window
	windowPeersKey := fmt.Sprintf("%s:%s:spam:reports:window:%d:peers", kb.ProtocolState, kb.DataMarket, windowID)
	peerIDs, err := m.redis.SMembers(m.ctx, windowPeersKey).Result()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"window_id":   windowID,
			"epoch_range": fmt.Sprintf("%d-%d", firstEpoch, lastEpoch),
			"peers":       []interface{}{},
			"peer_count":  0,
		})
		return
	}

	// Get aggregated reports for each peer
	peers := make([]map[string]interface{}, 0)
	for _, peerID := range peerIDs {
		aggKey := fmt.Sprintf("%s:%s:spam:reports:peer:%s:window:%d", kb.ProtocolState, kb.DataMarket, peerID, windowID)
		aggData, err := m.redis.Get(m.ctx, aggKey).Result()
		if err != nil {
			continue
		}

		var aggregated map[string]interface{}
		if json.Unmarshal([]byte(aggData), &aggregated) == nil {
			reportCount := 0
			if reports, ok := aggregated["reports"].([]interface{}); ok {
				reportCount = len(reports)
			}
			peers = append(peers, map[string]interface{}{
				"peer_id":         peerID,
				"validator_count": aggregated["validator_count"],
				"first_epoch":     aggregated["first_epoch"],
				"last_epoch":      aggregated["last_epoch"],
				"report_count":    reportCount,
				"reports":         aggregated["reports"],
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"window_id":   windowID,
		"epoch_range": fmt.Sprintf("%d-%d", firstEpoch, lastEpoch),
		"first_epoch": firstEpoch,
		"last_epoch":  lastEpoch,
		"peers":       peers,
		"peer_count":  len(peers),
		"timestamp":   time.Now(),
	})
}

// @Summary Get spam protection statistics
// @Description Get aggregated spam protection statistics including flagged counts, reports, and enforcement metrics
// @Tags spam
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {object} map[string]interface{} "Spam protection statistics"
// @Router /spam/stats [get]
func (m *MonitorAPI) SpamStats(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")

	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	stats := make(map[string]interface{})

	// Count flagged peers
	flaggedPeersKey := fmt.Sprintf("flagged_peers:%s", kb.DataMarket)
	flaggedPeerCount, _ := m.redis.SCard(m.ctx, flaggedPeersKey).Result()
	stats["flagged_peers_count"] = flaggedPeerCount

	// Count flagged snapshotters
	flaggedSnapshottersKey := fmt.Sprintf("flagged_snapshotters:%s", kb.DataMarket)
	flaggedSnapshotterCount, _ := m.redis.SCard(m.ctx, flaggedSnapshottersKey).Result()
	stats["flagged_snapshotters_count"] = flaggedSnapshotterCount

	// Count active validators (for consensus calculation)
	activeValidatorsKey := fmt.Sprintf("%s:%s:active:validators", kb.ProtocolState, kb.DataMarket)
	activeValidatorCount, _ := m.redis.SCard(m.ctx, activeValidatorsKey).Result()
	stats["active_validators_count"] = activeValidatorCount

	// Note: Per-epoch tracking keys are ephemeral (24h TTL) and would require scanning
	// For production, these should be aggregated by state-tracker or queried via Prometheus metrics

	stats["timestamp"] = time.Now()
	c.JSON(http.StatusOK, stats)
}

// @Summary List epochs with tracking data
// @Description Get list of epochs that have spam tracking data (peers tracked)
// @Tags spam
// @Produce json
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Param limit query int false "Maximum number of epochs to return (default: 100)"
// @Success 200 {object} map[string]interface{} "List of epochs with tracking data"
// @Router /spam/epochs [get]
func (m *MonitorAPI) SpamEpochs(c *gin.Context) {
	protocol := c.Query("protocol")
	market := c.Query("market")
	limitStr := c.DefaultQuery("limit", "100")

	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	// Check recent epochs (up to limit) for tracking data
	// We check epochs in reverse order (newest first)
	epochs := make([]map[string]interface{}, 0)

	// Start from a high epoch number and work backwards
	// In practice, you'd get current epoch from event monitor, but for discovery we check recent epochs
	currentEpoch := uint64(100000) // Start high, will be adjusted based on actual data

	checked := 0
	for epoch := currentEpoch; epoch > 0 && checked < limit*2; epoch-- {
		epochPeersKey := fmt.Sprintf("%s:%s:spam:epoch:%d:peers", kb.ProtocolState, kb.DataMarket, epoch)
		exists, err := m.redis.Exists(m.ctx, epochPeersKey).Result()
		if err != nil {
			continue
		}
		if exists > 0 {
			peerCount, _ := m.redis.SCard(m.ctx, epochPeersKey).Result()
			epochs = append(epochs, map[string]interface{}{
				"epoch_id":   epoch,
				"peer_count": peerCount,
			})
			checked++
			if len(epochs) >= limit {
				break
			}
		}
		checked++
	}

	c.JSON(http.StatusOK, gin.H{
		"epochs":    epochs,
		"count":     len(epochs),
		"timestamp": time.Now(),
	})
}

// @Summary Get epoch-by-epoch tracking for a peer
// @Description Get tracking data (submissions, validation failures) for a peer across multiple epochs
// @Tags spam
// @Produce json
// @Param peerID path string true "Peer ID (libp2p)"
// @Param startEpoch query int false "Start epoch (default: current epoch - 10)"
// @Param endEpoch query int false "End epoch (default: current epoch)"
// @Param protocol query string false "Protocol state identifier"
// @Param market query string false "Data market address"
// @Success 200 {object} map[string]interface{} "Epoch-by-epoch tracking data"
// @Router /spam/peer/{peerID}/epochs [get]
func (m *MonitorAPI) PeerSpamEpochs(c *gin.Context) {
	peerID := c.Param("peerID")
	protocol := c.Query("protocol")
	market := c.Query("market")
	startEpochStr := c.Query("startEpoch")
	endEpochStr := c.Query("endEpoch")

	kb := m.keyBuilder
	if protocol != "" || market != "" {
		if protocol == "" {
			protocol = m.keyBuilder.ProtocolState
		}
		if market == "" {
			market = m.keyBuilder.DataMarket
		}
		kb = keys.NewKeyBuilder(protocol, market)
	}

	// Parse epoch range
	var startEpoch, endEpoch uint64
	if endEpochStr != "" {
		endEpoch, _ = strconv.ParseUint(endEpochStr, 10, 64)
	} else {
		// Default to checking recent epochs
		endEpoch = 100000 // High number, will be adjusted
	}
	if startEpochStr != "" {
		startEpoch, _ = strconv.ParseUint(startEpochStr, 10, 64)
	} else {
		// Default: last 10 epochs
		if endEpoch > 10 {
			startEpoch = endEpoch - 10
		} else {
			startEpoch = 1
		}
	}

	// Limit range to prevent excessive queries
	if endEpoch-startEpoch > 100 {
		endEpoch = startEpoch + 100
	}

	epochData := make([]map[string]interface{}, 0)
	for epoch := startEpoch; epoch <= endEpoch; epoch++ {
		// Get submission count
		submissionKey := fmt.Sprintf("%s:%s:spam:submissions:peer:%s:%d", kb.ProtocolState, kb.DataMarket, peerID, epoch)
		submissions, _ := m.redis.Get(m.ctx, submissionKey).Int64()

		// Get validation failure count
		failureKey := fmt.Sprintf("%s:%s:spam:validation_failures:peer:%s:%d", kb.ProtocolState, kb.DataMarket, peerID, epoch)
		failures, _ := m.redis.Get(m.ctx, failureKey).Int64()

		// Only include epochs with data
		if submissions > 0 || failures > 0 {
			// Get snapshotter addresses
			mapKey := fmt.Sprintf("%s:%s:spam:peer_snapshotter_map:%s:%d", kb.ProtocolState, kb.DataMarket, peerID, epoch)
			snapshotterAddrs, _ := m.redis.SMembers(m.ctx, mapKey).Result()

			epochData = append(epochData, map[string]interface{}{
				"epoch_id":              epoch,
				"submission_count":      submissions,
				"validation_failures":   failures,
				"snapshotter_addresses": snapshotterAddrs,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"peer_id":     peerID,
		"epochs":      epochData,
		"count":       len(epochData),
		"start_epoch": startEpoch,
		"end_epoch":   endEpoch,
		"timestamp":   time.Now(),
	})
}

func main() {
	// Configure logger
	log.SetFormatter(&logrus.JSONFormatter{})
	if os.Getenv("LOG_LEVEL") == "debug" {
		log.SetLevel(logrus.DebugLevel)
	} else {
		log.SetLevel(logrus.InfoLevel)
	}

	// Get configuration
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	protocol := getEnv("PROTOCOL_STATE_CONTRACT", "")

	// Parse DATA_MARKET_ADDRESSES (could be comma-separated or JSON array)
	marketsEnv := getEnv("DATA_MARKET_ADDRESSES", "")

	// Extract first market address (handle comma-separated or JSON array)
	market := marketsEnv
	if strings.Contains(marketsEnv, ",") {
		market = strings.Split(marketsEnv, ",")[0]
	} else if strings.HasPrefix(marketsEnv, "[") {
		// JSON array - extract first address
		marketsEnv = strings.Trim(marketsEnv, "[]")
		marketsEnv = strings.ReplaceAll(marketsEnv, "\"", "")
		if strings.Contains(marketsEnv, ",") {
			market = strings.Split(marketsEnv, ",")[0]
		}
	}
	market = strings.TrimSpace(market)

	// Validate required configuration
	if protocol == "" {
		log.Fatal("PROTOCOL_STATE_CONTRACT is required")
	}
	if market == "" {
		log.Fatal("DATA_MARKET_ADDRESSES is required")
	}

	port := getEnv("MONITOR_API_PORT", "8080")

	// Connect to Redis
	ctx := context.Background()
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.WithFields(logrus.Fields{
		"redis":    fmt.Sprintf("%s:%s", redisHost, redisPort),
		"protocol": protocol,
		"market":   market,
	}).Info("Monitor API connected to Redis")

	// Create API instance
	api := NewMonitorAPI(redisClient, protocol, market)

	// Setup routes
	router := gin.Default()

	v1 := router.Group("/api/v1")
	{
		// Health check
		v1.GET("/health", api.Health)

		// Dashboard and stats endpoints
		v1.GET("/dashboard/summary", api.DashboardSummary)
		v1.GET("/stats/hourly", api.HourlyStats)
		v1.GET("/stats/daily", api.DailyStats)

		// P0/P1 Endpoints - Core visibility
		v1.GET("/batches/finalized", api.FinalizedBatches)
		v1.GET("/aggregation/results", api.AggregationResults)
		v1.GET("/epochs/timeline", api.EpochsTimeline)
		v1.GET("/epochs/:epochId/status", api.EpochStatus)
		v1.GET("/epochs/:epochId/submissions", api.EpochSubmissions)
		v1.GET("/epochs/active", api.ActiveEpochs)
		v1.GET("/epochs/gaps", api.EpochGaps)
		v1.GET("/queues/status", api.QueuesStatus)

		// Timeline events
		v1.GET("/timeline/recent", api.RecentTimeline)

		// Legacy compatibility endpoint
		v1.GET("/pipeline/overview", api.PipelineOverview)

		// VPA endpoints
		v1.GET("/vpa/epoch/:epochID", api.VPAEpochStatus)
		v1.GET("/vpa/epoch/:epochID/lifecycle", api.EpochLifecycle)
		v1.GET("/vpa/timeline", api.VPATimeline)
		v1.GET("/vpa/stats", api.VPAStats)

		// Spam Protection endpoints
		v1.GET("/spam/flagged/peers", api.FlaggedPeers)
		v1.GET("/spam/flagged/snapshotters", api.FlaggedSnapshotters)
		v1.GET("/spam/peer/:peerID", api.PeerSpamInfo)
		v1.GET("/spam/peer/:peerID/epochs", api.PeerSpamEpochs)
		v1.GET("/spam/windows", api.SpamWindows)
		v1.GET("/spam/windows/:windowID", api.SpamWindowDetails)
		v1.GET("/spam/epochs", api.SpamEpochs)
		v1.GET("/spam/stats", api.SpamStats)
	}

	// Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	log.WithFields(logrus.Fields{
		"port":    port,
		"swagger": fmt.Sprintf("http://localhost:%s/swagger/index.html", port),
	}).Info("🚀 Monitor API (Direct Redis) starting")

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
