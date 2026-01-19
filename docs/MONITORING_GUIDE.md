  # Batch Processing Pipeline Monitoring Guide

## Overview

The monitoring system provides comprehensive visibility into the batch processing pipeline, tracking data flow from submission collection through final aggregation.

**Key Features:**
- **Epoch-Centered Observability**: Complete state tracking for each epoch through all phases
- **Gap Detection**: Automatic identification of epochs with missing finalizations
- **Phase Tracking**: Real-time visibility into submission, finalization, aggregation, and on-chain phases
- **Relayer-Py Integration**: Automatic transaction status tracking - relayer-py writes `onchain_status`, `onchain_tx_hash`, and `onchain_block_number` directly to Redis (no log parsing needed)
- **Sorted Responses**: All timeline endpoints return epochs sorted by ID (most recent first)

The system supports both P2PSnapshotSubmission batch format (multiple submissions per message) and single SnapshotSubmission format, with proper Redis key patterns for data organization.

## RESTful Monitor API

The monitoring system includes a complete RESTful API with 10 endpoints and interactive Swagger UI for comprehensive monitoring and debugging.

### Accessing the Monitor API

```bash
# Monitor API runs on port 9091 (configurable via MONITOR_API_PORT)
http://localhost:9091/swagger/index.html

# Interactive Swagger UI provides:
# - Complete API documentation
# - Interactive testing of all endpoints
# - Request/response examples
# - Protocol/market query parameter support
```

### All Monitoring Endpoints

| Endpoint | Purpose | Query Parameters |
|----------|---------|-----------------|
| `/api/v1/health` | Service health check | None |
| `/api/v1/dashboard/summary` | Real-time dashboard metrics | protocol, market |
| `/api/v1/epochs/timeline` | Epoch progression timeline (sorted by epoch ID) | protocol, market, limit |
| `/api/v1/epochs/{epochId}/status` | Complete epoch state with all phase information | protocol, market |
| `/api/v1/epochs/{epochId}/submissions` | All submissions for an epoch (slot ID, peer ID, project ID, CID) | protocol, market |
| `/api/v1/epochs/active` | Epochs currently in progress (queries timeline directly for accuracy) | protocol, market |
| `/api/v1/epochs/gaps` | Identify epochs with missing finalizations | protocol, market, window_minutes |
| `/api/v1/batches/finalized` | Recently finalized batches (with phase and onchain status) | protocol, market, level, epoch_id, limit |
| `/api/v1/aggregation/results` | Network aggregation results | protocol, market |
| `/api/v1/timeline/recent` | Recent activity feed | protocol, market |
| `/api/v1/queues/status` | Queue monitoring | protocol, market |
| `/api/v1/pipeline/overview` | Pipeline status summary | protocol, market |
| `/api/v1/stats/daily` | Daily aggregated statistics | protocol, market |
| `/api/v1/stats/hourly` | Hourly performance metrics | protocol, market |

### Using Query Parameters

All endpoints support protocol/market filtering for multi-market environments:

```bash
# Filter by specific protocol and market
curl "http://localhost:9091/api/v1/dashboard/summary?protocol=powerloom&market=0x21cb57C1f2352ad215a463DD867b838749CD3b8f"

# Multiple markets (comma-separated)
curl "http://localhost:9091/api/v1/batches/finalized?market=0x21cb57C1f2352ad215a463DD867b838749CD3b8f,0x0C2E22fe7526fAeF28E7A58c84f8723dEFcE200c"

# JSON array format
curl "http://localhost:9091/api/v1/aggregation/results?market=[\"0x21cb57C1f2352ad215a463DD867b838749CD3b8f\",\"0x0C2E22fe7526fAeF28E7A58c84f8723dEFcE200c\"]"
```

### Working Endpoints Status

All endpoints are operational with real data:
- **Health Check**: Shows `data_fresh: true` with actual pipeline data
- **Dashboard**: Real metrics with participation rates, current epoch status, gap detection
- **Epoch Timeline**: Actual epoch progression with correct status reading, sorted by epoch ID
- **Epoch Status**: Complete epoch state with all phase transitions and on-chain status
- **Active Epochs**: Real-time view of epochs currently in progress
- **Epoch Gaps**: Identifies epochs with missing finalizations for troubleshooting
- **Finalized Batches**: Batch data with validator attribution, IPFS CIDs, phase, and onchain status
- **Aggregation Results**: Network-wide consensus with validator counting
- **Timeline Activity**: Recent submissions and batch completions
- **Queue Status**: Real-time queue depths and processing rates with accurate monitoring
- **Pipeline Overview**: Complete pipeline status with health indicators
- **Daily/Stats**: Actual aggregated data from pipeline metrics

### Queue Monitoring System

The queue monitoring system accurately reflects system health by monitoring active components.

#### Queue Monitoring Architecture
**Stream-Based System**:
- `stream:aggregation:notifications` - Primary operational queue
- Monitored via `getStreamLag()` for accurate consumer lag
- Handles real-time notifications efficiently

#### Queue Monitoring Commands:
```bash
# Check queue status
curl "http://localhost:9091/api/v1/queues/status" | jq '.aggregation_queue_depth'

# Monitor queue health history
curl "http://localhost:9091/api/v1/pipeline/overview" | jq '.queue_health_history'
```

### Key Features

- **Real-time Data**: All endpoints read from actual pipeline data sources
- **Proper Namespacing**: Uses `{protocol}:{market}:*` Redis keys correctly
- **Multi-market Support**: Handles comma-separated and JSON array market configurations
- **Production-safe**: Uses SCAN operations instead of KEYS
- **Validator Attribution**: Tracks which validators contributed to each batch
- **IPFS Integration**: Includes proper IPFS CIDs and Merkle roots in responses

### Monitoring Workflow

```bash
# 1. Check overall health
curl "http://localhost:9091/api/v1/health"

# 2. Get dashboard summary
curl "http://localhost:9091/api/v1/dashboard/summary"

# 3. View recent epochs
curl "http://localhost:9091/api/v1/epochs/timeline"

# 4. Check a specific epoch's status
curl "http://localhost:9091/api/v1/epochs/23875280/status?protocol=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401&market=0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d"

# 5. Get all submissions for an epoch (with slot ID, peer ID, project ID, CID)
curl "http://localhost:9091/api/v1/epochs/23875280/submissions?protocol=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401&market=0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d"

# 6. Check finalized batches
curl "http://localhost:9091/api/v1/batches/finalized"

# 7. Monitor network aggregation
curl "http://localhost:9091/api/v1/aggregation/results"
```

### Aggregator Metrics Counting

The aggregator metrics reporting system correctly counts finalized batches.

#### Technical Implementation:
```go
// Counts timeline entries with "aggregated:" prefix
timelineKey := a.keyBuilder.MetricsBatchesTimeline(protocol, market)
timelineEntries, err := a.redisClient.ZRange(a.ctx, timelineKey, 0, -1).Result()
for _, entry := range timelineEntries {
    if strings.HasPrefix(entry, "aggregated:") {
        aggregatedCount++
    }
}
```

#### Verification Commands:
```bash
# Check aggregator metrics
curl "http://localhost:9091/api/v1/aggregation/results" | jq '.finalized_batches'

# Monitor timeline entries directly
docker exec redis redis-cli ZRANGE "powerloom:powerloom-localnet:metrics:batches:timeline" 0 -1
```

### Queue Monitoring

The queue monitoring system provides accurate consumer lag metrics by monitoring Redis streams.

```bash
# Check queue status
curl "http://localhost:9091/api/v1/queues/status"
```

#### Key Features:
- Stream-based monitoring provides accurate consumer lag metrics
- Real-time visibility into actual processing performance

### Epoch ID Formatting

All epoch IDs display as integers for consistency across the system.

```bash
# API responses show consistent integer format
curl "http://localhost:9091/api/v1/aggregation/results?protocol=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401&market=0xae32c4FA72E2e5F53ed4D214E4aD049286Ded16f"

# Expected output: "epoch_id": "23646205" (integer)
```

#### Epoch ID Handling:
- All components handle epoch ID formats consistently
- Epoch formatting utility in `pkgs/utils/epoch_formatter.go`
- Consistent integer display in all API responses

## Quick Start

### Basic Monitoring
```bash
# Check service status
./dsv.sh status

# View all logs
./dsv.sh logs

# View specific component logs
./dsv.sh aggregator-logs
./dsv.sh p2p-logs
./dsv.sh finalizer-logs
./dsv.sh dequeuer-logs
./dsv.sh event-logs
./dsv.sh redis-logs

# Access interactive dashboard
./dsv.sh dashboard

# Stream debugging commands
./dsv.sh stream-info
./dsv.sh stream-groups
./dsv.sh stream-dlq

# Queue management
./dsv.sh clean-queue
./dsv.sh clean-timeline
```

