package spam

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	redislib "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// SpamReportWindowManager manages collection windows for spam reports per epoch
// Follows the same timer pattern as WindowManager for submission collection
type SpamReportWindowManager struct {
	activeWindows            map[string]*ReportWindow // key: dataMarket:epochID
	mu                       sync.RWMutex
	redisClient              *redis.Client
	keyBuilder               *redislib.KeyBuilder
	collectionWindowDuration time.Duration // level1_delay + 10s
	aggregator               *SpamAggregator
	ctx                      context.Context
}

// ReportWindow represents an active report collection window for an epoch
type ReportWindow struct {
	EpochID           uint64
	DataMarketAddress string
	StartTime         time.Time
	Timer             *time.Timer
	Done              chan struct{}
}

// NewSpamReportWindowManager creates a new SpamReportWindowManager
func NewSpamReportWindowManager(ctx context.Context, redisClient *redis.Client, keyBuilder *redislib.KeyBuilder, collectionWindowDuration time.Duration, aggregator *SpamAggregator) *SpamReportWindowManager {
	return &SpamReportWindowManager{
		activeWindows:            make(map[string]*ReportWindow),
		redisClient:              redisClient,
		keyBuilder:               keyBuilder,
		collectionWindowDuration: collectionWindowDuration,
		aggregator:               aggregator,
		ctx:                      ctx,
	}
}

// StartReportCollectionWindow starts a collection window timer for an epoch
// When the timer fires, all pending reports for that epoch will be collected and sent
func (m *SpamReportWindowManager) StartReportCollectionWindow(ctx context.Context, dataMarket string, epochID uint64, releaseTime time.Time) error {
	key := fmt.Sprintf("%s:%d", dataMarket, epochID)

	// Check if window already exists
	m.mu.RLock()
	if _, exists := m.activeWindows[key]; exists {
		m.mu.RUnlock()
		log.Debugf("Report collection window already active for epoch %d in market %s", epochID, dataMarket)
		return nil // Not an error, just skip
	}
	m.mu.RUnlock()

	// Calculate delay
	delay := m.collectionWindowDuration - time.Since(releaseTime)
	if delay <= 0 {
		// Old epoch, send immediately without batching
		log.Debugf("Epoch %d release time is past collection window, sending reports immediately", epochID)
		return m.sendReportsImmediately(ctx, dataMarket, epochID)
	}

	// Create window
	window := &ReportWindow{
		EpochID:           epochID,
		DataMarketAddress: dataMarket,
		StartTime:         time.Now(),
		Done:              make(chan struct{}),
	}

	// Start window timer
	window.Timer = time.AfterFunc(delay, func() {
		m.closeReportWindow(dataMarket, epochID)
	})

	// Add to active windows
	m.mu.Lock()
	m.activeWindows[key] = window
	activeCount := len(m.activeWindows)
	m.mu.Unlock()

	log.WithFields(log.Fields{
		"epoch_id":     epochID,
		"data_market":  dataMarket,
		"delay":        delay,
		"active_count": activeCount,
	}).Infof("⏰ Started spam report collection window for epoch %d (will send reports after %v)", epochID, delay)

	return nil
}

// closeReportWindow is called when the collection window timer fires
func (m *SpamReportWindowManager) closeReportWindow(dataMarket string, epochID uint64) {
	key := fmt.Sprintf("%s:%d", dataMarket, epochID)

	m.mu.Lock()
	window, exists := m.activeWindows[key]
	if !exists {
		m.mu.Unlock()
		return
	}

	// Remove from active windows
	delete(m.activeWindows, key)
	activeCount := len(m.activeWindows)
	m.mu.Unlock()

	// Close the done channel
	if window.Done != nil {
		close(window.Done)
	}

	log.WithFields(log.Fields{
		"epoch_id":     epochID,
		"data_market":  dataMarket,
		"active_count": activeCount,
	}).Infof("⏱️ Spam report collection window closed for epoch %d, collecting and sending reports", epochID)

	// Collect and send all pending reports
	reports, err := m.collectPendingReports(dataMarket, epochID)
	if err != nil {
		log.Errorf("Failed to collect pending reports for epoch %d: %v", epochID, err)
		return
	}

	if len(reports) > 0 {
		if err := m.batchSendReports(reports); err != nil {
			log.Errorf("Failed to send batched reports for epoch %d: %v", epochID, err)
		} else {
			log.WithFields(log.Fields{
				"epoch_id":     epochID,
				"report_count": len(reports),
			}).Infof("✅ Sent %d batched spam reports for epoch %d", len(reports), epochID)
		}
	} else {
		log.Debugf("No pending reports to send for epoch %d", epochID)
	}

	// Clean up Redis key
	m.cleanupPendingReportsKey(dataMarket, epochID)
}

