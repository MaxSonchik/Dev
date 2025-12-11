package ui

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devos-os/d-ci/internal/domain"
	"github.com/devos-os/d-ci/internal/ui/styles"
)

type Model struct {
	provider  domain.Provider
	pipelines map[string]domain.Pipeline
	sub       <-chan domain.PipelineEvent
	spinner   spinner.Model
	quitting  bool
	err       error
}

func NewModel(p domain.Provider) Model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(styles.Yellow)
	return Model{
		provider:  p,
		pipelines: make(map[string]domain.Pipeline),
		sub:       p.Subscribe(),
		spinner:   s,
	}
}

// Вспомогательное сообщение для событий
type waitForUpdateMsg domain.PipelineEvent

func (m Model) listen() tea.Cmd {
	return func() tea.Msg {
		return waitForUpdateMsg(<-m.sub)
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.listen())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case waitForUpdateMsg:
		event := domain.PipelineEvent(msg)
		switch event.Type {
		case "NEW", "UPDATE":
			m.pipelines[event.Pipeline.ID] = event.Pipeline
		case "DELETE":
			delete(m.pipelines, event.Pipeline.ID)
		}
		return m, m.listen()
	case error:
		m.err = msg
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "Bye!\n"
	}
	if m.err != nil {
		return styles.ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}
	
	s := "\n DevOS CI Monitor v2.0\n\n"

	if len(m.pipelines) == 0 {
		s += " Waiting for pipelines...\n"
	}

	var ids []string
	for id := range m.pipelines { ids = append(ids, id) }
	sort.Strings(ids)

	for _, id := range ids {
		p := m.pipelines[id]

		// Header
		header := fmt.Sprintf("%s  %s  %s",
			styles.ProjectStyle.Render(p.Project),
			styles.BranchStyle.Render(" "+p.Branch),
			lipgloss.NewStyle().Foreground(styles.Gray).Render(p.CommitMsg),
		)
		s += " " + header + "\n"
		
		// Jobs Flow
		var jobsView []string
		for i, job := range p.Jobs {
			var color lipgloss.Color
			var icon string
			
			switch job.Status {
			case domain.StatusSuccess:
				color = styles.Green
				icon = "✔"
			case domain.StatusFailed:
				color = styles.Red
				icon = "✖"
			case domain.StatusRunning:
				color = styles.Yellow
				icon = m.spinner.View()
			default:
				color = styles.Gray
				icon = "○"
			}
			
			// ИСПРАВЛЕНИЕ: Передаем color напрямую, без GetForeground()
			style := styles.JobBoxStyle.Copy().BorderForeground(color).Foreground(color)
			jobsView = append(jobsView, style.Render(fmt.Sprintf("%s %s", icon, job.Name)))
			
			if i < len(p.Jobs)-1 {
				jobsView = append(jobsView, styles.ArrowStyle.Render(" ──▶ "))
			}
		}
		s += " " + lipgloss.JoinHorizontal(lipgloss.Center, jobsView...) + "\n\n"
	}

	s += "\n " + styles.FooterStyle.Render("Press 'q' to quit")
	return s
}