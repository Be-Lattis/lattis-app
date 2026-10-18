package canvas

import (
	"context"

	"github.com/lalamefine/lattis/internal/grid"
)

// Suggestion de clé (choix de Louis : jamais de clé primaire automatique) :
// quand la première colonne d'un tableau sans clé est un entier unique et
// croissant, l'interface propose discrètement d'en faire la clé, en un clic.
// L'index UNIQUE n'est créé qu'à ce moment-là (SetKey).

// keyHint dit si la première colonne du tableau semble identifier chaque
// ligne : type entier, aucune valeur vide, valeurs strictement croissantes
// dans l'ordre d'affichage (donc uniques), au moins 2 lignes, et le tableau
// n'a ni clé ni refus (« Ne plus proposer »).
//
// Une seule requête, qui s'arrête à la première valeur fautive, gardée tant
// que le tableau et les métadonnées ne changent pas (cachedOn).
func (t *tx) keyHint(tb Table) (bool, error) {
	if tb.Key != 0 || tb.Rows < 2 || len(tb.Cols) == 0 {
		return false, nil
	}
	c := tb.Cols[0]
	if c.Type != grid.Integer || c.Formula != "" {
		return false, nil
	}
	return cachedOn(t, "keyhint:"+tb.sql(), []string{tb.sql()}, func() (bool, error) {
		var off bool
		if err := t.row(`SELECT no_key_hint FROM _lattis_tables WHERE id = ?`, tb.ID).Scan(&off); err != nil || off {
			return false, err
		}
		col := colSQL(c.ID)
		// Une valeur hors type (texte dans la colonne entière) ou vide n'est
		// pas « integer » ; v <= p : pas strictement croissante.
		var bad bool
		err := t.row(`SELECT EXISTS (SELECT 1 FROM (SELECT ` + col + ` AS v, lag(` + col + `) OVER (ORDER BY _pos, id) AS p FROM ` + tb.sql() + `)
			WHERE typeof(v) <> 'integer' OR v <= p)`).Scan(&bad)
		return !bad, err
	})
}

// DismissKeyHint ne propose plus la première colonne du tableau comme clé
// (mémorisé dans le classeur, annulable).
func (m *Model) DismissKeyHint(ctx context.Context, table string) error {
	return m.Do(ctx, "Ne plus proposer de clé", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		return t.apply(
			[]stmt{st(`UPDATE _lattis_tables SET no_key_hint = 1 WHERE id = ?`, tb.ID)},
			[]stmt{st(`UPDATE _lattis_tables SET no_key_hint = 0 WHERE id = ?`, tb.ID)},
		)
	})
}
