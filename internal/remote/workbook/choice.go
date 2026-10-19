package workbook

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/store"
)

// Onglets à choix d'un classeur partagé (canvas/choice.go ; conception
// regrouper-onglets-semblables §6.3, lot 5). La colonne choice de
// _lattis_sheets porte la catégorie de la feuille (tableau, colonne, nom)
// et la valeur choisie (value, all, fresh). La catégorie est publiée avec
// la mise en page ; la valeur choisie reste propre à chaque poste :
//
//   - publication (dumpLayout) : la catégorie seule (publishedChoice) ;
//   - ouverture, nouvelle version (Load) : la catégorie du serveur avec la
//     valeur choisie du poste quand c'est la même catégorie (mergeChoice) ;
//     une catégorie que le serveur n'a pas changée depuis la dernière mise
//     en page reçue garde le réglage du poste ; sans valeur choisie,
//     l'onglet prend la dernière (canvas.SettleChoices, doc.ShareChanged) ;
//     une mise en page d'avant (sans la colonne) laisse le choix du poste ;
//   - classeur collaboratif publié : un concepteur qui pose, change ou retire
//     la catégorie d'une feuille la publie (PublishChoices : mise en page du
//     serveur relue, catégorie et cellules par valeur de cette feuille
//     remplacées, save_meta, « *meta ») ; les autres postes la reçoivent
//     (ApplyChoices). Sans droit de conception, elle reste sur le poste.

// choicePick est la colonne choice (même JSON que canvas.choiceRaw).
type choicePick struct {
	Tbl   int64  `json:"tbl,omitempty"`
	Col   int64  `json:"col,omitempty"`
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
	All   bool   `json:"all,omitempty"`
	Fresh bool   `json:"fresh,omitempty"`
}

// parseChoice lit une colonne choice (nil : aucune, ou illisible).
func parseChoice(v any) *choicePick {
	var s string
	switch x := v.(type) {
	case string:
		s = x
	case []byte:
		s = string(x)
	default:
		return nil
	}
	var p choicePick
	if s == "" || json.Unmarshal([]byte(s), &p) != nil || (p.Tbl == 0 && strings.TrimSpace(p.Name) == "") {
		return nil
	}
	return &p
}

// def rend la catégorie seule.
func (p *choicePick) def() *choicePick {
	if p == nil {
		return nil
	}
	return &choicePick{Tbl: p.Tbl, Col: p.Col, Name: p.Name}
}

// choiceText écrit la colonne choice (nil : NULL).
func choiceText(p *choicePick) any {
	if p == nil {
		return nil
	}
	b, _ := json.Marshal(p)
	return string(b)
}

// defText : la catégorie seule, pour comparer ("" : aucune).
func defText(p *choicePick) string {
	s, _ := choiceText(p.def()).(string)
	return s
}

// publishedChoice : la catégorie publiée (sans la valeur choisie) ; une
// colonne source d'un tableau privé n'est désignée que par son nom.
func publishedChoice(v any, published map[int64]bool) any {
	p := parseChoice(v).def()
	if p != nil && p.Tbl != 0 && published != nil && !published[p.Tbl] {
		p.Tbl, p.Col = 0, 0
	}
	return choiceText(p)
}

// sameCategory : même colonne source, ou même nom.
func sameCategory(a, b *choicePick) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.Tbl != 0 && a.Tbl == b.Tbl && a.Col == b.Col {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(a.Name), strings.TrimSpace(b.Name))
}

// looseSame : même catégorie, la colonne source n'étant comparée que si
// les deux la désignent (une source qu'on ne sait pas traduire d'un
// classeur à l'autre est désignée par son nom).
func looseSame(a, b *choicePick) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.Tbl != 0 && b.Tbl != 0 && (a.Tbl != b.Tbl || a.Col != b.Col) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(a.Name), strings.TrimSpace(b.Name))
}

