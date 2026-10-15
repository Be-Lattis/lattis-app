package formula

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Locale fixe le séparateur décimal et le séparateur d'arguments.
type Locale int

const (
	FR Locale = iota // 3,5 et SOMME(a; b)
	EN               // 3.5 et SUM(a, b)
)

func (l Locale) sep() rune {
	if l == EN {
		return ','
	}
	return ';'
}

func (l Locale) decimal() rune {
	if l == EN {
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
	}
)

func (NumLit) node()  {}
func (StrLit) node()  {}
func (BoolLit) node() {}
func (ErrLit) node()  {}
func (Ref) node()     {}
func (Name) node()    {}
func (Range) node()   {}
func (Unary) node()   {}
func (Binary) node()  {}
func (Call) node()    {}

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
)

type token struct {
	kind tokKind
	text string
	num  float64
	pos  int
}

var a1Range = regexp.MustCompile(`^(?:[\p{L}\w]+!)?\$?[A-Za-z]{1,3}\$?\d+:\$?[A-Za-z]{1,3}\$?\d+`)

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
		case strings.ContainsRune("+-*/^&%", c):
			toks = append(toks, token{kind: tOp, text: string(c), pos: start})
			i++
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
		if unicode.IsLetter(c) || c == '_' {
			for i < len(rs) && (unicode.IsLetter(rs[i]) || unicode.IsDigit(rs[i]) || rs[i] == '_' || rs[i] == '.') {
				i++
			}
			toks = append(toks, token{kind: tIdent, text: string(rs[start:i]), pos: start})
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
}

// Parse analyse une formule (le « = » initial est facultatif).
func Parse(src string, loc Locale) (Node, error) {
	src = strings.TrimSpace(src)
	src = strings.TrimPrefix(src, "=")
	toks, err := lex(src, loc)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, loc: loc}
	n, err := p.comparison()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind != tEOF {
		return nil, &SyntaxError{t.pos, "suite inattendue"}
	}
	return n, nil
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
	case tRef:
		return parseRef("", t.text), nil
	case tLParen:
		n, err := p.comparison()
		if err != nil {
			return nil, err
		}
		if p.next().kind != tRParen {
			return nil, &SyntaxError{t.pos, "parenthèse non fermée"}
		}
		return n, nil
	case tIdent:
		switch p.peek().kind {
		case tLParen:
			return p.call(t)
		case tRef:
			return parseRef(t.text, p.next().text), nil
		}
		switch strings.ToUpper(t.text) {
		case "VRAI", "TRUE":
			return BoolLit{true}, nil
		case "FAUX", "FALSE":
			return BoolLit{false}, nil
		}
		return Name{t.text}, nil
	}
	return nil, &SyntaxError{t.pos, "valeur attendue"}
}

// parseRef lit le contenu des crochets : @Col, Col, [@Col], [Col]. Sans @,
// la référence désigne toute la colonne, comme dans Excel.
func parseRef(table, inner string) Ref {
	s := strings.TrimSpace(inner)
	s = strings.TrimPrefix(strings.TrimSuffix(s, "]"), "[")
	row := strings.HasPrefix(s, "@")
	s = strings.TrimPrefix(s, "@")
	s = strings.TrimPrefix(strings.TrimSuffix(s, "]"), "[")
	return Ref{Table: table, Col: strings.TrimSpace(s), Row: row}
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
			if t.kind != tSep {
				return nil, &SyntaxError{t.pos, fmt.Sprintf("« %c » ou « ) » attendu", p.loc.sep())}
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
		case loc == EN && n.V:
			b.WriteString("TRUE")
		case loc == EN:
			b.WriteString("FALSE")
		default:
			b.WriteString(Boolean(n.V).String())
		}
	case ErrLit:
		b.WriteString(n.V.String())
	case Ref:
		at := ""
		if n.Row {
			at = "@"
		}
		b.WriteString(n.Table + "[" + at + n.Col + "]")
	case Name:
		b.WriteString(n.Name)
	case Range:
		b.WriteString(n.Text)
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
		name := n.Fn
		if fn, ok := lookupFunc(n.Fn); ok {
			name = fn.name(loc)
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