### Advanced Monitoring
```bash
# Check accurate queue health (stream-based)
curl "http://localhost:9091/api/v1/queues/status"

# Monitor real-time aggregation results
curl "http://localhost:9091/api/v1/aggregation/results"

# Check dashboard summary
curl "http://localhost:9091/api/v1/dashboard/summary"

# Verify system pipeline status
curl "http://localhost:9091/api/v1/pipeline/overview"

# Get timeline data
curl "http://localhost:9091/api/v1/epochs/timeline"

# View recent activity
curl "http://localhost:9091/api/v1/timeline/recent"

# Get daily stats
curl "http://localhost:9091/api/v1/stats/daily"
```

## Pipeline Stages

The monitoring system tracks 6 distinct stages:

### Stage 1: Submission Collection
- **Active Windows**: Open submission windows accepting data (format: `epoch:market:epochID:window`)
- **Queue Depth**: Pending submissions in processing queue
- **Vote Distribution**: Multiple CIDs per project with vote counts
- **Batch vs Single Submissions**: Now handles P2PSnapshotSubmission batch format

### Stage 2: Batch Splitting
- **Split Metadata**: How epochs are divided into parallel batches
- **Finalization Queue**: Batches waiting for processing
- **Batch Size**: Configurable via `FINALIZATION_BATCH_SIZE` (default: 20)

### Stage 3: Parallel Finalization
- **Worker Status**: Individual finalizer worker health
- **Batch Parts**: Processing status of each batch part
- **Worker Assignment**: Which worker handles which batch

### Stage 4: Aggregation
- **Ready Epochs**: Epochs with all parts completed
- **Aggregation Queue**: Epochs awaiting final assembly
- **Aggregator Status**: Single worker combining batch parts

### Stage 5: Final Output
- **Finalized Batches**: Completed batches with IPFS CIDs
- **Merkle Roots**: Cryptographic proofs of batch contents
- **Validator Broadcasts**: Vote dissemination status

### Stage 6: Consensus/Batch Aggregation (LIVE)
- **Validator Batch Exchange**: Real-time tracking of batch broadcasts between validators
- **Local Aggregation**: Each validator aggregates all received batches locally
- **Vote Counting**: Per-project CID vote tallies from all validators
- **Consensus Determination**: Local consensus based on majority vote per project
- **Decentralized Aggregation**: Each validator maintains their own local batch aggregation
- **No Global Consensus**: Independent local vote determination
- **Gossipsub Broadcast**: Batches shared via `/powerloom/finalized-batches/all`

## Monitoring Commands

### dsv.sh Commands

| Command | Description |
|---------|-------------|
| `status` | Service status overview |
| `logs [N]` | All service logs |
| `dashboard` | Open monitoring dashboard |
| **Component Logs** | |
| `p2p-logs [N]` | P2P Gateway activity |
| `aggregator-logs [N]` | Aggregator logs |
| `finalizer-logs [N]` | Finalizer logs |
| `dequeuer-logs [N]` | Dequeuer logs |
| `event-logs [N]` | Event monitor logs |
| `redis-logs [N]` | Redis operation logs |
| `ipfs-logs [N]` | IPFS node logs |

### Stream Debugging Commands
| Command | Description |
|---------|-------------|
| `stream-info` | Show Redis streams status |
| `stream-groups` | Show consumer groups |
| `stream-dlq` | Show dead letter queue |
| `stream-reset` | Reset streams (dangerous!) |

### Queue Management Commands
| Command | Description |
|---------|-------------|
| `clean-queue` | Clean up stale aggregation queue items |
| `clean-timeline` | Clean up timeline scientific notation entries |

All log commands support optional line count (default: 100):
```bash
./dsv.sh p2p-logs 200       # Last 200 lines
./dsv.sh aggregator-logs    # Default 100 lines
```

### Direct Script Usage

```bash
# Run comprehensive pipeline monitor
./scripts/monitor_pipeline.sh

# Specify custom container
./scripts/monitor_pipeline.sh container-name
```

## Monitoring Workflow

```bash
# 1. Check overall health
curl "http://localhost:9091/api/v1/health"

# 2. Get dashboard summary
curl "http://localhost:9091/api/v1/dashboard/summary"

# 3. View recent epochs (sorted by epoch ID)
curl "http://localhost:9091/api/v1/epochs/timeline"

# 4. Get complete status for specific epoch
curl "http://localhost:9091/api/v1/epochs/23847425/status?protocol=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401&market=0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d"

# 5. View active epochs (currently processing)
curl "http://localhost:9091/api/v1/epochs/active"

# 6. Check for epoch gaps (missing finalizations)
curl "http://localhost:9091/api/v1/epochs/gaps?window_minutes=5"

# 7. Check finalized batches (with phase and onchain status)
curl "http://localhost:9091/api/v1/batches/finalized"

# 8. Monitor network aggregation
curl "http://localhost:9091/api/v1/aggregation/results"

# 9. Check queue status
curl "http://localhost:9091/api/v1/queues/status"

# 10. View pipeline overview
curl "http://localhost:9091/api/v1/pipeline/overview"
```


## Redis Keys Structure

### Consensus/Aggregation Tracking
```
# Validator batch storage
validator:{validatorID}:batch:{epochID}              # Individual validator's batch for epoch
validator:{validatorID}:epoch:{epochID}:batch        # Alternative key pattern

# Aggregation status
consensus:epoch:{epochID}:status                     # Local aggregation status for epoch
consensus:epoch:{epochID}:result                     # Final local consensus determination

# Epoch validator tracking
epoch:{epochID}:validators                           # Set of validators who submitted batches
```

### Submission Tracking (Namespaced and Corrected)
```
# Submission Collection
submissionQueue                                     # Raw P2P submissions
{protocol}:{market}:processed:{sequencerID}:{id}    # Individual processed submissions (10min TTL)
{protocol}:{market}:epoch:{epochId}:processed       # Set of submission IDs per epoch (1hr TTL)
{protocol}:{market}:epoch:{epochId}:project:{pid}:votes # Vote counts per CID (fixed namespacing)

# New consensus tracking keys with protocol/market context
{protocol}:{market}:consensus:{epochId}:total_validators    # Total validators for epoch
{protocol}:{market}:consensus:{epochId}:voting_progress     # Overall consensus voting status
{protocol}:{market}:consensus:{epochId}:project:{pid}:votes # Per-project validator votes
{protocol}:{market}:consensus:{epochId}:project:{pid}:cid   # Winning CID for project
{protocol}:{market}:consensus:{epochId}:status              # Epoch consensus status

# Type-safe performance tracking keys
{protocol}:{market}:metrics:total_processed         # Total submissions processed per market
{protocol}:{market}:metrics:processing_rate         # Submissions per minute per market
```

**Key Features**:
- Uses {protocol}:{market} namespacing for all consensus and metric keys
- Ensures consistent namespacing across submission and consensus tracking
- Provides type-safe performance metrics
- Supports multi-market performance tracking

### Window Management
```
epoch:{market}:{epochId}:window                     # Active submission windows
{protocol}:{market}:batch:ready:{epochId}          # Ready batches for finalization
batch:finalized:{epochId}                          # Finalized batches with metadata
```

### Batch Processing
```
{protocol}:{market}:epoch:{epochId}:batch:meta      # Split batch metadata
{protocol}:{market}:finalizationQueue               # Queue of batch parts
batch:{epochId}:part:{partId}:status                # Part processing status
epoch:{epochId}:parts:completed                     # Completed parts count
epoch:{epochId}:parts:total                         # Total parts count

# Stream-Based Queue System
stream:aggregation:notifications                  # Primary operational queue
epoch:{epochId}:parts:ready                        # Ready flag for aggregation
```

### Epoch ID Handling
```
# Epoch IDs are properly formatted in all components:
# Format: Integer (23646205)

# Managed via FormatEpochID() utility:
pkgs/utils/epoch_formatter.go                      # Epoch formatting
All components handle epoch ID formats consistently
```

### Queue Monitoring Architecture
```
# Stream-Based System:
- stream:aggregation:notifications
- getStreamLag() function for accurate consumer lag tracking
- Real-time queue depth monitoring
```

### Worker Monitoring
```
worker:finalizer:{id}:status              # Worker current status
worker:finalizer:{id}:heartbeat           # Last heartbeat timestamp
worker:finalizer:{id}:current_batch       # Batch being processed
worker:finalizer:{id}:batches_processed   # Total processed count
worker:aggregator:status                  # Aggregator status
worker:aggregator:current_epoch           # Epoch being aggregated
```

### Performance Metrics
```
# Namespaced Multi-Market Metrics
{protocol}:{market}:metrics:total_processed     # Total submissions processed per market/protocol
{protocol}:{market}:metrics:processing_rate     # Submissions per minute per market/protocol
{protocol}:{market}:metrics:avg_latency         # Average processing time per market/protocol
{protocol}:{market}:metrics:validator_count     # Number of active validators per market/protocol
{protocol}:{market}:metrics:consensus_rate      # Percentage of epochs reaching consensus
```

**Performance Tracking Features**:
- Includes protocol and market context in all metrics
- Provides validator and consensus performance metrics
- Enables granular performance analysis across different markets and protocols
- Supports multi-market performance comparison

## Worker Status Values

| Status | Description |
|--------|-------------|
| `idle` | Worker available for tasks |
| `processing` | Actively processing batch |
| `failed` | Error state requiring attention |

