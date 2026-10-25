package workbook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/store"
)

// Mise en page en direct d'un classeur collaboratif publié
// (docs/collaboration/synchronisation.md §2.14, décision 0018) : cellules
// libres, mises en forme, largeurs et hauteurs, fusions, règles, noms,
// graphiques, boutons, tableaux croisés, pages, feuilles, place des
// tableaux, cellules par valeur, thème. La mise en page reste un seul
// objet du serveur (save_meta(id, 'layout', …)) ; chaque poste la fusionne
// ligne à ligne, en trois temps :
//
//   - base : la dernière mise en page reçue (remote_base), ce sur quoi le
//     poste s'est accordé avec le serveur ;
//   - mine : la mise en page du poste (dumpLayout, dans les identifiants de
//     la mise en page) ;
//   - theirs : celle du serveur, relue dans la transaction qui l'écrit.
//
// Chaque ligne est désignée par sa clé naturelle (layoutKeys) ; champ par
// champ, ce qu'un seul côté a changé est pris, et sur un même champ changé
// des deux côtés la dernière écriture (le poste qui envoie) l'emporte, avec
// un conflit compté. Seuls les propriétaires et concepteurs envoient (ils
// sont aussi les seuls à pouvoir la modifier : Link.LayoutLock) ; les
// autres reçoivent, le serveur faisant foi.

// LayoutTables : tables de la mise en page publiée (Store.Watch : une
// écriture de l'une d'elles est une mise en page à envoyer).
func LayoutTables() []string { return slices.Clone(layoutTables) }

// layoutKeys : clé naturelle des lignes de chaque table de la mise en page.
var layoutKeys = map[string][]string{
	"_lattis_sheets":       {"id"},
	"_lattis_tables":       {"id"},
	"_lattis_columns":      {"tbl", "id"},
	"_lattis_cells":        {"sheet", "row", "col"},
	"_lattis_choice_cells": {"sheet", "row", "col", "value"},
	"_lattis_col_widths":   {"sheet", "col"},
	"_lattis_row_heights":  {"sheet", "row"},
	"_lattis_names":        {"name"}, // sans la casse (COLLATE NOCASE)
	"_lattis_styles":       {"tbl", "rid", "col"},
	"_lattis_cell_styles":  {"sheet", "row", "col"},
	"_lattis_row_styles":   {"sheet", "row"},
	"_lattis_col_styles":   {"sheet", "col"},
	"_lattis_row_rules":    {"sheet", "row"},
	"_lattis_col_rules":    {"sheet", "col"},
	"_lattis_pivots":       {"id"},
	"_lattis_charts":       {"id"},
	"_lattis_buttons":      {"id"},
	"_lattis_pages":        {"sheet"},
	"_lattis_range_cells":  {"sheet", "row", "col"},
	"_lattis_page_setup":   {"sheet"},
	"_lattis_images":       {"hash"},
	"_lattis_objects":      {"id"},
	"_lattis_sparklines":   {"id"},
	"_lattis_row_outline":  {"sheet", "row"},
	"_lattis_col_outline":  {"sheet", "col"},
}

// layoutOwn : colonnes que la mise en page en direct ne compare ni
// n'applique : propres à chaque poste (zoom, filtre, « Ne plus proposer »
// la clé), suivies par les onglets à choix (choice : catégorie, choice.go)
// ou par la structure (nom et clé d'un tableau, §2.9). Envoyées, elles
// gardent la valeur du serveur.
var layoutOwn = map[string]map[string]bool{
	"_lattis_sheets": {"choice": true, "zoom": true},
	"_lattis_tables": {"name": true, "key_col": true, "filter": true, "no_key_hint": true},
	// Plan : groupe réduit ou ouvert, comme le filtre (canvas/outline.go).
	"_lattis_row_outline": {"folded": true},
	"_lattis_col_outline": {"folded": true},
}

// dataColumnOwn : colonnes d'une colonne saisie d'un tableau publié suivies
// par la structure (nom, type, type figé) ; une colonne calculée n'existe
// que dans la mise en page.
var dataColumnOwn = map[string]bool{"name": true, "type": true, "fixed": true}

// surrogates : tables dont l'identifiant est donné par le poste (le plus
// grand + 1) : deux concepteurs peuvent ajouter au même moment deux
// feuilles, graphiques… différents sous le même identifiant (separate).
var surrogates = map[string]string{"_lattis_sheets": "sheet", "_lattis_pivots": "pivot", "_lattis_charts": "chart", "_lattis_buttons": "button", "_lattis_objects": "object", "_lattis_sparklines": "sparkline"}

// --- Vue d'une mise en page : lignes par clé ------------------------------------

type lrow map[string]any

type ltable struct {
	cols []string
	rows map[string]lrow
}

type lview struct {
	format int
	tables map[string]*ltable
	theme  *canvas.Theme
	// extra : tables inconnues de ce poste (format plus récent), gardées.
	extra map[string]*LayoutRows
}

func newView(format int) *lview {
	return &lview{format: format, tables: map[string]*ltable{}, extra: map[string]*LayoutRows{}}
}

