# DDoS Protection Monitoring Guide

Practical guide for monitoring and debugging the DSV DDoS protection system against spam and illegitimate submissions.

## Quick Debugging: Why Are Aggregation Windows Not Being Created?

If `/api/v1/spam/windows` returns empty, follow these steps:

### Step 0: Verify DDoS Protection Components Initialized

**CRITICAL FIRST STEP**: Check if components initialized correctly at startup.

**Dequeuer Component** (local tracking only):
```bash
# Check initialization logs in dequeuer (local tracking components)
./dsv.sh dequeuer-logs | grep -iE "(initializing spam|spam protection components initialized)"

# Expected logs:
# - "Initializing spam protection components" with fields:
#   - enable_spam_protection: true
#   - enable_spam_report_broadcast: false (dequeuer doesn't do P2P)
#   - pubsub_available: false (expected - dequeuer doesn't initialize P2P)
# - "✅ Spam protection components initialized (local tracking only - P2P handled by spam-aggregator component)"
#   - tracker_initialized: true
#   - rate_limiter_initialized: true
#   - flagging_initialized: true
#   - reporter_initialized: false (expected - no P2P in dequeuer)
#   - aggregator_initialized: true (local aggregator instance, but no P2P)
```

**Spam-Aggregator Component** (Redis queue-based P2P):
```bash
# Check initialization logs in spam-aggregator (Redis queue-based)
./dsv.sh spam-aggregator-logs | grep -iE "(spam aggregator component starting|initializing spam|spam protection components initialized|redis queue)"

# Expected logs:
# - "🛡️  SPAM AGGREGATOR COMPONENT STARTING"
# - "✅ Connected to Redis"
# - "✅ P2P operations handled via p2p-gateway (Redis queue-based)"
# - "Initializing spam protection components (Redis queue-based P2P)" with fields:
#   - enable_spam_protection: true
#   - enable_spam_report_broadcast: true
# - "Initialized spam aggregator with window size: 10 (Redis queue-based P2P)"
# - "Initialized spam reporter (Redis queue-based broadcasting with epoch batching)"
# - "Initialized spam report window manager (collection window: {duration}, consensus delay: {duration})"
# - "✅ Spam protection components initialized" with component status:
#   - aggregator_initialized: true (MUST be true)
#   - reporter_initialized: true (MUST be true - uses Redis queues)
```

**If spam-aggregator logs show errors:**
- Check environment variables: `ENABLE_SPAM_PROTECTION=true` and `ENABLE_SPAM_REPORT_BROADCAST=true`
- Check Redis connection is working (spam-aggregator uses Redis queues, not direct P2P)
- Verify p2p-gateway is running (handles all P2P operations for spam reports)
- Check Redis queue keys: `{protocol}:{market}:outgoing:spam-reports` and `{protocol}:{market}:incoming:spam-reports`

### Step 1: Check if EventMonitor is Processing Epochs

**Event-Monitor Component** (triggers window aggregation at epoch boundaries):
```bash
# Check initialization logs in event-monitor
./dsv.sh event-logs | grep -iE "(spam protection components initialized|event monitor started|initializing spam)"

# Check if epoch boundaries are being detected
./dsv.sh event-logs | grep -iE "(EpochReleased|epoch.*released|aggregation window boundary|epoch.*boundary|window boundary)"

# Expected logs when epoch boundary detected:
# - "📅 Epoch {epochID} released for market {address} at block {block}"
# - "Epoch {epochID} is an aggregation window boundary. Triggering local spam data aggregation."
# - "Calling CreateWindowAndAggregateLocalData for epoch {epochID}"
# - "Successfully created spam aggregation window for epoch {epochID}"
# - "Started spam report collection window for epoch {epochID}"
# - "⏱️ Spam report collection window closed for epoch {epochID}, collecting and sending reports"
# - "✅ Sent {count} batched spam reports for epoch {epochID}"
# - "⏳ Scheduling consensus check for window {epochID} after {duration} delay (waiting for other validators' reports)"
# - "🔍 Checking consensus for window {epochID} after {duration} delay"
# - "Checking consensus for window {epochID} after {duration}-second delay"

# If you see warnings instead:
# - "Spam components not initialized - skipping window creation" → Check spam component initialization
# - "Spam aggregator is nil at epoch boundary" → Aggregator not initialized
# - "Spam tracker is nil at epoch boundary" → Tracker not initialized
```

### Step 2: Check if Tracking is Happening

```bash
# Check dequeuer logs for tracking activity
./dsv.sh dequeuer-logs | grep -iE "(tracked.*submission|tracked.*validation|epoch.*peers|spam.*tracker|peer.*empty)"

# OR
./dsv.sh dequeuer-logs | grep -iE "(tracked.*submission|tracked.*validation|epoch.*peers|spam.*tracker|peer.*empty)"
# Enable debug logging if needed:
# LOG_LEVEL=debug in your docker-compose env

# Look for:
# - "Tracked submission for peer {peerID} epoch {epochID}"
# - "Tracked validation failure for peer {peerID} epoch {epochID}"
# - "Peer ID is empty - skipping DDoS protection tracking"
# - "Spam tracker is nil"
```

### Step 3: Check Current Epoch and Window Boundaries

```bash
# Get current epoch (endpoint returns array, get first epoch's ID)
CURRENT_EPOCH=$(curl -s "http://localhost:9091/api/v1/epochs/active" | jq -r '.[0].epoch_id')
echo "Current epoch: $CURRENT_EPOCH"

# Check if current epoch is a boundary (should be multiple of 10)
# Windows are created when epochID % 10 == 0 (epochs 10, 20, 30, etc.)
echo "Is boundary: $(( $CURRENT_EPOCH % 10 == 0 ))"
```

### Step 4: Check Redis for Epoch Tracking Data

```bash
# Replace {protocol} and {market} with your values
PROTOCOL="your-protocol"
MARKET="your-market"

# Check if epoch peer sets exist (shows tracking is happening)
docker exec <redis-container> redis-cli KEYS "${PROTOCOL}:${MARKET}:spam:epoch:*:peers"

# Check if windows master set exists
docker exec <redis-container> redis-cli SMEMBERS "${PROTOCOL}:${MARKET}:spam:reports:windows"
```

### Step 5: Check Monitoring API for Epoch Activity

```bash
# List epochs with tracking data
# NOTE: This endpoint returns empty unless an epoch boundary has been hit (windows created)
curl "http://localhost:9091/api/v1/spam/epochs?limit=20" | jq '.'

# If this returns empty, either:
# - No epoch boundaries have been hit yet (epochID % 10 != 0)
# - Tracking isn't happening
# If this returns epochs, check if any are multiples of 10
```

### Step 6: Verify Event-Monitor Window Creation

**Event-monitor triggers window aggregation at epoch boundaries** (`epochID % 10 == 0`):

```bash
# Check event-monitor logs for epoch boundary detection and window creation
./dsv.sh event-logs | grep -iE "(EpochReleased|aggregation window boundary|Calling CreateWindowAndAggregateLocalData|Successfully created spam aggregation window)"

# Expected logs:
# - "📅 Epoch {epochID} released for market {address} at block {block}"
# - "Epoch {epochID} is an aggregation window boundary. Triggering local spam data aggregation."
# - "Calling CreateWindowAndAggregateLocalData for epoch {epochID}"
# - "Successfully created spam aggregation window for epoch {epochID}" (GOOD)
# - "Failed to create spam aggregation window at epoch boundary {epochID}: {error}" (BAD - check error)

# NOTE: Window creation logs appear in event-monitor logs, not spam-aggregator logs
# Event-monitor calls CreateWindowAndAggregateLocalData on its own spam components instance
# Check event-monitor logs for window creation details:
./dsv.sh event-logs | grep -iE "(Creating spam aggregation window|Created spam aggregation window)"

# Expected logs in event-monitor:
# - "Creating spam aggregation window {windowID} for epochs {start}-{end}"
# - "Created spam aggregation window {windowID} with {peers_count} peers"
```

