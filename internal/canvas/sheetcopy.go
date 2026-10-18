package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// CopySheet copie un onglet (Déplacer ou copier… › Créer une copie) et place
// la copie au rang to (0 : premier ; au-delà du dernier : à la fin). La copie
// emporte les cellules libres, la mise en forme, les largeurs et hauteurs, les
// tableaux (données, colonnes calculées, styles, filtres, clé), les tableaux
// croisés, graphiques et boutons ; elle ne reprend ni la protection ni les
// cellules nommées (un nom désigne une seule cellule du classeur). Les
// tableaux et TCD copiés reçoivent un nom libre (Ventes → Ventes_2) ; un TCD ou
// un graphique de la feuille qui lisait un tableau copié lit sa copie.
// Renvoie l'identifiant de la nouvelle feuille.
func (m *Model) CopySheet(ctx context.Context, sheet int64, to int) (int64, error) {
	var id int64
	err := m.Do(ctx, "Copier la feuille", func(t *tx) (err error) {
		id, err = t.copySheet(sheet, to)
		return err
	})
	return id, err
}

func (t *tx) copySheet(sheet int64, to int) (int64, error) {
	if err := t.guardWorkbook(); err != nil {
		return 0, err
	}
	var name string
	if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&name); err != nil {
		return 0, fmt.Errorf("%w : feuille introuvable", ErrSheet)
	}
	var id int64
	var pos int
	if err := t.row(`SELECT coalesce(max(id), 0) + 1, coalesce(max(pos), -1) + 1 FROM _lattis_sheets`).Scan(&id, &pos); err != nil {
		return 0, err
	}
	copyName, err := t.copySheetName(name)
	if err != nil {
		return 0, err
	}
	if err := t.copyRows(`_lattis_sheets`, "id", sheet, id, map[string]any{"name": copyName, "pos": pos, "hidden": 0}); err != nil {
		return 0, err
	}
	for _, tbl := range append([]string{`_lattis_cells`}, sheetPositioned...) {
		if err := t.copyRows(tbl, "sheet", sheet, id, nil); err != nil {
			return 0, err
		}
	}
	// Tableaux, puis TCD (qui peuvent lire un tableau copié), puis graphiques
	// (qui peuvent lire l'un ou l'autre).
	renamed := map[string]string{}
	tables, err := t.tables(sheet)
	if err != nil {
		return 0, err
	}
	for _, tb := range tables {
		n, err := t.copyTable(tb, id)
		if err != nil {
			return 0, err
		}
		renamed[strings.ToLower(tb.Name)] = n
	}
	if err := t.retargetCopy(id, renamed); err != nil {
		return 0, err
	}
	if err := t.copyObjects(`_lattis_pivots`, sheet, id, func(cfg map[string]any) error {
		remapSource(cfg, renamed)
		n, err := t.pivotName()
		if old, ok := cfg["name"].(string); ok && old != "" {
			renamed[strings.ToLower(old)] = n
		}
		cfg["name"] = n
		return err
	}); err != nil {
		return 0, err
	}
	if err := t.copyObjects(`_lattis_charts`, sheet, id, func(cfg map[string]any) error {
		remapSource(cfg, renamed)
		return nil
	}); err != nil {
		return 0, err
	}
	// Les contrôles copiés prennent un nom libre (unique dans le classeur).
	if err := t.copyObjects(`_lattis_buttons`, sheet, id, func(cfg map[string]any) error {
		taken, err := t.controlNames(0)
		kind, _ := cfg["kind"].(string)
		cfg["name"] = NextControlName(kind, taken)
		return err
	}); err != nil {
		return 0, err
	}
	return id, t.moveSheet(id, to)
}

// retargetCopy fait viser aux formules de la feuille copiée sheet (cellules
// libres et colonnes calculées de ses tableaux) les copies des tableaux
// (renamed : ancien nom en minuscules → nom de la copie), comme Excel : la
// formule =SOMME(Ventes[Total]) de la copie lit Ventes_2. Les formules qui
// visent d'autres tableaux ne changent pas.
func (t *tx) retargetCopy(sheet int64, renamed map[string]string) error {
	if len(renamed) == 0 {
		return nil
	}
	type edit struct {
		q        string
		old, neu string
		key      []any
	}
	var edits []edit
	collect := func(q, upd string, loc formula.Locale) error {
		rs, err := t.query(q, sheet)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var a, b int64
			var raw string
			if err := rs.Scan(&a, &b, &raw); err != nil {
				return err
			}
			n, err := formula.Parse(raw, loc)
			if err != nil {
				continue
			}
			if x, ok := formula.RetargetTables(n, renamed); ok {
				edits = append(edits, edit{upd, raw, formula.Format(x, loc), []any{a, b}})
			}
		}
		return rs.Err()
	}
	if err := collect(`SELECT row, col, raw FROM _lattis_cells WHERE sheet = ? AND raw LIKE '=%'`,
		`UPDATE _lattis_cells SET raw = ? WHERE sheet = `+fmt.Sprint(sheet)+` AND row = ? AND col = ?`, formula.FR); err != nil {
		return err
	}
	if err := collect(`SELECT c.tbl, c.id, c.formula FROM _lattis_columns c JOIN _lattis_tables tb ON tb.id = c.tbl
		WHERE tb.sheet = ? AND c.formula IS NOT NULL AND c.formula <> ''`,
		`UPDATE _lattis_columns SET formula = ? WHERE tbl = ? AND id = ?`, formula.EN); err != nil {
		return err
	}
	var do, undo []stmt
	for _, e := range edits {
		do = append(do, st(e.q, append([]any{e.neu}, e.key...)...))
		undo = append(undo, st(e.q, append([]any{e.old}, e.key...)...))
	}
	if len(do) == 0 {
		return nil
	}
	return t.apply(do, undo)
}

