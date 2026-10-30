package transfer

import (
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Références structurées d'Excel (Ventes[Prix], Ventes[[#This Row],[Prix]],
// [@Prix], Ventes[#Data]) et noms définis, à l'import d'un .xlsx. Lattis a
// les mêmes références de tableau (Ventes[Prix], [@Prix] dans une colonne
// calculée) : un tableau d'Excel repris garde son nom, ses colonnes leurs
// en-têtes. C'est aussi ce qu'écrit l'export .xlsx de Lattis : un classeur
// enregistré en .xlsx se relit avec ses formules.

// sref est une référence structurée.
type sref struct {
	table   string   // "" : tableau de la formule ([@Col])
	thisRow bool     // #This Row, @
	item    string   // #Data (""), #All, #Headers, #Totals (minuscules)
	cols    []string // une colonne, ou deux (plage [A]:[B]) ; aucune : tout
}

// parseStructured lit la référence qui commence au crochet rs[at] (table :
// le nom qui le précède, "" sans) ; end : position après le crochet fermant.
func parseStructured(rs []rune, at int, table string) (*sref, int, error) {
	depth := 0
	end := -1
	for j := at; j < len(rs); j++ {
		switch rs[j] {
		case '\'':
			j++ // caractère échappé ('[, '], '#, '')
		case '[':
			depth++
		case ']':
			depth--
		}
		if depth == 0 {
			end = j
			break
		}
	}
	if end < 0 {
		return nil, 0, unsupported("référence de tableau non fermée")
	}
	body := strings.TrimSpace(string(rs[at+1 : end]))
	sr := &sref{table: table}
	if strings.HasPrefix(body, "@") {
		sr.thisRow = true
		body = strings.TrimSpace(body[1:])
		if body == "" {
			return nil, 0, unsupported("ligne entière d'un tableau non reprise")
		}
		if !strings.HasPrefix(body, "[") {
			sr.cols = []string{unescapeCol(body)}
			return sr, end + 1, nil
		}
	}
	if !strings.HasPrefix(body, "[") {
		if strings.HasPrefix(body, "#") {
			sr.item = strings.ToLower(body)
		} else if body != "" {
			sr.cols = []string{unescapeCol(body)}
		}
		return sr, end + 1, sr.checkItem()
	}
	// Liste d'éléments [x],[y] ou [A]:[B].
	for _, part := range splitItems(body) {
		part = strings.TrimSpace(part)
		if a, b, ok := cutRange(part); ok {
			sr.cols = append(sr.cols, unescapeCol(a), unescapeCol(b))
			continue
		}
		p := strings.TrimSuffix(strings.TrimPrefix(part, "["), "]")
		switch low := strings.ToLower(p); {
		case low == "#this row":
			sr.thisRow = true
		case strings.HasPrefix(low, "#"):
			sr.item = low
		default:
			sr.cols = append(sr.cols, unescapeCol(p))
		}
	}
	return sr, end + 1, sr.checkItem()
}

func (sr *sref) checkItem() error {
	switch sr.item {
	case "", "#data":
		sr.item = ""
		return nil
	}
	return unsupported("référence %s non reprise", sr.item)
}

// splitItems coupe « [a],[b] » aux virgules hors des crochets.
func splitItems(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			i++
		case '[':
			depth++
		case ']':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// cutRange lit « [A]:[B] ».
func cutRange(s string) (a, b string, ok bool) {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			i++
		case '[':
			depth++
		case ']':
			depth--
		case ':':
			if depth == 0 {
				x, y := strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
				return strings.Trim(x, "[]"), strings.Trim(y, "[]"), true
			}
		}
	}
	return "", "", false
}

// unescapeCol retire les apostrophes d'échappement d'un nom de colonne.
func unescapeCol(s string) string {
	if !strings.Contains(s, "'") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// structured traduit une référence structurée : Tableau[Col], [@Col] dans
// une colonne calculée du même tableau, Tableau (tout le tableau).
func (rv *resolver) structured(sr *sref, at where) (string, error) {
	is := at.table
	if sr.table != "" {
		is = rv.byXName[strings.ToLower(sr.table)]
	}
	if is == nil {
		if sr.table == "" {
			return "", unsupported("[@colonne] hors d'un tableau")
		}
		return "", unsupported("tableau %s introuvable", sr.table)
	}
	if len(sr.cols) > 1 {
		return "", unsupported("plage de colonnes de %s non reprise", is.name)
	}
	if len(sr.cols) == 0 {
		if sr.thisRow {
			return "", unsupported("ligne entière d'un tableau non reprise")
		}
		return is.name, nil
	}
	col := ""
	for _, h := range is.headers {
		if strings.EqualFold(strings.Join(strings.Fields(h), " "), strings.Join(strings.Fields(sr.cols[0]), " ")) {
			col = h
			break
		}
	}
	if col == "" {
		return "", unsupported("colonne %s introuvable dans %s", sr.cols[0], is.name)
	}
	if sr.thisRow {
		if is != at.table {
			return "", unsupported("ligne courante de %s hors de ce tableau", is.name)
		}
		return colRef("", col)
	}
	return colRef(is.name, col)
}

// named traduit un nom : cellule nommée reprise, nom défini d'une plage
// (remplacé par sa plage), tableau d'Excel (son nom dans Lattis), sinon tel
// quel (TRUE, FALSE…).
func (rv *resolver) named(name string, at where) (string, error) {
	low := strings.ToLower(name)
	if n, ok := rv.cellNames[low]; ok {
		return n, nil
	}
	if v, ok := rv.defined[low]; ok {
		// Lu depuis A1 : les décalages sont les indices ; un nom défini
		// désigne toujours la même plage.
		ps, err := splitFormula(strings.TrimPrefix(v, "="), 0, 0)
		if err != nil || len(ps) != 1 || ps[0].ref == nil {
			return "", unsupported("nom défini %s non repris", name)
		}
		r := *ps[0].ref
		r.r0.abs, r.r1.abs, r.c0.abs, r.c1.abs = true, true, true, true
		return rv.ref(&r, at)
	}
	if is := rv.byXName[low]; is != nil {
		return is.name, nil
	}
	return name, nil
}

// definedNames classe les noms définis du classeur : un nom d'une cellule
// hors des tableaux devient une cellule nommée (rendue par feuille, en
// minuscules) ; les autres sont remplacés par leur plage dans les formules.
func (x *xlsxFile) definedNames(rv *resolver) map[string][]canvas.ImportName {
	out := map[string][]canvas.ImportName{}
	for low, v := range x.names {
		if strings.HasPrefix(low, "_") {
			continue // noms internes (_xlnm.…, _FilterDatabase)
		}
		ps, err := splitFormula(strings.TrimPrefix(v, "="), 0, 0)
		if err != nil || len(ps) != 1 || ps[0].ref == nil || ps[0].ref.cols || ps[0].ref.sheet == "" {
			continue
		}
		r := ps[0].ref
		islands, ok := rv.bySheet[sheetKey(r.sheet)]
		if !ok {
			continue
		}
		rv.defined[low] = v
		if r.rangeOf {
			continue
		}
		row, col := r.r0.n, r.c0.n
		inTable := false
		for _, is := range islands {
			if is.table() && is.contains(row, col) {
				inTable = true
			}
		}
		name := x.nameCase[low]
		if inTable || name == "" || identifier(name) != name {
			continue
		}
		delete(rv.defined, low)
		rv.cellNames[low] = name
		out[sheetKey(r.sheet)] = append(out[sheetKey(r.sheet)], canvas.ImportName{Row: row, Col: col, Name: name})
	}
	return out
}
