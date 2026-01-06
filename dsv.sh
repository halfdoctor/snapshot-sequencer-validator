#!/bin/bash

# DSV - Decentralized Sequencer Validator Control Script
# Simplified and focused on production use

set -e

# Load environment variables if .env exists
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_color() {
    color=$1
    message=$2
    echo -e "${color}${message}${NC}"
}

# Detect docker compose command
if docker compose version &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker compose"
elif command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker-compose"
else
    print_color "$RED" "Error: Neither 'docker compose' nor 'docker-compose' found"
    exit 1
fi

# Function to show usage
show_usage() {
    echo "DSV - Decentralized Sequencer Validator Control"
    echo ""
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Main Commands:"
    echo "  start [--with-ipfs] [--with-vpa] [--no-monitor] [--rebuild]  - Start services (monitoring enabled by default)"
    echo "  stop                  - Stop all services"
    echo "  restart               - Restart all services"
    echo "  restart-monitor       - Restart only monitoring services (state-tracker, monitor-api)"
    echo "  status                - Show service status"
    echo "  clean                 - Stop and remove all containers/volumes"
    echo "  clean-queue          - Clean up stale aggregation queue items"
    echo "  clean-timeline       - Clean up timeline scientific notation entries"
    echo ""
    echo "Monitoring:"
    echo "  dashboard     - Open dashboard in browser (http://localhost:9091/swagger)"
    echo "  logs          - Show all logs"
    echo "  p2p-logs      - P2P Gateway logs"
    echo "  aggregator-logs - Aggregator logs"
    echo "  finalizer-logs - Finalizer logs"
    echo "  dequeuer-logs - Dequeuer logs"
    echo "  event-logs    - Event monitor logs"
    echo "  redis-logs    - Redis logs"
    echo "  ipfs-logs     - IPFS node logs"
    echo "  relayer-logs  - Relayer-PY VPA service logs"
    echo ""
    echo "Stream Debugging:"
    echo "  stream-info   - Show Redis streams status"
    echo "  stream-groups - Show consumer groups"
    echo "  stream-dlq    - Show dead letter queue"
    echo "  stream-reset  - Reset stream (dangerous!)"
    echo ""
    echo "Development:"
    echo "  build         - Build all binaries"
    echo "  dev           - Start unified sequencer (single container)"
}

# Check if separated mode is running
is_separated_running() {
    $DOCKER_COMPOSE_CMD -f docker-compose.separated.yml ps --services 2>/dev/null | grep -q p2p-gateway
}

# Check if code has changed since last build
check_code_changes() {
    local last_build_file=".dsv-last-build"
    local current_head=""
    local last_build_head=""
    
    # Check if we're in a git repository
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        return 0  # Not a git repo, skip check
    fi
    
    # Check for uncommitted changes
    if ! git diff-index --quiet HEAD -- 2>/dev/null; then
        print_color "$YELLOW" "⚠️  WARNING: You have uncommitted changes in your working directory"
        print_color "$YELLOW" "   These changes will NOT be included unless you rebuild"
        return 1
    fi
    
    # Get current HEAD
    current_head=$(git rev-parse HEAD 2>/dev/null)
    if [ -z "$current_head" ]; then
        return 0  # Can't get HEAD, skip check
    fi
    
    # Check if last build file exists
    if [ -f "$last_build_file" ]; then
        last_build_head=$(cat "$last_build_file" 2>/dev/null)
        
        if [ -n "$last_build_head" ] && [ "$current_head" != "$last_build_head" ]; then
            print_color "$YELLOW" "⚠️  WARNING: Code has changed since last build"
            print_color "$CYAN" "   Last build: $(git log -1 --format='%h %s' "$last_build_head" 2>/dev/null || echo "$last_build_head")"
            print_color "$CYAN" "   Current:    $(git log -1 --format='%h %s' "$current_head" 2>/dev/null || echo "$current_head")"
            print_color "$YELLOW" "   You may be running old code. Use --rebuild to rebuild with latest changes."
            return 1
        fi
    fi
    
    return 0
}

