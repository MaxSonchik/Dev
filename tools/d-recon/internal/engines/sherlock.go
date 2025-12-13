package engines

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	"github.com/devos-os/d-recon/internal/core"
)

// RunSherlock ищет никнейм в соцсетях
func RunSherlock(username string) ([]core.Host, error) {
	if _, err := exec.LookPath("sherlock"); err != nil {
		return nil, fmt.Errorf("sherlock not installed (pip install sherlock-project)")
	}

	fmt.Printf("🕵️  Hunting username '%s' via Sherlock...\n", username)

	// --timeout 1 --print-found (только найденные)
	cmd := exec.Command("sherlock", username, "--timeout", "1", "--print-found")
	stdout, _ := cmd.StdoutPipe()
	cmd.Start()

	var results []core.Host
	// Sherlock не возвращает IP, он возвращает URL профилей.
	// Мы упакуем их в структуру Host для отчета.
	
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "[+]") {
			url := strings.TrimSpace(strings.TrimPrefix(line, "[+]"))
			// Добавляем как "хост" для отображения в отчете
			results = append(results, core.Host{
				Hostname: url,
				Tags:     []string{"identity", "sherlock"},
				OS:       "Social Profile",
			})
			fmt.Printf("  ✅ Found: %s\n", url)
		}
	}
	cmd.Wait()
	return results, nil
}