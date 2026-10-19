package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/api"
)

// Suivi des appels d'outils : l'application (internal/bridge) voit passer
// chaque tools/call pour l'afficher (fil d'activité), demander l'accord de
// l'utilisateur avant une écriture et réunir les écritures d'une série
// d'appels en une étape d'annulation.

// Call décrit un appel d'outil, vu par Server.Watch.
type Call struct {
	// Client : nom du client (clientInfo.name de initialize : « claude-ai »
	// pour Claude Desktop, « claude-code »…), "" s'il est inconnu.
	Client  string
	Tool    string // nom MCP (« sheets_add »)
	Command string // commande de internal/api (« sheets.add »)
	Label   string // libellé lisible (« Ajouter une feuille « Ventes » »)
	// Write : l'outil peut modifier le classeur (commande d'écriture, script,
	// bouton, annulation, ouverture d'un autre classeur).
	Write bool
	// Target : feuille, tableau ou nom visé quand l'outil en a un (« Aller à »).
	Target string
	Args   map[string]any
}

// Watcher entoure l'exécution d'un outil : run l'exécute avec le contexte
// donné (l'application y ajoute son groupe d'annulation) ; une erreur rendue
// devient un résultat d'outil en erreur.
type Watcher func(ctx context.Context, c Call, run func(context.Context) (any, error)) (any, error)

// Writes dit si un appel peut modifier le classeur ou écrire un fichier :
// commandes d'écriture, mais aussi scripts et boutons (qui écrivent par
// leurs propres commandes), annuler, rétablir, nouveau classeur, ouverture,
// enregistrement sous un autre nom et exports. C'est le contraire de
// l'annotation readOnlyHint, hors requêtes HTTP et enregistrement simple.
func Writes(c *api.Command, args map[string]any) bool {
	if c.Write {
		return true
	}
	switch c.Name {
	case "scripts.run", "buttons.run", "doc.undo", "doc.redo", "doc.new", "doc.open",
		"doc.export_csv", "doc.export_xlsx", "doc.export_ods":
		return true
	case "doc.save":
		p, _ := args["path"].(string)
		return p != ""
	}
	return false
}

// Label est le libellé lisible d'un appel (même règle que l'assistant
// intégré, internal/ai : nom de l'étape d'annulation, sinon début du
// résumé, suivi de la feuille, du tableau ou du nom visé).
func Label(c *api.Command, args map[string]any) string {
	label := c.Label
	if label == "" {
		label = c.Summary
		if i := strings.IndexAny(label, ",.:;("); i > 0 {
			label = label[:i]
		}
	}
	if t := Target(args); t != "" {
		return fmt.Sprintf("%s « %s »", label, t)
	}
	return label
}

// Target est la feuille, le tableau ou le nom visé par un appel ("" sinon).
func Target(args map[string]any) string {
	for _, k := range []string{"table", "sheet", "name"} {
		if v, ok := args[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// ClientLabel est le nom affiché d'un client MCP : les clients d'Anthropic
// sont nommés comme dans leurs propres fenêtres.
func ClientLabel(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "":
		return "Client MCP"
	case "claude-ai", "claude desktop", "claude-desktop":
		return "Claude Desktop"
	case "claude-code", "claude code":
		return "Claude Code"
	}
	return name
}

type clientKey struct{}

// WithClient attache à ctx le nom du client qui envoie le message.
func WithClient(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, clientKey{}, name)
}

// ClientOf rend le nom du client attaché à ctx ("" sinon).
func ClientOf(ctx context.Context) string {
	s, _ := ctx.Value(clientKey{}).(string)
	return s
}

// ClientHeader porte le nom du client quand le relais stdio (« lattis
// mcp ») transmet ses messages : chaque requête HTTP du relais est isolée,
// le serveur ne verrait pas sinon qui l'envoie.
const ClientHeader = "X-Lattis-Client"
