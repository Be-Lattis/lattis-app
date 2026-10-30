# Documentation développeur

Index de `docs/`. L'aide de l'application, elle, est dans `frontend/src/lib/help/pages` (5 langues) ; le journal des versions dans `CHANGELOG.md`.

## Pour commencer

- [feuille-de-route.md](feuille-de-route.md) : ce qui reste à faire, par sujet, avec l'état au 30/10/2026.
- [glossaire.md](glossaire.md) : les mots de l'interface, de l'aide et des messages, et les règles d'écriture.
- [format-lattis.md](format-lattis.md) : le fichier `.lattis` (archive, base SQLite, tables de métadonnées, versions).
- [decoupage-sheet.md](decoupage-sheet.md) : plan de découpage de `Sheet.svelte` et des essais de l'interface.

## Fonctions et scripts

- [fonctions.md](fonctions.md) : bibliothèque des fonctions des formules, par catégorie.
- [scripting-lua.md](scripting-lua.md) : guide des scripts Lua pour l'utilisateur.
- [scripting.md](scripting.md) : référence technique de l'API Lua (moteur, bac à sable, écritures).
- [mcp.md](mcp.md) : le serveur MCP, pour piloter un classeur depuis un assistant.
- [i18n.md](i18n.md) : conception du multilingue (catalogues, aide, fonctions par langue).

## Déploiement et démonstration

- [configuration-entreprise.md](configuration-entreprise.md) : configuration en cascade pour un parc (sources externes, mises à jour, assistant).
- [demo.md](demo.md) : `lattis --demo`, classeurs de démonstration ; [demo/](demo/) : un plan d'exemple vérifié par les essais.
- [release/](release/) : publier une version (procédure, liste de contrôle, signature de code, choix de l'autorité).
- [captures/](captures/) : captures d'écran du README.

## Dossiers

- [audits/](audits/) : audits datés (général, performances, formules, collaboration, livraison, essais et CI, UX) et réflexions de mesure ; chaque constat porte son état (corrigé, arbitrage, plus tard).
- [conception/](conception/) : notes de conception d'une fonction avant sa réalisation (mise à jour automatique, formulaire, regroupement d'onglets).
- [decisions/](decisions/) : décisions numérotées (0001 à 0018) : le choix fait, ses raisons, ce qui a été écarté.
- [collaboration/](collaboration/) : classeurs partagés sur un Serveur Lattis (connexion, droits, synchronisation, journal, publication, Data Lake, version web).
- [design/](design/) : identité visuelle (logos, icône) et conception du mode sombre et du contraste élevé.
- [mesures/](mesures/) : mesures de référence (grille, 1 million de lignes).
- [audit-performance-gros-jeux.md](audit-performance-gros-jeux.md) : premier audit des gros jeux de données (17/10/2026), cité par le README.
