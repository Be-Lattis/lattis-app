# Lua scripts

The **Script** tab automates the workbook in Lua (version 5.1): read and write tables, drive sheets, ask the user questions, react to controls.

## The Script tab {#l-onglet-script}

- **On the left, the library**: the scripts stored in the workbook (they travel with the `.lattis` file). Right-click: rename, duplicate, delete.
- **In the center, the editor**, which checks the script as you type: syntax errors in red, unknown names and wrong numbers of arguments in yellow.
- **On the right, the reference**: the functions below, with a button to insert their example.
- **At the bottom, the output**: what `print` displays, then the summary of the run.

`Ctrl+Enter` runs the script, `Ctrl+S` saves it in the workbook (then remember to save the workbook).

## Read and write {#lire-et-ecrire}

Queries are written in SQL with the displayed names of tables and columns; a name with a space or an accent goes between double quotes:

```
for r in db.rows('SELECT Customer, sum("Net amount") AS total FROM Orders GROUP BY Customer') do
  print(r.Customer, r.total)
end
```

`db.insert`, `db.update` and `db.delete` write like typing in the grid. Everything a script writes forms **a single undo step**: `Ctrl+Z` clears it in one go.

## Security and limits {#securite-et-limites}

A script runs in a sandbox: no access to files, programs or environment variables. HTTP requests (`http.get`…) only reach the sites you have allowed. A script is stopped after 30 seconds, beyond 512 MB of memory, or with the Stop button. `db.query` returns 100,000 rows at most (`db.rows` has no limit).

A script received with a workbook (button, control, event) asks for your approval before it first runs: **Allow** or **Deny**, with **View code**. A script can read and change the whole workbook: only allow it if you trust the person who gave you this workbook. Scripts written on this computer are allowed automatically.

On opening, Lattis deletes from the workbook the SQL triggers (`TRIGGER`) it didn't create itself (Lattis never stores any in the file) and says so in the status bar.

## Buttons and controls {#boutons-et-controles}

A function of a script is attached to a button or a control on the sheet: see [Buttons and controls](controles).

## Workbook events {#evenements-du-classeur}

A script can react to what you do in the workbook: `events.on(event, function, script)` subscribes a function of a saved script, and the subscription travels with the file.

```
function entry(e)
  ui.notify(e.address .. ": " .. tostring(e.old) .. " → " .. tostring(e.value))
end
events.on("change", "entry", "My script")
```

| Event | When |
|---|---|
| `open` | the workbook has just been opened |
| `before_save` / `save` | before / after saving (returning `false` cancels the save) |
| `before_close` | before the workbook or the window closes (returning `false` cancels it) |
| `sheet` | another tab is displayed |
| `selection` | the selection has changed (once it stops moving) |
| `change` | cells have been changed: entry, paste, clearing, move, table row |
| `sheet_add` / `sheet_delete` | a sheet is added / deleted |
| `recalc` | formulas are recalculated (F9) |

The function receives a table: `sheet`, `row`, `col`, `address` (“B3”), `value` and `old` for a single typed cell… Only your actions trigger these events, never a script: a listener that writes to the sheet doesn't trigger itself again. Listeners run without blocking the grid, what they change is undone in one go (`Ctrl+Z`), and an error is reported only once. `events.enable(false)` turns them off for the session.

## Function reference {#reference-des-fonctions}
