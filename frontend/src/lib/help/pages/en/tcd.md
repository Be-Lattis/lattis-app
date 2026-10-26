# PivotTables

A PivotTable summarizes a table: totals by customer, by month, by region… Lattis calculates it in SQL (`GROUP BY`), which stays fast on a million rows.

## Create a PivotTable {#creer-un-tableau-croise}

1. Click in the source table.
2. Insert › PivotTable. The status bar says “Click the cell where to place the PivotTable (Esc: cancel)”.
3. Click the cell where the PivotTable's top-left corner should go. Nothing is blocked in the meantime: you can scroll the sheet, move with the keyboard, change tabs (the PivotTable is then placed on the other sheet) or use the ribbon. `Esc` (or **Cancel** in the status bar) gives up.
4. The PivotTable is placed and the **PivotTable Fields** pane opens on the right.

To place it alone on a new sheet, as Excel does by default, click **New sheet** in the status bar instead of choosing a cell.

## Move a PivotTable {#deplacer-un-tableau-croise}

- **Right-click › Move** on the PivotTable, then click the cell where its top-left corner should go (report filters included), on the same sheet or on another one; `Esc` cancels.
- Or drag the border of the selection, as for a table: a selection taken in the PivotTable (a single cell is enough) carries the whole PivotTable; with `Ctrl`, it is copied.
- Or cut the whole PivotTable (`Ctrl+X`) and paste it (`Ctrl+V`) elsewhere.

The PivotTable is placed exactly at the chosen cell. The tables and other PivotTables it covers or touches move aside to the right, keeping an empty column between them, and in turn push those they run into; clicking in another table therefore pushes it to the right of the PivotTable. Charts and buttons, which float above the grid, don't move. Each placement or move is undone in one go (`Ctrl+Z`).

The cells of a PivotTable can't be typed in one by one: double-click them to change its fields.

## The fields pane {#le-volet-des-champs}

The list at the top shows the source table's columns, with the icon of their type; search filters the list. Check a field to add it, or drag it into one of the four areas:

| Area | Role |
| --- | --- |
| Filters | Report filters, above the PivotTable |
| Columns | One PivotTable column per value of the field |
| Rows | One PivotTable row per value of the field (several fields are nested) |
| Values | The calculated numbers |

Each chip has its ▾ menu: move, remove, filter and, for a value, the summary and the display. With the keyboard: `Tab` goes from one item to the next, `Enter` opens a chip's menu, `Del` removes it, `Alt+Up` / `Alt+Down` move it.

A calculated column can't be used as a field yet.

### Calculated field (group by a formula) {#champ-calcule-regrouper-par-une-formule}

**Calculated field…**, under the list of fields, adds a field whose value is calculated for each row of the source table by a formula on its columns, written between brackets: `=YEAR([Date])` groups by year, `=TEXT([Date],"mmmm")` by month, `=LEFT([Code],2)` by the first two characters of a code, `=IF([Amount]>1000,"Large","Small")` by bracket. The dialog suggests functions and columns as you type, and shows the first values the formula gives (or its error). The field joins the list with the ƒ icon and is placed like a column: in Rows or Columns, it groups the rows by its result; in Filters, it filters; in Values, it is summarized. Its ▾ menu in the list edits or deletes it (it then leaves the areas). The PivotTable recalculates it on every update; renaming a column of the source updates the formula. `Ctrl+Z` undoes it. Scripts: `calc = {{name = "Year", text = "=YEAR([Date])"}}` in the configuration of `pivots.create` or `pivots.set`.

## Value summary and display {#synthese-et-affichage-des-valeurs}

Summaries: Sum, Count, Distinct count, Average, Min, Max. A column of numbers is summed by default, the others counted. **Show values as** (Value field settings) also computes: % of grand total, of column, of row, of parent row or column, of parent total; % of, difference and % difference from an item (or the previous, the next) of a base field; running total and % running total; rank; index. A number format (currency, percentage, decimals) is chosen per value. A calculated column of the source table works as a field like the others.

## Columns, groups and slicers {#colonnes-groupes-et-segments}

