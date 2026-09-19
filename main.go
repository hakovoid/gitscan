// gitscan : scanne un dossier, trouve tous les dépôts git et affiche leur état
// (branche, avance/retard sur le remote et sur main, modifications locales,
// branches non poussées, configuration).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const version = "0.1.0"

func main() {
	os.Exit(run())
}

func run() int {
	var (
		fetch        = flag.Bool("f", false, "lancer `git fetch --all --prune` sur chaque dépôt avant l'analyse")
		fetchTimeout = flag.Duration("fetch-timeout", 30*time.Second, "délai maximal d'un fetch")
		branches     = flag.Bool("b", false, "afficher toutes les branches locales de chaque dépôt")
		config       = flag.Bool("c", false, "afficher la configuration (remotes, auteur, hooks)")
		attention    = flag.Bool("a", false, "n'afficher que les dépôts qui demandent une action")
		jsonOut      = flag.Bool("json", false, "sortie JSON (pour scripts et CI)")
		mainBranch   = flag.String("main", "", "nom de la branche principale (détectée automatiquement sinon)")
		depth        = flag.Int("depth", 0, "profondeur maximale de recherche (0 = illimitée)")
		jobs         = flag.Int("j", runtime.NumCPU()*2, "nombre de dépôts analysés en parallèle")
		exclude      = flag.String("exclude", "node_modules,vendor,.cache,.venv,venv,target", "dossiers ignorés, séparés par des virgules")
		nested       = flag.Bool("nested", false, "chercher aussi des dépôts à l'intérieur d'autres dépôts")
		noColor      = flag.Bool("no-color", false, "désactiver les couleurs")
		check        = flag.Bool("check", false, "code de sortie 1 si un dépôt demande une action")
		showVersion  = flag.Bool("version", false, "afficher la version")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage : gitscan [options] [dossier]\n\n")
		fmt.Fprintf(os.Stderr, "Scanne récursivement un dossier et affiche l'état de chaque dépôt git.\n\nOptions :\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExemples :\n  gitscan ~/code\n  gitscan -f -a ~/code        # fetch puis n'afficher que ce qui demande une action\n  gitscan -b -c .             # détail des branches et de la config\n  gitscan -json ~/code | jq '.[] | select(.ahead > 0) | .path'\n")
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("gitscan", version)
		return 0
	}
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(os.Stderr, "gitscan : git est introuvable dans le PATH")
		return 2
	}

	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}
	root, _ = filepath.Abs(root)
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		fmt.Fprintf(os.Stderr, "gitscan : %s n'est pas un dossier\n", root)
		return 2
	}

	excludes := map[string]bool{}
	for _, e := range strings.Split(*exclude, ",") {
		if e = strings.TrimSpace(e); e != "" {
			excludes[e] = true
		}
	}

	start := time.Now()
	paths, err := discoverRepos(root, *depth, excludes, *nested)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitscan :", err)
		return 2
	}
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "Aucun dépôt git trouvé dans %s\n", root)
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	colors := !*noColor && os.Getenv("NO_COLOR") == "" && isTerminal(os.Stdout) && !*jsonOut
	p := newPalette(colors)
	progress := isTerminal(os.Stderr) && !*jsonOut

	opt := InspectOptions{
		Fetch:        *fetch,
		FetchTimeout: *fetchTimeout,
		MainOverride: *mainBranch,
		AllBranches:  *branches || *jsonOut,
		WithConfig:   *config || *jsonOut,
	}

	// Pool de workers : chaque dépôt est analysé indépendamment.
	repos := make([]*Repo, len(paths))
	idx := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for w := 0; w < max(1, *jobs); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				repos[i] = inspect(ctx, root, paths[i], opt)
				if progress {
					mu.Lock()
					done++
					verb := "Analyse"
					if *fetch {
						verb = "Fetch + analyse"
					}
					fmt.Fprintf(os.Stderr, "\r%s… %d/%d", verb, done, len(paths))
					mu.Unlock()
				}
			}
		}()
	}
	for i := range paths {
		idx <- i
	}
	close(idx)
	wg.Wait()
	if progress {
		fmt.Fprint(os.Stderr, "\r\033[K")
	}

	sort.Slice(repos, func(i, j int) bool { return repos[i].Path < repos[j].Path })

	shown := repos
	if *attention {
		shown = nil
		for _, r := range repos {
			if r.NeedsAttention() {
				shown = append(shown, r)
			}
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if shown == nil {
			shown = []*Repo{}
		}
		if err := enc.Encode(shown); err != nil {
			fmt.Fprintln(os.Stderr, "gitscan :", err)
			return 2
		}
	} else {
		if len(shown) == 0 {
			fmt.Printf("%s✓ Tous les dépôts sont propres et à jour.%s\n", p.green, p.reset)
		} else {
			renderTable(os.Stdout, shown, p, *branches, *config)
		}
		renderSummary(os.Stdout, repos, len(repos), p, time.Since(start))
		if !*fetch {
			fmt.Printf("%sAstuce : -f pour faire un fetch d'abord (sinon l'état distant peut être périmé).%s\n", p.dim, p.reset)
		}
	}

	if *check {
		for _, r := range repos {
			if r.NeedsAttention() {
				return 1
			}
		}
	}
	return 0
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
