# Audit de stabilité du mode collaboratif — 20 octobre 2026

*Périmètre : synchronisation des classeurs collaboratifs (écritures optimistes, réception, conflits, droits, structure, mise en page en direct, relais SSE) entre l'application (be-lattis-app, `main` à `44fc8b1`) et le serveur (be-lattis-server, `main` à `4883f9d`). Le mode hors ligne est audité à part. Aucun code n'a été modifié : les essais ont tourné sur une copie du dépôt.*

## Méthode

- Lecture de `docs/collaboration/synchronisation.md`, `droits.md`, du contrat client du serveur (§3.1, §6, §11) et du code : `internal/remote/collab` (envoi, réception, suivi, conflits, mise en page), `internal/remote/workbook` (pgx, `livelayout.go`), `internal/canvas/remote.go` ; côté serveur `schema/0004`, `0007`, `0008`, `0010`, `0003` (`save_meta`) et `internal/events/events.go`.
- **11 essais multi-postes** contre un vrai PostgreSQL 18 (conteneur `lattis-pgtest`, base jetable migrée avec le schéma du serveur), avec le harnais des essais existants (`pgSetupClients` : Alice propriétaire, Bob éditeur, Dave participant). Le fichier est joint : [`audit-collaboratif-2026-10-20-essais.go.txt`](audit-collaboratif-2026-10-20-essais.go.txt) (à copier en `internal/remote/collab/audit_pg_test.go`). Commande :

```bash
LATTIS_PG_TEST=1 LATTIS_TEST_PG_URL="postgres://postgres:<mot de passe>@127.0.0.1:55432/postgres?sslmode=disable" LATTIS_SERVER_SCHEMA=../be-lattis-server/schema go test ./internal/remote/collab/ -run TestAudit -count=1 -v
```

- Les essais `TestPostgres*` existants passent sur le même environnement (contrôle du harnais).

## Résumé

| # | Gravité | Côté | Problème | Essai |
|---|---|---|---|---|
| A1 | **Critique** | Appli | Une saisie en attente dans un tableau dont un collègue vient de changer la structure bloque le poste **hors ligne pour toujours** | `TestAuditStructureDuringInput` (3 cas), `…Fake` : échec |
| A2 | Élevée | Appli + serveur | Deux concepteurs qui enregistrent la mise en page au même moment : les cellules libres de l'un **disparaissent** sans conflit signalé | `TestAuditLayoutConcurrentSave` : échec |
| A3 | Élevée | Appli | « Garder la sienne » remet une valeur **périmée** si la collègue a encore modifié la cellule ; l'écart reste | `TestAuditStaleTheirs` : échec |
| A4 | Élevée | Appli + serveur | Droits de lecture changés en cours de session : les lignes devenues interdites **restent affichées**, celles devenues visibles **n'arrivent jamais** | `TestAuditReadFilterChanged`, `…Widened` : échec |
| S1 | Élevée | Serveur | Une ligne qui sort du filtre de lecture d'un collègue (sa valeur a changé) reste chez lui, avec l'ancienne valeur | `TestAuditRowLeavesFilter` : échec |
| A5 | Moyenne | Appli + serveur | Ligne ajoutée dont la réponse s'est perdue, supprimée entre-temps par un collègue : le renvoi la **recrée** | `TestAuditInsertResentAfterDelete` : échec |
| A6 | Moyenne | Appli | Décimal de plus de 17 chiffres écrit hors de l'appli : chaque modification part en **conflit sans fin** | `TestAuditDecimalPrecision` : échec |
| S2 | Moyenne | Serveur | Flux SSE : `ready` envoyé même quand les canaux ne sont pas écoutés ; des changements peuvent se perdre sans `reset` | lecture |
| S3 | Moyenne | Serveur | Relais SSE : réécoute complète à chaque abonnement, un aller-retour par notification, vérification des droits en rafale sur `*acl` | lecture |
| C1 | Moyenne | Appli + serveur | Montée en charge : une connexion permanente au parent par poste et par classeur ; le flux SSE n'est pas encore utilisé par l'appli ; une transaction longue gonfle tous les deltas | `TestAuditLoadFollowers` (mesure) |
| F1 | Faible | Appli | Une suppression l'emporte sans message sur une modification concurrente ; une écriture refusée suivie d'une autre sur la même cellule devient un conflit | lecture |
| F2 | Faible | Serveur | Le flux SSE reste ouvert après l'expiration du jeton ou la désactivation du compte | lecture |

