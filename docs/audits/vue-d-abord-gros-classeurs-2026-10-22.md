# La vue d'abord sur les gros classeurs : 22 octobre 2026

**Demande de Louis.** Sur un gros classeur, la grille reste longtemps blanche à l'ouverture, ou au retour sur la feuille (depuis l'onglet SQL, Script, une autre page du ruban, une autre feuille). La première requête doit présenter la zone visible. Le comptage des lignes, la vérification des références et le reste viennent après.

**Méthode.** `TestMesureVue` (`internal/bridge/vue_mesure_test.go`) construit un classeur de 1 M, puis de 10 M de lignes × 10 colonnes, avec une première colonne d'entiers 1, 2, 3…, l'enregistre et le rouvre. Il refait ensuite les appels de l'interface dans le même ordre : `reload()` à l'ouverture, puis les blocs de 200 lignes de la grille, une autre feuille et le retour. Il tourne ainsi : `LATTIS_MESURE=1000000 go test ./internal/bridge -run TestMesureVue -v`. Les profils CPU viennent de `go test -cpuprofile`. Le même classeur a aussi été ouvert dans la version web (`lattis-engine` et `npm run dev:web`), avec le temps de chaque appel relevé sur le WebSocket et un profil JavaScript de l'interface.

## Ce que la grille attendait avant d'afficher quoi que ce soit

L'interface ne dessine la feuille qu'après `Layout(feuille)`, puis demande les blocs visibles (`Window`). Jusqu'ici, `Layout` calculait tout avant de répondre :

| Étape (1 M de lignes) | Temps | Pourquoi |
|---|---:|---|
| Numérotation automatique (`autoNumbered`) | 2,1 s | Elle vérifie, avec une fonction de fenêtrage sur tout le tableau, que la première colonne suit une suite régulière. |
| Suggestion de clé (`keyHint`) | 0,5 s | `count(DISTINCT …)` sur la première colonne. |
| Nombre de lignes (`count(*)`) | 0,03 s | Gardé en cache tant que les lignes ne changent pas. |
| TCD et graphiques de la feuille | 6,5 s et 1,75 s | Mesure du 21/10 ; seulement s'il y en a sur la feuille. |
| Premier bloc : ancres du tableau (`grid.Table.Load`) | 0,34 s | Parcourt depuis Go les 1 M de `_pos` pour relever une ancre toutes les 1 000 lignes. |

**Cause principale.** Le cache de la numérotation dépendait de *tout* le tableau. Après n'importe quelle saisie dans un tableau de 1 M de lignes dont la première colonne est faite d'entiers, chaque relecture de la mise en page coûtait 2,3 s. Cela vaut pour chaque action suivie de `reload()` : une saisie, une requête SQL qui écrit, un script, Ctrl+Z. À l'ouverture, c'était 2,9 s de mise en page, puis 0,35 s pour le premier bloc.

**Retour depuis l'onglet SQL, Script ou une autre page du ruban.** La grille n'est pas détruite : elle reste montée et cachée (`visibility: hidden`), ses blocs restent en mémoire, et changer de vue ne fait aucun appel. Mesuré dans la version web, le retour est immédiat. Le blanc vient donc de ce qui relit la mise en page en revenant ou juste avant : une écriture faite dans l'onglet SQL ou par un script, une feuille pas encore vue, ou l'ouverture.

**Interface.** Profil JavaScript d'un changement de feuille : environ 30 ms de travail sur le fil principal (`applyData` 1 à 4 ms, dessin 2 à 5 ms). Rien à corriger de ce côté.

## Ce qui change

