// Package workbooktest fournit un Serveur Lattis en mémoire
// (workbook.Server) pour les essais : il suit les règles de l'API SQL du
// serveur (be-lattis-server, schema/0003_workbooks.sql) qui comptent pour
// la publication et l'ouverture des classeurs figés — brouillon,
// publication, republication en version de travail, droits par rôle
// fourni, noms valides, notifications *version:<n>.
package workbooktest

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// ErrDown est l'erreur d'un serveur injoignable (Fake.SetDown).
var ErrDown = errors.New("dial tcp 127.0.0.1:5432: connexion refusée")

// ErrPrivilege imite le refus de PostgreSQL (42501).
type ErrPrivilege struct{ Msg string }

func (e ErrPrivilege) Error() string { return "ERROR: " + e.Msg + " (SQLSTATE 42501)" }

// SQLState rend le code de l'erreur (comme pgconn.PgError).
func (e ErrPrivilege) SQLState() string { return "42501" }

// Fake est un serveur en mémoire, partagé par plusieurs utilisateurs
// (As).
type Fake struct {
	mu        sync.Mutex
	books     map[string]*book
	down      bool
	listeners map[string][]chan string
	// Calls compte les appels de l'API SQL par nom (essais).
	Calls map[string]int
	// Fail, s'il est fixé, fait échouer l'appel de ce nom (save_meta,
	// finish_publish…) : la transaction est défaite.
	Fail string
	// xid : dernière transaction validée (lattis_xid, sync_point).
	xid int64
	// levels : niveau imposé à un utilisateur sur un tableau (SetLevel).
	levels map[string]string
	// Clock donne la date des modifications (lattis_updated_at).
	Clock func() time.Time
	// acls : droits de chaque classeur (rights.go) ; users : utilisateurs
	// connus (As).
	acls  map[string]*bookACL
	users map[string]bool
	// Legacy imite un serveur sans la migration 0008 : changes_since sans
	// skip (ses propres écritures reviennent), pas de since_covered.
	Legacy bool
	// Before09 imite un serveur sans la migration 0009 : create_table sans
	// feuille, invite sans rôles personnalisés, pas de workbook_members,
	// my_capabilities sans sheet_id, read_filtered ni write_filtered.
	Before09 bool
	// pending : adresses invitées inconnues, comptes en attente (0009).
	pending map[string]bool
	// purged : plus grand xid des pierres tombales purgées (sync_horizon) ;
	// purgeXid : transaction de la dernière purge (migration 0010).
	purged, purgeXid int64
	// Before10 imite un serveur sans la migration 0010 : témoin de
	// rechargement par point de contrôle (xmax), pas d'instantané.
	Before10 bool
	// checks : filtres d'écriture (WITH CHECK) imposés par Forbid.
	checks []check
	// held : transactions en cours (Hold) ; sync_point ne les dépasse pas.
	held []int64
	// Before12 imite un serveur sans la migration 0012 : pas d'équipes
	// (list_teams absent, grant_role pour une personne seulement).
	Before12 bool
	// Before26 imite un serveur sans la migration 0026 : pas
	// d'insert_column (ErrUnsupported, comme pgWriter sur 42883), set_key
	// ne notifie que « *meta ».
	Before26 bool
	// quiet : notifications écartées (Quiet).
	quiet func(channel, payload string) bool
	// teams : équipes Lattis (CreateTeam) ; serverOwners : propriétaires du
	// serveur (ServerOwner), qui voient les membres des équipes.
	teams        []*fteam
	serverOwners map[string]bool
	// dataStore : données du Data Lake (data.go).
	dataStore *fdata
	// Creators : créateurs de classeurs (rôle de serveur) ; nil : tout le
	// monde publie. Les autres reçoivent 42501 de create_workbook.
	Creators map[string]bool
	// pipelines : pipelines qui alimentent un tableau (AddPipeline).
	pipelines []*fpipeline
	// sources : sources poussées du Data Lake (source.go) ;
	// NoPushedSources imite un serveur sans la migration 0027 (fonctions
	// inconnues, 42883). Envoi (Pusher) : PushBusy répond 409, PushSlow 202
	// (le bilan se lit ensuite par PushStatus), MaxPush (octets) 413 ;
	// LastPush garde le dernier CSV reçu.
	sources         []*fsource
	NoPushedSources bool
	PushBusy        bool
	PushSlow        bool
	MaxPush         int
	LastPush        string
	// Before28 imite un serveur sans la migration 0028 : pas
	// d'update_pushed_source (42883), 202 sans run_id, GET ?run= ignoré,
	// catalogue sans source.kind.
	Before28 bool
}

