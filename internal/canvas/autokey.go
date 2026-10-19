package canvas

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
)

// Clé posée d'office et numérotation automatique (demande de Louis,
// 19/10/2026, docs/decisions/0009-cle-automatique.md ; remplace en partie
// « jamais de clé automatique », keyhint.go).
//
// À la création d'un tableau par l'utilisateur (saisie, collage, Mettre sous
// forme de tableau), sa première colonne devient la clé quand elle ne
// contient que des entiers uniques et remplis, et :
//   - que son en-tête est « ID » (casse ignorée) ;
//   - ou que ses valeurs sont 1, 2, 3… dans l'ordre des lignes (au moins
//     deux lignes).
//
// L'import d'un fichier n'est pas concerné (le classeur reste fidèle à sa
// source ; la clé reste proposée d'un clic, keyhint.go).
//
// Une ligne ajoutée reçoit ensuite son numéro (autoNumber) : max + 1 dans
// une clé d'entiers, où qu'elle soit insérée (pas de renumérotation) ;
// dernier + pas dans une première colonne d'entiers en suite régulière
// (10, 20, 30…), seulement pour une ligne ajoutée à la fin.

// idHeader : en-tête qui désigne un identifiant.
func idHeader(name string) bool { return strings.EqualFold(strings.TrimSpace(name), "id") }

// autoKey pose la clé sur la première colonne d'un tableau qui vient d'être
// créé, si elle s'y prête (voir plus haut). Même étape d'annulation que la
// création.
func (t *tx) autoKey(id int64) error {
	tb, err := t.table(id)
	if err != nil {
		return err
	}
	if tb.Key != 0 || len(tb.Cols) == 0 || tb.Rows == 0 {
		return nil
	}
	c := tb.Cols[0]
	if c.Formula != "" || c.Type != grid.Integer {
		return nil
	}
	if why, err := t.keyCheck(tb, 0); err != nil || why != "" {
		return err
	}
	if !idHeader(c.Name) {
		// 1, 2, 3… : la valeur de chaque ligne est son rang, sur au moins
		// deux lignes (un 1 seul n'est pas une suite).
		if tb.Rows < 2 {
			return nil
		}
		var first any
		if err := t.row(`SELECT ` + colSQL(c.ID) + ` FROM ` + tb.sql() + ` ORDER BY _pos LIMIT 1`).Scan(&first); err != nil {
			return err
		}
		if n, ok := first.(int64); !ok || n != 1 {
			return nil
		}
		var off int
		if err := t.row(`SELECT count(*) FROM (SELECT ` + colSQL(c.ID) + ` AS v, row_number() OVER (ORDER BY _pos) AS n FROM ` + tb.sql() + `) WHERE v IS NOT n`).Scan(&off); err != nil {
			return err
		}
		if off > 0 {
			return nil
		}
	}
	// ensureKey dit « clé posée pour REFERENCE » (Event.Keyed) : pas ici.
	keyed := t.keyed
	if err := t.ensureKey(tb.Name, c.Name); err != nil {
		return err
	}
	t.keyed = keyed
	t.autoKeyed = c.Name
	if !t.undoBirth {
		t.notes = append(t.notes, fmt.Sprintf("Clé posée sur %s (Ctrl+Z pour annuler)", c.Name))
	}
	return nil
}

// autoNum : colonne qui se numérote d'elle-même, prochaine valeur en fin de
// tableau et pas ; key : c'est la clé (max + 1, aussi au milieu).
type autoNum struct {
	col        Column
	next, step int64
	key        bool
}