// viewOf range une mise en page par clé.
func viewOf(l *Layout) *lview {
	v := newView(l.Format)
	v.theme = l.Theme
	for name, r := range l.Tables {
		if _, ok := layoutKeys[name]; !ok {
			v.extra[name] = r
			continue
		}
		t := &ltable{cols: slices.Clone(r.Columns), rows: map[string]lrow{}}
		for _, row := range r.Rows {
			m := lrow{}
			for i, c := range r.Columns {
				if i < len(row) {
					m[c] = row[i]
				}
			}
			t.rows[keyOf(name, m)] = m
		}
		v.tables[name] = t
	}
	return v
}

// layout rend la mise en page d'une vue (lignes dans l'ordre des clés).
func (v *lview) layout() *Layout {
	l := &Layout{Key: layoutKey, Format: v.format, Tables: map[string]*LayoutRows{}, Theme: v.theme}
	for name, t := range v.tables {
		lr := &LayoutRows{Columns: slices.Clone(t.cols), Rows: [][]any{}}
		for _, k := range slices.Sorted(maps.Keys(t.rows)) {
			r := t.rows[k]
			row := make([]any, len(t.cols))
			for i, c := range t.cols {
				row[i] = r[c]
			}
			lr.Rows = append(lr.Rows, row)
		}
		l.Tables[name] = lr
	}
	for name, r := range v.extra {
		l.Tables[name] = r
	}
	return l
}

func (v *lview) row(name, key string) lrow {
	if t := v.tables[name]; t != nil {
		return t.rows[key]
	}
	return nil
}

// keyOf : clé naturelle d'une ligne.
func keyOf(name string, r lrow) string {
	var b strings.Builder
	for _, k := range layoutKeys[name] {
		switch x := r[k].(type) {
		case string:
			if k == "name" {
				x = strings.ToLower(x)
			}
			b.WriteString("s" + x)
		default:
			if n, ok := asInt(x); ok {
				b.WriteString("i" + strconv.FormatInt(n, 10))
			} else {
				b.WriteString("x" + enc(x))
			}
		}
		b.WriteByte(0)
	}
	return b.String()
}

func enc(v any) string {
	b, _ := json.Marshal(jsonValue(v))
	return string(b)
}

// same compare deux valeurs telles que relues du JSON (3 et 3.0 égaux).
func same(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	switch x := a.(type) {
	case string:
		if y, ok := b.(string); ok {
			return x == y
		}
	case int64:
		if y, ok := b.(int64); ok {
			return x == y
		}
	}
	return enc(a) == enc(b)
}

func isCalc(r lrow) bool {
	f, _ := r["formula"].(string)
	return strings.TrimSpace(f) != ""
}

// ownOf : colonnes non comparées des lignes d'une table.
func ownOf(name string, rows ...lrow) map[string]bool {
	if name != "_lattis_columns" {
		return layoutOwn[name]
	}
	for _, r := range rows {
		if r != nil && isCalc(r) {
			return nil
		}
	}
	return dataColumnOwn
}

func rowsEqual(a, b lrow, own map[string]bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	for c, v := range a {
		if !own[c] && !same(v, b[c]) {
			return false
		}
	}
	for c, v := range b {
		if _, ok := a[c]; !ok && !own[c] && v != nil {
			return false
		}
	}
	return true
}

// merge3 fusionne une ligne : base b, poste m, serveur t (nil : absente).
// Une ligne supprimée d'un côté et modifiée de l'autre : le poste l'emporte.
func merge3(b, m, t lrow, own map[string]bool, serverWins bool) (lrow, bool) {
	switch {
	case m == nil && t == nil:
		return nil, false
	case rowsEqual(m, t, own):
		return t, false
	case b == nil && m == nil:
		return t, false
	case b == nil && t == nil:
		return m, false
	case m == nil:
		if serverWins && !rowsEqual(t, b, own) {
			return t, true
		}
		return nil, !rowsEqual(t, b, own)
	case t == nil:
		if rowsEqual(m, b, own) {
			return nil, false
		}
		if serverWins {
			return nil, true
		}
		return m, true
	}
	return fieldMerge(b, m, t, own, serverWins)
}

// fieldMerge fusionne champ par champ deux versions d'une même ligne ; un
// champ changé des deux côtés prend la valeur du poste, ou du serveur si
// serverWins (retour d'une longue coupure).
func fieldMerge(b, m, t lrow, own map[string]bool, serverWins bool) (lrow, bool) {
	out := lrow{}
	conflict := false
	cols := map[string]bool{}
	for _, r := range []lrow{b, m, t} {
		for c := range r {
			cols[c] = true
		}
	}
	for c := range cols {
		if own[c] {
			if v, ok := t[c]; ok {
				out[c] = v
			} else {
				out[c] = m[c]
			}
			continue
		}
		mv, tv := m[c], t[c]
		switch {
		case same(mv, tv):
			out[c] = mv
		case b != nil && same(mv, b[c]):
			out[c] = tv
		case b != nil && same(tv, b[c]):
			out[c] = mv
		default:
			out[c] = mv
			if serverWins {
				out[c] = tv
			}
			conflict = true
		}
	}
	return out, conflict
}