Vérifié sans écart : suppression d'une ligne puis Ctrl+Z lus ensemble par un collègue (`TestAuditDeleteUndoSameDelta` : la ligne rétablie a un nouvel identifiant, rien ne se perd) ; 40 postes qui suivent le même classeur reçoivent une écriture en 314 ms au pire, regroupement de 200 ms compris.

---

## Application

### A1 — Critique : poste bloqué hors ligne après un changement de structure d'un collègue

**Scénario.** Bob saisit une valeur (elle attend dans sa file : envoi toutes les quelques centaines de millisecondes, ou pendant une coupure). Au même moment, Alice renomme le tableau, renomme ou supprime une colonne. Bob passe « hors ligne » et n'en sort plus : chaque nouvel essai rejoue la même erreur, ses saisies suivantes s'empilent dans la file et ne partent jamais, même après un redémarrage de l'application.

**Repro.** `TestAuditStructureDuringInput` : `bob.mustSet(1, 2, "8")`, Alice renomme le tableau (ou supprime « Prix », ou la renomme en « Tarif ») et envoie, puis deux `bob.e.Sync`. Résultat contre PostgreSQL :

```
retour au point de sauvegarde après « error preprocessing batch (prepare): ERROR: relation "wb_ventes_2026_….Ventes" does not exist (SQLSTATE 42P01) » : ERROR: savepoint "lattis_op" does not exist (SQLSTATE 3B001), file map[pending:1]
```

**Deux causes qui s'additionnent.**

1. `pgWriter.write` (`internal/remote/workbook/pg_sync.go:61`) envoie `SAVEPOINT`, l'écriture et `RELEASE` dans un même `pgx.Batch`. pgx prépare tout le lot avant de l'exécuter : une colonne ou une table inconnue échoue **à la préparation**, le `SAVEPOINT` n'a jamais tourné, le `ROLLBACK TO SAVEPOINT` échoue (`3B001`) et l'erreur rendue est un `fmt.Errorf("… %v …")` qui perd le `SQLSTATE` d'origine. `staleStructure` ne la reconnaît plus : c'est traité comme une coupure.
2. Même avec le bon code (faux serveur, `TestAuditStructureDuringInputFake`), `Engine.send` (`internal/remote/collab/send.go:120`) ne relit que sa propre correspondance et réessaie une fois, puis rend l'erreur. Le tour du suivi (`follow.go`, `cycle`) s'arrête alors avant `ReceiveStructure`, la seule étape qui mettrait la correspondance à jour : la boucle ne peut pas se dénouer.

**Piste.** Préparer l'écriture hors du lot (ou détecter `3B001` et rendre l'erreur de l'écriture), puis, sur `errStale` après le nouvel essai, recevoir la structure (`receiveStructure` pour ce tableau) avant de renvoyer ; une valeur d'une colonne supprimée est écartée comme pour une colonne non publiée, un tableau supprimé comme aujourd'hui.

### A2 — Élevée : mise en page de deux concepteurs enregistrée au même moment

**Scénario.** Alice et Carol (concepteurs) tapent chacune une note dans une cellule libre ; leurs envois (1 s après la dernière frappe) tombent en même temps. La note de l'une disparaît chez les deux, sans conflit compté.

**Repro.** `TestAuditLayoutConcurrentSave` force l'entrelacement (les deux transactions lisent la mise en page avant que l'une ne l'enregistre) : `alice (note de Carol) : cellule (12, 5) "" = ""`.

