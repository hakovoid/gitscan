package main

// Explication contextuelle (touche i) : ce que signale gitscan sur un dépôt,
// pourquoi, et la commande git qui règle le cas.

import (
	"fmt"
	"strings"
)

type advice struct {
	what string   // ce que ça veut dire
	cmds []string // commandes proposées (facultatif)
}

// adviceFor décrit un signal, en utilisant les valeurs réelles du dépôt.
func adviceFor(code string, r *Repo) advice {
	up := r.Upstream
	if up == "" {
		up = r.MatchRemote
	}
	switch code {
	case "error":
		return advice{"gitscan n'a pas pu lire ce dépôt. Dossier .git incomplet, ou droits insuffisants (fréquent sous /var/www, où les fichiers appartiennent à www-data).",
			[]string{"git status", "ls -la .git"}}
	case "fetch_failed":
		return advice{"Le serveur n'a pas pu être joint : réseau, ou clé SSH non chargée. Les chiffres affichés datent donc du dernier fetch réussi.",
			[]string{"ssh-add -l", "git fetch"}}
	case "operation":
		return advice{fmt.Sprintf("Un %s a été interrompu et n'a jamais été terminé. Tant qu'il dure, le dépôt est dans un état intermédiaire.", r.Operation),
			[]string{"git status", "git " + r.Operation + " --continue", "git " + r.Operation + " --abort"}}
	case "orphan_commits":
		return advice{"Des commits ont été faits alors qu'aucune branche n'était active. Aucune branche ni aucun tag ne les contient : git finira par les supprimer.",
			[]string{"git log --oneline HEAD --not --branches --remotes --tags", "git branch sauvegarde-" + r.HeadSHA}}
	case "conflicts":
		return advice{"Des fichiers sont en conflit : un merge ou un rebase attend que tu choisisses quoi garder.",
			[]string{"git status", "git diff --name-only --diff-filter=U"}}
	case "dirty":
		return advice{"Des fichiers suivis par git ont été modifiés sans être commités. Ce travail n'existe que sur cette machine.",
			[]string{"git diff --stat", "git add -A && git commit -m \"...\""}}
	case "mode_only":
		return advice{"Ces fichiers ont le même contenu qu'avant : seuls leurs droits ont changé, souvent après un chmod ou un déploiement. Tu peux dire à git d'ignorer les droits dans ce dépôt.",
			[]string{"git diff --summary | head", "git config core.fileMode false"}}
	case "untracked":
		return advice{"Des fichiers présents dans le dossier ne sont pas suivis par git : ni commités, ni ignorés.",
			[]string{"git status --short --untracked-files=all"}}
	case "submodules_changed":
		return advice{"Des sous-modules de ce dépôt ne sont pas sur le commit qu'il a enregistré. Soit ils ont été déplacés ici, soit ce dépôt est plus ancien qu'eux.",
			[]string{"git submodule status", "git submodule update --init"}}
	case "submodule_drift":
		return advice{fmt.Sprintf("Ce sous-module est sur %s, alors que son dépôt parent attend %s.", r.HeadSHA, r.SubExpected),
			[]string{"git log --oneline " + r.SubExpected + "..HEAD",
				"cd " + r.SuperProject + " && git submodule update -- " + strings.TrimPrefix(r.AbsPath, r.SuperProject+"/")}}
	case "submodule_new":
		return advice{"Ce sous-module n'apparaît pas dans le commit actuel du dépôt parent : il a été ajouté sans être commité, ou le parent est plus ancien.", nil}
	case "detached":
		if len(r.HeadTags) > 0 {
			return advice{"Aucune branche n'est active : le dépôt est figé sur une version taguée. C'est le fonctionnement normal d'un déploiement.", nil}
		}
		return advice{"Aucune branche n'est active. Tant que c'est le cas, un commit ne serait rattaché à rien.",
			[]string{"git switch main"}}
	case "no_remote":
		return advice{"Ce dépôt n'a aucun serveur déclaré : il n'existe que sur cette machine.",
			[]string{"git remote add origin <url>"}}
	case "no_upstream":
		if r.MatchRemote != "" {
			return advice{fmt.Sprintf("La branche n'est reliée à aucune branche distante, mais %s existe sur le serveur et il lui manque %d commit(s). gitscan compare avec elle.", r.MatchRemote, r.MatchAhead),
				[]string{"git branch -u " + r.MatchRemote, "git push"}}
		}
		return advice{"Cette branche n'existe pas sur le serveur : son travail n'est nulle part ailleurs que sur cette machine.",
			[]string{"git push -u origin " + r.Branch}}
	case "unlinked":
		return advice{fmt.Sprintf("La branche est déjà sur le serveur (%s, contenu identique), mais ta copie locale n'y est pas reliée : git ne peut donc pas afficher ↑↓ tout seul.", r.MatchRemote),
			[]string{"git branch -u " + r.MatchRemote}}
	case "upstream_gone":
		return advice{"La branche distante que suivait cette branche a été supprimée du serveur, en général après la fusion d'une merge request.",
			[]string{"git log --oneline origin/" + strings.TrimPrefix(r.MainRef, "origin/") + "..HEAD", "git switch main"}}
	case "upstream_other":
		return advice{fmt.Sprintf("La branche suit %s, qui porte un autre nom qu'elle. Les ↑↓ comparent donc avec cette branche-là, pas avec origin/%s.", r.Upstream, r.Branch),
			[]string{"git branch -u origin/" + r.Branch}}
	case "ahead":
		return advice{fmt.Sprintf("%d commit(s) sont chez toi et pas sur %s.", r.Ahead, up),
			[]string{"git log --oneline " + up + "..HEAD", "git push"}}
	case "behind":
		return advice{fmt.Sprintf("%d commit(s) sont sur %s et pas chez toi.", r.Behind, up),
			[]string{"git log --oneline HEAD.." + up, "git pull --ff-only"}}
	case "diverged":
		return advice{fmt.Sprintf("Chacun a avancé de son côté : %d commit(s) chez toi, %d sur %s. Un simple pull ne peut pas régler ça.", r.Ahead, r.Behind, up),
			[]string{"git log --oneline --left-right " + up + "...HEAD", "git pull --rebase"}}
	case "unpushed_branches":
		return advice{"D'autres branches locales contiennent des commits absents du serveur : " + strings.Join(r.UnpushedBranches, ", ") + ".",
			[]string{"git push -u origin " + r.UnpushedBranches[0]}}
	case "unlinked_branches":
		return advice{"D'autres branches locales sont déjà sur le serveur mais sans lien configuré : " + strings.Join(r.UnlinkedBranches, ", ") + ". La touche b ouvre la vue branches, où u les relie.", nil}
	case "behind_main":
		return advice{fmt.Sprintf("main a reçu %d commit(s) depuis que cette branche en est partie. Rien d'urgent, sauf si tu veux travailler sur une base à jour.", r.BehindMain),
			[]string{"git log --oneline HEAD.." + r.MainRef, "git rebase " + r.MainRef}}
	case "local_main_behind":
		return advice{fmt.Sprintf("Ta branche %s locale a %d commit(s) de retard sur %s. Une branche créée depuis elle partirait d'une base périmée.", r.LocalMainName, r.LocalMainBehind, r.MainRef),
			[]string{"git switch " + r.LocalMainName + " && git pull --ff-only"}}
	case "main_local":
		return advice{"Aucune branche principale distante n'a été trouvée : la comparaison « vs MAIN » se fait avec la main locale, qui peut elle-même être périmée.", nil}
	case "stash":
		return advice{fmt.Sprintf("%d modification(s) ont été mises de côté avec git stash. On les oublie facilement.", r.Stashes),
			[]string{"git stash list", "git stash show -p stash@{0}"}}
	case "stale_fetch":
		return advice{"Le dernier fetch est ancien : les colonnes vs SERVEUR et vs MAIN reposent sur des informations peut-être dépassées. La touche f met à jour.", nil}
	}
	return advice{}
}

