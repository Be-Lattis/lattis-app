# Audit des performances : calculs et démarrage, 30/10/2026

**Suite de** `audit-general-2026-10-30.md` § 3 (note 3,5/5) et de `reflexion-performances-2026-10-21.md` (pistes A à H).

**Méthode.** Mesures sur le poste de développement (Windows 10), par des essais gardés derrière une variable d'environnement (aucun coût pour `go test` ordinaire), profils CPU (`-cpuprofile`, `go tool pprof`). Le poste était chargé par d'autres travaux en parallèle : les durées longues varient de ±30 % d'un passage à l'autre ; les écarts cités ici sont bien au-delà.

| Mesure | Commande |
|---|---|
| Critères par ligne | `LATTIS_MESURE_CRITERES=30000 go test ./internal/canvas -run TestMesureCriteres -v` |
| TCD, graphique, import | `LATTIS_MESURE_TCD=1000000 go test ./internal/canvas -run TestMesureTCD -v` |
| Démarrage à froid, mémoire | `LATTIS_MESURE_DEMARRAGE=0,1000000,10000000 go test ./internal/bridge -run TestMesureDemarrage -v -timeout 90m` |
| Code chargé au démarrage | `npm run build`, puis fermeture des imports statiques de `dist/_app/immutable/{entry,nodes}` |

## Constats, par ordre d'importance

### 1. Critères de comparaison par ligne quadratiques (piste E) **[corrigé]** `f0f64fcd`

`SOMME.SI(T[Qté];">"&[@Qté];T[Qté])` dans une colonne calculée relisait tout le tableau à chaque ligne. L'agrégat de la colonne (`MOYENNE(T[col])`, ou tout calcul qui la lit en entier) devenait quadratique ; RANG et les égalités avaient déjà leur précalcul.

Correction (`internal/formula/compared.go`) : quand un seul critère qui dépend de la ligne est une comparaison (`<`, `<=`, `>`, `>=` écrit `"op"&[@Col]`), les lignes retenues par les autres critères sont rangées par la valeur comparée (nombres et dates, textes sans casse, booléens), avec les cumuls dans les deux sens ; chaque ligne lit son résultat par dichotomie. Les autres critères de la ligne restent des égalités (totaux par valeur, existants). SOMME, MOYENNE, NB, MIN, MAX des familles .SI et .SI.ENS. Tout autre cas (`<>`, joker, critère vide ou en erreur, deux comparaisons) garde le parcours complet. Au passage : un critère constant en erreur rend cette erreur aussi par les totaux par valeur, comme par le parcours complet.

| Agrégat de la colonne (`MOYENNE(T[K])`) | Avant, 3 000 l. | Après, 3 000 l. | Après, 30 000 l. |
|---|---:|---:|---:|
| `SOMME.SI(T[Qté];">"&[@Qté];T[Qté])` | 7,7 s | 13 ms | 89 ms |
| `NB.SI(T[Qté];"<="&[@Qté])` | 10,6 s | 12 ms | 82 ms |
| `SOMME.SI.ENS(T[Qté];T[Client];[@Client];T[Date];"<="&[@Date])` (cumul par client) | 40,7 s | 35 ms | 324 ms |
| `RANG([@Qté];T[Qté])`, `NB.SI(T[Client];[@Client])` (déjà précalculés) | 10 ms, 22 ms | inchangé | 36 ms, 177 ms |

Avant la correction, 30 000 lignes dépassaient 8 minutes. Valeurs identiques (999999, 1501,5, 1751,25…). Essai `TestComparedPrecomputed` : 29 formules, valeurs mêlées (nombres, textes qui écrivent un nombre, dates, booléens, vides, erreurs), précalcul = parcours complet ligne par ligne ; une erreur volontaire d'une unité dans la dichotomie le fait échouer.

### 2. Import : la moitié du temps dans des expressions régulières **[corrigé]** `b277c0f8`

Trouvé au profil de l'import (1 M de lignes) : le relevé des écritures du Store (`noteWrite`, `classify`, `trackRebuild`, qui disent quels tableaux et colonnes une requête touche) passait chaque exécution de la requête préparée d'insertion par une douzaine d'expressions régulières, soit un million de fois la même analyse. Le relevé de ce qui ne dépend que du texte est maintenant gardé par texte (`internal/store/qscan.go`, 1 024 requêtes au plus, requêtes de plus de 4 Ko non gardées) ; ce qui dépend des reconstructions en cours reste calculé à chaque fois.

