package ui

import (
	"fmt"
	"sort"
	//"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devos-os/d-ci/internal/domain"
	"github.com/devos-os/d-ci/internal/ui/styles"
)

// UI State
type viewState int

const (
	viewDashboard viewState = iota
	viewLogs
)

type Model struct {
	// Data
	providers map[string]domain.Provider // MAP: RepoName -> Provider
	pipelines map[string]domain.Pipeline
	sub       <-chan domain.PipelineEvent

	// UI State
	state        viewState
	spinner      spinner.Model
	width, height int
	
	// Navigation
	sortedKeys   []string
	cursor       int      
	expanded     map[string]bool
	childCursor  int      
	isChildFocus bool     

	// Logs
	logViewport viewport.Model
	logContent  string
	loadingLogs bool
	
	// Messages
	statusMsg string
	err       error
}

// Изменена сигнатура: принимает map
func NewModel(provs map[string]domain.Provider, sub <-chan domain.PipelineEvent) Model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(styles.Yellow)

	vp := viewport.New(0, 0)

	return Model{
		providers:   provs, 
		pipelines:   make(map[string]domain.Pipeline),
		sub:         sub,
		spinner:     s,
		expanded:    make(map[string]bool),
		logViewport: vp,
	}
}

type waitForUpdateMsg domain.PipelineEvent
type logLoadedMsg string
type actionResultMsg string // Сообщение об успешном действии (Retry/Cancel)
type errorMsg error

func (m Model) listen() tea.Cmd {
	return func() tea.Msg {
		return waitForUpdateMsg(<-m.sub)
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.listen())
}

// --- UPDATE ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.logViewport.Width = msg.Width - 4
		m.logViewport.Height = msg.Height - 4

	case tea.KeyMsg:
		if m.state == viewLogs {
			if msg.String() == "q" || msg.String() == "esc" {
				m.state = viewDashboard
				return m, nil
			}
			m.logViewport, cmd = m.logViewport.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		
		case "up", "k":
			if m.isChildFocus {
				m.childCursor--
				if m.childCursor < 0 { m.childCursor = 0 }
			} else {
				m.cursor--
				if m.cursor < 0 { m.cursor = 0 }
			}

		case "down", "j":
			if m.isChildFocus {
				if len(m.sortedKeys) > 0 {
					pid := m.sortedKeys[m.cursor]
					p := m.pipelines[pid]
					totalJobs := countJobs(p)
					if totalJobs > 0 {
						m.childCursor++
						if m.childCursor >= totalJobs { m.childCursor = totalJobs - 1 }
					}
				}
			} else {
				m.cursor++
				if m.cursor >= len(m.pipelines) { m.cursor = len(m.pipelines) - 1 }
			}

		case "enter", "right":
			if !m.isChildFocus && len(m.sortedKeys) > 0 {
				pid := m.sortedKeys[m.cursor]
				m.expanded[pid] = !m.expanded[pid]
				if m.expanded[pid] {
					m.isChildFocus = true
					m.childCursor = 0
				}
			}

		case "esc", "left":
			if m.isChildFocus {
				m.isChildFocus = false 
			}

		// --- ACTIONS ---
		case "l": // Logs (Job)
			if m.isChildFocus {
				repoName, jobID := m.getSelectedJobInfo()
				if jobID != "" {
					m.state = viewLogs
					m.loadingLogs = true
					m.logContent = "Loading logs..."
					m.logViewport.SetContent(m.logContent)
					return m, m.fetchLogs(repoName, jobID)
				}
			}
		
		case "r": // Retry (Pipeline or Job)
			if len(m.sortedKeys) > 0 {
				pid := m.sortedKeys[m.cursor]
				p := m.pipelines[pid]
				if m.isChildFocus {
					// Retry Job
					_, jobID := m.getSelectedJobInfo()
					return m, m.triggerAction(p.Project, func(prov domain.Provider) error {
						return prov.RetryJob(jobID)
					})
				} else {
					// Retry Pipeline
					return m, m.triggerAction(p.Project, func(prov domain.Provider) error {
						return prov.RetryPipeline(p.ID)
					})
				}
			}

		case "c": // Cancel (Pipeline)
			if len(m.sortedKeys) > 0 {
				pid := m.sortedKeys[m.cursor]
				p := m.pipelines[pid]
				return m, m.triggerAction(p.Project, func(prov domain.Provider) error {
					return prov.CancelPipeline(p.ID)
				})
			}
			
		case "o": // Open Web
			// TODO: Open `p.WebURL` in browser
		}

	case waitForUpdateMsg:
		event := domain.PipelineEvent(msg)
		if event.Type == "ERROR" {
			m.statusMsg = fmt.Sprintf("Error (%s): %v", event.RepoName, event.Error)
			return m, m.listen()
		}

		key := fmt.Sprintf("%s#%s", event.RepoName, event.Pipeline.ID)
		if event.Type == "UPDATE" || event.Type == "NEW" {
			m.pipelines[key] = event.Pipeline
			m.rebuildSort()
		}
		return m, m.listen()

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
		
	case logLoadedMsg:
		m.loadingLogs = false
		m.logContent = string(msg)
		m.logViewport.SetContent(m.logContent)
		
	case actionResultMsg:
		m.statusMsg = string(msg)

	case errorMsg:
		m.statusMsg = fmt.Sprintf("Error: %v", msg)
	}

	return m, tea.Batch(cmds...)
}

