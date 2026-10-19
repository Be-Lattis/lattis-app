package api

import (
	"context"
	"fmt"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/transfer"
)

var (
	pRow  = Param{Name: "row", Type: Integer, Desc: "ligne de la feuille, à partir de 1"}
	pCol  = Param{Name: "col", Type: Integer, Desc: "colonne de la feuille, à partir de 1"}
	pRow2 = Param{Name: "row2", Type: Integer, Desc: "dernière ligne de la plage (absente : row)", Optional: true}
	pCol2 = Param{Name: "col2", Type: Integer, Desc: "dernière colonne de la plage (absente : col)", Optional: true}
)

// MaxClearCells borne cells.clear.
const MaxClearCells = 10_000

// CellValue est le contenu d'une cellule de la feuille.
type CellValue struct {
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Raw    string `json:"raw"`             // saisie (formule comprise)
	Value  any    `json:"value"`           // valeur affichée
	Table  string `json:"table,omitempty"` // tableau qui contient la cellule
	Column string `json:"column,omitempty"`
	Header bool   `json:"header,omitempty"` // ligne d'en-tête du tableau
	Line   int    `json:"line,omitempty"`   // ligne de données du tableau, à partir de 1
	ID     int64  `json:"id,omitempty"`     // clé _id de la ligne
}

// rect lit une plage row, col, row2, col2 (à partir de 1) et la renvoie à
// partir de 0, dans l'ordre.
func (c *Call) rect() (r0, c0, r1, c1 int, err error) {
	if r0, err = c.Pos("row", 0); err != nil {
		return
	}
	if c0, err = c.Pos("col", 0); err != nil {
		return
	}
	if r1, err = c.Pos("row2", r0); err != nil {
		return
	}
	if c1, err = c.Pos("col2", c0); err != nil {
		return
	}
	r0, r1 = min(r0, r1), max(r0, r1)
	c0, c1 = min(c0, c1), max(c0, c1)
	return
}

// tableAt trouve le tableau qui contient (row, col), en-tête compris.
func tableAt(l canvas.Layout, row, col int) *canvas.TableInfo {
	for i, t := range l.Tables {
		if row >= t.Row && row <= t.Row+t.Rows && col >= t.Col && col < t.Col+len(t.Columns) {
			return &l.Tables[i]
		}
	}
	return nil
}

func readCell(ctx context.Context, d *doc.Document, l canvas.Layout, row, col int) (CellValue, error) {
	v := CellValue{Row: row + 1, Col: col + 1}
	if t := tableAt(l, row, col); t != nil {
		ci := col - t.Col
		v.Table, v.Column = t.Name, t.Columns[ci].Name
		if row == t.Row {
			v.Header, v.Raw, v.Value = true, t.Columns[ci].Name, t.Columns[ci].Name
			return v, nil
		}
		g, err := LoadGrid(ctx, d, *t)
		if err != nil {
			return v, err
		}
		n := row - t.Row - 1
		w, err := g.Window(ctx, n, 1)
		if err != nil {
			return v, err
		}
		v.Line = n + 1
		if len(w.Cells) > 0 {
			v.Value, v.ID = w.Cells[0][ci], w.IDs[0]
			v.Raw = raw(v.Value)
			for _, bad := range w.Invalid {
				if bad.Col == ci {
					v.Raw, v.Value = bad.Raw, bad.Raw
				}
			}
			if f := t.Columns[ci].Formula; f != "" {
				v.Raw = f
			}
		}
		return v, nil
	}
	for _, cell := range l.Cells {
		if cell.Row == row && cell.Col == col {
			v.Raw, v.Value = cell.Raw, cell.Raw
			if cell.Value != "" || cell.Raw != "" && cell.Raw[0] == '=' {
				v.Value = cell.Value
			}
			return v, nil
		}
	}
	return v, nil
}

