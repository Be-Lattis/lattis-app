# Audit : collaboration sous charge (30/10/2026)

*Suite de l'audit général du 30/10 (§ 6). Périmètre : suivi temps réel des classeurs collaboratifs à 100 et 300 postes, connexions PostgreSQL, relais `/api/events`, coupures et reprises. Application : `main` à `8f654358` ; serveur : be-lattis-server `main` à `2bdffd6` (lu, lancé, pas modifié).*

## Banc d'essai

Tout est réel sauf l'interface : PostgreSQL 18 de l'image `be-lattis/server:dev-pg18` (module `lattis_oauth`, TLS, `max_connections` 1000 puis 400), **lattisd** construit depuis le dépôt du serveur et lancé sur le poste (`--migrate`), et dans un seul processus d'essai N postes branchés comme l'application : `pgpool` en OAUTHBEARER avec un jeton signé par la clé de lattisd, relais `/api/events` (`workbook.WithEvents`), `collab.Engine.Follow`, `collab.Debounce` à 200 ms. Une seule machine Windows porte les trois (les chiffres de latence comprennent donc la charge des 300 postes simulés).

- `internal/remote/collab/lattisd_test.go` : harnais (comptes, jetons, postes, relais TCP coupable).
- `internal/remote/collab/charge_test.go` : `TestCharge`, lancé seulement avec `LATTIS_CHARGE=N` (`LATTIS_CHARGE_LISTEN=1` pour LISTEN, `LATTIS_CHARGE_METRICS` pour la mémoire de lattisd).
- `internal/remote/collab/lattisd_reprise_test.go` : coupures et reprises de bout en bout, lancés avec `LATTIS_LATTISD_URL`.

```bash
LATTIS_LATTISD_URL=http://127.0.0.1:8095 LATTIS_LATTISD_KEY=…/keys/signing.pem \
LATTIS_LATTISD_PG=127.0.0.1:55434 LATTIS_LATTISD_SERVICE='postgres://lattis_service:…@127.0.0.1:55434/lattis?sslmode=require' \
LATTIS_CHARGE=300 LATTIS_CHARGE_METRICS=http://127.0.0.1:9475/metrics go test ./internal/remote/collab/ -run 'TestCharge$|TestLattisd' -v -timeout 30m
```

## Mesures

Propagation : une saisie d'un poste visible chez tous les autres, 20 saisies, regroupement de 200 ms compris. Rafale : 30 postes saisissent en même temps.

| | 100 relais | 100 LISTEN | 300 relais | 300 LISTEN |
|---|---|---|---|---|
| Propagation, médiane / 95e c. / pire | 244 / 266 / 708 ms | 241 / 250 / 255 ms | **300 / 325 / 343 ms** | 588 / 971 ms / **2,83 s** |
| Rafale de 30 saisies, tout reçu partout | 316 ms | 333 ms | 510 ms | 545 ms |
| Connexions des postes (repos) | 101 | 202 | 301 | 602 |
| Connexions de lattisd par utilisateur (`lattisd/admin`) | ≈ 100 | 0 (restes) | **301** | 0 (restes) |
| Total sur la base | 249 | 342 | **620** | 921 |
| Transactions par saisie et par poste | 1,8 | 2,4 | 1,1 à 1,2 | 1,25 |
| lattisd : mémoire / goroutines | 88 Mo / 405 | — | 216 Mo / 1 100 à 1 700 | — |
| Application, par poste ouvert | 0,35 Mo de tas Go, ≈ 2 à 3 Mo de processus (petit classeur) | idem | idem | idem |

PostgreSQL au pire des essais à 300 : 11 cœurs et 3,7 Go pendant l'ouverture simultanée, quelques pour cent au repos.

