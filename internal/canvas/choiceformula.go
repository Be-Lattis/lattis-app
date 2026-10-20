package canvas

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// Catégorie calculée par une formule (docs/conception/
// regrouper-onglets-semblables.md §6.6) : l'onglet à choix (« Choisir par ›
// Une formule… ») et les champs calculés des tableaux croisés regroupent
// les lignes par le résultat d'une formule sur leurs colonnes,
// =ANNEE([Date]), =TEXTE([Date];"mmmm"), =SI([Montant]>1000;"Gros";"Petit").
//
//   - La formule cite les colonnes par leur nom, comme une colonne calculée
//     ([Date]) ; elle est gardée en anglais canonique. Elle s'applique à
//     tout tableau qui a ces colonnes : c'est ce qui décide qu'un tableau de
//     la feuille suit l'onglet (comme « a la colonne Année » pour une
//     catégorie par colonne).
//   - Elle est compilée en SQL pour chaque tableau (comme une colonne
//     calculée), et son résultat comparé en texte affiché (slt_label :
//     2025, « VRAI », « #VALEUR! »).
//   - Pas de colonne à remplir : une ligne ajoutée sous l'onglet ne reçoit
//     rien d'office. Pour qu'elle ne disparaisse pas pendant qu'on la saisit
//     (=ANNEE([Date]) vaut autre chose tant que la date est vide), elle est
//     gardée visible (choiceRaw.Keep) jusqu'au prochain changement de valeur
//     de l'onglet.

// maxChoiceKeep borne les lignes gardées visibles sous une catégorie
// calculée (un grand collage au-delà se range selon sa formule).
const maxChoiceKeep = 500

// formulaColumns donne les colonnes citées par leur seul nom ([Date],
// [@Date], [Date][1:3]) dans une formule, sans doublon.
func formulaColumns(n formula.Node) []string {
	var out []string
	add := func(r formula.Ref) {
		if r.Table == "" && r.Col != "" && !slices.ContainsFunc(out, func(s string) bool { return strings.EqualFold(s, r.Col) }) {
			out = append(out, r.Col)
		}
	}
	var walk func(n formula.Node)
	walk = func(n formula.Node) {
		switch n := n.(type) {
		case formula.Ref:
			add(n)
		case formula.RowRange:
			add(n.Ref)
		case formula.Unary:
			walk(n.X)
		case formula.Binary:
			walk(n.L)
			walk(n.R)
		case formula.Call:
			for _, a := range n.Args {
				walk(a)
			}
		}
	}
	walk(n)
	return out
}

// rowFormulaFits : le tableau a toutes les colonnes citées par la formule
// en (anglais canonique), et elle en cite au moins une.
func rowFormulaFits(tb Table, en string) bool {
	n, err := formula.Parse(en, formula.EN)
	if err != nil {
		return false
	}
	return len(missingColumns(tb, formulaColumns(n))) == 0 && len(formulaColumns(n)) > 0
}

// missingColumns : colonnes de cols que le tableau n'a pas.
func missingColumns(tb Table, cols []string) []string {
	var out []string
	for _, c := range cols {
		if colIndex(tb, c) < 0 {
			out = append(out, c)
		}
	}
	return out
}

// rowValueSQL compile la formule en (anglais canonique) pour chaque ligne du
// tableau : une expression SQL autonome (paramètres écrits en littéraux),
// qui lit les colonnes de la ligne sous le nom du tableau (t_3.c_1). Faux
// si le tableau n'a pas les colonnes de la formule, ou pendant le calcul
// d'un autre filtre calculé (pas de récursion : la compilation relit les
// tableaux et leurs filtres).
func (t *tx) rowValueSQL(tb Table, en string) (string, bool, error) {
	if !rowFormulaFits(tb, en) || t.inComputedFilter {
		return "", false, nil
	}
	n, err := formula.Parse(en, formula.EN)
	if err != nil {
		return "", false, nil
	}
	t.inComputedFilter = true
	defer func() { t.inComputedFilter = false }()
	schema, err := t.schema()
	if err != nil {
		return "", false, err
	}
	ft, ok := schema.Table(tb.Name)
	if !ok {
		return "", false, nil
	}
	names, err := t.nameValues(schema)
	if err != nil {
		return "", false, err
	}
	hoist, hoistErr := t.hoister()
	c := formula.CompileHoisted(n, schema, names, ft, tb.sql(), hoist)
	if *hoistErr != nil {
		return "", false, *hoistErr
	}
	e, err := inlineExpr(c.SQL, c.Args, false)
	if err != nil {
		return "", false, err
	}
	return e, true, nil
}

