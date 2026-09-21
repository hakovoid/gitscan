package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Repo décrit l'état complet d'un dépôt.
type Repo struct {
	Path    string `json:"path"`     // chemin relatif au dossier scanné
	AbsPath string `json:"abs_path"` // chemin absolu

	Branch       string   `json:"branch"`                   // branche courante
	Head         string   `json:"head,omitempty"`           // commit courant (court), toujours renseigné
	Detached     bool     `json:"detached,omitempty"`       // HEAD détachée
	HeadSHA      string   `json:"head_sha,omitempty"`       // si détachée : commit court
	HeadTags     []string `json:"head_tags,omitempty"`      // tags pointant exactement sur HEAD (plus récent d'abord)
	HeadDesc     string   `json:"head_desc,omitempty"`      // sinon : « sprint-33-2-g70d8a93 » (git describe)
	HeadAfterTag string   `json:"head_after_tag,omitempty"` // « 2 commits après sprint-33 »
	Upstream     string   `json:"upstream,omitempty"`       // ex. origin/feature
	UpstreamGone bool     `json:"upstream_gone,omitempty"`
	Ahead        int      `json:"ahead"`  // commits à pousser
	Behind       int      `json:"behind"` // commits à tirer

	MainRef         string `json:"main_ref,omitempty"`          // ex. origin/main
	LocalMainName   string `json:"local_main,omitempty"`        // la main locale correspondante
	LocalMainBehind int    `json:"local_main_behind,omitempty"` // son retard sur la main distante
	AheadMain       int    `json:"ahead_main"`                  // commits de la branche absents de main
	BehindMain      int    `json:"behind_main"`                 // commits de main absents de la branche

	Changed   int `json:"changed"`   // fichiers suivis modifiés (indexés ou non), comptés une fois
	ModeOnly  int `json:"mode_only"` // parmi eux : seuls les droits (chmod) ont changé
	Staged    int `json:"staged"`
	Modified  int `json:"modified"`
	Untracked int `json:"untracked"`
	Conflicts int `json:"conflicts"`
	Stashes   int `json:"stashes"`

	NoRemote          bool   `json:"no_remote,omitempty"`    // aucun remote configuré
	MatchRemote       string `json:"match_remote,omitempty"` // sans upstream : branche distante de même nom trouvée
	MatchAhead        int    `json:"match_ahead,omitempty"`  // commits absents de cette branche distante
	MatchBehind       int    `json:"match_behind,omitempty"`
	SubmodulesChanged int    `json:"submodules_changed,omitempty"` // sous-modules dont le commit a bougé
	Operation         string `json:"operation,omitempty"`          // rebase, merge, cherry-pick… en cours
	OrphanCommits     int    `json:"orphan_commits,omitempty"`     // commits sur HEAD détachée hors de toute branche

	// Sous-module : le dépôt parent attend un commit précis.
	Submodule    bool   `json:"submodule,omitempty"`
	SuperProject string `json:"superproject,omitempty"`
	SubExpected  string `json:"sub_expected,omitempty"` // commit attendu par le parent (court)
	SubInSync    bool   `json:"sub_in_sync,omitempty"`

	Branches []Branch `json:"branches,omitempty"`
	// Branches locales (hors courante) avec du travail non poussé.
	UnpushedBranches []string `json:"unpushed_branches,omitempty"`
	// Branches déjà sur le serveur, mais sans upstream configuré localement.
	UnlinkedBranches []string `json:"unlinked_branches,omitempty"`
	// Branches locales entièrement contenues dans main : supprimables sans perte.
	MergedBranches []string `json:"merged_branches,omitempty"`

	Config *Config `json:"config,omitempty"`

	LastFetch  *time.Time `json:"last_fetch,omitempty"`
	FetchError string     `json:"fetch_error,omitempty"`
	Error      string     `json:"error,omitempty"`

	Flags []Flag `json:"flags"`
}

// Branch décrit une branche locale.
type Branch struct {
	Name         string    `json:"name"`
	Current      bool      `json:"current"`
	Upstream     string    `json:"upstream,omitempty"`
	UpstreamGone bool      `json:"upstream_gone,omitempty"`
	Ahead        int       `json:"ahead"`
	Behind       int       `json:"behind"`
	AheadMain    int       `json:"ahead_main"`
	BehindMain   int       `json:"behind_main"`
	MergedInMain bool      `json:"merged_in_main"`
	MatchRemote  string    `json:"match_remote,omitempty"` // sans upstream : branche distante de même nom
	MatchAhead   int       `json:"match_ahead,omitempty"`
	MatchBehind  int       `json:"match_behind,omitempty"`
	LastCommit   time.Time `json:"last_commit"`
}

