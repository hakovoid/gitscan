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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const opTimeout = 2 * time.Minute

// ---------- Styles ----------

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
	path  string
	repo  *Repo
	log   string
	files string
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
)

type opResult struct {
	op  string
	err error
	out string
	at  time.Time
}

type detailData struct {
	repo  *Repo
	log   string
	files string
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

	confirmOp    string
	confirmPaths []string

	detailPath string
	detail     *detailData
	vp         viewport.Model

	spin   spinner.Model
	msg    string
	msgErr bool
}

func runTUI(root string, depth int, excludes map[string]bool, nested bool, opt InspectOptions, jobs int) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &model{
		ctx: ctx, cancel: cancel,
		root: root, depth: depth, excludes: excludes, nested: nested,
		opt:          InspectOptions{MainOverride: opt.MainOverride, FetchTimeout: opt.FetchTimeout},
		fetchOnStart: opt.Fetch,
		sem:          make(chan struct{}, max(1, jobs)),
		busy:         map[string]string{},
		results:      map[string]opResult{},
		selected:     map[string]bool{},
		spin:         spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(stCyan)),
		vp:           viewport.New(80, 20),
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.discoverCmd())
}

// ---------- Commandes (exécutées en arrière-plan) ----------

func (m *model) discoverCmd() tea.Cmd {
	root, depth, excludes, nested := m.root, m.depth, m.excludes, m.nested
	return func() tea.Msg {
		paths, err := discoverRepos(root, depth, excludes, nested)
		return pathsMsg{paths, err}
	}
}

// opCmd lance une action git sur un dépôt puis le ré-analyse.
func (m *model) opCmd(r *Repo, op string, initial bool) tea.Cmd {
	m.busy[r.AbsPath] = op
	ctx, root, opt, sem := m.ctx, m.root, m.opt, m.sem
	snapshot := *r
	return func() tea.Msg {
		sem <- struct{}{}
		defer func() { <-sem }()
		var out string
		var err error
		if op != "scan" {
			octx, cancel := context.WithTimeout(ctx, opTimeout)
			out, err = doOp(octx, &snapshot, op)
			cancel()
		}
		fresh := inspect(ctx, root, snapshot.AbsPath, opt)
		return opDoneMsg{path: snapshot.AbsPath, op: op, out: out, err: err, repo: fresh, initial: initial}
	}
}

func doOp(ctx context.Context, r *Repo, op string) (string, error) {
	dir := r.AbsPath
	switch op {
	case "fetch":
		return runGitCombined(ctx, dir, "fetch", "--all", "--prune")
	case "pull":
		if r.Detached {
			return "", errors.New("HEAD détachée")
		}
		if r.Upstream == "" || r.UpstreamGone {
			return "", errors.New("pas d'upstream, rien à tirer")
		}
		// --ff-only : jamais de merge implicite ; en cas de divergence, git refuse.
		return runGitCombined(ctx, dir, "pull", "--ff-only")
	case "push":
		if r.Detached {
			return "", errors.New("HEAD détachée")
		}
		if r.Upstream == "" || r.UpstreamGone {
			remote, err := pickRemote(ctx, dir)
			if err != nil {
				return "", err
			}
			return runGitCombined(ctx, dir, "push", "-u", remote, "HEAD")
		}
		return runGitCombined(ctx, dir, "push")
	}
	return "", fmt.Errorf("action inconnue : %s", op)
}

func pickRemote(ctx context.Context, dir string) (string, error) {
	out, err := runGit(ctx, dir, "remote")
	if err != nil {
		return "", err
	}
	remotes := strings.Fields(out)
	if len(remotes) == 0 {
		return "", errors.New("aucun remote configuré")
	}
	for _, r := range remotes {
		if r == "origin" {
			return r, nil
		}
	}
	return remotes[0], nil
}

