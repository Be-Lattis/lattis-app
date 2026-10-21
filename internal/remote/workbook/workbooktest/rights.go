package workbooktest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Droits d'un classeur (schema/0005_advanced_rights.sql, 0006_admin.sql,
// 0008_sync.sql, 0009_droits.sql et 0012_teams.sql du serveur ; équipes :
// teams.go) : rôles fournis et
// personnalisés, niveaux par nœud hérités (classeur → feuille → tableau ;
// la feuille est celle donnée à create_table), filtres (gardés, non
// évalués : la RLS se vérifie contre PostgreSQL), restrictions,
// invitations (rôles personnalisés par leur nom), membres, recompilation,
// écarts, « Voir en tant que… », notification « *acl ». Les utilisateurs du faux serveur ont pour
// identifiant leur nom et pour adresse <nom>@example.test.

// ErrInvalid imite un refus de paramètre (22023).
type ErrInvalid struct{ Msg string }

func (e ErrInvalid) Error() string { return "ERROR: " + e.Msg + " (SQLSTATE 22023)" }

// SQLState rend le code de l'erreur.
func (e ErrInvalid) SQLState() string { return "22023" }

var builtins = []string{"owner", "designer", "editor", "participant", "reader"}

var builtinNames = map[string]string{"owner": "Propriétaire", "designer": "Concepteur", "editor": "Éditeur", "participant": "Participant", "reader": "Lecteur"}

type frole struct {
	id, name, builtin string
	grants            map[string]workbook.Grant // node_kind/node_id
	members           []string                  // rôles personnalisés (fournis : book.roles)
	teams             []string                  // équipes membres du rôle (0012)
}

type bookACL struct {
	roles        []*frole
	restrictions []workbook.RestrictionRule
	invitations  []workbook.Invitation
	drift        []workbook.Drift
}

// acl rend (et crée) les droits d'un classeur : un rôle par rôle fourni,
// éditeur, participant et lecteur avec leur niveau sur le classeur.
func (f *Fake) acl(b *book) *bookACL {
	if f.acls == nil {
		f.acls = map[string]*bookACL{}
	}
	a := f.acls[b.id]
	if a != nil {
		return a
	}
	a = &bookACL{}
	for _, n := range builtins {
		r := &frole{id: uuid.NewString(), name: builtinNames[n], builtin: n, grants: map[string]workbook.Grant{}}
		switch n {
		case "editor", "participant", "reader":
			r.grants["workbook/"+b.id] = workbook.Grant{NodeKind: workbook.NodeWorkbook, NodeID: b.id, Level: n}
		}
		a.roles = append(a.roles, r)
	}
	f.acls[b.id] = a
	return a
}

// SetDrift impose les écarts que rendra check_drift (essais).
func (f *Fake) SetDrift(workbookID string, d []workbook.Drift) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if b := f.books[workbookID]; b != nil {
		f.acl(b).drift = d
	}
}

// userRoles : rôles (fournis et personnalisés) d'un utilisateur, en
// personne ou par une équipe.
func (f *Fake) userRoles(b *book, user string) []*frole {
	var out []*frole
	for _, r := range f.acl(b).roles {
		if (r.builtin != "" && slices.Contains(b.roles[user], r.builtin)) || slices.Contains(r.members, user) || f.inTeams(r, user) {
			out = append(out, r)
		}
	}
	return out
}

// effective : niveau d'un rôle sur un tableau (tableau, sinon sa feuille,
// sinon classeur).
func effective(b *book, r *frole, t *table) workbook.Grant {
	if r.builtin == "owner" || r.builtin == "designer" {
		return workbook.Grant{Level: workbook.LevelEditor}
	}
	if g, ok := r.grants["table/"+t.id]; ok {
		return g
	}
	if g, ok := r.grants["sheet/"+t.sheet]; ok && t.sheet != "" {
		return g
	}
	if g, ok := r.grants["workbook/"+b.id]; ok {
		return g
	}
	return workbook.Grant{Level: workbook.LevelNone}
}

