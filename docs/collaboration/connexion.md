# Connexion à un Serveur Lattis

*Étape 2 du découpage de la spécification de l'application collaborative (§4 et §13). Ce document décrit ce que fait le poste de bureau et, surtout, son alignement sur le contrat client de `lattisd` et les écarts qui restent. Chaque choix est rassemblé dans une constante ou un seul endroit du code, pour s'ajuster facilement au serveur réel.*

## Ce qui est fait

| Élément | Où |
|---|---|
| Fonctionnalité à activer : Fichier › Paramètres › Fonctionnalités › « Classeurs partagés (Serveur Lattis) » (`features.sharedWorkbooks`, coupée par défaut). Coupée, tout est masqué (menu Compte, source « Serveur Lattis », commandes) et le pont refuse les appels. | `internal/config`, `frontend/src/lib/shell/settings.ts` |
| Connexion de genre `lattis` (un seul champ : l'adresse du serveur) ; le nom affiché lu à l'ajout (`config.label`) remplace « PostgreSQL » dans les listes. Utilisable comme source de requête en lecture seule (sur un réplica quand il y en a). | `internal/query/lattis.go`, `ConnectionDialog.svelte`, `sources.ts` |
| Découverte `GET <url>/.well-known/lattis.json`, refus explicite si l'application est plus ancienne que `min_client_version`. | `internal/remote/auth/discovery.go` |
| Page de connexion : un bouton par fournisseur, dans l'ordre du serveur, avec son icône ; le fournisseur `password` aussi passe par le navigateur (la page de connexion de `lattisd`, décision de Louis du 18/10/2026 : la fenêtre de Lattis ne voit jamais le mot de passe) ; messages clairs pour les refus. | `frontend/src/lib/remote/LoginDialog.svelte` |
| OAuth 2.0 code d'autorisation + PKCE (S256), navigateur du système, redirection en boucle locale `http://127.0.0.1:<port>/callback` (RFC 8252) ; « device code » (RFC 8628) en secours. | `internal/remote/auth/oauth.go`, `loopback.go`, `manager.go` |
| Jeton de rafraîchissement au trousseau du système (`internal/query/secrets.go`, compte `lattis://<hôte>/<utilisateur>`, sans accord par classeur : la session est celle du poste) ; jeton d'accès en mémoire, renouvelé 60 s avant son expiration et à la première connexion après redémarrage ; rotation prise en compte ; refus du renouvellement = session fermée. | `internal/remote/auth/manager.go`, `internal/query/lattis.go` (`LattisTokens`) |
| Menu Compte (barre de titre) : utilisateur connecté par serveur, connexion, déconnexion (révocation au mieux, oubli au trousseau). | `frontend/src/lib/remote/AccountMenu.svelte` |
| Index des serveurs connus, sans secret : `UserConfigDir/Lattis/serveurs-lattis.json` (adresse, nom, logo, utilisateur), pour l'affichage hors ligne. | `internal/remote/auth/manager.go` |
| PostgreSQL en `OAUTHBEARER` : **pgx 5.11 le prend en charge** (`pgconn.Config.OAuthTokenProvider`, réponse initiale `n,,^Aauth=Bearer <jeton>^A^A` de la RFC 7628) ; rien à ajouter au pilote. Le fournisseur de jeton est appelé à chaque nouvelle connexion physique. Pool `rw`, pool `ro` quand le serveur annonce des réplicas, `application_name = Lattis/<version>/<usage>` (posé au démarrage puis par `SET` quand l'usage change ; tronqué à 63 octets). | `internal/remote/pgpool` |
| Stratégie d'entreprise : serveurs gérés (`connections.<id>` de genre `lattis`), verrouillage (`features.allowUserLattisServers: false`), fournisseur imposé (`connections.<id>.auth.provider`). | `internal/config/lattis.go`, `auth.Policy` |

Exemple de stratégie :

```json
{
  "features": { "sharedWorkbooks": true, "allowUserLattisServers": false },
  "connections": {
    "acme": { "kind": "lattis", "name": "Serveur Acme",
              "config": { "url": "https://lattis.acme.fr" },
              "auth": { "provider": "acme-keycloak" } }
  }
}
```

## Alignement sur le contrat de lattisd

Le contrat client de `lattisd` (`be-lattis-server`, branche `claude/socle-lattisd`, `docs/contrat-client.md`) fixe les échanges ; l'implémentation le suit. Ce qui en est repris :

- **Découverte** : `name`, `logo`, `version`, `min_client_version`, `edition`, `access_mode`, `issuer`, `client_id`, `scope`, `authorization_endpoint`, `token_endpoint`, `revocation_endpoint`, `providers[]` (`id`, `kind`, `label`, `icon`), `postgres` (`database`, `sslmode`, `rw: {host, port}`, `ro: [{host, port}]`). Les points OAuth annoncés sont utilisés tels quels, à condition d'être sur le **même schéma et le même hôte** que le serveur saisi (sinon la découverte est refusée : le navigateur et les jetons ne vont qu'au serveur choisi). Sans eux, les chemins de la spécification (§11) servent de valeurs par défaut (`auth.Path*`).
- **`icon`** reprend le `kind` (« password ») : ce n'est pas une image, le client prend son icône du genre. Une image (adresse relative au serveur, absolue sur le même hôte, ou URI `data:`, 256 Kio au plus) reste acceptée, et le logo est ramené à une URI `data:`.
- **Autorisation** : `response_type=code`, `client_id`, `redirect_uri=http://127.0.0.1:<port>/callback`, `scope` (celui de la découverte, `openid lattis` par défaut), `state`, `code_challenge` S256, `provider=<id>`. Au retour, **`iss` est comparé à `issuer`** (RFC 9207) ; un émetteur différent est refusé.
- **Jetons** : `pg_role` est lu au premier niveau de la réponse de `/oauth/token` (puis, à défaut, dans un objet `user` facultatif ou les revendications du jeton). Rotation du jeton de rafraîchissement à chaque usage, le nouveau remplace l'ancien au trousseau ; on ne présente jamais un ancien jeton (le jeton en mémoire et celui du trousseau sont mis à jour ensemble, sous verrou de session).
- **Déconnexion** : `POST revocation_endpoint` avec `token=<refresh_token>`.
- **PostgreSQL** : `OAUTHBEARER` par `cfg.OAuthTokenProvider` de pgx 5.11 ; **le rôle (`pg_role`, souvent avec « @ ») est posé dans `cfg.User`**, jamais dans la chaîne de connexion ; `sslmode` et points `rw`/`ro` de la découverte (`verify-full` par défaut) ; `application_name = Lattis/<version>/<usage>`. Un refus d'authentification de PostgreSQL (28000/28P01) fait oublier le jeton d'accès, qui est rafraîchi, puis **un seul** nouvel essai (`bridge.lattisOpen`).
- Un serveur par instance PostgreSQL : une connexion `lattis` = un serveur = une base.

## Écarts restants avec le contrat

| Point | Côté application | À trancher |
|---|---|---|
| Device code (secours) | `POST device_authorization_endpoint` (par défaut `/oauth/device`, §11 de la spécification), RFC 8628. | Le contrat ne l'annonce pas encore : à ajouter à la découverte (`device_authorization_endpoint`) et au serveur. |
| Codes de refus | `error=access_denied` avec `reason` ∈ `denied_tenant`, `not_invited`, `account_disabled` (alias `tenant_not_allowed`, `invitation_required`, `user_disabled`) au retour ou dans le corps JSON. | Le contrat ne fixe pas la forme du refus : confirmer le paramètre `reason` (aligné sur les étiquettes de `lattis_auth_logins_total`). |
| `client_id` | Celui de la découverte, `lattis-desktop` par défaut. | — |
| Révocation | `token_type_hint=refresh_token` et `client_id` sont ajoutés (RFC 7009), sans effet si le serveur les ignore. | — |
| `nonce` | Non envoyé (facultatif ; le jeton d'identité n'est pas vérifié par le client, qui ne s'en sert que pour l'affichage). | — |
| Signature des jetons | EdDSA, vérifiée par PostgreSQL (`lattis_oauth`) ; le client ne lit les revendications que pour l'affichage, sans vérifier. | — |
| Version minimale | Versions `x.y.z` (préfixe `v`, pré-version `-rc.1`). Une version de développement (`dev`) n'est jamais refusée ; `bridge.Version` n'est pas encore injectée au build. | Injecter la version (`-ldflags -X`). |
| Adresse | `https://` obligatoire ; `http://` seulement pour `localhost` / `127.0.0.1` / `[::1]` (essais). | — |

### Refus et messages

| `reason` (ou `error`) | Message |
|---|---|
| `denied_tenant` (`tenant_not_allowed`, `tenant_denied`) | compte hors d'une organisation autorisée |
| `not_invited` (`invitation_required`) | serveur sur invitation |
| `account_disabled` (`user_disabled`, `disabled`) | compte désactivé |
| `invalid_grant` (rafraîchissement) | session expirée : reconnexion demandée |

## Ce qui reste

- Côté serveur : trancher les écarts ci-dessus (device code, forme des refus).
- Pool `rw` / `ro` (`pgpool.Open`, `Acquire`) : utilisé par la publication, l'ouverture et l'écoute des republications (`publication.md`, `internal/bridge/sharing.go`) ; la synchronisation (étape 5, `synchronisation.md`) reprend la connexion `LISTEN` dédiée, avec reprise des deltas à chaque reconnexion. Réplicas (contrat §3.1, branche `claude/jalon-6-replicas`) : une lecture servie par `ro` qui reçoit un `42501` inattendu (réplica qui rejoue la création d'un classeur avec un cache des rôles périmé) est refaite sur une nouvelle connexion, la fautive étant fermée pour que le pool ne la redonne pas, puis sur `rw` (`workbook.pgServer.with`, `TestPostgresReplicaRetry`) ; les deltas restent sur `rw`. `SELECT lattis.login_summary()` (contrat §4) n'est pas encore appelé.
- Réutiliser ce module pour le module OIDC de la configuration d'entreprise (étape 6 de `configuration-entreprise.md`), avec `lattisd` comme émetteur unique.
- Version web : la connexion passera par le cookie de session de `lattisd` ; le navigateur du système et la boucle locale n'y ont pas cours.
- Essais d'intégration contre un vrai `lattisd` + PostgreSQL 18 (`LATTIS_PG_TEST=1`).
