```text
 ██████╗ ██╗████████╗███████╗ ██████╗ █████╗ ███╗   ██╗
██╔════╝ ██║╚══██╔══╝██╔════╝██╔════╝██╔══██╗████╗  ██║
██║  ███╗██║   ██║   ███████╗██║     ███████║██╔██╗ ██║
██║   ██║██║   ██║   ╚════██║██║     ██╔══██║██║╚██╗██║
╚██████╔╝██║   ██║   ███████║╚██████╗██║  ██║██║ ╚████║
 ╚═════╝ ╚═╝   ╚═╝   ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═══╝
```

# L'état de tous tes dépôts git, en un coup d'œil

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

Si `go version` affiche une erreur ou une version **inférieure à 1.24**, installe Go comme expliqué ci-dessous.

#### Installer Go sur Linux (serveur, devbox)

⚠️ N'utilise pas `apt install golang` : la version des distributions est souvent trop ancienne.

**Le plus simple : le script fourni** (marche aussi sur macOS).

```sh
sh scripts/install-go.sh           # installe dans /usr/local/go (demande sudo si besoin)
sh scripts/install-go.sh --user    # ou dans ~/.local/go, sans sudo
```

Il détecte ton système, télécharge la dernière version, vérifie le fichier, l'installe et règle le PATH. Relance-le plus tard pour mettre Go à jour : il ne fait rien si tu as déjà la dernière version.

**À la main**, si tu préfères : copie-colle ces commandes. Elles téléchargent la dernière version officielle et l'installent dans `/usr/local/go`.

```sh
# 1. Trouver la dernière version et le type de processeur
V=$(curl -s 'https://go.dev/VERSION?m=text' | head -1)
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

# 2. Télécharger et installer
curl -LO "https://go.dev/dl/$V.linux-$ARCH.tar.gz"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "$V.linux-$ARCH.tar.gz"
rm "$V.linux-$ARCH.tar.gz"

# 3. Ajouter Go au PATH (une seule fois)
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc
```

> Tu utilises zsh ? Remplace `~/.bashrc` par `~/.zshrc`.

**Pas d'accès `sudo` ?** Installe Go dans ton dossier personnel :

```sh
mkdir -p ~/.local
tar -C ~/.local -xzf "$V.linux-$ARCH.tar.gz"
echo 'export PATH=$PATH:$HOME/.local/go/bin:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc
```

#### Installer Go sur macOS

Avec [Homebrew](https://brew.sh) :

```sh
brew install go
```

Sinon, télécharge le fichier `.pkg` sur <https://go.dev/dl/> et double-clique dessus.

#### Installer Go sur Windows

Télécharge le fichier `.msi` sur <https://go.dev/dl/> et lance-le. Pour gitscan, le plus simple reste d'utiliser WSL (Linux sous Windows) et de suivre les instructions Linux.

#### Vérifier

Ouvre un **nouveau** terminal, puis :

```sh
go version        # doit afficher go1.24 ou plus
```

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
