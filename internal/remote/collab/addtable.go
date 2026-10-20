package collab

import (
	"context"
	"database/sql"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Tableau ajouté au classeur collaboratif après sa publication (« Ajouter
// au classeur partagé ») : créé sur le serveur avec ses lignes par son
// auteur (concepteur ou propriétaire, en ligne), reçu par les autres postes
// sur « *meta » (et à chaque connexion), puis synchronisé comme les
// autres. Voir docs/collaboration/synchronisation.md §2.10.

// ErrAddOffline : l'ajout passe par le serveur.
var ErrAddOffline = msg.New("collab.addTableOffline", nil)

// AddedNotice : message de la barre d'état après l'ajout.
func AddedNotice(name string) string {
	return "Tableau « " + name + " » ajouté au classeur partagé : les autres membres le reçoivent"
}

// ArrivedNotice : message de la barre d'état quand un tableau ajouté par un
// collègue est reçu.
func ArrivedNotice(name string) string {
	return "Tableau « " + name + " » ajouté au classeur partagé par un concepteur"
}

// CanAdd dit pourquoi le tableau local tbl ne peut pas être ajouté au
// classeur partagé ("" : il peut l'être).
func (e *Engine) CanAdd(tbl int64) string {
	info := e.link.Info()
	switch {
	case info == nil || info.Mode != workbook.Collaborative:
		return ErrNotCollaborative.Error()
	case e.link.TableShare(tbl) == "published":
		return "Ce tableau est déjà dans le classeur partagé"
	case !info.CanDesign():
		return workbook.ErrAddDesign.Error()
	case e.server() == nil || !e.link.Online():
		return ErrAddOffline.Error()
	}
	return ""
}

// AddTable ajoute le tableau local tbl au classeur collaboratif ouvert :
// créé sur le serveur avec ses lignes, puis synchronisé comme les autres
// tableaux partagés. Rend son nom. Un refus du serveur (texte en français)
// est une RefusedError ; rien ne change alors en local. L'ajout ne
// s'annule pas (Ctrl+Z) : « Supprimer le tableau » le retire du serveur.
func (e *Engine) AddTable(ctx context.Context, tbl int64) (string, error) {
	info, srv := e.active()
	if info == nil {
		return "", ErrNotCollaborative
	}
	if !info.CanDesign() {
		return "", workbook.ErrAddDesign
	}
	if !e.link.Online() {
		return "", ErrAddOffline
	}
	e.run.Lock()
	defer e.run.Unlock()
	cctx, cancel := context.WithTimeout(ctx, StructureTimeout)
	defer cancel()
	added, err := workbook.AddTable(cctx, srv, e.db, info, tbl, func(tx *sql.Tx, xid string) error {
		return addSkip(ctx, tx, xid)
	})
	if err != nil {
		if _, ok := workbook.Rejected(err); ok {
			return "", &RefusedError{Kind: command.KindDo, Msg: workbook.ServerMessage(err)}
		}
		return "", err
	}
	if err := e.link.Reload(ctx); err != nil {
		return added.Name, err
	}
	// Niveau sur le nouveau tableau (éditeur pour son auteur).
	if _, err := workbook.RefreshCapabilities(ctx, srv, e.link); err != nil {
		return added.Name, nil // relu à la prochaine connexion
	}
	return added.Name, nil
}

// receiveArrivals crée en local les tableaux du serveur qu'aucun tableau
// local ne désigne (ajoutés par un collègue), avec leurs lignes.
func (e *Engine) receiveArrivals(ctx context.Context) (Report, error) {
	var rep Report
	info, srv := e.active()
	if info == nil {
		return rep, nil
	}
	known := map[string]bool{}
	rs, err := e.db.Read().QueryContext(ctx, `SELECT remote_id FROM _lattis_remote_tables WHERE published = 1 AND remote_id IS NOT NULL`)
	if err != nil {
		return rep, err
	}
	for rs.Next() {
		var id string
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return rep, err
		}
		known[id] = true
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return rep, err
	}
	var a *workbook.Arrivals
	err = srv.Delta(ctx, func(r workbook.Reader) error {
		var err error
		a, err = workbook.ReadArrivals(ctx, r, info.Schema, known)
		return err
	})
	if err != nil || a == nil {
		return rep, err
	}
	var got []workbook.Installed
	err = e.db.Write(ctx, func(tx *sql.Tx) error {
		var err error
		got, err = workbook.InstallArrivals(ctx, tx, a)
		return err
	})
	if err != nil || len(got) == 0 {
		return rep, err
	}
	if err := e.link.Reload(ctx); err != nil {
		return rep, err
	}
	if _, err := workbook.RefreshCapabilities(ctx, srv, e.link); err != nil {
		return rep, err
	}
	e.settleChoices(ctx) // feuille reçue avec un onglet à choix
	for _, t := range got {
		rep.notice(ArrivedNotice(t.Name))
		rep.Applied.Tables = append(rep.Applied.Tables, t.Table)
	}
	return rep, nil
}
