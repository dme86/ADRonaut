package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

/* ------------------------------ Autosave ---------------------------------- */

const (
	autosaveDir      = ".adronaut"
	autosaveInterval = 3 * time.Second
)

/* ------------------------------ Counter --------------------------------- */

type badge struct {
	Label string
	Count int
}

/* ------------------------------ Messages --------------------------------- */

type saveDoneMsg struct {
	path string
	err  error
}

type autosaveTickMsg struct{}
type autosaveDoneMsg struct {
	path string
	err  error
}

/* --------- Listen-Feld (für Entscheidung/Konsequenzen/Alternativen) ------ */

type listField struct {
	name        string
	placeholder string
	items       []textarea.Model
	idx         int // aktiver Punkt
}

func newListField(name, placeholder string, h, w int) listField {
	lf := listField{name: name, placeholder: placeholder}
	lf.items = []textarea.Model{newTA(placeholder, h, w)}
	return lf
}

func newTA(ph string, h, w int) textarea.Model {
	t := textarea.New()
	t.Placeholder = ph
	t.SetHeight(h)
	t.SetWidth(w)
	t.ShowLineNumbers = false
	return t
}

func (lf *listField) current() *textarea.Model { return &lf.items[lf.idx] }

func (lf *listField) addAfterCurrent(h, w int) {
	newItem := newTA(lf.placeholder, h, w)
	if lf.idx >= len(lf.items)-1 {
		lf.items = append(lf.items, newItem)
		lf.idx = len(lf.items) - 1
	} else {
		lf.items = append(lf.items[:lf.idx+1], append([]textarea.Model{newItem}, lf.items[lf.idx+1:]...)...)
		lf.idx++
	}
}

func (lf *listField) deleteCurrent() {
	if len(lf.items) == 1 {
		lf.items[0].SetValue("")
		return
	}
	lf.items = append(lf.items[:lf.idx], lf.items[lf.idx+1:]...)
	if lf.idx >= len(lf.items) {
		lf.idx = len(lf.items) - 1
	}
}

func (lf *listField) setWidthAll(w int) {
	for i := range lf.items {
		lf.items[i].SetWidth(w)
	}
}
func (lf *listField) setHeightAll(h int) {
	for i := range lf.items {
		lf.items[i].SetHeight(h)
	}
}
func (lf *listField) focusCurrent() tea.Cmd { return lf.current().Focus() }

func (lf *listField) UpdateActive(msg tea.Msg) (tea.Cmd, bool) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "ctrl+o", "ctrl+enter":
			lf.addAfterCurrent(lf.current().Height(), lf.current().Width())
			return lf.focusCurrent(), true
		case "ctrl+g":
			if len(lf.items) > 0 {
				lf.idx = (lf.idx + 1) % len(lf.items)
			}
			return lf.focusCurrent(), true
		case "ctrl+x":
			lf.deleteCurrent()
			return lf.focusCurrent(), true
		}
	}
	var cmd tea.Cmd
	*lf.current(), cmd = lf.current().Update(msg)
	return cmd, false
}

func (lf *listField) Markdown() string {
	var parts []string
	for i, it := range lf.items {
		txt := strings.TrimSpace(it.Value())
		if txt == "" || strings.EqualFold(txt, "(noch offen)") {
			continue
		}
		parts = append(parts, fmt.Sprintf("%d. %s", i+1, txt))
	}
	if len(parts) == 0 {
		return "(noch offen)"
	}
	return strings.Join(parts, "\n\n")
}

func (lf *listField) NonEmptyCount() int {
	c := 0
	for i := range lf.items {
		v := strings.TrimSpace(lf.items[i].Value())
		if v != "" && !strings.EqualFold(v, "(noch offen)") {
			c++
		}
	}
	return c
}

// Draft-Helfer

func chipWithCount(b badge) string {
	bg := chipColors[b.Label]
	st := chipBase
	if bg != "" {
		st = st.Background(lipgloss.Color(bg))
	}
	// {Label} [Count]
	return st.Render(fmt.Sprintf("{%s} [%d]", b.Label, b.Count))
}

