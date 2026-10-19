package bridge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Droits d'un classeur partagé (docs/collaboration/droits.md) : écran
// « Partager » des propriétaires (membres, rôles et niveaux par nœud avec
// filtres, restrictions), « Voir en tant que… » et écarts (propriétaires et
// concepteurs), droits de l'utilisateur pour l'onglet SQL. Les appels au
// serveur passent par workbook.Rights ; l'interface est prévenue par
// Host.Emit (ShareEvent) quand les droits de l'utilisateur changent.

// sharePreviewer donne « Voir en tant que… » d'un serveur (pour les essais ;
// nil : par la découverte et le jeton de la session).
var sharePreviewer func(ctx context.Context, server string) (workbook.Previewer, error)

// RightsColumn est une colonne d'un tableau publié (nom et type du
// serveur : formulaire des filtres).
type RightsColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// RightsTable est un tableau publié : où il est dans le classeur local
// (arborescence classeur → feuilles → tableaux) et les droits de
// l'utilisateur dessus.
type RightsTable struct {
	Table     int64  `json:"table"`
	Name      string `json:"name"`
	RemoteID  string `json:"remoteId"`
	Sheet     int64  `json:"sheet"`
	SheetName string `json:"sheetName"`
	// SheetKey : feuille du tableau pour le serveur (workbook.SheetKey ;
	// vide : inconnue, publié avant la migration 0009, ou liste complète
	// non lue). Égale à SheetKey(Sheet), un niveau posé sur la feuille vaut
	// pour le tableau.
	SheetKey     string                 `json:"sheetKey"`
	Level        string                 `json:"level"`
	Filtered     bool                   `json:"filtered"`
	Restrictions []workbook.Restriction `json:"restrictions"`
	Columns      []RightsColumn         `json:"columns"`
}

// RightsView décrit les droits d'un classeur partagé : ceux de
// l'utilisateur (toujours, depuis le classeur local) et, en ligne, la liste
// complète (propriétaire) et les écarts (propriétaire ou concepteur).
type RightsView struct {
	Shared        bool          `json:"shared"`
	Collaborative bool          `json:"collaborative"`
	Online        bool          `json:"online"`
	CanManage     bool          `json:"canManage"`
	CanDesign     bool          `json:"canDesign"`
	User          string        `json:"user"`
	Roles         []string      `json:"roles"`
	CustomRoles   []string      `json:"customRoles"`
	Tables        []RightsTable `json:"tables"`
	// LocalTables : tableaux qui ne sont que sur ce poste (classeur
	// collaboratif, propriétaire ou concepteur), à ajouter au classeur
	// partagé (ShareAddTable).
	LocalTables []string `json:"localTables,omitempty"`
	// ACL : rôles, membres, restrictions, invitations (propriétaire, en
	// ligne ; nil sinon).
	ACL *workbook.ACL `json:"acl"`
	// Drift : écarts détectés par le serveur (propriétaire ou concepteur).
	Drift []workbook.Drift `json:"drift"`
	// Members : membres du classeur et leurs rôles (lattis.workbook_members,
	// propriétaire ou concepteur, en ligne ; nil : serveur sans la
	// migration 0009, liste tirée de ACL).
	Members []workbook.WorkbookMember `json:"members"`
	// CustomInvite : le serveur invite à un rôle personnalisé (migration
	// 0009).
	CustomInvite bool `json:"customInvite"`
	// Teams : équipes Lattis du serveur, à qui donner des rôles
	// (lattis.list_teams, propriétaire, en ligne ; membres de chacune pour un
	// propriétaire du serveur). nil : serveur sans la migration 0012 (pas
	// d'équipes ni de filtre « mon équipe »).
	Teams []workbook.Team `json:"teams"`
	// Error : le serveur n'a pas répondu (droits locaux seulement).
	Error string `json:"error,omitempty"`
}

