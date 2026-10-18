# Droits dans l'application

*Étape 6 du découpage de la spécification « Lattis collaboratif : modifications de l'application » (§15) : §8.1 (application des droits) et §8.2 (écran « Partager », « Voir en tant que… », écarts). S'appuie sur la publication (`publication.md` : `workbook.Server`, `Link`, `_lattis_capabilities`) et la synchronisation (`synchronisation.md` : suivi du canal, file d'envoi). Contrat suivi : `lattisd`, `docs/contrat-client.md` §5 à §7, `schema/0005_advanced_rights.sql`, `0006_admin.sql`, `0007_publication.sql` et `0008_sync.sql` (be-lattis-server, branches `main` et `claude/sync-ecarts`).*

## 1. Ce qui est fait

| Élément | Où |
|---|---|
| **Capacités** : `lattis.my_capabilities` lu au chargement, à chaque (re)connexion et sur la notification **`*acl`** du canal (migration 0008 : rôles, niveaux, membres ou restrictions changés) ; noté dans `_lattis_capabilities` (niveau par tableau, `filters` = « filtered », `restrictions` en JSON, rôles personnalisés sur la ligne `tbl = 0`) et dans les rôles de `_lattis_info` ; une relecture sans changement ne touche pas la base. Un utilisateur retiré du classeur (`42501`) voit tous ses tableaux passer au niveau « aucun ». Message « Vos droits sur ce classeur ont changé » en barre d'état | `workbook.RefreshCapabilities`, `collab.Engine.RefreshRights`, `follow.go` (`*acl`), `workbook.Watch` (classeur figé) |
| **Lecteur** : tableau en lecture seule (« Tableau en lecture seule : vos droits ne permettent pas de le modifier ») ; **aucun** : « Tableau inaccessible avec vos droits » ; même chemin que le classeur figé (grille, en-tête, SQL, scripts, API, MCP, par `Link.lock`) | `workbook.Link` |
| **Participant** : ajout de lignes permis ; modifier, supprimer ou déplacer une ligne dont `lattis_author` n'est pas l'utilisateur est refusé **en local** avant tout envoi (« Ligne d'un autre membre : en tant que participant, vous ne modifiez que vos propres lignes »), pour la grille comme pour le SQL, les scripts, l'API et le MCP (porte du journal logique). Une ligne ajoutée sur ce poste (auteur pas encore relu) est la sienne | `Link.Gate`, `Link.foreign` |
| Grille, participant : lignes des autres membres renvoyées par fenêtre (`grid.Window.Locked`) ; saisie refusée tout de suite avec le message ; ligne légèrement grisée au survol | `Link.LockedRows`, `bridge.lockedRows`, `cache.ts`, `Sheet.svelte` (`lockedRowAt`, `hoverLocked`) |
| **« Vue filtrée par vos droits »** sous le cadre d'un tableau dont la lecture est filtrée (filtre d'un de ses rôles, ou restriction de lecture `select`/`all` qui le vise) | `canvas.TableInfo.Filtered`, `Sheet.svelte` |
| Refus du serveur : déjà traités par la synchronisation (`why_not` « forbidden », `42501`) ; les refus de l'écran Partager remontent avec le message de la fonction SQL (sans `SQLSTATE`) | `send.go`, `bridge.rightsError` |
| **Écran « Partager »** (Fichier › Partager…, menu Compte « Partager « titre »… », bouton « Modifier… » de l'onglet SQL) : onglets Membres, Rôles, Restrictions (propriétaires), Voir en tant que… et Écarts | `RightsDialog.svelte`, `FilterEditor.svelte`, `rights.ts` |
| Membres : une ligne par personne, une case par rôle (fournis et personnalisés, plusieurs par personne), dernier propriétaire protégé, « Retirer » (tous ses rôles), invitations par adresse avec rôles fournis, invitations en attente et leur retrait | `ShareInvite`, `ShareGrantRole`, `ShareRevokeInvitation` |
| Rôles : rôles fournis et personnalisés (création, suppression), arborescence classeur → feuilles → tableaux avec un niveau par nœud (aucun, lecteur, participant, éditeur) ou « hérité », filtres de lecture et d'écriture par tableau en **formulaire simple** (colonne, opérateur, valeur, « moi », « mon équipe ») ou en **SQL libre** ; contrôle local des filtres (mêmes refus que `lattis.check_predicate`) | `ShareCreateRole`, `ShareDropRole`, `ShareSetLevels` |
| Restrictions : liste, ajout (tableau, commande, tous les rôles ou certains, condition en formulaire ou SQL, libellé), retrait | `ShareAddRestriction`, `ShareDropRestriction` |
| **Voir en tant que…** : membre et tableau choisis, lignes vues par ce membre (au plus 1 000, lecture seule) par `POST preview_endpoint` de lattisd avec le jeton d'accès | `workbook.HTTPPreview`, `SharePreview`, `auth.Discovery.PreviewEndpoint` |
| **Écarts** (`lattis.check_drift`) : tableaux dont les règles ou droits ont changé hors de Lattis, règles ajoutées à la main ; « Recompiler les droits » (un niveau reposé tel quel recompile tout) | `ShareRights`, `RightsDialog.svelte` |
| Onglet SQL : section « Droits (serveur) » de la liste des objets, en lecture seule : pour un propriétaire, chaque rôle avec ses membres et les règles générées par tableau (`SELECT USING (…)`, `ALL USING (…) WITH CHECK (…)`, participant), les restrictions (`AS RESTRICTIVE FOR …`) ; pour les autres, leurs propres droits (niveau, vue filtrée, restrictions) | `rights.ts` (`rightsObjects`), `SqlView.svelte` |
| `REFERENCE` vers une ligne invisible : rien à faire dans le moteur, la ligne n'est pas dans le cache. Vérifié : par la clé, `#N/A` comme une ligne absente (`formula.Env.reference`, `slt_ref`) | `TestPostgresACL` (la ligne filtrée n'arrive pas) |

## 2. Choix de conception

### 2.1 Interface Rights, comme le reste de l'API SQL

`workbook.Server` embarque `workbook.Rights` : `ACL` (`workbook_acl`), `Invite`, `RevokeInvitation`, `CreateRole`, `DropRole`, `GrantRole`, `RevokeRole`, `SetLevels` (plusieurs `set_level` dans une transaction), `AddRestriction`, `DropRestriction`, `Drift`. Implémentation pgx (`pg_rights.go`, transactions sur `rw`) et faux serveur (`workbooktest/rights.go` : rôles fournis et personnalisés, héritage classeur → tableau, filtres gardés mais non évalués, restrictions, invitations, écarts imposés, « Voir en tant que », notification `*acl`). « Voir en tant que… » n'est pas une fonction SQL (lattisd fait `SET LOCAL ROLE` dans sa propre connexion) : interface à part `workbook.Previewer`, implémentée par HTTP et par le faux serveur.

### 2.2 Contrôle local, serveur souverain

Les capacités servent à refuser tôt et à afficher (grisé, message, indicateur) ; le serveur reste seul juge (RLS). Une restriction n'est pas évaluée en local (son prédicat n'est connu que des propriétaires) : son refus revient par la synchronisation. Le participant est contrôlé ligne par ligne par `_lattis_row_ids.lattis_author` comparé au rôle PostgreSQL noté au chargement (`Info.User`, ce que le serveur met dans `lattis_author`).

