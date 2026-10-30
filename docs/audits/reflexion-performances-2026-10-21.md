# Performances à l'usage : réflexion du 21/10/2026

**But.** Repérer ce qui paraît lent quand on travaille dans Lattis, le mesurer quand c'est possible, et proposer un ordre de travail. Rien n'a été modifié dans le code.

**Méthode.** Mesures sur le poste de développement (Windows 10), par les essais `TestMesureOperations` (1 M de lignes × 6 colonnes) et par des essais jetables, retirés ensuite. Les chiffres « avant » viennent de `docs/mesures/performances-1M.md` et de `docs/audit-performance-gros-jeux.md`.

## Ce qu'on ressent, et pourquoi

### 1. Chaque saisie dans un gros tableau coûte environ une seconde

- **Mesure.** La saisie elle-même prend 3 à 4 ms (elle prenait 155 ms : corrigé). Mais l'interface relit ensuite la mise en page de la feuille, et cette relecture prend **0,93 à 0,98 s** après chaque saisie dans un tableau de 1 M de lignes. Après une saisie hors tableau : 1 ms.
- **Cause (profil CPU : 98 % du temps).** La suggestion de clé (`keyHint` → `keyCheck`, `internal/canvas/keyhint.go`) recompte les valeurs distinctes de la première colonne sur tout le tableau (`count(DISTINCT …)` sans casse). Son cache est vidé par **toute** écriture dans le tableau, même dans une autre colonne.
- **Pourquoi les mesures ne l'ont pas vu.** `TestMesureOperations` chronomètre la saisie seule, pas la mise en page qui la suit.

### 2. Un TCD ou un graphique ralentit la saisie dans son tableau source

- **Mesure (1 M de lignes).** Calcul d'un TCD Pays × Ville : 6,5 s ; d'un graphique : 1,75 s ; mise en page avec un graphique : 3,5 s.
- **Cause.** Ils restent en cache tant que leur tableau source ne change pas (corrigé le 19/10). Mais une saisie dans ce tableau les fait recalculer en entier à la mise en page suivante. On peut donc attendre plusieurs secondes après chaque saisie, si le TCD est sur la feuille affichée (déduit du code, pas chronométré de bout en bout).

### 3. Formules par ligne : rapides avec « = », très lentes avec « > »

- **Cas connu (agrégat sur une colonne calculée par SOMME.SI) : réglé quand le critère est une égalité.** `MOYENNE(T[col])`, où `col` vaut `SOMME.SI(T[Client];[@Client];…)`, prend 0,28 s sur 30 000 lignes : les totaux par valeur sont calculés une fois.
- **Encore lent.** Avec un critère de comparaison, par exemple `SOMME.SI(T[Qté];">"&[@Qté];T[Qté])`, le temps augmente comme le carré du nombre de lignes :

  | Lignes | Agrégat de cette colonne |
  |---:|---:|
  | 1 000 | 1,2 s |
  | 3 000 | 8,4 s |
  | 30 000 | plus de 8 min (arrêté) |

  Mêmes familles touchées : critères « < », « >= », « <> », jokers (« Dur* »), RANG, et SOMME.SI.ENS ou NB.SI.ENS avec ces critères.
- **Sans précalcul** (chemin pris hors de la grille et des cellules libres), même l'égalité est quadratique : 10 000 lignes, plus de 60 s.

### 4. Après chaque action, l'interface fait une dizaine d'appels l'un après l'autre

`reload()` (`frontend/src/routes/+page.svelte:637`) attend tour à tour l'état, la mise en page, l'aide des fonctions (catalogue de 82 Ko renvoyé à chaque fois), les scripts, les favoris, le partage, le travail à plusieurs, les onglets semblables et les sources. Sur le bureau, l'effet est faible : quelques millisecondes par appel. **Dans la version web**, chaque appel est un aller-retour réseau : avec 30 à 50 ms de latence, cela fait 0,3 à 0,5 s par saisie avant tout calcul (estimation, non mesurée sur un vrai réseau).

### 5. SQLite tourne deux fois moins vite que du SQLite natif

Le pilote `ncruces/go-sqlite3` exécute SQLite compilé en WebAssembly, puis traduit en Go. Mêmes requêtes, même fichier de 1 M de lignes :

| Requête | Lattis (ncruces) | SQLite natif (Python 3.50) |
|---|---:|---:|
| Contrôle de clé (`count(DISTINCT …)`) | 1,00 s | 0,45 s |
| `GROUP BY` + somme | 1,12 s | 0,55 s |

Tout ce qui parcourt un gros tableau (TCD, tri, recherche, filtres) paie ce facteur.

### 6. Opérations lourdes déjà connues (rares au quotidien)

