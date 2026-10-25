package formula

// Traductions espagnoles de docs_added.go.
func docsAddedES() map[string]docText {
	x := ta("x", "Valor que se evalúa.")
	xPos := ta("x", "Valor que se evalúa, positivo.")
	cum := ta("acumulado", "{=TRUE}: probabilidad acumulada (función de distribución); {=FALSE}: densidad.")
	mean := ta("media", "Media de la distribución.")
	sd := ta("desv_estándar", "Desviación estándar de la distribución (positiva).")
	prob := ta("probabilidad", "Probabilidad, entre 0 y 1.")
	df := ta("grados_de_libertad", "Número de grados de libertad (truncado).")
	df1 := ta("grados_de_libertad1", "Grados de libertad del numerador.")
	df2 := ta("grados_de_libertad2", "Grados de libertad del denominador.")
	alpha := ta("alfa", "Parámetro de forma.")
	beta := ta("beta", "Parámetro de escala.")
	knownY := ta("conocido_y", "Columna de valores que se explican (Tabla[Columna]) o rango.")
	knownX := ta("conocido_x", "Columna de valores explicativos, de la misma tabla, o rango de la misma forma.")
	konst := ta("constante", "{=TRUE} u omitido: recta con ordenada en el origen; {=FALSE}: pasa por 0.")
	agg := ta("número1", "Columna entera (Tabla[Columna]), rango (Cell[A1:B3]), matriz dinámica o constante, expresión sobre las columnas de una tabla, celda o valor.")
	s1 := ta("matriz1", "Primera columna (Tabla[Columna]) o rango.")
	s2 := ta("matriz2", "Segunda columna, de la misma tabla, o rango de la misma forma.")
	settle := ta("liquidación", "Fecha de compra del valor.")
	mat := ta("vencimiento", "Fecha de vencimiento del valor.")
	freq := ta("frecuencia", "Cupones por año: 1, 2 o 4.")
	basis := ta("base", "Cómputo de días: 0 u omitido 30/360, 1 real/real, 2 real/360, 3 real/365, 4 30/360 europeo.")
	cost := ta("costo", "Costo de compra del bien.")
	salvage := ta("valor_residual", "Valor del bien al final de la amortización.")
	life := ta("vida", "Duración de la amortización, en períodos.")
	period := ta("período", "Número del período.")
	db := ta("base_de_datos", "Tabla entera, por su nombre (Ventas).")
	field := ta("nombre_de_campo", "Nombre de la columna entre comillas (\"Importe\") o su número.")
	crit := ta("criterios", "Rango de celdas (Cell[E1:F3]): encabezados de columna y luego una condición por fila.")
	amorDate := ta("fecha_compra", "Fecha de compra del bien.")
	amorFirst := ta("primer_período", "Fecha de fin del primer período.")
	amorPer := ta("período", "Número del período (0: el primero, prorrateado).")
	amorRate := ta("tasa", "Tasa de amortización.")
	amorBasis := ta("base", "Cómputo de días: 0 u omitido 30/360, 1 real/real, 3 real/365, 4 30/360 europeo.")
	coupon := ta("tasa", "Tasa anual del cupón.")
	redemption := ta("amortización", "Valor de reembolso por 100 de valor nominal.")
	dbText := func(desc, ex string) docText { return tdx(desc, ex, db, field, crit) }
	return map[string]docText{
		"NORM.DIST":       tdx("Devuelve la distribución normal para una media y una desviación estándar.", `=NORM.DIST(42, 40, 1.5, TRUE)`, x, mean, sd, cum),
		"NORM.INV":        tdx("Devuelve el valor de la distribución normal con la probabilidad acumulada indicada.", `=NORM.INV(0.95, 40, 1.5)`, prob, mean, sd),
		"NORM.S.DIST":     tdx("Devuelve la distribución normal estándar (media 0, desviación estándar 1).", `=NORM.S.DIST(1.96, TRUE)`, ta("z", "Valor que se evalúa."), cum),
		"NORM.S.INV":      tdx("Devuelve el valor de la distribución normal estándar con la probabilidad acumulada indicada.", `=NORM.S.INV(0.975)`, prob),
		"NORMSDIST":       tdx("Devuelve la distribución normal estándar acumulada (forma antigua).", `=NORMSDIST(1.96)`, ta("z", "Valor que se evalúa.")),
		"LOGNORM.DIST":    tdx("Devuelve la distribución lognormal (ln(x) sigue una distribución normal).", `=LOGNORM.DIST(4, 3.5, 1.2, TRUE)`, xPos, ta("media", "Media de ln(x)."), ta("desv_estándar", "Desviación estándar de ln(x)."), cum),
		"LOGNORM.INV":     tdx("Devuelve el valor de la distribución lognormal con la probabilidad acumulada indicada.", `=LOGNORM.INV(0.04, 3.5, 1.2)`, prob, ta("media", "Media de ln(x)."), ta("desv_estándar", "Desviación estándar de ln(x).")),
		"T.DIST":          tdx("Devuelve la distribución t de Student (cola izquierda).", `=T.DIST(1.5, 10, TRUE)`, x, df, cum),
		"T.DIST.2T":       tdx("Devuelve la probabilidad de dos colas de la distribución t de Student.", `=T.DIST.2T(1.96, 60)`, xPos, df),
		"T.DIST.RT":       tdx("Devuelve la probabilidad de la cola derecha de la distribución t de Student.", `=T.DIST.RT(1.96, 60)`, x, df),
		"T.INV":           tdx("Devuelve el valor de la distribución t con la probabilidad de cola izquierda indicada.", `=T.INV(0.975, 10)`, prob, df),
		"T.INV.2T":        tdx("Devuelve el valor de la distribución t con la probabilidad de dos colas indicada.", `=T.INV.2T(0.05, 10)`, prob, df),
		"CHISQ.DIST":      tdx("Devuelve la distribución chi cuadrado (cola izquierda).", `=CHISQ.DIST(3, 2, TRUE)`, xPos, df, cum),
		"CHISQ.DIST.RT":   tdx("Devuelve la probabilidad de la cola derecha de la distribución chi cuadrado.", `=CHISQ.DIST.RT(18.3, 10)`, xPos, df),
		"CHISQ.INV":       tdx("Devuelve el valor de la distribución chi cuadrado con la probabilidad de cola izquierda indicada.", `=CHISQ.INV(0.95, 10)`, prob, df),
		"CHISQ.INV.RT":    tdx("Devuelve el valor de la distribución chi cuadrado con la probabilidad de cola derecha indicada.", `=CHISQ.INV.RT(0.05, 10)`, prob, df),
		"F.DIST":          tdx("Devuelve la distribución F (cola izquierda).", `=F.DIST(15, 6, 4, TRUE)`, xPos, df1, df2, cum),
		"F.DIST.RT":       tdx("Devuelve la probabilidad de la cola derecha de la distribución F.", `=F.DIST.RT(15, 6, 4)`, xPos, df1, df2),
		"F.INV":           tdx("Devuelve el valor de la distribución F con la probabilidad de cola izquierda indicada.", `=F.INV(0.95, 6, 4)`, prob, df1, df2),
		"F.INV.RT":        tdx("Devuelve el valor de la distribución F con la probabilidad de cola derecha indicada.", `=F.INV.RT(0.05, 6, 4)`, prob, df1, df2),
		"BINOM.DIST":      tdx("Devuelve la probabilidad de un número de éxitos en un número de ensayos (distribución binomial).", `=BINOM.DIST(6, 10, 0.5, FALSE)`, ta("núm_éxito", "Número de éxitos."), ta("ensayos", "Número de ensayos independientes."), ta("prob_éxito", "Probabilidad de éxito en cada ensayo."), cum),
		"BINOM.INV":       tdx("Devuelve el menor número de éxitos cuya probabilidad acumulada alcanza un umbral.", `=BINOM.INV(6, 0.5, 0.75)`, ta("ensayos", "Número de ensayos."), ta("prob_éxito", "Probabilidad de un éxito."), ta("alfa", "Umbral de probabilidad acumulada.")),
		"NEGBINOM.DIST":   tdx("Devuelve la probabilidad de un número de fracasos antes de un número de éxitos (distribución binomial negativa).", `=NEGBINOM.DIST(10, 5, 0.25, TRUE)`, ta("núm_fracasos", "Número de fracasos."), ta("núm_éxitos", "Número de éxitos buscado."), ta("prob_éxito", "Probabilidad de un éxito."), cum),
		"HYPGEOM.DIST":    tdx("Devuelve la probabilidad de un número de éxitos en una muestra sin reemplazo (distribución hipergeométrica).", `=HYPGEOM.DIST(1, 4, 8, 20, TRUE)`, ta("muestra_éxito", "Número de éxitos en la muestra."), ta("núm_de_muestra", "Tamaño de la muestra."), ta("población_éxito", "Número de éxitos en la población."), ta("núm_de_población", "Tamaño de la población."), cum),
		"POISSON.DIST":    tdx("Devuelve la distribución de Poisson: probabilidad de un número de sucesos.", `=POISSON.DIST(2, 5, TRUE)`, ta("x", "Número de sucesos."), ta("media", "Número esperado de sucesos."), cum),
		"EXPON.DIST":      tdx("Devuelve la distribución exponencial (tiempo entre dos sucesos).", `=EXPON.DIST(0.2, 10, TRUE)`, xPos, ta("lambda", "Parámetro de la distribución (positivo)."), cum),
		"GAMMA.DIST":      tdx("Devuelve la distribución gamma.", `=GAMMA.DIST(10, 9, 2, TRUE)`, xPos, alpha, beta, cum),
		"GAMMA.INV":       tdx("Devuelve el valor de la distribución gamma con la probabilidad acumulada indicada.", `=GAMMA.INV(0.07, 9, 2)`, prob, alpha, beta),
		"GAMMALN":         tdx("Devuelve el logaritmo natural de la función gamma.", `=GAMMALN(4)`, ta("x", "Número positivo.")),
		"GAMMA":           tdx("Devuelve el valor de la función gamma.", `=GAMMA(2.5)`, ta("número", "Número (ni 0 ni un entero negativo).")),
		"BETA.DIST":       tdx("Devuelve la distribución beta, entre 0 y 1 o entre dos límites.", `=BETA.DIST(2, 8, 10, TRUE, 1, 3)`, ta("x", "Valor que se evalúa, entre A y B."), ta("alfa", "Primer parámetro de forma."), ta("beta", "Segundo parámetro de forma."), cum, ta("A", "Límite inferior (0 si se omite)."), ta("B", "Límite superior (1 si se omite).")),
		"BETA.INV":        tdx("Devuelve el valor de la distribución beta con la probabilidad acumulada indicada.", `=BETA.INV(0.69, 8, 10, 1, 3)`, prob, ta("alfa", "Primer parámetro de forma."), ta("beta", "Segundo parámetro de forma."), ta("A", "Límite inferior (0 si se omite)."), ta("B", "Límite superior (1 si se omite).")),
		"WEIBULL.DIST":    tdx("Devuelve la distribución de Weibull (duraciones de vida, averías).", `=WEIBULL.DIST(105, 20, 100, TRUE)`, xPos, alpha, beta, cum),
		"CONFIDENCE.NORM": tdx("Devuelve la mitad del ancho del intervalo de confianza de una media (distribución normal).", `=CONFIDENCE.NORM(0.05, 2.5, 50)`, ta("alfa", "Riesgo: 0,05 para un intervalo del 95 %."), ta("desv_estándar", "Desviación estándar de la población."), ta("tamaño", "Tamaño de la muestra.")),
		"CONFIDENCE.T":    tdx("Devuelve la mitad del ancho del intervalo de confianza de una media (distribución t de Student).", `=CONFIDENCE.T(0.05, 1, 50)`, ta("alfa", "Riesgo: 0,05 para un intervalo del 95 %."), ta("desv_estándar", "Desviación estándar de la muestra."), ta("tamaño", "Tamaño de la muestra.")),
		"STANDARDIZE":     tdx("Devuelve el valor normalizado: (x − media) / desviación estándar.", `=STANDARDIZE([Nota], 12, 3)`, ta("x", "Valor que se normaliza."), mean, sd),
		"PHI":             tdx("Devuelve la densidad de la distribución normal estándar.", `=PHI(0.75)`, x),
		"GAUSS":           tdx("Devuelve la probabilidad de que un valor normal estándar caiga entre 0 y z.", `=GAUSS(2)`, ta("z", "Valor que se evalúa.")),
		"FISHER":          tdx("Devuelve la transformación de Fisher de un coeficiente de correlación.", `=FISHER(0.75)`, ta("x", "Valor estrictamente entre −1 y 1.")),
		"FISHERINV":       tdx("Devuelve la inversa de la transformación de Fisher.", `=FISHERINV(0.97)`, ta("y", "Valor transformado.")),

		"KURT":            tdx("Devuelve la curtosis de una serie: al menos 4 valores.", `=KURT(Ventas[Importe])`, agg),
		"SKEW":            tdx("Devuelve la asimetría de una serie (muestra): al menos 3 valores.", `=SKEW(Ventas[Importe])`, agg),
		"SKEW.P":          tdx("Devuelve la asimetría de una población: al menos 3 valores.", `=SKEW.P(Ventas[Importe])`, agg),
		"AVEDEV":          tdx("Devuelve el promedio de las desviaciones absolutas respecto de la media.", `=AVEDEV(Ventas[Importe])`, agg),
		"DEVSQ":           tdx("Devuelve la suma de los cuadrados de las desviaciones respecto de la media.", `=DEVSQ(Ventas[Importe])`, agg),
		"GEOMEAN":         tdx("Devuelve la media geométrica de números positivos.", `=GEOMEAN(Tasas[Factor])`, agg),
		"HARMEAN":         tdx("Devuelve la media armónica de números positivos.", `=HARMEAN(Viajes[Velocidad])`, agg),
		"COVARIANCE.S":    tdx("Devuelve la covarianza de muestra de dos columnas.", `=COVARIANCE.S(Ventas[Precio], Ventas[Cant])`, s1, s2),
		"COVARIANCE.P":    tdx("Devuelve la covarianza de población de dos columnas.", `=COVARIANCE.P(Ventas[Precio], Ventas[Cant])`, s1, s2),
		"RSQ":             tdx("Devuelve el coeficiente de determinación R² de la regresión lineal.", `=RSQ(Ventas[Importe], Ventas[Cant])`, knownY, knownX),
		"STEYX":           tdx("Devuelve el error típico de los valores previstos por la regresión lineal.", `=STEYX(Ventas[Importe], Ventas[Cant])`, knownY, knownX),
		"FORECAST.LINEAR": tdx("Pronostica un valor por regresión lineal.", `=FORECAST.LINEAR(30, Ventas[Importe], Ventas[Cant])`, ta("x", "Valor para el que se pronostica."), knownY, knownX),
		"TREND":           tdx("Devuelve el valor de la recta de regresión para un nuevo x.", `=TREND(Ventas[Importe], Ventas[Cant], [Cant])`, knownY, knownX, ta("nueva_x", "Valor para el que se pronostica, a menudo una columna de la fila ([Cant])."), konst),
		"GROWTH":          tdx("Devuelve el valor de la curva exponencial ajustada para un nuevo x.", `=GROWTH(Ventas[Importe], Ventas[Cant], 10)`, knownY, knownX, ta("nueva_x", "Valor para el que se pronostica."), ta("constante", "{=TRUE} u omitido: cálculo normal; {=FALSE}: curva que vale 1 en 0.")),
		"LINEST":          tdx("Devuelve la pendiente y la ordenada de la recta de regresión, y sus estadísticas; el resultado se desborda.", `=LINEST(Ventas[Importe], Ventas[Cant])`, knownY, knownX, konst, ta("estadística", "{=TRUE}: 5 filas de estadísticas; {=FALSE} u omitido: una fila (escrito en la fórmula).")),
		"LOGEST":          tdx("Devuelve los coeficientes de la curva exponencial ajustada, y sus estadísticas; el resultado se desborda.", `=LOGEST(Ventas[Importe], Ventas[Cant])`, knownY, knownX, ta("constante", "{=TRUE} u omitido: cálculo normal; {=FALSE}: coeficiente b igual a 1."), ta("estadística", "{=TRUE}: 5 filas de estadísticas; {=FALSE} u omitido: una fila (escrito en la fórmula).")),
		"T.TEST":          tdx("Devuelve la probabilidad de la prueba t de Student sobre dos series.", `=T.TEST(Pruebas[Antes], Pruebas[Después], 2, 1)`, s1, s2, ta("colas", "1: prueba de una cola; 2: de dos colas."), ta("tipo", "1: series emparejadas; 2: varianzas iguales; 3: varianzas distintas.")),
		"F.TEST":          tdx("Devuelve la probabilidad de dos colas de que dos series tengan la misma varianza.", `=F.TEST(Pruebas[Antes], Pruebas[Después])`, s1, s2),
		"Z.TEST":          tdx("Devuelve la probabilidad de una cola de la prueba z (media mayor que x).", `=Z.TEST(Pruebas[Medida], 4)`, ta("matriz", "Columna de valores (Tabla[Columna]) o rango."), ta("x", "Valor que se prueba."), ta("sigma", "Desviación estándar de la población (la de la muestra si se omite).")),
		"CHISQ.TEST":      tdx("Devuelve la probabilidad de la prueba de independencia chi cuadrado.", `=CHISQ.TEST(Cell[B2:C4], Cell[E2:F4])`, ta("rango_real", "Valores observados: columna o rango."), ta("rango_esperado", "Valores esperados, de la misma forma.")),
		"RANK.AVG":        tdx("Devuelve la jerarquía de un valor en una columna; los empates reciben su jerarquía media.", `=RANK.AVG([Puntos], Jugadores[Puntos])`, ta("número", "Valor que se clasifica."), ta("referencia", "Columna en la que se clasifica el valor (Tabla[Columna])."), ta("orden", "0 u omitido: descendente; 1: ascendente.")),
		"AGGREGATE": tdx("Calcula un agregado omitiendo, a elección, los errores o las filas ocultas por el filtro.", `=AGGREGATE(9, 6, Ventas[Importe])`,
			ta("núm_función", "1 {fn:AVERAGE}, 2 {fn:COUNT}, 3 {fn:COUNTA}, 4 {fn:MAX}, 5 {fn:MIN}, 6 {fn:PRODUCT}, 7 {fn:STDEV.S}, 8 {fn:STDEV.P}, 9 {fn:SUM}, 10 {fn:VAR.S}, 11 {fn:VAR.P}, 12 {fn:MEDIAN}, 13 {fn:MODE.SNGL}, 14 {fn:LARGE}, 15 {fn:SMALL}, 16 {fn:PERCENTILE.INC}, 17 {fn:QUARTILE.INC}, 18 {fn:PERCENTILE.EXC}, 19 {fn:QUARTILE.EXC}."),
			ta("opciones", "0 a 7: 6 omite los errores, 5 las filas ocultas, 7 ambos (0: nada)."),
			ta("ref1", "Columna (Tabla[Columna]), rango o expresión; para 14 a 19, seguida de k.")),

		"XNPV":       tdx("Devuelve el valor actual neto de flujos de caja fechados.", `=XNPV(8%, Flujos[Importe], Flujos[Fecha])`, ta("tasa", "Tasa de descuento anual."), ta("valores", "Columna de los flujos (Tabla[Columna])."), ta("fechas", "Columna de las fechas de los flujos, de la misma tabla; la primera es el origen.")),
		"XIRR":       tdx("Devuelve la tasa interna de retorno de flujos de caja fechados.", `=XIRR(Flujos[Importe], Flujos[Fecha])`, ta("valores", "Columna de los flujos, con al menos uno positivo y uno negativo."), ta("fechas", "Columna de las fechas de los flujos, de la misma tabla."), ta("estimar", "Tasa de partida del cálculo (10 % si se omite).")),
		"EFFECT":     tdx("Devuelve la tasa anual efectiva de una tasa nominal capitalizada varias veces al año.", `=EFFECT(5%, 12)`, ta("int_nominal", "Tasa nominal anual."), ta("núm_por_año", "Número de períodos de capitalización por año.")),
		"NOMINAL":    tdx("Devuelve la tasa anual nominal de una tasa efectiva.", `=NOMINAL(5.12%, 12)`, ta("tasa_efectiva", "Tasa efectiva anual."), ta("núm_por_año", "Número de períodos de capitalización por año.")),
		"SLN":        tdx("Devuelve la amortización lineal de un bien para un período.", `=SLN(30000, 7500, 10)`, cost, salvage, life),
		"SYD":        tdx("Devuelve la amortización por suma de dígitos de los años de un bien para un período.", `=SYD(30000, 7500, 10, 1)`, cost, salvage, life, ta("período", "Número del período, de 1 a vida.")),
		"DB":         tdx("Devuelve la amortización de saldo fijo decreciente de un bien para un período.", `=DB(1000000, 100000, 6, 1, 7)`, cost, salvage, life, period, ta("mes", "Meses del primer año (12 si se omite).")),
		"DDB":        tdx("Devuelve la amortización de saldo doblemente decreciente (u otro factor) de un bien para un período.", `=DDB(2400, 300, 10, 1)`, cost, salvage, life, period, ta("factor", "Factor de disminución (2 si se omite).")),
		"VDB":        tdx("Devuelve la amortización decreciente de un bien entre dos períodos, con paso a la lineal.", `=VDB(2400, 300, 10, 0, 1)`, cost, salvage, life, ta("período_inicial", "Inicio del período (0: compra)."), ta("período_final", "Fin del período."), ta("factor", "Factor de disminución (2 si se omite)."), ta("sin_cambios", "{=TRUE}: nunca pasa a la lineal.")),
		"AMORLINC":   tdx("Devuelve la amortización lineal de un período según el sistema contable francés (primer período prorrateado).", `=AMORLINC(2400, DATE(2026,8,19), DATE(2026,12,31), 300, 1, 15%, 1)`, cost, amorDate, amorFirst, salvage, amorPer, amorRate, amorBasis),
		"AMORDEGRC":  tdx("Devuelve la amortización decreciente de un período según el sistema contable francés.", `=AMORDEGRC(2400, DATE(2026,8,19), DATE(2026,12,31), 300, 1, 15%, 1)`, cost, amorDate, amorFirst, salvage, amorPer, amorRate, amorBasis),
		"PRICE":      tdx("Devuelve el precio de un valor con cupones por 100 de valor nominal.", `=PRICE(DATE(2026,2,15), DATE(2035,11,15), 5.75%, 6.5%, 100, 2, 0)`, settle, mat, coupon, ta("rendimiento", "Rendimiento anual del valor."), redemption, freq, basis),
		"YIELD":      tdx("Devuelve el rendimiento de un valor con cupones a partir de su precio.", `=YIELD(DATE(2026,2,15), DATE(2034,11,15), 5.75%, 95.04, 100, 2, 0)`, settle, mat, coupon, ta("precio", "Precio por 100 de valor nominal."), redemption, freq, basis),
		"ACCRINT":    tdx("Devuelve el interés acumulado de un valor con cupones.", `=ACCRINT(DATE(2026,3,1), DATE(2026,8,31), DATE(2026,5,1), 10%, 1000, 2, 0)`, ta("emisión", "Fecha de emisión del valor."), ta("primer_interés", "Fecha del primer cupón."), settle, coupon, ta("valor_nominal", "Valor nominal del valor."), freq, basis, ta("método_cálculo", "{=TRUE} u omitido: desde la emisión; {=FALSE}: desde el primer cupón.")),
		"COUPNUM":    tdx("Devuelve el número de cupones pendientes entre la liquidación y el vencimiento.", `=COUPNUM(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPPCD":    tdx("Devuelve la fecha del cupón anterior a la liquidación.", `=COUPPCD(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPNCD":    tdx("Devuelve la fecha del cupón siguiente a la liquidación.", `=COUPNCD(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYS":   tdx("Devuelve el número de días del período de cupón que contiene la liquidación.", `=COUPDAYS(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYBS":  tdx("Devuelve el número de días desde el cupón anterior hasta la liquidación.", `=COUPDAYBS(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),
		"COUPDAYSNC": tdx("Devuelve el número de días desde la liquidación hasta el cupón siguiente.", `=COUPDAYSNC(DATE(2026,1,25), DATE(2026,11,15), 2, 1)`, settle, mat, freq, basis),

		"DSUM":     dbText("Suma los valores de una columna para las filas que cumplen los criterios.", `=DSUM(Ventas, "Importe", Cell[E1:F3])`),
		"DAVERAGE": dbText("Devuelve el promedio de una columna para las filas que cumplen los criterios.", `=DAVERAGE(Ventas, "Importe", Cell[E1:F3])`),
		"DCOUNT":   dbText("Cuenta los números de una columna para las filas que cumplen los criterios.", `=DCOUNT(Ventas, "Importe", Cell[E1:F3])`),
		"DCOUNTA":  dbText("Cuenta los valores no vacíos de una columna para las filas que cumplen los criterios.", `=DCOUNTA(Ventas, "Cliente", Cell[E1:F3])`),
		"DMAX":     dbText("Devuelve el mayor valor de una columna para las filas que cumplen los criterios.", `=DMAX(Ventas, "Importe", Cell[E1:F3])`),
		"DMIN":     dbText("Devuelve el menor valor de una columna para las filas que cumplen los criterios.", `=DMIN(Ventas, "Importe", Cell[E1:F3])`),
		"DPRODUCT": dbText("Multiplica los valores de una columna para las filas que cumplen los criterios.", `=DPRODUCT(Tasas, "Factor", Cell[E1:E2])`),
		"DSTDEV":   dbText("Devuelve la desviación estándar de muestra de una columna para las filas que cumplen los criterios.", `=DSTDEV(Ventas, "Importe", Cell[E1:F3])`),
		"DSTDEVP":  dbText("Devuelve la desviación estándar de población de una columna para las filas que cumplen los criterios.", `=DSTDEVP(Ventas, "Importe", Cell[E1:F3])`),
		"DVAR":     dbText("Devuelve la varianza de muestra de una columna para las filas que cumplen los criterios.", `=DVAR(Ventas, "Importe", Cell[E1:F3])`),
		"DVARP":    dbText("Devuelve la varianza de población de una columna para las filas que cumplen los criterios.", `=DVARP(Ventas, "Importe", Cell[E1:F3])`),
		"DGET":     dbText("Devuelve el valor de una columna de la única fila que cumple los criterios.", `=DGET(Clientes, "Ciudad", Cell[E1:E2])`),

		"OFFSET": tdx("Devuelve una referencia desplazada un número de filas y columnas, del tamaño deseado.", `=SUM(OFFSET(Cell[B2], 0, 0, Cell[A1], 1))`,
			ta("ref", "Celda de partida (Cell[A1]), rango, columna (Tabla[Columna]) o fila (Tabla[Columna][2])."),
			ta("filas", "Desplazamiento hacia abajo (negativo: hacia arriba)."), ta("columnas", "Desplazamiento a la derecha (negativo: a la izquierda)."),
			ta("alto", "Número de filas del resultado (el de la referencia si se omite)."),
			ta("ancho", "Número de columnas del resultado (el de la referencia si se omite).")),
		"INDIRECT": tdx("Devuelve la referencia escrita en un texto: \"B3\", \"Hoja2!B3\", \"Cell[A1:A9]\", \"Ventas[Precio]\".", `=INDIRECT("Hoja2!B"&Cell[A1])`,
			ta("ref", "Texto de la referencia; una dirección como B3 designa una celda libre."),
			ta("a1", "{=TRUE} u omitido; {=FALSE} (estilo F1C1) no se admite.")),
		"CELL": tdx("Devuelve información sobre una celda: \"address\", \"row\", \"col\", \"contents\" o \"type\" (b vacía, l texto, v valor).", `=CELL("address", Cell[B3])`,
			ta("tipo_de_info", "Información deseada, entre comillas."), ta("referencia", "Celda, rango (su primera celda) o referencia de tabla.")),
		"TRANSPOSE": tdx("Intercambia las filas y las columnas de un rango de celdas o de una constante; el resultado se desborda.", `=TRANSPOSE(Cell[A1:C2])`,
			ta("matriz", "Rango de celdas (Cell[A1:C2]) o constante ({1;2;3}).")),
	}
}