// Config regroupe la configuration utile d'un dépôt.
type Config struct {
	UserName  string   `json:"user_name,omitempty"`
	UserEmail string   `json:"user_email,omitempty"`
	Remotes   []Remote `json:"remotes,omitempty"`
	HooksPath string   `json:"hooks_path,omitempty"`
	Hooks     []string `json:"hooks,omitempty"` // hooks actifs
	Signing   bool     `json:"commit_signing"`
}

type Remote struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Options de l'inspection.
type InspectOptions struct {
	Fetch        bool
	FetchTimeout time.Duration
	MainOverride string
	AllBranches  bool
	WithConfig   bool
	Normal       *normalRules // états déclarés normaux (.gitscan), peut être nil
}

func inspect(ctx context.Context, root, path string, opt InspectOptions) *Repo {
	abs, _ := filepath.Abs(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "" {
		rel = path
	}
	r := &Repo{Path: rel, AbsPath: abs}

	if opt.Fetch {
		fctx, cancel := context.WithTimeout(ctx, opt.FetchTimeout)
		_, ferr := runGit(fctx, path, "fetch", "--all", "--prune", "--quiet")
		cancel()
		if ferr != nil {
			r.FetchError = ferr.Error()
			if fctx.Err() == context.DeadlineExceeded {
				r.FetchError = "délai dépassé"
			}
		}
	}

	if err := r.readStatus(ctx, path); err != nil {
		r.Error = err.Error()
		r.computeFlags()
		return r
	}
	r.readModeOnly(ctx, path)
	r.readStash(ctx, path)
	r.readGitDir(ctx, path)
	r.readSubmodule(ctx, path)
	remotes := remoteRefs(ctx, path)
	if r.Upstream == "" && !r.Detached {
		if len(remotes) == 0 {
			if out, err := runGit(ctx, path, "remote"); err == nil && strings.TrimSpace(out) == "" {
				r.NoRemote = true
			}
		}
		// Pas d'upstream configuré, mais une branche distante du même nom existe
		// peut-être : le travail est alors déjà sauvegardé, seul le lien manque.
		if m := matchRemote(remotes, r.Branch); m != "" {
			r.MatchRemote = m
			r.MatchBehind, r.MatchAhead = leftRight(ctx, path, m, "HEAD")
		}
	}
	r.MainRef = detectMain(ctx, path, opt.MainOverride)
	if r.Detached {
		// Commits faits en HEAD détachée qu'aucune branche, aucun tag ne contient : perdables.
		if out, err := runGit(ctx, path, "rev-list", "--count", "HEAD", "--not", "--branches", "--remotes", "--tags"); err == nil {
			r.OrphanCommits, _ = strconv.Atoi(strings.TrimSpace(out))
		}
		if out, err := runGit(ctx, path, "rev-parse", "--short", "HEAD"); err == nil {
			r.HeadSHA = strings.TrimSpace(out)
		}
		// Tous les tags posés sur ce commit, du plus récent au plus ancien : git
		// describe n'en montre qu'un, souvent le plus ancien, ce qui induit en erreur.
		if out, err := runGit(ctx, path, "tag", "--points-at", "HEAD", "--sort=-v:refname"); err == nil {
			r.HeadTags = strings.Fields(strings.TrimSpace(out))
		}
		if len(r.HeadTags) == 0 {
			if out, err := runGit(ctx, path, "describe", "--tags", "--always"); err == nil {
				r.HeadDesc = strings.TrimSpace(out)
				// « sprint-33-2-g70d8a93 » → « 2 commits après sprint-33 »
				if i := strings.LastIndex(r.HeadDesc, "-g"); i > 0 {
					if j := strings.LastIndex(r.HeadDesc[:i], "-"); j > 0 {
						n := r.HeadDesc[j+1 : i]
						r.HeadAfterTag = fmt.Sprintf("%s après %s", plur(atoi(n), "commit", "commits"), r.HeadDesc[:j])
					}
				}
			}
		}
	}
	if r.MainRef != "" {
		r.BehindMain, r.AheadMain = leftRight(ctx, path, r.MainRef, "HEAD")
	}
	r.readBranches(ctx, path, remotes, opt.AllBranches)
	r.readMerged(ctx, path)
	if opt.WithConfig {
		r.Config = readConfig(ctx, path)
	}
	r.computeFlags()
	opt.Normal.apply(r)
	return r
}

// readStatus exploite `git status --porcelain=v2 --branch`, qui donne en un seul
// appel la branche, l'upstream, l'avance/retard et les fichiers modifiés.
func (r *Repo) readStatus(ctx context.Context, dir string) error {
	out, err := runGit(ctx, dir, "status", "--porcelain=v2", "--branch", "--untracked-files=normal")
	if err != nil {
		return err
	}
	hasAB := false
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			r.Branch = strings.TrimPrefix(line, "# branch.head ")
			if r.Branch == "(detached)" {
				r.Detached = true
			}
		case strings.HasPrefix(line, "# branch.oid "):
			if oid := strings.TrimPrefix(line, "# branch.oid "); oid != "(initial)" {
				r.Head = oid[:min(7, len(oid))]
			}
		case strings.HasPrefix(line, "# branch.upstream "):
			r.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			hasAB = true
			f := strings.Fields(strings.TrimPrefix(line, "# branch.ab "))
			if len(f) == 2 {
				r.Ahead, _ = strconv.Atoi(strings.TrimPrefix(f[0], "+"))
				r.Behind, _ = strconv.Atoi(strings.TrimPrefix(f[1], "-"))
			}
		case strings.HasPrefix(line, "1 "), strings.HasPrefix(line, "2 "):
			// Format : « 1 XY sub mH mI mW hH hI chemin » ; sub commence par S pour un sous-module.
			f := strings.Fields(line)
			if len(f) >= 3 && strings.HasPrefix(f[2], "S") {
				r.SubmodulesChanged++
				continue
			}
			if len(line) >= 4 {
				r.Changed++
				if line[2] != '.' {
					r.Staged++
				}
				if line[3] != '.' {
					r.Modified++
				}
			}
		case strings.HasPrefix(line, "u "):
			r.Conflicts++
		case strings.HasPrefix(line, "? "):
			r.Untracked++
		}
	}
	// Upstream configuré mais absent (branche distante supprimée).
	if r.Upstream != "" && !hasAB {
		r.UpstreamGone = true
	}
	return nil
}

