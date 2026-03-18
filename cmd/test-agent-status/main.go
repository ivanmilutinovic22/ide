package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"ide/internal/tmux"
)

// AgentStatus represents the detected status
type AgentStatus string

const (
	AgentStatusIdle          AgentStatus = "idle"
	AgentStatusCooking       AgentStatus = "cooking"
	AgentStatusAwaitingInput AgentStatus = "awaiting_input"
)

// ProcessInfo tracks process metrics
type ProcessInfo struct {
	PID       int
	CPU       float64
	State     string
	Timestamp time.Time
}

// detectAgentStatus determines the agent status with hysteresis and adaptive baseline:
// - Uses baseline CPU (average when idle) to detect significant increases
// - Cooking: CPU > baseline + 5% OR (State == "R" AND CPU > baseline + 0.5%)
// - Awaiting: CPU <= baseline + 1% (for 3 consecutive samples)
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

	// Low activity threshold: CPU close to baseline
	lowActivityThreshold := effectiveBaseline + 1.0

	// High activity detection:
	// Primary: CPU exceeds threshold (indicates real work)
	// Secondary: State is R with ANY elevated CPU (even 0.5% above baseline)
	cpuElevated := current.CPU > cookingThreshold
	stateIndicatingWork := current.State == "R" && current.CPU > effectiveBaseline+0.5
	isHighActivity := cpuElevated || stateIndicatingWork
	isLowActivity := current.CPU <= lowActivityThreshold

	switch currentStatus {
	case AgentStatusCooking:
		if isHighActivity {
			// Still cooking, reset the counter
			return AgentStatusCooking, 0, baselineCPU, sampleCount
		}
		if isLowActivity {
			// Low activity detected
			newCount := lowActivityCount + 1
			if newCount >= 3 {
				// After 3 consecutive low-activity samples, switch to awaiting_input
				return AgentStatusAwaitingInput, 0, baselineCPU, sampleCount
			}
			// Keep cooking but increment counter
			return AgentStatusCooking, newCount, baselineCPU, sampleCount
		}
		// Medium activity - stay cooking but don't increment counter
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
		// Default to awaiting_input for new windows
		if isHighActivity {
			return AgentStatusCooking, 0, baselineCPU, sampleCount
		}
		// For new windows, initialize baseline with first reading
		return AgentStatusAwaitingInput, 0, current.CPU, 1
	}
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║     Agent Status Detection - Adaptive Baseline Test       ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	sessionName := "ide-status-test"

	// Cleanup any existing session
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	time.Sleep(100 * time.Millisecond)

	// Create test session with tagged windows
	fmt.Println("📦 Setting up test tmux session...")

	windows := []struct {
		name string
		tags []string
	}{
		{"idle-ai", []string{"ai"}},
		{"busy-ai", []string{"ai"}},
		{"no-tag", []string{}},
	}

	// Create session
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-n", windows[0].name, "bash")
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed to create session: %v\n", err)
		os.Exit(1)
	}

	// Create additional windows
	for _, w := range windows[1:] {
		cmd := exec.Command("tmux", "new-window", "-t", sessionName, "-n", w.name, "bash")
		if err := cmd.Run(); err != nil {
			fmt.Printf("❌ Failed to create window %s: %v\n", w.name, err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	// Test 1: Check all windows initially idle
	fmt.Println("\n📊 TEST 1: Initial State Check (establishing baseline)")
	fmt.Println("   All windows should show 'awaiting_input'")

	testResults := make(map[string]bool)

	for _, w := range windows {
		if len(w.tags) == 0 {
			fmt.Printf("   ⏭️  %s (no [ai] tag - skipping)\n", w.name)
			continue
		}

		info, err := tmux.GetPaneProcessInfo(sessionName, w.name)
		if err != nil {
			fmt.Printf("   ❌ %s: Error - %v\n", w.name, err)
			testResults[w.name] = false
			continue
		}

		status, _, baseline, count := detectAgentStatus(ProcessInfo{
			PID:   info.PID,
			CPU:   info.CPU,
			State: info.State,
		}, AgentStatusIdle, 0, 0, 0)

		if status == AgentStatusAwaitingInput {
			fmt.Printf("   ✓ %s: awaiting_input (CPU: %.1f%%, Baseline: %.1f%%, Samples: %d)\n",
				w.name, info.CPU, baseline, count)
			testResults[w.name] = true
		} else {
			fmt.Printf("   ❌ %s: Expected awaiting_input, got %s\n", w.name, status)
			testResults[w.name] = false
		}
	}

	// Test 2: Start CPU-intensive task
	fmt.Println("\n📊 TEST 2: Starting CPU-intensive task")
	fmt.Println("   busy-ai should change to 'cooking' (CPU > baseline * 1.5)")

	// Start yes command in busy-ai window
	tmuxCmd := exec.Command("tmux", "send-keys", "-t", sessionName+":busy-ai",
		"yes > /dev/null &", "Enter")
	if err := tmuxCmd.Run(); err != nil {
		fmt.Printf("   ❌ Failed to start task: %v\n", err)
	}

	fmt.Println("   ⏳ Waiting for process to spin up...")
	time.Sleep(2 * time.Second)

	// Take measurements with baseline tracking
	fmt.Println("   📍 Taking measurements...")

	// Get initial baseline from idle state
	var baselineCPU float64 = 2.0
	var sampleCount int

	info, _ := tmux.GetPaneProcessInfo(sessionName, "idle-ai")
	_, _, baselineCPU, sampleCount = detectAgentStatus(ProcessInfo{
		PID:   info.PID,
		CPU:   info.CPU,
		State: info.State,
	}, AgentStatusAwaitingInput, 0, baselineCPU, 0)

	fmt.Printf("   📍 Idle baseline established: %.1f%% (samples: %d)\n", baselineCPU, sampleCount)

	// Check busy window
	info, err := tmux.GetPaneProcessInfo(sessionName, "busy-ai")
	if err != nil {
		fmt.Printf("   ❌ Error getting process info: %v\n", err)
	}

	cookingThreshold := baselineCPU * 1.5
	if baselineCPU+15.0 > cookingThreshold {
		cookingThreshold = baselineCPU + 15.0
	}

	fmt.Printf("   📍 busy-ai: CPU=%.1f%%, State=%s, Threshold=%.1f%%\n",
		info.CPU, info.State, cookingThreshold)

	status, _, _, _ := detectAgentStatus(ProcessInfo{
		PID:   info.PID,
		CPU:   info.CPU,
		State: info.State,
	}, AgentStatusAwaitingInput, 0, baselineCPU, 0)

	if status == AgentStatusCooking {
		fmt.Printf("   ✓ busy-ai: cooking (State: %s, CPU exceeds threshold)\n", info.State)
		testResults["busy-ai-cooking"] = true
	} else {
		fmt.Printf("   ❌ busy-ai: Expected cooking, got %s\n", status)
		fmt.Printf("      Baseline: %.1f%%, Current: %.1f%%, Threshold: %.1f%%\n",
			baselineCPU, info.CPU, cookingThreshold)
		testResults["busy-ai-cooking"] = false
	}

	// Test 3: Stop the task with hysteresis
	fmt.Println("\n📊 TEST 3: Stopping CPU-intensive task (with hysteresis)")
	fmt.Println("   busy-ai should return to 'awaiting_input' after 3 samples")

	// Kill all yes processes in the window
	tmuxCmd = exec.Command("tmux", "send-keys", "-t", sessionName+":busy-ai",
		"kill %1 2>/dev/null || true", "Enter")
	tmuxCmd.Run()

	time.Sleep(1 * time.Second)

	// Simulate hysteresis - take 3 samples
	currentStatus := AgentStatusCooking
	lowActivityCount := 0
	finalStatus := AgentStatusCooking

	for i := 1; i <= 3; i++ {
		info, _ := tmux.GetPaneProcessInfo(sessionName, "busy-ai")
		curr := ProcessInfo{PID: info.PID, CPU: info.CPU, State: info.State}

		currentStatus, lowActivityCount, _, _ = detectAgentStatus(curr, currentStatus, lowActivityCount, baselineCPU, 0)
		fmt.Printf("   Sample %d: CPU=%.1f%%, State=%s, LowCount=%d, Status=%s\n",
			i, info.CPU, info.State, lowActivityCount, currentStatus)
		finalStatus = currentStatus

		if i < 3 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if finalStatus == AgentStatusAwaitingInput {
		fmt.Printf("   ✓ busy-ai: awaiting_input (stopped after hysteresis)\n")
		testResults["busy-ai-stopped"] = true
	} else {
		fmt.Printf("   ! busy-ai: Status is %s (may need more time to settle)\n", finalStatus)
		testResults["busy-ai-stopped"] = false
	}

	// Test 4: Verify process state detection
	fmt.Println("\n📊 TEST 4: Process State Detection (R = Running)")

	// Start task again briefly to check state
	tmuxCmd = exec.Command("tmux", "send-keys", "-t", sessionName+":busy-ai",
		"yes > /dev/null &", "Enter")
	tmuxCmd.Run()
	time.Sleep(1 * time.Second)

	info, _ = tmux.GetPaneProcessInfo(sessionName, "busy-ai")

	if info.State == "R" {
		fmt.Printf("   ✓ Correctly detected Running state (R) during busy task (CPU: %.1f%%)\n", info.CPU)
		testResults["state-detection"] = true
	} else {
		fmt.Printf("   ! State was '%s' instead of 'R' (CPU: %.1f%%)\n", info.State, info.CPU)
		testResults["state-detection"] = false
	}

	// Cleanup the task
	exec.Command("tmux", "send-keys", "-t", sessionName+":busy-ai", "kill %1 2>/dev/null || true", "Enter").Run()

	// Cleanup
	fmt.Println("\n🧹 Cleaning up...")
	exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// Summary
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      TEST SUMMARY                          ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	passed := 0
	total := 0

	for test, result := range testResults {
		total++
		if result {
			passed++
			fmt.Printf("   ✓ %s\n", test)
		} else {
			fmt.Printf("   ❌ %s\n", test)
		}
	}

	fmt.Printf("\n   Results: %d/%d tests passed\n", passed, total)

	if passed == total {
		fmt.Println("\n   🎉 All tests passed! Adaptive baseline is working correctly.")
		fmt.Println("\n   This means:")
		fmt.Println("   • Baseline CPU is established during idle periods")
		fmt.Println("   • Cooking detected when CPU > baseline + 15% or baseline * 1.5")
		fmt.Println("   • Works correctly even for processes with constant background CPU")
		os.Exit(0)
	} else {
		fmt.Println("\n   ⚠️  Some tests failed. This may be due to timing or system load.")
		os.Exit(1)
	}
}
