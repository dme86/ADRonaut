package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"os/exec"
	"strings"
)

func loadGitInfoCmd() tea.Cmd {
	return func() tea.Msg {
		// liest "git config --get <key>"; leer bei Fehler/nicht gesetzt
		get := func(key string) string {
			out, err := exec.Command("git", "config", "--get", key).CombinedOutput()
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(out))
		}
		return gitInfoLoadedMsg{
			name:       get("user.name"),
			email:      get("user.email"),
			signingKey: get("user.signingkey"),
		}
	}
}
