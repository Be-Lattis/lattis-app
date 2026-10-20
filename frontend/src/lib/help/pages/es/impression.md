# Diseño de página e impresión

Archivo › Imprimir / PDF (`Ctrl+P`) imprime la hoja activa. Para un PDF, elija la impresora «Microsoft Print to PDF» (o «Guardar como PDF») en la ventana de impresión. Lo que se imprime se define en el grupo Configurar página de la pestaña **Vista** de la cinta (`Alt+W`). La configuración pertenece a la hoja: se guarda con el libro, se deshace con `Ctrl+Z` y sigue a una copia de la hoja.

## Grupo Configurar página (pestaña Vista) {#groupe-mise-en-page-onglet-affichage}

| Comando | Efecto |
| --- | --- |
| Márgenes | Normal, Ancho o Estrecho; «Márgenes personalizados…» abre el cuadro de diálogo Configurar página |
| Orientación | Vertical u Horizontal |
| Tamaño | Tamaño del papel: A4, A3, A5, B5, Carta, Oficio, Tabloide, Ejecutivo |
| Área de impresión | «Establecer»: solo se imprime la selección. «Borrar»: vuelve a imprimirse todo el rango usado |
| Saltos | «Insertar»: una nueva página empieza encima de la celda activa y a su izquierda (en la fila 1, solo a la izquierda). «Quitar» quita estos saltos. «Restablecer todos» quita todos los saltos manuales |
| Imprimir títulos | Filas que se repiten arriba y columnas que se repiten a la izquierda de cada página |

Insertar o eliminar filas y columnas desplaza con ellas el área, los títulos y los saltos.

## Ajuste de escala {#mise-a-l-echelle}

**Ancho** y **Alto** ajustan la hoja a un número de páginas: Lattis la reduce (hasta el 10 %) para que quepa. «1 página» de ancho y «Automático» de alto pone todas las columnas en el ancho de una página. Mientras se ajusta, los saltos de página manuales se ignoran, como en Excel. **Escala** imprime con un porcentaje fijo y deja de ajustar.

## Opciones de la hoja {#options-de-la-feuille}

«Ver líneas de cuadrícula» y «Ver encabezados» solo cambian la pantalla. «Imprimir líneas de cuadrícula» dibuja todas las celdas. «Imprimir encabezados» agrega las letras de columna y los números de fila. Sin líneas de cuadrícula, solo se enmarcan las celdas de las tablas.

## El cuadro de diálogo Configurar página {#la-boite-mise-en-page}

El pequeño botón abajo a la derecha de cada grupo abre el cuadro de diálogo **Configurar página**. Tiene cuatro pestañas:

- **Página**: orientación, escala o ajuste a páginas, tamaño del papel, número de la primera página.
- **Márgenes**: superior, inferior, izquierdo, derecho, encabezado y pie de página (en cm), centrado horizontal y vertical en la página.
- **Encabezado y pie de página**: un modelo («Página 1 de ?», nombre de la hoja, nombre del archivo…), o tres zonas libres (izquierda, centro, derecha). Los botones insertan códigos: `&[Page]`, `&[Pages]`, `&[Date]`, `&[Heure]` (hora), `&[Fichier]` (archivo), `&[Chemin]` (ruta), `&[Onglet]` (pestaña). Estos códigos son los mismos en todos los idiomas. Escriba `&&` para una sola y comercial.
- **Hoja**: área de impresión (`$A$1:$F$40`), filas que se repiten arriba (`$1:$1`) y columnas que se repiten a la izquierda (`$A:$A`). El botón **Selección** toma la selección de la hoja. Las líneas de cuadrícula, los encabezados, el blanco y negro y el orden de las páginas (hacia abajo y luego hacia la derecha, o al revés) también se definen aquí.

**Imprimir…** aplica la configuración y luego imprime.

## Saltos de página en la cuadrícula {#sauts-de-page-dans-la-grille}

Después de imprimir o de cambiar el diseño de página, unas líneas de puntos muestran dónde se cortan las páginas. Los saltos manuales se muestran con guiones más gruesos, y un área de impresión aparece enmarcada. Vista › **Saltos de página** los muestra u oculta.

## Lo que se imprime {#ce-qui-s-imprime}

Los valores tal como los muestra la cuadrícula, con los anchos de columna y los altos de fila. Los encabezados de las tablas van en negrita, y las celdas combinadas siguen combinadas. Las filas y columnas ocultas no se imprimen. Lattis corta las páginas por sí mismo. Cada página impresa tiene por tanto el tamaño del papel elegido, con sus márgenes, su encabezado y su pie de página.

Límites:

- El formato de las celdas (colores, fuentes, bordes) y los gráficos todavía no se imprimen.
- Por encima de 5.000 filas, el final no se imprime: filtre la hoja o exporte el libro.
- En la ventana de impresión, deje los márgenes en «Predeterminado» y la escala al 100 %. Si no, las páginas preparadas por Lattis se desplazan.
- Si la ventana ofrece la opción «Encabezados y pies de página», déjela sin marcar: agrega los del navegador (fecha, dirección).

## Excel y OpenDocument {#excel-et-opendocument}

El diseño de página acompaña al libro exportado en `.xlsx` u `.ods`: papel, orientación, márgenes, escala o ajuste, área de impresión, títulos, saltos de página, líneas de cuadrícula, encabezados, centrado, encabezado y pie de página. Al importar un `.xlsx`, Lattis recupera el diseño de página de cada hoja.

## Scripts {#scripts}

`sheets.page_setup()` lee el diseño de página de la hoja y `sheets.set_page_setup({...})` lo cambia (consulte [la referencia de Lua](lua:sheets.set_page_setup)).
