package canvas

import (
	"context"
	"fmt"
	"strings"
)

// MaxDuplicates borne une suppression de doublons (une ligne supprimée est
// une opération du journal).
const MaxDuplicates = 100000

// RemoveDuplicates supprime les lignes dont les colonnes cols (toutes si
// vide) répètent celles d'une ligne précédente, comme « Supprimer les
// doublons » d'Excel : la première occurrence reste. Renvoie le nombre de
// lignes supprimées ; une seule étape d'annulation.
func (m *Model) RemoveDuplicates(ctx context.Context, table string, cols []int) (int, error) {
	n := 0
	err := m.Do(ctx, "Supprimer les doublons", func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		if tb.Filtered() {
			return ErrFiltered{tb.Name}
		}
		ids, err := t.duplicates(tb, cols)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := t.deleteRow(tb, id); err != nil {
				return err
			}
		}
		n = len(ids)
		return nil
	})
	return n, err
}

// DuplicatesPreview compte les lignes que RemoveDuplicates retirerait.
func (m *Model) DuplicatesPreview(ctx context.Context, table string, cols []int) (int, error) {
	n := 0
	err := m.read(ctx, func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		ids, err := t.duplicates(tb, cols)
		n = len(ids)
		return err
	})
	return n, err
}

// duplicates donne les lignes en double selon les colonnes cols (toutes si
// vide), la première occurrence exceptée.
func (t *tx) duplicates(tb Table, cols []int) ([]int64, error) {
	{
		if len(cols) == 0 {
			for i := range tb.Cols {
				cols = append(cols, i)
			}
		}
		var key []string
		for _, i := range cols {
			if i < 0 || i >= len(tb.Cols) {
				return nil, fmt.Errorf("colonne %d hors du tableau", i)
			}
			c := tb.Cols[i]
			if c.Formula != "" {
				continue
			}
			// Le texte saisi compte, sans tenir compte de la casse (Excel).
			key = append(key, fmt.Sprintf("lower(coalesce(slt_text(%s, '%s'), ''))", colSQL(c.ID), c.Type))
		}
		if len(key) == 0 {
			return nil, nil
		}
		k := strings.Join(key, ", ")
		rs, err := t.query(`SELECT id FROM (SELECT id, row_number() OVER (PARTITION BY `+k+` ORDER BY _pos) AS rn FROM `+tb.sql()+
			`) WHERE rn > 1 LIMIT ?`, MaxDuplicates+1)
		if err != nil {
			return nil, err
		}
		var ids []int64
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return nil, err
			}
			ids = append(ids, id)
		}
		rs.Close()
		if len(ids) > MaxDuplicates {
			return nil, fmt.Errorf("plus de %d doublons : utilisez l'onglet SQL", MaxDuplicates)
		}
		return ids, nil
	}
}
