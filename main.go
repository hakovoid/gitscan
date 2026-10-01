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

	xterm "github.com/charmbracelet/x/term"
)

const version = "0.18.0"

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
		color        = flag.String("color", "auto", "couleurs : auto, always (garder dans un pipe, ex. | less -R), never")
		width        = flag.Int("width", 0, "largeur du tableau en colonnes (0 = celle du terminal)")
		check        = flag.Bool("check", false, "code de sortie 1 si un dépôt demande une action")
		showVersion  = flag.Bool("version", false, "afficher la version")
		interactive  = flag.Bool("i", false, "mode interactif (TUI) : naviguer, sélectionner, fetch/pull/push en lot")
		normalFile   = flag.String("normal", "", "fichier des états normaux (défaut : .gitscan dans le dossier scanné ou un parent)")
		strict       = flag.Bool("strict", false, "ignorer le fichier .gitscan : tout signaler")
		changesOnly  = flag.Bool("changes", false, "n'afficher que ce qui a changé depuis le dernier scan (rien si rien n'a changé)")
		noAnim       = flag.Bool("no-anim", false, "mode interactif : pas d'animation (indicateur fixe à la place des spinners)")
		noSave       = flag.Bool("no-save", false, "ne pas enregistrer ce scan comme référence pour le prochain (implicite avec -json et -check, sauf avec -changes)")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage : gitscan [options] [dossier]\n        gitscan help    comment lire le tableau\n\n")
		fmt.Fprintf(os.Stderr, "Scanne récursivement un dossier et affiche l'état de chaque dépôt git.\n\nOptions :\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExemples :\n  gitscan ~/code\n  gitscan -i ~/code           # interface interactive\n  gitscan -f -a ~/code        # fetch puis n'afficher que ce qui demande une action\n  gitscan -b -c .             # détail des branches et de la config\n  gitscan -width 120 -color=always ~/code | less -R   # tableau large, page par page\n  gitscan -json ~/code | jq '.[] | select(.ahead > 0) | .path'\n")
	}
	if len(os.Args) > 1 && (os.Args[1] == "help" || os.Args[1] == "aide") {
		printLegend(os.Stdout, newPalette(isTerminal(os.Stdout) && os.Getenv("NO_COLOR") == ""))
		return 0
	}
	flag.Parse()

	if *showVersion {
		if isTerminal(os.Stdout) {
			fmt.Println(renderBanner(0))
			fmt.Println(stDim.Render(tagline))
			fmt.Println()
		}
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

	var normal *normalRules
	if !*strict {
		file := *normalFile
		if file == "" {
			file = findNormalFile(root)
		}
		if file != "" {
			n, err := loadNormal(file)
			if err != nil {
				fmt.Fprintln(os.Stderr, "gitscan :", err)
				return 2
			}
			normal = n
			if !*interactive {
				for _, w := range n.warnings {
					fmt.Fprintln(os.Stderr, "gitscan : règle ignorée,", w)
				}
			}
		}
	}

	if *interactive {
		opt := InspectOptions{Fetch: *fetch, FetchTimeout: *fetchTimeout, MainOverride: *mainBranch, Normal: normal}
		ui := uiOptions{
			colors: !*noColor && *color != "never" && os.Getenv("NO_COLOR") == "",
			anim:   !*noAnim,
		}
		if err := runTUI(root, *depth, excludes, *nested, opt, *jobs, ui); err != nil {
			fmt.Fprintln(os.Stderr, "gitscan :", err)
			return 2
		}
		return 0
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
	switch *color {
	case "always":
		colors = !*jsonOut
	case "never":
		colors = false
	}
	p := newPalette(colors)
	progress := isTerminal(os.Stderr) && !*jsonOut

	opt := InspectOptions{
		Fetch:        *fetch,
		FetchTimeout: *fetchTimeout,
		MainOverride: *mainBranch,
		AllBranches:  *branches || *jsonOut,
		WithConfig:   *config || *jsonOut,
		Normal:       normal,
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

	// Comparaison avec le scan précédent, puis enregistrement de celui-ci.
	snapFile := snapshotFile(root, *nested, *depth, excludes)
	prev, snapErr := loadSnapshot(snapFile)
	if snapErr != nil {
		fmt.Fprintln(os.Stderr, "gitscan :", snapErr)
	}
	cur := takeSnapshot(root, repos)
	changes := diffSnapshots(prev, cur)
	// Un scan pour un script (-json, -check) ne déplace pas la référence : sinon un
	// cron consommerait les changements avant que l'utilisateur les voie. Avec
	// -changes, la demande porte justement sur les changements : on enregistre.
	if !*noSave && (*changesOnly || !(*jsonOut || *check)) {
		if err := saveSnapshot(snapFile, cur); err != nil {
			fmt.Fprintln(os.Stderr, "gitscan : scan non enregistré :", err)
		}
	}
	if *changesOnly && !*jsonOut {
		if len(changes) > 0 {
			renderChanges(os.Stdout, changes, prev.Time, p, termWidth(*width))
		}
		return checkExit(*check, repos)
	}

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
			renderTable(os.Stdout, shown, filepath.Base(root), p, *branches, *config, termWidth(*width))
		}
		if prev != nil {
			fmt.Println()
			renderChanges(os.Stdout, changes, prev.Time, p, termWidth(*width))
			fmt.Println()
		}
		renderSummary(os.Stdout, repos, p, time.Since(start), termWidth(*width))
		if n := normalCount(repos); n > 0 {
			printDim(os.Stdout, p, termWidth(*width), fmt.Sprintf("%s par %s · -strict pour tout voir",
				plur(n, "signal déclaré normal", "signaux déclarés normaux"), shortPath(normal.file)))
		}
		if !*fetch {
			printDim(os.Stdout, p, termWidth(*width), "Astuce : -f pour faire un fetch d'abord (sinon vs SERVEUR peut être périmé).")
		}
		var more []string
		if !*branches {
			more = append(more, "-b branches")
		}
		if !*config {
			more = append(more, "-c config")
		}
		more = append(more, "gitscan help (légende)")
		printDim(os.Stdout, p, termWidth(*width), "Détails : "+strings.Join(more, " · "))
	}

	return checkExit(*check, repos)
}

// checkExit : avec -check, 1 si un dépôt demande une action.
func checkExit(check bool, repos []*Repo) int {
	if check {
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

// termWidth : largeur demandée, sinon celle du terminal, 0 si la sortie est redirigée.
func termWidth(forced int) int {
	if forced > 0 {
		return forced
	}
	if !isTerminal(os.Stdout) {
		return 0
	}
	if w, _, err := xterm.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		return w
	}
	return 0
}