// remapSource fait lire à un objet copié la copie de son tableau source.
func remapSource(cfg map[string]any, renamed map[string]string) {
	if src, ok := cfg["source"].(string); ok {
		if n, ok := renamed[strings.ToLower(src)]; ok {
			cfg["source"] = n
		}
	}
}

// copySheetName choisit le nom d'une copie : « Ventes (2) », « Ventes (3) »…
func (t *tx) copySheetName(name string) (string, error) {
	base := name
	if k := strings.LastIndex(name, " ("); k > 0 && strings.HasSuffix(name, ")") {
		base = name[:k]
	}
	for n := 2; ; n++ {
		cand := fmt.Sprintf("%s (%d)", base, n)
		taken, err := t.sheetNameTaken(cand, 0)
		if err != nil {
			return "", err
		}
		if !taken {
			return cand, nil
		}
	}
}

// columnsOf donne les colonnes d'une table, dans l'ordre du schéma.
func (t *tx) columnsOf(table string) ([]string, error) {
	rs, err := t.query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
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

// copyRows recopie les lignes d'une table de métadonnées où key vaut from, en
// y mettant key = to et les valeurs de set ; l'annulation les supprime. Les
// colonnes sont lues dans le schéma : une colonne ajoutée par une migration
// suit sans changer ce code.
func (t *tx) copyRows(table, key string, from, to any, set map[string]any) error {
	cols, err := t.columnsOf(table)
	if err != nil {
		return err
	}
	var sel []string
	var args []any
	for _, c := range cols {
		switch v, ok := set[c]; {
		case c == key:
			sel = append(sel, "?")
			args = append(args, to)
		case ok:
			sel = append(sel, "?")
			args = append(args, v)
		default:
			sel = append(sel, c)
		}
	}
	args = append(args, from)
	q := `INSERT INTO ` + table + ` (` + strings.Join(cols, ", ") + `) SELECT ` + strings.Join(sel, ", ") + ` FROM ` + table + ` WHERE ` + key + ` = ?`
	return t.apply([]stmt{st(q, args...)}, []stmt{st(`DELETE FROM `+table+` WHERE `+key+` = ?`, to)})
}

// copyObjects recopie les objets (TCD, graphiques, boutons) d'une feuille sur
// une autre, avec de nouveaux identifiants ; edit ajuste leur configuration.
func (t *tx) copyObjects(table string, from, to int64, edit func(map[string]any) error) error {
	rs, err := t.query(`SELECT id, config FROM `+table+` WHERE sheet = ? ORDER BY id`, from)
	if err != nil {
		return err
	}
	type obj struct {
		id  int64
		cfg string
	}
	var objs []obj
	for rs.Next() {
		var o obj
		if err := rs.Scan(&o.id, &o.cfg); err != nil {
			rs.Close()
			return err
		}
		objs = append(objs, o)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	for _, o := range objs {
		cfg := o.cfg
		if edit != nil {
			var m map[string]any
			if err := json.Unmarshal([]byte(o.cfg), &m); err != nil {
				return err
			}
			if err := edit(m); err != nil {
				return err
			}
			b, err := json.Marshal(m)
			if err != nil {
				return err
			}
			cfg = string(b)
		}
		var id int64
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM ` + table).Scan(&id); err != nil {
			return err
		}
		if err := t.copyRows(table, "id", o.id, id, map[string]any{"sheet": to, "config": cfg}); err != nil {
			return err
		}
	}
	return nil
}

// copyTable recopie un tableau (table SQL, index, colonnes, saisies
// invalides, styles) sur la feuille sheet et renvoie le nom de la copie.
func (t *tx) copyTable(tb Table, sheet int64) (string, error) {
	var id int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_tables`).Scan(&id); err != nil {
		return "", err
	}
	name, err := t.copyTableName(tb.Name)
	if err != nil {
		return "", err
	}
	// Table SQL et index, d'après leur définition (STRICT, clé, ordre _pos).
	rs, err := t.query(`SELECT type, sql FROM sqlite_master WHERE tbl_name = ? AND sql IS NOT NULL ORDER BY type = 'table' DESC`, tb.sql())
	if err != nil {
		return "", err
	}
	var defs []string
	for rs.Next() {
		var typ, q string
		if err := rs.Scan(&typ, &q); err != nil {
			rs.Close()
			return "", err
		}
		defs = append(defs, q)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return "", err
	}
	if len(defs) == 0 {
		return "", fmt.Errorf("%w : table de %s introuvable", ErrTable, tb.Name)
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(tb.sql()) + `(\D|$)`)
	newSQL := tableSQL(id)
	do := []stmt{st(re.ReplaceAllString(defs[0], newSQL+"$1")), st(`INSERT INTO ` + newSQL + ` SELECT * FROM ` + tb.sql())}
	for _, q := range defs[1:] {
		do = append(do, st(re.ReplaceAllString(q, newSQL+"$1")))
	}
	if err := t.apply(do, []stmt{st(`DROP TABLE ` + newSQL)}); err != nil {
		return "", err
	}
	if err := t.copyRows(`_lattis_tables`, "id", tb.ID, id, map[string]any{"sheet": sheet, "name": name}); err != nil {
		return "", err
	}
	for _, meta := range []string{`_lattis_columns`, `_lattis_invalid`, `_lattis_styles`} {
		if err := t.copyRows(meta, "tbl", tb.ID, id, nil); err != nil {
			return "", err
		}
	}
	return name, nil
}

// copyTableName choisit le nom de la copie d'un tableau : Ventes_2, Ventes_3…
// (libre parmi les tableaux et les TCD).
func (t *tx) copyTableName(name string) (string, error) {
	base := name
	if k := strings.LastIndex(name, "_"); k > 0 && strings.Trim(name[k+1:], "0123456789") == "" && k < len(name)-1 {
		base = name[:k]
	}
	ps, err := t.allPivots()
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, p := range ps {
		taken[strings.ToLower(p.Cfg.Name)] = true
	}
	for n := 2; ; n++ {
		cand := fmt.Sprintf("%s_%d", base, n)
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_tables WHERE name = ? COLLATE NOCASE`, cand).Scan(&k); err != nil {
			return "", err
		}
		if k == 0 && !taken[strings.ToLower(cand)] {
			return cand, nil
		}
	}
}

// SetSheetColor fixe la couleur de l'onglet (« #rrggbb », vide : aucune).
func (m *Model) SetSheetColor(ctx context.Context, sheet int64, color string) error {
	color = strings.TrimSpace(color)
	if color != "" && !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(color) {
		return fmt.Errorf("%w : couleur « %s » invalide (#rrggbb attendu)", ErrSheet, color)
	}
	return m.Do(ctx, "Couleur d'onglet", func(t *tx) error {
		if err := t.guardWorkbook(); err != nil {
			return err
		}
		var old string
		if err := t.row(`SELECT color FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&old); err != nil {
			return fmt.Errorf("%w : feuille introuvable", ErrSheet)
		}
		if old == color {
			return nil
		}
		q := `UPDATE _lattis_sheets SET color = ? WHERE id = ?`
		return t.apply([]stmt{st(q, color, sheet)}, []stmt{st(q, old, sheet)})
	})
}

// SetSheetHidden masque (Masquer) ou affiche (Afficher…) un onglet. Le
// classeur garde au moins une feuille visible.
func (m *Model) SetSheetHidden(ctx context.Context, sheet int64, hidden bool) error {
	label := "Afficher la feuille"
	if hidden {
		label = "Masquer la feuille"
	}
	return m.Do(ctx, label, func(t *tx) error {
		if err := t.guardWorkbook(); err != nil {
			return err
		}
		var old bool
		if err := t.row(`SELECT hidden <> 0 FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&old); err != nil {
			return fmt.Errorf("%w : feuille introuvable", ErrSheet)
		}
		if old == hidden {
			return nil
		}
		if hidden {
			var visible int
			if err := t.row(`SELECT count(*) FROM _lattis_sheets WHERE hidden = 0`).Scan(&visible); err != nil {
				return err
			}
			if visible <= 1 {
				return fmt.Errorf("%w : un classeur garde au moins une feuille visible", ErrSheet)
			}
		}
		q := `UPDATE _lattis_sheets SET hidden = ? WHERE id = ?`
		return t.apply([]stmt{st(q, hidden, sheet)}, []stmt{st(q, old, sheet)})
	})
}
