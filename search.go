package main

import (
	"regexp"
	"sort"
	"strings"
)

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
