# Retour visuel sur les gros classeurs : mesures du 22/10/2026

**Demande de Louis.** Sur un très gros tableau, supprimer une colonne prend longtemps et la barre d'état ne dit que « Traitement… ». Il faut dire ce qui se passe, montrer tout de suite que l'action est prise en compte, et permettre d'annuler quand c'est sûr.

**Ce qui a été fait.**

- **Barre d'état** : l'action et son étape (« Suppression de la colonne Prix : réécriture du tableau », « Tri par Client : classement des lignes · 12 s »), l'avancement quand il est connu sans requête de plus (lignes d'un collage, lignes comptées au chargement), et un bouton **Annuler**. Rien ne paraît pour une opération de moins de 0,3 s. Suivi `internal/work`, porté par le contexte. Évènement `work:step`, le même sur le bureau et dans la version web. Interface : `frontend/src/lib/shell/work.ts` et `BusyBar.svelte`.
- **Reflet sur la zone traitée** : la colonne, les lignes, le tableau ou le bloc visé reçoit une teinte bleue, une bande lumineuse qui passe et un contour en pointillés qui défile, jusqu'à la fin de l'action (réussie ou non). Le reflet paraît après 150 ms et reste fixe si les animations sont réduites. Voir `frontend/src/lib/grid/working.ts` et `drawWorking` dans `Sheet.svelte`.
- **Annuler** : seulement pour les actions faites en une seule transaction (`Model.Do`). L'interruption arrête SQLite (`sqlite3_interrupt`), la transaction est défaite et le classeur reste tel qu'avant, sans étape d'annulation. Le message affiché est « Opération annulée : le classeur n'a pas été modifié ». Les essais `TestWorkCancelRollsBack` et `TestCancelWorkFromStatusBar` interrompent chaque type d'opération au milieu d'une étape. Dans l'application (version web, 1 M de lignes), la réponse arrive environ 0,9 s après le clic.
  - **Annulable** : suppression et insertion de colonnes ou de lignes (dans un tableau ou sur la feuille), tri et tri personnalisé, type d'une colonne, collage et recopie, copier-coller interne (`Transfer`), effacement, mise en forme (cellules, lignes, colonnes), conversion en valeurs, suppression des doublons, annuler et rétablir (Ctrl+Z, Ctrl+Y), requêtes de l'onglet SQL (lecture et écriture).
  - **Montré sans être annulable** : mise en page de la feuille (contrôle de la clé, TCD, graphiques, formules libres) et chargement d'un tableau (comptage des lignes, lecture des cellules visibles). Ce sont des lectures, que l'interface relance d'elle-même. Les autres actions passent encore par l'ancien « Traitement… » : import, requêtes d'import, création de TCD, etc.

**Méthode.** Mesures faites par `TestMesureRetourVisuel` (`internal/canvas/work_test.go`) sur un tableau de 10 colonnes, au niveau du modèle. L'interface ajoute ensuite une mise en page (voir la ligne « mise en page après »). Commande : `LATTIS_MESURE=1000000 go test ./internal/canvas -run TestMesureRetourVisuel -v -timeout 60m`. Pour chaque opération, la durée de chaque étape est celle que la barre d'état affiche. Les étapes de moins de 20 ms ne sont pas listées. Poste de développement, Windows 10.

## Temps et étapes