## Consensus Status Values

| Status | Description |
|--------|-------------|
| `initializing` | Consensus process starting |
| `voting` | Collecting validator votes |
| `threshold_reached` | Consensus vote threshold achieved |
| `timeout` | Consensus voting window expired |
| `finalized` | Batch approved and ready |
| `rejected` | Batch did not meet consensus requirements

## Health Indicators

### Healthy Pipeline
- Queue depth < 100 (stream-based monitoring)
- Finalization queue < 10 batches
- Worker heartbeats < 60s old
- Aggregation queue < 5 epochs (stream-based)
- Regular validator batch exchange (>= 2 validators per epoch)
- Consensus determination completing (projects have majority votes)
- IPFS batch retrieval succeeding
- Queue monitoring shows accurate stream lag

### Queue Health Metrics
**Stream-Based Queue**:
- Stream lag < 100 (actual consumer lag)
- Processing rate consistent with throughput
- No unbounded accumulation

### Warning Signs
- Queue depth > 100 (stream-based backlog forming)
- Worker heartbeat > 60s (stale worker)
- Finalization queue > 10 (workers overwhelmed)
- Only 1 validator participating (no aggregation possible)
- High vote divergence (no clear majority for projects)
- IPFS retrieval failures for validator batches
- Stale validator batches (no recent broadcasts)

### Critical Issues
- Queue depth > 1000 (severe backlog)
- No worker heartbeats (workers down)
- Aggregation blocked (parts incomplete)
- No validator batch exchange (network partition)
- Validator batch signature validation failures
- Consistent IPFS failures (storage layer down)

## Integration with Workers

When implementing parallel workers, use the provided monitoring utilities:

```go
import "github.com/powerloom/snapshot-sequencer-validator/pkgs/workers"

// Initialize worker monitor
monitor := workers.NewWorkerMonitor(
    redisClient,
    "worker-1",
    workers.WorkerTypeFinalizer,
    "powerloom-localnet",
)

// Start worker with monitoring
monitor.StartWorker(ctx)

// Track processing
monitor.ProcessingStarted("epoch:123:batch:5")
// ... do work ...
monitor.ProcessingCompleted()

// Handle failures
if err != nil {
    monitor.ProcessingFailed(err)
}
```

## Performance Optimization

### Batch Size Tuning
Adjust `FINALIZATION_BATCH_SIZE` based on:
- Number of projects per epoch
- Available worker capacity
- Network latency requirements

```bash
# For high-volume epochs (500+ projects)
FINALIZATION_BATCH_SIZE=50

# For low-latency requirements
FINALIZATION_BATCH_SIZE=10
```

### Worker Scaling
Monitor queue depths to determine scaling needs:
- Queue > 100: Add more dequeuer workers
- Finalization queue > 10: Add finalizer workers
- Aggregation queue > 5: Optimize aggregator

## Troubleshooting

### No Active Windows
```bash
# Check event monitor is running
./dsv.sh event-logs

# Verify EVENT_START_BLOCK setting
grep EVENT_START_BLOCK .env
```

### Workers Not Visible
Workers only appear when implemented. Current status:
- Dequeuer workers (5 instances)
- Finalizer workers and aggregation worker are not yet implemented

### Queue Backlog
```bash
# Check dequeuer performance
./dsv.sh dequeuer-logs | grep "Processing submission"

# Monitor queue depth trends
curl "http://localhost:9091/api/v1/queues/status" | jq '.aggregation_queue_depth'
```

### Data Format Issues
The system handles both batch and single submission formats:
```bash
# Check for batch format submissions
./dsv.sh dequeuer-logs | grep "P2PSnapshotSubmission"

# Check field names (camelCase format)
./dsv.sh dequeuer-logs | grep "epochId\|projectId"

# Verify conversion strategy
docker exec powerloom-sequencer-validator-dequeuer-1 \
  printenv SUBMISSION_FORMAT_STRATEGY

# Manual format override
# Possible values: 'auto', 'single', 'batch'
SUBMISSION_FORMAT_STRATEGY=single ./dsv.sh distributed
```

### Epoch ID Format Issues
```bash
# Check epoch ID format in logs (should show integers)
./dsv.sh aggregator-logs | grep -E "epoch.*=" | tail -3
# Expected: "epoch=23646205"

# Verify epoch ID formatting in monitor API
curl "http://localhost:9091/api/v1/epochs/timeline" | jq '.epochs[0:2] | .[] | {epoch_id, status}'
# Expected: epoch_id should be integers (23646205)

# Check if FormatEpochID is being used
docker exec <aggregator-container> grep -A5 "FormatEpochID" /app/main.go
```

### Queue Monitoring Accuracy Issues
```bash
# Verify queue monitoring (shows stream lag)
curl "http://localhost:9091/api/v1/queues/status" | jq '.aggregation_queue_depth'
# Expected: Reasonable stream-based lag

# Check if getStreamLag function is being used
docker exec <monitor-container> grep -A10 "getStreamLag" /app/main.go
```

#### Queue Monitoring Troubleshooting:
- **Stream Lag**: Should be reasonable (< 100 for healthy system)
- **Monitor API**: Shows accurate stream-based metrics

#### Conversion Troubleshooting
- **Auto Strategy**: Automatically detects and converts submission formats
- **Single Strategy**: Forces single submission parsing
- **Batch Strategy**: Forces batch submission parsing

##### Common Conversion Mappings
| Field | Format |
|-------|--------|
| `epoch_id`     | `epochId`       |
| `project_id`   | `projectId`     |
| `market_id`    | `marketId`      |
| `submitter`    | `submitterAddress` |

**Warning Signs**:
- Logs showing parsing errors
- Submissions not being processed
- Inconsistent queue depths
- Metrics not being collected

##### Debugging Conversion
1. Check logs for detailed parsing information
2. Verify container environment variables
3. Test with explicit conversion strategy
4. Monitor metrics and queue processing

**Type Conversion Features**:
- Helper functions to handle uint64 → float64 JSON unmarshaling
- FinalizedBatches endpoint extracts:
  - Full IPFS CID
  - ValidatorCount
  - Complete submission details
- Consistent type conversion across API endpoints
- Improved getCurrentEpochInfo to find actual current epoch

### Stale Heartbeats
Indicates worker process issues:
1. Check container health
2. Review worker logs
3. Restart affected workers

## Advanced Monitoring

### Custom Metrics Collection
```bash
# Export metrics to file
docker exec sequencer-container redis-cli --csv HGETALL metrics:* > metrics.csv

# Real-time metric streaming
watch -n 1 'docker exec sequencer-container redis-cli GET metrics:processing_rate'
```

### Batch Tracking Query
```bash
# Find all batches for specific epoch
docker exec sequencer-container redis-cli KEYS "batch:123:*"

# Check specific batch part status
docker exec sequencer-container redis-cli GET "batch:123:part:5:status"
```

## Debugging Tips

### Using Combined Logs

The combined log commands are particularly useful for debugging cross-component issues:

1. **Collection Issues** (`collection-logs`):
   - Track submissions from P2P receipt through window collection
   - Verify dequeuer is storing submissions with correct keys
   - Confirm event monitor is finding stored submissions

2. **Finalization Issues** (`finalization-logs`):
   - Verify batches are pushed to finalization queue
   - Check if finalizers are picking up batches
   - Track consensus selection and batch creation

3. **Full Pipeline Issues** (`pipeline-logs`):
   - End-to-end submission tracking
   - Identify where submissions get stuck
   - Correlate timing between components

### Common Issues and Solutions

| Issue | Check With | Solution |
|-------|------------|----------|
| Submissions not collected | `event-logs` | Verify protocolState matches between components |
| Finalizers idle | `finalizer-logs` | Check finalization queue has batches |
| Missing projects | `aggregator-logs` | Ensure JSON parsing matches data structure |
| Queue buildup | `curl api/v1/queues/status` | Increase worker count or check for errors |

## New Epoch-Centered Endpoints

### GET /api/v1/epochs/{epochId}/submissions

**Purpose**: Get all submissions received for a specific epoch with detailed metadata (slot ID, peer ID, project ID, snapshot CID).

**How It Works**:
1. Queries `{protocol}:{market}:metrics:submissions:timeline` (ZSET) for last 24 hours
2. Filters entity IDs matching the epoch ID
3. For each matching submission:
   - Fetches detailed metadata from `{protocol}:{market}:metrics:submissions:metadata:{entityId}`
   - Falls back to parsing entity ID format if metadata missing
4. Returns array of submissions sorted by timestamp (most recent first)

**Entity ID Formats**:
- **Enhanced**: `received:{epochId}:{slotId}:{projectId}:{timestamp}:{peerId}`
- **Legacy**: `{epochId}-{projectId}-{timestamp}`

**Response Fields**:
- `entity_id`: The entity ID from timeline
- `epoch_id`: Epoch ID
- `slot_id`: Snapshotter slot ID
- `project_id`: Project ID
- `snapshot_cid`: IPFS CID of snapshot (if available)
- `peer_id`: Peer ID that sent the submission (if available)
- `validator_id`: Validator ID (if available)
- `timestamp`: Unix timestamp
- `time`: RFC3339 formatted time

