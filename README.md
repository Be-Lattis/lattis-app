# Lattis

**Le tableur que vous connaissez, avec une vraie base de données dessous.**

Lattis a la souplesse d'Excel : une grille infinie, des formules en français, un ruban familier, Ctrl+C / Ctrl+V. Chaque tableau que vous tapez est en réalité une table **SQLite** typée, indexée et interrogeable en SQL. Résultat : un million de lignes ne fait plus peur, les données restent propres, et vous pouvez passer à tout moment de la souris au SQL, ou au script.

Application de bureau native (Go + Wails), **local-first** : vos classeurs sont de simples fichiers `.lattis`, sans compte ni cloud ni abonnement.

![Une feuille Lattis : deux tableaux reliés par une formule NB.SI](docs/captures/grille-formule.png)

---

## Pourquoi Lattis ?

| Dans un tableur classique… | Dans Lattis |
| --- | --- |
| Une plage de cellules sans structure | **Chaque tableau est une table** : colonnes typées (texte, entier, décimal, date, booléen), clé unique détectée automatiquement |
| `RECHERCHEV` fragile, décalée dès qu'on insère une colonne | `REFERENCE(Produits[Prix]; [Code])` : une jointure par **nom de colonne**, qui ne casse pas |
| Le fichier rame au-delà de 100 000 lignes | Défilement instantané sur **1 million de lignes** et plus : seules les lignes visibles sont lues |
| Les données sont prisonnières de la feuille | **Onglet SQL** intégré : interrogez vos tableaux, croisés dynamiques compris, comme des tables |
| Macros VBA risquées | **Scripts Lua** en bac à sable, impossibles à bloquer : une boucle infinie est interrompue |

## Ce que vous pouvez faire

### Saisir comme dans Excel, sans rien déclarer

Tapez quelques lignes n'importe où : Lattis reconnaît l'îlot de données, met l'en-tête en valeur, devine le type de chaque colonne et crée la table correspondante. Collez depuis Excel, glissez un tableau à côté d'un autre pour les fusionner, annulez tout avec Ctrl+Z.

Les formules parlent français (`SOMME.SI`, `NB.SI`, `SI`, `ARRONDI`…) et désignent les colonnes par leur nom : `=[Quantité]*[Prix unitaire]` s'applique à toute la colonne, comme un tableau structuré d'Excel. Les références sont colorées pendant la saisie et se placent d'un clic, et la colonne calculée concernée est mise en évidence.

### Analyser : tableaux croisés et graphiques

![Tableau croisé dynamique et graphiques alimentés par lui](docs/captures/tcd-graphiques.png)

- **Tableaux croisés dynamiques** calculés par SQLite (`GROUP BY`) : lignes, colonnes, plusieurs valeurs, pourcentages, filtres de rapport et tri directement dans la grille.
- **Graphiques** (barres, colonnes, courbes, aires, secteurs, anneaux, nuages de points) branchés sur un tableau ou un TCD, et mis à jour avec lui.
- Mise en forme conditionnelle, filtres et tris comme dans Excel, suppression des doublons, validation des données.

### Interroger en SQL

![L'onglet SQL : requête, résultat, envoi vers une feuille](docs/captures/sql.png)

L'onglet **SQL** liste les objets du classeur et exécute vos requêtes sur les noms que vous voyez à l'écran (`"Employés"`, `"Salaire brut"`), avec complétion, historique et favoris. Les tableaux croisés s'interrogent comme des vues. Un résultat se copie dans une feuille, ou s'y **référence** pour rester à jour.

### Automatiser en Lua

![L'éditeur de scripts : coloration, linter en direct, référence de l'API](docs/captures/script-lua.png)

L'onglet **SCRIPT** offre un vrai éditeur : coloration syntaxique, linter en direct (variables non définies, fautes de frappe dans l'API, `end` manquant) et une référence intégrée. Il donne accès à l'API `db.query`, `db.update`, `sheets.*`, `ui.*`… Les scripts tournent dans un bac à sable, et tout ce qu'ils modifient s'annule en un Ctrl+Z. → [Guide du scripting](docs/scripting-lua.md)

### Importer et transformer les données (façon Power Query)

