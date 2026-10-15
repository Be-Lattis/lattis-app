# Décisions du jalon M1 (spikes de dérisquage)

*Ouverte le 16/10/2026. Complétée au fil des spikes : les quatre spikes sont faits : 1.1 (grille), 1.2 (modèle canevas ↔ tables), 1.3 (formules), 1.4 (archive + Lua). Décisions 1.1 et 1.2 validées par Louis le 15/10/2026.*

| Sujet | Décision | Statut | Source |
|---|---|---|---|
| Driver SQLite | `ncruces/go-sqlite3` (sans CGO, fonctions Go 3 à 4× plus rapides qu'avec mattn) | Retenu | `spikes/sqlite-drivers/RESULTATS.md` |
| Rendu de la grille | `<canvas>` + éditeur DOM superposé, défilement virtuel, cache de blocs de 200 lignes | Retenu sous Windows ; Linux à confirmer sur un vrai poste | `docs/mesures/grille.md` |
| Stockage des colonnes | Tables `STRICT` + `_slitable_invalid` pour les saisies hors type | Retenu | ci-dessous, spike 1.2 |
| Annulation | Journal de commandes : chaque opération enregistre ses instructions SQL inverses | Retenu | ci-dessous, spike 1.2 |
| Modèle canevas ↔ tables | Identifiants internes, `_pos` fractionnaire, cellules libres en métadonnées, règles d'îlots | Retenu | ci-dessous, spike 1.2 |
| Archive | ZIP (manifeste + base) extrait dans le dossier cache, verrou du système, VACUUM INTO + renommage atomique, récupération des répertoires orphelins | Proposé | ci-dessous, spike 1.4 |
| Lua | gopher-lua sans bibliothèques dangereuses, db.query en lecture seule, db.execute par courtes transactions, arrêt par contexte, chien de garde mémoire | Proposé | ci-dessous, spike 1.4 |
| Formules | Un parseur, un AST, deux backends (Go et SQL) qui partagent le même code de sémantique Excel ; agrégats indépendants de la ligne calculés une fois | Proposé | ci-dessous, spike 1.3 |

---

## Spike 1.2 : modèle canevas ↔ tables

Code : `spikes/canvas` (≈ 1 500 lignes hors tests). Critère de sortie : `TestScenario`, une suite de 29 actions (naissance de tableaux, extension par la ligne et la colonne grisées, insertion au milieu avec décalage du tableau du dessous, ajout, insertion et suppression de colonnes, élargissement de type, type fixé et saisie invalide, renommage, suppression de ligne, effacements). Après **chaque** action, `Check` vérifie les invariants ; puis tout est **annulé pas à pas**, et l'état doit redevenir exactement l'état précédent (comparaison d'un dump complet : schéma, métadonnées, valeurs avec leur type SQLite) ; puis tout est **rétabli** jusqu'à l'état final identique. **Le critère est atteint.**

### Modèle retenu

- **Tables utilisateur** `t_<id>` en `STRICT`, colonnes `c_<id>` ; noms affichés, types logiques et rang d'affichage dans `_slitable_columns`. Un renommage ne touche jamais au schéma SQL, et les mots réservés, accents ou doublons ne posent aucun problème.
- **Ordre des lignes** : clé technique `id INTEGER PRIMARY KEY` + `_pos TEXT` indexé (clés fractionnaires de `internal/order`). Insérer au milieu ne renumérote rien.
- **Ancre** du tableau (ligne d'en-tête, première colonne) dans `_slitable_tables` ; déplacer un tableau ne touche pas aux données.
- **Cellules libres** dans `_slitable_cells`, en texte brut tel que saisi ; le type n'est inféré qu'à la naissance d'un tableau.
- **Ordre physique des colonnes** : il ne fait pas partie du modèle (une colonne supprimée puis restaurée revient à la fin de la table SQL). Tout le code nomme les colonnes explicitement, jamais `SELECT *`.

### Règles d'îlots (implémentées et testées)

- Une zone de cellules libres contiguës (4-connexité) de **2 lignes ou plus** devient un tableau à la validation de la saisie ; la première ligne donne les en-têtes (vides → « Colonne N »). Un tableau posé en haut à gauche de la feuille prend le nom de la feuille, les autres « Tableau N ».
- Saisie dans la **ligne grisée** sous un tableau (dans ses colonnes) : nouvelle ligne. Dans la **colonne grisée** à droite (dans ses lignes) : nouvelle colonne, nommée par la saisie si elle est faite sur la ligne d'en-tête.
- **Jamais de fusion** : quand un tableau grandit et que sa ligne (ou colonne) grisée atteindrait un autre objet, tout ce qui est dessous (ou à droite) dans sa bande descend (ou se décale), et chaque tableau déplacé dégage à son tour sa propre bande.
- Supprimer une ligne fait rétrécir le tableau ; ce qui est dessous ne remonte pas. Vider une ligne ne coupe pas le tableau.
- Une colonne encore vide prend le type de sa première valeur. Une valeur hors type **élargit** une colonne au type inféré (treillis booléen → entier → décimal → texte, date → texte) et la saisie le signale (`Event.Widened`) pour la notification « Annuler / Garder et marquer en erreur ». Dans une colonne au type **fixé**, la cellule vaut `NULL` et la saisie brute part dans `_slitable_invalid`.

### Stockage : STRICT ou ANY

Mesuré par `TestMesureElargissement` (`SLITABLE_MESURE=1000000`), tableau de 1 M de lignes × 20 colonnes, Windows, poste de Louis :

| | STRICT (reconstruction) | ANY (métadonnées seules) |
|---|---|---|
| Élargir une colonne entier → texte | 5,9 s | 1 ms |
| Annuler cet élargissement | 5,8 s | 1 ms |
| Lecture de la grille pendant la reconstruction | jamais bloquée : 1 889 fenêtres lues, pire 19 ms, 0 échec | — |
| Fichier + WAL | 355 → 519 Mo pendant la reconstruction | inchangé |

**Décision : STRICT.** Les requêtes SQL, TCD, graphiques et formules compilées voient des colonnes au type garanti ; avec `ANY`, `SUM`, les tris et les comparaisons rencontreraient des types mélangés (`'12' < 9`) et chaque requête devrait les filtrer. Le coût de la reconstruction est réel mais borné :
- elle est rare (au plus trois élargissements par colonne), et quasi gratuite sur un tableau qui vient de naître ; les gros tableaux arrivent par collage ou import, où l'inférence a lieu **avant** la création ;
- la lecture n'est jamais bloquée (WAL) : la grille reste fluide ;
- en revanche **l'écriture** est occupée pendant la reconstruction (≈ 6 s par million de lignes) : les saisies attendent dans la file. À traiter en phase 2 : barre de progression, et au-delà d'un seuil (par exemple 100 000 lignes), proposer par défaut « garder le type et marquer en erreur » plutôt que d'élargir.

### Annulation : journal d'instructions inverses

Chaque opération élémentaire (`setValue`, `insertRow`, `deleteRow`, `addColumn`, `dropColumn`, `retype`, `createTable`, `moveTable`, `moveCells`, `rename`) exécute ses instructions et **enregistre en même temps les instructions inverses**, calculées sur l'état lu dans la même transaction. Une action de l'utilisateur forme une commande ; annuler joue les inverses en ordre inverse, rétablir rejoue les instructions d'origine (identifiants et clés d'ordre sont fixés à la construction, donc le rejeu est déterministe).

- Couvre le DDL (création, reconstruction, ajout et suppression de colonnes), ce que des triggers de journal ne font pas, et l'extension *session* n'est exposée par aucun driver Go.
- Une suppression de colonne range les valeurs dans une table de corbeille : l'inverse tient en quelques instructions, quelle que soit la taille du tableau. Ces corbeilles devront être purgées quand la commande sort de la pile (phase 2).
- Les commandes sont des listes d'instructions SQL paramétrées : elles peuvent être **persistées** dans une table `_slitable_journal`, ce qui donnerait l'annulation après réouverture et un journal pour la récupération après crash.
- Un script Lua pourra être une seule commande, ses écritures passant par les mêmes primitives.

### Limites connues du prototype

- Lecture de la n-ième ligne par `OFFSET` : à remplacer par les ancres de `internal/grid` pour les gros tableaux.
- Les décalages relisent toute la mise en page de la feuille : suffisant pour des dizaines de tableaux, pas pour des milliers.
- Pas de scission ni de fusion volontaire de tableaux, pas de déplacement de tableau à la souris, pas de collage : prévus en phase 3.
- Élargir vers le texte convertit les nombres avec `CAST` (`12.5` et non `12,5`) : la conversion devra passer par le formateur de la locale.

---

## Spike 1.3 : formules

Code : `spikes/formula` (≈ 2 200 lignes hors tests). Critère de sortie : `testdata/conformite.txt`, **100 formules** avec leur résultat attendu (arithmétique, texte, comparaisons, logique, erreurs typées, arrondis, agrégats, références structurées, colonnes calculées, `REFERENCE`, cellules nommées, saisie en anglais), exécutées sur **les deux backends** : chacune doit donner le résultat attendu en Go et en SQL, et son réaffichage en français et en anglais doit redonner le même AST. **Le critère est atteint** (et la suite détecte bien une valeur fausse). Une colonne calculée reste fluide sur 1 M de lignes une fois les agrégats sortis de la requête (mesures ci-dessous).

### Architecture retenue

- **Un parseur** FR/EN (`;` et virgule décimale, ou `,` et point), références structurées `[@Col]`, `Tableau[Col]`, `Tableau[@Col]`, noms, erreurs littérales. Les fonctions sont stockées sous leur nom anglais et réaffichées dans la langue de l'utilisateur. Une fonction inconnue reste dans l'AST et vaut `#NOM?`.
- **Sémantique écrite une seule fois** (`value.go`, `funcs.go`) : division décimale, `#DIV/0!`, comparaison sans casse, `"3"+1` = 4 mais `"abc"+1` = `#VALEUR!`, vide = 0, ordre nombres < textes < booléens, arrondi au plus loin de zéro, agrégats qui ignorent texte et vides d'une colonne mais convertissent les valeurs directes, `NB` et `NBVAL` qui ne propagent pas les erreurs.
- **Évaluateur Go** pour les cellules nommées, avec un graphe de dépendances : ordre de calcul, noms à recalculer quand un tableau ou un nom change, référence circulaire → `#REF!`.
- **Compilation SQL** : chaque opérateur devient une fonction `slt_*` écrite en Go (`slt_add`, `slt_eq`, `slt_fn('IF', …)`…) qui appelle le même code que l'évaluateur. Booléens, erreurs typées et états d'agrégat voyagent en BLOB marqués. Les colonnes calculées sont recopiées en ligne (avec détection de cycle) ; chaque sous-requête a son alias, ce qui permet une recherche dans le tableau courant lui-même.
- **`REFERENCE(Tableau; Colonne; clé)`** : deux sous-requêtes sur l'index de la colonne clé ; une clé d'un autre type ne trouve rien, comme dans Excel (`"1"` ne trouve pas 1, alors que SQLite convertirait).
- **`RECHERCHEV(clé; plage; n; FAUX)`** est convertie en `REFERENCE` : la plage doit recouvrir un tableau, sa première colonne devient la clé (unicité vérifiée sans casse, index `UNIQUE` posé, doublons ensuite refusés), et la formule d'origine est gardée pour l'infobulle. La correspondance approchée est refusée avec un message qui oriente vers `REFERENCE.APPROCHEE`.

### Mesures (1 M de lignes × 20 colonnes, Windows)

| | Temps |
|---|---|
| Fenêtre de 200 lignes, 20 colonnes stockées | ≈ 1 ms |
| Même fenêtre + 4 colonnes calculées (arithmétique, `SI`, `REFERENCE`, part du total) | ≈ 1 ms (p95 2 ms) |
| … si l'agrégat `SOMME(Commandes[Montant HT])` reste dans la requête | 391 ms par fenêtre |
| Cet agrégat calculé une fois, à part | 0,4 s |
| `SOMME(Commandes[Calc HT])` compilée (fonctions `slt_*`) / en SQL natif | 0,8 s / 0,45 s |

**Décision : un agrégat qui ne dépend pas de la ligne est sorti de la requête** (`CompileHoisted`), calculé une fois et passé en paramètre ; il sera invalidé quand son tableau change, grâce au graphe de dépendances. Sans cela, chaque fenêtre de la grille recalcule le total du tableau. Les fonctions `slt_*` coûtent environ deux fois le SQL natif sur un agrégat complet : acceptable, et optimisable plus tard en émettant du SQL natif quand les types sont garantis (colonne STRICT numérique sans erreur possible).

### Limites connues du prototype

- Dates : pas encore de type date dans les formules (lot 3 du catalogue) ; une date est lue comme du texte ISO.
- La comparaison sans casse en SQL (`COLLATE NOCASE`) ne gère que l'ASCII : « é » et « É » diffèrent en SQL mais pas en Go. Il faudra une collation Unicode (extension `unicode` de ncruces).
- Somme de flottants : la `SOMME` de 1 M de parts affiche 1,00000000000007 ; il faudra une somme compensée (Kahan) ou un arrondi d'affichage.
- Le graphe ne suit pas encore les dépendances indirectes (un nom qui dépend d'un tableau dont une colonne calculée lit un autre tableau).
- Une vingtaine de fonctions seulement ; les familles `.SI` et `.SI.ENS`, texte, dates et tableaux dynamiques suivent l'ordre de livraison du catalogue (phase 4).

---

## Spike 1.4 : archive .slitable et Lua

Code : `spikes/archive` et `spikes/script`. Critères de sortie : un `while true do end` est interrompu sans geler la grille ; un processus tué puis relancé propose la récupération. **Les deux sont atteints** (`TestInfiniteLoopTimeout`, `TestCrashRecovery`).

### Archive

- **Ouverture** : extraction de `database.sqlite` dans un répertoire de travail sous le dossier cache de l'utilisateur (`os.UserCacheDir()/Slitable/work/<empreinte du chemin>`), base en WAL. Un manifeste d'une version plus récente est refusé.
- **Verrou** : un verrou du système (`LockFileEx` sous Windows, `flock` ailleurs) sur un fichier du répertoire de travail, tenu tant que le document est ouvert. Il empêche d'ouvrir deux fois la même archive et **disparaît avec le processus**, même tué : pas de fichier de verrou périmé à nettoyer.
- **Enregistrer** : `VACUUM INTO` (instantané cohérent des trois fichiers WAL, passé dans la file d'écriture), ZIP sans compression, écriture dans un fichier temporaire à côté de la cible puis renommage atomique. Manifeste minimal (`format_version`, `application`, `saved`), tout le reste dans SQLite.
- **Récupération** : au démarrage, `Orphans()` liste les répertoires dont le verrou est libre ; rouvrir une archive dont la session a été interrompue renvoie `RecoverableError` au lieu d'écraser le travail. `Recover` rouvre la base telle quelle (SQLite rejoue son WAL) : le test tue un processus après 1 000 écritures non enregistrées et les retrouve toutes.

| 1 M de lignes × 20 colonnes, Windows | Temps |
|---|---|
| Enregistrer (`VACUUM INTO` + ZIP) | 1,4 s, archive de 177 Mo |
| Ouvrir (extraction) | 0,3 s |

L'enregistrement occupe la file d'écriture pendant le `VACUUM INTO` (≈ 1 s par million de lignes) : les saisies attendent, la lecture de la grille non. Acceptable pour Ctrl+S ; une sauvegarde automatique devrait passer par l'API de backup incrémentale.

### Lua

- **Bac à sable** : gopher-lua ouvert sans bibliothèque (`SkipOpenLibs`), puis base, string, table et math seulement ; `dofile`, `loadfile`, `load`, `loadstring`, `require`, `module`, `setfenv`, `getfenv`, `collectgarbage` et `string.dump` retirés. `os`, `io` et `debug` n'existent pas.
- **API** : `db.query(sql, ...)` en lecture seule (pool `query_only`, nombre de lignes borné, message qui propose un `LIMIT`) ; `db.execute(sql, ...)` passe par la file d'écriture, **une courte transaction par appel**, ce qui laisse les saisies de la grille s'intercaler (2 000 écritures d'un script en 126 ms, saisies de la grille servies en 1,7 ms au pire pendant ce temps) ; `print` capturé.
- **Arrêt** : `SetContext` avec un délai (30 s par défaut) ou l'annulation du bouton Arrêter ; le même contexte est transmis aux requêtes SQL. Boucle infinie arrêtée au délai (500 ms dans le test), saisies de la grille servies en 36 µs au pire pendant la boucle ; bouton Arrêter effectif en 100 ms.
- **Mémoire** : gopher-lua ne borne pas le tas ; un chien de garde arrête le script si le tas du processus grossit de plus de 512 Mo (64 Mo dans le test : arrêt en 81 ms).

### À traiter en phase 2 et 5

- Un script = une étape d'annulation : ses `db.execute` devront s'enregistrer dans le journal de commandes (spike 1.2) sous une seule commande, sans tenir une transaction ouverte pendant tout le script.
- Le chien de garde mémoire surveille tout le processus : suffisant pour un script à la fois, à affiner si plusieurs scripts tournent ensemble.
- Purger les répertoires de travail d'un document fermé proprement (fait) et proposer les orphelins dans l'interface au démarrage (à faire avec le menu Fichier).
