# Audit du mode en ligne, 20 octobre 2026 (rapport fusionné)

*Fusion de trois audits du même jour, sans code modifié :*

- *stabilité du mode hors ligne (`2026-10-20-audit-hors-ligne.md`, branche `claude/project-thread-91uq6z`, 11 constats, notés **HL1** à **HL11**) ;*
- *stabilité du mode collaboratif contre un vrai PostgreSQL 18 (`audit-collaboratif-2026-10-20.md`, sur `main`, constats **A1** à **A6**, **S1** à **S4**, **C1**, **F1**, **F2**) ;*
- *différences d'affichage et de comportement entre mode en ligne, mode classique et édition web (`audit-modes-en-ligne-classique-2026-10-20.md`, constats **E1** à **E11**).*

*Périmètre commun : un classeur d'un Serveur Lattis (collaboratif ou figé, rôles propriétaire, concepteur, éditeur, participant, lecteur, hors ligne avec file d'attente), comparé au fichier `.lattis` local (y compris le travail à plusieurs sur un dossier partagé) et à l'édition web (`lattis-engine` sous `/app/`, téléphone compris). Code lu : be-lattis-app `main` jusqu'à 4007d3f, be-lattis-server `main` jusqu'à 4883f9d. Les trois rapports d'origine restent la référence pour le détail ; ce document les dédoublonne, les classe et rassemble les décisions à prendre.*

## 1. Doublons rapprochés

| Constat fusionné | Vient de | Pourquoi un seul |
|---|---|---|
| **1** Poste bloqué hors ligne après un changement de structure d'un collègue | HL1 + A1 | Même boucle (`send.go:120`, `follow.go` `cycle`) ; A1 ajoute une seconde cause (lot pgx préparé avant le `SAVEPOINT`, `3B001`) qui empêche même de reconnaître l'erreur |
| **4** Fichier de l'auteur d'un classeur partagé | HL4 + E1 | Même objet : le fichier d'auteur ne se présente pas comme en ligne (E1) et ses modifications hors ligne y restent coincées (HL4) |
| **14** Ligne recréée par un renvoi après une coupure au `COMMIT` | HL8 + A5 + S4 (moitié) | Même cause : `INSERT … ON CONFLICT (lattis_id) DO NOTHING` (`pg_sync.go:144`) accepte un identifiant qui a une pierre tombale |
| **9** Droits de lecture changés en cours de session | A4 + S1 | Même cause de fond : la visibilité change sans signal ni pierre tombale ; S1 règle la moitié « restriction posée » de A4 |
| **7** Mise en page de deux concepteurs enregistrée au même moment | A2 + S4 (moitié) | Côté client et côté serveur du même défaut (`save_meta` sans contrôle de version) |
| **23** Mise en page : la dernière écriture l'emporte sans que le collègue le sache | HL9 + E6 (conflits de mise en page) | Même règle de la décision 0018 ; HL9 en montre l'effet après une longue coupure, E6 que le compteur `LayoutConflicts` n'est jamais affiché |

## 2. Synthèse classée

Gravité : **P0** perte de données ou poste bloqué dans un cas courant ; **P1** perte ou écart probable, ou geste quotidien trompeur ; **P2** cas plus rare, ou incohérence sans perte ; **P3** finition.

