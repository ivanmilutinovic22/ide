package run

import (
	"ide/pkg/config"
	"ide/pkg/layout"

	tea "charm.land/bubbletea/v2"
)

type configLoadedMsg struct {
	cfg config.Config
	err error
}

type model struct {
	width  int
	height int
	cfg    config.Config
	err    error
}

func loadConfigCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		return configLoadedMsg{cfg: cfg, err: err}
	}
}

func (m model) Init() tea.Cmd {
	return loadConfigCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case configLoadedMsg:
		m.cfg = msg.cfg
		m.err = msg.err
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView(layout.Render(m.width, m.height, m.cfg.Sessions))
}

func Ide() model {
	return model{}
}