**Check consensus checking (after 10-second delay) - ONLY at epoch boundaries**:
```bash
# These logs only appear at epoch boundaries (epochID % 10 == 0)
./dsv.sh event-logs | grep -iE "(Waiting.*seconds|checking consensus|CheckWindowForConsensus)"
./dsv.sh spam-aggregator-logs | grep -iE "(Checking consensus|consensus.*reached|flagged.*peer)"
```

**Verify Local Aggregation and Broadcasting**:

Since event-monitor handles window creation, verify aggregation and broadcasting separately:

**1. Check if local reports are being generated** (in event-monitor/dequeuer):
```bash
# Check for local report generation (INFO level) - only appears when violations occur
./dsv.sh event-logs | grep -iE "(Spam check.*shouldReport=true|Stored.*spam report)"
./dsv.sh dequeuer-logs | grep -iE "(Spam check.*shouldReport=true|Stored.*spam report)"

# Look for:
# Regular peer reports:
# - "⚠️ Spam check for peer {peerID} epoch {epochID}: shouldReport=true, violationType={type}, reason={reason} (peer ID and associated snapshotter addresses will be flagged after consensus)"
#   - reason can be: "immediate (failures: {N} >= threshold: {M})" for validation failures
#   - reason can be: "consecutive (failures: {N} >= threshold: {M} for {K} consecutive epochs >= threshold: {L})" for consecutive violations
# Bulk service peer snapshotter reports:
# - "⚠️ Spam check for bulk service peer {peerID} snapshotter {snapshotterAddr} epoch {epochID}: shouldReport=true, violationType=rate_limit_snapshotter (snapshotter address will be flagged after consensus)"
# Report storage:
# - "📝 Stored peer ID spam report for peer {peerID} epoch {epochID} (will flag peer ID and associated snapshotter addresses after consensus)" (regular peers)
# - "📝 Stored snapshotter address spam report for bulk service peer {peerID} snapshotter {snapshotterAddr} epoch {epochID} (will flag snapshotter address only after consensus)" (bulk service peers)
# NOTE: These logs only appear when spam violations are detected, not at every epoch
```

**2. Check if reports are being queued for broadcasting** (in event-monitor/dequeuer):
```bash
# Check for report storage and collection window activity
./dsv.sh event-logs | grep -iE "(Stored spam report|Started spam report collection|Sent.*batched spam reports|collection window closed|Scheduling consensus check)"
./dsv.sh dequeuer-logs | grep -iE "(Stored spam report|Started spam report collection|Sent.*batched spam reports)"

# Look for:
# - "📝 Stored peer ID spam report for peer {peerID} epoch {epochID} (will flag peer ID and associated snapshotter addresses after consensus)" (INFO level, regular peers)
# - "📝 Stored snapshotter address spam report for bulk service peer {peerID} snapshotter {snapshotterAddr} epoch {epochID} (will flag snapshotter address only after consensus)" (INFO level, bulk service peers)
# - "⏰ Started spam report collection window for epoch {epochID} (will send reports after {delay})" (INFO level)
# - "⏱️ Spam report collection window closed for epoch {epochID}, collecting and sending reports" (INFO level)
# - "✅ Sent {count} batched spam reports for epoch {epochID}" (INFO level)
# - "⏳ Scheduling consensus check for window {epochID} after {duration} delay" (INFO level, window boundaries only)
```

**3. Check Redis queues directly**:
```bash
# Replace {protocol} and {market} with your values
PROTOCOL="0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401"
MARKET="0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d"

# Check outgoing queue (reports waiting to be broadcast)
docker exec <redis-container> redis-cli LLEN "${PROTOCOL}:${MARKET}:outgoing:spam-reports"

# Check incoming queue (reports received from other validators)
docker exec <redis-container> redis-cli LLEN "${PROTOCOL}:${MARKET}:incoming:spam-reports"

# If queues have items, spam-aggregator should be processing them
```

**4. Check spam-aggregator processing incoming reports**:
```bash
# Check if spam-aggregator is receiving and processing reports from Redis queue
./dsv.sh spam-aggregator-logs | grep -iE "(📨 Received spam report|Atomically aggregated|Error reading from spam reports queue)"

# Look for:
# - "📨 Received spam report from validator {validatorID} for peer {peerID} epoch {epochID}" (INFO level)
# - "Atomically aggregated spam report for peer {peerID} (window {windowID}, validators: {count})" (DEBUG level)
# - "Error reading from spam reports queue" (ERROR level - indicates queue reading issues)

# If no logs appear, spam-aggregator may not be receiving reports (check Redis queue above)
```

**5. Check p2p-gateway broadcasting**:
```bash
# Check p2p-gateway logs for spam report P2P operations
./dsv.sh p2p-logs | grep -iE "(spam report|broadcasted spam|queued incoming spam|Broadcasted spam report)"

# Look for:
# - "Broadcasted spam report via Gossipsub" (DEBUG level)
# - "Queued incoming spam report for spam-aggregator" (DEBUG level)
# - "Failed to broadcast spam report" (ERROR level)
```

**6. Check if windows exist in Redis** (verifies aggregation happened):
```bash
# Check if windows were created
curl "http://localhost:9091/api/v1/spam/windows" | jq '.windows[] | {window_id, epoch_range, peer_count}'

# If windows exist but spam-aggregator shows no activity:
# - Local aggregation happened (event-monitor created windows)
# - But no reports were generated/broadcast (check steps 1-2 above)
```

## Monitoring API Endpoints

### Check Windows Status

```bash
# List all aggregation windows
curl "http://localhost:9091/api/v1/spam/windows" | jq '.'

# Expected: If windows exist, you'll see:
# {
#   "windows": [
#     {"window_id": 30, "epoch_range": "21-30", "peer_count": 2},
#     {"window_id": 20, "epoch_range": "11-20", "peer_count": 1}
#   ],
#   "count": 2
# }
```

### Check Window Details

```bash
# Get details for a specific window (shows all peers and their aggregated reports)
WINDOW_ID=24189520  # Replace with your window ID
curl "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq '.'

# Response includes:
# - window_id: Window ID (end epoch)
# - epoch_range: Epochs covered (e.g., "24189511-24189520")
# - peers: Array of peer details including:
#   - peer_id: Peer ID
#   - validator_count: Number of validators that reported this peer (for consensus, requires >= 2)
#   - first_epoch: First epoch with violations
#   - last_epoch: Last epoch with violations
#   - report_count: Number of spam reports
#   - reports: Array of individual spam reports with epoch_id, violation_type, count, evidence

# Summary view - see all peers, validator counts, and report counts
curl "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq '{
  window_id: .window_id,
  epoch_range: .epoch_range,
  peer_count: .peer_count,
  peers: [.peers[] | {
    peer_id,
    validator_count,
    report_count,
    first_epoch,
    last_epoch,
    violation_types: [.reports[].violation_type] | unique
  }]
}'

# Check reports for a specific epoch within the window
EPOCH_ID=24189520
curl "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq ".peers[].reports[] | select(.epoch_id == ${EPOCH_ID})"

# Check consensus status (peers with >= 2 validators reporting)
curl "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq '.peers[] | select(.validator_count >= 2) | {peer_id, validator_count, report_count}'
```

## Drilling Down: Detailed Investigation

Once you have windows, here's how to investigate further:

### Step 1: Get Window Details

```bash
# Get full details for a window (replace with your window ID)
WINDOW_ID=24182100
curl "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq '.'
```

This shows:
- All peers in the window
- Validator count per peer (consensus indicator)
- Epoch range of violations
- Individual spam reports

### Step 2: Check Specific Peer Activity

