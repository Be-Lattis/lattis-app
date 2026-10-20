package formula

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Locale fixe la langue des noms de fonctions (et de VRAI/FAUX), le
// séparateur décimal et le séparateur d'arguments, choisis à part :
// l'allemand et l'espagnol écrivent 3,5 et SUMME(a; b) comme le français
// (voir ForLanguage). La valeur zéro est le français. Les formules des
// colonnes sont stockées en EN, quelle que soit la langue de l'utilisateur.
type Locale struct {
	Names NameLang
	// Dot : décimale « . » et arguments séparés par « , » (3.5, SUM(a, b)) ;
	// sinon décimale « , » et « ; » (3,5, SOMME(a; b)).
	Dot bool
}

// NameLang : langue des noms de fonctions. Seuls le français et l'anglais ont
// les leurs ; NamesDE et NamesES prennent les noms anglais en attendant.
type NameLang uint8

const (
	NamesFR NameLang = iota
	NamesEN
	NamesDE
	NamesES
)

var (
	FR = Locale{}                          // 3,5 et SOMME(a; b)
	EN = Locale{Names: NamesEN, Dot: true} // 3.5 et SUM(a, b)
)

// ForLanguage donne la locale des formules d'une langue de l'interface
// (fr, en, de, es, zh) : « ; » et décimale « , » en français, allemand et
// espagnol, « , » et « . » en anglais et en chinois ; noms français en
// français, anglais ailleurs. Langue inconnue : le français.
func ForLanguage(lang string) Locale {
	switch strings.ToLower(lang) {
	case "en", "zh":
		return EN
	case "de":
		return Locale{Names: NamesDE}
	case "es":
		return Locale{Names: NamesES}
	}
	return FR
}

// french : noms français (fonctions, VRAI et FAUX).
func (l Locale) french() bool { return l.Names == NamesFR }

func (l Locale) sep() rune {
	if l.Dot {
		return ','
	}
	return ';'
}

func (l Locale) decimal() rune {
	if l.Dot {
		return '.'
	}
	return ','
}

// --- AST --------------------------------------------------------------------

// Node est un nœud de l'AST. Les fonctions sont stockées sous leur nom
// anglais canonique et réaffichées dans la langue de l'utilisateur.
type Node interface{ node() }

type (
	NumLit  struct{ V float64 }
	StrLit  struct{ V string }
	BoolLit struct{ V bool }
	ErrLit  struct{ V ErrCode }
	// Ref est une référence structurée : [@Col] (Table vide, Row vrai),
	// Table[Col] (toute la colonne) ou Table[@Col].
	Ref struct {
		Table, Col string
		Row        bool
	}
	// Name désigne une cellule nommée, ou un tableau / une colonne en
	// argument de REFERENCE.
	Name  struct{ Name string }
	Range struct{ Text string } // plage A1, seulement dans une formule Excel à convertir
	// CellRange est une plage rectangulaire de cellules libres : Cell[A1:B3]
	// (Feuille!Cell[A1:B3] sur une autre feuille), lue dans les agrégats et
	// les fonctions qui parcourent des plages. Home est la feuille de la
	// formule, fixée par QualifyCells (0 : pas encore).
	CellRange struct {
		Sheet          string
		R1, C1, R2, C2 int
		Home           int64
	}
	// RowRange désigne les lignes From à To (rangs d'affichage, 1 = première
	// ligne de données) d'une colonne : Tableau[Col][1:3], ou [Col][1:3] dans
	// le tableau de la formule. Le rang ne tient pas compte de la clé.
	RowRange struct {
		Ref      Ref
		From, To int
	}
	Unary struct {
		Op string // "-", "+" ou "%"
		X  Node
	}
	Binary struct {
		Op   string
		L, R Node
	}
	Call struct {
		Fn     string // nom anglais canonique
		Args   []Node
		Origin string // formule Excel d'origine si la fonction a été convertie

		// rank fixe le rang du tirage d'ALEA (rang + 1 ; 0 : rang suivant de
		// la formule). Posé par LET sur les valeurs de ses noms, pour qu'un
		// nom utilisé deux fois donne le même tirage (voir expandLet).
		rank int
	}
	// ArrayLit est une constante matricielle : {1;2;3} ou {"a"."b"} en
	// français, {1;2;3} ou {"a","b"} en anglais (« ; » sépare les lignes).
	// Ses éléments sont des littéraux (NumLit, StrLit, BoolLit, ErrLit).
	ArrayLit struct {
		Rows [][]Node
	}
)

