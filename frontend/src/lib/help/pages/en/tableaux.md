# Tables

A table is a block of data typed, pasted or imported: Lattis recognizes it, types its columns and turns it into a table you can query in SQL.

## Detected tables {#tableaux-detectes}

Type a few rows anywhere on the sheet: a header row, then values. Lattis recognizes the block of data and turns it into a **table**: the header is highlighted, each column gets a type, and the table gets a name you can change in the table name box, to the left of the formula bar (type the new name, then Enter). The first table on a sheet takes the tab's name (without spaces: “Sales 2026” gives `Sales2026`), the next ones Table1, Table2… Renaming the tab also renames the table that carries its name (not the other way around); that table doesn't show its name badge, since the tab already shows it.

- It takes at least two columns over two rows (three filled cells of a 2 × 2 square): a single-column list, or a single row, stays as free cells.
- The first row you type is the header, even when it looks like the next ones (“Name | City” then “Lea | Paris”), unless it holds a value that is not text: a number, a date, a time, a Boolean, a percentage, an amount (“Lea | 31”, “Country | 2024”, “10 | 20”). Then all the typed rows are data and the header goes just above them, with names “Column 1”, “Column 2”… in gray italics: type the name of each column there, or right-click the 1st row › “Use first row as headers” (also in right-click › Table): it becomes the header in its place, the row of “Column n” disappears, formulas follow, and a column goes back to numbers if its only text was the name. To use a number as a column name, type it after an apostrophe ('2024). With no free space above (first row of the sheet, a cell, a title or a table just above), the cells stay free: Home › Format as Table (`Ctrl+L`) makes them a table, its first row giving the headers.
- A row typed just below a table extends it; so does a column typed just to its right.
- A block pasted from another program becomes a table, with the same header rule as typing: its 1st row is the header if it holds only text; a number, a date… (“Country | 2024”, “10 | 20”) makes it a data row, under “Column 1”, “Column 2”… (right-click › “Use first row as headers” to take it back). Pasted on the first row of the sheet, with no room above, the “Column n” header takes the place of the 1st pasted row and the block moves down one row.
- Headers typed after the data: type the names on the row just above the table. As soon as there are two names above a header that looks like data (“Lea | 31”, “Column 1”), the status bar offers “Use as headers”; otherwise, right-click one of these cells › “Use as headers of “Table1”” (or right-click the header › Table › “Use row above as headers”). The names replace the column names (a cell left empty keeps its own), the old header becomes the 1st data row, as numbers if it was numbers, and formulas follow; `Ctrl+Z` undoes it. A single word typed above stays a title.
- Drag a table next to another one to merge them.
- `Del` on cells of a table: a row with nothing typed left in it is deleted, as if it had never been typed (a calculated column, the hidden column of a choice tab or the number given automatically by an integer key don't keep it; the status bar says “N empty rows deleted”, `Ctrl+Z` brings them back). A row that was already empty, where `Del` cleared nothing, stays.
- Whatever is not in a table stays a **free cell**: a title, a parameter, a total.

Everything can be undone with `Ctrl+Z`, including creating a table.

**Right-clicking** in a table first offers what only applies there (Go to, Use first row as headers, Take this row out of the table on the last row, Use the row above as headers on a header…), then Cut, Copy, Paste; Clear contents, Format cells… (`Ctrl+1`), Conditional formatting…; the note. Last come the submenus of each object, titled with its name: **Sort & filter**, **Row “12”** (its key when the table has one: insert above or below, delete the row), **Column “Name”** (rename, type and key, validation, filter the tab, convert to values, insert left or right, delete the column) and **Table “Readings”** (headers, sharing, Convert to range, Delete table). Row numbers, column letters, PivotTables (**PivotTable “Sales”**), charts and buttons follow the same order.

### Header groups (row above) {#groupes-d-en-tetes-ligne-du-dessus}

When a row of groups sits above the headers (WOODLAND over Name, Status and Plot), right-click the header › Table › “Headers on 2 rows (groups above)”: these cells, merged or not, become the table's groups, shown above the headers. A column is then called “Group.Name”: `Inventory[WOODLAND.Name]` in a formula, `"WOODLAND.Name"` in SQL. Type in the group band to name or rename a group (formulas follow), clear it to remove it. “Remove header groups” puts the full names back on a single row.

A column without a group stays **on one level**: its cell in the band and its header form a single cell, merged over both rows, and its name has no dot. Lattis adapts on its own: a group over a single column whose header has no real name (Topography above “Column 17”) becomes the column's name, and a group that repeats the header (DISCOVERER above Discoverer) disappears. To adjust by hand, with Home › Merge & Center:

- **a column's band cell and header** › Merge: the column goes on one level; it keeps its sub-header, or takes the group's name if its header was automatic;
- **a column on one level** › Unmerge (the same button): the band cell separates from the header; type the group there (if you type nothing, it stays on one level). Typing “Group.Name” in the header does the same;
- **several adjacent band cells** › Merge: a single group over these columns, the first one they carried (“Group” if there was none: type its name in the band);
- **a group over several columns** › Unmerge: the group stays on its first column, the others go on one level.

Each time, the columns are renamed (formulas and SQL follow), in a single undo step, and the status bar says what changed. Merging the band with data, with cells outside the table or with another table is refused.

## Column types {#types-de-colonnes}

Each column has a type, guessed from what you type:

| Type | Examples |
| --- | --- |
| Text | Paris, Ref-001 |
| Whole number | 12, -3 |
| Decimal | 12.5; $1,234.50 |
| Date | 01/15/2026; with a time: 01/15/2026 2:30 PM or 14:30 01/15/2026 |
| True/False | TRUE, FALSE |
| Calculated | a formula applied to the whole column |

A value that no longer matches the type widens the column when possible (a decimal in a column of whole numbers turns it into decimals, and the status bar says so); otherwise the cell is marked as an error. The type is changed in the column header menu.

The text “null” typed, pasted or imported (CSV, .xlsx) leaves the cell empty; type 'null to keep the word. In a table, an empty cell shows a grey dash “–” (nothing is copied or exported); an empty free cell stays blank.

## Sort and filter {#trier-et-filtrer}

The ▾ of a column header (or right-click, or Data › Sort & Filter) sorts the table by that column: from A to Z, from Z to A, or in a **custom order**. Sorting changes the order of the rows for good; empty cells always go last. The same menu filters the column by values (“(Blanks)” at the end of the list) or by condition.

### Custom sort {#tri-personnalise}

**Custom sort…** opens the list of the column's values, to arrange in the order you want: drag them, use the Move up, Move down, Move to top, Move to bottom buttons, or the keyboard `Alt+↑` / `Alt+↓` (`Alt+Home` / `Alt+End`). Choose **In this order** or **Reversed**, then OK. Values are compared without regard to case; those not in the order follow in ascending order, and blanks stay at the end.

- **Built-in lists**: Days (Monday… Sunday), Short days, Months (January… December), Short months. When the column's values match a list, it is offered right away.
- **Workbook lists**: check “Save this order as a custom list” and name it (Sizes, Priorities…); it is then offered for any other column or PivotTable field, and travels with the file.
- The order is **kept for the column**: it is reused at the next Custom sort, and a table loaded by a query is sorted again in this order on every refresh (as long as an ordinary sort hasn't replaced it). Values of the order that are missing from the data stay in it, for when they appear.
- `Ctrl+Z` undoes the sort in one step. In scripts: `tables.sort_custom`, `tables.sort_lists`, `tables.set_sort_lists`, `columns.sort_order`.

## Keys {#cles}

A **key** is a column whose values are unique and all filled in: a customer code, a product reference. It identifies each row, is shown in the margin instead of the row numbers and is used for [REFERENCE](fonction:REFERENCE) lookups.

When you create a table (typing, pasting, Format as Table), its first column automatically becomes the key if it contains only whole numbers, unique and all filled in, and its header is **ID**, or its values are **1, 2, 3…** in order. The status bar says so (“key set on ID”) and the 🔑 appears in the header; `Ctrl+Z` undoes it along with the creation, right-click › Remove key removes it alone. An imported file keeps its tables as they are.

In other cases, when the first column of a table lends itself to it, Lattis discreetly suggests it, and one click is enough. “Don't suggest again” removes the suggestion for that table, which then keeps its row numbers.

**Automatic numbering.** A row added to a table whose key is a whole number gets the next number (the largest plus 1), whether it is typed under the table, pasted, inserted in the middle or added by a script; the other rows are not renumbered. A first column that follows a regular series without being the key (10, 20, 30…) is also filled in for a row added at the end. A value you type in the column replaces the suggested number. The header tooltip indicates a numbered column.

When you type a lookup ([REFERENCE](fonction:REFERENCE), VLOOKUP, XLOOKUP, EXISTS…) in a table that has no key, Lattis asks you what it should use:

- **the row number** of the table: 1 for the first row, in the table's order, which changes if you sort or insert rows. A VLOOKUP becomes `INDEX(…, MATCH(…, 0))`: the first row found, duplicates allowed. The choice applies to the whole table: the question doesn't come back;
- **the looked-up column**, which becomes the table's key: its values must be unique and all filled in, and a duplicate will then be refused there.

`Ctrl+Z` undoes the formula and this choice at once. A table that already has a key asks nothing, and a pasted or imported formula keeps the row numbers without asking.
