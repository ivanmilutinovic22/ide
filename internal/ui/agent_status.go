package ui

import (
	"fmt"
	"log"
	"time"

	"ide/internal/config"
	"ide/internal/tmux"
)

// Agent status detection functions

// detectAgentStatus determines the agent status with hysteresis and adaptive baseline:
// - Uses baseline CPU (average when idle) to detect significant increases
// - Cooking: State == "R" OR CPU > baseline + 15% OR CPU > baseline * 1.5
// - Awaiting: State != "R" AND CPU <= baseline + 10%
// - Baseline is continuously updated when in awaiting_input state
func detectAgentStatus(current ProcessInfo, currentStatus AgentStatus, lowActivityCount int, baselineCPU float64, sampleCount int) (AgentStatus, int, float64, int) {
	// Calculate thresholds using ORIGINAL baseline (before any updates)
	// This prevents high CPU readings from corrupting the baseline before cooking detection
	effectiveBaseline := baselineCPU
	if effectiveBaseline < 1.0 {
		effectiveBaseline = 1.0
	}

	// Cooking threshold: CPU must be significantly above baseline
	// Use +5% absolute or 1.3x multiplier, whichever gives lower threshold
	cookingThreshold := effectiveBaseline + 5.0
	if effectiveBaseline*1.3 > cookingThreshold {
		cookingThreshold = effectiveBaseline * 1.3
	}

	// High activity detection:
	// Primary: CPU exceeds threshold (indicates real work)
	// Agents like opencode use 10-18% CPU when actually working vs 4-5% when idle
	// Threshold of baseline + 5% reliably catches this (9% when baseline is 4%)
	cpuElevated := current.CPU > cookingThreshold
	isHighActivity := cpuElevated
	// To exit cooking state, we need CPU to drop below threshold
	// This creates natural hysteresis without a dead zone
	isLowActivity := current.CPU <= cookingThreshold

	log.Printf("[AGENT-DEBUG] detectAgentStatus: CPU=%.1f, State=%s, baseline=%.1f, threshold=%.1f, currentStatus=%s, sampleCount=%d",
		current.CPU, current.State, baselineCPU, cookingThreshold, currentStatus, sampleCount)

	switch currentStatus {
	case AgentStatusCooking:
		if isHighActivity {
			log.Printf("[AGENT-DEBUG] Still cooking (high activity)")
			return AgentStatusCooking, 0, baselineCPU, sampleCount
		}
		if isLowActivity {
			newCount := lowActivityCount + 1
			log.Printf("[AGENT-DEBUG] Low activity detected, count=%d/3", newCount)
			if newCount >= 3 {
				log.Printf("[AGENT-DEBUG] Switching to awaiting_input")
				return AgentStatusAwaitingInput, 0, baselineCPU, sampleCount
			}
			return AgentStatusCooking, newCount, baselineCPU, sampleCount
		}
		return AgentStatusCooking, lowActivityCount, baselineCPU, sampleCount

	case AgentStatusAwaitingInput, AgentStatusIdle:
		if isHighActivity {
			// Switch to cooking immediately, do NOT update baseline with high CPU reading
			return AgentStatusCooking, 0, baselineCPU, sampleCount
		}
		// Stay awaiting_input and update baseline with this (low) CPU reading
		newBaseline := baselineCPU
		newSampleCount := sampleCount
		if sampleCount == 0 {
			newBaseline = current.CPU
			newSampleCount = 1
		} else if sampleCount < 20 {
			// Build up initial baseline (first 20 samples)
			newBaseline = (baselineCPU*float64(sampleCount) + current.CPU) / float64(sampleCount+1)
			newSampleCount = sampleCount + 1
		} else {
			// Rolling average with more weight on recent samples
			newBaseline = baselineCPU*0.9 + current.CPU*0.1
			newSampleCount = sampleCount
		}
		return AgentStatusAwaitingInput, 0, newBaseline, newSampleCount

	default:
		// For new windows, start with awaiting_input and build baseline
		// Don't immediately assume cooking on first high reading
		// Collect samples first to establish proper baseline
		log.Printf("[AGENT-DEBUG] New window, sampleCount=%d", sampleCount)
		if sampleCount < 10 {
			// First 10 samples: just collect baseline, stay in awaiting
			newBaseline := current.CPU
			newSampleCount := sampleCount + 1
			if sampleCount > 0 {
				newBaseline = (baselineCPU*float64(sampleCount) + current.CPU) / float64(sampleCount+1)
			}
			log.Printf("[AGENT-DEBUG] Initializing sample %d, baseline=%.1f", newSampleCount, newBaseline)
			return AgentStatusAwaitingInput, 0, newBaseline, newSampleCount
		}

		// After 10 samples, use normal logic
		if isHighActivity {
			log.Printf("[AGENT-DEBUG] High activity detected after init, switching to cooking")
			return AgentStatusCooking, 0, baselineCPU, sampleCount
		}
		return AgentStatusAwaitingInput, 0, current.CPU, 1
	}
}

