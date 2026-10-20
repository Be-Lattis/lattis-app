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
| Windows 10/11, installeur pour l'utilisateur | `Lattis-windows-x64-setup.exe` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-windows-x64-setup.exe> |
| Windows 10/11, installeur pour la machine | `Lattis-windows-x64-setup-machine.exe` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-windows-x64-setup-machine.exe> |
| Windows 10/11, portable | `Lattis-windows-x64-portable.exe` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-windows-x64-portable.exe> |
| Windows 10/11, MSIX | `Lattis-windows-x64.msix` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-windows-x64.msix> |
| macOS (Apple Silicon et Intel) | `Lattis-macos-universal.dmg` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-macos-universal.dmg> |
| Linux, AppImage | `Lattis-linux-x64.AppImage` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-linux-x64.AppImage> |
| Linux, Debian/Ubuntu | `Lattis-linux-x64.deb` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-linux-x64.deb> |
| Linux, Fedora/RHEL | `Lattis-linux-x64.rpm` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/Lattis-linux-x64.rpm> |
| Empreintes | `SHA256SUMS.txt` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/SHA256SUMS.txt> |
| Manifeste | `latest.json` | <https://github.com/Be-Lattis/lattis-app/releases/latest/download/latest.json> |

**Deux installeurs (Windows).** `Lattis-windows-x64-setup.exe` installe Lattis **pour l'utilisateur** : dans `%LOCALAPPDATA%\Programs\Lattis`, clé de désinstallation sous `HKCU`, raccourcis et associations de l'utilisateur, sans droits d'administration ; c'est celui des postes sans droits d'administration, et celui qui se met à jour sans invite de Windows. `Lattis-windows-x64-setup-machine.exe` l'installe **pour la machine** : dans `Program Files\Lattis\Lattis`, clé sous `HKLM`, pour tous les comptes, avec les droits d'administration (sa mise à jour automatique passe par l'invite UAC). Les deux viennent du même `build/windows/nsis/project.nsi` (`wails3 task windows:package:nsis INSTALL_SCOPE=user`, puis `INSTALL_SCOPE=machine` : `bin/lattis-amd64-user-installer.exe` et `bin/lattis-amd64-installer.exe`). Ils peuvent coexister, mais chacun prévient s'il trouve l'autre déjà installé (sauf en silencieux, `/S`) ; pour l'utilisateur, la copie par utilisateur l'emporte (ses associations `HKCU` passent devant celles de `HKLM`). Les réglages, classeurs récents et caches sont dans le profil (`%APPDATA%\Lattis`, `%LOCALAPPDATA%\Lattis`), communs aux deux installations et à la version portable. Options de la ligne de commande, en plus de celles de NSIS (`/S`, `/D=dossier`, en dernier) : `/UPDATE` (ne recrée pas le raccourci du Bureau) et `/RELAUNCH` (rouvre Lattis après l'installation, sans les droits d'administration), utilisées par la mise à jour automatique.

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
    "windows-setup-machine": { "name": "Lattis-windows-x64-setup-machine.exe", "url": "…", "sha256": "…", "size": 0 },
    "windows-portable": { "name": "Lattis-windows-x64-portable.exe", "url": "…", "sha256": "…", "size": 0 },
    "windows-msix":   { "name": "Lattis-windows-x64.msix", "url": "…", "sha256": "…", "size": 0 },
    "macos-dmg":      { "name": "Lattis-macos-universal.dmg", "url": "…", "sha256": "…", "size": 0 },
    "linux-appimage": { "name": "Lattis-linux-x64.AppImage", "url": "…", "sha256": "…", "size": 0 },
    "linux-deb":      { "name": "Lattis-linux-x64.deb", "url": "…", "sha256": "…", "size": 0 },
    "linux-rpm":      { "name": "Lattis-linux-x64.rpm", "url": "…", "sha256": "…", "size": 0 }
  }
}
```

`date` : moment de la construction (UTC, ISO 8601) ; `size` : octets. Attention : la redirection `releases/latest/download/latest.json` ne renvoie pas d'en-têtes CORS, un navigateur ne peut donc pas la lire directement (un serveur, une fonction de build du site ou l'API GitHub le peuvent).

## Mise à jour automatique

L'application de bureau (`internal/update`) lit la liste des versions par l'API GitHub (`https://api.github.com/repos/Be-Lattis/lattis-app/releases`), 30 s après son démarrage puis au plus toutes les 12 heures, et propose la plus récente qui soit **strictement plus récente** que la sienne : une version bêta (`vX.Y.Z-beta`, `vX.Y.Z-beta.N`, tout tag avec un tiret) reçoit aussi les pré-versions, une version publiée seulement les versions publiées. Brouillons et compilations `dev` : rien. Au clic de l'utilisateur dans la barre d'état, elle télécharge le fichier qui correspond à son installation :

