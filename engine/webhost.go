package engine

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/bridge"
	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/transport"
)

// Évènements de l'hôte web, traités par le runtime web du frontend
// (frontend/src/lib/web/runtime.ts), jamais par l'application elle-même.
const (
	// OpenFileEvent demande au navigateur un fichier à envoyer
	// (OpenFileRequest).
	OpenFileEvent = "host:open-file"
	// DownloadEvent fait télécharger un fichier écrit par le moteur
	// (Download).
	DownloadEvent = "host:download"
)

// OpenFileRequest est la demande d'envoi d'un fichier.
type OpenFileRequest struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Accept []string `json:"accept"` // extensions (« .lattis ») pour l'attribut accept
	// URL où envoyer le fichier (POST, corps brut, ?name=) ou annuler
	// (?cancel=1).
	URL string `json:"url"`
}

// Download est un fichier à télécharger.
type Download struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

// PickTimeout borne l'attente d'un fichier demandé au navigateur.
const PickTimeout = 30 * time.Minute

type pickResult struct {
	path string
	err  error
}

// WebHost est l'hôte des services dans le moteur web : les boîtes de
// dialogue de fichier deviennent un envoi et un téléchargement, les
// évènements partent sur le WebSocket, la fenêtre n'existe pas.
type WebHost struct {
	hub       *transport.Hub
	dir       string // répertoire de la session
	base      string // préfixe des URL du moteur (« /engine »)
	scripting bool
	demo      bool // démo publique
	secrets   bridge.SecretStore
	clip      bridge.MemClipboard
	guard     *query.Guard // sources des requêtes : fichiers de dir, pas d'adresse interne

	mu        sync.Mutex
	picks     map[string]chan pickResult
	downloads map[string]string // jeton → fichier
}

var _ bridge.Host = (*WebHost)(nil)

func newToken() string {
	var b [16]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// OpenFile demande au navigateur d'envoyer un fichier, et attend qu'il
// arrive (ou que l'utilisateur annule, ou que l'onglet se ferme).
func (h *WebHost) OpenFile(o bridge.FileDialog) (string, error) {
	id := newToken()
	ch := make(chan pickResult, 1)
	h.mu.Lock()
	h.picks[id] = ch
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.picks, id)
		h.mu.Unlock()
	}()
	req := OpenFileRequest{ID: id, Title: o.Title, Accept: accept(o.Filters), URL: h.base + "/upload/" + id}
	if !h.hub.Emit(OpenFileEvent, req) {
		return "", errors.New("aucun navigateur connecté pour choisir un fichier")
	}
	select {
	case r := <-ch:
		return r.path, r.err
	case <-time.After(PickTimeout):
		return "", nil
	}
}

// accept traduit les filtres en extensions pour <input accept>, sans le
// filtre « tous les fichiers ».
func accept(fs []bridge.FileFilter) []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range fs {
		for _, p := range strings.Split(f.Pattern, ";") {
			p = strings.TrimSpace(p)
			if p == "*.*" || p == "*" {
				return nil
			}
			if ext := strings.TrimPrefix(p, "*"); strings.HasPrefix(ext, ".") && !seen[ext] {
				seen[ext] = true
				out = append(out, ext)
			}
		}
	}
	return out
}

// cancelPicks répond « annulé » aux demandes en attente (onglet fermé).
func (h *WebHost) cancelPicks() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.picks {
		deliver(ch, pickResult{})
	}
}

// deliver rend le résultat d'une demande, sans attendre (le premier
// arrivé l'emporte).
func deliver(ch chan pickResult, r pickResult) {
	select {
	case ch <- r:
	default:
	}
}

// safeName garde un nom de fichier sans chemin ni caractère douteux.
func safeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "fichier"
	}
	return name
}

// serveUpload reçoit le fichier choisi (POST /upload/<id>?name=…, corps
// brut) ou l'annulation (?cancel=1).
func (h *WebHost) serveUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST attendu", http.StatusMethodNotAllowed)
		return
	}
	id := r.PathValue("id")
	h.mu.Lock()
	ch := h.picks[id]
	h.mu.Unlock()
	if ch == nil {
		http.Error(w, "demande inconnue ou expirée", http.StatusNotFound)
		return
	}
	if r.URL.Query().Get("cancel") != "" {
		deliver(ch, pickResult{})
		w.WriteHeader(http.StatusNoContent)
		return
	}
	dir := filepath.Join(h.dir, "uploads", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p := filepath.Join(dir, safeName(r.URL.Query().Get("name")))
	f, err := os.Create(p)
	if err == nil {
		_, err = io.Copy(f, r.Body)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		deliver(ch, pickResult{err: err})
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	deliver(ch, pickResult{path: p})
	w.WriteHeader(http.StatusNoContent)
}

// SaveFile fait écrire le fichier dans le répertoire de la session, puis le
// fait télécharger par le navigateur.
func (h *WebHost) SaveFile(o bridge.FileDialog, write func(string) error) (string, error) {
	name := o.Filename
	if name == "" {
		name = "fichier" + o.Ext
	}
	name = safeName(name)
	if o.Ext != "" && !strings.HasSuffix(strings.ToLower(name), strings.ToLower(o.Ext)) {
		name += o.Ext
	}
	dir := filepath.Join(h.dir, "files")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, name)
	if write != nil {
		if err := write(p); err != nil {
			return "", err
		}
	}
	tok := newToken()
	h.mu.Lock()
	h.downloads[tok] = p
	h.mu.Unlock()
	h.hub.Emit(DownloadEvent, Download{URL: h.base + "/download/" + tok, Name: name})
	return p, nil
}

// serveDownload envoie un fichier écrit par SaveFile (GET /download/<jeton>).
func (h *WebHost) serveDownload(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	p, ok := h.downloads[r.PathValue("token")]
	h.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(filepath.Base(p), `"`, "_")+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, p)
}

// Emit pousse un évènement aux onglets connectés.
func (h *WebHost) Emit(event string, data any) bool { return h.hub.Emit(event, data) }

// Clipboard : le presse-papiers du navigateur est géré par la page ; côté
// moteur, une mémoire de session.
func (h *WebHost) Clipboard() bridge.Clipboard { return &h.clip }

// Secrets : fournis par lattisd (chiffrés côté serveur), en mémoire sinon.
func (h *WebHost) Secrets() bridge.SecretStore { return h.secrets }

// Window : pas de fenêtre en web.
func (h *WebHost) Window() bridge.WindowControl { return bridge.NoWindow() }

// QueryGuard : les requêtes s'exécutent sur le serveur ; elles ne lisent
// que les fichiers envoyés et ne joignent pas son réseau interne.
func (h *WebHost) QueryGuard() *query.Guard { return h.guard }

// Platform : version web, scripting selon le serveur, démo publique.
func (h *WebHost) Platform() bridge.Platform {
	return bridge.Platform{Web: true, Scripting: h.scripting, Demo: h.demo}
}
