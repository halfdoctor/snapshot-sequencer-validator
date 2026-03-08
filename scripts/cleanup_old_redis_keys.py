#!/usr/bin/env python3
"""
Redis Key Cleanup Script for DSV Node

PURPOSE:
Cleans up old Redis keys to prevent memory bloat. The script operates in two modes:
1. Epoch-based cleanup: Requires protocol/market to determine current epoch, then removes keys older than (current_epoch - keep_epochs)
2. Time-based cleanup: Uses --keep-hours to remove keys older than N hours (works without protocol/market for timelines/queues/streams)

WHY PROTOCOL/MARKET IS NEEDED:
- Epoch-based keys are namespaced: {protocol}:{market}:epoch:{epochId}:...
- To know what's "old", we need to know the current epoch
- Current epoch is stored in Redis keys that require protocol/market to access
- Queue/stream/timeline cleanup can work without protocol/market (they're time-based, not epoch-based)

Usage:
    python3 cleanup_old_redis_keys.py [--dry-run] [--keep-epochs 60] [--keep-hours HOURS] 
                                      [--aggressive-timeline] [--protocol PROTOCOL] 
                                      [--market MARKET] [--port 6380]
                                      [--discover] [--cleanup-streams] [--cleanup-queues]
                                      [--all-markets]

Examples:
    # Discovery mode: scan all keys and see what exists (no cleanup, no protocol/market needed)
    python3 cleanup_old_redis_keys.py --discover

    # Clean queues/streams (no protocol/market needed - time-based cleanup)
    python3 cleanup_old_redis_keys.py --cleanup-queues --queue-max-length 1000 --dry-run
    python3 cleanup_old_redis_keys.py --cleanup-queues --queue-max-length 1000

    # Clean non-namespaced timelines (no protocol/market needed - time-based cleanup)
    python3 cleanup_old_redis_keys.py --keep-hours 24 --dry-run
    python3 cleanup_old_redis_keys.py --keep-hours 24

    # Clean ALL markets automatically (discovers all protocol:market combinations, no protocol/market needed)
    python3 cleanup_old_redis_keys.py --all-markets --keep-hours 24 --cleanup-queues --dry-run
    python3 cleanup_old_redis_keys.py --all-markets --keep-hours 24 --cleanup-queues

    # Clean specific protocol/market (epoch-based cleanup - requires protocol/market)
    python3 cleanup_old_redis_keys.py --keep-epochs 60 --protocol 0x1234... --market 0x5678... --dry-run
    python3 cleanup_old_redis_keys.py --keep-epochs 60 --protocol 0x1234... --market 0x5678...
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
        self.discovered_keys = defaultdict(list)  # For discovery mode

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

    def find_keys_to_delete(self, current_epoch: int, keep_epochs: int, dry_run: bool = True, 
                           keep_hours: int = None, aggressive_timeline: bool = False) -> List[str]:
        """Find all keys that should be deleted.
        
        Args:
            current_epoch: Current epoch number
            keep_epochs: Number of epochs to keep (for epoch-based keys)
            dry_run: If True, don't actually delete anything
            keep_hours: If set, keep only last N hours (overrides keep_epochs for timeline)
            aggressive_timeline: If True, clean timeline entries based on epoch IDs in entries
        """
        cutoff_epoch = current_epoch - keep_epochs
        keys_to_delete = []

        print(f"\n🔍 Scanning for keys older than epoch {cutoff_epoch} (current: {current_epoch}, keeping: {keep_epochs})...")
        if keep_hours:
            print(f"   Timeline cleanup: keeping last {keep_hours} hours")
        if aggressive_timeline:
            print(f"   Aggressive timeline cleanup: enabled")

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
        if keep_hours:
            cutoff_timestamp = int(time.time()) - (keep_hours * 3600)  # Keep last N hours
        else:
            cutoff_timestamp = int(time.time()) - (keep_epochs * 60)  # Keep last N epochs (assuming ~1 epoch per minute)
        
        # Always include non-namespaced timeline keys
        timeline_keys = [
            "metrics:epochs:timeline",
            "metrics:batches:timeline",
            "metrics:submissions:timeline",
            "metrics:validations:timeline",
        ]
        
        if self.protocol and self.market:
            # Add namespaced timeline keys
            timeline_keys.extend([
                f"{self.protocol}:{self.market}:metrics:epochs:timeline",
                f"{self.protocol}:{self.market}:metrics:batches:timeline",
                f"{self.protocol}:{self.market}:metrics:submissions:timeline",
                f"{self.protocol}:{self.market}:metrics:validations:timeline",
            ])
        
        # Process all timeline keys (both namespaced and non-namespaced)
        for timeline_key in timeline_keys:
            try:
                # Check if key exists first
                if not self.redis_client.exists(timeline_key):
                    continue
                
                total_size = self.redis_client.zcard(timeline_key)
                if total_size == 0:
                    continue
                
                # Count entries that would be removed
                count_to_remove = self.redis_client.zcount(timeline_key, "-inf", cutoff_timestamp)
                
                if count_to_remove > 0:
                    if dry_run:
                        self.stats[f"{timeline_key} (timeline)"] = count_to_remove
                        print(f"  🔍 DRY RUN: Would remove {count_to_remove:,} entries from {timeline_key} (total: {total_size:,}, cutoff: {cutoff_timestamp})")
                    else:
                        # For very large deletions, use batch processing
                        if count_to_remove > 100000:
                            print(f"  ⚠ Large deletion detected ({count_to_remove:,} entries), using batch processing...")
                            removed = self._cleanup_timeline_batched(timeline_key, cutoff_timestamp, cutoff_epoch if aggressive_timeline else None)
                        else:
                            # Remove entries older than cutoff timestamp
                            removed = self.redis_client.zremrangebyscore(
                                timeline_key, "-inf", cutoff_timestamp
                            )
                        
                        if removed > 0:
                            self.stats[f"{timeline_key} (timeline)"] = removed
                            print(f"  ✓ Removed {removed:,} entries from {timeline_key}")
                        
                        # Aggressive epoch-based cleanup is redundant after timestamp cleanup
                        # Timestamp-based cleanup already removed old entries efficiently
                        # Only use epoch-based cleanup if timestamp cleanup didn't work (very rare edge case)
                        # if aggressive_timeline and removed == 0 and count_to_remove > 0:
                        #     # Only if timestamp cleanup failed but entries exist
                        #     self._cleanup_timeline_by_epoch(timeline_key, cutoff_epoch, dry_run)
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
                        if dry_run:
                            self.stats[f"{active_epochs_key} (set)"] = len(epochs_to_remove)
                            print(f"  🔍 DRY RUN: Would remove {len(epochs_to_remove)} old epochs from {active_epochs_key}")
                        else:
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
        
        # Clean up submission metadata keys (these have TTL but might accumulate)
        if self.protocol and self.market:
            metadata_pattern = f"{self.protocol}:{self.market}:metrics:submissions:metadata:*"
            try:
                metadata_keys_scanned = 0
                metadata_keys_to_delete = []
                for key in self.redis_client.scan_iter(match=metadata_pattern, count=100):
                    metadata_keys_scanned += 1
                    # Extract epoch from metadata key or check TTL
                    # Metadata keys have 24h TTL, but if they're old, delete them
                    ttl = self.redis_client.ttl(key)
                    if ttl == -1:  # No TTL set (shouldn't happen but check anyway)
                        # Try to extract epoch from key and check if old
                        epoch_num = self._extract_epoch_from_key(key)
                        if epoch_num is not None and epoch_num < cutoff_epoch:
                            metadata_keys_to_delete.append(key)
                    elif ttl == -2:  # Key doesn't exist (shouldn't happen in scan)
                        continue
                    
                    # Limit scan to avoid blocking
                    if metadata_keys_scanned >= 100000:
                        break
                
                if metadata_keys_to_delete:
                    if dry_run:
                        self.stats[f"{metadata_pattern} (metadata)"] = len(metadata_keys_to_delete)
                        print(f"  🔍 DRY RUN: Would remove {len(metadata_keys_to_delete):,} submission metadata keys")
                    else:
                        # Delete in batches
                        batch_size = 1000
                        total_deleted = 0
                        for i in range(0, len(metadata_keys_to_delete), batch_size):
                            batch = metadata_keys_to_delete[i:i + batch_size]
                            deleted = self.redis_client.delete(*batch)
                            total_deleted += deleted
                        self.stats[f"{metadata_pattern} (metadata)"] = total_deleted
                        print(f"  ✓ Removed {total_deleted:,} submission metadata keys")
            except Exception as e:
                print(f"⚠ Error cleaning submission metadata keys: {e}")

        return keys_to_delete

    def _cleanup_timeline_batched(self, timeline_key: str, cutoff_timestamp: int, cutoff_epoch: int = None) -> int:
        """Clean up timeline entries in batches to avoid blocking Redis."""
        total_removed = 0
        batch_size = 10000  # Process 10K entries at a time
        
        # Get range of entries to remove
        entries = self.redis_client.zrangebyscore(timeline_key, "-inf", cutoff_timestamp, withscores=True, start=0, num=batch_size)
        
        while entries:
            # Extract member names (not scores) for deletion
            members_to_remove = [member for member, score in entries]
            
            # Also filter by epoch if cutoff_epoch is provided
            if cutoff_epoch:
                filtered_members = []
                for member in members_to_remove:
                    epoch_num = self._extract_epoch_from_timeline_entry(member)
                    if epoch_num is None or epoch_num < cutoff_epoch:
                        filtered_members.append(member)
                members_to_remove = filtered_members
            
            if members_to_remove:
                removed = self.redis_client.zrem(timeline_key, *members_to_remove)
                total_removed += removed
                print(f"    Batch: removed {removed:,} entries (total so far: {total_removed:,})")
            
            # Get next batch
            entries = self.redis_client.zrangebyscore(timeline_key, "-inf", cutoff_timestamp, withscores=True, start=0, num=batch_size)
        
        return total_removed
    
    def _cleanup_timeline_by_epoch(self, timeline_key: str, cutoff_epoch: int, dry_run: bool):
        """Clean up timeline entries by extracting epoch IDs from entry values."""
        try:
            # Sample entries to check if they contain epoch IDs
            sample = self.redis_client.zrange(timeline_key, 0, 100, withscores=False)
            if not sample:
                return
            
            # Check if entries contain epoch IDs (format: received:{epochId}:... or {epochId}-...)
            entries_with_epochs = []
            for entry in sample:
                epoch_num = self._extract_epoch_from_timeline_entry(entry)
                if epoch_num is not None:
                    entries_with_epochs.append((entry, epoch_num))
            
            if not entries_with_epochs:
                return  # Entries don't contain epoch IDs, skip
            
            # Scan all entries and collect those with old epochs
            members_to_remove = []
            cursor = 0
            batch_size = 1000
            
            while True:
                entries = self.redis_client.zscan(timeline_key, cursor, count=batch_size)
                cursor = entries[0]
                members = entries[1]
                
                for member, score in members:
                    epoch_num = self._extract_epoch_from_timeline_entry(member)
                    if epoch_num is not None and epoch_num < cutoff_epoch:
                        members_to_remove.append(member)
                
                if cursor == 0:
                    break
                
                # Process in batches to avoid memory issues
                if len(members_to_remove) >= 10000:
                    if dry_run:
                        self.stats[f"{timeline_key} (timeline-epoch)"] = len(members_to_remove)
                        print(f"  🔍 DRY RUN: Would remove {len(members_to_remove):,} entries by epoch from {timeline_key}")
                    else:
                        removed = self.redis_client.zrem(timeline_key, *members_to_remove[:10000])
                        print(f"  ✓ Removed {removed:,} entries by epoch from {timeline_key}")
                    members_to_remove = members_to_remove[10000:]
            
            # Remove remaining entries
            if members_to_remove:
                if dry_run:
                    self.stats[f"{timeline_key} (timeline-epoch)"] = len(members_to_remove)
                    print(f"  🔍 DRY RUN: Would remove {len(members_to_remove):,} entries by epoch from {timeline_key}")
                else:
                    removed = self.redis_client.zrem(timeline_key, *members_to_remove)
                    print(f"  ✓ Removed {removed:,} entries by epoch from {timeline_key}")
        except Exception as e:
            print(f"⚠ Error in epoch-based timeline cleanup for {timeline_key}: {e}")
    
    def _extract_epoch_from_timeline_entry(self, entry: str) -> int:
        """Extract epoch number from timeline entry value.
        
        Supports formats:
        - received:{epochId}:{slotId}:{projectId}:{timestamp}:{peerId}
        - {epochId}-{projectId}-{timestamp}
        - open:{epochId}
        - closed:{epochId}
        """
        # Try format: received:{epochId}:...
        match = re.search(r'received:(\d+):', entry)
        if match:
            return int(match.group(1))
        
        # Try format: {epochId}-{projectId}-...
        match = re.search(r'^(\d+)-', entry)
        if match:
            return int(match.group(1))
        
        # Try format: open:{epochId} or closed:{epochId}
        match = re.search(r'(?:open|closed):(\d+)', entry)
        if match:
            return int(match.group(1))
        
        # Try to extract any number that looks like an epoch ID
        match = re.search(r':(\d+):', entry)
        if match:
            epoch_candidate = int(match.group(1))
            # Sanity check: epoch IDs are typically large numbers (> 1000000)
            if epoch_candidate > 1000000:
                return epoch_candidate
        
        return None

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

    def discover_all_keys(self, pattern="*", max_keys=100000):
        """Discover and categorize all keys in Redis."""
        print(f"\n🔍 Discovering all keys (pattern: {pattern}, max: {max_keys:,})...")
        
        key_types = defaultdict(int)
        key_patterns = defaultdict(list)
        streams = []
        queues = []
        unknown_keys = []
        
        cursor = 0
        scanned = 0
        
        while scanned < max_keys:
            cursor, keys = self.redis_client.scan(cursor, match=pattern, count=1000)
            scanned += len(keys)
            
            for key in keys:
                try:
                    key_type = self.redis_client.type(key)
                    key_types[key_type] += 1
                    
                    # Categorize key
                    if key_type == 'stream':
                        streams.append(key)
                        length = self.redis_client.xlen(key)
                        key_patterns['streams'].append((key, length))
                    elif key_type == 'list':
                        queues.append(key)
                        length = self.redis_client.llen(key)
                        key_patterns['queues'].append((key, length))
                    elif key_type == 'zset':
                        length = self.redis_client.zcard(key)
                        if 'timeline' in key.lower():
                            key_patterns['timelines'].append((key, length))
                        else:
                            key_patterns['zsets'].append((key, length))
                    elif key_type == 'set':
                        length = self.redis_client.scard(key)
                        key_patterns['sets'].append((key, length))
                    elif key_type == 'hash':
                        length = self.redis_client.hlen(key)
                        key_patterns['hashes'].append((key, length))
                    elif key_type == 'string':
                        key_patterns['strings'].append((key, 1))
                    else:
                        unknown_keys.append((key, key_type))
                    
                    # Extract protocol:market patterns
                    if ':' in key and len(key.split(':')) >= 2:
                        parts = key.split(':')
                        if len(parts) >= 2:
                            proto_market = f"{parts[0]}:{parts[1]}"
                            self.discovered_keys[proto_market].append(key)
                
                except Exception as e:
                    print(f"  ⚠ Error inspecting key {key}: {e}")
            
            if cursor == 0:
                break
        
        print(f"\n📊 Discovery Results:")
        print(f"  Total keys scanned: {scanned:,}")
        print(f"\n  Key types:")
        for ktype, count in sorted(key_types.items(), key=lambda x: x[1], reverse=True):
            print(f"    {ktype}: {count:,}")
        
        print(f"\n  Large streams (>1000 entries):")
        for key, length in sorted(key_patterns.get('streams', []), key=lambda x: x[1], reverse=True)[:10]:
            if length > 1000:
                print(f"    {key}: {length:,} entries")
        
        print(f"\n  Large queues (>1000 items):")
        for key, length in sorted(key_patterns.get('queues', []), key=lambda x: x[1], reverse=True)[:10]:
            if length > 1000:
                print(f"    {key}: {length:,} items")
        
        print(f"\n  Large timelines (>10000 entries):")
        for key, length in sorted(key_patterns.get('timelines', []), key=lambda x: x[1], reverse=True)[:10]:
            if length > 10000:
                print(f"    {key}: {length:,} entries")
        
        print(f"\n  Protocol:Market combinations found:")
        for proto_market, keys in sorted(self.discovered_keys.items(), key=lambda x: len(x[1]), reverse=True)[:10]:
            print(f"    {proto_market}: {len(keys):,} keys")
        
        return {
            'key_types': key_types,
            'streams': streams,
            'queues': queues,
            'patterns': key_patterns,
            'unknown': unknown_keys
        }

    def cleanup_streams(self, cutoff_timestamp: int, dry_run: bool = True, max_length: int = 10000):
        """Clean up Redis streams by trimming old entries."""
        print(f"\n🔍 Cleaning up streams (cutoff: {cutoff_timestamp}, max_length: {max_length})...")
        
        # Find all streams
        streams = []
        cursor = 0
        while True:
            cursor, keys = self.redis_client.scan(cursor, match="*:*:stream:*", count=1000)
            for key in keys:
                if self.redis_client.type(key) == 'stream':
                    streams.append(key)
            if cursor == 0:
                break
        
        # Also check for non-namespaced streams
        cursor = 0
        while True:
            cursor, keys = self.redis_client.scan(cursor, match="stream:*", count=1000)
            for key in keys:
                if self.redis_client.type(key) == 'stream' and key not in streams:
                    streams.append(key)
            if cursor == 0:
                break
        
        total_trimmed = 0
        for stream_key in streams:
            try:
                length = self.redis_client.xlen(stream_key)
                if length <= max_length:
                    continue
                
                if dry_run:
                    would_trim = length - max_length
                    self.stats[f"{stream_key} (stream)"] = would_trim
                    print(f"  🔍 DRY RUN: Would trim {would_trim:,} entries from {stream_key} (current: {length:,})")
                else:
                    # Trim stream to max_length using MINID (more efficient than MAXLEN for time-based)
                    trimmed = self.redis_client.xtrim(stream_key, maxlen=max_length, approximate=True)
                    if trimmed > 0:
                        self.stats[f"{stream_key} (stream)"] = trimmed
                        total_trimmed += trimmed
                        print(f"  ✓ Trimmed {trimmed:,} entries from {stream_key}")
            except Exception as e:
                print(f"  ⚠ Error cleaning stream {stream_key}: {e}")
        
        return total_trimmed

    def cleanup_queues(self, max_length: int = 1000, dry_run: bool = True):
        """Clean up Redis LIST queues that exceed max_length."""
        print(f"\n🔍 Cleaning up queues (max_length: {max_length})...")
        
        # Find all queues (LISTs)
        queues = []
        cursor = 0
        while True:
            cursor, keys = self.redis_client.scan(cursor, match="*:*:queue*", count=1000)
            for key in keys:
                if self.redis_client.type(key) == 'list':
                    queues.append(key)
            if cursor == 0:
                break
        
        # Also check for non-namespaced queues
        cursor = 0
        while True:
            cursor, keys = self.redis_client.scan(cursor, match="*queue*", count=1000)
            for key in keys:
                if self.redis_client.type(key) == 'list' and key not in queues:
                    queues.append(key)
            if cursor == 0:
                break
        
        total_trimmed = 0
        for queue_key in queues:
            try:
                length = self.redis_client.llen(queue_key)
                if length <= max_length:
                    continue
                
                if dry_run:
                    would_trim = length - max_length
                    self.stats[f"{queue_key} (queue)"] = would_trim
                    print(f"  🔍 DRY RUN: Would trim {would_trim:,} items from {queue_key} (current: {length:,})")
                else:
                    # Trim queue from the left (oldest items)
                    trimmed = self.redis_client.ltrim(queue_key, -max_length, -1)
                    if trimmed:
                        # LTRIM doesn't return count, so calculate it
                        new_length = self.redis_client.llen(queue_key)
                        trimmed_count = length - new_length
                        self.stats[f"{queue_key} (queue)"] = trimmed_count
                        total_trimmed += trimmed_count
                        print(f"  ✓ Trimmed {trimmed_count:,} items from {queue_key} (now: {new_length:,})")
            except Exception as e:
                print(f"  ⚠ Error cleaning queue {queue_key}: {e}")
        
        return total_trimmed

    def print_stats(self):
        """Print cleanup statistics."""
        print("\n" + "="*60)
        print("📊 Cleanup Statistics")
        print("="*60)
        total = sum(self.stats.values())
        for pattern, count in sorted(self.stats.items(), key=lambda x: x[1], reverse=True):
            print(f"  {pattern}: {count:,} keys/entries")
        print(f"\n  Total: {total:,} keys/entries")
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
    parser.add_argument('--keep-hours', type=int, default=None,
                        help='Number of recent hours to keep for timeline keys (overrides --keep-epochs for timelines)')
    parser.add_argument('--aggressive-timeline', action='store_true',
                        help='Enable aggressive timeline cleanup by extracting epoch IDs from entries')
    parser.add_argument('--discover', action='store_true',
                        help='Discovery mode: scan all keys and show what exists (no cleanup)')
    parser.add_argument('--cleanup-streams', action='store_true',
                        help='Clean up Redis streams (trim to max length)')
    parser.add_argument('--cleanup-queues', action='store_true',
                        help='Clean up Redis LIST queues (trim to max length)')
    parser.add_argument('--stream-max-length', type=int, default=10000,
                        help='Maximum length for streams after cleanup (default: 10000)')
    parser.add_argument('--queue-max-length', type=int, default=1000,
                        help='Maximum length for queues after cleanup (default: 1000)')
    parser.add_argument('--all-markets', action='store_true',
                        help='Clean up keys for all protocol:market combinations found (ignores --protocol and --market)')
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
    
    if args.discover:
        print("Mode: DISCOVERY (scanning all keys, no cleanup)")
    else:
        print(f"Keep epochs: {args.keep_epochs}")
        if args.keep_hours:
            print(f"Keep hours: {args.keep_hours}")
        print(f"Protocol: {args.protocol or 'auto-detect'}")
        print(f"Market: {args.market or 'auto-detect'}")
        if args.all_markets:
            print("⚠ All markets mode: will clean up keys for all protocol:market combinations")
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

        # Discovery mode
        if args.discover:
            discovery_results = cleanup.discover_all_keys()
            print("\n💡 Discovery complete. Use cleanup options to clean up keys.")
            return 0

        # Cleanup streams if requested
        if args.cleanup_streams:
            cutoff_timestamp = int(time.time()) - (args.keep_hours * 3600 if args.keep_hours else args.keep_epochs * 60)
            cleanup.cleanup_streams(cutoff_timestamp, dry_run=args.dry_run, max_length=args.stream_max_length)

        # Cleanup queues if requested
        if args.cleanup_queues:
            cleanup.cleanup_queues(max_length=args.queue_max_length, dry_run=args.dry_run)

        # If only queue/stream cleanup was requested (no epoch-based cleanup), exit here
        # This prevents unnecessary protocol/market checks since queue/stream cleanup doesn't need them
        if (args.cleanup_queues or args.cleanup_streams) and not args.all_markets and not args.protocol and not args.market and not args.keep_hours:
            cleanup.print_stats()
            if args.dry_run:
                print("\n💡 This was a dry run. Use without --dry-run to actually clean up.")
            else:
                print("\n✓ Queue/stream cleanup complete.")
            return 0

        # Handle all-markets mode
        if args.all_markets:
            # Discover all protocol:market combinations
            discovery_results = cleanup.discover_all_keys(max_keys=10000)
            proto_markets = list(cleanup.discovered_keys.keys())
            
            if not proto_markets:
                print("\n⚠ No protocol:market combinations found. Try --discover first.")
                return 1
            
            print(f"\n🔍 Found {len(proto_markets)} protocol:market combinations:")
            for pm in proto_markets[:20]:  # Show first 20
                print(f"  - {pm}: {len(cleanup.discovered_keys[pm])} keys")
            if len(proto_markets) > 20:
                print(f"  ... and {len(proto_markets) - 20} more")
            
            if args.dry_run:
                print("\n💡 DRY RUN: Would clean up keys for all markets above")
            else:
                confirm = input("\n⚠ Proceed with cleanup for ALL markets? (yes/no): ")
                if confirm.lower() != 'yes':
                    print("Aborted.")
                    return 1
            
            # Clean up each protocol:market combination
            total_deleted = 0
            total_failed = 0
            
            for proto_market in proto_markets:
                parts = proto_market.split(':')
                if len(parts) >= 2:
                    cleanup.protocol = parts[0]
                    cleanup.market = parts[1]
                    print(f"\n{'='*60}")
                    print(f"Cleaning up: {proto_market}")
                    print(f"{'='*60}")
                    
                    try:
                        current_epoch = cleanup.get_current_epoch()
                        keys_to_delete = cleanup.find_keys_to_delete(
                            current_epoch,
                            args.keep_epochs,
                            dry_run=args.dry_run,
                            keep_hours=args.keep_hours,
                            aggressive_timeline=args.aggressive_timeline
                        )
                        deleted, failed = cleanup.delete_keys(keys_to_delete, dry_run=args.dry_run)
                        total_deleted += deleted
                        total_failed += failed
                    except Exception as e:
                        print(f"⚠ Error cleaning {proto_market}: {e}")
                        total_failed += 1
            
            cleanup.print_stats()
            print(f"\n✓ Cleanup complete: {total_deleted:,} deleted, {total_failed} failed")
            return 0

        # Single protocol:market cleanup (original behavior)
        # Only require protocol/market if we need epoch-based cleanup (not for queue/stream/timeline-only cleanup)
        if (not args.protocol or not args.market) and not args.keep_hours and not args.cleanup_queues and not args.cleanup_streams:
            print("\n❌ Error: --protocol and --market are required for epoch-based cleanup")
            print("💡 Tip: Use --discover to see what protocol:market combinations exist")
            print("💡 Tip: Use --keep-hours to clean non-namespaced timelines without protocol/market")
            print("💡 Tip: Use --cleanup-queues or --cleanup-streams to clean queues/streams without protocol/market")
            return 1

        # Get current epoch (only needed for epoch-based cleanup)
        current_epoch = None
        if args.protocol and args.market:
            try:
                current_epoch = cleanup.get_current_epoch()
                print(f"\n✓ Current epoch: {current_epoch}")
            except ValueError as e:
                print(f"\n❌ Error: {e}")
                print("\nPlease specify --protocol and --market if auto-detection fails.")
                return 1
        elif args.keep_hours:
            # For timeline-only cleanup without protocol/market, just clean non-namespaced timelines
            print(f"\n💡 Timeline-only cleanup mode (using --keep-hours, no protocol/market)")
            # Call timeline cleanup directly
            cutoff_timestamp = int(time.time()) - (args.keep_hours * 3600)
            timeline_keys = [
                "metrics:epochs:timeline",
                "metrics:batches:timeline",
                "metrics:submissions:timeline",
                "metrics:validations:timeline",
            ]
            
            for timeline_key in timeline_keys:
                try:
                    if not cleanup.redis_client.exists(timeline_key):
                        continue
                    total_size = cleanup.redis_client.zcard(timeline_key)
                    if total_size == 0:
                        continue
                    count_to_remove = cleanup.redis_client.zcount(timeline_key, "-inf", cutoff_timestamp)
                    if count_to_remove > 0:
                        if args.dry_run:
                            cleanup.stats[f"{timeline_key} (timeline)"] = count_to_remove
                            print(f"  🔍 DRY RUN: Would remove {count_to_remove:,} entries from {timeline_key} (total: {total_size:,})")
                        else:
                            removed = cleanup.redis_client.zremrangebyscore(timeline_key, "-inf", cutoff_timestamp)
                            if removed > 0:
                                cleanup.stats[f"{timeline_key} (timeline)"] = removed
                                print(f"  ✓ Removed {removed:,} entries from {timeline_key}")
                except Exception as e:
                    print(f"⚠ Error cleaning timeline {timeline_key}: {e}")
            
            cleanup.print_stats()
            if args.dry_run:
                print("\n💡 This was a dry run. Use without --dry-run to actually delete keys.")
            else:
                print("\n✓ Timeline cleanup complete.")
            return 0

        # Find keys to delete
        keys_to_delete = cleanup.find_keys_to_delete(
            current_epoch or 0, 
            args.keep_epochs, 
            dry_run=args.dry_run,
            keep_hours=args.keep_hours,
            aggressive_timeline=args.aggressive_timeline
        )

        # Delete keys
        deleted, failed = cleanup.delete_keys(keys_to_delete, dry_run=args.dry_run)

        # Print stats
        cleanup.print_stats()

        if args.dry_run:
            print("\n💡 This was a dry run. Use without --dry-run to actually delete keys.")
        else:
            print(f"\n✓ Cleanup complete: {deleted:,} deleted, {failed} failed")

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

