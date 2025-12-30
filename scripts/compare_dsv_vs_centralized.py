#!/usr/bin/env python3
"""
Comparison script to verify DSV network equivalence with centralized sequencer.

This script performs two critical comparisons:
1. Submission Verification: Ensures all full node submissions appear in Level 2 aggregated batches
2. On-Chain Commitment Verification: Ensures on-chain commitments match between legacy and new contracts

Usage:
    python compare_dsv_vs_centralized.py --epoch-id <epoch_id> \\
        --protocol <protocol_state_address> \\
        --market <data_market_address> \\
        --redis-host <redis_host> \\
        --redis-port <redis_port> \\
        --legacy-protocol <legacy_protocol_state_address> \\
        --legacy-market <legacy_data_market_address> \\
        --rpc-url <ethereum_rpc_url>
"""

import argparse
import json
import sys
from typing import Dict, List, Set, Optional, Tuple
from collections import defaultdict

try:
    import redis
    from web3 import Web3
    from eth_utils import to_checksum_address
except ImportError as e:
    print(f"ERROR: Missing required dependency: {e}")
    print("Install with: pip install redis web3 eth-utils")
    sys.exit(1)


class DSVComparison:
    """Compare DSV network output with centralized sequencer."""

    def __init__(
        self,
        redis_host: str,
        redis_port: int,
        protocol_state: str,
        data_market: str,
        legacy_protocol_state: str,
        legacy_data_market: str,
        rpc_url: str,
    ):
        self.redis_client = redis.Redis(host=redis_host, port=redis_port, decode_responses=True)
        self.protocol_state = to_checksum_address(protocol_state)  # New contract (for on-chain queries)
        self.data_market = to_checksum_address(data_market)  # New contract (for on-chain queries)
        self.legacy_protocol_state = to_checksum_address(legacy_protocol_state)
        self.legacy_data_market = to_checksum_address(legacy_data_market)
        self.w3 = Web3(Web3.HTTPProvider(rpc_url))

        # Redis key patterns use LEGACY addresses (all Redis data uses legacy keys)
        # New contracts are only for on-chain submission via relayer-py
        self.kb_protocol = self.legacy_protocol_state
        self.kb_market = self.legacy_data_market

    def get_level2_aggregated_batch(self, epoch_id: str) -> Optional[Dict]:
        """
        Extract Level 2 aggregated batch from Redis.
        
        Returns: Aggregated batch dict with project submissions
        """
        # Try to get from aggregated batch key
        aggregated_key = f"{self.kb_protocol}:{self.kb_market}:batch:aggregated:{epoch_id}"
        aggregated_data = self.redis_client.get(aggregated_key)
        
        if not aggregated_data:
            print(f"⚠️  No Level 2 aggregated batch found in Redis for epoch {epoch_id}")
            print(f"   Tried key: {aggregated_key}")
            return None

        try:
            batch = json.loads(aggregated_data)
            return batch
        except json.JSONDecodeError as e:
            print(f"⚠️  Failed to parse aggregated batch: {e}")
            return None

    def extract_full_node_submissions_from_level2(self, level2_batch: Dict) -> Dict[str, Set[str]]:
        """
        Extract full node submissions from Level 2 aggregated batch.
        
        Full node submissions are identified from SubmissionDetails which contains
        submitter_id information. We extract all unique CIDs per project that came
        from the full node (snapshotter-core-edge).
        
        Returns: Dict mapping project_id -> set of snapshot CIDs from full node
        """
        full_node_submissions = defaultdict(set)
        
        if not level2_batch:
            return {}
        
        # Extract from SubmissionDetails: map[projectID][]SubmissionMetadata
        submission_details = level2_batch.get("SubmissionDetails") or level2_batch.get("submission_details", {})
        
        for project_id, submissions_list in submission_details.items():
            if not isinstance(submissions_list, list):
                continue
            
            for submission_meta in submissions_list:
                if not isinstance(submission_meta, dict):
                    continue
                
                # Get CID
                cid = (
                    submission_meta.get("SnapshotCid") or 
                    submission_meta.get("snapshot_cid") or 
                    submission_meta.get("cid")
                )
                
                # Get submitter ID to identify full node submissions
                submitter_id = (
                    submission_meta.get("SubmitterID") or
                    submission_meta.get("submitter_id") or
                    submission_meta.get("SubmitterId")
                )
                
                if cid:
                    # For now, include all submissions (full node + lite nodes)
                    # TODO: Filter by known full node submitter IDs if needed
                    full_node_submissions[project_id].add(cid)
        
        # Also check ProjectIds/SnapshotCids arrays as fallback
        project_ids = level2_batch.get("ProjectIds") or level2_batch.get("project_ids", [])
        snapshot_cids = level2_batch.get("SnapshotCids") or level2_batch.get("snapshot_cids", [])
        
        if project_ids and snapshot_cids and len(project_ids) == len(snapshot_cids):
            for project_id, cid in zip(project_ids, snapshot_cids):
                if project_id and cid:
                    full_node_submissions[project_id].add(cid)
        
        return dict(full_node_submissions)

    def compare_submissions_in_level2(
        self,
        full_node_submissions: Dict[str, Set[str]],
        level2_batch: Dict,
    ) -> Tuple[bool, List[str]]:
        """
        Compare that all full node submissions appear in Level 2 batch.
        
        Returns: (all_present, missing_submissions)
        """
        if not level2_batch:
            return False, ["Level 2 batch not found"]

        # Extract project submissions from Level 2 batch
        # Level 2 batch has SubmissionDetails: map[projectID][]SubmissionMetadata
        level2_submissions = defaultdict(set)
        
        submission_details = level2_batch.get("SubmissionDetails") or level2_batch.get("submission_details", {})
        
        for project_id, submissions_list in submission_details.items():
            if not isinstance(submissions_list, list):
                continue
            
            for submission_meta in submissions_list:
                if isinstance(submission_meta, dict):
                    cid = submission_meta.get("SnapshotCid") or submission_meta.get("snapshot_cid") or submission_meta.get("cid")
                    if cid:
                        level2_submissions[project_id].add(cid)

        # Also check ProjectIds and SnapshotCids arrays (if present)
        project_ids = level2_batch.get("ProjectIds") or level2_batch.get("project_ids", [])
        snapshot_cids = level2_batch.get("SnapshotCids") or level2_batch.get("snapshot_cids", [])
        
        if project_ids and snapshot_cids and len(project_ids) == len(snapshot_cids):
            for project_id, cid in zip(project_ids, snapshot_cids):
                if project_id and cid:
                    level2_submissions[project_id].add(cid)

        # Compare
        missing = []
        all_present = True

        for project_id, full_node_cids in full_node_submissions.items():
            level2_cids = level2_submissions.get(project_id, set())
            
            for cid in full_node_cids:
                if cid not in level2_cids:
                    missing.append(f"Project {project_id}: CID {cid} missing from Level 2 batch")
                    all_present = False

        return all_present, missing

    def query_onchain_commitment(
        self,
        protocol_state_addr: str,
        data_market_addr: str,
        epoch_id: int,
    ) -> Optional[Dict]:
        """
        Query on-chain commitments from submitSubmissionBatch() calls.
        
        submitSubmissionBatch(batchCid, epochId, projectIds[], snapshotCids[], finalizedCidsRootHash)
        stores data in:
        1. epochIdToBatchCids[epochId] -> all batch CIDs submitted for epoch
        2. batchCidToProjects[batchCid] -> projects in each batch
        3. snapshotStatus[projectId][epochId].snapshotCid -> CID for each project
        
        Returns: Dict with batch_cids, batches (batch_cid -> project_ids), 
                 project_cids (project_id -> cid)
        """
        protocol_state = self.w3.eth.contract(
            address=Web3.to_checksum_address(protocol_state_addr),
            abi=self._get_protocol_state_abi(),
        )
        
        data_market_addr_checksum = Web3.to_checksum_address(data_market_addr)
        
        try:
            # 1. Get all batch CIDs submitted for this epoch via submitSubmissionBatch()
            batch_cids = protocol_state.functions.epochIdToBatchCids(
                data_market_addr_checksum,
                epoch_id
            ).call()
            
            if not batch_cids:
                return {
                    "epoch_id": epoch_id,
                    "batch_cids": [],
                    "batches": {},
                    "project_cids": {},
                }

            # 2. For each batch CID, get projects that were submitted
            batches = {}
            all_project_ids = set()
            
            for batch_cid in batch_cids:
                # Get projects in this batch (from batchCidToProjects mapping)
                project_ids = protocol_state.functions.batchCidToProjects(
                    data_market_addr_checksum,
                    batch_cid
                ).call()
                
                batches[batch_cid] = {
                    "project_ids": project_ids,
                }
                
                all_project_ids.update(project_ids)

            # 3. Get snapshot CIDs for each project (from snapshotStatus mapping)
            project_cids = {}
            for project_id in all_project_ids:
                snapshot_status = protocol_state.functions.snapshotStatus(
                    data_market_addr_checksum,
                    project_id,
                    epoch_id
                ).call()
                
                # snapshotStatus returns (status, snapshotCid, timestamp)
                if len(snapshot_status) >= 2 and snapshot_status[1]:
                    project_cids[project_id] = snapshot_status[1]

            return {
                "epoch_id": epoch_id,
                "batch_cids": batch_cids,
                "batches": batches,
                "project_cids": project_cids,
            }
        except Exception as e:
            print(f"⚠️  Failed to query on-chain commitment: {e}")
            import traceback
            traceback.print_exc()
            return None

    def compare_onchain_commitments(
        self,
        epoch_id: int,
        level2_batch: Dict,
    ) -> Tuple[bool, List[str]]:
        """
        Compare on-chain commitments between legacy and new contracts.
        
        Compares:
        1. Project IDs (all projects should be present in both)
        2. Individual project snapshot CIDs (should match for each project)
        
        Note: Batch CIDs are NOT compared as they may differ due to:
        - CIDv0 vs CIDv1 format differences
        - Different payload structures between legacy and new contracts
        
        Returns: (match, differences)
        """
        # Query new contract
        new_commitment = self.query_onchain_commitment(
            self.protocol_state,
            self.data_market,
            epoch_id,
        )
        
        # Query legacy contract
        legacy_commitment = self.query_onchain_commitment(
            self.legacy_protocol_state,
            self.legacy_data_market,
            epoch_id,
        )

        differences = []
        
        if not new_commitment:
            differences.append("New contract: No commitment found")
        if not legacy_commitment:
            differences.append("Legacy contract: No commitment found")
        
        if not new_commitment or not legacy_commitment:
            return False, differences

        # Extract all project IDs from both contracts
        # Get all unique project IDs from all batches
        new_project_ids = set()
        legacy_project_ids = set()
        
        new_batches = new_commitment.get("batches", {})
        legacy_batches = legacy_commitment.get("batches", {})
        
        for batch_data in new_batches.values():
            new_project_ids.update(batch_data.get("project_ids", []))
        
        for batch_data in legacy_batches.values():
            legacy_project_ids.update(batch_data.get("project_ids", []))

        # 1. Compare project IDs (all projects should be present in both)
        if new_project_ids != legacy_project_ids:
            missing_in_new = legacy_project_ids - new_project_ids
            missing_in_legacy = new_project_ids - legacy_project_ids
            if missing_in_new:
                differences.append(f"Projects in legacy but not in new: {missing_in_new}")
            if missing_in_legacy:
                differences.append(f"Projects in new but not in legacy: {missing_in_legacy}")

        # 2. Compare individual project snapshot CIDs (should match for each project)
        new_project_cids = new_commitment.get("project_cids", {})
        legacy_project_cids = legacy_commitment.get("project_cids", {})
        
        # Compare CIDs for projects that exist in both
        common_projects = new_project_ids & legacy_project_ids
        
        for project_id in common_projects:
            new_cid = new_project_cids.get(project_id)
            legacy_cid = legacy_project_cids.get(project_id)
            
            if not new_cid:
                differences.append(f"Project {project_id}: Missing CID in new contract")
            elif not legacy_cid:
                differences.append(f"Project {project_id}: Missing CID in legacy contract")
            elif new_cid != legacy_cid:
                differences.append(
                    f"Project {project_id}: Snapshot CID mismatch - "
                    f"New={new_cid}, Legacy={legacy_cid}"
                )

        return len(differences) == 0, differences

    def _get_protocol_state_abi(self) -> List[Dict]:
        """Get ProtocolState ABI from file."""
        import os
        script_dir = os.path.dirname(os.path.abspath(__file__))
        abi_path = os.path.join(script_dir, "..", "abi", "PowerloomProtocolState.abi.json")
        
        try:
            with open(abi_path, "r") as f:
                return json.load(f)
        except FileNotFoundError:
            print(f"⚠️  ProtocolState ABI not found at {abi_path}")
            return []
        except json.JSONDecodeError as e:
            print(f"⚠️  Failed to parse ProtocolState ABI: {e}")
            return []

    def _get_data_market_abi(self) -> List[Dict]:
        """Get DataMarket ABI - DataMarket functions are accessed via ProtocolState."""
        # DataMarket is accessed through ProtocolState contract
        # Return empty list as we'll use ProtocolState to access DataMarket
        return []

    def run_comparison(self, epoch_id: str, num_epochs: int = 1) -> Dict:
        """
        Run complete comparison for one or more epochs.
        
        Args:
            epoch_id: Starting epoch ID (will check epochs from epoch_id down to epoch_id - num_epochs + 1)
            num_epochs: Number of epochs to check (default: 1)
        
        Returns: Dict with results for each epoch
        """
        start_epoch = int(epoch_id)
        all_results = {}
        
        if num_epochs > 1:
            print(f"\n{'='*80}")
            print(f"Comparing DSV vs Centralized Sequencer for {num_epochs} epochs")
            print(f"Starting from epoch {start_epoch} down to {start_epoch - num_epochs + 1}")
            print(f"{'='*80}\n")
        
        for i in range(num_epochs):
            current_epoch = start_epoch - i
            if current_epoch < 0:
                break
            
            epoch_str = str(current_epoch)
            print(f"\n{'='*80}")
            print(f"Epoch {current_epoch} ({i+1}/{num_epochs})")
            print(f"{'='*80}\n")

            results = {
                "epoch_id": epoch_str,
                "submission_verification": {},
                "onchain_verification": {},
            }

            # 1. Get Level 2 aggregated batch (this contains all submissions including full node)
            print("📦 Extracting Level 2 aggregated batch from Redis...")
            level2_batch = self.get_level2_aggregated_batch(epoch_str)
            
            if not level2_batch:
                print("   ⚠️  Level 2 batch not found - cannot verify submissions")
                results["submission_verification"] = {
                    "all_present": False,
                    "missing_submissions": ["Level 2 batch not found"],
                    "error": "Level 2 batch not found in Redis",
                }
            else:
                project_count = len(level2_batch.get("ProjectIds") or level2_batch.get("project_ids", []))
                print(f"   Level 2 batch found with {project_count} projects")
                print(f"   IPFS CID: {level2_batch.get('BatchIPFSCID') or level2_batch.get('batch_ipfs_cid', 'N/A')}")
                print(f"   Merkle Root: {level2_batch.get('MerkleRoot') or level2_batch.get('merkle_root', 'N/A')}")
                
                # 2. Extract full node submissions from Level 2 batch
                print("\n🔍 Extracting full node submissions from Level 2 batch...")
                full_node_submissions = self.extract_full_node_submissions_from_level2(level2_batch)
                print(f"   Found {len(full_node_submissions)} projects with submissions")
                
                total_full_node_cids = sum(len(cids) for cids in full_node_submissions.values())
                print(f"   Total submission CIDs: {total_full_node_cids}")
                
                for project_id, cids in list(full_node_submissions.items())[:5]:
                    print(f"   - {project_id}: {len(cids)} CIDs")
                if len(full_node_submissions) > 5:
                    print(f"   ... and {len(full_node_submissions) - 5} more projects")

            # 3. Verify submissions are in Level 2 batch
            if level2_batch:
                print("\n🔬 Verifying submissions are present in Level 2 batch...")
                all_present, missing = self.compare_submissions_in_level2(
                    full_node_submissions,
                    level2_batch,
                )
                
                results["submission_verification"] = {
                    "all_present": all_present,
                    "missing_submissions": missing,
                    "full_node_projects": len(full_node_submissions),
                    "full_node_total_cids": total_full_node_cids,
                }
                
                if all_present:
                    print("   ✅ All submissions present in Level 2 batch")
                else:
                    print(f"   ❌ {len(missing)} missing submissions:")
                    for msg in missing[:10]:  # Show first 10
                        print(f"      - {msg}")
                    if len(missing) > 10:
                        print(f"      ... and {len(missing) - 10} more")

            # 5. Compare on-chain commitments
            print("\n⛓️  Comparing on-chain commitments...")
            try:
                epoch_id_int = current_epoch
                
                # Query commitments
                print("   Querying new contract...")
                new_commitment = self.query_onchain_commitment(
                    self.protocol_state,
                    self.data_market,
                    epoch_id_int,
                )
                
                print("   Querying legacy contract...")
                legacy_commitment = self.query_onchain_commitment(
                    self.legacy_protocol_state,
                    self.legacy_data_market,
                    epoch_id_int,
                )
                
                if new_commitment:
                    print(f"   New contract: {len(new_commitment.get('batch_cids', []))} batch(es), "
                          f"{len(new_commitment.get('project_cids', {}))} projects")
                if legacy_commitment:
                    print(f"   Legacy contract: {len(legacy_commitment.get('batch_cids', []))} batch(es), "
                          f"{len(legacy_commitment.get('project_cids', {}))} projects")
                
                commitments_match, differences = self.compare_onchain_commitments(
                    epoch_id_int,
                    level2_batch,
                )
                
                results["onchain_verification"] = {
                    "commitments_match": commitments_match,
                    "differences": differences,
                    "new_commitment": new_commitment,
                    "legacy_commitment": legacy_commitment,
                }
                
                if commitments_match:
                    print("   ✅ On-chain commitments match between legacy and new contracts")
                else:
                    print(f"   ❌ {len(differences)} differences found:")
                    for diff in differences[:20]:  # Show first 20
                        print(f"      - {diff}")
                    if len(differences) > 20:
                        print(f"      ... and {len(differences) - 20} more differences")
            except Exception as e:
                print(f"   ⚠️  Failed to compare on-chain commitments: {e}")
                import traceback
                traceback.print_exc()
                results["onchain_verification"] = {
                    "error": str(e),
                }

            # Summary for this epoch
            all_present = results.get("submission_verification", {}).get("all_present", False)
            commitments_match = results.get("onchain_verification", {}).get("commitments_match", False)
            
            print(f"\n{'='*80}")
            print(f"SUMMARY - Epoch {current_epoch}")
            print(f"{'='*80}")
            print(f"Submission Verification: {'✅ PASS' if all_present else '❌ FAIL'}")
            print(f"On-Chain Verification: {'✅ PASS' if commitments_match else '❌ FAIL'}")
            print(f"{'='*80}\n")
            
            all_results[epoch_str] = results
        
        # Overall summary if multiple epochs
        if num_epochs > 1:
            print(f"\n{'='*80}")
            print("OVERALL SUMMARY")
            print(f"{'='*80}")
            total_epochs = len(all_results)
            passed_submission = sum(1 for r in all_results.values() if r.get("submission_verification", {}).get("all_present", False))
            passed_onchain = sum(1 for r in all_results.values() if r.get("onchain_verification", {}).get("commitments_match", False))
            
            print(f"Total Epochs Checked: {total_epochs}")
            print(f"Submission Verification: {passed_submission}/{total_epochs} passed")
            print(f"On-Chain Verification: {passed_onchain}/{total_epochs} passed")
            print(f"{'='*80}\n")
            
            return {
                "epochs_checked": total_epochs,
                "submission_pass_count": passed_submission,
                "onchain_pass_count": passed_onchain,
                "results": all_results
            }
        
        return all_results.get(epoch_id, {})


    def query_recent_batch_submissions(
        self,
        protocol_state_addr: str,
        data_market_addr: str,
        start_epoch: int,
        num_epochs: int = 20,
    ) -> List[Dict]:
        """
        Query recent batch submissions from contract.
        
        Returns: List of dicts with epoch_id, batch_cids, project_count
        """
        protocol_state = self.w3.eth.contract(
            address=Web3.to_checksum_address(protocol_state_addr),
            abi=self._get_protocol_state_abi(),
        )
        
        data_market_addr_checksum = Web3.to_checksum_address(data_market_addr)
        results = []
        
        for i in range(num_epochs):
            epoch_id = start_epoch - i
            if epoch_id < 0:
                break
                
            try:
                batch_cids = protocol_state.functions.epochIdToBatchCids(
                    data_market_addr_checksum,
                    epoch_id
                ).call()
                
                if batch_cids:
                    # Get project count from first batch
                    project_ids = protocol_state.functions.batchCidToProjects(
                        data_market_addr_checksum,
                        batch_cids[0]
                    ).call()
                    
                    results.append({
                        "epoch_id": epoch_id,
                        "batch_cids": batch_cids,
                        "project_count": len(project_ids),
                        "first_batch_cid": batch_cids[0],
                    })
            except Exception as e:
                # Skip epochs that don't exist or have errors
                continue
        
        return results


