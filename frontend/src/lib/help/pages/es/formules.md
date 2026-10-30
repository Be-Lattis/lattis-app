# Fórmulas

Una fórmula empieza por `=`. Las funciones se escriben con su nombre en español (`SUMA`, `SI`, `CONTAR.SI`…, igual que `VERDADERO`, `FALSO` y los valores de error `#¡VALOR!`, `#N/D`…); los nombres en inglés (`SUM`, `IF`…) también se aceptan. Los argumentos se separan con `;` y los decimales se escriben con coma: `=REDONDEAR([Precio]*1,2; 2)`.

Mientras escribe, Lattis sugiere funciones, tablas y columnas, muestra la sintaxis de la función en curso (el argumento que hay que escribir en negrita) y colorea las referencias. `F1` abre aquí la página de la función bajo el cursor; Fórmulas › Insertar función abre la lista completa.

## Referencias {#references}

| Notación | Designa |
| --- | --- |
| `[Columna]` | En una tabla: el valor de la columna en la fila de la fórmula |
| `[Columna][2]` | El valor de la columna en la 2.ª fila de la misma tabla |
| `Tabla[Columna]` | La columna entera (en un agregado, o fuera de la tabla); sola en una celda libre (`=Ventas[Precio]`), sus valores se desbordan hacia abajo |
| `Tabla[Columna][2]` | Un valor, por el número de su fila (1 = primera fila de datos) |
| `Tabla[Columna][1:3]` | Las filas 1 a 3 de una columna |
| `Cell[A1]` | La celda libre A1 de la hoja |
| `Hoja2!Cell[A1]` | La celda libre A1 de otra hoja (`'Mi hoja'!Cell[A1]` si el nombre tiene espacios) |
| `Cell[C14:C16]` | Un rango de celdas libres |

Los números de fila siguen el orden de presentación: ordenar una tabla cambia la fila designada, como en una hoja de cálculo. Para buscar una fila por su clave, use [REFERENCIA](fonction:REFERENCIA).

En modo fórmula, un clic en una celda inserta su referencia; arrastrar inserta un rango. Desde una celda libre, un clic en una celda de una tabla inserta ese único valor (`Informe[PRUEBA][2]`, por el número de su fila); un clic en el encabezado, o arrastrar sobre toda la columna, inserta la columna entera (`Informe[PRUEBA]`).

## Direcciones de celda (C14, A1:B5) {#adresses-a-la-excel}

Una dirección escrita por costumbre (`=C14*2`, `=SUMA(C14:C16)`, `$C$14`, `Hoja2!B3`) se convierte al confirmar: en `Cell[C14]` para una celda libre, en `Tabla[Columna][k]` en una tabla, en un rango para `C14:C16`.

[BUSCARV](fonction:BUSCARV), [BUSCARH](fonction:BUSCARH) y [BUSCARX](fonction:BUSCARX) también funcionan con celdas libres (`=BUSCARV(A2; D2:F20; 2; FALSO)`), con coincidencia exacta o aproximada: se convierten en [INDICE](fonction:INDICE) y [COINCIDIR](fonction:COINCIDIR) o [COINCIDIRX](fonction:COINCIDIRX).

## Columnas calculadas {#colonnes-calculees}

Una fórmula escrita en una columna de tabla se aplica a todas sus filas: la columna se vuelve **calculada**. `[Columna]` designa ahí el valor de la misma fila. Los agregados aceptan una expresión sobre las columnas de una tabla, calculada fila a fila: `=SUMA(Ventas[Precio]*Ventas[Cant])`.

### Convertir en valores {#convertir-en-valeurs}

