package bridge

import (
	"context"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/doc"
)

// Commentaires en fil (Révision › Nouveau commentaire, volet Commentaires) :
// voir canvas/comments.go. L'auteur est le compte du Serveur Lattis dans un
// classeur partagé, sinon le nom des réglages (ui.userName) ou celui de
// l'utilisateur du système.

// CommentIdentity est l'auteur des commentaires de ce poste, et les membres
// qu'on peut mentionner (@nom) dans un classeur partagé.
type CommentIdentity struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	// Members : membres du classeur partagé (propriétaires et concepteurs
	// seulement peuvent les lister) ; vide pour un classeur local.
	Members []CommentMember `json:"members"`
}

// CommentMember est un membre qu'on peut mentionner.
type CommentMember struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// localAuthor : le nom des réglages, sinon celui de l'utilisateur du système.
func localAuthor() canvas.Who {
	me := archive.Me()
	name := strings.TrimSpace(config.Current().String("ui.userName", ""))
	if name == "" {
		name = me.User
	}
	if name == "" {
		name = me.Login
	}
	return canvas.Who{Name: name, ID: "local:" + strings.ToLower(me.Login)}
}

// commentWho : l'auteur des commentaires du classeur d.
func commentWho(ctx context.Context, d *doc.Document) canvas.Who {
	info := d.Share.Info()
	if info == nil {
		return localAuthor()
	}
	who := canvas.Who{Name: info.User, ID: info.User}
	if shareServer != nil {
		return who // essais : faux serveur, sans session
	}
	if m, err := sessions(); err == nil {
		c, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if u, _, err := m.Session(c, info.Server); err == nil && u.Name() != "" {
			who.Name = u.Name()
		}
	}
	return who
}

// CommentAuthor donne l'auteur des commentaires de ce poste et les membres
// à mentionner.
func (s *DocumentService) CommentAuthor(ctx context.Context) (CommentIdentity, error) {
	d, err := s.current()
	if err != nil {
		return CommentIdentity{}, err
	}
	who := commentWho(ctx, d)
	out := CommentIdentity{Name: who.Name, ID: who.ID, Members: []CommentMember{}}
	info := d.Share.Info()
	if info == nil || !info.CanDesign() {
		return out, nil
	}
	srv, _, _, err := remoteServer(ctx, info.Server)
	if err != nil {
		return out, nil // hors ligne : pas de suggestions
	}
	members, err := srv.Members(ctx, info.WorkbookID)
	if err != nil {
		return out, nil
	}
	for _, m := range members {
		name := strings.TrimSpace(m.DisplayName)
		if name == "" {
			name, _, _ = strings.Cut(m.Email, "@")
		}
		if name != "" {
			out.Members = append(out.Members, CommentMember{Name: name, Email: m.Email})
		}
	}
	return out, nil
}

// Threads donne les fils de commentaires d'une feuille (0 : de tout le
// classeur), à leur place actuelle.
func (s *DocumentService) Threads(ctx context.Context, sheet int64) ([]canvas.Thread, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.Threads(ctx, sheet)
}

// AddComment ajoute un commentaire à la cellule (ouvre son fil, ou y répond).
func (s *DocumentService) AddComment(ctx context.Context, sheet int64, row, col int, text string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.AddComment(ctx, sheet, row, col, text, commentWho(ctx, d))
		return err
	})
}

// EditComment modifie un de ses commentaires.
func (s *DocumentService) EditComment(ctx context.Context, id, text string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.EditComment(ctx, id, text, commentWho(ctx, d)) })
}

// DeleteComment supprime un de ses commentaires (le premier : tout le fil).
func (s *DocumentService) DeleteComment(ctx context.Context, id string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteComment(ctx, id, commentWho(ctx, d)) })
}

// DeleteThread supprime le fil de commentaires de la cellule.
func (s *DocumentService) DeleteThread(ctx context.Context, sheet int64, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteThread(ctx, sheet, row, col) })
}

// ResolveThread résout (ou rouvre) un fil de commentaires.
func (s *DocumentService) ResolveThread(ctx context.Context, thread string, resolved bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ResolveThread(ctx, thread, resolved) })
}