// --- VIEW ---

func (m Model) View() string {
	if m.state == viewLogs {
		return fmt.Sprintf("%s\n%s\n%s", 
			styles.ProjectStyle.Render("View Logs (ESC to close)"),
			m.logViewport.View(),
			styles.FooterStyle.Render("Use Arrows/PgUp/PgDn to scroll"),
		)
	}

	s := "\n DevOS CI Control Center\n\n"

	if len(m.pipelines) == 0 {
		s += " Waiting for events..." + m.spinner.View() + "\n"
		return s
	}

	for i, key := range m.sortedKeys {
		p := m.pipelines[key]
		isSelected := (i == m.cursor)
		
		statusIcon := getStatusIcon(p.Status)
		
		cursorStr := "  "
		if isSelected && !m.isChildFocus {
			cursorStr = "> "
		}

		line := fmt.Sprintf("%s%s %s [%s] %s (%s)", 
			cursorStr,
			statusIcon,
			styles.ProjectStyle.Render(p.Project),
			styles.BranchStyle.Render(p.Ref),
			p.CommitMsg,
			p.Author,
		)
		
		if isSelected && !m.isChildFocus {
			line = styles.SelectedStyle.Render(line)
		}
		s += line + "\n"

		if m.expanded[key] {
			s += m.renderJobs(p, isSelected && m.isChildFocus)
		}
	}

	s += "\n " + styles.FooterStyle.Render("Keys: [Enter] Focus | [Esc] Back | [r] Retry | [c] Cancel | [l] Logs")
	if m.statusMsg != "" {
		s += "\n " + styles.ErrorStyle.Render(m.statusMsg)
	}
	
	return s
}

func (m Model) renderJobs(p domain.Pipeline, parentFocused bool) string {
	s := ""
	globalJobIndex := 0
	
	for _, stage := range p.Stages {
		for _, job := range stage.Jobs {
			isJobSelected := parentFocused && (globalJobIndex == m.childCursor)
			cursor := "    "
			if isJobSelected {
				cursor = "  > "
			}
			icon := getStatusIcon(job.Status)
			jobLine := fmt.Sprintf("%s └─ %s %s (%s)", cursor, icon, job.Name, stage.Name)
			
			if isJobSelected {
				jobLine = styles.SelectedStyle.Render(jobLine)
			}
			s += jobLine + "\n"
			globalJobIndex++
		}
	}
	return s
}

// --- Logic Implementation ---

func (m *Model) rebuildSort() {
	keys := make([]string, 0, len(m.pipelines))
	for k := range m.pipelines { keys = append(keys, k) }
	sort.Slice(keys, func(i, j int) bool {
		p1 := m.pipelines[keys[i]]
		p2 := m.pipelines[keys[j]]
		return p1.CreatedAt.After(p2.CreatedAt)
	})
	m.sortedKeys = keys
}

// Возвращает (RepoName, JobID)
func (m Model) getSelectedJobInfo() (string, string) {
	if !m.isChildFocus || len(m.sortedKeys) == 0 { return "", "" }
	pid := m.sortedKeys[m.cursor]
	p := m.pipelines[pid]
	
	idx := 0
	for _, stage := range p.Stages {
		for _, job := range stage.Jobs {
			if idx == m.childCursor {
				return p.Project, job.ID
			}
			idx++
		}
	}
	return "", ""
}

// Выполнение действия (Retry/Cancel) в отдельной горутине
func (m Model) triggerAction(repoName string, action func(domain.Provider) error) tea.Cmd {
	return func() tea.Msg {
		if prov, ok := m.providers[repoName]; ok {
			if err := action(prov); err != nil {
				return errorMsg(err)
			}
			return actionResultMsg("Action executed successfully")
		}
		return errorMsg(fmt.Errorf("provider not found for %s", repoName))
	}
}

// Загрузка логов
func (m Model) fetchLogs(repoName, jobId string) tea.Cmd {
	return func() tea.Msg {
		// Ищем нужного провайдера по имени репозитория
		if prov, ok := m.providers[repoName]; ok {
			logs, err := prov.GetJobLog(jobId)
			if err != nil {
				return errorMsg(fmt.Errorf("fetch logs error: %v", err))
			}
			return logLoadedMsg(logs)
		}
		return errorMsg(fmt.Errorf("provider not found for %s", repoName))
	}
}

func countJobs(p domain.Pipeline) int {
	c := 0
	for _, s := range p.Stages { c += len(s.Jobs) }
	return c
}

func getStatusIcon(s domain.Status) string {
	switch s {
	case domain.StatusSuccess: return lipgloss.NewStyle().Foreground(styles.Green).Render("✔")
	case domain.StatusFailed: return lipgloss.NewStyle().Foreground(styles.Red).Render("✖")
	case domain.StatusRunning: return lipgloss.NewStyle().Foreground(styles.Yellow).Render("↻")
	case domain.StatusPending: return lipgloss.NewStyle().Foreground(styles.Gray).Render("•")
	case domain.StatusSkipped: return lipgloss.NewStyle().Foreground(styles.Gray).Render("-")
	case domain.StatusCanceled: return lipgloss.NewStyle().Foreground(styles.Red).Render("⊘")
	default: return "?"
	}
}