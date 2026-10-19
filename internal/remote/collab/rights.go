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
	return workbook.RefreshCapabilities(ctx, srv, e.link)
}
