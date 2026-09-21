package main

import (
	"context"
	"os"
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
	if p.Blocked != "" || p.Back != 2 || p.Forward != 0 || short(p.Want) != want {
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

// Relecture n°3 : un fichier ignoré (config.php avec des secrets locaux) que le
// commit attendu suit, lui : git l'écraserait sans prévenir.
func TestSousModuleFichierIgnoreEcrase(t *testing.T) {
	_, parent, sub := parentAvecSousModule(t)
	commit(t, sub, "config.php", "valeur par défaut\n") // le parent attendra ce commit
	git(t, parent, "commit", "-q", "-am", "lib suit config.php")
	git(t, sub, "rm", "-q", "--cached", "config.php")
	write(t, filepath.Join(sub, ".gitignore"), "config.php\n")
	git(t, sub, "add", ".gitignore")
	git(t, sub, "commit", "-q", "-m", "config.php devient local")
	write(t, filepath.Join(sub, "config.php"), "secrets du serveur\n")

	plans, _ := planSubmodules(context.Background(), parent, nil)
	if len(plans) != 1 || !strings.Contains(plans[0].Blocked, "config.php") {
		t.Fatalf("blocage attendu (config.php serait écrasé) : %+v", plans)
	}
	if _, err := updateSubmodules(context.Background(), parent, []string{"lib"}); err == nil {
		t.Fatal("la mise à jour doit être refusée")
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(sub, "config.php"))); got != "secrets du serveur" {
		t.Errorf("config.php a été modifié : %q", got)
	}
}

// Relecture n°4 : submodule.<nom>.update=rebase ne doit pas réécrire la branche.
func TestSousModuleRegleEnRebase(t *testing.T) {
	_, parent, sub := parentAvecSousModule(t)
	git(t, parent, "config", "submodule.lib.update", "rebase")
	before := git(t, sub, "rev-parse", "main")
	commit(t, sub, "f", "1")
	after := git(t, sub, "rev-parse", "main")

	if _, err := updateSubmodules(context.Background(), parent, []string{"lib"}); err != nil {
		t.Fatal(err)
	}
	if got := git(t, sub, "rev-parse", "main"); got != after {
		t.Errorf("la branche main du sous-module a été réécrite : %s → %s", after, got)
	}
	if got := git(t, sub, "rev-parse", "HEAD"); got != before {
		t.Errorf("le sous-module doit être sur le commit attendu %s, il est sur %s", before, got)
	}
}

// Relecture n°5 et 6 : chemin avec espace, chemin qui n'est pas à mettre à jour.
func TestSousModuleCheminAvecEspace(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	lib := server(t, base, "lib")
	parent := clone(t, server(t, base, "parent"), base, "parent")
	git(t, parent, "submodule", "add", "-q", lib, "ma lib")
	git(t, parent, "commit", "-q", "-m", "ajoute ma lib")
	commit(t, filepath.Join(parent, "ma lib"), "f", "1")

	plans, err := planSubmodules(context.Background(), parent, []string{"ma lib"})
	if err != nil || len(plans) != 1 || plans[0].Path != "ma lib" || plans[0].Blocked != "" || plans[0].Back != 1 {
		t.Fatalf("« ma lib » recule d'un commit : %+v %v", plans, err)
	}
	if _, err := updateSubmodules(context.Background(), parent, []string{"README"}); err == nil {
		t.Error("un chemin qui n'est pas un sous-module décalé doit être refusé")
	}
	if _, err := updateSubmodules(context.Background(), parent, []string{"ma lib"}); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Seconde relecture : un fichier ignoré « cache » remplacé par un dossier « cache/ ».
func TestSousModuleFichierRemplaceParUnDossier(t *testing.T) {
	_, parent, sub := parentAvecSousModule(t)
	commit(t, sub, "cache/keep", "") // le parent attendra un dossier cache/
	git(t, parent, "commit", "-q", "-am", "lib a un dossier cache")
	git(t, sub, "rm", "-q", "-r", "--cached", "cache")
	os.RemoveAll(filepath.Join(sub, "cache"))
	write(t, filepath.Join(sub, ".gitignore"), "cache\n")
	git(t, sub, "add", ".gitignore")
	git(t, sub, "commit", "-q", "-m", "cache devient local")
	write(t, filepath.Join(sub, "cache"), "données locales\n")

	plans, _ := planSubmodules(context.Background(), parent, nil)
	if len(plans) != 1 || !strings.Contains(plans[0].Blocked, "cache") {
		t.Fatalf("blocage attendu (le fichier cache serait remplacé) : %+v", plans)
	}
}
