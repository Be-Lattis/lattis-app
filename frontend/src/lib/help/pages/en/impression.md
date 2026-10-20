# Page layout and printing

File › Print / PDF (`Ctrl+P`) prints the active sheet. For a PDF, choose the “Microsoft Print to PDF” printer (or “Save as PDF”) in the print window. What gets printed is set in the ribbon's **Page Layout** tab (`Alt+P`), as in Excel. The settings belong to the sheet: they are saved with the workbook, undone with `Ctrl+Z` and follow a copy of the sheet.

## Page Setup group (View tab) {#groupe-mise-en-page-onglet-affichage}

| Command | Effect |
| --- | --- |
| Margins | Normal, Wide or Narrow; “Custom Margins…” opens the Page Setup dialog box |
| Orientation | Portrait or Landscape |
| Size | Paper size: A4, A3, A5, B5, US Letter, US Legal, Tabloid, Executive |
| Print Area | “Set”: only the selection is printed. “Clear”: the whole used range is printed again |
| Breaks | “Insert”: a new page starts above the active cell and to its left (on row 1, only to the left). “Remove” removes these breaks. “Reset All” removes all manual breaks |
| Print Titles | Rows to repeat at the top and columns to repeat on the left of each page |

Inserting or deleting rows and columns moves the area, the titles and the breaks along with them.

## Scaling {#mise-a-l-echelle}

**Width** and **Height** fit the sheet to a number of pages: Lattis shrinks it (down to 10%) so that it fits. “1 page” wide and “Automatic” tall gives all the columns across the width of one page. While fitting, manual page breaks are ignored, as in Excel. **Scale** prints at a fixed percentage and stops fitting.

## Sheet options {#options-de-la-feuille}

“View gridlines” and “View headings” only change the screen. “Print gridlines” draws all the cells. “Print headings” adds the column letters and row numbers. Without gridlines, only the cells of tables are framed.

## The Page Setup dialog box {#la-boite-mise-en-page}

The small button at the bottom right of each group opens the **Page Setup** dialog box. It has four tabs:

- **Page**: orientation, scale or fit to pages, paper size, first page number.
- **Margins**: top, bottom, left, right, header and footer (in cm), horizontal and vertical centering on the page.
- **Header/Footer**: a preset (“Page 1 of ?”, sheet name, file name…), or three free areas (left, center, right). The buttons insert codes: `&[Page]`, `&[Pages]`, `&[Date]`, `&[Heure]` (time), `&[Fichier]` (file), `&[Chemin]` (path), `&[Onglet]` (tab). These codes are the same in every language. Write `&&` for a single ampersand.
- **Sheet**: print area (`$A$1:$F$40`), rows to repeat at the top (`$1:$1`) and columns to repeat on the left (`$A:$A`). The **Selection** button takes the sheet's selection. Gridlines, headings, black and white and the page order (down then over, or the reverse) are also set here.

**Print…** applies the settings then prints.

## Page breaks in the grid {#sauts-de-page-dans-la-grille}

After printing or changing the page layout, dotted lines show where the pages are cut. Manual breaks are shown with bolder dashes, and a print area is framed. View › **Page Breaks** shows or hides them.

## What gets printed {#ce-qui-s-imprime}

The values as the grid displays them, with the column widths and row heights. Table headers are in bold, and merged cells stay merged. Hidden rows and columns are not printed. Lattis cuts the pages itself. Each printed page therefore has the size of the chosen paper, with its margins, its header and its footer.

Limits:

- Cell formatting (colors, fonts, borders) and charts are not printed yet.
- Beyond 5,000 rows, the end is not printed: filter the sheet or export the workbook.
- In the print window, keep the margins on “Default” and the scale at 100%. Otherwise the pages prepared by Lattis are shifted.
- If the window offers the “Headers and footers” option, leave it unchecked: it adds the browser's own (date, address).

## Excel and OpenDocument {#excel-et-opendocument}

The page layout goes with the workbook exported as `.xlsx` or `.ods`: paper, orientation, margins, scale or fit, print area, titles, page breaks, gridlines, headings, centering, header and footer. When importing an `.xlsx`, Lattis takes over the page layout of each sheet.

## Scripts {#scripts}

`sheets.page_setup()` reads the sheet's page layout and `sheets.set_page_setup({...})` changes it (see [the Lua reference](lua:sheets.set_page_setup)).
