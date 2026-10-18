# Journal logique et file d'envoi (classeurs partagés)

*Étape 4 du découpage de la spécification « Lattis collaboratif : modifications de l'application » (§5.2, §6.1, écriture de la file du §6.2). Rien ici ne parle au réseau : l'envoyeur et le récepteur viennent ensuite.*

## 1. Ce qui est en place

| Élément | Où |
|---|---|
| Migration 27 → 28 : `_lattis_remote_tables`, `_lattis_row_ids`, `_lattis_outbox`, `_lattis_conflicts`, `_lattis_capabilities` | `internal/meta/meta.go` |
| Type `command.Op` (set, insert, delete, move, meta), `MetaChange`, encodage JSON, inverse | `internal/command/ops.go` |
| `Tx.Record(ops ...Op)` à côté de `Tx.Apply` ; interface `command.Recorder` appelée par `Bus.Do`, `Append`, `Undo`, `Redo` dans leur transaction | `internal/command/command.go` |
| Relevé des opérations sur les tableaux publiés et écriture de la file d'envoi | `internal/remote/oplog` |
| Publication d'un tableau (identifiants partagés), dépublication, lecture de la file | `oplog.Publish`, `Unpublish`, `Published`, `RowID`, `Entries`, `Pending` |
| Rejeu « serveur » de la file pour les tests | `internal/remote/oplog/oplogtest` |
| Branchement sur chaque classeur ouvert | `internal/doc/doc.go` (`bus.Recorder = oplog.New(db)`) |

## 2. Choix de conception

### 2.1 Relevé par différence plutôt qu'instrumentation opération par opération

La spécification prévoyait d'appeler `Tx.Record` dans chaque opération de `internal/canvas`. L'inventaire (§4) compte plus de quarante chemins d'écriture, dont plusieurs réécrivent des tables entières (tri, reconstruction au changement de type, corbeilles de colonnes, dédoublonnage, SQL arbitraire, scripts) et l'annulation, qui rejoue du SQL sans repasser par `canvas`. Instrumenter chacun reviendrait à écrire une deuxième fois, à la main, ce que ses instructions SQL disent déjà, avec le risque permanent qu'une nouvelle opération l'oublie.

Le relevé se fait donc **sous** `canvas`, sur les lignes elles-mêmes :

1. **Déclencheurs temporaires** (`CREATE TEMP TRIGGER`, connexion d'écriture seulement, jamais enregistrés dans le fichier) sur chaque `t_<id>` **publié** : à chaque `INSERT`, `UPDATE`, `DELETE`, ils notent `(tableau, id)` dans la table temporaire `_lattis_cap`. Ils ne citent aucune colonne de données (seulement `id`), donc ne gênent ni `ALTER TABLE … DROP COLUMN` ni les renommages.
2. **À la fin de la transaction** (`Capture.Finish`, avant la validation), l'état d'avant de ces lignes est relu dans un **instantané des lecteurs** : en WAL, un lecteur voit la dernière transaction validée, et l'écrivain est unique (`store`). L'état d'après est lu dans la transaction. La différence ligne à ligne donne les `Op`.
3. Les opérations vont dans `_lattis_outbox` **dans la même transaction SQLite** que l'étape.

Conséquences :
- toute écriture est vue, quel que soit son chemin (saisie, collage, tri, SQL arbitraire et ses corbeilles `_lattis_sqltrash_*`, scripts `db.*`, annulation, rétablissement) : la règle « toute commande qui touche un tableau publié sans produire d'`Op` est un bug » tient par construction, et le test structurel le vérifie ;
- les opérations décrivent l'**effet net** de l'étape : une ligne ajoutée puis supprimée dans la même étape ne produit rien, une cellule réécrite à l'identique non plus (le tri réécrit toutes les clés d'ordre, seules celles qui changent partent) ;
- un tableau **non publié** n'a pas de déclencheur : il ne coûte qu'une requête de quelques microsecondes par transaction (lecture de `_lattis_remote_tables`) ;
- `Tx.Record` reste le chemin des opérations que la base ne voit pas comme des lignes (mise en page, structure, `meta`). Elles sont gardées dans le journal (`Step.Ops`), si bien que l'annulation les inverse (`Op.Inverse`). Aucune opération de `canvas` ne l'appelle encore : c'est l'étape 7 (mise en page partagée).

