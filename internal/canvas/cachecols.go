package canvas

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/lalamefine/lattis/internal/store"
)

// Un TCD ou un graphique ne lit que quelques colonnes de sa source : ses
// zones, ses axes, et l'ordre des lignes (étiquettes dans l'ordre de la
// source). Son cache dépend de leurs versions (store.ColumnOf) et non de
// tout le tableau : une saisie dans une autre colonne ne le recalcule plus
// (6,3 s → quelques ms sur un million de lignes). Une ligne ajoutée ou
// supprimée, une écriture qui peut toucher toutes les colonnes et tout
// changement de structure (colonnes, types : métadonnées) le refont.

// filterCols : colonnes c_N citées par le filtre d'un tableau.
var filterCols = regexp.MustCompile(`\bc_\d+\b`)

// colDeps donne les versions dont dépend un calcul sur tb qui lit les
// colonnes dont le nom figure dans cfg (en JSON, sans tenir compte de la
// casse : un nom de trop ne fait qu'un calcul de plus), l'ordre des lignes
// et, avec where, les colonnes du filtre du tableau.
func colDeps(tb Table, cfg any, where bool) []string {
	b, _ := json.Marshal(cfg)
	text := strings.ToLower(string(b))
	deps := []string{store.CountOf(tb.sql()), store.ColumnOf(tb.sql(), "_pos")}
	for _, c := range tb.Cols {
		name, _ := json.Marshal(c.Name)
		if strings.Contains(text, strings.ToLower(string(name))) {
			deps = append(deps, store.ColumnOf(tb.sql(), colSQL(c.ID)))
		}
	}
	if where {
		for _, c := range filterCols.FindAllString(tb.where, -1) {
			deps = append(deps, store.ColumnOf(tb.sql(), c))
		}
	}
	return deps
}
