package app

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"path/filepath"
	"strings"
	"time"
)

type fileOption struct {
	Label string
	Path  string
	No    int
	Draft bool
}

const newAdrSentinel = "__NEW_ADR__"

type searchDoc struct {
	Title, Status, Beteiligte, Tags                   string
	Kontext, Entscheidung, Alternativen, Konsequenzen string
	Full                                              string // sämtlicher Text in Kleinbuchstaben für Volltext
}

type model struct {
	// Startup-Picker
	startup     bool
	allOptions  []fileOption
	pickOptions []fileOption
	pickIdx     int
	filter      textinput.Model
	searchIndex map[string]string

	createdDate  string
	lastEditedBy string
	lastEditedAt string

	gitName       string
	gitEmail      string
	gitSigningKey string

	searchDocs map[string]searchDoc
	hitBadges  map[string][]badge
	hitSnippet map[string]string
	lastQuery  string

	// Edit-Kontext
	editingPath    string
	editingNo      int
	draftFixedPath string

	step int

	// Inputs
	title     textinput.Model
	statusIdx int
	kontext   textarea.Model

	// Listen-Felder
	entscheidung listField
	konsequenzen listField
	alternativen listField

	beteiligte textinput.Model
	tags       textinput.Model

	// UI
	width  int
	height int

	saving     bool
	err        error
	confirming bool
}

func initialModel() model {
	m := model{}

	opts := scanADRFiles(".")
	drafts := scanDrafts(".")
	all := make([]fileOption, 0, 1+len(drafts)+len(opts))
	all = append(all, fileOption{Label: "➕ Neuer ADR", Path: newAdrSentinel, No: 0})
	all = append(all, drafts...)
	all = append(all, opts...)
	m.allOptions = all
	m.searchDocs = buildSearchDocs(all)
	//	m.searchIndex = buildSearchIndex(all)
	m.pickOptions = all
	m.startup = true
	m.pickIdx = 0

	// Suchfeld
	f := textinput.New()
	f.Placeholder = "Tippen zum Filtern (Titel/Tags/Inhalt)"
	f.Prompt = "🔎 "
	f.CharLimit = 256
	f.Width = 40
	m.filter = f
	m.filter.Focus()

	m.applyFilter("") // initial alle anzeigen
	m.startup = true
	m.pickIdx = 0

	t := textinput.New()
	t.Placeholder = "Kurzer Titel, z. B. \"Wahl des Service Mesh\""
	t.CharLimit = 256
	t.Prompt = "> "
	t.Width = 80
	m.title = t

	w := 80
	mk := textarea.New()
	mk.Placeholder = "Beschreibe den Kontext: Problem, Rahmenbedingungen, Annahmen …"
	mk.SetHeight(7)
	mk.SetWidth(w)
	mk.ShowLineNumbers = false
	m.kontext = mk

	m.entscheidung = newListField("Entscheidung", "Beschreibe einen Entscheidungspunkt …", 5, w)
	m.konsequenzen = newListField("Konsequenzen", "Positive/negative Konsequenz …", 5, w)
	m.alternativen = newListField("Alternativen", "Betrachtete Alternative mit Pros/Cons …", 5, w)

	b := textinput.New()
	b.Placeholder = "Beteiligte (Komma-getrennt), z. B. Ich, Du, Team Platform"
	b.CharLimit = 256
	b.Prompt = "> "
	b.Width = 80
	m.beteiligte = b

	tg := textinput.New()
	tg.Placeholder = "Tags (Komma-getrennt), z. B. architektur, sicherheit"
	tg.CharLimit = 256
	tg.Prompt = "> "
	tg.Width = 80
	m.tags = tg

	m.statusIdx = 0 // Vorgeschlagen

	// Wenn es weder ADR-Dateien noch Drafts gibt -> direkt in den Editor springen
	if len(opts) == 0 && len(drafts) == 0 {
		m.startup = false
		m.editingPath = ""
		m.editingNo = 0
		m.draftFixedPath = filepath.Join(autosaveDir, fmt.Sprintf("new-%d.draft.json", time.Now().UnixNano()))
		m.step = 0
		_ = m.title.Focus() // Cursor direkt in den Titel
	} else {
		// Nur wenn wir den Picker zeigen, die Suche befüllen
		m.applyFilter("") // initial alle anzeigen
		m.startup = true
		m.pickIdx = 0
	}

	return m
}

