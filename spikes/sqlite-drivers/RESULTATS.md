# Comparatif des drivers SQLite Go (spike 1.1)

Mesuré le 15/10/2026 sur une VM Linux 4 cœurs, SQLite 3.53.4, table `STRICT` de 1 000 000 lignes × 20 colonnes, WAL + `synchronous=NORMAL`.

| Mesure | mattn (CGO) | modernc (Go pur) | ncruces (wasm2go) |
|---|---|---|---|
| Chargement 1 M lignes (1 transaction) | 8,4 s | 13,4 s | 11,1 s |
| Index sur `_pos` | 0,4 s | 0,9 s | 1,2 s |
| Relevé des ancres (1 clé / 1000 lignes) | 0,47 s | 0,42 s | 0,55 s |
| Fenêtre de 200 lignes, p50 / p95 | 0,7 / 1,4 ms | 1,1 / 2,1 ms | 0,9 / 1,8 ms |
| Encodage JSON d'une fenêtre (61 Ko), p50 | 0,34 ms | 0,35 ms | 0,35 ms |
| Édition d'une cellule (commit), p50 | 0,02 ms | 0,02 ms | 0,02 ms |
| `slt_div` (fonction Go) sur 1 M lignes | 1,5 s | 0,48 s | 0,39 s |
| Fichier | 291 Mo | 291 Mo | 291 Mo |

Conclusions :
- Les trois tiennent largement la lecture par fenêtres (≈ 1 ms pour 200 lignes) : le driver n'est pas le goulot de la grille.
- Les fonctions SQL écrites en Go (sémantique Excel) sont 3 à 4 fois plus lentes avec mattn (passage CGO à chaque appel).
- mattn imposerait CGO sous Windows, où Wails n'en a pas besoin.
- ncruces expose en plus : savepoints imbriqués, API de backup, extensions `unicode` (casse et collation), `stats`, `pivot` (utile pour les TCD).
- Aucun n'expose l'extension *session* : l'annulation passera par notre propre journal.

**Recommandation : `ncruces/go-sqlite3`** (impose Go ≥ 1.26, comme modernc).

Relancer : `go run -tags ncruces . -rows 1000000` (tags `mattn`, `modernc`, `ncruces`).
