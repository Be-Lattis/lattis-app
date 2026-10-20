# Documentos y páginas web

Además de las hojas, un libro puede contener otros dos tipos de pestañas: los **Documentos**, para redactar un informe, y las **Páginas web**, para crear una pequeña aplicación. Ambos se agregan con un clic derecho en el «+» de las pestañas de abajo, o en una pestaña (Insertar).

## Documento {#document}

Una página de texto enriquecido sin fin: títulos, listas, citas, código, alineación, fuentes, colores, vínculos e imágenes, con formato desde la cinta de opciones como en un procesador de textos.

También se pueden insertar elementos vivos, alimentados por las tablas del libro:

- **gráficos** y **tablas dinámicas** (el mismo panel que en una hoja);
- **botones** que ejecutan una función de un script;
- **campos** (casilla de verificación, lista, cuadro de entrada…) cuyo valor se escribe en una celda.

Clic derecho en un elemento insertado: el mismo menú que en la hoja; en otro lugar, un menú para insertar. El texto se guarda solo un momento después de la última pulsación, como un único paso de deshacer del libro. Archivo › Imprimir / PDF se aplica al documento.

## Página web {#page-web}

Una función que se activa en Archivo › Configuración › Funciones (consulte [Configuración](parametres)); un libro que ya contiene páginas web las muestra y las usa incluso sin ella.

Una página web escrita por usted (HTML, CSS, JavaScript), mostrada en un marco aislado: no tiene acceso ni al disco ni al resto de la aplicación, y su acceso a la red pasa por Lattis. Se comunica con el libro mediante un pequeño objeto `lattis`:

```
lattis.call("sheets.list")
lattis.call("cells.set", {row: 1, col: 1, value: 42})
lattis.query("SELECT * FROM Pedidos WHERE Importe > ?", [50])
lattis.on("change", () => actualizar())
```

Los comandos de `lattis.call` son los de los [scripts Lua](lua). Una página solo llega a un sitio web mediante `lattis.call("http.get", …)`, con su aprobación. Una página que intenta abrir por sí misma otra dirección (`location.href = …`) se vuelve a cargar de inmediato; aun así, la solicitud a esa dirección puede haber salido: un motivo más para permitir solo páginas de personas de confianza.

Una página web recibida con un libro (o modificada en otro lugar) no se ejecuta de entrada: la pestaña pregunta **Permitir** o **No ejecutar**, con **Ver el código**. Una página puede leer y modificar el libro: permítala solo si confía en la persona que le dio este libro. La aprobación se aplica a esta versión de la página, en este equipo; las páginas que usted crea o modifica aquí se permiten automáticamente. Después de «No ejecutar», el botón **Decidir…** de la pestaña vuelve a hacer la pregunta.

El editor de la página está junto a la vista previa: `Ctrl+S` guarda, `Ctrl+Entrar` actualiza la vista previa sin guardar.

## Páginas web externas {#pages-web-externes}

Una página web también puede mostrar una página de un sitio web («Sitio web…»), cuando las páginas web están activadas.
