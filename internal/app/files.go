package app

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
)

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

func ensureDir(dir string) error { return os.MkdirAll(dir, 0o755) }

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

func noTitleSlug() string {
	// Großbuchstaben & Bindestrich behalten – bewusst NICHT durch slugify jagen.
	return "KEIN-TITEL-" + time.Now().Format("20060102-150405")
}

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