// check refuse (42501) l'écriture de value dans la colonne col d'un
// tableau.
type check struct {
	workbook, table, col string
	value                any
}

func (f *Fake) call(name string) error {
	f.Calls[name]++
	if f.Fail == name {
		return fmt.Errorf("échec simulé de %s", name)
	}
	return nil
}

type book struct {
	id, slug, title, mode, state string
	version                      int
	schema, staging              string
	roles                        map[string][]string // utilisateur → rôles fournis
	schemas                      map[string]*schema
	// kind : « source » pour le classeur technique d'une source poussée
	// (absent de my_workbooks) ; vide : classeur ordinaire.
	kind string
}

type schema struct {
	tables  []*table
	meta    map[string][]byte // kind/key
	scripts []workbook.Script
}

type table struct {
	id      string
	name    string
	localID int64
	sheet   string // sheet_id (0009)
	cols    []workbook.RemoteColumn
	rows    []row
	tombs   []tomb
	// key : colonne clé (lattis.set_key) ; indexes : index posés par
	// create_index (structure.go).
	key     string
	indexes []findex
}

type row struct {
	id, pos, author string
	vals            []any // par colonne, valeurs PostgreSQL (ToRemote)
	// xid : transaction de la dernière modification ; by, device, at : son
	// auteur (rôle et poste) et sa date ; other : dernière transaction d'un
	// autre auteur avant elle (lattis_other_xid, 0 : NULL).
	xid    int64
	by     string
	device string
	at     time.Time
	other  int64
}

// tomb est une pierre tombale (lattis.tombstones).
type tomb struct {
	id  string
	xid int64
	by  string
	at  time.Time
}

// New crée un serveur vide.
func New() *Fake {
	return &Fake{books: map[string]*book{}, listeners: map[string][]chan string{}, Calls: map[string]int{}, xid: 1000,
		levels: map[string]string{}, Clock: time.Now}
}

// As rend le serveur vu par un utilisateur (rôle PostgreSQL), sans
// identifiant de poste.
func (f *Fake) As(user string) workbook.Server { return f.AsDevice(user, "") }

// AsDevice rend le serveur vu par un utilisateur depuis un poste
// (SET lattis.device) : deux postes du même utilisateur sont deux auteurs.
func (f *Fake) AsDevice(user, device string) workbook.Server {
	f.mu.Lock()
	if f.users == nil {
		f.users = map[string]bool{}
	}
	f.users[user] = true
	f.mu.Unlock()
	return &session{f: f, user: user, device: device}
}

// Hold imite une transaction d'écriture longue, commencée maintenant :
// tant qu'elle dure, sync_point (pg_snapshot_xmin) ne la dépasse pas.
// release la termine.
func (f *Fake) Hold() (release func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.xid++
	x := f.xid
	f.held = append(f.held, x)
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.held = slices.DeleteFunc(f.held, func(h int64) bool { return h == x })
	}
}

// SetDown rend le serveur injoignable (ou de nouveau joignable) ; les
// écoutes en cours sont coupées.
func (f *Fake) SetDown(down bool) {
	f.mu.Lock()
	f.down = down
	ls := f.listeners
	if down {
		f.listeners = map[string][]chan string{}
	}
	f.mu.Unlock()
	if down {
		for _, cs := range ls {
			for _, c := range cs {
				close(c)
			}
		}
	}
}