func (NumLit) node()    {}
func (StrLit) node()    {}
func (BoolLit) node()   {}
func (ErrLit) node()    {}
func (Ref) node()       {}
func (Name) node()      {}
func (Range) node()     {}
func (CellRange) node() {}
func (RowRange) node()  {}
func (Unary) node()     {}
func (Binary) node()    {}
func (Call) node()      {}
func (ArrayLit) node()  {}

// --- Lexer ------------------------------------------------------------------

type tokKind int

const (
	tEOF tokKind = iota
	tNum
	tStr
	tIdent
	tRef // contenu entre crochets, Table devant éventuel
	tErr
	tOp
	tLParen
	tRParen
	tSep
	tRange
	tLBrace   // { d'une constante matricielle
	tRBrace   // }
	tArraySep // séparateur de constante matricielle qui n'est pas tSep : « ; » en anglais (lignes), « . » ou « \ » en français (colonnes)
)

type token struct {
	kind tokKind
	text string
	num  float64
	pos  int
}

var a1Range = regexp.MustCompile(`^(?:(?:[\p{L}\w]+|'(?:[^']|'')+')!)?\$?[A-Za-z]{1,3}\$?\d+:\$?[A-Za-z]{1,3}\$?\d+`)

// SyntaxError situe une erreur de saisie.
type SyntaxError struct {
	Pos int
	Msg string
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("position %d : %s", e.Pos+1, e.Msg) }

