package query

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/command"
	"github.com/lalamefine/slitable/internal/grid"
)

// Bornes de l'aperçu : lignes lues par source, lignes affichées.
const (
	PreviewSourceRows = 1000
	PreviewRows       = 200
)

// PasswordNeed demande le mot de passe d'une connexion.
type PasswordNeed struct {
	Connection string `json:"connection"`
	Account    string `json:"account"`
	Message    string `json:"message"`
}

// Preview est l'aperçu d'une requête à une étape.
type Preview struct {
	Columns []Column   `json:"columns"`
	Rows    [][]string `json:"rows"`
	// Count : lignes du résultat de l'aperçu (calculé sur les premières
	// lignes de chaque source si Sample).
	Count  int      `json:"count"`
	Sample bool     `json:"sample"`
	Step   int      `json:"step"`   // étapes appliquées
	Labels []string `json:"labels"` // source, puis chaque étape
	// Erreur : message et étape (-1 : la source) ; l'aperçu montre alors
	// la dernière étape réussie.
	Error        string        `json:"error"`
	ErrorStep    int           `json:"errorStep"`
	NeedPassword *PasswordNeed `json:"needPassword"`
	SQL          string        `json:"sql"` // requête SQLite compilée (pour les curieux)
}

func labels(q Query) []string {
	out := []string{"Source (" + SourceLabel(q.Source) + ")"}
	for _, s := range q.Steps {
		out = append(out, StepLabel(s))
	}
	return out
}

func needPassword(err error) *PasswordNeed {
	var pe *ErrPassword
	if errors.As(err, &pe) {
		return &PasswordNeed{Connection: pe.Connection, Account: pe.Account, Message: pe.Message}
	}
	return nil
}

// Preview calcule l'aperçu de q après ses upto premières étapes (-1 :
// toutes). fresh relit les sources au lieu de reprendre le cache.
func (s *Session) Preview(ctx context.Context, env Env, q Query, upto int, fresh bool) Preview {
	s.mu.Lock()
	defer s.mu.Unlock()
	if upto < 0 || upto > len(q.Steps) {
		upto = len(q.Steps)
	}
	pv := Preview{Columns: []Column{}, Rows: [][]string{}, Labels: labels(q), Step: upto, ErrorStep: -2}
	r := &run{s: s, env: env, ctx: ctx, limit: PreviewSourceRows, fresh: fresh}
	p, err := r.compile(q, upto)
	if err != nil {
		pv.Error = err.Error()
		pv.NeedPassword = needPassword(err)
		var se *StepError
		if errors.As(err, &se) {
			pv.ErrorStep = se.Step
			pv.Error = se.Err.Error()
		}
	}
	pv.Sample = r.truncated
	if p == nil {
		return pv
	}
	pv.Step = len(p.ctes) - 1
	pv.SQL = p.sql()
	if err := s.db.QueryRowContext(ctx, p.with("SELECT count(*) FROM "+p.last)).Scan(&pv.Count); err != nil {
		// Échec à l'exécution : la première étape fautive est cherchée en
		// remontant, et l'aperçu montre la précédente.
		k := 0
		for ; k < len(p.ctes); k++ {
			sub := p.upTo(k)
			if e := s.db.QueryRowContext(ctx, sub.with("SELECT count(*) FROM "+sub.last)).Scan(new(int)); e != nil {
				err = e
				break
			}
		}
		if pv.Error == "" {
			pv.Error, pv.ErrorStep = sqlMessage(err), k-1
		}
		if k == 0 {
			return pv
		}
		p = p.upTo(k - 1)
		pv.Step = k - 1
		if err := s.db.QueryRowContext(ctx, p.with("SELECT count(*) FROM "+p.last)).Scan(&pv.Count); err != nil {
			return pv
		}
	}
	if err := s.fill(ctx, p, &pv); err != nil && pv.Error == "" {
		pv.Error = err.Error()
	}
	return pv
}

// fill remplit les colonnes et les premières lignes de l'aperçu.
func (s *Session) fill(ctx context.Context, p *pipeline, pv *Preview) error {
	rs, err := s.db.QueryContext(ctx, p.with(fmt.Sprintf("SELECT * FROM %s LIMIT %d", p.last, PreviewRows)))
	if err != nil {
		return errors.New(sqlMessage(err))
	}
	defer rs.Close()
	n := len(p.cols)
	var raw [][]any
	for rs.Next() {
		row := make([]any, n)
		ptrs := make([]any, n)
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return err
		}
		raw = append(raw, row)
	}
	if err := rs.Err(); err != nil {
		return errors.New(sqlMessage(err))
	}
	cols := append([]Column{}, p.cols...)
	for i := range cols {
		if cols[i].Type != "" {
			continue
		}
		var t grid.Type
		for _, row := range raw {
			t = canvas.Join(t, inferValue(plain(row[i])))
		}
		cols[i].Type = final(t)
	}
	pv.Columns = cols
	for _, row := range raw {
		out := make([]string, n)
		for i, v := range row {
			out[i] = display(castValue(v, cols[i].Type), cols[i].Type)
		}
		pv.Rows = append(pv.Rows, out)
	}
	return nil
}

