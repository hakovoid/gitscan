package main

// États normaux : un fichier .gitscan indique, dépôt par dépôt, les signaux
// qui sont attendus et ne doivent plus compter comme « à traiter ».
//
//	# commentaire
//	normal  serveur/docs    mode_only
//	normal  */stopcom       detached
//	normal  archives/**     *
//
// Le motif est un chemin relatif au dossier du fichier : * remplace un nom,
// ** plusieurs niveaux ; un motif sans / vise le nom du dossier à toute
// profondeur. Les signaux « à risque » (conflits, commits hors branche,
// opération interrompue, erreur) ne peuvent jamais être déclarés normaux.

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const normalFileName = ".gitscan"

// Signaux qui peuvent être déclarés normaux.
var normalizable = map[string]bool{
	"fetch_failed": true, "dirty": true, "mode_only": true, "untracked": true, "stash": true,
	"submodules_changed": true, "submodule_new": true, "submodule_drift": true,
	"detached": true, "upstream_gone": true, "no_remote": true, "no_upstream": true, "unlinked": true,
	"ahead": true, "behind": true, "diverged": true, "upstream_other": true,
	"unlinked_branches": true, "unpushed_branches": true, "merged_branches": true,
	"local_main_behind": true, "main_local": true, "behind_main": true, "stale_fetch": true,
}

// Signaux à risque : toujours affichés, quoi que dise le fichier.
var riskCodes = map[string]bool{"error": true, "operation": true, "orphan_commits": true, "conflicts": true}

type normalRule struct {
	pattern string
	codes   map[string]bool // "*" = tous les signaux autorisés
	line    int
}

// normalRules : les règles d'un fichier .gitscan.
type normalRules struct {
	file     string // chemin du fichier
	base     string // dossier du fichier : les motifs sont relatifs à lui
	rules    []normalRule
	warnings []string // lignes ignorées, avec la raison
}

