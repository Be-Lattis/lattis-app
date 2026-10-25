package canvas

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Mise en forme d'un tableau croisé, par colonne, comme un tableau (demande
// de Louis : « si je fais une mise en forme [dans un TCD], elle s'affiche que
// sur une cellule. Elle ne prend pas la colonne »).
//
// Une mise en forme posée sur une ou plusieurs cellules d'un TCD vaut pour
// toute leur colonne : ses lignes (détail, groupes, sous-totaux, total), ou
// son en-tête si la sélection le touche. Elle est rangée dans la
// configuration du TCD (PivotConfig.Styles), par colonne désignée par ce
// qu'elle montre, pas par sa position : un champ des lignes (« r:ville »),
// une valeur (« v:salaire|sum| », toutes ses colonnes quand un champ est en
// Colonnes, comme le format d'un champ de valeur d'Excel), une colonne saisie
// (« h:2 »). Elle suit donc la colonne quand le TCD est actualisé, trié,
// filtré, s'élargit ou rétrécit, et revient avec un champ remis en place.
//
// Couches, de la moins à la plus précise : format par défaut de la valeur
// (pivotFormats), colonne puis ligne de la feuille, colonne du TCD, style de
// cellule (positionnel : les anciennes mises en forme, posées cellule par
// cellule avant ce changement). Mettre en forme une colonne retire ces
// mêmes propriétés des styles de cellule de la colonne (conversion à la
// première modification, comme une colonne de tableau). Les lignes de
// sous-total et de total prennent le style de la colonne, plus leur gras
// et leur fond habituels (un remplissage choisi l'emporte sur le fond).
//
// Seule la mise en forme proprement dite suit la colonne ; les bordures, les
// notes, la validation et les fusions restent attachées à la cellule.

// PivotColStyle est la mise en forme d'une colonne de TCD : ses lignes
// (Body) et son en-tête (Head).
type PivotColStyle struct {
	Head Style `json:"head,omitempty"`
	Body Style `json:"body,omitempty"`
}

// pivotColumnKeys : propriétés qui suivent la colonne d'un TCD.
var pivotColumnKeys = map[string]bool{
	"b": true, "i": true, "u": true, "x": true, "ff": true, "fs": true, "fg": true, "bg": true,
	"al": true, "va": true, "wr": true, "nf": true, "dp": true, "ng": true, "df": true, "nc": true,
}

// splitPivotPatch sépare un correctif entre ce qui va à la colonne du TCD et
// ce qui reste à la cellule.
func splitPivotPatch(patch Style) (col, cell Style) {
	col, cell = Style{}, Style{}
	for k, v := range patch {
		if pivotColumnKeys[k] {
			col[k] = v
		} else {
			cell[k] = v
		}
	}
	return col, cell
}

// pivotColKey désigne la colonne j d'un TCD affiché (base : nombre de
// colonnes calculées, sans les colonnes saisies).
func pivotColKey(cfg PivotConfig, base, j int) string {
	if j >= base {
		if h := j - base; h < len(cfg.Hand) {
			return "h:" + strconv.FormatInt(cfg.Hand[h].H, 10)
		}
		return ""
	}
	labels := max(1, len(cfg.RowFields))
	if cfg.compact() {
		labels = 1 // une seule colonne d'étiquettes (arborescence)
	}
	if j < labels {
		switch {
		case cfg.compact():
			return "r:*"
		case j < len(cfg.RowFields):
			return "r:" + strings.ToLower(strings.TrimSpace(cfg.RowFields[j]))
		}
		return "r:"
	}
	if len(cfg.Values) == 0 {
		return ""
	}
	// Les colonnes de mesure parcourent les valeurs dans l'ordre (par valeur
	// du champ des colonnes, puis les totaux), comme pivotFormats.
	v := cfg.Values[(j-labels)%len(cfg.Values)]
	return "v:" + strings.ToLower(strings.TrimSpace(v.Field)) + "|" + v.Agg + "|" + v.Show
}

// pivotStylesAttach donne à un TCD calculé (colonnes saisies comprises) la
// mise en forme de chacune de ses colonnes.
func pivotStylesAttach(p *PivotInfo, hand []PivotHandCol) {
	if p.Error != "" || len(p.Cells) == 0 || len(p.Config.Styles) == 0 {
		return
	}
	cfg := p.Config
	cfg.Hand = hand
	w := len(p.Cells[0])
	base := w - len(hand)
	body, head := make([]string, w), make([]string, w)
	found := false
	for j := range w {
		s, ok := cfg.Styles[pivotColKey(cfg, base, j)]
		if !ok {
			continue
		}
		body[j], _ = encodeStyle(s.Body)
		head[j], _ = encodeStyle(s.Head)
		found = true
	}
	if found {
		p.Styles, p.HeadStyles = body, head
	}
}

// pivotStyleEdit : colonnes d'un TCD touchées par une mise en forme.
type pivotStyleEdit struct {
	p     pivotPlace
	head  map[int]bool // colonne de la feuille → en-tête touché
	body  map[int]bool // colonne de la feuille → lignes touchées
	cells int
}

// pivotPlaceAt : le TCD de places dont le cadre (en-tête, lignes, colonnes
// saisies ; pas les filtres de rapport au-dessus) contient (r, c).
func pivotPlaceAt(places []pivotPlace, r, c int) int {
	for i, p := range places {
		if r >= p.top && r <= p.bottom() && c >= p.col && c <= p.right() {
			return i
		}
	}
	return -1
}

