package workbook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Formulaires d'un tableau publié (docs/conception/partager-comme-formulaire.md,
// §10 ; lot 1 du §11) : le serveur garde la définition, sert la page
// /f/<code>, valide les réponses et insère les lignes sous l'identité
// technique du formulaire. L'application conçoit et gère : création
// (brouillon), modification (version), état, suppression, audience, lien
// d'aperçu, images (logo, bannière). Les colonnes sont citées par leur
// column_id du serveur (lattis_meta_columns, §14), jamais par leur nom.

// États d'un formulaire (lattis.forms.state).
const (
	FormDraft  = "draft"
	FormOpen   = "open"
	FormPaused = "paused"
	FormClosed = "closed"
)

// Modes d'accès (lattis.forms.access) : membres du classeur, personnes et
// équipes choisies, tout compte du serveur (§15, réponse 2), lien public
// (lot 3, selon forms_public du serveur).
const (
	FormMembers        = "members"
	FormAudience       = "audience"
	FormServerAccounts = "server_accounts"
	FormPublic         = "public"
)

// FormAccents : noms de la palette de couleurs d'accent (§4.1), dont le
// contraste est vérifié côté application (forms.ts) et côté serveur.
var FormAccents = []string{"green", "teal", "blue", "indigo", "purple", "red", "orange", "slate"}

// Jetons des valeurs par défaut (§3.2) et des valeurs posées par le
// serveur (§3.1, §15).
const (
	TokenToday       = "today"
	TokenNow         = "now"
	TokenYear        = "year"
	TokenRespondent  = "respondent"
	TokenReceivedAt  = "received_at"
	TokenFormTitle   = "form_title"
	TokenNextNumber  = "next_number"
	FormFieldSection = "section"
	// FormFieldPosition : champ « Position » (§15, réponse 7), rempli par le
	// navigateur sur demande du répondant (« Ma position ») : deux colonnes
	// décimales (Latitude, Longitude) ou une colonne texte « lat, lon »
	// (Column).
	FormFieldPosition = "position"
)

// FormDefinition est la définition d'un formulaire (definition jsonb, §10.2).
type FormDefinition struct {
	Format       int               `json:"format"`
	Lang         string            `json:"lang"`
	Appearance   FormAppearance    `json:"appearance"`
	Fields       []FormField       `json:"fields"`
	ServerValues []FormServerValue `json:"server_values"`
}

// FormAppearance est la personnalisation du §4 ; Logo et Banner sont des
// identifiants d'images (POST /api/forms/<id>/images), vides : aucune
// (logo : celui du serveur).
type FormAppearance struct {
	Title         string `json:"title"`
	Intro         string `json:"intro"`
	Logo          string `json:"logo,omitempty"`
	Banner        string `json:"banner,omitempty"`
	Accent        string `json:"accent"`
	SubmitLabel   string `json:"submit_label"`
	Thanks        string `json:"thanks"`
	Another       bool   `json:"another"`
	ClosedMessage string `json:"closed_message"`
	Privacy       string `json:"privacy"`
	Paged         bool   `json:"paged"`
}

