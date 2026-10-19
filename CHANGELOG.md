# Journal des modifications

Ce qui change pour les utilisateurs de Lattis, version par version. Le détail technique reste dans l'historique git.

**Tenir ce journal.** Chaque modification visible (fonction, comportement, correction, sécurité) ajoute une ligne sous « Non publié », dans la bonne rubrique, dans le même commit que le code : une phrase qui dit ce que l'utilisateur voit, sans nom de fichier ni de personne. Rubriques : Nouveautés, Changements, Corrections, Sécurité, Retraits. À la publication, `scripts/release.sh` n'a rien à faire de plus : on renomme « Non publié » en `## 0.1.0 (JJ/MM/AAAA)` dans le commit qui précède le tag, puis on rouvre un « Non publié » vide ; `scripts/release-notes.sh` reprend alors cette section comme notes de version (voir `docs/release/publier.md`).

## Non publié

Aucune version n'est encore publiée. Voici, regroupé par jour, ce que contient la branche `main`.

### 19 octobre 2026

**Nouveautés**

- Regrouper des onglets à plusieurs tableaux : les tableaux s'apparient d'un onglet à l'autre (ventes, notes du mois) et deviennent chacun un tableau commun qui suit l'onglet à choix ; une liste recopiée sur chaque onglet (tarifs) n'est gardée qu'une fois ; la feuille commune reprend la disposition du dernier onglet et ses cellules libres, avec une cellule par mois quand elle changeait d'un onglet à l'autre (objectif, titre) ; le mois peut venir d'une cellule (« Janvier 2025 » en A1) ou des titres de tableaux empilés sur une feuille ; « Suivre l'année » sur chaque tableau (clic droit › Tableau, ou la pastille du tableau) ; un onglet copié une troisième fois propose le regroupement (format 34).
- Regrouper des onglets : un ancien onglet gardé parce qu'il contient autre chose reste visible, avec un onglet orange pour le retrouver (Ctrl+Z lui rend sa couleur).
- Onglet à choix : clic droit sur un onglet › Choisir par › une colonne (Année, Mois, Ville…) ; la flèche de l'onglet (« Ventes · 2025 ▾ », Alt+↓) choisit la valeur affichée par tous les tableaux de la feuille qui ont cette colonne, leurs TCD, SOUS.TOTAL et les formules de la feuille ; colonne masquée, valeur donnée aux nouvelles lignes, « Nouvelle année… », fonction ONGLET.CHOIX(), Ctrl+Alt+Pg.Suiv / Pg.Préc (format 33).
- Colonnes saisies à côté d'un tableau chargé par une requête (onglet SQL › Référencer, ou Power Query) : un nom tapé à droite de l'en-tête ajoute une colonne modifiable (en-tête vert), dont les valeurs suivent leurs lignes à chaque actualisation par une clé (la colonne `id`, ou une colonne choisie) ; les valeurs d'une ligne disparue sont mises de côté et reviennent avec elle.
- Data Lake Lattis : partager un tableau sur un Serveur Lattis (clic droit › Partager au Data Lake Lattis…) et l'insérer dans un autre classeur comme tableau lié, à jour en direct ; vues `donnees.*` dans l'onglet SQL.
- Mise en page par feuille (onglet Mise en page) : papier, orientation, marges, échelle, zone d'impression, titres répétés, sauts de page ; impression et export .xlsx qui la respectent.
- Fonctions : statistiques (ECARTYPE, VAR, MODE, CENTILE, QUARTILE…), finances (VPM, VA, VC, TAUX, VAN, TRI…), mathématiques, texte, dates ouvrées internationales, LIGNE et COLONNE, FRACTIONNER.TEXTE ; table des fonctions d'Excel absentes et de leurs remplaçants.
- Export .xlsx et .ods avec de vrais tableaux Excel (nom, filtre, colonnes calculées) et un compte rendu de ce qui n'est pas exporté tel quel.
- Fichier › Exporter › La feuille entière en CSV.
- Tableaux à en-têtes sur deux lignes (groupe et sous-colonne).
- Sélection de plusieurs feuilles (Ctrl+clic, Maj+clic), déplacées ou supprimées ensemble.
- Onglet SQL : bouton Arrêter pendant une requête.
- Assistant : outils de mise en forme et de réparation des tableaux, plusieurs conversations par classeur, choix « Tout le classeur » pour le partage des données.
- Accessibilité : la grille annonce la cellule active aux lecteurs d'écran (adresse, tableau, colonne, valeur, mode modification) ; la conversation de l'assistant est lue au fil des réponses.
- Glossaire des termes de l'interface (`docs/glossaire.md`).