// Grant donne un rôle fourni (reader, owner…) sur un classeur.
func (f *Fake) Grant(workbookID, user, role string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := f.books[workbookID]
	if b == nil {
		return errors.New("classeur inconnu")
	}
	if !slices.Contains(b.roles[user], role) {
		b.roles[user] = append(b.roles[user], role)
	}
	return nil
}

// SetTitle renomme un classeur (titre relu par my_capabilities ; pas de
// notification).
func (f *Fake) SetTitle(workbookID, title string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if b := f.books[workbookID]; b != nil {
		b.title = title
	}
}

// Rows donne les lignes d'un tableau de la version publiée (essais) :
// lattis_id, lattis_pos, puis les valeurs.
func (f *Fake) Rows(workbookID, name string) [][]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := f.books[workbookID]
	if b == nil {
		return nil
	}
	s := b.schemas[b.schema]
	for _, t := range s.tables {
		if t.name == name {
			var out [][]any
			for _, r := range t.sorted() {
				out = append(out, append([]any{r.id, r.pos}, r.vals...))
			}
			return out
		}
	}
	return nil
}

// Book décrit un classeur (essais).
func (f *Fake) Book(id string) (state, schema string, version int, tables []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := f.books[id]
	if b == nil {
		return "", "", 0, nil
	}
	for _, t := range b.schemas[b.schema].tables {
		tables = append(tables, t.name)
	}
	return b.state, b.schema, b.version, tables
}

// Meta lit un objet de mise en page de la version publiée (essais).
func (f *Fake) Meta(id, kind string) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := f.books[id]
	if b == nil {
		return nil
	}
	for k, v := range b.schemas[b.schema].meta {
		if strings.HasPrefix(k, kind+"/") {
			return v
		}
	}
	return nil
}

func (t *table) sorted() []row {
	out := slices.Clone(t.rows)
	slices.SortStableFunc(out, func(a, b row) int {
		if c := strings.Compare(a.pos, b.pos); c != 0 {
			return c
		}
		return strings.Compare(a.id, b.id)
	})
	return out
}

func (s *schema) clone() *schema {
	c := &schema{meta: map[string][]byte{}, scripts: slices.Clone(s.scripts)}
	for k, v := range s.meta {
		c.meta[k] = v
	}
	for _, t := range s.tables {
		tc := *t
		tc.cols = slices.Clone(t.cols)
		tc.rows = slices.Clone(t.rows)
		tc.tombs = slices.Clone(t.tombs)
		tc.indexes = slices.Clone(t.indexes)
		for i := range tc.rows {
			tc.rows[i].vals = slices.Clone(t.rows[i].vals)
		}
		c.tables = append(c.tables, &tc)
	}
	return c
}

func (b *book) clone() *book {
	c := *b
	c.roles = map[string][]string{}
	for k, v := range b.roles {
		c.roles[k] = slices.Clone(v)
	}
	c.schemas = map[string]*schema{}
	for k, v := range b.schemas {
		c.schemas[k] = v.clone()
	}
	return &c
}

func (f *Fake) snapshot() map[string]*book {
	out := map[string]*book{}
	for k, v := range f.books {
		out[k] = v.clone()
	}
	return out
}

type session struct {
	f            *Fake
	user, device string
}

func (s *session) begin(name string) error {
	s.f.Calls[name]++
	if s.f.down {
		return ErrDown
	}
	return nil
}

// roles : rôles fournis de l'utilisateur, en personne ou par une équipe.
func (s *session) roles(b *book) []string {
	out := b.roles[s.user]
	for _, r := range s.f.teamRoles(b, s.user) {
		if r.builtin != "" && !slices.Contains(out, r.builtin) {
			out = append(slices.Clip(out), r.builtin)
		}
	}
	return out
}

func (s *session) has(b *book, want ...string) bool {
	for _, r := range s.roles(b) {
		if slices.Contains(want, r) {
			return true
		}
	}
	return false
}

