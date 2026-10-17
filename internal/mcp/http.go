package mcp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultPort est le port d'écoute proposé (un autre est pris s'il est occupé).
const DefaultPort = 47390

// Settings sont les réglages du serveur, gardés avec les données de
// l'application ; « lattis mcp » y lit le port et le jeton.
type Settings struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
}

// settingsFile est le fichier des réglages ; surchargeable pour les tests.
var settingsFile = func() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Lattis", "mcp.json"), nil
}

// LoadSettings lit les réglages (désactivé par défaut) et crée le jeton s'il
// manque.
func LoadSettings() (Settings, error) {
	s := Settings{Port: DefaultPort}
	p, err := settingsFile()
	if err != nil {
		return s, err
	}
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	if s.Port <= 0 || s.Port > 65535 {
		s.Port = DefaultPort
	}
	if s.Token == "" {
		s.Token = newToken()
		err = SaveSettings(s)
	}
	return s, err
}

// SaveSettings enregistre les réglages (lisibles par l'utilisateur seul).
func SaveSettings(s Settings) error {
	p, err := settingsFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(p, b, 0o600)
}

func newToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Listener est le serveur HTTP en cours.
type Listener struct {
	srv  *http.Server
	Port int
}

// Listen démarre le serveur sur 127.0.0.1:port (un port libre si celui-ci
// est pris ; Port donne celui retenu).
func Listen(s *Server, port int, token string) (*Listener, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		if ln, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			return nil, err
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", Handler(s, token))
	l := &Listener{srv: &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}, Port: ln.Addr().(*net.TCPAddr).Port}
	go l.srv.Serve(ln)
	return l, nil
}

// Close arrête le serveur.
func (l *Listener) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return l.srv.Shutdown(ctx)
}

// maxBody borne une requête.
const maxBody = 16 << 20

// Handler sert le point d'accès MCP (POST /mcp).
func Handler(s *Server, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && !localOrigin(o) {
			http.Error(w, "origine refusée", http.StatusForbidden)
			return
		}
		auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(auth), []byte(token)) != 1 {
			http.Error(w, "jeton manquant ou invalide", http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodPost:
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK) // fin de session : rien à libérer
			return
		default:
			w.Header().Set("Allow", "POST")
			http.Error(w, "POST seulement", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		out := s.Handle(r.Context(), body)
		if out == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	})
}

// localOrigin accepte les origines locales (outils de développement).
func localOrigin(o string) bool {
	u, err := url.Parse(o)
	if err != nil {
		return false
	}
	h := u.Hostname()
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

// URL est l'adresse du point d'accès.
func URL(port int) string { return fmt.Sprintf("http://127.0.0.1:%d/mcp", port) }

// --- Relais stdio -------------------------------------------------------------

// ErrNotRunning : Lattis n'est pas ouvert, ou son serveur est désactivé.
var ErrNotRunning = errors.New("Lattis n'est pas ouvert, ou son serveur MCP est désactivé (Fichier › Serveur MCP)")

// Proxy relaie les messages MCP lus sur in (un par ligne) vers l'application
// ouverte et écrit ses réponses sur out : c'est la commande « lattis mcp »
// que lancent les clients stdio.
func Proxy(in io.Reader, out io.Writer) error {
	client := &http.Client{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	write := func(b []byte) {
		mu.Lock()
		defer mu.Unlock()
		out.Write(append(bytes.TrimSpace(b), '\n'))
	}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 64<<10), maxBody)
	for sc.Scan() {
		line := append([]byte(nil), sc.Bytes()...)
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		wg.Add(1)
		go func() { // une boîte de dialogue peut attendre l'utilisateur : ne pas bloquer le reste
			defer wg.Done()
			resp, err := forward(client, line)
			if err != nil {
				if r := errorReply(line, err); r != nil {
					write(r)
				}
				return
			}
			if resp != nil {
				write(resp)
			}
		}()
	}
	wg.Wait()
	return sc.Err()
}

func forward(client *http.Client, msg []byte) ([]byte, error) {
	s, err := LoadSettings()
	if err != nil {
		return nil, err
	}
	if !s.Enabled {
		return nil, ErrNotRunning
	}
	req, err := http.NewRequest(http.MethodPost, URL(s.Port), bytes.NewReader(msg))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+s.Token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrNotRunning
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusAccepted {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Lattis : %s", strings.TrimSpace(string(body)))
	}
	return body, nil
}

// errorReply répond en erreur à une requête que l'application n'a pas pu
// recevoir (rien pour une notification).
func errorReply(msg []byte, err error) []byte {
	var req request
	if json.Unmarshal(msg, &req) != nil || req.ID == nil {
		return nil
	}
	if req.Method == "initialize" {
		// Le client garde la connexion : il pourra réessayer une fois
		// l'application ouverte.
		return encode(response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{
			"protocolVersion": versions[0],
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "lattis", "version": ""},
			"instructions":    err.Error(),
		}})
	}
	if req.Method == "tools/list" {
		return encode(response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": Tools()}})
	}
	if req.Method == "tools/call" {
		return encode(response{JSONRPC: "2.0", ID: req.ID, Result: toolResult(err.Error(), true)})
	}
	return encode(response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{codeInternal, err.Error()}})
}