// collectPendingReports collects all pending reports for an epoch from Redis
// Uses deterministic Redis LIST collection (like submission collection)
func (m *SpamReportWindowManager) collectPendingReports(dataMarket string, epochID uint64) ([]*SpamReport, error) {
	// Use deterministic Redis key
	pendingKey := m.getPendingReportsKey(dataMarket, epochID)

	// Get all reports from LIST (deterministic, no SCAN)
	reportData, err := m.redisClient.LRange(m.ctx, pendingKey, 0, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return []*SpamReport{}, nil
		}
		return nil, fmt.Errorf("failed to get pending reports from Redis: %w", err)
	}

	reports := make([]*SpamReport, 0, len(reportData))
	for _, data := range reportData {
		var report SpamReport
		if err := json.Unmarshal([]byte(data), &report); err != nil {
			log.Warnf("Failed to unmarshal pending report: %v", err)
			continue
		}
		reports = append(reports, &report)
	}

	return reports, nil
}

// batchSendReports sends all reports together via Redis queue and direct aggregator injection
func (m *SpamReportWindowManager) batchSendReports(reports []*SpamReport) error {
	broadcastQueue := m.keyBuilder.OutgoingSpamReports()
	sentCount := 0

	for _, report := range reports {
		// Marshal report
		data, err := json.Marshal(report)
		if err != nil {
			log.Warnf("Failed to marshal spam report for broadcasting: %v", err)
			continue
		}

		// Inject directly into aggregator (Gossipsub doesn't deliver self-messages)
		if m.aggregator != nil {
			go m.aggregator.processSpamReportDirect(data)
		}

		// Queue report for broadcasting via p2p-gateway
		if err := m.redisClient.LPush(m.ctx, broadcastQueue, data).Err(); err != nil {
			log.Warnf("Failed to queue spam report for broadcasting: %v", err)
			continue
		}

		sentCount++
	}

	log.WithFields(log.Fields{
		"total_reports": len(reports),
		"sent_count":    sentCount,
	}).Debugf("Batched and sent %d/%d spam reports", sentCount, len(reports))

	return nil
}

// sendReportsImmediately sends reports immediately without batching (for old epochs)
func (m *SpamReportWindowManager) sendReportsImmediately(ctx context.Context, dataMarket string, epochID uint64) error {
	reports, err := m.collectPendingReports(dataMarket, epochID)
	if err != nil {
		return fmt.Errorf("failed to collect reports: %w", err)
	}

	if len(reports) > 0 {
		if err := m.batchSendReports(reports); err != nil {
			return fmt.Errorf("failed to send reports: %w", err)
		}
		log.WithFields(log.Fields{
			"epoch_id":     epochID,
			"report_count": len(reports),
		}).Infof("Sent %d spam reports immediately for epoch %d (past collection window)", len(reports), epochID)
		m.cleanupPendingReportsKey(dataMarket, epochID)
	}

	return nil
}

// cleanupPendingReportsKey removes the pending reports key from Redis
func (m *SpamReportWindowManager) cleanupPendingReportsKey(dataMarket string, epochID uint64) {
	pendingKey := m.getPendingReportsKey(dataMarket, epochID)
	if err := m.redisClient.Del(m.ctx, pendingKey).Err(); err != nil {
		log.Debugf("Failed to cleanup pending reports key for epoch %d: %v", epochID, err)
	}
}

// getPendingReportsKey returns the Redis key for pending reports for an epoch
func (m *SpamReportWindowManager) getPendingReportsKey(dataMarket string, epochID uint64) string {
	return fmt.Sprintf("%s:%s:spam:reports:pending:epoch:%d", m.keyBuilder.ProtocolState, dataMarket, epochID)
}

// Shutdown sends all pending reports immediately and cleans up
func (m *SpamReportWindowManager) Shutdown() {
	log.Info("Shutting down spam report window manager, sending all pending reports...")

	m.mu.Lock()
	windows := make([]*ReportWindow, 0, len(m.activeWindows))
	for _, window := range m.activeWindows {
		windows = append(windows, window)
	}
	m.mu.Unlock()

	// Send all pending reports immediately
	for _, window := range windows {
		if window.Timer != nil {
			window.Timer.Stop()
		}
		reports, err := m.collectPendingReports(window.DataMarketAddress, window.EpochID)
		if err == nil && len(reports) > 0 {
			m.batchSendReports(reports)
			m.cleanupPendingReportsKey(window.DataMarketAddress, window.EpochID)
		}
	}

	log.Info("Spam report window manager shutdown complete")
}
