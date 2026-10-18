package bridge

import (
	"context"
	"errors"
	"sync"

	"github.com/lalamefine/lattis/internal/ai"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Onglet IA : conversation avec un assistant qui agit sur le classeur par
// les commandes de internal/api (voir internal/ai). Opt-in (ai.enabled) ;
// fournisseur, modèle et partage des données dans la configuration de
// l'utilisateur ; connexion par compte (sans clé) quand le fournisseur la
// permet, sinon clés d'API dans le trousseau du système.

// AIEvent porte le déroulé d'une réponse (texte au fil de l'eau, outils).
const AIEvent = "ai:event"

func init() {
	application.RegisterEvent[AIEventData](AIEvent)
}

// AIEventData est un évènement d'une réponse ; Run la désigne.
type AIEventData struct {
	Run int `json:"run"`
	ai.Event
}

type aiState struct {
	mu      sync.Mutex
	session ai.Session
	run     int
	cancel  context.CancelFunc
}

// AIProvider est un fournisseur pour l'interface.
type AIProvider struct {
	ai.Info
	HasKey bool `json:"hasKey"`
	// Account : connexion par compte, sans clé (nil : non proposée).
	Account *ai.Account `json:"account,omitempty"`
}

// AIStatus décrit l'onglet IA.
type AIStatus struct {
	Enabled bool `json:"enabled"`
	// Locked : coupé ou imposé par une stratégie de l'organisation.
	Locked      bool         `json:"locked"`
	Provider    string       `json:"provider"`
	Model       string       `json:"model"`
	DataSharing string       `json:"dataSharing"`
	Managed     bool         `json:"managed"` // passerelle de l'entreprise (ai.endpoint)
	Providers   []AIProvider `json:"providers"`
	Busy        bool         `json:"busy"`
	Messages    int          `json:"messages"`
}

// AIState décrit l'onglet IA : réglages, fournisseurs, clés présentes.
func (s *DocumentService) AIState() AIStatus {
	c := config.Current()
	st := AIStatus{
		Enabled:     c.Bool("ai.enabled", false),
		Locked:      c.Locked("ai.enabled"),
		Provider:    c.String("ai.provider", "anthropic"),
		Model:       c.String("ai.model", ""),
		DataSharing: c.String("ai.dataSharing", ai.SchemaOnly),
		Managed:     c.String("ai.endpoint", "") != "",
		Messages:    s.ai.session.Len(),
	}
	for _, info := range ai.Catalog() {
		p := AIProvider{Info: info}
		if info.Available {
			_, p.HasKey = ai.DefaultKeys.Get(info.ID)
			if ap, ok := ai.Lookup(info.ID).(ai.AccountProvider); ok {
				acc := ap.Account()
				p.Account = &acc
			}
		}
		st.Providers = append(st.Providers, p)
		if info.ID == st.Provider && st.Model == "" {
			st.Model = info.DefaultModel
		}
	}
	s.ai.mu.Lock()
	st.Busy = s.ai.cancel != nil
	s.ai.mu.Unlock()
	return st
}

// AISetKey enregistre la clé d'API d'un fournisseur dans le trousseau. Si
// le trousseau la refuse, elle sert quand même pendant la session et
// l'erreur le dit.
func (s *DocumentService) AISetKey(provider, key string) (AIStatus, error) {
	if p := ai.Lookup(provider); p == nil {
		return s.AIState(), errors.New("fournisseur pas encore disponible")
	}
	err := ai.DefaultKeys.Set(provider, key)
	if err != nil && key != "" {
		err = errors.New("clé gardée pour cette session seulement : le trousseau du système l'a refusée (" + err.Error() + ")")
	}
	return s.AIState(), err
}

// AILogin connecte le compte du fournisseur (navigateur) et rend la main
// quand la connexion est finie.
func (s *DocumentService) AILogin(ctx context.Context, provider string) (AIStatus, error) {
	ap, ok := ai.Lookup(provider).(ai.AccountProvider)
	if !ok {
		return s.AIState(), errors.New("ce fournisseur ne propose pas de connexion par compte")
	}
	err := ap.Login(context.WithoutCancel(ctx))
	return s.AIState(), err
}

// AIForgetKey oublie la clé d'API d'un fournisseur.
func (s *DocumentService) AIForgetKey(provider string) (AIStatus, error) {
	err := ai.DefaultKeys.Delete(provider)
	return s.AIState(), err
}

// AISend envoie un message à l'assistant et rend la main quand sa réponse
// est finie ; le déroulé arrive par l'évènement ai:event, marqué de run
// (numéro choisi par l'interface). Tout ce que l'assistant modifie pendant la réponse
// forme une seule étape d'annulation.
func (s *DocumentService) AISend(ctx context.Context, run int, text string) error {
	c := config.Current()
	if !c.Bool("ai.enabled", false) {
		return errors.New("l'onglet IA est désactivé (Fichier › Paramètres › Fonctionnalités)")
	}
	id := c.String("ai.provider", "anthropic")
	p := ai.Lookup(id)
	if p == nil {
		return errors.New("fournisseur pas encore disponible : " + id)
	}
	// Clé du trousseau d'abord ; sinon le compte connecté, s'il y en a un.
	key, ok := ai.DefaultKeys.Get(id)
	if !ok {
		ap, isAcc := p.(ai.AccountProvider)
		if !isAcc || !ap.Account().Connected {
			return ai.ErrNoKey
		}
	}
	model := c.String("ai.model", "")
	if model == "" {
		model = p.Info().DefaultModel
	}
	sharing := c.String("ai.dataSharing", ai.SchemaOnly)

	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.ai.mu.Lock()
	if s.ai.cancel != nil {
		s.ai.mu.Unlock()
		cancel()
		return ai.ErrBusy
	}
	s.ai.cancel = cancel
	s.ai.mu.Unlock()
	defer func() {
		s.ai.mu.Lock()
		s.ai.cancel = nil
		s.ai.mu.Unlock()
		cancel()
	}()

	h := host{s}
	turn := ai.Turn{
		Provider: p,
		Cred:     ai.Credentials{Key: key, Endpoint: c.String("ai.endpoint", "")},
		Model:    model,
		System:   ai.System(sharing),
		Tools:    ai.Tools(sharing),
		Exec:     ai.Exec(h, sharing),
		Label:    ai.Label,
	}
	emit := func(e ai.Event) {
		if app := application.Get(); app != nil {
			app.Event.Emit(AIEvent, AIEventData{Run: run, Event: e})
		}
	}
	err := s.ai.session.Send(command.WithGroup(ctx, "Assistant IA"), turn, text, emit)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// AIStop interrompt la réponse en cours.
func (s *DocumentService) AIStop() {
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	if s.ai.cancel != nil {
		s.ai.cancel()
	}
}

// AIReset commence une nouvelle conversation.
func (s *DocumentService) AIReset() error {
	return s.ai.session.Reset()
}