var rank = map[string]int{workbook.LevelNone: 0, workbook.LevelReader: 1, workbook.LevelParticipant: 2, workbook.LevelEditor: 3}

// access : niveau effectif (union des rôles) et lecture filtrée.
func (f *Fake) access(b *book, user string, t *table) (string, bool) {
	best, filt := workbook.LevelNone, false
	for _, r := range f.userRoles(b, user) {
		g := effective(b, r, t)
		if rank[g.Level] > rank[best] {
			best = g.Level
		}
		filt = filt || g.ReadFilter != "" || g.WriteFilter != ""
	}
	return best, filt
}

func (s *session) book(id string) (*book, error) {
	b := s.f.books[id]
	if b == nil {
		return nil, errors.New("classeur inconnu")
	}
	return b, nil
}

// filters : lecture et écriture filtrées pour l'utilisateur
// (my_capabilities.read_filtered et write_filtered, migration 0009) :
// filtres de ses rôles et restrictions qui le visent (« tous les rôles » :
// hors propriétaires et concepteurs).
func (s *session) filters(b *book, t *table) (read, write bool) {
	if b.mode != workbook.Collaborative {
		return false, false
	}
	mine := s.f.userRoles(b, s.user)
	for _, r := range mine {
		g := effective(b, r, t)
		read = read || g.ReadFilter != ""
		write = write || g.WriteFilter != ""
	}
	full := s.has(b, "owner", "designer")
	for _, x := range s.f.acl(b).restrictions {
		if x.TableID != t.id {
			continue
		}
		hit := len(x.AppliesTo) == 0 && !full
		for _, m := range mine {
			hit = hit || slices.Contains(x.AppliesTo, m.id)
		}
		if hit {
			read = read || x.Command == "select" || x.Command == "all"
			write = write || x.Command != "select"
		}
	}
	return read, write
}

// owner vérifie que l'utilisateur est propriétaire du classeur.
func (s *session) owner(id string) (*book, error) {
	b, err := s.book(id)
	if err != nil {
		return nil, err
	}
	if !s.has(b, "owner") {
		return nil, ErrPrivilege{"réservé aux propriétaires du classeur"}
	}
	return b, nil
}

func (s *session) collaborative(b *book) error {
	if b.mode != workbook.Collaborative {
		return ErrInvalid{"classeurs collaboratifs seulement"}
	}
	return nil
}

func (s *session) role(id string) (*book, *frole, error) {
	for _, b := range s.f.books {
		if a := s.f.acls[b.id]; a != nil {
			for _, r := range a.roles {
				if r.id == id {
					if !s.has(b, "owner") {
						return nil, nil, ErrPrivilege{"réservé aux propriétaires du classeur"}
					}
					return b, r, nil
				}
			}
		}
	}
	return nil, nil, errors.New("rôle inconnu")
}

// aclChanged : notification « *acl » sur le canal du classeur, envoyée
// hors du verrou.
func (s *session) aclChanged(b *book) {
	ch := "wb_" + b.slug
	go s.f.notify(ch, workbook.NoticeACL)
}

func email(user string) string { return user + "@example.test" }

