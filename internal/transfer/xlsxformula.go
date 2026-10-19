package transfer

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/lalamefine/lattis/internal/formula"
)

// Conversion des formules Excel : Lattis n'a pas d'adresses A1. Une
// référence à la cellule de la même ligne d'un tableau devient [@Colonne],
// une plage qui couvre une colonne de tableau devient Tableau[Colonne], et
// la plage d'une RECHERCHEV qui commence à la première colonne d'un tableau
// devient ce tableau (converti ensuite en REFERENCE par le moteur de
// formules). Tout le reste est refusé : la colonne garde alors ses valeurs.

// ref est une référence A1 : une cellule, une plage ou des colonnes
// entières. Les parties relatives sont gardées en décalage par rapport à la
// cellule de la formule (comme en notation L1C1) : deux cellules d'une
// colonne qui ont la même formule « recopiée » ont les mêmes pièces.
type ref struct {
	sheet   string // vide : la feuille de la formule
	cols    bool   // colonnes entières (B:D)
	r0, r1  coord
	c0, c1  coord
	fn      string // fonction qui reçoit la référence en argument
	arg     int    // rang de l'argument
	rangeOf bool   // plage (deux coins) plutôt qu'une cellule
}

type coord struct {
	n   int // absolu : indice ; relatif : décalage
	abs bool
}

func (c coord) at(base int) int {
	if c.abs {
		return c.n
	}
	return base + c.n
}

// piece est un morceau de formule : du texte tel quel, ou une référence.
type piece struct {
	text string
	ref  *ref
}

// errFormula signale une formule Excel que Lattis ne sait pas reprendre.
type errFormula struct{ msg string }

func (e errFormula) Error() string { return e.msg }

func unsupported(format string, a ...any) error { return errFormula{fmt.Sprintf(format, a...)} }

// splitFormula découpe une formule Excel (telle qu'écrite dans le fichier,
// en anglais, sans =) écrite dans la cellule (row, col).
func splitFormula(src string, row, col int) ([]piece, error) {
	rs := []rune(src)
	var out []piece
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			out = append(out, piece{text: lit.String()})
			lit.Reset()
		}
	}
	type frame struct {
		fn  string
		arg int
	}
	var stack []frame
	lastIdent := ""
	for i := 0; i < len(rs); {
		c := rs[i]
		switch {
		case c == '"':
			j := i + 1
			for j < len(rs) {
				if rs[j] == '"' {
					if j+1 < len(rs) && rs[j+1] == '"' {
						j += 2
						continue
					}
					break
				}
				j++
			}
			lit.WriteString(string(rs[i:min(j+1, len(rs))]))
			i = j + 1
			lastIdent = ""
			continue
		case c == '[':
			return nil, unsupported("références de tableau Excel non reprises")
		case c == '{':
			return nil, unsupported("constantes matricielles non reprises")
		case c == '(':
			stack = append(stack, frame{fn: lastIdent})
		case c == ')':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case c == ',':
			if len(stack) > 0 {
				stack[len(stack)-1].arg++
			}
		case c == '\'' || c == '$' || c == '_' || unicode.IsLetter(c) || unicode.IsDigit(c) || c == '.':
			sheet := ""
			j := i
			if c == '\'' {
				j++
				var b strings.Builder
				for {
					if j >= len(rs) {
						return nil, unsupported("nom de feuille non fermé")
					}
					if rs[j] == '\'' {
						if j+1 < len(rs) && rs[j+1] == '\'' {
							b.WriteRune('\'')
							j += 2
							continue
						}
						break
					}
					b.WriteRune(rs[j])
					j++
				}
				if j+1 >= len(rs) || rs[j+1] != '!' {
					return nil, unsupported("nom entre apostrophes inattendu")
				}
				sheet = b.String()
				j += 2
			}
			k := j
			for k < len(rs) && (rs[k] == '$' || rs[k] == '_' || rs[k] == '.' || rs[k] == ':' || rs[k] == '!' || unicode.IsLetter(rs[k]) || unicode.IsDigit(rs[k])) {
				k++
			}
			word := string(rs[j:k])
			if sheet == "" {
				if b := strings.LastIndexByte(word, '!'); b >= 0 {
					sheet, word = word[:b], word[b+1:]
				}
			}
			call := k < len(rs) && rs[k] == '('
			if r, ok := parseRef(word, row, col); ok && !call {
				r.sheet = sheet
				if len(stack) > 0 {
					r.fn, r.arg = strings.ToUpper(stack[len(stack)-1].fn), stack[len(stack)-1].arg
				}
				flush()
				out = append(out, piece{ref: &r})
				lastIdent = ""
				i = k
				continue
			}
			if sheet != "" {
				return nil, unsupported("nom défini %s!%s non repris", sheet, word)
			}
			if strings.ContainsAny(word, ":!") {
				return nil, unsupported("référence %s non reprise", word)
			}
			lastIdent = word
			lit.WriteString(word)
			i = k
			continue
		}
		if !unicode.IsSpace(c) {
			lastIdent = ""
		}
		lit.WriteRune(c)
		i++
	}
	flush()
	return out, nil
}