func (lf *listField) Values() []string {
	out := make([]string, 0, len(lf.items))
	for i := range lf.items {
		if v := strings.TrimSpace(lf.items[i].Value()); v != "" && !strings.EqualFold(v, "(noch offen)") {
			out = append(out, v)
		}
	}
	return out
}
func (lf *listField) SetFromSlice(items []string, h, w int) {
	if len(items) == 0 {
		lf.items = []textarea.Model{newTA(lf.placeholder, h, w)}
		lf.idx = 0
		return
	}
	lf.items = make([]textarea.Model, 0, len(items))
	for _, it := range items {
		t := newTA(lf.placeholder, h, w)
		t.SetValue(it)
		lf.items = append(lf.items, t)
	}
	lf.idx = 0
}

func (lf *listField) SetFromMarkdown(text string, h, w int) {
	text = strings.TrimSpace(text)
	if text == "" || strings.EqualFold(text, "(noch offen)") {
		lf.items = []textarea.Model{newTA(lf.placeholder, h, w)}
		lf.idx = 0
		return
	}
	re := regexp.MustCompile(`(?m)^\s*\d+\.\s+(.*\S)\s*$`)
	matches := re.FindAllStringSubmatch(text, -1)
	var items []string
	for _, m := range matches {
		items = append(items, strings.TrimSpace(m[1]))
	}
	if len(items) == 0 {
		items = []string{text}
	}
	lf.items = make([]textarea.Model, 0, len(items))
	for _, it := range items {
		t := newTA(lf.placeholder, h, w)
		t.SetValue(it)
		lf.items = append(lf.items, t)
	}
	lf.idx = 0
}

/* --------------------------------- Picker -------------------------------- */

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

/* --------------------------------- Model ---------------------------------- */

type model struct {
	// Startup-Picker
	startup     bool
	allOptions  []fileOption
	pickOptions []fileOption
	pickIdx     int
	filter      textinput.Model
	searchIndex map[string]string

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

/* --------------------------- Model Construction --------------------------- */

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
		return textinput.Blink
	}
	// Direkt im Editor: Fokus + Autosave starten
	return tea.Batch(textinput.Blink, m.focusForStep(), scheduleAutosave())
}

/* ---------------------------------- View ---------------------------------- */

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

/* ------------------------------- Counter ------------------------------------ */

// zählt alle (nicht überlappenden) Treffer aller Tokens – case-insensitive
func countAllMatchesCI(text string, toks []string) int {
	if text == "" {
		return 0
	}
	total := 0
	lower := strings.ToLower(text)
	for _, t := range toks {
		if t == "" {
			continue
		}
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(t))
		total += len(re.FindAllStringIndex(lower, -1))
	}
	return total
}

/* ------------------------------- Focus ------------------------------------ */

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

