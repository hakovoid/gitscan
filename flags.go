package main

import (
	"fmt"
	"strings"
	"time"
)

type Level int

const (
	Info  Level = iota // pour information
	Warn               // une action est probablement nécessaire
	Error              // problème bloquant
)

func (l Level) MarshalText() ([]byte, error) {
	return []byte([...]string{"info", "warn", "error"}[l]), nil
}

// Flag est un signal sur l'état d'un dépôt (ex. « à pousser ↑2 »).
type Flag struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Level Level  `json:"level"`
}

// staleAfter : au-delà, on signale que les infos distantes sont peut-être périmées.
const staleAfter = 7 * 24 * time.Hour

func (r *Repo) add(code string, lvl Level, format string, a ...any) {
	r.Flags = append(r.Flags, Flag{Code: code, Label: fmt.Sprintf(format, a...), Level: lvl})
}

func (r *Repo) computeFlags() {
	r.Flags = []Flag{}
	if r.Error != "" {
		r.add("error", Error, "erreur : %s", r.Error)
		return
	}
	if r.FetchError != "" {
		r.add("fetch_failed", Warn, "fetch échoué")
	}
	if r.Operation != "" {
		r.add("operation", Error, "%s en cours", r.Operation)
	}
	if r.OrphanCommits > 0 {
		r.add("orphan_commits", Error, "%s hors branche", plur(r.OrphanCommits, "commit", "commits"))
	}
	if r.Conflicts > 0 {
		r.add("conflicts", Error, "conflits %d", r.Conflicts)
	}
	switch {
	case r.Changed > 0 && r.ModeOnly == r.Changed:
		// Seuls les droits ont changé : contenu identique, rien de perdu.
		r.add("mode_only", Info, "droits modifiés %d (contenu identique)", r.ModeOnly)
	case r.ModeOnly > 0:
		r.add("dirty", Warn, "modifié %d (dont %d droits seuls)", r.Changed, r.ModeOnly)
	case r.Changed > 0:
		r.add("dirty", Warn, "modifié %d", r.Changed)
	}
	if r.SubmodulesChanged > 0 {
		r.add("submodules_changed", Warn, "%d sous-module(s) décalé(s)", r.SubmodulesChanged)
	}
	if r.Submodule {
		switch {
		case r.SubExpected == "":
			r.add("submodule_new", Info, "sous-module absent du commit parent")
		case !r.SubInSync:
			r.add("submodule_drift", Warn, "décalé : le parent attend %s", r.SubExpected)
		}
	}
	if r.Untracked > 0 {
		r.add("untracked", Warn, "non suivi %d", r.Untracked)
	}
	switch {
	case r.Detached && r.Submodule:
		// Normal pour un sous-module : il est figé sur le commit choisi par le parent.
	case r.Detached && r.HeadOnTag:
		// Courant pour un déploiement : on est sur une version taguée.
		r.add("detached", Info, "sur le tag %s", r.HeadDesc)
	case r.Detached:
		r.add("detached", Warn, "HEAD détachée")
	case r.UpstreamGone:
		r.add("upstream_gone", Warn, "upstream supprimé")
	case r.Upstream == "" && r.NoRemote:
		r.add("no_remote", Info, "aucun remote")
	case r.Upstream == "":
		r.add("no_upstream", Warn, "jamais poussée")
	case r.Ahead > 0 && r.Behind > 0:
		r.add("diverged", Warn, "divergé ↑%d ↓%d", r.Ahead, r.Behind)
	case r.Ahead > 0:
		r.add("ahead", Warn, "à pousser ↑%d", r.Ahead)
	case r.Behind > 0:
		r.add("behind", Warn, "à tirer ↓%d", r.Behind)
	}
	if n := len(r.UnpushedBranches); n > 0 {
		// Jusqu'à 3 noms directement dans le tableau, sinon renvoi vers -b.
		if n <= 3 {
			r.add("unpushed_branches", Warn, "non poussée(s) : %s", strings.Join(r.UnpushedBranches, ", "))
		} else {
			r.add("unpushed_branches", Warn, "%d branches non poussées (%s, …)", n, strings.Join(r.UnpushedBranches[:2], ", "))
		}
	}
	// En retard sur main : seulement pertinent quand on n'est pas sur main.
	if r.BehindMain > 0 && r.MainRef != "" && !isMainBranch(r.Branch, r.MainRef) {
		r.add("behind_main", Info, "retard main ↓%d", r.BehindMain)
	}
	if r.Stashes > 0 {
		r.add("stash", Info, "stash %d", r.Stashes)
	}
	if r.LastFetch != nil && time.Since(*r.LastFetch) > staleAfter {
		r.add("stale_fetch", Info, "fetch il y a %s", humanAge(time.Since(*r.LastFetch)))
	}
}

func isMainBranch(branch, mainRef string) bool {
	return branch == mainRef || branch == strings.TrimPrefix(mainRef, "origin/")
}

// NeedsAttention : vrai si au moins un signal de niveau Warn ou Error.
func (r *Repo) NeedsAttention() bool {
	for _, f := range r.Flags {
		if f.Level >= Warn {
			return true
		}
	}
	return false
}

func (r *Repo) has(code string) bool {
	for _, f := range r.Flags {
		if f.Code == code {
			return true
		}
	}
	return false
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%d j", int(d.Hours()/24))
	case d < 730*24*time.Hour:
		return fmt.Sprintf("%d mois", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%d ans", int(d.Hours()/24/365))
	}
}
