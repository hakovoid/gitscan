package main

// Styles et mise en forme de la TUI : couleurs, icônes d'état, segments
// colorés, ajustement à la largeur.

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	stTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	// Gris des textes secondaires : le gris « 8 » des 16 couleurs est presque
	// illisible sur beaucoup de thèmes sombres. 246 (#949494) sur fond sombre
	// et 242 (#6c6c6c) sur fond clair gardent un contraste d'au moins 4,5:1.
	stDim    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "242", Dark: "246"})
	stBold   = lipgloss.NewStyle().Bold(true)
	stGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	stYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	stRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	stCyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	stMag    = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	stCursor = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	stKey    = lipgloss.NewStyle().Bold(true)
	stBox    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6")).Padding(1, 2)
)

func levelStyle(l Level) lipgloss.Style {
	switch l {
	case Error:
		return stRed
	case Warn:
		return stYellow
	default:
		return stDim
	}
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-lipgloss.Width(s)))
}

func toneStyle(t tone) lipgloss.Style {
	switch t {
	case toneOK:
		return stGreen
	case toneWarn:
		return stYellow
	case toneErr:
		return stRed
	case toneInfo:
		return stDim
	case toneAccent:
		return stCyan
	case toneTag:
		return stMag
	}
	return lipgloss.NewStyle()
}

func renderSegs(segs []seg, sep string) string {
	parts := make([]string, len(segs))
	for i, s := range segs {
		parts[i] = toneStyle(s.tone).Render(s.text)
	}
	return strings.Join(parts, stDim.Render(sep))
}

func statusRune(l Level) string {
	switch l {
	case Error:
		return "✗"
	case Warn:
		return "●"
	}
	return "✓"
}

func statusStyle(l Level) lipgloss.Style {
	switch l {
	case Error:
		return stRed
	case Warn:
		return stYellow
	}
	return stGreen
}

func statusIconTUI(l Level) string { return statusStyle(l).Render(statusRune(l)) }

func helpLine(keys [][2]string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = stKey.Render(k[0]) + " " + stDim.Render(k[1])
	}
	return strings.Join(parts, stDim.Render(" · "))
}

// helpLineFit : la ligne de touches qui tient dans width. Les touches sont
// classées de la plus utile à la moins utile : on retire les dernières
// plutôt que de couper la ligne, et la dernière (« ? aide ») reste toujours.
func helpLineFit(keys [][2]string, width int) string {
	if len(keys) == 0 {
		return ""
	}
	last := keys[len(keys)-1]
	for n := len(keys) - 1; n > 0; n-- {
		if s := helpLine(append(keys[:n:n], last)); lipgloss.Width(s) <= width {
			return s
		}
	}
	return fit(helpLine([][2]string{last}), width)
}

func shortPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func indent(s, pre string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = pre + lines[i]
	}
	return strings.Join(lines, "\n")
}

// checkbox : case à cocher, lisible sans couleur ([x] / [ ]).
func checkbox(on bool) string {
	if on {
		return stMag.Render("[x]")
	}
	return stDim.Render("[ ]")
}
