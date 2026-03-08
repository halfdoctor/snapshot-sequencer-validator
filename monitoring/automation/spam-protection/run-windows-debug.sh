#!/bin/bash

# DSV Windows Debug Script
# Focused diagnostic for why /api/v1/spam/windows is empty
# Run from the decentralized-sequencer repository root

set -e

# Find repo root (where dsv.sh and this script exist)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
DSV_DIR="$REPO_ROOT"
LOG_DIR="$SCRIPT_DIR/logs"

mkdir -p "$LOG_DIR"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_FILE="$LOG_DIR/windows-debug-${TIMESTAMP}.txt"

# Write header
{
  echo "=== DSV Windows Debug Analysis ==="
  echo "Started: $(date)"
  echo "Running on: $(hostname)"
  echo "Repo root: $REPO_ROOT"
  echo ""
} > "$LOG_FILE"

# Run Claude with inline prompt (bypass all permission checks)
claude -p --permission-mode bypassPermissions >> "$LOG_FILE" 2>&1 <<EOF
You are debugging why spam aggregation windows are not being created on the DSV system.

You are in the repository root: $REPO_ROOT

Change to DSV directory and run these diagnostic steps:
cd $DSV_DIR

1. CHECK SPAM-AGGREGATOR IS RUNNING:
   ./dsv.sh status | grep spam-aggregator

2. CHECK SPAM-AGGREGATOR INITIALIZATION:
   ./dsv.sh spam-aggregator-logs | grep -iE "(spam aggregator component starting|initializing spam|redis queue|event monitor|spam protection components initialized)"

3. CHECK FOR EPOCH BOUNDARY EVENTS:
   ./dsv.sh spam-aggregator-logs | grep -iE "(epoch.*released|aggregation window boundary|creating window|window.*aggregated|Waiting.*seconds before sending reports|checking consensus|CheckWindowForConsensus)"

4. CHECK CURRENT EPOCH:
   CURRENT_EPOCH=\$(curl -s "http://localhost:9091/api/v1/epochs/active" | jq -r '.current_epoch')
   echo "Current epoch: \$CURRENT_EPOCH"
   echo "Is boundary: \$(( \$CURRENT_EPOCH % 10 == 0 ))"
   echo "Last boundary: \$(( (\$CURRENT_EPOCH / 10) * 10 ))"
   echo "Next boundary: \$(( ((\$CURRENT_EPOCH / 10) + 1) * 10 ))"

5. CHECK REDIS FOR WINDOWS:
   docker exec snapshot-sequencer-validator-redis-1 redis-cli SMEMBERS "0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401:0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d:spam:reports:windows"

6. CHECK EVENT-MONITOR WINDOW CREATION AND REPORT COLLECTION:
   ./dsv.sh event-logs | grep -iE "(EpochReleased|aggregation window boundary|CreateWindowAndAggregateLocalData|Successfully created spam aggregation window|Failed to create spam aggregation window|Generated local spam report|Stored.*spam report|Waiting.*seconds before sending reports|checking consensus|CheckWindowForConsensus)"

7. CHECK FOR TRACKING DATA (shows if system is working at all):
   docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:epoch:*:peers" | head -5
   
8. CHECK FOR SNAPSHOTTER ADDRESS TRACKING (bulk service peers):
   docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:submissions:snapshotter:*" | head -5
   
9. CHECK FOR SNAPSHOTTER AGGREGATION WINDOWS:
   docker exec snapshot-sequencer-validator-redis-1 redis-cli KEYS "*spam:reports:snapshotter:*:window:*" | head -5

DIAGNOSIS FORMAT:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
                  WINDOWS CREATION DIAGNOSIS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

COMPONENT STATUS:
  Spam-Aggregator:  [RUNNING/STOPPED/ERROR]
  Event-Monitor:    [RUNNING/STOPPED/ERROR]
  Initialized:      [YES/NO] - based on logs

EPOCH STATUS:
  Current Epoch:    [number]
  Is Boundary:      [yes/no]
  Last Boundary:    [epoch number]
  Next Boundary:    [epoch number]
  Time to Boundary: [estimate if possible]

WINDOW STATUS:
  Redis Windows (Peer ID):    [COUNT or EMPTY]
  Redis Windows (Snapshotter): [COUNT or EMPTY]
  API Windows:      [COUNT or EMPTY]
  Last Created:     [window ID or "none"]
  Collection Window: [ACTIVE/INACTIVE] - per-epoch report batching
  Consensus Delay:  [SCHEDULED/NOT SCHEDULED] - at boundaries

INITIALIZATION CHECKS:
  Spam Components:  [INITIALIZED/NOT INITIALIZED/ERROR]
  Redis Connected:  [YES/NO]
  Event Monitor:    [WORKING/NOT WORKING]

LOG EVIDENCE:
  [Relevant log lines showing:
   - Initialization messages
   - Epoch boundary detection
   - Window creation attempts
   - Any errors]

ROOT CAUSE ANALYSIS:
  [Most likely reason windows are not being created]

RECOMMENDED ACTIONS:
  1. [Specific action item]
  2. [Specific action item]
  3. [Specific action item]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Show actual command outputs. Be specific about what's wrong and what needs to be fixed.
EOF

# Write footer
{
  echo ""
  echo "Log saved to: $LOG_FILE"
} >> "$LOG_FILE"

# Display output
cat "$LOG_FILE"
