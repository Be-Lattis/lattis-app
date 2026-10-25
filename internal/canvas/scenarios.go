package canvas

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
)

// Gestionnaire de scénarios (Données › Analyse de scénarios) : des jeux de
// valeurs nommés pour des cellules variables d'une feuille. Afficher pose
// les valeurs (une étape d'annulation) ; la synthèse crée un onglet qui
// compare les cellules résultantes d'un scénario à l'autre.
//
// Gardés dans le classeur (_lattis_info, clé « scenarios », JSON), comme les
// listes personnalisées : sur le poste seulement dans un classeur partagé
// (_lattis_info n'est pas publié avec la mise en page, hors du thème). Un
// scénario est un outil d'analyse de celui qui l'écrit ; les valeurs qu'il
// pose, elles, sont des saisies ordinaires, partagées comme les autres. Les
// adresses ne suivent pas une insertion de lignes ou de colonnes.

const scenariosKey = "scenarios"

// Scenario : un jeu de valeurs. Cells : adresses des cellules variables
// (B1, B2) ; Values : une valeur par cellule, dans la langue de la session à
// l'interface, gardée en forme du classeur.
type Scenario struct {
	Name    string   `json:"name"`
	Sheet   int64    `json:"sheet"`
	Cells   []string `json:"cells"`
	Values  []string `json:"values"`
	Comment string   `json:"comment,omitempty"`
}

func (t *tx) scenarios() []Scenario {
	raw, _, _ := t.infoValue(scenariosKey)
	out := []Scenario{}
	if raw != "" && json.Unmarshal([]byte(raw), &out) != nil {
		return []Scenario{}
	}
	return out
}

func (t *tx) setScenarios(all []Scenario) error {
	if len(all) == 0 {
		return t.setInfo(scenariosKey, "")
	}
	b, err := json.Marshal(all)
	if err != nil {
		return err
	}
	return t.setInfo(scenariosKey, string(b))
}

// Scenarios rend les scénarios de la feuille (valeurs dans la langue de la
// session).
func (m *Model) Scenarios(ctx context.Context, sheet int64) ([]Scenario, error) {
	out := []Scenario{}
	err := m.read(ctx, func(t *tx) error {
		u := t.ui()
		for _, s := range t.scenarios() {
			if s.Sheet != sheet {
				continue
			}
			vals := make([]string, len(s.Values))
			for i, v := range s.Values {
				vals[i] = u.valueOut(v)
			}
			s.Values = vals
			out = append(out, s)
		}
		return nil
	})
	return out, err
}

// WhatIfCell : une cellule d'entrée et sa valeur actuelle (session).
type WhatIfCell struct {
	Cell  string `json:"cell"`
	Value string `json:"value"`
}

// WhatIfCells vérifie une liste de cellules variables (constantes ou
// cellules saisies d'un tableau) et rend leurs valeurs actuelles : la boîte
// d'un nouveau scénario les propose.
func (m *Model) WhatIfCells(ctx context.Context, sheet int64, refs string) ([]WhatIfCell, error) {
	cells, err := parseCells(refs)
	if err != nil {
		return nil, err
	}
	out := []WhatIfCell{}
	err = m.read(ctx, func(t *tx) error {
		for _, c := range cells {
			if _, err := t.inputCell(sheet, c); err != nil {
				return err
			}
			v, err := t.cellValue(sheet, c)
			if err != nil {
				return err
			}
			out = append(out, WhatIfCell{c.addr(), t.showValue(v)})
		}
		return nil
	})
	return out, err
}

