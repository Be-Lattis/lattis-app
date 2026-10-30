# Audit spécialisé : architecture du frontend et du pont (30/10/2026)

*Suite de l'audit général du 30/10 (§ 7, note 2,5/5). Constats classés,
chacun marqué **[corrigé]** (fait, avec le commit), **[arbitrage]** (décision
de Louis) ou **[plus tard]** (trop gros pour maintenant, avec une estimation).*

## 1. Mesures

| | Avant (de09ec2f) | Après |
|---|---|---|
| `grid/Sheet.svelte` | 15 511 lignes | **4 448** |
| `routes/+page.svelte` | 4 184 lignes | **2 493** |
| `internal/bridge/document.go` | 2 047 lignes, 161 méthodes | 852 lignes ; méthodes réparties dans 7 fichiers par domaine |
| Essais de bout en bout | aucun | 15 parcours Playwright, ≈ 20 s |
| Défilement de 100 crans sur 100 000 lignes (script, médiane de 5) | 212 ms | 228 ms (+0,16 ms par image) |

Pas de changement de comportement : chaque étape est vérifiée par
`svelte-check`, la construction web, les 15 parcours ; les surfaces sorties
du gabarit par un relevé des styles calculés avant / après (identiques) ; le
dessin par des captures du canvas de 7 onglets de la démonstration
(identiques au pixel).

## 2. Constats et suites

1. **[corrigé] Aucun essai de bout en bout.** 15 parcours sur la version web
   (`frontend/e2e`) : démonstration, saisie d'un tableau, colonne calculée et
   complétion, tri et filtre, TCD, graphique, requête SQL, annuler et
   rétablir, enregistrement téléchargé, paramètre gardé, menu contextuel,
   rechercher, format de cellule, mise en forme conditionnelle, défilement et
   recopie, note, validation des données, lignes masquées. Un moteur neuf et
   un profil isolé par parcours (`e2e/fixtures.ts`), en parallèle ; navigateur
   déjà installé (Edge sous Windows, Chrome ailleurs), aucun téléchargement.
   `npm run e2e` (construit `dist-web` et le moteur), `npm run e2e:run` ; job
   CI « Bout en bout », facultatif (`continue-on-error`). Commits bc879138,
   0703c547, 6872ce33 et suivants.
2. **[corrigé] `Sheet.svelte` : 15 511 → 4 448 lignes**, en 45 étapes d'un
   commit chacune (de 03d0427f à 24f45d99). Logique dans 27 modules
   `grid/sheet/*.svelte.ts`, surfaces posées sur la grille dans 17 composants
   `grid/sheet/*.svelte`, types dans `sheet/model.ts`, mesures dans
   `sheet/consts.ts`. Carte en § 3.
3. **[corrigé] `+page.svelte` : 4 184 → 2 493 lignes** (5203359e et les 8
   commits précédents) : `lib/app/` (source de la feuille, partage, Fichier,
   regroupement d'onglets, protection et thème ; trois composants de boîtes).
4. **[corrigé] Méthode.** Un module reçoit un seul objet d'accès
   (`sheetCtx`, `appCtx`) : des accesseurs vers l'état du composant, lus au
   moment de l'appel ; chaque module déclare dans son interface (`DrawHost`,
   `FindBarView`…) ce qu'il lit et modifie, que `svelte-check` vérifie.
   L'état affiché par le gabarit reste dans le composant. Le code déplacé
   est le même, seuls ses accès à la feuille sont préfixés (`h.`, `sh.`) :
   un correctif écrit sur l'ancien `Sheet.svelte` se reporte à la main sans
   ambiguïté.
5. **[corrigé] `document.go`** : 161 méthodes réparties par domaine dans
   `document_cells.go`, `_sheets`, `_tables`, `_format`, `_objects`,
   `_formulas`, `_protect` (même service, même API : liaisons TypeScript
   régénérées, identiques).
6. **[arbitrage] Découper `DocumentService` en plusieurs services Wails**
   (Fichiers, Partage, Données, IA, Pages…). Les liaisons sont générées par
   service : 65 fichiers du frontend et ≈ 500 appels changent d'import ; il
   faut aussi enregistrer les services dans `main.go`, `engine/engine.go`
   (JSON-RPC de la version web) et partager l'état du classeur ouvert entre
   eux. Mécanique mais large, et en conflit avec tout chantier qui ajoute une
   méthode. Options : (a) garder un service, fichiers par domaine (fait) ;
   (b) découper après la fusion des chantiers en cours, ≈ 1 jour ; (c) une
   façade par domaine côté TypeScript (`lib/api/files.ts`…) sans toucher Go.
   Recommandation : (a) maintenant, (c) si l'on veut des imports par domaine.
