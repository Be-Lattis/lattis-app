package api

import (
	"context"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/doc"
)

// Host est l'application qui exécute les commandes (internal/bridge) : le
// classeur ouvert et l'interface.
type Host interface {
	// Doc donne le classeur ouvert.
	Doc() (*doc.Document, error)
	// Changed prévient l'interface que le classeur a changé.
	Changed(ctx context.Context)

	// Selection donne la feuille affichée et la sélection de la grille.
	Selection() Selection
	// Select affiche la feuille de s (vue grille) et y sélectionne la plage.
	Select(ctx context.Context, s Selection) error
	// ShowView affiche une vue : "grid", "sql" ou "script".
	ShowView(ctx context.Context, view string) error
	// Dialog ouvre une boîte de dialogue et attend la réponse de
	// l'utilisateur ; ok est faux s'il l'a fermée sans valider.
	Dialog(ctx context.Context, d Dialog) (values map[string]any, ok bool, err error)
	// Notify affiche un message dans la barre d'état.
	Notify(msg string)
	// Print reçoit une ligne de sortie d'un script (console de l'onglet SCRIPT).
	Print(line string)

	// Classeur (serveur MCP seulement).
	NewDoc(ctx context.Context) error
	OpenDoc(ctx context.Context, path string) error
	SaveDoc(ctx context.Context, path string) error
}

// Selection est une plage de la feuille, à partir de 0, R0 ≤ R1 et C0 ≤ C1.
type Selection struct {
	Sheet int64  `json:"sheet"`
	R0    int    `json:"r0"`
	C0    int    `json:"c0"`
	R1    int    `json:"r1"`
	C1    int    `json:"c1"`
	View  string `json:"view,omitempty"` // grid, sql ou script
}

// Dialog est une boîte de dialogue ouverte par un script ou un assistant.
type Dialog struct {
	Title   string  `json:"title"`
	Message string  `json:"message,omitempty"`
	Fields  []Field `json:"fields"`
	OK      string  `json:"ok"`               // libellé du bouton de validation
	Cancel  string  `json:"cancel,omitempty"` // vide : pas de bouton Annuler
}

// Types de champs d'une boîte de dialogue.
var fieldTypes = map[string]bool{
	"text": true, "textarea": true, "password": true, "number": true, "integer": true,
	"checkbox": true, "select": true, "radio": true, "date": true, "color": true,
}

// Field est un champ de boîte de dialogue.
type Field struct {
	Name        string   `json:"name"`
	Label       string   `json:"label,omitempty"`
	Type        string   `json:"type,omitempty"` // text par défaut
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"` // select, radio
	Placeholder string   `json:"placeholder,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
}

// batchHost regroupe les avis de modification d'un script : l'interface
// relit le classeur au plus tous les quelques dixièmes de seconde, avant
// toute commande d'interface et à la fin du script.
type batchHost struct {
	Host
	mu    sync.Mutex
	dirty bool
	last  time.Time
}

const batchEvery = 300 * time.Millisecond

func (b *batchHost) Changed(ctx context.Context) {
	b.mu.Lock()
	b.dirty = true
	due := time.Since(b.last) >= batchEvery
	b.mu.Unlock()
	if due {
		b.flush(ctx)
	}
}

func (b *batchHost) flush(ctx context.Context) {
	b.mu.Lock()
	dirty := b.dirty
	b.dirty, b.last = false, time.Now()
	b.mu.Unlock()
	if dirty {
		b.Host.Changed(ctx)
	}
}

func (b *batchHost) Select(ctx context.Context, s Selection) error {
	b.flush(ctx)
	return b.Host.Select(ctx, s)
}

func (b *batchHost) ShowView(ctx context.Context, view string) error {
	b.flush(ctx)
	return b.Host.ShowView(ctx, view)
}

func (b *batchHost) Dialog(ctx context.Context, d Dialog) (map[string]any, bool, error) {
	b.flush(ctx)
	return b.Host.Dialog(ctx, d)
}
