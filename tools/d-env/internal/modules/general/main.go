package general

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type GeneralData struct {
	Stacks      []Stack
	HealthScore int
	Risks       []string
	Tree        string
}

type Stack struct { Name, Version, Color string }

func Analyze(root string) GeneralData {
	g := GeneralData{HealthScore: 100}
	
	// Используем map для дедупликации: "Name+Version" -> bool
	seenStacks := make(map[string]bool)

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil { return nil }
		if info.IsDir() && (info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "target" || info.Name() == "dist") { 
			return filepath.SkipDir 
		}
		
		var s Stack
		found := false

		if info.Name() == "go.mod" { s = Stack{"Go", "1.x", "#00ADD8"}; found = true }
		if info.Name() == "package.json" { s = Stack{"Node.js", "?", "#87D75F"}; found = true }
		if info.Name() == "requirements.txt" { s = Stack{"Python", "Pip", "#3776AB"}; found = true }
		if info.Name() == "Cargo.toml" { s = Stack{"Rust", "Cargo", "#DEA584"}; found = true }

		// Если нашли стек и его еще не было
		if found {
			key := s.Name + s.Version
			if !seenStacks[key] {
				g.Stacks = append(g.Stacks, s)
				seenStacks[key] = true
			}
		}

		if info.Name() == ".env" {
			g.HealthScore -= 20
			gitignore, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
			if !strings.Contains(string(gitignore), ".env") {
				g.HealthScore -= 30
				g.Risks = append(g.Risks, "CRITICAL: .env file exists and is NOT ignored in git!")
			} else {
				g.Risks = append(g.Risks, "Warning: .env file found (local config)")
			}
		}
		return nil
	})

	if len(g.Stacks) == 0 {
		g.HealthScore -= 10
		g.Risks = append(g.Risks, "No standard project configuration found")
	}

	g.Tree = generateTree(root, "", true, 0)
	return g
}

func generateTree(path string, prefix string, isRoot bool, depth int) string {
	if depth > 2 { return "" } // Уменьшили глубину до 2 для компактности
	var sb strings.Builder
	files, _ := os.ReadDir(path)
	
	filtered := []os.DirEntry{}
	for _, f := range files {
		if f.Name() != ".git" && f.Name() != "node_modules" && f.Name() != "target" { filtered = append(filtered, f) }
	}

	for i, f := range filtered {
		isLast := i == len(filtered)-1
		pointer := "├──"
		if isLast { pointer = "└──" }
		
		icon := "📄"
		if f.IsDir() { icon = "📂" }
		
		sb.WriteString(fmt.Sprintf("%s%s %s %s\n", prefix, pointer, icon, f.Name()))

		if f.IsDir() {
			newPrefix := prefix + "│   "
			if isLast { newPrefix = prefix + "    " }
			sb.WriteString(generateTree(filepath.Join(path, f.Name()), newPrefix, false, depth+1))
		}
	}
	return sb.String()
}