![L'éditeur de requêtes : étapes appliquées, aperçu, menus de colonnes](docs/captures/power-query.png)

L'éditeur de requêtes importe des données depuis :
- des fichiers CSV, Excel, JSON ou Parquet ;
- une base SQLite, PostgreSQL, MySQL ou SQL Server ;
- le web ;
- un autre tableau du classeur.

Chaque transformation devient une étape rejouable : filtrer, trier, choisir des colonnes, regrouper, fusionner, pivoter et dépivoter, fractionner, ajouter une colonne… Un clic sur une colonne la sélectionne, et Ctrl+Z retire la dernière étape. Le résultat se charge dans un tableau et s'actualise à la demande.

### Et aussi

- **Import / export** `.xlsx` et `.csv`, impression.
- **Serveur MCP** intégré : laissez un assistant comme Claude lire et modifier le classeur ouvert, en local et avec un jeton. → [docs/mcp.md](docs/mcp.md)
- **Fichier `.lattis`** : une seule archive (base SQLite + manifeste), facile à envoyer par e-mail ou à archiver, avec enregistrement atomique et récupération après incident.
- Protection de feuilles et de classeur, notes façon post-it, cellules fusionnées, volets figés, zoom, animations de sélection.

## Les performances, mesurées

Sur un tableau de **1 000 000 de lignes** (poste de développement, Windows 10) :

| Opération | Temps |
| --- | ---: |
| Défiler n'importe où dans le tableau | 1 à 9 ms |
| Insérer ou supprimer une ligne au milieu | 10 à 35 ms |
| Ajouter une colonne calculée | 2 ms |
| Coller 100 000 × 10 cellules en zone libre | 2,5 s |
| Tableau croisé déjà calculé | instantané (en cache) |

Le détail est dans [docs/mesures/performances-1M.md](docs/mesures/performances-1M.md) et dans l'[audit des gros jeux de données](docs/audit-performance-gros-jeux.md).

## Essayer

```bash
go run . --demo med      # classeur de démonstration : commandes, clients, employés, TCD et graphiques
go run . --demo refs     # 5 tableaux reliés par des REFERENCE, SOMME.SI, NB.SI
go run . --demo 1Mx10    # un million de lignes
go run . --help          # toutes les options
```

---

## Pour les développeurs

### Stack

- **Backend Go** : SQLite en WAL (`ncruces/go-sqlite3`), une file d'écriture unique et un pool de lecture, un bus de commandes réversibles avec journal d'annulation persisté, et `gopher-lua` pour les scripts.
- **Frontend SvelteKit / Svelte 5** : grille dessinée en `<canvas>`, CodeMirror pour les éditeurs SQL et Lua.
- **Wails v3** : un binaire natif, sans Electron.

### Prérequis

- Go 1.26 (imposé par `ncruces/go-sqlite3` ; avec `GOTOOLCHAIN=auto`, Go le télécharge tout seul)
- Node 22
- Wails v3 : `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.30`
- Linux : `libgtk-4-dev libwebkitgtk-6.0-dev`

### Commandes

```bash
wails3 build          # binaire dans bin/
go test ./internal/...
cd frontend && npm run check && npm test
LATTIS_BENCH=resultat.json ./bin/lattis   # mesure automatique de la grille
go run . --demo med [--seed 42]                 # ouvre un classeur de démonstration
go run . --sql                                  # journal des requêtes SQL dans la console
```

### Données de démonstration

`--demo [preset]` ouvre un classeur neuf rempli de fausses données plausibles. Le générateur est déterministe : la même `--seed` donne le même classeur. Sans preset, c'est `med` qui s'ouvre ; un preset inconnu affiche la liste.

| Preset | Contenu |
| --- | --- |
| `1Kx10`, `10Kx10`, `100Kx10`, `1Mx10`, `10Mx10` | 1 onglet de commandes, de 1 000 à 10 M lignes × 10 colonnes |
| `1Kx100`, `10Kx100`, `100Kx100`, `1Mx100`, `10Mx100` | 1 onglet de relevés météo (stations et capteurs), × 100 colonnes |
| `sml` | 100 lignes × 10 colonnes, 1 onglet |
| `med` | `sml`, plus Clients (1 000 × 10), Employés (1 200 × 10) et un onglet Analyse (un TCD et 2 graphiques) |
| `big` | `med`, plus 8 onglets de 5 à 50 colonnes et de 100 à 100 000 lignes |
| `refs` | 5 tableaux sur 3 onglets reliés par des `REFERENCE` en chaîne, `SOMME.SI` et `NB.SI` |

En Go : `demo.New(ctx, "med", demo.Options{Seed: 42})` (internal/demo), ou `demo.Build` pour ajouter un preset à un classeur ouvert. `LATTIS_DEMO_LOURD=1 go test ./internal/demo/` fabrique aussi les gros presets.

### Organisation

```
main.go               point d'entrée Wails (à la racine : les tâches Wails l'attendent ici)
internal/bridge/      API exposée au frontend, seule couche qui dépend de Wails
internal/doc/         un classeur ouvert : archive + base + journal + modèle
internal/archive/     fichier .lattis : extraction, verrou, enregistrement atomique, récupération
internal/meta/        schéma des tables _lattis_* et migrations
internal/command/     bus de commandes réversibles, journal d'annulation persisté
internal/canvas/      modèle canevas ↔ tables : îlots, lignes, colonnes, types, TCD, graphiques
internal/formula/     analyse et calcul des formules (traduites en SQL quand c'est possible)
internal/query/       requêtes d'import (sources externes et étapes de transformation)
internal/script/      moteur Lua en bac à sable
internal/api/         commandes communes aux scripts et au serveur MCP
internal/mcp/         serveur MCP local
internal/transfer/    import / export xlsx et csv, impression
internal/store/       SQLite : WAL, pool de lecture, file d'écriture unique
internal/demo/        générateur de fausses données et presets --demo
internal/grid/        lecture par fenêtres (ancres + clé d'ordre)
internal/order/       clés d'ordre fractionnaires (_pos)
frontend/             SvelteKit en SPA (adapter-static) ; feuille dans src/lib/grid, ruban et onglets dans src/lib/shell
```

Les bindings TypeScript (`frontend/bindings`) sont générés par `wails3 generate bindings` (fait par `wails3 build`) et importés via `#bindings/...`.
