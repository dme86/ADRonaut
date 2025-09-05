package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func saveCmd(m model) tea.Cmd {
	return func() tea.Msg {
		path, err := writeADR(m)
		return saveDoneMsg{path: path, err: err}
	}
}

func (m model) handleSaveDone(msg saveDoneMsg) (model, tea.Cmd) {
	m.saving = false
	m.err = msg.err
	if msg.err == nil {
		fmt.Println(okStyle.Render("✔ ADR gespeichert: ") + msg.path)
		// Draft entfernen, wenn vorhanden
		if dp := m.draftPath(); dp != "" {
			_ = os.Remove(dp)
		}
		return m, tea.Quit
	}
	return m, nil
}

func writeADR(m model) (string, error) {
	dir := "."
	if err := ensureDir(dir); err != nil {
		return "", err
	}

	no := m.editingNo
	oldPath := m.editingPath
	path := oldPath
	title := strings.TrimSpace(m.Title())

	if path == "" {
		var err error
		no, err = nextADRNumber(dir)
		if err != nil {
			return "", err
		}
		slug := noTitleSlug()
		if title != "" {
			slug = slugify(title)
		}
		path = filepath.Join(dir, fmt.Sprintf("ADR-%04d-%s.md", no, slug))
	} else if title != "" {
		desired := filepath.Join(dir, fmt.Sprintf("ADR-%04d-%s.md", no, slugify(title)))
		if desired != path {
			path = desired
		}
	}

	now := time.Now().Format("2006-01-02")

	created := strings.TrimSpace(m.createdDate)
	if created == "" {
		created = now
		m.createdDate = created
	}

	by := strings.TrimSpace(m.gitName)
	if by == "" {
		if strings.TrimSpace(m.gitEmail) != "" {
			by = m.gitEmail
		} else {
			by = "Unbekannt"
		}
	}
	editedAt := now
	m.lastEditedBy = by
	m.lastEditedAt = editedAt

	content := buildMarkdown(
		no,
		m.Title(),
		created,
		m.Status(),
		m.Beteiligte(),
		m.Tags(),
		m.gitName, m.gitEmail, m.gitSigningKey,
		by, editedAt,
		m.Kontext(), m.Entscheidung(), m.Alternativen(), m.Konsequenzen(),
	)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	if oldPath != "" && oldPath != path {
		_ = os.Remove(oldPath)
	}
	return path, nil
}

func buildMarkdownPreview(m model) string {
	no := m.editingNo
	today := time.Now().Format("2006-01-02")

	created := strings.TrimSpace(m.createdDate)
	if created == "" {
		created = today
	}

	by := strings.TrimSpace(m.gitName)
	if by == "" {
		if strings.TrimSpace(m.gitEmail) != "" {
			by = m.gitEmail
		} else {
			by = "Unbekannt"
		}
	}

	editedAt := strings.TrimSpace(m.lastEditedAt)
	if editedAt == "" {
		editedAt = today
	}

	md := buildMarkdown(
		no,
		m.Title(),
		created,
		m.Status(),
		m.Beteiligte(),
		m.Tags(),
		m.gitName, m.gitEmail, m.gitSigningKey,
		by, editedAt,
		m.Kontext(), m.Entscheidung(), m.Alternativen(), m.Konsequenzen(),
	)
	lines := strings.Split(md, "\n")
	if len(lines) > 20 {
		lines = lines[:20]
	}
	return strings.Join(lines, "\n")
}

