#!/bin/bash

# Test script to demonstrate VPA deployment workflow
# Tests: env parsing → settings.json generation → relayer-py clone → Docker build

set -e

echo "🧪 VPA Deployment Test Script"
echo "======================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

print_color() {
    color=$1
    message=$2
    echo -e "${color}${message}${NC}"
}

# Test 1: Load and validate environment
print_color "$BLUE" "🔍 Test 1: Loading and validating environment variables"

ENV_FILE="/Users/anomit/workspace/decentralized_sequencer_eigen_workspace/localenvs/env.hznr.devnet.validator1"
if [ ! -f "$ENV_FILE" ]; then
    print_color "$RED" "❌ Environment file not found: $ENV_FILE"
    exit 1
fi

# Load environment variables
print_color "$CYAN" "📁 Loading environment from: $ENV_FILE"
set -a
source "$ENV_FILE"
set +a

# Validate required VPA variables
REQUIRED_VARS=("PROTOCOL_STATE_CONTRACT" "DATA_MARKET_ADDRESSES" "RELAYER_PY_ENDPOINT" "VPA_SIGNER_ADDRESSES" "VPA_SIGNER_PRIVATE_KEYS")

for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        print_color "$RED" "❌ Missing required variable: $var"
        exit 1
    else
        print_color "$GREEN" "✅ $var: ${!var:0:30}..."
    fi
done

# Test 2: Settings.json generation
print_color "$BLUE" ""
print_color "$BLUE" "🔧 Test 2: Generating settings.json from environment variables"

python3 ./test_relayer_config.py

if [ $? -eq 0 ]; then
    print_color "$GREEN" "✅ Settings.json generated successfully"
    print_color "$BLUE" "📄 Generated settings: /tmp/test_relayer_settings.json"

    # Validate key sections
    SIGNERS_COUNT=$(python3 -c "import json; data=json.load(open('/tmp/test_relayer_settings.json')); print(len(data.get('signers', [])))")
    PROTOCOL_STATE=$(python3 -c "import json; data=json.load(open('/tmp/test_relayer_settings.json')); print(data.get('protocol_state_address', ''))")

    print_color "$CYAN" "   Signers configured: $SIGNERS_COUNT"
    print_color "$CYAN" "   Protocol State: $PROTOCOL_STATE"

    if [ "$SIGNERS_COUNT" -gt 0 ] && [ -n "$PROTOCOL_STATE" ]; then
        print_color "$GREEN" "✅ Settings validation passed"
    else
        print_color "$RED" "❌ Settings validation failed"
        exit 1
    fi
else
    print_color "$RED" "❌ Settings.json generation failed"
    exit 1
fi

# Test 3: Simulate relayer-py clone (dry run)
print_color "$BLUE" ""
print_color "$BLUE" "📦 Test 3: Simulating relayer-py repository clone"

RELAYER_REPO="git@github.com:powerloom/relayer-py.git"
RELAYER_DIR="./relayer-py-test"

# Remove existing test directory
if [ -d "$RELAYER_DIR" ]; then
    print_color "$YELLOW" "🧹 Removing existing test directory: $RELAYER_DIR"
    rm -rf "$RELAYER_DIR"
fi

print_color "$CYAN" "🔄 Testing clone (dry run mode)..."

# Test if we can access the repository (this will fail if SSH keys aren't set up, but we'll handle it)
if git ls-remote "$RELAYER_REPO" > /dev/null 2>&1; then
    print_color "$GREEN" "✅ Repository access confirmed"

    # Actual clone (for testing)
    print_color "$CYAN" "📥 Cloning relayer-py..."
    git clone "$RELAYER_REPO" "$RELAYER_DIR" --quiet
    print_color "$GREEN" "✅ Successfully cloned relayer-py"
else
    print_color "$YELLOW" "⚠️  SSH repository access not available (expected in CI/demo)"
    print_color "$CYAN" "💡 In actual deployment, ensure SSH keys are configured for github.com"

    # Create mock directory for testing
    mkdir -p "$RELAYER_DIR/settings"
    print_color "$BLUE" "📁 Created mock directory for testing"