1. **`LayoutFirst`** (`internal/canvas/view.go`, liaison `DocumentService.LayoutFirst`). C'est la mise en page à afficher tout de suite : feuilles, tableaux, colonnes, largeurs, styles, cellules libres et nombre de lignes. Sur un tableau de plus de `quickRows` (20 000) lignes, ce qui parcourt tout le tableau n'est pas calculé, sauf si c'est déjà en cache : suggestion de clé, numérotation, données des TCD et des graphiques. La mise en page le signale par `Pending`, pour un tableau, un TCD ou un graphique. `Layout` reste la mise en page complète : tous les autres appelants, et les essais, gardent le même résultat.
2. **Interface** (`+page.svelte`). `reload()` lit `LayoutFirst`, dessine, puis demande `Layout` en arrière-plan (`completeLayout`), une seule fois par feuille. Le résultat remplace la mise en page si rien d'autre ne l'a relue entre-temps. En attendant (`lib/shell/layoutfirst.ts`, `keepShown`), un TCD, un graphique ou une suggestion de clé déjà affichés le restent tels quels, avec leurs anciennes valeurs. Seul ce qui n'a jamais été calculé montre une attente : « Tableau croisé : calcul… » en gris, ou « Calcul du graphique… » sur un fond qui pulse doucement (pulsation coupée avec `prefers-reduced-motion`). Les cellules pas encore lues ont déjà leur gabarit gris (`C.placeholder`, sans animation).
3. **Numérotation en cache par colonne** (`autoNumberedIn`). Elle dépend de la première colonne et de la clé (`store.ColumnOf`, qui avance aussi quand les lignes changent ou sont réordonnées), plus de tout le tableau. Une saisie dans une autre colonne ne la recalcule plus.
4. **Ancres relevées par SQLite** (`grid.Table.anchorsOf`). Chaque ancre se lit depuis la précédente (`_pos >= ancre … LIMIT 1 OFFSET 1000`), dans une même transaction de lecture. Le nombre de lignes s'en déduit. SQLite avance dans l'index sans rendre chaque ligne à Go.
5. **Lectures internes** : la recherche du tableau avant le premier bloc (`tableInfo`) et les évènements de sélection des scripts (`RangeEventData`) passent par `LayoutFirst`. Ils n'ont besoin que des tableaux, et ne déclenchent donc plus le calcul d'un TCD avant le premier bloc.

## Temps, avant et après

| | 1 M × 10, avant | 1 M × 10, après | 10 M × 10, après |
|---|---:|---:|---:|
| Mise en page avant le premier dessin (ouverture) | 2 937 ms | 39 ms | 304 ms |
| Premier bloc (ancres comprises) | 351 ms | 66 ms | 556 ms |
| **Grille affichée après l'ouverture** | **3,3 s** | **0,1 s** | **0,86 s** |
| Mise en page complète, en arrière-plan | (avant le dessin) | 2,8 s | 30,2 s |
| Mise en page après une saisie dans une autre colonne | 2 332 ms | 1 ms | 1 ms |
| Retour sur la feuille (mise en page + bloc) | 1,6 ms | 1,2 ms | 1,0 ms |
| Ancres seules (`grid.Table.Load`, essai de la démo) | 303 ms | 26 ms | (≈ 0,27 s) |

À 10 M de lignes, l'ancien chemin attendait la mise en page complète (30 s mesurées) et le parcours des ancres (environ 3 s, estimé d'après 1 M) avant le premier dessin. Dans la version web, à l'ouverture de 1 M de lignes, les blocs visibles arrivent 375 ms après le clic, ouverture du fichier comprise (246 ms). La mise en page complète suit 2,9 s plus tard, sans rien bloquer.

## Ce qui reste

- **Nombre de lignes gardé dans le fichier.** Non fait. `count(*)` coûte 0,3 s à 10 M de lignes, une fois par ouverture, et le premier bloc relit de toute façon les lignes pour ses ancres (0,27 s). Garder un nombre de la dernière fois demanderait que la grille accepte un nombre provisoire, avec des blocs refusés au-delà de la fin réelle. Le gain est faible pour ce risque.
- **Ouverture du fichier à 10 M** : 1,9 s à chaud, pour décompresser l'archive (zstd) et exécuter `openDir`. La toute première mesure, juste après l'enregistrement, a pris 37 s : sans doute le cache disque ou l'antivirus sur un fichier de 360 Mo qui venait d'être écrit. C'est à revoir à part.
- **Un TCD ou un graphique sur un grand tableau, après une saisie dans sa source**, se recalcule encore en entier (6,5 s pour le TCD à 1 M). La saisie et la grille ne l'attendent plus, et l'ancienne valeur reste affichée pendant le calcul. La mise à jour par différence reste la piste D de la réflexion du 21/10.
- **`Layout` complet à 10 M** : 30 s de calcul en arrière-plan, pour la numérotation et la suggestion de clé, qui parcourent tout le tableau. On pourrait s'arrêter au premier doublon ou au premier écart (piste A du 21/10).
