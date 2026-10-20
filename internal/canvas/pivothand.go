package canvas

import (
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

// Colonnes saisies à droite d'un tableau croisé (docs/decisions/0008, partie
// TCD).
//
// Un nom tapé juste à droite de l'en-tête d'un TCD y ajoute une colonne
// saisie, dessinée dans son cadre (en-tête vert). Ses cellules reçoivent des
// valeurs, ligne par ligne, ou une formule pour toute la colonne ([Nombre]
// y désigne la valeur de la ligne, TCD1[Nombre] la colonne entière, une
// adresse A1 une cellule de la feuille).
//
// Les valeurs suivent les étiquettes de lignes : elles sont rangées, toutes,
// dans _lattis_hand_kept (owner « pivot:<id> »), par clé = champs des lignes
// et étiquettes de la ligne. Le TCD recalculé (actualisation, filtre, tri,
// champs changés) les reprend sur la ligne de même clé ; une clé absente du
// TCD laisse ses valeurs de côté, jusqu'à son retour ou « Oublier les
// valeurs mises de côté ». Les noms et formules des colonnes vivent dans la
// configuration du TCD (PivotConfig.Hand) ; elles s'ajoutent à droite des
// cellules calculées, et suivent donc le bord droit quand le TCD s'élargit.

// PivotHandCol est une colonne saisie d'un tableau croisé.
type PivotHandCol struct {
	// H : numéro stable (les valeurs le citent ; un renommage ne le change pas).
	H    int64  `json:"h"`
	Name string `json:"name"`
	// Formula : formule de la colonne (anglais canonique, comme une colonne
	// calculée) ; vide :
	// valeurs saisies.
	Formula string `json:"formula,omitempty"`
}

// PivotHandInfo décrit une colonne saisie affichée (les dernières colonnes
// de PivotInfo.Cells).
type PivotHandInfo struct {
	Name string `json:"name"`
	// Formula : formule de la colonne en français, « = » compris (vide : valeurs).
	Formula string `json:"formula,omitempty"`
}

func pivotOwner(id int64) string { return "pivot:" + strconv.FormatInt(id, 10) }

// pivotKeySep sépare les étiquettes d'une clé ; pivotSigSep sépare les
// champs des lignes (signature) des étiquettes.
const (
	pivotKeySep = "\x1f"
	pivotSigSep = "\x1e"
)

// pivotHandSig : les champs des lignes, en tête de chaque clé. Changer de
// champ en lignes met les valeurs de côté plutôt que de les poser sur des
// étiquettes qui auraient le même texte ; y revenir les rend.
func pivotHandSig(cfg PivotConfig) string {
	f := make([]string, len(cfg.RowFields))
	for i, x := range cfg.RowFields {
		f[i] = strings.ToLower(strings.TrimSpace(x))
	}
	return strings.Join(f, pivotKeySep) + pivotSigSep
}

// pivotRowKeys donne la clé de chaque ligne affichée (indice dans cells) :
// "" pour l'en-tête, le total général, les groupes et sous-totaux de
// l'arborescence (pas de saisie : seule une formule de colonne s'y calcule).
func pivotRowKeys(cfg PivotConfig, cells [][]string, rows []PivotRowInfo) []string {
	keys := make([]string, len(cells))
	nk := len(cfg.RowFields)
	if nk == 0 || len(cells) < 2 {
		return keys
	}
	sig := pivotHandSig(cfg)
	if rows != nil {
		for i, info := range rows {
			if i+1 >= len(cells) || info.Group || info.Subtotal || info.Total || len(info.Path) != nk {
				continue
			}
			keys[i+1] = sig + strings.Join(info.Path, pivotKeySep)
		}
		return keys
	}
	n := len(cells)
	if !cfg.NoGrandTotal && len(cells[0]) > nk {
		n-- // total général
	}
	for i := 1; i < n; i++ {
		if len(cells[i]) >= nk {
			keys[i] = sig + strings.Join(cells[i][:nk], pivotKeySep)
		}
	}
	return keys
}

// pivotLiveKeys : clés de toutes les lignes du TCD (calcul à plat : groupes
// pliés compris), sans le filtre de l'onglet à choix (une navigation ne met
// rien de côté).
func (t *tx) pivotLiveKeys(cfg PivotConfig) (map[string]bool, error) {
	out := map[string]bool{}
	nk := len(cfg.RowFields)
	if nk == 0 {
		return out, nil
	}
	cfg.Choice = nil
	body, err := t.pivotBody(cfg)
	if err != nil {
		return nil, err
	}
	sig := pivotHandSig(cfg)
	for _, r := range body[1:] {
		if len(r) >= nk {
			out[sig+strings.Join(r[:nk], pivotKeySep)] = true
		}
	}
	return out, nil
}

// handVals : valeurs saisies d'un TCD, par clé puis par colonne (h).
type handVals map[string]map[int64]string

func (t *tx) pivotHandVals(owner string) (handVals, error) {
	rs, err := t.query(`SELECT k, h, v FROM _lattis_hand_kept WHERE owner = ?`, owner)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := handVals{}
	for rs.Next() {
		var k string
		var h int64
		var v any
		if err := rs.Scan(&k, &h, &v); err != nil {
			return nil, err
		}
		if out[k] == nil {
			out[k] = map[int64]string{}
		}
		out[k][h] = handKeyText(v)
	}
	return out, rs.Err()
}

// shownHand : texte affiché d'une saisie (sans l'apostrophe qui force le texte).
func shownHand(raw string) string {
	if s := strings.TrimLeft(raw, " \t"); strings.HasPrefix(s, "'") {
		return strings.TrimPrefix(s, "'")
	}
	return raw
}

// handSQL écrit une saisie en littéral SQL lisible par les formules : nombre
// (date, heure et pourcentage compris), booléen, texte ; vide : NULL.
func handSQL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "NULL"
	}
	switch n := valueLiteral(raw).(type) {
	case formula.NumLit:
		return strconv.FormatFloat(n.V, 'g', -1, 64)
	case formula.BoolLit:
		if n.V {
			return "1"
		}
		return "0"
	case formula.StrLit:
		return sqlLiteral(n.V)
	case formula.Call:
		if d, ok := grid.ParseDate(strings.TrimSpace(raw)); ok {
			return strconv.FormatFloat(formula.TimeSerial(d), 'g', -1, 64)
		}
	}
	return sqlLiteral(shownHand(raw))
}

