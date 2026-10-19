package api

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/trust"
)

// Accord des sources Power Query (audit 6, I4). Une requête est écrite
// dans le classeur : comme un script ou une page, un classeur reçu ne
// joint un hôte ou ne lit un fichier du poste qu'avec l'accord de
// l'utilisateur. Les hôtes suivent la règle des scripts (même liste,
// onglet SCRIPT › Accès réseau) ; les fichiers sont accordés par classeur
// sur ce poste (internal/trust). Une requête enregistrée depuis l'éditeur
// de ce poste accorde d'office ses sources (GrantSource) : seul un
// classeur reçu, ou une source modifiée ailleurs, demande.

// AllowSource vérifie qu'une source du classeur d peut faire l'accès a,
// en consultant l'utilisateur par h la première fois si ctx le permet.
func AllowSource(ctx context.Context, h Host, d *doc.Document, a query.Access) error {
	c := &Call{Context: ctx, Host: h, doc: d}
	if a.Path != "" {
		return c.allowFile(a.Path)
	}
	if a.Host == "" {
		return nil
	}
	return c.allowHostFor(a.Host, "Une requête de ce classeur veut lire des données sur « %s ». Ne l'autorisez que si vous faites confiance au classeur et à ce serveur : il apprendra que vous ouvrez ce classeur, et l'adresse de la requête peut lui transmettre des informations.")
}

// queryEnv : environnement des requêtes lancées par un script ou le serveur
// MCP ; leurs sources demandent l'accord comme celles de l'interface.
func (c *Call) queryEnv(d *doc.Document) query.Env {
	return query.Env{Model: d.Model, Workbook: d.SecretScope(),
		Allow: func(ctx context.Context, a query.Access) error { return AllowSource(ctx, c.Host, d, a) }}
}

// GrantSource accorde l'accès a aux requêtes du classeur d (requête écrite
// sur ce poste).
func GrantSource(ctx context.Context, d *doc.Document, a query.Access) error {
	switch {
	case a.Path != "":
		return trust.Default().AllowFiles(d.SecretScope(), a.Path)
	case a.Host != "":
		return d.AllowHTTPHost(ctx, a.Host)
	}
	return nil
}

// fileSession retient les réponses données pour la session (classeur →
// fichier → autorisé).
var (
	fileMu      sync.Mutex
	fileSession = map[string]map[string]bool{}
)

func fileKey(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		p = a
	}
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

func (c *Call) allowFile(path string) error {
	d, err := c.Doc()
	if err != nil {
		return err
	}
	scope, key := d.SecretScope(), fileKey(path)
	sess := fmt.Sprintf("%s|%p", scope, d) // réponses du classeur ouvert
	check := func() (bool, error) {
		if trust.Default().FileAllowed(scope, path) {
			return true, nil
		}
		fileMu.Lock()
		ok, known := fileSession[sess][key]
		fileMu.Unlock()
		if ok {
			return true, nil
		}
		if known {
			return false, fmt.Errorf("%w : la lecture de « %s » a été refusée pour cette session (rouvrez le classeur pour qu'elle soit redemandée)", query.ErrDenied, path)
		}
		if !CanPrompt(c) {
			return false, fmt.Errorf("%w : « %s » n'est pas autorisé pour ce classeur ; seul l'utilisateur peut l'autoriser dans Lattis (en actualisant la requête)", query.ErrDenied, path)
		}
		return false, nil
	}
	if ok, err := check(); ok || err != nil {
		return err
	}
	select {
	case askHost <- struct{}{}:
	case <-c.Done():
		return context.Cause(c)
	}
	defer func() { <-askHost }()
	if ok, err := check(); ok || err != nil {
		return err
	}
	vals, err := c.dialog(Dialog{
		Title:   "Lecture d'un fichier",
		Message: fmt.Sprintf("Une requête de ce classeur veut lire le fichier « %s ». Ne l'autorisez que si vous attendiez cette source : son contenu sera copié dans le classeur, et partira avec lui si vous l'envoyez.", path),
		Fields:  []Field{{Name: "remember", Label: "Mémoriser pour ce classeur", Type: "checkbox"}},
		OK:      "Autoriser",
		Cancel:  "Refuser",
	})
	if err != nil {
		return err
	}
	remember, _ := vals["remember"].(bool)
	fileMu.Lock()
	if fileSession[sess] == nil {
		fileSession[sess] = map[string]bool{}
	}
	fileSession[sess][key] = vals != nil
	fileMu.Unlock()
	if vals == nil {
		return fmt.Errorf("%w : lecture de « %s » refusée par l'utilisateur", query.ErrDenied, path)
	}
	if remember {
		return trust.Default().AllowFiles(scope, path)
	}
	return nil
}
