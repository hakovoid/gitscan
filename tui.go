package main

// Mode interactif (gitscan -i), construit avec Bubble Tea.
//
// Bubble Tea suit l'architecture Elm : un modèle (l'état), une fonction Update
// qui reçoit des messages (touches, résultats de commandes git…) et renvoie le
// nouvel état, et une fonction View qui dessine l'écran à partir de l'état.
// Les opérations longues (fetch, pull, push) sont des tea.Cmd : Bubble Tea les
// exécute dans des goroutines et renvoie leur résultat sous forme de message.

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------- Messages ----------

type pathsMsg struct {
	paths []string
	err   error
}

type opDoneMsg struct {
	path    string
	op      string // scan, fetch, pull, push
	out     string
	err     error
	repo    *Repo
	initial bool
}

type detailMsg struct {
	path   string
	repo   *Repo
	log    string
	logRef string
	files  string
}

type execDoneMsg struct {
	path string
	err  error
}

// ---------- Modèle ----------

type viewMode int

const (
	modeList viewMode = iota
	modeDetail
	modeHelp
	modeConfirm
	modeBranches
	modeInfo
	modeChanges
)

type opResult struct {
	op  string
	err error
	out string
	at  time.Time
}

type detailData struct {
	repo   *Repo
	log    string
	logRef string // branche des commits affichés
	files  string
}

type model struct {
	ctx    context.Context
	cancel context.CancelFunc

	root         string
	depth        int
	excludes     map[string]bool
	nested       bool
	opt          InspectOptions
	fetchOnStart bool
	sem          chan struct{}

	repos    []*Repo
	busy     map[string]string // AbsPath -> action en cours
	results  map[string]opResult
	selected map[string]bool

	mode          viewMode
	cursor        int
	offset        int
	width, height int

	attentionOnly bool
	sortByState   bool
	searching     bool
	query         string

	scanning            bool
	welcomed            bool // premier scan terminé : on quitte l'écran d'accueil
	scanDone, scanTotal int

	confirm *confirmation // question oui/non en cours (push, ménage, sous-modules)

	snapFile   string    // instantané du scan précédent (~/.cache/gitscan)
	prevSnap   *snapshot // nil au premier scan de ce dossier
	changes    []change  // différences avec lui, calculées après le premier scan
	snapLoaded bool

	detailPath   string
	branchCursor int             // vue branches
	branchSel    map[string]bool // vue branches : branches cochées pour suppression
	wantBranches bool            // ouvrir la vue branches dès que le détail est chargé
	detailRef    string          // branche dont on affiche les commits ("" = branche courante)
	detail       *detailData
	vp           viewport.Model

	infoVP   viewport.Model // encadré d'explication (touche i)
	infoBack viewMode       // vue à restaurer en fermant l'encadré
	infoPath string         // dépôt expliqué
	helpVP   viewport.Model // écran des raccourcis (touche ?)
	helpBack viewMode       // vue à restaurer en fermant les raccourcis
	spin     spinner.Model
	msg      string
	msgErr   bool
}

func runTUI(root string, depth int, excludes map[string]bool, nested bool, opt InspectOptions, jobs int) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &model{
		ctx: ctx, cancel: cancel,
		root: root, depth: depth, excludes: excludes, nested: nested,
		opt:          InspectOptions{MainOverride: opt.MainOverride, FetchTimeout: opt.FetchTimeout, Normal: opt.Normal},
		fetchOnStart: opt.Fetch,
		sem:          make(chan struct{}, max(1, jobs)),
		busy:         map[string]string{},
		results:      map[string]opResult{},
		selected:     map[string]bool{},
		spin:         spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(stCyan)),
		vp:           viewport.New(80, 20),
		infoVP:       viewport.New(80, 20),
		helpVP:       viewport.New(80, 20),
		snapFile:     snapshotFile(root, nested, depth, excludes),
	}
	if n := opt.Normal; n != nil && len(n.warnings) > 0 {
		m.setMsg(true, "%s dans %s : %s", plur(len(n.warnings), "règle ignorée", "règles ignorées"), normalFileName, n.warnings[0])
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.discoverCmd())
}

// ---------- Update ----------

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.vp.Width, m.vp.Height = msg.Width, max(1, msg.Height-2)
		m.infoVP.Width, m.infoVP.Height = msg.Width-4, max(1, msg.Height-4)
		if m.detail != nil {
			m.vp.SetContent(m.renderDetail())
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case pathsMsg:
		return m, m.onPaths(msg)

	case opDoneMsg:
		return m, m.onOpDone(msg)

	case subPlanMsg:
		m.onSubPlan(msg)
		return m, nil

	case normalEditedMsg:
		return m, m.onNormalEdited(msg)

	case detailMsg:
		if msg.path == m.detailPath {
			m.detail = &detailData{repo: msg.repo, log: msg.log, logRef: msg.logRef, files: msg.files}
			if m.wantBranches {
				m.wantBranches, m.mode, m.branchCursor, m.branchSel = false, modeBranches, 0, map[string]bool{}
				m.msg = ""
			}
			m.replaceRepo(msg.repo)
			m.vp.SetContent(m.renderDetail())
		}
		return m, nil

	case linkDoneMsg:
		if msg.err != nil {
			m.setMsg(true, "✗ %s : %v", msg.branch, msg.err)
		} else {
			m.setMsg(false, "✓ %s suit maintenant %s", msg.branch, msg.remote)
		}
		return m, m.loadDetailCmd(msg.path)

	case execDoneMsg:
		if msg.err != nil {
			m.setMsg(true, "commande terminée avec une erreur : %v", msg.err)
		}
		if r := m.repoByPath(msg.path); r != nil {
			cmds := []tea.Cmd{m.opCmd(r, "scan", false)}
			if m.mode == modeDetail {
				cmds = append(cmds, m.loadDetailCmd(msg.path))
			}
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.saveSnapshot()
			m.cancel()
			return m, tea.Quit
		}
		switch m.mode {
		case modeHelp:
			m.keyHelp(msg)
			return m, nil
		case modeConfirm:
			return m, m.keyConfirm(msg)
		case modeInfo:
			return m, m.keyInfo(msg)
		case modeChanges:
			return m, m.keyChanges(msg)
		case modeBranches:
			return m, m.keyBranches(msg)
		case modeDetail:
			return m, m.keyDetail(msg)
		default:
			if m.searching {
				m.keySearch(msg)
				return m, nil
			}
			return m, m.keyList(msg)
		}
	}
	return m, nil
}

