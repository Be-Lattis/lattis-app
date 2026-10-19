// Package sharing applique le réglage « Données transmises à l'assistant »
// (ai.dataSharing) à ce qu'un assistant lit du classeur. L'assistant intégré
// (internal/ai) et Claude Desktop ou Claude Code par le serveur MCP
// (internal/mcp) suivent les mêmes règles :
//
//   - schema-only (par défaut) : la structure (feuilles, tableaux, noms et
//     types des colonnes, nombre de lignes, cardinalités), aucune valeur ;
//   - sample : en plus quelques lignes d'exemple, bornées à chaque lecture
//     (SampleRows lignes, SampleText caractères par valeur, SampleBytes en
//     tout) ;
//   - full : tout le classeur, sans borne.
package sharing

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/query"
)

// Partage des données.
const (
	SchemaOnly = "schema-only"
	Sample     = "sample"
	Full       = "full"
)

// Mode rend un partage connu : schema-only pour une valeur absente ou
// inconnue.
func Mode(s string) string {
	switch s {
	case Sample, Full:
		return s
	}
	return SchemaOnly
}

// Bornes du mode sample, pour chaque lecture.
const (
	SampleRows   = 20   // lignes
	SampleValues = 30   // éléments d'une liste (valeurs distinctes…)
	SampleText   = 200  // caractères d'une valeur
	SampleBytes  = 8000 // taille du résultat
)

// Structure : commandes de l'assistant intégré dans tous les modes, qui ne
// rendent pas de valeurs (ou dont Filter retire les valeurs).
var Structure = []string{
	// Lecture de la structure.
	"sheets.list", "sheets.page_setup", "help.search", "tables.list", "tables.get", "formulas.functions", "queries.list", "connections.list",
	"columns.type_preview", "columns.type_mix", "tables.key_check", "tables.indexes", "buttons.list",
	// Disposition des feuilles (où sont tableaux, TCD, graphiques, cellules
	// libres) ; hors partage complet, sans les valeurs (Filter).
	"sheets.get",
	// Construction de l'analyse.
	"sheets.add", "sheets.rename", "sheets.color", "tables.rename", "tables.sort", "tables.sort_custom", "tables.set_sort_lists",
	"columns.insert", "columns.rename", "columns.set_type", "columns.fix_type", "columns.set_rules",
	"tables.set_key", "tables.keep_row_numbers", "cells.set", "cells.style", "cells.name",
	// Mise en forme : styles de lignes et colonnes entières, fusion, largeurs
	// et hauteurs, volets figés, filtres, mise en forme conditionnelle (ci-dessus).
	"cells.style_lines", "cells.merge", "cells.unmerge", "sheets.set_column_width", "sheets.set_row_height", "sheets.freeze", "columns.set_filter",
	// Réparer des tableaux mal créés : limites (tables.to_range puis
	// tables.from_range), en-tête, titre ou note pris dans le tableau, lignes,
	// colonnes à scinder, déplacement.
	"tables.from_range", "tables.to_range", "tables.promote_header", "tables.header_to_title", "tables.last_row_out", "tables.two_level_headers",
	"tables.insert_row", "tables.delete_row", "tables.move", "tables.text_to_columns", "cells.text_to_columns", "cells.paste", "cells.clear",
	"pivots.create", "pivots.set", "charts.create", "charts.set", "charts.place",
	// Demandes courantes : supprimer une colonne, un graphique, un tableau
	// croisé, les doublons ; filtres, notes, lignes masquées, colonnes
	// sorties vers un nouveau tableau, remplacement d'un texte (sans le
	// nombre de cellules modifiées hors partage : Filter) ; dépendances
	// d'une formule (noms et positions) avant de toucher à un tableau.
	"columns.delete", "charts.delete", "pivots.delete", "tables.duplicates", "tables.remove_duplicates",
	"tables.clear_filters", "cells.note", "sheets.hide_lines", "tables.extract_columns", "sheets.replace",
	"cells.dependencies",
	// Onglet à choix (« Choisir par… », « Nouvelle année… », valeur affichée).
	"sheets.choose_by", "sheets.set_choice", "sheets.new_choice", "sheets.clear_choice",
	// Regrouper les onglets aux mêmes colonnes : détection (noms de
	// feuilles, de tableaux et de colonnes) et fusion (sans valeurs hors
	// partage : Filter).
	"tables.similar", "tables.merge_sheets",
}

