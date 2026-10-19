# Découper `Sheet.svelte` et tester l'interface

*Plan, 19/10/2026 (audit 6, I12). Rien n'est encore découpé : ce document fixe l'ordre et la méthode pour le faire par petites étapes, chacune livrable seule, sans changer le comportement.*

## 1. Point de départ

`frontend/src/lib/grid/Sheet.svelte` fait environ 10 400 lignes : 320 fonctions, 65 `$state`, 11 `$effect`, un gabarit d'environ 1 300 lignes (lignes 8 260 à 9 550) et 850 lignes de styles. Les calculs purs en sont déjà sortis dans une quarantaine de modules `.ts` testés (`axis.ts`, `clipboard.ts`, `fill.ts`, `formulaEdit.ts`, `scroll.ts`…). Ce qui reste mêle trois choses : l'état de la grille (sélection, édition, cache des tableaux), le dessin sur le canvas, et une douzaine de menus, panneaux et boîtes de dialogue posés par-dessus.

Le fichier est découpé par des bandeaux `// --- Titre ---`. Les zones les plus grosses :

| Zone (bandeau) | Lignes environ | Nature |
|---|---|---|
| Tableaux posés sur la feuille, chargement, cache | 620 à 1 160 | état central |
| Dessin (cellules, en-têtes, sélection, objets) | 1 160 à 3 100 | canvas |
| Format de cellule, validation, notes | 3 490 à 4 090 | menus et boîtes |
| Filtres (tableau et TCD), mise en forme conditionnelle | 4 090 à 4 480 | panneaux |
| Graphiques, tableau croisé, ruban Formules, audit | 4 480 à 5 340 | objets et commandes |
| Sélection, clavier, souris, recopie, raccourcis | 5 340 à 6 645 | interaction |
| Édition, zone de nom, rechercher, presse-papiers | 6 645 à 7 790 | interaction |
| Type de données, menu contextuel, mesures | 7 790 à 8 260 | menus |
| Gabarit : menus contextuels, panneaux de filtre, notes, validation | 8 260 à 9 550 | balisage |

## 2. Méthode