### 2.3 Feuilles

Le serveur ne connaît pas les feuilles (`lattis_meta_tables.sheet_id` toujours `NULL`, voir §4) : un niveau `set_level('sheet', …)` ne s'appliquerait à aucun tableau. L'écran montre l'arborescence du classeur local ; un niveau posé sur une feuille est posé sur chacun de ses tableaux publiés (une seule transaction), et la feuille affiche le niveau commun de ses tableaux, ou « mixte ».

### 2.4 Filtres en formulaire

Le formulaire écrit une expression que le serveur accepte telle quelle : noms entre guillemets doubles, textes entre apostrophes (doublées), nombres avec un point, dates ISO, `true`/`false`, `strpos(…) > 0` (contient), `starts_with(…)` (commence par), `coalesce(…, '') = ''` ou `IS NULL` (vide), conditions jointes par `AND`. « moi » donne `current_user` (le rôle PostgreSQL de la personne, souvent son adresse) et se compare à une colonne de texte. Un filtre relu se montre en formulaire s'il a cette forme, en SQL libre sinon.

## 3. Formats

`_lattis_capabilities` : `tbl = 0` : `level` (`editor` pour propriétaire ou concepteur, sinon `reader`), `role` (rôles fournis, séparés par des virgules), `filters` (rôles personnalisés, JSON) ; un tableau publié : `level`, `filters` (`filtered` ou `NULL`), `restrictions` (`[{"table_id": "", "command": "delete", "label": "…"}]` ou `NULL`).

## 4. Écarts avec le serveur

