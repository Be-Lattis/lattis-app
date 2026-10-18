package workbook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Droits d'un classeur partagé (spécification de l'application
// collaborative, §8 ; contrat client de lattisd, §5 et §7) : lecture de la
// liste des droits, membres, invitations, rôles personnalisés, niveaux par
// nœud et filtres, restrictions, écarts et « Voir en tant que… ». Voir
// docs/collaboration/droits.md.

// Niveaux d'un rôle sur un nœud (lattis.role_grants.level).
const (
	LevelNone        = "none"
	LevelReader      = "reader"
	LevelParticipant = "participant"
	LevelEditor      = "editor"
)

// Nœuds de l'arborescence des droits (lattis.set_level).
const (
	NodeWorkbook = "workbook"
	NodeSheet    = "sheet"
	NodeTable    = "table"
)

// Rights est la gestion des droits d'un classeur (propriétaires du
// classeur, sauf Drift, Members et Recompile : concepteurs aussi). Chaque appel est une
// transaction du serveur ; le serveur recompile les règles (compile_acl)
// et journalise.
type Rights interface {
	// ACL lit les rôles (fournis et personnalisés) avec leurs niveaux et
	// leurs membres, les restrictions, les invitations en attente et les
	// tableaux (lattis.workbook_acl).
	ACL(ctx context.Context, workbookID string) (ACL, error)
	// Invite invite une adresse (lattis.invite) avec des rôles fournis et,
	// sur un serveur qui a la migration 0009, des noms de rôles
	// personnalisés (sans casse) ; un utilisateur déjà connu reçoit les
	// rôles tout de suite, une adresse inconnue un compte en attente.
	Invite(ctx context.Context, workbookID, email string, roles []string) (string, error)
	RevokeInvitation(ctx context.Context, id string) error
	// CreateRole crée un rôle personnalisé, sans droit tant qu'aucun niveau
	// n'est posé (lattis.create_role).
	CreateRole(ctx context.Context, workbookID, name string) (string, error)
	DropRole(ctx context.Context, roleID string) error
	// GrantRole et RevokeRole ajoutent ou retirent un membre d'un rôle.
	GrantRole(ctx context.Context, roleID, userID string) error
	RevokeRole(ctx context.Context, roleID, userID string) error
	// SetLevels pose des niveaux (et filtres) dans une seule transaction
	// (lattis.set_level pour chacun).
	SetLevels(ctx context.Context, levels []Level) error
	// AddRestriction ajoute une restriction (roles vide : tous les rôles
	// sauf propriétaires et concepteurs).
	AddRestriction(ctx context.Context, workbookID string, r RestrictionRule) (string, error)
	DropRestriction(ctx context.Context, id string) error
	// Drift liste les tableaux dont les règles ou droits ont changé hors de
	// Lattis (lattis.check_drift).
	Drift(ctx context.Context, workbookID string) ([]Drift, error)
	// Members liste les membres du classeur et leurs rôles
	// (lattis.workbook_members : propriétaires et concepteurs) ;
	// ErrUnsupported sur un serveur sans la migration 0009.
	Members(ctx context.Context, workbookID string) ([]WorkbookMember, error)
	// Recompile recompile les droits du classeur sans rien changer
	// (lattis.compile_acl : propriétaires et concepteurs) : remet d'aplomb
	// les écarts (les règles ajoutées à la main restent).
	Recompile(ctx context.Context, workbookID string) error
}

// ErrUnsupported : le serveur ne propose pas cette fonction (migration
// manquante) ; l'appelant garde l'ancien comportement.
var ErrUnsupported = errors.New("fonction absente de ce serveur (mise à jour de lattisd nécessaire)")

