# DSV Spam Protection Monitoring

Automated monitoring and debugging scripts for DSV spam protection and DDoS prevention.

## Scripts

### run-quick-status.sh
Fast health check for spam protection components, windows, and epoch.

```bash
cd /path/to/decentralized-sequencer
./monitoring/automation/spam-protection/run-quick-status.sh
```

### run-spam-check.sh
Full spam protection analysis following Steps 0-6. Checks for:
- Component initialization (dequeuer, spam-aggregator, event-monitor)
- Epoch processing and boundary detection
- Peer and snapshotter address tracking (including bulk service peers)
- Consecutive validation failure tracking
- Window creation (peer ID and snapshotter address aggregation)
- Collection window and consensus delay scheduling
- Redis tracking data

```bash
./monitoring/automation/spam-protection/run-spam-check.sh
```

### run-windows-debug.sh
Diagnostic for when `/api/v1/spam/windows` returns empty. Checks for:
- Spam-aggregator and event-monitor initialization
- Epoch boundary detection and window creation
- Collection window timers and report batching
- Consensus delay scheduling
- Redis window keys (both peer ID and snapshotter address windows)
- Tracking data existence

```bash
./monitoring/automation/spam-protection/run-windows-debug.sh
```

### test-bulk-service-flagging.sh
Test script for verifying snapshotter address flagging for bulk service peers. Checks:
- Peer ID whitelist configuration (BULK_SERVICE_PEER_IDS)
- Snapshotter address tracking (NOT peer ID tracking)
- Snapshotter address aggregation windows
- Report generation and collection (rate_limit_snapshotter)
- Consensus at window boundaries
- Snapshotter address flagging (peer ID remains whitelisted)
- Enforcement (P2P Gateway allows, Dequeuer rejects)

```bash
./monitoring/automation/spam-protection/test-bulk-service-flagging.sh
```

### test-peer-id-flagging.sh
Test script for verifying peer ID flagging for regular peers. Checks:
- Peer ID NOT in whitelist
- Peer ID tracking (both peer ID and snapshotter address)
- Peer ID aggregation windows
- Report generation and collection (rate_limit or validation_failure)
- Consensus at window boundaries
- Peer ID flagging (and associated snapshotter addresses)
- Enforcement (P2P Gateway rejects, Dequeuer rejects)

```bash
./monitoring/automation/spam-protection/test-peer-id-flagging.sh
```

### test-consensus-flow.sh
Test script for verifying consensus flow with 2 DSV nodes. Checks:
- Both DSV nodes running and connected
- Active validators (should be 2)
- Spam report broadcasting (local reports)
- Spam report receiving (remote reports)
- Report aggregation (validator counts)
- Consensus checking at window boundaries
- Flagging synchronization between nodes

```bash
# Run on both DSV nodes to verify synchronization
./monitoring/automation/spam-protection/test-consensus-flow.sh
```

## Structure

```
monitoring/
├── automation/
│   └── spam-protection/
│       ├── logs/
│       ├── README.md
│       ├── run-quick-status.sh
│       ├── run-spam-check.sh
│       ├── run-windows-debug.sh
│       ├── test-bulk-service-flagging.sh
│       ├── test-peer-id-flagging.sh
│       └── test-consensus-flow.sh
└── grafana/
```
