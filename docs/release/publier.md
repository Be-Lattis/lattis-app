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

**Un installeur (Windows), deux façons d'installer.** `Lattis-windows-x64-setup.exe` démarre **sans droits d'administration** (pas d'invite UAC à l'ouverture) et demande, comme l'installeur de Python, pour qui installer :

- « Installer pour moi seulement » (choix par défaut) : dans `%LOCALAPPDATA%\Programs\Lattis`, clé de désinstallation sous `HKCU`, raccourcis et associations de l'utilisateur, sans droits d'administration ; c'est le choix des postes sans droits d'administration, et l'installation qui se met à jour sans invite de Windows ;
- « Installer pour tous les utilisateurs de ce poste » : dans `Program Files\Lattis\Lattis`, clé sous `HKLM`, pour tous les comptes ; l'installeur se relance alors avec les droits d'administration (invite UAC, bouclier sur le bouton Suivant) ; refusée, on reste sur la page du choix. Sa mise à jour automatique passe aussi par l'invite UAC.

Si Lattis est déjà installé, la page coche l'installation existante (« déjà installé ») et l'installeur la met à jour sur place, dans son dossier ; c'est aussi le cas des installations faites par les deux installeurs de la 0.3.0-beta (même clé de désinstallation `LattisLattis`, mêmes dossiers). **Une seule installation par utilisateur** (jamais deux entrées « Lattis » dans Applications installées). Choisir l'autre portée remplace l'installation existante, après confirmation (« Lattis 0.3.3-beta est déjà installé pour tous les utilisateurs de ce poste (Program Files). Il sera remplacé… ») :

