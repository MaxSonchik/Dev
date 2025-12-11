package code

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"github.com/devos-os/d-guard/internal/core"
)

func Scan(files []string) []core.Issue {
	var issues []core.Issue
	// Regex: ищет localhost, 127.0.0.1, 0.0.0.0, игнорируя комментарии
	re := regexp.MustCompile(`(?i)(https?://)?(localhost|127\.0\.0\.1|0\.0\.0\.0)`)

	for _, path := range files {
		// Игнорируем сам код d-guard (рекурсивная проблема)
		if strings.Contains(path, "internal/modules/code/scanner.go") { continue }
		if strings.Contains(path, ".git") || strings.HasSuffix(path, ".sum") { continue }

		f, err := os.Open(path)
		if err != nil { continue }
		
		scanner := bufio.NewScanner(f)
		lineNum := 0
		
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			trim := strings.TrimSpace(line)
			
			// Пропускаем комментарии
			if strings.HasPrefix(trim, "//") || strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, "*") { continue }
			// Пропускаем строки объявления regex (чтобы не находить саму проверку)
			if strings.Contains(line, "regexp.MustCompile") { continue }

			if re.MatchString(line) {
				issues = append(issues, core.Issue{
					Scanner:     "Code Quality",
					Severity:    core.SevLow,
					Message:     "Hardcoded local address detected",
					File:        path,
					Line:        lineNum,
					Suggestion:  "Use environment variables (e.g. OS_HOST) instead of hardcoding localhost",
				})
			}
		}
		f.Close()
	}
	return issues
}