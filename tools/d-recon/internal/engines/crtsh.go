package engines

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/devos-os/d-recon/internal/core"
)

type CrtEntry struct {
	NameValue string `json:"name_value"`
}

func RunCrtSh(domain string) ([]core.Host, error) {
	fmt.Printf("📡 Querying CRT.sh for subdomains of %s...\n", domain)

	url := fmt.Sprintf("https://crt.sh/?q=%%.%s&output=json", domain)
	client := http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var entries []CrtEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		// Иногда crt.sh возвращает HTML при ошибке или перегрузке
		return nil, fmt.Errorf("crt.sh parsing failed (api might be busy)")
	}

	// Дедупликация
	uniqueDomains := make(map[string]bool)
	var results []core.Host

	for _, e := range entries {
		if !uniqueDomains[e.NameValue] {
			uniqueDomains[e.NameValue] = true
			results = append(results, core.Host{
				Hostname: e.NameValue,
				Tags:     []string{"subdomain", "crt.sh"},
			})
		}
	}

	return results, nil
}