package cli

import (
	"fmt"
	"os"
	"strings"

	"ide/internal/config"
)

func dispatchEnv(args []string) int {
	if len(args) == 0 {
		return usagef(os.Stderr, "usage: ide env <list|show|add|set|rename|rm|window> ...")
	}
	switch args[0] {
	case "list", "ls":
		return envList(args[1:])
	case "show":
		return envShow(args[1:])
	case "add", "create":
		return envAdd(args[1:])
	case "set", "edit":
		return envSet(args[1:])
	case "rename", "mv":
		return envRename(args[1:])
	case "rm", "remove", "delete":
		return envRm(args[1:])
	case "window", "windows":
		return dispatchEnvWindow(args[1:])
	}
	return usagef(os.Stderr, "ide: unknown env subcommand %q", args[0])
}

func envList(args []string) int {
	if len(args) != 0 {
		return usagef(os.Stderr, "usage: ide env list")
	}
	envs, err := config.Load()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	if len(envs) == 0 {
		fmt.Println("(no environments)")
		return 0
	}
	for _, e := range envs {
		root := e.Root
		if root == "" {
			root = "-"
		}
		fmt.Printf("%s\t%d windows\t%s\n", e.Name, len(e.Windows), root)
	}
	return 0
}

func envShow(args []string) int {
	if len(args) != 1 {
		return usagef(os.Stderr, "usage: ide env show <name>")
	}
	envs, err := config.Load()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	idx := findEnv(envs, args[0])
	if idx < 0 {
		return errf(os.Stderr, "no such environment %q", args[0])
	}
	e := envs[idx]
	fmt.Printf("name:   %s\n", e.Name)
	fmt.Printf("root:   %s\n", emptyDash(e.Root))
	fmt.Printf("folder: %s\n", emptyDash(e.Folder))
	fmt.Printf("db:     %s\n", emptyDash(e.DBConnection))
	fmt.Printf("windows (%d):\n", len(e.Windows))
	for i, w := range e.Windows {
		fmt.Printf("  %d. %s\tcmd=%q\tcwd=%q\n", i+1, w.Name, w.Cmd, w.Cwd)
	}
	return 0
}

func envAdd(args []string) int {
	fs := newFlagSet("env add")
	root := fs.string("root", "filesystem root for the environment")
	db := fs.string("db", "database connection string")
	folder := fs.string("folder", "display folder/group")
	template := fs.string("template", "template name to seed windows from")
	if err := fs.parse(args); err != nil {
		return parseUsagef(os.Stderr, err, "usage: ide env add <name> [--root PATH] [--db CONN] [--folder NAME] [--template NAME]")
	}
	pos := fs.positional()
	if len(pos) != 1 {
		return usagef(os.Stderr, "usage: ide env add <name> [--root PATH] [--db CONN] [--folder NAME] [--template NAME]")
	}
	name := trim(pos[0])
	if name == "" {
		return errf(os.Stderr, "name is required")
	}

	data, err := config.LoadAll()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	if findEnv(data.Environments, name) >= 0 {
		return errf(os.Stderr, "environment %q already exists", name)
	}

	env := config.Environment{
		Name:         name,
		Root:         trim(*root),
		Folder:       trim(*folder),
		DBConnection: trim(*db),
	}

	if t := trim(*template); t != "" {
		tIdx := findTemplate(data.Templates, t)
		if tIdx < 0 {
			return errf(os.Stderr, "no such template %q", t)
		}
		env.Windows = cloneWindows(data.Templates[tIdx].Windows)
	}

	data.Environments = append(data.Environments, env)
	if err := config.SaveAll(data); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("added environment %q\n", name)
	return 0
}

func envSet(args []string) int {
	fs := newFlagSet("env set")
	root := fs.string("root", "filesystem root")
	db := fs.string("db", "database connection string")
	folder := fs.string("folder", "display folder/group")
	if err := fs.parse(args); err != nil {
		return parseUsagef(os.Stderr, err, "usage: ide env set <name> [--root PATH] [--db CONN] [--folder NAME]")
	}
	pos := fs.positional()
	if len(pos) != 1 {
		return usagef(os.Stderr, "usage: ide env set <name> [--root PATH] [--db CONN] [--folder NAME]")
	}
	name := pos[0]

	envs, err := config.Load()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	idx := findEnv(envs, name)
	if idx < 0 {
		return errf(os.Stderr, "no such environment %q", name)
	}
	if fs.provided("root") {
		envs[idx].Root = trim(*root)
	}
	if fs.provided("db") {
		envs[idx].DBConnection = trim(*db)
	}
	if fs.provided("folder") {
		envs[idx].Folder = trim(*folder)
	}
	if err := config.Save(envs); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("updated environment %q\n", envs[idx].Name)
	return 0
}

func envRename(args []string) int {
	if len(args) != 2 {
		return usagef(os.Stderr, "usage: ide env rename <old> <new>")
	}
	oldName, newName := args[0], trim(args[1])
	if newName == "" {
		return errf(os.Stderr, "new name is required")
	}
	envs, err := config.Load()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	idx := findEnv(envs, oldName)
	if idx < 0 {
		return errf(os.Stderr, "no such environment %q", oldName)
	}
	if dup := findEnv(envs, newName); dup >= 0 && dup != idx {
		return errf(os.Stderr, "environment %q already exists", newName)
	}
	envs[idx].Name = newName
	if err := config.Save(envs); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("renamed %q → %q\n", oldName, newName)
	return 0
}

func envRm(args []string) int {
	if len(args) != 1 {
		return usagef(os.Stderr, "usage: ide env rm <name>")
	}
	name := args[0]
	envs, err := config.Load()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	idx := findEnv(envs, name)
	if idx < 0 {
		return errf(os.Stderr, "no such environment %q", name)
	}
	envs = append(envs[:idx], envs[idx+1:]...)
	if err := config.Save(envs); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("removed environment %q\n", name)
	return 0
}

func findEnv(envs []config.Environment, name string) int {
	name = strings.TrimSpace(name)
	for i, e := range envs {
		if strings.EqualFold(strings.TrimSpace(e.Name), name) {
			return i
		}
	}
	return -1
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func cloneWindows(in []config.WindowTemplate) []config.WindowTemplate {
	out := make([]config.WindowTemplate, len(in))
	copy(out, in)
	return out
}
