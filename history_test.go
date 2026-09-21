package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func textOf(c change) string { return plainOf(c.Items, " · ") }

func TestChangementsEntreDeuxScans(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	root := filepath.Join(base, "code")
	bare := server(t, base, "web")
	web := clone(t, bare, base, "web")
	api := clone(t, server(t, base, "api"), base, "api")
	ancien := clone(t, server(t, base, "ancien"), base, "ancien")
	collegue := clone(t, bare, base, "collegue")

	scanAll := func(paths ...string) *snapshot {
		var repos []*Repo
		for _, p := range paths {
			repos = append(repos, scan(t, root, p, false))
		}
		return takeSnapshot(root, repos)
	}

	commit(t, web, "a", "1")
	write(t, filepath.Join(api, "tmp.txt"), "x")
	avant := scanAll(web, api, ancien, collegue)

	// Entre les deux scans : web pousse et gagne 2 commits locaux, api est
	// nettoyé, ancien disparaît, un nouveau dépôt arrive.
	commit(t, web, "b", "2")
	commit(t, web, "c", "3")
	write(t, filepath.Join(api, "tmp.txt"), "")
	git(t, api, "clean", "-qfd")
	nouveau := clone(t, server(t, base, "nouveau"), base, "nouveau")
	apres := scanAll(web, api, collegue, nouveau)

	changes := diffSnapshots(avant, apres)
	got := map[string]change{}
	for _, c := range changes {
		got[c.Path] = c
	}
	if c := got["web"]; !strings.Contains(textOf(c), "à pousser ↑1 → à pousser ↑3") {
		t.Errorf("web : « ↑1 → ↑3 » attendu, trouvé %q", textOf(c))
	}
	if c := got["web"]; !strings.Contains(textOf(c), "HEAD ") {
		t.Errorf("web : le changement de commit doit apparaître, trouvé %q", textOf(c))
	}
	if c := got["api"]; !strings.Contains(textOf(c), "réglé : non suivi 1") || c.Before != Warn || c.After != Info {
		t.Errorf("api : « réglé » et ● → ✓ attendus, trouvé %q (%v → %v)", textOf(c), c.Before, c.After)
	}
	if got["ancien"].Kind != "gone" || got["nouveau"].Kind != "new" {
		t.Errorf("ancien disparu et nouveau apparu attendus : %+v", changes)
	}
	if _, ok := got["collegue"]; ok {
		t.Errorf("collegue n'a pas changé : il ne doit pas apparaître")
	}

	// Aller-retour par le disque.
	file := snapshotFile(root, false, 0, map[string]bool{"vendor": true})
	if err := saveSnapshot(file, apres); err != nil {
		t.Fatal(err)
	}
	relu, err := loadSnapshot(file)
	if err != nil || len(diffSnapshots(relu, apres)) != 0 {
		t.Errorf("un instantané relu doit être identique : %v %v", err, diffSnapshots(relu, apres))
	}
	if other := snapshotFile(root, true, 0, map[string]bool{"vendor": true}); other == file {
		t.Error("-nested change la liste des dépôts : l'instantané doit être distinct")
	}
}

func TestPremierScanSansHistorique(t *testing.T) {
	gitEnv(t)
	s, err := loadSnapshot(filepath.Join(t.TempDir(), "absent.json"))
	if s != nil || err != nil {
		t.Errorf("pas d'instantané : nil, nil attendu (%v %v)", s, err)
	}
	if diffSnapshots(nil, &snapshot{}) != nil {
		t.Error("sans scan précédent, aucun changement")
	}
}

func TestTransitionServeur(t *testing.T) {
	avant := &snapshot{Repos: map[string]snapRepo{"web": {Branch: "main", Head: "a", Status: Warn,
		Flags: map[string]snapFlag{"ahead": {Label: "à pousser ↑1", Level: Warn}}}}}
	apres := &snapshot{Repos: map[string]snapRepo{"web": {Branch: "main", Head: "a", Status: Warn,
		Flags: map[string]snapFlag{"diverged": {Label: "divergé ↑1 ↓2", Level: Warn}}}}}
	c := diffSnapshots(avant, apres)
	if len(c) != 1 || textOf(c[0]) != "à pousser ↑1 → divergé ↑1 ↓2" {
		t.Errorf("une seule transition attendue, trouvé %+v", c)
	}
}

// Relecture n°8 : « à pousser » déclaré normal, puis divergence : la
// divergence, elle, doit être signalée.
func TestTransitionDepuisUnEtatNormal(t *testing.T) {
	avant := &snapshot{Repos: map[string]snapRepo{"web": {Branch: "main", Head: "a", Status: Info,
		Flags: map[string]snapFlag{"ahead": {Label: "à pousser ↑1", Level: Info, Normal: true}}}}}
	apres := &snapshot{Repos: map[string]snapRepo{"web": {Branch: "main", Head: "a", Status: Warn,
		Flags: map[string]snapFlag{"diverged": {Label: "divergé ↑1 ↓2", Level: Warn}}}}}
	c := diffSnapshots(avant, apres)
	if len(c) != 1 || !strings.Contains(textOf(c[0]), "+ divergé ↑1 ↓2") {
		t.Errorf("la divergence doit apparaître, trouvé %+v", c)
	}
}