func (r *Repo) readStash(ctx context.Context, dir string) {
	out, err := runGit(ctx, dir, "stash", "list")
	if err == nil && strings.TrimSpace(out) != "" {
		r.Stashes = len(strings.Split(strings.TrimSpace(out), "\n"))
	}
}

// readGitDir regarde dans le dossier .git : date du dernier fetch et opération
// interrompue (rebase, merge…).
func (r *Repo) readGitDir(ctx context.Context, dir string) {
	out, err := runGit(ctx, dir, "rev-parse", "--absolute-git-dir", "--git-common-dir")
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	gitDir, common := lines[0], lines[0]
	if len(lines) > 1 {
		common = lines[1]
		if !filepath.IsAbs(common) {
			common = filepath.Join(dir, common)
		}
	}
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(gitDir, name))
		return err == nil
	}
	for _, d := range []string{gitDir, common} {
		if st, err := os.Stat(filepath.Join(d, "FETCH_HEAD")); err == nil {
			t := st.ModTime()
			r.LastFetch = &t
			break
		}
	}
	switch {
	case exists("rebase-merge"), exists("rebase-apply"):
		r.Operation = "rebase"
	case exists("MERGE_HEAD"):
		r.Operation = "merge"
	case exists("CHERRY_PICK_HEAD"):
		r.Operation = "cherry-pick"
	case exists("REVERT_HEAD"):
		r.Operation = "revert"
	case exists("BISECT_LOG"):
		r.Operation = "bisect"
	}
}

