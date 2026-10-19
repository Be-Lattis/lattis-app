package transfer

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// Traduction des formules de Lattis pour l'export : en .xlsx, les
// références de tableau deviennent des références structurées d'Excel
// (Ventes[Prix], Ventes[[#This Row],[Prix]]) ; en .ods, des adresses
// ([.B2], [$Feuille1.$B$2:.$B$9]). Les cellules libres reprennent leur
// adresse. REFERENCE devient INDEX/EQUIV sur la colonne clé. Ce qui n'a pas
// d'équivalent (calculs entre lignes, tableaux dynamiques, REQUETE…) n'est
// pas traduit : la cellule garde sa valeur calculée.

type target int

const (
	toXLSX target = iota + 1
	toODS
)

// tablePlace situe un tableau dans le classeur exporté.
type tablePlace struct {
	name     string // nom exporté (valide pour Excel)
	sheet    string // nom exporté de sa feuille
	row, col int    // en-tête
	cols     []string
	key      int // rang de la colonne clé, -1 sans clé
	total    int // lignes de données
}

// colIndex donne le rang d'une colonne (sans tenir compte de la casse).
func (t *tablePlace) colIndex(name string) int {
	for i, c := range t.cols {
		if strings.EqualFold(c, name) {
			return i
		}
	}
	return -1
}

// lastRow est la dernière ligne de données (au moins une ligne, vide).
func (t *tablePlace) lastRow() int { return t.row + max(t.total, 1) }

type cellPlace struct {
	sheet    string // nom exporté
	row, col int
}

// book décrit le classeur pour la traduction des formules.
type book struct {
	target target
	sheets map[string]string // nom de feuille de Lattis (minuscules) → nom exporté
	tables map[string]*tablePlace
	names  map[string]cellPlace // cellules nommées (minuscules)
}

// at est la cellule d'une formule.
type at struct {
	sheet string      // nom exporté
	table *tablePlace // colonne calculée de ce tableau, sinon nil
}

// rowMark tient la place du numéro de ligne d'une colonne calculée en .ods :
// la formule est traduite une fois, puis le numéro remplace la marque.
const rowMark = "\x00"

// translate traduit une formule de Lattis (français, avec =) ; faux si elle
// ne se traduit pas.
func (b *book) translate(src string, a at) (string, bool) {
	s, _, ok := b.translateWhy(src, a)
	return s, ok
}

// translateWhy traduit une formule ; quand elle ne se traduit pas, why
// nomme (en français) les fonctions sans équivalent, s'il y en a.
func (b *book) translateWhy(src string, a at) (s string, why []string, ok bool) {
	n, err := formula.Parse(src, formula.FR)
	if err != nil {
		return "", nil, false
	}
	tr := translator{b: b, at: a, ok: true}
	out := tr.walk(n)
	if !tr.ok {
		return "", tr.why, false
	}
	s = formula.Format(out, formula.EN)[1:]
	return b.separators(s), nil, true
}

// separators réécrit les séparateurs d'arguments de formula.Format (« , »)
// hors des textes et des références : « , » en .xlsx, « ; » en .ods.
func (b *book) separators(s string) string {
	sep := ","
	if b.target == toODS {
		sep = ";"
	}
	var out strings.Builder
	inStr, depth := false, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inStr:
			if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr = true
		case c == '[':
			depth++
		case c == ']':
			depth--
		case c == '\'' && depth == 0:
			// nom de feuille entre apostrophes ('a,b'!A1)
			j := i + 1
			for j < len(s) {
				if s[j] == '\'' {
					if j+1 < len(s) && s[j+1] == '\'' {
						j += 2
						continue
					}
					break
				}
				j++
			}
			out.WriteString(s[i:min(j+1, len(s))])
			i = j
			continue
		case c == ',' && depth == 0:
			out.WriteString(sep)
			if i+1 < len(s) && s[i+1] == ' ' {
				i++
			}
			continue
		}
		out.WriteByte(c)
	}
	return out.String()
}

type translator struct {
	b   *book
	at  at
	ok  bool
	why []string // fonctions sans équivalent (noms français)
}

// raw est un morceau de formule déjà écrit (formula.Format écrit le texte
// d'une Range tel quel).
func raw(s string) formula.Node { return formula.Range{Text: s} }

func (tr *translator) fail() formula.Node {
	tr.ok = false
	return raw("")
}

// failFn : la fonction fn n'a pas d'équivalent.
func (tr *translator) failFn(fn string) formula.Node {
	tr.why = append(tr.why, formula.LocalName(fn, formula.FR))
	return tr.fail()
}

