package ui

import (
	"fmt"
	"log"
	"time"

	"ide/internal/agentstatus"
	"ide/internal/config"
	"ide/internal/tmux"
)

// isAIToolProcess and windowKey are thin shims around the agentstatus
// package so existing call sites in this package don't need to be rewritten.
func isAIToolProcess(name string) bool      { return agentstatus.IsAITool(name) }
func windowKey(session, window string) string { return agentstatus.Key(session, window) }
func detectAgentStatus(current ProcessInfo, currentStatus AgentStatus, lowActivityCount int, baselineCPU float64, sampleCount int) (AgentStatus, int, float64, int) {
	return agentstatus.Detect(current, currentStatus, lowActivityCount, baselineCPU, sampleCount)
}

// isAIWindow reports whether the window should be tracked as an AI agent
// window — either because the template has the [ai] tag or because its
// current foreground process is a known AI CLI.
func (m Model) isAIWindow(env config.Environment, windowName, currentProcess string) bool {
	if tmpl, ok := findWindowTemplate(env, windowName); ok && HasTag(tmpl, "ai") {
		return true
	}
	return isAIToolProcess(currentProcess)
}

// getWindowAgentStatus returns the current agent status for a window
// Only returns non-idle status if window has [ai] tag or its current
// foreground process is a known AI CLI.
func (m Model) getWindowAgentStatus(session, window string) AgentStatus {
	log.Printf("[getWindowAgentStatus] session=%s window=%s", session, window)

	env, ok := m.currentEnv()
	if !ok {
		log.Printf("[getWindowAgentStatus] No current environment, returning idle")
		return AgentStatusIdle
	}

	key := windowKey(session, window)
	info, hasInfo := m.windowProcessInfo[key]
	currentProcess := ""
	if hasInfo {
		currentProcess = info.Command
	}

	if !m.isAIWindow(env, window, currentProcess) {
		return AgentStatusIdle
	}

	if hasInfo {
		log.Printf("[getWindowAgentStatus] Returning tracked status: %s for key=%s", info.Status, key)
		return info.Status
	}
	log.Printf("[getWindowAgentStatus] No tracking info for key=%s, returning idle", key)
	return AgentStatusIdle
}

// getWindowStatusColor returns the color for a given agent status
func (m Model) getWindowStatusColor(status AgentStatus) string {
	theme := m.currentTheme()
	switch status {
	case AgentStatusCooking:
		return "#fbbf24" // Amber/yellow for cooking
	case AgentStatusAwaitingInput:
		return "#22d3ee" // Cyan for awaiting input
	default:
		return theme.Muted
	}
}

// jumpToNextCookingSession cycles to the next/previous session that has an active AI agent.
func (m *Model) jumpToNextCookingSession(direction int) {
	if len(m.environments) == 0 {
		return
	}
	start := m.selectedEnv
	for i := 1; i <= len(m.environments); i++ {
		idx := (start + i*direction + len(m.environments)*2) % len(m.environments)
		env := m.environments[idx]
		session := tmux.SessionName(env.Name)
		if _, running := m.sessions[session]; !running {
			continue
		}
		status := m.getSessionAgentStatus(env)
		if status == AgentStatusCooking || status == AgentStatusAwaitingInput {
			m.selectedEnv = idx
			m.selectedWindow = 0
			m.focusPane = focusPaneEnvironments
			statusLabel := "Cooking"
			if status == AgentStatusAwaitingInput {
				statusLabel = "Awaiting Input"
			}
			m.status = fmt.Sprintf("Jumped to %s (%s)", env.Name, statusLabel)
			return
		}
	}
	m.status = "No sessions with active AI agents found."
}

// windowNamesForEnv returns window names for a given environment, preferring live session windows.
func (m Model) windowNamesForEnv(env config.Environment) []string {
	session := tmux.SessionName(env.Name)
	if windows, ok := m.sessionWindows[session]; ok && len(windows) > 0 {
		return windows
	}
	return tmux.WindowNames(env)
}

// getSessionAgentStatus returns the highest-priority agent status across all windows of a session.
// Priority: Cooking > AwaitingInput > Idle
func (m Model) getSessionAgentStatus(env config.Environment) AgentStatus {
	session := tmux.SessionName(env.Name)
	windows := m.windowNamesForEnv(env)
	highest := AgentStatusIdle
	for _, wName := range windows {
		key := windowKey(session, wName)
		info, hasInfo := m.windowProcessInfo[key]
		currentProcess := ""
		if hasInfo {
			currentProcess = info.Command
		}
		if !m.isAIWindow(env, wName, currentProcess) {
			continue
		}
		if hasInfo {
			if info.Status == AgentStatusCooking {
				return AgentStatusCooking
			}
			if info.Status == AgentStatusAwaitingInput {
				highest = AgentStatusAwaitingInput
			}
		}
	}
	return highest
}