// readModeOnly compte les fichiers dont seuls les droits ont changé (chmod), cas
// fréquent sur un serveur web : on relance status en ignorant les droits.
func (r *Repo) readModeOnly(ctx context.Context, dir string) {
	if r.Changed == 0 {
		return
	}
	out, err := runGit(ctx, dir, "-c", "core.fileMode=false", "status", "--porcelain=v2", "--untracked-files=no")
	if err != nil {
		return
	}
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 ") {
			if f := strings.Fields(line); len(f) >= 3 && !strings.HasPrefix(f[2], "S") {
				n++
			}
		}
	}
	r.ModeOnly = max(0, r.Changed-n)
}

// readSubmodule : si le dépôt est un sous-module, compare son commit à celui
// qu'attend le dépôt parent.
func (r *Repo) readSubmodule(ctx context.Context, dir string) {
	if st, err := os.Stat(filepath.Join(dir, ".git")); err != nil || st.IsDir() {
		return // un sous-module a un fichier .git, pas un dossier
	}
	out, err := runGit(ctx, dir, "rev-parse", "--show-superproject-working-tree")
	parent := strings.TrimSpace(out)
	if err != nil || parent == "" {
		return
	}
	r.Submodule, r.SuperProject = true, parent
	abs, _ := filepath.Abs(dir)
	rel, err := filepath.Rel(parent, abs)
	if err != nil {
		return
	}
	// « 160000 commit <sha>\t<chemin> »
	out, err = runGit(ctx, parent, "ls-tree", "HEAD", "--", rel)
	f := strings.Fields(out)
	if err != nil || len(f) < 3 {
		return
	}
	expected := f[2]
	head, err := runGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return
	}
	r.SubExpected = expected[:min(7, len(expected))]
	r.SubInSync = strings.TrimSpace(head) == expected
}

// detectMain trouve la branche de référence : de préférence la version distante
// (origin/main), car c'est elle qui dit si l'on est vraiment « à jour avec main ».
func detectMain(ctx context.Context, dir, override string) string {
	var candidates []string
	if override != "" {
		candidates = []string{"origin/" + override, override}
	} else {
		if out, err := runGit(ctx, dir, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
			candidates = append(candidates, strings.TrimSpace(out))
		}
		candidates = append(candidates, "origin/main", "origin/master", "main", "master")
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := runGit(ctx, dir, "rev-parse", "--verify", "--quiet", c+"^{commit}"); err == nil {
			return c
		}
	}
	return ""
}

// leftRight renvoie (commits dans a absents de b, commits dans b absents de a).
func leftRight(ctx context.Context, dir, a, b string) (int, int) {
	out, err := runGit(ctx, dir, "rev-list", "--left-right", "--count", a+"..."+b)
	if err != nil {
		return 0, 0
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return 0, 0
	}
	l, _ := strconv.Atoi(f[0])
	rr, _ := strconv.Atoi(f[1])
	return l, rr
}

