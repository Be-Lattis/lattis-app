package workbook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/store"
	"github.com/lalamefine/lattis/protocol"
)

// Messages des refus d'écriture sur un tableau publié (§5.4, §10).
var (
	ErrReadOnly = errors.New("Classeur publié en lecture seule")
	ErrOffline  = errors.New("Hors ligne, lecture seule")
	// ErrStructure refuse un changement de structure d'un tableau publié
	// d'un classeur collaboratif quand la synchronisation avec le serveur
	// n'est pas en place (§6.4 : la structure passe par le serveur,
	// internal/remote/collab, Engine.Structure).
	ErrStructure = errors.New("La structure d'un tableau partagé (colonnes, types, suppression) se modifie sur le serveur : attendez que le classeur soit connecté")
	// ErrDesign : seuls les propriétaires et les concepteurs changent la
	// structure d'un tableau partagé (§6.4, §8.4 du serveur).
	ErrDesign = errors.New("Seuls le propriétaire et les concepteurs du classeur modifient la structure d'un tableau partagé (colonnes, types, clé, nom, suppression)")
	// Droits d'un classeur collaboratif (§8.1).
	ErrReader   = errors.New("Tableau en lecture seule : vos droits ne permettent pas de le modifier")
	ErrNoAccess = errors.New("Tableau inaccessible avec vos droits")
	ErrNotYours = errors.New("Ligne d'un autre membre : en tant que participant, vous ne modifiez que vos propres lignes")
	// ErrCannotCreate : publication refusée, le compte n'a pas le droit de
	// créer de classeur sur ce serveur (Server.CanCreateWorkbooks).
	ErrCannotCreate = errors.New("Votre compte ne peut pas créer de classeur sur ce serveur : demandez à son propriétaire le rôle « créateur de classeurs »")
)

// Link est l'état d'un classeur partagé ouvert : ce qui vient du serveur
// (Info), les tableaux publiés, et la connexion (en ligne ou non). Il
// décide des écritures sur les tableaux publiés, pour la grille
// (canvas.Sharing) comme pour tout ce qui passe par le bus de commandes
// (oplog.Recorder.Gate : SQL, scripts, API, MCP, annulation).
//
// Règles :
//   - classeur local, ou tableau privé (non publié) : tout est permis ;
//   - hors ligne : les tableaux publiés d'un classeur figé sont en lecture
//     seule (§10) ; ceux d'un classeur collaboratif restent modifiables
//     selon les derniers droits connus, la file part à la reconnexion
//     (décision 0014) ;
//   - figé : lecture seule, sauf pour son auteur (propriétaire ou
//     concepteur), qui prépare la prochaine version en local puis
//     republie ; le tri reste permis à tous (local, jamais renvoyé) ;
//   - collaboratif : selon le niveau du tableau (lecteur : lecture seule),
//     les opérations partent dans la file d'envoi.
type Link struct {
	db *store.Store

	mu        sync.RWMutex
	info      *Info
	published map[int64]bool
	levels    map[int64]string // collaboratif : niveau par tableau local
	// filtered : tableaux dont la lecture est filtrée par les droits (filtre
	// d'un rôle, ou restriction de lecture) ; restricted : restrictions qui
	// visent l'utilisateur, par tableau.
	filtered   map[int64]bool
	restricted map[int64][]Restriction
	online     bool
	offline    string // raison de la coupure (affichage)
	// gone : le serveur répond, mais le classeur n'y est plus (GoneDeleted)
	// ou plus pour l'utilisateur (GoneRevoked) ; vide sinon (SetGone).
	gone       string
	newVersion int
	// Collaboratif : étapes en attente d'envoi et conflits ouverts
	// (SetQueue, tenus par la synchronisation) ; sending : la file part
	// (SetSending).
	pending, conflicts int
	sending            bool
}

// Attach lit l'état de partage d'un classeur ouvert (hors ligne tant que la
// connexion au serveur n'est pas confirmée par SetOnline).
func Attach(ctx context.Context, db *store.Store) (*Link, error) {
	l := &Link{db: db}
	return l, l.Reload(ctx)
}