// FormField est un champ (Column), un intertitre (Kind « section » : Title,
// Description) ou une position (Kind « position » : Latitude et
// Longitude, ou Column texte). Param : nom court du paramètre de
// pré-remplissage par l'adresse (§3.3 bis), Locked : « verrouillé par le
// lien ». Hidden : valeur par défaut non modifiable, champ invisible.
type FormField struct {
	Kind        string     `json:"kind,omitempty"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Column      string     `json:"column,omitempty"`
	Latitude    string     `json:"latitude,omitempty"`
	Longitude   string     `json:"longitude,omitempty"`
	Label       string     `json:"label,omitempty"`
	Help        string     `json:"help,omitempty"`
	Placeholder string     `json:"placeholder,omitempty"`
	Required    bool       `json:"required,omitempty"`
	Widget      string     `json:"widget,omitempty"`
	Default     *FormValue `json:"default,omitempty"`
	Min         *float64   `json:"min,omitempty"`
	Max         *float64   `json:"max,omitempty"`
	Step        *float64   `json:"step,omitempty"`
	MaxLength   *int       `json:"max_length,omitempty"`
	Format      string     `json:"format,omitempty"`
	Unit        string     `json:"unit,omitempty"`
	List        *FormList  `json:"list,omitempty"`
	Param       string     `json:"param,omitempty"`
	Locked      bool       `json:"locked,omitempty"`
	Hidden      bool       `json:"hidden,omitempty"`
}

// FormValue est une valeur par défaut ou posée par le serveur : un jeton
// (Token*) ou une valeur fixe.
type FormValue struct {
	Token string `json:"token,omitempty"`
	Fixed any    `json:"fixed,omitempty"`
}

// FormList est la liste d'un champ : valeurs fixes (copiées ; CopiedAt et
// Source si elles viennent d'une plage locale, pour « Mettre à jour »), ou
// colonne d'un tableau publié du même classeur, relue par le serveur.
type FormList struct {
	Values   []string `json:"values,omitempty"`
	TableID  string   `json:"table_id,omitempty"`
	Column   string   `json:"column,omitempty"`
	CopiedAt string   `json:"copied_at,omitempty"`
	Source   string   `json:"source,omitempty"`
}

// FormServerValue est une valeur posée par le serveur sur une colonne.
type FormServerValue struct {
	Column string    `json:"column"`
	Value  FormValue `json:"value"`
}

// FormSpec décrit un formulaire à créer (p de lattis.create_form) :
// définition, accès, audience (personnes et équipes par identifiant), et
// options (lot 3 : dates, plafond, réponses par personne, modification).
type FormSpec struct {
	Definition FormDefinition `json:"definition"`
	Access     string         `json:"access"`
	Audience   []string       `json:"audience"`
	OpensAt    *string        `json:"opens_at,omitempty"`
	ClosesAt   *string        `json:"closes_at,omitempty"`
	// MaxResponses : plafond (nil : sans plafond).
	MaxResponses *int `json:"max_responses,omitempty"`
	// MaxPerRespondent : réponses par personne (nil : sans limite, 1 : une
	// seule, n), §15 réponse 5.
	MaxPerRespondent *int `json:"max_per_respondent,omitempty"`
	// Edit : none, until_close ou une durée (« 7 days »).
	Edit string `json:"edit,omitempty"`
}

// FormChange modifie un formulaire (p de lattis.update_form) ; nil :
// inchangé.
type FormChange struct {
	Definition       *FormDefinition `json:"definition,omitempty"`
	Access           *string         `json:"access,omitempty"`
	OpensAt          *string         `json:"opens_at,omitempty"`
	ClosesAt         *string         `json:"closes_at,omitempty"`
	MaxResponses     *int            `json:"max_responses,omitempty"`
	MaxPerRespondent *int            `json:"max_per_respondent,omitempty"`
	Edit             *string         `json:"edit,omitempty"`
}

// FormCreated est le résultat de lattis.create_form.
type FormCreated struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	URL  string `json:"url"`
}

// FormAudienceEntry est une personne ou une équipe de l'audience.
type FormAudienceEntry struct {
	Principal string `json:"principal"`
	Kind      string `json:"kind"` // user, team
	Name      string `json:"name"`
}

// Form est un formulaire du classeur (lattis.list_forms). Les chiffres, la
// définition, la version et l'audience ne sont donnés qu'aux propriétaires
// et concepteurs (Manageable).
type Form struct {
	ID               string              `json:"id"`
	Code             string              `json:"code"`
	URL              string              `json:"url"`
	TableID          string              `json:"table_id"`
	Title            string              `json:"title"`
	State            string              `json:"state"`
	PauseReason      string              `json:"pause_reason"`
	Access           string              `json:"access"`
	Responses        int                 `json:"responses"`
	LastResponseAt   string              `json:"last_response_at"`
	Manageable       bool                `json:"manageable"`
	Version          int                 `json:"version,omitempty"`
	Definition       *FormDefinition     `json:"definition,omitempty"`
	Audience         []FormAudienceEntry `json:"audience,omitempty"`
	MaxPerRespondent *int                `json:"max_per_respondent,omitempty"`
}

// Forms est l'API SQL des formulaires (§10.3), sous l'identité de
// l'utilisateur. Un serveur sans les formulaires répond 42883 : voir
// FormError.
type Forms interface {
	// FormsSettings dit si le serveur sert les formulaires et permet le lien
	// public (lattis.forms_settings ; 42883 : serveur sans formulaires).
	FormsSettings(ctx context.Context) (FormsSettings, error)
	// CreateForm crée un formulaire en brouillon (propriétaires et
	// concepteurs) : identité technique et rôle interne du classeur.
	CreateForm(ctx context.Context, workbookID, tableID string, p FormSpec) (FormCreated, error)
	// UpdateForm modifie le formulaire de la version donnée et rend la
	// nouvelle ; 40001 : modifié entre-temps.
	UpdateForm(ctx context.Context, formID string, p FormChange, version int) (int, error)
	// SetFormState ouvre, met en pause ou ferme ; open revérifie la
	// définition sur la structure actuelle (22023 avec les champs en cause).
	SetFormState(ctx context.Context, formID, state string) error
	// DropForm supprime le formulaire (les lignes restent).
	DropForm(ctx context.Context, formID string) error
	// ListForms liste les formulaires du classeur.
	ListForms(ctx context.Context, workbookID string) ([]Form, error)
	// FormPreviewURL rend un lien signé valable 10 minutes, qui montre la
	// vraie page sans enregistrer les réponses.
	FormPreviewURL(ctx context.Context, formID string) (string, error)
	// FormAudienceAdd ajoute une personne ou une équipe (identifiant),
	// FormAudienceAddByEmail une personne par son adresse (P0002 : adresse
	// sans compte), FormAudienceRemove la retire.
	FormAudienceAdd(ctx context.Context, formID, principal string) error
	FormAudienceAddByEmail(ctx context.Context, formID, email string) error
	FormAudienceRemove(ctx context.Context, formID, principal string) error
}

// FormsSettings : réglages des formulaires du serveur.
type FormsSettings struct {
	Forms       bool `json:"forms"`
	FormsPublic bool `json:"forms_public"`
}

// ErrNoForms : le serveur ne propose pas les formulaires (42883, ou forms
// absent de la découverte).
var ErrNoForms = errors.New("Ce serveur ne propose pas encore les formulaires.")

// ErrFormModified : le formulaire a changé depuis sa lecture (40001).
var ErrFormModified = errors.New("Ce formulaire a été modifié entre-temps : rouvrez-le pour repartir de sa dernière version.")

// FormInvalid est une définition refusée par le serveur (22023) : Fields
// donne la raison par column_id (DETAIL {"fields": {…}}).
type FormInvalid struct {
	Message string
	Fields  map[string]string
}

func (e *FormInvalid) Error() string {
	if len(e.Fields) == 0 {
		return "Formulaire refusé par le serveur : " + e.Message
	}
	var parts []string
	for _, r := range e.Fields {
		parts = append(parts, r)
	}
	return "Formulaire refusé par le serveur : " + strings.Join(parts, " ; ")
}

// errDetail rend le DETAIL d'une erreur du serveur ("" sans).
func errDetail(err error) string {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Detail
	}
	var d interface{ ErrDetail() string }
	if errors.As(err, &d) {
		return d.ErrDetail()
	}
	return ""
}

// FormError rend une erreur des fonctions des formulaires lisible (§10.3) ;
// nil reste nil, une coupure reste telle quelle.
func FormError(err error) error {
	if err == nil {
		return nil
	}
	code, ok := Rejected(err)
	if !ok {
		return err
	}
	msg := ServerMessage(err)
	switch code {
	case "42883":
		return ErrNoForms
	case "40001":
		return ErrFormModified
	case "42501":
		if strings.Contains(strings.ToLower(msg), "public") {
			return errors.New("Le lien public n'est pas autorisé par ce serveur : choisissez un autre mode d'accès.")
		}
		return errors.New("Réservé aux propriétaires et concepteurs du classeur.")
	case "55000":
		return fmt.Errorf("Un formulaire demande un tableau publié d'un classeur collaboratif (%s).", msg)
	case "P0002":
		return fmt.Errorf("Introuvable sur le serveur : %s.", strings.TrimSuffix(msg, "."))
	case "22023":
		inv := &FormInvalid{Message: msg}
		var d struct {
			Fields map[string]string `json:"fields"`
		}
		if json.Unmarshal([]byte(errDetail(err)), &d) == nil && len(d.Fields) > 0 {
			inv.Fields = d.Fields
		}
		return inv
	}
	return errors.New(msg)
}

// --- pgx ----------------------------------------------------------------

func (s pgServer) CreateForm(ctx context.Context, workbookID, tableID string, p FormSpec) (FormCreated, error) {
	var out FormCreated
	if p.Audience == nil {
		p.Audience = []string{}
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT lattis.create_form($1::uuid, $2::uuid, $3::jsonb)::text`, workbookID, tableID, p).Scan(&raw); err != nil {
			return err
		}
		return json.Unmarshal(raw, &out)
	})
	return out, err
}