Structure détectée au passage (opérations `meta`, pour que l'envoyeur sache qu'une étape touche la structure, que le §6.4 fait passer par des fonctions du serveur avant l'application locale) :
- tableau disparu (`table.drop`) ou apparu (`table.create`, suivi d'un `insert` par ligne) ;
- colonne ajoutée (`column.add`), retirée (`column.remove`), type changé (`column.retype` ; les valeurs converties de cette colonne ne sont pas envoyées une à une) ;
- tableau reconstruit (`CREATE` + copie + `DROP` + `RENAME`, comme `rebuildStmts` ou `dropColumns`) : les déclencheurs, tombés avec l'ancienne table, sont reposés à l'instruction suivante (`Capture.Statement` suit les instructions de structure) et le tableau est comparé en entier.

### 2.2 Identifiants partagés : table `_lattis_row_ids`, posés à la publication

La spécification proposait deux colonnes `lattis_id` et `lattis_author` dans chaque `t_<id>`. Elles sont dans une table à part, `_lattis_row_ids (tbl, rid, lattis_id UNIQUE, lattis_author)`, clé `(tbl, rid)`, `rid` étant l'`id` local de la ligne :

- les reconstructions de tableaux (`rebuildStmts`, `dropColumns`, conversion de type, copies de feuilles, conversions en plage, extraction de colonnes) ne recopient que `id`, `_pos` et les colonnes `c_<id>` connues : une colonne cachée y serait perdue, et chaque perte vaudrait la suppression puis la réinsertion de toutes les lignes sur le serveur ;
- le SQL des utilisateurs (`SELECT *`), les exports et les vues de l'onglet SQL ne voient pas de colonnes système ;
- un classeur non partagé n'est pas touché : la migration ne modifie aucun `t_<id>`, les identifiants ne sont posés que par `oplog.Publish`, tableau par tableau (« paresseusement ») ;
- l'`id` local reste stable dans toutes ces opérations : c'est lui qui relie la ligne à son identifiant.

Une ligne ajoutée reçoit un UUIDv7 à la fin de l'étape ; une ligne supprimée perd le sien (une annulation la réinsère sous un nouvel identifiant, envoyée comme une ligne nouvelle) ; un tableau supprimé garde ceux de ses lignes, que son rétablissement réutilise. `lattis_author` est vide en local : le serveur le pose (déclencheur `session_user`), le récepteur le recopiera.

L'identifiant d'un tableau publié ne resert jamais, même supprimé (`canvas.nextTableID`) : sans cela, un nouveau tableau aurait pris l'identifiant d'un tableau publié mis à la corbeille et aurait été pris pour lui.

### 2.3 Annulation compensatoire

L'annulation et le rétablissement passent par le même relevé (le SQL inverse est rejoué, les déclencheurs le voient). La valeur « vue » (`Old`) d'un `set` ou d'un `move` est ensuite remplacée par **ce que la commande avait écrit**, lu dans ses entrées `do` de la file (même `seq`) : « la cellule vaut toujours ce que j'y ai mis » (§7), même si une modification reçue d'un collègue a changé la valeur locale entre-temps. Le rétablissement est conditionné à la valeur d'avant la commande. Une commande antérieure à la publication n'a pas d'entrées `do` : la condition reste la valeur locale.

Les écritures faites hors du bus (le futur récepteur, qui applique les changements des autres hors journal) sont notées par les déclencheurs mais effacées au début de la transaction suivante du bus : elles ne repartent pas.

## 3. Formats

### 3.1 `command.Op` (JSON compact)

```json
{"k":"set","t":3,"r":"0192f0c4-…","c":2,"o":1000,"n":1200}
{"k":"insert","t":3,"r":"0192f0c4-…","n":{"1":"Café","2":3.5},"p":"aV"}
{"k":"delete","t":3,"r":"0192f0c4-…","o":{"1":"Thé"}}
{"k":"move","t":3,"r":"0192f0c4-…","o":"aV","p":"bC"}
{"k":"meta","t":3,"m":{"object":"column","action":"retype","id":2,"before":"decimal","after":"text"}}
```

- `t` tableau local, `c` colonne locale (la correspondance avec le serveur est dans `_lattis_remote_tables.columns`) ; `r` le `lattis_id` ; `p` la clé d'ordre (`lattis_pos`).
- Valeurs : nombres JSON (relus en `json.Number`, entiers exacts), textes, `null`/absent pour vide. L'envoyeur les convertit selon le type figé de la colonne. Les réels infinis sont écrits `"Infinity"` / `"-Infinity"`.
- `insert` : `n` donne les colonnes non vides ; `delete` : `o` donne la ligne vue (information, la suppression n'est pas conditionnée, §6.2).
- Les colonnes calculées et les saisies invalides (`_lattis_invalid`) ne sont pas envoyées (types figés en mode partagé, §9) ; la mise en forme par ligne (`_lattis_styles`) relève de la mise en page (étape 7).

### 3.2 `_lattis_outbox`

| Colonne | Contenu |
|---|---|
| `id` | ordre d'envoi (`AUTOINCREMENT` : jamais réutilisé) |
| `seq` | commande du journal ; `NULL` quand le numéro a été repris par une autre commande (commandes annulées puis remplacées) |
| `kind` | `do` (commande ou suite d'une commande, `Bus.Append`), `undo`, `redo` |
| `batch` | une transaction du bus = une transaction PostgreSQL : toutes les entrées d'un même `batch` (id de la première) partent ensemble ; une grande étape est découpée en entrées de 5 000 opérations |
| `ops` | tableau JSON de `command.Op` |
| `state` | `pending`, `sent`, `conflict`, `rejected` (contrainte `CHECK`) |
| `error`, `created` | diagnostic, horodatage UTC |

Contrat pour l'envoyeur : vider la file dans l'ordre de `id`, une transaction par `batch` ; `set` → `UPDATE … WHERE lattis_id = $r AND <col> IS NOT DISTINCT FROM $o` ; `insert` → `INSERT` avec `lattis_id` et `lattis_pos` ; `delete` → `DELETE … WHERE lattis_id = $r` ; `move` → `UPDATE … SET lattis_pos = $p WHERE lattis_id = $r AND lattis_pos = $o` ; `meta` → fonctions de structure ou `save_meta`. **Ne pas supprimer les entrées `do` envoyées** tant que leur `seq` est dans le journal : l'annulation y relit ce que la commande avait écrit (la purge peut retirer les entrées `sent` dont `seq` est `NULL` ou absent de `_lattis_journal`).

## 4. Inventaire des écritures dans les tableaux (§16.3)

Toutes passent par `Model.Do` ou `Editor` (`Bus.Append`), donc par le relevé. Colonne « Op » : ce que produit l'opération sur un tableau publié. « Test » : couverture dans `internal/integration/oplog_test.go` (`TestOplogCoversCommands`, chaque action suivie de son annulation et de son rétablissement, rejoués par le miroir) ou `TestOplogRandomEdits`.

| Opération | Code | Op | Test |
|---|---|---|---|
| Saisie, effacement d'une cellule de tableau | `actions.go` `SetCell` → `setCellAt`, `setValue` | set | saisie, saisie d'un texte, effacement |
| Saisie qui élargit le type d'une colonne | `actions.go` `retype` → `rebuildStmts` | meta column.retype (+ set) | — (types figés en mode partagé, §9) |
| Saisie sous / à droite d'un tableau (agrandissement, rattachement) | `actions.go`, `adjacent.go` `addColumn`, `insertRow` | insert ; meta column.add + set | saisie sous le tableau, colonne collée à droite |
| Collage | `paste.go` `Paste` | set, insert, (meta column.add) | collage, collage qui agrandit, doublon |
| Collage, copie et déplacement de cellules (glisser, recopie) | `transfer.go` `Transfer` | set, insert | déplacement, copie de cellules |
| Effacement d'une plage | `transfer.go` `ClearRange` ; `cells.clear` | set | effacement d'une plage, effacement par la feuille |
| Cellules libres seulement | `cellmove.go` `MoveCells`, `ClearCells` | — (pas de tableau) | |
| Insérer / supprimer une ligne de tableau | `actions.go` `InsertRow`, `DeleteRow` (`ops.go` `insertRow`, `deleteRow`) | insert / delete | insertion, suppression de ligne |
| Insérer / supprimer des lignes ou colonnes de la feuille | `sheetrows.go` | insert, delete, meta column.add/remove | lignes de la feuille |
| Insérer / supprimer / déplacer des colonnes | `actions.go` `InsertColumn`, `DeleteColumn` ; `ops.go` `addColumn`, `dropColumn(s)` ; `tables.go` `MoveColumns` | meta column.add / remove (+ set à l'annulation d'une suppression) | insertion, suppression de colonne |
| Tri | `sort.go` `SortTable` (réécrit `_pos`) | move (lignes dont la clé change) | tri ; aléatoire |
| Dédoublonnage | `dedup.go` `RemoveDuplicates` | delete | dédoublonnage |
| Rechercher / remplacer | `find.go` `Replace` | set | remplacement |
| Conversion de type | `convert.go` `SetType` (`rebuildStmts`, corbeille de conversion) | meta column.retype | changement de type |
| Texte en colonnes | `textcols.go` `TextToColumns` | set, meta column.add, retype | texte en colonnes |
| Ligne d'en-tête promue | `paste.go` `PromoteHeader` | delete + renommages (mise en page) | |
| Supprimer un tableau, une feuille | `tables.go` `DeleteTables`, `sheets.go` `DeleteSheet` | meta table.drop ; annulation : table.create + insert | suppression du tableau |
| Convertir en plage | `torange.go` `ConvertToRange` | meta table.drop | conversion en plage |
| Mettre sous forme de tableau, importer, copier une feuille, extraire des colonnes | `torange.go` `FormatAsTable`, `import.go` `Import`, `sheetcopy.go`, `tables.go` `ExtractColumns` | nouveau tableau, non publié : rien ; colonnes extraites d'un tableau publié : meta column.remove | (ExtractColumns : couvert par le relevé, pas de test dédié) |
| Fusion de deux tableaux voisins | `tables.go` (`addColumn` + `UPDATE … FROM`), `adjacent.go` | meta column.add + set dans le tableau gardé ; table.drop pour l'autre | colonne collée à droite |
| Actualiser une requête (Power Query) | `queries.go` `RefreshQuery` | meta table.drop sur l'ancien tableau ; le nouveau a un autre identifiant (à republier : voir §5) | |
| SQL arbitraire (onglet SQL, `db.execute`) | `sqlexec.go` `ExecSQL` (corbeilles et déclencheurs `_lattis_sqltrash_*`) | set, insert, delete, move (différence ligne à ligne) | SQL INSERT / UPDATE / DELETE ; aléatoire |
| Scripts (`db.insert`, `db.update`, `db.delete`, boutons, événements) | `editor.go` (`Bus.Append` : une étape par script) | set, insert, delete | script ; aléatoire |
| API / MCP (`internal/api`) | appellent les opérations ci-dessus | idem | toutes les étapes du test passent par `api.Invoke` |
| Annuler / Rétablir | `command.Bus.Undo`, `Redo` | inverses, conditionnés (§2.3) | chaque étape ; `TestOplogUndoIsConditioned` |

Écritures hors du bus (aucune sur les tableaux) : `protect.go` (protection), `zoom.go`, `httphosts.go`, `doc.go` (identifiant du classeur), migrations.

## 5. Tests

- `internal/command/ops_test.go` : encodage, inverse, passage de `Do`/`Append`/`Undo`/`Redo` par le `Recorder`, opérations enregistrées gardées dans le journal.
- `internal/remote/oplog/oplog_test.go` : tableau non publié (rien), set/insert/delete/move et effet net, annulation et rétablissement conditionnés (avec une écriture hors journal entre les deux), reconstruction de table, changement de type, colonnes ajoutées et retirées, tableau supprimé puis rétabli, `Tx.Record`, grande étape découpée en entrées d'un même `batch`, numéro de commande réutilisé, réouverture (déclencheurs reposés) et dépublication.
- `internal/integration/oplog_test.go` : test structurel (§14) — après chaque commande de l'inventaire, son annulation et son rétablissement, si un tableau publié a changé, la file a grandi, et le miroir (`oplogtest.Mirror`, qui applique les opérations avec les conditions du serveur et refuse toute condition non tenue) est identique au classeur ; mêmes vérifications sur les suites aléatoires de `TestRandomEditsKeepInvariants` avec le tableau publié, réouverture et « tout annuler » compris.
- `internal/meta/migrate_test.go` : `TestMigrateEveryVersion` (schéma du format 28 identique à celui d'un classeur neuf) et `TestMigrateCollaboration` (tables vides, `t_<id>` inchangés, contraintes).
- Mesures : `LATTIS_MESURE=1000000 go test ./internal/canvas -run TestMesureOplog -v` (voir §6).

## 6. Performances

Sur un tableau **non publié**, le Recorder ne change rien de mesurable (une requête par transaction). Sur un tableau **publié**, le coût est proportionnel au nombre de lignes écrites : relecture des lignes touchées dans les deux états, puis encodage des opérations (environ 60 octets par opération dans la file). Mesures (100 000 lignes, poste de développement, tableau créé par collage ; le test charge désormais le tableau par import pour aller jusqu'à 1 M de lignes, le collage de 1 M de lignes en un tableau épuisant déjà la mémoire de SQLite sans journal logique) :

| Opération | Sans Recorder | Non publié | Publié |
|---|---|---|---|
| Collage de 100 000 lignes (nouveau tableau) | 3,2 s | 3,5 s | 3,5 s |
| Publication (100 000 identifiants) | | | 0,6 s |
| Tri (100 000 move) | 0,9 s | 0,9 s | 1,9 s |
| UPDATE SQL de toutes les lignes (100 000 set) | 1,8 s | 1,7 s | 3,2 s |
| INSERT SQL de 10 000 lignes | 0,7 s | 0,8 s | 1,4 s |

Un grand collage dans un tableau existant est déjà refusé au-delà de quelques centaines de milliers de cellules (« collez dans une zone vide ») ; un nouveau tableau n'est pas publié. Les lignes au-delà de la plus grande clé d'avant ne sont pas relues dans l'instantané d'avant (ajouts en masse).

## 7. Reste à faire (envoyeur, récepteur, droits)

- ~~**Envoyeur** (§6.2)~~ : fait (`synchronisation.md`, `internal/remote/collab`). Prévu : goroutine par classeur qui lit `oplog.Pending` dans l'ordre, une transaction PostgreSQL par `batch`, traduction des colonnes par `_lattis_remote_tables.columns`, mise à jour de `state`, `lattis.why_not` sur 0 ligne touchée, écriture de `_lattis_conflicts` ; marque « en attente » dans la grille (jointure `_lattis_row_ids` + entrées `pending`).
- ~~**Récepteur** (§6.3)~~ : fait (`canvas.Model.ApplyRemote`). Prévu : appliquer les changements reçus **hors journal** (`store.Write` direct) dans `t_<id>`, `_pos`, `_lattis_row_ids` (avec `lattis_author`) ; les déclencheurs notent ces lignes mais le Recorder les oublie au début de la transaction suivante du bus. Recalcul par `Store.TakeTouched` (les noms `t_<id>` des requêtes du récepteur suffisent).
- ~~**Publication** (§5.3)~~ : faite (`publication.md`) — `_lattis_remote_tables` rempli, clés `_pos` réparties, identifiants posés à la copie ; `Recorder.Gate` refuse les écritures sur un tableau publié en lecture seule et n'envoie rien d'un classeur figé.
- **Structure et mise en page** (§6.4, étape 7) : appeler `Tx.Record` avec des `meta` dans les opérations de mise en page ; faire passer les opérations de structure par le serveur avant l'application locale. Une requête actualisée (Power Query) remplace son tableau par un nouveau : à republier ou à réécrire en place.
- **Sécurité (C2)** : l'autorisateur SQLite (`internal/store/authorizer.go`) laisse passer les instructions faites par un déclencheur (`inner`) : les écritures des déclencheurs de relevé dans la table temporaire `_lattis_cap` passent pendant le SQL des utilisateurs (vérifié par les étapes SQL et script du test structurel). À garder si l'autorisateur se resserre.
- **Types figés** (§9) : la publication fixe le type des colonnes publiées (`fixed = 1`) : plus d'élargissement automatique, une saisie hors type devient une saisie invalide comme dans toute colonne au type fixé (la republication la signale). Fait depuis (`synchronisation.md`) : le refus à la saisie et le collage (valeurs refusées listées) ; restent l'import (il crée toujours un tableau neuf, non publié) et le changement de type par le concepteur (appel serveur). Avant : reste le refus à la saisie demandé par le §9 en collaboratif (aujourd'hui une saisie invalide n'écrit rien dans la colonne, donc rien ne part), le collage et l'import (valeurs refusées listées) et le changement de type par le concepteur (appel serveur).
