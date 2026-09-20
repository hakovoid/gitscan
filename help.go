package main

import (
	"fmt"
	"io"
)

// printLegend affiche `gitscan help` : comment lire le tableau.
func printLegend(w io.Writer, p palette) {
	h := func(s string) { fmt.Fprintf(w, "\n%s%s%s\n", p.bold+p.cyan, s, p.reset) }
	row := func(k, v string) {
		if len([]rune(k)) > 18 { // libellé trop long : description à la ligne
			fmt.Fprintf(w, "  %s%s%s\n  %-18s %s\n", p.bold, k, p.reset, "", v)
			return
		}
		fmt.Fprintf(w, "  %s%-18s%s %s\n", p.bold, k, p.reset, v)
	}
	note := func(s string) { fmt.Fprintf(w, "  %s%s%s\n", p.dim, s, p.reset) }

	fmt.Fprintf(w, "%sgitscan%s : comment lire le tableau\n", p.bold, p.reset)

	h("L'ICÔNE EN DÉBUT DE LIGNE")
	row("✓ en ordre", "rien à faire (au plus des infos en gris)")
	row("● à traiter", "une action est à prévoir : pousser, tirer, commiter…")
	row("✗ à risque", "du travail peut se perdre ou une opération est bloquée")

	h("LES COLONNES")
	row("DÉPÔT", "chemin du dépôt ; ├─ └─ = dépôt imbriqué (sous-module) sous son parent")
	row("BRANCHE", "branche actuelle, ou @commit / @tag si aucune branche (HEAD détachée)")
	row("SERVEUR", "la branche comparée à sa copie sur le serveur (son « upstream »)")
	row("MAIN", "la branche comparée à main")
	row("LOCAL", "ce qui n'est pas commité : fichiers modifiés, nouveaux, stash…")
	row("À VOIR", "les autres points d'attention (voir plus bas)")
	note("Terminal étroit : À VOIR passe sur une ligne « ↳ » sous le dépôt.")

	h("LES FLÈCHES (SERVEUR, MAIN)")
	row("↑3", "3 commits chez toi, absents de l'autre côté  → à pousser")
	row("↓90", "90 commits de l'autre côté, absents chez toi → à récupérer")
	row("↑3 ↓90", "les deux (en rouge dans SERVEUR) : historiques divergents")
	row("=", "identique, rien à faire")
	row("—", "pas de comparaison possible (pas de branche, pas de main…)")

	h("COLONNE SERVEUR : AUTRES VALEURS")
	row("jamais poussée", "la branche n'existe pas sur le serveur → git push -u origin HEAD")
	row("supprimée", "la branche distante a été effacée (souvent : PR fusionnée)")
	row("aucun", "le dépôt n'a aucun serveur (remote) configuré")
	row("fetch ✗", "impossible de joindre le serveur (réseau, clé SSH…)")

	h("COLONNE LOCAL")
	row("3 modifiés", "fichiers suivis modifiés, pas encore commités")
	row("dont 2 droits", "parmi eux, seuls les droits ont changé (chmod) : contenu identique")
	row("droits modifiés", "tous ne diffèrent que par les droits → git config core.fileMode false")
	row("1 nouveau", "fichier jamais ajouté à git")
	row("1 conflit", "fichier en conflit (merge ou rebase)")
	row("2 sous-modules", "sous-modules dont le commit ne correspond plus au dépôt parent")
	row("stash 1", "modification mise de côté avec git stash")
	row("propre", "rien à commiter")

	h("COLONNE À VOIR")
	row("rebase en cours", "opération interrompue (aussi : merge, cherry-pick…) → à terminer ou annuler")
	row("1 commit hors branche", "commit fait en HEAD détachée, sur aucune branche : il peut se perdre !")
	row("", "  → git branch sauvegarde   (crée une branche qui le garde)")
	row("HEAD détachée", "aucune branche (hors sous-module et hors tag, où c'est normal)")
	row("décalé : le parent attend x", "le sous-module n'est pas sur le commit enregistré par son parent")
	row("sous-module ✓", "sous-module sur le commit attendu par son parent")
	row("non poussée(s) : a, b", "autres branches locales avec des commits absents du serveur")
	row("n branches non poussées", "idem, plus de 3 (liste complète : gitscan -b)")
	row("sur le tag v1.4.2", "(info) version taguée : normal pour un déploiement")
	row("fetch il y a …", "(info) dernier fetch ancien : SERVEUR est peut-être périmé")

	h("LES COULEURS")
	fmt.Fprintf(w, "  %srouge%s = risque   %sjaune%s = à faire   %sgris%s = info   %svert%s = bon   %scyan%s = branche   %smagenta%s = commit ou tag\n",
		p.red, p.reset, p.yellow, p.reset, p.dim, p.reset, p.green, p.reset, p.cyan, p.reset, p.magenta, p.reset)

	h("IMPORTANT")
	note("Les chiffres SERVEUR et MAIN viennent du dernier « git fetch ».")
	note("Sans -f, ils peuvent être en retard sur la réalité du serveur :")
	fmt.Fprintf(w, "  %sgitscan -f <dossier>%s\n", p.bold, p.reset)

	h("POUR ALLER PLUS LOIN")
	fmt.Fprintln(w, "  gitscan -b <dossier>   détail de toutes les branches")
	fmt.Fprintln(w, "  gitscan -c <dossier>   configuration (remotes, auteur, hooks)")
	fmt.Fprintln(w, "  gitscan -i <dossier>   mode interactif (touche ? pour l'aide)")
	fmt.Fprintln(w, "  gitscan -h             toutes les options")
}
