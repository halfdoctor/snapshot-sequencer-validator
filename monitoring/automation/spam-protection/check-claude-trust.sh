#!/bin/bash

# Check if Claude trusts a directory
# Usage: ./check-claude-trust.sh [directory_path]

set -e

DIR_TO_CHECK="${1:-$(pwd)}"
DIR_TO_CHECK="$(cd "$DIR_TO_CHECK" && pwd)"  # Normalize path

echo "Checking Claude trust for: $DIR_TO_CHECK"
echo ""

# Check global settings
GLOBAL_SETTINGS="$HOME/.claude/settings.json"
if [ -f "$GLOBAL_SETTINGS" ]; then
    echo "=== Global Settings ($GLOBAL_SETTINGS) ==="
    if command -v jq &> /dev/null; then
        echo "Permissions configuration:"
        jq -r '.permissions // "Not configured"' "$GLOBAL_SETTINGS" 2>/dev/null || echo "  (Invalid JSON or no permissions section)"
        echo ""
    else
        echo "  (Install 'jq' for better JSON parsing)"
        grep -i "permission\|allow\|trust" "$GLOBAL_SETTINGS" || echo "  (No permission-related settings found)"
        echo ""
    fi
else
    echo "=== Global Settings ==="
    echo "  Not found: $GLOBAL_SETTINGS"
    echo ""
fi

# Check project-level settings
PROJECT_SETTINGS="$DIR_TO_CHECK/.claude/settings.json"
PROJECT_LOCAL_SETTINGS="$DIR_TO_CHECK/.claude/settings.local.json"

if [ -f "$PROJECT_SETTINGS" ]; then
    echo "=== Project Settings ($PROJECT_SETTINGS) ==="
    if command -v jq &> /dev/null; then
        echo "Permissions configuration:"
        jq -r '.permissions // "Not configured"' "$PROJECT_SETTINGS" 2>/dev/null || echo "  (Invalid JSON)"
        echo ""
    else
        grep -i "permission\|allow\|trust" "$PROJECT_SETTINGS" || echo "  (No permission-related settings found)"
        echo ""
    fi
else
    echo "=== Project Settings ==="
    echo "  Not found: $PROJECT_SETTINGS"
    echo ""
fi

if [ -f "$PROJECT_LOCAL_SETTINGS" ]; then
    echo "=== Project Local Settings ($PROJECT_LOCAL_SETTINGS) ==="
    if command -v jq &> /dev/null; then
        echo "Permissions configuration:"
        jq -r '.permissions // "Not configured"' "$PROJECT_LOCAL_SETTINGS" 2>/dev/null || echo "  (Invalid JSON)"
        echo ""
    else
        grep -i "permission\|allow\|trust" "$PROJECT_LOCAL_SETTINGS" || echo "  (No permission-related settings found)"
        echo ""
    fi
else
    echo "=== Project Local Settings ==="
    echo "  Not found: $PROJECT_LOCAL_SETTINGS"
    echo ""
fi

# Check parent directories for .claude settings
echo "=== Checking Parent Directories ==="
CURRENT_DIR="$DIR_TO_CHECK"
FOUND_PARENT_SETTINGS=false

while [ "$CURRENT_DIR" != "/" ] && [ "$CURRENT_DIR" != "$HOME" ]; do
    PARENT_SETTINGS="$CURRENT_DIR/.claude/settings.json"
    if [ -f "$PARENT_SETTINGS" ]; then
        echo "Found: $PARENT_SETTINGS"
        if command -v jq &> /dev/null; then
            jq -r '.permissions.defaultMode // "Not set"' "$PARENT_SETTINGS" 2>/dev/null | sed 's/^/  defaultMode: /'
        fi
        FOUND_PARENT_SETTINGS=true
    fi
    CURRENT_DIR="$(dirname "$CURRENT_DIR")"
done

if [ "$FOUND_PARENT_SETTINGS" = false ]; then
    echo "  No parent directory settings found"
fi

echo ""
echo "=== Summary ==="
echo "To trust this directory, create: $DIR_TO_CHECK/.claude/settings.json"
echo "With content:"
echo '{'
echo '  "permissions": {'
echo '    "defaultMode": "bypassPermissions",'
echo '    "allow": ["Read(./*)", "Edit(./*)", "Write(./*)"]'
echo '  }'
echo '}'
