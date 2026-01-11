#!/bin/bash

# DSV Quick Health Check Script
# Fast status overview of DSV components
# Run from the decentralized-sequencer repository root

set -e

# Find repo root (where dsv.sh and this script exist)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
DSV_DIR="$REPO_ROOT"
LOG_DIR="$SCRIPT_DIR/logs"

mkdir -p "$LOG_DIR"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_FILE="$LOG_DIR/quick-status-${TIMESTAMP}.txt"

# Write header
{
  echo "=== DSV Quick Health Check ==="
  echo "Started: $(date)"
  echo "Running on: $(hostname)"
  echo "Repo root: $REPO_ROOT"
  echo ""
} > "$LOG_FILE"

# Run Claude with inline prompt (bypass all permission checks)
claude -p --permission-mode bypassPermissions >> "$LOG_FILE" 2>&1 <<EOF
You are running a quick health check on the DSV system.

You are in the repository root: $REPO_ROOT

Change to DSV directory and run these checks:
cd $DSV_DIR

1. COMPONENT STATUS:
   ./dsv.sh status | grep -E "(dequeuer|spam-aggregator|p2p-gateway|event-monitor)"

2. RECENT SPAM-AGGREGATOR LOGS (last 20 lines):
   ./dsv.sh spam-aggregator-logs | tail -20

3. WINDOWS FROM API:
   curl -s "http://localhost:9091/api/v1/spam/windows" | jq '.'

4. CURRENT EPOCH:
   curl -s "http://localhost:9091/api/v1/epochs/active" | jq '{current_epoch, market}'

OUTPUT FORMAT:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
                      QUICK STATUS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

OVERALL:    [HEALTHY/DEGRADED/ERROR]

COMPONENTS:
  Dequeuer:        [RUNNING/STOPPED/ERROR]
  Spam-Aggregator: [RUNNING/STOPPED/ERROR]
  P2P-Gateway:     [RUNNING/STOPPED/ERROR]
  Event-Monitor:   [RUNNING/STOPPED/ERROR]

WINDOWS:
  Count:     [number]
  Latest:    [window ID or "none"]

EPOCH:
  Current:   [number]
  Boundary:  [yes/no]

SUMMARY:
  [One-line status description]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Be concise. Show actual command outputs. Do not ask to read files.
EOF

# Write footer
{
  echo ""
  echo "Completed: $(date)"
  echo "Log saved to: $LOG_FILE"
} >> "$LOG_FILE"

# Display output
cat "$LOG_FILE"
