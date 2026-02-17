package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type WindowTemplate struct {
	Name string `json:"name"`
	Cmd  string `json:"cmd,omitempty"`
	Cwd  string `json:"cwd,omitempty"`
}

type Template struct {
	Name    string           `json:"name"`
	Windows []WindowTemplate `json:"windows"`
}

type Environment struct {
	Name         string           `json:"name"`
	Root         string           `json:"root,omitempty"`
	Folder       string           `json:"folder,omitempty"`
	DBConnection string           `json:"db_connection,omitempty"`
	Windows      []WindowTemplate `json:"windows"`
}

type Data struct {
	Environments []Environment
	Templates    []Template
	Theme        string
}

type fileSchema struct {
	Environments []Environment `json:"environments"`
	Templates    []Template    `json:"templates,omitempty"`
	Theme        string        `json:"theme,omitempty"`
}

func ConfigFilePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(base, "ide", "environments.json"), nil
}

func EnsureExists() error {
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat config file: %w", err)
	}

	initial := fileSchema{
		Environments: []Environment{},
		Templates:    DefaultTemplates(),
		Theme:        "Midnight",
	}
	b, err := json.MarshalIndent(initial, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal default config: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write default config: %w", err)
	}
	return nil
}

func Load() ([]Environment, error) {
	data, err := LoadAll()
	if err != nil {
		return nil, err
	}
	return data.Environments, nil
}

func LoadTemplates() ([]Template, error) {
	data, err := LoadAll()
	if err != nil {
		return nil, err
	}
	return data.Templates, nil
}

func LoadAll() (Data, error) {
	if err := EnsureExists(); err != nil {
		return Data{}, err
	}
	path, err := ConfigFilePath()
	if err != nil {
		return Data{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Data{}, fmt.Errorf("read config file: %w", err)
	}
	var cfg fileSchema
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Data{}, fmt.Errorf("parse config json: %w", err)
	}

	for i := range cfg.Environments {
		normalizeEnvironment(&cfg.Environments[i])
	}

	for i := range cfg.Templates {
		normalizeTemplate(&cfg.Templates[i])
	}
	if len(cfg.Templates) == 0 {
		cfg.Templates = DefaultTemplates()
	}

	return Data{
		Environments: cfg.Environments,
		Templates:    cfg.Templates,
		Theme:        strings.TrimSpace(cfg.Theme),
	}, nil
}

func Save(environments []Environment) error {
	data, err := LoadAll()
	if err != nil {
		return err
	}
	data.Environments = environments
	return SaveAll(data)
}

func SaveTemplates(templates []Template) error {
	data, err := LoadAll()
	if err != nil {
		return err
	}
	data.Templates = templates
	return SaveAll(data)
}

func SaveTheme(theme string) error {
	data, err := LoadAll()
	if err != nil {
		return err
	}
	data.Theme = strings.TrimSpace(theme)
	return SaveAll(data)
}

func SaveAll(data Data) error {
	if err := EnsureExists(); err != nil {
		return err
	}
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}

	for i := range data.Environments {
		normalizeEnvironment(&data.Environments[i])
	}
	for i := range data.Templates {
		normalizeTemplate(&data.Templates[i])
	}

	cfg := fileSchema{
		Environments: data.Environments,
		Templates:    data.Templates,
		Theme:        strings.TrimSpace(data.Theme),
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config json: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func DefaultWindows() []WindowTemplate {
	windows := legacyDefaultWindows("")
	return cloneWindows(windows)
}

func DefaultTemplates() []Template {
	return []Template{
		{
			Name:    "default",
			Windows: DefaultWindows(),
		},
	}
}

func cloneWindows(windows []WindowTemplate) []WindowTemplate {
	out := make([]WindowTemplate, len(windows))
	copy(out, windows)
	return out
}

func normalizeEnvironment(env *Environment) {
	env.Name = strings.TrimSpace(env.Name)
	env.Folder = strings.TrimSpace(env.Folder)
	env.DBConnection = strings.TrimSpace(env.DBConnection)
	if env.Root == "" {
		env.Root = env.Folder
	}
	env.Root = normalizePath(env.Root)
	if len(env.Windows) == 0 {
		env.Windows = legacyDefaultWindows(env.DBConnection)
	}
	env.Windows = normalizeWindows(env.Windows)
}

func normalizeTemplate(template *Template) {
	template.Name = strings.TrimSpace(template.Name)
	if template.Name == "" {
		template.Name = "template"
	}
	if len(template.Windows) == 0 {
		template.Windows = DefaultWindows()
	}
	template.Windows = normalizeWindows(template.Windows)
}

func normalizeWindows(windows []WindowTemplate) []WindowTemplate {
	if len(windows) == 0 {
		return []WindowTemplate{{Name: "shell"}}
	}
	out := make([]WindowTemplate, 0, len(windows))
	for i := range windows {
		w := windows[i]
		w.Name = strings.TrimSpace(w.Name)
		w.Cmd = strings.TrimSpace(w.Cmd)
		w.Cwd = strings.TrimSpace(w.Cwd)
		if w.Name == "" {
			w.Name = fmt.Sprintf("window-%d", len(out)+1)
		}
		out = append(out, w)
	}
	return out
}

func normalizePath(value string) string {
	value = os.ExpandEnv(strings.TrimSpace(value))
	if strings.HasPrefix(value, "~/") {
		home, hErr := os.UserHomeDir()
		if hErr == nil {
			value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	if value == "" {
		return value
	}
	return filepath.Clean(value)
}

func legacyDefaultWindows(dbConnection string) []WindowTemplate {
	dbCmd := "dbdash"
	if strings.TrimSpace(dbConnection) != "" {
		dbCmd = "dbdash connect " + strings.TrimSpace(dbConnection)
	}

	return []WindowTemplate{
		{Name: "editor", Cmd: "nvim ."},
		{Name: "terminal"},
		{Name: "lazygit", Cmd: "lazygit"},
		{Name: "k9s", Cmd: "k9s -n te"},
		{Name: "database", Cmd: dbCmd},
	}
}