func lex(src string, loc Locale) ([]token, error) {
	rs := []rune(src)
	var toks []token
	i := 0
	for i < len(rs) {
		c := rs[i]
		start := i
		switch {
		case unicode.IsSpace(c):
			i++
			continue
		case c == '"':
			var b strings.Builder
			i++
			for {
				if i >= len(rs) {
					return nil, &SyntaxError{start, "guillemet non fermé"}
				}
				if rs[i] == '"' {
					if i+1 < len(rs) && rs[i+1] == '"' {
						b.WriteRune('"')
						i += 2
						continue
					}
					i++
					break
				}
				b.WriteRune(rs[i])
				i++
			}
			toks = append(toks, token{kind: tStr, text: b.String(), pos: start})
			continue
		case unicode.IsDigit(c) || c == loc.decimal() && i+1 < len(rs) && unicode.IsDigit(rs[i+1]):
			for i < len(rs) && (unicode.IsDigit(rs[i]) || rs[i] == loc.decimal()) {
				i++
			}
			if i < len(rs) && (rs[i] == 'e' || rs[i] == 'E') {
				i++
				if i < len(rs) && (rs[i] == '+' || rs[i] == '-') {
					i++
				}
				for i < len(rs) && unicode.IsDigit(rs[i]) {
					i++
				}
			}
			text := strings.Replace(string(rs[start:i]), string(loc.decimal()), ".", 1)
			f, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return nil, &SyntaxError{start, "nombre invalide"}
			}
			toks = append(toks, token{kind: tNum, num: f, pos: start})
			continue
		case c == '#':
			for _, name := range errNames {
				if strings.HasPrefix(strings.ToUpper(string(rs[i:])), name) {
					toks = append(toks, token{kind: tErr, text: name, pos: start})
					i += len([]rune(name))
					break
				}
			}
			if len(toks) > 0 && toks[len(toks)-1].pos == start {
				continue
			}
			for alias, code := range errAliases {
				if strings.HasPrefix(strings.ToUpper(string(rs[i:])), alias) {
					toks = append(toks, token{kind: tErr, text: code.String(), pos: start})
					i += len([]rune(alias))
					break
				}
			}
			if len(toks) > 0 && toks[len(toks)-1].pos == start {
				continue
			}
			return nil, &SyntaxError{start, "erreur inconnue"}
		case c == '[':
			text, n, err := bracket(rs[i:])
			if err != nil {
				return nil, &SyntaxError{start, err.Error()}
			}
			i += n
			toks = append(toks, token{kind: tRef, text: text, pos: start})
			continue
		case c == '(':
			toks = append(toks, token{kind: tLParen, pos: start})
			i++
			continue
		case c == ')':
			toks = append(toks, token{kind: tRParen, pos: start})
			i++
			continue
		case c == loc.sep():
			toks = append(toks, token{kind: tSep, pos: start})
			i++
			continue
		case c == '{' || c == '}':
			kind := tLBrace
			if c == '}' {
				kind = tRBrace
			}
			toks = append(toks, token{kind: kind, pos: start})
			i++
			continue
		case loc.Dot && c == ';', !loc.Dot && (c == '.' || c == '\\'):
			toks = append(toks, token{kind: tArraySep, text: string(c), pos: start})
			i++
			continue
		case strings.ContainsRune("+-*/^&%", c):
			toks = append(toks, token{kind: tOp, text: string(c), pos: start})
			i++
			continue
		case c == '!' && i+1 < len(rs) && rs[i+1] == '=':
			// « != » s'écrit aussi pour « <> » (différent de).
			toks = append(toks, token{kind: tOp, text: "<>", pos: start})
			i += 2
			continue
		case c == '<' || c == '>' || c == '=':
			op := string(c)
			if i+1 < len(rs) && (c == '<' && (rs[i+1] == '>' || rs[i+1] == '=') || c == '>' && rs[i+1] == '=') {
				op += string(rs[i+1])
			}
			toks = append(toks, token{kind: tOp, text: op, pos: start})
			i += len(op)
			continue
		}
		if m := a1Range.FindString(string(rs[i:])); m != "" {
			toks = append(toks, token{kind: tRange, text: m, pos: start})
			i += len([]rune(m))
			continue
		}
		if c == '\'' {
			// 'Ma feuille'!Cell[A1] : nom de feuille entre apostrophes.
			j := i + 1
			for j < len(rs) && rs[j] != '\'' {
				j++
			}
			if j+1 < len(rs) && rs[j+1] == '!' {
				name := string(rs[i+1 : j])
				i = j + 2
				k := i
				for i < len(rs) && (unicode.IsLetter(rs[i]) || unicode.IsDigit(rs[i]) || rs[i] == '_' || rs[i] == '$') {
					i++
				}
				toks = append(toks, token{kind: tIdent, text: name + "!" + strings.ReplaceAll(string(rs[k:i]), "$", ""), pos: start})
				continue
			}
		}
		if unicode.IsLetter(c) || c == '_' || c == '$' {
			// Les $ d'une adresse Excel ($C$14, Feuille2!C$3) sont acceptés
			// puis retirés : les références de Lattis sont toujours fixes.
			for i < len(rs) && (unicode.IsLetter(rs[i]) || unicode.IsDigit(rs[i]) || rs[i] == '_' || rs[i] == '.' || rs[i] == '$' ||
				rs[i] == '!' && i+1 < len(rs) && (unicode.IsLetter(rs[i+1]) || rs[i+1] == '$')) {
				i++
			}
			text := string(rs[start:i])
			if strings.Contains(text, "$") {
				text = strings.ReplaceAll(text, "$", "")
				if _, _, _, ok := A1Address(text); !ok {
					return nil, &SyntaxError{start, "caractère inattendu « $ »"}
				}
			}
			toks = append(toks, token{kind: tIdent, text: text, pos: start})
			continue
		}
		return nil, &SyntaxError{start, fmt.Sprintf("caractère inattendu %q", c)}
	}
	return append(toks, token{kind: tEOF, pos: len(rs)}), nil
}

// bracket lit [ ... ] (crochets imbriqués de Excel, [[Col]] compris).
func bracket(rs []rune) (string, int, error) {
	depth := 0
	for i, r := range rs {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return string(rs[1:i]), i + 1, nil
			}
		}
	}
	return "", 0, fmt.Errorf("crochet non fermé")
}

// --- Parseur ----------------------------------------------------------------

type parser struct {
	toks []token
	i    int
	loc  Locale
	src  []rune
}

