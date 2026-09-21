package main

// Ce qui a changé depuis le dernier scan : gitscan garde un instantané de
// chaque scan (dans ~/.cache/gitscan) et le compare au suivant.

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type snapFlag struct {
	Label  string `json:"label"`
	Level  Level  `json:"level"`
	Normal bool   `json:"normal,omitempty"`
}

type snapRepo struct {
	Branch string              `json:"branch"`
	Head   string              `json:"head"`
	Tags   []string            `json:"tags,omitempty"`
	Status Level               `json:"status"`
	Flags  map[string]snapFlag `json:"flags"`
}

type snapshot struct {
	Time  time.Time           `json:"time"`
	Root  string              `json:"root"`
	Repos map[string]snapRepo `json:"repos"` // clé : chemin relatif au dossier scanné
}

// snapshotFile : un fichier par dossier scanné et par jeu d'options qui
// changent la liste des dépôts (-nested, -depth, -exclude).
func snapshotFile(root string, nested bool, depth int, excludes map[string]bool) string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	var ex []string
	for e := range excludes {
		ex = append(ex, e)
	}
	sort.Strings(ex)
	sum := sha1.Sum([]byte(strings.Join([]string{root, strconv.FormatBool(nested), strconv.Itoa(depth), strings.Join(ex, ",")}, "\x00")))
	name := filepath.Base(root)
	if name == "/" || name == "." {
		name = "racine"
	}
	return filepath.Join(dir, "gitscan", name+"-"+hex.EncodeToString(sum[:])[:12]+".json")
}

func takeSnapshot(root string, repos []*Repo) *snapshot {
	s := &snapshot{Time: time.Now(), Root: root, Repos: map[string]snapRepo{}}
	for _, r := range repos {
		if r.Branch == "" && r.Error == "" {
			continue // pas encore analysé (TUI)
		}
		sr := snapRepo{Branch: r.Branch, Head: r.Head, Tags: r.HeadTags, Status: buildCells(r).status, Flags: map[string]snapFlag{}}
		for _, f := range r.Flags {
			sr.Flags[f.Code] = snapFlag{Label: f.Label, Level: f.Level, Normal: f.Normal != ""}
		}
		s.Repos[r.Path] = sr
	}
	return s
}

// loadSnapshot renvoie nil, sans erreur, s'il n'y a pas encore d'instantané.
func loadSnapshot(file string) (*snapshot, error) {
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("instantané illisible (%s) : %v", file, err)
	}
	return &s, nil
}

func saveSnapshot(file string, s *snapshot) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// change : l'évolution d'un dépôt entre deux scans.
type change struct {
	Path          string
	Kind          string // "new", "gone", "changed"
	Before, After Level
	Items         []seg
}

