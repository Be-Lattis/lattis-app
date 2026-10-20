package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// Onglet à choix (docs/conception/regrouper-onglets-semblables.md §6.3,
// lot 3) : une feuille porte une catégorie (« Année ») et une valeur
// choisie (« 2025 ») sur son onglet (« Ventes · 2025 ▾ »).
//
//   - Le choix filtre tous les tableaux de la feuille qui ont une colonne de
//     ce nom (loadFilter), et donc ce qui suit leur filtre : la grille,
//     SOUS.TOTAL, les graphiques, la recherche, le collage. Les TCD de la
//     feuille reçoivent un filtre imposé (pivotChoice), même quand leur
//     tableau source est sur une autre feuille (qui n'en est pas filtrée :
//     « Filtrer l'onglet par ce champ » sur un TCD). Les formules libres
//     de la feuille voient les tableaux filtrés (schemaFor) ; celles des
//     autres feuilles, le SQL et les scripts voient toutes les lignes.
//   - Ce n'est pas un filtre de l'utilisateur : « Effacer le filtre » le
//     laisse, et une ligne ajoutée (saisie sous le tableau ou dans les lignes
//     masquées, collage, insertion) reçoit la valeur choisie (insertRow).
//   - La catégorie désigne sa colonne source par identifiants (tableau,
//     colonne) : renommer la colonne renomme la catégorie, et l'onglet suit
//     (décision 12.4). Le nom gardé ne sert qu'en repli, si la colonne
//     source disparaît.
//   - Changer de valeur est de la navigation, hors journal comme le zoom ;
//     définir ou retirer la catégorie, et « Nouvelle année… », sont annulables.
//   - Classeur partagé : la catégorie est publiée avec la mise en page, la
//     valeur choisie reste propre à chaque poste (remote/workbook/choice.go ;
//     une catégorie reçue sans valeur prend la dernière, SettleChoices).
//   - La catégorie peut aussi être une formule sur les colonnes de chaque
//     ligne (« Choisir par › Une formule… », =ANNEE([Date]) ;
//     choiceformula.go) : sans colonne, elle est suivie par les tableaux qui
//     ont les colonnes de la formule.

// MaxChoiceValues borne les colonnes proposées par « Choisir par… » (nombre
// de valeurs distinctes).
const MaxChoiceValues = 100

// choiceRaw est la colonne choice de _lattis_sheets (JSON).
type choiceRaw struct {
	Tbl   int64  `json:"tbl,omitempty"`
	Col   int64  `json:"col,omitempty"`
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
	All   bool   `json:"all,omitempty"`
	// Fresh : valeur posée par « Nouvelle année… » et pas encore quittée ;
	// à l'ouverture, elle reste affichée même sans ligne (décision 12.2).
	Fresh bool `json:"fresh,omitempty"`
	// Formula : catégorie calculée (anglais canonique, colonnes citées par
	// leur nom : =YEAR([Date])) ; Tbl et Col sont alors vides.
	Formula string `json:"formula,omitempty"`
	// Keep : lignes ajoutées sous une catégorie calculée depuis le dernier
	// changement de valeur, montrées même hors de la valeur choisie (une
	// ligne qu'on commence à saisir ne disparaît pas ; choiceformula.go).
	Keep []int64 `json:"keep,omitempty"`
}

// SheetChoice décrit l'onglet à choix d'une feuille.
type SheetChoice struct {
	// Name : nom de la catégorie (celui de sa colonne source) ; Type : type
	// de cette colonne ; Table : tableau de la colonne source.
	Name  string    `json:"name"`
	Type  grid.Type `json:"type"`
	Table string    `json:"table,omitempty"`
	// Value : valeur choisie (texte affiché) ; All : « (Toutes) », aucun
	// filtre (la colonne réapparaît).
	Value string `json:"value"`
	All   bool   `json:"all"`
	Fresh bool   `json:"fresh,omitempty"`
	// Formula : catégorie calculée par une formule (affichée en français),
	// sans colonne ; vide pour une catégorie par colonne.
	Formula string `json:"formula,omitempty"`

	en   string  // formule en anglais canonique
	keep []int64 // voir choiceRaw.Keep
}

