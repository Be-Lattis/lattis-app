# Consultas SQL

Cada tabla del libro es una tabla SQLite. La pestaña **SQL** (a la derecha de las pestañas de la cinta) las consulta con los nombres que ve en pantalla.

## Escribir una consulta {#ecrire-une-requete}

```
SELECT Cliente, SUM("Importe neto") AS Total
FROM Pedidos
WHERE Pagado = 0
GROUP BY Cliente
ORDER BY Total DESC
```

- Un nombre con un espacio, un acento o una mayúscula va entre comillas dobles: `"Importe neto"`.
- `Ctrl+Entrar` ejecuta la selección, si no la instrucción bajo el cursor (las instrucciones se separan con `;` o una línea vacía).
- `Ctrl+Espacio` sugiere nombres de tablas y columnas; `Alt+Arriba` / `Alt+Abajo` recorren el historial.
- El resultado muestra como máximo 1.000 filas.

Las [tablas dinámicas](tcd) se consultan como las tablas. `Cell_A1` designa una celda libre de la hoja mostrada.

## El panel izquierdo {#le-volet-de-gauche}

Tres listas: los **objetos** del libro (tablas, tablas dinámicas y sus columnas; un clic lleva a la tabla en su hoja), los **favoritos** (consultas guardadas en el libro) y el **historial**.

Con la sesión iniciada en un Servidor Lattis, los objetos muestran también los **conjuntos de datos del Data Lake de Lattis** que usted puede leer, en su propio grupo: vistas `donnees.nombre` (doble clic: consulta de ejemplo). Una consulta que los cita se ejecuta en el servidor, en solo lectura y limitada a las filas de su audiencia; se escribe en el SQL de PostgreSQL y no puede citar las tablas del libro (para combinarlos, inserte el conjunto de datos en el libro: Insertar › Data Lake de Lattis).

## Enviar el resultado a una hoja {#envoyer-le-resultat-sur-une-feuille}

El resultado de una consulta se copia en una nueva tabla, o queda **referenciado** por ella: la tabla sigue entonces vinculada a la consulta y se vuelve a leer con clic derecho › Tabla › Actualizar consulta (o `F5`). Conserva los encabezados del resultado; las columnas técnicas (`_pos`) nunca entran en ella.

### Escribir junto a un resultado vinculado {#ecrire-a-cote-d-un-resultat-lie}

Las columnas cargadas por la consulta (encabezado azul, o el color de orígenes externos del tema elegido en Vista › Tema) no se pueden editar. Para agregar su propia información, escriba un nombre justo a la derecha del encabezado: la nueva **columna de entrada** (encabezado verde) se rellena a mano, o con una fórmula, y las fórmulas del libro la ven como a las demás (`Consulta1[Comentario]`).

En cada actualización, sus valores siguen a sus filas mediante una **clave**: la columna `id` del resultado cuando existe (como con `SELECT *`), si no, una columna sin duplicados ni celdas vacías que Lattis pide con el primer valor que usted escribe. Cuando ninguna columna sola sirve, Lattis sugiere pares: con `SELECT *, 2021 AS Año FROM Seguimiento2021 UNION SELECT *, 2022 FROM Seguimiento2022`, los mismos valores de `id` vuelven de un año al siguiente, y es «id + Año» lo que identifica las filas. El número de fila es posible, pero no se recomienda: si la consulta cambia el orden de sus filas, los valores ya no siguen.

- Una fila que desaparece del resultado: sus valores se **apartan**, y vuelven si su clave vuelve. Clic derecho › Tabla › Olvidar los valores apartados los borra.
- Una clave duplicada en el resultado: los valores de esa clave se apartan, con una advertencia.
- Cambiar el nombre o eliminar una columna de entrada se hace como en una tabla normal; la clave se cambia con clic derecho › Tabla › Clave de las columnas de entrada…
- Todo se deshace con `Ctrl+Z`, actualización incluida.

Lo mismo vale para una tabla cargada por una consulta del editor ([Importar datos](connexions)).

## Modificar los datos {#modifier-les-donnees}

Las consultas `INSERT`, `UPDATE` y `DELETE` modifican las tablas como una entrada; se deshacen con `Ctrl+Z`. Mientras se ejecuta una consulta, el botón **Detener** la interrumpe; una lectura se detiene sola después de 30 s, una escritura después de 2 min (bloquea las entradas mientras dura). Una escritura detenida no cambia nada.

## Una consulta en una fórmula {#une-requete-dans-une-formule}

La función [QUERY](fonction:QUERY) ejecuta un `SELECT` en solo lectura y desborda su resultado en la hoja:

```
=QUERY("SELECT Cliente, SUM(Importe) FROM Ventas GROUP BY Cliente")
```

## En los scripts {#dans-les-scripts}

`db.query`, `db.rows` y `db.execute` pasan por el mismo SQL: consulte [Scripts Lua](lua).
