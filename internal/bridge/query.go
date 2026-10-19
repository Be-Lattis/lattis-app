package bridge

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/query"
)

// Requêtes d'import (Données › Obtenir des données) et connexions aux
// sources externes : voir docs/decisions/0003.

// pqState garde la base de travail des requêtes du classeur ouvert (cache
// des sources lues pour l'aperçu).
type pqState struct {
	mu   sync.Mutex
	doc  *doc.Document
	sess *query.Session
}

// querySession donne la base de travail du classeur ouvert, recréée quand
// le classeur change.
func (s *DocumentService) querySession() (*doc.Document, *query.Session, error) {
	d, err := s.current()
	if err != nil {
		return nil, nil, err
	}
	s.pq.mu.Lock()
	defer s.pq.mu.Unlock()
	if s.pq.doc != d || s.pq.sess == nil {
		if s.pq.sess != nil {
			s.pq.sess.Close()
			s.pq.sess = nil
		}
		sess, err := query.NewSession()
		if err != nil {
			return nil, nil, err
		}
		s.pq.doc, s.pq.sess = d, sess
	}
	return d, s.pq.sess, nil
}

func (s *DocumentService) closeQuerySession() {
	s.pq.mu.Lock()
	defer s.pq.mu.Unlock()
	if s.pq.sess != nil {
		s.pq.sess.Close()
		s.pq.doc, s.pq.sess = nil, nil
	}
}

// queryEnv : sources lues avec l'accord de l'utilisateur (api.AllowSource).
func (s *DocumentService) queryEnv(d *doc.Document) query.Env {
	return query.Env{Model: d.Model, Secrets: query.DefaultSecrets, Workbook: d.SecretScope(),
		Allow: func(ctx context.Context, a query.Access) error { return api.AllowSource(ctx, host{s}, d, a) }}
}

// grantSources accorde les sources d'une requête écrite dans l'éditeur de
// ce poste : elles ne redemandent pas d'accord à l'actualisation.
func grantSources(ctx context.Context, d *doc.Document, q query.Query) {
	conns := func(name string) (query.Connection, error) {
		def, err := d.Connections(ctx)
		if err != nil {
			return query.Connection{}, err
		}
		for _, c := range def {
			if strings.EqualFold(c.Name, name) {
				return query.DecodeConnection(c)
			}
		}
		return query.Connection{}, fmt.Errorf("connexion « %s » introuvable", name)
	}
	if a, ok, err := query.SourceAccess(q.Source, conns); err == nil && ok {
		api.GrantSource(ctx, d, a)
	}
}

// saveDoc enregistre d dans path.
func saveDoc(ctx context.Context, d *doc.Document, path string) error {
	return d.Save(ctx, path)
}

// secretsOf rend les mots de passe propres au classeur d (voir query.Secrets).
func secretsOf(d *doc.Document) *query.Secrets {
	return query.DefaultSecrets.For(d.SecretScope())
}

// Queries liste les requêtes du classeur.
func (s *DocumentService) Queries(ctx context.Context) ([]query.Query, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	defs, err := d.Queries(ctx)
	if err != nil {
		return nil, err
	}
	out := []query.Query{}
	for _, def := range defs {
		q, err := query.Decode(def)
		if err != nil {
			// Une définition illisible reste listée, pour pouvoir la supprimer.
			q = query.Query{ID: def.ID, Name: def.Name, Steps: []query.Step{}, Table: def.Table, Sheet: def.Sheet}
		}
		out = append(out, q)
	}
	return out, nil
}

// SaveQuery crée ou modifie une requête (sans l'actualiser) et renvoie son
// identifiant.
func (s *DocumentService) SaveQuery(ctx context.Context, q query.Query) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	def, err := query.Encode(q)
	if err != nil {
		return 0, err
	}
	id, err := d.SaveQuery(ctx, def)
	if err != nil {
		return 0, err
	}
	grantSources(ctx, d, q)
	_, err = s.changed(ctx)
	return id, err
}

// DeleteQuery supprime une requête (son tableau reste sur la feuille).
func (s *DocumentService) DeleteQuery(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteQuery(ctx, id) })
}

// NewQueryName propose un nom libre (« Requête1 », ou d'après base).
func (s *DocumentService) NewQueryName(ctx context.Context, base string) (string, error) {
	d, err := s.current()
	if err != nil {
		return "", err
	}
	defs, err := d.Queries(ctx)
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, q := range defs {
		taken[strings.ToLower(q.Name)] = true
	}
	base = strings.TrimSpace(base)
	if base != "" && !taken[strings.ToLower(base)] {
		return base, nil
	}
	if base == "" {
		base = "Requête"
	}
	for n := 1; ; n++ {
		name := fmt.Sprintf("%s%d", base, n)
		if !taken[strings.ToLower(name)] {
			return name, nil
		}
	}
}

