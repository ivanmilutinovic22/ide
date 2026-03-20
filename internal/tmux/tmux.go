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

func HasSession(session string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", session)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func ListSessions() ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if strings.Contains(stderr.String(), "no server running") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("list tmux sessions: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	res := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		res = append(res, line)
	}
	return res, nil
}

func KillSession(session string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", session)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		text := stderr.String()
		if strings.Contains(text, "can't find session") || strings.Contains(text, "no server running") {
			return nil
		}
		return fmt.Errorf("kill tmux session %q: %w", session, err)
	}
	return nil
}

func ListWindows(session string) ([]string, error) {
	cmd := exec.Command("tmux", "list-windows", "-t", session, "-F", "#{window_name}")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		text := stderr.String()
		if strings.Contains(text, "can't find session") || strings.Contains(text, "no server running") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("list windows for %q: %w", session, err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	res := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		res = append(res, line)
	}
	return res, nil
}

func HasWindow(session, window string) (bool, error) {
	window = safeWindowName(window)
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

	if HasSession(session) {
		log.Printf("EnsureSession: session %q already exists, skipping", session)
		return nil
	}
	if len(env.Windows) == 0 {
		log.Printf("EnsureSession: no windows defined, falling back to default shell window")
		env.Windows = []config.WindowTemplate{{Name: "shell"}}
	}

	first := env.Windows[0]
	firstName := safeWindowName(first.Name)
	firstCwd := resolveCwd(env.Root, first.Cwd)

	args := []string{"new-session", "-d", "-s", session, "-n", firstName}
	if firstCwd != "" {
		args = append(args, "-c", firstCwd)
	}
	if firstCommand := startupCommand(first.Cmd); firstCommand != "" {
		args = append(args, firstCommand)
	}
	log.Printf("EnsureSession: creating session with first window %q cwd=%q cmd=%q args=%v", firstName, firstCwd, first.Cmd, args)
	if err := exec.Command("tmux", args...).Run(); err != nil {
		log.Printf("EnsureSession: ERROR creating session %q: %v", session, err)
		return fmt.Errorf("create tmux session %q: %w", session, err)
	}
	log.Printf("EnsureSession: session %q created", session)

	for i, w := range env.Windows[1:] {
		name := safeWindowName(w.Name)
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
// search popup directly inside the tmux session using display-popup.
func BindSearchKey(ideBinary string) {
	_ = exec.Command("tmux", "bind-key", "-T", "prefix", "a",
		"display-popup", "-E", "-w", "80%", "-h", "80%", ideBinary, "--search").Run()
}

func AttachTarget(env config.Environment, windowName string) string {
	session := SessionName(env.Name)
	if strings.TrimSpace(windowName) == "" {
		return session
	}
	return session + ":" + safeWindowName(windowName)
}

func WindowNames(env config.Environment) []string {
	if len(env.Windows) == 0 {
		return []string{"shell"}
	}
	out := make([]string, 0, len(env.Windows))
	for _, w := range env.Windows {
		out = append(out, safeWindowName(w.Name))
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

func safeWindowName(name string) string {
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
	target := session + ":" + safeWindowName(window)
	cmd := exec.Command("tmux", "display-message", "-p", "-t", target, "#{pane_current_command}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

func CapturePane(session, window string) (string, error) {
	target := session + ":" + safeWindowName(window)
	cmd := exec.Command("tmux", "capture-pane", "-p", "-e", "-t", target)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", nil
	}
	return out.String(), nil
}

func CheckTmuxExists() error {
	_, err := exec.LookPath("tmux")
	if err != nil {
		return errors.New("tmux is not installed or not in PATH")
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
	target := session + ":" + safeWindowName(window)

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