/* --------------------------------- Update --------------------------------- */

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch mm := msg.(type) {

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

/* ------------------------------- File Picker ---------------------------------- */

// Relevanzgewichtung: Titel-Start > Titel-Contain > Rest im Inhalt
func (m *model) applyFilter(q string) {
	m.lastQuery = q
	q = strings.ToLower(strings.TrimSpace(q))
	base := m.allOptions
	if len(base) == 0 {
		m.pickOptions = nil
		return
	}

	out := make([]fileOption, 0, len(base))
	out = append(out, base[0]) // "+ Neuer ADR" immer oben
	m.hitBadges = make(map[string][]badge)
	m.hitSnippet = make(map[string]string)

	if q == "" {
		out = append(out, base[1:]...)
		m.pickOptions = out
		if m.pickIdx >= len(m.pickOptions) {
			m.pickIdx = 0
		}
		return
	}

	toks := strings.Fields(q)

	type scored struct {
		opt   fileOption
		score int
	}
	hits := []scored{}

	for _, opt := range base[1:] { // sentinel überspringen
		doc := m.searchDocs[opt.Path]
		label := strings.ToLower(opt.Label)

		// AND: jedes Token muss vorkommen
		combined := label + " " + doc.Full
		ok := true
		for _, t := range toks {
			if !strings.Contains(combined, t) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}

		// ---- BADGES MIT COUNTS ----
		cFile := countAllMatchesCI(opt.Label, toks)
		cTitle := countAllMatchesCI(doc.Title, toks)
		cTags := countAllMatchesCI(doc.Tags, toks)
		cBeteiligte := countAllMatchesCI(doc.Beteiligte, toks)
		cKontext := countAllMatchesCI(doc.Kontext, toks)
		cEntsch := countAllMatchesCI(doc.Entscheidung, toks)
		cAlt := countAllMatchesCI(doc.Alternativen, toks)
		cKonseq := countAllMatchesCI(doc.Konsequenzen, toks)

		badges := make([]badge, 0, 8)
		add := func(lbl string, c int) {
			if c > 0 {
				badges = append(badges, badge{Label: lbl, Count: c})
			}
		}
		add("Dateiname", cFile)
		add("Titel", cTitle)
		add("Tags", cTags)
		add("Beteiligte", cBeteiligte)
		add("Kontext", cKontext)
		add("Entscheidung", cEntsch)
		add("Alternativen", cAlt)
		add("Konsequenzen", cKonseq)

		// ---- SCORING (deine Basis + Bonus nach Count) ----
		s := 0
		if strings.HasPrefix(label, q) {
			s += 120
		}
		if strings.Contains(label, q) {
			s += 80
		}
		for _, t := range toks {
			if strings.Contains(strings.ToLower(doc.Title), t) {
				s += 70
			}
			if strings.Contains(strings.ToLower(doc.Tags), t) {
				s += 50
			}
			if strings.Contains(strings.ToLower(doc.Beteiligte), t) {
				s += 30
			}
			if strings.Contains(strings.ToLower(doc.Kontext), t) ||
				strings.Contains(strings.ToLower(doc.Entscheidung), t) ||
				strings.Contains(strings.ToLower(doc.Alternativen), t) ||
				strings.Contains(strings.ToLower(doc.Konsequenzen), t) {
				s += 10
			}
		}
		// Bonus: reine Trefferanzahl je Feld
		s += 2*cTitle + cTags + cBeteiligte + cKontext + cEntsch + cAlt + cKonseq + cFile

		// ---- SNIPPET (Feld mit größter Trefferzahl) ----
		var snippet string
		{
			max := 0
			srcLabel, srcText := "", ""
			check := func(c int, l, t string) {
				if c > max {
					max, srcLabel, srcText = c, l, t
				}
			}
			check(cTitle, "Titel", doc.Title)
			check(cTags, "Tags", doc.Tags)
			check(cBeteiligte, "Beteiligte", doc.Beteiligte)
			check(cKontext, "Kontext", doc.Kontext)
			check(cEntsch, "Entscheidung", doc.Entscheidung)
			check(cAlt, "Alternativen", doc.Alternativen)
			check(cKonseq, "Konsequenzen", doc.Konsequenzen)
			check(cFile, "Dateiname", opt.Label)

			if srcText != "" {
				snippet = snippetStyle.Render(srcLabel+": ") + highlightAll(srcText, toks)
			}
		}

		// speichern
		m.hitBadges[opt.Path] = badges
		m.hitSnippet[opt.Path] = snippet
		hits = append(hits, scored{opt: opt, score: s})
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score == hits[j].score {
			if hits[i].opt.No == hits[j].opt.No {
				return hits[i].opt.Label < hits[j].opt.Label
			}
			if hits[i].opt.No == 0 {
				return false
			}
			if hits[j].opt.No == 0 {
				return true
			}
			return hits[i].opt.No < hits[j].opt.No
		}
		return hits[i].score > hits[j].score
	})

	for _, h := range hits {
		out = append(out, h.opt)
	}
	m.pickOptions = out
	if m.pickIdx >= len(m.pickOptions) {
		m.pickIdx = 0
	}
}

func highlightAll(text string, toks []string) string {
	out := text
	for _, t := range toks {
		if t == "" {
			continue
		}
		// case-insensitive ersetzen
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(t))
		out = re.ReplaceAllStringFunc(out, func(m string) string {
			return highlightStyle.Render(m)
		})
	}
	return out
}

