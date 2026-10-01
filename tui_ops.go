package main

// Commandes exécutées en arrière-plan par la TUI : découverte des dépôts,
// actions git (fetch, pull, push…), chargement du détail, programmes externes.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const opTimeout = 2 * time.Minute

func (m *model) discoverCmd() tea.Cmd {
	root, depth, excludes, nested := m.root, m.depth, m.excludes, m.nested
	return func() tea.Msg {
		paths, err := discoverRepos(root, depth, excludes, nested)
		return pathsMsg{paths, err}
	}
}

// opCmd lance une action git sur un dépôt puis le ré-analyse. args précise
// l'action si besoin (branches à supprimer, sous-modules à remettre…).
func (m *model) opCmd(r *Repo, op string, initial bool, args ...string) tea.Cmd {
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
			out, err = doOp(octx, &snapshot, op, args)
			cancel()
		}
		fresh := inspect(ctx, root, snapshot.AbsPath, opt)
		return opDoneMsg{path: snapshot.AbsPath, op: op, out: out, err: err, repo: fresh, initial: initial}
	}
}

func doOp(ctx context.Context, r *Repo, op string, args []string) (string, error) {
	dir := r.AbsPath
	switch op {
	case opClean:
		done, err := deleteMerged(ctx, dir, r.mainRef(), args)
		return strings.Join(done, "\n"), err
	case opSubmodules:
		return updateSubmodules(ctx, dir, args)
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
		if r.UpstreamGone {
			// Souvent une merge request fusionnée : pousser recréerait la branche.
			return "", errors.New("branche distante supprimée, gitscan ne la recrée pas")
		}
		if r.Upstream == "" {
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
	ref := m.detailRef
	return func() tea.Msg {
		r := inspect(ctx, root, path, opt)
		target, label := "HEAD", r.Branch
		if ref != "" {
			target, label = ref, ref
		}
		if r.Detached && ref == "" {
			label = "@" + r.HeadSHA
		}
		log, _ := runGit(ctx, path, "log", "-n", "15", "--color=always",
			"--format=%C(yellow)%h%C(reset) %s %C(dim)· %an, %cr%C(reset)%C(auto)%d", target)
		files, _ := runGit(ctx, path, "-c", "color.status=always", "status", "--short")
		return detailMsg{path: path, repo: r, log: log, logRef: label, files: files}
	}
}

func (m *model) execIn(path string, name string, args ...string) tea.Cmd {
	c := exec.Command(name, args...)
	c.Dir = path
	return tea.ExecProcess(c, func(err error) tea.Msg { return execDoneMsg{path, err} })
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
