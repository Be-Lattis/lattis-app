# Similar tabs and choice tabs

Combine tabs that share the same columns (one per month, year or city) into a single table, then show one value at a time.

## Combine tabs {#regrouper-des-onglets}

Many workbooks have **one tab per month, per year or per city**, with the same columns. Lattis spots them: their tabs are underlined with the same colored line, and a “⧉ 12 similar tabs” chip follows the series (hovering it highlights the tabs concerned). After importing an .xlsx file, on opening, or when you copy a tab of the series for the third time (even before renaming the copy), the status bar offers it once: **Combine…**, **Later** (the chip stays) or **Don't suggest again** (kept in the file: a colleague who receives it isn't asked again; also on right-click on the chip).

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
