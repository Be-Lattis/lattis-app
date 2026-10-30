# Audit général de Lattis (30/10/2026)

*Lecture seule, sur `main` au commit 32ed3f58. But : noter chaque aspect de l'application, dire ce qui tient et ce qui faiblit, et préparer les audits spécialisés à lancer ensuite. Rien n'a été corrigé.*

**Mesures faites pour cet audit** (poste de développement, Windows 10, Go 1.26.0) :

| Mesure | Résultat |
|---|---|
| `go vet ./internal/... ./engine/... ./protocol/...` | propre, 27 s |
| `go test -count=1` (mêmes paquets) | tout vert, 3 min 10 s ; les plus lents : `canvas` 171 s, `integration` 152 s, `bridge` 77 s, `doc` 75 s, `api` 74 s |
| `vitest run` | 166 fichiers, 1 661 essais verts, 22 s |
| Essais instables, `-count=10` | `TestShareFrozenWorkbook` 2 échecs sur 10 (`sharing_test.go:162`), `TestLayoutFollowed` 1 sur 10 (`layout_test.go:275`) ; les 5 autres de la liste : 0 sur 10 |
| `go test -race` (coedit, remote/collab, store, grid, formula, canvas, sharing, remote/workbook ; 11 min) | **aucune course détectée** ; `remote/collab` échoue sous `-race` (4 essais à délai : `TestAutoShareRelayAndListen`, `TestChoiceTabFollowed`, `TestFollowStopsWhenDeleted`, `TestLayoutFollowed`) ; `canvas` dépasse le délai de 10 min de `go test` (arrêté dans `TestRangeSheetCopyDelete`) |
| CI GitHub (`ci.yml`, 60 derniers passages) | 33 réussis, 14 échoués, 13 annulés ; durée ≈ 8 min ; les échecs vus sont tous `TestShareFrozenWorkbook` ou `TestShareRemoveMember` (et `TestFormInfo` une fois) |
| `go build` production (Windows) | `lattis.exe` 67,0 Mo ; `lattis-engine` (web) 59,0 Mo |
| `npm run build` | 78 s ; `dist` 5,6 Mo ; JS 5,1 Mo (1,6 Mo gzip) ; **un bloc de 2,82 Mo** (878 Ko gzip) ; avertissement : import dynamique de CodeMirror sans effet (`onboarding/steps.ts`) |
| Release 0.5.0-beta | installeurs Windows 27 Mo, portable 67 Mo, `.dmg` 51 Mo, **AppImage 100 Mo**, `.deb`/`.rpm` 26 Mo ; **ni `.asc`, ni MSIX** : aucune signature publiée |
| Taille du code | Go : 183 k lignes hors essais, 67 k lignes d'essais (389 fichiers) ; frontend : 98 k lignes, 13 k lignes d'essais |

## Synthèse

| # | Aspect | Note /5 | Audit spécialisé |
|---|---|---|---|
| 1 | Fonctionnalités, parité tableur | 4 | Oui : formules dynamiques et écarts restants |
| 2 | UX, ergonomie, i18n | 3,5 | Oui : parcours clavier, accessibilité, longueur des textes |
| 3 | Performance | 3,5 | Oui : calculs quadratiques, TCD sur source vive, bundle |
| 4 | Stabilité, fiabilité | 3,5 | Oui : essais instables et CI |
| 5 | Sécurité | 4 | Non (audit du 26/10 en cours de décision) ; une vérification ciblée « mise à jour » |
| 6 | Collaboration, Serveur Lattis | 3,5 | Oui : suivi temps réel sous charge et délais |
| 7 | Qualité du code, maintenabilité | 2,5 | Oui : découpage `Sheet.svelte` et `DocumentService` |
| 8 | Distribution | 2,5 | Oui : signature et mise à jour macOS / Linux |
| 9 | Assistant IA | 4 | Non (P2/P3 déjà listés) ; plus tard : banc d'essai de réponses |
| 10 | Documentation, aide | 4 | Non ; une passe de raccourcissement suffit |

