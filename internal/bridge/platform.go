package bridge

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/config"
)

// Hôte des services : ce que le bureau (Wails) et la version web (moteur
// servi par lattisd, cmd/lattis-engine) font différemment. Les services ne
// touchent ni aux boîtes de dialogue du système, ni aux évènements, ni à la
// fenêtre autrement que par Host (spécification collaboration, § 12.1).

// FileFilter est un filtre d'une boîte de dialogue de fichier.
type FileFilter struct {
	Name    string // libellé (« Classeur Lattis »)
	Pattern string // motifs séparés par « ; » (« *.lattis »)
}

// FileDialog décrit une demande de fichier.
type FileDialog struct {
	Title     string
	Filename  string // nom proposé (enregistrement)
	Directory string // dossier de départ (bureau)
	Filters   []FileFilter
	// Ext : extension ajoutée au nom choisi s'il ne l'a pas (« .lattis »).
	Ext string
}

// Host est l'hôte des services : bureau (wailsHost) ou web (fourni par le
// moteur web, voir engine.WebHost).
type Host interface {
	// OpenFile demande un fichier à lire et rend son chemin local, "" si
	// l'utilisateur annule. Web : le navigateur envoie le fichier, déposé
	// dans le répertoire de la session.
	OpenFile(opts FileDialog) (string, error)
	// SaveFile demande où enregistrer, fait écrire le fichier par write à
	// ce chemin et rend le chemin ("" : annulé, write n'est pas appelé).
	// Web : write écrit dans le répertoire de la session, puis le
	// navigateur télécharge le fichier.
	SaveFile(opts FileDialog, write func(path string) error) (string, error)
	// Emit pousse un évènement vers l'interface ; faux si aucune interface
	// n'est là pour le recevoir.
	Emit(event string, data any) bool
	Clipboard() Clipboard
	Secrets() SecretStore
	Window() WindowControl
	// Platform décrit l'hôte pour l'interface (web, scripting permis).
	Platform() Platform
}

// Clipboard est le presse-papiers de l'hôte (texte).
type Clipboard interface {
	Text() (string, bool)
	SetText(text string) bool
}

// SecretStore garde les secrets (mots de passe, jetons, clés d'API) :
// trousseau du système sur le bureau, secrets chiffrés par lattisd en web.
type SecretStore interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

// ErrSecretNotFound : aucun secret pour ce compte.
var ErrSecretNotFound = errors.New("secret introuvable")

// WindowControl agit sur la fenêtre de l'application (sans effet en web).
type WindowControl interface {
	SetTitle(title string)
	// Quit ferme l'application (bureau) ; en web, rien.
	Quit()
}

// Platform décrit l'hôte à l'interface (DocumentService.Platform).
type Platform struct {
	// Web : application dans le navigateur (pas de fenêtre, fichiers
	// envoyés et téléchargés).
	Web bool `json:"web"`
	// Scripting : Lua, boutons, pages web et serveur MCP permis. Toujours
	// vrai sur le bureau ; en web, réglage du serveur (web_scripting) lu à
	// l'ouverture de la session.
	Scripting bool `json:"scripting"`
}

// errNoScripting refuse un script quand l'hôte ne le permet pas.
var errNoScripting = errors.New("les scripts sont désactivés sur ce serveur (scripting web non autorisé par l'administrateur)")

// defaultHost sert aux services sans hôte attribué : Wails sur le bureau
// (host_wails.go), aucun hôte dans le moteur web compilé avec -tags web.
var defaultHost Host = NoHost{}

// UseHost attribue un hôte aux services (moteur web). Sans appel, les
// services du bureau passent par Wails.
func UseHost(h Host, g *GridService, d *DocumentService) {
	if g != nil {
		g.hostMu.Lock()
		g.h = h
		g.hostMu.Unlock()
	}
	if d != nil {
		d.hostMu.Lock()
		d.h = h
		d.hostMu.Unlock()
	}
}

type hostRef struct {
	hostMu sync.Mutex
	h      Host
}

