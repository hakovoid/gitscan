package main

// Remettre les sous-modules d'un dépôt sur le commit qu'il attend
// (git submodule update), en montrant d'abord ce que cela change.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// subPlan : ce que `git submodule update` ferait à un sous-module.
type subPlan struct {
	Path    string // relatif au dépôt parent
	Current string // commit actuel (complet), vide si non initialisé
	Want    string // commit attendu par le parent (complet)
	Forward int    // commits que le sous-module va gagner
	Back    int    // commits qu'il va quitter (ils restent sur leur branche)
	Uninit  bool   // jamais initialisé : il sera cloné
	Blocked string // raison de ne pas y toucher
}

func short(sha string) string { return sha[:min(7, len(sha))] }

// planSubmodules liste les sous-modules décalés de parent (tous, ou ceux de
// only). Les chemins viennent de l'index du parent (ls-files -z), qui est aussi
// ce qu'utilise git submodule update : pas de découpage sur les espaces.
func planSubmodules(ctx context.Context, parent string, only []string) ([]subPlan, error) {
	out, err := runGit(ctx, parent, "ls-files", "-s", "-z")
	if err != nil {
		return nil, err
	}
	type entry struct {
		want     string
		conflict bool
	}
	entries := map[string]*entry{}
	var order []string
	for _, rec := range strings.Split(out, "\x00") {
		meta, path, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 || f[0] != "160000" {
			continue
		}
		if len(only) > 0 && !contains(only, path) {
			continue
		}
		e := entries[path]
		if e == nil {
			e = &entry{}
			entries[path] = e
			order = append(order, path)
		}
		if f[2] != "0" {
			e.conflict = true
		} else {
			e.want = f[1]
		}
	}
	var plans []subPlan
	for _, path := range order {
		e := entries[path]
		p := subPlan{Path: path, Want: e.want}
		dir := filepath.Join(parent, path)
		switch {
		case e.conflict:
			p.Blocked = "conflit sur ce sous-module dans le dépôt parent"
		case !exists(filepath.Join(dir, ".git")):
			p.Uninit = true
		default:
			head, err := runGit(ctx, dir, "rev-parse", "--verify", "--quiet", "HEAD")
			if err != nil {
				p.Blocked = "illisible : " + err.Error()
				break
			}
			if p.Current = strings.TrimSpace(head); p.Current == p.Want {
				continue // déjà au bon commit
			}
			checkSubmodule(ctx, dir, &p)
		}
		plans = append(plans, p)
	}
	return plans, nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// checkSubmodule mesure le déplacement et vérifie que rien ne sera perdu.
func checkSubmodule(ctx context.Context, dir string, p *subPlan) {
	if st, err := runGit(ctx, dir, "status", "--porcelain", "--untracked-files=no"); err != nil {
		p.Blocked = "illisible : " + err.Error()
		return
	} else if strings.TrimSpace(st) != "" {
		p.Blocked = "modifications non commitées dans le sous-module"
		return
	}
	if _, err := runGit(ctx, dir, "cat-file", "-e", p.Want+"^{commit}"); err != nil {
		// Sans le commit, impossible de vérifier ce que la mise à jour écraserait.
		p.Blocked = "commit attendu absent : fais d'abord un fetch (f) sur ce sous-module"
		return
	}
	out, _ := runGit(ctx, dir, "rev-list", "--left-right", "--count", p.Want+"..."+p.Current)
	if lr := strings.Fields(out); len(lr) == 2 {
		p.Forward, _ = strconv.Atoi(lr[0])
		p.Back, _ = strconv.Atoi(lr[1])
	}
	// Des commits qu'aucune branche, aucun tag ne contient deviendraient introuvables.
	out, _ = runGit(ctx, dir, "rev-list", "--count", p.Current, "--not", "--branches", "--remotes", "--tags", p.Want)
	if n, _ := strconv.Atoi(strings.TrimSpace(out)); n > 0 {
		p.Blocked = fmt.Sprintf("%s sur aucune branche : %s perdu%s", plur(n, "commit", "commits"),
			map[bool]string{true: "ils seraient", false: "il serait"}[n > 1], map[bool]string{true: "s", false: ""}[n > 1])
		return
	}
	// Fichiers que le commit attendu ajoute alors qu'ils existent déjà, non
	// suivis : git les écraserait sans prévenir s'ils sont ignorés (un
	// config.php local, par exemple).
	out, _ = runGit(ctx, dir, "diff", "--name-only", "-z", "--no-renames", "--diff-filter=A", p.Current, p.Want)
	var clash []string
	for _, f := range strings.Split(out, "\x00") {
		if f == "" {
			continue
		}
		if exists(filepath.Join(dir, f)) {
			clash = append(clash, f)
			continue
		}
		// Un fichier à la place d'un dossier que le commit attendu crée
		// (fichier « cache » ignoré, commit qui ajoute « cache/keep ») serait
		// remplacé par ce dossier.
		for p := filepath.Dir(f); p != "." && p != "/"; p = filepath.Dir(p) {
			if st, err := os.Lstat(filepath.Join(dir, p)); err == nil {
				if !st.IsDir() {
					clash = append(clash, filepath.ToSlash(p))
				}
				break
			}
		}
	}
	if len(clash) > 0 {
		list := strings.Join(clash[:min(3, len(clash))], ", ")
		if len(clash) > 3 {
			list += fmt.Sprintf(" (+%d)", len(clash)-3)
		}
		p.Blocked = "fichiers non suivis qui seraient écrasés : " + list
	}
}

// updateSubmodules remet les sous-modules paths au commit attendu par parent,
// après avoir revérifié chacun : un chemin absent du plan ou bloqué annule tout.
func updateSubmodules(ctx context.Context, parent string, paths []string) (string, error) {
	if len(paths) == 0 {
		return "", errors.New("aucun sous-module à remettre")
	}
	plans, err := planSubmodules(ctx, parent, paths)
	if err != nil {
		return "", err
	}
	planned := map[string]subPlan{}
	for _, p := range plans {
		planned[p.Path] = p
	}
	for _, path := range paths {
		p, ok := planned[path]
		switch {
		case !ok:
			return "", fmt.Errorf("%s : pas un sous-module décalé, rien à faire", path)
		case p.Blocked != "":
			return "", fmt.Errorf("%s : %s", path, p.Blocked)
		}
	}
	// --checkout : toujours placer sur le commit attendu, même si le dépôt a
	// réglé submodule.<nom>.update sur rebase ou merge (qui réécriraient une
	// branche). Chemins littéraux : pas de motif * ou [ interprété.
	args := append([]string{"submodule", "update", "--init", "--checkout", "--"}, paths...)
	return runGitCombinedEnv(ctx, parent, []string{"GIT_LITERAL_PATHSPECS=1"}, args...)
}

// describe : une phrase courte pour la confirmation.
func (p subPlan) describe() string {
	switch {
	case p.Blocked != "":
		return "bloqué : " + p.Blocked
	case p.Uninit:
		return "jamais initialisé : sera cloné"
	case p.Back > 0 && p.Forward == 0:
		return fmt.Sprintf("recule de %s (ils restent sur leur branche)", plur(p.Back, "commit", "commits"))
	case p.Back > 0:
		return fmt.Sprintf("change de ligne : +%d, −%d commits (ceux quittés restent sur leur branche)", p.Forward, p.Back)
	default:
		return fmt.Sprintf("avance de %s", plur(p.Forward, "commit", "commits"))
	}
}