// WorkbookMember est un membre du classeur (lattis.workbook_members) :
// Roles nomme ses rôles, fournis (owner, reader…) ou personnalisés (nom).
type WorkbookMember struct {
	UserID      string   `json:"user_id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	PGRole      string   `json:"pg_role"`
	Roles       []string `json:"roles"`
}

// ACL est la liste des droits d'un classeur (lattis.workbook_acl).
type ACL struct {
	Workbook     ACLWorkbook       `json:"workbook"`
	Tables       []ACLTable        `json:"tables"`
	Roles        []Role            `json:"roles"`
	Restrictions []RestrictionRule `json:"restrictions"`
	Invitations  []Invitation      `json:"invitations"`
}

// ACLWorkbook décrit le classeur.
type ACLWorkbook struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Mode    string `json:"mode"`
	State   string `json:"state"`
	Version int    `json:"version"`
	Schema  string `json:"schema"`
}

// ACLTable est un tableau du classeur ; SheetID est sa feuille pour le
// serveur (SheetKey ; vide : publié sans feuille, avant la migration 0009).
type ACLTable struct {
	TableID string `json:"table_id"`
	Name    string `json:"name"`
	SheetID string `json:"sheet_id"`
}

// Role est un rôle du classeur : fourni (Builtin : owner, designer,
// editor, participant, reader) ou personnalisé (Builtin vide).
type Role struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Builtin string   `json:"builtin"`
	Grants  []Grant  `json:"grants"`
	Members []Member `json:"members"`
}

// Grant est le niveau propre d'un rôle sur un nœud ; les filtres ne se
// posent que sur un tableau.
type Grant struct {
	NodeKind    string `json:"node_kind"`
	NodeID      string `json:"node_id"`
	Level       string `json:"level"`
	ReadFilter  string `json:"read_filter"`
	WriteFilter string `json:"write_filter"`
}

// Member est un membre d'un rôle.
type Member struct {
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// RestrictionRule est une restriction (lattis.restrictions) : elle retire
// des droits sur un tableau (règle AS RESTRICTIVE), pour les rôles
// AppliesTo (vide : tous sauf propriétaires et concepteurs).
type RestrictionRule struct {
	ID        string   `json:"id"`
	TableID   string   `json:"table_id"`
	Command   string   `json:"command"` // select, insert, update, delete, all
	AppliesTo []string `json:"applies_to"`
	Predicate string   `json:"predicate"`
	Label     string   `json:"label"`
}

// Invitation est une invitation en attente.
type Invitation struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
	ExpiresAt string   `json:"expires_at"`
}

// Level est un niveau à poser : Level vide retire le niveau propre du
// nœud (retour à l'héritage ; « aucun » au niveau du classeur).
type Level struct {
	RoleID      string `json:"roleId"`
	NodeKind    string `json:"nodeKind"`
	NodeID      string `json:"nodeId"`
	Level       string `json:"level"`
	ReadFilter  string `json:"readFilter"`
	WriteFilter string `json:"writeFilter"`
}

// Drift est un tableau dont les règles ou les droits ont changé hors de
// Lattis ; ForeignPolicies : règles posées à la main.
type Drift struct {
	TableID         string   `json:"table_id"`
	Name            string   `json:"name"`
	ForeignPolicies []string `json:"foreign_policies"`
}

// Commandes d'une restriction.
var restrictionCommands = []string{"select", "insert", "update", "delete", "all"}

// CheckLevel vérifie un niveau avant l'appel (contrôle local ; le serveur
// vérifie de nouveau).
func CheckLevel(l Level) error {
	switch l.NodeKind {
	case NodeWorkbook, NodeSheet, NodeTable:
	default:
		return fmt.Errorf("nœud inconnu : %q (workbook, sheet ou table)", l.NodeKind)
	}
	switch l.Level {
	case "", LevelNone, LevelReader, LevelParticipant, LevelEditor:
	default:
		return fmt.Errorf("niveau inconnu : %q (aucun, lecteur, participant ou éditeur)", l.Level)
	}
	if l.NodeKind != NodeTable && (strings.TrimSpace(l.ReadFilter) != "" || strings.TrimSpace(l.WriteFilter) != "") {
		return errors.New("les filtres se posent sur un tableau")
	}
	return nil
}

// CheckRestriction vérifie une restriction avant l'appel.
func CheckRestriction(r RestrictionRule) error {
	ok := false
	for _, c := range restrictionCommands {
		ok = ok || c == r.Command
	}
	if !ok {
		return fmt.Errorf("commande inconnue : %q (select, insert, update, delete ou all)", r.Command)
	}
	if r.TableID == "" {
		return errors.New("tableau manquant")
	}
	if strings.TrimSpace(r.Predicate) == "" {
		return errors.New("condition manquante")
	}
	return nil
}

// --- « Voir en tant que… » -------------------------------------------------

// Preview est ce que verrait un membre d'un tableau (au plus 1 000 lignes,
// lues en lecture seule sous son rôle) : une ligne par objet JSON, colonnes
// par leur nom (colonnes système comprises).
type Preview struct {
	Rows      []map[string]any `json:"rows"`
	Truncated bool             `json:"truncated"`
}

// Previewer lit un tableau comme le verrait un membre (POST
// preview_endpoint de lattisd, contrat client §7) : propriétaires et
// concepteurs.
type Previewer interface {
	Preview(ctx context.Context, workbookID, userID, tableID string) (Preview, error)
}

// ErrNoPreview : le serveur ne propose pas « Voir en tant que… » (pas de
// preview_endpoint dans sa découverte).
var ErrNoPreview = errors.New("ce serveur ne propose pas « Voir en tant que… »")

// HTTPPreview est le Previewer de lattisd : POST endpoint avec le jeton
// d'accès de l'utilisateur (le même que pour PostgreSQL).
func HTTPPreview(hc *http.Client, endpoint string, token func(ctx context.Context) (string, error)) Previewer {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return httpPreview{hc, endpoint, token}
}

type httpPreview struct {
	hc       *http.Client
	endpoint string
	token    func(ctx context.Context) (string, error)
}

func (p httpPreview) Preview(ctx context.Context, workbookID, userID, tableID string) (Preview, error) {
	if p.endpoint == "" {
		return Preview{}, ErrNoPreview
	}
	tok, err := p.token(ctx)
	if err != nil {
		return Preview{}, err
	}
	body, _ := json.Marshal(map[string]string{"workbook_id": workbookID, "user_id": userID, "table_id": tableID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return Preview{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.hc.Do(req)
	if err != nil {
		return Preview{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return Preview{}, err
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Description string `json:"error_description"`
		}
		_ = json.Unmarshal(raw, &e)
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return Preview{}, errors.New("« Voir en tant que… » : jeton refusé, reconnectez-vous")
		case http.StatusForbidden:
			return Preview{}, errors.New("« Voir en tant que… » est réservé aux propriétaires et concepteurs du classeur")
		case http.StatusNotFound:
			return Preview{}, errors.New("« Voir en tant que… » : classeur, tableau ou membre inconnu")
		}
		if e.Description == "" {
			e.Description = resp.Status
		}
		return Preview{}, fmt.Errorf("« Voir en tant que… » : %s", e.Description)
	}
	var out Preview
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&out); err != nil {
		return Preview{}, fmt.Errorf("« Voir en tant que… » : réponse illisible : %w", err)
	}
	if out.Rows == nil {
		out.Rows = []map[string]any{}
	}
	return out, nil
}