1. **D'abord des modules `.ts` purs**, comme jusqu'ici : une fonction qui ne lit que ses arguments sort sans risque et se teste en Node. C'est le gros du gain restant dans les zones « interaction ».
2. **Puis des composants enfants pour les surfaces posées sur la grille**, sur le modèle déjà en place de `CondFormat.svelte` (interface `CondHost`) et `SheetButtons.svelte` : le composant reçoit un petit objet « hôte » (lecture de l'état utile, commandes) et ne voit jamais les variables internes de la feuille. Un composant par menu ou panneau.
3. **L'état central reste dans `Sheet.svelte`** tant que le dessin y est : déplacer la sélection ou le cache des tableaux dans un module `.svelte.ts` ne se fera qu'en dernier, quand le reste sera sorti.
4. **Une étape = un commit**, qui ne change aucun comportement, avec `npm run check`, `vitest` et un passage manuel (saisie, menus, Ctrl+Z) ; jamais en même temps qu'une correction.
5. **Pas de refonte en parallèle d'un autre chantier sur la grille** : annoncer l'étape, la faire courte (une demi-journée au plus) pour limiter les conflits de fusion.

## 3. Ordre proposé

| Étape | Sortie | Contenu | Interface avec la feuille |
|---|---|---|---|
| 1 | `NoteDialog.svelte`, `NoteTip.svelte` | Notes : boîte d'édition, post-it au survol, épinglage | `{ note(r, c), setNote, close }` |
| 2 | `ValidationDialog.svelte` | Validation des données (liste déroulante et règles) | `{ rule(sel), apply, close }` |
| 3 | `FilterPanel.svelte` | Panneau ▾ d'une colonne : tri, valeurs, conditions, type de données | `{ column, values(), setFilter, sort, setType }` |
| 4 | `PivotFilter.svelte` | Filtres d'un TCD dans la grille | `{ pivot, field, setFilter }` |
| 5 | `CellMenu.svelte`, `TableMenu.svelte`, `HeaderMenu.svelte` | Menus contextuels (cellule, tableau, en-têtes de feuille) ; les entrées restent décrites par des données, le rendu par le composant | `{ items, run(id), close }` |
| 6 | `FindBar.svelte` | Rechercher et remplacer | `{ find, replace, zone }` |
| 7 | `FormulaBar.svelte` | Zone de nom, barre de formule, aide de signature et complétion | `{ nameBox, fxText, commit, cancel }` |
| 8 | `editing.ts`, `keyboard.ts`, `mouse.ts` | Logique de saisie, de raccourcis et de glisser, en fonctions pures (état en entrée, actions en sortie) | appelées par `onKeyDown`, `onMouseDown`… |
| 9 | `draw/*.ts` | Dessin du canvas par couche (cellules, en-têtes, sélection, objets), sur un contexte et une vue | appelées par `paint()` |
| 10 | `sheetState.svelte.ts` | Sélection, édition, tableaux chargés : l'état central, s'il reste utile de le sortir | importé par la feuille et les composants |

Les étapes 1 à 7 retirent environ 3 000 lignes de gabarit et de logique de menus ; 8 et 9 en retirent autant de logique. Objectif : `Sheet.svelte` sous 4 000 lignes, chaque composant sous 600.

Couche d'accessibilité : la cellule active et la région `aria-live` sont déjà dans `GridA11y.svelte` (audit 6, I11), alimentées par `refreshSpeech` ; ce composant suit la même méthode.

## 4. Tests

### Ce qui existe

- Les modules `.ts` extraits : plus de 85 fichiers vitest.
- **Tests de composant par rendu serveur** (`render` de `svelte/server`, sans navigateur ni dépendance de plus) : `grid/a11y.test.ts` (`GridA11y.svelte`), `help/Markdown.test.ts` (toutes les pages de l'aide, et le glossaire), `ui/PanelHeader.test.ts`. Ils vérifient le balisage produit (rôles ARIA, libellés, échappement), pas les interactions.

### Ce qui manque, et ce qu'il faudrait ajouter

| Besoin | Outil | Dépendances nouvelles |
|---|---|---|
| Interactions d'un composant (clic, clavier, focus) | `@testing-library/svelte` sur `jsdom` ou `happy-dom` | 2 paquets de développement |
| Parcours de bout en bout dans l'application réelle | Playwright contre la version web | `@playwright/test` et un navigateur téléchargé en CI |

Aucune de ces dépendances n'est dans `package.json` aujourd'hui ; les ajouter est une décision à prendre. Chaque composant sorti à la section 3 devrait arriver avec son test d'interaction, ce qui plaide pour `@testing-library/svelte` d'abord.

### Lot Playwright de départ (fumée)

Contre la version web, sans Wails : `go run ./cmd/lattis-engine --addr 127.0.0.1:8102` puis `npm run dev:web` (Vite relaie `/ws`, `/upload` et `/download` vers le moteur). En CI, un job Linux qui lance les deux, attend le port 9245, puis :

1. Nouveau classeur : la grille a le focus, la zone de nom affiche A1, la région `aria-live` existe.
2. Saisir `Produit`, `Prix` puis deux lignes : un tableau naît, la cellule cachée annonce « B2, Prix, … ».
3. Écrire `=[Prix]*2` dans la colonne suivante : colonne calculée, valeurs attendues.
4. Ctrl+Z : la colonne calculée disparaît ; Ctrl+Y : elle revient.
5. Enregistrer (téléchargement), rouvrir le fichier (envoi) : mêmes valeurs.
6. Onglet SQL : `SELECT SUM(Prix) FROM Tableau1` donne le total.
7. Clic droit sur une cellule : le menu s'ouvre, Échap le ferme et rend le focus à la grille.

La grille étant un canvas, les assertions passent par la zone de nom, la barre de formule, la cellule cachée de `GridA11y.svelte` (son texte décrit la cellule active) et, pour les valeurs, par l'onglet SQL : pas de comparaison d'images.
