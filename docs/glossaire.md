# Glossaire et règles d'écriture

Ce document fixe les mots de l'interface, de l'aide intégrée (`frontend/src/lib/help/pages`), des messages du moteur et de la documentation. Un terme nouveau s'ajoute ici avant d'apparaître dans l'interface. Les écarts connus sont listés à la fin.

## Typographie

| Règle | Exemple | À éviter |
|---|---|---|
| Apostrophe droite `'` partout (interface, aide, messages Go, documentation). En TypeScript, une chaîne qui en contient s'écrit entre guillemets doubles (`"l'onglet"`), comme le ferait Prettier. | `"Nom de l'onglet"` | `'Nom de l’onglet'` |
| Guillemets français « … » avec une espace de chaque côté du texte, pour citer un nom, une valeur ou un libellé. | `le tableau « Ventes »` | `"Ventes"`, `“Ventes”` |
| Points de suspension en un seul caractère `…`, à la fin d'un libellé qui ouvre une boîte de dialogue. | `Publier sur un serveur…` | `Publier sur un serveur...` |
| Chemin de menu avec `›` entouré d'espaces. | `Fichier › Paramètres` | `Fichier > Paramètres` |
| Touches à la française, reliées par `+` sans espace. | `Ctrl+Maj+L`, `Échap`, `Entrée`, `Suppr` | `Ctrl+Shift+L`, `Esc`, `Enter` |
| Libellés en casse de phrase : majuscule au premier mot seulement, sauf nom propre et sigle. | `Mise en forme conditionnelle` | `Mise En Forme Conditionnelle` |
| Noms d'onglet du ruban en casse de titre ; un sigle reste en capitales. | `Accueil`, `Données`, `Script`, `SQL` | `SCRIPT` |
| Nombres et dates au format français. | `12,50 €`, `19/10/2026` | `12.50`, `2026-10-19` (sauf SQL) |

## Termes

| Terme | Définition | Ne pas dire |
|---|---|---|
| **classeur** | Le fichier `.lattis` : une base SQLite et sa mise en page. | fichier Lattis, document (sauf l'onglet Document) |
| **feuille** | Un onglet de grille du classeur. | page, onglet de feuille (sauf pour désigner l'onglet lui-même) |
| **onglet** | Ce qu'on choisit dans la barre du bas (feuille, page web, document) ou dans le ruban (Accueil, Insertion…, SQL, Script). | |
| **tableau** | Une plage structurée de la grille, qui est une vraie table SQLite typée. On l'interroge par son nom en SQL et dans les formules (`Ventes[Prix]`). | table (sauf en SQL), liste |
| **cellule libre** | Une cellule hors de tout tableau, adressée en A1 (`Cell[B3]` dans une formule). | cellule orpheline |
| **colonne calculée** | Une colonne de tableau définie par une formule, la même pour toutes les lignes. | colonne formule |
| **type fixé / type inféré** | Le type d'une colonne choisi par l'utilisateur, ou déduit des valeurs saisies. | type forcé, type auto |
| **clé** | La colonne qui identifie une ligne d'un tableau (pour `REFERENCE`, `RECHERCHEV`, les liens). | identifiant, index |
| **tableau croisé dynamique** | Synthèse d'un tableau par `GROUP BY`. « TCD » s'emploie dans un texte suivi après une première mention en toutes lettres, et dans les noms par défaut (`TCD1`, qui s'écrit tel quel en SQL). Les titres, menus et infobulles disent « tableau croisé dynamique » ou « tableau croisé ». | pivot |
| **requête** | Une instruction SQL (onglet SQL) ou une requête d'import (sources externes, étapes de transformation). | query |
| **connexion** | Les paramètres d'accès à une base ou à un service externe (PostgreSQL, MySQL, SQL Server, web, Serveur Lattis). | source de données (réservé à l'ensemble des connexions et fichiers importés) |
| **Serveur Lattis** | Le serveur d'une organisation (`lattisd`), où l'on se connecte par le menu Compte. Nom propre : majuscules aux deux mots. | serveur lattis, backend |
| **classeur partagé** | Un classeur publié sur un Serveur Lattis (Fichier › Publier sur un serveur…), figé ou collaboratif, rouvert depuis le serveur. Nom commun : minuscule, sauf en début de libellé. | classeur en ligne, classeur distant |
| **Data Lake Lattis** | Le catalogue des données d'un Serveur Lattis : des tableaux partagés pour être lus depuis d'autres classeurs (Insertion › Data Lake Lattis, clic droit › Partager au Data Lake Lattis…). Nom propre de la fonction, choisi tel quel (commit b3dc7bb) ; une « donnée du Data Lake » est un de ses tableaux. | donnée d'entreprise (ancien nom) |
| **page web** | Un onglet HTML, CSS et JavaScript qui lit et modifie le classeur, ou qui affiche un site. | onglet HTML (sauf dans les raccourcis) |
| **script** | Un programme Lua de la bibliothèque du classeur (onglet Script). | macro |
| **contrôle** | Un bouton, une case, une liste… posé sur la grille ou dans un document, lié à une cellule et à un script. | widget |
| **assistant** | L'assistant IA (bouton « Assistant ✦ »). | IA, chatbot, agent |
| **volet** | Une zone ancrée sur le côté de la fenêtre (Aide, Assistant, Champs de tableau croisé dynamique). | panneau, panel (« panneau » reste admis pour un menu flottant de la grille, comme le filtre d'une colonne) |
| **boîte de dialogue** | Une fenêtre modale. | popup, modale |
| **barre d'état** | La ligne du bas qui affiche les messages et la somme de la sélection. | statusbar |
| **zone de nom** | La case à gauche de la barre de formule (adresse ou nom de la cellule active). | name box |
| **Annuler / Rétablir** | Ctrl+Z, Ctrl+Y. | Défaire, Refaire |

## Écarts connus

Corrigés avec ce glossaire (audit du 19/10/2026, M10) : l'onglet « SCRIPT » devient « Script » partout (ruban, aide, messages, documentation) ; le nom affiché par défaut d'un tableau croisé sans nom ; « Refaire » de l'éditeur de requête devient « Rétablir » ; l'apostrophe typographique des fichiers `.ts` hors grille, rubans et assistant ; un commentaire périmé sur les adresses A1 (`internal/transfer/xlsxformula.go`) ; les erreurs de connexion courantes de PostgreSQL, MySQL et SQL Server sont traduites (`internal/query/driverfr.go`).

Restent à reprendre, au fil des modifications de ces fichiers :

- l'apostrophe typographique `’` dans les composants `.svelte` (environ 270 occurrences, surtout `Ribbon.svelte`, `Sheet.svelte`, `RightsDialog.svelte` et les volets de l'assistant) et dans les fichiers `.ts` de `grid/`, `chart/` et `ai/` ;
- les erreurs des pilotes en dehors de la connexion (exécution d'une requête sur une base externe), qui restent en anglais ;
- l'identifiant interne `'SCRIPT'` du ruban (`Ribbon.svelte`, `keytips.ts`), sans effet visible.
