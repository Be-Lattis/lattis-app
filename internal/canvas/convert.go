package canvas

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/work"
	"github.com/ncruces/go-sqlite3"
)

// Register déclare sur une connexion les fonctions SQL du modèle :
//
//	slt_convert(v, de, vers) : convertit une valeur stockée d'un type logique à
//	                           un autre, NULL si elle ne se convertit pas ;
//	slt_text(v, type)        : la valeur telle que l'utilisateur la lit.
//
// Les conversions passent par les mêmes parseurs que la saisie (locale
// française) : « 12,5 » devient 12.5, 12.5 redevient « 12,5 ».
func Register(c *sqlite3.Conn) error {
	// Les fonctions du moteur de formules (slt_add, slt_fn…) aussi.
	if err := formula.Register(c); err != nil {
		return err
	}
	if err := c.CreateFunction("slt_convert", 3, sqlite3.DETERMINISTIC|sqlite3.INNOCUOUS, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		v, ok := convert(value(arg[0]), grid.Type(arg[1].Text()), grid.Type(arg[2].Text()))
		if !ok {
			ctx.ResultNull()
			return
		}
		result(ctx, v)
	}); err != nil {
		return err
	}
	// slt_fold(t) : texte replié pour comparer les valeurs d'un ordre
	// personnalisé (customsort.go).
	if err := c.CreateFunction(FoldFunc, 1, sqlite3.DETERMINISTIC|sqlite3.INNOCUOUS, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		if arg[0].Type() == sqlite3.NULL {
			ctx.ResultNull()
			return
		}
		ctx.ResultText(Fold(arg[0].Text()))
	}); err != nil {
		return err
	}
	return c.CreateFunction("slt_text", 2, sqlite3.DETERMINISTIC|sqlite3.INNOCUOUS, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		v := value(arg[0])
		if v == nil {
			ctx.ResultNull()
			return
		}
		ctx.ResultText(text(v, grid.Type(arg[1].Text())))
	})
}

func value(a sqlite3.Value) any {
	switch a.Type() {
	case sqlite3.INTEGER:
		return a.Int64()
	case sqlite3.FLOAT:
		return a.Float()
	case sqlite3.TEXT:
		return a.Text()
	}
	return nil
}

func result(ctx sqlite3.Context, v any) {
	switch v := v.(type) {
	case int64:
		ctx.ResultInt64(v)
	case float64:
		ctx.ResultFloat(v)
	case string:
		ctx.ResultText(v)
	default:
		ctx.ResultNull()
	}
}

// text écrit une valeur stockée comme l'utilisateur la saisirait.
func text(v any, t grid.Type) string {
	switch v := v.(type) {
	case int64:
		if t == grid.Boolean {
			if v != 0 {
				return "VRAI"
			}
			return "FAUX"
		}
		return strconv.FormatInt(v, 10)
	case float64:
		return strings.Replace(strconv.FormatFloat(v, 'f', -1, 64), ".", ",", 1)
	case string:
		if t == grid.Date {
			return grid.ShowStored(v)
		}
		return v
	}
	return fmt.Sprint(v)
}

// convert convertit une valeur stockée du type from au type to.
func convert(v any, from, to grid.Type) (any, bool) {
	if v == nil {
		return nil, true
	}
	if from == to {
		return v, true
	}
	switch to {
	case grid.Text:
		return text(v, from), true
	case grid.Decimal:
		if n, ok := v.(int64); ok {
			return float64(n), true
		}
	case grid.Integer:
		switch n := v.(type) {
		case int64:
			return n, true
		case float64:
			if n == math.Trunc(n) && math.Abs(n) < 1<<53 {
				return int64(n), true
			}
			return nil, false
		}
	case grid.Boolean:
		switch n := v.(type) {
		case int64:
			if n == 0 || n == 1 {
				return n, true
			}
			return nil, false
		case float64:
			if n == 0 || n == 1 {
				return int64(n), true
			}
			return nil, false
		}
	}
	if s, ok := v.(string); ok {
		if from == grid.Date && to != grid.Text {
			return nil, false
		}
		out, err := grid.Parse(to, s)
		if err != nil || out == nil {
			return nil, false
		}
		return out, true
	}
	return nil, false
}

