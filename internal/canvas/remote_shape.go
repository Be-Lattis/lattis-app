package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/grid"
)

// Structure d'un tableau publié changée sur le serveur par un concepteur
// (notification « *structure:<table_id> » ; docs/collaboration/
// synchronisation.md) : le tableau local prend la forme du serveur hors du
// journal d'annulation, comme les lignes reçues (ApplyRemote). Ce qui cite
// une colonne ou le tableau renommés suit (formules, TCD, graphiques).

// RemoteShape est la forme d'un tableau publié sur le serveur, en
// identifiants locaux.
type RemoteShape struct {
	Table int64
	// Gone : tableau supprimé sur le serveur.
	Gone bool
	// Name : nom du tableau ("" : inchangé).
	Name string
	// Columns : colonnes publiées, dans l'ordre du serveur.
	Columns []RemoteShapeColumn
	// Published : colonnes locales publiées jusqu'ici ; celles qui ne sont
	// plus dans Columns sont supprimées.
	Published []int64
	// KeySet : la clé est à suivre ; KeyAt : rang de la colonne clé dans
	// Columns (-1 : plus de clé).
	KeySet bool
	KeyAt  int
	// Calc : colonnes converties en valeurs (ou redevenues calculées) dont
	// le serveur a refusé le changement, avec leur formule (anglais) : une
	// colonne qui n'est pas sur le serveur la reprend au lieu d'être
	// supprimée, une colonne du serveur la perd.
	Calc map[int64]string
}

// RemoteShapeColumn est une colonne publiée.
type RemoteShapeColumn struct {
	// ID : colonne locale (0 : nouvelle) ; Want : identifiant local proposé
	// pour une nouvelle colonne (local_id noté par le serveur), pris s'il
	// est libre.
	ID, Want int64
	Name     string
	// Type : nouveau type local ("" : inchangé).
	Type grid.Type
}

// ApplyRemoteShapes donne aux tableaux publiés la forme reçue du serveur,
// dans une transaction locale hors du journal. plan, appelé dans la
// transaction, rend les formes à appliquer (lues avec l'état local à
// jour) ; after, appelé ensuite dans la même transaction, reçoit pour
// chaque tableau les colonnes locales de ses colonnes publiées (dans
// l'ordre de Columns). Rend les tableaux changés.
func (m *Model) ApplyRemoteShapes(ctx context.Context, plan func(*sql.Tx) ([]RemoteShape, error), after func(tx *sql.Tx, cols map[int64][]int64) error) ([]int64, error) {
	var changed []int64
	err := m.db.Write(ctx, func(sqlTx *sql.Tx) error {
		changed = nil
		shapes, err := plan(sqlTx)
		if err != nil {
			return err
		}
		t := &tx{Tx: &command.Tx{Tx: sqlTx, Ctx: ctx}, m: m}
		ids := map[int64][]int64{}
		for _, sh := range shapes {
			cols, ok, err := t.applyShape(sh)
			if err != nil {
				return err
			}
			if ok {
				changed = append(changed, sh.Table)
			}
			ids[sh.Table] = cols
		}
		// Hors du journal : les corbeilles des colonnes et tableaux supprimés
		// ne serviront pas.
		if err := t.Tx.Discard(); err != nil {
			return err
		}
		if after != nil {
			return after(sqlTx, ids)
		}
		return nil
	})
	return changed, err
}

func shapeCol(tb Table, id int64) int {
	for i, c := range tb.Cols {
		if c.ID == id {
			return i
		}
	}
	return -1
}