func (m *model) loadDetailCmd(path string) tea.Cmd {
	ctx, root := m.ctx, m.root
	opt := m.opt
	opt.AllBranches, opt.WithConfig = true, true
	return func() tea.Msg {
		r := inspect(ctx, root, path, opt)
		log, _ := runGit(ctx, path, "log", "-n", "15", "--color=always",
			"--format=%C(yellow)%h%C(reset) %s %C(dim)· %an, %cr%C(reset)%C(auto)%d")
		files, _ := runGit(ctx, path, "-c", "color.status=always", "status", "--short")
		return detailMsg{path: path, repo: r, log: log, files: files}
	}
}

func (m *model) execIn(path string, name string, args ...string) tea.Cmd {
	c := exec.Command(name, args...)
	c.Dir = path
	return tea.ExecProcess(c, func(err error) tea.Msg { return execDoneMsg{path, err} })
}

// ---------- Update ----------

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.vp.Width, m.vp.Height = msg.Width, max(1, msg.Height-2)
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

	case detailMsg:
		if msg.path == m.detailPath {
			m.detail = &detailData{repo: msg.repo, log: msg.log, files: msg.files}
			m.replaceRepo(msg.repo)
			m.vp.SetContent(m.renderDetail())
		}
		return m, nil

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
			m.cancel()
			return m, tea.Quit
		}
		switch m.mode {
		case modeHelp:
			m.mode = modeList
			return m, nil
		case modeConfirm:
			return m, m.keyConfirm(msg)
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
		}
	}
	if msg.op != "scan" {
		m.results[msg.path] = opResult{op: msg.op, err: msg.err, out: msg.out, at: time.Now()}
		if msg.err != nil {
			m.setMsg(true, "✗ %s %s : %v", msg.op, msg.repo.Path, msg.err)
		} else if !msg.initial {
			m.setMsg(false, "✓ %s %s", msg.op, msg.repo.Path)
		}
	}
	if m.mode == modeDetail && msg.path == m.detailPath {
		return m.loadDetailCmd(msg.path)
	}
	return nil
}

func (m *model) keyList(msg tea.KeyMsg) tea.Cmd {
	vis := m.visible()
	switch msg.String() {
	case "q":
		m.cancel()
		return tea.Quit
	case "up", "k":
		m.cursor--
	case "down", "j":
		m.cursor++
	case "pgup":
		m.cursor -= m.rowsHeight()
	case "pgdown":
		m.cursor += m.rowsHeight()
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(vis) - 1
	case " ", "x":
		if r := m.current(); r != nil {
			if m.selected[r.AbsPath] {
				delete(m.selected, r.AbsPath)
			} else {
				m.selected[r.AbsPath] = true
			}
			m.cursor++
		}
	case "a":
		all := len(vis) > 0
		for _, r := range vis {
			if !m.selected[r.AbsPath] {
				all = false
				break
			}
		}
		for _, r := range vis {
			if all {
				delete(m.selected, r.AbsPath)
			} else {
				m.selected[r.AbsPath] = true
			}
		}
	case "esc":
		switch {
		case m.query != "":
			m.query = ""
		case len(m.selected) > 0:
			m.selected = map[string]bool{}
		case m.attentionOnly:
			m.attentionOnly = false
		}
	case "t":
		m.attentionOnly = !m.attentionOnly
		m.cursor = 0
	case "o":
		m.sortByState = !m.sortByState
	case "/":
		m.searching = true
	case "?":
		m.mode = modeHelp
	case "enter":
		if r := m.current(); r != nil {
			m.mode = modeDetail
			m.detailPath = r.AbsPath
			m.detail = nil
			m.vp.SetContent(stDim.Render("  Chargement…"))
			m.vp.GotoTop()
			return m.loadDetailCmd(r.AbsPath)
		}
	case "f":
		return m.startOp("fetch", m.targets())
	case "p":
		return m.startOp("pull", m.targets())
	case "P":
		m.askPush(m.targets())
	case "r":
		return m.startOp("scan", m.targets())
	case "R":
		m.setMsg(false, "Nouveau scan…")
		return m.discoverCmd()
	case "s":
		if r := m.current(); r != nil {
			return m.execIn(r.AbsPath, shell())
		}
	case "l":
		if r := m.current(); r != nil {
			return m.lazygit(r.AbsPath)
		}
	}
	m.clampCursor()
	return nil
}