// findNormalFile cherche .gitscan dans root puis dans ses dossiers parents.
func findNormalFile(root string) string {
	dir, _ := filepath.Abs(root)
	for {
		p := filepath.Join(dir, normalFileName)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func loadNormal(file string) (*normalRules, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	abs, _ := filepath.Abs(file)
	n := &normalRules{file: abs, base: filepath.Dir(abs)}
	sc := bufio.NewScanner(f)
	for num := 1; sc.Scan(); num++ {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		warn := func(format string, a ...any) {
			n.warnings = append(n.warnings, fmt.Sprintf("%s:%d : ", filepath.Base(file), num)+fmt.Sprintf(format, a...))
		}
		if fields[0] != "normal" {
			warn("mot-clé inconnu %q (attendu : normal)", fields[0])
			continue
		}
		if len(fields) < 3 {
			warn("il faut un motif puis au moins un signal : normal <motif> <signal>…")
			continue
		}
		rule := normalRule{pattern: filepath.ToSlash(strings.Trim(fields[1], "/")), codes: map[string]bool{}, line: num}
		for _, code := range fields[2:] {
			switch {
			case code == "*" || normalizable[code]:
				rule.codes[code] = true
			case riskCodes[code]:
				warn("%s ne peut pas être déclaré normal : c'est un risque de perte", code)
			default:
				warn("signal inconnu %q (voir gitscan help)", code)
			}
		}
		if len(rule.codes) > 0 {
			n.rules = append(n.rules, rule)
		}
	}
	return n, sc.Err()
}

// apply marque comme normaux les signaux prévus pour ce dépôt : ils passent au
// niveau « info » et ne comptent plus dans les dépôts à traiter.
func (n *normalRules) apply(r *Repo) {
	if n == nil || len(n.rules) == 0 {
		return
	}
	rel, err := filepath.Rel(n.base, r.AbsPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return
	}
	rel = filepath.ToSlash(rel)
	for i := range r.Flags {
		f := &r.Flags[i]
		if !normalizable[f.Code] {
			continue
		}
		if f.Code == "detached" && len(r.HeadTags) > 0 {
			continue // « tag v1.2 » : c'est la version déployée, une information à garder
		}
		for _, rule := range n.rules {
			if (rule.codes["*"] || rule.codes[f.Code]) && matchPattern(rule.pattern, rel) {
				f.Level, f.Normal = Info, fmt.Sprintf("%s:%d", filepath.Base(n.file), rule.line)
				break
			}
		}
	}
}

// matchPattern : * pour un nom, ** pour plusieurs niveaux ; sans /, le motif
// vise le nom du dossier à n'importe quelle profondeur.
func matchPattern(pattern, rel string) bool {
	if pattern == "" || pattern == "." {
		return rel == "."
	}
	if !strings.Contains(pattern, "/") && pattern != "**" {
		ok, _ := path.Match(pattern, path.Base(rel))
		return ok
	}
	return globSegs(strings.Split(pattern, "/"), strings.Split(rel, "/"))
}

func globSegs(p, s []string) bool {
	if len(p) == 0 {
		return len(s) == 0
	}
	if p[0] == "**" {
		for i := 0; i <= len(s); i++ {
			if globSegs(p[1:], s[i:]) {
				return true
			}
		}
		return false
	}
	if len(s) == 0 {
		return false
	}
	ok, _ := path.Match(p[0], s[0])
	return ok && globSegs(p[1:], s[1:])
}

// isNormal : le signal code est présent et déclaré normal pour ce dépôt.
func (r *Repo) isNormal(code string) bool {
	for _, f := range r.Flags {
		if f.Code == code && f.Normal != "" {
			return true
		}
	}
	return false
}

// normalCount : nombre de signaux déclarés normaux sur l'ensemble des dépôts.
func normalCount(repos []*Repo) int {
	n := 0
	for _, r := range repos {
		for _, f := range r.Flags {
			if f.Normal != "" {
				n++
			}
		}
	}
	return n
}

// suggestRule : la ligne à ajouter dans .gitscan pour déclarer normaux les
// signaux actuels d'un dépôt (ceux qui peuvent l'être et ne le sont pas déjà).
func suggestRule(n *normalRules, root string, r *Repo) string {
	base := root
	if n != nil {
		base = n.base
	}
	rel, err := filepath.Rel(base, r.AbsPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	var cs []string
	for _, f := range r.Flags {
		if f.Code == "detached" && len(r.HeadTags) > 0 {
			continue
		}
		if normalizable[f.Code] && f.Normal == "" && !contains(cs, f.Code) {
			cs = append(cs, f.Code)
		}
	}
	if len(cs) == 0 {
		return ""
	}
	sort.Strings(cs)
	return fmt.Sprintf("normal  %s  %s", filepath.ToSlash(rel), strings.Join(cs, " "))
}

const normalHeader = `# gitscan : états normaux
#
# Une règle par ligne :   normal  <dossier>  <signal> [<signal>…]
#   <dossier> : chemin relatif à ce fichier ; * remplace un nom, ** plusieurs
#               niveaux ; sans /, le nom du dossier à n'importe quelle profondeur.
#   <signal>  : code affiché entre crochets par la touche i (ou * pour tous).
#
# Un signal déclaré normal ne compte plus comme « à traiter » et disparaît de
# la colonne À VOIR (il reste visible avec la touche i et dans le détail).
# Les risques (conflits, commits hors branche, opération interrompue) restent
# toujours signalés. gitscan -strict ignore ce fichier.
#
# Exemples :
#   normal  serveur/docs  mode_only untracked
#   normal  archives/**   *
`

// prepareNormalFile crée le fichier s'il n'existe pas, et y ajoute en
// commentaire la règle suggérée pour un dépôt (à décommenter par l'utilisateur).
func prepareNormalFile(file, suggestion, repoPath string) error {
	content, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		content, err = []byte(normalHeader), nil
	}
	if err != nil {
		return err
	}
	text := string(content)
	if suggestion != "" && !strings.Contains(text, suggestion) {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += fmt.Sprintf("\n# %s : retire le # pour activer, enlève les signaux à garder\n# %s\n", repoPath, suggestion)
	}
	return os.WriteFile(file, []byte(text), 0o644)
}