| # | Prio | Constat | Origine | Preuve |
|---|---|---|---|---|
| 1 | **P0** | Saisie en attente + structure changée par un collègue : poste hors ligne pour toujours | HL1, A1 | `TestAuditStructureDuringInput` (3 cas, PostgreSQL), `…Fake`, `TestAuditStaleColumnLivelock`, `TestAuditStaleTableLivelock` |
| 2 | **P0** | Après un plantage hors ligne, rouvrir depuis le serveur écrase le cache et sa file | HL2 | `TestAuditShareOpenAfterCrash` |
| 3 | **P1** | Erreur passagère du serveur (interblocage, délai) prise pour un refus : saisie défaite | HL3 | `TestAuditTransientErrorIsRejection` |
| 4 | **P1** | Fichier d'auteur : pas présenté comme en ligne, modifications hors ligne coincées | HL4, E1 | lu dans le code |
| 5 | **P1** | Dossier partagé (NAS) : poste coupé plus de 30 s qui rate pour toujours ce qui a été compacté | HL5 | `TestAuditCompactedWhileAway` |
| 6 | **P1** | File effacée si l'enregistrement du cache échoue à la fermeture | HL6 | lu dans le code |
| 7 | **P1** | Deux concepteurs enregistrent la mise en page en même temps : cellules libres perdues | A2, S4 | `TestAuditLayoutConcurrentSave` |
| 8 | **P1** | « Garder la sienne » remet une valeur périmée | A3 | `TestAuditStaleTheirs` |
| 9 | **P1** | Droits de lecture changés : lignes interdites qui restent, lignes autorisées qui n'arrivent pas (confidentialité) | A4, S1 | `TestAuditReadFilterChanged`, `…Widened`, `TestAuditRowLeavesFilter` |
| 10 | **P1** | En ligne : point « modifié », « Enregistrer les modifications ? », « Quitter le site ? » alors que tout est sur le serveur | E3 | lu dans le code |
| 11 | **P1** | Le rôle de l'utilisateur n'est affiché nulle part | E2 | lu dans le code |
| 12 | **P1** | Édition web : perte du lien avec le moteur et session expirée invisibles | E9 | lu dans le code |
| 13 | P2 | Ouvrir un autre classeur perd la mise en page en attente | HL7 | lu dans le code |
| 14 | P2 | Ligne recréée par un renvoi après une coupure au `COMMIT` | HL8, A5, S4 | `TestAuditAmbiguousCommit`, `TestAuditInsertResentAfterDelete` |
| 15 | P2 | Décimal de plus de 17 chiffres : conflit sans fin | A6 | `TestAuditDecimalPrecision` |
| 16 | P2 | Serveur, flux SSE : `ready` envoyé sans écoute | S2 | lu dans le code |
| 17 | P2 | Serveur, relais SSE : coût à l'échelle | S3 | lu dans le code |
| 18 | P2 | Montée en charge : une connexion `LISTEN` par poste et par classeur, transaction longue | C1 | `TestAuditLoadFollowers` (mesure) |
| 19 | P2 | Clic droit, menu ▾, Documents et Importer proposent ce que le moteur refuse | E4 | lu dans le code |
| 20 | P2 | Scripts d'un classeur collaboratif ni partagés ni protégés | E5 | lu dans le code |
| 21 | P2 | Tableau chargé par une requête : plus actualisable une fois publié | E7 | lu dans le code |
| 22 | P2 | Édition web, classeur local : Ctrl+S ne télécharge que la première fois | E8 | lu dans le code |
| 23 | P3 | Mise en page : la dernière écriture l'emporte sans que le collègue le sache | HL9, E6 | lu dans le code |
| 24 | P3 | Droits de conception retirés pendant la coupure : mise en page jamais envoyée, sans message | HL10 | lu dans le code |
| 25 | P3 | Pas de délai sur l'envoi des données | HL11 | lu dans le code |
| 26 | P3 | Suppression qui l'emporte sans message ; écriture refusée suivie d'une autre devenue conflit | F1 | lu dans le code |
| 27 | P3 | Flux SSE ouvert après l'expiration du jeton | F2 | lu dans le code |
| 28 | P3 | Édition web : menu Compte du bureau | E10 | lu dans le code |
| 29 | P3 | Pas de présence des membres sur un classeur du serveur | E6 | lu dans le code |
| 30 | P3 | Détails : copie d'un figé, réglages d'un lecteur, thème | E11 | lu dans le code |

Vérifié sans écart : la file est écrite dans la même transaction que la modification ; le renvoi d'une étape déjà validée ne crée pas de doublon ; la longue coupure avec pierres tombales purgées relit le tableau après l'envoi ; le classeur supprimé du serveur garde sa file et le dit ; suppression puis Ctrl+Z lus ensemble par un collègue (`TestAuditDeleteUndoSameDelta`) ; 40 postes reçoivent une écriture en 314 ms au pire. Les droits sont appliqués par le moteur sur tous les chemins (grille, SQL, scripts, API, MCP).

---

## 3. Constats

### 1. P0 : poste bloqué hors ligne après un changement de structure d'un collègue (HL1, A1)

**Scénario.** Bob a une saisie en attente (envoi différé de quelques centaines de millisecondes, ou coupure). Alice renomme ou supprime la colonne, ou le tableau. Bob passe « hors ligne » et n'en sort plus : chaque essai rejoue la même erreur, ses saisies suivantes s'empilent, il ne reçoit plus rien des autres, même après un redémarrage.

**Deux causes qui s'additionnent.**
1. `pgWriter.write` (`internal/remote/workbook/pg_sync.go:61`) met `SAVEPOINT`, l'écriture et `RELEASE` dans un même `pgx.Batch` : une colonne ou une table inconnue échoue à la préparation, le `ROLLBACK TO SAVEPOINT` échoue (`3B001`) et l'erreur rendue perd le `SQLSTATE` d'origine ; `staleStructure` ne la reconnaît plus et la traite comme une coupure.
2. Même avec le bon code, `Engine.send` (`internal/remote/collab/send.go:120`) réessaie une fois avec la correspondance **locale**, puis le tour du suivi (`follow.go:205-247`, `cycle`) s'arrête avant `ReceiveStructure`, la seule étape qui apprendrait le changement.