// pivotHandCells calcule les colonnes saisies d'un TCD affiché (cells, rows :
// le calcul sans elles) : une ligne de cellules par ligne de cells, l'en-tête
// portant leurs noms. names : valeurs des cellules libres pour les formules
// (nil pendant le calcul de ces valeurs : une adresse y vaut une case vide).
func (t *tx) pivotHandCells(id, sheet int64, cfg PivotConfig, cells [][]string, rows []PivotRowInfo, names map[string]formula.Value) ([][]string, error) {
	hand := cfg.Hand
	out := make([][]string, len(cells))
	if len(hand) == 0 || len(cells) == 0 {
		return out, nil
	}
	vals, err := t.pivotHandVals(pivotOwner(id))
	if err != nil {
		return nil, err
	}
	keys := pivotRowKeys(cfg, cells, rows)
	formulas := false
	for i := range cells {
		row := make([]string, len(hand))
		for j, h := range hand {
			switch {
			case i == 0:
				row[j] = h.Name
			case h.Formula != "":
				formulas = true
			case keys[i] != "":
				row[j] = shownHand(vals[keys[i]][h.H])
			}
		}
		out[i] = row
	}
	if !formulas || len(cells) < 2 {
		return out, nil
	}
	res := t.pivotHandEval(sheet, cfg, cells, rows, keys, vals, names)
	for j, h := range hand {
		if r, ok := res[h.H]; ok {
			for i := 1; i < len(cells); i++ {
				out[i][j] = r[i]
			}
		}
	}
	return out, nil
}