func (s pgServer) UpdateForm(ctx context.Context, formID string, p FormChange, version int) (int, error) {
	var out struct {
		Version int `json:"version"`
	}
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT lattis.update_form($1::uuid, $2::jsonb, $3)::text`, formID, p, version).Scan(&raw); err != nil {
			return err
		}
		return json.Unmarshal(raw, &out)
	})
	return out.Version, err
}

func (s pgServer) SetFormState(ctx context.Context, formID, state string) error {
	return s.exec(ctx, `SELECT lattis.set_form_state($1::uuid, $2)`, formID, state)
}

func (s pgServer) DropForm(ctx context.Context, formID string) error {
	return s.exec(ctx, `SELECT lattis.drop_form($1::uuid)`, formID)
}

func (s pgServer) FormsSettings(ctx context.Context) (FormsSettings, error) {
	var out FormsSettings
	err := s.jsonCall(ctx, `SELECT lattis.forms_settings()::text`, &out)
	return out, err
}

func (s pgServer) ListForms(ctx context.Context, workbookID string) ([]Form, error) {
	out := []Form{}
	err := s.jsonCall(ctx, `SELECT lattis.list_forms($1::uuid)::text`, &out, workbookID)
	return out, err
}

func (s pgServer) FormPreviewURL(ctx context.Context, formID string) (string, error) {
	var u string
	err := s.tx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT lattis.form_preview_url($1::uuid)`, formID).Scan(&u)
	})
	return u, err
}