# Record current HEAD as last build
record_build() {
    local last_build_file=".dsv-last-build"
    
    if git rev-parse --git-dir > /dev/null 2>&1; then
        git rev-parse HEAD > "$last_build_file" 2>/dev/null
    fi
}

# Start production (separated) mode
start_services() {
    # Check for flags
    local enable_monitoring=true
    local enable_ipfs=false
    local enable_vpa=false
    local force_rebuild=false
    local compose_args="-f docker-compose.separated.yml"

    # Parse arguments
    for arg in "$@"; do
        case $arg in
            --no-monitor)
                enable_monitoring=false
                ;;
            --with-ipfs)
                enable_ipfs=true
                ;;
            --with-vpa)
                enable_vpa=true
                ;;
            --rebuild)
                force_rebuild=true
                ;;
        esac
    done

    # Check IPFS directory if --with-ipfs flag is used
    if [ "$enable_ipfs" = true ]; then
        IPFS_DIR="${IPFS_DATA_DIR:-/data/ipfs}"

        if [ ! -d "$IPFS_DIR" ]; then
            print_color "$RED" "❌ CRITICAL: IPFS data directory does not exist"
            print_color "$YELLOW" "Directory needed: $IPFS_DIR"
            print_color "$YELLOW" "Run these commands BEFORE starting services:"
            print_color "$YELLOW" "  sudo mkdir -p $IPFS_DIR"
            print_color "$YELLOW" "  sudo chown -R 1000:1000 $IPFS_DIR"
            print_color "$YELLOW" "  sudo chmod -R 755 $IPFS_DIR"
            print_color "$YELLOW" ""
            print_color "$YELLOW" "Then try again: ./dsv.sh start --with-ipfs"
            exit 1
        fi

    # Check permissions if IPFS was enabled
        if [ "$enable_ipfs" = true ] && ([ ! -r "$IPFS_DIR" ] || [ ! -w "$IPFS_DIR" ]); then
            print_color "$RED" "❌ CRITICAL: IPFS data directory has incorrect permissions"
            print_color "$YELLOW" "Directory: $IPFS_DIR"
            print_color "$YELLOW" "Required: Read/write access for user 1000:1000"
            print_color "$YELLOW" "Fix with: sudo chown -R 1000:1000 $IPFS_DIR && sudo chmod -R 755 $IPFS_DIR"
            exit 1
        fi

        if [ "$enable_ipfs" = true ]; then
            print_color "$GREEN" "✅ IPFS directory verified: $IPFS_DIR"
        fi
    fi

    # Check VPA configuration if --with-vpa flag is used
    if [ "$enable_vpa" = true ]; then
        if [ -z "$RELAYER_PY_ENDPOINT" ]; then
            print_color "$YELLOW" "⚠️  WARNING: RELAYER_PY_ENDPOINT not set, using default"
        fi
        if [ -z "$VPA_SIGNER_ADDRESSES" ] || [ -z "$VPA_SIGNER_PRIVATE_KEYS" ]; then
            print_color "$RED" "❌ CRITICAL: VPA multi-signer configuration incomplete"
            print_color "$YELLOW" "Required environment variables:"
            print_color "$YELLOW" "  VPA_SIGNER_ADDRESSES - Comma-separated list of signer addresses"
            print_color "$YELLOW" "  VPA_SIGNER_PRIVATE_KEYS - Comma-separated list of private keys"
            print_color "$YELLOW" ""
            print_color "$YELLOW" "Add these to your .env file, then try again:"
            print_color "$YELLOW" "  ./dsv.sh start --with-vpa"
            exit 1
        fi

        # Clone relayer-py repository (relative to current directory)
        RELAYER_REPO="https://github.com/powerloom/relayer-py.git"
        RELAYER_DIR="./relayer-py"

        # Remove existing relayer-py directory if it exists
        if [ -d "$RELAYER_DIR" ]; then
            print_color "$YELLOW" "🗑️  Removing existing relayer-py directory..."
            rm -rf "$RELAYER_DIR"
        fi

        # Clone relayer-py repository
        print_color "$CYAN" "📦 Cloning relayer-py repository to $RELAYER_DIR..."
        if ! git clone "$RELAYER_REPO" "$RELAYER_DIR"; then
            print_color "$RED" "❌ Failed to clone relayer-py repository"
            print_color "$YELLOW" "Repository: $RELAYER_REPO"
            print_color "$YELLOW" "Target directory: $RELAYER_DIR"
            exit 1
        fi
        print_color "$GREEN" "✅ Successfully cloned relayer-py"

        # Switch to master branch
        print_color "$CYAN" "🔄 Switching to master branch..."
        if ! (cd "$RELAYER_DIR" && git checkout master); then
            print_color "$RED" "❌ Failed to switch to master branch"
            print_color "$YELLOW" "Continuing with default branch"
        else
            print_color "$GREEN" "✅ Switched to master branch"
        fi

        # relayer-py now reads settings directly from environment variables
        print_color "$CYAN" "🔧 relayer-py will configure from environment variables"
        print_color "$GREEN" "✅ VPA integration configured via env vars"
    fi

    print_color "$GREEN" "🚀 Starting Separated Architecture"

    if [ ! -f docker-compose.separated.yml ]; then
        print_color "$RED" "Error: docker-compose.separated.yml not found"
        exit 1
    fi

    # Check environment
    if [ ! -f .env ]; then
        print_color "$YELLOW" "Warning: .env file not found. Using defaults."
    fi

    # Build profiles argument array
    local profile_args=()
    if [ "$enable_monitoring" = true ]; then
        profile_args+=(--profile monitoring)
    fi
    if [ "$enable_ipfs" = true ]; then
        profile_args+=(--profile ipfs)
    fi
    if [ "$enable_vpa" = true ]; then
        profile_args+=(--profile vpa)
    fi

    # Check for code changes if not forcing rebuild
    if [ "$force_rebuild" = false ]; then
        if ! check_code_changes; then
            echo ""
            read -p "Continue without rebuilding? (y/N) " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                print_color "$YELLOW" "Cancelled. Use --rebuild flag to rebuild: ./dsv.sh start --rebuild"
                exit 1
            fi
        fi
    fi
    
    # Start services with specified profiles
    print_color "$CYAN" "Starting services..."
    
    # Build flag - only add if force_rebuild is true
    local build_flag=""
    if [ "$force_rebuild" = true ]; then
        build_flag="--build"
        print_color "$YELLOW" "⚠️  Force rebuild enabled - images will be rebuilt"
    fi
    
    if [ ${#profile_args[@]} -gt 0 ]; then
        print_color "$CYAN" "With profiles: ${profile_args[*]}"
        if [ -n "$build_flag" ]; then
            $DOCKER_COMPOSE_CMD $compose_args "${profile_args[@]}" up -d $build_flag
        else
            $DOCKER_COMPOSE_CMD $compose_args "${profile_args[@]}" up -d
        fi
    else
        print_color "$CYAN" "Without additional profiles"
        if [ -n "$build_flag" ]; then
            $DOCKER_COMPOSE_CMD $compose_args up -d $build_flag
        else
            $DOCKER_COMPOSE_CMD $compose_args up -d
        fi
    fi

    if [ $? -eq 0 ]; then
        # Record build if we rebuilt
        if [ "$force_rebuild" = true ]; then
            record_build
        fi
        
        print_color "$GREEN" "✅ Services started successfully"
        echo ""
        print_color "$CYAN" "Components:"
        echo "  • P2P Gateway (port ${P2P_PORT:-9001})"
        echo "  • Aggregator (consensus)"
        echo "  • Finalizer (batch creation)"
        echo "  • Dequeuer (submission processing)"
        echo "  • Event Monitor (epoch tracking)"
        if [ "$enable_ipfs" = true ]; then
            echo "  • IPFS Node (ports ${IPFS_API_PORT:-5001}/${IPFS_GATEWAY_PORT:-8080})"
        fi
        if [ "$enable_monitoring" = true ]; then
            echo "  • State Tracker (data aggregation)"
            echo "  • Monitor API (dashboard)"
        fi
        echo ""
        if [ "$enable_monitoring" = true ]; then
            echo "View dashboard: http://localhost:${MONITOR_API_PORT:-9091}/swagger/index.html"
        fi
        echo "Logs: ./dsv.sh logs"
    else
        print_color "$RED" "❌ Failed to start services"
        exit 1
    fi
}

# Stop services
stop_services() {
    print_color "$YELLOW" "Stopping all services..."
    if is_separated_running; then
        # Stop all services including monitoring, ipfs, and vpa profiles
        $DOCKER_COMPOSE_CMD -f docker-compose.separated.yml --profile monitoring --profile ipfs --profile vpa down
    else
        # Try to stop any running containers
        $DOCKER_COMPOSE_CMD down 2>/dev/null || true
    fi
    print_color "$GREEN" "✓ All services stopped"
}

# Restart monitoring services only
restart_monitoring() {
    print_color "$CYAN" "🔄 Restarting monitoring services (state-tracker, monitor-api)..."
    if is_separated_running; then
        # Restart only monitoring profile services
        $DOCKER_COMPOSE_CMD -f docker-compose.separated.yml --profile monitoring restart
        if [ $? -eq 0 ]; then
            print_color "$GREEN" "✅ Monitoring services restarted successfully"
            echo ""
            print_color "$CYAN" "Services restarted:"
            echo "  • State Tracker (data aggregation)"
            echo "  • Monitor API (dashboard)"
        else
            print_color "$RED" "❌ Failed to restart monitoring services"
            exit 1
        fi
    else
        print_color "$YELLOW" "No services running. Start services first with: ./dsv.sh start"
        exit 1
    fi
}

# Show status
show_status() {
    print_color "$BLUE" "Service Status:"
    echo ""
    if is_separated_running; then
        $DOCKER_COMPOSE_CMD -f docker-compose.separated.yml ps
    else
        print_color "$YELLOW" "No services running"
    fi
}


# Open dashboard
open_dashboard() {
    MONITOR_PORT="${MONITOR_API_PORT:-9091}"
    URL="http://localhost:${MONITOR_PORT}/swagger/index.html"

    print_color "$CYAN" "Opening dashboard at: $URL"

    # Check if monitor API is running
    if curl -s -o /dev/null -w "%{http_code}" "http://localhost:${MONITOR_PORT}/api/v1/health" | grep -q "200"; then
        print_color "$GREEN" "✓ Monitor API is running"

        # Try to open in browser
        if command -v open > /dev/null 2>&1; then
            open "$URL"
        elif command -v xdg-open > /dev/null 2>&1; then
            xdg-open "$URL"
        else
            print_color "$YELLOW" "Please open in your browser: $URL"
        fi
    else
        print_color "$YELLOW" "Monitor API is not running"
        print_color "$YELLOW" "Start it with: ./dsv.sh monitor-start"
    fi
}


# Clean Redis cache state (stale keys from old deployments)
clean_cache() {
    print_color "$BLUE" "🗑️  Cleaning Redis cache state"

    # Find Redis container
    REDIS_CONTAINER=$(docker ps --format "{{.Names}}" | grep -i redis | head -1)

    if [ -z "$REDIS_CONTAINER" ]; then
        print_color "$RED" "Error: Redis container not found"
        print_color "$YELLOW" "Start services first with: ./dsv.sh start"
        return 1
    fi

    print_color "$YELLOW" "This will flush stale keys from Redis:"
    echo "  - Finalized batches (*:*:finalized:*)"
    echo "  - Aggregated batches (*:*:batch:aggregated:*)"
    echo "  - Batch parts (*:*:batch:part:*)"
    echo "  - Aggregation queues"
    echo ""
    read -p "Continue? (y/N) " -n 1 -r
    echo

    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Get Redis port from container
        REDIS_PORT=$(docker exec "$REDIS_CONTAINER" sh -c 'echo $REDIS_PORT' 2>/dev/null)
        if [ -z "$REDIS_PORT" ]; then
            REDIS_PORT=6379
        fi

        # Delete finalized batches using SCAN
        FINALIZED_COUNT=0
        docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT --scan --pattern '*:*:finalized:*' 2>/dev/null | while read key; do
            if [ ! -z "$key" ]; then
                docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT DEL "$key" >/dev/null 2>&1
                FINALIZED_COUNT=$((FINALIZED_COUNT + 1))
            fi
        done

        # Delete aggregated batches using SCAN
        AGG_COUNT=0
        docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT --scan --pattern '*:*:batch:aggregated:*' 2>/dev/null | while read key; do
            if [ ! -z "$key" ]; then
                docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT DEL "$key" >/dev/null 2>&1
                AGG_COUNT=$((AGG_COUNT + 1))
            fi
        done

        # Delete batch parts using SCAN
        PARTS_COUNT=0
        docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT --scan --pattern '*:*:batch:part:*' 2>/dev/null | while read key; do
            if [ ! -z "$key" ]; then
                docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT DEL "$key" >/dev/null 2>&1
                PARTS_COUNT=$((PARTS_COUNT + 1))
            fi
        done

        # Clear namespaced aggregation queues using SCAN
        docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT --scan --pattern '*:*:aggregationQueue' 2>/dev/null | while read key; do
            [ ! -z "$key" ] && docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT DEL "$key" >/dev/null 2>&1
        done
        docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT --scan --pattern '*:*:aggregation:queue' 2>/dev/null | while read key; do
            [ ! -z "$key" ] && docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT DEL "$key" >/dev/null 2>&1
        done

        print_color "$GREEN" "✓ Cache cleaned:"
        print_color "$GREEN" "  - Finalized batches: $FINALIZED_COUNT"
        print_color "$GREEN" "  - Aggregated batches: $AGG_COUNT"
        print_color "$GREEN" "  - Batch parts: $PARTS_COUNT"
    else
        print_color "$YELLOW" "Cancelled"
    fi
}

# Clean everything
clean_all() {
    print_color "$YELLOW" "⚠️  This will remove sequencer containers and volumes"
    read -p "Continue? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        stop_services
        stop_monitoring
        # Only remove volumes belonging to this project
        if is_separated_running || docker ps | grep -q snapshot-sequencer; then
            $DOCKER_COMPOSE_CMD -f docker-compose.separated.yml --profile vpa down -v 2>/dev/null || true
        fi
        if [ -f docker-compose.monitoring.yml ]; then
            $DOCKER_COMPOSE_CMD -f docker-compose.monitoring.yml down -v 2>/dev/null || true
        fi
        print_color "$GREEN" "✓ Cleanup complete"
    else
        print_color "$YELLOW" "Cancelled"
    fi
}

# Stream debugging functions

# Show Redis streams information
show_stream_info() {
    print_color "$BLUE" "📊 Redis Streams Status"
    echo ""

    # Find Redis container
    REDIS_CONTAINER=$(docker ps --format "{{.Names}}" | grep -i redis | head -1)

    if [ -z "$REDIS_CONTAINER" ]; then
        print_color "$RED" "Error: Redis container not found"
        print_color "$YELLOW" "Start services first with: ./dsv.sh start"
        return 1
    fi

    # Get Redis port
    REDIS_PORT=$(docker exec "$REDIS_CONTAINER" sh -c 'echo $REDIS_PORT' 2>/dev/null)
    if [ -z "$REDIS_PORT" ]; then
        REDIS_PORT=6379
    fi

    # Find all aggregation streams (namespaced)
    print_color "$CYAN" "Scanning for aggregation streams..."
    STREAMS=$(docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT --scan --pattern '*:*:stream:aggregation:notifications' 2>/dev/null)

    if [ -z "$STREAMS" ]; then
        print_color "$YELLOW" "No aggregation streams found"
        echo ""
        print_color "$CYAN" "Stream notifications may be disabled. Check ENABLE_STREAM_NOTIFICATIONS in .env"
        return
    fi

    for stream in $STREAMS; do
        print_color "$GREEN" "Stream: $stream"

        # Get stream info
        INFO=$(docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT XINFO STREAM "$stream" 2>/dev/null)
        if [ $? -eq 0 ]; then
            echo "$INFO" | while IFS= read -r line; do
                if [[ $line =~ length ]]; then
                    print_color "$CYAN" "  ├─ Entries: $(echo $line | cut -d' ' -f2)"
                elif [[ $line =~ last-generated-id ]]; then
                    print_color "$CYAN" "  ├─ Last ID: $(echo $line | cut -d' ' -f2)"
                elif [[ $line =~ groups ]]; then
                    print_color "$CYAN" "  └─ Groups: $(echo $line | cut -d' ' -f2)"
                fi
            done
        else
            print_color "$RED" "  └─ Error: Failed to get stream info"
        fi
        echo ""
    done
}

# Show consumer groups information
show_stream_groups() {
    print_color "$BLUE" "👥 Stream Consumer Groups"
    echo ""

    # Find Redis container
    REDIS_CONTAINER=$(docker ps --format "{{.Names}}" | grep -i redis | head -1)

    if [ -z "$REDIS_CONTAINER" ]; then
        print_color "$RED" "Error: Redis container not found"
        return 1
    fi

    # Get Redis port
    REDIS_PORT=$(docker exec "$REDIS_CONTAINER" sh -c 'echo $REDIS_PORT' 2>/dev/null)
    if [ -z "$REDIS_PORT" ]; then
        REDIS_PORT=6379
    fi

    # Find all aggregation streams
    STREAMS=$(docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT --scan --pattern '*:*:stream:aggregation:notifications' 2>/dev/null)

    if [ -z "$STREAMS" ]; then
        print_color "$YELLOW" "No aggregation streams found"
        return
    fi

    for stream in $STREAMS; do
        print_color "$GREEN" "Stream: $stream"

        # Get consumer groups
        GROUPS=$(docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT XINFO GROUPS "$stream" 2>/dev/null)
        if [ $? -eq 0 ]; then
            echo "$GROUPS" | while IFS= read -r line; do
                if [[ $line =~ name ]]; then
                    GROUP_NAME=$(echo $line | cut -d' ' -f2)
                    print_color "$CYAN" "  ├─ Group: $GROUP_NAME"
                elif [[ $line =~ consumers ]]; then
                    print_color "$CYAN" "  │  ├─ Consumers: $(echo $line | cut -d' ' -f2)"
                elif [[ $line =~ pending ]]; then
                    print_color "$CYAN" "  │  └─ Pending: $(echo $line | cut -d' ' -f2)"
                elif [[ $line =~ last-delivered-id ]]; then
                    print_color "$CYAN" "  │     └─ Last Delivered: $(echo $line | cut -d' ' -f2)"
                fi
            done

            # Get consumers for each group
            GROUPS_LIST=$(docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT XINFO GROUPS "$stream" 2>/dev/null | grep "name" | cut -d' ' -f2)
            for group in $GROUPS_LIST; do
                print_color "$YELLOW" "  └─ Consumers in group '$group':"

                CONSUMERS=$(docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT XINFO CONSUMERS "$stream" "$group" 2>/dev/null)
                if [ $? -eq 0 ]; then
                    echo "$CONSUMERS" | while IFS= read -r line; do
                        if [[ $line =~ name ]]; then
                            CONSUMER_NAME=$(echo $line | cut -d' ' -f2)
                            print_color "$CYAN" "     ├─ $CONSUMER_NAME"
                        elif [[ $line =~ pending ]]; then
                            print_color "$CYAN" "     │  ├─ Pending: $(echo $line | cut -d' ' -f2)"
                        elif [[ $line =~ idle ]]; then
                            IDLE_MS=$(echo $line | cut -d' ' -f2)
                            IDLE_SEC=$((IDLE_MS / 1000))
                            print_color "$CYAN" "     │  └─ Idle: ${IDLE_SEC}s"
                        fi
                    done
                else
                    print_color "$RED" "     └─ No consumers found"
                fi
            done
        else
            print_color "$RED" "  └─ No consumer groups found"
        fi
        echo ""
    done
}

# Show dead letter queue
show_stream_dlq() {
    print_color "$BLUE" "💀 Stream Dead Letter Queue"
    echo ""

    # Find Redis container
    REDIS_CONTAINER=$(docker ps --format "{{.Names}}" | grep -i redis | head -1)

    if [ -z "$REDIS_CONTAINER" ]; then
        print_color "$RED" "Error: Redis container not found"
        return 1
    fi

    # Get Redis port
    REDIS_PORT=$(docker exec "$REDIS_CONTAINER" sh -c 'echo $REDIS_PORT' 2>/dev/null)
    if [ -z "$REDIS_PORT" ]; then
        REDIS_PORT=6379
    fi

    # Find all dead letter queues (stream names ending with :dlq)
    DLQ_STREAMS=$(docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT --scan --pattern '*:*:stream:aggregation:notifications:dlq' 2>/dev/null)

    if [ -z "$DLQ_STREAMS" ]; then
        print_color "$GREEN" "✅ No messages in dead letter queues"
        return
    fi

    for dlq in $DLQ_STREAMS; do
        print_color "$YELLOW" "Dead Letter Queue: $dlq"

        # Get DLQ info
        INFO=$(docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT XINFO STREAM "$dlq" 2>/dev/null)
        if [ $? -eq 0 ]; then
            echo "$INFO" | while IFS= read -r line; do
                if [[ $line =~ length ]]; then
                    COUNT=$(echo $line | cut -d' ' -f2)
                    print_color "$RED" "  ├─ Failed Messages: $COUNT"
                fi
            done

            # Show last 5 messages
            print_color "$CYAN" "  └─ Recent failed messages:"
            docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT XREVRANGE "$dlq" + - COUNT 5 2>/dev/null | while IFS= read -r line; do
                if [[ $line =~ ^[0-9] ]]; then
                    MESSAGE_ID=$(echo $line | cut -d' ' -f1)
                    print_color "$RED" "     ├─ Message ID: $MESSAGE_ID"
                elif [[ $line =~ error_reason ]]; then
                    REASON=$(echo $line | cut -d' ' -f2-)
                    print_color "$RED" "     │  └─ Error: $REASON"
                elif [[ $line =~ error_time ]]; then
                    ERROR_TIME=$(echo $line | cut -d' ' -f2)
                    ERROR_DATE=$(date -d "@$ERROR_TIME" 2>/dev/null || echo "Invalid time")
                    print_color "$RED" "     │     └─ Time: $ERROR_DATE"
                fi
            done
        fi
        echo ""
    done
}

# Reset streams (dangerous operation)
reset_streams() {
    print_color "$RED" "⚠️  DANGER: This will delete all aggregation streams and consumer groups!"
    print_color "$YELLOW" "This will interrupt ongoing aggregation and may cause data loss."
    echo ""
    read -p "Are you absolutely sure? Type 'RESET' to continue: " confirmation

    if [ "$confirmation" != "RESET" ]; then
        print_color "$YELLOW" "Cancelled"
        return
    fi

    # Find Redis container
    REDIS_CONTAINER=$(docker ps --format "{{.Names}}" | grep -i redis | head -1)

    if [ -z "$REDIS_CONTAINER" ]; then
        print_color "$RED" "Error: Redis container not found"
        return 1
    fi

    # Get Redis port
    REDIS_PORT=$(docker exec "$REDIS_CONTAINER" sh -c 'echo $REDIS_PORT' 2>/dev/null)
    if [ -z "$REDIS_PORT" ]; then
        REDIS_PORT=6379
    fi

    print_color "$BLUE" "Resetting Redis streams..."

    # Delete all aggregation streams
    STREAMS=$(docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT --scan --pattern '*:*:stream:aggregation:notifications*' 2>/dev/null)
    DELETED_COUNT=0

    for stream in $STREAMS; do
        if docker exec $REDIS_CONTAINER redis-cli -p $REDIS_PORT DEL "$stream" >/dev/null 2>&1; then
            DELETED_COUNT=$((DELETED_COUNT + 1))
            print_color "$GREEN" "  ✓ Deleted: $stream"
        else
            print_color "$RED" "  ✗ Failed to delete: $stream"
        fi
    done

    print_color "$GREEN" "✅ Stream reset complete. Deleted $DELETED_COUNT streams."
    print_color "$YELLOW" "Note: Services will automatically recreate streams as needed."
}

# Build binaries
build_binaries() {
    print_color "$BLUE" "Building binaries..."
    if [ -f build-binary.sh ]; then
        ./build-binary.sh
    else
        print_color "$RED" "build-binary.sh not found"
        exit 1
    fi
}

# Show logs for specific service
show_service_logs() {
    service=$1
    lines=${2:-100}

    if is_separated_running; then
        $DOCKER_COMPOSE_CMD -f docker-compose.separated.yml logs -f --tail="$lines" "$service"
    else
        print_color "$YELLOW" "Services not running. Start with: ./dsv.sh start"
    fi
}

# Main command handler
case "${1:-}" in
    start|up)
        shift
        start_services "$@"
        ;;
    stop|down)
        stop_services
        ;;
    restart)
        stop_services
        sleep 2
        shift
        start_services "$@"
        ;;
    restart-monitor)
        restart_monitoring
        ;;
    status|ps)
        show_status
        ;;
      dashboard)
        open_dashboard
        ;;
    clean-cache)
        clean_cache
        ;;
    logs)
        if is_separated_running; then
            $DOCKER_COMPOSE_CMD -f docker-compose.separated.yml logs -f ${2:+--tail=$2}
        else
            print_color "$YELLOW" "No services running"
        fi
        ;;
    p2p-logs)
        show_service_logs "p2p-gateway" "$2"
        ;;
    aggregator-logs)
        show_service_logs "aggregator" "$2"
        ;;
    finalizer-logs)
        show_service_logs "finalizer" "$2"
        ;;
    dequeuer-logs)
        show_service_logs "dequeuer" "$2"
        ;;
    event-logs)
        show_service_logs "event-monitor" "$2"
        ;;
    redis-logs)
        show_service_logs "redis" "$2"
        ;;
    ipfs-logs)
        show_service_logs "ipfs" "$2"
        ;;
    relayer-logs)
        show_service_logs "relayer-py" "$2"
        ;;
    clean)
        clean_all
        ;;
    clean-queue)
        print_color "$YELLOW" "Cleaning up stale aggregation queue..."
        if [ -f "scripts/cleanup_stale_queue.sh" ]; then
            ./scripts/cleanup_stale_queue.sh
        else
            print_color "$RED" "Error: cleanup_stale_queue.sh script not found"
            exit 1
        fi
        ;;
    clean-timeline)
        print_color "$YELLOW" "Cleaning up timeline scientific notation entries..."
        if [ -f "scripts/cleanup_timeline_scientific_notation.sh" ]; then
            ./scripts/cleanup_timeline_scientific_notation.sh
        else
            print_color "$RED" "Error: cleanup_timeline_scientific_notation.sh script not found"
            exit 1
        fi
        ;;
    stream-info)
        show_stream_info
        ;;
    stream-groups)
        show_stream_groups
        ;;
    stream-dlq)
        show_stream_dlq
        ;;
    stream-reset)
        reset_streams
        ;;
    build)
        build_binaries
        ;;
    dev)
        print_color "$BLUE" "Starting unified sequencer (development mode)..."
        $DOCKER_COMPOSE_CMD up -d
        ;;
    help|--help|-h|"")
        show_usage
        ;;
    *)
        print_color "$RED" "Unknown command: $1"
        echo ""
        show_usage
        exit 1
        ;;
esac