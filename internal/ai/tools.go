package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/mcp"
)

// Outils de l'assistant : une sélection des commandes de internal/api (les
// mêmes que les scripts Lua et le serveur MCP), selon le partage des données
// réglé par ai.dataSharing :
//
//   - schema-only (par défaut) : l'assistant voit la structure (feuilles,
//     tableaux, noms et types des colonnes, nombre de lignes, cardinalités)
//     mais aucune valeur des cellules ; il peut construire l'analyse
//     (colonnes, types, clés, formules, tableaux croisés, graphiques) ;
//   - sample : il lit en plus quelques lignes d'exemple (au plus SampleRows)
//     et les valeurs distinctes d'une colonne.
//
// Les commandes qui suppriment un tableau ou une feuille, touchent aux
// fichiers, au réseau, aux scripts ou à la protection ne sont pas proposées,
// ni celles de l'interface (ui.*) : afficher une feuille quitterait l'onglet
// IA pendant la réponse.
// Tout ce que l'assistant modifie pendant une réponse forme une seule étape
// d'annulation (Ctrl+Z), posée par l'appelant (internal/bridge).

// Partage des données.
const (
	SchemaOnly = "schema-only"
	Sample     = "sample"
)

// SampleRows borne les lignes lues en mode sample.
const SampleRows = 20

// sampleValues borne les listes (valeurs distinctes…) en mode sample.
const sampleValues = 30

var structureCommands = []string{
	// Lecture de la structure.
	"sheets.list", "tables.list", "tables.get", "formulas.functions", "queries.list", "connections.list",
	"columns.type_preview", "columns.type_mix", "tables.key_check", "buttons.list",
	// Construction de l'analyse.
	"sheets.add", "sheets.rename", "sheets.color", "tables.rename", "tables.sort",
	"columns.insert", "columns.rename", "columns.set_type", "columns.fix_type", "columns.set_rules",
	"tables.set_key", "cells.set", "cells.style", "cells.name",
	"pivots.create", "pivots.set", "charts.create", "charts.set", "charts.place",
}

var sampleCommands = []string{"tables.rows", "columns.values", "db.query", "sheets.get", "cells.get"}

// profileTool : profil d'un tableau (cardinalités), propre à l'assistant.
const profileTool = "tables_profile"

// commands donne les commandes permises pour ce partage.
func commands(sharing string) map[string]*api.Command {
	out := map[string]*api.Command{}
	add := func(names []string) {
		for _, n := range names {
			if c := api.Lookup(n); c != nil {
				out[mcp.ToolName(n)] = c
			}
		}
	}
	add(structureCommands)
	if sharing == Sample {
		add(sampleCommands)
	}
	return out
}

// Tools décrit les outils de l'assistant pour ce partage.
func Tools(sharing string) []Tool {
	allowed := commands(sharing)
	var out []Tool
	for _, t := range mcp.Tools() {
		if allowed[t.Name] == nil {
			continue
		}
		desc := t.Description
		if sharing == Sample && (t.Name == "tables_rows" || t.Name == "db_query") {
			desc += fmt.Sprintf("\n\nPartage limité : au plus %d lignes sont rendues.", SampleRows)
		}
		out = append(out, Tool{Name: t.Name, Description: desc, Schema: t.InputSchema})
	}
	out = append(out, Tool{
		Name:        profileTool,
		Description: "Profil d'un tableau sans en montrer les valeurs : pour chaque colonne, son type, son nombre de valeurs distinctes et de cellules vides, et le nombre de lignes.",
		Schema: map[string]any{"type": "object", "properties": map[string]any{
			"table": map[string]any{"type": "string", "description": "nom du tableau"},
		}, "required": []string{"table"}},
	})
	return out
}

// Exec rend l'exécuteur des outils pour host, avec ce partage.
func Exec(host api.Host, sharing string) Executor {
	allowed := commands(sharing)
	return func(ctx context.Context, name string, input json.RawMessage) (string, bool) {
		args := map[string]any{}
		if len(input) > 0 {
			if err := json.Unmarshal(input, &args); err != nil {
				return "paramètres illisibles : " + err.Error(), true
			}
		}
		if name == profileTool {
			return profile(ctx, host, args)
		}
		cmd := allowed[name]
		if cmd == nil {
			return "outil inconnu ou non permis : " + name, true
		}
		if cmd.Name == "tables.rows" {
			if n, ok := args["count"].(float64); !ok || n > SampleRows {
				args["count"] = float64(SampleRows)
			}
		}
		// L'assistant ne fait pas répondre l'utilisateur à sa place (boîtes
		// de dialogue, accès réseau).
		res, err := api.Invoke(api.WithoutPrompt(ctx), host, cmd.Name, args)
		if err != nil {
			return err.Error(), true
		}
		return encode(res, sharing == Sample && isSample(cmd.Name)), false
	}
}

func isSample(name string) bool {
	for _, n := range sampleCommands {
		if n == name {
			return true
		}
	}
	return false
}

