package ui

import (
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ide/internal/config"
	"ide/internal/tmux"
)

type configLoadedMsg struct {
	envs      []config.Environment
	templates []config.Template
	theme     string
	err       error
}

type sessionsLoadedMsg struct {
	names   []string
	windows map[string][]string
	err     error
}

type attachReadyMsg struct {
	target string
	err    error
}

type attachDoneMsg struct {
	err error
}

type environmentCreatedMsg struct {
	env        config.Environment
	sessionErr error
	err        error
}

type windowMovedMsg struct {
	envName    string
	direction  int
	sessionErr error
	err        error
}

type templateSavedMsg struct {
	name   string
	edited bool
	err    error
}

type templateDeletedMsg struct {
	name string
	err  error
}

type sessionKilledMsg struct {
	session string
	err     error
}

type environmentDeletedMsg struct {
	environment string
	session     string
	killed      bool
	err         error
}

type themePersistedMsg struct {
	name string
	err  error
}

type panePreviewMsg struct {
	session string
	window  string
	content string
	process string
}

type previewTickMsg struct{}

// agentStatusUpdateMsg carries process info updates for agent status detection
type agentStatusUpdateMsg struct {
	session  string
	window   string
	procInfo ProcessInfo
}

func loadConfigCmd() tea.Cmd {
	return func() tea.Msg {
		log.Printf("loadConfig: reading config")
		data, err := config.LoadAll()
		if err != nil {
			log.Printf("loadConfig: ERROR %v", err)
			return configLoadedMsg{err: err}
		}
		envs := data.Environments
		templates := data.Templates
		theme := strings.TrimSpace(data.Theme)
		log.Printf("loadConfig: loaded %d environments, %d templates, theme=%q", len(envs), len(templates), theme)
		sort.Slice(envs, func(i, j int) bool {
			return strings.ToLower(envs[i].Name) < strings.ToLower(envs[j].Name)
		})
		sort.SliceStable(templates, func(i, j int) bool {
			leftDefault := strings.EqualFold(templates[i].Name, "default")
			rightDefault := strings.EqualFold(templates[j].Name, "default")
			if leftDefault && !rightDefault {
				return true
			}
			if !leftDefault && rightDefault {
				return false
			}
			return strings.ToLower(templates[i].Name) < strings.ToLower(templates[j].Name)
		})
		return configLoadedMsg{envs: envs, templates: templates, theme: theme}
	}
}

func loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		log.Printf("loadSessions: listing tmux sessions")
		names, err := tmux.ListSessions()
		if err != nil {
			log.Printf("loadSessions: ERROR listing sessions: %v", err)
			return sessionsLoadedMsg{err: err}
		}
		log.Printf("loadSessions: found %d sessions: %v", len(names), names)
		windows := map[string][]string{}
		var mu sync.Mutex
		var wg sync.WaitGroup
		var firstErr error
		for _, name := range names {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				ws, winErr := tmux.ListWindows(name)
				mu.Lock()
				defer mu.Unlock()
				if winErr != nil {
					log.Printf("loadSessions: ERROR listing windows for %q: %v", name, winErr)
					if firstErr == nil {
						firstErr = winErr
					}
					return
				}
				log.Printf("loadSessions: session %q has windows: %v", name, ws)
				windows[name] = ws
			}(name)
		}
		wg.Wait()
		if firstErr != nil {
			return sessionsLoadedMsg{err: firstErr}
		}
		return sessionsLoadedMsg{names: names, windows: windows}
	}
}

func saveThemePreferenceCmd(theme string) tea.Cmd {
	return func() tea.Msg {
		theme = strings.TrimSpace(theme)
		if theme == "" {
			return themePersistedMsg{err: fmt.Errorf("theme name is required")}
		}
		err := config.SaveTheme(theme)
		return themePersistedMsg{name: theme, err: err}
	}
}