// rowLabelSQL est rowValueSQL en texte affiché (slt_label), pour comparer
// à une valeur choisie.
func (t *tx) rowLabelSQL(tb Table, en string) (string, bool, error) {
	e, ok, err := t.rowValueSQL(tb, en)
	if !ok || err != nil {
		return "", ok, err
	}
	return formula.LabelFunc + "(" + e + ")", true, nil
}

// prepareRowFormula analyse une formule par ligne saisie en français (le =
// est facultatif) : anglais canonique et colonnes citées (au moins une).
func (t *tx) prepareRowFormula(sheet int64, src string) (string, []string, error) {
	src = strings.TrimSpace(src)
	if src == "" || src == "=" {
		return "", nil, ErrFormula{"formule vide"}
	}
	if !strings.HasPrefix(src, "=") {
		src = "=" + src
	}
	n, err := t.prepareFormula(sheet, src)
	if err != nil {
		return "", nil, err
	}
	cols := formulaColumns(n)
	if len(cols) == 0 {
		return "", nil, ErrFormula{"la formule doit citer au moins une colonne entre crochets, par exemple =ANNEE([Date])"}
	}
	return formula.Format(n, formula.EN), cols, nil
}

// ChoicePreview est l'aperçu de « Choisir par › Une formule… » : premières
// valeurs (Values, au plus 10, Count en tout), tableaux de la feuille qui
// suivraient (Tables) et ceux qui ne suivraient pas, faute de colonnes
// (Others) ; Error : formule refusée (rien d'autre n'est rempli).
type ChoicePreview struct {
	Values []FilterValue `json:"values"`
	Count  int           `json:"count"`
	Tables []string      `json:"tables"`
	Others []string      `json:"others"`
	Error  string        `json:"error,omitempty"`
}

// choicePreviewValues : valeurs montrées par l'aperçu.
const choicePreviewValues = 10

// PreviewChoiceFormula calcule l'aperçu d'une catégorie calculée pour la
// feuille (src en français), sans rien changer.
func (m *Model) PreviewChoiceFormula(ctx context.Context, sheet int64, src string) (ChoicePreview, error) {
	out := ChoicePreview{Values: []FilterValue{}, Tables: []string{}, Others: []string{}}
	err := m.read(ctx, func(t *tx) error {
		en, cols, err := t.prepareRowFormula(sheet, src)
		if err != nil {
			out.Error = formulaError(err)
			return nil
		}
		tables, err := t.formulaTables(sheet)
		if err != nil {
			return err
		}
		at := map[string]int{}
		var all []FilterValue
		for _, tb := range tables {
			if miss := missingColumns(tb, cols); len(miss) > 0 {
				if tb.Sheet == sheet {
					out.Others = append(out.Others, tb.Name)
				}
				continue
			}
			expr, ok, err := t.rowLabelSQL(tb, en)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if tb.Sheet == sheet {
				out.Tables = append(out.Tables, tb.Name)
			}
			rs, err := t.query(`SELECT `+expr+` AS v, count(*) FROM `+tb.sql()+` GROUP BY v HAVING v <> '' LIMIT ?`, MaxFilterValues)
			if err != nil {
				return err
			}
			for rs.Next() {
				var v FilterValue
				if err := rs.Scan(&v.Text, &v.Count); err != nil {
					rs.Close()
					return err
				}
				if i, ok := at[v.Text]; ok {
					all[i].Count += v.Count
					continue
				}
				at[v.Text] = len(all)
				all = append(all, v)
			}
			rs.Close()
			if err := rs.Err(); err != nil {
				return err
			}
		}
		if len(out.Tables) == 0 && len(all) == 0 {
			out.Error = m.say("sheet.noTableWithColumnsHint", msg.Args{"columns": quoteList(cols)})
			return nil
		}
		sortChoiceValues(all)
		out.Count = len(all)
		out.Values = append(out.Values, all[:min(len(all), choicePreviewValues)]...)
		return nil
	})
	return out, err
}

