package main

// Écran des raccourcis clavier (touche ?), accessible depuis toutes les vues.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpRows : toutes les touches, regroupées par thème.
// « § » en première colonne ouvre une section.
func helpRows() [][2]string {
	return [][2]string{
		{"§", "NAVIGUER"},
		{"↑ ↓  j k", "monter / descendre   ·   pgup pgdown   ·   g G : début / fin"},
		{"échap", "revenir en arrière ; efface la recherche, la sélection, puis le filtre"},
		{"q", "quitter (ctrl+c aussi)"},
		{"§", "SÉLECTIONNER   (sans sélection, les actions visent le dépôt sous le curseur)"},
		{"espace  x", "cocher / décocher le dépôt"},
		{"a", "tout cocher, ou tout décocher, dans la vue affichée"},
		{"§", "AGIR SUR GIT"},
		{"f", "fetch --all --prune : met à jour les infos du serveur, sans toucher aux fichiers"},
		{"p", "pull --ff-only : jamais de merge implicite ; refuse si divergence"},
		{"P", "push, avec confirmation ; -u origin HEAD si la branche n'a pas d'upstream (jamais si sa distante a été supprimée)"},
		{"S", "sous-modules : les remettre au commit attendu par le parent (plan, puis confirmation)"},
		{"D", "supprimer les branches fusionnées dans main (confirmation ; rien n'est perdu)"},
		{"  espace", "(dans ces confirmations) décocher un dépôt ou un sous-module ; a : tout ; ↑ ↓ pour choisir"},
		{"r / R", "ré-analyser la sélection / re-scanner tout le dossier"},
		{"§", "VOIR PLUS"},
		{"entrée", "détail : fichiers modifiés, branches, config, 15 derniers commits"},
		{"b", "vue branches (voir plus bas)"},
		{"i", "expliquer les signaux du dépôt, avec les commandes git à lancer"},
		{"  e", "(dans i) déclarer des signaux normaux : ouvre le fichier .gitscan"},
		{"c", "ce qui a changé depuis le dernier scan"},
		{"s / l", "ouvrir un shell / lazygit dans le dépôt (exit pour revenir)"},
		{"§", "AFFICHAGE"},
		{"t", "n'afficher que les dépôts qui demandent une action"},
		{"o", "trier par nom / par gravité"},
		{"/", "rechercher (chemin ou branche) ; entrée valide, échap efface"},
		{"+ / -", "élargir / rétrécir la colonne BRANCHE (liste et vue branches) ; 0 : largeur automatique. Un nom coupé sous le curseur s'affiche en entier dans la barre d'état"},
		{"§", "DANS LA VUE BRANCHES (touche b)"},
		{"espace  x", "cocher / décocher une branche fusionnée (seules celles-là ont une case)"},
		{"a", "cocher toutes les branches fusionnées, ou tout décocher"},
		{"d", "supprimer les branches cochées, ou celle sous le curseur (confirmation)"},
		{"D", "supprimer toutes les branches fusionnées dans main"},
		{"u", "relier la branche à la branche distante de même nom (git branch -u)"},
		{"entrée", "voir les commits de cette branche"},
		{"r / échap", "rafraîchir / décocher, puis retour"},
		{"§", "PARTOUT"},
		{"?", "cet écran ; ↑ ↓ pour dérouler s'il ne tient pas"},
	}
}

// helpText : les raccourcis mis en page à la largeur voulue, avec ou sans
// lignes vides entre les sections. Les descriptions trop longues passent à la
// ligne sous elles-mêmes plutôt que d'être coupées.
func helpText(airy bool, width int) string {
	const keyCol = 12 // « %-10s » + deux espaces
	var b strings.Builder
	for i, r := range helpRows() {
		if r[0] == "§" {
			if airy && i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(stTitle.Render(wrapText(r[1], max(10, width))) + "\n")
			continue
		}
		desc := strings.Split(wrapText(r[1], max(12, width-keyCol)), "\n")
		b.WriteString(fmt.Sprintf("%s  %s\n", stKey.Render(fmt.Sprintf("%-10s", r[0])), desc[0]))
		for _, l := range desc[1:] {
			b.WriteString(strings.Repeat(" ", keyCol) + l + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// openHelp prépare l'écran des raccourcis : il défile si l'écran est trop court.
func (m *model) openHelp() {
	m.helpVP.Width = max(20, min(100, m.width-6))
	content := helpText(true, m.helpVP.Width)
	avail := m.height - 8 // bordures, marges, titre, pied de page
	if lipgloss.Height(content) > avail {
		content = helpText(false, m.helpVP.Width) // sans les lignes vides, ça tient peut-être
	}
	m.helpVP.Height = max(3, min(lipgloss.Height(content), avail))
	m.helpVP.SetContent(content)
	m.helpVP.GotoTop()
	m.helpBack = m.mode
	m.mode = modeHelp
}

// keyHelp : les touches de défilement font défiler, toutes les autres referment.
func (m *model) keyHelp(msg tea.KeyMsg) {
	scrollable := m.helpVP.TotalLineCount() > m.helpVP.Height
	if scrollable {
		switch msg.String() {
		case "up", "down", "k", "j", "pgup", "pgdown", "ctrl+u", "ctrl+d":
			m.helpVP, _ = m.helpVP.Update(msg)
			return
		case "g", "home":
			m.helpVP.GotoTop()
			return
		case "G", "end":
			m.helpVP.GotoBottom()
			return
		}
	}
	m.mode = m.helpBack
}

func (m *model) viewHelp() string {
	title := stTitle.Render("gitscan") + stDim.Render(" — raccourcis clavier")
	footer := stDim.Render("Une touche pour revenir.")
	if m.helpVP.TotalLineCount() > m.helpVP.Height {
		footer = stDim.Render(fmt.Sprintf("↑↓ dérouler (%d%%) · une autre touche pour revenir",
			int(m.helpVP.ScrollPercent()*100)))
	}
	st := stBox
	if w := m.helpVP.Width + st.GetHorizontalFrameSize(); w > m.width {
		st = st.Padding(0, 1)
	}
	box := st.Render(title + "\n\n" + m.helpVP.View() + "\n\n" + footer)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
