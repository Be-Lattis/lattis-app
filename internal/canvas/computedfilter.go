package canvas

import (
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// Tri et filtre d'une colonne calculée : ses valeurs ne sont pas stockées
// (c_<id> reste vide), on les lit dans la table du tableau avec ses
// colonnes calculées (computedSource), sous le même nom (t_<id>, c_<id>).
// Comme dans Excel, une formule qui dépend de l'ordre des lignes (ligne
// précédente, cumul) est triée sur ses valeurs avant le tri.

// computedView donne la requête qui rend le tableau avec les valeurs de ses
// colonnes calculées, et la colonne c avec le type de son résultat (texte
// s'il est inconnu).
func (t *tx) computedView(tb Table, c Column) (string, Column, error) {
	src, ok, err := t.computedSource(tb)
	if err != nil {
		return "", c, err
	}
	if !ok {
		return "", c, msg.New("computed.notFound", msg.Args{"column": c.Name})
	}
	c.Type = t.computedResult(tb, c)
	if c.Type == "" {
		c.Type = grid.Text
	}
	return "(" + src + ") AS " + tb.sql(), c, nil
}

// computedCond rend la condition d'un filtre posé sur une colonne calculée :
// les lignes dont la valeur calculée vérifie where(c) (c : la colonne au
// type de son résultat, lue dans la vue calculée). Vide si la vue ne peut
// pas être calculée pendant le calcul d'une autre (pas de récursion : la
// compilation d'une formule relit les tableaux et leurs filtres).
func (t *tx) computedCond(tb Table, c Column, where func(Column) (string, error)) (string, error) {
	if t.inComputedFilter {
		return "", nil
	}
	t.inComputedFilter = true
	defer func() { t.inComputedFilter = false }()
	view, cc, err := t.computedView(tb, c)
	if err != nil {
		return "", err
	}
	w, err := where(cc)
	if err != nil || w == "" {
		return "", err
	}
	return "id IN (SELECT id FROM " + view + " WHERE " + w + ")", nil
}
