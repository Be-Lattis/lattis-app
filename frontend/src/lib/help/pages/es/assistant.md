# Asistente de IA

El **Asistente ✦** le ayuda a construir un análisis: lee la estructura de sus tablas, sugiere un enfoque y puede agregar columnas calculadas, claves, tablas dinámicas y gráficos, ordenar o dar formato. Se abre en un panel a la derecha de la cuadrícula, como Copilot en Excel, y sigue abierto cuando cambia de hoja.

## Activar el asistente {#activer-l-assistant}

El asistente está desactivado de forma predeterminada: marque **Asistente de IA** en Archivo › Configuración › Funciones. El botón **Asistente ✦** aparece entonces arriba a la derecha de la cinta (`Alt+I`) y abre su panel; desmarcado, no hay botón, atajo ni panel. En una empresa, la directiva de la organización puede desactivarlo o imponerlo.

## Conectarse {#se-connecter}

El botón **Conexión**, en el encabezado del panel, ofrece dos vías. El panel de conexión se abre solo mientras no haya nada conectado.

### Clave de API (recomendado) {#cle-d-api-recommande}

1. Elija el proveedor (Anthropic, Mistral, OpenAI, Gemini, Mammouth…) y el modelo, y luego **Abrir la consola**: la página del proveedor donde se crea una clave.
2. Copie la clave y luego **Pegar** (o `Ctrl+V` en el campo).
3. **Verificar y guardar**: Lattis prueba la clave con el proveedor antes de guardarla. Una clave rechazada o una cuenta sin créditos da un mensaje claro en el panel.

La clave se guarda en el llavero del sistema, nunca en el libro. **Olvidar la clave** la quita.

> En Anthropic, una clave solo funciona con créditos: agréguelos en la página **Billing** de la consola (platform.claude.com). Las suscripciones **Claude Max** y **Team** dan créditos de API cada mes (de 100 a 500 $ según el plan): vincule la organización de la consola en claude.ai › Settings › Billing para recibirlos. Las suscripciones Pro y Enterprise no dan ninguno.

### Aplicación de escritorio (suscripción Claude Pro, Max, Team) {#application-de-bureau-abonnement-claude-pro-max-team}

Una suscripción a Claude no puede usarse directamente en otra aplicación, pero **Claude Desktop** y **Claude Code** pueden manejar Lattis a través de su servidor MCP. La pestaña «Aplicación de escritorio» del panel de conexión (aplicación: Claude) los configura con un clic; después, reinicie Claude Desktop.

Entonces habla con Claude en su aplicación. El panel del asistente muestra **Desde Claude**: lo que Claude hace en el libro, a medida que ocurre, con **Deshacer estos cambios** para cada serie de acciones. La opción **Preguntar antes de cambiar** le hace aprobar cada escritura en Lattis, incluidos guardar con otro nombre, las exportaciones y las importaciones. El ajuste **Datos enviados al asistente** también se aplica a Claude (ver más abajo).

### Puerta de enlace de la organización {#passerelle-de-l-organisation}

Cuando la empresa proporciona su propia puerta de enlace, el panel indica «Conectado a través de la puerta de enlace de su organización»: no hay clave que escribir ni proveedor que elegir.

### Modelo local {#modele-local}

El proveedor **Modelo local** usa un modelo instalado en este equipo (Ollama, LM Studio…): ningún dato sale del equipo, y no hay clave. Su dirección se define en el panel de conexión o en Archivo › Configuración › Asistente de IA. Es experimental: los modelos pequeños llaman peor a las herramientas de Lattis.

## Conversar {#converser}

Escriba su solicitud y luego `Entrar` (`Mayús+Entrar` empieza una nueva línea). Cada mensaje sale con lo que está mirando: la hoja activa, la tabla bajo el cursor y la selección, mostradas encima del cuadro de entrada («Contexto: Pedidos, selección Importe[3:10]»). «Analiza esta tabla» o «explica esta columna» se aplican por tanto a lo que está seleccionado.

- Los pasos del asistente (leer una tabla, agregar una columna…) se agrupan en una línea **N pasos**, desplegada durante la respuesta y luego contraída; un clic la vuelve a abrir. El nombre de un paso subrayado lleva a la hoja o la tabla afectada.
- Un paso fallido que el asistente corrigió después se muestra en gris; solo un error que quedó sin resolver aparece en rojo.
- `Esc` detiene la respuesta en curso. Si se detiene antes de que el modelo responda, o falla en ese momento (red, cuota), desaparece y su mensaje vuelve al cuadro de entrada: el modelo no lo recibió.
- La flecha arriba, en un cuadro de entrada vacío, recupera el mensaje anterior; el botón de copiar, bajo una respuesta, la pone en el Portapapeles.
- **Reintentar**, bajo una respuesta interrumpida por un error (proveedor sobrecargado, caída), la reanuda donde se detuvo, sin volver a enviar su mensaje: lo que ya cambió no se vuelve a hacer, y la respuesta puede seguir deshaciéndose de una vez. Cuando el error pide esperar («reintentar en 30 s»), el botón cuenta los segundos.
- Una respuesta muy larga se detiene después de 100 pasos: **Continuar** la reanuda de la misma manera.
- Una pregunta sobre los datos («¿qué ciudad tiene la mejor facturación?») recibe un **resultado** bajo la respuesta: el asistente escribe una consulta, Lattis la ejecuta en el equipo y le muestra como máximo 50 filas. En el modo «Solo la estructura», el resultado es solo para usted: el asistente no ve ni sus valores ni su número de filas.
- Un vínculo en una respuesta muestra su dominio («Ver el informe (example.com)»); un clic muestra la dirección completa y pide confirmación antes de abrirla en el navegador.
- **Nueva** (encabezado del panel) empieza una conversación nueva; la anterior queda en la lista de conversaciones (☰), desde donde puede reanudarse o eliminarse (tras confirmación).
- Cuando la conversación se alarga, el asistente resume su comienzo para continuar: el marcador **Sesión compactada** muestra dónde. Todo sigue mostrándose en el panel; solo el modelo trabaja ahora sobre el resumen.

