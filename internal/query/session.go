package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/ncruces/go-sqlite3"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/store"
)

// Env donne au moteur ce qu'il lit hors de la base de travail : le classeur
// (tableaux, requêtes, connexions) et les mots de passe.
type Env struct {
	Model   *canvas.Model
	Secrets *Secrets
	// Workbook désigne le classeur pour ses mots de passe
	// (doc.Document.SecretScope) : ceux d'un autre classeur ne servent pas.
	Workbook string
	// Allow vérifie chaque accès d'une source hors du classeur (hôte joint,
	// fichier lu) avant qu'il ait lieu ; nil : tout est permis.
	Allow AllowFunc
}

func (e Env) secrets() *Secrets {
	s := e.Secrets
	if s == nil {
		s = DefaultSecrets
	}
	if e.Workbook == "" {
		return s
	}
	return s.For(e.Workbook)
}

// Session est une base SQLite de travail (fichier temporaire) où les
// sources sont chargées et les étapes exécutées. Les sources lues pour
// l'aperçu y restent en cache jusqu'à Forget ; une actualisation relit
// toujours ses sources. Une session sert un classeur ouvert.
type Session struct {
	mu    sync.Mutex
	db    *sql.DB
	path  string
	n     int
	cache map[string]loaded
}

type loaded struct {
	table     string
	cols      []Column
	truncated bool
}

// NewSession ouvre une base de travail.
func NewSession() (*Session, error) {
	f, err := os.CreateTemp("", "lattis-requetes-*.db")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	f.Close()
	db, err := store.OpenDB("file:"+path, "requêtes", func(c *sqlite3.Conn) error {
		for _, p := range []string{"PRAGMA journal_mode=OFF", "PRAGMA synchronous=OFF", "PRAGMA temp_store=MEMORY", "PRAGMA cache_size=-65536"} {
			if err := c.Exec(p); err != nil {
				return err
			}
		}
		if err := canvas.Register(c); err != nil {
			return err
		}
		return registerFuncs(c)
	})
	if err != nil {
		os.Remove(path)
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &Session{db: db, path: path, cache: map[string]loaded{}}, nil
}

// Close ferme la base de travail et supprime son fichier.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.db.Close()
	os.Remove(s.path)
	return err
}

// Forget vide le cache des sources (Actualiser l'aperçu).
func (s *Session) Forget() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forget()
}

func (s *Session) forget() {
	for _, l := range s.cache {
		s.db.Exec(`DROP TABLE IF EXISTS ` + qi(l.table))
	}
	s.cache = map[string]loaded{}
}

func (s *Session) name(prefix string) string {
	s.n++
	return prefix + strconv.Itoa(s.n)
}

// registerFuncs déclare les fonctions propres aux étapes.
func registerFuncs(c *sqlite3.Conn) error {
	// slt_split(texte, délimiteur, k) : k-ième morceau (à partir de 1), NULL
	// s'il n'y en a pas autant.
	return c.CreateFunction("slt_split", 3, sqlite3.DETERMINISTIC|sqlite3.INNOCUOUS, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		if arg[0].Type() == sqlite3.NULL {
			ctx.ResultNull()
			return
		}
		parts := strings.Split(arg[0].Text(), arg[1].Text())
		k := int(arg[2].Int64())
		if k < 1 || k > len(parts) {
			ctx.ResultNull()
			return
		}
		ctx.ResultText(parts[k-1])
	})
}

// run est une exécution : compilation d'une ou plusieurs requêtes (citées
// les unes par les autres) sur la base de travail.
type run struct {
	s     *Session
	env   Env
	ctx   context.Context
	limit int  // lignes lues par source (0 : toutes)
	fresh bool // relire les sources même en cache
	stack []string
	// truncated : une source a été coupée à limit lignes.
	truncated bool
	// steps : étapes de la requête dont la source SQL se charge (JSON en
	// minuscules), pour cites.
	steps string
}

// cites dit si les étapes de la requête en cours citent la colonne name
// (mot entier) : une source SQL garde alors ce nom tel quel.
func (r *run) cites(name string) bool {
	if r.steps == "" || name == "" {
		return false
	}
	re, err := regexp.Compile(`(^|[^\pL\pN_])` + regexp.QuoteMeta(strings.ToLower(name)) + `($|[^\pL\pN_])`)
	return err == nil && re.MatchString(r.steps)
}

// errLimit arrête la lecture d'une source à la limite de l'aperçu.
var errLimit = errors.New("limite atteinte")

