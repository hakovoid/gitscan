# gitscan — aide-mémoire

## Terminal (rapport)

```sh
gitscan                       # le dossier courant
gitscan ~/code                # un dossier et tous ses sous-dossiers
gitscan -f ~/code             # git fetch d'abord (chiffres à jour) — recommandé
gitscan -a ~/code             # seulement ce qui demande une action
gitscan -nested ~/code        # inclut les dépôts imbriqués (sous-modules)
gitscan -b ~/code             # + toutes les branches de chaque dépôt
gitscan -c ~/code             # + config (remotes, auteur, hooks, signature)
gitscan -i ~/code             # mode interactif
gitscan -changes -f ~/code    # seulement ce qui a changé depuis le dernier scan
gitscan -strict ~/code        # ignorer .gitscan : tout signaler
gitscan help                  # légende : icônes, colonnes, messages
gitscan -h                    # toutes les options
gitscan -version              # version installée
```

### Toutes les options

| Option | Rôle |
|---|---|
| `-f` | `git fetch --all --prune` avant l'analyse (en parallèle) |
| `-fetch-timeout 30s` | délai maximal d'un fetch |
| `-a` | n'affiche que les dépôts qui demandent une action |
| `-b` | liste toutes les branches locales (état vs upstream et vs main) |
| `-c` | affiche la configuration de chaque dépôt |
| `-i` | mode interactif (TUI) |
| `-nested` | cherche aussi des dépôts dans d'autres dépôts (sous-modules) |
| `-depth N` | profondeur maximale de recherche (0 = illimitée) |
| `-exclude a,b` | dossiers ignorés (défaut : `node_modules,vendor,.cache,.venv,venv,target`) |
| `-main nom` | force le nom de la branche principale |
| `-j N` | nombre de dépôts analysés en parallèle |
| `-json` | sortie JSON complète (branches + config) |
| `-check` | code de sortie 1 si un dépôt demande une action (utile en CI) |
| `-width N` | force la largeur du tableau (0 = celle du terminal) |
| `-color auto\|always\|never` | `always` garde les couleurs dans un tuyau |
| `-no-color` | désactive les couleurs (comme `NO_COLOR=1`), mode interactif compris |
| `-no-anim` | mode interactif sans animation (pas de spinner) |
| `-changes` | n'affiche que les changements depuis le dernier scan (rien si rien n'a changé) |
| `-no-save` | n'enregistre pas ce scan comme référence (implicite avec `-json` et `-check`, sauf avec `-changes`) |
| `-strict` | ignore le fichier `.gitscan` (états normaux) |
| `-normal fichier` | utilise ce fichier d'états normaux |

Les options se combinent : `gitscan -f -a -nested /var/www/eglise-ops/`

### Recettes utiles

```sh
# Vue complète d'un serveur, à jour
gitscan -f -nested /var/www/eglise-ops/

# Tableau large, page par page (← → pour défiler, q pour quitter)
gitscan -width 200 -color=always -nested ~/code | less -RS

# Sauvegarder l'état dans un fichier
gitscan -width 200 -nested ~/code > /tmp/etat.txt

# Dépôts avec des commits non poussés
gitscan -json ~/code | jq -r '.[] | select(.ahead > 0) | .path'

# Branches déjà fusionnées dans main (supprimables)
gitscan -json ~/code | jq -r '.[] | .path as $p | .branches[]
  | select(.merged_in_main and (.current|not)) | "\($p): \(.name)"'

# Sous-modules décalés du commit attendu par leur parent
gitscan -json -nested ~/code | jq -r '.[] | select(.submodule and (.sub_in_sync|not)) | .path'

# Alerte en CI : sort en erreur si un dépôt demande une action
gitscan -check -f ~/code

# Chaque matin (crontab -e) : un mail seulement si quelque chose a bougé
0 8 * * *  gitscan -f -changes -nested -color=never -width 120 /var/www
```

### États normaux : fichier `.gitscan`

À placer dans le dossier scanné (ou un parent). Une règle par ligne :

```
normal  serveur/docs   mode_only untracked   # droits et uploads : voulus
normal  */stopcom      detached              # figé sur un tag : normal
normal  archives/**    *                     # tout est normal ici
```

`*` = un nom, `**` = plusieurs niveaux, sans `/` = le nom à toute profondeur. Les codes : touche `i` (entre crochets) ou `gitscan help`. Jamais déclarables : conflits, commits hors branche, opération interrompue.

---

## Mode interactif (`gitscan -i`)

### Navigation

