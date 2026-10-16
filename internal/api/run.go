package api

import (
	"context"
	"errors"
	"sync"

	"github.com/lalamefine/slitable/internal/command"
	"github.com/lalamefine/slitable/internal/script"
)

// ScriptResult est l'issue d'un script.
type ScriptResult struct {
	Output     []string `json:"output"`
	Writes     int      `json:"writes"`
	DurationMs int64    `json:"durationMs"`
	Error      string   `json:"error"`
}

// Un seul script à la fois, que le bouton Arrêter peut annuler.
var running struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

// ErrBusy refuse un script pendant qu'un autre tourne.
var ErrBusy = errors.New("un script est déjà en cours")

// RunScript exécute un script Lua (entry : fonction appelée ensuite, vide
// pour le script seul). Tout ce qu'il modifie forme une seule étape
// d'annulation, nommée d'après le script.
func RunScript(host Host, name, src, entry string) (ScriptResult, error) {
	d, err := host.Doc()
	if err != nil {
		return ScriptResult{}, err
	}
	if err := script.Check(src); err != nil {
		return ScriptResult{Error: err.Error()}, nil
	}
	running.mu.Lock()
	if running.cancel != nil {
		running.mu.Unlock()
		return ScriptResult{}, ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	running.cancel = cancel
	running.mu.Unlock()
	defer func() {
		running.mu.Lock()
		running.cancel = nil
		running.mu.Unlock()
		cancel()
	}()

	label := "Script"
	if name != "" {
		label = "Script « " + name + " »"
	}
	ctx = command.WithGroup(ctx, label)
	bh := &batchHost{Host: host}
	env := script.Env{
		Read:  d.DB().Read(),
		Edit:  d.Editor(label),
		Print: host.Print,
		API:   scriptAPI{bh},
		Entry: entry,
	}
	res := script.Run(ctx, env, src, script.DefaultLimits)
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
	return out, nil
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
