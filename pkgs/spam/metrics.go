package spam

import (
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/metrics"
)

// SpamMetrics tracks Prometheus metrics for spam protection
type SpamMetrics struct {
	reportsSent        *metrics.Counter
	reportsReceived    *metrics.Counter
	consensusReached   *metrics.Counter
	submissionsRejected *metrics.Counter
	validationFailures *metrics.Counter
	registry           *metrics.Registry
}

// NewSpamMetrics creates a new SpamMetrics instance
func NewSpamMetrics(registry *metrics.Registry) *SpamMetrics {
	return &SpamMetrics{
		reportsSent: registry.GetOrCreate(metrics.MetricConfig{
			Name:   "spam_reports_sent_total",
			Type:   metrics.MetricTypeCounter,
			Help:   "Total spam reports sent to validator mesh",
			Labels: metrics.Labels{},
		}).(*metrics.Counter),
		reportsReceived: registry.GetOrCreate(metrics.MetricConfig{
			Name:   "spam_reports_received_total",
			Type:   metrics.MetricTypeCounter,
			Help:   "Total spam reports received from other validators",
			Labels: metrics.Labels{},
		}).(*metrics.Counter),
		consensusReached: registry.GetOrCreate(metrics.MetricConfig{
			Name:   "spam_consensus_reached_total",
			Type:   metrics.MetricTypeCounter,
			Help:   "Total times consensus was reached for flagging peers",
			Labels: metrics.Labels{},
		}).(*metrics.Counter),
		submissionsRejected: registry.GetOrCreate(metrics.MetricConfig{
			Name:   "spam_submissions_rejected_total",
			Type:   metrics.MetricTypeCounter,
			Help:   "Total submissions rejected due to spam protection",
			Labels: metrics.Labels{},
		}).(*metrics.Counter),
		validationFailures: registry.GetOrCreate(metrics.MetricConfig{
			Name:   "spam_validation_failures_total",
			Type:   metrics.MetricTypeCounter,
			Help:   "Total validation failures tracked for spam detection",
			Labels: metrics.Labels{},
		}).(*metrics.Counter),
		registry: registry,
	}
}

// IncReportsSent increments the spam reports sent counter
func (m *SpamMetrics) IncReportsSent() {
	if m.reportsSent != nil {
		m.reportsSent.Inc()
	}
}

// IncReportsReceived increments the spam reports received counter
func (m *SpamMetrics) IncReportsReceived() {
	if m.reportsReceived != nil {
		m.reportsReceived.Inc()
	}
}

// IncConsensusReached increments the consensus reached counter
func (m *SpamMetrics) IncConsensusReached() {
	if m.consensusReached != nil {
		m.consensusReached.Inc()
	}
}

// IncSubmissionsRejected increments the submissions rejected counter
func (m *SpamMetrics) IncSubmissionsRejected(reason string) {
	if m.submissionsRejected != nil {
		m.submissionsRejected.Inc()
	}
}

// IncValidationFailures increments the validation failures counter
func (m *SpamMetrics) IncValidationFailures() {
	if m.validationFailures != nil {
		m.validationFailures.Inc()
	}
}

