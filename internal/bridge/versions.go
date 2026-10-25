package bridge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/trace"
	"github.com/lalamefine/lattis/internal/versions"
)

// Fichier › Historique des versions d'un classeur local (internal/versions) :
// une copie gardée à chaque enregistrement (automatique), ou sous un nom ;
// ouvrir une version en lecture seule, la restaurer (la version d'avant est
// gardée : restaurer se défait en la restaurant à son tour).

// VersionEntry est une version pour l'interface.
type VersionEntry struct {
	ID     string `json:"id"`
	At     string `json:"at"` // RFC 3339
	Author string `json:"author"`
	Size   int64  `json:"size"`
	Name   string `json:"name"`
}

// VersionList est l'historique du classeur ouvert.
type VersionList struct {
	// Reason : pourquoi il n'y a pas d'historique (classeur partagé, jamais
	// enregistré) ; vide sinon.
	Reason  string         `json:"reason,omitempty"`
	Entries []VersionEntry `json:"entries"`
	// Viewing : version ouverte en lecture seule (son identifiant), vide
	// sinon.
	Viewing string `json:"viewing,omitempty"`
}

// versionsPath : le classeur dont d montre les versions (son fichier, ou
// celui d'une version ouverte en lecture seule), et la version ouverte.
func versionsPath(d *doc.Document) (path, viewing string, err error) {
	p := d.Path()
	if orig, ok := versions.OriginalOf(p); ok {
		return orig, filepathBase(p), nil
	}
	switch {
	case liveShared(d):
		return "", "", msg.New("versions.shared", nil)
	case p == "":
		return "", "", msg.New("versions.unsaved", nil)
	}
	return p, "", nil
}

// liveShared : classeur du serveur (cache) ou collaboratif : ses versions
// seraient celles d'une copie, pas du classeur partagé.
func liveShared(d *doc.Document) bool {
	if remoteDoc(d) != nil || inShareCache(d.Path()) {
		return true
	}
	info := d.Share.Info()
	return info != nil && info.Mode == workbook.Collaborative
}

func filepathBase(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// keepVersion garde une version automatique du classeur d juste enregistré
// dans son fichier (saveDoc) ; un échec n'empêche pas l'enregistrement.
func keepVersion(ctx context.Context, d *doc.Document) {
	p := d.Path()
	if p == "" || publicDemo.Load() || liveShared(d) {
		return
	}
	if _, ok := versions.OriginalOf(p); ok {
		return
	}
	if _, _, err := versions.Snapshot(p, localAuthor().Name, ""); err != nil {
		trace.Logf(1, trace.Doc, "version de %s non gardée : %v", p, err)
	}
}

// Versions donne l'historique des versions du classeur ouvert.
func (s *DocumentService) Versions(ctx context.Context) (VersionList, error) {
	out := VersionList{Entries: []VersionEntry{}}
	d, err := s.current()
	if err != nil {
		return out, err
	}
	path, viewing, err := versionsPath(d)
	if err != nil {
		out.Reason = msg.Text(s.sessionLang(), err)
		return out, nil
	}
	out.Viewing = viewing
	list, err := versions.List(path)
	if err != nil {
		return out, err
	}
	for _, e := range list {
		out.Entries = append(out.Entries, VersionEntry{ID: e.ID, At: e.At.UTC().Format(time.RFC3339), Author: e.Author, Size: e.Size, Name: e.Name})
	}
	return out, nil
}

// SaveVersion enregistre le classeur, puis en garde une version sous le nom
// name (« Enregistrer une version… »).
func (s *DocumentService) SaveVersion(ctx context.Context, name string) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return DocState{}, msg.New("versions.nameEmpty", nil)
	}
	path, viewing, err := versionsPath(d)
	if err != nil {
		return DocState{}, err
	}
	if viewing != "" || d.ReadOnly() {
		return DocState{}, msg.New("versions.readOnly", nil)
	}
	if dirty, _ := d.Dirty(ctx); dirty {
		if err := saveDoc(ctx, d, ""); err != nil {
			return s.saveFailed(ctx, d, err)
		}
	}
	if _, _, err := versions.Snapshot(path, localAuthor().Name, name); err != nil {
		return DocState{}, err
	}
	st, err := s.saved(ctx, d)
	st.Notice = s.say("versions.saved", msg.Args{"name": name})
	return st, err
}

