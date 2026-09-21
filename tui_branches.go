package main

// Vue « branches » du mode interactif : la liste des branches d'un dépôt, leur
// lien avec le serveur, et de quoi corriger un lien manquant (touche u).

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// branchLink décrit, en segments colorés, le lien d'une branche avec le serveur.
func branchLink(b Branch) []seg {
	switch {
	case b.UpstreamGone:
		return []seg{{"upstream supprimé", toneWarn}}
	case b.Upstream != "":
		t := toneInfo
		if b.Ahead > 0 || b.Behind > 0 {
			t = toneWarn
		}
		segs := []seg{{b.Upstream + " " + counts(b.Ahead, b.Behind), t}}
		if _, short, ok := strings.Cut(b.Upstream, "/"); ok && short != b.Name {
			segs = append(segs, seg{" ⚠ autre nom", toneWarn})
		}
		return segs
	case b.MatchRemote != "":
		t := toneInfo
		if b.MatchAhead > 0 || b.MatchBehind > 0 {
			t = toneWarn
		}
		return []seg{
			{b.MatchRemote + " " + counts(b.MatchAhead, b.MatchBehind), t},
			{" (sans upstream)", toneInfo},
		}
	default:
		return []seg{{"jamais poussée", toneWarn}}
	}
}

// linkable : la branche peut être reliée à une branche distante de même nom.
func linkable(b Branch) bool {
	if b.MatchRemote == "" {
		return false
	}
	if b.Upstream == "" || b.UpstreamGone {
		return true
	}
	_, short, ok := strings.Cut(b.Upstream, "/")
	return ok && short != b.Name // upstream d'un autre nom : proposer la correction
}

func (m *model) branches() []Branch {
	if m.detail == nil {
		return nil
	}
	return m.detail.repo.Branches
}

func (m *model) keyBranches(msg tea.KeyMsg) tea.Cmd {
	list := m.branches()
	switch msg.String() {
	case "esc", "q", "b", "left", "h":
		m.mode = modeDetail
		return nil
	case "?":
		m.openHelp()
		return nil
	case "up", "k":
		m.branchCursor--
	case "down", "j":
		m.branchCursor++
	case "g", "home":
		m.branchCursor = 0
	case "G", "end":
		m.branchCursor = len(list) - 1
	case "enter":
		// Voir les commits de cette branche dans le détail.
		if m.branchCursor < len(list) {
			m.detailRef = list[m.branchCursor].Name
			m.mode = modeDetail
			return m.loadDetailCmd(m.detailPath)
		}
	case "u":
		if m.branchCursor < len(list) {
			b := list[m.branchCursor]
			if !linkable(b) {
				m.setMsg(true, "%s : aucune branche distante du même nom", b.Name)
				return nil
			}
			return m.linkCmd(b)
		}
	case "d":
		if r := m.repoByPath(m.detailPath); r != nil && m.branchCursor < len(list) {
			m.askClean([]*Repo{r}, list[m.branchCursor].Name)
		}
	case "D":
		if r := m.repoByPath(m.detailPath); r != nil {
			m.askClean([]*Repo{r})
		}
	case "r":
		return m.loadDetailCmd(m.detailPath)
	}
	m.branchCursor = max(0, min(m.branchCursor, len(list)-1))
	return nil
}

// linkCmd relie une branche locale à sa branche distante : git branch -u.
func (m *model) linkCmd(b Branch) tea.Cmd {
	ctx, path := m.ctx, m.detailPath
	name, remote := b.Name, b.MatchRemote
	return func() tea.Msg {
		out, err := runGitCombined(ctx, path, "branch", "-u", remote, name)
		return linkDoneMsg{path: path, branch: name, remote: remote, out: out, err: err}
	}
}

type linkDoneMsg struct {
	path, branch, remote, out string
	err                       error
}

func (m *model) viewBranches() string {
	list := m.branches()
	if len(list) == 0 {
		return ""
	}
	r := m.detail.repo

	title := stTitle.Render("‹ ") + stBold.Render(r.Path) + stDim.Render(fmt.Sprintf("  %d branches", len(list)))
	if m.msg != "" {
		st := stDim
		if m.msgErr {
			st = stRed
		}
		title += "   " + st.Render(m.msg)
	}

	// Largeurs
	w := []int{len("BRANCHE"), len("LIEN AVEC LE SERVEUR"), len("vs MAIN")}
	links := make([][]seg, len(list))
	mains := make([]string, len(list))
	for i, b := range list {
		links[i] = branchLink(b)
		switch {
		case r.MainRef == "" || isMainBranch(b.Name, r.MainRef):
			mains[i] = "—"
		case b.MergedInMain:
			mains[i] = "fusionnée"
		default:
			mains[i] = counts(b.AheadMain, b.BehindMain)
		}
		w[0] = max(w[0], lipgloss.Width(b.Name))
		w[1] = max(w[1], widthOf(links[i], ""))
		w[2] = max(w[2], lipgloss.Width(mains[i]))
	}
	w[0], w[1] = min(w[0], 34), min(w[1], 40)
	sep := stDim.Render(" │ ")

	var b strings.Builder
	b.WriteString(fit(title, m.width) + "\n")
	hdr := "    " + fit(stBold.Render("BRANCHE"), w[0]) + sep + fit(stBold.Render("LIEN AVEC LE SERVEUR"), w[1]) +
		sep + fit(stBold.Render("vs MAIN"), w[2]) + sep + stBold.Render("DERNIER COMMIT")
	b.WriteString(fit(hdr, m.width) + "\n")

	h := max(1, m.height-4)
	start := max(0, min(m.branchCursor-h/2, len(list)-h))
	for i := start; i < min(len(list), start+h); i++ {
		br := list[i]
		cur := "  "
		name := stCyan.Render(br.Name)
		if br.Current {
			name = stGreen.Render("* ") + stCyan.Render(br.Name)
		}
		if i == m.branchCursor {
			cur = stCursor.Render("❯ ")
			name = stCursor.Render(br.Name)
			if br.Current {
				name = stGreen.Render("* ") + stCursor.Render(br.Name)
			}
		}
		mainCell := mains[i]
		switch {
		case mainCell == "fusionnée":
			mainCell = stDim.Render(mainCell)
		case br.BehindMain > 0:
			mainCell = stYellow.Render(mainCell)
		default:
			mainCell = stDim.Render(mainCell)
		}
		age := stDim.Render("il y a " + humanAge(time.Since(br.LastCommit)))
		mark := "  "
		switch {
		case linkable(br):
			mark = stYellow.Render("u ") // corrigeable avec la touche u
		case contains(r.MergedBranches, br.Name):
			mark = stDim.Render("d ") // supprimable avec la touche d
		}
		line := cur + mark + fit(name, w[0]) + sep + fit(renderSegs(links[i], ""), w[1]) +
			sep + fit(mainCell, w[2]) + sep + age
		b.WriteString(fit(line, m.width) + "\n")
	}
	b.WriteString(strings.Repeat("\n", max(0, h-min(len(list), h))))
	b.WriteString(fit(helpLine([][2]string{
		{"u", "relier au serveur"}, {"d", "supprimer (fusionnée)"}, {"⏎", "commits"}, {"r", "rafraîchir"},
		{"esc", "retour"}, {"↑↓", "naviguer"}, {"?", "aide"},
	}), m.width))
	return b.String()
}
