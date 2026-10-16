# Slitable

Tableur relationnel local-first : la souplesse visuelle d'un tableur, la rigueur d'une base SQLite. Go + Wails v3 + SvelteKit.

État : jalons M1 (prototypes, voir [docs/decisions/0001-jalon-m1.md](docs/decisions/0001-jalon-m1.md)), M2 (fondations) et M3 (« tableur-base » : saisie libre, tableaux qui naissent, collage depuis Excel, types, annuler, enregistrer) atteints ; phase 4 (formules) en cours. Mesures de la grille : [docs/mesures/grille.md](docs/mesures/grille.md).

## Prérequis

- Go 1.26 (imposé par `ncruces/go-sqlite3` ; avec `GOTOOLCHAIN=auto`, Go le télécharge tout seul)
- Node 22
- Wails v3 : `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.30`
- Linux : `libgtk-4-dev libwebkitgtk-6.0-dev`

## Commandes

```bash
wails3 build          # binaire dans bin/
go test ./internal/...
cd frontend && npm run check && npm test
SLITABLE_BENCH=resultat.json ./bin/slitable   # mesure automatique de la grille
go run . --demo med [--seed 42]                 # ouvre un classeur de démonstration
```

### Données de démonstration

`--demo [preset]` ouvre un classeur neuf rempli de fausses données plausibles (générateur déterministe : la même `--seed` donne le même classeur). Sans preset : `med`. Un preset inconnu affiche la liste.

| Preset | Contenu |
| --- | --- |
| `1Kx10`, `10Kx10`, `100Kx10`, `1Mx10` | 1 onglet de commandes, 1 000 à 1 M lignes × 10 colonnes |
| `1Kx100`, `10Kx100`, `100Kx100`, `1Mx100` | 1 onglet de relevés météo (stations et capteurs), × 100 colonnes |
| `sml` | 100 lignes × 10 colonnes, 1 onglet |
| `med` | `sml` + Clients (1 000 × 10) + Employés (1 200 × 10) + onglet Analyse : un TCD (salaire moyen par service) et 2 graphiques (barres sur le TCD, nuage ancienneté / salaire) |
| `big` | `med` + 8 onglets (stock, vols, livres, factures, interventions, relevés, étudiants, projets) de 5 à 50 colonnes et 100 à 100 000 lignes, répartis logarithmiquement |
| `refs` | 5 tableaux sur 3 onglets (Catégories, Produits, Clients, Commandes, Lignes) reliés par des `REFERENCE` en chaîne, `SOMME.SI` et `NB.SI`, avec des totaux en cellules libres |

En Go : `demo.New(ctx, "med", demo.Options{Seed: 42})` (internal/demo), ou `demo.Build` pour ajouter un preset à un classeur ouvert. `SLITABLE_DEMO_LOURD=1 go test ./internal/demo/` fabrique aussi les gros presets.

## Organisation

```
main.go               point d'entrée Wails (à la racine : les tâches Wails l'attendent ici)
internal/bridge/      API exposée au frontend, seule couche qui dépend de Wails
internal/doc/         un classeur ouvert : archive + base + journal + modèle
internal/archive/     fichier .slitable : extraction, verrou, enregistrement atomique, récupération
internal/meta/        schéma des tables _slitable_* et migrations
internal/command/     bus de commandes réversibles, journal d'annulation persisté
internal/canvas/      modèle canevas ↔ tables : îlots, lignes, colonnes, types
internal/store/       SQLite : WAL, pool de lecture, file d'écriture unique
internal/demo/        générateur de fausses données et presets --demo
internal/grid/        lecture par fenêtres (ancres + clé d'ordre), tableau de démo
internal/order/       clés d'ordre fractionnaires (_pos)
frontend/             SvelteKit 3 en SPA (adapter-static) ; feuille dans src/lib/grid, ruban et onglets dans src/lib/shell
spikes/               prototypes encore à intégrer (formules, Lua) et comparatif des drivers SQLite
```

Les bindings TypeScript (`frontend/bindings`) sont générés par `wails3 generate bindings` (fait par `wails3 build`) et importés via `#bindings/...`.
