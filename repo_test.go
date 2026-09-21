package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDepotPropre(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "api"), base, "api")

	r := scan(t, base, dir, false)
	if len(r.Flags) != 0 {
		t.Errorf("dépôt propre : aucun signal attendu, trouvé %v", codes(r))
	}
	if r.Branch != "main" || r.Upstream != "origin/main" || r.MainRef != "origin/main" {
		t.Errorf("branche/upstream/main inattendus : %q %q %q", r.Branch, r.Upstream, r.MainRef)
	}
	if r.Head == "" {
		t.Error("le commit courant (Head) doit toujours être renseigné")
	}
	if r.NeedsAttention() {
		t.Error("un dépôt propre ne demande aucune action")
	}
}

func TestAvanceRetardDivergence(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	bare := server(t, base, "web")
	dir := clone(t, bare, base, "web")
	other := clone(t, bare, base, "collegue")

	commit(t, dir, "a", "1")
	r := scan(t, base, dir, false)
	mustHave(t, r, "ahead")
	if r.Ahead != 1 {
		t.Errorf("↑1 attendu, trouvé ↑%d", r.Ahead)
	}

	commit(t, other, "b", "2")
	git(t, other, "push", "-q")
	git(t, dir, "fetch", "-q")
	r = scan(t, base, dir, false)
	mustHave(t, r, "diverged")
	mustNotHave(t, r, "ahead", "behind")
}

func TestModificationsLocales(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "blog"), base, "blog")

	write(t, filepath.Join(dir, "README"), "changé\n")
	write(t, filepath.Join(dir, "nouveau.txt"), "x")
	git(t, dir, "stash", "-q")
	write(t, filepath.Join(dir, "README"), "changé encore\n")
	write(t, filepath.Join(dir, "nouveau.txt"), "x")

	r := scan(t, base, dir, false)
	mustHave(t, r, "dirty", "untracked", "stash")
	if r.Changed != 1 || r.Untracked != 1 || r.Stashes != 1 {
		t.Errorf("1 modifié, 1 nouveau, 1 stash attendus : %d %d %d", r.Changed, r.Untracked, r.Stashes)
	}
}

func TestDroitsSeulement(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "docs"), base, "docs")
	if err := os.Chmod(filepath.Join(dir, "README"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := scan(t, base, dir, false)
	mustHave(t, r, "mode_only")
	mustNotHave(t, r, "dirty")
	if r.NeedsAttention() {
		t.Error("un simple chmod ne doit pas demander d'action")
	}
}

// Bogue signalé : une branche poussée sans -u apparaissait « jamais poussée ».
func TestPousseeSansUpstream(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "app"), base, "app")
	git(t, dir, "switch", "-q", "-c", "init-prd")
	commit(t, dir, "x", "1")
	git(t, dir, "push", "-q", "origin", "init-prd") // sans -u

	r := scan(t, base, dir, true)
	mustHave(t, r, "unlinked")
	mustNotHave(t, r, "no_upstream")
	if r.MatchRemote != "origin/init-prd" || r.MatchAhead != 0 {
		t.Errorf("comparaison avec origin/init-prd attendue : %q ↑%d", r.MatchRemote, r.MatchAhead)
	}
}

func TestUpstreamDUnAutreNom(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "app"), base, "app")
	git(t, dir, "push", "-q", "origin", "main:nbl")
	git(t, dir, "switch", "-q", "-c", "init-prd")
	git(t, dir, "branch", "-q", "-u", "origin/nbl")

	r := scan(t, base, dir, false)
	mustHave(t, r, "upstream_other")
}

// Bogue signalé : les tags n'apparaissaient plus.
func TestHeadDetacheeSurTags(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "stopcom"), base, "stopcom")
	for _, tag := range []string{"sprint-30", "sprint-31", "sprint-33"} {
		git(t, dir, "tag", tag)
	}
	git(t, dir, "checkout", "-q", "--detach", "HEAD")

	r := scan(t, base, dir, false)
	if len(r.HeadTags) != 3 || r.HeadTags[0] != "sprint-33" {
		t.Fatalf("3 tags, sprint-33 en premier, attendus : %v", r.HeadTags)
	}
	c := buildCells(r)
	if !strings.Contains(plainOf(c.alerts, " · "), "tag sprint-33 (+2 autres sur ce commit)") {
		t.Errorf("le tag doit apparaître dans À VOIR : %q", plainOf(c.alerts, " · "))
	}
	if r.NeedsAttention() {
		t.Error("une version taguée n'est pas un problème")
	}
}

func TestCommitsHorsBranche(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "webform"), base, "webform")
	git(t, dir, "checkout", "-q", "--detach")
	commit(t, dir, "perdu", "x")

	r := scan(t, base, dir, false)
	mustHave(t, r, "orphan_commits")
	if buildCells(r).status != Error {
		t.Error("des commits hors branche doivent être « à risque »")
	}
}

func TestRebaseInterrompu(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	bare := server(t, base, "site")
	dir := clone(t, bare, base, "site")
	other := clone(t, bare, base, "autre")
	commit(t, other, "README", "version serveur\n")
	git(t, other, "push", "-q")
	commit(t, dir, "README", "version locale\n")
	git(t, dir, "fetch", "-q")
	cmd := []string{"-C", dir, "rebase", "origin/main"}
	_ = runQuiet(cmd...) // échoue sur conflit : c'est voulu

	r := scan(t, base, dir, false)
	mustHave(t, r, "operation", "conflicts")
	if r.Operation != "rebase" {
		t.Errorf("rebase attendu, trouvé %q", r.Operation)
	}
}

func TestSousModuleDecale(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	lib := server(t, base, "lib")
	parent := clone(t, server(t, base, "parent"), base, "parent")
	git(t, parent, "submodule", "add", "-q", lib, "lib")
	git(t, parent, "commit", "-q", "-m", "ajoute lib")
	sub := filepath.Join(parent, "lib")
	commit(t, sub, "f", "nouveau")

	r := scan(t, base, sub, false)
	if !r.Submodule || r.SubInSync {
		t.Fatalf("sous-module décalé attendu : submodule=%v insync=%v", r.Submodule, r.SubInSync)
	}
	mustHave(t, r, "submodule_drift")

	p := scan(t, base, parent, false)
	mustHave(t, p, "submodules_changed")
}

func runQuiet(args ...string) error {
	return exec.Command("git", args...).Run()
}