Clic derecho en una columna calculada › **Convertir en valores** (o Fórmulas › Convertir en valores): la fórmula se quita y la columna se vuelve una columna normal que conserva sus valores actuales, con el tipo de esos valores (número entero, decimal, fecha, booleano, si no texto; un error como `#¡DIV/0!` sigue señalado). Conserva su nombre: las fórmulas que la citan la siguen leyendo, pero ya no sigue a las columnas que leía su fórmula. Varias columnas calculadas seleccionadas se convierten juntas; en celdas libres, el comando sustituye sus fórmulas por sus valores (un resultado desbordado conserva su fórmula). Ctrl+Z devuelve las fórmulas. No se permite con una estructura protegida y, por ahora, en una tabla compartida de un libro colaborativo.

## Resultados que se desbordan {#resultats-qui-se-deversent}

Algunas funciones devuelven varios valores ([FILTRAR](fonction:FILTRAR), [ORDENAR](fonction:ORDENAR), [UNICOS](fonction:UNICOS), [SECUENCIA](fonction:SECUENCIA), [DIVIDIRTEXTO](fonction:DIVIDIRTEXTO), [CONSULTA](fonction:CONSULTA)…): el resultado se desborda en las celdas vecinas, como las matrices dinámicas de una hoja de cálculo. Una columna entera sola en una celda libre (`=Ventas[Precio]`) se desborda igual, en el orden de las filas de la tabla. Si el espacio está ocupado, la fórmula muestra `#¡DESBORDAMIENTO!`. En un agregado, aportan sus valores: `=CONTARA(DIVIDIRTEXTO(Cell[A1]; ","))` cuenta los trozos de un texto; las funciones que recorren una columna (CONTAR.SI, K.ESIMO.MAYOR, COINCIDIR…) no los leen (`#¡VALOR!`). UNICOS, ORDENAR, TOMAR y EXCLUIR leen una columna (`=UNICOS(SeguimientoNidos[ID nido])`), una tabla entera, un rango de celdas (`Cell[A1:A20]`) u otra matriz dinámica; FILTRAR y ORDENARPOR, una tabla o una columna.

`=DIVIDIRTEXTO("Nombre=García;Ciudad=Madrid"; "="; ";")` divide un texto en filas (en `;`) y luego en columnas (en `=`); las filas más cortas se completan con `#N/D`, o con el último argumento. Para desbordarse, el texto no debe depender de una fila de tabla: una celda, o un valor como `Ventas[Notas][2]`.

Una **constante matricial** se escribe entre llaves: `{1;2;3}` (una columna), `{"a"."b"}` (una fila), `{1.2;3.4}`. Se desborda como una matriz dinámica y sirve sobre todo para dar varios delimitadores: `=DIVIDIRTEXTO(Cell[A1]; {",";" "})`.

### Convertir un resultado desbordado en tabla {#convertir-un-resultat-deverse-en-tableau}

El área desbordada se enmarca en azul; su información sobre herramientas recuerda la celda de la fórmula. **Clic derecho** en esta área:

- **Convertir en tabla vinculada**: una tabla real sustituye al resultado. Sigue vinculada a la fórmula (guardada en una consulta del libro, origen «Fórmula», visible en Datos › Consultas) y **se recalcula sola** cuando cambian sus orígenes: una entrada en `SeguimientoNidos` actualiza al instante la tabla obtenida de `=UNICOS(SeguimientoNidos[ID nido])`, y un solo Ctrl+Z deshace a la vez la entrada y el recálculo. Sus columnas calculadas son de solo lectura (en azul, el color del tema para los orígenes externos); un nombre escrito a la derecha de su encabezado le agrega una **columna de entrada** (en verde, el color de las tablas) cuyos valores siguen a su fila cuando el resultado cambia (por la propia columna para un `UNICOS` de una columna, si no por la clave pedida), como junto a una consulta. F5 también la actualiza. Si agrega pasos a su consulta (Datos › Consultas), a partir de entonces solo se actualiza con F5.
- **Convertir en tabla (valores)**: una tabla normal conserva los valores actuales, sin vínculo.

