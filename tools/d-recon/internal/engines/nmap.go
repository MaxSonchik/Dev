package engines

import (
	"encoding/xml"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/devos-os/d-recon/internal/core"
)

// Структуры для XML маппинга
type NmapRun struct {
	Hosts []NmapHost `xml:"host"`
}
type NmapHost struct {
	Addresses []NmapAddr  `xml:"address"`
	Hostnames NmapNames   `xml:"hostnames"`
	Ports     NmapPorts   `xml:"ports"`
	Os        NmapOs      `xml:"os"`
}
type NmapAddr struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}
type NmapNames struct {
	Hostnames []struct{ Name string `xml:"name,attr"` } `xml:"hostname"`
}
type NmapPorts struct {
	Ports []struct {
		Protocol string `xml:"protocol,attr"`
		PortID   string `xml:"portid,attr"`
		State    struct { State string `xml:"state,attr"` } `xml:"state"`
		Service  struct {
			Name    string `xml:"name,attr"`
			Product string `xml:"product,attr"`
			Version string `xml:"version,attr"`
		} `xml:"service"`
	} `xml:"port"`
}
type NmapOs struct {
	OsMatch []struct { Name string `xml:"name,attr"` } `xml:"osmatch"`
}

func RunNmap(target string, flags string) ([]core.Host, error) {
	// Аргументы: -oX - (вывод XML в stdout)
	// Добавляем пользовательские флаги, но форсируем XML
	args := []string{"-oX", "-", target}
	if flags != "" {
		// Внимание: это упрощенно. В проде нужен полноценный парсер флагов.
		// Но для MVP мы просто добавляем стандартные опции, если флагов нет.
	} else {
		// Дефолтный "умный" скан: версии сервисов, OS detection
		args = append([]string{"-sV", "-O", "-T4"}, args...)
	}

	cmd := exec.Command("nmap", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nmap exec failed: %v", err)
	}

	var nmapRun NmapRun
	if err := xml.Unmarshal(output, &nmapRun); err != nil {
		return nil, fmt.Errorf("xml parse error: %v", err)
	}

	// Конвертация в наш формат
	var results []core.Host
	for _, h := range nmapRun.Hosts {
		host := core.Host{IP: getIP(h)}
		if len(h.Hostnames.Hostnames) > 0 {
			host.Hostname = h.Hostnames.Hostnames[0].Name
		}
		if len(h.Os.OsMatch) > 0 {
			host.OS = h.Os.OsMatch[0].Name
		}

		for _, p := range h.Ports.Ports {
			if p.State.State == "open" {
				portNum, _ := strconv.Atoi(p.PortID)
				version := p.Service.Product
				if p.Service.Version != "" {
					version += " " + p.Service.Version
				}
				
				host.AddPort(core.Port{
					Number:   portNum,
					Protocol: p.Protocol,
					State:    p.State.State,
					Service:  p.Service.Name,
					Version:  version,
					Source:   "nmap",
				})
			}
		}
		results = append(results, host)
	}
	return results, nil
}

func getIP(h NmapHost) string {
	for _, a := range h.Addresses {
		if a.AddrType == "ipv4" { return a.Addr }
	}
	return ""
}