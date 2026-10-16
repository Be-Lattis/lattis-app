package canvas

import (
	"context"
	"fmt"

	"github.com/lalamefine/slitable/internal/command"
)

// Editor écrit dans les tableaux pour un script. Chaque appel est une courte
// transaction de la file d'écriture, ajoutée à la même commande
// (command.Bus.Append) : la grille reste utilisable pendant le script, et
// tout ce que le script a écrit s'annule en une seule étape.
//
// Les valeurs sont des saisies brutes, traitées exactement comme une frappe
// dans la grille : inférence, élargissement de type, saisie invalide dans une
// colonne au type fixé. Une saisie vide efface la cellule.
type Editor struct {
	m    *Model
	name string
	seq  int64
}

// Editor ouvre un éditeur ; name est le nom de l'étape d'annulation.
func (m *Model) Editor(name string) *Editor { return &Editor{m: m, name: name} }

func (e *Editor) do(ctx context.Context, fn func(*tx) error) error {
	seq, err := e.m.bus.Append(ctx, e.seq, e.name, func(c *command.Tx) error { return fn(&tx{Tx: c, m: e.m}) })
	if err == nil && seq != 0 {
		e.seq = seq
	}
	return err
}

// Insert ajoute une ligne en fin de tableau et renvoie sa clé.
func (e *Editor) Insert(ctx context.Context, table string, vals map[string]string) (int64, error) {
	var id int64
	err := e.do(ctx, func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		idx, err := colIndexes(tb, vals)
		if err != nil {
			return err
		}
		if err := t.appendRows(tb, 1); err != nil {
			return err
		}
		if err := t.row(`SELECT max(id) FROM ` + tb.sql()).Scan(&id); err != nil {
			return err
		}
		return t.setCells(tb.ID, tb.Rows, idx, vals)
	})
	return id, err
}

// Update modifie les colonnes données de la ligne de clé id.
func (e *Editor) Update(ctx context.Context, table string, id int64, vals map[string]string) error {
	return e.do(ctx, func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		idx, err := colIndexes(tb, vals)
		if err != nil {
			return err
		}
		n, err := t.rowIndex(tb, id)
		if err != nil {
			return err
		}
		return t.setCells(tb.ID, n, idx, vals)
	})
}

// Delete supprime la ligne de clé id (le tableau rétrécit, comme dans la grille).
func (e *Editor) Delete(ctx context.Context, table string, id int64) error {
	return e.do(ctx, func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if _, err := t.rowIndex(tb, id); err != nil {
			return err
		}
		return t.deleteRow(tb, id)
	})
}

// colIndexes associe chaque nom de colonne à son rang d'affichage.
func colIndexes(tb Table, vals map[string]string) (map[string]int, error) {
	idx := map[string]int{}
	for name := range vals {
		found := false
		for i, c := range tb.Cols {
			if c.Name == name {
				idx[name], found = i, true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("tableau %q : pas de colonne %q", tb.Name, name)
		}
	}
	return idx, nil
}

// rowIndex donne le rang d'affichage de la ligne de clé id.
func (t *tx) rowIndex(tb Table, id int64) (int, error) {
	var n, ok int
	err := t.row(`SELECT count(*), (SELECT count(*) FROM `+tb.sql()+` WHERE id = ?) FROM `+tb.sql()+
		` WHERE _pos < (SELECT _pos FROM `+tb.sql()+` WHERE id = ?)`, id, id).Scan(&n, &ok)
	if err == nil && ok == 0 {
		err = fmt.Errorf("tableau %q : pas de ligne %d", tb.Name, id)
	}
	return n, err
}

// setCells écrit les saisies dans la ligne n. Le tableau est relu après
// chaque cellule : une saisie peut avoir changé le type d'une colonne.
func (t *tx) setCells(table int64, n int, idx map[string]int, vals map[string]string) error {
	var ev Event
	for name, i := range idx {
		tb, err := t.table(table)
		if err != nil {
			return err
		}
		if err := t.setCell(tb, n, i, vals[name], &ev); err != nil {
			return err
		}
	}
	return nil
}
