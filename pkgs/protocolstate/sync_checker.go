package protocolstate

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// WaitForColdSyncCompletion waits for the protocol state cacher to complete cold sync
// by checking Redis for the last sync timestamp. This is used by components that
// depend on cached slot data but don't run the cacher themselves.
func WaitForColdSyncCompletion(
	ctx context.Context,
	redisClient *redis.Client,
	protocolStateContract string,
	snapshotterStateContract string,
	syncInterval time.Duration,
	maxWaitTime time.Duration,
) error {
	if redisClient == nil {
		return fmt.Errorf("Redis client is required")
	}
	if protocolStateContract == "" {
		return fmt.Errorf("ProtocolState contract address is required")
	}
	if snapshotterStateContract == "" {
		return fmt.Errorf("SnapshotterState contract address is required")
	}

	protocolStateAddr := common.HexToAddress(protocolStateContract)
	snapshotterStateAddr := common.HexToAddress(snapshotterStateContract)
	syncKey := getLastSyncKey(protocolStateAddr, snapshotterStateAddr)

	log.Info("⏳ Waiting for protocol state cold sync to complete...")
	log.Infof("   Checking Redis key: %s", syncKey)

	startTime := time.Now()
	ticker := time.NewTicker(2 * time.Second) // Check every 2 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for cold sync")
		case <-ticker.C:
			// Check if max wait time exceeded
			if time.Since(startTime) > maxWaitTime {
				return fmt.Errorf("timeout waiting for cold sync completion after %v", maxWaitTime)
			}

			// Check Redis for last sync timestamp
			timestampStr, err := redisClient.Get(ctx, syncKey).Result()
			if err == redis.Nil {
				// No timestamp yet - cacher hasn't completed initial sync
				elapsed := time.Since(startTime)
				if elapsed.Seconds() > 0 && int(elapsed.Seconds())%10 == 0 {
					log.Infof("   Still waiting for cold sync... (elapsed: %v)", elapsed.Round(time.Second))
				}
				continue
			}
			if err != nil {
				log.Warnf("   Error checking cold sync status: %v (retrying...)", err)
				continue
			}

			// Parse timestamp
			lastSync, err := time.Parse(time.RFC3339, timestampStr)
			if err != nil {
				log.Warnf("   Error parsing sync timestamp: %v (retrying...)", err)
				continue
			}

			// Check if sync is recent enough
			age := time.Since(lastSync)
			if age > syncInterval {
				log.Warnf("   Last cold sync was %v ago (threshold: %v) - may need resync", age, syncInterval)
				// Still proceed - cacher will handle resync
			}

			elapsed := time.Since(startTime)
			log.Infof("✅ Cold sync completed (last sync: %s, waited: %v)",
				lastSync.Format(time.RFC3339), elapsed.Round(time.Second))
			return nil
		}
	}
}

// getLastSyncKey returns the Redis key for last sync timestamp
func getLastSyncKey(protocolStateAddr, snapshotterStateAddr common.Address) string {
	return fmt.Sprintf("%s:%s:ColdSync.LastSyncTimestamp",
		protocolStateAddr.Hex(),
		snapshotterStateAddr.Hex())
}
