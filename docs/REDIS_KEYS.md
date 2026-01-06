# Redis Key Architecture for Separated Container Design

## Overview
This document defines the Redis keys used for inter-container communication in the separated architecture where each container has a single responsibility.

## Container Communication Flow

```
P2P Gateway ←→ Redis ←→ Dequeuer
                ↓
           Finalizer
                ↓
           Aggregator
                ↓
           P2P Gateway
```

## Key Definitions

### State Tracker Keys

#### Active Epoch Management
- `ActiveEpochs()` - SET: Currently active epoch IDs
  - Written by: Event Monitor, P2P Gateway
  - Read by: State Tracker (deterministic aggregation)
  - Purpose: Direct access to active epochs instead of SCAN operations
  - Format: Set of epoch IDs
  - **TTL**: 24 hours (set when adding new epochs, not refreshed on reads)
  - **Pruning**: State-tracker removes epochs older than 7 days periodically
  - **Monitoring**: Alert if size exceeds 100K members
  - **Note**: TTL refresh is optimized to only occur when adding NEW epochs, preventing unnecessary refreshes

- `EpochValidators({epochId})` - SET: Validator IDs participating in each epoch
  - Written by: Event Monitor/Aggregator
  - Read by: State Tracker (deterministic aggregation)
  - Purpose: Efficient validator detection per epoch
  - Format: Set of validator IDs

- `EpochProcessed({epochId})` - SET: Processed submission IDs per epoch
  - Written by: Dequeuer
  - Read by: State Tracker (deterministic aggregation)
  - Purpose: Fast submission counting without expensive operations
  - Format: Set of submission IDs

### P2P Gateway Keys

#### Incoming (from network to Redis) - ALL NAMESPACED
- `{protocol}:{market}:submissionQueue` - LIST: Raw P2P submissions from network
  - Written by: P2P Gateway
  - Read by: Dequeuer
  - Format: JSON encoded P2PSnapshotSubmission

- `{protocol}:{market}:incoming:batch:{epochId}:{validatorId}` - STRING: Received batch from validator
  - Written by: P2P Gateway
  - Read by: Aggregator
  - TTL: 30 minutes
  - Format: JSON encoded FinalizedBatch

- `{protocol}:{market}:aggregation:queue` - LIST: Epochs ready for Level 2 aggregation
  - Written by: P2P Gateway (when batch received)
  - Read by: Aggregator (Level 2)
  - Format: epochId as string
  - **Status**: Legacy queue (may be unused in current implementation)
  - **Cleanup**: Cleanup script warns if exceeds 10K items
  - **Note**: Consider running `cleanup_stale_queue.sh` if this queue is not actively used

- `validator:active:{validatorId}` - STRING: Active validator tracking
  - Written by: P2P Gateway
  - Read by: Aggregator (for metrics)
  - TTL: 5 minutes
  - Format: Unix timestamp

#### Outgoing (from Redis to network) - NAMESPACED
- `{protocol}:{market}:outgoing:broadcast:batch` - LIST: Batches to broadcast
  - Written by: Aggregator (after Level 1 aggregation)
  - Read by: P2P Gateway
  - Format: JSON with type and data fields

### Dequeuer Keys

- `processingSubmission:{id}` - STRING: Submission being processed
  - Written by: Dequeuer
  - Read by: Dequeuer (for recovery)
  - TTL: 5 minutes

- `{protocol}:{market}:processed:{sequencerId}:{submissionId}` - STRING: Validated submission (legacy, kept for backward compatibility)
  - Written by: Dequeuer
  - Read by: (deprecated - use epoch-keyed structures instead)
  - Format: JSON-encoded ProcessedSubmission
  - TTL: 1 hour

- `{protocol}:{market}:epoch:{epochId}:processed` - SET: Submission IDs for epoch (deprecated, no longer used)
  - Written by: Dequeuer (kept for backward compatibility, may be removed in future)
  - Read by: (deprecated - State-Tracker now uses ZSET)
  - TTL: 1 hour
  - Note: Both Event Monitor and State-Tracker now use deterministic epoch-keyed structures (`EpochSubmissionsIds` ZSET)

- `{protocol}:{market}:epoch:{epochId}:submissions:ids` - ZSET: Submission IDs for epoch (deterministic, ordered by timestamp)
  - Written by: Dequeuer
  - Read by: Event Monitor (for collecting epoch submissions)
  - Score: Unix timestamp
  - Member: Submission ID
  - TTL: 2 hours (refreshed on each write)
  - Purpose: Deterministic list of all submission IDs for an epoch, ordered by timestamp

- `{protocol}:{market}:epoch:{epochId}:submissions:data` - HASH: Submission data for epoch (deterministic lookup)
  - Written by: Dequeuer
  - Read by: Event Monitor (for collecting epoch submissions)
  - Field: Submission ID
  - Value: JSON-encoded ProcessedSubmission
  - TTL: 2 hours (refreshed on each write)
  - Purpose: Deterministic lookup of submission data by epoch and submission ID

### Event Monitor Keys - NAMESPACED

- `{protocol}:{market}:epoch:{epochId}:window` - STRING: Submission window status
  - Written by: Event Monitor
  - Read by: Dequeuer, Finalizer
  - Values: "open" or "closed"
  - TTL: 1 hour after close