// tableChoice : un tableau qui suit le choix de l'onglet. Col est la
// colonne de catégorie (ID 0 pour une catégorie calculée) ; Expr, le texte
// de la catégorie de chaque ligne en SQL ; Name, le nom de la catégorie.
type tableChoice struct {
	Col   Column
	Name  string
	Expr  string
	Value string
	All   bool
	Keep  []int64
}

// formula : la catégorie est calculée (pas de colonne).
func (c *tableChoice) formula() bool { return c.Col.ID == 0 }

// cond : condition SQL des lignes de la valeur choisie (et des lignes
// gardées d'une catégorie calculée).
func (c *tableChoice) cond() string {
	w := c.Expr + " = " + sqlLiteral(c.Value)
	if len(c.Keep) == 0 {
		return w
	}
	ids := make([]string, len(c.Keep))
	for i, id := range c.Keep {
		ids[i] = strconv.FormatInt(id, 10)
	}
	return "(" + w + " OR id IN (" + strings.Join(ids, ", ") + "))"
}

// columnChoice : le choix suivi par la colonne de catégorie c.
func columnChoice(c Column, ch *SheetChoice) *tableChoice {
	return &tableChoice{Col: c, Name: c.Name, Expr: textExpr(c), Value: ch.Value, All: ch.All}
}

// followChoice : comment le tableau suit le choix ch (nil : il ne le suit
// pas, faute de colonne de catégorie ou des colonnes de la formule).
func (t *tx) followChoice(tb Table, ch *SheetChoice) (*tableChoice, error) {
	if ch.en == "" {
		if c, ok := choiceColumn(tb, ch.Name); ok {
			return columnChoice(c, ch), nil
		}
		return nil, nil
	}
	expr, ok, err := t.rowLabelSQL(tb, ch.en)
	if err != nil || !ok {
		return nil, err
	}
	return &tableChoice{Name: ch.Name, Expr: expr, Value: ch.Value, All: ch.All, Keep: ch.keep}, nil
}

// ChoiceCandidate est une colonne proposée par « Choisir par… ».
type ChoiceCandidate struct {
	Table  string `json:"table"`
	Column string `json:"column"`
	Values int    `json:"values"` // valeurs distinctes (non vides)
}

// choiceColumn trouve la colonne de catégorie d'un tableau (même nom, sans
// tenir compte de la casse ; pas une colonne calculée).
func choiceColumn(tb Table, name string) (Column, bool) {
	for _, c := range tb.Cols {
		if c.Formula == "" && strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return Column{}, false
}

// readChoice lit la catégorie enregistrée d'une feuille (nil : aucune).
func (t *tx) readChoice(sheet int64) (*choiceRaw, error) {
	var raw sql.NullString
	err := t.row(`SELECT choice FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (!raw.Valid || raw.String == "")) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var r choiceRaw
	if json.Unmarshal([]byte(raw.String), &r) != nil {
		return nil, nil // illisible : ignoré
	}
	return &r, nil
}

// sheetChoice donne l'onglet à choix d'une feuille, nil sans choix (une
// lecture par génération du Store).
func (t *tx) sheetChoice(sheet int64) (*SheetChoice, error) {
	return cached(t, fmt.Sprintf("choice:%d", sheet), func() (*SheetChoice, error) {
		r, err := t.readChoice(sheet)
		if err != nil || r == nil {
			return nil, err
		}
		return t.resolveChoice(r)
	})
}

// resolveChoice : nom et type actuels de la colonne source.
func (t *tx) resolveChoice(r *choiceRaw) (*SheetChoice, error) {
	ch := &SheetChoice{Name: r.Name, Type: grid.Text, Value: r.Value, All: r.All, Fresh: r.Fresh}
	if r.Formula != "" {
		ch.Formula, ch.en, ch.keep = displayFormula(r.Formula), r.Formula, r.Keep
	} else if r.Tbl != 0 {
		var name, typ, table string
		err := t.row(`SELECT c.name, c.type, t.name FROM _lattis_columns c JOIN _lattis_tables t ON t.id = c.tbl WHERE c.tbl = ? AND c.id = ?`, r.Tbl, r.Col).Scan(&name, &typ, &table)
		switch {
		case err == nil:
			ch.Name, ch.Type, ch.Table = name, grid.Type(typ), table
		case !errors.Is(err, sql.ErrNoRows):
			return nil, err
		}
	}
	if strings.TrimSpace(ch.Name) == "" {
		return nil, nil
	}
	return ch, nil
}

// writeChoice remplace la catégorie de la feuille dans le journal
// (annulable) ; nil la retire.
func (t *tx) writeChoice(sheet int64, r *choiceRaw) error {
	var old sql.NullString
	if err := t.row(`SELECT choice FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&old); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return msg.Wrap(ErrSheet, "sheet.notFound", nil)
		}
		return err
	}
	var neu, prev any
	if r != nil {
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		neu = string(b)
	}
	if old.Valid {
		prev = old.String
	}
	if neu == prev {
		return nil
	}
	// Cellules par valeur (choicecells.go) : celles de l'ancienne valeur
	// rangées, celles de la nouvelle montrées, dans la même étape.
	var before *choiceRaw
	if old.Valid && old.String != "" {
		var o choiceRaw
		if json.Unmarshal([]byte(old.String), &o) == nil {
			before = &o
		}
	}
	if before != nil {
		if err := t.storeChoiceCells(sheet, choiceKey(before)); err != nil {
			return err
		}
	}
	q := `UPDATE _lattis_sheets SET choice = ? WHERE id = ?`
	if err := t.apply([]stmt{st(q, neu, sheet)}, []stmt{st(q, prev, sheet)}); err != nil {
		return err
	}
	if r != nil && (before == nil || choiceKey(before) != choiceKey(r)) {
		return t.showChoiceCells(sheet, choiceKey(r))
	}
	return nil
}

