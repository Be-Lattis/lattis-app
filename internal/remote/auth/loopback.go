package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Redirection en boucle locale (RFC 8252, §7.3) : un petit serveur HTTP
// écoute sur 127.0.0.1, sur un port libre choisi par le système, le temps
// de la connexion ; le navigateur du système y revient avec le code.

// CallbackPath est le chemin de la redirection locale.
const CallbackPath = "/callback"

type callbackResult struct {
	code string
	err  error
}

type loopback struct {
	ln       net.Listener
	srv      *http.Server
	redirect string
	once     sync.Once
	result   chan callbackResult
}

// startLoopback ouvre l'écoute locale ; state est comparé au retour.
func startLoopback(state, server string) (*loopback, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	l := &loopback{
		ln:       ln,
		redirect: fmt.Sprintf("http://127.0.0.1:%d%s", ln.Addr().(*net.TCPAddr).Port, CallbackPath),
		result:   make(chan callbackResult, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			// Pas notre demande (onglet ancien, page tierce) : ignorée.
			page(w, http.StatusBadRequest, "Lien de connexion inconnu", "Cette page ne correspond pas à la connexion en cours dans Lattis. Recommencez depuis Lattis.")
			return
		}
		var res callbackResult
		switch {
		case q.Get("error") != "":
			res.err = newLoginError(q.Get("error"), q.Get("error_description"), q.Get("reason"), server)
			page(w, http.StatusOK, "Connexion refusée", res.err.Error())
		case q.Get("code") != "":
			res.code = q.Get("code")
			page(w, http.StatusOK, "Connexion réussie", "Vous pouvez fermer cet onglet et revenir à Lattis.")
		default:
			page(w, http.StatusBadRequest, "Réponse incomplète", "Le serveur n'a renvoyé ni code ni erreur. Recommencez depuis Lattis.")
			return
		}
		l.once.Do(func() { l.result <- res })
	})
	l.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go l.srv.Serve(ln)
	return l, nil
}

// wait attend le retour du navigateur.
func (l *loopback) wait(ctx context.Context) (string, error) {
	select {
	case r := <-l.result:
		return r.code, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (l *loopback) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	l.srv.Shutdown(ctx)
}

func page(w http.ResponseWriter, status int, title, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html><html lang="fr"><head><meta charset="utf-8"><title>%s – Lattis</title>
<style>body{font:15px 'Segoe UI',system-ui,sans-serif;color:#1f2328;display:flex;justify-content:center;margin-top:18vh}
main{max-width:460px;border:1px solid #d4d7db;border-radius:8px;padding:24px 28px}h1{font-size:18px;color:#1b5e3a;margin:0 0 8px}</style>
</head><body><main><h1>%s</h1><p>%s</p></main></body></html>`, html.EscapeString(title), html.EscapeString(title), html.EscapeString(msg))
}

// ErrBrowser : le navigateur du système n'a pas pu être ouvert.
var ErrBrowser = errors.New("impossible d'ouvrir le navigateur du système")

// OpenBrowser ouvre une adresse http(s) dans le navigateur du système.
func OpenBrowser(u string) error {
	if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		return ErrBrowser
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w : %v", ErrBrowser, err)
	}
	go cmd.Wait()
	return nil
}
