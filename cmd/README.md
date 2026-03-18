# Agent Status Detection Tests

This directory contains tests for the agent status detection feature.

## Test Programs

### 1. `cmd/test-agent-status/`
**Comprehensive integration test** - Tests the full agent status detection system:
- Creates test tmux sessions with `[ai]` tagged windows
- Verifies `awaiting_input` on idle windows
- Starts CPU-intensive tasks and verifies `cooking` status
- Tests hysteresis (3 samples before switching back)
- Verifies isolation (windows without `[ai]` tag not tracked)

**Run:** `./test.sh` or `go run cmd/test-agent-status/main.go`

### 2. `cmd/test-opencode/`
**Real-world OpenCode test** - Tests with actual opencode instance:
- Sends commands to opencode to trigger work
- Monitors with real adaptive baseline logic
- Shows when cooking status is triggered
- Verifies the fix for opencode's subtle State=R behavior

**Run:** `go run cmd/test-opencode/main.go`

**Usage:** Run while opencode is running in a tmux window

### 3. `cmd/test-sensitivity/`
**Ultra-sensitive State=R detection** - Diagnostic tool:
- Uses ANY State=R as cooking indicator (no CPU threshold)
- Helps diagnose processes that enter State=R but don't spike CPU
- Shows whether a process uses foreground State=R for work

**Run:** `go run cmd/test-sensitivity/main.go`

**Usage:** Use this when a process doesn't trigger cooking status to see if it enters State=R at all

### 4. `cmd/test-workload/`
**Workload detection test** - Tests CPU spike detection:
- Establishes baseline
- Sends commands to trigger work
- Monitors for CPU spikes above threshold
- Shows max CPU observed vs thresholds

**Run:** `go run cmd/test-workload/main.go`

## Threshold Values

The tests use these thresholds:

- **Baseline**: Rolling average CPU during `awaiting_input` state
- **Cooking Trigger**:
  - CPU > baseline + 5% OR CPU > baseline × 1.3, OR
  - State = "R" AND CPU > baseline + 0.5%
- **Awaiting Trigger**: CPU ≤ baseline + 1% (for 3 consecutive samples)

## Why These Tests Matter

**The OpenCode Problem:**
- OpenCode baseline: ~8% CPU (UI updates)
- When working: Enters State=R but CPU only increases to ~8.2% (+0.2%)
- Old threshold (baseline + 2% = 10%): Never triggered!
- New threshold (baseline + 0.5% = 8.5%): Catches it immediately!

These tests verify the adaptive baseline system works with real-world agents.

## Quick Start

```bash
# Run the full integration test
./test.sh

# Test with real opencode (make sure opencode is running in a tmux window)
go run cmd/test-opencode/main.go

# Check if a process uses State=R
go run cmd/test-sensitivity/main.go

# Test CPU spike detection
go run cmd/test-workload/main.go
```