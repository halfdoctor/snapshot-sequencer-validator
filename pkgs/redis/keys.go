package redis

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// KeyBuilder provides methods to generate namespaced Redis keys
type KeyBuilder struct {
	ProtocolState string
	DataMarket    string
}

// checksumAddress converts an Ethereum address to checksummed format (EIP-55).
// If the input is not a valid Ethereum address, it returns the input unchanged.
// This ensures all Redis keys use consistent checksummed addresses.
func checksumAddress(addr string) string {
	// Handle empty addresses
	if addr == "" {
		return addr
	}

	// Try to parse as Ethereum address and convert to checksummed format
	if common.IsHexAddress(addr) {
		address := common.HexToAddress(addr)
		return address.Hex() // .Hex() returns checksummed format (EIP-55)
	}

	// If not a valid address, return as-is (might be a non-address identifier)
	return addr
}

// NewKeyBuilder creates a new KeyBuilder instance with checksummed addresses.
// All Ethereum addresses are converted to EIP-55 checksummed format for consistent Redis keys.
func NewKeyBuilder(protocolState, dataMarket string) *KeyBuilder {
	return &KeyBuilder{
		ProtocolState: checksumAddress(protocolState),
		DataMarket:    checksumAddress(dataMarket),
	}
}

// P2P Gateway Keys

// SubmissionQueue returns the key for raw P2P submissions queue
func (kb *KeyBuilder) SubmissionQueue() string {
	return fmt.Sprintf("%s:%s:submissionQueue", kb.ProtocolState, kb.DataMarket)
}

// IncomingBatch returns the key for received batch from validator
func (kb *KeyBuilder) IncomingBatch(epochID, validatorID string) string {
	return fmt.Sprintf("%s:%s:incoming:batch:%s:%s", kb.ProtocolState, kb.DataMarket, epochID, validatorID)
}

// AggregationQueue returns the key for Level 2 aggregation queue
func (kb *KeyBuilder) AggregationQueue() string {
	return fmt.Sprintf("%s:%s:aggregation:queue", kb.ProtocolState, kb.DataMarket)
}

// OutgoingBroadcastBatch returns the key for batches to broadcast
func (kb *KeyBuilder) OutgoingBroadcastBatch() string {
	return fmt.Sprintf("%s:%s:outgoing:broadcast:batch", kb.ProtocolState, kb.DataMarket)
}

// OutgoingSpamReports returns the key for spam reports to broadcast
func (kb *KeyBuilder) OutgoingSpamReports() string {
	return fmt.Sprintf("%s:%s:outgoing:spam-reports", kb.ProtocolState, kb.DataMarket)
}

// IncomingSpamReports returns the key for received spam reports queue
func (kb *KeyBuilder) IncomingSpamReports() string {
	return fmt.Sprintf("%s:%s:incoming:spam-reports", kb.ProtocolState, kb.DataMarket)
}

// Dequeuer Keys

// ProcessingSubmission returns the key for submission being processed
func (kb *KeyBuilder) ProcessingSubmission(id string) string {
	return fmt.Sprintf("processingSubmission:%s", id)
}

// ProcessedSubmission returns the key for validated submission
func (kb *KeyBuilder) ProcessedSubmission(sequencerID, submissionID string) string {
	return fmt.Sprintf("%s:%s:processed:%s:%s", kb.ProtocolState, kb.DataMarket, sequencerID, submissionID)
}

// EpochProcessed returns the key for submission IDs in an epoch
func (kb *KeyBuilder) EpochProcessed(epochID string) string {
	return fmt.Sprintf("%s:%s:epoch:%s:processed", kb.ProtocolState, kb.DataMarket, epochID)
}

// EpochSubmissionsIds returns the ZSET key for submission IDs in an epoch (deterministic, ordered by timestamp)
func (kb *KeyBuilder) EpochSubmissionsIds(epochID string) string {
	return fmt.Sprintf("%s:%s:epoch:%s:submissions:ids", kb.ProtocolState, kb.DataMarket, epochID)
}