// Parse analyse une formule (le « = » initial est facultatif).
func Parse(src string, loc Locale) (Node, error) {
	src = strings.TrimSpace(src)
	src = strings.TrimPrefix(src, "=")
	toks, err := lex(src, loc)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, loc: loc, src: []rune(src)}
	n, err := p.comparison()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind != tEOF {
		return nil, p.extra(t)
	}
	return lookups(n), nil
}

// lookups lit REFERENCE(Tableau[Colonne]; clé) (et REFERENCE.APPROCHEE),
// la forme écrite depuis le 17/10/2026 (choix de Louis), comme l'ancienne
// REFERENCE(Tableau; Colonne; clé), toujours acceptée : en interne, les
// arguments restent tableau, colonne, clé.
func lookups(n Node) Node {
	switch v := n.(type) {
	case Unary:
		v.X = lookups(v.X)
		return v
	case Binary:
		v.L, v.R = lookups(v.L), lookups(v.R)
		return v
	case Call:
		args := make([]Node, len(v.Args))
		for i, a := range v.Args {
			args[i] = lookups(a)
		}
		if (v.Fn == "REFERENCE" || v.Fn == "REFERENCE.APPROX") && len(args) == 2 {
			if r, ok := args[0].(Ref); ok && r.Table != "" && !r.Row {
				args = []Node{Name{Name: r.Table}, Name{Name: r.Col}, args[1]}
			}
		}
		v.Args = args
		return v
	}
	return n
}

func (p *parser) peek() token { return p.toks[p.i] }
func (p *parser) next() token { t := p.toks[p.i]; p.i++; return t }

func (p *parser) isOp(ops ...string) (string, bool) {
	t := p.peek()
	if t.kind != tOp {
		return "", false
	}
	for _, o := range ops {
		if t.text == o {
			return o, true
		}
	}
	return "", false
}

type level func() (Node, error)

func (p *parser) binary(next level, ops ...string) (Node, error) {
	l, err := next()
	if err != nil {
		return nil, err
	}
	for {
		op, ok := p.isOp(ops...)
		if !ok {
			return l, nil
		}
		p.next()
		r, err := next()
		if err != nil {
			return nil, err
		}
		l = Binary{op, l, r}
	}
}

func (p *parser) comparison() (Node, error) {
	return p.binary(p.concat, "=", "<>", "<", "<=", ">", ">=")
}
func (p *parser) concat() (Node, error)   { return p.binary(p.additive, "&") }
func (p *parser) additive() (Node, error) { return p.binary(p.term, "+", "-") }
func (p *parser) term() (Node, error)     { return p.binary(p.power, "*", "/") }
func (p *parser) power() (Node, error)    { return p.binary(p.unary, "^") }

// unary : comme dans Excel, -2^2 vaut 4 (le signe lie plus fort que ^).
func (p *parser) unary() (Node, error) {
	if op, ok := p.isOp("-", "+"); ok {
		p.next()
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		return Unary{op, x}, nil
	}
	x, err := p.primary()
	if err != nil {
		return nil, err
	}
	for {
		if _, ok := p.isOp("%"); !ok {
			return x, nil
		}
		p.next()
		x = Unary{"%", x}
	}
}

