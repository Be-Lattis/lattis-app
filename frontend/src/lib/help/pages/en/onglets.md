# Documents and web pages

Besides sheets, a workbook can contain two other kinds of tabs: **Documents**, to write a report, and **Web pages**, to build a small application. Both are added by right-clicking the “+” of the tabs at the bottom, or a tab (Insert).

## Document {#document}

An endless rich-text page: headings, lists, quotes, code, alignment, fonts, colors, links and images, formatted from the ribbon as in a word processor.

You can also insert live items into it, fed by the workbook's tables:

- **charts** and **PivotTables** (the same pane as on a sheet);
- **buttons** that run a function of a script;
- **fields** (check box, list, input box…) whose value is written to a cell.

Right-click an inserted item: the same menu as on the sheet; elsewhere, a menu to insert. The text is saved on its own a moment after the last keystroke, as one undo step of the workbook. File › Print / PDF applies to the document.

## Web page {#page-web}

A feature to turn on in File › Settings › Features (see [Settings](parametres)); a workbook that already contains web pages shows and uses them even without it.

A web page written by you (HTML, CSS, JavaScript), shown in an isolated frame: it has access neither to the disk nor to the rest of the application, and its network access goes through Lattis. It talks to the workbook through a small `lattis` object:

```
lattis.sql("SELECT * FROM Orders WHERE Amount > ?", [50])
lattis.query("Top cities")
lattis.on("change", () => refresh())
lattis.call("sheets.list")
lattis.call("cells.set", {row: 1, col: 1, value: 42})
```

`lattis.sql` runs a read SQL query (SELECT or WITH) and `lattis.query` a saved query of the [SQL tab](sql) (its favorites), by name; both return an array of objects, one property per column. `lattis.queries()` lists the saved queries.

The commands of `lattis.call` are those of [Lua scripts](lua). A page only reaches a website through `lattis.call("http.get", …)`, with your approval. A page that tries to open another address by itself (`location.href = …`) is immediately reloaded; the request to that address may still have gone out, though: one more reason to only allow pages from people you trust.

**Without scripts** (web edition whose server does not allow scripts, public demo), a page keeps reading: `lattis.sql`, `lattis.query`, `lattis.on` and the read commands of `lattis.call` (`sheets.list`, `tables.rows`, `cells.get`…). A command that changes the workbook, reaches a website or runs a script is refused, and a banner above the page says so.

A web page received with a workbook (or changed elsewhere) doesn't run right away: the tab asks **Allow** or **Don't run**, with **View code**. A page can read and change the workbook: only allow it if you trust the person who gave you this workbook. The approval applies to this version of the page, on this computer; pages you create or change here are allowed automatically. After “Don't run”, the tab's **Decide…** button asks the question again.

The page editor is next to the preview: `Ctrl+S` saves, `Ctrl+Enter` updates the preview without saving.

## External web pages {#pages-web-externes}

A web page can also show a page from a website (“Website…”), when web pages are turned on.
