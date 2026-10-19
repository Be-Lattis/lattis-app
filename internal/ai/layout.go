package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/api"
)

// Disposition d'une feuille sans aucune valeur (sheets_layout), permise
// dans tous les partages : de quoi repérer des tableaux mal créés (titre
// pris pour l'en-tête, en-tête resté dans les données, total ou note pris
// comme dernière ligne, cellules libres qui devraient être un tableau) et
// les réparer sans lire le contenu des cellules. Chaque cellule est réduite
// à sa nature (texte, nombre, date, booléen, formule, erreur, vide) ; seuls
// deux indices tirés du texte sont rendus : « titre » et « total ».

const layoutTool = "sheets_layout"

// layoutCells borne les cellules libres rendues.
const layoutCells = 300

var layoutToolDef = Tool{
	Name: layoutTool,
	Description: "Disposition d'une feuille, sans aucune valeur, pour repérer des tableaux mal créés avant de les réparer : " +
		"pour chaque tableau, sa ligne d'en-tête et sa colonne (à partir de 1), son nombre de lignes, ses colonnes, la nature des cellules de sa première et de sa dernière ligne de données (texte, nombre, date, booléen, formule, erreur, vide) et des indices (« en-tête » : la première ligne ressemble à des en-têtes ; « total » : la dernière ligne ressemble à un total ; « noms automatiques » : en-têtes Colonne1, Colonne2… inventés par Lattis) ; " +
		"pour chaque cellule libre, sa position, sa nature, gras et fusion, et l'indice « titre » ou « total » ; tableaux croisés, graphiques et boutons par leur position.",
	Schema: map[string]any{"type": "object", "properties": map[string]any{
		"sheet": map[string]any{"type": "string", "description": "nom de la feuille (absent : la feuille active)"},
	}},
}

var (
	totalWord = regexp.MustCompile(`^((sous[- ]?|grand )?tota(l|ux)|somme|moyenne|ensemble|cumul)\b`)
	dateLike  = regexp.MustCompile(`^\d{1,4}[/.-]\d{1,2}[/.-]\d{1,4}$`)
	autoName  = regexp.MustCompile(`^(Colonne|Column)\d+$`)
)

// kindOf réduit une valeur à sa nature.
func kindOf(v any) string {
	switch x := v.(type) {
	case nil:
		return "vide"
	case bool:
		return "booléen"
	case float64, int, int64:
		return "nombre"
	case string:
		t := strings.TrimSpace(x)
		switch {
		case t == "":
			return "vide"
		case strings.HasPrefix(t, "="):
			return "formule"
		case strings.HasPrefix(t, "#") && (strings.HasSuffix(t, "!") || strings.HasSuffix(t, "?") || t == "#N/A"):
			return "erreur"
		case t == "VRAI" || t == "FAUX":
			return "booléen"
		case dateLike.MatchString(t):
			return "date"
		}
		n := strings.NewReplacer(" ", "", " ", "", " ", "", ",", ".", "€", "", "%", "", "$", "").Replace(t)
		if _, err := strconv.ParseFloat(n, 64); err == nil {
			return "nombre"
		}
		return "texte"
	}
	return "texte"
}

// isTotal : le texte ressemble au libellé d'une ligne de total.
func isTotal(v any) bool {
	s, ok := v.(string)
	return ok && totalWord.MatchString(strings.ToLower(strings.TrimSpace(s)))
}

type layoutTable struct {
	Name     string   `json:"name"`
	Row      int      `json:"row"`
	Col      int      `json:"col"`
	Rows     int      `json:"rows"`
	Columns  []string `json:"columns"`
	Types    []string `json:"types"`
	First    []string `json:"firstRow,omitempty"`
	Last     []string `json:"lastRow,omitempty"`
	Hints    []string `json:"hints,omitempty"`
	TwoLevel bool     `json:"twoLevelHeaders,omitempty"`
}

type layoutCell struct {
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Kind  string `json:"kind"`
	Bold  bool   `json:"bold,omitempty"`
	Merge string `json:"merge,omitempty"` // « 1×4 » : lignes × colonnes
	Hint  string `json:"hint,omitempty"`
}

type layoutObject struct {
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
	Row  int    `json:"row"`
	Col  int    `json:"col"`
}