- pour l'utilisateur alors qu'une installation pour tous existe : celle-ci est retirée **avant** la copie des fichiers, par son désinstalleur silencieux lancé par une instance élevée de l'installeur (`/REMOVE=machine`, invite UAC). Refusée : si ce serait une seconde installation, **rien n'est installé** et l'installeur dit de relancer en choisissant « pour tous les utilisateurs » (mise à jour sur place) ; si l'installation pour l'utilisateur existait déjà, elle est mise à jour et l'autre reste (l'application proposera de la retirer) ;
- pour tous alors qu'une installation pour l'utilisateur existe : celle-ci est retirée **après** l'installation, dans le compte de l'utilisateur (par l'instance non élevée une fois l'instance élevée terminée : l'instance élevée peut tourner sous un autre compte, dont `HKCU` n'est pas celui de l'utilisateur) ;
- en silencieux (`/S`, mise à jour automatique) : l'installation existante est mise à jour sur place, jamais une seconde créée — sans option de portée, la portée de l'installation existante est prise même si `/D=` désigne un autre dossier. Si les deux existent, celle de `/D=` (celle d'où tourne l'application) est mise à jour et l'autre retirée quand c'est possible sans invite (droits d'administration déjà là) ; sinon l'application le propose au lancement suivant. `/CurrentUser` sans installation pour l'utilisateur alors qu'une installation pour tous existe demande l'invite UAC pour la retirer (refusée : rien n'est installé).

Le retrait passe par le désinstalleur de l'autre installation (`/S /KEEPDATA _?=<dossier>`) ; sans désinstalleur (clé orpheline), sa clé et ses fichiers connus sont retirés. Les données WebView2 de l'utilisateur (`%APPDATA%\lattis.exe` : historique SQL, préférences de l'interface), communes à ses installations, sont mises de côté pendant le retrait d'une installation pour l'utilisateur (les désinstalleurs jusqu'à la 0.3.5-beta les effacent) ; impossible si Lattis est ouvert : l'installation n'est alors pas retirée. Les restes sans désinstalleur ni clé aux dossiers par défaut (`Program Files\Lattis\Lattis\lattis.exe` d'une installation ancienne) sont retirés quand l'installeur en a le droit (Program Files : avec les droits d'administration), fichiers connus seulement (`lattis.exe`, `.old`, `.new`), jamais les données. Au lancement, si deux installations sont inscrites, la barre d'état affiche « Deux installations de Lattis sur ce poste — Retirer l'ancienne » : le clic lance le désinstalleur de l'autre (invite UAC si elle est pour tous ; une installation pour l'utilisateur d'avant la 0.3.6-beta, dont le désinstalleur effacerait les données WebView2, est retirée par l'application elle-même : fichiers connus, clé, raccourcis, classe de fichiers), puis l'avis disparaît quand elle a quitté le Registre.

Le désinstalleur retire l'installation de son dossier, et demande les droits d'administration si c'est celle pour tous ; il retire sa clé de désinstallation, ses raccourcis, l'association `.lattis` (la classe d'avant remise, la valeur de sauvegarde retirée) et les liens `lattis://` s'ils le désignent (une autre installation a pu les reprendre), `Program Files\Lattis` s'il est vide ; les données WebView2 restent si une autre installation reste ou avec `/KEEPDATA`. Les réglages, classeurs récents et caches sont dans le profil (`%APPDATA%\Lattis`, `%LOCALAPPDATA%\Lattis`), communs aux deux installations et à la version portable. Textes de l'installeur en français, anglais, allemand, espagnol et chinois simplifié, selon la langue de Windows (l'anglais sinon). Construction : `wails3 task windows:package:nsis` (`build/windows/nsis/project.nsi`, `lattis_scope.nsh`, `lattis_lang.nsh`) donne `bin/lattis-amd64-installer.exe`.

Ligne de commande (déploiement) :

| Option | Effet |
|---|---|
| `/S` | silencieux |
| `/CurrentUser` | pour l'utilisateur, sans demander de droits — sauf pour retirer une installation pour tous quand il n'y a pas encore d'installation pour l'utilisateur (invite UAC ; refusée : rien n'est installé) |
| `/AllUsers` | pour tous les utilisateurs ; si l'installeur n'a pas les droits d'administration, il se relance avec l'invite UAC, attend et rend le code de sortie de l'installation |
| ni l'un ni l'autre | avec `/D=`, la portée de l'installation de ce dossier (pour tous sous Program Files) ; sans `/D=`, celle de l'installation existante (les deux : pour tous si l'installeur a déjà les droits, pour l'utilisateur sinon) ; sinon la page du choix, ou en silencieux pour tous si l'installeur a les droits d'administration (outil de déploiement), pour l'utilisateur sinon ; en silencieux, une installation existante l'emporte toujours (mise à jour sur place, même avec un `/D=` nouveau) |
| `/UPDATE` | ne recrée pas le raccourci du Bureau (mise à jour automatique) |
| `/RELAUNCH` | rouvre Lattis après l'installation, sans les droits d'administration (mise à jour automatique) |
| `/D=dossier` | dossier d'installation, en dernier, sans guillemets |

Exemples : `Lattis-windows-x64-setup.exe /S /AllUsers` (déploiement sur un parc, lancé avec les droits), `Lattis-windows-x64-setup.exe /S /CurrentUser`. Désinstallation silencieuse : `"<dossier>\uninstall.exe" /S`.

**Compatibilité 0.3.0-beta.** La release publie aussi `Lattis-windows-x64-setup-machine.exe`, **copie exacte** de `Lattis-windows-x64-setup.exe`, avec son entrée `windows-setup-machine` dans `latest.json` : la 0.3.0-beta installée pour tous les utilisateurs cherche sa mise à jour sous ce nom (voir [Mise à jour automatique](#mise-à-jour-automatique)). Ne pas l'annoncer sur le site ; la retirer quand plus aucune 0.3.0-beta par machine n'est en service.

**Version portable (Windows).** `Lattis-windows-x64-portable.exe` est l'exécutable seul, lancé sans installation (clé USB, poste sans droits d'administration). Les liens `lattis://` marchent : l'application les déclare pour l'utilisateur au premier lancement, si aucune version installée ne le fait déjà. Ne sont pas faits : l'association des fichiers `.lattis` (double-clic), le raccourci du menu Démarrer et l'entrée dans Ajout/Suppression de programmes. Les réglages, classeurs récents et caches restent dans le profil de l'utilisateur (`%APPDATA%`, `%LOCALAPPDATA%`), communs avec une version installée. Il faut WebView2, présent sur Windows 10 à jour et Windows 11.

**AppImage (Linux).** `Lattis-linux-x64.AppImage` embarque GTK 4 et WebKitGTK 6 d'Ubuntu 24.04 (machine de construction) et doit démarrer sur toute distribution récente, Debian/Ubuntu ou non (Fedora, Arch, openSUSE). `wails3 task linux:release` (tâche `linux:create:appimage`) appelle `wails3 generate appimage` (linuxdeploy), puis `build/linux/appimage/patch-appdir.sh` corrige l'AppDir et reconstruit l'AppImage avec appimagetool :

- libwebkitgtk-6.0 lance ses processus auxiliaires (`WebKitWebProcess`, `WebKitNetworkProcess`, `WebKitGPUProcess`) depuis un chemin absolu compilé, `/usr/lib/x86_64-linux-gnu/webkitgtk-6.0`, qui n'existe pas hors Debian (et `WEBKIT_EXEC_PATH` n'est lu que par les compilations de développement de WebKit). Le script copie les auxiliaires dans l'AppDir sous ce même chemin, exécutables, et remplace dans la bibliothèque embarquée ce chemin par le chemin relatif de même longueur `././/lib/x86_64-linux-gnu/webkitgtk-6.0` ;
- l'AppRun (`build/linux/appimage/apprun.sh`) se place dans `$APPDIR/usr` avant de lancer `lattis`, pour que ce chemin relatif mène aux auxiliaires embarqués ; les fichiers passés en chemin relatif sur la ligne de commande sont d'abord rendus absolus. Il lit aussi les réglages GTK de linuxdeploy (`apprun-hooks`) : thème Adwaita, X11 par défaut (`GDK_BACKEND` de l'utilisateur gardé) ;
- le bac à sable bubblewrap de WebKit est coupé dans l'AppImage (`WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1`) : bwrap ne sait pas monter le chemin relatif des auxiliaires et WebKit s'arrêterait au démarrage. Le `.deb` et le `.rpm` utilisent le WebKit du système et gardent le bac à sable ;
- le module TLS de GIO (glib-networking) est embarqué, pour le HTTPS de WebKit quand celui du système n'est pas à l'emplacement Debian.

Le système doit fournir ce que l'AppImage n'embarque pas (liste d'exclusion de linuxdeploy) : glibc, pilotes OpenGL/EGL (mesa), libX11/libxcb, fontconfig, freetype, harfbuzz, fribidi, libstdc++ ; tout poste de bureau les a. Essai sans FUSE : `./Lattis-linux-x64.AppImage --appimage-extract-and-run`.

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

L'application de bureau (`internal/update`) lit la liste des versions par l'API GitHub (`https://api.github.com/repos/Be-Lattis/lattis-app/releases`), dès que son interface est chargée (au plus tard 30 s après le démarrage) puis au plus toutes les 12 heures, et propose la plus récente qui soit **strictement plus récente** que la sienne : une version bêta (`vX.Y.Z-beta`, `vX.Y.Z-beta.N`, tout tag avec un tiret) reçoit aussi les pré-versions, une version publiée seulement les versions publiées. Brouillons et compilations `dev` : rien. Au clic de l'utilisateur dans la barre d'état, elle télécharge le fichier qui correspond à son installation :

| Installation détectée | Fichier téléchargé | Application, à la fermeture de Lattis |
|---|---|---|
| pour l'utilisateur (`InstallLocation` de `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\LattisLattis` = dossier de l'exécutable) | `Lattis-windows-x64-setup.exe` | `setup.exe /S /CurrentUser /UPDATE [/RELAUNCH] /D=<dossier>`, sans élévation |
| pour tous les utilisateurs (même clé sous `HKLM`) | `Lattis-windows-x64-setup.exe` | `setup.exe /S /AllUsers /UPDATE [/RELAUNCH] /D=<dossier>`, lancé avec l'invite UAC ; refusée, la mise à jour reste prête et la barre d'état en donne la raison |
| portable (ni l'une ni l'autre) | `Lattis-windows-x64-portable.exe` | téléchargé à côté (`lattis.exe.new`), mis à la place de l'exécutable (l'ancien devient `.old`, supprimé au lancement suivant) ; dossier en lecture seule : le clic ouvre la page de la version |
| MSIX | — | rien : Windows (Store, App Installer) le met à jour |
| macOS, Linux | — | la nouvelle version est signalée, le clic ouvre sa page |

**Versions 0.3.0-beta déjà installées.** Leur code (avant l'installeur unique) cherche, dans la liste des fichiers de la release GitHub puis dans `latest.json` par son nom, `Lattis-windows-x64-setup.exe` pour une installation pour l'utilisateur et `Lattis-windows-x64-setup-machine.exe` pour une installation pour la machine, et lance ce fichier avec `/S /UPDATE [/RELAUNCH] /D=<dossier>`, **sans** `/CurrentUser` ni `/AllUsers` (avec l'invite UAC pour la machine). D'où, pour qu'elles se mettent à jour : la release publie l'installeur aussi sous l'ancien nom par machine (même fichier, entrée `windows-setup-machine` de `latest.json`), et l'installeur, sans option de portée, prend celle de l'installation dont le dossier est `/D=` (`HKLM` : pour tous, il a déjà les droits par l'invite UAC ; `HKCU` : pour l'utilisateur, sans élévation). La mise à jour arrive donc dans le même dossier, avec la même clé de désinstallation.

« Redémarrez l'application pour appliquer la mise à jour » ferme Lattis par le chemin habituel (questions d'enregistrement, écouteurs `before_close`) puis relance l'application après l'installation (`/RELAUNCH`) ; une fermeture ordinaire installe aussi la mise à jour, sans relance. L'installeur attend que `lattis.exe` soit libre (30 s au plus) avant de le remplacer.

**Vérifications**, avant toute installation : HTTPS seulement (redirections comprises), aucun cookie ni identifiant, `User-Agent: Lattis/<version> (<système>)` ; empreinte SHA-256 et taille lues dans le `latest.json` de la version (dont la version doit être celle du tag), sinon dans `SHA256SUMS.txt` : **sans empreinte publiée, rien n'est installé** ; 512 Mio au plus ; si l'exécutable en cours porte une signature Authenticode valide, le fichier téléchargé doit en porter une aussi. Une version qui n'est pas plus récente n'est jamais installée. Les installeurs attendent dans `%LOCALAPPDATA%\Lattis\updates` ; l'état (dernière recherche, version trouvée, mise à jour prête, dernière version lancée) est dans `%APPDATA%\Lattis\update.json`, hors de `config.json`. Au premier lancement d'une version plus récente que la précédente, la barre d'état renvoie vers ses notes (`releases/tag/<tag>`).

Désactivation : Fichier › Paramètres › Mises à jour, ou `updates.check = false` dans la configuration d'entreprise (en stratégie : verrouillé, voir [configuration-entreprise.md](../configuration-entreprise.md)). Rien en édition web.

**À éprouver sur une vraie version** (les tests passent par un faux GitHub local) : la construction de l'installeur par makensis en CI, le remplacement de l'installation de l'autre portée dans les deux sens (et le refus de l'invite UAC), l'avis « Deux installations » et son retrait, la page du choix et la relance élevée pour tous les utilisateurs (et son refus), la mise à jour d'une 0.3.0-beta installée de chacune des deux façons, l'installation silencieuse par-dessus une installation existante (attente de `lattis.exe`, `/D=`, relance), l'invite UAC et la relance sans élévation par l'Explorateur, le remplacement de l'exécutable portable en cours d'exécution, les redirections réelles de GitHub vers `objects.githubusercontent.com`. Pour cela : publier une pré-version `vX.Y.Z-beta.1`, l'installer de chacune des trois façons, puis publier `vX.Y.Z-beta.2`.

## Notes de version

`scripts/release-notes.sh v1.2.0` les produit (aussi en local, pour relire avant de taguer) : la section `## 1.2.0 (JJ/MM/AAAA)` de `CHANGELOG.md` telle qu'elle est au tag, puis le tableau des fichiers à télécharger. Avant de taguer, renommer « Non publié » en `## 1.2.0 (JJ/MM/AAAA)` dans `CHANGELOG.md` et rouvrir un « Non publié » vide (la façon de tenir le journal est rappelée en tête du fichier). Sans section pour cette version, les notes reprennent la liste des commits depuis le tag précédent, sans les fusions ni les régénérations de liaisons ; la première version dit alors simplement « Première version publique de Lattis ».

## Brouillon ou publication directe

La release est publiée directement. Pour la relire avant publication, créer la variable de dépôt `RELEASE_DRAFT` à `true` (Settings › Secrets and variables › Actions › Variables) : la release reste alors en brouillon, à publier depuis GitHub ; tant qu'elle est en brouillon, les liens `releases/latest` désignent encore la version précédente.

## Essai sans release

Actions › Release › Run workflow, avec une version d'essai (`v0.0.0-test`) : les paquets sont construits, puis `SHA256SUMS.txt`, `latest.json` et les notes sont préparés et déposés dans l'artefact `release-essai` du run, sans créer de release ni de tag.
