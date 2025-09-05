package app

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

/* ------------------------------- Styles ---------------------------------- */

// Gruvbox xterm-256 approximations
const (
	gbRed    = "203" // #fb4934
	gbGreen  = "142" // #b8bb26
	gbYellow = "214" // #fabd2f
	gbBlue   = "109" // #83a598
	gbPurple = "175" // #d3869b
	gbAqua   = "108" // #8ec07c
	gbOrange = "208" // #fe8019
	gbGray   = "245" // #928374
	gbBg0    = "235" // #282828
)

var (
	statuses = []string{"Vorgeschlagen", "Angenommen", "Abgelehnt", "Veraltet"}

	titleStyle    = lipgloss.NewStyle().Bold(true)
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Bold(true)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	optionStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
	selectedStyle = optionStyle.Copy().Foreground(lipgloss.Color("205")).Underline(true)

	activeStyle  = lipgloss.NewStyle().Bold(true).Underline(true)
	framePadding = 2

	chipBase = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true).
			MarginLeft(1).
			Foreground(lipgloss.Color("0"))

	chipColors = map[string]string{
		"Titel":        gbYellow,
		"Tags":         gbPurple,
		"Beteiligte":   gbBlue,
		"Kontext":      gbGreen,
		"Entscheidung": gbOrange,
		"Alternativen": gbAqua,
		"Konsequenzen": gbRed,
		"Dateiname":    gbGray,
	}

	snippetStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	highlightStyle = lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("205")).Bold(true)
)

func chipWithCount(b badge) string {
	bg := chipColors[b.Label]
	st := chipBase
	if bg != "" {
		st = st.Background(lipgloss.Color(bg))
	}
	// {Label} [Count]
	return st.Render(fmt.Sprintf("{%s} [%d]", b.Label, b.Count))
}
