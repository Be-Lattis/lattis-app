package canvas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/order"
)

// ImportTable est un tableau créé d'un bloc par Import (fichier CSV, classeur
// Excel). Les types sont décidés par l'appelant, qui a vu toutes les valeurs :
// rien n'est élargi après coup.
type ImportTable struct {
	Name     string // identifiant libre (voir TableNames)
	Row, Col int    // ancre : cellule d'en-tête en haut à gauche
	Headers  []string
	Types    []grid.Type
	// Formulas donne, par colonne, la formule (français, commençant par =)
	// d'une colonne calculée, ou "". Si elle ne peut pas être enregistrée, la
	// colonne garde ses valeurs et Import le signale dans ses avertissements.
	Formulas []string
	Count    int // lignes de données
	// Rows produit les Count lignes, chacune de len(Headers) valeurs déjà au
	// type de stockage de leur colonne : int64 (entier, booléen 0/1),
	// float64 (décimal), string (texte, date ISO aaaa-mm-jj) ou nil.
	Rows func(yield func([]any) error) error
	// Select remplace Rows pour les gros volumes : une requête SQL qui rend
	// les Count lignes, colonnes n (1 à Count, l'ordre des lignes) puis v1…vk
	// (une par en-tête, au type de stockage), insérées par un seul
	// INSERT … SELECT, sans aller-retour avec Go. SelectArgs : ses paramètres.
	Select     string
	SelectArgs []any
	// SelectKeys : la requête rend aussi la colonne k, clés d'ordre (_pos)
	// des lignes, déjà croissantes avec n (sinon elles sont calculées ici).
	SelectKeys bool
	// Selects : Select découpé en morceaux exécutés dans l'ordre (mêmes
	// colonnes), pour suivre l'avancement ; Progress reçoit le nombre de
	// lignes écrites après chaque morceau.
	Selects  []string
	Progress func(rows int)
}

// ImportCell est une cellule libre importée.
type ImportCell struct {
	Row, Col int
	Raw      string // saisie ; une formule (français) commence par =
	Fallback string // saisie gardée si la formule ne peut pas être enregistrée
}

// ImportSheet est une feuille importée, ajoutée après les feuilles existantes.
type ImportSheet struct {
	Name   string // rendu unique si besoin (« Ventes (2) »)
	Tables []ImportTable
	Cells  []ImportCell
}

// ImportResult décrit ce qu'Import a créé.
type ImportResult struct {
	Sheets   []int64  `json:"sheets"`
	Tables   []string `json:"tables"`
	Warnings []string `json:"warnings"`
}

// importPrefix nomme les tables de chargement d'un import. Elles restent
// dans le fichier tant que l'import peut être rétabli (comme une corbeille).
const importPrefix = "_lattis_trash_import_"

// TableNames donne les noms de tableaux déjà pris dans le classeur.
func (m *Model) TableNames(ctx context.Context) (map[string]bool, error) {
	names := map[string]bool{}
	err := m.read(ctx, func(t *tx) error {
		rs, err := t.query(`SELECT name FROM _lattis_tables`)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var n string
			if err := rs.Scan(&n); err != nil {
				return err
			}
			names[n] = true
		}
		return rs.Err()
	})
	return names, err
}

