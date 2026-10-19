# Prêt à publier ? Liste de contrôle

À relire avant chaque version, et en entier avant la première (`v0.1.0-beta.1`). La marche à suivre pour publier est dans [publier.md](publier.md) ; la signature dans [signature.md](signature.md) et [choix-autorite.md](choix-autorite.md). État relevé le 19/10/2026 (audit 6, I14).

Légende : ✅ fait, ❌ à faire, ⚠️ accepté en connaissance de cause (à revoir).

## 1. Bloquant pour la première version

| | Point | État au 19/10/2026 | Comment vérifier |
|---|---|---|---|
| ❌ | La version s'affiche dans « À propos » | `bridge.Version` vaut `"dev"` : seule l'image du moteur web le fixe (`-X …/internal/bridge.Version=` dans `build/engine/Dockerfile`). Le workflow Release passe par `build/release/set-version.sh`, qui ne touche que les fichiers d'empaquetage. Ajouter `-X github.com/lalamefine/lattis/internal/bridge.Version=$VERSION` aux options de build de production (`build/*/Taskfile.yml`). | Installer l'artefact d'un essai (Actions › Release › Run workflow, `v0.0.0-test`) et ouvrir Fichier › À propos. |
| ❌ | `go.mod` propre | `go.yaml.in/yaml/v4` est utilisé directement mais marqué `// indirect` ; la modification locale de `go.mod` n'est pas commitée. Commiter `go mod tidy`, ou retirer l'usage direct. | `go mod tidy && git diff --exit-code go.mod go.sum` |
| ❌ | Licences des dépendances | `NOTICE.md` ne cite pas `microsoft/go-mssqldb`, `parquet-go`, `anthropic-sdk-go`, `google/uuid` ni `klauspost/compress`. | `go-licenses report ./...` (outil à lancer hors du dépôt), comparé à `NOTICE.md` ; côté frontend, `npx license-checker --production`. |
| ❌ | Chemin du module | `github.com/lalamefine/lattis`, alors que le dépôt public est `Be-Lattis/lattis-app`. À aligner avant le premier utilisateur externe (il apparaît dans les liaisons du frontend et les imports). | `head -1 go.mod` |
| ✅ | CI verte sur `main`, Linux et Windows | `.github/workflows/ci.yml` (push sur `main`, PR) : Go sous Ubuntu et Windows (`internal`, `engine`, `protocol`), svelte-check et vitest. | Onglet Actions, dernier run de `main`. |
| ✅ | Tests avant le tag | `scripts/release.sh` lance `go test ./internal/... ./engine/... ./protocol/...`. | — |
| ❌ | Essai du workflow Release | Aucune étiquette `v*` n'existe encore. Faire d'abord un essai sans release (`v0.0.0-test`), puis publier `v0.1.0-beta.1` (pré-version, jamais « Latest »). | Artefact `release-essai` du run ; release GitHub marquée « Pre-release ». |

## 2. Dépendances

| | Point | État | Décision |
|---|---|---|---|
| ⚠️ | Wails `v3.0.0-beta.30` | Bêta : l'API peut changer entre deux bêtas. | Garder la version épinglée (`WAILS_VERSION` dans les workflows) ; ne monter qu'avec un essai complet (build des trois OS, récupération, association de fichiers). |
| ⚠️ | `go.yaml.in/yaml/v4 v4.0.0-rc.2` | Version candidate. | Acceptable pour une bêta ; repasser en v3 ou attendre la v4 finale avant la 1.0. |
| ⚠️ | Go 1.26 imposé | `go 1.26.0` dans `go.mod`. | Sans effet pour l'utilisateur ; les contributeurs ont besoin de cette version. |
| ✅ | Frontend épinglé | `package-lock.json` commité, `npm ci` en CI. | — |

## 3. Livraison

| | Point | État |
|---|---|---|
| ⚠️ | Signature Windows | Voies `.pfx` et Azure Trusted Signing prêtes ; aucun certificat configuré. Décision : demander SignPath Foundation ([choix-autorite.md](choix-autorite.md)). Sans signature, SmartScreen avertit et le MSIX n'est pas publié. |
| ⚠️ | Signature macOS | Sans certificat Apple, le `.dmg` est signé ad hoc : ouverture manuelle (clic droit › Ouvrir). |
| ❌ | Contrôle de mise à jour | `latest.json` est publié mais l'application ne le lit jamais. À ajouter : lecture discrète au démarrage, désactivable par la configuration d'entreprise (`docs/configuration-entreprise.md`), canal bêta pour les étiquettes `-beta`. |
| ✅ | Notes de version | `CHANGELOG.md` tenu à la main, repris par `scripts/release-notes.sh`. Avant le tag : renommer « Non publié » en `## 0.1.0-beta.1 (JJ/MM/AAAA)`. |
| ✅ | Fichiers aux noms stables | Liens `releases/latest/download/<nom>` ([publier.md](publier.md)). |

## 4. Produit

À vérifier à la main sur l'installeur de l'essai, sous Windows et sous Linux au moins :

- [ ] Premier lancement : écran d'accueil, nouveau classeur, saisie d'un tableau, formule, Ctrl+Z, enregistrement, réouverture.
- [ ] Association de fichiers : double-clic sur un `.lattis`, lien `lattis://open`.
- [ ] Récupération : tuer le processus pendant une saisie, relancer, récupérer.
- [ ] Classeur reçu : un script ou une page web demande l'accord avant de s'exécuter.
- [ ] Import et export .xlsx d'un classeur réel, export CSV ouvert dans Excel.
- [ ] Paramètres : fonctions désactivées (pages web, assistant, serveur MCP) réellement absentes.
- [ ] Désinstallation propre (pas de fichier laissé hors du profil).
- [ ] « À propos » : version, licence MPL-2.0, lien vers le code source.

## 5. Documentation et juridique

| | Point | État |
|---|---|---|
| ✅ | Licence | `LICENSE` (MPL-2.0), `TRADEMARKS.md`, `NOTICE.md` (à compléter, voir §1). |
| ✅ | Aide intégrée | Pages hors ligne (F1), glossaire des termes (`docs/glossaire.md`). |
| ❌ | Format `.lattis` | Pas de description des tables `_lattis_*` ni de la politique de migration (audit 6, M15). |
| ⚠️ | Mentions de personnes dans le code | Des commentaires citent des décisions nominatives ; à remplacer par la raison de la décision avant d'ouvrir les contributions (audit 6, M12). |
