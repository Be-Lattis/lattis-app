# Archivos y recuperación

Un libro de Lattis es un único archivo `.lattis`: un archivo ZIP que contiene una base de datos SQLite comprimida y un breve manifiesto. Puede enviarlo por correo y archivarlo como un libro de Excel; una vez descomprimido con 7-Zip o WinRAR (el Explorador de archivos de Windows no lee su compresión, Zstandard), cualquier herramienta SQLite puede leer sus tablas.

## Guardar {#enregistrer}

`Ctrl+S` guarda, `F12` o `Ctrl+Mayús+S` guarda con otro nombre. Guardar es seguro: el archivo se escribe primero junto al antiguo, se vuelve a leer y luego ocupa su lugar; un apagado brusco durante el guardado deja intacto el archivo antiguo.

- Un libro sin cambios no se vuelve a escribir.
- En un libro grande, la barra de estado muestra el progreso.
- Si el disco está demasiado lleno, Lattis lo indica antes de escribir nada («… necesarios, … disponibles»).
- El historial de deshacer y las columnas y filas eliminadas nunca entran en el archivo.

Cerrar el libro o la ventana con cambios sin guardar pregunta primero si desea guardarlos.

## Historial de versiones {#historique-des-versions}

Cada guardado conserva una versión del libro junto al archivo (carpeta oculta `.lattis-versions`): una cada 10 minutos, 20 como máximo y 10 veces el tamaño del libro en total. Archivo › Historial de versiones las lista (fecha, autor, tamaño); **Guardar una versión** conserva una con nombre, que se mantiene. **Abrir** muestra una versión en solo lectura; **Restaurar** reemplaza el libro por ella y la versión anterior queda en la lista. Un libro compartido en un Lattis Server no tiene historial.

## Recuperación tras un incidente {#recuperation-apres-un-incident}

Mientras trabaja, Lattis conserva una copia de trabajo del libro en su equipo. Si Lattis se detiene sin haber guardado (bloqueo, apagado forzado, corte de luz), el siguiente inicio ofrece recuperar los cambios: recuperar vuelve a abrir el libro en el estado en que estaba, con su historial de deshacer; descartar vuelve al archivo guardado.

Sus entradas y comandos se escriben en el disco en cuanto se hacen: sobreviven incluso a un corte de luz.

## Libro compartido en la red {#classeur-partage-sur-le-reseau}

Un libro abierto desde una carpeta compartida se señala a los demás equipos mediante un pequeño archivo oculto `~$nombre.lattis`. Otro equipo que lo abre recibe un aviso y puede abrirlo en solo lectura. Si el archivo se modificó en otro lugar después de abrirlo, `Ctrl+S` ofrece guardar una copia, sobrescribir o cancelar.

## Agregar una tabla a un libro compartido en directo {#ajouter-un-tableau-a-un-classeur-partage-en-direct}

En un libro colaborativo (compartido en directo en un Servidor Lattis), una tabla creada después de publicar por el propietario o un diseñador (escrita, pegada, insertada, convertida de un rango…) se agrega sola al libro compartido en cuanto se crea; sin conexión, al reconectarse. Una tabla que se queda en su equipo (cargada por una consulta, rechazada por el servidor o creada por un editor) lleva la etiqueta ámbar **en este equipo** encima de su marco: los demás miembros no la ven. Un clic en la etiqueta, o clic derecho en la tabla › **Agregar al libro compartido…**, la crea en el libro del servidor, con sus filas: los demás miembros la reciben de inmediato (en el mismo lugar de la hoja, o debajo si el lugar está ocupado en su lado), con un mensaje en la barra de estado, y después se sincroniza como las demás tablas compartidas (entradas, conflictos, derechos). La misma acción se ofrece en **Compartir en el Data Lake de Lattis…** («Agregar esta tabla al libro compartido», primero) y en **Administrar acceso** › Roles y derechos, bajo la lista de tablas.

- Reservado a los propietarios y diseñadores del libro, y solo con conexión: si no, el cuadro de diálogo lo indica.
- Los tipos de columna se fijan como para las demás tablas compartidas; las columnas calculadas siguen calculándose en cada equipo. Primero hay que corregir las entradas no válidas.
- Un rechazo del servidor (nombre ya usado por otra tabla del libro, por ejemplo) aparece en la barra de estado y no cambia nada.
- Agregar no se puede deshacer con `Ctrl+Z` (el cuadro de confirmación lo recuerda): para quitar la tabla del libro compartido, **Eliminar tabla** la elimina para todos. Deshacer (`Ctrl+Z`) la creación misma de la tabla también la elimina del servidor, para todos; `Ctrl+Y` la vuelve a crear y a agregar.