// Import ajoute des feuilles, leurs tableaux et leurs cellules libres, en
// une seule étape d'annulation nommée action.
//
// Les lignes ne passent pas par le journal (un million de lignes y
// doublerait le fichier) : elles sont chargées dans une table à part, que
// l'opération renomme en tableau ; l'annulation la renomme en sens inverse,
// ce qui permet de rétablir. Une table de chargement dont plus aucune
// commande ne parle est supprimée au prochain import.
func (m *Model) Import(ctx context.Context, action string, sheets []ImportSheet) (ImportResult, error) {
	var res ImportResult
	err := m.Do(ctx, action, func(t *tx) error {
		if err := t.guardWorkbook(); err != nil {
			return err
		}
		res = ImportResult{Sheets: []int64{}, Tables: []string{}, Warnings: []string{}}
		if err := t.purgeImports(); err != nil {
			return err
		}
		type made struct {
			id int64
			it *ImportTable
		}
		var tables []made
		for si := range sheets {
			s := &sheets[si]
			id, err := t.newSheet(s.Name)
			if err != nil {
				return err
			}
			res.Sheets = append(res.Sheets, id)
			for i := range s.Tables {
				tb, err := t.importTable(id, &s.Tables[i])
				if err != nil {
					return err
				}
				tables = append(tables, made{tb.ID, &s.Tables[i]})
				res.Tables = append(res.Tables, tb.Name)
			}
		}
		// Les formules viennent quand tous les tableaux existent : elles
		// peuvent chercher dans un tableau d'une autre feuille.
		for _, mk := range tables {
			for i, f := range mk.it.Formulas {
				if f == "" {
					continue
				}
				tb, err := t.table(mk.id)
				if err != nil {
					return err
				}
				if err := t.setFormula(tb, i, f); err != nil {
					if !userError(err) {
						return err
					}
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s[%s] : %s ; valeurs importées sans la formule %s", tb.Name, tb.Cols[i].Name, err, f))
				}
			}
		}
		for si, s := range sheets {
			for _, c := range s.Cells {
				raw := c.Raw
				if isFormula(raw) {
					n, err := t.prepareFormula(res.Sheets[si], strings.TrimSpace(raw))
					switch {
					case err == nil:
						raw = formula.Format(n, formula.FR)
					case userError(err):
						res.Warnings = append(res.Warnings, fmt.Sprintf("%s, ligne %d, colonne %d : %s ; valeur importée sans la formule %s", s.Name, c.Row+1, c.Col+1, err, raw))
						raw = c.Fallback
					default:
						return err
					}
				}
				if err := t.setFreeCell(res.Sheets[si], c.Row, c.Col, raw); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return res, err
}

// userError distingue une formule refusée (à signaler) d'une panne.
func userError(err error) bool {
	var fe ErrFormula
	var ke ErrKey
	return errors.As(err, &fe) || errors.As(err, &ke)
}

// newSheet ajoute un onglet à la fin, sous un nom rendu unique.
func (t *tx) newSheet(name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Feuille"
	}
	var id int64
	var pos int
	if err := t.row(`SELECT coalesce(max(id), 0) + 1, coalesce(max(pos), -1) + 1 FROM _lattis_sheets`).Scan(&id, &pos); err != nil {
		return 0, err
	}
	base := name
	for n := 2; ; n++ {
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_sheets WHERE name = ?`, name).Scan(&k); err != nil {
			return 0, err
		}
		if k == 0 {
			break
		}
		name = fmt.Sprintf("%s (%d)", base, n)
	}
	return id, t.apply(
		[]stmt{st(`INSERT INTO _lattis_sheets (id, name, pos, gridlines) VALUES (?, ?, ?, 0)`, id, name, pos)},
		[]stmt{st(`DELETE FROM _lattis_sheets WHERE id = ?`, id)},
	)
}

// importTable charge un tableau hors journal puis le met en place par une
// opération réversible.
func (t *tx) importTable(sheet int64, it *ImportTable) (Table, error) {
	if len(it.Headers) == 0 || len(it.Types) != len(it.Headers) || len(it.Formulas) > len(it.Headers) {
		return Table{}, fmt.Errorf("import du tableau %s : colonnes incohérentes", it.Name)
	}
	if identifier(it.Name) != it.Name || it.Name == "" {
		return Table{}, fmt.Errorf("import : %q n'est pas un nom de tableau valable", it.Name)
	}
	var taken int
	if err := t.row(`SELECT count(*) FROM _lattis_tables WHERE name = ?`, it.Name).Scan(&taken); err != nil {
		return Table{}, err
	}
	if taken > 0 {
		return Table{}, fmt.Errorf("import : le tableau %s existe déjà", it.Name)
	}
	var id int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_tables`).Scan(&id); err != nil {
		return Table{}, err
	}
	tb := Table{ID: id, Sheet: sheet, Name: it.Name, Row: it.Row, Col: it.Col, Rows: it.Count}
	staging := fmt.Sprintf("%s%d_%d", importPrefix, id, time.Now().UnixNano())
	defs := []string{"id INTEGER PRIMARY KEY", "_pos TEXT NOT NULL"}
	names := []string{"id", "_pos"}
	meta := []stmt{st(`INSERT INTO _lattis_tables (id, sheet, name, row, col) VALUES (?, ?, ?, ?, ?)`, id, sheet, it.Name, it.Row, it.Col)}
	for i, h := range it.Headers {
		c := Column{ID: int64(i + 1), Name: strings.TrimSpace(h), Type: it.Types[i]}
		if c.Name == "" {
			c.Name = fmt.Sprintf("Colonne %d", i+1)
		}
		switch c.Type {
		case grid.Integer, grid.Decimal, grid.Text, grid.Boolean, grid.Date:
		default:
			return Table{}, fmt.Errorf("import : type %q inconnu", c.Type)
		}
		tb.Cols = append(tb.Cols, c)
		defs = append(defs, colSQL(c.ID)+" "+sqlType(c.Type))
		names = append(names, colSQL(c.ID))
		meta = append(meta, st(`INSERT INTO _lattis_columns (tbl, id, name, type, pos) VALUES (?, ?, ?, ?, ?)`, id, c.ID, c.Name, string(c.Type), i))
	}
	// Chargement hors journal, dans la transaction de la commande : si
	// l'import échoue, rien ne reste.
	if _, err := t.ExecContext(t.Ctx, `CREATE TABLE `+staging+` (`+strings.Join(defs, ", ")+`) STRICT`); err != nil {
		return Table{}, err
	}
	n := 0
	if it.Select != "" && len(it.Selects) == 0 {
		it.Selects = []string{it.Select}
	}
	if len(it.Selects) > 0 {
		// Index d'ordre posé avant le remplissage : les clés arrivent déjà
		// triées, il s'agrandit par la fin (un CREATE INDEX après coup trierait
		// des millions de clés en mémoire).
		if _, err := t.ExecContext(t.Ctx, `CREATE INDEX `+tb.sql()+`_pos ON `+staging+`(_pos)`); err != nil {
			return Table{}, err
		}
		keys := order.SpreadSQL(it.Count, "q.n")
		if it.SelectKeys {
			keys = "q.k"
		}
		vs := make([]string, len(it.Headers))
		for i := range vs {
			vs[i] = fmt.Sprintf("q.v%d", i+1)
		}
		for _, sel := range it.Selects {
			if err := t.Ctx.Err(); err != nil {
				return Table{}, err
			}
			res, err := t.ExecContext(t.Ctx, `INSERT INTO `+staging+` (`+strings.Join(names, ", ")+`) SELECT q.n, `+keys+`, `+strings.Join(vs, ", ")+` FROM (`+sel+`) AS q`, it.SelectArgs...)
			if err != nil {
				return Table{}, fmt.Errorf("import du tableau %s : %w", it.Name, err)
			}
			k, _ := res.RowsAffected()
			n += int(k)
			if it.Progress != nil {
				it.Progress(n)
			}
		}
	}
	ins, err := t.PrepareContext(t.Ctx, `INSERT INTO `+staging+` (`+strings.Join(names, ", ")+`) VALUES (`+marks(len(names))+`)`)
	if err != nil {
		return Table{}, err
	}
	defer ins.Close()
	var keys []string
	if len(it.Selects) == 0 {
		keys = order.Spread(it.Count)
	}
	args := make([]any, len(names))
	if it.Rows != nil && len(it.Selects) == 0 {
		err = it.Rows(func(vals []any) error {
			if n >= it.Count {
				return fmt.Errorf("import du tableau %s : plus de %d lignes", it.Name, it.Count)
			}
			if len(vals) != len(it.Headers) {
				return fmt.Errorf("import du tableau %s : ligne %d de %d valeurs au lieu de %d", it.Name, n+1, len(vals), len(it.Headers))
			}
			if n%4096 == 0 {
				if err := t.Ctx.Err(); err != nil {
					return err
				}
			}
			args[0], args[1] = int64(n+1), keys[n]
			copy(args[2:], vals)
			if _, err := ins.ExecContext(t.Ctx, args...); err != nil {
				return fmt.Errorf("import du tableau %s, ligne %d : %w", it.Name, n+1, err)
			}
			n++
			return nil
		})
		if err != nil {
			return Table{}, err
		}
	}
	if n != it.Count {
		return Table{}, fmt.Errorf("import du tableau %s : %d lignes au lieu de %d", it.Name, n, it.Count)
	}
	do := []stmt{st(`ALTER TABLE ` + staging + ` RENAME TO ` + tb.sql())}
	if len(it.Selects) == 0 {
		do = append(do, st(`CREATE INDEX `+tb.sql()+`_pos ON `+tb.sql()+`(_pos)`))
	}
	do = append(do, meta...)
	undo := []stmt{
		st(`DELETE FROM _lattis_invalid WHERE tbl = ?`, id),
		st(`DELETE FROM _lattis_columns WHERE tbl = ?`, id),
		st(`DELETE FROM _lattis_tables WHERE id = ?`, id),
	}
	if len(it.Selects) == 0 {
		undo = append(undo, st(`DROP INDEX `+tb.sql()+`_pos`))
	}
	undo = append(undo, st(`ALTER TABLE `+tb.sql()+` RENAME TO `+staging))
	return tb, t.apply(do, undo)
}

// purgeImports supprime les tables de chargement qu'aucune commande de
// l'historique ne peut plus remettre en place (import annulé puis remplacé
// par une autre action).
func (t *tx) purgeImports() error {
	rs, err := t.query(`SELECT name FROM sqlite_schema WHERE type = 'table' AND substr(name, 1, ?) = ?`, len(importPrefix), importPrefix)
	if err != nil {
		return err
	}
	var names []string
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			rs.Close()
			return err
		}
		names = append(names, n)
	}
	rs.Close()
	for _, n := range names {
		var used int
		if err := t.row(`SELECT count(*) FROM _lattis_journal WHERE instr(steps, ?) > 0`, n).Scan(&used); err != nil {
			return err
		}
		if used == 0 {
			if _, err := t.ExecContext(t.Ctx, `DROP TABLE `+n); err != nil {
				return err
			}
		}
	}
	return nil
}

// ExportColumn décrit une colonne lue par ExportTable.
type ExportColumn struct {
	Name     string
	Type     grid.Type
	Computed bool
}

// ExportTable lit un tableau dans l'ordre d'affichage. header reçoit les
// colonnes, puis each chaque ligne : valeurs stockées (int64, float64,
// string ou nil ; booléens en 0/1, dates en aaaa-mm-jj), formula.Value pour
// une colonne calculée, et pour une saisie hors type gardée en erreur, la
// saisie brute de type Invalid.
func (m *Model) ExportTable(ctx context.Context, table string, header func([]ExportColumn) error, each func([]any) error) error {
	var tb Table
	invalid := map[[2]int64]string{}
	err := m.read(ctx, func(t *tx) error {
		var err error
		if tb, err = t.tableByName(table); err != nil {
			return err
		}
		rs, err := t.query(`SELECT rid, col, raw FROM _lattis_invalid WHERE tbl = ?`, tb.ID)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var rid, col int64
			var raw string
			if err := rs.Scan(&rid, &col, &raw); err != nil {
				return err
			}
			invalid[[2]int64{rid, col}] = raw
		}
		return rs.Err()
	})
	if err != nil {
		return err
	}
	exprs, args, computed, err := m.ColumnExprs(ctx, tb.ID)
	if err != nil {
		return err
	}
	cols := make([]ExportColumn, len(tb.Cols))
	for i, c := range tb.Cols {
		cols[i] = ExportColumn{Name: c.Name, Type: c.Type, Computed: computed[i]}
	}
	if err := header(cols); err != nil {
		return err
	}
	q := `SELECT ` + tb.sql() + `.id`
	for _, e := range exprs {
		q += ", " + e
	}
	q += ` FROM ` + tb.sql() + ` ORDER BY _pos`
	rs, err := m.db.Read().QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rs.Close()
	dest := make([]any, len(exprs)+1)
	ptrs := make([]any, len(dest))
	for i := range dest {
		ptrs[i] = &dest[i]
	}
	row := make([]any, len(exprs))
	for rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return err
		}
		rid, _ := dest[0].(int64)
		for i, c := range tb.Cols {
			v := dest[i+1]
			if b, ok := v.([]byte); ok && !computed[i] {
				v = string(b)
			}
			switch {
			case computed[i]:
				v = formula.FromResult(v)
			case v == nil:
				if raw, ok := invalid[[2]int64{rid, c.ID}]; ok {
					v = Invalid(raw)
				}
			}
			row[i] = v
		}
		if err := each(row); err != nil {
			return err
		}
	}
	return rs.Err()
}

// Invalid est une saisie hors type, gardée telle quelle (voir ExportTable).
type Invalid string
