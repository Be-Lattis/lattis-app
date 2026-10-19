package canvas

import (
	"context"
)

// Suggestion de clé (choix de Louis du 18/10 : pas de clé primaire
// automatique ; en partie remplacé le 19/10 : une première colonne ID ou
// 1, 2, 3… devient la clé à la création, autokey.go et
// docs/decisions/0010-cle-automatique.md) : dès que les valeurs de la
// première colonne d'un tableau sans clé sont uniques et toutes remplies,
// quel que soit leur type (nombre, texte, date…), l'interface propose
// discrètement d'en faire la clé, en un clic. L'index UNIQUE n'est créé
// qu'à ce moment-là (SetKey).

// keyHint dit si la première colonne du tableau peut identifier chaque
// ligne (la même règle que SetKey : keyCheck), avec au moins 2 lignes, et
// si le tableau n'a ni clé ni refus (« Ne plus proposer »). Le résultat est
// gardé tant que le tableau et les métadonnées ne changent pas (cachedOn).
func (t *tx) keyHint(tb Table) (bool, error) {
	if tb.Key != 0 || tb.Rows < 2 || len(tb.Cols) == 0 || tb.Cols[0].Formula != "" {
		return false, nil
	}
	return cachedOn(t, "keyhint:"+tb.sql(), []string{tb.sql()}, func() (bool, error) {
		var off bool
		if err := t.row(`SELECT no_key_hint FROM _lattis_tables WHERE id = ?`, tb.ID).Scan(&off); err != nil || off {
			return false, err
		}
		why, err := t.keyCheck(tb, 0)
		return why == "", err
	})
}

// DismissKeyHint ne propose plus la première colonne du tableau comme clé
// (mémorisé dans le classeur, annulable).
func (m *Model) DismissKeyHint(ctx context.Context, table string) error {
	return m.Do(ctx, "Ne plus proposer de clé", func(t *tx) error {
		tb, err := t.sortTable(table)
		if err != nil {
			return err
		}
		return t.apply(
			[]stmt{st(`UPDATE _lattis_tables SET no_key_hint = 1 WHERE id = ?`, tb.ID)},
			[]stmt{st(`UPDATE _lattis_tables SET no_key_hint = 0 WHERE id = ?`, tb.ID)},
		)
	})
}
