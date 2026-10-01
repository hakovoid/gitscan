package main

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// listModel : un modèle TUI sur la liste, avec les dépôts déjà analysés.
func listModel(t *testing.T, base string, repos ...*Repo) *model {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &model{
		ctx: ctx, cancel: cancel, root: base, sem: make(chan struct{}, 2), repos: repos,
		busy: map[string]string{}, results: map[string]opResult{}, selected: map[string]bool{},
		welcomed: true, width: 160, height: 20,
	}
}

func keyLabels(keys [][2]string) string {
	var s []string
	for _, k := range keys {
		s = append(s, k[0]+" "+k[1])
	}
	return strings.Join(s, " | ")
}

func TestListeCasesEtPiedDePageContextuel(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "web"), base, "web")
	git(t, dir, "branch", "vieille")
	commit(t, dir, "a", "1")
	commit(t, dir, "b", "2")
	m := listModel(t, base, scan(t, base, dir, false))

	keys := m.listKeys()
	if keys[0] != [2]string{"P", "pousser ↑2"} {
		t.Errorf("le dépôt à pousser doit proposer P en premier, pied de page : %s", keyLabels(keys))
	}
	if !strings.Contains(keyLabels(keys), "D 1 branche fusionnée") {
		t.Errorf("la branche fusionnée doit être proposée au ménage : %s", keyLabels(keys))
	}
	if last := keys[len(keys)-1]; last[0] != "?" {
		t.Errorf("« ? aide » doit rester en dernier, trouvé %v", last)
	}
	if strings.Count(keyLabels(keys), "P ") != 1 {
		t.Errorf("une touche ne doit apparaître qu'une fois : %s", keyLabels(keys))
	}

	if v := m.viewList(); !strings.Contains(v, "[ ]") || strings.Contains(v, "○") {
		t.Errorf("la liste doit montrer des cases [ ] :\n%s", v)
	}
	m.keyList(key(" "))
	if v := m.viewList(); !strings.Contains(v, "[x]") || !strings.Contains(v, "fetch (1)") {
		t.Errorf("après espace : case [x] et actions sur la sélection attendues :\n%s", v)
	}
}

func TestPiedDePageTientDansLaLargeur(t *testing.T) {
	keys := [][2]string{{"P", "pousser ↑2"}, {"⏎", "détail"}, {"b", "branches"}, {"c", "changements"}, {"?", "aide"}}
	for _, w := range []int{80, 40, 25, 12} {
		s := helpLineFit(keys, w)
		if lipgloss.Width(s) > w {
			t.Errorf("largeur %d : ligne de %d colonnes : %q", w, lipgloss.Width(s), s)
		}
		if w >= 25 && (!strings.Contains(s, "P pousser") || !strings.Contains(s, "? aide")) {
			t.Errorf("largeur %d : la première touche et « ? aide » doivent rester : %q", w, s)
		}
	}
	if s := helpLineFit(keys, 200); !strings.Contains(s, "c changements") {
		t.Errorf("assez de place : toutes les touches attendues : %q", s)
	}
}

func TestDetailSansCouleur(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "api"), base, "api")
	write(t, dir+"/README", "changé\n")
	m := listModel(t, base, scan(t, base, dir, false))
	m.detailPath = dir
	msg := m.loadDetailCmd(dir)().(detailMsg)
	m.detail = &detailData{repo: msg.repo, log: msg.log, logRef: msg.logRef, files: msg.files}
	if out := m.renderDetail(); strings.Contains(out, "\x1b[") {
		t.Errorf("couleurs coupées : aucun code ANSI attendu dans le détail :\n%q", out)
	}
	m.colors = true
	msg = m.loadDetailCmd(dir)().(detailMsg)
	if !strings.Contains(msg.log, "\x1b[") {
		t.Errorf("couleurs autorisées : git log doit être coloré, trouvé %q", msg.log)
	}
}

func TestLargeurColonneBranche(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "app"), base, "app")
	long := "feature/ALTAIRSUP-1825-size-pop-up-email-creation"
	git(t, dir, "switch", "-q", "-c", long)
	m := listModel(t, base, scan(t, base, dir, false))
	m.width = 150

	v := m.viewList()
	if strings.Contains(v, "│ "+long) || !strings.Contains(v, "branche : "+long) || !strings.Contains(v, "+ élargir BRANCHE") {
		t.Fatalf("nom long : coupé dans le tableau, en entier dans la barre d'état, + proposé\n%s", v)
	}
	for i := 0; i < 10 && m.branchCut; i++ {
		m.keyList(key("+"))
		v = m.viewList()
	}
	if !strings.Contains(v, "│ "+long) || strings.Contains(v, "branche : ") {
		t.Fatalf("après +, le nom doit tenir dans la colonne\n%s", v)
	}
	m.keyList(key("+"))
	if !strings.Contains(m.msg, "déjà") {
		t.Errorf("+ au maximum : un message attendu, trouvé %q", m.msg)
	}
	wide := m.branchColW
	m.keyList(key("-"))
	m.viewList()
	if m.branchColW != wide-branchStep {
		t.Errorf("- : largeur %d attendue, trouvé %d", wide-branchStep, m.branchColW)
	}
	m.keyList(key("0"))
	m.viewList()
	if m.branchW != 0 || m.branchColW != 24 {
		t.Errorf("0 : retour à la largeur automatique (24), trouvé %d (voulu %d)", m.branchColW, m.branchW)
	}
}

func TestLargeurBrancheDansLaVueBranches(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "app"), base, "app")
	long := "chore/tres-long-nom-de-branche-qui-ne-tient-pas-dans-la-colonne"
	git(t, dir, "branch", long)
	m := branchModel(t, base, dir)
	cursorOn(t, m, long)
	if v := m.viewBranches(); !strings.Contains(v, "branche : "+long) {
		t.Fatalf("nom coupé : il doit apparaître en entier dans le titre\n%s", v)
	}
	for i := 0; i < 10 && m.branchColW < m.branchColMax; i++ {
		m.keyBranches(key("+"))
		m.viewBranches()
	}
	if v := m.viewBranches(); strings.Contains(v, "branche : ") || !strings.Contains(v, long+" ") {
		t.Errorf("après +, le nom doit tenir dans la colonne\n%s", v)
	}
}