func (s *session) Workbooks(ctx context.Context) ([]workbook.Listing, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("my_workbooks"); err != nil {
		return nil, err
	}
	out := []workbook.Listing{}
	for _, b := range s.f.books {
		if b.kind == "source" || len(s.roles(b)) == 0 || (b.state == "draft" && !s.has(b, "owner", "designer")) {
			continue
		}
		out = append(out, workbook.Listing{ID: b.id, Title: b.title, Mode: b.mode, State: b.state, Version: b.version, Schema: b.schema, Roles: slices.Clone(s.roles(b))})
	}
	slices.SortFunc(out, func(a, b workbook.Listing) int { return cmp.Compare(a.Title, b.Title) })
	return out, nil
}

func (s *session) caps(id string) (workbook.Capabilities, error) {
	b := s.f.books[id]
	if b == nil {
		return workbook.Capabilities{}, errors.New("classeur inconnu")
	}
	if len(s.f.userRoles(b, s.user)) == 0 {
		return workbook.Capabilities{}, ErrPrivilege{"aucun accès à ce classeur"}
	}
	roles := slices.Clone(s.roles(b))
	if roles == nil {
		roles = []string{}
	}
	c := workbook.Capabilities{WorkbookID: b.id, Title: b.title, Mode: b.mode, Version: b.version, Schema: b.schema, Channel: "wb_" + b.slug,
		Roles: roles, CanDesign: s.has(b, "owner", "designer"), CanManage: s.has(b, "owner"), Tables: map[string]workbook.TableAccess{},
		CustomRoles: s.customRoles(b), Restrictions: s.myRestrictions(b)}
	for _, t := range b.schemas[b.schema].tables {
		lv, filt := s.levelOf(b, t)
		a := workbook.TableAccess{Name: t.name, Level: lv, Filtered: filt}
		if !s.f.Before09 {
			rf, wf := s.filters(b, t)
			a.SheetID, a.ReadFiltered, a.WriteFiltered = t.sheet, &rf, &wf
		}
		c.Tables[t.id] = a
	}
	return c, nil
}

// level est le niveau de l'utilisateur sur un tableau : lecteur dans un
// classeur figé ; sinon l'union des niveaux de ses rôles (fournis :
// éditeur pour propriétaire, concepteur et éditeur ; participant ; lecteur ;
// personnalisés : niveaux posés), sauf niveau imposé (SetLevel).
func (s *session) level(b *book, t *table) string {
	lv, _ := s.levelOf(b, t)
	return lv
}

func (s *session) Capabilities(ctx context.Context, id string) (workbook.Capabilities, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("my_capabilities"); err != nil {
		return workbook.Capabilities{}, err
	}
	return s.caps(id)
}

func (s *session) Publish(ctx context.Context, fn func(workbook.Publisher) error) error {
	s.f.mu.Lock()
	if err := s.begin("publish"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	saved := s.f.snapshot()
	p := &publisher{s: s}
	err := fn(p)
	if err != nil {
		s.f.books = saved
		s.f.mu.Unlock()
		return err
	}
	notes := p.notes
	s.f.xid++
	s.f.mu.Unlock()
	for _, n := range notes {
		s.f.notify(n[0], n[1])
	}
	return nil
}

// Quiet écarte les notifications pour lesquelles fn rend vrai (essais :
// ne garder que « *structure:<id> », par exemple) ; nil : toutes passent.
func (f *Fake) Quiet(fn func(channel, payload string) bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quiet = fn
}

func (f *Fake) notify(channel, payload string) {
	f.mu.Lock()
	cs := slices.Clone(f.listeners[channel])
	quiet := f.quiet
	f.mu.Unlock()
	if quiet != nil && quiet(channel, payload) {
		return
	}
	for _, c := range cs {
		select {
		case c <- payload:
		default:
		}
	}
}

func (s *session) Read(ctx context.Context, fn func(workbook.Reader) error) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("read"); err != nil {
		return err
	}
	return fn(&reader{s: s})
}

