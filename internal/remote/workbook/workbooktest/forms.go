package workbooktest

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Formulaires (docs/conception/partager-comme-formulaire.md, §10) : le
// serveur ne les a pas encore ; ce faux suit le contrat écrit pour lui.
// create_form (brouillon, propriétaires et concepteurs, classeur
// collaboratif publié, tableau connu), update_form (version, 40001),
// set_form_state (open revérifie la définition), drop_form, list_forms
// (tout pour les propriétaires et concepteurs ; formulaires ouverts sans
// les chiffres pour les autres membres), form_preview_url, audience. Une
// définition invalide est refusée en 22023 avec DETAIL {"fields": {…}}.
// NoForms imite un serveur sans les formulaires (42883) ; FormsPublic
// permet le lien public (forms_public). Notification « *form:<id> ».

// ErrDetail imite une erreur du serveur avec un DETAIL (RAISE … USING
// DETAIL), comme pgconn.PgError.
type ErrDetail struct{ Code, Msg, Detail string }

func (e ErrDetail) Error() string { return "ERROR: " + e.Msg + " (SQLSTATE " + e.Code + ")" }

// SQLState rend le code de l'erreur.
func (e ErrDetail) SQLState() string { return e.Code }

// ErrDetail rend le DETAIL de l'erreur.
func (e ErrDetail) ErrDetail() string { return e.Detail }

type fform struct {
	id, code, workbook, table string
	state, access             string
	def                       workbook.FormDefinition
	audience                  []string
	maxPer                    *int
	version                   int
	responses                 int
	dropped                   bool
}

// Forms rend les formulaires d'un classeur (essais) : identifiant, état,
// version.
func (f *Fake) Forms(workbookID string) []workbook.Form {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []workbook.Form
	for _, x := range f.forms {
		if x.workbook == workbookID {
			out = append(out, workbook.Form{ID: x.id, Code: x.code, TableID: x.table, Title: x.def.Appearance.Title, State: x.state, Access: x.access, Version: x.version})
		}
	}
	return out
}

func (s *session) formsCall(name string) error {
	if err := s.begin(name); err != nil {
		return err
	}
	if s.f.NoForms {
		return ErrServer{"42883", "function lattis." + name + " does not exist"}
	}
	return nil
}

// formBook : classeur et tableau d'un formulaire à créer, avec les droits
// (mu tenu).
func (s *session) formBook(workbookID string) (*book, error) {
	b := s.f.books[workbookID]
	if b == nil || len(s.roles(b)) == 0 {
		return nil, ErrServer{"P0002", "classeur inconnu"}
	}
	if !s.has(b, "owner", "designer") {
		return nil, ErrPrivilege{"réservé aux propriétaires et concepteurs du classeur"}
	}
	if b.mode != workbook.Collaborative {
		return nil, ErrServer{"55000", "un classeur figé ne reçoit pas de lignes"}
	}
	if b.state == "draft" {
		return nil, ErrServer{"55000", "classeur non publié"}
	}
	return b, nil
}

func (s *session) formTable(b *book, tableID string) *table {
	for _, t := range b.schemas[b.schema].tables {
		if t.id == tableID {
			return t
		}
	}
	return nil
}

