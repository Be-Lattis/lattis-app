# Audit Livraison et mise à jour (30/10/2026)

Suite de l'audit général du 30/10 (§ 8, note 2,5/5). Périmètre : `internal/update`, `.github/workflows/release.yml`, `scripts/release*.sh`, `build/` (NSIS, AppImage), releases publiées (0.5.0-beta). Étiquettes : **[corrigé]** (commit), **[arbitrage]** (décision de Louis), **[plus tard]** (estimation).

## Mesures

| Élément | Valeur | Source |
|---|---|---|
| `lattis.exe` de production | 67,0 Mo (91,6 Mo sans `-s -w`) | `go build -tags production -trimpath -ldflags "-w -s"`, ce poste |
| Composition (non strippé) | code 25,2 Mo ; table des lignes (`pclntab`) 17,4 Mo ; chaînes 8,0 Mo ; types 5,1 Mo ; interface embarquée 5,6 Mo ; communes (géo) 4,3 Mo | `go tool nm -size` |
| Installeur Windows 0.5.0-beta | 27,4 Mo (NSIS zlib, réglage par défaut) | `gh release view v0.5.0-beta` |
| Compression de `lattis.exe` | gzip -9 : 25,7 Mo ; xz -9 : 19,8 Mo | ce poste |
| AppImage / .deb / .rpm / .dmg | 100,2 / 25,9 / 26,6 / 50,6 Mo | release 0.5.0-beta |
| Fichiers de contrôle publiés | `SHA256SUMS.txt`, `latest.json` ; ni `.asc`, ni MSIX, ni signature du manifeste | release 0.5.0-beta |

`-s -w` et `-trimpath` sont **déjà** appliqués aux trois systèmes (`build/*/Taskfile.yml`) : les 67 Mo sont du code et des données réelles. UPX est écarté (faux positifs des antivirus, mémoire décompressée à chaque lancement, incompatible avec la signature Authenticode « propre »).

## Constats

### 1. [corrigé] Le manifeste des versions n'était pas signé — 9242baa3

La mise à jour vérifiait l'empreinte SHA-256 du fichier téléchargé, mais cette empreinte venait de `latest.json` (ou `SHA256SUMS.txt`) **dans la même release GitHub** : un compte ou un jeton volé pouvait remplacer l'installeur et son empreinte ensemble. Le contrôle Authenticode (`updater.go`, `Verify`) ne s'applique que si l'application est elle-même signée, ce qui n'est jamais le cas aujourd'hui.

Fait : la CI signe `latest.json` en Ed25519 (`latest.json.sig`, `cmd/lattis-updsign`, secret `UPDATE_SIGNING_KEY`) ; l'application vérifie la signature avec les clés publiques de `internal/update/sign.go` (`publishedKeys`) **avant** tout téléchargement, puis l'empreinte de l'installeur lue dans ce manifeste, sans repli sur `SHA256SUMS.txt`. Indépendant des certificats, gratuit, valable pour les trois systèmes.

**Transition.** `publishedKeys` est vide : rien ne change tant que Louis n'a pas créé la clé. Les versions installées (≤ 0.5.1) n'ont pas de clé et acceptent donc la première version signée ; seules les versions qui embarquent la clé exigent ensuite la signature. Garde-fous : le job Release échoue si une clé est dans le code et que le secret manque ou ne correspond pas (sinon les installations refuseraient la version), et `scripts/release.sh` le vérifie avant de taguer (`gh secret list`).

**À faire par Louis (5 min) :** `go run ./cmd/lattis-updsign keygen`, `gh secret set UPDATE_SIGNING_KEY < lattis-update.key`, coller la clé publique dans `publishedKeys`, ranger la clé privée hors du disque (`docs/release/signature.md` § 4).