**Preuves.** Contre PostgreSQL : `TestAuditStructureDuringInput` (tableau renommé, « Prix » supprimée, « Prix » renommée en « Tarif ») ; faux serveur : `TestAuditStructureDuringInputFake`, `TestAuditStaleColumnLivelock`, `TestAuditStaleTableLivelock`.

**Piste.** Préparer l'écriture hors du lot (ou rendre l'erreur de l'écriture sur `3B001`) ; sur `errStale`, recevoir la structure du tableau concerné avant de renvoyer ; valeurs d'une colonne renommée envoyées sous le nouveau nom, d'une colonne ou d'un tableau supprimés écartées avec un message. Garde-fou général : une étape qui échoue toujours est mise de côté après N échecs (visible dans le volet Conflits) et ne bloque plus la réception (question 6).

### 2. P0 : après un plantage, rouvrir depuis le serveur écrase le cache et sa file (HL2)

**Où.** `internal/bridge/sharing.go:741-755` (`ShareOpen`) : toute erreur d'ouverture du cache, dont `RecoverableError` (session interrompue), donne un classeur neuf rechargé du serveur puis enregistré **sur** le cache.

**Scénario.** Modifications hors ligne, la machine s'éteint ; au redémarrage le serveur répond, Louis rouvre depuis les récents ou un lien `lattis://` : pas de proposition de récupération, la file du cache est perdue, et la session plantée n'est plus que dans un répertoire orphelin que l'on croira inutile.

**Preuve.** `TestAuditShareOpenAfterCrash`. **Piste.** Ne créer un classeur neuf que si le cache n'existe pas ; rendre `OpenResult{Recoverable}` comme `open` ; sur une autre erreur, ne pas écraser le cache (le renommer ou demander).

### 3. P1 : une erreur passagère du serveur défait la saisie (HL3)

**Où.** `send.go:308-313`, `:340` ; `internal/remote/workbook/server.go:311` (`Rejected` : toute erreur avec un `SQLSTATE`) ; `structure.go:181`.

Interblocage (`40P01`), `statement_timeout` (`57014`), `lock_timeout` (`55P03`), annulation administrative : pris pour un refus, la cellule reprend la valeur du serveur avec « Modification refusée par le serveur, annulée » ; pour la structure, tout le changement et les suivants du même tableau sont défaits. **Preuve.** `TestAuditTransientErrorIsRejection`. **Piste.** Classes `40`, `53`, `08`, `57P0x`, `55P03`, `57014`, `25006` traitées comme une coupure ; un refus, c'est `42501`, `23xxx`, `22xxx`, `P0001` et les codes du schéma `lattis`.

### 4. P1 : fichier de l'auteur d'un classeur partagé (HL4, E1)

**Ce que l'auteur voit (E1).** Juste après la publication, Alice continue sur son propre `Ventes.lattis`. Ce n'est pas un cache : `remoteDoc` (`internal/bridge/remotedoc.go:39`) le prend pour un fichier local, donc ni badge « en ligne sur X » (`+page.svelte:2151`), ni message de Ctrl+S sur le serveur (« Enregistré : D:\…\Ventes.lattis », `doctitle.ts:51`), et la question « Enregistrer ? » à la fermeture, alors que ses saisies partent au serveur.

**Ce qui se perd (HL4).** Hors ligne, Ctrl+S met la file dans ce fichier (décision 0014). Depuis c2f880e, rouvrir ce fichier ouvre le classeur **depuis le serveur** (`document.go:217`, `:232`, `sharedFile`) : elle voit la version du serveur, ses modifications restent dans le fichier et ne partent jamais.

**Piste.** Soit le fichier d'auteur reste le classeur vivant : badge et messages tirés de l'état de partage, et `sharedFile` l'ouvre tel quel s'il a une file ou des conflits ; soit on bascule l'auteur sur le cache juste après la publication, le fichier devenant une copie d'archive, et on le lui dit (question 2).

### 5. P1 : dossier partagé (NAS), poste coupé qui rate ce qui a été compacté (HL5)

**Où.** `internal/coedit/apply.go:505-532` (`compact` ne compte que les sessions vivantes) ; `apply.go:119`, `:141` (journal disparu réputé « dans le fichier »).

Bob perd le NAS plus de 30 s ; Alice enregistre, le segment est compacté ; Bob revient et garde l'ancienne valeur sans message ; « Écraser » à son enregistrement efface la saisie d'Alice. **Preuve.** `TestAuditCompactedWhileAway` (le sens retour fonctionne : `TestAuditReturnAfterCompaction`). **Piste** : question 5.

### 6. P1 : file effacée si l'enregistrement du cache échoue à la fermeture (HL6)

