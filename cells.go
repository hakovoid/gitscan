package main

// Contenu des cellules du tableau, partagé par le rapport (render.go) et la TUI
// (tui.go). Chaque cellule est une suite de segments avec un « ton » ; chaque
// affichage traduit ensuite le ton en couleur.

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

type tone int

const (
	toneNeutral tone = iota
	toneOK           // vert
	toneWarn         // jaune
	toneErr          // rouge
	toneInfo         // gris
	toneAccent       // cyan (branches)
	toneTag          // magenta (tags, commits)
)

type seg struct {
	text string
	tone tone
}

type rowCells struct {
	status Level // gravité la plus haute (Info si tout va bien)
	branch []seg
	server []seg
	main   []seg
	local  []seg
	alerts []seg
}

func plainOf(segs []seg, sep string) string {
	parts := make([]string, len(segs))
	for i, s := range segs {
		parts[i] = s.text
	}
	return strings.Join(parts, sep)
}

func widthOf(segs []seg, sep string) int { return utf8.RuneCountInString(plainOf(segs, sep)) }

func plur(n int, one, many string) string {
	if n > 1 {
		return fmt.Sprintf("%d %s", n, many)
	}
	return fmt.Sprintf("%d %s", n, one)
}

// Signaux déjà représentés par une colonne : on ne les répète pas dans « À VOIR ».
var inColumns = map[string]bool{
	"ahead": true, "behind": true, "diverged": true, "no_upstream": true, "no_remote": true,
	"upstream_gone": true, "behind_main": true, "dirty": true, "mode_only": true,
	"untracked": true, "conflicts": true, "stash": true, "submodules_changed": true,
}

func buildCells(r *Repo) rowCells {
	c := rowCells{status: Info}
	for _, f := range r.Flags {
		c.status = max(c.status, f.Level)
	}
	if r.Branch == "" && r.Error == "" { // pas encore analysé (TUI)
		return c
	}
	if r.Error != "" {
		c.alerts = []seg{{"erreur : " + r.Error, toneErr}}
		return c
	}

	// BRANCHE
	switch {
	case !r.Detached:
		c.branch = []seg{{r.Branch, toneAccent}}
	default:
		// Toujours le commit : c'est l'information sûre. Les tags et la distance
		// au dernier tag vont dans « À VOIR ».
		sha := r.HeadSHA
		if sha == "" {
			sha = "?"
		}
		t := toneTag
		if !r.Submodule && len(r.HeadTags) == 0 {
			t = toneWarn
		}
		c.branch = []seg{{"@" + sha, t}}
	}

	// SERVEUR : la branche comparée à sa branche distante
	switch {
	case r.FetchError != "":
		c.server = []seg{{"fetch ✗", toneErr}}
	case r.Detached:
		c.server = []seg{{"—", toneInfo}}
	case r.UpstreamGone:
		c.server = []seg{{"supprimée", toneWarn}}
	case r.Upstream == "" && r.NoRemote:
		c.server = []seg{{"aucun", toneInfo}}
	case r.Upstream == "":
		c.server = []seg{{"jamais poussée", toneWarn}}
	case r.Ahead > 0 && r.Behind > 0:
		c.server = []seg{{fmt.Sprintf("↑%d ↓%d", r.Ahead, r.Behind), toneErr}}
	case r.Ahead > 0:
		c.server = []seg{{fmt.Sprintf("↑%d", r.Ahead), toneWarn}}
	case r.Behind > 0:
		c.server = []seg{{fmt.Sprintf("↓%d", r.Behind), toneWarn}}
	default:
		c.server = []seg{{"=", toneOK}}
	}

	// MAIN
	switch {
	case r.MainRef == "":
		c.main = []seg{{"—", toneInfo}}
	case r.AheadMain == 0 && r.BehindMain == 0:
		c.main = []seg{{"=", toneOK}}
	default:
		if r.AheadMain > 0 {
			c.main = append(c.main, seg{fmt.Sprintf("↑%d", r.AheadMain), toneNeutral})
		}
		if r.BehindMain > 0 {
			c.main = append(c.main, seg{fmt.Sprintf("↓%d", r.BehindMain), toneWarn})
		}
	}

	// LOCAL : ce qui n'est pas commité
	if r.Conflicts > 0 {
		c.local = append(c.local, seg{plur(r.Conflicts, "conflit", "conflits"), toneErr})
	}
	switch {
	case r.Changed > 0 && r.ModeOnly == r.Changed:
		c.local = append(c.local, seg{plur(r.ModeOnly, "droit modifié", "droits modifiés"), toneInfo})
	case r.ModeOnly > 0:
		c.local = append(c.local, seg{plur(r.Changed, "modifié", "modifiés"), toneWarn},
			seg{fmt.Sprintf("dont %d droits", r.ModeOnly), toneInfo})
	case r.Changed > 0:
		c.local = append(c.local, seg{plur(r.Changed, "modifié", "modifiés"), toneWarn})
	}
	if r.Untracked > 0 {
		c.local = append(c.local, seg{plur(r.Untracked, "nouveau", "nouveaux"), toneWarn})
	}
	if r.SubmodulesChanged > 0 {
		c.local = append(c.local, seg{plur(r.SubmodulesChanged, "sous-module", "sous-modules"), toneWarn})
	}
	if r.Stashes > 0 {
		c.local = append(c.local, seg{fmt.Sprintf("stash %d", r.Stashes), toneInfo})
	}
	if len(c.local) == 0 {
		c.local = []seg{{"propre", toneOK}}
	}

	// À VOIR : le reste des signaux
	for _, f := range r.Flags {
		if inColumns[f.Code] {
			continue
		}
		t := toneInfo
		switch f.Level {
		case Error:
			t = toneErr
		case Warn:
			t = toneWarn
		}
		c.alerts = append(c.alerts, seg{f.Label, t})
	}
	if r.HeadAfterTag != "" {
		c.alerts = append(c.alerts, seg{r.HeadAfterTag, toneInfo})
	}
	if r.Submodule && r.SubInSync {
		c.alerts = append(c.alerts, seg{"sous-module ✓", toneInfo})
	}
	return c
}

