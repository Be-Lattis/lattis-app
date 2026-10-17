package query

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/transfer"
)

// storer remplit une table de la base de travail avec les lignes d'une
// source.
type storer struct {
	r         *run
	table     string
	cols      []Column
	tx        *sql.Tx
	ins       *sql.Stmt
	args      []any
	n         int
	truncated bool
}

// begin crée la table d'une source (noms de colonnes rendus uniques et non
// vides : une requête SQL peut rendre deux « id »).
func (r *run) begin(cols []Column) (*storer, error) {
	if len(cols) == 0 {
		return nil, errors.New("la source n'a aucune colonne")
	}
	var named []Column
	for i, c := range cols {
		name := strings.Join(strings.Fields(c.Name), " ")
		if name == "" || !utf8.ValidString(name) {
			name = fmt.Sprintf("Colonne %d", i+1)
		}
		named = append(named, Column{Name: unique(named, name), Type: c.Type})
	}
	s := &storer{r: r, table: r.s.name("src"), cols: named, args: make([]any, len(named))}
	defs := make([]string, len(named))
	for i, c := range named {
		defs[i] = qi(c.Name)
	}
	if _, err := r.s.db.ExecContext(r.ctx, `CREATE TABLE `+qi(s.table)+` (`+strings.Join(defs, ", ")+`)`); err != nil {
		return nil, err
	}
	var err error
	if s.tx, err = r.s.db.BeginTx(r.ctx, nil); err != nil {
		s.drop()
		return nil, err
	}
	if s.ins, err = s.tx.PrepareContext(r.ctx, `INSERT INTO `+qi(s.table)+` VALUES (`+strings.TrimSuffix(strings.Repeat("?, ", len(named)), ", ")+`)`); err != nil {
		s.abort()
		return nil, err
	}
	return s, nil
}

// insert ajoute une ligne ; errLimit au-delà de la limite de l'aperçu.
func (s *storer) insert(row []any) error {
	if s.r.limit > 0 && s.n >= s.r.limit {
		s.truncated = true
		return errLimit
	}
	if s.n%4096 == 0 {
		if err := s.r.ctx.Err(); err != nil {
			return err
		}
	}
	for i := range s.args {
		s.args[i] = nil
		if i < len(row) {
			if s.cols[i].Type == "" {
				s.args[i] = plain(row[i])
			} else {
				s.args[i] = castValue(row[i], s.cols[i].Type)
			}
		}
	}
	s.n++
	_, err := s.ins.ExecContext(s.r.ctx, s.args...)
	return err
}

func (s *storer) drop() { s.r.s.db.Exec(`DROP TABLE IF EXISTS ` + qi(s.table)) }

func (s *storer) abort() {
	if s.ins != nil {
		s.ins.Close()
	}
	if s.tx != nil {
		s.tx.Rollback()
	}
	s.drop()
}

// finish valide la table ; err : erreur de lecture de la source (errLimit
// n'en est pas une).
func (s *storer) finish(err error) (loaded, error) {
	if err != nil && !errors.Is(err, errLimit) {
		s.abort()
		return loaded{}, err
	}
	s.ins.Close()
	if err := s.tx.Commit(); err != nil {
		s.drop()
		return loaded{}, err
	}
	return loaded{table: s.table, cols: s.cols, truncated: s.truncated}, nil
}

func (r *run) loadCSV(src Source) (loaded, error) {
	if src.Path == "" {
		return loaded{}, errors.New("fichier CSV non choisi")
	}
	var opt transfer.CSVOptions
	if d := src.Delimiter; d != "" {
		if d == `\t` || strings.EqualFold(d, "tab") {
			d = "\t"
		}
		opt.Comma, _ = utf8.DecodeRuneInString(d)
	}
	sheet, err := transfer.ReadCSV(r.ctx, src.Path, opt)
	if err != nil {
		return loaded{}, err
	}
	return r.storeImport(sheet.Tables[0])
}

func (r *run) storeImport(t canvas.ImportTable) (loaded, error) {
	cols := make([]Column, len(t.Headers))
	for i, h := range t.Headers {
		cols[i] = Column{Name: h, Type: t.Types[i]}
	}
	s, err := r.begin(cols)
	if err != nil {
		return loaded{}, err
	}
	if t.Rows != nil {
		err = t.Rows(s.insert)
	}
	return s.finish(err)
}

// XLSXTable décrit un tableau (îlot) d'un classeur .xlsx.
type XLSXTable struct {
	Sheet   string   `json:"sheet"`
	Index   int      `json:"index"`
	Headers []string `json:"headers"`
	Rows    int      `json:"rows"`
}

// XLSXTables liste les tableaux d'un classeur .xlsx.
func XLSXTables(ctx context.Context, path string) ([]XLSXTable, error) {
	sheets, closeFn, err := transfer.ReadXLSX(ctx, path)
	if err != nil {
		return nil, err
	}
	defer closeFn()
	out := []XLSXTable{}
	for _, s := range sheets {
		for i, t := range s.Tables {
			out = append(out, XLSXTable{Sheet: s.Name, Index: i, Headers: t.Headers, Rows: t.Count})
		}
	}
	return out, nil
}

