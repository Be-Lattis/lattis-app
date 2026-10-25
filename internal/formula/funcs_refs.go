package formula

import (
	"strconv"
	"strings"
)

// Références calculées (DECALER, INDIRECT), informations sur une cellule
// (CELLULE), AGREGAT et fonctions de base de données (BDSOMME…).
//
// DECALER et INDIRECT rendent une référence de la nomenclature : une
// cellule libre (Cell[B3], Feuille2!Cell[B3]), une plage de cellules
// (Cell[A1:B3]), une colonne (Ventes[Prix]), une ligne ou des lignes d'une
// colonne (Ventes[Prix][2], Ventes[Prix][1:3]), ou dans une colonne
// calculée une colonne de la ligne ([Prix], [Prix][-1]). La référence est
// calculée avant la formule qui la lit, par le même code pour les deux
// backends (dynRef) : leurs arguments ne doivent dépendre que de littéraux
// et de cellules (constNode), pas d'une ligne de tableau ni d'un agrégat,
// sinon #REF!. Une adresse A1 écrite dans un texte (INDIRECT("B3")) désigne
// une cellule libre. Le style L1C1 n'est pas pris en charge (#REF!).
//
// Dépendances : quand la cible est connue sans les valeurs des cellules
// (littéraux), Deps la compte ; sinon la formule est « volatile »
// (IsVolatile) : calculée après toutes les autres, et recalculée à chaque
// changement (voir Names.order).

// dynFns : fonctions qui rendent une référence calculée.
func dynFn(fn string) bool { return fn == "OFFSET" || fn == "INDIRECT" }

// dynCtx : ce dont dynRef a besoin pour résoudre une référence.
type dynCtx struct {
	schema *Schema
	names  map[string]Value // nil : sans valeurs (analyse des dépendances)
	table  *Table           // tableau de la formule, ou nil
	home   int64            // feuille de la formule (0 : inconnue)
	// static calcule un argument qui ne dépend que de littéraux et de
	// cellules ; ok faux sinon.
	static func(Node) (Value, bool)
}

// staticWith : static d'un contexte de valeurs de cellules (names nil :
// littéraux seulement).
func staticWith(s *Schema, names map[string]Value, home int64) func(Node) (Value, bool) {
	return func(n Node) (Value, bool) {
		if !constNode(n, s) {
			return Value{}, false
		}
		if names == nil && !literalOnly(n) {
			return Value{}, false
		}
		if home != 0 {
			n = QualifyCells(n, home)
		}
		return Eval(n, &Env{Schema: s, Names: names}), true
	}
}

// literalOnly : une expression sans cellule (calculable sans valeurs).
func literalOnly(n Node) bool {
	switch n := n.(type) {
	case nil, NumLit, StrLit, BoolLit, ErrLit:
		return true
	case Unary:
		return literalOnly(n.X)
	case Binary:
		return literalOnly(n.L) && literalOnly(n.R)
	case Call:
		for _, a := range n.Args {
			if !literalOnly(a) {
				return false
			}
		}
		fn, ok := lookupFunc(n.Fn)
		return ok && fn.kind == scalar
	}
	return false
}

// dynRef résout DECALER ou INDIRECT en une référence.
func (x dynCtx) dynRef(c Call) (Node, ErrCode) {
	if c.home != 0 && x.home == 0 {
		x.home = c.home
	}
	switch c.Fn {
	case "INDIRECT":
		return x.indirect(c)
	case "OFFSET":
		return x.offset(c)
	}
	return nil, ErrValue
}

func (x dynCtx) num(n Node, def float64) (float64, ErrCode) {
	if n == nil {
		return def, 0
	}
	v, ok := x.static(n)
	if !ok {
		return 0, ErrRef
	}
	if v.Kind == Empty {
		return def, 0
	}
	f, e := ToNumber(v)
	if e.IsErr() {
		return 0, e.Err
	}
	return float64(int64(f)), 0
}

