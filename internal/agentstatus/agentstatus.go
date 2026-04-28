// Package agentstatus tracks AI-agent CLI processes inside tmux panes.
// It contains the pure detection logic — no UI, no tmux calls — so the ui
// layer can drive it from polling messages and unit tests can exercise the
// state machine directly.
package agentstatus

import (
	"strings"
	"time"
)

// Status represents the detected status of an AI agent in a window.
type Status string

const (
	StatusIdle          Status = "idle"
	StatusCooking       Status = "cooking"
	StatusAwaitingInput Status = "awaiting_input"
)

// ProcessInfo is a snapshot of process metrics used to drive status detection.
type ProcessInfo struct {
	PID       int
	CPU       float64
	State     string // ps state code: R, S, D, T, Z, etc.
	Timestamp time.Time
}

// WindowInfo is the per-window tracking state the ui layer keeps for each
// agent window. Detect reads/writes the activity counters and baseline.
type WindowInfo struct {
	Current          ProcessInfo
	Previous         ProcessInfo
	Status           Status
	LowActivityCount int     // consecutive samples below the cooking threshold
	BaselineCPU      float64 // rolling baseline CPU when in awaiting_input
	SampleCount      int     // samples accumulated for the baseline
	Command          string  // most recently observed pane_current_command
}

// KnownTools is the set of process names recognised as AI-agent CLIs.
// Lookup is case-insensitive; values are lowercase and stripped of any path
// or argument noise before comparison.
var KnownTools = map[string]struct{}{
	"claude":       {}, // Anthropic Claude Code (npm @anthropic-ai/claude-code)
	"opencode":     {}, // sst/opencode terminal AI agent
	"codex":        {}, // openai/codex CLI
	"aider":        {}, // Aider-AI/aider pair programmer
	"gemini":       {}, // google-gemini/gemini-cli
	"cursor-agent": {}, // Cursor CLI agent
	"copilot":      {}, // github/copilot-cli (npm @github/copilot)
	"crush":        {}, // charmbracelet/crush
	"goose":        {}, // block/goose AI agent
	"cn":           {}, // continuedev/continue CLI
	"jules":        {}, // Google Jules Tools CLI
	"mods":         {}, // charmbracelet/mods
	"llm":          {}, // simonw/llm
	"sgpt":         {}, // TheR1D/shell_gpt and tbckr/sgpt
	"shell-gpt":    {}, // TheR1D/shell_gpt alias
	"tgpt":         {}, // aandrew-me/tgpt
	"chatgpt":      {}, // j178/chatgpt and similar interactive CLIs
	"q":            {}, // Amazon Q Developer CLI (q chat)
}

// IsAITool reports whether name is a known AI-agent CLI. Accepts paths and
// argv with arguments — only the last path component up to the first space
// is checked, case-insensitively.
func IsAITool(name string) bool {
	if name == "" {
		return false
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if i := strings.IndexAny(name, " \t"); i >= 0 {
		name = name[:i]
	}
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	_, ok := KnownTools[name]
	return ok
}

// Key formats the canonical "session:window" tracking key.
// Nothing escapes colons, so callers must avoid colons inside session names.
func Key(session, window string) string {
	return session + ":" + window
}

// Detect drives the per-window status state machine. It returns the new
// (status, lowActivityCount, baselineCPU, sampleCount). The caller persists
// these back into its WindowInfo for the next sample.
//
// State machine:
//   - Cooking → AwaitingInput after 3 consecutive low-activity samples
//   - Awaiting/Idle → Cooking immediately on a high-activity sample;
//     baseline updates only on low-activity samples (so a cooking spike
//     doesn't poison the idle baseline)
//   - First 10 samples for a new window are baseline-only (stay Awaiting)
//
// Cooking threshold = max(baseline+5, baseline*1.3), with baseline floored at 1.0.
func Detect(current ProcessInfo, currentStatus Status, lowActivityCount int, baselineCPU float64, sampleCount int) (Status, int, float64, int) {
	effectiveBaseline := baselineCPU
	if effectiveBaseline < 1.0 {
		effectiveBaseline = 1.0
	}
	cookingThreshold := effectiveBaseline + 5.0
	if effectiveBaseline*1.3 > cookingThreshold {
		cookingThreshold = effectiveBaseline * 1.3
	}
	cpuElevated := current.CPU > cookingThreshold
	isHighActivity := cpuElevated
	isLowActivity := current.CPU <= cookingThreshold

	switch currentStatus {
	case StatusCooking:
		if isHighActivity {
			return StatusCooking, 0, baselineCPU, sampleCount
		}
		if isLowActivity {
			newCount := lowActivityCount + 1
			if newCount >= 3 {
				return StatusAwaitingInput, 0, baselineCPU, sampleCount
			}
			return StatusCooking, newCount, baselineCPU, sampleCount
		}
		return StatusCooking, lowActivityCount, baselineCPU, sampleCount

	case StatusAwaitingInput, StatusIdle:
		if isHighActivity {
			return StatusCooking, 0, baselineCPU, sampleCount
		}
		newBaseline := baselineCPU
		newSampleCount := sampleCount
		switch {
		case sampleCount == 0:
			newBaseline = current.CPU
			newSampleCount = 1
		case sampleCount < 20:
			newBaseline = (baselineCPU*float64(sampleCount) + current.CPU) / float64(sampleCount+1)
			newSampleCount = sampleCount + 1
		default:
			newBaseline = baselineCPU*0.9 + current.CPU*0.1
		}
		return StatusAwaitingInput, 0, newBaseline, newSampleCount

	default:
		// New window: collect 10 baseline samples before allowing cooking.
		if sampleCount < 10 {
			newBaseline := current.CPU
			newSampleCount := sampleCount + 1
			if sampleCount > 0 {
				newBaseline = (baselineCPU*float64(sampleCount) + current.CPU) / float64(sampleCount+1)
			}
			return StatusAwaitingInput, 0, newBaseline, newSampleCount
		}
		if isHighActivity {
			return StatusCooking, 0, baselineCPU, sampleCount
		}
		return StatusAwaitingInput, 0, current.CPU, 1
	}
}
