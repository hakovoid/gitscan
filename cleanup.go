package main

// Ménage des branches : repérer les branches locales dont tous les commits sont
// déjà dans main, et les supprimer sans rien perdre.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Branches de longue durée : jamais proposées au ménage, même si main les contient.
var protectedBranches = map[string]bool{
	"main": true, "master": true, "develop": true, "dev": true, "staging": true,
	"preprod": true, "prod": true, "production": true,
}

// readMerged liste les branches locales entièrement contenues dans main : leur
// suppression ne perd aucun commit. Exclues : la branche courante, la main
// locale, les branches protégées et celles ouvertes dans un autre worktree.
func (r *Repo) readMerged(ctx context.Context, dir string) {
	if r.MainRef == "" {
		return
	}
	const sep = "\x1f"
	// lstrip=2 : le nom exact de la branche, même si un tag porte le même nom
	// (refname:short donnerait alors « heads/nom »).
	out, err := runGit(ctx, dir, "for-each-ref", "--merged="+r.mainRef(),
		"--format=%(refname:lstrip=2)"+sep+"%(HEAD)"+sep+"%(worktreepath)", "refs/heads")
	if err != nil {
		return
	}
	mainLocal := strings.TrimPrefix(r.MainRef, "origin/")
	var inUse map[string]bool
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Split(line, sep)
		if len(f) != 3 || f[0] == "" {
			continue
		}
		name, current, worktree := f[0], f[1] == "*", f[2]
		if current || worktree != "" || name == mainLocal || protectedBranches[name] || "refs/heads/"+name == r.mainRef() {
			continue
		}
		if inUse == nil {
			inUse = branchesInOperation(ctx, dir)
		}
		if inUse[name] {
			continue
		}
		r.MergedBranches = append(r.MergedBranches, name)
	}
}

// branchesInOperation : branches qu'un rebase ou un bisect en cours, dans
// n'importe quel worktree, s'attend à retrouver. Pendant un rebase HEAD est
// détachée : ni %(HEAD) ni %(worktreepath) ne signalent la branche.
func branchesInOperation(ctx context.Context, dir string) map[string]bool {
	used := map[string]bool{}
	out, err := runGit(ctx, dir, "worktree", "list", "--porcelain")
	if err != nil {
		return used
	}
	for _, line := range strings.Split(out, "\n") {
		wt, ok := strings.CutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		for _, f := range []string{"rebase-merge/head-name", "rebase-apply/head-name", "BISECT_START"} {
			p, err := runGit(ctx, wt, "rev-parse", "--git-path", f)
			if err != nil {
				continue
			}
			p = strings.TrimSpace(p)
			if !filepath.IsAbs(p) {
				p = filepath.Join(wt, p)
			}
			if b, err := os.ReadFile(p); err == nil {
				name := strings.TrimPrefix(strings.TrimSpace(string(b)), "refs/heads/")
				if name != "" {
					used[name] = true
				}
			}
		}
	}
	return used
}

// deleteMerged supprime des branches locales. Pour chacune, juste avant : son
// commit est lu, gitscan vérifie qu'il est dans mainRef (référence complète),
// puis la supprime seulement si elle pointe toujours sur ce commit (un commit
// arrivé entre-temps annule la suppression). Renvoie une ligne par branche
// supprimée, avec le commit qui permet de la recréer ; les refus sont réunis
// dans l'erreur, sans arrêter les autres branches.
func deleteMerged(ctx context.Context, dir, mainRef string, names []string) ([]string, error) {
	if mainRef == "" {
		return nil, errors.New("pas de branche main de référence")
	}
	mainSHA, err := runGit(ctx, dir, "rev-parse", "--verify", "--quiet", mainRef+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("%s introuvable", mainRef)
	}
	mainSHA = strings.TrimSpace(mainSHA)
	cur, _ := runGit(ctx, dir, "symbolic-ref", "--quiet", "HEAD")
	cur = strings.TrimSpace(cur)

	inUse := branchesInOperation(ctx, dir)
	var done, refused []string
	for _, name := range names {
		if inUse[name] {
			refused = append(refused, name+" : un rebase ou un bisect en cours l'utilise")
			continue
		}
		sha, reason := deleteOne(ctx, dir, name, mainRef, mainSHA, cur)
		if reason != "" {
			refused = append(refused, name+" : "+reason)
			continue
		}
		done = append(done, fmt.Sprintf("%s (était %s)", name, sha[:min(7, len(sha))]))
	}
	if len(refused) > 0 {
		return done, errors.New(strings.Join(refused, " ; "))
	}
	return done, nil
}

// deleteOne supprime une branche et renvoie son commit, ou la raison du refus.
func deleteOne(ctx context.Context, dir, name, mainRef, mainSHA, cur string) (sha, reason string) {
	ref := "refs/heads/" + name
	switch {
	case ref == cur:
		return "", "branche courante"
	case ref == mainRef:
		return "", "c'est la branche main de référence"
	case protectedBranches[name]:
		return "", "branche protégée"
	}
	out, err := runGit(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return "", "introuvable"
	}
	sha = strings.TrimSpace(out)
	if wt, _ := runGit(ctx, dir, "for-each-ref", "--format=%(worktreepath)", ref); strings.TrimSpace(wt) != "" {
		return "", "ouverte dans un autre worktree"
	}
	if _, err := runGit(ctx, dir, "merge-base", "--is-ancestor", sha, mainSHA); err != nil {
		return "", "contient des commits absents de main"
	}
	// Suppression conditionnelle : échoue si la branche a bougé depuis la vérification.
	if _, err := runGit(ctx, dir, "update-ref", "-d", ref, sha); err != nil {
		return "", "a changé pendant l'opération, non supprimée"
	}
	// Le réglage de la branche (upstream…) n'a plus de raison d'être.
	_, _ = runGit(ctx, dir, "config", "--remove-section", "branch."+name)
	return sha, ""
}