Essais : `internal/update/sign_test.go` (signature valide, autre clé, manifeste modifié après signature, version non signée, repli `SHA256SUMS.txt` refusé, comportement inchangé sans clé, téléchargement de bout en bout) ; commande essayée à la main (clé absente, mauvaise clé, bonne clé, vérification).

### 2. [corrigé] Installeur Windows compressé en zlib — a316a42e

NSIS compressait par défaut en zlib. `SetCompressor /SOLID lzma` : environ 20 Mo au lieu de 27,4 (−27 %), pour chaque installation et chaque mise à jour automatique. Décompression un peu plus lente (1 à 2 s). **À vérifier** au prochain « Run workflow » de Release (makensis absent de ce poste).

### 3. [corrigé] Documentation de publication contradictoire — 9242baa3

`signature.md` disait que la release est créée en brouillon et montrait un `git tag` léger ; `publier.md` et le workflow publient directement (brouillon seulement avec `RELEASE_DRAFT`) et `scripts/release.sh` crée un tag annoté. Aligné.

### 4. [arbitrage] Signature de code : aucun paquet signé

SmartScreen avertit (« Windows a protégé votre ordinateur ») au premier lancement de l'installeur ; Gatekeeper refuse le `.dmg` téléchargé (clic droit › Ouvrir) ; le MSIX n'est pas publié faute de signature. Le workflow sait déjà utiliser chaque option : il suffit d'ajouter les secrets (`signature.md` §§ 1–3). Prix publics constatés, à confirmer à l'achat :

| Option | Coût / an | Mise en place | CI | Effet |
|---|---|---|---|---|
| **Azure Trusted Signing** (« Artifact Signing ») | ≈ 120 $ (Basic 9,99 $/mois) | Abonnement Azure, validation d'identité Microsoft ; **éligibilité** : organisations d'au moins 3 ans, individus de quelques pays seulement — à vérifier pour Louis en France | Déjà écrite (`azure/trusted-signing-action`) | Authenticode sur exe, installeur, MSIX ; réputation SmartScreen qui s'acquiert vite |
| **SignPath Foundation** (projets libres) | 0 € | Dépôt public sous MPL-2.0 : éligible a priori ; dossier à déposer, règles (build en CI uniquement, pas de binaire tiers non libre) ; éditeur affiché « SignPath Foundation » | Action GitHub SignPath, étape à écrire (½ j) | Même effet qu'un OV |
| Certificat **OV** (Certum, Sectigo, SSL.com…) | 70 à 400 € + HSM en ligne (≈ 100 à 250 €/an) pour la CI | Clé sur jeton matériel ou HSM depuis 2023 ; vérification d'identité | Jeton : signature locale seulement ; HSM en ligne : étape à écrire (½ j) | SmartScreen avertit jusqu'à ce que la réputation vienne |
| Certificat **EV** | 300 à 600 € | Comme OV, plus société | Comme OV | Plus de réputation immédiate depuis 2024 : **sans intérêt** face à OV |
| **Microsoft Store** (MSIX) | 0 € (individus), ≈ 99 $ une fois (sociétés) | Compte Partner Center, fiche de l'application | Envoi manuel ou API Store | Le Store signe le MSIX : installation sans avertissement, mises à jour par Windows ; ne couvre pas l'installeur `.exe` |
| **Apple Developer Program** | 99 €/an | Certificat Developer ID (.p12), clé API App Store Connect (.p8) | Déjà écrite (signature, notarisation, agrafage) | Plus de blocage Gatekeeper ; **préalable** à une mise à jour automatique sur macOS |
| **Clé GPG** | 0 € | `gpg --quick-gen-key`, 2 secrets | Déjà écrite | `.asc` et `.deb`/`.rpm` signés ; aucun effet sur l'ouverture |

Recommandation : GPG tout de suite (gratuit, 10 min) ; pour Windows, SignPath Foundation si le dossier passe, sinon Trusted Signing si Louis est éligible, sinon OV + HSM ; Apple Developer seulement si macOS compte (et avant toute mise à jour macOS). Store en complément éventuel.