// checkForm vérifie une définition comme le serveur (§10.2) ; refus 22023
// avec la raison par colonne.
func (s *session) checkForm(b *book, t *table, def workbook.FormDefinition, access string) error {
	fields := map[string]string{}
	bad := func(col, why string) {
		if col == "" {
			col = "_"
		}
		if _, ok := fields[col]; !ok {
			fields[col] = why
		}
	}
	if def.Format != 1 {
		bad("", "format inconnu")
	}
	if def.Lang != "fr" && def.Lang != "en" {
		bad("", "langue : fr ou en")
	}
	a := def.Appearance
	if strings.TrimSpace(a.Title) == "" || len([]rune(a.Title)) > 120 {
		bad("", "titre : 1 à 120 caractères")
	}
	if len([]rune(a.Intro)) > 2000 {
		bad("", "introduction : 2 000 caractères au plus")
	}
	if !slices.Contains(workbook.FormAccents, a.Accent) {
		bad("", "couleur hors de la palette : "+a.Accent)
	}
	if len([]rune(a.SubmitLabel)) > 40 {
		bad("", "bouton : 40 caractères au plus")
	}
	if strings.Contains(a.Thanks, "](") || strings.Contains(a.Thanks, "http") {
		bad("", "remerciement : ni lien ni redirection")
	}
	switch access {
	case workbook.FormMembers, workbook.FormAudience, workbook.FormServerAccounts:
	case workbook.FormPublic:
		if !s.f.FormsPublic {
			return ErrPrivilege{"le lien public n'est pas permis sur ce serveur"}
		}
	default:
		bad("", "mode d'accès inconnu : "+access)
	}
	col := func(id string) *workbook.RemoteColumn {
		for i := range t.cols {
			if t.cols[i].ID == id {
				return &t.cols[i]
			}
		}
		return nil
	}
	used := map[string]bool{}
	use := func(id string) bool {
		if id == "" || col(id) == nil {
			bad(id, "colonne inconnue")
			return false
		}
		if used[id] {
			bad(id, "colonne citée deux fois")
			return false
		}
		used[id] = true
		return true
	}
	sections, params := 0, map[string]bool{}
	for _, fd := range def.Fields {
		switch fd.Kind {
		case workbook.FormFieldSection:
			sections++
			if strings.TrimSpace(fd.Title) == "" {
				bad("", "intertitre sans texte")
			}
			continue
		case workbook.FormFieldPosition:
			if fd.Column != "" {
				if use(fd.Column) && col(fd.Column).Type != "text" {
					bad(fd.Column, "position dans une seule colonne : colonne de texte")
				}
			} else {
				for _, c := range []string{fd.Latitude, fd.Longitude} {
					if use(c) && col(c).Type != "decimal" {
						bad(c, "latitude et longitude : colonnes décimales")
					}
				}
			}
			continue
		case "":
		default:
			bad("", "genre de champ inconnu : "+fd.Kind)
			continue
		}
		if !use(fd.Column) {
			continue
		}
		if c := col(fd.Column); c.Key && c.Type == "integer" {
			bad(fd.Column, "la clé numérotée est posée par le serveur")
		}
		if len([]rune(fd.Label)) > 200 || len([]rune(fd.Help)) > 500 {
			bad(fd.Column, "libellé ou aide trop long")
		}
		if fd.List != nil && fd.List.TableID == "" && len(fd.List.Values) == 0 {
			bad(fd.Column, "liste vide")
		}
		if fd.List != nil && fd.List.TableID != "" {
			src := s.formTable(b, fd.List.TableID)
			if src == nil || !slices.ContainsFunc(src.cols, func(c workbook.RemoteColumn) bool { return c.ID == fd.List.Column }) {
				bad(fd.Column, "liste : colonne source inconnue")
			}
		}
		if fd.Param != "" {
			if params[fd.Param] {
				bad(fd.Column, "nom court en double : "+fd.Param)
			}
			params[fd.Param] = true
		}
		if fd.Default != nil && fd.Default.Token != "" && !slices.Contains([]string{workbook.TokenToday, workbook.TokenNow, workbook.TokenYear, workbook.TokenRespondent}, fd.Default.Token) {
			bad(fd.Column, "valeur par défaut : jeton inconnu")
		}
		if fd.Default != nil && fd.Default.Token == workbook.TokenRespondent && access == workbook.FormPublic {
			bad(fd.Column, "pas de répondant connu avec le lien public")
		}
	}
	if sections > 20 {
		bad("", "20 sections au plus")
	}
	for _, sv := range def.ServerValues {
		if !use(sv.Column) {
			continue
		}
		switch sv.Value.Token {
		case "":
			if sv.Value.Fixed == nil {
				bad(sv.Column, "valeur posée par le serveur vide")
			}
		case workbook.TokenReceivedAt, workbook.TokenFormTitle, workbook.TokenNextNumber:
		case workbook.TokenRespondent:
			if access == workbook.FormPublic {
				bad(sv.Column, "pas de répondant connu avec le lien public")
			}
		default:
			bad(sv.Column, "jeton inconnu : "+sv.Value.Token)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	detail, _ := json.Marshal(map[string]any{"fields": fields})
	var msgs []string
	for _, v := range fields {
		msgs = append(msgs, v)
	}
	slices.Sort(msgs)
	return ErrDetail{"22023", "définition invalide : " + strings.Join(msgs, " ; "), string(detail)}
}

func (s *session) CreateForm(ctx context.Context, workbookID, tableID string, p workbook.FormSpec) (workbook.FormCreated, error) {
	s.f.mu.Lock()
	if err := s.formsCall("create_form"); err != nil {
		s.f.mu.Unlock()
		return workbook.FormCreated{}, err
	}
	b, err := s.formBook(workbookID)
	if err != nil {
		s.f.mu.Unlock()
		return workbook.FormCreated{}, err
	}
	t := s.formTable(b, tableID)
	if t == nil {
		s.f.mu.Unlock()
		return workbook.FormCreated{}, ErrServer{"P0002", "tableau inconnu"}
	}
	if err := s.checkForm(b, t, p.Definition, p.Access); err != nil {
		s.f.mu.Unlock()
		return workbook.FormCreated{}, err
	}
	x := &fform{id: uuid.NewString(), workbook: b.id, table: t.id, state: workbook.FormDraft, access: p.Access, def: p.Definition,
		audience: slices.Clone(p.Audience), maxPer: p.MaxPerRespondent, version: 1}
	x.code = strings.ToUpper(strings.ReplaceAll(x.id, "-", ""))[:10]
	s.f.forms = append(s.f.forms, x)
	ch := "wb_" + b.slug
	s.f.mu.Unlock()
	s.f.notify(ch, "*form:"+x.id)
	return workbook.FormCreated{ID: x.id, Code: x.code, URL: formURL(x)}, nil
}

func formURL(x *fform) string { return "https://lattis.example.test/f/" + x.code }

// form : formulaire géré par l'utilisateur (mu tenu).
func (s *session) form(id string) (*fform, *book, error) {
	for _, x := range s.f.forms {
		if x.id == id && !x.dropped {
			b := s.f.books[x.workbook]
			if b == nil || len(s.roles(b)) == 0 {
				break
			}
			if !s.has(b, "owner", "designer") {
				return nil, nil, ErrPrivilege{"réservé aux propriétaires et concepteurs du classeur"}
			}
			return x, b, nil
		}
	}
	return nil, nil, ErrServer{"P0002", "formulaire inconnu"}
}

// formDo applique fn à un formulaire géré, puis notifie « *form:<id> ».
func (s *session) formDo(name, id string, fn func(x *fform, b *book) error) error {
	s.f.mu.Lock()
	if err := s.formsCall(name); err != nil {
		s.f.mu.Unlock()
		return err
	}
	x, b, err := s.form(id)
	if err == nil {
		err = fn(x, b)
	}
	ch := ""
	if b != nil {
		ch = "wb_" + b.slug
	}
	s.f.mu.Unlock()
	if err == nil {
		s.f.notify(ch, "*form:"+id)
	}
	return err
}

func (s *session) UpdateForm(ctx context.Context, id string, p workbook.FormChange, version int) (int, error) {
	var out int
	err := s.formDo("update_form", id, func(x *fform, b *book) error {
		if version != x.version {
			return ErrServer{"40001", "formulaire modifié entre-temps"}
		}
		def, access := x.def, x.access
		if p.Definition != nil {
			def = *p.Definition
		}
		if p.Access != nil {
			access = *p.Access
		}
		t := s.formTable(b, x.table)
		if t == nil {
			return ErrServer{"P0002", "tableau inconnu"}
		}
		if err := s.checkForm(b, t, def, access); err != nil {
			return err
		}
		x.def, x.access = def, access
		if p.MaxPerRespondent != nil {
			x.maxPer = p.MaxPerRespondent
		}
		x.version++
		out = x.version
		return nil
	})
	return out, err
}

func (s *session) SetFormState(ctx context.Context, id, state string) error {
	return s.formDo("set_form_state", id, func(x *fform, b *book) error {
		switch state {
		case workbook.FormOpen:
			t := s.formTable(b, x.table)
			if t == nil {
				return ErrServer{"55000", "le tableau du formulaire n'existe plus"}
			}
			if err := s.checkForm(b, t, x.def, x.access); err != nil {
				return err
			}
		case workbook.FormPaused, workbook.FormClosed:
			if x.state == workbook.FormDraft && state == workbook.FormPaused {
				return ErrServer{"22023", "un brouillon ne se met pas en pause"}
			}
		default:
			return ErrServer{"22023", "état inconnu : " + state}
		}
		x.state = state
		return nil
	})
}

func (s *session) DropForm(ctx context.Context, id string) error {
	return s.formDo("drop_form", id, func(x *fform, b *book) error {
		x.dropped = true
		return nil
	})
}

func (s *session) FormsSettings(ctx context.Context) (workbook.FormsSettings, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.formsCall("forms_settings"); err != nil {
		return workbook.FormsSettings{}, err
	}
	return workbook.FormsSettings{Forms: true, FormsPublic: s.f.FormsPublic}, nil
}

func (s *session) ListForms(ctx context.Context, workbookID string) ([]workbook.Form, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.formsCall("list_forms"); err != nil {
		return nil, err
	}
	b := s.f.books[workbookID]
	if b == nil || len(s.roles(b)) == 0 {
		return nil, ErrPrivilege{"aucun accès à ce classeur"}
	}
	manage := s.has(b, "owner", "designer")
	out := []workbook.Form{}
	for _, x := range s.f.forms {
		if x.workbook != workbookID || x.dropped {
			continue
		}
		f := workbook.Form{ID: x.id, Code: x.code, URL: formURL(x), TableID: x.table, Title: x.def.Appearance.Title, State: x.state, Access: x.access, Manageable: manage}
		if manage {
			def := x.def
			f.Responses, f.Version, f.Definition, f.MaxPerRespondent = x.responses, x.version, &def, x.maxPer
			for _, p := range x.audience {
				e := workbook.FormAudienceEntry{Principal: p, Kind: "user", Name: p + "@example.test"}
				if t := s.f.team(p); t != nil {
					e.Kind, e.Name = "team", t.name
				}
				f.Audience = append(f.Audience, e)
			}
		} else if x.state != workbook.FormOpen || !s.formOpenTo(x, b) {
			continue
		}
		out = append(out, f)
	}
	return out, nil
}

// formOpenTo dit si l'utilisateur peut répondre au formulaire (mu tenu).
func (s *session) formOpenTo(x *fform, b *book) bool {
	switch x.access {
	case workbook.FormMembers:
		return len(s.roles(b)) > 0
	case workbook.FormAudience:
		for _, p := range x.audience {
			if p == s.user {
				return true
			}
			if t := s.f.team(p); t != nil && slices.Contains(t.members, s.user) {
				return true
			}
		}
		return false
	}
	return true
}

func (s *session) FormPreviewURL(ctx context.Context, id string) (string, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if err := s.formsCall("form_preview_url"); err != nil {
		return "", err
	}
	x, _, err := s.form(id)
	if err != nil {
		return "", err
	}
	return formURL(x) + "?p=apercu-" + x.id[:8], nil
}

func (s *session) FormAudienceAdd(ctx context.Context, id, principal string) error {
	return s.formDo("form_audience_add", id, func(x *fform, b *book) error {
		if s.f.team(principal) == nil && !s.f.users[principal] {
			return ErrServer{"P0002", "personne ou équipe inconnue"}
		}
		if !slices.Contains(x.audience, principal) {
			x.audience = append(x.audience, principal)
		}
		return nil
	})
}

func (s *session) FormAudienceAddByEmail(ctx context.Context, id, email string) error {
	return s.formDo("form_audience_add_by_email", id, func(x *fform, b *book) error {
		user := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(email)), "@example.test")
		if !s.f.users[user] {
			return ErrServer{"P0002", fmt.Sprintf("aucun compte pour %s", email)}
		}
		if !slices.Contains(x.audience, user) {
			x.audience = append(x.audience, user)
		}
		return nil
	})
}

func (s *session) FormAudienceRemove(ctx context.Context, id, principal string) error {
	return s.formDo("form_audience_remove", id, func(x *fform, b *book) error {
		x.audience = slices.DeleteFunc(x.audience, func(p string) bool { return p == principal })
		return nil
	})
}

// UploadFormImage imite POST /api/forms/{id}/images (workbook.FormImages) :
// rend un identifiant d'image ; LastFormImage garde le dernier envoi.
func (s *session) UploadFormImage(ctx context.Context, formID, kind, contentType string, data []byte) (string, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	if _, _, err := s.form(formID); err != nil {
		return "", err
	}
	if kind != "logo" && kind != "banner" {
		return "", fmt.Errorf("image : genre inconnu %q", kind)
	}
	s.f.LastFormImage = kind + ":" + contentType
	return uuid.NewString(), nil
}
