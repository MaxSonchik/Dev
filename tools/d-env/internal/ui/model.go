package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devos-os/d-env/internal/analyzer"
)

// --- Styling ---
var (
	primary   = lipgloss.Color("#6C5CE7") // DevOS Purple
	secondary = lipgloss.Color("#00CEC9") // Cyan
	accent    = lipgloss.Color("#FD79A8") // Pink (ТОТ САМЫЙ АКЦЕНТ)
	text      = lipgloss.Color("#DFE6E9")
	dark      = lipgloss.Color("#2d3436")
	
	headerStyle = lipgloss.NewStyle().Background(primary).Foreground(text).Bold(true).Padding(0, 1)
	
	// Tabs
	tabStyle    = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("#636e72"))
	activeTab   = lipgloss.NewStyle().Padding(0, 2).Foreground(primary).Bold(true).Underline(true)
	
	// Boxes & Titles
	boxTitle = lipgloss.NewStyle().Foreground(secondary).Bold(true).Underline(true)
	dockerBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(secondary).Padding(0, 1).Width(50)
	svcBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).MarginRight(1).BorderForeground(accent)
	jobBox = lipgloss.NewStyle().Background(dark).Foreground(text).Padding(0, 2).MarginRight(1).Bold(true)
)

type model struct {
	data      analyzer.Report
	activeTab int
	viewport  viewport.Model
	ready     bool
	quitting  bool
}

func InitialModel(path string) model { return model{data: analyzer.Analyze(path)} }
func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k := msg.String()
		if k == "q" || k == "ctrl+c" { return m, tea.Quit }
		if k == "tab" || k == "right" { m.activeTab = (m.activeTab + 1) % 4; m.viewport.SetContent(m.render()) }
		if k == "left" || k == "shift+tab" { m.activeTab--; if m.activeTab < 0 { m.activeTab = 3 }; m.viewport.SetContent(m.render()) }
	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-6)
			m.viewport.SetContent(m.render())
			m.ready = true
		} else { m.viewport.Width = msg.Width; m.viewport.Height = msg.Height - 6 }
	}
	m.viewport, _ = m.viewport.Update(msg)
	return m, nil
}

func (m model) View() string {
	if !m.ready { return "Scanning project..." }
	
	tabs := []string{"1. MAIN", "2. GIT", "3. DOCKER", "4. INFRA"}
	var row []string
	for i, t := range tabs {
		if i == m.activeTab { row = append(row, activeTab.Render(t)) } else { row = append(row, tabStyle.Render(t)) }
	}
	
	return fmt.Sprintf("%s\n%s\n%s", 
		headerStyle.Render(" DevOS Project MRI 1.5 "), 
		lipgloss.JoinHorizontal(lipgloss.Top, row...),
		m.viewport.View())
}

func (m model) render() string {
	var s strings.Builder
	d := m.data

	switch m.activeTab {
	// --- TAB 1: MAIN ---
	case 0:
		s.WriteString(fmt.Sprintf("\n📊 Project Health: %d/100\n", d.General.HealthScore))
		if len(d.General.Risks) > 0 {
			s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ff7675")).Render("⚠️  RISKS:") + "\n")
			for _, r := range d.General.Risks { s.WriteString(fmt.Sprintf(" - %s\n", r)) }
		}
		
		s.WriteString("\n" + boxTitle.Render("DETECTED STACK") + "\n")
		for _, st := range d.General.Stacks {
			s.WriteString(fmt.Sprintf(" • %s (%s)\n", lipgloss.NewStyle().Foreground(lipgloss.Color(st.Color)).Render(st.Name), st.Version))
		}
		
		s.WriteString("\n" + boxTitle.Render("FILE TREE") + "\n")
		s.WriteString(d.General.Tree)

	// --- TAB 2: GIT ---
	case 1:
		if !d.Git.IsRepo { return "\nNot a git repository." }
		s.WriteString(fmt.Sprintf("\n🌿 Branch: %s  |  commit: %s\n", d.Git.Branch, d.Git.Hash))
		
		s.WriteString("\n" + boxTitle.Render("COMMIT HISTORY (Graph)") + "\n")
		s.WriteString(d.Git.Graph) // ASCII graph rendered by git log
		
		s.WriteString("\n" + boxTitle.Render("STATUS") + "\n")
		for _, item := range d.Git.StatusItems { s.WriteString(item + "\n") }

	// --- TAB 3: DOCKER ---
	case 2:
		if !d.Docker.Found { return "\nNo Docker found." }
		
		// Dockerfile
		if d.Docker.Dockerfile.Found {
			s.WriteString("\n" + boxTitle.Render("BUILD PIPELINE") + "\n\n")
			for i, stage := range d.Docker.Dockerfile.Stages {
				header := fmt.Sprintf("🏗️  STAGE %d: %s", i+1, stage.Name)
				if stage.IsFinal { header += " (FINAL)" }
				
				content := fmt.Sprintf("Base: %s\n", stage.BaseImage)
				for _, link := range stage.Links {
					content += lipgloss.NewStyle().Foreground(accent).Render(fmt.Sprintf("⬅ From %s", link)) + "\n"
				}
				
				s.WriteString(dockerBox.Render(header + "\n" + content))
				if i < len(d.Docker.Dockerfile.Stages)-1 { s.WriteString("\n      ⬇\n") }
			}
		}
		
		// Compose
		if d.Docker.Compose.Found {
			s.WriteString("\n\n" + boxTitle.Render("COMPOSE TOPOLOGY") + "\n\n")
			var boxes []string
			for _, svc := range d.Docker.Compose.Services {
				info := fmt.Sprintf("📦 %s\nImage: %s", svc.Name, svc.Image)
				if len(svc.Links) > 0 {
					info += fmt.Sprintf("\n\nNeeds:\n%s", strings.Join(svc.Links, "\n"))
				}
				boxes = append(boxes, svcBox.Render(info))
			}
			s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, boxes...))
		}

	// --- TAB 4: INFRA ---
	case 3:
		s.WriteString("\n" + boxTitle.Render("INFRASTRUCTURE RESOURCES") + "\n")
		
		if len(d.Infra.K8sObjs) > 0 {
			s.WriteString(lipgloss.NewStyle().Foreground(secondary).Render("Kubernetes:") + "\n")
			for _, k := range d.Infra.K8sObjs { s.WriteString(" ☸️  " + k + "\n") }
		}
		if len(d.Infra.TfRes) > 0 {
			s.WriteString(lipgloss.NewStyle().Foreground(secondary).Render("Terraform:") + "\n")
			for _, t := range d.Infra.TfRes { s.WriteString(" 🏗️  " + t + "\n") }
		}

		s.WriteString("\n" + boxTitle.Render("CI/CD PIPELINE") + "\n")
		if d.Infra.CiSystem != "" {
			s.WriteString(fmt.Sprintf("System: %s\n\n", d.Infra.CiSystem))
			
			// Render Graph Levels
			for i, level := range d.Infra.CiGraph {
				var jobs []string
				for _, job := range level {
					jobs = append(jobs, jobBox.Render(job))
				}
				// Render parallel jobs horizontally
				s.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, jobs...))
				
				// Arrow down
				if i < len(d.Infra.CiGraph)-1 {
					s.WriteString("\n      ⬇ (depends on)\n")
				}
			}
		} else {
			s.WriteString("No CI/CD configuration detected.")
		}
	}
	return s.String()
}