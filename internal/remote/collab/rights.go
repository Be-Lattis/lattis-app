package collab

import (
	"context"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// RightsNotice : message de la barre d'état quand les droits de
// l'utilisateur sur le classeur ont changé (§8.1).
const RightsNotice = "Vos droits sur ce classeur ont changé"

// RefreshRights relit les droits de l'utilisateur (lattis.my_capabilities)
// dans _lattis_capabilities (à la connexion et sur la notification *acl ;
// voir docs/collaboration/droits.md) ; vrai s'ils ont changé.
func (e *Engine) RefreshRights(ctx context.Context) (bool, error) {
	info, srv := e.active()
	if info == nil {
		return false, nil
	}
	changed, reads, err := workbook.RefreshCapabilitiesReads(ctx, srv, e.link)
	if len(reads) > 0 {
		// Lignes visibles changées avec les droits : ces tableaux sont relus
		// en entier à la prochaine réception (lignes devenues visibles
		// reçues, devenues invisibles retirées).
		e.mu.Lock()
		if e.rereads == nil {
			e.rereads = map[int64]bool{}
		}
		for tbl := range reads {
			e.rereads[tbl] = true
		}
		e.mu.Unlock()
	}
	return changed, err
}

// takeRereads rend (et oublie) les tableaux à relire en entier après un
// changement de droits.
func (e *Engine) takeRereads() map[int64]bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := e.rereads
	e.rereads = nil
	return r
}
