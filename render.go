package main

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// Couleurs ANSI ; vides si les couleurs sont désactivées.
type palette struct{ reset, bold, dim, red, yellow, green, cyan, blue string }

func newPalette(enabled bool) palette {
	if !enabled {
		return palette{}
	}
	return palette{"\033[0m", "\033[1m", "\033[2m", "\033[31m", "\033[33m", "\033[32m", "\033[36m", "\033[34m"}
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

func renderTable(w io.Writer, repos []*Repo, p palette, showBranches, showConfig bool) {
	headers := []string{"DÉPÔT", "BRANCHE", "REMOTE", "VS MAIN", "ÉTAT"}
	rows := make([][]cell, 0, len(repos))

	for _, r := range repos {
		branch := c(p, p.cyan, r.Branch)
		if r.Detached {
			branch = c(p, p.yellow, detachedLabel(r))
		}

		remote := c(p, p.dim, "—")
		switch {
		case r.Error != "":
		case r.UpstreamGone:
			remote = c(p, p.yellow, "supprimé")
		case r.Upstream != "":
			col := p.green
			if r.Ahead > 0 || r.Behind > 0 {
				col = p.yellow
			}
			remote = c(p, col, counts(r.Ahead, r.Behind))
		}

		vsMain := c(p, p.dim, "—")
		if r.MainRef != "" && r.Error == "" {
			if isMainBranch(r.Branch, r.MainRef) && r.AheadMain == 0 && r.BehindMain == 0 {
				vsMain = c(p, p.green, "=")
			} else {
				col := p.reset
				if r.BehindMain > 0 {
					col = p.yellow
				}
				vsMain = c(p, col, counts(r.AheadMain, r.BehindMain))
			}
		}

		state := c(p, p.green, "✓ propre")
		if len(r.Flags) > 0 {
			var plain, styled []string
			for _, f := range r.Flags {
				plain = append(plain, f.Label)
				styled = append(styled, p.level(f.Level)+f.Label+p.reset)
			}
			state = cell{strings.Join(plain, ", "), strings.Join(styled, p.dim+", "+p.reset)}
		}

		rows = append(rows, []cell{c(p, p.bold, r.Path), branch, remote, vsMain, state})
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, row := range rows {
		for i, cl := range row {
			if n := utf8.RuneCountInString(cl.plain); n > widths[i] {
				widths[i] = n
			}
		}
	}

	line := func(cells []cell) string {
		var b strings.Builder
		for i, cl := range cells {
			b.WriteString(cl.styled)
			if i < len(cells)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cl.plain)+2))
			}
		}
		return b.String()
	}

	hdr := make([]cell, len(headers))
	for i, h := range headers {
		hdr[i] = c(p, p.dim, h)
	}
	fmt.Fprintln(w, line(hdr))

	indent := strings.Repeat(" ", 4)
	for i, r := range repos {
		fmt.Fprintln(w, line(rows[i]))
		if showConfig && r.Config != nil {
			renderConfig(w, r, p, indent)
		}
		if showBranches && len(r.Branches) > 0 {
			renderBranches(w, r, p, indent)
		}
		if (showConfig || showBranches) && i < len(repos)-1 {
			fmt.Fprintln(w)
		}
	}
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

func renderSummary(w io.Writer, repos []*Repo, total int, p palette, elapsed time.Duration) {
	type stat struct {
		label string
		codes []string
		color string
	}
	stats := []stat{
		{"à pousser", []string{"ahead", "diverged", "no_upstream", "unpushed_branches"}, p.yellow},
		{"à tirer", []string{"behind", "diverged"}, p.yellow},
		{"modifié(s)", []string{"dirty", "untracked", "conflicts"}, p.yellow},
		{"en erreur", []string{"error", "fetch_failed"}, p.red},
	}
	parts := []string{fmt.Sprintf("%s%d %s%s", p.bold, total, plural(total, "dépôt"), p.reset)}
	clean := 0
	for _, r := range repos {
		if !r.NeedsAttention() {
			clean++
		}
	}
	for _, s := range stats {
		n := 0
		for _, r := range repos {
			for _, code := range s.codes {
				if r.has(code) {
					n++
					break
				}
			}
		}
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%s%d %s%s", s.color, n, s.label, p.reset))
		}
	}
	parts = append(parts, fmt.Sprintf("%s%d %s%s", p.green, clean, plural(clean, "propre"), p.reset))
	fmt.Fprintf(w, "\n%s  %s(%s)%s\n", strings.Join(parts, " · "), p.dim, elapsed.Round(time.Millisecond), p.reset)
}

func plural(n int, word string) string {
	if n > 1 {
		return word + "s"
	}
	return word
}
