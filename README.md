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
- en option, **toutes les branches** (fusionnées ou non dans main) et la **configuration** (remotes, auteur, hooks, signature) ;
- **ce qui a changé depuis le dernier scan** ;
- en tenant compte des **états normaux** que tu déclares (fichier `.gitscan`), pour que `●` veuille toujours dire « à traiter ».

Deux modes : un **rapport** en ligne de commande (scriptable, JSON) et une **interface interactive** (`-i`) pour agir en lot : fetch, pull, push, ménage des branches fusionnées, remise des sous-modules au bon commit.

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
espace sélect. · a tout · f fetch · p pull · P push · ⏎ détail · b branches · i expliquer · c changements · t à traiter · / chercher · ? aide · q quitter
```

| Touche | Action |
|---|---|
| `↑ ↓` / `j k` | naviguer (`g`/`G` début/fin, pgup/pgdown) |
| `espace` / `x` | sélectionner ; `a` tout sélectionner ; `échap` efface |
| `f` | fetch --all --prune (sélection, ou dépôt sous le curseur) |
| `p` | pull **--ff-only** : jamais de merge implicite, git refuse s'il y a divergence |
| `P` | push, **avec confirmation** ; `-u origin HEAD` si la branche n'a jamais été poussée |
| `S` | sous-modules : les remettre au commit attendu par le parent — plan d'abord (avance / recule de n commits), puis confirmation |
| `D` | supprimer les branches fusionnées dans main, **avec confirmation** (voir « Ménage des branches ») |
| `r` / `R` | ré-analyser la sélection / re-scanner le dossier |
| `entrée` | détail : fichiers modifiés, branches, config, 15 derniers commits, sortie de la dernière action |
| `b` | vue branches : lien avec le serveur, `u` relie à la branche distante de même nom, `d` supprime une branche fusionnée, `⏎` montre ses commits |
| `i` | encadré d'explication : chaque signal du dépôt (avec son code), sa cause et les commandes git correspondantes ; `e` y ouvre `.gitscan` pour déclarer des signaux normaux |
| `c` | ce qui a changé depuis le dernier scan de ce dossier |
| `s` / `l` | ouvrir un shell / lazygit dans le dépôt (retour dans gitscan en quittant) |
| `t` | n'afficher que les dépôts qui demandent une action |
| `o` | trier par nom ou par gravité |
| `/` | rechercher par chemin ou branche |
| `?` | la liste complète des raccourcis, depuis n'importe quelle vue (`↑` `↓` pour dérouler) |
| `q` | quitter |

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
gitscan -changes -f ~/code  # seulement ce qui a changé depuis le dernier scan
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
| `-color auto\|always\|never` | `always` garde les couleurs dans un tuyau : `gitscan ~/code -color=always \| less -R` |
| `-width N` | force la largeur du tableau (utile en pipe, où gitscan ne connaît pas la largeur) |
| `-changes` | n'affiche que les changements depuis le dernier scan, et **rien** s'il n'y en a pas |
| `-no-save` | n'enregistre pas ce scan comme référence pour le suivant |
| `-strict` | ignore le fichier `.gitscan` : tout est signalé |
| `-normal fichier` | utilise ce fichier d'états normaux au lieu de chercher `.gitscan` |

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
| `main locale ↓12 vs origin/main` | info | ta branche `main` locale est en retard sur celle du serveur (gitscan compare toujours à la main **distante** quand elle existe) |
| `2 commits après sprint-33` | info | aucun tag sur ce commit : distance au tag le plus proche |
| `déjà sur origin/x : git branch -u origin/x pour la relier` | info | la branche est sur le serveur sous le même nom mais sans upstream ; `vs SERVEUR` la compare alors à cette branche et affiche `(sans upstream)` |
| `suit origin/nbl, pas init-prd` | à traiter | la branche est reliée à une branche distante d'un autre nom : les ↑↓ de SERVEUR comparent à celle-là (`git branch -u origin/<branche>` pour corriger) |
| `n branches fusionnées dans main, supprimables` | info | branches locales dont tous les commits sont dans main (touche `D` du mode interactif) |
| `fetch il y a …` | info | dernier fetch de plus de 7 jours : SERVEUR est peut-être périmé |

Les dépôts dont seuls les **droits** ont changé (cas fréquent sur un serveur après un `chmod -R`) sont signalés à part. Si c'est voulu : `git config core.fileMode false` dans le dépôt.

## États normaux (`.gitscan`)

Sur un serveur, certains signaux sont attendus : droits modifiés après un déploiement, dossier d'uploads non suivi, version figée sur un tag… À force, on ne voit plus les `●` qui comptent. Un fichier `.gitscan`, placé dans le dossier scanné (ou un de ses parents), les déclare normaux :

```
# normal  <dossier>  <signal> [<signal>…]
normal  serveur/docs    mode_only untracked
normal  */stopcom       detached
normal  archives/**     *
```

- `<dossier>` est relatif au fichier : `*` remplace un nom, `**` plusieurs niveaux ; sans `/`, le motif vise le nom du dossier à n'importe quelle profondeur.
- `<signal>` est un code : la touche `i` du mode interactif les affiche entre crochets, `gitscan help` les liste tous, `*` les prend tous.
- Un signal déclaré normal passe en « info » : le dépôt peut redevenir `✓`, il sort de `-a`, `-check` et de la colonne À VOIR, mais reste visible avec `i` et dans le détail (« (normal) »). Le résumé dit combien de signaux sont concernés.
- **Jamais déclarables** : conflits, commits hors branche, opération interrompue, erreur. Le tag d'une HEAD détachée reste toujours affiché.
- `-strict` ignore le fichier. Dans le mode interactif, `i` puis `e` ouvre `.gitscan` (créé au besoin) avec une règle prête pour le dépôt, en commentaire, puis recharge.

## Depuis le dernier scan

Chaque scan est enregistré dans `~/.cache/gitscan` (un fichier par dossier scanné et par jeu d'options `-nested` / `-depth` / `-exclude`) et comparé au suivant. Sous le tableau :

```
   Depuis le dernier scan (il y a 3 j)
   −      perso/notes      disparu (supprimé, déplacé ou exclu)
          serveur/stopcom  tag sprint-33 → tag sprint-34
   ● → ✓  work/infra       HEAD f23c379 → e243ccb · réglé : à tirer ↓3
          work/web         à pousser ↑2 → à pousser ↑3
```

`gitscan -changes` n'affiche que cette partie, et rien du tout quand rien n'a changé : en tâche planifiée, cron n'envoie un mail que s'il y a une sortie.

```sh
# chaque matin à 8 h : fetch, puis mail seulement si quelque chose a bougé
0 8 * * *  gitscan -f -changes -nested -color=never -width 120 /var/www
```

Dans le mode interactif, un message annonce les changements au démarrage et `c` les montre ; l'état est enregistré en quittant.

## Ménage des branches

gitscan repère les branches locales dont **tous les commits sont déjà dans main** : les supprimer ne perd rien. Elles sont signalées en gris (`n branches fusionnées dans main, supprimables`). Dans le mode interactif, `D` les supprime pour la sélection (ou le dépôt sous le curseur), `d` en supprime une depuis la vue branches, toujours après confirmation.

- Jamais proposées : la branche courante, la main locale, une branche ouverte dans un autre worktree, et `main`, `master`, `develop`, `dev`, `staging`, `preprod`, `prod`, `production`.
- Juste avant de supprimer, gitscan revérifie chaque branche (`git merge-base --is-ancestor`), puis affiche son dernier commit : `git branch <nom> <commit>` la recrée.
- Une branche fusionnée par *squash* n'est pas un ancêtre de main : gitscan ne peut pas prouver qu'elle est sans risque et ne la propose pas.
- Une branche créée depuis main sans aucun commit propre est aussi « fusionnée » : la supprimer ne perd rien, mais regarde la liste avant de confirmer.

## Sous-modules : remettre au commit attendu

Quand des sous-modules ne sont pas sur le commit qu'enregistre leur parent, `S` (mode interactif) calcule d'abord ce que ferait `git submodule update` pour chacun : commit actuel → attendu, et s'il **avance** ou **recule** de n commits. Rien n'est fait avant confirmation.

- Sélectionner le parent vise tous ses sous-modules décalés ; sélectionner un sous-module ne vise que lui.
- Bloqués (jamais touchés) : un sous-module avec des modifications non commitées, ou dont des commits ne sont sur aucune branche (ils seraient perdus).
- Si le parent est lui-même en retard sur le serveur, gitscan le signale : il attend peut-être d'anciennes versions, et le bon ordre est souvent `p` (pull) sur le parent, puis `S`.
- Après l'opération, les sous-modules sont en HEAD détachée sur le commit attendu, comme après un clone ; les commits quittés restent sur leur branche.

## Fonctionnement

- Découverte : parcours récursif ; un dossier contenant `.git` (dossier ou fichier, donc worktrees et submodules compris) est un dépôt. On ne descend pas dedans, sauf avec `-nested`.
- Sous-modules : détectés via `git rev-parse --show-superproject-working-tree`, puis comparés au commit enregistré par le parent (`git ls-tree HEAD`). Commits hors branche : `git rev-list HEAD --not --branches --remotes --tags`. Droits seuls : second `git status` avec `core.fileMode=false`.
- Chaque dépôt est analysé par un pool de goroutines. L'essentiel vient d'un seul `git status --porcelain=v2 --branch`, complété par `for-each-ref` (branches) et `rev-list --left-right --count` (comparaison avec main).
- La TUI suit l'architecture Elm de Bubble Tea : un modèle (état), `Update` (réagit aux touches et aux résultats git), `View` (dessine l'écran). Voir `tui.go`.
- Les commandes git tournent avec `GIT_TERMINAL_PROMPT=0` (jamais bloqué par une demande de mot de passe) et `GIT_OPTIONAL_LOCKS=0` (n'interfère pas avec un éditeur ouvert).

## Tester

```sh
go test ./...                # tests automatisés, sur de vrais dépôts git créés à la volée
scripts/demo.sh /tmp/demo    # dépôts de démonstration dans tous les états ci-dessus
gitscan -b -c -nested /tmp/demo/code
```

Chaque test crée un « serveur » (dépôt nu) et des clones dans un dossier temporaire, provoque une situation (branche poussée sans `-u`, HEAD détachée sur tags, sous-module qui reculerait…) et vérifie ce que gitscan en conclut. Les bogues déjà rencontrés y ont chacun leur test.

## Pistes pour la suite

- Changer de branche depuis la vue branches, `stash` / `stash pop`.
- Branches protégées configurables dans `.gitscan` (aujourd'hui : liste fixe).
- Instantanés : garder plusieurs scans pour voir l'évolution sur une semaine.
