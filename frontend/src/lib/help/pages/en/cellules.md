# Formatting, notes and display

The ribbon commands follow Excel's, in the same places. They apply to the selection (or to the active cell) and are undone with `Ctrl+Z`.

## Cell formatting {#mise-en-forme-des-cellules}

Home › Font, Alignment and Number format the selection; the small launcher in the corner of each group, or `Ctrl+1`, opens the **Format Cells** dialog box (number, alignment, font, border, fill). Clicking the alignment already chosen (Left, Center, Right) goes back to the standard alignment, as in Excel.

**Borders ▾** draws an edge (bottom, top, left, right, outside, inside, thick box) with the chosen **line style** (thin, medium, thick, double, dotted, dashed) and **line color**; the Border tab of `Ctrl+1` does the same edge by edge. Format Cells › Number also offers Accounting, Fraction, Scientific and **Custom**, a number format code as in a spreadsheet (`#,##0.00 "kg"`, `0.00E+00`, `# ?/?`, `[Red]-#,##0`, sections `positive;negative;zero;text`, `dd/mm/yyyy hh:mm`); the TEXT function understands the same codes.

## Font and fill colors {#couleurs-de-police-et-de-remplissage}

Home › Font Color and Fill Color (Highlight in a Document): the button applies the last color chosen, the ▾ arrow opens the palette, as in Excel.

- **Theme Colors**: the ten colors of the workbook's theme (Background 1 and Text 1, white and black; Background 2 and Text 2; Accent 1 to 6, including the color of tables and that of external sources), and under each one five lighter or darker variants. A color taken from here **follows the theme**: changing the theme (View › Theme) recolors these cells.
- **Standard Colors**: dark red, red, orange, yellow, light green, green, light blue, blue, dark blue and purple, the same whatever the theme.
- **Recent Colors**: those chosen with “More Colors…” on this computer.
- **No Fill** or **Automatic** removes the color; **More Colors…** lets you pick any color.

Each swatch's tooltip gives its name (“Green, Accent 1, Lighter 40%”). With the keyboard, the arrows move through the swatches and `Enter` chooses. The same palette is used in Format Cells (`Ctrl+1`) and in conditional formatting. When exporting to Excel or OpenDocument, theme colors are written as they are displayed.

Home › Editing › Clear removes the formats, the contents (`Del`) or the note of the selection. Fill copies the first row down (`Ctrl+D`) or the first column to the right (`Ctrl+R`).

**Paste Special** (`Ctrl+Alt+V`, Home › Paste ▾ or right-click) pastes only the values, formulas, formats, values and number formats, or column widths; it can add to, subtract from, multiply or divide the destination, skip empty cells (Skip blanks) and transpose. A copy made in a spreadsheet pastes with its colors, fonts, borders, number formats and merges; a copy from Lattis pasted into a spreadsheet keeps its formatting too.

Home › Format as Table (`Ctrl+L`) makes a table from the block of free cells around the active cell, its first row giving the headers. On an empty cell, Insert › Table (or `Ctrl+L`) places an empty table with one column, “Column 1”, its header on the cell: type below it to fill it, in the header to rename the column.

## Conditional formatting {#mise-en-forme-conditionnelle}

Home › Conditional Formatting colors cells according to their value: highlight rules (greater than, between, text that contains, duplicates…), data bars, color scales and icon sets. The **rules manager** lists the sheet's rules, to edit or delete them. In a table, a rule applies to the whole column, including future rows.

## Data validation {#validation-des-donnees}

Data › Data Validation gives the list of values allowed in the selection, or in a whole table column: the cell shows a drop-down arrow (`Alt+Down` opens it), and any other entry is refused.

The list is typed (**Typed values**, one per line) or taken from a **Range**, as in Excel:

- a table column, for example a lookup table: `Customers[City]` (or some of its rows, `Customers[City][1:5]`);
- cells: `Cell[A1:A10]`, or `Sheet2!Cell[A1:A10]` on another sheet (`A1:A10` typed the Excel way is accepted);
- a named cell.

**Select** hides the dialog while you point to the range (click, drag or `Shift+arrows`; clicking the header of a table column takes the whole column), if needed on another sheet: click its tab, the reference becomes `Sheet2!Cell[A1:A10]`. `Enter` takes it, `Esc` goes back without changing it; in both cases, you return to the original sheet and selection. The dialog's title says what the validation applies to (table column, cells), and the dialog shows a preview of the values.

The list of a range is reread each time the drop-down list opens and on each entry: distinct values, in the range's order, without empty cells. The reference follows renaming of the table, the column or the sheet, and insertion or deletion of rows and columns. If the source disappears (table deleted, rows of the range deleted), the drop-down list and the dialog say so, and entries are refused until the validation is fixed.

### Reference columns: switching from one table to another {#colonnes-de-reference-basculer-d-un-tableau-a-l-autre}

