# Tableau publié seul au Data Lake (source poussée)

Deuxième voie de **clic droit sur un tableau › Partager au Data Lake Lattis…** : *publier uniquement ce tableau comme nouvelle source de données liée à ce classeur*, sans publier le classeur. La première voie publie tout le classeur avec la gestion des droits (Fichier › Publier sur un serveur…), puis la boîte revient pour partager le tableau comme aujourd'hui.

Contrat du serveur : `be-lattis-server`, migration `schema/0027_pushed_sources.sql` et `docs/contrat-client.md` §9.1. Client : `internal/remote/workbook/pg_source.go` (API SQL et envoi HTTP), `internal/bridge/table_source.go` (lien, envoi, colonnes), `frontend/src/lib/remote/TableSourceForm.svelte`, `TableSourcesList.svelte`, `source.ts`.

## 1. Ce que fait le serveur

`lattis.create_pushed_source` crée, pour un publieur de données (`login_summary().can_publish_data`) :

- un **classeur technique** (`lattis.workbooks.kind = 'source'`, collaboratif, publié, absent de `my_workbooks` et des listes) dont l'utilisateur est **propriétaire**, avec un seul tableau ;
- un **pipeline** `source_kind = 'push'` (identité technique « Pipeline : <nom> », éditeur du classeur technique), mode `replace`, `append` ou `upsert` avec `key_columns` ;
- la **donnée** du catalogue sur ce tableau (`create_dataset`), dont l'audience se règle comme toute donnée.

Tout l'existant des données s'applique : vue `donnees.<sql_name>`, `data_catalog` (`source.workbook_id` = classeur technique), tableaux liés et leurs relevés, « Utilisée par », administration des pipelines (exécutions dans `lattis.pipeline_runs`).

## 2. Appels du client

| Besoin | Appel | Client |
|---|---|---|
| Le serveur sait-il faire ? | `to_regprocedure('lattis.create_pushed_source(jsonb)')` | `PushedSourcesSupported` ; sinon la voie est fermée avec la raison |
| Créer | `create_pushed_source(p) → {pipeline_id, dataset_id, workbook_id, table_id, sql_name}` ; `p` : `name`, `description`, `folder_id`, `sql_name`, `table_name` (nom du tableau local), `columns` (`[{name, type}]`, types de `create_table`), `mode`, `key_columns`, `origin` (`{workbook: titre, workbook_local_id, table, table_local_id}`) | `CreatePushedSource` |
| Envoyer | `POST <émetteur>/api/push/<pipeline_id>?wait=60`, `Authorization: Bearer <jeton d'accès>`, `Content-Type: text/csv`, CSV UTF-8, en-tête = noms des colonnes | `PushClient.Push` |
| Suivre un envoi long (202) | `GET <émetteur>/api/push/<pipeline_id>` (`pending`, `last_run`) toutes les 2 s, 10 min au plus | `PushClient.PushStatus`, `bridge.waitRun` |
| Colonnes changées en local | `add_column`, `rename_column`, `drop_column`, `alter_column_type` sur `workbook_id` / `table_id` (une transaction, `Server.Write`) | `bridge.syncColumns` |
| Nom, description, dossier, audience | `update_dataset`, `add_data_audience…`, `remove_data_audience` | comme un tableau publié |
| Retirer | `drop_pushed_source(pipeline_id)` (`*withdrawn:<dataset_id>`) ; inconnue : le lien est seulement oublié | `DropPushedSource` |
| Lister | `my_pushed_sources()` | `PushedSources` (pas encore utilisé par l'interface) |

**Émetteur et jeton** : l'émetteur est `issuer` de la découverte (`/.well-known/lattis.json`, l'adresse du serveur si absent) ; le jeton est le jeton d'accès de la session PostgreSQL (`auth.Manager.Token`). Un `401` fait oublier le jeton (`Manager.Invalidate`) et réessayer une fois avec un jeton rafraîchi.

**CSV envoyé** (`bridge.sourceCSV`) : séparateur `;`, première ligne = noms des colonnes envoyées, décimales avec un point, dates `AAAA-MM-JJ`, booléens `true`/`false`, vide = NULL. Les valeurs viennent de `canvas.Model.ExportTable` (colonnes calculées : leurs résultats, pas leurs formules), converties au type de la colonne (`workbook.ToRemote`) ; une erreur de formule ou une saisie hors type part vide et est comptée (« valeurs envoyées vides »).