// EpochSubmissionsData returns the HASH key for submission data in an epoch (deterministic lookup)
func (kb *KeyBuilder) EpochSubmissionsData(epochID string) string {
	return fmt.Sprintf("%s:%s:epoch:%s:submissions:data", kb.ProtocolState, kb.DataMarket, epochID)
}

// Event Monitor Keys

// EpochWindow returns the key for submission window status
func (kb *KeyBuilder) EpochWindow(epochID string) string {
	return fmt.Sprintf("%s:%s:epoch:%s:window", kb.ProtocolState, kb.DataMarket, epochID)
}

// EpochState returns the key for comprehensive epoch state hash
// Format: {protocol}:{market}:epoch:{epochId}:state
func (kb *KeyBuilder) EpochState(epochID string) string {
	return fmt.Sprintf("%s:%s:epoch:%s:state", kb.ProtocolState, kb.DataMarket, epochID)
}

// FinalizationQueue returns the key for epochs ready for finalization
func (kb *KeyBuilder) FinalizationQueue() string {
	return fmt.Sprintf("%s:%s:finalizationQueue", kb.ProtocolState, kb.DataMarket)
}

// Finalizer Keys

// BatchPart returns the key for partial batch from worker
func (kb *KeyBuilder) BatchPart(epochID string, partID int) string {
	return fmt.Sprintf("%s:%s:batch:part:%s:%d", kb.ProtocolState, kb.DataMarket, epochID, partID)
}

// EpochPartsCompleted returns the key for count of completed parts
func (kb *KeyBuilder) EpochPartsCompleted(epochID string) string {
	return fmt.Sprintf("%s:%s:epoch:%s:parts:completed", kb.ProtocolState, kb.DataMarket, epochID)
}

// EpochPartsTotal returns the key for total expected parts
func (kb *KeyBuilder) EpochPartsTotal(epochID string) string {
	return fmt.Sprintf("%s:%s:epoch:%s:parts:total", kb.ProtocolState, kb.DataMarket, epochID)
}

// EpochPartsReady returns the key for ready status flag
func (kb *KeyBuilder) EpochPartsReady(epochID string) string {
	return fmt.Sprintf("%s:%s:epoch:%s:parts:ready", kb.ProtocolState, kb.DataMarket, epochID)
}

// AggregationQueueLevel1 returns the key for worker parts ready for Level 1 aggregation
func (kb *KeyBuilder) AggregationQueueLevel1() string {
	return fmt.Sprintf("%s:%s:aggregationQueue", kb.ProtocolState, kb.DataMarket)
}

// FinalizedBatch returns the key for complete local finalized batch
func (kb *KeyBuilder) FinalizedBatch(epochID string) string {
	return fmt.Sprintf("%s:%s:finalized:%s", kb.ProtocolState, kb.DataMarket, epochID)
}

// VPA Priority Caching Keys

// VPAPriorities returns the key for cached validator priorities for an epoch
func (kb *KeyBuilder) VPAPriorities(epochID string) string {
	return fmt.Sprintf("%s:%s:vpa:priorities:%s", kb.ProtocolState, kb.DataMarket, epochID)
}

// VPAValidatorPriority returns the key for cached priority of a specific validator
func (kb *KeyBuilder) VPAValidatorPriority(epochID, validatorID string) string {
	return fmt.Sprintf("%s:%s:vpa:priority:%s:%s", kb.ProtocolState, kb.DataMarket, epochID, validatorID)
}

// VPATopValidator returns the key for cached top priority validator for an epoch
func (kb *KeyBuilder) VPATopValidator(epochID string) string {
	return fmt.Sprintf("%s:%s:vpa:top:%s", kb.ProtocolState, kb.DataMarket, epochID)
}

