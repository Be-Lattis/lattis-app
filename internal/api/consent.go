package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/trust"
)

// Accord des scripts (point C2 : sécurité des classeurs reçus).
//
// Un script de la bibliothèque du classeur (action d'un bouton ou d'un
// contrôle, écouteur d'évènement, bouton d'un onglet Document, scripts.run
// par son nom) ne s'exécute que si sa version (empreinte du source) est
// autorisée sur ce poste (internal/trust). Un script enregistré ou lancé
// depuis l'onglet Script de ce poste l'est d'office (doc.SaveScript,
// RunScript) : un classeur créé ici ne demande jamais rien. Sinon (classeur
// reçu, script modifié ailleurs, et plus tard script publié sur un Serveur
// Lattis), la première exécution de chaque version demande « Autoriser /
// Refuser », avec le code à voir. Un refus vaut pour la session ; le serveur
// MCP, qui ne peut pas consulter l'utilisateur, n'exécute que des versions
// déjà autorisées. Le code passé directement à scripts.run (source) n'est
// pas celui du classeur : il n'est pas soumis à l'accord.

// ErrScriptNotAllowed refuse un script dont la version n'est pas autorisée.
var ErrScriptNotAllowed = errors.New("script non autorisé")

// TrustWarning est la mise en garde des accords d'exécution (scripts ici,
// pages web dans l'interface : frontend/src/lib/page/consent.ts).
const TrustWarning = "N'autorisez que si vous faites confiance à la personne qui vous a fourni ce classeur. " +
	"Dans le doute, refusez : le classeur reste utilisable sans ce script."

// askScript sérialise les demandes d'accord : deux écouteurs du même script
// ne posent qu'une question.
var askScript = make(chan struct{}, 1)

// allowScript vérifie que la version src du script name peut s'exécuter,
// en consultant l'utilisateur la première fois si ctx le permet.
func allowScript(ctx context.Context, host Host, d *doc.Document, name, src string) error {
	if name == "" {
		return nil
	}
	st := trust.Default()
	h := trust.Hash(src)
	check := func() (bool, error) {
		if st.ScriptAllowed(h) {
			return true, nil
		}
		if st.ScriptRefused(h) {
			return false, fmt.Errorf("%w : « %s » a été refusé pour cette session", ErrScriptNotAllowed, name)
		}
		if !CanPrompt(ctx) {
			return false, fmt.Errorf("%w : cette version de « %s » n'a pas encore été autorisée sur ce poste ; seul l'utilisateur peut l'autoriser dans Lattis (à sa première exécution, ou en le lançant depuis l'onglet Script)", ErrScriptNotAllowed, name)
		}
		return false, nil
	}
	if ok, err := check(); ok || err != nil {
		return err
	}
	select {
	case askScript <- struct{}{}:
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	defer func() { <-askScript }()
	if ok, err := check(); ok || err != nil { // réponse donnée pendant l'attente
		return err
	}
	vals, ok, err := host.Dialog(ctx, Dialog{
		Title: "Exécuter un script du classeur ?",
		Message: fmt.Sprintf("Le classeur veut exécuter le script « %s », qui n'a encore jamais été autorisé sur ce poste "+
			"(classeur reçu, ou script modifié ailleurs). Un script peut lire et modifier tout le classeur, "+
			"vous poser des questions et, avec votre accord, contacter des sites web.", name),
		Warning: TrustWarning,
		Fields:  []Field{{Name: "all", Label: "Autoriser aussi les autres scripts de ce classeur", Type: "checkbox"}},
		OK:      "Autoriser",
		Cancel:  "Refuser",
		Code:    src,
	})
	if err != nil {
		return err
	}
	if !ok || vals == nil {
		st.RefuseScript(h)
		return fmt.Errorf("%w : « %s » refusé par l'utilisateur", ErrScriptNotAllowed, name)
	}
	if all, _ := vals["all"].(bool); all {
		if err := d.AllowAllScripts(ctx); err != nil {
			return err
		}
	}
	return st.AllowScript(h, name)
}