Moyenne : 3,55. Le produit est riche et testé ; les faiblesses sont surtout **structurelles** (un fichier d'interface de 15 500 lignes, un service à 502 méthodes), **de livraison** (rien n'est signé) et **de fiabilité des essais** (une CI sur quatre échoue sans régression réelle).

---

## 1. Fonctionnalités visibles, parité avec un tableur : 4/5

**Ce qui est bien**
- Le chantier de parité (« audit, points 2 à 16 », commits du 25 au 26/10) a livré : collage spécial, polices et bordures, formats personnalisés, liens hypertexte, images et formes, .xlsx (mise en forme, MFC, notes, validations, graphiques natifs, TCD à l'import), import .ods, plan et sous-totaux, valeur cible / scénarios / solveur, validation complète, TCD à plusieurs champs en colonnes et groupements, graphiques (tendances, barres d'erreur, bulles, boursier, sparklines), rechercher / remplacer avancé, commentaires en fil, ~80 fonctions ajoutées.
- Ce qu'un tableur classique n'a pas : tableaux typés SQLite, onglet SQL, Power Query, scripts Lua, MCP, pages HTML, formulaires, classeurs partagés.
- Les critiques de l'audit du 19/10 sur les fonctions sont réglées (graphiques en SQL sur toutes les lignes, 2 000 points signalés : 23774625 ; historique non enregistré et verrou `~$` : adeaa887).
- `docs/fonctions.md` documente chaque écart (✅ / ⚠️ / ❌).

**Ce qui faiblit**
- Fonctions de tableaux dynamiques récentes absentes du moteur : `LAMBDA`, `BYROW`/`MAP`/`REDUCE`/`SCAN`/`MAKEARRAY`, `VSTACK`/`HSTACK`, `CHOOSECOLS`/`CHOOSEROWS`, `WRAPROWS`, `XMATCH`, `REGEXTEST`/`REGEXEXTRACT`, `PIVOTBY` (aucune occurrence dans `internal/formula`). Elles arrivent par les classeurs Excel récents.
- `RECHERCHEH` refusée (`docs/fonctions.md:154`), `RECHERCHEV`/`RECHERCHEX` en correspondance exacte seulement (`:149`), `SOUS.TOTAL` ignore le filtre de la grille (`:94`), `PRENDRE`/`EXCLURE` sans 3ᵉ argument (`:199`), `INDIRECT` sans L1C1 (`:156`).
- Pas d'export PDF direct : on passe par l'impression (« Microsoft Print to PDF », clés `printPdf`).
- Fidélité .xlsx aller-retour non mesurée sur un corpus de vrais fichiers (les essais sont unitaires).

**Audit spécialisé : oui.** *Parité formules et fichiers Excel* : (1) liste chiffrée des fonctions Excel 365 manquantes ou restreintes, classées par fréquence d'usage ; (2) aller-retour .xlsx sur un corpus de 20 à 30 classeurs réels (formules, MFC, TCD, graphiques, noms), avec ce qui se perd ; (3) proposition d'ordre pour LAMBDA et les fonctions de tableaux.

## 2. UX, ergonomie, i18n : 3,5/5

**Ce qui est bien**
- Cinq langues complètes côté interface : 6 025 clés dans chacune (fr, en, de, es, zh) ; catalogues Go `internal/msg/*.go` de même taille ; aide en 15 pages par langue.
- Textes courts dans l'ensemble : médiane 20 caractères, 90 % sous 70, 27 messages au-delà de 150 sur 5 100.
- Accélérateurs du ruban (`ui/keytips.ts`), piège du focus (`ui/focus.ts`), annonce de la cellule active (`GridA11y.svelte`), visite guidée (`onboarding/`), mode sombre documenté (`docs/design/mode-sombre.md`), réglages imposés grisés avec cadenas.