## Administrar el acceso a un libro compartido {#gerer-l-acces-a-un-classeur-partage}

**Archivo › Administrar acceso…** (o el menú de la cuenta) define quién ve y modifica un libro publicado en un Servidor Lattis. Cada miembro tiene uno o varios **roles**; sus derechos son los de sus roles combinados.

- **Miembros**: invite a una dirección con sus roles (el cuadro de diálogo indica lo que permitirán), quite un rol con la ✕ de su etiqueta, asigne uno con **+ Rol**. Bajo cada persona, una línea indica lo que puede hacer («Ver, agregar, editar sus filas»). Los roles recibidos a través de un equipo aparecen punteados y se cambian en **Equipos**. El último propietario está protegido: primero dé ese rol a otra persona.
- **Roles y derechos**: una tabla muestra, para cada rol, si puede **Ver**, **Agregar filas**, **Editar sus filas** (las que agregó, eliminación incluida), **Editar todas las filas** y cambiar **Estructura y derechos** (✓ sí, ◐ en parte: algunas filas o algunas tablas, – no). Elija un rol para definir su acceso al libro, a cada hoja y a cada tabla: *Sin acceso*, *Ver*, *Ver, agregar, editar sus filas* o *Ver y editar todo*; «Igual que la hoja» o «Igual que el libro» conserva el acceso del nivel superior. Bajo cada tabla, los filtros se leen como frases («Solo ve las filas donde Región = «Norte»», «Solo edita las filas donde Vendedor = la persona que ha iniciado sesión»); **Limitar…** o **Editar** los define. El propietario tiene todos los derechos; el diseñador también, salvo sobre los derechos.
- **Reglas avanzadas** (propietarios): una regla quita un derecho sobre una tabla sea cual sea el acceso del rol, y se lee como una frase («Nadie puede eliminar filas en «Ventas», salvo los propietarios y los diseñadores.»). Parta de un modelo —**Prohibir la eliminación**, **Solo agregar**, **Editar solo las filas con estado…**, **Editar sus filas sin crear ninguna** (para un rol que agrega y edita sus filas: a partir de entonces solo puede editar)— o elija la tabla, los roles, las acciones prohibidas y la posible excepción.
- **Ver como…**: desde la línea de un miembro o de un rol, o en su pestaña, muestra lo que ese miembro puede hacer en una tabla y las filas que ve, leídas en el servidor con su identidad (reglas incluidas).
- **Diferencias**: reglas cambiadas fuera de Lattis, directamente en PostgreSQL; **Recompilar los derechos** restablece las de Lattis.

**Retirar del servidor…** (propietario; al pie de este cuadro, o en Archivo) elimina el libro del servidor para todos: datos y permisos, pipelines y formularios; sus compartidos del Data Lake se cortan. Las copias ya presentes en los equipos se quedan: los demás miembros ven que el libro se ha eliminado y pueden conservar la suya (**Guardar una copia local…**). La suya sigue abierta como libro local. Una confirmación lo pregunta antes; no se puede deshacer.

## Servidor inaccesible: trabajar sin conexión o desvincularse {#serveur-injoignable-travailler-hors-ligne-ou-se-detacher}

Cuando el servidor de un libro compartido deja de responder (servidor detenido, red caída), aparece una franja naranja bajo la cinta, por ejemplo «El servidor localhost:8080 no responde». **Detalles** muestra el error técnico (también como información sobre herramientas y en el registro).

- **Libro colaborativo**: sigue siendo editable. Las entradas, las filas agregadas o eliminadas, las columnas agregadas o con nombre cambiado esperan en su equipo (pequeño triángulo gris en la esquina de las celdas, «Sin conexión — 3 pendientes» en la barra de estado), aunque cierre Lattis: se conservan con el libro. Cuando el servidor vuelve, se envían en el orden en que las hizo. Una celda cambiada entretanto por un compañero se convierte en un **conflicto** (panel Conflictos: conservar la suya o la de él); lo que el servidor rechaza (derechos retirados durante la caída, columna que se volvió intocable, fila eliminada por un compañero) se deshace en su equipo, con un mensaje.
- **Libro inmovilizado**: sigue siendo visible, en solo lectura, hasta que vuelve el servidor.

Lattis reintenta por sí mismo, cada vez con menos frecuencia (de unos segundos a dos minutos); la franja desaparece en cuanto el servidor responde. Los botones de la franja:

