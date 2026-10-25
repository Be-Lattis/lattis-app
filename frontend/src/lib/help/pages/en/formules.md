# Formulas

A formula starts with `=`. Functions are written with their English names (`SUM`, `IF`, `COUNTIF`…). Arguments are separated by `,` and decimals are written with a period: `=ROUND([Price]*1.2, 2)`.

While you type, Lattis suggests functions, tables and columns, shows the signature of the current function (the argument to enter in bold) and colors the references. `F1` opens the page of the function under the cursor here; Formulas › Insert Function opens the full list.

## References {#references}

| Notation | Refers to |
| --- | --- |
| `[Column]` | In a table: the column's value for the formula's row |
| `[Column][2]` | The column's value on the 2nd row of the same table |
| `Table[Column]` | The whole column (in an aggregate, or outside the table); alone in a free cell (`=Sales[Price]`), its values spill below |
| `Table[Column][2]` | One value, by its row's rank (1 = first data row) |
| `Table[Column][1:3]` | Rows 1 to 3 of a column |
| `Cell[A1]` | Free cell A1 of the sheet |
| `Sheet2!Cell[A1]` | Free cell A1 of another sheet (`'My sheet'!Cell[A1]` if the name has spaces) |
| `Cell[C14:C16]` | A range of free cells |

Ranks follow the display order: sorting a table changes the row referred to, as in Excel. To look up a row by its key, use [REFERENCE](fonction:REFERENCE).