// Reads : lectures de valeurs, permises à partir du mode sample (bornées
// en sample). sheets.find rend des positions, mais le texte cherché les
// rend parlantes.
var Reads = []string{"tables.rows", "columns.values", "db.query", "cells.get", "cells.read", "sheets.find"}

// values : commandes qui rendent des valeurs du classeur, permettent de les
// retrouver (recherche, script, page) ou de les écrire dans un fichier
// choisi par l'assistant, sans que leur résultat puisse être épuré. Le
// serveur MCP ne les propose qu'en partage complet (Reads aussi en sample).
var values = map[string]bool{
	"tables.rows": true, "columns.values": true, "db.query": true, "cells.get": true, "cells.read": true,
	"queries.preview": true, "sheets.find": true, "pages.get": true,
	// Ordres personnalisés et listes du classeur : des valeurs des colonnes.
	"columns.sort_order": true, "tables.sort_lists": true,
	"scripts.run": true, "buttons.run": true, "controls.fire": true,
	"doc.export_csv": true, "doc.export_xlsx": true, "doc.export_ods": true,

	// Valeurs de la colonne de catégorie d'un onglet à choix.
	"sheets.choice": true,
}

// IsRead dit si une commande est une lecture de valeurs bornée en sample.
func IsRead(cmd string) bool { return slices.Contains(Reads, cmd) }

// Exposed dit si le serveur MCP propose la commande pour ce partage.
func Exposed(cmd, mode string) bool {
	switch {
	case mode == Full || !values[cmd]:
		return true
	case mode == Sample:
		return IsRead(cmd)
	}
	return false
}

// Note est la précision ajoutée à la description d'une lecture bornée.
func Note(cmd, mode string) string {
	if mode == Sample && IsRead(cmd) {
		return fmt.Sprintf("\n\nPartage limité : au plus %d lignes, et %d caractères par valeur, sont rendus.", SampleRows, SampleText)
	}
	return ""
}

// Refused est le message d'un outil que le partage ne permet pas.
func Refused(tool string) string {
	return "outil non permis par le réglage « Données transmises à l'assistant » (Fichier › Paramètres › Assistant IA) : " + tool
}

// Args borne les lignes demandées par une lecture en sample ; args peut
// être nil.
func Args(cmd string, args map[string]any, mode string) map[string]any {
	if args == nil {
		args = map[string]any{}
	}
	if mode != Sample {
		return args
	}
	switch cmd {
	case "tables.rows":
		if n, ok := args["count"].(float64); !ok || n > SampleRows {
			args["count"] = float64(SampleRows)
		}
	case "cells.read":
		r, ok1 := args["row"].(float64)
		r2, ok2 := args["row2"].(float64)
		if ok1 && ok2 && r2-r >= SampleRows {
			args["row2"] = r + SampleRows - 1
		}
	}
	return args
}