func (m *model) keyDetail(msg tea.KeyMsg) tea.Cmd {
	r := m.repoByPath(m.detailPath)
	switch msg.String() {
	case "esc", "q", "backspace", "left", "h":
		m.mode = modeList
		m.detail = nil
		return nil
	case "f", "p", "r":
		if r != nil {
			op := map[string]string{"f": "fetch", "p": "pull", "r": "scan"}[msg.String()]
			return m.startOp(op, []*Repo{r})
		}
	case "P":
		if r != nil {
			m.askPush([]*Repo{r})
		}
		return nil
	case "s":
		return m.execIn(m.detailPath, shell())
	case "l":
		return m.lazygit(m.detailPath)
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return cmd
}

func (m *model) keySearch(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyEsc:
		m.searching, m.query = false, ""
	case tea.KeyEnter:
		m.searching = false
	case tea.KeyBackspace:
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
		}
	case tea.KeyRunes, tea.KeySpace:
		m.query += string(msg.Runes)
	}
	m.cursor = 0
	m.clampCursor()
}

func (m *model) keyConfirm(msg tea.KeyMsg) tea.Cmd {
	back := modeList
	if m.detailPath != "" && m.detail != nil {
		back = modeDetail
	}
	switch msg.String() {
	case "o", "y", "enter":
		m.mode = back
		var rs []*Repo
		for _, p := range m.confirmPaths {
			if r := m.repoByPath(p); r != nil {
				rs = append(rs, r)
			}
		}
		return m.startOp(m.confirmOp, rs)
	case "n", "esc", "q":
		m.mode = back
		m.setMsg(false, "Push annulé.")
	}
	return nil
}

func (m *model) startOp(op string, rs []*Repo) tea.Cmd {
	var cmds []tea.Cmd
	for _, r := range rs {
		if _, busy := m.busy[r.AbsPath]; busy {
			continue
		}
		cmds = append(cmds, m.opCmd(r, op, false))
	}
	if len(cmds) == 0 {
		return nil
	}
	if len(cmds) > 1 {
		m.setMsg(false, "%s sur %d dépôts…", op, len(cmds))
	}
	return tea.Batch(cmds...)
}

// askPush ouvre la confirmation, en ne gardant que les dépôts qui ont quelque chose à pousser.
func (m *model) askPush(rs []*Repo) {
	m.confirmPaths = nil
	for _, r := range rs {
		if !r.Detached && r.Error == "" && (r.Ahead > 0 || r.Upstream == "" || r.UpstreamGone) {
			m.confirmPaths = append(m.confirmPaths, r.AbsPath)
		}
	}
	if len(m.confirmPaths) == 0 {
		m.setMsg(false, "Rien à pousser dans la sélection.")
		return
	}
	m.confirmOp = "push"
	m.mode = modeConfirm
}

func (m *model) lazygit(path string) tea.Cmd {
	if _, err := exec.LookPath("lazygit"); err != nil {
		m.setMsg(true, "lazygit n'est pas installé")
		return nil
	}
	return m.execIn(path, "lazygit")
}

func shell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "sh"
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

func severity(r *Repo) int {
	s := 0
	for _, f := range r.Flags {
		s = max(s, int(f.Level)*10+len(r.Flags))
	}
	return s
}

func (m *model) visible() []*Repo {
	q := strings.ToLower(m.query)
	var out []*Repo
	for _, r := range m.repos {
		if m.attentionOnly && r.Branch != "" && !r.NeedsAttention() {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(r.Path+" "+r.Branch), q) {
			continue
		}
		out = append(out, r)
	}
	if m.sortByState {
		sort.SliceStable(out, func(i, j int) bool { return severity(out[i]) > severity(out[j]) })
	}
	return out
}