// mergeChoice : la catégorie du serveur (nil : retirée), avec la valeur
// choisie du poste si c'est la même catégorie ; sinon sans valeur (la
// dernière, canvas.SettleChoices).
func mergeChoice(local, server *choicePick) *choicePick {
	if server == nil {
		return nil
	}
	out := server.def()
	if local != nil && sameCategory(local, server) {
		out.Value, out.All, out.Fresh = local.Value, local.All, local.Fresh
	}
	return out
}

// mapChoice traduit la colonne source d'une catégorie d'un classeur à
// l'autre (identifiants des tableaux) ; une source inconnue n'est plus
// désignée que par son nom.
func mapChoice(p *choicePick, ids map[int64]int64) *choicePick {
	p = p.def()
	if p != nil && p.Tbl != 0 {
		if to, ok := ids[p.Tbl]; ok {
			p.Tbl = to
		} else {
			p.Tbl, p.Col = 0, 0
		}
	}
	return p
}

// sheetRow trouve une feuille dans la table _lattis_sheets d'une mise en
// page : même identifiant et même nom, sinon même nom, sinon même
// identifiant (-1 : aucune).
func sheetRow(r *LayoutRows, id int64, name string) int {
	if r == nil {
		return -1
	}
	idAt, nAt := r.index("id"), r.index("name")
	if idAt < 0 || nAt < 0 {
		return -1
	}
	byID, byName := -1, -1
	for k, row := range r.Rows {
		rid, _ := asInt(row[idAt])
		n, _ := row[nAt].(string)
		same := strings.EqualFold(n, name)
		switch {
		case rid == id && same:
			return k
		case same && byName < 0:
			byName = k
		case rid == id:
			byID = k
		}
	}
	if byName >= 0 {
		return byName
	}
	return byID
}

// layoutChoice lit la catégorie de la ligne at de _lattis_sheets.
func layoutChoice(l *Layout, at int) *choicePick {
	r := l.Tables["_lattis_sheets"]
	if r == nil || at < 0 {
		return nil
	}
	if ci := r.index("choice"); ci >= 0 {
		return parseChoice(r.Rows[at][ci])
	}
	return nil
}

// setLayoutChoice met dans la mise en page la catégorie def de la feuille
// de la ligne at de _lattis_sheets et ses cellules par valeur (cells, dans
// l'ordre des colonnes cols, sous l'identifiant de cette feuille).
func setLayoutChoice(l *Layout, at int, def *choicePick, cols []string, cells [][]any) {
	r := l.Tables["_lattis_sheets"]
	ci := r.index("choice")
	if ci < 0 {
		r.Columns = append(r.Columns, "choice")
		for i := range r.Rows {
			r.Rows[i] = append(r.Rows[i], nil)
		}
		ci = len(r.Columns) - 1
	}
	r.Rows[at][ci] = choiceText(def.def())
	sid, _ := asInt(r.Rows[at][r.index("id")])
	if len(cols) == 0 {
		return
	}
	cc := l.Tables["_lattis_choice_cells"]
	if cc == nil {
		cc = &LayoutRows{Columns: slices.Clone(cols), Rows: [][]any{}}
		l.Tables["_lattis_choice_cells"] = cc
	}
	sAt := cc.index("sheet")
	if sAt < 0 {
		return
	}
	cc.Rows = slices.DeleteFunc(cc.Rows, func(row []any) bool {
		s, _ := asInt(row[sAt])
		return s == sid
	})
	src := &LayoutRows{Columns: cols}
	for _, row := range cells {
		p := projectRow(src, row, cc.Columns)
		p[sAt] = sid
		cc.Rows = append(cc.Rows, p)
	}
}

// encodeLayout écrit une mise en page (valeurs prêtes pour le JSON).
func encodeLayout(l *Layout) ([]byte, error) {
	for _, r := range l.Tables {
		for _, row := range r.Rows {
			for i, v := range row {
				row[i] = jsonValue(v)
			}
		}
	}
	return json.Marshal(l)
}

