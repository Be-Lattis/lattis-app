# Gráficos

Un gráfico está conectado a una tabla o a una tabla dinámica, y se actualiza con ella.

## Insertar un gráfico {#inserer-un-graphique}

1. Haga clic en la tabla (o la tabla dinámica) que quiere representar.
2. Insertar › elija un tipo de gráfico.
3. El gráfico se coloca en la hoja y el panel **Gráfico** se abre a la derecha.

Si hay varias columnas de la tabla seleccionadas, definen el gráfico, como en Excel: la primera columna de texto da las categorías, las columnas de números las series.

El gráfico se mueve con el mouse y se cambia de tamaño con sus controladores. Un clic en él vuelve a abrir el panel; clic derecho: cortar, copiar, copiar como imagen (para pegarlo en un correo o un documento), eliminar. Con el teclado, una vez seleccionado el gráfico: las flechas lo mueven, Mayús+flechas cambian su tamaño, Tab pasa al gráfico siguiente, Supr lo elimina.

## Datos representados {#donnees-tracees}

Como en Excel, un gráfico solo muestra las filas que deja visibles el filtro de la tabla; la casilla **Filas ocultas por el filtro** del panel las vuelve a incluir todas.

Un gráfico representa como máximo 2.000 puntos: por encima, lo indica bajo el dibujo («Primeros 2.000 puntos de 50.000»). Una línea demasiado larga se reduce en lugar de cortarse: cada intervalo conserva su primer y su último punto, su mínimo y su máximo. Los gráficos de cajas y bigotes y los histogramas se calculan siempre sobre todas las filas.

Los valores toman el formato de su columna (€, %, decimales, fechas) en los ejes, las etiquetas y la información sobre herramientas. Una columna de fechas en X da un eje de tiempo para los gráficos de líneas y de áreas, y también puede leerse en un gráfico de dispersión.

## Tipos {#types}

Columnas, barras, líneas, áreas, circular, anillo, dispersión, combinado (columnas y líneas, con eje secundario posible), columnas 3D y circular 3D, cascada, rectángulos, radial, embudo, cajas y bigotes, histograma, medidor, y **mapa** (puntos o densidad sobre un mapa de OpenStreetMap, a partir de columnas de latitud y longitud).

## El panel Gráfico {#le-volet-graphique}

- **Tipo**: cambia la forma sin perder los datos elegidos; «Apilado» para columnas y áreas.
- **Datos**: el origen (una tabla o una tabla dinámica), la columna de categorías (eje horizontal) y una o varias series de valores, en el orden elegido con las flechas ▲ ▼; para las líneas, las **celdas vacías** cortan la línea (hueco), cuentan como cero o se unen.
- **Colores**: el de cada serie, o el de cada categoría para los gráficos circulares, de anillo, de embudo y de rectángulos, y aumento, disminución y total para la cascada.
- **Títulos**: título del gráfico y de los ejes.
- **Elementos**: leyenda, etiquetas de datos; eje secundario para una serie de un gráfico combinado.

## Mapas {#cartes}

Un mapa lee dos columnas (latitud, longitud), o una sola columna que contiene ambas. Los mapas base vienen de OpenStreetMap: necesitan conexión, y el acceso al servidor de teselas requiere su aprobación.

Para hacer zoom, seleccione el mapa (un clic): la rueda del mouse acerca alrededor del puntero, arrastrar mueve el mapa, un doble clic acerca (Mayús+doble clic aleja), y las teclas **+** y **−** también hacen zoom. Mientras el mapa no está seleccionado, la rueda del mouse desplaza la hoja o el documento; Alt+arrastrar mueve el gráfico seleccionado. Los botones **+**, **−** y **recentrar** aparecen arriba a la derecha al pasar el puntero. La vista elegida se guarda con el libro (Ctrl+Z la deshace); **recentrar**, aquí o en el panel del gráfico, vuelve a la vista ajustada a los puntos.

**Colorear por** (panel del gráfico, presentación en puntos): elija una columna, y cada valor distinto recibe su propio color, repartido por todo el espectro; una leyenda abajo a la izquierda los nombra (casilla **Leyenda de colores** para ocultarla). Los colores siguen el orden de los valores ordenados, no el de las filas: ordenar o filtrar la tabla no los mezcla. Por encima de 20 valores, los 19 más frecuentes conservan un color y los demás son grises («Otros»); las celdas vacías son gris claro. La densidad sigue siendo una sola capa: allí no se ofrece la opción.

Al pasar el puntero sobre un punto se muestran su nombre, su color y su valor. Una vez seleccionado el mapa, un clic en un punto (sin arrastrar) selecciona su fila en la tabla, en su hoja, incluso desde un documento; si varios puntos se superponen, es el más cercano al puntero. El primer clic en un mapa no seleccionado solo lo selecciona.

## Copiar un gráfico {#copier-un-graphique}

Un gráfico copiado puede pegarse en otra hoja o en un [documento](onglets), donde sigue conectado a sus datos.