Mesurées aujourd'hui sur 1 M de lignes : tri 12,3 s (8,8 s le 17/10), annulation du tri environ 3 s, UPDATE SQL de 10 % des lignes 5,8 s, valeurs du filtre 1,1 s, import 9,4 s (4,1 s le 17/10). Les écarts avec le 17/10 sont à revérifier à froid : une autre mesure tournait peut-être en parallèle. Les points ouverts de l'audit du 17/10 (tri, types, suppression de colonne, SQL vers feuille) restent valables.

### 7. Synchronisation et serveur

L'audit collaboratif du 20/10 mesure 314 ms au pire pour 40 postes, regroupement de 200 ms compris. C'est correct. Les limites viendront du nombre de connexions permanentes (une par poste et par classeur) et du relais d'événements (un `pg_current_wal_lsn()` par notification). Rien n'est urgent tant qu'il y a moins de quelques centaines de postes.

## Pistes, par ordre conseillé

| # | Piste | Gain attendu | Coût | Risque |
|---|---|---|---|---|
| A | **Suggestion de clé** : ne la recalculer que si la première colonne change. La calculer en arrière-plan, ou s'arrêter dès le premier doublon, au-delà de 100 000 lignes | Saisie dans un tableau de 1 M de lignes : environ 1 s → quelques ms | Faible | Faible |
| B | **Rechargement de l'interface** : appels en parallèle ou un seul appel groupé ; aide des fonctions lue une seule fois (elle ne change qu'avec la langue) | Version web : plusieurs centaines de ms par action. Bureau : léger | Faible | Faible |
| C | **Mesure « saisie puis mise en page »** dans `TestMesureOperations`, avec et sans TCD sur la feuille | Évite que les points A et D reviennent sans qu'on le voie | Faible | Nul |
| D | **TCD et graphiques** : recalcul en arrière-plan avec l'ancienne valeur affichée (indicateur « à jour… »), saisies rapprochées regroupées. Plus tard, mise à jour par différence pour SOMME, NB, MIN et MAX | Saisie sur la source d'un TCD : plusieurs secondes → immédiate | Moyen | Moyen : valeurs brièvement périmées |
| E | **SOMME.SI et autres avec « > » / « < » par ligne** : passer par un tri et des sommes cumulées (fonctions de fenêtrage) plutôt qu'un parcours par ligne. Sinon, au minimum, prévenir et rendre le calcul interruptible au-delà d'un seuil | 30 000 lignes : plus de 8 min → moins d'une seconde | Moyen | Moyen : il faut reproduire la sémantique exacte des critères (texte et nombre, vides) |
| F | **Tri** : piste de l'audit du 17/10 (positions dans une table étroite) | 12 s → 2 à 3 s sur 1 M de lignes | Moyen à élevé | Moyen (modèle des tableaux) |
| G | **Pilote SQLite natif** (cgo) à la place de ncruces | Environ ×2 sur tout ce qui est lourd | Élevé : compilation croisée, moteur web, fonctions `slt_*` et VFS à reporter | Élevé ; à n'envisager qu'après A à E, et après une mesure sur un prototype |
| H | **Version web** : format d'échange plus compact pour les blocs de la grille (déjà repéré dans `docs/mesures/grille.md`) ; à l'échelle, bascule de la synchro sur le flux d'événements du serveur | Défilement plus fluide sur un réseau lent ; plus de postes par serveur | Moyen | Faible |

**Pour commencer : A, B et C**, en une petite série de corrections peu risquées. Ce sont elles que l'utilisateur sentira le plus, à chaque saisie. Ensuite D et E, qui demandent quelques choix (affichage d'une valeur en cours de calcul, sémantique des critères). G est une décision d'architecture à part.

## État au 21 octobre

| Piste | État |
|---|---|
| A | **Fait** : le Store suit les écritures par colonne (`store.ColumnOf`, `internal/store/colver.go`) ; la suggestion de clé ne se recalcule que si la première colonne ou les lignes changent. Mise en page après une saisie, 1 M de lignes : 1 s → 2 ms (inchangé, 1 s, après une saisie dans la première colonne) |
| B | **Fait** : `reload()` lance ses appels ensemble ; l'aide des fonctions est encore relue à chaque fois |
| C | **Fait** : `TestMesureOperations` mesure la mise en page après une saisie, et le TCD après une saisie dans sa source (6,5 s : piste D) |
| D | **En partie (30/10)** : TCD et graphiques ne dépendent que des colonnes qu'ils lisent ; une saisie ailleurs ne les recalcule plus (6,3 s → 2 ms). Saisie dans une colonne lue : voir `audit-performance-2026-10-30.md` |
| E | **Fait (30/10)** : critères `<`, `<=`, `>`, `>=` par ligne précalculés (tri et cumuls) ; 30 000 lignes : plus de 8 min → 89 ms |