// Fonctions de même nom et de même sens dans Excel et LibreOffice ; la valeur
// est le nom à écrire en .xlsx et en .ods ("" : pas d'équivalent).
var exportFuncs = map[string][2]string{}

func init() {
	for _, f := range strings.Fields(`IF AND OR NOT ISERROR ISNA ISBLANK ISNUMBER ISTEXT ISLOGICAL CHOOSE
		ROUND ROUNDUP ROUNDDOWN INT TRUNC ABS SIGN MOD POWER SQRT EXP LN LOG10 PI RAND RANDBETWEEN
		CONCATENATE LEN UPPER LOWER PROPER TRIM LEFT RIGHT MID SUBSTITUTE REPLACE FIND SEARCH VALUE REPT EXACT
		TODAY NOW DATE YEAR MONTH DAY HOUR MINUTE SECOND WEEKDAY WEEKNUM TIME EOMONTH EDATE DATEDIF NETWORKDAYS WORKDAY
		SUM AVERAGE COUNT COUNTA COUNTBLANK MIN MAX MEDIAN LARGE SMALL STDEV PRODUCT SUBTOTAL
		SUMIF COUNTIF AVERAGEIF SUMIFS COUNTIFS AVERAGEIFS SUMPRODUCT MATCH INDEX VLOOKUP HLOOKUP
		VAR STDEVP MODE PERCENTILE QUARTILE CORREL SLOPE INTERCEPT
		PMT PV FV NPER RATE IPMT PPMT NPV IRR
		LOG SIN COS TAN ASIN ACOS ATAN RADIANS DEGREES MROUND QUOTIENT ISEVEN ISODD`) {
		exportFuncs[f] = [2]string{f, f}
	}
	// Fonctions récentes d'Excel : préfixe _xlfn. dans le fichier ; LibreOffice
	// les connaît sous le nom COM.MICROSOFT.* (ou sous leur nom OpenFormula).
	for f, ods := range map[string]string{
		"IFS": "COM.MICROSOFT.IFS", "SWITCH": "COM.MICROSOFT.SWITCH", "IFNA": "IFNA", "XOR": "XOR",
		"CONCAT": "COM.MICROSOFT.CONCAT", "TEXTJOIN": "COM.MICROSOFT.TEXTJOIN",
		"MAXIFS": "COM.MICROSOFT.MAXIFS", "MINIFS": "COM.MICROSOFT.MINIFS",
		"DAYS": "DAYS", "ISOWEEKNUM": "ISOWEEKNUM", "XLOOKUP": "",
	} {
		exportFuncs[f] = [2]string{"_xlfn." + f, ods}
	}
	// Fonctions d'Excel 2010 (_xlfn.) que LibreOffice nomme COM.MICROSOFT.*.
	for _, f := range strings.Fields(`STDEV.S STDEV.P VAR.S VAR.P MODE.SNGL
		PERCENTILE.INC PERCENTILE.EXC QUARTILE.INC QUARTILE.EXC`) {
		exportFuncs[f] = [2]string{"_xlfn." + f, "COM.MICROSOFT." + f}
	}
}

func (tr *translator) walk(n formula.Node) formula.Node {
	switch v := n.(type) {
	case nil:
		return nil
	case formula.NumLit, formula.StrLit:
		return v
	case formula.BoolLit:
		s := "FALSE"
		if v.V {
			s = "TRUE"
		}
		if tr.b.target == toODS {
			s += "()"
		}
		return raw(s)
	case formula.ErrLit:
		e, ok := excelErrors[v.V.String()]
		if !ok || v.V.String() == "#CIRC!" {
			return tr.fail()
		}
		return raw(e)
	case formula.Unary:
		v.X = tr.walk(v.X)
		return v
	case formula.Binary:
		v.L, v.R = tr.walk(v.L), tr.walk(v.R)
		return v
	case formula.Ref:
		return tr.ref(v)
	case formula.Name:
		return tr.name(v.Name)
	case formula.CellRange:
		sheet, ok := tr.sheet(v.Sheet)
		if !ok {
			return tr.fail()
		}
		return tr.cells(sheet, v.R1, v.C1, v.R2, v.C2, tr.at.table != nil)
	case formula.RowRange:
		t := tr.table(v.Ref.Table)
		if t == nil {
			return tr.fail()
		}
		c := t.colIndex(v.Ref.Col)
		if c < 0 || v.To > t.total {
			return tr.fail()
		}
		return tr.cells(t.sheet, t.row+v.From, t.col+c, t.row+v.To, t.col+c, true)
	case formula.Call:
		return tr.call(v)
	}
	return tr.fail()
}

