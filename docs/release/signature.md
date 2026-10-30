# Installeurs signés : ce qu'il faut fournir

Le workflow `.github/workflows/release.yml` construit, sur chaque tag `v*`, les installeurs de Lattis pour les trois systèmes et les publie dans une release GitHub (publique ; brouillon seulement avec la variable `RELEASE_DRAFT`, voir [publier.md](publier.md)). Chaque signature dépend d'un secret : s'il manque, l'étape correspondante est sautée avec un avertissement dans le journal et le paquet sort non signé. On peut donc lancer une release dès maintenant et ajouter les certificats un par un.

| Système | Fichiers produits | Signature | À fournir |
|---|---|---|---|
| Windows | `Lattis-windows-x64-setup.exe` (NSIS), `Lattis-windows-x64.msix` (publié seulement signé : non signé, Windows refuse de l'installer) | Authenticode SHA-256 horodaté (signtool) : `lattis.exe`, installeur (et désinstalleur avec le `.pfx`), paquet MSIX | Un certificat de signature de code : Azure Trusted Signing **ou** certificat OV/EV |
| macOS | `Lattis-macos-universal.dmg` (arm64 + Intel) | Developer ID + hardened runtime, notarisation Apple, ticket agrafé à l'app et au `.dmg` | Adhésion Apple Developer Program, certificat « Developer ID Application » (.p12), clé API App Store Connect (.p8) |
| Linux | `Lattis-linux-x64.AppImage`, `Lattis-linux-x64.deb`, `Lattis-linux-x64.rpm` | Signature GPG intégrée aux `.deb`/`.rpm` ; signature détachée `.asc` de chaque fichier de la release et de `SHA256SUMS.txt` | Facultatif : une clé GPG |

Tous les paquets associent l'extension **`.lattis`** à Lattis (double-clic : le classeur s'ouvre). L'ancienne extension `.slitable` n'est plus lue par l'application depuis le renommage ; elle n'est donc associée nulle part.

## 1. Windows

### Ce qu'il faut choisir

Depuis juin 2023, les autorités de certification ne délivrent plus de certificat de signature de code (OV comme EV) dont la clé privée soit exportable en `.pfx` : la clé vit sur un jeton matériel ou dans un HSM en ligne. Deux voies réalistes :

