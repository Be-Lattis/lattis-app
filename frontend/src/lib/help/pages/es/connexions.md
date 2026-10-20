# Importar datos y conexiones

Datos › Obtener datos importa datos en una tabla, al estilo de Power Query: cada transformación se convierte en un paso que puede volver a aplicarse, y la tabla se actualiza a petición.

## Orígenes {#sources}

| Origen | Detalles |
| --- | --- |
| Archivo de texto o CSV | Delimitador a elección |
| Libro de Excel | Una hoja a elección |
| Archivo JSON, archivo Parquet | |
| Base de datos SQLite (archivo) | Una tabla o una consulta |
| PostgreSQL, MySQL / MariaDB, SQL Server | Mediante una conexión (ver más abajo) |
| Web (API JSON) | Una dirección que devuelve JSON |
| Tabla del libro, Otra consulta, Consulta SQL del libro | Para transformar lo que ya existe |

## El editor de consultas {#l-editeur-de-requete}

El editor muestra una vista previa de los datos y la lista de **pasos aplicados**. Los comandos de su cinta agregan un paso:

- **Columnas**: elegir, quitar, cambiar el nombre, cambiar el tipo;
- **Filas**: filtrar, ordenar (con, si hace falta, un orden personalizado: primero los valores enumerados, en ese orden), quitar duplicados, conservar las primeras filas;
- **Transformar**: dividir, reemplazar valores, dar formato (RECORTAR, MAYÚSCULAS…), columna personalizada, columna de índice, agrupar por, dinamizar, anular dinamización;
- **Combinar**: combinar (unir) o anexar las filas de otra consulta o de una tabla.

Un clic en una columna de la vista previa la selecciona; `Ctrl+Z` quita el último paso. El resultado se carga en una tabla, o queda como una simple conexión.

## Consultas y conexiones {#requetes-et-connexions}

El panel **Consultas y conexiones** (Datos › Consultas y conexiones) enumera las consultas del libro: editar, actualizar, eliminar. `F5` en una tabla cargada por una consulta la vuelve a ejecutar; `Ctrl+F5` o Datos › Actualizar todo vuelve a ejecutar todas las consultas y luego recalcula las fórmulas. Un nombre escrito a la derecha de su encabezado agrega una columna de entrada, que se conserva al actualizar (consulte [Escribir junto a un resultado vinculado](sql)).

## Libro recibido: aprobar los orígenes {#classeur-recu-accord-des-sources}

Una consulta está escrita en el libro. Para que un libro recibido no pueda, al actualizarse, llegar a un servidor desconocido o copiar un archivo de su equipo, Lattis le pide su aprobación la primera vez que una consulta quiere leer un archivo o llegar a un host (API web, servidor de base de datos). La aprobación se aplica a la sesión, o a este libro en este equipo con «Recordar». Los orígenes de una consulta escrita en el editor, y los archivos que usted elige en él, se aprueban automáticamente. Los hosts aprobados pueden revisarse en la pestaña Script › Acceso a la red.

## Conexiones a bases de datos {#connexions-aux-bases}

Una **conexión** describe un servidor (PostgreSQL, MySQL / MariaDB, SQL Server, o una API web): host, puerto, base de datos, usuario. Se guarda en el libro y sirve a varias consultas. Su menú **⋯** en el panel le cambia el nombre: las consultas que la usan siguen su nuevo nombre (`Ctrl+Z` lo deshace todo).

Las contraseñas nunca se guardan en el libro: permanecen en el llavero del sistema, y Lattis las pide en la primera actualización. El panel también permite olvidarlas.

En una empresa, las conexiones pueden venir de la configuración del equipo; aparecen sin tener que escribir nada.
