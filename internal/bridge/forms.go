package bridge

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/protocol"
)

// Partager un tableau comme formulaire (Insertion › Formulaire, sur le
// tableau de la cellule active ;
// docs/conception/partager-comme-formulaire.md, lot 1). Le pont dit si le
// tableau peut avoir un formulaire (conditions du §2.2, avec l'action à
// proposer), donne au concepteur les colonnes publiées avec leur column_id
// du serveur, leurs listes de validation, la clé et l'onglet à choix, puis
// passe les appels au serveur (workbook.Forms). La définition elle-même se
// construit dans le frontend (forms.ts), qui rend aussi l'aperçu.

// Raisons pour lesquelles un tableau n'a pas de formulaire (FormInfo.Reason).
const (
	FormLocal    = "local"   // classeur local : Publier sur un serveur…
	FormPrivate  = "private" // tableau privé : Ajouter au classeur partagé…
	FormFrozen   = "frozen"  // classeur figé
	FormQuery    = "query"   // tableau chargé par une requête
	FormLinked   = "linked"  // tableau lié du Data Lake
	FormSource   = "source"  // source poussée du Data Lake
	FormOffline  = "offline" // hors ligne : Réessayer
	FormRights   = "rights"  // ni propriétaire ni concepteur
	FormNoServer = "noforms" // serveur sans les formulaires
)

// formsDiscovery lit la découverte d'un serveur (essais : remplacée ; nil
// d'erreur et Forms faux : serveur sans les formulaires).
var formsDiscovery = func(ctx context.Context, server string) (protocol.Discovery, error) {
	m, err := sessions()
	if err != nil {
		return protocol.Discovery{}, err
	}
	server, err = auth.NormalizeURL(server)
	if err != nil {
		return protocol.Discovery{}, err
	}
	_, disc, err := m.Session(ctx, server)
	if err != nil || disc == nil {
		return protocol.Discovery{}, err
	}
	return *disc, nil
}

// formImages donne l'envoi des images d'un serveur (essais : remplacée).
var formImages = func(ctx context.Context, server string, srv workbook.Server) (workbook.FormImages, error) {
	if fi, ok := srv.(workbook.FormImages); ok {
		return fi, nil
	}
	m, err := sessions()
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
	issuer := disc.Issuer
	if issuer == "" {
		issuer = server
	}
	return workbook.FormImageClient{Issuer: issuer, Token: func(ctx context.Context) (string, error) { return m.Token(ctx, server) },
		Invalidate: func() { m.Invalidate(server) }}, nil
}

// FormList est la liste de validation d'une colonne : valeurs fixes
// (Fixed), ou tirées d'une plage (Source) ; une source qui est la colonne
// d'un tableau publié du classeur donne TableID et ColumnID (le serveur la
// relit), sinon la liste est copiée.
type FormList struct {
	Values   []string `json:"values"`
	Fixed    bool     `json:"fixed"`
	Source   string   `json:"source"`
	TableID  string   `json:"tableId"`
	ColumnID string   `json:"columnId"`
	Error    string   `json:"error"`
}

// FormColumn est une colonne publiée du tableau.
type FormColumn struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ColumnID string `json:"columnId"`
	// Type : type du serveur (text, integer, decimal, date, datetime, boolean).
	Type string `json:"type"`
	// Key : clé du tableau ; Auto : clé d'entiers numérotée (posée par le
	// serveur, absente du formulaire).
	Key  bool `json:"key"`
	Auto bool `json:"auto"`
	// AvgLength : longueur moyenne des valeurs (texte long proposé au-delà
	// de 80) ; Format : format de nombre de la colonne (unité).
	AvgLength float64   `json:"avgLength"`
	Format    string    `json:"format"`
	List      *FormList `json:"list"`
}

// FormChoice est la colonne de catégorie de l'onglet à choix (§3.3).
type FormChoice struct {
	Column string `json:"column"`
	Value  string `json:"value"`
}