// wrapSegs répartit des segments sur plusieurs lignes de largeur max width
// (0 = pas de limite).
func wrapSegs(segs []seg, sep string, width int) [][]seg {
	if width <= 0 || widthOf(segs, sep) <= width {
		return [][]seg{segs}
	}
	// Un segment plus long que la colonne est découpé en morceaux de mots.
	var pieces []seg
	for _, s := range segs {
		if utf8.RuneCountInString(s.text) <= width {
			pieces = append(pieces, s)
			continue
		}
		line := ""
		for _, word := range strings.Fields(s.text) {
			if line != "" && utf8.RuneCountInString(line+" "+word) > width {
				pieces = append(pieces, seg{line, s.tone})
				line = ""
			}
			if line != "" {
				line += " "
			}
			line += truncRunes(word, width)
		}
		pieces = append(pieces, seg{line, s.tone})
	}
	var lines [][]seg
	var cur []seg
	for _, s := range pieces {
		if len(cur) > 0 && widthOf(append(append([]seg{}, cur...), s), sep) > width {
			lines = append(lines, cur)
			cur = nil
		}
		cur = append(cur, s)
	}
	return append(lines, cur)
}

// ---------- Arborescence ----------

type treeRow struct {
	repo    *Repo
	name    string // nom affiché (relatif au parent)
	prefix  string // « ├─ », « └─ », « │  »…
	newGrp  bool   // un séparateur précède cette ligne
	isChild bool
}

// buildTree range les dépôts imbriqués (sous-modules, -nested) sous leur parent.
func buildTree(repos []*Repo, rootName string) []treeRow {
	paths := map[string]*Repo{}
	for _, r := range repos {
		paths[r.Path] = r
	}
	parentOf := func(r *Repo) *Repo {
		if r.Path == "." {
			return nil
		}
		for p := filepath.Dir(r.Path); ; p = filepath.Dir(p) {
			if pr, ok := paths[p]; ok && pr != r {
				return pr
			}
			if p == "." || p == "/" {
				return nil
			}
		}
	}
	children := map[*Repo][]*Repo{}
	var roots []*Repo
	for _, r := range repos {
		if p := parentOf(r); p != nil {
			children[p] = append(children[p], r)
		} else {
			roots = append(roots, r)
		}
	}
	byPath := func(s []*Repo) { sort.SliceStable(s, func(i, j int) bool { return s[i].Path < s[j].Path }) }
	byPath(roots)

	var rows []treeRow
	var walk func(r *Repo, parent *Repo, indent string, last bool, depth int)
	walk = func(r *Repo, parent *Repo, indent string, last bool, depth int) {
		name := r.Path
		if parent != nil {
			name, _ = filepath.Rel(parent.Path, r.Path)
		}
		if name == "." {
			name = rootName
		}
		prefix := ""
		childIndent := ""
		if depth > 0 {
			if last {
				prefix, childIndent = indent+"└─ ", indent+"   "
			} else {
				prefix, childIndent = indent+"├─ ", indent+"│  "
			}
		}
		rows = append(rows, treeRow{repo: r, name: name, prefix: prefix, isChild: depth > 0})
		kids := children[r]
		byPath(kids)
		for i, k := range kids {
			walk(k, r, childIndent, i == len(kids)-1, depth+1)
		}
	}
	prevGroup := ""
	for i, r := range roots {
		group := strings.SplitN(r.Path, string(filepath.Separator), 2)[0]
		start := len(rows)
		walk(r, nil, "", true, 0)
		hasKids := len(children[r]) > 0
		if i > 0 && (group != prevGroup || hasKids || (start > 0 && rows[start-1].isChild)) {
			rows[start].newGrp = true
		}
		prevGroup = group
	}
	return rows
}
