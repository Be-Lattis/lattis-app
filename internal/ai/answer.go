package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/sharing"
)

// Répondre à une question sans toucher au classeur : l'assistant écrit une
// requête de lecture (query_show), Lattis l'exécute sur le poste et montre
// le résultat à l'utilisateur dans la conversation (EventResult). En
// partage « structure seulement », le modèle n'en reçoit que la forme
// (colonnes, nombre de lignes), jamais les valeurs. La structure du
// classeur est jointe au premier message (Structure) : plus besoin
// d'explorer avant de répondre.

const showTool = "query_show"

// ShowRows borne les lignes montrées à l'utilisateur.
const ShowRows = 50

// MaxBlindShows borne les requêtes query_show d'une réponse sans partage
// des données.
const MaxBlindShows = 10

// EventResult : résultat d'une requête montré à l'utilisateur.
const EventResult = "result"

// Result est un résultat montré dans la conversation.
type Result struct {
	Title     string     `json:"title,omitempty"`
	Columns   []string   `json:"columns"`
	Rows      [][]string `json:"rows"`
	Total     int        `json:"total"`
	Truncated bool       `json:"truncated,omitempty"`
	// More : la requête a plus de lignes que la limite de lecture
	// (api.QueryLimit) ; Total est un minimum.
	More bool `json:"more,omitempty"`
}

type showKey struct{}

// WithShow fait montrer à l'utilisateur, par show, les résultats des
// requêtes de l'assistant (query_show).
func WithShow(ctx context.Context, show func(Result)) context.Context {
	return context.WithValue(ctx, showKey{}, show)
}

var showToolDef = Tool{
	Name: showTool,
	Description: "Répond à une question sur les données sans modifier le classeur : exécute une requête SQL de lecture (SELECT ou WITH, noms affichés des tableaux et des colonnes entre guillemets doubles, colonnes calculées comprises) et montre son résultat à l'utilisateur, sous votre réponse (au plus " + fmt.Sprint(ShowRows) + " lignes). " +
		"Exemple : SELECT \"Ville\", \"CA HT\" FROM \"Villes\" ORDER BY \"CA HT\" DESC LIMIT 1. " +
		"Préférez des requêtes courtes et triées (ORDER BY, LIMIT) qui répondent directement.",
	Schema: map[string]any{"type": "object", "properties": map[string]any{
		"sql":   map[string]any{"type": "string", "description": "requête SELECT ou WITH"},
		"title": map[string]any{"type": "string", "description": "titre court du résultat (« Meilleure ville par CA »)"},
	}, "required": []string{"sql"}},
}

