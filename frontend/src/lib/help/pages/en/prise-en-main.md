# Getting started

Lattis works like a spreadsheet: an endless grid, a ribbon at the top, sheet tabs at the bottom. The difference is underneath: every table you type becomes a real database table, typed and queryable in SQL.

## Guided tour {#visite-guidee}

In three minutes, the guided tour shows the essentials on a small new workbook: a typed table, a calculated column, two tables linked by REFERENCE, a PivotTable with its chart, then a SQL query. Do each action yourself, or click Next to let Lattis do it; Skip (or `Esc`) stops it. It is offered at first launch and can be restarted from the help (? › Guided tour).

## Detected tables {#tableaux-detectes}

Type a header row then a few rows of values, anywhere: Lattis turns them into a **table**, with a type per column and a name (the tab's name for the first one, changed to the left of the formula bar). A row or column typed right next to it extends it; a pasted block also becomes a table. If the first row holds numbers or dates, it is data: the header “Column 1”, “Column 2”… goes above, ready to be renamed. `Ctrl+L` formats a range as a table.

Whatever is not in a table stays a **free cell**: a title, a parameter, a total. Everything can be undone with `Ctrl+Z`, including the creation of a table. Detection rules, headers typed afterwards, two-row header groups, right-click menu: see [Tables](tableaux).

## Column types {#types-de-colonnes}

Each column has a type, guessed from what you type: Text, Integer, Decimal, Date (with or without time), Boolean, or Calculated (one formula for the whole column). The type is changed in the column header menu; see [Column types](tableaux#types-de-colonnes).

## Sort and filter {#trier-et-filtrer}

The ▾ of a header sorts the table (A to Z, Z to A, custom order) and filters the column by values or by condition. Sorting changes the order of the rows for good; blanks go last. See [Sort and filter](tableaux#trier-et-filtrer).

## Keys {#cles}

A **key** is a column whose values are unique and all filled in (a customer code): it identifies each row, is shown in the margin and is used for [REFERENCE](fonction:REFERENCE) lookups. A first column named ID or holding 1, 2, 3… becomes the key automatically, and an added row gets the next number. See [Keys](tableaux#cles).

## Formulas {#formules}

A formula starts with `=` and refers to columns by their name. Typed in a table column, it applies to the whole column:

```
=[Quantity]*[Unit price]
```

See [Formulas](formules) for references and the list of functions. While typing, `F1` opens the page of the function under the cursor.

## Combine tabs {#regrouper-des-onglets}

One tab per month, per year or per city, with the same columns? Data › Combine tabs… brings them together in a single table, with a category column, and updates the formulas that cited them. See [Combine tabs](regroupement).

## Choice tab (one year, one month, one city at a time) {#onglet-a-choix-une-annee-un-mois-une-ville-a-la-fois}

Right-click a tab › **Choose by** › a column (Year): the tab shows one value at a time, picked from its ▾ list, like one tab per year, with **New year…** to add one. See [Choice tab](regroupement#onglet-a-choix-une-annee-un-mois-une-ville-a-la-fois).

## Save {#enregistrer}

A workbook is a single `.lattis` file. `Ctrl+S` saves; after a crash, the changes are offered for recovery at the next start. `Ctrl+Z` works until the workbook is closed, even after `Ctrl+S`, but the history is not kept in the file. File › Import and Export read and write CSV, `.xlsx` and `.ods`. See [Files and recovery](fichiers) and [Page layout and printing](impression).

## Protect a sheet or the workbook {#proteger-une-feuille-ou-le-classeur}

Review › Protect Sheet, Protect structure (entry in tables only) and Protect Workbook prevent **accidental changes**, with an optional password (see [Protect](cellules#proteger)). This is not security: the file is not encrypted, and someone with the right tools can read its data or remove the protection without the password. Don't rely on it for confidential data.

## Next steps {#et-ensuite}

- [PivotTables](tcd) and [charts](graphiques) to analyze.
- [The SQL tab](sql) to query tables.
- [Lua scripts](lua) to automate.
- [Import data](connexions) from files, databases or the web.
