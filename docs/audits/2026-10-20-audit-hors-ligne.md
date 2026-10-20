# Audit de stabilité du mode hors ligne, 20 octobre 2026

Périmètre : un classeur collaboratif ouvert depuis un Serveur Lattis et modifié hors ligne (décisions 0012, 0014), la mise en page en direct (décision 0018, ajoutée le jour même), l'ouverture du fichier d'auteur d'un classeur partagé (commit c2f880e), le classeur supprimé du serveur, et le travail à plusieurs sur un dossier partagé coupé du réseau (décision 0011). Le mode collaboratif en ligne fait l'objet d'un autre audit.

Méthode : lecture du code de `main` (jusqu'à a9937df), puis essais écrits pour chaque piste, lancés sur une copie du dépôt (aucun fichier du dépôt modifié). Les essais sont en annexe ([essais-hors-ligne-2026-10-20/](essais-hors-ligne-2026-10-20/), extension `.go.txt` pour qu'ils ne soient pas compilés) ; ils échouent tous aujourd'hui, c'est le constat. Les essais existants passent : `go test ./internal/remote/... ./internal/coedit/` et les essais du pont sur le partage, le détachement, la récupération et le travail à plusieurs.

## Synthèse

| # | Gravité | Constat | Essai |
|---|---|---|---|
| 1 | Critique | Une colonne renommée ou supprimée (ou un tableau supprimé) par un collègue pendant la coupure bloque le classeur hors ligne pour toujours | `TestAuditStaleColumnLivelock`, `TestAuditStaleTableLivelock` |
| 2 | Critique | Après un plantage hors ligne, rouvrir le classeur depuis le serveur écrase le cache et sa file sans proposer la récupération | `TestAuditShareOpenAfterCrash` |
| 3 | Élevé | Une erreur passagère du serveur (interblocage, délai) est prise pour un refus : la saisie est défaite | `TestAuditTransientErrorIsRejection` |
| 4 | Élevé | Le fichier d'auteur qui a des modifications hors ligne en attente s'ouvre désormais depuis le serveur : elles restent coincées dans le fichier | lu dans le code |
| 5 | Élevé | Dossier partagé (NAS) : un poste coupé plus de 30 s rate pour toujours ce que les autres ont enregistré pendant sa coupure | `TestAuditCompactedWhileAway` |
| 6 | Élevé | Si l'enregistrement du cache échoue à la fermeture, la file est effacée avec le répertoire de travail, sans message | lu dans le code |
| 7 | Moyen | Ouvrir un autre classeur ne fait pas partir la mise en page en attente : hors ligne, elle est perdue | lu dans le code |
| 8 | Moyen | Coupure pendant la validation d'un envoi : une ligne supprimée entre-temps par un collègue ressuscite sur le serveur, et les postes divergent | `TestAuditAmbiguousCommit` |
| 9 | Mineur | Mise en page après une longue coupure : la version du poste qui revient écrase les changements récents des collègues | lu dans le code |
| 10 | Mineur | Droits de conception retirés pendant la coupure : la mise en page faite hors ligne ne part jamais, sans message | lu dans le code |
| 11 | Mineur | Aucun délai sur l'envoi des données : une connexion à demi coupée tient le verrou de synchronisation | lu dans le code |

Ce qui tient bien : la file est écrite dans la même transaction que la modification (rien ne se perd en cas de plantage local) ; le renvoi d'une étape déjà validée sur le serveur ne crée pas de doublon (écritures conditionnées, `ON CONFLICT DO NOTHING`) ; la longue coupure avec pierres tombales purgées relit le tableau après l'envoi sans perdre les conflits ; le classeur supprimé du serveur garde sa file et le dit ; la file d'un cache est enregistrée à la fermeture même après « Ne pas enregistrer ».

---

## 1. Critique : classeur bloqué hors ligne pour toujours après un renommage de colonne par un collègue

**Où.** `internal/remote/collab/send.go:306` (une écriture sur une colonne ou un tableau inconnu du serveur devient `errStale`), `send.go:120` (un seul nouvel essai, avec la correspondance **locale** relue), `internal/remote/collab/follow.go:205-247` (le cycle envoie avant de recevoir ; une erreur d'envoi saute la réception et repasse hors ligne).

**Scénario.** Bob est hors ligne et corrige un prix. Pendant ce temps, Alice (conceptrice) renomme la colonne « Prix » en « Tarif ». Bob revient : son `UPDATE … SET "Prix"` reçoit 42703, l'étape est réessayée une fois avec la même correspondance (rien n'a été relu du serveur), puis l'envoi échoue. Le cycle s'arrête avant la réception, qui aurait appris le renommage ; le classeur repasse « hors ligne » avec le message technique, réessaie, et recommence indéfiniment. Même chose si la colonne ou le tableau a été supprimé (42P01).

