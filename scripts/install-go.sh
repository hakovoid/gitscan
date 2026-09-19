#!/bin/sh
# Installe (ou met à jour) la dernière version officielle de Go.
#
# Usage :
#   sh install-go.sh            # installe dans /usr/local/go (utilise sudo si besoin)
#   sh install-go.sh --user     # installe dans ~/.local/go (sans sudo)
#   sh install-go.sh --version go1.24.2   # une version précise
#
# Fonctionne sur Linux et macOS (amd64, arm64, 386, armv6l).
set -eu

# ---------- Options ----------
MODE=system
VERSION=""
while [ $# -gt 0 ]; do
  case "$1" in
    --user) MODE=user ;;
    --version) VERSION="${2:?--version attend une valeur, ex. go1.24.2}"; shift ;;
    -h|--help) sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "Option inconnue : $1 (voir --help)" >&2; exit 1 ;;
  esac
  shift
done

say()  { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31mErreur :\033[0m %s\n' "$*" >&2; exit 1; }

# ---------- Outils nécessaires ----------
if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL "$1"; }
  download() { curl -fL --progress-bar -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -qO- "$1"; }
  download() { wget -q --show-progress -O "$2" "$1"; }
else
  fail "curl ou wget est nécessaire."
fi
command -v tar >/dev/null 2>&1 || fail "tar est nécessaire."

# ---------- Système et processeur ----------
case "$(uname -s)" in
  Linux)  OS=linux ;;
  Darwin) OS=darwin ;;
  *) fail "système non géré : $(uname -s). Sous Windows, utilise l'installeur .msi de https://go.dev/dl/" ;;
esac
case "$(uname -m)" in
  x86_64|amd64)  ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  i386|i686)     ARCH=386 ;;
  armv6l|armv7l) ARCH=armv6l ;;
  *) fail "processeur non géré : $(uname -m)" ;;
esac

# ---------- Version ----------
if [ -z "$VERSION" ]; then
  VERSION=$(fetch 'https://go.dev/VERSION?m=text' | head -n 1)
  [ -n "$VERSION" ] || fail "impossible de connaître la dernière version (réseau ?)."
fi
case "$VERSION" in go*) ;; *) VERSION="go$VERSION" ;; esac

# ---------- Destination ----------
SUDO=""
if [ "$MODE" = user ]; then
  PREFIX="$HOME/.local"
elif [ -w /usr/local ] || [ "$(id -u)" -eq 0 ]; then
  PREFIX=/usr/local
elif command -v sudo >/dev/null 2>&1; then
  PREFIX=/usr/local
  SUDO=sudo
else
  say "Pas de sudo : installation dans ~/.local/go"
  PREFIX="$HOME/.local"
fi
GOROOT_DIR="$PREFIX/go"

# Déjà à jour ?
if [ -x "$GOROOT_DIR/bin/go" ]; then
  CURRENT=$("$GOROOT_DIR/bin/go" env GOVERSION 2>/dev/null || echo "")
  if [ "$CURRENT" = "$VERSION" ]; then
    say "$VERSION est déjà installé dans $GOROOT_DIR. Rien à faire."
    exit 0
  fi
  say "Mise à jour : ${CURRENT:-version inconnue} → $VERSION"
fi

# ---------- Téléchargement et vérification ----------
FILE="$VERSION.$OS-$ARCH.tar.gz"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

say "Téléchargement de $FILE"
download "https://dl.google.com/go/$FILE" "$TMP/$FILE" || fail "téléchargement impossible (version inexistante ?)."

EXPECTED=$(fetch "https://dl.google.com/go/$FILE.sha256" 2>/dev/null | cut -c1-64 || true)
if [ -n "$EXPECTED" ]; then
  if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL=$(sha256sum "$TMP/$FILE" | cut -d' ' -f1)
  else
    ACTUAL=$(shasum -a 256 "$TMP/$FILE" | cut -d' ' -f1)
  fi
  [ "$ACTUAL" = "$EXPECTED" ] || fail "somme de contrôle incorrecte : fichier corrompu, installation annulée."
  say "Somme de contrôle vérifiée"
else
  say "Somme de contrôle indisponible, vérification ignorée"
fi

# ---------- Installation ----------
say "Installation dans $GOROOT_DIR"
$SUDO mkdir -p "$PREFIX"
$SUDO rm -rf "$GOROOT_DIR"
$SUDO tar -C "$PREFIX" -xzf "$TMP/$FILE"

# ---------- PATH ----------
LINE="export PATH=\"\$PATH:$GOROOT_DIR/bin:\$HOME/go/bin\""
case "${SHELL:-}" in
  */zsh)  RC="$HOME/.zshrc" ;;
  */bash) RC="$HOME/.bashrc" ;;
  *)      RC="$HOME/.profile" ;;
esac
if ! grep -qs "$GOROOT_DIR/bin" "$RC"; then
  printf '\n# Go\n%s\n' "$LINE" >> "$RC"
  say "PATH ajouté dans $RC"
fi

say "Terminé : $("$GOROOT_DIR/bin/go" version)"
case ":$PATH:" in
  *":$GOROOT_DIR/bin:"*) ;;
  *) echo "   Ouvre un nouveau terminal (ou lance : source $RC) pour utiliser la commande go." ;;
esac