In formula mode, clicking a cell inserts its reference; dragging inserts a range. From a free cell, clicking a cell of a table inserts that single value (`Report[TEST][2]`, by its row's rank); clicking the header, or dragging over the whole column, inserts the entire column (`Report[TEST]`).

## Excel-style addresses {#adresses-a-la-excel}

An address typed out of habit (`=C14*2`, `=SUM(C14:C16)`, `$C$14`, `Sheet2!B3`) is converted when you confirm: to `Cell[C14]` for a free cell, to `Table[Column][k]` in a table, to a range for `C14:C16`.

## Calculated columns {#colonnes-calculees}

A formula typed in a table column applies to all its rows: the column becomes **calculated**. `[Column]` there refers to the value on the same row. Aggregates accept an expression on a table's columns, calculated row by row: `=SUM(Sales[Price]*Sales[Qty])`.

### Convert to values {#convertir-en-valeurs}

Right-click a calculated column › **Convert to values** (or Formulas › Convert to values): the formula is removed and the column becomes an ordinary column that keeps its current values, with the type of those values (whole number, decimal, date, Boolean, otherwise text; an error such as `#DIV/0!` stays flagged). It keeps its name: formulas that refer to it keep reading it, but it no longer follows the columns its formula read. Several selected calculated columns are converted together; on free cells, the command replaces their formulas with their values (a spilled result keeps its formula). Ctrl+Z brings the formulas back. Refused under a protected structure, and for now in a shared table of a collaborative workbook.

## Results that spill {#resultats-qui-se-deversent}

Some functions return several values ([FILTER](fonction:FILTER), [SORT](fonction:SORT), [UNIQUE](fonction:UNIQUE), [SEQUENCE](fonction:SEQUENCE), [TEXTSPLIT](fonction:TEXTSPLIT), [QUERY](fonction:QUERY)…): the result spills into the neighboring cells, like Excel's dynamic arrays. A whole column alone in a free cell (`=Sales[Price]`) spills the same way, in the table's row order. If the space is taken, the formula shows `#SPILL!`. In an aggregate, they provide their values: `=COUNTA(TEXTSPLIT(Cell[A1], ","))` counts the pieces of a text; functions that scan a column (COUNTIF, LARGE, MATCH…) don't read them (`#VALUE!`). UNIQUE, SORT, TAKE and DROP read a column (`=UNIQUE(NestTracking[Nest ID])`), a whole table, a range of cells (`Cell[A1:A20]`) or another dynamic array; FILTER and SORTBY, a table or a column.

`=TEXTSPLIT("Name=Smith;City=Paris", "=", ";")` splits a text into rows (at `;`) then into columns (at `=`); shorter rows are padded with `#N/A`, or with the last argument. To spill, the text must not depend on a table row: a cell, or a value such as `Sales[Notes][2]`.

An **array constant** is written between braces: `{1;2;3}` (a column), `{"a","b"}` (a row), `{1.2;3.4}`. It spills like a dynamic array and is useful in particular to give several delimiters: `=TEXTSPLIT(Cell[A1], {","," "})`.

### Convert a spilled result to a table {#convertir-un-resultat-deverse-en-tableau}

The spilled area is framed in blue; its tooltip recalls the formula's cell. **Right-click** this area:

- **Convert to linked table**: a real table replaces the result. It stays linked to the formula (kept in a query of the workbook, “Formula” source, visible in Data › Queries) and **recalculates on its own** when its sources change: an entry in `NestTracking` immediately updates the table taken from `=UNIQUE(NestTracking[Nest ID])`, and a single Ctrl+Z undoes both the entry and the recalculation. Its calculated columns are read-only (in blue, the theme's color for external sources); a name typed to the right of its header adds a **typed column** to it (in green, the color of tables) whose values follow their row when the result changes (by the column itself for a `UNIQUE` of a column, otherwise by the requested key), as next to a query. F5 also refreshes it. If you add steps to its query (Data › Queries), it then only refreshes with F5.
- **Convert to table (values)**: an ordinary table keeps the current values, with no link.

In both cases, **the headers** are the texts placed just above the formula, if there is one above each column (a title “Nests” above `=UNIQUE(…)` becomes the header of the table, named Nests); otherwise the names of the formula's columns (“Nest ID”, “Value” for SEQUENCE), on the empty row above the formula (the values don't move) or, failing that, on the formula's row (the values move down one row). A single undo step brings the formula back.

## Numbers written as text {#nombres-ecrits-en-texte}

In a text column, functions that expect numbers ([SUM](fonction:SUM), [AVERAGE](fonction:AVERAGE), [MIN](fonction:MIN), [MAX](fonction:MAX), [SUMIF](fonction:SUMIF), [SUMPRODUCT](fonction:SUMPRODUCT), statistics…) read each text as the number it spells: “12”, “1,234.5”, “$12”, “15%” (0.15), “(12)” (−12), and foreign notations “1.234,56” or “1'234.5”. **A text that isn't a number counts as 0**: the average of “12” and “abc” is 6. Empty cells are still ignored. [COUNT](fonction:COUNT) only counts the texts that are numbers: `=COUNTA(Sales[Score]) - COUNT(Sales[Score])` gives the number of texts that aren't.

This is not the case in Excel, which ignores these texts. A range of free cells (`Cell[A1:A10]`) follows Excel: its texts are labels, ignored. In a direct calculation (`=[Score]*2`), a text that isn't a number gives `#VALUE!`.

## Intermediate names {#noms-intermediaires}

[LET](fonction:LET) names values, calculated once, usable in the following values and in the final calculation: `=LET(net, [Price]*[Qty], tax, net*0.2, net+tax)`. A name can be neither a cell address (`A1`, `Cell[A1]`) nor a function name.

## Error values {#valeurs-d-erreur}

| Error | Usual cause |
| --- | --- |
| `#VALUE!` | Argument of the wrong type, or columns of two tables mixed |
| `#DIV/0!` | Division by zero |
| `#N/A` | Value not found (XLOOKUP, MATCH, REFERENCE, MUNICIPALITY…) |
| `#NAME?` | Unknown function, table or column |
| `#REF!` | Deleted cell or range |
| `#CIRC!` | Circular reference: the formula depends on itself |
| `#NUM!` | Number out of range (square root of a negative…) |

## Statistics and finance {#statistiques-et-finances}

Statistics read a whole column, a range or an expression on a table's columns; Booleans and blanks are ignored, texts are read as described above: [STDEV.S](fonction:STDEV.S), [VAR](fonction:VAR), [MODE](fonction:MODE), [PERCENTILE](fonction:PERCENTILE), [QUARTILE](fonction:QUARTILE), [CORREL](fonction:CORREL), [SLOPE](fonction:SLOPE)…: `=QUARTILE(Sales[Amount], 3)`.

Financial functions follow Excel's conventions: an amount paid out is negative, and the rate is the rate per period. The monthly payment on a loan of $200,000 over 20 years at 4% per year is written `=PMT(4%/12, 20*12, 200000)` (−1,211.96). See also [PV](fonction:PV), [FV](fonction:FV), [NPER](fonction:NPER), [RATE](fonction:RATE), [IPMT](fonction:IPMT) and [PPMT](fonction:PPMT) for an amortization table. [NPV](fonction:NPV) and [IRR](fonction:IRR) read the cash flows of a column **in the table's display order**: `=IRR(CashFlows[Amount])`.

Probability distributions ([NORM.DIST](fonction:NORM.DIST), [T.DIST](fonction:T.DIST)…), tests ([T.TEST](fonction:T.TEST)), regression ([FORECAST.LINEAR](fonction:FORECAST.LINEAR), [LINEST](fonction:LINEST)), depreciation ([SLN](fonction:SLN), [AMORDEGRC](fonction:AMORDEGRC)) and securities ([PRICE](fonction:PRICE)) are also available. [DSUM](fonction:DSUM) and the other database functions read their criteria from a cell range: `=DSUM(Sales, "Amount", Cell[E1:F3])`. [OFFSET](fonction:OFFSET) and [INDIRECT](fonction:INDIRECT) compute a reference: `=SUM(OFFSET(Cell[B2], 0, 0, Cell[A1]))`.

## Municipality of a GPS point {#commune-d-un-point-gps}

[MUNICIPALITY](fonction:MUNICIPALITY) gives the French municipality (commune) that contains a point: `=MUNICIPALITY([Latitude], [Longitude])`, or its INSEE code (`"code"`), its postal codes (`"cp"`), its department (`"departement"`) or its region (`"region"`). The calculation is done offline, on the municipality boundaries embedded in Lattis (mainland France and overseas), and stays fast on a column of 100,000 rows. A point at sea or outside France gives `#N/A`. As the boundaries are simplified, a point a few dozen meters from a boundary may fall in the neighboring municipality.

*Contains IGN and INSEE data (ADMIN EXPRESS municipality boundaries, 2026 edition, distributed by Etalab) and La Poste postal codes via geo.api.gouv.fr, under the Licence Ouverte / Etalab 2.0.*

## Formula links {#liens-entre-formules}

**Formulas › Show links** traces the links of the selected cells: blue arrows from what their formula reads, red arrows to the formulas that read them. A whole column (`Customers[City]`) is pointed to by its header; a specific row (`Customers[City][1]`, `[Balance][-1]`, the row found by `REFERENCE`) by its cell, and a range of rows (`Sales[Amount][2:5]`) is framed. If that row is hidden (filter, choice tab, hidden row) or can't be found, the arrow points to the column and the status bar says why. Links to another sheet are counted there. Clicking in the grid (or `Esc`) clears the arrows.

## What-if analysis {#analyse-de-scenarios}

**Data › What-if analysis**: **Goal seek** finds the value of a cell that gives a formula the wanted result; **Scenario manager** keeps named sets of values, shows them and summarizes them on a new sheet; **Data table** computes a formula for a list of values (one or two variables, results as fixed values: run it again to recalculate); **Solver** maximizes, minimizes or reaches a value under constraints (exact for a linear problem, approximate otherwise). Nothing changes before **Keep**; `Ctrl+Z` undoes.

## Recalculation {#recalcul}

Formulas recalculate on their own when their data changes. `F9` recalculates everything; `TODAY`, `NOW` and `RAND` only change on opening and with `F9`.

## All functions {#toutes-les-fonctions}
