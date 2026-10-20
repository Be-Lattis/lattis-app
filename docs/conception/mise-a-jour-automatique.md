# Mise à jour automatique de l'application de bureau

*Étude de conception, 20/10/2026, demandée par Louis : « Étudie un système de self update ». Pas de code. Lue sur `main` (`efacc52`) : décision 0006 (contrôle de mise à jour, proposée, non implémentée), décision 0014 (file d'envoi gardée), `docs/release/` (publier, checklist, signature, choix-autorite), `.github/workflows/release.yml`, `scripts/release-manifest.sh`, `build/windows/nsis` (`wails_tools.nsh`), `build/windows/msix`, `build/darwin`, `build/linux`, `docs/configuration-entreprise.md`, `internal/config` (schéma, `adminOnly`, `lattis config sign`), `internal/instance`, `main.go`, `internal/bridge/about.go`. Côté Wails : `github.com/wailsapp/wails/v3@v3.0.0-beta.30/pkg/updater`.*

## En bref

- **Étendre la décision 0006, pas la remplacer** : le contrôle (un `GET` de `latest.json`, désactivé par défaut, pilotable par la DSI) reste le socle. La mise à jour automatique s'y ajoute plateforme par plateforme, seulement là où elle peut être sûre et fluide.
- **Signer le manifeste** (Ed25519, clé publique compilée dans l'application) : c'est ce qui rend l'installation automatique acceptable tant que les binaires Windows et macOS ne sont pas signés.
- **Moteur** : `pkg/updater` de Wails v3, déjà présent dans le module (aucune dépendance nouvelle), en mode sans fenêtre, avec un fournisseur maison qui lit `latest.json`. Repli connu : `minio/selfupdate`.
- **Ordre** : lot 1 contrôle + avis + téléchargement vérifié ; lot 2 Windows installeur silencieux ; lot 3 Windows portable et AppImage par remplacement ; macOS après la signature Developer ID ; MSIX, `.deb`, `.rpm` laissés à leur gestionnaire.

## 1. Objectifs et contraintes

**Vie privée.** Aucune télémétrie. Une seule requête `GET` du manifeste, sans cookie ni identifiant, `User-Agent: Lattis/<version> (<système>)` (0006). Le téléchargement d'un installeur est une seconde requête, uniquement si l'utilisateur l'a demandé ou l'a autorisé (`updates.auto`). Le tout désactivable par l'utilisateur et verrouillable par une stratégie.

**Canal bêta.** Les tags avec tiret (`v0.2.0-beta`) sont des pré-versions. Constat : **toutes les versions publiées à ce jour sont des pré-versions** (`v0.1.0-beta.*`, `v0.2.0-beta`), donc `releases/latest/download/latest.json` répond 404 : le canal stable n'a encore rien à proposer. Tant que dure la bêta, le canal par défaut des compilations `-beta` doit être `beta` (sinon personne n'est prévenu). Comparaison en SemVer (`0.2.0` > `0.2.0-beta.1` > `0.2.0-beta`) ; une compilation `dev` ne contrôle jamais.

**Entreprises hors ligne.** Une DSI coupe tout (`updates.check = false`, verrouillé), ou pointe vers un miroir interne (`updates.url`, couche administrateur seulement, comme `configUrl`). Un poste sans réseau ne voit qu'un échec silencieux noté au journal.

**Binaires non signés (Windows, macOS).** Conséquences :

- *Intégrité.* Le `sha256` de `latest.json` lu en HTTPS protège contre un fichier corrompu ou un CDN défaillant, **pas** contre quelqu'un qui contrôle la release : il remplacerait l'exécutable et le manifeste ensemble. La signature GPG de `SHA256SUMS.txt` existe mais n'est vérifiée par personne et dépend d'une clé facultative. D'où la recommandation : **signature Ed25519 de `latest.json`**, clé privée hors du dépôt, clé publique compilée dans Lattis. L'application refuse tout manifeste dont la signature ne vérifie pas avec la clé qu'elle porte ; la release ne peut pas imposer sa propre clé. Lattis a déjà l'outillage (`lattis config keygen`, `lattis config sign`, JWS EdDSA pour la configuration distante) : même algorithme, même code de vérification.
- *SmartScreen.* Un fichier téléchargé par Lattis lui-même (client HTTP Go) ne porte pas de « Mark of the Web » : SmartScreen ne l'arrête pas au lancement, contrairement au navigateur. La signature du manifeste tient alors lieu de preuve d'origine. Limites : Smart App Control (Windows 11 neuf) bloque tout exécutable non signé, et l'UAC d'un installeur par machine affiche « Éditeur inconnu ».
- *Gatekeeper.* Un `.app` téléchargé par Lattis n'a pas d'attribut de quarantaine, mais la signature ad hoc change à chaque version : le trousseau macOS redemande l'accès aux mots de passe enregistrés après chaque mise à jour, et l'App Translocation (app lancée sans être déplacée dans Applications) rend le remplacement impossible. Voir § 2.

## 2. Par plateforme

L'application doit savoir comment elle est installée. Proposition : `internal/update.InstallKind()` → `nsis`, `portable`, `msix`, `macos-app`, `appimage`, `system` (deb/rpm), `dev`. Détection à l'exécution : identité de paquet (MSIX) ; `InstallLocation` de la clé de désinstallation NSIS égale au dossier de l'exécutable (sinon portable, puisque le portable est le même `lattis.exe`) ; `$APPIMAGE` ; exécutable dans un `.app` ; `/usr/bin` ou `/opt` (paquet système).

| Installation | Recommandation | Mécanisme |
|---|---|---|
| Windows, installeur NSIS | Lot 2 | Télécharger `setup.exe`, vérifier, lancer `/S` après la fermeture de Lattis, relancer |
| Windows, portable | Lot 3 | Remplacement de l'exécutable au redémarrage |
| Windows, MSIX | Ne rien faire soi-même | `.appinstaller` ou Microsoft Store |
| macOS `.dmg` | Avis + téléchargement jusqu'à la signature Developer ID | Puis remplacement du `.app` (archive `.zip`) |
| Linux AppImage | Lot 3 | Remplacement du fichier `$APPIMAGE` ; en option zsync |
| Linux `.deb` / `.rpm` | Avis seulement | Gestionnaire de paquets ; dépôt APT/RPM plus tard |

**Windows NSIS.** `wails_tools.nsh` installe par défaut **par machine** : `WAILS_INSTALL_SCOPE "machine"`, `RequestExecutionLevel admin`, `$PROGRAMFILES64\…`. Un utilisateur ordinaire ne peut donc ni remplacer `lattis.exe` ni lancer l'installeur sans UAC. Deux voies :

1. *Garder par machine* : Lattis télécharge et vérifie `Lattis-windows-x64-setup.exe`, l'utilisateur choisit « Installer et redémarrer », Lattis se ferme, un assistant (le même `lattis.exe` en mode auxiliaire, non élevé) lance `setup.exe /S` élevé (`ShellExecute runas`), attend sa fin, puis relance Lattis **non élevé** (lancé par l'installeur, il hériterait des droits administrateur). Une invite UAC par mise à jour ; impossible sans droits d'administration (le cas des parcs gérés, qui coupent de toute façon le contrôle).
2. *Passer par utilisateur* (`WAILS_INSTALL_SCOPE "user"`, `%LOCALAPPDATA%\Programs\Lattis`) : plus d'UAC, `setup.exe /S` tourne sans élévation, comme VS Code « User setup ». Coût : migrer les installations par machine existantes (l'installeur par utilisateur doit détecter et proposer de désinstaller l'ancienne, ce qui demande l'UAC une fois) ; pour les DSI, garder un installeur par machine ou le MSIX.