// Liest Inhalte für die Volltextsuche (MD + Drafts)
func buildSearchDocs(opts []fileOption) map[string]searchDoc {
	idx := make(map[string]searchDoc, len(opts))
	for _, o := range opts {
		if o.Path == newAdrSentinel {
			continue
		}
		var d searchDoc
		if strings.HasSuffix(o.Path, ".md") {
			d = parseADRForSearch(o.Path)
		} else if strings.HasSuffix(o.Path, ".draft.json") {
			d = parseDraftForSearch(o.Path)
		}
		// Fulltext (alles kleingeschrieben)
		var sb strings.Builder
		sb.WriteString(strings.ToLower(o.Label) + " ")
		sb.WriteString(strings.ToLower(d.Title) + " ")
		sb.WriteString(strings.ToLower(d.Status) + " ")
		sb.WriteString(strings.ToLower(d.Beteiligte) + " ")
		sb.WriteString(strings.ToLower(d.Tags) + " ")
		sb.WriteString(strings.ToLower(d.Kontext) + " ")
		sb.WriteString(strings.ToLower(d.Entscheidung) + " ")
		sb.WriteString(strings.ToLower(d.Alternativen) + " ")
		sb.WriteString(strings.ToLower(d.Konsequenzen))
		d.Full = sb.String()
		idx[o.Path] = d
	}
	return idx
}

func parseADRForSearch(path string) searchDoc {
	b, err := os.ReadFile(path)
	if err != nil {
		return searchDoc{}
	}
	txt := string(b)

	// Titel + Nummer
	h1 := regexp.MustCompile(`(?m)^#\s*(?:ADR\s+\d+:\s*)?(.*)$`).FindStringSubmatch(txt)
	title := ""
	if len(h1) >= 2 {
		t := strings.TrimSpace(h1[1])
		if !strings.HasPrefix(t, "|") {
			title = t
		}
	}

	// Tabellen-Felder
	rowRe := regexp.MustCompile(`(?m)^\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|`)
	var status, beteiligte, tags string
	for _, mm := range rowRe.FindAllStringSubmatch(txt, -1) {
		key := strings.TrimSpace(strings.ToLower(mm[1]))
		val := strings.TrimSpace(mm[2])
		switch key {
		case "status":
			status = val
		case "beteiligte":
			beteiligte = val
		case "tags":
			tags = val
		}
	}

	// Abschnitte
	kontext := extractSection(txt, "Kontext")
	entscheidung := extractSection(txt, "Entscheidung")
	alternativen := extractSection(txt, "Alternativen")
	konsequenzen := extractSection(txt, "Konsequenzen")

	return searchDoc{
		Title: title, Status: status, Beteiligte: beteiligte, Tags: tags,
		Kontext: kontext, Entscheidung: entscheidung, Alternativen: alternativen, Konsequenzen: konsequenzen,
	}
}

func parseDraftForSearch(path string) searchDoc {
	b, err := os.ReadFile(path)
	if err != nil {
		return searchDoc{}
	}
	var d struct {
		Title, Kontext                           string
		Entscheidung, Konsequenzen, Alternativen []string
		Beteiligte, Tags                         string
		StatusIdx                                int
	}
	if json.Unmarshal(b, &d) != nil {
		return searchDoc{}
	}
	status := ""
	if d.StatusIdx >= 0 && d.StatusIdx < len(statuses) {
		status = statuses[d.StatusIdx]
	}
	return searchDoc{
		Title:        d.Title,
		Status:       status,
		Beteiligte:   d.Beteiligte,
		Tags:         d.Tags,
		Kontext:      d.Kontext,
		Entscheidung: strings.Join(d.Entscheidung, " "),
		Alternativen: strings.Join(d.Alternativen, " "),
		Konsequenzen: strings.Join(d.Konsequenzen, " "),
	}
}

/* --------------------------------- Save ----------------------------------- */

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

/* ----------------------------- Scan & Parsing ----------------------------- */

var adrFileRe = regexp.MustCompile(`^ADR-([0-9]{4})-.*\.md$`)

func scanADRFiles(dir string) []fileOption {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	opts := make([]fileOption, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !adrFileRe.MatchString(name) {
			continue
		}
		no := 0
		if m := adrFileRe.FindStringSubmatch(name); len(m) == 2 {
			fmt.Sscanf(m[1], "%04d", &no)
		}
		path := filepath.Join(dir, name)
		lbl := quickTitleForFile(path)
		if lbl == "" {
			lbl = name
		}
		opts = append(opts, fileOption{
			Label: fmt.Sprintf("%04d — %s", no, lbl),
			Path:  path,
			No:    no,
		})
	}
	sort.Slice(opts, func(i, j int) bool {
		if opts[i].No == 0 && opts[j].No == 0 {
			return opts[i].Label < opts[j].Label
		}
		if opts[i].No == 0 {
			return false
		}
		if opts[j].No == 0 {
			return true
		}
		return opts[i].No < opts[j].No
	})
	return opts
}