// indirect : INDIRECT(texte; [a1]).
func (x dynCtx) indirect(c Call) (Node, ErrCode) {
	if len(c.Args) > 1 && c.Args[1] != nil {
		v, ok := x.static(c.Args[1])
		if !ok {
			return nil, ErrRef
		}
		if b, e := Truth(v); e.IsErr() {
			return nil, e.Err
		} else if !b {
			return nil, ErrRef // style L1C1
		}
	}
	v, ok := x.static(c.Args[0])
	if !ok {
		return nil, ErrRef
	}
	if v.IsErr() {
		return nil, v.Err
	}
	s, e := ToText(v)
	if e.IsErr() || strings.TrimSpace(s) == "" {
		return nil, ErrRef
	}
	return x.parseRef(s)
}

// parseRef lit le texte d'une référence : nomenclature de Lattis
// (Cell[B3], Ventes[Prix]…), adresse A1 (B3, Feuille2!B3, A1:B3) ou nom de
// cellule.
func (x dynCtx) parseRef(s string) (Node, ErrCode) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "=")
	n, err := Parse("="+s, FR)
	if err != nil {
		if n, err = Parse("="+s, EN); err != nil {
			return nil, ErrRef
		}
	}
	n, err = ConvertA1(n, freeLocator{x})
	if err != nil {
		return nil, ErrRef
	}
	switch r := n.(type) {
	case Name:
		if _, ok := x.schema.tableOK(r.Name); ok {
			return nil, ErrRef // un tableau entier n'est pas une référence de cellule
		}
	case CellRange, Ref, RowRange:
	case Call:
		if r.Fn != "ITEM" {
			return nil, ErrRef
		}
	default:
		return nil, ErrRef
	}
	if x.home != 0 {
		n = QualifyCells(n, x.home)
	}
	return n, 0
}

// freeLocator : pour INDIRECT, une adresse A1 désigne une cellule libre.
type freeLocator struct{ x dynCtx }

func (l freeLocator) Known(name string) bool {
	if l.x.schema != nil {
		if _, ok := l.x.schema.Table(name); ok {
			return true
		}
	}
	_, ok := l.x.names[strings.ToLower(name)]
	return ok
}

func (l freeLocator) Tables(sheet string) ([]TablePos, bool) {
	s := l.x.schema
	return nil, sheet == "" || s == nil || s.Sheets == nil || s.Sheets[strings.ToLower(sheet)]
}

func (freeLocator) Key(string, int) (Node, bool) { return nil, false }

// cellKey lit la clé d'une cellule libre : « Cell_B3 », « Feuille2.Cell_B3 »
// ou « cell_b3#2 » (rattachée à la feuille 2).
func cellKey(key string) (sheet string, home int64, row, col int, ok bool) {
	addr := key
	if k := strings.LastIndex(addr, "#"); k > 0 {
		h, err := strconv.ParseInt(addr[k+1:], 10, 64)
		if err != nil {
			return "", 0, 0, 0, false
		}
		home, addr = h, addr[:k]
	}
	if k := strings.LastIndex(addr, "."); k >= 0 {
		sheet, addr = addr[:k], addr[k+1:]
	}
	row, col, ok = CellRef(addr)
	return sheet, home, row, col, ok
}

// cellNode écrit la cellule (row, col) d'une feuille.
func cellNode(sheet string, home int64, row, col int) Node {
	switch {
	case sheet != "":
		return Name{Name: sheet + "." + CellName(row, col)}
	case home != 0:
		return Name{Name: CellKey(home, row, col)}
	}
	return Name{Name: CellName(row, col)}
}

// offset : DECALER(référence; lignes; colonnes; [hauteur]; [largeur]).
func (x dynCtx) offset(c Call) (Node, ErrCode) {
	base := c.Args[0]
	if b, ok := base.(Call); ok && dynFn(b.Fn) {
		r, e := x.dynRef(b)
		if e != 0 {
			return nil, e
		}
		base = r
	}
	dr, e := x.num(c.Args[1], 0)
	if e != 0 {
		return nil, e
	}
	dc, e := x.num(c.Args[2], 0)
	if e != 0 {
		return nil, e
	}
	var hn, wn Node
	if len(c.Args) > 3 {
		hn = c.Args[3]
	}
	if len(c.Args) > 4 {
		wn = c.Args[4]
	}
	switch b := base.(type) {
	case Name:
		sheet, home, r, col, ok := cellKey(b.Name)
		if !ok {
			return nil, ErrValue
		}
		return x.cells(CellRange{Sheet: sheet, Home: home, R1: r, C1: col, R2: r, C2: col}, int(dr), int(dc), hn, wn)
	case CellRange:
		return x.cells(b, int(dr), int(dc), hn, wn)
	case Ref, RowRange, Call:
		return x.tableOffset(base, int(dr), int(dc), hn, wn)
	}
	return nil, ErrValue
}

