package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type parsedADR struct {
	No           int
	Title        string
	Date         string
	CreatedDate  string
	LastEditedBy string
	LastEditedAt string
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
		case "datum (erstellt)":
			pa.CreatedDate = val
		case "datum": // Abwärtskompatibilität
			pa.Date = val
			if pa.CreatedDate == "" {
				pa.CreatedDate = val
			}
		case "zuletzt editiert von":
			pa.LastEditedBy = val
		case "zuletzt editiert am":
			pa.LastEditedAt = val
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

	if strings.TrimSpace(p.CreatedDate) != "" {
		m.createdDate = p.CreatedDate
	} else if strings.TrimSpace(p.Date) != "" {
		m.createdDate = p.Date
	}
	m.lastEditedBy = strings.TrimSpace(p.LastEditedBy)
	m.lastEditedAt = strings.TrimSpace(p.LastEditedAt)
	m.editingNo = p.No
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
