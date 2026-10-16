package canvas

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/lalamefine/slitable/internal/grid"
)

// pivotFormats donne le format de nombre par défaut de chaque colonne d'un
// tableau croisé calculé (cells) : comme Excel, une valeur reprend le format
// de sa colonne source (€, %, décimales) ; à défaut, une moyenne ou un
// agrégat de décimaux s'affiche avec deux décimales (au lieu de
// 3979,08045977011). Ajouter / Réduire une décimale pose ensuite un style
// de cellule, qui l'emporte.
func (t *tx) pivotFormats(cfg PivotConfig, cells [][]string) ([]string, error) {
	if len(cells) == 0 || len(cfg.Values) == 0 {
		return nil, nil
	}
	p, err := t.pivotPlan(cfg)
	if err != nil {
		return nil, nil // déjà signalé par le calcul
	}
	labels := max(1, len(p.rows))
	out := make([]string, len(cells[0]))
	byValue := make([]string, len(cfg.Values))
	for vi, v := range cfg.Values {
		if byValue[vi], err = t.pivotValueFormat(p.tb, v, p.vals[vi]); err != nil {
			return nil, err
		}
	}
	// Les colonnes de mesure parcourent les valeurs dans l'ordre (par valeur
	// du champ des colonnes, puis les totaux) : la j-ième est la valeur j % n.
	for j := labels; j < len(out); j++ {
		out[j] = byValue[(j-labels)%len(cfg.Values)]
	}
	return out, nil
}

func (t *tx) pivotValueFormat(tb Table, v PivotValue, c *Column) (string, error) {
	if v.Show != "" || c == nil || v.Agg == "count" || v.Agg == "countDistinct" {
		return "", nil
	}
	var raw string
	err := t.row(`SELECT style FROM _slitable_styles WHERE tbl = ? AND rid = 0 AND col = ?`, tb.ID, c.ID).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	f := map[string]any{}
	if raw != "" {
		var st map[string]any
		if json.Unmarshal([]byte(raw), &st) == nil {
			for _, k := range []string{"nf", "dp"} {
				if x, ok := st[k]; ok && x != nil {
					f[k] = x
				}
			}
		}
	}
	if len(f) == 0 && (v.Agg == "avg" || c.Type == grid.Decimal) {
		f["dp"] = 2
	}
	if len(f) == 0 {
		return "", nil
	}
	b, err := json.Marshal(f)
	return string(b), err
}