7. **[arbitrage] Coût du découpage au défilement : +7 % de script**
   (212 → 228 ms pour 100 crans, ≈ 0,16 ms par image ; le dessin lit déjà
   contexte, palette et axes une fois par fonction). Options : l'accepter ;
   donner au dessin un objet « image » figé par appel ; revenir sur la sortie
   du dessin (`draw.svelte.ts`, commit 40).
8. **[arbitrage] Ordre des fusions avec les chantiers parallèles** (impression,
   collage spécial, polices et bordures, liens, images) qui modifient
   `Sheet.svelte` : soit cette branche d'abord et les autres reportent leurs
   modifications dans `sheet/*` (code identique au préfixe près), soit
   l'inverse et ce découpage est rejoué avec les mêmes outils (≈ ½ jour).
9. **[arbitrage] Dépendance `@playwright/test`** (développement seulement,
   sans navigateur téléchargé) et job CI facultatif : le rendre bloquant
   après une semaine sans échec ?
10. **[plus tard] Zones laissées dans `Sheet.svelte`** parce que d'autres
    chantiers y travaillent : impression, presse-papiers et collage spécial,
    liens hypertexte, format de cellule et styles (≈ 750 lignes) ; puis la
    barre de formule et l'éditeur de cellule en composants. ≈ ½ jour une fois
    ces chantiers fusionnés ; `Sheet.svelte` passerait sous 3 500 lignes.
11. **[plus tard] `Ribbon.svelte` (2 504 lignes)** : même méthode, ≈ ½ jour.
12. **[plus tard] `internal/canvas`** (121 fichiers, 52 k lignes, un paquet) :
    sous-paquets par domaine (TCD, graphiques, styles, formules), plusieurs
    jours ; à préparer par un graphe des dépendances internes.
13. **[plus tard] Essais d'interaction des composants sortis**
    (`@testing-library/svelte`) : maintenant qu'ils sont séparés, chacun peut
    avoir son essai (≈ 1 h par composant).
14. **[plus tard] Instabilités vues sous forte charge** (6 parcours en
    parallèle, 6 moteurs) : « Connexion au serveur perdue » (WebSocket coupé
    puis repris, onglet renommé « Sheet1 ») ; volet des champs du TCD parfois
    absent après « Nouvelle feuille » ; une saisie rapide perdue une fois. Non
    reproduits à 4 parcours en parallèle (plus de 200 passages) ; le journal
    du moteur est joint au rapport d'un parcours en échec pour les suivre.

## 3. Carte des modules

`grid/Sheet.svelte` garde l'état (tableaux chargés, sélection, édition,
défilement), le cycle de vie (`onMount`), le gabarit de la grille (canvas,
éditeur, barre de formule) et les zones du § 2.10.

| Module (`grid/sheet/`) | Contenu |
|---|---|
| `model.ts`, `consts.ts` | types échangés avec la page et internes ; mesures et polices |
| `data.svelte.ts` | données reçues du classeur (`applyData`) |
| `draw.svelte.ts` | dessin du canvas (cellules, bandeaux, fusions, sparklines, pastilles) |
| `anim`, `measure`, `formula`, `edit` | animations, largeurs et hauteurs, saisie de formule, éditeur et zone de nom |
| `mouse`, `keyboard`, `scrolling` | souris, clavier, défilement (doigt, main, molette) |
| `menus`, `filters`, `validation`, `notes`, `find` | menus ▾ et contextuel, filtres, validation, notes, rechercher |
| `charts`, `pivot`, `pivotfold`, `sparklines`, `badges` | objets posés sur la feuille |
| `commands`, `datatools`, `outline`, `audit`, `condformat`, `status`, `bench` | commandes du ruban, outils de données, plan, audit, MFC, barre d'état, mesure |
| `CellMenus`, `FilterPanel`, `ValidationDialog`, `FindBar`, `PivotFilter`… | 17 composants : menus et boîtes posés sur la grille |

| Module (`lib/app/`) | Contenu |
|---|---|
| `sheetsource.svelte.ts` | la `SheetSource` (grille ↔ `DocumentService`) |
| `files`, `sharing`, `tabmerge`, `protect` | menu Fichier, classeur partagé, onglets aux mêmes colonnes, protection et thème |
| `AskDialogs`, `ShareDialogs`, `ToolDialogs` | questions, boîtes du partage, boîtes des menus |

## 4. Lancer les essais de bout en bout

```
cd frontend
npm run e2e        # construit dist-web, compile le moteur web, 15 parcours
npm run e2e:run    # parcours seuls (dist-web déjà construit)
```

Go dans le `PATH` (ou `LATTIS_E2E_ENGINE` vers un moteur compilé avec
`go build -tags web ./cmd/lattis-engine`) ; navigateur : `LATTIS_E2E_CHANNEL`
(`msedge`, `chrome`, ou `chromium` après `npx playwright install chromium`).
