package canvas

import (
	"context"
	"fmt"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/store"
	"github.com/lalamefine/lattis/internal/work"
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
// si le tableau n'a ni clé ni refus (« Ne plus proposer »). Le contrôle
// (keyCheck, un parcours de toute la colonne : 0,9 s sur 1 M de lignes)
// n'est refait que si le nombre de lignes ou les valeurs de la première
// colonne changent, ou si une autre colonne devient la première ou que son
// type change (demande de Louis du 22/10) : pas après une saisie, un tri,
// ni l'ajout, la suppression ou la conversion d'une autre colonne. La clé,
// le refus et la formule se lisent hors du cache ; l'identifiant et le type
// de la colonne sont dans sa clé (store.NoMeta).
func (t *tx) keyHint(tb Table) (bool, error) {
	hint, _, err := t.keyHintIn(tb, false)
	return hint, err
}

// keyHintIn est keyHint ; later : sur un grand tableau (plus de quickRows
// lignes), rien n'est calculé s'il n'est pas déjà en cache, pending le dit
// (la mise en page s'affiche d'abord, LayoutHints complète ensuite).
func (t *tx) keyHintIn(tb Table, later bool) (hint, pending bool, err error) {
	if tb.Key != 0 || tb.Rows < 2 || len(tb.Cols) == 0 || tb.Cols[0].Formula != "" {
		return false, false, nil
	}
	var off bool
	if err := t.row(`SELECT no_key_hint FROM _lattis_tables WHERE id = ?`, tb.ID).Scan(&off); err != nil || off {
		return false, false, err
	}
	c := tb.Cols[0]
	key := fmt.Sprintf("keyhint:%s:%d:%s", tb.sql(), c.ID, c.Type)
	deps := []string{store.NoMeta, store.ColumnOf(tb.sql(), colSQL(c.ID))}
	if later && tb.Rows > quickRows {
		v, ok := peekOn[bool](t, key, deps)
		return v, !ok, nil
	}
	hint, err = cachedOn(t, key, deps, func() (bool, error) {
		// Entiers en suite régulière (1, 2, 3…) : remplis et sans doublon,
		// sans le comptage des valeurs distinctes (un tri de la colonne :
		// 0,9 s sur 1 M de lignes). La suite est lue de toute façon pour la
		// numérotation automatique (même cache).
		if c.Type == grid.Integer {
			if i, err := t.autoNumbered(tb); err != nil || i == 0 {
				return i == 0, err
			}
		}
		work.Phase(t.Ctx, "keyCheck")
		why, err := t.keyCheck(tb, 0)
		return why == "", err
	})
	return hint, false, err
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
