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
		if len(cols) == 0 {
			for i := range tb.Cols {
				cols = append(cols, i)
			}
		}
		var key []string
		for _, i := range cols {
			if i < 0 || i >= len(tb.Cols) {
				return fmt.Errorf("colonne %d hors du tableau", i)
			}
			c := tb.Cols[i]
			if c.Formula != "" {
				continue
			}
			// Le texte saisi compte, sans tenir compte de la casse (Excel).
			key = append(key, fmt.Sprintf("lower(coalesce(slt_text(%s, '%s'), ''))", colSQL(c.ID), c.Type))
		}
		if len(key) == 0 {
			return nil
		}
		k := strings.Join(key, ", ")
		rs, err := t.query(`SELECT id FROM (SELECT id, row_number() OVER (PARTITION BY `+k+` ORDER BY _pos) AS rn FROM `+tb.sql()+
			`) WHERE rn > 1 LIMIT ?`, MaxDuplicates+1)
		if err != nil {
			return err
		}
		var ids []int64
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return err
			}
			ids = append(ids, id)
		}
		rs.Close()
		if len(ids) > MaxDuplicates {
			return fmt.Errorf("plus de %d doublons : utilisez l'onglet SQL", MaxDuplicates)
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
