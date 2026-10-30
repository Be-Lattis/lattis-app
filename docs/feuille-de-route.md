# Feuille de route

Sujets connus, pas urgents, à reprendre plus tard. Le plus récent en haut. Le détail de chaque point est dans l'audit cité ; un point fait est marqué **fait** avec son commit, puis retiré à la passe suivante.

## Après les audits du 30/10/2026

Audit général (`docs/audits/audit-general-2026-10-30.md`) et ses audits spécialisés du même jour. Faits ce jour-là : manifeste des mises à jour signé, installeur en LZMA, essais et CI fiabilisés, critères par ligne, TCD et tri plus rapides, fonctions et export .xlsx des fonctions récentes, deux défauts de la collaboration, aide raccourcie et index de `docs/`.

**À décider (Louis)**

- Signature de code des installeurs (Windows, macOS) et publication de la 0.5.1-beta : `audit-distribution-2026-10-30.md` § 4 et 5.
- Formules : famille LAMBDA (MAP, BYROW, BYCOL…), recherches gardées telles que tapées, RECHERCHEH sur un tableau, CHOISIRLIGNES hors limites, moteur des expressions régulières : `audit-formules-2026-10-30.md` n° 4, 5, 8, 9, 13.
- Collaboration : connexions PostgreSQL de lattisd (une par utilisateur abonné, `max_connections`), repli sur LISTEN après un 503, connexion gardée par poste au repos : `audit-collaboration-2026-10-30.md` S5, S6, A1, A2.
- Performances : extraction à l'ouverture d'un classeur de 10 M de lignes, pilote SQLite natif (piste G) : `audit-performance-2026-10-30.md` § 4 et 7.
- CI : antivirus des machines Windows, `-race` bloquant, `-race` sur `bridge` et `canvas` : `audit-tests-ci-2026-10-30.md`, Arbitrages.

**Plus tard**

- Formules : fonctions encore absentes (PGCD, PPCM, CTXT, ROMAIN, conversions, matrices, FREQUENCE…), TABLEAU.ALEA, ORGA.LIGNES, DEVELOPPER, `par_colonne` de TRIER et UNIQUE, SOUS.TOTAL 101 à 111 sur une plage avec des lignes masquées : `audit-formules-2026-10-30.md` n° 10, 11, 15, 17.
- Performances : table étroite des positions pour le tri (piste F), TCD avec un tri choisi, numérotation automatique gardée dans le fichier, clé sur une colonne de texte, dialogues chargés à la demande : `audit-performance-2026-10-30.md` § 5, 10, 11.
- Livraison : mise à jour réelle sous macOS et Linux, retour arrière, « ignorer cette version », miroir interne (`updates.url`), contrôle Authenticode, AppImage de 100 Mo, chaîne d'outils de la CI : `audit-distribution-2026-10-30.md` § 6 à 10.
- Collaboration : latence de propagation (deltas lus une fois par lattisd), essai de charge déclenché à la main : `audit-collaboration-2026-10-30.md` P1, P2.
- Essais : horloge injectable (`coedit`, `work`, évènements), paquet `integration` découpé, essais lents en `-short`, durée réelle de la CI : `audit-tests-ci-2026-10-30.md`, Plus tard.
- Audits encore à faire : architecture du frontend et du pont (`docs/decoupage-sheet.md` à jour), ergonomie et accessibilité (audit général § 2 et § 7).
- Documentation : verser au dépôt l'audit de sécurité du 26/10 ; resserrer les pages d'aide encore longues (Tableaux, Onglets semblables, Partage en direct, Mise en forme), et retirer de l'aide le nom d'un autre tableur.

## Version web : échanges et montée en charge (21/10/2026)

- Piste H de docs/audits/reflexion-performances-2026-10-21.md.
- Blocs de la grille : un format d'échange plus compact que le JSON (binaire), pour un défilement fluide sur un réseau lent. Sur un bloc de 200 lignes, l'échange coûte déjà environ 5 ms, contre 1,6 ms pour SQLite (docs/mesures/grille.md).
- Après chaque action, l'interface relit encore l'aide des fonctions (82 Ko) : ne la lire qu'à l'ouverture et quand la langue change.
- À l'échelle (plusieurs centaines de postes) : **fait** pour la bascule, le classeur suit le relais `/api/events` du serveur depuis le 20/10 (d3383781), repli sur LISTEN ; S2 corrigé côté serveur, S3 en partie. Reste la charge côté serveur (S5, S6 ci-dessus).

## Mise à jour automatique : installation silencieuse (21/10/2026)

- Constat (poste de Louis, v0.3.2-beta par utilisateur) : « Redémarrez l'application pour appliquer la mise à jour » fermait Lattis sans rien installer, et le message revenait à chaque lancement.
- Corrigé (871a2bf) : l'installeur part après `app.Run()`, à toute fermeture ; les essais sont notés, un échec s'affiche (« Installer manuellement »), l'installation automatique s'arrête après deux échecs, l'installeur laisse un compte rendu (`install-result.txt`). Installation silencieuse par utilisateur **vérifiée** sur le poste de Louis le 21/10.
- **Fait** : signature Ed25519 de `latest.json` (33395cb0, `audit-distribution-2026-10-30.md` § 1).
- Reste à faire :
  - éprouver l'installation silencieuse pour tous les utilisateurs (UAC) et la version portable, de version en version ;
  - les versions jusqu'à 0.3.3-beta gardent le défaut : une installation à la main, une fois ;
  - suites de l'étude (docs/conception/mise-a-jour-automatique.md) : voir Livraison ci-dessus.
