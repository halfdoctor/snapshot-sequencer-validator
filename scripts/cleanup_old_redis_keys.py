#!/usr/bin/env python3
"""
Redis Key Cleanup Script for DSV Node

Cleans up old Redis keys from epochs older than (current_epoch - keep_epochs).
Safely removes keys that are no longer needed.

Usage:
    python3 cleanup_old_redis_keys.py [--dry-run] [--keep-epochs 60] [--protocol PROTOCOL] [--market MARKET] [--port 6380]

Examples:
    # Dry run (see what would be deleted)
    python3 cleanup_old_redis_keys.py --dry-run

    # Actually delete old keys (keep last 60 epochs)
    python3 cleanup_old_redis_keys.py --keep-epochs 60

    # Keep more epochs (e.g., 100)
    python3 cleanup_old_redis_keys.py --keep-epochs 100
"""

import argparse
import redis
import json
import re
import time
from typing import List, Set, Tuple
from collections import defaultdict


class RedisCleanup:
    def __init__(self, host='localhost', port=6380, db=0, protocol=None, market=None):
        self.redis_client = redis.Redis(host=host, port=port, db=db, decode_responses=True)
        self.protocol = protocol
        self.market = market
        self.stats = defaultdict(int)

    def get_current_epoch(self) -> int:
        """Get current epoch from Redis using consistent method for both dry-run and live mode."""
        if not self.protocol or not self.market:
            raise ValueError("Protocol and market must be specified")
        
        # Try methods in order of reliability, collecting all results for validation
        candidates = []
        
        # Method 1: Try to get from metrics:current_epoch key (most reliable)
        current_epoch_key = f"{self.protocol}:{self.market}:metrics:current_epoch"
        try:
            data = self.redis_client.get(current_epoch_key)
            if data:
                epoch_info = json.loads(data)
                epoch_id = epoch_info.get('epoch_id', '')
                if epoch_id:
                    epoch_num = self._extract_epoch_number(epoch_id)
                    if epoch_num is not None:
                        candidates.append(('metrics', epoch_num))
        except Exception as e:
            print(f"⚠ Could not get current epoch from metrics: {e}")

        # Method 2: Try to get from ActiveEpochs SET (fallback)
        active_epochs_key = f"{self.protocol}:{self.market}:epochs:active"
        try:
            epochs = self.redis_client.smembers(active_epochs_key)
            if epochs:
                epoch_nums = [self._extract_epoch_number(e) for e in epochs]
                epoch_nums = [e for e in epoch_nums if e is not None]
                if epoch_nums:
                    current = max(epoch_nums)
                    candidates.append(('ActiveEpochs', current))
        except Exception as e:
            print(f"⚠ Could not get current epoch from ActiveEpochs: {e}")

        # Method 3: Try to get from timeline (most recent open epoch)
        timeline_key = f"{self.protocol}:{self.market}:metrics:epochs:timeline"
        try:
            entries = self.redis_client.zrevrange(timeline_key, 0, 9, withscores=True)
            for entry, score in entries:
                if entry.startswith('open:'):
                    epoch_id = entry.split(':', 1)[1]
                    epoch_num = self._extract_epoch_number(epoch_id)
                    if epoch_num is not None:
                        candidates.append(('timeline', epoch_num))
                        break
        except Exception as e:
            print(f"⚠ Could not get current epoch from timeline: {e}")

        # Validate consistency: if we have multiple candidates, they should be close
        if candidates:
            # Prefer metrics, then ActiveEpochs, then timeline
            source_order = {'metrics': 0, 'ActiveEpochs': 1, 'timeline': 2}
            candidates.sort(key=lambda x: (source_order.get(x[0], 99), -x[1]))
            
            selected = candidates[0]
            selected_epoch = selected[1]
            
            # Warn if there's significant discrepancy (> 100 epochs)
            for source, epoch in candidates[1:]:
                if abs(epoch - selected_epoch) > 100:
                    print(f"⚠ Warning: Epoch mismatch detected - {selected[0]}: {selected_epoch}, {source}: {epoch}")
            
            print(f"✓ Found current epoch from {selected[0]}: {selected_epoch}")
            return selected_epoch

        # Fallback: scan for highest epoch number in epoch state keys
        print("⚠ Could not determine current epoch from standard keys, scanning...")
        pattern = f"{self.protocol}:{self.market}:epoch:*:state"
        try:
            max_epoch = 0
            for key in self.redis_client.scan_iter(match=pattern, count=100):
                parts = key.split(':')
                if len(parts) >= 4:
                    epoch_id = parts[3]
                    epoch_num = self._extract_epoch_number(epoch_id)
                    if epoch_num is not None and epoch_num > max_epoch:
                        max_epoch = epoch_num
            if max_epoch > 0:
                print(f"✓ Found current epoch from scanning: {max_epoch}")
                return max_epoch
        except Exception as e:
            print(f"⚠ Could not scan for current epoch: {e}")

        raise ValueError("Could not determine current epoch. Please specify --protocol and --market")

    def _extract_epoch_number(self, epoch_id: str) -> int:
        """Extract numeric epoch ID from string."""
        # Try to extract number from epoch ID
        # Epoch IDs can be formatted as "123" or "epoch_123" or just numbers
        match = re.search(r'(\d+)', str(epoch_id))
        if match:
            return int(match.group(1))
        return None

    def find_keys_to_delete(self, current_epoch: int, keep_epochs: int) -> List[str]:
        """Find all keys that should be deleted."""
        cutoff_epoch = current_epoch - keep_epochs
        keys_to_delete = []

        print(f"\n🔍 Scanning for keys older than epoch {cutoff_epoch} (current: {current_epoch}, keeping: {keep_epochs})...")

        # Key patterns to clean up
        patterns = []

        if self.protocol and self.market:
            # Epoch-specific keys
            patterns.extend([
                f"{self.protocol}:{self.market}:epoch:*:state",
                f"{self.protocol}:{self.market}:epoch:*:window",
                f"{self.protocol}:{self.market}:epoch:*:submissions:ids",
                f"{self.protocol}:{self.market}:epoch:*:submissions:data",
                f"{self.protocol}:{self.market}:epoch:*:processed",
                f"{self.protocol}:{self.market}:epoch:*:info",
                f"{self.protocol}:{self.market}:finalized:*",
                f"{self.protocol}:{self.market}:batch:aggregated:*",
                f"{self.protocol}:{self.market}:batch:part:*",
                f"{self.protocol}:{self.market}:incoming:batch:*",
                f"{self.protocol}:{self.market}:metrics:batch:local:*",
                f"{self.protocol}:{self.market}:metrics:batch:aggregated:*",
                f"{self.protocol}:{self.market}:metrics:batch:*:validators",
                f"{self.protocol}:{self.market}:metrics:epoch:*:info",
            ])

        # Relayer-py keys (no namespace)
        patterns.extend([
            "epoch_batch_size:*",
            "epoch_batch_submissions:*",
            "end_batch_submission_called:*",
        ])

        # Event collector keys
        patterns.extend([
            "EpochMarkerSet.*",
            "EpochMarkerDetails.*",
            "DayRolloverEpochMarkerSet.*",
            "DayRolloverEpochMarkerDetails.*",
        ])

        # Scan for keys matching patterns
        for pattern in patterns:
            try:
                for key in self.redis_client.scan_iter(match=pattern, count=100):
                    epoch_num = self._extract_epoch_from_key(key)
                    if epoch_num is not None and epoch_num < cutoff_epoch:
                        keys_to_delete.append(key)
                        self.stats[pattern] += 1
            except Exception as e:
                print(f"⚠ Error scanning pattern {pattern}: {e}")

        # Clean up timeline keys older than cutoff (by score/timestamp)
        # Timeline zsets use Unix timestamps as scores, not epoch numbers
        cutoff_timestamp = int(time.time()) - (keep_epochs * 60)  # Keep last N epochs (assuming ~1 epoch per minute)
        
        if self.protocol and self.market:
            timeline_keys = [
                f"{self.protocol}:{self.market}:metrics:epochs:timeline",
                f"{self.protocol}:{self.market}:metrics:batches:timeline",
                f"{self.protocol}:{self.market}:metrics:submissions:timeline",
                f"{self.protocol}:{self.market}:metrics:validations:timeline",
            ]
            
            # Also check non-namespaced timeline keys
            timeline_keys.extend([
                "metrics:epochs:timeline",
                "metrics:batches:timeline",
                "metrics:submissions:timeline",
                "metrics:validations:timeline",
            ])

            for timeline_key in timeline_keys:
                try:
                    # Check if key exists first
                    if not self.redis_client.exists(timeline_key):
                        continue
                    
                    # Remove entries older than cutoff timestamp
                    # Timeline zsets use Unix timestamps as scores
                    removed = self.redis_client.zremrangebyscore(
                        timeline_key, "-inf", cutoff_timestamp
                    )
                    if removed > 0:
                        self.stats[f"{timeline_key} (timeline)"] = removed
                        print(f"  ✓ Removed {removed} entries from {timeline_key} (cutoff: {cutoff_timestamp})")
                except Exception as e:
                    if "no such key" not in str(e).lower():
                        print(f"⚠ Error cleaning timeline {timeline_key}: {e}")
        
        # Prune epochs:active SET to remove old epochs
        if self.protocol and self.market:
            active_epochs_key = f"{self.protocol}:{self.market}:epochs:active"
            try:
                if self.redis_client.exists(active_epochs_key):
                    epochs = self.redis_client.smembers(active_epochs_key)
                    epochs_to_remove = []
                    for epoch_str in epochs:
                        epoch_num = self._extract_epoch_number(epoch_str)
                        if epoch_num is not None and epoch_num < cutoff_epoch:
                            epochs_to_remove.append(epoch_str)
                    
                    if epochs_to_remove:
                        removed = self.redis_client.srem(active_epochs_key, *epochs_to_remove)
                        if removed > 0:
                            self.stats[f"{active_epochs_key} (set)"] = removed
                            print(f"  ✓ Removed {removed} old epochs from {active_epochs_key}")
            except Exception as e:
                print(f"⚠ Error pruning {active_epochs_key}: {e}")
        
        # Clean up legacy aggregation:queue LIST if it exceeds threshold
        if self.protocol and self.market:
            aggregation_queue_key = f"{self.protocol}:{self.market}:aggregation:queue"
            try:
                queue_length = self.redis_client.llen(aggregation_queue_key)
                if queue_length > 10000:  # Threshold: 10K items
                    print(f"  ⚠ Legacy aggregation:queue has {queue_length} items (threshold: 10000)")
                    print(f"  💡 Consider running cleanup_stale_queue.sh to remove unused legacy queue")
                    self.stats[f"{aggregation_queue_key} (legacy)"] = queue_length
            except Exception as e:
                if "no such key" not in str(e).lower():
                    print(f"⚠ Error checking {aggregation_queue_key}: {e}")

        return keys_to_delete

    def _extract_epoch_from_key(self, key: str) -> int:
        """Extract epoch number from a Redis key."""
        # Try different patterns
        patterns = [
            r':epoch:(\d+):',  # {protocol}:{market}:epoch:{epochId}:...
            r':epoch:([^:]+):',  # Handle non-numeric epoch IDs
            r'epoch_batch_size:(\d+)',  # epoch_batch_size:{epochId}
            r'epoch_batch_submissions:(\d+)',  # epoch_batch_submissions:{epochId}
            r'end_batch_submission_called:[^:]+:(\d+)',  # end_batch_submission_called:{market}:{epochId}
            r':finalized:(\d+)',  # {protocol}:{market}:finalized:{epochId}
            r':batch:aggregated:(\d+)',  # {protocol}:{market}:batch:aggregated:{epochId}
            r':batch:part:(\d+):',  # {protocol}:{market}:batch:part:{epochId}:...
            r':incoming:batch:(\d+):',  # {protocol}:{market}:incoming:batch:{epochId}:...
            r':batch:local:(\d+)',  # {protocol}:{market}:metrics:batch:local:{epochId}
            r':batch:aggregated:(\d+)',  # {protocol}:{market}:metrics:batch:aggregated:{epochId}
            r':epoch:(\d+):info',  # {protocol}:{market}:metrics:epoch:{epochId}:info
            r'\.(\d+)\.',  # EpochMarkerDetails.{market}.{epochId}
        ]

        for pattern in patterns:
            match = re.search(pattern, key)
            if match:
                epoch_str = match.group(1)
                try:
                    return int(epoch_str)
                except ValueError:
                    # Try to extract number from epoch string
                    num_match = re.search(r'(\d+)', epoch_str)
                    if num_match:
                        return int(num_match.group(1))
        return None

    def delete_keys(self, keys: List[str], dry_run: bool = True) -> Tuple[int, int]:
        """Delete keys from Redis."""
        if not keys:
            print("\n✓ No keys to delete.")
            return 0, 0

        deleted = 0
        failed = 0

        if dry_run:
            print(f"\n🔍 DRY RUN: Would delete {len(keys)} keys:")
            # Group by pattern for better output
            by_pattern = defaultdict(list)
            for key in keys:
                pattern = self._get_pattern_for_key(key)
                by_pattern[pattern].append(key)

            for pattern, pattern_keys in sorted(by_pattern.items()):
                print(f"\n  {pattern}: {len(pattern_keys)} keys")
                # Show first 5 examples
                for key in pattern_keys[:5]:
                    epoch = self._extract_epoch_from_key(key)
                    print(f"    - {key} (epoch: {epoch})")
                if len(pattern_keys) > 5:
                    print(f"    ... and {len(pattern_keys) - 5} more")
        else:
            print(f"\n🗑️  Deleting {len(keys)} keys...")
            # Delete in batches
            batch_size = 100
            for i in range(0, len(keys), batch_size):
                batch = keys[i:i + batch_size]
                try:
                    deleted_count = self.redis_client.delete(*batch)
                    deleted += deleted_count
                    if deleted_count < len(batch):
                        failed += (len(batch) - deleted_count)
                    print(f"  Deleted batch {i//batch_size + 1}: {deleted_count}/{len(batch)} keys")
                except Exception as e:
                    print(f"  ⚠ Error deleting batch: {e}")
                    failed += len(batch)

        return deleted, failed

    def _get_pattern_for_key(self, key: str) -> str:
        """Get pattern category for a key."""
        if 'epoch_batch_size' in key:
            return 'epoch_batch_size'
        elif 'epoch_batch_submissions' in key:
            return 'epoch_batch_submissions'
        elif 'end_batch_submission_called' in key:
            return 'end_batch_submission_called'
        elif ':epoch:' in key and ':state' in key:
            return 'epoch_state'
        elif ':epoch:' in key and ':submissions:' in key:
            return 'epoch_submissions'
        elif ':finalized:' in key:
            return 'finalized_batch'
        elif ':batch:aggregated:' in key:
            return 'aggregated_batch'
        elif 'EpochMarker' in key:
            return 'epoch_marker'
        else:
            return 'other'

    def print_stats(self):
        """Print cleanup statistics."""
        print("\n" + "="*60)
        print("📊 Cleanup Statistics")
        print("="*60)
        total = sum(self.stats.values())
        for pattern, count in sorted(self.stats.items(), key=lambda x: x[1], reverse=True):
            print(f"  {pattern}: {count} keys")
        print(f"\n  Total: {total} keys")
        print("="*60)