Recommandation : voie 2 à moyen terme (c'est elle qui rend la mise à jour automatique fluide), voie 1 en lot 2 pour ne pas bloquer. Dans les deux cas on lance l'**installeur** plutôt que de remplacer `lattis.exe` sur place : il tient à jour la version affichée dans Ajout/Suppression de programmes, le désinstalleur et les associations.

**Windows portable.** Télécharger `Lattis-windows-x64-portable.exe` dans le même dossier (`lattis.exe.new`), vérifier, puis au redémarrage : renommer l'exécutable en cours (`.old`, Windows l'autorise même chargé), mettre le nouveau à sa place, relancer, supprimer `.old` au démarrage suivant. Si le dossier n'est pas inscriptible (clé USB protégée, partage), revenir à « avis + téléchargement ». C'est exactement ce que fait `pkg/updater` (§ 2 bis).

**Windows MSIX.** Pas de mise à jour par Lattis. Hors Store : publier un `Lattis.appinstaller` (adresse stable, `UpdateSettings` `OnLaunch HoursBetweenUpdateChecks="24"`, `ShowPrompt`, `AutomaticBackgroundTask`) ; App Installer le relit et met à jour, à condition que le MSIX soit signé (il n'est publié que signé). Dans le Store (décision de choix-autorite.md) : le Store met à jour, et ses règles interdisent à une application de se mettre à jour elle-même. Lattis détecte son identité de paquet et désactive tout, contrôle compris (ou avis seulement).

**macOS.** `pkg/updater` remplace un `.app` mais ne lit pas un `.dmg` (seulement `.zip` et `.tar.gz`) : il faudrait publier en plus `Lattis-macos-universal.zip` (`ditto -c -k --keepParent`). Sans Developer ID, le remplacement marche techniquement mais l'expérience est mauvaise (trousseau, translocation, `/Applications` non inscriptible pour un compte standard). Recommandation : **avis + bouton qui ouvre la page de la version** jusqu'à la signature et la notarisation, puis remplacement du `.app` (lot macOS).

**Linux.** *AppImage* : remplacer le fichier désigné par `$APPIMAGE` (s'il est inscriptible), relancer ; `pkg/updater` le gère déjà (`appImageOr`). Bonus peu coûteux : inscrire l'information de mise à jour (`gh-releases-zsync|Be-Lattis|lattis-app|latest|Lattis-linux-x64.AppImage.zsync`) et publier le `.zsync`, pour AppImageUpdate et Gear Lever. *deb/rpm* : avis seulement (« mettez à jour avec votre gestionnaire de paquets ») ; plus tard un dépôt APT/RPM signé avec la clé GPG existante, publié sur GitHub Pages.

## 2 bis. Moteur : `pkg/updater` de Wails ou une bibliothèque

Wails v3 beta.30 contient un **updater complet** (`pkg/updater`, exposé en `app.Updater`, `updater.HandleHelperMode()` appelé par `application.New`) :

- fournisseurs enfichables (`Provider` : `Check`, `Download`) ; fournis : GitHub, point d'accès JSON (« Wails Update Manifest »), appcast Sparkle, keygen.sh ;
- téléchargement avec empreinte calculée au fil de l'eau, vérification SHA-256/512 et signature Ed25519 / ECDSA-P256 contre `Config.PublicKey` seulement, refus si signature sans clé (« fail closed ») ;
- remplacement par un processus auxiliaire (le même exécutable relancé avec des variables d'environnement) qui attend la sortie de l'application, sauvegarde en `.bak`, remplace (renommage de côté sous Windows, `.app` entier sous macOS, `$APPIMAGE` sous Linux), relance, et **restaure la sauvegarde si la relance échoue** ;
- évènements `wails:updater:*` et fenêtre intégrée (en anglais) ou `WindowNone`.

Proposition : l'utiliser en mode `WindowNone`, avec un fournisseur `lattis` qui lit `latest.json`, vérifie sa signature, choisit l'entrée de `files` selon `InstallKind()` et remplit `Verification{sha256}` ; l'interface reste la nôtre (Svelte, traduite). Réserves :

- API **bêta** (le code dit encore « v1 of this branch » par endroits) : l'isoler derrière `internal/update` pour pouvoir changer de moteur.
- Il remplace un exécutable, il ne lance pas d'installeur : la voie NSIS (lot 2) reste à écrire, sur le même modèle d'auxiliaire.
- La restauration ne couvre qu'un échec de lancement, pas une version qui plante au démarrage, et le `.bak` est supprimé aussitôt. À ajouter de notre côté : garder `lattis.exe.previous`, écrire un marqueur « démarrage réussi » une fois la fenêtre prête ; deux démarrages sans marqueur → proposer de revenir à la version précédente.
- `main.go` fait des choses avant `application.New` (sous-commandes `mcp`, `config`, `instance.Forward`) : vérifier que le mode auxiliaire n'est pas intercepté, ou appeler `updater.HandleHelperMode()` en tout premier.
- L'auxiliaire relance l'exécutable sans arguments : rouvrir les classeurs ouverts demande une liste que Lattis note avant de quitter.

Alternatives connues : **`minio/selfupdate`** (application d'un binaire avec somme de contrôle et signature Ed25519, renommage `.old` sous Windows, retour arrière si l'application échoue ; stable, sans interface) ; **`creativeprojects/go-selfupdate`** (au-dessus du précédent : sources GitHub/GitLab/Gitea, filtre des pré-versions, validation par somme ou ECDSA) ; `rhysd/go-github-selfupdate` (ancien, peu maintenu, à éviter). Aucune ne lance d'installeur ni ne gère un `.app` aussi bien. Wails d'abord, `minio/selfupdate` en repli.

## 3. Ce que voit l'utilisateur

- **Contrôle** : 30 s après le démarrage, puis au plus une fois par jour, en arrière-plan (0006). Jamais de fenêtre au démarrage, jamais pendant une saisie.
- **Avis discret** dans la barre d'état : « Lattis 0.3.0-beta est disponible ». Le clic ouvre un petit panneau (ou À propos) : notes de version (section du `CHANGELOG`, lien `release_url`), taille, et selon l'installation « Installer et redémarrer », « Installer à la fermeture », « Télécharger » (ouvre la page) ; « Ignorer cette version » ; « Plus tard ».
- **Travail non enregistré** : « Installer et redémarrer » passe par la fermeture normale (questions d'enregistrement, écouteurs `before_close` des scripts) ; si l'utilisateur annule, la mise à jour attend. « Installer à la fermeture » ne pose aucune question de plus : l'installation part quand Lattis se ferme normalement, et le prochain lancement est la nouvelle version.
- **Classeurs collaboratifs** : à l'arrêt, le cache est déjà enregistré avec sa file d'envoi et ses conflits (décision 0014, `keepCollabQueue`) : rien n'est perdu, les modifications partent à la réouverture. En plus, si le serveur répond, tenter d'envoyer la file avant de quitter (5 s au plus), et le dire : « 3 modifications envoyées » ou « 3 modifications en attente, envoyées au prochain démarrage ».
- **Après la mise à jour** : une ligne « Lattis a été mis à jour en 0.3.0-beta — Nouveautés » ; en cas d'échec, « La mise à jour n'a pas pu s'installer — Détails » (journal de l'auxiliaire) et l'ancienne version tourne.

## 4. Réglages

Dans la logique de 0006 et du schéma existant (`features.*`, `limits.*`, `ui.*`) :

| Clé | Rôle | Défaut | Notes |
|---|---|---|---|
| `updates.check` | contrôler les nouvelles versions | `false` (0006) | `false` en stratégie : tout coupé, section masquée |
| `updates.channel` | `stable` ou `beta` | `beta` pour une compilation `-beta`, `stable` sinon | `oneOf("stable", "beta")` |
| `updates.auto` | télécharger seul et installer à la fermeture | `false` | sans effet là où l'installation n'est pas prise en charge (MSIX, deb/rpm, macOS non signé) |
| `updates.url` | manifeste d'un miroir interne | GitHub | `adminOnly`, `isHTTPSURL`, comme `configUrl` |
| `updates.publicKey` | clé Ed25519 d'un miroir qui signe lui-même | aucune | `adminOnly` ; remplace la clé compilée pour ce miroir seulement |

Toutes verrouillables par stratégie (`HKLM\SOFTWARE\Policies\Lattis`, `policy.json`). Variables : `LATTIS_UPDATES__CHECK=false`. L'état (dernier contrôle, version vue, version ignorée, mise à jour en attente) va dans un petit fichier du dossier de configuration, hors de `config.json` (0006). Avec un miroir, les fichiers se téléchargent **à côté du manifeste** (`updates.url` + `name`), l'adresse absolue de GitHub est ignorée : le miroir n'a qu'à recopier la release, signature comprise, sans rien réécrire. Une DSI qui veut choisir elle-même la version publie son propre `latest.json` signé avec `updates.publicKey`.

## 5. Sécurité

- **HTTPS seulement**, redirections HTTPS seulement (GitHub redirige vers `objects.githubusercontent.com`) ; délai de 5 s pour le manifeste.
- **Tailles bornées** : manifeste ≤ 64 Kio ; fichier ≤ `size` du manifeste et ≤ 512 Mio ; téléchargement dans le dossier temporaire de l'utilisateur, nom aléatoire.
- **Ordre des vérifications** : signature du manifeste → version strictement plus récente → empreinte SHA-256 et taille du fichier → seulement alors exécution ou remplacement. Un échec arrête tout et supprime le fichier.
- **Pas de retour en arrière** : refuser toute version ≤ version courante, même signée (un ancien manifeste rejoué ne fait pas réinstaller une version vulnérable). Option : champ `expires` signé contre le gel sur une ancienne version.
- **Remplacement atomique** (renommage, jamais réécriture sur place) ; version précédente conservée ; retour automatique si la nouvelle ne démarre pas (§ 2 bis).
- **Si quelqu'un contrôle les releases GitHub** (compte ou jeton volé, workflow détourné) : sans signature du manifeste, il remplace l'exécutable et le manifeste avec des empreintes cohérentes, et chaque Lattis qui s'installe seul exécute son code ; c'est le risque majeur de toute mise à jour automatique. Avec la clé Ed25519 compilée, il lui faut aussi la clé privée. Donc **la clé ne doit pas être volable par la même personne** : secret dans un environnement GitHub protégé (`release`, relecteur obligatoire) au minimum, ou signature hors de la CI (Louis signe le `latest.json` produit par le run, puis le dépose). Prévoir la rotation : compiler deux clés publiques (courante et suivante).
- Le jour où Windows et macOS sont signés, ajouter la vérification Authenticode / `codesign` du fichier téléchargé, en plus.

## 6. Côté serveur

Rien n'est nécessaire : GitHub Releases sert le manifeste, sa signature et les fichiers. À ajouter au workflow : `latest.json.sig` (signature détachée Ed25519, base64, des octets exacts de `latest.json` : le site continue de lire `latest.json` tel quel), l'archive macOS `.zip` le moment venu, éventuellement `.zsync` et `Lattis.appinstaller`. Pour le canal bêta, 0006 prévoit l'API GitHub (`releases?per_page=10`) ; plus simple et sans limite de débit : publier en plus, sur chaque release, une copie `latest-beta.json` sur une adresse stable (GitHub Pages). Option entreprise : `lattisd` pourrait servir `/updates/latest.json` (recopie de la release, ou version retenue par la DSI), ce qui n'ajoute qu'un réglage `updates.url` sur les postes.

## 7. Lots

| Lot | Contenu | Taille |
|---|---|---|
| **1 — Contrôle, avis, téléchargement vérifié** (recommandé en premier) | Décision 0006 telle quelle + signature Ed25519 de `latest.json` (workflow, clé publique compilée), `internal/update` (manifeste, signature, SemVer, `InstallKind`), réglages `updates.*` et stratégie, barre d'état, À propos, « Ignorer cette version » ; « Télécharger » ouvre la page de la version. Tests sans réseau. | M |
| 2 — Windows installeur silencieux | Téléchargement en arrière-plan, vérification, auxiliaire qui lance `setup.exe /S` (élevé tant que l'installeur est par machine), relance non élevée, « Installer à la fermeture », envoi de la file collaborative avant de quitter | M |
| 3 — Remplacement en place : portable et AppImage | `pkg/updater` en `WindowNone` avec le fournisseur `lattis`, `.previous` et marqueur de démarrage réussi | M |
| 4 — Installeur NSIS par utilisateur | `WAILS_INSTALL_SCOPE "user"`, migration des installations par machine, documentation DSI | M |
| 5 — Linux annexes | `.zsync` pour l'AppImage ; plus tard dépôt APT/RPM | S puis L |
| 6 — MSIX | `Lattis.appinstaller`, désactivation dans le Store | S |
| 7 — macOS | Après Developer ID + notarisation : `.zip` du `.app`, remplacement | M |

Le lot 1 livre seul une vraie valeur (les testeurs de la bêta savent qu'une version existe) sans exécuter de code téléchargé, et il pose les deux pièces dont tout le reste dépend : la signature du manifeste et `InstallKind`.

## Risques

- **Clé de signature** : sa fuite rend la mise à jour automatique dangereuse ; sa perte oblige à diffuser une version manuelle avec une nouvelle clé (d'où les deux clés compilées).
- **Binaires non signés** : Smart App Control et certains antivirus peuvent bloquer l'installeur téléchargé ; l'UAC affiche « Éditeur inconnu ». La signature (SignPath, Store) reste la vraie solution.
- **Updater Wails en bêta** : API susceptible de changer ; isolé derrière `internal/update`, repli `minio/selfupdate`.
- **Canal stable vide** : tant qu'aucune version sans tiret n'est publiée, le canal stable ne propose rien ; défaut `beta` pour les compilations bêta.
- **Parcs gérés** : installation par machine sans droits d'administration → pas d'installation automatique ; la DSI coupe le contrôle ou passe par son miroir.

## Décisions de Louis (20/10/2026) — à réaliser après le multilingue

1. **Deux installeurs Windows** : un installeur **par utilisateur** (dans le profil, sans droits d'administration : postes sans droit admin, mise à jour silencieuse sans UAC) et un installeur **par machine** (Program Files, droits d'administration), publiés côte à côte dans chaque release (noms stables à définir, par exemple `Lattis-windows-x64-setup.exe` pour l'utilisateur et `Lattis-windows-x64-setup-machine.exe`), décrits dans `latest.json`.
2. **Comportement de la mise à jour**, dans la barre d'état en bas :
   - version disponible : « Une nouvelle version est disponible, cliquez ici pour mettre à jour automatiquement » ;
   - après le clic, téléchargement et vérification ; une fois prête : « Redémarrez l'application pour appliquer la mise à jour » ;
   - première ouverture après un changement de version : « Mise à jour effectuée, consultez les notes de version » (lien vers la release GitHub de la version installée).
3. Ordre : après la fin du multilingue (les textes ci-dessus passent par `t()`).

## Réalisé (20/10/2026)

Les décisions ci-dessus, sur Windows ; le détail d'exploitation est dans [docs/release/publier.md](../release/publier.md#mise-à-jour-automatique).

- **Installeurs** : `project.nsi` produit l'installeur par utilisateur (`Lattis-windows-x64-setup.exe`, `%LOCALAPPDATA%\Programs\Lattis`, `HKCU`) et celui par machine (`Lattis-windows-x64-setup-machine.exe`, Program Files, `HKLM`) ; chacun prévient si l'autre est installé, écrit `InstallLocation`, attend que `lattis.exe` soit libre, et comprend `/UPDATE` et `/RELAUNCH` (relance sans élévation par l'Explorateur pour l'installeur par machine). `latest.json` décrit les deux (`windows-setup`, `windows-setup-machine`).
- **`internal/update`** (bibliothèque standard et `golang.org/x/sys`, sans dépendance nouvelle ; ni `pkg/updater` de Wails, ni `minio/selfupdate`) : versions SemVer, choix de la version par l'API GitHub (le canal suit la version courante : une bêta reçoit les bêtas), genre d'installation (`HKCU`/`HKLM` ou portable, MSIX exclu), téléchargement vérifié (empreinte et taille de `latest.json`, sinon `SHA256SUMS.txt` ; Authenticode exigé si l'application est signée), état dans `update.json`, application à la fermeture (`PostShutdown`) : installeur silencieux, installeur élevé (UAC), ou remplacement de l'exécutable portable.
- **Interface** : `UpdateStatus.svelte` dans la barre d'état (bureau seulement), les trois messages des décisions, plus « Téléchargement… », l'échec (clic : réessayer) et, hors installation automatique, « cliquez ici pour la télécharger » (page de la version).
- **Réglage** `updates.check`, **activé par défaut** (la décision 0006 le prévoyait coupé : les décisions de Louis supposent que la barre d'état prévienne), verrouillable par stratégie, section Fichier › Paramètres › Mises à jour masquée en édition web.

Pas encore fait, de l'étude : signature Ed25519 de `latest.json` (§ 5 : tant qu'elle manque, quiconque contrôle les releases GitHub peut publier un installeur et son empreinte ; l'Authenticode le couvre une fois les binaires signés), miroir d'entreprise `updates.url`, « Ignorer cette version », restauration automatique si la nouvelle version ne démarre pas, macOS et Linux (signalés seulement).