- `{protocol}:{market}:epoch:{epochId}:state` - HASH: Comprehensive epoch state tracking
  - Written by: Event Monitor, Aggregator, Finalizer, State-Tracker
  - Read by: Monitor API, State-Tracker
  - TTL: 7 days
  - Fields:
    - `window_status`: "open" | "closed"
    - `window_opened_at`: timestamp
    - `window_closes_at`: timestamp
    - `phase`: "submission" | "level1_finalization" | "level2_aggregation" | "onchain_submission" | "complete" | "failed"
    - `submissions_count`: count of processed submissions
    - `level1_status`: "pending" | "in_progress" | "completed" | "failed"
    - `level1_started_at`: timestamp (when finalization started)
    - `level1_completed_at`: timestamp (when Level 1 batch created)
    - `level2_status`: "pending" | "collecting" | "aggregating" | "completed" | "failed"
    - `level2_started_at`: timestamp (when Level 2 aggregation window opened)
    - `level2_completed_at`: timestamp (when Level 2 batch created)
    - `onchain_status`: "pending" | "queued" | "submitted" | "confirmed" | "failed"
    - `onchain_tx_hash`: transaction hash
    - `onchain_block_number`: block number
    - `onchain_submitted_at`: timestamp
    - `onchain_error`: error message when transaction fails (stored by relayer-py)
    - `priority`: validator priority for this epoch
    - `vpa_submission_attempted`: boolean
    - `last_updated`: timestamp

- `{protocol}:{market}:finalizationQueue` - LIST: Epochs ready for finalization
  - Written by: Event Monitor (on window close)
  - Read by: Finalizer

### Finalizer Keys

- `{protocol}:{market}:batch:part:{epochId}:{partId}` - STRING: Partial batch from worker
  - Written by: Finalizer workers
  - Read by: Aggregator (Level 1 aggregation)
  - Format: JSON with project results subset
  - TTL: 2 hours

- `{protocol}:{market}:epoch:{epochId}:parts:completed` - STRING: Count of completed parts
  - Written by: Finalizer workers
  - Read by: Workers monitoring
  - Format: Integer count

- `{protocol}:{market}:epoch:{epochId}:parts:total` - STRING: Total expected parts
  - Written by: Event Monitor/Finalizer
  - Read by: Workers monitoring
  - Format: Integer count

- `{protocol}:{market}:epoch:{epochId}:parts:ready` - STRING: Flag for ready status
  - Written by: Finalizer workers when all parts complete
  - Read by: Aggregator
  - Format: "true"

- `{protocol}:{market}:aggregationQueue` - LIST: Worker parts ready for Level 1 aggregation
  - Written by: Finalizer workers (via UpdateBatchPartsProgress)
  - Read by: Aggregator (Level 1)
  - Format: JSON with epoch_id, parts_completed

- `{protocol}:{market}:finalized:{epochId}` - STRING: Complete local finalized batch
  - Written by: Aggregator (after Level 1 aggregation)
  - Read by: Aggregator (for Level 2), Monitoring
  - Format: JSON encoded FinalizedBatch with IPFS CID

### Aggregator Keys - ALL NAMESPACED

- `{protocol}:{market}:batch:aggregated:{epochId}` - STRING: Network-wide consensus batch
  - Written by: Aggregator (Level 2 aggregation)
  - Read by: Monitoring/API
  - Format: JSON with all validator batches merged

### State-Tracker Monitoring Keys - ALL NAMESPACED

#### Dashboard Metrics
- `{protocol}:{market}:dashboard:summary` - STRING: Pre-aggregated system metrics for API
  - Written by: State-Tracker (aggregateCurrentMetrics)
  - Read by: Monitor API (metrics endpoint)
  - TTL: 60 seconds
  - Format: JSON with rates, counts, recent activity
  - Fields: active_validators, batch_rate, epochs_rate, epochs_1m, batches_1m, epochs_5m, batches_5m, submissions_1m, submissions_5m, etc.

- `{protocol}:{market}:stats:current` - HASH: Current operational stats (same data as dashboard:summary but as hash)
  - Written by: State-Tracker (aggregateCurrentMetrics)
  - Read by: Monitor API (current_stats endpoint)
  - TTL: 60 seconds
  - Purpose: Easy field access for API responses

#### Participation Metrics (24-hour)
- `{protocol}:{market}:metrics:participation` - STRING: Validator participation and inclusion statistics
  - Written by: State-Tracker (aggregateParticipationMetrics)
  - Read by: Monitor API (participation_stats endpoint)
  - TTL: 300 seconds (5 minutes)
  - Format: JSON with 24h aggregated data
  - Fields: epochs_participated_24h, level1_batches_24h, level2_inclusions_24h, participation_rate, inclusion_rate, epochs_total_24h

#### Current Epoch Status
- `{protocol}:{market}:metrics:current_epoch` - STRING: Current epoch timing and phase information
  - Written by: State-Tracker (aggregateCurrentEpochStatus)
  - Read by: Monitor API (current_status endpoint)
  - TTL: 30 seconds
  - Format: JSON with epoch status
  - Fields: epoch_id, phase, time_remaining_seconds, window_duration, submissions_received

