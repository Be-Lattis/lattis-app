# Synchronisation des classeurs collaboratifs

*Étape 5 du découpage de la spécification « Lattis collaboratif : modifications de l'application » (§15) : §6.2 (envoi), §6.3 (réception), §7 (conflits et annulation), §9 (types figés), §10 (hors ligne). S'appuie sur le journal logique (`journal-logique.md` : file d'envoi, `command.Op`), la publication (`publication.md` : `workbook.Server`, `Link`, cache) et la connexion (`connexion.md`). Contrat suivi : `lattisd`, `docs/contrat-client.md` §3.1, §5 et §6, `schema/0004_collaborative.sql` et `0007_publication.sql` (branches `claude/socle-lattisd` et `claude/jalon-6-replicas` de be-lattis-server).*

## 1. Ce qui est fait

| Élément | Où |
|---|---|
| API de synchronisation derrière `workbook.Server` : `Write` (transaction d'écriture sur `rw` : `Update` conditionné, `Insert`, `Delete`, `Move`, `WhyNot`), `Delta` (lecture `REPEATABLE READ` sur `rw` : `SyncPoint`, `Changes`, `RowsByID`) ; implémentation pgx ; faux serveur avec les mêmes règles (droits par niveau, RLS du participant, pierres tombales, `xid`, notifications) | `internal/remote/workbook/server.go`, `pg_sync.go`, `workbooktest/sync.go` |
| **Envoi** : file vidée dans l'ordre, une transaction PostgreSQL par étape (`batch`), écritures conditionnées, `lattis.why_not` sur 0 ligne, états `sent` / `conflict` / `rejected`, purge des entrées dont l'annulation n'a plus besoin | `internal/remote/collab/send.go` |
| **Réception** : `lattis.changes_since` depuis le point de reprise de chaque tableau, appliqué hors du journal d'annulation par une entrée dédiée de `canvas` (`Model.ApplyRemote` : `t_<id>`, `_pos`, `_lattis_row_ids` avec `lattis_author`, saisies invalides et mises en forme des lignes supprimées, tableau agrandi qui pousse ce qui est dessous), nouveau `since` dans la même transaction locale | `internal/remote/collab/receive.go`, `internal/canvas/remote.go` |
| Suivi du serveur : `LISTEN` sur le canal du classeur ; à chaque (re)connexion, envoi de la file puis lecture de tous les tableaux ; sur notification (identifiant de tableau), lecture de ce tableau, notifications regroupées sur 200 ms (§16, point 2) ; envoi réveillé après chaque modification locale ; coupure : hors ligne, nouvel essai toutes les 30 s | `collab.Engine.Follow`, `follow.go`, `bridge.followCollab` |
| Recalcul et grille : par les mécanismes existants (`Store.TakeTouched`, évènement `doc:changed`) ; l'état de partage (`share:changed`) porte les étapes en attente et le nombre de conflits | `bridge/sharing.go`, `workbook.State.Pending`, `Conflicts` |
| **Affichage optimiste** : la saisie est visible tout de suite ; petit triangle gris bleu dans le coin bas droit tant que le serveur ne l'a pas confirmée, orange en cas de conflit | `collab.Engine.Marks`, `grid.Window.Sync`, `Sheet.svelte` (`syncCorner`), `cache.ts` |
| **Conflits** : `_lattis_conflicts` ; volet de droite (largeur commune `resizePanel`) « Modifié par Alice à 10:42 : 1 100 (vous : 1 200) », Aller à la cellule, **Garder la mienne** (nouvelle entrée de la file, conditionnée à la valeur du serveur) / **Garder la sienne** (valeur du serveur, hors journal) ; bandeau « N conflits… Voir » ; le volet s'ouvre de lui-même quand un conflit apparaît | `collab.Engine.Conflicts`, `Resolve`, `ConflictPanel.svelte`, `share.ts` |
| Cellule en cours de saisie : une modification reçue pour elle attend la validation (puis conflit si les valeurs diffèrent) | `DocumentService.ShareEditing`, `SheetSource.editing` |
| **Annulation** de ses seules actions : une cellule modifiée par un collègue depuis (modification reçue) n'est ni annulée ni rétablie, elle garde la valeur du collègue, le reste de l'étape l'est ; message en barre d'état ; si la modification du collègue n'est pas encore reçue, le serveur refuse (`why_not` « modified ») et la ligne reprend la valeur du serveur | `oplog.writer.keep`, `Recorder.TakeSkipped`, `collab.UndoNotice`, `outcome.refused` |
| Refus de droits : ligne refusée par la RLS (`why_not` « forbidden ») ou tableau refusé (`42501`, toute l'étape) : modification défaite en local (lignes relues sur le serveur), message « Droits insuffisants… » | `send.go` |
| **Types figés** (§9) : dans un classeur partagé, une saisie hors du type d'une colonne d'un tableau publié est refusée (message clair) au lieu d'être gardée en erreur ; un collage ou une recopie écarte et liste les valeurs refusées, le reste passe | `canvas.ErrOutOfType`, `Event.Refused`, `refusedNotice` |
| Structure d'un tableau publié d'un classeur collaboratif (colonne ajoutée, retirée, type, suppression) refusée en local tant que le §6.4 n'est pas fait | `workbook.ErrStructure` (`Link.Gate`) |
| **Hors ligne** (§10) : bandeau « Hors ligne, lecture seule » (avec le nombre de modifications en attente), aucune écriture sur les tableaux publiés ; la file est gardée et part à la reconnexion avec la détection de conflit normale ; le cache d'un classeur collaboratif se rouvre tel quel (file comprise) et se met à jour par ses deltas | `Link.lock`, `keepCollabCache`, `share.ts` |
| Réplicas (contrat §3.1) : une lecture sur `ro` refusée à tort (`42501`) est refaite sur une nouvelle connexion (la fautive est fermée, le pool ne la redonne pas), puis sur `rw` | `workbook.pgServer.with` |

## 2. Choix de conception

### 2.1 Une étape, une transaction ; un refus n'est pas une coupure

Les entrées d'un même `batch` partent dans une transaction (`Server.Write`). Une écriture sans effet (0 ligne) n'est pas une erreur : `lattis.why_not` l'explique dans la même transaction, et le reste de l'étape s'applique (deux colonnes d'une même ligne ne sont jamais en conflit, une cellule en conflit n'empêche pas les autres). Une **erreur SQL** (`SQLSTATE` présent : droits sur tout le tableau, contrainte) défait la transaction du serveur : toute l'étape est refusée (`rejected`) et ses lignes reprennent l'état du serveur. Toute **autre erreur** est une coupure : l'étape reste `pending`, le classeur passe hors ligne, tout repartira à la reconnexion. Les valeurs sont converties au type du serveur avant l'envoi : une valeur inconvertible est refusée sans aller-retour.

Idempotence (renvoi après une coupure entre la validation du serveur et la mise à jour locale) : `INSERT … ON CONFLICT (lattis_id) DO NOTHING`, `DELETE` sans effet sur une ligne déjà supprimée, et un `UPDATE` refusé dont la valeur actuelle est déjà la nôtre est compté comme envoyé.

### 2.2 Ce que la réception ne touche pas

`collab.Engine.keep`, relu dans la transaction de réception : les cellules des entrées `pending` (saisies pas encore confirmées), les cellules en conflit, la cellule en cours de saisie, et les lignes supprimées en local dont la suppression n'est pas encore partie (elles ne reviennent pas). Tout le reste prend la valeur du serveur : le serveur fait foi.

### 2.3 Annulation compensatoire

Le Recorder (`oplog`) conditionne déjà l'annulation à « ce que j'avais écrit ». S'y ajoute la règle du §7 : si la valeur locale avant l'annulation n'est plus celle-là (un collègue l'a modifiée, reçue entre-temps), la cellule est remise à la valeur du collègue dans la transaction de l'annulation et rien ne part pour elle. Le rétablissement suit la même règle (condition : la valeur d'avant la commande). Le compte des cellules gardées donne le message de la barre d'état.

### 2.4 Conflits

`_lattis_conflicts (outbox, tbl, row_id, col, local, server, author, at)` : valeurs en JSON. Un nouveau conflit sur la même cellule remplace l'ancien. « Garder la mienne » crée une entrée `do` sans commande (`seq` NULL), conditionnée à la valeur du serveur ; « Garder la sienne » écrit la valeur du serveur hors journal. Une entrée en conflit ou refusée reste dans la file tant que sa commande est dans le journal (l'annulation y relit ce qu'elle avait écrit).

### 2.5 Points de reprise

`since` par tableau (`_lattis_remote_tables.since`) : `lattis.sync_point()` lu au début de la lecture `REPEATABLE READ`, noté dans la même transaction locale que les données. Un tableau publié depuis ce poste n'a pas encore de `since` : sa première lecture part de 0 (idempotente). Les deltas restent sur `rw` (lecture de ses propres écritures, contrat §3.1), même si le serveur les permet désormais sur un réplica.

## 3. Essais

- `internal/remote/collab/collab_test.go` (faux serveur, deux postes) : même ligne et colonnes différentes, même cellule (conflit, marque, réception pendant le conflit, Garder la mienne, Garder la sienne), annulation après la modification d'un collègue (reçue, ou refusée par le serveur), refus de droits (lecteur : `42501` ; participant : ligne d'un autre refusée, sa ligne acceptée, auteur recopié), file hors ligne (gardée, marques, envoyée à la reconnexion avec conflit), lignes ajoutées, supprimées, triées, réception idempotente, types figés (saisie refusée, collage partiel, structure refusée), cellule en cours de saisie, suivi par notifications et passage hors ligne.
- `internal/remote/oplog/oplog_test.go`, `internal/integration/oplog_test.go` : annulation qui garde la cellule d'un collègue.
- `internal/bridge/collab_test.go` : parcours par le pont (ouverture, en ligne, hors ligne, saisie envoyée, réception sur notification, marques de la fenêtre, conflit signalé dans l'état, résolution).
- Contre PostgreSQL 18 (`LATTIS_PG_TEST=1`, migrations 0001 à 0007 du serveur, voir `publication.md` §6) : `TestPostgresSync` (colonnes différentes, conflit et why_not, Garder la mienne, lignes, annulation refusée, file gardée pendant une coupure), `TestPostgresRights` (participant sous RLS, droit retiré sur le tableau), `TestPostgresFollow` (`LISTEN` réel), `TestPostgresReplicaRetry` (lecture refusée à tort refaite). Harnais commun : `workbooktest.PGSetup`.
- Frontend : `share.test.ts` (bandeau des conflits et de la file, texte d'un conflit, valeurs refusées, marques).

## 4. Écarts avec le serveur et reste à faire

| Point | Constat | Proposition |
|---|---|---|
| Échos | `changes_since` rend aussi ses propres écritures (aucun filtre par auteur ou par transaction) : chaque envoi revient au poste qui l'a fait (sans effet, mais relu). | Paramètre facultatif pour écarter les lignes dont `lattis_updated_by = session_user` et `lattis_xid` connu du client, ou rendre l'`xid` de la transaction d'écriture. |
| Refus d'une partie de l'étape | Un `INSERT` refusé par une règle `WITH CHECK` (filtre d'écriture) lève une erreur qui défait toute la transaction ; le client refuse alors toute l'étape. | Points de sauvegarde par opération côté client (coût), ou une fonction serveur d'écriture par lot qui rend un statut par opération. |
| `why_not` et l'ordre | Un `move` refusé (clé d'ordre changée) est rendu comme « modified » sans comparer `lattis_pos` : le client relit la ligne et prend l'ordre du serveur. | — |
| Clés d'ordre égales | Deux postes qui insèrent au même endroit produisent la même clé ; le serveur départage par `lattis_id`, le classeur local (`_pos` seul) non. | Départage local par `lattis_id` à l'affichage (§16, point 4). |
| ~~Droits qui changent~~ | Réglé : notification `*acl` (migration 0008 du serveur), capacités relues à la connexion et sur notification, grille grisée avant tout refus (voir `droits.md`). | — |
| Pierres tombales | Purgées après 30 jours : un cache sans rien en attente et chargé depuis plus de 25 jours est rechargé en entier à l'ouverture ; un cache plus vieux avec une file en attente est gardé (la file part, les suppressions anciennes peuvent manquer). | Une fonction qui dit si `since` est encore couvert par les pierres tombales. |
| Structure et mise en page | `*meta` ignoré ; structure d'un tableau publié refusée en local. | Étape 7 (§6.4). |
| `datetime` | Lu en texte ISO, jamais écrit depuis l'application (pas de type local). | — |

Côté application, restent : départage des clés d'ordre égales, édition hors ligne (la file est prête).