// OpenVersion ouvre une version en lecture seule, à la place du classeur
// (enregistré d'abord s'il a changé).
func (s *DocumentService) OpenVersion(ctx context.Context, id string) (OpenResult, error) {
	d, err := s.current()
	if err != nil {
		return OpenResult{}, err
	}
	path, viewing, err := versionsPath(d)
	if err != nil {
		return OpenResult{}, err
	}
	if viewing == "" && !d.ReadOnly() {
		if dirty, _ := d.Dirty(ctx); dirty {
			if err := saveDoc(ctx, d, ""); err != nil {
				return OpenResult{}, err
			}
		}
	}
	file, err := versions.File(path, id)
	if err != nil {
		return OpenResult{}, msg.New("versions.notFound", nil)
	}
	v, err := doc.OpenAs(ctx, file, doc.OpenReadOnly)
	if err != nil {
		return OpenResult{}, err
	}
	s.replace(v)
	st, err := s.State(ctx)
	st.Notice = s.say("versions.opened", nil)
	return OpenResult{State: st}, err
}

// RestoreVersion remplace le classeur par une version : le classeur est
// enregistré s'il a changé, sa version actuelle gardée (« Avant
// restauration »), puis il est rouvert.
func (s *DocumentService) RestoreVersion(ctx context.Context, id string) (OpenResult, error) {
	d, err := s.current()
	if err != nil {
		return OpenResult{}, err
	}
	path, viewing, err := versionsPath(d)
	if err != nil {
		return OpenResult{}, err
	}
	if _, err := versions.File(path, id); err != nil {
		return OpenResult{}, msg.New("versions.notFound", nil)
	}
	switch {
	case viewing == "" && d.ReadOnly(), s.CoEditState().Active:
		return OpenResult{}, msg.New("versions.restoreBusy", nil)
	case viewing != "":
		if q, _ := s.openQuestion(path); q != nil {
			return OpenResult{}, msg.New("versions.restoreBusy", nil)
		}
	default:
		if dirty, _ := d.Dirty(ctx); dirty {
			if err := saveDoc(ctx, d, ""); err != nil {
				return OpenResult{}, err
			}
		}
	}
	// Le classeur fermé (verrou rendu), sa version actuelle gardée, puis
	// remplacé et rouvert.
	if err := s.replace(nil); err != nil {
		return OpenResult{}, err
	}
	if err := versions.Restore(path, id, localAuthor().Name, s.say("versions.beforeRestore", nil)); err != nil {
		// Rien de remplacé : le classeur se rouvre tel quel.
		r, oerr := s.open(ctx, path, doc.OpenNormal)
		if oerr != nil {
			return r, errors.Join(err, oerr)
		}
		return r, err
	}
	r, err := s.open(ctx, path, doc.OpenNormal)
	if err == nil && r.State.Open {
		r.State.Notice = s.say("versions.restored", nil)
	}
	return r, err
}

// DeleteVersion supprime une version gardée.
func (s *DocumentService) DeleteVersion(ctx context.Context, id string) error {
	d, err := s.current()
	if err != nil {
		return err
	}
	path, viewing, err := versionsPath(d)
	if err != nil {
		return err
	}
	if id == viewing {
		return msg.New("versions.deleteOpen", nil)
	}
	if err := versions.Delete(path, id); errors.Is(err, versions.ErrNotFound) {
		return msg.New("versions.notFound", nil)
	} else if err != nil {
		return err
	}
	return nil
}
