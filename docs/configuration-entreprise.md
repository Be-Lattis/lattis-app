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
                "luaHttp": { "allowedDomains": ["api.acme.com"] } },
  "limits": { "undoDepth": 200, "scriptSeconds": 30, "sqlReadSeconds": 30, "sqlWriteSeconds": 120 },
  "updates": { "check": true }
}
```

`limits` borne ce qui peut tenir l'application ou la mémoire ; chaque clé est un entier, une valeur hors bornes est ignorée. L'utilisateur les règle aussi dans Fichier › Paramètres › Limites, sauf si une stratégie les impose. Un changement vaut tout de suite.

| Clé | Rôle | Défaut | Bornes |
|---|---|---:|---|
| `limits.undoDepth` | commandes que `Ctrl+Z` peut annuler | 200 | 10 à 10 000 |
| `limits.scriptSeconds` | durée maximale d'un script Lua, en secondes | 30 | 1 à 3 600 |
| `limits.sqlReadSeconds` | durée maximale d'une lecture de l'onglet SQL ou d'un script | 30 | 1 à 3 600 |
| `limits.sqlWriteSeconds` | durée maximale d'une écriture de l'onglet SQL (annulée au-delà) | 120 | 1 à 3 600 |

### Thèmes de couleur

Le thème choisi dans Affichage › Thème est enregistré avec le classeur. La configuration donne le thème des classeurs qui n'en ont pas choisi, et la liste proposée (décision 0017) :

| Clé | Rôle | Défaut |
|---|---|---|
| `ui.theme` | identifiant du thème par défaut : un prédéfini (`vert-bleu`, `bleu-orange`, `violet-sarcelle`, `sarcelle-violet`, `gris-bleu`, `orange-bleu`, `bordeaux-gris`) ou une clé de `ui.themes` | `vert-bleu` |
| `ui.themes.<id>` | thème proposé : `name`, et deux couleurs `#rrggbb` : `table` (tableaux ordinaires) et `source` (tableaux d'une source externe) ; les nuances s'en déduisent. En option, les autres couleurs des palettes de police et de remplissage : `accents` (Accent 3 à 6, quatre `#rrggbb` au plus), `dark` (Texte 2) et `light` (Arrière-plan 2) ; absentes, elles se déduisent des deux bases | aucun |
| `ui.builtinThemes` | proposer les thèmes prédéfinis ; `false` : seulement ceux de `ui.themes` | `true` |

```json
{
  "ui": {
    "builtinThemes": false,
    "theme": "acme",
    "themes": {
      "acme":       { "name": "ACME",            "table": "#0b5394", "source": "#e69138" },
      "acme-sobre": { "name": "ACME (sobre)",     "table": "#475569", "source": "#0b5394" }
    }
  }
}
```

Les thèmes de l'organisation s'ajoutent à ceux que l'utilisateur garde dans son `config.json` (« Ajouter à mes thèmes ») ; il ne peut pas les retirer. Posé dans la stratégie, `ui.theme` est imposé (Fichier › Paramètres › Affichage le montre verrouillé) ; un classeur qui a choisi son thème le garde. Un thème invalide (nom vide, couleur qui n'est pas `#rrggbb`) est ignoré. Les couleurs du thème appliqué sont exposées dans l'interface en variables CSS : `--lattis-table`, `--lattis-table-head`, `--lattis-table-text`, `--lattis-table-border`, `--lattis-table-icon`, `--lattis-table-badge`, `--lattis-table-strong`, et les mêmes en `--lattis-source…`.

