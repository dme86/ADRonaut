package app

import tea "github.com/charmbracelet/bubbletea"

func Run() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

