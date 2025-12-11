package tools

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/devos-os/d-guard/internal/core"
)

type GitleaksResult struct {
	Description string `json:"Description"`
	File        string `json:"File"`
	StartLine   int    `json:"StartLine"`
	Secret      string `json:"Secret"`
	Match       string `json:"Match"`
	RuleID      string `json:"RuleID"`
}

func RunGitleaks(root string, files []string) []core.Issue {
	bin, err := EnsureTool("gitleaks")
	if err != nil {
		// Fallback на нативный сканер, если нет gitleaks
		return nil 
	}

	// Создаем временный файл для отчета
	tmpReport := "gitleaks-report.json"
	defer os.Remove(tmpReport)

	// Запуск: gitleaks detect --source . --no-git --report-path ...
	// --no-git используем для проверки текущего состояния файлов (unstaged/untracked)
	cmd := exec.Command(bin, "detect", "--source", root, "--no-git", "--report-path", tmpReport, "--exit-code", "0")
	cmd.Run() // Игнорируем ошибку exit code, так как нам нужен отчет

	// Читаем отчет
	data, err := os.ReadFile(tmpReport)
	if err != nil { return nil }

	var results []GitleaksResult
	json.Unmarshal(data, &results)

	var issues []core.Issue
	for _, res := range results {
		// Фильтруем, если нам нужны только конкретные файлы (для git hook)
		// Если files пустой - значит полный скан
		if len(files) > 0 {
			found := false
			for _, f := range files {
				if f == res.File || f == (root+"/"+res.File) { found = true; break }
			}
			if !found { continue }
		}

		issues = append(issues, core.Issue{
			Scanner:     "Gitleaks",
			Severity:    core.SevCritical,
			Message:     res.Description,
			File:        res.File,
			Line:        res.StartLine,
			Description: "Secret detected: " + res.RuleID,
			Suggestion:  "Revoke this secret immediately.",
		})
	}
	return issues
}