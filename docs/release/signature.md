# Installeurs signés : ce qu'il faut fournir

Le workflow `.github/workflows/release.yml` construit, sur chaque tag `v*`, les installeurs de Lattis pour les trois systèmes et les publie dans une release GitHub (publique ; brouillon seulement avec la variable `RELEASE_DRAFT`, voir [publier.md](publier.md)). Chaque signature dépend d'un secret : s'il manque, l'étape correspondante est sautée avec un avertissement dans le journal et le paquet sort non signé. On peut donc lancer une release dès maintenant et ajouter les certificats un par un.

| Système | Fichiers produits | Signature | À fournir |
|---|---|---|---|
| Windows | `Lattis-windows-x64-setup.exe` (NSIS), `Lattis-windows-x64.msix` (publié seulement signé : non signé, Windows refuse de l'installer) | Authenticode SHA-256 horodaté (signtool) : `lattis.exe`, installeur (et désinstalleur avec eSigner ou le `.pfx`), paquet MSIX | Un certificat de signature de code : SSL.com eSigner (recommandé), Azure Trusted Signing **ou** `.pfx` |
| macOS | `Lattis-macos-universal.dmg` (arm64 + Intel) | Developer ID + hardened runtime, notarisation Apple, ticket agrafé à l'app et au `.dmg` | Adhésion Apple Developer Program, certificat « Developer ID Application » (.p12), clé API App Store Connect (.p8) |
| Linux | `Lattis-linux-x64.AppImage`, `Lattis-linux-x64.deb`, `Lattis-linux-x64.rpm` | Signature GPG intégrée aux `.deb`/`.rpm` ; signature détachée `.asc` de chaque fichier de la release et de `SHA256SUMS.txt` | Facultatif : une clé GPG |

Tous les paquets associent l'extension **`.lattis`** à Lattis (double-clic : le classeur s'ouvre). L'ancienne extension `.slitable` n'est plus lue par l'application depuis le renommage ; elle n'est donc associée nulle part.

## 1. Windows

### Ce qu'il faut choisir

Depuis juin 2023, les autorités de certification ne délivrent plus de certificat de signature de code (OV comme EV) dont la clé privée soit exportable en `.pfx` : la clé vit sur un jeton matériel ou dans un HSM en ligne. Le comparatif des autorités et le choix motivé sont dans [choix-autorite.md](choix-autorite.md). Le workflow accepte trois sources, dans cet ordre de préférence ; la première dont les secrets existent est utilisée, sinon les paquets sortent non signés comme avant :