func createEnvironmentCmd(name, root string, windows []config.WindowTemplate) tea.Cmd {
	return func() tea.Msg {
		name = strings.TrimSpace(name)
		root = normalizeRootPath(root)
		log.Printf("createEnvironment: name=%q root=%q windows=%d", name, root, len(windows))
		if name == "" {
			return environmentCreatedMsg{err: fmt.Errorf("name is required")}
		}
		if root == "" {
			return environmentCreatedMsg{err: fmt.Errorf("root path is required")}
		}

		data, err := config.LoadAll()
		if err != nil {
			log.Printf("createEnvironment: ERROR loading config: %v", err)
			return environmentCreatedMsg{err: err}
		}
		for _, env := range data.Environments {
			if strings.EqualFold(strings.TrimSpace(env.Name), name) {
				log.Printf("createEnvironment: environment %q already exists", name)
				return environmentCreatedMsg{err: fmt.Errorf("environment %q already exists", name)}
			}
		}
		if len(windows) == 0 {
			windows = config.DefaultWindows()
			log.Printf("createEnvironment: no windows provided, using defaults (%d windows)", len(windows))
		}
		for i, w := range windows {
			log.Printf("createEnvironment: window[%d] name=%q cmd=%q cwd=%q", i, w.Name, w.Cmd, w.Cwd)
		}

		newEnv := config.Environment{
			Name:    name,
			Root:    root,
			Windows: cloneWindowTemplates(windows),
		}
		data.Environments = append(data.Environments, newEnv)
		if err := config.SaveAll(data); err != nil {
			log.Printf("createEnvironment: ERROR saving config: %v", err)
			return environmentCreatedMsg{err: err}
		}

		sessionErr := tmux.CheckTmuxExists()
		if sessionErr == nil {
			sessionErr = tmux.EnsureSession(newEnv)
		}
		if sessionErr != nil {
			log.Printf("createEnvironment: ERROR ensuring session: %v", sessionErr)
		} else {
			log.Printf("createEnvironment: session ready for %q", name)
		}

		return environmentCreatedMsg{env: newEnv, sessionErr: sessionErr}
	}
}

func saveTemplateCmd(originalName, name string, windows []config.WindowTemplate) tea.Cmd {
	return func() tea.Msg {
		originalName = strings.TrimSpace(originalName)
		name = strings.TrimSpace(name)
		log.Printf("saveTemplate: originalName=%q name=%q windows=%d", originalName, name, len(windows))
		for i, w := range windows {
			log.Printf("saveTemplate: window[%d] name=%q cmd=%q cwd=%q", i, w.Name, w.Cmd, w.Cwd)
		}
		if name == "" {
			return templateSavedMsg{err: fmt.Errorf("template name is required")}
		}
		if len(windows) == 0 {
			return templateSavedMsg{err: fmt.Errorf("template must contain at least one window")}
		}

		data, err := config.LoadAll()
		if err != nil {
			return templateSavedMsg{err: err}
		}

		targetIdx := -1
		if originalName != "" {
			for i := range data.Templates {
				if strings.EqualFold(strings.TrimSpace(data.Templates[i].Name), originalName) {
					targetIdx = i
					break
				}
			}
			if targetIdx < 0 {
				return templateSavedMsg{err: fmt.Errorf("template %q not found", originalName)}
			}
		}

		for i, existing := range data.Templates {
			if strings.EqualFold(strings.TrimSpace(existing.Name), name) {
				if targetIdx >= 0 && i == targetIdx {
					continue
				}
				return templateSavedMsg{err: fmt.Errorf("template %q already exists", name)}
			}
		}

		template := config.Template{Name: name, Windows: cloneWindowTemplates(windows)}
		edited := targetIdx >= 0
		if edited {
			data.Templates[targetIdx] = template
		} else {
			data.Templates = append(data.Templates, template)
		}

		if err := config.SaveAll(data); err != nil {
			return templateSavedMsg{err: err}
		}

		return templateSavedMsg{name: name, edited: edited}
	}
}

func killSessionCmd(session string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("killSession: session=%q", session)
		if err := tmux.CheckTmuxExists(); err != nil {
			log.Printf("killSession: tmux not found: %v", err)
			return sessionKilledMsg{session: session, err: err}
		}
		err := tmux.KillSession(session)
		if err != nil {
			log.Printf("killSession: ERROR killing %q: %v", session, err)
		} else {
			log.Printf("killSession: killed %q", session)
		}
		return sessionKilledMsg{session: session, err: err}
	}
}