#### Timeline Event Tracking
- `{protocol}:{market}:metrics:epochs:timeline` - ZSET: Epoch lifecycle events
  - Written by: Event Monitor (epoch open/close events)
  - Read by: State-Tracker (for epoch counting), Monitor API
  - No TTL (pruned daily by state-tracker)
  - Format: Sorted set by timestamp
  - Members: "open:{epochId}", "close:{epochId}"

- `{protocol}:{market}:metrics:batches:timeline` - ZSET: Batch creation events
  - Written by: Aggregator (local and aggregated batches)
  - Read by: State-Tracker (for batch counting), Monitor API
  - No TTL (pruned daily by state-tracker)
  - Format: Sorted set by timestamp
  - Members: "local:{epochId}", "aggregated:{epochId}"

- `{protocol}:{market}:metrics:submissions:timeline` - ZSET: Submission receipt events
  - Written by: P2P Gateway, Dequeuer (enhanced format)
  - Read by: State-Tracker (for submission counting), Monitor API (epoch submissions endpoint)
  - No TTL (pruned daily by state-tracker)
  - Format: Sorted set by timestamp (score = Unix timestamp)
  - Members: Enhanced entity IDs like `received:{epochId}:{slotId}:{projectId}:{timestamp}:{peerId}` or legacy format `{epochId}-{projectId}-{timestamp}`
  - Purpose: Track all submissions with epoch context for querying submissions per epoch

- `{protocol}:{market}:metrics:submissions:metadata:{entityId}` - STRING: Detailed submission metadata
  - Written by: P2P Gateway, Dequeuer (when enhanced entity ID format is used)
  - Read by: Monitor API (epoch submissions endpoint, timeline with metadata)
  - TTL: 24 hours
  - Format: JSON object with fields:
    - `epoch_id`: Epoch ID (string)
    - `slot_id`: Snapshotter slot ID (string or number)
    - `project_id`: Project ID (string)
    - `cid` or `snapshot_cid`: IPFS CID of snapshot (string)
    - `peer_id`: Peer ID that sent the submission (string)
    - `validator_id`: Validator ID if available (string, optional)
    - `timestamp`: Unix timestamp (number)
    - `entity_id`: The entity ID from timeline (string)
  - Purpose: Store detailed metadata for each submission to enable epoch-centered queries

- `{protocol}:{market}:metrics:validations:timeline` - ZSET: Validation completion events
  - Written by: Dequeuer (ProcessSubmission)
  - Read by: State-Tracker (for validation metrics)
  - No TTL (pruned daily by state-tracker - CRITICAL: must be pruned to prevent unbounded growth)
  - Format: Sorted set by timestamp
  - Members: submissionId
  - **Pruning**: State-tracker removes entries older than 24 hours daily
  - **Monitoring**: Alert if size exceeds 1M members

#### Validator-Specific Tracking
- `{protocol}:{market}:metrics:validator:{validatorId}:batches` - ZSET: Per-validator batch timeline
  - Written by: Aggregator (when batches created)
  - Read by: State-Tracker (participation metrics)
  - No TTL (pruned daily by state-tracker)
  - Format: Sorted set by timestamp
  - Members: epochId
  - Purpose: Track individual validator participation

- `{protocol}:{market}:metrics:batch:{epochId}:validators` - STRING: Validator list for specific batch
  - Written by: Aggregator (during batch creation)
  - Read by: State-Tracker (participation metrics)
  - TTL: 24 hours
  - Format: JSON array of validator IDs
  - Purpose: Track who participated in each batch

#### Submission Tracking Keys

**Deterministic Epoch Submission Storage (Primary)**:
- `{protocol}:{market}:epoch:{epochId}:submissions:ids` - ZSET: Submission IDs per epoch (deterministic, ordered)
  - Written by: Dequeuer (when processing submissions)
  - Read by: Event Monitor (for collecting epoch submissions)
  - Score: Unix timestamp
  - Member: Submission ID
  - TTL: 2 hours (refreshed on each write)
  - Purpose: Deterministic list of all submission IDs for an epoch, ordered by timestamp
  - **No SCAN operations needed** - direct ZRANGE lookup

- `{protocol}:{market}:epoch:{epochId}:submissions:data` - HASH: Submission data per epoch (deterministic lookup)
  - Written by: Dequeuer (when processing submissions)
  - Read by: Event Monitor (for collecting epoch submissions)
  - Field: Submission ID
  - Value: JSON-encoded ProcessedSubmission
  - TTL: 2 hours (refreshed on each write)
  - Purpose: Deterministic lookup of submission data by epoch and submission ID
  - **No SCAN operations needed** - direct HGETALL lookup

**Legacy Keys (Deprecated - no longer used)**:
- `{protocol}:{market}:epoch:{epochId}:processed` - SET: Processed submission IDs per epoch
  - Written by: Dequeuer (kept for backward compatibility, may be removed in future)
  - Read by: (deprecated - State-Tracker now uses ZSET)
  - TTL: 1 hour after epoch window closes
  - Format: Set of submission IDs (internal format, not entity IDs)
  - Purpose: Legacy tracking of processed submissions (replaced by deterministic ZSET)
  - Note: Both Event Monitor and State-Tracker now use deterministic epoch-keyed structures (`EpochSubmissionsIds` ZSET)

