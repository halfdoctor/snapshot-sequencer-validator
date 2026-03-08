#!/bin/bash

# DSV Bulk Service Snapshotter Address Flagging Test
# Verifies snapshotter address flagging for bulk service peers
# Peer ID whitelisted, snapshotter addresses flagged independently
# Run from the decentralized-sequencer repository root

LOG_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/logs"
DSV_DIR="/home/ubuntu/snapshot-sequencer-validator"
MONITORING_DOC="$DSV_DIR/docs/SPAM_PROTECTION_MONITORING.md"

mkdir -p "$LOG_DIR"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_FILE="$LOG_DIR/test-bulk-service-flagging-${TIMESTAMP}.txt"

# Write header
{
  echo "=== DSV Bulk Service Snapshotter Address Flagging Test ==="
  echo "Started: $(date)"
  echo "Running on: $(hostname)"
  echo "DSV Directory: $DSV_DIR"
  echo ""
} | tee "$LOG_FILE"

# Run Claude with inline prompt (bypass all permission checks)
claude -p --permission-mode bypassPermissions >> "$LOG_FILE" 2>&1 <<'EOF'
You are a DSV monitoring assistant testing bulk service peer snapshotter address flagging.

PROTOCOL UNDERSTANDING:
- Windows created at epoch boundaries (epochID % 10 == 0)
- Window ID = end epoch (10, 20, 30...)
- Collection windows happen per-epoch (LEVEL1_FINALIZATION_DELAY_SECONDS + 10s)
- Consensus checking ONLY at window boundaries after two-stage delay
- Snapshotter address reports aggregated separately from peer ID reports
- Consensus requires >= 2 validators reporting same snapshotter address in same window
- Bulk service peers: Peer ID whitelisted (BULK_SERVICE_PEER_IDS), snapshotter addresses tracked independently

TASK: Verify snapshotter address flagging for bulk service peers.

Change to DSV directory and run the following checks sequentially:
cd /home/ubuntu/snapshot-sequencer-validator

Step 1: Verify Peer ID Whitelist Configuration
- Check environment variables for BULK_SERVICE_PEER_IDS
- Verify local collector peer ID is in whitelist
- Check initialization logs: ./dsv.sh dequeuer-logs | grep -iE "(initialized peer whitelist|bulk service)"

Step 2: Verify Snapshotter Address Tracking (NOT Peer ID Tracking)
- Check Redis for snapshotter address tracking keys:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:submissions:snapshotter:*" | head -10
- Verify peer ID tracking is SKIPPED (bulk service peers):
  docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:submissions:peer:*" | grep -v "snapshotter" | head -5
  (Should NOT find peer ID tracking keys for bulk service peer)
- Check dequeuer logs for bulk service tracking:
  ./dsv.sh dequeuer-logs | grep -iE "Tracked submission for bulk service peer.*snapshotter"

Step 3: Check Snapshotter Address Aggregation Windows
- Check Redis for snapshotter address windows:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:reports:snapshotter:*:window:*" | head -10
- Check snapshotter address window SET:
  CURRENT_EPOCH=$(curl -s "http://localhost:9091/api/v1/epochs/active" | jq -r '.[0].epoch_id')
  WINDOW_ID=$(( (($CURRENT_EPOCH + 9) / 10) * 10 ))
  docker exec snapshot-sequencer-validator-redis-1 redis-cli SMEMBERS "*spam:reports:window:${WINDOW_ID}:snapshotters"
- Query API for snapshotter address windows:
  curl -s "http://localhost:9091/api/v1/spam/windows" | jq '.windows[] | select(.snapshotters != null) | {window_id, snapshotters}'

Step 4: Monitor Report Generation and Collection
- Check for snapshotter address spam reports (violationType="rate_limit_snapshotter"):
  ./dsv.sh event-logs | grep -iE "⚠️.*bulk service.*snapshotter.*shouldReport=true.*rate_limit_snapshotter"
- Check for report storage:
  ./dsv.sh event-logs | grep -iE "📝.*Stored snapshotter address spam report"
- Check collection window timers:
  ./dsv.sh event-logs | grep -iE "Started spam report collection window"
- Check report batching:
  ./dsv.sh event-logs | grep -iE "Sent.*batched spam reports"

Step 5: Monitor Consensus at Window Boundaries
- Get current epoch and check if it's a boundary:
  CURRENT_EPOCH=$(curl -s "http://localhost:9091/api/v1/epochs/active" | jq -r '.[0].epoch_id')
  echo "Current epoch: $CURRENT_EPOCH"
  echo "Is boundary: $(( $CURRENT_EPOCH % 10 == 0 ))"
  WINDOW_ID=$(( (($CURRENT_EPOCH + 9) / 10) * 10 ))
  echo "Window ID: $WINDOW_ID"