```bash
# Get peer tracking for a specific epoch
PEER_ID="12D3KooWKYSAndoFZEBENnFV9wi5CwPVWeT9ArUYnUZR5CA3Qc5L"
EPOCH_ID=24182098
curl "http://localhost:9091/api/v1/spam/peer/${PEER_ID}?epochID=${EPOCH_ID}" | jq '.'
```

### Step 3: Check Peer Activity Across Epochs

**Endpoint**: `GET /api/v1/spam/peer/{peerID}/epochs`

**Query Parameters**:
- `startEpoch` (optional): Start epoch (default: current epoch - 10)
- `endEpoch` (optional): End epoch (default: current epoch)
- `protocol` (optional): Protocol state identifier
- `market` (optional): Data market address

**Example**:
```bash
# Get epoch-by-epoch tracking for a peer within a window
PEER_ID="12D3KooWKYSAndoFZEBENnFV9wi5CwPVWeT9ArUYnUZR5CA3Qc5L"
WINDOW_ID=24182100
START_EPOCH=$((WINDOW_ID - 9))  # Window start
END_EPOCH=$WINDOW_ID            # Window end

curl "http://localhost:9091/api/v1/spam/peer/${PEER_ID}/epochs?startEpoch=${START_EPOCH}&endEpoch=${END_EPOCH}" | jq '.'
```

**Response**:
```json
{
  "peer_id": "12D3KooW...",
  "epochs": [
    {
      "epoch_id": 24182091,
      "submission_count": 5,
      "validation_failures": 0,
      "snapshotter_addresses": ["0x1234...", "0x5678..."]
    },
    {
      "epoch_id": 24182092,
      "submission_count": 3,
      "validation_failures": 2,
      "snapshotter_addresses": ["0x1234..."]
    }
  ],
  "count": 2,
  "start_epoch": 24182091,
  "end_epoch": 24182100,
  "timestamp": "2026-01-07T10:00:00Z"
}
```

**What it shows**:
- Submission count per epoch
- Validation failures per epoch
- Snapshotter addresses used per epoch

### Step 4: Check Redis Directly for Aggregated Data

```bash
PROTOCOL=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401
MARKET=0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d
WINDOW_ID=24182100
PEER_ID="12D3KooWKYSAndoFZEBENnFV9wi5CwPVWeT9ArUYnUZR5CA3Qc5L"

# Get aggregated report for a peer in a window
docker exec snapshot-sequencer-validator-redis-1 redis-cli GET "${PROTOCOL}:${MARKET}:spam:reports:peer:${PEER_ID}:window:${WINDOW_ID}" | jq '.'

# Get all peers in a window
docker exec snapshot-sequencer-validator-redis-1 redis-cli SMEMBERS "${PROTOCOL}:${MARKET}:spam:reports:window:${WINDOW_ID}:peers"
```

### Step 5: Check Submission Counts Per Epoch

```bash
PROTOCOL=0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401
MARKET=0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d
PEER_ID="12D3KooWKYSAndoFZEBENnFV9wi5CwPVWeT9ArUYnUZR5CA3Qc5L"
EPOCH_ID=24182098

# Get submission count for a peer in an epoch
docker exec snapshot-sequencer-validator-redis-1 redis-cli GET "${PROTOCOL}:${MARKET}:spam:submissions:peer:${PEER_ID}:${EPOCH_ID}"

# Get validation failure count
docker exec snapshot-sequencer-validator-redis-1 redis-cli GET "${PROTOCOL}:${MARKET}:spam:validation_failures:peer:${PEER_ID}:${EPOCH_ID}"
```

### Step 6: Check Consensus Status

```bash
# Check if consensus was reached (validator_count >= 2)
# This is shown in window details endpoint
curl "http://localhost:9091/api/v1/spam/windows/24182100" | jq '.peers[] | select(.validator_count >= 2)'

# Check if peer is flagged (consensus reached and flagged)
curl "http://localhost:9091/api/v1/spam/flagged/peers" | jq '.'
```

**Important**: A peer will only be flagged if:
1. **Spam reports are generated** (requires 3 consecutive epochs with violations for rate limits)
2. **Consensus is reached** (>= 2 validators report the same peer in the same window)
3. **Window boundary passed** (consensus check happens at epochID % 10 == 0, after a 10-second delay to allow all validator reports to arrive)

If only 1 validator has the new code, `validator_count` will be 1, so consensus won't be reached and the peer won't be flagged. Check logs for spam report generation:

```bash
# Check if spam reports are being generated
./dsv.sh dequeuer-logs | grep -iE "(broadcasted spam report|shouldReport=true|spam check for peer)"

# Look for:
# - "⚠️ Spam check for peer {peerID} epoch {epochID}: shouldReport=true, violationType=rate_limit, reason={reason} (peer ID and associated snapshotter addresses will be flagged after consensus)"
# - "📢 Broadcasted spam report for peer {peerID}"
```

### Step 7: Understanding Why a Peer Isn't Flagged

**Scenario**: Peer exceeds rate limit (>2 submissions/epoch) but isn't flagged.

**Check 1: Are spam reports being generated?**
```bash
PEER_ID="12D3KooWKYSAndoFZEBENnFV9wi5CwPVWeT9ArUYnUZR5CA3Qc5L"
./dsv.sh dequeuer-logs | grep -iE "spam check for peer.*${PEER_ID}" | grep "shouldReport=true"
```

**Check 2: What's the validator count in the window?**
```bash
WINDOW_ID=24182120
curl "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq ".peers[] | select(.peer_id == \"${PEER_ID}\") | {peer_id, validator_count, report_count}"
```

**Possible reasons peer isn't flagged:**
- **No consensus**: `validator_count < 2` (need >= 2 validators to report)
- **Reports not generated**: Need 3 consecutive epochs with violations before first report
- **Window not checked yet**: Consensus check happens at window boundary (epochID % 10 == 0)
- **Peer is whitelisted**: Check `FULL_NODE_PEER_IDS` and `BULK_SERVICE_PEER_IDS` in env (whitelisted peers are NOT tracked at all)

### Step 8: Check Which Epochs Have Activity

```bash
# List all epochs with tracking data
curl "http://localhost:9091/api/v1/spam/epochs?limit=50" | jq '.epochs[] | select(.epoch_id >= 24182091 and .epoch_id <= 24182100)'
```

### Check Epoch Tracking

```bash
# List epochs with tracking data
# NOTE: This endpoint returns empty unless an epoch boundary has been hit (windows created)
curl "http://localhost:9091/api/v1/spam/epochs?limit=20" | jq '.'

# Shows which epochs have peer activity (only epochs that are part of created windows)
```

### Check Peer Tracking

**Single Epoch** (`GET /api/v1/spam/peer/{peerID}?epochID={epochID}`):
```bash
# Get tracking for a specific peer in an epoch
PEER_ID="12D3KooW..."
EPOCH_ID=24176205
curl "http://localhost:9091/api/v1/spam/peer/${PEER_ID}?epochID=${EPOCH_ID}" | jq '.'
```

**Multiple Epochs** (`GET /api/v1/spam/peer/{peerID}/epochs`):
```bash
# Get epoch-by-epoch tracking for a peer across multiple epochs
PEER_ID="12D3KooW..."
curl "http://localhost:9091/api/v1/spam/peer/${PEER_ID}/epochs?startEpoch=24176200&endEpoch=24176210" | jq '.'
```

See **Step 3** above for detailed endpoint documentation.

### Check Flagged Peers

```bash
# List all flagged peers
curl "http://localhost:9091/api/v1/spam/flagged/peers" | jq '.'

# List all flagged snapshotters
curl "http://localhost:9091/api/v1/spam/flagged/snapshotters" | jq '.'
```

### Check Stats

```bash
# Get DDoS protection statistics
curl "http://localhost:9091/api/v1/spam/stats" | jq '.'
```

## Log Monitoring