**Changements**

- Un tableau créé (saisie, collage, Mettre sous forme de tableau) dont la première colonne s'appelle ID, ou vaut 1, 2, 3…, a d'office cette colonne pour clé (entiers uniques et remplis ; Ctrl+Z l'annule avec la création) ; une ligne ajoutée reçoit ensuite le numéro suivant de la clé, où qu'elle soit insérée, ou celui d'une première colonne en suite régulière (10, 20, 30…) pour une ligne ajoutée à la fin.
- Menu Fichier réorganisé comme Excel : « Imprimer / PDF… », « Ouvrir un fichier récent », sous-menu Exporter ; le serveur MCP se règle dans Paramètres.
- Ctrl+flèche et Ctrl+Maj+flèche vont au bord du bloc rempli puis au bloc suivant, comme Excel.
- Supprimer ou effacer des colonnes resserre le tableau ; une colonne vide au milieu le coupe en deux.
- Une recherche (REFERENCE, RECHERCHEV…) dans un tableau sans clé ne pose plus de clé en silence : la saisie demande quoi faire.
- Le fichier `.lattis` n'enregistre plus l'historique d'annulation ni les corbeilles (Ctrl+Z reste disponible pendant la session).
- Les volets du classeur (aide, requêtes, conflits, assistant) se logent côte à côte à droite.
- L'onglet SCRIPT s'appelle désormais Script ; « Refaire » devient « Rétablir » dans l'éditeur de requête.
- Les erreurs de connexion courantes de PostgreSQL, MySQL et SQL Server sont en français.
- Mémoire de SQLite bornée par classeur.

**Corrections**

- Tableau posé depuis l'onglet SQL : l'actualisation (F5) garde ses en-têtes et ne fait plus apparaître `_pos` ni les noms internes (c_1…) ; un résultat copié n'est plus pris pour celui d'une ancienne requête dont le tableau a été supprimé.
- Volets Graphique et TCD : le focus reste dans le volet pendant un réglage ; fermer le volet d'un TCD en mise à jour différée applique les changements.
- Raccourcis du classeur sans effet sous une boîte modale.
- Rechercher relit les occurrences après une modification.
- Une panique du moteur est journalisée et signalée au lieu de fermer l'application.

**Sécurité**

- Les sources Power Query qui joignent un hôte ou lisent un fichier demandent l'accord de l'utilisateur, comme les scripts.
- Version web : les requêtes et le module `http` ne joignent plus les adresses internes du serveur.
- Une page web ne peut plus naviguer d'elle-même vers une autre adresse.
- Les motifs Lua (`string.find`, `gsub`…) s'interrompent au délai et à Arrêter.
- Les écritures de l'onglet SQL s'arrêtent au bout de 2 minutes sans rien modifier.
- Export CSV pour Excel : un texte qui commence par `=`, `+`, `-` ou `@` ne devient plus une formule.
- La protection de feuille est présentée pour ce qu'elle est : une protection contre les erreurs, pas un secret.

**Retraits**

- RECHERCHEH n'est plus proposée (refusée à la saisie, REFERENCE la remplace).

### 18 octobre 2026

**Nouveautés**

- Assistant ✦ en volet latéral : Claude ou un fournisseur compatible OpenAI (dont un modèle local), passerelle d'entreprise, annulation d'une réponse en une fois.
- Serveur MCP : Claude Desktop et Claude Code pilotent le classeur ouvert, avec accord avant écriture.
- Serveur Lattis : connexion par le menu Compte, publication de classeurs figés ou collaboratifs, ouverture depuis un serveur, synchronisation avec gestion des conflits, écran « Partager » et droits, équipes.
- Version web de Lattis (moteur sans Wails) et image `lattis-engine` pour le serveur.
- Aide intégrée hors ligne (F1) avec recherche.
- Accessibilité clavier : KeyTips du ruban (Alt, F10), F6 entre les régions, Maj+F10 sur la grille.
- Fonction COMMUNE : commune française d'un point GPS, hors ligne.
- Cartes zoomables, couleur par colonne.
- Configuration d'entreprise en cascade (stratégies du système, fichier, serveur).
- Évènements du classeur dans les scripts Lua (`events.on`).
- Ruban réduit (Ctrl+F1), Rechercher et remplacer dans la sélection, Convertir en plage et Mettre sous forme de tableau.
- Ouverture d'un classeur par l'association de fichiers et par les liens `lattis://open`.
- Installeurs Windows, macOS et Linux construits sur chaque tag, signés quand les certificats sont configurés.

**Changements**

- Onglets HTML (pages web) à activer dans Fichier › Paramètres.
- Un tableau ne naît qu'à partir de trois cases remplies d'un carré de 2 × 2 ; titres, notes et totaux restent hors du tableau.
- Barre d'état à deux niveaux de message, qui s'effacent seuls.

**Corrections**

- Classeur endommagé : « Récupérer les données » ; erreurs claires sur un fichier illisible.
- Renommer un onglet par le clic droit ne plante plus.

**Sécurité**

- Classeurs reçus : accord avant la première exécution d'un script ou d'une page web, accords réseau rangés sur le poste, autorisateur SQLite sur le SQL de l'utilisateur, liste autorisée des commandes des pages.
- Mots de passe des connexions gardés par ordinateur, liés au classeur.

### 17 octobre 2026

**Nouveautés**

- Le projet s'appelle Lattis (extension `.lattis`).
- Sources de requêtes : MySQL/MariaDB, SQL Server, fichiers SQLite, JSON, Parquet, API web JSON ; éditeur de requête façon Power Query.
- Onglet SQL : résultat vers une feuille en valeurs figées ou en tableau lié à la requête.
- Graphiques combinés, axe secondaire, histogramme, cartes OpenStreetMap.
- Onglet Document : texte riche avec graphiques, tableaux croisés, boutons et contrôles insérés, export PDF.
- Onglets page web (HTML) et requêtes HTTP dans les scripts Lua.
- Contrôles liés à une cellule : case à cocher, liste, champ texte, sélecteur de fichier.
- Mise en forme conditionnelle façon Excel, notes en post-it, cellules fusionnées, hauteur automatique, zoom.
- Formules : références relatives `[Col][-1]`, nouvelle syntaxe REFERENCE(Tableau[Colonne]; clé), audit des antécédents et dépendants.
- Copier, couper, coller et glisser comme Excel ; animations après une modification.

**Changements**

- Feuille sans limite pratique de lignes et de colonnes ; performances sur 1 à 10 millions de lignes (tri, totaux, collage, annulation).

**Retraits**

- Sources DuckDB et ODBC abandonnées.

### 15 et 16 octobre 2026

**Nouveautés**

- Interface façon Excel : ruban, feuille infinie dessinée sur canvas, onglets de feuilles, onglet SQL.
- Chaque tableau est une table SQLite typée ; colonnes calculées, formules, cellules nommées, tableaux dynamiques déversés.
- Tri, filtres, totaux de la sélection, rechercher et remplacer, mise en forme, protection des feuilles et du classeur.
- Tableaux croisés dynamiques et graphiques (barres, courbes, secteurs, nuage de points).
- Onglet Script : scripts Lua avec éditeur, référence et linter ; API Lua et MCP.
- Import et export CSV et .xlsx ; requêtes d'import (PostgreSQL, CSV, xlsx).
- Annulation persistante par bus de commandes, récupération après un arrêt brutal.
- Données de démonstration (`--demo`).
