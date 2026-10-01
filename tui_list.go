package main

// Vue liste (écran principal) : navigation, sélection, recherche, filtres,
// et dessin du tableau des dépôts.

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *model) keyList(msg tea.KeyMsg) tea.Cmd {
	vis := m.visible()
	switch msg.String() {
	case "q":
		m.saveSnapshot()
		m.cancel()
		return tea.Quit
	case "c":
		m.openChanges()
	case "up", "k":
		m.cursor--
	case "down", "j":
		m.cursor++
	case "pgup":
		m.cursor -= m.rowsHeight()
	case "pgdown":
		m.cursor += m.rowsHeight()
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(vis) - 1
	case " ", "x":
		if r := m.current(); r != nil {
			if m.selected[r.AbsPath] {
				delete(m.selected, r.AbsPath)
			} else {
				m.selected[r.AbsPath] = true
			}
			m.cursor++
		}
	case "a":
		all := len(vis) > 0
		for _, r := range vis {
			if !m.selected[r.AbsPath] {
				all = false
				break
			}
		}
		for _, r := range vis {
			if all {
				delete(m.selected, r.AbsPath)
			} else {
				m.selected[r.AbsPath] = true
			}
		}
	case "esc":
		switch {
		case m.query != "":
			m.query = ""
		case len(m.selected) > 0:
			m.selected = map[string]bool{}
		case m.attentionOnly:
			m.attentionOnly = false
		}
	case "t":
		m.attentionOnly = !m.attentionOnly
		m.cursor = 0
	case "o":
		m.sortByState = !m.sortByState
	case "/":
		m.searching = true
	case "?":
		m.openHelp()
	case "i":
		if r := m.current(); r != nil {
			m.openInfo(r)
		}
	case "b":
		if r := m.current(); r != nil {
			m.detailPath, m.detailRef, m.detail = r.AbsPath, "", nil
			m.wantBranches = true
			m.setMsg(false, "Chargement des branches…")
			return m.loadDetailCmd(r.AbsPath)
		}
	case "enter":
		if r := m.current(); r != nil {
			m.mode = modeDetail
			m.detailPath, m.detailRef = r.AbsPath, ""
			m.detail = nil
			m.vp.SetContent(stDim.Render("  Chargement…"))
			m.vp.GotoTop()
			return m.loadDetailCmd(r.AbsPath)
		}
	case "f":
		return m.startOp("fetch", m.targets())
	case "p":
		return m.startOp("pull", m.targets())
	case "P":
		m.askPush(m.targets())
	case "D":
		m.askClean(m.targets())
	case "S":
		return m.askSubmodules(m.targets())
	case "r":
		return m.startOp("scan", m.targets())
	case "R":
		m.setMsg(false, "Nouveau scan…")
		return m.discoverCmd()
	case "s":
		if r := m.current(); r != nil {
			return m.execIn(r.AbsPath, shell())
		}
	case "l":
		if r := m.current(); r != nil {
			return m.lazygit(r.AbsPath)
		}
	}
	m.clampCursor()
	return nil
}

func (m *model) keySearch(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyEsc:
		m.searching, m.query = false, ""
	case tea.KeyEnter:
		m.searching = false
	case tea.KeyBackspace:
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
		}
	case tea.KeyRunes, tea.KeySpace:
		m.query += string(msg.Runes)
	}
	m.cursor = 0
	m.clampCursor()
}

func severity(r *Repo) int {
	s := 0
	for _, f := range r.Flags {
		s = max(s, int(f.Level)*10+len(r.Flags))
	}
	return s
}

func (m *model) visible() []*Repo {
	q := strings.ToLower(m.query)
	var out []*Repo
	for _, r := range m.repos {
		if m.attentionOnly && r.Branch != "" && !r.NeedsAttention() {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(r.Path+" "+r.Branch), q) {
			continue
		}
		out = append(out, r)
	}
	if m.sortByState {
		sort.SliceStable(out, func(i, j int) bool { return severity(out[i]) > severity(out[j]) })
	}
	return out
}

func (m *model) current() *Repo {
	rows := m.visibleRows()
	if m.cursor >= 0 && m.cursor < len(rows) {
		return rows[m.cursor].repo
	}
	return nil
}

// targets : la sélection, ou à défaut le dépôt sous le curseur.
func (m *model) targets() []*Repo {
	var rs []*Repo
	for _, r := range m.repos {
		if m.selected[r.AbsPath] {
			rs = append(rs, r)
		}
	}
	if len(rs) == 0 {
		if r := m.current(); r != nil {
			rs = append(rs, r)
		}
	}
	return rs
}