## Reparar tablas mal reconocidas {#reparer-des-tableaux-mal-reconnus}

La sugerencia **Reparar las tablas de esta hoja** hace que el asistente vuelva a leer el diseño de la hoja: dónde están las tablas, la naturaleza de las celdas de sus primeras y últimas filas (texto, número, fecha, fórmula, vacía), las celdas libres en negrita o combinadas, y las filas que parecen un título o un total. No lee sus valores, ni siquiera en el modo «Solo la estructura». Después corrige con los comandos que conservan la tabla, su clave y las fórmulas que la citan: título tomado por el encabezado, encabezado que quedó en los datos, fila de total sacada de la tabla, encabezados en dos filas, columnas que dividir.

Lo que el asistente lee en el libro (nombres, texto de las celdas) se le presenta como datos, nunca como una instrucción: no debe obedecer a una celda que le «pide» eliminar una columna, y lo dice. Solo elimina una columna, un gráfico, una tabla dinámica o duplicados si usted se lo pide.

El ajuste **Conservar la conversación en el libro** (Archivo › Configuración › Asistente de IA) la guarda con el libro; si no, se borra al cerrar el libro o al abrir otro. Cuando se conserva, viaja con el archivo, incluidos los resultados mostrados y las filas leídas por el asistente, y solo se guarda si el libro también ha cambiado. Si el libro cambia fuera de las respuestas (importación, `Ctrl+Z`, conversación reanudada más tarde), el asistente recibe su estructura actualizada con el siguiente mensaje.

## Deshacer {#annuler}

Todo lo que el asistente cambia durante una respuesta forma un único paso: la respuesta termina con su resumen («3 cambios») y el botón **Deshacer esta respuesta**. `Ctrl+Z` en la cuadrícula hace lo mismo. El botón solo está activo para la última respuesta, mientras no se haya cambiado nada más desde entonces; si no, use `Ctrl+Z`.

## Datos enviados {#donnees-transmises}

El ajuste **Datos enviados al asistente** (Archivo › Configuración › Asistente de IA, recordado al pie del panel) elige lo que el asistente puede leer:

| Ajuste | Lo que recibe el proveedor |
| --- | --- |
| Solo la estructura (predeterminado) | Nombres de hojas, tablas y columnas, tipos, número de filas y de valores distintos, fórmulas, posición de las tablas y las celdas en la hoja (sin su contenido, con su naturaleza y las indicaciones «título» o «total»); ni el resultado de las consultas mostradas, ni el número de celdas de un reemplazo; los pasos de las consultas y los parámetros de conexión, sin contraseñas ni los valores de los encabezados HTTP y de los filtros. Las columnas de una tabla dinámica que llevan los valores de un campo se resumen («3 columna(s) de valores del campo «Cliente»») |
| Estructura y filas de muestra | Lo mismo, más algunas filas en cada lectura: como máximo 20 filas, 200 caracteres por valor |
| Todo el libro | Todo lo que el asistente decida leer, sin límite |

El proveedor también recibe sus mensajes y el contexto (hoja, tabla, selección). Con un modelo local, nada sale del equipo.

El ajuste también se aplica a **Claude Desktop** y **Claude Code** a través del servidor MCP: sin «Todo el libro», las herramientas que leen valores (filas de tablas, SQL, celdas, scripts, exportaciones) no se les ofrecen o se limitan de la misma manera. Después de un cambio, reinicie Claude para que vea la nueva lista de herramientas. Claude Code también lee los archivos de su disco: un libro guardado sigue siendo directamente legible para él.

## Costo {#cout}

Con una clave de API, el proveedor factura cada respuesta según el número de tokens leídos y escritos. Con una clave de Anthropic, el pie del panel muestra una estimación del costo de la conversación («≈ 0,12 $ por esta conversación») basada en los precios públicos; lo que cuenta es la factura del proveedor. No se muestra nada para otros proveedores ni a través de una puerta de enlace, cuyos precios difieren. Una serie larga de pasos cuesta más: un modelo más ligero (Sonnet en lugar de Opus) suele bastar.

## Límites {#limites}

- El asistente puede equivocarse: revise lo que sugiere, y deshaga una respuesta que no le convenga.
- En el modo «Solo la estructura», no ve los valores: no puede juzgar el contenido de las celdas. En el modo «Estructura y filas de muestra», solo ve una parte.
- Una conversación muy larga se resume («Sesión compactada»): el asistente puede entonces olvidar un detalle de los primeros intercambios. Para un tema nuevo, mejor empiece una conversación nueva.
- Con Claude Desktop o Claude Code, la conversación se queda en Claude; Lattis solo ve las acciones.