**Conséquences.** La saisie de Bob ne part jamais, et Bob ne reçoit plus rien des autres (données, structure, mise en page) alors que le serveur répond. Le bandeau dit « ne répond pas ». Seules issues : copie locale ou détacher. C'est le scénario normal d'une longue coupure dans une équipe où quelqu'un réorganise un tableau.

**Repro.** `TestAuditStaleColumnLivelock` : trois `Sync` de suite rendent « structure du tableau changée sur le serveur : la colonne « Prix » n'existe pas (SQLSTATE 42703) », la file reste à 1. `TestAuditStaleTableLivelock` : idem avec le tableau supprimé (42P01).

**Piste.** Sur `errStale`, relire la structure du serveur (`receiveStructure` du tableau concerné) avant le nouvel essai, puis traiter l'étape : valeurs d'une colonne renommée envoyées sous le nouveau nom, valeurs d'une colonne ou d'un tableau supprimés écartées avec un message (« Colonne supprimée entre-temps par Alice : 1 modification abandonnée »). Plus généralement, une étape qui échoue toujours ne devrait pas bloquer la réception : après N échecs identiques, la mettre de côté (état `stuck`, visible dans le volet Conflits) et continuer.

## 2. Critique : après un plantage, rouvrir depuis le serveur écrase le cache et sa file

**Où.** `internal/bridge/sharing.go:741-755` (`ShareOpen`).

```go
if _, err := os.Stat(path); err == nil {
    if d, err = doc.Open(ctx, path); err == nil {
        fresh = false
    }
}
if d == nil { d, err = doc.New(ctx) … }   // toute erreur d'ouverture → classeur vide
if fresh || !keepCollabCache(ctx, d) { workbook.Load(…, fresh) }  // rechargé du serveur
…
saveDoc(ctx, d, path)                       // enregistré par-dessus le cache
```

**Scénario.** Louis modifie hors ligne un classeur ouvert depuis le serveur ; la machine s'éteint (batterie, plantage). Au redémarrage, le serveur est revenu ; il rouvre le classeur depuis les récents (ou un lien `lattis://`). `doc.Open` rend `RecoverableError` (session interrompue) : l'erreur est avalée, le classeur est rechargé du serveur et **enregistré sur le cache**. Les modifications des sessions précédentes que le cache gardait (fermeture hors ligne, décision 0014) disparaissent du cache ; celles de la session plantée ne sont plus que dans le répertoire orphelin, proposé au prochain lancement. Si Louis l'écarte en pensant que le serveur a tout, elles sont perdues ; s'il le récupère, la récupération se fait par-dessus un cache qui a changé (« modifié par ailleurs »).

Même effet pour toute autre erreur d'ouverture du cache (fichier abîmé, répertoire de travail verrouillé).

**Repro.** `TestAuditShareOpenAfterCrash` : saisie hors ligne fermée (cache avec file), réouverture hors ligne et seconde saisie, état d'un plantage reconstitué (cache d'avant, répertoire orphelin), serveur revenu, `ShareOpen` : pas de question de récupération, 0 modification en attente, prix du serveur affichés.

**Piste.** Dans `ShareOpen`, ne passer à `doc.New` que si le cache n'existe pas ; rendre `OpenResult{Recoverable: …}` sur `RecoverableError` (comme `open`), et sur une autre erreur, ne pas écraser le cache (le renommer en `.abime` ou demander).

## 3. Élevé : une erreur passagère du serveur défait la saisie

**Où.** `internal/remote/collab/send.go:308-313` et `:340` ; `internal/remote/workbook/server.go:311` (`Rejected` : toute erreur qui a un code SQLSTATE) ; `internal/remote/collab/structure.go:181` pour la structure.

**Scénario.** Au retour d'une coupure, la file de Bob part pendant qu'Alice trie le même tableau : PostgreSQL détecte un interblocage (40P01) sur une ligne. Ou bien un délai `statement_timeout`/`lock_timeout` du serveur expire (57014, 55P03), ou une annulation administrative. Ces erreurs ont un code SQLSTATE, donc `Rejected` les prend pour un refus : l'opération est marquée refusée, la ligne reprend la valeur du serveur, message « Modification refusée par le serveur, annulée ». Pour un changement de structure (sans point de sauvegarde), tout le changement est défait (`refuseStructure`), les changements suivants du même tableau abandonnés, et les valeurs d'une colonne ajoutée hors ligne écartées.

**Repro.** `TestAuditTransientErrorIsRejection` (40P01, 57014, 55P03) : la saisie hors ligne « 77 » redevient « 3 ».