// VPAActiveValidators returns the key for cached list of active validators for an epoch
func (kb *KeyBuilder) VPAActiveValidators(epochID string) string {
	return fmt.Sprintf("%s:%s:vpa:validators:%s", kb.ProtocolState, kb.DataMarket, epochID)
}

// VPAPriorityMetadata returns the key for priority assignment metadata
func (kb *KeyBuilder) VPAPriorityMetadata(epochID string) string {
	return fmt.Sprintf("%s:%s:vpa:metadata:%s", kb.ProtocolState, kb.DataMarket, epochID)
}

// VPA Submission Queue Keys

// VPASubmissionQueue returns the key for VPA submission requests queue
func (kb *KeyBuilder) VPASubmissionQueue() string {
	return fmt.Sprintf("%s:%s:vpa:submission:queue", kb.ProtocolState, kb.DataMarket)
}

// VPA Monitoring Keys

// VPAPriorityAssignment returns the key for priority assignment per epoch
// Format: {protocol}:{market}:vpa:priority:{epochID}
func (kb *KeyBuilder) VPAPriorityAssignment(epochID string) string {
	return fmt.Sprintf("%s:%s:vpa:priority:%s", kb.ProtocolState, kb.DataMarket, epochID)
}

// VPASubmissionResult returns the key for submission result per epoch
// Format: {protocol}:{market}:vpa:submission:{epochID}
func (kb *KeyBuilder) VPASubmissionResult(epochID string) string {
	return fmt.Sprintf("%s:%s:vpa:submission:%s", kb.ProtocolState, kb.DataMarket, epochID)
}

// VPAPriorityTimeline returns the key for priority assignment timeline (namespaced by protocol:market)
// Format: {protocol}:{market}:vpa:priority:timeline
func (kb *KeyBuilder) VPAPriorityTimeline() string {
	return fmt.Sprintf("%s:%s:vpa:priority:timeline", kb.ProtocolState, kb.DataMarket)
}

// VPASubmissionTimeline returns the key for submission timeline (namespaced by protocol:market)
// Format: {protocol}:{market}:vpa:submission:timeline
func (kb *KeyBuilder) VPASubmissionTimeline() string {
	return fmt.Sprintf("%s:%s:vpa:submission:timeline", kb.ProtocolState, kb.DataMarket)
}

// VPAStats returns the key for VPA statistics per market
// Format: {protocol}:{market}:vpa:stats
func (kb *KeyBuilder) VPAStats() string {
	return fmt.Sprintf("%s:%s:vpa:stats", kb.ProtocolState, kb.DataMarket)
}

// VPAEpochStatus returns the key for per-epoch VPA status (priority + submission combined)
// Format: {protocol}:{market}:vpa:epoch:{epochID}:status
func (kb *KeyBuilder) VPAEpochStatus(epochID string) string {
	return fmt.Sprintf("%s:%s:vpa:epoch:%s:status", kb.ProtocolState, kb.DataMarket, epochID)
}

// Aggregator Keys

// BatchAggregated returns the key for network-wide consensus batch
func (kb *KeyBuilder) BatchAggregated(epochID string) string {
	return fmt.Sprintf("%s:%s:batch:aggregated:%s", kb.ProtocolState, kb.DataMarket, epochID)
}

// Stream-based Aggregation Keys

// AggregationStream returns the key for aggregation notification stream
func (kb *KeyBuilder) AggregationStream() string {
	return fmt.Sprintf("%s:%s:stream:aggregation:notifications", kb.ProtocolState, kb.DataMarket)
}

// AggregatorConsumerGroup returns the key for aggregator consumer group
func (kb *KeyBuilder) AggregatorConsumerGroup() string {
	return fmt.Sprintf("%s:%s:consumers:aggregator", kb.ProtocolState, kb.DataMarket)
}

// EpochValidators returns the key for validator set tracking per epoch
func (kb *KeyBuilder) EpochValidators(epochID string) string {
	return fmt.Sprintf("%s:%s:epoch:%s:validators", kb.ProtocolState, kb.DataMarket, epochID)
}

