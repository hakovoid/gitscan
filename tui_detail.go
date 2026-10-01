package main

// Vue détail d'un dépôt (touche entrée) : fichiers modifiés, branches,
// configuration, derniers commits.

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *model) keyDetail(msg tea.KeyMsg) tea.Cmd {
	r := m.repoByPath(m.detailPath)
	switch msg.String() {
	case "esc", "q", "backspace", "left", "h":
		m.mode = modeList
		m.detail = nil
		return nil
	case "f", "p", "r":
		if r != nil {
			op := map[string]string{"f": "fetch", "p": "pull", "r": "scan"}[msg.String()]
			return m.startOp(op, []*Repo{r})
		}
	case "P":
		if r != nil {
			m.askPush([]*Repo{r})
		}
		return nil
	case "D":
		if r != nil {
			m.askClean([]*Repo{r})
		}
		return nil
	case "S":
		if r != nil {
			return m.askSubmodules([]*Repo{r})
		}
		return nil
	case "?":
		m.openHelp()
		return nil
	case "i":
		if r := m.repoByPath(m.detailPath); r != nil {
			m.openInfo(r)
		}
		return nil
	case "b":
		if m.detail != nil && len(m.detail.repo.Branches) > 0 {
			m.mode, m.branchCursor, m.branchSel = modeBranches, 0, map[string]bool{}
		}
		return nil
	case "s":
		return m.execIn(m.detailPath, shell())
	case "l":
		return m.lazygit(m.detailPath)
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return cmd
}

func (m *model) renderDetail() string {
	d := m.detail
	if d == nil {
		return ""
	}
	r := d.repo
	p := newPalette(m.colors)
	var b strings.Builder
	section := func(t string) { b.WriteString("\n" + stTitle.Render(t) + "\n") }

	b.WriteString(stDim.Render("  "+shortPath(r.AbsPath)) + "\n")
	var flags []string
	for _, f := range r.Flags {
		label := f.Label
		if f.Normal != "" {
			label += " (normal)"
		}
		flags = append(flags, levelStyle(f.Level).Render(label))
	}
	if len(flags) == 0 {
		flags = append(flags, stGreen.Render("✓ propre et à jour"))
	}
	b.WriteString("  " + strings.Join(flags, stDim.Render(" · ")) + "\n")

	if res, ok := m.results[r.AbsPath]; ok {
		section(fmt.Sprintf("Dernière action : %s (il y a %s)", res.op, humanAge(time.Since(res.at))))
		if res.err != nil {
			b.WriteString("  " + stRed.Render("✗ "+res.err.Error()) + "\n")
		} else {
			b.WriteString("  " + stGreen.Render("✓ réussi") + "\n")
		}
		if res.out != "" {
			b.WriteString(indent(stDim.Render(res.out), "    ") + "\n")
		}
	}

	section("Fichiers modifiés")
	if strings.TrimSpace(d.files) == "" {
		b.WriteString(stDim.Render("  aucun") + "\n")
	} else {
		b.WriteString(indent(strings.TrimRight(d.files, "\n"), "  ") + "\n")
	}

	section("Branches")
	renderBranches(&b, r, p, "  ")

	if r.Config != nil {
		section("Configuration")
		renderConfig(&b, r, p, "  ")
	}

	section("Derniers commits — " + d.logRef)
	if strings.TrimSpace(d.log) == "" {
		b.WriteString(stDim.Render("  aucun commit") + "\n")
	} else {
		b.WriteString(indent(strings.TrimRight(d.log, "\n"), "  ") + "\n")
	}
	return b.String()
}

func (m *model) viewDetail() string {
	r := m.repoByPath(m.detailPath)
	title := stTitle.Render("‹ ")
	if r != nil {
		title += stBold.Render(r.Path) + "  " + renderSegs(buildCells(r).branch, "")
		if op, ok := m.busy[r.AbsPath]; ok {
			title += "  " + stCyan.Render(m.spinner()+" "+op+"…")
		}
	}
	if m.msg != "" {
		st := stDim
		if m.msgErr {
			st = stRed
		}
		title += "   " + st.Render(m.msg)
	}
	pct := ""
	if m.vp.TotalLineCount() > m.vp.Height {
		pct = stDim.Render(fmt.Sprintf("   %d%%", int(m.vp.ScrollPercent()*100)))
	}
	footer := helpLineFit([][2]string{
		{"esc", "retour"}, {"↑↓", "défiler"}, {"b", "branches"}, {"i", "expliquer"}, {"f", "fetch"},
		{"p", "pull"}, {"P", "push"}, {"r", "rafraîchir"}, {"s", "shell"}, {"l", "lazygit"},
		{"?", "aide"},
	}, m.width-lipgloss.Width(pct)) + pct
	return fit(title, m.width) + "\n" + m.vp.View() + "\n" + fit(footer, m.width)
}