func unionCols(ts ...*ltable) []string {
	var out []string
	for _, t := range ts {
		if t == nil {
			continue
		}
		for _, c := range t.cols {
			if !slices.Contains(out, c) {
				out = append(out, c)
			}
		}
	}
	return out
}

// derived : cellules libres aux positions des cellules par valeur : elles
// montrent la valeur choisie de chaque poste (canvas.ShowChoiceCells), ni
// envoyées ni reçues (les cellules par valeur le sont).
func derived(vs ...*lview) map[string]bool {
	out := map[string]bool{}
	for _, v := range vs {
		if v == nil {
			continue
		}
		if t := v.tables["_lattis_choice_cells"]; t != nil {
			for _, r := range t.rows {
				out[keyOf("_lattis_cells", r)] = true
			}
		}
	}
	return out
}

// mergeViews fusionne trois versions d'une mise en page ; rend le nombre
// de conflits (champs changés des deux côtés, le poste l'emportant).
func mergeViews(b, m, t *lview) (*lview, int) {
	return mergeViewsAs(b, m, t, false)
}

// mergeViewsAs fusionne comme mergeViews ; serverWins : un champ changé des
// deux côtés prend la valeur du serveur (retour d'une longue coupure).
func mergeViewsAs(b, m, t *lview, serverWins bool) (*lview, int) {
	out := newView(t.format)
	out.extra = t.extra
	n := 0
	skip := derived(b, m, t)
	for _, name := range layoutTables {
		tb, tm, tt := b.tables[name], m.tables[name], t.tables[name]
		if tb == nil && tm == nil && tt == nil {
			continue
		}
		ot := &ltable{cols: unionCols(tt, tm, tb), rows: map[string]lrow{}}
		keys := map[string]bool{}
		for _, x := range []*ltable{tb, tm, tt} {
			if x != nil {
				for k := range x.rows {
					keys[k] = true
				}
			}
		}
		for k := range keys {
			br, mr, tr := b.row(name, k), m.row(name, k), t.row(name, k)
			if name == "_lattis_cells" && skip[k] && br != nil {
				mr = br // valeur choisie du poste : pas un changement
			}
			r, c := merge3(br, mr, tr, ownOf(name, br, mr, tr), serverWins)
			if r != nil {
				ot.rows[k] = r
			}
			if c {
				n++
			}
		}
		out.tables[name] = ot
	}
	tb, tm, tt := enc(b.theme), enc(m.theme), enc(t.theme)
	switch {
	case tm == tt, tm == tb:
		out.theme = t.theme
	case tt == tb:
		out.theme = m.theme
	default:
		out.theme = m.theme
		if serverWins {
			out.theme = t.theme
		}
		n++
	}
	return out, n
}

// differs dit si deux vues diffèrent (colonnes comparées, sans les
// cellules montrant la valeur choisie).
func differs(a, b *lview) bool {
	if enc(a.theme) != enc(b.theme) {
		return true
	}
	skip := derived(a, b)
	for _, name := range layoutTables {
		ta, tb := a.tables[name], b.tables[name]
		keys := map[string]bool{}
		for _, x := range []*ltable{ta, tb} {
			if x != nil {
				for k := range x.rows {
					keys[k] = true
				}
			}
		}
		for k := range keys {
			if name == "_lattis_cells" && skip[k] {
				continue
			}
			ra, rb := a.row(name, k), b.row(name, k)
			if !rowsEqual(ra, rb, ownOf(name, ra, rb)) {
				return true
			}
		}
	}
	return false
}

// keepBase : vue du poste qui ne change rien à la base (poste sans droit
// de conception : le serveur fait foi) ; ce qui n'est que sur le poste
// reste.
func keepBase(b, m *lview) *lview {
	out := newView(m.format)
	out.theme = b.theme
	for _, name := range layoutTables {
		tb, tm := b.tables[name], m.tables[name]
		if tb == nil && tm == nil {
			continue
		}
		ot := &ltable{cols: unionCols(tm, tb), rows: map[string]lrow{}}
		if tm != nil {
			for k, r := range tm.rows {
				if tb == nil || tb.rows[k] == nil {
					ot.rows[k] = r
				}
			}
		}
		if tb != nil {
			maps.Copy(ot.rows, tb.rows)
		}
		out.tables[name] = ot
	}
	return out
}

// --- Identifiants du poste et de la mise en page --------------------------------

// idsKey : correspondance des identifiants du poste avec ceux de la mise en
// page (_lattis_info, JSON) : "table" pour tous les tableaux publiés ;
// "sheet", "pivot", "chart", "button" pour ceux qui diffèrent (identité
// sinon).
const idsKey = "layout_ids"

type idMaps map[string]map[int64]int64

func readIDs(ctx context.Context, q queryer) (idMaps, error) {
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, idsKey).Scan(&raw)
	out := idMaps{}
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	if json.Unmarshal([]byte(raw), &out) != nil {
		return idMaps{}, nil
	}
	return out, nil
}

