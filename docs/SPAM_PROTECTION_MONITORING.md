# DDoS Protection Monitoring Guide

Practical guide for monitoring and debugging the DSV DDoS protection system against spam and illegitimate submissions.

## Quick Debugging: Why Are Aggregation Windows Not Being Created?

If `/api/v1/spam/windows` returns empty, follow these steps:

### Step 0: Verify DDoS Protection Components Initialized

**CRITICAL FIRST STEP**: Check if components initialized correctly at startup.

```bash
# Check initialization logs (look for these at node startup)
./dsv.sh dequeuer-logs | grep -i "initializing spam\|spam protection components initialized\|aggregator.*initialized"

# Expected logs:
# - "Initializing spam protection components" with fields:
#   - enable_spam_protection: true
#   - enable_spam_report_broadcast: true
#   - pubsub_available: true (if P2P is enabled)
# - "Initialized spam aggregator with window size: 10 (broadcast enabled, subscription handler started)"
#   OR
#   "Initialized spam aggregator with window size: 10 (local aggregation only, broadcast disabled or pubsub unavailable)"
# - "✅ Spam protection components initialized" with component status:
#   - tracker_initialized: true
#   - rate_limiter_initialized: true
#   - flagging_initialized: true
#   - reporter_initialized: true/false (depends on broadcast)
#   - aggregator_initialized: true (MUST be true)
```

**If `aggregator_initialized: false` or aggregator logs are missing:**
- Check environment variables: `ENABLE_SPAM_PROTECTION=true` and `ENABLE_SPAM_REPORT_BROADCAST=true`
- Verify Redis connection is working
- Check if P2P/pubsub is initialized (required for broadcast, but aggregator works without it)

### Step 1: Check if EventMonitor is Processing Epochs

```bash
# Check event monitor logs for epoch releases
docker logs <event-monitor-container> | grep -i "epoch.*released\|epoch.*boundary\|aggregation window"

# Look for these messages:
# - "EpochReleased event received: epoch={epochID}"
# - "Epoch {epochID} is an aggregation window boundary"
# - "Creating spam aggregation window"
```

### Step 2: Check if Tracking is Happening

```bash
# Check dequeuer logs for tracking activity
docker logs <dequeuer-container> | grep -i "tracked.*submission\|tracked.*validation\|epoch.*peers\|spam.*tracker\|peer.*empty"

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
# Get current epoch
curl "http://localhost:9091/api/v1/epochs/active" | jq '.current_epoch'

# Check if current epoch is a boundary (should be multiple of 10)
# Windows are created when epochID % 10 == 0 (epochs 10, 20, 30, etc.)
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
curl "http://localhost:9091/api/v1/spam/epochs?limit=20" | jq '.'

# If this returns empty, tracking isn't happening
# If this returns epochs, check if any are multiples of 10
```

### Step 6: Verify EventMonitor Configuration

```bash
# Check if DDoS protection components are passed to EventMonitor
./dsv.sh event-logs | grep -i "spam.*component\|spam.*aggregator.*nil\|epoch.*boundary"

# Look for:
# - "Epoch {epochID} is an aggregation window boundary. Triggering local spam data aggregation."
# - "Spam aggregator is nil at epoch boundary {epochID}" (BAD - aggregator not initialized)
# - "Spam tracker is nil at epoch boundary {epochID}" (BAD - tracker not initialized)
# - "Successfully triggered spam aggregation window creation for epoch {epochID}" (GOOD)
# - "Failed to create spam aggregation window at epoch boundary {epochID}: {error}" (BAD - check error)

# EventMonitor needs spamComponents.Aggregator to create windows
# If aggregator is nil, check Step 0 initialization logs
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
# Get details for a specific window
curl "http://localhost:9091/api/v1/spam/windows/24176210" | jq '.'

# Replace 24176210 with actual window ID (multiple of 10)
```

### Check Epoch Tracking

```bash
# List epochs with tracking data
curl "http://localhost:9091/api/v1/spam/epochs?limit=20" | jq '.'

# Shows which epochs have peer activity
```

### Check Peer Tracking

```bash
# Get tracking for a specific peer in an epoch
PEER_ID="12D3KooW..."
EPOCH_ID=24176205
curl "http://localhost:9091/api/v1/spam/peer/${PEER_ID}?epochID=${EPOCH_ID}" | jq '.'

# Get epoch-by-epoch tracking for a peer
curl "http://localhost:9091/api/v1/spam/peer/${PEER_ID}/epochs?startEpoch=24176200&endEpoch=24176210" | jq '.'
```

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

**Unified Sequencer (Full System)**:
```bash
./dsv.sh dequeuer-logs | grep -i "initializing spam\|spam protection components initialized"
```
Look for:
- `"Initializing spam protection components"` with fields:
  - `enable_spam_protection: true`
  - `enable_spam_report_broadcast: true`
  - `pubsub_available: true` (if P2P enabled)
- `"Initialized spam aggregator with window size: 10 (broadcast enabled, subscription handler started)"` (GOOD)
- `"Initialized spam aggregator with window size: 10 (local aggregation only, broadcast disabled or pubsub unavailable)"` (OK - aggregator still works)
- `"✅ Spam protection components initialized"` with:
  - `aggregator_initialized: true` (MUST be true)
  - `tracker_initialized: true`
  - `reporter_initialized: true/false`