// Reload relit l'état de partage dans la base (après une publication ou un
// chargement).
func (l *Link) Reload(ctx context.Context) error {
	info, err := ReadInfo(ctx, l.db.Read())
	if err != nil {
		return err
	}
	pub := map[int64]bool{}
	rs, err := l.db.Read().QueryContext(ctx, `SELECT tbl FROM _lattis_remote_tables WHERE published = 1`)
	if err != nil {
		return err
	}
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return err
		}
		pub[id] = true
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	levels := map[int64]string{}
	filtered := map[int64]bool{}
	restricted := map[int64][]Restriction{}
	rs, err = l.db.Read().QueryContext(ctx, `SELECT tbl, level, coalesce(filters, ''), coalesce(restrictions, '') FROM _lattis_capabilities WHERE tbl <> 0`)
	if err != nil {
		return err
	}
	for rs.Next() {
		var id int64
		var lv, filt, restr string
		if err := rs.Scan(&id, &lv, &filt, &restr); err != nil {
			rs.Close()
			return err
		}
		levels[id] = lv
		if restr != "" {
			var r []Restriction
			if json.Unmarshal([]byte(restr), &r) == nil {
				restricted[id] = r
			}
		}
		// Lecture filtrée (writeCapabilities, TableAccess.ReadsFiltered).
		filtered[id] = filt != ""
	}
	rs.Close()
	l.mu.Lock()
	l.info, l.published, l.levels, l.filtered, l.restricted = info, pub, levels, filtered, restricted
	if info != nil && l.newVersion <= info.Version {
		l.newVersion = 0
	}
	l.mu.Unlock()
	return rs.Err()
}

// Info rend une copie de la description du classeur partagé (nil : local).
func (l *Link) Info() *Info {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.info == nil {
		return nil
	}
	c := *l.info
	return &c
}

// SetOnline note l'état de la connexion au serveur (reason : pourquoi hors
// ligne).
func (l *Link) SetOnline(on bool, reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.gone == GoneDeleted {
		return // plus rien à joindre : ni en ligne ni « hors ligne »
	}
	l.online = on
	if on {
		reason = ""
	}
	l.offline = reason
}

// Classeur disparu du serveur (Link.SetGone, State.Gone) : à distinguer
// d'un serveur qui ne répond pas, qu'on réessaie.
const (
	// GoneDeleted : classeur supprimé du serveur (P0002, « classeur
	// inconnu ») ; définitif, le suivi s'arrête, plus rien n'est mis en
	// file : le cache n'est plus qu'un classeur local en attente d'une
	// copie.
	GoneDeleted = "deleted"
	// GoneRevoked : l'utilisateur n'a plus accès au classeur (42501 de
	// lattis.my_capabilities) ; ses tableaux publiés passent au niveau
	// « aucun », l'écoute reste posée (accès rendu : *acl).
	GoneRevoked = "revoked"
)

// GoneReason dit si err, réponse de lattis.my_capabilities, veut dire que
// le classeur a disparu pour l'utilisateur (GoneDeleted, GoneRevoked) ;
// vide pour une coupure ou une autre erreur. Le 42501 d'un réplica en
// retard est déjà relu sur rw (pgServer.with).
func GoneReason(err error) string {
	switch code, _ := Rejected(err); code {
	case "P0002":
		return GoneDeleted
	case "42501":
		return GoneRevoked
	}
	return ""
}

// SetGone note que le classeur a disparu du serveur (GoneDeleted,
// GoneRevoked) ou y est de nouveau (vide) ; vrai si cela change. Supprimé :
// hors ligne pour de bon, sans raison de coupure.
func (l *Link) SetGone(reason string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.gone == reason {
		return false
	}
	l.gone = reason
	if reason == GoneDeleted {
		l.online, l.offline, l.sending = false, "", false
	}
	return true
}

// Gone dit si le classeur a disparu du serveur (GoneDeleted, GoneRevoked).
func (l *Link) Gone() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.gone
}

// Online dit si le serveur répond.
func (l *Link) Online() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.online
}

// SetNewVersion note qu'une version plus récente est publiée (bandeau
// « Nouvelle version disponible ») ; sans effet si elle n'est pas plus
// récente que la version chargée.
func (l *Link) SetNewVersion(v int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.info == nil || v <= l.info.Version || v <= l.newVersion {
		return false
	}
	l.newVersion = v
	return true
}

// Unpublish retire un tableau des tableaux publiés (supprimé sur le
// serveur), sans attendre Reload.
func (l *Link) Unpublish(tbl int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.published, tbl)
	delete(l.levels, tbl)
}

// TableShare dit si un tableau est publié (canvas.Sharing).
func (l *Link) TableShare(tbl int64) string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	switch {
	case l.info == nil:
		return ""
	case l.published[tbl]:
		return "published"
	}
	return "private"
}

// TableLock refuse la modification d'un tableau publié en lecture seule
// (canvas.Sharing).
func (l *Link) TableLock(tbl int64) error {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lock(tbl)
}

