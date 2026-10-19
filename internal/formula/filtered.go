package formula

import (
	"strings"
)

// Tableau remplacé par une partie d'un autre (Regrouper des onglets,
// docs/conception/regrouper-onglets-semblables.md, §8) : une fois les
// onglets réunis, Janvier[Montant] devient « Ventes[Montant] sur les lignes
// où Ventes[Mois] = "Janvier" ». Ce n'est pas un renommage : la formule
// change de forme, et seulement là où l'équivalent est exact.
//
//	SOMME(Janvier[Montant])            SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier")
//	MOYENNE, MAX, MIN d'une colonne    MOYENNE.SI.ENS, MAX.SI.ENS, MIN.SI.ENS (une expression
//	                                   sur les colonnes aussi : SOMME(Janvier[Prix]*Janvier[Qté]))
//	NBVAL(Janvier[X])                  NB.SI.ENS(Ventes[X]; "<>"; Ventes[Mois]; "=Janvier")
//	NB(Janvier[X])                     NB(FILTRE(Ventes[X]; Ventes[Mois]="Janvier")) : NB.SI.ENS
//	                                   ne sait pas compter les seuls nombres
//	SOMME.SI(.ENS), NB.SI(.ENS)…       le critère de catégorie en plus
//	LIGNES(Janvier)                    NB.SI.ENS(Ventes[Mois]; "=Janvier")
//	autre colonne entière lue comme    FILTRE(Ventes[X]; Ventes[Mois]="Janvier") : argument d'un
//	une plage                          agrégat (MEDIANE, plusieurs colonnes…), source d'un tableau
//	                                   dynamique (UNIQUE, TRIER, FILTRE…)
//
// Restent à reprendre à la main : une ligne précise (Janvier[X][3],
// Janvier[@X]), le tableau entier (FILTRE(Janvier; …), qui gagnerait les
// colonnes des autres onglets), REFERENCE, RECHERCHEV et EXISTE (ils
// cherchent une clé dans un tableau nommé, pas dans un tableau filtré),
// SOUS.TOTAL (il suit le filtre de l'ancien tableau), les autres fonctions de
// parcours (EQUIV, INDEX, GRANDE.VALEUR…), une colonne entière hors d'une
// fonction qui lit une plage.

// Filtered décrit un ancien tableau devenu une partie du tableau commun.
type Filtered struct {
	Table    string // le tableau commun
	Category string // sa colonne de catégorie
	// Value : la valeur de la catégorie (StrLit, NumLit ou DATE(a; m; j)) ;
	// Crit : le critère d'une fonction .SI.ENS qui ne retient qu'elle, nil
	// s'il n'y en a pas d'exact (FILTRE est alors la seule forme possible).
	Value, Crit Node
	// Columns : colonne de l'ancien tableau (minuscules) → colonne du
	// tableau commun ; une colonne absente n'a pas d'équivalent.
	Columns map[string]string
}

// CategoryNodes donne la valeur d'une catégorie (texte, nombre ou date) en
// formule, et son critère exact pour SOMME.SI.ENS et les autres : "=Janvier"
// (les jokers * et ? échappés), 2025, DATE(2025; 1; 1). Le critère est nil
// quand le texte se lirait autrement (« 2025 », « VRAI », « 01/2025 »).
func CategoryNodes(v Value) (value, crit Node) {
	switch v.Kind {
	case Number:
		return NumLit{V: v.Num}, NumLit{V: v.Num}
	case Date:
		t := SerialTime(v.Num)
		d := Call{Fn: "DATE", Args: []Node{NumLit{V: float64(t.Year())}, NumLit{V: float64(t.Month())}, NumLit{V: float64(t.Day())}}}
		return d, d
	case Text:
		s := v.Str
		lit := s
		if strings.ContainsAny(s, "*?") {
			lit = strings.NewReplacer("~", "~~", "*", "~*", "?", "~?").Replace(s)
		}
		c := parseCriterion(Str("=" + lit))
		if c.op != "=" || c.val.Kind != Text || c.wild && !Wildcard(c.val.Str, s) || !c.wild && c.val.Str != s {
			return StrLit{V: s}, nil
		}
		return StrLit{V: s}, StrLit{V: "=" + lit}
	}
	return nil, nil
}

// FilterTables réécrit une formule dont des tableaux (clés de tables, en
// minuscules) sont devenus des parties d'un autre. cites dit si la formule
// les citait ; why, si elle ne peut pas être réécrite exactement, pourquoi
// (la formule rendue est alors celle d'origine).
func FilterTables(n Node, tables map[string]Filtered) (out Node, cites bool, why string) {
	f := &filterer{tables: tables}
	out = f.walk(n, "")
	if f.why != "" {
		return n, true, f.why
	}
	return out, f.cites, ""
}

