package formula

import (
	"sort"
	"strings"
	"unicode"
)

// Renames décrit des changements de noms à reporter dans les formules :
// tableau renommé ou absorbé par un autre (fusion), colonne renommée,
// feuille renommée, cellule nommée renommée. Les clés sont les anciens noms,
// en minuscules.
type Renames struct {
	Tables map[string]string // ancien nom de tableau → nouveau
	// Columns : ancien nom du tableau → ancienne colonne → nouveau nom. Le
	// tableau est désigné par son nom d'avant Tables.
	Columns map[string]map[string]string
	Sheets  map[string]string // ancien nom de feuille → nouveau
	Names   map[string]string // cellule nommée → nouveau nom
}

// Empty : rien à réécrire.
func (r Renames) Empty() bool {
	return len(r.Tables) == 0 && len(r.Columns) == 0 && len(r.Sheets) == 0 && len(r.Names) == 0
}

// Table donne le nouveau nom d'un tableau (lui-même s'il ne change pas).
func (r Renames) Table(name string) string {
	if n, ok := r.Tables[strings.ToLower(name)]; ok {
		return n
	}
	return name
}

// Column donne le nouveau nom d'une colonne du tableau table (ancien nom).
func (r Renames) Column(table, col string) string {
	if n, ok := r.Columns[strings.ToLower(table)][strings.ToLower(strings.TrimSpace(col))]; ok {
		return n
	}
	return col
}

// Sheet donne le nouveau nom d'une feuille.
func (r Renames) Sheet(name string) string {
	if n, ok := r.Sheets[strings.ToLower(name)]; ok {
		return n
	}
	return name
}

// RenameRefs réécrit dans une formule (saisie de cellule, formule de colonne
// calculée ou de règle, avec ou sans « = », dans la langue loc) les
// références que r change : Tableau[Colonne] et ses lignes, [Colonne] (dans
// le tableau self, ou dans la source d'un FILTRE, TRIERPAR…), les arguments
// tableau et colonne de REFERENCE et EXISTE, Feuille!Cell[A1] (et les
// anciennes formes), les cellules nommées. Le reste du texte (espaces,
// adresses A1 avec $, majuscules) est gardé tel quel. Faux si rien ne
// change (ou si la formule ne se lit pas).
func RenameRefs(src string, loc Locale, r Renames, self string) (string, bool) {
	if r.Empty() {
		return src, false
	}
	lead := len(src) - len(strings.TrimLeftFunc(src, unicode.IsSpace))
	body := src[lead:]
	prefix := src[:lead]
	if strings.HasPrefix(body, "=") {
		prefix += "="
		body = body[1:]
	}
	out, ok := renameBody(body, loc, r, self)
	if !ok {
		return src, false
	}
	return prefix + out, true
}

// renameFrame suit une parenthèse ouverte : fonction (nom anglais), rang de
// l'argument en cours et tableau source (ancien nom) quand le premier
// argument en est un.
type renameFrame struct {
	fn    string
	spill bool
	arg   int
	table string
}

