package doc

import (
	"context"
	"errors"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
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
		return errors.New("hôte invalide : nom de domaine (et port éventuel) attendu")
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
// au moment de la mise à jour) : ses scripts actuels sont autorisés et ses
// hôtes enregistrés dans le fichier sont mémorisés sur ce poste. Les hôtes
// enregistrés dans un autre classeur (reçu) sont ignorés.
func (d *Document) claimLegacy(ctx context.Context) {
	if !trust.Default().ClaimLegacy(d.Path()) {
		return
	}
	d.AllowAllScripts(ctx)
	if hosts, err := d.LegacyHTTPHosts(ctx); err == nil && len(hosts) > 0 {
		trust.Default().AllowHosts(d.SecretScope(), hosts...)
	}
}