**Réponses** : `200` bilan `{status ok|warning|error, rows_read, inserted, updated, deleted, rejected, rejects[{row, column, value, error}], error}` ; `status = error` est un échec (message du pipeline) ; `warning` est réussi, avec le nombre de refus et le premier refus (ligne, colonne, valeur, raison) dans la barre d'état. `202` : suivi par `GET`. `409` (envoi précédent pas encore écrit, source suspendue), `413` (plus de 100 Mo), `403`, `404` : message en français dans la barre d'état et dans l'état de la source (`LastError`).

## 3. Ce que garde le classeur

`_lattis_info`, clé `table_sources` (JSON, version 2), un lien par tableau : tableau local (`tbl`, stable au renommage), serveur, `pipeline`, `dataset`, `workbookId`, `tableId`, `sqlName`, nom, `mode`, colonnes envoyées (`cols` : identifiant local, nom et type tels que le serveur les a), `keys`, envoi à l'enregistrement, dernier envoi (date, bilan, erreur, valeurs vides). Créer, modifier ou retirer le lien rend le classeur « à enregistrer » ; un bilan d'envoi, non.

## 4. Quand le client envoie

- À la demande : clic droit sur le tableau › **Mettre à jour la source du Data Lake**, bouton « Mettre à jour maintenant » de la boîte, listes des sources dans Publier sur un serveur et Gérer l'accès (onglet Membres).
- À chaque enregistrement si la case est cochée : en arrière-plan après `Ctrl+S`, bilan dans la barre d'état (évènement `data:source`).
- Avant chaque envoi, les colonnes envoyées renommées, converties ou supprimées dans le classeur le sont sur le serveur (ajouts d'abord, suppressions ensuite) ; une colonne ajoutée à l'envoi depuis la boîte est ajoutée sur le serveur. Un refus du serveur (colonne citée par un filtre d'audience, clé du pipeline) annule l'envoi et son message s'affiche.

Jamais de publication implicite : la voie « tableau seul » n'appelle aucune fonction de publication de classeur ; un classeur partagé dont on n'est pas concepteur ne la propose pas ; un tableau déjà publié avec son classeur se partage directement.

## 5. Ce qui manque encore

Côté serveur :

- **Changer le mode ou la clé** d'une source après sa création : `update_pipeline` est réservé aux propriétaires du serveur. L'application le refuse et propose de retirer puis republier la source. Une fonction `update_pushed_source(pipeline_id, {mode, key_columns, enabled})` pour le propriétaire de la source suffirait.
- **Suivre un envoi long** : `GET /api/push/<id>` ne dit pas quel envoi est le dernier ; le client attend que `pending` soit faux et que `last_run` soit terminé. Rendre l'identifiant de l'exécution dans la réponse `202` (et l'accepter en paramètre du `GET`) lèverait l'ambiguïté si un autre poste envoie en même temps.
- **Nom du classeur technique** : « Source : <nom> » ; le titre du classeur local n'est que dans `origin`. `data_catalog().source.title` montre donc « Source : … ».

Côté application :

- Les colonnes suivent au moment de l'envoi, pas au moment du changement local (pas de propagation tant qu'on n'envoie pas).
- `my_pushed_sources()` n'est pas encore montré (sources créées depuis un autre poste ou un autre classeur).
- Le CSV est construit en mémoire (100 Mo au plus côté serveur).

## 6. Essais

- `internal/remote/workbook/pg_source_test.go` : `PushClient` contre un faux lattisd HTTP (CSV, en-têtes et `wait` envoyés, bilan avec refus, `202`, `401` puis jeton rafraîchi, `409`, `413`, `404`).
- `internal/bridge/table_source_test.go`, avec le faux serveur `workbooktest/source.go` (mêmes fonctions, CSV lu comme le pipeline : colonnes par nom, conversion par type, refus par valeur, modes) : choix proposé pour un classeur local ; création sans publication (aucun classeur visible, partage inchangé) ; mise à jour par clé ; lien relu après enregistrement et réouverture ; envoi à l'enregistrement ; colonne ajoutée ; mode non modifiable ; colonne renommée et supprimée en local suivies sur le serveur ; serveur sans 0027, clé non envoyée, upsert sans clé ; `409`, `413`, `202` ; bilan avec refus ; conversions.
- `frontend/src/lib/remote/source.test.ts` : voies proposées, messages de la barre d'état (refus compris), état d'une source, nom SQL proposé.

Non essayé contre un vrai serveur : le lattisd de développement en service n'a pas la migration 0027, et le reconstruire sortait du cadre de ce travail.
