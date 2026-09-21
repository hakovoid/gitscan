package main

import (
	"context"
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
