package tools

import (
	"encoding/json"
	"os/exec"

	"github.com/devos-os/d-guard/internal/core"
)

type SemgrepOutput struct {
	Results []struct {
		CheckID string `json:"check_id"`
		Path    string `json:"path"`
		Start   struct { Line int `json:"line"` } `json:"start"`
		Extra   struct { Message string `json:"message"` ; Severity string `json:"severity"` } `json:"extra"`
	} `json:"results"`
}

func RunSemgrep(root string, files []string) []core.Issue {
	bin, err := EnsureTool("semgrep")
	if err != nil { return nil }

	// semgrep scan --config=auto --json [files...]
	args := []string{"scan", "--config=auto", "--json", "--quiet"}
	
	// Если файлов мало (git hook), передаем их явно для скорости
	if len(files) > 0 && len(files) < 50 {
		args = append(args, files...)
	} else {
		args = append(args, root)
	}

	cmd := exec.Command(bin, args...)
	out, _ := cmd.Output() // Semgrep может вернуть exit 1 если нашел баги, это норм

	var report SemgrepOutput
	json.Unmarshal(out, &report)

	var issues []core.Issue
	for _, res := range report.Results {
		sev := core.SevMedium
		if res.Extra.Severity == "ERROR" { sev = core.SevHigh }
		
		issues = append(issues, core.Issue{
			Scanner:     "Semgrep SAST",
			Severity:    sev,
			Message:     res.CheckID,
			File:        res.Path,
			Line:        res.Start.Line,
			Description: res.Extra.Message,
			Suggestion:  "Check code logic",
		})
	}
	return issues
}