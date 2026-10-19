# Tableau publié seul au Data Lake (source poussée)

Deuxième voie de **clic droit sur un tableau › Partager au Data Lake Lattis…** : *publier uniquement ce tableau comme nouvelle source de données liée à ce classeur*, sans publier le classeur. La première voie publie tout le classeur avec la gestion des droits (Fichier › Publier sur un serveur…), puis la boîte revient pour partager le tableau comme aujourd'hui.

Contrat du serveur : `be-lattis-server`, migrations `schema/0027_pushed_sources.sql` et `schema/0028_pushed_sources_suite.sql`, `docs/contrat-client.md` §9.1. Client : `internal/remote/workbook/pg_source.go` (API SQL et envoi HTTP), `internal/bridge/table_source.go` (lien, envoi, colonnes, mode), `frontend/src/lib/remote/TableSourceForm.svelte`, `TableSourcesList.svelte`, `DataCatalogDialog.svelte`, `source.ts`.

## 1. Ce que fait le serveur

`lattis.create_pushed_source` crée, pour un publieur de données (`login_summary().can_publish_data`) :

- un **classeur technique** (`lattis.workbooks.kind = 'source'`, collaboratif, publié, absent de `my_workbooks` et des listes) dont l'utilisateur est **propriétaire**, avec un seul tableau ;
- un **pipeline** `source_kind = 'push'` (identité technique « Pipeline : <nom> », éditeur du classeur technique), mode `replace`, `append` ou `upsert` avec `key_columns` ;
- la **donnée** du catalogue sur ce tableau (`create_dataset`), dont l'audience se règle comme toute donnée.

Tout l'existant des données s'applique : vue `donnees.<sql_name>`, tableaux liés et leurs relevés, « Utilisée par », administration des pipelines (exécutions dans `lattis.pipeline_runs`). Avec 0028, `data_catalog().datasets[].source` porte, pour qui gère la donnée, `kind` (`source` ou `workbook`), `pipeline_id` et `origin` (`{workbook, workbook_local_id, table, table_local_id}`) ; `title` reste « Source : <nom> ».

## 2. Appels du client

| Besoin | Appel | Client |
|---|---|---|
| Le serveur sait-il faire ? | `to_regprocedure('lattis.create_pushed_source(jsonb)')` | `PushedSourcesSupported` ; sinon la voie est fermée avec la raison |
| Créer | `create_pushed_source(p) → {pipeline_id, dataset_id, workbook_id, table_id, sql_name}` ; `p` : `name`, `description`, `folder_id`, `sql_name`, `table_name` (nom du tableau local), `columns` (`[{name, type}]`, types de `create_table`), `mode`, `key_columns`, `origin` (`{workbook: titre, workbook_local_id, table, table_local_id}`) | `CreatePushedSource` |
| Envoyer | `POST <émetteur>/api/push/<pipeline_id>?wait=60`, `Authorization: Bearer <jeton d'accès>`, `Content-Type: text/csv`, CSV UTF-8, en-tête = noms des colonnes | `PushClient.Push` |
| Suivre un envoi long (202) | 0028 : `GET <émetteur>/api/push/<pipeline_id>?run=<run_id>` (`run_id` de la réponse 202) → `{id, status: pending}`, `{status: running}`, puis le bilan ; avant 0028 (pas de `run_id`, ou `status` absent) : `GET <émetteur>/api/push/<pipeline_id>` (`pending`, `last_run`). Toutes les 2 s, 10 min au plus | `PushClient.PushRun`, `PushClient.PushStatus`, `bridge.waitRun` |
| Mode, clé, suspension | `update_pushed_source(pipeline_id, {mode, key_columns, enabled})` (0028) ; serveur sans (`42883`) : l'application dit de retirer puis republier la source | `UpdatePushedSource` |
| Colonnes changées en local | `add_column`, `rename_column`, `drop_column`, `alter_column_type` sur `workbook_id` / `table_id` (une transaction, `Server.Write`) | `bridge.syncColumns` |
| Nom, description, dossier, audience | `update_dataset`, `add_data_audience…`, `remove_data_audience` | comme un tableau publié |
| Retirer | `drop_pushed_source(pipeline_id)` (`*withdrawn:<dataset_id>`) ; inconnue : le lien est seulement oublié | `DropPushedSource` |
| Lister | `my_pushed_sources()` (sans 0027 : `42883`, liste vide) | `DocumentService.PushedSources`, montrée dans Insertion › Data Lake Lattis |

**Émetteur et jeton** : l'émetteur est `issuer` de la découverte (`/.well-known/lattis.json`, l'adresse du serveur si absent) ; le jeton est le jeton d'accès de la session PostgreSQL (`auth.Manager.Token`). Un `401` fait oublier le jeton (`Manager.Invalidate`) et réessayer une fois avec un jeton rafraîchi.

**CSV envoyé** (`bridge.sourceCSV`) : séparateur `;`, première ligne = noms des colonnes envoyées, décimales avec un point, dates `AAAA-MM-JJ`, booléens `true`/`false`, vide = NULL. Les valeurs viennent de `canvas.Model.ExportTable` (colonnes calculées : leurs résultats, pas leurs formules), converties au type de la colonne (`workbook.ToRemote`) ; une erreur de formule ou une saisie hors type part vide et est comptée (« valeurs envoyées vides »).

