package collab

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Tableau ajouté au classeur collaboratif après sa publication (« Ajouter
// au classeur partagé ») : créé sur le serveur avec ses lignes par son
// auteur (concepteur ou propriétaire, en ligne), reçu par les autres postes
// sur « *meta » (et à chaque connexion), puis synchronisé comme les
// autres. Voir docs/collaboration/synchronisation.md §2.10.

// ErrAddOffline : l'ajout passe par le serveur.
var ErrAddOffline = msg.New("collab.addTableOffline", nil)

// AddedNotice : message de la barre d'état après l'ajout, dans la langue l.
func AddedNotice(l msg.Lang, name string) string {
	return msg.Format(l, "collab.tableAdded", msg.Args{"table": name})
}

// ArrivedNotice : message de la barre d'état quand un tableau ajouté par un
// collègue est reçu, dans la langue l.
func ArrivedNotice(l msg.Lang, name string) string {
	return msg.Format(l, "collab.tableArrived", msg.Args{"table": name})
}

// CanAdd dit pourquoi le tableau local tbl ne peut pas être ajouté au
// classeur partagé ("" : il peut l'être).
func (e *Engine) CanAdd(tbl int64) string {
	info := e.link.Info()
	switch {
	case info == nil || info.Mode != workbook.Collaborative:
		return msg.Text(e.lang(), ErrNotCollaborative)
	case e.link.TableShare(tbl) == "published":
		return e.say("collab.alreadyShared", nil)
	case !info.CanDesign():
		return msg.Text(e.lang(), workbook.ErrAddDesign)
	case e.server() == nil || !e.link.Online():
		return msg.Text(e.lang(), ErrAddOffline)
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
		if _, ok := workbook.Refusal(err); ok {
			return "", &RefusedError{Kind: command.KindDo, Text: workbook.ServerMessage(err)}
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

// AutoShare ajoute au classeur partagé les tableaux nés sur ce poste
// (oplog.Born : saisis, collés, insérés, convertis, importés…) : partage
// automatique, sans confirmation, comme « Ajouter au classeur partagé ».
// Propriétaires et concepteurs, en ligne : hors ligne, ils attendent la
// reconnexion (la liste est gardée dans le classeur). Restent sur ce
// poste, avec leur marque « sur ce poste » et l'action à la main : un
// tableau chargé par une requête (choix de l'actualisation à faire à la
// main), celui d'un éditeur (un message, une fois par session), celui que
// le serveur ou les vérifications refusent (un message). Une erreur
// passagère (coupure) le laisse dans la liste et est rendue.
func (e *Engine) AutoShare(ctx context.Context) (Report, error) {
	var rep Report
	info, srv := e.active()
	if info == nil || srv == nil {
		return rep, nil
	}
	born, err := oplog.Born(ctx, e.db)
	if err != nil || len(born) == 0 {
		return rep, err
	}
	var done []int64
	defer func() {
		if len(done) > 0 {
			_ = oplog.ForgetBorn(context.WithoutCancel(ctx), e.db, done...)
		}
	}()
	for _, tbl := range born {
		var name, query string
		err := e.db.Read().QueryRowContext(ctx, `SELECT t.name, coalesce((SELECT q.name FROM _lattis_queries q WHERE q.target = t.id), '')
			FROM _lattis_tables t WHERE t.id = ?`, tbl).Scan(&name, &query)
		if errors.Is(err, sql.ErrNoRows) {
			done = append(done, tbl) // supprimé ou annulé depuis
			continue
		}
		if err != nil {
			return rep, err
		}
		switch {
		case e.link.TableShare(tbl) == "published" || query != "":
			done = append(done, tbl)
			continue
		case !info.CanDesign():
			done = append(done, tbl)
			e.mu.Lock()
			told := e.toldLocal
			e.toldLocal = true
			e.mu.Unlock()
			if !told {
				rep.notice(e.say("collab.tableStaysLocal", msg.Args{"table": name}))
			}
			continue
		case !e.link.Online():
			continue // à la reconnexion
		}
		added, err := e.AddTable(ctx, tbl)
		if added != "" {
			done = append(done, tbl)
			rep.notice(AddedNotice(e.lang(), added))
			rep.Shared = append(rep.Shared, tbl)
			continue
		}
		if !definitive(err) {
			return rep, err
		}
		done = append(done, tbl)
		rep.notice(e.say("collab.tableNotAdded", msg.Args{"table": name, "err": msg.Text(e.lang(), err)}))
	}
	return rep, nil
}

// definitive : l'ajout d'un tableau est refusé pour de bon (serveur,
// vérifications locales), pas seulement remis (coupure, ajout concurrent).
func definitive(err error) bool {
	var refused *RefusedError
	if errors.As(err, &refused) {
		return true
	}
	if errors.Is(err, ErrAddOffline) || errors.Is(err, workbook.ErrAddBusy) {
		return false
	}
	var m *msg.Error
	return errors.As(err, &m) && m.Err == nil
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
		rep.notice(ArrivedNotice(e.lang(), t.Name))
		rep.Applied.Tables = append(rep.Applied.Tables, t.Table)
	}
	return rep, nil
}
