# Publier une version

Un tag `v*` poussé sur GitHub lance le workflow **Release** (`.github/workflows/release.yml`) : il construit les installeurs Windows, macOS et Linux, les signe quand les secrets sont configurés (voir [signature.md](signature.md)), puis publie la release GitHub (publique, ni brouillon ni pré-version pour `vX.Y.Z`) avec les fichiers, leurs empreintes, `latest.json` et les notes de version. Sans aucun secret, tout est produit et publié quand même, non signé.

Avant de taguer, relire la [liste de contrôle](checklist.md).

## Créer le tag

```bash
scripts/release.sh v1.2.0
```

Le script vérifie qu'on est sur `main`, à jour avec `origin/main`, sans fichier suivi modifié et que le tag n'existe pas ; il lance les tests Go (`./internal/...`, `./engine/...`, `./protocol/...`), crée le tag annoté, affiche les notes de version puis pousse le tag. Options : `--skip-tests`, `--allow-dirty` (tague malgré des modifications locales, qui ne sont pas dans la version).

- `v1.2.0` : version publiée, marquée « Latest ».
- `v1.2.0-beta.1` (tout tag avec un tiret) : pré-version, jamais « Latest » (les liens `releases/latest` ci-dessous l'ignorent).

Si un job échoue, « Re-run failed jobs » sur le run suffit : une release déjà créée pour ce tag voit ses fichiers remplacés.

## Fichiers publiés et liens de téléchargement

Les noms ne contiennent pas la version : le lien `releases/latest/download/<nom>` mène toujours au fichier de la dernière version publiée (GitHub ignore les brouillons et les pré-versions).

| Système | Fichier | Lien direct (dernière version) |
|---|---|---|
| Windows 10/11, installeur | `Lattis-windows-x64-setup.exe` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-windows-x64-setup.exe> |
| Windows 10/11, portable | `Lattis-windows-x64-portable.exe` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-windows-x64-portable.exe> |
| Windows 10/11, MSIX | `Lattis-windows-x64.msix` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-windows-x64.msix> |
| macOS (Apple Silicon et Intel) | `Lattis-macos-universal.dmg` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-macos-universal.dmg> |
| Linux, AppImage | `Lattis-linux-x64.AppImage` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-linux-x64.AppImage> |
| Linux, Debian/Ubuntu | `Lattis-linux-x64.deb` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-linux-x64.deb> |
| Linux, Fedora/RHEL | `Lattis-linux-x64.rpm` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-linux-x64.rpm> |
| Empreintes | `SHA256SUMS.txt` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/SHA256SUMS.txt> |
| Manifeste | `latest.json` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/latest.json> |

**Version portable (Windows).** `Lattis-windows-x64-portable.exe` est l'exécutable seul, lancé sans installation (clé USB, poste sans droits d'administration). Les liens `lattis://` marchent : l'application les déclare pour l'utilisateur au premier lancement, si aucune version installée ne le fait déjà. Ne sont pas faits : l'association des fichiers `.lattis` (double-clic), le raccourci du menu Démarrer et l'entrée dans Ajout/Suppression de programmes. Les réglages, classeurs récents et caches restent dans le profil de l'utilisateur (`%APPDATA%`, `%LOCALAPPDATA%`), communs avec une version installée. Il faut WebView2, présent sur Windows 10 à jour et Windows 11.

Le MSIX n'est publié que **signé** : Windows refuse d'installer un MSIX non signé par un certificat de confiance. Sans certificat Windows, il manque à la release (il reste dans l'artefact `lattis-windows-msix-non-signe` du run, pour essai) et son lien répond 404 ; l'installeur `.exe` est toujours là. Sans certificat Apple, le `.dmg` est signé ad hoc : macOS demande alors une ouverture manuelle (clic droit › Ouvrir, ou Réglages › Confidentialité et sécurité).

Avec la clé GPG (voir [signature.md](signature.md)), chaque fichier `Lattis-*` et `SHA256SUMS.txt` ont aussi une signature `<nom>.asc`, et la clé publique est publiée (`lattis-release-key.asc`).

Le site peut aussi lire l'API GitHub (`https://api.github.com/repos/Be-Lattis/lattis-app/releases/latest`, champ `assets[].browser_download_url`) et choisir le fichier par son extension.

## latest.json

Produit par `scripts/release-manifest.sh` (utilisable en local : `scripts/release-manifest.sh v1.2.0 dossier`). Seuls les fichiers présents dans la release y figurent (pas de `windows-msix` sans certificat). Les adresses désignent la version précise (`releases/download/<tag>/<nom>`), pas `latest`.

```json
{
  "version": "1.2.0",
  "tag": "v1.2.0",
  "prerelease": false,
  "date": "2026-10-18T12:00:00Z",
  "release_url": "https://github.com/Be-Lattis/lattis-app/releases/tag/v1.2.0",
  "files": {
    "windows-setup": {
      "name": "Lattis-windows-x64-setup.exe",
      "url": "https://github.com/Be-Lattis/lattis-app/releases/download/v1.2.0/Lattis-windows-x64-setup.exe",
      "sha256": "…64 caractères hexadécimaux…",
      "size": 12345678
    },
    "windows-msix":   { "name": "Lattis-windows-x64.msix", "url": "…", "sha256": "…", "size": 0 },
    "macos-dmg":      { "name": "Lattis-macos-universal.dmg", "url": "…", "sha256": "…", "size": 0 },
    "linux-appimage": { "name": "Lattis-linux-x64.AppImage", "url": "…", "sha256": "…", "size": 0 },
    "linux-deb":      { "name": "Lattis-linux-x64.deb", "url": "…", "sha256": "…", "size": 0 },
    "linux-rpm":      { "name": "Lattis-linux-x64.rpm", "url": "…", "sha256": "…", "size": 0 }
  }
}
```

`date` : moment de la construction (UTC, ISO 8601) ; `size` : octets. Attention : la redirection `releases/latest/download/latest.json` ne renvoie pas d'en-têtes CORS, un navigateur ne peut donc pas la lire directement (un serveur, une fonction de build du site ou l'API GitHub le peuvent).

## Notes de version

`scripts/release-notes.sh v1.2.0` les produit (aussi en local, pour relire avant de taguer) : la section `## 1.2.0 (JJ/MM/AAAA)` de `CHANGELOG.md` telle qu'elle est au tag, puis le tableau des fichiers à télécharger. Avant de taguer, renommer « Non publié » en `## 1.2.0 (JJ/MM/AAAA)` dans `CHANGELOG.md` et rouvrir un « Non publié » vide (la façon de tenir le journal est rappelée en tête du fichier). Sans section pour cette version, les notes reprennent la liste des commits depuis le tag précédent, sans les fusions ni les régénérations de liaisons ; la première version dit alors simplement « Première version publique de Lattis ».

## Brouillon ou publication directe

La release est publiée directement. Pour la relire avant publication, créer la variable de dépôt `RELEASE_DRAFT` à `true` (Settings › Secrets and variables › Actions › Variables) : la release reste alors en brouillon, à publier depuis GitHub ; tant qu'elle est en brouillon, les liens `releases/latest` désignent encore la version précédente.

## Essai sans release

Actions › Release › Run workflow, avec une version d'essai (`v0.0.0-test`) : les paquets sont construits, puis `SHA256SUMS.txt`, `latest.json` et les notes sont préparés et déposés dans l'artefact `release-essai` du run, sans créer de release ni de tag.
