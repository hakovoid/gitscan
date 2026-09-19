package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func gitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // jamais de demande de mot de passe interactive
		"GIT_OPTIONAL_LOCKS=0",  // ne pas verrouiller l'index pendant un status
		"LC_ALL=C",              // sortie stable, non traduite
	)
	// Détache git du terminal : ssh ne peut plus demander de passphrase
	// (ce qui bloquerait ou casserait l'affichage de la TUI).
	detach(cmd)
	return cmd
}

// runGit exécute une commande git dans le dépôt dir et renvoie sa sortie standard.
// On appelle le binaire git plutôt que go-git : c'est plus rapide et
// cela respecte exactement la configuration de l'utilisateur.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := gitCommand(ctx, dir, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		msg = strings.TrimPrefix(strings.TrimPrefix(firstLine(msg), "fatal: "), "error: ")
		msg = strings.TrimSuffix(msg, " (or any of the parent directories): .git")
		return stdout.String(), fmt.Errorf("git %s: %s", args[0], msg)
	}
	return stdout.String(), nil
}

// runGitCombined renvoie stdout+stderr mêlés (utile pour push/pull, qui écrivent
// sur stderr). En cas d'échec, l'erreur contient la ligne la plus parlante.
func runGitCombined(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := gitCommand(ctx, dir, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	text := strings.TrimSpace(out.String())
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return text, errors.New("délai dépassé")
		}
		return text, errors.New(usefulLine(text, err))
	}
	return text, nil
}

func usefulLine(text string, fallback error) string {
	lines := strings.Split(text, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.Contains(l, "[rejected]") || strings.HasPrefix(l, "fatal:") || strings.HasPrefix(l, "error:") {
			return l
		}
	}
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return fallback.Error()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
