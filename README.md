```text
 ██████╗ ██╗████████╗███████╗ ██████╗ █████╗ ███╗   ██╗
██╔════╝ ██║╚══██╔══╝██╔════╝██╔════╝██╔══██╗████╗  ██║
██║  ███╗██║   ██║   ███████╗██║     ███████║██╔██╗ ██║
██║   ██║██║   ██║   ╚════██║██║     ██╔══██║██║╚██╗██║
╚██████╔╝██║   ██║   ███████║╚██████╗██║  ██║██║ ╚████║
 ╚═════╝ ╚═╝   ╚═╝   ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═══╝
```

**L'état de tous tes dépôts git, en un coup d'œil.**

Scanne un dossier (et ses sous-dossiers), trouve tous les dépôts git et affiche en un coup d'œil :

- la **branche courante** et son avance/retard sur le **remote** (à pousser ↑ / à tirer ↓) ;
- l'avance/retard par rapport à **main** (détecté automatiquement : `origin/HEAD`, `main`, `master`) ;
- les **modifications locales** : fichiers modifiés, non suivis, conflits, stash ;
- les **branches locales jamais poussées** ou dont la branche distante a été supprimée ;
- en option, **toutes les branches** (fusionnées ou non dans main) et la **configuration** (remotes, auteur, hooks, signature).

Deux modes : un **rapport** en ligne de commande (scriptable, JSON) et une **interface interactive** (`-i`) pour agir en lot : fetch, pull, push.

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
go mod tidy                    # télécharge les dépendances (Bubble Tea…) — Go 1.24+
go build -o gitscan .
sudo mv gitscan /usr/local/bin # ou : go install .
```

## Mode interactif (`gitscan -i`)

```
gitscan  8 dépôts · 3 à pousser · 1 à tirer · 2 modifié(s) · 3 propres   ~/code
    DÉPÔT        BRANCHE        REMOTE    VS MAIN    ÉTAT
  ○ perso/blog   main           ↑1 ↓1     ↑1 ↓1      modifié 1, non suivi 1, divergé ↑1 ↓1
  ● perso/notes  master         —         =          ✗ push : aucun remote configuré, jamais poussée
❯ ○ work/infra   main           =         =          ✓ pull, ✓ propre
  ● work/web     main           =         =          ✓ push, non suivi 1
2 sélectionné(s)  ✓ push work/web
espace sélect. · a tout · f fetch · p pull · P push · ⏎ détail · t à traiter · / chercher · ? aide · q quitter
```

| Touche | Action |
|---|---|
| `↑ ↓` / `j k` | naviguer (`g`/`G` début/fin, pgup/pgdown) |
| `espace` / `x` | sélectionner ; `a` tout sélectionner ; `échap` efface |
| `f` | fetch --all --prune (sélection, ou dépôt sous le curseur) |
| `p` | pull **--ff-only** : jamais de merge implicite, git refuse s'il y a divergence |
| `P` | push, **avec confirmation** ; `-u origin HEAD` si la branche n'a jamais été poussée |
| `r` / `R` | ré-analyser la sélection / re-scanner le dossier |
| `entrée` | détail : fichiers modifiés, branches, config, 15 derniers commits, sortie de la dernière action |
| `s` / `l` | ouvrir un shell / lazygit dans le dépôt (retour dans gitscan en quittant) |
| `t` | n'afficher que les dépôts qui demandent une action |
| `o` | trier par nom ou par gravité |
| `/` | rechercher par chemin ou branche |
| `?` / `q` | aide / quitter |

Les actions tournent en parallèle, avec un spinner par dépôt, et chaque dépôt est ré-analysé une fois l'action terminée. `gitscan -i -f` fait un fetch général au démarrage.

Les commandes git sont détachées du terminal : si une clé SSH demande une passphrase (pas d'agent ssh), l'action échoue proprement au lieu de bloquer l'interface. Lance `ssh-add` avant.

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
- La TUI suit l'architecture Elm de Bubble Tea : un modèle (état), `Update` (réagit aux touches et aux résultats git), `View` (dessine l'écran). Voir `tui.go`.
- Les commandes git tournent avec `GIT_TERMINAL_PROMPT=0` (jamais bloqué par une demande de mot de passe) et `GIT_OPTIONAL_LOCKS=0` (n'interfère pas avec un éditeur ouvert).

## Tester

`scripts/demo.sh /tmp/demo` crée des dépôts dans tous les états ci-dessus, puis `gitscan -b -c /tmp/demo/code`.

## Pistes pour la suite

- Dans la TUI : supprimer les branches fusionnées, changer de branche, `stash` / `stash pop`.
- Actions en masse : `gitscan pull --ff-only`, `gitscan prune-merged`.
- Fichier de config (`~/.config/gitscan.toml`) : dossiers par défaut, exclusions, branche principale par dépôt.
