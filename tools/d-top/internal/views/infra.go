package views

import (
	"context"
	"fmt"
	"io" // <--- ДОБАВЛЕН ИМПОРТ
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// --- Стили ---
var (
	colorBorder = lipgloss.Color("62")
	colorActive = lipgloss.Color("205")
	colorGray   = lipgloss.Color("240")

	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	styleTitle = lipgloss.NewStyle().
			Background(colorBorder).
			Foreground(lipgloss.Color("#FFF")).
			Bold(true).
			Padding(0, 1)

	styleLogBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(colorActive).
			Padding(1, 2)
)

// --- Состояния ---
type infraState int
const (
	viewDashboard infraState = iota
	viewLogs
)

// --- Модель ---
type InfraModel struct {
	state      infraState
	client     *client.Client
	containers []types.Container
	selectedID string 
	
	table      table.Model
	details    viewport.Model 
	logs       viewport.Model 
	
	err        error
	statusMsg  string 
	width      int
	height     int
}

// Сообщения
type DockerReadyMsg struct { Client *client.Client; Path string }
type DockerErrorMsg error
type DockerDataMsg []types.Container
type DockerLogMsg string 
type execFinishedMsg struct{ err error }

func NewInfraModel() *InfraModel {
	columns := []table.Column{
		{Title: "State", Width: 4},
		{Title: "Name", Width: 20},
		{Title: "Image", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorGray).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return &InfraModel{
		table:     t,
		details:   viewport.New(0, 0),
		logs:      viewport.New(0, 0),
		state:     viewDashboard,
		statusMsg: "Initializing...",
		width:     100, 
		height:    30,
	}
}

// --- Логика Docker ---

func connectDocker() tea.Cmd {
	return func() tea.Msg {
		uid := os.Getuid()
		candidates := []string{
			fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", uid),
			"unix:///var/run/docker.sock",
			"unix:///run/podman/podman.sock",
		}
		
		if env := os.Getenv("DOCKER_HOST"); env != "" {
			candidates = append([]string{env}, candidates...)
		}

		for _, host := range candidates {
			socketPath := strings.TrimPrefix(host, "unix://")
			if _, err := os.Stat(socketPath); os.IsNotExist(err) {
				continue
			}

			cli, err := client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
			if err != nil { continue }
			
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			_, err = cli.Ping(ctx)
			cancel()
			
			if err == nil { return DockerReadyMsg{Client: cli, Path: host} }
			cli.Close()
		}
		return DockerErrorMsg(fmt.Errorf("no docker socket found"))
	}
}

func fetchContainers(cli *client.Client) tea.Cmd {
	return func() tea.Msg {
		if cli == nil { return nil }
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		list, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
		if err != nil { return DockerErrorMsg(err) }
		return DockerDataMsg(list)
	}
}

// ИСПРАВЛЕНО: Умное чтение логов (TTY vs Multiplexed)
func fetchLogs(cli *client.Client, id string) tea.Cmd {
	return func() tea.Msg {
		if cli == nil { return DockerLogMsg("No client") }
		
		// 1. Проверяем настройки контейнера (есть ли TTY?)
		ctxInspect, cancelInspect := context.WithTimeout(context.Background(), 1*time.Second)
		details, err := cli.ContainerInspect(ctxInspect, id)
		cancelInspect()
		if err != nil { return DockerLogMsg(fmt.Sprintf("Inspect Error: %v", err)) }

		// 2. Запрашиваем логи
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		reader, err := cli.ContainerLogs(ctx, id, types.ContainerLogsOptions{
			ShowStdout: true, 
			ShowStderr: true, 
			Tail:       "200", // Берем побольше
		})
		if err != nil { return DockerLogMsg(fmt.Sprintf("Log Error: %v", err)) }
		defer reader.Close()

		var out strings.Builder

		// 3. Читаем в зависимости от TTY
		if details.Config.Tty {
			// Если TTY включен, данные идут "сырым" потоком
			_, err = io.Copy(&out, reader)
		} else {
			// Если TTY выключен, данные мультиплексированы (header + payload)
			_, err = stdcopy.StdCopy(&out, &out, reader)
		}

		if err != nil && err != io.EOF {
			return DockerLogMsg(fmt.Sprintf("Stream Error: %v", err))
		}

		logs := out.String()
		if logs == "" {
			logs = "(No logs found or container is silent)"
		}
		return DockerLogMsg(logs)
	}
}

func execShell(id string) tea.Cmd {
	bin := "podman"
	if _, err := exec.LookPath("podman"); err != nil { bin = "docker" }
	
	c := exec.Command(bin, "exec", "-it", id, "sh", "-c", "if [ -x /bin/bash ]; then exec /bin/bash; else exec /bin/sh; fi")
	
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return execFinishedMsg{err}
	})
}

