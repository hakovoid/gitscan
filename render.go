package main

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// Couleurs ANSI ; vides si les couleurs sont désactivées.
type palette struct{ reset, bold, dim, red, yellow, green, cyan, magenta string }

func newPalette(enabled bool) palette {
	if !enabled {
		return palette{}
	}
	return palette{"\033[0m", "\033[1m", "\033[2m", "\033[31m", "\033[33m", "\033[32m", "\033[36m", "\033[35m"}
}

func (p palette) level(l Level) string {
	switch l {
	case Error:
		return p.red
	case Warn:
		return p.yellow
	default:
		return p.dim
	}
}

// cell : un texte brut (pour calculer la largeur) et sa version colorée.
type cell struct{ plain, styled string }

func c(p palette, color, s string) cell { return cell{s, color + s + p.reset} }

func counts(ahead, behind int) string {
	if ahead == 0 && behind == 0 {
		return "="
	}
	var parts []string
	if ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", ahead))
	}
	if behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", behind))
	}
	return strings.Join(parts, " ")
}

func (p palette) tone(t tone) string {
	switch t {
	case toneOK:
		return p.green
	case toneWarn:
		return p.yellow
	case toneErr:
		return p.red
	case toneInfo:
		return p.dim
	case toneAccent:
		return p.cyan
	case toneTag:
		return p.magenta
	}
	return ""
}

// segsANSI colore des segments et les complète par des espaces jusqu'à width.
func (p palette) segs(segs []seg, sep string, width int) string {
	var b strings.Builder
	for i, s := range segs {
		if i > 0 {
			b.WriteString(p.dim + sep + p.reset)
		}
		if col := p.tone(s.tone); col != "" {
			b.WriteString(col + s.text + p.reset)
		} else {
			b.WriteString(s.text)
		}
	}
	if pad := width - widthOf(segs, sep); pad > 0 {
		b.WriteString(strings.Repeat(" ", pad))
	}
	return b.String()
}

func truncRunes(s string, w int) string {
	if w > 0 && utf8.RuneCountInString(s) > w {
		return string([]rune(s)[:max(1, w-1)]) + "…"
	}
	return s
}

func statusIcon(p palette, l Level) string {
	switch l {
	case Error:
		return p.red + "✗" + p.reset
	case Warn:
		return p.yellow + "●" + p.reset
	}
	return p.green + "✓" + p.reset
}

