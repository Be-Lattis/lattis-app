package workbooktest

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Recherche de personnes et invitations (migration 0046 du serveur) :
// search_people parmi les utilisateurs déjà connectés (As), par nom
// affiché (SetName) ou adresse ; invite_person (granted pour un compte
// connu, sinon invitation dont les rôles sont posés à la première
// connexion, courriel si Mailer) ; workbook_invitations.

// SetName donne un nom affiché à un utilisateur (search_people).
func (f *Fake) SetName(user, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.names == nil {
		f.names = map[string]string{}
	}
	f.names[user] = name
}

func (f *Fake) nameOf(user string) string {
	if n := f.names[user]; n != "" {
		return n
	}
	return user
}

// firstLogin applique les invitations d'invite_person à l'adresse de
// l'utilisateur (sous le verrou) : rôles posés, invitation retirée.
func (f *Fake) firstLogin(user string) {
	for _, b := range f.books {
		a := f.acls[b.id]
		if a == nil {
			continue
		}
		n := len(a.invitations)
		a.invitations = slices.DeleteFunc(a.invitations, func(i workbook.Invitation) bool {
			if !f.onLogin[i.ID] || i.Email != email(user) {
				return false
			}
			f.give(b, user, i.Roles)
			delete(f.onLogin, i.ID)
			return true
		})
		if len(a.invitations) != n {
			go f.notify("wb_"+b.slug, workbook.NoticeACL)
		}
	}
}

func (s *session) SearchPeople(ctx context.Context, query string, limit int) ([]workbook.Person, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("search_people"); err != nil {
		return nil, err
	}
	if s.f.Before46 {
		return nil, workbook.ErrUnsupported
	}
	out := []workbook.Person{}
	q := strings.ToLower(strings.TrimSpace(query))
	if len([]rune(q)) < 2 {
		return out, nil
	}
	if limit <= 0 {
		limit = 10
	}
	limit = min(limit, 20)
	for u := range s.f.users {
		name := s.f.nameOf(u)
		if strings.Contains(strings.ToLower(name), q) || strings.Contains(email(u), q) {
			out = append(out, workbook.Person{ID: u, DisplayName: name, Email: email(u)})
		}
	}
	slices.SortFunc(out, func(x, y workbook.Person) int {
		return cmp.Or(strings.Compare(strings.ToLower(x.DisplayName), strings.ToLower(y.DisplayName)), strings.Compare(x.Email, y.Email))
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *session) InvitePerson(ctx context.Context, workbookID, mail string, roles []string) (workbook.InviteResult, error) {
	s.f.mu.Lock()
	if err := s.begin("invite_person"); err != nil {
		s.f.mu.Unlock()
		return workbook.InviteResult{}, err
	}
	if s.f.Before46 {
		s.f.mu.Unlock()
		return workbook.InviteResult{}, workbook.ErrUnsupported
	}
	if !strings.Contains(mail, "@") {
		s.f.mu.Unlock()
		return workbook.InviteResult{}, ErrInvalid{"adresse invalide"}
	}
	id, granted, b, err := s.invite(workbookID, mail, roles)
	if err != nil {
		s.f.mu.Unlock()
		return workbook.InviteResult{}, err
	}
	out := workbook.InviteResult{InvitationID: id, Status: workbook.InviteGranted, LinkPath: "/w/" + b.id}
	if !granted {
		out.Status, out.Mailed = workbook.InviteInvited, s.f.Mailer
		if s.f.onLogin == nil {
			s.f.onLogin = map[string]bool{}
		}
		s.f.onLogin[id] = true
	}
	s.f.mu.Unlock()
	s.aclChanged(b)
	return out, nil
}

func (s *session) WorkbookInvitations(ctx context.Context, workbookID string) ([]workbook.Invitation, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("workbook_invitations"); err != nil {
		return nil, err
	}
	if s.f.Before46 {
		return nil, workbook.ErrUnsupported
	}
	b, err := s.owner(workbookID)
	if err != nil {
		return nil, err
	}
	out := slices.Clone(s.f.acl(b).invitations)
	if out == nil {
		out = []workbook.Invitation{}
	}
	return out, nil
}