// Filter retire d'un résultat ce que le partage ne permet pas de voir.
// Hors partage complet : disposition des feuilles sans contenu des
// cellules, en-têtes HTTP des requêtes masqués. Sans partage des données,
// en plus : colonnes d'un tableau croisé qui portent les valeurs de son
// champ en colonnes, littéraux des étapes des requêtes, valeurs des
// contrôles.
func Filter(cmd string, res any, mode string) any {
	if mode == Full {
		return res
	}
	if cmd == "sheets.replace" && mode == SchemaOnly {
		// Le nombre de cellules modifiées dirait si le texte cherché est
		// dans la feuille.
		return "remplacement fait (le nombre de cellules modifiées n'est pas transmis sans partage des données)"
	}
	switch v := res.(type) {
	case api.SheetView:
		return layoutOnly(v)
	case []api.TableEntry:
		if mode == SchemaOnly {
			return pivotColumns(v)
		}
	case []query.Query:
		return maskQueries(v, mode)
	case canvas.MergeResult:
		if mode == SchemaOnly {
			return maskMerge(v)
		}
	case api.ControlView:
		if mode == SchemaOnly {
			return maskControl(v)
		}
	case []api.ControlView:
		if mode == SchemaOnly {
			out := make([]api.ControlView, len(v))
			for i, c := range v {
				out[i] = maskControl(c)
			}
			return out
		}
	}
	return res
}

// maskMerge : le compte rendu d'un regroupement d'onglets sans valeurs (la
// clé en double donnée en exemple, le texte des formules à reprendre).
func maskMerge(v canvas.MergeResult) canvas.MergeResult {
	if v.KeyNote != "" {
		v.KeyNote = "Pas de clé : une même valeur de clé est dans plusieurs onglets"
	}
	pending := make([]canvas.PendingRef, len(v.Pending))
	for i, p := range v.Pending {
		p.Formula = masked
		pending[i] = p
	}
	v.Pending = pending
	notes := make([]string, 0, len(v.Notes))
	for _, n := range v.Notes {
		if strings.HasPrefix(n, "Pas de clé") {
			n = v.KeyNote
		}
		notes = append(notes, n)
	}
	v.Notes = notes
	if k := strings.Index(v.Notice, " ; Pas de clé"); k >= 0 {
		rest := v.Notice[k+3:]
		end := strings.Index(rest, " ; ")
		tail := ""
		if end >= 0 {
			tail = rest[end:]
		}
		v.Notice = v.Notice[:k] + " ; " + v.KeyNote + tail
	}
	return v
}

// layoutOnly : la disposition d'une feuille (sheets.get) garde les
// positions et la nature des cellules libres, pas leur contenu ; tableaux
// croisés et graphiques sans leur configuration (filtres : des valeurs).
func layoutOnly(v api.SheetView) api.SheetView {
	cells := make([]api.CellView, len(v.Cells))
	for i, c := range v.Cells {
		kind := "texte"
		switch {
		case strings.HasPrefix(c.Raw, "="):
			kind = "formule"
		case c.Raw != "" && strings.Trim(c.Raw, "0123456789,.- ") == "":
			kind = "nombre"
		}
		cells[i] = api.CellView{Row: c.Row, Col: c.Col, Raw: "(" + kind + ")", Error: c.Error}
	}
	v.Cells = cells
	strip := func(objs []api.ObjectView) []api.ObjectView {
		out := make([]api.ObjectView, len(objs))
		for i, o := range objs {
			o.Config = nil
			out[i] = o
		}
		return out
	}
	v.Pivots, v.Charts = strip(v.Pivots), strip(v.Charts)
	return v
}

// pivotColumns remplace les colonnes d'un tableau croisé tirées des
// valeurs de son champ en colonnes (« 2024 », « Dupont SA »…) par
// « N colonne(s) de valeurs du champ « X » ».
func pivotColumns(list []api.TableEntry) []api.TableEntry {
	out := make([]api.TableEntry, len(list))
	for i, t := range list {
		if k := t.ColsFrom - 1; t.ColsField != "" && k >= 0 && k < len(t.Columns) {
			t.Columns = append(slices.Clone(t.Columns[:k]), fmt.Sprintf("%d colonne(s) de valeurs du champ « %s »", len(t.Columns)-k, t.ColsField))
			if k < len(t.Types) {
				t.Types = append(slices.Clone(t.Types[:k]), t.Types[k])
			}
		}
		out[i] = t
	}
	return out
}