// formulaTables : tableaux de la feuille, puis sources de ses TCD posées
// ailleurs (comme choiceSources).
func (t *tx) formulaTables(sheet int64) ([]Table, error) {
	tables, err := t.tables(sheet)
	if err != nil {
		return nil, err
	}
	srcs, err := t.pivotSources(sheet)
	if err != nil {
		return nil, err
	}
	return append(tables, srcs...), nil
}

// formulaError : message d'une formule refusée.
func formulaError(err error) string {
	var fe ErrFormula
	if errors.As(err, &fe) {
		return fe.Error()
	}
	return err.Error()
}

// quoteList : « Date », « Montant ».
func quoteList(cols []string) string {
	q := make([]string, len(cols))
	for i, c := range cols {
		q[i] = "« " + c + " »"
	}
	return strings.Join(q, ", ")
}

// SetSheetChoiceFormula fait de la feuille un onglet à choix par une
// formule sur les colonnes des lignes (« Choisir par › Une formule… ») :
// name nomme la catégorie (« Année »), src est la formule en français,
// value la valeur affichée d'abord ("" : toutes) ; une étape d'annulation.
func (m *Model) SetSheetChoiceFormula(ctx context.Context, sheet int64, name, src, value string) error {
	name = strings.TrimSpace(name)
	return m.Do(ctx, m.say("sheet.chooseByLabel", msg.Args{"column": name}), func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		if name == "" {
			return msg.Wrap(ErrSheet, "sheet.nameCategory", nil)
		}
		en, cols, err := t.prepareRowFormula(sheet, src)
		if err != nil {
			return err
		}
		tables, err := t.formulaTables(sheet)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(tables, func(tb Table) bool { return len(missingColumns(tb, cols)) == 0 }) {
			return msg.Wrap(ErrSheet, "sheet.noTableWithColumns", msg.Args{"columns": quoteList(cols)})
		}
		return t.writeChoice(sheet, &choiceRaw{Name: name, Formula: en, Value: value, All: value == ""})
	})
}

// keepChoiceRow garde visible une ligne ajoutée sous une catégorie
// calculée (choiceRaw.Keep), dans l'étape d'annulation de l'ajout.
func (t *tx) keepChoiceRow(sheet int64, rid int64) error {
	r, err := t.readChoice(sheet)
	if err != nil || r == nil || r.Formula == "" || r.All || len(r.Keep) >= maxChoiceKeep || slices.Contains(r.Keep, rid) {
		return err
	}
	old, err := encodeChoice(r)
	if err != nil {
		return err
	}
	r.Keep = append(r.Keep, rid)
	neu, err := encodeChoice(r)
	if err != nil {
		return err
	}
	q := `UPDATE _lattis_sheets SET choice = ? WHERE id = ?`
	return t.apply([]stmt{st(q, neu, sheet)}, []stmt{st(q, old, sheet)})
}

// setFormulaFollow : « Suivre » d'un tableau sous une catégorie calculée.
// Rien à ajouter : le tableau la suit s'il a les colonnes de la formule.
func (t *tx) setFormulaFollow(tb Table, ch *SheetChoice, follow bool, what string) (string, error) {
	if follow {
		n, err := formula.Parse(ch.en, formula.EN)
		if err != nil {
			return "", err
		}
		if miss := missingColumns(tb, formulaColumns(n)); len(miss) > 0 {
			return "", msg.Wrap(ErrTable, "table.missingChoiceColumns", msg.Args{"table": tb.Name, "columns": quoteList(miss), "formula": ch.Formula})
		}
		if err := t.setFollowFlag(tb, nil); err != nil {
			return "", err
		}
		return t.say("follow.follows", msg.Args{"table": tb.Name, "what": what}), nil
	}
	if err := t.setFollowFlag(tb, 0); err != nil {
		return "", err
	}
	return t.say("follow.stops", msg.Args{"table": tb.Name, "what": what}), nil
}

