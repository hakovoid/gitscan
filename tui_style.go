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
	stTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	stDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
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
