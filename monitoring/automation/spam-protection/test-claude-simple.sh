#!/bin/bash

# Simple test to verify Claude CLI is working
# This helps diagnose if Claude itself is the problem

echo "=== Simple Claude Test ==="
echo "Testing basic Claude functionality..."
echo ""

echo "Test 1: Check if Claude exists"
if command -v claude &> /dev/null; then
    echo "✓ Claude found: $(which claude)"
    claude --version 2>&1 || echo "✗ Version check failed"
else
    echo "✗ Claude not found in PATH"
    exit 1
fi
echo ""

echo "Test 2: Simple prompt via echo pipe"
echo "Say 'test passed' and nothing else" | claude -p 2>&1
echo "Exit code: $?"
echo ""

echo "Test 3: Simple heredoc"
claude -p --permission-mode bypassPermissions 2>&1 <<EOF
Say "heredoc test passed" and nothing else.
EOF
echo "Exit code: $?"
echo ""

echo "Test 4: Heredoc with permission mode"
claude -p --permission-mode bypassPermissions 2>&1 <<EOF
You are a test assistant. Respond with only: "Permission test passed"
EOF
echo "Exit code: $?"
echo ""

echo "=== Test Complete ==="