// setSheetChoice fait de la feuille un onglet à choix par la colonne column
// du tableau table, réglé sur value ("" : « (Toutes) »), dans la
// transaction en cours (une étape d'annulation avec le reste de l'action).
// Pour la fusion des onglets (lot 1), qui pose la catégorie sur la feuille
// commune une fois le tableau créé. Les garde-fous (feuille protégée) sont
// ceux de l'action appelante.
func (t *tx) setSheetChoice(sheet int64, table string, column string, value string) error {
	tb, err := t.tableByName(table)
	if err != nil {
		return err
	}
	var col *Column
	for i := range tb.Cols {
		if strings.EqualFold(tb.Cols[i].Name, column) {
			col = &tb.Cols[i]
		}
	}
	if col == nil {
		return msg.Wrap(ErrSheet, "sheet.choiceColumnNotFound", msg.Args{"column": column, "table": tb.Name})
	}
	if col.Formula != "" {
		return msg.Wrap(ErrSheet, "sheet.choiceComputedColumn", msg.Args{"column": col.Name})
	}
	return t.writeChoice(sheet, &choiceRaw{Tbl: tb.ID, Col: col.ID, Name: col.Name, Value: value, All: value == ""})
}

// SetSheetChoice fait de la feuille un onglet à choix par une colonne d'un
// de ses tableaux (« Choisir par… », « Filtrer l'onglet par cette
// colonne »), ou du tableau source d'un de ses TCD posé ailleurs (« Filtrer
// l'onglet par ce champ »), réglé sur value ("" : toutes les valeurs) ; une
// étape d'annulation.
func (m *Model) SetSheetChoice(ctx context.Context, sheet int64, table, column, value string) error {
	return m.Do(ctx, fmt.Sprintf("Choisir par « %s »", column), func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		return t.setSheetChoice(sheet, table, column, value)
	})
}

// ClearSheetChoice retire l'onglet à choix : tous les tableaux de la feuille
// montrent de nouveau toutes leurs lignes ; une étape d'annulation.
func (m *Model) ClearSheetChoice(ctx context.Context, sheet int64) error {
	return m.Do(ctx, "Ne plus choisir", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		return t.writeChoice(sheet, nil)
	})
}

// SheetChoice donne l'onglet à choix de la feuille (nil : aucun).
func (m *Model) SheetChoice(ctx context.Context, sheet int64) (*SheetChoice, error) {
	var out *SheetChoice
	err := m.read(ctx, func(t *tx) error {
		var err error
		out, err = t.sheetChoice(sheet)
		return err
	})
	return out, err
}

