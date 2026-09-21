package main

// Remettre les sous-modules d'un dépôt sur le commit qu'il attend
// (git submodule update), en montrant d'abord ce que cela change.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// subPlan : ce que `git submodule update` ferait à un sous-module.
type subPlan struct {
	Path    string // relatif au dépôt parent
	Current string // commit actuel (court), vide si non initialisé
	Want    string // commit attendu par le parent (court)
	Forward int    // commits que le sous-module va gagner
	Back    int    // commits qu'il va quitter (ils restent sur leur branche)
	Missing bool   // commit attendu absent localement : il sera récupéré
	Uninit  bool   // jamais initialisé : il sera cloné
	Blocked string // raison de ne pas y toucher
}

// planSubmodules liste les sous-modules décalés de parent (tous, ou ceux de only).
func planSubmodules(ctx context.Context, parent string, only []string) ([]subPlan, error) {
	out, err := runGit(ctx, parent, "submodule", "status")
	if err != nil {
		return nil, err
	}
	var plans []subPlan
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 2 {
			continue
		}
		state := line[0]
		f := strings.Fields(line[1:])
		if len(f) < 2 || state == ' ' {
			continue
		}
		p := subPlan{Path: f[1]}
		if len(only) > 0 && !contains(only, p.Path) {
			continue
		}
		// Le commit attendu est celui de l'index du parent : c'est lui qu'utilise git submodule update.
		if ls, err := runGit(ctx, parent, "ls-files", "-s", "--", p.Path); err == nil {
			if lf := strings.Fields(ls); len(lf) >= 2 {
				p.Want = lf[1][:min(7, len(lf[1]))]
			}
		}
		switch state {
		case 'U':
			p.Blocked = "conflit sur ce sous-module dans le dépôt parent"
		case '-':
			p.Uninit = true
		case '+':
			p.Current = f[0][:min(7, len(f[0]))]
			checkSubmodule(ctx, filepath.Join(parent, p.Path), &p)
		}
		plans = append(plans, p)
	}
	return plans, nil
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
	count := func(args ...string) int {
		out, err := runGit(ctx, dir, append([]string{"rev-list", "--count"}, args...)...)
		if err != nil {
			return 0
		}
		n, _ := strconv.Atoi(strings.TrimSpace(out))
		return n
	}
	_, err := runGit(ctx, dir, "cat-file", "-e", p.Want+"^{commit}")
	p.Missing = err != nil
	// Des commits du sous-module qu'aucune branche, aucun tag ne contient
	// deviendraient introuvables une fois HEAD déplacée.
	orphan := []string{"HEAD", "--not", "--branches", "--remotes", "--tags"}
	if !p.Missing {
		orphan = append(orphan, p.Want)
		out, _ := runGit(ctx, dir, "rev-list", "--left-right", "--count", p.Want+"...HEAD")
		if lr := strings.Fields(out); len(lr) == 2 {
			p.Forward, _ = strconv.Atoi(lr[0])
			p.Back, _ = strconv.Atoi(lr[1])
		}
	}
	if n := count(orphan...); n > 0 {
		p.Blocked = fmt.Sprintf("%s sur aucune branche : %s perdu%s", plur(n, "commit", "commits"),
			map[bool]string{true: "ils seraient", false: "il serait"}[n > 1], map[bool]string{true: "s", false: ""}[n > 1])
	}
}

// updateSubmodules remet les sous-modules paths au commit attendu par parent,
// après avoir revérifié qu'aucun n'est bloqué.
func updateSubmodules(ctx context.Context, parent string, paths []string) (string, error) {
	if len(paths) == 0 {
		return "", errors.New("aucun sous-module à remettre")
	}
	plans, err := planSubmodules(ctx, parent, paths)
	if err != nil {
		return "", err
	}
	for _, p := range plans {
		if p.Blocked != "" {
			return "", fmt.Errorf("%s : %s", p.Path, p.Blocked)
		}
	}
	return runGitCombined(ctx, parent, append([]string{"submodule", "update", "--init", "--"}, paths...)...)
}

// describe : une phrase courte pour la confirmation.
func (p subPlan) describe() string {
	switch {
	case p.Blocked != "":
		return "bloqué : " + p.Blocked
	case p.Uninit:
		return "jamais initialisé : sera cloné"
	case p.Missing:
		return "commit attendu absent : sera récupéré sur le serveur"
	case p.Back > 0 && p.Forward == 0:
		return fmt.Sprintf("recule de %s (ils restent sur leur branche)", plur(p.Back, "commit", "commits"))
	case p.Back > 0:
		return fmt.Sprintf("change de ligne : +%d, −%d commits (ceux quittés restent sur leur branche)", p.Forward, p.Back)
	default:
		return fmt.Sprintf("avance de %s", plur(p.Forward, "commit", "commits"))
	}
}
