#!/bin/bash

# Debug script to check what Claude is doing when it hangs

echo "=== Claude Hang Debugging ==="
echo ""

echo "1. Check if Claude process is running:"
ps aux | grep -E "[c]laude" || echo "  No Claude processes found"
echo ""

echo "2. Check Claude processes with details:"
ps aux | grep -E "[c]laude" | head -5
echo ""

echo "3. Check for stuck file descriptors:"
if [ -f "/proc/$$/fd/0" ]; then
    echo "  Process file descriptors:"
    ls -la /proc/$$/fd/ | grep -E "(pipe|socket)" || echo "  (Cannot check - /proc not available)"
else
    echo "  (Cannot check - /proc not available on this system)"
fi
echo ""

echo "4. Test simple Claude invocation (5 second timeout):"
timeout 5 claude -p --permission-mode bypassPermissions 2>&1 <<EOF
Say "test" and exit immediately.
EOF
SIMPLE_EXIT=$?
if [ $SIMPLE_EXIT -eq 124 ]; then
    echo "  ✗ Simple test TIMED OUT - Claude is hanging even on simple prompts"
elif [ $SIMPLE_EXIT -ne 0 ]; then
    echo "  ✗ Simple test FAILED with exit code: $SIMPLE_EXIT"
else
    echo "  ✓ Simple test PASSED"
fi
echo ""

echo "5. Check Claude environment:"
echo "  CLAUDE_API_KEY: ${CLAUDE_API_KEY:+SET (hidden)}${CLAUDE_API_KEY:-NOT SET}"
echo "  PATH: $PATH"
echo ""

echo "6. Check Claude config:"
if [ -f "$HOME/.claude/settings.json" ]; then
    echo "  Global settings exist"
    cat "$HOME/.claude/settings.json" | head -20
else
    echo "  No global settings found"
fi
echo ""

echo "=== Debug Complete ==="
echo ""
echo "If Claude is hanging, try:"
echo "  1. Kill any stuck processes: pkill -f claude"
echo "  2. Check Claude logs: ~/.claude/logs/ (if exists)"
echo "  3. Verify API key is set: echo \$CLAUDE_API_KEY"
echo "  4. Test with simpler command: echo 'test' | claude -p"