// Drafts in ./.adronaut anzeigen (neueste zuerst)
func scanDrafts(dir string) []fileOption {
	asDir := filepath.Join(dir, autosaveDir)
	dh, err := os.ReadDir(asDir)
	if err != nil {
		return nil
	}
	type item struct {
		opt fileOption
		mod time.Time
	}
	items := []item{}
	for _, e := range dh {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".draft.json") {
			continue
		}
		full := filepath.Join(asDir, name)
		title, base := draftTitlePreview(full)
		lbl := "🔄 Entwurf: " + title
		if title == "" {
			lbl = "🔄 Entwurf: " + base
		}
		st, _ := os.Stat(full)
		mt := time.Time{}
		if st != nil {
			mt = st.ModTime()
		}
		items = append(items, item{
			opt: fileOption{Label: lbl, Path: full, No: 0, Draft: true},
			mod: mt,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod.After(items[j].mod) })
	out := make([]fileOption, 0, len(items))
	for _, it := range items {
		out = append(out, it.opt)
	}
	return out
}

func quickTitleForFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := string(b)

	re := regexp.MustCompile(`(?m)^#\s*(?:ADR\s+\d+:\s*)?(.*)$`)
	m := re.FindStringSubmatch(text)
	if len(m) != 2 {
		return ""
	}

	t := strings.TrimSpace(m[1])

	if t == "" || strings.HasPrefix(t, "|") {
		return ""
	}

	return t
}

type parsedADR struct {
	No           int
	Title        string
	Date         string
	Status       string
	Beteiligte   string
	Tags         string
	Kontext      string
	Entscheidung string
	Alternativen string
	Konsequenzen string
}

func (m *model) loadFromFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	txt := string(content)

	pa := parsedADR{}

	base := filepath.Base(path)
	if mm := adrFileRe.FindStringSubmatch(base); len(mm) == 2 {
		fmt.Sscanf(mm[1], "%04d", &pa.No)
	}

	// neu: Titel defensiv bereinigen – Tabellenzeile nicht als Titel übernehmen
	h1 := regexp.MustCompile(`(?m)^#\s*(?:ADR\s+(\d+):\s*)?(.*)$`).FindStringSubmatch(txt)
	if len(h1) >= 3 {
		t := strings.TrimSpace(h1[2])
		if strings.HasPrefix(t, "|") { // z.B. "| Feld | Wert |"
			t = ""
		}
		pa.Title = t
		if pa.No == 0 && h1[1] != "" {
			fmt.Sscanf(h1[1], "%d", &pa.No)
		}
	}

	rowRe := regexp.MustCompile(`(?m)^\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|`)
	for _, mm := range rowRe.FindAllStringSubmatch(txt, -1) {
		key := strings.TrimSpace(strings.ToLower(mm[1]))
		val := strings.TrimSpace(mm[2])
		switch key {
		case "datum":
			pa.Date = val
		case "status":
			pa.Status = val
		case "beteiligte":
			pa.Beteiligte = val
		case "tags":
			pa.Tags = val
		}
	}

	pa.Kontext = extractSection(txt, "Kontext")
	pa.Entscheidung = extractSection(txt, "Entscheidung")
	pa.Alternativen = extractSection(txt, "Alternativen")
	pa.Konsequenzen = extractSection(txt, "Konsequenzen")

	m.fillFromParsed(pa)
	return nil
}

/* --------- Abschnittsextraktion ohne Lookahead (RE2-kompatibel) ---------- */

func extractSection(txt, heading string) string {
	hdr := regexp.MustCompile(`(?m)^##\s+` + regexp.QuoteMeta(heading) + `\s*$`)
	loc := hdr.FindStringIndex(txt)
	if loc == nil {
		return ""
	}

	start := loc[1]
	if start < len(txt) && txt[start] == '\r' {
		start++
	}
	if start < len(txt) && txt[start] == '\n' {
		start++
	}

	nextHdr := regexp.MustCompile(`(?m)^##\s+`)
	rest := txt[start:]
	next := nextHdr.FindStringIndex(rest)

	end := len(txt)
	if next != nil {
		end = start + next[0]
	}
	return strings.TrimSpace(txt[start:end])
}