// localSheet est une feuille du classeur local et sa colonne choice.
type localSheet struct {
	id     int64
	name   string
	choice *choicePick
}

// readSheets lit les feuilles et leur choix (ok faux : format sans onglet
// à choix).
func readSheets(ctx context.Context, q txOrDB) ([]localSheet, bool, error) {
	cols, err := tableColumns(ctx, q, "_lattis_sheets")
	if err != nil || !slices.Contains(cols, "choice") {
		return nil, false, err
	}
	rs, err := q.QueryContext(ctx, `SELECT id, name, choice FROM _lattis_sheets ORDER BY id`)
	if err != nil {
		return nil, false, err
	}
	defer rs.Close()
	var out []localSheet
	for rs.Next() {
		var s localSheet
		var c sql.NullString
		if err := rs.Scan(&s.id, &s.name, &c); err != nil {
			return nil, false, err
		}
		if c.Valid {
			s.choice = parseChoice(c.String)
		}
		out = append(out, s)
	}
	return out, true, rs.Err()
}

// readChoiceCells lit les cellules par valeur d'une feuille (cols : nil
// pour un format sans elles).
func readChoiceCells(ctx context.Context, q txOrDB, sheet int64) ([]string, [][]any, error) {
	cols, err := tableColumns(ctx, q, "_lattis_choice_cells")
	if err != nil || len(cols) == 0 {
		return nil, nil, err
	}
	rs, err := q.QueryContext(ctx, `SELECT `+quoteCols(cols)+` FROM _lattis_choice_cells WHERE sheet = ? ORDER BY row, col, value`, sheet)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	var out [][]any
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		out = append(out, vals)
	}
	return cols, out, rs.Err()
}

// ChoiceSignature résume les catégories des onglets à choix du classeur
// local (sans les valeurs choisies) : elle change quand une catégorie est
// posée, changée ou retirée, pas quand on change d'année.
func ChoiceSignature(ctx context.Context, db *sql.DB) (string, error) {
	sheets, _, err := readSheets(ctx, db)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, s := range sheets {
		b.WriteString(s.name + "\x00" + defText(s.choice) + "\x00")
	}
	return b.String(), nil
}

// tableIDs relie les tableaux publiés du poste à leurs identifiants dans la
// mise en page du serveur (RemoteTable.LocalID) : toLayout et l'inverse.
func tableIDs(ctx context.Context, q txOrDB, tables []RemoteTable) (toLayout, toLocal map[int64]int64, err error) {
	toLayout, toLocal = map[int64]int64{}, map[int64]int64{}
	byID := map[string]int64{}
	for _, t := range tables {
		if t.LocalID != 0 {
			byID[t.ID] = t.LocalID
		}
	}
	rs, err := q.QueryContext(ctx, `SELECT tbl, remote_id FROM _lattis_remote_tables WHERE published = 1 AND remote_id IS NOT NULL`)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var tbl int64
		var rid string
		if err := rs.Scan(&tbl, &rid); err != nil {
			return nil, nil, err
		}
		if lid, ok := byID[rid]; ok {
			toLayout[tbl], toLocal[lid] = lid, tbl
		}
	}
	return toLayout, toLocal, rs.Err()
}