func (s pgServer) FormAudienceAdd(ctx context.Context, formID, principal string) error {
	return s.exec(ctx, `SELECT lattis.form_audience_add($1::uuid, $2::uuid)`, formID, principal)
}

func (s pgServer) FormAudienceAddByEmail(ctx context.Context, formID, email string) error {
	return s.exec(ctx, `SELECT lattis.form_audience_add_by_email($1::uuid, $2)`, formID, email)
}

func (s pgServer) FormAudienceRemove(ctx context.Context, formID, principal string) error {
	return s.exec(ctx, `SELECT lattis.form_audience_remove($1::uuid, $2::uuid)`, formID, principal)
}

// --- Images (POST /api/forms/{id}/images, §10.4) ------------------------

// FormImages envoie le logo ou la bannière d'un formulaire et rend
// l'identifiant de l'image, à mettre dans la définition.
type FormImages interface {
	UploadFormImage(ctx context.Context, formID, kind, contentType string, data []byte) (string, error)
}

// Bornes des images (§4.1) : le serveur réduit et retire les métadonnées.
const (
	MaxFormLogo   = 200 << 10
	MaxFormBanner = 1 << 20
)

// FormImageClient est l'envoi HTTP des images : émetteur (issuer de la
// découverte) et jeton d'accès de la session, comme PushClient.
type FormImageClient struct {
	Issuer     string
	Token      func(ctx context.Context) (string, error)
	Invalidate func()
	HTTP       *http.Client
}

func (c FormImageClient) UploadFormImage(ctx context.Context, formID, kind, contentType string, data []byte) (string, error) {
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 2 * time.Minute}
	}
	u := strings.TrimRight(c.Issuer, "/") + "/api/forms/" + url.PathEscape(formID) + "/images?kind=" + url.QueryEscape(kind)
	for try := 0; ; try++ {
		tok, err := c.Token(ctx)
		if err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(data))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", contentType)
		res, err := hc.Do(req)
		if err != nil {
			return "", err
		}
		if res.StatusCode == http.StatusUnauthorized && try == 0 && c.Invalidate != nil {
			res.Body.Close()
			c.Invalidate()
			continue
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		switch res.StatusCode {
		case http.StatusOK, http.StatusCreated:
			var out struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &out); err != nil || out.ID == "" {
				return "", fmt.Errorf("image : réponse illisible du serveur")
			}
			return out.ID, nil
		case http.StatusUnauthorized:
			return "", errors.New("image : session expirée sur ce serveur, reconnectez-vous")
		case http.StatusForbidden:
			return "", errors.New("image : réservé aux propriétaires et concepteurs du classeur")
		case http.StatusNotFound:
			return "", ErrNoForms
		case http.StatusRequestEntityTooLarge:
			return "", errors.New("image trop lourde : 200 Ko au plus pour un logo, 1 Mo pour une bannière")
		case http.StatusUnsupportedMediaType:
			return "", errors.New("image refusée : PNG, JPEG ou SVG seulement")
		}
		return "", fmt.Errorf("image : le serveur a répondu %d", res.StatusCode)
	}
}
