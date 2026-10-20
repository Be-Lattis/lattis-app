# Classeurs figés : publication, ouverture, lecture seule

*Étape 3 du découpage de la spécification « Lattis collaboratif : modifications de l'application » (§15) : §5.1, §5.3, §5.4, §10 (consultation hors ligne) et §9 (types figés) pour ce dont la publication a besoin. S'appuie sur la connexion (`connexion.md`) et le journal logique (`journal-logique.md`). Le contrat suivi est celui de `lattisd` (be-lattis-server, branche `main`, `docs/contrat-client.md` §5, `schema/0003_workbooks.sql`, `schema/0004_collaborative.sql`).*

## 1. Ce qui est fait

| Élément | Où |
|---|---|
| API SQL du serveur derrière une interface (`workbook.Server` : `Publisher` pour une publication, `Reader` pour une lecture cohérente, `Listen`) ; implémentation pgx (`workbook.PG`) ; serveur en mémoire pour les essais (`workbooktest.Fake`, mêmes règles : brouillon, version de travail, droits, noms, notifications) | `internal/remote/workbook/server.go`, `pg.go`, `workbooktest/fake.go` |
| **Fichier › Publier sur un serveur…** : serveur (sessions ouvertes du menu Compte), titre, mode figé ou collaboratif, tableaux à publier (les autres restent **privés**), vérifications | `frontend/src/lib/remote/ShareDialog.svelte`, `DocumentService.ShareCheck`, `SharePublish` |
| Vérifications avant publication : noms valides en PostgreSQL (`lattis.check_name` : 1 à 63 octets, sans espaces autour, préfixe `lattis_` réservé, unicité sans casse), colonnes saisies (une colonne calculée n'est pas publiée), saisies invalides listées avec quelques exemples, **à corriger ou à accepter comme texte** (`Model.AcceptAsText` : la colonne passe en texte avec ses saisies telles quelles, annulable), types inférés signalés « figés à la publication » | `workbook.Inspect`, `Check.Blocking`, `canvas/convert.go` |
| Publication en **une seule transaction du serveur** : `create_workbook` (ou `republish`), `my_capabilities` (schéma), `create_table` par tableau, `COPY` (pgx `CopyFrom`) avec `lattis_id` (UUIDv7) et `lattis_pos`, `save_meta('layout', …)`, `publish_script` par script, `finish_publish`, `my_capabilities` (canal, rôles). Un échec défait tout côté serveur ; rien n'est noté en local | `workbook.Publish`, `publish.go` |
| Clés d'ordre réparties avant publication (`order.SpreadSQL`, dans l'ordre courant) | `publish.go` (`spread`) |
| Après publication : `_lattis_remote_tables` (tableau publié, nom et `table_id` du serveur, correspondance des colonnes en JSON), `_lattis_row_ids` (identifiants posés), **types figés** (`fixed = 1`, §9), `_lattis_capabilities`, description du classeur partagé (`_lattis_info`, clé `remote`) et mise en page publiée (clé `remote_base`) | `published.record` |
| **Section `remote` du manifeste** : serveur, identifiant du classeur, schéma, mode, version, `since` par tableau publié ; la base fait foi, le manifeste est réécrit à chaque enregistrement ; une version plus ancienne de Lattis l'ignore | `internal/archive` (`Manifest.Remote`, `ReadManifest`), `workbook.Manifest` |
| **Fichier › Ouvrir depuis un serveur** : `my_workbooks()` (brouillons écartés), puis chargement complet dans un **cache local** : `<UserCacheDir>/Lattis/partages/<hôte>/<id>.lattis` (rouvert et actualisé s'il existe, ajouté aux récents) | `OpenRemoteDialog.svelte`, `DocumentService.ShareWorkbooks`, `ShareOpen`, `workbook.Load` |
| **Le cache n'est jamais montré** : barre de titre et fenêtre du système au titre du serveur avec « en ligne sur <serveur> » (« hors ligne — <serveur> »), récents notés par serveur, identifiant et titre (rouverts depuis le serveur, copie locale si le serveur ne répond pas), Enregistrer sous et exports au nom du titre, &[Fichier] / &[Chemin] = titre / serveur ; Ctrl+S dit ce qui arrive au serveur (« Enregistré sur <serveur> : <titre> », modifications en cours d'envoi, hors ligne, ou copie locale d'un classeur figé à republier) ; titre renommé sur le serveur relu à la (re)connexion et sur `*acl` | `DocState.Remote`, `RecentFile`, `remotedoc.go`, `RefreshCapabilities`, `lib/shell/doctitle.ts`, `TitleBar.svelte` |
| **Lecture seule** des tableaux publiés d'un classeur figé : grille (refus immédiat à l'édition, en-tête compris), onglet SQL, scripts, API et MCP (tout ce qui passe par le bus de commandes, par le journal logique), structure (renommer, types, colonnes, supprimer), avec le message « Classeur publié en lecture seule » | `workbook.Link` (`TableLock`, `Gate`), `canvas.Sharing`, `oplog.Recorder.Gate` |
| Permis en local, jamais renvoyé : tri, filtres, mise en forme conditionnelle, volets, cellules libres, **tableaux privés** (icône dédiée dans l'en-tête), feuilles ajoutées | idem |
| L'auteur (propriétaire ou concepteur) modifie sa copie d'un classeur figé (prochaine version) ; **Fichier › Republier** publie la nouvelle version (même chemin, dans la version de travail) | `workbook.Republish`, `DocumentService.ShareRepublish` |
| Suivi du serveur pendant que le classeur est ouvert : `LISTEN` sur le canal, version vérifiée à chaque (re)connexion, bandeau **« Nouvelle version disponible »** (bouton Recharger) sur `*version:<n>` | `workbook.Watch`, `DocumentService.watchShare`, évènement `share:changed`, `ShareBanner.svelte` |
| **Rechargement** d'une nouvelle version : mise en page et données remplacées ; ce que l'utilisateur a ajouté en local (tableaux privés, cellules libres, feuilles…) est gardé, renuméroté ou renommé si la nouvelle version a pris ses identifiants ou ses noms ; l'historique d'annulation est vidé | `workbook.Load(…, fresh=false)` |
| **Hors ligne** : le cache se rouvre sans serveur ; bandeau « Le serveur … ne répond pas : ce classeur partagé est en lecture seule » (classeur figé : aucune écriture sur les tableaux publiés ; collaboratif : modifiable, décision 0014), erreur technique dans « Détails », boutons Réessayer (ou « Se connecter… » quand la session manque), Travailler sur une copie locale, Détacher du serveur (propriétaire) ; nouveaux essais espacés de 5 s à 2 min | `Link.SetOnline`, `WatchWith`, `workbook.Backoff`, `ShareBanner.svelte` |
| Classeur collaboratif : publié par le même chemin ; ouvert, ses tableaux se modifient selon le niveau (`my_capabilities`) et les opérations vont dans la file d'envoi (synchronisation : étape 5) ; hors ligne aussi, selon les derniers droits connus (décision 0014) | `Link.Gate` |

## 2. Choix de conception

### 2.1 Mise en page publiée en un seul objet

Le serveur range la mise en page dans `<schéma>.lattis_meta_objects (kind, key, payload jsonb)` par `save_meta`. Un classeur figé la publie en un seul objet `kind = 'layout'`, `payload = {"key": "workbook", "format": <meta.Version>, "tables": {"_lattis_sheets": {"columns": […], "rows": [[…]]}, …}}` : les lignes des tables de métadonnées locales d'une **liste blanche** (feuilles, tableaux, colonnes, cellules libres, largeurs, hauteurs, noms, styles, règles, TCD, graphiques, boutons, pages, plages), sans celles des tableaux privés. Rien d'autre ne part : ni protection, ni requêtes favorites, ni connexions et requêtes d'import, ni hôtes HTTP autorisés (C2), ni journal. Une table de métadonnées ajoutée plus tard n'est pas publiée tant qu'on ne l'ajoute pas à la liste.

- Les mises en forme par ligne (`_lattis_styles`, clé `(tbl, rid)`) sont publiées par `lattis_id` (le classeur qui les reçoit a d'autres `rid`).
- `format` : un classeur publié par une version plus récente de Lattis est refusé à l'ouverture (« mettre Lattis à jour ») ; d'un format voisin, seules les colonnes communes sont reprises.
- La mise en page reçue (ou publiée) est gardée dans `_lattis_info` (clé `remote_base`) : au rechargement, ce qui n'y est pas, ligne à ligne, est ce que l'utilisateur a ajouté en local.

Après la publication, la mise en page d'un classeur collaboratif suit en direct dans ce même objet (`synchronisation.md` §2.14, décision 0018) ; la découper par feuille resterait possible sans changer le serveur.

### 2.2 Identifiants locaux conservés

`create_table` reçoit l'identifiant local du tableau et de chaque colonne (`local_id`), relus dans `lattis_meta_tables` / `lattis_meta_columns` : le lecteur recrée les tableaux avec les mêmes identifiants que l'auteur (`t_<id>`, `c_<id>`), si bien que la mise en page, les formules et les TCD s'appliquent tels quels. Les lignes reçoivent des `rid` 1…n dans l'ordre de `lattis_pos` ; leur `lattis_id` et `lattis_author` vont dans `_lattis_row_ids`.

### 2.3 Lecture seule par le journal logique

Le relevé du journal logique (`oplog`) voit déjà toute écriture sur un tableau publié, quel que soit son chemin. On lui donne une porte (`Recorder.Gate`) que tient `workbook.Link` : pour chaque opération, la garder dans la file (collaboratif en ligne), l'écarter (classeur figé : rien ne part, le tri reste local) ou refuser l'étape entière. La grille et les opérations de structure (renommer une colonne n'écrit aucune ligne) demandent en plus `canvas.Sharing.TableLock` pour refuser tout de suite, avec le même message. Les écritures du chargement passent hors du bus : elles ne sont ni relevées ni refusées.

Règles (`Link.lock`) : classeur local ou tableau privé → permis ; figé hors ligne → « Hors ligne, lecture seule » (collaboratif hors ligne : comme en ligne, la file attend le serveur, décision 0014) ; figé → « Classeur publié en lecture seule » sauf propriétaire ou concepteur ; collaboratif → selon le niveau du tableau (lecteur : lecture seule).

### 2.4 Rechargement

Une seule transaction locale : tableaux privés mis de côté sous un nom provisoire, tableaux publiés d'avant supprimés, tables de mise en page vidées (dans l'ordre inverse, clés étrangères), nouvelle mise en page, tableaux et lignes du serveur, mises en forme par ligne remises par `lattis_id`, puis ajouts locaux : un tableau privé dont l'identifiant est repris est renuméroté (table et index renommés, saisies invalides suivies), un nom pris devient « Nom 2 » ; une feuille locale aussi ; une feuille du serveur disparue mais encore citée par un ajout local revient en feuille locale. Le serveur l'emporte sur une même cellule ; les réglages locaux des feuilles et tableaux du serveur (filtre, zoom…) sont remis à ceux de la nouvelle version. L'historique d'annulation est vidé (`command.Forget`), la file d'envoi aussi.

Le bandeau propose le rechargement ; il n'est pas automatique, à l'ouverture comme sur notification (l'utilisateur garde la main sur le moment où ses tris locaux disparaissent).

### 2.5 Cache local

Un classeur ouvert depuis un serveur est un `.lattis` ordinaire, enregistré dans `<UserCacheDir>/Lattis/partages/<hôte>/<id>.lattis` après chaque chargement : il se rouvre par les récents sans serveur (hors ligne). Un classeur publié depuis un fichier reste ce fichier (réenregistré après la publication pour que son manifeste dise le partage) ; jamais enregistré, il est marqué modifié.

## 3. Formats

`_lattis_info`, clé `remote` (JSON) :

```json
{"server": "https://lattis.acme.fr", "serverName": "Serveur Acme", "workbookId": "…uuid…",
 "title": "Ventes 2026", "schema": "wb_ventes_2026", "channel": "wb_ventes_2026",
 "mode": "frozen", "version": 2, "roles": ["reader"], "user": "bob@acme.fr", "synced": "2026-10-18T…Z"}
```

`_lattis_remote_tables.columns` : `[{"id": "<column_id>", "name": "Prix", "local": 3, "type": "decimal"}]` ; `remote_id` : `table_id` du serveur ; `since` : `lattis.sync_point()` au chargement.

Manifeste :

```yaml
format_version: 1
application: Lattis
saved: 2026-10-18T12:00:00Z
remote:
  server: "https://lattis.acme.fr"
  workbook_id: "…"
  schema: "wb_ventes_2026"
  mode: "frozen"
  version: 2
  since:
    "3": "1234"
```

Types : texte → `text`, entier → `integer` (`bigint`), décimal → `decimal` (`numeric`, envoyé depuis le réel local), booléen → `boolean`, date → `date` ; `datetime` du serveur se lit en texte ISO 8601.

## 4. Hypothèses sur l'API SQL du serveur

Toutes vérifiées contre PostgreSQL 18 avec les migrations de la branche serveur (`TestPostgresFrozenLifecycle`, `TestPostgresCollaborativePublish`), sous `SET SESSION AUTHORIZATION <pg_role>` (ce que voit PostgreSQL après une connexion `OAUTHBEARER`).

| Appel | Utilisation et hypothèse |
|---|---|
| `lattis.my_workbooks() → jsonb` | Liste « Ouvrir depuis un serveur » : `id`, `title`, `mode`, `state`, `version`, `schema`, `roles`, `published_at`. Brouillons (`state = 'draft'`) écartés côté client. |
| `lattis.my_capabilities(id) → jsonb` | Schéma de la version courante (brouillon compris, pour la publication), `channel`, `roles`, `version`, `tables` (`table_id` → `level`). Appelée dans la transaction de publication, après `create_workbook` et après `finish_publish`. |
| `lattis.create_workbook(title, mode) → uuid` | Dans la même transaction que la suite : le rôle de propriétaire accordé par la fonction vaut pour le `COPY` qui suit (vérifié). |
| `lattis.create_table(id, name, columns jsonb) → uuid` | `columns = [{"name", "type", "local_id", "table_local_id"}]` ; types `text`, `integer`, `decimal`, `date`, `boolean`. Colonnes relues dans `"<schéma>".lattis_meta_columns` (`column_id`, `local_id`, `position`), lisibles par l'auteur dans le brouillon et la version de travail. |
| `COPY "<schéma>"."<tableau>" (lattis_id, lattis_pos, <colonnes>)` | Colonnes par leur nom ; `lattis_author` et les autres colonnes système posées par les déclencheurs. |
| `lattis.save_meta(id, 'layout', payload)` | Objet unique, clé `payload->>'key' = 'workbook'` ; relu dans `"<schéma>".lattis_meta_objects`. |
| `lattis.publish_script(id, name, source)` | Une fois par script ; la dernière version de chacun est relue dans `lattis_meta_script_versions`. |
| `lattis.finish_publish(id) → version` | Fin de publication ou bascule de la republication ; notification `*version:<n>` sur `wb_<slug>`. |
| `lattis.republish(id) → schéma de travail` | Puis les mêmes appels que la publication (tous les tableaux sont recréés). |
| `lattis.sync_point()` | Lu au chargement, gardé comme `since` (pas encore utilisé pour un classeur figé). |
| Lecture | Transaction `REPEATABLE READ` en lecture seule : capacités, `lattis_meta_tables` / `_columns` / `_objects` / `_script_versions`, lignes `ORDER BY lattis_pos, lattis_id` (décimal lu en `float8`, date en texte ISO). |
| `LISTEN "<channel>"` | Connexion dédiée, gardée tant que le classeur est ouvert. |

`abort_publish` n'est pas utilisé : la publication étant une seule transaction, un échec ne laisse pas de brouillon.

## 5. Écarts avec la branche serveur

| Point | Constat | Proposition |
|---|---|---|
| Identifiant local du tableau | `create_table` lit `p_columns -> 0 ->> 'table_local_id'` (dans le premier élément des colonnes), non documenté dans le contrat. Le client le met dans chaque élément. | Un paramètre `local_id` de `create_table`, ou le documenter. |
| `lattis_meta_tables.sheet_id` | Toujours `NULL` (`create_table` ne le reçoit pas). Le client garde les feuilles dans l'objet de mise en page. | À prévoir pour les droits par feuille (§8.1) ; la mise en page en direct (`synchronisation.md` §2.14) n'en a pas besoin. |
| ~~Notifications pendant une republication~~ | Réglé côté serveur (migration `0007_publication.sql`) : la version de travail ne notifie plus, les lecteurs ne reçoivent que `*version:<n>`. Le client garde son filtre (`VersionNotice`) pour un serveur plus ancien ; `TestPostgresFrozenLifecycle` vérifie, quand la migration est là, qu'un lecteur ne reçoit que `*version:<n>`. | — |
| Niveau dans un classeur figé | `my_capabilities` rend `reader` pour tous les tableaux, propriétaire compris. Le client se fonde sur les rôles (`owner`, `designer`) pour laisser l'auteur préparer la version suivante. | — (cohérent avec « lecture seule pour tous » côté serveur). |
| ~~Titre~~ | Réglé (migration 0007) : `my_capabilities` donne `title`, que le chargement préfère (à défaut, celui de `my_workbooks`, puis celui déjà noté, pour un serveur plus ancien). | — |
| Identifiants des tableaux d'une version à l'autre | Une republication recrée les tableaux : nouveaux `table_id`, nouveaux `column_id`. Le client relie par `local_id`. | À garder en tête pour les références serveur (restrictions, `role_grants` par tableau) qui ne survivraient pas à une republication. |
| ~~Invitations~~ | Réglé (étape 6) : écran Partager (voir `droits.md`). | — |
| Délai de grâce | Un lecteur ouvert sur l'ancienne version peut la lire `republish_grace` secondes ; au-delà, son schéma disparaît. Le client recharge sur notification (bandeau) ; une lecture après suppression échoue avec l'erreur de PostgreSQL. | Message dédié côté client si besoin. |

## 6. Essais

- `internal/remote/workbook` (faux serveur) : publication figée et ouverture (valeurs, types, mise en page, scripts, tableau privé non publié, lecture seule par saisie, en-tête, SQL, structure ; tri, cellules libres et tableau privé permis ; rien dans la file), republication et rechargement avec ajouts locaux gardés (tableau privé renuméroté, cellule libre, feuille), hors ligne (manifeste, réouverture, refus, retour en ligne), vérifications (noms, saisies invalides, « accepter comme texte » et son annulation), échecs (serveur coupé, échec en fin de publication, seconde publication, republication par un lecteur), suivi du serveur (`Watch` : en ligne, nouvelle version, coupure, reprise).
- `internal/bridge` (`TestShareFrozenWorkbook`) : parcours complet par le pont (publication, liste, ouverture dans le cache, refus par la grille et par l'API, notification, rechargement, réouverture hors ligne).
- Contre PostgreSQL 18 (`LATTIS_PG_TEST=1`, spécification §14) : `TestPostgresFrozenLifecycle`, `TestPostgresCollaborativePublish`. Variables : `LATTIS_TEST_PG_URL` (superutilisateur) et `LATTIS_SERVER_SCHEMA` (dossier des migrations `NNNN_*.sql` du serveur). Une base jetable est créée et migrée sous `lattis_service`, comme le fait `lattisd`. Exemple :

```sh
docker run -d --name lattis-pg -e POSTGRES_PASSWORD=pg -p 55433:5432 postgres:18
mkdir schema && for f in 0001_catalog 0002_server_api 0003_workbooks 0004_collaborative 0005_advanced_rights 0006_admin 0007_publication; do
  git -C ../be-lattis-server show main:schema/$f.sql > schema/$f.sql; done
LATTIS_PG_TEST=1 LATTIS_TEST_PG_URL=postgres://postgres:pg@127.0.0.1:55433/postgres?sslmode=disable \
  LATTIS_SERVER_SCHEMA=$PWD/schema go test ./internal/remote/workbook -run Postgres -v
```

  Ces essais passent par l'API SQL sans `lattisd` (pas d'`OAUTHBEARER` : connexion du superutilisateur passée sous le rôle de l'utilisateur). L'essai de bout en bout avec `lattisd` et le module `lattis_oauth` reste à écrire (Docker Compose du serveur).
- Frontend : `src/lib/remote/share.test.ts` (sélection par défaut, blocages, bandeau, Republier).

## 7. Reste à faire

- ~~Synchronisation des classeurs collaboratifs (étape 5)~~ : faite, voir `synchronisation.md`.
- Accord à la première exécution d'un script publié (§11 : aujourd'hui, l'accord par empreinte du source d'`internal/trust` s'applique).
- ~~Écran Partager (invitations, rôles)~~ : fait, voir `droits.md`.
- Graphiques ou TCD d'un auteur qui citent un de ses tableaux privés : publiés tels quels, ils s'affichent en erreur chez les lecteurs (non filtrés).
- Les réglages locaux d'un lecteur sur un tableau publié (filtre, mise en forme) ne survivent pas à un rechargement.
- Version web : `ShareOpen` écrit le cache dans le dossier de cache du moteur ; à revoir quand `lattisd` embarquera le moteur (le classeur y est déjà sur le serveur).