// cells décale une plage de cellules libres.
func (x dynCtx) cells(r CellRange, dr, dc int, hn, wn Node) (Node, ErrCode) {
	rows, cols := r.Size()
	h, e := x.num(hn, float64(rows))
	if e != 0 {
		return nil, e
	}
	w, e := x.num(wn, float64(cols))
	if e != 0 {
		return nil, e
	}
	r1, c1 := r.R1+dr, r.C1+dc
	if h < 1 || w < 1 || r1 < 0 || c1 < 0 || h*w > MaxRangeCells {
		return nil, ErrRef
	}
	out := CellRange{Sheet: r.Sheet, Home: r.Home, R1: r1, C1: c1, R2: r1 + int(h) - 1, C2: c1 + int(w) - 1}
	if out.R1 == out.R2 && out.C1 == out.C2 {
		return cellNode(out.Sheet, out.Home, out.R1, out.C1), 0
	}
	if out.Home == 0 && out.Sheet == "" && x.home != 0 {
		out.Home = x.home
	}
	return out, 0
}

// tableOffset décale une référence de tableau : colonne entière, lignes,
// ligne de rang k, ou colonne de la ligne courante.
func (x dynCtx) tableOffset(base Node, dr, dc int, hn, wn Node) (Node, ErrCode) {
	w, e := x.num(wn, 1)
	if e != 0 {
		return nil, e
	}
	if w != 1 {
		return nil, ErrRef // une seule colonne de tableau à la fois
	}
	var ref Ref
	from, to := 0, 0 // rangs (0 : colonne entière ; ligne courante si ref.Row)
	switch b := base.(type) {
	case Ref:
		ref = b
	case RowRange:
		ref, from, to = b.Ref, b.From, b.To
	case Call:
		r, ok := b.Args[0].(Ref)
		k, ok2 := b.Args[1].(NumLit)
		if b.Fn != "ITEM" || len(b.Args) != 2 || !ok || !ok2 {
			return nil, ErrRef
		}
		ref, from, to = r, int(k.V), int(k.V)
	}
	// Colonne voisine.
	t := x.table
	if ref.Table != "" && x.schema != nil {
		t, _ = x.schema.Table(ref.Table)
	}
	if t == nil {
		return nil, ErrRef
	}
	i, ok := t.col(ref.Col)
	if !ok || i+dc < 0 || i+dc >= len(t.Cols) {
		return nil, ErrRef
	}
	ref.Col = t.Cols[i+dc].Name
	if ref.Row {
		// Colonne de la ligne courante : [Col], [Col][-1]…
		if hn != nil {
			if h, e := x.num(hn, 1); e != 0 || h != 1 {
				return nil, ErrRef
			}
		}
		if dr == 0 {
			return ref, 0
		}
		op := "+"
		if dr < 0 {
			op = "-"
		}
		return Call{Fn: "ITEM", Args: []Node{Ref{Table: ref.Table, Col: ref.Col}, Unary{Op: op, X: NumLit{V: float64(abs(dr))}}}}, 0
	}
	if from == 0 {
		// Colonne entière : seulement vers une autre colonne.
		if dr != 0 || hn != nil {
			return nil, ErrRef
		}
		return ref, 0
	}
	h, e := x.num(hn, float64(to-from+1))
	if e != 0 {
		return nil, e
	}
	from += dr
	to = from + int(h) - 1
	if h < 1 || from < 1 {
		return nil, ErrRef
	}
	ref.Row = false
	if from == to {
		return Call{Fn: "ITEM", Args: []Node{ref, NumLit{V: float64(from)}}}, 0
	}
	return RowRange{Ref: ref, From: from, To: to}, 0
}