| Opération | 1 M lignes | 10 M lignes | Étapes à 10 M (s) |
|---|---:|---:|---|
| Supprimer une colonne pleine (Prix) | 3,4 s | 40 s | copie pour l'annulation 12,5 · réécriture 19,3 · écriture sur le disque 7,2 |
| Supprimer une colonne vide | 1,9 s (2,6 avant) | 23 s | copie 2,1 · réécriture 16,8 · disque 3,4 |
| Annuler la suppression | 3,2 s | 44 s | modification du tableau 4,9 · mise à jour des lignes 33,6 · disque 5,4 |
| Supprimer 2 colonnes | 4,1 s | 49 s | comptage 3,0 · copie 10,4 · réécriture 16,9 · indexation 9,8 · disque 6,7 |
| Insérer une colonne | 0,4 s | 5,6 s | modification du tableau 4,9 |
| Trier par Prix | 10,9 s | 149 s | copie 9,0 · classement 82,6 · nouvel ordre 33,6 · indexation 12,3 · disque 9,7 |
| Annuler le tri | 4,8 s | 46 s | nouvel ordre 29,5 · indexation 9,5 · disque 6,0 |
| Type de Qté → texte | 4,0 s | 45 s | copie 5,2 · réécriture 22,0 · indexation 9,3 · disque 6,2 |
| Mise en page après une opération de structure | 0,9 à 1,3 s | 9,2 s | contrôle de la clé 8,9 |
| Colonne calculée `[@Prix]*[@Qté]` | 0,6 s | 6,4 s | modification du tableau 4,8 |
| Insérer une ligne, supprimer 10 lignes, coller 1 000 × 2, mise en forme de 10 000 cellules | < 0,7 s | < 1,2 s | comptage des lignes 0,2 à 0,7 |

## Constats