- Check for consensus delay scheduling (at boundaries):
  ./dsv.sh event-logs | grep -iE "Scheduling consensus check.*window"
- Check for consensus reached for snapshotter addresses:
  ./dsv.sh spam-aggregator-logs | grep -iE "Consensus reached for snapshotter address"
- Check validator counts in windows:
  curl -s "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq '.snapshotters[] | {snapshotter_address, validator_count, report_count}'

Step 6: Check Flagged Snapshotter Addresses
- Query API for flagged snapshotter addresses:
  curl -s "http://localhost:9091/api/v1/spam/flagged/snapshotters" | jq '.'
- Check Redis flagged snapshotter SET:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli SMEMBERS "flagged_snapshotters:*"
- Check individual flagged snapshotter keys:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:consensus_flagged:snapshotter:*" | head -5

Step 7: Verify Enforcement (Peer ID Whitelisted, Snapshotter Flagged)
- P2P Gateway should ALLOW (peer ID whitelisted):
  ./dsv.sh p2p-logs | grep -iE "Whitelisted peer.*bypassing.*spam checks"
- Dequeuer should REJECT (snapshotter address flagged):
  ./dsv.sh dequeuer-logs | grep -iE "Rejected submission from flagged snapshotter"
- Verify peer ID is NOT flagged:
  curl -s "http://localhost:9091/api/v1/spam/flagged/peers" | jq '.peers[] | select(.peer_id == "<bulk_service_peer_id>")'
  (Should return empty - peer ID should NOT be flagged)

FINAL OUTPUT FORMAT:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
          BULK SERVICE SNAPSHOTTER FLAGGING TEST
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

WHITELIST CONFIGURATION:
  Peer ID Whitelisted:    [YES/NO] - BULK_SERVICE_PEER_IDS configured
  Local Collector Peer ID: [peer ID if found]

TRACKING STATUS:
  Snapshotter Tracking:   [YES/NO] - snapshotter address keys exist
  Peer ID Tracking:       [SKIPPED/ACTIVE] - should be SKIPPED for bulk service peers
  Sample Snapshotter:     [address if found]

AGGREGATION WINDOWS:
  Snapshotter Windows:    [YES/NO] - snapshotter address windows exist
  Latest Window:          [window ID if available]
  Snapshotter Count:      [count in latest window]

REPORT GENERATION:
  Reports Generated:      [YES/NO] - rate_limit_snapshotter reports found
  Collection Windows:     [ACTIVE/INACTIVE] - per-epoch batching
  Reports Batched:        [YES/NO] - batched reports sent

CONSENSUS STATUS:
  Current Epoch:          [number]
  Is Boundary:            [yes/no]
  Window ID:              [number]
  Consensus Scheduled:    [YES/NO] - at boundaries
  Consensus Reached:      [YES/NO] - >= 2 validators
  Validator Count:        [number] - for snapshotter addresses

FLAGGING STATUS:
  Snapshotter Flagged:    [YES/NO] - snapshotter addresses flagged
  Flagged Count:          [number]
  Sample Flagged:         [snapshotter address if found]
  Peer ID Flagged:        [NO] - should NOT be flagged (whitelisted)

ENFORCEMENT:
  P2P Gateway:            [ALLOWS/REJECTS] - should ALLOW (peer ID whitelisted)
  Dequeuer:               [ALLOWS/REJECTS] - should REJECT (snapshotter flagged)

ISSUES FOUND:
  [List each issue with severity: CRITICAL/WARNING/INFO]

RECOMMENDATIONS:
  [List actionable recommendations]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

IMPORTANT INSTRUCTIONS:
- All documentation is provided above. DO NOT ask to read any files.
- You are already on the VPS - run commands directly in /home/ubuntu/snapshot-sequencer-validator
- Use ./dsv.sh {component}-logs for log access
- Use curl for API checks (port 9091)
- Use docker exec for Redis queries
- Provide actual command outputs in your analysis
- Focus on snapshotter address tracking (NOT peer ID tracking)
- Verify peer ID remains whitelisted while snapshotter addresses get flagged

Begin analysis now.
EOF

# Write footer
{
  echo ""
  echo "Completed: $(date)"
  echo "Log saved to: $LOG_FILE"
} >> "$LOG_FILE"

# Display output
cat "$LOG_FILE"
