# AI Agent Status Detection Fix Summary

## Problem
The AI agent status detection was always showing "awaiting_input" because the model state was not being persisted. The status updates were happening inside a `tea.Cmd` function, which runs asynchronously and receives the model by value - meaning any changes to `m.windowProcessInfo` were lost.

## Root Cause
In Bubble Tea, commands (`tea.Cmd`) are executed asynchronously and cannot modify model state:

```go
// WRONG - This doesn't work!
func (m Model) captureCurrentWindowCmd() tea.Cmd {
    return func() tea.Msg {
        // This runs in a separate goroutine!
        m.updateWindowProcessInfo(session, window)  // Changes LOST!
        return capturePaneMsg{...}
    }
}
```

## Solution
Restructured to use proper message-passing architecture:

1. **Added new message type** (`internal/ui/model.go:262`):
```go
type agentStatusUpdateMsg struct {
    session  string
    window   string
    procInfo ProcessInfo
}
```

2. **Created status check command** (`internal/ui/model.go:3663`):
```go
func checkAgentStatusCmd(session, window string) tea.Cmd {
    return func() tea.Msg {
        procInfo, err := tmux.GetPaneProcessInfo(session, window)
        if err != nil {
            return nil
        }
        return agentStatusUpdateMsg{
            session: session,
            window:  window,
            procInfo: ProcessInfo{...},
        }
    }
}
```

3. **Added message handler in Update** (`internal/ui/model.go:1028`):
```go
case agentStatusUpdateMsg:
    log.Printf("[Update] Received agentStatusUpdateMsg...")
    m.updateWindowProcessInfoFromMsg(msg.session, msg.window, msg.procInfo)
    return m, nil
```

4. **Created proper update function** (`internal/ui/model.go:3614`):
```go
func (m *Model) updateWindowProcessInfoFromMsg(session, window string, procInfo ProcessInfo) {
    // Updates m.windowProcessInfo directly - changes persist!
}
```

## Debug Logging Added

Added comprehensive logging to trace the flow:

- `tmux.go:314-337` - Logs process detection (pgrep results, foreground PID)
- `model.go:3478-3482` - Logs getWindowAgentStatus lookups
- `model.go:3569` - Logs process info received from tmux
- `model.go:3593` - Logs current tracking state
- `model.go:3600` - Logs status detection results
- `model.go:3616-3637` - Logs message processing in updateWindowProcessInfoFromMsg

## Testing

Run the integration test:
```bash
go run cmd/test-agent-status/main.go
```

All 5 tests pass:
- ✓ Idle detection (baseline establishment)
- ✓ Cooking detection (CPU spike detection)
- ✓ Hysteresis (3-sample requirement)
- ✓ Process state detection (Running vs Sleeping)
- ✓ Window tag filtering ([ai] tag check)

## How to Monitor in Real Usage

When running the IDE with opencode:

1. Launch IDE: `./ide`
2. Select environment with [ai] tagged window
3. Press Enter to create/attach session
4. Run opencode in the tagged window
5. Check logs: `tail -f /tmp/ide.log` (or wherever stderr is directed)

You'll see logs like:
```
[TMUX-DEBUG] Session=ide-test Window=opencode Pane PID=12345
[TMUX-DEBUG] Foreground process PID=12346 (shell PID=12345)
[TMUX-DEBUG] Using foreground process: PID=12346 Name=opencode
[TMUX-DEBUG] Process metrics: CPU=8.50 State=S
[Update] Received agentStatusUpdateMsg for session=ide-test window=opencode
[updateWindowProcessInfoFromMsg] New status: awaiting_input baseline=8.50
```

## Files Modified

- `internal/tmux/tmux.go` - Added debug logging to process detection
- `internal/ui/model.go` - Fixed message-passing architecture and added debug logging

## Key Changes in model.go

Lines modified/added:
- 262-267: Added `agentStatusUpdateMsg` struct
- 1028-1032: Added message handler in Update
- 3478-3510: Enhanced `getWindowAgentStatus` with logging
- 3614-3650: Added `updateWindowProcessInfoFromMsg` function
- 3627-3661: Fixed `captureCurrentWindowCmd` to use message pattern
- 3663-3681: Added `checkAgentStatusCmd` function
