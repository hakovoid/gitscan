package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestBranchesFusionnees(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	bare := server(t, base, "crm")
	dir := clone(t, bare, base, "crm")

	// fusionnee : ses commits sont arrivés sur origin/main.
	git(t, dir, "switch", "-q", "-c", "fusionnee")
	commit(t, dir, "f", "1")
	git(t, dir, "switch", "-q", "main")
	git(t, dir, "merge", "-q", "--ff-only", "fusionnee")
	git(t, dir, "push", "-q")
	// en-cours : un commit qui n'est nulle part ailleurs.
	git(t, dir, "switch", "-q", "-c", "en-cours")
	commit(t, dir, "g", "2")
	// develop : fusionnée aussi, mais protégée.
	git(t, dir, "branch", "develop", "main")
	// courante : fusionnée, mais c'est la branche active.
	git(t, dir, "switch", "-q", "-c", "courante", "main")

	r := scan(t, base, dir, false)
	if !reflect.DeepEqual(r.MergedBranches, []string{"fusionnee"}) {
		t.Fatalf("seule « fusionnee » est supprimable, trouvé %v", r.MergedBranches)
	}
	mustHave(t, r, "merged_branches")
	for _, f := range r.Flags {
		if f.Code == "merged_branches" && f.Level != Info {
			t.Error("des branches fusionnées sont une information, pas une alerte")
		}
	}

	ctx := context.Background()
	done, err := deleteMerged(ctx, dir, r.MainRef, []string{"fusionnee"})
	if err != nil || len(done) != 1 || !strings.HasPrefix(done[0], "fusionnee (était ") {
		t.Fatalf("suppression attendue : %v %v", done, err)
	}

	// Garde-fous : jamais une branche avec des commits propres, jamais la courante.
	if _, err := deleteMerged(ctx, dir, r.MainRef, []string{"en-cours"}); err == nil {
		t.Error("« en-cours » a des commits absents de main : la suppression doit être refusée")
	}
	if _, err := deleteMerged(ctx, dir, r.MainRef, []string{"courante"}); err == nil {
		t.Error("la branche courante ne doit pas être supprimée")
	}
	if _, err := deleteMerged(ctx, dir, r.MainRef, []string{"develop"}); err == nil {
		t.Error("une branche protégée ne doit pas être supprimée")
	}
	left := strings.Fields(git(t, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads"))
	sort.Strings(left)
	if !reflect.DeepEqual(left, []string{"courante", "develop", "en-cours", "main"}) {
		t.Errorf("branches restantes inattendues : %v", left)
	}
}

// Relecture n°1 : une branche locale nommée « origin/main » ne doit jamais être
// prise pour la main du serveur.
func TestBrancheLocaleNommeeOriginMain(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "app"), base, "app")
	git(t, dir, "remote", "set-head", "origin", "-d") // pas d'origin/HEAD
	git(t, dir, "switch", "-q", "-c", "origin/main")  // branche locale piège
	commit(t, dir, "piege", "1")
	git(t, dir, "switch", "-q", "-c", "feature")
	commit(t, dir, "f", "2")
	git(t, dir, "switch", "-q", "main")

	r := scan(t, base, dir, false)
	if r.mainRef() != "refs/remotes/origin/main" {
		t.Fatalf("main de référence : refs/remotes/origin/main attendu, trouvé %q", r.mainRef())
	}
	if len(r.MergedBranches) != 0 {
		t.Fatalf("aucune branche n'est fusionnée dans la vraie origin/main : %v", r.MergedBranches)
	}
	if _, err := deleteMerged(context.Background(), dir, r.mainRef(), []string{"feature", "origin/main"}); err == nil {
		t.Fatal("les deux suppressions doivent être refusées")
	}
	if n := git(t, dir, "for-each-ref", "--count=9", "--format=x", "refs/heads"); strings.Count(n, "x") != 3 {
		t.Errorf("les trois branches doivent être intactes")
	}
}

// Relecture n°7 : une branche qui porte le même nom qu'un tag.
func TestBrancheHomonymeDUnTag(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "app"), base, "app")
	git(t, dir, "branch", "old")
	git(t, dir, "tag", "old")
	git(t, dir, "branch", "autre")

	r := scan(t, base, dir, true)
	if !contains(r.MergedBranches, "old") {
		t.Fatalf("« old » (et non « heads/old ») attendue : %v", r.MergedBranches)
	}
	for _, b := range r.Branches {
		if strings.HasPrefix(b.Name, "heads/") {
			t.Errorf("nom de branche mal lu : %q", b.Name)
		}
	}
	done, err := deleteMerged(context.Background(), dir, r.mainRef(), []string{"old", "autre"})
	if err != nil || len(done) != 2 {
		t.Fatalf("les deux branches doivent être supprimées : %v %v", done, err)
	}
	if git(t, dir, "tag", "-l", "old") != "old" {
		t.Error("le tag du même nom ne doit pas être touché")
	}
}

func TestBrancheOuverteDansUnWorktree(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "app"), base, "app")
	git(t, dir, "worktree", "add", "-q", filepath.Join(base, "wt"), "-b", "ailleurs")

	r := scan(t, base, dir, false)
	if contains(r.MergedBranches, "ailleurs") {
		t.Error("une branche ouverte dans un worktree ne doit pas être proposée")
	}
	if _, err := deleteMerged(context.Background(), dir, r.mainRef(), []string{"ailleurs"}); err == nil {
		t.Error("sa suppression doit être refusée")
	}
}

// Seconde relecture : une branche en cours de rebase (HEAD détachée) ne doit
// être ni proposée ni supprimée.
func TestBrancheEnCoursDeRebase(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "app"), base, "app")
	git(t, dir, "switch", "-q", "-c", "feat")
	commit(t, dir, "f", "1")
	git(t, dir, "switch", "-q", "main")
	git(t, dir, "merge", "-q", "--ff-only", "feat")
	git(t, dir, "push", "-q")
	git(t, dir, "switch", "-q", "feat")
	cmd := exec.Command("git", "-C", dir, "rebase", "-i", "HEAD~1")
	cmd.Env = append(os.Environ(), "GIT_SEQUENCE_EDITOR=sed -i s/^pick/edit/")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("rebase : %v %s", err, out)
	}

	r := scan(t, base, dir, false)
	if contains(r.MergedBranches, "feat") {
		t.Error("une branche en cours de rebase ne doit pas être proposée")
	}
	if _, err := deleteMerged(context.Background(), dir, r.mainRef(), []string{"feat"}); err == nil {
		t.Error("sa suppression doit être refusée")
	}
	git(t, dir, "rev-parse", "--verify", "refs/heads/feat")
}