func (m model) Init() tea.Cmd {
	if m.startup {
		return tea.Batch(textinput.Blink, loadGitInfoCmd())
	}
	return tea.Batch(textinput.Blink, m.focusForStep(), scheduleAutosave(), loadGitInfoCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch mm := msg.(type) {

	case gitInfoLoadedMsg:
		m.gitName = mm.name
		m.gitEmail = mm.email
		m.gitSigningKey = mm.signingKey
		return m, nil

	case saveDoneMsg:
		return m.handleSaveDone(mm)

	case tea.WindowSizeMsg:
		m.width, m.height = mm.Width, mm.Height
		w := max(50, m.width-2*framePadding)

		m.kontext.SetWidth(w)
		m.entscheidung.setWidthAll(w)
		m.konsequenzen.setWidthAll(w)
		m.alternativen.setWidthAll(w)

		m.title.Width = w
		m.beteiligte.Width = w
		m.tags.Width = w
		return m, nil

	case tea.KeyMsg:
		// --- Startup Picker ---
		// --- Startup Picker ---
		if m.startup {
			// Navigation/Fokuswechsel
			switch mm.String() {
			case "tab":
				// Suche -> Liste
				if m.filter.Focused() {
					m.filter.Blur()
					if m.pickIdx >= len(m.pickOptions) {
						m.pickIdx = 0
					}
					return m, nil
				}
				// in der Liste weiter nach unten
				if len(m.pickOptions) > 0 {
					m.pickIdx = (m.pickIdx + 1) % len(m.pickOptions)
				}
				return m, nil

			case "shift+tab":
				// Liste -> zurück zur Suche
				if !m.filter.Focused() {
					_ = m.filter.Focus()
					// Cursor ans Ende setzen (optional):
					// m.filter.SetCursor(len(m.filter.Value()))
					return m, nil
				}
				return m, nil

			case "down", "ctrl+n":
				if !m.filter.Focused() && len(m.pickOptions) > 0 {
					m.pickIdx = (m.pickIdx + 1) % len(m.pickOptions)
				}
				return m, nil

			case "up", "ctrl+p":
				if !m.filter.Focused() && len(m.pickOptions) > 0 {
					m.pickIdx--
					if m.pickIdx < 0 {
						m.pickIdx = len(m.pickOptions) - 1
					}
				}
				return m, nil

			case "enter":
				choice := m.pickOptions[m.pickIdx]
				if choice.Path == newAdrSentinel {
					m.startup = false
					m.editingPath = ""
					m.editingNo = 0
					m.draftFixedPath = filepath.Join(autosaveDir, fmt.Sprintf("new-%d.draft.json", time.Now().UnixNano()))
					m.step = 0
					return m, tea.Batch(m.focusForStep(), scheduleAutosave())
				}
				if choice.Draft {
					if err := m.loadDraft(choice.Path); err != nil {
						m.err = fmt.Errorf("Konnte Entwurf nicht laden: %w", err)
						return m, nil
					}
					m.draftFixedPath = choice.Path
					m.startup = false
					m.step = 0
					return m, tea.Batch(m.focusForStep(), scheduleAutosave())
				}
				if err := m.loadFromFile(choice.Path); err != nil {
					m.err = fmt.Errorf("Konnte Datei nicht laden: %w", err)
					return m, nil
				}
				m.draftFixedPath = ""
				m.startup = false
				m.editingPath = choice.Path
				m.step = 0
				return m, tea.Batch(m.focusForStep(), scheduleAutosave())

			case "esc", "ctrl+c":
				return m, tea.Quit
			}

			// Alle anderen Tasten gehen in das Suchfeld (und filtern live).
			// Wenn die Suche nicht fokussiert ist und der User tippt/backspacet,
			// Fokus zurück auf die Suche.
			if !m.filter.Focused() && (mm.Type == tea.KeyRunes || mm.Type == tea.KeyBackspace) {
				_ = m.filter.Focus()
			}
			var cmd tea.Cmd
			old := m.filter.Value()
			m.filter, cmd = m.filter.Update(mm)
			if m.filter.Value() != old {
				m.applyFilter(m.filter.Value())
				m.pickIdx = 0
			}
			return m, cmd

		}

		// --- Schritte mit TAB/SHIFT+TAB ---
		switch mm.String() {
		case "tab":
			if m.step < 8 {
				m.step++
				return m, m.focusForStep()
			}
			return m, nil
		case "shift+tab":
			if m.step > 0 {
				m.step--
				return m, m.focusForStep()
			}
			return m, nil
		}

		// Status-Auswahl über CTRL+N/CTRL+P
		switch mm.String() {
		case "ctrl+n":
			if m.step == 1 {
				m.statusIdx = (m.statusIdx + 1) % len(statuses)
				return m, nil
			}
		case "ctrl+p":
			if m.step == 1 {
				m.statusIdx--
				if m.statusIdx < 0 {
					m.statusIdx = len(statuses) - 1
				}
				return m, nil
			}
		}

		switch mm.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}

		switch mm.String() {
		case "space", "enter":
			if m.step == 1 {
				m.step++
				return m, m.focusForStep()
			}
			if m.step == 0 || m.step == 6 || m.step == 7 {
				m.step++
				return m, m.focusForStep()
			}
		}

		if m.step == 8 && !m.saving {
			switch strings.ToLower(mm.String()) {
			case "s":
				if !m.confirming {
					m.confirming = true
					return m, nil
				}
				m.saving = true
				return m, saveCmd(m)
			case "j", "enter":
				if m.confirming {
					m.saving = true
					return m, saveCmd(m)
				}
			case "n", "esc":
				if m.confirming {
					m.confirming = false
					return m, nil
				}
			case "b":
				if m.confirming {
					m.confirming = false
					return m, nil
				}
				m.step = 7
				return m, m.focusForStep()
			}
		}
	}

	// Autosave Tick / Done
	switch msg := msg.(type) {
	case autosaveTickMsg:
		return m, tea.Batch(autosaveCmd(m), scheduleAutosave())
	case autosaveDoneMsg:
		_ = msg
		return m, nil
	}

	// Feld-spezifische Updates
	switch m.step {
	case 0:
		var cmd tea.Cmd
		m.title, cmd = m.title.Update(msg)
		return m, cmd
	case 2:
		var cmd tea.Cmd
		m.kontext, cmd = m.kontext.Update(msg)
		return m, cmd
	case 3:
		cmd, _ := m.entscheidung.UpdateActive(msg)
		return m, cmd
	case 4:
		cmd, _ := m.konsequenzen.UpdateActive(msg)
		return m, cmd
	case 5:
		cmd, _ := m.alternativen.UpdateActive(msg)
		return m, cmd
	case 6:
		var cmd tea.Cmd
		m.beteiligte, cmd = m.beteiligte.Update(msg)
		return m, cmd
	case 7:
		var cmd tea.Cmd
		m.tags, cmd = m.tags.Update(msg)
		return m, cmd
	case 8:
		return m, nil
	}
	return m, nil
}

func (m *model) focusForStep() tea.Cmd {
	m.title.Blur()
	m.beteiligte.Blur()
	m.tags.Blur()
	m.kontext.Blur()
	for i := range m.entscheidung.items {
		m.entscheidung.items[i].Blur()
	}
	for i := range m.konsequenzen.items {
		m.konsequenzen.items[i].Blur()
	}
	for i := range m.alternativen.items {
		m.alternativen.items[i].Blur()
	}

	switch m.step {
	case 0:
		return m.title.Focus()
	case 2:
		return m.kontext.Focus()
	case 3:
		return m.entscheidung.focusCurrent()
	case 4:
		return m.konsequenzen.focusCurrent()
	case 5:
		return m.alternativen.focusCurrent()
	case 6:
		return m.beteiligte.Focus()
	case 7:
		return m.tags.Focus()
	}
	return nil
}
