# Tablas dinámicas

Una tabla dinámica resume una tabla: totales por cliente, por mes, por región… Lattis la calcula en SQL (`GROUP BY`), lo que sigue siendo rápido con un millón de filas.

## Crear una tabla dinámica {#creer-un-tableau-croise}

1. Haga clic en la tabla de origen.
2. Insertar › Tabla dinámica. La barra de estado indica «Haga clic en la celda donde colocar la tabla dinámica (Esc: cancelar)».
3. Haga clic en la celda donde debe ir la esquina superior izquierda de la tabla dinámica. Mientras tanto no se bloquea nada: puede desplazar la hoja, moverse con el teclado, cambiar de pestaña (la tabla dinámica se coloca entonces en la otra hoja) o usar la cinta. `Esc` (o **Cancelar** en la barra de estado) renuncia.
4. La tabla dinámica se coloca y el panel **Campos de tabla dinámica** se abre a la derecha.

Para colocarla sola en una hoja nueva, como hace Excel de forma predeterminada, haga clic en **Hoja nueva** en la barra de estado en lugar de elegir una celda.

## Mover una tabla dinámica {#deplacer-un-tableau-croise}

- **Clic derecho › Mover** en la tabla dinámica, y luego haga clic en la celda donde debe ir su esquina superior izquierda (filtros de informe incluidos), en la misma hoja o en otra; `Esc` cancela.
- O arrastre el borde de la selección, como para una tabla: una selección tomada en la tabla dinámica (basta una celda) lleva consigo toda la tabla dinámica; con `Ctrl`, se copia.
- O corte toda la tabla dinámica (`Ctrl+X`) y péguela (`Ctrl+V`) en otro lugar.

La tabla dinámica se coloca exactamente en la celda elegida. Las tablas y otras tablas dinámicas que cubre o toca se apartan hacia la derecha, dejando una columna vacía entre ellas, y empujan a su vez a las que encuentran; un clic en otra tabla la empuja por tanto a la derecha de la tabla dinámica. Los gráficos y los botones, que flotan sobre la cuadrícula, no se mueven. Cada colocación o movimiento se deshace de una vez (`Ctrl+Z`).

Las celdas de una tabla dinámica no se pueden escribir una a una: haga doble clic en ellas para cambiar sus campos.

## El panel de campos {#le-volet-des-champs}

La lista de arriba muestra las columnas de la tabla de origen, con el icono de su tipo; la búsqueda filtra la lista. Active un campo para agregarlo, o arrástrelo a una de las cuatro áreas:

| Área | Función |
| --- | --- |
| Filtros | Filtros de informe, encima de la tabla dinámica |
| Columnas | Una columna de la tabla dinámica por cada valor del campo |
| Filas | Una fila de la tabla dinámica por cada valor del campo (varios campos se anidan) |
| Valores | Los números calculados |

Cada etiqueta de campo tiene su menú ▾: mover, quitar, filtrar y, para un valor, el resumen y la presentación. Con el teclado: `Tab` pasa de un elemento al siguiente, `Entrar` abre el menú de una etiqueta, `Supr` la quita, `Alt+Arriba` / `Alt+Abajo` la mueven.

Una columna calculada todavía no puede usarse como campo.

### Campo calculado (agrupar por una fórmula) {#champ-calcule-regrouper-par-une-formule}

**Campo calculado…**, bajo la lista de campos, agrega un campo cuyo valor se calcula para cada fila de la tabla de origen con una fórmula sobre sus columnas, escritas entre corchetes: `=AÑO([Fecha])` agrupa por año, `=TEXTO([Fecha];"mmmm")` por mes, `=IZQUIERDA([Código];2)` por los dos primeros caracteres de un código, `=SI([Importe]>1000;"Grande";"Pequeño")` por tramo. El cuadro de diálogo sugiere funciones y columnas mientras escribe, y muestra los primeros valores que da la fórmula (o su error). El campo se une a la lista con el icono ƒ y se coloca como una columna: en Filas o Columnas, agrupa las filas por su resultado; en Filtros, filtra; en Valores, se resume. Su menú ▾ en la lista lo edita o lo elimina (entonces sale de las áreas). La tabla dinámica lo recalcula en cada actualización; cambiar el nombre de una columna del origen actualiza la fórmula. `Ctrl+Z` lo deshace. Scripts: `calc = {{name = "Año", text = "=AÑO([Fecha])"}}` en la configuración de `pivots.create` o `pivots.set`.

## Resumen y presentación de los valores {#synthese-et-affichage-des-valeurs}

Resúmenes: Suma, Cuenta, Recuento distinto, Promedio, Mín, Máx. Una columna de números se suma por defecto, las demás se cuentan. **Mostrar valores como** (Configuración de campo de valor) también calcula: % del total general, de la columna, de la fila, de la fila o columna principal, del total principal; % de, diferencia y % de diferencia respecto de un elemento (o el anterior, el siguiente) de un campo base; total acumulado y % acumulado; rango; índice. Un formato de número (moneda, porcentaje, decimales) se elige por valor. Una columna calculada de la tabla de origen sirve de campo como las demás.

## Columnas, grupos y segmentaciones {#colonnes-groupes-et-segments}

