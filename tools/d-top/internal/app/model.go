package app

import (
	"fmt"
	"os/exec"
	"strings"
	"os"
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

// Состояния
const (
	stateMenu sessionState = iota
	stateConnectForm
	stateInfra
	stateRedTeam
)
type sessionState int

type Model struct {
	state        sessionState
	list         list.Model
	inputs       []textinput.Model 
	focusIndex   int
	
	infraModel   *views.InfraModel
	redModel     *views.RedTeamModel
	
	connected    bool
	sshConfig    views.SSHConfig
	socketPath   string
	
	choice       string
	quitting     bool
	err          error
}

type item struct { title, desc, id string; locked bool }
func (i item) Title() string { 
	if i.locked { return "🔒 " + i.title }
	return i.title 
}
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

func NewModel() Model {
	items := []list.Item{
		item{title: "Connect", desc: "Establish Master Session", id: "connect"},
		item{title: "Local Monitor", desc: "Run btop locally", id: "local"},
		item{title: "Infrastructure", desc: "Docker Dashboard", id: "infra"},
		item{title: "Remote Monitor", desc: "View remote stats", id: "remote", locked: true},
		item{title: "Red Team Ops", desc: "Process manipulation", id: "red", locked: true},
	}
	
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "d-top: Control Center"

	inputs := make([]textinput.Model, 5)
	inputs[0] = textinput.New(); inputs[0].Placeholder = "127.0.0.1"; inputs[0].Prompt = "Host:     "
	inputs[1] = textinput.New(); inputs[1].Placeholder = "root";      inputs[1].Prompt = "User:     "
	inputs[2] = textinput.New(); inputs[2].Placeholder = "2222";      inputs[2].Prompt = "Port:     "
	inputs[3] = textinput.New(); inputs[3].Placeholder = "Password";  inputs[3].Prompt = "Password: "; inputs[3].EchoMode = textinput.EchoPassword; inputs[3].EchoCharacter = '•'
	inputs[4] = textinput.New(); inputs[4].SetValue("Standard");      inputs[4].Prompt = "Mode:     "

	m := Model{
		state:      stateMenu,
		list:       l,
		inputs:     inputs,
		infraModel: views.NewInfraModel(),
		redModel:   views.NewRedTeamModel(),
		socketPath: fmt.Sprintf("/tmp/d-top-ssh-%d.sock", os.Getpid()),
	}
	m.updateMenuItems()
	return m
}

func (m *Model) updateMenuItems() {
	items := []list.Item{
		item{title: "Connect", desc: "Establish Master Session", id: "connect"},
		item{title: "Local Monitor", desc: "Run btop locally", id: "local"},
		item{title: "Infrastructure", desc: "Docker Dashboard", id: "infra"},
		item{title: "Remote Monitor", desc: "View remote stats", id: "remote", locked: !m.connected},
		item{title: "Red Team Ops", desc: "Process manipulation", id: "red", locked: !m.connected},
	}
	m.list.SetItems(items)
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.infraModel.Init(),
		m.redModel.Init(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	
	case toolFinishedMsg:
		m.choice = ""
		if m.state == stateConnectForm && msg.err == nil {
			m.connected = true
			m.updateMenuItems()
			m.state = stateMenu
		}
		if msg.err != nil { m.err = msg.err }
		return m, tea.Batch(tea.ClearScreen)

	case tea.KeyMsg:
		if m.state == stateMenu && (msg.String() == "q" || msg.String() == "ctrl+c") {
			_ = exec.Command("ssh", "-S", m.socketPath, "-O", "exit", "dummy").Run()
			return m, tea.Quit
		}
		if (m.state == stateInfra || m.state == stateRedTeam) && msg.String() == "esc" {
			m.state = stateMenu
			return m, nil
		}

	// ИСПРАВЛЕНИЕ: Передаем размеры окна во все под-модели!
	case tea.WindowSizeMsg:
		// 1. Обновляем меню
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height)
		
		// 2. Обновляем Infra Model (это починит отображение таблицы)
		var iModel tea.Model
		iModel, cmd = m.infraModel.Update(msg)
		m.infraModel = iModel.(*views.InfraModel)
		cmds = append(cmds, cmd)

		// 3. Обновляем Red Team Model
		var rModel tea.Model
		rModel, cmd = m.redModel.Update(msg)
		m.redModel = rModel.(*views.RedTeamModel)
		cmds = append(cmds, cmd)
		
		return m, tea.Batch(cmds...)
	}

	switch m.state {
	case stateMenu:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "enter" {
				i, ok := m.list.SelectedItem().(item)
				if ok {
					if i.locked { return m, nil }
					switch i.id {
					case "connect":
						m.state = stateConnectForm
						m.inputs[0].Focus()
						return m, nil
					case "remote":
						return m.launchRemoteBtop()
					case "local":
						return m.runLocalBtop()
					case "infra":
						m.state = stateInfra
						// Force resize on enter just in case
						// m.infraModel.Update(tea.WindowSizeMsg{Width: m.list.Width(), Height: m.list.Height()})
						return m, m.infraModel.Refresh()
					case "red":
						m.state = stateRedTeam
						m.redModel.SetConnection(&m.sshConfig)
						m.sshConfig.SocketPath = m.socketPath
						return m, m.redModel.Refresh()
					}
				}
			}
		}
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

	case stateConnectForm:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "esc" { m.state = stateMenu; return m, nil }
			if m.focusIndex == 4 && msg.String() == " " {
				if m.inputs[4].Value() == "Standard" { m.inputs[4].SetValue("Stealth (Red Team)") } else { m.inputs[4].SetValue("Standard") }
				return m, nil
			}
			if msg.String() == "enter" && m.focusIndex == len(m.inputs) {
				return m.startMasterSession()
			}
			if msg.String() == "tab" || msg.String() == "down" { m.focusIndex = (m.focusIndex + 1) % (len(m.inputs) + 1); return m, m.updateInputsFocus() }
			if msg.String() == "shift+tab" || msg.String() == "up" { m.focusIndex--; if m.focusIndex < 0 { m.focusIndex = len(m.inputs) }; return m, m.updateInputsFocus() }
		}
		for i := range m.inputs { if i != 4 { m.inputs[i], cmd = m.inputs[i].Update(msg) } }
		return m, cmd

	case stateInfra:
		var model tea.Model
		model, cmd = m.infraModel.Update(msg)
		m.infraModel = model.(*views.InfraModel)
		cmds = append(cmds, cmd)
	
	case stateRedTeam:
		var model tea.Model
		model, cmd = m.redModel.Update(msg)
		m.redModel = model.(*views.RedTeamModel)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) startMasterSession() (tea.Model, tea.Cmd) {
	m.sshConfig = views.SSHConfig{
		Host: m.inputs[0].Value(),
		User: m.inputs[1].Value(),
		Port: m.inputs[2].Value(),
		Stealth: strings.Contains(m.inputs[4].Value(), "Stealth"),
		SocketPath: m.socketPath,
	}
	password := m.inputs[3].Value()

	os.Remove(m.socketPath)

	sshArgs := []string{
		"-M", "-S", m.socketPath, 
		"-p", m.sshConfig.Port,
		"-o", "ControlPersist=yes",
		"-N",
	}
	
	if m.sshConfig.Stealth {
		sshArgs = append(sshArgs, "-o", "UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no")
	}
	
	target := fmt.Sprintf("%s@%s", m.sshConfig.User, m.sshConfig.Host)
	sshArgs = append(sshArgs, target)

	var c *exec.Cmd
	if password != "" {
		finalArgs := append([]string{"-p", password, "ssh"}, sshArgs...)
		c = exec.Command("sshpass", finalArgs...)
	} else {
		c = exec.Command("ssh", sshArgs...)
	}
	
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		if _, statErr := os.Stat(m.socketPath); statErr == nil {
			return toolFinishedMsg{nil}
		}
		return toolFinishedMsg{fmt.Errorf("connection failed")}
	})
}

