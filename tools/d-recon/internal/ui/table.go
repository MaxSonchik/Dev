package ui

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	"github.com/devos-os/d-recon/internal/core"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00CEC9")).MarginBottom(1)
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6C5CE7")).Underline(true)
	
	success    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00B894"))
	warning    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FD79A8"))
	meta       = lipgloss.NewStyle().Foreground(lipgloss.Color("#636e72"))
)

func PrintResults(hosts []core.Host) {
	for _, h := range hosts {
		// Special Case: Identity Recon (Sherlock)
		if len(h.Tags) > 0 && h.Tags[0] == "identity" {
			fmt.Printf("👤 IDENTITY FOUND: %s\n", success.Render(h.Hostname))
			continue
		}

		// Normal Host
		fmt.Println(titleStyle.Render(fmt.Sprintf("\n🎯 TARGET: %s (%s)", h.IP, h.Hostname)))
		if h.OS != "" {
			fmt.Printf("   💿 OS Detection: %s\n", h.OS)
		}
		if len(h.Tags) > 0 {
			fmt.Printf("   🏷️  Tags: %v\n", h.Tags)
		}
		
		if len(h.Ports) == 0 {
			fmt.Println("   (No open ports or data found)")
			continue
		}

		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, headerStyle.Render("PORT\tPROTO\tSTATE\tSERVICE\tVERSION\tSOURCE"))

		for _, p := range h.Ports {
			state := success.Render(p.State)
			if p.State != "open" && p.State != "DETECTED" { state = warning.Render(p.State) }
			
			source := meta.Render(p.Source)
			version := p.Version
			if version == "" { version = "-" }

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				strconv.Itoa(p.Number),
				p.Protocol,
				state,
				p.Service,
				version,
				source,
			)
		}
		w.Flush()
		fmt.Println()
	}
}