// table donne le tableau nommé ("" : celui de la formule).
func (tr *translator) table(name string) *tablePlace {
	if name == "" {
		return tr.at.table
	}
	return tr.b.tables[strings.ToLower(name)]
}

// sheet donne le nom exporté d'une feuille de Lattis ("" : celle de la formule).
func (tr *translator) sheet(name string) (string, bool) {
	if name == "" {
		return tr.at.sheet, true
	}
	s, ok := tr.b.sheets[strings.ToLower(name)]
	return s, ok
}

var simpleCol = regexp.MustCompile(`^[\p{L}\p{N}_ ]+$`)

// structCol écrit un nom de colonne dans une référence structurée d'Excel.
func structCol(c string) string {
	return strings.NewReplacer("'", "''", "[", "'[", "]", "']", "#", "'#").Replace(c)
}

// column écrit une colonne entière de données.
func (tr *translator) column(t *tablePlace, c int) formula.Node {
	if tr.b.target == toXLSX {
		name := structCol(t.cols[c])
		if simpleCol.MatchString(t.cols[c]) && strings.TrimSpace(t.cols[c]) == t.cols[c] {
			return raw(t.name + "[" + name + "]")
		}
		return raw(t.name + "[[" + name + "]]")
	}
	return tr.cells(t.sheet, t.row+1, t.col+c, t.lastRow(), t.col+c, true)
}

// body écrit les données d'un tableau (toutes ses colonnes).
func (tr *translator) body(t *tablePlace) formula.Node {
	if tr.b.target == toXLSX {
		return raw(t.name)
	}
	return tr.cells(t.sheet, t.row+1, t.col, t.lastRow(), t.col+len(t.cols)-1, true)
}

func (tr *translator) ref(r formula.Ref) formula.Node {
	t := tr.table(r.Table)
	if t == nil {
		return tr.fail()
	}
	c := t.colIndex(r.Col)
	if c < 0 {
		return tr.fail()
	}
	if !r.Row {
		return tr.column(t, c)
	}
	// Ligne courante : seulement dans une colonne calculée du même tableau.
	if t != tr.at.table {
		return tr.fail()
	}
	if tr.b.target == toXLSX {
		return raw(t.name + "[[#This Row],[" + structCol(t.cols[c]) + "]]")
	}
	return raw("[." + colName(t.col+c) + rowMark + "]")
}

// name traduit une cellule nommée ou une adresse Cell_A1 (Feuille.Cell_A1).
func (tr *translator) name(n string) formula.Node {
	if p, ok := tr.b.names[strings.ToLower(n)]; ok {
		return tr.cells(p.sheet, p.row, p.col, p.row, p.col, true)
	}
	if t := tr.b.tables[strings.ToLower(n)]; t != nil {
		return tr.body(t)
	}
	sheet, addr := "", n
	if k := strings.LastIndex(n, "."); k >= 0 {
		sheet, addr = n[:k], n[k+1:]
	}
	r, c, ok := formula.CellRef(addr)
	if !ok {
		return tr.fail()
	}
	s, ok := tr.sheet(sheet)
	if !ok {
		return tr.fail()
	}
	return tr.cells(s, r, c, r, c, tr.at.table != nil)
}

// cells écrit une adresse ou une plage (abs : avec des $).
func (tr *translator) cells(sheet string, r1, c1, r2, c2 int, abs bool) formula.Node {
	if tr.b.target == toXLSX && (r2 >= ExcelRows || c2 >= ExcelCols) {
		return tr.fail()
	}
	d := ""
	if abs {
		d = "$"
	}
	a := func(r, c int) string { return d + colName(c) + d + strconv.Itoa(r+1) }
	quoted := "'" + strings.ReplaceAll(sheet, "'", "''") + "'"
	if tr.b.target == toODS {
		s := "["
		if sheet != tr.at.sheet {
			s += "$" + quoted
		}
		s += "." + a(r1, c1)
		if r1 != r2 || c1 != c2 {
			s += ":." + a(r2, c2)
		}
		return raw(s + "]")
	}
	s := ""
	if sheet != tr.at.sheet {
		s = quoted + "!"
	}
	s += a(r1, c1)
	if r1 != r2 || c1 != c2 {
		s += ":" + a(r2, c2)
	}
	return raw(s)
}

