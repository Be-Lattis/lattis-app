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

// nameValues calcule la valeur de chaque cellule nommée : une saisie est
// convertie selon son type inféré, une formule est calculée en SQL dans
// l'ordre des dépendances ; une référence circulaire donne #REF!.
func (t *tx) nameValues(schema *formula.Schema) (map[string]formula.Value, error) {
	rs, err := t.query(`SELECT n.name, coalesce(c.raw, '') FROM _slitable_names n
		LEFT JOIN _slitable_cells c ON c.sheet = n.sheet AND c.row = n.row AND c.col = n.col`)
	if err != nil {
		return nil, err
	}
	ns := &formula.Names{Formulas: map[string]formula.Node{}}
	for rs.Next() {
		var name, raw string
		if err := rs.Scan(&name, &raw); err != nil {
			rs.Close()
			return nil, err
		}
		ns.Formulas[strings.ToLower(name)] = literal(raw)
	}
	rs.Close()
	values := map[string]formula.Value{}
	order, cyclic := ns.Order()
	for _, n := range cyclic {
		values[n] = formula.Err(formula.ErrRef)
	}
	for _, n := range order {
		c := formula.Compile(ns.Formulas[n], schema, values, nil, "")
		var v any
		if err := t.row(`SELECT `+c.SQL, c.Args...).Scan(&v); err != nil {
			values[n] = formula.Err(formula.ErrValue)
			continue
		}
		values[n] = formula.FromResult(v)
	}
	return values, nil
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
