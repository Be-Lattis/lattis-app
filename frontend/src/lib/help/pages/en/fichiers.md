# Files and recovery

A Lattis workbook is a single `.lattis` file: a ZIP archive that contains a compressed SQLite database and a short manifest. You can email it and archive it like an Excel workbook; once unzipped with 7-Zip or WinRAR (Windows File Explorer can't read its compression, Zstandard), any SQLite tool can read its tables.

## Save {#enregistrer}

`Ctrl+S` saves, `F12` or `Ctrl+Shift+S` saves under another name. Saving is safe: the file is first written next to the old one, read back, then put in its place; an abrupt shutdown during the save leaves the old file intact.

- An unchanged workbook is not rewritten.
- On a large workbook, the status bar shows the progress.
- If the disk is too full, Lattis says so before writing anything (“… needed, … left”).
- The undo history and deleted columns and rows never go into the file.

Closing the workbook or the window with unsaved changes first asks whether to save them.

## Open an .xlsx file {#ouvrir-un-fichier-xlsx}

File › Open (or “Open with”, or a drag and drop onto the window) also opens an `.xlsx` file: the title bar shows “name.xlsx”. On the first `Ctrl+S`, Lattis asks whether to keep the `.xlsx` format or convert to a `.lattis` workbook, and says what `.xlsx` does not keep (scripts, pivot tables, formulas without an equivalent…). “Don't ask again” remembers the choice (File › Settings › Files). Formulas, formats, styles, merges, notes, links, validations, conditional formatting, charts and images are written back to the file; a plain range stays a range. Recovery after an incident also works for an `.xlsx`.

## Version history {#historique-des-versions}

Each save keeps a version of the workbook next to the file (hidden `.lattis-versions` folder): one per 10 minutes, 20 at most, and 10 times the workbook size in total. File › Version history lists them (date, author, size); **Save a version** keeps one under a name, which stays. **Open** shows a version read-only; **Restore** replaces the workbook with it, the previous version staying in the list. A workbook shared on a Lattis Server has no history.

## Recovery after an incident {#recuperation-apres-un-incident}

While you work, Lattis keeps a working copy of the workbook on your computer. If Lattis stops without having saved (crash, forced shutdown, power cut), the next start offers to recover the changes: recovering reopens the workbook in the state it was in, with its undo history; discarding goes back to the saved file.

Your entries and commands are written to disk as soon as they are made: they even survive a power cut.

## Workbook shared on the network {#classeur-partage-sur-le-reseau}

A workbook opened from a shared folder is signaled to other computers by a small hidden `~$name.lattis` file. Another computer that opens it is warned and can open it read-only. If the file was changed elsewhere since it was opened, `Ctrl+S` offers to save a copy, overwrite or cancel.

To work together live on a Lattis Server, see [Live sharing and Data Lake](partage).

## Import {#importer}

File › Import reads a CSV file (or delimited text), `.xlsx`, `.xlsm` and OpenDocument `.ods` workbooks: each sheet becomes a tab, each block of data a table, with its formatting, notes, validation lists and conditional formatting. Charts and PivotTables are carried over when their data is a table of the workbook. Whatever isn't carried over (macros, other objects) is reported at the end of the import.

For data to refresh (JSON or Parquet file, SQLite database, PostgreSQL, MySQL, SQL Server, web page), use a query instead: see [Import data and connections](connexions).

## Export {#exporter}

- **CSV**: the active table or the whole sheet. In a CSV for Excel, a text that starts with `=`, `+`, `-` or `@` is preceded by an apostrophe, so that Excel doesn't run it as a formula.
- **Excel (`.xlsx`) and OpenDocument (`.ods`)**: the whole workbook. Tables become Excel tables, formulas are translated when the format has an equivalent, otherwise exported as their value. PivotTables go out as values; charts, conditional formatting, validation lists and notes go out in `.xlsx`; scripts are not exported. A report says what didn't go out as is.
- **SQL (`.sql`)**: a `CREATE TABLE` then `INSERT` script of the tables and PivotTables (as values), for the whole workbook, the active sheet or the active table. The Standard dialect suits most databases; PostgreSQL, MySQL, SQL Server and SQLite adjust names, types and booleans to their database. The key column becomes the primary key; a calculated column goes out as values, its formula in a comment; free cells are not exported.
- **PDF**: File › Print, with the “Microsoft Print to PDF” printer (see [Page layout and printing](impression)).

## Sample workbook {#classeur-d-exemple}

The start page offers to open a sample workbook: sales, customers, PivotTables, charts, a map, SQL queries and scripts, with a “Discover” tab that presents them. It is never saved on its own: File › Save As to keep it.