// hasDyn dit si une expression contient DECALER ou INDIRECT hors d'un LET
// (dont les noms ne sont connus qu'une fois remplacés).
func hasDyn(n Node) bool {
	switch n := n.(type) {
	case Unary:
		return hasDyn(n.X)
	case Binary:
		return hasDyn(n.L) || hasDyn(n.R)
	case Call:
		if dynFn(n.Fn) {
			return true
		}
		if n.Fn == "LET" {
			return false
		}
		for _, a := range n.Args {
			if hasDyn(a) {
				return true
			}
		}
	}
	return false
}

// substDyn remplace, dans les arguments d'un appel, chaque DECALER et
// INDIRECT par la référence qu'il désigne (une erreur littérale s'il n'en
// désigne pas) : SOMME(DECALER(Cell[A1]; 0; 0; 3)) lit alors Cell[A1:A3].
func (x dynCtx) substDyn(n Call) Call {
	if n.Fn == "LET" || !hasDyn(n) {
		return n
	}
	args := make([]Node, len(n.Args))
	for i, a := range n.Args {
		args[i] = x.subst(a)
	}
	n.Args = args
	return n
}

func (x dynCtx) subst(n Node) Node {
	switch v := n.(type) {
	case Unary:
		v.X = x.subst(v.X)
		return v
	case Binary:
		v.L, v.R = x.subst(v.L), x.subst(v.R)
		return v
	case Call:
		if dynFn(v.Fn) {
			v = x.substDyn(v) // DECALER(INDIRECT(…); …)
			r, e := x.dynRef(v)
			if e != 0 {
				return ErrLit{V: e}
			}
			return r
		}
		return x.substDyn(v)
	}
	return n
}

// dynDeps : la cible d'un DECALER ou INDIRECT connue sans les valeurs des
// cellules (arguments littéraux), pour Deps.
func dynDeps(c Call) (Node, bool) {
	x := dynCtx{static: staticWith(nil, nil, 0)}
	r, e := x.dynRef(x.substDyn(c))
	return r, e == 0
}

// IsVolatile dit si une formule lit une référence calculée (DECALER,
// INDIRECT) dont la cible dépend des valeurs des cellules : elle est alors
// calculée après les autres, et à chaque recalcul.
func IsVolatile(n Node) bool {
	switch n := n.(type) {
	case Unary:
		return IsVolatile(n.X)
	case Binary:
		return IsVolatile(n.L) || IsVolatile(n.R)
	case Call:
		if dynFn(n.Fn) {
			if _, ok := dynDeps(n); !ok {
				return true
			}
		}
		for _, a := range n.Args {
			if IsVolatile(a) {
				return true
			}
		}
	}
	return false
}

// --- CELLULE -------------------------------------------------------------------------

// cellInfo : CELLULE(type_info; référence), récrite en une formule ordinaire
// selon le type d'information : "adresse" (address), "ligne" (row),
// "colonne" / "col", "contenu" (contents), "type" (b : vide, l : texte,
// v : autre valeur). La référence est obligatoire.
func cellInfo(c Call, static func(Node) (Value, bool)) (Node, ErrCode) {
	if len(c.Args) < 2 || c.Args[1] == nil {
		return nil, ErrValue
	}
	v, ok := static(c.Args[0])
	if !ok {
		return nil, ErrValue
	}
	info, e := ToText(v)
	if e.IsErr() {
		return nil, e.Err
	}
	ref := c.Args[1]
	// Valeur de la première cellule de la référence.
	first := func() (Node, ErrCode) {
		switch r := ref.(type) {
		case Name, Call:
			return r, 0
		case CellRange:
			return cellNode(r.Sheet, r.Home, r.R1, r.C1), 0
		case RowRange:
			return Call{Fn: "ITEM", Args: []Node{r.Ref, NumLit{V: float64(r.From)}}}, 0
		case Ref:
			if r.Row {
				return r, 0
			}
			return Call{Fn: "ITEM", Args: []Node{r, NumLit{V: 1}}}, 0
		}
		return nil, ErrValue
	}
	switch foldAccents(strings.ToLower(strings.TrimSpace(info))) {
	case "adresse", "address":
		return cellAddress(ref)
	case "ligne", "row":
		return Call{Fn: "ROW", Args: []Node{ref}}, 0
	case "col", "colonne", "column":
		return Call{Fn: "COLUMN", Args: []Node{ref}}, 0
	case "contenu", "contents":
		return first()
	case "type":
		f, e := first()
		if e != 0 {
			return nil, e
		}
		return Call{Fn: "IF", Args: []Node{Call{Fn: "ISBLANK", Args: []Node{f}}, StrLit{V: "b"},
			Call{Fn: "IF", Args: []Node{Call{Fn: "ISTEXT", Args: []Node{f}}, StrLit{V: "l"}, StrLit{V: "v"}}}}}, 0
	}
	return nil, ErrValue
}

