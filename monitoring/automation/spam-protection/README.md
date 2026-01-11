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
Full spam protection analysis following Steps 0-6.

```bash
./monitoring/automation/spam-protection/run-spam-check.sh
```

### run-windows-debug.sh
Diagnostic for when `/api/v1/spam/windows` returns empty.

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
