package tmux

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"ide/internal/config"
)

func SessionName(envName string) string {
	clean := strings.TrimSpace(strings.ToLower(envName))
	clean = strings.ReplaceAll(clean, " ", "-")
	if clean == "" {
		return "ide"
	}
	return "ide-" + clean
}

// runTmux runs tmux with the given args and returns stdout. Errors that mean
// "nothing to report" — `no server running`, `can't find session` — are
// translated to (empty, nil) so callers can treat them as a benign empty result.
func runTmux(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		text := stderr.String()
		if strings.Contains(text, "no server running") || strings.Contains(text, "can't find session") {
			return "", nil
		}
		return "", err
	}
	return stdout.String(), nil
}

func splitNonEmptyLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func HasSession(session string) bool {
	return exec.Command("tmux", "has-session", "-t", session).Run() == nil
}

func ListSessions() ([]string, error) {
	out, err := runTmux("list-sessions", "-F", "#{session_name}")
	if err != nil {
		return nil, fmt.Errorf("list tmux sessions: %w", err)
	}
	return splitNonEmptyLines(out), nil
}

func KillSession(session string) error {
	if _, err := runTmux("kill-session", "-t", session); err != nil {
		return fmt.Errorf("kill tmux session %q: %w", session, err)
	}
	return nil
}

func ListWindows(session string) ([]string, error) {
	out, err := runTmux("list-windows", "-t", session, "-F", "#{window_name}")
	if err != nil {
		return nil, fmt.Errorf("list windows for %q: %w", session, err)
	}
	return splitNonEmptyLines(out), nil
}

// SessionsSnapshot is the result of one batched `tmux list-panes -a` call:
// every session, its windows, and the foreground command of each window's
// first pane — all in a single tmux subprocess instead of one per session
// plus one per window.
type SessionsSnapshot struct {
	Names    []string                     // session names, in tmux's default order
	Windows  map[string][]string          // window names per session
	Commands map[string]map[string]string // session -> window -> first-pane command
}

// ListSessionsSnapshot fetches every session/window/pane-command in one shot.
// Empty server (no tmux running) returns an empty snapshot with nil error.
func ListSessionsSnapshot() (SessionsSnapshot, error) {
	out, err := runTmux("list-panes", "-a", "-F", "#{session_name}\t#{window_name}\t#{pane_current_command}")
	if err != nil {
		return SessionsSnapshot{}, fmt.Errorf("list panes: %w", err)
	}
	snap := SessionsSnapshot{
		Windows:  map[string][]string{},
		Commands: map[string]map[string]string{},
	}
	seenSession := map[string]bool{}
	seenWindow := map[string]map[string]bool{}
	for _, line := range splitNonEmptyLines(out) {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		s, w, cmd := parts[0], parts[1], parts[2]
		if !seenSession[s] {
			seenSession[s] = true
			snap.Names = append(snap.Names, s)
			seenWindow[s] = map[string]bool{}
			snap.Commands[s] = map[string]string{}
		}
		if !seenWindow[s][w] {
			seenWindow[s][w] = true
			snap.Windows[s] = append(snap.Windows[s], w)
			// First pane of the window represents the window's foreground
			// command, matching what `display-message #{pane_current_command}`
			// returns for an unspecified pane target.
			snap.Commands[s][w] = cmd
		}
	}
	return snap, nil
}

func HasWindow(session, window string) (bool, error) {
	window = SafeWindowName(window)
	if window == "" {
		return true, nil
	}
	windows, err := ListWindows(session)
	if err != nil {
		return false, err
	}
	for _, w := range windows {
		if strings.TrimSpace(w) == window {
			return true, nil
		}
	}
	return false, nil
}