**Réponses** : `200` bilan `{id, status ok|warning|error, rows_read, inserted, updated, deleted, rejected, rejects[{row, column, value, error}], error}` ; `status = error` est un échec (message du pipeline) ; `warning` est réussi, avec le nombre de refus et le premier refus (ligne, colonne, valeur, raison) dans la barre d'état. `202` : suivi par `GET`. `409` (envoi précédent pas encore écrit, source suspendue), `413` (plus de 100 Mo), `403`, `404` : message en français dans la barre d'état et dans l'état de la source (`LastError`).

## 3. Ce que garde le classeur

`_lattis_info`, clé `table_sources` (JSON, version 2), un lien par tableau : tableau local (`tbl`, stable au renommage), serveur, `pipeline`, `dataset`, `workbookId`, `tableId`, `sqlName`, nom, `mode`, colonnes envoyées (`cols` : identifiant local, nom et type tels que le serveur les a), `keys`, envoi à l'enregistrement, suspension, dernier envoi (date, bilan, erreur, valeurs vides). Créer, modifier ou retirer le lien rend le classeur « à enregistrer » ; un bilan d'envoi, non.

## 4. Interface

- **Partager au Data Lake Lattis…** sur un tableau d'un classeur local : choix entre publier le classeur et publier le tableau seul ; pour une source existante : nom, description, dossier, colonnes envoyées, mode (remplacer, ajouter, mettre à jour par clé) et clé modifiables, envoi à l'enregistrement, **suspendre** (plus aucun envoi, la donnée reste avec ses dernières lignes), audience, « Mettre à jour maintenant », « Retirer la source ». Changer le mode ou la clé aligne d'abord les colonnes (la clé doit exister sur le serveur).
- **Insertion › Data Lake Lattis** : une donnée publiée seule porte le badge « pipeline » ; son détail dit le classeur local d'origine (`origin.workbook › origin.table`). Sans donnée choisie, un publieur voit « Vos sources poussées » (`my_pushed_sources`) : toutes ses sources sur ce serveur, quel que soit le classeur ou le poste d'où elles viennent (origine, mode, clé, suspendue, envoi en attente, dernière mise à jour).
- Listes des sources du classeur dans Publier sur un serveur et Gérer l'accès (onglet Membres).

## 5. Quand le client envoie

- À la demande : clic droit sur le tableau › **Mettre à jour la source du Data Lake**, bouton « Mettre à jour maintenant », listes des sources.
- À chaque enregistrement si la case est cochée et la source n'est pas suspendue : en arrière-plan après `Ctrl+S`, bilan dans la barre d'état (évènement `data:source`).
- Avant chaque envoi, les colonnes envoyées renommées, converties ou supprimées dans le classeur le sont sur le serveur (ajouts d'abord, suppressions ensuite) ; une colonne ajoutée à l'envoi depuis la boîte est ajoutée sur le serveur. Un refus du serveur (colonne citée par un filtre d'audience, clé du pipeline) annule l'envoi et son message s'affiche.

Jamais de publication implicite : la voie « tableau seul » n'appelle aucune fonction de publication de classeur ; un classeur partagé dont on n'est pas concepteur ne la propose pas ; un tableau déjà publié avec son classeur se partage directement.

## 6. Ce qui manque encore

Côté serveur : rien de bloquant. `data_catalog().source.origin` n'est donné qu'à qui gère la donnée : un lecteur ne voit pas d'où vient une source poussée (seulement son nom).

Côté application :

- Les colonnes suivent au moment de l'envoi, pas au moment du changement local (pas de propagation tant qu'on n'envoie pas).
- La liste « Vos sources poussées » est en lecture seule : une source d'un autre classeur se gère depuis ce classeur (pas de reprise du lien dans un autre classeur, pas de retrait depuis la liste).
- La suspension faite ailleurs (administration) n'est connue qu'au refus `409` du prochain envoi.
- Le CSV est construit en mémoire (100 Mo au plus côté serveur).

## 7. Essais

- `internal/remote/workbook/pg_source_test.go` : `PushClient` contre un faux lattisd HTTP (CSV, en-têtes et `wait` envoyés, bilan avec refus, `202` avec `run_id` puis `GET ?run=`, `401` puis jeton rafraîchi, `409`, `413`, `404`).
- `internal/bridge/table_source_test.go`, avec le faux serveur `workbooktest/source.go` (mêmes fonctions, CSV lu comme le pipeline : colonnes par nom, conversion par type, refus par valeur, modes ; `Before28` imite un serveur 0027) : choix proposé pour un classeur local ; création sans publication ; mise à jour par clé ; lien relu après enregistrement et réouverture ; envoi à l'enregistrement ; colonne ajoutée ; origine au catalogue ; mode changé (la clé tombe, remplacement) ; « Vos sources poussées » ; suspension et reprise ; colonnes renommée et supprimée suivies ; serveur sans 0027, clé non envoyée, upsert sans clé ; `409`, `413` ; `202` suivi par `run_id` (`pending` d'abord) puis, sans 0028, par l'état de la source ; mode refusé sans 0028 ; bilan avec refus ; conversions.
- `frontend/src/lib/remote/source.test.ts` : voies proposées, messages de la barre d'état (refus compris), état d'une source (suspendue comprise), origine au catalogue, ligne de « Vos sources poussées », nom SQL proposé.

Non essayé contre un vrai serveur : le lattisd de développement en service (`deploy/compose`) n'a ni 0027 ni 0028. Le fichier compose construit bien depuis le dépôt local du serveur, qui les contient, mais le reconstruire aurait migré la base de développement en service de Louis, et un envoi réel demande en plus de se connecter avec un compte de test (jeton OAuth).
