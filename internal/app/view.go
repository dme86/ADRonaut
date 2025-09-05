package app

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"path/filepath"
	"strings"
)

func (m model) header() string {
	prefix := "ADRonaut"
	if m.editingPath != "" {
		prefix += " – Bearbeite: " + filepath.Base(m.editingPath)
	}
	steps := []string{
		"Titel", "Status", "Kontext", "Entscheidung",
		"Konsequenzen", "Alternativen", "Beteiligte", "Tags", "Speichern",
	}
	parts := make([]string, len(steps))
	for i, s := range steps {
		if i == m.step {
			parts[i] = activeStyle.Render(s)
		} else {
			parts[i] = s
		}
	}
	titleLine := titleStyle.Render(prefix)
	menuLine := strings.Join(parts, "  ·  ")
	return lipgloss.JoinVertical(lipgloss.Left, titleLine, menuLine) + "\n"
}

func (m model) help(keys string) string { return helpStyle.Render(keys) }

func (m model) viewPicker() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("ADRonaut – Datei auswählen oder neuen ADR anlegen"))
	b.WriteString("\n\n")

	// Suchfeld
	b.WriteString(m.filter.View())
	b.WriteString("\n\n")

	hasDraft := false
	// (Rest unverändert …)

	if len(m.pickOptions) == 1 { // nur "Neuer ADR"
		b.WriteString("(Keine ADRs im aktuellen Verzeichnis gefunden)\n\n")
	}
	if hasDraft {
		b.WriteString(helpStyle.Render("Es liegen unveröffentlichte Entwürfe vor – du kannst sie wiederherstellen.") + "\n\n")
	}

	// Liste rendern
	for i, opt := range m.pickOptions {
		st := optionStyle
		isSel := (!m.filter.Focused() && i == m.pickIdx)
		if isSel {
			st = selectedStyle
		}

		line := st.Render(opt.Label)

		// Badges anhängen
		if bs := m.hitBadges[opt.Path]; len(bs) > 0 {
			for _, bb := range bs {
				line += " " + chipWithCount(bb)
			}
		}
		b.WriteString(line + "\n")

		// Snippet nur für die aktuelle Auswahl zeigen (gegen Clutter)
		if isSel {
			if sn := strings.TrimSpace(m.hitSnippet[opt.Path]); sn != "" {
				b.WriteString("  " + sn + "\n")
			}
		}
	}

	// Kontextsensitive Hilfe
	helpText := "TAB oder ↑/↓ wählen · SHIFT+Tab zurück zur Suche · ENTER öffnen · ESC/STRG+C beenden"
	if m.filter.Focused() {
		helpText = "TAB zur Liste · ENTER öffnen · ESC/STRG+C beenden"
	}
	b.WriteString("\n" + m.help(helpText))

	return lipgloss.NewStyle().Padding(0, framePadding).Render(b.String())
}

func (m model) View() string {
	if m.startup {
		return m.viewPicker()
	}

	var b strings.Builder
	b.WriteString(m.header())

	switch m.step {
	case 0:
		b.WriteString(labelStyle.Render("Titel") + "\n")
		b.WriteString(m.title.View())
		b.WriteString("\n\n" + m.help("TAB weiter · SHIFT+TAB zurück · ENTER weiter · ESC/STRG+C abbrechen"))
	case 1:
		b.WriteString(labelStyle.Render("Status") + "\n")
		for i, s := range statuses {
			st := optionStyle
			if i == m.statusIdx {
				st = selectedStyle
			}
			b.WriteString(st.Render(s))
			if i != len(statuses)-1 {
				b.WriteString("   ")
			}
		}
		b.WriteString("\n\n" + m.help("CTRL+N/CTRL+P wählen · ENTER/SPACE bestätigen · TAB weiter · SHIFT+TAB zurück"))
	case 2:
		b.WriteString(labelStyle.Render("Kontext") + "\n")
		b.WriteString(m.kontext.View())
		b.WriteString("\n\n" + m.help("TAB weiter · SHIFT+TAB zurück"))
	case 3:
		b.WriteString(labelStyle.Render("Entscheidung"))
		b.WriteString(fmt.Sprintf("  (%d/%d)\n", m.entscheidung.idx+1, len(m.entscheidung.items)))
		b.WriteString(m.entscheidung.current().View())
		b.WriteString("\n\n" + m.help("CTRL+O neuer Punkt · CTRL+G nächster Punkt · CTRL+X Punkt löschen · TAB weiter · SHIFT+TAB zurück"))
	case 4:
		b.WriteString(labelStyle.Render("Konsequenzen"))
		b.WriteString(fmt.Sprintf("  (%d/%d)\n", m.konsequenzen.idx+1, len(m.konsequenzen.items)))
		b.WriteString(m.konsequenzen.current().View())
		b.WriteString("\n\n" + m.help("CTRL+O neuer Punkt · CTRL+G nächster Punkt · CTRL+X Punkt löschen · TAB weiter · SHIFT+TAB zurück"))

	case 5:
		b.WriteString(labelStyle.Render("Alternativen"))
		b.WriteString(fmt.Sprintf("  (%d/%d)\n", m.alternativen.idx+1, len(m.alternativen.items)))
		b.WriteString(m.alternativen.current().View())
		b.WriteString("\n\n" + m.help("CTRL+O neuer Punkt · CTRL+G nächster Punkt · CTRL+X Punkt löschen · TAB weiter · SHIFT+TAB zurück"))

	case 6:
		b.WriteString(labelStyle.Render("Beteiligte (Komma-getrennt)") + "\n")
		b.WriteString(m.beteiligte.View())
		b.WriteString("\n\n" + m.help("TAB weiter · SHIFT+TAB zurück · ENTER weiter"))
	case 7:
		b.WriteString(labelStyle.Render("Tags (Komma-getrennt)") + "\n")
		b.WriteString(m.tags.View())
		b.WriteString("\n\n" + m.help("TAB weiter · SHIFT+TAB zurück · ENTER weiter"))
	case 8:
		b.WriteString(labelStyle.Render("Speichern") + "\n")
		preview := buildMarkdownPreview(m)

		if m.saving {
			b.WriteString(okStyle.Render("Speichere …"))
		} else if m.err != nil {
			b.WriteString(errorStyle.Render("Fehler: ") + m.err.Error())
			b.WriteString("\n\n" + m.help("S erneut speichern · B/SHIFT+TAB zurück"))
		} else if m.confirming {
			sum, missing := m.buildSaveSummary()
			b.WriteString("Vorschau (Kopf):\n")
			b.WriteString(preview)
			b.WriteString("\n\n" + labelStyle.Render("Überprüfung") + "\n")
			b.WriteString(sum)
			if len(missing) > 0 {
				b.WriteString("\n" + errorStyle.Render("Fehlt/leer: "+strings.Join(missing, ", ")))
			}
			b.WriteString("\n\n" + m.help("J/ENTER speichern · N/B/ESC abbrechen"))
		} else {
			b.WriteString("Vorschau (Kopf):\n")
			b.WriteString(preview)
			b.WriteString("\n\n" + m.help("S Speicherdialog öffnen · B/SHIFT+TAB zurück · ESC/STRG+C abbrechen"))
		}
	}

	return lipgloss.NewStyle().Padding(0, framePadding).Render(b.String())
}