**Ce qui faiblit**
- Erreurs de l'assistant **en français dans toutes les langues** : `internal/ai/anthropic.go:227-234`, `internal/ai/agent.go:94,171,231`, `internal/ai/ai.go:191`, 14 dans `openai.go` (des `errors.New` hors `internal/msg`). En tout, ~126 `fmt.Errorf`/`errors.New` en français hors catalogue (une partie dans des faux serveurs d'essai).
- Messages longs pour un utilisateur pressé : `shell:share.addTableConfirm` 292 car., `grid:pivot.handKept` 248, `grid:local.tooltip` 242 ; 41 messages Go de plus de 200 car. dans `internal/msg/fr.go`. La page d'aide `prise-en-main.md` fait 4 900 mots.
- 106 avertissements d'accessibilité Svelte masqués (`svelte-ignore a11y_*`), dont 24 dans `Sheet.svelte` et 7 dans `Ribbon.svelte` : éléments cliquables sans rôle ni clavier.
- Aucun essai d'interaction de bout en bout (pas de Playwright) : la cohérence menus / ruban / raccourcis ne se vérifie qu'à la main.

**Audit spécialisé : oui.** *Ergonomie et accessibilité* : (1) parcours au clavier seul et au lecteur d'écran (NVDA) des 10 tâches courantes, avec les 106 `svelte-ignore` triés ; (2) relevé des textes de plus de 120 caractères (interface, `internal/msg`, aide) avec une version courte proposée ; (3) cohérence ruban / menus contextuels / raccourcis (même commande, même nom, même raccourci) et erreurs non traduites.

## 3. Performance : 3,5/5

**Ce qui est bien**
- Travail mesuré et suivi : `docs/mesures/performances-1M.md`, réflexion du 21/10 (pistes A, B, C faites), « vue d'abord » (premier dessin à 10 M de lignes sans attendre un `Layout` de 30 s), retour visuel avec Annuler, publication 10 M suivie et annulable.
- Grille virtuelle : 1 M de lignes défile ; saisie dans un tableau de 1 M : 1 s → 2 ms après la piste A.
- Démarrage de la version web : blocs visibles 375 ms après le clic à 1 M de lignes.

