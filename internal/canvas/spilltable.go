package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Résultat déversé converti en tableau (clic droit sur un déversement) :
//
//   - « Convertir en tableau (valeurs) » remplace la formule par un tableau
//     ordinaire qui garde les valeurs du moment ;
//   - « Convertir en tableau lié » la remplace par un tableau chargé par une
//     requête du classeur de source « formule » (docs/decisions/0008) : la
//     formule est gardée dans la requête, le tableau se recalcule quand ses
//     sources changent (syncFormulaTables, dans la même étape d'annulation
//     que la modification) et à l'actualisation (F5) ; ses colonnes chargées
//     sont en lecture seule, et des colonnes saisies peuvent s'ajouter à sa
//     droite, leurs valeurs suivant les lignes par une clé.
//
// En-têtes : si chaque colonne du résultat a, juste au-dessus de l'ancre, un
// texte (un titre comme « Nids » au-dessus de =UNIQUE(…)), ces textes
// deviennent les en-têtes et le tableau commence sur leur ligne ; sinon les
// en-têtes sont les noms naturels des colonnes (Suividesnids[NID suivi] :
// « NID suivi » ; « Valeur » pour SEQUENCE, Colonne1… à défaut), posés sur la
// ligne vide au-dessus de l'ancre s'il y en a une (les valeurs ne bougent
// pas), sinon sur la ligne de l'ancre (les valeurs descendent d'une ligne).

// SourceFormula : genre de source d'une requête liée à une formule
// (internal/query.SourceFormula).
const SourceFormula = "formula"

// ErrSpill signale une conversion impossible d'un résultat déversé.
var ErrSpill = errors.New("résultat déversé")

// formulaSource est la source JSON d'une requête « formule » (voir
// internal/query.Source).
type formulaSource struct {
	Kind    string `json:"kind"`
	Formula string `json:"formula"`
	// Sheet : feuille où se lisent les adresses Cell_A1 sans feuille (celle
	// de la formule d'origine).
	Sheet int64 `json:"formulaSheet"`
	// Columns : en-têtes du tableau, colonne par colonne (vides ou absents :
	// noms naturels).
	Columns []string `json:"columns,omitempty"`
}

// FormulaResult est le résultat d'une formule de tableau dynamique, prêt à
// charger : en-têtes (uniques), types et valeurs au type de stockage.
type FormulaResult struct {
	Headers []string
	Types   []grid.Type
	Rows    [][]any
}

