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

### 1. Critères de comparaison par ligne quadratiques (piste E) **[corrigé]** `66ff25a1`

`SOMME.SI(T[Qté];">"&[@Qté];T[Qté])` dans une colonne calculée relisait tout le tableau à chaque ligne. L'agrégat de la colonne (`MOYENNE(T[col])`, ou tout calcul qui la lit en entier) devenait quadratique ; RANG et les égalités avaient déjà leur précalcul.

Correction (`internal/formula/compared.go`) : quand un seul critère qui dépend de la ligne est une comparaison (`<`, `<=`, `>`, `>=` écrit `"op"&[@Col]`), les lignes retenues par les autres critères sont rangées par la valeur comparée (nombres et dates, textes sans casse, booléens), avec les cumuls dans les deux sens ; chaque ligne lit son résultat par dichotomie. Les autres critères de la ligne restent des égalités (totaux par valeur, existants). SOMME, MOYENNE, NB, MIN, MAX des familles .SI et .SI.ENS. Tout autre cas (`<>`, joker, critère vide ou en erreur, deux comparaisons) garde le parcours complet. Au passage : un critère constant en erreur rend cette erreur aussi par les totaux par valeur, comme par le parcours complet.

| Agrégat de la colonne (`MOYENNE(T[K])`) | Avant, 3 000 l. | Après, 3 000 l. | Après, 30 000 l. |
|---|---:|---:|---:|
| `SOMME.SI(T[Qté];">"&[@Qté];T[Qté])` | 7,7 s | 13 ms | 89 ms |
| `NB.SI(T[Qté];"<="&[@Qté])` | 10,6 s | 12 ms | 82 ms |
| `SOMME.SI.ENS(T[Qté];T[Client];[@Client];T[Date];"<="&[@Date])` (cumul par client) | 40,7 s | 35 ms | 324 ms |
| `RANG([@Qté];T[Qté])`, `NB.SI(T[Client];[@Client])` (déjà précalculés) | 10 ms, 22 ms | inchangé | 36 ms, 177 ms |

Avant la correction, 30 000 lignes dépassaient 8 minutes. Valeurs identiques (999999, 1501,5, 1751,25…). Essai `TestComparedPrecomputed` : 29 formules, valeurs mêlées (nombres, textes qui écrivent un nombre, dates, booléens, vides, erreurs), précalcul = parcours complet ligne par ligne ; une erreur volontaire d'une unité dans la dichotomie le fait échouer.

### 2. Import : la moitié du temps dans des expressions régulières **[corrigé]** `02956028`

Trouvé au profil de l'import (1 M de lignes) : le relevé des écritures du Store (`noteWrite`, `classify`, `trackRebuild`, qui disent quels tableaux et colonnes une requête touche) passait chaque exécution de la requête préparée d'insertion par une douzaine d'expressions régulières, soit un million de fois la même analyse. Le relevé de ce qui ne dépend que du texte est maintenant gardé par texte (`internal/store/qscan.go`, 1 024 requêtes au plus, requêtes de plus de 4 Ko non gardées) ; ce qui dépend des reconstructions en cours reste calculé à chaque fois.

Import de 1 M de lignes × 6 colonnes : **40,5 s → 5,3 s** (mêmes conditions, cache désactivé puis activé ; 5,4 s au passage suivant).

### 3. TCD et graphique recalculés après toute saisie dans leur source (piste D) **[corrigé en partie]** `c8d1f9a8`

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

### 5. Bloc JavaScript de 2,8 Mo **[corrigé en partie]** `23c78e37`

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
| 1 | Critères `<`/`>` par ligne (piste E) | **[corrigé]** `66ff25a1` |
| 2 | Import : expressions régulières par ligne | **[corrigé]** `02956028` |
| 3 | TCD/graphique après une saisie non lue (piste D) | **[corrigé]** `c8d1f9a8` ; saisie lue **[plus tard]** |
| 4 | Démarrage à froid et mémoire 0 / 1 M / 10 M | **[mesuré]** ; extraction à l'ouverture (10 M) **[arbitrage]** ; numérotation auto **[plus tard]** |
| 5 | Bloc JS : Quill et CodeMirror à la demande | **[corrigé]** `23c78e37` ; dialogues **[plus tard]** |
| 6 | Tri 1 M (piste F) | **[plus tard]** |
| 7 | Pilote SQLite natif (piste G) | **[arbitrage]** |

## Suite du 30/10 : les « plus tard » sans arbitrage

Deux mesures de plus (`internal/canvas/mesure_suite_test.go`) :

| Mesure | Commande |
|---|---|
| TCD seul, cinq formes, après une saisie lue | `LATTIS_MESURE_TCD_SEUL=1000000 go test ./internal/canvas -run TestMesureTCDSeul -v` |
| Tri (nombre, texte), annulation, rétablissement | `LATTIS_MESURE_TRI=10000000 go test ./internal/canvas -run TestMesureTri -v -timeout 90m` |

