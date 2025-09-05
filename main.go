package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
)

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println(errorStyle.Render("Fehler:"), err)
		os.Exit(1)
	}
}