- **Reintentar** (también Archivo › **Reintentar la conexión**): lo intenta de inmediato.
- **Trabajar en una copia local**: guarda el libro con el nombre que elija como un libro normal, sin vínculo con el servidor, y luego lo abre. Los cambios aún no enviados están en la copia (el mensaje lo indica); también siguen pendientes en el libro compartido, que no se toca.
- **Desvincular del servidor…** (propietario del libro; también en Archivo, con o sin conexión): este libro se convierte en un libro local normal, que ya no se sincroniza. El libro del servidor no cambia (los demás miembros lo conservan, usted puede volver a abrirlo desde el servidor). Un libro abierto desde el servidor es solo una copia en la caché de Lattis: Lattis pregunta dónde guardarlo y luego quita la caché. Desvincular no se puede deshacer, y una confirmación lo recuerda.

## Compartir una tabla en el Data Lake de Lattis {#partager-un-tableau-au-data-lake-lattis}

Clic derecho en una tabla › **Compartir en el Data Lake de Lattis…** hace que la tabla pueda leerse desde otros libros en el Servidor Lattis (Insertar › Data Lake de Lattis, pestaña SQL), según una audiencia: personas, equipos, con un filtro de filas para cada uno. Si el libro no está publicado, el cuadro de diálogo ofrece dos maneras de hacerlo:

- **Publicar el libro con gestión de derechos**: todo el libro se publica en el servidor (Archivo › Publicar en un servidor…), con sus roles y filtros; después el cuadro de diálogo vuelve para compartir la tabla.
- **Publicar solo esta tabla como origen de datos**: el libro se queda en su equipo y no se pone a disposición. El servidor conserva una copia de la tabla, alimentada por una canalización cuyo origen es su libro. Solo se envían las **columnas activadas**, con sus valores (una columna calculada envía sus resultados, no su fórmula); las demás tablas, las hojas, las fórmulas, los scripts y las columnas no activadas no salen del equipo.

Un origen se actualiza cuando usted lo pide: clic derecho en la tabla › **Actualizar el origen del Data Lake**, o desde el mismo cuadro de diálogo; active «Actualizar el origen cada vez que se guarde el libro» para enviarlo con cada `Ctrl+S`. Cada actualización envía la tabla completa, escrita según el modo elegido: **Reemplazar todo** (el conjunto de datos pasa a ser la tabla enviada), **Anexar** (las filas se agregan a las ya recibidas) o **Actualizar por clave** (una fila con la misma clave se cambia, una nueva se agrega). El resumen aparece en la barra de estado: filas leídas, agregadas, cambiadas, eliminadas, y los valores rechazados por el servidor, con el primero (fila, columna, motivo). Una columna enviada a la que cambia el nombre, convierte o elimina en el libro también se renombra, convierte o elimina en el servidor en la siguiente actualización. El vínculo con el origen se conserva en el libro (guárdelo); el cuadro de diálogo también permite cambiar el nombre, las columnas enviadas, el modo, la clave y la audiencia, **suspender** el origen (ya no se envía nada, el conjunto de datos conserva sus últimas filas) y **quitar el origen** (los libros que lo usan conservan su última copia). Los cuadros de diálogo Publicar en un servidor y Administrar acceso enumeran los orígenes del libro, marcados «canalización». En Insertar › Data Lake de Lattis, un conjunto de datos publicado solo lleva la misma marca e indica de qué libro viene; sin conjunto de datos elegido, la lista «Sus orígenes enviados» muestra todos sus orígenes en el servidor, sea cual sea el libro del que vienen.

Necesita el rol «publicador de datos» en el servidor, y un servidor que acepte orígenes enviados por un libro; si no, el cuadro de diálogo lo indica.

## Compartir una tabla como formulario {#partager-un-tableau-comme-formulaire}

Ponga la celda activa en una tabla, y luego **Insertar › Formulario**: una página web permite rellenar esta tabla sin abrir el libro, desde un teléfono o un equipo. Cada envío agrega una fila, que los miembros ven llegar en directo; la fila se escribe a nombre del formulario («Formulario: <título>»), la persona que responde no tiene derechos sobre el libro.

El formulario lo sirve el Servidor Lattis: la tabla debe estar publicada en un libro colaborativo, por un propietario o un diseñador del libro, con conexión. Si no, el cuadro de diálogo indica lo que falta y ofrece la acción que hay que hacer primero: **Publicar en un servidor…** (libro local) o **Agregar al libro compartido…** (tabla que solo está en su equipo), y luego se abre el diseñador. Un libro inmovilizado, una tabla cargada por una consulta, un conjunto de datos del Data Lake vinculado o un origen enviado no pueden tener formulario. Un miembro que no es ni propietario ni diseñador puede copiar el vínculo de los formularios abiertos para él.