func init() {
	register(
		&Command{
			Name:    "cells.get",
			Summary: "Lit une cellule de la feuille : saisie, valeur affichée et, dans un tableau, le tableau, la colonne et la ligne.",
			Returns: "{row, col, raw, value, table, column, header, line, id}",
			Params:  []Param{pRow, pCol, pSheet},
			Example: `local c = cells.get(2, 3)
print(c.raw, c.value, c.table, c.column)`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				l, err := d.Layout(c, s.ID)
				if err != nil {
					return nil, err
				}
				return readCell(c, d, l, row, col)
			},
		},
		&Command{
			Name:    "cells.text_to_columns",
			Label:   "Convertir",
			Summary: "Convertit le texte des cellules de la colonne col, lignes row à row2, en colonnes (assistant Convertir d'Excel) : dans des cellules libres, les parties s'écrivent vers la droite (ou à partir de options.dest) ; dans un tableau, de nouvelles colonnes sont insérées. Renvoie le nombre de colonnes importées.",
			Details: textColumnsHelp,
			Params:  []Param{pRow, pCol, pRow2, {Name: "options", Type: Object, Desc: "{delims, merge, quote, fixed, breaks, columns, decimal, thousands, dest}"}, pSheet},
			Write:   true,
			Example: `cells.text_to_columns(2, 1, 20, {delims = ",", quote = "\""})`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				row2, err := c.Pos("row2", row)
				if err != nil {
					return nil, err
				}
				var o canvas.TextColumnsOptions
				if err := c.Decode("options", &o); err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return d.TextToColumns(c, s.ID, row, row2, col, o)
			},
		},
		&Command{
			Name:    "cells.set",
			Label:   "Saisie",
			Summary: "Saisit une valeur dans une cellule de la feuille, exactement comme une frappe dans la grille.",
			Details: "Un texte qui commence par = est une formule. Saisir à côté d'un tableau l'agrandit, saisir deux lignes contiguës crée un tableau, \"\" efface la cellule. Renvoie ce que la saisie a provoqué (tableau créé, colonne élargie…).",
			Returns: "{created, widened, from, to, invalid, keyed}",
			Params:  []Param{pRow, pCol, {Name: "value", Type: Value, Desc: "saisie : texte, nombre, booléen, formule (=…)"}, pSheet},
			Write:   true,
			Example: `cells.set(1, 1, "Produit")
cells.set(1, 2, "Prix")
cells.set(2, 1, "Café")
cells.set(2, 2, 3.5)`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return d.SetCell(c, s.ID, row, col, raw(c.Raw("value")))
			},
		},
		&Command{
			Name:    "cells.paste",
			Label:   "Coller",
			Summary: "Écrit un bloc de valeurs à partir d'une cellule, comme un collage : une ligne d'en-têtes suivie de lignes de données crée un tableau ; range = true l'écrit en plage de cellules libres.",
			Params:  []Param{pRow, pCol, {Name: "values", Type: Array, Items: Array, Desc: "lignes de valeurs : { {\"Nom\", \"Âge\"}, {\"Léa\", 31} }"}, {Name: "range", Type: Boolean, Desc: "vrai : des cellules libres (une plage), sans créer de tableau", Optional: true}, pSheet},
			Write:   true,
			Example: `cells.paste(1, 1, {
  {"Produit", "Prix"},
  {"Café", 3.5},
  {"Thé", 2.8},
})`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				var rows [][]any
				if err := c.Decode("values", &rows); err != nil {
					return nil, err
				}
				block := make([][]string, len(rows))
				for i, r := range rows {
					block[i] = make([]string, len(r))
					for j, v := range r {
						block[i][j] = raw(v)
					}
				}
				d, _ := c.Doc()
				asRange, err := c.Bool("range")
				if err != nil {
					return nil, err
				}
				if asRange {
					return d.PasteRange(c, s.ID, row, col, block)
				}
				return d.Paste(c, s.ID, row, col, block)
			},
		},
		&Command{
			Name:    "cells.read",
			Summary: "Lit les valeurs affichées d'une plage de la feuille, ligne par ligne : tableaux, tableaux croisés et cellules libres confondus (\"\" : cellule vide).",
			Details: "100 000 cellules au plus ; pour tout un tableau, tables.rows ou db.query sont plus directs.",
			Returns: "{ {valeur, ...}, ... }",
			Params:  []Param{pRow, pCol, {Name: "row2", Type: Integer, Desc: "dernière ligne de la plage"}, {Name: "col2", Type: Integer, Desc: "dernière colonne de la plage"}, pSheet},
			Example: `for _, ligne in ipairs(cells.read(1, 1, 10, 4)) do
  print(table.concat(ligne, " | "))
end`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				var p [4]int
				for i, n := range []string{"row", "col", "row2", "col2"} {
					if p[i], err = c.Pos(n, 0); err != nil {
						return nil, err
					}
				}
				d, _ := c.Doc()
				return transfer.SheetValues(c, d.Model, s.ID, p[0], p[1], p[2], p[3])
			},
		},
		&Command{
			Name:    "cells.clear",
			Label:   "Effacer",
			Summary: "Efface le contenu d'une plage de cellules (les tableaux gardent leur forme).",
			Params:  []Param{pRow, pCol, pRow2, pCol2, pSheet},
			Write:   true,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				r0, c0, r1, c1, err := c.rect()
				if err != nil {
					return nil, err
				}
				if (r1-r0+1)*(c1-c0+1) > MaxClearCells {
					return nil, fmt.Errorf("plage trop grande (%d cellules au plus)", MaxClearCells)
				}
				d, _ := c.Doc()
				l, err := d.Layout(c, s.ID)
				if err != nil {
					return nil, err
				}
				for r := r0; r <= r1; r++ {
					for k := c0; k <= c1; k++ {
						if t := tableAt(l, r, k); t != nil && r == t.Row {
							continue // un en-tête ne s'efface pas
						}
						if _, err := d.SetCell(c, s.ID, r, k, ""); err != nil {
							return nil, err
						}
					}
				}
				return nil, nil
			},
		},
		&Command{
			Name:    "cells.style",
			Label:   "Mise en forme",
			Summary: "Met en forme une plage de cellules.",
			Details: styleHelp,
			Params:  []Param{pRow, pCol, pRow2, pCol2, {Name: "style", Type: Object, Desc: "propriétés à poser : {bold = true, fill = \"#fff2cc\"}"}, pSheet},
			Write:   true,
			Example: `cells.style{row = 1, col = 1, col2 = 4, style = {bold = true, fill = "#fff2cc"}}`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				r0, c0, r1, c1, err := c.rect()
				if err != nil {
					return nil, err
				}
				st, err := c.style("style")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.ApplyStyle(c, s.ID, r0, c0, r1, c1, st)
			},
		},
		&Command{
			Name:    "cells.style_lines",
			Label:   "Mise en forme des lignes ou colonnes",
			Summary: "Met en forme des lignes ou des colonnes entières de la feuille (toutes leurs cellules, présentes et à venir, tableaux et tableaux croisés compris). Une cellule l'emporte sur sa ligne, une ligne sur sa colonne.",
			Details: styleHelp,
			Params: []Param{{Name: "axis", Type: String, Desc: `"rows" (lignes) ou "cols" (colonnes)`},
				{Name: "from", Type: Integer, Desc: "première ligne ou colonne, à partir de 1"},
				{Name: "to", Type: Integer, Desc: "dernière ligne ou colonne (absente : from)", Optional: true},
				{Name: "style", Type: Object, Desc: "propriétés à poser : {bold = true, fill = \"#fff2cc\"}"}, pSheet},
			Write:   true,
			Example: `cells.style_lines{axis = "rows", from = 1, style = {bold = true}}`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				axis, err := c.Str("axis")
				if err != nil {
					return nil, err
				}
				if axis != "rows" && axis != "cols" {
					return nil, fmt.Errorf(`axis : "rows" ou "cols" attendu`)
				}
				i0, err := c.Pos("from", 0)
				if err != nil {
					return nil, err
				}
				i1, err := c.Pos("to", i0)
				if err != nil {
					return nil, err
				}
				st, err := c.style("style")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.ApplyLineStyle(c, s.ID, axis == "rows", i0, i1, st)
			},
		},
		&Command{
			Name:    "cells.merge",
			Label:   "Fusionner et centrer",
			Summary: "Fusionne des cellules libres en une seule, au contenu centré (refusé dans un tableau ou un tableau croisé). Comme Excel, seule la valeur en haut à gauche est gardée. " +
				"Dans un tableau aux groupes d'en-têtes (tables.two_level_headers), seule la bande des groupes se fusionne : sa case et l'en-tête d'une colonne mettent la colonne sur un niveau (nom simple : son sous-en-tête, ou le groupe si l'en-tête était automatique) ; plusieurs cases voisines de la bande forment un seul groupe (le premier qu'elles portent, « Groupe » sinon). Les colonnes sont renommées, les formules suivent.",
			Returns: "nombre de valeurs effacées",
			Params:  []Param{pRow, pCol, pRow2, pCol2, pSheet},
			Write:   true,
			Example: `cells.merge{row = 40, col = 12, col2 = 15}`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				r0, c0, r1, c1, err := c.rect()
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return d.MergeCells(c, s.ID, r0, c0, r1, c1)
			},
		},
		&Command{
			Name:    "cells.unmerge",
			Label:   "Annuler la fusion",
			Summary: "Annule les fusions de cellules qui touchent la plage. Dans la bande des groupes d'en-têtes d'un tableau, un groupe sur plusieurs colonnes reste sur la première, les autres passent sur un niveau (renommées) ; une colonne sur un niveau ne change pas : posez-lui un groupe avec cells.set sur la bande ou columns.rename(« Groupe.Sous »).",
			Params:  []Param{pRow, pCol, pRow2, pCol2, pSheet},
			Write:   true,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				r0, c0, r1, c1, err := c.rect()
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.UnmergeCells(c, s.ID, r0, c0, r1, c1)
			},
		},
		&Command{
			Name:    "cells.note",
			Label:   "Note",
			Summary: "Pose la note d'une cellule (coin rouge, affichée au survol) ; un texte vide la retire. Dans un tableau, la note suit sa ligne.",
			Params:  []Param{pRow, pCol, {Name: "text", Type: String, Desc: "texte de la note (vide : la retirer)"}, pSheet},
			Write:   true,
			Example: `cells.note{row = 2, col = 3, text = "Chiffre provisoire"}`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				text, _ := c.Str("text")
				d, _ := c.Doc()
				return nil, d.SetNote(c, s.ID, row, col, text)
			},
		},
		&Command{
			Name:    "cells.name",
			Label:   "Nommer la cellule",
			Summary: "Nomme une cellule libre, pour la citer dans les formules ; un nom vide retire le nom.",
			Params:  []Param{pRow, pCol, {Name: "name", Type: String, Desc: "nom (vide : retirer)"}, pSheet},
			Write:   true,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				name, _ := c.Str("name")
				d, _ := c.Doc()
				return nil, d.SetName(c, s.ID, row, col, name)
			},
		},
		&Command{
			Name:    "cells.dependencies",
			Summary: "Donne les antécédents (ce que lit la formule de la cellule) et les dépendants (les formules qui la lisent).",
			Returns: "{subject, precedents, dependents}",
			Params:  []Param{pRow, pCol, pSheet},
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				deps, err := d.Dependencies(c, s.ID, row, col)
				for _, l := range [][]canvas.DepTarget{deps.Precedents, deps.Dependents} {
					for i := range l {
						l[i].Row++
						l[i].Col++
					}
				}
				return deps, err
			},
		},
	)
}