func deleteTemplateCmd(name string) tea.Cmd {
	return func() tea.Msg {
		name = strings.TrimSpace(name)
		if name == "" {
			return templateDeletedMsg{err: fmt.Errorf("template name is required")}
		}

		data, err := config.LoadAll()
		if err != nil {
			return templateDeletedMsg{err: err}
		}

		idx := -1
		for i := range data.Templates {
			if strings.EqualFold(strings.TrimSpace(data.Templates[i].Name), name) {
				idx = i
				break
			}
		}
		if idx < 0 {
			return templateDeletedMsg{err: fmt.Errorf("template %q not found", name)}
		}

		deletedName := data.Templates[idx].Name
		data.Templates = append(data.Templates[:idx], data.Templates[idx+1:]...)
		if err := config.SaveAll(data); err != nil {
			return templateDeletedMsg{err: err}
		}

		return templateDeletedMsg{name: deletedName}
	}
}

func deleteEnvironmentCmd(name string) tea.Cmd {
	return func() tea.Msg {
		data, err := config.LoadAll()
		if err != nil {
			return environmentDeletedMsg{err: err}
		}

		idx := -1
		removed := config.Environment{}
		for i := range data.Environments {
			if strings.EqualFold(strings.TrimSpace(data.Environments[i].Name), strings.TrimSpace(name)) {
				idx = i
				removed = data.Environments[i]
				break
			}
		}
		if idx < 0 {
			return environmentDeletedMsg{err: fmt.Errorf("environment %q not found", name)}
		}

		data.Environments = append(data.Environments[:idx], data.Environments[idx+1:]...)
		if err := config.SaveAll(data); err != nil {
			return environmentDeletedMsg{err: err}
		}

		session := tmux.SessionName(removed.Name)
		killed := false
		if tmux.CheckTmuxExists() == nil && tmux.HasSession(session) {
			if err := tmux.KillSession(session); err != nil {
				return environmentDeletedMsg{err: err}
			}
			killed = true
		}

		return environmentDeletedMsg{environment: removed.Name, session: session, killed: killed}
	}
}

func moveWindowOrderCmd(envName, windowName string, direction int) tea.Cmd {
	return func() tea.Msg {
		if direction == 0 {
			return windowMovedMsg{err: fmt.Errorf("direction must be non-zero")}
		}

		envs, err := config.Load()
		if err != nil {
			return windowMovedMsg{err: err}
		}

		envIdx := -1
		for i := range envs {
			if strings.EqualFold(strings.TrimSpace(envs[i].Name), strings.TrimSpace(envName)) {
				envIdx = i
				break
			}
		}
		if envIdx < 0 {
			return windowMovedMsg{err: fmt.Errorf("environment %q not found", envName)}
		}

		env := &envs[envIdx]
		if len(env.Windows) < 2 {
			return windowMovedMsg{err: fmt.Errorf("environment %q has fewer than 2 windows", env.Name)}
		}

		targetWindow := tmux.SafeWindowName(windowName)
		sourceIdx := -1
		for i := range env.Windows {
			if tmux.SafeWindowName(env.Windows[i].Name) == targetWindow {
				sourceIdx = i
				break
			}
		}
		if sourceIdx < 0 {
			return windowMovedMsg{err: fmt.Errorf("window %q not found in template", windowName)}
		}

		destinationIdx := sourceIdx + direction
		if destinationIdx < 0 || destinationIdx >= len(env.Windows) {
			if direction < 0 {
				return windowMovedMsg{err: fmt.Errorf("window is already first")}
			}
			return windowMovedMsg{err: fmt.Errorf("window is already last")}
		}

		sourceWindow := tmux.SafeWindowName(env.Windows[sourceIdx].Name)
		destinationWindow := tmux.SafeWindowName(env.Windows[destinationIdx].Name)
		env.Windows[sourceIdx], env.Windows[destinationIdx] = env.Windows[destinationIdx], env.Windows[sourceIdx]

		if err := config.Save(envs); err != nil {
			return windowMovedMsg{err: err}
		}

		var sessionErr error
		session := tmux.SessionName(env.Name)
		if tmux.HasSession(session) {
			proc := exec.Command("tmux", "swap-window", "-s", session+":"+sourceWindow, "-t", session+":"+destinationWindow)
			if err := proc.Run(); err != nil {
				sessionErr = fmt.Errorf("swap live tmux windows: %w", err)
			}
		}

		return windowMovedMsg{envName: env.Name, direction: direction, sessionErr: sessionErr}
	}
}