El diseñador tiene tres pestañas, con la vista previa de la página a la derecha (ancho de teléfono o de equipo); **Abrir la vista previa en el navegador** muestra la página real del servidor, sin guardar las respuestas.

- **Campos**: un campo por columna (casilla «en el formulario», controlador para mover, etiqueta, ayuda, texto de ejemplo, obligatorio, valor predeterminado fijo u hoy, ahora, año actual, dirección de quien responde). El tipo de campo sigue a la columna: texto o texto largo, número (con límites y unidad), fecha, fecha y hora, casilla de verificación o Sí / No, botones de opción o una lista con búsqueda para una columna con lista de validación. Una lista tomada de una columna de una tabla publicada la vuelve a leer el servidor; cualquier otra lista se copia («Lista copiada el…», **Actualizar**). La clave numerada la fija el servidor; una clave escrita es obligatoria. Un campo **Posición** rellena Latitud y Longitud (o una columna «lat, lon») cuando quien responde toca «Mi ubicación». Los **valores fijados por el servidor** rellenan una columna sin campo visible: Recibido el, Persona que responde, Formulario (su título), número siguiente o valor fijo; **Agregar una columna Recibido el** crea la columna en la tabla. Con una pestaña de elección, la categoría es un valor fijo, el año actual o un campo. Los intertítulos dividen el formulario en secciones.
- **Apariencia**: título, introducción (negrita, cursiva, vínculos, listas), logotipo y banda, color de una paleta legible, una sección por página, etiqueta del botón, mensaje de agradecimiento, mensaje de cierre, aviso de protección de datos (con un modelo), idioma de la página (francés o inglés). Sin carga de fotos ni de archivos.
- **Acceso**: los miembros del libro (lectores incluidos), las personas y los equipos elegidos, o cualquiera que tenga una cuenta en el servidor; todos inician sesión con su cuenta de Lattis.

Abajo, las comprobaciones aparecen en naranja (lista vacía, valor predeterminado que no está en la lista, valores de la lista visibles para quienes responden…). **Guardar borrador** conserva el formulario sin abrirlo; **Abrir el formulario** lo pone en línea y da su vínculo, su código QR (para guardar como PNG o SVG) y **Vínculo rellenado previamente…**: un vínculo por valor (un código QR por sitio, por ejemplo), que rellena campos al abrirse, con la opción «bloqueado por el vínculo» para que no se puedan cambiar. Si la tabla ya tiene formularios, el cuadro de diálogo los enumera primero (Copiar vínculo, Editar…) con **Nuevo formulario…**.

## Importar {#importer}

Archivo › Importar lee un archivo CSV (o texto delimitado), los libros `.xlsx`, `.xlsm` y OpenDocument `.ods`: cada hoja se convierte en una pestaña, cada bloque de datos en una tabla, con su formato, notas, listas de validación y formato condicional. Los gráficos y las tablas dinámicas se trasladan cuando sus datos son una tabla del libro. Lo que no se traslada (macros, otros objetos) se indica al final de la importación.

Para datos que actualizar (archivo JSON o Parquet, base de datos SQLite, PostgreSQL, MySQL, SQL Server, página web), use mejor una consulta: consulte [Importar datos y conexiones](connexions).

## Exportar {#exporter}

- **CSV**: la tabla activa o toda la hoja. En un CSV para Excel, un texto que empieza por `=`, `+`, `-` o `@` va precedido de un apóstrofo, para que Excel no lo ejecute como una fórmula.
- **Excel (`.xlsx`) y OpenDocument (`.ods`)**: todo el libro. Las tablas se convierten en tablas de Excel, las fórmulas se traducen cuando el formato tiene un equivalente; si no, se exportan como su valor. Las tablas dinámicas salen como valores; los gráficos, el formato condicional, las listas de validación y las notas salen en `.xlsx`; los scripts no se exportan. Un informe indica lo que no salió tal cual.
- **PDF**: Archivo › Imprimir, con la impresora «Microsoft Print to PDF» (consulte [Diseño de página e impresión](impression)).

## Libro de ejemplo {#classeur-d-exemple}

La página de inicio ofrece abrir un libro de ejemplo: ventas, clientes, tablas dinámicas, gráficos, un mapa, consultas SQL y scripts, con una pestaña «Descubrir» que los presenta. Nunca se guarda solo: Archivo › Guardar como para conservarlo.
