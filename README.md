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
   DÉPÔT        │ BRANCHE       │ vs SERVEUR │ vs MAIN │ LOCAL                      │ À VOIR
────────────────┼───────────────┼───────────┼───────┼────────────────────────────┼───────────────────────────────────
 ● perso/blog   │ main          │ ↑1 ↓1     │ ↑1 ↓1 │ 1 modifié · 1 nouveau      │
 ✓ perso/notes  │ master        │ aucun     │ =     │ propre                     │
────────────────┼───────────────┼───────────┼───────┼────────────────────────────┼───────────────────────────────────
 ● serveur/docs │ main          │ =         │ =     │ 3 modifiés · dont 2 droits │
────────────────┼───────────────┼───────────┼───────┼────────────────────────────┼───────────────────────────────────
 ✗ serveur/site │ @c6e87cf      │ —         │ ↑1    │ 1 conflit · 2 sous-modules │ rebase en cours
                │               │           │       │                            │ non poussée(s) : tmp
 ✓ ├─ mobile    │ @6a3896d      │ —         │ =     │ propre                     │ sous-module ✓
 ● ├─ portail   │ @eb37fd7      │ —         │ =     │ propre                     │ décalé : le parent attend 6a3896d
 ✗ └─ webform   │ @0dad4ed      │ —         │ ↑1    │ propre                     │ 1 commit hors branche
                │               │           │       │                            │ décalé : le parent attend 6a3896d
────────────────┼───────────────┼───────────┼───────┼────────────────────────────┼───────────────────────────────────
 ✓ work/api     │ main          │ =         │ =     │ propre                     │
 ● work/app     │ feature/login │ =         │ ↑1 ↓4 │ stash 1                    │ non poussée(s) : experiment
 ● work/infra   │ main          │ ↓3        │ ↓3    │ propre                     │
 ● work/lib     │ feature/old   │ supprimée │ ↑1    │ propre                     │
 ● work/web     │ main          │ ↑2        │ ↑2    │ 1 nouveau                  │
────────────────┼───────────────┼───────────┼───────┼────────────────────────────┼───────────────────────────────────
   12 dépôts   ✗ 2 à risque   ● 7 à traiter   ✓ 3 en ordre   (159ms)
   à pousser 4 · à tirer 2 · modifiés 4 · sous-modules décalés 2
   ↑ à pousser · ↓ à tirer · = à jour · @ commit ou tag (pas de branche)
```

## Installation

```sh
go mod tidy                    # télécharge les dépendances (Bubble Tea…) — Go 1.24+
go build -o gitscan .
sudo mv gitscan /usr/local/bin # ou : go install .
```

## Mode interactif (`gitscan -i`)

```
gitscan  12 dépôts   ✗ 2 à risque   ● 7 à traiter   ✓ 3 en ordre   ~/code
      DÉPÔT        │ BRANCHE   │ vs SERVEUR │ vs MAIN │ LOCAL                 │ À VOIR
❯ ○ ● perso/blog   │ main      │ ↑1 ↓1   │ ↑1 ↓1 │ 1 modifié · 1 nouveau │
  ● ✗ serveur/site │ @c6e87cf  │ —       │ ↑1    │ 1 conflit             │ rebase en cours
  ○ ● ├─ portail   │ @eb37fd7  │ —       │ =     │ propre                │ décalé : le parent attend 6a3896d
  ○ ✗ └─ webform   │ @0dad4ed  │ —       │ ↑1    │ propre                │ 1 commit hors branche
1 sélectionné(s)
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
gitscan help                # comment lire le tableau (colonnes, flèches, messages)
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

## Lecture du tableau

`gitscan help` affiche cette légende dans le terminal.

**Icône** : `✓` en ordre · `●` à traiter · `✗` à risque (du travail peut se perdre, ou une opération est bloquée).

