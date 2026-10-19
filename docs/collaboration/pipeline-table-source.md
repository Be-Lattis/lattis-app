# Source du Data Lake tirée d'un classeur local (pipeline « classeur »)

Contrat demandé à lattisd (be-lattis-server) pour la deuxième voie de **clic droit sur un tableau › Partager au Data Lake Lattis…** : *publier uniquement ce tableau comme nouvelle source de données liée à ce classeur*, sans publier le classeur.

État : le client est fait (`internal/remote/workbook/pg_source.go`, `internal/bridge/table_source.go`, boîte `frontend/src/lib/remote/TableSourceForm.svelte`) et essayé contre le faux serveur (`workbooktest/source.go`). Le serveur n'a pas encore ces fonctions : le client le détecte (`to_regprocedure('lattis.create_table_source(jsonb)')`, sinon `42883`) et le dit à l'utilisateur au lieu de proposer la voie.

## 1. Pourquoi l'existant ne suffit pas

- `lattis.create_dataset` (migration 0018) ne partage qu'un tableau d'un classeur **publié** dont on est propriétaire ou concepteur : c'est la première voie (publier le classeur avec ses droits).
- `lattis.create_pipeline` (migration 0020) est réservé aux **propriétaires du serveur** (`require_server_owner`), écrit dans un tableau d'un classeur **collaboratif publié**, et sa source est lue **par lattisd** (dépôt, HTTP, SFTP, SQL). Rien ne permet à un poste de pousser des lignes, ni à un publieur de données de créer un pipeline, ni de tenir un tableau hors d'un classeur publié.

## 2. Principe proposé

Une source « classeur » est un pipeline de `source_kind = 'workbook'` : la source est le classeur local d'une personne, qui envoie des **instantanés complets** du tableau. lattisd garde la copie dans un **classeur porteur** qu'il crée lui-même (collaboratif, publié, sans membre humain, absent de `my_workbooks`, un par source), si bien que tout l'existant des données s'applique sans changement : vue `donnees.<sql_name>`, audience et filtres, `data_catalog`, `dataset_tombstones` / `dataset_changed_ids`, notifications sur le canal du classeur porteur, « Utilisée par », administration `/admin/pipelines` (avec les exécutions dans `lattis.pipeline_runs`).

Ce qui est partagé : les **valeurs** des colonnes cochées de ce tableau, à chaque envoi. Ce qui ne l'est pas : le classeur (autres tableaux, feuilles, formules, scripts, mise en page), les colonnes non cochées, l'historique. Le classeur n'apparaît pas sur le serveur ; seuls son titre et le nom du tableau sont notés comme origine.

## 3. Fonctions (schéma `lattis`, `SECURITY DEFINER`, `GRANT EXECUTE … TO PUBLIC`)

### 3.1 `lattis.create_table_source(p jsonb) RETURNS jsonb`

- **Droits** : `lattis.require_data_publisher()` (publieur de données ou propriétaire du serveur). Pas de rôle sur un classeur.
- **p** :
  - `name` (texte, 1 à 200), `description` (texte), `folder_id` (uuid ou absent : racine), `sql_name` (absent ou vide : `lattis.dataset_sql_name(name)` ; sinon mêmes règles et même refus `23505` que `create_dataset`) ;
  - `columns` : `[{ "name": "Produit", "type": "text" }, …]`, types de `lattis.create_table` (`text`, `integer`, `decimal`, `date`, `datetime`, `boolean`), au moins une ;
  - `key_columns` : noms parmi `columns` (peut être vide) ;
  - `origin` : `{ "workbook": "<identifiant aléatoire du classeur local>", "title": "Ventes 2026", "table": "Ventes" }` (affichage seulement).
