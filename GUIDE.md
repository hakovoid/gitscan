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
   DÉPÔT        │ BRANCHE       │ SERVEUR   │ MAIN  │ LOCAL                      │ À VOIR
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

### Comprendre le tableau depuis le terminal

```sh
gitscan help
```

Explique l'icône en début de ligne, chaque colonne, les flèches et chaque message.

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

Tape `gitscan help` pour avoir cette explication dans le terminal.

### L'icône en début de ligne

| Icône | Signification |
|---|---|
| `✓` vert | en ordre, rien à faire |
| `●` jaune | une action est à prévoir |
| `✗` rouge | **à risque** : du travail peut se perdre, ou une opération est bloquée |

### Les colonnes

| Colonne | Ce qu'elle dit |
|---|---|
| **DÉPÔT** | le dossier du projet. `├─` et `└─` : projet rangé dans un autre (sous-module) |
| **BRANCHE** | la branche sur laquelle tu es. `@a1b2c3d` : tu es sur ce commit, pas sur une branche (ses tags sont dans À VOIR) |
| **SERVEUR** | ta branche comparée au serveur : `↑2` à envoyer, `↓3` à récupérer, `=` à jour |
| **MAIN** | ta branche comparée à `main` |
| **LOCAL** | ce que tu n'as pas encore commité |
| **À VOIR** | tout le reste qui mérite ton attention |

### Les messages et quoi faire

| Tu vois | Ça veut dire | Quoi faire |
|---|---|---|
| `↑2` (SERVEUR) | 2 commits pas encore envoyés | `P` (ou `git push`) |
| `↓3` (SERVEUR) | 3 nouveaux commits sur le serveur | `p` (ou `git pull`) |
| `↑1 ↓1` en rouge | tu as des commits, le serveur aussi | `git pull --rebase` à la main |
| `jamais poussée` | la branche n'existe pas sur le serveur | `P` |
| `supprimée` | la branche a été effacée du serveur (souvent après une PR fusionnée) | supprimer la branche locale |
| `2 modifiés` | fichiers modifiés, non commités | commiter ou annuler |
| `dont 2 droits` | pour ces fichiers, seuls les droits (chmod) ont changé, pas le contenu | rien, ou `git config core.fileMode false` |
| `1 nouveau` | fichier jamais ajouté à git | `git add`, ou l'ignorer |
| `stash 1` | du travail mis de côté avec `git stash` | le récupérer ou le supprimer |
| `rebase en cours` ✗ | une opération git a été interrompue | la terminer (`git rebase --continue`) ou l'annuler (`--abort`) |
| `1 commit hors branche` ✗ | un commit fait sans branche : **il peut se perdre** | `git branch sauvegarde` pour le garder |
| `décalé : le parent attend …` | le sous-module n'est pas sur la version prévue par le projet parent | `git submodule update` depuis le parent (si c'est voulu) |
| `sous-module ✓` | le sous-module est sur la bonne version | rien |
| `non poussée(s) : dev, fix` | ces branches ont des commits qui ne sont pas sur le serveur | les pousser, ou les supprimer si inutiles |
| `tag sprint-33 (+3 autres)` | ce commit porte ce tag, et 3 autres tags pointent dessus | rien, si c'est la version voulue |
| `2 commits après sprint-33` | aucun tag sur ce commit : il est 2 commits après le dernier tag | rien |
| `suit origin/nbl, pas init-prd` | ta branche est reliée à une branche distante qui porte un autre nom : les ↑↓ comparent avec celle-là | `git branch -u origin/<la bonne branche>` |

Couleurs : **rouge** = risque · **jaune** = à faire · **gris** = info · **vert** = bon · **cyan** = branche · **magenta** = commit ou tag.

---

## Bon à savoir

- **gitscan ne modifie rien tout seul.** Le rapport se contente de lire. En mode interactif, seules les touches `f`, `p` et `P` agissent.
- **Le pull est sans risque** : si une fusion était nécessaire, gitscan refuse et te laisse faire à la main.
- **Le push demande toujours confirmation.**
- **Clé SSH avec mot de passe ?** Lance `ssh-add` avant d'ouvrir gitscan, sinon push et pull échouent (proprement, avec un message).
- Les dossiers `node_modules`, `vendor`, `.venv`… sont ignorés pour aller plus vite.
