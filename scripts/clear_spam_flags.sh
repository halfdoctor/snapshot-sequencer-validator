#!/bin/bash

# Utility script to clear spam protection flagged state from Redis
# Useful for testing and resetting spam protection state
#
# Usage:
#   ./clear_spam_flags.sh [protocol_state] [data_market] [--peer PEER_ID] [--snapshotter ADDRESS]
#
# Options:
#   --peer PEER_ID          Clear specific peer ID only
#   --snapshotter ADDRESS   Clear specific snapshotter address only
#   --all                   Clear all flagged state (default if no --peer/--snapshotter specified)
#
# If protocol_state and data_market are not provided, will prompt for them
# or use environment variables PROTOCOL_STATE_CONTRACT and DATA_MARKET_ADDRESSES
#
# Examples:
#   # Clear all flagged state
#   ./clear_spam_flags.sh 0x123... 0x456...
#
#   # Clear specific peer ID
#   ./clear_spam_flags.sh 0x123... 0x456... --peer QmPeerID123
#
#   # Clear specific snapshotter address
#   ./clear_spam_flags.sh 0x123... 0x456... --snapshotter 0x789...
#
#   # Clear both specific peer and snapshotter
#   ./clear_spam_flags.sh 0x123... 0x456... --peer QmPeerID123 --snapshotter 0x789...

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Redis connection (defaults to localhost:6379)
REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"

# Parse arguments
CLEAR_ALL=true
SPECIFIC_PEER_ID=""
SPECIFIC_SNAPSHOTTER_ADDR=""

# Parse positional and optional arguments
ARGS=()
while [[ $# -gt 0 ]]; do
    case $1 in
        --peer)
            SPECIFIC_PEER_ID="$2"
            CLEAR_ALL=false
            shift 2
            ;;
        --snapshotter)
            SPECIFIC_SNAPSHOTTER_ADDR="$2"
            CLEAR_ALL=false
            shift 2
            ;;
        --all)
            CLEAR_ALL=true
            shift
            ;;
        *)
            ARGS+=("$1")
            shift
            ;;
    esac
done

# Get protocol state and data market from positional args or env
PROTOCOL_STATE="${ARGS[0]:-${PROTOCOL_STATE_CONTRACT}}"
DATA_MARKET="${ARGS[1]:-${DATA_MARKET_ADDRESSES}}"

# If DATA_MARKET_ADDRESSES is comma-separated, take first one
if [[ "$DATA_MARKET" == *","* ]]; then
    DATA_MARKET=$(echo "$DATA_MARKET" | cut -d',' -f1 | tr -d ' ')
fi

# Prompt if still missing
if [ -z "$PROTOCOL_STATE" ]; then
    echo -e "${YELLOW}Protocol State Contract not provided.${NC}"
    read -p "Enter Protocol State Contract address: " PROTOCOL_STATE
fi

if [ -z "$DATA_MARKET" ]; then
    echo -e "${YELLOW}Data Market address not provided.${NC}"
    read -p "Enter Data Market address: " DATA_MARKET
fi

echo -e "${GREEN}Clearing spam protection flagged state...${NC}"
echo "Protocol State: $PROTOCOL_STATE"
echo "Data Market: $DATA_MARKET"
echo "Redis: $REDIS_HOST:$REDIS_PORT"
if [ "$CLEAR_ALL" = true ]; then
    echo "Mode: Clear ALL flagged state"
else
    if [ -n "$SPECIFIC_PEER_ID" ]; then
        echo "Mode: Clear specific peer ID: $SPECIFIC_PEER_ID"
    fi
    if [ -n "$SPECIFIC_SNAPSHOTTER_ADDR" ]; then
        echo "Mode: Clear specific snapshotter address: $SPECIFIC_SNAPSHOTTER_ADDR"
    fi
fi
echo ""

# Redis CLI command
REDIS_CLI="redis-cli -h $REDIS_HOST -p $REDIS_PORT"

# Check Redis connection
if ! $REDIS_CLI ping > /dev/null 2>&1; then
    echo -e "${RED}Error: Cannot connect to Redis at $REDIS_HOST:$REDIS_PORT${NC}"
    exit 1
fi

# Build patterns based on mode
if [ "$CLEAR_ALL" = true ]; then
    # Clear all flagged state
    PEER_PATTERN="${PROTOCOL_STATE}:${DATA_MARKET}:spam:consensus_flagged:peer:*"
    SNAPSHOTTER_PATTERN="${PROTOCOL_STATE}:${DATA_MARKET}:spam:consensus_flagged:snapshotter:*"
    CLEAR_PEER_SET=true
    CLEAR_SNAPSHOTTER_SET=true
