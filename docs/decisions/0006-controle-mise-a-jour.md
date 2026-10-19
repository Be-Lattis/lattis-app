# Contrôle de mise à jour et canal bêta (audit 6, Id4)

*19/10/2026. Proposition, **non implémentée**. Elle complète I14 (« Prêt à publier ? ») : `latest.json` est publié par le workflow de version (`docs/release/publier.md`) mais l'application ne le lit pas.*

## Pourquoi ce n'est pas fait tout de suite

- Aucune version n'est encore publiée (pas d'étiquette `v*`) et l'application se compile avec `Version = "dev"` : le contrôle ne peut pas être éprouvé de bout en bout.
- C'est une sortie réseau au démarrage. Elle doit être un choix explicite (désactivée par défaut), pilotable par la DSI, et décrite dans l'aide avant d'exister. Mieux vaut la livrer avec la première version publique, une fois le workflow de version éprouvé (`v0.1.0-beta.1`).

## Ce que voit l'utilisateur

- Fichier › Paramètres › Mises à jour : « Vérifier les mises à jour » (décoché par défaut) et « Recevoir les versions bêta ». La première fois que l'accueil s'affiche après l'installation, une ligne discrète propose de l'activer ; elle ne revient plus une fois répondue.
- Une nouvelle version : un message dans la barre d'état (« Lattis 1.3.0 est disponible ») et, dans À propos, « Télécharger » qui ouvre la page de la version dans le navigateur. Rien ne se télécharge ni ne s'installe tout seul ; « Ignorer cette version » la fait taire.
- Jamais de fenêtre au démarrage, jamais pendant une saisie.

## Réglages

| Clé | Rôle | Défaut |
|---|---|---|
| `updates.check` | contrôle actif | `false` |
| `updates.channel` | `stable` ou `beta` | `stable` |
| `updates.url` | manifeste d'un miroir interne (couche administrateur seulement, comme `configUrl`) | GitHub |

Une stratégie `updates.check = false` coupe tout et masque le réglage (parc géré par Intune ou winget, où la DSI pousse elle-même les versions).

## Fonctionnement

- **Quand** : 30 s après le démarrage, puis une fois par jour au plus, en arrière-plan ; délai de 5 s ; un échec (hors ligne, proxy) est silencieux et noté dans le journal de diagnostic.
- **Quoi** : un simple `GET` sans cookie ni identifiant ; l'en-tête `User-Agent` dit seulement `Lattis/<version> (<système>)`. Canal stable : `https://github.com/Be-Lattis/lattis-app/releases/latest/download/latest.json` (la redirection `latest` ignore les pré-versions). Canal bêta : l'API `https://api.github.com/repos/Be-Lattis/lattis-app/releases?per_page=10` (60 appels par heure sans jeton, largement assez), dont on prend la plus récente, pré-version comprise, puis son `latest.json`.
- **Comparaison** : versions sémantiques (`1.2.0` > `1.2.0-beta.3` > `1.2.0-beta.1`) ; une compilation `dev` ne contrôle jamais.
- **Mémoire** : dernier contrôle, dernière version vue et version ignorée dans un petit fichier du dossier de configuration de l'utilisateur, hors de `config.json`.
- **Intégrité** : l'installeur se télécharge dans le navigateur et se vérifie par sa signature (Authenticode, notarisation Apple, GPG des paquets Linux, voir `docs/release/signature.md`). Les empreintes `sha256` de `latest.json` serviront le jour où Lattis téléchargera lui-même la mise à jour.
- **Version web** : sans objet (la version est celle du serveur).

## Canal bêta côté publication

Rien à changer au workflow : un tag avec un tiret (`v1.3.0-beta.1`) est déjà publié en pré-version, avec son `latest.json` (`"prerelease": true`). Il suffira de documenter le canal dans `docs/release/publier.md` et de publier les bêtas au rythme des fusions sur `main`.

## Mise en œuvre prévue

- `internal/update` : lecture du manifeste (`Check(ctx, channel, current) (Release, bool, error)`), comparaison des versions, choix de la plus récente pré-version ; tests sur des manifestes et des listes de l'API enregistrés, sans réseau.
- `internal/config` : clés `updates.*` (booléens, `oneOf("stable", "beta")`, `updates.url` réservée aux couches administrateur).
- `internal/bridge` : tâche de fond démarrée par `ServiceStartup` si le réglage est actif ; évènement vers l'interface ; `DocumentService.UpdateState()` pour À propos.
- Interface : section « Mises à jour » des paramètres (`settings.ts`), ligne de la barre d'état, bouton d'À propos.
- Aide : page Paramètres, et ce qui est envoyé.
