# Spam Protection Monitoring Guide

## Overview

The DSV spam protection system provides multi-layer DDoS protection through tracking, validator coordination, enforcement, and on-chain flagging. This guide explains how to monitor and track the spam protection feature.

## Feature Status

✅ **IMPLEMENTED** - All phases completed:
- ✅ Phase 1: Tracking Layer (spam tracker, rate limiter, whitelist)
- ✅ Phase 2: Validator Coordination (spam reporter, aggregator)
- ✅ Phase 3: Enforcement Layer (P2P Gateway + Dequeuer)
- ✅ Phase 4: On-Chain Flagging (flagging service, sync service)
- ✅ Phase 5: Configuration & Monitoring (environment variables, metrics)

## Configuration

### Environment Variables

Enable spam protection by setting:

```bash
# Master switch
ENABLE_SPAM_PROTECTION=true

# Enable validator coordination
ENABLE_SPAM_REPORT_BROADCAST=true

# Spam report topic (validator-only)
# If empty, auto-constructs from GOSSIPSUB_VALIDATOR_PRESENCE_TOPIC prefix + "/spam-reports"
# Example: If GOSSIPSUB_VALIDATOR_PRESENCE_TOPIC=/powerloom/validator/presence,
#          then spam report topic will be /powerloom/validator/spam-reports
# To override, set explicit topic: SPAM_REPORT_TOPIC=/custom/path/spam-reports
SPAM_REPORT_TOPIC=

# Peer ID Whitelisting (comma-separated)
FULL_NODE_PEER_IDS=QmPeerID1,QmPeerID2,QmPeerID3
BULK_SERVICE_PEER_IDS=QmBulkPeerID1,QmBulkPeerID2

# Cache TTL (hours)
SPAM_CACHE_TTL_HOURS=24

# Flag expiry (on-chain)
SPAM_FLAG_EXPIRY_DAYS=7
SPAM_FLAG_EXPIRY_EPOCHS=100

# State synchronization
SPAM_SYNC_ON_STARTUP=true
SPAM_SYNC_INTERVAL_HOURS=6
```

### Hardcoded Thresholds

**Important**: These thresholds are hardcoded for consensus consistency across all validators:

- `MAX_VALIDATION_FAILURES_PER_EPOCH = 5` (per peer ID)
- `MAX_SUBMISSIONS_PER_EPOCH_LITE = 2` (per peer ID, per epoch)
- `CONSISTENT_VIOLATIONS_THRESHOLD = 3` (consecutive epochs required for rate limit violations)
- `SPAM_CONSENSUS_THRESHOLD = 2` (2 out of 3 validators)
- `SPAM_AGGREGATION_WINDOW_SIZE = 10` (epochs per aggregation window)

**Violation Reporting Behavior**:

1. **Validation Failures** (immediate reporting):
   - Triggers when `validationFailureCount >= 5` in **any single epoch**
   - No consecutive epochs required
   - Example: Epoch 100 has 5 failures → report immediately at epoch 100

2. **Rate Limit Violations** (consecutive epochs required):
   - Requires **3 consecutive epochs** with >2 submissions before reporting
   - Example: Epochs 100, 101, 102 all have >2 submissions → report at epoch 102
   - If epoch 101 had ≤2 submissions, no report is generated (breaks consecutive chain)

## Monitoring Metrics

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

## Redis Key Monitoring

### Per-Epoch Tracking Keys

Monitor validation failures and submission counts per peer:

```bash
# Check validation failures for a peer in an epoch
redis-cli GET "{protocol}:{market}:spam:validation_failures:peer:{peerID}:{epochID}"

# Check submission count for a peer in an epoch
redis-cli GET "{protocol}:{market}:spam:submissions:peer:{peerID}:{epochID}"

# Check peer-snapshotter associations
redis-cli SMEMBERS "{protocol}:{market}:spam:peer_snapshotter_map:{peerID}:{epochID}"
```

### Aggregation Window Keys

Monitor aggregated spam reports:

```bash
# Check aggregated reports for a peer in a window
redis-cli HGETALL "{protocol}:{market}:spam:reports:peer:{peerID}:window:{windowID}"

# Get validator count
redis-cli HGET "{protocol}:{market}:spam:reports:peer:{peerID}:window:{windowID}" validator_count

# Get snapshotter addresses
redis-cli SMEMBERS "{protocol}:{market}:spam:reports:peer:{peerID}:window:{windowID}:snapshotter_addrs"
```

### Flagged State Keys

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

### Active Validators

Monitor active validators for consensus calculation:

```bash
# List active validators
redis-cli SMEMBERS "{protocol}:{market}:active:validators"

# Count active validators
redis-cli SCARD "{protocol}:{market}:active:validators"
```

## Log Monitoring

### Key Log Messages