| Installation détectée | Fichier téléchargé | Application, à la fermeture de Lattis |
|---|---|---|
| pour l'utilisateur (`InstallLocation` de `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\LattisLattis` = dossier de l'exécutable) | `Lattis-windows-x64-setup.exe` | `setup.exe /S /UPDATE [/RELAUNCH] /D=<dossier>`, sans élévation |
| pour la machine (même clé sous `HKLM`) | `Lattis-windows-x64-setup-machine.exe` | idem, lancé avec l'invite UAC ; refusée, la mise à jour reste prête et la barre d'état en donne la raison |
| portable (ni l'une ni l'autre) | `Lattis-windows-x64-portable.exe` | téléchargé à côté (`lattis.exe.new`), mis à la place de l'exécutable (l'ancien devient `.old`, supprimé au lancement suivant) ; dossier en lecture seule : le clic ouvre la page de la version |
| MSIX | — | rien : Windows (Store, App Installer) le met à jour |
| macOS, Linux | — | la nouvelle version est signalée, le clic ouvre sa page |

« Redémarrez l'application pour appliquer la mise à jour » ferme Lattis par le chemin habituel (questions d'enregistrement, écouteurs `before_close`) puis relance l'application après l'installation (`/RELAUNCH`) ; une fermeture ordinaire installe aussi la mise à jour, sans relance. L'installeur attend que `lattis.exe` soit libre (30 s au plus) avant de le remplacer.

**Vérifications**, avant toute installation : HTTPS seulement (redirections comprises), aucun cookie ni identifiant, `User-Agent: Lattis/<version> (<système>)` ; empreinte SHA-256 et taille lues dans le `latest.json` de la version (dont la version doit être celle du tag), sinon dans `SHA256SUMS.txt` : **sans empreinte publiée, rien n'est installé** ; 512 Mio au plus ; si l'exécutable en cours porte une signature Authenticode valide, le fichier téléchargé doit en porter une aussi. Une version qui n'est pas plus récente n'est jamais installée. Les installeurs attendent dans `%LOCALAPPDATA%\Lattis\updates` ; l'état (dernière recherche, version trouvée, mise à jour prête, dernière version lancée) est dans `%APPDATA%\Lattis\update.json`, hors de `config.json`. Au premier lancement d'une version plus récente que la précédente, la barre d'état renvoie vers ses notes (`releases/tag/<tag>`).

Désactivation : Fichier › Paramètres › Mises à jour, ou `updates.check = false` dans la configuration d'entreprise (en stratégie : verrouillé, voir [configuration-entreprise.md](../configuration-entreprise.md)). Rien en édition web.

**À éprouver sur une vraie version** (les tests passent par un faux GitHub local) : la construction des deux installeurs par makensis en CI, l'installation silencieuse par-dessus une installation existante (attente de `lattis.exe`, `/D=`, relance), l'invite UAC et la relance sans élévation par l'Explorateur, le remplacement de l'exécutable portable en cours d'exécution, les redirections réelles de GitHub vers `objects.githubusercontent.com`. Pour cela : publier une pré-version `vX.Y.Z-beta.1`, l'installer de chacune des trois façons, puis publier `vX.Y.Z-beta.2`.

## Notes de version

`scripts/release-notes.sh v1.2.0` les produit (aussi en local, pour relire avant de taguer) : la section `## 1.2.0 (JJ/MM/AAAA)` de `CHANGELOG.md` telle qu'elle est au tag, puis le tableau des fichiers à télécharger. Avant de taguer, renommer « Non publié » en `## 1.2.0 (JJ/MM/AAAA)` dans `CHANGELOG.md` et rouvrir un « Non publié » vide (la façon de tenir le journal est rappelée en tête du fichier). Sans section pour cette version, les notes reprennent la liste des commits depuis le tag précédent, sans les fusions ni les régénérations de liaisons ; la première version dit alors simplement « Première version publique de Lattis ».

## Brouillon ou publication directe

La release est publiée directement. Pour la relire avant publication, créer la variable de dépôt `RELEASE_DRAFT` à `true` (Settings › Secrets and variables › Actions › Variables) : la release reste alors en brouillon, à publier depuis GitHub ; tant qu'elle est en brouillon, les liens `releases/latest` désignent encore la version précédente.

## Essai sans release

Actions › Release › Run workflow, avec une version d'essai (`v0.0.0-test`) : les paquets sont construits, puis `SHA256SUMS.txt`, `latest.json` et les notes sont préparés et déposés dans l'artefact `release-essai` du run, sans créer de release ni de tag.
