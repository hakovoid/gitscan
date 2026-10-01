package main

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// branchModel : un modèle TUI ouvert sur la vue branches d'un vrai dépôt.
func branchModel(t *testing.T, base, dir string) *model {
	t.Helper()
	r := scan(t, base, dir, true)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &model{
		ctx: ctx, cancel: cancel, root: base, sem: make(chan struct{}, 2),
		repos: []*Repo{r}, busy: map[string]string{}, results: map[string]opResult{}, selected: map[string]bool{},
		mode: modeBranches, detailPath: r.AbsPath, detail: &detailData{repo: r}, branchSel: map[string]bool{},
		width: 140, height: 30,
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// cursorOn place le curseur de la vue branches sur une branche.
func cursorOn(t *testing.T, m *model, name string) {
	t.Helper()
	for i, b := range m.branches() {
		if b.Name == name {
			m.branchCursor = i
			return
		}
	}
	t.Fatalf("branche %s absente de la vue", name)
}

// runCmd exécute une commande Bubble Tea et ses éventuels lots, et renvoie les messages.
func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, runCmd(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func TestVueBranchesCocherPuisSupprimer(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "app"), base, "app")
	git(t, dir, "branch", "fusion-a")
	git(t, dir, "branch", "fusion-b")
	git(t, dir, "branch", "develop") // protégée, même fusionnée
	git(t, dir, "switch", "-q", "-c", "wip")
	commit(t, dir, "w", "travail")
	git(t, dir, "switch", "-q", "main")

	m := branchModel(t, base, dir)

	// Une branche avec du travail hors main n'a pas de case.
	cursorOn(t, m, "wip")
	m.keyBranches(key(" "))
	if len(m.checkedBranches()) != 0 || !m.msgErr || !strings.Contains(m.msg, "absent") {
		t.Errorf("wip ne doit pas pouvoir être cochée (msg %q, cochées %v)", m.msg, m.checkedBranches())
	}
	cursorOn(t, m, "develop")
	m.keyBranches(key(" "))
	if len(m.checkedBranches()) != 0 {
		t.Errorf("develop est protégée, cochées : %v", m.checkedBranches())
	}

	// a coche toutes les fusionnées, puis décoche tout.
	m.keyBranches(key("a"))
	if got := strings.Join(m.checkedBranches(), ","); got != "fusion-a,fusion-b" && got != "fusion-b,fusion-a" {
		t.Errorf("a : fusion-a et fusion-b attendues, cochées %q", got)
	}
	m.keyBranches(key("a"))
	if n := len(m.checkedBranches()); n != 0 {
		t.Errorf("second a : tout doit être décoché, %d cochée(s)", n)
	}

	// échap décoche avant de quitter la vue.
	cursorOn(t, m, "fusion-b")
	m.keyBranches(key("x"))
	m.keyBranches(key("esc"))
	if m.mode != modeBranches || len(m.checkedBranches()) != 0 {
		t.Errorf("échap avec des cases cochées : décocher et rester (mode %v, cochées %v)", m.mode, m.checkedBranches())
	}

	// L'affichage montre les cases et le nombre de cochées.
	cursorOn(t, m, "fusion-a")
	m.keyBranches(key(" "))
	if v := m.viewBranches(); !strings.Contains(v, "[x]") || !strings.Contains(v, "1 cochée") || !strings.Contains(v, "supprimer la cochée") {
		t.Errorf("vue branches : case [x], compteur et pied de page attendus\n%s", v)
	}

	// d ne propose que les cochées, et la confirmation les supprime vraiment.
	m.keyBranches(key("d"))
	if m.mode != modeConfirm || m.confirm == nil {
		t.Fatalf("d doit demander confirmation, mode %v", m.mode)
	}
	lines := strings.Join(rowTexts(m.confirm.rows), "\n")
	if !strings.Contains(lines, "fusion-a") || strings.Contains(lines, "fusion-b") {
		t.Errorf("la confirmation doit viser fusion-a seule :\n%s", lines)
	}
	for _, msg := range runCmd(m.keyConfirm(key("o"))) {
		if done, ok := msg.(opDoneMsg); ok && done.err != nil {
			t.Errorf("suppression : %v", done.err)
		}
	}
	left := git(t, dir, "branch", "--format=%(refname:short)")
	if strings.Contains(left, "fusion-a") || !strings.Contains(left, "fusion-b") || !strings.Contains(left, "wip") {
		t.Errorf("seule fusion-a devait disparaître, reste :\n%s", left)
	}
}
