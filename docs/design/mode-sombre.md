# Mode sombre et contraste élevé : conception

*Note de conception, 19/10/2026 (audit 6, Id6, et la partie « contraste élevé » de I11). Rien n'est encore fait : ce n'est pas un changement simple, ce document dit pourquoi et comment s'y prendre.*

## 1. Point de départ

- Les couleurs sont écrites en dur : environ 1 760 valeurs hexadécimales dans les `.svelte` et `.ts` du frontend, 226 distinctes. Les plus fréquentes : `#217346` (vert Lattis, 237 fois), `#fff` (227), `#6b7280` (gris du texte secondaire, 123), `#d4d7db` et `#c9cdd2` (bordures), `#1f2328` (texte).
- Seule l'échelle d'empilement est déjà en variables CSS (`--z-panel`, `--z-dialog`…, dans `routes/+page.svelte`), ce qui montre la voie.
- La grille, les graphiques et les cartes sont **dessinés sur canvas** : une variable CSS ne les atteint pas, le code de dessin lit ses couleurs dans des constantes JavaScript (`grid/Sheet.svelte`, `grid/style.ts`, `chart/*`).
- Les couleurs choisies par l'utilisateur (remplissage, police, mise en forme conditionnelle, couleur d'onglet, séries de graphique) sont des **données du classeur** : elles ne changent pas avec le thème.
- L'identité visuelle a déjà ses variantes sombres (`be-lattis-logo-sombre.svg`, menthe `#b9ecca` sur fond sombre, voir `identite-visuelle.md`).
- La version web expose déjà `IsDarkMode` (`lib/web/runtime.ts`), comme le runtime Wails.

## 2. Principes

1. **Des jetons sémantiques, pas des couleurs** : `--surface`, `--surface-2`, `--text`, `--text-muted`, `--border`, `--accent` (vert Lattis), `--accent-text`, `--danger`, `--warning`, `--focus`, `--selection`, `--grid-line`, `--header-bg`… une trentaine au plus. Les 226 valeurs actuelles se ramènent à ces rôles.
2. **Un seul endroit** : les jetons sont définis sur `:root` dans un fichier `frontend/src/lib/theme.css`, avec une variante `@media (prefers-color-scheme: dark)` et une variante forcée `:root[data-theme="dark"]` / `[data-theme="light"]` pour le réglage manuel.
3. **Le canvas lit les mêmes jetons** : un module `lib/theme.ts` lit `getComputedStyle(document.documentElement)` une fois par changement de thème (écoute de `matchMedia('(prefers-color-scheme: dark)')` et du réglage), et fournit un objet `palette` au dessin ; changer de thème redessine.
4. **Les couleurs de l'utilisateur restent les siennes**, comme dans Excel : une cellule remplie en jaune reste jaune. Seules les couleurs « par défaut » (texte automatique, fond sans remplissage, quadrillage) suivent le thème. Un texte « automatique » sur un remplissage choisi prend la couleur lisible (noir ou blanc selon la luminance du fond).
5. **Contraste vérifié** : chaque paire texte / fond des jetons respecte 4,5:1 (WCAG AA), 3:1 pour les bordures utiles et l'indicateur de focus.

## 3. Contraste élevé de Windows (`forced-colors`)

Avec le contraste élevé, le navigateur remplace les couleurs CSS par celles du système, mais **pas celles du canvas**. Il faut donc :

- dans `theme.css`, un bloc `@media (forced-colors: active)` qui associe les jetons aux couleurs système (`Canvas`, `CanvasText`, `Highlight`, `HighlightText`, `ButtonFace`, `ButtonText`, `GrayText`, `LinkText`) ;
- dans `theme.ts`, la même détection par `matchMedia('(forced-colors: active)')` : la palette du canvas prend alors ces couleurs système (lues par un élément témoin et `getComputedStyle`), et le dessin ignore les remplissages de l'utilisateur pour le texte (le contenu reste lisible) ;
- garder la sélection et la cellule active visibles par un trait épais, pas seulement par une teinte ;
- les icônes SVG en `currentColor` (c'est déjà le cas de la plupart, `lib/icons`).

Ce point sert aussi l'accessibilité (I11, RGAA) et passe avant le mode sombre s'il faut choisir.

## 4. Étapes

| Étape | Contenu | Coût estimé |
|---|---|---|
| 1 | `theme.css` (jetons du thème clair actuel, valeurs identiques : aucun changement visible) et `theme.ts`. | 0,5 jour |
| 2 | Remplacement mécanique des valeurs hexadécimales par les jetons, fichier par fichier, en commençant par la coque (ruban, onglets, barre d'état, boîtes de dialogue) ; un test vitest interdit les nouvelles valeurs en dur hors `theme.css` et des couleurs de l'utilisateur. | 3 à 4 jours, à caler sur le découpage de `Sheet.svelte` |
| 3 | Palette du canvas (grille, en-têtes, sélection, graphiques, cartes) branchée sur `theme.ts`. | 2 jours |
| 4 | `forced-colors` (section 3). | 1 jour |
| 5 | Valeurs sombres des jetons, logo sombre, éditeurs CodeMirror et Quill en thème sombre, réglage Fichier › Paramètres › Apparence (Système, Clair, Sombre) et clé de configuration d'entreprise `ui.theme`. | 2 à 3 jours |
| 6 | Revue des contrastes (outil automatique sur les jetons, puis passage à l'œil sur chaque écran) et captures pour la documentation. | 1 jour |

## 5. Points ouverts

- **Pages web d'un classeur** (onglets HTML) : elles ont leurs propres styles. Leur passer `prefers-color-scheme` suffit ; on ne force rien.
- **Export et impression** : toujours en thème clair (PDF, aperçu avant impression), quel que soit le thème de l'écran.
- **Couleurs par défaut des graphiques** : la palette des séries doit rester distincte sur les deux fonds ; à valider avec quelques graphiques réels.