func (s *session) Listen(ctx context.Context, channel string, ready func(), fn func(string)) error {
	s.f.mu.Lock()
	if err := s.begin("listen"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	c := make(chan string, 16)
	s.f.listeners[channel] = append(s.f.listeners[channel], c)
	s.f.mu.Unlock()
	defer func() {
		s.f.mu.Lock()
		s.f.listeners[channel] = slices.DeleteFunc(s.f.listeners[channel], func(x chan string) bool { return x == c })
		s.f.mu.Unlock()
	}()
	if ready != nil {
		ready()
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case p, ok := <-c:
			if !ok {
				return errors.New("connexion au serveur perdue")
			}
			fn(p)
		}
	}
}

// --- Publication ------------------------------------------------------------

type publisher struct {
	s     *session
	notes [][2]string
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func (p *publisher) CreateWorkbook(ctx context.Context, title, mode string) (string, error) {
	if err := p.s.f.call("create_workbook"); err != nil {
		return "", err
	}
	if !p.s.canCreate() {
		return "", ErrPrivilege{"permission denied: seuls les créateurs de classeurs publient"}
	}
	if mode != workbook.Frozen && mode != workbook.Collaborative {
		return "", errors.New("mode : frozen ou collaborative")
	}
	if strings.TrimSpace(title) == "" {
		return "", errors.New("titre manquant")
	}
	slug := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(title), "_"), "_")
	if slug == "" {
		slug = "classeur"
	}
	base := slug
	for n := 2; p.slugTaken(slug); n++ {
		slug = base + "_" + strconv.Itoa(n)
	}
	id := uuid.NewString()
	b := &book{id: id, slug: slug, title: strings.TrimSpace(title), mode: mode, state: "draft", version: 1, schema: "wb_" + slug,
		roles: map[string][]string{p.s.user: {"owner"}}, schemas: map[string]*schema{}}
	b.schemas[b.schema] = &schema{meta: map[string][]byte{}}
	p.s.f.books[id] = b
	return id, nil
}

func (p *publisher) slugTaken(slug string) bool {
	for _, b := range p.s.f.books {
		if b.slug == slug {
			return true
		}
	}
	return false
}

func (p *publisher) book(id string, roles ...string) (*book, error) {
	b := p.s.f.books[id]
	if b == nil {
		return nil, errors.New("classeur inconnu")
	}
	if !p.s.has(b, roles...) {
		return nil, ErrPrivilege{fmt.Sprintf("droits insuffisants sur le classeur « %s »", b.title)}
	}
	return b, nil
}

// target est le schéma où l'on publie (lattis.publish_target).
func (b *book) target() (string, error) {
	switch {
	case b.state == "draft":
		return b.schema, nil
	case b.state == "republishing":
		return b.staging, nil
	case b.mode == workbook.Collaborative:
		return b.schema, nil
	}
	return "", errors.New("classeur figé publié : republier pour le modifier")
}

func (p *publisher) Republish(ctx context.Context, id string) (string, error) {
	if err := p.s.f.call("republish"); err != nil {
		return "", err
	}
	b, err := p.book(id, "owner", "designer")
	if err != nil {
		return "", err
	}
	if b.mode != workbook.Frozen || b.state != "published" {
		return "", errors.New("seul un classeur figé publié se republie")
	}
	b.staging = fmt.Sprintf("wb_%s__v%d", b.slug, b.version+1)
	b.schemas[b.staging] = &schema{meta: map[string][]byte{}}
	b.state = "republishing"
	return b.staging, nil
}

func (p *publisher) Capabilities(ctx context.Context, id string) (workbook.Capabilities, error) {
	if err := p.s.f.call("my_capabilities"); err != nil {
		return workbook.Capabilities{}, err
	}
	return p.s.caps(id)
}

func checkName(name, what string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" || len(n) > 63 {
		return "", fmt.Errorf("%s : nom vide ou trop long (63 octets)", what)
	}
	if strings.HasPrefix(strings.ToLower(n), "lattis_") {
		return "", fmt.Errorf("%s : le préfixe lattis_ est réservé (« %s »)", what, n)
	}
	return n, nil
}