// SaveScenario ajoute (old vide) ou remplace (old : son nom actuel) un
// scénario de la feuille ; une étape d'annulation.
func (m *Model) SaveScenario(ctx context.Context, sheet int64, old string, sc Scenario) error {
	sc.Name = strings.TrimSpace(sc.Name)
	if sc.Name == "" {
		return msg.New("whatif.scenarioName", nil)
	}
	cells, err := parseCells(strings.Join(sc.Cells, ";"))
	if err != nil {
		return err
	}
	if len(cells) == 0 || len(sc.Values) != len(cells) {
		return msg.New("whatif.scenarioValues", nil)
	}
	u := m.ui(ctx)
	sc.Sheet = sheet
	sc.Cells = make([]string, len(cells))
	for i, c := range cells {
		sc.Cells[i] = c.addr()
		sc.Values[i] = freeRaw(u.valueIn(strings.TrimSpace(sc.Values[i])))
	}
	return m.Do(ctx, "Scénario", func(t *tx) error {
		for _, c := range cells {
			if _, err := t.inputCell(sheet, c); err != nil {
				return err
			}
		}
		all := t.scenarios()
		at := -1
		for i, s := range all {
			if s.Sheet != sheet {
				continue
			}
			if old != "" && strings.EqualFold(s.Name, old) {
				at = i
			} else if strings.EqualFold(s.Name, sc.Name) {
				return msg.New("whatif.scenarioTaken", msg.Args{"name": sc.Name})
			}
		}
		if old != "" && at < 0 {
			return msg.New("whatif.scenarioMissing", msg.Args{"name": old})
		}
		if at >= 0 {
			all[at] = sc
		} else {
			all = append(all, sc)
		}
		return t.setScenarios(all)
	})
}

// DeleteScenario supprime un scénario de la feuille.
func (m *Model) DeleteScenario(ctx context.Context, sheet int64, name string) error {
	return m.Do(ctx, "Supprimer le scénario", func(t *tx) error {
		all := t.scenarios()
		for i, s := range all {
			if s.Sheet == sheet && strings.EqualFold(s.Name, name) {
				return t.setScenarios(append(all[:i], all[i+1:]...))
			}
		}
		return msg.New("whatif.scenarioMissing", msg.Args{"name": name})
	})
}

func (t *tx) scenario(sheet int64, name string) (Scenario, []whatCell, error) {
	for _, s := range t.scenarios() {
		if s.Sheet == sheet && strings.EqualFold(s.Name, name) {
			cells, err := parseCells(strings.Join(s.Cells, ";"))
			if err != nil || len(cells) != len(s.Values) {
				return s, nil, msg.New("whatif.scenarioValues", nil)
			}
			return s, cells, nil
		}
	}
	return Scenario{}, nil, msg.New("whatif.scenarioMissing", msg.Args{"name": name})
}

// ShowScenario pose les valeurs d'un scénario dans ses cellules (une étape
// d'annulation).
func (m *Model) ShowScenario(ctx context.Context, sheet int64, name string) error {
	return m.Do(ctx, "Afficher le scénario", func(t *tx) error {
		s, cells, err := t.scenario(sheet, name)
		if err != nil {
			return err
		}
		return t.putInputs(sheet, cells, s.Values)
	})
}