// localRights lit les droits notés dans le classeur local.
func localRights(ctx context.Context, d *doc.Document) (RightsView, error) {
	v := RightsView{Roles: []string{}, CustomRoles: []string{}, Tables: []RightsTable{}, Drift: []workbook.Drift{}}
	info := d.Share.Info()
	if info == nil {
		return v, nil
	}
	st := d.Share.State()
	v.Shared, v.Collaborative, v.Online = true, info.Mode == workbook.Collaborative, st.Online
	v.CanManage, v.CanDesign, v.User = info.CanManage(), info.CanDesign(), info.User
	v.Roles = append(v.Roles, info.Roles...)
	db := d.DB().Read()
	var custom sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT filters FROM _lattis_capabilities WHERE tbl = 0`).Scan(&custom); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return v, err
	}
	if custom.Valid {
		_ = json.Unmarshal([]byte(custom.String), &v.CustomRoles)
	}
	rs, err := db.QueryContext(ctx, `SELECT r.tbl, t.name, coalesce(r.remote_id, ''), t.sheet, s.name, coalesce(r.columns, '[]'), coalesce(c.level, '')
		FROM _lattis_remote_tables r JOIN _lattis_tables t ON t.id = r.tbl JOIN _lattis_sheets s ON s.id = t.sheet
		LEFT JOIN _lattis_capabilities c ON c.tbl = r.tbl
		WHERE r.published = 1 ORDER BY s.pos, t.name`)
	if err != nil {
		return v, err
	}
	defer rs.Close()
	for rs.Next() {
		var t RightsTable
		var cols string
		if err := rs.Scan(&t.Table, &t.Name, &t.RemoteID, &t.Sheet, &t.SheetName, &cols, &t.Level); err != nil {
			return v, err
		}
		var rc []workbook.RemoteColumn
		_ = json.Unmarshal([]byte(cols), &rc)
		t.Columns = []RightsColumn{}
		for _, c := range rc {
			t.Columns = append(t.Columns, RightsColumn{Name: c.Name, Type: c.Type})
		}
		if lv, filt := d.Share.TableAccess(t.Table); lv != "" {
			t.Level, t.Filtered = lv, filt
		}
		if info.Mode == workbook.Frozen {
			t.Level = workbook.LevelReader
		}
		t.Restrictions = d.Share.Restrictions(t.Table)
		if t.Restrictions == nil {
			t.Restrictions = []workbook.Restriction{}
		}
		v.Tables = append(v.Tables, t)
	}
	if err := rs.Err(); err != nil {
		return v, err
	}
	rs.Close()
	if v.Collaborative && v.CanDesign {
		// Tableaux créés sur ce poste après la publication : à ajouter au
		// classeur partagé (ShareAddTable).
		lr, err := db.QueryContext(ctx, `SELECT t.name FROM _lattis_tables t JOIN _lattis_sheets s ON s.id = t.sheet
			WHERE t.id NOT IN (SELECT tbl FROM _lattis_remote_tables WHERE published = 1)
			AND t.id NOT IN (SELECT target FROM _lattis_queries WHERE target IS NOT NULL) ORDER BY s.pos, t.name`)
		if err != nil {
			return v, err
		}
		defer lr.Close()
		for lr.Next() {
			var n string
			if err := lr.Scan(&n); err != nil {
				return v, err
			}
			v.LocalTables = append(v.LocalTables, n)
		}
		return v, lr.Err()
	}
	return v, nil
}

// ShareRights décrit les droits du classeur partagé ouvert ; remote :
// demander aussi au serveur la liste complète (propriétaire) et les écarts
// (propriétaire ou concepteur).
func (s *DocumentService) ShareRights(ctx context.Context, remote bool) (RightsView, error) {
	d, err := s.current()
	if err != nil {
		return RightsView{}, err
	}
	v, err := localRights(ctx, d)
	if err != nil || !v.Shared || !remote || !(v.CanManage || v.CanDesign) {
		return v, err
	}
	info := d.Share.Info()
	srv, _, _, err := remoteServer(ctx, info.Server)
	if err != nil {
		v.Error = err.Error()
		return v, nil
	}
	if v.CanManage {
		acl, err := srv.ACL(ctx, info.WorkbookID)
		if err != nil {
			v.Error = rightsError(err)
			return v, nil
		}
		v.ACL = &acl
		sheetKeys(v.Tables, acl)
		// Équipes (0012) : sans elles, l'écran reste celui d'avant.
		if teams, err := srv.Teams(ctx); err == nil {
			v.Teams = teams
		}
	}
	switch m, err := srv.Members(ctx, info.WorkbookID); {
	case errors.Is(err, workbook.ErrUnsupported):
	case err != nil:
		v.Error = rightsError(err)
		return v, nil
	default:
		v.Members, v.CustomInvite = m, v.Collaborative
	}
	if v.Collaborative {
		drift, err := srv.Drift(ctx, info.WorkbookID)
		if err != nil {
			v.Error = rightsError(err)
			return v, nil
		}
		v.Drift = drift
	}
	return v, nil
}

// sheetKeys note la feuille de chaque tableau pour le serveur (liste
// complète des droits).
func sheetKeys(tables []RightsTable, acl workbook.ACL) {
	keys := map[string]string{}
	for _, t := range acl.Tables {
		keys[t.TableID] = t.SheetID
	}
	for i := range tables {
		tables[i].SheetKey = keys[tables[i].RemoteID]
	}
}

// rightsError rend un refus du serveur lisible (le message de la fonction
// SQL, sans « ERROR: … (SQLSTATE …) »).
func rightsError(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Message
	}
	msg := strings.TrimPrefix(err.Error(), "ERROR: ")
	if i := strings.LastIndex(msg, " (SQLSTATE "); i > 0 {
		msg = msg[:i]
	}
	return msg
}

// rightsDo fait un changement de droits sur le serveur du classeur ouvert
// (propriétaires), puis relit les droits de l'utilisateur (ils ont pu
// changer : interface prévenue) et rend la nouvelle liste.
func (s *DocumentService) rightsDo(ctx context.Context, fn func(srv workbook.Server, info *workbook.Info) error) (RightsView, error) {
	return s.rightsDoAs(ctx, false, fn)
}

// rightsDoAs : rightsDo ; design : concepteurs admis aussi.
func (s *DocumentService) rightsDoAs(ctx context.Context, design bool, fn func(srv workbook.Server, info *workbook.Info) error) (RightsView, error) {
	d, err := s.current()
	if err != nil {
		return RightsView{}, err
	}
	info := d.Share.Info()
	if info == nil {
		return RightsView{}, errors.New("ce classeur n'est pas partagé : Fichier › Publier sur un serveur…")
	}
	switch {
	case design && !info.CanDesign():
		return RightsView{}, errors.New("réservé aux propriétaires et concepteurs du classeur")
	case !design && !info.CanManage():
		return RightsView{}, errors.New("seuls les propriétaires du classeur gèrent ses droits")
	}
	srv, _, _, err := remoteServer(ctx, info.Server)
	if err != nil {
		return RightsView{}, err
	}
	if err := fn(srv, info); err != nil {
		return RightsView{}, errors.New(rightsError(err))
	}
	if changed, err := workbook.RefreshCapabilities(ctx, srv, d.Share); err == nil && changed {
		s.emitShare(d)
	}
	return s.ShareRights(ctx, true)
}

// ShareInvite invite une adresse au classeur ouvert avec des rôles fournis
// (reader, participant, editor, designer, owner) et, si le serveur le
// permet (RightsView.CustomInvite), des rôles personnalisés par leur nom.
func (s *DocumentService) ShareInvite(ctx context.Context, email string, roles []string) (RightsView, error) {
	email = strings.TrimSpace(email)
	if !strings.Contains(email, "@") {
		return RightsView{}, fmt.Errorf("adresse invalide : %q", email)
	}
	if len(roles) == 0 {
		return RightsView{}, errors.New("choisir au moins un rôle")
	}
	return s.rightsDo(ctx, func(srv workbook.Server, info *workbook.Info) error {
		_, err := srv.Invite(ctx, info.WorkbookID, email, roles)
		return err
	})
}

// ShareRevokeInvitation retire une invitation en attente.
func (s *DocumentService) ShareRevokeInvitation(ctx context.Context, id string) (RightsView, error) {
	return s.rightsDo(ctx, func(srv workbook.Server, _ *workbook.Info) error { return srv.RevokeInvitation(ctx, id) })
}

// ShareCreateRole crée un rôle personnalisé (sans droit tant qu'aucun
// niveau n'est posé).
func (s *DocumentService) ShareCreateRole(ctx context.Context, name string) (RightsView, error) {
	if strings.TrimSpace(name) == "" {
		return RightsView{}, errors.New("nom de rôle vide")
	}
	return s.rightsDo(ctx, func(srv workbook.Server, info *workbook.Info) error {
		_, err := srv.CreateRole(ctx, info.WorkbookID, strings.TrimSpace(name))
		return err
	})
}

// ShareDropRole supprime un rôle personnalisé.
func (s *DocumentService) ShareDropRole(ctx context.Context, roleID string) (RightsView, error) {
	return s.rightsDo(ctx, func(srv workbook.Server, _ *workbook.Info) error { return srv.DropRole(ctx, roleID) })
}

// ShareGrantRole donne un rôle à un membre, personne (user_id) ou équipe
// (team_id de RightsView.Teams, migration 0012) ; on faux : le lui retire.
func (s *DocumentService) ShareGrantRole(ctx context.Context, roleID, principal string, on bool) (RightsView, error) {
	return s.rightsDo(ctx, func(srv workbook.Server, _ *workbook.Info) error {
		if on {
			return srv.GrantRole(ctx, roleID, principal)
		}
		return srv.RevokeRole(ctx, roleID, principal)
	})
}

// ShareSetLevels pose des niveaux (et filtres) dans une seule transaction.
// L'interface envoie un niveau de feuille avec NodeKind « sheet » et
// l'identifiant local de la feuille : si le serveur connaît la feuille de
// chacun de ses tableaux publiés (migration 0009, SheetKey), le niveau est
// posé sur la feuille (set_level('sheet', …), hérité par ses tableaux) ;
// sinon sur chacun de ses tableaux publiés (tableaux publiés avant 0009).
func (s *DocumentService) ShareSetLevels(ctx context.Context, levels []workbook.Level) (RightsView, error) {
	d, err := s.current()
	if err != nil {
		return RightsView{}, err
	}
	local, err := localRights(ctx, d)
	if err != nil {
		return RightsView{}, err
	}
	sheets := false
	for _, l := range levels {
		if l.NodeKind == workbook.NodeSheet {
			sheets = true
		} else if err := workbook.CheckLevel(l); err != nil {
			return RightsView{}, err
		}
	}
	return s.rightsDo(ctx, func(srv workbook.Server, info *workbook.Info) error {
		if sheets {
			acl, err := srv.ACL(ctx, info.WorkbookID)
			if err != nil {
				return err
			}
			sheetKeys(local.Tables, acl)
		}
		out, err := sheetLevels(levels, local.Tables)
		if err != nil {
			return err
		}
		return srv.SetLevels(ctx, out)
	})
}

// sheetLevels traduit les niveaux de feuille (identifiant local) : sur la
// feuille du serveur si tous ses tableaux publiés y sont rattachés, sinon
// sur chacun d'eux.
func sheetLevels(levels []workbook.Level, tables []RightsTable) ([]workbook.Level, error) {
	var out []workbook.Level
	for _, l := range levels {
		if l.NodeKind != workbook.NodeSheet {
			out = append(out, l)
			continue
		}
		var on []RightsTable
		for _, t := range tables {
			if fmt.Sprint(t.Sheet) == l.NodeID && t.RemoteID != "" {
				on = append(on, t)
			}
		}
		if len(on) == 0 {
			return nil, errors.New("aucun tableau publié sur cette feuille")
		}
		key := ""
		if id, err := strconv.ParseInt(l.NodeID, 10, 64); err == nil {
			key = workbook.SheetKey(id)
		}
		if key != "" && !slices.ContainsFunc(on, func(t RightsTable) bool { return t.SheetKey != key }) {
			out = append(out, workbook.Level{RoleID: l.RoleID, NodeKind: workbook.NodeSheet, NodeID: key, Level: l.Level})
			continue
		}
		for _, t := range on {
			out = append(out, workbook.Level{RoleID: l.RoleID, NodeKind: workbook.NodeTable, NodeID: t.RemoteID, Level: l.Level})
		}
	}
	for _, l := range out {
		if err := workbook.CheckLevel(l); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ShareRecompile recompile les droits du classeur (lattis.compile_acl :
// propriétaires et concepteurs) : remet d'aplomb les écarts.
func (s *DocumentService) ShareRecompile(ctx context.Context) (RightsView, error) {
	return s.rightsDoAs(ctx, true, func(srv workbook.Server, info *workbook.Info) error { return srv.Recompile(ctx, info.WorkbookID) })
}

// ShareAddRestriction ajoute une restriction.
func (s *DocumentService) ShareAddRestriction(ctx context.Context, r workbook.RestrictionRule) (RightsView, error) {
	if err := workbook.CheckRestriction(r); err != nil {
		return RightsView{}, err
	}
	return s.rightsDo(ctx, func(srv workbook.Server, info *workbook.Info) error {
		_, err := srv.AddRestriction(ctx, info.WorkbookID, r)
		return err
	})
}

// ShareDropRestriction retire une restriction.
func (s *DocumentService) ShareDropRestriction(ctx context.Context, id string) (RightsView, error) {
	return s.rightsDo(ctx, func(srv workbook.Server, _ *workbook.Info) error { return srv.DropRestriction(ctx, id) })
}

// PreviewView est un tableau tel que le voit un membre (« Voir en tant
// que… ») : colonnes du tableau publié, puis l'auteur de la ligne.
type PreviewView struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated"`
}

