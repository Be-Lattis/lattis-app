# Import data and connections

Data › Get Data imports data into a table, in the manner of Power Query: each transformation becomes a step that can be replayed, and the table is refreshed on demand.

## Sources {#sources}

| Source | Details |
| --- | --- |
| Text or CSV file | Delimiter of your choice |
| Excel workbook | One sheet of your choice |
| JSON file, Parquet file | |
| SQLite database (file) | A table or a query |
| PostgreSQL, MySQL / MariaDB, SQL Server | Through a connection (see below) |
| Web (JSON API) | An address that returns JSON |
| Workbook table, Other query, Workbook SQL query | To transform what is already there |

## The query editor {#l-editeur-de-requete}

The editor shows a preview of the data and the list of **applied steps**. The commands of its ribbon add a step:

- **Columns**: choose, remove, rename, change the type;
- **Rows**: filter, sort (with, if needed, a custom order: the listed values first, in that order), remove duplicates, keep the top rows;
- **Transform**: split, replace values, format (TRIM, UPPERCASE…), custom column, index column, group by, pivot, unpivot;
- **Combine**: merge (join) or append the rows of another query or of a table.

Clicking a column of the preview selects it; `Ctrl+Z` removes the last step. The result is loaded into a table, or stays a simple connection.

## Queries & Connections {#requetes-et-connexions}

The **Queries & Connections** pane (Data › Queries & Connections) lists the workbook's queries: edit, refresh, delete. `F5` in a table loaded by a query runs it again; `Ctrl+F5` or Data › Refresh All runs all the queries again then recalculates the formulas. A name typed to the right of its header adds a typed column, kept on refresh (see [Write next to a linked result](sql)).

## Received workbook: approving sources {#classeur-recu-accord-des-sources}

A query is written in the workbook. So that a received workbook can't, on refresh, reach an unknown server or copy a file from your computer, Lattis asks for your approval the first time a query wants to read a file or reach a host (web API, database server). The approval applies to the session, or to this workbook on this computer with “Remember”. The sources of a query written in the editor, and the files you choose there, are approved automatically. Approved hosts can be reviewed in the Script tab › Network access.

## Database connections {#connexions-aux-bases}

A **connection** describes a server (PostgreSQL, MySQL / MariaDB, SQL Server, or a web API): host, port, database, user. It is stored in the workbook and serves several queries.

Passwords are never saved in the workbook: they stay in the system keychain, and Lattis asks for them on the first refresh. The pane also lets you forget them.

In a company, connections can be provided by the computer's configuration; they appear without having to enter anything.