| Touche | Action |
|---|---|
| `↑` `↓` ou `k` `j` | monter / descendre |
| `pgup` `pgdown` | page précédente / suivante |
| `g` / `G` | début / fin de la liste |
| `entrée` | détail du dépôt (fichiers, branches, config, 15 derniers commits) |
| `b` | vue **branches** : lien avec le serveur, `u` pour relier, `espace` cocher (`a` toutes les fusionnées), `d` supprimer les cochées (ou celle sous le curseur), `⏎` pour voir les commits |
| `i` | **expliquer** les signaux du dépôt, avec les commandes git à lancer ; `e` y ouvre `.gitscan` avec une règle prête |
| `c` | ce qui a **changé** depuis le dernier scan |
| `échap` | revenir en arrière ; efface la recherche, puis la sélection, puis le filtre |
| `q` | quitter (`ctrl+c` aussi) |
| `?` | **tous les raccourcis** à l'écran ; marche depuis n'importe quelle vue, `↑` `↓` pour dérouler si l'écran est court |

### Sélection

| Touche | Action |
|---|---|
| `espace` ou `x` | cocher / décocher le dépôt |
| `a` | tout cocher (ou tout décocher) dans la vue affichée |

Sans sélection, les actions s'appliquent au dépôt **sous le curseur**.

### Actions git

| Touche | Commande lancée | Effet |
|---|---|---|
| `f` | `git fetch --all --prune` | met à jour les infos du serveur ; **ne touche à aucun fichier** |
| `p` | `git pull --ff-only` | met à jour la branche courante ; **refuse** s'il y a divergence, HEAD détachée ou pas d'upstream |
| `P` | `git push`, ou `git push -u origin HEAD` | envoie les commits ; **demande confirmation** (`o`/`entrée` = oui, `n`/`échap` = non) ; jamais de `--force` ; ne recrée jamais une branche distante supprimée |
| `D` | `git branch -D` sur les branches fusionnées | **demande confirmation** ; revérifie avant (tous les commits dans main) ; jamais main/master/develop/staging/prod ni la branche courante |
| `S` | `git submodule update --init -- <chemins>` | montre d'abord, pour chaque sous-module, s'il **avance** ou **recule** ; **demande confirmation** ; n'y touche pas s'il a des modifications ou des commits qui seraient perdus |
| `r` | — | ré-analyse la sélection |
| `R` | — | re-scanne tout le dossier |

Dans ces confirmations, chaque élément a une case cochée : `↑` `↓` puis `espace` pour décocher, `a` pour tout (dé)cocher, `o` pour lancer.

Les actions tournent en parallèle, avec un spinner par dépôt, puis le dépôt est ré-analysé. Le résultat reste affiché : `✓ push` ou `✗ pull : <erreur>`. Le détail (`entrée`) montre la sortie git complète.

### Affichage

| Touche | Action |
|---|---|
| `t` | n'afficher que les dépôts qui demandent une action |
| `o` | trier par nom ou par gravité |
| `+` / `-` | élargir / rétrécir la colonne BRANCHE (liste et vue branches), `0` revient à la largeur automatique ; un nom coupé sous le curseur s'affiche en entier dans la barre d'état |
| `/` | rechercher (chemin ou branche) ; `entrée` valide, `échap` efface |

### Sortir vers un autre outil

| Touche | Action |
|---|---|
| `s` | ouvrir un shell dans le dépôt (`exit` pour revenir) |
| `l` | ouvrir lazygit dans le dépôt (s'il est installé) |

---

## Lire le tableau

**Icône** : `✓` en ordre · `●` à traiter · `✗` à risque (travail perdable ou dépôt bloqué)

| Colonne | Contenu |
|---|---|
| DÉPÔT | chemin ; `├─ └─` = dépôt imbriqué sous son parent |
| BRANCHE | branche, ou `@a1b2c3d` (commit) si HEAD détachée |
| vs SERVEUR | ta branche vs sa copie sur le serveur |
| vs MAIN | ta branche vs `origin/main` (à défaut, la main locale) |
| LOCAL | non commité : modifiés, nouveaux, conflits, stash, sous-modules |
| À VOIR | le reste : opérations en cours, branches non poussées, tags… |

**Flèches** : `↑3` 3 commits à pousser · `↓90` 90 à récupérer · `↑3 ↓90` divergence · `=` identique · `—` pas de comparaison possible

**Couleurs** : rouge = risque · jaune = action à prévoir · gris = rien à faire ou information · cyan = branche · magenta = commit ou tag

### Vue branches (`b` dans le mode interactif)

| Touche | Action |
|---|---|
| `↑` `↓` | naviguer |
| `u` | relier la branche à la branche distante de même nom (`git branch -u`) |
| `d` | supprimer cette branche si elle est fusionnée dans main (confirmation) |
| `D` | supprimer toutes les branches fusionnées du dépôt (confirmation) |
| `⏎` | voir les commits de cette branche |
| `r` | rafraîchir |
| `esc` | retour |

Un `u` jaune en début de ligne marque les branches qu'il est possible de relier : sans upstream, ou reliées à une branche d'un autre nom. Un `d` gris marque celles qu'on peut supprimer sans rien perdre.

⚠️ Les chiffres `vs SERVEUR` et `vs MAIN` datent du dernier `git fetch` : utilise `-f` pour les rafraîchir.