var types = map[string]bool{"text": true, "integer": true, "decimal": true, "date": true, "datetime": true, "boolean": true}

func (p *publisher) CreateTable(ctx context.Context, id, schemaName, name string, localID int64, sheet string, cols []workbook.ColumnSpec) (string, []workbook.RemoteColumn, error) {
	if err := p.s.f.call("create_table"); err != nil {
		return "", nil, err
	}
	b, err := p.book(id, "owner", "designer")
	if err != nil {
		return "", nil, err
	}
	target, err := b.target()
	if err != nil {
		return "", nil, err
	}
	if target != schemaName {
		return "", nil, fmt.Errorf("schéma %q : la publication vise %q", schemaName, target)
	}
	s := b.schemas[target]
	if name, err = checkName(name, "tableau"); err != nil {
		return "", nil, err
	}
	for _, t := range s.tables {
		if strings.EqualFold(t.name, name) {
			return "", nil, fmt.Errorf("la relation « %s » existe déjà", name)
		}
	}
	if len(cols) == 0 {
		return "", nil, errors.New("colonnes : liste non vide attendue")
	}
	t := &table{id: uuid.NewString(), name: name, localID: localID}
	if !p.s.f.Before09 {
		t.sheet = sheet
	}
	seen := map[string]bool{}
	for i, c := range cols {
		n, err := checkName(c.Name, "colonne")
		if err != nil {
			return "", nil, err
		}
		if !types[c.Type] {
			return "", nil, fmt.Errorf("colonne « %s » : type inconnu « %s »", n, c.Type)
		}
		if seen[strings.ToLower(n)] {
			return "", nil, fmt.Errorf("la colonne « %s » est indiquée plus d'une fois", n)
		}
		seen[strings.ToLower(n)] = true
		t.cols = append(t.cols, workbook.RemoteColumn{ID: uuid.NewString(), Name: n, LocalID: c.LocalID, Type: c.Type, Position: i + 1})
	}
	s.tables = append(s.tables, t)
	return t.id, slices.Clone(t.cols), nil
}

func (p *publisher) Copy(ctx context.Context, schemaName, name string, cols []string, src workbook.CopySource) (int64, error) {
	if err := p.s.f.call("copy"); err != nil {
		return 0, err
	}
	var s *schema
	for _, b := range p.s.f.books {
		if x, ok := b.schemas[schemaName]; ok {
			target, err := b.target()
			if err != nil || target != schemaName || !p.s.has(b, "owner", "designer") {
				return 0, ErrPrivilege{"droit refusé pour la table " + name}
			}
			s = x
		}
	}
	if s == nil {
		return 0, fmt.Errorf("le schéma « %s » n'existe pas", schemaName)
	}
	var t *table
	for _, x := range s.tables {
		if x.name == name {
			t = x
		}
	}
	if t == nil {
		return 0, fmt.Errorf("la relation « %s.%s » n'existe pas", schemaName, name)
	}
	if len(cols) < 2 || cols[0] != "lattis_id" || cols[1] != "lattis_pos" {
		return 0, errors.New("lattis_id et lattis_pos attendus en tête")
	}
	at := make([]int, len(cols)-2)
	for i, c := range cols[2:] {
		at[i] = slices.IndexFunc(t.cols, func(rc workbook.RemoteColumn) bool { return rc.Name == c })
		if at[i] < 0 {
			return 0, fmt.Errorf("la colonne « %s » n'existe pas", c)
		}
	}
	ids := map[string]bool{}
	for _, r := range t.rows {
		ids[r.id] = true
	}
	var n int64
	for src.Next() {
		v, err := src.Values()
		if err != nil {
			return n, err
		}
		id, _ := v[0].(string)
		if _, err := uuid.Parse(id); err != nil {
			return n, fmt.Errorf("lattis_id invalide : %q", id)
		}
		if ids[id] {
			return n, fmt.Errorf("lattis_id en double : %s", id)
		}
		ids[id] = true
		pos, _ := v[1].(string)
		r := row{id: id, pos: pos, author: p.s.user, vals: make([]any, len(t.cols)), xid: p.s.f.xid + 1, by: p.s.user, at: p.s.f.Clock()}
		for i, j := range at {
			r.vals[j] = v[2+i]
		}
		t.rows = append(t.rows, r)
		n++
	}
	return n, src.Err()
}

