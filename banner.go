package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// banner : le titre de l'application (police figlet « ANSI Shadow »).
const banner = ` ██████╗ ██╗████████╗███████╗ ██████╗ █████╗ ███╗   ██╗
██╔════╝ ██║╚══██╔══╝██╔════╝██╔════╝██╔══██╗████╗  ██║
██║  ███╗██║   ██║   ███████╗██║     ███████║██╔██╗ ██║
██║   ██║██║   ██║   ╚════██║██║     ██╔══██║██║╚██╗██║
╚██████╔╝██║   ██║   ███████║╚██████╗██║  ██║██║ ╚████║
 ╚═════╝ ╚═╝   ╚═╝   ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═══╝`

const tagline = "L'état de tous tes dépôts git, en un coup d'œil"

var bannerWidth = lipgloss.Width(strings.Split(banner, "\n")[0])

// renderBanner renvoie le titre en couleur, ou une version texte si la largeur
// disponible est insuffisante.
func renderBanner(width int) string {
	if width > 0 && width < bannerWidth+2 {
		return stTitle.Render("gitscan")
	}
	return stTitle.Render(banner)
}
