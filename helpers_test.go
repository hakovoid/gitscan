package main

// Outils communs aux tests : de vrais dépôts git créés dans un dossier
// temporaire, un « serveur » (dépôt nu) et des clones. Rien n'est simulé :
// gitscan est testé contre git lui-même.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gitEnv isole les tests de la configuration de la machine.
func gitEnv(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	// Les sous-modules clonés depuis un chemin local sont bloqués par défaut.
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")
	t.Setenv("GIT_CONFIG_KEY_1", "init.defaultBranch")
	t.Setenv("GIT_CONFIG_VALUE_1", "main")
}

// git lance une commande et fait échouer le test si elle échoue.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s : %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// commit écrit un fichier et le commite.
func commit(t *testing.T, dir, file, content string) string {
	t.Helper()
	write(t, filepath.Join(dir, file), content)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "modifie "+file)
	return git(t, dir, "rev-parse", "--short", "HEAD")
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// server crée un dépôt nu avec un premier commit sur main, et renvoie son chemin.
func server(t *testing.T, base, name string) string {
	t.Helper()
	bare := filepath.Join(base, "serveur", name+".git")
	git(t, base, "init", "-q", "--bare", bare)
	seed := filepath.Join(base, "seed-"+name)
	git(t, base, "clone", "-q", bare, seed)
	commit(t, seed, "README", "début\n")
	git(t, seed, "push", "-q", "origin", "main")
	return bare
}

// clone clone le serveur dans base/code/name.
func clone(t *testing.T, bare, base, name string) string {
	t.Helper()
	dir := filepath.Join(base, "code", name)
	git(t, base, "clone", "-q", bare, dir)
	return dir
}

// scan analyse un dépôt comme le ferait gitscan.
func scan(t *testing.T, root, dir string, all bool) *Repo {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return inspect(ctx, root, dir, InspectOptions{AllBranches: all, WithConfig: all, FetchTimeout: 10 * time.Second})
}

// codes renvoie les codes des signaux, pour des assertions lisibles.
func codes(r *Repo) []string {
	var cs []string
	for _, f := range r.Flags {
		cs = append(cs, f.Code)
	}
	return cs
}

func hasCode(r *Repo, code string) bool {
	for _, c := range codes(r) {
		if c == code {
			return true
		}
	}
	return false
}

func mustHave(t *testing.T, r *Repo, want ...string) {
	t.Helper()
	for _, w := range want {
		if !hasCode(r, w) {
			t.Errorf("%s : signal %q attendu, trouvé %v", r.Path, w, codes(r))
		}
	}
}

func mustNotHave(t *testing.T, r *Repo, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if hasCode(r, u) {
			t.Errorf("%s : signal %q inattendu (signaux : %v)", r.Path, u, codes(r))
		}
	}
}