// ActiveEpochs returns the key for active epochs tracking
func (kb *KeyBuilder) ActiveEpochs() string {
	return fmt.Sprintf("%s:%s:epochs:active", kb.ProtocolState, kb.DataMarket)
}

// EpochsGaps returns the key for epoch gaps tracking
func (kb *KeyBuilder) EpochsGaps() string {
	return fmt.Sprintf("%s:%s:epochs:gaps", kb.ProtocolState, kb.DataMarket)
}

// Metrics Keys (namespaced per protocol:market)

// MetricsEpochsTimeline returns the namespaced key for epochs timeline
func (kb *KeyBuilder) MetricsEpochsTimeline() string {
	return fmt.Sprintf("%s:%s:metrics:epochs:timeline", kb.ProtocolState, kb.DataMarket)
}

// MetricsBatchesTimeline returns the namespaced key for batches timeline
func (kb *KeyBuilder) MetricsBatchesTimeline() string {
	return fmt.Sprintf("%s:%s:metrics:batches:timeline", kb.ProtocolState, kb.DataMarket)
}

// MetricsBatchLocal returns the namespaced key for local batch metadata
func (kb *KeyBuilder) MetricsBatchLocal(epochID string) string {
	return fmt.Sprintf("%s:%s:metrics:batch:local:%s", kb.ProtocolState, kb.DataMarket, epochID)
}

// MetricsBatchAggregated returns the namespaced key for aggregated batch metadata
func (kb *KeyBuilder) MetricsBatchAggregated(epochID string) string {
	return fmt.Sprintf("%s:%s:metrics:batch:aggregated:%s", kb.ProtocolState, kb.DataMarket, epochID)
}

// MetricsBatchValidators returns the namespaced key for batch validator list
func (kb *KeyBuilder) MetricsBatchValidators(epochID string) string {
	return fmt.Sprintf("%s:%s:metrics:batch:%s:validators", kb.ProtocolState, kb.DataMarket, epochID)
}

// MetricsEpochInfo returns the namespaced key for epoch info
func (kb *KeyBuilder) MetricsEpochInfo(epochID string) string {
	return fmt.Sprintf("%s:%s:metrics:epoch:%s:info", kb.ProtocolState, kb.DataMarket, epochID)
}

// MetricsEpochParts returns the namespaced key for epoch parts tracking
func (kb *KeyBuilder) MetricsEpochParts(epochID string) string {
	return fmt.Sprintf("%s:%s:metrics:epoch:%s:parts", kb.ProtocolState, kb.DataMarket, epochID)
}

// MetricsEpochValidated returns the namespaced key for validated epoch status
func (kb *KeyBuilder) MetricsEpochValidated(epochID string) string {
	return fmt.Sprintf("%s:%s:metrics:epoch:%s:validated", kb.ProtocolState, kb.DataMarket, epochID)
}

// MetricsValidatorBatches returns the namespaced key for validator batch timeline
func (kb *KeyBuilder) MetricsValidatorBatches(validatorID string) string {
	return fmt.Sprintf("%s:%s:metrics:validator:%s:batches", kb.ProtocolState, kb.DataMarket, validatorID)
}

// MetricsBatchPart returns the namespaced key for batch part metrics
func (kb *KeyBuilder) MetricsBatchPart(epochID string, partID int) string {
	return fmt.Sprintf("%s:%s:metrics:part:%s:%d", kb.ProtocolState, kb.DataMarket, epochID, partID)
}

// State-Tracker Keys (namespaced)

// DashboardSummary returns the namespaced key for dashboard summary data
func (kb *KeyBuilder) DashboardSummary() string {
	return fmt.Sprintf("%s:%s:dashboard:summary", kb.ProtocolState, kb.DataMarket)
}