1. **Corrigé : la corbeille d'une colonne vide.** Avant d'être supprimée, une colonne était copiée en entier dans la corbeille de l'annulation, cellules vides comprises. Seules les cellules remplies y vont désormais, et l'annulation reste exacte (`dropColumn`, `dropColumns`). Gain sur une colonne vide : 0,8 → 0,2 s à 1 M lignes, et 2,1 s à 10 M lignes (environ 8 s avant, estimé d'après 1 M). Gain proportionnel sur une colonne creuse.
2. **Le contrôle de clé coûte 0,9 s (1 M) à 9 s (10 M) à la mise en page qui suit toute opération de structure.** Exemples : suppression, insertion ou type d'une colonne. La suggestion de clé recompte la première colonne parce que le Store compte une reconstruction de la table comme une écriture de toutes ses colonnes. Correction possible : garder la version de la première colonne quand une reconstruction ne la change pas, ou calculer la suggestion en arrière-plan au-delà de 1 M lignes. Ni l'une ni l'autre n'est immédiate (suivi des versions du Store). C'est la suite de la piste A de la réflexion du 21/10. **C'est le gain le plus net pour l'utilisateur.**
3. **Ajouter une colonne parcourt toute la table : 0,4 s à 1 M lignes, 4,9 s à 10 M.** Sur une table STRICT, SQLite vérifie toutes les lignes (`pragma_quick_check`) à chaque `ALTER TABLE … ADD COLUMN`. Cela touche l'insertion d'une colonne, la création d'une colonne calculée et l'annulation d'une suppression. Pas de correction simple : il faudrait renoncer à STRICT ou ajouter la colonne par une reconstruction.
4. **L'annulation d'une suppression de colonne est lente : 34 s à 10 M lignes.** Elle remet les valeurs par un `UPDATE … FROM corbeille`, ligne par ligne. Une reconstruction de la table en jointure, comme `dropColumns` le fait déjà pour plusieurs colonnes, devrait diviser ce temps par deux.
5. **Tri : le classement (`row_number()`) prend 55 % du temps.** Le reste se partage entre la réécriture de `_pos` et la reconstruction de l'index. C'est la piste F, déjà connue (positions dans une table étroite).
6. **L'écriture sur le disque prend 5 à 10 s à 10 M lignes.** C'est la validation en `synchronous=FULL` d'une transaction de plusieurs centaines de Mo dans le WAL. Elle est incompressible tant que l'annulation garde une copie complète des données.
7. **Le comptage des lignes (`count(*)`) prend 0,2 à 0,7 s à 10 M lignes au début de presque chaque action.** Il est relu après chaque écriture qui change les lignes. Il est en cache entre deux actions et fait partie du chantier « vue d'abord » (nombre de lignes gardé).

## Suite du constat 2 : contrôle de la clé gardé (22/10)

**Demande de Louis.** « Le contrôle de la clé, c'est à refaire que si le nombre de lignes change ou si on modifie les colonnes de la clé. Sinon, non. »

**Ce qui a été fait.**

- **Versions par colonne sans les métadonnées** (`internal/store/colver.go`). Un calcul peut demander sa version sans les métadonnées (`store.NoMeta`) et met dans la clé de son cache ce qu'il en lit (identifiant et type de la colonne, clé, « ne plus proposer » lu hors du cache). La version d'une colonne n'avance plus que si des lignes sont ajoutées ou supprimées, si une écriture peut toucher toutes les colonnes, ou si cette colonne est écrite.
- **Le Store reconnaît les changements de structure** : `ALTER TABLE … ADD|DROP COLUMN c_K` ne touche que c_K ; une reconstruction (`t_N_rebuild` remplie depuis `t_N`, puis renommée, dans la même tâche) garde la version des colonnes recopiées telles quelles ; une écriture dans une corbeille ou une table de rangs ne fait que lire `t_N` ; un index créé ou supprimé ne change rien. Toute autre écriture qui nomme `t_N`, ou qui écrit des lignes sans les nommer (déclencheur, vue), compte toujours pour toutes les colonnes.
- **L'ordre des lignes est une colonne à part** (`_pos`) : un tri ne refait pas le contrôle de la clé, il refait seulement la numérotation automatique, qui dépend de l'ordre.
- **Même règle pour la numérotation automatique** (première colonne et clé, plus l'ordre) et pour le **comptage des lignes** (`store.CountOf` : ni une saisie, ni un tri, ni un changement de colonne ne le refont).
- Le contrôle est refait après : ajout ou suppression de lignes (saisie, collage, onglet SQL, scripts), modification de la colonne contrôlée (saisie, collage, recherche et remplacement, effacement, onglet SQL, annulation), suppression ou conversion de cette colonne, ou une autre colonne devenue la première. Essais : `TestColumnVersionStructure` (Store) et `TestKeyHintCacheKept`, `TestKeyHintCacheDropped` (modèle).
- **Limite connue** : une requête de l'onglet SQL passe par des vues et des déclencheurs. Le Store ne sait pas quelles colonnes elle écrit, et elle compte encore pour tout le tableau, même si elle ne modifie qu'une autre colonne.

**Mesures** (`TestMesureRetourVisuel`, mise en page après chaque opération, 1 M lignes × 10, même poste) :

| Mise en page après… | Avant | Après |
|---|---:|---:|
| insérer une colonne | 882 ms (contrôle de la clé 858) | 2 ms |
| supprimer une colonne (vide ou pleine) | 884 à 922 ms | 1 ms |
| annuler la suppression | 882 ms | 1 ms |
| supprimer 2 colonnes, puis annuler | 868 et 882 ms | 1 ms |
| type d'une autre colonne (Qté → texte), puis annuler | 907 et 890 ms | 1 à 2 ms |
| trier, puis annuler le tri | 910 et 902 ms | 1 ms |

L'insertion de colonne et la suppression d'une colonne ne recomptent plus les lignes non plus (environ 22 ms de moins à 1 M lignes).

À 10 M lignes, la mise en page après chacune de ces opérations (insertion, suppressions, annulations, tri, type d'une autre colonne) prend désormais 1 à 2 ms, contre 9,2 s avant (contrôle de la clé 8,9 s, ligne « Mise en page après une opération de structure » du tableau plus haut). La première mise en page (8,8 s) et celle qui suit un ajout ou une suppression de lignes (8,7 s) refont le contrôle, comme il se doit.

## Captures (version web, 1 M lignes × 10)

- Suppression de la colonne Ville : reflet sur la colonne, « Suppression de la colonne Ville : réécriture du tableau », bouton Annuler.
- Tri : reflet sur tout le tableau, « Tri par Client : classement des lignes · 3 s ».
- Ouverture : « Mise en page de la feuille : contrôle de la clé » (constat 2).
