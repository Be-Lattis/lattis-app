# Archivos y recuperación

Un libro de Lattis es un único archivo `.lattis`: un archivo ZIP que contiene una base de datos SQLite comprimida y un breve manifiesto. Puede enviarlo por correo y archivarlo como un libro de Excel; una vez descomprimido con 7-Zip o WinRAR (el Explorador de archivos de Windows no lee su compresión, Zstandard), cualquier herramienta SQLite puede leer sus tablas.

## Guardar {#enregistrer}

`Ctrl+S` guarda, `F12` o `Ctrl+Mayús+S` guarda con otro nombre. Guardar es seguro: el archivo se escribe primero junto al antiguo, se vuelve a leer y luego ocupa su lugar; un apagado brusco durante el guardado deja intacto el archivo antiguo.

- Un libro sin cambios no se vuelve a escribir.
- En un libro grande, la barra de estado muestra el progreso.
- Si el disco está demasiado lleno, Lattis lo indica antes de escribir nada («… necesarios, … disponibles»).
- El historial de deshacer y las columnas y filas eliminadas nunca entran en el archivo.

Cerrar el libro o la ventana con cambios sin guardar pregunta primero si desea guardarlos.

## Abrir un archivo .xlsx {#ouvrir-un-fichier-xlsx}

Archivo › Abrir (o «Abrir con», o arrastrar y soltar sobre la ventana) abre también un archivo `.xlsx`: la barra de título muestra «nombre.xlsx». Al primer `Ctrl+S`, Lattis pregunta si conservar el formato `.xlsx` o convertir en libro `.lattis`, y dice lo que `.xlsx` no conserva (scripts, tablas dinámicas, fórmulas sin equivalente…). «No volver a preguntar» recuerda la elección (Archivo › Configuración › Archivos). Fórmulas, formatos, estilos, combinaciones, notas, vínculos, validaciones, formato condicional, gráficos e imágenes se vuelven a escribir en el archivo; un rango normal sigue siendo un rango. La recuperación tras un incidente también vale para un `.xlsx`.

## Historial de versiones {#historique-des-versions}

Cada guardado conserva una versión del libro junto al archivo (carpeta oculta `.lattis-versions`): una cada 10 minutos, 20 como máximo y 10 veces el tamaño del libro en total. Archivo › Historial de versiones las lista (fecha, autor, tamaño); **Guardar una versión** conserva una con nombre, que se mantiene. **Abrir** muestra una versión en solo lectura; **Restaurar** reemplaza el libro por ella y la versión anterior queda en la lista. Un libro compartido en un Lattis Server no tiene historial.

## Recuperación tras un incidente {#recuperation-apres-un-incident}

Mientras trabaja, Lattis conserva una copia de trabajo del libro en su equipo. Si Lattis se detiene sin haber guardado (bloqueo, apagado forzado, corte de luz), el siguiente inicio ofrece recuperar los cambios: recuperar vuelve a abrir el libro en el estado en que estaba, con su historial de deshacer; descartar vuelve al archivo guardado.

Sus entradas y comandos se escriben en el disco en cuanto se hacen: sobreviven incluso a un corte de luz.

## Libro compartido en la red {#classeur-partage-sur-le-reseau}

Un libro abierto desde una carpeta compartida se señala a los demás equipos mediante un pequeño archivo oculto `~$nombre.lattis`. Otro equipo que lo abre recibe un aviso y puede abrirlo en solo lectura. Si el archivo se modificó en otro lugar después de abrirlo, `Ctrl+S` ofrece guardar una copia, sobrescribir o cancelar.

Para trabajar en directo con otras personas en un Servidor Lattis, consulte [Uso compartido en directo y Data Lake](partage).

## Importar {#importer}

Archivo › Importar lee un archivo CSV (o texto delimitado), los libros `.xlsx`, `.xlsm` y OpenDocument `.ods`: cada hoja se convierte en una pestaña, cada bloque de datos en una tabla, con su formato, notas, listas de validación y formato condicional. Los gráficos y las tablas dinámicas se trasladan cuando sus datos son una tabla del libro. Lo que no se traslada (macros, otros objetos) se indica al final de la importación.

Para datos que actualizar (archivo JSON o Parquet, base de datos SQLite, PostgreSQL, MySQL, SQL Server, página web), use mejor una consulta: consulte [Importar datos y conexiones](connexions).

## Exportar {#exporter}

- **CSV**: la tabla activa o toda la hoja. En un CSV para Excel, un texto que empieza por `=`, `+`, `-` o `@` va precedido de un apóstrofo, para que Excel no lo ejecute como una fórmula.
- **Excel (`.xlsx`) y OpenDocument (`.ods`)**: todo el libro. Las tablas se convierten en tablas de Excel, las fórmulas se traducen cuando el formato tiene un equivalente; si no, se exportan como su valor. Las tablas dinámicas salen como valores; los gráficos, el formato condicional, las listas de validación y las notas salen en `.xlsx`; los scripts no se exportan. Un informe indica lo que no salió tal cual.
- **SQL (`.sql`)**: un script `CREATE TABLE` y luego `INSERT` de las tablas y las tablas dinámicas (en valores), para todo el libro, la hoja activa o la tabla activa. El dialecto Estándar sirve para la mayoría de las bases; PostgreSQL, MySQL, SQL Server y SQLite ajustan nombres, tipos y booleanos a su base. La columna clave pasa a ser la clave primaria; una columna calculada sale como valores, con su fórmula en un comentario; las celdas libres no se exportan.
- **PDF**: Archivo › Imprimir, con la impresora «Microsoft Print to PDF» (consulte [Diseño de página e impresión](impression)).

## Libro de ejemplo {#classeur-d-exemple}

La página de inicio ofrece abrir un libro de ejemplo: ventas, clientes, tablas dinámicas, gráficos, un mapa, consultas SQL y scripts, con una pestaña «Descubrir» que los presenta. Nunca se guarda solo: Archivo › Guardar como para conservarlo.