En ambos casos, **los encabezados** son los textos colocados justo encima de la fórmula, si hay uno encima de cada columna (un título «Nidos» encima de `=UNICOS(…)` se convierte en el encabezado de la tabla, llamada Nidos); si no, los nombres de las columnas de la fórmula («ID nido», «Valor» para SECUENCIA), en la fila vacía encima de la fórmula (los valores no se mueven) o, en su defecto, en la fila de la fórmula (los valores bajan una fila). Un solo paso de deshacer devuelve la fórmula.

## Números escritos como texto {#nombres-ecrits-en-texte}

En una columna de texto, las funciones que esperan números ([SUMA](fonction:SUMA), [PROMEDIO](fonction:PROMEDIO), [MIN](fonction:MIN), [MAX](fonction:MAX), [SUMAR.SI](fonction:SUMAR.SI), [SUMAPRODUCTO](fonction:SUMAPRODUCTO), las estadísticas…) leen cada texto como el número que representa: «12», «1.234,5», «12 €», «15 %» (0,15), «(12)» (−12), y las notaciones extranjeras «1,234.56» o «1'234.5». **Un texto que no es un número cuenta como 0**: el promedio de «12» y «abc» es 6. Las celdas vacías siguen ignorándose. [CONTAR](fonction:CONTAR) solo cuenta los textos que son números: `=CONTARA(Ventas[Nota]) - CONTAR(Ventas[Nota])` da el número de textos que no lo son.

Una hoja de cálculo ignora estos textos. Un rango de celdas libres (`Cell[A1:A10]`) sigue esta regla: sus textos son etiquetas, ignoradas. En un cálculo directo (`=[Nota]*2`), un texto que no es un número da `#¡VALOR!`.

## Nombres intermedios {#noms-intermediaires}

[LET](fonction:LET) da nombre a valores, calculados una sola vez, que pueden usarse en los valores siguientes y en el cálculo final: `=LET(neto; [Precio]*[Cant]; iva; neto*0,21; neto+iva)`. Un nombre no puede ser ni una dirección de celda (`A1`, `Cell[A1]`) ni un nombre de función.

## Valores de error {#valeurs-d-erreur}

| Error | Causa habitual |
| --- | --- |
| `#¡VALOR!` | Argumento de tipo incorrecto, o columnas de dos tablas mezcladas |
| `#¡DIV/0!` | División por cero |
| `#N/D` | Valor no encontrado (BUSCARX, COINCIDIR, REFERENCIA, MUNICIPIO…) |
| `#¿NOMBRE?` | Función, tabla o columna desconocida |
| `#¡REF!` | Celda o rango eliminado |
| `#CIRC!` | Referencia circular: la fórmula depende de sí misma |
| `#¡NUM!` | Número fuera de rango (raíz cuadrada de un negativo…) |

## Estadística y finanzas {#statistiques-et-finances}

Las funciones estadísticas leen una columna entera, un rango o una expresión sobre las columnas de una tabla; los booleanos y los vacíos se ignoran, los textos se leen como se describe arriba: [DESVEST.M](fonction:DESVEST.M), [VAR](fonction:VAR), [MODA](fonction:MODA), [PERCENTIL](fonction:PERCENTIL), [CUARTIL](fonction:CUARTIL), [COEF.DE.CORREL](fonction:COEF.DE.CORREL), [PENDIENTE](fonction:PENDIENTE)…: `=CUARTIL(Ventas[Importe]; 3)`.

Las funciones financieras siguen las convenciones habituales de las hojas de cálculo: un importe pagado es negativo, y la tasa es la de un período. La cuota mensual de un préstamo de 200.000 € a 20 años al 4 % anual se escribe `=PAGO(4%/12; 20*12; 200000)` (−1.211,96). Consulte también [VA](fonction:VA), [VF](fonction:VF), [NPER](fonction:NPER), [TASA](fonction:TASA), [PAGOINT](fonction:PAGOINT) y [PAGOPRIN](fonction:PAGOPRIN) para un cuadro de amortización. [VNA](fonction:VNA) e [TIR](fonction:TIR) leen los flujos de una columna **en el orden de presentación** de la tabla: `=TIR(Flujos[Importe])`.