type filterer struct {
	tables map[string]Filtered
	cites  bool
	why    string
}

func (f *filterer) fail(why string) {
	f.cites = true
	if f.why == "" {
		f.why = why
	}
}

// old donne l'ancien tableau name (vide : pas un ancien tableau).
func (f *filterer) old(name string) (Filtered, bool) {
	if name == "" {
		return Filtered{}, false
	}
	ft, ok := f.tables[strings.ToLower(name)]
	if ok {
		f.cites = true
	}
	return ft, ok
}

func (f *filterer) isOld(name string) bool {
	_, ok := f.tables[strings.ToLower(name)]
	return ok && name != ""
}

// column : Tableau[Colonne] du tableau commun pour une colonne de l'ancien.
func (f *filterer) column(ft Filtered, r Ref) (Ref, bool) {
	c, ok := ft.Columns[strings.ToLower(strings.TrimSpace(r.Col))]
	if !ok {
		f.fail("la colonne « " + r.Col + " » de « " + r.Table + " » n'est pas dans « " + ft.Table + " »")
		return r, false
	}
	return Ref{Table: ft.Table, Col: c}, true
}

func (ft Filtered) category() Ref { return Ref{Table: ft.Table, Col: ft.Category} }

// condition : Ventes[Mois]="Janvier".
func (ft Filtered) condition() Node {
	return Binary{Op: "=", L: ft.category(), R: ft.Value}
}

// filter : FILTRE(Ventes[X]; Ventes[Mois]="Janvier").
func (f *filterer) filter(ft Filtered, r Ref) Node {
	c, ok := f.column(ft, r)
	if !ok {
		return r
	}
	return Call{Fn: "FILTER", Args: []Node{c, ft.condition()}}
}

// wholeColumn : r est une colonne entière d'un ancien tableau.
func (f *filterer) wholeColumn(n Node) (Filtered, Ref, bool) {
	r, ok := n.(Ref)
	if !ok || r.Row || !f.isOld(r.Table) {
		return Filtered{}, Ref{}, false
	}
	ft, _ := f.old(r.Table)
	return ft, r, true
}

// rowTables : les anciens tableaux dont n lit les colonnes ligne à ligne
// (hors des agrégats, parcours et tableaux dynamiques imbriqués, qui lisent
// leurs colonnes entières pour leur compte).
func (f *filterer) rowTables(n Node) []string {
	var out []string
	add := func(t string) {
		for _, x := range out {
			if strings.EqualFold(x, t) {
				return
			}
		}
		out = append(out, t)
	}
	var walk func(Node)
	walk = func(n Node) {
		switch n := n.(type) {
		case Ref:
			if f.isOld(n.Table) {
				add(n.Table)
			}
		case Unary:
			walk(n.X)
		case Binary:
			walk(n.L)
			walk(n.R)
		case Call:
			if fn, ok := lookupFunc(n.Fn); ok && fn.kind != scalar {
				return
			}
			for _, a := range n.Args {
				walk(a)
			}
		}
	}
	walk(n)
	return out
}

var ifsOf = map[string]string{"SUM": "SUMIFS", "AVERAGE": "AVERAGEIFS", "MAX": "MAXIFS", "MIN": "MINIFS"}

// walk réécrit n. row : l'ancien tableau (minuscules) dont les colonnes se
// lisent ici ligne à ligne (colonne d'une fonction .SI.ENS, condition d'un
// FILTRE), "" ailleurs.
func (f *filterer) walk(n Node, row string) Node {
	switch v := n.(type) {
	case Ref:
		if v.Table == "" {
			if row != "" {
				f.fail("[" + v.Col + "] désigne une colonne de l'ancien tableau")
			}
			return v
		}
		ft, ok := f.old(v.Table)
		if !ok {
			return v
		}
		switch {
		case v.Row:
			f.fail(v.Table + "[@" + v.Col + "] : la ligne en cours d'un ancien tableau")
		case strings.EqualFold(v.Table, row):
			c, _ := f.column(ft, v)
			return c
		default:
			f.fail(v.Table + "[" + v.Col + "] : colonne entière hors d'une fonction qui lit une plage")
		}
		return v
	case RowRange:
		if _, ok := f.old(v.Ref.Table); ok {
			f.fail(v.Ref.Table + "[" + v.Ref.Col + "] : des lignes précises d'un ancien tableau")
		}
		return v
	case Name:
		if _, ok := f.old(v.Name); ok {
			f.fail("« " + v.Name + " » : le tableau entier")
		}
		return v
	case Unary:
		v.X = f.walk(v.X, row)
		return v
	case Binary:
		v.L, v.R = f.walk(v.L, row), f.walk(v.R, row)
		return v
	case Call:
		return f.call(v, row)
	}
	return n
}

