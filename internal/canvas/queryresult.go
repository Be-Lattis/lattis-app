package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/store"
)

// MaxResultRows borne les lignes d'un résultat SQL envoyé vers une feuille.
const MaxResultRows = 1_000_000

// QueryToSheet exécute une requête en lecture seule (déjà traduite en noms
// internes) et pose son résultat dans un nouveau tableau, sur une nouvelle
// feuille nommée name (vide : « Requête1 », « Requête2 »…). headers donne les
// en-têtes affichés (noms de colonnes de la requête si leur nombre diffère).
// Les types des colonnes viennent des valeurs (entier, décimal, date, texte).
// Une seule étape d'annulation.
func (m *Model) QueryToSheet(ctx context.Context, q, name string, headers []string) (ImportResult, error) {
	if strings.TrimSpace(q) == "" {
		return ImportResult{}, grid.ErrEmptyQuery
	}
	conn, err := store.UserConn(ctx, m.db.Read())
	if err != nil {
		return ImportResult{}, err
	}
	defer store.ReleaseUserConn(conn)
	rs, err := conn.QueryContext(ctx, q)
	if err != nil {
		return ImportResult{}, err
	}
	cols, err := rs.Columns()
	if err != nil {
		rs.Close()
		return ImportResult{}, err
	}
	if len(cols) == 0 {
		rs.Close()
		return ImportResult{}, msg.New("query.noColumns", nil)
	}
	var rows [][]any
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rs.Next() {
		if len(rows) == MaxResultRows {
			rs.Close()
			return ImportResult{}, msg.New("query.tooManyRows", msg.Args{"max": MaxResultRows})
		}
		if err := rs.Scan(ptrs...); err != nil {
			rs.Close()
			return ImportResult{}, err
		}
		row := make([]any, len(cols))
		copy(row, vals)
		rows = append(rows, row)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return ImportResult{}, err
	}
	// Colonnes techniques (_pos…) : pas sur la feuille. Les en-têtes donnés
	// sont ceux de toutes les colonnes, ou des seules colonnes visibles
	// (aperçu de l'onglet SQL).
	idx := grid.Visible(cols)
	if len(idx) == 0 {
		return ImportResult{}, msg.New("query.onlyTechnicalColumns", nil)
	}
	if len(headers) == len(cols) {
		var hs []string
		for _, i := range idx {
			hs = append(hs, headers[i])
		}
		headers = hs
	}
	if len(headers) != len(idx) {
		headers = nil
		for _, i := range idx {
			headers = append(headers, cols[i])
		}
	}
	if len(idx) < len(cols) {
		for j, row := range rows {
			rows[j] = grid.Pick(row, idx)
		}
	}
	if name, err = m.resultName(ctx, name); err != nil {
		return ImportResult{}, err
	}
	taken, err := m.lowerTableNames(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	sheet := ImportSheet{Name: name, Tables: []ImportTable{resultTable(freeTableName(name, taken), headers, rows)}}
	return m.Import(ctx, "Résultat vers une feuille", []ImportSheet{sheet})
}

// resultName donne le nom de la feuille : celui demandé, sinon le premier
// « RequêteN » libre parmi les feuilles et les tableaux.
func (m *Model) resultName(ctx context.Context, name string) (string, error) {
	if name = strings.TrimSpace(name); name != "" {
		return name, nil
	}
	tables, err := m.lowerTableNames(ctx)
	if err != nil {
		return "", err
	}
	l, err := m.Layout(ctx, 0)
	if err != nil {
		return "", err
	}
	sheets := map[string]bool{}
	for _, s := range l.Sheets {
		sheets[strings.ToLower(s.Name)] = true
	}
	for n := 1; ; n++ {
		cand := fmt.Sprintf("Requête%d", n)
		if !sheets[strings.ToLower(cand)] && !tables[strings.ToLower(cand)] {
			return cand, nil
		}
	}
}

// lowerTableNames donne les noms de tableaux pris, en minuscules.
func (m *Model) lowerTableNames(ctx context.Context) (map[string]bool, error) {
	names, err := m.TableNames(ctx)
	out := map[string]bool{}
	for n := range names {
		out[strings.ToLower(n)] = true
	}
	return out, err
}

// freeTableName rend un nom de tableau valable et libre (taken : noms en
// minuscules) à partir de name.
func freeTableName(name string, taken map[string]bool) string {
	base := identifier(name)
	if base == "" {
		base = "Requête"
	} else if A1Like(base) {
		base += "_" // pas une adresse de cellule
	}
	isTaken := func(n string) bool { return taken[strings.ToLower(n)] }
	if !isTaken(base) {
		return base
	}
	for n := 2; ; n++ {
		cand := fmt.Sprintf("%s_%d", base, n)
		if !isTaken(cand) {
			return cand
		}
	}
}

// resultValue lit une valeur rendue par SQLite : nombre, texte, date ISO, ou
// valeur de formule (erreur, booléen, date) encodée par les fonctions slt_*.
func resultValue(v any) (grid.Type, any) {
	switch v := v.(type) {
	case nil:
		return "", nil
	case int64:
		return grid.Integer, v
	case float64:
		return grid.Decimal, v
	case string:
		if isoDate.MatchString(v) {
			if _, ok := formula.ParseDate(v); ok {
				return grid.Date, v
			}
		}
		return grid.Text, v
	case []byte:
		f := formula.FromResult(v)
		switch f.Kind {
		case formula.Empty:
			return "", nil
		case formula.Number:
			return grid.Decimal, f.Num
		case formula.Bool:
			return grid.Boolean, f.Bool
		case formula.Date:
			return grid.Date, formula.ISODate(f.Num)
		}
		return grid.Text, f.String()
	}
	return grid.Text, fmt.Sprint(v)
}

// resultTable construit le tableau d'un résultat : types joints colonne par
// colonne, valeurs converties au type de stockage.
func resultTable(name string, headers []string, rows [][]any) ImportTable {
	k := len(headers)
	types := make([]grid.Type, k)
	for _, r := range rows {
		for j := range k {
			t, _ := resultValue(r[j])
			types[j] = Join(types[j], t)
		}
	}
	for j := range types {
		if types[j] == "" {
			types[j] = grid.Text
		}
	}
	return ImportTable{
		Name: name, Headers: uniqueHeaders(headers), Types: types, Count: len(rows),
		Rows: func(yield func([]any) error) error {
			out := make([]any, k)
			for _, r := range rows {
				for j := range k {
					out[j] = storeAs(r[j], types[j])
				}
				if err := yield(out); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// storeAs convertit une valeur au type de stockage de sa colonne.
func storeAs(v any, to grid.Type) any {
	t, x := resultValue(v)
	if t == "" {
		return nil
	}
	switch to {
	case grid.Integer:
		switch x := x.(type) {
		case int64:
			return x
		case bool:
			return boolInt(x)
		}
	case grid.Decimal:
		switch x := x.(type) {
		case int64:
			return float64(x)
		case float64:
			return x
		case bool:
			return float64(boolInt(x))
		}
	case grid.Boolean:
		if b, ok := x.(bool); ok {
			return boolInt(b)
		}
	case grid.Date:
		if s, ok := x.(string); ok {
			return s
		}
	}
	// Texte : la valeur telle que la grille l'affiche.
	switch x := x.(type) {
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return formula.FormatNumber(x)
	case bool:
		return formula.Boolean(x).String()
	case string:
		if t == grid.Date {
			if f, ok := formula.ParseDate(x); ok {
				return formula.FormatDate(f)
			}
		}
		return x
	}
	return fmt.Sprint(x)
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// uniqueHeaders complète et rend uniques les en-têtes (« Nom », « Nom 2 »).
func uniqueHeaders(hs []string) []string {
	out := make([]string, len(hs))
	seen := map[string]bool{}
	for i, h := range hs {
		h = strings.TrimSpace(h)
		if h == "" {
			h = fmt.Sprintf("Colonne %d", i+1)
		}
		cand := h
		for n := 2; seen[strings.ToLower(cand)]; n++ {
			cand = fmt.Sprintf("%s %d", h, n)
		}
		seen[strings.ToLower(cand)] = true
		out[i] = cand
	}
	return out
}

// citedTable repère les noms internes de tableaux (t_12) cités dans une
// requête.
var citedTable = regexp.MustCompile(`(?i)\bt_(\d+)\b`)

// SQLHeaders donne les en-têtes affichés des colonnes rendues par une requête
// écrite en noms internes (requête traduite par l'onglet SQL, ou plus
// ancienne avec SELECT *) : une colonne c_N d'un seul des tableaux cités
// reprend son nom affiché, comme displayHeaders (frontend/src/lib/sql). Les
// autres noms restent tels quels.
func (m *Model) SQLHeaders(ctx context.Context, q string, names []string) ([]string, error) {
	out := slices.Clone(names)
	cited := map[int64]bool{}
	for _, g := range citedTable.FindAllStringSubmatch(q, -1) {
		if id, err := strconv.ParseInt(g[1], 10, 64); err == nil {
			cited[id] = true
		}
	}
	if len(cited) == 0 {
		return out, nil
	}
	shown := map[string][]string{} // c_N en minuscules → noms affichés
	err := m.read(ctx, func(t *tx) error {
		for id := range cited {
			tb, err := t.table(id)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			for _, c := range tb.Cols {
				k := strings.ToLower(colSQL(c.ID))
				shown[k] = append(shown[k], c.Name)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for i, n := range out {
		if hits := shown[strings.ToLower(n)]; len(hits) == 1 || (len(hits) > 1 && allSame(hits)) {
			out[i] = hits[0]
		}
	}
	return out, nil
}

func allSame(s []string) bool {
	for _, x := range s[1:] {
		if x != s[0] {
			return false
		}
	}
	return true
}

// ReadSQL exécute une requête de l'onglet SQL en lecture seule, telle qu'une
// requête liée (Référencer) la relit à chaque actualisation : adresses de
// cellules remplacées par leurs valeurs (home : feuille des adresses sans
// feuille, "" : la première), tableaux croisés cités définis. cols reçoit les
// noms des colonnes, row chaque ligne.
func (m *Model) ReadSQL(ctx context.Context, q, home string, cols func([]string) error, row func([]any) error) error {
	if strings.TrimSpace(q) == "" {
		return grid.ErrEmptyQuery
	}
	var sheet int64
	if err := m.read(ctx, func(t *tx) error {
		return t.row(`SELECT id FROM _lattis_sheets WHERE ? = '' OR lower(name) = lower(?) ORDER BY ? = '' AND pos, id LIMIT 1`, home, home, home).Scan(&sheet)
	}); err != nil {
		return msg.New("sheet.notFoundNamed", msg.Args{"sheet": home})
	}
	q, err := m.CellSQL(ctx, q, sheet)
	if err != nil {
		return err
	}
	if q, err = m.ReadCTE(ctx, q); err != nil {
		return err
	}
	// Requête rangée dans le classeur : sous l'autorisateur (store.UserConn).
	conn, err := store.UserConn(ctx, m.db.Read())
	if err != nil {
		return err
	}
	defer store.ReleaseUserConn(conn)
	rs, err := conn.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rs.Close()
	names, err := rs.Columns()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return msg.New("query.noColumns", nil)
	}
	if err := cols(names); err != nil {
		return err
	}
	vals := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return err
		}
		if err := row(vals); err != nil {
			return err
		}
	}
	return rs.Err()
}