func renameBody(body string, loc Locale, r Renames, self string) (string, bool) {
	toks, err := lex(body, loc)
	if err != nil {
		return body, false
	}
	rs := []rune(body)
	// raw : le texte du jeton i tel qu'il a été tapé.
	raw := func(i int) string {
		end := len(rs)
		if i+1 < len(toks) {
			end = toks[i+1].pos
		}
		return strings.TrimRightFunc(string(rs[toks[i].pos:end]), unicode.IsSpace)
	}
	type edit struct {
		from, to int
		text     string
	}
	var edits []edit
	replace := func(i int, text string) {
		if text != raw(i) {
			edits = append(edits, edit{toks[i].pos, toks[i].pos + len([]rune(raw(i))), text})
		}
	}
	var stack []renameFrame
	bound := map[string]bool{} // noms posés par LET : jamais renommés
	top := func() *renameFrame {
		if len(stack) == 0 {
			return nil
		}
		return &stack[len(stack)-1]
	}
	// whole : le jeton i est à lui seul l'argument en cours.
	whole := func(i int) bool {
		if i == 0 || i+1 >= len(toks) {
			return false
		}
		p, n := toks[i-1].kind, toks[i+1].kind
		return (p == tLParen || p == tSep) && (n == tSep || n == tRParen)
	}
	// home : le tableau des [Colonne] sans nom de tableau, à cet endroit.
	home := func() string {
		for k := len(stack) - 1; k >= 0; k-- {
			if f := stack[k]; f.spill && f.table != "" && f.arg >= 1 {
				return f.table
			}
		}
		return self
	}
	// keys réécrit les crochets qui suivent une référence ([2], [Clé]) :
	// une expression comme une autre.
	keys := func(i int) int {
		for i < len(toks) && toks[i].kind == tRef {
			if neu, ok := renameBody(toks[i].text, loc, r, self); ok {
				replace(i, "["+neu+"]")
			}
			i++
		}
		return i
	}
	for i := 0; i < len(toks); {
		tk := toks[i]
		switch tk.kind {
		case tLParen:
			stack = append(stack, renameFrame{})
			i++
		case tSep:
			if f := top(); f != nil {
				f.arg++
			}
			i++
		case tRParen:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			i++
		case tRange:
			if sh, rest, ok := splitSheet(raw(i)); ok {
				if n := r.Sheet(sh); n != sh {
					replace(i, sheetPrefix(n)+rest)
				}
			}
			i++
		case tStr:
			if f := top(); f != nil && whole(i) && isLookup(f.fn) {
				switch f.arg {
				case 0:
					f.table = tk.text
					replace(i, quote(r.Table(tk.text)))
				case 1:
					replace(i, quote(r.Column(f.table, tk.text)))
				}
			}
			i++
		case tRef:
			// [Colonne] : la colonne du tableau de la formule.
			tbl := home()
			if f := top(); f != nil && f.spill && f.arg == 0 && whole(i) {
				f.table = tbl
			}
			replace(i, renameColumn(tk.text, r.Column(tbl, refColumn(tk.text))))
			i = keys(i + 1)
		case tIdent:
			next := toks[i+1].kind
			switch {
			case next == tLParen:
				fn := strings.ToUpper(tk.text)
				isSpill := false
				if def, ok := lookupFunc(tk.text); ok {
					fn, isSpill = def.EN, def.kind == spill
				}
				stack = append(stack, renameFrame{fn: fn, spill: isSpill})
				i += 2
			case next == tRef:
				sheet, word := "", tk.text
				if k := strings.LastIndex(tk.text, "!"); k >= 0 {
					sheet, word = tk.text[:k], tk.text[k+1:]
				}
				if strings.EqualFold(word, "cell") {
					// Feuille!Cell[A1] : la feuille.
					if sheet != "" {
						if n := r.Sheet(sheet); n != sheet {
							w := raw(i)
							replace(i, sheetPrefix(n)+w[strings.LastIndex(w, "!")+1:])
						}
					}
					i += 2
					continue
				}
				// Tableau[Colonne] : la colonne d'abord (sous l'ancien nom du
				// tableau), puis le tableau.
				if f := top(); f != nil && f.spill && f.arg == 0 {
					f.table = tk.text
				}
				replace(i+1, renameColumn(toks[i+1].text, r.Column(tk.text, refColumn(toks[i+1].text))))
				replace(i, r.Table(tk.text))
				i = keys(i + 2)
			default:
				f := top()
				if f != nil && f.fn == "LET" && f.arg%2 == 0 && whole(i) && toks[i+1].kind == tSep {
					bound[strings.ToLower(tk.text)] = true
				}
				switch {
				case bound[strings.ToLower(tk.text)]:
				case f != nil && whole(i) && (isLookup(f.fn) || f.fn == "EXISTS" || f.spill) && f.arg == 0:
					f.table = tk.text
					replace(i, r.Table(tk.text))
				case f != nil && whole(i) && isLookup(f.fn) && f.arg == 1:
					replace(i, r.Column(f.table, tk.text))
				default:
					replace(i, renameName(raw(i), tk.text, r))
				}
				i++
			}
		default:
			i++
		}
	}
	if len(edits) == 0 {
		return body, false
	}
	sort.Slice(edits, func(a, b int) bool { return edits[a].from < edits[b].from })
	var b strings.Builder
	at := 0
	for _, e := range edits {
		b.WriteString(string(rs[at:e.from]))
		b.WriteString(e.text)
		at = e.to
	}
	b.WriteString(string(rs[at:]))
	return b.String(), true
}

func isLookup(fn string) bool { return fn == "REFERENCE" || fn == "REFERENCE.APPROX" }

func quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

// refColumn lit le nom de colonne entre crochets : Col, @Col, [@Col], @[Col].
func refColumn(inner string) string { return parseRef("x", inner).Col }

// renameColumn réécrit le contenu des crochets d'une référence avec la
// colonne col (garde l'@ de la forme Excel) ; inchangé si col est le même.
func renameColumn(inner, col string) string {
	old := refColumn(inner)
	if col == old || strings.ContainsAny(col, "[]") {
		return "[" + inner + "]"
	}
	at := ""
	if strings.HasPrefix(strings.TrimSpace(inner), "@") {
		at = "@"
	}
	return "[" + at + col + "]"
}

// splitSheet sépare « Feuille! » (ou « 'Ma feuille'! ») du reste d'un jeton.
func splitSheet(text string) (sheet, rest string, ok bool) {
	k := strings.LastIndex(text, "!")
	if k <= 0 {
		return "", "", false
	}
	sheet, rest = text[:k], text[k+1:]
	if len(sheet) >= 2 && sheet[0] == '\'' && sheet[len(sheet)-1] == '\'' {
		sheet = strings.ReplaceAll(sheet[1:len(sheet)-1], "''", "'")
	}
	return sheet, rest, true
}

// renameName réécrit un nom seul : tableau (argument d'une fonction),
// cellule nommée, adresse d'une autre feuille (Feuille!B3, Feuille.Cell_B3).
func renameName(rawText, text string, r Renames) string {
	if n, ok := r.Names[strings.ToLower(text)]; ok {
		return n
	}
	if n, ok := r.Tables[strings.ToLower(text)]; ok {
		return n
	}
	if sh, rest, ok := splitSheet(rawText); ok {
		if n := r.Sheet(sh); n != sh {
			return sheetPrefix(n) + rest
		}
		return rawText
	}
	if k := strings.LastIndex(text, "."); k > 0 {
		if _, _, ok := CellRef(text[k+1:]); ok {
			if n := r.Sheet(text[:k]); n != text[:k] {
				return CellText(n + "." + text[k+1:])
			}
		}
	}
	return rawText
}
