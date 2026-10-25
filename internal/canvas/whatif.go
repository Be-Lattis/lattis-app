package canvas

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/optim"
)

// Analyse de scénarios (Données › Analyse de scénarios) : valeur cible,
// table de données, gestionnaire de scénarios (scenarios.go) et solveur
// (solver.go). Les calculs se font sur les formules du classeur : une
// cellule d'entrée (constante d'une cellule libre, ou cellule saisie d'un
// tableau) reçoit une valeur d'essai dans une transaction qui est ensuite
// annulée (trial), et la cellule de résultat est relue comme la grille la
// calcule (formule libre, colonne calculée). Rien ne change dans le
// classeur avant « Conserver » : une seule étape d'annulation.

// maxWhatCells borne les plages des cellules variables et des résultats.
const maxWhatCells = 200

// whatCell : une cellule de la feuille (à partir de 0).
type whatCell struct{ Row, Col int }

func (c whatCell) addr() string { return formula.ColumnLetters(c.Col) + strconv.Itoa(c.Row+1) }

var whatRefRe = regexp.MustCompile(`^\$?([A-Za-z]{1,3})\$?([1-9][0-9]{0,6})$`)

// whatRef lit une adresse (B3, $B$3).
func whatRef(s string) (whatCell, bool) {
	m := whatRefRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return whatCell{}, false
	}
	r, c, ok := formula.CellRef("Cell_" + m[1] + m[2])
	return whatCell{r, c}, ok
}

// parseCells lit une liste d'adresses et de plages (B1; B3:C4), séparées
// par « ; », « , » ou des espaces.
func parseCells(s string) ([]whatCell, error) {
	var out []whatCell
	seen := map[whatCell]bool{}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == ',' || r == ' ' }) {
		a, b, isRange := strings.Cut(part, ":")
		p, ok := whatRef(a)
		q := p
		if isRange {
			var ok2 bool
			q, ok2 = whatRef(b)
			ok = ok && ok2
		}
		if !ok {
			return nil, msg.New("whatif.badRef", msg.Args{"ref": part})
		}
		for r := min(p.Row, q.Row); r <= max(p.Row, q.Row); r++ {
			for c := min(p.Col, q.Col); c <= max(p.Col, q.Col); c++ {
				if len(out) >= maxWhatCells {
					return nil, msg.New("whatif.tooMany", msg.Args{"max": maxWhatCells})
				}
				if w := (whatCell{r, c}); !seen[w] {
					seen[w] = true
					out = append(out, w)
				}
			}
		}
	}
	return out, nil
}

// oneCell lit une seule adresse.
func oneCell(s string) (whatCell, error) {
	cs, err := parseCells(s)
	if err != nil {
		return whatCell{}, err
	}
	if len(cs) != 1 {
		return whatCell{}, msg.New("whatif.oneCell", msg.Args{"ref": strings.TrimSpace(s)})
	}
	return cs[0], nil
}

// whatNumber lit un nombre tapé dans la langue de la session.
func (m *Model) whatNumber(ctx context.Context, s string) (float64, error) {
	if f, ok := pasteNumber(m.ui(ctx).valueIn(strings.TrimSpace(s))); ok {
		return f, nil
	}
	return 0, msg.New("whatif.badNumber", msg.Args{"value": s})
}

// errTrial : fin d'un essai, la transaction est annulée.
var errTrial = errors.New("essai")

// trial exécute fn dans une transaction d'écriture qui est ensuite annulée.
func (m *Model) trial(ctx context.Context, fn func(*tx) error) error {
	err := m.Do(ctx, "Essai", func(t *tx) error {
		if err := fn(t); err != nil {
			return err
		}
		return errTrial
	})
	if errors.Is(err, errTrial) {
		return nil
	}
	return err
}

// whatInput : une cellule que l'analyse peut changer.
type whatInput struct {
	sheet int64
	at    whatCell
	table int64 // 0 : cellule libre
	col   int   // rang de la colonne dans le tableau
	rid   int64
}