// StepError est une erreur de compilation ou d'exécution d'une étape
// (Step : -1 pour la source).
type StepError struct {
	Step int
	Err  error
}

func (e *StepError) Error() string {
	if e.Step < 0 {
		return "source : " + e.Err.Error()
	}
	return fmt.Sprintf("étape %d : %s", e.Step+1, e.Err)
}

func (e *StepError) Unwrap() error { return e.Err }

// compile compile la source et les upto premières étapes de q. En cas
// d'erreur, le pipeline rendu s'arrête à la dernière étape réussie (nil si
// la source échoue).
func (r *run) compile(q Query, upto int) (*pipeline, error) {
	for _, n := range r.stack {
		if strings.EqualFold(n, q.Name) {
			return nil, &StepError{-1, fmt.Errorf("la requête « %s » se cite elle-même", q.Name)}
		}
	}
	r.stack = append(r.stack, q.Name)
	defer func() { r.stack = r.stack[:len(r.stack)-1] }()
	if q.Source.Kind == SourceSQL {
		b, _ := json.Marshal(q.Steps)
		saved := r.steps
		r.steps = strings.ToLower(string(b))
		defer func() { r.steps = saved }()
	}
	from, cols, err := r.from(q.Source)
	if err != nil {
		return nil, &StepError{-1, err}
	}
	prefix := r.s.name("q") + "_s"
	p := &pipeline{}
	p.push(prefix+"0", "SELECT * FROM "+from, cols)
	if upto < 0 || upto > len(q.Steps) {
		upto = len(q.Steps)
	}
	for i, st := range q.Steps[:upto] {
		if err := r.ctx.Err(); err != nil {
			return p, err
		}
		sel, cols, err := r.step(p, st)
		if err != nil {
			return p, &StepError{i, err}
		}
		p.push(prefix+strconv.Itoa(i+1), sel, cols)
	}
	return p, nil
}

// from donne l'expression SQL (table ou sous-requête) d'une source.
func (r *run) from(src Source) (string, []Column, error) {
	if src.Kind == SourceQuery {
		def, err := r.env.Model.QueryByName(r.ctx, src.Query)
		if err != nil {
			return "", nil, err
		}
		q, err := Decode(def)
		if err != nil {
			return "", nil, err
		}
		p, err := r.compile(q, -1)
		if err != nil {
			var se *StepError
			if errors.As(err, &se) {
				return "", nil, fmt.Errorf("requête « %s », %w", q.Name, se)
			}
			return "", nil, err
		}
		return "(" + p.sql() + ")", p.cols, nil
	}
	l, err := r.load(src)
	if err != nil {
		return "", nil, err
	}
	return qi(l.table), l.cols, nil
}

// load charge une source dans la base de travail (ou la reprend du cache).
func (r *run) load(src Source) (loaded, error) {
	key, _ := json.Marshal(src)
	k := string(key) + "|" + strconv.Itoa(r.limit)
	if l, ok := r.s.cache[k]; ok && !r.fresh && !workbookSource(src.Kind) {
		r.truncated = r.truncated || l.truncated
		return l, nil
	}
	if old, ok := r.s.cache[k]; ok {
		r.s.db.ExecContext(r.ctx, `DROP TABLE IF EXISTS `+qi(old.table))
		delete(r.s.cache, k)
	}
	if a, ok, err := SourceAccess(src, r.connection); err != nil {
		return loaded{}, err
	} else if ok {
		if err := r.allow(a); err != nil {
			return loaded{}, err
		}
	}
	var l loaded
	var err error
	switch src.Kind {
	case SourceCSV:
		l, err = r.loadCSV(src)
	case SourceXLSX:
		l, err = r.loadXLSX(src)
	case SourceTable:
		l, err = r.loadTable(src)
	case SourcePostgres, SourceMySQL, SourceSQLServer, SourceLattis:
		l, err = r.loadDB(src)
	case SourceSQLite:
		l, err = r.loadFileDB(src)
	case SourceJSON:
		l, err = r.loadJSON(src)
	case SourceParquet:
		l, err = r.loadParquet(src)
	case SourceWeb:
		l, err = r.loadWeb(src)
	case SourceSQL:
		l, err = r.loadSQL(src)
	case SourceFormula:
		l, err = r.loadFormula(src)
	case "":
		err = errors.New("source manquante")
	default:
		err = fmt.Errorf("source %q inconnue", src.Kind)
	}
	if err != nil {
		return l, err
	}
	r.truncated = r.truncated || l.truncated
	if !workbookSource(src.Kind) {
		r.s.cache[k] = l
	}
	return l, nil
}