func (m *model) fillFromParsed(p parsedADR) {
	if p.Title != "" {
		m.title.SetValue(p.Title)
	}
	if p.Status != "" {
		idx := 0
		for i, s := range statuses {
			if strings.EqualFold(strings.TrimSpace(p.Status), s) {
				idx = i
				break
			}
		}
		m.statusIdx = idx
	} else {
		m.statusIdx = 0
	}
	if p.Kontext != "" {
		m.kontext.SetValue(p.Kontext)
	}
	w := m.kontext.Width()
	m.entscheidung.SetFromMarkdown(p.Entscheidung, 5, w)
	m.konsequenzen.SetFromMarkdown(p.Konsequenzen, 5, w)
	m.alternativen.SetFromMarkdown(p.Alternativen, 5, w)

	if strings.TrimSpace(p.Beteiligte) != "" {
		m.beteiligte.SetValue(p.Beteiligte)
	}
	if strings.TrimSpace(p.Tags) != "" {
		m.tags.SetValue(p.Tags)
	}

	m.editingNo = p.No
}

/* ------------------------------- Draft I/O -------------------------------- */

type draftFile struct {
	EditingPath  string    `json:"editing_path"`
	EditingNo    int       `json:"editing_no"`
	Title        string    `json:"title"`
	StatusIdx    int       `json:"status_idx"`
	Kontext      string    `json:"kontext"`
	Entscheidung []string  `json:"entscheidung"`
	Konsequenzen []string  `json:"konsequenzen"`
	Alternativen []string  `json:"alternativen"`
	Beteiligte   string    `json:"beteiligte"`
	Tags         string    `json:"tags"`
	SavedAt      time.Time `json:"saved_at"`
}

func (m model) draftPath() string {
	_ = os.MkdirAll(autosaveDir, 0o755)
	if m.draftFixedPath != "" { // fester Pfad (neuer ADR oder aus Draft)
		return m.draftFixedPath
	}
	if m.editingPath != "" { // bestehende Datei
		return filepath.Join(autosaveDir, filepath.Base(m.editingPath)+".draft.json")
	}
	// Fallback (sollte selten greifen)
	t := strings.TrimSpace(m.title.Value())
	name := "KEIN-TITEL-" + time.Now().Format("20060102-150405")
	if t != "" {
		name = slugify(t)
	}
	return filepath.Join(autosaveDir, fmt.Sprintf("new-%s.draft.json", name))
}

func (m model) toDraft() draftFile {
	return draftFile{
		EditingPath:  m.editingPath,
		EditingNo:    m.editingNo,
		Title:        m.title.Value(),
		StatusIdx:    m.statusIdx,
		Kontext:      m.kontext.Value(),
		Entscheidung: m.entscheidung.Values(),
		Konsequenzen: m.konsequenzen.Values(),
		Alternativen: m.alternativen.Values(),
		Beteiligte:   m.beteiligte.Value(),
		Tags:         m.tags.Value(),
		SavedAt:      time.Now(),
	}
}

func (m *model) loadDraft(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var d draftFile
	if err := json.Unmarshal(b, &d); err != nil {
		return err
	}
	m.editingPath = d.EditingPath
	m.editingNo = d.EditingNo
	m.title.SetValue(d.Title)
	if d.StatusIdx >= 0 && d.StatusIdx < len(statuses) {
		m.statusIdx = d.StatusIdx
	} else {
		m.statusIdx = 0
	}
	m.kontext.SetValue(d.Kontext)
	w := m.kontext.Width()
	m.entscheidung.SetFromSlice(d.Entscheidung, 5, w)
	m.konsequenzen.SetFromSlice(d.Konsequenzen, 5, w)
	m.alternativen.SetFromSlice(d.Alternativen, 5, w)
	m.beteiligte.SetValue(d.Beteiligte)
	m.tags.SetValue(d.Tags)
	return nil
}

