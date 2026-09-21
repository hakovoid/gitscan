package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMotifs(t *testing.T) {
	cas := []struct {
		motif, chemin string
		ok            bool
	}{
		{"serveur/docs", "serveur/docs", true},
		{"serveur/docs", "serveur/docs2", false},
		{"serveur/*", "serveur/docs", true},
		{"serveur/*", "serveur/site/webform", false},
		{"serveur/**", "serveur/site/webform", true},
		{"**/webform", "serveur/site/webform", true},
		{"docs", "serveur/docs", true}, // sans / : le nom, à toute profondeur
		{"doc*", "a/b/docs", true},
		{"docs", "docs-old", false},
		{".", ".", true},
		{".", "api", false},
		{"..cache", "..cache", true}, // un nom qui commence par « .. » reste un nom
	}
	for _, c := range cas {
		if got := matchPattern(c.motif, c.chemin); got != c.ok {
			t.Errorf("matchPattern(%q, %q) = %v, attendu %v", c.motif, c.chemin, got, c.ok)
		}
	}
}

func TestFichierNormal(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	write(t, filepath.Join(base, ".gitscan"), `# états attendus sur ce serveur
normal  code/docs   untracked stale_fetch   # le dossier uploads n'est pas suivi
normal  code/*      conflicts               # refusé : c'est un risque
normal  code/prod   inconnu
ranger  code/x      dirty
`)
	n, err := loadNormal(filepath.Join(base, ".gitscan"))
	if err != nil {
		t.Fatal(err)
	}
	if len(n.rules) != 1 {
		t.Errorf("une seule règle valide attendue, trouvé %d", len(n.rules))
	}
	joined := strings.Join(n.warnings, "\n")
	for _, w := range []string{":3 : conflicts ne peut pas", ":4 : signal inconnu", ":5 : mot-clé inconnu"} {
		if !strings.Contains(joined, w) {
			t.Errorf("avertissement %q attendu dans :\n%s", w, joined)
		}
	}

	// Le fichier est trouvé depuis un sous-dossier du scan.
	sub := filepath.Join(base, "code")
	write(t, filepath.Join(sub, "vide"), "")
	if got := findNormalFile(sub); got != filepath.Join(base, ".gitscan") {
		t.Errorf("fichier .gitscan du dossier parent attendu, trouvé %q", got)
	}

	docs := clone(t, server(t, base, "docs"), base, "docs")
	write(t, filepath.Join(docs, "uploads", "photo.jpg"), "x")
	write(t, filepath.Join(docs, "README"), "modifié\n")

	r := scan(t, base, docs, false)
	r.computeFlags()
	n.apply(r)
	var untracked, dirty *Flag
	for i := range r.Flags {
		switch r.Flags[i].Code {
		case "untracked":
			untracked = &r.Flags[i]
		case "dirty":
			dirty = &r.Flags[i]
		}
	}
	if untracked == nil || untracked.Normal != ".gitscan:2" || untracked.Level != Info {
		t.Errorf("untracked doit être déclaré normal par .gitscan:2 : %+v", untracked)
	}
	if dirty == nil || dirty.Normal != "" || dirty.Level != Warn {
		t.Errorf("dirty n'est pas dans la règle : il doit rester une alerte : %+v", dirty)
	}
	if !r.NeedsAttention() {
		t.Error("le fichier modifié demande toujours une action")
	}
	c := buildCells(r)
	for _, s := range c.local {
		if strings.Contains(s.text, "nouveau") && s.tone != toneInfo {
			t.Errorf("« nouveau » déclaré normal ne doit plus être en jaune")
		}
	}

	// Une fois le fichier modifié commité, plus rien ne demande d'action.
	git(t, docs, "commit", "-q", "-am", "maj")
	git(t, docs, "push", "-q")
	r = scan(t, base, docs, false)
	n.apply(r)
	if r.NeedsAttention() || buildCells(r).status != Info {
		t.Errorf("seul un signal normal reste : le dépôt doit être ✓ (signaux %v)", codes(r))
	}
	if line := suggestRule(n, base, r); line != "" {
		t.Errorf("aucune règle à suggérer quand tout est déjà normal, trouvé %q", line)
	}
}

func TestRisqueJamaisNormal(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	write(t, filepath.Join(base, ".gitscan"), "normal  **  *\n")
	n, _ := loadNormal(filepath.Join(base, ".gitscan"))
	dir := clone(t, server(t, base, "webform"), base, "webform")
	git(t, dir, "checkout", "-q", "--detach")
	commit(t, dir, "perdu", "x")

	r := scan(t, base, dir, false)
	n.apply(r)
	if buildCells(r).status != Error {
		t.Errorf("même avec « * », un commit hors branche reste à risque (signaux %v)", codes(r))
	}
	if !hasCode(r, "orphan_commits") || r.isNormal("orphan_commits") {
		t.Error("orphan_commits ne doit jamais être déclaré normal")
	}
}

func TestTagToujoursAffiche(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	write(t, filepath.Join(base, ".gitscan"), "normal  stopcom  detached\n")
	n, _ := loadNormal(filepath.Join(base, ".gitscan"))
	dir := clone(t, server(t, base, "stopcom"), base, "stopcom")
	git(t, dir, "tag", "sprint-33")
	git(t, dir, "checkout", "-q", "--detach")

	r := scan(t, base, dir, false)
	n.apply(r)
	if !strings.Contains(plainOf(buildCells(r).alerts, " · "), "tag sprint-33") {
		t.Errorf("le tag doit rester affiché même si detached est déclaré normal")
	}
}

func TestDossierQuiCommenceParDeuxPoints(t *testing.T) {
	if rel, ok := relInside("/srv", "/srv/..cache"); !ok || rel != "..cache" {
		t.Errorf("« ..cache » est bien dans /srv : %q %v", rel, ok)
	}
	if _, ok := relInside("/srv/www", "/srv/autre"); ok {
		t.Error("/srv/autre n'est pas dans /srv/www")
	}
}
