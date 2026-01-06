# Spam Protection Specification: Multi-Layer DDoS Defense with Validator Coordination

## Overview

This specification defines the multi-layer DDoS protection system for DSV nodes. The system protects against DDoS attacks by tracking validation failures and submission rates per snapshotter, enabling validators to share spam reports via P2P before on-chain flagging, and enforcing limits at both P2P Gateway and Dequeuer levels.

## Table of Contents

1. [Architecture](#architecture)
2. [Phase 1: Tracking Layer](#phase-1-tracking-layer)
3. [Phase 2: Validator Coordination Layer](#phase-2-validator-coordination-layer)
4. [Phase 3: Enforcement Layer](#phase-3-enforcement-layer)
5. [Phase 4: On-Chain Flagging](#phase-4-on-chain-flagging)
6. [Phase 5: Configuration & Monitoring](#phase-5-configuration--monitoring)
7. [Spam Report Flow](#spam-report-flow)
8. [Monitoring Guide](#monitoring-guide)

---

## Architecture

```javascript
┌─────────────────┐
│  P2P Gateway    │
│  (Receive)      │
└────────┬────────┘
         │
         ▼
    ┌─────────┐
    │ Flagged?│──Yes──► Reject Immediately
    └────┬────┘
         │No
         ▼
    ┌─────────┐
    │Rate Limit│──Exceeded──► Track & Queue
    └────┬────┘
         │OK
         ▼
    ┌─────────┐
    │  Redis  │
    │  Queue  │
    └────┬────┘
         │
         ▼
┌─────────────────┐
│    Dequeuer     │
└────────┬────────┘
         │
         ▼
    ┌─────────┐
    │Signature│──Invalid──► Track Failure
    │ Verify  │
    └────┬────┘
         │Valid
         ▼
    ┌─────────┐
    │  Slot   │──Mismatch──► Track Failure
    │ Validate│
    └────┬────┘
         │Match
         ▼
    ┌─────────┐
    │ Process │
    └─────────┘

Failure Tracking:
    ┌─────────────┐
    │Spam Tracker │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │Threshold?   │──Yes──► Broadcast Report
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │Validator    │
    │   Mesh      │
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │ Consensus?  │──Yes──► Flag On-Chain
    └──────┬──────┘
           │
           ▼
    ┌─────────────┐
    │All Validators│
    │   Reject     │
    └─────────────┘
```

---

## Phase 1: Tracking Layer

### 1.0 Peer ID Whitelisting

**CRITICAL**: Two categories of peers must be whitelisted by Peer ID:

1. **Full Nodes**: Currently whitelisted by snapshotter signer addresses (`FULL_NODE_ADDRESSES`), but also need Peer ID whitelisting
   - These nodes can submit unlimited submissions per epoch
   - Bypass rate limiting but NOT validation checks
   - Environment variable: `FULL_NODE_PEER_IDS` (comma-separated list of libp2p peer IDs)

2. **Bulk Service Snapshotters**: Run by delegators who submit on behalf of other snapshotters
   - Submissions carry unique snapshotter addresses (cannot whitelist by snapshotter address)
   - Must be whitelisted by Peer ID to bypass rate limits
   - Environment variable: `BULK_SERVICE_PEER_IDS` (comma-separated list of libp2p peer IDs)

**Whitelist Behavior**:
- Whitelisted Peer IDs bypass rate limiting (unlimited submissions per epoch)
- Whitelisted Peer IDs still undergo validation checks (signature verification, slot validation)
- Whitelisted Peer IDs are NOT tracked for spam reporting (no validation failure tracking, no submission count tracking)
- Whitelisted Peer IDs cannot be flagged (even if consensus reached, whitelist takes precedence)

### 1.1 Track Validation Failures

**CRITICAL**: Track by **Peer ID** (primary) and **Snapshotter Address** (secondary)
**IMPORTANT**: Skip tracking if Peer ID is whitelisted (`FULL_NODE_PEER_IDS` or `BULK_SERVICE_PEER_IDS`)

- **Primary tracking**: Failed validations per `(peerID, epochID)` in Redis
- Key: `{protocol}:{market}:spam:validation_failures:peer:{peerID}:{epochID}`
- Increment on ANY validation failure from this peer
- **What constitutes a "validation failure"** (technical errors that cause submission rejection):
  1. **Invalid submission format/structure** - `validateSubmission()` fails (malformed JSON, missing fields, etc.)
  2. **Signature verification failure** - `verifySignature()` fails (invalid EIP-712 signature, wrong signer, etc.)
  3. **Slot validation failure** - `ValidateSnapshotterForSlot()` fails when `ENABLE_SLOT_VALIDATION=true` (snapshotter not registered for the slot)
- **TTL**: Default expiry of 2 hours (set on first increment)

**Important**: Rate limit violations are **NOT** validation failures. They are tracked separately:
- Validation failures = technical errors (submission is invalid/rejected)
- Rate limit violations = too many **successful** submissions (submissions that passed all validations)

### 1.2 Track Submissions Per Epoch Per Peer

**CRITICAL**: Track by **Peer ID** (primary identifier for DDoS protection)
**IMPORTANT**: Skip tracking and rate limiting if Peer ID is whitelisted (`FULL_NODE_PEER_IDS` or `BULK_SERVICE_PEER_IDS`)

- **Primary tracking**: Submission count per `(peerID, epochID)`
- Key: `{protocol}:{market}:spam:submissions:peer:{peerID}:snapshotter:{snapshotterAddr}:epoch:{epochID}`
- Increment on successful signature verification (before slot validation)
- Check against limit: 2 submissions/epoch for lite nodes (unless peer is whitelisted by Peer ID)
- **TTL**: Default expiry of 24 hours (set on first increment)

### 1.3 Spam Tracker Component

**Thresholds** (HARDCODED - not configurable for consensus consistency):

```go
const (
    // Maximum validation failures per epoch before reporting
    // Triggers immediately when threshold reached in a single epoch (no consecutive requirement)
    MAX_VALIDATION_FAILURES_PER_EPOCH = 5
    
    // Maximum submissions per epoch for lite nodes (full nodes bypass)
    MAX_SUBMISSIONS_PER_EPOCH_LITE = 2
    
    // Number of consecutive epochs with rate limit violations before reporting
    // Rate limit violations require this many consecutive epochs with violations
    CONSISTENT_VIOLATIONS_THRESHOLD = 3
    
    // Minimum validators needed for consensus (hardcoded for consistency)
    SPAM_CONSENSUS_THRESHOLD = 2  // 2 out of 3 validators
    
    // Aggregation window size (hardcoded for consensus consistency)
    DEFAULT_AGGREGATION_WINDOW_SIZE = 10
)
```

**Rationale**: Thresholds must be identical across all validators for consensus to work. Making them configurable would break consensus if validators have different values.

**Violation Reporting Logic**:

1. **Validation Failures** (immediate reporting):
   - **What it is**: Technical errors that cause submission rejection (invalid format, bad signature, wrong slot)
   - **When tracked**: Called `TrackValidationFailure()` when submission fails validation
   - **Reporting threshold**: If `validationFailureCount >= 5` in **any single epoch** → report immediately
   - **No consecutive epochs required** - triggers immediately when threshold reached
   - **Example**: Epoch 100 has 5 validation failures (bad signatures, invalid formats) → report at epoch 100

2. **Rate Limit Violations** (consecutive epochs required):
   - **What it is**: Too many **successful** submissions (submissions that passed all validations)
   - **When tracked**: Called `TrackSubmissionCount()` AFTER successful signature verification
   - **Reporting threshold**: If `submissionCount > 2` in current epoch, check previous epochs
   - **Report only if**: **all 3 consecutive epochs** (including current) have violations
   - **Example**: 
     - Epochs 100, 101, 102 all have >2 successful submissions → report at epoch 102 ✓
     - Epoch 101 has ≤2 submissions → no report (breaks consecutive chain) ✗

**Key Distinction**:
- **Validation failures** = submissions that FAILED validation (rejected due to errors)
- **Rate limit violations** = submissions that PASSED validation but exceed rate limit (too many successful submissions)
- These are **separate tracking mechanisms** - a peer can have validation failures OR rate limit violations OR both

---

## Phase 2: Validator Coordination Layer

### 2.1 Spam Report Broadcasting

**Topic Choice**: Dedicated topic: `{prefix}/spam-reports` (validator-only)
**IMPORTANT**: Skip spam report generation if Peer ID is whitelisted (`FULL_NODE_PEER_IDS` or `BULK_SERVICE_PEER_IDS`)

- When threshold exceeded, check if peer is whitelisted FIRST
- If whitelisted, skip report generation (whitelist takes precedence)
- Broadcast to validator mesh via dedicated topic: `{prefix}/spam-reports`
- Reports include both Peer ID (primary) and Snapshotter Address (secondary) for comprehensive flagging

**Spam Report Structure**:

```go
type SpamReport struct {
    PeerID          string   // PRIMARY: libp2p peer ID
    SnapshotterAddr string   // Secondary: Ethereum address
    ViolationType   string   // "validation_failure" or "rate_limit"
    EpochID         uint64   // Epoch where violation occurred
    Count           int      // Number of violations/submissions
    Evidence        []string // Details (e.g., "validation_failures: 5")
    ReporterID      string   // Validator ID that generated this report
    Timestamp       int64    // Unix timestamp
}
```

### 2.2 Spam Report Aggregation

**CRITICAL**: Aggregate by **Peer ID** (primary identifier), not snapshotter address

- Receive spam reports from other validators
- **Aggregate reports per epoch**: `(peerID, epochID)` - PRIMARY aggregation key
- Track which validators reported (for consensus)
- **Aggregation Windows**: Use 10-epoch windows (hardcoded for consensus consistency)

### 2.3 Consensus Mechanism & On-Chain Flagging

**Window-Based Aggregation**:
- At end of aggregation window (when `epochID % 10 == 0`):
  1. For each peer with reports in the window:
     - **CRITICAL**: Check if peer is whitelisted FIRST (whitelisted peers cannot be flagged)
     - If whitelisted, skip flagging (whitelist takes precedence over consensus)
     - Check if reports received from >= `SPAM_CONSENSUS_THRESHOLD` validators (hardcoded: 2)
  2. If consensus reached for a peer (and peer is NOT whitelisted):
     - **Update on-chain flagged state** (permanent until cleared)
     - Store peer ID and associated snapshotter addresses
  3. **Update Redis cache** from on-chain state

**On-Chain State Management**:
- **On-chain state is permanent** (no TTL concept)
- **Clearing mechanism**:
  - Time-based expiration: Clear entries where `flaggedAt + 7 days < now` (configurable)
  - Manual clearing: Admin function to clear specific peers/addresses
  - Automatic clearing: When `lastFlaggedEpoch + N epochs < currentEpoch` (e.g., N=100 epochs)
- **State synchronization**: On node startup, query on-chain contract and populate Redis cache

---

## Phase 3: Enforcement Layer

### 3.1 P2P Gateway Enforcement

- In `processSubmissionMessage`, before queuing:
  1. Extract **Peer ID** from message: `msg.ReceivedFrom` (always available)
  2. **Check Peer ID whitelist FIRST** (whitelisted peers bypass all enforcement)
  3. Check flagged peers (only for non-whitelisted peers)
  4. If peer is flagged, drop message immediately (log warning, don't queue)
  5. Track dropped messages for metrics

**Why Peer ID check in Gateway?**
- Peer ID is available immediately from `msg.ReceivedFrom` (no signature verification needed)
- Early rejection prevents malicious peer from flooding Redis queue
- Catches all submissions from flagged peer, regardless of which snapshotter address they use

### 3.2 Dequeuer Enforcement

- In `ProcessSubmission`, after signature verification:
  1. Extract Peer ID from metadata (already stored by P2P Gateway)
  2. Check flagged peers (defense in depth - state can change between Gateway and Dequeuer)
  3. Check flagged snapshotters (after signature verification - Gateway can't check this)
  4. Check rate limit per peer per epoch (Gateway doesn't enforce rate limits)
  5. Track all failures for spam reporting (by peer ID and snapshotter address)

**Why Dequeuer checks are needed even though Gateway already filters:**
- **State changes**: Peer can be flagged AFTER Gateway accepts but BEFORE Dequeuer processes (race condition)
- **Snapshotter address checks**: Gateway only checks peer ID, but Dequeuer can check snapshotter address AFTER signature verification
- **Rate limiting**: Gateway doesn't enforce rate limits, only Dequeuer does
- **Whitelist check NOT needed**: Gateway already checked whitelist before queuing, and whitelist is static (doesn't change)

### 3.3 Rate Limiting Per Peer

- Track submissions per peer ID per epoch (PRIMARY - peer-based enforcement)
- Track submissions per snapshotter address per epoch (SECONDARY - for evidence)
- Enforce limits:
  - Lite nodes: 2 submissions/epoch (unless Peer ID is whitelisted)
  - Whitelisted Peer IDs: No limit (unlimited submissions)
- **CRITICAL**: Check Peer ID whitelist BEFORE applying rate limits
- **Consecutive Epochs Requirement**: Rate limit violations require `CONSISTENT_VIOLATIONS_THRESHOLD` (3) consecutive epochs with violations before reporting
  - Current epoch must have >2 submissions
  - Previous 2 epochs must also have had >2 submissions
  - Only then is a spam report generated
  - This prevents false positives from temporary spikes

---

## Phase 4: On-Chain Flagging

### 4.1 On-Chain Flagging Service

**On-Chain Flagging** (REQUIRED - not optional):
- When consensus reached at end of aggregation window:
  1. **Flag on-chain** (primary source of truth)
  2. **Sync Redis cache** from on-chain state
  3. Log flagging event with evidence

### 4.2 State Synchronization

**On Node Startup**:
1. Query on-chain contract for all flagged peers
2. Populate Redis cache
3. Set cache TTL: 24 hours (refresh periodically)

**Periodic Sync** (every hour or on epoch transition):
1. Query on-chain contract for changes since last sync
2. Update Redis cache with new/changed flagged peers
3. Remove cleared peers from Redis cache

---

## Phase 5: Configuration & Monitoring

### 5.1 Environment Variables

```bash
# Spam Protection
ENABLE_SPAM_PROTECTION=true  # Master switch
ENABLE_SPAM_REPORT_BROADCAST=true  # Enable validator coordination
SPAM_REPORT_TOPIC=  # Dedicated topic for spam reports (empty = auto-construct from validator presence prefix + "/spam-reports")
# Example: If GOSSIPSUB_VALIDATOR_PRESENCE_TOPIC=/powerloom/validator/presence,
#          then spam report topic will be /powerloom/validator/spam-reports

# Peer ID Whitelisting (comma-separated lists of libp2p peer IDs)
FULL_NODE_PEER_IDS=QmPeerID1,QmPeerID2,QmPeerID3  # Full node Peer IDs
BULK_SERVICE_PEER_IDS=QmBulkPeerID1,QmBulkPeerID2  # Bulk service Peer IDs

# Cache TTL (hours)
SPAM_CACHE_TTL_HOURS=24

# NOTE: The following configs are for FUTURE on-chain implementation (not yet used):
# On-Chain State Clearing (FUTURE - not yet implemented)
# SPAM_FLAG_EXPIRY_DAYS=7  # Days before flagged peers can be cleared
# SPAM_FLAG_EXPIRY_EPOCHS=100  # Epochs before flagged peers can be cleared

# State Synchronization (FUTURE - not yet implemented)
# SPAM_SYNC_ON_STARTUP=true  # Sync flagged state from on-chain on node startup
# SPAM_SYNC_INTERVAL_HOURS=6  # Periodic sync interval (hours)
```

**Note**: `SPAM_AGGREGATION_WINDOW_SIZE` is hardcoded to 10 for consensus consistency and is not configurable.

### 5.2 Monitoring & Metrics

Track metrics:
- `spam_reports_sent`: Counter
- `spam_reports_received`: Counter
- `spam_consensus_reached`: Counter
- `spam_submissions_rejected`: Counter (by reason)
- `spam_validation_failures`: Counter (per snapshotter)

---

## Spam Report Flow

### Overview

The spam protection system uses a two-phase approach:
1. **Immediate Reporting**: When thresholds are exceeded in an epoch, reports are immediately broadcast
2. **Window-Based Aggregation**: Reports are collected over 10-epoch windows before checking consensus

### Phase 1: Within-Epoch Reporting

#### When Reports Are Generated

A spam report is **immediately broadcast** when a peer exceeds thresholds in the current epoch:

**1. Validation Failures (Immediate)**
- **Trigger**: `validationFailureCount >= 5` in **any single epoch**
- **Example**: 
  - Epoch 100: Peer submits 5 submissions with invalid signatures
  - **Result**: Spam report broadcast immediately at epoch 100
- **No consecutive epochs required** - triggers as soon as threshold reached

**2. Rate Limit Violations (Consecutive Epochs Required)**
- **Trigger**: `submissionCount > 2` in current epoch **AND** previous 2 epochs also had violations
- **Example**:
  - Epoch 98: 3 submissions (>2) ✓
  - Epoch 99: 3 submissions (>2) ✓
  - Epoch 100: 3 submissions (>2) ✓
  - **Result**: Spam report broadcast at epoch 100 (3 consecutive epochs)
- **If epoch 99 had ≤2 submissions**: No report (breaks consecutive chain)

#### Report Generation Flow

```
Dequeuer.ProcessSubmission()
    ↓
TrackSubmissionCount() or TrackValidationFailure()
    ↓
CheckAndReport() called after tracking
    ↓
ShouldReportSpam() checks thresholds
    ↓
If threshold exceeded → ReportSpam() broadcasts to validator mesh
```

### Phase 2: Aggregation Windows

#### Window Calculation

Reports are aggregated into **10-epoch windows**. The window ID is the **end epoch ID** of each window (round up to next multiple of 10):

```
windowID = ((epochID + 9) / 10) * 10
```

**Note**: Epoch 0 is dummy/heartbeat only and is never processed. Windows start from epoch 1.

**Simpler explanation**: Round up the epoch ID to the next multiple of 10:
- Epoch 1-10 → Window 10
- Epoch 11-20 → Window 20
- Epoch 21-30 → Window 30

**Window IDs**: 10, 20, 30, 40... (each window represents a 10-epoch range)

**Window Mapping**:
- **Window 10**: Epochs 1-10
- **Window 20**: Epochs 11-20
- **Window 30**: Epochs 21-30
- **Window 40**: Epochs 31-40
- And so on...

**Boundary Detection**: The modulus (`epochID % 10 == 0`) is used **only** to detect when we're at a window boundary, not to calculate the window ID.

#### Aggregation Process

When a spam report is received:

1. **Calculate Window ID**: `windowID = ((epochID + 9) / 10) * 10` (round up to next multiple of 10)
   - Epoch 1-10 → Window 10
   - Epoch 11-20 → Window 20
   - Epoch 21-30 → Window 30
   - etc.
   - **Note**: Epoch 0 is dummy/heartbeat only, not processed
2. **Store in Window**: Reports are stored per peer per window:
   - Key: `{protocol}:{market}:spam:reports:peer:{peerID}:window:{windowID}`
   - Window peers set: `{protocol}:{market}:spam:reports:window:{windowID}:peers` (SET)
3. **Track Validators**: Each unique validator that reports the same peer is counted
4. **Track Snapshotters**: Associated snapshotter addresses are collected
5. **Track Frequency**: Count total reports per peer across the window

#### Aggregation Metrics

For each peer in a window, the aggregated state tracks:
- **Validator Count**: Number of unique validators reporting this peer
- **Report Count**: Total number of reports (frequency of violations)
- **Epoch Range**: First and last epoch where violations occurred
- **Snapshotter Addresses**: All snapshotter addresses associated with violations
- **Violation Types**: Types of violations reported (validation_failure, rate_limit)

**Key Principle**: The aggregation focuses on **frequency and consistency** of violations:
- How many times was this peer reported? (frequency)
- How many different validators reported it? (consensus)
- Over how many epochs did violations occur? (consistency)

### Phase 3: Consensus Check (End of Window)

#### When Consensus is Checked

**CRITICAL**: Consensus check and flagging (blacklisting) **ONLY happens at the END of the window**, not while the window is still active.

**During the window** (epochs 1-10, 11-20, etc.):
- Reports are collected and aggregated
- **NO consensus checking**
- **NO flagging/blacklisting**
- Just accumulation of evidence

**At window boundary** (`epochID % 10 == 0`), the system checks the **completed window**:

```
At epoch 10: Window 10 (epochs 1-10) is complete → Check consensus → Flag if threshold reached
At epoch 20: Window 20 (epochs 11-20) is complete → Check consensus → Flag if threshold reached
At epoch 30: Window 30 (epochs 21-30) is complete → Check consensus → Flag if threshold reached
```

**Key Principle**: 
- **During window**: Collect and aggregate reports (no decisions made)
- **At boundary**: Finalize aggregated state, check consensus, flag/blacklist if threshold reached
- This ensures decisions are based on the complete 10-epoch window, not partial data

#### Consensus Threshold

- **Required**: Reports from `>= 2` validators (hardcoded `SPAM_CONSENSUS_THRESHOLD = 2`)
- **What counts**: The frequency and consistency of violations across the 10-epoch window
  - Multiple validators reporting the same peer = consensus
  - Same validator reporting multiple times = frequency (but not consensus)
  - Violations across multiple epochs = consistency

#### Consensus Check Flow

**During Window 10** (epochs 1-10):
- Reports are received and aggregated
- **No consensus checking**
- **No flagging/blacklisting**
- Just accumulation

**At epoch 10** (`10 % 10 == 0` - window boundary):
1. **Window 10 is now complete** (all epochs 1-10 have passed)
2. **Finalize aggregated state** for window 10
3. **Get all peers** with reports in window 10 (using deterministic SET: `{protocol}:{market}:spam:reports:window:{windowID}:peers`)
4. For each peer:
   - Get aggregated report (contains validatorCount, reportCount, etc.)
   - Skip if whitelisted
   - Check if `validatorCount >= 2`
   - If yes → **Flag peer on-chain (blacklist)**
5. **Clear/Reset** window 10 (or mark as processed) to start fresh for window 20

#### Example: Full Flow

**Cycle 1: Epochs 1-10** (Window 10):
- Epoch 3: Validator A reports peer `QmPeer123` (validation failures) → Window 10
- Epoch 5: Validator B reports peer `QmPeer123` (rate limit) → Window 10
- Epoch 5: Validator C reports peer `QmPeer123` (validation failures) → Window 10
- Epoch 7: Validator A reports peer `QmPeer123` again → Window 10

**At Epoch 10** (`10 % 10 == 0`):
- **Check window 10** (all epochs 1-10):
  - Window 10 contains 3 validators (A, B, C) reporting peer `QmPeer123`
  - `validatorCount = 3 >= 2` → **Consensus reached!** → Flag peer
- **Result**: Peer `QmPeer123` flagged on-chain due to consensus in window 10

**No Consensus Example**:
- **Epochs 1-10** (Window 10):
  - Epoch 3: Validator A reports peer `QmPeer123` → Window 10
  - Epoch 5: Validator A reports peer `QmPeer123` again → Window 10 (same validator)

- **At Epoch 10**:
  - Check window 10: Contains 1 validator (A) → No consensus (need 2 different validators)
  - **Result**: No consensus reached

### Key Points

1. **Reports are immediate**: Generated as soon as thresholds are exceeded in an epoch
2. **Aggregation is window-based**: Reports collected over 10-epoch windows (window IDs: 10, 20, 30...)
3. **Consensus is delayed**: Checked **ONLY at epoch boundaries** (`epochID % 10 == 0`)
4. **No flagging during window**: During the window (epochs 1-10, 11-20, etc.), reports are collected but **NO consensus checking** and **NO flagging/blacklisting** happens
5. **Flagging only at boundary**: Consensus checking and flagging (blacklisting) **ONLY happens at the END of the window** when the window is complete
6. **Window ID = End Epoch**: Window ID is the end epoch of the 10-epoch range (not modulus result)
7. **Modulus for boundaries only**: `epochID % 10` is used only to detect boundaries, not to calculate window ID
8. **Consensus requires 2 validators**: At least 2 different validators must report the same peer in the same window
9. **Frequency matters**: The aggregation tracks frequency of violations (how many times reported) and consistency (across how many epochs)
10. **Window calculation**: Code uses `windowID = ((epochID + 9) / 10) * 10` to round up to next multiple of 10 (10, 20, 30...)
11. **Epoch 0 is dummy**: Epoch 0 is only for heartbeat/simulation, never processed. Windows start from epoch 1.

---

## Monitoring Guide

### Feature Status

✅ **IMPLEMENTED** - All phases completed:
- ✅ Phase 1: Tracking Layer (spam tracker, rate limiter, whitelist)
- ✅ Phase 2: Validator Coordination (spam reporter, aggregator)
- ✅ Phase 3: Enforcement Layer (P2P Gateway + Dequeuer)
- ✅ Phase 4: On-Chain Flagging (flagging service, sync service)
- ✅ Phase 5: Configuration & Monitoring (environment variables, metrics)

### Prometheus Integration

**Yes, the DSV node has Prometheus metrics support**. Each component exposes metrics on its own port:
- Unified Sequencer: Port 9092 (configurable via `METRICS_PORT`)
- P2P Gateway: Port 9093
- State Tracker: Port 9094
- Aggregator: Port 9091
- Monitor API: Port 9096

The spam protection system integrates with the existing Prometheus metrics infrastructure via the `pkgs/metrics` package.

### Prometheus Metrics

The spam protection system exposes the following Prometheus metrics:

#### Spam Reports
- `spam_reports_sent_total` - Counter: Total spam reports sent by this validator
- `spam_reports_received_total` - Counter: Total spam reports received from other validators
- `spam_reports_invalid_total` - Counter: Total invalid spam reports received
- `spam_reports_ignored_whitelisted_total` - Counter: Spam reports ignored because peer is whitelisted

#### Consensus & Flagging
- `spam_consensus_reached_total` - Counter: Number of times consensus was reached to flag a peer
- `spam_flagging_success_total` - Counter: Successful peer/snapshotter flagging operations
- `spam_flagging_failed_total` - Counter: Failed peer/snapshotter flagging operations

#### Enforcement
- `spam_submissions_dropped_gateway_total` - Counter: Submissions dropped at P2P Gateway due to flagged peer
- `spam_submissions_dropped_dequeuer_total` - Counter: Submissions dropped at Dequeuer due to spam protection
- `spam_submissions_rejected_total` - Counter: Total submissions rejected due to spam protection (by reason)

### Querying Metrics

```bash
# Check spam reports sent
curl http://localhost:9090/metrics | grep spam_reports_sent_total

# Check consensus reached
curl http://localhost:9090/metrics | grep spam_consensus_reached_total

# Check submissions dropped
curl http://localhost:9090/metrics | grep spam_submissions_dropped
```

### Redis Key Monitoring

#### Per-Epoch Tracking Keys

Monitor validation failures and submission counts per peer:

```bash
# Check validation failures for a peer in an epoch
redis-cli GET "{protocol}:{market}:spam:validation_failures:peer:{peerID}:{epochID}"

# Check submission count for a peer in an epoch
redis-cli GET "{protocol}:{market}:spam:submissions:peer:{peerID}:{epochID}"

# Check peer-snapshotter associations
redis-cli SMEMBERS "{protocol}:{market}:spam:peer_snapshotter_map:{peerID}:{epochID}"
```

#### Aggregation Window Keys

Monitor aggregated spam reports:

```bash
# Check aggregated reports for a peer in a window
redis-cli GET "{protocol}:{market}:spam:reports:peer:{peerID}:window:{windowID}"

# Get all peers with reports in a window (deterministic SET)
redis-cli SMEMBERS "{protocol}:{market}:spam:reports:window:{windowID}:peers"
```

#### Flagged State Keys

Monitor flagged peers and snapshotters:

```bash
# Check if peer is flagged (cache)
redis-cli GET "{protocol}:{market}:spam:consensus_flagged:peer:{peerID}"

# Check if snapshotter is flagged (cache)
redis-cli GET "{protocol}:{market}:spam:consensus_flagged:snapshotter:{snapshotterAddr}"

# List all flagged peers (quick lookup)
redis-cli SMEMBERS "flagged_peers:{dataMarket}"

# List all flagged snapshotters (quick lookup)
redis-cli SMEMBERS "flagged_snapshotters:{dataMarket}"
```

#### Active Validators

Monitor active validators for consensus calculation:

```bash
# List active validators
redis-cli SMEMBERS "{protocol}:{market}:active:validators"

# Count active validators
redis-cli SCARD "{protocol}:{market}:active:validators"
```

### Log Monitoring

#### Key Log Messages

**P2P Gateway**:
- `"Whitelisted peer {peerID} bypasses P2P Gateway spam checks"` - Whitelisted peer detected
- `"🚫 P2P Gateway: Dropping submission from flagged peer {peerID}"` - Submission dropped due to flagged peer
- `"Rejected submission from flagged peer: {peerID}"` - Flagged peer rejection

**Dequeuer**:
- `"Whitelisted peer {peerID} bypasses spam checks and rate limiting"` - Whitelisted peer detected
- `"Rejected submission from flagged peer: {peerID}"` - Flagged peer rejection
- `"Rejected submission from flagged snapshotter: {snapshotterAddr}"` - Flagged snapshotter rejection
- `"Rate limit exceeded for peer {peerID} (epoch {epochID}): {count} submissions (max {max})"` - Rate limit exceeded
- `"Validation failure tracked for peer {peerID} (snapshotter {snapshotterAddr}, epoch {epochID}): count = {count}"` - Validation failure tracked

**Spam Reporter**:
- `"🚨 Broadcasted spam report for peer {peerID} (epoch {epochID}, reason: {reason}, count: {count})"` - Spam report broadcast

**Spam Aggregator**:
- `"Received spam report from {reporterID} for peer {peerID} (epoch {epochID}, reason: {reason})"` - Report received
- `"🚩 Consensus reached for peer {peerID} (validators: {validatorCount})"` - Consensus reached
- `"🚩 Successfully flagged peer {peerID} on-chain and in cache."` - Peer flagged

**Flagging Service**:
- `"Peer {peerID} and associated snapshotters flagged in Redis (expires in {days} days)"` - Flagging successful
- `"Cannot flag whitelisted peer {peerID}"` - Attempt to flag whitelisted peer blocked

### Log Filtering

```bash
# Filter spam protection logs
grep -i "spam\|flagged\|whitelist\|rate limit" /var/log/dsv-node.log

# Filter P2P Gateway spam logs
grep "P2P Gateway.*spam\|flagged\|whitelist" /var/log/dsv-node.log

# Filter Dequeuer spam logs
grep "Dequeuer.*spam\|flagged\|rate limit" /var/log/dsv-node.log

# Filter consensus logs
grep "consensus\|Consensus\|CONSENSUS" /var/log/dsv-node.log
```

### Monitoring Checklist

#### Daily Checks

- [ ] Check spam reports sent/received metrics
- [ ] Monitor flagged peer count: `redis-cli SCARD "flagged_peers:{dataMarket}"`
- [ ] Check consensus reached count: `curl http://localhost:9090/metrics | grep spam_consensus_reached_total`
- [ ] Review submissions dropped metrics
- [ ] Check active validators count

#### Weekly Checks

- [ ] Review aggregation window keys for patterns
- [ ] Check on-chain flagged state synchronization
- [ ] Verify whitelist configuration (FULL_NODE_PEER_IDS, BULK_SERVICE_PEER_IDS)
- [ ] Review validation failure patterns
- [ ] Check rate limit enforcement effectiveness

### Alerting Thresholds

Set up alerts for:

1. **High Spam Report Rate**: `spam_reports_sent_total` > 100/hour
2. **Consensus Reached**: `spam_consensus_reached_total` increases (immediate alert)
3. **High Drop Rate**: `spam_submissions_dropped_gateway_total` > 50/hour
4. **Flagged Peer Count**: `SCARD "flagged_peers:{dataMarket}"` > 100
5. **Active Validators**: `SCARD "{protocol}:{market}:active:validators"` < 2 (consensus may fail)

### Troubleshooting

#### Issue: No spam reports being sent

**Check**:
1. Is `ENABLE_SPAM_PROTECTION=true`?
2. Is `ENABLE_SPAM_REPORT_BROADCAST=true`?
3. Are thresholds being exceeded? Check validation failure and submission counts
4. Are peers whitelisted? Whitelisted peers don't generate reports

#### Issue: Consensus not reached

**Check**:
1. Are enough validators active? `redis-cli SCARD "{protocol}:{market}:active:validators"`
2. Are validators receiving reports? Check `spam_reports_received_total`
3. Is aggregation window size correct? Default is 10 epochs (hardcoded)
4. Are reports being aggregated? Check aggregation window keys

#### Issue: Flagged peers not being rejected

**Check**:
1. Is Redis cache synced? Check flagged state keys
2. Is on-chain state synced? Check sync logs
3. Are whitelist checks happening first? Whitelisted peers bypass flagging
4. Are enforcement checks enabled? Check P2P Gateway and Dequeuer logs

#### Issue: Rate limits not enforced

**Check**:
1. Is peer whitelisted? Whitelisted peers bypass rate limits
2. Is spam protection enabled? `ENABLE_SPAM_PROTECTION=true`
3. Are submission counts being tracked? Check Redis keys
4. Is rate limiter initialized? Check Dequeuer initialization logs

### Monitoring API Endpoints

The monitor-api component exposes REST endpoints for spam protection information:

#### Endpoints

- `GET /api/v1/spam/flagged/peers` - List all flagged peer IDs with metadata
- `GET /api/v1/spam/flagged/snapshotters` - List all flagged snapshotter addresses with metadata
- `GET /api/v1/spam/peer/:peerID` - Get spam tracking info for a specific peer (validation failures, submission counts, aggregation info)
- `GET /api/v1/spam/stats` - Get aggregated spam protection statistics (flagged counts, active validators)

#### Example Usage

```bash
# Get all flagged peers
curl http://localhost:8080/api/v1/spam/flagged/peers

# Get spam info for a specific peer
curl http://localhost:8080/api/v1/spam/peer/QmPeerID123?epochID=12345

# Get spam protection statistics
curl http://localhost:8080/api/v1/spam/stats
```

---

## Key Design Decisions

1. **Primary Identifier: Peer ID**: Peer ID is the primary identifier for DDoS protection (libp2p identity, harder to spoof)
2. **Aggregation Windows**: Use 10-epoch aggregation windows (hardcoded for consensus consistency)
3. **On-Chain State Management**: On-chain state is source of truth (permanent until cleared)
4. **Dual Enforcement**: P2P Gateway (early rejection) + Dequeuer (defense in depth)
5. **Peer ID Whitelisting**: Two categories of peers whitelisted by Peer ID (Full Nodes and Bulk Service Snapshotters)
6. **Deterministic Redis Keys**: All aggregation uses deterministic SET keys (no SCAN operations)

## Implementation Status

✅ **COMPLETED** - All phases implemented:
- ✅ Phase 1: Tracking Layer (spam tracker, rate limiter, whitelist)
- ✅ Phase 2: Validator Coordination (spam reporter, aggregator)
- ✅ Phase 3: Enforcement Layer (P2P Gateway + Dequeuer)
- ✅ Phase 4: On-Chain Flagging (flagging service, sync service)
- ✅ Phase 5: Configuration & Monitoring (environment variables, metrics)

## Files Created/Modified

**New Files**:
- `decentralized-sequencer/pkgs/spam/tracker.go`
- `decentralized-sequencer/pkgs/spam/reporter.go`
- `decentralized-sequencer/pkgs/spam/aggregator.go`
- `decentralized-sequencer/pkgs/spam/rate_limiter.go`
- `decentralized-sequencer/pkgs/spam/flagging.go`
- `decentralized-sequencer/pkgs/spam/sync.go`
- `decentralized-sequencer/pkgs/spam/whitelist.go`
- `decentralized-sequencer/pkgs/spam/metrics.go`
- `decentralized-sequencer/pkgs/spam/init.go`

**Modified Files**:
- `decentralized-sequencer/pkgs/submissions/dequeuer.go`
- `decentralized-sequencer/cmd/p2p-gateway/main.go`
- `decentralized-sequencer/cmd/unified/main.go`
- `decentralized-sequencer/config/settings.go`
- `decentralized-sequencer/.env.example`
- `decentralized-sequencer/pkgs/redis/keys.go`

## Related Documentation

- [Redis Keys Reference](../docs/REDIS_KEYS.md) - Redis key structure documentation
- [Monitoring Guide](../docs/MONITORING_GUIDE.md) - General monitoring documentation
- [DSV Node Setup Guide](../docs/DSV_NODE_SETUP.md) - Node deployment guide