func atomicWrite(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tf, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmp := tf.Name()
	_, werr := tf.Write(data)
	serr := tf.Sync()
	cerr := tf.Close()
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	if serr != nil {
		_ = os.Remove(tmp)
		return serr
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return cerr
	}
	if err := os.Chmod(tmp, perm); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

/* --------------------------- Autosave Tick/Cmd ---------------------------- */

func draftTitlePreview(path string) (title, base string) {
	base = filepath.Base(path)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", base
	}
	var d draftFile
	if json.Unmarshal(b, &d) == nil {
		return strings.TrimSpace(d.Title), base
	}
	return "", base
}

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

/* -------------------------------- Helpers -------------------------------- */

func (m model) Title() string   { return strings.TrimSpace(m.title.Value()) }
func (m model) Status() string  { return statuses[m.statusIdx] }
func (m model) Kontext() string { return strings.TrimSpace(m.kontext.Value()) }

func (m model) Entscheidung() string { return m.entscheidung.Markdown() }
func (m model) Konsequenzen() string { return m.konsequenzen.Markdown() }
func (m model) Alternativen() string { return m.alternativen.Markdown() }
func (m model) Beteiligte() string   { return trimJoin(splitCSV(m.beteiligte.Value())) }
func (m model) Tags() string         { return trimJoin(splitCSV(m.tags.Value())) }

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
func trimJoin(parts []string) string { return strings.Join(parts, ", ") }

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacements := map[string]string{"ä": "ae", "ö": "oe", "ü": "ue", "ß": "ss"}
	for k, v := range replacements {
		s = strings.ReplaceAll(s, k, v)
	}
	re := regexp.MustCompile("[^a-z0-9]+")
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "record"
	}
	return s
}

// neuer Helper: Default-Filename, wenn kein Titel gesetzt ist
func noTitleSlug() string {
	// Großbuchstaben & Bindestrich behalten – bewusst NICHT durch slugify jagen.
	return "KEIN-TITEL-" + time.Now().Format("20060102-150405")
}

func ensureDir(dir string) error { return os.MkdirAll(dir, 0o755) }

func nextADRNumber(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 1, nil
		}
		return 0, err
	}
	re := regexp.MustCompile(`^ADR-([0-9]{4})-`)
	nums := []int{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := re.FindStringSubmatch(e.Name())
		if len(m) == 2 {
			var n int
			fmt.Sscanf(m[1], "%04d", &n)
			nums = append(nums, n)
		}
	}
	if len(nums) == 0 {
		return 1, nil
	}
	sort.Ints(nums)
	return nums[len(nums)-1] + 1, nil
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
		// Neuer ADR
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
	} else {
		// Bestehende Datei: falls jetzt ein Titel vorhanden ist, Zielname anpassen
		if title != "" {
			desired := filepath.Join(dir, fmt.Sprintf("ADR-%04d-%s.md", no, slugify(title)))
			if desired != path {
				path = desired
			}
		}
	}

	date := time.Now().Format("2006-01-02")
	content := buildMarkdown(
		no, m.Title(), date, m.Status(), m.Beteiligte(), m.Tags(),
		m.Kontext(), m.Entscheidung(), m.Alternativen(), m.Konsequenzen(),
	)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}

	// Alte Datei entfernen, wenn der Name geändert wurde
	if oldPath != "" && oldPath != path {
		_ = os.Remove(oldPath)
	}
	return path, nil
}

func buildMarkdownPreview(m model) string {
	no := m.editingNo
	date := time.Now().Format("2006-01-02")
	md := buildMarkdown(no, m.Title(), date, m.Status(), m.Beteiligte(), m.Tags(), m.Kontext(), m.Entscheidung(), m.Alternativen(), m.Konsequenzen())
	lines := strings.Split(md, "\n")
	if len(lines) > 20 {
		lines = lines[:20]
	}
	return strings.Join(lines, "\n")
}

func buildMarkdown(no int, title, date, status, beteiligte, tags, kontext, entscheidung, alternativen, konsequenzen string) string {
	noTitle := title
	if no > 0 {
		noTitle = fmt.Sprintf("ADR %04d: %s", no, title)
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "# %s\n\n", noTitle)
	fmt.Fprintf(b, "| Feld | Wert |\n|------|------|\n")
	fmt.Fprintf(b, "| Datum | %s |\n", date)
	fmt.Fprintf(b, "| Status | %s |\n", status)
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

/* --------------------------------- main ----------------------------------- */

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println(errorStyle.Render("Fehler:"), err)
		os.Exit(1)
	}
}