func (p *parser) primary() (Node, error) {
	t := p.next()
	switch t.kind {
	case tNum:
		return NumLit{t.num}, nil
	case tStr:
		return StrLit{t.text}, nil
	case tErr:
		for code, name := range errNames {
			if name == t.text {
				return ErrLit{code}, nil
			}
		}
	case tRange:
		return Range{t.text}, nil
	case tLBrace:
		return p.array(t)
	case tRef:
		ref := parseRef("", t.text)
		// [Col][ligne] : une autre ligne du tableau de la formule.
		if p.peek().kind == tRef {
			kt := p.next()
			if rr, ok, err := rowRange(ref, kt); ok || err != nil {
				return rr, err
			}
			key, err := parseInner(kt.text, p.loc)
			if err != nil {
				return nil, &SyntaxError{kt.pos, err.Error()}
			}
			return Call{Fn: "ITEM", Args: []Node{ref, key}}, nil
		}
		return ref, nil
	case tLParen:
		n, err := p.comparison()
		if err != nil {
			return nil, err
		}
		if e := p.next(); e.kind != tRParen {
			if e.kind != tEOF {
				return nil, &SyntaxError{e.pos, "il manque un opérateur ou « ) » avant « " + p.word(e) + " »"}
			}
			return nil, &SyntaxError{t.pos, "il manque une parenthèse fermante « ) »"}
		}
		return n, nil
	case tIdent:
		switch p.peek().kind {
		case tLParen:
			return p.call(t)
		case tRef:
			inner := p.next().text
			// Cell[A1], Feuille1!Cell[A1] : cellule libre par son adresse.
			sheet, word := "", t.text
			if k := strings.LastIndex(t.text, "!"); k >= 0 {
				sheet, word = t.text[:k], t.text[k+1:]
			}
			if strings.EqualFold(word, "cell") {
				if a, b, ok := strings.Cut(inner, ":"); ok {
					// Cell[A1:B3] : plage de cellules libres.
					return cellRange(sheet, a, b, t.pos)
				}
				if r, c, ok := CellRef("Cell_" + strings.TrimSpace(inner)); ok {
					name := CellName(r, c)
					if sheet != "" {
						name = sheet + "." + name
					}
					return Name{name}, nil
				}
				return nil, &SyntaxError{t.pos, "adresse de cellule invalide : Cell[" + inner + "]"}
			}
			ref := parseRef(t.text, inner)
			// Tableau[Colonne][ligne] : une valeur du tableau.
			if p.peek().kind == tRef {
				kt := p.next()
				if rr, ok, err := rowRange(ref, kt); ok || err != nil {
					return rr, err
				}
				key, err := parseInner(kt.text, p.loc)
				if err != nil {
					return nil, &SyntaxError{kt.pos, err.Error()}
				}
				return Call{Fn: "ITEM", Args: []Node{ref, key}}, nil
			}
			return ref, nil
		}
		switch strings.ToUpper(t.text) {
		case "VRAI", "TRUE":
			return BoolLit{true}, nil
		case "FAUX", "FALSE":
			return BoolLit{false}, nil
		}
		return Name{t.text}, nil
	}
	return nil, p.missing(t)
}

// array lit une constante matricielle, après « { » : des littéraux, les
// colonnes séparées par « . » (ou « \ ») en français et « , » en anglais,
// les lignes par « ; ». Toutes les lignes ont le même nombre de valeurs.
func (p *parser) array(open token) (Node, error) {
	colSep, rowSep := tArraySep, tSep
	if p.loc.Dot {
		colSep, rowSep = tSep, tArraySep
	}
	var rows [][]Node
	var row []Node
	for {
		t := p.next()
		neg := false
		if t.kind == tOp && (t.text == "-" || t.text == "+") {
			neg = t.text == "-"
			if t = p.next(); t.kind != tNum {
				return nil, &SyntaxError{t.pos, "une constante matricielle ne contient que des nombres, des textes, VRAI, FAUX ou des erreurs"}
			}
		}
		var v Node
		switch t.kind {
		case tNum:
			if neg {
				v = NumLit{-t.num}
			} else {
				v = NumLit{t.num}
			}
		case tStr:
			v = StrLit{t.text}
		case tErr:
			for code, name := range errNames {
				if name == t.text {
					v = ErrLit{code}
				}
			}
		case tIdent:
			// En français, le « . » qui suit VRAI ou FAUX est lu avec le mot
			// (les noms de fonction en contiennent) : c'est le séparateur.
			word, dot := t.text, false
			if !p.loc.Dot && strings.HasSuffix(word, ".") && strings.Count(word, ".") == 1 {
				word, dot = strings.TrimSuffix(word, "."), true
			}
			switch strings.ToUpper(word) {
			case "VRAI", "TRUE":
				v = BoolLit{true}
			case "FAUX", "FALSE":
				v = BoolLit{false}
			}
			if v != nil && dot {
				row = append(row, v)
				continue
			}
		case tEOF:
			return nil, &SyntaxError{open.pos, "il manque une accolade fermante « } »"}
		}
		if v == nil {
			return nil, &SyntaxError{t.pos, "une constante matricielle ne contient que des nombres, des textes, VRAI, FAUX ou des erreurs"}
		}
		row = append(row, v)
		switch s := p.next(); s.kind {
		case colSep:
		case rowSep:
			rows = append(rows, row)
			row = nil
		case tRBrace:
			rows = append(rows, row)
			for _, r := range rows {
				if len(r) != len(rows[0]) {
					return nil, &SyntaxError{open.pos, "toutes les lignes d'une constante matricielle doivent avoir le même nombre de valeurs"}
				}
			}
			return ArrayLit{Rows: rows}, nil
		case tEOF:
			return nil, &SyntaxError{open.pos, "il manque une accolade fermante « } »"}
		default:
			return nil, &SyntaxError{s.pos, fmt.Sprintf("séparateur attendu dans la constante matricielle : « %s » entre colonnes, « ; » entre lignes", arrayColSep(p.loc))}
		}
	}
}