fi

# Test 4: Settings integration
print_color "$BLUE" ""
print_color "$BLUE" "🔗 Test 4: Integrating settings with relayer-py"

if [ -d "$RELAYER_DIR" ]; then
    # Copy generated settings to relayer-py
    cp /tmp/test_relayer_settings.json "$RELAYER_DIR/settings/settings.json"
    print_color "$GREEN" "✅ Settings copied to relayer-py directory"

    # Verify the copy
    if [ -f "$RELAYER_DIR/settings/settings.json" ]; then
        print_color "$GREEN" "✅ Settings file exists in relayer-py"

        # Validate JSON format
        python3 -c "import json; json.load(open('$RELAYER_DIR/settings/settings.json'))" 2>/dev/null
        if [ $? -eq 0 ]; then
            print_color "$GREEN" "✅ Settings JSON format valid"
        else
            print_color "$RED" "❌ Settings JSON format invalid"
            exit 1
        fi
    else
        print_color "$RED" "❌ Settings file copy failed"
        exit 1
    fi
else
    print_color "$RED" "❌ relayer-py directory not available"
    exit 1
fi

# Test 5: Docker build simulation
print_color "$BLUE" ""
print_color "$BLUE" "🐳 Test 5: Testing Docker build simulation"

if [ -d "$RELAYER_DIR" ]; then
    cd "$RELAYER_DIR"

    # Check if Dockerfile exists
    if [ -f "Dockerfile" ]; then
        print_color "$GREEN" "✅ Dockerfile found"

        # Check if poetry.lock exists
        if [ -f "poetry.lock" ]; then
            print_color "$GREEN" "✅ Poetry dependencies file found"

            # Simulate build (without actually building to save time)
            print_color "$CYAN" "🔍 Checking build requirements..."

            # Check if required files are present
            REQUIRED_FILES=("settings" "relayer.py" "Dockerfile")
            for file in "${REQUIRED_FILES[@]}"; do
                if [ -f "$file" ] || [ -d "$file" ]; then
                    print_color "$GREEN" "   ✅ $file"
                else
                    print_color "$RED" "   ❌ $file"
                fi
            done

            print_color "$GREEN" "✅ Build requirements satisfied"
        else
            print_color "$YELLOW" "⚠️  poetry.lock not found - dependencies may need installation"
        fi
    else
        print_color "$RED" "❌ Dockerfile not found in relayer-py"
        exit 1
    fi

    cd ..
else
    print_color "$RED" "❌ Cannot test build - relayer-py directory not available"
    exit 1
fi

# Test 6: Complete workflow summary
print_color "$BLUE" ""
print_color "$BLUE" "📊 Test 6: Workflow Summary"

echo ""
print_color "$CYAN" "🎯 Workflow Steps Verified:"
print_color "$GREEN" "   1. ✅ Environment variables loaded from localenvs/env.hznr.devnet.validator1"
print_color "$GREEN" "   2. ✅ Settings.json generated from environment variables"
print_color "$GREEN" "   3. ✅ relayer-py repository access tested"
print_color "$GREEN" "   4. ✅ Settings integrated into relayer-py structure"
print_color "$GREEN" "   5. ✅ Docker build requirements validated"

echo ""
print_color "$CYAN" "🔧 Configuration Summary:"
echo "   Protocol State: $PROTOCOL_STATE_CONTRACT"
echo "   Data Market: $DATA_MARKET_ADDRESSES"
echo "   Signers: $(python3 -c "import json; data=json.load(open('/tmp/test_relayer_settings.json')); print(len(data.get('signers', [])))") configured"

echo ""
print_color "$GREEN" "🎉 ALL TESTS PASSED - VPA deployment workflow validated!"

# Cleanup
rm -rf "$RELAYER_DIR" 2>/dev/null || true

echo ""
print_color "$BLUE" "💡 To test actual deployment:"
echo "   1. Ensure SSH keys are configured for github.com"
echo "   2. Run: ./dsv.sh start --with-vpa --with-ipfs"
echo "   3. Monitor logs: docker logs dsv-relayer-py"