def main():
    parser = argparse.ArgumentParser(
        description='Clean up old Redis keys from DSV node',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )
    parser.add_argument('--dry-run', action='store_true',
                        help='Show what would be deleted without actually deleting')
    parser.add_argument('--keep-epochs', type=int, default=60,
                        help='Number of recent epochs to keep (default: 60)')
    parser.add_argument('--protocol', type=str,
                        help='Protocol state address (e.g., 0x1234...)')
    parser.add_argument('--market', type=str,
                        help='Data market address (e.g., 0x5678...)')
    parser.add_argument('--host', type=str, default='localhost',
                        help='Redis host (default: localhost)')
    parser.add_argument('--port', type=int, default=6380,
                        help='Redis port (default: 6380)')
    parser.add_argument('--db', type=int, default=0,
                        help='Redis database (default: 0)')

    args = parser.parse_args()

    print("="*60)
    print("🔧 DSV Redis Key Cleanup Script")
    print("="*60)
    print(f"Host: {args.host}:{args.port}")
    print(f"DB: {args.db}")
    print(f"Keep epochs: {args.keep_epochs}")
    print(f"Protocol: {args.protocol or 'auto-detect'}")
    print(f"Market: {args.market or 'auto-detect'}")
    if args.dry_run:
        print("Mode: DRY RUN (no keys will be deleted)")
    else:
        print("Mode: LIVE (keys will be deleted)")
    print("="*60)

    try:
        cleanup = RedisCleanup(
            host=args.host,
            port=args.port,
            db=args.db,
            protocol=args.protocol,
            market=args.market
        )

        # Get current epoch
        try:
            current_epoch = cleanup.get_current_epoch()
            print(f"\n✓ Current epoch: {current_epoch}")
        except ValueError as e:
            print(f"\n❌ Error: {e}")
            print("\nPlease specify --protocol and --market if auto-detection fails.")
            return 1

        # Find keys to delete
        keys_to_delete = cleanup.find_keys_to_delete(current_epoch, args.keep_epochs)

        # Delete keys
        deleted, failed = cleanup.delete_keys(keys_to_delete, dry_run=args.dry_run)

        # Print stats
        cleanup.print_stats()

        if args.dry_run:
            print("\n💡 This was a dry run. Use without --dry-run to actually delete keys.")
        else:
            print(f"\n✓ Cleanup complete: {deleted} deleted, {failed} failed")

        return 0

    except redis.ConnectionError as e:
        print(f"\n❌ Could not connect to Redis: {e}")
        return 1
    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()
        return 1


if __name__ == '__main__':
    exit(main())