func (f *filterer) args(c Call, row string) Call {
	args := make([]Node, len(c.Args))
	for i, a := range c.Args {
		if a != nil {
			args[i] = f.walk(a, row)
		}
	}
	c.Args = args
	return c
}

func (f *filterer) call(c Call, row string) Node {
	fn, known := lookupFunc(c.Fn)
	// Le tableau nommé de REFERENCE, EXISTE, RECHERCHEV… : une clé cherchée
	// dans l'ancien tableau, sans équivalent sur un tableau filtré.
	switch c.Fn {
	case "REFERENCE", "REFERENCE.APPROX", "EXISTS", "VLOOKUP", "HLOOKUP", "XLOOKUP":
		if len(c.Args) > 0 {
			var name string
			switch a := c.Args[0].(type) {
			case Name:
				name = a.Name
			case StrLit:
				name = a.V
			}
			if f.isOld(name) {
				f.old(name)
				f.fail(fn.name(FR) + " sur « " + name + " » : la recherche ne se fait pas dans une partie de tableau")
				return c
			}
		}
		return f.args(c, "")
	case "ITEM":
		if len(c.Args) > 0 {
			if len(f.rowTables(c.Args[0])) > 0 {
				f.fail(strings.TrimPrefix(Format(c, FR), "=") + " : une ligne précise d'un ancien tableau")
				return c
			}
		}
		return f.args(c, row)
	case "ROWS":
		// LIGNES(Janvier) ou LIGNES(Janvier[X]) : le nombre de lignes du mois.
		if len(c.Args) == 1 {
			var name string
			switch a := c.Args[0].(type) {
			case Name:
				name = a.Name
			case Ref:
				if !a.Row {
					name = a.Table
				}
			}
			if ft, ok := f.old(name); ok {
				if ft.Crit == nil {
					f.fail("LIGNES(" + name + ") : la catégorie n'a pas de critère exact")
					return c
				}
				return Call{Fn: "COUNTIFS", Args: []Node{ft.category(), ft.Crit}}
			}
		}
		return f.args(c, "")
	}
	if !known {
		return f.args(c, row)
	}
	switch fn.kind {
	case aggregate:
		return f.aggregate(c)
	case scan:
		return f.scan(c)
	case spill:
		return f.spill(c)
	case special:
		if c.Fn == "LET" {
			return f.args(c, row)
		}
		return f.args(c, "")
	}
	return f.args(c, row)
}

// aggregate : SOMME(Janvier[Montant]) et les autres agrégats.
func (f *filterer) aggregate(c Call) Node {
	lead := aggLead[c.Fn]
	// Une seule colonne (ou expression sur les colonnes d'un ancien
	// tableau) : la forme .SI.ENS.
	if len(c.Args) == lead+1 && lead == 0 {
		a := c.Args[0]
		if ts := f.rowTables(a); len(ts) == 1 {
			ft, _ := f.old(ts[0])
			_, plain := a.(Ref)
			switch {
			case ifsOf[c.Fn] != "" && ft.Crit != nil:
				col := f.walk(a, strings.ToLower(ts[0]))
				return Call{Fn: ifsOf[c.Fn], Args: []Node{col, ft.category(), ft.Crit}}
			case c.Fn == "COUNTA" && plain && ft.Crit != nil:
				col := f.walk(a, strings.ToLower(ts[0]))
				return Call{Fn: "COUNTIFS", Args: []Node{col, StrLit{V: "<>"}, ft.category(), ft.Crit}}
			}
		}
	}
	args := make([]Node, len(c.Args))
	for i, a := range c.Args {
		if a == nil {
			continue
		}
		if i < lead {
			args[i] = f.walk(a, "")
			continue
		}
		if ft, r, ok := f.wholeColumn(a); ok {
			// Plusieurs colonnes : une somme de sommes reste exacte ; pour
			// les autres agrégats, la colonne filtrée.
			if c.Fn == "SUM" && ft.Crit != nil {
				col, _ := f.column(ft, r)
				args[i] = Call{Fn: "SUMIFS", Args: []Node{col, ft.category(), ft.Crit}}
			} else {
				args[i] = f.filter(ft, r)
			}
			continue
		}
		if len(f.rowTables(a)) > 0 {
			f.fail(strings.TrimPrefix(Format(c, FR), "=") + " : une expression sur les colonnes d'un ancien tableau, parmi d'autres arguments")
			args[i] = a
			continue
		}
		args[i] = f.walk(a, "")
	}
	c.Args = args
	return c
}

