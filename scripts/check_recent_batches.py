#!/usr/bin/env python3
"""
Quick script to check recent batch submissions on contracts.
"""

import sys
from web3 import Web3
from eth_utils import to_checksum_address
import json

def load_abi():
    import os
    script_dir = os.path.dirname(os.path.abspath(__file__))
    abi_path = os.path.join(script_dir, "..", "abi", "PowerloomProtocolState.abi.json")
    with open(abi_path, "r") as f:
        return json.load(f)

def check_recent_batches(protocol_state_addr, data_market_addr, rpc_url, start_epoch, num_epochs=20):
    w3 = Web3(Web3.HTTPProvider(rpc_url))
    protocol_state = w3.eth.contract(
        address=Web3.to_checksum_address(protocol_state_addr),
        abi=load_abi(),
    )
    
    data_market_addr_checksum = Web3.to_checksum_address(data_market_addr)
    results = []
    
    print(f"Checking epochs {start_epoch} down to {start_epoch - num_epochs + 1}...")
    
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
            continue
    
    return results

if __name__ == "__main__":
    if len(sys.argv) < 6:
        print("Usage: python check_recent_batches.py <protocol_state> <data_market> <rpc_url> <start_epoch> [num_epochs]")
        print("\nExample:")
        print("  python check_recent_batches.py \\")
        print("    0x3B5A0FB70ef68B5dd677C7d614dFB89961f97401 \\")
        print("    0xb5cE2F9B71e785e3eC0C45EDE06Ad95c3bb71a4d \\")
        print("    https://rpc-devnet.powerloom.dev \\")
        print("    23976321 20")
        sys.exit(1)
    
    protocol_state = sys.argv[1]
    data_market = sys.argv[2]
    rpc_url = sys.argv[3]
    start_epoch = int(sys.argv[4])
    num_epochs = int(sys.argv[5]) if len(sys.argv) > 5 else 20
    
    results = check_recent_batches(protocol_state, data_market, rpc_url, start_epoch, num_epochs)
    
    if results:
        print(f"\n✅ Found {len(results)} epochs with batch submissions:\n")
        for sub in results:
            print(f"  Epoch {sub['epoch_id']}:")
            print(f"    - Batch CIDs: {len(sub['batch_cids'])}")
            print(f"    - Projects: {sub['project_count']}")
            print(f"    - First Batch CID: {sub['first_batch_cid']}")
            print()
    else:
        print(f"\n⚠️  No batch submissions found in last {num_epochs} epochs")

