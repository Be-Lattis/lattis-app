# Slitable

Tableur relationnel local-first : la souplesse visuelle d'un tableur, la rigueur d'une base SQLite. Go + Wails v3 + SvelteKit.

État : phase 0 (socle) et prototype de grille (spike 1.1). Voir [docs/mesures/grille.md](docs/mesures/grille.md).

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
```

## Organisation

```
main.go               point d'entrée Wails (à la racine : les tâches Wails l'attendent ici)
internal/bridge/      API exposée au frontend, seule couche qui dépend de Wails
internal/store/       SQLite : WAL, pool de lecture, file d'écriture unique
internal/grid/        lecture par fenêtres (ancres + clé d'ordre), saisie, tableau de démo
internal/order/       clés d'ordre fractionnaires (_pos)
frontend/             SvelteKit 3 en SPA (adapter-static) ; grille dans src/lib/grid
spikes/               code de mesure jetable (comparatif des drivers SQLite)
```

Les bindings TypeScript (`frontend/bindings`) sont générés par `wails3 generate bindings` (fait par `wails3 build`) et importés via `#bindings/...`.
