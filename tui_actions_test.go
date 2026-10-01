package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPushDecocherUnDepot(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	bareA, bareB := server(t, base, "a"), server(t, base, "b")
	a, b := clone(t, bareA, base, "a"), clone(t, bareB, base, "b")
	commit(t, a, "f", "1")
	commit(t, b, "f", "1")
	// Une branche dont la distante a été supprimée : listée, mais sans case.
	gone := clone(t, server(t, base, "c"), base, "c")
	git(t, gone, "switch", "-q", "-c", "fini")
	commit(t, gone, "f", "1")
	git(t, gone, "push", "-q", "-u", "origin", "fini")
	git(t, gone, "push", "-q", "origin", "--delete", "fini")
	git(t, gone, "fetch", "-q", "--prune")

	m := listModel(t, base, scan(t, base, a, false), scan(t, base, b, false), scan(t, base, gone, false))
	for _, r := range m.repos {
		m.selected[r.AbsPath] = true
	}
	m.askPush(m.targets())
	c := m.confirm
	if m.mode != modeConfirm || len(c.choices()) != 2 || len(c.checked()) != 2 {
		t.Fatalf("deux dépôts cochés attendus (le troisième sans case), lignes : %v", rowTexts(c.rows))
	}
	if v := m.viewConfirm(); !strings.Contains(v, "2 / 2 coché(s)") || !strings.Contains(v, "non poussée") {
		t.Errorf("encadré : compteur et dépôt non poussé attendus\n%s", v)
	}

	// Tout décocher : la confirmation refuse de partir à vide.
	m.keyConfirm(key("a"))
	if cmd := m.keyConfirm(key("o")); cmd != nil || m.mode != modeConfirm {
		t.Fatal("rien de coché : o ne doit rien lancer ni fermer l'encadré")
	}
	if v := m.viewConfirm(); !strings.Contains(v, "Rien de coché") {
		t.Errorf("rien de coché : l'encadré doit le dire\n%s", v)
	}

	// Recocher tout, décocher le premier (a), confirmer : seul b est poussé.
	m.keyConfirm(key("a"))
	m.keyConfirm(key(" "))
	for _, msg := range runCmd(m.keyConfirm(key("o"))) {
		if done, ok := msg.(opDoneMsg); ok && done.err != nil {
			t.Errorf("push : %v", done.err)
		}
	}
	if n := git(t, bareA, "rev-list", "--count", "main"); n != "1" {
		t.Errorf("a était décoché : son serveur ne doit pas bouger (%s commits)", n)
	}
	if n := git(t, bareB, "rev-list", "--count", "main"); n != "2" {
		t.Errorf("b était coché : son commit doit être sur le serveur (%s commits)", n)
	}
}

func TestMenageDecocherUnDepot(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	a := clone(t, server(t, base, "a"), base, "a")
	b := clone(t, server(t, base, "b"), base, "b")
	git(t, a, "branch", "vieille")
	git(t, b, "branch", "vieille")
	m := listModel(t, base, scan(t, base, a, false), scan(t, base, b, false))
	for _, r := range m.repos {
		m.selected[r.AbsPath] = true
	}
	m.askClean(m.targets())
	m.keyConfirm(key("j")) // curseur sur b
	m.keyConfirm(key(" "))
	runCmd(m.keyConfirm(key("o")))
	if git(t, a, "branch", "--list", "vieille") != "" {
		t.Error("a était coché : sa branche fusionnée doit être supprimée")
	}
	if git(t, b, "branch", "--list", "vieille") == "" {
		t.Error("b était décoché : sa branche doit rester")
	}
}

func TestSousModulesDecocherUn(t *testing.T) {
	base, parent, lib := parentAvecSousModule(t)
	git(t, parent, "submodule", "add", "-q", server(t, base, "lib2"), "lib2")
	git(t, parent, "commit", "-q", "-m", "ajoute lib2")
	lib2 := filepath.Join(parent, "lib2")
	wantLib := git(t, lib, "rev-parse", "HEAD")
	commit(t, lib, "f", "1") // les deux sous-modules ont avancé sur leur branche
	commit(t, lib2, "f", "1")
	movedLib2 := git(t, lib2, "rev-parse", "HEAD")

	m := listModel(t, base, scan(t, base, parent, false))
	msgs := runCmd(m.askSubmodules(m.repos))
	if len(msgs) != 1 {
		t.Fatalf("un plan attendu, trouvé %v", msgs)
	}
	m.onSubPlan(msgs[0].(subPlanMsg))
	c := m.confirm
	if c == nil || len(c.choices()) != 2 {
		t.Fatalf("deux sous-modules à cocher attendus : %v", c)
	}
	m.keyConfirm(key("j")) // lib2
	m.keyConfirm(key(" "))
	for _, msg := range runCmd(m.keyConfirm(key("o"))) {
		if done, ok := msg.(opDoneMsg); ok && done.err != nil {
			t.Errorf("sous-modules : %v", done.err)
		}
	}
	if got := git(t, lib, "rev-parse", "HEAD"); got != wantLib {
		t.Errorf("lib était coché : il doit être revenu au commit attendu")
	}
	if got := git(t, lib2, "rev-parse", "HEAD"); got != movedLib2 {
		t.Errorf("lib2 était décoché : il ne doit pas bouger")
	}
}