// convertExpr est l'expression SQL de conversion d'une colonne.
func convertExpr(col string, from, to grid.Type) string {
	if from == to {
		return col
	}
	return fmt.Sprintf("slt_convert(%s, '%s', '%s')", col, from, to)
}

// ConvertPreview compte les valeurs d'une colonne qui ne se convertiraient
// pas au type to (elles seraient marquées en erreur).
func (m *Model) ConvertPreview(ctx context.Context, table string, i int, to grid.Type) (int, error) {
	var n int
	err := m.read(ctx, func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		if i < 0 || i >= len(tb.Cols) {
			return msg.New("table.columnIndexOutOfRange", msg.Args{"n": i})
		}
		c := tb.Cols[i]
		col := colSQL(c.ID)
		return t.row(`SELECT count(*) FROM ` + tb.sql() + ` WHERE ` + col + ` IS NOT NULL AND ` + convertExpr(col, c.Type, to) + ` IS NULL`).Scan(&n)
	})
	return n, err
}

// Mix décrit une colonne passée en texte par élargissement alors que la
// plupart de ses valeurs sont d'un autre type (Type vide : pas de tel type).
type Mix struct {
	Type  grid.Type `json:"type"`  // type majoritaire
	Off   int       `json:"off"`   // valeurs qui n'en sont pas
	Total int       `json:"total"` // valeurs non vides
}

// TypeMix cherche, pour une colonne en texte au type inféré, le type que
// plus de la moitié de ses valeurs accepteraient (le plus étroit à égalité),
// pour signaler les valeurs qui ont provoqué l'élargissement.
func (m *Model) TypeMix(ctx context.Context, table string, i int) (Mix, error) {
	var mix Mix
	err := m.read(ctx, func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		if i < 0 || i >= len(tb.Cols) {
			return msg.New("table.columnIndexOutOfRange", msg.Args{"n": i})
		}
		c := tb.Cols[i]
		if c.Type != grid.Text || c.Fixed || c.Formula != "" {
			return nil
		}
		col := colSQL(c.ID)
		// Un seul parcours du tableau : le total et, pour chaque type, les
		// valeurs qui ne s'y convertiraient pas (un parcours par type prenait
		// cinq fois plus de temps sur un million de lignes).
		types := []grid.Type{grid.Boolean, grid.Integer, grid.Decimal, grid.Date}
		sums := make([]string, len(types))
		for k, to := range types {
			sums[k] = "coalesce(sum(" + convertExpr(col, c.Type, to) + " IS NULL), 0)"
		}
		off := make([]int, len(types))
		dest := []any{&mix.Total}
		for k := range off {
			dest = append(dest, &off[k])
		}
		if err := t.row(`SELECT count(*), ` + strings.Join(sums, ", ") + ` FROM ` + tb.sql() + ` WHERE ` + col + ` IS NOT NULL`).Scan(dest...); err != nil || mix.Total == 0 {
			return err
		}
		best := mix.Total
		for k, to := range types {
			if off[k] < best {
				best, mix.Type = off[k], to
			}
		}
		if best == 0 || 2*best >= mix.Total {
			mix.Type = ""
		}
		if mix.Type != "" {
			mix.Off = best
		}
		return nil
	})
	return mix, err
}

// SetType fixe le type d'une colonne choisi par l'utilisateur. Les valeurs
// qui ne se convertissent pas sont gardées telles quelles comme saisies en
// erreur (cellule vide, valeur brute dans _lattis_invalid) ; l'annulation
// les rend intactes.
func (m *Model) SetType(ctx context.Context, table string, i int, to grid.Type) error {
	return m.Do(ctx, "Changer le type", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if i >= 0 && i < len(tb.Cols) {
			work.Op(ctx, "setType", tb.Cols[i].Name)
		}
		return t.setType(tb, i, to)
	})
}