// cellAddress : l'adresse absolue d'une cellule libre ($B$3, Feuille2!$B$3),
// ou la référence d'un tableau dans la nomenclature (Ventes[Prix][2]).
func cellAddress(ref Node) (Node, ErrCode) {
	addr := func(sheet string, r, c int) Node {
		return StrLit{V: sheetPrefix(sheet) + "$" + ColumnLetters(c) + "$" + strconv.Itoa(r+1)}
	}
	switch r := ref.(type) {
	case Name:
		sheet, _, row, col, ok := cellKey(r.Name)
		if !ok {
			return nil, ErrValue
		}
		return addr(sheet, row, col), 0
	case CellRange:
		return addr(r.Sheet, r.R1, r.C1), 0
	case Ref, RowRange, Call:
		return StrLit{V: strings.TrimPrefix(Format(ref, FR), "=")}, 0
	}
	return nil, ErrValue
}

// --- AGREGAT ----------------------------------------------------------------------

// aggregateFns : les fonctions de AGREGAT par numéro (1 à 19).
var aggregateFns = []string{"", "AVERAGE", "COUNT", "COUNTA", "MAX", "MIN", "PRODUCT", "STDEV.S", "STDEV.P",
	"SUM", "VAR.S", "VAR.P", "MEDIAN", "MODE.SNGL", "LARGE", "SMALL", "PERCENTILE.INC", "QUARTILE.INC",
	"PERCENTILE.EXC", "QUARTILE.EXC"}

// aggregateOf lit AGREGAT(no_fonction; options; réf1; …) : la fonction, ses
// arguments (les erreurs ignorées si l'option le demande : chaque valeur
// devient SIERREUR(valeur; vide)) et le filtre des lignes masquées.
func aggregateOf(c Call, static func(Node) (Value, bool)) (fn string, args []Node, hidden bool, err ErrCode) {
	if len(c.Args) < 3 {
		return "", nil, false, ErrValue
	}
	num := func(n Node) (int, bool) {
		v, ok := static(n)
		if !ok {
			return 0, false
		}
		f, e := ToNumber(v)
		return int(f), !e.IsErr() && f == float64(int(f))
	}
	code, ok1 := num(c.Args[0])
	opt, ok2 := num(c.Args[1])
	if !ok1 || !ok2 || code < 1 || code >= len(aggregateFns) || opt < 0 || opt > 7 {
		return "", nil, false, ErrValue
	}
	fn = aggregateFns[code]
	ignoreErr := opt == 2 || opt == 3 || opt == 6 || opt == 7
	hidden = opt%2 == 1
	refs := c.Args[2:]
	if code >= 14 {
		if len(refs) != 2 {
			return "", nil, false, ErrValue
		}
		refs = refs[:1]
	}
	for _, r := range refs {
		if ignoreErr && r != nil && !IsSpill(r) {
			r = Call{Fn: "IFERROR", Args: []Node{r, nil}}
		}
		args = append(args, r)
	}
	if code >= 14 {
		args = append(args, c.Args[3])
	}
	return fn, args, hidden, 0
}

// --- Base de données ------------------------------------------------------------

