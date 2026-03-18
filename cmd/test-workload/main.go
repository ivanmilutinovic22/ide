package main

import (
	"fmt"
	"os/exec"
	"time"

	"ide/internal/tmux"
)

// Test to verify workload detection with adaptive baseline
// This test triggers CPU-intensive work and verifies cooking detection
func main() {
	sessionName := "ide-ide"
	windowName := "opencode"

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║     Workload Detection Test                              ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Target: %s:%s\n", sessionName, windowName)
	fmt.Println()

	// Establish baseline
	fmt.Println("⏳ Establishing baseline (3 seconds)...")
	time.Sleep(3 * time.Second)

	info, _ := tmux.GetPaneProcessInfo(sessionName, windowName)
	baselineCPU := info.CPU
	fmt.Printf("📍 Baseline established: %.1f%%\n", baselineCPU)
	fmt.Println()

	// Calculate expected thresholds
	effectiveBaseline := baselineCPU
	if effectiveBaseline < 1.0 {
		effectiveBaseline = 1.0
	}
	cookingThreshold := effectiveBaseline + 5.0
	if effectiveBaseline*1.3 > cookingThreshold {
		cookingThreshold = effectiveBaseline * 1.3
	}
	stateThreshold := effectiveBaseline + 0.5

	fmt.Printf("Expected thresholds:\n")
	fmt.Printf("  • Cooking (CPU): > %.1f%%\n", cookingThreshold)
	fmt.Printf("  • Cooking (State): State=R AND CPU > %.1f%%\n", stateThreshold)
	fmt.Println()

	// Trigger work
	fmt.Println("🔥 Triggering work in opencode...")
	fmt.Println("   Sending: 'find all go files'")
	cmd := exec.Command("tmux", "send-keys", "-t", sessionName+":"+windowName,
		"find all go files in this project", "Enter")
	cmd.Run()
	fmt.Println()

	// Monitor for cooking
	fmt.Println("📊 Monitoring for cooking status...")
	fmt.Println()

	cookingDetected := false
	maxObservedCPU := baselineCPU
	var cookingReason string

	for i := 0; i < 20; i++ {
		info, err := tmux.GetPaneProcessInfo(sessionName, windowName)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if info.CPU > maxObservedCPU {
			maxObservedCPU = info.CPU
		}

		// Check cooking conditions
		cpuElevated := info.CPU > cookingThreshold
		stateIndicatingWork := info.State == "R" && info.CPU > stateThreshold

		if cpuElevated || stateIndicatingWork {
			cookingDetected = true
			if cpuElevated {
				cookingReason = fmt.Sprintf("CPU spike (%.1f%% > %.1f%%)", info.CPU, cookingThreshold)
			} else {
				cookingReason = fmt.Sprintf("State=R with elevated CPU (%.1f%% > %.1f%%)",
					info.CPU, stateThreshold)
			}
			fmt.Printf("T+%2.1fs: 🟡 COOKING - %s\n", float64(i+1)*0.5, cookingReason)
		}

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      RESULTS                               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Baseline CPU:     %.1f%%\n", baselineCPU)
	fmt.Printf("Max CPU observed: %.1f%%\n", maxObservedCPU)
	fmt.Printf("Cooking threshold: %.1f%%\n", cookingThreshold)
	fmt.Printf("State threshold:   %.1f%%\n", stateThreshold)
	fmt.Println()

	if cookingDetected {
		fmt.Println("✅ SUCCESS! Cooking status detected!")
		fmt.Printf("   Reason: %s\n", cookingReason)
		fmt.Println()
		fmt.Println("The adaptive baseline is working correctly.")
	} else {
		fmt.Println("❌ Cooking status NOT detected")
		fmt.Println()
		if maxObservedCPU <= baselineCPU+1.0 {
			fmt.Println("The workload didn't significantly increase CPU.")
			fmt.Println("Opencode may have processed the request too quickly.")
		} else {
			fmt.Printf("CPU increased to %.1f%% but didn't exceed threshold.\n", maxObservedCPU)
			fmt.Println("The thresholds might need adjustment for this workload.")
		}
	}
}
