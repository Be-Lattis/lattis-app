package canvas

import (
	"encoding/json"

	"github.com/lalamefine/lattis/internal/msg"
)

// Graphiques et tableaux croisés d'un classeur importé (.xlsx) : posés
// quand tous les tableaux existent (leur source en est un). Un objet dont
// la source ne se lit pas est passé, avec un avertissement.

// ImportChart est un graphique à poser (Config.Source : un tableau importé).
type ImportChart struct {
	Row, Col int
	Config   ChartConfig
}

// ImportPivot est un tableau croisé à poser : Row, Col est sa cellule en haut
// à gauche, filtres de rapport compris.
type ImportPivot struct {
	Row, Col int
	Config   PivotConfig
}

func (m *Model) importObjects(t *tx, sheets []ImportSheet, ids []int64) ([]string, error) {
	var warn []string
	for si, s := range sheets {
		for _, p := range s.Pivots {
			cfg := p.Config.created().stored()
			cfg.Lang = t.captionLang()
			err := t.prepareCalc(&cfg)
			if err == nil {
				_, err = t.pivotPlan(cfg)
			}
			if err != nil {
				warn = append(warn, m.say("import.pivotDropped", msg.Args{"sheet": s.Name, "err": err}))
				continue
			}
			var id int64
			if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_pivots`).Scan(&id); err != nil {
				return nil, err
			}
			if cfg.Name == "" {
				name, err := t.pivotName()
				if err != nil {
					return nil, err
				}
				cfg.Name = name
			}
			b, err := json.Marshal(cfg)
			if err != nil {
				return nil, err
			}
			if err := t.apply(
				[]stmt{st(`INSERT INTO _lattis_pivots (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, ids[si], max(p.Row, 0), max(p.Col, 0), string(b))},
				[]stmt{st(`DELETE FROM _lattis_pivots WHERE id = ?`, id)},
			); err != nil {
				return nil, err
			}
		}
	}
	// Les graphiques après les TCD : un graphique peut en tirer ses données.
	for si, s := range sheets {
		for _, c := range s.Charts {
			if _, err := t.chartData(c.Config); err != nil {
				warn = append(warn, m.say("import.chartDropped", msg.Args{"sheet": s.Name, "err": err}))
				continue
			}
			var id int64
			if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_charts`).Scan(&id); err != nil {
				return nil, err
			}
			b, err := json.Marshal(c.Config)
			if err != nil {
				return nil, err
			}
			if err := t.apply(
				[]stmt{st(`INSERT INTO _lattis_charts (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, ids[si], max(c.Row, 0), max(c.Col, 0), string(b))},
				[]stmt{st(`DELETE FROM _lattis_charts WHERE id = ?`, id)},
			); err != nil {
				return nil, err
			}
		}
	}
	return warn, nil
}