func (m *model) current() *Repo {
	vis := m.visible()
	if m.cursor >= 0 && m.cursor < len(vis) {
		return vis[m.cursor]
	}
	return nil
}

// targets : la sélection, ou à défaut le dépôt sous le curseur.
func (m *model) targets() []*Repo {
	var rs []*Repo
	for _, r := range m.repos {
		if m.selected[r.AbsPath] {
			rs = append(rs, r)
		}
	}
	if len(rs) == 0 {
		if r := m.current(); r != nil {
			rs = append(rs, r)
		}
	}
	return rs
}

func (m *model) rowsHeight() int { return max(1, m.height-4) }

func (m *model) clampCursor() {
	n := len(m.visible())
	m.cursor = max(0, min(m.cursor, n-1))
	h := m.rowsHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	m.offset = max(0, min(m.offset, max(0, n-h)))
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
	}
	if !m.welcomed {
		return m.viewWelcome()
	}
	return m.viewList()
}

// viewWelcome : écran d'accueil affiché pendant le premier scan.
func (m *model) viewWelcome() string {
	status := m.spin.View() + " Recherche des dépôts…"
	if m.scanTotal > 0 {
		status = fmt.Sprintf("%s Analyse des dépôts  %d / %d", m.spin.View(), m.scanDone, m.scanTotal)
		const barW = 30
		filled := barW * m.scanDone / m.scanTotal
		status += "\n\n" + stCyan.Render(strings.Repeat("━", filled)) + stDim.Render(strings.Repeat("━", barW-filled))
	}
	content := lipgloss.JoinVertical(lipgloss.Center,
		renderBanner(m.width),
		"",
		stDim.Render(tagline),
		"",
		"",
		stCyan.Render(status),
		"",
		stDim.Render(shortPath(m.root)),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-lipgloss.Width(s)))
}

func (m *model) summary() string {
	var push, pull, dirty, errs, clean int
	for _, r := range m.repos {
		if r.Branch == "" && r.Error == "" {
			continue
		}
		if r.has("ahead") || r.has("diverged") || r.has("no_upstream") || r.has("unpushed_branches") {
			push++
		}
		if r.has("behind") || r.has("diverged") {
			pull++
		}
		if r.has("dirty") || r.has("untracked") || r.has("conflicts") {
			dirty++
		}
		if r.has("error") || r.has("fetch_failed") {
			errs++
		}
		if !r.NeedsAttention() {
			clean++
		}
	}
	parts := []string{stBold.Render(fmt.Sprintf("%d %s", len(m.repos), plural(len(m.repos), "dépôt")))}
	add := func(n int, label string, st lipgloss.Style) {
		if n > 0 {
			parts = append(parts, st.Render(fmt.Sprintf("%d %s", n, label)))
		}
	}
	add(push, "à pousser", stYellow)
	add(pull, "à tirer", stYellow)
	add(dirty, "modifié(s)", stYellow)
	add(errs, "en erreur", stRed)
	add(clean, plural(clean, "propre"), stGreen)
	return strings.Join(parts, stDim.Render(" · "))
}

func remoteCell(r *Repo) string {
	switch {
	case r.Branch == "" || r.Error != "":
		return stDim.Render("—")
	case r.UpstreamGone:
		return stYellow.Render("supprimé")
	case r.Upstream == "":
		return stDim.Render("—")
	case r.Ahead > 0 || r.Behind > 0:
		return stYellow.Render(counts(r.Ahead, r.Behind))
	default:
		return stGreen.Render("=")
	}
}

func mainCell(r *Repo) string {
	if r.MainRef == "" || r.Detached || r.Error != "" || r.Branch == "" {
		return stDim.Render("—")
	}
	txt := counts(r.AheadMain, r.BehindMain)
	switch {
	case txt == "=":
		return stGreen.Render(txt)
	case r.BehindMain > 0:
		return stYellow.Render(txt)
	default:
		return txt
	}
}

