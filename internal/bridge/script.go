package bridge

import (
	"context"
	"errors"
	"sync"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/doc"
	"github.com/lalamefine/slitable/internal/script"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// PrintEvent porte une ligne de sortie d'un script en cours (onglet SCRIPT).
const PrintEvent = "script:print"

func init() {
	application.RegisterEvent[string](PrintEvent)
}

// ScriptResult est l'issue d'un script pour l'interface.
type ScriptResult struct {
	Output     []string `json:"output"`
	Writes     int      `json:"writes"`
	DurationMs int64    `json:"durationMs"`
	Error      string   `json:"error"`
}

// scripts : un seul script à la fois, que le bouton Arrêter peut annuler.
type scripts struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

var running scripts

// RunScript exécute un script Lua sur le classeur ouvert. Ses écritures
// forment une seule étape d'annulation, nommée d'après le script.
func (s *DocumentService) RunScript(ctx context.Context, name, src string) (ScriptResult, error) {
	d, err := s.current()
	if err != nil {
		return ScriptResult{}, err
	}
	if err := script.Check(src); err != nil {
		return ScriptResult{Error: err.Error()}, nil
	}
	running.mu.Lock()
	if running.cancel != nil {
		running.mu.Unlock()
		return ScriptResult{}, errors.New("un script est déjà en cours")
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
	env := script.Env{
		Read: d.DB().Read(),
		Edit: d.Editor(label),
		Print: func(line string) {
			if app := application.Get(); app != nil {
				app.Event.Emit(PrintEvent, line)
			}
		},
	}
	res := script.Run(ctx, env, src, script.DefaultLimits)
	out := ScriptResult{Output: res.Output, Writes: res.Writes, DurationMs: res.Duration.Milliseconds()}
	if res.Err != nil {
		out.Error = res.Err.Error()
	}
	if res.Writes > 0 {
		s.changed(context.Background())
	}
	return out, nil
}

// StopScript arrête le script en cours (bouton Arrêter).
func (s *DocumentService) StopScript() {
	running.mu.Lock()
	defer running.mu.Unlock()
	if running.cancel != nil {
		running.cancel()
	}
}

// Scripts liste la bibliothèque de scripts du classeur.
func (s *DocumentService) Scripts(ctx context.Context) ([]canvas.ScriptInfo, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.Scripts(ctx)
}

// SaveScript enregistre un script (id 0 : nouveau) et renvoie son identifiant.
func (s *DocumentService) SaveScript(ctx context.Context, id int64, name, source string) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err = d.SaveScript(ctx, id, name, source)
	if err == nil {
		s.changed(ctx)
	}
	return id, err
}

// DeleteScript supprime un script.
func (s *DocumentService) DeleteScript(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteScript(ctx, id) })
}