// dbFns : fonctions de base de données et l'agrégat de leurs valeurs.
var dbFns = map[string]string{
	"DSUM": "SUM", "DAVERAGE": "AVERAGE", "DCOUNT": "COUNT", "DCOUNTA": "COUNTA", "DMAX": "MAX", "DMIN": "MIN",
	"DPRODUCT": "PRODUCT", "DSTDEV": "STDEV.S", "DSTDEVP": "STDEV.P", "DVAR": "VAR.S", "DVARP": "VAR.P", "DGET": "",
}

// dbScan : BDSOMME(base; champ; critères) et les autres, en fonction de
// parcours : la colonne du champ, puis les colonnes testées par les
// critères. Les critères sont une plage de cellules dont la première ligne
// nomme des colonnes de la base ; chaque ligne suivante est une condition
// (critères d'une même ligne : ET ; lignes entre elles : OU ; une cellule
// vide ne teste rien). Un texte sans opérateur trouve les valeurs qui
// commencent par lui (« Dur » trouve « Durand ») ; « =Dur » l'égalité.
func dbScan(c Call, s *Schema, names map[string]Value, home int64, static func(Node) (Value, bool)) scanCall {
	fail := func(e ErrCode) scanCall { return scanCall{err: e} }
	if len(c.Args) != 3 {
		return fail(ErrValue)
	}
	tn, ok := c.Args[0].(Name)
	if !ok || s == nil {
		return fail(ErrValue)
	}
	t, ok := s.Table(tn.Name)
	if !ok {
		return fail(ErrValue)
	}
	fv, ok := static(c.Args[1])
	if !ok {
		return fail(ErrValue)
	}
	field := -1
	switch fv.Kind {
	case Number:
		if k := int(fv.Num); float64(k) == fv.Num && k >= 1 && k <= len(t.Cols) {
			field = k - 1
		}
	case Text:
		field, _ = t.col(strings.TrimSpace(fv.Str))
	case Error:
		return fail(fv.Err)
	}
	if field < 0 {
		return fail(ErrValue)
	}
	cr, ok := c.Args[2].(CellRange)
	if !ok {
		return fail(ErrValue)
	}
	rows, cols := cr.Size()
	if rows < 2 {
		return fail(ErrValue)
	}
	keys := cr.keys(home)
	sc := scanCall{def: scans[c.Fn], name: c.Fn, cols: []Node{Ref{Table: t.Name, Col: t.Cols[field].Name}}}
	var critCols []int // colonnes de la plage qui ont un en-tête
	for j := range cols {
		h := names[keys[j]]
		if blank(h) {
			continue
		}
		name, e := ToText(h)
		if e.IsErr() {
			return fail(ErrValue)
		}
		i, ok := t.col(strings.TrimSpace(name))
		if !ok {
			return fail(ErrValue)
		}
		critCols = append(critCols, j)
		sc.cols = append(sc.cols, Ref{Table: t.Name, Col: t.Cols[i].Name})
	}
	sc.params = []Node{NumLit{V: float64(len(critCols))}, NumLit{V: float64(rows - 1)}}
	for r := 1; r < rows; r++ {
		for _, j := range critCols {
			sc.params = append(sc.params, Name{Name: keys[r*cols+j]})
		}
	}
	return sc
}

// dbScanner : le parcours d'une fonction de base de données.
type dbScanner struct {
	fn     string
	crits  [][]criterion // par ligne de critères ; nil : cellule vide
	agg    Agg
	hits   int
	found  Value
	err    Value
	numRul numRule
}

func newDB(fn string) func([]Value) scanner {
	return func(p []Value) scanner {
		s := &dbScanner{fn: fn, agg: newAgg(dbFns[fn], numRule{})}
		if len(p) < 2 {
			s.err = Err(ErrValue)
			return s
		}
		k, n := int(p[0].Num), int(p[1].Num)
		if len(p) != 2+k*n {
			s.err = Err(ErrValue)
			return s
		}
		for r := range n {
			row := make([]criterion, k)
			for j := range k {
				v := p[2+r*k+j]
				switch {
				case blank(v):
					row[j] = criterion{op: "*"} // ne teste rien
				case v.Kind == Text && !strings.ContainsAny(v.Str[:1], "<>=!"):
					row[j] = parseCriterion(Str(v.Str + "*")) // commence par
				default:
					row[j] = parseCriterion(v)
				}
				if row[j].err.IsErr() && !s.err.IsErr() {
					s.err = row[j].err
				}
			}
			s.crits = append(s.crits, row)
		}
		return s
	}
}