func (m *model) stateCell(r *Repo) string {
	if op, ok := m.busy[r.AbsPath]; ok {
		label := map[string]string{"scan": "analyse", "fetch": "fetch", "pull": "pull", "push": "push"}[op]
		return stCyan.Render(m.spin.View() + " " + label + "…")
	}
	var parts []string
	if res, ok := m.results[r.AbsPath]; ok && time.Since(res.at) < 5*time.Minute {
		if res.err != nil {
			parts = append(parts, stRed.Render("✗ "+res.op+" : "+res.err.Error()))
		} else {
			parts = append(parts, stGreen.Render("✓ "+res.op))
		}
	}
	for _, f := range r.Flags {
		parts = append(parts, levelStyle(f.Level).Render(f.Label))
	}
	if len(r.Flags) == 0 {
		parts = append(parts, stGreen.Render("✓ propre"))
	}
	return strings.Join(parts, stDim.Render(", "))
}

func (m *model) viewList() string {
	var b strings.Builder
	vis := m.visible()
	m.clampCursor()

	// En-tête
	head := stTitle.Render("gitscan") + "  " + m.summary()
	if m.scanning {
		head += "   " + stCyan.Render(fmt.Sprintf("%s analyse %d/%d", m.spin.View(), m.scanDone, m.scanTotal))
	}
	head += "   " + stDim.Render(shortPath(m.root))
	b.WriteString(fit(head, m.width) + "\n")

	// Largeurs de colonnes
	pw, bw := len("DÉPÔT"), len("BRANCHE")
	for _, r := range vis {
		pw = max(pw, lipgloss.Width(r.Path))
		bw = max(bw, lipgloss.Width(r.Branch))
	}
	pw, bw = min(pw, 40), min(bw, 26)
	const rw, mw = 8, 9
	fixed := 4 + pw + 2 + bw + 2 + rw + 2 + mw + 2
	if rest := m.width - fixed; rest < 24 {
		pw = max(10, pw-(24-rest))
		fixed = 4 + pw + 2 + bw + 2 + rw + 2 + mw + 2
	}
	sw := max(10, m.width-fixed)

	hdr := "    " + fit("DÉPÔT", pw) + "  " + fit("BRANCHE", bw) + "  " + fit("REMOTE", rw) + "  " + fit("VS MAIN", mw) + "  " + "ÉTAT"
	b.WriteString(stDim.Render(fit(hdr, m.width)) + "\n")

	h := m.rowsHeight()
	for i := m.offset; i < min(len(vis), m.offset+h); i++ {
		r := vis[i]
		cur, sel := "  ", stDim.Render("○ ")
		path := r.Path
		if m.selected[r.AbsPath] {
			sel = stMag.Render("● ")
		}
		if i == m.cursor {
			cur = stCursor.Render("❯ ")
			path = stCursor.Render(path)
		} else {
			path = stBold.Render(path)
		}
		branch := stCyan.Render(r.Branch)
		if r.Detached {
			branch = stYellow.Render("(détachée)")
		}
		line := cur + sel + fit(path, pw) + "  " + fit(branch, bw) + "  " + fit(remoteCell(r), rw) + "  " +
			fit(mainCell(r), mw) + "  " + fit(m.stateCell(r), sw)
		b.WriteString(line + "\n")
	}
	shown := min(len(vis), h)
	if len(vis) == 0 && !m.scanning {
		b.WriteString(stDim.Render("    Aucun dépôt ne correspond.") + "\n")
		shown = 1
	}
	b.WriteString(strings.Repeat("\n", max(0, h-shown)))

	// Barre d'état
	var status []string
	if m.searching {
		status = append(status, stCyan.Render("/")+m.query+stCyan.Render("█"))
	} else if m.query != "" {
		status = append(status, stCyan.Render("recherche : ")+m.query)
	}
	if m.attentionOnly {
		status = append(status, stCyan.Render("[à traiter]"))
	}
	if m.sortByState {
		status = append(status, stCyan.Render("[tri : état]"))
	}
	if n := len(m.selected); n > 0 {
		status = append(status, stMag.Render(fmt.Sprintf("%d sélectionné(s)", n)))
	}
	if len(vis) > h {
		status = append(status, stDim.Render(fmt.Sprintf("%d–%d / %d", m.offset+1, m.offset+shown, len(vis))))
	}
	if m.msg != "" {
		st := stDim
		if m.msgErr {
			st = stRed
		}
		status = append(status, st.Render(m.msg))
	}
	b.WriteString(fit(strings.Join(status, "  "), m.width) + "\n")
	b.WriteString(fit(helpLine([][2]string{
		{"espace", "sélect."}, {"a", "tout"}, {"f", "fetch"}, {"p", "pull"}, {"P", "push"},
		{"⏎", "détail"}, {"t", "à traiter"}, {"/", "chercher"}, {"?", "aide"}, {"q", "quitter"},
	}), m.width))
	return b.String()
}

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

