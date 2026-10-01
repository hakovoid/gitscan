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
// Avec rows et pick à la place de lines et yes, chaque élément a une case :
// tout est coché au départ, on décoche ce qu'on ne veut pas, puis on confirme.
type confirmation struct {
	title  string
	lines  []string // déjà mises en forme
	note   string   // précision sous la liste (facultative)
	yes    func() tea.Cmd
	back   viewMode
	cancel string // message si l'on répond non

	rows    []confirmRow
	pick    func(keys []string) tea.Cmd // reçoit les clés des éléments cochés
	titleOf func(keys []string) string  // titre selon les éléments cochés
	cursor  int                         // index dans rows, toujours sur un élément à cocher
}

// confirmRow : une ligne de la confirmation ; avec une clé, c'est un élément à cocher.
type confirmRow struct {
	text string
	key  string
	on   bool
}

// choices : les index des lignes à cocher.
func (c *confirmation) choices() []int {
	var idx []int
	for i, r := range c.rows {
		if r.key != "" {
			idx = append(idx, i)
		}
	}
	return idx
}

// checked : les clés des éléments cochés, dans l'ordre d'affichage.
func (c *confirmation) checked() []string {
	var keys []string
	for _, r := range c.rows {
		if r.key != "" && r.on {
			keys = append(keys, r.key)
		}
	}
	return keys
}

// moveCursor déplace le curseur d'un élément à cocher au suivant (d = ±1).
func (c *confirmation) moveCursor(d int) {
	idx := c.choices()
	if len(idx) == 0 {
		return
	}
	pos := 0
	for i, r := range idx {
		if r == c.cursor {
			pos = i
		}
	}
	c.cursor = idx[max(0, min(len(idx)-1, pos+d))]
}