También están disponibles distribuciones de probabilidad ([DISTR.NORM.N](fonction:DISTR.NORM.N), [DISTR.T.N](fonction:DISTR.T.N)…), pruebas ([PRUEBA.T.N](fonction:PRUEBA.T.N)), regresión ([PRONOSTICO.LINEAL](fonction:PRONOSTICO.LINEAL), [ESTIMACION.LINEAL](fonction:ESTIMACION.LINEAL)), amortizaciones ([SLN](fonction:SLN), [AMORTIZ.PROGRE](fonction:AMORTIZ.PROGRE)) y valores ([PRECIO](fonction:PRECIO)). [BDSUMA](fonction:BDSUMA) y las demás funciones de base de datos leen sus criterios en un rango de celdas: `=BDSUMA(Ventas; "Importe"; Cell[E1:F3])`. [DESREF](fonction:DESREF) e [INDIRECTO](fonction:INDIRECTO) calculan una referencia: `=SUMA(DESREF(Cell[B2]; 0; 0; Cell[A1]))`.

## Municipio de un punto GPS {#commune-d-un-point-gps}

[MUNICIPIO](fonction:MUNICIPIO) da el municipio francés (commune) que contiene un punto: `=MUNICIPIO([Latitud]; [Longitud])`, o su código INSEE (`"code"`), sus códigos postales (`"cp"`), su departamento (`"departement"`) o su región (`"region"`). El cálculo se hace sin conexión, con los límites municipales integrados en Lattis (Francia metropolitana y de ultramar), y sigue siendo rápido en una columna de 100.000 filas. Un punto en el mar o fuera de Francia da `#N/D`. Como los límites están simplificados, un punto a unas decenas de metros de un límite puede caer en el municipio vecino.

*Contiene datos del IGN y del INSEE (límites municipales ADMIN EXPRESS, edición 2026, distribuidos por Etalab) y los códigos postales de La Poste a través de geo.api.gouv.fr, bajo la Licence Ouverte / Etalab 2.0.*

## Vínculos entre fórmulas {#liens-entre-formules}

**Fórmulas › Mostrar vínculos** traza los vínculos de las celdas seleccionadas: flechas azules desde lo que lee su fórmula, flechas rojas hacia las fórmulas que las leen. Una columna entera (`Clientes[Ciudad]`) se señala por su encabezado; una fila concreta (`Clientes[Ciudad][1]`, `[Saldo][-1]`, la fila encontrada por `REFERENCIA`) por su celda, y un rango de filas (`Ventas[Importe][2:5]`) se enmarca. Si esa fila está oculta (filtro, pestaña de elección, fila oculta) o no se encuentra, la flecha señala la columna y la barra de estado indica por qué. Los vínculos hacia otra hoja se cuentan ahí. Un clic en la cuadrícula (o `Esc`) borra las flechas.

## Análisis de hipótesis {#analyse-de-scenarios}

**Datos › Análisis de hipótesis**: **Buscar objetivo** encuentra el valor de una celda que da a una fórmula el resultado deseado; el **Administrador de escenarios** guarda conjuntos de valores con nombre, los muestra y los resume en una hoja nueva; la **Tabla de datos** calcula una fórmula para una lista de valores (una o dos variables, resultados como valores fijos: vuelva a ejecutarla para recalcular); **Solver** maximiza, minimiza o alcanza un valor con restricciones (exacto para un problema lineal, aproximado si no). Nada cambia antes de **Conservar**; `Ctrl+Z` deshace. Actívelo en Archivo › Configuración › Funciones.

## Recálculo {#recalcul}

Las fórmulas se recalculan solas cuando cambian sus datos. `F9` lo recalcula todo; `HOY`, `AHORA` y `ALEATORIO` solo cambian al abrir y con `F9`.

## Todas las funciones {#toutes-les-fonctions}
