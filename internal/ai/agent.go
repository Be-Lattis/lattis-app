package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// MaxSteps borne le nombre de tours d'une réponse (appels d'outils compris).
const MaxSteps = 40

// Types d'évènements envoyés à l'interface pendant une réponse.
const (
	EventText     = "text"      // morceau de texte de l'assistant
	EventTool     = "tool"      // début d'un appel d'outil
	EventToolDone = "tool-done" // fin d'un appel d'outil
	EventNotice   = "notice"    // remarque (réponse tronquée, refus…)
)

// Event est un évènement d'une réponse en cours.
type Event struct {
	Kind  string `json:"kind"`
	Text  string `json:"text,omitempty"`
	Tool  string `json:"tool,omitempty"`  // nom de l'outil
	Label string `json:"label,omitempty"` // libellé lisible de l'appel
	ID    string `json:"id,omitempty"`    // identifiant de l'appel
	Error bool   `json:"error,omitempty"`
}

// Executor exécute un appel d'outil et rend son résultat (texte, souvent du
// JSON) ; isError : l'outil a échoué (le modèle le lit et peut corriger).
type Executor func(ctx context.Context, name string, input json.RawMessage) (result string, isError bool)

// Turn décrit une réponse à demander : fournisseur, modèle, outils.
type Turn struct {
	Provider Provider
	Cred     Credentials
	Model    string
	System   string
	Tools    []Tool
	Exec     Executor
	// Label donne le libellé lisible d'un appel d'outil (facultatif).
	Label func(name string, input json.RawMessage) string
}

// Session est une conversation. Elle n'est pas enregistrée dans le classeur.
type Session struct {
	mu      sync.Mutex
	history []Message
	busy    bool
}

// ErrBusy : une réponse est déjà en cours.
var ErrBusy = errors.New("une réponse est déjà en cours")

// Reset vide la conversation.
func (s *Session) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		return ErrBusy
	}
	s.history = nil
	return nil
}

// Len donne le nombre de messages de la conversation.
func (s *Session) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.history)
}

// Send ajoute le message de l'utilisateur et fait répondre le modèle,
// outils compris, jusqu'à sa réponse finale. Les évènements (texte au fil de
// l'eau, appels d'outils) passent par emit. Une erreur avant toute réponse
// du modèle retire le message de l'utilisateur : il peut le renvoyer.
func (s *Session) Send(ctx context.Context, t Turn, text string, emit func(Event)) error {
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return ErrBusy
	}
	s.busy = true
	start := len(s.history)
	s.appendUser(Block{Type: TextBlock, Text: text})
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
	}()

	for step := 0; ; step++ {
		if step >= MaxSteps {
			emit(Event{Kind: EventNotice, Text: fmt.Sprintf("Réponse arrêtée après %d étapes.", MaxSteps)})
			return nil
		}
		s.mu.Lock()
		msgs := append([]Message(nil), s.history...)
		s.mu.Unlock()
		reply, err := t.Provider.Turn(ctx, t.Cred, Request{Model: t.Model, System: t.System, Messages: msgs, Tools: t.Tools}, emit)
		if err == nil && reply.Stop == StopRefusal {
			err = errors.New("le modèle a refusé de répondre à cette demande")
		}
		if err != nil {
			if step == 0 {
				s.mu.Lock()
				s.history = s.history[:start]
				s.mu.Unlock()
			}
			return err
		}
		s.mu.Lock()
		s.history = append(s.history, reply.Message)
		s.mu.Unlock()
		if reply.Stop == StopMaxTokens {
			emit(Event{Kind: EventNotice, Text: "Réponse tronquée (longueur maximale atteinte)."})
		}
		var calls []Block
		for _, b := range reply.Message.Content {
			if b.Type == ToolUseBlock {
				calls = append(calls, b)
			}
		}
		if len(calls) == 0 {
			return nil
		}
		// Chaque appel reçoit un résultat, même interrompu : la conversation
		// reste valide pour la suite.
		results := make([]Block, 0, len(calls))
		for _, c := range calls {
			label := c.Name
			if t.Label != nil {
				label = t.Label(c.Name, c.Input)
			}
			if ctx.Err() != nil {
				results = append(results, Block{Type: ToolResultBlock, ToolUseID: c.ID, Text: "interrompu par l'utilisateur", IsError: true})
				continue
			}
			emit(Event{Kind: EventTool, Tool: c.Name, Label: label, ID: c.ID})
			res, isErr := t.Exec(ctx, c.Name, c.Input)
			emit(Event{Kind: EventToolDone, Tool: c.Name, Label: label, ID: c.ID, Error: isErr, Text: summary(res, isErr)})
			results = append(results, Block{Type: ToolResultBlock, ToolUseID: c.ID, Text: res, IsError: isErr})
		}
		s.mu.Lock()
		s.appendUser(results...)
		s.mu.Unlock()
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

// appendUser ajoute des blocs de l'utilisateur ; ils rejoignent le dernier
// message s'il est déjà de l'utilisateur (résultats d'outils d'une réponse
// interrompue suivis d'un nouveau message), les fournisseurs exigeant
// l'alternance des rôles. À appeler sous s.mu.
func (s *Session) appendUser(blocks ...Block) {
	if n := len(s.history); n > 0 && s.history[n-1].Role == User {
		s.history[n-1].Content = append(s.history[n-1].Content, blocks...)
		return
	}
	s.history = append(s.history, Message{Role: User, Content: blocks})
}

// summary : l'erreur d'un outil, pour l'interface (le résultat complet ne
// sert qu'au modèle).
func summary(res string, isErr bool) string {
	if !isErr {
		return ""
	}
	r := []rune(res)
	if len(r) > 300 {
		return string(r[:300]) + "…"
	}
	return res
}
