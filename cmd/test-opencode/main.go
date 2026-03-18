package main

import (
	"fmt"
	"os/exec"
	"time"

	"ide/internal/tmux"
)

// Test to verify that opencode triggers cooking status with fixed thresholds
// This test sends a command to opencode and monitors for State=R detection
func main() {
	sessionName := "ide-ide"
	windowName := "opencode"

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║     OpenCode Real-World Test                             ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Monitoring: %s:%s\n", sessionName, windowName)
	fmt.Println()
	fmt.Println("This test verifies that opencode correctly triggers 'cooking' status")
	fmt.Println("when you interact with it.")
	fmt.Println()
	fmt.Println("Thresholds:")
	fmt.Println("  • Cooking: CPU > baseline + 5% OR (State=R AND CPU > baseline + 0.5%)")
	fmt.Println("  • Awaiting: CPU ≤ baseline + 1%")
	fmt.Println()
	fmt.Println("With ~8% baseline:")
	fmt.Println("  • Cooking at: CPU > 13% OR (State=R AND CPU > 8.5%)")
	fmt.Println()

	// Give user time to read
	fmt.Println("Starting in 3 seconds...")
	time.Sleep(3 * time.Second)

	// Send a command to trigger work
	fmt.Println("🔥 Sending command to opencode: 'list all files'")
	cmd := exec.Command("tmux", "send-keys", "-t", sessionName+":"+windowName,
		"list all files in the current directory", "Enter")
	cmd.Run()
	fmt.Println()

	// Monitor
	fmt.Println("📊 Monitoring for 10 seconds...")
	fmt.Println()

	baselineCPU := 0.0
	sampleCount := 0
	status := "awaiting_input"
	lowCount := 0
	cookingDetected := false

	for i := 0; i < 20; i++ {
		info, err := tmux.GetPaneProcessInfo(sessionName, windowName)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Update baseline when awaiting
		if status == "awaiting_input" {
			if sampleCount == 0 {
				baselineCPU = info.CPU
			} else if sampleCount < 20 {
				baselineCPU = (baselineCPU*float64(sampleCount) + info.CPU) / float64(sampleCount+1)
			} else {
				baselineCPU = baselineCPU*0.9 + info.CPU*0.1
			}
			sampleCount++
		}

		// Calculate thresholds
		effectiveBaseline := baselineCPU
		if effectiveBaseline < 1.0 {
			effectiveBaseline = 1.0
		}
		cookingThreshold := effectiveBaseline + 5.0
		if effectiveBaseline*1.3 > cookingThreshold {
			cookingThreshold = effectiveBaseline * 1.3
		}
		stateThreshold := effectiveBaseline + 0.5
		lowThreshold := effectiveBaseline + 1.0

		// Detect status
		oldStatus := status
		cpuElevated := info.CPU > cookingThreshold
		stateWork := info.State == "R" && info.CPU > stateThreshold
		isCooking := cpuElevated || stateWork
		isLow := info.CPU <= lowThreshold

		if status == "cooking" {
			if isCooking {
				lowCount = 0
			} else if isLow {
				lowCount++
				if lowCount >= 3 {
					status = "awaiting_input"
					lowCount = 0
				}
			}
		} else {
			if isCooking {
				status = "cooking"
				lowCount = 0
			}
		}

		if status == "cooking" {
			cookingDetected = true
		}

		// Show status changes
		if status != oldStatus {
			icon := "🔵"
			if status == "cooking" {
				icon = "🟡"
			}
			reason := ""
			if cpuElevated && status == "cooking" {
				reason = fmt.Sprintf(" (CPU %.1f%% > %.1f%%)", info.CPU, cookingThreshold)
			} else if stateWork && status == "cooking" {
				reason = fmt.Sprintf(" (State=R, CPU %.1f%% > %.1f%%)", info.CPU, stateThreshold)
			}
			fmt.Printf("T+%2.1fs: %s %s%s\n", float64(i+1)*0.5, icon, status, reason)
		}

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      RESULTS                               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Baseline CPU: %.1f%%\n", baselineCPU)
	fmt.Println()

	if cookingDetected {
		fmt.Println("✅ SUCCESS! Cooking status was detected!")
		fmt.Println("   The feature is working correctly with opencode.")
	} else {
		fmt.Println("⚠️  Cooking status was NOT detected")
		fmt.Println()
		fmt.Println("This could mean:")
		fmt.Println("  1. Opencode finished processing before detection")
		fmt.Println("  2. The command didn't trigger any work")
		fmt.Println("  3. Try running this test again while actively using opencode")
	}
}