func (m *model) ask(c *confirmation) {
	if idx := c.choices(); len(idx) > 0 {
		c.cursor = idx[0]
		for i := range c.rows {
			c.rows[i].on = c.rows[i].key != ""
		}
	}
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
	if c.pick != nil {
		switch msg.String() {
		case "up", "k":
			c.moveCursor(-1)
			return nil
		case "down", "j":
			c.moveCursor(1)
			return nil
		case " ", "x":
			if c.cursor < len(c.rows) && c.rows[c.cursor].key != "" {
				c.rows[c.cursor].on = !c.rows[c.cursor].on
			}
			return nil
		case "a":
			all := len(c.checked()) == len(c.choices())
			for _, i := range c.choices() {
				c.rows[i].on = !all
			}
			return nil
		case "o", "y", "enter":
			keys := c.checked()
			if len(keys) == 0 {
				return nil // rien de coché : le pied de l'encadré le dit
			}
			m.mode, m.confirm = c.back, nil
			return c.pick(keys)
		}
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
		if c.yes == nil && c.pick == nil {
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
	title := c.title
	if c.titleOf != nil {
		title = c.titleOf(c.checked())
	}
	b.WriteString(stBold.Render(title) + "\n\n")
	maxLines := max(3, m.height-12)
	if c.pick != nil {
		// Fenêtre qui suit le curseur quand la liste ne tient pas.
		start := max(0, min(c.cursor-maxLines/2, len(c.rows)-maxLines))
		if start > 0 {
			b.WriteString(stDim.Render(fmt.Sprintf("  … %d au-dessus", start)) + "\n")
		}
		for i := start; i < min(len(c.rows), start+maxLines); i++ {
			r := c.rows[i]
			switch {
			case r.key == "":
				b.WriteString("      " + r.text + "\n")
			case i == c.cursor:
				b.WriteString(stCursor.Render("❯ ") + checkbox(r.on) + " " + r.text + "\n")
			default:
				b.WriteString("  " + checkbox(r.on) + " " + r.text + "\n")
			}
		}
		if rest := len(c.rows) - start - maxLines; rest > 0 {
			b.WriteString(stDim.Render(fmt.Sprintf("  … %d en dessous", rest)) + "\n")
		}
	} else {
		for i, l := range c.lines {
			if i == maxLines {
				b.WriteString(stDim.Render(fmt.Sprintf("  … et %d autre(s)", len(c.lines)-maxLines)) + "\n")
				break
			}
			b.WriteString("  " + l + "\n")
		}
	}
	if c.note != "" {
		b.WriteString("\n" + stDim.Render(wrapText(c.note, max(30, min(90, m.width-10)))) + "\n")
	}
	switch {
	case c.pick != nil:
		n, total := len(c.checked()), len(c.choices())
		b.WriteString("\n" + stMag.Render(fmt.Sprintf("%d / %d coché(s)", n, total)) + "   ")
		b.WriteString(helpLine([][2]string{{"espace", "cocher"}, {"a", "tout"}}) + "\n")
		if n == 0 {
			b.WriteString(stYellow.Render("Rien de coché : coche au moins un élément, ou échap pour annuler."))
		} else {
			b.WriteString(stKey.Render("o") + stDim.Render(" / entrée : oui    ") + stKey.Render("n") + stDim.Render(" / échap : non"))
		}
	case c.yes == nil:
		b.WriteString("\n" + stKey.Render("échap") + stDim.Render(" : fermer"))
	default:
		b.WriteString("\n" + stKey.Render("o") + stDim.Render(" / entrée : oui    ") + stKey.Render("n") + stDim.Render(" / échap : non"))
	}
	st := stBox
	if lipgloss.Width(b.String())+st.GetHorizontalFrameSize() > m.width {
		st = st.Width(max(20, m.width-2))
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, st.Render(b.String()))
}

// ---------- Push ----------

// askPush ne garde que les dépôts qui ont quelque chose à pousser. Une branche
// dont la distante a été supprimée n'est pas poussée : c'est en général une
// merge request fusionnée, et un push recréerait la branche sur le serveur.
func (m *model) askPush(rs []*Repo) {
	var paths, gone []string
	var rows []confirmRow
	for _, r := range rs {
		if r.Detached || r.Error != "" {
			continue
		}
		if r.UpstreamGone {
			gone = append(gone, r.Path)
			continue
		}
		if r.Ahead == 0 && r.Upstream != "" {
			continue
		}
		paths = append(paths, r.AbsPath)
		what := stYellow.Render(fmt.Sprintf("↑%d", r.Ahead))
		if r.Upstream == "" {
			what = stCyan.Render("nouvelle branche distante")
		}
		rows = append(rows, confirmRow{key: r.AbsPath,
			text: fmt.Sprintf("%s  %s  %s", stBold.Render(r.Path), stCyan.Render(r.Branch), what)})
	}
	if len(paths) == 0 {
		if len(gone) > 0 {
			m.setMsg(true, "%s : branche distante supprimée (souvent après fusion), gitscan ne la recrée pas — i pour les détails", strings.Join(gone, ", "))
		} else {
			m.setMsg(false, "Rien à pousser dans la sélection.")
		}
		return
	}
	note := ""
	for _, g := range gone {
		rows = append(rows, confirmRow{text: stRed.Render("✗ ") + stBold.Render(g) + stDim.Render("  branche distante supprimée : non poussée")})
	}
	if len(gone) > 0 {
		note = "Une branche distante supprimée l'est souvent après la fusion de sa merge request : gitscan ne la recrée pas. " +
			"Pour la recréer quand même : git push -u origin HEAD dans le dépôt."
	}
	m.ask(&confirmation{
		titleOf: func(keys []string) string { return fmt.Sprintf("Pousser %s ?", plur(len(keys), "dépôt", "dépôts")) },
		rows:    rows,
		note:    note,
		cancel:  "Push annulé.",
		pick:    func(keys []string) tea.Cmd { return m.startOp("push", m.reposByPaths(keys)) },
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
	var rows []confirmRow
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
		rows = append(rows, confirmRow{key: r.AbsPath,
			text: stBold.Render(r.Path) + stDim.Render(" : ") + stCyan.Render(strings.Join(names, ", "))})
	}
	if len(jobs) == 0 {
		if len(only) > 0 {
			m.setMsg(true, "%s a des commits absents de main : gitscan ne la supprime pas", only[0])
		} else {
			m.setMsg(false, "Aucune branche fusionnée à supprimer dans la sélection.")
		}
		return
	}
	m.ask(&confirmation{
		titleOf: func(keys []string) string {
			n := 0
			for _, j := range jobs {
				if contains(keys, j.r.AbsPath) {
					n += len(j.names)
				}
			}
			return fmt.Sprintf("Supprimer %s ?", plur(n, "branche fusionnée", "branches fusionnées"))
		},
		rows: rows,
		note: "Tous leurs commits sont déjà dans main : rien n'est perdu. gitscan le revérifie " +
			"branche par branche juste avant de supprimer, et affiche le commit de chacune pour pouvoir la recréer.",
		cancel: "Ménage annulé.",
		pick: func(keys []string) tea.Cmd {
			var cmds []tea.Cmd
			for _, j := range jobs {
				if _, busy := m.busy[j.r.AbsPath]; !busy && contains(keys, j.r.AbsPath) {
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
