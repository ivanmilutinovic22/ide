package cli

import (
	"fmt"
	"os"
	"strings"

	"ide/internal/config"
)

func dispatchEnvWindow(args []string) int {
	if len(args) == 0 {
		return usagef(os.Stderr, "usage: ide env window <list|add|set|rm> ...")
	}
	switch args[0] {
	case "list", "ls":
		return envWindowList(args[1:])
	case "add", "create":
		return envWindowAdd(args[1:])
	case "set", "edit":
		return envWindowSet(args[1:])
	case "rm", "remove", "delete":
		return envWindowRm(args[1:])
	}
	return usagef(os.Stderr, "ide: unknown env window subcommand %q", args[0])
}

func dispatchTemplateWindow(args []string) int {
	if len(args) == 0 {
		return usagef(os.Stderr, "usage: ide template window <list|add|set|rm> ...")
	}
	switch args[0] {
	case "list", "ls":
		return templateWindowList(args[1:])
	case "add", "create":
		return templateWindowAdd(args[1:])
	case "set", "edit":
		return templateWindowSet(args[1:])
	case "rm", "remove", "delete":
		return templateWindowRm(args[1:])
	}
	return usagef(os.Stderr, "ide: unknown template window subcommand %q", args[0])
}

// --- env window ----------------------------------------------------------

func envWindowList(args []string) int {
	if len(args) != 1 {
		return usagef(os.Stderr, "usage: ide env window list <env>")
	}
	envs, err := config.Load()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	idx := findEnv(envs, args[0])
	if idx < 0 {
		return errf(os.Stderr, "no such environment %q", args[0])
	}
	return printWindows(envs[idx].Windows)
}

func envWindowAdd(args []string) int {
	fs := newFlagSet("env window add")
	cmd := fs.string("cmd", "startup command")
	cwd := fs.string("cwd", "working directory (relative to env root)")
	if err := fs.parse(args); err != nil {
		return usagef(os.Stderr, "usage: ide env window add <env> <window> [--cmd CMD] [--cwd CWD]")
	}
	pos := fs.positional()
	if len(pos) != 2 {
		return usagef(os.Stderr, "usage: ide env window add <env> <window> [--cmd CMD] [--cwd CWD]")
	}
	envName, winName := pos[0], trim(pos[1])
	if winName == "" {
		return errf(os.Stderr, "window name is required")
	}
	envs, err := config.Load()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	idx := findEnv(envs, envName)
	if idx < 0 {
		return errf(os.Stderr, "no such environment %q", envName)
	}
	if findWindow(envs[idx].Windows, winName) >= 0 {
		return errf(os.Stderr, "window %q already exists in %q", winName, envs[idx].Name)
	}
	envs[idx].Windows = append(envs[idx].Windows, config.WindowTemplate{
		Name: winName,
		Cmd:  trim(*cmd),
		Cwd:  trim(*cwd),
	})
	if err := config.Save(envs); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("added window %q to env %q\n", winName, envs[idx].Name)
	return 0
}

func envWindowSet(args []string) int {
	fs := newFlagSet("env window set")
	name := fs.string("name", "new window name")
	cmd := fs.string("cmd", "startup command")
	cwd := fs.string("cwd", "working directory")
	if err := fs.parse(args); err != nil {
		return usagef(os.Stderr, "usage: ide env window set <env> <window> [--name NEW] [--cmd CMD] [--cwd CWD]")
	}
	pos := fs.positional()
	if len(pos) != 2 {
		return usagef(os.Stderr, "usage: ide env window set <env> <window> [--name NEW] [--cmd CMD] [--cwd CWD]")
	}
	envName, winName := pos[0], pos[1]

	envs, err := config.Load()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	eIdx := findEnv(envs, envName)
	if eIdx < 0 {
		return errf(os.Stderr, "no such environment %q", envName)
	}
	wIdx := findWindow(envs[eIdx].Windows, winName)
	if wIdx < 0 {
		return errf(os.Stderr, "no such window %q in env %q", winName, envs[eIdx].Name)
	}
	if fs.provided("name") {
		nn := trim(*name)
		if nn == "" {
			return errf(os.Stderr, "--name cannot be empty")
		}
		if dup := findWindow(envs[eIdx].Windows, nn); dup >= 0 && dup != wIdx {
			return errf(os.Stderr, "window %q already exists in env %q", nn, envs[eIdx].Name)
		}
		envs[eIdx].Windows[wIdx].Name = nn
	}
	if fs.provided("cmd") {
		envs[eIdx].Windows[wIdx].Cmd = trim(*cmd)
	}
	if fs.provided("cwd") {
		envs[eIdx].Windows[wIdx].Cwd = trim(*cwd)
	}
	if err := config.Save(envs); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("updated window %q in env %q\n", envs[eIdx].Windows[wIdx].Name, envs[eIdx].Name)
	return 0
}

func envWindowRm(args []string) int {
	if len(args) != 2 {
		return usagef(os.Stderr, "usage: ide env window rm <env> <window>")
	}
	envs, err := config.Load()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	eIdx := findEnv(envs, args[0])
	if eIdx < 0 {
		return errf(os.Stderr, "no such environment %q", args[0])
	}
	wIdx := findWindow(envs[eIdx].Windows, args[1])
	if wIdx < 0 {
		return errf(os.Stderr, "no such window %q in env %q", args[1], envs[eIdx].Name)
	}
	envs[eIdx].Windows = append(envs[eIdx].Windows[:wIdx], envs[eIdx].Windows[wIdx+1:]...)
	if err := config.Save(envs); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("removed window %q from env %q\n", args[1], envs[eIdx].Name)
	return 0
}