// ChooseSheetValue règle l'onglet à choix sur une valeur (all : « (Toutes) »).
// C'est de la navigation, comme changer d'onglet ou de zoom : hors du
// journal d'annulation, permise sur une feuille protégée, mais le classeur
// est à enregistrer (la dernière valeur choisie est gardée dans le fichier).
func (m *Model) ChooseSheetValue(ctx context.Context, sheet int64, value string, all bool) error {
	changed := false
	err := m.db.Write(ctx, func(sqlTx *sql.Tx) error {
		var raw sql.NullString
		if err := sqlTx.QueryRowContext(ctx, `SELECT choice FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&raw); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return msg.Wrap(ErrSheet, "sheet.notFound", nil)
			}
			return err
		}
		var r choiceRaw
		if !raw.Valid || raw.String == "" || json.Unmarshal([]byte(raw.String), &r) != nil {
			return msg.Wrap(ErrSheet, "sheet.noChoiceTabHint", nil)
		}
		if all {
			value = ""
		}
		if r.All == all && r.Value == value && !r.Fresh {
			return nil
		}
		r.Value, r.All, r.Fresh, r.Keep = value, all, false, nil
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		changed = true
		if _, err = sqlTx.ExecContext(ctx, `UPDATE _lattis_sheets SET choice = ? WHERE id = ?`, string(b), sheet); err != nil {
			return err
		}
		// Les cellules par valeur montrent celles de la nouvelle valeur.
		return showChoiceCellsRaw(ctx, sqlTx, sheet, choiceKey(&r))
	})
	if err == nil && changed {
		m.protMu.Lock()
		m.protGen++
		m.protMu.Unlock()
	}
	return err
}

// choiceTables : tableaux de la feuille qui suivent le choix.
func (t *tx) choiceTables(sheet int64) ([]Table, error) {
	tables, err := t.tables(sheet)
	if err != nil {
		return nil, err
	}
	var out []Table
	for _, tb := range tables {
		if tb.choice != nil {
			out = append(out, tb)
		}
	}
	return out, nil
}

// pivotSources : tableaux sources des TCD de la feuille posés sur une autre
// feuille (« Filtrer l'onglet par ce champ » sur un TCD dont la source est
// ailleurs), sans doublon ; une source introuvable est passée.
func (t *tx) pivotSources(sheet int64) ([]Table, error) {
	rs, err := t.query(`SELECT config FROM _lattis_pivots WHERE sheet = ? ORDER BY id`, sheet)
	if err != nil {
		return nil, err
	}
	var names []string
	for rs.Next() {
		var raw string
		if err := rs.Scan(&raw); err != nil {
			rs.Close()
			return nil, err
		}
		var cfg PivotConfig
		if json.Unmarshal([]byte(raw), &cfg) == nil && cfg.Source != "" && !slices.Contains(names, cfg.Source) {
			names = append(names, cfg.Source)
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	var out []Table
	for _, n := range names {
		tb, err := t.tableByName(n)
		if err != nil || tb.Sheet == sheet {
			continue
		}
		out = append(out, tb)
	}
	return out, nil
}

// choiceSources : les tableaux qui donnent les valeurs de l'onglet à choix,
// ceux de la feuille qui suivent le choix (choiceTables), puis ceux d'autres
// feuilles qui ont la colonne de catégorie : la colonne source du choix et
// les sources des TCD de la feuille. Ces derniers ne sont pas filtrés (leur
// choice n'est réglé ici que pour lire la colonne de catégorie).
func (t *tx) choiceSources(sheet int64, ch *SheetChoice, r *choiceRaw) ([]Table, error) {
	tables, err := t.choiceTables(sheet)
	if err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	for _, tb := range tables {
		seen[tb.ID] = true
	}
	var other []Table
	if r != nil && r.Tbl != 0 {
		if tb, err := t.table(r.Tbl); err == nil && tb.Sheet != sheet {
			other = append(other, tb)
		}
	}
	srcs, err := t.pivotSources(sheet)
	if err != nil {
		return nil, err
	}
	for _, tb := range append(other, srcs...) {
		if seen[tb.ID] {
			continue
		}
		seen[tb.ID] = true
		tc, err := t.followChoice(tb, ch)
		if err != nil {
			return nil, err
		}
		if tc != nil {
			tb.choice = tc
			tables = append(tables, tb)
		}
	}
	return tables, nil
}

// choiceList : valeurs de la liste de validation (dv) d'une colonne.
func (t *tx) choiceList(tbl, col int64) ([]string, error) {
	s, err := t.tableStyle(tbl, 0, col)
	if err != nil {
		return nil, err
	}
	var out []string
	if l, ok := s["dv"].([]any); ok {
		for _, v := range l {
			if x, ok := v.(string); ok && x != "" {
				out = append(out, x)
			}
		}
	}
	return out, nil
}

// SheetChoiceValues donne les valeurs proposées par la flèche de l'onglet :
// celles des tableaux qui suivent le choix (sans les vides), dans l'ordre de
// la colonne (ordre d'apparition pour un texte, qui suit les onglets d'une
// fusion : Janvier avant Février), puis celles de la liste de validation de
// la colonne source qui n'ont encore aucune ligne (une année ajoutée par
// « Nouvelle année… », Count 0).
func (m *Model) SheetChoiceValues(ctx context.Context, sheet int64) ([]FilterValue, error) {
	out := []FilterValue{}
	err := m.read(ctx, func(t *tx) error {
		var err error
		out, err = t.choiceValues(sheet)
		return err
	})
	return out, err
}

func (t *tx) choiceValues(sheet int64) ([]FilterValue, error) {
	out := []FilterValue{}
	ch, err := t.sheetChoice(sheet)
	if err != nil || ch == nil {
		return out, err
	}
	r, err := t.readChoice(sheet)
	if err != nil {
		return out, err
	}
	tables, err := t.choiceSources(sheet, ch, r)
	if err != nil {
		return out, err
	}
	// Le tableau de la colonne source d'abord : il donne l'ordre.
	slices.SortStableFunc(tables, func(a, b Table) int {
		ka, kb := a.Name != ch.Table, b.Name != ch.Table
		switch {
		case !ka && kb:
			return -1
		case ka && !kb:
			return 1
		}
		return 0
	})
	at := map[string]int{}
	for _, tb := range tables {
		c := tb.choice.Col
		order := "min(_pos)"
		if !tb.choice.formula() && c.Type != grid.Text {
			order = "min(" + colSQL(c.ID) + ")"
		}
		rs, err := t.query(`SELECT `+tb.choice.Expr+` AS v, count(*) FROM `+tb.sql()+` GROUP BY v HAVING v <> '' ORDER BY `+order+` LIMIT ?`, MaxFilterValues)
		if err != nil {
			return out, err
		}
		for rs.Next() {
			var v FilterValue
			if err := rs.Scan(&v.Text, &v.Count); err != nil {
				rs.Close()
				return out, err
			}
			if i, ok := at[v.Text]; ok {
				out[i].Count += v.Count
				continue
			}
			at[v.Text] = len(out)
			out = append(out, v)
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return out, err
		}
	}
	if r != nil && r.Tbl != 0 {
		list, err := t.choiceList(r.Tbl, r.Col)
		if err != nil {
			return out, err
		}
		for _, v := range list {
			if _, ok := at[v]; !ok {
				at[v] = len(out)
				out = append(out, FilterValue{Text: v})
			}
		}
	}
	// Catégorie calculée : ordre naturel (2024 avant 2025, janvier avant
	// février), faute d'ordre de saisie d'une colonne.
	if ch.en != "" {
		sortChoiceValues(out)
	}
	// La valeur choisie reste proposée, même sans ligne.
	if !ch.All && ch.Value != "" {
		if _, ok := at[ch.Value]; !ok {
			out = append(out, FilterValue{Text: ch.Value})
		}
	}
	return out, nil
}

// NewSheetChoiceValue ajoute une valeur à la catégorie (« Nouvelle
// année… ») : elle rejoint la liste de validation de la colonne source (et
// de la colonne de catégorie des autres tableaux qui en ont une), et
// l'onglet se règle dessus ; la feuille paraît vide, prête à être remplie.
// Une étape d'annulation (Ctrl+Z revient à la valeur d'avant).
func (m *Model) NewSheetChoiceValue(ctx context.Context, sheet int64, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return msg.Wrap(ErrSheet, "sheet.emptyChoiceValue", nil)
	}
	return m.Do(ctx, "Nouvelle valeur de l'onglet", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		r, err := t.readChoice(sheet)
		if err != nil {
			return err
		}
		if r == nil {
			return msg.Wrap(ErrSheet, "sheet.noChoiceTab", nil)
		}
		if r.Formula != "" {
			return msg.Wrap(ErrSheet, "sheet.choiceComputedValue", msg.Args{"column": r.Name})
		}
		vals, err := t.choiceValues(sheet)
		if err != nil {
			return err
		}
		ch, err := t.sheetChoice(sheet)
		if err != nil || ch == nil {
			return err
		}
		// Les tableaux d'autres feuilles (source d'un TCD de la feuille)
		// valident le type de la valeur, mais leur liste n'est pas touchée.
		tables, err := t.choiceSources(sheet, ch, r)
		if err != nil {
			return err
		}
		// Le type de la colonne doit accepter la valeur (2026 dans une
		// colonne Année entière).
		for _, tb := range tables {
			if _, err := grid.Parse(tb.choice.Col.Type, value); err != nil && (r.Tbl == 0 || tb.ID == r.Tbl) {
				return msg.Wrap(ErrSheet, "sheet.choiceValueInvalid", msg.Args{"value": value, "column": tb.choice.Col.Name, "err": err})
			}
		}
		list := make([]any, 0, len(vals)+1)
		have := false
		for _, v := range vals {
			list = append(list, v.Text)
			have = have || v.Text == value
		}
		if !have {
			list = append(list, value)
		}
		for _, tb := range tables {
			if tb.Sheet != sheet {
				continue
			}
			c := tb.choice.Col
			old, err := t.choiceList(tb.ID, c.ID)
			if err != nil {
				return err
			}
			if tb.ID != r.Tbl && len(old) == 0 {
				continue // pas de liste déroulante : rien à compléter
			}
			if slices.Contains(old, value) {
				continue
			}
			if err := t.styleTable(tb.ID, 0, c.ID, Style{"dv": list}, nil); err != nil {
				return err
			}
		}
		r.Value, r.All, r.Fresh = value, false, true
		return t.writeChoice(sheet, r)
	})
}

// SheetChoiceCandidates donne les colonnes des tableaux de la feuille, puis
// des tableaux sources de ses TCD posés sur d'autres feuilles, qui peuvent
// servir de choix d'onglet : saisies (pas calculées), avec au moins une
// valeur et au plus MaxChoiceValues valeurs distinctes.
func (m *Model) SheetChoiceCandidates(ctx context.Context, sheet int64) ([]ChoiceCandidate, error) {
	out := []ChoiceCandidate{}
	err := m.read(ctx, func(t *tx) error {
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		srcs, err := t.pivotSources(sheet)
		if err != nil {
			return err
		}
		for _, tb := range append(tables, srcs...) {
			for _, c := range tb.Cols {
				if c.Formula != "" {
					continue
				}
				var n int
				q := `SELECT count(*) FROM (SELECT DISTINCT ` + textExpr(c) + ` AS v FROM ` + tb.sql() + ` WHERE ` + colSQL(c.ID) + ` IS NOT NULL LIMIT ?)`
				if err := t.row(q, MaxChoiceValues+1).Scan(&n); err != nil {
					return err
				}
				if n >= 1 && n <= MaxChoiceValues {
					out = append(out, ChoiceCandidate{Table: tb.Name, Column: c.Name, Values: n})
				}
			}
		}
		return nil
	})
	return out, err
}

// SettleChoices règle, à l'ouverture du classeur, les onglets à choix dont
// la dernière valeur choisie n'a plus aucune ligne ni place dans la liste de
// validation : ils montrent la dernière valeur de la liste. Juste après
// « Nouvelle année… » (Fresh), la valeur ajoutée reste affichée même vide
// (décision 12.2 : l'onglet reprend la dernière valeur choisie, sauf juste
// après « Nouvelle année… »). Hors journal ; le classeur n'en est pas
// modifié pour autant.
func (m *Model) SettleChoices(ctx context.Context) error {
	type fix struct {
		sheet int64
		r     choiceRaw
	}
	var fixes []fix
	err := m.read(ctx, func(t *tx) error {
		rs, err := t.query(`SELECT id FROM _lattis_sheets WHERE choice IS NOT NULL AND choice <> ''`)
		if err != nil {
			return err
		}
		var ids []int64
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return err
			}
			ids = append(ids, id)
		}
		rs.Close()
		for _, id := range ids {
			r, err := t.readChoice(id)
			if err != nil {
				return err
			}
			if r == nil || r.All || r.Fresh {
				continue
			}
			vals, err := t.choiceValues(id)
			if err != nil {
				return err
			}
			known := false
			for _, v := range vals {
				known = known || (v.Text == r.Value && v.Count > 0)
			}
			if r.Tbl != 0 && !known {
				list, err := t.choiceList(r.Tbl, r.Col)
				if err != nil {
					return err
				}
				known = slices.Contains(list, r.Value)
			}
			if known || len(vals) == 0 {
				continue
			}
			last := ""
			for _, v := range vals {
				if v.Count > 0 || last == "" {
					last = v.Text
				}
			}
			r.Value = last
			fixes = append(fixes, fix{id, *r})
		}
		return nil
	})
	if err != nil || len(fixes) == 0 {
		return err
	}
	return m.db.Write(ctx, func(sqlTx *sql.Tx) error {
		for _, f := range fixes {
			b, err := json.Marshal(f.r)
			if err != nil {
				return err
			}
			if _, err := sqlTx.ExecContext(ctx, `UPDATE _lattis_sheets SET choice = ? WHERE id = ?`, string(b), f.sheet); err != nil {
				return err
			}
			if err := showChoiceCellsRaw(ctx, sqlTx, f.sheet, choiceKey(&f.r)); err != nil {
				return err
			}
		}
		return nil
	})
}

// imposeChoice donne à une ligne ajoutée la valeur choisie de l'onglet (une
// saisie hors du type de la colonne y reste en erreur, comme une saisie).
func (t *tx) imposeChoice(tb Table, rid int64) error {
	ch := tb.choice
	if ch == nil || ch.All || ch.Value == "" {
		return nil
	}
	if ch.formula() {
		return t.keepChoiceRow(tb.Sheet, rid)
	}
	v, err := grid.Parse(ch.Col.Type, ch.Value)
	if err != nil {
		return t.setValue(tb, rid, ch.Col, nil, ch.Value)
	}
	return t.setValue(tb, rid, ch.Col, v, "")
}

// choiceValue est la valeur choisie d'une feuille pour ONGLET.CHOIX(),
// avec le type de sa colonne source ; vide pour « (Toutes) ». Celle d'une
// catégorie calculée est un nombre quand elle s'écrit comme un nombre
// (=ANNEE([Date]) : 2025), un texte sinon.
func choiceValue(ch *SheetChoice) formula.Value {
	if ch.All {
		return formula.Value{}
	}
	if ch.en != "" {
		if f, ok := frenchNumber(ch.Value); ok {
			return formula.Num(f)
		}
		return formula.Str(ch.Value)
	}
	v, err := grid.Parse(ch.Type, ch.Value)
	if err != nil {
		return formula.Str(ch.Value)
	}
	return formula.FromSQL(v, ch.Type)
}

// pivotChoice impose le choix de l'onglet de la feuille sheet à un tableau
// croisé posé dessus, comme un filtre de rapport, quand son tableau source
// a la colonne de catégorie, où que soit ce tableau (sur une autre feuille,
// il n'est pas filtré pour autant : seul le TCD suit). « Suivre l'année »
// décoché sur la source ne compte que si elle est sur la feuille du TCD.
func (t *tx) pivotChoice(sheet int64, cfg *PivotConfig) error {
	cfg.Choice = nil
	ch, err := t.sheetChoice(sheet)
	if err != nil || ch == nil || ch.All {
		return err
	}
	tb, err := t.tableByName(cfg.Source)
	if err != nil {
		return nil // source introuvable : l'erreur du TCD le dira
	}
	if tb.Sheet == sheet && tb.NoFollow {
		return nil
	}
	if ch.en != "" {
		// Catégorie calculée : filtre par la formule, si la source a ses colonnes.
		if rowFormulaFits(tb, ch.en) {
			cfg.Choice = &PivotFilter{Field: ch.Name, Formula: ch.en, Values: []string{ch.Value}}
		}
		return nil
	}
	if c, ok := choiceColumn(tb, ch.Name); ok {
		cfg.Choice = &PivotFilter{Field: c.Name, Values: []string{ch.Value}}
	}
	return nil
}