**Example**:
```bash
curl "http://localhost:9091/api/v1/epochs/23875280/submissions?protocol=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401&market=0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d" | jq '.submissions[0:3]'
```

**Use Cases**:
- Debugging: "Which snapshotter sent what for epoch X?"
- Analysis: "How many submissions per slot ID for this epoch?"
- Validation: "Did peer Y send submissions for epoch X?"
- Monitoring: "What projects submitted data for this epoch?"

### GET /api/v1/epochs/active

**Purpose**: Get epochs currently in progress (window open, level 1/2 in progress).

**⚠️ Important Disclaimer**: This endpoint **excludes** epochs that have completed Level 2 finalization (`level2_status == "completed"`) and/or been committed on-chain (`onchain_status == "confirmed"`). The endpoint only returns epochs that are actively processing, not completed ones. To view completed epochs, use `/api/v1/epochs/{epochId}/status` or `/api/v1/epochs/timeline`.

**How It Works** (Fixed Implementation):
1. Queries `{protocol}:{market}:metrics:epochs:timeline` (ZSET) for last 100 epochs
2. For each epoch ID:
   - Queries `{protocol}:{market}:epoch:{epochId}:state` (HASH) for current state
   - Checks `window_status`, `level1_status`, `level2_status`, `onchain_status`
   - Includes epoch if:
     - `window_status == "open"` OR
     - `level1_status == "in_progress"` OR `level1_status == "pending"` OR
     - `level2_status == "collecting"` OR `level2_status == "aggregating"` OR `level2_status == "pending"` OR
     - `onchain_status == "queued"` OR `onchain_status == "submitted"`
   - **Excludes** epochs where:
     - `level2_status == "completed"` (Level 2 finalized)
     - `onchain_status == "confirmed"` (committed on-chain)
3. Returns filtered epochs sorted by epoch ID (descending)

**Why This Approach**:
- **Reliability**: Queries epoch state directly instead of relying on potentially stale `epochs:active` SET
- **Accuracy**: Always reflects current state from authoritative source (epoch state hash)
- **Consistency**: Same data source as `/epochs/{epochId}/status` endpoint

**Response Fields**:
- `epoch_id`: Epoch ID
- `phase`: Current phase (submission, level1_finalization, level2_aggregation, etc.)
- `status`: Window status (open, closed)
- `start_time`: Window open timestamp
- `duration`: Window duration in seconds
- `level1_batch_exists`: Whether Level 1 batch exists
- `level2_batch_exists`: Whether Level 2 batch exists

**Example**:
```bash
curl "http://localhost:9091/api/v1/epochs/active?protocol=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401&market=0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d" | jq '.[] | {epoch_id, phase, status}'
```

**Use Cases**:
- Real-time monitoring: "What epochs are currently being processed?"
- Debugging: "Is epoch X stuck in a phase?"
- Operations: "Which epochs need attention?"

### GET /api/v1/epochs/{epochId}/status

Get complete epoch state with all phase information.

**⚠️ IMPORTANT**: You **MUST** pass `protocol` and `market` query parameters to see on-chain submission details written by relayer-py. Relayer-py writes using the protocol state contract address (from `PROTOCOL_STATE_CONTRACT` env var) and the data market address from the transaction. If you don't pass these parameters, the endpoint uses default addresses and won't find the relayer-py data.

**Example:**
```bash
# CORRECT - Pass protocol and market to match relayer-py's Redis keys
curl "http://localhost:9091/api/v1/epochs/23847425/status?protocol=0xC9e7304f719D35919b0371d8B242ab59E0966d63&market=0xb6c1392944a335b72b9e34f9D4b8c0050cdb511f"

# WRONG - Missing query params, won't find relayer-py transaction data
curl "http://localhost:9091/api/v1/epochs/23847425/status"
```

**Query Parameters**:
- `protocol` (REQUIRED for relayer-py data): Protocol state contract address (must match `PROTOCOL_STATE_CONTRACT` in relayer-py)
- `market` (REQUIRED for relayer-py data): Data market address (must match the data market address in the transaction)

**Response:**
```json
{
  "epoch_id": "23847425",
  "state": {
    "window_status": "closed",
    "window_opened_at": 1763798270,
    "window_closes_at": 1763798290,
    "phase": "onchain_submission",
    "submissions_count": 45,
    "level1_status": "completed",
    "level1_completed_at": 1763798300,
    "level2_status": "completed",
    "level2_completed_at": 1763798400,
    "onchain_status": "confirmed",
    "onchain_tx_hash": "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "onchain_block_number": 12345678,
    "onchain_submitted_at": 1763798500,
    "priority": 1,
    "vpa_submission_attempted": true,
    "last_updated": 1763798600
  },
  "timestamp": "2025-01-22T08:00:00Z"
}
```

**Note**: The `onchain_status`, `onchain_tx_hash`, `onchain_block_number`, and `onchain_submitted_at` fields are automatically updated by relayer-py when transactions are submitted and confirmed. No log parsing needed!

### GET /api/v1/epochs/active

Get epochs currently in progress (window open, level 1/2 in progress).

**⚠️ Important Disclaimer**: This endpoint **excludes** epochs that have completed Level 2 finalization (`level2_status == "completed"`) and/or been committed on-chain (`onchain_status == "confirmed"`). The endpoint only returns epochs that are actively processing, not completed ones. To view completed epochs, use `/api/v1/epochs/{epochId}/status` or `/api/v1/epochs/timeline`.

**Example:**
```bash
curl "http://localhost:9091/api/v1/epochs/active?protocol=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401&market=0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d"
```

**Response:**
```json
[
  {
    "epoch_id": "23847425",
    "phase": "level2_aggregation",
    "status": "closed",
    "start_time": 1763798270,
    "duration": 20,
    "level1_batch": true,
    "level2_batch": false
  }
]
```

### GET /api/v1/epochs/gaps

Identify epochs that should have finalizations but don't.

**Query Parameters:**
- `window_minutes`: Time window to check for gaps (default: 5)

**Example:**
```bash
curl "http://localhost:9091/api/v1/epochs/gaps?window_minutes=5&protocol=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401&market=0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d"
```

**Response:**
```json
{
  "gaps": [
    {
      "epoch_id": "23847420",
      "gap_type": "missing_level1",
      "diagnostic": "Window closed but Level 1 finalization not started",
      "state": {
        "window_status": "closed",
        "level1_status": "pending",
        "phase": "level1_finalization"
      }
    }
  ],
  "count": 1,
  "timestamp": "2025-01-22T08:00:00Z"
}
```

## Future Enhancements

Potential monitoring improvements:
- HTTP metrics endpoint (`/metrics`)
- Prometheus integration
- Grafana dashboards
- WebSocket real-time updates
- Historical trend analysis
- Alert system for persistent gaps
- ~~On-chain transaction confirmation tracking~~ ✅ **COMPLETE**: Relayer-py now writes transaction status to Redis automatically

### Consensus Troubleshooting

#### No Validator Batches Received
Check the aggregation results endpoint to see if validator batches are being received and processed:

```bash
# Check aggregation results for recent validator participation
curl "http://localhost:9091/api/v1/aggregation/results"

# Check Redis for validator presence in recent epochs
docker exec <container> redis-cli SMEMBERS "epoch:12345:validators"

# Check P2P Gateway logs for connectivity
./dsv.sh p2p-logs | grep -E "Connected|peer"
```

#### High Vote Divergence
Monitor the aggregation results to see vote distribution:

```bash
# Check aggregation results for vote distribution
curl "http://localhost:9091/api/v1/aggregation/results" | jq '.results[]'

# Look for projects with inconsistent CID votes across epochs
curl "http://localhost:9091/api/v1/batches/finalized" | jq '.batches[] | {epoch_id, projects}'
```

#### Validator Batches Not Aggregating
Check if the minimum validator threshold is being met:

```bash
# Check aggregation results for validator count
curl "http://localhost:9091/api/v1/aggregation/results"

# Check Redis for validator presence
docker exec <container> redis-cli SMEMBERS "epoch:12345:validators"

# Check aggregator logs for processing issues
./dsv.sh aggregator-logs | grep "AGGREGATION"
```

#### IPFS Batch Retrieval Failures
Check IPFS connectivity and retrieval:

```bash
# Check IPFS logs for connectivity issues
./dsv.sh ipfs-logs

# Check IPFS client configuration
docker exec <container> printenv | grep IPFS

# Test IPFS connectivity
docker exec <container> curl http://localhost:5001/api/v0/version
```

#### Network Issues
Monitor P2P connectivity and message passing:

```bash
# Check P2P Gateway logs for connectivity
./dsv.sh p2p-logs | grep -E "Connected|peer"

# Check gossipsub topic subscriptions
./dsv.sh p2p-logs | grep "gossipsub"

# Monitor aggregation processing
./dsv.sh aggregator-logs | grep "AGGREGATION"
```

### Consensus Monitoring Features
- Detailed validator performance tracking
- Stake-weighted voting visualization
- Cross-epoch validator reputation system
- Advanced consensus game theory modeling
- Automated validator onboarding/offboarding tools

