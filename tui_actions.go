package main

// Actions qui modifient un dépôt et demandent donc une confirmation : push,
// ménage des branches fusionnées, remise des sous-modules au commit attendu.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	opClean      = "ménage"
	opSubmodules = "sous-modules"
)

// confirmation : une question oui/non, avec ce qui va se passer ligne à ligne.
type confirmation struct {
	title  string
	lines  []string // déjà mises en forme
	note   string   // précision sous la liste (facultative)
	yes    func() tea.Cmd
	back   viewMode
	cancel string // message si l'on répond non
}

func (m *model) ask(c *confirmation) {
	c.back = m.mode
	m.confirm = c
	m.mode = modeConfirm
}

func (m *model) keyConfirm(msg tea.KeyMsg) tea.Cmd {
	c := m.confirm
	if c == nil {
		m.mode = modeList
		return nil
	}
	switch msg.String() {
	case "o", "y", "enter":
		m.mode, m.confirm = c.back, nil
		if c.yes == nil {
			return nil
		}
		return c.yes()
	case "n", "esc", "q":
		m.mode, m.confirm = c.back, nil
		m.setMsg(false, "%s", c.cancel)
		if c.yes == nil {
			m.msg = ""
		}
	}
	return nil
}

func (m *model) viewConfirm() string {
	c := m.confirm
	if c == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(stBold.Render(c.title) + "\n\n")
	maxLines := max(3, m.height-12)
	for i, l := range c.lines {
		if i == maxLines {
			b.WriteString(stDim.Render(fmt.Sprintf("  … et %d autre(s)", len(c.lines)-maxLines)) + "\n")
			break
		}
		b.WriteString("  " + l + "\n")
	}
	if c.note != "" {
		b.WriteString("\n" + stDim.Render(wrapText(c.note, max(30, min(90, m.width-10)))) + "\n")
	}
	if c.yes == nil {
		b.WriteString("\n" + stKey.Render("échap") + stDim.Render(" : fermer"))
	} else {
		b.WriteString("\n" + stKey.Render("o") + stDim.Render(" / entrée : oui    ") + stKey.Render("n") + stDim.Render(" / échap : non"))
	}
	st := stBox
	if lipgloss.Width(b.String())+st.GetHorizontalFrameSize() > m.width {
		st = st.Width(max(20, m.width-2))
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, st.Render(b.String()))
}

// ---------- Push ----------

// askPush ne garde que les dépôts qui ont quelque chose à pousser.
func (m *model) askPush(rs []*Repo) {
	var paths, lines []string
	for _, r := range rs {
		if r.Detached || r.Error != "" || !(r.Ahead > 0 || r.Upstream == "" || r.UpstreamGone) {
			continue
		}
		paths = append(paths, r.AbsPath)
		what := stYellow.Render(fmt.Sprintf("↑%d", r.Ahead))
		if r.Upstream == "" || r.UpstreamGone {
			what = stCyan.Render("nouvelle branche distante")
		}
		lines = append(lines, fmt.Sprintf("%s  %s  %s", stBold.Render(r.Path), stCyan.Render(r.Branch), what))
	}
	if len(paths) == 0 {
		m.setMsg(false, "Rien à pousser dans la sélection.")
		return
	}
	m.ask(&confirmation{
		title:  fmt.Sprintf("Pousser %d dépôt(s) ?", len(paths)),
		lines:  lines,
		cancel: "Push annulé.",
		yes:    func() tea.Cmd { return m.startOp("push", m.reposByPaths(paths)) },
	})
}

// ---------- Ménage des branches ----------

// askClean propose de supprimer des branches fusionnées. only limite à
// certaines branches (vue branches) ; vide = toutes celles du dépôt.
func (m *model) askClean(rs []*Repo, only ...string) {
	type job struct {
		r     *Repo
		names []string
	}
	var jobs []job
	var lines []string
	total := 0
	for _, r := range rs {
		names := r.MergedBranches
		if len(only) > 0 {
			names = nil
			for _, n := range only {
				if contains(r.MergedBranches, n) {
					names = append(names, n)
				}
			}
		}
		if len(names) == 0 {
			continue
		}
		jobs = append(jobs, job{r, names})
		total += len(names)
		lines = append(lines, stBold.Render(r.Path)+stDim.Render(" : ")+stCyan.Render(strings.Join(names, ", ")))
	}
	if len(jobs) == 0 {
		if len(only) > 0 {
			m.setMsg(true, "%s a des commits absents de main : gitscan ne la supprime pas", only[0])
		} else {
			m.setMsg(false, "Aucune branche fusionnée à supprimer dans la sélection.")
		}
		return
	}
	title := fmt.Sprintf("Supprimer %s ?", plur(total, "branche fusionnée", "branches fusionnées"))
	m.ask(&confirmation{
		title: title,
		lines: lines,
		note: "Tous leurs commits sont déjà dans main : rien n'est perdu. gitscan le revérifie " +
			"branche par branche juste avant de supprimer, et affiche le commit de chacune pour pouvoir la recréer.",
		cancel: "Ménage annulé.",
		yes: func() tea.Cmd {
			var cmds []tea.Cmd
			for _, j := range jobs {
				if _, busy := m.busy[j.r.AbsPath]; !busy {
					cmds = append(cmds, m.opCmd(j.r, opClean, false, j.names...))
				}
			}
			return tea.Batch(cmds...)
		},
	})
}

func (m *model) reposByPaths(paths []string) []*Repo {
	var rs []*Repo
	for _, p := range paths {
		if r := m.repoByPath(p); r != nil {
			rs = append(rs, r)
		}
	}
	return rs
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
