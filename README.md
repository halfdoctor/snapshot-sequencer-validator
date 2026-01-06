# Powerloom Decentralized Snapshot Sequencer and Validator

## 🚀 Quick Start

For a complete guide on deploying and running a DSV node, see the **[DSV Node Setup Guide](docs/DSV_NODE_SETUP.md)**.

## Quick Commands

```bash
# Clone and setup
git clone https://github.com/powerloom/snapshot-sequencer-validator.git
cd snapshot-sequencer-validator
cp .env.example .env

# Start services (with monitoring)
./dsv.sh start

# View status
./dsv.sh status

# Access monitoring dashboard
./dsv.sh dashboard
```

## Features

### EIP-712 Signature Verification and Slot Validation

The snapshot sequencer implements EIP-712 signature verification for submissions to ensure authenticity and integrity.

#### Key Features
- Cryptographic signature verification using EIP-712 standard
- Signature-based snapshotter address recovery
- Optional slot validation against protocol state cache

#### Environment Variable: ENABLE_SLOT_VALIDATION
- **Purpose**: Enable optional validation of submission slots against protocol state
- **Default**: false
- **Requirement**: Requires protocol-state-cacher to be deployed

```bash
ENABLE_SLOT_VALIDATION=true
```

### Enhanced P2P Gateway Submission Monitoring

The p2p-gateway includes enhanced submission entity ID generation with detailed monitoring capabilities.

#### Enhanced Entity IDs
- **New Format**: `received:{epochID}:{slotID}:{projectID}:{timestamp}:{peerID}`
- **Legacy Format**: `received:peer-{peer-truncated}-{timestamp}:{timestamp}` (backward compatible)
- **Automatic Detection**: System automatically determines format based on message content

### DDoS Protection & Spam Prevention

The DSV node includes a multi-layer DDoS protection system to prevent malicious peers from flooding the network with invalid submissions.

#### Key Features
- **Peer ID-based tracking**: Primary identifier for spam detection (harder to spoof than Ethereum addresses)
- **Validator coordination**: Validators exchange spam reports via P2P before on-chain flagging
- **Dual enforcement**: Early rejection at P2P Gateway + defense in depth at Dequeuer
- **Consensus-based flagging**: Peers flagged on-chain when multiple validators agree
- **Rate limiting**: Enforces 2 submissions/epoch limit for lite nodes (whitelisted peers bypass)
- **Peer ID whitelisting**: Full nodes and bulk service snapshotters can be whitelisted by Peer ID

#### Environment Variables
```bash
# Enable spam protection
ENABLE_SPAM_PROTECTION=true
ENABLE_SPAM_REPORT_BROADCAST=true

# Peer ID Whitelisting (comma-separated libp2p peer IDs)
FULL_NODE_PEER_IDS=QmPeerID1,QmPeerID2
BULK_SERVICE_PEER_IDS=QmBulkPeerID1,QmBulkPeerID2

# Note: SPAM_AGGREGATION_WINDOW_SIZE is hardcoded to 10 in code for consensus consistency
```

#### Monitoring
- Prometheus metrics: `spam_reports_sent_total`, `spam_consensus_reached_total`, `spam_submissions_dropped_gateway_total`
- Redis keys: Per-epoch tracking, aggregation windows, flagged state cache
- Log messages: Flagged peer rejections, rate limit violations, consensus reached

For comprehensive documentation, see:
- **[DSV Node Setup Guide](docs/DSV_NODE_SETUP.md)**
- **[Spam Protection Monitoring Guide](docs/SPAM_PROTECTION_MONITORING.md)**
- **[DDoS Protection Plan](../ai-coord-docs/phase3/DDoS_PROTECTION_PLAN.md)**

## EIP-712 Signature Verification and Slot Validation

### Signature Verification
The snapshot sequencer now implements EIP-712 signature verification for submissions. This ensures the authenticity and integrity of submissions by verifying the cryptographic signature of the snapshotter.

#### Key Features
- Cryptographic signature verification using EIP-712 standard
- Signature-based snapshotter address recovery
- Optional slot validation against protocol state cache

### Slot Validation

#### Environment Variable: ENABLE_SLOT_VALIDATION
- **Purpose**: Enable optional validation of submission slots against protocol state
- **Default**: false
- **Requirement**: Requires protocol-state-cacher to be deployed

#### Configuration
To enable slot validation, set the following environment variable:
```bash
ENABLE_SLOT_VALIDATION=true
```

When enabled, the system will:
- Verify the submission slot against the protocol state cache
- Provide detailed error logging for signature verification failures
- Reject submissions that fail signature or slot validation

### Deployment Considerations
- Ensure proper configuration of ENABLE_SLOT_VALIDATION
- Have protocol-state-cacher deployed when slot validation is required
- Monitor logs for signature verification details

## Enhanced P2P Gateway Submission Monitoring

The p2p-gateway now includes enhanced submission entity ID generation with detailed monitoring capabilities.

### Features

#### Enhanced Entity IDs
- **New Format**: `received:{epochID}:{slotID}:{projectID}:{timestamp}:{peerID}`
- **Legacy Format**: `received:peer-{peer-truncated}-{timestamp}:{timestamp}` (maintained for backward compatibility)
- **Automatic Detection**: System automatically determines format based on message content

#### Detailed Metadata Storage
- Full submission metadata stored in Redis for 24 hours
- Includes epoch ID, slot ID, project ID, peer ID, timestamp, data market, node version
- Accessible via centralized Redis key patterns using the key builder

#### Monitoring Benefits
- **Better Attribution**: Clear mapping between submissions and epochs/slots/projects
- **Pattern Analysis**: Enables analysis of submission patterns by project or time period
- **Debugging Support**: Detailed context for troubleshooting submission issues
- **Performance Metrics**: Enhanced visibility into submission processing

### Implementation Details

#### Entity ID Generation
- Enhanced IDs used when message contains valid epochID, slotID, and projectID
- Legacy fallback for malformed or incomplete messages
- Full peer ID preservation for accurate attribution

#### Metadata Storage
- Namespaced Redis keys: `{protocol}:{dataMarket}:metrics:submissions:metadata:{entityID}`
- 24-hour TTL for automatic cleanup
- Centralized key management via pkgs/redis package

#### Monitoring Integration
- Enhanced logging shows detailed submission context
- Compatible with existing monitoring infrastructure
- Maintains existing timeline entries for backward compatibility

### Usage Examples

**Enhanced Entity ID:**
```
received:12345:678:project-alpha:1699123456:12D3KooWExamplePeerIDString
```

**Legacy Entity ID:**
```
received:peer-12D3KooW-1699123456:1699123456
```

This enhancement provides significantly better visibility into submission patterns and attribution while maintaining full backward compatibility with existing monitoring systems.
