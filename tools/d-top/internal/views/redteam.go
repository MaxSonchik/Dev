package views

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)
var redBaseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))
// Конфигурация подключения
type SSHConfig struct {
	Host, User, Port string
	Stealth          bool
	SocketPath       string // <--- Добавлено поле
}

type RedTeamModel struct {
	table     table.Model
	processes []RemoteProcess
	sshCfg    *SSHConfig 
	
	filterInput textinput.Model
	filtering   bool
	loading     bool
	err         error
	
	selectedPID string 
}

type RemoteProcess struct {
	PID, User, Name, CPU, Mem, State string
}

type ProcessMsg []RemoteProcess
type ProcessErrorMsg error

func NewRedTeamModel() *RedTeamModel {
	columns := []table.Column{
		{Title: "PID", Width: 8},
		{Title: "User", Width: 10},
		{Title: "Command", Width: 25},
		{Title: "CPU", Width: 6},
		{Title: "Mem", Width: 6},
		{Title: "Stat", Width: 6},
	}

	t := table.New(table.WithColumns(columns), table.WithFocused(true), table.WithHeight(12))
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("196")).Bold(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("124")).Bold(true)
	t.SetStyles(s)

	ti := textinput.New()
	ti.Placeholder = "Filter..."
	ti.CharLimit = 20
	ti.Width = 30

	return &RedTeamModel{
		table:       t,
		filterInput: ti,
	}
}

func (m *RedTeamModel) SetConnection(cfg *SSHConfig) {
	m.sshCfg = cfg
	m.processes = []RemoteProcess{}
}

func fetchRemoteProcesses(cfg *SSHConfig) tea.Cmd {
	return func() tea.Msg {
		if cfg == nil {
			return ProcessErrorMsg(fmt.Errorf("not connected"))
		}
		// Используем ps для получения списка
		cmdStr := "ps -Ao pid,user,comm,pcpu,pmem,stat --no-headers --sort=-pcpu | head -n 50"
		
		// Используем Master Socket (-S) для подключения без пароля
		sshArgs := []string{"-S", cfg.SocketPath, fmt.Sprintf("%s@%s", cfg.User, cfg.Host), cmdStr}

		// ИСПРАВЛЕНИЕ: Выполняем команду с аргументами
		out, err := exec.Command("ssh", sshArgs...).Output()
		if err != nil {
			return ProcessErrorMsg(fmt.Errorf("ssh failed: %v", err))
		}

		var procs []RemoteProcess
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 6 {
				p := RemoteProcess{
					PID: fields[0], User: fields[1], Name: fields[2],
					CPU: fields[3], Mem: fields[4], State: fields[5],
				}
				procs = append(procs, p)
			}
		}
		return ProcessMsg(procs)
	}
}

func sendRemoteSignal(cfg *SSHConfig, pid string, signal string) tea.Cmd {
	return func() tea.Msg {
		if cfg == nil { return nil }
		cmdStr := fmt.Sprintf("kill -%s %s", signal, pid)
		
		// Используем Master Socket
		sshArgs := []string{"-S", cfg.SocketPath, fmt.Sprintf("%s@%s", cfg.User, cfg.Host), cmdStr}
		
		// ИСПРАВЛЕНИЕ: Выполняем команду с аргументами
		exec.Command("ssh", sshArgs...).Run()
		return "signal_sent"
	}
}

func (m *RedTeamModel) Refresh() tea.Cmd {
	m.loading = true
	return fetchRemoteProcesses(m.sshCfg)
}

func (m *RedTeamModel) Init() tea.Cmd { return nil }

func (m *RedTeamModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	
	case ProcessMsg:
		m.loading = false
		m.err = nil
		m.processes = msg
		m.updateTable()
		return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return "refresh" })

	case ProcessErrorMsg:
		m.loading = false
		m.err = msg
		return m, nil

	case string:
		if msg == "refresh" || msg == "signal_sent" {
			return m, fetchRemoteProcesses(m.sshCfg)
		}

	case tea.KeyMsg:
		if m.filtering {
			if msg.String() == "enter" || msg.String() == "esc" {
				m.filtering = false
				m.filterInput.Blur()
				return m, nil
			}
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.updateTable()
			return m, cmd
		}

		switch msg.String() {
		case "esc": return m, nil
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, nil
		
		case "k": // KILL
			if pid := m.getSelectedPID(); pid != "" {
				return m, sendRemoteSignal(m.sshCfg, pid, "SEGV")
			}
		case "f": // FREEZE
			if pid := m.getSelectedPID(); pid != "" {
				return m, sendRemoteSignal(m.sshCfg, pid, "STOP")
			}
		case "r": // RESUME
			if pid := m.getSelectedPID(); pid != "" {
				return m, sendRemoteSignal(m.sshCfg, pid, "CONT")
			}
		case "up", "down":
			m.table, cmd = m.table.Update(msg)
			m.selectedPID = m.getSelectedPID()
			return m, cmd
		}
	}

	m.table, cmd = m.table.Update(msg)
	m.selectedPID = m.getSelectedPID()
	return m, cmd
}

func (m *RedTeamModel) updateTable() {
	var rows []table.Row
	filter := strings.ToLower(m.filterInput.Value())
	
	sort.Slice(m.processes, func(i, j int) bool {
		c1, _ := strconv.ParseFloat(m.processes[i].CPU, 64)
		c2, _ := strconv.ParseFloat(m.processes[j].CPU, 64)
		return c1 > c2
	})

	for _, p := range m.processes {
		if filter != "" && !strings.Contains(strings.ToLower(p.Name), filter) {
			continue
		}
		rows = append(rows, table.Row{p.PID, p.User, p.Name, p.CPU, p.Mem, p.State})
	}
	m.table.SetRows(rows)

	if m.selectedPID != "" {
		for i, r := range rows {
			if r[0] == m.selectedPID {
				m.table.SetCursor(i)
				break
			}
		}
	}
}

func (m *RedTeamModel) getSelectedPID() string {
	if row := m.table.SelectedRow(); row != nil {
		return row[0]
	}
	return ""
}

func sendSignal(pid int, sig syscall.Signal) {
	if p, err := os.FindProcess(pid); err == nil {
		p.Signal(sig)
	}
}

func (m *RedTeamModel) View() string {
	if m.err != nil { return fmt.Sprintf("Remote Error: %v", m.err) }
	if m.sshCfg == nil { return "Please connect via 'Connect' tab first." }
	
	header := ""
	if m.filtering { header = fmt.Sprintf("Filter: %s", m.filterInput.View()) }
	
	help := "\n [k] Crash (SEGV) | [f] Freeze | [r] Resume | [/] Filter | [esc] Back"
	return header + baseStyle.Render(m.table.View()) + lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(help)
}