// FormInfo dit si le tableau peut avoir un formulaire et donne de quoi le
// concevoir.
type FormInfo struct {
	// Reason : FormLocal… ("" : possible) ; Message : le texte du §2.2.
	Reason     string `json:"reason"`
	Message    string `json:"message"`
	Table      string `json:"table"`
	Server     string `json:"server"`
	ServerName string `json:"serverName"`
	WorkbookID string `json:"workbookId"`
	TableID    string `json:"tableId"`
	// Public : le serveur permet le lien public (forms_public).
	Public  bool         `json:"public"`
	Columns []FormColumn `json:"columns"`
	Choice  *FormChoice  `json:"choice"`
	// Restricted : colonnes interdites d'ajout par une restriction de tous
	// les rôles (le formulaire en est bloqué aussi, §8) ; leurs libellés.
	Restricted []string `json:"restricted"`
	// Forms : formulaires du tableau (gérés, ou ouverts au membre).
	Forms []workbook.Form `json:"forms"`
	Teams []DataTeam      `json:"teams"`
}

var formMessages = map[string]string{
	FormLocal:    "Ce tableau n'est pas partagé. Un formulaire est servi par un Serveur Lattis : publiez d'abord le classeur en mode collaboratif.",
	FormPrivate:  "Ce tableau n'est que sur votre poste. Ajoutez-le au classeur partagé pour en faire un formulaire.",
	FormFrozen:   "Un classeur figé ne reçoit pas de lignes. Les formulaires demandent un classeur collaboratif.",
	FormQuery:    "Ce tableau est rempli par une requête : ses lignes seraient remplacées.",
	FormLinked:   "Ce tableau est une donnée liée du Data Lake : ses lignes seraient remplacées.",
	FormSource:   "Ce tableau est rempli par l'envoi de votre classeur (source du Data Lake) : ses lignes seraient remplacées.",
	FormOffline:  "Créer ou modifier un formulaire demande le serveur.",
	FormRights:   "Réservé aux propriétaires et concepteurs du classeur.",
	FormNoServer: workbook.ErrNoForms.Error(),
}

func (i *FormInfo) refuse(reason string) {
	i.Reason, i.Message = reason, formMessages[reason]
}