func (r *hostRef) sys() Host {
	r.hostMu.Lock()
	defer r.hostMu.Unlock()
	if r.h != nil {
		return r.h
	}
	return defaultHost
}

// withExt ajoute l'extension attendue à un nom qui ne l'a pas.
func withExt(p, ext string) string {
	if p == "" || ext == "" || strings.HasSuffix(strings.ToLower(p), strings.ToLower(ext)) {
		return p
	}
	return p + ext
}

// NoHost est un hôte sans interface : pas de dialogue, pas d'évènement
// (tests, moteur sans session).
type NoHost struct{}

func (NoHost) OpenFile(FileDialog) (string, error) {
	return "", errors.New("aucune interface pour choisir un fichier")
}

func (NoHost) SaveFile(FileDialog, func(string) error) (string, error) {
	return "", errors.New("aucune interface pour choisir un fichier")
}

func (NoHost) Emit(string, any) bool { return false }
func (NoHost) Clipboard() Clipboard  { return &MemClipboard{} }
func (NoHost) Secrets() SecretStore  { return &MemSecrets{} }
func (NoHost) Window() WindowControl { return noWindow{} }
func (NoHost) Platform() Platform    { return Platform{Scripting: true} }

type noWindow struct{}

func (noWindow) SetTitle(string) {}
func (noWindow) Quit()           {}

// NoWindow est la fenêtre absente (web).
func NoWindow() WindowControl { return noWindow{} }

// MemClipboard est un presse-papiers en mémoire (web : le vrai est celui du
// navigateur, géré par la page).
type MemClipboard struct {
	mu   sync.Mutex
	text string
	set  bool
}

func (c *MemClipboard) Text() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text, c.set
}

func (c *MemClipboard) SetText(t string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.text, c.set = t, true
	return true
}

// MemSecrets garde des secrets en mémoire, le temps de la session (moteur
// web sans coffre fourni par lattisd).
type MemSecrets struct {
	mu sync.Mutex
	m  map[string]string
}

func (m *MemSecrets) Get(service, account string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.m[service+"\x00"+account]
	if !ok {
		return "", ErrSecretNotFound
	}
	return v, nil
}

func (m *MemSecrets) Set(service, account, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.m == nil {
		m.m = map[string]string{}
	}
	m.m[service+"\x00"+account] = secret
	return nil
}

func (m *MemSecrets) Delete(service, account string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, service+"\x00"+account)
	return nil
}

// Start démarre les services hors de Wails (moteur web) : ce que fait
// ServiceStartup sur le bureau, sauf la démonstration d'un million de lignes
// du GridService, préparée seulement si grid n'est pas nil.
func Start(ctx context.Context, grid *GridService, docs *DocumentService) error {
	if grid != nil {
		if err := grid.startup(ctx); err != nil {
			return err
		}
	}
	return docs.startup(ctx)
}

// Platform décrit l'hôte à l'interface : version web (pas de boutons de
// fenêtre, fichiers envoyés et téléchargés) et scripting permis (onglet
// SCRIPT, boutons, pages web et serveur MCP masqués sinon).
func (s *DocumentService) Platform() Platform { return s.sys().Platform() }

var errNoMCPWeb = errors.New("le serveur MCP local n'existe pas dans la version web")

var errNoMCPAI = errors.New("le serveur MCP est coupé avec l'assistant IA : activez-le dans Fichier › Paramètres › Fonctionnalités")

// mcpRefused refuse le serveur MCP en web, sans scripting, et quand
// l'assistant IA est désactivé (choix de Louis : couper les assistants
// coupe aussi le serveur qui sert aux assistants externes).
func (s *DocumentService) mcpRefused() error {
	if s.sys().Platform().Web {
		return errNoMCPWeb
	}
	if !config.Current().Bool("ai.enabled", false) {
		return errNoMCPAI
	}
	return s.scripting()
}

// scripting refuse Lua, boutons, pages web et MCP quand l'hôte ne les
// permet pas (version web sans web_scripting, § 12.4).
func (s *DocumentService) scripting() error {
	if !s.sys().Platform().Scripting {
		return errNoScripting
	}
	return nil
}