**Où.** `remotedoc.go:56-68` (`keepCollabQueue` : échec seulement journalisé), `document.go:185-186` (`old.Close()` supprime ensuite le répertoire de travail). Disque plein, antivirus, « modifié par ailleurs » : la file disparaît sans message. **Piste.** Sur échec, `Release` au lieu de `Close` (récupération au prochain lancement) et le dire.

### 7. P1 : mise en page de deux concepteurs enregistrée en même temps (A2, S4)

**Où.** `workbook.SyncLayout` (`livelayout.go:764`) relit par un `SELECT` simple (`Writer.Meta`, `pg_structure.go:153`), fusionne, puis `save_meta` (`schema/0003_workbooks.sql:455`) fait `INSERT … ON CONFLICT DO UPDATE` : en READ COMMITTED, la seconde écrase la première ; au tour suivant, la première voit sa modification « retirée par le serveur » et l'efface. **Preuve.** `TestAuditLayoutConcurrentSave`. **Piste.** Côté appli `SELECT … FOR UPDATE` ou `pg_advisory_xact_lock` ; côté serveur, plus solide, `save_meta` avec la version attendue et refus `40001` (question 8). Les onglets à choix (`PublishChoices`) suivent le même chemin.

### 8. P1 : « Garder la sienne » remet une valeur périmée (A3)

Une cellule en conflit est gardée par la réception (`Engine.keep`) qui avance pourtant le point de reprise ; `_lattis_conflicts.server` n'est jamais mis à jour ; `Resolve(…, false)` (`receive.go:468`) écrit la valeur du moment du conflit, et le volet montre l'ancien auteur. **Preuve.** `TestAuditStaleTheirs`. **Piste.** Mettre à jour `server`, `author`, `at` à la réception, ou relire la ligne dans `Resolve`.

### 9. P1 : droits de lecture changés en cours de session (A4, S1)

- Restriction posée, ou valeur de la colonne filtrée changée (« Thé » devient « Interdit ») : la ligne reste chez le lecteur, à l'écran et sur son disque (`changes_since`, `schema/0008_sync.sql:72`, est `SECURITY INVOKER` : pas de pierre tombale pour une ligne devenue invisible).
- Restriction retirée ou filtre élargi : les lignes devenues visibles n'arrivent jamais (pas de `lattis_xid` récent) ; sur `*acl` seul, le suivi ne relit aucun tableau (`follow.go`, `cycle`) ; `RefreshCapabilities` (`capabilities.go:120`) relit les niveaux, pas les données.

**Preuves.** `TestAuditReadFilterChanged`, `TestAuditReadFilterWidened`, `TestAuditRowLeavesFilter`. **Piste.** Serveur : rendre en `deleted = true` les identifiants modifiés depuis `since` que l'appelant ne voit pas (sous-requête `SECURITY DEFINER` qui ne rend que des `lattis_id`) ; appli : relire en entier un tableau dont les capacités changent (`RemoteTable.Full`, cellules en attente gardées) ; serveur : `*acl:<table_id>`.

### 10. P1 : « modifié » et questions d'enregistrement en ligne (E3)

**Où.** `internal/doc/doc.go:387` (`Dirty`), `+page.svelte:921` (question à la fermeture), `+page.svelte:1292` (`beforeunload` en web).

