package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// Langue de la session (docs/i18n.md) : l'interface la résout (réglage
// ui.language, sinon la langue du système que donne Platform().Locale sur le
// bureau, ou celle du navigateur en web) et la transmet par SetLanguage, au
// démarrage et à chaque changement. Elle sert aux messages du moteur
// (*msg.Error, traduits en rendant l'erreur) et à la locale des formules du
// classeur ouvert.

type langState struct {
	mu   sync.Mutex
	lang msg.Lang // "" : pas encore transmise
}

// SetLanguage fixe la langue de la session (fr, en, de, es, zh).
func (s *DocumentService) SetLanguage(ctx context.Context, lang string) error {
	l, ok := msg.Parse(lang)
	if !ok {
		return errors.New("langue inconnue : " + lang)
	}
	s.lang.mu.Lock()
	s.lang.lang = l
	s.lang.mu.Unlock()
	if !s.sys().Platform().Web {
		msg.SetCurrent(l) // bureau : une seule session (MarshalError)
	}
	if d, err := s.current(); err == nil {
		applyLocale(d, l)
	}
	return nil
}

// sessionLang : la langue de la session ; avant SetLanguage, le réglage
// ui.language, sinon la langue du système (bureau).
func (s *DocumentService) sessionLang() msg.Lang {
	s.lang.mu.Lock()
	l := s.lang.lang
	s.lang.mu.Unlock()
	if l != "" {
		return l
	}
	system := ""
	if !s.sys().Platform().Web {
		system = msg.SystemLocale()
	}
	return msg.Resolve(config.Current().String("ui.language", msg.Auto), system)
}

// applyLocale donne au classeur la locale des formules de la langue l.
func applyLocale(d *doc.Document, l msg.Lang) {
	if d != nil && d.Model != nil {
		d.SetLocale(formula.ForLanguage(string(l)))
	}
}

// ErrorCause : ce que l'interface reçoit d'un *msg.Error, en plus du texte
// français de l'erreur (cause de l'erreur d'appel, lue par errorText dans
// frontend/src/lib/i18n) : l'identifiant, les paramètres et le texte dans
// la langue de la session.
type ErrorCause struct {
	ID      msg.ID   `json:"id"`
	Args    msg.Args `json:"args,omitempty"`
	Message string   `json:"message"`
}

// errorCause : la cause d'une erreur dans la langue l ; faux pour une erreur
// ordinaire (texte français, comme avant).
func errorCause(l msg.Lang, err error) (ErrorCause, bool) {
	var e *msg.Error
	if !errors.As(err, &e) {
		return ErrorCause{}, false
	}
	return ErrorCause{ID: e.ID, Args: e.Args, Message: msg.Text(l, err)}, true
}

// MarshalError traduit les erreurs rendues à l'interface du bureau (option
// MarshalError de Wails, main.go) : la cause d'un *msg.Error dans la langue
// de la session ; nil pour les autres (traitement par défaut de Wails).
func MarshalError(err error) []byte {
	c, ok := errorCause(msg.Current(), err)
	if !ok {
		return nil
	}
	b, jerr := json.Marshal(c)
	if jerr != nil {
		return nil
	}
	return b
}

// ErrorTranslator traduit les erreurs rendues à l'interface web de la
// session d (transport.Hub.Translate) : texte dans sa langue, et la cause
// d'un *msg.Error (nil sinon). Une fonction, pas une méthode : les méthodes
// du service sont exposées à l'interface.
func ErrorTranslator(d *DocumentService) func(error) (string, any) {
	return func(err error) (string, any) {
		if c, ok := errorCause(d.sessionLang(), err); ok {
			return c.Message, c
		}
		return err.Error(), nil
	}
}