## Related Documentation

- [Entity ID Reference Guide](../../specs/ENTITY_ID_REFERENCE_GUIDE.md) - Comprehensive entity ID format reference and usage guide
- [PHASE_2_CURRENT_WORK.md](../../PHASE_2_CURRENT_WORK.md) - Architecture overview
- [PROJECT_STATUS.md](../../PROJECT_STATUS.md) - Implementation status

## Entity ID Understanding

For detailed information about entity ID formats, metadata, and how to interpret monitoring data, see the [Entity ID Reference Guide](../../specs/ENTITY_ID_REFERENCE_GUIDE.md). This comprehensive guide explains:

- All entity ID formats (enhanced and legacy)
- Metadata extraction and usage
- Entity type descriptions and purposes
- Practical examples and parsing guides
- Troubleshooting and best practices

### Key Entity ID Types
- **Submissions**: `received:{epoch}:{slot}:{project}:{timestamp}:{peer}` - Format with rich context
- **Validations**: `{epoch}-baseSnapshot:{validator}:{market}:{slot}-{project}:{ipfs}` - Validator contributions
- **Batch Events**: `peer_discovery:{count}:{timestamp}`, `local:{epoch}`, `validator:{validator}:{epoch}`, `aggregated:{epoch}`
- **Epoch Events**: `open:{epoch}`, `close:{epoch}` - Window lifecycle events

## VPA Integration Monitoring

### Overview
The Validator Priority Assigner (VPA) integration enables priority-based batch submission to on-chain contracts. This section provides comprehensive monitoring guidance for understanding your node's priority assignments and submission status.

### Priority-Based Submission Logic

**Key Concept**: Priority 2+ validators only submit if Priority 1 (and all lower priorities) failed to submit within their windows. This prevents duplicate submissions.

**Submission Flow:**
1. **Priority Check**: Validator checks if they have priority > 0 for the epoch
2. **Window Wait**: Validator waits for their submission window to open
3. **Duplicate Check** (Priority 2+ only): Checks if higher priority validator already submitted
4. **Submission**: If no duplicate found, submits via relayer-py

### Monitor API Endpoints

The Monitor API provides dedicated endpoints for VPA monitoring, eliminating the need for log grepping:

#### GET /api/v1/vpa/epoch/{epochID}

Get priority assignment and submission status for a specific epoch.

**Example:**
```bash
# Get VPA status for epoch 23847425
curl "http://localhost:9091/api/v1/vpa/epoch/23847425?protocol=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401&market=0xb6c1392944a335b72b9e34f9D4b8c0050cdb511f"
```

**Response:**
```json
{
  "epoch_id": "23847425",
  "status": {
    "epoch_id": "23847425",
    "priority": 1,
    "priority_status": "assigned",
    "submission_success": true,
    "submission_tx_hash": "",
    "submission_timestamp": 1763798276,
    "data_market": "0xb6c1392944a335b72b9e34f9D4b8c0050cdb511f",
    "validator": "0x164446B87AbbC0C1A389bD9bAF7F5aE813Cf6aB8",
    "priority_details": {
      "epoch_id": "23847425",
      "priority": 1,
      "status": "assigned",
      "timestamp": 1763798270
    },
    "submission_details": {
      "epoch_id": "23847425",
      "priority": 1,
      "success": true,
      "tx_hash": "",
      "timestamp": 1763798276
    }
  },
  "timestamp": "2025-11-22T08:00:00Z"
}
```

#### GET /api/v1/vpa/timeline

Get priority assignment and submission timeline.

**Query Parameters:**
- `type`: `priority`, `submission`, or `both` (default: `both`)
- `limit`: Number of entries (default: 50, max: 1000)
- `protocol`: Protocol state identifier (optional)
- `market`: Data market address (optional)

**Example:**
```bash
# Get last 20 priority assignments
curl "http://localhost:9091/api/v1/vpa/timeline?type=priority&limit=20"

# Get last 50 submissions
curl "http://localhost:9091/api/v1/vpa/timeline?type=submission&limit=50"

# Get both timelines
curl "http://localhost:9091/api/v1/vpa/timeline?limit=100"
```

**Response:**
```json
{
  "timeline": {
    "priority_timeline": [
      {
        "epoch_id": "23847425",
        "priority": "1",
        "status": "assigned",
        "timestamp": 1763798270
      }
    ],
    "submission_timeline": [
      {
        "epoch_id": "23847425",
        "priority": "1",
        "status": "success",
        "timestamp": 1763798276
      }
    ]
  },
  "timestamp": "2025-11-22T08:00:00Z"
}
```

#### GET /api/v1/vpa/stats

Get aggregated VPA statistics (priority assignments, submissions, success rates).

**Example:**
```bash
curl "http://localhost:9091/api/v1/vpa/stats?protocol=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401&market=0xb6c1392944a335b72b9e34f9D4b8c0050cdb511f"
```

**Response:**
```json
{
  "stats": {
    "total_priority_assignments": 150,
    "priority_1_count": 50,
    "priority_2_count": 50,
    "priority_3_count": 50,
    "no_priority_count": 10,
    "total_submissions_success": 140,
    "total_submissions_failed": 10,
    "priority_1_submissions_success": 48,
    "priority_2_submissions_success": 47,
    "priority_3_submissions_success": 45,
    "submission_success_rate": 93.33,
    "priority_assignments_24h": 25,
    "submissions_24h": 23
  },
  "timestamp": "2025-11-22T08:00:00Z"
}
```

#### Dashboard Summary (includes VPA metrics)

The `/api/v1/dashboard/summary` endpoint now includes VPA metrics in the `vpa_metrics` section:

**Example:**
```bash
curl "http://localhost:9091/api/v1/dashboard/summary"
```

**Response includes:**
```json
{
  "system_metrics": {
    "vpa_priority_assignments_total": 150,
    "vpa_submissions_success": 140,
    "vpa_submissions_failed": 10,
    "vpa_submission_success_rate": 93.33,
    "vpa_priority_assignments_24h": 25,
    "vpa_submissions_24h": 23
  },
  "vpa_metrics": {
    "priority_assignments_total": 150,
    "submissions_success": 140,
    "submissions_failed": 10,
    "submission_success_rate": 93.33,
    "priority_assignments_24h": 25,
    "submissions_24h": 23
  }
}
```

### Log-Based Monitoring (Fallback)

If Monitor API is unavailable, you can still use log grepping as a fallback:

**Priority Assignment:**
```bash
# Check if your node has priority for recent epochs
./dsv.sh aggregator-logs | grep -E "(priority|Priority)" | tail -20

# Monitor priority assignment results
./dsv.sh aggregator-logs | grep -E "No VPA priority|priority assigned"

# Check priority values (1, 2, 3, etc.)
./dsv.sh aggregator-logs | grep "priority=" | tail -10
```

**Submission Flow:**
```bash
# Track complete submission flow
./dsv.sh aggregator-logs | grep -E "(⏳|✅|⏭️|🚀)" | tail -30

# Check if waiting for submission window
./dsv.sh aggregator-logs | grep "⏳ Waiting for submission window"

# Check if submission window opened
./dsv.sh aggregator-logs | grep "✅ Submission window is open"

# Check if skipped due to higher priority submission
./dsv.sh aggregator-logs | grep "⏭️.*Epoch already has a submission"

# Check successful submissions (queued to relayer)
./dsv.sh aggregator-logs | grep "✅ VPA batch submission queued successfully"
```

**Key Log Patterns:**
- `No VPA priority assigned, skipping new contract submission` - Your node has priority 0 (no submission)
- `priority=1` - Priority 1 (first to submit)
- `priority=2` - Priority 2 (backup, only submits if Priority 1 fails)
- `priority=3+` - Lower priority (only submits if all higher priorities fail)
- `⏳ Waiting for submission window to open...` - Waiting for your priority window
- `✅ Submission window is open, checking if submission already exists...` - Window opened, checking duplicates
- `⏭️ Epoch already has a submission from a higher priority validator, skipping submission` - Priority 2+ skipped (Priority 1 already submitted)
- `✅ No existing submission found, proceeding with submission` - No duplicate found, proceeding
- `🚀 Submitting batch via relayer-py` - Sending to relayer service
- `✅ VPA batch submission queued successfully - relayer processing asynchronously` - Submission queued (relayer processes async)
- `✅ Sent batch size to relayer` - Batch size notification sent

### Redis Key Structure (For Direct Access)

**Note**: Direct Redis access is not recommended. Use Monitor API endpoints instead.

The aggregator stores priority and submission metrics in Redis with namespaced keys:

- `{protocol}:{market}:vpa:priority:{epochID}` - Priority assignment per epoch (7-day TTL)
- `{protocol}:{market}:vpa:submission:{epochID}` - Submission result per epoch (7-day TTL)
- `{protocol}:{market}:vpa:epoch:{epochID}:status` - Combined epoch status (priority + submission)
- `{protocol}:{market}:vpa:priority:timeline` - Priority assignment timeline (ZSET)
- `{protocol}:{market}:vpa:submission:timeline` - Submission timeline (ZSET)
- `{protocol}:{market}:vpa:stats` - Statistics hash (7-day TTL)
- `{protocol}:vpa:priority:timeline` - Historical priority assignments (sorted set)
- `{protocol}:vpa:submission:timeline` - Historical submission attempts (sorted set)
- `{protocol}:vpa:stats:{dataMarket}` - Aggregated statistics (7-day TTL)