Import de 1 M de lignes × 6 colonnes : **40,5 s → 5,3 s** (mêmes conditions, cache désactivé puis activé ; 5,4 s au passage suivant).

### 3. TCD et graphique recalculés après toute saisie dans leur source (piste D) **[corrigé en partie]** `12077e13`

Leur cache dépendait de tout le tableau source. Il dépend maintenant des colonnes qu'ils lisent (zones, filtres, groupes, segments ; axes, séries et filtre du tableau pour un graphique), de l'ordre des lignes et de leur nombre (`internal/canvas/cachecols.go`, versions par colonne `store.ColumnOf`). Essai `TestPivotChartColumnCache` : après chaque opération (saisie lue et non lue, annulation, ligne insérée et supprimée, tri, filtre), ce qui est affiché est ce que donne un calcul complet.

| 1 M de lignes, feuille avec TCD Pays × Ville + graphique | Avant | Après |
|---|---:|---:|
| Mise en page après une saisie dans une colonne **non lue** | 6,3 s | 2 ms |
| Mise en page après une saisie dans une colonne **lue** | 7,7 s | 7,7 à 18 s (inchangé, poste chargé) |
| Mise en page après un tri | 14 s | inchangé |

Reste **[plus tard]** : la saisie dans une colonne lue. Deux étapes possibles, sans arbitrage de produit :
- **Une seule lecture de la source** (≈ 1 jour) : le cube lit aujourd'hui 4 passes `GROUP BY` (détail des lignes, des colonnes, croisement, total). Pour SOMME, NB, MIN, MAX et MOYENNE (somme et nombre), les niveaux supérieurs se déduisent du croisement en Go. Gain attendu : 6,5 s → 2 à 3 s. Risque : types des résultats (entier ou réel) à garder identiques.
- **Mise à jour par différence** (≈ 3 jours) : pour une saisie d'une cellule, retrancher l'ancienne valeur et ajouter la nouvelle dans les cellules du cube concernées. Rapide (quelques ms) mais demande l'ancienne valeur et les étiquettes de la ligne au moment de l'écriture, et ne vaut ni pour MIN/MAX (retrait du minimum), ni pour « nombre distinct », ni pour les affichages en % qui dépendent du total.

### 4. Démarrage à froid et mémoire **[mesuré]**

Mesure nouvelle (`internal/bridge/demarrage_mesure_test.go`) : un processus neuf ouvre un classeur de 10 colonnes, affiche la première vue (`LayoutFirst` puis le premier bloc de 200 lignes, ce qu'attend la grille), puis fait la mise en page complète. Le moteur est le même pour le bureau et la version web (`engine` sert les mêmes services). « 1er » : fichier lu du disque ; « 2e » : dans le cache du système.

| Lignes (fichier) | Init. Go | Ouverture | Première vue | Mise en page complète | Tas Go | Pic du processus |
|---|---:|---:|---:|---:|---:|---:|
| 0 (vide) | 65 ms | 72 ms | 74 ms | 1 ms | 7 Mo | 34 Mo |
| 1 M (34 Mo) 1er / 2e | 72 / 61 ms | 279 / 292 ms | 346 / 355 ms | 3,2 / 3,1 s | 6 Mo | 78 Mo |
| 10 M (343 Mo) 1er / 2e | 410 / 61 ms | 17,2 / 5,3 s | 19,0 / 6,5 s | 51 / 33 s | 7 Mo | 81 Mo |

- **Mémoire** : sobre et stable (moins de 100 Mo au pic, même à 10 M) ; SQLite lit le fichier par pages, rien n'est chargé en entier.
- **Initialisation Go** : 60 à 70 ms, dont 46 ms pour le module WebView2 de Wails (hors de notre code ; le nôtre : moins de 10 ms).
- **Ouverture à 10 M** : 5 à 17 s, presque toute dans l'extraction de la base (archive ZIP Zstandard de 343 Mo décompressée dans le répertoire de travail, `internal/archive`). **[arbitrage]** : (a) garder (l'archive n'est jamais modifiée, choix d'origine) ; (b) réutiliser la base déjà extraite quand l'archive n'a pas changé depuis la dernière fermeture (taille, date et empreinte du ZIP gardées à côté) : réouverture en moins d'une seconde, mais une copie de travail à garder sûre (≈ 1 jour) ; (c) servir la première vue pendant l'extraction (≈ 2 jours, plus risqué).
- **Mise en page complète à 1 M (3,1 s), faite après la première vue** : 80 % dans la détection de la numérotation automatique (`autoNumber`, `internal/canvas/autokey.go` : la première colonne est-elle une suite 1, 2, 3… ? fonction de fenêtre sur tout le tableau), recalculée à chaque ouverture. **[plus tard]** (≈ ½ jour) : garder le résultat dans le fichier avec la version de la colonne, ou s'arrêter au premier écart ; l'utilisateur voit déjà la grille (première vue à 350 ms), seule la suggestion de numéro arrive après.