**Cause.** `workbook.SyncLayout` (`internal/remote/workbook/livelayout.go:764`) relit la mise en page par un `SELECT` simple (`Writer.Meta`, `pg_structure.go:153`), fusionne, puis `save_meta` fait un `INSERT … ON CONFLICT DO UPDATE` (`schema/0003_workbooks.sql:455`). En READ COMMITTED, la seconde transaction attend le verrou de ligne puis écrase la première avec une fusion faite sur l'ancienne version. Au tour suivant, la première voit sa propre modification « retirée par le serveur » et l'efface en local. La fenêtre est la durée de la transaction (un aller-retour plus la fusion d'un objet qui peut peser plusieurs centaines de Ko) : rare, mais probable sur une journée de travail à deux concepteurs.

**Piste.** Côté appli : `SELECT … FOR UPDATE` dans `Writer.Meta` quand il sert à écrire, ou `pg_advisory_xact_lock` du classeur en tête de transaction (comme `create_table`). Côté serveur, plus solide : `save_meta` avec la version attendue (`lattis_xid` lu), refus `40001` si elle a changé, et le client refait la fusion. Les onglets à choix (`PublishChoices`) suivent le même chemin.

### A3 — Élevée : « Garder la sienne » remet une valeur périmée

**Scénario.** Conflit sur une cellule (Alice 11, Bob 12). Alice la change encore (15) avant que Bob ne tranche. Bob choisit « Garder la sienne » : sa cellule affiche 11, le serveur a 15, et rien ne corrige l'écart tant que personne ne retouche la ligne.

**Repro.** `TestAuditStaleTheirs` : `Bob après « Garder la sienne » : ["4.5" "11.0" "3.25"], attendu ["4.5" "15.0" "3.25"]`.

**Cause.** Une cellule en conflit est dans `keep` (`collab.go`, `Engine.keep`) : la réception ne la touche pas mais avance quand même le point de reprise, et `_lattis_conflicts.server` n'est jamais mis à jour. `Resolve(…, false)` (`receive.go:468`) écrit la valeur notée au moment du conflit. Le volet Conflits montre aussi l'ancienne valeur et l'ancien auteur.

**Piste.** À la réception, quand une cellule gardée est en conflit, mettre à jour `server`, `author` et `at` du conflit ; ou relire la ligne sur le serveur (`RowsByID`) dans `Resolve`.

### A4 — Élevée : droits de lecture changés en cours de session

**Scénario.** La propriétaire pose une restriction de lecture (« sans les lignes Thé ») : Bob, déjà ouvert, garde la ligne à l'écran et dans son cache. Elle la retire, ou élargit un filtre : un membre qui avait ouvert le classeur filtré ne reçoit jamais les lignes devenues visibles.

**Repro.** `TestAuditReadFilterChanged` : `Bob avec la restriction : ["Café" "Thé" "Chocolat"] (attendu sans Thé)`. `TestAuditReadFilterWidened` : `Carol : ["Café" "Chocolat"] … après son retrait (attendu les 3)`. Les deux essais appellent pourtant `RefreshRights` puis `Sync` (plus que ce que fait le suivi).

**Cause.** `RefreshCapabilities` (`workbook/capabilities.go:120`) relit les niveaux sans relire les données ; `changes_since` ne rend que les lignes visibles **modifiées depuis** le point de reprise : une ligne ancienne qui devient visible n'a pas de `lattis_xid` récent, une ligne qui devient invisible n'a pas de pierre tombale. Sur `*acl` seul, le tour du suivi ne relit d'ailleurs aucun tableau (`follow.go`, `cycle` : ni `full`, ni `meta`, ni `due`). C'est aussi une question de confidentialité : la donnée retirée reste sur le disque du poste.

**Piste.** Quand les capacités d'un tableau changent (niveau, `read_filtered`, restrictions, rôles), le relire en entier (`RemoteTable.Full`, cellules en attente gardées). Le serveur peut aider en notifiant `*acl:<table_id>`.

### A5 — Moyenne : ligne recréée par un renvoi

**Scénario.** Alice ajoute une ligne ; le serveur la valide mais la réponse se perd (coupure). Bob reçoit la ligne et la supprime. À la reconnexion, Alice renvoie l'ajout : la ligne revient chez tout le monde.

**Repro.** `TestAuditInsertResentAfterDelete` (serveur qui valide puis rend une coupure une fois) : `Bob après le renvoi d'Alice : ["Café" "Thé" "Chocolat" "Tisane"]`.

**Cause.** `pgWriter.Insert` fait `ON CONFLICT (lattis_id) DO NOTHING` (`pg_sync.go:144`) et `sendBatch` compte l'ajout comme fait (`wr.Rows = 1`, `send.go:294`) ; après la suppression, il n'y a plus de conflit d'identifiant, l'ajout réussit.

**Piste.** Serveur : refuser un `INSERT` dont le `lattis_id` a une pierre tombale (déclencheur `BEFORE INSERT`), `why_not` répondant « deleted » ; l'appli traite déjà ce cas (« Ligne supprimée entre-temps… »).

### A6 — Moyenne : décimaux longs en conflit sans fin

**Scénario.** Un décimal de plus de 17 chiffres significatifs arrive par un formulaire, la version web, un pipeline ou du SQL. Toute modification de la cellule depuis l'appli part en conflit (« Modifié par … : 0,12345678901234568 (vous : 7) »), et « Garder la mienne » reboucle sur le même conflit.

**Repro.** `TestAuditDecimalPrecision` (`0,1` puis `0,2` passent ; la valeur `0.12345678901234567891` posée en SQL donne `Conflicts:1`, puis encore `Conflicts:1` après « Garder la mienne »).

**Cause.** L'appli lit les décimaux en `float8` (`selectExpr`, `pg.go`) mais conditionne l'écriture par `col IS NOT DISTINCT FROM $3::numeric` (`pg_sync.go:116`) : la valeur arrondie ne vaut jamais la valeur exacte.

**Piste.** Pour une colonne `decimal`, conditionner par `col::float8 IS NOT DISTINCT FROM $3::float8`.

### C1 — Moyenne : montée en charge

- **Connexions.** Chaque poste garde une connexion permanente au parent par classeur collaboratif ouvert (`LISTEN`, `Engine.Follow`), et lit ses deltas sur `rw`. Mesure (`TestAuditLoadFollowers`) : 40 postes, 41 connexions. Avec `LATTIS_PG_MAX_CONNECTIONS` à 400 (Compose), la limite pratique est de quelques centaines de postes ouverts, moins les connexions de lattisd. Le flux `/api/events` (jalon 8) n'est référencé nulle part dans l'appli (`grep api/events` : aucun résultat dans `internal`). C'est la bascule à prévoir.
- **Transaction longue.** `sync_point()` est le plus petit `xid` en cours **sur toute l'instance**. Une sauvegarde `pg_dump`, une requête d'analyse ou une session « idle in transaction » retient le point de reprise de tous les postes : chaque notification relit toutes les lignes modifiées depuis le début de cette transaction, et `sync_skip` grossit jusqu'à 2 000 entrées. Aucun `idle_in_transaction_session_timeout` n'est posé (`deploy/postgres/lattis-conf.sh`).
- Latence mesurée : 314 ms au pire pour 40 postes (regroupement de 200 ms compris).

### F1 — Faible

- `Delete` n'est pas conditionné : une suppression faite après la modification d'un collègue, pas encore reçue, l'emporte sans message (dernière écriture gagnante, voulu ?).
- Une écriture refusée par les droits (`rejected`, ligne relue) suivie dans la file d'une autre écriture sur la même cellule : la seconde est gardée en local puis part en conflit au lieu d'être refusée.

---

## Serveur

### S1 — Élevée : une ligne qui sort du filtre de lecture reste chez le lecteur

**Scénario.** Bob ne voit pas les lignes « Interdit » (restriction de lecture). Alice renomme « Thé » en « Interdit ». Chez Bob, la ligne reste, avec l'ancienne valeur « Thé ». Même chose pour un filtre par équipe ou par région quand la valeur de la colonne filtrée change.

**Repro.** `TestAuditRowLeavesFilter` : `Bob : ["Café" "Thé" "Chocolat"]`.

**Cause.** `changes_since` (`schema/0008_sync.sql:72`) est `SECURITY INVOKER` : la RLS cache la ligne modifiée, et aucune pierre tombale n'est écrite puisque la ligne n'est pas supprimée.

**Piste.** Dans `changes_since`, rendre aussi en `deleted = true` les identifiants des lignes modifiées depuis `since` que l'appelant ne voit pas (sous-requête `SECURITY DEFINER` qui ne rend que des `lattis_id`). Cela règle aussi la moitié « restriction posée » de A4.

### S2 — Moyenne : `ready` sans écoute

`Hub.Handler` (`internal/events/events.go`, boucle `for i := 0; i < 100 && !h.covered(s); i++`) attend 5 s que les canaux soient écoutés, puis envoie `ready` **même s'ils ne le sont pas**. Cas concret : PostgreSQL refuse une nouvelle connexion (limite atteinte), `listen` échoue, l'ancienne écoute reste en place sans couvrir le nouveau canal ; elle n'est pas « cassée », donc aucun `reset` ne part. Le poste croit tout recevoir et manque les changements jusqu'à sa prochaine relecture complète. **Piste :** ne pas envoyer `ready` sans couverture (répondre `503`), ou envoyer `reset` à ce poste quand la couverture arrive.

### S3 — Moyenne : coût du relais SSE à l'échelle

- Chaque changement de l'ensemble des canaux (un poste qui ouvre un nouveau classeur, un canal qui expire après 2 min) ouvre une nouvelle connexion et refait un `LISTEN` par canal, un aller-retour chacun (`listen`). Avec des milliers de classeurs suivis, chaque abonnement coûte des secondes et les doublons de la période de recouvrement se multiplient. **Piste :** `LISTEN`/`UNLISTEN` incrémentaux sur la même connexion.
- `dispatch` fait un `SELECT pg_current_wal_lsn()` par notification sur la connexion d'écoute : le débit est borné par la latence. **Piste :** lire la position une fois par rafale.
- Sur `*acl`, `recheck` lance une goroutine **par abonné** qui ouvre une transaction sous l'identité de l'utilisateur (`my_capabilities`) : 500 postes sur un classeur, une modification de membres donne 500 transactions simultanées. **Piste :** regrouper par classeur et ne revérifier que les utilisateurs concernés.

### S4 — Moyenne : concurrence de `save_meta` et ajout après suppression

Côté serveur de A2 (pas de contrôle de version dans `save_meta`) et de A5 (`INSERT` accepté sur un identifiant qui a une pierre tombale) : voir ces points.

### F2 — Faible : flux SSE après expiration du jeton

Le jeton n'est vérifié qu'à l'ouverture du flux ; un compte désactivé continue de recevoir des identifiants de tableaux tant qu'aucun `*acl` ne touche le classeur. Les charges ne portent pas de données. **Piste :** fermer le flux à l'expiration du jeton (`exp`).

---

## Ordre de traitement suggéré

1. A1 (bloquant au premier renommage de colonne pendant que quelqu'un saisit).
2. S1 et A4 ensemble (même cause : visibilité changée sans signal), puis A3.
3. A2 (perte de cellules libres entre concepteurs).
4. A5, A6, puis la bascule de l'appli sur `/api/events` (C1) avec S2 et S3 réglés avant.