**Lecture.** Le relais tient 300 postes avec une latence plate (+60 ms de 100 à 300) ; LISTEN se dégrade nettement à 300 (pire 2,8 s) et double les connexions. Mais **avec le relais, lattisd ouvre une connexion par utilisateur abonné** (vérification des droits) : le total revient au même ordre que LISTEN. Avec `max_connections` à 400 (valeur par défaut de l'image), **300 postes ne passent pas** : lattisd ne peut plus ouvrir sa propre écoute (« too many clients already »), le relais répond 503 et les postes repassent sur LISTEN, ce qui ajoute encore des connexions. L'annonce « de l'ordre de 300 postes par parent » (spécification du serveur, § 21, jalon 8) ne tient donc pas aujourd'hui.

## Constats

### C1 [corrigé, c7ecd7bc] Hors ligne toutes les 15 minutes

lattisd ferme le flux à l'expiration du jeton (`expired`, correction F2 du serveur). L'application traitait `expired` (et `reset`) comme une coupure : bandeau « hors ligne » quelques instants à chaque renouvellement du jeton (15 min), nouvel essai après le délai de reprise. Mesuré : 2 passages hors ligne en 7 s avec un jeton de 3 s. Corrigé : `EventsServer.Listen` se réabonne aussitôt avec un jeton neuf ; « ready » fait tout relire, rien n'est perdu ; plus de 3 flux de suite qui se referment en moins de 5 s : coupure comme avant. Essais : `TestEventsExpiredResubscribes`, `TestLattisdTokenExpiresWhileFollowing`.

### C2 [corrigé, bf943ae2] Droits réduits pendant une coupure, pas relus à la reprise

À la reprise, la première connexion prise dans le pool est souvent morte (« conn closed ») : la relecture des droits échouait, l'erreur était ignorée et la file partait avec les droits d'avant la coupure. Le serveur refusait la saisie sans message, et le poste restait « éditeur » jusqu'à la reprise suivante (la notification `*acl` est partie pendant la coupure). Corrigé (`follow.go`) : relecture en échec, rien ne part, le tour est refait comme après une coupure. Essais : `TestOfflineRightsUnreadAtReconnect` (faux serveur), `TestLattisdRightsReducedMeanwhile`.

### C3 Vérifié sans écart, de bout en bout

Coupure plus longue que le jeton (jeton gardé périmé, refusé par PostgreSQL et par le relais, renouvelé) ; colonne renommée et colonne supprimée pendant la coupure (A1 du 20/10 tient : rien de bloqué, la valeur arrive dans la colonne renommée) ; classeur supprimé (suivi arrêté, file gardée) ; accès retiré (noté, file gardée).

### C4 État des points serveur du 20/10

- **S2** (`ready` sans écoute) : **corrigé** côté serveur (`14721dd`, 503 et `Retry-After`).
- **F2** (flux après expiration du jeton) : **corrigé** (`14721dd`, événement `expired`, compte désactivé refusé) ; côté application, voir C1.
- **S3** (coût du relais) : **en partie**. Fait : LISTEN en un aller-retour, droits revérifiés par utilisateur, 4 à la fois. Reste : chaque changement de l'ensemble des canaux rouvre une connexion et réécoute tout ; `SELECT pg_current_wal_lsn()` à chaque notification ; voir aussi S5.
- **Bascule sur `/api/events`** : faite dans l'application depuis le 20/10 (`d3383781`), repli sur LISTEN si 404 ou 503. La position WAL des événements n'est pas utilisée : les deltas sont lus sur le parent (pas de réplica dans le chemin de suivi).

### S5 [arbitrage, serveur] Une connexion PostgreSQL par utilisateur dans lattisd

`events.Hub.channelOf` vérifie l'accès sous l'identité de l'utilisateur par `userdb.OAuth` : un pool par utilisateur (`MaxConns 2`, connexion gardée 2 min, pool 10 min), soit **une connexion par personne abonnée**, et de nouveau à chaque réabonnement (toutes les 15 min, après chaque coupure, après un `*acl`). À 300 postes : 301 connexions de plus, 1 100 goroutines dans lattisd. Propositions pour le fil serveur :
1. vérifier l'accès sur le pool de service par une fonction `SECURITY DEFINER` (`lattis.can_follow(user_id, workbook_id)`, même règle que `my_capabilities`) : plus aucune connexion par utilisateur ;
2. ou, dans l'esprit du jalon 9, une transaction de service avec `SET LOCAL ROLE <pg_role>` ;
3. a minima, mettre en cache le résultat par (utilisateur, classeur) jusqu'au prochain `*acl`, et fermer ces pools après quelques secondes.

### S6 [arbitrage, serveur] `max_connections` et dimensionnement

Aujourd'hui : un poste = 1 connexion (relais) ou 2 (LISTEN), plus S5. Avec S5 réglé, 300 postes tiennent sous 400 (≈ 320). Proposer dans l'installation : `max_connections` = postes × 1,2 + 50, et corriger la phrase du jalon 8 ; au-delà de quelques centaines, le jalon 9 (écritures et deltas par lattisd) reste la vraie réponse.

### A1 [arbitrage, appli] 503 du relais : repli immédiat sur LISTEN

Hors redémarrage de lattisd, un 503 fait écouter PostgreSQL directement. Quand le 503 vient d'une base saturée (cas mesuré à 400 connexions), ce repli ajoute une connexion par poste et aggrave la saturation. Options : (a) garder ; (b) respecter `Retry-After` et redemander le relais pendant 30 s avant le repli (le poste reste « hors ligne » pendant ce temps à la connexion) ; (c) ne jamais se replier si le serveur annonce le relais (404 seul déclenche LISTEN). Préférence : (b).

### A2 [arbitrage, appli] Connexion gardée par poste au repos

Le pool `rw` de chaque poste garde sa connexion 30 min sans usage (réglage par défaut de pgx) : 300 postes ouverts = 300 connexions, même si personne ne saisit. Un délai de 1 à 2 min (`MaxConnIdleTime`) ramènerait les connexions au nombre de postes actifs, au prix d'une reconnexion (TLS et jeton, ≈ 10 à 30 ms en local) à la première lecture après un repos, et d'une rafale de reconnexions quand une saisie réveille un classeur suivi par tous. À décider avec S6.

### P1 [plus tard] Latence de propagation

La médiane (≈ 250 à 300 ms) est faite pour l'essentiel du regroupement de 200 ms (`collab.Debounce`) et de la lecture du delta par chaque poste. Lire le delta une fois dans lattisd et le pousser dans l'événement (jalon 8, « lit les deltas une fois ») diviserait par N les transactions par saisie (aujourd'hui ≈ 1 par poste). Estimation : 3 à 5 jours avec le serveur (contrat client § 11, droits par ligne à respecter par poste).

### P2 [plus tard] Essai de charge en intégration continue

`TestCharge` demande lattisd et une base OAuth : pas en CI. Une tâche manuelle (workflow déclenché à la main, compose du serveur) vaudrait une demi-journée.