// diffSnapshots compare deux scans. Ne sont retenus que les changements qui
// comptent : branche, commit courant, signaux apparus, réglés ou dont la
// valeur a bougé. Les signaux déclarés normaux sont ignorés.
func diffSnapshots(old, cur *snapshot) []change {
	if old == nil || cur == nil {
		return nil
	}
	var out []change
	for path, n := range cur.Repos {
		o, ok := old.Repos[path]
		if !ok {
			out = append(out, change{Path: path, Kind: "new", After: n.Status, Items: []seg{{"nouveau dépôt", toneAccent}}})
			continue
		}
		c := change{Path: path, Kind: "changed", Before: o.Status, After: n.Status}
		switch {
		case o.Branch != n.Branch:
			c.Items = append(c.Items, seg{fmt.Sprintf("branche %s → %s", branchName(o), branchName(n)), toneAccent})
		case o.Head != n.Head && o.Head != "" && n.Head != "":
			from, to := o.Head, n.Head
			if len(o.Tags) > 0 {
				from = "tag " + o.Tags[0]
			}
			if len(n.Tags) > 0 {
				to = "tag " + n.Tags[0]
			}
			c.Items = append(c.Items, seg{fmt.Sprintf("HEAD %s → %s", from, to), toneTag})
		}
		codes := make([]string, 0, len(n.Flags)+len(o.Flags))
		for code := range n.Flags {
			codes = append(codes, code)
		}
		for code := range o.Flags {
			if _, dup := n.Flags[code]; !dup {
				codes = append(codes, code)
			}
		}
		sort.Slice(codes, func(i, j int) bool { return flagOrder(codes[i]) < flagOrder(codes[j]) })
		// L'état vis-à-vis du serveur est un seul signal parmi plusieurs
		// exclusifs : « à pousser → divergé » est une transition, pas un
		// problème réglé plus un nouveau.
		oldS, newS := serverState(o), serverState(n)
		if oldS != "" && newS != "" && oldS != newS && !o.Flags[oldS].Normal && !n.Flags[newS].Normal {
			nf := n.Flags[newS]
			c.Items = append(c.Items, seg{o.Flags[oldS].Label + " → " + nf.Label, levelTone(nf.Level)})
		}
		for _, code := range codes {
			if oldS != "" && newS != "" && oldS != newS && (code == oldS || code == newS) {
				continue
			}
			of, had := o.Flags[code]
			nf, has := n.Flags[code]
			switch {
			case has && nf.Normal, had && !has && of.Normal:
				// signal déclaré normal : pas un changement à signaler
			case has && !had:
				c.Items = append(c.Items, seg{"+ " + nf.Label, levelTone(nf.Level)})
			case had && !has:
				c.Items = append(c.Items, seg{"réglé : " + of.Label, toneOK})
			case of.Label != nf.Label && code != "stale_fetch":
				c.Items = append(c.Items, seg{of.Label + " → " + nf.Label, levelTone(nf.Level)})
			}
		}
		if len(c.Items) > 0 {
			out = append(out, c)
		}
	}
	for path, o := range old.Repos {
		if _, ok := cur.Repos[path]; !ok {
			out = append(out, change{Path: path, Kind: "gone", Before: o.Status, Items: []seg{{"disparu (supprimé, déplacé ou exclu)", toneInfo}}})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

var serverCodes = []string{"ahead", "behind", "diverged", "no_upstream", "unlinked", "upstream_gone", "no_remote"}

func serverState(s snapRepo) string {
	for _, c := range serverCodes {
		if _, ok := s.Flags[c]; ok {
			return c
		}
	}
	return ""
}

func branchName(s snapRepo) string {
	if s.Branch == "(detached)" {
		return "@" + s.Head
	}
	return s.Branch
}

func levelTone(l Level) tone {
	switch l {
	case Error:
		return toneErr
	case Warn:
		return toneWarn
	}
	return toneInfo
}

// flagOrder : l'ordre d'affichage des signaux, le plus grave d'abord.
func flagOrder(code string) int {
	order := []string{"error", "operation", "orphan_commits", "conflicts", "dirty", "untracked",
		"diverged", "ahead", "behind", "no_upstream", "upstream_gone", "unpushed_branches"}
	for i, c := range order {
		if c == code {
			return i
		}
	}
	return len(order)
}

// renderChanges affiche les changements depuis le scan précédent.
func renderChanges(w io.Writer, changes []change, since time.Time, p palette, width int) {
	age := sinceText(since)
	if len(changes) == 0 {
		fmt.Fprintf(w, "   %sAucun changement depuis le dernier scan (%s).%s\n", p.dim, age, p.reset)
		return
	}
	fmt.Fprintf(w, "   %sDepuis le dernier scan%s %s(%s)%s\n", p.bold, p.reset, p.dim, age, p.reset)
	pathW := 0
	for _, c := range changes {
		pathW = max(pathW, utf8.RuneCountInString(c.Path))
	}
	pathW = min(pathW, 32)
	const lead = 3 + 5 + 2 // marge, « ● → ✓ », espace
	avail := 0
	if width > 0 {
		avail = max(20, width-lead-pathW-2)
	}
	for _, c := range changes {
		var mark string
		switch {
		case c.Kind == "new":
			mark = p.cyan + "+" + p.reset + "    "
		case c.Kind == "gone":
			mark = p.dim + "−" + p.reset + "    "
		case c.Before != c.After:
			mark = statusIcon(p, c.Before) + p.dim + " → " + p.reset + statusIcon(p, c.After)
		default:
			mark = "     "
		}
		name := truncRunes(c.Path, pathW)
		for i, line := range wrapSegs(c.Items, " · ", avail) {
			if i == 0 {
				fmt.Fprintf(w, "   %s  %s%s%s%s  %s\n", mark, p.bold, name, p.reset,
					strings.Repeat(" ", pathW-utf8.RuneCountInString(name)), p.segs(line, " · ", 0))
			} else {
				fmt.Fprintf(w, "%s%s\n", strings.Repeat(" ", lead+pathW+2), p.segs(line, " · ", 0))
			}
		}
	}
}

func sinceText(t time.Time) string {
	if time.Since(t) < time.Minute {
		return "à l'instant"
	}
	return "il y a " + humanAge(time.Since(t))
}
