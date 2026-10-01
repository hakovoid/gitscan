package main

// Touche c : ce qui a changé depuis le scan précédent de ce dossier.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// compareWithPrevious : appelé une fois le premier scan terminé. Compare avec
// l'instantané enregistré, puis enregistre le scan courant à sa place.
func (m *model) compareWithPrevious() {
	if m.snapLoaded {
		return
	}
	m.snapLoaded = true
	prev, err := loadSnapshot(m.snapFile)
	if err != nil {
		m.setMsg(true, "%v", err)
	}
	m.prevSnap = prev
	m.changes = diffSnapshots(prev, takeSnapshot(m.root, m.repos))
	m.saveSnapshot()
	if prev == nil || m.msgErr {
		return
	}
	if len(m.changes) == 0 {
		m.setMsg(false, "Aucun changement depuis le dernier scan (%s).", sinceText(prev.Time))
	} else {
		m.setMsg(false, "%s depuis le dernier scan (%s) · c pour voir",
			plur(len(m.changes), "dépôt a changé", "dépôts ont changé"), sinceText(prev.Time))
	}
}

// saveSnapshot enregistre l'état affiché, pour la prochaine comparaison.
func (m *model) saveSnapshot() {
	if !m.snapLoaded || m.scanning {
		return
	}
	_ = saveSnapshot(m.snapFile, takeSnapshot(m.root, m.repos))
}

func (m *model) openChanges() {
	switch {
	case !m.snapLoaded:
		m.setMsg(false, "Analyse en cours…")
		return
	case m.prevSnap == nil:
		m.setMsg(false, "Premier scan de ce dossier : la prochaine fois, c montrera ce qui a changé.")
		return
	}
	var b strings.Builder
	m.infoVP.Width = max(20, m.width-8)
	renderChanges(&b, m.changes, m.prevSnap.Time, newPalette(m.colors), m.infoVP.Width)
	content := strings.TrimRight(b.String(), "\n")
	m.infoVP.Height = max(1, min(lipgloss.Height(content), m.height-6))
	m.infoVP.SetContent(content)
	m.infoVP.GotoTop()
	m.infoBack = m.mode
	m.mode = modeChanges
}

func (m *model) keyChanges(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q", "c", "enter":
		m.mode = m.infoBack
		return nil
	case "?":
		m.openHelp()
		return nil
	}
	var cmd tea.Cmd
	m.infoVP, cmd = m.infoVP.Update(msg)
	return cmd
}

func (m *model) viewChanges() string {
	footer := stDim.Render("↑↓ défiler · c ou échap pour fermer")
	if m.infoVP.TotalLineCount() > m.infoVP.Height {
		footer += stDim.Render(fmt.Sprintf("   %d%%", int(m.infoVP.ScrollPercent()*100)))
	}
	box := stBox.Render(m.infoVP.View() + "\n\n" + footer)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