// ImportTable prépare le chargement du résultat.
func (r FormulaResult) ImportTable() ImportTable {
	rows := r.Rows
	return ImportTable{
		Headers: slices.Clone(r.Headers),
		Types:   slices.Clone(r.Types),
		Count:   len(rows),
		Rows: func(yield func([]any) error) error {
			for _, row := range rows {
				if err := yield(row); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

var isoDay = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// valueType donne le type le plus étroit d'une valeur calculée.
func valueType(v formula.Value) grid.Type {
	switch v.Kind {
	case formula.Number:
		if v.Num == math.Trunc(v.Num) && math.Abs(v.Num) < 1<<53 {
			return grid.Integer
		}
		return grid.Decimal
	case formula.Bool:
		return grid.Boolean
	case formula.Date:
		if v.Num == math.Trunc(v.Num) {
			return grid.Date
		}
		return grid.Text
	case formula.Text:
		if isoDay.MatchString(v.Str) {
			return grid.Date
		}
		return grid.Text
	case formula.Error:
		return grid.Text
	}
	return ""
}

// storedValue écrit une valeur calculée au stockage d'une colonne de type typ.
func storedValue(v formula.Value, typ grid.Type) any {
	if v.Kind == formula.Empty {
		return nil
	}
	num := v.Num
	if v.Kind == formula.Bool {
		num = 0
		if v.Bool {
			num = 1
		}
	}
	switch typ {
	case grid.Integer, grid.Boolean:
		return int64(math.Round(num))
	case grid.Decimal:
		return num
	case grid.Date:
		if v.Kind == formula.Date {
			return formula.ISODate(v.Num)
		}
		return v.Str
	}
	return v.String()
}

// formulaResult met en forme le résultat d'un tableau dynamique : heads, ses
// en-têtes naturels ; names, les en-têtes choisis (vides : naturels).
func formulaResult(heads []string, rows [][]formula.Value, names []string) FormulaResult {
	k := len(heads)
	hs := make([]string, k)
	for j := range hs {
		h := ""
		if j < len(names) {
			h = strings.TrimSpace(names[j])
		}
		if h == "" {
			h = strings.TrimSpace(heads[j])
		}
		if h == "" {
			h = "Colonne" + strconv.Itoa(j+1)
		}
		hs[j] = h
	}
	res := FormulaResult{Headers: uniqueColumnNames(hs), Types: make([]grid.Type, k)}
	for _, row := range rows {
		for j := 0; j < k && j < len(row); j++ {
			res.Types[j] = Join(res.Types[j], valueType(row[j]))
		}
	}
	for j, typ := range res.Types {
		if typ == "" {
			res.Types[j] = grid.Text
		}
	}
	for _, row := range rows {
		out := make([]any, k)
		for j := 0; j < k && j < len(row); j++ {
			out[j] = storedValue(row[j], res.Types[j])
		}
		res.Rows = append(res.Rows, out)
	}
	return res
}

var tableSQLName = regexp.MustCompile(`\bt_\d+\b`)

// spillFormula est une formule de tableau dynamique compilée pour la
// feuille sheet : deps, les tableaux (t_N) qu'elle lit ; cells, elle lit
// des cellules (adresses ou noms).
type spillFormula struct {
	sp    formula.Spill
	deps  []string
	cells bool
}

// compileSpillFormula compile une formule de cellule libre qui se déverse.
func (t *tx) compileSpillFormula(sheet int64, raw string) (spillFormula, error) {
	notSpill := fmt.Errorf("%w : la formule %s ne rend pas un tableau de valeurs", ErrSpill, raw)
	n, err := formula.Parse(raw, formula.FR)
	if err != nil {
		return spillFormula{}, fmt.Errorf("%w : %v", ErrSpill, err)
	}
	_, column := n.(formula.Ref)
	if !column && !formula.IsSpill(n) {
		return spillFormula{}, notSpill
	}
	base, err := t.schema()
	if err != nil {
		return spillFormula{}, err
	}
	schema, err := t.schemaFor(sheet)
	if err != nil {
		return spillFormula{}, err
	}
	if column && !formula.IsFreeSpill(n, schema) {
		return spillFormula{}, notSpill
	}
	if n, _, err = formula.Convert(n, schema, noRanges{}, raw); err != nil {
		return spillFormula{}, fmt.Errorf("%w : %v", ErrSpill, err)
	}
	n = formula.QualifyCells(n, sheet)
	cells := strings.Contains(strings.ToLower(raw), "cell")
	if !cells {
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_names`).Scan(&k); err != nil {
			return spillFormula{}, err
		}
		cells = k > 0
	}
	names, err := t.nameValuesWith(base, cells)
	if err != nil {
		return spillFormula{}, err
	}
	hoist, _ := t.hoister()
	sp, e := formula.CompileSpillHoisted(n, schema, names, hoist)
	if e.IsErr() {
		return spillFormula{}, fmt.Errorf("%w : la formule %s rend %s", ErrSpill, raw, e)
	}
	f := spillFormula{sp: sp, cells: cells}
	for _, d := range tableSQLName.FindAllString(sp.SQL, -1) {
		if !slices.Contains(f.deps, d) {
			f.deps = append(f.deps, d)
		}
	}
	return f, nil
}

// run calcule la formule : en-têtes naturels et lignes.
func (f spillFormula) run(t *tx, raw string) ([]string, [][]formula.Value, error) {
	heads, rows, e := t.runSpill(f.sp)
	if e.IsErr() {
		return nil, nil, fmt.Errorf("%w : la formule %s rend %s", ErrSpill, raw, e)
	}
	if len(heads) == 0 {
		return nil, nil, fmt.Errorf("%w : la formule %s ne rend aucune colonne", ErrSpill, raw)
	}
	return heads, rows, nil
}

// evalFormulaSource calcule le résultat d'une requête « formule ».
func (t *tx) evalFormulaSource(src formulaSource) (FormulaResult, spillFormula, error) {
	raw := strings.TrimSpace(src.Formula)
	if !isFormula(raw) {
		return FormulaResult{}, spillFormula{}, fmt.Errorf("%w : formule vide", ErrSpill)
	}
	var k int
	if err := t.row(`SELECT count(*) FROM _lattis_sheets WHERE id = ?`, src.Sheet).Scan(&k); err != nil {
		return FormulaResult{}, spillFormula{}, err
	}
	if k == 0 {
		return FormulaResult{}, spillFormula{}, fmt.Errorf("%w : la feuille de la formule %s a été supprimée", ErrSpill, raw)
	}
	f, err := t.compileSpillFormula(src.Sheet, raw)
	if err != nil {
		return FormulaResult{}, f, err
	}
	heads, rows, err := f.run(t, raw)
	if err != nil {
		return FormulaResult{}, f, err
	}
	return formulaResult(heads, rows, src.Columns), f, nil
}

// FormulaTable calcule le résultat d'une formule de tableau dynamique
// (source « formule » d'une requête, internal/query) : sheet est la feuille
// des adresses Cell_A1, columns les en-têtes choisis (vides : naturels).
func (m *Model) FormulaTable(ctx context.Context, sheet int64, raw string, columns []string) (FormulaResult, error) {
	var res FormulaResult
	err := m.read(ctx, func(t *tx) error {
		var err error
		res, _, err = t.evalFormulaSource(formulaSource{Kind: SourceFormula, Formula: raw, Sheet: sheet, Columns: columns})
		return err
	})
	return res, err
}

// spillPlace dit où poser le tableau d'un déversement de k colonnes ancré en
// (row, col) : ligne de l'en-tête et, s'il y en a, les textes au-dessus de
// chaque colonne qui deviennent ses en-têtes.
func (t *tx) spillPlace(sheet int64, row, col, k int) (header int, labels []string, err error) {
	if row == 0 {
		return row, nil, nil
	}
	cells, err := t.cells(sheet)
	if err != nil {
		return 0, nil, err
	}
	tables, err := t.tables(sheet)
	if err != nil {
		return 0, nil, err
	}
	for _, tb := range tables {
		if tb.Top() <= row-1 && tb.Bottom() >= row-1 && tb.Col <= col+k-1 && tb.Right() >= col {
			return row, nil, nil // un tableau juste au-dessus
		}
	}
	above := map[int]string{}
	for _, c := range cells {
		if c.Row == row-1 && c.Col >= col && c.Col < col+k {
			above[c.Col-col] = c.Raw
		}
	}
	if len(above) == 0 {
		return row - 1, nil, nil // ligne vide : les valeurs restent en place
	}
	if len(above) < k {
		return row, nil, nil // un titre au-dessus d'une partie seulement : il reste
	}
	for j := range k {
		s := strings.TrimSpace(above[j])
		if s == "" || isFormula(s) || Infer(s) != grid.Text {
			return row, nil, nil
		}
		labels = append(labels, s)
	}
	return row - 1, labels, nil
}

// freeQueryName donne un nom de requête libre tiré de base.
func (t *tx) freeQueryName(base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "Formule"
	}
	cand := base
	for n := 2; ; n++ {
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_queries WHERE name = ? COLLATE NOCASE`, cand).Scan(&k); err != nil {
			return "", err
		}
		if k == 0 {
			return cand, nil
		}
		cand = base + strconv.Itoa(n)
	}
}

// ConvertSpill remplace le résultat déversé de la formule de la cellule
// (row, col) par un tableau, en une seule étape d'annulation : lié (requête
// « formule » recalculée quand ses sources changent) ou de valeurs. Rend le
// nom du tableau.
func (m *Model) ConvertSpill(ctx context.Context, sheet int64, row, col int, linked bool) (string, error) {
	label := "Convertir en tableau (valeurs)"
	if linked {
		label = "Convertir en tableau lié"
	}
	var name string
	err := m.Do(ctx, label, func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		var raw string
		switch err := t.row(`SELECT raw FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col).Scan(&raw); {
		case errors.Is(err, sql.ErrNoRows):
			raw = ""
		case err != nil:
			return err
		}
		raw = strings.TrimSpace(raw)
		if !isFormula(raw) {
			return fmt.Errorf("%w : la cellule ne contient pas de formule", ErrSpill)
		}
		f, err := t.compileSpillFormula(sheet, raw)
		if err != nil {
			return err
		}
		heads, rows, err := f.run(t, raw)
		if err != nil {
			return err
		}
		k := len(heads)
		// Un déversement bloqué n'a rien affiché : rien à convertir.
		cells, err := t.cells(sheet)
		if err != nil {
			return err
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		shown := make([][]string, len(rows))
		for i := range shown {
			shown[i] = make([]string, k)
		}
		if spillBlocked(row, col, shown, cells, tables) {
			return fmt.Errorf("%w : la formule ne peut pas se déverser (%s) ; libérez la place d'abord", ErrSpill, ErrSpillText)
		}
		header, labels, err := t.spillPlace(sheet, row, col, k)
		if err != nil {
			return err
		}
		if err := t.setFreeCell(sheet, row, col, ""); err != nil {
			return err
		}
		for j := range labels {
			if err := t.setFreeCell(sheet, row-1, col+j, ""); err != nil {
				return err
			}
		}
		res := formulaResult(heads, rows, labels)
		it := res.ImportTable()
		it.Row, it.Col = header, col
		base := ""
		switch {
		case len(labels) > 0:
			base = labels[0]
		case k == 1:
			base = res.Headers[0]
		}
		var tb Table
		if linked {
			qname, err := t.freeQueryName(base)
			if err != nil {
				return err
			}
			src, err := json.Marshal(formulaSource{Kind: SourceFormula, Formula: raw, Sheet: sheet, Columns: res.Headers})
			if err != nil {
				return err
			}
			var id int64
			if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_queries`).Scan(&id); err != nil {
				return err
			}
			if err := t.apply(
				[]stmt{st(`INSERT INTO _lattis_queries (id, name, source, steps) VALUES (?, ?, ?, '[]')`, id, qname, string(src))},
				[]stmt{st(`DELETE FROM _lattis_queries WHERE id = ?`, id)},
			); err != nil {
				return err
			}
			it.At = &QueryAnchor{Sheet: sheet, Row: header, Col: col}
			var ql QueryLoad
			if err := t.loadQuery(id, qname, it, &ql); err != nil {
				return err
			}
			if tb, err = t.tableByName(ql.Table); err != nil {
				return err
			}
			// Une seule colonne aux valeurs uniques (=UNIQUE(…)) : elle sert
			// d'office de clé aux colonnes saisies à droite.
			if k == 1 {
				hi, err := t.hand(queryOwner(id))
				if err != nil {
					return err
				}
				why, err := t.keyWhy(tb, []int{0})
				if err != nil {
					return err
				}
				if why == "" && !hi.keyed {
					if err := t.setHandKey(&hi, tb.Cols[0].Name); err != nil {
						return err
					}
				}
			}
		} else {
			if base == "" {
				it.Name, err = t.freshName()
			} else {
				it.Name, err = t.freeTableName(base)
			}
			if err != nil {
				return err
			}
			if err := t.guardWorkbook(); err != nil {
				return err
			}
			if err := t.purgeImports(); err != nil {
				return err
			}
			if tb, err = t.importTable(sheet, &it); err != nil {
				return err
			}
			if err := t.settle(sheet, tb.ID); err != nil {
				return err
			}
		}
		// En-tête sur la ligne de l'ancre : les valeurs descendent d'une
		// ligne, ce qui était collé dessous s'écarte.
		if header == row && len(rows) > 0 {
			if err := t.ensureGapBelow(sheet, tb.ID, row+len(rows)-1, tb.Bottom(), col, col+k-1); err != nil {
				return err
			}
		}
		name = tb.Name
		return nil
	})
	return name, err
}

// formulaLink est une requête « formule » chargée dans un tableau.
type formulaLink struct {
	id     int64
	name   string
	src    formulaSource
	target int64
}

// formulaLinks liste les requêtes « formule » chargées, sans étapes (une
// requête à étapes, modifiée dans l'éditeur de requêtes, s'actualise par
// F5).
func (t *tx) formulaLinks() ([]formulaLink, error) {
	rs, err := t.query(`SELECT id, name, source, steps, target FROM _lattis_queries
		WHERE target IS NOT NULL AND json_valid(source) AND json_extract(source, '$.kind') = ? ORDER BY id`, SourceFormula)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []formulaLink
	for rs.Next() {
		var l formulaLink
		var src, steps string
		if err := rs.Scan(&l.id, &l.name, &src, &steps, &l.target); err != nil {
			return nil, err
		}
		if s := strings.TrimSpace(steps); s != "" && s != "[]" {
			continue
		}
		if json.Unmarshal([]byte(src), &l.src) != nil {
			continue
		}
		out = append(out, l)
	}
	return out, rs.Err()
}

// syncFormulaTables recalcule, à la fin d'une action, les tableaux liés à
// une formule dont les sources ont été écrites par l'action : le nouveau
// résultat est chargé dans la même étape d'annulation (un Ctrl+Z défait
// l'action et le recalcul ensemble). Un tableau dont le résultat ne change
// pas n'est pas réécrit. Ce qu'un recalcul écrit peut en relancer un autre
// (un tableau lié lu par une autre formule), quelques tours au plus.
func (t *tx) syncFormulaTables() error {
	if t.ro || t.syncingFormulas || t.m == nil || t.m.db == nil {
		return nil
	}
	links, err := t.formulaLinks()
	if err != nil || len(links) == 0 {
		return err
	}
	t.syncingFormulas = true
	defer func() { t.syncingFormulas = false }()
	var written map[string]bool // tableaux réécrits au tour précédent
	for pass := range 4 {
		next := map[string]bool{}
		for _, l := range links {
			tb, err := t.table(l.target)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if t.guard(tb.Sheet) != nil || t.lockTable(tb.ID) != nil || t.guardWorkbook() != nil {
				continue // feuille protégée, tableau publié figé : F5 plus tard
			}
			raw := strings.TrimSpace(l.src.Formula)
			f, err := t.compileSpillFormula(l.src.Sheet, raw)
			if err != nil {
				continue // formule en erreur : le tableau garde ses valeurs
			}
			deps := slices.DeleteFunc(slices.Clone(f.deps), func(d string) bool { return d == tb.sql() })
			if pass == 0 {
				if !t.m.db.Pending(f.cells, deps...) {
					continue
				}
			} else if !slices.ContainsFunc(deps, func(d string) bool { return written[d] }) {
				continue
			}
			heads, rows, err := f.run(t, raw)
			if err != nil {
				continue
			}
			res := formulaResult(heads, rows, l.src.Columns)
			same, err := t.sameLoaded(l.id, tb, res)
			if err != nil {
				return err
			}
			if same {
				continue
			}
			var ql QueryLoad
			if err := t.loadQuery(l.id, l.name, res.ImportTable(), &ql); err != nil {
				return err
			}
			if neu, err := t.tableByName(ql.Table); err == nil {
				next[neu.sql()] = true
			}
		}
		if len(next) == 0 {
			return nil
		}
		written = next
	}
	return nil
}

// sameLoaded dit si les colonnes chargées du tableau tb (requête id) ont
// déjà les en-têtes, les types et les lignes de res (dans n'importe quel
// ordre : un tri de l'utilisateur est gardé).
func (t *tx) sameLoaded(id int64, tb Table, res FormulaResult) (bool, error) {
	hi, err := t.hand(queryOwner(id))
	if err != nil {
		return false, err
	}
	loaded := hi.loadedCols(tb)
	if len(loaded) != len(res.Headers) {
		return false, nil
	}
	sel := make([]string, len(loaded))
	for j, i := range loaded {
		c := tb.Cols[i]
		if c.Name != res.Headers[j] || c.Type != res.Types[j] {
			return false, nil
		}
		sel[j] = colSQL(c.ID)
	}
	var n int
	if err := t.row(`SELECT count(*) FROM ` + tb.sql()).Scan(&n); err != nil {
		return false, err
	}
	if n != len(res.Rows) {
		return false, nil
	}
	key := func(vals []any) string {
		parts := make([]string, len(vals))
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			parts[i] = fmt.Sprintf("%T:%v", v, v)
		}
		return strings.Join(parts, "\x1f")
	}
	want := make([]string, len(res.Rows))
	for i, r := range res.Rows {
		want[i] = key(r)
	}
	rs, err := t.query(`SELECT ` + strings.Join(sel, ", ") + ` FROM ` + tb.sql())
	if err != nil {
		return false, err
	}
	defer rs.Close()
	var got []string
	for rs.Next() {
		vals := make([]any, len(sel))
		ptrs := make([]any, len(sel))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return false, err
		}
		got = append(got, key(vals))
	}
	if err := rs.Err(); err != nil {
		return false, err
	}
	slices.Sort(want)
	slices.Sort(got)
	return slices.Equal(want, got), nil
}
