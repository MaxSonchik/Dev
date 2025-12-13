package app

import (
	"os"
	"os/exec"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	title, desc string
	id          string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type Model struct {
	list     list.Model
	choice   string
	quitting bool
}

func NewModel() Model {
	items := []list.Item{
		item{title: "Local Monitor", desc: "Run btop on this machine", id: "local"},
		item{title: "Remote Monitor", desc: "Connect via SSH (Agentless/Stealth)", id: "remote"},
		item{title: "Infrastructure", desc: "Docker & Kubernetes Dashboard", id: "infra"},
		item{title: "Red Team Ops", desc: "Process injection & Stealth kill", id: "red"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "d-top: Infrastructure Control"

	return Model{list: l}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		if msg.String() == "enter" {
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = i.id
				return m, runTool(i.id)
			}
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.choice != "" {
		return "" // UI очищается перед запуском тулзы
	}
	if m.quitting {
		return "Bye!"
	}
	return docStyle.Render(m.list.View())
}

// runTool запускает внешние утилиты или переключает вид
func runTool(id string) tea.Cmd {
	return func() tea.Msg {
		if id == "local" {
			// Трюк: мы временно выходим из BubbleTea, запускаем btop, потом возвращаемся
			cmd := exec.Command("btop")
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
			return nil // Возвращаемся в меню после выхода из btop
		}
		// Для остальных пока заглушка
		return nil
	}
}