// StatsCurrent returns the namespaced key for current statistics
func (kb *KeyBuilder) StatsCurrent() string {
	return fmt.Sprintf("%s:%s:stats:current", kb.ProtocolState, kb.DataMarket)
}

// StatsDaily returns the namespaced key for daily statistics
func (kb *KeyBuilder) StatsDaily() string {
	return fmt.Sprintf("%s:%s:stats:daily", kb.ProtocolState, kb.DataMarket)
}

// StatsHourly returns the namespaced key for hourly statistics
func (kb *KeyBuilder) StatsHourly(timestamp int64) string {
	return fmt.Sprintf("%s:%s:stats:hourly:%d", kb.ProtocolState, kb.DataMarket, timestamp)
}

// MetricsSubmissionsTimeline returns the namespaced key for submissions timeline
func (kb *KeyBuilder) MetricsSubmissionsTimeline() string {
	return fmt.Sprintf("%s:%s:metrics:submissions:timeline", kb.ProtocolState, kb.DataMarket)
}

// MetricsSubmissionsMetadata returns the namespaced key for submissions metadata
func (kb *KeyBuilder) MetricsSubmissionsMetadata(entityID string) string {
	return fmt.Sprintf("%s:%s:metrics:submissions:metadata:%s", kb.ProtocolState, kb.DataMarket, entityID)
}

// MetricsValidationsTimeline returns the namespaced key for validations timeline
func (kb *KeyBuilder) MetricsValidationsTimeline() string {
	return fmt.Sprintf("%s:%s:metrics:validations:timeline", kb.ProtocolState, kb.DataMarket)
}

// MetricsPartsTimeline returns the namespaced key for parts timeline
func (kb *KeyBuilder) MetricsPartsTimeline() string {
	return fmt.Sprintf("%s:%s:metrics:parts:timeline", kb.ProtocolState, kb.DataMarket)
}

// MetricsParticipation returns the namespaced key for participation metrics
func (kb *KeyBuilder) MetricsParticipation() string {
	return fmt.Sprintf("%s:%s:metrics:participation", kb.ProtocolState, kb.DataMarket)
}

// MetricsCurrentEpoch returns the namespaced key for current epoch status
func (kb *KeyBuilder) MetricsCurrentEpoch() string {
	return fmt.Sprintf("%s:%s:metrics:current_epoch", kb.ProtocolState, kb.DataMarket)
}

// Simulation Message Keys (namespaced)
// Used for caching epoch 0 simulation messages from snapshotters at startup
// These messages contain real CIDs and EIP-712 signatures (unlike heartbeats which have empty CIDs)

// SimulationsTimeline returns the ZSET key for simulation events ordered by timestamp
// Format: {protocol}:{market}:simulations:timeline
func (kb *KeyBuilder) SimulationsTimeline() string {
	return fmt.Sprintf("%s:%s:simulations:timeline", kb.ProtocolState, kb.DataMarket)
}

// SimulationMetadata returns the HASH key for detailed simulation metadata
// Format: {protocol}:{market}:simulations:metadata:{entityID}
// Fields: peer_id, snapshotter_address, slot_id, project_id, snapshot_cid, data_market, timestamp
func (kb *KeyBuilder) SimulationMetadata(entityID string) string {
	return fmt.Sprintf("%s:%s:simulations:metadata:%s", kb.ProtocolState, kb.DataMarket, entityID)
}

// SimulationsByPeer returns the SET key for tracking simulations per peer ID
// Format: {protocol}:{market}:simulations:peer:{peerID}
// Members: simulation entity IDs
func (kb *KeyBuilder) SimulationsByPeer(peerID string) string {
	return fmt.Sprintf("%s:%s:simulations:peer:%s", kb.ProtocolState, kb.DataMarket, peerID)
}