func (tr *translator) call(v formula.Call) formula.Node {
	switch v.Fn {
	case "REFERENCE":
		return tr.reference(v, false)
	case "REFERENCE.APPROX":
		return tr.reference(v, true)
	case "ITEM":
		// Tableau[Col][k] : la k-ième ligne de données.
		// [Col][-1] (signe écrit) : ligne voisine, vide hors du tableau ; pas
		// traduite.
		r, ok := v.Args[0].(formula.Ref)
		if len(v.Args) != 2 || !ok {
			return tr.fail()
		}
		if u, ok := v.Args[1].(formula.Unary); ok && (u.Op == "-" || u.Op == "+") {
			tr.why = append(tr.why, "ligne voisine [Colonne][-1]")
			return tr.fail()
		}
		r.Row = false
		return formula.Call{Fn: "INDEX", Args: []formula.Node{tr.walk(r), tr.walk(v.Args[1])}}
	case "RANK":
		// RANG([@Score]) : rang dans sa propre colonne.
		if r, ok := v.Args[0].(formula.Ref); ok && r.Row && (len(v.Args) == 1 || len(v.Args) == 2 && !columnArg(v.Args[1])) {
			args := append([]formula.Node{v.Args[0], formula.Ref{Table: r.Table, Col: r.Col}}, v.Args[1:]...)
			return tr.fn("RANK", args)
		}
		return tr.fn("RANK", v.Args)
	case "CEILING", "FLOOR":
		// Excel exige la précision ; LibreOffice n'a pas le même arrondi des
		// négatifs.
		if len(v.Args) != 2 || tr.b.target == toODS {
			return tr.failFn(v.Fn)
		}
		return tr.fn(v.Fn, v.Args)
	}
	if _, ok := exportFuncs[v.Fn]; !ok {
		return tr.failFn(v.Fn)
	}
	return tr.fn(v.Fn, v.Args)
}

// fn écrit un appel de fonction aux arguments traduits.
func (tr *translator) fn(name string, args []formula.Node) formula.Node {
	if name == "RANK" || name == "CEILING" || name == "FLOOR" {
		return formula.Call{Fn: name, Args: tr.args(args)}
	}
	names := exportFuncs[name]
	out := names[0]
	if tr.b.target == toODS {
		out = names[1]
	}
	if out == "" {
		return tr.failFn(name)
	}
	return formula.Call{Fn: out, Args: tr.args(args)}
}

func (tr *translator) args(args []formula.Node) []formula.Node {
	out := make([]formula.Node, len(args))
	for i, a := range args {
		out[i] = tr.walk(a)
	}
	return out
}

// columnArg dit si un argument est une colonne entière.
func columnArg(n formula.Node) bool {
	r, ok := n.(formula.Ref)
	return ok && !r.Row
}

// reference traduit REFERENCE(Tableau; Colonne; clé) : INDEX(colonne;
// EQUIV(clé; colonne clé; 0)), ou INDEX(colonne; rang) sans clé ;
// REFERENCE.APPROCHEE (plus grande clé inférieure ou égale) devient
// RECHERCHEX(…; -1) en .xlsx.
func (tr *translator) reference(v formula.Call, approx bool) formula.Node {
	if len(v.Args) != 3 {
		return tr.fail()
	}
	tn, ok1 := v.Args[0].(formula.Name)
	cn, ok2 := v.Args[1].(formula.Name)
	if !ok1 || !ok2 {
		return tr.fail()
	}
	t := tr.b.tables[strings.ToLower(tn.Name)]
	if t == nil {
		return tr.fail()
	}
	c := t.colIndex(cn.Name)
	if c < 0 {
		return tr.fail()
	}
	key := tr.walk(v.Args[2])
	col := tr.column(t, c)
	if t.key < 0 {
		if approx {
			return tr.failFn(v.Fn)
		}
		return formula.Call{Fn: "INDEX", Args: []formula.Node{col, key}}
	}
	keys := tr.column(t, t.key)
	if approx {
		if tr.b.target == toODS {
			return tr.failFn(v.Fn)
		}
		return formula.Call{Fn: "_xlfn.XLOOKUP", Args: []formula.Node{key, keys, col, raw("NA()"), formula.NumLit{V: -1}}}
	}
	return formula.Call{Fn: "INDEX", Args: []formula.Node{col, formula.Call{Fn: "MATCH", Args: []formula.Node{key, keys, formula.NumLit{V: 0}}}}}
}
