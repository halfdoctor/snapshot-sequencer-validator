# DSV vs Centralized Sequencer Comparison Script

This script verifies that the DSV (Decentralized Sequencer Validator) network produces equivalent results to the centralized sequencer before migration.

## Purpose

The script performs two critical comparisons:

1. **Submission Verification**: Ensures all submissions from the full node (`snapshotter-core-edge`) appear in Level 2 aggregated batches from DSV nodes
2. **On-Chain Commitment Verification**: Ensures on-chain commitments match between legacy and new protocol state contracts

## Prerequisites

```bash
pip install redis web3 eth-utils
```

## Usage

### Single Epoch Comparison

```bash
python scripts/compare_dsv_vs_centralized.py \
    --epoch-id <epoch_id> \
    --protocol <new_protocol_state_address> \
    --market <new_data_market_address> \
    --legacy-protocol <legacy_protocol_state_address> \
    --legacy-market <legacy_data_market_address> \
    --rpc-url <ethereum_rpc_url> \
    [--redis-host localhost] \
    [--redis-port 6379]
```

### Multi-Epoch Report

Generate a report for multiple epochs (starting from the specified epoch and going backwards):

```bash
python scripts/compare_dsv_vs_centralized.py \
    --epoch-id <epoch_id> \
    --num-epochs 10 \
    --protocol <new_protocol_state_address> \
    --market <new_data_market_address> \
    --legacy-protocol <legacy_protocol_state_address> \
    --legacy-market <legacy_data_market_address> \
    --rpc-url <ethereum_rpc_url> \
    [--redis-host localhost] \
    [--redis-port 6379]
```

### Example

```bash
python scripts/compare_dsv_vs_centralized.py \
    --epoch-id 23988780 \
    --num-epochs 10 \
    --protocol 0xC9e7304f719D35919b0371d8B242ab59E0966d63 \
    --market 0xb6c1392944a335b72b9e34f9D4b8c0050cdb511f \
    --legacy-protocol 0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401 \
    --legacy-market 0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d \
    --rpc-url https://devnet-orbit-rpc.aws2.powerloom.io/rpc?uuid=9dff8954-24f0-11f0-b88f-devnet \
    --redis-port 6380
```

## What It Checks

### 1. Submission Verification

- Extracts Level 2 aggregated batch from Redis for the specified epoch
- Identifies full node submissions from the Level 2 batch (from `snapshotter-core-edge`)
- Verifies that all full node submission CIDs appear in the Level 2 batch

**Redis Keys Used:**
- `{protocol}:{market}:batch:aggregated:{epochId}` (STRING) - Level 2 aggregated batch

**Note:** The script uses **legacy** protocol/market addresses for Redis keys, as all Redis data (including Level 2 batches) is keyed by legacy addresses.

### 2. On-Chain Commitment Verification

- Queries new protocol state contract for epoch commitments
- Queries legacy protocol state contract for epoch commitments
- Compares:
  1. **Project IDs**: All project IDs should be present in both contracts
  2. **Snapshot CIDs**: Finalized snapshot CID for each project should match between contracts

**Important:** Batch CIDs are **NOT** compared because:
- CIDv0 vs CIDv1 format differences
- Different payload structures between legacy and new contracts
- Batch CIDs may differ even when the underlying data is equivalent

**Contract Mappings Used:**
- `epochIdToBatchCids[epochId]` → Array of batch CIDs for epoch
- `batchCidToProjects[batchCid]` → Array of project IDs in batch
- `snapshotStatus[projectId][epochId].snapshotCid` → Individual project snapshot CID

**Contract Functions Used (via ProtocolState):**
- `epochIdToBatchCids(dataMarket, epochId)` → Returns all batch CIDs
- `batchCidToProjects(dataMarket, batchCid)` → Returns projects in batch
- `snapshotStatus(dataMarket, projectId, epochId)` → Returns (status, cid, timestamp)

**Note:** The script uses **new** protocol/market addresses for on-chain contract queries, as these are the contracts where DSV nodes submit batches.

