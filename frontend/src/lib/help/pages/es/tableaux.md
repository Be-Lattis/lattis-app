# Tablas

Una tabla es un bloque de datos escrito, pegado o importado: Lattis lo reconoce, asigna un tipo a sus columnas y lo convierte en una tabla consultable en SQL.

## Tablas detectadas {#tableaux-detectes}

Escriba unas cuantas filas en cualquier lugar de la hoja: una fila de encabezados y luego valores. Lattis reconoce el bloque de datos y lo convierte en una **tabla**: el encabezado se resalta, cada columna recibe un tipo, y la tabla recibe un nombre que puede cambiar en el cuadro de nombre de tabla, a la izquierda de la barra de fórmulas (escriba el nuevo nombre y luego Entrar). La primera tabla de una hoja toma el nombre de la pestaña (sin espacios: «Ventas 2026» da `Ventas2026`), las siguientes Tabla1, Tabla2… Cambiar el nombre de la pestaña también cambia el de la tabla que lleva su nombre (no al revés); esa tabla no muestra su etiqueta de nombre, ya que la pestaña ya lo muestra.

- Hacen falta al menos dos columnas en dos filas (tres celdas llenas de un cuadrado de 2 × 2): una lista de una sola columna, o una sola fila, se queda como celdas libres.
- La primera fila que escribe es el encabezado, aunque se parezca a las siguientes («Nombre | Ciudad» y luego «Lea | Madrid»), salvo si contiene un valor que no es texto: un número, una fecha, una hora, un booleano, un porcentaje, un importe («Lea | 31», «País | 2024», «10 | 20»). Entonces todas las filas escritas son datos y el encabezado se coloca justo encima, con los nombres «Columna 1», «Columna 2»… en cursiva gris: escriba ahí el nombre de cada columna, o clic derecho en la 1.ª fila › «Usar la primera fila como encabezados» (también en clic derecho › Tabla): se convierte en el encabezado en su lugar, la fila de «Columna n» desaparece, las fórmulas la siguen, y una columna vuelve a ser numérica si su único texto era el nombre. Para que un número sea un nombre de columna, escríbalo tras un apóstrofo ('2024). Sin espacio libre encima (primera fila de la hoja, una celda, un título o una tabla justo encima), las celdas quedan libres: Inicio › Dar formato como tabla (`Ctrl+L`) las convierte en una tabla, cuya primera fila da los encabezados.
- Una fila escrita justo debajo de una tabla la amplía; también una columna escrita justo a su derecha.
- Un bloque pegado desde otro programa se convierte en una tabla, con la misma regla de encabezado que al escribir: su 1.ª fila es el encabezado si solo contiene texto; un número, una fecha… («País | 2024», «10 | 20») la convierten en una fila de datos, bajo «Columna 1», «Columna 2»… (clic derecho › «Usar la primera fila como encabezados» para recuperarla). Pegado en la primera fila de la hoja, sin espacio encima, el encabezado «Columna n» ocupa el lugar de la 1.ª fila pegada y el bloque baja una fila.
- Encabezados escritos después de los datos: escriba los nombres en la fila justo encima de la tabla. En cuanto hay dos nombres encima de un encabezado que parece datos («Lea | 31», «Columna 1»), la barra de estado ofrece «Usar como encabezados»; si no, clic derecho en una de estas celdas › «Usar como encabezados de «Tabla1»» (o clic derecho en el encabezado › Tabla › «Usar la fila de encima como encabezados»). Los nombres sustituyen a los nombres de columna (una celda dejada vacía conserva el suyo), el encabezado antiguo se convierte en la 1.ª fila de datos, como números si eran números, y las fórmulas lo siguen; `Ctrl+Z` lo deshace. Una sola palabra escrita encima sigue siendo un título.
- Arrastre una tabla junto a otra para combinarlas.
- `Supr` en celdas de una tabla: una fila en la que ya no queda nada escrito se elimina, como si nunca se hubiera escrito (una columna calculada, la columna oculta de una pestaña de elección o el número dado automáticamente por una clave entera no la conservan; la barra de estado indica «N filas vacías eliminadas», `Ctrl+Z` las devuelve). Una fila que ya estaba vacía, donde `Supr` no borró nada, se queda.
- Lo que no está en una tabla sigue siendo una **celda libre**: un título, un parámetro, un total.

Todo se puede deshacer con `Ctrl+Z`, incluida la creación de una tabla.

**Un clic derecho** en una tabla ofrece primero lo que solo vale ahí (Ir a, Usar la primera fila como encabezados, Sacar esta fila de la tabla en la última fila, Usar la fila superior como encabezados en un encabezado…), luego Cortar, Copiar, Pegar; Borrar contenido, Formato de celdas… (`Ctrl+1`), Formato condicional…; la nota. Al final vienen los submenús de cada objeto, titulados con su nombre: **Ordenar y filtrar**, **Fila «12»** (su clave cuando la tabla tiene una: insertar encima o debajo, eliminar la fila), **Columna «Nombre»** (cambiar nombre, tipo y clave, validación, filtrar la pestaña, convertir en valores, insertar a la izquierda o a la derecha, eliminar la columna) y **Tabla «Lecturas»** (encabezados, uso compartido, Convertir en rango, Eliminar tabla). Los números de fila, las letras de columna, las tablas dinámicas (**Tabla dinámica «Ventas»**), los gráficos y los botones siguen el mismo orden.

### Grupos de encabezados (fila de encima) {#groupes-d-en-tetes-ligne-du-dessus}

Cuando una fila de grupos está encima de los encabezados (BOSQUE sobre Nombre, Estado y Parcela), clic derecho en el encabezado › Tabla › «Encabezados en 2 filas (grupos encima)»: estas celdas, combinadas o no, se convierten en los grupos de la tabla, mostrados encima de los encabezados. Una columna se llama entonces «Grupo.Nombre»: `Inventario[BOSQUE.Nombre]` en una fórmula, `"BOSQUE.Nombre"` en SQL. Escriba en la banda de grupos para dar o cambiar el nombre de un grupo (las fórmulas lo siguen), bórrelo para quitarlo. «Quitar los grupos de encabezados» vuelve a poner los nombres completos en una sola fila.

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

El texto «null» escrito, pegado o importado (CSV, .xlsx) deja la celda vacía; escriba 'null para conservar la palabra. En una tabla, una celda vacía muestra un guion gris «–» (no se copia ni se exporta nada); una celda libre vacía queda en blanco.

## Ordenar y filtrar {#trier-et-filtrer}

El ▾ de un encabezado de columna (o clic derecho, o Datos › Ordenar y filtrar) ordena la tabla por esa columna: de A a Z, de Z a A, o en un **orden personalizado**. Ordenar cambia el orden de las filas de forma definitiva; las celdas vacías van siempre al final. El mismo menú filtra la columna por valores («(en blanco)» al final de la lista) o por condición.

### Orden personalizado {#tri-personnalise}

**Orden personalizado…** abre la lista de los valores de la columna, para disponerlos en el orden que quiera: arrástrelos, use los botones Subir, Bajar, Mover al principio, Mover al final, o el teclado `Alt+↑` / `Alt+↓` (`Alt+Inicio` / `Alt+Fin`). Elija **En este orden** o **Invertido**, y luego Aceptar. Los valores se comparan sin distinguir mayúsculas y minúsculas; los que no están en el orden siguen en orden ascendente, y los vacíos se quedan al final.

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