Varios campos en **Columnas** dan encabezados anidados, con un subtotal por grupo y una casilla −/+ para contraer un grupo. **Agrupar…** (menú ▾ de un campo de fecha o número en el panel) ordena una fecha por años, trimestres, meses o días (varios niveles a la vez: Años y Trimestres pasan a ser campos delante de la fecha) y un número por intervalos (comienzo, final, paso); **Desagrupar** vuelve a los valores. **Insertar segmentación** coloca en la hoja un panel de botones que filtra la tabla dinámica: clic para un valor, Ctrl+clic para añadir más, el embudo para volver a todo; clic derecho para filtrar también otras tablas dinámicas del mismo origen. **Insertar escala de tiempo** hace lo mismo con los períodos de una fecha (Mayús+clic: un rango).

## Ordenar y filtrar las etiquetas {#trier-et-filtrer-les-etiquettes}

El ▾ de una etiqueta de fila (o el menú ▾ de una etiqueta de campo en las áreas Filas y Columnas del panel) ordena el campo: de A a Z, de Z a A, en el orden del origen, o con **Orden personalizado…**, que dispone los valores del campo en el orden que usted quiera o según una lista (Días, Meses, listas del libro), como para una [columna de tabla](prise-en-main). El ▾ de una etiqueta también filtra sus valores activándolos. Sin orden elegido, las etiquetas siguen el orden de la tabla de origen. El orden elegido se conserva en la tabla dinámica: cuando los datos cambian, un valor de la lista que aparece ocupa su lugar, los demás siguen en orden ascendente. Como en Excel, las celdas vacías del campo forman siempre el último elemento, sea cual sea el orden: la última fila (o columna) antes del total, y «(en blanco)» al final de la lista en el menú ▾.

## Actualización {#mise-a-jour}

La tabla dinámica sigue los cambios del panel tras un breve retraso, en un único paso de deshacer. También sigue los datos de la tabla de origen. El panel vuelve a abrirse con un clic en la tabla dinámica.

## Agregar columnas a la derecha {#ajouter-des-colonnes-a-droite}

Escriba un nombre en la celda justo a la derecha del encabezado de la tabla dinámica: esta recibe una **columna de entrada**, dentro de su marco, con encabezado verde (el resto de la tabla dinámica conserva su azul de datos calculados). Sus celdas se editan como celdas normales:

- **Un valor** se escribe fila a fila (una nota, un objetivo, un comentario). Sigue a su etiqueta de fila: cuando la tabla dinámica cambia (datos de la tabla de origen, orden, filtro, campos), se queda junto a la misma etiqueta. Una etiqueta que desaparece **aparta** sus valores; vuelven con ella. El panel de campos indica cuántas filas están apartadas, y **Olvidar los valores apartados** los borra (Ctrl+Z para deshacer). Quitar todos los campos de Filas aparta todos los valores; vuelven cuando vuelve el campo. Lo mismo al cambiar el campo de Filas: los valores se guardan por campo y por etiqueta.
- **Una fórmula** (que empieza por =) se aplica a toda la columna, como una columna calculada: `=[Cuenta]*2` lee el valor de la fila en la columna «Cuenta» de la tabla dinámica (o en otra columna de entrada), `=[Cuenta]/SUMA(TCD1[Cuenta])` compara con toda la columna (solo filas de detalle), y una dirección como `=B3` o `=A20*[Cuenta]` lee una celda de la hoja. Una fórmula también se calcula en las filas de total. Un valor escrito en una columna de fórmula se rechaza; borrar una de sus celdas borra la fórmula (los valores escritos antes vuelven).
- Las filas de **total** y subtotal no admiten valores.
- Cambiar el nombre: vuelva a escribir el encabezado. Quitar la columna: borre su encabezado (Supr en toda la columna, o encabezado borrado), sus valores se van con ella (Ctrl+Z para deshacer).

Las columnas de entrada se quedan en el borde derecho de la tabla dinámica cuando esta se ensancha (un valor o un campo más en columnas), y empujan lo que tienen al lado, como la propia tabla dinámica. Se mueven con ella; una copia de la tabla dinámica copia también sus valores. Una fórmula en otro lugar las lee por su dirección (`=E5`), y la vista SQL de la tabla dinámica las contiene.

## Usar una tabla dinámica en otro lugar {#utiliser-un-tableau-croise-ailleurs}

Una tabla dinámica tiene un nombre (mostrado en la parte superior del panel): puede consultarse en la [pestaña SQL](sql) como una tabla, y sirve de origen para [gráficos](graphiques). La función [AGRUPARPOR](fonction:AGRUPARPOR) da una agrupación simple directamente en una fórmula.

**En una fórmula**, una tabla dinámica se lee como una tabla con nombre, por su nombre (la etiqueta azul arriba a la izquierda de su marco): `=SUMA(TCD1[Cuenta])` lee la columna entera (solo las filas de detalle, sin grupos, subtotales ni total), `=TCD1[NID][2]` su segunda fila de detalle, y sus columnas de entrada se leen igual (`TCD1[test]`). Al escribir una fórmula, un clic en la tabla dinámica escribe estas referencias en lugar de una dirección: `[Cuenta]` en la fila de una fórmula escrita en una columna de entrada de la tabla dinámica, `TCD1[Cuenta][2]` en otro lugar, `TCD1[Cuenta]` en un encabezado o una fila de total. En una columna de entrada, `[Cuenta]` (o `[@Cuenta]`) también se calcula en las filas de grupo, subtotal y total, con sus propios valores; los campos de fila más finos que la fila quedan vacíos.
