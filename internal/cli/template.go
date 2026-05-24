package cli

import (
	"fmt"
	"os"
	"strings"

	"ide/internal/config"
)

func dispatchTemplate(args []string) int {
	if len(args) == 0 {
		return usagef(os.Stderr, "usage: ide template <list|show|add|rename|rm|window> ...")
	}
	switch args[0] {
	case "list", "ls":
		return templateList(args[1:])
	case "show":
		return templateShow(args[1:])
	case "add", "create":
		return templateAdd(args[1:])
	case "rename", "mv":
		return templateRename(args[1:])
	case "rm", "remove", "delete":
		return templateRm(args[1:])
	case "window", "windows":
		return dispatchTemplateWindow(args[1:])
	}
	return usagef(os.Stderr, "ide: unknown template subcommand %q", args[0])
}

func templateList(args []string) int {
	if len(args) != 0 {
		return usagef(os.Stderr, "usage: ide template list")
	}
	templates, err := config.LoadTemplates()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	if len(templates) == 0 {
		fmt.Println("(no templates)")
		return 0
	}
	for _, t := range templates {
		fmt.Printf("%s\t%d windows\n", t.Name, len(t.Windows))
	}
	return 0
}

func templateShow(args []string) int {
	if len(args) != 1 {
		return usagef(os.Stderr, "usage: ide template show <name>")
	}
	templates, err := config.LoadTemplates()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	idx := findTemplate(templates, args[0])
	if idx < 0 {
		return errf(os.Stderr, "no such template %q", args[0])
	}
	t := templates[idx]
	fmt.Printf("name: %s\n", t.Name)
	fmt.Printf("windows (%d):\n", len(t.Windows))
	for i, w := range t.Windows {
		fmt.Printf("  %d. %s\tcmd=%q\tcwd=%q\n", i+1, w.Name, w.Cmd, w.Cwd)
	}
	return 0
}

func templateAdd(args []string) int {
	if len(args) != 1 {
		return usagef(os.Stderr, "usage: ide template add <name>")
	}
	name := trim(args[0])
	if name == "" {
		return errf(os.Stderr, "name is required")
	}
	templates, err := config.LoadTemplates()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	if findTemplate(templates, name) >= 0 {
		return errf(os.Stderr, "template %q already exists", name)
	}
	templates = append(templates, config.Template{Name: name, Windows: []config.WindowTemplate{{Name: "shell"}}})
	if err := config.SaveTemplates(templates); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("added template %q\n", name)
	return 0
}

func templateRename(args []string) int {
	if len(args) != 2 {
		return usagef(os.Stderr, "usage: ide template rename <old> <new>")
	}
	oldName, newName := args[0], trim(args[1])
	if newName == "" {
		return errf(os.Stderr, "new name is required")
	}
	templates, err := config.LoadTemplates()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	idx := findTemplate(templates, oldName)
	if idx < 0 {
		return errf(os.Stderr, "no such template %q", oldName)
	}
	if dup := findTemplate(templates, newName); dup >= 0 && dup != idx {
		return errf(os.Stderr, "template %q already exists", newName)
	}
	templates[idx].Name = newName
	if err := config.SaveTemplates(templates); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("renamed %q → %q\n", oldName, newName)
	return 0
}

func templateRm(args []string) int {
	if len(args) != 1 {
		return usagef(os.Stderr, "usage: ide template rm <name>")
	}
	templates, err := config.LoadTemplates()
	if err != nil {
		return errf(os.Stderr, "%v", err)
	}
	idx := findTemplate(templates, args[0])
	if idx < 0 {
		return errf(os.Stderr, "no such template %q", args[0])
	}
	templates = append(templates[:idx], templates[idx+1:]...)
	if err := config.SaveTemplates(templates); err != nil {
		return errf(os.Stderr, "%v", err)
	}
	fmt.Printf("removed template %q\n", args[0])
	return 0
}

func findTemplate(templates []config.Template, name string) int {
	name = strings.TrimSpace(name)
	for i, t := range templates {
		if strings.EqualFold(strings.TrimSpace(t.Name), name) {
			return i
		}
	}
	return -1
}