func (m *model) rowsHeight() int { return max(1, m.height-4) }

func (m *model) clampCursor() {
	n := len(m.visible())
	m.cursor = max(0, min(m.cursor, n-1))
	h := m.rowsHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	m.offset = max(0, min(m.offset, max(0, n-h)))
}

// viewWelcome : écran d'accueil affiché pendant le premier scan.
func (m *model) viewWelcome() string {
	status := m.spin.View() + " Recherche des dépôts…"
	if m.scanTotal > 0 {
		status = fmt.Sprintf("%s Analyse des dépôts  %d / %d", m.spin.View(), m.scanDone, m.scanTotal)
		const barW = 30
		filled := barW * m.scanDone / m.scanTotal
		status += "\n\n" + stCyan.Render(strings.Repeat("━", filled)) + stDim.Render(strings.Repeat("━", barW-filled))
	}
	content := lipgloss.JoinVertical(lipgloss.Center,
		renderBanner(m.width),
		"",
		stDim.Render(tagline),
		"",
		"",
		stCyan.Render(status),
		"",
		stDim.Render(shortPath(m.root)),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *model) summary() string {
	var bad, todo, ok int
	for _, r := range m.repos {
		if r.Branch == "" && r.Error == "" {
			continue // pas encore analysé
		}
		switch buildCells(r).status {
		case Error:
			bad++
		case Warn:
			todo++
		default:
			ok++
		}
	}
	parts := []string{stBold.Render(fmt.Sprintf("%d %s", len(m.repos), plural(len(m.repos), "dépôt")))}
	if bad > 0 {
		parts = append(parts, stRed.Render(fmt.Sprintf("✗ %d à risque", bad)))
	}
	if todo > 0 {
		parts = append(parts, stYellow.Render(fmt.Sprintf("● %d à traiter", todo)))
	}
	parts = append(parts, stGreen.Render(fmt.Sprintf("✓ %d en ordre", ok)))
	if n := normalCount(m.repos); n > 0 {
		parts = append(parts, stDim.Render(fmt.Sprintf("(%s)", plur(n, "signal normal", "signaux normaux"))))
	}
	return strings.Join(parts, "   ")
}

// alertsCell : action en cours, résultat de la dernière action, puis les alertes.
func (m *model) alertsCell(r *Repo, c rowCells) string {
	if op, ok := m.busy[r.AbsPath]; ok {
		label := op
		if op == "scan" {
			label = "analyse"
		}
		return stCyan.Render(m.spin.View() + " " + label + "…")
	}
	var parts []string
	if res, ok := m.results[r.AbsPath]; ok && time.Since(res.at) < 5*time.Minute {
		if res.err != nil {
			parts = append(parts, stRed.Render("✗ "+res.op+" : "+res.err.Error()))
		} else {
			parts = append(parts, stGreen.Render("✓ "+res.op))
		}
	}
	if len(c.alerts) > 0 {
		parts = append(parts, renderSegs(c.alerts, " · "))
	}
	return strings.Join(parts, stDim.Render(" · "))
}

// visibleRows : dépôts affichés, en arbre (sous-modules sous leur parent)
// sauf en tri par gravité.
func (m *model) visibleRows() []treeRow {
	vis := m.visible()
	if m.sortByState {
		rows := make([]treeRow, len(vis))
		for i, r := range vis {
			rows[i] = treeRow{repo: r, name: r.Path}
		}
		return rows
	}
	return buildTree(vis, filepath.Base(m.root))
}

func (m *model) viewList() string {
	var b strings.Builder
	rows := m.visibleRows()
	m.clampCursor()

	// En-tête
	head := stTitle.Render("gitscan") + "  " + m.summary()
	if m.scanning {
		head += "   " + stCyan.Render(fmt.Sprintf("%s analyse %d/%d", m.spin.View(), m.scanDone, m.scanTotal))
	}
	head += "   " + stDim.Render(shortPath(m.root))
	b.WriteString(fit(head, m.width) + "\n")

	// Largeurs de colonnes
	cells := make([]rowCells, len(rows))
	w := []int{len("DÉPÔT"), len("BRANCHE"), len("vs SERVEUR"), len("vs MAIN"), len("LOCAL")}
	for i, tr := range rows {
		cells[i] = buildCells(tr.repo)
		w[0] = max(w[0], lipgloss.Width(tr.prefix+tr.name))
		w[1] = max(w[1], widthOf(cells[i].branch, ""))
		w[2] = max(w[2], widthOf(cells[i].server, ""))
		w[3] = max(w[3], widthOf(cells[i].main, " "))
		w[4] = max(w[4], widthOf(cells[i].local, " · "))
	}
	w[0], w[1], w[4] = min(w[0], 36), min(w[1], 24), min(w[4], 28)
	const lead = 6 // curseur, sélection, icône
	sep := stDim.Render(" │ ")
	fixed := func() int { return lead + w[0] + w[1] + w[2] + w[3] + w[4] + 5*3 }
	for _, k := range []struct{ col, floor int }{{0, 14}, {4, 14}, {1, 10}} {
		if over := fixed() + 24 - m.width; over > 0 {
			w[k.col] -= min(over, max(0, w[k.col]-k.floor))
		}
	}
	aw := max(8, m.width-fixed())

	hdrCells := []string{fit("DÉPÔT", w[0]), fit("BRANCHE", w[1]), fit("vs SERVEUR", w[2]), fit("vs MAIN", w[3]), fit("LOCAL", w[4]), "À VOIR"}
	for i := range hdrCells {
		hdrCells[i] = stBold.Render(hdrCells[i])
	}
	b.WriteString(fit(strings.Repeat(" ", lead)+strings.Join(hdrCells, sep), m.width) + "\n")

	h := m.rowsHeight()
	for i := m.offset; i < min(len(rows), m.offset+h); i++ {
		tr, c := rows[i], cells[i]
		r := tr.repo
		cur, sel := "  ", stDim.Render("○ ")
		if m.selected[r.AbsPath] {
			sel = stMag.Render("● ")
		}
		// Le nom du dépôt courant est en vidéo inverse : repérable même au milieu
		// d'une ligne pleine de couleurs (un fond sur toute la ligne serait coupé
		// par les réinitialisations des segments colorés).
		name := stBold.Render(tr.name)
		if i == m.cursor {
			cur = stCursor.Render("❯ ")
			name = stCursor.Reverse(true).Render(tr.name)
		}
		path := stDim.Render(tr.prefix) + name
		line := cur + sel + statusIconTUI(c.status) + " " + strings.Join([]string{
			fit(path, w[0]), fit(renderSegs(c.branch, ""), w[1]), fit(renderSegs(c.server, ""), w[2]),
			fit(renderSegs(c.main, " "), w[3]), fit(renderSegs(c.local, " · "), w[4]), fit(m.alertsCell(r, c), aw),
		}, sep)
		b.WriteString(line + "\n")
	}
	vis := rows
	shown := min(len(vis), h)
	if len(vis) == 0 && !m.scanning {
		b.WriteString(stDim.Render("    Aucun dépôt ne correspond.") + "\n")
		shown = 1
	}
	b.WriteString(strings.Repeat("\n", max(0, h-shown)))

	// Barre d'état
	var status []string
	if m.searching {
		status = append(status, stCyan.Render("/")+m.query+stCyan.Render("█"))
	} else if m.query != "" {
		status = append(status, stCyan.Render("recherche : ")+m.query)
	}
	if m.attentionOnly {
		status = append(status, stCyan.Render("[à traiter]"))
	}
	if m.sortByState {
		status = append(status, stCyan.Render("[tri : état]"))
	}
	if n := len(m.selected); n > 0 {
		status = append(status, stMag.Render(fmt.Sprintf("%d sélectionné(s)", n)))
	}
	if len(vis) > h {
		status = append(status, stDim.Render(fmt.Sprintf("%d–%d / %d", m.offset+1, m.offset+shown, len(vis))))
	}
	if m.msg != "" {
		st := stDim
		if m.msgErr {
			st = stRed
		}
		status = append(status, st.Render(m.msg))
	}
	b.WriteString(fit(strings.Join(status, "  "), m.width) + "\n")
	b.WriteString(fit(helpLine([][2]string{
		{"espace", "sélect."}, {"a", "tout"}, {"f", "fetch"}, {"p", "pull"}, {"P", "push"},
		{"⏎", "détail"}, {"b", "branches"}, {"i", "expliquer"}, {"c", "changements"}, {"t", "à traiter"}, {"/", "chercher"}, {"?", "aide"}, {"q", "quitter"},
	}), m.width))
	return b.String()
}
