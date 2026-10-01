package main

// Cadre commun des vues plein écran (liste, détail, branches) : une bordure
// arrondie avec le fil d'Ariane et une information en haut, une ligne d'état
// séparée du contenu, et les touches dans la bordure du bas. En dessous de
// frameMinW × frameMinH, le cadre s'efface : chaque colonne compte.

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	frameMinW = 60
	frameMinH = 10
)

// Bordure discrète : le même gris que les textes secondaires.
var stBorder = stDim

// screen : le contenu d'une vue, que frame met en page.
type screen struct {
	crumbs []string    // fil d'Ariane, du plus général au plus précis
	info   string      // à droite du titre (compteurs…), déjà mis en forme
	body   []string    // lignes du contenu, sans retour à la ligne final
	status string      // ligne d'état (messages, nom complet…), peut être vide
	keys   [][2]string // touches, de la plus utile à la moins utile
	suffix string      // après les touches (pourcentage de défilement…)
}

func (m *model) framed() bool { return m.width >= frameMinW && m.height >= frameMinH }

// innerW : la largeur utile du contenu, bordures et marges (une colonne de
// chaque côté) déduites.
func (m *model) innerW() int {
	if m.framed() {
		return m.width - 4
	}
	return m.width
}

// bodyH : le nombre de lignes de contenu. Avec le cadre : bordure du haut,
// séparateur, ligne d'état, bordure du bas ; sans : titre, état, touches.
func (m *model) bodyH() int {
	if m.framed() {
		return max(1, m.height-4)
	}
	return max(1, m.height-3)
}

// crumbTitle : « gitscan › serveur/site › branches ».
func crumbTitle(crumbs []string) string {
	parts := make([]string, len(crumbs))
	for i, c := range crumbs {
		switch {
		case i == 0:
			parts[i] = stTitle.Render(c)
		case i == len(crumbs)-1:
			parts[i] = stBold.Render(c)
		default:
			parts[i] = c
		}
	}
	return strings.Join(parts, stDim.Render(" › "))
}

// frame met une vue en page sur tout l'écran.
func (m *model) frame(s screen) string {
	title := crumbTitle(s.crumbs)
	var b strings.Builder
	if !m.framed() {
		head := title
		if s.info != "" {
			head += "   " + s.info
		}
		b.WriteString(fit(head, m.width) + "\n")
		for i := 0; i < m.bodyH(); i++ {
			line := ""
			if i < len(s.body) {
				line = s.body[i]
			}
			b.WriteString(fit(line, m.width) + "\n")
		}
		b.WriteString(fit(s.status, m.width) + "\n")
		b.WriteString(helpLineFit(s.keys, m.width-lipgloss.Width(s.suffix)) + s.suffix)
		return b.String()
	}

	iw := m.width - 2
	bar := func(n int) string { return stBorder.Render(strings.Repeat("─", max(0, n))) }

	// Haut : ╭─ titre ───── info ─╮ ; l'info cède la place au titre si besoin.
	left, right := " "+title+" ", ""
	if s.info != "" {
		right = " " + s.info + " "
	}
	if lipgloss.Width(left)+lipgloss.Width(right)+4 > m.width {
		right = ""
	}
	if lipgloss.Width(left)+4 > m.width {
		left = " " + ansi.Truncate(title, m.width-6, "…") + " "
	}
	fill := m.width - 4 - lipgloss.Width(left) - lipgloss.Width(right)
	b.WriteString(stBorder.Render("╭─") + left + bar(fill) + right + stBorder.Render("─╮") + "\n")

	side := stBorder.Render("│")
	for i := 0; i < m.bodyH(); i++ {
		line := ""
		if i < len(s.body) {
			line = s.body[i]
		}
		b.WriteString(side + " " + fit(line, iw-2) + " " + side + "\n")
	}
	b.WriteString(stBorder.Render("├") + bar(iw) + stBorder.Render("┤") + "\n")
	b.WriteString(side + " " + fit(s.status, iw-2) + " " + side + "\n")

	// Bas : ╰─ touches ─────╯
	keys := " " + helpLineFit(s.keys, m.width-6-lipgloss.Width(s.suffix)) + s.suffix + " "
	b.WriteString(stBorder.Render("╰─") + keys + bar(m.width-3-lipgloss.Width(keys)) + stBorder.Render("╯"))
	return b.String()
}
