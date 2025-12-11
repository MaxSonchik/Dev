package external

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/devos-os/d-guard/internal/core"
)

// Структуры для парсинга JSON вывода Trivy
type TrivyReport struct {
	Results []struct {
		Target          string `json:"Target"`
		Vulnerabilities []struct {
			VulnerabilityID  string `json:"VulnerabilityID"`
			PkgName          string `json:"PkgName"`
			InstalledVersion string `json:"InstalledVersion"`
			FixedVersion     string `json:"FixedVersion"`
			Severity         string `json:"Severity"`
			Description      string `json:"Description"`
		} `json:"Vulnerabilities"`
		Misconfigurations []struct {
			ID          string `json:"ID"`
			Title       string `json:"Title"`
			Severity    string `json:"Severity"`
			Description string `json:"Description"`
			Message     string `json:"Message"`
		} `json:"Misconfigurations"`
	} `json:"Results"`
}

func RunTrivyFs(root string) []core.Issue {
	var issues []core.Issue

	// Проверяем наличие trivy в системе
	if _, err := exec.LookPath("trivy"); err != nil {
		fmt.Println("⚠️  Trivy not found (install via 'dnf install trivy' or curl). Skipping SCA.")
		return nil
	}

	fmt.Println("[Orchestrator] Executing Trivy (External Security Scanner)...")

	// Запускаем: trivy fs . --format json --scanners vuln,config
	cmd := exec.Command("trivy", "fs", ".", "--format", "json", "--scanners", "vuln,config")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("⚠️  Trivy execution failed: %v\n", err)
		return nil
	}

	var report TrivyReport
	if err := json.Unmarshal(out, &report); err != nil {
		fmt.Println("⚠️  Failed to parse Trivy output")
		return nil
	}

	// Конвертируем результаты Trivy в формат d-guard
	for _, res := range report.Results {
		// 1. CVE (Уязвимости)
		for _, vuln := range res.Vulnerabilities {
			issues = append(issues, core.Issue{
				Scanner:     "Trivy (Vuln)",
				Severity:    mapSeverity(vuln.Severity),
				Message:     fmt.Sprintf("%s: %s (%s)", vuln.VulnerabilityID, vuln.PkgName, vuln.InstalledVersion),
				File:        res.Target,
				Line:        1, // Trivy часто не дает строку для зависимостей
				Description: vuln.Description,
				Suggestion:  fmt.Sprintf("Update to version %s", vuln.FixedVersion),
			})
		}
		// 2. Misconfigurations (IaC)
		for _, mis := range res.Misconfigurations {
			issues = append(issues, core.Issue{
				Scanner:     "Trivy (IaC)",
				Severity:    mapSeverity(mis.Severity),
				Message:     mis.Title,
				File:        res.Target,
				Line:        1,
				Description: mis.Description,
				Suggestion:  mis.Message,
			})
		}
	}

	return issues
}

func mapSeverity(s string) core.Severity {
	switch s {
	case "CRITICAL": return core.SevCritical
	case "HIGH": return core.SevHigh
	case "MEDIUM": return core.SevMedium
	default: return core.SevLow
	}
}