// PreviewQuery calcule l'aperçu d'une requête (pas forcément enregistrée)
// après ses upto premières étapes (-1 : toutes). fresh relit les sources.
func (s *DocumentService) PreviewQuery(ctx context.Context, q query.Query, upto int, fresh bool) (query.Preview, error) {
	ctx = query.GuardFrom(ctx, s.sys())
	d, sess, err := s.querySession()
	if err != nil {
		return query.Preview{}, err
	}
	return sess.Preview(ctx, s.queryEnv(d), q, upto, fresh), nil
}

// QueryRefresh est le résultat d'une actualisation : tableaux chargés, ou
// mot de passe à demander.
type QueryRefresh struct {
	Loads        []canvas.QueryLoad  `json:"loads"`
	NeedPassword *query.PasswordNeed `json:"needPassword"`
	Errors       []string            `json:"errors"`
	State        DocState            `json:"state"`
}

func (s *DocumentService) refreshed(ctx context.Context, out QueryRefresh, err error) (QueryRefresh, error) {
	var pe *query.ErrPassword
	switch {
	case errors.As(err, &pe):
		out.NeedPassword = &query.PasswordNeed{Connection: pe.Connection, Account: pe.Account, Message: pe.Message, Stored: pe.Stored, Lattis: pe.Lattis}
	case err != nil:
		for _, e := range strings.Split(err.Error(), "\n") {
			out.Errors = append(out.Errors, e)
		}
	}
	if out.Loads == nil {
		out.Loads = []canvas.QueryLoad{}
	}
	if out.Errors == nil {
		out.Errors = []string{}
	}
	st, serr := s.changed(ctx)
	out.State = st
	return out, serr
}

// RefreshQuery actualise une requête : source relue, étapes rejouées,
// tableau remplacé (une étape d'annulation).
func (s *DocumentService) RefreshQuery(ctx context.Context, id int64) (QueryRefresh, error) {
	ctx = query.GuardFrom(ctx, s.sys())
	d, sess, err := s.querySession()
	if err != nil {
		return QueryRefresh{}, err
	}
	l, err := sess.Refresh(ctx, s.queryEnv(d), id)
	out := QueryRefresh{}
	if err == nil {
		out.Loads = []canvas.QueryLoad{l}
	}
	return s.refreshed(ctx, out, err)
}

// RefreshAllQueries actualise toutes les requêtes chargées (Actualiser
// tout), en une étape d'annulation.
func (s *DocumentService) RefreshAllQueries(ctx context.Context) (QueryRefresh, error) {
	ctx = query.GuardFrom(ctx, s.sys())
	d, sess, err := s.querySession()
	if err != nil {
		return QueryRefresh{}, err
	}
	ls, err := sess.RefreshAll(ctx, s.queryEnv(d))
	return s.refreshed(ctx, QueryRefresh{Loads: ls}, err)
}

// ForgetQueryCache vide le cache des sources de l'aperçu.
func (s *DocumentService) ForgetQueryCache() {
	s.pq.mu.Lock()
	sess := s.pq.sess
	s.pq.mu.Unlock()
	if sess != nil {
		sess.Forget()
	}
}

// Connections liste les connexions du classeur.
func (s *DocumentService) Connections(ctx context.Context) ([]query.Connection, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	defs, err := d.Connections(ctx)
	if err != nil {
		return nil, err
	}
	out := []query.Connection{}
	for _, def := range defs {
		c, err := query.DecodeConnection(def)
		if err != nil {
			continue
		}
		c.Remembered = secretsOf(d).Remembered(c.Account())
		out = append(out, c)
	}
	return out, nil
}

func (s *DocumentService) connection(ctx context.Context, name string) (query.Connection, error) {
	cs, err := s.Connections(ctx)
	if err != nil {
		return query.Connection{}, err
	}
	for _, c := range cs {
		if strings.EqualFold(c.Name, name) {
			return c, nil
		}
	}
	return query.Connection{}, fmt.Errorf("connexion « %s » introuvable", name)
}

