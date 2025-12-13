package engines

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/devos-os/d-recon/internal/core"
)

type BBOTEvent struct {
	Type string `json:"type"` 
	Data string `json:"data"`
}

func RunBBOT(target string) ([]core.Host, error) {
	if _, err := exec.LookPath("bbot"); err != nil {
		return nil, fmt.Errorf("bbot not installed")
	}

	fmt.Println("🕷️  Unleashing BBOT (Passive OSINT)...")
	fmt.Println("   (Excluding heavy modules for speed: jadx, extractous, trufflehog)")

	// ИСПРАВЛЕНИЕ: Заменили -e на -em (exclude modules)
	args := []string{
		"-t", target, 
		"-f", "subdomain-enum", 
		"--flags", "passive", 
		"-em", "jadx", "extractous", "trufflehog", "social", 
		"-y", 
		"--output-modules", "json", 
		"-o", "-",
	}

	cmd := exec.Command("bbot", args...)
	
	// Оставляем stderr пользователю
	cmd.Stderr = os.Stderr 
	
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start bbot: %v", err)
	}

	var results []core.Host
	scanner := bufio.NewScanner(stdout)
	
	// Увеличиваем буфер
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		
		if !strings.HasPrefix(line, "{") { continue }

		var event BBOTEvent
		if err := json.Unmarshal([]byte(line), &event); err == nil {
			if event.Type == "DNS_NAME" {
				results = append(results, core.Host{
					Hostname: event.Data,
					Tags:     []string{"subdomain", "bbot"},
				})
			} else if event.Type == "IP_ADDRESS" {
				results = append(results, core.Host{
					IP:   event.Data,
					Tags: []string{"ip", "bbot"},
				})
			}
		}
	}
	cmd.Wait()

	return results, nil
}