// inputCell vérifie qu'une cellule peut recevoir une valeur : cellule libre
// vide ou constante, ou cellule saisie d'un tableau (pas une colonne
// calculée ni un en-tête).
func (t *tx) inputCell(sheet int64, c whatCell) (whatInput, error) {
	in := whatInput{sheet: sheet, at: c}
	bad := msg.New("whatif.notInput", msg.Args{"cell": c.addr()})
	tables, err := t.tables(sheet)
	if err != nil {
		return in, err
	}
	if tb := tableOver(tables, c.Row, c.Col); tb != nil || bandOver(tables, c) {
		if tb == nil || c.Row <= tb.Row {
			return in, bad
		}
		i := c.Col - tb.Col
		if tb.Cols[i].Formula != "" {
			return in, bad
		}
		rid, _, err := t.rowID(*tb, c.Row-tb.Row-1)
		if err != nil {
			return in, bad
		}
		in.table, in.col, in.rid = tb.ID, i, rid
		return in, nil
	}
	var raw string
	if err := t.row(`SELECT raw FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, c.Row, c.Col).Scan(&raw); err == nil && isFormula(raw) {
		return in, bad
	}
	return in, nil
}

func bandOver(tables []Table, c whatCell) bool {
	for _, tb := range tables {
		if tb.TwoLevel && c.Row == tb.Row-1 && c.Col >= tb.Col && c.Col <= tb.Right() {
			return true
		}
	}
	return false
}

// setInput écrit raw (forme gardée) dans la cellule.
func (t *tx) setInput(in whatInput, raw string) error {
	if in.table == 0 {
		return t.setFreeCell(in.sheet, in.at.Row, in.at.Col, raw)
	}
	tb, err := t.table(in.table)
	if err != nil {
		return err
	}
	var ev Event
	_, err = t.setCellAt(tb, in.rid, in.col, raw, &ev)
	return err
}

// setNumber écrit un nombre dans la cellule.
func (t *tx) setNumber(in whatInput, f float64) error { return t.setInput(in, numberRaw(f)) }

// numberRaw : un nombre dans la forme gardée, sans le bruit de l'arithmétique
// binaire (un entier à 1e-9 près s'écrit entier).
func numberRaw(f float64) string {
	if r := math.Round(f); math.Abs(f-r) <= 1e-9*math.Max(1, math.Abs(f)) {
		f = r
	}
	return formula.FormatNumber(f)
}

// cellValue lit la valeur d'une cellule telle que la grille la calcule.
func (t *tx) cellValue(sheet int64, c whatCell) (formula.Value, error) {
	tables, err := t.tables(sheet)
	if err != nil {
		return formula.Value{}, err
	}
	if tb := tableOver(tables, c.Row, c.Col); tb != nil {
		i := c.Col - tb.Col
		if c.Row == tb.Row {
			return formula.Str(tb.Cols[i].Name), nil
		}
		rid, _, err := t.rowID(*tb, c.Row-tb.Row-1)
		if err != nil {
			return formula.Value{}, nil
		}
		exprs, args, _, err := t.columnSQL(*tb)
		if err != nil {
			return formula.Err(formula.ErrValue), nil
		}
		vals := make([]any, len(exprs))
		ptrs := make([]any, len(exprs))
		for k := range vals {
			ptrs[k] = &vals[k]
		}
		q := `SELECT ` + strings.Join(exprs, ", ") + ` FROM ` + tb.sql() + ` WHERE ` + tb.sql() + `.id = ` + strconv.FormatInt(rid, 10)
		if err := t.row(q, args...).Scan(ptrs...); err != nil {
			return formula.Err(formula.ErrValue), nil
		}
		return formula.FromResult(vals[i]), nil
	}
	var raw string
	if err := t.row(`SELECT raw FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, c.Row, c.Col).Scan(&raw); err != nil {
		return formula.Value{}, nil
	}
	if isFormula(raw) {
		return t.evalFreeRaw(sheet, raw), nil
	}
	switch n := valueLiteral(raw).(type) {
	case formula.NumLit:
		return formula.Num(n.V), nil
	case formula.BoolLit:
		return formula.Boolean(n.V), nil
	case formula.StrLit:
		return formula.Str(n.V), nil
	}
	return t.evalFreeRaw(sheet, "="+raw), nil
}

// cellNumber : la valeur d'une cellule en nombre (vide : 0) ; faux pour un
// texte ou une erreur.
func (t *tx) cellNumber(sheet int64, c whatCell) (float64, bool, error) {
	v, err := t.cellValue(sheet, c)
	if err != nil {
		return 0, false, err
	}
	f, ok := valueNumber(v)
	return f, ok, nil
}