func EnsureSession(env config.Environment) error {
	session := SessionName(env.Name)
	log.Printf("EnsureSession: env=%q session=%q windows=%d", env.Name, session, len(env.Windows))

	if len(env.Windows) == 0 {
		log.Printf("EnsureSession: no windows defined, falling back to default shell window")
		env.Windows = []config.WindowTemplate{{Name: "shell"}}
	}

	first := env.Windows[0]
	firstName := SafeWindowName(first.Name)
	firstCwd := resolveCwd(env.Root, first.Cwd)

	args := []string{"new-session", "-d", "-s", session, "-n", firstName}
	if firstCwd != "" {
		args = append(args, "-c", firstCwd)
	}
	if firstCommand := startupCommand(first.Cmd); firstCommand != "" {
		args = append(args, firstCommand)
	}
	log.Printf("EnsureSession: creating session with first window %q cwd=%q cmd=%q args=%v", firstName, firstCwd, first.Cmd, args)
	cmd := exec.Command("tmux", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// tmux reports "duplicate session: NAME" when the session already exists; treat as no-op so this is race-free vs. concurrent creators.
		if strings.Contains(stderr.String(), "duplicate session") {
			log.Printf("EnsureSession: session %q already exists, skipping", session)
			return nil
		}
		log.Printf("EnsureSession: ERROR creating session %q: %v: %s", session, err, strings.TrimSpace(stderr.String()))
		return fmt.Errorf("create tmux session %q: %w", session, err)
	}
	log.Printf("EnsureSession: session %q created", session)

	for i, w := range env.Windows[1:] {
		name := SafeWindowName(w.Name)
		cwd := resolveCwd(env.Root, w.Cwd)
		args = []string{"new-window", "-t", session, "-n", name}
		if cwd != "" {
			args = append(args, "-c", cwd)
		}
		if command := startupCommand(w.Cmd); command != "" {
			args = append(args, command)
		}
		log.Printf("EnsureSession: creating window[%d] %q cwd=%q cmd=%q args=%v", i+1, name, cwd, w.Cmd, args)
		if err := exec.Command("tmux", args...).Run(); err != nil {
			log.Printf("EnsureSession: ERROR creating window %q: %v", name, err)
			return fmt.Errorf("create window %q: %w", name, err)
		}
		log.Printf("EnsureSession: window %q created", name)
	}

	// Bind prefix+a to open the search popup
	if exe, err := os.Executable(); err == nil {
		BindSearchKey(exe)
	}

	log.Printf("EnsureSession: done, session %q has %d windows", session, len(env.Windows))
	return nil
}

// BindSearchKey sets up a tmux keybinding (prefix + a) that opens the IDE
// search popup directly inside the tmux session using display-popup. A
// failure here is non-fatal — the rest of the session still works without
// the popup binding — but log it so users have a breadcrumb.
func BindSearchKey(ideBinary string) {
	cmd := exec.Command("tmux", "bind-key", "-T", "prefix", "a",
		"display-popup", "-E", "-w", "80%", "-h", "80%", ideBinary, "--search")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Printf("BindSearchKey: failed to bind prefix+a: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
}

func AttachTarget(env config.Environment, windowName string) string {
	session := SessionName(env.Name)
	if strings.TrimSpace(windowName) == "" {
		return session
	}
	return session + ":" + SafeWindowName(windowName)
}

func WindowNames(env config.Environment) []string {
	if len(env.Windows) == 0 {
		return []string{"shell"}
	}
	out := make([]string, 0, len(env.Windows))
	for _, w := range env.Windows {
		out = append(out, SafeWindowName(w.Name))
	}
	return out
}

func resolveCwd(root, override string) string {
	override = strings.TrimSpace(override)
	root = strings.TrimSpace(root)
	if override == "" {
		return root
	}
	if filepath.IsAbs(override) {
		return override
	}
	if root == "" {
		return override
	}
	return filepath.Join(root, override)
}

func SafeWindowName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "shell"
	}
	return strings.ReplaceAll(name, " ", "-")
}

