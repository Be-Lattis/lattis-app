# Botones y controles

Como los controles de formulario de una hoja de cálculo, los controles se colocan libremente en una hoja (o en un [documento](onglets)): Insertar › Botón, o Insertar › Campo.

## Tipos de controles {#types-de-controles}

| Control | Qué hace |
| --- | --- |
| Botón | Un clic ejecuta la acción vinculada |
| Casilla de verificación | Escribe VERDADERO o FALSO en la celda vinculada |
| Lista desplegable | Escribe la opción elegida en la celda vinculada |
| Cuadro de entrada | Escribe el texto en la celda vinculada al salir de él, o con `Entrar` |
| Cuadro de texto | Como el cuadro de entrada, en varias líneas (`Ctrl+Entrar` confirma) |
| Selector de archivos | Escribe la ruta del archivo elegido en la celda vinculada |

## El panel del control {#le-volet-du-controle}

Al insertarlo (o con clic derecho › Editar), el panel de la derecha define:

- el **nombre** del control, para los scripts (automático si está vacío);
- su **etiqueta**;
- la **celda vinculada**: un clic en la hoja la rellena mientras el campo tiene el foco;
- la **acción**: una función de un script de la biblioteca (pestaña Script).

Un control puede crearse sin acción y vincularse más tarde. Todo lo que cambia una acción se deshace con un solo `Ctrl+Z`.

## Trabajar con un control {#manipuler-un-controle}

Arrástrelo para moverlo; `Ctrl+clic` lo selecciona para cambiar su tamaño; clic derecho: ejecutar, editar, cortar, copiar, eliminar.

## En un script {#dans-un-script}

Los scripts leen y escuchan los controles con `controls.list`, `controls.get` y `controls.on`: consulte [Scripts Lua](lua).