// renderTable dessine le rapport : une grille avec séparateurs, une icône d'état
// par dépôt, les dépôts imbriqués rangés sous leur parent.
// termWidth = 0 : pas de limite (sortie redirigée).
func renderTable(w io.Writer, repos []*Repo, rootName string, p palette, showBranches, showConfig bool, termWidth int) {
	const sep = " │ "
	const listSep = " · "
	headers := []string{"DÉPÔT", "BRANCHE", "SERVEUR", "MAIN", "LOCAL", "À VOIR"}
	rows := buildTree(repos, rootName)
	cells := make([]rowCells, len(rows))

	// Largeurs : le contenu, plafonné pour laisser de la place à « À VOIR ».
	wid := make([]int, len(headers))
	for i, h := range headers {
		wid[i] = utf8.RuneCountInString(h)
	}
	for i, tr := range rows {
		cells[i] = buildCells(tr.repo)
		c := cells[i]
		wid[0] = max(wid[0], utf8.RuneCountInString(tr.prefix+tr.name))
		wid[1] = max(wid[1], widthOf(c.branch, ""))
		wid[2] = max(wid[2], widthOf(c.server, ""))
		wid[3] = max(wid[3], widthOf(c.main, " "))
		wid[4] = max(wid[4], widthOf(c.local, listSep))
		wid[5] = max(wid[5], widthOf(c.alerts, listSep))
	}
	wid[0], wid[1], wid[4] = min(wid[0], 40), min(wid[1], 26), min(wid[4], 36)
	natural := append([]int{}, wid...)
	lead := 3        // « ✓ » + espaces
	stacked := false // « À VOIR » sous la ligne quand la largeur manque
	if termWidth > 0 {
		used := lead + wid[0] + wid[1] + wid[2] + wid[3] + wid[4] + 5*utf8.RuneCountInString(sep)
		// Trop étroit : on réduit le chemin, la branche puis LOCAL (qui passe à la ligne)
		// pour garder au moins 30 colonnes à « À VOIR ».
		const minAlerts = 30
		for _, k := range []struct{ col, floor int }{{0, 16}, {1, 12}, {4, 18}} {
			if rest := termWidth - used - 1; rest < minAlerts {
				d := min(minAlerts-rest, max(0, wid[k.col]-k.floor))
				wid[k.col] -= d
				used -= d
			}
		}
		stacked = termWidth-used-1 < minAlerts
		if stacked {
			// Sans colonne « À VOIR », les autres colonnes reprennent leur largeur,
			// puis on réduit LOCAL, le chemin et la branche seulement si nécessaire.
			copy(wid, natural)
			used = lead + wid[0] + wid[1] + wid[2] + wid[3] + wid[4] + 4*utf8.RuneCountInString(sep)
			for _, k := range []struct{ col, floor int }{{4, 14}, {0, 14}, {1, 10}} {
				if over := used - (termWidth - 1); over > 0 {
					d := min(over, max(0, wid[k.col]-k.floor))
					wid[k.col] -= d
					used -= d
				}
			}
		}
		wid[5] = max(12, min(wid[5], termWidth-used-1))
	}
	cols := len(headers)
	if stacked {
		cols = 5
	}

	vsep := p.dim + sep + p.reset
	rule := func() string {
		parts := make([]string, cols)
		for i, n := range wid[:cols] {
			parts[i] = strings.Repeat("─", n)
		}
		return p.dim + strings.Repeat("─", lead) + strings.Join(parts, "─┼─") + p.reset
	}

	hdr := make([]string, cols)
	for i, h := range headers[:cols] {
		hdr[i] = p.bold + h + p.reset + strings.Repeat(" ", wid[i]-utf8.RuneCountInString(h))
	}
	fmt.Fprintln(w, strings.Repeat(" ", lead)+strings.Join(hdr, vsep))
	fmt.Fprintln(w, rule())

	for i, tr := range rows {
		c := cells[i]
		if tr.newGrp {
			fmt.Fprintln(w, rule())
		}
		name := truncRunes(tr.prefix+tr.name, wid[0])
		pfx := utf8.RuneCountInString(tr.prefix)
		nameCol := p.dim + string([]rune(name)[:min(pfx, utf8.RuneCountInString(name))]) + p.reset +
			p.bold + string([]rune(name)[min(pfx, utf8.RuneCountInString(name)):]) + p.reset +
			strings.Repeat(" ", wid[0]-utf8.RuneCountInString(name))

		branch := c.branch
		if len(branch) == 1 {
			branch = []seg{{truncRunes(branch[0].text, wid[1]), branch[0].tone}}
		}
		locals := wrapSegs(c.local, listSep, wid[4])
		alerts := wrapSegs(c.alerts, listSep, wid[5])
		if stacked {
			alerts = nil
		}
		n := max(len(locals), len(alerts))
		for l := 0; l < n; l++ {
			col := func(k int, segs []seg, s string) string {
				if l > 0 {
					return strings.Repeat(" ", wid[k])
				}
				return p.segs(segs, s, wid[k])
			}
			lineOf := func(ls [][]seg, k int) string {
				if l < len(ls) {
					return p.segs(ls[l], listSep, wid[k])
				}
				return strings.Repeat(" ", wid[k])
			}
			icon, nc := " ", strings.Repeat(" ", wid[0])
			if l == 0 {
				icon, nc = statusIcon(p, c.status), nameCol
			} else if len(tr.prefix) > 0 && strings.Contains(tr.prefix, "├") {
				// continuité du trait de l'arbre sur les lignes suivantes
				nc = p.dim + strings.Replace(strings.Replace(tr.prefix, "├─ ", "│  ", 1), "└─ ", "   ", 1) + p.reset +
					strings.Repeat(" ", wid[0]-utf8.RuneCountInString(tr.prefix))
			}
			cells := []string{nc, col(1, branch, ""), col(2, c.server, ""), col(3, c.main, " "), lineOf(locals, 4)}
			if !stacked {
				cells = append(cells, lineOf(alerts, 5))
			}
			line := " " + icon + " " + strings.Join(cells, vsep)
			fmt.Fprintln(w, strings.TrimRight(line, " "))
		}

		indent := strings.Repeat(" ", lead+2+utf8.RuneCountInString(tr.prefix))
		if stacked && len(c.alerts) > 0 {
			avail := termWidth - utf8.RuneCountInString(indent) - 2
			for _, l := range wrapSegs(c.alerts, listSep, avail) {
				fmt.Fprintln(w, indent+p.dim+"↳ "+p.reset+p.segs(l, listSep, 0))
			}
		}
		if showConfig && tr.repo.Config != nil {
			renderConfig(w, tr.repo, p, indent)
		}
		if showBranches && len(tr.repo.Branches) > 0 {
			renderBranches(w, tr.repo, p, indent)
		}
	}
	fmt.Fprintln(w, rule())
}