- **Effet**, dans une transaction : classeur porteur (titre = `origin.title`, mode collaboratif, publié, `kind`/drapeau « porteur » pour l'exclure de `my_workbooks`, de la création de rôles et de l'ouverture) avec un tableau (`origin.table`) et ses colonnes ; identité technique « Pipeline : <name> » comme `create_pipeline`, éditeur du classeur porteur ; ligne `lattis.pipelines` avec `source_kind = 'workbook'`, `mode = 'upsert'` si `key_columns` sinon `'replace'`, `schedule_kind = 'manual'`, `user_id` = l'identité technique, `created_by` = `lattis.actor()`, `source = {"owner": <id de l'appelant>, "origin": …}` ; donnée `lattis.datasets` sur ce tableau (`inherit_audience = true`), avec dans `data_catalog` une origine distinguable (voir 3.6). `audit_log('create_table_source', …)`. `NOTIFY lattis_data, '*catalog'`.
- **Rend** : `{ "pipeline_id": uuid, "dataset_id": uuid, "sql_name": text }`.
- **Refus** : `42501` (pas publieur), `22023` (nom, colonnes ou clé invalides : « colonne clé inconnue : X »), `23505` (nom SQL pris), `P0002` (dossier inconnu).
- `CHECK (source_kind IN (…))` de `lattis.pipelines` et `pipelines_upload_manual` à étendre à `'workbook'` ; `workbook_id`, `table_id`, `table_name` pointent le classeur porteur.

### 3.2 Envoi : `begin_table_push`, `push_table_rows`, `finish_table_push`

Le client les appelle dans **une seule transaction** (tout ou rien) : `begin`, puis `push_table_rows` par paquets de 2 000 lignes, puis `finish`.

`lattis.begin_table_push(p_id uuid, p jsonb) RETURNS uuid` (le run)

- **Droits** : l'appelant est `source->>'owner'` du pipeline, ou propriétaire du serveur ; sinon `42501` (« seul le créateur de la source l'alimente »). Pipeline inconnu ou pas `workbook` : `P0002`. Pipeline désactivé (`enabled = false`) : `55000`.
- **p** : `columns` (comme 3.1, l'ordre de l'envoi), `key_columns`, `origin`, `rows` (nombre annoncé, pour la mesure).
- **Structure** : aligne le tableau porteur sur `columns` avec les fonctions de `0025_structure.sql` (sans passer par le poste) : colonne nouvelle ajoutée, absente supprimée, type changé converti ; refus de ces fonctions remontés tels quels (colonne citée par un filtre d'audience : `2BP01`, texte en français). Clé : `key_columns` remplace celle du pipeline (`pipeline_check`). Une colonne garde son identité par son nom.
- Pose `running_since` (une exécution à la fois, comme I6 de 0023 : `55P03` si déjà en cours), crée la ligne `pipeline_runs` (`trigger = 'workbook'`, `status = 'running'`) et une table temporaire `ON COMMIT DROP` qui reçoit les lignes.

`lattis.push_table_rows(p_run uuid, p_rows jsonb) RETURNS void`

- `p_rows` : tableau de lignes, chacune un tableau de valeurs dans l'ordre de `columns` : nombres JSON, chaînes, booléens, `null` ; dates en `"aaaa-mm-jj"`, date et heure en RFC 3339. Valeur inconvertible : la ligne est **rejetée** (comptée, gardée dans `rejects` avec son numéro et le message), l'envoi continue.

`lattis.finish_table_push(p_run uuid) RETURNS jsonb`

- Avec clé : doublon de clé dans l'envoi → refus `23505` de tout l'envoi (« clé en double aux lignes 4 et 9 : l'envoi est refusé »). Sinon, sous l'identité technique (RLS, déclencheurs, pierres tombales, `lattis_author`) : lignes nouvelles insérées, lignes changées mises à jour (`lattis_id` gardé), lignes absentes de l'envoi supprimées ; ordre (`lattis_pos`) = ordre de l'envoi. Sans clé : tout est remplacé.
- Termine le run (`status` `ok`, ou `warning` s'il y a des rejets), efface `running_since`, met `updated_at` de la donnée.
- **Notifications** : canal du classeur porteur, charge `table_id` (comme une écriture ordinaire) ; si la structure a changé, `*version:<n>` (les tableaux liés relisent tout, comme après une republication). Webhook du pipeline en cas d'échec, comme les autres pipelines.
- **Rend** : `{ "run_id", "status", "rows", "inserted", "updated", "deleted", "rejected", "rejects": [{ "row": 3, "error": "…" }] }` (au plus 100 rejets détaillés).

### 3.3 `lattis.my_table_sources() RETURNS jsonb`

Les sources dont l'appelant est le créateur (toutes pour un propriétaire du serveur) : `[{ "pipeline_id", "dataset_id", "name", "sql_name", "columns": [{name,type}], "key_columns", "origin", "owner": <adresse>, "last_run": <ligne pipeline_runs ou null> }]`.

### 3.4 `lattis.drop_table_source(p_id uuid) RETURNS void`

Créateur ou propriétaire du serveur. Retire la donnée (comme `drop_dataset` : `NOTIFY lattis_data, '*withdrawn:<dataset_id>'`, les classeurs liés gardent leur dernière copie), le pipeline (comme `drop_pipeline` : identité technique désactivée) et le classeur porteur. Inconnu : `P0002` (le client oublie alors le lien sans erreur).

### 3.5 Données existantes

- `update_dataset`, `add_data_audience`, `add_data_audience_by_email`, `remove_data_audience` servent telles quelles pour le nom, la description, le dossier, l'audience et ses filtres : `can_manage_dataset` doit accepter le créateur de la source (aujourd'hui : propriétaire ou concepteur du classeur source, que le porteur n'a pas) en plus des propriétaires du serveur.
- `drop_dataset` sur une donnée de source : refuser (`55000`, « retirez la source ») ou faire `drop_table_source`.
- `admin_pipelines()` les montre avec les autres (« Classeur local : <origin.title> › <origin.table>, par <owner> ») ; « Exécuter » n'a pas de sens (`run_pipeline` refuse `source_kind = 'workbook'` : `55000`, « alimentée par un classeur local ») ; désactiver (`update_pipeline` `enabled`) suspend les envois.
- Purge : un classeur porteur sans envoi depuis N jours peut être signalé dans l'administration (pas supprimé d'office).

### 3.6 `data_catalog()`

Pour une donnée de source, `source` vaut `{ "kind": "pipeline", "pipeline_id": …, "workbook_id": <porteur>, "title": <origin.title>, "table": <origin.table>, "hidden_columns": [], "inherit_audience": … }` (`kind` absent pour un tableau de classeur publié). `table_id` et `channel` sont ceux du porteur : les tableaux liés et l'onglet SQL fonctionnent sans changement côté client.

## 4. Côté client (fait)

- `workbook.TableSources` (interface ajoutée à `workbook.Server`) : `TableSourcesSupported`, `CreateTableSource`, `PushTableSource` (begin / paquets / finish dans une transaction), `TableSources`, `DropTableSource` ; `workbook.Unsupported(err)` reconnaît `42883`.
- Lien gardé dans le classeur : `_lattis_info`, clé `table_sources` (JSON, version 1) : tableau local (`tbl`, stable au renommage), serveur, `pipeline`, `dataset`, `sqlName`, nom, colonnes envoyées, clé, envoi à l'enregistrement, dernier envoi (date, compte rendu, erreur). Créer ou retirer le lien rend le classeur « à enregistrer » ; un compte rendu d'envoi, non.
- Envoi : lecture du tableau par `canvas.Model.ExportTable` (valeurs, y compris celles des colonnes calculées ; une erreur de formule ou une saisie hors type part vide et est comptée), conversion `workbook.ToRemote` au type figé de la colonne. À la demande (clic droit › Mettre à jour la source du Data Lake, boîte Partager au Data Lake, listes des boîtes Publier sur un serveur et Gérer l'accès) ou, si coché, après chaque enregistrement (en arrière-plan, compte rendu dans la barre d'état par l'évènement `data:source`).
- Jamais de publication implicite : la voie « tableau seul » n'appelle aucune fonction de publication ; un classeur partagé dont on n'est pas concepteur ne propose pas la voie, un tableau déjà publié se partage directement.

## 5. Essais

- `internal/bridge/table_source_test.go` (faux serveur `workbooktest`, `Fake.NoTableSources` pour un serveur sans ce contrat) : choix proposé pour un classeur local, création sans publication (aucun classeur visible, partage inchangé, catalogue avec `kind = pipeline`), envoi avec clé (une ligne modifiée), lien relu après enregistrement et réouverture, envoi à l'enregistrement, colonne retirée, retrait ; refus : serveur ancien, clé non envoyée, clé en double (source créée, envoi refusé en entier) ; conversions des valeurs.
- `frontend/src/lib/remote/source.test.ts` : voies proposées, messages de la barre d'état, état d'une source, nom SQL proposé.

À vérifier quand le serveur aura ces fonctions : essai de `pg_source.go` contre PostgreSQL (le client n'a été essayé que contre le faux serveur), lecture d'une source par un autre classeur (Insertion › Data Lake Lattis) avec relevé des changements, refus de structure par un filtre d'audience.