func (p *publisher) SaveMeta(ctx context.Context, id, kind string, payload []byte) error {
	if err := p.s.f.call("save_meta"); err != nil {
		return err
	}
	b, err := p.book(id, "owner", "designer")
	if err != nil {
		return err
	}
	target, err := b.target()
	if err != nil {
		return err
	}
	var obj struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(payload, &obj); err != nil {
		return fmt.Errorf("contenu JSON invalide : %w", err)
	}
	b.schemas[target].meta[kind+"/"+obj.Key] = slices.Clone(payload)
	return nil
}

func (p *publisher) PublishScript(ctx context.Context, id, name, source string) (int, error) {
	if err := p.s.f.call("publish_script"); err != nil {
		return 0, err
	}
	b, err := p.book(id, "owner", "designer")
	if err != nil {
		return 0, err
	}
	target, err := b.target()
	if err != nil {
		return 0, err
	}
	s := b.schemas[target]
	v := 1
	for _, x := range s.scripts {
		if x.Name == name && x.Version >= v {
			v = x.Version + 1
		}
	}
	s.scripts = append(s.scripts, workbook.Script{Name: name, Version: v, Source: source})
	return v, nil
}

func (p *publisher) FinishPublish(ctx context.Context, id string) (int, error) {
	if err := p.s.f.call("finish_publish"); err != nil {
		return 0, err
	}
	b, err := p.book(id, "owner", "designer")
	if err != nil {
		return 0, err
	}
	switch b.state {
	case "draft":
		b.state = "published"
	case "republishing":
		delete(b.schemas, b.schema) // le délai de grâce n'est pas imité
		b.schema, b.staging, b.state = b.staging, "", "published"
		b.version++
	default:
		return 0, errors.New("rien à publier")
	}
	p.notes = append(p.notes, [2]string{"wb_" + b.slug, "*version:" + strconv.Itoa(b.version)})
	return b.version, nil
}

// --- Lecture ------------------------------------------------------------------

type reader struct{ s *session }

func (r *reader) Capabilities(ctx context.Context, id string) (workbook.Capabilities, error) {
	return r.s.caps(id)
}

func (r *reader) SyncPoint(ctx context.Context) (string, error) {
	x := r.s.f.xid + 1
	for _, h := range r.s.f.held {
		x = min(x, h)
	}
	return strconv.FormatInt(x, 10), nil
}

// Probe : instantané de la lecture (xmin:xmax:xip, comme
// pg_current_snapshot) ; point de contrôle (xmax) sans la migration 0010.
func (r *reader) Probe(ctx context.Context) (string, error) {
	xmax := r.s.f.xid + 1
	if r.s.f.Legacy || r.s.f.Before10 {
		return strconv.FormatInt(xmax, 10), nil
	}
	xmin, xip := xmax, []string{}
	for _, h := range slices.Sorted(slices.Values(r.s.f.held)) {
		xmin = min(xmin, h)
		xip = append(xip, strconv.FormatInt(h, 10))
	}
	return fmt.Sprintf("%d:%d:%s", xmin, xmax, strings.Join(xip, ",")), nil
}

