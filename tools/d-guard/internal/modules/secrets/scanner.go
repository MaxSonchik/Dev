package secrets

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/devos-os/d-guard/internal/core"
)

var rules = []struct {
	Name        string
	Regex       string
	Description string
}{
	{
		Name:        "Generic API Key",
		// Улучшенный regex: ищет assignment с длинной строкой
		Regex:       `(?i)(api_key|apikey|secret|token)["']?\s*[:=]\s*["'][a-zA-Z0-9_=-]{20,}["']`,
		Description: "Detected potential hardcoded API key",
	},
	{
		Name:        "Private Key",
		Regex:       `-----BEGIN ((EC|PGP|DSA|RSA|OPENSSH) )?PRIVATE KEY( BLOCK)?-----`,
		Description: "Found private key block",
	},
}

func Scan(files []string) []core.Issue {
	var issues []core.Issue

	for _, path := range files {
		// Игнорируем сам бинарник d-guard и go.sum/mod
		if strings.HasSuffix(path, "/d-guard") && !strings.HasSuffix(path, ".go") { continue }
		if strings.HasSuffix(path, "go.sum") || strings.HasSuffix(path, "go.mod") { continue }
		// Игнорируем папку .git
		if strings.Contains(path, "/.git/") { continue }

		f, err := os.Open(path)
		if err != nil { continue }
		
		scanner := bufio.NewScanner(f)
		lineNum := 0
		
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			
			// Проверка на длину строки (чтобы не сканировать минифицированный JS/JSON)
			if len(line) > 1000 { continue }

			for _, rule := range rules {
				re := regexp.MustCompile(rule.Regex)
				if re.MatchString(line) {
					issues = append(issues, core.Issue{
						Scanner:     "Secrets",
						Severity:    core.SevCritical,
						Message:     fmt.Sprintf("%s found", rule.Name),
						File:        path,
						Line:        lineNum,
						Description: rule.Description,
						Suggestion:  "Move secret to environment variables or Vault",
					})
				}
			}
		}
		f.Close()
	}
	return issues
}