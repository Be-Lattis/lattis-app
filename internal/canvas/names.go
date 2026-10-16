package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/lalamefine/slitable/internal/formula"
	"github.com/lalamefine/slitable/internal/grid"
)

// ErrName signale un nom de cellule refusé.
type ErrName struct{ Name, Why string }

func (e ErrName) Error() string { return fmt.Sprintf("nom « %s » refusé : %s", e.Name, e.Why) }

var nameRe = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_.]*$`)

// SetName donne un nom à une cellule libre (vide : retire son nom). Un nom
// sert à la référencer dans les formules : =SOMME(Ventes[Montant]) / Objectif.
func (m *Model) SetName(ctx context.Context, sheet int64, row, col int, name string) error {
	name = strings.TrimSpace(name)
	return m.Do(ctx, "Nommer la cellule", func(t *tx) error {
		var old string
		err := t.row(`SELECT name FROM _slitable_names WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col).Scan(&old)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if old == name {
			return nil
		}
		if name != "" {
			if err := t.checkName(name, sheet, row, col); err != nil {
				return err
			}
		}
		del := st(`DELETE FROM _slitable_names WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col)
		put := func(n string) stmt {
			return st(`INSERT INTO _slitable_names (name, sheet, row, col) VALUES (?, ?, ?, ?)`, n, sheet, row, col)
		}
		do, undo := []stmt{del}, []stmt{del}
		if name != "" {
			do = append(do, put(name))
		}
		if old != "" {
			undo = append(undo, put(old))
		}
		return t.apply(do, undo)
	})
}

// checkName vérifie qu'un nom est utilisable dans une formule et libre.
func (t *tx) checkName(name string, sheet int64, row, col int) error {
	if !nameRe.MatchString(name) {
		return ErrName{name, "il doit commencer par une lettre et ne contenir ni espace ni symbole"}
	}
	switch strings.ToUpper(name) {
	case "VRAI", "FAUX", "TRUE", "FALSE":
		return ErrName{name, "mot réservé"}
	}
	if _, _, ok := formula.CellRef(name); ok {
		return ErrName{name, "c'est une adresse de cellule"}
	}
	for _, f := range formula.Functions(formula.FR) {
		if strings.EqualFold(f.Name, name) || strings.EqualFold(f.Other, name) {
			return ErrName{name, "c'est le nom d'une fonction"}
		}
	}
	var n int
	if err := t.row(`SELECT count(*) FROM _slitable_tables WHERE name = ? COLLATE NOCASE`, name).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrName{name, "c'est le nom d'un tableau"}
	}
	if err := t.row(`SELECT count(*) FROM _slitable_names WHERE name = ? AND NOT (sheet = ? AND row = ? AND col = ?)`, name, sheet, row, col).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrName{name, "déjà utilisé par une autre cellule"}
	}
	return nil
}

// moveNames fait suivre leurs noms aux cellules libres déplacées (deux temps,
// pour ne jamais occuper deux fois la même position).
func (t *tx) moveNames(sheet int64, moved []cell, dRow, dCol int) error {
	type named struct {
		name     string
		row, col int
	}
	var ns []named
	for _, c := range moved {
		var n string
		err := t.row(`SELECT name FROM _slitable_names WHERE sheet = ? AND row = ? AND col = ?`, sheet, c.Row, c.Col).Scan(&n)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		ns = append(ns, named{n, c.Row, c.Col})
	}
	if len(ns) == 0 {
		return nil
	}
	q := `UPDATE _slitable_names SET row = ?, col = ? WHERE name = ?`
	var do, undo []stmt
	for i, n := range ns {
		do = append(do, st(q, -1-i, -1, n.name))
		undo = append(undo, st(q, -1-i, -1, n.name))
	}
	for _, n := range ns {
		do = append(do, st(q, n.row+dRow, n.col+dCol, n.name))
		undo = append(undo, st(q, n.row, n.col, n.name))
	}
	return t.apply(do, undo)
}

// nameValues calcule la valeur de chaque cellule nommée et, si une formule
// en cite, de chaque cellule libre (adresses Cell_A1, clés formula.CellKey,
// alias « Feuille.Cell_A1 ») : une saisie est convertie selon son type
// inféré, une formule est calculée en SQL dans l'ordre des dépendances (y
// compris celles des colonnes calculées des tableaux cités) ; une référence
// circulaire donne #REF!.
func (t *tx) nameValues(schema *formula.Schema) (map[string]formula.Value, error) {
	return cached(t, "names", func() (map[string]formula.Value, error) { return t.nameValuesRaw(schema) })
}

func (t *tx) nameValuesRaw(schema *formula.Schema) (map[string]formula.Value, error) {
	return t.nameValuesWith(schema, false)
}

// nameValuesWith : allCells charge les cellules libres même si aucune
// formule enregistrée ne cite d'adresse (requêtes de l'onglet SQL).
func (t *tx) nameValuesWith(schema *formula.Schema, allCells bool) (map[string]formula.Value, error) {
	ns := &formula.Names{Formulas: map[string]formula.Node{}, Extra: map[string][]string{}}
	rs, err := t.query(`SELECT n.name, n.sheet, coalesce(c.raw, '') FROM _slitable_names n
		LEFT JOIN _slitable_cells c ON c.sheet = n.sheet AND c.row = n.row AND c.col = n.col`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var name, raw string
		var sheet int64
		if err := rs.Scan(&name, &sheet, &raw); err != nil {
			rs.Close()
			return nil, err
		}
		ns.Formulas[strings.ToLower(name)] = formula.QualifyCells(literal(raw), sheet)
	}
	rs.Close()
	cited := allCells
	if !cited {
		if err := t.row(`SELECT EXISTS (SELECT 1 FROM _slitable_cells WHERE raw LIKE '%cell%')
			OR EXISTS (SELECT 1 FROM _slitable_columns WHERE formula LIKE '%cell%')`).Scan(&cited); err != nil {
			return nil, err
		}
	}
	if cited {
		if err := t.cellFormulas(ns, schema); err != nil {
			return nil, err
		}
	}
	values := map[string]formula.Value{}
	order, cyclic := ns.Order()
	for _, n := range cyclic {
		values[n] = formula.Err(formula.ErrRef)
	}
	for _, n := range order {
		hoist, _ := t.hoister()
		c := formula.CompileHoisted(ns.Formulas[n], schema, values, nil, "", hoist)
		var v any
		if err := t.row(`SELECT `+c.SQL, c.Args...).Scan(&v); err != nil {
			values[n] = formula.Err(formula.ErrValue)
			continue
		}
		values[n] = formula.FromResult(v)
	}
	return values, nil
}

// cellFormulas ajoute aux noms les cellules libres de toutes les feuilles,
// et les dépendances qui passent par les colonnes calculées des tableaux.
func (t *tx) cellFormulas(ns *formula.Names, schema *formula.Schema) error {
	sheetNames := map[int64]string{}
	rs, err := t.query(`SELECT id, name FROM _slitable_sheets`)
	if err != nil {
		return err
	}
	for rs.Next() {
		var id int64
		var name string
		if err := rs.Scan(&id, &name); err != nil {
			rs.Close()
			return err
		}
		sheetNames[id] = strings.ToLower(name)
	}
	rs.Close()
	rs, err = t.query(`SELECT sheet, row, col, raw FROM _slitable_cells`)
	if err != nil {
		return err
	}
	for rs.Next() {
		var sheet int64
		var row, col int
		var raw string
		if err := rs.Scan(&sheet, &row, &col, &raw); err != nil {
			rs.Close()
			return err
		}
		n := literal(raw)
		if isFormula(raw) {
			if c, _, err := formula.Convert(n, schema, noRanges{}, raw); err == nil {
				n = c
			}
		}
		key := formula.CellKey(sheet, row, col)
		ns.Formulas[key] = formula.QualifyCells(n, sheet)
		ns.Formulas[sheetNames[sheet]+"."+strings.ToLower(formula.CellName(row, col))] = formula.Name{Name: key}
	}
	rs.Close()
	// Noms lus par les colonnes calculées de chaque tableau, et tableaux
	// qu'elles citent (fermeture transitive ensuite).
	reads := map[string][]string{}
	cites := map[string][]string{}
	for _, ft := range schema.Tables {
		k := strings.ToLower(ft.Name)
		for _, c := range ft.Cols {
			if c.Formula == nil {
				continue
			}
			names, tables := formula.Deps(formula.QualifyCells(c.Formula, ft.Sheet))
			reads[k] = append(reads[k], names...)
			cites[k] = append(cites[k], tables...)
		}
	}
	for key, n := range ns.Formulas {
		_, tables := formula.Deps(n)
		seen := map[string]bool{}
		for len(tables) > 0 {
			tb := tables[0]
			tables = tables[1:]
			if seen[tb] {
				continue
			}
			seen[tb] = true
			ns.Extra[key] = append(ns.Extra[key], reads[tb]...)
			tables = append(tables, cites[tb]...)
		}
	}
	return nil
}

// literal donne le nœud d'une saisie : formule analysée, nombre, booléen ou
// texte selon le type inféré.
func literal(raw string) formula.Node {
	if isFormula(raw) {
		n, err := formula.Parse(raw, formula.FR)
		if err != nil {
			return formula.ErrLit{V: formula.ErrName}
		}
		return n
	}
	switch Infer(raw) {
	case grid.Integer, grid.Decimal:
		if v, err := grid.Parse(grid.Decimal, raw); err == nil && v != nil {
			return formula.NumLit{V: v.(float64)}
		}
	case grid.Boolean:
		v, _ := grid.Parse(grid.Boolean, raw)
		return formula.BoolLit{V: v == int64(1)}
	}
	return formula.StrLit{V: raw}
}