// applyShape donne à un tableau la forme sh ; rend les colonnes locales de
// sh.Columns.
func (t *tx) applyShape(sh RemoteShape) ([]int64, bool, error) {
	tb, err := t.table(sh.Table)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil // supprimé en local entre-temps
	}
	if err != nil {
		return nil, false, err
	}
	if sh.Gone {
		return nil, true, t.deleteTable(tb)
	}
	changed := false
	if sh.Name != "" && sh.Name != tb.Name {
		name, err := t.freeRemoteName(sh.Name, tb.ID)
		if err != nil {
			return nil, false, err
		}
		if name != tb.Name {
			if err := t.renameTable(tb, name); err != nil {
				return nil, false, err
			}
			changed = true
		}
	}
	keep := map[int64]bool{}
	for _, c := range sh.Columns {
		if c.ID != 0 {
			keep[c.ID] = true
		}
	}
	for _, id := range sh.Published {
		if keep[id] {
			continue
		}
		if tb, err = t.table(sh.Table); err != nil {
			return nil, false, err
		}
		i := shapeCol(tb, id)
		if i < 0 || len(tb.Cols) <= 1 {
			continue
		}
		if f, ok := sh.Calc[id]; ok && f != "" {
			// Conversion en valeurs refusée : la colonne redevient calculée.
			if err := t.setFormulaEN(tb, i, f); err != nil {
				return nil, false, err
			}
			changed = true
			continue
		}
		if tb.Cols[i].Formula != "" {
			// Redevenue calculée ici (annulation d'une conversion en valeurs,
			// reçue par la mise en page) : elle n'est plus que de la mise en
			// page.
			continue
		}
		if err := t.dropColumn(tb, tb.Cols[i], i); err != nil {
			return nil, false, err
		}
		changed = true
	}
	out := make([]int64, len(sh.Columns))
	for k, sc := range sh.Columns {
		if tb, err = t.table(sh.Table); err != nil {
			return nil, false, err
		}
		i := -1
		if sc.ID != 0 {
			i = shapeCol(tb, sc.ID)
		}
		if i < 0 {
			continue // nouvelle colonne, ajoutée ensuite
		}
		out[k] = sc.ID
		c := tb.Cols[i]
		if _, ok := sh.Calc[c.ID]; ok && c.Formula != "" {
			// Annulation d'une conversion refusée : colonne saisie du serveur.
			if err := t.setFormulaEN(tb, i, ""); err != nil {
				return nil, false, err
			}
			changed = true
			if tb, err = t.table(sh.Table); err != nil {
				return nil, false, err
			}
		}
		if sc.Name != "" && sc.Name != c.Name {
			if err := t.rename(tb, c, freeColumnNameExcept(tb, c.ID, sc.Name)); err != nil {
				return nil, false, err
			}
			changed = true
			if tb, err = t.table(sh.Table); err != nil {
				return nil, false, err
			}
		}
		if sc.Type != "" && sc.Type != c.Type {
			if err := t.setType(tb, i, sc.Type); err != nil {
				return nil, false, err
			}
			changed = true
		}
	}
	for k, sc := range sh.Columns {
		if out[k] != 0 {
			continue
		}
		if tb, err = t.table(sh.Table); err != nil {
			return nil, false, err
		}
		// Colonne calculée convertie en valeurs par un concepteur : la
		// colonne calculée de la mise en page (même identifiant, ou même
		// nom) devient cette colonne, sans doublon (« Total_2 »).
		if j := adoptable(tb, sc, out, sh.Published); j >= 0 {
			id, ok, err := t.adoptColumn(tb, j, sc)
			if err != nil {
				return nil, false, err
			}
			out[k] = id
			changed = changed || ok
			continue
		}
		id := sc.Want
		var taken int
		if err := t.row(`SELECT count(*) FROM _lattis_columns WHERE tbl = ? AND id = ?`, tb.ID, id).Scan(&taken); err != nil {
			return nil, false, err
		}
		if id <= 0 || taken > 0 {
			if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_columns WHERE tbl = ?`, tb.ID).Scan(&id); err != nil {
				return nil, false, err
			}
		}
		// Place : après la colonne publiée qui la précède sur le serveur.
		pos := 0
		for p := k - 1; p >= 0; p-- {
			if j := shapeCol(tb, out[p]); out[p] != 0 && j >= 0 {
				pos = j + 1
				break
			}
		}
		typ := sc.Type
		if typ == "" {
			typ = grid.Text
		}
		if err := t.addColumn(tb, id, freeColumnNameExcept(tb, 0, sc.Name), typ, pos); err != nil {
			return nil, false, err
		}
		fix := `UPDATE _lattis_columns SET fixed = ? WHERE tbl = ? AND id = ?`
		if err := t.apply([]stmt{st(fix, 1, tb.ID, id)}, []stmt{st(fix, 0, tb.ID, id)}); err != nil {
			return nil, false, err
		}
		if err := t.ensureGapRight(tb.Sheet, tb.ID, tb.Right(), tb.Right()+1, tb.Row, tb.Bottom()); err != nil {
			return nil, false, err
		}
		out[k] = id
		changed = true
	}
	if sh.KeySet {
		if tb, err = t.table(sh.Table); err != nil {
			return nil, false, err
		}
		var key int64
		if sh.KeyAt >= 0 && sh.KeyAt < len(out) {
			key = out[sh.KeyAt]
		}
		if key != tb.Key {
			i := shapeCol(tb, key)
			why := ""
			if key != 0 {
				if i < 0 {
					why = "colonne introuvable"
				} else if why, err = t.keyCheck(tb, i); err != nil {
					return nil, false, err
				}
			}
			// Une clé que les lignes visibles ici ne permettent pas (vide,
			// doublon) reste celle d'avant.
			if why == "" {
				if tb.Key != 0 {
					if err := t.dropKey(tb); err != nil {
						return nil, false, err
					}
				}
				if key != 0 {
					if err := t.ensureKey(tb.Name, tb.Cols[i].Name); err != nil {
						return nil, false, err
					}
				}
				changed = true
			}
		}
	}
	return out, changed, nil
}

// adoptable : rang de la colonne locale qui devient la nouvelle colonne
// du serveur sc (colonne calculée convertie en valeurs par un concepteur,
// arrivée dans la mise en page et par la structure) : celle de
// l'identifiant noté par le serveur si elle a le même nom (calculée, ou
// déjà convertie par la mise en page), sinon une colonne calculée du même
// nom ; ni publiée ni déjà prise. -1 : aucune.
func adoptable(tb Table, sc RemoteShapeColumn, taken, published []int64) int {
	free := func(id int64) bool { return !slices.Contains(taken, id) && !slices.Contains(published, id) }
	if i := shapeCol(tb, sc.Want); sc.Want > 0 && i >= 0 && free(sc.Want) {
		if strings.EqualFold(tb.Cols[i].Name, sc.Name) {
			return i
		}
	}
	for i, c := range tb.Cols {
		if c.Formula != "" && strings.EqualFold(c.Name, sc.Name) && free(c.ID) {
			return i
		}
	}
	return -1
}

// adoptColumn fait de la colonne j de tb la colonne du serveur sc : plus
// de formule, nom et type du serveur, type fixé ; ses valeurs viennent de
// la relecture du tableau.
func (t *tx) adoptColumn(tb Table, j int, sc RemoteShapeColumn) (int64, bool, error) {
	c := tb.Cols[j]
	changed := false
	var err error
	if c.Formula != "" {
		if err := t.setFormulaEN(tb, j, ""); err != nil {
			return 0, false, err
		}
		changed = true
		if tb, err = t.table(tb.ID); err != nil {
			return 0, false, err
		}
	}
	if sc.Name != "" && sc.Name != c.Name {
		if err := t.rename(tb, tb.Cols[j], freeColumnNameExcept(tb, c.ID, sc.Name)); err != nil {
			return 0, false, err
		}
		changed = true
		if tb, err = t.table(tb.ID); err != nil {
			return 0, false, err
		}
	}
	if sc.Type != "" && sc.Type != tb.Cols[j].Type {
		if err := t.setType(tb, j, sc.Type); err != nil {
			return 0, false, err
		}
		changed = true
	}
	fix := `UPDATE _lattis_columns SET fixed = ? WHERE tbl = ? AND id = ?`
	if err := t.apply([]stmt{st(fix, 1, tb.ID, c.ID)}, []stmt{st(fix, 0, tb.ID, c.ID)}); err != nil {
		return 0, false, err
	}
	return c.ID, changed, nil
}

// setFormulaEN pose la formule (anglais, telle que rangée) de la colonne i
// de tb, "" : plus de formule.
func (t *tx) setFormulaEN(tb Table, i int, en string) error {
	c := tb.Cols[i]
	q := `UPDATE _lattis_columns SET formula = ? WHERE tbl = ? AND id = ?`
	var now, old any
	if en != "" {
		now = en
	}
	if c.Formula != "" {
		old = c.Formula
	}
	return t.apply([]stmt{st(q, now, tb.ID, c.ID)}, []stmt{st(q, old, tb.ID, c.ID)})
}

// freeRemoteName rend name en nom de tableau valide et libre (un tableau
// local du même nom garde le sien : « name_2 »…).
func (t *tx) freeRemoteName(name string, self int64) (string, error) {
	base := identifier(strings.TrimSpace(name))
	if base == "" || A1Like(base) {
		base = "Tableau_" + base
	}
	cand := base
	for n := 2; ; n++ {
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_tables WHERE name = ? COLLATE NOCASE AND id <> ?`, cand, self).Scan(&k); err != nil {
			return "", err
		}
		if k == 0 {
			return cand, nil
		}
		cand = fmt.Sprintf("%s_%d", base, n)
	}
}
