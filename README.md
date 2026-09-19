# gitscan

Scanne un dossier (et ses sous-dossiers), trouve tous les dépôts git et affiche en un coup d'œil :

- la **branche courante** et son avance/retard sur le **remote** (à pousser ↑ / à tirer ↓) ;
- l'avance/retard par rapport à **main** (détecté automatiquement : `origin/HEAD`, `main`, `master`) ;
- les **modifications locales** : fichiers modifiés, non suivis, conflits, stash ;
- les **branches locales jamais poussées** ou dont la branche distante a été supprimée ;
- en option, **toutes les branches** (fusionnées ou non dans main) et la **configuration** (remotes, auteur, hooks, signature).

Aucune dépendance : bibliothèque standard Go + le binaire `git`.

```
DÉPÔT        BRANCHE        REMOTE    VS MAIN  ÉTAT
perso/blog   main           ↑1 ↓1     ↑1 ↓1    modifié 1, non suivi 1, divergé ↑1 ↓1
perso/notes  master         —         =        jamais poussée
work/api     main           =         =        ✓ propre
work/app     feature/login  =         ↑1 ↓4    1 branche(s) non poussée(s), retard main ↓4, stash 1
work/infra   main           ↓3        ↓3       à tirer ↓3
work/lib     feature/old    supprimé  ↑1       upstream supprimé
work/web     main           ↑2        ↑2       non suivi 1, à pousser ↑2

7 dépôts · 4 à pousser · 2 à tirer · 2 modifié(s) · 1 propre  (79ms)
```

## Installation

```sh
go build -o gitscan .          # Go 1.21+
sudo mv gitscan /usr/local/bin # ou go install .
```

## Utilisation

```sh
gitscan ~/code              # état de tous les dépôts
gitscan -f ~/code           # git fetch d'abord (recommandé : sinon l'état distant peut être périmé)
gitscan -a ~/code           # seulement ce qui demande une action
gitscan -b ~/code           # + détail de chaque branche
gitscan -c ~/code           # + configuration (remotes, auteur, hooks)
gitscan -json ~/code        # sortie JSON complète
gitscan -check ~/code       # code de sortie 1 si un dépôt demande une action
```

| Option | Rôle |
|---|---|
| `-f` | `git fetch --all --prune` sur chaque dépôt, en parallèle |
| `-fetch-timeout 30s` | délai maximal d'un fetch |
| `-b` | liste toutes les branches locales, leur état vs upstream et vs main |
| `-c` | affiche la configuration de chaque dépôt |
| `-a` | n'affiche que les dépôts qui demandent une action |
| `-json` | sortie JSON (inclut branches et config) |
| `-main nom` | force le nom de la branche principale |
| `-depth N` | profondeur maximale de recherche (0 = illimitée) |
| `-j N` | nombre de dépôts analysés en parallèle |
| `-exclude a,b` | dossiers ignorés (défaut : `node_modules,vendor,.cache,.venv,venv,target`) |
| `-nested` | cherche aussi des dépôts dans d'autres dépôts |
| `-check` | code de sortie 1 si un dépôt demande une action |
| `-no-color` | désactive les couleurs (aussi via `NO_COLOR`) |

### Exemples avec jq

```sh
# Dépôts avec des commits non poussés
gitscan -json ~/code | jq -r '.[] | select(.ahead > 0) | .path'
# Branches déjà fusionnées dans main (à nettoyer)
gitscan -json ~/code | jq -r '.[] | .path as $p | .branches[] | select(.merged_in_main and (.current|not)) | "\($p): \(.name)"'
# Dépôts sans user.email local
gitscan -json ~/code | jq -r '.[] | select(.config.user_email == null) | .path'
```

## Lecture des signaux

| Signal | Niveau | Signification |
|---|---|---|
| `à pousser ↑n` | action | n commits locaux absents du remote |
| `à tirer ↓n` | action | n commits distants à récupérer |
| `divergé ↑a ↓b` | action | les deux : il faudra rebase ou merge |
| `modifié n` / `non suivi n` | action | travail non commité |
| `conflits n` | erreur | merge ou rebase en cours |
| `jamais poussée` | action | la branche courante n'a pas d'upstream |
| `upstream supprimé` | action | la branche distante a disparu (souvent : PR fusionnée) |
| `n branche(s) non poussée(s)` | action | d'autres branches locales ont des commits absents du remote |
| `HEAD détachée` | action | pas sur une branche |
| `retard main ↓n` | info | main a avancé depuis la création de la branche |
| `stash n` | info | des stash traînent |
| `fetch il y a …` | info | dernier fetch de plus de 7 jours |

Colonne **VS MAIN** : `↑` = commits de la branche absents de main, `↓` = commits de main absents de la branche.

## Fonctionnement

- Découverte : parcours récursif ; un dossier contenant `.git` (dossier ou fichier, donc worktrees et submodules compris) est un dépôt. On ne descend pas dedans, sauf avec `-nested`.
- Chaque dépôt est analysé par un pool de goroutines. L'essentiel vient d'un seul `git status --porcelain=v2 --branch`, complété par `for-each-ref` (branches) et `rev-list --left-right --count` (comparaison avec main).
- Les commandes git tournent avec `GIT_TERMINAL_PROMPT=0` (jamais bloqué par une demande de mot de passe) et `GIT_OPTIONAL_LOCKS=0` (n'interfère pas avec un éditeur ouvert).

## Tester

`scripts/demo.sh /tmp/demo` crée des dépôts dans tous les états ci-dessus, puis `gitscan -b -c /tmp/demo/code`.

## Pistes pour la suite

- Interface texte interactive (Bubble Tea) : naviguer, cocher des dépôts, lancer pull / push / fetch en lot.
- Actions en masse : `gitscan pull --ff-only`, `gitscan prune-merged`.
- Fichier de config (`~/.config/gitscan.toml`) : dossiers par défaut, exclusions, branche principale par dépôt.