// show exécute la requête, la montre à l'utilisateur et rend au modèle ce
// que le partage permet.
func show(ctx context.Context, host api.Host, args map[string]any, mode string) (string, bool) {
	q, _ := args["sql"].(string)
	title, _ := args["title"].(string)
	head := strings.ToUpper(strings.TrimSpace(q))
	if !strings.HasPrefix(head, "SELECT") && !strings.HasPrefix(head, "WITH") {
		return "query_show n'accepte qu'une requête de lecture (SELECT ou WITH)", true
	}
	res, err := api.Invoke(api.WithoutPrompt(ctx), host, "db.query", map[string]any{"sql": q})
	if err != nil {
		return err.Error(), true
	}
	b, err := json.Marshal(res)
	if err != nil {
		return err.Error(), true
	}
	var raw struct {
		Columns   []string `json:"columns"`
		Rows      [][]any  `json:"rows"`
		Truncated bool     `json:"truncated"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err.Error(), true
	}
	r := Result{Title: title, Columns: raw.Columns, Rows: [][]string{}, Total: len(raw.Rows), Truncated: raw.Truncated || len(raw.Rows) > ShowRows, More: raw.Truncated}
	for i, row := range raw.Rows {
		if i == ShowRows {
			break
		}
		cells := make([]string, len(row))
		for j, v := range row {
			if v != nil {
				cells[j] = fmt.Sprint(v)
			}
		}
		r.Rows = append(r.Rows, cells)
	}
	if f, ok := ctx.Value(showKey{}).(func(Result)); ok {
		f(r)
	}
	// Total plafonné : « plus de 10 000 », pas « 10000 ».
	var total any = r.Total
	if r.More {
		total = fmt.Sprintf("plus de %d", r.Total)
	}
	switch mode {
	case Sample:
		return sharing.Encode(map[string]any{"shown": "à l'utilisateur, sous votre réponse", "columns": r.Columns, "rows": r.Rows, "total": total}), false
	case Full:
		return encode(map[string]any{"shown": "à l'utilisateur, sous votre réponse", "columns": r.Columns, "rows": r.Rows, "total": total, "truncated": r.Truncated}), false
	}
	// Les colonnes d'un TCD tirées des valeurs de son champ en colonnes
	// (SELECT * FROM "TCD1") sont des valeurs.
	cols := r.Columns
	if list, err := api.Invoke(api.WithoutPrompt(ctx), host, "tables.list", nil); err == nil {
		if l, ok := list.([]api.TableEntry); ok {
			hide := sharing.ValueColumns(l)
			cols = make([]string, len(r.Columns))
			for i, c := range r.Columns {
				cols[i] = c
				if hide[strings.ToLower(c)] {
					cols[i] = "(colonne de valeurs d'un tableau croisé)"
				}
			}
		}
	}
	// Ni valeurs ni nombre de lignes : un nombre de lignes répond déjà à
	// une question sur les valeurs (WHERE substr("Email", 1, 1) = 'a').
	return fmt.Sprintf("Résultat montré à l'utilisateur, colonnes %s. Vous n'en voyez ni les valeurs ni le nombre de lignes : dites en une phrase ce que montre le résultat, sans inventer de valeur.", strings.Join(cols, ", ")), false
}

const (
	structureHead    = "[Structure du classeur (jointe par Lattis)"
	structureChanged = "[Structure modifiée depuis le dernier message (jointe par Lattis, elle remplace la précédente)"
)

// StructureUpdate rend la structure st (Structure) à joindre à un message
// quand elle a changé hors des réponses de l'assistant (import, annulation,
// conversation reprise) ; st vide : le classeur n'a plus de tableau.
func StructureUpdate(st string) string {
	if st == "" {
		return structureChanged + " : le classeur n'a plus de tableau.]\n"
	}
	return strings.Replace(st, structureHead, structureChanged, 1)
}

// Structure décrit le classeur pour le premier message d'une conversation
// (digest, formules en français : StructureIn) ; vide s'il n'y a pas de
// tableau ou si la lecture échoue.
func Structure(ctx context.Context, host api.Host, mode string) string {
	return StructureIn(ctx, host, mode, "fr")
}

// thinStructure : une ligne par tableau (feuille, colonnes et leurs types,
// nombre de lignes), quand le digest ne peut pas se construire.
func thinStructure(ctx context.Context, host api.Host, mode string) string {
	res, err := api.Invoke(api.WithoutPrompt(ctx), host, "tables.list", map[string]any{})
	if err != nil {
		return ""
	}
	b, _ := json.Marshal(sharing.Filter("tables.list", res, sharing.Mode(mode)))
	var list []struct {
		Name    string   `json:"name"`
		Kind    string   `json:"kind"`
		Sheet   string   `json:"sheet"`
		Rows    int      `json:"rows"`
		Columns []string `json:"columns"`
		Types   []string `json:"types"`
	}
	if json.Unmarshal(b, &list) != nil || len(list) == 0 {
		return ""
	}
	var s strings.Builder
	s.WriteString(structureHead + " :\n")
	for i, t := range list {
		if i == 40 {
			fmt.Fprintf(&s, "- … et %d autre(s) : tables_list\n", len(list)-i)
			break
		}
		kind := "tableau"
		if t.Kind == "pivot" {
			kind = "tableau croisé"
		}
		cols := make([]string, 0, len(t.Columns))
		for j, c := range t.Columns {
			if j == 60 {
				cols = append(cols, "…")
				break
			}
			c = oneLine(c)
			if j < len(t.Types) && t.Types[j] != "" {
				c += " (" + t.Types[j] + ")"
			}
			cols = append(cols, c)
		}
		fmt.Fprintf(&s, "- %s « %s », feuille « %s », %d ligne(s) : %s\n", kind, oneLine(t.Name), oneLine(t.Sheet), t.Rows, strings.Join(cols, ", "))
	}
	s.WriteString("]\n")
	return s.String()
}
