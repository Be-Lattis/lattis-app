package api

import (
	"context"
	"errors"
	"sync"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/script"
)

// ScriptResult est l'issue d'un script.
type ScriptResult struct {
	Output     []string `json:"output"`
	Writes     int      `json:"writes"`
	DurationMs int64    `json:"durationMs"`
	Error      string   `json:"error"`
}

// add ajoute à r l'issue d'un script lancé ensuite (écouteur d'évènement).
func (r *ScriptResult) add(o ScriptResult) {
	if r.Output == nil {
		r.Output = []string{}
	}
	r.Output = append(r.Output, o.Output...)
	r.Writes += o.Writes
	r.DurationMs += o.DurationMs
	if r.Error == "" {
		r.Error = o.Error
	}
}

// Un seul script à la fois, que le bouton Arrêter peut annuler.
var running struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

// ErrBusy refuse un script pendant qu'un autre tourne.
var ErrBusy = errors.New("un script est déjà en cours")

type ctxKey int

const (
	scriptKey ctxKey = iota // nom du script en cours
	queueKey                // *eventQueue : évènements à traiter après le script
)

// currentScript donne le nom du script qui appelle une commande ("" hors
// script : serveur MCP).
func currentScript(ctx context.Context) string {
	s, _ := ctx.Value(scriptKey).(string)
	return s
}

func scriptLabel(name string) string {
	if name == "" {
		return "Script"
	}
	return "Script « " + name + " »"
}

// RunScript exécute un script Lua (entry : fonction appelée ensuite, vide
// pour le script seul). Tout ce qu'il modifie forme une seule étape
// d'annulation, nommée d'après le script. Les évènements des contrôles
// qu'il provoque (controls.set d'une valeur…) sont traités à sa fin, dans
// la même étape.
func RunScript(host Host, name, src, entry string) (ScriptResult, error) {
	return RunScriptFrom(context.Background(), host, name, src, entry)
}

// RunScriptFrom est RunScript lancé depuis parent : un script lancé par le
// serveur MCP (WithoutPrompt) ne consulte pas l'utilisateur pour joindre un
// hôte HTTP. parent n'arrête pas le script (seuls Arrêter et la durée
// maximale le font).
func RunScriptFrom(parent context.Context, host Host, name, src, entry string) (ScriptResult, error) {
	ctx := context.Background()
	if !CanPrompt(parent) {
		ctx = WithoutPrompt(ctx)
	}
	ctx, q := withQueue(command.WithGroup(ctx, scriptLabel(name)))
	res, err := runOne(ctx, host, name, src, entry, nil)
	if err != nil || res.Error != "" {
		return res, err
	}
	return res, drain(ctx, host, q, &res)
}

// runOne exécute un script dans ctx (qui porte déjà son groupe
// d'annulation) ; event, s'il est donné, est passé à la fonction entry.
func runOne(ctx context.Context, host Host, name, src, entry string, event any) (ScriptResult, error) {
	res, _, err := runLimited(ctx, host, name, src, entry, event, script.DefaultLimits)
	return res, err
}

// runLimited est runOne avec des bornes choisies ; il renvoie aussi la
// valeur rendue par la fonction entry.
func runLimited(ctx context.Context, host Host, name, src, entry string, event any, lim script.Limits) (ScriptResult, any, error) {
	d, err := host.Doc()
	if err != nil {
		return ScriptResult{}, nil, err
	}
	if err := script.Check(src); err != nil {
		return ScriptResult{Output: []string{}, Error: err.Error()}, nil, nil
	}
	running.mu.Lock()
	if running.cancel != nil {
		running.mu.Unlock()
		return ScriptResult{}, nil, ErrBusy
	}
	ctx, cancel := context.WithCancel(ctx)
	running.cancel = cancel
	running.mu.Unlock()
	defer func() {
		running.mu.Lock()
		running.cancel = nil
		running.mu.Unlock()
		cancel()
	}()

	ctx = context.WithValue(ctx, scriptKey, name)
	bh := &batchHost{Host: host}
	env := script.Env{
		Read:    d.DB().Read(),
		Edit:    d.Editor(scriptLabel(name)),
		Print:   host.Print,
		API:     scriptAPI{bh},
		Entry:   entry,
		Event:   event,
		Prepare: d.ReadCTE,
	}
	res := script.Run(ctx, env, src, lim)
	out := ScriptResult{Output: res.Output, Writes: res.Writes, DurationMs: res.Duration.Milliseconds()}
	if out.Output == nil {
		out.Output = []string{}
	}
	if res.Err != nil {
		out.Error = res.Err.Error()
	}
	if res.Writes > 0 {
		bh.dirty = true
	}
	bh.flush(context.Background())
	return out, res.Return, nil
}

// Busy dit si un script tourne.
func Busy() bool {
	running.mu.Lock()
	defer running.mu.Unlock()
	return running.cancel != nil
}

// StopScript arrête le script en cours (bouton Arrêter).
func StopScript() {
	running.mu.Lock()
	defer running.mu.Unlock()
	if running.cancel != nil {
		running.cancel()
	}
}

var contextBG = context.Background()