// CoveredBy : lattis.since_covered(since, instantané) (migration 0010),
// ou Covered(probe) pour un point de contrôle.
func (r *reader) CoveredBy(ctx context.Context, since, probe string) (bool, error) {
	parts := strings.Split(probe, ":")
	if len(parts) != 3 {
		ok, _, err := r.Covered(ctx, probe)
		return ok, err
	}
	if r.s.f.Legacy || r.s.f.Before10 {
		return false, nil
	}
	r.s.f.Calls["since_covered"]++
	from, _ := strconv.ParseInt(since, 10, 64)
	if from == 0 || from > r.s.f.purged {
		return true, nil
	}
	p := r.s.f.purgeXid
	xmin, _ := strconv.ParseInt(parts[0], 10, 64)
	xmax, _ := strconv.ParseInt(parts[1], 10, 64)
	visible := p < xmin || (p < xmax && !slices.Contains(strings.Split(parts[2], ","), strconv.FormatInt(p, 10)))
	return p != 0 && visible, nil
}

func (r *reader) schema(name string) (*schema, error) {
	for _, b := range r.s.f.books {
		if s, ok := b.schemas[name]; ok {
			if len(r.s.roles(b)) == 0 || (name != b.schema && !r.s.has(b, "owner", "designer")) {
				return nil, ErrPrivilege{"droit refusé pour le schéma " + name}
			}
			if b.state == "draft" && !r.s.has(b, "owner", "designer") {
				return nil, ErrPrivilege{"droit refusé pour le schéma " + name}
			}
			return s, nil
		}
	}
	return nil, fmt.Errorf("le schéma « %s » n'existe pas", name)
}

func (r *reader) Tables(ctx context.Context, name string) ([]workbook.RemoteTable, error) {
	s, err := r.schema(name)
	if err != nil {
		return nil, err
	}
	var out []workbook.RemoteTable
	for i, t := range s.tables {
		out = append(out, workbook.RemoteTable{ID: t.id, Name: t.name, LocalID: t.localID, Position: i + 1, Columns: slices.Clone(t.cols), Key: t.key})
	}
	return out, nil
}

func (r *reader) Meta(ctx context.Context, name, kind, key string) ([]byte, error) {
	s, err := r.schema(name)
	if err != nil {
		return nil, err
	}
	return s.meta[kind+"/"+key], nil
}

func (r *reader) Scripts(ctx context.Context, name string) ([]workbook.Script, error) {
	s, err := r.schema(name)
	if err != nil {
		return nil, err
	}
	last := map[string]workbook.Script{}
	for _, x := range s.scripts {
		if x.Version > last[x.Name].Version {
			last[x.Name] = x
		}
	}
	var out []workbook.Script
	for _, x := range last {
		out = append(out, x)
	}
	slices.SortFunc(out, func(a, b workbook.Script) int { return cmp.Compare(a.Name, b.Name) })
	return out, nil
}

func (r *reader) Rows(ctx context.Context, name string, t workbook.RemoteTable, fn func(id, pos, author string, vals []any) error) error {
	s, err := r.schema(name)
	if err != nil {
		return err
	}
	for _, x := range s.tables {
		if x.id != t.ID {
			continue
		}
		for _, rw := range x.sorted() {
			vals := make([]any, len(t.Columns))
			for i, c := range t.Columns {
				j := slices.IndexFunc(x.cols, func(rc workbook.RemoteColumn) bool { return rc.ID == c.ID })
				if j >= 0 {
					vals[i] = workbook.FromRemote(c.Type, rw.vals[j])
				}
			}
			if err := fn(rw.id, rw.pos, rw.author, vals); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("tableau %s introuvable", t.Name)
}

func (s *session) CanCreateWorkbooks(ctx context.Context) (bool, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("login_summary"); err != nil {
		return false, err
	}
	return s.canCreate(), nil
}

// canCreate : publication ouverte à tous, créateur de classeurs ou
// propriétaire du serveur (mu tenu).
func (s *session) canCreate() bool {
	return s.f.Creators == nil || s.f.Creators[s.user] || s.f.serverOwners[s.user]
}

func (s *session) RecordOpen(ctx context.Context, id string) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	return s.begin("record_open")
}

// RecordedOpens compte les appels de record_open.
func (f *Fake) RecordedOpens() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Calls["record_open"]
}