func writeIDs(ctx context.Context, tx *sql.Tx, m idMaps) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, idsKey, string(b))
	return err
}

// noteLayoutID note que l'identifiant local d'un tableau ("table") ou d'une
// feuille ("sheet") est lid dans la mise en page (AddTable,
// InstallArrivals).
func noteLayoutID(ctx context.Context, tx *sql.Tx, kind string, local, lid int64) error {
	m, err := readIDs(ctx, tx)
	if err != nil {
		return err
	}
	if kind != "table" && local == lid && m[kind] == nil {
		return nil
	}
	m.set(kind, local, lid)
	return writeIDs(ctx, tx, m)
}

func (m idMaps) set(kind string, local, lid int64) {
	if m[kind] == nil {
		m[kind] = map[int64]int64{}
	}
	m[kind][local] = lid
}

func (m idMaps) clone() idMaps {
	out := idMaps{}
	for k, v := range m {
		out[k] = maps.Clone(v)
	}
	return out
}

// toLayout : identifiant de la mise en page d'un objet du poste (ok faux :
// tableau non relié).
func (m idMaps) toLayout(kind string, local int64) (int64, bool) {
	if v, ok := m[kind][local]; ok {
		return v, true
	}
	return local, kind != "table"
}

// toLocal : identifiant du poste d'un objet de la mise en page (ok faux :
// inconnu, ou pris en local par un autre objet).
func (m idMaps) toLocal(kind string, lid int64) (int64, bool) {
	for l, v := range m[kind] {
		if v == lid {
			return l, true
		}
	}
	if _, taken := m[kind][lid]; taken || kind == "table" {
		return 0, false
	}
	return lid, true
}

// --- Mise en page du poste ------------------------------------------------------

// local est la mise en page du poste : vue dans les identifiants de la
// mise en page, et lignes telles qu'en local (orig), par la même clé.
type local struct {
	dump *Layout
	view *lview
	orig map[string]map[string]lrow
	// published : tableaux publiés du poste.
	published map[int64]bool
}