// layout rend la disposition d'une feuille, sans valeur.
func layout(ctx context.Context, host api.Host, args map[string]any) (string, bool) {
	ctx = api.WithoutPrompt(ctx)
	get := map[string]any{}
	if s, _ := args["sheet"].(string); s != "" {
		get["sheet"] = s
	}
	res, err := api.Invoke(ctx, host, "sheets.get", get)
	if err != nil {
		return err.Error(), true
	}
	v, ok := res.(api.SheetView)
	if !ok {
		return "disposition illisible", true
	}
	out := struct {
		Sheet     string         `json:"sheet"`
		Tables    []layoutTable  `json:"tables"`
		Cells     []layoutCell   `json:"cells"`
		Objects   []layoutObject `json:"objects,omitempty"`
		Truncated bool           `json:"truncated,omitempty"`
	}{Sheet: v.Name, Tables: []layoutTable{}, Cells: []layoutCell{}}

	for _, t := range v.Tables {
		lt := layoutTable{Name: t.Name, Row: t.Row, Col: t.Col, Rows: t.Rows, TwoLevel: t.TwoLevel}
		auto, typed := 0, false
		for _, c := range t.Columns {
			lt.Columns = append(lt.Columns, c.Name)
			lt.Types = append(lt.Types, c.Type)
			if autoName.MatchString(c.Name) {
				auto++
			}
			if c.Type != "text" && c.Formula == "" {
				typed = true
			}
		}
		if auto > 0 {
			lt.Hints = append(lt.Hints, "noms automatiques")
		}
		first, _ := edgeRow(ctx, host, t.Name, 1)
		lt.First = first
		var lastRaw []any
		if t.Rows > 1 {
			lt.Last, lastRaw = edgeRow(ctx, host, t.Name, t.Rows)
		}
		// Première ligne tout en texte sous des colonnes typées : des en-têtes.
		if len(first) > 0 && typed {
			texts, others := 0, 0
			for _, k := range first {
				switch k {
				case "texte":
					texts++
				case "vide":
				default:
					others++
				}
			}
			if texts > 0 && others == 0 {
				lt.Hints = append(lt.Hints, "en-tête")
			}
		}
		if rowLooksTotal(lastRaw) {
			lt.Hints = append(lt.Hints, "total")
		}
		out.Tables = append(out.Tables, lt)
	}

	// Gras et fusions des cellules libres.
	type pos struct{ r, c int }
	bold, merge := map[pos]bool{}, map[pos]string{}
	if d, err := host.Doc(); err == nil {
		if l, err := d.Layout(ctx, v.ID); err == nil {
			for _, st := range l.Styles {
				var m map[string]any
				if json.Unmarshal([]byte(st.Style), &m) != nil {
					continue
				}
				p := pos{st.Row + 1, st.Col + 1}
				if b, _ := m["b"].(bool); b {
					bold[p] = true
				}
				mr, _ := m["mr"].(float64)
				mc, _ := m["mc"].(float64)
				if mr*mc >= 2 {
					merge[p] = fmt.Sprintf("%d×%d", int(mr), int(mc))
				}
			}
		}
	}
	perRow := map[int]int{}
	for _, c := range v.Cells {
		if strings.TrimSpace(c.Raw) != "" {
			perRow[c.Row]++
		}
	}
	for _, c := range v.Cells {
		if strings.TrimSpace(c.Raw) == "" {
			continue
		}
		if len(out.Cells) == layoutCells {
			out.Truncated = true
			break
		}
		p := pos{c.Row, c.Col}
		lc := layoutCell{Row: c.Row, Col: c.Col, Kind: kindOf(c.Raw), Bold: bold[p], Merge: merge[p]}
		if c.Error {
			lc.Kind = "erreur"
		}
		switch {
		case lc.Kind == "texte" && isTotal(c.Raw):
			lc.Hint = "total"
		case lc.Kind == "texte" && perRow[c.Row] == 1 && (lc.Bold || lc.Merge != "" || aboveTable(c.Row, c.Col, v.Tables)):
			lc.Hint = "titre"
		}
		out.Cells = append(out.Cells, lc)
	}
	for _, p := range v.Pivots {
		out.Objects = append(out.Objects, layoutObject{Kind: "tableau croisé", Name: p.Name, Row: p.Row, Col: p.Col})
	}
	for _, ch := range v.Charts {
		out.Objects = append(out.Objects, layoutObject{Kind: "graphique", Name: ch.Name, Row: ch.Row, Col: ch.Col})
	}
	for _, b := range v.Buttons {
		out.Objects = append(out.Objects, layoutObject{Kind: "bouton", Name: b.Name, Row: b.Row, Col: b.Col})
	}
	return encode(out), false
}

// edgeRow lit la ligne n (à partir de 1) d'un tableau, réduite à la nature
// de ses cellules ; rend aussi ses valeurs pour les indices.
func edgeRow(ctx context.Context, host api.Host, table string, n int) ([]string, []any) {
	res, err := api.Invoke(ctx, host, "tables.rows", map[string]any{"table": table, "first": n, "count": 1})
	if err != nil {
		return nil, nil
	}
	b, _ := json.Marshal(res)
	var r struct {
		Rows [][]any `json:"rows"`
	}
	if json.Unmarshal(b, &r) != nil || len(r.Rows) == 0 {
		return nil, nil
	}
	kinds := make([]string, len(r.Rows[0]))
	for i, x := range r.Rows[0] {
		kinds[i] = kindOf(x)
	}
	return kinds, r.Rows[0]
}

// rowLooksTotal : la première cellule non vide d'une ligne ressemble à un
// libellé de total.
func rowLooksTotal(row []any) bool {
	for _, x := range row {
		if kindOf(x) == "vide" {
			continue
		}
		return isTotal(x)
	}
	return false
}

// aboveTable : la cellule est juste au-dessus (trois lignes au plus) de
// l'en-tête d'un tableau dont elle couvre une colonne.
func aboveTable(row, col int, tables []api.Table) bool {
	for _, t := range tables {
		if row < t.Row && row >= t.Row-3 && col >= t.Col && col < t.Col+len(t.Columns) {
			return true
		}
	}
	return false
}
