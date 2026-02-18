package tmux

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
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
	if HasSession(session) {
		return nil
	}
	if len(env.Windows) == 0 {
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
	if err := exec.Command("tmux", args...).Run(); err != nil {
		return fmt.Errorf("create tmux session %q: %w", session, err)
	}

	for _, w := range env.Windows[1:] {
		name := safeWindowName(w.Name)
		cwd := resolveCwd(env.Root, w.Cwd)
		args = []string{"new-window", "-t", session, "-n", name}
		if cwd != "" {
			args = append(args, "-c", cwd)
		}
		if command := startupCommand(w.Cmd); command != "" {
			args = append(args, command)
		}
		if err := exec.Command("tmux", args...).Run(); err != nil {
			return fmt.Errorf("create window %q: %w", name, err)
		}
	}

	return nil
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
	script := strings.TrimSpace(command) + `; exec "${SHELL:-/bin/sh}" -i`
	return "sh -lc " + shellQuote(script)
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
	cmd := exec.Command("tmux", "capture-pane", "-p", "-t", target)
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