// readLocal lit la mise en page du poste (tableaux publiés seulement).
func readLocal(ctx context.Context, q txOrDB, ids idMaps) (*local, error) {
	pub := map[int64]bool{}
	rs, err := q.QueryContext(ctx, `SELECT tbl FROM _lattis_remote_tables WHERE published = 1 AND remote_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return nil, err
		}
		pub[id] = true
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	rids, err := readRowIDs(ctx, q, slices.Collect(maps.Keys(pub)))
	if err != nil {
		return nil, err
	}
	dump, err := dumpLayout(ctx, q, meta.Version, pub, rids)
	if err != nil {
		return nil, err
	}
	l := &local{dump: dump, published: pub}
	l.translate(ids)
	return l, nil
}

// translate (re)fait la vue dans les identifiants de la mise en page.
func (l *local) translate(ids idMaps) {
	v := newView(l.dump.Format)
	v.theme = l.dump.Theme
	l.orig = map[string]map[string]lrow{}
	toLayout := map[int64]int64{}
	for tbl := range l.published {
		if lid, ok := ids.toLayout("table", tbl); ok {
			toLayout[tbl] = lid
		}
	}
	for name, r := range l.dump.Tables {
		t := &ltable{cols: slices.Clone(r.Columns), rows: map[string]lrow{}}
		o := map[string]lrow{}
	rows:
		for _, row := range r.Rows {
			orig := lrow{}
			for i, c := range r.Columns {
				orig[c] = row[i]
			}
			m := maps.Clone(orig)
			for c, v := range orig {
				switch {
				case c == "tbl" || c == "id" && name == "_lattis_tables":
					n, _ := asInt(v)
					lid, ok := toLayout[n]
					if !ok {
						continue rows // tableau publié pas encore relié
					}
					m[c] = lid
				case c == "sheet" || c == "id" && surrogates[name] != "":
					kind := "sheet"
					if c == "id" {
						kind = surrogates[name]
					}
					if n, ok := asInt(v); ok {
						m[c], _ = ids.toLayout(kind, n)
					}
				case c == "choice" && name == "_lattis_sheets":
					m[c] = choiceText(mapChoice(parseChoice(v), toLayout))
				}
			}
			k := keyOf(name, m)
			t.rows[k] = m
			o[k] = orig
		}
		v.tables[name] = t
		l.orig[name] = o
	}
	l.view = v
}

// separate donne un nouvel identifiant de mise en page aux objets ajoutés
// sur le poste (feuille, graphique…) dont l'identifiant vient d'être pris
// sur le serveur par un autre objet (ajouté par un autre concepteur) ; rend
// vrai si la correspondance a changé (vue à refaire).
func separate(b, m, t *lview, l *local, ids idMaps) bool {
	changed := false
	for name, kind := range surrogates {
		tm, tt := m.tables[name], t.tables[name]
		if tm == nil || tt == nil {
			continue
		}
		for k, r := range tm.rows {
			if b.row(name, k) != nil || tt.rows[k] == nil || rowsEqual(r, tt.rows[k], ownOf(name, r)) {
				continue
			}
			o := l.orig[name][k]
			lo, ok := asInt(o["id"])
			if !ok {
				continue
			}
			next := int64(0)
			for _, v := range []*lview{b, m, t} {
				if x := v.tables[name]; x != nil {
					for _, row := range x.rows {
						if n, ok := asInt(row["id"]); ok {
							next = max(next, n)
						}
					}
				}
			}
			for lo2, lid := range ids[kind] {
				next = max(next, lid, lo2)
			}
			ids.set(kind, lo, next+1)
			changed = true
		}
	}
	return changed
}

// --- Synchronisation ------------------------------------------------------------

// LayoutSync décrit une synchronisation de la mise en page.
type LayoutSync struct {
	// Sent : mise en page envoyée (save_meta).
	Sent bool
	// Applied : lignes reçues appliquées en local ; Refused : lignes
	// reçues laissées (feuille d'un nom pris, tableau pas encore reçu,
	// place occupée…), Conflicts : champs changés des deux côtés (le poste
	// qui envoie l'emporte).
	Applied, Refused, Conflicts int
	// Forgot : historique d'annulation effacé (feuille supprimée, colonne
	// calculée ajoutée ou retirée par un collègue).
	Forgot bool
	// Gen : écritures de la mise en page couvertes (Store.WatchGen).
	Gen int64
	// ServerWon : synchronisation au retour d'une longue coupure, les
	// conflits ont gardé la version du serveur (SyncLayoutAs).
	ServerWon bool
	// Lost : le serveur a refusé la mise en page du poste (droits de
	// conception retirés, souvent pendant une coupure).
	Lost bool
}

// SyncLayout envoie et reçoit la mise en page d'un classeur collaboratif
// publié (voir le début du fichier). force : relire le serveur
// (connexion, « *meta ») ; sinon (modification locale), rien n'est lu ni
// envoyé tant que la mise en page du poste est celle reçue. Un refus du
// serveur (droits retirés) n'envoie rien, sans erreur.
func SyncLayout(ctx context.Context, srv Server, db *store.Store, info *Info, force bool) (LayoutSync, error) {
	return SyncLayoutAs(ctx, srv, db, info, force, false)
}

// SyncLayoutAs synchronise comme SyncLayout ; serverWins : retour d'une
// longue coupure, un élément changé des deux côtés garde la version du
// serveur (les changements récents des collègues) au lieu de celle du poste
// (choix de Louis, audit du mode en ligne, question 3).
func SyncLayoutAs(ctx context.Context, srv Server, db *store.Store, info *Info, force, serverWins bool) (LayoutSync, error) {
	var res LayoutSync
	res.ServerWon = serverWins
	res.Gen = db.WatchGen()
	if info == nil || info.Mode != Collaborative {
		return res, nil
	}
	design := info.CanDesign()
	if !force && !design {
		return res, nil
	}
	var mine *local
	var base *lview
	var ids idMaps
	err := readTx(ctx, db, func(tx *sql.Tx) error {
		var err error
		if ids, err = readIDs(ctx, tx); err != nil {
			return err
		}
		raw, err := readBase(ctx, tx)
		if err != nil || raw == nil {
			return err
		}
		b, err := decodeLayout(raw)
		if err != nil {
			return err
		}
		base = viewOf(b)
		mine, err = readLocal(ctx, tx, ids)
		return err
	})
	if err != nil || base == nil {
		return res, err
	}
	unmapped := false
	for tbl := range mine.published {
		if _, ok := ids["table"][tbl]; !ok {
			unmapped = true
		}
	}
	if !force && !unmapped && !differs(mine.view, base) {
		return res, nil
	}
	// Mise en page et tableaux du serveur (correspondance des tableaux).
	var theirs *lview
	var tables []RemoteTable
	err = srv.Delta(ctx, func(r Reader) error {
		var err error
		if tables, err = r.Tables(ctx, info.Schema); err != nil {
			return err
		}
		raw, err := r.Meta(ctx, info.Schema, layoutKind, layoutKey)
		if err != nil || raw == nil {
			return err
		}
		l, err := decodeLayout(raw)
		if err == nil {
			theirs = viewOf(l)
		}
		return err
	})
	if err != nil {
		return res, err
	}
	if theirs == nil {
		theirs = base
	}
	toLayout, _, err := tableIDs(ctx, db.Read(), tables)
	if err != nil {
		return res, err
	}
	ids = ids.clone()
	ids["table"] = toLayout
	mine.translate(ids)
	// Envoi (concepteurs) : mise en page du serveur relue dans la
	// transaction qui l'écrit.
	if design && differs(mine.view, base) {
		var sent *lview
		var n int
		err = srv.Write(ctx, func(w Writer) error {
			raw, err := w.Meta(ctx, info.Schema, layoutKind, layoutKey)
			if err != nil {
				return err
			}
			t := base
			if raw != nil {
				l, err := decodeLayout(raw)
				if err != nil {
					return err
				}
				t = viewOf(l)
			}
			theirs = t
			if separate(base, mine.view, t, mine, ids) {
				mine.translate(ids)
			}
			m, c := mergeViewsAs(base, mine.view, t, serverWins)
			if !differs(m, t) && enc(m.theme) == enc(t.theme) {
				return nil
			}
			out, err := encodeLayout(m.layout())
			if err != nil {
				return err
			}
			if err := w.SaveMeta(ctx, info.WorkbookID, layoutKind, out); err != nil {
				return err
			}
			sent, n = m, c
			return nil
		})
		if _, ok := Refusal(err); ok {
			// Droits retirés : rien ne part, et la mise en page du serveur
			// est reçue par-dessus (le poste le dit, Refused).
			err, sent = nil, nil
			res.Lost, design = true, false
		}
		if err != nil {
			return res, err
		}
		if sent != nil {
			theirs, res.Sent, res.Conflicts = sent, true, n
		}
	}
	if !res.Sent && !differs(theirs, base) && !unmapped {
		return res, nil
	}
	// Réception, dans une transaction locale : ce qui a changé sur le
	// serveur (et pas sur le poste) est appliqué hors du journal.
	err = db.Write(ctx, func(tx *sql.Tx) error {
		l2, err := readLocal(ctx, tx, ids)
		if err != nil {
			return err
		}
		if separate(base, l2.view, theirs, l2, ids) {
			l2.translate(ids)
		}
		m := l2.view
		if !design {
			m = keepBase(base, m)
		}
		target, nc := mergeViewsAs(base, m, theirs, serverWins)
		if serverWins && design && res.Conflicts == 0 {
			// Retour d'une longue coupure : les changements du poste écartés
			// au profit de ceux des collègues (rien n'est envoyé pour eux).
			res.Conflicts = nc
		}
		a := &applier{ctx: ctx, tx: tx, ids: ids, cols: map[string][]string{}, refused: map[string]map[string]bool{}}
		if err := a.apply(l2, target); err != nil {
			return err
		}
		if enc(target.theme) != enc(l2.view.theme) {
			if err := applyTheme(ctx, tx, &Layout{Theme: target.theme}); err != nil {
				return err
			}
			a.applied++
		}
		if a.choices {
			if err := canvas.ShowChoiceCells(ctx, tx); err != nil {
				return err
			}
		}
		// Nouvelle base : le serveur, sauf ce qui n'a pas pu être appliqué
		// (le poste ne le prend pas pour une modification à envoyer).
		nb := keepRefused(theirs, l2.view, a.refused)
		out, err := encodeLayout(nb.layout())
		if err != nil {
			return err
		}
		if err := writeBase(ctx, tx, out); err != nil {
			return err
		}
		if err := writeIDs(ctx, tx, ids); err != nil {
			return err
		}
		if a.forget {
			if err := command.Forget(ctx, tx); err != nil {
				return err
			}
		}
		res.Applied, res.Refused, res.Forgot = a.applied, a.nrefused, a.forget
		res.Gen = db.WatchGen()
		return nil
	})
	return res, err
}

// readTx lit dans une transaction de lecture (instantané cohérent).
func readTx(ctx context.Context, db *store.Store, fn func(*sql.Tx) error) error {
	tx, err := db.Read().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return fn(tx)
}

// keepRefused : la mise en page du serveur t, où les lignes refusées en
// local prennent leur valeur du poste (colonnes comparées).
func keepRefused(t, l *lview, refused map[string]map[string]bool) *lview {
	out := newView(t.format)
	out.theme, out.extra = t.theme, t.extra
	for name, x := range t.tables {
		out.tables[name] = &ltable{cols: x.cols, rows: maps.Clone(x.rows)}
	}
	for name, keys := range refused {
		ot := out.tables[name]
		if ot == nil {
			ot = &ltable{cols: unionCols(l.tables[name]), rows: map[string]lrow{}}
			out.tables[name] = ot
		}
		for k := range keys {
			lr, tr := l.row(name, k), ot.rows[k]
			switch {
			case lr == nil:
				delete(ot.rows, k)
			case tr == nil:
				ot.rows[k] = lr
			default:
				r := maps.Clone(lr)
				for c := range ownOf(name, lr, tr) {
					if v, ok := tr[c]; ok {
						r[c] = v
					}
				}
				ot.rows[k] = r
			}
		}
	}
	return out
}

// --- Application en local -------------------------------------------------------

type applier struct {
	ctx      context.Context
	tx       *sql.Tx
	ids      idMaps
	cols     map[string][]string
	refused  map[string]map[string]bool
	applied  int
	nrefused int
	choices  bool
	forget   bool
}

func (a *applier) refuse(name, key string) {
	if a.refused[name] == nil {
		a.refused[name] = map[string]bool{}
	}
	a.refused[name][key] = true
	a.nrefused++
}

// try exécute fn sous un point de sauvegarde : une contrainte (nom de
// feuille pris, feuille encore utilisée…) ne défait que cette ligne.
func (a *applier) try(fn func() error) bool {
	if _, err := a.tx.ExecContext(a.ctx, `SAVEPOINT lattis_layout`); err != nil {
		return false
	}
	if err := fn(); err != nil {
		a.tx.ExecContext(a.ctx, `ROLLBACK TO lattis_layout`)
		a.tx.ExecContext(a.ctx, `RELEASE lattis_layout`)
		return false
	}
	_, err := a.tx.ExecContext(a.ctx, `RELEASE lattis_layout`)
	return err == nil
}

func (a *applier) exec(q string, args ...any) error {
	_, err := a.tx.ExecContext(a.ctx, q, args...)
	return err
}

func (a *applier) columns(name string) []string {
	if c, ok := a.cols[name]; ok {
		return c
	}
	c, _ := tableColumns(a.ctx, a.tx, name)
	a.cols[name] = c
	return c
}

// apply amène la mise en page du poste l (lue dans la transaction) à
// target : suppressions (ordre inverse des tables), puis ajouts et
// modifications.
func (a *applier) apply(l *local, target *lview) error {
	skip := derived(l.view, target)
	for _, name := range slices.Backward(layoutTables) {
		lt := l.view.tables[name]
		if lt == nil {
			continue
		}
		for _, k := range slices.Sorted(maps.Keys(lt.rows)) {
			if target.row(name, k) != nil || name == "_lattis_cells" && skip[k] {
				continue
			}
			if ok := a.remove(name, l.orig[name][k]); ok {
				a.applied++
			} else {
				a.refuse(name, k)
			}
		}
	}
	for _, name := range layoutTables {
		tt := target.tables[name]
		if tt == nil {
			continue
		}
		for _, k := range slices.Sorted(maps.Keys(tt.rows)) {
			if name == "_lattis_cells" && skip[k] {
				continue
			}
			want, have := tt.rows[k], l.view.row(name, k)
			own := ownOf(name, want, have)
			if rowsEqual(want, have, own) {
				continue
			}
			var ok bool
			if have == nil {
				ok = a.insert(name, want)
			} else {
				ok = a.update(name, l.orig[name][k], have, want, own)
			}
			if ok {
				a.applied++
			} else {
				a.refuse(name, k)
			}
		}
	}
	return nil
}

// localRow traduit une ligne de la mise en page dans les identifiants du
// poste (ok faux : tableau, ligne ou objet inconnu ici).
func (a *applier) localRow(name string, r lrow) (lrow, bool) {
	out := maps.Clone(r)
	for c, v := range r {
		switch {
		case c == "tbl" || c == "id" && name == "_lattis_tables":
			n, _ := asInt(v)
			lo, ok := a.ids.toLocal("table", n)
			if !ok {
				return nil, false
			}
			out[c] = lo
		case c == "sheet" || c == "id" && surrogates[name] != "":
			kind := "sheet"
			table := "_lattis_sheets"
			if c == "id" {
				kind, table = surrogates[name], name
			}
			n, ok := asInt(v)
			if !ok {
				return nil, false
			}
			lo, ok := a.ids.toLocal(kind, n)
			if !ok {
				if c != "id" {
					return nil, false // feuille pas encore reçue
				}
				// Identifiant pris ici par un autre objet : un nouveau.
				if err := a.tx.QueryRowContext(a.ctx, `SELECT coalesce(max(id), 0) + 1 FROM `+table).Scan(&lo); err != nil {
					return nil, false
				}
				for k := range a.ids[kind] {
					lo = max(lo, k+1)
				}
				a.ids.set(kind, lo, n)
			}
			out[c] = lo
		}
	}
	if s, ok := r["rid"].(string); ok {
		tbl, _ := asInt(out["tbl"])
		var rid int64
		if err := a.tx.QueryRowContext(a.ctx, `SELECT rid FROM _lattis_row_ids WHERE tbl = ? AND lattis_id = ?`, tbl, s).Scan(&rid); err != nil {
			return nil, false // ligne pas encore reçue
		}
		out["rid"] = rid
	}
	return out, true
}

// where : clause sur la clé d'une ligne locale.
func where(name string, orig lrow) (string, []any) {
	var conds []string
	var args []any
	for _, k := range layoutKeys[name] {
		conds = append(conds, quoteCols([]string{k})+` = ?`)
		args = append(args, orig[k])
	}
	return strings.Join(conds, " AND "), args
}

func (a *applier) remove(name string, orig lrow) bool {
	switch {
	case name == "_lattis_tables":
		return false // tableaux : structure (§2.9)
	case name == "_lattis_columns" && !isCalc(orig):
		return false // colonne saisie : structure
	}
	return a.try(func() error {
		if name == "_lattis_columns" {
			tbl, _ := asInt(orig["tbl"])
			id, _ := asInt(orig["id"])
			if err := a.exec(`ALTER TABLE t_` + strconv.FormatInt(tbl, 10) + ` DROP COLUMN c_` + strconv.FormatInt(id, 10)); err != nil {
				return err
			}
			a.forget = true
		}
		w, args := where(name, orig)
		if err := a.exec(`DELETE FROM `+name+` WHERE `+w, args...); err != nil {
			return err
		}
		switch name {
		case "_lattis_sheets":
			a.forget, a.choices = true, true
		case "_lattis_choice_cells":
			a.choices = true
		}
		return nil
	})
}

func (a *applier) insert(name string, want lrow) bool {
	switch {
	case name == "_lattis_tables":
		return false // tableau reçu : InstallArrivals
	case name == "_lattis_columns" && !isCalc(want):
		return false // colonne saisie : structure
	}
	r, ok := a.localRow(name, want)
	if !ok {
		return false
	}
	if name == "_lattis_sheets" {
		r["choice"] = choiceText(mapChoice(parseChoice(want["choice"]), inverse(a.ids["table"])))
	}
	have := a.columns(name)
	var cols []string
	var args []any
	for _, c := range have {
		if v, ok := r[c]; ok {
			cols = append(cols, c)
			args = append(args, v)
		}
	}
	if len(cols) == 0 {
		return false
	}
	return a.try(func() error {
		if name == "_lattis_columns" {
			tbl, _ := asInt(r["tbl"])
			id, _ := asInt(r["id"])
			typ, _ := r["type"].(string)
			if err := a.exec(`ALTER TABLE t_` + strconv.FormatInt(tbl, 10) + ` ADD COLUMN ` + canvas.ColumnDef(id, grid.Type(typ))); err != nil {
				return err
			}
			a.forget = true
		}
		if err := a.exec(`INSERT INTO `+name+` (`+quoteCols(cols)+`) VALUES (`+strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")+`)`, args...); err != nil {
			return err
		}
		if name == "_lattis_sheets" || name == "_lattis_choice_cells" {
			a.choices = true
		}
		return nil
	})
}

func (a *applier) update(name string, orig, have, want lrow, own map[string]bool) bool {
	r, ok := a.localRow(name, want)
	if !ok {
		return false
	}
	keys := layoutKeys[name]
	var sets []string
	var args []any
	moved := false
	for _, c := range a.columns(name) {
		if own[c] || slices.Contains(keys, c) {
			continue
		}
		v, ok := r[c]
		if !ok || same(have[c], want[c]) {
			continue
		}
		sets = append(sets, quoteCols([]string{c})+` = ?`)
		args = append(args, v)
		moved = moved || c == "sheet" || c == "row" || c == "col"
	}
	if len(sets) == 0 {
		return true
	}
	if name == "_lattis_tables" && moved {
		// Tableau déplacé : pas sur ce qu'il y a ici (cellules, tableaux).
		if busy, err := a.occupied(r); err != nil || busy {
			return false
		}
	}
	w, wargs := where(name, orig)
	return a.try(func() error {
		if err := a.exec(`UPDATE `+name+` SET `+strings.Join(sets, ", ")+` WHERE `+w, append(args, wargs...)...); err != nil {
			return err
		}
		if name == "_lattis_choice_cells" || name == "_lattis_sheets" {
			a.choices = true
		}
		return nil
	})
}

// occupied dit si la place d'un tableau déplacé (ligne de _lattis_tables,
// identifiants du poste) recouvre une cellule libre ou un autre tableau de
// sa feuille.
func (a *applier) occupied(r lrow) (bool, error) {
	id, _ := asInt(r["id"])
	sheet, _ := asInt(r["sheet"])
	row, _ := asInt(r["row"])
	col, _ := asInt(r["col"])
	var rows, cols int64
	if err := a.tx.QueryRowContext(a.ctx, `SELECT count(*) FROM _lattis_columns WHERE tbl = ?`, id).Scan(&cols); err != nil {
		return false, err
	}
	if err := a.tx.QueryRowContext(a.ctx, `SELECT count(*) FROM t_`+strconv.FormatInt(id, 10)).Scan(&rows); err != nil {
		return false, err
	}
	r1, c1 := row+rows, col+max(cols, 1)-1
	var n int
	if err := a.tx.QueryRowContext(a.ctx, `SELECT count(*) FROM _lattis_cells WHERE sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`, sheet, row, r1, col, c1).Scan(&n); err != nil || n > 0 {
		return n > 0, err
	}
	rs, err := a.tx.QueryContext(a.ctx, `SELECT id, row, col, (SELECT count(*) FROM _lattis_columns k WHERE k.tbl = t.id) FROM _lattis_tables t WHERE sheet = ? AND id <> ?`, sheet, id)
	if err != nil {
		return false, err
	}
	type tb struct{ id, r, c, n int64 }
	var others []tb
	for rs.Next() {
		var x tb
		if err := rs.Scan(&x.id, &x.r, &x.c, &x.n); err != nil {
			rs.Close()
			return false, err
		}
		others = append(others, x)
	}
	rs.Close()
	for _, x := range others {
		var k int64
		_ = a.tx.QueryRowContext(a.ctx, `SELECT count(*) FROM t_`+strconv.FormatInt(x.id, 10)).Scan(&k)
		if x.r <= r1 && row <= x.r+k && x.c <= c1 && col <= x.c+max(x.n, 1)-1 {
			return true, nil
		}
	}
	return false, rs.Err()
}

// inverse : identifiant de la mise en page → du poste.
func inverse(m map[int64]int64) map[int64]int64 {
	out := map[int64]int64{}
	for k, v := range m {
		out[v] = k
	}
	return out
}
