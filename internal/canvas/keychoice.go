package canvas

import (
	"context"
	"fmt"
)

// Question posée quand une formule cherche dans un tableau sans clé
// (REFERENCE, REFERENCE.APPROCHEE, EXISTE, RECHERCHEV convertie) : ses
// lignes se désignent par leur numéro, ou une colonne devient la clé. Lattis
// ne choisit jamais seul (choix de Louis, 19/10/2026).

// KeyChoice est la réponse : KeyIndex (numéro de ligne) ou KeyColumn (la
// colonne cherchée devient la clé). Vide : pas encore de réponse.
type KeyChoice string

const (
	KeyIndex  KeyChoice = "index"
	KeyColumn KeyChoice = "column"
)

type keyChoiceCtx struct{}

// WithKeyChoice porte la réponse pour l'action lancée avec ctx.
func WithKeyChoice(ctx context.Context, c KeyChoice) context.Context {
	if c == "" {
		return ctx
	}
	return context.WithValue(ctx, keyChoiceCtx{}, c)
}

// KeyChoiceFrom lit la réponse portée par ctx ("" : aucune).
func KeyChoiceFrom(ctx context.Context) KeyChoice {
	if ctx == nil {
		return ""
	}
	c, _ := ctx.Value(keyChoiceCtx{}).(KeyChoice)
	return c
}

// ErrKeyChoice refuse une formule qui cherche dans un tableau sans clé tant
// que la question n'a pas de réponse. Column : la colonne qui deviendrait la
// clé ; Why : pourquoi elle ne le peut pas (vide : elle le peut).
type ErrKeyChoice struct{ Table, Column, Why string }

func (e ErrKeyChoice) Error() string {
	s := fmt.Sprintf("clé à choisir : « %s » n'a pas de clé ; la recherche peut désigner ses lignes par leur numéro, ou par la colonne « %s », qui deviendrait la clé du tableau (les doublons y seraient refusés)", e.Table, e.Column)
	if e.Why != "" {
		s += fmt.Sprintf(" ; « %s » ne peut pas servir de clé : %s", e.Column, e.Why)
	}
	return s
}

// keepRowNumbers retient que le tableau garde ses numéros de ligne : les
// formules ne posent plus la question, et la clé n'est plus proposée.
func (t *tx) keepRowNumbers(table string) error {
	tb, err := t.tableByName(table)
	if err != nil {
		return err
	}
	var off bool
	if err := t.row(`SELECT no_key_hint FROM _lattis_tables WHERE id = ?`, tb.ID).Scan(&off); err != nil || off {
		return err
	}
	return t.apply(
		[]stmt{st(`UPDATE _lattis_tables SET no_key_hint = 1 WHERE id = ?`, tb.ID)},
		[]stmt{st(`UPDATE _lattis_tables SET no_key_hint = 0 WHERE id = ?`, tb.ID)},
	)
}
