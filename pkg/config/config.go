package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const filename = "ide.yaml"

type Window struct {
	Name string `yaml:"name"`
	Cmd  string `yaml:"cmd,omitempty"`
}

type Session struct {
	Name    string   `yaml:"name"`
	Root    string   `yaml:"root,omitempty"`
	Windows []Window `yaml:"windows,omitempty"`
}

// Template stores name=cmd pairs encoded as "name=cmd;name2=cmd2;"
type Template struct {
	Name    string `yaml:"name"`
	Windows string `yaml:"windows"`
}

type Config struct {
	Theme     string     `yaml:"theme,omitempty"`
	Sessions  []Session  `yaml:"sessions,omitempty"`
	Templates []Template `yaml:"templates,omitempty"`
}

func ConfigPath() (string, error) {
	// XDG_CONFIG_HOME takes priority; falls back to ~/.config
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, filename), nil
}

func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}

	if err := ensureExists(path); err != nil {
		return Config{}, err
	}

	cfgFileContents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(cfgFileContents, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

func defaultTemplates() []Template {
	return []Template{
		{
			Name:    "default",
			Windows: "term;editor=vim;claude=claude;opencode=opencode",
		},
	}
}

func Save(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, b, 0o644)
}

func ensureExists(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat config: %w", err)
	}

	defaults := Config{
		Theme:     "Midnight",
		Sessions:  []Session{},
		Templates: defaultTemplates(),
	}

	b, err := yaml.Marshal(defaults)
	if err != nil {
		return fmt.Errorf("marshal default config: %w", err)
	}

	return os.WriteFile(path, b, 0o644)
}

// ParseTemplateWindows parses "name=cmd;name2=cmd2;" into []Window.
func ParseTemplateWindows(raw string) []Window {
	parts := strings.Split(raw, ";")
	windows := make([]Window, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		name, cmd, _ := strings.Cut(p, "=")
		windows = append(windows, Window{Name: strings.TrimSpace(name), Cmd: strings.TrimSpace(cmd)})
	}
	return windows
}