// windowKey creates a unique key for a window
func windowKey(session, window string) string {
	return session + ":" + window
}

// getWindowAgentStatus returns the current agent status for a window
// Only returns non-idle status if window has [ai] tag
func (m Model) getWindowAgentStatus(session, window string) AgentStatus {
	log.Printf("[getWindowAgentStatus] session=%s window=%s", session, window)

	env, ok := m.currentEnv()
	if !ok {
		log.Printf("[getWindowAgentStatus] No current environment, returning idle")
		return AgentStatusIdle
	}

	// Find the window template to check for [ai] tag
	windowIdx := -1
	windows := m.currentWindowNames()
	log.Printf("[getWindowAgentStatus] env=%s windows=%v", env.Name, windows)
	for i, w := range windows {
		if w == window {
			windowIdx = i
			break
		}
	}

	if windowIdx < 0 || windowIdx >= len(env.Windows) {
		log.Printf("[getWindowAgentStatus] Window %s not found in config (idx=%d, len=%d), returning idle", window, windowIdx, len(env.Windows))
		return AgentStatusIdle
	}

	// Check if window has [ai] tag
	winTemplate := env.Windows[windowIdx]
	hasAITag := HasTag(winTemplate, "ai")
	log.Printf("[getWindowAgentStatus] Found window %s, tags=%v, has_ai=%v", window, winTemplate.Tags, hasAITag)
	if !hasAITag {
		return AgentStatusIdle
	}

	// Return tracked status
	key := windowKey(session, window)
	if info, ok := m.windowProcessInfo[key]; ok {
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
		tmpl, ok := findWindowTemplate(env, wName)
		if !ok || !HasTag(tmpl, "ai") {
			continue
		}
		key := windowKey(session, wName)
		if info, ok := m.windowProcessInfo[key]; ok {
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

	// Find the window template to check for [ai] tag
	windowIdx := -1
	windows := m.currentWindowNames()
	for i, w := range windows {
		if w == window {
			windowIdx = i
			break
		}
	}

	if windowIdx < 0 || windowIdx >= len(env.Windows) {
		return
	}

	// Only track if window has [ai] tag
	winTemplate := env.Windows[windowIdx]
	if !HasTag(winTemplate, "ai") {
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
	}
}

// updateWindowProcessInfoFromMsg updates the process info from an agentStatusUpdateMsg
// This should be called from the Update method when receiving agentStatusUpdateMsg
func (m *Model) updateWindowProcessInfoFromMsg(session, window string, procInfo ProcessInfo) {
	log.Printf("[updateWindowProcessInfoFromMsg] Processing msg for session=%s window=%s", session, window)

	key := windowKey(session, window)

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

	log.Printf("[updateWindowProcessInfoFromMsg] Current tracking: status=%s baseline=%.2f samples=%d",
		currentStatus, baselineCPU, sampleCount)

	// Detect status with hysteresis and adaptive baseline
	status, newLowActivityCount, newBaselineCPU, newSampleCount := detectAgentStatus(
		procInfo, currentStatus, lowActivityCount, baselineCPU, sampleCount,
	)

	log.Printf("[updateWindowProcessInfoFromMsg] New status: %s baseline=%.2f", status, newBaselineCPU)

	// Update tracking
	m.windowProcessInfo[key] = WindowProcessInfo{
		Current:          procInfo,
		Previous:         previous,
		Status:           status,
		LowActivityCount: newLowActivityCount,
		BaselineCPU:      newBaselineCPU,
		SampleCount:      newSampleCount,
	}
}

// formatWindowLabel formats a window name with its status suffix
func (m Model) formatWindowLabel(name string, status AgentStatus) string {
	switch status {
	case AgentStatusCooking:
		return name + " ● Cooking"
	case AgentStatusAwaitingInput:
		return name + " ◆ Awaiting Input"
	default:
		return name
	}
}
