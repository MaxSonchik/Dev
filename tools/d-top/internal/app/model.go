package app

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devos-os/d-top/internal/views"
)

// --- Стили ---
var (
	docStyle      = lipgloss.NewStyle().Margin(1, 2)
	focusedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	noStyle       = lipgloss.NewStyle()
	
	titleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF7DB")).Background(lipgloss.Color("#888B7E")).Padding(0, 1)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
)

type toolFinishedMsg struct{ err error }

type sessionState int

const (
	stateMenu sessionState = iota
	stateConnectForm
	stateInfra // Состояние для Docker Dashboard
)

type item struct {
	title, desc, id string
}
func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type Model struct {
	state        sessionState
	list         list.Model
	inputs       []textinput.Model 
	focusIndex   int
	
	infraModel   views.InfraModel // Встроенная модель
	
	choice       string
	quitting     bool
	err          error
}

func NewModel() Model {
	// Меню
	items := []list.Item{
		item{title: "Local Monitor", desc: "Run btop locally", id: "local"},
		item{title: "Remote Monitor", desc: "Connect via SSH", id: "remote"},
		item{title: "Infrastructure", desc: "Docker & K8s Dashboard", id: "infra"},
		item{title: "Red Team Ops", desc: "Stealth kill & injections", id: "red"},
	}
	
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "d-top: Control Center"

	// Форма подключения
	inputs := make([]textinput.Model, 4)
	inputs[0] = textinput.New(); inputs[0].Placeholder = "127.0.0.1"; inputs[0].Prompt = "Host: "
	inputs[1] = textinput.New(); inputs[1].Placeholder = "root"; inputs[1].Prompt = "User: "
	inputs[2] = textinput.New(); inputs[2].Placeholder = "22"; inputs[2].Prompt = "Port: "
	inputs[3] = textinput.New(); inputs[3].SetValue("Standard"); inputs[3].Prompt = "Mode: "

	return Model{
		state:      stateMenu,
		list:       l,
		inputs:     inputs,
		infraModel: views.NewInfraModel(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.infraModel.Init() // Запускаем таймер обновления Docker
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case toolFinishedMsg:
		m.choice = ""
		if msg.err != nil { m.err = msg.err }
		return m, tea.Batch(tea.ClearScreen)

	case tea.KeyMsg:
		// Глобальный выход
		if m.state == stateMenu && (msg.String() == "q" || msg.String() == "ctrl+c") {
			m.quitting = true
			return m, tea.Quit
		}
		// Возврат в меню из Infra
		if m.state == stateInfra && msg.String() == "esc" {
			m.state = stateMenu
			return m, nil
		}
	}

	// Делегирование
	switch m.state {
	case stateMenu:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "enter" {
				i, ok := m.list.SelectedItem().(item)
				if ok {
					return m.handleMenuSelect(i.id)
				}
			}
		case tea.WindowSizeMsg:
			m.list.SetWidth(msg.Width)
			m.list.SetHeight(msg.Height)
		}
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

	case stateConnectForm:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "esc" {
				m.state = stateMenu
				return m, nil
			}
			// Переключение Mode
			if m.focusIndex == 3 && msg.String() == " " {
				if m.inputs[3].Value() == "Standard" {
					m.inputs[3].SetValue("Stealth (Red Team)")
					m.inputs[3].TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4757"))
				} else {
					m.inputs[3].SetValue("Standard")
					m.inputs[3].TextStyle = focusedStyle
				}
				return m, nil
			}

			// Навигация
			if msg.String() == "tab" || msg.String() == "shift+tab" || msg.String() == "enter" || msg.String() == "up" || msg.String() == "down" {
				s := msg.String()
				if s == "enter" && m.focusIndex == len(m.inputs) {
					return m, m.connectSSH()
				}
				if s == "up" || s == "shift+tab" { m.focusIndex-- } else { m.focusIndex++ }
				if m.focusIndex > len(m.inputs) { m.focusIndex = 0 }
				if m.focusIndex < 0 { m.focusIndex = len(m.inputs) }

				for i := 0; i <= len(m.inputs)-1; i++ {
					if i == m.focusIndex {
						m.inputs[i].Focus()
						m.inputs[i].PromptStyle = focusedStyle
						if i != 3 { m.inputs[i].TextStyle = focusedStyle }
					} else {
						m.inputs[i].Blur()
						m.inputs[i].PromptStyle = noStyle
						if i != 3 { m.inputs[i].TextStyle = noStyle }
					}
				}
				return m, nil
			}
		}
		for i := range m.inputs {
			if i == 3 { continue }
			m.inputs[i], cmd = m.inputs[i].Update(msg)
			cmds = append(cmds, cmd)
		}

	case stateInfra:
		// Делегируем события в InfraModel
		var infraModel tea.Model
		infraModel, cmd = m.infraModel.Update(msg)
		m.infraModel = infraModel.(views.InfraModel)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleMenuSelect(id string) (tea.Model, tea.Cmd) {
	switch id {
	case "local":
		m.choice = "local"
		c := exec.Command("btop")
		return *m, tea.ExecProcess(c, func(err error) tea.Msg { return toolFinishedMsg{err} })
	case "remote":
		m.state = stateConnectForm
		m.focusIndex = 0
		m.inputs[0].Focus()
		return *m, nil
	case "infra":
		m.state = stateInfra
		m.infraModel.Refresh() // Обновляем список контейнеров при входе
		return *m, nil
	case "red":
		return *m, nil
	}
	return *m, nil
}

func (m *Model) connectSSH() tea.Cmd {
	host := m.inputs[0].Value()
	user := m.inputs[1].Value()
	port := m.inputs[2].Value()
	mode := m.inputs[3].Value()

	if host == "" { return nil }
	if user == "" { user = "root" }
	if port == "" { port = "22" }

	args := []string{"-p", port, "-t"}
	if strings.Contains(mode, "Stealth") {
		args = append(args, "-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no", "-o", "LogLevel=QUIET")
	}
	args = append(args, fmt.Sprintf("%s@%s", user, host), "btop")

	c := exec.Command("ssh", args...)
	m.state = stateMenu
	m.choice = "remote"
	return tea.ExecProcess(c, func(err error) tea.Msg { return toolFinishedMsg{err} })
}

func (m Model) View() string {
	if m.choice != "" && m.state == stateMenu { return "" }
	
	if m.state == stateMenu {
		if m.err != nil { return fmt.Sprintf("Error: %v\nPress any key...", m.err) }
		return docStyle.Render(m.list.View())
	}

	if m.state == stateConnectForm {
		var b strings.Builder
		b.WriteString(titleStyle.Render(" Remote Connection ") + "\n\n")
		for i := range m.inputs { b.WriteString(m.inputs[i].View() + "\n") }
		
		b.WriteString(helpStyle.Render("\n(Space: Toggle Mode | Enter: Connect | Esc: Back)"))
		
		button := "[ CONNECT ]"
		if m.focusIndex == len(m.inputs) { button = focusedStyle.Render("[ CONNECT ]") }
		fmt.Fprintf(&b, "\n\n%s", button)
		
		return docStyle.Render(b.String())
	}

	if m.state == stateInfra {
		return m.infraModel.View()
	}

	return ""
}