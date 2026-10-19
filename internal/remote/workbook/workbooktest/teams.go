package workbooktest

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Équipes Lattis (schema/0012_teams.sql du serveur) : créées par les
// propriétaires du serveur (ici par l'essai : CreateTeam, AddTeamMember),
// membres d'un rôle de classeur comme une personne (grant_role avec
// l'identifiant de l'équipe) ; leurs membres reçoivent les rôles de
// l'équipe. list_teams pour tous, membres pour les propriétaires du serveur
// (ServerOwner). Un membre ajouté ou retiré envoie « *acl » sur le canal de
// chaque classeur où l'équipe a un rôle.

type fteam struct {
	id, name string
	members  []string
}

// CreateTeam crée une équipe et rend son identifiant.
func (f *Fake) CreateTeam(name string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := &fteam{id: uuid.NewString(), name: name}
	f.teams = append(f.teams, t)
	return t.id
}

// AddTeamMember ajoute un utilisateur à une équipe (*acl aux classeurs où
// l'équipe a un rôle).
func (f *Fake) AddTeamMember(teamID, user string) {
	f.teamMember(teamID, user, true)
}

// RemoveTeamMember retire un utilisateur d'une équipe.
func (f *Fake) RemoveTeamMember(teamID, user string) {
	f.teamMember(teamID, user, false)
}

func (f *Fake) teamMember(teamID, user string, on bool) {
	f.mu.Lock()
	t := f.team(teamID)
	if t == nil {
		f.mu.Unlock()
		return
	}
	if on && !slices.Contains(t.members, user) {
		t.members = append(t.members, user)
	} else if !on {
		t.members = slices.DeleteFunc(t.members, func(x string) bool { return x == user })
	}
	var channels []string
	for _, b := range f.books {
		if a := f.acls[b.id]; a != nil && slices.ContainsFunc(a.roles, func(r *frole) bool { return slices.Contains(r.teams, teamID) }) {
			channels = append(channels, "wb_"+b.slug)
		}
	}
	f.mu.Unlock()
	for _, ch := range channels {
		go f.notify(ch, workbook.NoticeACL)
	}
}

// ServerOwner fait d'un utilisateur un propriétaire du serveur (il voit
// les membres des équipes).
func (f *Fake) ServerOwner(user string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.serverOwners == nil {
		f.serverOwners = map[string]bool{}
	}
	f.serverOwners[user] = true
}

func (f *Fake) team(id string) *fteam {
	if f.Before12 {
		return nil
	}
	for _, t := range f.teams {
		if t.id == id {
			return t
		}
	}
	return nil
}

// inTeams : l'utilisateur est membre d'une des équipes du rôle.
func (f *Fake) inTeams(r *frole, user string) bool {
	for _, id := range r.teams {
		if t := f.team(id); t != nil && slices.Contains(t.members, user) {
			return true
		}
	}
	return false
}

// teamRoles : rôles du classeur donnés à une équipe de l'utilisateur.
func (f *Fake) teamRoles(b *book, user string) []*frole {
	if f.Before12 || len(f.teams) == 0 {
		return nil
	}
	var out []*frole
	for _, r := range f.acl(b).roles {
		if f.inTeams(r, user) {
			out = append(out, r)
		}
	}
	return out
}

// Teams : lattis.list_teams.
func (s *session) Teams(ctx context.Context) ([]workbook.Team, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.begin("list_teams"); err != nil {
		return nil, err
	}
	if s.f.Before12 {
		return nil, workbook.ErrUnsupported
	}
	out := []workbook.Team{}
	for _, t := range s.f.teams {
		x := workbook.Team{ID: t.id, Name: t.name, MemberCount: len(t.members)}
		if s.f.serverOwners[s.user] {
			x.Members = []workbook.Member{}
			ms := slices.Clone(t.members)
			slices.Sort(ms)
			for _, u := range ms {
				x.Members = append(x.Members, workbook.Member{UserID: u, Email: email(u), DisplayName: u})
			}
		}
		out = append(out, x)
	}
	slices.SortFunc(out, func(x, y workbook.Team) int { return strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name)) })
	return out, nil
}