### 5. Bloc JavaScript de 2,8 Mo **[corrigé en partie]** `c1fa8658`

Quill (pages document) et CodeMirror (éditeur SQL, scripts Lua, pages web) sont chargés à la première ouverture. Il n'y a pas de bibliothèque de cartes : les cartes sont dessinées par l'application (rien à séparer).

| Code chargé au démarrage | Avant | Après |
|---|---:|---:|
| Fermeture des imports statiques | 3 113 Ko (gzip 970 Ko) | 2 269 Ko (gzip 695 Ko) |
| Plus gros bloc | 2 824 Ko | 1 376 Ko (le code de la page principale) |

Vérifié dans la version web (`lattis-engine --demo med`) : page document (dont la mise en forme depuis le ruban), onglet SQL, onglet Script.

Reste **[plus tard]** (≈ 1 jour, `+page.svelte` partagé avec d'autres travaux) : charger à la demande les dialogues et volets rares (Paramètres, Thèmes, Partage, volet TCD, Assistant, MCP) et la référence Lua (`api.gen.ts`, 145 Ko, importée au démarrage par l'aide via `help.ts`). Gain estimé : 400 à 600 Ko de plus.

### 6. Tri d'un grand tableau (piste F) **[plus tard]**

Mesuré : 18 à 33 s pour 1 M de lignes selon la charge du poste, tout dans SQLite (renumérotation de `_pos`, index). Piste de l'audit du 17/10 (positions dans une table étroite), 2 à 3 jours, à faire avec le modèle des tableaux.

### 7. Pilote SQLite en WebAssembly (piste G) **[arbitrage]**

Le pilote `ncruces/go-sqlite3` reste environ deux fois plus lent que SQLite natif sur les lectures complètes (TCD, tri, recherche). Options : (a) garder (aucune dépendance C, compilation croisée et moteur web simples) ; (b) prototype cgo (`mattn/go-sqlite3` ou SQLite amalgamé) sur le bureau seulement, mesuré sur `TestMesureTCD`, avant toute décision ; coût élevé (fonctions `slt_*`, VFS chiffré, autorisateur et compilation Windows/macOS à reprendre). Recommandation : (a) tant que les pistes D et F ne sont pas faites.

## Résumé

| # | Sujet | État |
|---|---|---|
| 1 | Critères `<`/`>` par ligne (piste E) | **[corrigé]** `f0f64fcd` |
| 2 | Import : expressions régulières par ligne | **[corrigé]** `b277c0f8` |
| 3 | TCD/graphique après une saisie non lue (piste D) | **[corrigé]** `12077e13` ; saisie lue **[plus tard]** |
| 4 | Démarrage à froid et mémoire 0 / 1 M / 10 M | **[mesuré]** ; extraction à l'ouverture (10 M) **[arbitrage]** ; numérotation auto **[plus tard]** |
| 5 | Bloc JS : Quill et CodeMirror à la demande | **[corrigé]** `c1fa8658` ; dialogues **[plus tard]** |
| 6 | Tri 1 M (piste F) | **[plus tard]** |
| 7 | Pilote SQLite natif (piste G) | **[arbitrage]** |