### Key Log Messages to Look For

**Component Initialization (CRITICAL - Check at Startup)**:

**Dequeuer Component (Local Tracking Only)**:
```bash
./dsv.sh dequeuer-logs | grep -i "initializing spam\|spam protection components initialized"
```
Look for:
- `"Initializing spam protection components"` with fields:
  - `enable_spam_protection: true`
  - `enable_spam_report_broadcast: false` (expected - dequeuer doesn't do P2P)
  - `pubsub_available: false` (expected - dequeuer doesn't initialize P2P)
- `"✅ Spam protection components initialized (local tracking only - P2P handled by spam-aggregator component)"` with:
  - `tracker_initialized: true`
  - `rate_limiter_initialized: true`
  - `flagging_initialized: true`
  - `reporter_initialized: false` (expected - no P2P in dequeuer)
  - `aggregator_initialized: true` (local instance, but no P2P)

**Spam-Aggregator Component (Redis Queue-Based P2P)**:
```bash
./dsv.sh spam-aggregator-logs | grep -i "spam aggregator component starting\|initializing spam\|redis queue"
```
Look for:
- `"🛡️  SPAM AGGREGATOR COMPONENT STARTING"`
- `"✅ Connected to Redis"`
- `"✅ P2P operations handled via p2p-gateway (Redis queue-based)"`
- `"Initializing spam protection components (Redis queue-based P2P)"` with fields:
  - `enable_spam_protection: true`
  - `enable_spam_report_broadcast: true`
- `"Initialized spam aggregator with window size: 10 (Redis queue-based P2P)"` (GOOD)
- `"Initialized spam reporter (Redis queue-based broadcasting with epoch batching)"`
- `"Initialized spam report window manager (collection window: {duration}, consensus delay: {duration})"`
- `"✅ Spam protection components initialized"` with:
  - `aggregator_initialized: true` (MUST be true)
  - `reporter_initialized: true` (MUST be true - uses Redis queues)

**P2P Gateway (Early Rejection)**:
```bash
./dsv.sh p2p-logs | grep -i "initialized spam protection"
```
Look for:
- `"Initialized spam protection: whitelist ({N} full nodes, {M} bulk service), flagging service"`

**Event-Monitor Component (Window Creation)**:
```bash
./dsv.sh event-logs | grep -iE "(aggregation window|epoch.*boundary|creating window|window.*aggregated|EpochReleased)"
```
Look for:
- `"📅 Epoch {epochID} released for market {market} at block {block}"`
- `"Epoch {epochID} is an aggregation window boundary. Triggering local spam data aggregation."`
- `"Calling CreateWindowAndAggregateLocalData for epoch {epochID}"`
- `"Successfully created spam aggregation window for epoch {epochID}"` (GOOD)
- `"Failed to create spam aggregation window at epoch boundary {epochID}: {error}"` (BAD - check error)
- `"Spam components not initialized - skipping window creation"` (BAD - check initialization)

**Spam-Aggregator Component (Window Aggregation)**:
```bash
./dsv.sh spam-aggregator-logs | grep -iE "(aggregated.*window|window.*aggregated|creating window)"
```
Look for:
- `"Aggregated local data for window {windowID}: {peerCount} peers, {reportCount} reports"`

**Dequeuer (DDoS Protection Tracking)**:
```bash
./dsv.sh dequeuer-logs | grep -iE "(tracked|spam|validation failure|peer.*empty|spam.*tracker.*nil|bulk service)"
```
Look for:
- **Regular peer tracking** (DEBUG level):
  - `"Tracked submission for peer {peerID} epoch {epochID} (count: {N})"` (regular peers)
- **Bulk service peer tracking** (DEBUG level):
  - `"Tracked submission for bulk service peer {peerID} snapshotter {snapshotterAddr} epoch {epochID} (snapshotter_count: {N})"` (bulk service peers tracked by snapshotter address)
- **Validation failure tracking** (DEBUG level):
  - `"Tracked validation failure for peer {peerID} epoch {epochID} (count: {N})"` (regular peers)
- **Error logs**:
  - `"Peer ID is empty - skipping DDoS protection tracking"`
  - `"Spam tracker is nil (DDoS protection enabled but tracker not initialized)"`
  - `"DDoS protection disabled - skipping tracking"`


**Spam-Aggregator (Redis Queue-Based Broadcasting and Aggregation)**:
```bash
./dsv.sh spam-aggregator-logs | grep -iE "(queued|received|aggregated.*report|generated.*report|📨|waiting.*seconds|checking consensus)"
```
Look for:
- **Storing reports for batching** (INFO level):
  - `"📝 Stored peer ID spam report for peer {peerID} epoch {epochID} (will flag peer ID and associated snapshotter addresses after consensus)"` (regular peers, from reporter)
  - `"📝 Stored snapshotter address spam report for bulk service peer {peerID} snapshotter {snapshotterAddr} epoch {epochID} (will flag snapshotter address only after consensus)"` (bulk service peers, from reporter)
  - `"⏰ Started spam report collection window for epoch {epochID}"` (from window manager)
  - `"✅ Sent {count} batched spam reports for epoch {epochID}"` (from window manager after collection window)
- **Receiving reports from Redis queue** (INFO level):
  - `"📨 Received spam report from validator {validatorID} for peer {peerID} epoch {epochID}"` (when report received from another validator via Redis queue)
- **Processing reports** (INFO level):
  - `"Atomically aggregated spam report for peer {peerID} (window {windowID}, validators: {N})"` (peer ID reports)
  - `"Atomically aggregated spam report for snapshotter {snapshotterAddr} (window {windowID}, validators: {N})"` (snapshotter address reports from bulk service peers)
- **Generating local reports** (DEBUG level):
  - `"Generated local spam report for peer {peerID} epoch {epochID}"`
- **Window aggregation**:
  - `"Aggregated local data for window {windowID}: {peerCount} peers, {reportCount} reports"`
- **Consensus checking with delay**:
  - `"Waiting 10 seconds for validator reports before checking consensus for window {windowID}"`
  - `"Checking consensus for window {windowID} after 10-second delay"`
- **Consensus reached and flagging** (INFO level):
  - `"🚩 Consensus reached for peer {peerID} (validators: {N})"` (peer ID flagging)
  - `"🚩 Consensus reached for snapshotter address {snapshotterAddr} (validators: {N})"` (snapshotter address flagging for bulk service peers)
  - `"🚩 Flagged peer {peerID} with {N} snapshotter addresses"` (peer ID flagging)
  - `"🚩 Flagged snapshotter address {snapshotterAddr} (independent of peer ID)"` (snapshotter address flagging)

**P2P Gateway (Enforcement)**:
```bash
./dsv.sh p2p-logs | grep -iE "(flagged|whitelist|dropping)"
```
Look for:
- `"🚫 P2P Gateway: Dropping submission from flagged peer {peerID}"`
- `"Whitelisted peer {peerID} bypasses P2P Gateway spam checks"`

### Enable Debug Logging

Set `LOG_LEVEL=debug` in your docker-compose environment variables.

## Redis Key Checks

### Check Epoch Tracking

```bash
# Replace {protocol} and {market} with your values
PROTOCOL="your-protocol"
MARKET="your-market"

# Check if epoch peer sets exist (shows tracking is happening)
docker exec <redis-container> redis-cli KEYS "${PROTOCOL}:${MARKET}:spam:epoch:*:peers"

# Check a specific epoch
EPOCH_ID=24176205
docker exec <redis-container> redis-cli SMEMBERS "${PROTOCOL}:${MARKET}:spam:epoch:${EPOCH_ID}:peers"

# Check submission counts
docker exec <redis-container> redis-cli KEYS "${PROTOCOL}:${MARKET}:spam:submissions:peer:*"
```

### Check Windows

```bash
# Master windows set (should contain window IDs like "10", "20", "30")
docker exec <redis-container> redis-cli SMEMBERS "${PROTOCOL}:${MARKET}:spam:reports:windows"

# Check peers in a specific window
WINDOW_ID=24176210
docker exec <redis-container> redis-cli SMEMBERS "${PROTOCOL}:${MARKET}:spam:reports:window:${WINDOW_ID}:peers"

# Get aggregated report for a peer
PEER_ID="12D3KooW..."
docker exec <redis-container> redis-cli GET "${PROTOCOL}:${MARKET}:spam:reports:peer:${PEER_ID}:window:${WINDOW_ID}" | jq '.'
```

### Check Flagged State

```bash
# List flagged peers
docker exec <redis-container> redis-cli SMEMBERS "flagged_peers:${MARKET}"

# List flagged snapshotters
docker exec <redis-container> redis-cli SMEMBERS "flagged_snapshotters:${MARKET}"

# Check if specific peer is flagged
docker exec <redis-container> redis-cli GET "${PROTOCOL}:${MARKET}:spam:consensus_flagged:peer:${PEER_ID}"
```

## Configuration

### Required Environment Variables

```bash
# Master switch for DDoS protection (protects against spam and illegitimate submissions)
ENABLE_SPAM_PROTECTION=true

# Enable validator coordination (share DDoS reports with other validators)
ENABLE_SPAM_REPORT_BROADCAST=true

# Spam report topic (auto-constructs if empty)
SPAM_REPORT_TOPIC=

# Peer ID Whitelisting (comma-separated)
# Full nodes and bulk service snapshotters bypass rate limiting AND spam tracking
# IMPORTANT: Whitelisted peers are NOT tracked at all (no submission counts, no validation failures)
# They cannot be flagged even if consensus is reached
FULL_NODE_PEER_IDS=QmPeerID1,QmPeerID2,QmPeerID3
BULK_SERVICE_PEER_IDS=QmBulkPeerID1,QmBulkPeerID2

# Cache TTL (hours)
SPAM_CACHE_TTL_HOURS=24
```

### Hardcoded Thresholds

These are hardcoded for consensus consistency:

- `MAX_VALIDATION_FAILURES_PER_EPOCH = 5` (per peer ID)
- `MAX_SUBMISSIONS_PER_EPOCH_LITE = 2` (per peer ID, per epoch)
- `CONSISTENT_VIOLATIONS_THRESHOLD = 3` (consecutive epochs required for rate limit violations)
- `SPAM_CONSENSUS_THRESHOLD = 2` (2 out of 3 validators)
- `SPAM_AGGREGATION_WINDOW_SIZE = 10` (epochs per aggregation window)

**Window Creation**: Windows are created at epochs where `epochID % 10 == 0` (epochs 10, 20, 30, etc.)

**Consensus Checking with Delay**: After window creation at epoch boundary, the system waits 10 seconds before checking consensus. This delay ensures all validators' spam reports have time to arrive via P2P and be aggregated before consensus decisions are made. Look for log messages:
- `"Waiting 10 seconds for validator reports before checking consensus for window {windowID}"`
- `"Checking consensus for window {windowID} after 10-second delay"`

## Troubleshooting

### Windows Not Being Created

**Symptoms**: `/api/v1/spam/windows` returns empty, no windows in Redis

**Debug Steps**:

1. **Check spam-aggregator is running** (CRITICAL):
   ```bash
   ./dsv.sh status | grep spam-aggregator
   ```

2. **Check if epochs are being processed by spam-aggregator**:
   ```bash
   ./dsv.sh spam-aggregator-logs | tail -50 | grep -iE "(epoch.*released|epoch.*boundary)"
   ```

3. **Check if current epoch is a boundary**:
   ```bash
   CURRENT_EPOCH=$(curl -s "http://localhost:9091/api/v1/epochs/active" | jq -r '.[0].epoch_id')
   echo "Current epoch: $CURRENT_EPOCH"
   echo "Is boundary: $(( $CURRENT_EPOCH % 10 == 0 ))"
   ```

4. **Check if spam components are initialized in dequeuer**:
   ```bash
   ./dsv.sh dequeuer-logs | grep -iE "(spam.*component|spam.*initialized)"
   ```

5. **Check if tracking is happening in dequeuer**:
   ```bash
   # Enable debug logging first (LOG_LEVEL=debug in env)
   # Then check for tracking logs
   ./dsv.sh dequeuer-logs | grep -iE "(tracked.*submission|tracked.*validation|peer.*empty)"
   ```

6. **Check Redis for epoch peer sets**:
   ```bash
   docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:epoch:*:peers" | head -10
   ```

### No Tracking Data

**Symptoms**: `/api/v1/spam/epochs` returns empty, no epoch peer sets in Redis

**Note**: The `/api/v1/spam/epochs` endpoint queries epochs from windows (not epoch peer sets, which are deleted after aggregation). **This endpoint returns empty unless an epoch boundary has been hit** (epochID % 10 == 0). If windows exist but epochs endpoint is empty, check that windows contain epoch data.

**Debug Steps**:

1. **Check dequeuer is running and processing submissions**:
   ```bash
   ./dsv.sh status | grep dequeuer
   ./dsv.sh dequeuer-logs | tail -20 | grep -iE "(processing|tracked)"
   ```

2. **Verify spam protection is enabled**:
   ```bash
   ./dsv.sh dequeuer-logs | grep -iE "(spam.*protection.*enabled|ENABLE_SPAM_PROTECTION)"
   ```

3. **Check if submissions are being processed**:
   ```bash
   ./dsv.sh dequeuer-logs | grep -iE "(processed.*submission|worker.*processing|tracked.*submission)"
   ```

4. **Check if peers are whitelisted** (whitelisted peers are NOT tracked at all):
   ```bash
   # Check your env vars
   docker exec snapshot-sequencer-validator-dequeuer-1 env | grep FULL_NODE_PEER_IDS
   docker exec snapshot-sequencer-validator-dequeuer-1 env | grep BULK_SERVICE_PEER_IDS
   
   # If a peer is whitelisted, it will NOT appear in:
   # - /api/v1/spam/epochs (no tracking data)
   # - /api/v1/spam/windows (not included in windows)
   # - Redis spam tracking keys
   ```

5. **Check if peerID is being passed**:
   ```bash
   ./dsv.sh dequeuer-logs | grep -iE "(peer.*empty|peer_id)"
   ```

6. **Check if windows exist** (epochs endpoint queries from windows):
   ```bash
   curl "http://localhost:9091/api/v1/spam/windows" | jq '.windows[] | {window_id, epoch_range, peer_count}'
   ```

### No Spam Reports Being Sent

**Debug Steps**:

1. **Check thresholds are being exceeded**:
   ```bash
   docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:validation_failures:peer:*"
   docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:submissions:peer:*"
   ```

2. **Check if report broadcast is enabled**:
   ```bash
   docker exec snapshot-sequencer-validator-spam-aggregator-1 env | grep ENABLE_SPAM_REPORT_BROADCAST
   ```

3. **Check spam-aggregator logs for report generation and broadcasting**:
   ```bash
   ./dsv.sh spam-aggregator-logs | grep -iE "(broadcasting.*report|generated.*report|spam.*report)"
   ```

4. **Check dequeuer logs for tracking** (dequeuer doesn't broadcast, only tracks):
   ```bash
   ./dsv.sh dequeuer-logs | grep -i "tracked.*submission\|tracked.*validation"
   ```

## Window to Epoch Mapping

**Window ID = End epoch of the 10-epoch range**:
- Window 10: Epochs 1-10
- Window 20: Epochs 11-20
- Window 30: Epochs 21-30
- Window 40: Epochs 31-40

**Formula**: `windowID = ((epochID + 9) / 10) * 10`

**Example**:
```bash
EPOCH=25
WINDOW_ID=$(( (($EPOCH + 9) / 10) * 10 ))
echo "Epoch $EPOCH belongs to window $WINDOW_ID"
# Output: Epoch 25 belongs to window 30
```

## Whitelisted Peers: Understanding FULL_NODE_PEER_IDS and FULL_NODE_ADDRESSES

### Whitelisted Peer Behavior

When a peer is included in `FULL_NODE_PEER_IDS` or `BULK_SERVICE_PEER_IDS`:

**Spam Protection Behavior:**
- ✅ **Bypasses rate limiting** (unlimited submissions per epoch)
- ❌ **NOT tracked for spam** (no submission counts, no validation failures tracked)
- ❌ **Cannot be flagged** (even if consensus reached, whitelist takes precedence)
- ❌ **Will NOT appear in monitoring endpoints** (`/api/v1/spam/epochs`, `/api/v1/spam/windows`)
- ❌ **Will NOT appear in Redis spam tracking keys**

**Code Reference**: See `pkgs/spam/tracker.go`:
- `TrackValidationFailure()` returns early if peer is whitelisted (line 54-56)
- `TrackSubmissionCount()` returns early if peer is whitelisted (line 112-114)
- `ShouldReportSpam()` returns false if peer is whitelisted (line 190-192)

### FULL_NODE_PEER_IDS vs FULL_NODE_ADDRESSES

**FULL_NODE_PEER_IDS** (spam protection):
- Used by spam protection system (`pkgs/spam/`)
- Whitelists peers by their libp2p Peer ID
- Bypasses rate limiting AND disables spam tracking
- Environment variable: `FULL_NODE_PEER_IDS`

**FULL_NODE_ADDRESSES** (identity verification):
- Used by identity verifier (`pkgs/identity/verifier.go`)
- Marks snapshotter addresses as full nodes for identity verification
- Does NOT affect spam protection (separate system)
- Environment variable: `FULL_NODE_ADDRESSES`

**When Both Are Set:**
If a peer is in `FULL_NODE_PEER_IDS` AND its snapshotter address is in `FULL_NODE_ADDRESSES`:
- The peer bypasses spam tracking (because of `FULL_NODE_PEER_IDS`)
- The snapshotter address is marked as a full node in identity verification (because of `FULL_NODE_ADDRESSES`)
- These are independent systems - `FULL_NODE_ADDRESSES` does not affect spam protection

### Monitoring Whitelisted Peers

**Logs to Monitor:**

**1. Initialization Logs** (shows whitelist configuration at startup):
```bash
# Check spam component initialization logs
./dsv.sh spam-aggregator-logs | grep -iE "(initialized peer whitelist|whitelist)"
./dsv.sh event-logs | grep -iE "(initialized peer whitelist|whitelist)"
./dsv.sh dequeuer-logs | grep -iE "(initialized peer whitelist|whitelist)"

# Expected log:
# "Initialized peer whitelist: {N} full nodes, {M} bulk service snapshotters"
```

**2. P2P Gateway Logs** (shows when whitelisted peers bypass spam checks):
```bash
# Enable DEBUG logging first (LOG_LEVEL=debug in env)
# Then check for whitelist bypass messages
./dsv.sh p2p-logs | grep -iE "whitelisted peer.*bypassing"

# Expected log (DEBUG level):
# "Whitelisted peer {peerID} bypassing spam checks"
```

**3. Tracker/Reporter Logs** (whitelisted peers are silently skipped):
```bash
# Whitelisted peers will NOT generate these logs:
# - No "Tracked submission for peer {peerID}" logs
# - No "Tracked validation failure for peer {peerID}" logs
# - No "Spam check for peer {peerID}" logs
# - No "Generated local spam report" logs

# To verify a peer is NOT being tracked (indicating it might be whitelisted):
PEER_ID="12D3KooW..."
./dsv.sh dequeuer-logs | grep "Tracked.*${PEER_ID}"  # Should return empty
./dsv.sh event-logs | grep "spam report.*${PEER_ID}"  # Should return empty
```

**API Endpoints:**

**Note**: There is NO API endpoint that directly exposes whitelisted peer IDs. However, you can verify whitelisting indirectly:

**1. Check `/api/v1/spam/peer/{peerID}`** (returns empty/zero for whitelisted peers):
```bash
PEER_ID="12D3KooW..."
curl "http://localhost:9091/api/v1/spam/peer/${PEER_ID}?epochID=24211826" | jq '.'

# If whitelisted, response will show:
# {
#   "peer_id": "12D3KooW...",
#   "validation_failures": 0,
#   "submission_count": 0,
#   "snapshotter_addresses": []
# }
# 
# NOTE: This could also mean the peer simply hasn't submitted anything,
# so verify with logs and environment variables
```

**2. Check `/api/v1/spam/stats`** (does NOT include whitelist info):
```bash
curl "http://localhost:9091/api/v1/spam/stats" | jq '.'

# Response shows flagged counts, but NOT whitelist counts:
# {
#   "flagged_peers_count": 0,
#   "flagged_snapshotters_count": 0,
#   "active_validators_count": 3,
#   "timestamp": "..."
# }
```

**3. Verify Peer is NOT in Tracking Endpoints:**
```bash
# Whitelisted peers will NOT appear in:
# - /api/v1/spam/epochs (no tracking data)
# - /api/v1/spam/windows (not included in windows)
# - Redis keys: {protocol}:{market}:spam:submissions:peer:{peerID}:{epochID}
# - Redis keys: {protocol}:{market}:spam:validation_failures:peer:{peerID}:{epochID}
```

**Direct Verification Methods:**

**1. Check Environment Variables:**
```bash
# Check whitelist configuration
PEER_ID="12D3KooW..."
docker exec snapshot-sequencer-validator-dequeuer-1 env | grep FULL_NODE_PEER_IDS
docker exec snapshot-sequencer-validator-dequeuer-1 env | grep BULK_SERVICE_PEER_IDS

# Check if specific peer is in whitelist
docker exec snapshot-sequencer-validator-dequeuer-1 env | grep FULL_NODE_PEER_IDS | grep "${PEER_ID}"
```

**2. Check Redis (verify no tracking keys exist):**
```bash
PROTOCOL="0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401"
MARKET="0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d"
PEER_ID="12D3KooW..."
EPOCH_ID=24211826

# Check if tracking keys exist (should NOT exist for whitelisted peers)
docker exec <redis-container> redis-cli GET "${PROTOCOL}:${MARKET}:spam:submissions:peer:${PEER_ID}:${EPOCH_ID}"
docker exec <redis-container> redis-cli GET "${PROTOCOL}:${MARKET}:spam:validation_failures:peer:${PEER_ID}:${EPOCH_ID}"

# Both should return (nil) if peer is whitelisted
```

**Expected Behavior Summary for Whitelisted Peers:**
- ✅ Initialization log shows whitelist count
- ✅ P2P Gateway DEBUG logs show "Whitelisted peer {peerID} bypassing spam checks" when peer submits
- ❌ No tracking logs in dequeuer/event-monitor
- ❌ No spam reports generated
- ❌ No Redis keys created in spam tracking namespace
- ❌ Empty/zero values in `/api/v1/spam/peer/{peerID}` endpoint
- ❌ Peer does NOT appear in `/api/v1/spam/epochs` or `/api/v1/spam/windows`

## Rate Limiting Behavior: Understanding Enforcement vs Tracking

### Important: Rate Limiting is TRACKING Only, Not Enforcement

**Critical Understanding**: The rate limit (`MAX_SUBMISSIONS_PER_EPOCH_LITE = 2`) is used for **tracking and reporting**, NOT for **rejecting submissions**. Submissions exceeding the rate limit are still **accepted and processed**.

**Enforcement** only happens **AFTER** consensus flagging:
1. Violations are tracked (submissions > 2 per epoch)
2. After 3 consecutive epochs with violations, spam reports are generated
3. After consensus (>= 2 validators report), peer gets flagged
4. **Only then** are submissions from flagged peers rejected

### Scenario 1: Bulk Service Peer (in BULK_SERVICE_PEER_IDS)

**Situation**: Bulk service peer sends > 2 submissions per epoch for a specific snapshotter address (not in `FULL_NODE_ADDRESSES`)

**Behavior**:
- ✅ **Submissions are ACCEPTED** (no rate limit enforcement)
- ❌ **NO tracking** (whitelisted peer bypasses all tracking)
- ❌ **NO rate limit check** (whitelisted peer bypasses rate limiting)
- ❌ **NO spam reports generated** (not tracked)
- ❌ **Cannot be flagged** (whitelist takes precedence)

**Code Reference**: `pkgs/spam/tracker.go`:
- `TrackSubmissionCount()` returns early if peer is whitelisted (line 112-114)
- `TrackValidationFailure()` returns early if peer is whitelisted (line 54-56)
- `ShouldReportSpam()` returns false if peer is whitelisted (line 190-192)

**Key Point**: Since bulk service peers are whitelisted by Peer ID, they can send **unlimited submissions per epoch** for **any snapshotter address**, regardless of whether that snapshotter address is in `FULL_NODE_ADDRESSES`. The whitelist check happens **before** any tracking or rate limiting.

### Scenario 2: Regular Peer (NOT in FULL_NODE_PEER_IDS or BULK_SERVICE_PEER_IDS)

**Situation**: Regular peer sends > 2 submissions per epoch

**Behavior**:
- ✅ **Submissions are ACCEPTED** (rate limiting is tracking only, not enforcement)
- ✅ **Tracking happens** (submission count incremented per `(peerID, epochID)`)
- ✅ **Rate limit violation detected** (`count > MAX_SUBMISSIONS_PER_EPOCH_LITE`)
- ✅ **Violation tracked** (counted towards consecutive violations threshold)
- ⚠️ **After 3 consecutive epochs with violations**: Spam reports generated
- ⚠️ **After consensus (>= 2 validators report)**: Peer gets flagged
- 🚫 **After flagging**: Future submissions from this peer are rejected

**Code Reference**: `pkgs/submissions/dequeuer.go`:
- Line 183: Comment states "Rate limiting is NOT enforced here - only flagged peers (after consensus) are rejected"
- Line 190: `TrackSubmissionCount()` is called (not skipped for non-whitelisted peers)
- Line 196: `CheckAndReport()` checks if spam should be reported (requires 3 consecutive violations)

**Timeline Example**:
```
Epoch 1: Peer sends 3 submissions → Accepted, tracked (count=3, violation=1)
Epoch 2: Peer sends 3 submissions → Accepted, tracked (count=3, violation=2)
Epoch 3: Peer sends 3 submissions → Accepted, tracked (count=3, violation=3)
         → Spam report generated (3 consecutive violations)
Epoch 4: Other validators also report → Consensus reached → Peer flagged
Epoch 5: Peer sends submission → REJECTED (peer is flagged)
```

### Scenario 3: Bulk Service Peer with Misbehaving Snapshotter Address

**Situation**: Bulk service peer (in `BULK_SERVICE_PEER_IDS`) sends > MAX_SUBMISSIONS_PER_EPOCH_LITE submissions per epoch for a specific snapshotter address across CONSISTENT_VIOLATIONS_THRESHOLD consecutive epochs

**Current Behavior** (IMPLEMENTED):
- ✅ **Submissions are ACCEPTED** (bulk service peer ID is whitelisted, P2P Gateway allows)
- ✅ **Tracking happens** (snapshotter address tracked independently, peer ID tracking skipped)
- ✅ **Rate limit violation detected** (snapshotter address count > MAX_SUBMISSIONS_PER_EPOCH_LITE)
- ✅ **Violation tracked** (counted towards consecutive violations threshold per snapshotter address)
- ⚠️ **After CONSISTENT_VIOLATIONS_THRESHOLD consecutive epochs with violations**: Snapshotter address spam reports generated (violationType="rate_limit_snapshotter")
- ⚠️ **After consensus (>= 2 validators report)**: Snapshotter address gets flagged independently (peer ID remains whitelisted)
- 🚫 **After flagging**: Future submissions from this snapshotter address are rejected by dequeuer (even though peer ID is whitelisted)

**Monitoring Logs**:
```bash
# Check bulk service peer snapshotter tracking
./dsv.sh dequeuer-logs | grep -iE "bulk service.*snapshotter.*tracked"

# Check snapshotter address report generation
./dsv.sh event-logs | grep -iE "⚠️.*bulk service.*snapshotter.*shouldReport=true"

# Check snapshotter address report storage
./dsv.sh event-logs | grep -iE "📝.*Stored snapshotter address spam report"

# Check snapshotter address aggregation
./dsv.sh spam-aggregator-logs | grep -iE "aggregated.*snapshotter"

# Check snapshotter address consensus and flagging
./dsv.sh spam-aggregator-logs | grep -iE "Consensus reached for snapshotter address|Flagged snapshotter address"
```

**Timeline Example**:
```
Epoch 1: Bulk service peer sends 3 submissions for snapshotterAddr → Accepted, tracked by snapshotter (snapshotter_count=3, violation=1)
Epoch 2: Bulk service peer sends 3 submissions for snapshotterAddr → Accepted, tracked by snapshotter (snapshotter_count=3, violation=2)
Epoch 3: Bulk service peer sends 3 submissions for snapshotterAddr → Accepted, tracked by snapshotter (snapshotter_count=3, violation=3)
         → Snapshotter address spam report generated (3 consecutive violations)
Epoch 4: Other validators also report → Consensus reached → Snapshotter address flagged (peer ID remains whitelisted)
Epoch 5: Bulk service peer sends submission for snapshotterAddr → REJECTED by dequeuer (snapshotter address is flagged)
         → P2P Gateway allows (peer ID whitelisted), but dequeuer rejects (snapshotter address flagged)
```

**Code Reference**: `pkgs/spam/flagging.go`:
- `FlagPeer()` returns early if peer is whitelisted (line 47-51)
- When a peer is flagged, its associated snapshotter addresses are also flagged (line 79-98)
- But if the peer is whitelisted, nothing gets flagged

**How Flagged Snapshotter Addresses Are Populated**:

Flagged snapshotter addresses are stored in **Redis** (not environment variables):

1. **Redis SET**: `flagged_snapshotters:{dataMarket}` - Quick lookup set
2. **Individual keys**: `{protocol}:{market}:spam:consensus_flagged:snapshotter:{snapshotterAddr}` - Detailed metadata

**Population Flow**:
```
Spam reports received → Aggregated by peer ID → Consensus check → FlagPeer() called
                                                                    ↓
                                    Snapshotter addresses extracted from aggregated reports
                                                                    ↓
                                    Stored in Redis SET and individual keys
```

**Code Reference**: `pkgs/spam/flagging.go`:
- `FlagPeer()` stores snapshotter addresses in Redis SET (line 109-117)
- Snapshotter addresses come from `aggregated.SnapshotterAddrs` (line 476 in `aggregator.go`)
- `aggregated.SnapshotterAddrs` collects all unique snapshotter addresses from spam reports in a window (line 375-386 in `aggregator.go`)

**Do Spam Reports Include Snapshotter Addresses?**

**YES!** Spam reports include snapshotter addresses:
- `SpamReport` struct has `SnapshotterAddr string` field (line 18 in `reporter.go`)
- When reports are aggregated, snapshotter addresses are collected from each report
- The aggregator maintains a list of all unique snapshotter addresses associated with a peer ID in a window

**How Bulk Service Peer Snapshotter Tracking Works**:

**How it works**:
1. **Tracking**: For bulk service peers, `TrackSubmissionCount()` skips peer ID tracking but **still tracks by snapshotter address** (`tracker.go` line 134-158)
2. **Violation Detection**: `ShouldReportSpamForSnapshotter()` checks rate limit violations **per snapshotter address** (not peer ID) for bulk service peers (`tracker.go` line 295-320)
3. **Reporting**: When thresholds exceeded (CONSISTENT_VIOLATIONS_THRESHOLD consecutive epochs with > MAX_SUBMISSIONS_PER_EPOCH_LITE), snapshotter address reports are generated with `violationType="rate_limit_snapshotter"` (`reporter.go` line 148-182)
4. **Aggregation**: Reports are aggregated by snapshotter address in separate windows (`aggregator.go` line 272-278, 573-612)
5. **Consensus**: Consensus is checked per snapshotter address independently (`aggregator.go` line 573-612)
6. **Flagging**: `FlagSnapshotter()` flags snapshotter addresses independently without flagging the peer ID (`flagging.go` line 157-203)
7. **Enforcement**: 
   - **P2P Gateway**: Checks peer ID whitelist → Bulk service peer is whitelisted → **Allows submission through**
   - **Dequeuer**: Checks flagged snapshotter addresses (line 152-160) → Snapshotter address is flagged → **Rejects submission**

**Code Reference**:
- Tracker: `TrackSubmissionCount()` line 134-158 - Bulk service peers tracked by snapshotter address only
- Tracker: `ShouldReportSpamForSnapshotter()` line 295-320 - Checks snapshotter address violations
- Reporter: `CheckAndReport()` line 148-182 - Generates snapshotter address reports for bulk service peers
- Aggregator: `processSpamReportDirect()` line 272-278 - Aggregates snapshotter reports separately
- Aggregator: `CheckWindowForConsensus()` line 573-612 - Checks consensus per snapshotter address
- Flagging: `FlagSnapshotter()` line 157-203 - Flags snapshotter addresses independently
- Dequeuer: Line 152-160 - Checks `IsSnapshotterFlagged()` independently of peer ID

### Summary Table

| Peer Type | Rate Limit Check | Tracking | Submissions Accepted? | Can Be Flagged? |
|-----------|------------------|----------|----------------------|-----------------|
| **Bulk Service** (in `BULK_SERVICE_PEER_IDS`) | ✅ Checked (by snapshotter address) | ✅ Tracked (by snapshotter address) | ✅ Yes (even if > 2) | ✅ Yes (snapshotter address flagged after consensus) |
| **Full Node** (in `FULL_NODE_PEER_IDS`) | ❌ Bypassed | ❌ Not tracked | ✅ Yes (unlimited) | ❌ No |
| **Regular Peer** | ✅ Checked | ✅ Tracked | ✅ Yes (even if > 2) | ✅ Yes (after consensus) |

### Monitoring Rate Limit Violations

**To check if a regular peer is exceeding rate limits:**
```bash
PEER_ID="12D3KooW..."
EPOCH_ID=24211826

# Check submission count for a peer in an epoch
curl "http://localhost:9091/api/v1/spam/peer/${PEER_ID}?epochID=${EPOCH_ID}" | jq '.submission_count'

# If count > 2, violation is tracked (but submissions still accepted)
# Check consecutive violations:
curl "http://localhost:9091/api/v1/spam/peer/${PEER_ID}/epochs?startEpoch=$((EPOCH_ID-2))&endEpoch=${EPOCH_ID}" | jq '.epochs[] | {epoch_id, submission_count}'
```

**To check if violations are being reported:**
```bash
# Check if spam reports were generated (requires 3 consecutive violations)
./dsv.sh event-logs | grep -iE "⚠️.*spam check.*${PEER_ID}.*shouldReport=true"
./dsv.sh event-logs | grep -iE "📝.*Stored.*spam report.*${PEER_ID}"
./dsv.sh spam-aggregator-logs | grep -iE "generated.*report.*${PEER_ID}"
```

## Slot Validation Failures and Spam Protection

### How Slot Validation Failures Are Tracked

When `ENABLE_SLOT_VALIDATION=true`, slot validation failures are tracked as **validation failures** for spam protection:

**Code Reference**: `pkgs/submissions/dequeuer.go`:
- Line 164-179: Slot validation happens if `enableSlotValidation` is true
- Line 165: `ValidateSnapshotterForSlot()` checks if snapshotter is authorized for the slot
- Line 168: If validation fails, `TrackValidationFailure()` is called
- Line 175: Submission is rejected with error "slot validation failed"

**Current Thresholds**:
- **Immediate reporting**: `MAX_VALIDATION_FAILURES_PER_EPOCH = 5`
  - If a peer has >= 5 validation failures (including slot validation failures) in a **single epoch**, a spam report is generated immediately
- **Consecutive reporting**: `MAX_VALIDATION_FAILURES_PER_EPOCH_CONSECUTIVE = 2` and `CONSISTENT_VIOLATIONS_THRESHOLD = 3`
  - If a peer has >= 2 validation failures per epoch for >= 3 consecutive epochs, a spam report is generated
  - This detects persistent low-level abuse patterns

**Code Reference**: `pkgs/spam/tracker.go`:
- Line 263-266: Immediate validation failures check - `failureCount >= MAX_VALIDATION_FAILURES_PER_EPOCH`
- Line 268-276: Consecutive validation failures check - `failureCount >= MAX_VALIDATION_FAILURES_PER_EPOCH_CONSECUTIVE` for `CONSISTENT_VIOLATIONS_THRESHOLD` consecutive epochs
- Line 387-410: `CheckConsecutiveValidationFailures()` method checks consecutive epochs

**Monitoring Logs**:
```bash
# Check validation failure tracking
./dsv.sh dequeuer-logs | grep -iE "Tracked validation failure"

# Check validation failure reports (immediate)
./dsv.sh event-logs | grep -iE "⚠️.*Spam check.*validation_failure.*reason=immediate.*peer ID and associated snapshotter addresses will be flagged"

# Check validation failure reports (consecutive)
./dsv.sh event-logs | grep -iE "⚠️.*Spam check.*validation_failure.*reason=consecutive.*peer ID and associated snapshotter addresses will be flagged"
```

**Behavior Examples**:

**Immediate Reporting**:
```
Epoch 100: Peer submits 5 times with wrong slot assignments → 5 slot validation failures
         → ⚠️ Spam check: shouldReport=true, violationType=validation_failure, reason="immediate (failures: 5 >= threshold: 5) (peer ID and associated snapshotter addresses will be flagged after consensus)"
         → 📝 Stored peer ID spam report (will flag peer ID and associated snapshotter addresses after consensus)
```

**Consecutive Reporting**:
```
Epoch 100: Peer submits 2 times with wrong slot assignments → 2 failures (below immediate threshold)
Epoch 101: Peer submits 2 times with wrong slot assignments → 2 failures (consecutive=2)
Epoch 102: Peer submits 2 times with wrong slot assignments → 2 failures (consecutive=3)
         → ⚠️ Spam check: shouldReport=true, violationType=validation_failure, reason="consecutive (failures: 2 >= threshold: 2 for 3 consecutive epochs >= threshold: 3) (peer ID and associated snapshotter addresses will be flagged after consensus)"
         → 📝 Stored peer ID spam report (will flag peer ID and associated snapshotter addresses after consensus)
```

**What Counts as Validation Failures**:
1. Invalid submission format/structure (malformed JSON, missing fields)
2. Signature verification failure (invalid EIP-712 signature, wrong signer)
3. **Slot validation failure** - `ValidateSnapshotterForSlot()` fails when `ENABLE_SLOT_VALIDATION=true` (snapshotter not registered for the slot)

All three types are tracked together under the same `MAX_VALIDATION_FAILURES_PER_EPOCH` threshold.


## Related Documentation

- [Redis Keys Reference](./REDIS_KEYS.md) - Redis key structure documentation

