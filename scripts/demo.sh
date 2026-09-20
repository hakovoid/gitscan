#!/usr/bin/env bash
# Crée un dossier de démonstration avec des dépôts dans des états variés.
# Usage : scripts/demo.sh [dossier]   (défaut : /tmp/gitscan-demo)
set -euo pipefail

D="${1:-/tmp/gitscan-demo}"
rm -rf "$D" && mkdir -p "$D/remotes" "$D/code/perso" "$D/code/work"
export GIT_AUTHOR_NAME=Demo GIT_AUTHOR_EMAIL=demo@example.com
export GIT_COMMITTER_NAME=Demo GIT_COMMITTER_EMAIL=demo@example.com
g() { git -c init.defaultBranch=main -c advice.detachedHead=false "$@" >/dev/null 2>&1; }

commit() { echo "$2" >> "$1/f.txt"; g -C "$1" add -A; g -C "$1" commit -m "$2"; }

# Crée un remote nu + un clone initial poussé.
mkrepo() {
  local name=$1 dest=$2
  g init --bare "$D/remotes/$name.git"
  g clone "$D/remotes/$name.git" "$dest"
  commit "$dest" "init"
  g -C "$dest" push -u origin main
  g -C "$dest" remote set-head origin main
}
# Simule un collègue qui pousse sur main.
push_other() {
  local name=$1 n=$2 tmp="$D/tmp-$1"
  g clone "$D/remotes/$name.git" "$tmp"
  for i in $(seq "$n"); do commit "$tmp" "autre $i"; done
  g -C "$tmp" push origin main
  rm -rf "$tmp"
}

# 1. propre et à jour
mkrepo api "$D/code/work/api"

# 2. commits à pousser
mkrepo web "$D/code/work/web"
commit "$D/code/work/web" "local 1"; commit "$D/code/work/web" "local 2"

# 3. en retard (collègue a poussé), après fetch
mkrepo infra "$D/code/work/infra"
push_other infra 3
g -C "$D/code/work/infra" fetch

# 4. divergé + fichiers modifiés + non suivis
mkrepo blog "$D/code/perso/blog"
push_other blog 1
commit "$D/code/perso/blog" "mon article"
g -C "$D/code/perso/blog" fetch
echo "brouillon" >> "$D/code/perso/blog/f.txt"
echo "x" > "$D/code/perso/blog/nouveau.md"

# 5. branche de feature poussée mais en retard sur main + branche locale jamais poussée + stash
mkrepo app "$D/code/work/app"
g -C "$D/code/work/app" checkout -b feature/login
commit "$D/code/work/app" "login"
g -C "$D/code/work/app" push -u origin feature/login
g -C "$D/code/work/app" checkout -b experiment
commit "$D/code/work/app" "essai"
g -C "$D/code/work/app" checkout main
g -C "$D/code/work/app" checkout -b old-fix
g -C "$D/code/work/app" checkout feature/login
push_other app 4
g -C "$D/code/work/app" fetch
echo tmp >> "$D/code/work/app/f.txt"; g -C "$D/code/work/app" stash
cat > "$D/code/work/app/.git/hooks/pre-commit" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod +x "$D/code/work/app/.git/hooks/pre-commit"

# 6. dépôt local sans remote, branche master
g -c init.defaultBranch=master init "$D/code/perso/notes"
commit "$D/code/perso/notes" "notes"

# 7. branche distante supprimée
mkrepo lib "$D/code/work/lib"
g -C "$D/code/work/lib" checkout -b feature/old
commit "$D/code/work/lib" "vieux"
g -C "$D/code/work/lib" push -u origin feature/old
g -C "$D/code/work/lib" push origin --delete feature/old
g -C "$D/code/work/lib" fetch --prune

# 8. Façon « serveur » : un dépôt parent avec des sous-modules
S="$D/code/serveur/site"
for m in mobile portail webform; do mkrepo "$m" "$D/tmp-sub-$m"; done
mkrepo site "$S"
for m in mobile portail webform; do
  g -C "$S" -c protocol.file.allow=always submodule add "$D/remotes/$m.git" "$m"
done
g -C "$S" commit -m "ajout des sous-modules"; g -C "$S" push
# Comme après un « git submodule update » : sous-modules en HEAD détachée
g -C "$S" -c protocol.file.allow=always submodule update --init
for m in mobile portail webform; do g -C "$S/$m" checkout --detach; done
# site : un rebase interrompu par un conflit
g -C "$S" checkout -b tmp; commit "$S" "conflit A"; g -C "$S" checkout main
echo "conflit B" >> "$S/f.txt"; g -C "$S" add f.txt; g -C "$S" commit -m "conflit B"
g -C "$S" checkout tmp; g -C "$S" rebase main || true
# portail : décalé (quelqu'un a fait un checkout d'un autre commit)
push_other portail 2; g -C "$S/portail" fetch; g -C "$S/portail" checkout origin/main
# webform : commit fait en HEAD détachée, hors de toute branche
commit "$S/webform" "correctif direct sur le serveur"
# un autre projet du serveur : droits modifiés par un chmod (contenu identique)
mkrepo docs "$D/code/serveur/docs"
for i in 1 2 3; do echo "$i" > "$D/code/serveur/docs/p$i.md"; done
g -C "$D/code/serveur/docs" add -A; g -C "$D/code/serveur/docs" commit -m pages; g -C "$D/code/serveur/docs" push
chmod +x "$D"/code/serveur/docs/*.md
echo "vraie modif" >> "$D/code/serveur/docs/p1.md"; chmod -x "$D/code/serveur/docs/p1.md"
# un déploiement sur un commit portant plusieurs tags (cas fréquent : releases successives)
mkrepo stopcom "$D/code/serveur/stopcom"
for t in sprint-30 sprint-31 sprint-32 sprint-33; do g -C "$D/code/serveur/stopcom" tag "$t"; done
g -C "$D/code/serveur/stopcom" push --tags
g -C "$D/code/serveur/stopcom" checkout --detach

rm -rf "$D"/tmp-sub-*

# Un node_modules qui contient un dépôt (doit être ignoré)
mkdir -p "$D/code/work/web/node_modules/pkg" && g init "$D/code/work/web/node_modules/pkg"

echo "Démo créée dans $D/code"
