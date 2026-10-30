# Live sharing and Data Lake

A workbook shared on a Lattis Server is edited by several people, live; its tables can also feed the Data Lake or be filled in through a form.

## Add a table to a live shared workbook {#ajouter-un-tableau-a-un-classeur-partage-en-direct}

In a collaborative workbook (shared live on a Lattis Server), a table created after publishing by an owner or a designer (typed, pasted, inserted, converted from a range…) is added to the shared workbook automatically, as soon as it is created; offline, on reconnection. A table that stays on your computer (loaded by a query, refused by the server, or created by an editor) carries the amber **on this computer** badge above its frame: the other members don't see it. Clicking the badge, or right-clicking the table › **Add to shared workbook…**, creates it in the server's workbook, with its rows: the other members receive it right away (in the same place on the sheet, or below if the place is taken on their side), with a message in the status bar, and it then syncs like the other shared tables (entries, conflicts, rights). The same action is offered by **Share to Lattis Data Lake…** (“Add this table to the shared workbook”, first) and in **Manage access** › Roles and rights, under the list of tables.

- Reserved for the workbook's owners and designers, and only online: otherwise the dialog says so.
- Column types are fixed as for the other shared tables; calculated columns stay calculated on each computer. Invalid entries must be fixed first.
- A refusal from the server (name already taken by another table of the workbook, for example) shows in the status bar and changes nothing.
- Adding can't be undone with `Ctrl+Z` (the confirmation box reminds you): to remove the table from the shared workbook, **Delete table** deletes it for everyone. Undoing (`Ctrl+Z`) the creation of the table itself also deletes it from the server, for everyone; `Ctrl+Y` recreates it and adds it again.

## Manage access to a shared workbook {#gerer-l-acces-a-un-classeur-partage}

**File › Manage access…** (or the account menu) sets who sees and changes a workbook published on a Lattis Server. Each member has one or more **roles**; their rights are those of their roles combined.

- **Members**: invite an address with its roles (the dialog says what they will allow), remove a role with the ✕ on its chip, give one with **+ Role**. Under each person, a line says what they can do (“View, add, edit own rows”). Roles received through a team are dotted and are changed in **Teams**. The last owner is protected: first give that role to someone else.
- **Roles and rights**: a table shows, for each role, whether it can **View**, **Add rows**, **Edit own rows** (those it added, deletion included), **Edit all rows** and change **Structure and rights** (✓ yes, ◐ partly: some rows or some tables, – no). Choose a role to set its access to the workbook, to each sheet and to each table: *No access*, *View*, *View, add, edit own rows* or *View and edit everything*; “Same as sheet” or “Same as workbook” keeps the access of the level above. Under each table, filters read as sentences (“Only sees rows where Region = “North””, “Only edits rows where Salesperson = the signed-in person”); **Limit…** or **Edit** sets them. The owner has all rights; so does the designer, except over rights.
- **Advanced rules** (owners): a rule removes a right on a table whatever the role's access, and reads as one sentence (“Nobody can delete rows in “Sales”, except owners and designers.”). Start from a preset — **Forbid deletion**, **Add only**, **Edit only rows with status…**, **Edit own rows without creating any** (for a role that adds and edits its rows: it can now only edit) — or choose the table, the roles, the forbidden actions and the possible exception.
- **View as…**: from the line of a member or a role, or in its tab, shows what that member can do on a table and the rows they see, read on the server under their identity (rules included).
- **Discrepancies**: rules changed outside Lattis, directly in PostgreSQL; **Recompile rights** restores those from Lattis.

**Remove from the server…** (owner; at the bottom of this box, or in File) deletes the workbook from the server for everyone: data and permissions, pipelines and forms; its Data Lake shares are cut. Copies already on computers stay: the other members see “This workbook was deleted” and can keep theirs (**Save a local copy…**). Yours stays open as a local workbook. A confirmation asks first; it can't be undone.

## Server unreachable: work offline, or detach {#serveur-injoignable-travailler-hors-ligne-ou-se-detacher}

When the server of a shared workbook no longer responds (server stopped, network down), an orange banner shows under the ribbon, for example “The server localhost:8080 is not responding”. **Details** shows the technical error (also as a tooltip and in the log).

- **Collaborative workbook**: it stays editable. Entries, added or deleted rows, added or renamed columns wait on your computer (small gray triangle in the corner of the cells, “Offline — 3 pending” in the status bar), even if you close Lattis: they are kept with the workbook. When the server comes back, they are sent in the order you made them. A cell changed in the meantime by a colleague becomes a **conflict** (Conflicts pane: keep yours or theirs); whatever the server refuses (rights removed during the outage, column that became untouchable, row deleted by a colleague) is undone on your computer, with a message.
- **Frozen workbook**: it stays viewable, read-only, until the server comes back.

Lattis retries on its own, less and less often (from a few seconds to two minutes); the banner disappears as soon as the server responds. The banner's buttons:

- **Retry** (also File › **Retry connection**): tries right away.
- **Work on a local copy**: saves the workbook under the name of your choice as an ordinary workbook, with no link to the server, then opens it. Changes not yet sent are in the copy (the message says so); they also stay pending in the shared workbook, which is not touched.
- **Detach from server…** (owner of the workbook; also in File, online or offline): this workbook becomes an ordinary local workbook, which no longer syncs. The server's workbook is not changed (the other members keep it, you can reopen it from the server). A workbook opened from the server is only a copy in Lattis's cache: Lattis asks where to save it, then removes the cache. Detaching can't be undone, and a confirmation reminds you.

## Share a table to the Lattis Data Lake {#partager-un-tableau-au-data-lake-lattis}

Right-click a table › **Share to Lattis Data Lake…** makes the table readable from other workbooks on the Lattis Server (Insert › Lattis Data Lake, SQL tab), according to an audience: people, teams, with a row filter for each. If the workbook isn't published, the dialog offers two ways to do it:

- **Publish the workbook with rights management**: the whole workbook is published on the server (File › Publish to a server…), with its roles and filters; the dialog then comes back to share the table.
- **Publish only this table as a data source**: the workbook stays on your computer and isn't made available. The server keeps a copy of the table, fed by a pipeline whose source is your workbook. Only the **checked columns** are sent, with their values (a calculated column sends its results, not its formula); the other tables, the sheets, formulas, scripts and unchecked columns don't leave the computer.

A source is updated when you ask for it: right-click the table › **Update Data Lake source**, or from the same dialog; check “Update the source every time the workbook is saved” to send it on every `Ctrl+S`. Each update sends the whole table, written according to the chosen mode: **Replace all** (the data item becomes the table sent), **Append** (rows are added to those already received) or **Update by key** (a row with the same key is changed, a new one is added). The summary shows in the status bar: rows read, added, changed, deleted, and the values refused by the server, with the first one (row, column, reason). A sent column that you rename, convert or delete in the workbook is also renamed, converted or deleted on the server at the next update. The link with the source is kept in the workbook (save it); the dialog also lets you change the name, the columns sent, the mode, the key and the audience, **suspend** the source (nothing is sent any more, the data item keeps its last rows), and **remove the source** (workbooks that use it keep their last copy). The Publish to a server and Manage access dialogs list the workbook's sources, marked “pipeline”. In Insert › Lattis Data Lake, a data item published alone carries the same mark and says which workbook it comes from; with no data item chosen, the “Your pushed sources” list shows all your sources on the server, whatever workbook they come from.

You need the “data publisher” role on the server, and a server that accepts sources sent by a workbook; otherwise the dialog says so.

## Share a table as a form {#partager-un-tableau-comme-formulaire}

Put the active cell in a table, then **Insert › Form**: a web page lets people fill in this table without opening the workbook, from a phone or a computer. Each submission adds a row, which members see arrive live; the row is written in the form's name (“Form: <title>”), the respondent has no rights on the workbook.

The form is served by the Lattis Server: the table must be published in a collaborative workbook, by an owner or a designer of the workbook, online. Otherwise, the dialog says what is missing and offers the action to do first: **Publish to a server…** (local workbook) or **Add to shared workbook…** (table that is only on your computer), then the designer opens. A frozen workbook, a table loaded by a query, a linked Data Lake data item or a pushed source can't get a form. A member who is neither owner nor designer can copy the link of the forms open to them.

The designer has three tabs, with the page preview on the right (phone or computer width); **Open preview in browser** shows the server's real page, without saving the answers.

- **Fields**: one field per column (“in the form” box, handle to move, label, help, placeholder text, required, default value fixed or today, now, current year, respondent's address). The field type follows the column: text or long text, number (with bounds and unit), date, date and time, check box or Yes / No, option buttons or a searchable list for a column with a validation list. A list taken from a column of a published table is reread by the server; any other list is copied (“List copied on…”, **Update**). The numbered key is set by the server; a typed key is required. A **Position** field fills Latitude and Longitude (or a “lat, lon” column) when the respondent taps “My location”. **Values set by the server** fill a column without a visible field: Received on, Respondent, Form (its title), next number or fixed value; **Add a Received on column** creates the column in the table. With a choice tab, the category is a fixed value, the current year or a field. Subheadings split the form into sections.
- **Appearance**: title, introduction (bold, italics, links, lists), logo and banner, color from a readable palette, one section per page, button label, thank-you message, closing message, data protection notice (with a template), page language (French or English). No photo or file upload.
- **Access**: members of the workbook (viewers included), chosen people and teams, or anyone who has an account on the server; all of them sign in with their Lattis account.

At the bottom, checks show in orange (empty list, default value not in the list, list values visible to respondents…). **Save draft** keeps the form without opening it; **Open form** puts it online and gives its link, its QR code (to save as PNG or SVG) and **Prefilled link…**: one link per value (one QR code per site, for example), which fills in fields on opening, with the option “locked by the link” so they can't be changed. If the table already has forms, the dialog lists them first (Copy link, Edit…) with **New form…**.
