package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// parentAvecSousModule : un dépôt parent qui contient le sous-module « lib ».
func parentAvecSousModule(t *testing.T) (base, parent, sub string) {
	t.Helper()
	gitEnv(t)
	base = t.TempDir()
	lib := server(t, base, "lib")
	parent = clone(t, server(t, base, "parent"), base, "parent")
	git(t, parent, "submodule", "add", "-q", lib, "lib")
	git(t, parent, "commit", "-q", "-m", "ajoute lib")
	return base, parent, filepath.Join(parent, "lib")
}

func TestSousModuleQuiRecule(t *testing.T) {
	_, parent, sub := parentAvecSousModule(t)
	want := git(t, sub, "rev-parse", "--short", "HEAD")
	commit(t, sub, "f", "1") // sur la branche main du sous-module : rien ne sera perdu
	commit(t, sub, "g", "2")

	ctx := context.Background()
	plans, err := planSubmodules(ctx, parent, nil)
	if err != nil || len(plans) != 1 {
		t.Fatalf("un sous-module décalé attendu : %v %v", plans, err)
	}
	p := plans[0]
	if p.Blocked != "" || p.Back != 2 || p.Forward != 0 || p.Want != want {
		t.Fatalf("recul de 2 commits vers %s attendu : %+v", want, p)
	}
	if !strings.Contains(p.describe(), "recule de 2 commits") {
		t.Errorf("la confirmation doit annoncer le recul : %q", p.describe())
	}

	if _, err := updateSubmodules(ctx, parent, []string{"lib"}); err != nil {
		t.Fatal(err)
	}
	if got := git(t, sub, "rev-parse", "--short", "HEAD"); got != want {
		t.Errorf("le sous-module doit être sur %s, il est sur %s", want, got)
	}
	// Les deux commits quittés sont toujours sur la branche main du sous-module.
	if n := git(t, sub, "rev-list", "--count", want+"..main"); n != "2" {
		t.Errorf("les commits quittés doivent rester sur leur branche (%s trouvés)", n)
	}
}

func TestSousModuleQuiAvance(t *testing.T) {
	_, parent, sub := parentAvecSousModule(t)
	git(t, sub, "checkout", "-q", "--detach", "HEAD")
	commit(t, sub, "f", "1")
	git(t, sub, "branch", "suite") // le commit est sur une branche
	git(t, parent, "commit", "-q", "-am", "lib avance")
	git(t, sub, "checkout", "-q", "HEAD~1")

	plans, err := planSubmodules(context.Background(), parent, nil)
	if err != nil || len(plans) != 1 || plans[0].Forward != 1 || plans[0].Back != 0 || plans[0].Blocked != "" {
		t.Fatalf("avance d'un commit attendue : %+v %v", plans, err)
	}
}

func TestSousModuleBloque(t *testing.T) {
	_, parent, sub := parentAvecSousModule(t)
	ctx := context.Background()

	// Modification non commitée : on n'y touche pas.
	commit(t, sub, "f", "1")
	write(t, filepath.Join(sub, "f"), "en cours")
	plans, _ := planSubmodules(ctx, parent, nil)
	if len(plans) != 1 || !strings.Contains(plans[0].Blocked, "non commitées") {
		t.Fatalf("blocage attendu (modifications) : %+v", plans)
	}
	if _, err := updateSubmodules(ctx, parent, []string{"lib"}); err == nil {
		t.Fatal("la mise à jour doit être refusée")
	}
	git(t, sub, "checkout", "-q", "--", "f")

	// Commit fait en HEAD détachée : il serait perdu.
	git(t, sub, "checkout", "-q", "--detach")
	commit(t, sub, "h", "orphelin")
	plans, _ = planSubmodules(ctx, parent, nil)
	if len(plans) != 1 || !strings.Contains(plans[0].Blocked, "sur aucune branche") {
		t.Fatalf("blocage attendu (commit orphelin) : %+v", plans)
	}
	if _, err := updateSubmodules(ctx, parent, []string{"lib"}); err == nil {
		t.Fatal("la mise à jour doit être refusée")
	}
}
