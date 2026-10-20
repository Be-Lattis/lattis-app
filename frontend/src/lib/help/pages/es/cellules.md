# Formato, notas y presentación

Los comandos de la cinta siguen los de Excel, en los mismos lugares. Se aplican a la selección (o a la celda activa) y se deshacen con `Ctrl+Z`.

## Formato de las celdas {#mise-en-forme-des-cellules}

Inicio › Fuente, Alineación y Número dan formato a la selección; el pequeño iniciador en la esquina de cada grupo, o `Ctrl+1`, abre el cuadro de diálogo **Formato de celdas** (número, alineación, fuente, borde, relleno). Un clic en la alineación ya elegida (Izquierda, Centro, Derecha) vuelve a la alineación estándar, como en Excel.

## Colores de fuente y de relleno {#couleurs-de-police-et-de-remplissage}

Inicio › Color de fuente y Color de relleno (Color de resaltado en un documento): el botón aplica el último color elegido, la flecha ▾ abre la paleta, como en Excel.

- **Colores del tema**: los diez colores del tema del libro (Fondo 1 y Texto 1, blanco y negro; Fondo 2 y Texto 2; Énfasis 1 a 6, entre ellos el color de las tablas y el de los orígenes externos), y debajo de cada uno cinco variantes más claras o más oscuras. Un color tomado de aquí **sigue al tema**: cambiar el tema (Vista › Tema) vuelve a colorear estas celdas.
- **Colores estándar**: rojo oscuro, rojo, naranja, amarillo, verde claro, verde, azul claro, azul, azul oscuro y morado, los mismos sea cual sea el tema.
- **Colores recientes**: los elegidos con «Más colores…» en este equipo.
- **Sin relleno** o **Automático** quita el color; **Más colores…** permite elegir cualquier color.

La información sobre herramientas de cada muestra da su nombre («Verde, Énfasis 1, 40 % más claro»). Con el teclado, las flechas recorren las muestras y `Entrar` elige. La misma paleta se usa en Formato de celdas (`Ctrl+1`) y en el formato condicional. Al exportar a Excel u OpenDocument, los colores del tema se escriben tal como se muestran.

Inicio › Edición › Borrar quita los formatos, el contenido (`Supr`) o la nota de la selección. Rellenar copia la primera fila hacia abajo (`Ctrl+D`) o la primera columna hacia la derecha (`Ctrl+R`).

Inicio › Dar formato como tabla (`Ctrl+L`) crea una tabla a partir del bloque de celdas libres alrededor de la celda activa; su primera fila da los encabezados.

## Formato condicional {#mise-en-forme-conditionnelle}

Inicio › Formato condicional colorea las celdas según su valor: reglas para resaltar (mayor que, entre, texto que contiene, duplicados…), barras de datos, escalas de color y conjuntos de iconos. El **administrador de reglas** enumera las reglas de la hoja, para editarlas o eliminarlas. En una tabla, una regla se aplica a toda la columna, incluidas las filas futuras.

## Validación de datos {#validation-des-donnees}

Datos › Validación de datos da la lista de valores permitidos en la selección, o en toda una columna de tabla: la celda muestra una flecha desplegable (`Alt+Abajo` la abre), y cualquier otra entrada se rechaza.

La lista se escribe (**Valores escritos**, uno por línea) o se toma de un **Rango**, como en Excel:

- una columna de tabla, por ejemplo una tabla de referencia: `Clientes[Ciudad]` (o algunas de sus filas, `Clientes[Ciudad][1:5]`);
- celdas: `Cell[A1:A10]`, o `Hoja2!Cell[A1:A10]` en otra hoja (se acepta `A1:A10` escrito a la manera de Excel);
- una celda con nombre.

**Seleccionar** oculta el cuadro de diálogo mientras señala el rango (clic, arrastrar o `Mayús+flechas`; un clic en el encabezado de una columna de tabla toma la columna entera), si hace falta en otra hoja: haga clic en su pestaña, la referencia se convierte en `Hoja2!Cell[A1:A10]`. `Entrar` la toma, `Esc` vuelve sin cambiarla; en ambos casos, vuelve a la hoja y a la selección de partida. El título del cuadro de diálogo indica a qué se aplica la validación (columna de tabla, celdas), y el cuadro muestra una vista previa de los valores.

La lista de un rango se vuelve a leer cada vez que se abre la lista desplegable y en cada entrada: valores distintos, en el orden del rango, sin celdas vacías. La referencia sigue los cambios de nombre de la tabla, de la columna o de la hoja, y la inserción o eliminación de filas y columnas. Si el origen desaparece (tabla eliminada, filas del rango eliminadas), la lista desplegable y el cuadro de diálogo lo indican, y las entradas se rechazan hasta que se corrija la validación.

### Columnas de referencia: ir de una tabla a otra {#colonnes-de-reference-basculer-d-un-tableau-a-l-autre}

Cuando el rango es la **columna clave** de otra tabla (la marcada con 🔑, por ejemplo `Especies[Nombre]`), la columna validada se convierte en una **referencia** a esa tabla, como una clave externa: la barra de estado lo indica.

- **Ir a**: `Ctrl+clic` en una celda de la columna, el pequeño botón ↗ junto a su flecha de lista, o clic derecho › Ir a «Especies» › Urraca, lleva a la fila referenciada (en su hoja), que se ilumina un momento. Si el valor no existe en la tabla de destino, el elemento aparece atenuado y lo indica.
- **Volver**: `Alt+Izquierda`, o «← Volver a…» en la barra de estado, le devuelve a la celda de partida (varios saltos se deshacen uno a uno).
- **Filas que la citan**: clic derecho en una celda de la columna clave de la tabla de destino › Filas que citan «Urraca» (3) filtra la tabla que la cita por ese valor y va a su primera fila.