// pivotHandEval calcule les colonnes à formule d'un TCD : le TCD affiché
// devient une table (une ligne par ligne affichée, totaux compris) où la
// formule se calcule comme une colonne calculée ; TCD1[Col], dans un
// agrégat, ne lit que les lignes de détail (pas les totaux). Rend, par
// colonne (h), le texte de chaque ligne (indice dans cells).
func (t *tx) pivotHandEval(sheet int64, cfg PivotConfig, cells [][]string, rows []PivotRowInfo, keys []string, vals handVals, names map[string]formula.Value) map[int64][]string {
	header := cells[0]
	ft := &formula.Table{Name: cfg.Name, Sheet: sheet}
	taken := map[string]bool{}
	add := func(name, sql string, f formula.Node) int {
		low := strings.ToLower(strings.TrimSpace(name))
		if low == "" || taken[low] {
			return -1
		}
		taken[low] = true
		ft.Cols = append(ft.Cols, formula.Column{Name: strings.TrimSpace(name), SQL: sql, Formula: f})
		return len(ft.Cols) - 1
	}
	sel := []string{"column1 AS id", "column2 AS _pos", "column3 AS _d"}
	n := 3
	col := func(alias string) string {
		n++
		sel = append(sel, fmt.Sprintf("column%d AS %s", n, alias))
		return alias
	}
	for j, h := range header {
		add(h, col(fmt.Sprintf("c%d", j)), nil)
	}
	compact := rows != nil
	if compact {
		for k, f := range cfg.RowFields {
			add(f, col(fmt.Sprintf("k%d", k)), nil)
		}
	}
	var typed []PivotHandCol
	for _, h := range cfg.Hand {
		if h.Formula == "" {
			typed = append(typed, h)
			add(h.Name, col(fmt.Sprintf("h%d", h.H)), nil)
		}
	}
	idx := map[int64]int{}
	for _, h := range cfg.Hand {
		if h.Formula == "" {
			continue
		}
		node, err := formula.Parse(h.Formula, formula.EN)
		if err != nil {
			node = formula.ErrLit{V: formula.ErrName}
		}
		if i := add(h.Name, fmt.Sprintf("f%d", h.H), formula.QualifyCells(node, sheet)); i >= 0 {
			idx[h.H] = i
		}
	}
	var lines []string
	for i := 1; i < len(cells); i++ {
		d := "0"
		if keys[i] != "" {
			d = "1"
		}
		parts := []string{strconv.Itoa(i), strconv.Itoa(i), d}
		for j := range header {
			v := ""
			if j < len(cells[i]) {
				v = cells[i][j]
			}
			parts = append(parts, sqlValue(v))
		}
		if compact {
			var path []string
			if i-1 < len(rows) {
				path = rows[i-1].Path
			}
			for k := range cfg.RowFields {
				if k < len(path) && path[k] != "" {
					parts = append(parts, sqlValue(path[k]))
				} else {
					parts = append(parts, "NULL")
				}
			}
		}
		for _, h := range typed {
			v := ""
			if keys[i] != "" {
				v = vals[keys[i]][h.H]
			}
			parts = append(parts, handSQL(v))
		}
		lines = append(lines, "("+strings.Join(parts, ", ")+")")
	}
	values := " FROM (VALUES " + strings.Join(lines, ", ") + ")"
	base := "SELECT " + strings.Join(sel, ", ") + values
	ft.SQL = "(SELECT " + strings.Join(sel, ", ") + values + " WHERE column3 = 1)"
	fail := func(code formula.ErrCode) map[int64][]string {
		out := map[int64][]string{}
		for h := range idx {
			r := make([]string, len(cells))
			for i := 1; i < len(cells); i++ {
				r[i] = formula.Err(code).String()
			}
			out[h] = r
		}
		return out
	}
	schema, err := t.schemaFor(sheet)
	if err != nil {
		return fail(formula.ErrValue)
	}
	s := *schema
	if _, ok := schema.Table(cfg.Name); !ok {
		s.Tables = append(slices.Clip(schema.Tables), ft)
	}
	hoist, _ := t.hoister()
	var exprs []string
	var args []any
	var order []int64
	for _, h := range cfg.Hand {
		i, ok := idx[h.H]
		if !ok {
			continue
		}
		cc := formula.CompileColumnFrom(ft, i, &s, names, "p", hoist, len(args))
		exprs = append(exprs, cc.SQL)
		args = append(args, cc.Args...)
		order = append(order, h.H)
	}
	if len(exprs) == 0 {
		return nil
	}
	rs, err := t.query(`SELECT `+strings.Join(exprs, ", ")+` FROM (`+base+`) AS p ORDER BY p._pos`, args...)
	if err != nil {
		return fail(formula.ErrValue)
	}
	defer rs.Close()
	out := map[int64][]string{}
	for _, h := range order {
		out[h] = make([]string, len(cells))
	}
	got := make([]any, len(exprs))
	ptrs := make([]any, len(exprs))
	for i := range got {
		ptrs[i] = &got[i]
	}
	for i := 1; rs.Next() && i < len(cells); i++ {
		if err := rs.Scan(ptrs...); err != nil {
			return fail(formula.ErrValue)
		}
		for k, h := range order {
			out[h][i] = formula.FromResult(got[k]).String()
		}
	}
	if rs.Err() != nil {
		return fail(formula.ErrValue)
	}
	return out
}

