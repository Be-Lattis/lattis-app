# Pestañas similares y pestaña de elección

Reunir pestañas con las mismas columnas (una por mes, año o ciudad) en una sola tabla y mostrar después un valor a la vez.

## Combinar pestañas {#regrouper-des-onglets}

Muchos libros tienen **una pestaña por mes, por año o por ciudad**, con las mismas columnas. Lattis las detecta: sus pestañas se subrayan con la misma línea de color, y una etiqueta «⧉ 12 pestañas similares» sigue a la serie (al pasar el puntero se resaltan las pestañas afectadas). Después de importar un archivo .xlsx, al abrir, o cuando copia una pestaña de la serie por tercera vez (incluso antes de cambiar el nombre de la copia), la barra de estado lo ofrece una vez: **Combinar…**, **Más tarde** (la etiqueta se queda) o **No volver a sugerir** (se conserva en el archivo: a un compañero que lo recibe no se le vuelve a preguntar; también con clic derecho en la etiqueta).

Las pestañas van juntas cuando al menos dos tercios de sus columnas se encuentran de una pestaña a otra: una columna agregada durante el año se queda vacía donde faltaba, y una columna con otro nombre («Importe €») se empareja con su vecina («Importe»), cosa que el panel muestra siempre. Se reconocen los nombres de pestañas en serie: meses, años, trimestres, semanas, fechas, secuencias de números o de letras («Lote 1», «Lote 2»; «Anexo A», «Anexo B»), municipios.

**Datos › Combinar pestañas…** (o clic derecho en una selección de pestañas) abre el panel:

1. **Pestañas**: las tablas que combinar, que hay que activar, en el orden de las pestañas;
2. **Columna de categoría**: su nombre, rellenado a partir de los nombres de las pestañas (Mes, Año, Trimestre, Ciudad, la parte común como «Lote», si no Categoría) y siempre editable, su tipo, y el valor tomado de cada pestaña, que se puede corregir si hace falta. Va en primer lugar en la tabla, con una lista desplegable de sus valores;
3. **Columnas**: la correspondencia (✓ presente, ∅ ausente, «Importe € → Importe» emparejada) y el tipo final; se señala una columna que pasa a texto;
4. **Después**: el nombre de la tabla y de la hoja nueva, y lo que pasa con las pestañas antiguas: **eliminadas** (de forma predeterminada) o **conservadas ocultas**, con sus tablas renombradas `Enero_ancien`;
5. **Vista previa**: las primeras filas y el número total de filas, y luego el resumen de las fórmulas que citaban las pestañas antiguas: «14 fórmulas trasladadas, 2 que corregir a mano», con la lista (hoja, celda, fórmula antes → después).

**Combinar** lo hace todo en un único paso: `Ctrl+Z` devuelve las pestañas de origen. El formato y las entradas con error siguen a sus filas. La **clave** se conserva si ningún valor se repite de una pestaña a otra; si no, la tabla combinada no tiene clave (los valores se quedan), y Lattis lo indica con un ejemplo: «Sin clave: F-001 está en enero y en febrero».

Lo que citaba las tablas antiguas se **traslada** a la tabla combinada, restringido al mes (el año, la ciudad) de la pestaña antigua, en el mismo paso de deshacer y con los mismos valores. Una hoja «Resumen» que hacía `SUMA(Enero[Importe])`, `SUMA(Febrero[Importe])`… se traslada así:

| Antes | Después |
|---|---|
| `SUMA(Enero[Importe])` | `SUMAR.SI.CONJUNTO(Ventas[Importe]; Ventas[Mes]; "=Enero")` |
| `PROMEDIO`, `MAX`, `MIN` de una columna (o de un cálculo sobre las columnas, `SUMA(Enero[Precio]*Enero[Cant])`) | `PROMEDIO.SI.CONJUNTO`, `MAX.SI.CONJUNTO`, `MIN.SI.CONJUNTO`, `SUMAR.SI.CONJUNTO` |
| `CONTARA(Enero[Cliente])` | `CONTAR.SI.CONJUNTO(Ventas[Cliente]; "<>"; Ventas[Mes]; "=Enero")` |
| `SUMAR.SI`, `CONTAR.SI`, `SUMAR.SI.CONJUNTO`… | lo mismo, en `…SI.CONJUNTO`, con el criterio del mes agregado |
| `FILAS(Enero)` | `CONTAR.SI.CONJUNTO(Ventas[Mes]; "=Enero")` |
| `CONTAR`, `MEDIANA`, varias columnas en `MAX` o `PROMEDIO`, `UNICOS(Enero[Cliente])`… | la columna filtrada: `FILTRAR(Ventas[X]; Ventas[Mes]="Enero")` |
| una tabla dinámica sobre Enero | la misma sobre Ventas, con un filtro de informe Mes = Enero |
| un gráfico sobre Enero | el mismo sobre Ventas, con un filtro Mes = Enero (mostrado en su panel, se puede quitar) |

