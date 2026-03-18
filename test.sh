#!/bin/bash
# Quick integration test for agent status detection

set -e

echo "=== Agent Status Detection Quick Test ==="
echo ""

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Building test..."
go run cmd/test-agent-status/main.go

echo ""
echo "=== Test Complete ==="