package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	Gray   = lipgloss.Color("#555")
	Green  = lipgloss.Color("#2ecc71")
	Red    = lipgloss.Color("#e74c3c")
	Yellow = lipgloss.Color("#f1c40f")
	Purple = lipgloss.Color("#9b59b6")
	White  = lipgloss.Color("#ecf0f1")

	// General
	ProjectStyle = lipgloss.NewStyle().Foreground(White).Bold(true)
	BranchStyle  = lipgloss.NewStyle().Foreground(Purple)
	
	// Job Boxes
	JobBoxStyle = lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.RoundedBorder())
	
	// Elements
	ArrowStyle  = lipgloss.NewStyle().Foreground(Gray).Bold(true)
	FooterStyle = lipgloss.NewStyle().Foreground(Gray)
	ErrorStyle  = lipgloss.NewStyle().Foreground(Red)
)