// previewer rend « Voir en tant que… » du serveur.
func previewer(ctx context.Context, server string, srv workbook.Server) (workbook.Previewer, error) {
	if sharePreviewer != nil {
		return sharePreviewer(ctx, server)
	}
	if p, ok := srv.(workbook.Previewer); ok {
		return p, nil
	}
	m, err := lattisOn()
	if err != nil {
		return nil, err
	}
	server, err = auth.NormalizeURL(server)
	if err != nil {
		return nil, err
	}
	_, disc, err := m.Session(ctx, server)
	if err != nil {
		return nil, err
	}
	return workbook.HTTPPreview(nil, disc.PreviewEndpoint, func(ctx context.Context) (string, error) { return m.Token(ctx, server) }), nil
}

// SharePreview montre un tableau publié tel que le voit un membre (lecture
// seule, au plus 1 000 lignes) : propriétaires et concepteurs.
func (s *DocumentService) SharePreview(ctx context.Context, userID string, table int64) (PreviewView, error) {
	d, err := s.current()
	if err != nil {
		return PreviewView{}, err
	}
	info := d.Share.Info()
	if info == nil || !info.CanDesign() {
		return PreviewView{}, errors.New("« Voir en tant que… » est réservé aux propriétaires et concepteurs du classeur")
	}
	local, err := localRights(ctx, d)
	if err != nil {
		return PreviewView{}, err
	}
	var t *RightsTable
	for i := range local.Tables {
		if local.Tables[i].Table == table {
			t = &local.Tables[i]
		}
	}
	if t == nil || t.RemoteID == "" {
		return PreviewView{}, errors.New("tableau non publié")
	}
	srv, _, _, err := remoteServer(ctx, info.Server)
	if err != nil {
		return PreviewView{}, err
	}
	p, err := previewer(ctx, info.Server, srv)
	if err != nil {
		return PreviewView{}, err
	}
	res, err := p.Preview(ctx, info.WorkbookID, userID, t.RemoteID)
	if err != nil {
		return PreviewView{}, err
	}
	out := PreviewView{Columns: []string{}, Rows: [][]any{}, Truncated: res.Truncated}
	for _, c := range t.Columns {
		out.Columns = append(out.Columns, c.Name)
	}
	out.Columns = append(out.Columns, "Auteur")
	for _, r := range res.Rows {
		row := make([]any, 0, len(out.Columns))
		for _, c := range t.Columns {
			row = append(row, r[c.Name])
		}
		row = append(row, r["lattis_author"])
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

// lockedRows marque les lignes d'une fenêtre de la grille qu'un
// participant ne peut pas modifier (lignes d'autres membres, §8.1).
func (s *DocumentService) lockedRows(ctx context.Context, table int64, w *grid.Window) error {
	if len(w.IDs) == 0 {
		return nil
	}
	d, err := s.current()
	if err != nil {
		return nil
	}
	locked, err := d.Share.LockedRows(ctx, table, w.IDs)
	if err != nil || len(locked) == 0 {
		return err
	}
	for i, rid := range w.IDs {
		if locked[rid] {
			w.Locked = append(w.Locked, i)
		}
	}
	return nil
}