func final(t grid.Type) grid.Type {
	if t == "" {
		return grid.Text
	}
	return t
}

// sqlMessage rend une erreur SQLite lisible.
func sqlMessage(err error) string {
	s := err.Error()
	s = strings.TrimPrefix(s, "sqlite3: ")
	return s
}

// Result est le résultat complet d'une requête, prêt à charger.
type Result struct {
	Table canvas.ImportTable
	close func()
}

// Close libère le résultat dans la base de travail.
func (r *Result) Close() {
	if r.close != nil {
		r.close()
	}
}

// result calcule le résultat complet de q (sources relues), le matérialise
// dans la base de travail et infère le type des colonnes « auto ». À
// appeler session verrouillée.
func (s *Session) result(ctx context.Context, env Env, q Query) (*Result, error) {
	r := &run{s: s, env: env, ctx: ctx, fresh: true}
	p, err := r.compile(q, -1)
	if err != nil {
		return nil, err
	}
	table := s.name("res")
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE `+qi(table)+` AS `+p.sql()); err != nil {
		return nil, fmt.Errorf("requête « %s » : %s", q.Name, sqlMessage(err))
	}
	drop := func() { s.db.Exec(`DROP TABLE IF EXISTS ` + qi(table)) }
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM `+qi(table)).Scan(&count); err != nil {
		drop()
		return nil, err
	}
	cols := append([]Column{}, p.cols...)
	var auto []int
	for i, c := range cols {
		if c.Type == "" {
			auto = append(auto, i)
		}
	}
	if len(auto) > 0 {
		types := make([]grid.Type, len(cols))
		err := s.scan(ctx, table, len(cols), func(row []any) error {
			for _, i := range auto {
				types[i] = canvas.Join(types[i], inferValue(plain(row[i])))
			}
			return nil
		})
		if err != nil {
			drop()
			return nil, err
		}
		for _, i := range auto {
			cols[i].Type = final(types[i])
		}
	}
	it := canvas.ImportTable{Count: count}
	for _, c := range cols {
		it.Headers = append(it.Headers, c.Name)
		it.Types = append(it.Types, c.Type)
	}
	it.Rows = func(yield func([]any) error) error {
		out := make([]any, len(cols))
		return s.scan(ctx, table, len(cols), func(row []any) error {
			for i, v := range row {
				out[i] = castValue(v, cols[i].Type)
			}
			return yield(out)
		})
	}
	return &Result{Table: it, close: drop}, nil
}

// scan lit une table de la base de travail ligne à ligne.
func (s *Session) scan(ctx context.Context, table string, n int, each func([]any) error) error {
	rs, err := s.db.QueryContext(ctx, `SELECT * FROM `+qi(table)+` ORDER BY rowid`)
	if err != nil {
		return err
	}
	defer rs.Close()
	row := make([]any, n)
	ptrs := make([]any, n)
	for i := range row {
		ptrs[i] = &row[i]
	}
	for rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return err
		}
		if err := each(row); err != nil {
			return err
		}
	}
	return rs.Err()
}

// Refresh actualise la requête id : relit sa source, rejoue ses étapes et
// charge le résultat dans son tableau (une étape d'annulation). Une
// *ErrPassword signale un mot de passe à demander.
func (s *Session) Refresh(ctx context.Context, env Env, id int64) (canvas.QueryLoad, error) {
	def, err := env.Model.QueryByID(ctx, id)
	if err != nil {
		return canvas.QueryLoad{}, err
	}
	q, err := Decode(def)
	if err != nil {
		return canvas.QueryLoad{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.result(ctx, env, q)
	if err != nil {
		var se *StepError
		if errors.As(err, &se) && needPassword(err) == nil {
			return canvas.QueryLoad{}, fmt.Errorf("requête « %s », %w", q.Name, se)
		}
		return canvas.QueryLoad{}, err
	}
	defer res.Close()
	return env.Model.LoadQuery(ctx, id, res.Table)
}

// RefreshAll actualise toutes les requêtes déjà chargées dans une feuille,
// en une seule étape d'annulation. Les erreurs sont rendues par requête.
func (s *Session) RefreshAll(ctx context.Context, env Env) ([]canvas.QueryLoad, error) {
	defs, err := env.Model.Queries(ctx)
	if err != nil {
		return nil, err
	}
	if !command.InGroup(ctx) {
		ctx = command.WithGroup(ctx, "Actualiser tout")
	}
	var out []canvas.QueryLoad
	var errs []error
	for _, d := range defs {
		if d.Table == "" {
			continue
		}
		l, err := s.Refresh(ctx, env, d.ID)
		if err != nil {
			if needPassword(err) != nil {
				return out, err
			}
			errs = append(errs, err)
			continue
		}
		out = append(out, l)
	}
	return out, errors.Join(errs...)
}

// Refresh actualise une requête avec une base de travail jetable (scripts,
// MCP).
func Refresh(ctx context.Context, env Env, id int64) (canvas.QueryLoad, error) {
	s, err := NewSession()
	if err != nil {
		return canvas.QueryLoad{}, err
	}
	defer s.Close()
	return s.Refresh(ctx, env, id)
}
