package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// checkScreen : la vue occupe exactement l'écran, ligne par ligne.
func checkScreen(t *testing.T, name, v string, w, h int) {
	t.Helper()
	lines := strings.Split(v, "\n")
	if len(lines) != h {
		t.Errorf("%s %dx%d : %d lignes au lieu de %d", name, w, h, len(lines), h)
	}
	for i, l := range lines {
		if got := lipgloss.Width(l); got != w && !(i == len(lines)-1 && got <= w) {
			t.Errorf("%s %dx%d : ligne %d de %d colonnes : %q", name, w, h, i+1, got, l)
		}
	}
}

func TestCadreOccupeLEcran(t *testing.T) {
	gitEnv(t)
	base := t.TempDir()
	dir := clone(t, server(t, base, "app"), base, "app")
	git(t, dir, "switch", "-q", "-c", "feature/un-nom-de-branche-assez-long-pour-etre-coupe")
	git(t, dir, "branch", "vieille")
	commit(t, dir, "f", "1")

	m := listModel(t, base, scan(t, base, dir, false))
	b := branchModel(t, base, dir)
	for _, sz := range [][2]int{{140, 30}, {80, 12}, {60, 10}, {59, 20}, {100, 9}} {
		m.width, m.height = sz[0], sz[1]
		v := m.viewList()
		checkScreen(t, "liste", v, sz[0], sz[1])
		framed := sz[0] >= frameMinW && sz[1] >= frameMinH
		if strings.HasPrefix(v, "╭─") != framed {
			t.Errorf("liste %dx%d : cadre attendu = %v\n%s", sz[0], sz[1], framed, v)
		}
		if !strings.Contains(v, "? aide") {
			t.Errorf("liste %dx%d : « ? aide » doit rester visible\n%s", sz[0], sz[1], v)
		}
		b.width, b.height = sz[0], sz[1]
		checkScreen(t, "branches", b.viewBranches(), sz[0], sz[1])
	}

	m.width, m.height = 120, 20
	if v := m.viewList(); !strings.Contains(strings.Split(v, "\n")[0], "gitscan") {
		t.Errorf("le fil d'Ariane doit être dans la bordure du haut :\n%s", v)
	}
	b.width, b.height = 120, 20
	if top := strings.Split(b.viewBranches(), "\n")[0]; !strings.Contains(top, "code/app") || !strings.Contains(top, "branches") {
		t.Errorf("vue branches : fil d'Ariane gitscan › dépôt › branches attendu : %q", top)
	}

	// Vue détail : le défilement est dimensionné au redimensionnement de la fenêtre.
	m.detailPath = dir
	msg := m.loadDetailCmd(dir)().(detailMsg)
	for _, sz := range [][2]int{{120, 25}, {70, 12}, {50, 15}} {
		m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		m.Update(msg)
		m.mode = modeDetail
		v := m.viewDetail()
		checkScreen(t, "détail", v, sz[0], sz[1])
		if !strings.Contains(ansi.Strip(v), "Fichiers modifiés") {
			t.Errorf("détail %dx%d : contenu attendu\n%s", sz[0], sz[1], v)
		}
	}
}