Chaque saisie dans un classeur collaboratif en ligne met le point « modifié », et la fermeture demande « Enregistrer les modifications de « Ventes » ? » alors que tout est sur le serveur (« Ne pas enregistrer » ne perd rien, `keepCollabQueue` garde la file s'il en reste). En web, le navigateur ajoute « Quitter le site ? ». **Piste.** Collaboratif : ni point ni question quand la file est vide ; sinon une question qui dit la vérité (« 3 modifications ne sont pas encore parties vers X : elles partiront à la prochaine ouverture »). Figé : inchangé (question 7).

### 11. P1 : le rôle n'est affiché nulle part (E2)

Le rôle n'apparaît que dans l'infobulle d'une commande grisée (`+page.svelte:2189`) : rien dans le titre, le bandeau ou le menu Compte, et au téléphone rien du tout. **Piste.** Une pastille à côté du badge du titre (« Éditeur », « Lecteur », « Participant », rôle personnalisé), avec au clic ce qu'elle permet (capacités déjà calculées dans `access.ts`) et « Gérer l'accès… » pour les propriétaires.

### 12. P1 : édition web, perte du lien avec le moteur invisible (E9)

**Où.** `frontend/src/lib/web/runtime.ts:159-182` : reconnexion automatique, appels en cours rejetés avec « Connexion au moteur Lattis perdue », appels suivants mis en file ; personne n'écoute `host:reconnected`. Au téléphone (réseau instable), rien ne s'affiche ; moteur fermé (30 min d'inactivité) ou session expirée : reconnexions sans fin et sans message. **Piste.** Bandeau « Connexion perdue, nouvelle tentative… » (saisie suspendue), « Session expirée : se reconnecter » sur un refus d'authentification, message quand le moteur a redémarré.

### 13. P2 : ouvrir un autre classeur perd la mise en page en attente (HL7)

`FlushLayout` n'est appelé que par `stopShare` (`sharing.go:311-318`) ; `replace` (`document.go:175`) passe par `watchShare`, qui coupe l'ancien suivi sans vider. Hors ligne, une cellule libre saisie puis « Ne pas enregistrer » est perdue ; en ligne, l'envoi différé d'une seconde (`LayoutDelay`) aussi. **Piste.** `FlushLayout` dans `watchShare` avant d'annuler l'ancien suivi ; `keepCollabQueue` comparant la mise en page à `remote_base` plutôt qu'à un drapeau en mémoire.

### 14. P2 : ligne recréée par un renvoi (HL8, A5, S4)

Ajout validé sur le serveur mais réponse perdue ; un collègue supprime la ligne ; le renvoi la recrée (`pg_sync.go:144`, compté comme fait `send.go:294`) : divergence durable entre postes. Variante : une cellule renvoyée devient un faux conflit contre sa propre valeur. **Preuves.** `TestAuditAmbiguousCommit` (faux serveur), `TestAuditInsertResentAfterDelete` (PostgreSQL). **Piste.** Serveur : déclencheur `BEFORE INSERT` qui refuse un `lattis_id` ayant une pierre tombale, `why_not` « deleted » ; l'appli traite déjà ce cas.

### 15. P2 : décimaux longs en conflit sans fin (A6)

Lecture en `float8` (`selectExpr`, `pg.go`) mais écriture conditionnée par `IS NOT DISTINCT FROM $3::numeric` (`pg_sync.go:116`). Une valeur posée hors de l'appli avec plus de 17 chiffres met chaque modification en conflit, et « Garder la mienne » reboucle. **Preuve.** `TestAuditDecimalPrecision`. **Piste.** Comparer en `float8` des deux côtés.

### 16. P2 : serveur, `ready` envoyé sans écoute (S2)

`Hub.Handler` (`be-lattis-server/internal/events/events.go`) attend 5 s puis envoie `ready` même si les canaux ne sont pas écoutés (connexion PostgreSQL refusée) ; aucun `reset` ne suit. **Piste.** `503` sans couverture, ou `reset` quand elle arrive.

### 17. P2 : serveur, coût du relais SSE (S3)

Un `LISTEN` par canal sur une nouvelle connexion à chaque changement d'abonnements ; un `SELECT pg_current_wal_lsn()` par notification ; sur `*acl`, une transaction par abonné. **Piste.** `LISTEN`/`UNLISTEN` incrémentaux, position lue par rafale, revérification regroupée par classeur.

### 18. P2 : montée en charge (C1)

Une connexion `LISTEN` au parent par poste et par classeur ouvert (40 postes, 41 connexions ; limite pratique de quelques centaines avec `LATTIS_PG_MAX_CONNECTIONS=400`) ; l'appli n'utilise pas encore `/api/events`. `sync_point()` est le plus petit `xid` de toute l'instance : une sauvegarde ou une session « idle in transaction » fait relire à tous les postes tout ce qui a changé depuis ; pas d'`idle_in_transaction_session_timeout` (`deploy/postgres/lattis-conf.sh`). **Mesure.** `TestAuditLoadFollowers`. **Piste.** Bascule sur `/api/events` une fois 16 et 17 réglés (question 9) ; délai d'inactivité en transaction côté serveur.

### 19. P2 : commandes proposées que le moteur refuse ensuite (E4)

Clic droit (`Sheet.svelte:11295` et suivantes, dont les actions de 7e23f25), menu ▾ des colonnes, Documents et pages (`DocPage`/`HtmlPage` reçoivent `locked={lockedMsg}` sans le verrou de mise en page, `+page.svelte:2401`, `2457`), Fichier › Importer pour un éditeur : l'action s'affiche, le moteur la refuse avec un message. Le ruban, lui, est grisé avec le motif. **Piste.** Même règle que le ruban (`gate` de `ribbon.ts`) partout.

### 20. P2 : scripts d'un classeur collaboratif (E5)

Les scripts partent une seule fois, à la publication (`layout.go:37` : hors de la mise en page en direct). Ensuite, la correction d'un concepteur reste chez lui sans message ; un éditeur ou un lecteur peut modifier ou créer des scripts dans sa copie (`canvas/scripts.go:46`, `SaveScript` sans `layoutLock`). La spécification du serveur (§8.1) prévoit des scripts publiés et exécutés chez les autres après leur accord ; l'accord à la première exécution reste à faire (`publication.md` §7). **Piste** : question 4.

### 21. P2 : tableau chargé par une requête dans un classeur collaboratif (E7)

Actualiser remplace le tableau ; une fois publié, le moteur le refuse (`ErrImplicitDrop`, `oplog/structure.go:270` : « ne peut pas être remplacé ni retiré par cette action »). Connexions et requêtes restent sur le poste de l'auteur. **Piste** : question 1 ; en attendant, griser « Actualiser » sur un tableau publié avec le motif.

### 22. P2 : édition web, classeur local et Ctrl+S (E8)

Un classeur local en web vit dans le répertoire de la session. Le premier Ctrl+S télécharge, les suivants écrivent dans la session sans télécharger, avec le même message « Enregistré : Ventes.lattis » ; la session se ferme après 30 minutes d'inactivité. **Piste** : question 10.

### 23. P3 : mise en page, la dernière écriture l'emporte sans que le collègue le sache (HL9, E6)

Décision 0018 : un champ changé des deux côtés est gagné par le poste qui envoie. Après une semaine hors ligne, un concepteur écrase les changements récents des collègues ; le compteur `State.LayoutConflicts` (`link.go:478`) n'est lu par aucun écran. **Piste.** Montrer ces remplacements dans le volet Conflits ; règle après une longue coupure : question 3.

### 24. P3 : mise en page hors ligne jamais envoyée si les droits de conception ont été retirés (HL10)

`livelayout.go:796` : rien ne part, sans message, et la mise en page locale reste différente du serveur. Le même chemin avale aussi les erreurs passagères (sans perte, l'envoi est retenté). **Piste.** Message « Vos changements de mise en page faits hors ligne n'ont pas été envoyés : vous n'êtes plus concepteur ».

### 25. P3 : pas de délai sur l'envoi des données (HL11)

`srv.Write` sans `context.WithTimeout` (contrairement à `StructureTimeout`) : une connexion à demi coupée tient `e.run`, réception et mise en page attendent. **Piste.** Délai par étape proportionné à sa taille.

### 26. P3 : suppression et écritures refusées (F1)

`Delete` n'est pas conditionné : une suppression l'emporte sans message sur une modification concurrente pas encore reçue (question 11). Une écriture refusée par les droits suivie d'une autre sur la même cellule part en conflit au lieu d'être refusée.

### 27. P3 : flux SSE après expiration du jeton (F2)

Le jeton n'est vérifié qu'à l'ouverture ; un compte désactivé continue de recevoir des identifiants de tableaux. **Piste.** Fermer le flux à `exp`.

### 28. P3 : édition web, menu Compte du bureau (E10)

`AccountMenu.svelte:52`, `:82` : « Se déconnecter » (qui passe par le gestionnaire du bureau, `lattis.go:176`, pas par `FixedSession`) et « Gérer les serveurs… » proposés ; l'utilisateur s'affiche par son rôle PostgreSQL. **Piste.** Nom et adresse donnés par lattisd, déconnexion de lattisd, pas de gestion des serveurs.

### 29. P3 : présence des membres (E6)

Le travail à plusieurs sur un dossier partagé montre qui est là (`CoEditPresence`) ; un classeur du serveur non. `ShareEditing` existe déjà pour la cellule en cours de saisie. **Piste.** Initiales des membres présents dans la barre d'état, après la bascule sur `/api/events` (18).

### 30. P3 : détails (E11)

Figé, lecteur : Ctrl+S enregistre une « copie locale » du cache, qui n'est pas un fichier de l'utilisateur ; proposer « Enregistrer une copie… ». Réglages locaux d'un lecteur (filtre, mise en forme) perdus au rechargement (`publication.md` §7). Thème d'un classeur collaboratif pour un éditeur : vérifier qu'il est grisé avec le motif plutôt que refusé après coup.

---

## 4. Ce qui est en place (à ne pas refaire)

Titre et badge d'un classeur ouvert depuis le serveur, cache jamais montré ; barre d'état « Synchronisation… », « Hors ligne — 3 en attente » ; bandeau hors ligne, disparu, accès retiré, conflits, nouvelle version ; ruban grisé avec motif ; lignes des autres grisées pour un participant ; « Vue filtrée par vos droits » ; Gérer l'accès ; Ctrl+Z limité à ses propres actions ; structure réservée aux concepteurs ; hors ligne modifiable avec file gardée ; Travailler sur une copie locale, Détacher ; en web : scripts coupés par défaut, MCP absent, raccourcis du navigateur remplacés, chemins masqués, sources réseau filtrées, téléphone (défilement, appui long, ruban réduit).

## 5. Ordre de traitement proposé

1. **1** puis **2** (P0).
2. **3**, **6**, **13** : petites corrections du même chemin d'envoi et de fermeture, sans décision à prendre.
3. **9** (avec la modification serveur), puis **8**, **7** (selon la question 8), **14** (serveur), **15**.
4. **10**, **11**, **12** : présentation, rapides et visibles.
5. **4** et **5** après les questions 2 et 5.
6. **19** ; **20**, **21**, **22** après les questions 4, 1 et 10.
7. **16**, **17**, puis la bascule **18** et la présence **29**.
8. P3 restants.

## 6. Questions pour Louis

1. **Tableaux de requête publiés (21).** Une fois publié, qui actualise et comment ? (a) le poste qui a la connexion, l'actualisation remplaçant les lignes au lieu du tableau ; (b) le tableau devient une source du Data Lake actualisée par un pipeline du serveur.
   **Recommandation : (a)**, qui marche avec les connexions d'aujourd'hui ; (b) reste possible plus tard pour les sources partagées par plusieurs classeurs.
2. **Fichier de l'auteur (4).** Après publication, (a) le fichier d'auteur reste le classeur vivant (badge, messages, réouverture telle quelle s'il a une file) ; ou (b) l'auteur bascule sur la copie du serveur et son fichier devient une archive.
   **Recommandation : (b)**, dans la ligne de c2f880e : une seule copie vivante, plus de modifications coincées dans un fichier que plus rien n'envoie.
3. **Mise en page après une longue coupure (23).** Le poste qui revient l'emporte aujourd'hui. (a) garder, en montrant les remplacements ; (b) au-delà d'une durée (par exemple 24 h), le serveur l'emporte sur les champs en conflit.
   **Recommandation : (b)**, en montrant aussi les remplacements : après une longue absence, le travail récent des collègues compte plus que l'ancien.
4. **Scripts d'un classeur collaboratif (20).** (a) seuls propriétaires et concepteurs les modifient, ils partent chez tous, et chacun donne son accord avant d'exécuter une nouvelle version ; (b) scripts figés à la publication, modifiables seulement par une republication.
   **Recommandation : (a)**, c'est ce que prévoit la spécification du serveur (§8.1), et l'accord protège chaque membre d'un script modifié.
5. **Dossier partagé (NAS), poste coupé (5).** (a) ne compacter qu'une fois que les postes partis depuis moins de 24 h ont tout appliqué ; (b) au retour, si un journal a disparu, demander de rouvrir le classeur.
   **Recommandation : (a)**, invisible pour l'utilisateur ; (b) reste le filet si un poste revient après plus de 24 h.
6. **Étape qui échoue toujours (1).** Après quelques échecs identiques, la mettre de côté dans le volet Conflits pour que le reste continue ? (a) oui ; (b) non, la valeur d'une colonne supprimée est simplement écartée avec un message et rien d'autre ne change.
   **Recommandation : (a)** : la réception ne doit jamais être bloquée par une seule étape, quelle qu'en soit la cause.
7. **Enregistrer en ligne (10).** Pour un classeur collaboratif : (a) plus de point « modifié » ni de question à la fermeture (Ctrl+S reste, sans effet visible) ; (b) garder le point mais remplacer la question par l'état de la file.
   **Recommandation : (a)** : c'est le comportement attendu d'un classeur en ligne ; la question ne reste que s'il y a des modifications pas encore parties.
8. **Mise en page concurrente (7).** Corriger (a) côté serveur, `save_meta` avec la version attendue (le plus sûr, migration à écrire) ; ou (b) côté appli seulement, par un verrou dans la transaction (plus rapide, moins robuste).
   **Recommandation : (a)** : le serveur est le seul à voir les deux enregistrements ; un verrou côté appli ne protège pas d'un autre client.
9. **Bascule sur `/api/events` (18).** (a) la planifier maintenant, après 16 et 17 ; (b) la repousser tant que l'on reste sous quelques centaines de postes.
   **Recommandation : (b)** : rien ne presse à l'échelle actuelle ; corriger d'abord les constats de stabilité, puis basculer avec la présence (29).
10. **Édition web, classeur local (22).** (a) message juste et commande « Télécharger » explicite ; (b) en plus, inviter à publier sur le serveur comme vraie sauvegarde.
   **Recommandation : (b)** : le message et le bouton sont petits, et le serveur est la seule sauvegarde durable en web.
11. **Suppression concurrente (26).** Une suppression qui l'emporte sur la modification d'un collègue : (a) voulu, dernière écriture gagnante, avec un message ; (b) conditionner la suppression et en faire un conflit.
   **Recommandation : (a)**, comme Excel en ligne : simple et prévisible ; le message suffit à prévenir le collègue.

## Annexe : essais

Les essais de l'audit sont devenus des essais de non-régression, qui passent : `internal/remote/collab/enligne_test.go` (faux serveur), `internal/remote/collab/enligne_pg_test.go` (PostgreSQL, `LATTIS_PG_TEST=1`, voir `publication.md` §6), `internal/bridge/enligne_test.go`, `internal/coedit/enligne_test.go`, `internal/remote/workbook/events_test.go`. Les fichiers d'origine (`.go.txt`) restent sur les branches des audits.

## 7. Suivi (20 octobre 2026)

Décisions de Louis : 1 « au choix à la publication », 2a, 3b (en expliquant pourquoi), 4a, 5a, 6a, 7a, 8a, 9a, 10b, 11a. Réalisé sur `main` des deux dépôts (be-lattis-server : 883f7df migration 0032, 14721dd et 53e9543 relais `/api/events`, 832cf9f réglage PostgreSQL).

| # | État | Commit(s) | Remarque |
|---|---|---|---|
| 1 | Corrigé | 6d255b0, 23ea58e | structure reçue au second essai ; étape qui échoue toujours mise de côté (question 6) |
| 2 | Corrigé | 2af92ed | récupération proposée, cache jamais écrasé (abîmé : mis de côté) |
| 3 | Corrigé | ff1a44c | `workbook.Transient`, `Refusal` |
| 4 | Corrigé | d76386a | badge et message de l'auteur ; rouvert tel quel s'il a des modifications pas encore envoyées (manifeste `unsent`) |
| 5 | Corrigé | f8e2269 | poste coupé attendu 24 h (`coedit.AwayKeep`) |
| 6 | Corrigé | 2af92ed | `Release` au lieu de `Close`, message |
| 7 | Corrigé | 1d167a4 + serveur 883f7df | `save_meta` avec version attendue (40001, refusion aussitôt) |
| 8 | Corrigé | ccfd851 | conflit mis à jour à la réception |
| 9 | Corrigé | 1d167a4 + serveur 883f7df | tableaux relus après un changement de droits ; lignes sorties du filtre rendues supprimées |
| 10 | Fait | 04b332f | ni point « modifié » ni question ; file envoyée à la fermeture ; question seulement hors ligne |
| 11 | Fait | d76386a | pastille du rôle |
| 12 | Fait | 54b0dd5 | bandeau de connexion perdue, session expirée |
| 13 | Corrigé | 2af92ed | mise en page envoyée avant d'ouvrir un autre classeur |
| 14 | Corrigé | 1d167a4 + serveur 883f7df | ligne supprimée jamais recréée ; le renvoi est traité comme « ligne supprimée » |
| 15 | Corrigé | ccfd851 | décimaux comparés en `float8` |
| 16 | Corrigé | serveur 14721dd | `503` sans écoute |
| 17 | En partie | serveur 14721dd | LISTEN en un aller-retour, revérification des droits par utilisateur (4 à la fois) ; la position WAL reste lue à chaque notification (la lire une fois par rafale pourrait la donner trop tôt) |
| 18 | Fait | d338378 + serveur 832cf9f | l'appli suit le relais (repli sur LISTEN sans relais ; `LATTIS_LISTEN=1` force LISTEN) ; `idle_in_transaction_session_timeout` 5 min. Non essayé de bout en bout contre lattisd (essais : flux simulé) |
| 19 | Fait | d3832ad | clic droit, ▾, Documents, Importer |
| 20 | Fait | aaf39fb | scripts partagés, réservés aux concepteurs ; accord par version (`internal/trust`). Reste : un script supprimé n'est pas retiré du serveur |
| 21 | Fait | cc5b976 | lignes remplacées sur place ; choix à la publication (Data Lake : boîte de la source après publication) |
| 22 | Fait | a5d0d12 | Fichier › Télécharger une copie, message |
| 23 | Fait | d038da2 | au-delà de 24 h sans synchronisation, le serveur l'emporte (message) ; remplacements dans le volet Conflits |
| 24 | Fait | d038da2 | message, mise en page du serveur reprise |
| 25 | Fait | 9557ea7 | délai de 30 s + 20 ms par écriture |
| 26 | En partie | 9557ea7 | suppression concurrente signalée ; l'écriture refusée suivie d'une autre sur la même cellule part toujours en conflit |
| 27 | Corrigé | serveur 14721dd | flux fermé à l'expiration (`expired`) |
| 28 | Fait | 9557ea7 | menu Compte de l'édition web |
| 29 | Fait | d338378 + serveur 53e9543 | présence par le relais (événement `presence`) |
| 30 | En partie | 9557ea7 | figé en lecteur et thème ; les réglages locaux d'un lecteur perdus au rechargement restent à faire |
