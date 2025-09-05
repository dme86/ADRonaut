package app

/* --------- Listen-Feld (für Entscheidung/Konsequenzen/Alternativen) ------ */

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

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
