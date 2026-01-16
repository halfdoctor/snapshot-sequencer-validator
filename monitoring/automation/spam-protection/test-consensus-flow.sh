#!/bin/bash

# DSV Consensus Flow Test
# Verifies consensus flow with 2 DSV nodes (validator coordination via P2P)
# Run from the decentralized-sequencer repository root

LOG_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/logs"
DSV_DIR="/home/ubuntu/snapshot-sequencer-validator"
MONITORING_DOC="$DSV_DIR/docs/SPAM_PROTECTION_MONITORING.md"

mkdir -p "$LOG_DIR"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_FILE="$LOG_DIR/test-consensus-flow-${TIMESTAMP}.txt"

# Write header
{
  echo "=== DSV Consensus Flow Test ==="
  echo "Started: $(date)"
  echo "Running on: $(hostname)"
  echo "DSV Directory: $DSV_DIR"
  echo ""
} | tee "$LOG_FILE"

# Run Claude with inline prompt (bypass all permission checks)
claude -p --permission-mode bypassPermissions >> "$LOG_FILE" 2>&1 <<'EOF'
You are a DSV monitoring assistant testing consensus flow with 2 DSV nodes.

PROTOCOL UNDERSTANDING:
- 2 DSV nodes must coordinate via P2P Gossipsub
- Reports broadcast via OutgoingSpamReports queue → p2p-gateway → Gossipsub
- Reports received via Gossipsub → p2p-gateway → IncomingSpamReports queue → spam-aggregator
- Consensus requires >= 2 validators reporting same peer/snapshotter in same window
- Consensus checking ONLY at window boundaries (epochID % 10 == 0) after two-stage delay
- Both nodes should flag same peers/snapshotters after consensus reached
- Windows created at epoch boundaries, window ID = end epoch (10, 20, 30...)
- Collection windows happen per-epoch (LEVEL1_FINALIZATION_DELAY_SECONDS + 10s)

TASK: Verify consensus flow with 2 DSV nodes (validator coordination via P2P).

Change to DSV directory and run the following checks sequentially:
cd /home/ubuntu/snapshot-sequencer-validator

Step 1: Verify Both DSV Nodes Are Running and Connected
- Check component status:
  ./dsv.sh status | grep -E "(dequeuer|spam-aggregator|p2p-gateway|event-monitor)"
- Check P2P connectivity:
  ./dsv.sh p2p-logs | grep -iE "(connected.*peer|peer.*connected)" | tail -10
- Verify both nodes are active validators:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli SMEMBERS "*:active:validators"

Step 2: Check Active Validators
- List active validators:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli SMEMBERS "*:active:validators"
- Count active validators:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli SCARD "*:active:validators"
- Should show 2 validators (validator1 and validator2)
- Check validator presence:
  ./dsv.sh p2p-logs | grep -iE "validator.*presence|presence.*heartbeat" | tail -5

Step 3: Monitor Spam Report Broadcasting (Local Reports)
- Check local report generation:
  ./dsv.sh event-logs | grep -iE "📝.*Stored.*spam report" | tail -10
- Check report batching (after collection window):
  ./dsv.sh event-logs | grep -iE "Sent.*batched spam reports" | tail -5
- Check P2P Gateway broadcasting:
  ./dsv.sh p2p-logs | grep -iE "Broadcasted spam report via Gossipsub|📤.*Broadcasting spam report" | tail -10
- Verify reports are queued for broadcasting:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli LLEN "*:outgoing:spam-reports"

Step 4: Monitor Spam Report Receiving (Remote Reports)
- Check P2P Gateway receiving:
  ./dsv.sh p2p-logs | grep -iE "Queued incoming spam report|Received spam report" | tail -10
- Check spam-aggregator receiving:
  ./dsv.sh spam-aggregator-logs | grep -iE "📨.*Received spam report from validator" | tail -10
- Verify reports are queued for aggregation:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli LLEN "*:incoming:spam-reports"
- Check report source (should show different validator IDs):
  ./dsv.sh spam-aggregator-logs | grep -iE "Received spam report from validator" | grep -oE "validator[0-9]+" | sort -u

Step 5: Monitor Report Aggregation
- Check aggregation logs:
  ./dsv.sh spam-aggregator-logs | grep -iE "Atomically aggregated.*report" | tail -10
- Get current epoch and window:
  CURRENT_EPOCH=$(curl -s "http://localhost:9091/api/v1/epochs/active" | jq -r '.[0].epoch_id')
  WINDOW_ID=$(( (($CURRENT_EPOCH + 9) / 10) * 10 ))
  echo "Current epoch: $CURRENT_EPOCH, Window ID: $WINDOW_ID"
- Check validator counts in windows (should show >= 2 for consensus):
  curl -s "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq '.peers[] | {peer_id, validator_count, report_count}'
  curl -s "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq '.snapshotters[] | {snapshotter_address, validator_count, report_count}'
- Verify reports from multiple validators are aggregated:
  curl -s "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq '.peers[] | select(.validator_count >= 2)'
  curl -s "http://localhost:9091/api/v1/spam/windows/${WINDOW_ID}" | jq '.snapshotters[] | select(.validator_count >= 2)'