// pivotHandAttach ajoute à un TCD calculé ses colonnes saisies (cellules,
// formats, description) et le compte des valeurs mises de côté.
func (t *tx) pivotHandAttach(p *PivotInfo, sheet int64, hand []PivotHandCol, names map[string]formula.Value) error {
	if len(hand) == 0 || p.Error != "" || len(p.Cells) == 0 {
		return nil
	}
	cfg := p.Config
	cfg.Hand = hand
	extra, err := t.pivotHandCells(p.ID, sheet, cfg, p.Cells, p.Rows, names)
	if err != nil {
		return err
	}
	// Les cellules viennent du cache : copiées avant d'être allongées.
	cells := make([][]string, len(p.Cells))
	for i, r := range p.Cells {
		cells[i] = append(slices.Clip(r), extra[i]...)
	}
	p.Cells = cells
	if p.Formats != nil {
		p.Formats = append(slices.Clip(p.Formats), make([]string, len(hand))...)
	}
	for _, h := range hand {
		p.Hand = append(p.Hand, PivotHandInfo{Name: h.Name, Formula: displayFormula(h.Formula)})
	}
	if p.HandKept, err = t.pivotKeptCount(p.ID, cfg); err != nil {
		return err
	}
	if len(cfg.RowFields) == 0 {
		p.HandNote = "Aucun champ en lignes : les valeurs saisies à droite du tableau croisé sont mises de côté ; elles reviendront avec leur champ en lignes."
	}
	return nil
}

// pivotKeptCount : nombre de lignes (clés) dont les valeurs saisies sont
// mises de côté (étiquettes absentes du TCD).
func (t *tx) pivotKeptCount(id int64, cfg PivotConfig) (int, error) {
	keys, err := t.pivotStoredKeys(pivotOwner(id))
	if err != nil || len(keys) == 0 {
		return 0, err
	}
	live, err := t.pivotLiveKeys(cfg)
	if err != nil {
		return 0, nil // TCD en erreur : rien à compter
	}
	n := 0
	for _, k := range keys {
		if !live[k] {
			n++
		}
	}
	return n, nil
}

func (t *tx) pivotStoredKeys(owner string) ([]string, error) {
	rs, err := t.query(`SELECT DISTINCT k FROM _lattis_hand_kept WHERE owner = ? ORDER BY k`, owner)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var k string
		if err := rs.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rs.Err()
}

// pivotPlace est un TCD posé sur une feuille, calculé sans ses colonnes
// saisies : la géométrie que lisent la saisie et l'effacement.
type pivotPlace struct {
	id    int64
	row   int // ancre (premier filtre de rapport, sinon l'en-tête)
	top   int // ligne de l'en-tête
	col   int
	cfg   PivotConfig
	cells [][]string
	rows  []PivotRowInfo
}

func (p pivotPlace) base() int      { return len(p.cells[0]) }
func (p pivotPlace) handCol() int   { return p.col + p.base() }
func (p pivotPlace) right() int     { return p.handCol() + len(p.cfg.Hand) - 1 }
func (p pivotPlace) bottom() int    { return p.top + len(p.cells) - 1 }
func (p pivotPlace) keys() []string { return pivotRowKeys(p.cfg, p.cells, p.rows) }

