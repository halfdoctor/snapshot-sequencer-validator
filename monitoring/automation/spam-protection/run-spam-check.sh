#!/bin/bash

# DSV Spam Protection Monitoring Script
# Full spam protection health check following Steps 0-6
# Run from the decentralized-sequencer repository root

LOG_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/logs"
DSV_DIR="/home/ubuntu/snapshot-sequencer-validator"
MONITORING_DOC="$DSV_DIR/docs/SPAM_PROTECTION_MONITORING.md"

mkdir -p "$LOG_DIR"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_FILE="$LOG_DIR/spam-check-${TIMESTAMP}.txt"

# Write header
{
  echo "=== DSV Spam Protection Check ==="
  echo "Started: $(date)"
  echo "Running on: $(hostname)"
  echo "DSV Directory: $DSV_DIR"
  echo ""
} | tee "$LOG_FILE"

# Run Claude with inline prompt (bypass all permission checks)
claude -p --permission-mode bypassPermissions >> "$LOG_FILE" 2>&1 <<'EOF'
You are a DSV monitoring assistant analyzing spam protection on this system.

TASK: Execute Steps 0-6 from the SPAM_PROTECTION_MONITORING guide to diagnose spam protection status.

Change to DSV directory and run the following checks sequentially:
cd /home/ubuntu/snapshot-sequencer-validator

Step 0: Verify DDoS Protection Components Initialized
- Check dequeuer logs for initialization messages
- Check spam-aggregator logs for startup and Redis connection
- Verify event-monitor logs for spam component initialization

Step 1: Check if EventMonitor is Processing Epochs
- Look for "EpochReleased" events
- Check for "aggregation window boundary" detection
- Verify epoch boundary processing

Step 2: Check if Tracking is Happening
- Check dequeuer logs for "tracked.*submission" or "tracked.*validation"
- Look for peer tracking activity

Step 3: Check Current Epoch and Window Boundaries
- Get current epoch from API
- Determine if current epoch is a boundary (multiple of 10)

Step 4: Check Redis for Epoch Tracking Data
- Verify epoch peer sets exist
- Check for windows master set

Step 5: Check Monitoring API for Epoch Activity
- Query /api/v1/spam/epochs
- Query /api/v1/spam/windows

Step 6: Verify Event-Monitor Window Creation
- Check for "Successfully created spam aggregation window" messages
- Verify window creation at boundaries

FINAL OUTPUT FORMAT:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
                    SPAM PROTECTION STATUS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

COMPONENT INITIALIZATION:
  Dequeuer:      [OK/ERROR] - brief status
  Spam-Aggregator: [OK/ERROR] - brief status
  Event-Monitor:     [OK/ERROR] - brief status

EPOCH PROCESSING:
  Current Epoch:    [number]
  Is Boundary:      [yes/no]
  Epochs to Next:   [number]
  Event Release:    [DETECTED/NOT DETECTED]

TRACKING STATUS:
  Active:           [YES/NO]
  Peers Tracked:    [count if available]

WINDOW CREATION:
  Windows Exist:    [YES/NO]
  Last Window:      [ID if available]
  Creation Status:  [WORKING/NOT WORKING]

REDIS DATA:
  Epoch Keys:       [EXISTS/EMPTY]
  Window Keys:      [EXISTS/EMPTY]

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
