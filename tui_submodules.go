package main

// Touche S : remettre les sous-modules sur le commit attendu par leur parent.
// Le plan est calculé d'abord (en arrière-plan), puis présenté pour confirmation.

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type subPlanMsg struct {
	parents []string             // dans l'ordre d'affichage
	plans   map[string][]subPlan // parent -> sous-modules décalés
	errs    map[string]error
}

// askSubmodules part de la sélection : un dépôt parent vise tous ses
// sous-modules décalés, un sous-module ne vise que lui-même.
func (m *model) askSubmodules(rs []*Repo) tea.Cmd {
	want := map[string][]string{} // parent -> chemins (nil = tous)
	all := map[string]bool{}
	for _, r := range rs {
		if r.SubmodulesChanged > 0 {
			all[r.AbsPath] = true
			want[r.AbsPath] = nil
		}
		if r.Submodule && !r.SubInSync && r.SuperProject != "" && !all[r.SuperProject] {
			if rel, err := filepath.Rel(r.SuperProject, r.AbsPath); err == nil {
				want[r.SuperProject] = append(want[r.SuperProject], rel)
			}
		}
	}
	if len(want) == 0 {
		m.setMsg(false, "Aucun sous-module décalé dans la sélection.")
		return nil
	}
	m.setMsg(false, "Analyse des sous-modules…")
	ctx := m.ctx
	return func() tea.Msg {
		msg := subPlanMsg{plans: map[string][]subPlan{}, errs: map[string]error{}}
		for parent, only := range want {
			msg.parents = append(msg.parents, parent)
			plans, err := planSubmodules(ctx, parent, only)
			msg.plans[parent], msg.errs[parent] = plans, err
		}
		sort.Strings(msg.parents)
		return msg
	}
}

func (m *model) onSubPlan(msg subPlanMsg) {
	m.msg = ""
	var lines []string
	jobs := map[string][]string{}
	var behind []string
	for _, parent := range msg.parents {
		name := m.displayPath(parent)
		if err := msg.errs[parent]; err != nil {
			lines = append(lines, stBold.Render(name)+"  "+stRed.Render("✗ "+err.Error()))
			continue
		}
		plans := msg.plans[parent]
		if len(plans) == 0 {
			continue
		}
		head := stBold.Render(name)
		if pr := m.repoByPath(parent); pr != nil && pr.Behind > 0 {
			head += "  " + stYellow.Render(fmt.Sprintf("(le parent a ↓%d à tirer)", pr.Behind))
			behind = append(behind, fmt.Sprintf("%s (↓%d)", name, pr.Behind))
		}
		lines = append(lines, head)
		pathW := 0
		for _, p := range plans {
			pathW = max(pathW, len([]rune(p.Path)))
		}
		for _, p := range plans {
			from := p.Current
			if p.Uninit {
				from = "—"
			}
			move := stMag.Render(fmt.Sprintf("%-7s → %-7s", from, p.Want))
			st := stDim
			switch {
			case p.Blocked != "":
				st = stRed
			case p.Back > 0:
				st = stYellow
			}
			mark := "  "
			if p.Blocked != "" {
				mark = stRed.Render("✗ ")
			} else {
				jobs[parent] = append(jobs[parent], p.Path)
			}
			lines = append(lines, fmt.Sprintf("  %s%s  %s  %s", mark,
				stCyan.Render(fmt.Sprintf("%-*s", pathW, p.Path)), move, st.Render(p.describe())))
		}
	}
	n := 0
	for _, paths := range jobs {
		n += len(paths)
	}
	if n == 0 {
		if len(lines) == 0 {
			m.setMsg(false, "Rien à faire : les sous-modules sont déjà sur le commit attendu (voir l'index du parent).")
			return
		}
		m.ask(&confirmation{
			title: "Aucun sous-module ne peut être remis sans risque",
			lines: lines,
			note:  "Règle d'abord ce qui bloque (commit, branche…), puis relance S.",
			yes:   nil, // simple information
		})
		return
	}
	note := "Chaque sous-module sera placé sur le commit exact qu'attend son parent (HEAD détachée, " +
		"comme après un clone). Les sous-modules marqués ✗ ne seront pas touchés."
	if len(behind) > 0 {
		note += " ⚠ " + strings.Join(behind, ", ") + " : le parent est en retard sur le serveur et attend " +
			"peut-être d'anciennes versions. Le bon ordre est souvent : p (pull) sur le parent, puis S."
	}
	m.ask(&confirmation{
		title:  fmt.Sprintf("Remettre %s au commit attendu ?", plur(n, "sous-module", "sous-modules")),
		lines:  lines,
		note:   note,
		cancel: "Sous-modules non modifiés.",
		yes: func() tea.Cmd {
			var cmds []tea.Cmd
			for parent, paths := range jobs {
				r := m.repoByPath(parent)
				if r == nil {
					r = &Repo{AbsPath: parent, Path: m.displayPath(parent)}
				}
				if _, busy := m.busy[parent]; !busy {
					cmds = append(cmds, m.opCmd(r, opSubmodules, false, paths...))
				}
			}
			return tea.Batch(cmds...)
		},
	})
}

// afterSubmodules : ré-analyse les sous-modules d'un parent qui vient d'être traité.
func (m *model) afterSubmodules(parent string) tea.Cmd {
	var cmds []tea.Cmd
	for _, r := range m.repos {
		if r.SuperProject == parent {
			if _, busy := m.busy[r.AbsPath]; !busy {
				cmds = append(cmds, m.opCmd(r, "scan", false))
			}
		}
	}
	return tea.Batch(cmds...)
}

// displayPath : chemin relatif au dossier scanné quand c'est possible.
func (m *model) displayPath(abs string) string {
	if rel, err := filepath.Rel(m.root, abs); err == nil && !strings.HasPrefix(rel, "..") {
		if rel == "." {
			return filepath.Base(m.root)
		}
		return rel
	}
	return shortPath(abs)
}