// ifFamily : les fonctions .SI et .SI.ENS, réécrites en .SI.ENS avec le
// critère de catégorie en plus. Rend la plage des valeurs (-1 : aucune) et
// les paires (plage ; critère) d'origine.
func ifFamily(c Call) (to string, value int, pairs [][2]int, ok bool) {
	n := len(c.Args)
	switch c.Fn {
	case "SUMIFS", "AVERAGEIFS", "MAXIFS", "MINIFS":
		for i := 1; i+1 < n; i += 2 {
			pairs = append(pairs, [2]int{i, i + 1})
		}
		return c.Fn, 0, pairs, n >= 3 && n%2 == 1
	case "COUNTIFS":
		for i := 0; i+1 < n; i += 2 {
			pairs = append(pairs, [2]int{i, i + 1})
		}
		return c.Fn, -1, pairs, n >= 2 && n%2 == 0
	case "COUNTIF":
		return "COUNTIFS", -1, [][2]int{{0, 1}}, n == 2
	case "SUMIF", "AVERAGEIF":
		to = map[string]string{"SUMIF": "SUMIFS", "AVERAGEIF": "AVERAGEIFS"}[c.Fn]
		value = 0
		if n == 3 && c.Args[2] != nil {
			value = 2
		}
		return to, value, [][2]int{{0, 1}}, n == 2 || n == 3
	}
	return "", 0, nil, false
}

// scan : fonctions de parcours. Les .SI et .SI.ENS sur un ancien tableau
// gagnent le critère de catégorie ; les autres sont à reprendre.
func (f *filterer) scan(c Call) Node {
	def := scans[c.Fn]
	var roles []role
	if def.roles != nil {
		roles, _ = def.roles(len(c.Args))
	}
	var olds []string
	for i, a := range c.Args {
		if i < len(roles) && roles[i] == column && a != nil {
			for _, t := range f.rowTables(a) {
				dup := false
				for _, o := range olds {
					dup = dup || strings.EqualFold(o, t)
				}
				if !dup {
					olds = append(olds, t)
				}
			}
		}
	}
	if len(olds) == 0 {
		// Les colonnes lues sont d'autres tableaux : les paramètres (un
		// critère SOMME(Janvier[X])) se réécrivent comme le reste.
		return f.args(c, "")
	}
	ft, _ := f.old(olds[0])
	to, value, pairs, ok := ifFamily(c)
	switch {
	case len(olds) > 1:
		f.fail(strings.TrimPrefix(Format(c, FR), "=") + " : plusieurs anciens tableaux")
		return c
	case !ok:
		if fn, found := lookupFunc(c.Fn); found {
			f.fail(fn.name(FR) + " sur « " + olds[0] + " » : pas d'équivalent exact sur une partie de tableau")
		} else {
			f.fail(c.Fn + " sur « " + olds[0] + " »")
		}
		return c
	case ft.Crit == nil:
		f.fail("« " + olds[0] + " » : la catégorie n'a pas de critère exact")
		return c
	}
	row := strings.ToLower(olds[0])
	col := func(i int) Node { return f.walk(c.Args[i], row) }
	var args []Node
	if value >= 0 {
		args = append(args, col(value))
	}
	for _, p := range pairs {
		args = append(args, col(p[0]), f.walk(c.Args[p[1]], ""))
	}
	args = append(args, ft.category(), ft.Crit)
	return Call{Fn: to, Args: args}
}

// spill : tableaux dynamiques. Une colonne d'un ancien tableau en source
// devient la colonne filtrée ; les conditions de FILTRE et les clés de
// TRIERPAR, lues sur les lignes de la source, suivent.
func (f *filterer) spill(c Call) Node {
	if !NamesSource(c) || len(c.Args) == 0 {
		return f.args(c, "")
	}
	args := make([]Node, len(c.Args))
	src := ""
	if ft, r, ok := f.wholeColumn(c.Args[0]); ok {
		args[0] = f.filter(ft, r)
		src = strings.ToLower(r.Table)
	} else if c.Args[0] != nil {
		args[0] = f.walk(c.Args[0], "")
	}
	for i := 1; i < len(c.Args); i++ {
		if c.Args[i] == nil {
			continue
		}
		if src != "" && (c.Fn == "FILTER" && i == 1 || c.Fn == "SORTBY" && i%2 == 1) {
			args[i] = f.walk(c.Args[i], src)
			continue
		}
		args[i] = f.walk(c.Args[i], "")
	}
	c.Args = args
	return c
}
