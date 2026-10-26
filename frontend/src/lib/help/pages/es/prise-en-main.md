# Introducción

Lattis funciona como una hoja de cálculo: una cuadrícula sin fin, una cinta de opciones arriba, pestañas de hojas abajo. La diferencia está debajo: cada tabla que escribe se convierte en una verdadera tabla de base de datos, con tipos y consultable en SQL.

## Visita guiada {#visite-guidee}

En tres minutos, la visita guiada muestra lo esencial en un pequeño libro nuevo: una tabla escrita, una columna calculada, dos tablas vinculadas con REFERENCIA, una tabla dinámica con su gráfico y una consulta SQL. Haga cada acción, o haga clic en Siguiente para que Lattis la haga; Omitir (o `Esc`) la detiene. Se propone en el primer inicio y se reinicia desde la ayuda (? › Visita guiada).

## Tablas detectadas {#tableaux-detectes}

Escriba unas cuantas filas en cualquier lugar de la hoja: una fila de encabezados y luego valores. Lattis reconoce el bloque de datos y lo convierte en una **tabla**: el encabezado se resalta, cada columna recibe un tipo, y la tabla recibe un nombre que puede cambiar en el cuadro de nombre de tabla, a la izquierda de la barra de fórmulas (escriba el nuevo nombre y luego Entrar). La primera tabla de una hoja toma el nombre de la pestaña (sin espacios: «Ventas 2026» da `Ventas2026`), las siguientes Tabla1, Tabla2… Cambiar el nombre de la pestaña también cambia el de la tabla que lleva su nombre (no al revés); esa tabla no muestra su etiqueta de nombre, ya que la pestaña ya lo muestra.