// ValueColumns donne, en minuscules, les noms des colonnes de tableaux
// croisés tirés des valeurs d'un champ (que pivotColumns masque).
func ValueColumns(list []api.TableEntry) map[string]bool {
	out := map[string]bool{}
	for _, t := range list {
		if k := t.ColsFrom - 1; t.ColsField != "" && k >= 0 {
			for _, c := range t.Columns[min(k, len(t.Columns)):] {
				out[strings.ToLower(c)] = true
			}
		}
	}
	return out
}

const masked = "(masqué)"

// maskQueries masque les valeurs des en-têtes HTTP et des paramètres
// d'adresse (des jetons), et sans partage des données les littéraux des
// étapes (filtres, remplacements).
func maskQueries(qs []query.Query, mode string) []query.Query {
	out := make([]query.Query, len(qs))
	for i, q := range qs {
		q.Source = maskSource(q.Source)
		steps := make([]query.Step, len(q.Steps))
		for j, s := range q.Steps {
			if s.Other != nil {
				o := maskSource(*s.Other)
				s.Other = &o
			}
			if mode == SchemaOnly {
				conds := make([]query.Cond, len(s.Conditions))
				for k, c := range s.Conditions {
					if c.Value != "" {
						c.Value = masked
					}
					conds[k] = c
				}
				if s.Conditions != nil {
					s.Conditions = conds
				}
				if s.Value != "" {
					s.Value = masked
				}
				if s.With != "" {
					s.With = masked
				}
			}
			steps[j] = s
		}
		if q.Steps != nil {
			q.Steps = steps
		}
		out[i] = q
	}
	return out
}

func maskSource(s query.Source) query.Source {
	if s.Headers != nil {
		h := make([]query.Header, len(s.Headers))
		for i, x := range s.Headers {
			h[i] = query.Header{Name: x.Name, Value: masked}
		}
		s.Headers = h
	}
	if base, q, ok := strings.Cut(s.URL, "?"); ok {
		parts := strings.Split(q, "&")
		for i, p := range parts {
			if k, _, ok := strings.Cut(p, "="); ok {
				parts[i] = k + "=" + masked
			}
		}
		s.URL = base + "?" + strings.Join(parts, "&")
	}
	return s
}

// maskControl retire la valeur d'un contrôle (saisie, case, liste) et les
// choix d'une liste tirés d'une colonne.
func maskControl(c api.ControlView) api.ControlView {
	c.Value = nil
	if c.OptionsFrom != "" {
		c.Options = nil
	}
	return c
}

// Encode rend en JSON une lecture de valeurs du mode sample, bornée :
// SampleRows lignes, SampleValues éléments d'une liste, SampleText
// caractères par valeur, SampleBytes en tout.
func Encode(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return clipText(fmt.Sprint(v), SampleBytes)
	}
	var g any
	if json.Unmarshal(b, &g) != nil {
		return clipText(string(b), SampleBytes)
	}
	switch x := g.(type) {
	case []any:
		n := SampleValues
		if len(x) > 0 {
			if _, rows := x[0].([]any); rows {
				n = SampleRows // cells.read : des lignes
			}
		}
		if len(x) > n {
			g = map[string]any{"items": x[:n], "truncated": true, "total": len(x)}
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
	b, _ = json.Marshal(clip(g))
	if len(b) > SampleBytes {
		return clipText(string(b), SampleBytes) + fmt.Sprintf("\n[résultat tronqué : partage limité à %d caractères par lecture]", SampleBytes)
	}
	return string(b)
}

// clip borne les chaînes d'un résultat décodé.
func clip(v any) any {
	switch x := v.(type) {
	case string:
		if utf8.RuneCountInString(x) > SampleText {
			return string([]rune(x)[:SampleText]) + "…"
		}
	case []any:
		for i := range x {
			x[i] = clip(x[i])
		}
	case map[string]any:
		for k, e := range x {
			x[k] = clip(e)
		}
	}
	return v
}

// clipText coupe s à n octets au plus, sur une limite de caractère.
func clipText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