// AcceptAsText convertit la colonne i en texte et y met ses saisies
// invalides telles qu'elles ont été tapées (vérifications avant de publier
// sur un Serveur Lattis : « accepter comme texte »).
func (m *Model) AcceptAsText(ctx context.Context, table string, i int) error {
	return m.Do(ctx, "Accepter comme texte", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if err := t.setType(tb, i, grid.Text); err != nil {
			return err
		}
		c := tb.Cols[i]
		col := colSQL(c.ID)
		trash := fmt.Sprintf("_lattis_trash_text_%d_%d_%d", tb.ID, c.ID, uniq())
		return t.ApplyStep(command.Step{Do: []stmt{
			st(`CREATE TABLE `+trash+` AS SELECT rid, raw FROM _lattis_invalid WHERE tbl = ? AND col = ?`, tb.ID, c.ID),
			st(`UPDATE ` + tb.sql() + ` SET ` + col + ` = ` + trash + `.raw FROM ` + trash + ` WHERE ` + trash + `.rid = ` + tb.sql() + `.id`),
			st(`DELETE FROM _lattis_invalid WHERE tbl = ? AND col = ?`, tb.ID, c.ID),
		}, Undo: []stmt{
			st(`UPDATE ` + tb.sql() + ` SET ` + col + ` = NULL WHERE id IN (SELECT rid FROM ` + trash + `)`),
			st(`INSERT INTO _lattis_invalid (tbl, rid, col, raw) SELECT ?, rid, ?, raw FROM `+trash, tb.ID, c.ID),
			st(`DROP TABLE ` + trash),
		}, Purge: []stmt{st(`DROP TABLE IF EXISTS ` + trash)}})
	})
}

// setType change le type de la colonne i (voir SetType).
func (t *tx) setType(tb Table, i int, to grid.Type) error {
	{
		if i < 0 || i >= len(tb.Cols) {
			return msg.New("table.columnIndexOutOfRange", msg.Args{"n": i})
		}
		c := tb.Cols[i]
		fix := `UPDATE _lattis_columns SET fixed = ? WHERE tbl = ? AND id = ?`
		if c.Type == to {
			return t.apply([]stmt{st(fix, 1, tb.ID, c.ID)}, []stmt{st(fix, c.Fixed, tb.ID, c.ID)})
		}
		physical, err := t.physical(tb)
		if err != nil {
			return err
		}
		col := colSQL(c.ID)
		trash := fmt.Sprintf("_lattis_trash_conv_%d_%d_%d", tb.ID, c.ID, uniq())
		do := []stmt{
			// Les valeurs qui ne passeront pas : gardées pour l'annulation…
			st(`CREATE TABLE ` + trash + ` (id INTEGER PRIMARY KEY, v)`),
			st(`INSERT INTO ` + trash + ` (id, v) SELECT id, ` + col + ` FROM ` + tb.sql() +
				` WHERE ` + col + ` IS NOT NULL AND ` + convertExpr(col, c.Type, to) + ` IS NULL`),
			// … et marquées en erreur avec leur valeur lisible.
			st(`INSERT OR REPLACE INTO _lattis_invalid (tbl, rid, col, raw) SELECT ?, id, ?, slt_text(v, ?) FROM `+trash, tb.ID, c.ID, string(c.Type)),
		}
		do = append(do, rebuildStmts(tb, physical, c.ID, c.Type, to)...)
		do = append(do, st(fix, 1, tb.ID, c.ID))
		undo := rebuildStmts(tb, physical, c.ID, to, c.Type)
		undo = append(undo,
			st(`UPDATE `+tb.sql()+` SET `+col+` = `+trash+`.v FROM `+trash+` WHERE `+trash+`.id = `+tb.sql()+`.id`),
			st(`DELETE FROM _lattis_invalid WHERE tbl = ? AND col = ? AND rid IN (SELECT id FROM `+trash+`)`, tb.ID, c.ID),
			st(`DROP TABLE `+trash),
			st(fix, c.Fixed, tb.ID, c.ID),
		)
		return t.ApplyStep(command.Step{Do: do, Undo: undo, Purge: []stmt{st(`DROP TABLE IF EXISTS ` + trash)}})
	}
}