// --- TEA MODEL ---

func (m *InfraModel) Init() tea.Cmd {
	return connectDocker()
}

func (m *InfraModel) Refresh() tea.Cmd {
	if m.client != nil {
		m.statusMsg = "Refreshing..."
		return fetchContainers(m.client)
	}
	return connectDocker()
}

func (m *InfraModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case DockerReadyMsg:
		m.client = msg.Client
		m.statusMsg = fmt.Sprintf("Connected to %s", msg.Path)
		return m, fetchContainers(m.client)

	case DockerErrorMsg:
		m.err = msg
		m.statusMsg = "Connection Error"

	case DockerDataMsg:
		m.containers = msg
		m.statusMsg = fmt.Sprintf("Updated: %d containers", len(msg))
		m.updateTable()

	case DockerLogMsg:
		m.state = viewLogs
		m.logs.SetContent(string(msg))
		m.logs.GotoBottom() // Скролл вниз
		return m, nil

	case execFinishedMsg:
		return m, tea.Batch(tea.ClearScreen, m.Refresh())

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeComponents()

	case tea.KeyMsg:
		if m.state == viewLogs {
			if msg.String() == "esc" || msg.String() == "q" {
				m.state = viewDashboard
				return m, nil
			}
			m.logs, cmd = m.logs.Update(msg) // Прокрутка логов
			return m, cmd
		}

		switch msg.String() {
		case "esc": return m, nil 
		
		case "s", "x", "r", "l", "e", "enter":
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.containers) {
				id := m.containers[idx].ID
				m.selectedID = id
				
				switch msg.String() {
				case "s":
					go m.client.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
					return m, tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg { return "refresh" })
				case "x":
					go func() {
						t := 1
						m.client.ContainerStop(context.Background(), id, container.StopOptions{Timeout: &t})
					}()
					return m, tea.Tick(1*time.Second, func(_ time.Time) tea.Msg { return "refresh" })
				case "r":
					go func() {
						t := 1
						m.client.ContainerRestart(context.Background(), id, container.StopOptions{Timeout: &t})
					}()
					return m, tea.Tick(1*time.Second, func(_ time.Time) tea.Msg { return "refresh" })
				case "l":
					// Запрашиваем логи
					m.statusMsg = "Fetching logs..."
					return m, fetchLogs(m.client, id)
				case "e", "enter":
					if m.containers[idx].State == "running" {
						return m, execShell(id)
					}
				}
			}
		}
	
	case string:
		if msg == "refresh" { return m, fetchContainers(m.client) }
	}

	if m.state == viewDashboard {
		m.table, cmd = m.table.Update(msg)
		// Обновление деталей при смене курсора
		if m.table.Cursor() >= 0 && m.table.Cursor() < len(m.containers) {
			m.selectedID = m.containers[m.table.Cursor()].ID
			m.updateDetails()
		}
	}

	return m, cmd
}

func (m *InfraModel) updateTable() {
	var rows []table.Row
	
	sort.Slice(m.containers, func(i, j int) bool {
		return m.containers[i].State == "running" && m.containers[j].State != "running"
	})

	for _, c := range m.containers {
		name := ""
		if len(c.Names) > 0 { name = strings.TrimPrefix(c.Names[0], "/") }
		
		status := "🧊"
		if c.State == "running" { status = "🟢" }
		if c.State == "exited" || c.State == "dead" { status = "💀" }
		
		img := c.Image
		if len(img) > 25 { img = img[:22] + "..." }

		rows = append(rows, table.Row{
			status,
			name,
			img,
		})
	}
	m.table.SetRows(rows)
	m.updateDetails()
}