// infoContent : le texte de l'encadré d'explication pour un dépôt.
func (m *model) infoContent(r *Repo) string {
	p := newPalette(true)
	var b strings.Builder
	b.WriteString(stTitle.Render(r.Path) + "  " + renderSegs(buildCells(r).branch, "") + "\n")
	b.WriteString(stDim.Render(truncRunes(r.AbsPath, max(20, m.infoVP.Width-2))) + "\n")

	if len(r.Flags) == 0 {
		b.WriteString("\n" + stGreen.Render("✓ Rien à signaler : tout est commité et à jour.") + "\n")
		return b.String()
	}
	for _, f := range r.Flags {
		a := adviceFor(f.Code, r)
		b.WriteString("\n" + levelStyle(f.Level).Render("■ ") + stBold.Render(f.Label) + "\n")
		if a.what != "" {
			b.WriteString(indent(wrapText(a.what, max(30, m.infoVP.Width-4)), "  ") + "\n")
		}
		for _, c := range a.cmds {
			b.WriteString("  " + stDim.Render("$ ") + stCyan.Render(c) + "\n")
		}
	}
	b.WriteString("\n" + stDim.Render("Commandes à lancer dans le dépôt : la touche s y ouvre un shell. gitscan ne les exécute pas.") + "\n")
	_ = p
	return b.String()
}

// wrapText coupe un paragraphe à la largeur voulue, sans couper les mots.
func wrapText(s string, width int) string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && len([]rune(line+" "+w)) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	return strings.Join(append(lines, line), "\n")
}