func (m *model) onPaths(msg pathsMsg) tea.Cmd {
	if msg.err != nil {
		m.setMsg(true, "erreur de parcours : %v", msg.err)
		return nil
	}
	old := map[string]*Repo{}
	for _, r := range m.repos {
		old[r.AbsPath] = r
	}
	m.repos = m.repos[:0]
	var cmds []tea.Cmd
	op := "scan"
	if m.fetchOnStart {
		op = "fetch"
		m.fetchOnStart = false
	}
	for _, p := range msg.paths {
		r := old[p]
		if r == nil {
			rel, _ := filepath.Rel(m.root, p)
			r = &Repo{Path: rel, AbsPath: p}
		}
		m.repos = append(m.repos, r)
		cmds = append(cmds, m.opCmd(r, op, true))
	}
	sort.Slice(m.repos, func(i, j int) bool { return m.repos[i].Path < m.repos[j].Path })
	for p := range m.selected {
		if m.repoByPath(p) == nil {
			delete(m.selected, p)
		}
	}
	m.scanning, m.scanDone, m.scanTotal = true, 0, len(msg.paths)
	if len(msg.paths) == 0 {
		m.scanning = false
		m.welcomed = true
		m.setMsg(false, "Aucun dépôt git trouvé.")
	}
	return tea.Batch(cmds...)
}

func (m *model) onOpDone(msg opDoneMsg) tea.Cmd {
	delete(m.busy, msg.path)
	m.replaceRepo(msg.repo)
	if msg.initial {
		m.scanDone++
		if m.scanDone >= m.scanTotal {
			m.scanning = false
			m.welcomed = true
			m.compareWithPrevious()
		}
	}
	var cmds []tea.Cmd
	if msg.op != "scan" {
		m.results[msg.path] = opResult{op: msg.op, err: msg.err, out: msg.out, at: time.Now()}
		switch {
		case msg.err != nil:
			m.setMsg(true, "✗ %s %s : %v", msg.op, msg.repo.Path, msg.err)
		case msg.op == opClean:
			done := strings.Split(strings.TrimSpace(msg.out), "\n")
			if len(done) == 1 {
				m.setMsg(false, "✓ %s : branche %s supprimée", msg.repo.Path, done[0])
			} else {
				m.setMsg(false, "✓ %s : %d branches supprimées · le commit de chacune est dans le détail (entrée)",
					msg.repo.Path, len(done))
			}
		case msg.op == opSubmodules:
			m.setMsg(false, "✓ sous-modules de %s remis au commit attendu", msg.repo.Path)
			cmds = append(cmds, m.afterSubmodules(msg.path))
		case !msg.initial:
			m.setMsg(false, "✓ %s %s", msg.op, msg.repo.Path)
		}
	}
	if (m.mode == modeDetail || m.mode == modeBranches) && msg.path == m.detailPath {
		cmds = append(cmds, m.loadDetailCmd(msg.path))
	}
	return tea.Batch(cmds...)
}

// ---------- Utilitaires d'état ----------

func (m *model) setMsg(isErr bool, format string, a ...any) {
	m.msg, m.msgErr = fmt.Sprintf(format, a...), isErr
}

func (m *model) repoByPath(p string) *Repo {
	for _, r := range m.repos {
		if r.AbsPath == p {
			return r
		}
	}
	return nil
}

func (m *model) replaceRepo(nr *Repo) {
	for i, r := range m.repos {
		if r.AbsPath == nr.AbsPath {
			// Garder les branches/config déjà chargées si le nouveau scan est léger.
			if nr.Config == nil && r.Config != nil {
				c := *nr
				c.Config, c.Branches = r.Config, r.Branches
				nr = &c
			}
			m.repos[i] = nr
			return
		}
	}
}

// ---------- View ----------

func (m *model) View() string {
	if m.width == 0 {
		return ""
	}
	switch m.mode {
	case modeHelp:
		return m.viewHelp()
	case modeDetail:
		return m.viewDetail()
	case modeConfirm:
		return m.viewConfirm()
	case modeBranches:
		return m.viewBranches()
	case modeInfo:
		return m.viewInfo()
	case modeChanges:
		return m.viewChanges()
	}
	if !m.welcomed {
		return m.viewWelcome()
	}
	return m.viewList()
}
