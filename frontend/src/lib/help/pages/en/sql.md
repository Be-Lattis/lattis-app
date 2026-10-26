# SQL queries

Each table in the workbook is a SQLite table. The **SQL** tab (to the right of the ribbon tabs) queries them with the names you see on screen.

## Write a query {#ecrire-une-requete}

```
SELECT Customer, SUM("Net amount") AS Total
FROM Orders
WHERE Paid = 0
GROUP BY Customer
ORDER BY Total DESC
```

- A name with a space, an accent or a capital letter goes between double quotes: `"Net amount"`.
- `Ctrl+Enter` runs the selection, otherwise the statement under the cursor (statements are separated by `;` or an empty line).
- `Ctrl+Space` suggests table and column names; `Alt+Up` / `Alt+Down` go through the history.
- The result shows 1,000 rows at most.
- The **Reference** button (or `F1` on a keyword in the editor) opens the common syntax, useful functions and Lattis-specific rules; each entry has an example to insert, written with your tables' names.

[PivotTables](tcd) are queried like tables. `Cell_A1` refers to a free cell of the displayed sheet.

## The left pane {#le-volet-de-gauche}

Three lists: the workbook's **objects** (tables, PivotTables and their columns; a click goes to the table on its sheet), **favorites** (queries saved in the workbook) and the **history**.

When signed in to a Lattis Server, the objects also show the **Lattis Data Lake data items** you can read, in their own group: `donnees.name` views (double-click: sample query). A query that refers to them runs on the server, read-only and limited to the rows of your audience; it is written in PostgreSQL's SQL and can't refer to the workbook's tables (to combine them, insert the data item into the workbook: Insert › Lattis Data Lake).

## Send the result to a sheet {#envoyer-le-resultat-sur-une-feuille}

A query's result is copied into a new table, or **referenced** by it: the table then stays linked to the query and is reread with right-click › Table › Refresh query (or `F5`). It keeps the result's headers; technical columns (`_pos`) never come into it.

### Write next to a linked result {#ecrire-a-cote-d-un-resultat-lie}

The columns loaded by the query (blue header, or the external sources color of the theme chosen in View › Theme) can't be edited. To add your own information, type a name just to the right of the header: the new **typed column** (green header) is filled in by hand, or by a formula, and the workbook's formulas see it like the others (`Query1[Comment]`).

On each refresh, its values follow their rows by a **key**: the result's `id` column when there is one (as with `SELECT *`), otherwise a column without duplicates or empty cells that Lattis asks for at the first value you type. When no single column fits, Lattis suggests pairs: with `SELECT *, 2021 AS Year FROM Tracking2021 UNION SELECT *, 2022 FROM Tracking2022`, the same `id` values come back from one year to the next, and it's “id + Year” that identifies the rows. The row rank is possible but not recommended: if the query changes the order of its rows, the values no longer follow.

- A row that disappears from the result: its values are **set aside**, and come back if its key comes back. Right-click › Table › Forget the values set aside clears them.
- A duplicate key in the result: the values for that key are set aside, with a warning.
- Renaming or deleting a typed column is done as in an ordinary table; the key is changed with right-click › Table › Key of typed columns…
- Everything is undone with `Ctrl+Z`, refresh included.

The same goes for a table loaded by a query from the editor ([Import data](connexions)).

## Change the data {#modifier-les-donnees}

`INSERT`, `UPDATE` and `DELETE` queries change tables like an entry; they are undone with `Ctrl+Z`. While a query runs, the **Stop** button interrupts it; a read stops on its own after 30 s, a write after 2 min (it blocks entries while it lasts). A stopped write changes nothing.

## A query in a formula {#une-requete-dans-une-formule}

The [QUERY](fonction:QUERY) function runs a read-only `SELECT` and spills its result onto the sheet:

```
=QUERY("SELECT Customer, SUM(Amount) FROM Sales GROUP BY Customer")
```

## In scripts {#dans-les-scripts}

`db.query`, `db.rows` and `db.execute` go through the same SQL: see [Lua scripts](lua).