| Colonne | Contenu |
|---|---|
| DÉPÔT | chemin ; `├─ └─` = dépôt imbriqué (sous-module) rangé sous son parent |
| BRANCHE | branche actuelle, ou `@commit` (SHA court) en HEAD détachée ; les tags de ce commit sont listés dans À VOIR |
| vs SERVEUR | ta branche vs sa copie distante : `↑` à pousser, `↓` à tirer, `=` à jour ; `jamais poussée`, `supprimée`, `aucun` (pas de remote), `fetch ✗` |
| vs MAIN | ta branche vs `origin/main` (à défaut `origin/HEAD`, `origin/master`, puis la main locale) : `↑` commits en plus, `↓` commits de main manquants |
| LOCAL | non commité : `n modifiés` (`dont n droits` = seul le chmod a changé), `n nouveaux`, `n conflits`, `n sous-modules` décalés, `stash n` |
| À VOIR | le reste ; en terminal étroit, passe sur une ligne `↳` sous le dépôt |

| Signal (À VOIR) | Niveau | Signification |
|---|---|---|
| `rebase en cours` (merge, cherry-pick, revert, bisect) | risque | opération interrompue, à terminer ou annuler |
| `n commits hors branche` | risque | commits faits en HEAD détachée, sur aucune branche ni tag : perdables (`git branch sauvegarde` pour les garder) |
| `HEAD détachée` | à traiter | aucune branche (ne s'affiche pas pour un sous-module ou un tag, où c'est normal) |
| `décalé : le parent attend x` | à traiter | sous-module pas sur le commit enregistré par son dépôt parent |
| `sous-module ✓` | info | sous-module sur le commit attendu |
| `non poussée(s) : a, b` | à traiter | autres branches locales avec des commits absents du serveur (au-delà de 3 : `n branches non poussées`) |
| `tag sprint-33 (+3 autres sur ce commit)` | info | tags de ce commit, le plus récent d'abord ; `git describe` n'en montre qu'un, souvent le plus ancien |
| `2 commits après sprint-33` | info | aucun tag sur ce commit : distance au tag le plus proche |
| `suit origin/nbl, pas init-prd` | à traiter | la branche est reliée à une branche distante d'un autre nom : les ↑↓ de SERVEUR comparent à celle-là (`git branch -u origin/<branche>` pour corriger) |
| `fetch il y a …` | info | dernier fetch de plus de 7 jours : SERVEUR est peut-être périmé |

Les dépôts dont seuls les **droits** ont changé (cas fréquent sur un serveur après un `chmod -R`) sont signalés à part. Si c'est voulu : `git config core.fileMode false` dans le dépôt.

## Fonctionnement

- Découverte : parcours récursif ; un dossier contenant `.git` (dossier ou fichier, donc worktrees et submodules compris) est un dépôt. On ne descend pas dedans, sauf avec `-nested`.
- Sous-modules : détectés via `git rev-parse --show-superproject-working-tree`, puis comparés au commit enregistré par le parent (`git ls-tree HEAD`). Commits hors branche : `git rev-list HEAD --not --branches --remotes --tags`. Droits seuls : second `git status` avec `core.fileMode=false`.
- Chaque dépôt est analysé par un pool de goroutines. L'essentiel vient d'un seul `git status --porcelain=v2 --branch`, complété par `for-each-ref` (branches) et `rev-list --left-right --count` (comparaison avec main).
- La TUI suit l'architecture Elm de Bubble Tea : un modèle (état), `Update` (réagit aux touches et aux résultats git), `View` (dessine l'écran). Voir `tui.go`.
- Les commandes git tournent avec `GIT_TERMINAL_PROMPT=0` (jamais bloqué par une demande de mot de passe) et `GIT_OPTIONAL_LOCKS=0` (n'interfère pas avec un éditeur ouvert).

## Tester

`scripts/demo.sh /tmp/demo` crée des dépôts dans tous les états ci-dessus, puis `gitscan -b -c /tmp/demo/code`.

## Pistes pour la suite

- Dans la TUI : supprimer les branches fusionnées, changer de branche, `stash` / `stash pop`.
- Actions en masse : `gitscan pull --ff-only`, `gitscan prune-merged`.
- Fichier de config (`~/.config/gitscan.toml`) : dossiers par défaut, exclusions, branche principale par dépôt.
