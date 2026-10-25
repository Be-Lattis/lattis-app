package workbook

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
)

// Mise en page publiée (§5.2 du serveur : lattis_meta_*, miroir des tables
// _lattis_* locales). Le classeur figé la publie en un seul objet
// (save_meta(id, 'layout', {"key": "workbook", …})) : les lignes des tables
// de métadonnées de la liste ci-dessous, sans celles des tableaux privés,
// telles quelles. Une liste blanche : une table de métadonnées ajoutée plus
// tard (réglages de l'utilisateur, secrets…) n'est jamais publiée sans
// qu'on l'y mette.
var layoutTables = []string{
	"_lattis_sheets", "_lattis_tables", "_lattis_columns", "_lattis_cells", "_lattis_choice_cells",
	"_lattis_col_widths", "_lattis_row_heights", "_lattis_names",
	"_lattis_styles", "_lattis_cell_styles", "_lattis_row_styles", "_lattis_col_styles",
	"_lattis_row_rules", "_lattis_col_rules", "_lattis_pivots", "_lattis_charts",
	"_lattis_buttons", "_lattis_pages", "_lattis_range_cells", "_lattis_page_setup",
	"_lattis_images", "_lattis_objects", "_lattis_sparklines", "_lattis_row_outline", "_lattis_col_outline",
}

// Images d'un classeur partagé : la mise en page est un seul objet du
// serveur, renvoyé entier à chaque changement de mise en page. Seules les
// images qu'un objet cite, de canvas.MaxSharedImageBytes au plus, y vont
// (une plus lourde, posée avant le partage, reste sur le poste : les autres
// membres voient son cadre vide) ; dans un classeur déjà partagé, une image
// plus lourde est refusée à l'insertion (canvas.CreateImage). Une image ne
// change jamais (clé : son empreinte), la fusion ligne à ligne n'a donc
// rien à comparer.
var imagesFilter = ` WHERE length(data) <= ` + strconv.Itoa(canvas.MaxSharedImageBytes) +
	` AND hash IN (SELECT json_extract(config, '$.image') FROM _lattis_objects)`

// Onglets à choix : _lattis_sheets.choice est publiée sans la valeur
// choisie (la catégorie seule, publishedChoice), qui reste propre à chaque
// poste (voir choice.go ; loader.mergeChoices la remet).

// Plan des feuilles (canvas/outline.go) : les niveaux des groupes sont
// publiés ; l'état réduit (folded) reste propre à chaque poste en direct
// (layoutOwn), pris du serveur à l'ouverture.

// Hors de la liste, volontairement : les données des tableaux (tables du
// serveur), les scripts (publish_script), les saisies invalides (refusées à
// la publication), la protection, les requêtes favorites, les connexions et
// requêtes d'import (propres au poste), les hôtes HTTP autorisés (C2), le
// journal et tout l'état de synchronisation.

const (
	layoutKind = "layout"
	layoutKey  = "workbook"
)

// Layout est la mise en page publiée.
type Layout struct {
	Key string `json:"key"`
	// Format : version du format local (meta.Version) de l'auteur.
	Format int                    `json:"format"`
	Tables map[string]*LayoutRows `json:"tables"`
	// Theme : thème de couleur du classeur (_lattis_info, clé theme, seule
	// clé de cette table publiée ; nil : thème par défaut de chaque poste).
	Theme *canvas.Theme `json:"theme,omitempty"`
}

// LayoutRows sont les lignes d'une table de métadonnées.
type LayoutRows struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

// index donne le rang d'une colonne (-1 si absente).
func (r *LayoutRows) index(name string) int { return slices.Index(r.Columns, name) }

// rowIDs relie l'identifiant local d'une ligne (rid) de chaque tableau
// publié à son identifiant partagé : les mises en forme par ligne
// (_lattis_styles) sont publiées par lattis_id, le classeur qui les reçoit
// ayant d'autres rid.
type rowIDs map[int64]map[int64]string