### Understanding Your Node's Priority Status

**Quick Status Check:**
```bash
# Check recent priority assignments
./dsv.sh aggregator-logs | grep -E "priority=" | tail -10 | awk '{print $NF}'

# Count priority assignments by value
./dsv.sh aggregator-logs | grep "priority=" | grep -o "priority=[0-9]*" | sort | uniq -c

# Check submission success rate (queued successfully)
./dsv.sh aggregator-logs | grep "✅ VPA batch submission queued successfully" | wc -l

# Verify actual on-chain transactions in relayer logs
docker logs dsv-relayer-py --tail 100 | grep "Transaction.*submitted with hash" | wc -l
```

**Priority Status Interpretation:**
- **Priority 0**: No priority assigned - your node will not submit for this epoch
- **Priority 1**: Primary submitter - your node submits first (if window opens)
- **Priority 2+**: Backup submitter - your node only submits if Priority 1 fails

### Monitoring Submission Windows

**Window Timing:**
```bash
# Check if waiting for window
./dsv.sh aggregator-logs | grep "⏳ Waiting for submission window"

# Check window timeout warnings
./dsv.sh aggregator-logs | grep "Timeout waiting for submission window"

# Check window opened successfully
./dsv.sh aggregator-logs | grep "✅ Submission window is open"
```

**Window Status:**
- Window opens based on: `epochReleaseTime + preSubmissionWindow + (priority-1) * pNSubmissionWindow`
- Priority 1 window: `epochReleaseTime + preSubmissionWindow` to `epochReleaseTime + preSubmissionWindow + p1SubmissionWindow`
- Priority 2+ windows: Sequential windows after Priority 1 closes

### Monitoring Duplicate Submission Checks

**Priority 2+ Behavior:**
```bash
# Check if duplicate check occurred
./dsv.sh aggregator-logs | grep "checking if submission already exists"

# Check if skipped due to existing submission
./dsv.sh aggregator-logs | grep "⏭️.*Epoch already has a submission"

# Check on-chain event log queries
./dsv.sh aggregator-logs | grep "Checked BatchSubmissionsCompleted event logs"
```

**Duplicate Check Logic:**
- Priority 1: No duplicate check (submits first)
- Priority 2+: Queries `BatchSubmissionsCompleted` event logs
- If event found: Skips submission (Priority 1 already completed)
- If no event: Proceeds with submission

### Relayer-PY Integration Monitoring

**Relayer-Py State Tracking:**

Relayer-py now writes epoch state directly to Redis, providing seamless integration with DSV monitoring. Transaction status is automatically updated in the epoch state hash (`{protocol}:{market}:epoch:{epochId}:state`) at two points:

1. **Transaction Submitted**: `onchain_status: "submitted"`, `onchain_tx_hash` set, `onchain_submitted_at` timestamp
2. **Transaction Confirmed**: `onchain_status: "confirmed"`, `onchain_block_number` set (from receipt)

**Query Transaction Status via Monitor API:**
```bash
# Get complete epoch status including transaction details
EPOCH_ID=23847425
curl "http://localhost:9091/api/v1/epochs/${EPOCH_ID}/status?protocol=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401&market=0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d" | jq '.state | {onchain_status, onchain_tx_hash, onchain_block_number, onchain_submitted_at}'

# Check finalized batches with on-chain status
curl "http://localhost:9091/api/v1/batches/finalized?epoch_id=${EPOCH_ID}" | jq '.batches[] | {epoch_id, onchain_status, onchain_tx_hash, onchain_block_number}'
```

**Relayer Service Logs (Fallback):**
```bash
# Monitor relayer endpoint calls
./dsv.sh aggregator-logs | grep "🚀 Submitting batch via relayer-py"

# Check relayer response
./dsv.sh aggregator-logs | grep -E "VPA batch submission|relayer returned"

# Monitor relayer-py service logs (for detailed transaction processing)
docker logs dsv-relayer-py --tail 50 | grep -E "(submitSubmissionBatch|submitBatchSize|Transaction.*submitted with hash)"
```

**Relayer Flow:**
1. `submitBatchSize` - Informs relayer of expected batch count (typically 1 for DSV)
2. `submitSubmissionBatch` - Submits the actual batch data
3. Relayer processes and submits on-chain transaction
4. **Relayer-py writes to Redis**: Updates epoch state with `onchain_status: "submitted"` and `onchain_tx_hash`
5. Transaction receipt received: Updates epoch state with `onchain_status: "confirmed"` and `onchain_block_number`

### End-to-End Submission Tracking

**Complete Flow Monitoring:**
```bash
# Track complete submission flow for specific epoch
EPOCH_ID=23847425
./dsv.sh aggregator-logs | grep "$EPOCH_ID" | grep -E "(priority|⏳|✅|⏭️|🚀)"
```

**Expected Flow (Priority 1):**
1. `priority=1` - Priority assigned
2. `⏳ Waiting for submission window to open...` - Waiting for P1 window
3. `✅ Submission window is open, checking if submission already exists...` - Window opened
4. `✅ No existing submission found, proceeding with submission` - No duplicate
5. `✅ Sent batch size to relayer` - Batch size notification sent
6. `🚀 Submitting batch via relayer-py` - Sending to relayer
7. `✅ VPA batch submission queued successfully - relayer processing asynchronously` - Queued
8. **Relayer-py updates Redis**: `onchain_status: "submitted"`, `onchain_tx_hash` set
9. **Transaction confirmed**: `onchain_status: "confirmed"`, `onchain_block_number` set (query via `/api/v1/epochs/{epochId}/status`)

**Expected Flow (Priority 2+):**
1. `priority=2` (or higher) - Priority assigned
2. `⏳ Waiting for submission window to open...` - Waiting for P2+ window
3. `✅ Submission window is open, checking if submission already exists...` - Window opened
4. Either:
   - `⏭️ Epoch already has a submission from a higher priority validator, skipping submission` - Priority 1 already submitted
   - `✅ No existing submission found, proceeding with submission` - Priority 1 failed, proceeding
5. `✅ Sent batch size to relayer` - Batch size notification sent
6. `🚀 Submitting batch via relayer-py` - Sending to relayer (if no duplicate)
7. `✅ VPA batch submission queued successfully - relayer processing asynchronously` - Queued
8. **Relayer-py updates Redis**: `onchain_status: "submitted"`, `onchain_tx_hash` set (if submitted)
9. **Transaction confirmed**: `onchain_status: "confirmed"`, `onchain_block_number` set (query via `/api/v1/epochs/{epochId}/status`)

### VPA Health Indicators

**Healthy VPA Integration:**
- Regular priority assignments (check logs for `priority=` entries)
- Priority distribution: Mix of Priority 0, 1, 2+ over time
- Successful submissions when Priority 1 assigned (query epoch status endpoint for `onchain_status: "confirmed"`)
- Priority 2+ correctly skipping when Priority 1 submits
- No timeout errors waiting for submission windows
- Relayer service responding successfully (200 OK responses)
- Epoch state shows `onchain_status` progressing: "queued" → "submitted" → "confirmed"
- Transaction hashes and block numbers available in epoch state (no log parsing needed)

**Warning Signs:**
- No priority assignments for multiple epochs (check VPA client initialization)
- All epochs showing Priority 0 (validator not registered in VPA)
- Frequent "Timeout waiting for submission window" (window timing issues)
- Priority 2+ submitting when Priority 1 already submitted (duplicate check failing)
- Relayer service errors or non-200 responses

**Critical Issues:**
- VPA client not initialized (check `VPA_VALIDATOR_ADDRESS` configuration)
- Consistent relayer transaction failures
- Submission window timeouts for all epochs
- No priority assignments for extended periods

### Querying Priority History

**Redis-Based Queries:**
```bash
# Get priority for last 10 epochs
docker exec redis redis-cli ZREVRANGE "{protocol}:vpa:priority:timeline" 0 9

# Get submission results for last 10 epochs
docker exec redis redis-cli ZREVRANGE "{protocol}:vpa:submission:timeline" 0 9

# Check specific epoch priority
EPOCH_ID=23847425
docker exec redis redis-cli GET "{protocol}:vpa:priority:{dataMarket}:${EPOCH_ID}"

# Check specific epoch submission
docker exec redis redis-cli GET "{protocol}:vpa:submission:{dataMarket}:${EPOCH_ID}"
```

**Statistics Summary:**
```bash
# Get priority statistics
docker exec redis redis-cli HGETALL "{protocol}:vpa:stats:{dataMarket}"

# Expected fields:
# - total_priority_assignments: Total epochs with priority > 0
# - priority_1_count: Number of Priority 1 assignments
# - priority_2_count: Number of Priority 2 assignments
# - no_priority_count: Number of Priority 0 epochs
# - total_submissions_success: Successful submissions
# - total_submissions_failed: Failed submission attempts
# - priority_1_submissions_success: Priority 1 successful submissions
# - priority_2_submissions_success: Priority 2 successful submissions
```