// FormInfo lit de quoi faire un formulaire du tableau table du classeur
// ouvert.
func (s *DocumentService) FormInfo(ctx context.Context, table string) (FormInfo, error) {
	out := FormInfo{Table: table, Columns: []FormColumn{}, Restricted: []string{}, Forms: []workbook.Form{}, Teams: []DataTeam{}}
	d, err := s.current()
	if err != nil {
		return out, err
	}
	tbl, err := tableIDByName(ctx, d, table)
	if err != nil {
		return out, err
	}
	db := d.DB().Read()
	var src sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT source FROM _lattis_queries WHERE target = ?`, tbl).Scan(&src); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	if src.Valid {
		if strings.Contains(src.String, `"dataset"`) {
			out.refuse(FormLinked)
		} else {
			out.refuse(FormQuery)
		}
		return out, nil
	}
	if link, err := sourceOf(ctx, d, tbl); err != nil || link != nil {
		out.refuse(FormSource)
		return out, err
	}
	info := d.Share.Info()
	if info == nil {
		out.refuse(FormLocal)
		return out, nil
	}
	out.Server, out.ServerName, out.WorkbookID = info.Server, info.ServerName, info.WorkbookID
	if info.Mode == workbook.Frozen {
		out.refuse(FormFrozen)
		return out, nil
	}
	var cols string
	err = db.QueryRowContext(ctx, `SELECT coalesce(remote_id, ''), coalesce(columns, '[]') FROM _lattis_remote_tables WHERE tbl = ? AND published = 1`, tbl).Scan(&out.TableID, &cols)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	design := info.CanDesign()
	if !d.Share.State().Online {
		out.refuse(FormOffline)
		return out, nil
	}
	srv, _, name, err := remoteServer(ctx, info.Server)
	if err != nil {
		return out, err
	}
	if out.ServerName == "" {
		out.ServerName = name
	}
	// Disponibilité : lattis.forms_settings (la découverte ne la dit pas).
	set, err := srv.FormsSettings(ctx)
	if err != nil || !set.Forms {
		if err = workbook.FormError(err); err != nil && !errors.Is(err, workbook.ErrNoForms) {
			return out, err
		}
		out.refuse(FormNoServer)
		return out, nil
	}
	out.Public = set.FormsPublic
	if disc, err := formsDiscovery(ctx, info.Server); err == nil {
		if disc.Name != "" {
			out.ServerName = disc.Name
		}
	}
	if out.TableID == "" {
		if design {
			out.refuse(FormPrivate)
		} else {
			out.refuse(FormRights)
		}
		return out, nil
	}
	// Formulaires du tableau : un membre peut copier le lien de ceux qui
	// lui sont ouverts.
	forms, err := srv.ListForms(ctx, info.WorkbookID)
	if err != nil {
		if errors.Is(workbook.FormError(err), workbook.ErrNoForms) {
			out.refuse(FormNoServer)
			return out, nil
		}
		return out, workbook.FormError(err)
	}
	for _, f := range forms {
		if f.TableID == out.TableID {
			f.URL = formURL(info.Server, f.URL)
			out.Forms = append(out.Forms, f)
		}
	}
	if !design {
		out.refuse(FormRights)
		return out, nil
	}
	if out.Columns, err = formColumns(ctx, d, tbl, cols); err != nil {
		return out, err
	}
	if ch, err := tableChoice(ctx, d, tbl, out.Columns); err == nil {
		out.Choice = ch
	}
	if info.CanManage() {
		if acl, err := srv.ACL(ctx, info.WorkbookID); err == nil {
			for _, r := range acl.Restrictions {
				if r.TableID == out.TableID && len(r.AppliesTo) == 0 && (r.Command == "insert" || r.Command == "all") {
					out.Restricted = append(out.Restricted, cmpLabel(r))
				}
			}
		}
	}
	if teams, err := srv.Teams(ctx); err == nil {
		for _, t := range teams {
			out.Teams = append(out.Teams, DataTeam{ID: t.ID, Name: t.Name})
		}
	}
	return out, nil
}

func cmpLabel(r workbook.RestrictionRule) string {
	if r.Label != "" {
		return r.Label
	}
	return r.Predicate
}

// formColumns lit les colonnes publiées du tableau tbl (cols : colonnes du
// serveur notées à la publication), dans l'ordre du tableau local.
func formColumns(ctx context.Context, d *doc.Document, tbl int64, cols string) ([]FormColumn, error) {
	var rc []workbook.RemoteColumn
	_ = json.Unmarshal([]byte(cols), &rc)
	byLocal := map[int64]workbook.RemoteColumn{}
	for _, c := range rc {
		byLocal[c.LocalID] = c
	}
	db := d.DB().Read()
	var key, sheet int64
	if err := db.QueryRowContext(ctx, `SELECT coalesce(key_col, 0), sheet FROM _lattis_tables WHERE id = ?`, tbl).Scan(&key, &sheet); err != nil {
		return nil, err
	}
	pub, err := publishedColumns(ctx, d)
	if err != nil {
		return nil, err
	}
	locals, err := localColumns(ctx, d, tbl)
	if err != nil {
		return nil, err
	}
	out := []FormColumn{}
	for _, lc := range locals {
		r, ok := byLocal[lc.ID]
		if !ok {
			continue // colonne calculée ou pas encore envoyée : absente du serveur
		}
		c := FormColumn{ID: lc.ID, Name: lc.Name, ColumnID: r.ID, Type: r.Type, Key: lc.ID == key}
		if c.Type == "" {
			c.Type = lc.Type
		}
		c.Auto = c.Key && c.Type == "integer"
		col := "c_" + strconv.FormatInt(lc.ID, 10)
		if c.Type == "text" {
			var avg sql.NullFloat64
			_ = db.QueryRowContext(ctx, `SELECT avg(length(`+col+`)) FROM t_`+strconv.FormatInt(tbl, 10)+` WHERE `+col+` IS NOT NULL AND `+col+` <> ''`).Scan(&avg)
			c.AvgLength = avg.Float64
		}
		var style string
		if err := db.QueryRowContext(ctx, `SELECT style FROM _lattis_styles WHERE tbl = ? AND rid = 0 AND col = ?`, tbl, lc.ID).Scan(&style); err == nil {
			var st map[string]any
			if json.Unmarshal([]byte(style), &st) == nil {
				c.Format, _ = st["nf"].(string)
				c.List = formList(ctx, d, sheet, st["dv"], pub)
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// publishedColumns : tableaux publiés du classeur, par nom local (sans
// casse) → identifiant du serveur et colonnes (nom sans casse → column_id).
func publishedColumns(ctx context.Context, d *doc.Document) (map[string]struct {
	ID   string
	Cols map[string]string
}, error) {
	out := map[string]struct {
		ID   string
		Cols map[string]string
	}{}
	rs, err := d.DB().Read().QueryContext(ctx, `SELECT t.name, coalesce(r.remote_id, ''), coalesce(r.columns, '[]') FROM _lattis_remote_tables r JOIN _lattis_tables t ON t.id = r.tbl WHERE r.published = 1`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var name, id, cols string
		if err := rs.Scan(&name, &id, &cols); err != nil {
			return nil, err
		}
		var rc []workbook.RemoteColumn
		_ = json.Unmarshal([]byte(cols), &rc)
		m := map[string]string{}
		for _, c := range rc {
			m[strings.ToLower(c.Name)] = c.ID
		}
		out[strings.ToLower(name)] = struct {
			ID   string
			Cols map[string]string
		}{id, m}
	}
	return out, rs.Err()
}

// formList lit la validation dv d'une colonne : liste fixe, ou plage (lue
// maintenant ; colonne d'un tableau publié : citée par ses identifiants).
func formList(ctx context.Context, d *doc.Document, sheet int64, dv any, pub map[string]struct {
	ID   string
	Cols map[string]string
}) *FormList {
	switch v := dv.(type) {
	case []any:
		l := &FormList{Fixed: true, Values: []string{}}
		for _, x := range v {
			if s := strings.TrimSpace(fmt.Sprint(x)); s != "" {
				l.Values = append(l.Values, s)
			}
		}
		return l
	case map[string]any:
		src, ok := canvas.DVSource(v)
		if !ok {
			return nil
		}
		l := &FormList{Source: src, Values: []string{}}
		vl, err := d.Model.ValidationList(ctx, sheet, src)
		if err != nil {
			l.Error = err.Error()
		} else {
			l.Values, l.Error = vl.Values, vl.Error
		}
		// Colonne entière d'un tableau publié : relue par le serveur.
		if n, err := formula.Parse(strings.TrimPrefix(strings.TrimSpace(src), "="), formula.FR); err == nil {
			if ref, ok := n.(formula.Ref); ok && ref.Table != "" && !ref.Row {
				if t, ok := pub[strings.ToLower(ref.Table)]; ok && t.ID != "" {
					if c := t.Cols[strings.ToLower(strings.TrimSpace(ref.Col))]; c != "" {
						l.TableID, l.ColumnID = t.ID, c
					}
				}
			}
		}
		return l
	}
	return nil
}

// tableChoice : la colonne de catégorie de l'onglet à choix, si le tableau
// l'a parmi ses colonnes publiées.
func tableChoice(ctx context.Context, d *doc.Document, tbl int64, cols []FormColumn) (*FormChoice, error) {
	var sheet int64
	if err := d.DB().Read().QueryRowContext(ctx, `SELECT sheet FROM _lattis_tables WHERE id = ?`, tbl).Scan(&sheet); err != nil {
		return nil, err
	}
	ch, err := d.Model.SheetChoice(ctx, sheet)
	if err != nil || ch == nil {
		return nil, err
	}
	for _, c := range cols {
		if strings.EqualFold(c.Name, ch.Name) {
			v := ch.Value
			if ch.All {
				v = ""
			}
			return &FormChoice{Column: c.Name, Value: v}, nil
		}
	}
	return nil, nil
}

// FormSave est l'enregistrement d'un formulaire : création (ID vide) ou
// modification de la version Version ; Add et Remove modifient l'audience.
type FormSave struct {
	Server     string              `json:"server"`
	WorkbookID string              `json:"workbookId"`
	TableID    string              `json:"tableId"`
	ID         string              `json:"id"`
	Version    int                 `json:"version"`
	Spec       workbook.FormSpec   `json:"spec"`
	Add        []DataAudienceEntry `json:"add"`
	Remove     []string            `json:"remove"`
	// Names : noms des colonnes par column_id, pour dire les refus du
	// serveur avec les noms de l'utilisateur.
	Names map[string]string `json:"names"`
}

// FormSaved est le formulaire enregistré.
type FormSaved struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	URL     string `json:"url"`
	Version int    `json:"version"`
	// Refused : entrées d'audience refusées (adresse sans compte) ; le reste
	// est enregistré.
	Refused []string `json:"refused"`
}

// formError rend un refus du serveur avec les noms des colonnes.
func formError(err error, names map[string]string) error {
	err = workbook.FormError(err)
	var inv *workbook.FormInvalid
	if errors.As(err, &inv) && len(inv.Fields) > 0 {
		var parts []string
		for id, why := range inv.Fields {
			if n := names[id]; n != "" {
				parts = append(parts, n+" : "+why)
			} else {
				parts = append(parts, why)
			}
		}
		slices.Sort(parts)
		return errors.New("Formulaire refusé par le serveur : " + strings.Join(parts, " ; "))
	}
	return err
}

// FormSave crée le formulaire (brouillon) ou l'enregistre, puis modifie
// son audience.
func (s *DocumentService) FormSave(ctx context.Context, req FormSave) (FormSaved, error) {
	out := FormSaved{ID: req.ID, Version: req.Version, Refused: []string{}}
	srv, _, _, err := remoteServer(ctx, req.Server)
	if err != nil {
		return out, err
	}
	if req.Spec.Audience == nil {
		req.Spec.Audience = []string{}
	}
	if req.ID == "" {
		c, err := srv.CreateForm(ctx, req.WorkbookID, req.TableID, req.Spec)
		if err != nil {
			return out, formError(err, req.Names)
		}
		out.ID, out.Code, out.URL, out.Version = c.ID, c.Code, formURL(req.Server, c.URL), 1
	} else {
		def, access := req.Spec.Definition, req.Spec.Access
		v, err := srv.UpdateForm(ctx, req.ID, workbook.FormChange{Definition: &def, Access: &access, MaxPerRespondent: req.Spec.MaxPerRespondent}, req.Version)
		if err != nil {
			return out, formError(err, req.Names)
		}
		out.Version = v
	}
	for _, p := range req.Remove {
		if err := srv.FormAudienceRemove(ctx, out.ID, p); err != nil {
			return out, formError(err, req.Names)
		}
	}
	for _, a := range req.Add {
		var err error
		switch {
		case a.Team != "":
			err = srv.FormAudienceAdd(ctx, out.ID, a.Team)
		case strings.TrimSpace(a.Email) != "":
			err = srv.FormAudienceAddByEmail(ctx, out.ID, strings.TrimSpace(a.Email))
		}
		if err != nil {
			out.Refused = append(out.Refused, strings.TrimSpace(a.Email+" "+a.Team)+" : "+workbook.ServerMessage(err))
		}
	}
	if out.URL == "" {
		if forms, err := srv.ListForms(ctx, req.WorkbookID); err == nil {
			for _, f := range forms {
				if f.ID == out.ID {
					out.Code, out.URL = f.Code, formURL(req.Server, f.URL)
				}
			}
		}
	}
	return out, nil
}

// FormSetState ouvre, met en pause ou ferme un formulaire.
func (s *DocumentService) FormSetState(ctx context.Context, server, id, state string, names map[string]string) error {
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return err
	}
	return formError(srv.SetFormState(ctx, id, state), names)
}

// FormDrop supprime un formulaire (les lignes reçues restent).
func (s *DocumentService) FormDrop(ctx context.Context, server, id string) error {
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return err
	}
	return workbook.FormError(srv.DropForm(ctx, id))
}

// formURL rend l'adresse complète d'un lien du serveur, qui peut ne rendre
// qu'un chemin (/f/<code>, aperçu /f/<code>?p=…).
func formURL(server, u string) string {
	if strings.HasPrefix(u, "/") {
		return strings.TrimRight(server, "/") + u
	}
	return u
}

// FormPreviewURL rend le lien d'aperçu (10 minutes, réponses non
// enregistrées).
func (s *DocumentService) FormPreviewURL(ctx context.Context, server, id string) (string, error) {
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return "", err
	}
	u, err := srv.FormPreviewURL(ctx, id)
	return formURL(server, u), workbook.FormError(err)
}

// FormImage envoie le logo ou la bannière (kind) d'un formulaire, lus
// dans le frontend (data : contenu en base64, name : nom du fichier pour
// le type) ; rend l'identifiant de l'image.
func (s *DocumentService) FormImage(ctx context.Context, server, id, kind, name, data string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", errors.New("image illisible")
	}
	var ctype string
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		ctype = "image/png"
	case ".jpg", ".jpeg":
		ctype = "image/jpeg"
	case ".svg":
		ctype = "image/svg+xml"
	default:
		return "", msg.New("forms.imageType", nil)
	}
	limit, what := workbook.MaxFormBanner, "1 Mo pour une bannière"
	if kind == "logo" {
		limit, what = workbook.MaxFormLogo, "200 Ko pour un logo"
	}
	if len(raw) > limit {
		return "", msg.New("forms.imageTooLarge", msg.Args{"max": what})
	}
	srv, _, _, err := remoteServer(ctx, server)
	if err != nil {
		return "", err
	}
	fi, err := formImages(ctx, server, srv)
	if err != nil {
		return "", err
	}
	return fi.UploadFormImage(ctx, id, kind, ctype, raw)
}

// FormAddColumn ajoute au tableau une colonne pour une valeur posée par le
// serveur (« Ajouter une colonne Reçu le ») par les opérations de
// structure du classeur partagé, l'envoie au serveur et rend la
// description du tableau à jour.
func (s *DocumentService) FormAddColumn(ctx context.Context, table, name, typ string) (FormInfo, error) {
	d, err := s.current()
	if err != nil {
		return FormInfo{}, err
	}
	tbl, err := tableIDByName(ctx, d, table)
	if err != nil {
		return FormInfo{}, err
	}
	l, err := d.Model.TableColumns(ctx, tbl)
	if err != nil {
		return FormInfo{}, err
	}
	if _, err := s.act(ctx, func(d *doc.Document) error { return d.InsertColumn(ctx, table, len(l), name) }); err != nil {
		return FormInfo{}, err
	}
	if typ != "" && typ != "text" {
		if _, err := s.act(ctx, func(d *doc.Document) error { return d.SetType(ctx, table, len(l), workbook.LocalType(typ)) }); err != nil {
			return FormInfo{}, err
		}
	}
	if eng := s.collab(); eng != nil {
		if _, err := eng.Send(ctx); err != nil {
			return FormInfo{}, err
		}
	}
	return s.FormInfo(ctx, table)
}

// FormSaveQR enregistre le QR code d'un formulaire (généré sur le poste)
// dans un fichier choisi : ext « svg » (data : le texte SVG) ou « png »
// (data : base64). Rend le fichier écrit, vide si l'utilisateur a annulé.
func (s *DocumentService) FormSaveQR(ctx context.Context, name, ext, data string) (string, error) {
	var raw []byte
	switch ext {
	case "svg":
		raw = []byte(data)
	case "png":
		b, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return "", errors.New("image illisible")
		}
		raw = b
	default:
		return "", msg.New("forms.unknownFormat", msg.Args{"ext": ext})
	}
	dir := ""
	if d, err := s.current(); err == nil {
		dir = docFolder(d)
	}
	return s.sys().SaveFile(FileDialog{
		Title:     "Enregistrer le QR code",
		Filename:  name + "." + ext,
		Directory: dir,
		Filters:   []FileFilter{{strings.ToUpper(ext), "*." + ext}},
		Ext:       "." + ext,
	}, func(p string) error { return os.WriteFile(p, raw, 0o644) })
}
