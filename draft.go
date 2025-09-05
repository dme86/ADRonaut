package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
	CreatedDate  string    `json:"created_date"`
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
		CreatedDate:  m.createdDate,
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
	m.createdDate = d.CreatedDate
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