// arrayColSep : séparateur des colonnes d'une constante matricielle.
func arrayColSep(loc Locale) string {
	if loc.Dot {
		return ","
	}
	return "."
}

// word donne le texte d'un jeton tel qu'il a été tapé (pour les messages).
func (p *parser) word(t token) string {
	switch t.kind {
	case tOp:
		return t.text
	case tLParen:
		return "("
	case tRParen:
		return ")"
	case tSep:
		return string(p.loc.sep())
	}
	if t.pos >= len(p.src) {
		return t.text
	}
	end := t.pos + 1
	for end < len(p.src) && end-t.pos < 20 && !unicode.IsSpace(p.src[end]) && !strings.ContainsRune("+-*/^&%<>=();", p.src[end]) {
		end++
	}
	return string(p.src[t.pos:end])
}

// missing explique, en français ordinaire, une valeur manquante : t est le
// jeton trouvé à sa place (déjà lu).
func (p *parser) missing(t token) error {
	var prev *token
	if k := p.i - 2; k >= 0 {
		prev = &p.toks[k]
	}
	after := ""
	if prev != nil && (prev.kind == tOp || prev.kind == tLParen || prev.kind == tSep) {
		after = " après « " + p.word(*prev) + " »"
	}
	switch t.kind {
	case tEOF:
		if prev == nil {
			return &SyntaxError{t.pos, "la formule est vide"}
		}
		return &SyntaxError{t.pos, "il manque une valeur" + after}
	case tRParen, tSep, tOp:
		if after != "" {
			return &SyntaxError{t.pos, "il manque une valeur" + after}
		}
		return &SyntaxError{t.pos, "il manque une valeur avant « " + p.word(t) + " »"}
	}
	return &SyntaxError{t.pos, "« " + p.word(t) + " » inattendu"}
}

// extra explique un jeton en trop après une formule complète.
func (p *parser) extra(t token) error {
	switch t.kind {
	case tRParen:
		return &SyntaxError{t.pos, "parenthèse fermante « ) » en trop"}
	case tSep:
		return &SyntaxError{t.pos, fmt.Sprintf("« %c » hors des parenthèses d'une fonction", p.loc.sep())}
	}
	return &SyntaxError{t.pos, "il manque un opérateur avant « " + p.word(t) + " »"}
}

// MaxRangeCells borne la taille d'une plage Cell[A1:B3].
const MaxRangeCells = 10000

// cellRange lit Cell[a:b] ; une plage d'une seule cellule est cette cellule.
func cellRange(sheet, a, b string, pos int) (Node, error) {
	r1, c1, ok1 := CellRef("Cell_" + strings.TrimSpace(a))
	r2, c2, ok2 := CellRef("Cell_" + strings.TrimSpace(b))
	if !ok1 || !ok2 {
		return nil, &SyntaxError{pos, "adresse de cellule invalide : Cell[" + a + ":" + b + "]"}
	}
	r1, r2 = min(r1, r2), max(r1, r2)
	c1, c2 = min(c1, c2), max(c1, c2)
	if r1 == r2 && c1 == c2 {
		name := CellName(r1, c1)
		if sheet != "" {
			name = sheet + "." + name
		}
		return Name{name}, nil
	}
	if (r2-r1+1)*(c2-c1+1) > MaxRangeCells {
		return nil, &SyntaxError{pos, fmt.Sprintf("plage trop grande : %d cellules au plus", MaxRangeCells)}
	}
	return CellRange{Sheet: sheet, R1: r1, C1: c1, R2: r2, C2: c2}, nil
}

