//go:build !web

package bridge

import (
	"context"
	"fmt"
	"os"

	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/trace"
	"github.com/lalamefine/lattis/internal/update"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Mise à jour de l'application de bureau (internal/update ; décisions de
// Louis du 20/10/2026 dans docs/conception/mise-a-jour-automatique.md) :
// la barre d'état propose la nouvelle version, la télécharge et la vérifie
// au clic, puis l'installe à la fermeture. Service du bureau seulement : le
// moteur web (-tags web) ne l'a pas, la version y est celle du serveur.

// UpdateEvent porte l'état de la mise à jour vers l'interface.
const UpdateEvent = "update:status"

func init() {
	application.RegisterEvent[update.Status](UpdateEvent)
}

// UpdateService : la mise à jour vue de l'interface (barre d'état).
type UpdateService struct {
	u   *update.Updater
	ctx context.Context
}

// NewUpdateService prépare la mise à jour de l'exécutable en cours ; la
// recherche est permise par updates.check (verrouillable par stratégie).
func NewUpdateService() *UpdateService {
	u := update.New(Version)
	u.Enabled = func() bool { return config.UpdateCheck(config.Current()) }
	u.OnChange = func(s update.Status) {
		if app := application.Get(); app != nil {
			app.Event.Emit(UpdateEvent, s)
		}
	}
	return &UpdateService{u: u, ctx: context.Background()}
}

// ServiceStartup (Wails) : note le lancement (premier lancement après une
// mise à jour ?) puis cherche les nouvelles versions en arrière-plan.
func (s *UpdateService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.ctx = ctx
	s.u.Init()
	trace.Logf(1, trace.Doc, "mise à jour : version %s, installation %s", Version, s.u.Kind)
	go s.u.Run(ctx, update.FirstCheckDelay)
	return nil
}

// Status : état de la mise à jour, textes dans la langue de la session.
func (s *UpdateService) Status() update.Status {
	return s.u.Status(msg.Current())
}

// Prepare télécharge et vérifie la nouvelle version, en arrière-plan ;
// l'évènement update:status suit l'avancement.
func (s *UpdateService) Prepare() {
	go func() {
		defer recoverPanic("mise à jour")
		if err := s.u.Prepare(s.ctx); err != nil {
			trace.Logf(1, trace.Doc, "mise à jour : %v", err)
		}
	}()
}

// RequestRestart : l'utilisateur redémarre pour appliquer la mise à jour ;
// l'interface ferme ensuite la fenêtre par le chemin habituel (questions
// d'enregistrement, écouteurs before_close), et l'application se relance
// après l'installation.
func (s *UpdateService) RequestRestart() {
	s.u.RequestRestart()
}

// DismissUpdated ferme l'avis « Mise à jour effectuée ».
func (s *UpdateService) DismissUpdated() {
	s.u.DismissUpdated()
}

// ApplyPendingUpdate installe la mise à jour prête, une fois l'application
// fermée (main.go, PostShutdown).
func ApplyPendingUpdate(s *UpdateService) {
	if s == nil {
		return
	}
	// Un échec est gardé dans update.json et dit au lancement suivant.
	if err := s.u.ApplyOnExit(); err != nil {
		fmt.Fprintf(os.Stderr, "lattis : mise à jour : %v\n", err)
	}
}