// autoNumber dit si une ligne ajoutée au tableau reçoit un numéro, et lequel.
func (t *tx) autoNumber(tb Table) (autoNum, bool, error) {
	if len(tb.Cols) == 0 {
		return autoNum{}, false, nil
	}
	if tb.Key != 0 {
		for _, c := range tb.Cols {
			if c.ID != tb.Key || c.Type != grid.Integer || c.Formula != "" {
				continue
			}
			var next int64
			if err := t.row(`SELECT coalesce(max(` + colSQL(c.ID) + `), 0) + 1 FROM ` + tb.sql()).Scan(&next); err != nil {
				return autoNum{}, false, err
			}
			return autoNum{col: c, next: next, step: 1, key: true}, true, nil
		}
	}
	c := tb.Cols[0]
	if c.ID == tb.Key || c.Type != grid.Integer || c.Formula != "" {
		return autoNum{}, false, nil
	}
	// Suite régulière d'au moins deux entiers, dans l'ordre des lignes : les
	// deux premiers donnent le pas, le dernier doit tomber juste (lectures
	// par l'index de l'ordre) avant de parcourir la colonne.
	col := colSQL(c.ID)
	var total int64
	if err := t.row(`SELECT count(*) FROM ` + tb.sql()).Scan(&total); err != nil {
		return autoNum{}, false, err
	}
	if total < 2 {
		return autoNum{}, false, nil
	}
	var a, b, z sql.NullInt64
	if err := t.row(`SELECT (SELECT `+col+` FROM `+tb.sql()+` ORDER BY _pos LIMIT 1),
		(SELECT `+col+` FROM `+tb.sql()+` ORDER BY _pos LIMIT 1 OFFSET 1),
		(SELECT `+col+` FROM `+tb.sql()+` ORDER BY _pos DESC LIMIT 1)`).Scan(&a, &b, &z); err != nil {
		return autoNum{}, false, err
	}
	step, last := b.Int64-a.Int64, z.Int64
	if !a.Valid || !b.Valid || !z.Valid || step == 0 || last != a.Int64+(total-1)*step {
		return autoNum{}, false, nil
	}
	if total > 2 {
		var off int
		if err := t.row(`SELECT count(*) FROM (SELECT `+col+` - lag(`+col+`) OVER (ORDER BY _pos) AS d, row_number() OVER (ORDER BY _pos) AS n FROM `+tb.sql()+`) WHERE n > 1 AND (d IS NULL OR d <> ?)`, step).Scan(&off); err != nil {
			return autoNum{}, false, err
		}
		if off > 0 {
			return autoNum{}, false, nil
		}
	}
	return autoNum{col: c, next: last + step, step: step}, true, nil
}

// autoNumbered dit, pour l'affichage, quelle colonne se numérote d'elle-même
// (rang, -1 si aucune). Gardé tant que le tableau ne change pas.
func (t *tx) autoNumbered(tb Table) (int, error) {
	if len(tb.Cols) == 0 || tb.Rows == 0 {
		return -1, nil
	}
	// La clé et le type des colonnes sont dans la clé du cache (ils
	// changent dans _lattis_tables et _lattis_columns).
	key := fmt.Sprintf("autonum:%s:%d", tb.sql(), tb.Key)
	for i, c := range tb.Cols {
		if i == 0 || c.ID == tb.Key {
			key += fmt.Sprintf(":%d:%s:%t", c.ID, c.Type, c.Formula != "")
		}
	}
	id, err := cachedOn(t, key, []string{tb.sql()}, func() (int64, error) {
		a, ok, err := t.autoNumber(tb)
		if err != nil || !ok {
			return 0, err
		}
		return a.col.ID, nil
	})
	if err != nil || id == 0 {
		return -1, err
	}
	for i, c := range tb.Cols {
		if c.ID == id {
			return i, nil
		}
	}
	return -1, nil
}

// numberRow donne son numéro à la ligne rid qui vient d'être ajoutée en pos
// (a : calculé avant l'ajout). Hors clé, seulement pour une ligne à la fin.
func (t *tx) numberRow(tb Table, a autoNum, rid int64, pos string) error {
	if !a.key {
		var after int
		if err := t.row(`SELECT EXISTS (SELECT 1 FROM `+tb.sql()+` WHERE _pos > ?)`, pos).Scan(&after); err != nil || after != 0 {
			return err
		}
	}
	return t.setValue(tb, rid, a.col, a.next, "")
}