**Piste.** Classer les codes passagers (classe 40, 53, 55P03, 57014, 57P0x, 08, `25006` réplica en lecture seule) comme une coupure : l'étape reste en attente et repart au tour suivant. Un refus, c'est 42501, 23xxx, 22xxx, P0001 et les codes propres au schéma `lattis`.

## 4. Élevé : modifications hors ligne coincées dans le fichier d'auteur

**Où.** `internal/bridge/document.go:217` et `:232` (`sharedFile`), `frontend/src/routes/+page.svelte` (`opened` : `r.shared` → `openRecent`).

**Scénario.** Alice a publié `Ventes.lattis` en collaboratif ; elle le modifie dans un train (hors ligne), fait `Ctrl+S` : la file est dans son fichier (décision 0014, « le fichier publié d'un propriétaire suit les règles d'un fichier local »). De retour au bureau, elle double-clique le fichier. Depuis c2f880e, `Open` voit un fichier d'auteur collaboratif et l'ouvre **depuis le serveur** (cache, un autre fichier) : Alice voit la version du serveur, sans ses modifications, et rien ne les envoie. Le message dit « ouvert en ligne ». Elles ne partiront que si elle ouvre un jour ce fichier par « copie locale » (seulement proposée quand le serveur est injoignable).

**Piste.** Dans `sharedFile`, lire `workbook.Unsent` du fichier (ou une marque du manifeste écrite à l'enregistrement) : s'il a des modifications en attente ou des conflits, l'ouvrir tel quel (il se synchronise de lui-même en ligne) ou demander « Vos modifications hors ligne de ce fichier n'ont pas été envoyées : les envoyer maintenant ? ».

## 5. Élevé : dossier partagé, un poste coupé rate ce qui a été compacté pendant sa coupure

**Où.** `internal/coedit/apply.go:505-532` (`compact` ne compte que les sessions vivantes) ; `apply.go:119` et `:141` (un journal disparu est considéré comme « dans le fichier », sans relire le fichier).

**Scénario.** Bob perd le NAS plus de 30 s (Wi-Fi). Pour Alice, Bob est « parti ». Alice saisit, puis `Ctrl+S` : son segment est couvert par le fichier et ouvre le suivant ; 10 s plus tard, le compactage supprime l'ancien segment (aucune session vivante ne l'attend). Bob revient : le journal a disparu, ses lignes ne viendront plus, et Bob ne relit pas le fichier. Il garde l'ancienne valeur, sans message ; les lignes suivantes d'Alice lui arrivent normalement (leurs dépendances sur le journal disparu sont réputées satisfaites). À l'enregistrement de Bob, « modifié par ailleurs » protège le fichier, mais « Écraser » efface la saisie d'Alice.

**Repro.** `TestAuditCompactedWhileAway` : Alice `Café|42|…`, Bob `Café|3|…` après 20 tours ; la saisie suivante d'Alice (Thé 43) arrive bien chez Bob. (Le retour de Bob côté écriture fonctionne : `TestAuditReturnAfterCompaction` passe.)

**Piste.** Ne compacter un segment que si toutes les sessions **présentes dans le dossier** (vivantes ou parties depuis moins d'un délai long, par exemple 24 h, ou jusqu'à ce que leur présence disparaisse) l'ont appliqué ; ou, côté retour, quand un journal non entièrement appliqué a disparu, traiter comme une barrière (« Alice a enregistré pendant votre déconnexion : rouvrez le classeur »).

## 6. Élevé : file effacée si l'enregistrement du cache échoue à la fermeture