func (m *InfraModel) updateDetails() {
	if m.selectedID == "" && len(m.containers) > 0 {
		m.selectedID = m.containers[0].ID
	}
	
	var container types.Container
	found := false
	for _, c := range m.containers {
		if c.ID == m.selectedID { container = c; found = true; break }
	}
	
	if !found {
		m.details.SetContent("Select a container...")
		return
	}

	var b strings.Builder
	title := styleTitle.Render(fmt.Sprintf(" %s ", strings.TrimPrefix(container.Names[0], "/")))
	b.WriteString(fmt.Sprintf("%s\n\n", title))
	
	kv := func(k, v string) string {
		return fmt.Sprintf("%s %s\n", lipgloss.NewStyle().Bold(true).Foreground(colorBorder).Render(k+":"), v)
	}

	b.WriteString(kv("ID", container.ID[:12]))
	b.WriteString(kv("Image", container.Image))
	b.WriteString(kv("State", container.State))
	b.WriteString(kv("Status", container.Status))
	b.WriteString(kv("Command", container.Command))
	b.WriteString(kv("Created", time.Unix(container.Created, 0).Format(time.RFC822)))
	
	if len(container.Ports) > 0 {
		b.WriteString("\n" + lipgloss.NewStyle().Underline(true).Render("Ports:") + "\n")
		for _, p := range container.Ports {
			b.WriteString(fmt.Sprintf("  • %d/%s -> %d\n", p.PrivatePort, p.Type, p.PublicPort))
		}
	}

	if len(container.Mounts) > 0 {
		b.WriteString("\n" + lipgloss.NewStyle().Underline(true).Render("Mounts:") + "\n")
		for _, m := range container.Mounts {
			b.WriteString(fmt.Sprintf("  • %s \n    ➜ %s\n", m.Source, m.Destination))
		}
	}

	m.details.SetContent(b.String())
}

func (m *InfraModel) resizeComponents() {
	if m.width == 0 || m.height == 0 { return }

	listWidth := int(float64(m.width) * 0.35)
	if listWidth < 40 { listWidth = 40 }
	
	detailsWidth := m.width - listWidth - 6
	if detailsWidth < 10 { detailsWidth = 10 }
	
	columns := m.table.Columns()
	if len(columns) >= 3 {
		columns[1].Width = listWidth - 30 
		if columns[1].Width < 10 { columns[1].Width = 10 }
	}
	m.table.SetColumns(columns)
	m.table.SetWidth(listWidth)
	m.table.SetHeight(m.height - 5)

	m.details.Width = detailsWidth
	m.details.Height = m.height - 5
	
	m.logs.Width = m.width - 4
	m.logs.Height = m.height - 4
}

func (m *InfraModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Docker Error: %v\n\nEnsure 'systemctl --user start podman.socket' is run.", m.err)
	}
	if m.state == viewLogs {
		return styleLogBox.Width(m.width-4).Height(m.height-2).Render(
			lipgloss.JoinVertical(lipgloss.Left, 
				styleTitle.Render(" CONTAINER LOGS (Esc to close) "),
				m.logs.View(),
			),
		)
	}

	listView := baseStyle.
		Width(m.table.Width()).
		Height(m.height - 2).
		Render(m.table.View())

	detailsView := baseStyle.
		Width(m.width - m.table.Width() - 6).
		Height(m.height - 2).
		Render(m.details.View())

	content := lipgloss.JoinHorizontal(lipgloss.Top, listView, detailsView)
	
	footer := lipgloss.NewStyle().Foreground(colorGray).Render(
		fmt.Sprintf(" %s | [s]tart [x]stop [r]estart [l]ogs [e]xec | [esc] back", m.statusMsg),
	)

	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}