- **Azure Trusted Signing** (recommandé pour la CI ; Microsoft l'a renommé « Artifact Signing », mêmes principes). Abonnement Azure, compte Trusted Signing (offre Basic, environ 10 $/mois), validation d'identité par Microsoft, profil de certificat « Public Trust ». Vérifier l'éligibilité au moment de l'inscription : elle a longtemps été réservée aux organisations ayant au moins trois ans d'existence vérifiable, et aux développeurs individuels de certains pays seulement.
  - Créer une inscription d'application (Entra ID) avec un secret client, et lui donner le rôle **Trusted Signing Certificate Profile Signer** sur le compte de signature.
  - Noter le **sujet** du certificat du profil (portail Azure, profil de certificat → « Subject name », de la forme `CN=Nom, O=Nom, L=Ville, S=Région, C=FR`) : il devient l'éditeur (Publisher) du MSIX, qui doit lui être identique au caractère près.
  - Limite : avec Trusted Signing, le désinstalleur NSIS (embarqué dans l'installeur) n'est pas signé ; l'installeur, `lattis.exe` et le MSIX le sont.
- **Certificat OV ou EV d'une autorité** (Sectigo, DigiCert, Certum, SSL.com…). Sur jeton matériel, il sert à signer **en local** (voir § 6, `WINDOWS_CERT_THUMBPRINT`) ; la CI ne peut pas l'utiliser. Si l'autorité fournit malgré tout un `.pfx` (ancien certificat encore valide, offre particulière), la CI le prend en charge directement. Les HSM en ligne des autorités (SSL.com eSigner, DigiCert KeyLocker…) demanderaient leur propre étape dans le workflow, non écrite ici.

Le MSIX signé avec un certificat public s'installe d'un double-clic (Windows 10 1903 ou plus récent). SmartScreen peut afficher un avertissement tant que l'éditeur n'a pas acquis de réputation, y compris avec un certificat EV.

### Secrets et variables GitHub

Dans le dépôt : **Settings → Secrets and variables → Actions**.

Voie `.pfx` :

| Secret | Contenu |
|---|---|
| `WINDOWS_CERT_PFX_BASE64` | Le fichier `.pfx` encodé en base64 : `[Convert]::ToBase64String([IO.File]::ReadAllBytes("lattis.pfx")) \| Set-Clipboard` |
| `WINDOWS_CERT_PASSWORD` | Mot de passe du `.pfx` |

Le Publisher du MSIX est alors lu dans le certificat : rien d'autre à déclarer.

Voie Azure Trusted Signing (utilisée seulement si `WINDOWS_CERT_PFX_BASE64` est absent) :

| Nom | Type | Contenu |
|---|---|---|
| `AZURE_TENANT_ID` | secret | Identifiant du locataire Entra ID |
| `AZURE_CLIENT_ID` | secret | Identifiant de l'application (service principal) |
| `AZURE_CLIENT_SECRET` | secret | Secret client de cette application |
| `AZURE_SIGNING_ENDPOINT` | variable | Point d'accès de la région du compte, par ex. `https://weu.codesigning.azure.net/` |
| `AZURE_SIGNING_ACCOUNT` | variable | Nom du compte Trusted Signing |
| `AZURE_SIGNING_PROFILE` | variable | Nom du profil de certificat |
| `WINDOWS_PUBLISHER` | variable | Sujet exact du certificat du profil (`CN=…, O=…, C=FR`) |

Facultatif, dans les deux cas :

| Variable | Contenu |
|---|---|
| `WINDOWS_PUBLISHER_DISPLAY_NAME` | Nom d'éditeur affiché par le MSIX (« Lattis » par défaut) |
| `WINDOWS_TIMESTAMP_URL` | Serveur d'horodatage RFC 3161 de la voie `.pfx` (`http://timestamp.digicert.com` par défaut) |

L'étape Trusted Signing utilise l'action `azure/trusted-signing-action@v0.5.1` ; si Microsoft l'a renommée ou a publié une version plus récente, mettre à jour les deux occurrences dans `release.yml`.

## 2. macOS

### Ce qu'il faut

1. **Apple Developer Program** (99 USD par an, en euros dans la boutique française), au nom de Louis ou d'une société. Le compte « Account Holder » est le seul à pouvoir créer un certificat Developer ID.
2. **Certificat « Developer ID Application »** : developer.apple.com → Certificates → « + » → Developer ID Application (profil G2 Sub-CA), à partir d'une demande de signature créée par Trousseaux d'accès sur un Mac. L'installer dans le trousseau, puis exporter le certificat **avec sa clé privée** au format `.p12`, protégé par un mot de passe.
3. **Clé API App Store Connect** pour la notarisation : App Store Connect → Utilisateurs et accès → Intégrations → App Store Connect API → clés d'équipe → « + », rôle **Developer**. Télécharger le fichier `AuthKey_XXXXXXXXXX.p8` (une seule fois possible), noter le **Key ID** (10 caractères) et l'**Issuer ID** (UUID affiché en haut de la page).

### Secrets GitHub

| Secret | Contenu |
|---|---|
| `MACOS_CERT_P12_BASE64` | Le `.p12` en base64 : `base64 -i DeveloperID.p12 \| pbcopy` |
| `MACOS_CERT_PASSWORD` | Mot de passe du `.p12` |
| `MACOS_SIGN_IDENTITY` | Facultatif : `Developer ID Application: Nom (TEAMID)` ; trouvé dans le `.p12` sinon |
| `MACOS_NOTARY_KEY` | Le **contenu** du fichier `.p8` (texte, lignes `-----BEGIN PRIVATE KEY-----` comprises) |
| `MACOS_NOTARY_KEY_ID` | Key ID de la clé API |
| `MACOS_NOTARY_ISSUER_ID` | Issuer ID |

Le certificat est importé dans un trousseau temporaire, supprimé en fin de job. Sans certificat, l'app est signée ad hoc : elle se lance sur le Mac qui l'a construite, mais Gatekeeper la refuse une fois téléchargée.

L'application est signée avec le hardened runtime et les droits de `build/darwin/entitlements.plist` (aucun droit d'exécution de code dynamique). Le binaire est universel (arm64 + amd64, macOS 12 minimum).

## 3. Linux (facultatif)

Une clé GPG dédiée aux releases :

```sh
gpg --quick-gen-key "Lattis Releases <adresse@exemple.fr>" rsa4096 sign 3y
gpg --armor --export-secret-keys "Lattis Releases" > lattis-release-secret.asc
gpg --armor --export "Lattis Releases" > lattis-release-key.asc   # clé publique à diffuser
```

| Secret | Contenu |
|---|---|
| `LINUX_GPG_PRIVATE_KEY` | Contenu de `lattis-release-secret.asc` |
| `LINUX_GPG_PASSPHRASE` | Sa phrase de passe |

Avec cette clé, nfpm signe les `.deb` (signature `_gpgorigin`, vérifiable par `dpkg-sig`/`debsig-verify`) et les `.rpm` (`rpm --checksig` après `rpm --import lattis-release-key.asc`), et la release reçoit `SHA256SUMS.txt`, une signature `.asc` par fichier et la clé publique `lattis-release-key.asc`. Vérification par un utilisateur :

```sh
gpg --import lattis-release-key.asc
gpg --verify SHA256SUMS.txt.asc SHA256SUMS.txt && sha256sum -c --ignore-missing SHA256SUMS.txt
```

L'AppImage n'a pas de signature intégrée : sa signature détachée `.asc` en tient lieu.

## 4. Mises à jour : signature de latest.json (Ed25519)

La mise à jour automatique installe un fichier seulement si son empreinte SHA-256 est celle de `latest.json`. Sans signature, quiconque peut modifier la release GitHub (compte ou jeton volé) peut changer le fichier **et** l'empreinte. Avec une clé, la CI signe `latest.json` (`latest.json.sig`) et l'application refuse toute version dont la signature manque ou ne vérifie pas avec la clé publique qu'elle embarque. Gratuit, indépendant des certificats ci-dessus, valable pour les trois systèmes.

**Mise en place (une fois) :**

```sh
go run ./cmd/lattis-updsign keygen          # écrit lattis-update.key, affiche la clé publique
gh secret set UPDATE_SIGNING_KEY < lattis-update.key
```

1. Coller la clé publique affichée dans `publishedKeys` (`internal/update/sign.go`), committer.
2. Ranger `lattis-update.key` hors du dépôt (gestionnaire de mots de passe), puis le supprimer du disque. Perdue, la clé ne se remplace qu'avec une version signée par l'ancienne (ou une réinstallation à la main).
3. Publier une version : elle est signée, et c'est la première qui exige des signatures.

**Transition.** Les versions déjà installées (jusqu'à celle qui précède la clé) ont une liste de clés vide : elles acceptent un `latest.json` non signé, comme avant, et installent donc la première version signée sans difficulté. Seules les versions qui embarquent la clé exigent ensuite la signature. Rien ne casse si la clé n'est jamais créée.

**Garde-fous de la CI** (`release.yml`, étape « Signature de latest.json ») : sans secret et sans clé dans le code, avertissement et rien n'est signé ; clé dans le code mais secret absent, ou secret qui ne correspond à aucune clé du code : le job échoue avant la release (sinon les installations refuseraient la version).

**Changer de clé :** ajouter la nouvelle clé publique à `publishedKeys` à côté de l'ancienne, publier une version signée par l'ancienne ; puis remplacer le secret et retirer l'ancienne clé quelques versions plus tard.

Vérifier une release : `go run ./cmd/lattis-updsign verify latest.json` (avec `latest.json.sig` à côté).

## 5. Publier une release

```sh
scripts/release.sh v0.2.0
```

Le workflow reporte la version du tag dans les fichiers de build (`build/release/set-version.sh`, sans commit), construit les trois plateformes en parallèle, puis publie la release `v0.2.0` (marquée « pré-version » si le tag contient un tiret, par ex. `v0.2.0-rc.1` ; brouillon seulement avec la variable `RELEASE_DRAFT`, voir [publier.md](publier.md)).

Pour essayer sans publier : Actions → **Release** → Run workflow, avec une version de test. Les paquets restent dans les artefacts du run.

## 6. Construire et signer en local

Commande commune, sur chaque système : `wails3 task release` (ajouter `VERSION=v0.2.0` pour reporter d'abord une version ; ne pas committer les fichiers ainsi modifiés). Résultats dans `bin/`.

### Windows

Prérequis : SDK Windows 10/11 (MakeAppx, MakePri, signtool), NSIS (`makensis` dans le PATH), PowerShell 7 (`pwsh`).

```powershell
# Sans certificat : paquets non signés
wails3 task release

# Avec un .pfx
$env:WINDOWS_CERT_PATH = 'C:\certs\lattis.pfx'
$env:WINDOWS_CERT_PASSWORD = '…'
wails3 task release

# Avec un certificat sur jeton (EV / OV récent) : empreinte SHA-1 du certificat
$env:WINDOWS_CERT_THUMBPRINT = '0123…'
wails3 task release
```

Tâches séparées : `windows:build`, `windows:sign:exe`, `windows:package:nsis`, `windows:package:msix` (les trois dernières ne reconstruisent pas `lattis.exe`). Le MSIX seul : `pwsh build/windows/msix/build-msix.ps1 [-CertPath … -CertPassword …] [-NoSign] [-KeepStage]`.

Essai complet de la chaîne sans vrai certificat, avec un certificat auto-signé créé en mémoire (rien n'est importé dans les magasins de Windows) :

```powershell
pwsh build/windows/new-test-cert.ps1 -Out $env:TEMP\lattis-test.pfx -Password test
pwsh build/windows/msix/build-msix.ps1 -CertPath $env:TEMP\lattis-test.pfx -CertPassword test
signtool verify /pa /v bin\lattis-*.msix   # « terminated in a root certificate which is not trusted » : attendu
```

Installer ce MSIX de test demanderait d'importer le certificat dans « Personnes autorisées » de l'ordinateur : à réserver à une machine de test.

### macOS

```sh
export MACOS_SIGN_IDENTITY="Developer ID Application: Louis Triboulin (TEAMID)"   # certificat du trousseau de session
# Notarisation : profil enregistré une fois pour toutes…
xcrun notarytool store-credentials lattis-notary --key AuthKey_XXXXXXXXXX.p8 --key-id XXXXXXXXXX --issuer <issuer-id>
export MACOS_NOTARY_PROFILE=lattis-notary
# … ou directement : MACOS_NOTARY_KEY_PATH, MACOS_NOTARY_KEY_ID, MACOS_NOTARY_ISSUER_ID
wails3 task release
spctl --assess --type execute -vv bin/lattis.app   # « source=Notarized Developer ID »
```

### Linux

```sh
export NFPM_SIGNING_KEY_FILE=$HOME/lattis-release-secret.asc NFPM_PASSPHRASE='…'   # facultatif
wails3 task release
```

## Fichiers concernés

- `.github/workflows/release.yml` : le workflow (le workflow `ci.yml` n'est pas modifié).
- `build/release/set-version.sh` : report de la version du tag.
- Mises à jour : `internal/update/sign.go` (clés publiques, vérification), `cmd/lattis-updsign` (clé, signature).
- Windows : `build/windows/sign.ps1`, `build/windows/sdk-tools.ps1`, `build/windows/new-test-cert.ps1`, `build/windows/msix/build-msix.ps1` et `AppxManifest.xml` (identité `fr.triboulin.lattis`, association `.lattis`, alias `lattis` en ligne de commande pour `lattis mcp` et `lattis config`, écriture des fichiers dans le vrai `%AppData%`), `build/windows/nsis/` (association `.lattis`, signature de l'installeur et du désinstalleur), `build/windows/Taskfile.yml`.
- macOS : `build/darwin/sign-notarize.sh`, `build/darwin/entitlements.plist`, `build/darwin/Info.plist` (type de document `fr.triboulin.lattis.workbook`), `build/darwin/Taskfile.yml`.
- Linux : `build/linux/nfpm/nfpm.yaml` (binaire dans `/usr/bin`, type MIME, signature), `build/linux/mime/lattis.xml`, `build/linux/lattis.desktop` et `build/linux/Taskfile.yml`.