func (m *Model) runLocalBtop() (tea.Model, tea.Cmd) {
	m.choice = "local"
	c := exec.Command("btop")
	return m, tea.ExecProcess(c, func(err error) tea.Msg { return toolFinishedMsg{err} })
}

func (m *Model) launchRemoteBtop() (tea.Model, tea.Cmd) {
	args := []string{"-S", m.socketPath, "-t", fmt.Sprintf("%s@%s", m.sshConfig.User, m.sshConfig.Host), "btop"}
	c := exec.Command("ssh", args...)
	m.choice = "remote"
	return m, tea.ExecProcess(c, func(err error) tea.Msg { return toolFinishedMsg{err} })
}

func (m *Model) updateInputsFocus() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := 0; i < len(m.inputs); i++ {
		if i == m.focusIndex { cmds[i] = m.inputs[i].Focus(); m.inputs[i].PromptStyle = focusedStyle } else { m.inputs[i].Blur(); m.inputs[i].PromptStyle = noStyle }
	}
	return tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.choice != "" { return "" }
	if m.state == stateMenu { return docStyle.Render(m.list.View()) }
	if m.state == stateConnectForm {
		var b strings.Builder
		b.WriteString(titleStyle.Render(" SSH Master Connection ") + "\n\n")
		for i := range m.inputs { b.WriteString(m.inputs[i].View() + "\n") }
		b.WriteString(helpStyle.Render("\n(Space: Mode | Enter: Connect | Esc: Back)"))
		button := "[ CONNECT ]"
		if m.focusIndex == len(m.inputs) { button = focusedStyle.Render(button) }
		fmt.Fprintf(&b, "\n%s", button)
		return docStyle.Render(b.String())
	}
	if m.state == stateInfra { return m.infraModel.View() }
	if m.state == stateRedTeam { return m.redModel.View() }
	return ""
}