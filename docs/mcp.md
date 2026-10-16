# Piloter Slitable depuis un assistant (serveur MCP)

Slitable embarque un serveur [MCP](https://modelcontextprotocol.io) : un assistant
compatible (Claude Desktop, Claude Code…) lit et modifie le classeur ouvert avec les mêmes
commandes que les scripts Lua (voir [scripting-lua.md](scripting-lua.md), section 5).

## Activer le serveur

**Fichier › Serveur MCP (assistants)…** puis cocher *Activer le serveur MCP*. Le réglage est
gardé : le serveur redémarre avec l'application. La fenêtre donne la configuration à copier
pour chaque client.

- Le serveur n'écoute que sur **cet ordinateur** (`127.0.0.1`, port 47390 par défaut, un
  autre s'il est pris).
- Chaque requête doit porter le **jeton** affiché (`Authorization: Bearer …`) ; une requête
  venue d'une page web (en-tête `Origin` étranger) est refusée.
- Le serveur agit sur le classeur **ouvert dans la fenêtre** : l'utilisateur voit chaque
  modification, et **chaque appel qui modifie le classeur est une étape d'annulation**
  (Ctrl+Z).
- Désactivé (par défaut), rien n'écoute.

Les réglages (port, jeton) sont dans `mcp.json`, dans le dossier de configuration de
l'utilisateur (`%AppData%\Slitable` sous Windows, `~/Library/Application Support/Slitable`
sous macOS, `~/.config/Slitable` sous Linux). Supprimer le fichier renouvelle le jeton.

## Brancher un client

**Claude Desktop** (et les clients qui lancent un programme en stdio) : dans
`claude_desktop_config.json`,

```json
{
  "mcpServers": {
    "slitable": { "command": "C:\\Program Files\\Slitable\\slitable.exe", "args": ["mcp"] }
  }
}
```

`slitable mcp` ne lance pas de deuxième fenêtre : il relaie les messages vers l'application
ouverte (et répond « Slitable n'est pas ouvert » sinon).

**Claude Code** :

```sh
claude mcp add --transport http slitable http://127.0.0.1:47390/mcp --header "Authorization: Bearer <jeton>"
```

**Autres clients HTTP** : transport *Streamable HTTP*, `POST http://127.0.0.1:47390/mcp`,
réponses JSON.

## Outils

Chaque commande de l'application est un outil ; son nom remplace le point par un souligné
(`sheets.add` → `sheets_add`). Les paramètres et la forme du résultat sont décrits dans le
schéma de chaque outil (`tools/list`).

| Famille | Outils |
|---|---|
| Classeur | `doc_state`, `doc_undo`, `doc_redo`, `doc_new`, `doc_open`, `doc_save`, `doc_import`, `doc_export_csv`, `doc_recalc` |
| SQL | `db_query` (lecture, noms affichés, 10 000 lignes au plus), `db_execute` (INSERT, UPDATE, DELETE annulables) |
| Feuilles | `sheets_list`, `sheets_get`, `sheets_add`, `sheets_rename`, `sheets_move`, `sheets_delete`, `sheets_set_column_width`, `sheets_set_row_height`, `sheets_hide_lines`, `sheets_freeze`, `sheets_find`, `sheets_replace`, `sheets_protect`, `sheets_unprotect`, `sheets_protect_workbook`, `sheets_unprotect_workbook` |
| Cellules | `cells_get`, `cells_set`, `cells_paste`, `cells_clear`, `cells_style`, `cells_style_lines`, `cells_merge`, `cells_unmerge`, `cells_note`, `cells_name`, `cells_dependencies` |
| Tableaux | `tables_list`, `tables_get`, `tables_rows`, `tables_rename`, `tables_move`, `tables_delete`, `tables_sort`, `tables_insert_row`, `tables_delete_row`, `tables_clear_filters`, `tables_duplicates`, `tables_remove_duplicates`, `tables_promote_header`, `tables_key_check`, `tables_set_key`, `tables_clear_key`, `tables_extract_columns` |
| Colonnes | `columns_insert`, `columns_rename`, `columns_delete`, `columns_set_type`, `columns_type_preview`, `columns_type_mix`, `columns_fix_type`, `columns_values`, `columns_set_filter`, `columns_set_rules` |
| Interface | `ui_selection`, `ui_select`, `ui_select_object`, `ui_select_cells`, `ui_show_sheet`, `ui_show_view`, `ui_notify`, `ui_alert`, `ui_confirm`, `ui_prompt`, `ui_dialog` |
| Objets | `charts_create`, `charts_set`, `charts_place`, `charts_delete`, `pivots_create`, `pivots_set`, `pivots_delete`, `buttons_create`, `buttons_set`, `buttons_place`, `buttons_delete`, `buttons_list`, `buttons_run` |
| Scripts | `scripts_list`, `scripts_save`, `scripts_delete`, `scripts_check`, `scripts_run`, `formulas_functions` |
| Requêtes d'import | `queries_list`, `queries_save`, `queries_preview`, `queries_refresh`, `queries_refresh_all`, `queries_delete`, `connections_list` (sans mot de passe : une connexion PostgreSQL qui en demande un se déverrouille depuis l'interface) |

Une feuille protégée (Révision › Protéger la feuille, ou `sheets_protect`) refuse toute
écriture, outils `cells_*`, `tables_*`, `columns_*`, objets et `db_execute` compris, avec
le message « La feuille « X » est protégée : ôtez la protection (Révision › Ôter la
protection) pour la modifier ». La structure du classeur protégée (`sheets_protect_workbook`)
refuse d'ajouter, supprimer, renommer ou déplacer des feuilles. `doc_undo` et `doc_redo`
refusent aussi une action qui toucherait une feuille protégée.

`ui_dialog`, `ui_confirm` et `ui_prompt` ouvrent une boîte de dialogue dans la fenêtre et
attendent la réponse de l'utilisateur : l'assistant peut ainsi lui demander une
confirmation ou une saisie.

## Pour les développeurs

Les commandes sont définies une seule fois, dans `internal/api` (nom, paramètres, aide,
exemple, fonction Go). Le serveur (`internal/mcp`) en tire les outils et leurs schémas, les
scripts (`internal/script`) leurs fonctions Lua, et la référence de l'onglet SCRIPT
(`frontend/src/lib/script/api.gen.ts`) est générée par
`go test ./internal/api -run TestReference -update`. Ajouter une commande l'ajoute partout.