// SaveConnection crée ou modifie une connexion ; password (facultatif) est
// gardé pour la session, et dans le trousseau du système si remember. Il
// n'est jamais écrit dans le classeur.
func (s *DocumentService) SaveConnection(ctx context.Context, c query.Connection, password string, remember bool) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	if err := checkLattisConnection(c); err != nil {
		return 0, err
	}
	def, err := query.EncodeConnection(c)
	if err != nil {
		return 0, err
	}
	id, err := d.SaveConnection(ctx, def)
	if err != nil {
		return 0, err
	}
	if password != "" {
		if err := secretsOf(d).Set(c.Account(), password, remember); err != nil {
			return id, fmt.Errorf("connexion enregistrée, mais le trousseau du système a refusé le mot de passe (gardé pour cette session) : %w", err)
		}
	}
	_, err = s.changed(ctx)
	return id, err
}

// DeleteConnection supprime une connexion qu'aucune requête n'utilise.
func (s *DocumentService) DeleteConnection(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteConnection(ctx, id) })
}

// TestConnection essaie une connexion (password vide : celui qui est
// connu). Renvoie "" si elle répond, sinon le message d'erreur.
func (s *DocumentService) TestConnection(ctx context.Context, c query.Connection, password string, remember bool) (string, error) {
	ctx = query.GuardFrom(ctx, s.sys())
	if c.Kind == "" {
		c.Kind = query.KindPostgres
	}
	d, err := s.current()
	if err != nil {
		return "", err
	}
	if err := query.TestConnection(ctx, secretsOf(d), c, password, remember); err != nil {
		return err.Error(), nil
	}
	return "", nil
}

// SetConnectionPassword essaie le mot de passe d'une connexion du classeur
// et le garde s'il est accepté (pour la session, et dans le trousseau si
// remember). Renvoie "" s'il est accepté, sinon le message d'erreur.
func (s *DocumentService) SetConnectionPassword(ctx context.Context, name, password string, remember bool) (string, error) {
	c, err := s.connection(ctx, name)
	if err != nil {
		return "", err
	}
	return s.TestConnection(ctx, c, password, remember)
}

// AllowConnectionPassword autorise ce classeur à se servir du mot de passe
// enregistré sur ce poste pour une connexion (« Autoriser », retenu).
func (s *DocumentService) AllowConnectionPassword(ctx context.Context, name string) error {
	c, err := s.connection(ctx, name)
	if err != nil {
		return err
	}
	d, err := s.current()
	if err != nil {
		return err
	}
	return secretsOf(d).Grant(c.Account())
}

// ForgetConnectionPassword oublie le mot de passe d'une connexion (session
// et trousseau).
func (s *DocumentService) ForgetConnectionPassword(ctx context.Context, name string) error {
	c, err := s.connection(ctx, name)
	if err != nil {
		return err
	}
	d, err := s.current()
	if err != nil {
		return err
	}
	return secretsOf(d).Forget(c.Account())
}

// TableList liste les tables et vues d'une connexion (navigateur).
// needPassword est rempli si le serveur demande un mot de passe.
type TableList struct {
	Tables       []query.DBTable     `json:"tables"`
	NeedPassword *query.PasswordNeed `json:"needPassword"`
}

// ConnectionTables liste les tables d'une connexion du classeur
// (PostgreSQL, MySQL, SQL Server ; vide pour une connexion Web).
func (s *DocumentService) ConnectionTables(ctx context.Context, name string) (TableList, error) {
	ctx = query.GuardFrom(ctx, s.sys())
	c, err := s.connection(ctx, name)
	if err != nil {
		return TableList{}, err
	}
	d, err := s.current()
	if err != nil {
		return TableList{}, err
	}
	ts, err := query.DBTables(ctx, secretsOf(d), c)
	var pe *query.ErrPassword
	if errors.As(err, &pe) {
		return TableList{Tables: []query.DBTable{}, NeedPassword: &query.PasswordNeed{Connection: pe.Connection, Account: pe.Account, Message: pe.Message, Stored: pe.Stored, Lattis: pe.Lattis}}, nil
	}
	return TableList{Tables: ts}, err
}

// FileTables liste les tables et vues d'un fichier de base (kind : "sqlite").
func (s *DocumentService) FileTables(ctx context.Context, kind, path string) ([]query.DBTable, error) {
	ctx = query.GuardFrom(ctx, s.sys())
	return query.FileTables(ctx, kind, path)
}

// TableSQL écrit la requête qui lit une table entière (kind : genre de la
// connexion ou du fichier de base, pour les guillemets des identifiants).
func (s *DocumentService) TableSQL(kind, schema, table string) string {
	return query.TableSQL(kind, schema, table)
}

