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

// HasSession reports whether the tmux server is running and has a session
// with the given name. The bool answers the question; the error is non-nil
// only when tmux itself failed in a way distinct from "no such session"
// (e.g. tmux not installed or socket dir unreadable).
func HasSession(session string) (bool, error) {
	cmd := exec.Command("tmux", "has-session", "-t", session)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		text := stderr.String()
		// tmux exits non-zero with these messages when the session simply
		// doesn't exist; that's the "no" answer, not an error.
		if strings.Contains(text, "no server running") ||
			strings.Contains(text, "can't find session") ||
			strings.Contains(text, "session not found") {
			return false, nil
		}
		// Distinguish process-startup failures (tmux missing, permission
		// denied) from session-not-found by inspecting the exit error kind.
		if _, ok := err.(*exec.ExitError); ok {
			// Non-zero exit with an unrecognised stderr — treat as no.
			return false, nil
		}
		return false, fmt.Errorf("has-session: %w", err)
	}
	return true, nil
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

	// The search popup (ide --search) and current-session window switcher
	// (ide --windows) are both wired up via the user's tmux config rather than
	// runtime bindings, so nothing is bound here.

	log.Printf("EnsureSession: done, session %q has %d windows", session, len(env.Windows))
	return nil
}

// SwapWindow swaps two windows live in the running tmux session.
// Best-effort: returns the underlying error so callers can decide whether
// to surface or ignore it.
func SwapWindow(session, src, dst string) error {
	if _, err := runTmux("swap-window", "-s", session+":"+src, "-t", session+":"+dst); err != nil {
		return fmt.Errorf("swap-window %s:%s -> %s:%s: %w", session, src, session, dst, err)
	}
	return nil
}

// SelectWindow brings target to the foreground in the running tmux session.
// Best-effort: any error is returned.
func SelectWindow(target string) error {
	if _, err := runTmux("select-window", "-t", target); err != nil {
		return fmt.Errorf("select-window %s: %w", target, err)
	}
	return nil
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
	// -J preserves trailing whitespace and its styling. Without it tmux drops
	// row-tail spaces even when they carry a non-default BG (e.g. nvim's
	// gruvbox Normal hl), so the preview would lose the row-fill colour.
	out, err := runTmux("capture-pane", "-p", "-e", "-J", "-t", target)
	if err != nil {
		return "", fmt.Errorf("capture pane %q: %w", target, err)
	}
	return out, nil
}

// PaneSize returns the current pane's columns and rows via tmux display-message.
func PaneSize(session, window string) (int, int, error) {
	target := session + ":" + SafeWindowName(window)
	out, err := runTmux("display-message", "-p", "-t", target, "#{pane_width} #{pane_height}")
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("pane size: unexpected output %q", strings.TrimSpace(out))
	}
	cols, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("pane size: cols: %w", err)
	}
	rows, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("pane size: rows: %w", err)
	}
	return cols, rows, nil
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

// procRow is one entry from the system-wide `ps` snapshot.
type procRow struct {
	pid   int
	ppid  int
	cpu   float64
	state string
}

// snapshotProcesses runs `ps` ONCE and returns the full process table keyed
// by PID. Old code spawned one `pgrep` per node plus one `ps` per child of
// the pane's process tree, for every AI window, every 500ms. That fork-bombed
// the system; this version is one subprocess per poll regardless of tree size.
func snapshotProcesses() (map[int]procRow, error) {
	// Use "=" suffix on format specifiers to suppress headers (works on
	// macOS and Linux). Order: pid, ppid, %cpu, state.
	cmd := exec.Command("ps", "-A", "-o", "pid=", "-o", "ppid=", "-o", "%cpu=", "-o", "state=")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ps -A: %w", err)
	}
	rows := map[int]procRow{}
	for _, line := range strings.Split(out.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		cpu, _ := strconv.ParseFloat(fields[2], 64)
		state := fields[3]
		if len(state) > 0 {
			state = state[:1]
		}
		rows[pid] = procRow{pid: pid, ppid: ppid, cpu: cpu, state: state}
	}
	return rows, nil
}

// buildChildMap groups the process table by parent PID.
func buildChildMap(rows map[int]procRow) map[int][]int {
	children := map[int][]int{}
	for pid, row := range rows {
		children[row.ppid] = append(children[row.ppid], pid)
	}
	return children
}

// GetPaneProcessInfo retrieves the current process info for a pane.
// It sums CPU usage across the pane's shell AND all its descendants, so a
// single-process pane (no shell wrapper) is still detected as active.
func GetPaneProcessInfo(session, window string) (ProcessInfo, error) {
	target := session + ":" + SafeWindowName(window)

	out, err := runTmux("display-message", "-p", "-t", target, "#{pane_pid}")
	if err != nil {
		return ProcessInfo{}, err
	}
	shellPID, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return ProcessInfo{}, fmt.Errorf("parse pane PID %q: %w", out, err)
	}

	rows, err := snapshotProcesses()
	if err != nil {
		return ProcessInfo{}, err
	}
	children := buildChildMap(rows)

	// Walk the subtree (including the shell itself) and aggregate CPU + a
	// running flag. Using an iterative stack instead of recursion so an
	// adversarial process loop (shouldn't happen, but ps under namespacing
	// has produced cycles before) doesn't blow the call stack.
	totalCPU := 0.0
	hasRunning := false
	stack := []int{shellPID}
	visited := map[int]bool{}
	for len(stack) > 0 {
		n := len(stack) - 1
		pid := stack[n]
		stack = stack[:n]
		if visited[pid] {
			continue
		}
		visited[pid] = true
		if row, ok := rows[pid]; ok {
			totalCPU += row.cpu
			if row.state == "R" {
				hasRunning = true
			}
		}
		stack = append(stack, children[pid]...)
	}

	state := "S"
	if hasRunning {
		state = "R"
	}
	return ProcessInfo{
		PID:   shellPID,
		CPU:   totalCPU,
		State: state,
	}, nil
}
