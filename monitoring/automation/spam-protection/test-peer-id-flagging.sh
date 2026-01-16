#!/bin/bash

# DSV Regular Peer ID Flagging Test
# Verifies peer ID flagging for regular peers
# Peer ID flagged, P2P Gateway rejects submissions
# Run from the decentralized-sequencer repository root

LOG_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/logs"
DSV_DIR="/home/ubuntu/snapshot-sequencer-validator"
MONITORING_DOC="$DSV_DIR/docs/SPAM_PROTECTION_MONITORING.md"

mkdir -p "$LOG_DIR"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_FILE="$LOG_DIR/test-peer-id-flagging-${TIMESTAMP}.txt"

# Write header
{
  echo "=== DSV Regular Peer ID Flagging Test ==="
  echo "Started: $(date)"
  echo "Running on: $(hostname)"
  echo "DSV Directory: $DSV_DIR"
  echo ""
} | tee "$LOG_FILE"

# Run Claude with inline prompt (bypass all permission checks)
claude -p --permission-mode bypassPermissions >> "$LOG_FILE" 2>&1 <<'EOF'
You are a DSV monitoring assistant testing regular peer ID flagging.

PROTOCOL UNDERSTANDING:
- Windows created at epoch boundaries (epochID % 10 == 0)
- Window ID = end epoch (10, 20, 30...)
- Collection windows happen per-epoch (LEVEL1_FINALIZATION_DELAY_SECONDS + 10s)
- Consensus checking ONLY at window boundaries after two-stage delay
- Peer ID reports aggregated separately from snapshotter address reports
- Consensus requires >= 2 validators reporting same peer ID in same window
- When peer ID flagged, both peer ID and associated snapshotter addresses are flagged
- Regular peers: NOT in whitelist, tracked by peer ID, flagged by peer ID

TASK: Verify peer ID flagging for regular peers.

Change to DSV directory and run the following checks sequentially:
cd /home/ubuntu/snapshot-sequencer-validator

Step 1: Verify Peer ID is NOT in Whitelist
- Check environment variables for BULK_SERVICE_PEER_IDS and FULL_NODE_PEER_IDS
- Verify local collector peer ID is NOT in whitelist
- Check initialization logs: ./dsv.sh dequeuer-logs | grep -iE "(initialized peer whitelist)"
- Verify peer is being tracked (not skipped)

Step 2: Check Peer ID Tracking (Both Peer ID and Snapshotter Address Tracking)
- Check Redis for peer ID tracking keys:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:submissions:peer:*" | grep -v "snapshotter" | head -10
- Check snapshotter address tracking (for associated flagging):
  docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:submissions:snapshotter:*" | head -10
- Check dequeuer logs for peer ID tracking:
  ./dsv.sh dequeuer-logs | grep -iE "Tracked submission for peer.*epoch"
- Verify both peer ID and snapshotter addresses are tracked

Step 3: Check Peer ID Aggregation Windows
- Check Redis for peer ID windows:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:reports:peer:*:window:*" | head -10
- Check peer ID window SET:
  CURRENT_EPOCH=$(curl -s "http://localhost:9091/api/v1/epochs/active" | jq -r '.[0].epoch_id')
  WINDOW_ID=$(( (($CURRENT_EPOCH + 9) / 10) * 10 ))
  docker exec snapshot-sequencer-validator-redis-1 redis-cli SMEMBERS "*spam:reports:window:${WINDOW_ID}:peers"
- Query API for peer ID windows:
  curl -s "http://localhost:9091/api/v1/spam/windows" | jq '.windows[] | select(.peers != null) | {window_id, peer_count, peers: [.peers[] | {peer_id, validator_count, report_count}]}'

Step 4: Monitor Report Generation and Collection
- Check for peer ID spam reports (violationType="rate_limit" or "validation_failure", NOT "rate_limit_snapshotter"):
  ./dsv.sh event-logs | grep -iE "⚠️.*Spam check for peer.*shouldReport=true.*rate_limit|validation_failure" | grep -v "rate_limit_snapshotter"
- Check for report storage:
  ./dsv.sh event-logs | grep -iE "📝.*Stored peer ID spam report"
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
- Check for consensus reached for peer IDs:
  ./dsv.sh spam-aggregator-logs | grep -iE "Consensus reached for peer"
- Check validator counts in windows:
  curl -s "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq '.peers[] | {peer_id, validator_count, report_count, snapshotter_addresses}'

Step 6: Check Flagged Peer IDs and Associated Snapshotter Addresses
- Query API for flagged peer IDs:
  curl -s "http://localhost:9091/api/v1/spam/flagged/peers" | jq '.'
- Check Redis flagged peer SET:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli SMEMBERS "flagged_peers:*"
- Check individual flagged peer keys:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:consensus_flagged:peer:*" | head -5
- Verify associated snapshotter addresses are also flagged:
  curl -s "http://localhost:9091/api/v1/spam/flagged/peers" | jq '.peers[] | {peer_id, snapshotter_addresses}'

Step 7: Verify Enforcement (Peer ID Flagged)
- P2P Gateway should REJECT (peer ID flagged):
  ./dsv.sh p2p-logs | grep -iE "Rejected submission from flagged peer|Dropping.*flagged"
- Dequeuer should REJECT (peer ID flagged):
  ./dsv.sh dequeuer-logs | grep -iE "Rejected submission from flagged peer"
- Verify both peer ID and associated snapshotter addresses are flagged

FINAL OUTPUT FORMAT:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
              REGULAR PEER ID FLAGGING TEST
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

WHITELIST CONFIGURATION:
  Peer ID Whitelisted:    [NO] - NOT in BULK_SERVICE_PEER_IDS or FULL_NODE_PEER_IDS
  Local Collector Peer ID: [peer ID if found]

TRACKING STATUS:
  Peer ID Tracking:       [YES/NO] - peer ID keys exist
  Snapshotter Tracking:   [YES/NO] - snapshotter address keys exist (for associated flagging)
  Sample Peer:            [peer ID if found]
  Sample Snapshotter:      [address if found]

AGGREGATION WINDOWS:
  Peer ID Windows:        [YES/NO] - peer ID windows exist
  Latest Window:          [window ID if available]
  Peer Count:             [count in latest window]

REPORT GENERATION:
  Reports Generated:      [YES/NO] - rate_limit or validation_failure reports found
  Violation Type:         [rate_limit/validation_failure] - NOT rate_limit_snapshotter
  Collection Windows:     [ACTIVE/INACTIVE] - per-epoch batching
  Reports Batched:        [YES/NO] - batched reports sent

CONSENSUS STATUS:
  Current Epoch:          [number]
  Is Boundary:            [yes/no]
  Window ID:              [number]
  Consensus Scheduled:    [YES/NO] - at boundaries
  Consensus Reached:      [YES/NO] - >= 2 validators
  Validator Count:        [number] - for peer IDs

FLAGGING STATUS:
  Peer ID Flagged:        [YES/NO] - peer ID flagged
  Flagged Count:          [number]
  Sample Flagged:         [peer ID if found]
  Snapshotter Addresses:  [YES/NO] - associated snapshotter addresses also flagged
  Snapshotter Count:      [number] - associated snapshotter addresses flagged

ENFORCEMENT:
  P2P Gateway:            [ALLOWS/REJECTS] - should REJECT (peer ID flagged)
  Dequeuer:               [ALLOWS/REJECTS] - should REJECT (peer ID flagged)

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
- Focus on peer ID tracking and flagging
- Verify both peer ID and associated snapshotter addresses are flagged

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
