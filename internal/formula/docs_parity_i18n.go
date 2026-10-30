package formula

// Traductions de docs_parity.go.
var docsParityEN = map[string]docText{
	"XMATCH": tdx("Returns the position of a value in a column, with an exact, approximate or wildcard match, from the first or the last.", `=XMATCH(250, Brackets[Threshold], -1)`,
		ta("lookup_value", "Value to find."),
		ta("lookup_array", "Column to search in (Table[Column]) or range (Cell[A1:A10])."),
		ta("match_mode", "0: exact (default); -1: exact or next smaller; 1: exact or next larger; 2: wildcards * ?."),
		ta("search_mode", "1: from the first (default); -1: from the last; 2 and -2: like 1.")),
	"HLOOKUP": tdx("Looks up a value in the first row of a cell range and returns the one from another row; converted to {fn:INDEX} and {fn:MATCH} on entry.", `=HLOOKUP("T2", Cell[B2:F4], 3, FALSE)`,
		ta("lookup_value", "Value to find in the first row."),
		ta("table_array", "Range of free cells (B2:F4); not a table."),
		ta("row_index_num", "Number of the row to return within the range (1 = the first)."),
		ta("range_lookup", "{=FALSE} or 0: exact match; {=TRUE} or omitted: largest value less than or equal.")),
}

var docsParityDE = map[string]docText{
	"XMATCH": tdx("Gibt die Position eines Werts in einer Spalte zurück, mit genauer, ungefährer oder Platzhalter-Übereinstimmung, vom ersten oder letzten an.", `=XMATCH(250, Staffel[Schwelle], -1)`,
		ta("Suchkriterium", "Zu findender Wert."),
		ta("Suchmatrix", "Spalte, in der gesucht wird (Tabelle[Spalte]), oder Bereich (Cell[A1:A10])."),
		ta("Vergleichsmodus", "0: genau (Standard); -1: genau oder nächstkleiner; 1: genau oder nächstgrößer; 2: Platzhalter * ?."),
		ta("Suchmodus", "1: vom ersten an (Standard); -1: vom letzten an; 2 und -2: wie 1.")),
	"HLOOKUP": tdx("Sucht einen Wert in der ersten Zeile eines Zellbereichs und gibt den Wert einer anderen Zeile zurück; wird bei der Eingabe in {fn:INDEX} und {fn:MATCH} umgewandelt.", `=HLOOKUP("T2", Cell[B2:F4], 3, FALSE)`,
		ta("Suchkriterium", "In der ersten Zeile zu findender Wert."),
		ta("Matrix", "Bereich freier Zellen (B2:F4); keine Tabelle."),
		ta("Zeilenindex", "Nummer der zurückzugebenden Zeile im Bereich (1 = die erste)."),
		ta("Bereich_Verweis", "{=FALSE} oder 0: genaue Übereinstimmung; {=TRUE} oder weggelassen: größter Wert kleiner oder gleich.")),
}

var docsParityES = map[string]docText{
	"XMATCH": tdx("Devuelve la posición de un valor en una columna, con coincidencia exacta, aproximada o con comodines, desde el primero o el último.", `=XMATCH(250, Baremos[Umbral], -1)`,
		ta("valor_buscado", "Valor que se busca."),
		ta("matriz_buscada", "Columna en la que se busca (Tabla[Columna]) o rango (Cell[A1:A10])."),
		ta("modo_de_coincidencia", "0: exacta (predeterminado); -1: exacta o inferior; 1: exacta o superior; 2: comodines * ?."),
		ta("modo_de_búsqueda", "1: desde el primero (predeterminado); -1: desde el último; 2 y -2: como 1.")),
	"HLOOKUP": tdx("Busca un valor en la primera fila de un rango de celdas y devuelve el de otra fila; se convierte en {fn:INDEX} y {fn:MATCH} al escribirla.", `=HLOOKUP("T2", Cell[B2:F4], 3, FALSE)`,
		ta("valor_buscado", "Valor que se busca en la primera fila."),
		ta("matriz_buscar_en", "Rango de celdas libres (B2:F4); no una tabla."),
		ta("indicador_filas", "Número de la fila que se devuelve dentro del rango (1 = la primera)."),
		ta("ordenado", "{=FALSE} o 0: coincidencia exacta; {=TRUE} u omitido: el mayor valor menor o igual.")),
}

var docsParityZH = map[string]docText{
	"XMATCH": tdx("返回值在列中的位置，支持精确、近似或通配符匹配，可从第一项或最后一项开始。", `=XMATCH(250, 档位[门槛], -1)`,
		ta("查找值", "要查找的值。"),
		ta("查找数组", "要在其中查找的列（表[列]）或区域（Cell[A1:A10]）。"),
		ta("匹配模式", "0：精确（默认）；-1：精确或下一个较小值；1：精确或下一个较大值；2：通配符 * ?。"),
		ta("搜索模式", "1：从第一项开始（默认）；-1：从最后一项开始；2 和 -2：同 1。")),
	"HLOOKUP": tdx("在单元格区域的第一行中查找值并返回另一行的值；输入时转换为 {fn:INDEX} 与 {fn:MATCH}。", `=HLOOKUP("T2", Cell[B2:F4], 3, FALSE)`,
		ta("查找值", "要在第一行中查找的值。"),
		ta("数据表", "自由单元格区域（B2:F4）；不能是表。"),
		ta("行序数", "区域中要返回的行的序号（1 为第一行）。"),
		ta("匹配条件", "{=FALSE} 或 0：精确匹配；{=TRUE} 或省略：小于或等于的最大值。")),
}

func init() {
	for _, l := range []struct {
		set  *docSet
		docs map[string]docText
	}{{&docsEN, docsParityEN}, {&docsDE, docsParityDE}, {&docsES, docsParityES}, {&docsZH, docsParityZH}} {
		for k, v := range l.docs {
			l.set.funcs[k] = v
		}
	}
}