func (l *Link) lock(tbl int64) error {
	if l.info == nil || !l.published[tbl] || l.gone == GoneDeleted {
		return nil // supprimé du serveur : données locales, libres
	}
	// Hors ligne : un classeur figé est en lecture seule ; un classeur
	// collaboratif reste modifiable avec les derniers droits connus, tout
	// part par la file à la reconnexion (décision 0014).
	if !l.online && l.info.Mode == Frozen {
		return ErrOffline
	}
	switch l.info.Mode {
	case Frozen:
		if l.info.CanDesign() {
			return nil
		}
		return ErrReadOnly
	default:
		switch l.levels[tbl] {
		case LevelReader:
			return ErrReader
		case LevelNone:
			return ErrNoAccess
		}
		return nil
	}
}

// participant dit si l'utilisateur est participant sur un tableau publié
// d'un classeur collaboratif (il ne modifie que ses lignes).
func (l *Link) participant(tbl int64) bool {
	return l.info != nil && l.info.Mode == Collaborative && l.published[tbl] && l.levels[tbl] == LevelParticipant
}

// foreign dit si la ligne id (lattis_id) d'un tableau est d'un autre
// membre : son auteur est connu et n'est pas l'utilisateur. Une ligne
// absente de _lattis_row_ids (ajoutée dans l'étape en cours) ou sans
// auteur (ajoutée sur ce poste, pas encore relue) est la sienne.
func (l *Link) foreign(id string) (bool, error) {
	var author sql.NullString
	err := l.db.Read().QueryRow(`SELECT lattis_author FROM _lattis_row_ids WHERE lattis_id = ?`, id).Scan(&author)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return author.Valid && author.String != "" && author.String != l.info.User, nil
}

// TableAccess donne le niveau de l'utilisateur sur un tableau publié d'un
// classeur collaboratif (vide sinon) et si sa lecture est filtrée par ses
// droits (canvas.TableInfo : « Vue filtrée par vos droits »).
func (l *Link) TableAccess(tbl int64) (level string, filtered bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.info == nil || l.info.Mode != Collaborative || !l.published[tbl] {
		return "", false
	}
	return l.levels[tbl], l.filtered[tbl]
}

// Restrictions rend les restrictions qui visent l'utilisateur sur un
// tableau.
func (l *Link) Restrictions(tbl int64) []Restriction {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]Restriction(nil), l.restricted[tbl]...)
}

// LockedRows dit, pour un participant, quelles lignes (clés locales rids)
// d'un tableau sont à d'autres membres (lecture seule, grisées au survol) ;
// nil si l'utilisateur n'est pas participant sur ce tableau.
func (l *Link) LockedRows(ctx context.Context, tbl int64, rids []int64) (map[int64]bool, error) {
	l.mu.RLock()
	part, user := l.participant(tbl), ""
	if l.info != nil {
		user = l.info.User
	}
	l.mu.RUnlock()
	if !part || len(rids) == 0 {
		return nil, nil
	}
	ids, err := json.Marshal(rids)
	if err != nil {
		return nil, err
	}
	rs, err := l.db.Read().QueryContext(ctx, `SELECT rid FROM _lattis_row_ids WHERE tbl = ? AND rid IN (SELECT value FROM json_each(?))
		AND coalesce(lattis_author, '') <> '' AND lattis_author <> ?`, tbl, string(ids), user)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[int64]bool{}
	for rs.Next() {
		var rid int64
		if err := rs.Scan(&rid); err != nil {
			return nil, err
		}
		out[rid] = true
	}
	return out, rs.Err()
}

// Gate décide d'une opération relevée sur un tableau publié
// (oplog.Recorder.Gate).
func (l *Link) Gate(op command.Op) (bool, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.info == nil {
		return true, nil // tableau publié hors d'un classeur partagé (essais du journal)
	}
	if l.gone == GoneDeleted {
		return false, nil // supprimé du serveur : rien ne part plus
	}
	frozen := l.info.Mode == Frozen
	if op.Table != 0 {
		if err := l.lock(op.Table); err != nil {
			if frozen && op.Kind == command.OpMove {
				return false, nil // tri local
			}
			return false, err
		}
		// Collaboratif : la structure d'un tableau publié (colonnes, type,
		// suppression) passe par les fonctions du serveur (§6.4), appelées
		// avant la validation locale par la synchronisation
		// (oplog.Recorder.SetStructure, collab.Engine.Structure : la porte ne
		// voit plus ces opérations). Sans elle, la changer en local
		// désaccorderait les deux copies.
		if !frozen && op.Kind == command.OpMeta && l.published[op.Table] {
			return false, ErrStructure
		}
		// Participant (§8.1) : ajouter des lignes, modifier et supprimer les
		// siennes seulement (contrôle local ; la RLS du serveur décide).
		if l.participant(op.Table) && op.Row != "" && (op.Kind == command.OpSet || op.Kind == command.OpDelete || op.Kind == command.OpMove) {
			other, err := l.foreign(op.Row)
			if err != nil {
				return false, err
			}
			if other {
				return false, ErrNotYours
			}
		}
	}
	// Figé : rien ne part, l'auteur republie la version entière.
	return !frozen, nil
}