1. **`.pfx`** (`WINDOWS_CERT_PFX_BASE64`) : ancien certificat exportable, ou certificat de test.
2. **SSL.com eSigner** (`ESIGNER_*`), **recommandé** : certificat OV (société) ou IV (personne physique) conservé dans le HSM cloud de SSL.com. Sur le runner, eSigner CKA présente ce certificat dans le magasin Windows comme un jeton USB virtuel ; signtool l'utilise par son empreinte. Tout est signé : `lattis.exe`, désinstalleur et installeur NSIS, MSIX (Publisher lu dans le certificat). Coût ≈ 129 $/an de certificat + 180 $/an de forfait eSigner (20 signatures/mois). Chaque run signé consomme **4 signatures** ; pour un essai manuel sans les consommer, décocher « sign » dans Actions → Release → Run workflow.
3. **Azure Trusted Signing** (« Artifact Signing » depuis 2026), conservé en alternative : environ 10 $/mois, mais réservé aux organisations des États-Unis, du Canada, de l'UE et du Royaume-Uni ayant au moins trois ans d'historique vérifiable, et aux particuliers des États-Unis et du Canada.
   - Créer une inscription d'application (Entra ID) avec un secret client, et lui donner le rôle **Trusted Signing Certificate Profile Signer** sur le compte de signature.
   - Noter le **sujet** du certificat du profil (portail Azure, profil de certificat → « Subject name », de la forme `CN=Nom, O=Nom, L=Ville, S=Région, C=FR`) : il devient l'éditeur (Publisher) du MSIX, qui doit lui être identique au caractère près.
   - Limite : le désinstalleur NSIS (embarqué dans l'installeur) n'est pas signé ; l'installeur, `lattis.exe` et le MSIX le sont.

Un certificat sur jeton matériel physique sert à signer **en local** (voir § 5, `WINDOWS_CERT_THUMBPRINT`).

Le MSIX signé avec un certificat public s'installe d'un double-clic (Windows 10 1903 ou plus récent). SmartScreen peut afficher un avertissement tant que l'éditeur n'a pas acquis de réputation, y compris avec un certificat EV.

### SSL.com eSigner : les démarches, pas à pas

Ce que Louis fait lui-même (personne ne peut le faire à sa place : compte, identité, paiement) :

1. **Créer le compte** sur [ssl.com](https://secure.ssl.com/users/new). Noter le **nom d'utilisateur** et le **mot de passe** : ce sont ceux que la CI utilisera.
2. **Commander le certificat**, durée 1 an (validité maximale actuelle) :
   - Lattis porté par une société ou une entreprise individuelle immatriculée : **OV Code Signing** ([achat](https://www.ssl.com/certificates/code-signing/buy/)) ; l'éditeur affiché sera le nom de la société.
   - Sinon, en nom propre : **IV Code Signing** ([fiche](https://ssl.com/products/software-integrity/code-signing/iv/)) ; l'éditeur affiché sera « Louis Triboulin ».
   - Stockage de la clé : **eSigner Cloud Signing** (pas de YubiKey). Ne pas prendre la validation express (599 $).
   - Payer par carte (129 $ HT).
3. **Fournir les pièces de validation** dans le tableau de bord de la commande (3 à 5 jours ouvrés) :
   - OV : lien vers l'immatriculation dans un registre public (fiche [annuaire-entreprises.data.gouv.fr](https://annuaire-entreprises.data.gouv.fr) ou extrait Kbis), adresse et téléphone de l'entreprise vérifiables ; si l'entreprise a moins de 3 ans, scan recto verso de la pièce d'identité du demandeur. SSL.com peut rappeler au numéro public de l'entreprise.
   - IV : scan recto verso de la carte d'identité ou du passeport (le numéro peut être masqué ; nom, adresse, année de naissance et photo visibles) et une photo du demandeur.
4. **Activer eSigner** une fois le certificat émis : dans la commande, définir le code PIN eSigner, puis afficher le **QR code** de l'authentification à deux facteurs. Sous le QR code figure le **secret TOTP** (texte en base32) : le copier dans un gestionnaire de mots de passe, c'est lui que la CI utilise pour produire ses codes à usage unique. Scanner aussi le QR code dans une application d'authentification pour signer à la main si besoin.
5. **Désactiver le « Malware Blocker »** d'eSigner s'il est proposé dans les réglages du certificat : signtool ne transmet que l'empreinte des fichiers, et le blocage fait échouer la signature (`SignerSign() failed … 0x80090003`).
6. **Souscrire le forfait eSigner** IV/OV **Tier 1 annuel** (180 $/an, 20 signatures par mois, report des signatures non utilisées). Les 30 premiers jours sont gratuits et illimités : on peut tester avant de payer.
7. **Créer les trois secrets GitHub** (ci-dessous), puis lancer Actions → **Release** → Run workflow : le journal du job Windows doit afficher « Certificat eSigner : CN=… » et la vérification des signatures doit passer.

Tout le reste est déjà en place dans le workflow (installation d'eSigner CKA, signature, éditeur du MSIX, nettoyage).

### Secrets et variables GitHub

Dans le dépôt : **Settings → Secrets and variables → Actions**, ou en ligne de commande depuis le dossier du dépôt (`gh` demande la valeur sans l'afficher) :

```sh
gh secret set ESIGNER_USERNAME
gh secret set ESIGNER_PASSWORD
gh secret set ESIGNER_TOTP_SECRET
```

Voie SSL.com eSigner (utilisée si `WINDOWS_CERT_PFX_BASE64` est absent) :

| Nom | Type | Contenu |
|---|---|---|
| `ESIGNER_USERNAME` | secret | Nom d'utilisateur du compte SSL.com |
| `ESIGNER_PASSWORD` | secret | Mot de passe du compte SSL.com |
| `ESIGNER_TOTP_SECRET` | secret | Secret TOTP d'eSigner (texte affiché sous le QR code, étape 4) |
| `ESIGNER_MODE` | variable, facultative | `product` par défaut ; `sandbox` pour un compte de test sur sandbox.ssl.com |

Le Publisher du MSIX est lu dans le certificat chargé : rien d'autre à déclarer. L'horodatage passe par `http://ts.ssl.com` (sauf `WINDOWS_TIMESTAMP_URL`). La version d'eSigner CKA téléchargée est fixée par `ESIGNER_CKA_VERSION` dans `release.yml` ([versions publiées](https://github.com/SSLcom/eSignerCKA/releases)). En cas d'échec, les journaux d'eSigner CKA sont déposés en artefact `esigner-cka-logs`.

Voie `.pfx` (prioritaire si présente) :

| Secret | Contenu |
|---|---|
| `WINDOWS_CERT_PFX_BASE64` | Le fichier `.pfx` encodé en base64 : `[Convert]::ToBase64String([IO.File]::ReadAllBytes("lattis.pfx")) \| Set-Clipboard` |
| `WINDOWS_CERT_PASSWORD` | Mot de passe du `.pfx` |

Le Publisher du MSIX est alors lu dans le certificat : rien d'autre à déclarer.

Voie Azure Trusted Signing (utilisée seulement sans `.pfx` ni secrets eSigner) :

| Nom | Type | Contenu |
|---|---|---|
| `AZURE_TENANT_ID` | secret | Identifiant du locataire Entra ID |
| `AZURE_CLIENT_ID` | secret | Identifiant de l'application (service principal) |
| `AZURE_CLIENT_SECRET` | secret | Secret client de cette application |
| `AZURE_SIGNING_ENDPOINT` | variable | Point d'accès de la région du compte, par ex. `https://weu.codesigning.azure.net/` |
| `AZURE_SIGNING_ACCOUNT` | variable | Nom du compte Trusted Signing |
| `AZURE_SIGNING_PROFILE` | variable | Nom du profil de certificat |
| `WINDOWS_PUBLISHER` | variable | Sujet exact du certificat du profil (`CN=…, O=…, C=FR`) |

Facultatif, dans tous les cas :

| Variable | Contenu |
|---|---|
| `WINDOWS_PUBLISHER_DISPLAY_NAME` | Nom d'éditeur affiché par le MSIX (« Lattis » par défaut) |
| `WINDOWS_TIMESTAMP_URL` | Serveur d'horodatage RFC 3161 des voies `.pfx` et eSigner (`http://timestamp.digicert.com` pour le `.pfx`, `http://ts.ssl.com` pour eSigner, par défaut) |

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

## 4. Publier une release

```sh
git tag v0.2.0
git push origin v0.2.0
```

Le workflow reporte la version du tag dans les fichiers de build (`build/release/set-version.sh`, sans commit), construit les trois plateformes en parallèle, puis crée la release `v0.2.0` en brouillon (marquée « pré-version » si le tag contient un tiret, par ex. `v0.2.0-rc.1`). La relire dans GitHub → Releases, puis publier.

Pour essayer sans publier : Actions → **Release** → Run workflow, avec une version de test. Les paquets restent dans les artefacts du run.

## 5. Construire et signer en local

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

- `.github/workflows/release.yml` : le workflow (le workflow `ci.yml` n'est pas modifié) ; choix de l'autorité Windows : [choix-autorite.md](choix-autorite.md).
- `build/release/set-version.sh` : report de la version du tag.
- Windows : `build/windows/sign.ps1`, `build/windows/sdk-tools.ps1`, `build/windows/new-test-cert.ps1`, `build/windows/msix/build-msix.ps1` et `AppxManifest.xml` (identité `fr.triboulin.lattis`, association `.lattis`, alias `lattis` en ligne de commande pour `lattis mcp` et `lattis config`, écriture des fichiers dans le vrai `%AppData%`), `build/windows/nsis/` (association `.lattis`, signature de l'installeur et du désinstalleur), `build/windows/Taskfile.yml`.
- macOS : `build/darwin/sign-notarize.sh`, `build/darwin/entitlements.plist`, `build/darwin/Info.plist` (type de document `fr.triboulin.lattis.workbook`), `build/darwin/Taskfile.yml`.
- Linux : `build/linux/nfpm/nfpm.yaml` (binaire dans `/usr/bin`, type MIME, signature), `build/linux/mime/lattis.xml`, `build/linux/lattis.desktop` et `build/linux/Taskfile.yml`.
