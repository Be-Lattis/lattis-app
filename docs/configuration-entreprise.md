# Configuration d'entreprise

Lattis lit sa configuration en cascade : pour chaque clé, la couche la plus prioritaire qui la définit l'emporte. Une DSI peut ainsi déployer les sources externes (et plus tard l'onglet IA) sur tout un parc, sans action de l'utilisateur.

## Ordre des couches

De la plus prioritaire à la moins prioritaire :

| Couche | Windows | macOS | Linux |
|---|---|---|---|
| Stratégie (verrouillée) | `HKLM\SOFTWARE\Policies\Lattis`, `%ProgramData%\Lattis\policy.json` | `/Library/Application Support/Lattis/policy.json` | `/etc/lattis/policy.json` |
| Ligne de commande | `--set chemin=valeur` | idem | idem |
| Environnement | `LATTIS_*` | idem | idem |
| Registre | `HKCU\SOFTWARE\Lattis`, puis `HKLM\SOFTWARE\Lattis` | | |
| Utilisateur | `%APPDATA%\Lattis\config.json` | `~/Library/Application Support/Lattis/config.json` | `~/.config/Lattis/config.json` |
| Machine | `%ProgramData%\Lattis\config.json` et `config.d\*.json` | `/Library/Application Support/Lattis/config.json` | `/etc/lattis/config.json` et `config.d/*.json` |
| Distante | document servi à `configUrl`, mis en cache | idem | idem |
| Défaut | compilé | | |

Une clé posée par une stratégie est verrouillée : l'environnement, le Registre et les fichiers ne peuvent pas la changer. Seule une stratégie qui pose `"allowPolicyOverride": true` laisse `LATTIS_ALLOW_POLICY_OVERRIDE=1` la faire passer sous l'environnement (tests, dépannage).

Les objets sont fusionnés clé par clé : une couche peut ne changer que `connections.erp.config.host`. `null` retire une clé posée plus bas. Une valeur invalide est ignorée pour cette clé seulement, et la couche suivante la fournit.

## Format

```json
{
  "version": 1,
  "organization": "ACME",
  "configUrl": "https://config.acme.com/lattis.jws",
  "remote": { "publicKey": "<clé publique Ed25519 en base64>", "refreshMinutes": 60 },
  "identity": {
    "providers": {
      "entra": { "type": "oidc", "flavor": "entra",
                 "issuer": "https://login.microsoftonline.com/<tenant>/v2.0",
                 "clientId": "<id>", "scopes": ["openid", "profile", "offline_access"] }
    }
  },
  "connections": {
    "erp": { "name": "ERP", "kind": "sqlserver",
             "config": { "host": "sql-erp.acme.local", "database": "ERP" },
             "auth": { "mode": "integrated" } }
  },
  "ai": { "enabled": false, "endpoint": "https://ia-gateway.acme.com/v1", "dataSharing": "schema-only" },
  "features": { "mcp": { "enabled": false }, "allowUserConnections": true,
                "luaHttp": { "allowedDomains": ["api.acme.com"] } }
}
```

**Aucun secret** : `password`, `secret`, `clientSecret`, `token`, `apiKey` sont refusés partout. Les mots de passe et jetons de chaque utilisateur restent dans le trousseau du système. Seule exception : `identity.providers.*.desktopClientSecret`, que Google exige pour une application de bureau sans le tenir pour confidentiel.

`configUrl` et `remote.*` sont réservés aux couches posées par un administrateur (stratégie, `HKLM`, fichier machine protégé) dès que l'une d'elles les définit. Sur un poste sans configuration d'entreprise, l'environnement peut les poser (tests).

### Registre et environnement

- Registre : le nom de chaque valeur est le chemin pointé (`connections.erp.config.host`) ; une sous-clé ajoute son nom au chemin. `REG_SZ` est lu comme du JSON quand c'en est (`true`, `12`, `{...}`), comme du texte sinon ; `REG_DWORD` 0/1 vaut faux/vrai pour un booléen ; `REG_MULTI_SZ` donne une liste. La valeur `ConfigFile` désigne un fichier JSON lu au même rang.
- Environnement : `LATTIS_CONNECTIONS__ERP__CONFIG__HOST=sql-staging` (`__` sépare les niveaux), `LATTIS_CONFIG=chemin.json`, `LATTIS_CONFIG_JSON='{...}'`, et les raccourcis `LATTIS_CONFIG_URL`, `LATTIS_AI_ENDPOINT`.

## Configuration distante

Lattis n'attend jamais le réseau au démarrage : il lit la dernière configuration distante en cache (`UserCacheDir/Lattis/remote-config.json`), puis la rafraîchit en arrière-plan (avec `ETag`) toutes les `remote.refreshMinutes` minutes. Un document invalide ne remplace jamais le cache, et un poste hors ligne démarre sur la dernière version valide.

Quand `remote.publicKey` est posée, le document doit être signé (JWS compact, EdDSA). Un document signé avec une clé posée par un administrateur peut porter une stratégie dans sa clé `policy` ; un document distant ne peut pas changer `configUrl` ni `remote`.

```sh
lattis config keygen cle-privee.pem          # affiche remote.publicKey
lattis config sign cle-privee.pem lattis.json > lattis.jws
```

## Déploiement Windows (MSIX + Intune)

Un paquet MSIX ne peut ni écrire dans `%ProgramData%` ni recevoir de paramètres à l'installation. La voie la plus simple est une seule valeur de stratégie, posée par Intune ou une GPO :

```
HKLM\SOFTWARE\Policies\Lattis
    configUrl         REG_SZ  https://config.acme.com/lattis.jws
    remote.publicKey  REG_SZ  <clé publique>
```

Tout le reste se gère dans le document distant. Sans serveur, un script Intune peut déposer `%ProgramData%\Lattis\config.json`. Le dossier `%ProgramData%\Lattis` doit alors être en lecture seule pour les utilisateurs (par défaut, `%ProgramData%` leur permet d'y créer des fichiers) : Lattis vérifie ces droits, ignore un `policy.json` modifiable par tous et ne tient pas pour fiable un `config.json` modifiable par tous.

## Diagnostic

```sh
lattis config                 # configuration effective, provenance, 🔒 = imposée
lattis config --json
lattis config --set ai.enabled=true
lattis config check fichier.json
lattis config refresh         # télécharge la configuration distante
```