func (m *model) renderDetail() string {
	d := m.detail
	if d == nil {
		return ""
	}
	r := d.repo
	p := newPalette(true)
	var b strings.Builder
	section := func(t string) { b.WriteString("\n" + stTitle.Render(t) + "\n") }

	b.WriteString(stDim.Render("  "+shortPath(r.AbsPath)) + "\n")
	var flags []string
	for _, f := range r.Flags {
		flags = append(flags, levelStyle(f.Level).Render(f.Label))
	}
	if len(flags) == 0 {
		flags = append(flags, stGreen.Render("✓ propre et à jour"))
	}
	b.WriteString("  " + strings.Join(flags, stDim.Render(" · ")) + "\n")

	if res, ok := m.results[r.AbsPath]; ok {
		section(fmt.Sprintf("Dernière action : %s (il y a %s)", res.op, humanAge(time.Since(res.at))))
		if res.err != nil {
			b.WriteString("  " + stRed.Render("✗ "+res.err.Error()) + "\n")
		} else {
			b.WriteString("  " + stGreen.Render("✓ réussi") + "\n")
		}
		if res.out != "" {
			b.WriteString(indent(stDim.Render(res.out), "    ") + "\n")
		}
	}

	section("Fichiers modifiés")
	if strings.TrimSpace(d.files) == "" {
		b.WriteString(stDim.Render("  aucun") + "\n")
	} else {
		b.WriteString(indent(strings.TrimRight(d.files, "\n"), "  ") + "\n")
	}

	section("Branches")
	renderBranches(&b, r, p, "  ")

	if r.Config != nil {
		section("Configuration")
		renderConfig(&b, r, p, "  ")
	}

	section("Derniers commits")
	if strings.TrimSpace(d.log) == "" {
		b.WriteString(stDim.Render("  aucun commit") + "\n")
	} else {
		b.WriteString(indent(strings.TrimRight(d.log, "\n"), "  ") + "\n")
	}
	return b.String()
}

func indent(s, pre string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = pre + lines[i]
	}
	return strings.Join(lines, "\n")
}

func (m *model) viewDetail() string {
	r := m.repoByPath(m.detailPath)
	title := stTitle.Render("‹ ")
	if r != nil {
		title += stBold.Render(r.Path) + "  " + stCyan.Render(r.Branch)
		if op, ok := m.busy[r.AbsPath]; ok {
			title += "  " + stCyan.Render(m.spin.View()+" "+op+"…")
		}
	}
	if m.msg != "" {
		st := stDim
		if m.msgErr {
			st = stRed
		}
		title += "   " + st.Render(m.msg)
	}
	footer := helpLine([][2]string{
		{"esc", "retour"}, {"f", "fetch"}, {"p", "pull"}, {"P", "push"}, {"r", "rafraîchir"},
		{"s", "shell"}, {"l", "lazygit"}, {"↑↓", "défiler"},
	})
	if pct := m.vp.ScrollPercent(); m.vp.TotalLineCount() > m.vp.Height {
		footer += stDim.Render(fmt.Sprintf("   %d%%", int(pct*100)))
	}
	return fit(title, m.width) + "\n" + m.vp.View() + "\n" + fit(footer, m.width)
}

