package doc

import (
	"context"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/trust"
)

// Accords de l'utilisateur sur ce poste (internal/trust) : versions de
// scripts autorisées et hôtes HTTP du classeur, rangés hors du fichier.

// SaveScript enregistre un script (canvas.Model.SaveScript) ; sa version
// est autorisée sur ce poste, puisqu'elle y a été écrite.
func (d *Document) SaveScript(ctx context.Context, id int64, name, source string) (int64, error) {
	id, err := d.Model.SaveScript(ctx, id, name, source)
	if err != nil {
		return id, err
	}
	return id, trust.Default().AllowScript(trust.Hash(source), name)
}

// AllowAllScripts autorise sur ce poste la version actuelle de chaque script
// du classeur (« Autoriser aussi les autres scripts de ce classeur »).
func (d *Document) AllowAllScripts(ctx context.Context) error {
	list, err := d.Scripts(ctx)
	if err != nil {
		return err
	}
	v := map[string]string{}
	for _, s := range list {
		v[trust.Hash(s.Source)] = s.Name
	}
	return trust.Default().AllowScripts(v)
}

// AddPage ajoute un onglet page HTML (canvas.Model.AddPage) ; le modèle de
// départ est autorisé sur ce poste.
func (d *Document) AddPage(ctx context.Context, before int64) (int64, error) {
	id, err := d.Model.AddPage(ctx, before)
	if err != nil {
		return id, err
	}
	return id, trust.Default().AllowPage(trust.Hash(canvas.PageTemplate), d.sheetName(ctx, id))
}

// SetPage remplace le contenu d'un onglet page HTML ou Document
// (canvas.Model.SetPage) ; la version d'une page HTML est autorisée sur ce
// poste, puisqu'elle y a été écrite.
func (d *Document) SetPage(ctx context.Context, sheet int64, source string) error {
	if err := d.Model.SetPage(ctx, sheet, source); err != nil {
		return err
	}
	if kind, _ := d.PageKind(ctx, sheet); kind != canvas.KindHTML {
		return nil
	}
	return trust.Default().AllowPage(trust.Hash(source), d.sheetName(ctx, sheet))
}

// PageAllowed dit si cette version d'une page web peut s'afficher (son code
// s'exécuter) sur ce poste.
func PageAllowed(source string) bool {
	return trust.Default().PageAllowed(trust.Hash(source))
}

// AllowPage autorise sur ce poste cette version de la page web de l'onglet
// sheet (l'utilisateur a répondu « Autoriser »).
func (d *Document) AllowPage(ctx context.Context, sheet int64, source string) error {
	return trust.Default().AllowPage(trust.Hash(source), d.sheetName(ctx, sheet))
}

// AllowAllPages autorise sur ce poste la version actuelle de chaque page web
// du classeur (reprise d'un ancien classeur de ce poste).
func (d *Document) AllowAllPages(ctx context.Context) error {
	pages, err := d.HTMLPages(ctx)
	if err != nil {
		return err
	}
	v := map[string]string{}
	for name, src := range pages {
		v[trust.Hash(src)] = name
	}
	return trust.Default().AllowPages(v)
}

func (d *Document) sheetName(ctx context.Context, id int64) string {
	var name string
	d.DB().Read().QueryRowContext(ctx, `SELECT name FROM _lattis_sheets WHERE id = ?`, id).Scan(&name)
	return name
}

// HTTPHosts liste les hôtes mémorisés pour ce classeur sur ce poste, par
// ordre alphabétique.
func (d *Document) HTTPHosts(ctx context.Context) ([]string, error) {
	return trust.Default().Hosts(d.SecretScope()), nil
}

// HTTPHostAllowed dit si l'hôte est autorisé : mémorisé pour ce classeur
// sur ce poste, ou accordé pour la session. known est faux si
// l'utilisateur n'a encore rien répondu pour cet hôte.
func (d *Document) HTTPHostAllowed(ctx context.Context, host string) (allowed, known bool, err error) {
	if trust.Default().HostAllowed(d.SecretScope(), host) {
		return true, true, nil
	}
	allowed, known = d.SessionHost(host)
	return allowed, known, nil
}

// AllowHTTPHost mémorise l'hôte pour ce classeur, sur ce poste (le
// classeur n'est pas modifié).
func (d *Document) AllowHTTPHost(ctx context.Context, host string) error {
	host = canvas.NormalizeHost(host)
	if host == "" || strings.ContainsAny(host, "/ \t") {
		return msg.New("doc.invalidHost", nil)
	}
	d.SetSessionHost(host, true)
	return trust.Default().AllowHosts(d.SecretScope(), host)
}

// ForgetHTTPHost retire l'hôte et oublie la réponse donnée pour la
// session : la prochaine requête redemandera.
func (d *Document) ForgetHTTPHost(ctx context.Context, host string) error {
	d.ForgetSessionHost(host)
	return trust.Default().ForgetHost(d.SecretScope(), host)
}

// claimLegacy reprend, une fois, les accords d'un classeur de ce poste
// ouvert avant que les accords ne soient rangés hors des classeurs (récent
// au moment de la mise à jour) : ses scripts et ses pages web actuels sont
// autorisés et ses hôtes enregistrés dans le fichier sont mémorisés sur ce
// poste. Les hôtes
// enregistrés dans un autre classeur (reçu) sont ignorés.
func (d *Document) claimLegacy(ctx context.Context) {
	if !trust.Default().ClaimLegacy(d.Path()) {
		return
	}
	d.AllowAllScripts(ctx)
	d.AllowAllPages(ctx)
	if hosts, err := d.LegacyHTTPHosts(ctx); err == nil && len(hosts) > 0 {
		trust.Default().AllowHosts(d.SecretScope(), hosts...)
	}
}