`ai.dataSharing` vaut `schema-only` (structure seulement, par défaut), `sample` (quelques lignes d'exemple par lecture) ou `full` (tout le classeur). Il s'applique à l'assistant intégré comme à Claude Desktop ou Claude Code par le serveur MCP.

**Aucun secret** : `password`, `secret`, `clientSecret`, `token`, `apiKey` sont refusés partout. Les mots de passe et jetons de chaque utilisateur restent dans le trousseau du système (Gestionnaire d'identification Windows, Trousseau macOS, Secret Service sous Linux : gnome-keyring, KWallet). Sur un poste qui n'en a pas, ou dont le trousseau ne répond pas en quelques secondes (session kiosque, conteneur, invite de déverrouillage impossible à afficher), Lattis les garde en mémoire jusqu'à sa fermeture, jamais dans un fichier, et le dit une fois dans la barre d'état : l'utilisateur se reconnecte à chaque lancement. Seule exception : `identity.providers.*.desktopClientSecret`, que Google exige pour une application de bureau sans le tenir pour confidentiel.

`configUrl` et `remote.*` sont réservés aux couches posées par un administrateur (stratégie, `HKLM`, fichier machine protégé) dès que l'une d'elles les définit. Sur un poste sans configuration d'entreprise, l'environnement peut les poser (tests).

### Langue

| Clé | Rôle | Défaut |
|---|---|---|
| `ui.userName` | nom affiché comme auteur des commentaires et des versions d'un classeur local (80 caractères au plus) ; vide : le nom de l'utilisateur du système | vide |
| `ui.language` | langue de l'interface : `auto` (celle du système, ou du navigateur en version web ; l'anglais si elle n'est pas proposée), `fr`, `en`, `de`, `es` ou `zh` (chinois simplifié) | `auto` |

L'utilisateur la choisit dans Fichier › Paramètres › Affichage ; posée dans la stratégie, elle est imposée. Elle règle aussi l'affichage des nombres et des dates, et la lecture des nombres écrits en texte par les fonctions (« 1,234 ») ; l'écriture des formules la suivra (séparateurs `;` et décimale `,` en français, allemand et espagnol ; `,` et `.` en anglais et en chinois). Le classeur enregistré n'en dépend pas (docs/i18n.md). Les traductions sont en cours : l'interface reste en français pour les textes pas encore traduits.

### Mises à jour

| Clé | Rôle | Défaut |
|---|---|---|
| `updates.check` | l'application de bureau cherche les nouvelles versions sur GitHub et propose de s'y mettre à jour (barre d'état) | `true` |

une recherche est un `GET` de l'API GitHub, dès que l'application est chargée puis au plus toutes les 12 heures, sans cookie ni identifiant (`User-Agent: Lattis/<version> (<système>)`) ; rien n'est téléchargé sans le clic de l'utilisateur, et rien n'est installé sans empreinte publiée (détail : [release/publier.md](release/publier.md#mise-à-jour-automatique)). L'utilisateur la coupe dans Fichier › Paramètres › Mises à jour ; un parc qui distribue lui-même les versions (Intune, winget, MSIX) pose `"updates": { "check": false }` dans la stratégie (`HKLM\SOFTWARE\Policies\Lattis`, valeur `updates.check` en `REG_DWORD` 0, ou `policy.json`) : plus aucune requête ni proposition, le réglage est grisé. Variable : `LATTIS_UPDATES__CHECK=0`. Sans objet en édition web ; un paquet MSIX ne cherche jamais (Windows le met à jour). L'installeur `Lattis-windows-x64-setup.exe` installe au choix pour l'utilisateur (sans droits d'administration) ou pour tous les utilisateurs du poste (Program Files, droits d'administration) ; en déploiement : `Lattis-windows-x64-setup.exe /S /AllUsers` ou `/S /CurrentUser` (options : [release/publier.md](release/publier.md#fichiers-publiés-et-liens-de-téléchargement)). Une installation pour tous les utilisateurs demande les droits d'administration à chaque mise à jour (invite UAC), un poste sans ces droits ne peut donc pas l'appliquer : couper la recherche (et déployer les versions soi-même), ou préférer l'installation pour l'utilisateur ou le MSIX.

### Pages web

| Clé | Rôle | Défaut |
|---|---|---|
| `features.htmlPages` | onglets « page web » des classeurs (HTML, CSS et JavaScript dans un cadre, ou site externe) : affichage, ajout, modification, scripts et assistant qui les créent | `false` |

Les pages web sont coupées par défaut et n'ont pas de case dans Fichier › Paramètres : le JavaScript d'une page vient avec le classeur et, même dans son cadre isolé, peut faire sortir des données en faisant naviguer ce cadre vers une adresse de son choix, ce qu'aucune règle ne bloque. Ne les activez que pour des classeurs de confiance. Coupées, un onglet page web existant reste dans le fichier mais l'onglet n'affiche que « Les pages web sont désactivées » (on peut le supprimer) ; `pages.add` et `pages.set` refusent une page web (les Documents restent permis), l'assistant et le serveur MCP n'en proposent pas.

Pour les activer sur un poste : `"features": { "htmlPages": true }` dans le `config.json` de l'utilisateur, ou `LATTIS_FEATURES__HTMLPAGES=true` ; pour un parc : la stratégie (`HKLM\SOFTWARE\Policies\Lattis`, valeur `features.htmlPages` en `REG_DWORD` 1, ou `policy.json`), qui peut aussi les imposer coupées (verrouillé, l'utilisateur ne les active plus). La valeur est lue au démarrage. La version web (navigateur) et la démo publique les refusent toujours, quel que soit ce réglage. L'ancienne clé `features.htmlTabs` (case des paramètres jusqu'à la 0.3.4-beta) est tolérée mais n'a plus d'effet.

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