No se guarda nada más: quitar la validación (o `Ctrl+Z`) o la clave de la tabla de destino deshace la referencia, y esta sigue los cambios de nombre de la tabla y de su columna clave. Los valores que faltan no los bloquea una restricción de la base de datos: es la validación la que los rechaza al escribirlos.

Las validaciones no se exportan a Excel (.xlsx), ni se leen al importar.

## Notas {#notes}

Revisar › Nueva nota (`Mayús+F2`) asocia una nota a la celda activa: una esquina roja la señala, y se muestra al pasar el puntero. En una celda que ya tiene una, el comando se convierte en **Editar nota**. La chincheta de la nota la mantiene visible. Revisar › Eliminar nota la quita.

## Inmovilizar paneles {#figer-les-volets}

Vista › Inmovilizar paneles mantiene filas y columnas en pantalla al desplazarse:

- **Inmovilizar paneles**: las filas de encima y las columnas a la izquierda de la celda activa;
- **Inmovilizar fila superior** o **Primera columna**;
- **Movilizar paneles** permite que vuelvan a desplazarse.

La configuración pertenece a la hoja y se guarda con el libro.

## Tema de color {#theme-de-couleur}

Vista › Tema elige el color de las tablas del libro: el de las tablas normales (verde de forma predeterminada) y el de las tablas cargadas por un origen externo o una consulta (azul de forma predeterminada). Cada tema del menú muestra sus dos colores; el encabezado, su texto, el marco, los iconos y la etiqueta del nombre se derivan de ellos, con un texto siempre legible sobre el encabezado.

- **Personalizar…**: dos colores a su elección, con una vista previa inmediata en la cuadrícula, y luego los demás colores de la paleta (Énfasis 3 a 6, Texto 2, Fondo 2), derivados de los dos primeros si no los toca; **Agregar a mis temas** lo conserva en la lista para sus otros libros (se puede quitar desde la misma ventana).
- Cada tema tiene sus **colores de paleta** (consulte Colores de fuente y de relleno): las celdas coloreadas con un color del tema cambian con él. Un libro sin tema propio conserva el tema predeterminado en cuanto se usa en él un color del tema, para que el color siga siendo el mismo para un compañero.
- El tema se guarda con el libro (`Ctrl+Z` lo deshace): un compañero lo abre con los mismos colores, incluso sin ese tema. Un libro compartido en un Servidor Lattis lo publica con su diseño.
- Un libro que no ha elegido tema (libro nuevo, o anterior a los temas) toma el **tema predeterminado** de Archivo › Configuración › Vista.

Su organización puede ofrecer sus propios temas, ofrecer solo esos o imponer el tema predeterminado. Los colores del tema también se exponen como variables CSS (`--lattis-table-head`, `--lattis-source-head`…).

## Proteger {#proteger}

Revisar › Proteger hoja rechaza cualquier cambio en su contenido; aún puede moverse, seleccionar, copiar, buscar, filtrar e imprimir. Revisar › Proteger estructura lo bloquea todo salvo la entrada en las tablas: escribe, pega (solo valores), borra valores en sus celdas de datos, agrega filas escribiendo debajo, inserta o elimina filas, ordena y filtra; se rechazan las celdas libres, el formato, las columnas y los encabezados, las propias tablas, los objetos, los nombres, las reglas, las filas y columnas de la hoja, el diseño de página y la pestaña. Revisar › Proteger libro rechaza agregar, eliminar, cambiar el nombre, mover u ocultar hojas; puede elegir bloquear también la estructura de todas las hojas (solo entrada en las tablas), o su contenido. Los comandos rechazados aparecen atenuados, y su información sobre herramientas indica cómo quitar la protección (consulte [Introducción](prise-en-main#proteger-une-feuille-ou-le-classeur)).

En un libro compartido en un Servidor Lattis, una tabla publicada que sus derechos no le permiten cambiar (rol Lector, libro inmovilizado, sin conexión) atenúa igualmente los comandos que cambiarían sus datos; su formato, la ordenación y los filtros siguen permitidos, ya que solo se aplican en su equipo.

En un libro colaborativo, el propietario y los diseñadores cambian la **estructura** de una tabla compartida como la de una tabla local: agregar, cambiar el nombre, eliminar una columna, cambiar su tipo, elegir la clave, cambiar el nombre o eliminar la tabla. El cambio se hace en el servidor antes de aplicarse en el equipo, y los demás lo reciben de inmediato (su tabla se vuelve a leer; se borra el historial de deshacer que la afectaba). El servidor puede negarse, y su mensaje aparece en la barra de estado: una columna citada por el filtro de un rol, una restricción o la audiencia de un conjunto de datos no puede cambiar de nombre, eliminarse ni convertirse; una tabla que alimenta el Data Lake o una canalización no puede eliminarse. `Ctrl+Z` deshace el cambio también en el servidor (si se niega, el paso queda por deshacer); una tabla eliminada y luego restaurada solo vuelve en su equipo, como tabla local. Los demás miembros (editores, participantes) no cambian la estructura, hay que estar conectado, y los índices del servidor se definen con los comandos `tables.add_index` y `tables.drop_index` (scripts, asistente).