### 5. [arbitrage] Version 0.5.1-beta non publiée

Notée au journal le 27/10. À taguer (`scripts/release.sh v0.5.1-beta`) sur décision de Louis ; idéalement après la clé du § 1 pour que les installations suivantes exigent la signature.

### 6. [plus tard] Mise à jour réelle sous macOS et Linux (≈ 3 à 4 j)

Aujourd'hui : la nouvelle version est signalée et sa page s'ouvre (`KindOther`). Le téléchargement contrôlé (`Client.Download`, empreinte du manifeste signé) est déjà indépendant du système ; manque l'installation :
- **AppImage** (1 j) : remplacer le fichier `$APPIMAGE` comme l'exécutable portable Windows (`replaceExe`), si son dossier est inscriptible.
- **.deb / .rpm** (1 à 2 j + **arbitrage hébergement**) : un dépôt APT/RPM (GitHub Pages ou serveur) laisse le système mettre à jour ; sinon téléchargement + `pkexec apt install`/`dnf install`, fragile.
- **macOS** (1 à 2 j) : remplacer `Lattis.app` depuis le `.dmg` ; sans signature Developer ID + notarisation, la copie est mise en quarantaine : dépend du § 4.

### 7. [plus tard] Retour arrière, « Ignorer cette version », miroir interne (≈ 2 j, comportement visible : **arbitrage** sur l'interface)

`download` refuse toute version non plus récente (« jamais de retour en arrière »). Un retour arrière voulu serait « Réinstaller la version précédente » (installeur de la release précédente, même contrôle de signature) dans Paramètres › Mises à jour. « Ignorer cette version » : une clé `skipped` dans `update.json`. Miroir interne pour les parcs sans accès à GitHub : un réglage d'entreprise `updates.url` (manifeste signé : le miroir n'a pas besoin d'être de confiance).

### 8. [plus tard] Contrôle Authenticode trop large (½ j)

Quand l'application sera signée, `VerifySignature` accepte un fichier signé par **n'importe quel** éditeur de confiance. À faire en même temps que le § 4 : comparer le sujet du certificat du fichier téléchargé à celui de `lattis.exe` (WTHelperProvDataFromStateData). Le manifeste signé (§ 1) couvre déjà l'essentiel du risque.

### 9. [plus tard] AppImage de 100 Mo (½ j, sur la CI Linux)

Le .deb, qui utilise le WebKit du système, fait 26 Mo ; le reste de l'AppImage est GTK 4, WebKitGTK 6 et leurs dépendances (dont ICU, ≈ 30 Mo de données). Pistes à mesurer sur la CI : `appimagetool --comp xz` contre le zstd par défaut (−10 à −20 % attendus, démarrage un peu plus lent) ; liste des 20 plus grosses bibliothèques de l'AppDir pour repérer ce qui est embarqué sans nécessité. Exclure ICU ou WebKit casserait les distributions non Debian : non.

### 10. [plus tard] Chaîne d'outils de la CI (1 h, à valider par un « Run workflow »)

- `patch-appdir.sh` télécharge `appimagetool` depuis la version **continuous**, sans version ni empreinte : figer une version et vérifier son SHA-256.
- Actions appelées par tag (`actions/checkout@v4`, `azure/trusted-signing-action@v0.5.1`…) et non par SHA ; Dependabot pour les tenir à jour.
- Attestations de provenance GitHub (`actions/attest-build-provenance`, gratuites) : `gh attestation verify` sur chaque fichier.

### 11. Non retenu

- **Taille du binaire** : rien de sûr à gagner sans changer le produit ; le plus gros poste est le code (pilotes SQL Server / PostgreSQL, SQLite en WASM, SDK Anthropic, Parquet). Charger les communes (4,3 Mo) à la demande : gain faible pour une complexité réelle.
- **UPX** : voir Mesures.
