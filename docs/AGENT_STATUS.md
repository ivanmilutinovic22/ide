# Agent Status Detection

This feature detects the activity status of AI agents (like Claude Code, opencode) running in tmux windows and displays visual indicators in the IDE.

## Features

- **Automatic Status Detection**: Monitors process state (Running/Sleeping) and CPU usage
- **Visual Indicators**: Color-coded window tabs with status suffixes
- **Hysteresis**: Prevents flickering by requiring 3 consecutive low-activity samples before switching from "cooking" to "awaiting_input"
- **Tag-based Filtering**: Only monitors windows with `[ai]` tag

## Status Types

| Status | Color | Suffix | Meaning |
|--------|-------|--------|---------|
| `cooking` | Amber 🟡 | `-cooking` | Agent is actively working (high CPU or Running state) |
| `awaiting_input` | Cyan 🔵 | `-awaiting_input` | Agent is idle/waiting for user input |
| `idle` | Default | (none) | Window has no `[ai]` tag, not monitored |

## Usage

### 1. Tag Your Windows

Add the `[ai]` tag to windows running AI agents:

**In environments.json:**
```json
{
  "environments": [
    {
      "name": "my-project",
      "root": "/path/to/project",
      "windows": [
        {
          "name": "editor",
          "cmd": "nvim ."
        },
        {
          "name": "claude",
          "cmd": "claude",
          "tags": ["ai"]
        },
        {
          "name": "opencode",
          "cmd": "opencode",
          "tags": ["ai", "main"]
        }
      ]
    }
  ]
}
```

**In window spec format:**
```
claude=claude [ai];editor=nvim .;opencode=opencode [ai][main]
```

### 2. Launch IDE

```bash
./ide
```

The IDE will automatically:
- Monitor all windows tagged with `[ai]`
- Display amber tabs when agents are working
- Display cyan tabs when agents are waiting for input
- Update status every 500ms

## How It Works

### Status Detection Logic

### Adaptive Baseline Approach

Instead of using fixed thresholds, the system establishes a **baseline CPU** during idle periods and detects relative increases:

1. **Baseline Establishment**:
   - Average CPU measured during `awaiting_input` state
   - Rolling average over first 20 samples, then 90/10 weighting
   - Minimum baseline of 2% to avoid division issues

2. **Cooking Detection** (immediate):
   - CPU > baseline + 5% OR CPU > baseline × 1.3, OR
   - State is "R" (Running) AND CPU > baseline + 0.5%

3. **Awaiting Input Detection** (with hysteresis):
   - CPU ≤ baseline + 1%
   - Must be true for **3 consecutive samples** (1.5 seconds)

### Why Adaptive?

Different agents have different baseline CPU usage:
- **OpenCode**: ~8% idle CPU (UI updates), enters State=R when working but only increases to ~8.5%
- **Claude**: ~3-4% idle CPU
- **Other tools**: Varies

Fixed thresholds (e.g., CPU > 5%) would show "cooking" even when idle.
Adaptive thresholds prevent false positives while still detecting real work.

### Why Ultra-Sensitive State Detection?

OpenCode was tricky because:
- Baseline: ~8% CPU
- When working: Enters State=R but CPU only increases to ~8.2%
- Old threshold required CPU > baseline + 2% (10% total) - never triggered!
- New threshold: CPU > baseline + 0.5% (8.5% total) - catches it immediately!

### Hysteresis Benefits

- Prevents status flickering during brief pauses
- Agents often have micro-pauses while working (network, disk I/O)
- Status only changes when truly idle for a sustained period

## Testing

Run the integration test:

```bash
# Quick test
./test.sh

# Or run directly
go run cmd/test-agent-status/main.go
```

This will:
1. Create a test tmux session
2. Verify initial `awaiting_input` state
3. Start a CPU-intensive task and verify `cooking` state
4. Stop the task and verify hysteresis behavior
5. Confirm proper cleanup

## Configuration Tips

### Multiple Tags
You can add multiple tags to a window:
```json
{
  "name": "ai-dev",
  "cmd": "claude",
  "tags": ["ai", "main", "dev"]
}
```

### Tag Placement in Spec
Tags come at the end of the window spec:
```
name=command|cwd [tag1][tag2]
```

Examples:
```
claude=claude [ai]
code=code .|~/project [ai][main]
nvim=nvim . [editor]
```

## Troubleshooting

### Status not showing
- Ensure window has `[ai]` tag
- Check that tmux session is running
- Verify process is actually running in the window (not just shell)

### Status flickering
- This is normal without hysteresis - it's been implemented to prevent this
- If still occurring, the threshold might need adjustment in the code

### Wrong status
- Process detection uses foreground process (not shell)
- Some commands spawn sub-processes which might not be detected
- Check with: `tmux display-message -p "#{pane_current_command}"`