func (s *session) ACL(ctx context.Context, workbookID string) (workbook.ACL, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("workbook_acl"); err != nil {
		return workbook.ACL{}, err
	}
	b, err := s.owner(workbookID)
	if err != nil {
		return workbook.ACL{}, err
	}
	a := s.f.acl(b)
	out := workbook.ACL{
		Workbook: workbook.ACLWorkbook{ID: b.id, Title: b.title, Mode: b.mode, State: b.state, Version: b.version, Schema: b.schema},
		Tables:   []workbook.ACLTable{}, Roles: []workbook.Role{},
		Restrictions: slices.Clone(a.restrictions), Invitations: []workbook.Invitation{},
	}
	for _, i := range a.invitations {
		// workbook_acl : sans auteur ni date d'envoi (workbook_invitations).
		i.InvitedBy, i.CreatedAt = "", ""
		out.Invitations = append(out.Invitations, i)
	}
	if out.Restrictions == nil {
		out.Restrictions = []workbook.RestrictionRule{}
	}
	for _, t := range b.schemas[b.schema].tables {
		out.Tables = append(out.Tables, workbook.ACLTable{TableID: t.id, Name: t.name, SheetID: t.sheet})
	}
	users := make([]string, 0, len(b.roles))
	for u := range b.roles {
		users = append(users, u)
	}
	slices.Sort(users)
	for _, r := range a.roles {
		role := workbook.Role{ID: r.id, Name: r.name, Builtin: r.builtin, Grants: []workbook.Grant{}, Members: []workbook.Member{}}
		if !s.f.Before12 {
			role.Teams = []workbook.RoleTeam{}
			for _, id := range r.teams {
				if t := s.f.team(id); t != nil {
					role.Teams = append(role.Teams, workbook.RoleTeam{TeamID: t.id, Name: t.name})
				}
			}
			slices.SortFunc(role.Teams, func(x, y workbook.RoleTeam) int {
				return strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
			})
		}
		for _, g := range r.grants {
			role.Grants = append(role.Grants, g)
		}
		slices.SortFunc(role.Grants, func(x, y workbook.Grant) int { return strings.Compare(x.NodeKind+x.NodeID, y.NodeKind+y.NodeID) })
		members := slices.Clone(r.members)
		if r.builtin != "" {
			for _, u := range users {
				if slices.Contains(b.roles[u], r.builtin) {
					members = append(members, u)
				}
			}
		}
		slices.Sort(members)
		for _, u := range members {
			role.Members = append(role.Members, workbook.Member{UserID: u, Email: email(u), DisplayName: u})
		}
		out.Roles = append(out.Roles, role)
	}
	return out, nil
}

func (s *session) Invite(ctx context.Context, workbookID, mail string, roles []string) (string, error) {
	s.f.mu.Lock()
	if err := s.begin("invite"); err != nil {
		s.f.mu.Unlock()
		return "", err
	}
	id, _, b, err := s.invite(workbookID, mail, roles)
	s.f.mu.Unlock()
	if err != nil {
		return "", err
	}
	s.aclChanged(b)
	return id, nil
}

// invite : lattis.invite et lattis.invite_person (sous le verrou) ; un
// utilisateur connu reçoit les rôles tout de suite (granted), une autre
// adresse une invitation en attente.
func (s *session) invite(workbookID, mail string, roles []string) (id string, granted bool, b *book, err error) {
	b, err = s.owner(workbookID)
	if err != nil {
		return "", false, nil, err
	}
	if len(roles) == 0 {
		return "", false, nil, ErrInvalid{"au moins un rôle"}
	}
	a := s.f.acl(b)
	var custom []*frole
	for _, r := range roles {
		if slices.Contains(builtins, r) {
			continue
		}
		i := slices.IndexFunc(a.roles, func(x *frole) bool { return x.builtin == "" && strings.EqualFold(x.name, r) })
		if i < 0 || s.f.Before09 {
			return "", false, nil, ErrInvalid{fmt.Sprintf("rôle « %s » inexistant sur ce classeur", r)}
		}
		custom = append(custom, a.roles[i])
	}
	mail = strings.ToLower(strings.TrimSpace(mail))
	id = uuid.NewString()
	user, ok := strings.CutSuffix(mail, "@example.test")
	if ok && s.f.known(user) {
		s.f.give(b, user, roles)
		return id, true, b, nil
	}
	if ok && !s.f.Before09 {
		// Compte en attente : grant_role vaut ensuite pour lui.
		if s.f.pending == nil {
			s.f.pending = map[string]bool{}
		}
		s.f.pending[user] = true
	}
	now := s.f.Clock().UTC()
	a.invitations = append(a.invitations, workbook.Invitation{ID: id, Email: mail, Roles: slices.Clone(roles), InvitedBy: email(s.user),
		CreatedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(14 * 24 * time.Hour).Format(time.RFC3339)})
	return id, false, b, nil
}