#### Deterministic Aggregation Keys
- `{protocol}:{market}:epochs:active` - SET: Currently active epoch IDs (legacy, may be stale)
  - Written by: Event Monitor (when epoch window opens)
  - Read by: State-Tracker (deterministic aggregation) - but Monitor API queries timeline directly instead
  - Purpose: Direct access to active epochs instead of SCAN operations
  - Format: Set of epoch IDs as strings
  - Note: Monitor API's `/epochs/active` endpoint queries timeline directly for accuracy, not this SET

- `{protocol}:{market}:EpochValidators({epochId})` - SET: Validator IDs participating in each epoch
  - Written by: Event Monitor/Aggregator
  - Read by: State-Tracker (deterministic aggregation)
  - Purpose: Efficient validator detection per epoch
  - Format: Set of validator IDs

- `{protocol}:{market}:EpochProcessed({epochId})` - SET: Processed submission IDs per epoch
  - Written by: Dequeuer
  - Read by: State-Tracker (deterministic aggregation), Event Monitor (for collecting submissions)
  - Purpose: Fast submission counting without expensive operations
  - Format: Set of submission IDs (internal format)
  - Note: This is the same as `{protocol}:{market}:epoch:{epochId}:processed` - both keys exist for compatibility

- `{protocol}:{market}:epochs:gaps` - ZSET: Epoch gaps tracking
  - Written by: State-Tracker (detectEpochGaps)
  - Read by: Monitor API (epochs/gaps endpoint)
  - TTL: 24 hours (old gaps pruned after 1 hour)
  - Format: Sorted set by timestamp
  - Members: "{epochId}:{gapType}" where gapType is "missing_level1", "missing_level2", or "missing_onchain"
  - Purpose: Track epochs with missing finalizations for gap detection and alerting

#### Legacy Health Monitoring
- `pipeline:health:{component}` - STRING: Component health status
  - Written by: Each component
  - Read by: Monitoring
  - Format: JSON with status, last_update, metrics

- `submission_stats:{epochId}` - HASH: Epoch statistics
  - Written by: Finalizer
  - Read by: Monitoring/API
  - Fields: total_submissions, unique_projects, timestamp

## Data Flow Examples

### 1. Submission Flow with Monitoring
```
Network → P2P Gateway → submissionQueue → Dequeuer → processed:{id} → Event Monitor
                                           ↓                          ↓
                    metrics:submissions:timeline              metrics:epochs:timeline
                    (entityId: received:{epoch}:{slot}:...)          ↓
                                           ↓                  epochs:active SET
                    metrics:submissions:metadata:{entityId}   epoch:{epochId}:state
                    (detailed metadata: slot_id, peer_id, etc.)      ↓
                                           ↓                  epoch:{epochId}:processed SET
                    Monitor API (/epochs/{id}/submissions)
                    (queries timeline + metadata)
```

### 2. Level 1 Aggregation with Monitoring (Worker Parts → Local Batch)
```
Finalizer Workers → batch:part:{epoch}:{0..N} → aggregationQueue → Aggregator
→ {protocol}:{market}:finalized:{epochId} + outgoing:broadcast:batch
        ↓                                              ↓
EpochProcessed({epochId}) SET                     metrics:batches:timeline
        ↓                                              ↓
  metrics:validator:{validatorId}:batches      dashboard:summary (rates)
```

### 3. Batch Broadcast Flow
```
Aggregator (Level 1) → outgoing:broadcast:batch → P2P Gateway → Network
```

### 4. Batch Reception Flow with Participation Tracking
```
Network → P2P Gateway → incoming:batch:{epochId}:{validatorId} + aggregation:queue → Aggregator
                                                                                   ↓
                                                                     metrics:batch:{epochId}:validators
```

### 5. Level 2 Aggregation with Participation Metrics
```
{protocol}:{market}:finalized:{epochId} + incoming:batch:{epochId}:*
→ Aggregator → batch:aggregated:{epochId}
                    ↓
            metrics:participation (24h stats)
```

### 6. State-Tracker Monitoring Flow
```
Timeline Events (epochs, batches, submissions, validations) → State-Tracker
                                                            ↓
                                        dashboard:summary + stats:current
                                                            ↓
                                        metrics:participation + current_epoch
                                        + epoch:{epochId}:state (submissions_count)
                                        + epochs:gaps (gap detection)
                                                            ↓
                                                    Monitor API Response
```

### 8. Epoch Submissions Query Flow
```
Monitor API: GET /epochs/{epochId}/submissions
    ↓
Query: metrics:submissions:timeline (ZRANGEBYSCORE, last 24h)
    ↓
Filter: Parse entity IDs, match epoch ID
    ↓
For each matching entity ID:
    Query: metrics:submissions:metadata:{entityId}
    Fallback: Parse entity ID format if metadata missing
    ↓
Return: Array of SubmissionInfo (slot_id, peer_id, project_id, cid, etc.)
```

### 9. Active Epochs Query Flow (Fixed Implementation)
```
Monitor API: GET /epochs/active
    ↓
Query: metrics:epochs:timeline (ZREVRANGE, last 100 epochs)
    ↓
For each epoch ID:
    Query: epoch:{epochId}:state (HGETALL)
    Check: window_status, level1_status, level2_status
    Filter: Only include if window="open" OR level1="in_progress" OR level2="collecting"/"aggregating"
    ↓
Return: Array of active EpochInfo (sorted by epoch ID descending)
```