func (s *dbScanner) setRule(r numRule) { s.agg.rule = r }

func (s *dbScanner) add(cols []Value) {
	if s.err.IsErr() {
		return
	}
	match := false
	for _, row := range s.crits {
		ok := true
		for j, cr := range row {
			if cr.op != "*" && !cr.match(cols[1+j]) {
				ok = false
				break
			}
		}
		if ok {
			match = true
			break
		}
	}
	if !match {
		return
	}
	s.hits++
	if s.fn == "DGET" {
		s.found = cols[0]
		return
	}
	s.agg.AddRange(cols[0])
}

func (s *dbScanner) result() Value {
	if s.err.IsErr() {
		return s.err
	}
	if s.fn == "DGET" {
		switch s.hits {
		case 0:
			return Err(ErrValue)
		case 1:
			return s.found
		}
		return Err(ErrNum)
	}
	if s.hits == 0 && (s.fn == "DAVERAGE") {
		return Err(ErrDiv0)
	}
	return s.agg.Result()
}

func dbDef(fn string) scanDef {
	return scanDef{roles: func(int) ([]role, bool) { return nil, false }, make: newDB(fn)}
}

// --- Contextes des deux backends ------------------------------------------------

// dyn : le contexte de résolution d'une référence calculée dans l'évaluateur Go.
func (env *Env) dyn(n Call) dynCtx {
	x := dynCtx{schema: env.Schema, names: env.Names, table: env.Table, home: n.home}
	if env.Table != nil && env.Table.Sheet != 0 {
		x.home = env.Table.Sheet
	}
	x.static = func(a Node) (Value, bool) {
		if !constNode(a, env.Schema) {
			return Value{}, false
		}
		return Eval(a, env), true
	}
	return x
}

// dyn : le même contexte en SQL, avec les valeurs des cellules connues à la
// compilation.
func (c *compiler) dyn(n Call, sc scope) dynCtx {
	x := dynCtx{schema: c.schema, names: c.names, table: sc.table, home: n.home}
	if sc.table != nil && sc.table.Sheet != 0 {
		x.home = sc.table.Sheet
	}
	x.static = staticWith(c.schema, c.names, x.home)
	return x
}

// --- TRANSPOSE ------------------------------------------------------------------

// constGrid : les valeurs d'une source de forme connue à la compilation : une
// plage de cellules libres ou une constante matricielle (TRANSPOSE).
func (env *Env) constGrid(n Node) ([][]Value, Value) {
	var home int64
	if env.Table != nil {
		home = env.Table.Sheet
	}
	vals, e := constGrid(n, env.Names, home)
	if e != 0 {
		return nil, Err(e)
	}
	return vals, Value{}
}

func (c *compiler) constGrid(n Node, sc scope) ([][]Value, ErrCode) {
	var home int64
	if sc.table != nil {
		home = sc.table.Sheet
	}
	return constGrid(n, c.names, home)
}

func constGrid(n Node, names map[string]Value, home int64) ([][]Value, ErrCode) {
	switch a := n.(type) {
	case ArrayLit:
		return arrayValues(a), 0
	case CellRange:
		vals, ok := cellGrid(a, names, home)
		if !ok {
			return nil, ErrValue
		}
		return vals, 0
	}
	return nil, ErrValue
}

// transpose échange lignes et colonnes ; une cellule vide reste vide.
func transpose(vals [][]Value) [][]Value {
	out := make([][]Value, len(vals[0]))
	for j := range out {
		out[j] = make([]Value, len(vals))
		for i := range vals {
			out[j][i] = vals[i][j]
		}
	}
	return out
}

// tableOK : Table sur un schéma qui peut manquer (analyse des dépendances).
func (s *Schema) tableOK(name string) (*Table, bool) {
	if s == nil {
		return nil, false
	}
	return s.Table(name)
}