// give donne des rôles (fournis, ou personnalisés par leur nom, sans casse)
// à un utilisateur.
func (f *Fake) give(b *book, user string, roles []string) {
	for _, r := range roles {
		if slices.Contains(builtins, r) {
			if !slices.Contains(b.roles[user], r) {
				b.roles[user] = append(b.roles[user], r)
			}
			continue
		}
		for _, x := range f.acl(b).roles {
			if x.builtin == "" && strings.EqualFold(x.name, r) && !slices.Contains(x.members, user) {
				x.members = append(x.members, user)
			}
		}
	}
}

// known : utilisateur déjà vu par le faux serveur (As ou rôle donné).
func (f *Fake) known(user string) bool {
	if f.users[user] {
		return true
	}
	for _, b := range f.books {
		if _, ok := b.roles[user]; ok {
			return true
		}
	}
	for _, t := range f.teams {
		if slices.Contains(t.members, user) {
			return true
		}
	}
	return false
}

func (s *session) RevokeInvitation(ctx context.Context, id string) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("revoke_invitation"); err != nil {
		return err
	}
	for bid, a := range s.f.acls {
		if i := slices.IndexFunc(a.invitations, func(x workbook.Invitation) bool { return x.ID == id }); i >= 0 {
			if _, err := s.owner(bid); err != nil {
				return err
			}
			a.invitations = slices.Delete(a.invitations, i, i+1)
			return nil
		}
	}
	return errors.New("invitation inconnue ou déjà acceptée")
}

func (s *session) CreateRole(ctx context.Context, workbookID, name string) (string, error) {
	s.f.mu.Lock()
	if err := s.begin("create_role"); err != nil {
		s.f.mu.Unlock()
		return "", err
	}
	b, err := s.owner(workbookID)
	if err == nil {
		err = s.collaborative(b)
	}
	name = strings.TrimSpace(name)
	if err == nil && (name == "" || len(name) > 80) {
		err = ErrInvalid{"nom de rôle vide ou trop long"}
	}
	if err != nil {
		s.f.mu.Unlock()
		return "", err
	}
	a := s.f.acl(b)
	for _, r := range a.roles {
		if strings.EqualFold(r.name, name) {
			s.f.mu.Unlock()
			return "", fmt.Errorf("le rôle « %s » existe déjà", name)
		}
	}
	r := &frole{id: uuid.NewString(), name: name, grants: map[string]workbook.Grant{
		"workbook/" + b.id: {NodeKind: workbook.NodeWorkbook, NodeID: b.id, Level: workbook.LevelNone},
	}}
	a.roles = append(a.roles, r)
	s.f.mu.Unlock()
	s.aclChanged(b)
	return r.id, nil
}

