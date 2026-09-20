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

	h("LES COLONNES")
	row("DÉPÔT", "chemin du dépôt, relatif au dossier scanné (« . » = le dossier lui-même)")
	row("BRANCHE", "branche sur laquelle tu es actuellement")
	row("  @v1.4.2", "pas de branche (HEAD détachée) : tag ou commit sur lequel tu es")
	row("REMOTE", "ta branche comparée à sa branche distante (son « upstream »)")
	row("VS MAIN", "ta branche comparée à main (origin/main de préférence)")
	row("ÉTAT", "résumé de ce qui demande ton attention (voir plus bas)")

	h("LES FLÈCHES")
	row("↑3", "3 commits chez toi, absents de l'autre côté  → à pousser")
	row("↓90", "90 commits de l'autre côté, absents chez toi → à récupérer")
	row("↑3 ↓90", "les deux à la fois : les historiques ont divergé")
	row("=", "identique, rien à faire")
	row("—", "pas de comparaison possible (pas d'upstream, pas de main…)")
	row("supprimé", "la branche distante n'existe plus")

	h("LA COLONNE ÉTAT")
	row("✓ propre", "tout est à jour et commité")
	row("à pousser ↑n", "n commits à envoyer         → git push")
	row("à tirer ↓n", "n commits à récupérer       → git pull")
	row("divergé ↑a ↓b", "commits des deux côtés      → git pull --rebase (ou merge)")
	row("modifié n", "n fichiers suivis modifiés, pas encore commités")
	row("non suivi n", "n nouveaux fichiers jamais ajoutés à git")
	row("conflits n", "merge ou rebase en cours, n fichiers en conflit")
	row("jamais poussée", "la branche n'existe pas sur le serveur → git push -u origin HEAD")
	row("upstream supprimé", "branche distante effacée (souvent : PR fusionnée)")
	row("non poussée(s) : x, y", "autres branches locales avec des commits absents du serveur")
	row("n branches non poussées", "idem, quand il y en a plus de 3 (liste complète : gitscan -b)")
	row("HEAD détachée", "tu n'es sur aucune branche (ex. un vieux commit)")
	row("sur le tag v1.4.2", "HEAD détachée sur une version taguée : normal pour un déploiement")
	row("retard main ↓n", "main a reçu n commits depuis que ta branche en est partie")
	row("stash n", "n modifications mises de côté avec git stash")
	row("fetch il y a …", "dernier fetch ancien : les chiffres distants sont peut-être périmés")
	row("fetch échoué", "impossible de joindre le serveur (réseau, clé SSH…)")

	h("LES COULEURS")
	fmt.Fprintf(w, "  %srouge%s = problème   %sjaune%s = action à faire   %sgris%s = pour info   %svert%s = tout va bien\n",
		p.red, p.reset, p.yellow, p.reset, p.dim, p.reset, p.green, p.reset)

	h("IMPORTANT")
	note("Les chiffres REMOTE et VS MAIN viennent du dernier « git fetch ».")
	note("Sans -f, ils peuvent être en retard sur la réalité du serveur :")
	fmt.Fprintf(w, "  %sgitscan -f <dossier>%s\n", p.bold, p.reset)

	h("POUR ALLER PLUS LOIN")
	fmt.Fprintln(w, "  gitscan -b <dossier>   détail de toutes les branches")
	fmt.Fprintln(w, "  gitscan -c <dossier>   configuration (remotes, auteur, hooks)")
	fmt.Fprintln(w, "  gitscan -i <dossier>   mode interactif (touche ? pour l'aide)")
	fmt.Fprintln(w, "  gitscan -h             toutes les options")
}