**P2P Gateway (Early Rejection)**:
```bash
./dsv.sh p2p-logs | grep -i "initialized spam protection"
```
Look for:
- `"Initialized spam protection: whitelist ({N} full nodes, {M} bulk service), flagging service"`

**EventMonitor (Window Creation)**:
```bash
./dsv.sh event-logs | grep -i "aggregation window\|epoch.*boundary\|spam.*aggregator.*nil"
```
Look for:
- `"Epoch {epochID} is an aggregation window boundary. Triggering local spam data aggregation."`
- `"Creating spam aggregation window {windowID} for epochs {start}-{end}"`
- `"Created spam aggregation window {windowID} with {N} peers"`
- `"Successfully triggered spam aggregation window creation for epoch {epochID}"` (GOOD)
- `"Spam aggregator is nil at epoch boundary {epochID}"` (BAD - check initialization)
- `"Spam tracker is nil at epoch boundary {epochID}"` (BAD - check initialization)
- `"Spam components not initialized - skipping window creation"` (BAD - check initialization)

**Dequeuer (DDoS Protection Tracking)**:
```bash
docker logs <dequeuer-container> | grep -i "tracked\|spam\|validation failure\|peer.*empty\|spam.*tracker.*nil"
```
Look for:
- `"Tracked submission for peer {peerID} epoch {epochID} (count: {N})"` (debug level)
- `"Tracked validation failure for peer {peerID} epoch {epochID} (count: {N})"` (debug level)
- `"Peer ID is empty - skipping DDoS protection tracking"`
- `"Spam tracker is nil (DDoS protection enabled but tracker not initialized)"`
- `"DDoS protection disabled - skipping tracking"`


**Spam Aggregator**:
```bash
docker logs <dequeuer-container> | grep -i "aggregated.*local\|window.*peer"
```
Look for:
- `"Aggregated local tracking data for peer {peerID} in window {windowID}"`

**P2P Gateway (Enforcement)**:
```bash
docker logs <p2p-gateway-container> | grep -i "flagged\|whitelist\|dropping"
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
# Full nodes and bulk service snapshotters bypass rate limiting
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

## Troubleshooting

### Windows Not Being Created

**Symptoms**: `/api/v1/spam/windows` returns empty, no windows in Redis

**Debug Steps**:

1. **Check EventMonitor is running**:
   ```bash
   docker ps | grep event-monitor
   ```

2. **Check if epochs are being processed**:
   ```bash
   docker logs <event-monitor-container> --tail 50 | grep -i "epoch.*released"
   ```

3. **Check if current epoch is a boundary**:
   ```bash
   CURRENT_EPOCH=$(curl -s "http://localhost:9091/api/v1/epochs/active" | jq -r '.current_epoch')
   echo "Current epoch: $CURRENT_EPOCH"
   echo "Is boundary: $(( $CURRENT_EPOCH % 10 == 0 ))"
   ```

4. **Check if spam components are initialized**:
   ```bash
   docker logs <dequeuer-container> | grep -i "spam.*component\|spam.*initialized"
   ```

5. **Check if tracking is happening**:
   ```bash
   # Enable debug logging first (LOG_LEVEL=debug in env)
   # Then check for tracking logs
   docker logs <dequeuer-container> | grep -i "tracked.*submission\|tracked.*validation\|peer.*empty"
   ```

6. **Check Redis for epoch peer sets**:
   ```bash
   docker exec <redis-container> redis-cli KEYS "*spam:epoch:*:peers" | head -10
   ```

### No Tracking Data

**Symptoms**: `/api/v1/spam/epochs` returns empty, no epoch peer sets in Redis

**Debug Steps**:

1. **Verify spam protection is enabled**:
   ```bash
   docker logs <dequeuer-container> | grep -i "spam.*protection.*enabled\|ENABLE_SPAM_PROTECTION"
   ```

2. **Check if submissions are being processed**:
   ```bash
   docker logs <dequeuer-container> | grep -i "processed.*submission\|worker.*processing"
   ```

3. **Check if peers are whitelisted** (whitelisted peers aren't tracked):
   ```bash
   # Check your env vars in docker-compose
   docker exec <dequeuer-container> env | grep FULL_NODE_PEER_IDS
   docker exec <dequeuer-container> env | grep BULK_SERVICE_PEER_IDS
   ```

4. **Check if peerID is being passed**:
   ```bash
   docker logs <dequeuer-container> | grep -i "peer.*empty\|peer_id"
   ```

### No Spam Reports Being Sent

**Debug Steps**:

1. **Check thresholds are being exceeded**:
   ```bash
   docker exec <redis-container> redis-cli KEYS "*spam:validation_failures:peer:*"
   docker exec <redis-container> redis-cli KEYS "*spam:submissions:peer:*"
   ```

2. **Check if report broadcast is enabled**:
   ```bash
   docker exec <dequeuer-container> env | grep ENABLE_SPAM_REPORT_BROADCAST
   ```

3. **Check reporter logs**:
   ```bash
   docker logs <dequeuer-container> | grep -i "broadcasted.*spam\|spam.*report"
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

## Related Documentation

- [Redis Keys Reference](./REDIS_KEYS.md) - Redis key structure documentation
- [DDoS Protection Plan](../ai-coord-docs/phase3/DDoS_PROTECTION_PLAN.md) - Complete implementation plan and architecture
- [Spam Report Flow](../ai-coord-docs/phase3/SPAM_REPORT_FLOW.md) - Detailed flow of spam reports and aggregation windows

