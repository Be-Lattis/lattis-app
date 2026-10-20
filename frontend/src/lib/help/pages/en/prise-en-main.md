# Getting started

Lattis works like a spreadsheet: an endless grid, a ribbon at the top, sheet tabs at the bottom. The difference is underneath: every table you type becomes a real database table, typed and queryable in SQL.

## Detected tables {#tableaux-detectes}

Type a few rows anywhere on the sheet: a header row, then values. Lattis recognizes the block of data and turns it into a **table**: the header is highlighted, each column gets a type, and the table gets a name you can change in the table name box, to the left of the formula bar (type the new name, then Enter). The first table on a sheet takes the tab's name (without spaces: “Sales 2026” gives `Sales2026`), the next ones Table1, Table2… Renaming the tab also renames the table that carries its name (not the other way around); that table doesn't show its name badge, since the tab already shows it.

- It takes at least two columns over two rows (three filled cells of a 2 × 2 square): a single-column list, or a single row, stays as free cells.
- The first row you type is the header, even when it looks like the next ones (“Name | City” then “Lea | Paris”), unless it holds a value that is not text: a number, a date, a time, a Boolean, a percentage, an amount (“Lea | 31”, “Country | 2024”, “10 | 20”). Then all the typed rows are data and the header goes just above them, with names “Column 1”, “Column 2”… in gray italics: type the name of each column there, or right-click the 1st row › “Use first row as headers” (also in right-click › Table): it becomes the header in its place, the row of “Column n” disappears, formulas follow, and a column goes back to numbers if its only text was the name. To use a number as a column name, type it after an apostrophe ('2024). With no free space above (first row of the sheet, a cell, a title or a table just above), the cells stay free: Home › Format as Table (`Ctrl+L`) makes them a table, its first row giving the headers.
- A row typed just below a table extends it; so does a column typed just to its right.
- A block pasted from Excel or another program becomes a table, with the same header rule as typing: its 1st row is the header if it holds only text; a number, a date… (“Country | 2024”, “10 | 20”) makes it a data row, under “Column 1”, “Column 2”… (right-click › “Use first row as headers” to take it back). Pasted on the first row of the sheet, with no room above, the “Column n” header takes the place of the 1st pasted row and the block moves down one row.
- Headers typed after the data: type the names on the row just above the table. As soon as there are two names above a header that looks like data (“Lea | 31”, “Column 1”), the status bar offers “Use as headers”; otherwise, right-click one of these cells › “Use as headers of “Table1”” (or right-click the header › Table › “Use row above as headers”). The names replace the column names (a cell left empty keeps its own), the old header becomes the 1st data row, as numbers if it was numbers, and formulas follow; `Ctrl+Z` undoes it. A single word typed above stays a title.
- Drag a table next to another one to merge them.
- `Del` on cells of a table: a row with nothing typed left in it is deleted, as if it had never been typed (a calculated column, the hidden column of a choice tab or the number given automatically by an integer key don't keep it; the status bar says “N empty rows deleted”, `Ctrl+Z` brings them back). A row that was already empty, where `Del` cleared nothing, stays.
- Whatever is not in a table stays a **free cell**, as in Excel: a title, a parameter, a total.

Everything can be undone with `Ctrl+Z`, including creating a table.

**Right-clicking** in a table first offers what only applies there (Go to, Use first row as headers, Take this row out of the table on the last row, Use the row above as headers on a header…), then Cut, Copy, Paste; Clear contents, Format cells… (`Ctrl+1`), Conditional formatting…; the note. Last come the submenus of each object, titled with its name: **Sort & filter**, **Row “12”** (its key when the table has one: insert above or below, delete the row), **Column “Name”** (rename, type and key, validation, filter the tab, convert to values, insert left or right, delete the column) and **Table “Readings”** (headers, sharing, Convert to range, Delete table). Row numbers, column letters, PivotTables (**PivotTable “Sales”**), charts and buttons follow the same order.

### Header groups (row above) {#groupes-d-en-tetes-ligne-du-dessus}

When a row of groups sits above the headers (WOODLAND over Name, Status and Plot), right-click the header › Table › “Headers on 2 rows (groups above)”: these cells, merged or not, become the table's groups, shown above the headers as in Excel. A column is then called “Group.Name”: `Inventory[WOODLAND.Name]` in a formula, `"WOODLAND.Name"` in SQL. Type in the group band to name or rename a group (formulas follow), clear it to remove it. “Remove header groups” puts the full names back on a single row.

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
| Date | 01/15/2026 |
| True/False | TRUE, FALSE |
| Calculated | a formula applied to the whole column |

A value that no longer matches the type widens the column when possible (a decimal in a column of whole numbers turns it into decimals, and the status bar says so); otherwise the cell is marked as an error. The type is changed in the column header menu.

## Sort and filter {#trier-et-filtrer}

The ▾ of a column header (or right-click, or Data › Sort & Filter) sorts the table by that column: from A to Z, from Z to A, or in a **custom order**. Sorting changes the order of the rows for good; empty cells always go last. The same menu filters the column by values (“(Blanks)” at the end of the list) or by condition.

### Custom sort {#tri-personnalise}

**Custom sort…** opens the list of the column's values, to arrange in the order you want, like Excel's custom lists: drag them, use the Move up, Move down, Move to top, Move to bottom buttons, or the keyboard `Alt+↑` / `Alt+↓` (`Alt+Home` / `Alt+End`). Choose **In this order** or **Reversed**, then OK. Values are compared without regard to case; those not in the order follow in ascending order, and blanks stay at the end.

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

## Combine tabs {#regrouper-des-onglets}

Many workbooks have **one tab per month, per year or per city**, with the same columns. Lattis spots them: their tabs are underlined with the same colored line, and a “⧉ 12 similar tabs” chip follows the series (hovering it highlights the tabs concerned). After importing an Excel workbook, on opening, or when you copy a tab of the series for the third time (even before renaming the copy), the status bar offers it once: **Combine…**, **Later** (the chip stays) or **Don't suggest again** (kept in the file: a colleague who receives it isn't asked again; also on right-click on the chip).

Tabs go together when at least two thirds of their columns are found from one tab to the next: a column added during the year stays empty where it was missing, and a renamed column (“Amount $”) is paired with its neighbor (“Amount”), which the pane always shows. Tab names in a series are recognized: months, years, quarters, weeks, dates, sequences of numbers or letters (“Lot 1”, “Lot 2”; “Appendix A”, “Appendix B”), municipalities.

**Data › Combine tabs…** (or right-click a selection of tabs) opens the pane:

1. **Tabs**: the tables to combine, to check, in tab order;
2. **Category column**: its name, prefilled from the tab names (Month, Year, Quarter, City, the common part such as “Lot”, otherwise Category) and always editable, its type, and the value taken from each tab, to correct if needed. It goes first in the table, with a drop-down list of its values;
3. **Columns**: the matching (✓ present, ∅ missing, “Amount $ → Amount” paired) and the final type; a column that becomes text is flagged;
4. **After**: the name of the table and of the new sheet, and what becomes of the old tabs: **deleted** (by default) or **kept hidden**, their tables renamed `January_ancien`;
5. **Preview**: the first rows and the total number of rows, then the summary of the formulas that referred to the old tabs: “14 formulas carried over, 2 to fix by hand”, with the list (sheet, cell, formula before → after).

**Combine** does everything in a single step: `Ctrl+Z` gives back the original tabs. Formatting and entries in error follow their rows. The **key** is kept if no value repeats from one tab to another; otherwise the combined table has no key (the values stay), and Lattis says so with an example: “No key: F-001 is in January and in February”.

Whatever referred to the old tables is **carried over** to the combined table, restricted to the month (the year, the city) of the old tab, in the same undo step and with the same values. A “Summary” sheet that did `SUM(January[Amount])`, `SUM(February[Amount])`… is carried over like this:

| Before | After |
|---|---|
| `SUM(January[Amount])` | `SUMIFS(Sales[Amount], Sales[Month], "=January")` |
| `AVERAGE`, `MAX`, `MIN` of a column (or of a calculation on the columns, `SUM(January[Price]*January[Qty])`) | `AVERAGEIFS`, `MAXIFS`, `MINIFS`, `SUMIFS` |
| `COUNTA(January[Customer])` | `COUNTIFS(Sales[Customer], "<>", Sales[Month], "=January")` |
| `SUMIF`, `COUNTIF`, `SUMIFS`… | the same, as `…IFS`, with the month criterion added |
| `ROWS(January)` | `COUNTIFS(Sales[Month], "=January")` |
| `COUNT`, `MEDIAN`, several columns in `MAX` or `AVERAGE`, `UNIQUE(January[Customer])`… | the filtered column: `FILTER(Sales[X], Sales[Month]="January")` |
| a PivotTable on January | the same on Sales, with a report filter Month = January |
| a chart on January | the same on Sales, with a filter Month = January (shown in its pane, removable) |

Calculated columns of other tables and conditional formatting rules follow the same rules. Left **to fix by hand**, with the reason: a specific row (`January[Amount][3]`), `REFERENCE`, `VLOOKUP` and `EXISTS` (they look up a key in a named table), `SUBTOTAL`, the whole table (`FILTER(January, …)`), a check list taken from an old table, a query; SQL text and Lua scripts are not detected. In that case, the old tabs are kept hidden by default so that these formulas keep working; otherwise they are deleted. Without a category column, nothing tells the tabs apart any more: everything is left to fix. Afterwards, a summary by month is better off as a Month × Amount PivotTable, which follows new rows on its own.

Combining is refused on a protected sheet, on a filtered table (remove the filter first), on a table loaded by a query or shared on a server.

### Tabs with several tables {#onglets-a-plusieurs-tableaux}

A “2025” tab often holds several tables: sales and expenses, which change every year, but also a price list copied as is from one tab to the next. Lattis then combines **whole sheets**: it pairs the tables from one tab to the next, family by family (same columns, as above), and the pane shows, under **Other tables on the tabs**, what becomes of each one:

- **combine them** (tables that change: sales, monthly notes): a combined table with the category column, which **follows** the choice tab;
- **keep a single copy** (offered when the data is the same everywhere: the price list): the one from the last tab, without a category column, the same for all years. If a copy differs (“Prices changes in 2024”), the pane flags it and suggests combining them instead, so as to lose nothing;
- **leave them on their tab** (offered for a table found in only one tab): the tab is then kept, visible, with an orange tab to find it again, and the pane says so.

The combined sheet takes the **layout of the most recent tab**: each combined table takes the place its own had, whatever is below moves down by as much as it grew, and tables missing from that tab come next, below. The tabs' **free cells** are carried over (check box to clear in the pane): a title, a label or a total that is identical everywhere is kept only once; `=SUM(Sales2025[Amount])` under the table becomes `=SUM(Sales[Amount])`, which gives the total of the displayed year; a value that changes from one tab to another (“Target: 5,000” in 2024, “6,000” in 2025, a title “January 2025”) becomes a **per-year cell**: it shows the value of the chosen year, and what you type there only applies to that year (empty with “(All years)”).

The category can also come from a **cell** rather than from the tab name (a title “January 2025” in A1 of each tab, when the tabs are called Sheet1, Sheet2…): the pane suggests the most reliable source, to choose in **Taken from**. Similar tables **stacked on the same sheet** under titles (“Paris”, “Lyon”, “Marseille”) are combined the same way, the category coming from the titles. If each tab has a **Date** column whose values all fall in its month, the Month column is pointless: the “Don't add a column” box is checked by default (filtering by date is enough); clear it to add the column anyway.

**Follow the year**: on a sheet with a choice tab, a table that follows carries a small **badge** with the displayed year in the corner of its frame; a table without a badge is the same for all years. Right-click the table › Table › **Follow the year** (or click the badge) checks or clears it: checking adds the Year column last, filled with the displayed year (hidden while a year is chosen); clearing removes it if it has only one year, otherwise keeps it (nothing is lost) and the table shows all its rows. A table formatted (Insert › Table) on a sheet set to a year follows it automatically; the status bar says so. `Ctrl+Z` undoes it. Scripts: `tables.follow_choice("Prices", false)`.

## Formulas {#formules}

A formula starts with `=` and refers to columns by their name. Typed in a table column, it applies to the whole column:

```
=[Quantity]*[Unit price]
```

See [Formulas](formules) for references and the list of functions. While typing, `F1` opens the page of the function under the cursor.

## Choice tab (one year, one month, one city at a time) {#onglet-a-choix-une-annee-un-mois-une-ville-a-la-fois}

Instead of one tab per year, keep a single table with a Year column and make its sheet a **choice tab**: right-click the tab › **Choose by** › the column (Lattis suggests those with few different values). The tab becomes “Sales · Year ▾”; its small arrow (or `Alt+↓` on the tab) drops down the list of values:

- **a value** (2025): all the tables on the sheet that have a column with that name show only its rows, and the Year column, the same everywhere, is hidden. Tables without that column (a price list) don't change. The sheet's PivotTables only count that year, like an imposed report filter; `SUBTOTAL`, charts and search follow it as they follow a filter;
- **(All years)**: nothing is hidden any more, the column reappears;
- **New year…** suggests the next one (2026 after 2025, December after November, Lot 4 after Lot 3), adds it to the column's drop-down list and shows it: the sheet looks empty, ready to be filled in, like a new tab. `Ctrl+Z` undoes it.

**From the column**: on a table that is already filled in, right-click the Year column header (or its ▾ menu) › **Filter the tab by this column** does the same, without going through the tab or combining tabs. The tab starts on all values: choose the year with its ▾ arrow. The item is grayed out, with a tooltip explaining why, for a calculated column, an empty one or one with more than 100 different values; on the column already used as the choice, it becomes **Stop filtering the tab**. `Ctrl+Z` undoes it.

**By a formula**: right-click the tab › **Choose by** › **A formula…** when the value to choose isn't a column but is calculated from each row's columns: `=YEAR([Date])` gives one tab per year without a Year column, `=TEXT([Date],"mmmm")` per month, `=IF([Amount]>1000,"Large","Small")` per bracket. The dialog suggests functions and columns as you type, guesses the category name (Year, Month…, editable) and shows the formula's first values and the sheet's tables that will follow it: those that have all the columns it refers to (the others don't change; right-click › Table › **Follow** removes them or puts them back). The tab's list sorts the values in natural order (2024 before 2025, January before February). No column to fill in, so no “New year…”: a row typed under the table gets nothing automatically, but it stays displayed while you type it, until the tab's value next changes, when it joins the value its formula gives. `=SHEETCHOICE()` returns the chosen value (a number for `=YEAR(…)`); the sheet's PivotTables built on a table that has the formula's columns only count the chosen value. Renaming a referenced column updates the formula. Scripts: `sheets.choose_by_formula("Sales", "=YEAR([Date])", "Year")`.

**From a PivotTable**: right-click a field in rows, columns or filters in the PivotTable pane (or its ▾ in the grid) › **Filter the tab by this field**. The choice tab is set on the PivotTable's sheet, by the column of its source table: the PivotTable only counts the chosen value, like the sheet's other PivotTables built on a table that has this column, and the sheet's tables that have it. The source table, if it is on another sheet, is not filtered; its values fill the tab's list. This also works for a PivotTable built on a table loaded by a query.

A row typed under the table, or in the grayed band of the other years' rows, automatically gets the displayed year; the same goes for a pasted or inserted row. The choice is not a filter: “Clear filter” leaves it alone, and your ▾ filters add to it. `Ctrl+Alt+PgDn` and `Ctrl+Alt+PgUp` go to the next or previous year; the status bar reminds you “Year: 2025 (tab choice)”.

Free formulas **on the sheet** see its tables filtered: `=SUM(Sales[Amount])` placed under the table gives the total of the displayed year, as in the old “2025” tab. Those on other sheets, the SQL tab and scripts see all rows. `=SHEETCHOICE()` returns the chosen value, for a summary that follows the tab: `=SUMIFS(Sales[Amount], Sales[Year], SHEETCHOICE())`.

Changing the year is navigation, like changing tabs: it isn't an undo step, but the last chosen value is kept in the file and comes back on opening (if it no longer has any rows, the tab shows the last value, except right after “New year…”). Renaming the column renames the category: the tab follows. Right-click › Choose by › “Stop choosing by…” removes the choice (can be undone). In a shared workbook, the choice stays on your computer: it isn't sent to the others.

Scripts: `sheets.choice()`, `sheets.set_choice("Sales", 2025)`, `sheets.choose_by("Sales", "Year")` (for the sheet of a PivotTable placed elsewhere: `sheets.choose_by{table = "Sales", column = "Year", sheet = "Summary"}`), `sheets.new_choice("Sales", 2026)` (see [Lua](lua)).

## Save {#enregistrer}

A workbook is a single `.lattis` file (a SQLite database and its description). `Ctrl+S` saves. Closing the workbook or the Lattis window with unsaved changes first asks whether to save them; if Lattis stops without saving (crash, forced shutdown), the changes are offered for recovery at the next start (see [Files and recovery](fichiers)).

The undo history is never saved in the file: `Ctrl+Z` works until the workbook is closed, even after `Ctrl+S`, but a reopened file (or one sent to someone) no longer has any history, nor the deleted columns or rows.

A workbook opened from a shared folder (network, OneDrive…) is signaled to other computers by a small hidden `~$name.lattis` file placed next to it: another computer that opens it is warned and can open it read-only. If the file was changed elsewhere since it was opened, `Ctrl+S` offers to save a copy, overwrite or cancel.

File › Import reads CSV and Excel files; File › Export writes CSV, `.xlsx` or OpenDocument (`.ods`) (in a CSV for Excel, a text that starts with `=`, `+`, `-` or `@` is preceded by an apostrophe, so that Excel doesn't run it as a formula); File › Print / PDF prints or saves as PDF, according to View › Page Layout (see [Page layout and printing](impression)).

## Protect a sheet or the workbook {#proteger-une-feuille-ou-le-classeur}

Review › Protect Sheet refuses any change to the sheet's content; Protect structure only allows entry in tables (values, added or deleted rows, sort, filter); Protect Workbook refuses adding, deleting or renaming sheets, and can also lock the structure or the content of all sheets. An optional password prevents removing the protection by mistake. As in Excel, this is protection against **accidental changes**, not security: the file is not encrypted, and someone with the right tools can read its data or remove the protection without the password. Don't rely on it for confidential data.

## Next steps {#et-ensuite}

- [PivotTables](tcd) and [charts](graphiques) to analyze.
- [The SQL tab](sql) to query tables.
- [Lua scripts](lua) to automate.
- [Import data](connexions) from files, databases or the web.
