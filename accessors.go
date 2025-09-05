// accessors.go
package main

import "strings"

func (m model) Title() string   { return strings.TrimSpace(m.title.Value()) }
func (m model) Status() string  { return statuses[m.statusIdx] }
func (m model) Kontext() string { return strings.TrimSpace(m.kontext.Value()) }

func (m model) Entscheidung() string { return m.entscheidung.Markdown() }
func (m model) Konsequenzen() string { return m.konsequenzen.Markdown() }
func (m model) Alternativen() string { return m.alternativen.Markdown() }

func (m model) Beteiligte() string { return trimJoin(splitCSV(m.beteiligte.Value())) }
func (m model) Tags() string       { return trimJoin(splitCSV(m.tags.Value())) }

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
