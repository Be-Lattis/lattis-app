# Introducción

Lattis funciona como una hoja de cálculo: una cuadrícula sin fin, una cinta de opciones arriba, pestañas de hojas abajo. La diferencia está debajo: cada tabla que escribe se convierte en una verdadera tabla de base de datos, con tipos y consultable en SQL.

## Visita guiada {#visite-guidee}

En tres minutos, la visita guiada muestra lo esencial en un pequeño libro nuevo: una tabla escrita, una columna calculada, dos tablas vinculadas con REFERENCIA, una tabla dinámica con su gráfico y una consulta SQL. Haga cada acción, o haga clic en Siguiente para que Lattis la haga; Omitir (o `Esc`) la detiene. Se propone en el primer inicio y se reinicia desde la ayuda (? › Visita guiada).

## Tablas detectadas {#tableaux-detectes}

Escriba una fila de encabezados y luego algunas filas de valores, en cualquier lugar: Lattis las convierte en una **tabla**, con un tipo por columna y un nombre (el de la pestaña para la primera, que se cambia a la izquierda de la barra de fórmulas). Una fila o columna escrita justo al lado la amplía; un bloque pegado también se convierte en tabla. Si la primera fila contiene números o fechas, son datos: el encabezado «Columna 1», «Columna 2»… se coloca encima, listo para renombrar. `Ctrl+L` da formato de tabla a un rango.

Lo que no está en una tabla sigue siendo una **celda libre**: un título, un parámetro, un total. Todo se deshace con `Ctrl+Z`, incluida la creación de una tabla. Reglas de detección, encabezados escritos después, grupos de encabezados en dos filas, menú contextual: consulte [Tablas](tableaux).

## Tipos de columnas {#types-de-colonnes}

Cada columna tiene un tipo, deducido de lo que escribe: Texto, Entero, Decimal, Fecha (con o sin hora), Booleano, o Calculada (una fórmula para toda la columna). El tipo se cambia en el menú del encabezado de columna; consulte [Tipos de columnas](tableaux#types-de-colonnes).

## Ordenar y filtrar {#trier-et-filtrer}

El ▾ de un encabezado ordena la tabla (de A a Z, de Z a A, orden personalizado) y filtra la columna por valores o por condición. Ordenar cambia el orden de las filas de forma definitiva; las celdas vacías van al final. Consulte [Ordenar y filtrar](tableaux#trier-et-filtrer).

## Claves {#cles}

Una **clave** es una columna cuyos valores son únicos y están todos rellenos (un código de cliente): identifica cada fila, se muestra en el margen y sirve para las búsquedas con [REFERENCIA](fonction:REFERENCIA). Una primera columna llamada ID o con 1, 2, 3… se convierte automáticamente en la clave, y una fila agregada recibe el número siguiente. Consulte [Claves](tableaux#cles).

## Fórmulas {#formules}

Una fórmula empieza por `=` y cita las columnas por su nombre. Escrita en una columna de tabla, se aplica a toda la columna:

```
=[Cantidad]*[Precio unitario]
```

Consulte [Fórmulas](formules) para las referencias y la lista de funciones. Mientras escribe, `F1` abre la página de la función bajo el cursor.

## Combinar pestañas {#regrouper-des-onglets}

¿Una pestaña por mes, por año o por ciudad, con las mismas columnas? Datos › Combinar pestañas… las reúne en una sola tabla, con una columna de categoría, y actualiza las fórmulas que las citaban. Consulte [Combinar pestañas](regroupement).

## Pestaña de elección (un año, un mes, una ciudad a la vez) {#onglet-a-choix-une-annee-un-mois-une-ville-a-la-fois}

Clic derecho en una pestaña › **Elegir por** › una columna (Año): la pestaña muestra un solo valor a la vez, elegido en su lista ▾, como una pestaña por año, con **Nuevo año…** para agregar uno. Consulte [Pestaña de elección](regroupement#onglet-a-choix-une-annee-un-mois-une-ville-a-la-fois).

## Guardar {#enregistrer}

Un libro es un único archivo `.lattis`. `Ctrl+S` guarda; tras un bloqueo, los cambios se ofrecen para recuperarlos en el siguiente inicio. `Ctrl+Z` funciona hasta que se cierra el libro, incluso después de `Ctrl+S`, pero el historial no se guarda en el archivo. Archivo › Importar y Exportar leen y escriben CSV, `.xlsx` y `.ods`. Consulte [Archivos y recuperación](fichiers) y [Diseño de página e impresión](impression).

## Proteger una hoja o el libro {#proteger-une-feuille-ou-le-classeur}

Revisar › Proteger hoja, Proteger estructura (solo la entrada en las tablas) y Proteger libro evitan los **cambios accidentales**, con una contraseña opcional (consulte [Proteger](cellules#proteger)). No es una seguridad: el archivo no está cifrado, y alguien con las herramientas adecuadas puede leer sus datos o quitar la protección sin la contraseña. No confíe en ella para datos confidenciales.

## Y después {#et-ensuite}

- Las [tablas dinámicas](tcd) y los [gráficos](graphiques) para analizar.
- [La pestaña SQL](sql) para consultar las tablas.
- Los [scripts Lua](lua) para automatizar.
- [Importar datos](connexions) de archivos, bases de datos o la Web.
