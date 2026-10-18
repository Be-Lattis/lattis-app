// Package instance transmet un lien lattis:// à l'application déjà ouverte
// (contrat de lattisd, §8 : « si l'instance tourne déjà, lui transmettre
// l'adresse »). L'instance qui tourne écoute sur la boucle locale et note
// son port et un jeton dans un fichier du dossier de configuration de
// l'utilisateur ; un second lancement avec un lien lit ce fichier, envoie le
// lien et s'arrête. Un exécutable par fichier : une version de
// développement ne transmet pas à la version installée.
package instance

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type record struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}

// file est le fichier de l'instance de cet exécutable.
var file = func() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.ToLower(exe)))
	return filepath.Join(dir, "Lattis", "instance-"+hex.EncodeToString(sum[:6])+".json"), nil
}

const (
	pathLink = "/link"
	header   = "X-Lattis-Token"
	maxLink  = 8 << 10
)

// Serve fait de ce processus l'instance qui reçoit les liens : onLink est
// appelé (dans une goroutine du serveur) pour chaque lien transmis. stop
// arrête l'écoute et retire le fichier s'il est encore le sien.
func Serve(onLink func(link string)) (stop func(), err error) {
	path, err := file()
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		ln.Close()
		return nil, err
	}
	tok := hex.EncodeToString(b)
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+pathLink, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(header) != tok {
			http.Error(w, "jeton invalide", http.StatusForbidden)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxLink))
		if err != nil || len(body) == 0 {
			http.Error(w, "lien absent", http.StatusBadRequest)
			return
		}
		onLink(string(body))
		w.WriteHeader(http.StatusNoContent)
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(ln)
	rec, _ := json.Marshal(record{Port: ln.Addr().(*net.TCPAddr).Port, Token: tok, PID: os.Getpid()})
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		os.WriteFile(path, rec, 0o600)
	}
	return func() {
		srv.Close()
		if cur, err := os.ReadFile(path); err == nil && bytes.Equal(cur, rec) {
			os.Remove(path)
		}
	}, nil
}

// ErrNoInstance : aucune instance ne tourne (ou elle ne répond pas).
var ErrNoInstance = errors.New("aucune instance de Lattis ouverte")

// Forward transmet link à l'instance ouverte ; ErrNoInstance s'il n'y en a
// pas : ce lancement doit alors ouvrir le lien lui-même.
func Forward(link string) error {
	path, err := file()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ErrNoInstance
	}
	var rec record
	if json.Unmarshal(b, &rec) != nil || rec.Port == 0 {
		return ErrNoInstance
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:"+strconv.Itoa(rec.Port)+pathLink, strings.NewReader(link))
	if err != nil {
		return err
	}
	req.Header.Set(header, rec.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ErrNoInstance
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return ErrNoInstance
	}
	return nil
}