### 7. Deterministic Aggregation Flow
```
Event Monitor → ActiveEpochs SET + EpochValidators({epochId}) SET + metrics:epochs:timeline
                     ↓                                          ↓
            State-Tracker (direct access)          State-Tracker (timeline counting)
                     ↓                                          ↓
           Eliminates SCAN operations                Accurate rate calculations
```

## Component Responsibilities

### P2P Gateway
**Writes:**
- `{protocol}:{market}:submissionQueue` - Raw submissions from network
- `{protocol}:{market}:incoming:batch:{epochId}:{validatorId}` - Received batches from validators
- `{protocol}:{market}:aggregation:queue` - Epochs ready for Level 2 aggregation
- `validator:active:{validatorId}` - Active validator tracking
- `metrics:submissions:timeline` - Submission receipt events (NEW)

**Reads:**
- `{protocol}:{market}:outgoing:broadcast:batch` - Batches to broadcast

### Dequeuer
**Writes:**
- `{protocol}:{market}:processed:{sequencerId}:{submissionId}` - Validated submissions (legacy, kept for compatibility)
- `{protocol}:{market}:epoch:{epochId}:processed` - Set of processed submissions (legacy, kept for compatibility)
- `{protocol}:{market}:epoch:{epochId}:submissions:ids` - ZSET: Deterministic submission IDs per epoch (NEW)
- `{protocol}:{market}:epoch:{epochId}:submissions:data` - HASH: Deterministic submission data per epoch (NEW)
- `{protocol}:{market}:EpochProcessed({epochId})` - Deterministic aggregation set
- `metrics:validations:timeline` - Validation completion events (NEW)

**Reads:**
- `{protocol}:{market}:submissionQueue` - Raw submissions to process

### Event Monitor
**Writes:**
- `{protocol}:{market}:epoch:{epochId}:window` - Submission window status
- `{protocol}:{market}:epoch:{epochId}:state` - Initial epoch state hash (window status, phase, timestamps)
- `{protocol}:{market}:finalizationQueue` - Epochs ready for finalization
- `{protocol}:{market}:metrics:epochs:timeline` - Epoch lifecycle events
- `{protocol}:{market}:ActiveEpochs` - SET of active epoch IDs
- `{protocol}:{market}:EpochValidators({epochId})` - SET of validators per epoch

**Reads:**
- `{protocol}:{market}:epoch:{epochId}:submissions:ids` - ZSET: Deterministic submission IDs (no SCAN needed)
- `{protocol}:{market}:epoch:{epochId}:submissions:data` - HASH: Deterministic submission data (no SCAN needed)

### Finalizer
**Writes:**
- `{protocol}:{market}:batch:part:{epochId}:{partId}` - Partial batch results
- `{protocol}:{market}:epoch:{epochId}:parts:*` - Progress tracking
- `{protocol}:{market}:aggregationQueue` - Ready for Level 1 aggregation
- `submission_stats:{epochId}` - Epoch statistics
- `{protocol}:{market}:epoch:{epochId}:state` - Updates level1_status to "in_progress" when finalization starts

### Aggregator
**Writes:**
- `{protocol}:{market}:finalized:{epochId}` - Complete local batch
- `{protocol}:{market}:batch:aggregated:{epochId}` - Network consensus batch
- `{protocol}:{market}:metrics:batches:timeline` - Batch creation events
- `{protocol}:{market}:metrics:validator:{validatorId}:batches` - Per-validator timeline
- `{protocol}:{market}:metrics:batch:{epochId}:validators` - Validator list per batch
- `{protocol}:{market}:metrics:participation` - 24h participation metrics
- `{protocol}:{market}:epoch:{epochId}:state` - Updates level1_status, level2_status, onchain_status, priority

**Reads:**
- `{protocol}:{market}:aggregationQueue` - Worker parts to aggregate
- `{protocol}:{market}:incoming:batch:{epochId}:{validatorId}` - Remote batches
- `{protocol}:{market}:EpochValidators({epochId})` - Active validators

### State-Tracker (NEW)
**Writes:**
- `{protocol}:{market}:dashboard:summary` - Pre-aggregated system metrics
- `{protocol}:{market}:stats:current` - Current operational stats (hash)
- `{protocol}:{market}:metrics:current_epoch` - Current epoch status
- `{protocol}:{market}:epoch:{epochId}:state` - Updates submission_count field
- `{protocol}:{market}:epochs:gaps` - Epoch gaps tracking
- Pruning of old timeline data

**Reads:**
- `{protocol}:{market}:metrics:*:timeline` - All timeline events for counting
- `{protocol}:{market}:ActiveEpochs` - Direct epoch access
- `{protocol}:{market}:EpochValidators({epochId})` - Validator sets
- `{protocol}:{market}:EpochProcessed({epochId})` - Submission sets
- `{protocol}:{market}:epoch:{epochId}:state` - Epoch state for gap detection

### Monitor API
**Writes:**
- None (read-only component)

