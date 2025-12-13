package engines

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/devos-os/d-recon/internal/core"
)

type MasscanEntry struct {
	IP    string `json:"ip"`
	Ports []struct {
		Port   int    `json:"port"`
		Proto  string `json:"proto"`
		Status string `json:"status"`
	} `json:"ports"`
}

func CheckMasscan() bool {
	_, err := exec.LookPath("masscan")
	return err == nil
}

func RunMasscan(target string, ports string) ([]core.Host, error) {
	if !CheckMasscan() {
		return nil, fmt.Errorf("masscan not installed (sudo dnf install masscan)")
	}
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("masscan requires root privileges (run with sudo)")
	}

	fmt.Println("🚀 Launching Masscan...")

	args := []string{target, "-p", ports, "--rate", "1000", "-oJ", "-"}
	
	cmd := exec.Command("masscan", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("masscan failed: %v", err)
	}

	var entries []MasscanEntry
	if err := json.Unmarshal(output, &entries); err != nil {
		return nil, fmt.Errorf("masscan json parse error: %v", err)
	}

	var results []core.Host
	for _, entry := range entries {
		host := core.Host{IP: entry.IP, OS: "Unknown"}
		for _, p := range entry.Ports {
			host.AddPort(core.Port{
				Number:   p.Port,
				Protocol: p.Proto,
				State:    p.Status,
				Service:  "unknown",
				Source:   "masscan",
			})
		}
		results = append(results, host)
	}

	return results, nil
}