// PickDataFile demande un fichier source (kind : "csv", "xlsx", "json",
// "parquet" ou "sqlite") ; "" si l'utilisateur annule.
func (s *DocumentService) PickDataFile(kind string) (string, error) {
	o := FileDialog{Title: "Obtenir des données"}
	switch kind {
	case "xlsx":
		o.Filters = []FileFilter{{"Classeur Excel", "*.xlsx;*.xlsm"}}
	case "json":
		o.Filters = []FileFilter{{"JSON", "*.json;*.ndjson;*.jsonl;*.geojson"}}
	case "parquet":
		o.Filters = []FileFilter{{"Parquet", "*.parquet;*.parq"}}
	case "sqlite":
		o.Filters = []FileFilter{{"Base SQLite", "*.db;*.sqlite;*.sqlite3;*.db3"}}
	default:
		o.Filters = []FileFilter{{"Texte ou CSV", "*.csv;*.tsv;*.txt"}}
	}
	o.Filters = append(o.Filters, FileFilter{"Tous les fichiers", "*.*"})
	p, err := s.sys().OpenFile(o)
	if p != "" && err == nil {
		// Choisi ici : l'aperçu et l'actualisation ne redemandent pas.
		if d, derr := s.current(); derr == nil {
			api.GrantSource(context.Background(), d, query.Access{Path: p})
		}
	}
	return p, err
}

// ExcelTables liste les tableaux d'un classeur .xlsx (navigateur).
func (s *DocumentService) ExcelTables(ctx context.Context, path string) ([]query.XLSXTable, error) {
	ctx = query.GuardFrom(ctx, s.sys())
	return query.XLSXTables(ctx, path)
}

// WorkbookTables liste les tableaux du classeur, par nom (sources et
// fusions).
func (s *DocumentService) WorkbookTables(ctx context.Context) ([]string, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	names, err := d.TableNames(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(names))
	for n := range names {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

// FileBase donne le nom d'un fichier sans dossier ni extension (nom
// proposé pour une requête).
func (s *DocumentService) FileBase(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// ReferenceQuery pose le résultat d'une requête de l'onglet SQL sur une
// nouvelle feuille, comme un tableau lié (Référencer) : une requête du
// classeur (source « requête SQL du classeur ») nommée name (vide :
// « Requête1 »…) est créée puis chargée ; le tableau s'actualise ensuite
// comme les sources externes. Les adresses Cell[A1] sans feuille se lisent
// sur la feuille affichée. headers : en-têtes affichés des colonnes
// (columns : noms rendus par la requête), repris par des étapes Renommer.
func (s *DocumentService) ReferenceQuery(ctx context.Context, q, name string, columns, headers []string) (QueryRefresh, error) {
	ctx = query.GuardFrom(ctx, s.sys())
	d, err := s.current()
	if err != nil {
		return QueryRefresh{}, err
	}
	if strings.TrimSpace(q) == "" {
		return QueryRefresh{}, errors.New("requête SQL vide")
	}
	home := ""
	if l, err := d.Layout(ctx, 0); err == nil {
		for _, sh := range l.Sheets {
			if sh.ID == s.activeSheet() {
				home = sh.Name
			}
		}
	}
	if name, err = s.NewQueryName(ctx, name); err != nil {
		return QueryRefresh{}, err
	}
	id, err := s.SaveQuery(ctx, query.Query{Name: name, Source: query.Source{Kind: query.SourceSQL, SQL: q, Sheet: home}, Steps: renameSteps(columns, headers)})
	if err != nil {
		return QueryRefresh{}, err
	}
	out, err := s.RefreshQuery(ctx, id)
	if err == nil && len(out.Loads) == 0 {
		// Rien de chargé : la requête vide n'est pas gardée.
		d.DeleteQuery(ctx, id)
		out.State, err = s.changed(ctx)
	}
	return out, err
}

// renameSteps renomme les colonnes dont l'en-tête affiché diffère du nom
// rendu par la requête (noms en double ou déjà pris : laissés tels quels).
func renameSteps(columns, headers []string) []query.Step {
	steps := []query.Step{}
	if len(columns) != len(headers) {
		return steps
	}
	taken := map[string]int{}
	for _, c := range columns {
		taken[strings.ToLower(c)]++
	}
	for i, c := range columns {
		h := strings.TrimSpace(headers[i])
		if h == "" || h == c || taken[strings.ToLower(c)] > 1 || (taken[strings.ToLower(h)] > 0 && !strings.EqualFold(h, c)) {
			continue
		}
		taken[strings.ToLower(h)]++
		taken[strings.ToLower(c)]--
		steps = append(steps, query.Step{Op: query.OpRename, Column: c, Name: h})
	}
	return steps
}