// parseRef lit B2, $B$2, B2:C10, B:C. Les lignes entières (2:5) ne sont pas
// reprises.
func parseRef(w string, row, col int) (ref, bool) {
	parts := strings.Split(w, ":")
	if len(parts) > 2 {
		return ref{}, false
	}
	type end struct {
		r, c       coord
		hasR, hasC bool
	}
	read := func(s string) (end, bool) {
		var e end
		i := 0
		abs := false
		if i < len(s) && s[i] == '$' {
			abs, i = true, i+1
		}
		j := i
		n := 0
		for i < len(s) && (s[i]|0x20) >= 'a' && (s[i]|0x20) <= 'z' {
			n = n*26 + int(s[i]|0x20-'a'+1)
			i++
		}
		if i > j {
			if i-j > 3 || n > 16384 {
				return e, false
			}
			e.c, e.hasC = coord{n - 1, abs}, true
			if !abs {
				e.c.n -= col
			}
			abs = false
			if i < len(s) && s[i] == '$' {
				abs, i = true, i+1
			}
		}
		if i < len(s) {
			r, err := strconv.Atoi(s[i:])
			if err != nil || r < 1 || r > 1048576 || s[i] == '+' || s[i] == '-' {
				return e, false
			}
			e.r, e.hasR = coord{r - 1, abs}, true
			if !abs {
				e.r.n -= row
			}
		} else if abs && !e.hasC {
			return e, false
		}
		return e, e.hasC || e.hasR
	}
	a, ok := read(parts[0])
	if !ok || !a.hasC {
		return ref{}, false
	}
	if len(parts) == 1 {
		if !a.hasR {
			return ref{}, false
		}
		return ref{r0: a.r, r1: a.r, c0: a.c, c1: a.c}, true
	}
	b, ok := read(parts[1])
	if !ok || !b.hasC || a.hasR != b.hasR {
		return ref{}, false
	}
	if !a.hasR {
		return ref{cols: true, c0: a.c, c1: b.c, rangeOf: true}, true
	}
	return ref{r0: a.r, r1: b.r, c0: a.c, c1: b.c, rangeOf: true}, true
}

// key identifie une formule recopiée : même texte, mêmes décalages.
func key(ps []piece) string {
	var b strings.Builder
	for _, p := range ps {
		if p.ref == nil {
			b.WriteString(p.text)
			continue
		}
		r := p.ref
		fmt.Fprintf(&b, "\x00%s|%v|%v|%v|%v|%v|%v\x00", r.sheet, r.cols, r.r0, r.r1, r.c0, r.c1, r.rangeOf)
	}
	return b.String()
}

// island est un îlot d'une feuille, avec ce qu'il faut pour résoudre les
// références : nom du tableau et en-têtes (vides pour des cellules libres).
type island struct {
	rect
	sheet   string
	name    string
	headers []string
}

func (is *island) table() bool { return is.r1 > is.r0 }

// resolver situe les références dans les îlots du classeur.
type resolver struct {
	bySheet map[string][]*island // nom de feuille en minuscules
}

// where décrit la cellule d'une formule.
type where struct {
	sheet    string
	row, col int
	table    *island // tableau de la cellule (colonne calculée), sinon nil
}

// rewrite traduit une formule découpée en formule Lattis (français, avec
// =), ou explique pourquoi elle ne peut pas l'être.
func (rv *resolver) rewrite(ps []piece, at where) (string, error) {
	var b strings.Builder
	b.WriteByte('=')
	for _, p := range ps {
		if p.ref == nil {
			t := p.text
			for _, prefix := range []string{"_xlfn._xlws.", "_xlfn.", "_xlws."} {
				t = strings.ReplaceAll(t, prefix, "")
			}
			b.WriteString(t)
			continue
		}
		s, err := rv.ref(p.ref, at)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
	}
	n, err := formula.Parse(b.String(), formula.EN)
	if err != nil {
		return "", unsupported("formule illisible (%v)", err)
	}
	n = booleans(n)
	if err := checkFormula(n, false); err != nil {
		return "", err
	}
	return formula.Format(n, formula.FR), nil
}

