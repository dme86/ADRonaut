package app

import (
	"encoding/json"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

const (
	autosaveDir      = ".adronaut"
	autosaveInterval = 3 * time.Second
)

func scheduleAutosave() tea.Cmd {
	return tea.Tick(autosaveInterval, func(time.Time) tea.Msg { return autosaveTickMsg{} })
}

func autosaveCmd(m model) tea.Cmd {
	df := m.toDraft()
	path := m.draftPath()
	return func() tea.Msg {
		b, err := json.MarshalIndent(df, "", "  ")
		if err == nil {
			err = atomicWrite(path, b, 0o644)
		}
		return autosaveDoneMsg{path: path, err: err}
	}
}