func renderBranches(w io.Writer, r *Repo, p palette, indent string) {
	nameW := 0
	for _, b := range r.Branches {
		nameW = max(nameW, utf8.RuneCountInString(b.Name))
	}
	for _, b := range r.Branches {
		mark := " "
		if b.Current {
			mark = p.green + "*" + p.reset
		}
		var up string
		switch {
		case b.UpstreamGone:
			up = p.yellow + "upstream supprimé" + p.reset
		case b.Upstream == "" && b.AheadMain > 0 && !isMainBranch(b.Name, r.MainRef):
			up = p.yellow + "jamais poussée" + p.reset
		case b.Upstream == "":
			up = p.dim + "pas d'upstream" + p.reset
		default:
			col := p.green
			if b.Ahead > 0 || b.Behind > 0 {
				col = p.yellow
			}
			up = fmt.Sprintf("%s %s%s%s", b.Upstream, col, counts(b.Ahead, b.Behind), p.reset)
		}
		var mainInfo string
		if r.MainRef != "" && !isMainBranch(b.Name, r.MainRef) {
			if b.MergedInMain {
				mainInfo = p.dim + "fusionnée dans main" + p.reset
				if b.BehindMain > 0 {
					mainInfo += p.dim + fmt.Sprintf(" (↓%d)", b.BehindMain) + p.reset
				}
			} else {
				mainInfo = "main " + counts(b.AheadMain, b.BehindMain)
			}
		}
		age := p.dim + "il y a " + humanAge(time.Since(b.LastCommit)) + p.reset
		fmt.Fprintf(w, "%s%s %s%s%s  %s", indent, mark, p.cyan, b.Name, p.reset,
			strings.Repeat(" ", nameW-utf8.RuneCountInString(b.Name)))
		fmt.Fprintf(w, "%s", up)
		if mainInfo != "" {
			fmt.Fprintf(w, "  %s·%s %s", p.dim, p.reset, mainInfo)
		}
		fmt.Fprintf(w, "  %s·%s %s\n", p.dim, p.reset, age)
	}
}

func renderConfig(w io.Writer, r *Repo, p palette, indent string) {
	cfg := r.Config
	kv := func(k, v string) {
		fmt.Fprintf(w, "%s%s%-10s%s %s\n", indent, p.dim, k, p.reset, v)
	}
	if len(cfg.Remotes) == 0 {
		kv("remotes", p.yellow+"aucun"+p.reset)
	}
	for i, rm := range cfg.Remotes {
		k := ""
		if i == 0 {
			k = "remotes"
		}
		kv(k, rm.Name+"  "+p.dim+rm.URL+p.reset)
	}
	user := strings.TrimSpace(fmt.Sprintf("%s <%s>", cfg.UserName, cfg.UserEmail))
	if cfg.UserName == "" && cfg.UserEmail == "" {
		user = p.yellow + "non défini" + p.reset
	}
	kv("auteur", user)
	if r.MainRef != "" {
		kv("main", r.MainRef)
	}
	if len(cfg.Hooks) > 0 {
		kv("hooks", strings.Join(cfg.Hooks, ", "))
	}
	if cfg.HooksPath != "" {
		kv("hooksPath", cfg.HooksPath)
	}
	if cfg.Signing {
		kv("signature", "commits signés")
	}
	if r.LastFetch != nil {
		kv("fetch", "il y a "+humanAge(time.Since(*r.LastFetch)))
	}
	if r.FetchError != "" {
		kv("erreur", p.red+r.FetchError+p.reset)
	}
}

func renderSummary(w io.Writer, repos []*Repo, p palette, elapsed time.Duration) {
	var bad, todo, ok int
	for _, r := range repos {
		switch buildCells(r).status {
		case Error:
			bad++
		case Warn:
			todo++
		default:
			ok++
		}
	}
	count := func(codes ...string) int {
		n := 0
		for _, r := range repos {
			for _, code := range codes {
				if r.has(code) {
					n++
					break
				}
			}
		}
		return n
	}

	line1 := []string{p.bold + fmt.Sprintf("%d %s", len(repos), plural(len(repos), "dépôt")) + p.reset}
	if bad > 0 {
		line1 = append(line1, p.red+fmt.Sprintf("✗ %d à risque", bad)+p.reset)
	}
	if todo > 0 {
		line1 = append(line1, p.yellow+fmt.Sprintf("● %d à traiter", todo)+p.reset)
	}
	line1 = append(line1, p.green+fmt.Sprintf("✓ %d en ordre", ok)+p.reset)
	fmt.Fprintf(w, "   %s   %s(%s)%s\n", strings.Join(line1, "   "), p.dim, elapsed.Round(time.Millisecond), p.reset)

	var line2 []string
	add := func(n int, label string) {
		if n > 0 {
			line2 = append(line2, fmt.Sprintf("%s %s%d%s", label, p.bold, n, p.reset))
		}
	}
	add(count("ahead", "diverged", "no_upstream", "unpushed_branches"), "à pousser")
	add(count("behind", "diverged"), "à tirer")
	add(count("dirty", "untracked", "conflicts"), "modifiés")
	add(count("submodule_drift"), "sous-modules décalés")
	if len(line2) > 0 {
		fmt.Fprintf(w, "   %s\n", strings.Join(line2, p.dim+" · "+p.reset))
	}
	fmt.Fprintf(w, "   %s↑ à pousser · ↓ à tirer · = à jour · @ commit ou tag (pas de branche)%s\n", p.dim, p.reset)
}

func plural(n int, word string) string {
	if n > 1 {
		return word + "s"
	}
	return word
}