// --- template window -----------------------------------------------------

func templateWindowList(args []string) int {
	if len(args) != 1 {
		return usagef(os.Stderr, "usage: ide template window list <template>")
	}
	templates, err := config.LoadTemplates()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	idx := findTemplate(templates, args[0])
	if idx < 0 {
		return errf(os.Stderr, "no such template %q", args[0])
	}
	return printWindows(templates[idx].Windows)
}

func templateWindowAdd(args []string) int {
	fs := newFlagSet("template window add")
	cmd := fs.string("cmd", "startup command")
	cwd := fs.string("cwd", "working directory")
	if err := fs.parse(args); err != nil {
		return usagef(os.Stderr, "usage: ide template window add <template> <window> [--cmd CMD] [--cwd CWD]")
	}
	pos := fs.positional()
	if len(pos) != 2 {
		return usagef(os.Stderr, "usage: ide template window add <template> <window> [--cmd CMD] [--cwd CWD]")
	}
	tName, winName := pos[0], trim(pos[1])
	if winName == "" {
		return errf(os.Stderr, "window name is required")
	}
	templates, err := config.LoadTemplates()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	idx := findTemplate(templates, tName)
	if idx < 0 {
		return errf(os.Stderr, "no such template %q", tName)
	}
	if findWindow(templates[idx].Windows, winName) >= 0 {
		return errf(os.Stderr, "window %q already exists in template %q", winName, templates[idx].Name)
	}
	templates[idx].Windows = append(templates[idx].Windows, config.WindowTemplate{
		Name: winName,
		Cmd:  trim(*cmd),
		Cwd:  trim(*cwd),
	})
	if err := config.SaveTemplates(templates); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("added window %q to template %q\n", winName, templates[idx].Name)
	return 0
}

func templateWindowSet(args []string) int {
	fs := newFlagSet("template window set")
	name := fs.string("name", "new window name")
	cmd := fs.string("cmd", "startup command")
	cwd := fs.string("cwd", "working directory")
	if err := fs.parse(args); err != nil {
		return usagef(os.Stderr, "usage: ide template window set <template> <window> [--name NEW] [--cmd CMD] [--cwd CWD]")
	}
	pos := fs.positional()
	if len(pos) != 2 {
		return usagef(os.Stderr, "usage: ide template window set <template> <window> [--name NEW] [--cmd CMD] [--cwd CWD]")
	}
	tName, winName := pos[0], pos[1]
	templates, err := config.LoadTemplates()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	tIdx := findTemplate(templates, tName)
	if tIdx < 0 {
		return errf(os.Stderr, "no such template %q", tName)
	}
	wIdx := findWindow(templates[tIdx].Windows, winName)
	if wIdx < 0 {
		return errf(os.Stderr, "no such window %q in template %q", winName, templates[tIdx].Name)
	}
	if fs.provided("name") {
		nn := trim(*name)
		if nn == "" {
			return errf(os.Stderr, "--name cannot be empty")
		}
		if dup := findWindow(templates[tIdx].Windows, nn); dup >= 0 && dup != wIdx {
			return errf(os.Stderr, "window %q already exists in template %q", nn, templates[tIdx].Name)
		}
		templates[tIdx].Windows[wIdx].Name = nn
	}
	if fs.provided("cmd") {
		templates[tIdx].Windows[wIdx].Cmd = trim(*cmd)
	}
	if fs.provided("cwd") {
		templates[tIdx].Windows[wIdx].Cwd = trim(*cwd)
	}
	if err := config.SaveTemplates(templates); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("updated window %q in template %q\n", templates[tIdx].Windows[wIdx].Name, templates[tIdx].Name)
	return 0
}

func templateWindowRm(args []string) int {
	if len(args) != 2 {
		return usagef(os.Stderr, "usage: ide template window rm <template> <window>")
	}
	templates, err := config.LoadTemplates()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	tIdx := findTemplate(templates, args[0])
	if tIdx < 0 {
		return errf(os.Stderr, "no such template %q", args[0])
	}
	wIdx := findWindow(templates[tIdx].Windows, args[1])
	if wIdx < 0 {
		return errf(os.Stderr, "no such window %q in template %q", args[1], templates[tIdx].Name)
	}
	templates[tIdx].Windows = append(templates[tIdx].Windows[:wIdx], templates[tIdx].Windows[wIdx+1:]...)
	if err := config.SaveTemplates(templates); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("removed window %q from template %q\n", args[1], templates[tIdx].Name)
	return 0
}

// --- helpers -------------------------------------------------------------

func findWindow(windows []config.WindowTemplate, name string) int {
	name = strings.TrimSpace(name)
	for i, w := range windows {
		if strings.EqualFold(strings.TrimSpace(w.Name), name) {
			return i
		}
	}
	return -1
}

func printWindows(windows []config.WindowTemplate) int {
	if len(windows) == 0 {
		fmt.Println("(no windows)")
		return 0
	}
	for i, w := range windows {
		fmt.Printf("%d. %s\tcmd=%q\tcwd=%q\n", i+1, w.Name, w.Cmd, w.Cwd)
	}
	return 0
}