else
    # Clear specific peer/snapshotter
    if [ -n "$SPECIFIC_PEER_ID" ]; then
        PEER_PATTERN="${PROTOCOL_STATE}:${DATA_MARKET}:spam:consensus_flagged:peer:${SPECIFIC_PEER_ID}"
        CLEAR_PEER_SET=true
    else
        PEER_PATTERN=""
        CLEAR_PEER_SET=false
    fi
    
    if [ -n "$SPECIFIC_SNAPSHOTTER_ADDR" ]; then
        SNAPSHOTTER_PATTERN="${PROTOCOL_STATE}:${DATA_MARKET}:spam:consensus_flagged:snapshotter:${SPECIFIC_SNAPSHOTTER_ADDR}"
        CLEAR_SNAPSHOTTER_SET=true
    else
        SNAPSHOTTER_PATTERN=""
        CLEAR_SNAPSHOTTER_SET=false
    fi
fi

# Flagged sets (no protocol prefix)
FLAGGED_PEERS_SET="flagged_peers:${DATA_MARKET}"
FLAGGED_SNAPSHOTTERS_SET="flagged_snapshotters:${DATA_MARKET}"

# Count keys before deletion
PEER_KEYS=0
SNAPSHOTTER_KEYS=0
if [ -n "$PEER_PATTERN" ]; then
    PEER_KEYS=$($REDIS_CLI --scan --pattern "$PEER_PATTERN" 2>/dev/null | wc -l | tr -d ' ')
fi
if [ -n "$SNAPSHOTTER_PATTERN" ]; then
    SNAPSHOTTER_KEYS=$($REDIS_CLI --scan --pattern "$SNAPSHOTTER_PATTERN" 2>/dev/null | wc -l | tr -d ' ')
fi

PEER_SET_COUNT=0
SNAPSHOTTER_SET_COUNT=0
if [ "$CLEAR_PEER_SET" = true ]; then
    if [ "$CLEAR_ALL" = true ]; then
        PEER_SET_COUNT=$($REDIS_CLI SCARD "$FLAGGED_PEERS_SET" 2>/dev/null || echo "0")
    elif [ -n "$SPECIFIC_PEER_ID" ]; then
        # Check if specific peer exists in set
        EXISTS=$($REDIS_CLI SISMEMBER "$FLAGGED_PEERS_SET" "$SPECIFIC_PEER_ID" 2>/dev/null || echo "0")
        if [ "$EXISTS" = "1" ]; then
            PEER_SET_COUNT=1
        fi
    fi
fi

if [ "$CLEAR_SNAPSHOTTER_SET" = true ]; then
    if [ "$CLEAR_ALL" = true ]; then
        SNAPSHOTTER_SET_COUNT=$($REDIS_CLI SCARD "$FLAGGED_SNAPSHOTTERS_SET" 2>/dev/null || echo "0")
    elif [ -n "$SPECIFIC_SNAPSHOTTER_ADDR" ]; then
        # Check if specific snapshotter exists in set
        EXISTS=$($REDIS_CLI SISMEMBER "$FLAGGED_SNAPSHOTTERS_SET" "$SPECIFIC_SNAPSHOTTER_ADDR" 2>/dev/null || echo "0")
        if [ "$EXISTS" = "1" ]; then
            SNAPSHOTTER_SET_COUNT=1
        fi
    fi
fi

echo "Found:"
if [ -n "$PEER_PATTERN" ]; then
    echo "  - Flagged peer keys matching pattern: $PEER_KEYS"
fi
if [ -n "$SNAPSHOTTER_PATTERN" ]; then
    echo "  - Flagged snapshotter keys matching pattern: $SNAPSHOTTER_KEYS"
fi
if [ "$CLEAR_PEER_SET" = true ]; then
    echo "  - Flagged peers set members to remove: $PEER_SET_COUNT"
fi
if [ "$CLEAR_SNAPSHOTTER_SET" = true ]; then
    echo "  - Flagged snapshotters set members to remove: $SNAPSHOTTER_SET_COUNT"
fi
echo ""

TOTAL_TO_CLEAR=$((PEER_KEYS + SNAPSHOTTER_KEYS + PEER_SET_COUNT + SNAPSHOTTER_SET_COUNT))
if [ "$TOTAL_TO_CLEAR" -eq 0 ]; then
    echo -e "${GREEN}No flagged state found matching criteria. Nothing to clear.${NC}"
    exit 0
fi

# Confirm deletion
if [ "$CLEAR_ALL" = true ]; then
    CONFIRM_MSG="Are you sure you want to clear ALL flagged state? (yes/no): "
else
    CONFIRM_MSG="Are you sure you want to clear the specified flagged state? (yes/no): "