// readBranches liste les branches locales. La comparaison de chaque branche
// avec main (un appel git par branche) n'est faite que si all est vrai.
// remoteRefs renvoie les branches distantes connues (origin/main, upstream/dev…).
func remoteRefs(ctx context.Context, dir string) map[string]bool {
	out, err := runGit(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/remotes")
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, l := range strings.Fields(out) {
		set[l] = true
	}
	return set
}

// matchRemote cherche une branche distante portant le même nom, origin d'abord.
func matchRemote(remotes map[string]bool, branch string) string {
	if branch == "" {
		return ""
	}
	if remotes["origin/"+branch] {
		return "origin/" + branch
	}
	for ref := range remotes {
		if _, short, ok := strings.Cut(ref, "/"); ok && short == branch {
			return ref
		}
	}
	return ""
}

func (r *Repo) readBranches(ctx context.Context, dir string, remotes map[string]bool, all bool) {
	const sep = "\x1f"
	format := strings.Join([]string{
		"%(refname:short)", "%(upstream:short)", "%(upstream:track)",
		"%(committerdate:unix)", "%(HEAD)",
	}, sep)
	out, err := runGit(ctx, dir, "for-each-ref", "--format="+format, "refs/heads")
	if err != nil {
		return
	}
	mainLocal := strings.TrimPrefix(r.MainRef, "origin/")
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Split(line, sep)
		if len(f) != 5 {
			continue
		}
		b := Branch{Name: f[0], Upstream: f[1], Current: f[4] == "*"}
		track := strings.Trim(f[2], "[]")
		if track == "gone" {
			b.UpstreamGone = true
		} else {
			for _, part := range strings.Split(track, ", ") {
				if n, ok := strings.CutPrefix(part, "ahead "); ok {
					b.Ahead, _ = strconv.Atoi(n)
				}
				if n, ok := strings.CutPrefix(part, "behind "); ok {
					b.Behind, _ = strconv.Atoi(n)
				}
			}
		}
		if ts, err := strconv.ParseInt(f[3], 10, 64); err == nil {
			b.LastCommit = time.Unix(ts, 0)
		}

		// La main locale est-elle en retard sur celle du serveur ? On démarre
		// souvent une branche depuis elle sans s'en rendre compte.
		if b.Name == mainLocal && r.MainRef != mainLocal {
			r.LocalMainName = b.Name
			r.LocalMainBehind, _ = leftRight(ctx, dir, r.MainRef, b.Name)
		}

		// Sans upstream, la branche est peut-être déjà sur le serveur sous le même
		// nom : on la compare alors à celle-là (vrai pour la branche courante aussi).
		if m := matchRemote(remotes, b.Name); m != "" {
			b.MatchRemote = m
			if b.Upstream == "" {
				b.MatchBehind, b.MatchAhead = leftRight(ctx, dir, m, b.Name)
			}
		}

		// Travail non poussé sur une autre branche que la courante.
		if !b.Current && b.Name != mainLocal {
			switch {
			case b.Upstream != "" && !b.UpstreamGone:
				if b.Ahead > 0 {
					r.UnpushedBranches = append(r.UnpushedBranches, b.Name)
				}
			case b.MatchRemote != "":
				if b.MatchAhead > 0 {
					r.UnpushedBranches = append(r.UnpushedBranches, b.Name)
				} else {
					r.UnlinkedBranches = append(r.UnlinkedBranches, b.Name)
				}
			case r.MainRef == "":
				r.UnpushedBranches = append(r.UnpushedBranches, b.Name)
			default:
				if _, ahead := leftRight(ctx, dir, r.MainRef, b.Name); ahead > 0 {
					b.AheadMain = ahead
					r.UnpushedBranches = append(r.UnpushedBranches, b.Name)
				}
			}
		}

		if all && r.MainRef != "" {
			b.BehindMain, b.AheadMain = leftRight(ctx, dir, r.MainRef, b.Name)
			b.MergedInMain = b.AheadMain == 0
		}
		if all {
			r.Branches = append(r.Branches, b)
		}
	}
	sort.Slice(r.Branches, func(i, j int) bool {
		if r.Branches[i].Current != r.Branches[j].Current {
			return r.Branches[i].Current
		}
		return r.Branches[i].LastCommit.After(r.Branches[j].LastCommit)
	})
}

func readConfig(ctx context.Context, dir string) *Config {
	c := &Config{}
	get := func(key string) string {
		out, _ := runGit(ctx, dir, "config", "--get", key)
		return strings.TrimSpace(out)
	}
	c.UserName = get("user.name")
	c.UserEmail = get("user.email")
	c.HooksPath = get("core.hooksPath")
	c.Signing = get("commit.gpgsign") == "true"

	if out, err := runGit(ctx, dir, "remote", "-v"); err == nil {
		seen := map[string]bool{}
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			f := strings.Fields(line)
			if len(f) >= 2 && !seen[f[0]] {
				seen[f[0]] = true
				c.Remotes = append(c.Remotes, Remote{Name: f[0], URL: f[1]})
			}
		}
	}

	// Hooks actifs : fichiers exécutables sans suffixe .sample.
	hooksDir := c.HooksPath
	if hooksDir == "" {
		if out, err := runGit(ctx, dir, "rev-parse", "--git-path", "hooks"); err == nil {
			hooksDir = strings.TrimSpace(out)
		}
	}
	if hooksDir != "" {
		if !filepath.IsAbs(hooksDir) {
			hooksDir = filepath.Join(dir, hooksDir)
		}
		if entries, err := os.ReadDir(hooksDir); err == nil {
			for _, e := range entries {
				if e.IsDir() || strings.HasSuffix(e.Name(), ".sample") {
					continue
				}
				if info, err := e.Info(); err == nil && info.Mode()&0o111 != 0 {
					c.Hooks = append(c.Hooks, e.Name())
				}
			}
		}
	}
	return c
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
