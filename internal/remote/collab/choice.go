package collab

import (
	"context"
	"database/sql"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Onglets à choix d'un classeur collaboratif publié (conception
// regrouper-onglets-semblables §6.3, lot 5 ; remote/workbook/choice.go) :
// la catégorie d'une feuille posée, changée ou retirée par un concepteur
// part dans la mise en page du serveur (PublishChoices, au tour du suivi
// qui suit la modification, ou à la connexion) ; les autres postes la
// reçoivent sur « *meta » et à la connexion (receiveChoices), en gardant
// leur valeur choisie. Sans droit de conception, la catégorie reste sur le
// poste.

// PublishChoices publie les catégories d'onglets à choix changées sur le
// poste depuis la dernière mise en page reçue. Rien n'est relu tant
// qu'aucune catégorie n'a changé en local (changer d'année n'en est pas
// une), sauf force (connexion). Un refus du serveur (droits retirés) laisse
// la catégorie sur le poste, sans erreur.
func (e *Engine) PublishChoices(ctx context.Context, force bool) error {
	e.run.Lock()
	defer e.run.Unlock()
	info, srv := e.active()
	if info == nil || !e.link.Online() {
		return nil
	}
	sig, err := workbook.ChoiceSignature(ctx, e.db.Read())
	if err != nil || (sig == e.choices && !force) {
		return err
	}
	if _, err := workbook.PublishChoices(ctx, srv, e.db, info); err != nil {
		if _, ok := workbook.Rejected(err); !ok {
			return err
		}
	}
	e.choices = sig
	return nil
}

// receiveChoices applique en local les catégories d'onglets à choix
// changées sur le serveur.
func (e *Engine) receiveChoices(ctx context.Context) (Report, error) {
	var rep Report
	info, srv := e.active()
	if info == nil {
		return rep, nil
	}
	var c *workbook.Choices
	err := srv.Delta(ctx, func(r workbook.Reader) error {
		var err error
		c, err = workbook.ReadChoices(ctx, r, info.Schema)
		return err
	})
	if err != nil {
		return rep, err
	}
	changed := false
	err = e.db.Write(ctx, func(tx *sql.Tx) error {
		var err error
		changed, err = workbook.ApplyChoices(ctx, tx, c)
		return err
	})
	if err != nil || !changed {
		return rep, err
	}
	e.settleChoices(ctx)
	if sig, err := workbook.ChoiceSignature(ctx, e.db.Read()); err == nil {
		e.choices = sig
	}
	rep.Choices = true
	return rep, nil
}

// settleChoices règle les onglets à choix reçus sans valeur choisie sur la
// dernière valeur (canvas.SettleChoices).
func (e *Engine) settleChoices(ctx context.Context) {
	if e.model != nil {
		_ = e.model.SettleChoices(ctx)
	}
}