fi
read -p "$CONFIRM_MSG" CONFIRM
if [ "$CONFIRM" != "yes" ]; then
    echo "Aborted."
    exit 0
fi

# Delete flagged peer keys
DELETED_PEERS=0
if [ -n "$PEER_PATTERN" ]; then
    echo -e "${YELLOW}Deleting flagged peer keys...${NC}"
    while IFS= read -r key; do
        if [ -n "$key" ]; then
            $REDIS_CLI DEL "$key" > /dev/null 2>&1
            DELETED_PEERS=$((DELETED_PEERS + 1))
            echo "  Deleted: $key"
        fi
    done < <($REDIS_CLI --scan --pattern "$PEER_PATTERN" 2>/dev/null)
fi

# Delete flagged snapshotter keys
DELETED_SNAPSHOTTERS=0
if [ -n "$SNAPSHOTTER_PATTERN" ]; then
    echo -e "${YELLOW}Deleting flagged snapshotter keys...${NC}"
    while IFS= read -r key; do
        if [ -n "$key" ]; then
            $REDIS_CLI DEL "$key" > /dev/null 2>&1
            DELETED_SNAPSHOTTERS=$((DELETED_SNAPSHOTTERS + 1))
            echo "  Deleted: $key"
        fi
    done < <($REDIS_CLI --scan --pattern "$SNAPSHOTTER_PATTERN" 2>/dev/null)
fi

# Remove from flagged sets
DELETED_FROM_PEER_SET=0
DELETED_FROM_SNAPSHOTTER_SET=0
if [ "$CLEAR_PEER_SET" = true ]; then
    echo -e "${YELLOW}Removing from flagged peers set...${NC}"
    if [ "$CLEAR_ALL" = true ]; then
        $REDIS_CLI DEL "$FLAGGED_PEERS_SET" > /dev/null 2>&1
        DELETED_FROM_PEER_SET=$PEER_SET_COUNT
        echo "  Deleted entire set: $FLAGGED_PEERS_SET"
    elif [ -n "$SPECIFIC_PEER_ID" ]; then
        REMOVED=$($REDIS_CLI SREM "$FLAGGED_PEERS_SET" "$SPECIFIC_PEER_ID" 2>/dev/null || echo "0")
        if [ "$REMOVED" = "1" ]; then
            DELETED_FROM_PEER_SET=1
            echo "  Removed peer ID: $SPECIFIC_PEER_ID"
        else
            echo "  Peer ID not found in set: $SPECIFIC_PEER_ID"
        fi
    fi
fi

if [ "$CLEAR_SNAPSHOTTER_SET" = true ]; then
    echo -e "${YELLOW}Removing from flagged snapshotters set...${NC}"
    if [ "$CLEAR_ALL" = true ]; then
        $REDIS_CLI DEL "$FLAGGED_SNAPSHOTTERS_SET" > /dev/null 2>&1
        DELETED_FROM_SNAPSHOTTER_SET=$SNAPSHOTTER_SET_COUNT
        echo "  Deleted entire set: $FLAGGED_SNAPSHOTTERS_SET"
    elif [ -n "$SPECIFIC_SNAPSHOTTER_ADDR" ]; then
        REMOVED=$($REDIS_CLI SREM "$FLAGGED_SNAPSHOTTERS_SET" "$SPECIFIC_SNAPSHOTTER_ADDR" 2>/dev/null || echo "0")
        if [ "$REMOVED" = "1" ]; then
            DELETED_FROM_SNAPSHOTTER_SET=1
            echo "  Removed snapshotter address: $SPECIFIC_SNAPSHOTTER_ADDR"
        else
            echo "  Snapshotter address not found in set: $SPECIFIC_SNAPSHOTTER_ADDR"
        fi
    fi
fi

echo ""
echo -e "${GREEN}✓ Cleared spam protection flagged state${NC}"
if [ "$DELETED_PEERS" -gt 0 ]; then
    echo "  - Deleted $DELETED_PEERS flagged peer key(s)"
fi
if [ "$DELETED_SNAPSHOTTERS" -gt 0 ]; then
    echo "  - Deleted $DELETED_SNAPSHOTTERS flagged snapshotter key(s)"
fi
if [ "$DELETED_FROM_PEER_SET" -gt 0 ]; then
    echo "  - Removed $DELETED_FROM_PEER_SET peer ID(s) from flagged peers set"
fi
if [ "$DELETED_FROM_SNAPSHOTTER_SET" -gt 0 ]; then
    echo "  - Removed $DELETED_FROM_SNAPSHOTTER_SET snapshotter address(es) from flagged snapshotters set"
fi
echo ""
echo -e "${YELLOW}Note: This only clears Redis cache. On-chain flagged state (if implemented) is not affected.${NC}"

