# Références entre tableaux

*20/10/2026. Adoptée (demande de Louis : « Quand on fait validation de données sur une colonne et qu'on vise une autre colonne qui est en mode clé primaire, alors tu peux automatiquement créer une clé étrangère de l'autre côté. Comme ça, c'est facile de basculer sur la référence. On peut ajouter un lien « Basculer sur » pour basculer entre les tableaux. »). Code : `internal/canvas/colref.go`, `frontend/src/lib/grid/reflink.ts`, `Sheet.svelte` (`refAt`, `followRef`, `refMenuItems`), `+page.svelte` (`followReference`, `refGoBack`) ; essais : `colref_test.go`, `reflink.test.ts`, `api/validation_test.go` (`TestTableColumnRef`).*

## Règle

**La référence se déduit de la validation.** Une validation par liste dont la source est exactement la **colonne clé entière** d'un tableau (`Espèces[Nom]` quand `Nom` porte la 🔑 d'`Espèces`) fait de la cellule validée une référence vers ce tableau ; posée sur toute une colonne de tableau, la colonne est une **colonne de référence** (une clé étrangère). Ne le sont pas : une autre colonne (`Espèces[Famille]`), quelques lignes (`Espèces[Nom][1:5]`), une plage de cellules, un tableau sans clé.

**Rien n'est rangé de plus.** Pas de champ `ref` dans `_lattis_columns`, pas de migration du format : la source de la validation suffit. Il en découle :

- Ctrl+Z sur la validation, ou la validation retirée, défait la référence ;
- renommer le tableau visé ou sa colonne clé la suit (les sources des validations sont déjà réécrites, `rewriteValidations`) ;
- retirer la clé du tableau visé (ou la poser sur une autre colonne) défait la référence ; la remettre la rend ;
- un tableau visé supprimé : la source est en erreur, ce n'est plus une référence.

**Pas de contrainte FOREIGN KEY dans SQLite.** Les tableaux restent des grilles qu'on remplit, colle, trie et vide librement ; une contrainte refuserait un collage ou une suppression de ligne du tableau visé, sans dialogue possible. La validation refuse déjà à la saisie une valeur absente de la liste ; les valeurs qui deviennent orphelines (ligne visée supprimée) restent, et « Basculer sur » le dit.

## Interface

- **Message** quand la validation est posée : « « Espèce » devient une référence vers « Espèces » : Ctrl+clic ou clic droit › Basculer sur… ».
- **Basculer sur** une cellule de la colonne de référence qui a une valeur : `Ctrl+clic`, le bouton ↗ à droite de la flèche de la liste (cellule active seulement, pour ne pas charger la grille), ou clic droit › Basculer sur « Espèces » › Pie (en tête du menu). La feuille du tableau visé s'ouvre au besoin, la cellule clé de la ligne est sélectionnée et la ligne s'éclaire un instant. Valeur absente : l'entrée est grisée, son infobulle dit « « X » n'existe pas dans Espèces ». La recherche se fait comme la clé (sans casse pour un texte ou une date).
- **Revenir** : `Alt+←` (comme un navigateur) ou le bouton « ← Revenir à Suivi des nids!C5 » de la barre d'état, tant qu'il reste un retour. Pile de 20 retours, vidée au changement de classeur.
- **Lignes qui citent** (sens inverse) : clic droit sur une cellule de la colonne clé › Lignes qui citent « Pie » (3), un sous-menu par tableau quand plusieurs le citent. Le tableau qui cite est **filtré** sur la valeur (filtre par valeurs de sa colonne de référence, une étape d'annulation, « Effacer le filtre » pour tout revoir) et sa première ligne sélectionnée : c'est le comportement le plus utile avec l'API de filtre existante. Seules les colonnes de référence (validation de toute la colonne) comptent ici, pas des cellules isolées.
- **Scripts et MCP** : `tables.get` donne `ref = "Espèces[Nom]"` pour une colonne de référence.

## Laissé de côté

- L'onglet SQL ne montre pas la relation (il faudrait lire les validations de chaque colonne à chaque affichage de la liste des objets) ; `tables.get` suffit pour l'instant.
- Pas d'export vers Excel (les validations ne le sont pas).
