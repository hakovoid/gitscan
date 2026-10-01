package main

// Vue « branches » du mode interactif : la liste des branches d'un dépôt, leur
// lien avec le serveur, de quoi corriger un lien manquant (touche u) et cocher
// les branches fusionnées à supprimer (espace, puis d).

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

// keepReason : pourquoi gitscan ne propose pas de supprimer cette branche ;
// vide si elle est supprimable sans rien perdre.
func keepReason(r *Repo, b Branch) string {
	switch {
	case contains(r.MergedBranches, b.Name):
		return ""
	case b.Current:
		return "branche courante"
	case r.MainRef == "" || isMainBranch(b.Name, r.MainRef):
		return "branche principale"
	case protectedBranches[b.Name]:
		return "branche protégée"
	case b.AheadMain > 0:
		return plur(b.AheadMain, "commit absent", "commits absents") + " de main"
	case b.MergedInMain:
		return "ouverte dans un worktree ou utilisée par un rebase"
	}
	return "non vérifiée"
}

// checkedBranches : les branches cochées encore supprimables, dans l'ordre de
// la liste (une branche supprimée ou qui a bougé sort d'elle-même).
func (m *model) checkedBranches() []string {
	if m.detail == nil {
		return nil
	}
	var names []string
	for _, b := range m.branches() {
		if m.branchSel[b.Name] && keepReason(m.detail.repo, b) == "" {
			names = append(names, b.Name)
		}
	}
	return names
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
	case "esc":
		if len(m.checkedBranches()) > 0 {
			m.branchSel = map[string]bool{}
			return nil
		}
		m.mode = modeDetail
		return nil
	case "q", "b", "left", "h":
		m.mode = modeDetail
		return nil
	case " ", "x":
		if m.branchCursor < len(list) {
			b := list[m.branchCursor]
			if why := keepReason(m.detail.repo, b); why != "" {
				m.setMsg(true, "%s : non supprimable (%s)", b.Name, why)
				return nil
			}
			if m.branchSel[b.Name] {
				delete(m.branchSel, b.Name)
			} else {
				m.branchSel[b.Name] = true
			}
			m.msg = ""
			m.branchCursor++
		}
	case "a":
		// Tout cocher, ou tout décocher si tout l'est déjà.
		merged := m.detail.repo.MergedBranches
		all := len(merged) > 0 && len(m.checkedBranches()) == len(merged)
		m.branchSel = map[string]bool{}
		if !all {
			for _, n := range merged {
				m.branchSel[n] = true
			}
		}
		if len(merged) == 0 {
			m.setMsg(false, "Aucune branche fusionnée à cocher.")
		}
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
		// Les branches cochées, ou à défaut celle sous le curseur.
		r := m.repoByPath(m.detailPath)
		if r == nil {
			return nil
		}
		if names := m.checkedBranches(); len(names) > 0 {
			m.askClean([]*Repo{r}, names...)
		} else if m.branchCursor < len(list) {
			b := list[m.branchCursor]
			if why := keepReason(m.detail.repo, b); why != "" {
				m.setMsg(true, "%s : non supprimable (%s)", b.Name, why)
				return nil
			}
			m.askClean([]*Repo{r}, b.Name)
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
	checked := m.checkedBranches()
	if n := len(checked); n > 0 {
		title += "   " + stMag.Render(plur(n, "cochée", "cochées"))
	}
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
		nameW := lipgloss.Width(b.Name)
		if b.Current {
			nameW += 2 // « * » devant la branche courante
		}
		w[0] = max(w[0], nameW)
		w[1] = max(w[1], widthOf(links[i], ""))
		w[2] = max(w[2], lipgloss.Width(mains[i]))
	}
	w[0], w[1] = min(w[0], 34), min(w[1], 40)
	sep := stDim.Render(" │ ")

	var b strings.Builder
	b.WriteString(fit(title, m.width) + "\n")
	hdr := "      " + fit(stBold.Render("BRANCHE"), w[0]) + sep + fit(stBold.Render("LIEN AVEC LE SERVEUR"), w[1]) +
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
		// Case à cocher seulement si la branche est supprimable sans perte ;
		// sinon la raison, sur la ligne du curseur (la répéter partout chargerait l'écran).
		box := "    "
		why := keepReason(r, br)
		if why == "" {
			box = checkbox(m.branchSel[br.Name]) + " "
		} else if i == m.branchCursor && !br.Current && !isMainBranch(br.Name, r.MainRef) {
			age += stDim.Render("   " + why)
		}
		line := cur + box + fit(name, w[0]) + sep + fit(renderSegs(links[i], ""), w[1]) +
			sep + fit(mainCell, w[2]) + sep + age
		b.WriteString(fit(line, m.width) + "\n")
	}
	b.WriteString(strings.Repeat("\n", max(0, h-min(len(list), h))))
	b.WriteString(helpLineFit(m.branchKeys(list, checked), m.width))
	return b.String()
}

// branchKeys : le pied de page de la vue branches, avec d'abord ce qui est
// possible sur la ligne du curseur, puis les touches de base.
func (m *model) branchKeys(list []Branch, checked []string) [][2]string {
	r := m.detail.repo
	var keys [][2]string
	var cur Branch
	if m.branchCursor < len(list) {
		cur = list[m.branchCursor]
	}
	deletable := cur.Name != "" && keepReason(r, cur) == ""
	if deletable {
		keys = append(keys, [2]string{"espace", "cocher"})
	}
	if len(r.MergedBranches) > 0 {
		keys = append(keys, [2]string{"a", "toutes les fusionnées"})
	}
	switch {
	case len(checked) > 0:
		label := fmt.Sprintf("supprimer les %d cochées", len(checked))
		if len(checked) == 1 {
			label = "supprimer la cochée"
		}
		keys = append(keys, [2]string{"d", label})
	case deletable:
		keys = append(keys, [2]string{"d", "supprimer celle-ci"})
	}
	if cur.Name != "" && linkable(cur) {
		keys = append(keys, [2]string{"u", "relier au serveur"})
	}
	keys = append(keys, [2]string{"⏎", "commits"}, [2]string{"r", "rafraîchir"})
	if len(checked) > 0 {
		keys = append(keys, [2]string{"esc", "décocher"})
	} else {
		keys = append(keys, [2]string{"esc", "retour"})
	}
	return append(keys, [2]string{"?", "aide"})
}