func valueNumber(v formula.Value) (float64, bool) {
	switch v.Kind {
	case formula.Number, formula.Date:
		return v.Num, !math.IsNaN(v.Num) && !math.IsInf(v.Num, 0)
	case formula.Bool:
		if v.Bool {
			return 1, true
		}
		return 0, true
	case formula.Empty:
		return 0, true
	case formula.Text:
		return pasteNumber(v.Str)
	}
	return 0, false
}

// formulaCell vérifie que la cellule est calculée (formule libre, colonne
// calculée).
func (t *tx) formulaCell(sheet int64, c whatCell) error {
	bad := msg.New("whatif.notFormula", msg.Args{"cell": c.addr()})
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	if tb := tableOver(tables, c.Row, c.Col); tb != nil {
		if c.Row == tb.Row || tb.Cols[c.Col-tb.Col].Formula == "" {
			return bad
		}
		return nil
	}
	var raw string
	if err := t.row(`SELECT raw FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, c.Row, c.Col).Scan(&raw); err != nil || !isFormula(raw) {
		return bad
	}
	return nil
}

// showValue : une valeur calculée dans l'écriture de la session.
func (t *tx) showValue(v formula.Value) string { return t.ui().valueOut(valueRaw(v)) }

// --- Valeur cible ---------------------------------------------------------

// GoalSeekResult : issue d'une recherche de valeur cible. Raw (forme
// gardée) est la valeur à poser par SetWhatIfValues (« Conserver »).
type GoalSeekResult struct {
	Found  bool   `json:"found"`
	Cell   string `json:"cell"`
	Raw    string `json:"raw"`
	Value  string `json:"value"`
	Result string `json:"result"`
	Evals  int    `json:"evals"`
}

// GoalSeek cherche la valeur de la cellule changing (constante ou cellule
// saisie d'un tableau) pour laquelle la formule de target vaut goal (nombre
// tapé dans la langue de la session). Rien n'est changé dans le classeur.
func (m *Model) GoalSeek(ctx context.Context, sheet int64, target, goal, changing string) (GoalSeekResult, error) {
	var res GoalSeekResult
	tc, err := oneCell(target)
	if err != nil {
		return res, err
	}
	cc, err := oneCell(changing)
	if err != nil {
		return res, err
	}
	want, err := m.whatNumber(ctx, goal)
	if err != nil {
		return res, err
	}
	res.Cell = cc.addr()
	err = m.trial(ctx, func(t *tx) error {
		if err := t.guardCells(sheet, Rect{cc.Row, cc.Col, cc.Row, cc.Col}, false); err != nil {
			return err
		}
		if err := t.formulaCell(sheet, tc); err != nil {
			return err
		}
		in, err := t.inputCell(sheet, cc)
		if err != nil {
			return err
		}
		x0, ok, err := t.cellNumber(sheet, cc)
		if err != nil {
			return err
		}
		if !ok {
			x0 = 0
		}
		var failed error
		f := func(x float64) (float64, bool) {
			if failed != nil || ctx.Err() != nil {
				return 0, false
			}
			if err := t.setNumber(in, x); err != nil {
				failed = err
				return 0, false
			}
			y, ok, err := t.cellNumber(sheet, tc)
			if err != nil {
				failed = err
				return 0, false
			}
			return y - want, ok
		}
		r := optim.Root(f, x0, 300, 1e-9*math.Max(1, math.Abs(want)))
		if failed != nil {
			return failed
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		res.Found, res.Evals = r.Found, r.Evals
		if math.IsNaN(r.F) {
			r.X = x0
		}
		res.Raw = numberRaw(r.X)
		res.Value = t.ui().valueOut(res.Raw)
		// Le résultat au point gardé (arrondi à 15 chiffres).
		if err := t.setInput(in, res.Raw); err != nil {
			return err
		}
		v, err := t.cellValue(sheet, tc)
		if err != nil {
			return err
		}
		res.Result = t.showValue(v)
		return nil
	})
	return res, err
}

// SetWhatIfValues pose des valeurs (forme gardée) dans des cellules
// d'entrée, en une étape d'annulation : « Conserver » de la valeur cible
// (kind "goal") et du solveur ("solver").
func (m *Model) SetWhatIfValues(ctx context.Context, sheet int64, kind string, cells, raws []string) error {
	if len(cells) != len(raws) {
		return msg.New("whatif.scenarioValues", nil)
	}
	label := "Valeur cible"
	if kind == "solver" {
		label = "Solveur"
	}
	var at []whatCell
	for _, s := range cells {
		c, err := oneCell(s)
		if err != nil {
			return err
		}
		at = append(at, c)
	}
	return m.Do(ctx, label, func(t *tx) error { return t.putInputs(sheet, at, raws) })
}

// putInputs écrit les valeurs dans les cellules d'entrée.
func (t *tx) putInputs(sheet int64, at []whatCell, raws []string) error {
	ins := make([]whatInput, len(at))
	for i, c := range at {
		if err := t.guardCells(sheet, Rect{c.Row, c.Col, c.Row, c.Col}, false); err != nil {
			return err
		}
		in, err := t.inputCell(sheet, c)
		if err != nil {
			return err
		}
		ins[i] = in
	}
	for i, in := range ins {
		if err := t.setInput(in, raws[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- Table de données -----------------------------------------------------

// dataTablesKey : définitions des tables de données (_lattis_info), pour
// les recalculer en rouvrant la boîte sur la même plage.
const dataTablesKey = "dataTables"

// DataTableDef : une table de données posée. Row : cellule d'entrée en ligne
// (valeurs de la première ligne), Col : en colonne (valeurs de la première
// colonne) ; les deux : table à deux variables.
type DataTableDef struct {
	Sheet int64  `json:"sheet"`
	Area  Rect   `json:"area"`
	Row   string `json:"row,omitempty"`
	Col   string `json:"col,omitempty"`
}

func (t *tx) dataTables() []DataTableDef {
	raw, _, _ := t.infoValue(dataTablesKey)
	var out []DataTableDef
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}

// DataTableAt rend la table de données posée sur cette plage, ou nil.
func (m *Model) DataTableAt(ctx context.Context, sheet int64, area Rect) (*DataTableDef, error) {
	var out *DataTableDef
	err := m.read(ctx, func(t *tx) error {
		for _, d := range t.dataTables() {
			if d.Sheet == sheet && d.Area == area {
				out = &d
				break
			}
		}
		return nil
	})
	return out, err
}

// DataTable calcule une table de données (comme celle d'un tableur) : la
// plage area a ses valeurs d'entrée dans la première ligne (cellule
// d'entrée rowInput) et/ou la première colonne (colInput), et ses formules
// dans l'autre bord (une variable) ou dans le coin (deux variables). Les
// résultats sont posés en valeurs fixes (une étape d'annulation) : relancer
// la commande sur la même plage les recalcule. Rend le nombre de résultats.
func (m *Model) DataTable(ctx context.Context, sheet int64, area Rect, rowInput, colInput string) (int, error) {
	area = Rect{min(area.R0, area.R1), min(area.C0, area.C1), max(area.R0, area.R1), max(area.C0, area.C1)}
	if area.R1-area.R0 < 1 || area.C1-area.C0 < 1 {
		return 0, msg.New("whatif.tableArea", nil)
	}
	if (area.R1-area.R0)*(area.C1-area.C0) > 10_000 {
		return 0, msg.New("whatif.tooMany", msg.Args{"max": 10_000})
	}
	var ri, ci *whatCell
	for _, p := range []struct {
		s   string
		out **whatCell
	}{{rowInput, &ri}, {colInput, &ci}} {
		if strings.TrimSpace(p.s) == "" {
			continue
		}
		c, err := oneCell(p.s)
		if err != nil {
			return 0, err
		}
		if c.Row >= area.R0 && c.Row <= area.R1 && c.Col >= area.C0 && c.Col <= area.C1 {
			return 0, msg.New("whatif.inputInside", nil)
		}
		*p.out = &c
	}
	if ri == nil && ci == nil {
		return 0, msg.New("whatif.noInput", nil)
	}
	// Résultats : (ligne, colonne) → forme gardée.
	type put struct {
		at  whatCell
		raw string
	}
	var puts []put
	err := m.trial(ctx, func(t *tx) error {
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			if tb.Col <= area.C1 && tb.Right() >= area.C0 && tb.Top() <= area.R1 && tb.Bottom() >= area.R0 {
				return msg.New("whatif.tableArea", nil)
			}
		}
		var rin, cin whatInput
		for _, p := range []struct {
			c  *whatCell
			in *whatInput
		}{{ri, &rin}, {ci, &cin}} {
			if p.c == nil {
				continue
			}
			if *p.in, err = t.inputCell(sheet, *p.c); err != nil {
				return err
			}
		}
		// Valeur d'entrée d'un bord : celle que la cellule affiche, posée
		// telle quelle (forme gardée) dans la cellule d'entrée.
		entry := func(c whatCell) (string, error) {
			v, err := t.cellValue(sheet, c)
			if err != nil {
				return "", err
			}
			return valueRaw(v), nil
		}
		type job struct {
			at, formula, rv, cv whatCell
			hasR, hasC          bool
		}
		var jobs []job
		switch {
		case ri != nil && ci != nil:
			for r := area.R0 + 1; r <= area.R1; r++ {
				for c := area.C0 + 1; c <= area.C1; c++ {
					jobs = append(jobs, job{at: whatCell{r, c}, formula: whatCell{area.R0, area.C0}, rv: whatCell{area.R0, c}, cv: whatCell{r, area.C0}, hasR: true, hasC: true})
				}
			}
		case ci != nil:
			// Valeurs en colonne, formules dans la première ligne.
			for r := area.R0 + 1; r <= area.R1; r++ {
				for c := area.C0 + 1; c <= area.C1; c++ {
					jobs = append(jobs, job{at: whatCell{r, c}, formula: whatCell{area.R0, c}, cv: whatCell{r, area.C0}, hasC: true})
				}
			}
		default:
			// Valeurs en ligne, formules dans la première colonne.
			for r := area.R0 + 1; r <= area.R1; r++ {
				for c := area.C0 + 1; c <= area.C1; c++ {
					jobs = append(jobs, job{at: whatCell{r, c}, formula: whatCell{r, area.C0}, rv: whatCell{area.R0, c}, hasR: true})
				}
			}
		}
		// Au moins une formule dans le bord (ou le coin).
		some := false
		for _, j := range jobs {
			if t.formulaCell(sheet, j.formula) == nil {
				some = true
				break
			}
		}
		if !some {
			return msg.New("whatif.notFormula", msg.Args{"cell": jobs[0].formula.addr()})
		}
		// Les valeurs d'entrée lues avant tout essai.
		entries := map[whatCell]string{}
		for _, j := range jobs {
			for _, e := range []struct {
				c  whatCell
				on bool
			}{{j.rv, j.hasR}, {j.cv, j.hasC}} {
				if _, ok := entries[e.c]; e.on && !ok {
					if entries[e.c], err = entry(e.c); err != nil {
						return err
					}
				}
			}
		}
		formulas := map[whatCell]bool{}
		for _, j := range jobs {
			if err := ctx.Err(); err != nil {
				return err
			}
			// Pas de formule à cette place du bord : rien à calculer.
			isF, ok := formulas[j.formula]
			if !ok {
				isF = t.formulaCell(sheet, j.formula) == nil
				formulas[j.formula] = isF
			}
			if !isF {
				puts = append(puts, put{j.at, ""})
				continue
			}
			if j.hasR {
				if err := t.setInput(rin, entries[j.rv]); err != nil {
					return err
				}
			}
			if j.hasC {
				if err := t.setInput(cin, entries[j.cv]); err != nil {
					return err
				}
			}
			v, err := t.cellValue(sheet, j.formula)
			if err != nil {
				return err
			}
			puts = append(puts, put{j.at, valueRaw(v)})
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	n := 0
	err = m.Do(ctx, "Table de données", func(t *tx) error {
		if err := t.guardCells(sheet, Rect{area.R0 + 1, area.C0 + 1, area.R1, area.C1}, false); err != nil {
			return err
		}
		for _, p := range puts {
			if err := t.setFreeCell(sheet, p.at.Row, p.at.Col, p.raw); err != nil {
				return err
			}
			if p.raw != "" {
				n++
			}
		}
		def := DataTableDef{Sheet: sheet, Area: area}
		if ri != nil {
			def.Row = ri.addr()
		}
		if ci != nil {
			def.Col = ci.addr()
		}
		defs := []DataTableDef{def}
		for _, d := range t.dataTables() {
			if d.Sheet != sheet || d.Area != area {
				defs = append(defs, d)
			}
		}
		b, err := json.Marshal(defs[:min(len(defs), 100)])
		if err != nil {
			return err
		}
		if err := t.setInfo(dataTablesKey, string(b)); err != nil {
			return err
		}
		t.notes = append(t.notes, t.say("whatif.dataTableDone", msg.Args{"n": n}))
		return nil
	})
	return n, err
}
