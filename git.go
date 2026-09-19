package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runGit exécute une commande git dans le dépôt dir et renvoie sa sortie standard.
// On appelle le binaire git plutôt que go-git : c'est plus rapide et
// cela respecte exactement la configuration de l'utilisateur.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // jamais de demande de mot de passe interactive
		"GIT_OPTIONAL_LOCKS=0",  // ne pas verrouiller l'index pendant un status
		"LC_ALL=C",              // sortie stable, non traduite
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("git %s: %s", args[0], firstLine(msg))
	}
	return stdout.String(), nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