**Reads:**
- `{protocol}:{market}:dashboard:summary` - System metrics
- `{protocol}:{market}:stats:current` - Current stats
- `{protocol}:{market}:metrics:participation` - Participation data
- `{protocol}:{market}:metrics:current_epoch` - Epoch status
- `{protocol}:{market}:metrics:epochs:timeline` - Epoch timeline (for active epochs query)
- `{protocol}:{market}:epoch:{epochId}:state` - Epoch state hash (for status, active epochs, gaps)
- `{protocol}:{market}:metrics:submissions:timeline` - Submissions timeline (for epoch submissions endpoint)
- `{protocol}:{market}:metrics:submissions:metadata:{entityId}` - Submission metadata (for detailed submission info)
- `{protocol}:{market}:metrics:batches:timeline` - Batches timeline
- `{protocol}:{market}:metrics:batch:local:{epochId}` - Level 1 batch metadata
- `{protocol}:{market}:metrics:batch:aggregated:{epochId}` - Level 2 batch metadata
- `{protocol}:{market}:metrics:batch:{epochId}:validators` - Validator list for batch
- `{protocol}:{market}:metrics:epoch:{epochId}:info` - Epoch info hash
- `{protocol}:{market}:epochs:gaps` - Epoch gaps ZSET
- All queue depths for real-time status (LLen operations)

## Key Naming Conventions

1. **Queues**: Simple names for lists (e.g., `submissionQueue`)
2. **Temporary**: Prefixed with action (e.g., `processingSubmission:{id}`)
3. **Persistent**: Namespaced by protocol/market (e.g., `{protocol}:{market}:processed:{id}`)
4. **Communication**: Direction prefix (e.g., `incoming:`, `outgoing:`)
5. **Status**: Component:metric format (e.g., `pipeline:health:{component}`)
6. **Monitoring**: `metrics:{type}:timeline` for event tracking
7. **Deterministic**: `ActiveEpochs`, `Epoch*` for direct access patterns

## TTL Guidelines

### Core Data Flow
- **Temporary processing**: 5 minutes
- **Window status**: 1 hour after close
- **Incoming batches**: 30 minutes
- **Finalized batches**: 24 hours minimum
- **Aggregated batches**: Persistent (no TTL)
- **Active validators**: 5 minutes

### Monitoring & Metrics
- **Dashboard summary**: 60 seconds (real-time metrics)
- **Current stats**: 60 seconds (same as dashboard summary)
- **Participation metrics**: 5 minutes (24h calculations less frequent)
- **Current epoch status**: 30 seconds (frequent updates)
- **Timeline events**: No TTL (pruned daily by state-tracker)
- **Validator batch timelines**: No TTL (pruned daily by state-tracker)
- **Batch validator lists**: 24 hours (participation tracking)
- **Health status**: 5 minutes (component monitoring)

### Pruning Strategy

#### Timeline Pruning (CRITICAL)
State-tracker prunes the following timeline zsets daily (removes entries older than 24 hours):
- `{protocol}:{market}:metrics:epochs:timeline` ✅
- `{protocol}:{market}:metrics:batches:timeline` ✅
- `{protocol}:{market}:metrics:submissions:timeline` ✅ (Fixed - was missing)
- `{protocol}:{market}:metrics:validations:timeline` ✅ (Fixed - was missing)

**Note**: If pruning fails, these zsets will grow unbounded. Monitor key sizes and alert if they exceed 1M members.

#### ActiveEpochs SET Pruning
The `{protocol}:{market}:epochs:active` SET is pruned periodically by state-tracker to remove epochs older than 7 days. This prevents unbounded growth when TTL keeps getting refreshed.