**Ce qui faiblit**
- **Critères de comparaison par ligne quadratiques** (piste E du 21/10, aucun commit trouvé depuis) : `SOMME.SI(T[Qté];">"&[@Qté];…)` 8,4 s à 3 000 lignes, plus de 8 min à 30 000 ; même famille : RANG, NB.SI.ENS avec `<`, `>=`, jokers.
- TCD ou graphique sur un grand tableau : recalcul complet après chaque saisie dans la source (6,5 s à 1 M, piste D ; l'ancienne valeur reste affichée, mais le calcul tourne).
- `Layout` complet à 10 M : 30 s en arrière-plan ; tri 1 M : 8,8 à 12 s ; pilote SQLite en WebAssembly environ deux fois plus lent que le natif (piste G non tranchée).
- Frontend : un seul bloc de 2,82 Mo (878 Ko gzip) chargé au démarrage ; CodeMirror et Quill ne sont pas séparés.
- Démarrage du bureau et mémoire à 1 M / 10 M jamais mesurés (aucun chiffre dans `docs/mesures`).

**Audit spécialisé : oui.** *Performance des calculs et du démarrage* : (1) piste E (critères `<`/`>` et RANG par ligne : fenêtrage SQL ou tri + cumuls) et piste D (TCD incrémental) chiffrées ; (2) démarrage à froid et mémoire du bureau avec 0, 1 M et 10 M de lignes ; (3) découpage du bloc de 2,8 Mo (CodeMirror, Quill, cartes chargés à la demande).

## 4. Stabilité, fiabilité : 3,5/5

**Ce qui est bien**
- Fichier robuste : enregistrement atomique, copie de travail récupérée après plantage (`archive.Recover`, `doc.Recover`), sauvetage d'un fichier abîmé (`archive/salvage.go`), verrou `~$`.
- Paniques rattrapées et journalisées avec message lisible (`internal/bridge/panic.go`).
- `go vet` propre, aucune course de données détectée par `-race` sur huit paquets centraux.

**Ce qui faiblit**
- **CI instable** : 14 échecs sur 60 passages, tous dus à des essais à délai (`TestShareFrozenWorkbook`, `TestShareRemoveMember`, `TestFormInfo`) et non à une régression. Reproduit ici : `TestShareFrozenWorkbook` échoue 2 fois sur 10 en local. Cause probable : `sharing_test.go:155-160` lit `ShareState().ReadOnly` puis agit, alors que le suivi d'Alice relit ses droits en même temps (contrôle puis action non atomique sur un faux serveur partagé par `current`).
- `remote/collab` : 4 essais échouent sous `-race` (attentes « jamais atteint » trop courtes quand l'exécution ralentit) ; `TestLayoutFollowed` 1 sur 10 sans `-race`.
- 47 `time.Sleep` dans les essais (`coedit/enligne_test.go` 6, `bridge/events_test.go` 6…).
- Paquets d'essais lents (`canvas` 171 s, `integration` 152 s) : la CI Windows tourne près de 8 min ; sous `-race`, `canvas` dépasse les 10 min par défaut, ce qui interdit d'ajouter `-race` à la CI en l'état.

**Audit spécialisé : oui.** *Fiabilité des essais et de la CI* : (1) chaque essai à délai de `bridge`, `remote/collab`, `coedit` lancé 50 fois avec et sans `-race`, sous Windows et Linux, cause de chaque échec ; (2) remplacement des `time.Sleep` et attentes fixes par des signaux ; (3) objectif : CI verte 50 fois de suite et sous 5 min (essais lents marqués ou parallélisés).

## 5. Sécurité : 4/5

L'audit de sécurité du 26/10 n'est pas dans `docs/` ; ses corrections H1 (fichiers de la session seulement, web et démo), M2 (bornes d'import .ods/.xlsx), M4 (`Guard.DialContext` pour PostgreSQL, MySQL, SQL Server) et, côté serveur, M7 (`LATTIS_TRUSTED_PROXIES`) sont livrées (CHANGELOG 0.5.1, commits b0874f4c, fc90c921, 1702749e). Les autres points attendent les décisions de Louis : **non refaits ici**.

**Ce qui est bien (constaté)**
- Lua : liste blanche de bibliothèques, `string.dump` retiré, `string.rep` borné, pile et registre bornés, tas limité à 512 Mo, délai de 30 s (`internal/script/sandbox.go:49-85`, `script.go:74`).
- MCP : écoute sur 127.0.0.1 seulement, jeton comparé en temps constant, origines non locales refusées, corps borné à 16 Mo (`internal/mcp/http.go:95-170`).
- Pages HTML : `iframe sandbox="allow-scripts"` sans même origine, CSP stricte, accord avant exécution (`page/HtmlPage.svelte:346-366`, `page/bridge.ts:36`).

**Constats nouveaux (à rapprocher de l'audit du 26/10)**
- **Mise à jour sans contrôle de signature en pratique** : la vérification Authenticode n'est active que si l'exécutable en cours est signé (`internal/update/updater.go:166`) ; aucune release n'est signée, donc seul le SHA-256 de `latest.json`, publié au même endroit que l'installeur, protège l'installation automatique. La signature Ed25519 de `latest.json` est dans `docs/feuille-de-route.md`, pas faite. Hors Windows, `VerifySignature` rend `ErrUnsupported` (`platform_other.go:16`).
- Rien d'autre de nouveau à niveau moyen ou plus dans ce qui a été lu (MCP, Lua, pages, impression).

**Audit spécialisé : non** pour la sécurité générale. À la place, une vérification ciblée *chaîne de mise à jour* dans l'audit Distribution (§ 8).

## 6. Collaboration, Serveur Lattis : 3,5/5

**Ce qui est bien**
- Modèle complet : classeurs partagés, droits par équipe et par ligne, publication figée ou collaborative, édition hors ligne (décision 0014), écritures optimistes (0012), coédition sur dossier partagé (0011), version web `lattis-engine`.
- Audit du 20/10 : A1 à A6 corrigés (1d167a48, ccfd8514), essais gardés en non-régression ; relais `/api/events` utilisé avec repli sur LISTEN (d3383781).
- Publication 10 M de lignes suivie, annulable, mémoire bornée.

**Ce qui faiblit**
- Points serveur S2 (`ready` envoyé sans écoute), S3 (coût du relais SSE à l'échelle), F2 (flux après expiration du jeton) : aucun commit identifié dans `be-lattis-server` depuis le 20/10.
- Montée en charge (C1) mesurée à 40 postes seulement ; une connexion PostgreSQL par poste et par classeur reste le schéma de repli.
- Les essais de suivi temps réel sont les plus instables de la base (§ 4), ce qui rend une régression réelle difficile à voir.
- Version web : échanges en JSON (≈ 5 ms par bloc de 200 lignes) ; format binaire en feuille de route.

**Audit spécialisé : oui.** *Collaboration sous charge* : (1) 100 à 300 postes simulés sur un serveur (latence de propagation, connexions PostgreSQL, mémoire de lattisd) ; (2) état de S2, S3, F2 et du passage complet sur `/api/events` ; (3) coupures réseau et reprises (hors ligne long, jeton expiré, structure changée entre-temps) en essais de bout en bout.

## 7. Qualité du code, maintenabilité : 2,5/5

**Ce qui est bien**
- Rapport essais / code correct : 67 k lignes d'essais Go pour 183 k, 1 661 essais vitest ; `go vet` propre ; logique pure sortie de la grille dans une quarantaine de modules `.ts` testés.
- Décisions écrites (`docs/decisions/0001` à `0018`), journal tenu, conventions i18n claires (`docs/i18n.md § 0`).
- Dépendances Go et npm peu nombreuses et choisies (pas de framework UI, pas de cgo pour SQLite).

**Ce qui faiblit**
- **`Sheet.svelte` : 15 511 lignes**, contre 10 400 quand le plan de découpage a été écrit le 19/10 (`docs/decoupage-sheet.md:7` ; objectif : sous 4 000). Le fichier a grossi de 50 % en dix jours ; aucune étape du plan n'est faite.
- `routes/+page.svelte` 4 116 lignes, `Ribbon.svelte` 2 504.
- `DocumentService` : **502 méthodes** réparties dans 55 fichiers de `internal/bridge` ; `internal/canvas` : 121 fichiers, 52 k lignes dans un seul paquet.
- Aucun essai de composant interactif ni de bout en bout : les régressions de la grille ne sont vues qu'à la main.
- Wails v3 encore en bêta (`v3.0.0-beta.30`) : risque de rupture à chaque montée.

**Audit spécialisé : oui.** *Architecture du frontend et du pont* : (1) reprendre `docs/decoupage-sheet.md` sur l'état actuel (15 500 lignes) : zones, couplages, étapes de moins d'une demi-journée ; (2) découpage de `DocumentService` par domaine (fichier, partage, IA, données…) et de `canvas` ; (3) socle d'essais de bout en bout (Playwright sur la version web) pour 10 parcours.

## 8. Distribution : 2,5/5

**Ce qui est bien**
- Chaîne complète par tag (`release.yml`, ≈ 12 min) : NSIS par utilisateur et par machine, portable, `.dmg` universel, AppImage, `.deb`, `.rpm`, `SHA256SUMS.txt`, `latest.json`.
- Mise à jour silencieuse Windows corrigée et vérifiée sur poste (21/10), échec affiché, arrêt après deux échecs.
- Réglages d'entreprise (stratégie, Registre, `LATTIS_*`) documentés.

**Ce qui faiblit**
- **Aucun paquet signé** : pas de certificat Authenticode, pas de notarisation Apple, pas de clé GPG (release 0.5.0-beta : ni `.asc` ni MSIX). SmartScreen et Gatekeeper bloquent ou avertissent au premier lancement.
- Mise à jour réelle sous macOS et Linux non faite (`docs/feuille-de-route.md`), ni retour arrière, ni miroir interne, ni « ignorer cette version ».
- AppImage de 100 Mo (le binaire en fait 67).
- 0.5.1-beta notée au journal (27/10) mais non publiée : à publier sur décision de Louis.

**Audit spécialisé : oui.** *Livraison et mise à jour* : (1) choix et coût de la signature (Azure Trusted Signing ou OV, Apple Developer, GPG) et effet sur SmartScreen / Gatekeeper ; (2) chaîne de confiance de la mise à jour (Ed25519 sur `latest.json`, contrôle hors Windows, retour arrière) ; (3) mise à jour macOS et Linux, taille de l'AppImage.

## 9. Assistant IA : 4/5

L'audit du 28/10 et sa suite (P1 faits le 29/10) couvrent le sujet. **Ce qui reste** (`docs/audits/audit-assistant-2026-10-28.md:226`) :
- Descriptions d'outils propres à l'assistant (≈ −40 %) : le fixe par appel reste à 23,3 k jetons.
- Définition des TCD et graphiques en *schema-only* dans `sheets_get` ; cellule active dans le contexte.
- Statistiques en *sample* / *full*, échantillon réparti, jeu d'outils réduit pour les modèles locaux, consignes MCP.
- Nouveau ici : erreurs des fournisseurs non traduites (§ 2) ; aucun banc d'essai de réponses (jeu de questions avec réponses attendues) pour mesurer l'effet des changements de consigne.

**Audit spécialisé : non** pour l'instant. Plus tard : un banc d'essai de 30 demandes types sur la démo `med`, pour comparer consignes et modèles.

## 10. Documentation, aide utilisateur : 4/5

**Ce qui est bien**
- Aide intégrée en 15 pages dans les 5 langues (≈ 20 000 mots en français), recherchable par l'assistant, MCP et Lua (`help.search`).
- Référence des fonctions par langue, référence SQL intégrée, référence Lua dans l'onglet Script, `docs/mcp.md`, `docs/scripting-lua.md`, `docs/configuration-entreprise.md`.
- README anglais et français, captures, démo en ligne.

**Ce qui faiblit**
- Aide longue : `prise-en-main.md` fait 4 900 mots, à l'opposé de la règle « textes courts ».
- Documentation développeur dispersée (`docs/`, `docs/audits`, `docs/conception`, `docs/decisions`) sans index ; l'audit de sécurité du 26/10 et l'audit de parité Excel ne sont pas dans le dépôt.
- `docs/decoupage-sheet.md` et `docs/feuille-de-route.md` ne reflètent plus l'état (§ 7, § 3).

**Audit spécialisé : non.** Une passe suffit : prise en main en une page, index de `docs/`, audits manquants versés au dépôt.

---

## Audits spécialisés proposés, par ordre conseillé

1. **Fiabilité des essais et de la CI** (§ 4) : petit, débloque tout le reste.
2. **Architecture du frontend et du pont** (§ 7) : `Sheet.svelte` grossit plus vite qu'on ne le découpe.
3. **Livraison et mise à jour** (§ 8) : signature et chaîne de confiance, avant une diffusion plus large.
4. **Performance des calculs et du démarrage** (§ 3).
5. **Collaboration sous charge** (§ 6), avec le dépôt `be-lattis-server`.
6. **Ergonomie et accessibilité** (§ 2).
7. **Parité formules et fichiers Excel** (§ 1).