// stylePivotColumns pose la mise en forme col (déjà réduite aux propriétés
// de colonne) sur les colonnes touchées d'un TCD : configuration du TCD
// (annulable), et les mêmes propriétés retirées des styles de cellule de
// ces colonnes.
func (t *tx) stylePivotColumns(sheet int64, e *pivotStyleEdit, col Style, lines sheetLines) error {
	if len(col) == 0 {
		return nil
	}
	p := e.p
	var old string
	if err := t.row(`SELECT config FROM _lattis_pivots WHERE id = ?`, p.id).Scan(&old); err != nil {
		return err
	}
	m := map[string]any{}
	if err := json.Unmarshal([]byte(old), &m); err != nil {
		return err
	}
	styles := map[string]PivotColStyle{}
	if raw, ok := m["styles"]; ok {
		b, _ := json.Marshal(raw)
		_ = json.Unmarshal(b, &styles)
	}
	var formats []string
	if len(e.body) > 0 {
		formats, _ = t.pivotFormats(p.cfg, p.cells)
	}
	base := p.base()
	apply := func(cur Style, inh Style) (Style, error) {
		enc, err := encodeStyle(cur)
		if err != nil {
			return nil, err
		}
		enc, err = merge(enc, col, inh)
		if err != nil {
			return nil, err
		}
		return parseStyle(enc), nil
	}
	for c := p.col; c <= p.right(); c++ {
		if !e.head[c] && !e.body[c] {
			continue
		}
		j := c - p.col
		key := pivotColKey(p.cfg, base, j)
		if key == "" {
			continue
		}
		s := styles[key]
		var err error
		if e.head[c] {
			if s.Head, err = apply(s.Head, lines.at(p.top, c)); err != nil {
				return err
			}
			if err := t.stripPivotCells(sheet, p.top, p.top, c, col); err != nil {
				return err
			}
		}
		if e.body[c] {
			inh := over(lines.cols[c], lines.rowsIn(p.top+1, p.bottom()))
			if j < len(formats) && formats[j] != "" {
				inh = over(parseStyle(formats[j]), inh)
			}
			if s.Body, err = apply(s.Body, inh); err != nil {
				return err
			}
			if err := t.stripPivotCells(sheet, p.top+1, p.bottom(), c, col); err != nil {
				return err
			}
		}
		if len(s.Head) == 0 && len(s.Body) == 0 {
			delete(styles, key)
		} else {
			styles[key] = s
		}
	}
	if len(styles) == 0 {
		delete(m, "styles")
	} else {
		m["styles"] = styles
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if string(b) == old {
		return nil
	}
	q := `UPDATE _lattis_pivots SET config = ? WHERE id = ?`
	return t.apply([]stmt{st(q, string(b), p.id)}, []stmt{st(q, old, p.id)})
}

// stripPivotCells retire les propriétés de patch des styles de cellule
// (positionnels) des lignes [r0, r1] de la colonne c.
func (t *tx) stripPivotCells(sheet int64, r0, r1, c int, patch Style) error {
	rs, err := t.query(`SELECT row FROM _lattis_cell_styles WHERE sheet = ? AND col = ? AND row BETWEEN ? AND ?`, sheet, c, r0, r1)
	if err != nil {
		return err
	}
	var rows []int
	for rs.Next() {
		var r int
		if err := rs.Scan(&r); err != nil {
			rs.Close()
			return err
		}
		rows = append(rows, r)
	}
	rs.Close()
	for _, r := range rows {
		err := t.setStyle(`_lattis_cell_styles`, `sheet = ? AND row = ? AND col = ?`, `sheet, row, col`, []any{sheet, r, c},
			func(old string) (string, error) { return strip(old, patch) })
		if err != nil {
			return err
		}
	}
	return nil
}

// stripPivotColumn retire les propriétés de patch de la mise en forme des
// colonnes de TCD que traverse la colonne c de la feuille (une colonne de la
// feuille mise en forme l'emporte, comme sur un tableau).
func (t *tx) stripPivotColumn(places []pivotPlace, c int, patch Style) error {
	for _, p := range places {
		if c < p.col || c > p.right() {
			continue
		}
		key := pivotColKey(p.cfg, p.base(), c-p.col)
		if key == "" {
			continue
		}
		var old string
		if err := t.row(`SELECT config FROM _lattis_pivots WHERE id = ?`, p.id).Scan(&old); err != nil {
			return err
		}
		var cfg struct {
			Styles map[string]PivotColStyle `json:"styles"`
		}
		if json.Unmarshal([]byte(old), &cfg) != nil || cfg.Styles == nil {
			continue
		}
		s, ok := cfg.Styles[key]
		if !ok {
			continue
		}
		for k := range patch {
			delete(s.Head, k)
			delete(s.Body, k)
		}
		m := map[string]any{}
		if err := json.Unmarshal([]byte(old), &m); err != nil {
			return err
		}
		if len(s.Head) == 0 && len(s.Body) == 0 {
			delete(cfg.Styles, key)
		} else {
			cfg.Styles[key] = s
		}
		if len(cfg.Styles) == 0 {
			delete(m, "styles")
		} else {
			m["styles"] = cfg.Styles
		}
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		if string(b) == old {
			continue
		}
		q := `UPDATE _lattis_pivots SET config = ? WHERE id = ?`
		if err := t.apply([]stmt{st(q, string(b), p.id)}, []stmt{st(q, old, p.id)}); err != nil {
			return err
		}
	}
	return nil
}

// stylePivotPlaces : les TCD de la feuille (aucun calcul si elle n'en a pas).
func (t *tx) stylePivotPlaces(sheet int64) ([]pivotPlace, error) {
	var n int
	if err := t.row(`SELECT count(*) FROM _lattis_pivots WHERE sheet = ?`, sheet).Scan(&n); err != nil || n == 0 {
		return nil, err
	}
	return t.pivotPlaces(sheet)
}