func buildMarkdown(
	no int,
	title, createdDate, status, beteiligte, tags string,
	authorName, authorEmail, signingKey string,
	lastEditedBy, lastEditedAt string,
	kontext, entscheidung, alternativen, konsequenzen string,
) string {
	noTitle := title
	if no > 0 {
		noTitle = fmt.Sprintf("ADR %04d: %s", no, title)
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "# %s\n\n", noTitle)
	fmt.Fprintf(b, "| Feld | Wert |\n|------|------|\n")

	// Erstell-Datum
	if strings.TrimSpace(createdDate) != "" {
		fmt.Fprintf(b, "| Datum (erstellt) | %s |\n", createdDate)
	} else {
		// Fallback, sollte praktisch nicht mehr vorkommen
		fmt.Fprintf(b, "| Datum (erstellt) | %s |\n", time.Now().Format("2006-01-02"))
	}

	// Status
	fmt.Fprintf(b, "| Status | %s |\n", status)

	// Autor (aus git config)
	author := ""
	switch {
	case authorName != "" && authorEmail != "":
		author = fmt.Sprintf("%s <%s>", authorName, authorEmail)
	case authorName != "":
		author = authorName
	case authorEmail != "":
		author = authorEmail
	}
	if strings.TrimSpace(author) != "" {
		fmt.Fprintf(b, "| Autor | %s |\n", author)
	}

	// Signing-Key
	if strings.TrimSpace(signingKey) != "" {
		fmt.Fprintf(b, "| Signing-Key | %s |\n", signingKey)
	}

	// Zuletzt editiert (neu)
	if strings.TrimSpace(lastEditedBy) != "" {
		fmt.Fprintf(b, "| Zuletzt editiert von | %s |\n", lastEditedBy)
	}
	if strings.TrimSpace(lastEditedAt) != "" {
		fmt.Fprintf(b, "| Zuletzt editiert am | %s |\n", lastEditedAt)
	}

	// Beteiligte/Tags
	if strings.TrimSpace(beteiligte) != "" {
		fmt.Fprintf(b, "| Beteiligte | %s |\n", beteiligte)
	}
	if strings.TrimSpace(tags) != "" {
		fmt.Fprintf(b, "| Tags | %s |\n", tags)
	}

	b.WriteString("\n## Kontext\n")
	if strings.TrimSpace(kontext) == "" {
		kontext = "(noch offen)"
	}
	b.WriteString(kontext + "\n\n")

	b.WriteString("## Entscheidung\n")
	if strings.TrimSpace(entscheidung) == "" {
		entscheidung = "(noch offen)"
	}
	b.WriteString(entscheidung + "\n\n")

	b.WriteString("## Alternativen\n")
	if strings.TrimSpace(alternativen) == "" {
		alternativen = "(keine oder noch offen)"
	}
	b.WriteString(alternativen + "\n\n")

	b.WriteString("## Konsequenzen\n")
	if strings.TrimSpace(konsequenzen) == "" {
		konsequenzen = "(noch offen)"
	}
	b.WriteString(konsequenzen + "\n\n")

	b.WriteString("## Verweise\n- \n")
	return b.String()
}

func (m model) buildSaveSummary() (summary string, missing []string) {
	title := strings.TrimSpace(m.title.Value())
	if title == "" {
		missing = append(missing, "Titel")
	}

	kontext := strings.TrimSpace(m.kontext.Value())
	if kontext == "" {
		missing = append(missing, "Kontext")
	}

	dec := m.entscheidung.NonEmptyCount()
	con := m.konsequenzen.NonEmptyCount()
	alt := m.alternativen.NonEmptyCount()
	if dec == 0 {
		missing = append(missing, "Entscheidung(en)")
	}

	beteiligteCount := len(splitCSV(m.beteiligte.Value()))
	tagsCount := len(splitCSV(m.tags.Value()))

	b := &strings.Builder{}
	fmt.Fprintf(b, "• Entscheidungen: %d\n", dec)
	fmt.Fprintf(b, "• Konsequenzen: %d\n", con)
	fmt.Fprintf(b, "• Alternativen: %d\n", alt)
	if beteiligteCount == 0 {
		fmt.Fprintf(b, "• Beteiligte: (keine)\n")
	} else {
		fmt.Fprintf(b, "• Beteiligte: %d\n", beteiligteCount)
	}
	if tagsCount == 0 {
		fmt.Fprintf(b, "• Tags: (keine)\n")
	} else {
		fmt.Fprintf(b, "• Tags: %d\n", tagsCount)
	}
	if kontext == "" {
		fmt.Fprintf(b, "• Kontext: (leer)\n")
	} else {
		fmt.Fprintf(b, "• Kontext: ok\n")
	}
	if title == "" {
		fmt.Fprintf(b, "• Titel: (leer)\n")
	} else {
		fmt.Fprintf(b, "• Titel: „%s“\n", title)
	}
	return b.String(), missing
}
