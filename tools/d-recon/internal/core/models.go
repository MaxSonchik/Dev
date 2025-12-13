package core

// Host представляет один IP или Домен
type Host struct {
	IP       string
	Hostname string
	Ports    []Port
	OS       string
	Tags     []string
}

type Port struct {
	Number   int
	Protocol string // tcp/udp
	Service  string // http, ssh
	Version  string // nginx 1.14.2
	State    string // open, filtered
	Source   string // nmap, shodan
}

// Result - результат сканирования
type Result struct {
	Hosts []Host
}

func (h *Host) AddPort(p Port) {
	// Простая дедупликация
	for i, existing := range h.Ports {
		if existing.Number == p.Number && existing.Protocol == p.Protocol {
			// Если новый источник точнее (nmap > shodan), обновляем
			if p.Source == "nmap" {
				h.Ports[i] = p
			}
			return
		}
	}
	h.Ports = append(h.Ports, p)
}