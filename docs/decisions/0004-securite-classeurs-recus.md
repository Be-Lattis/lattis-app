# Sécurité des classeurs reçus (point C2)

*18/10/2026. Phase 9, point 8 du plan (point C2 de la revue n° 3), prérequis de la collaboration (spécification de l'application collaborative, § 3 et § 15, étape 1). Code : `internal/store/authorizer.go`, `internal/trust`, `internal/doc/trust.go`, `internal/api/consent.go`, `internal/api/cmd_pages.go`.*

## En bref

Un classeur peut venir de quelqu'un d'autre, et bientôt d'un Serveur Lattis où un concepteur publie des scripts exécutés chez tous les membres. Principe : **rien de ce que contient le fichier ne s'accorde un droit**. Les accords de l'utilisateur sont rangés sur le poste, hors du classeur, et le SQL écrit par l'utilisateur ou rangé dans le classeur ne sort pas des tableaux.

| Sujet | Décision | Statut |
|---|---|---|
| SQL de l'utilisateur | Autorisateur SQLite (`sqlite3_set_authorizer`, `Conn.SetAuthorizer` de ncruces) posé **le temps de la requête** : connexion du pool de lecture réservée (`store.UserConn`) ou connexion d'écriture pendant l'instruction de l'utilisateur (`Store.AsUser`). Le moteur garde un accès complet hors de ces fenêtres | Fait |
| Ce qui est refusé | `ATTACH`/`DETACH` ; toute structure, temporaire comprise (un déclencheur temporaire resterait sur la connexion et s'exécuterait au nom du moteur) ; transactions et points de sauvegarde ; `PRAGMA` sauf ceux qui décrivent les tables ; `load_extension`, `fts3_tokenizer` ; tables `_lattis_*`, `_slt_*` et `sqlite_*` (sauf lecture de `sqlite_schema`) ; schéma `temp` en lecture, et en écriture hormis les vues aux noms affichés du moteur. Ce qui s'exécute dans un déclencheur ou une vue (`inner` de l'autorisateur) reste permis : ce sont ceux du moteur | Fait |
| Chemins concernés | Onglet SQL (lecture, écriture, résultat vers une feuille), `db.query`/`db.execute` des scripts, des pages HTML et du serveur MCP (`grid.QueryArgs`, `canvas.ExecSQL`), requêtes liées et source « ce classeur » des requêtes d'import (`canvas.ReadSQL`) | Fait |
| Accords hors du fichier | Paquet `internal/trust`, fichier `Lattis/accords.json` du dossier de configuration (`os.UserConfigDir`), comme les accords des mots de passe (`connexions-autorisees.json`). Sous `go test`, le magasin par défaut reste en mémoire | Fait |
| Réseau | Hôtes mémorisés par classeur (identifiant aléatoire `_lattis_info.id`, `SecretScope`) sur le poste ; `_lattis_http_hosts` n'est plus lu ni écrit. « Mémoriser » ne rend plus le classeur modifié | Fait |
| Reprise des anciens accords | Au premier lancement (fichier des accords absent), les chemins de la liste des récents sont notés ; rouvert, un de ces classeurs reprend une fois ses hôtes enregistrés dans le fichier et autorise ses scripts actuels. Les accords enregistrés dans tout autre classeur sont ignorés (classeur reçu : on redemande) | Fait |
| Accord des scripts | Par **version** (SHA-256 du source, fins de ligne normalisées), pour l'utilisateur, sur le poste. Autorisée d'office : un script enregistré sur ce poste (`doc.Document.SaveScript`, onglet SCRIPT, `scripts.save`) ou lancé depuis l'onglet SCRIPT (code sous les yeux). Sinon, première exécution d'un script de la bibliothèque (bouton, contrôle, évènement, onglet Document, `scripts.run` par nom) : « Autoriser / Refuser », « Voir le code », case « Autoriser aussi les autres scripts de ce classeur ». Refus retenu pour la session. MCP : refus sans question. Le code passé par `scripts.run` (`source`) n'est pas du classeur et n'est pas soumis à l'accord | Fait |
| Pages HTML | **Liste autorisée** (`pageAllowed`) au lieu d'une liste d'interdictions : lecture, valeurs annulables, boîtes de dialogue, réseau (accord par hôte), actions des boutons et contrôles (accord des scripts), actualisation des requêtes. Refusés : fichiers, annulation, code (scripts, pages, écouteurs, actions des boutons), structure, protections, sources de données | Fait |
| Trousseau | Déjà couvert : un mot de passe du trousseau ne sert à un classeur qu'avec l'accord de l'utilisateur pour ce classeur (`query.Secrets`, par `SecretScope`) | Vérifié |

## Pourquoi un accord par version de script plutôt que par classeur

Un accord « ce classeur est de confiance », attaché à l'identifiant du classeur, ne résiste pas à l'aller-retour : un classeur envoyé par Alice à Bob garde son identifiant ; Bob y ajoute un script et le renvoie, et le poste d'Alice le reconnaîtrait. Une empreinte du code, elle, change dès qu'une ligne change. Comme tout script écrit sur le poste est autorisé à son enregistrement, un classeur créé localement ne pose jamais de question ; seul un code venu d'ailleurs en pose une, une fois par version. La même mécanique servira aux scripts publiés d'un classeur partagé (« Script publié par Alice, version 3 ») : il suffira d'ajouter l'auteur et la version au message.

## Limites connues

- Les accords réseau restent attachés à l'identifiant du classeur : un classeur renvoyé (aller-retour) retrouve les hôtes autorisés pour lui. Les scripts qu'il contient demandent leur accord, mais une **page HTML** s'exécute à l'affichage de son onglet sans accord et peut joindre ces hôtes. Piste : un accord par version de page, comme pour les scripts, ou des accords réseau par empreinte.
- Un fichier piégé peut contenir des déclencheurs ou des vues dans le schéma principal (`CREATE TRIGGER` écrit hors de Lattis) : ils s'exécutent avec les écritures du moteur. L'autorisateur n'y change rien ; un contrôle à l'ouverture (refuser tout objet de schéma inconnu) reste à faire.
- Les classeurs créés avant cette protection et absents de la liste des récents demandent l'accord de chacun de leurs scripts une fois (ou de tous, avec la case).
