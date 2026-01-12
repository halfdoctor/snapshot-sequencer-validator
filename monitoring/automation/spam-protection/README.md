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

## Structure

```
monitoring/
├── automation/
│   └── spam-protection/
│       ├── logs/
│       ├── README.md
│       ├── run-quick-status.sh
│       ├── run-spam-check.sh
│       └── run-windows-debug.sh
└── grafana/
```