// State est l'état de partage pour l'interface.
type State struct {
	Shared     bool   `json:"shared"`
	Server     string `json:"server"`
	ServerName string `json:"serverName"`
	WorkbookID string `json:"workbookId"`
	// Link : adresse à partager (<serveur>/w/<id>, contrat de lattisd §8).
	Link    string   `json:"link,omitempty"`
	Title   string   `json:"title"`
	Mode    string   `json:"mode"`
	Version int      `json:"version"`
	Roles   []string `json:"roles"`
	// CanDesign : propriétaire ou concepteur (Fichier › Republier).
	CanDesign bool `json:"canDesign"`
	// CanManage : propriétaire (écran « Partager » : membres, rôles,
	// restrictions).
	CanManage bool `json:"canManage"`
	Online    bool `json:"online"`
	// Offline : raison de la coupure, vide en ligne.
	Offline string `json:"offline,omitempty"`
	// Gone : le serveur répond mais le classeur n'y est plus
	// (GoneDeleted) ou plus pour l'utilisateur (GoneRevoked) ; vide sinon.
	Gone string `json:"gone,omitempty"`
	// ReadOnly : message du bandeau quand les tableaux publiés ne se
	// modifient pas (« Hors ligne, lecture seule », « Classeur publié en
	// lecture seule »), vide sinon.
	ReadOnly string `json:"readOnly,omitempty"`
	// NewVersion : version publiée plus récente que celle chargée (0 sinon).
	NewVersion int `json:"newVersion,omitempty"`
	Published  int `json:"published"`
	// Pending : étapes pas encore confirmées par le serveur ; Conflicts :
	// conflits en attente de décision (classeur collaboratif).
	Pending   int `json:"pending,omitempty"`
	Conflicts int `json:"conflicts,omitempty"`
	// Sending : la file part vers le serveur (barre d'état :
	// « Synchronisation… »).
	Sending bool `json:"sending,omitempty"`
}

// SetQueue note les étapes en attente d'envoi et les conflits ouverts
// (classeur collaboratif). state : l'état de partage change (conflits :
// bandeau) ; sync : la file change (barre d'état, SyncStatus, y compris
// hors ligne : « Hors ligne — 3 en attente »), sans relire la mise en page.
func (l *Link) SetQueue(pending, conflicts int) (state, sync bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	sync = l.pending != pending
	state = l.conflicts != conflicts
	l.pending, l.conflicts = pending, conflicts
	return state, sync
}

// SetSending note que la file part vers le serveur (ou plus) ; vrai si
// cela change.
func (l *Link) SetSending(on bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sending == on {
		return false
	}
	l.sending = on
	return true
}

// SyncStatus décrit la file d'envoi d'un classeur collaboratif pour la
// barre d'état (« Synchronisation… », « Hors ligne — 3 en attente »).
type SyncStatus struct {
	// Shared : classeur collaboratif ouvert depuis son serveur.
	Shared  bool `json:"shared"`
	Online  bool `json:"online"`
	Pending int  `json:"pending"`
	Sending bool `json:"sending"`
}

// Sync décrit la file d'envoi (classeur collaboratif ; Shared faux sinon).
func (l *Link) Sync() SyncStatus {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.info == nil || l.info.Mode != Collaborative || l.gone == GoneDeleted {
		return SyncStatus{} // supprimé : plus de file (le bandeau le dit)
	}
	return SyncStatus{Shared: true, Online: l.online, Pending: l.pending, Sending: l.sending}
}

// State décrit l'état de partage.
func (l *Link) State() State {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.info == nil {
		return State{Roles: []string{}}
	}
	s := State{
		Shared: true, Server: l.info.Server, ServerName: l.info.ServerName, WorkbookID: l.info.WorkbookID,
		Title: l.info.Title, Mode: l.info.Mode, Version: l.info.Version, Roles: append([]string{}, l.info.Roles...),
		CanDesign: l.info.CanDesign(), CanManage: l.info.CanManage(), Online: l.online, Offline: l.offline, Gone: l.gone, NewVersion: l.newVersion, Published: len(l.published),
		Pending: l.pending, Conflicts: l.conflicts, Sending: l.sending,
	}
	s.Link = protocol.WorkbookLink(s.Server, s.WorkbookID)
	switch {
	case l.gone == GoneDeleted:
	case !l.online && l.info.Mode == Frozen:
		s.ReadOnly = ErrOffline.Error()
	case l.info.Mode == Frozen && !s.CanDesign:
		s.ReadOnly = ErrReadOnly.Error()
	}
	return s
}
