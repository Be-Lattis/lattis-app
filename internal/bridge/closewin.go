package bridge

import (
	"context"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Fermeture de la fenêtre (bureau, audit 6, I1) : un classeur modifié fait
// poser à l'interface la question de Fichier › Fermer (Enregistrer, Ne pas
// enregistrer, Annuler) ; la fenêtre ne se ferme qu'ensuite (QuitApp).

// CloseRequestEvent demande à l'interface de fermer le classeur modifié
// avant de quitter.
const CloseRequestEvent = "window:close-request"

// closeAckTimeout : une interface qui n'a pas pris la demande en compte
// dans ce délai est bloquée ; un nouveau clic sur fermer ferme alors la
// fenêtre (les modifications restent proposées en récupération).
const closeAckTimeout = 5 * time.Second

type closeAction int

const (
	closeAllow   closeAction = iota // laisser fermer
	closeAsked                      // question posée par l'interface : attendre
	closeScripts                    // écouteurs before_close à consulter
)

type closeRequest struct {
	mu    sync.Mutex
	asked time.Time // demande envoyée, pas encore prise en compte
}

// windowClosing décide de ce que devient un clic sur la croix de la fenêtre.
func (s *DocumentService) windowClosing(now time.Time) closeAction {
	if s.ev.closeOK.Load() {
		return closeAllow
	}
	d, err := s.current()
	if err != nil {
		return closeAllow
	}
	dirty, _ := d.Dirty(context.Background())
	if liveCache(d) {
		// Classeur collaboratif du serveur : la question ne porte que sur
		// des modifications qui ne peuvent pas partir (hors ligne) ; en
		// ligne, la file part à la fermeture (stopShare).
		pending, _, _ := workbook.Unsent(context.Background(), d.DB().Read())
		dirty = pending > 0 && !d.Share.Online()
	}
	if dirty {
		s.closeReq.mu.Lock()
		defer s.closeReq.mu.Unlock()
		if !s.closeReq.asked.IsZero() && now.Sub(s.closeReq.asked) > closeAckTimeout {
			return closeAllow
		}
		if !s.sys().Emit(CloseRequestEvent, nil) {
			return closeAllow
		}
		if s.closeReq.asked.IsZero() {
			s.closeReq.asked = now
		}
		return closeAsked
	}
	if s.listening(api.EventBeforeClose) {
		return closeScripts
	}
	return closeAllow
}

// CloseRequestSeen : l'interface a reçu la demande de fermeture de la
// fenêtre et pose la question.
func (s *DocumentService) CloseRequestSeen() {
	s.closeReq.mu.Lock()
	s.closeReq.asked = time.Time{}
	s.closeReq.mu.Unlock()
}

// QuitApp ferme la fenêtre une fois le classeur enregistré ou fermé (réponse
// à CloseRequestEvent).
func (s *DocumentService) QuitApp() {
	s.ev.closeOK.Store(true)
	s.sys().Window().Quit()
}
