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
	EventUsage    = "usage"     // jetons consommés par un tour du modèle
	EventDone     = "done"      // fin de la réponse (émis par l'appelant)
)

// Event est un évènement d'une réponse en cours.
type Event struct {
	Kind  string `json:"kind"`
	Text  string `json:"text,omitempty"`
	Tool  string `json:"tool,omitempty"`  // nom de l'outil
	Label string `json:"label,omitempty"` // libellé lisible de l'appel
	ID    string `json:"id,omitempty"`    // identifiant de l'appel
	Error bool   `json:"error,omitempty"`
	// Write : l'outil modifie le classeur (tool, tool-done).
	Write bool `json:"write,omitempty"`
	// Target : feuille ou tableau visé par l'outil, pour « Aller à ».
	Target string `json:"target,omitempty"`
	// Usage : jetons et coût du tour (usage).
	Usage *Usage `json:"usage,omitempty"`
	// Changes : nombre d'appels d'outils d'écriture réussis de la réponse ;
	// Undo : la réponse peut être annulée d'un coup (done).
	Changes int  `json:"changes,omitempty"`
	Undo    bool `json:"undo,omitempty"`
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
	// Write dit si un outil modifie le classeur ; Target donne la feuille
	// ou le tableau qu'il vise (facultatifs).
	Write  func(name string) bool
	Target func(name string, input json.RawMessage) string
}

// Session est une conversation, dans le format neutre. Elle n'est gardée
// avec le classeur que sur demande (réglage ai.keepConversation, voir
// internal/bridge).
type Session struct {
	mu      sync.Mutex
	history []Message
	busy    bool
	usage   Usage
	// gen change quand la conversation est remplacée (Restore) : une
	// réponse encore en cours n'y ajoute plus rien.
	gen int
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
	s.usage = Usage{}
	return nil
}

// Restore remplace la conversation (classeur ouvert ou fermé), même
// pendant une réponse : celle-ci, interrompue par l'appelant, n'ajoute plus
// rien à la nouvelle conversation.
func (s *Session) Restore(history []Message, usage Usage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append([]Message(nil), history...)
	s.usage = usage
	s.gen++
}

// Snapshot rend la conversation dans sa forme neutre (sans les messages
// propres au fournisseur) et le cumul des jetons, pour l'enregistrer.
func (s *Session) Snapshot() ([]Message, Usage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Message, len(s.history))
	for i, m := range s.history {
		out[i] = Message{Role: m.Role, Content: m.Content}
	}
	return out, s.usage
}

// Usage donne le cumul des jetons et du coût de la conversation.
func (s *Session) Usage() Usage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.usage
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
	gen := s.gen
	// Point de retour en cas d'échec au premier tour ; le message peut
	// rejoindre le dernier message de l'utilisateur (appendUser).
	start, merged := len(s.history), -1
	if start > 0 && s.history[start-1].Role == User {
		merged = len(s.history[start-1].Content)
	}
	s.appendUser(Block{Type: TextBlock, Text: text})
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
	}()
	// update modifie la conversation, sauf si elle a été remplacée entre
	// temps (classeur ouvert pendant la réponse).
	update := func(fn func()) bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.gen != gen {
			return false
		}
		fn()
		return true
	}

	for step := 0; ; step++ {
		if step >= MaxSteps {
			emit(Event{Kind: EventNotice, Text: fmt.Sprintf("Réponse arrêtée après %d étapes.", MaxSteps)})
			return nil
		}
		var msgs []Message
		if !update(func() {
			s.shorten(MaxHistoryChars)
			msgs = append([]Message(nil), s.history...)
		}) {
			return context.Canceled
		}
		reply, err := t.Provider.Turn(ctx, t.Cred, Request{Model: t.Model, System: t.System, Messages: msgs, Tools: t.Tools}, emit)
		if err == nil && reply.Stop == StopRefusal {
			err = errors.New("le modèle a refusé de répondre à cette demande")
		}
		if err != nil {
			if step == 0 {
				update(func() {
					s.history = s.history[:start]
					if merged >= 0 {
						last := &s.history[start-1]
						last.Content = last.Content[:merged:merged]
					}
				})
				return unsent{err}
			}
			return err
		}
		u := reply.Usage
		if u.Cost == 0 {
			u.Cost = Cost(t.Model, u)
		}
		if !update(func() {
			s.history = append(s.history, reply.Message)
			s.usage.Add(u)
		}) {
			return context.Canceled
		}
		if u != (Usage{}) {
			emit(Event{Kind: EventUsage, Usage: &u})
		}
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
			ev := Event{Kind: EventTool, Tool: c.Name, Label: label, ID: c.ID}
			if t.Write != nil {
				ev.Write = t.Write(c.Name)
			}
			if t.Target != nil {
				ev.Target = t.Target(c.Name, c.Input)
			}
			emit(ev)
			res, isErr := t.Exec(ctx, c.Name, c.Input)
			ev.Kind, ev.Error, ev.Text = EventToolDone, isErr, summary(res, isErr)
			emit(ev)
			results = append(results, Block{Type: ToolResultBlock, ToolUseID: c.ID, Text: res, IsError: isErr})
		}
		if !update(func() { s.appendUser(results...) }) {
			return context.Canceled
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

// unsent : le message n'a pas été reçu par le modèle (échec au premier
// tour) ; il est retiré de la conversation.
type unsent struct{ error }

func (e unsent) Unwrap() error { return e.error }

// Unsent dit si err vient d'un message retiré de la conversation, que
// l'utilisateur peut renvoyer.
func Unsent(err error) bool {
	var u unsent
	return errors.As(err, &u)
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
