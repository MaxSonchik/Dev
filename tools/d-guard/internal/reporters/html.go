package reporters

import (
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/devos-os/d-guard/internal/core"
)

const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
	<title>DevOS Security Report</title>
	<style>
		body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background: #f4f4f9; padding: 40px; }
		.container { max-width: 900px; margin: 0 auto; }
		.header { background: linear-gradient(135deg, #6c5ce7, #a29bfe); color: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
		.issue { background: white; margin: 15px 0; padding: 20px; border-left: 6px solid #ccc; border-radius: 4px; box-shadow: 0 2px 5px rgba(0,0,0,0.05); }
		.CRITICAL { border-left-color: #ff4757; }
		.HIGH { border-left-color: #ffa502; }
		.MEDIUM { border-left-color: #eccc68; }
		.LOW { border-left-color: #70a1ff; }
		h3 { margin-top: 0; }
		.meta { color: #666; font-size: 0.9em; font-family: monospace; background: #eee; padding: 2px 5px; border-radius: 3px; }
		.suggestion { background: #e3f2fd; padding: 10px; border-radius: 4px; margin-top: 10px; color: #0d47a1; }
	</style>
</head>
<body>
<div class="container">
	<div class="header">
		<h1>🛡️ d-guard Scan Report</h1>
		<p><strong>Generated:</strong> {{ .Date }} | <strong>Issues Found:</strong> {{ .Count }}</p>
	</div>
	{{ range .Issues }}
	<div class="issue {{ .Severity }}">
		<h3>[{{ .Severity }}] {{ .Scanner }}: {{ .Message }}</h3>
		<p>📍 Location: <span class="meta">{{ .File }}:{{ .Line }}</span></p>
		<p>{{ .Description }}</p>
		{{ if .Suggestion }}
		<div class="suggestion"><strong>💡 Fix:</strong> {{ .Suggestion }}</div>
		{{ end }}
	</div>
	{{ end }}
</div>
</body>
</html>
`

func GenerateHTML(issues []core.Issue, filename string) {
	t, _ := template.New("report").Parse(htmlTemplate)
	f, _ := os.Create(filename)
	defer f.Close()

	data := struct {
		Date   string
		Count  int
		Issues []core.Issue
	}{
		Date:   time.Now().Format(time.RFC822),
		Count:  len(issues),
		Issues: issues,
	}

	t.Execute(f, data)
	fmt.Printf("\n📄 HTML Report generated: %s\n", filename)
}