const styleHelp = `style : bold, italic, underline, strike (booléens), font (police) et size (taille en points), color et fill (couleurs "#rrggbb"), align ("left", "center" ou "right"), valign ("top", "middle" ou "bottom"), wrap (renvoyer à la ligne), format ("0", "0,00", "%", "€", "date", "time" ou "text"), decimals (nombre de décimales), no_grouping (sans séparateur de milliers), date_format (motif d'une date ou d'une heure : "jj/mm/aaaa", "jj mmm aaaa", "aaaa-mm-jj", "hh:mm"…), borders (les quatre bordures) ou border_top, border_bottom, border_left, border_right, border_color, et validation (liste des valeurs permises, liste déroulante). false ou "" retire la propriété.`

// styleKeys traduit les noms lisibles des propriétés de mise en forme vers
// les clés du classeur (internal/canvas/styles.go).
var styleKeys = map[string]string{
	"bold": "b", "italic": "i", "underline": "u", "color": "fg", "fill": "bg",
	"align": "al", "format": "nf", "decimals": "dp",
	"border_top": "bt", "border_bottom": "bb", "border_left": "bl", "border_right": "br",
	"strike": "x", "font": "ff", "size": "fs", "valign": "va", "wrap": "wr", "no_grouping": "ng",
	"date_format": "df", "border_color": "bc", "validation": "dv",
}

var styleRaw = map[string]bool{"b": true, "i": true, "u": true, "x": true, "ff": true, "fs": true, "fg": true, "bg": true, "al": true, "va": true, "wr": true,
	"nf": true, "dp": true, "ng": true, "df": true, "bt": true, "bb": true, "bl": true, "br": true, "bc": true, "dv": true}

func toStyle(m map[string]any) (canvas.Style, error) {
	st := canvas.Style{}
	for k, v := range m {
		if k == "borders" {
			for _, b := range []string{"bt", "bb", "bl", "br"} {
				st[b] = v
			}
			continue
		}
		if key, ok := styleKeys[k]; ok {
			k = key
		} else if !styleRaw[k] {
			return nil, fmt.Errorf("propriété de style %q inconnue", k)
		}
		st[k] = v
	}
	return st, nil
}

// style lit un paramètre de mise en forme.
func (c *Call) style(name string) (canvas.Style, error) {
	var m map[string]any
	if err := c.Decode(name, &m); err != nil {
		return nil, err
	}
	return toStyle(m)
}