When the range is the **key column** of another table (the one marked 🔑, for example `Species[Name]`), the validated column becomes a **reference** to that table, like a foreign key: the status bar says so.

- **Switch to**: `Ctrl+click` on a cell of the column, the small ↗ button next to its list arrow, or right-click › Switch to “Species” › Magpie, goes to the referenced row (on its sheet), which lights up for a moment. If the value doesn't exist in the target table, the item is grayed out and says so.
- **Go back**: `Alt+Left`, or “← Back to…” in the status bar, takes you back to the starting cell (several jumps are undone one by one).
- **Rows that refer to it**: right-click a cell of the target table's key column › Rows referring to “Magpie” (3) filters the referring table on that value and goes to its first row.

Nothing else is saved: removing the validation (or `Ctrl+Z`) or the target table's key undoes the reference, and it follows renaming of the table and of its key column. Missing values are not blocked by a database constraint: it is the validation that refuses them on entry.

Validations are not exported to Excel (.xlsx), nor read on import.

## Notes {#notes}

Review › New Note (`Shift+F2`) attaches a note to the active cell: a red corner marks it, and it shows on hover. On a cell that has one, the command becomes **Edit Note**. The note's pin keeps it displayed. Review › Delete Note removes it.

## Links {#liens}

Insert › Link (`Ctrl+K`) turns the cell into a link to a web page, an e-mail address or a place in the workbook (sheet and cell, table, named cell). A web or e-mail address you type becomes a link by itself (`Ctrl+Z` removes it). A click on the text follows the link; click and hold selects the cell. The `HYPERLINK` function does the same in a formula.

## Freeze panes {#figer-les-volets}

View › Freeze Panes keeps rows and columns on screen while scrolling:

- **Freeze Panes**: the rows above and the columns to the left of the active cell;
- **Freeze Top Row** or **First Column**;
- **Unfreeze Panes** lets them scroll again.

The setting belongs to the sheet and is saved with the workbook.

## Color theme {#theme-de-couleur}

View › Theme chooses the color of the workbook's tables: that of ordinary tables (green by default) and that of tables loaded by an external source or a query (blue by default). Each theme in the menu shows its two colors; the header, its text, the frame, the icons and the name badge are derived from them, with text that is always readable on the header.

- **Customize…**: two colors of your choice, with an immediate preview in the grid, then the other palette colors (Accent 3 to 6, Text 2, Background 2), derived from the first two if you don't touch them; **Add to my themes** keeps it in the list for your other workbooks (it can be removed from the same window).
- Each theme has its **palette colors** (see Font and fill colors): cells colored with a theme color change with it. A workbook without its own theme keeps the default theme as soon as a theme color is used in it, so that the color stays the same for a colleague.
- The theme is saved with the workbook (`Ctrl+Z` undoes it): a colleague opens it with the same colors, even without that theme. A workbook shared on a Lattis Server publishes it with its layout.
- A workbook that hasn't chosen a theme (new workbook, or one from before themes) takes the **default theme** from File › Settings › Display.

Your organization can offer its own themes, offer only those, or impose the default theme. Theme colors are also exposed as CSS variables (`--lattis-table-head`, `--lattis-source-head`…).

## Protect {#proteger}

Review › Protect Sheet refuses any change to its content; you can still move around, select, copy, search, filter and print. Review › Protect structure locks everything except entry in tables: you type, paste (values only), clear values in their data cells, add rows by typing below, insert or delete rows, sort and filter; free cells, formatting, columns and headers, the tables themselves, objects, names, rules, the sheet's rows and columns, page layout and the tab are refused. Review › Protect Workbook refuses adding, deleting, renaming, moving or hiding sheets; you can choose to also lock the structure of all sheets (entry in tables only), or their content. Refused commands are grayed out, and their tooltip says how to remove the protection (see [Getting started](prise-en-main#proteger-une-feuille-ou-le-classeur)).

In a workbook shared on a Lattis Server, a published table that your rights don't let you change (Viewer role, frozen workbook, offline) likewise grays out the commands that would change its data; its formatting, sorting and filters remain allowed, since they only apply on your computer.

In a collaborative workbook, the owner and the designers change the **structure** of a shared table like that of a local table: add, rename, delete a column, change its type, choose the key, rename or delete the table. The change is made on the server before being applied on the computer, and the others receive it right away (their table is reread; the undo history that touched it is cleared). The server can refuse, and its message shows in the status bar: a column referred to by a role's filter, a restriction or a data item's audience can't be renamed, deleted or converted; a table that feeds the Data Lake or a pipeline can't be deleted. `Ctrl+Z` undoes the change on the server too (if it refuses, the step remains to be undone); a table deleted then restored only comes back on your computer, as a local table. The other members (editors, participants) don't change the structure, you must be online, and server indexes are set with the `tables.add_index` and `tables.drop_index` commands (scripts, assistant).