// SimulationsBySnapshotter returns the SET key for tracking simulations per snapshotter address
// Format: {protocol}:{market}:simulations:snapshotter:{address}
// Members: simulation entity IDs
func (kb *KeyBuilder) SimulationsBySnapshotter(address string) string {
	return fmt.Sprintf("%s:%s:simulations:snapshotter:%s", kb.ProtocolState, kb.DataMarket, checksumAddress(address))
}

// Monitoring Keys (not namespaced)

// PipelineHealth returns the key for component health status
func PipelineHealth(component string) string {
	return fmt.Sprintf("pipeline:health:%s", component)
}

// SubmissionStats returns the key for epoch statistics
func SubmissionStats(epochID string) string {
	return fmt.Sprintf("submission_stats:%s", epochID)
}

// ValidatorActive returns the key for active validator tracking
func ValidatorActive(validatorID string) string {
	return fmt.Sprintf("validator:active:%s", validatorID)
}

// Worker Keys (not namespaced)

// WorkerStatus returns the key for worker status
func WorkerStatus(workerType, workerID string) string {
	return fmt.Sprintf("worker:%s:%s:status", workerType, workerID)
}

// WorkerHeartbeat returns the key for worker heartbeat
func WorkerHeartbeat(workerType, workerID string) string {
	return fmt.Sprintf("worker:%s:%s:heartbeat", workerType, workerID)
}

// WorkerCurrentBatch returns the key for current batch being processed
func WorkerCurrentBatch(workerType, workerID string) string {
	return fmt.Sprintf("worker:%s:%s:current_batch", workerType, workerID)
}

// WorkerCurrentEpoch returns the key for current epoch (aggregator only)
func WorkerCurrentEpoch(workerType string) string {
	return fmt.Sprintf("worker:%s:current_epoch", workerType)
}

// WorkerBatchesProcessed returns the key for processed count
func WorkerBatchesProcessed(workerType, workerID string) string {
	return fmt.Sprintf("worker:%s:%s:batches_processed", workerType, workerID)
}

// WorkerLastError returns the key for last error info
func WorkerLastError(workerType, workerID string) string {
	return fmt.Sprintf("worker:%s:%s:last_error", workerType, workerID)
}

// BatchPartStatus returns the key for batch part status tracking
func BatchPartStatus(epochID string, batchID int) string {
	return fmt.Sprintf("batch:%s:part:%d:status", epochID, batchID)
}

// BatchPartWorker returns the key for worker assignment
func BatchPartWorker(epochID string, batchID int) string {
	return fmt.Sprintf("batch:%s:part:%d:worker", epochID, batchID)
}

// Metrics Keys (not namespaced)

// MetricsProcessingRate returns the key for processing rate metric
func MetricsProcessingRate() string {
	return "metrics:processing_rate"
}

// MetricsAvgLatency returns the key for average latency metric
func MetricsAvgLatency() string {
	return "metrics:avg_latency"
}

// MetricsTotalProcessed returns the key for total processed metric
func MetricsTotalProcessed() string {
	return "metrics:total_processed"
}

// MetricsSubmissionsTimeline returns the key for submissions timeline
func MetricsSubmissionsTimeline() string {
	return "metrics:submissions:timeline"
}

// MetricsValidationsTimeline returns the key for validations timeline
func MetricsValidationsTimeline() string {
	return "metrics:validations:timeline"
}

// MetricsEpochsTimeline returns the key for epochs timeline
func MetricsEpochsTimeline() string {
	return "metrics:epochs:timeline"
}

// MetricsBatchesTimeline returns the key for batches timeline
func MetricsBatchesTimeline() string {
	return "metrics:batches:timeline"
}

// MetricsPartsTimeline returns the key for parts timeline
func MetricsPartsTimeline() string {
	return "metrics:parts:timeline"
}

// Legacy/Deprecated Keys (for backward compatibility if needed)

// BatchFinalized returns the legacy key for finalized batch
func BatchFinalized(epochID string) string {
	return fmt.Sprintf("batch:finalized:%s", epochID)
}
