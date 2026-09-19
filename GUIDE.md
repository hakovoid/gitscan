# gitscan : l'état de tous tes dépôts git, en un coup d'œil

## C'est quoi ?

Quand on a beaucoup de projets git, on finit par oublier des choses : un commit jamais poussé, une branche en retard sur `main`, des fichiers modifiés dans un coin…

**gitscan** parcourt un dossier et tous ses sous-dossiers, trouve chaque dépôt git et te dit pour chacun :

- sur quelle **branche** tu es ;
- s'il y a des commits **à pousser** (↑) ou **à tirer** (↓) ;
- où tu en es par rapport à **main** ;
- s'il y a des **fichiers modifiés** ou non commités ;
- s'il y a des **branches oubliées**, jamais poussées.

Il a deux modes :

- **un rapport** : un tableau affiché dans le terminal ;
- **une interface interactive** : tu navigues au clavier et tu lances fetch, pull ou push sur plusieurs dépôts d'un coup.

```
DÉPÔT        BRANCHE        REMOTE    VS MAIN  ÉTAT
perso/blog   main           ↑1 ↓1     ↑1 ↓1    modifié 1, divergé ↑1 ↓1
work/api     main           =         =        ✓ propre
work/infra   main           ↓3        ↓3       à tirer ↓3
work/web     main           ↑2        ↑2       à pousser ↑2
```

---

## Installation

### 1. Il te faut Go (version 1.24 ou plus) et git

Vérifie :

```sh
go version
git --version
```

Si Go manque, installe-le depuis <https://go.dev/dl/>.

### 2. Compiler gitscan

```sh
tar xzf gitscan.tar.gz      # si tu pars de l'archive
cd gitscan
go mod tidy                 # télécharge les dépendances
go build -o gitscan .       # crée le programme
```

### 3. Le rendre disponible partout

```sh
mkdir -p ~/.local/bin
cp gitscan ~/.local/bin/
```

Si la commande `gitscan` n'est pas trouvée ensuite, ajoute cette ligne à ton `~/.bashrc` (ou `~/.zshrc`), puis rouvre le terminal :

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Vérifie : `gitscan -version`

---

## Utilisation

### Voir l'état de tous tes projets

```sh
gitscan ~/code
```

Remplace `~/code` par le dossier qui contient tes projets.

💡 **Conseil** : ajoute `-f` pour récupérer d'abord les nouveautés des serveurs (`git fetch`). Sans ça, gitscan compare avec ce qu'il connaissait lors du dernier fetch.

```sh
gitscan -f ~/code
```

### Les options utiles

| Commande | Ce qu'elle fait |
|---|---|
| `gitscan -a ~/code` | n'affiche **que** les dépôts qui demandent une action |
| `gitscan -b ~/code` | affiche aussi **toutes les branches** de chaque dépôt |
| `gitscan -c ~/code` | affiche aussi la **configuration** (remote, auteur, hooks) |
| `gitscan -json ~/code` | sortie JSON, pour les scripts |

Tu peux les combiner : `gitscan -f -a ~/code`

### Le mode interactif

```sh
gitscan -i ~/code
```

Tu te déplaces dans la liste, tu coches des dépôts, puis tu appuies sur une touche pour agir.

| Touche | Action |
|---|---|
| `↑` `↓` | se déplacer |
| `espace` | cocher / décocher un dépôt |
| `a` | tout cocher |
| `f` | **fetch** : récupérer les nouveautés |
| `p` | **pull** : mettre à jour le dépôt |
| `P` | **push** : envoyer tes commits (demande confirmation) |
| `entrée` | voir le **détail** du dépôt (fichiers, branches, commits) |
| `échap` | revenir en arrière |
| `t` | n'afficher que ce qui demande une action |
| `/` | rechercher un dépôt |
| `s` | ouvrir un terminal dans le dépôt |
| `?` | aide |
| `q` | quitter |

Si aucun dépôt n'est coché, l'action s'applique au dépôt sous le curseur.

---

## Comprendre ce qui s'affiche

| Tu vois | Ça veut dire | Quoi faire |
|---|---|---|
| `✓ propre` | tout est à jour | rien 🎉 |
| `à pousser ↑2` | 2 commits pas encore envoyés | `P` (ou `git push`) |
| `à tirer ↓3` | 3 nouveaux commits sur le serveur | `p` (ou `git pull`) |
| `divergé ↑1 ↓1` | tu as des commits, le serveur aussi | `git pull --rebase` à la main |
| `modifié 2` | fichiers modifiés, non commités | commiter ou annuler |
| `non suivi 1` | nouveaux fichiers jamais ajoutés à git | `git add` ou les ignorer |
| `jamais poussée` | la branche n'existe pas encore sur le serveur | `P` |
| `upstream supprimé` | la branche a été supprimée du serveur (souvent après une PR fusionnée) | supprimer la branche locale |
| `retard main ↓4` | `main` a avancé depuis ta branche | mettre ta branche à jour si besoin |
| `stash 1` | du travail mis de côté avec `git stash` | le récupérer ou le supprimer |

Couleurs : **rouge** = problème · **jaune** = action à faire · **gris** = pour info.

---

## Bon à savoir

- **gitscan ne modifie rien tout seul.** Le rapport se contente de lire. En mode interactif, seules les touches `f`, `p` et `P` agissent.
- **Le pull est sans risque** : si une fusion était nécessaire, gitscan refuse et te laisse faire à la main.
- **Le push demande toujours confirmation.**
- **Clé SSH avec mot de passe ?** Lance `ssh-add` avant d'ouvrir gitscan, sinon push et pull échouent (proprement, avec un message).
- Les dossiers `node_modules`, `vendor`, `.venv`… sont ignorés pour aller plus vite.