// frenchNumber lit un nombre écrit comme FormatNumber (2025, 1,5, -3).
func frenchNumber(s string) (float64, bool) {
	if s == "" || strings.TrimSpace(s) != s || strings.Contains(s, ".") {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	return f, err == nil
}

// sortChoiceValues range les valeurs d'une catégorie calculée dans l'ordre
// naturel : nombres croissants, puis textes (jours et mois dans l'ordre du
// calendrier quand ils en sont tous, sinon alphabétique avec les nombres
// qu'ils contiennent dans l'ordre : Lot 2 avant Lot 10).
func sortChoiceValues(vals []FilterValue) {
	rank := func(string) int { return -1 }
	for _, l := range BuiltinLists {
		idx := map[string]int{}
		for i, v := range l.Values {
			idx[Fold(v)] = i
		}
		all := len(vals) > 0
		for _, v := range vals {
			if _, ok := frenchNumber(v.Text); ok {
				continue
			}
			if _, ok := idx[Fold(v.Text)]; !ok {
				all = false
				break
			}
		}
		if all {
			rank = func(s string) int {
				if i, ok := idx[Fold(s)]; ok {
					return i
				}
				return -1
			}
			break
		}
	}
	slices.SortStableFunc(vals, func(a, b FilterValue) int {
		fa, na := frenchNumber(a.Text)
		fb, nb := frenchNumber(b.Text)
		switch {
		case na && nb:
			if fa < fb {
				return -1
			} else if fa > fb {
				return 1
			}
			return 0
		case na:
			return -1
		case nb:
			return 1
		}
		if ra, rb := rank(a.Text), rank(b.Text); ra >= 0 && rb >= 0 {
			return ra - rb
		}
		return naturalCompare(a.Text, b.Text)
	})
}

// naturalCompare compare deux textes sans tenir compte de la casse, les
// suites de chiffres comme des nombres (Lot 2 < Lot 10).
func naturalCompare(a, b string) int {
	a, b = Fold(a), Fold(b)
	for a != "" && b != "" {
		da, db := digits(a), digits(b)
		if da > 0 && db > 0 {
			x, y := strings.TrimLeft(a[:da], "0"), strings.TrimLeft(b[:db], "0")
			if len(x) != len(y) {
				return len(x) - len(y)
			}
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
			a, b = a[da:], b[db:]
			continue
		}
		ra, sa := firstRune(a)
		rb, sb := firstRune(b)
		if ra != rb {
			return strings.Compare(a[:sa], b[:sb])
		}
		a, b = a[sa:], b[sb:]
	}
	return len(a) - len(b)
}

func digits(s string) int {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

func firstRune(s string) (rune, int) {
	for i, r := range s {
		if i == 0 {
			n := len(string(r))
			return r, n
		}
	}
	return 0, 0
}

// rewriteChoiceFormulas : une colonne renommée dans un tableau d'une feuille
// à catégorie calculée l'est aussi dans la formule de la catégorie (le
// tableau continue de la suivre).
func (t *tx) rewriteChoiceFormulas(rc refChange) error {
	rs, err := t.query(`SELECT s.id, s.choice, coalesce((SELECT group_concat(name, char(0)) FROM _lattis_tables WHERE sheet = s.id), '') FROM _lattis_sheets s WHERE s.choice LIKE '%"formula"%'`)
	if err != nil {
		return err
	}
	var ups []textUpdate
	for rs.Next() {
		var id int64
		var raw, names string
		if err := rs.Scan(&id, &raw, &names); err != nil {
			rs.Close()
			return err
		}
		var r choiceRaw
		if json.Unmarshal([]byte(raw), &r) != nil || r.Formula == "" {
			continue
		}
		for _, name := range strings.Split(names, "\x00") {
			if neu, ok := formula.RenameRefs(r.Formula, formula.EN, rc.Renames, name); ok && name != "" {
				r.Formula = neu
				if s, err := encodeChoice(&r); err == nil {
					ups = append(ups, textUpdate{[]any{id}, raw, s})
				}
				break
			}
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	return t.applyUpdates(`UPDATE _lattis_sheets SET choice = ? WHERE id = ?`, ups)
}

// encodeChoice écrit la colonne choice.
func encodeChoice(r *choiceRaw) (string, error) {
	b, err := json.Marshal(r)
	return string(b), err
}