// encode rend un résultat en JSON ; limit : borne les lignes et les listes.
func encode(v any, limit bool) string {
	switch x := v.(type) {
	case nil:
		return "ok"
	case string:
		if x == "" {
			return "ok"
		}
		return x
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	if !limit {
		return string(b)
	}
	var g any
	if json.Unmarshal(b, &g) != nil {
		return string(b)
	}
	switch x := g.(type) {
	case []any:
		if len(x) > sampleValues {
			g = map[string]any{"items": x[:sampleValues], "truncated": true, "total": len(x)}
		}
	case map[string]any:
		if rows, ok := x["rows"].([]any); ok && len(rows) > SampleRows {
			x["rows"] = rows[:SampleRows]
			x["truncated"] = true
			if ids, ok := x["ids"].([]any); ok && len(ids) > SampleRows {
				x["ids"] = ids[:SampleRows]
			}
		}
	}
	b, _ = json.Marshal(g)
	return string(b)
}

// quoteIdent met un nom affiché entre guillemets doubles (SQL).
func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

// profile calcule les cardinalités d'un tableau par une requête d'agrégat :
// aucune valeur ne quitte le classeur.
func profile(ctx context.Context, host api.Host, args map[string]any) (string, bool) {
	table, _ := args["table"].(string)
	if table == "" {
		return "paramètre table manquant", true
	}
	info, err := api.Invoke(ctx, host, "tables.get", map[string]any{"table": table})
	if err != nil {
		return err.Error(), true
	}
	b, _ := json.Marshal(info)
	var t struct {
		Columns []struct {
			Name    string `json:"name"`
			Type    string `json:"type"`
			Formula string `json:"formula,omitempty"`
		} `json:"columns"`
	}
	if err := json.Unmarshal(b, &t); err != nil || len(t.Columns) == 0 {
		return "colonnes du tableau introuvables", true
	}
	sel := []string{"COUNT(*)"}
	for _, c := range t.Columns {
		q := quoteIdent(c.Name)
		sel = append(sel, "COUNT(DISTINCT "+q+")", "SUM("+q+" IS NULL OR "+q+" = '')")
	}
	res, err := api.Invoke(ctx, host, "db.query", map[string]any{"sql": "SELECT " + strings.Join(sel, ", ") + " FROM " + quoteIdent(table)})
	if err != nil {
		return err.Error(), true
	}
	b, _ = json.Marshal(res)
	var r struct {
		Rows [][]any `json:"rows"`
	}
	if err := json.Unmarshal(b, &r); err != nil || len(r.Rows) != 1 || len(r.Rows[0]) != len(sel) {
		return "profil impossible à calculer", true
	}
	row := r.Rows[0]
	type col struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Formula  string `json:"formula,omitempty"`
		Distinct any    `json:"distinct"`
		Empty    any    `json:"empty"`
	}
	out := struct {
		Table   string `json:"table"`
		Rows    any    `json:"rows"`
		Columns []col  `json:"columns"`
	}{Table: table, Rows: row[0]}
	for i, c := range t.Columns {
		out.Columns = append(out.Columns, col{c.Name, c.Type, c.Formula, row[1+2*i], row[2+2*i]})
	}
	return encode(out, false), false
}

// Label donne le libellé lisible d'un appel d'outil, pour l'interface.
func Label(name string, input json.RawMessage) string {
	var args map[string]any
	_ = json.Unmarshal(input, &args)
	label := ""
	if name == profileTool {
		label = "Profil du tableau"
	} else if c := commandOf(name); c != nil {
		label = c.Label
		if label == "" {
			label = c.Summary
			if i := strings.IndexAny(label, ",.:;("); i > 0 {
				label = label[:i]
			}
		}
	} else {
		label = name
	}
	for _, k := range []string{"table", "sheet", "name"} {
		if v, ok := args[k].(string); ok && v != "" {
			return fmt.Sprintf("%s « %s »", label, v)
		}
	}
	return label
}

func commandOf(tool string) *api.Command {
	for _, c := range api.Commands() {
		if mcp.ToolName(c.Name) == tool {
			return c
		}
	}
	return nil
}

// System est la consigne de l'assistant.
func System(sharing string) string {
	data := "Vous ne voyez pas les valeurs des cellules : seulement la structure (feuilles, tableaux, noms et types des colonnes, nombre de lignes) et les cardinalités (outil tables_profile). Ne demandez pas à l'utilisateur de vous coller ses données ; raisonnez sur la structure."
	if sharing == Sample {
		data = fmt.Sprintf("Vous voyez la structure du classeur et pouvez lire quelques lignes d'exemple (au plus %d par lecture) et les valeurs distinctes d'une colonne ; vous ne voyez jamais tout le contenu.", SampleRows)
	}
	return `Vous êtes l'assistant d'analyse de données de Lattis, un tableur relationnel de bureau : chaque tableau d'une feuille est une table SQLite, avec des colonnes typées (texte, entier, décimal, date, booléen). ` +
		`Vous aidez l'utilisateur à construire une analyse pertinente : choisir les bons types, les clés, les colonnes calculées, les tableaux croisés dynamiques et les graphiques, et à comprendre ses données.

Conventions : les feuilles, tableaux et colonnes se désignent par leur nom ; les lignes et colonnes de la feuille se comptent à partir de 1. Commencez par tables_list (et tables_profile au besoin) pour connaître le classeur avant de proposer quoi que ce soit.
Formules : elles commencent par = et utilisent des fonctions proches d'Excel en français (outil formulas_functions). Dans un tableau, [Colonne] désigne la valeur de la ligne courante, [Colonne][-1] la ligne précédente, Tableau[Colonne] toute la colonne d'un autre tableau, Tableau[Colonne][n] une valeur ; Cell[A1] une cellule fixe de la feuille. Une formule saisie dans une cellule de données d'un tableau fait de toute la colonne une colonne calculée : pour en ajouter une, columns_insert puis cells_set de la formule dans la première ligne de données (position du tableau : tables_get).
` + data + `

Chaque outil qui modifie le classeur s'applique tout de suite ; tout ce que vous modifiez pendant une réponse s'annule d'un coup (Ctrl+Z). Ne supprimez rien que l'utilisateur n'a pas demandé. Quand une demande est ambiguë ou touche beaucoup de données, proposez d'abord ce que vous comptez faire.
Répondez en français, brièvement, en texte simple (listes à puces permises, pas de tableaux Markdown).`
}
