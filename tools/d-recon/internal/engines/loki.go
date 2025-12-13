package engines

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	"github.com/devos-os/d-recon/internal/core"
)

func RunLoki(path string) ([]core.Host, error) {
	if _, err := exec.LookPath("loki"); err != nil {
		return nil, fmt.Errorf("loki not found (check /usr/local/bin/loki wrapper)")
	}

	fmt.Printf("🛡️  Scanning for IOCs in %s via Loki (this may take time)...\n", path)

	// Убрали --noindicator, чтобы видеть прогресс, если запускаем руками
	// Добавили --noprocscan, чтобы не сканировать RAM (нужен root)
	// --dontwait чтобы не ждал нажатия клавиши
	cmd := exec.Command("loki", "-p", path, "--noprocscan", "--dontwait", "--only-relevant")
	
	// Читаем Stdout в реальном времени
	stdout, _ := cmd.StdoutPipe()
	cmd.Start()

	var results []core.Host
	scanner := bufio.NewScanner(stdout)
	
	host := core.Host{
		IP:       "LOCAL-FS",
		Hostname: path,
		Tags:     []string{"threat-intel"},
	}
	found := false

	for scanner.Scan() {
		line := scanner.Text()
		
		// Логика обнаружения
		if strings.Contains(line, "ALERT:") || strings.Contains(line, "WARNING:") {
			// Выводим сразу в консоль, чтобы пользователь не скучал
			fmt.Printf("  🚨 %s\n", line)
			
			parts := strings.Split(line, ":")
			msg := strings.TrimSpace(line)
			if len(parts) > 1 {
				msg = strings.TrimSpace(parts[1])
			}
			
			host.AddPort(core.Port{
				Service: "IOC",
				Version: msg,
				State:   "DETECTED",
				Source:  "loki",
			})
			found = true
		}
	}
	cmd.Wait()

	if found {
		results = append(results, host)
	}

	return results, nil
}