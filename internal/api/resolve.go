package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/doc"
	"github.com/lalamefine/slitable/internal/formula"
	"github.com/lalamefine/slitable/internal/grid"
)

// sheetRef est une feuille du classeur.
type sheetRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func sheetList(ctx context.Context, d *doc.Document) ([]sheetRef, error) {
	rs, err := d.DB().Read().QueryContext(ctx, `SELECT id, name FROM _slitable_sheets ORDER BY pos, id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []sheetRef
	for rs.Next() {
		var s sheetRef
		if err := rs.Scan(&s.ID, &s.Name); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rs.Err()
}

// sheet résout le paramètre name : nom (sans tenir compte de la casse) ou
// identifiant ; absent, la feuille affichée.
func (c *Call) sheet(name string) (sheetRef, error) {
	d, err := c.Doc()
	if err != nil {
		return sheetRef{}, err
	}
	list, err := sheetList(c, d)
	if err != nil {
		return sheetRef{}, err
	}
	if len(list) == 0 {
		return sheetRef{}, fmt.Errorf("le classeur n'a pas de feuille")
	}
	v := c.args[name]
	if v == nil {
		cur := c.Host.Selection().Sheet
		for _, s := range list {
			if s.ID == cur {
				return s, nil
			}
		}
		return list[0], nil
	}
	if s, ok := v.(string); ok {
		for _, x := range list {
			if x.Name == s {
				return x, nil
			}
		}
		for _, x := range list {
			if strings.EqualFold(x.Name, s) {
				return x, nil
			}
		}
		return sheetRef{}, fmt.Errorf("feuille « %s » introuvable", s)
	}
	if f, ok := number(v); ok {
		for _, x := range list {
			if float64(x.ID) == f {
				return x, nil
			}
		}
		return sheetRef{}, fmt.Errorf("feuille %v introuvable", v)
	}
	return sheetRef{}, fmt.Errorf("%s : nom de feuille attendu", name)
}

// tableRef est un tableau et sa feuille.
type tableRef struct {
	canvas.TableInfo
	Sheet sheetRef
}

// table résout le paramètre name (nom du tableau, la casse exacte d'abord).
func (c *Call) table(name string) (tableRef, error) {
	s, err := c.Str(name)
	if err != nil {
		return tableRef{}, err
	}
	return findTable(c, c, s)
}

func findTable(ctx context.Context, c *Call, name string) (tableRef, error) {
	d, err := c.Doc()
	if err != nil {
		return tableRef{}, err
	}
	all, err := d.AllTables(ctx)
	if err != nil {
		return tableRef{}, err
	}
	var hit *canvas.TableNames
	for i := range all {
		if all[i].Kind == "table" && all[i].Name == name {
			hit = &all[i]
			break
		}
	}
	if hit == nil {
		for i := range all {
			if all[i].Kind == "table" && strings.EqualFold(all[i].Name, name) {
				hit = &all[i]
				break
			}
		}
	}
	if hit == nil {
		return tableRef{}, fmt.Errorf("tableau « %s » introuvable", name)
	}
	l, err := d.Layout(ctx, hit.Sheet)
	if err != nil {
		return tableRef{}, err
	}
	for _, t := range l.Tables {
		if t.Name == hit.Name {
			ref := tableRef{TableInfo: t, Sheet: sheetRef{ID: hit.Sheet}}
			for _, s := range l.Sheets {
				if s.ID == hit.Sheet {
					ref.Sheet.Name = s.Name
				}
			}
			return ref, nil
		}
	}
	return tableRef{}, fmt.Errorf("tableau « %s » introuvable", name)
}

// column résout une colonne du tableau t : nom ou rang (à partir de 1).
// Renvoie son rang à partir de 0.
func (c *Call) column(t tableRef, name string) (int, error) {
	v := c.args[name]
	if s, ok := v.(string); ok {
		if i, err := strconv.Atoi(s); err == nil {
			v = float64(i)
		} else {
			for i, col := range t.Columns {
				if col.Name == s {
					return i, nil
				}
			}
			for i, col := range t.Columns {
				if strings.EqualFold(col.Name, s) {
					return i, nil
				}
			}
			return 0, fmt.Errorf("tableau « %s » : pas de colonne « %s »", t.Name, s)
		}
	}
	f, ok := number(v)
	if !ok {
		return 0, fmt.Errorf("%s : nom ou rang de colonne attendu", name)
	}
	if f < 1 || int(f) > len(t.Columns) {
		return 0, fmt.Errorf("tableau « %s » : colonne %v hors du tableau (1 à %d)", t.Name, v, len(t.Columns))
	}
	return int(f) - 1, nil
}

// columns résout une liste de colonnes (absente : toutes).
func (c *Call) columns(t tableRef, name string) ([]int, error) {
	if !c.Has(name) {
		out := make([]int, len(t.Columns))
		for i := range out {
			out[i] = i
		}
		return out, nil
	}
	list, ok := c.args[name].([]any)
	if !ok {
		list = []any{c.args[name]}
	}
	var out []int
	for _, v := range list {
		sub := &Call{Context: c.Context, Host: c.Host, cmd: c.cmd, args: map[string]any{"_": v}, doc: c.doc}
		i, err := sub.column(t, "_")
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, nil
}

// --- Descriptions renvoyées (positions à partir de 1) -------------------------

// Column décrit une colonne de tableau.
type Column struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Fixed   bool   `json:"fixed"`
	Formula string `json:"formula,omitempty"`
	Errors  int    `json:"errors,omitempty"`
	Key     bool   `json:"key,omitempty"`
}

// Table décrit un tableau.
type Table struct {
	Name    string   `json:"name"`
	Sheet   string   `json:"sheet"`
	Row     int      `json:"row"` // ligne d'en-tête sur la feuille
	Col     int      `json:"col"`
	Rows    int      `json:"rows"`
	Hidden  int      `json:"hidden,omitempty"` // lignes masquées par un filtre
	Columns []Column `json:"columns"`
}

func describeTable(t canvas.TableInfo, sheet string) Table {
	out := Table{Name: t.Name, Sheet: sheet, Row: t.Row + 1, Col: t.Col + 1, Rows: t.Rows, Hidden: t.Total - t.Rows, Columns: []Column{}}
	for i, c := range t.Columns {
		out.Columns = append(out.Columns, Column{Name: c.Name, Type: string(c.Type), Fixed: c.Fixed, Formula: c.Formula, Errors: c.Errors, Key: i == t.Key})
	}
	return out
}

// loadGrid prépare la lecture par fenêtres d'un tableau (valeurs affichées,
// colonnes calculées comprises). L'interface s'en sert aussi.
func LoadGrid(ctx context.Context, d *doc.Document, ti canvas.TableInfo) (*grid.Table, error) {
	cols := make([]grid.Column, len(ti.Columns))
	sqlCols := make([]string, len(ti.Columns))
	ids := make([]int64, len(ti.Columns))
	for i, c := range ti.Columns {
		cols[i], sqlCols[i], ids[i] = grid.Column{Name: c.Name, Type: c.Type}, c.SQL, c.ID
	}
	exprs, args, computed, err := d.ColumnExprs(ctx, ti.ID)
	if err != nil {
		return nil, err
	}
	decode := func(c int, v any) any {
		if c < len(computed) && computed[c] {
			return DisplayValue(formula.FromResult(v))
		}
		if b, ok := v.([]byte); ok {
			return string(b)
		}
		return v
	}
	t := grid.NewTable(d.DB(), ti.Source, ti.Name, cols, sqlCols).WithInvalid(ti.ID, ids).WithExprs(exprs, args, decode)
	if err := t.Load(ctx); err != nil {
		return nil, err
	}
	return t, nil
}

// DisplayValue convertit une valeur calculée : nombre, texte, booléen, ou le
// libellé d'une erreur (« #DIV/0! »).
func DisplayValue(v formula.Value) any {
	switch v.Kind {
	case formula.Number:
		return v.Num
	case formula.Text:
		return v.Str
	case formula.Bool:
		return v.Bool
	case formula.Error:
		return v.Err.String()
	}
	return nil
}