#### P2P Gateway
- `"Whitelisted peer {peerID} bypasses P2P Gateway spam checks"` - Whitelisted peer detected
- `"🚫 P2P Gateway: Dropping submission from flagged peer {peerID}"` - Submission dropped due to flagged peer
- `"Rejected submission from flagged peer: {peerID}"` - Flagged peer rejection

#### Dequeuer
- `"Whitelisted peer {peerID} bypasses spam checks and rate limiting"` - Whitelisted peer detected
- `"Rejected submission from flagged peer: {peerID}"` - Flagged peer rejection
- `"Rejected submission from flagged snapshotter: {snapshotterAddr}"` - Flagged snapshotter rejection
- `"Rate limit exceeded for peer {peerID} (epoch {epochID}): {count} submissions (max {max})"` - Rate limit exceeded
- `"Validation failure tracked for peer {peerID} (snapshotter {snapshotterAddr}, epoch {epochID}): count = {count}"` - Validation failure tracked

#### Spam Reporter
- `"🚨 Broadcasted spam report for peer {peerID} (epoch {epochID}, reason: {reason}, count: {count})"` - Spam report broadcast

#### Spam Aggregator
- `"Received spam report from {reporterID} for peer {peerID} (epoch {epochID}, reason: {reason})"` - Report received
- `"✅ Consensus reached for flagging peer {peerID} (window {windowID}): {validatorCount}/{activeValidatorCount} validators reported ({consensusRatio}%)"` - Consensus reached
- `"🚩 Successfully flagged peer {peerID} on-chain and in cache."` - Peer flagged

#### Flagging Service
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

## Monitoring Checklist

### Daily Checks

- [ ] Check spam reports sent/received metrics
- [ ] Monitor flagged peer count: `redis-cli SCARD "flagged_peers:{dataMarket}"`
- [ ] Check consensus reached count: `curl http://localhost:9090/metrics | grep spam_consensus_reached_total`
- [ ] Review submissions dropped metrics
- [ ] Check active validators count

### Weekly Checks

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

## Troubleshooting

### Issue: No spam reports being sent

**Check**:
1. Is `ENABLE_SPAM_PROTECTION=true`?
2. Is `ENABLE_SPAM_REPORT_BROADCAST=true`?
3. Are thresholds being exceeded? Check validation failure and submission counts
4. Are peers whitelisted? Whitelisted peers don't generate reports

### Issue: Consensus not reached

**Check**:
1. Are enough validators active? `redis-cli SCARD "{protocol}:{market}:active:validators"`
2. Are validators receiving reports? Check `spam_reports_received_total`
3. Is aggregation window size correct? Default is 10 epochs
4. Are reports being aggregated? Check aggregation window keys

### Issue: Flagged peers not being rejected

**Check**:
1. Is Redis cache synced? Check flagged state keys
2. Is on-chain state synced? Check sync logs
3. Are whitelist checks happening first? Whitelisted peers bypass flagging
4. Are enforcement checks enabled? Check P2P Gateway and Dequeuer logs

### Issue: Rate limits not enforced

**Check**:
1. Is peer whitelisted? Whitelisted peers bypass rate limits
2. Is spam protection enabled? `ENABLE_SPAM_PROTECTION=true`
3. Are submission counts being tracked? Check Redis keys
4. Is rate limiter initialized? Check Dequeuer initialization logs

## Integration with Existing Monitoring

The spam protection system integrates with existing DSV monitoring:

- **Prometheus Metrics**: All spam metrics are exposed via Prometheus
- **Redis Keys**: All spam keys follow existing naming conventions (`{protocol}:{market}:...`)
- **Logging**: Uses existing logrus logger with structured logging
- **State Sync**: Integrates with existing on-chain state synchronization

## Monitoring API Endpoints

The monitor-api component exposes REST endpoints for spam protection information:

### Endpoints

- `GET /api/v1/spam/flagged/peers` - List all flagged peer IDs with metadata
- `GET /api/v1/spam/flagged/snapshotters` - List all flagged snapshotter addresses with metadata
- `GET /api/v1/spam/peer/:peerID` - Get spam tracking info for a specific peer (validation failures, submission counts, aggregation info)
- `GET /api/v1/spam/stats` - Get aggregated spam protection statistics (flagged counts, active validators)

### Example Usage

```bash
# Get all flagged peers
curl http://localhost:8080/api/v1/spam/flagged/peers

# Get spam info for a specific peer
curl http://localhost:8080/api/v1/spam/peer/QmPeerID123?epochID=12345

# Get spam protection statistics
curl http://localhost:8080/api/v1/spam/stats
```

## Related Documentation

- [DDoS Protection Plan](../ai-coord-docs/phase3/DDoS_PROTECTION_PLAN.md) - Complete implementation plan
- [Redis Keys Reference](./REDIS_KEYS.md) - Redis key structure documentation
- [Monitoring Guide](./MONITORING_GUIDE.md) - General monitoring documentation
- [DSV Node Setup Guide](./DSV_NODE_SETUP.md) - Node deployment guide