// rowLookup donne l'identifiant partagé d'une ligne (rid) d'un tableau.
// tracked : le tableau a des identifiants partagés (sinon le rid reste tel
// quel) ; ok : la ligne en a un (sinon elle n'est pas publiée).
type rowLookup interface {
	rowID(ctx context.Context, tbl, rid int64) (u string, tracked, ok bool, err error)
}

func (ids rowIDs) rowID(_ context.Context, tbl, rid int64) (string, bool, bool, error) {
	m, tracked := ids[tbl]
	u, ok := m[rid]
	return u, tracked, ok, nil
}

type txOrDB interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

// dumpLayout lit la mise en page. published : tableaux publiés (nil : tous
// les tableaux sont gardés, tableaux privés compris).
func dumpLayout(ctx context.Context, q txOrDB, format int, published map[int64]bool, ids rowLookup) (*Layout, error) {
	l := &Layout{Key: layoutKey, Format: format, Tables: map[string]*LayoutRows{}}
	for _, name := range layoutTables {
		cols, err := tableColumns(ctx, q, name)
		if err != nil {
			return nil, err
		}
		if len(cols) == 0 {
			continue // format plus ancien
		}
		order := make([]string, len(cols))
		for i := range cols {
			order[i] = strconv.Itoa(i + 1)
		}
		filter := ""
		if name == "_lattis_images" {
			filter, order = imagesFilter, order[:1]
		}
		rs, err := q.QueryContext(ctx, `SELECT `+quoteCols(cols)+` FROM `+name+filter+` ORDER BY `+strings.Join(order, ", "))
		if err != nil {
			return nil, err
		}
		lr := &LayoutRows{Columns: cols, Rows: [][]any{}}
		tblAt, ridAt, choiceAt := lr.index("tbl"), lr.index("rid"), -1
		switch name {
		case "_lattis_tables":
			tblAt = lr.index("id")
		case "_lattis_sheets":
			choiceAt = lr.index("choice")
		}
		for rs.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				rs.Close()
				return nil, err
			}
			for i, v := range vals {
				vals[i] = jsonValue(v)
			}
			if choiceAt >= 0 {
				vals[choiceAt] = publishedChoice(vals[choiceAt], published)
			}
			if tblAt >= 0 && published != nil {
				if tbl, ok := asInt(vals[tblAt]); ok && !published[tbl] {
					continue // tableau privé
				}
			}
			if ridAt >= 0 && tblAt >= 0 && ids != nil {
				tbl, _ := asInt(vals[tblAt])
				rid, _ := asInt(vals[ridAt])
				if rid != 0 {
					u, tracked, ok, err := ids.rowID(ctx, tbl, rid)
					if err != nil {
						rs.Close()
						return nil, err
					}
					if tracked && !ok {
						continue // ligne sans identifiant partagé
					}
					if tracked {
						vals[ridAt] = u
					}
				}
			}
			lr.Rows = append(lr.Rows, vals)
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return nil, err
		}
		l.Tables[name] = lr
	}
	var raw string
	switch err := q.QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, canvas.ThemeKey).Scan(&raw); {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return nil, err
	default:
		l.Theme = canvas.ParseTheme(raw)
	}
	return l, nil
}

// applyTheme remplace le thème local par celui de la mise en page reçue
// (le serveur l'emporte, comme pour le reste de la mise en page).
func applyTheme(ctx context.Context, tx *sql.Tx, l *Layout) error {
	if l.Theme == nil {
		_, err := tx.ExecContext(ctx, `DELETE FROM _lattis_info WHERE key = ?`, canvas.ThemeKey)
		return err
	}
	b, err := json.Marshal(l.Theme)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, canvas.ThemeKey, string(b))
	return err
}

// tableColumns donne les colonnes d'une table (aucune si elle n'existe pas).
func tableColumns(ctx context.Context, q txOrDB, name string) ([]string, error) {
	rs, err := q.QueryContext(ctx, `SELECT name FROM pragma_table_info(?) ORDER BY cid`, name)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rs.Err()
}