func (r *run) loadXLSX(src Source) (loaded, error) {
	if src.Path == "" {
		return loaded{}, errors.New("classeur Excel non choisi")
	}
	sheets, closeFn, err := transfer.ReadXLSX(r.ctx, src.Path)
	if err != nil {
		return loaded{}, err
	}
	defer closeFn()
	for _, s := range sheets {
		if !strings.EqualFold(s.Name, src.Sheet) {
			continue
		}
		if src.Index < 0 || src.Index >= len(s.Tables) {
			return loaded{}, fmt.Errorf("la feuille « %s » n'a pas de tableau n° %d", s.Name, src.Index+1)
		}
		return r.storeImport(s.Tables[src.Index])
	}
	return loaded{}, fmt.Errorf("feuille « %s » introuvable dans le classeur", src.Sheet)
}

func (r *run) loadTable(src Source) (loaded, error) {
	if src.Table == "" {
		return loaded{}, errors.New("tableau non choisi")
	}
	var s *storer
	row := []any{}
	err := r.env.Model.ExportTable(r.ctx, src.Table, func(ec []canvas.ExportColumn) error {
		cols := make([]Column, len(ec))
		for i, c := range ec {
			cols[i] = Column{Name: c.Name, Type: c.Type}
			if c.Computed {
				cols[i].Type = "" // résultat de formule : inféré
			}
		}
		var err error
		s, err = r.begin(cols)
		return err
	}, func(vals []any) error {
		row = row[:0]
		for _, v := range vals {
			row = append(row, plain(v))
		}
		return s.insert(row)
	})
	if s == nil {
		if err == nil {
			err = fmt.Errorf("tableau « %s » illisible", src.Table)
		}
		return loaded{}, err
	}
	return s.finish(err)
}

// connection trouve une connexion du classeur par son nom.
func (r *run) connection(name string) (Connection, error) {
	cs, err := r.env.Model.Connections(r.ctx)
	if err != nil {
		return Connection{}, err
	}
	for _, c := range cs {
		if strings.EqualFold(c.Name, name) {
			return DecodeConnection(c)
		}
	}
	return Connection{}, fmt.Errorf("connexion « %s » introuvable", name)
}

// loadDB lit une source SGBD : connexion du classeur, requête SQL en
// lecture seule.
func (r *run) loadDB(src Source) (loaded, error) {
	if trimSQL(src.SQL) == "" {
		return loaded{}, errors.New("requête SQL vide")
	}
	conn, err := r.connection(src.Connection)
	if err != nil {
		return loaded{}, err
	}
	if conn.Kind != src.Kind {
		return loaded{}, fmt.Errorf("la connexion « %s » est de type %s, pas %s", conn.Name, KindLabel(conn.Kind), KindLabel(src.Kind))
	}
	db, d, err := connectDB(r.ctx, r.env.secrets(), conn)
	if err != nil {
		return loaded{}, err
	}
	defer db.Close()
	return r.readInto(db, d, src.SQL, d.label+" « "+conn.Name+" »")
}

// loadFileDB lit une source fichier de base (SQLite).
func (r *run) loadFileDB(src Source) (loaded, error) {
	if trimSQL(src.SQL) == "" {
		return loaded{}, errors.New("requête SQL vide")
	}
	db, d, err := openFileDB(src.Kind, src.Path)
	if err != nil {
		return loaded{}, err
	}
	defer db.Close()
	return r.readInto(db, d, src.SQL, d.label+" "+baseName(src.Path))
}

// readInto exécute la requête q et en charge le résultat ; la limite de
// l'aperçu est poussée vers la base quand le dialecte le permet.
func (r *run) readInto(db *sql.DB, d *dialect, q, label string) (loaded, error) {
	var s *storer
	err := readDB(r.ctx, db, d, q, r.limit, func(cols []Column) error {
		var err error
		s, err = r.begin(cols)
		return err
	}, func(row []any) error { return s.insert(row) })
	if s == nil {
		if err == nil {
			err = errors.New("la requête ne rend aucune colonne")
		}
		return loaded{}, fmt.Errorf("%s : %w", label, err)
	}
	if err != nil && !errors.Is(err, errLimit) {
		err = fmt.Errorf("%s : %w", label, err)
	}
	l, err := s.finish(err)
	if err == nil && r.limit > 0 && s.n >= r.limit {
		l.truncated = true
	}
	return l, err
}

// loadSQL lit une requête SQL sur le classeur (résultat référencé depuis
// l'onglet SQL) : relue à chaque actualisation, jamais gardée en cache.
func (r *run) loadSQL(src Source) (loaded, error) {
	var s *storer
	row := []any{}
	err := r.env.Model.ReadSQL(r.ctx, src.SQL, src.Sheet, func(names []string) error {
		cols := make([]Column, len(names))
		for i, n := range names {
			cols[i] = Column{Name: n}
		}
		var err error
		s, err = r.begin(cols)
		return err
	}, func(vals []any) error {
		row = row[:0]
		for _, v := range vals {
			row = append(row, plain(v))
		}
		return s.insert(row)
	})
	if s == nil {
		if err == nil {
			err = errors.New("la requête ne rend aucune colonne")
		}
		return loaded{}, err
	}
	l, err := s.finish(err)
	if err == nil && r.limit > 0 && s.n >= r.limit {
		l.truncated = true
	}
	return l, err
}
