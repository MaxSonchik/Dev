package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type InfraModel struct {
	table      table.Model
	client     *client.Client
	containers []types.Container
	err        error
	width      int
	height     int
}

func NewInfraModel() InfraModel {
	columns := []table.Column{
		{Title: "ID", Width: 12},
		{Title: "Image", Width: 30},
		{Title: "Name", Width: 20},
		{Title: "Status", Width: 20},
		{Title: "State", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	cli, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	m := InfraModel{
		table:  t,
		client: cli,
	}
	m.Refresh()
	return m
}

func (m *InfraModel) Refresh() {
	if m.client == nil { return }
	
	containers, err := m.client.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		m.err = err
		return
	}
	m.containers = containers

	var rows []table.Row
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 { name = strings.TrimPrefix(c.Names[0], "/") }
		
		rows = append(rows, table.Row{
			c.ID[:10],
			c.Image,
			name,
			c.Status,
			c.State,
		})
	}
	m.table.SetRows(rows)
}

func (m InfraModel) Init() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type tickMsg time.Time

func (m InfraModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, nil // Signal to parent to go back
		case "s": // Start
			if row := m.table.SelectedRow(); row != nil {
				m.client.ContainerStart(context.Background(), getFullID(m.containers, row[0]), types.ContainerStartOptions{})
				m.Refresh()
			}
		case "x": // Stop
			if row := m.table.SelectedRow(); row != nil {
				timeout := 2
				opts := container.StopOptions{Timeout: &timeout}
				m.client.ContainerStop(context.Background(), getFullID(m.containers, row[0]), opts)
				m.Refresh()
			}
		case "r": // Restart
			if row := m.table.SelectedRow(); row != nil {
				timeout := 2
				// ИСПРАВЛЕНИЕ: В SDK v24 ContainerRestart использует StopOptions
				opts := container.StopOptions{Timeout: &timeout}
				m.client.ContainerRestart(context.Background(), getFullID(m.containers, row[0]), opts)
				m.Refresh()
			}
		}
	case tickMsg:
		m.Refresh()
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	}
	
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m InfraModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Docker Error: %v\n(Is docker socket accessible?)", m.err)
	}
	return baseStyle.Render(m.table.View()) + "\n s: start | x: stop | r: restart | esc: back"
}

func getFullID(containers []types.Container, shortID string) string {
	for _, c := range containers {
		if strings.HasPrefix(c.ID, shortID) { return c.ID }
	}
	return shortID
}