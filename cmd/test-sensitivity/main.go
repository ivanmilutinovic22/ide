package main

import (
	"fmt"
	"os/exec"
	"time"

	"ide/internal/tmux"
)

// Ultra-sensitive test to detect if a process enters State=R
// This test uses State=R as the ONLY indicator (no CPU threshold)
// Useful for diagnosing processes that use State=R but don't spike CPU
func main() {
	sessionName := "ide-ide"
	windowName := "opencode"

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║     Ultra-Sensitive State=R Detection Test               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Monitoring: %s:%s\n", sessionName, windowName)
	fmt.Println()
	fmt.Println("This test uses ULTRA-SENSITIVE detection:")
	fmt.Println("  • ANY State=R = COOKING (regardless of CPU)")
	fmt.Println()
	fmt.Println("Purpose: Diagnose if a process enters State=R when working")
	fmt.Println("even if CPU doesn't spike significantly.")
	fmt.Println()

	// Send a command to trigger work
	fmt.Println("🔥 Sending command: 'what files are here?'")
	cmd := exec.Command("tmux", "send-keys", "-t", sessionName+":"+windowName,
		"what files are here", "Enter")
	cmd.Run()
	fmt.Println()

	// Monitor
	fmt.Println("📊 Monitoring (State=R will show as COOKING)...")
	fmt.Println()

	cookingCount := 0
	maxCPU := 0.0

	for i := 0; i < 20; i++ {
		info, err := tmux.GetPaneProcessInfo(sessionName, windowName)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if info.CPU > maxCPU {
			maxCPU = info.CPU
		}

		// Ultra-sensitive: State=R = cooking
		if info.State == "R" {
			cookingCount++
			fmt.Printf("T+%2.1fs: 🟡 COOKING (State: %s, CPU: %.1f%%)\n",
				float64(i+1)*0.5, info.State, info.CPU)
		}

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      RESULTS                               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("State=R detected: %d/20 times (%.0f%% of samples)\n",
		cookingCount, float64(cookingCount)/20*100)
	fmt.Printf("Max CPU observed: %.1f%%\n", maxCPU)
	fmt.Println()

	if cookingCount > 0 {
		fmt.Println("✅ Process DOES enter State=R when working!")
		fmt.Println()
		fmt.Println("DIAGNOSIS:")
		fmt.Println("  The process enters State=R but CPU stays near baseline.")
		fmt.Println("  Your CPU thresholds may be too high.")
		fmt.Println()
		fmt.Println("SOLUTION:")
		fmt.Println("  Lower the State+R CPU threshold from 'baseline + X%' to")
		fmt.Println("  'baseline + 0.5%' to catch these subtle activities.")
	} else {
		fmt.Println("❌ Process never entered State=R")
		fmt.Println()
		fmt.Println("This suggests:")
		fmt.Println("  • Process uses background threads (not foreground)")
		fmt.Println("  • Operations are async/non-blocking")
		fmt.Println("  • Process was already idle when tested")
	}
}