**Où.** `internal/bridge/remotedoc.go:56-68` (`keepCollabQueue` : l'échec est seulement écrit au journal), `internal/bridge/document.go:185-186` (`replace` : `old.Close()` juste après, qui supprime le répertoire de travail, `internal/archive/archive.go:1117`).

**Scénario.** Fermeture (ou ouverture d'un autre classeur) d'un classeur du serveur hors ligne, avec des modifications en attente ; l'enregistrement du cache échoue (disque plein, fichier tenu par un antivirus ou un outil de synchronisation, « modifié par ailleurs »). Le message ne va qu'au journal technique, puis `Close` efface le répertoire de travail : la file est perdue. (`ServiceShutdown` est protégé : un classeur modifié garde son répertoire pour la récupération.)

**Piste.** Si `keepCollabQueue` échoue, `Release` au lieu de `Close` (le répertoire reste, proposé en récupération) et le dire à l'utilisateur.

## 7. Moyen : ouvrir un autre classeur perd la mise en page en attente (code du jour)

**Où.** `internal/bridge/sharing.go:311-318` : `FlushLayout` (qui note `LayoutUnsent`) n'est appelé que par `stopShare` ; `internal/bridge/document.go:175` : `replace` appelle `watchShare(d)` pour le nouveau classeur, qui coupe le suivi de l'ancien **sans** `FlushLayout`.

**Scénario.** Concepteur hors ligne, saisie d'une cellule libre seulement (aucune donnée de tableau en attente), puis Fichier › Ouvrir un autre classeur (ou un récent) et « Ne pas enregistrer » : `LayoutUnsent` est faux, `keepCollabQueue` n'enregistre pas le cache, la cellule libre est perdue. En ligne, l'envoi différé d'une seconde (`LayoutDelay`) est aussi perdu si l'on change de classeur pendant ce délai. Fermer le classeur (`CloseDocument`) ou quitter l'application passent bien par `stopShare`.

**Piste.** Appeler `FlushLayout` dans `watchShare` avant d'annuler l'ancien suivi (ou faire de `stopShare` la première étape de `replace`). Indépendamment, `keepCollabQueue` pourrait comparer la mise en page à `remote_base` plutôt que de dépendre d'un drapeau en mémoire.

## 8. Moyen : coupure pendant la validation, ligne ressuscitée et postes divergents

**Où.** `internal/remote/workbook/pg_sync.go:144` (`INSERT … ON CONFLICT (lattis_id) DO NOTHING`), `internal/remote/collab/send.go:294`.

**Scénario.** La transaction d'un ajout de ligne est validée sur le serveur, mais la réponse se perd (coupure au moment du `COMMIT`) : l'étape reste en attente. Avant le renvoi, une collègue reçoit la ligne et la supprime. Au renvoi, l'`INSERT` ne trouve plus la ligne (supprimée) et la recrée. Le serveur et la collègue ont la ligne ; le poste de l'auteur l'a supprimée (il a reçu la suppression) : divergence durable. Variante sans gravité : une cellule renvoyée qu'un collègue a changée entre-temps devient un faux conflit contre sa propre valeur.

**Repro.** `TestAuditAmbiguousCommit` : Alice `[Café Thé Chocolat Sucre]`, Bob `[Café Thé Chocolat]`. L'essai utilise le faux serveur ; un vrai serveur qui refuse de réinsérer un identifiant ayant une pierre tombale n'aurait pas le défaut (à vérifier dans lattisd).

**Piste.** Côté serveur, `INSERT` refusé (ou sans effet) quand `lattis_id` a une pierre tombale ; côté client, traiter « supprimée » (`why_not`) au renvoi d'un ajout comme pour une modification (`Lost`).

## 9. Mineur : la mise en page du poste qui revient d'une longue coupure l'emporte

**Où.** Décision 0018, « fusion à trois versions » : un champ changé des deux côtés est gagné par le poste qui envoie.

Après une semaine hors ligne, un concepteur qui revient écrase les changements récents des collègues sur les mêmes champs (largeur, mise en forme, formule d'une cellule libre), avec seulement un message compté. Pas de volet pour revenir en arrière. Piste : au-delà d'une coupure longue, faire gagner le serveur sur les champs en conflit, ou garder les valeurs remplacées pour un « Annuler » côté collègue.

## 10. Mineur : mise en page hors ligne jamais envoyée si les droits ont été retirés

**Où.** `internal/remote/workbook/livelayout.go:796` (« droits retirés : rien ne part »).

Le refus est silencieux et la mise en page locale reste différente du serveur, sans message. Piste : un message « Vos changements de mise en page faits hors ligne n'ont pas été envoyés : vous n'êtes plus concepteur ». Ce même chemin avale aussi les erreurs passagères (constat 3), sans perte ici puisque l'envoi est retenté.

## 11. Mineur : pas de délai sur l'envoi des données

**Où.** `internal/remote/collab/send.go` (`srv.Write` sans `context.WithTimeout`, contrairement à la structure, `StructureTimeout`).

Une connexion à demi coupée (changement de réseau) bloque l'envoi jusqu'à ce que TCP s'en aperçoive, en tenant `e.run` : réception et mise en page attendent aussi. Piste : un délai par étape, proportionné à sa taille.

---

## Annexe : essais

Dans [essais-hors-ligne-2026-10-20/](essais-hors-ligne-2026-10-20/) :

- `collab_audit_offline_test.go.txt` → `internal/remote/collab/` (constats 1, 3, 8) ;
- `bridge_audit_offline_test.go.txt` → `internal/bridge/` (constat 2) ;
- `coedit_audit_offline_test.go.txt` → `internal/coedit/` (constat 5).

Pour les lancer : copier le fichier dans son paquet sans `.txt`, puis `go test ./internal/remote/collab/ ./internal/bridge/ ./internal/coedit/ -run TestAudit -v`. Ils deviennent des essais de non-régression une fois les corrections faites.