### VPA Configuration Verification

**Required Configuration:**
```bash
# Check VPA client initialization
./dsv.sh aggregator-logs | grep -E "(VPA client|VPA caching client initialized)"

# Verify VPA validator address
docker exec <aggregator-container> printenv | grep VPA_VALIDATOR_ADDRESS

# Check VPA contract address
docker exec <aggregator-container> printenv | grep VPA_CONTRACT_ADDRESS

# Verify relayer endpoint
docker exec <aggregator-container> printenv | grep RELAYER_PY_ENDPOINT
```

**Configuration Checklist:**
- `VPA_VALIDATOR_ADDRESS` - Your validator's Ethereum address
- `VPA_CONTRACT_ADDRESS` - VPA contract address (or auto-fetched from ProtocolState)
- `PROTOCOL_STATE_CONTRACT` - ProtocolState contract address
- `RELAYER_PY_ENDPOINT` - Relayer service endpoint (e.g., `http://relayer-py:8080`)
- `ENABLE_ONCHAIN_SUBMISSION=true` - Enable VPA submissions

### Troubleshooting Priority Issues

**No Priority Assignments:**
```bash
# Check VPA client initialization
./dsv.sh aggregator-logs | grep "VPA client not initialized"

# Verify validator address is registered in ValidatorState contract
# Check on-chain: ValidatorState.getNodeIdForValidator(validatorAddress)

# Check VPA contract connectivity
./dsv.sh aggregator-logs | grep "Failed to get VPA priority"
```

**Priority 2+ Submitting When Should Skip:**
```bash
# Check duplicate detection logs
./dsv.sh aggregator-logs | grep "Checked BatchSubmissionsCompleted event logs"

# Verify on-chain event query is working
./dsv.sh aggregator-logs | grep "has_submission"

# Check if RPC client is initialized
./dsv.sh aggregator-logs | grep "RPC client initialized for on-chain submission checks"
```

**Submission Window Timeouts:**
```bash
# Check window wait logs
./dsv.sh aggregator-logs | grep "Timeout waiting for submission window"

# Verify epoch release timing
./dsv.sh event-logs | grep "Epoch.*released"

# Check if epoch-manager releases epochs to both contracts simultaneously
```

### Recommended Monitoring Queries

**Daily Priority Summary:**
```bash
# Count priority assignments by value for today
./dsv.sh aggregator-logs | grep "$(date +%Y-%m-%d)" | grep "priority=" | \
  grep -o "priority=[0-9]*" | sort | uniq -c

# Count successful submissions by priority
./dsv.sh aggregator-logs | grep "$(date +%Y-%m-%d)" | \
  grep "✅ VPA batch submission successful" | grep -o "priority=[0-9]*" | sort | uniq -c
```

**Submission Success Rate:**
```bash
# Calculate success rate
TOTAL=$(./dsv.sh aggregator-logs | grep "🚀 Submitting batch via relayer-py" | wc -l)
SUCCESS=$(./dsv.sh aggregator-logs | grep "✅ VPA batch submission successful" | wc -l)
echo "Success rate: $(( SUCCESS * 100 / TOTAL ))%"
```

**Priority Distribution:**
```bash
# View priority distribution over last 100 epochs
./dsv.sh aggregator-logs | grep "priority=" | tail -100 | \
  grep -o "priority=[0-9]*" | cut -d= -f2 | sort -n | uniq -c
```

### Quick Reference: Understanding Your Node's Status

**For each epoch, your node can be in one of these states:**

1. **Priority 0 (No Priority)**
   - Log: `ℹ️ No VPA priority assigned (Priority 0), skipping new contract submission`
   - Meaning: Your validator was not assigned priority for this epoch
   - Action: Normal - priority rotates across validators
   - Redis: `{protocol}:vpa:priority:{market}:{epoch}` with `status: "no_priority"`

2. **Priority 1 (Primary Submitter)**
   - Log: `🎯 VPA Priority assigned for epoch` with `priority=1`
   - Meaning: Your node is the primary submitter for this epoch
   - Expected Flow: Wait for window → Submit → Success
   - Redis: `{protocol}:vpa:priority:{market}:{epoch}` with `priority: 1`

3. **Priority 2+ (Backup Submitter)**
   - Log: `🎯 VPA Priority assigned for epoch` with `priority=2` (or higher)
   - Meaning: Your node is a backup submitter
   - Expected Flow: Wait for window → Check if Priority 1 submitted → Either skip or submit
   - Redis: `{protocol}:vpa:priority:{market}:{epoch}` with `priority: 2+`

**Submission Outcomes:**

- **Success (Queued)**: `✅ VPA batch submission queued successfully - relayer processing asynchronously`
  - Redis: `{protocol}:vpa:submission:{market}:{epoch}` with `success: true`
  - **Epoch State**: `{protocol}:{market}:epoch:{epochId}:state` updated by relayer-py:
    - `onchain_status: "submitted"` (when transaction submitted)
    - `onchain_tx_hash` set (transaction hash)
    - `onchain_status: "confirmed"` (when receipt received)
    - `onchain_block_number` set (block number from receipt)
  - **Query via API**: Use `/api/v1/epochs/{epochId}/status` to get complete transaction status (no log parsing needed)
  
- **Skipped (Priority 2+)**: `⏭️ Epoch already has a submission from a higher priority validator, skipping submission`
  - Redis: `{protocol}:vpa:submission:{market}:{epoch}` with `success: false` (no tx_hash)
  - Redis: `{protocol}:vpa:priority:{market}:{epoch}` with `status: "skipped_higher_priority_submitted"`
  
- **Window Timeout**: `⏰ Timeout waiting for submission window (10min), skipping submission`
  - Redis: `{protocol}:vpa:priority:{market}:{epoch}` with `status: "window_timeout"`
  
- **Relayer Error**: `❌ VPA relayer returned non-200 status` or `❌ Failed to submit batch to relayer-py`
  - Redis: `{protocol}:vpa:submission:{market}:{epoch}` with `success: false`

### Node Operator Checklist

**Daily Monitoring:**
```bash
# 1. Check priority distribution (should see mix of 0, 1, 2+)
./dsv.sh aggregator-logs | grep "priority=" | tail -50 | grep -o "priority=[0-9]*" | sort | uniq -c

# 2. Check submission success rate
SUCCESS=$(./dsv.sh aggregator-logs | grep "✅ VPA batch submission successful" | wc -l)
ATTEMPTS=$(./dsv.sh aggregator-logs | grep "🚀 Submitting batch via relayer-py" | wc -l)
echo "Success rate: $(( SUCCESS * 100 / ATTEMPTS ))%"

# 3. Check for Priority 1 assignments
./dsv.sh aggregator-logs | grep "priority=1" | wc -l

# 4. Check for skipped submissions (Priority 2+)
./dsv.sh aggregator-logs | grep "⏭️.*Epoch already has a submission" | wc -l

# 5. Check for timeouts
./dsv.sh aggregator-logs | grep "⏰ Timeout waiting for submission window" | wc -l
```

**Weekly Analysis:**
```bash
# Get priority statistics from Redis
docker exec redis redis-cli HGETALL "{protocol}:vpa:stats:{dataMarket}"

# View priority timeline for last week
docker exec redis redis-cli ZREVRANGEBYSCORE "{protocol}:vpa:priority:timeline" \
  $(date -d '7 days ago' +%s) +inf

# View submission timeline for last week
docker exec redis redis-cli ZREVRANGEBYSCORE "{protocol}:vpa:submission:timeline" \
  $(date -d '7 days ago' +%s) +inf
```

### Additional Tracking Recommendations

**What's Currently Tracked:**
- ✅ Priority assignments per epoch (Redis: `{protocol}:vpa:priority:{market}:{epoch}`)
- ✅ Submission attempts and results (Redis: `{protocol}:vpa:submission:{market}:{epoch}`)
- ✅ Priority timeline (Redis sorted set)
- ✅ Submission timeline (Redis sorted set)
- ✅ Aggregated statistics (success/failure counts by priority)

**Recommended Additional Tracking:**

1. **Window Timing Metrics:**
   - Track when submission windows open/close per priority
   - Monitor average wait time for window opening
   - Alert on frequent window timeouts

2. **Priority Distribution Analysis:**
   - Track priority assignment frequency over time
   - Monitor if your node consistently gets Priority 0 (indicates registration issue)
   - Track priority fairness (should be distributed over time)

3. **Submission Success Rate by Priority:**
   - Track success rate separately for Priority 1 vs Priority 2+
   - Monitor if Priority 2+ submissions are frequently skipped (indicates healthy Priority 1)
   - Track relayer response times by priority

4. **Epoch-Level Summary:**
   - Store complete epoch submission status (priority, window opened, submitted, skipped reason)
   - Enable quick lookup: "Did I submit epoch X? What was my priority?"
   - Track epochs where Priority 1 failed and Priority 2+ stepped in