var rowSpanRe = regexp.MustCompile(`^\s*(\d+)\s*:\s*(\d+)\s*$`)

// rowRange lit [1:3] après une colonne : les lignes 1 à 3 (rangs).
func rowRange(ref Ref, kt token) (Node, bool, error) {
	m := rowSpanRe.FindStringSubmatch(kt.text)
	if m == nil {
		return nil, false, nil
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	a, b = min(a, b), max(a, b)
	if a < 1 {
		return nil, false, &SyntaxError{kt.pos, "les lignes d'un tableau se comptent à partir de 1"}
	}
	ref.Row = false
	return RowRange{Ref: ref, From: a, To: b}, true, nil
}

// parseRef lit le contenu des crochets. Nomenclature de Lattis (sans @) :
// [Col] seul est la valeur de la ligne courante, Tableau[Col] toute la
// colonne. [@Col] et Tableau[@Col] (forme Excel) restent acceptés.
func parseRef(table, inner string) Ref {
	s := strings.TrimSpace(inner)
	s = strings.TrimPrefix(strings.TrimSuffix(s, "]"), "[")
	row := strings.HasPrefix(s, "@") || table == ""
	s = strings.TrimPrefix(s, "@")
	s = strings.TrimPrefix(strings.TrimSuffix(s, "]"), "[")
	return Ref{Table: table, Col: strings.TrimSpace(s), Row: row}
}

// parseInner analyse l'expression entre crochets de Tableau[Col][…].
func parseInner(src string, loc Locale) (Node, error) {
	toks, err := lex(src, loc)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, loc: loc}
	n, err := p.comparison()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tEOF {
		return nil, fmt.Errorf("ligne attendue entre crochets")
	}
	return n, nil
}

func (p *parser) call(name token) (Node, error) {
	p.next() // (
	fn, ok := lookupFunc(name.text)
	c := Call{Fn: strings.ToUpper(name.text)}
	if ok {
		c.Fn = fn.EN
	}
	if p.peek().kind == tRParen {
		p.next()
	} else {
		for {
			if k := p.peek().kind; k == tSep || k == tRParen {
				// argument omis : RECHERCHEV(a; b; 2;) comme dans Excel
				c.Args = append(c.Args, nil)
			} else {
				arg, err := p.comparison()
				if err != nil {
					return nil, err
				}
				c.Args = append(c.Args, arg)
			}
			t := p.next()
			if t.kind == tRParen {
				break
			}
			if t.kind == tEOF {
				return nil, &SyntaxError{t.pos, "il manque une parenthèse fermante « ) » à " + strings.ToUpper(name.text)}
			}
			if t.kind != tSep {
				return nil, &SyntaxError{t.pos, fmt.Sprintf("il manque « %c » ou « ) » avant « %s »", p.loc.sep(), p.word(t))}
			}
		}
	}
	if !ok {
		// Fonction inconnue : l'AST la garde, l'évaluation donne #NOM?.
		return c, nil
	}
	if len(c.Args) < fn.Min || fn.Max >= 0 && len(c.Args) > fn.Max {
		return nil, &SyntaxError{name.pos, fmt.Sprintf("%s attend %s", fn.name(FR), fn.arity())}
	}
	if c.Fn == "LET" {
		// Comme Excel, une forme fautive est refusée à la saisie.
		if len(c.Args)%2 == 0 {
			return nil, &SyntaxError{name.pos, "LET attend des paires nom" + string(p.loc.sep()) + " valeur, puis le calcul : le nombre d'arguments doit être impair"}
		}
		for i := 0; i+1 < len(c.Args); i += 2 {
			if _, ok := letName(c.Args[i]); !ok {
				what := "manquant"
				if c.Args[i] != nil {
					what = "« " + strings.TrimPrefix(Format(c.Args[i], p.loc), "=") + " » invalide"
				}
				return nil, &SyntaxError{name.pos, fmt.Sprintf("LET : nom n° %d %s ; un nom est un identifiant, ni une adresse de cellule ni un nom de fonction", i/2+1, what)}
			}
		}
	}
	return c, nil
}

