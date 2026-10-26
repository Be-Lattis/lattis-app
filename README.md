# Lattis

**English** · [Français](README.fr.md)

[![Download](https://img.shields.io/badge/Download-Windows%20%C2%B7%20macOS%20%C2%B7%20Linux-217346?style=for-the-badge)](https://lattis.be/#telecharger) [![Online demo](https://img.shields.io/badge/Try%20the-online%20demo-1f6feb?style=for-the-badge)](https://main.lattis.be/demo/)

**The spreadsheet you know, with a real database underneath.**

Lattis feels like a spreadsheet: an infinite grid, formulas in your language, a familiar ribbon, Ctrl+C / Ctrl+V. Every table you type is in fact a typed, indexed **SQLite** table you can query in SQL. A million rows is no longer a problem, the data stays clean, and you can switch at any time from the mouse to SQL, or to a script.

A native desktop application (Go + Wails), **local-first**: workbooks are plain `.lattis` files, usable without an account or a subscription. To work together, your organization's Lattis Server shares them.

![A Lattis sheet: two tables linked by a COUNTIF formula](docs/captures/grille-formule.png)

---

## Why Lattis?

| In a classic spreadsheet… | In Lattis |
| --- | --- |
| A range of cells with no structure | **Every table is a table**: typed columns (text, integer, decimal, date, boolean), unique key detected automatically |
| A fragile `VLOOKUP` that shifts as soon as a column is inserted | `REFERENCE(Products[Price], [Code])`: a join by **column name** that doesn't break |
| The file crawls past 100,000 rows | Instant scrolling on **1 million rows** and more: only the visible rows are read |
| Data is locked inside the sheet | A built-in **SQL tab**: query your tables, pivot tables included, as tables |
| Risky macros | Sandboxed **Lua scripts** that can't freeze the app: an infinite loop is stopped |

## What you can do

### Type like in a spreadsheet, with nothing to declare

Type a few rows anywhere: Lattis recognizes the block of data, highlights the header, guesses each column's type and creates the matching table. Paste from another spreadsheet, drag a table next to another to merge them, undo anything with Ctrl+Z.

Formulas speak your language (in English: `SUMIF`, `COUNTIF`, `IF`, `ROUND`…; also French, German, Spanish and Chinese) and refer to columns by name: `=[Quantity]*[Unit price]` applies to the whole column. References are colored while you type and can be placed with a click, and the calculated column involved is highlighted.

### Analyze: pivot tables and charts

![A pivot table and the charts it feeds](docs/captures/tcd-graphiques.png)

- **Pivot tables** computed by SQLite (`GROUP BY`): rows, columns, several values, percentages, report filters and sorting right in the grid.
- **Charts** (bar, column, line, area, pie, doughnut, scatter and more) connected to a table or a pivot table, and updated with it.
- Conditional formatting, filters and sorting, duplicate removal, data validation.

### Query in SQL

![The SQL tab: query, result, sent to a sheet](docs/captures/sql.png)

The **SQL** tab lists the workbook's objects and runs your queries on the names you see on screen (`"Employees"`, `"Gross salary"`), with completion, history and favorites. Pivot tables can be queried like views. A result can be copied into a sheet, or **referenced** there to stay up to date.

### Automate with Lua

![The script editor: highlighting, live linter, API reference](docs/captures/script-lua.png)

The **Script** tab is a real editor: syntax highlighting, a live linter (undefined variables, typos in the API, missing `end`) and a built-in reference. It gives access to the `db.query`, `db.update`, `sheets.*`, `ui.*`… API. Scripts run in a sandbox, and everything they change is undone with one Ctrl+Z. Scripts in a workbook you received ask for your consent the first time they run. → [Scripting guide](docs/scripting-lua.md) (in French)

### Import and transform data

![The query editor: applied steps, preview, column menus](docs/captures/power-query.png)

The query editor imports data from:
- CSV, XLSX, JSON or Parquet files;
- a SQLite, PostgreSQL, MySQL or SQL Server database;
- the web;
- another table in the workbook.

Each transformation becomes a replayable step: filter, sort, choose columns, group, merge, pivot and unpivot, split, add a column… Clicking a column selects it, and Ctrl+Z removes the last step. The result loads into a table and refreshes on demand.

### And also

- **Import / export** `.xlsx` and `.csv`, `.ods` import and export (translated formulas, formatting, tables), printing and PDF.
- A built-in **MCP server**: let an assistant such as Claude read and edit the open workbook, locally and with a token. → [docs/mcp.md](docs/mcp.md)
- **The `.lattis` file**: a single archive (SQLite database + manifest), easy to e-mail or archive, with atomic saving and crash recovery. The undo history and deleted data are never saved in it. Opened from a shared folder, it is flagged to other computers (`~$name.lattis`), and a file changed elsewhere is never overwritten without asking. The format is described in [docs/format-lattis.md](docs/format-lattis.md): what another tool can read or write in it.
- **Municipality of a GPS point**, offline: `=MUNICIPALITY([Latitude], [Longitude])` gives the French municipality (or its INSEE code, postcode, department, region) from the official boundaries shipped with the app, even on 100,000 rows. Contains IGN and INSEE data, Licence Ouverte / Etalab 2.0. → [docs/fonctions.md](docs/fonctions.md#géographie)
- Sheet and workbook protection, sticky notes, merged cells, frozen panes, zoom, selection animations.

## Performance, measured

On a **1,000,000-row** table (development machine, Windows 10):

| Operation | Time |
| --- | ---: |
| Scroll anywhere in the table | 1 to 9 ms |
| Insert or delete a row in the middle | 10 to 35 ms |
| Add a calculated column | 2 ms |
| Paste 100,000 × 10 cells in a free area | 2.5 s |
| Pivot table already computed | instant (cached) |

Details are in [docs/mesures/performances-1M.md](docs/mesures/performances-1M.md) and in the [large dataset audit](docs/audit-performance-gros-jeux.md) (in French).

**Known limits** on a million rows and more: sorting a wide table takes from 10 s to several minutes, changing a column's type or deleting a column rebuilds the table (a few seconds to a minute), a SQL `UPDATE` on every row takes 20 to 70 s, and saving a modified workbook rewrites the whole file (14 s for 2.2 GB). The status of each point is kept in the [large dataset audit](docs/audit-performance-gros-jeux.md#état-au-19-octobre).

## Try it

Download the installer for your system (Windows, macOS, Linux) from the [releases](https://github.com/Be-Lattis/lattis-app/releases), or try it without installing anything in the [online demo](https://main.lattis.be/demo/). From the sources:

```bash
go run . --demo med      # showcase: document, map, gauge, tree pivot table, SQL query, scripts
go run . --demo refs     # 5 tables linked by REFERENCE, SUMIF, COUNTIF
go run . --demo 1Mx10    # a million rows
go run . --demo docs/demo/exemple.yaml   # workbook described by a YAML file (docs/demo.md)
go run . --help          # all options
```

---

## For developers

### Stack

- **Go backend**: SQLite in WAL mode (`ncruces/go-sqlite3`), a single write queue and a read pool, a bus of reversible commands with an undo journal kept in the working copy (never in the saved file), and `gopher-lua` for scripts.
- **SvelteKit / Svelte 5 frontend**: a grid drawn on a `<canvas>`, CodeMirror for the SQL and Lua editors.
- **Wails v3**: one native binary, without Electron.

### Requirements

- Go 1.26 (required by `ncruces/go-sqlite3`; with `GOTOOLCHAIN=auto`, Go downloads it by itself)
- Node 22
- Wails v3: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.30`
- Linux: `libgtk-4-dev libwebkitgtk-6.0-dev`

### Commands

```bash
wails3 build          # binary in bin/
go test ./internal/...
cd frontend && npm run check && npm test
LATTIS_BENCH=result.json ./bin/lattis   # automatic grid benchmark
go run . --demo med [--seed 42]                 # opens a demo workbook
go run . --sql                                  # logs SQL queries to the console
go run . --dev                                  # rebuilds the interface (npm run build) before starting
```

"Open in Lattis" links from the Lattis Server web page in development (Windows): `go run .` registers the `lattis://` scheme to its own temporary executable (HKCU\Software\Classes\lattis, message in the console); the link works while that instance runs, and an installed version takes the key back at its next launch. `--no-register-protocol` turns registration off.

### Demo data

`--demo [preset]` opens a new workbook filled with plausible fake data. The generator is deterministic: the same `--seed` gives the same workbook. Without a preset, `med` opens; an unknown preset shows the list. `--demo plan.yaml` builds the workbook described by a YAML file (sheets, tables and column generators, formulas, pivot tables, charts, controls, queries, documents, pages, scripts): format in [docs/demo.md](docs/demo.md), example in [docs/demo/exemple.yaml](docs/demo/exemple.yaml); `--seed` takes precedence over the file's seed.

| Preset | Content |
| --- | --- |
| `1Kx10`, `10Kx10`, `100Kx10`, `1Mx10`, `10Mx10` | 1 sheet of orders, from 1,000 to 10 M rows × 10 columns |
| `1Kx100`, `10Kx100`, `100Kx100`, `1Mx100`, `10Mx100` | 1 sheet of weather readings (stations and sensors), × 100 columns |
| `sml` | 100 rows × 10 columns, 1 sheet |
| `med` | the showcase (9 sheets): a « À découvrir » Document (live text, chart and pivot table, Lua button, linked field), Ventes (2,000 keyed orders, REFERENCE, conditional formatting, a list driving formulas, buttons), Villes (OpenStreetMap map), Synthèse (tree pivot table with subtotals, binning, radar), Ventes mensuelles (SQL query with window functions, refreshable with F5; combo, gauge, waterfall), Impayés (import query with steps), Clients, Employés and Analyse |
| `big` | `med`, plus 8 sheets of 5 to 50 columns and 100 to 100,000 rows |
| `refs` | 5 tables on 3 sheets linked by chained `REFERENCE`, `SUMIF` and `COUNTIF` |

In Go: `demo.New(ctx, "med", demo.Options{Seed: 42})` (internal/demo), or `demo.Build` to add a preset to an open workbook. `LATTIS_DEMO_LOURD=1 go test ./internal/demo/` also builds the large presets.

### Layout

```
main.go               Wails entry point (at the root: the Wails tasks expect it here)
internal/bridge/      API exposed to the frontend, the only layer that depends on Wails
internal/doc/         an open workbook: archive + database + journal + model
internal/archive/     .lattis file: extraction, locks, atomic save without history, recovery
internal/meta/        schema of the _lattis_* tables and migrations
internal/command/     bus of reversible commands, session undo journal
internal/canvas/      canvas ↔ tables model: blocks, rows, columns, types, pivot tables, charts
internal/formula/     formula parsing and evaluation (translated to SQL when possible)
internal/geo/         embedded municipality boundaries (go generate): municipality of a GPS point, offline
internal/query/       import queries (external sources and transformation steps)
internal/script/      sandboxed Lua engine
internal/api/         commands shared by scripts and the MCP server
internal/mcp/         local MCP server
internal/transfer/    xlsx, ods and csv import / export, printing
internal/store/       SQLite: WAL, read pool, single write queue, authorizer for the user's SQL
internal/trust/       user consents outside workbooks (scripts per version, network hosts)
internal/demo/        fake data generator and --demo presets
internal/grid/        windowed reads (anchors + order key)
internal/order/       fractional order keys (_pos)
frontend/             SvelteKit as an SPA (adapter-static); sheet in src/lib/grid, ribbon and tabs in src/lib/shell
```

The TypeScript bindings (`frontend/bindings`) are generated by `wails3 generate bindings` (run by `wails3 build`) and imported through `#bindings/...`.

## License

Lattis is free software, under the [Mozilla Public License 2.0](LICENSE): you may use, study, modify and redistribute it. Modified files stay under MPL-2.0 and their code must be published with the distributed version; the rest of an application that embeds Lattis may keep its own license.

The names « Lattis », « Be-Lattis » and the logo are not part of this license: see [TRADEMARKS.md](TRADEMARKS.md). Third-party components and data are listed in [NOTICE.md](NOTICE.md). Lattis Server, a separate piece of software, is under the AGPL-3.0 license ([lattis-server](https://github.com/Be-Lattis/lattis-server)).