// ScenarioSummary crée un onglet de synthèse : pour chaque scénario de la
// feuille, les valeurs de ses cellules variables et celles des cellules
// résultantes (results : adresses) qu'il donne, à côté des valeurs
// actuelles. Rend l'onglet créé ; une étape d'annulation.
func (m *Model) ScenarioSummary(ctx context.Context, sheet int64, results string) (int64, error) {
	res, err := parseCells(results)
	if err != nil {
		return 0, err
	}
	var all []Scenario
	var changing []whatCell
	// grid[i][j] : ligne i (cellules variables puis résultantes), colonne j
	// (valeurs actuelles puis chaque scénario), forme gardée.
	var grid [][]string
	err = m.trial(ctx, func(t *tx) error {
		for _, s := range t.scenarios() {
			if s.Sheet == sheet {
				all = append(all, s)
			}
		}
		if len(all) == 0 {
			return msg.New("whatif.noScenario", nil)
		}
		seen := map[whatCell]bool{}
		for _, s := range all {
			_, cells, err := t.scenario(sheet, s.Name)
			if err != nil {
				return err
			}
			for _, c := range cells {
				if !seen[c] {
					seen[c] = true
					changing = append(changing, c)
				}
			}
		}
		rows := append(append([]whatCell(nil), changing...), res...)
		read := func() ([]string, error) {
			out := make([]string, len(rows))
			for i, c := range rows {
				v, err := t.cellValue(sheet, c)
				if err != nil {
					return nil, err
				}
				out[i] = valueRaw(v)
			}
			return out, nil
		}
		cols := [][]string{}
		cur, err := read()
		if err != nil {
			return err
		}
		cols = append(cols, cur)
		for _, s := range all {
			_, cells, _ := t.scenario(sheet, s.Name)
			if err := t.putInputs(sheet, cells, s.Values); err != nil {
				return err
			}
			col, err := read()
			if err != nil {
				return err
			}
			cols = append(cols, col)
			// Retour aux valeurs actuelles avant le scénario suivant (une
			// cellule qu'il ne cite pas garde sa valeur d'origine).
			for i, c := range changing {
				if err := t.putInputs(sheet, []whatCell{c}, []string{cur[i]}); err != nil {
					return err
				}
			}
		}
		grid = make([][]string, len(rows))
		for i := range rows {
			for _, col := range cols {
				grid[i] = append(grid[i], col[i])
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	var id int64
	err = m.Do(ctx, "Synthèse de scénarios", func(t *tx) error {
		var err error
		if id, err = t.addNamedSheet(t.defaultName("name.scenarioSummary")); err != nil {
			return err
		}
		say := func(k msg.ID) string { return textRaw(msg.Format(t.nameLang(), k, nil)) }
		put := func(r, c int, raw string) error { return t.setFreeCell(id, r, c, raw) }
		bold := []whatCell{{0, 0}, {2, 0}, {3, 0}}
		if err := put(0, 0, say("whatif.summary.title")); err != nil {
			return err
		}
		if err := put(2, 1, say("whatif.summary.current")); err != nil {
			return err
		}
		for j, s := range all {
			if err := put(2, 2+j, textRaw(s.Name)); err != nil {
				return err
			}
			bold = append(bold, whatCell{2, 2 + j})
		}
		bold = append(bold, whatCell{2, 1})
		if err := put(3, 0, say("whatif.summary.changing")); err != nil {
			return err
		}
		r := 4
		for i := range grid {
			if i == len(changing) {
				if err := put(r, 0, say("whatif.summary.results")); err != nil {
					return err
				}
				bold = append(bold, whatCell{r, 0})
				r++
			}
			c := changing
			k := i
			if i >= len(changing) {
				c, k = res, i-len(changing)
			}
			if err := put(r, 0, textRaw(c[k].addr())); err != nil {
				return err
			}
			for j, v := range grid[i] {
				if err := put(r, 1+j, v); err != nil {
					return err
				}
			}
			r++
		}
		// Colonnes assez larges pour les libellés (« Valeurs actuelles »).
		var wdo, wundo []stmt
		for c := 0; c <= 1+len(all); c++ {
			wdo = append(wdo, st(`INSERT INTO _lattis_col_widths (sheet, col, width) VALUES (?, ?, ?)`, id, c, 130))
			wundo = append(wundo, st(`DELETE FROM _lattis_col_widths WHERE sheet = ? AND col = ?`, id, c))
		}
		if err := t.apply(wdo, wundo); err != nil {
			return err
		}
		lines, err := t.sheetLines(id)
		if err != nil {
			return err
		}
		for _, b := range bold {
			s := Style{"b": true}
			if b.Row == 2 {
				s["al"] = "right" // en-têtes au-dessus des nombres
			}
			if err := t.styleFree(id, b.Row, b.Col, s, lines.at(b.Row, b.Col)); err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}