func quoteCols(cols []string) string {
	q := make([]string, len(cols))
	for i, c := range cols {
		q[i] = `"` + strings.ReplaceAll(c, `"`, `""`) + `"`
	}
	return strings.Join(q, ", ")
}

// jsonValue prépare une valeur SQLite pour le JSON (les blobs, rares dans
// les métadonnées, sont écrits {"$b": base64}).
func jsonValue(v any) any {
	if b, ok := v.([]byte); ok {
		return map[string]string{"$b": base64.StdEncoding.EncodeToString(b)}
	}
	return v
}

// sqlValue est l'inverse de jsonValue pour une valeur relue du JSON
// (json.Number : entier ou réel).
func sqlValue(v any) any {
	switch x := v.(type) {
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return n
		}
		f, _ := x.Float64()
		return f
	case map[string]any:
		if s, ok := x["$b"].(string); ok {
			b, _ := base64.StdEncoding.DecodeString(s)
			return b
		}
		b, _ := json.Marshal(x)
		return string(b)
	case bool:
		if x {
			return int64(1)
		}
		return int64(0)
	}
	return v
}

func asInt(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case float64:
		return int64(x), true
	case json.Number:
		n, err := x.Int64()
		return n, err == nil
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		return n, err == nil
	}
	return 0, false
}

// decodeLayout lit une mise en page publiée.
func decodeLayout(raw []byte) (*Layout, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var l Layout
	if err := d.Decode(&l); err != nil {
		return nil, msg.Wrap(err, "workbook.layoutUnreadable", msg.Args{"err": err})
	}
	if l.Tables == nil {
		l.Tables = map[string]*LayoutRows{}
	}
	for _, r := range l.Tables {
		for _, row := range r.Rows {
			for i, v := range row {
				row[i] = sqlValue(v)
			}
		}
	}
	return &l, nil
}

// rowKey écrit une ligne pour la comparer à une autre (même colonnes).
func rowKey(row []any) string {
	b, _ := json.Marshal(row)
	return string(b)
}

// project donne les lignes de r dans l'ordre des colonnes cols (absentes :
// NULL), pour comparer deux versions d'une même table.
func (r *LayoutRows) project(cols []string) [][]any {
	at := make([]int, len(cols))
	for i, c := range cols {
		at[i] = r.index(c)
	}
	out := make([][]any, len(r.Rows))
	for k, row := range r.Rows {
		p := make([]any, len(cols))
		for i, j := range at {
			if j >= 0 {
				p[i] = normalize(row[j])
			}
		}
		out[k] = p
	}
	return out
}

// normalize ramène une valeur à sa forme JSON relue (un entier écrit en
// réel, 3.0, se relit 3).
func normalize(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var out any
	if d.Decode(&out) != nil {
		return v
	}
	return sqlValue(out)
}

// insertRows écrit des lignes dans une table de métadonnées (colonnes
// communes seulement : une mise en page d'un format voisin passe).
func insertRows(ctx context.Context, tx *sql.Tx, name string, cols []string, rows [][]any, ignore bool) error {
	if len(rows) == 0 {
		return nil
	}
	have, err := tableColumns(ctx, tx, name)
	if err != nil {
		return err
	}
	var keep []int
	var names []string
	for i, c := range cols {
		if slices.Contains(have, c) {
			keep = append(keep, i)
			names = append(names, c)
		}
	}
	if len(keep) == 0 {
		return nil
	}
	verb := "INSERT"
	if ignore {
		verb = "INSERT OR IGNORE"
	}
	stmt, err := tx.PrepareContext(ctx, verb+` INTO `+name+` (`+quoteCols(names)+`) VALUES (`+strings.TrimSuffix(strings.Repeat("?, ", len(keep)), ", ")+`)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	args := make([]any, len(keep))
	for _, row := range rows {
		for i, j := range keep {
			args[i] = row[j]
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return fmt.Errorf("%s : %w", name, err)
		}
	}
	return nil
}
