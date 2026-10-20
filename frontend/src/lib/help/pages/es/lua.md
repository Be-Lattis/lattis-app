# Scripts Lua

La pestaña **Script** automatiza el libro en Lua (versión 5.1): leer y escribir tablas, manejar las hojas, hacer preguntas al usuario, reaccionar a los controles.

## La pestaña Script {#l-onglet-script}

- **A la izquierda, la biblioteca**: los scripts guardados en el libro (viajan con el archivo `.lattis`). Clic derecho: cambiar nombre, duplicar, eliminar.
- **En el centro, el editor**, que revisa el script a medida que escribe: errores de sintaxis en rojo, nombres desconocidos y número incorrecto de argumentos en amarillo.
- **A la derecha, la referencia**: las funciones de abajo, con un botón para insertar su ejemplo.
- **Abajo, la salida**: lo que muestra `print`, y luego el resumen de la ejecución.

`Ctrl+Entrar` ejecuta el script, `Ctrl+S` lo guarda en el libro (después, no olvide guardar el libro).

## Leer y escribir {#lire-et-ecrire}

Las consultas se escriben en SQL con los nombres mostrados de las tablas y las columnas; un nombre con un espacio o un acento va entre comillas dobles:

```
for r in db.rows('SELECT Cliente, sum("Importe neto") AS total FROM Pedidos GROUP BY Cliente') do
  print(r.Cliente, r.total)
end
```

`db.insert`, `db.update` y `db.delete` escriben como si se escribiera en la cuadrícula. Todo lo que escribe un script forma **un único paso de deshacer**: `Ctrl+Z` lo borra de una vez.

## Seguridad y límites {#securite-et-limites}

Un script se ejecuta en un espacio aislado: sin acceso a archivos, programas ni variables de entorno. Las solicitudes HTTP (`http.get`…) solo llegan a los sitios que usted ha permitido. Un script se detiene después de 30 segundos, por encima de 512 MB de memoria, o con el botón Detener. `db.query` devuelve como máximo 100.000 filas (`db.rows` no tiene límite).

Un script recibido con un libro (botón, control, evento) pide su aprobación antes de ejecutarse por primera vez: **Permitir** o **Denegar**, con **Ver el código**. Un script puede leer y modificar todo el libro: permítalo solo si confía en la persona que le dio este libro. Los scripts escritos en este equipo se permiten automáticamente.

Al abrir, Lattis elimina del libro los desencadenadores SQL (`TRIGGER`) que no ha creado él mismo (Lattis nunca guarda ninguno en el archivo) y lo indica en la barra de estado.

## Botones y controles {#boutons-et-controles}

Una función de un script se asocia a un botón o a un control de la hoja: consulte [Botones y controles](controles).

## Eventos del libro {#evenements-du-classeur}

Un script puede reaccionar a lo que usted hace en el libro: `events.on(evento, función, script)` suscribe una función de un script guardado, y la suscripción viaja con el archivo.

```
function entrada(e)
  ui.notify(e.address .. ": " .. tostring(e.old) .. " → " .. tostring(e.value))
end
events.on("change", "entrada", "Mi script")
```

| Evento | Cuándo |
|---|---|
| `open` | el libro acaba de abrirse |
| `before_save` / `save` | antes / después de guardar (devolver `false` cancela el guardado) |
| `before_close` | antes de que se cierre el libro o la ventana (devolver `false` lo cancela) |
| `sheet` | se muestra otra pestaña |
| `selection` | la selección ha cambiado (cuando deja de moverse) |
| `change` | se han cambiado celdas: entrada, pegado, borrado, movimiento, fila de tabla |
| `sheet_add` / `sheet_delete` | se agrega / elimina una hoja |
| `recalc` | se recalculan las fórmulas (F9) |

La función recibe una tabla: `sheet`, `row`, `col`, `address` («B3»), `value` y `old` para una sola celda escrita… Solo sus acciones desencadenan estos eventos, nunca un script: un agente de escucha que escribe en la hoja no se vuelve a desencadenar a sí mismo. Los agentes de escucha se ejecutan sin bloquear la cuadrícula, lo que cambian se deshace de una vez (`Ctrl+Z`), y un error se notifica una sola vez. `events.enable(false)` los desactiva durante la sesión.

## Referencia de funciones {#reference-des-fonctions}