- Hacen falta al menos dos columnas en dos filas (tres celdas llenas de un cuadrado de 2 × 2): una lista de una sola columna, o una sola fila, se queda como celdas libres.
- La primera fila que escribe es el encabezado, aunque se parezca a las siguientes («Nombre | Ciudad» y luego «Lea | Madrid»), salvo si contiene un valor que no es texto: un número, una fecha, una hora, un booleano, un porcentaje, un importe («Lea | 31», «País | 2024», «10 | 20»). Entonces todas las filas escritas son datos y el encabezado se coloca justo encima, con los nombres «Columna 1», «Columna 2»… en cursiva gris: escriba ahí el nombre de cada columna, o clic derecho en la 1.ª fila › «Usar la primera fila como encabezados» (también en clic derecho › Tabla): se convierte en el encabezado en su lugar, la fila de «Columna n» desaparece, las fórmulas la siguen, y una columna vuelve a ser numérica si su único texto era el nombre. Para que un número sea un nombre de columna, escríbalo tras un apóstrofo ('2024). Sin espacio libre encima (primera fila de la hoja, una celda, un título o una tabla justo encima), las celdas quedan libres: Inicio › Dar formato como tabla (`Ctrl+L`) las convierte en una tabla, cuya primera fila da los encabezados.
- Una fila escrita justo debajo de una tabla la amplía; también una columna escrita justo a su derecha.
- Un bloque pegado desde Excel u otro programa se convierte en una tabla, con la misma regla de encabezado que al escribir: su 1.ª fila es el encabezado si solo contiene texto; un número, una fecha… («País | 2024», «10 | 20») la convierten en una fila de datos, bajo «Columna 1», «Columna 2»… (clic derecho › «Usar la primera fila como encabezados» para recuperarla). Pegado en la primera fila de la hoja, sin espacio encima, el encabezado «Columna n» ocupa el lugar de la 1.ª fila pegada y el bloque baja una fila.
- Encabezados escritos después de los datos: escriba los nombres en la fila justo encima de la tabla. En cuanto hay dos nombres encima de un encabezado que parece datos («Lea | 31», «Columna 1»), la barra de estado ofrece «Usar como encabezados»; si no, clic derecho en una de estas celdas › «Usar como encabezados de «Tabla1»» (o clic derecho en el encabezado › Tabla › «Usar la fila de encima como encabezados»). Los nombres sustituyen a los nombres de columna (una celda dejada vacía conserva el suyo), el encabezado antiguo se convierte en la 1.ª fila de datos, como números si eran números, y las fórmulas lo siguen; `Ctrl+Z` lo deshace. Una sola palabra escrita encima sigue siendo un título.
- Arrastre una tabla junto a otra para combinarlas.
- `Supr` en celdas de una tabla: una fila en la que ya no queda nada escrito se elimina, como si nunca se hubiera escrito (una columna calculada, la columna oculta de una pestaña de elección o el número dado automáticamente por una clave entera no la conservan; la barra de estado indica «N filas vacías eliminadas», `Ctrl+Z` las devuelve). Una fila que ya estaba vacía, donde `Supr` no borró nada, se queda.
- Lo que no está en una tabla sigue siendo una **celda libre**, como en Excel: un título, un parámetro, un total.

Todo se puede deshacer con `Ctrl+Z`, incluida la creación de una tabla.

**Un clic derecho** en una tabla ofrece primero lo que solo vale ahí (Ir a, Usar la primera fila como encabezados, Sacar esta fila de la tabla en la última fila, Usar la fila superior como encabezados en un encabezado…), luego Cortar, Copiar, Pegar; Borrar contenido, Formato de celdas… (`Ctrl+1`), Formato condicional…; la nota. Al final vienen los submenús de cada objeto, titulados con su nombre: **Ordenar y filtrar**, **Fila «12»** (su clave cuando la tabla tiene una: insertar encima o debajo, eliminar la fila), **Columna «Nombre»** (cambiar nombre, tipo y clave, validación, filtrar la pestaña, convertir en valores, insertar a la izquierda o a la derecha, eliminar la columna) y **Tabla «Lecturas»** (encabezados, uso compartido, Convertir en rango, Eliminar tabla). Los números de fila, las letras de columna, las tablas dinámicas (**Tabla dinámica «Ventas»**), los gráficos y los botones siguen el mismo orden.

### Grupos de encabezados (fila de encima) {#groupes-d-en-tetes-ligne-du-dessus}

Cuando una fila de grupos está encima de los encabezados (BOSQUE sobre Nombre, Estado y Parcela), clic derecho en el encabezado › Tabla › «Encabezados en 2 filas (grupos encima)»: estas celdas, combinadas o no, se convierten en los grupos de la tabla, mostrados encima de los encabezados como en Excel. Una columna se llama entonces «Grupo.Nombre»: `Inventario[BOSQUE.Nombre]` en una fórmula, `"BOSQUE.Nombre"` en SQL. Escriba en la banda de grupos para dar o cambiar el nombre de un grupo (las fórmulas lo siguen), bórrelo para quitarlo. «Quitar los grupos de encabezados» vuelve a poner los nombres completos en una sola fila.

Una columna sin grupo se queda **en un nivel**: su celda de la banda y su encabezado forman una sola celda, combinada en las dos filas, y su nombre no tiene punto. Lattis se adapta solo: un grupo sobre una sola columna cuyo encabezado no tiene nombre real (Topografía encima de «Columna 17») se convierte en el nombre de la columna, y un grupo que repite el encabezado (DESCUBRIDOR encima de Descubridor) desaparece. Para ajustarlo a mano, con Inicio › Combinar y centrar:

- **la celda de la banda y el encabezado de una columna** › Combinar: la columna pasa a un nivel; conserva su subencabezado, o toma el nombre del grupo si su encabezado era automático;
- **una columna en un nivel** › Separar (el mismo botón): la celda de la banda se separa del encabezado; escriba ahí el grupo (si no escribe nada, se queda en un nivel). Escribir «Grupo.Nombre» en el encabezado hace lo mismo;
- **varias celdas contiguas de la banda** › Combinar: un solo grupo sobre estas columnas, el primero que llevaban («Grupo» si no había ninguno: escriba su nombre en la banda);
- **un grupo sobre varias columnas** › Separar: el grupo se queda en su primera columna, las demás pasan a un nivel.

Cada vez, las columnas cambian de nombre (las fórmulas y el SQL las siguen), en un único paso de deshacer, y la barra de estado indica lo que cambió. Se rechaza combinar la banda con datos, con celdas fuera de la tabla o con otra tabla.

## Tipos de columnas {#types-de-colonnes}

Cada columna tiene un tipo, deducido de lo que escribe:

| Tipo | Ejemplos |
| --- | --- |
| Texto | Madrid, Ref-001 |
| Número entero | 12, -3 |
| Decimal | 12,5; 1.234,50 € |
| Fecha | 15/01/2026; con hora: 15/01/2026 14:30 o 14:30 15/01/2026 |
| Verdadero/Falso | VERDADERO, FALSO |
| Calculada | una fórmula aplicada a toda la columna |

Un valor que ya no corresponde al tipo amplía la columna cuando es posible (un decimal en una columna de números enteros la convierte en decimales, y la barra de estado lo indica); si no, la celda se marca como error. El tipo se cambia en el menú del encabezado de la columna.

El texto «null» escrito, pegado o importado (CSV, Excel) deja la celda vacía; escriba 'null para conservar la palabra. En una tabla, una celda vacía muestra un guion gris «–» (no se copia ni se exporta nada); una celda libre vacía queda en blanco.

## Ordenar y filtrar {#trier-et-filtrer}

El ▾ de un encabezado de columna (o clic derecho, o Datos › Ordenar y filtrar) ordena la tabla por esa columna: de A a Z, de Z a A, o en un **orden personalizado**. Ordenar cambia el orden de las filas de forma definitiva; las celdas vacías van siempre al final. El mismo menú filtra la columna por valores («(en blanco)» al final de la lista) o por condición.

### Orden personalizado {#tri-personnalise}

**Orden personalizado…** abre la lista de los valores de la columna, para disponerlos en el orden que quiera, como las listas personalizadas de Excel: arrástrelos, use los botones Subir, Bajar, Mover al principio, Mover al final, o el teclado `Alt+↑` / `Alt+↓` (`Alt+Inicio` / `Alt+Fin`). Elija **En este orden** o **Invertido**, y luego Aceptar. Los valores se comparan sin distinguir mayúsculas y minúsculas; los que no están en el orden siguen en orden ascendente, y los vacíos se quedan al final.

- **Listas integradas**: Días (lunes… domingo), Días abreviados, Meses (enero… diciembre), Meses abreviados. Cuando los valores de la columna corresponden a una lista, se ofrece de inmediato.
- **Listas del libro**: active «Guardar este orden como lista personalizada» y póngale un nombre (Tallas, Prioridades…); se ofrece entonces para cualquier otra columna o campo de tabla dinámica, y viaja con el archivo.
- El orden **se conserva para la columna**: se reutiliza en el siguiente Orden personalizado, y una tabla cargada por una consulta se vuelve a ordenar en este orden en cada actualización (mientras una ordenación normal no lo haya sustituido). Los valores del orden que faltan en los datos se quedan en él, para cuando aparezcan.
- `Ctrl+Z` deshace la ordenación en un paso. En los scripts: `tables.sort_custom`, `tables.sort_lists`, `tables.set_sort_lists`, `columns.sort_order`.

## Claves {#cles}

Una **clave** es una columna cuyos valores son únicos y están todos rellenos: un código de cliente, una referencia de producto. Identifica cada fila, se muestra en el margen en lugar de los números de fila y sirve para las búsquedas con [REFERENCIA](fonction:REFERENCIA).

Al crear una tabla (escritura, pegado, Dar formato como tabla), su primera columna se convierte automáticamente en la clave si solo contiene números enteros, únicos y todos rellenos, y su encabezado es **ID**, o sus valores son **1, 2, 3…** en orden. La barra de estado lo indica («clave definida en ID») y la 🔑 aparece en el encabezado; `Ctrl+Z` la deshace junto con la creación, clic derecho › Quitar la clave la quita sola. Un archivo importado conserva sus tablas tal cual.

En los demás casos, cuando la primera columna de una tabla se presta a ello, Lattis lo sugiere discretamente, y basta un clic. «No volver a sugerir» quita la sugerencia para esa tabla, que conserva entonces sus números de fila.

**Numeración automática.** Una fila agregada a una tabla cuya clave es un número entero recibe el número siguiente (el mayor más 1), tanto si se escribe bajo la tabla como si se pega, se inserta en medio o la agrega un script; las demás filas no se vuelven a numerar. Una primera columna que sigue una serie regular sin ser la clave (10, 20, 30…) también se rellena para una fila agregada al final. Un valor que escribe en la columna sustituye al número sugerido. La información sobre herramientas del encabezado indica una columna numerada.

Cuando escribe una búsqueda ([REFERENCIA](fonction:REFERENCIA), BUSCARV, BUSCARX, EXISTE…) en una tabla que no tiene clave, Lattis le pregunta qué debe usar:

- **el número de fila** de la tabla: 1 para la primera fila, en el orden de la tabla, que cambia si ordena o inserta filas. Un BUSCARV se convierte en `INDICE(…; COINCIDIR(…; 0))`: la primera fila encontrada, duplicados permitidos. La elección se aplica a toda la tabla: la pregunta no vuelve;
- **la columna buscada**, que se convierte en la clave de la tabla: sus valores deben ser únicos y estar todos rellenos, y a partir de entonces se rechazará en ella un duplicado.

`Ctrl+Z` deshace a la vez la fórmula y esta elección. Una tabla que ya tiene clave no pregunta nada, y una fórmula pegada o importada conserva los números de fila sin preguntar.

## Combinar pestañas {#regrouper-des-onglets}

Muchos libros tienen **una pestaña por mes, por año o por ciudad**, con las mismas columnas. Lattis las detecta: sus pestañas se subrayan con la misma línea de color, y una etiqueta «⧉ 12 pestañas similares» sigue a la serie (al pasar el puntero se resaltan las pestañas afectadas). Después de importar un libro de Excel, al abrir, o cuando copia una pestaña de la serie por tercera vez (incluso antes de cambiar el nombre de la copia), la barra de estado lo ofrece una vez: **Combinar…**, **Más tarde** (la etiqueta se queda) o **No volver a sugerir** (se conserva en el archivo: a un compañero que lo recibe no se le vuelve a preguntar; también con clic derecho en la etiqueta).

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

## Fórmulas {#formules}

Una fórmula empieza por `=` y cita las columnas por su nombre. Escrita en una columna de tabla, se aplica a toda la columna:

```
=[Cantidad]*[Precio unitario]
```

Consulte [Fórmulas](formules) para las referencias y la lista de funciones. Mientras escribe, `F1` abre la página de la función bajo el cursor.

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

## Guardar {#enregistrer}

Un libro es un único archivo `.lattis` (una base de datos SQLite y su descripción). `Ctrl+S` guarda. Cerrar el libro o la ventana de Lattis con cambios sin guardar pregunta primero si desea guardarlos; si Lattis se detiene sin guardar (bloqueo, apagado forzado), los cambios se ofrecen para recuperarlos en el siguiente inicio (consulte [Archivos y recuperación](fichiers)).

El historial de deshacer nunca se guarda en el archivo: `Ctrl+Z` funciona hasta que se cierra el libro, incluso después de `Ctrl+S`, pero un archivo vuelto a abrir (o enviado a alguien) ya no tiene historial, ni las columnas o filas eliminadas.

Un libro abierto desde una carpeta compartida (red, OneDrive…) se señala a los demás equipos mediante un pequeño archivo oculto `~$nombre.lattis` colocado junto a él: otro equipo que lo abre recibe un aviso y puede abrirlo en solo lectura. Si el archivo se modificó en otro lugar después de abrirlo, `Ctrl+S` ofrece guardar una copia, sobrescribir o cancelar.

Archivo › Importar lee archivos CSV y Excel; Archivo › Exportar escribe CSV, `.xlsx` u OpenDocument (`.ods`) (en un CSV para Excel, un texto que empieza por `=`, `+`, `-` o `@` va precedido de un apóstrofo, para que Excel no lo ejecute como una fórmula); Archivo › Imprimir / PDF imprime o guarda como PDF, según Vista › Diseño de página (consulte [Diseño de página e impresión](impression)).

## Proteger una hoja o el libro {#proteger-une-feuille-ou-le-classeur}

Revisar › Proteger hoja rechaza cualquier cambio en el contenido de la hoja; Proteger estructura solo permite la entrada en las tablas (valores, filas agregadas o eliminadas, ordenar, filtrar); Proteger libro rechaza agregar, eliminar o cambiar el nombre de hojas, y también puede bloquear la estructura o el contenido de todas las hojas. Una contraseña opcional evita quitar la protección por error. Como en Excel, es una protección contra los **cambios accidentales**, no una seguridad: el archivo no está cifrado, y alguien con las herramientas adecuadas puede leer sus datos o quitar la protección sin la contraseña. No confíe en ella para datos confidenciales.

## Y después {#et-ensuite}

- Las [tablas dinámicas](tcd) y los [gráficos](graphiques) para analizar.
- [La pestaña SQL](sql) para consultar las tablas.
- Los [scripts Lua](lua) para automatizar.
- [Importar datos](connexions) de archivos, bases de datos o la Web.