// PublishChoices publie les catégories d'onglets à choix posées, changées
// ou retirées sur le poste depuis la dernière mise en page reçue (classeur
// collaboratif publié) : la mise en page du serveur est relue, seules la
// catégorie de ces feuilles et leurs cellules par valeur y sont remplacées
// (save_meta, qui notifie « *meta »). Rend le nombre de feuilles publiées.
// Sans droit de conception, rien ne part : la catégorie reste sur le poste.
func PublishChoices(ctx context.Context, srv Server, db *store.Store, info *Info) (int, error) {
	if info == nil || info.Mode != Collaborative || !info.CanDesign() {
		return 0, nil
	}
	var tables []RemoteTable
	err := srv.Delta(ctx, func(r Reader) error {
		var err error
		tables, err = r.Tables(ctx, info.Schema)
		return err
	})
	if err != nil {
		return 0, err
	}
	n := 0
	err = db.Write(ctx, func(tx *sql.Tx) error {
		sheets, ok, err := readSheets(ctx, tx)
		if err != nil || !ok {
			return err
		}
		baseRaw, err := readBase(ctx, tx)
		if err != nil || baseRaw == nil {
			return err
		}
		base, err := decodeLayout(baseRaw)
		if err != nil {
			return err
		}
		toLayout, _, err := tableIDs(ctx, tx, tables)
		if err != nil {
			return err
		}
		type change struct {
			sheet localSheet
			def   *choicePick
			at    int
			cols  []string
			cells [][]any
		}
		var changes []change
		for _, s := range sheets {
			at := sheetRow(base.Tables["_lattis_sheets"], s.id, s.name)
			if at < 0 {
				continue // feuille du poste, pas dans le classeur partagé
			}
			def := mapChoice(s.choice, toLayout)
			if looseSame(def, layoutChoice(base, at)) {
				continue
			}
			c := change{sheet: s, def: def, at: at}
			if c.cols, c.cells, err = readChoiceCells(ctx, tx, s.id); err != nil {
				return err
			}
			changes = append(changes, c)
		}
		if len(changes) == 0 {
			return nil
		}
		err = srv.Write(ctx, func(w Writer) error {
			raw, err := w.Meta(ctx, info.Schema, layoutKind, layoutKey)
			if err != nil {
				return err
			}
			l := &Layout{Key: layoutKey, Format: base.Format, Tables: map[string]*LayoutRows{}}
			if raw != nil {
				if l, err = decodeLayout(raw); err != nil {
					return err
				}
			}
			put := 0
			for _, c := range changes {
				at := sheetRow(l.Tables["_lattis_sheets"], c.sheet.id, c.sheet.name)
				if at < 0 {
					continue // feuille retirée du serveur entre-temps
				}
				setLayoutChoice(l, at, c.def, c.cols, c.cells)
				put++
			}
			if put == 0 {
				return nil
			}
			out, err := encodeLayout(l)
			if err != nil {
				return err
			}
			return w.SaveMeta(ctx, info.WorkbookID, layoutKind, out)
		})
		if err != nil {
			return err
		}
		// La mise en page reçue suit : ces catégories ne repartent pas.
		for _, c := range changes {
			setLayoutChoice(base, c.at, c.def, c.cols, c.cells)
		}
		out, err := encodeLayout(base)
		if err != nil {
			return err
		}
		n = len(changes)
		return writeBase(ctx, tx, out)
	})
	return n, err
}

// Choices est ce qu'il faut lire du serveur pour recevoir les catégories
// d'onglets à choix : la mise en page et les tableaux.
type Choices struct {
	Layout []byte
	Tables []RemoteTable
}

// ReadChoices lit, dans la lecture r, la mise en page et les tableaux du
// classeur.
func ReadChoices(ctx context.Context, r Reader, schema string) (*Choices, error) {
	tables, err := r.Tables(ctx, schema)
	if err != nil {
		return nil, err
	}
	raw, err := r.Meta(ctx, schema, layoutKind, layoutKey)
	if err != nil {
		return nil, err
	}
	return &Choices{Layout: raw, Tables: tables}, nil
}