// --- Affichage --------------------------------------------------------------

func prec(n Node) int {
	switch n := n.(type) {
	case Binary:
		switch n.Op {
		case "=", "<>", "<", "<=", ">", ">=":
			return 1
		case "&":
			return 2
		case "+", "-":
			return 3
		case "*", "/":
			return 4
		case "^":
			return 5
		}
	case Unary:
		return 6
	}
	return 7
}

// Format réécrit l'AST dans une langue, avec le minimum de parenthèses.
func Format(n Node, loc Locale) string {
	var b strings.Builder
	format(&b, n, loc)
	return "=" + b.String()
}

func format(b *strings.Builder, n Node, loc Locale) {
	sub := func(x Node, min int) {
		if x != nil && prec(x) < min {
			b.WriteByte('(')
			format(b, x, loc)
			b.WriteByte(')')
		} else if x != nil {
			format(b, x, loc)
		}
	}
	switch n := n.(type) {
	case NumLit:
		s := strconv.FormatFloat(n.V, 'f', -1, 64)
		b.WriteString(strings.Replace(s, ".", string(loc.decimal()), 1))
	case StrLit:
		b.WriteString(`"` + strings.ReplaceAll(n.V, `"`, `""`) + `"`)
	case BoolLit:
		switch {
		case !loc.french() && n.V:
			b.WriteString("TRUE")
		case !loc.french():
			b.WriteString("FALSE")
		default:
			b.WriteString(Boolean(n.V).String())
		}
	case ErrLit:
		b.WriteString(n.V.String())
	case Ref:
		at := ""
		if n.Row && n.Table != "" {
			at = "@" // forme Excel, sans équivalent dans la nomenclature de Lattis
		}
		b.WriteString(n.Table + "[" + at + n.Col + "]")
	case Name:
		b.WriteString(CellText(n.Name))
	case Range:
		b.WriteString(n.Text)
	case CellRange:
		b.WriteString(n.String())
	case RowRange:
		b.WriteString(n.Ref.Table + "[" + n.Ref.Col + "]" + fmt.Sprintf("[%d:%d]", n.From, n.To))
	case ArrayLit:
		b.WriteByte('{')
		for i, r := range n.Rows {
			if i > 0 {
				b.WriteByte(';')
			}
			for j, v := range r {
				if j > 0 {
					b.WriteString(arrayColSep(loc))
				}
				format(b, v, loc)
			}
		}
		b.WriteByte('}')
	case Unary:
		if n.Op == "%" {
			sub(n.X, 7)
			b.WriteString("%")
		} else {
			b.WriteString(n.Op)
			sub(n.X, 6)
		}
	case Binary:
		p := prec(n)
		sub(n.L, p)
		b.WriteString(n.Op)
		sub(n.R, p+1) // opérateurs associatifs à gauche
	case Call:
		if n.Fn == "ITEM" && len(n.Args) == 2 {
			format(b, n.Args[0], loc)
			b.WriteString("[")
			format(b, n.Args[1], loc)
			b.WriteString("]")
			return
		}
		name := n.Fn
		if fn, ok := lookupFunc(n.Fn); ok {
			name = fn.name(loc)
		}
		// REFERENCE s'écrit avec la colonne et son tableau : REFERENCE(T[Col]; clé).
		if (n.Fn == "REFERENCE" || n.Fn == "REFERENCE.APPROX") && len(n.Args) == 3 {
			t, ok1 := nameOf(n.Args[0])
			c, ok2 := nameOf(n.Args[1])
			if ok1 && ok2 {
				b.WriteString(name + "(" + t + "[" + c + "]" + string(loc.sep()) + " ")
				format(b, n.Args[2], loc)
				b.WriteString(")")
				return
			}
		}
		b.WriteString(name + "(")
		for i, a := range n.Args {
			if i > 0 {
				b.WriteRune(loc.sep())
				b.WriteByte(' ')
			}
			if a != nil {
				format(b, a, loc)
			}
		}
		b.WriteString(")")
	}
}