// booleans remplace les fonctions TRUE() et FALSE() (écrites ainsi par
// LibreOffice) par des constantes.
func booleans(n formula.Node) formula.Node {
	switch m := n.(type) {
	case formula.Unary:
		m.X = booleans(m.X)
		return m
	case formula.Binary:
		m.L, m.R = booleans(m.L), booleans(m.R)
		return m
	case formula.Call:
		if len(m.Args) == 0 && (m.Fn == "TRUE" || m.Fn == "FALSE") {
			return formula.BoolLit{V: m.Fn == "TRUE"}
		}
		args := make([]formula.Node, len(m.Args))
		for i, a := range m.Args {
			if a != nil {
				args[i] = booleans(a)
			}
		}
		m.Args = args
		return m
	}
	return n
}

// checkFormula refuse ce que Lattis calculerait en #NOM? : fonctions inconnues,
// noms définis d'Excel.
func checkFormula(n formula.Node, nameOK bool) error {
	switch n := n.(type) {
	case formula.Name:
		if !nameOK {
			return unsupported("nom %s inconnu", n.Name)
		}
	case formula.Range:
		return unsupported("plage %s non reprise", n.Text)
	case formula.Unary:
		return checkFormula(n.X, false)
	case formula.Binary:
		if err := checkFormula(n.L, false); err != nil {
			return err
		}
		return checkFormula(n.R, false)
	case formula.Call:
		if !formula.Known(n.Fn) {
			return unsupported("fonction %s non disponible", n.Fn)
		}
		for i, a := range n.Args {
			if a == nil {
				continue
			}
			ok := n.Fn == "VLOOKUP" && i == 1 || n.Fn == "REFERENCE" && i < 2
			if err := checkFormula(a, ok); err != nil {
				return err
			}
		}
	}
	return nil
}

func (rv *resolver) ref(r *ref, at where) (string, error) {
	sheet := r.sheet
	if sheet == "" {
		sheet = at.sheet
	}
	islands, ok := rv.bySheet[strings.ToLower(sheet)]
	if !ok && r.sheet != "" {
		return "", unsupported("feuille %s introuvable", r.sheet)
	}
	c0, c1 := r.c0.at(at.col), r.c1.at(at.col)
	if c0 > c1 {
		c0, c1 = c1, c0
	}
	if !r.rangeOf {
		row := r.r0.at(at.row)
		for _, is := range islands {
			if !is.table() || !is.contains(row, c0) || row == is.r0 {
				continue
			}
			if is != at.table || row != at.row || !strings.EqualFold(sheet, at.sheet) {
				break
			}
			return colRef("", is.headers[c0-is.c0])
		}
		return "", unsupported("référence à une cellule (%s) : Lattis n'a pas d'adresses", cellName(row, c0))
	}
	r0, r1 := 0, 1<<30
	if !r.cols {
		r0, r1 = r.r0.at(at.row), r.r1.at(at.row)
		if r0 > r1 {
			r0, r1 = r1, r0
		}
		if at.table != nil && at.table.r1-at.table.r0 > 1 && !(r.r0.abs && r.r1.abs) {
			return "", unsupported("plage relative %s : elle change d'une ligne à l'autre", cellName(r0, c0)+":"+cellName(r1, c1))
		}
	}
	var found *island
	for _, is := range islands {
		if is.r0 > r1 || is.r1 < r0 || is.c0 > c1 || is.c1 < c0 {
			continue
		}
		if found != nil || !is.table() || c0 < is.c0 || c1 > is.c1 || r0 > is.r0+1 || (!r.cols && r1 < is.r1) {
			return "", unsupported("la plage %s ne correspond pas à une colonne de tableau", cellName(r0, c0)+":"+cellName(r1, c1))
		}
		found = is
	}
	if found == nil {
		return "", unsupported("la plage %s est vide", cellName(r0, c0)+":"+cellName(r1, c1))
	}
	if c0 == c1 {
		return colRef(found.name, found.headers[c0-found.c0])
	}
	if r.fn == "VLOOKUP" && r.arg == 1 && c0 == found.c0 {
		return found.name, nil
	}
	return "", unsupported("plage de plusieurs colonnes hors RECHERCHEV")
}

func colRef(table, col string) (string, error) {
	if strings.ContainsAny(col, "[]") {
		return "", unsupported("nom de colonne %q", col)
	}
	if table == "" {
		return "[@" + col + "]", nil
	}
	return table + "[" + col + "]", nil
}

// cellName écrit une adresse A1, pour les messages.
func cellName(row, col int) string {
	if row >= 1<<29 {
		row = 1048575
	}
	var b []byte
	for n := col + 1; n > 0; n = (n - 1) / 26 {
		b = append([]byte{byte('A' + (n-1)%26)}, b...)
	}
	return string(b) + strconv.Itoa(row+1)
}

var errInconsistent = errors.New("formules différentes d'une ligne à l'autre")