func (s *session) DropRole(ctx context.Context, roleID string) error {
	s.f.mu.Lock()
	if err := s.begin("drop_role"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	b, r, err := s.role(roleID)
	if err == nil && r.builtin != "" {
		err = errors.New("les rôles fournis ne se suppriment pas")
	}
	if err != nil {
		s.f.mu.Unlock()
		return err
	}
	a := s.f.acl(b)
	a.roles = slices.DeleteFunc(a.roles, func(x *frole) bool { return x == r })
	for i := range a.restrictions {
		a.restrictions[i].AppliesTo = slices.DeleteFunc(a.restrictions[i].AppliesTo, func(x string) bool { return x == roleID })
	}
	s.f.mu.Unlock()
	s.aclChanged(b)
	return nil
}

func (s *session) GrantRole(ctx context.Context, roleID, userID string) error {
	s.f.mu.Lock()
	if err := s.begin("grant_role"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	b, r, err := s.role(roleID)
	team := s.f.team(userID)
	if err == nil && team == nil && !s.f.known(userID) && !s.f.pending[userID] {
		err = errors.New("utilisateur ou équipe inconnu")
	}
	if err != nil {
		s.f.mu.Unlock()
		return err
	}
	if team != nil {
		if !slices.Contains(r.teams, team.id) {
			r.teams = append(r.teams, team.id)
		}
	} else if r.builtin != "" {
		if !slices.Contains(b.roles[userID], r.builtin) {
			b.roles[userID] = append(b.roles[userID], r.builtin)
		}
	} else if !slices.Contains(r.members, userID) {
		r.members = append(r.members, userID)
	}
	s.f.mu.Unlock()
	s.aclChanged(b)
	return nil
}

func (s *session) RevokeRole(ctx context.Context, roleID, userID string) error {
	s.f.mu.Lock()
	if err := s.begin("revoke_role"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	b, r, err := s.role(roleID)
	if err != nil {
		s.f.mu.Unlock()
		return err
	}
	if team := s.f.team(userID); team != nil {
		r.teams = slices.DeleteFunc(r.teams, func(x string) bool { return x == team.id })
	} else if r.builtin != "" {
		if r.builtin == "owner" && slices.Contains(b.roles[userID], "owner") {
			n := 0
			for _, rs := range b.roles {
				if slices.Contains(rs, "owner") {
					n++
				}
			}
			if n <= 1 {
				s.f.mu.Unlock()
				return errors.New("un classeur garde au moins un propriétaire")
			}
		}
		b.roles[userID] = slices.DeleteFunc(b.roles[userID], func(x string) bool { return x == r.builtin })
	} else {
		r.members = slices.DeleteFunc(r.members, func(x string) bool { return x == userID })
	}
	s.f.mu.Unlock()
	s.aclChanged(b)
	return nil
}

// checkFilter imite lattis.check_predicate : ce qui sortirait de
// l'expression est refusé (le reste n'est pas analysé).
func checkFilter(expr string) error {
	inStr, inID := false, false
	depth := 0
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		switch {
		case c == '\'' && !inID:
			inStr = !inStr
		case c == '"' && !inStr:
			inID = !inID
		case inStr || inID:
		case c == ';' || c == '$' || c == '\\' || strings.HasPrefix(expr[i:], "--") || strings.HasPrefix(expr[i:], "/*"):
			return ErrInvalid{fmt.Sprintf("filtre refusé : « %c » n'est pas permis hors d'une chaîne", c)}
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth < 0 {
				return ErrInvalid{"filtre refusé : parenthèses déséquilibrées"}
			}
		}
	}
	if inStr || inID {
		return ErrInvalid{"filtre refusé : chaîne ou nom non fermé"}
	}
	if depth != 0 {
		return ErrInvalid{"filtre refusé : parenthèses déséquilibrées"}
	}
	return nil
}

func (s *session) SetLevels(ctx context.Context, levels []workbook.Level) error {
	s.f.mu.Lock()
	if err := s.begin("set_level"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	var b *book
	type change struct {
		r *frole
		l workbook.Level
	}
	var todo []change
	for _, l := range levels {
		if err := workbook.CheckLevel(l); err != nil {
			s.f.mu.Unlock()
			return ErrInvalid{err.Error()}
		}
		rb, r, err := s.role(l.RoleID)
		if err == nil {
			err = s.collaborative(rb)
		}
		if err == nil && (r.builtin == "owner" || r.builtin == "designer") {
			err = ErrInvalid{"propriétaires et concepteurs ont tous les droits"}
		}
		if err == nil && l.NodeKind == workbook.NodeTable && !slices.ContainsFunc(rb.schemas[rb.schema].tables, func(t *table) bool { return t.id == l.NodeID }) {
			err = errors.New("tableau inconnu")
		}
		if err == nil {
			if err = checkFilter(l.ReadFilter); err == nil {
				err = checkFilter(l.WriteFilter)
			}
		}
		if err != nil {
			s.f.mu.Unlock()
			return err
		}
		if l.NodeKind == workbook.NodeWorkbook {
			l.NodeID = rb.id
			if l.Level == "" {
				l.Level = workbook.LevelNone
			}
		}
		b = rb
		todo = append(todo, change{r, l})
	}
	for _, c := range todo {
		k := c.l.NodeKind + "/" + c.l.NodeID
		if c.l.Level == "" {
			delete(c.r.grants, k)
			continue
		}
		c.r.grants[k] = workbook.Grant{NodeKind: c.l.NodeKind, NodeID: c.l.NodeID, Level: c.l.Level,
			ReadFilter: strings.TrimSpace(c.l.ReadFilter), WriteFilter: strings.TrimSpace(c.l.WriteFilter)}
	}
	s.f.mu.Unlock()
	if b != nil {
		s.aclChanged(b)
	}
	return nil
}

func (s *session) AddRestriction(ctx context.Context, workbookID string, r workbook.RestrictionRule) (string, error) {
	s.f.mu.Lock()
	if err := s.begin("add_restriction"); err != nil {
		s.f.mu.Unlock()
		return "", err
	}
	b, err := s.owner(workbookID)
	if err == nil {
		err = s.collaborative(b)
	}
	if err == nil {
		if e := workbook.CheckRestriction(r); e != nil {
			err = ErrInvalid{e.Error()}
		}
	}
	if err == nil {
		err = checkFilter(r.Predicate)
	}
	if err != nil {
		s.f.mu.Unlock()
		return "", err
	}
	a := s.f.acl(b)
	for _, id := range r.AppliesTo {
		if !slices.ContainsFunc(a.roles, func(x *frole) bool { return x.id == id }) {
			s.f.mu.Unlock()
			return "", ErrInvalid{"rôle étranger au classeur"}
		}
	}
	r.ID = uuid.NewString()
	if r.AppliesTo == nil {
		r.AppliesTo = []string{}
	}
	a.restrictions = append(a.restrictions, r)
	s.f.mu.Unlock()
	s.aclChanged(b)
	return r.ID, nil
}

func (s *session) DropRestriction(ctx context.Context, id string) error {
	s.f.mu.Lock()
	if err := s.begin("drop_restriction"); err != nil {
		s.f.mu.Unlock()
		return err
	}
	for bid, a := range s.f.acls {
		if i := slices.IndexFunc(a.restrictions, func(x workbook.RestrictionRule) bool { return x.ID == id }); i >= 0 {
			b, err := s.owner(bid)
			if err != nil {
				s.f.mu.Unlock()
				return err
			}
			a.restrictions = slices.Delete(a.restrictions, i, i+1)
			s.f.mu.Unlock()
			s.aclChanged(b)
			return nil
		}
	}
	s.f.mu.Unlock()
	return errors.New("restriction inconnue")
}

func (s *session) Drift(ctx context.Context, workbookID string) ([]workbook.Drift, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("check_drift"); err != nil {
		return nil, err
	}
	b, err := s.book(workbookID)
	if err != nil {
		return nil, err
	}
	if !s.has(b, "owner", "designer") {
		return nil, ErrPrivilege{"réservé aux propriétaires et concepteurs du classeur"}
	}
	out := slices.Clone(s.f.acl(b).drift)
	if out == nil {
		out = []workbook.Drift{}
	}
	return out, nil
}

// Preview : « Voir en tant que… » (workbook.Previewer) : les lignes que
// verrait userID (aucune sans accès au tableau ; les filtres ne sont pas
// évalués par le faux serveur).
func (s *session) Preview(ctx context.Context, workbookID, userID, tableID string) (workbook.Preview, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("preview"); err != nil {
		return workbook.Preview{}, err
	}
	b, err := s.book(workbookID)
	if err != nil {
		return workbook.Preview{}, err
	}
	if !s.has(b, "owner", "designer") {
		return workbook.Preview{}, msg.New("rights.viewAsReserved", nil)
	}
	var t *table
	for _, x := range b.schemas[b.schema].tables {
		if x.id == tableID {
			t = x
		}
	}
	if t == nil || !s.f.known(userID) {
		return workbook.Preview{}, msg.New("viewAs.unknown", nil)
	}
	out := workbook.Preview{Rows: []map[string]any{}}
	if lv, _ := (&session{f: s.f, user: userID}).levelOf(b, t); lv == workbook.LevelNone {
		return out, nil
	}
	for _, r := range t.sorted() {
		m := map[string]any{"lattis_id": r.id, "lattis_pos": r.pos, "lattis_author": r.author}
		for i, c := range t.cols {
			m[c.Name] = r.vals[i]
		}
		out.Rows = append(out.Rows, m)
	}
	return out, nil
}

// levelOf : niveau et lecture filtrée d'un utilisateur sur un tableau.
func (s *session) levelOf(b *book, t *table) (string, bool) {
	if len(s.roles(b)) == 0 && len(s.f.userRoles(b, s.user)) == 0 {
		return workbook.LevelNone, false
	}
	if b.mode != workbook.Collaborative {
		return workbook.LevelReader, false
	}
	if lv, ok := s.f.levels[b.id+"\x00"+s.user+"\x00"+t.name]; ok {
		return lv, false
	}
	return s.f.access(b, s.user, t)
}

// myRestrictions : restrictions qui visent l'utilisateur.
func (s *session) myRestrictions(b *book) []workbook.Restriction {
	out := []workbook.Restriction{}
	mine := s.f.userRoles(b, s.user)
	for _, r := range s.f.acl(b).restrictions {
		hit := len(r.AppliesTo) == 0
		for _, m := range mine {
			hit = hit || slices.Contains(r.AppliesTo, m.id)
		}
		if hit {
			out = append(out, workbook.Restriction{TableID: r.TableID, Command: r.Command, Label: r.Label})
		}
	}
	return out
}

// customRoles : noms des rôles personnalisés de l'utilisateur.
func (s *session) customRoles(b *book) []string {
	out := []string{}
	for _, r := range s.f.userRoles(b, s.user) {
		if r.builtin == "" {
			out = append(out, r.name)
		}
	}
	slices.Sort(out)
	return out
}

// Members : lattis.workbook_members (migration 0009), pour propriétaires et
// concepteurs ; rôles par nom (fournis : owner…, personnalisés : nom).
func (s *session) Members(ctx context.Context, workbookID string) ([]workbook.WorkbookMember, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("workbook_members"); err != nil {
		return nil, err
	}
	if s.f.Before09 {
		return nil, workbook.ErrUnsupported
	}
	b, err := s.book(workbookID)
	if err != nil {
		return nil, err
	}
	if !s.has(b, "owner", "designer") {
		return nil, ErrPrivilege{"réservé aux propriétaires et concepteurs du classeur"}
	}
	by := map[string][]string{}
	via := map[string][]string{} // équipes par lesquelles l'utilisateur est membre (0012)
	add := func(u, name string) {
		if !slices.Contains(by[u], name) {
			by[u] = append(by[u], name)
		}
	}
	for u, rs := range b.roles {
		for _, r := range rs {
			add(u, r)
		}
	}
	for _, r := range s.f.acl(b).roles {
		key := r.builtin
		if key == "" {
			key = r.name
			for _, u := range r.members {
				add(u, key)
			}
		}
		for _, id := range r.teams {
			t := s.f.team(id)
			if t == nil {
				continue
			}
			for _, u := range t.members {
				add(u, key)
				if !slices.Contains(via[u], t.name) {
					via[u] = append(via[u], t.name)
				}
			}
		}
	}
	out := []workbook.WorkbookMember{}
	for u, rs := range by {
		if len(rs) == 0 {
			continue
		}
		slices.Sort(rs)
		m := workbook.WorkbookMember{UserID: u, Email: email(u), DisplayName: u, PGRole: u, Roles: rs}
		if !s.f.Before12 {
			m.Teams = via[u]
			if m.Teams == nil {
				m.Teams = []string{}
			}
			slices.Sort(m.Teams)
		}
		out = append(out, m)
	}
	slices.SortFunc(out, func(x, y workbook.WorkbookMember) int { return strings.Compare(x.Email, y.Email) })
	return out, nil
}

// Recompile : lattis.compile_acl ; l'empreinte des droits est reprise, les
// écarts disparaissent (même les règles ajoutées à la main, qui restent).
func (s *session) Recompile(ctx context.Context, workbookID string) error {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("compile_acl"); err != nil {
		return err
	}
	b, err := s.book(workbookID)
	if err != nil {
		return err
	}
	if !s.has(b, "owner", "designer") {
		return ErrPrivilege{"droits insuffisants"}
	}
	a := s.f.acl(b)
	a.drift = nil
	return nil
}
