package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// discoverRepos parcourt root et renvoie le chemin de chaque dépôt git trouvé.
// Un dossier est un dépôt s'il contient un .git (dossier, ou fichier dans le
// cas d'un worktree ou d'un submodule). Par défaut on ne descend pas dans
// un dépôt une fois trouvé, sauf si nested est vrai.
func discoverRepos(root string, maxDepth int, excludes map[string]bool, nested bool) ([]string, error) {
	root = filepath.Clean(root)
	rootDepth := strings.Count(root, string(os.PathSeparator))
	var repos []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Dossier illisible (permissions...) : on l'ignore sans s'arrêter.
			if d != nil && d.IsDir() && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != root && (name == ".git" || excludes[name]) {
			return fs.SkipDir
		}
		if maxDepth > 0 && strings.Count(path, string(os.PathSeparator))-rootDepth > maxDepth {
			return fs.SkipDir
		}
		if _, statErr := os.Stat(filepath.Join(path, ".git")); statErr == nil {
			repos = append(repos, path)
			if !nested {
				return fs.SkipDir
			}
		}
		return nil
	})
	return repos, err
}