5. **On-Chain Verification:**
   - Periodically verify on-chain that submissions actually occurred
   - Cross-reference Redis tracking with on-chain `BatchSubmissionsCompleted` events
   - Detect discrepancies between Redis tracking and on-chain state

**Example Enhanced Queries:**
```bash
# Check your node's priority history for last 24 hours
docker exec redis redis-cli ZREVRANGEBYSCORE "{protocol}:vpa:priority:timeline" \
  $(date -d '24 hours ago' +%s) +inf

# Find epochs where you had Priority 1 but didn't submit
# (Compare priority timeline with submission timeline)

# Check submission success rate by priority
docker exec redis redis-cli HGETALL "{protocol}:vpa:stats:{dataMarket}" | \
  grep -E "priority_[0-9]+_submissions"
```

### Additional Tracking Recommendations

**What's Currently Tracked:**
- ✅ Priority assignments per epoch (Redis: `{protocol}:vpa:priority:{market}:{epoch}`)
- ✅ Submission attempts and results (Redis: `{protocol}:vpa:submission:{market}:{epoch}`)
- ✅ Priority timeline (Redis sorted set)
- ✅ Submission timeline (Redis sorted set)
- ✅ Aggregated statistics (success/failure counts by priority)

**Recommended Additional Tracking:**

1. **Window Timing Metrics:**
   - Track when submission windows open/close per priority
   - Monitor average wait time for window opening
   - Alert on frequent window timeouts

2. **Priority Distribution Analysis:**
   - Track priority assignment frequency over time
   - Monitor if your node consistently gets Priority 0 (indicates registration issue)
   - Track priority fairness (should be distributed over time)

3. **Submission Success Rate by Priority:**
   - Track success rate separately for Priority 1 vs Priority 2+
   - Monitor if Priority 2+ submissions are frequently skipped (indicates healthy Priority 1)
   - Track relayer response times by priority

4. **Epoch-Level Summary:**
   - Store complete epoch submission status (priority, window opened, submitted, skipped reason)
   - Enable quick lookup: "Did I submit epoch X? What was my priority?"
   - Track epochs where Priority 1 failed and Priority 2+ stepped in

5. **On-Chain Verification:**
   - Periodically verify on-chain that submissions actually occurred
   - Cross-reference Redis tracking with on-chain `BatchSubmissionsCompleted` events
   - Detect discrepancies between Redis tracking and on-chain state

**Example Enhanced Queries:**
```bash
# Check your node's priority history for last 24 hours
docker exec redis redis-cli ZREVRANGEBYSCORE "{protocol}:vpa:priority:timeline" \
  $(date -d '24 hours ago' +%s) +inf

# Find epochs where you had Priority 1 but didn't submit
# (Compare priority timeline with submission timeline)

# Check submission success rate by priority
docker exec redis redis-cli HGETALL "{protocol}:vpa:stats:{dataMarket}" | \
  grep -E "priority_[0-9]+_submissions"
```
## Automated Epoch Status Analysis

The DSV repository includes automated analysis scripts to verify transaction hash presence and identify issues with epoch processing.

### analyze_epoch_status.py

**Purpose**: Automated analysis of active epochs to verify transaction hash presence and identify processing issues.

**Key Features**:
- Compares active epochs (queried with OLD/default addresses) with their detailed status (queried with NEW addresses where relayer-py writes)
- Automatically lowercases addresses to match Redis key format (relayer-py lowercases addresses)
- Tracks transaction hash presence by on-chain status
- Identifies epochs with missing transaction hashes
- Reports failed submissions with error details

**Usage**:
```bash
# Basic usage (requires NEW addresses where relayer-py writes)
python3 scripts/analyze_epoch_status.py \
  --status-protocol 0xC9e7304f719D35919b0371d8B242ab59E0966d63 \
  --status-market 0xb6c1392944a335b72b9e34f9D4b8c0050cdb511f

# Full usage with both OLD and NEW addresses
python3 scripts/analyze_epoch_status.py \
  --api-url http://localhost:9092 \
  --active-protocol 0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401 \
  --active-market 0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d \
  --status-protocol 0xC9e7304f719D35919b0371d8B242ab59E0966d63 \
  --status-market 0xb6c1392944a335b72b9e34f9D4b8c0050cdb511f \
  --output table

# JSON output for programmatic processing
python3 scripts/analyze_epoch_status.py \
  --status-protocol 0xC9e7304f719D35919b0371d8B242ab59E0966d63 \
  --status-market 0xb6c1392944a335b72b9e34f9D4b8c0050cdb511f \
  --output json | jq '.stats'
```

**Output Formats**:
- `summary` (default): Human-readable summary with statistics
- `table`: Detailed table showing each epoch's status
- `json`: Machine-readable JSON output

**What It Checks**:
1. **Transaction Hash Presence**: Verifies that epochs with `onchain_status` of "submitted", "confirmed", or "failed" have transaction hashes
2. **Status Consistency**: Checks that epochs in "onchain_submission" phase have appropriate on-chain status
3. **Error Details**: Identifies failed submissions without error details
4. **VPA Submission Tracking**: Verifies that VPA submission attempts are properly tracked

**Example Output**:
```
================================================================================
SUMMARY
================================================================================
Total active epochs: 32
With status data (NEW addresses): 32
Without status data: 0

On-chain Status Distribution:
  confirmed: 15
  queued: 3
  submitted: 2

Transaction Hash Analysis:
  Epochs WITH tx_hash: 17
  Epochs WITHOUT tx_hash: 3

Failed Submissions: 0
Failed without error details: 0
```

**Exit Codes**:
- `0`: No critical issues found
- `1`: Critical issues detected (failed epochs without error details, or statuses requiring tx_hash but missing it)

### check_tx_hashes.sh

**Purpose**: Convenient wrapper script for quick transaction hash analysis.

**Usage**:
```bash
# Use default addresses
./scripts/check_tx_hashes.sh

# Specify custom addresses
./scripts/check_tx_hashes.sh \
  http://localhost:9092 \
  0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401 \
  0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d \
  0xC9e7304f719D35919b0371d8B242ab59E0966d63 \
  0xb6c1392944a335b72b9e34f9D4b8c0050cdb511f
```

**What It Does**:
1. Runs `analyze_epoch_status.py` with summary output
2. Runs `analyze_epoch_status.py` with table output
3. Provides both high-level summary and detailed epoch-by-epoch breakdown

### Understanding Address Normalization

**Important**: The scripts automatically lowercase Ethereum addresses to match Redis key format. Relayer-py lowercases addresses when writing to Redis, so the scripts normalize addresses to ensure correct key matching.

**Why This Matters**:
- Redis keys use lowercase addresses: `0xc9e7304f719d35919b0371d8b242ab59e0966d63:0xb6c1392944a335b72b9e34f9d4b8c0050cdb511f:epoch:23875427:state`
- Relayer-py writes using lowercase addresses (from `PROTOCOL_STATE_CONTRACT` env var)
- The Monitor API accepts addresses in any case, but Redis keys are case-sensitive
- The scripts normalize addresses to ensure correct key matching

**Verification**:
```bash
# Check Redis key format (addresses are lowercase)
redis-cli KEYS "*c9e7304f719d35919b0371d8b242ab59e0966d63*23875427*"

# Query API with mixed-case addresses (works, but scripts normalize to lowercase)
curl "http://localhost:9092/api/v1/epochs/23875427/status?protocol=0xC9e7304f719D35919b0371d8B242ab59E0966d63&market=0xb6c1392944a335b72b9e34f9D4b8c0050cdb511f"
```

### Integration with Monitoring Workflow

**Recommended Workflow**:
1. **Daily Check**: Run `check_tx_hashes.sh` to verify transaction hash presence
2. **Issue Investigation**: Use `analyze_epoch_status.py --output table` to see detailed epoch-by-epoch status
3. **Automated Monitoring**: Integrate `analyze_epoch_status.py --output json` into monitoring systems
4. **Alerting**: Script exits with code 1 if critical issues detected (can be used in cron jobs)

**Example Cron Job**:
```bash
# Run every hour, alert on critical issues
0 * * * * /path/to/dsv/scripts/check_tx_hashes.sh >> /var/log/dsv/tx_hash_check.log 2>&1 || echo "Critical issues detected" | mail -s "DSV Transaction Hash Check Failed" admin@example.com
```

### Troubleshooting

**No Transaction Hashes Found**:
1. Verify relayer-py is running and processing transactions
2. Check relayer-py logs for "Updated epoch state in Redis" messages
3. Verify `PROTOCOL_STATE_CONTRACT` env var in relayer-py matches the protocol address used in script
4. Check Redis connectivity from relayer-py
5. Verify addresses are correctly normalized (lowercase)

**Status Data Not Found**:
1. Ensure NEW protocol/market addresses are correct
2. Verify epochs exist in the active epochs list
3. Check that relayer-py has written to Redis for these epochs
4. Verify Redis key format matches expected pattern

**Case Sensitivity Issues**:
- The scripts automatically handle case normalization
- If manually querying Redis, use lowercase addresses
- Monitor API accepts any case, but Redis keys are lowercase