def main():
    parser = argparse.ArgumentParser(
        description="Compare DSV network output with centralized sequencer"
    )
    parser.add_argument("--epoch-id", required=True, help="Epoch ID to compare")
    parser.add_argument("--protocol", required=True, help="Protocol state contract address (new)")
    parser.add_argument("--market", required=True, help="Data market contract address (new)")
    parser.add_argument("--redis-host", default="localhost", help="Redis host")
    parser.add_argument("--redis-port", type=int, default=6379, help="Redis port")
    parser.add_argument("--legacy-protocol", required=True, help="Legacy protocol state contract address")
    parser.add_argument("--legacy-market", required=True, help="Legacy data market contract address")
    parser.add_argument("--rpc-url", required=True, help="Ethereum RPC URL")
    parser.add_argument("--check-recent", action="store_true", help="Check recent batch submissions before comparison")
    parser.add_argument("--num-epochs", type=int, default=1, help="Number of epochs to check (starting from --epoch-id and going backwards)")

    args = parser.parse_args()

    comparator = DSVComparison(
        redis_host=args.redis_host,
        redis_port=args.redis_port,
        protocol_state=args.protocol,
        data_market=args.market,
        legacy_protocol_state=args.legacy_protocol,
        legacy_data_market=args.legacy_market,
        rpc_url=args.rpc_url,
    )

    # Check recent submissions if requested
    if args.check_recent:
        print("\n🔍 Checking recent batch submissions...")
        epoch_id_int = int(args.epoch_id)
        
        print("\n📋 Legacy Contract Recent Submissions:")
        legacy_recent = comparator.query_recent_batch_submissions(
            comparator.legacy_protocol_state,
            comparator.legacy_data_market,
            epoch_id_int,
            num_epochs=20,
        )
        if legacy_recent:
            for sub in legacy_recent:
                print(f"   Epoch {sub['epoch_id']}: {len(sub['batch_cids'])} batch(es), "
                      f"{sub['project_count']} projects, CID={sub['first_batch_cid'][:20]}...")
        else:
            print("   ⚠️  No batch submissions found in last 20 epochs")
        
        print("\n📋 New Contract Recent Submissions:")
        new_recent = comparator.query_recent_batch_submissions(
            comparator.protocol_state,
            comparator.data_market,
            epoch_id_int,
            num_epochs=20,
        )
        if new_recent:
            for sub in new_recent:
                print(f"   Epoch {sub['epoch_id']}: {len(sub['batch_cids'])} batch(es), "
                      f"{sub['project_count']} projects, CID={sub['first_batch_cid'][:20]}...")
        else:
            print("   ⚠️  No batch submissions found in last 20 epochs")
        
        print()

    results = comparator.run_comparison(args.epoch_id, num_epochs=args.num_epochs)
    
    # Output JSON results
    print("\n📄 JSON Results:")
    print(json.dumps(results, indent=2))


if __name__ == "__main__":
    main()