func (t *tx) pivotPlaces(sheet int64) ([]pivotPlace, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _lattis_pivots WHERE sheet = ? ORDER BY id`, sheet)
	if err != nil {
		return nil, err
	}
	var list []pivotPlace
	for rs.Next() {
		var p pivotPlace
		var raw string
		if err := rs.Scan(&p.id, &p.row, &p.col, &raw); err != nil {
			rs.Close()
			return nil, err
		}
		if json.Unmarshal([]byte(raw), &p.cfg) != nil {
			continue
		}
		p.cfg = p.cfg.normalized()
		if p.cfg.Name == "" {
			p.cfg.Name = fmt.Sprintf("TCD%d", p.id)
		}
		p.top = p.row
		if n := len(p.cfg.Filters); n > 0 {
			p.top += n + 1
		}
		list = append(list, p)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	var out []pivotPlace
	for _, p := range list {
		if err := t.pivotChoice(sheet, &p.cfg); err != nil {
			return nil, err
		}
		cells, rows, err := t.pivotCells(p.cfg)
		if err != nil || len(cells) == 0 || len(cells[0]) == 0 {
			continue // TCD en erreur : seule sa cellule de message
		}
		p.cells, p.rows = cells, rows
		out = append(out, p)
	}
	return out, nil
}

// pivotHandInput : saisie dans la cellule (row, col) d'une feuille. Rend
// vrai si elle concerne les colonnes saisies d'un TCD : un nom juste à
// droite de son en-tête (nouvelle colonne), l'en-tête d'une colonne saisie
// (renommer ; effacé : retirer la colonne), ou une de ses cellules.
func (t *tx) pivotHandInput(sheet int64, row, col int, raw string) (bool, error) {
	places, err := t.pivotPlaces(sheet)
	if err != nil || len(places) == 0 {
		return false, err
	}
	for _, p := range places {
		nh := len(p.cfg.Hand)
		switch {
		case row == p.top && col == p.handCol()+nh:
			name := strings.TrimSpace(raw)
			if name == "" || isFormula(raw) {
				continue
			}
			if busy, err := t.cellTaken(sheet, row, col, p.id); err != nil || busy {
				if err != nil {
					return false, err
				}
				continue
			}
			return true, t.addPivotHand(sheet, p, col, name)
		case col >= p.handCol() && col <= p.right() && row >= p.top && row <= p.bottom():
			if err := t.guardObject(`_lattis_pivots`, p.id); err != nil {
				return true, err
			}
			j := col - p.handCol()
			if row == p.top {
				return true, t.renamePivotHand(p, j, raw)
			}
			return true, t.setPivotHandCell(sheet, p, j, row-p.top, raw)
		}
	}
	return false, nil
}

// cellTaken : la cellule est dans un tableau ou un autre TCD (skip).
func (t *tx) cellTaken(sheet int64, row, col int, skip int64) (bool, error) {
	tables, err := t.tables(sheet)
	if err != nil {
		return false, err
	}
	for _, tb := range tables {
		if row >= tb.Row && row <= tb.Bottom() && col >= tb.Col && col <= tb.Right() {
			return true, nil
		}
	}
	places, err := t.pivotPlaces(sheet)
	if err != nil {
		return false, err
	}
	for _, o := range places {
		if o.id != skip && row >= o.row && row <= o.bottom() && col >= o.col && col <= o.right() {
			return true, nil
		}
	}
	return false, nil
}

// pivotHandNameTaken : un nom déjà pris par une colonne du TCD (calculée ou
// saisie, sauf la colonne saisie skip).
func pivotHandNameTaken(p pivotPlace, name string, skip int) bool {
	for _, h := range p.cells[0] {
		if strings.EqualFold(strings.TrimSpace(h), name) {
			return true
		}
	}
	for j, h := range p.cfg.Hand {
		if j != skip && strings.EqualFold(h.Name, name) {
			return true
		}
	}
	return false
}

func errHandName(p pivotPlace, name string) error {
	return msg.Wrap(ErrPivot, "pivot.nameIsColumn", msg.Args{"name": name, "pivot": p.cfg.Name})
}

// savePivotHand enregistre les colonnes saisies d'un TCD (annulable) ; le
// reste de sa configuration est gardé tel quel.
func (t *tx) savePivotHand(id int64, hand []PivotHandCol) error {
	var old string
	if err := t.row(`SELECT config FROM _lattis_pivots WHERE id = ?`, id).Scan(&old); err != nil {
		return err
	}
	m := map[string]any{}
	if err := json.Unmarshal([]byte(old), &m); err != nil {
		return err
	}
	if len(hand) == 0 {
		delete(m, "hand")
	} else {
		m["hand"] = hand
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	q := `UPDATE _lattis_pivots SET config = ? WHERE id = ?`
	return t.apply([]stmt{st(q, string(b), id)}, []stmt{st(q, old, id)})
}

// addPivotHand ajoute la colonne saisie name au TCD p, en colonne col de sa
// feuille. Les cellules libres déjà tapées dessous, sur des lignes de
// détail, deviennent ses valeurs.
func (t *tx) addPivotHand(sheet int64, p pivotPlace, col int, name string) (err error) {
	if err := t.guardObject(`_lattis_pivots`, p.id); err != nil {
		return err
	}
	if pivotHandNameTaken(p, name, -1) {
		return errHandName(p, name)
	}
	owner := pivotOwner(p.id)
	// Numéro jamais pris, même par des valeurs d'une colonne retirée.
	var h int64
	if err := t.row(`SELECT coalesce(max(h), 0) FROM _lattis_hand_kept WHERE owner = ?`, owner).Scan(&h); err != nil {
		return err
	}
	for _, c := range p.cfg.Hand {
		h = max(h, c.H)
	}
	h++
	hand := append(slices.Clip(p.cfg.Hand), PivotHandCol{H: h, Name: name})
	if err := t.savePivotHand(p.id, hand); err != nil {
		return err
	}
	// Une colonne de plus : ce qui touche le TCD à droite s'écarte.
	defer func() {
		if err == nil {
			err = t.makeRoom(sheet, p.id)
		}
	}()
	// Cellules libres reprises : l'en-tête tapé, les valeurs dessous.
	keys := p.keys()
	rs, err := t.query(`SELECT row, raw FROM _lattis_cells WHERE sheet = ? AND col = ? AND row BETWEEN ? AND ?`, sheet, col, p.top, p.bottom())
	if err != nil {
		return err
	}
	type taken struct {
		row int
		raw string
	}
	var take []taken
	for rs.Next() {
		var c taken
		if err := rs.Scan(&c.row, &c.raw); err != nil {
			rs.Close()
			return err
		}
		if c.row == p.top || (keys[c.row-p.top] != "" && !isFormula(c.raw)) {
			take = append(take, c)
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	for _, c := range take {
		if err := t.deleteWhere(`_lattis_cells`, `sheet = ? AND row = ? AND col = ?`, sheet, c.row, col); err != nil {
			return err
		}
		if c.row != p.top && strings.TrimSpace(c.raw) != "" {
			if err := t.putPivotHand(owner, keys[c.row-p.top], h, c.raw); err != nil {
				return err
			}
		}
	}
	return nil
}

// renamePivotHand renomme la colonne saisie j ; un nom effacé la retire,
// avec ses valeurs (annulable).
func (t *tx) renamePivotHand(p pivotPlace, j int, raw string) error {
	name := strings.TrimSpace(raw)
	hand := slices.Clone(p.cfg.Hand)
	if name == "" {
		if err := t.forgetKept(pivotOwner(p.id), `h = ?`, hand[j].H); err != nil {
			return err
		}
		return t.savePivotHand(p.id, slices.Delete(hand, j, j+1))
	}
	if isFormula(raw) {
		return msg.Wrap(ErrPivot, "pivot.columnNameNoEquals", nil)
	}
	if name == hand[j].Name {
		return nil
	}
	if pivotHandNameTaken(p, name, j) {
		return errHandName(p, name)
	}
	hand[j].Name = name
	return t.savePivotHand(p.id, hand)
}

// setPivotHandCell : saisie dans la cellule de la colonne saisie j, ligne i
// du TCD (1 : première sous l'en-tête). Une formule vaut pour toute la
// colonne ; une valeur, pour la ligne (pas sur un total).
func (t *tx) setPivotHandCell(sheet int64, p pivotPlace, j, i int, raw string) error {
	hand := slices.Clone(p.cfg.Hand)
	h := hand[j]
	empty := strings.TrimSpace(raw) == ""
	if isFormula(raw) {
		n, err := t.prepareFormula(sheet, strings.TrimSpace(raw))
		if err != nil {
			return err
		}
		hand[j].Formula = formula.Format(n, formula.EN)
		if hand[j].Formula == h.Formula {
			return nil
		}
		return t.savePivotHand(p.id, hand)
	}
	if h.Formula != "" {
		if empty {
			hand[j].Formula = "" // formule effacée : les valeurs saisies reviennent
			return t.savePivotHand(p.id, hand)
		}
		return ErrComputed{h.Name}
	}
	key := p.keys()[i]
	if key == "" {
		if empty {
			return nil
		}
		label := ""
		if len(p.cells[i]) > 0 {
			label = p.cells[i][0]
		}
		return msg.Wrap(ErrPivot, "pivot.totalRowReadOnly", msg.Args{"label": label})
	}
	if empty {
		raw = ""
	} else {
		raw = freeRaw(raw)
	}
	return t.putPivotHand(pivotOwner(p.id), key, h.H, raw)
}

// putPivotHand écrit (raw vide : efface) la valeur de la colonne h sur la
// ligne de clé k (annulable).
func (t *tx) putPivotHand(owner, k string, h int64, raw string) error {
	var old any
	var inv bool
	err := t.row(`SELECT v, invalid FROM _lattis_hand_kept WHERE owner = ? AND k = ? AND h = ?`, owner, k, h).Scan(&old, &inv)
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	del := st(`DELETE FROM _lattis_hand_kept WHERE owner = ? AND k = ? AND h = ?`, owner, k, h)
	ins := func(v any, inv bool) stmt {
		return st(`INSERT INTO _lattis_hand_kept (owner, k, h, v, invalid) VALUES (?, ?, ?, ?, ?)`, owner, k, h, v, inv)
	}
	switch {
	case raw == "" && !exists:
		return nil
	case raw == "":
		return t.apply([]stmt{del}, []stmt{ins(old, inv)})
	case exists:
		return t.apply([]stmt{del, ins(raw, false)}, []stmt{del, ins(old, inv)})
	}
	return t.apply([]stmt{ins(raw, false)}, []stmt{del})
}

// clearPivotHand efface (Suppr) les colonnes saisies des TCD de la feuille
// dans la zone : valeurs des lignes, formule si toute la colonne est prise ;
// un en-tête dans la zone retire sa colonne si plus rien n'y reste. Un TCD
// pris en entier est laissé à clearPivots (supprimé).
func (t *tx) clearPivotHand(sheet int64, zone Rect) error {
	places, err := t.pivotPlaces(sheet)
	if err != nil {
		return err
	}
	for _, p := range places {
		nh := len(p.cfg.Hand)
		if nh == 0 || zone.R0 > p.bottom() || zone.R1 < p.top || zone.C0 > p.right() || zone.C1 < p.handCol() {
			continue
		}
		if zone.has(p.row, p.col) && zone.has(p.bottom(), p.right()) {
			continue
		}
		if err := t.guardObject(`_lattis_pivots`, p.id); err != nil {
			return err
		}
		owner := pivotOwner(p.id)
		keys := p.keys()
		vals, err := t.pivotHandVals(owner)
		if err != nil {
			return err
		}
		hand := slices.Clone(p.cfg.Hand)
		var drop []int
		changed := false
		for j := max(0, zone.C0-p.handCol()); j < nh && p.handCol()+j <= zone.C1; j++ {
			h := &hand[j]
			for r := max(zone.R0, p.top+1); r <= min(zone.R1, p.bottom()); r++ {
				if k := keys[r-p.top]; k != "" {
					if _, ok := vals[k][h.H]; ok {
						if err := t.putPivotHand(owner, k, h.H, ""); err != nil {
							return err
						}
						delete(vals[k], h.H)
					}
				}
			}
			all := zone.R0 <= p.top+1 && zone.R1 >= p.bottom()
			if h.Formula != "" && all {
				h.Formula = ""
				changed = true
			}
			if zone.R0 > p.top {
				continue
			}
			// En-tête dans la zone : la colonne part si plus rien ne s'y voit.
			left := h.Formula != ""
			for i := 1; i < len(keys) && !left; i++ {
				if k := keys[i]; k != "" {
					_, left = vals[k][h.H]
				}
			}
			if left {
				t.notes = append(t.notes, fmt.Sprintf("L'en-tête « %s » ne s'efface pas : la colonne contient des valeurs (sélectionnez toute la colonne puis Suppr pour la retirer)", clip(h.Name)))
				continue
			}
			drop = append(drop, j)
		}
		for _, j := range slices.Backward(drop) {
			if err := t.forgetKept(owner, `h = ?`, hand[j].H); err != nil {
				return err
			}
			hand = slices.Delete(hand, j, j+1)
			changed = true
		}
		if changed {
			if err := t.savePivotHand(p.id, hand); err != nil {
				return err
			}
		}
	}
	return nil
}

// forgetPivotHand oublie toutes les valeurs saisies du TCD id (supprimé,
// avec sa feuille ou non) ; annulable.
func (t *tx) forgetPivotHand(id int64) error {
	return t.forgetKept(pivotOwner(id), `1`)
}

// forgetSheetPivotsHand : forgetPivotHand pour chaque TCD de la feuille.
func (t *tx) forgetSheetPivotsHand(sheet int64) error {
	rs, err := t.query(`SELECT id FROM _lattis_pivots WHERE sheet = ?`, sheet)
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
	if err := rs.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := t.forgetPivotHand(id); err != nil {
			return err
		}
	}
	return nil
}

// forgetPivotKept oublie les valeurs mises de côté d'un TCD (étiquettes
// absentes) ; rend le nombre de lignes oubliées.
func (t *tx) forgetPivotKept(id int64, cfg PivotConfig) (int, error) {
	owner := pivotOwner(id)
	keys, err := t.pivotStoredKeys(owner)
	if err != nil || len(keys) == 0 {
		return 0, err
	}
	live, err := t.pivotLiveKeys(cfg)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, k := range keys {
		if live[k] {
			continue
		}
		if err := t.forgetKept(owner, `k = ?`, k); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// pivotHandByName : le TCD de ce nom (toutes feuilles), ok faux sinon.
func (t *tx) pivotHandByName(name string) (pivotRow, bool, error) {
	ps, err := t.allPivots()
	if err != nil {
		return pivotRow{}, false, err
	}
	for _, p := range ps {
		if strings.EqualFold(p.Cfg.Name, name) {
			return p, true, nil
		}
	}
	return pivotRow{}, false, nil
}

// pivotHandDeps : cellules lues par les formules des colonnes saisies des
// TCD (clés formula.CellKey), pour le calcul des cellules libres.
func (t *tx) pivotHandDeps() ([]string, error) {
	rs, err := t.query(`SELECT sheet, config FROM _lattis_pivots WHERE config LIKE '%"hand"%'`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var sheet int64
		var raw string
		if err := rs.Scan(&sheet, &raw); err != nil {
			return nil, err
		}
		var cfg PivotConfig
		if json.Unmarshal([]byte(raw), &cfg) != nil {
			continue
		}
		for _, h := range cfg.Hand {
			if h.Formula == "" {
				continue
			}
			if n, err := formula.Parse(h.Formula, formula.EN); err == nil {
				deps, _ := formula.Deps(formula.QualifyCells(n, sheet))
				out = append(out, deps...)
			}
		}
	}
	return out, rs.Err()
}

// pivotQueryHand est pivotQuery (vue SQL du TCD) avec ses colonnes saisies
// à droite, jointes par les étiquettes des lignes.
func (t *tx) pivotQueryHand(id, sheet int64, cfg PivotConfig) (string, []string, int, error) {
	body, err := t.pivotBody(cfg)
	if err != nil || len(cfg.Hand) == 0 || len(body) == 0 {
		if err != nil {
			return "", nil, 0, err
		}
		return pivotValuesQuery(body)
	}
	cells, rows, err := t.pivotCells(cfg)
	if err != nil {
		return "", nil, 0, err
	}
	var names map[string]formula.Value
	if schema, err := t.schema(); err == nil {
		names, _ = t.nameValues(schema)
	}
	extra, err := t.pivotHandCells(id, sheet, cfg, cells, rows, names)
	if err != nil {
		return "", nil, 0, err
	}
	keys := pivotRowKeys(cfg, cells, rows)
	byKey := map[string][]string{}
	for i, k := range keys {
		if k != "" {
			byKey[k] = extra[i]
		}
	}
	nk := len(cfg.RowFields)
	sig := pivotHandSig(cfg)
	out := make([][]string, len(body))
	out[0] = append(slices.Clip(body[0]), extra[0]...)
	for i, r := range body[1:] {
		var add []string
		switch {
		case nk == 0 && len(cells) > 1:
			add = extra[len(cells)-1] // la seule ligne : le total
		case len(r) >= nk:
			add = byKey[sig+strings.Join(r[:nk], pivotKeySep)]
		}
		if len(add) != len(cfg.Hand) {
			add = make([]string, len(cfg.Hand))
		}
		out[i+1] = append(slices.Clip(r), add...)
	}
	return pivotValuesQuery(out)
}

// pivotWidth : nombre de colonnes du TCD de configuration cfg posé sur la
// feuille, colonnes saisies comprises (0 : en erreur).
func (t *tx) pivotWidth(sheet int64, cfg PivotConfig) int {
	cfg = cfg.normalized()
	if err := t.pivotChoice(sheet, &cfg); err != nil {
		return 0
	}
	cells, _, err := t.pivotCells(cfg)
	if err != nil || len(cells) == 0 {
		return 0
	}
	return len(cells[0]) + len(cfg.Hand)
}

// copyPivotHand copie les valeurs saisies du TCD from vers sa copie to
// (annulable avec la copie).
func (t *tx) copyPivotHand(from, to int64) error {
	var n int
	if err := t.row(`SELECT count(*) FROM _lattis_hand_kept WHERE owner = ?`, pivotOwner(from)).Scan(&n); err != nil || n == 0 {
		return err
	}
	return t.apply(
		[]stmt{st(`INSERT INTO _lattis_hand_kept (owner, k, h, v, invalid) SELECT ?, k, h, v, invalid FROM _lattis_hand_kept WHERE owner = ?`, pivotOwner(to), pivotOwner(from))},
		[]stmt{st(`DELETE FROM _lattis_hand_kept WHERE owner = ?`, pivotOwner(to))},
	)
}