func (m *model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(stBold.Render(fmt.Sprintf("Pousser %d dépôt(s) ?", len(m.confirmPaths))) + "\n\n")
	for i, p := range m.confirmPaths {
		if i == 12 {
			b.WriteString(stDim.Render(fmt.Sprintf("  … et %d autre(s)", len(m.confirmPaths)-12)) + "\n")
			break
		}
		r := m.repoByPath(p)
		if r == nil {
			continue
		}
		what := stYellow.Render(fmt.Sprintf("↑%d", r.Ahead))
		if r.Upstream == "" || r.UpstreamGone {
			what = stCyan.Render("nouvelle branche distante")
		}
		b.WriteString(fmt.Sprintf("  %s  %s  %s\n", stBold.Render(r.Path), stCyan.Render(r.Branch), what))
	}
	b.WriteString("\n" + stKey.Render("o") + stDim.Render(" / entrée : oui    ") + stKey.Render("n") + stDim.Render(" / échap : non"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, stBox.Render(b.String()))
}

func (m *model) viewHelp() string {
	rows := [][2]string{
		{"↑ ↓  j k", "naviguer (pgup/pgdown, g/G : début/fin)"},
		{"espace  x", "sélectionner / désélectionner"},
		{"a", "tout sélectionner (dans la vue filtrée)"},
		{"échap", "effacer la recherche, puis la sélection, puis le filtre"},
		{"", ""},
		{"f", "fetch --all --prune (sélection ou dépôt courant)"},
		{"p", "pull --ff-only (jamais de merge implicite)"},
		{"P", "push, avec confirmation (-u si la branche n'a pas d'upstream)"},
		{"r / R", "ré-analyser la sélection / re-scanner tout le dossier"},
		{"", ""},
		{"entrée", "détail : fichiers, branches, config, commits"},
		{"s", "ouvrir un shell dans le dépôt"},
		{"l", "ouvrir lazygit dans le dépôt"},
		{"", ""},
		{"t", "n'afficher que les dépôts qui demandent une action"},
		{"o", "trier par nom / par gravité"},
		{"/", "rechercher (chemin ou branche)"},
		{"q", "quitter"},
	}
	lines := func(compact bool) string {
		var b strings.Builder
		for _, r := range rows {
			if r[0] == "" {
				if !compact {
					b.WriteString("\n")
				}
				continue
			}
			b.WriteString(fmt.Sprintf("  %s  %s\n", stKey.Render(fmt.Sprintf("%-10s", r[0])), r[1]))
		}
		return strings.TrimRight(b.String(), "\n")
	}
	back := stDim.Render("Une touche pour revenir.")
	fitBox := func(st lipgloss.Style, content string) string {
		if w := lipgloss.Width(content) + st.GetHorizontalFrameSize(); w > m.width {
			st = st.Width(max(10, m.width-st.GetHorizontalBorderSize()))
		}
		return st.Render(content)
	}

	// Du plus confortable au plus compact, on garde la première version qui tient.
	candidates := []string{
		fitBox(stBox, renderBanner(m.width-8)+"\n\n"+stBold.Render("Raccourcis clavier")+"\n\n"+lines(false)+"\n\n"+back),
		fitBox(stBox, stTitle.Render("gitscan")+stDim.Render(" — raccourcis clavier")+"\n\n"+lines(false)+"\n\n"+back),
		fitBox(stBox.Padding(0, 1), stTitle.Render("gitscan")+stDim.Render(" — raccourcis · une touche pour revenir")+"\n"+lines(true)),
	}
	box := candidates[len(candidates)-1]
	for _, c := range candidates {
		if lipgloss.Height(c) <= m.height {
			box = c
			break
		}
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