Several fields in **Columns** give nested headers, with a subtotal per group and a −/+ box to collapse a group. **Group…** (▾ menu of a date or number field in the pane) arranges a date by years, quarters, months or days (several levels at once: Years and Quarters become fields before the date) and a number by intervals (start, end, step); **Ungroup** goes back to the values. **Insert slicer** places on the sheet a panel of buttons that filters the PivotTable: click for one value, Ctrl+click to add more, the funnel to take everything back; right-click to also filter other PivotTables on the same source. **Insert timeline** does the same with the periods of a date (Shift+click: a range).

## Sort and filter labels {#trier-et-filtrer-les-etiquettes}

The ▾ of a row label (or the ▾ menu of a chip in the pane's Rows and Columns areas) sorts the field: from A to Z, from Z to A, in the source's order, or with **Custom sort…**, which arranges the field's values in the order you want or according to a list (Days, Months, workbook lists), as for a [table column](prise-en-main). A label's ▾ also filters its values by checking them. With no sort chosen, labels follow the source table's order. The chosen order is kept in the PivotTable: when the data changes, a value of the list that appears takes its place, the others follow in ascending order. As in Excel, the field's empty cells always form the last item, whatever the sort: the last row (or column) before the total, and “(Blanks)” at the end of the list in the ▾ menu.

## Updating {#mise-a-jour}

The PivotTable follows changes in the pane after a short delay, in a single undo step. It also follows the source table's data. The pane reopens with a click in the PivotTable.

## Add columns on the right {#ajouter-des-colonnes-a-droite}

Type a name in the cell just to the right of the PivotTable's header: it gets a **typed column**, within its frame, with a green header (the rest of the PivotTable keeps its blue of calculated data). Its cells are edited like ordinary cells:

- **A value** is typed row by row (a note, a target, a comment). It follows its row label: when the PivotTable changes (source table data, sort, filter, fields), it stays next to the same label. A label that disappears puts its values **aside**; they come back with it. The fields pane says how many rows are set aside, and **Forget the values set aside** clears them (Ctrl+Z to undo). Removing all the fields in Rows sets all the values aside; they come back when the field comes back. Changing the field in Rows too: values are stored by field and by label.
- **A formula** (starting with =) applies to the whole column, like a calculated column: `=[Count]*2` reads the row's value in the PivotTable's “Count” column (or in another typed column), `=[Count]/SUM(TCD1[Count])` compares to the whole column (detail rows only), and an address such as `=B3` or `=A20*[Count]` reads a cell of the sheet. A formula is also calculated on total rows. A value typed in a formula column is refused; clearing one of its cells clears the formula (values typed before come back).
- **Total** and subtotal rows don't take a value.
- Rename: retype the header. Remove the column: clear its header (Del on the whole column, or header cleared), its values go with it (Ctrl+Z to undo).

Typed columns stay at the right edge of the PivotTable when it widens (one more value or field in columns), and push what is next to them, like the PivotTable itself. They move with it; a copy of the PivotTable also copies their values. A formula elsewhere reads them by their address (`=E5`), and the PivotTable's SQL view contains them.

## Use a PivotTable elsewhere {#utiliser-un-tableau-croise-ailleurs}

A PivotTable has a name (shown at the top of the pane): it can be queried in the [SQL tab](sql) like a table, and serves as a source for [charts](graphiques). The [GROUPBY](fonction:GROUPBY) function gives a simple grouping directly in a formula.

**In a formula**, a PivotTable reads like a named table, by its name (the blue badge at the top left of its frame): `=SUM(TCD1[Count])` reads the whole column (detail rows only, without groups, subtotals or total), `=TCD1[NID][2]` its second detail row, and its typed columns read the same way (`TCD1[test]`). While typing a formula, clicking in the PivotTable writes these references rather than an address: `[Count]` on the row of a formula typed in a typed column of the PivotTable, `TCD1[Count][2]` elsewhere, `TCD1[Count]` on a header or a total row. In a typed column, `[Count]` (or `[@Count]`) is also calculated on group, subtotal and total rows, with their own values; row fields finer than the row are empty there.