func startupCommand(command string) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	script := strings.TrimSpace(command) + "; exec " + shell + " -i"
	return shell + " -lc " + shellQuote(script)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func CurrentProcess(session, window string) string {
	target := session + ":" + SafeWindowName(window)
	out, err := runTmux("display-message", "-p", "-t", target, "#{pane_current_command}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func CapturePane(session, window string) (string, error) {
	target := session + ":" + SafeWindowName(window)
	out, err := runTmux("capture-pane", "-p", "-e", "-t", target)
	if err != nil {
		return "", fmt.Errorf("capture pane %q: %w", target, err)
	}
	return out, nil
}

func CheckTmuxExists() error {
	_, err := exec.LookPath("tmux")
	if err != nil {
		return errors.New("tmux is not installed or not in PATH — install via `brew install tmux` (macOS) or `apt install tmux` (Debian/Ubuntu)")
	}
	return nil
}

// ProcessInfo contains process metrics for a pane
type ProcessInfo struct {
	PID   int
	CPU   float64
	State string
}

// GetPaneProcessInfo retrieves the current process info for a pane.
// It sums CPU usage across all descendant processes of the pane's shell,
// giving an accurate picture of total activity in the pane.
func GetPaneProcessInfo(session, window string) (ProcessInfo, error) {
	target := session + ":" + SafeWindowName(window)

	// Get pane PID (this is the shell process)
	cmd := exec.Command("tmux", "display-message", "-p", "-t", target, "#{pane_pid}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		log.Printf("[TMUX-DEBUG] Failed to get pane PID: %v", err)
		return ProcessInfo{}, err
	}
	pidStr := strings.TrimSpace(out.String())
	shellPID, err := strconv.Atoi(pidStr)
	if err != nil {
		log.Printf("[TMUX-DEBUG] Failed to parse PID %s: %v", pidStr, err)
		return ProcessInfo{}, err
	}

	// Sum CPU of ALL descendants (shell -> agent -> agent's subprocesses)
	// This captures total pane activity regardless of process tree shape
	totalCPU := sumDescendantCPU(shellPID)
	hasRunning := hasRunningDescendant(shellPID)

	state := "S"
	if hasRunning {
		state = "R"
	}

	log.Printf("[TMUX-DEBUG] Session=%s Window=%s ShellPID=%d totalCPU=%.2f state=%s",
		session, window, shellPID, totalCPU, state)

	return ProcessInfo{
		PID:   shellPID,
		CPU:   totalCPU,
		State: state,
	}, nil
}

// getChildPIDs returns the direct child PIDs of a process
func getChildPIDs(pid int) []int {
	cmd := exec.Command("pgrep", "-P", strconv.Itoa(pid))
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var pids []int
	for _, line := range lines {
		if p, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
			pids = append(pids, p)
		}
	}
	return pids
}

// sumDescendantCPU recursively sums CPU usage of all descendants of a process
func sumDescendantCPU(pid int) float64 {
	children := getChildPIDs(pid)
	var total float64
	for _, child := range children {
		cpu, _ := getProcessMetrics(child)
		total += cpu + sumDescendantCPU(child)
	}
	return total
}

// hasRunningDescendant checks if any descendant process is in Running state
func hasRunningDescendant(pid int) bool {
	children := getChildPIDs(pid)
	for _, child := range children {
		_, state := getProcessMetrics(child)
		if state == "R" {
			return true
		}
		if hasRunningDescendant(child) {
			return true
		}
	}
	return false
}

// getProcessMetrics retrieves CPU percentage and state for a process
func getProcessMetrics(pid int) (float64, string) {
	// Get CPU percentage and state using ps
	// Use "=" suffix on format specifiers to suppress headers (works on macOS and Linux)
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "%cpu=", "-o", "state=")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		log.Printf("[TMUX-DEBUG] getProcessMetrics: ps failed for PID %d: %v", pid, err)
		return 0, ""
	}

	line := strings.TrimSpace(out.String())
	parts := strings.Fields(line)
	if len(parts) < 2 {
		log.Printf("[TMUX-DEBUG] getProcessMetrics: unexpected ps output for PID %d: %q", pid, line)
		return 0, ""
	}

	cpu, _ := strconv.ParseFloat(parts[0], 64)
	state := parts[1]
	if len(state) > 0 {
		state = string(state[0]) // Just the first character (R, S, D, etc.)
	}

	return cpu, state
}