func prepareAttachCmd(env config.Environment, windowName string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("prepareAttach: env=%q window=%q", env.Name, windowName)
		if err := tmux.CheckTmuxExists(); err != nil {
			log.Printf("prepareAttach: tmux not found: %v", err)
			return attachReadyMsg{err: err}
		}
		if err := tmux.EnsureSession(env); err != nil {
			log.Printf("prepareAttach: ERROR ensuring session for %q: %v", env.Name, err)
			return attachReadyMsg{err: err}
		}
		session := tmux.SessionName(env.Name)
		target := tmux.AttachTarget(env, windowName)
		log.Printf("prepareAttach: session=%q target=%q", session, target)
		if strings.TrimSpace(windowName) != "" {
			hasWindow, err := tmux.HasWindow(session, windowName)
			if err != nil {
				log.Printf("prepareAttach: ERROR checking window %q: %v", windowName, err)
				return attachReadyMsg{err: err}
			}
			log.Printf("prepareAttach: hasWindow(%q)=%v", windowName, hasWindow)
			if hasWindow {
				_ = exec.Command("tmux", "select-window", "-t", target).Run()
			} else {
				log.Printf("prepareAttach: window %q not found, falling back to session root", windowName)
				target = session
			}
		}
		log.Printf("prepareAttach: ready, attaching to %q", target)
		return attachReadyMsg{target: target}
	}
}

func execAttachCmd(target string) tea.Cmd {
	proc := exec.Command("tmux", "attach-session", "-t", target)
	return tea.ExecProcess(proc, func(err error) tea.Msg {
		return attachDoneMsg{err: err}
	})
}

func capturePaneCmd(session, window string) tea.Cmd {
	return func() tea.Msg {
		content, _ := tmux.CapturePane(session, window)
		process := tmux.CurrentProcess(session, window)
		return panePreviewMsg{session: session, window: window, content: content, process: process}
	}
}

func (m Model) captureCurrentWindowCmd() tea.Cmd {
	var cmds []tea.Cmd

	// Check agent status for all AI windows across ALL running environments
	for _, e := range m.environments {
		s := tmux.SessionName(e.Name)
		if _, live := m.sessions[s]; !live {
			continue
		}
		ws := m.windowNamesForEnv(e)
		for _, w := range ws {
			if tmpl, ok := findWindowTemplate(e, w); ok && HasTag(tmpl, "ai") {
				cmds = append(cmds, checkAgentStatusCmd(s, w))
			}
		}
	}

	// Also capture the pane preview for the currently selected window
	if env, ok := m.currentEnv(); ok {
		session := tmux.SessionName(env.Name)
		if _, live := m.sessions[session]; live {
			windows := m.currentWindowNames()
			if len(windows) > 0 && m.selectedWindow < len(windows) {
				cmds = append(cmds, capturePaneCmd(session, windows[m.selectedWindow]))
			}
		}
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// checkAgentStatusCmd creates a command to check agent status for a window
func checkAgentStatusCmd(session, window string) tea.Cmd {
	return func() tea.Msg {
		procInfo, err := tmux.GetPaneProcessInfo(session, window)
		if err != nil {
			return nil
		}
		return agentStatusUpdateMsg{
			session: session,
			window:  window,
			procInfo: ProcessInfo{
				PID:       procInfo.PID,
				CPU:       procInfo.CPU,
				State:     procInfo.State,
				Timestamp: time.Now(),
			},
		}
	}
}