Le poste était encore plus chargé que le matin (d'autres travaux compilaient et lançaient leurs essais) : une même mesure varie du simple au double. Les chiffres viennent de binaires d'avant et d'après lancés **tour à tour**, deux ou trois fois ; on donne le meilleur de chaque côté.

### 8. TCD après une saisie dans une colonne lue : une seule lecture de la source **[corrigé]** `50ad8f17`

Le cube lisait la source une fois par niveau : (champs en lignes + 1) × (champs en colonnes + 1) passes `GROUP BY`, 4 pour Pays × Ville, 6 pour deux champs en lignes et un en colonnes. Il n'en fait plus qu'une, au niveau le plus fin (tous les champs), qui rend aussi la première ligne de chaque groupe (`min(_pos)`) et des agrégats partiels ; les sous-totaux et totaux en sont déduits en Go (`internal/canvas/pivotone.go`) : nombres additionnés, sommes entières exactes, sommes réelles par la même somme compensée que SQLite, moyenne = somme ÷ nombre à la fin, min et max comparés comme SQLite (nombres, textes, données binaires). Sans tri choisi, l'ordre des lignes et des colonnes (première apparition, vides en dernier) est calculé à partir de ces premières lignes ; avec un tri, les deux passes ordonnées restent en SQL (3 passes au lieu de 4 ou plus).

Restent en plusieurs passes, sans changement : le nombre distinct (il ne se déduit pas), un dépassement d'entier, deux valeurs qui s'affichent pareil (« » et « (vide) »), plus de 10 000 lignes ou 50 colonnes (l'erreur vient du calcul d'avant).

| TCD seul, 1 M de lignes, après une saisie lue | Passes avant → après | Avant | Après |
|---|---|---:|---:|
| Pays × Ville (somme, nombre) | 4 → 1 | 4,8 s | 2,0 s |
| Pays, Ville en lignes (somme) | 3 → 1 | 3,3 s | 2,0 s |
| Client (5 000) × Pays (moyenne, max) | 4 → 1 | 4,2 s | 1,8 s |
| Pays × Ville, Pays trié | 4 → 3 | 4,5 s | 4,4 s |
| Pays × Ville (nombre distinct) | 4 → 4 | 6,2 s | 6,1 s |

Le graphique de la même feuille (une passe déjà) garde ses 1,5 à 2 s ; la mise en page de `TestMesureTCD` après une saisie lue passe de 6,0 s à 3,7 s (meilleurs tours).

**Mêmes résultats.** Essai `TestPivotOnePassSame` : 300 TCD tirés au hasard (jusqu'à 3 champs en lignes et 2 en colonnes, sous-totaux en haut, en bas ou aucun, total masqué, tri, ordre personnalisé, les 15 « Afficher les valeurs » avec champ et élément de base, filtres, étiquettes gardées, segments, groupes de dates et d'intervalles, champs calculés, nombre distinct) sur 600 lignes mêlées (vides, textes dans une colonne de nombres, casse différente) : cube (types compris), vue et TCD à plat identiques au calcul en plusieurs passes ; une erreur volontaire dans l'ordre des vides le fait échouer. Une seule nuance : une somme réelle de sous-total est une somme de sommes ; elle peut différer de la somme de SQLite au dernier bit. Rien ne se voit avec des données à quelques décimales (montants, mesures) ; avec des réels à 17 chiffres tirés au hasard, l'essai a trouvé un 15e chiffre affiché différent (1,51284622372342 au lieu de …41). L'ancien calcul n'était pas plus juste : SQLite additionne dans l'ordre où il lit les lignes, qui n'est pas le même d'une passe à l'autre.

### 9. Numérotation automatique et suggestion de clé à l'ouverture **[corrigé]** `58ed2236`

- `autoNumber` (la première colonne est-elle une suite 1, 2, 3… ?) vérifiait la suite par `lag(…) OVER (ORDER BY _pos)` sur tout le tableau : 3,5 s à 1 M de lignes, même pour un tableau qui n'en est pas une (une fonction de fenêtre lit tout avant de répondre). Elle lit maintenant la colonne dans l'ordre de l'index et s'arrête au premier écart (`steadySteps`) : 0,5 s pour une vraie suite, quelques ms sinon. Écarts calculés comme SQLite (texte converti, dépassement d'un entier en réel, réel et entier comparés sans perte). Essai `TestSteadyStepsSame` : même réponse que la fonction de fenêtre (écarts, vides, réels entiers, dépassements, pas nul).
- La suggestion de clé comptait les valeurs distinctes de la première colonne (un tri : 0,9 s à 1 M). Une colonne d'entiers en suite régulière est forcément remplie et sans doublon : elle reprend le résultat de la numérotation (même cache) au lieu de compter.

| Ouverture, 1 M de lignes × 10 colonnes (première colonne 1, 2, 3…) | Avant | Après |
|---|---:|---:|
| Première vue (inchangée) | 0,3 s | 0,3 s |
| Mise en page complète (numérotation, suggestion de clé) | 2,8 à 5,4 s | 0,5 s |

Garder le résultat dans le fichier (avec la version de la colonne) n'est pas fait : les versions de colonnes ne vivent qu'en mémoire, il faudrait les écrire ; gain restant 0,5 s, après la première vue **[plus tard]** (≈ ½ jour).

### 10. Tri d'un grand tableau (piste F) **[corrigé en partie]** `ac165d4b`

Profil du tri : copie des anciennes positions pour l'annulation (`keep`), calcul des rangs (`rank`, le plus long), réécriture de `_pos` (`order`), index, validation. Les rangs venaient d'un `row_number() OVER (ORDER BY …)` écrit dans une table rangée par identifiant, donc insérée dans le désordre. Ils viennent maintenant de deux tris simples : `INSERT INTO seq (id) SELECT id … ORDER BY …` (numéros 1, 2, 3… dans l'ordre du tri, `by` finit par `_pos` : aucune égalité), puis relecture dans l'ordre des identifiants (insertion à la fin), dans des tables temporaires (hors du fichier et du journal). Ordre, vides en dernier, casse ignorée, ordre personnalisé et annulation exacte inchangés ; essai `TestSortSameOrder` (valeurs mêlées, croissant, décroissant, personnalisé ; annuler rend les clés d'avant, rétablir le même ordre).

| Tri, 10 colonnes (meilleurs tours) | 1 M avant | 1 M après | 10 M avant | 10 M après |
|---|---:|---:|---:|---:|
| Calcul des rangs (`rank`) | 4,5 s | 3,1 s | 114 s | 59 s |
| Trier par un nombre (tout compris) | 11,0 s | 8,5 s | 220 s | 142 s |
| Trier par un texte | 11,2 s | 10,4 s | 241 s | 184 s |
| Annuler le tri (inchangé) | 4,3 s | 4,2 s | 77 s | 91 s (poste chargé) |

À 10 M, le calcul d'avant passait l'essentiel de son temps à insérer 10 M de lignes dans le désordre ; le gain y est le plus net. Par un texte, les comparaisons sans casse dominent le premier tri : gain faible à 1 M.

Essayé et écarté : mettre à jour `_pos` en suivant l'ordre des rangs (accès au hasard dans le grand tableau : 33 s) ; un index sur la colonne triée (le construire est le même tri). Ce qui reste (réécrire `_pos` dans chaque ligne du tableau, 3 s ; l'index, 0,9 s ; la copie pour l'annulation, 0,8 s ; la validation, 1 s) tient au rangement des positions dans le tableau lui-même : seule la table étroite des positions (piste F de l'audit du 17/10, 2 à 3 jours, avec le modèle des tableaux) le supprime **[plus tard]**.

### 11. Autres constats du profil

- **Validation sur disque** : dans le profil de `TestMesureTCD`, 12,5 s sur 55 s sont l'écriture forcée sur disque (`FlushFileBuffers`) à la validation de l'import et du tri. C'est le choix `synchronous=FULL` des commandes de l'utilisateur (audit 6, M3) ; son coût suit le nombre de pages écrites, que les points 8 et 10 réduisent. Rien à changer **[constat]**.
- **Lecture sans regroupement** : lire les lignes du TCD sans `GROUP BY` et regrouper en Go prend 1,1 s au lieu de 1,9 s (1 M de lignes) ; mais les sommes des groupes ne seraient plus celles de SQLite. **[plus tard]**, à voir avec le graphique (même passe).
- **TCD avec un tri choisi** (4,4 s) : les deux passes ordonnées restent en SQL ; ranger les étiquettes en Go demanderait de refaire en Go les tris personnalisés et la comparaison des valeurs d'origine **[plus tard]** (≈ ½ jour). Le nombre distinct garde ses passes (6 s).
- **Suggestion de clé sur une première colonne de texte** : toujours un `count(DISTINCT … COLLATE NOCASE)`, 0,9 s, après la première vue **[plus tard]**.
- Les empreintes des commits de ce document (§ 1 à 5) désignaient des commits d'avant un rebase ; elles sont corrigées.

### Résumé de la suite

| # | Sujet | État |
|---|---|---|
| 8 | TCD après une saisie lue : une passe | **[corrigé]** `50ad8f17` |
| 9 | Numérotation automatique et suggestion de clé à l'ouverture | **[corrigé]** `58ed2236` ; résultat gardé dans le fichier **[plus tard]** |
| 10 | Tri : rangs par deux tris simples | **[corrigé en partie]** `ac165d4b` ; table étroite des positions **[plus tard]** |
| 11 | Validation sur disque, lecture sans regroupement, clé sur du texte | **[constat]**, **[plus tard]** |
