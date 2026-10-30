# Découper `Sheet.svelte` et tester l'interface

*Plan du 19/10/2026 (audit 6, I12), repris et exécuté le 30/10
(`docs/audits/audit-architecture-2026-10-30.md`).*

## 1. État au 30/10

`frontend/src/lib/grid/Sheet.svelte` : 15 511 → 4 448 lignes ;
`routes/+page.svelte` : 4 184 → 2 493. Le reste est dans :

- `grid/sheet/*.svelte.ts` : 27 modules de logique (données reçues, dessin,
  souris, clavier, saisie, menus, filtres, validation, TCD, graphiques…) ;
- `grid/sheet/*.svelte` : 17 composants posés sur la grille (menus
  contextuels, panneau ▾, validation, rechercher, notes, boîtes) ;
- `grid/sheet/model.ts` (types), `grid/sheet/consts.ts` (mesures, polices) ;
- `lib/app/` : modules et boîtes de la fenêtre (source de la feuille,
  Fichier, partage, protection).

Restent dans `Sheet.svelte` : l'état central (tableaux chargés, sélection,
édition, défilement), `onMount`, le gabarit de la grille (canvas, éditeur,
barre de formule), et quatre zones laissées le temps que les chantiers en
cours y finissent : impression, presse-papiers et collage spécial, liens
hypertexte, format de cellule.

## 2. Méthode

1. **Un module = une factory** `createX(h: XHost)` dans un `.svelte.ts` :
   le code est déplacé tel quel ; ce qu'il lit ou modifie de la feuille passe
   par `h.nom` (accesseurs). `XHost` liste ces noms avec leur type :
   `svelte-check` vérifie que la feuille les fournit.
2. **Un seul objet d'accès** par composant parent : `sheetCtx` dans
   `Sheet.svelte`, `appCtx` dans `+page.svelte`, passé à chaque factory et à
   chaque composant (`v={…}`). Les accesseurs sont lus au moment de l'appel :
   l'ordre des déclarations ne compte pas.
3. **L'état lu par le gabarit reste dans le composant** ; une fonction sortie
   revient par déstructuration (`const { a, b } = xCtl`) à l'endroit où elle
   était, les fonctions exportées (API de la feuille) par `export const`.
4. **Les surfaces posées sur la grille** deviennent des composants avec leur
   gabarit et leurs règles CSS ; on relève les styles calculés avant et
   après (identiques).
5. **Une étape = un commit**, sans changement de comportement, vérifiée par
   `npm run check`, la construction web et `npm run e2e:run`.
6. **Dessin** : le contexte du canvas, la palette et les fonctions d'axe sont
   lus une fois en tête de chaque fonction de `draw.svelte.ts` (coût mesuré :
   +0,16 ms par image au défilement).

Pour ajouter du code : dans le module du domaine ; s'il lit un nouvel état
de la feuille, ajouter le nom à l'interface `XHost` du module et l'accesseur
à `sheetCtx`.

## 3. Suite

| Étape | Contenu | Estimation |
|---|---|---|
| 1 | Impression, presse-papiers et collage spécial, liens, format de cellule vers `sheet/` (après les chantiers en cours) | ½ jour |
| 2 | `FormulaBar.svelte` et éditeur de cellule (avec les snippets `coloured` et `suggestions`) | ½ jour |
| 3 | `Ribbon.svelte` (2 504 lignes) | ½ jour |
| 4 | Essais d'interaction des composants sortis (`@testing-library/svelte`) | 1 h par composant |

## 4. Tests

- Modules `.ts` purs : plus de 160 fichiers vitest.
- Rendu serveur de composants (`svelte/server`) : `grid/a11y.test.ts`,
  `help/Markdown.test.ts`, `ui/PanelHeader.test.ts`.
- **Bout en bout** (`frontend/e2e`, Playwright, version web) : 15 parcours,
  un moteur neuf et un profil isolé par parcours. `npm run e2e` (construit
  `dist-web` et le moteur), `npm run e2e:run` (parcours seuls) ; job CI
  « Bout en bout », facultatif. La grille est un canvas : les vérifications
  passent par la zone de nom, la barre de formule, la cellule cachée de
  `GridA11y.svelte` (« B2, Prix, 3 »), la barre d'état et l'onglet SQL.
