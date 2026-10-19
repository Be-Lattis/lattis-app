//go:build !web

package bridge

import (
	"context"
	"errors"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/zalando/go-keyring"
)

// Hôte du bureau : Wails (boîtes de dialogue du système, évènements vers la
// fenêtre, presse-papiers, trousseau). Seul fichier du paquet à importer
// Wails ; le moteur web se compile avec -tags web, sans lui.

func init() {
	defaultHost = wailsHost{}
	// Évènements typés pour les liaisons générées (eventdata.d.ts).
	application.RegisterEvent[AIEventData](AIEvent)
	application.RegisterEvent[application.Void](AIResetEvent)
	application.RegisterEvent[DocState](ChangedEvent)
	application.RegisterEvent[UICommand](UIEvent)
	application.RegisterEvent[string](OpenFileEvent)
	application.RegisterEvent[string](PrintEvent)
}

type wailsHost struct{}

// PanicHandler est le PanicHandler de l'application Wails (main.go) : une
// panique d'une méthode liée ou d'un évènement est journalisée et
// signalée (ReportPanic) au lieu d'arrêter l'application.
func PanicHandler(p *application.PanicDetails) {
	var err any = p.Error
	if p.Error == nil {
		err = "panique"
	}
	ReportPanic("application", err, p.StackTrace)
}

func addFilters[T interface{ AddFilter(string, string) T }](d T, fs []FileFilter) T {
	for _, f := range fs {
		d = d.AddFilter(f.Name, f.Pattern)
	}
	return d
}

func (wailsHost) OpenFile(o FileDialog) (string, error) {
	app := application.Get()
	if app == nil {
		return "", errors.New("aucune fenêtre pour choisir un fichier")
	}
	d := app.Dialog.OpenFile().SetTitle(o.Title)
	if o.Directory != "" {
		d = d.SetDirectory(o.Directory)
	}
	return addFilters(d, o.Filters).PromptForSingleSelection()
}

func (wailsHost) SaveFile(o FileDialog, write func(string) error) (string, error) {
	app := application.Get()
	if app == nil {
		return "", errors.New("aucune fenêtre pour choisir un fichier")
	}
	d := app.Dialog.SaveFile().SetMessage(o.Title).SetFilename(o.Filename)
	if o.Directory != "" {
		d = d.SetDirectory(o.Directory)
	}
	p, err := addFilters(d, o.Filters).PromptForSingleSelection()
	if err != nil || p == "" {
		return "", err
	}
	p = withExt(p, o.Ext)
	if write != nil {
		if err := write(p); err != nil {
			return p, err
		}
	}
	return p, nil
}

func (wailsHost) Emit(event string, data any) bool {
	app := application.Get()
	if app == nil {
		return false
	}
	app.Event.Emit(event, data)
	return true
}

func (wailsHost) Clipboard() Clipboard  { return wailsClipboard{} }
func (wailsHost) Secrets() SecretStore  { return keyringStore{} }
func (wailsHost) Window() WindowControl { return wailsWindow{} }
func (wailsHost) Platform() Platform    { return Platform{Scripting: true} }

type wailsClipboard struct{}

func (wailsClipboard) Text() (string, bool) {
	if app := application.Get(); app != nil {
		return app.Clipboard.Text()
	}
	return "", false
}

func (wailsClipboard) SetText(t string) bool {
	if app := application.Get(); app != nil {
		return app.Clipboard.SetText(t)
	}
	return false
}

type keyringStore struct{}

func (keyringStore) Get(service, account string) (string, error) {
	v, err := keyring.Get(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrSecretNotFound
	}
	return v, err
}

func (keyringStore) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}

func (keyringStore) Delete(service, account string) error {
	if err := keyring.Delete(service, account); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}

type wailsWindow struct{}

func (wailsWindow) SetTitle(t string) {
	if app := application.Get(); app != nil {
		if w := app.Window.Current(); w != nil {
			w.SetTitle(t)
		}
	}
}

func (wailsWindow) Quit() {
	if app := application.Get(); app != nil {
		app.Quit()
	}
}

// ServiceStartup (Wails) : voir Startup.
func (s *GridService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	return s.startup(ctx)
}

// ServiceStartup (Wails) démarre le serveur MCP s'il est activé.
func (s *DocumentService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	return s.startup(ctx)
}

// GuardClose fait passer la fermeture de la fenêtre par les écouteurs
// before_close du classeur ouvert : la fermeture attend leur réponse, et
// n'a lieu que s'ils la laissent faire.
func GuardClose(s *DocumentService, w *application.WebviewWindow) {
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if s.ev.closeOK.Load() || !s.listening(api.EventBeforeClose) {
			return
		}
		e.Cancel()
		if s.ev.closing.Swap(true) {
			return // les écouteurs tournent déjà
		}
		go func() {
			defer recoverPanic("fermeture de la fenêtre")
			defer s.ev.closing.Store(false)
			if s.BeforeClose(context.Background()) {
				s.ev.closeOK.Store(true)
				w.Close()
			}
		}()
	})
}
