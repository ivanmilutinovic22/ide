package run

import (
	"ide/pkg/config"

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
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView("new")
}

func Ide() model {
	return model{}
}