// updateWindowProcessInfo updates the process info and status for a window
func (m *Model) updateWindowProcessInfo(session, window string) {
	log.Printf("[updateWindowProcessInfo] Starting check for session=%s window=%s", session, window)
	env, ok := m.currentEnv()
	if !ok {
		return
	}

	currentCmd := tmux.CurrentProcess(session, window)
	if !m.isAIWindow(env, window, currentCmd) {
		return
	}

	// Get current process info from tmux
	procInfo, err := tmux.GetPaneProcessInfo(session, window)
	if err != nil {
		return
	}

	log.Printf("[updateWindowProcessInfo] Got procInfo: PID=%d CPU=%.2f State=%s", procInfo.PID, procInfo.CPU, procInfo.State)

	key := windowKey(session, window)
	current := ProcessInfo{
		PID:       procInfo.PID,
		CPU:       procInfo.CPU,
		State:     procInfo.State,
		Timestamp: time.Now(),
	}

	// Get previous info and current tracking state
	var previous ProcessInfo
	var currentStatus AgentStatus
	var lowActivityCount int
	var baselineCPU float64
	var sampleCount int
	if existing, ok := m.windowProcessInfo[key]; ok {
		previous = existing.Current
		currentStatus = existing.Status
		lowActivityCount = existing.LowActivityCount
		baselineCPU = existing.BaselineCPU
		sampleCount = existing.SampleCount
	}

	log.Printf("[updateWindowProcessInfo] Current tracking state: status=%s baselineCPU=%.2f sampleCount=%d", currentStatus, baselineCPU, sampleCount)

	// Detect status with hysteresis and adaptive baseline
	status, newLowActivityCount, newBaselineCPU, newSampleCount := detectAgentStatus(
		current, currentStatus, lowActivityCount, baselineCPU, sampleCount,
	)

	log.Printf("[updateWindowProcessInfo] Detected new status: %s", status)

	// Update tracking
	m.windowProcessInfo[key] = WindowProcessInfo{
		Current:          current,
		Previous:         previous,
		Status:           status,
		LowActivityCount: newLowActivityCount,
		BaselineCPU:      newBaselineCPU,
		SampleCount:      newSampleCount,
		Command:          currentCmd,
	}
}

// updateWindowProcessInfoFromMsg updates the process info from an agentStatusUpdateMsg
// This should be called from the Update method when receiving agentStatusUpdateMsg
func (m *Model) updateWindowProcessInfoFromMsg(session, window string, procInfo ProcessInfo, command string) {
	log.Printf("[updateWindowProcessInfoFromMsg] Processing msg for session=%s window=%s", session, window)

	key := windowKey(session, window)

	// Get previous info and current tracking state
	var previous ProcessInfo
	var currentStatus AgentStatus
	var lowActivityCount int
	var baselineCPU float64
	var sampleCount int
	previousCommand := ""
	if existing, ok := m.windowProcessInfo[key]; ok {
		previous = existing.Current
		currentStatus = existing.Status
		lowActivityCount = existing.LowActivityCount
		baselineCPU = existing.BaselineCPU
		sampleCount = existing.SampleCount
		previousCommand = existing.Command
	}

	log.Printf("[updateWindowProcessInfoFromMsg] Current tracking: status=%s baseline=%.2f samples=%d",
		currentStatus, baselineCPU, sampleCount)

	// Detect status with hysteresis and adaptive baseline
	status, newLowActivityCount, newBaselineCPU, newSampleCount := detectAgentStatus(
		procInfo, currentStatus, lowActivityCount, baselineCPU, sampleCount,
	)

	log.Printf("[updateWindowProcessInfoFromMsg] New status: %s baseline=%.2f", status, newBaselineCPU)

	if command == "" {
		command = previousCommand
	}

	// Update tracking
	m.windowProcessInfo[key] = WindowProcessInfo{
		Current:          procInfo,
		Previous:         previous,
		Status:           status,
		LowActivityCount: newLowActivityCount,
		BaselineCPU:      newBaselineCPU,
		SampleCount:      newSampleCount,
		Command:          command,
	}
}

// formatWindowLabel formats a window name with its status suffix
func (m Model) formatWindowLabel(name string, status AgentStatus) string {
	switch status {
	case AgentStatusCooking:
		return name + " ●"
	case AgentStatusAwaitingInput:
		return name + " ◆"
	default:
		return name
	}
}