**TTL Behavior**: 
- TTL is only refreshed when adding NEW epochs (not when epoch already exists)
- Event monitor checks if epoch was actually added before refreshing TTL
- P2P gateway only sets TTL if missing (doesn't refresh on every add)
- The set has a 24-hour TTL as a safety net, but relies on periodic pruning for cleanup

#### Legacy Queue Cleanup
The `{protocol}:{market}:aggregation:queue` LIST is a legacy structure that is not used by the active aggregation system (which uses Redis streams instead). 

**Automated Cleanup**: State-tracker automatically deletes this queue if it exceeds 10K items to prevent unbounded growth. The cleanup script will also warn about it if it exceeds the threshold.

#### Size Monitoring
State-tracker monitors Redis key sizes and logs warnings if they exceed thresholds:
- **ZSETs**: Alert if > 1M members
- **SETs**: Alert if > 100K members
- **LISTs**: Alert if > 10K items

### Deterministic Sets
- **ActiveEpochs**: Pruned periodically (removes epochs older than 7 days)
- **EpochValidators**: Managed per epoch, cleaned up with epoch state
- **EpochSubmissionsIds**: Managed per epoch, cleaned up with epoch state

### Metrics Aggregation
State-tracker manages rolling windows for metrics aggregation. Timeline keys are the source of truth and must be pruned regularly.

## Spam Protection Keys (namespaced per protocol:market)

### Per-Epoch Tracking Keys (TTL: 24 hours)

**Validation Failures**:
- `{protocol}:{market}:spam:validation_failures:peer:{peerID}:{epochID}` - Counter (INT) - PRIMARY
  - Written by: Dequeuer (on validation failure)
  - Read by: Spam Reporter (to check thresholds)
  - TTL: 24 hours (set on first increment)
  - Purpose: Track validation failures per peer per epoch

- `{protocol}:{market}:spam:validation_failures:snapshotter:{snapshotterAddr}:{epochID}` - Counter (INT) - SECONDARY
  - Written by: Dequeuer (on validation failure)
  - Read by: Spam Reporter (for evidence)
  - TTL: 24 hours (set on first increment)
  - Purpose: Track validation failures per snapshotter address per epoch

**Submission Counts**:
- `{protocol}:{market}:spam:submissions:peer:{peerID}:{epochID}` - Counter (INT) - PRIMARY
  - Written by: Dequeuer (after signature verification)
  - Read by: Rate Limiter (to check limits)
  - TTL: 24 hours (set on first increment)
  - Purpose: Track submission count per peer per epoch (enforcement)

- `{protocol}:{market}:spam:submissions:snapshotter:{snapshotterAddr}:{epochID}` - Counter (INT) - SECONDARY
  - Written by: Dequeuer (after signature verification)
  - Read by: Spam Reporter (for evidence)
  - TTL: 24 hours (set on first increment)
  - Purpose: Track submission count per snapshotter address per epoch (evidence)

**Peer-Snapshotter Association**:
- `{protocol}:{market}:spam:peer_snapshotter_map:{peerID}:{epochID}` - SET (snapshotter addresses)
  - Written by: Dequeuer (when tracking submissions/failures)
  - Read by: Spam Aggregator (to collect all addresses for flagging)
  - TTL: 24 hours (set on first association)
  - Purpose: Track which snapshotter addresses are used by which peer in an epoch

### Aggregation Window Keys (TTL: 2 hours)

**Aggregated Reports**:
- `{protocol}:{market}:spam:reports:peer:{peerID}:window:{windowID}` - String (JSON) - Aggregated reports per peer per window
  - Written by: Spam Aggregator (when receiving reports)
  - Read by: Spam Aggregator (to check consensus)
  - TTL: 2 hours (set on first report)
  - Value: JSON `AggregatedReport` with:
    - `reports` (JSON array): All spam reports for this peer in this window
    - `validator_ids` (array): Validator IDs that reported
    - `validator_count` (INT): Number of unique validators reporting
    - `snapshotter_addrs` (array): All snapshotter addresses associated with this peer
    - `first_epoch` (INT): First epoch in window with reports
    - `last_epoch` (INT): Last epoch in window with reports
    - `first_seen` (INT): Timestamp when first report received
    - `last_updated` (INT): Timestamp when last report received
  - Purpose: Aggregate spam reports over multiple epochs before checking consensus
  - Window calculation: `windowID = ((epochID + 9) / 10) * 10` (rounds up to next multiple of 10, hardcoded window size 10)

**Window Peers Set**:
- `{protocol}:{market}:spam:reports:window:{windowID}:peers` - SET (peer IDs)
  - Written by: Spam Aggregator (when adding first report for a peer in a window)
  - Read by: Spam Aggregator (to get all peers with reports in a window for consensus checking)
  - TTL: 2 hours (set on first peer added, matches aggregation window TTL)
  - Purpose: **Deterministic key** to track all peer IDs with reports in a window - enables efficient consensus checking

**Master Windows Set** (for discovery/indexing):
- `{protocol}:{market}:spam:reports:windows` - SET (window IDs as strings)
  - Written by: Spam Aggregator (when adding first report for any peer in a window)
  - Read by: Monitoring API (to list all windows with reports)
  - Pruned by: Spam Aggregator (periodically removes expired windows - windows expire when their peers set TTL expires after 2 hours)
  - TTL: None (persistent index, pruned when windows expire)
  - Purpose: **Master index** of all windows that have spam reports - enables window discovery without knowing window IDs

**Report Sent Tracking**:
- `{protocol}:{market}:spam:report:sent:reporter:{reporterID}:peer:{peerID}:epoch:{epochID}:reason:{reason}` - String ("true")
  - Written by: Spam Reporter (to prevent duplicate reports)
  - Read by: Spam Reporter (to check if already sent)
  - TTL: 24 hours (matches cache TTL)
  - Purpose: Prevent a single validator from sending duplicate reports for the same peer/epoch/reason

### Global Flagged State (Redis Cache - synced from on-chain, TTL: 24 hours)

**Flagged Peers**:
- `{protocol}:{market}:spam:consensus_flagged:peer:{peerID}` - String (JSON) - PRIMARY
  - Written by: Flagging Service (after consensus reached, synced from on-chain)
  - Read by: P2P Gateway (early rejection), Dequeuer (defense in depth)
  - TTL: 24 hours (cache TTL, refreshed on sync)
  - Value: JSON with `{"flagged_at": timestamp, "first_epoch": epochID, "last_epoch": epochID, "snapshotter_addrs": [addresses], "synced_at": timestamp}`
  - Source of truth: On-chain contract
  - Purpose: Cache flagged peer state for fast lookup (early rejection in P2P Gateway)

**Flagged Snapshotters**:
- `{protocol}:{market}:spam:consensus_flagged:snapshotter:{snapshotterAddr}` - String (JSON) - SECONDARY
  - Written by: Flagging Service (after consensus reached, synced from on-chain)
  - Read by: Dequeuer (defense in depth)
  - TTL: 24 hours (cache TTL, refreshed on sync)
  - Value: JSON with `{"flagged_at": timestamp, "associated_peer_id": peerID, "first_epoch": epochID, "last_epoch": epochID, "synced_at": timestamp}`
  - Source of truth: On-chain contract
  - Purpose: Cache flagged snapshotter state for fast lookup (defense in depth in Dequeuer)

**Flagged Sets** (for quick lookup):
- `flagged_peers:{dataMarket}` - SET (peer IDs) - NEW
  - Written by: Flagging Service (synced from on-chain)
  - Read by: P2P Gateway, Dequeuer (for quick lookup)
  - TTL: 24 hours (cache TTL, refreshed on sync)
  - Purpose: Quick lookup set of all flagged peer IDs for a data market

- `flagged_snapshotters:{dataMarket}` - SET (snapshotter addresses) - EXISTING, EXTENDED
  - Written by: Flagging Service (synced from on-chain), existing identity verifier
  - Read by: Dequeuer, identity verifier (for quick lookup)
  - TTL: 24 hours (cache TTL, refreshed on sync)
  - Purpose: Quick lookup set of all flagged snapshotter addresses for a data market

**Active Validators** (for consensus calculation):
- `{protocol}:{market}:active:validators` - SET (validator Peer IDs)
  - Written by: P2P Gateway (presence heartbeat), Spam Aggregator (monitoring)
  - Read by: Spam Aggregator (to calculate consensus threshold)
  - TTL: 5 minutes (presence heartbeat)
  - Purpose: Track active validators for consensus threshold calculation

### On-Chain State (Permanent - source of truth)

**Contract Storage**:
- Protocol contract: `flaggedPeers(bytes peerID) → FlaggedPeer`
  - Stores peer ID, associated snapshotter addresses, first/last flagged epochs, timestamp
  - Permanent until cleared via contract method
  - Source of truth for flagged state

- Protocol contract: `snapshotterToPeer(address snapshotter) → bytes peerID`
  - Maps snapshotter address to peer ID
  - Used for reverse lookup

**Clearing Mechanism**:
- Time-based: Clear entries where `flaggedAt + SPAM_FLAG_EXPIRY_DAYS < now` (default: 7 days)
- Epoch-based: Clear entries where `lastFlaggedEpoch + SPAM_FLAG_EXPIRY_EPOCHS < currentEpoch` (default: 100 epochs)
- Manual: Admin function to clear specific peers/addresses

### State Synchronization

**On Node Startup**:
1. Query on-chain contract for all flagged peers
2. Populate Redis cache with flagged peer and snapshotter keys
3. Update flagged sets (`flagged_peers:{dataMarket}`, `flagged_snapshotters:{dataMarket}`)
4. Set cache TTL: 24 hours

**Periodic Sync** (every `SPAM_SYNC_INTERVAL_HOURS`):
1. Query on-chain contract for changes since last sync
2. Update Redis cache with new/changed flagged peers
3. Remove cleared peers from Redis cache
4. Refresh TTL on existing cache entries

**Cache Refresh Strategy**:
- On startup: Full sync from on-chain
- Periodic: Incremental sync (check for new flags)
- On flagging event: Immediate sync after on-chain update
- Fallback: If Redis cache miss, query on-chain contract directly

### Key Usage by Component

**P2P Gateway**:
- Reads: `{protocol}:{market}:spam:consensus_flagged:peer:{peerID}`, `flagged_peers:{dataMarket}`
- Purpose: Early rejection of submissions from flagged peers (before queuing)

**Dequeuer**:
- Writes: All per-epoch tracking keys (validation failures, submission counts, peer-snapshotter maps)
- Reads: `{protocol}:{market}:spam:consensus_flagged:peer:{peerID}`, `{protocol}:{market}:spam:consensus_flagged:snapshotter:{snapshotterAddr}`, `flagged_peers:{dataMarket}`, `flagged_snapshotters:{dataMarket}`
- Purpose: Track spam behavior, enforce rate limits, reject flagged peers/snapshotters

**Spam Reporter**:
- Writes: `{protocol}:{market}:spam:report:sent:reporter:{reporterID}:peer:{peerID}:epoch:{epochID}:reason:{reason}`
- Reads: Per-epoch tracking keys (to check thresholds)
- Purpose: Broadcast spam reports when thresholds exceeded

**Spam Aggregator**:
- Writes: `{protocol}:{market}:spam:reports:peer:{peerID}:window:{windowID}`, `{protocol}:{market}:spam:reports:window:{windowID}:peers` (SET), `{protocol}:{market}:spam:reports:windows` (master set)
- Reads: Aggregated report keys, window peers set (`{protocol}:{market}:spam:reports:window:{windowID}:peers`), master windows set (`{protocol}:{market}:spam:reports:windows`), `{protocol}:{market}:active:validators`
- Purpose: Aggregate reports from multiple validators, check consensus, trigger flagging

**Flagging Service**:
- Writes: All flagged state keys (peer, snapshotter, sets)
- Reads: On-chain contract (source of truth)
- Purpose: Flag peers on-chain, sync state to Redis cache

### TTL Management

- **Per-epoch keys**: 24 hours TTL (set on first increment)
- **Aggregation window keys**: 2 hours TTL (set on first report)
- **Flagged state cache**: 24 hours TTL (refreshed on sync)
- **On-chain state**: Permanent (no TTL, cleared via contract methods)
- **Active validators**: 5 minutes TTL (presence heartbeat)

All counters set TTL on first increment to prevent unbounded growth.