Step 6: Monitor Consensus Checking at Window Boundaries
- Get current epoch and check if it's a boundary:
  CURRENT_EPOCH=$(curl -s "http://localhost:9091/api/v1/epochs/active" | jq -r '.[0].epoch_id')
  echo "Current epoch: $CURRENT_EPOCH"
  echo "Is boundary: $(( $CURRENT_EPOCH % 10 == 0 ))"
  WINDOW_ID=$(( (($CURRENT_EPOCH + 9) / 10) * 10 ))
  echo "Window ID: $WINDOW_ID"
- Check for consensus delay scheduling (at boundaries):
  ./dsv.sh event-logs | grep -iE "Scheduling consensus check.*window|⏳.*Scheduling consensus check" | tail -5
- Check for consensus checking:
  ./dsv.sh spam-aggregator-logs | grep -iE "Checking consensus for window|🔍.*Checking consensus" | tail -5
- Check for consensus reached:
  ./dsv.sh spam-aggregator-logs | grep -iE "Consensus reached for peer|Consensus reached for snapshotter|🚩.*Consensus reached" | tail -10
- Verify consensus threshold (>= 2 validators):
  ./dsv.sh spam-aggregator-logs | grep -iE "Consensus reached" | grep -oE "validators: [0-9]+" | tail -5

Step 7: Verify Flagging Synchronization
- Check flagged peers on this node:
  curl -s "http://localhost:9091/api/v1/spam/flagged/peers" | jq '.peers[] | {peer_id, flagged_at, snapshotter_addresses}'
- Check flagged snapshotters on this node:
  curl -s "http://localhost:9091/api/v1/spam/flagged/snapshotters" | jq '.snapshotters[] | {snapshotter_address, flagged_at}'
- Check Redis flagged state:
  docker exec snapshot-sequencer-validator-redis-1 redis-cli SMEMBERS "flagged_peers:*"
  docker exec snapshot-sequencer-validator-redis-1 redis-cli SMEMBERS "flagged_snapshotters:*"
- Note: To compare with other node, run this script on both nodes and compare results
- Check flagging logs:
  ./dsv.sh spam-aggregator-logs | grep -iE "Flagged peer|Flagged snapshotter|🚩.*Flagged" | tail -10

FINAL OUTPUT FORMAT:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
                  CONSENSUS FLOW TEST
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

NODE STATUS:
  Components Running:     [YES/NO] - dequeuer, spam-aggregator, p2p-gateway, event-monitor
  P2P Connected:         [YES/NO] - connected to other validators
  Node ID:               [validator ID if found]

VALIDATOR COORDINATION:
  Active Validators:      [count] - should be 2
  Validator IDs:          [list of validator IDs]
  This Node ID:           [validator ID]

REPORT BROADCASTING:
  Local Reports Generated: [YES/NO] - reports stored locally
  Reports Batched:        [YES/NO] - after collection window
  Reports Broadcast:       [YES/NO] - via P2P Gossipsub
  Outgoing Queue:         [count] - reports waiting to broadcast

REPORT RECEIVING:
  Reports Received:       [YES/NO] - from other validators
  Incoming Queue:         [count] - reports waiting to aggregate
  Remote Validators:       [list] - validator IDs that sent reports

REPORT AGGREGATION:
  Current Epoch:          [number]
  Is Boundary:            [yes/no]
  Window ID:              [number]
  Peers in Window:        [count]
  Snapshotters in Window: [count]
  Validator Counts:       [list] - validator_count per peer/snapshotter
  Consensus Candidates:   [count] - peers/snapshotters with validator_count >= 2

CONSENSUS CHECKING:
  Consensus Scheduled:    [YES/NO] - at boundaries
  Consensus Checked:      [YES/NO] - after delay
  Consensus Reached:      [YES/NO] - >= 2 validators
  Peers Flagged:          [count] - after consensus
  Snapshotters Flagged:   [count] - after consensus

FLAGGING SYNCHRONIZATION:
  Flagged Peers:          [list] - peer IDs flagged on this node
  Flagged Snapshotters:   [list] - snapshotter addresses flagged on this node
  Note:                   [Run this script on both nodes to compare]

ISSUES FOUND:
  [List each issue with severity: CRITICAL/WARNING/INFO]

RECOMMENDATIONS:
  [List actionable recommendations]
  [If validator_count < 2, note that consensus requires 2 validators]
  [If reports not received, check P2P connectivity]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

IMPORTANT INSTRUCTIONS:
- All documentation is provided above. DO NOT ask to read any files.
- You are already on the VPS - run commands directly in /home/ubuntu/snapshot-sequencer-validator
- Use ./dsv.sh {component}-logs for log access
- Use curl for API checks (port 9091)
- Use docker exec for Redis queries
- Provide actual command outputs in your analysis
- Focus on validator coordination and consensus
- Verify reports are exchanged between nodes
- Check that consensus requires >= 2 validators
- Note: Run this script on BOTH nodes to verify synchronization

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