| Point | Constat | Proposition |
|---|---|---|
| ~~Notification de changement de droits~~ | Réglé (migration `0008_sync.sql`, branche `claude/sync-ecarts`) : `*acl` sur le canal ; le client relit `my_capabilities` (vérifié par `TestPostgresACL`). Sans la migration, les droits sont relus à chaque (re)connexion seulement. | — |
| Feuilles | `create_table` ne reçoit pas de feuille : `sheet_id` `NULL`, `set_level('sheet', …)` sans effet. Le client pose le niveau sur chaque tableau de la feuille. | Un paramètre `sheet_id` (identifiant local de la feuille) à `create_table`, et une fonction pour déplacer un tableau de feuille (étape 7). |
| Groupes d'annuaire | `members.principal` ne référence que `users` : pas de groupe (Entra ID, Google Groups…). L'onglet Membres n'a que des personnes. | Table de groupes et appartenances synchronisées par le fournisseur. |
| Invitation à un rôle personnalisé | `invite` n'accepte que les rôles fournis ; `grant_role` exige un utilisateur connu. Le client invite avec un rôle fourni, puis donne le rôle personnalisé une fois la personne membre. | `invite` qui accepte des identifiants de rôles personnalisés (appliqués à l'acceptation). |
| « mon équipe » | Aucune notion d'équipe côté serveur : l'option est refusée avec un message (« écrire la condition en SQL »). | Attribut d'équipe des utilisateurs (ou groupes), et une fonction `lattis.my_team()` utilisable dans les filtres. |
| « Voir en tant que… » pour un concepteur | `authorize_preview` l'autorise, mais `workbook_acl` (liste des membres) est réservée aux propriétaires : un concepteur ne peut pas choisir un membre. L'onglet le dit. | Une liste des membres (sans les niveaux) lisible par les concepteurs. |
| `filtered` de `my_capabilities` | Vrai pour un filtre de lecture **ou d'écriture** ; une restriction de lecture n'y compte pas. Le client ajoute les restrictions `select`/`all` qui visent l'utilisateur ; un filtre d'écriture seul affiche à tort « Vue filtrée ». | Deux drapeaux : `read_filtered` (filtre de lecture ou restriction `select`/`all`) et `write_filtered`. |
| Restrictions de l'utilisateur | `my_capabilities.restrictions` ne donne ni le prédicat ni les rôles : le client ne peut pas contrôler une restriction en local. | Acceptable (le prédicat peut révéler des données) ; à garder. |
| `REFERENCE` par rang | Sur un tableau sans clé, `REFERENCE` lit la ligne par son rang (choix de Louis) : avec un filtre de lecture, les rangs sont ceux des lignes visibles. Par la clé, une ligne invisible donne `#N/A`. | Documenter : préférer une clé pour les tableaux filtrés. |
| Écarts à recompiler | Pas de fonction pour recompiler sans rien changer : le client repose un niveau tel quel (`set_level` appelle `compile_acl`). Les règles ajoutées à la main restent. | `lattis.recompile(workbook_id)` réservé aux propriétaires (et, au besoin, suppression des règles étrangères). |
| Niveaux par feuille dans `my_capabilities` | Cohérent avec `sheet_id` `NULL`. | Avec `sheet_id`. |

## 5. Essais

- `internal/remote/collab/rights_test.go` (faux serveur) : participant (relecture, niveau, lignes verrouillées, refus local, rien dans la file, ajout puis modification de sa ligne), lecteur puis révocation (niveau « aucun »), rôle personnalisé avec filtre (filtre dangereux refusé, filtre sur le classeur refusé, indicateur), restriction de lecture, suppression du rôle, droits refusés à un non-propriétaire, invitations (en attente, retrait, utilisateur connu, rôle personnalisé refusé, dernier propriétaire), notification `*acl` suivie par `Follow`, écarts et « Voir en tant que ».
- `internal/remote/workbook/rights_test.go` : `HTTPPreview` (corps, jeton, 401, 403, sans adresse), contrôles locaux ; contre PostgreSQL 18 (`LATTIS_PG_TEST=1`, voir `publication.md` §6, migrations 0001 à 0008) : `TestPostgresACL` (liste des droits, refus pour un participant, participant et lignes verrouillées, rôle personnalisé avec filtre de lecture : une seule ligne arrive dans le cache, restriction de lecture, restrictions vues par l'utilisateur, filtre dangereux refusé en `22023`, écarts après une règle ajoutée à la main, invitation et retrait, notification `*acl`).
- `internal/remote/collab/pg_test.go` (`TestPostgresRights`) : la ligne d'un autre est désormais refusée en local avant l'envoi.
- `internal/bridge/rights_test.go` : parcours par le pont (propriétaire : liste, invitation, rôle, niveau de feuille posé sur ses tableaux, filtre refusé sans `SQLSTATE`, restriction, « Voir en tant que » ; participant : niveau dans la mise en page, lignes verrouillées de la fenêtre, refus, pas de gestion des droits ; droits changés par `*acl` : plus de verrou).
- Frontend : `src/lib/remote/rights.test.ts` (filtres en formulaire : SQL de chaque opérateur, refus, relecture ; filtre libre ; héritage des niveaux, feuille mixte ; membres ; objets de l'onglet SQL).

## 6. Reste à faire

- Groupes, équipes, invitation directe à un rôle personnalisé : selon le serveur (§4).
- Structure d'un tableau publié par le concepteur (§6.4) et feuilles connues du serveur : étape 7.
- L'onglet SQL ne montre pas les règles générées pour un tableau privé (il n'y en a pas) ; la liste des objets se relit à l'ouverture de l'onglet et à chaque changement d'état de partage.
