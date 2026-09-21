package main

// Ménage des branches : repérer les branches locales dont tous les commits sont
// déjà dans main, et les supprimer sans rien perdre.

import (
	"context"
	"fmt"
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
	out, err := runGit(ctx, dir, "for-each-ref", "--merged="+r.MainRef,
		"--format=%(refname:short)"+sep+"%(HEAD)"+sep+"%(worktreepath)", "refs/heads")
	if err != nil {
		return
	}
	mainLocal := strings.TrimPrefix(r.MainRef, "origin/")
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Split(line, sep)
		if len(f) != 3 || f[0] == "" {
			continue
		}
		name, current, worktree := f[0], f[1] == "*", f[2]
		if current || worktree != "" || name == mainLocal || protectedBranches[name] {
			continue
		}
		r.MergedBranches = append(r.MergedBranches, name)
	}
}

// deleteMerged supprime des branches locales, en revérifiant juste avant, pour
// chacune, que tous ses commits sont dans mainRef. Renvoie une ligne par branche
// supprimée, avec le commit qui permet de la recréer.
func deleteMerged(ctx context.Context, dir, mainRef string, names []string) ([]string, error) {
	if mainRef == "" {
		return nil, fmt.Errorf("pas de branche main de référence")
	}
	cur, _ := runGit(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	var done []string
	for _, name := range names {
		ref := "refs/heads/" + name
		switch {
		case name == strings.TrimSpace(cur):
			return done, fmt.Errorf("%s est la branche courante", name)
		case protectedBranches[name]:
			return done, fmt.Errorf("%s est une branche protégée", name)
		}
		if _, err := runGit(ctx, dir, "merge-base", "--is-ancestor", ref, mainRef); err != nil {
			return done, fmt.Errorf("%s contient des commits absents de %s : non supprimée", name, mainRef)
		}
		sha, err := runGit(ctx, dir, "rev-parse", "--short", ref)
		if err != nil {
			return done, err
		}
		if _, err := runGitCombined(ctx, dir, "branch", "-D", name); err != nil {
			return done, err
		}
		done = append(done, fmt.Sprintf("%s (était %s)", name, strings.TrimSpace(sha)))
	}
	return done, nil
}