Las columnas calculadas de otras tablas y las reglas de formato condicional siguen las mismas reglas. Quedan **por corregir a mano**, con el motivo: una fila concreta (`Enero[Importe][3]`), `REFERENCIA`, `BUSCARV` y `EXISTE` (buscan una clave en una tabla con nombre), `SUBTOTALES`, la tabla entera (`FILTRAR(Enero; …)`), una lista de control tomada de una tabla antigua, una consulta; el texto SQL y los scripts Lua no se detectan. En ese caso, las pestañas antiguas se conservan ocultas de forma predeterminada para que estas fórmulas sigan funcionando; si no, se eliminan. Sin columna de categoría, ya nada distingue las pestañas: todo queda por corregir. Después, un resumen por mes queda mejor como tabla dinámica Mes × Importe, que sigue sola las filas nuevas.

No se permite combinar en una hoja protegida, en una tabla filtrada (quite antes el filtro), en una tabla cargada por una consulta o compartida en un servidor.

### Pestañas con varias tablas {#onglets-a-plusieurs-tableaux}

Una pestaña «2025» contiene a menudo varias tablas: ventas y gastos, que cambian cada año, pero también una lista de precios copiada tal cual de una pestaña a otra. Lattis combina entonces **hojas enteras**: empareja las tablas de una pestaña a otra, familia por familia (mismas columnas, como arriba), y el panel muestra, en **Otras tablas de las pestañas**, lo que pasa con cada una:

- **combinarlas** (tablas que cambian: ventas, notas mensuales): una tabla combinada con la columna de categoría, que **sigue** a la pestaña de elección;
- **conservar una sola copia** (se ofrece cuando los datos son iguales en todas partes: la lista de precios): la de la última pestaña, sin columna de categoría, la misma para todos los años. Si una copia difiere («Precios cambia en 2024»), el panel lo señala y sugiere mejor combinarlas, para no perder nada;
- **dejarlas en su pestaña** (se ofrece para una tabla que solo está en una pestaña): la pestaña se conserva entonces, visible, con una pestaña naranja para encontrarla, y el panel lo indica.

La hoja combinada toma el **diseño de la pestaña más reciente**: cada tabla combinada ocupa el lugar que tenía la suya, lo que está debajo baja tanto como ella ha crecido, y las tablas que faltan en esa pestaña vienen a continuación, debajo. Las **celdas libres** de las pestañas se trasladan (casilla que se puede desactivar en el panel): un título, una etiqueta o un total idéntico en todas partes se conserva una sola vez; `=SUMA(Ventas2025[Importe])` bajo la tabla se convierte en `=SUMA(Ventas[Importe])`, que da el total del año mostrado; un valor que cambia de una pestaña a otra («Objetivo: 5.000» en 2024, «6.000» en 2025, un título «Enero 2025») se convierte en una **celda por año**: muestra el valor del año elegido, y lo que escribe en ella solo se aplica a ese año (vacía con «(Todos los años)»).

La categoría también puede venir de una **celda** en lugar del nombre de la pestaña (un título «Enero 2025» en A1 de cada pestaña, cuando las pestañas se llaman Hoja1, Hoja2…): el panel sugiere el origen más fiable, que se elige en **Tomada de**. Las tablas similares **apiladas en la misma hoja** bajo títulos («Madrid», «Sevilla», «Valencia») se combinan de la misma manera, y la categoría viene de los títulos. Si cada pestaña tiene una columna **Fecha** cuyos valores caen todos en su mes, la columna Mes es inútil: la casilla «No agregar columna» está activada de forma predeterminada (basta con filtrar por fecha); desactívela para agregar la columna de todos modos.

**Seguir el año**: en una hoja con pestaña de elección, una tabla que sigue lleva una pequeña **insignia** con el año mostrado en la esquina de su marco; una tabla sin insignia es la misma para todos los años. Clic derecho en la tabla › Tabla › **Seguir el año** (o clic en la insignia) lo activa o desactiva: activarlo agrega la columna Año en último lugar, rellenada con el año mostrado (oculta mientras haya un año elegido); desactivarlo la quita si solo tiene un año; si no, la conserva (no se pierde nada) y la tabla muestra todas sus filas. Una tabla con formato (Insertar › Tabla) en una hoja situada en un año la sigue automáticamente; la barra de estado lo indica. `Ctrl+Z` lo deshace. Scripts: `tables.follow_choice("Precios", false)`.

## Pestaña de elección (un año, un mes, una ciudad a la vez) {#onglet-a-choix-une-annee-un-mois-une-ville-a-la-fois}

En lugar de una pestaña por año, conserve una sola tabla con una columna Año y convierta su hoja en una **pestaña de elección**: clic derecho en la pestaña › **Elegir por** › la columna (Lattis sugiere las que tienen pocos valores distintos). La pestaña se convierte en «Ventas · Año ▾»; su pequeña flecha (o `Alt+↓` en la pestaña) despliega la lista de valores:

- **un valor** (2025): todas las tablas de la hoja que tienen una columna con ese nombre muestran solo sus filas, y la columna Año, la misma en todas partes, se oculta. Las tablas sin esa columna (una lista de precios) no cambian. Las tablas dinámicas de la hoja solo cuentan ese año, como un filtro de informe impuesto; `SUBTOTALES`, los gráficos y la búsqueda lo siguen como siguen un filtro;
- **(Todos los años)**: ya no se oculta nada, la columna vuelve a aparecer;
- **Nuevo año…** sugiere el siguiente (2026 después de 2025, diciembre después de noviembre, Lote 4 después de Lote 3), lo agrega a la lista desplegable de la columna y lo muestra: la hoja parece vacía, lista para rellenarse, como una pestaña nueva. `Ctrl+Z` lo deshace.

**Desde la columna**: en una tabla ya rellena, clic derecho en el encabezado de la columna Año (o su menú ▾) › **Filtrar la pestaña por esta columna** hace lo mismo, sin pasar por la pestaña ni combinar pestañas. La pestaña empieza en todos los valores: elija el año con su flecha ▾. El elemento aparece atenuado, con una información sobre herramientas que explica por qué, para una columna calculada, vacía o con más de 100 valores distintos; en la columna ya usada como elección, se convierte en **Dejar de filtrar la pestaña**. `Ctrl+Z` lo deshace.

**Por una fórmula**: clic derecho en la pestaña › **Elegir por** › **Una fórmula…** cuando el valor que elegir no es una columna sino que se calcula a partir de las columnas de cada fila: `=AÑO([Fecha])` da una pestaña por año sin columna Año, `=TEXTO([Fecha];"mmmm")` por mes, `=SI([Importe]>1000;"Grande";"Pequeño")` por tramo. El cuadro de diálogo sugiere funciones y columnas mientras escribe, deduce el nombre de la categoría (Año, Mes…, editable) y muestra los primeros valores de la fórmula y las tablas de la hoja que la seguirán: las que tienen todas las columnas que cita (las demás no cambian; clic derecho › Tabla › **Seguir** las quita o las vuelve a poner). La lista de la pestaña ordena los valores en orden natural (2024 antes de 2025, enero antes de febrero). No hay columna que rellenar, así que no hay «Nuevo año…»: una fila escrita bajo la tabla no recibe nada automáticamente, pero sigue mostrándose mientras la escribe, hasta el siguiente cambio de valor de la pestaña, cuando se une al valor que da su fórmula. `=HOJA.ELECCION()` devuelve el valor elegido (un número para `=AÑO(…)`); las tablas dinámicas de la hoja basadas en una tabla que tiene las columnas de la fórmula solo cuentan el valor elegido. Cambiar el nombre de una columna citada actualiza la fórmula. Scripts: `sheets.choose_by_formula("Ventas", "=AÑO([Fecha])", "Año")`.

**Desde una tabla dinámica**: clic derecho en un campo de filas, columnas o filtros en el panel de la tabla dinámica (o su ▾ en la cuadrícula) › **Filtrar la pestaña por este campo**. La pestaña de elección se define en la hoja de la tabla dinámica, por la columna de su tabla de origen: la tabla dinámica solo cuenta el valor elegido, como las demás tablas dinámicas de la hoja basadas en una tabla que tiene esta columna, y las tablas de la hoja que la tienen. La tabla de origen, si está en otra hoja, no se filtra; sus valores rellenan la lista de la pestaña. Esto también funciona para una tabla dinámica basada en una tabla cargada por una consulta.

Una fila escrita bajo la tabla, o en la banda atenuada de las filas de los demás años, recibe automáticamente el año mostrado; lo mismo ocurre con una fila pegada o insertada. La elección no es un filtro: «Borrar filtro» no la toca, y sus filtros ▾ se suman a ella. `Ctrl+Alt+AvPág` y `Ctrl+Alt+RePág` pasan al año siguiente o al anterior; la barra de estado lo recuerda: «Año: 2025 (elección de la pestaña)».

Las fórmulas libres **de la hoja** ven sus tablas filtradas: `=SUMA(Ventas[Importe])` colocado bajo la tabla da el total del año mostrado, como en la antigua pestaña «2025». Las de otras hojas, la pestaña SQL y los scripts ven todas las filas. `=HOJA.ELECCION()` devuelve el valor elegido, para un resumen que sigue a la pestaña: `=SUMAR.SI.CONJUNTO(Ventas[Importe]; Ventas[Año]; HOJA.ELECCION())`.

Cambiar de año es navegación, como cambiar de pestaña: no es un paso de deshacer, pero el último valor elegido se conserva en el archivo y vuelve al abrir (si ya no tiene filas, la pestaña muestra el último valor, salvo justo después de «Nuevo año…»). Cambiar el nombre de la columna cambia el de la categoría: la pestaña lo sigue. Clic derecho › Elegir por › «Dejar de elegir por…» quita la elección (se puede deshacer). En un libro compartido, la elección se queda en su equipo: no se envía a los demás.

Scripts: `sheets.choice()`, `sheets.set_choice("Ventas", 2025)`, `sheets.choose_by("Ventas", "Año")` (para la hoja de una tabla dinámica colocada en otro lugar: `sheets.choose_by{table = "Ventas", column = "Año", sheet = "Resumen"}`), `sheets.new_choice("Ventas", 2026)` (consulte [Lua](lua)).
