# DSV (Decentralized Sequencer Validator) Node Setup Guide

This comprehensive guide walks you through deploying and configuring a DSV node for the Powerloom snapshot sequencer and validator network.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start](#quick-start)
3. [Environment Configuration](#environment-configuration)
4. [Component Architecture](#component-architecture)
5. [Deployment Options](#deployment-options)
6. [P2P Network Setup](#p2p-network-setup)
7. [Monitoring and Operations](#monitoring-and-operations)
8. [Troubleshooting](#troubleshooting)
9. [Advanced Configuration](#advanced-configuration)
10. [VPA (Validator Priority Assigner) Integration](#vpa-validator-priority-assigner-integration)

---

## Prerequisites

### System Requirements

- **Operating System**: Linux (Ubuntu 20.04+ recommended for production)
- **Docker**: Docker Engine 20.10+ or Docker Compose 2.0+
- **Go**: Go 1.25+ (for building from source)
- **RAM**: 4GB minimum, 8GB+ recommended
- **Storage**: 20GB+ free space
- **Network**: Port 9001 (P2P) and 6379 (Redis) must be accessible

### Network Requirements

- **Open Ports**:
  - `9001` (P2P Gateway) - may be different based on `P2P_PORT` config
  - `6379` (Redis) - internal Docker network
  - `5001` (IPFS API) - optional, if using IPFS
  - `8080` (Monitor API) - optional
- **Firewall**: Ensure external access to P2P port
- **NAT**: Public IP must be accessible (no NAT masquerading)

### Software Dependencies

```bash
# Install Docker (Ubuntu/Debian)
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/download/1.29.2/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Install Go (if building from source)
wget https://go.dev/dl/go1.25.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

---

## Quick Start

### Option 1: Production Deployment (Recommended)

```bash
# Clone the repository
git clone https://github.com/powerloom/snapshot-sequencer-validator.git
cd snapshot-sequencer-validator

# Copy and configure environment
cp .env.example .env
nano .env

# Create data directory for IPFS (if using IPFS)
sudo mkdir -p /data/ipfs
sudo chown -R 1000:1000 /data/ipfs
sudo chmod -R 755 /data/ipfs

# Start services with monitoring
./dsv.sh start

# Or start with IPFS support
./dsv.sh start --with-ipfs

# Or start with VPA (Validator Priority Assigner) support
./dsv.sh start --with-vpa

# Or start with both IPFS and VPA
./dsv.sh start --with-ipfs --with-vpa
```

### Option 2: Development Mode (Single Container)

```bash
# Start unified sequencer for testing
./dsv.sh dev
```

### Option 3: Manual Docker Compose

```bash
# Build and start specific components
docker compose -f docker-compose.separated.yml up -d

# Scale components as needed
docker compose -f docker-compose.separated.yml up -d --scale dequeuer=3 --scale finalizer=2
```

---

## Environment Configuration

### Core Configuration (.env file)

Create `.env` file from `.env.example` and configure essential settings:

```bash
# Unique identifier for this sequencer
SEQUENCER_ID=validator-001

# P2P networking
P2P_PORT=9001
BOOTSTRAP_MULTIADDR=/ip4/BOOTSTRAP_IP/tcp/PORT/p2p/PEER_ID
RENDEZVOUS_POINT=powerloom-snapshot-sequencer-network
PUBLIC_IP=YOUR_SERVER_IP

# Redis configuration
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=

# Component toggles (true/false)
ENABLE_LISTENER=true
ENABLE_DEQUEUER=true
ENABLE_FINALIZER=true
ENABLE_BATCH_AGGREGATION=true
ENABLE_EVENT_MONITOR=false
```

### Example Configurations

#### Local Development
```bash
# .env for local development
SEQUENCER_ID=local-dev-1
P2P_PORT=9001
REDIS_HOST=localhost
REDIS_PORT=6379
PRIVATE_KEY=
PUBLIC_IP=127.0.0.1
DEBUG_MODE=true
```

#### Production
```bash
# .env for production
SEQUENCER_ID=prod-validator-001
P2P_PORT=9001
BOOTSTRAP_MULTIADDR=/ip4/123.45.67.89/tcp/9100/p2p/12D3KooWEXAMPLEPEERID
RENDEZVOUS_POINT=powerloom-snapshot-sequencer-network
PUBLIC_IP=123.45.67.89
REDIS_HOST=redis
REDIS_PASSWORD=secure_password_here
PRIVATE_KEY=your_hex_private_key_here
DEBUG_MODE=false
```

---

## Component Architecture

The DSV system consists of modular components that can be deployed independently:

### Core Components

1. **P2P Gateway** - Singleton handling all P2P communication
   - Listens on port `P2P_PORT` (default: 9001)
   - Manages peer discovery and gossipsub topics
   - Routes messages between components

2. **Dequeuer** - Processes submissions from Redis queue
   - Scalable: multiple replicas supported
   - Validates submissions and removes duplicates
   - Handles EIP-712 signature verification

3. **Finalizer** - Creates batches and stores in IPFS
   - Parallel batch processing
   - Uploads to IPFS and tracks CIDs
   - Can be scaled horizontally

4. **Aggregator** - Consensus and batch aggregation
   - Coordinates between validators
   - Implements consensus algorithm
   - Handles batch exchange and voting

5. **Event Monitor** - Watches blockchain events
   - Tracks EpochReleased events
   - Manages submission windows
   - Optional, disabled by default

### Support Components

6. **State Tracker** - Data aggregation for monitoring
7. **Monitor API** - REST API for monitoring and debugging
8. **IPFS Node** - Optional local IPFS service

---

## Deployment Options

### Production Deployment (Separated Architecture)

```bash
# Start production services with monitoring
./dsv.sh start

# Start with IPFS and monitoring
./dsv.sh start --with-ipfs --with-monitoring

# Scale components
docker compose -f docker-compose.separated.yml up -d --scale dequeuer=3
```

### Components and Profiles

#### Base Profile (Essential Services)
- P2P Gateway
- Dequeuer (2 replicas)
- Finalizer (2 replicas)
- Aggregator
- Redis

#### Monitoring Profile
- State Tracker
- Monitor API (port 9091)
- Prometheus (port 9090)
- Grafana (port 3000)

#### IPFS Profile
- IPFS Node (port 5001 API, 8080 Gateway)
- Automatic cleanup and management

#### VPA Profile
- relayer-py service (port 8080)
- Multi-signer transaction relayer for new contracts
- Automatic repository cloning and configuration

### Environment Variables for Deployment

```bash
# Component scaling
DEQUEUER_REPLICAS=3
FINALIZER_REPLICAS=2

# Performance tuning
DEQUEUER_WORKERS=10
FINALIZER_WORKERS=8
FINALIZATION_BATCH_SIZE=50

# P2P networking
# CRITICAL: DSV nodes serve thousands of snapshotter peers (local collectors)
# These limits must be high enough to prevent pruning publishers
CONN_MANAGER_LOW_WATER=2000   # Minimum connections (was 100 - too low for DSV)
CONN_MANAGER_HIGH_WATER=5000  # Maximum before pruning (was 400 - way too low for DSV)

# Storage
STORAGE_PROVIDER=ipfs
IPFS_HOST=ipfs:5001
```

---

## P2P Network Setup

### Bootstrap Nodes

Bootstrap nodes are essential for peer discovery. Each node needs to connect to at least one bootstrap node:

```bash
# Configure bootstrap nodes in .env
BOOTSTRAP_MULTIADDR=/ip4/192.168.1.100/tcp/9100/p2p/12D3KooWEXAMPLE1
BOOTSTRAP_MULTIADDR=/ip4/192.168.1.101/tcp/9100/p2p/12D3KooWEXAMPLE2
```

### Peer Discovery

The system uses libp2p with gossipsub for peer discovery:

- **Discovery Topic**: `RENDEZVOUS_POINT` (default: `powerloom-snapshot-sequencer-network`)
- **Message Topics**:
  - `/powerloom/{prefix}/snapshot-submissions/0` - Discovery topic (peer discovery)
  - `/powerloom/{prefix}/snapshot-submissions/all` - Actual submissions
  - `/powerloom/finalized-batches/all` - Batch consensus

### Heartbeat Message Handling

The DSV node recognizes and handles heartbeat messages from local collectors to prevent unnecessary processing overhead. Local collectors publish two types of heartbeat messages:

#### Type 1: Discovery Topic Heartbeat

**Format:**
```json
{
  "epoch_id": 0,
  "submissions": null,
  "snapshotter_id": "...",
  "signature": "..."
}
```

**DSV Processing:**
- **Detection**: `epoch_id == 0 && submissions == null` (unified/main.go line 731)
- **Action**: Skipped immediately at queue level (no processing overhead)
- **Logging**: Debug level: `💓 Heartbeat received from {peer} (skipping queue)`

#### Type 2: Submissions Topic Heartbeat

**Format:**
```json
{
  "epoch_id": 0,
  "submissions": [{
    "request": {
      "epoch_id": 0,
      "project_id": "test:mesh-formation:local-collector",
      "snapshot_cid": ""
    }
  }],
  "snapshotter_id": "...",
  "signature": "..."
}
```

**DSV Processing:**
- **Detection**: `EpochId == 0 && SnapshotCid == ""` (dequeuer.go line 229)
- **Action**: Queued but skipped during validation (recognized as heartbeat)
- **Error Handling**: Validation returns `"epoch 0 heartbeat: skipping"` error, caught and logged as debug (unified/main.go line 1023)
- **Logging**: Debug level: `Skipped epoch 0 heartbeat (P2P mesh maintenance)`

**Why Two Types?**
- Discovery topic uses `null` submissions for minimal overhead
- Submissions topic uses non-nil array with empty CID to help mesh formation while still being recognized as heartbeat

Both heartbeat types help maintain mesh connectivity and prevent pruning, but are automatically skipped by the DSV node to avoid processing overhead.

### Network Testing

Test network connectivity:

```bash
# Check if P2P gateway is running
curl http://localhost:9001/debug/peers

# Test peer connections
telnet YOUR_SERVER_IP 9001

# Check mesh status
curl http://localhost:9001/metrics | grep mesh
```

### Multiple Bootstrap Support

For enhanced network resilience, configure multiple bootstrap nodes:

```bash
# In .env file
# Format: comma-separated list of multiaddrs
BOOTSTRAP_PEERS=/ip4/192.168.1.100/tcp/9100/p2p/12D3KooWEXAMPLE1,/ip4/192.168.1.101/tcp/9100/p2p/12D3KooWEXAMPLE2
```

> [!TIP]
> For a single bootstrap node, just use the single multiaddr in the BOOTSTRAP_PEERS variable, without the comma.

---

## Monitoring and Operations

### Service Management

```bash
# Start all services
./dsv.sh start

# Stop all services
./dsv.sh stop

# Restart services
./dsv.sh restart

# Show service status
./dsv.sh status

# View logs
./dsv.sh logs

# Monitor specific services
./dsv.sh p2p-logs
./dsv.sh aggregator-logs
./dsv.sh dequeuer-logs
./dsv.sh finalizer-logs
```

### Monitoring Dashboard

Access the monitoring dashboard at: `http://localhost:9091/swagger/index.html`

Key endpoints:
- Health check: `GET /api/v1/health`
- Aggregation results: `GET /api/v1/aggregation/results`
- Component status: `GET /api/v1/status`

### Metrics Collection

```bash
# Check Redis streams
./dsv.sh stream-info

# Check consumer groups
./dsv.sh stream-groups

# Check dead letter queue
./dsv.sh stream-dlq

# Reset streams (dangerous!)
./dsv.sh stream-reset
```

### Health Checks

```bash
# Check P2P gateway health
curl http://localhost:9001/debug/health

# Check Redis health
docker exec redis redis-cli ping

# Check IPFS health (if using IPFS)
curl http://localhost:5001/api/v0/id
```

---

## Troubleshooting

### Common Issues

#### 1. Services Won't Start

```bash
# Check Docker status
docker ps

# Check logs for errors
./dsv.sh logs

# Verify .env configuration
cat .env
```

#### 2. P2P Connection Issues

```bash
# Test bootstrap connectivity
telnet BOOTSTRAP_IP BOOTSTRAP_PORT

# Check P2P peer connections
curl http://localhost:9001/debug/peers

# Verify public IP is accessible
curl ifconfig.me
```

#### 3. Redis Connection Issues

```bash
# Check Redis container status
docker ps | grep redis

# Test Redis connectivity
docker exec redis redis-cli ping

# Check Redis memory usage
docker exec redis redis-cli INFO memory
```

#### 4. IPFS Issues

```bash
# Check IPFS status
docker exec ipfs ipfs id

# Check IPFS storage
curl http://localhost:5001/api/v0/repo/stat

# Restart IPFS if needed
./dsv.sh stop && ./dsv.sh start --with-ipfs
```

### Debug Mode

Enable debug logging for troubleshooting:

```bash
# Enable debug mode
DEBUG_MODE=true
./dsv.sh restart

# View detailed logs
./dsv.sh logs
```

### Resource Cleanup

#### Quick Cleanup (dsv.sh commands)

```bash
# Clean Redis cache (stale keys)
./dsv.sh clean-cache

# Clean stale aggregation queue
./dsv.sh clean-queue

# Clean timeline scientific notation issues
./dsv.sh clean-timeline

# Remove all containers and volumes
./dsv.sh clean
```

#### Advanced Cleanup (cleanup_old_redis_keys.py)

For comprehensive Redis key cleanup, especially after refactoring or when dealing with memory pressure:

**Discovery** (see what keys exist):
```bash
python3 scripts/cleanup_old_redis_keys.py --discover
```

**Clean all markets automatically** (recommended for multi-market deployments):
```bash
# Dry run first
python3 scripts/cleanup_old_redis_keys.py --all-markets --keep-hours 24 --cleanup-queues --dry-run

# Actually clean
python3 scripts/cleanup_old_redis_keys.py --all-markets --keep-hours 24 --cleanup-queues
```

**Clean queues/streams** (no protocol/market needed):
```bash
python3 scripts/cleanup_old_redis_keys.py --cleanup-queues --queue-max-length 1000
python3 scripts/cleanup_old_redis_keys.py --cleanup-streams --stream-max-length 10000
```

**Clean non-namespaced timelines** (no protocol/market needed):
```bash
python3 scripts/cleanup_old_redis_keys.py --keep-hours 24
```

**Clean specific protocol/market** (requires protocol/market addresses):
```bash
python3 scripts/cleanup_old_redis_keys.py --keep-epochs 60 --protocol 0x1234... --market 0x5678...
```

See [REDIS_KEYS.md](./REDIS_KEYS.md#manual-redis-key-cleanup) for detailed cleanup documentation and troubleshooting.

---

## Advanced Configuration

### Component Optimization

#### Dequeuer Configuration
```bash
# Number of parallel workers
DEQUEUER_WORKERS=10

# Maximum submissions per epoch
MAX_SUBMISSIONS_PER_EPOCH=1000

# Deduplication settings
DEDUP_ENABLED=true
DEDUP_LOCAL_CACHE_SIZE=10000
```

#### Finalizer Configuration
```bash
# Batch processing
FINALIZER_WORKERS=8
FINALIZATION_BATCH_SIZE=50

# Storage configuration
STORAGE_PROVIDER=ipfs
IPFS_HOST=127.0.0.1:5001
```

#### Aggregator Configuration
```bash
# Consensus parameters (testing only)
VOTING_THRESHOLD=0.67
MIN_VALIDATORS=3
BATCH_AGGREGATION_TIMEOUT=300

# Network aggregation
AGGREGATION_WINDOW_SECONDS=30
```

### Performance Tuning

#### P2P Network Optimization
```bash
# Connection management
CONN_MANAGER_LOW_WATER=200
CONN_MANAGER_HIGH_WATER=800

# Gossipsub settings
GOSSIPSUB_HEARTBEAT_MS=700
```

#### Memory Management
```bash
# Redis memory limit
# Set in docker-compose.separated.yml
# command: redis-server --appendonly yes --maxmemory 8gb
```

### Security Configuration

#### Identity Verification
```bash
# Enable full verification
ENABLE_SLOT_VALIDATION=true
SKIP_IDENTITY_VERIFICATION=false
CHECK_FLAGGED_SNAPSHOTTERS=true

# Verification cache
VERIFICATION_CACHE_TTL=600
```

### Multi-Node Deployment

For multi-node deployments:

1. **Unique Sequencer IDs**: Each node must have a unique `SEQUENCER_ID`
2. **Bootstrap Nodes**: Use dedicated bootstrap nodes for production
3. **Load Balancing**: Distribute dequeuer and finalizer replicas across nodes
4. **Network Isolation**: Ensure proper firewall rules between nodes

### Data Persistence

#### Redis Data
```bash
# Data persists in Docker volumes
# Location depends on Docker configuration
docker volume inspect sequencer-net_redis-data
```

#### IPFS Data
```bash
# Configure IPFS data location
IPFS_DATA_DIR=/mnt/storage/ipfs

# Create directory with proper permissions
sudo mkdir -p /mnt/storage/ipfs
sudo chown -R 1000:1000 /mnt/storage/ipfs
```

---

## VPA (Validator Priority Assigner) Integration

The VPA system enables priority-based batch submission to protocol contracts using a multi-signer approach.

### VPA Architecture Overview

The VPA integration adds a Python-based relayer service that handles transaction submission:

1. **relayer-py Service**: Multi-signer transaction relayer
2. **Priority Caching**: Redis-based validator priority storage
3. **Automatic Setup**: Repository cloning and configuration via dsv.sh

### VPA Environment Configuration

Add these variables to your `.env` file:

```bash
# Protocol contract addresses
PROTOCOL_STATE_CONTRACT=0xC9e7304f719D35919b0371d8B242ab59E0966d63
DATA_MARKET_ADDRESSES=0xb6c1392944a335b72b9e34f9D4b8c0050cdb511f

# relayer-py service endpoint
RELAYER_PY_ENDPOINT=http://relayer-py:8080

# VPA validator address (for priority checking)
VPA_VALIDATOR_ADDRESS=0xYourValidatorAddress

# Multi-signer configuration (comma-separated)
VPA_SIGNER_ADDRESSES=0xSIGNER1_ADDRESS,0xSIGNER2_ADDRESS
VPA_SIGNER_PRIVATE_KEYS=0xSIGNER1_PRIVATE_KEY,0xSIGNER2_PRIVATE_KEY

# RPC timeouts and connection pooling (relayer-py, mitigates "Timeout on reading data from socket")
# Precedence: RPC_SOCK_READ_TIMEOUT_S overrides RPC_REQUEST_TIMEOUT_S when both set
RPC_REQUEST_TIMEOUT_S=60
RPC_SOCK_READ_TIMEOUT_S=60
RPC_MAX_CONNECTIONS=100
RPC_MAX_KEEPALIVE_CONNECTIONS=50
RPC_KEEPALIVE_EXPIRY_S=300
```

### VPA Deployment

#### Option 1: Quick Start with VPA

```bash
# Start with VPA support
./dsv.sh start --with-vpa

# Start with both IPFS and VPA
./dsv.sh start --with-ipfs --with-vpa

# Start with monitoring and VPA
./dsv.sh start --with-monitoring --with-vpa
```

#### Option 2: Manual VPA Setup

```bash
# Clone relayer-py repository (done automatically by dsv.sh)
git clone git@github.com:powerloom/relayer-py.git

# Generate settings.json from environment variables
python3 ./test_relayer_config.py

# Copy settings to relayer-py
cp /tmp/test_relayer_settings.json ./relayer-py/settings/settings.json

# Build and start relayer-py
cd relayer-py
docker build -t relayer-py .
docker run -p 8080:8080 relayer-py
```

### VPA Service Management

```bash
# Check VPA service status
docker ps | grep relayer-py

# View VPA service logs
docker logs dsv-relayer-py

# Test VPA health
curl http://localhost:8080/health

# Restart VPA service
./dsv.sh stop && ./dsv.sh start --with-vpa
```

### VPA Contract Behavior

- Submits to ProtocolState and DataMarket contracts
- Priority-based submission via VPA authorization
- Multi-signer support for higher throughput
- Automatic priority assignment and window timing

### Multi-Signer Configuration

The VPA system supports multiple authorized signers per validator for parallel batch submission:

```bash
# Example: 2 signers for load balancing
VPA_SIGNER_ADDRESSES=0x123...,0x456...
VPA_SIGNER_PRIVATE_KEYS=0xabc...,0xdef...

# Each signer can submit independently
# Load balancing handled by relayer-py PM2 workers
```

### VPA Testing and Validation

Use the built-in test suite to validate VPA configuration:

```bash
# Run VPA deployment test
./test_vpa_deployment.sh

# Test validates:
# - Environment variable loading
# - Settings.json generation
# - Repository access
# - Docker build requirements
# - Complete workflow end-to-end
```

### VPA Monitoring

The VPA service integrates with the DSV monitoring system:

- **Health Endpoint**: `http://localhost:8080/health`
- **Service Logs**: Available via `./dsv.sh logs`
- **Redis Metrics**: Priority caching statistics
- **Contract Events**: Priority assignments tracked in EventMonitor

### Security Considerations

- **SSH Access**: Ensure SSH keys are configured for `git@github.com:powerloom/relayer-py.git`
- **Key Management**: Private keys are stored in environment variables, not in code
- **Network Isolation**: VPA service runs in isolated Docker container
- **Rate Limiting**: Built-in rate limiting prevents abuse

---

## Support and Community

### Getting Help

- **Documentation**: Check `/docs` directory for detailed guides
- **Monitoring**: Use the dashboard at `http://localhost:9091/swagger`
- **Logging**: View logs with `./dsv.sh logs`

### Issue Reporting

When reporting issues, please include:
- System information (OS, Docker version)
- Configuration files (redacted sensitive data)
- Full logs from all components
- Network connectivity information

### Contributing

To contribute to the DSV project:
1. Fork the repository
2. Create a feature branch
3. Test changes thoroughly
4. Submit a pull request with detailed documentation

---

*Last Updated: November 2025*