## Output

The script outputs:

1. **Console Output**: Step-by-step progress and comparison results for each epoch
2. **JSON Results**: Complete comparison data in JSON format

### Single Epoch Output

```
================================================================================
Comparing DSV vs Centralized Sequencer for Epoch 23988780
================================================================================

📦 Extracting Level 2 aggregated batch from Redis...
   Level 2 batch found with 29 projects
   IPFS CID: /ipfs/QmfP6jpeGwFsYLq8zFQCHQS8LLxBh2Q4UwK4iHuvzAjZDB
   Merkle Root: wLemEHiivJFKsHxg1Ff01c759CsnqTxSIBJ6xVawsVI=

🔍 Extracting full node submissions from Level 2 batch...
   Found 29 projects with submissions
   Total submission CIDs: 29

🔬 Verifying submissions are present in Level 2 batch...
   ✅ All submissions present in Level 2 batch

⛓️  Comparing on-chain commitments...
   Querying new contract...
   Querying legacy contract...
   New contract: 1 batch(es), 29 projects
   Legacy contract: 1 batch(es), 29 projects
   ✅ On-chain commitments match between legacy and new contracts

================================================================================
SUMMARY - Epoch 23988780
================================================================================
Submission Verification: ✅ PASS
On-Chain Verification: ✅ PASS
================================================================================
```

### Multi-Epoch Output

When using `--num-epochs`, the script will:
- Process each epoch sequentially (from `epoch-id` down to `epoch-id - num-epochs + 1`)
- Show individual summaries for each epoch
- Display an overall summary at the end

```
================================================================================
OVERALL SUMMARY
================================================================================
Total Epochs Checked: 10
Submission Verification: 10/10 passed
On-Chain Verification: 10/10 passed
================================================================================
```

## Troubleshooting

### No submissions found

- Verify Redis connection and that submissions exist for the epoch
- Check that protocol/market addresses match your DSV node configuration (use **legacy** addresses for Redis keys)
- Ensure epoch ID is correct and submissions have been processed

### Level 2 batch not found

- Verify Level 2 aggregation has completed for the epoch
- Check aggregator logs for errors
- Ensure aggregation window has expired (default 30 seconds)
- Verify you're using **legacy** protocol/market addresses for Redis keys

### On-chain queries fail

- Verify RPC URL is accessible
- Check contract addresses are correct:
  - Use **new** protocol/market addresses for on-chain queries
  - Use **legacy** protocol/market addresses for Redis keys
- Ensure ABI file exists at `abi/PowerloomProtocolState.abi.json`
- Verify epoch has been committed on-chain via `submitSubmissionBatch()`
- Check that both centralized sequencer (legacy) and DSV nodes (new) have submitted batches

### Missing submissions in Level 2

- Check P2P network connectivity between full node and DSV nodes
- Verify submissions were received by DSV nodes (check dequeuer logs)
- Check Level 1 aggregation completed successfully
- Verify aggregation window timing

### Project ID or Snapshot CID mismatches

- Verify both contracts have the same projects for the epoch
- Check that snapshot CIDs match for each project
- Ensure both centralized sequencer and DSV nodes are processing the same epoch data
- Check for timing issues (one contract may be ahead of the other)

## Integration with Monitoring

This script can be integrated into monitoring pipelines to:

- Run periodically after each epoch finalization
- Generate reports for multiple epochs (use `--num-epochs`)
- Alert on mismatches between DSV and centralized sequencer
- Validate migration readiness before switching traffic
- Track consistency over time with multi-epoch reports

## Related Documentation

- [AGGREGATOR_WORKFLOW.md](../docs/AGGREGATOR_WORKFLOW.md): Level 2 aggregation details
- [PROTOCOL_META_REFERENCE.md](../../ai-coord-docs/specs/PROTOCOL_META_REFERENCE.md): Complete protocol workflow
- [PROTOCOL_STATE_TRANSITIONS.md](../../ai-coord-docs/specs/PROTOCOL_STATE_TRANSITIONS.md): State machine documentation
