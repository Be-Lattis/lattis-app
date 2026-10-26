# Charts

A chart is connected to a table or a PivotTable, and updates with it.

## Insert a chart {#inserer-un-graphique}

1. Click in the table (or the PivotTable) to chart.
2. Insert › choose a chart type.
3. The chart is placed on the sheet and the **Chart** pane opens on the right.

If several columns of the table are selected, they set up the chart, as in Excel: the first text column gives the categories, the number columns the series.

The chart moves with the mouse and is resized by its handles. Clicking it reopens the pane; right-click: cut, copy, copy as picture (to paste in an email or a document), delete. With the keyboard, once the chart is selected: the arrows move it, Shift+arrows resize it, Tab goes to the next chart, Del deletes it.

## Plotted data {#donnees-tracees}

As in Excel, a chart only shows the rows displayed by the table's filter; the pane's **Rows hidden by the filter** box takes them all back.

A chart plots at most 2,000 points: beyond that, it says so under the drawing (“First 2,000 points of 50,000”). A line that is too long is reduced rather than cut: each interval keeps its first and last points, its minimum and its maximum. Box & whisker charts and histograms are always calculated on all rows.

Values take the format of their column ($, %, decimals, dates) on the axes, labels and tooltips. A date column on X gives a time axis for line and area charts, and can also be read in a scatter chart.

## Types {#types}

Column, bar, line, area, pie, doughnut, scatter, combo (columns and lines, secondary axis possible), 3-D column and 3-D pie, waterfall, treemap, radar, funnel, box & whisker, histogram, gauge, and **map** (points or density on an OpenStreetMap map, from latitude and longitude columns).

## The Chart pane {#le-volet-graphique}

- **Type**: changes the shape without losing the chosen data; “Stacked” for columns and areas.
- **Data**: the source (a table or a PivotTable), the category column (horizontal axis) and one or more value series, in the order chosen with the ▲ ▼ arrows; for lines, **empty cells** break the line (gap), count as zero or are connected.
- **Colors**: that of each series, or of each category for pie, doughnut, funnel and treemap charts, and increase, decrease and total for the waterfall.
- **Titles**: chart and axis titles.
- **Elements**: legend, data labels; secondary axis for a series of a combo chart.

## Maps {#cartes}

A map reads either two columns (latitude, longitude), or a single column that contains both. Base maps come from OpenStreetMap: they need a connection, and access to the tile server requires your approval. This approval does not depend on scripts (it also applies in the web edition); after a refusal, the map's **Show the base map** button gives it. The public demo has no base map: it connects to nothing, and the map says so.

To zoom, select the map (one click): the mouse wheel zooms around the pointer, dragging moves the map, a double-click zooms in (Shift+double-click zooms out), and the **+** and **−** keys zoom too. As long as the map isn't selected, the mouse wheel scrolls the sheet or the document; Alt+drag moves the selected chart. The **+**, **−** and **recenter** buttons appear at the top right on hover. The chosen view is saved with the workbook (Ctrl+Z undoes it); **recenter**, here or in the chart pane, goes back to the view fitted to the points.

**Color by** (chart pane, points display): choose a column, and each distinct value gets its own color, spread across the whole spectrum; a legend at the bottom left names them (**Color legend** box to hide it). Colors follow the sorted order of the values, not that of the rows: sorting or filtering the table doesn't mix them up. Beyond 20 values, the 19 most frequent keep a color and the others are gray (“Others”); empty cells are light gray. Density stays a single layer: the option isn't offered there.

Hovering over a point shows its name, its color and its value. Once the map is selected, clicking a point (without dragging) selects its row in the table, on its sheet, even from a document; if several points overlap, it's the one closest to the pointer. The first click on a map that isn't selected only selects it.

## Copy a chart {#copier-un-graphique}

A copied chart can be pasted on another sheet or in a [document](onglets), where it stays connected to its data.

## Pictures, shapes and text boxes {#images-formes-et-zones-de-texte}

Insert › **Picture** (or Ctrl+V of a copied image), **Shapes** or **Text Box**: the object is placed at the active cell and moves with its rows and columns. Drag it to move it, pull its handles to resize it (a picture keeps its proportions; Shift frees them), double-click a shape to type text in it. The bar above the object sets the colors, the line, the transparency and the text; Delete removes it, Ctrl+D duplicates it. A very large picture is reduced when inserted; 10 MB at most (1 MB in a shared workbook).

## Trendlines and error bars {#tendance-et-erreurs}

Chart pane › **Analysis** (bars, lines, areas, scatter, bubbles): pick a series, then a **trendline** (linear, exponential, logarithmic, polynomial, power, moving average), with its forecast before or after the data, its equation and its R². **Error bars**: fixed value, percentage, standard deviation, standard error, or amounts from a column. The **Bubble** (X, Y, size) and **Stock** (open, high, low, close, volume) types are in More charts.

## Sparklines {#sparklines}

Insert › **Sparklines**: a small chart in a cell (line, column, win/loss). Select the columns of a table row (Jan to Dec): every row of the table gets its own, new rows included. Outside a table, give the data range and the location. The same menu, on a cell that has one, sets the highlighted points, the colors and the axis, or deletes it. Creating them is turned on in File › Settings › Features; those in a workbook are always shown.
