package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type GitData struct {
	IsRepo      bool
	Branch      string
	Hash        string
	StatusItems []string
	Graph       string
}

func Analyze(root string) GitData {
	g := GitData{}
	if _, err := os.Stat(root + "/.git"); err != nil {
		return g
	}
	g.IsRepo = true

	// 1. Basic Info
	g.Branch = run(root, "rev-parse", "--abbrev-ref", "HEAD")
	g.Hash = run(root, "rev-parse", "--short", "HEAD")

	// 2. Status Parsing (FIXED)
	statusRaw := run(root, "status", "--porcelain")
	if statusRaw != "" {
		for _, line := range strings.Split(statusRaw, "\n") {
			if len(line) < 3 { continue }
			
			// Porcelain format: "XY PATH" (XY are 2 chars status, then space)
			// Мы берем первые 2 символа как код
			code := line[:2]
			// А имя файла берем, обрезая первые 3 символа и удаляя пробелы по краям
			// Если строка короче, защищаемся
			var file string
			if len(line) > 3 {
				file = strings.TrimSpace(line[3:])
			} else {
				file = line
			}

			icon := "📝" // Modified
			if strings.Contains(code, "?") { icon = "✨" } // Untracked
			if strings.Contains(code, "A") { icon = "➕" } // Added
			if strings.Contains(code, "D") { icon = "🗑️" } // Deleted
			
			// Форматируем строку для UI
			g.StatusItems = append(g.StatusItems, fmt.Sprintf("%s %s", icon, file))
		}
	}

	// 3. Graph Visualization
	// Добавили --topo-order, чтобы линии рисовались понятнее
	// Формат: Hash - (Refs) Subject (Time)
	cmd := exec.Command("git", "log", "--graph", "--abbrev-commit", "--decorate", "--date=relative", "--format=format:%h -%d %s (%cr)", "--all", "--color=always", "--topo-order", "-n", "15")
	cmd.Dir = root
	out, _ := cmd.Output()
	g.Graph = string(out)

	return g
}

func run(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}