// ApplyChoices applique en local, hors du journal d'annulation, les
// catégories d'onglets à choix que le serveur a changées depuis la dernière
// mise en page reçue (un concepteur les a publiées) : la valeur choisie du
// poste est gardée pour une même catégorie, les cellules par valeur de la
// feuille sont celles du serveur. Une catégorie que le serveur n'a pas
// changée reste celle du poste ; une mise en page sans catégories (format
// d'avant) ne change rien. Rend vrai si une feuille a changé (les valeurs
// sans choix sont à régler : canvas.SettleChoices).
func ApplyChoices(ctx context.Context, tx *sql.Tx, c *Choices) (bool, error) {
	if c == nil || c.Layout == nil {
		return false, nil
	}
	l, err := decodeLayout(c.Layout)
	if err != nil {
		return false, err
	}
	ss := l.Tables["_lattis_sheets"]
	if ss == nil || ss.index("choice") < 0 || ss.index("id") < 0 || ss.index("name") < 0 {
		return false, nil
	}
	sheets, ok, err := readSheets(ctx, tx)
	if err != nil || !ok {
		return false, err
	}
	baseRaw, err := readBase(ctx, tx)
	if err != nil {
		return false, err
	}
	base := &Layout{Key: layoutKey, Format: l.Format, Tables: map[string]*LayoutRows{}}
	if baseRaw != nil {
		if base, err = decodeLayout(baseRaw); err != nil {
			return false, err
		}
	}
	_, toLocal, err := tableIDs(ctx, tx, c.Tables)
	if err != nil {
		return false, err
	}
	cellCols, err := tableColumns(ctx, tx, "_lattis_choice_cells")
	if err != nil {
		return false, err
	}
	var srvCols []string
	var srvCells [][]any
	if cc := l.Tables["_lattis_choice_cells"]; cc != nil {
		srvCols, srvCells = cc.Columns, cc.Rows
	}
	cellsOf := func(sid int64) [][]any {
		at := slices.Index(srvCols, "sheet")
		var out [][]any
		if at < 0 {
			return nil
		}
		for _, row := range srvCells {
			if s, _ := asInt(row[at]); s == sid {
				out = append(out, slices.Clone(row))
			}
		}
		return out
	}
	changed := false
	for k, row := range ss.Rows {
		sid, _ := asInt(row[ss.index("id")])
		name, _ := row[ss.index("name")].(string)
		sd := layoutChoice(l, k)
		bat := sheetRow(base.Tables["_lattis_sheets"], sid, name)
		if defText(sd) == defText(layoutChoice(base, bat)) {
			continue // inchangée sur le serveur : celle du poste
		}
		li := slices.IndexFunc(sheets, func(s localSheet) bool { return s.id == sid && strings.EqualFold(s.name, name) })
		if li < 0 {
			li = slices.IndexFunc(sheets, func(s localSheet) bool { return strings.EqualFold(s.name, name) })
		}
		if li < 0 {
			continue // feuille pas encore reçue
		}
		local := sheets[li]
		v := choiceText(mergeChoice(local.choice, mapChoice(sd, toLocal)))
		if _, err := tx.ExecContext(ctx, `UPDATE _lattis_sheets SET choice = ? WHERE id = ?`, v, local.id); err != nil {
			return false, err
		}
		cells := cellsOf(sid)
		if len(cellCols) > 0 && slices.Contains(srvCols, "sheet") {
			if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_choice_cells WHERE sheet = ?`, local.id); err != nil {
				return false, err
			}
			at := slices.Index(srvCols, "sheet")
			for _, r := range cells {
				r[at] = local.id
			}
			if err := insertRows(ctx, tx, "_lattis_choice_cells", srvCols, cells, true); err != nil {
				return false, err
			}
		}
		if bat >= 0 {
			setLayoutChoice(base, bat, sd, srvCols, cellsOf(sid))
		}
		changed = true
	}
	if !changed {
		return false, nil
	}
	if err := canvas.ShowChoiceCells(ctx, tx); err != nil {
		return false, err
	}
	if baseRaw != nil {
		out, err := encodeLayout(base)
		if err != nil {
			return false, err
		}
		if err := writeBase(ctx, tx, out); err != nil {
			return false, err
		}
	}
	return true, nil
}
