# Audit de performance : gros jeux de données (17/10/2026)

**Méthode.** Les mesures ont été prises avec `DocumentService`, appelé comme le fait l'interface (rechargement compris), sur trois démos : 1M×10, 1M×100 et une partie de 10M×10. Le code mesuré date d'**avant** trois optimisations de main : la saisie qui garde le tableau, le contrôle par EXISTS et TypeMix en un seul parcours. Les points marqués **[à remesurer]** sont peut-être déjà améliorés.

**Pas de bug à la réouverture.** L'erreur « archive .slitable invalide » venait du test lui-même. La réouverture fonctionne : 0,6 s en 1M×10.

## Problèmes, du plus gênant au moins gênant

| # | Opération | 1M×10 | 1M×100 | 10M×10 | Cause | Piste |
|---|---|---|---|---|---|---|
| 1 | Coller dans un tableau, `db.update` Lua [à remesurer] | 200×6 cellules : 254 s | 2000×6 : ne finit pas | — | Deux `count` sur toute la table **par cellule** (`input` → `tables`/`fill`, `setCell`, `editor.setCells`) | EXISTS, tableaux lus une fois par commande. Environ 5 ms par cellule |
| 2 | Saisir une cellule [à remesurer] | 227 ms | 733 ms | 2,6 s | Les mêmes `count`, plus le rechargement du tableau (`grid.Table.Load` : 4,1 s en 10M) | Rechargement déjà évité sur main. Sinon, calculer les ancres en SQL |
| 3 | TCD et graphiques recalculés après **n'importe quelle** écriture | 1,6 s + 1,3 s | 2,4 s + 1,7 s | 18 s + 16 s | Cache invalidé à chaque écriture. `AllTables` calcule chaque TCD juste pour en connaître les colonnes | Invalider seulement quand la source change, et lire les colonnes du TCD dans sa configuration |
| 4 | Export xlsx | 43 s, 4,2 Go | > 25 Go, arrêté | — | Toute la feuille est gardée en mémoire (`xlsxwrite.go`) | Écrire en flux. Import xlsx : 56 s |
| 5 | UPDATE SQL de toutes les lignes | 22 s | 73 s | — | Déclencheurs ligne par ligne et capture pour l'annulation (`sqlexec.go`) | Viser la table physique et capturer en une seule requête. Environ ×4 |
| 6 | Trier | 10 s (annuler : 3,9 s) | 22 s | 242 s | `_pos` réécrit sur des lignes larges (`sort.go`). Les saisies attendent la fin du tri | `_pos` dans une table étroite, ou reconstruction et renommage |
| 7 | Rechercher un texte absent | 5,3 s | 47 s | 60 s | `slt_text()` sur chaque cellule (`find.go`) | Sauter les colonnes qui ne peuvent pas contenir le texte cherché. Plus de ×10 |
| 8 | Changer un type, saisie hors type | 3,6 à 6,8 s | 15 à 33 s | 42 s | Table STRICT recréée en entier (`convert.go`) | Confirmer au-delà de N lignes. À terme, colonnes ANY |
| 9 | Supprimer une colonne (et annuler) | 2,9 s (3,2 s) | 11,5 s (13,6 s) | 40 s (67 s) | `ALTER DROP COLUMN` réécrit toute la table | Suppression logique, purgée plus tard |
| 10 | Totaux de la barre d'état | colonne 0,7 s, Ctrl+A 4 s | Ctrl+A **40 s** | 42 s | Sous-requête `ORDER BY _pos`. La demande n'est pas annulée quand la sélection change | Agréger directement, rendre la demande annulable |
| 11 | Menu ▾ au 1er affichage [à remesurer] | 1 à 4,6 s | 2 à 7,5 s | 14 à 48 s | Liste des valeurs, contrôle des types, contrôle de clé | Contrôle des types sur un échantillon, valeurs les plus fréquentes |
| 12 | SQL → feuille, Power Query sur un tableau | 14 à 34 s | 52 à 94 s, 4,7 Go | — | Toutes les lignes passent par Go | `INSERT … SELECT` direct dans SQLite. ×5 à ×10 |
| 13 | Supprimer les doublons | 12 s | 26 s | 143 s | Corbeille, puis `DELETE … IN` | Au-delà de 50 % supprimé : garder les lignes restantes et mettre l'ancienne table à la corbeille |
| 14 | Calcul initial d'un TCD ou d'un graphique | 1,3 à 1,6 s | 1,7 à 2,4 s | 16 à 18 s | Normal | Voir le point 3 |
| 15 | Enregistrer | 4,2 s (479 Mo) | 14 s (2,2 Go) | — | Enregistre **même sans changement**, historique d'annulation compris | Ne rien faire si le classeur n'a pas changé, borner l'historique |
| 16 | Copier une grande plage | — | — | — | **Bug** : les lignes non chargées partent vides dans le presse-papiers. Refus de collage tardif | Lire les valeurs côté moteur |
| 17 | Ouverture de la démo | 9,8 s | 56 s | 41 s | `count(*)` dans `fill` | Garder le nombre de lignes en cache |

## Ce qui est déjà rapide

- Défilement : 1 à 9 ms ; 28 ms pour 2000 lignes en 1M×100.
- Insérer ou supprimer une ligne de tableau : 10 à 23 ms.
- Filtrer : 15 ms.
- Totaux déjà calculés : instantané.
- Créer une colonne calculée : 2 ms.
- Créer un TCD : 10 ms.
- `SELECT` limité : 1 à 9 ms.
- Coller 100 000 × 10 en zone libre : 2,5 s.
- Réouvrir un fichier : 0,6 s.
