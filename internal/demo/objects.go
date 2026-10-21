package demo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/query"
)

// Objets d'un plan au-delà des tableaux : tableaux croisés, graphiques,
// contrôles, mise en forme posés sur une feuille ; requêtes chargées dans
// une feuille ; onglets Document et pages HTML ; scripts. Les presets les
// écrivent en Go, les fichiers YAML (yaml.go) les décrivent : même chemin.

// PivotPlacement est un tableau croisé posé sur une feuille, coin haut
// gauche en (Row, Col).
type PivotPlacement struct {
	Row, Col int
	Config   canvas.PivotConfig
}

// ControlSpec est un contrôle (bouton, case, liste, champ) posé sur une
// feuille.
type ControlSpec struct {
	Row, Col int
	Config   canvas.ButtonConfig
}

// RuleSpec pose des règles de mise en forme conditionnelle sur une colonne
// de tableau.
type RuleSpec struct {
	Table, Column string
	Rules         []canvas.Rule
}

// StyleSpec met en forme le rectangle [R0, R1] × [C0, C1] de la feuille.
type StyleSpec struct {
	R0, C0, R1, C1 int
	Style          canvas.Style
}

// WidthSpec fixe la largeur (px) d'une colonne de la feuille.
type WidthSpec struct{ Col, Width int }

// QuerySpec est une requête du classeur chargée dans une feuille. Avec SQL,
// c'est une requête SQL sur le classeur (comme l'onglet SQL › Résultat vers
// une feuille, tableau lié) : les tableaux et colonnes s'y écrivent
// {{Tableau}} et {{Tableau[Colonne]}}, remplacés par leurs noms internes.
// Sinon Source (tableau du classeur, autre requête…) et Steps, comme
// Données › Obtenir des données.
type QuerySpec struct {
	SQL    string
	Source query.Source
	Steps  []query.Step
	// Table : nom du tableau chargé (vide : tiré du nom de l'onglet).
	Table string
}

// ScriptSpec est un script de la bibliothèque.
type ScriptSpec struct{ Name, Source string }

// FavoriteSpec est une requête favorite de l'onglet SQL : son nom et son
// texte (noms affichés). Une page web l'exécute par son nom
// (lattis.query(nom)), en lecture seule.
type FavoriteSpec struct{ Name, SQL string }

// DocBlock est un paragraphe d'un onglet Document. Kind : "h1", "h2", "h3",
// "p", "bullet", "number", "quote", ou un objet seul sur sa ligne : "chart",
// "pivot". Le texte accepte **gras** et *italique*. Un paragraphe se
// termine, sur la même ligne, par Button et Control s'ils sont donnés.
type DocBlock struct {
	Kind    string
	Text    string
	Chart   *canvas.ChartConfig
	Pivot   *canvas.PivotConfig
	Button  *DocButton
	Control *canvas.ButtonConfig
}

// DocButton est un bouton d'un onglet Document : il exécute un script de la
// bibliothèque, ou l'une de ses fonctions.
type DocButton struct {
	Label    string `json:"label"`
	Script   string `json:"script"`
	Function string `json:"function,omitempty"`
	Color    string `json:"color,omitempty"`
	Text     string `json:"textColor,omitempty"`
}

var docKinds = map[string]bool{"h1": true, "h2": true, "h3": true, "p": true, "bullet": true, "number": true, "quote": true, "chart": true, "pivot": true}

// placeholder : {{Tableau}} ou {{Tableau[Colonne]}} dans le SQL d'une requête.
var placeholder = regexp.MustCompile(`\{\{\s*([^\[\]{}]+?)\s*(?:\[\s*([^\[\]{}]+?)\s*\])?\s*\}\}`)

// expandSQL remplace les {{Tableau}} et {{Tableau[Colonne]}} par les noms
// internes (t_3, c_5) ; un tableau croisé garde son nom.
func expandSQL(ctx context.Context, m *canvas.Model, q string) (string, error) {
	all, err := m.AllTables(ctx)
	if err != nil {
		return "", err
	}
	var bad error
	out := placeholder.ReplaceAllStringFunc(q, func(s string) string {
		p := placeholder.FindStringSubmatch(s)
		for _, t := range all {
			if !strings.EqualFold(t.Name, p[1]) {
				continue
			}
			if p[2] == "" {
				return t.SQL
			}
			for i, c := range t.Columns {
				if strings.EqualFold(c, p[2]) {
					return t.ColumnSQL[i]
				}
			}
			if bad == nil {
				bad = fmt.Errorf("{{%s[%s]}} : colonne introuvable", p[1], p[2])
			}
			return s
		}
		if bad == nil {
			bad = fmt.Errorf("{{%s}} : tableau introuvable", p[1])
		}
		return s
	})
	return out, bad
}

// loadQuery enregistre et charge la requête d'un onglet ; renvoie la
// feuille créée.
func loadQuery(ctx context.Context, m *canvas.Model, s SheetSpec) (int64, error) {
	qs := s.Query
	src := qs.Source
	if strings.TrimSpace(qs.SQL) != "" {
		q, err := expandSQL(ctx, m, qs.SQL)
		if err != nil {
			return 0, err
		}
		src = query.Source{Kind: query.SourceSQL, SQL: q}
	}
	def, err := query.Encode(query.Query{Name: s.Name, Source: src, Steps: qs.Steps})
	if err != nil {
		return 0, err
	}
	id, err := m.SaveQuery(ctx, def)
	if err != nil {
		return 0, err
	}
	res, err := query.Refresh(ctx, query.Env{Model: m}, id)
	if err != nil {
		return 0, err
	}
	if qs.Table != "" && qs.Table != res.Table {
		if err := m.RenameTable(ctx, res.Table, qs.Table); err != nil {
			return 0, err
		}
	}
	return res.Sheet, nil
}

// placeObjects pose les objets d'une feuille.
func placeObjects(ctx context.Context, m *canvas.Model, sheet int64, s SheetSpec) error {
	for _, w := range s.Widths {
		if err := m.SetColumnWidth(ctx, sheet, w.Col, w.Width); err != nil {
			return err
		}
	}
	for _, r := range s.Rules {
		if err := setRules(ctx, m, r); err != nil {
			return err
		}
	}
	for _, st := range s.Styles {
		if err := m.ApplyStyle(ctx, sheet, st.R0, st.C0, st.R1, st.C1, st.Style); err != nil {
			return fmt.Errorf("mise en forme : %w", err)
		}
	}
	for _, p := range s.Pivots {
		id, err := m.PlacePivot(ctx, sheet, p.Row, p.Col, p.Config)
		if err == nil && p.Config.Name != "" {
			err = m.SetPivot(ctx, id, p.Config) // PlacePivot choisit un nom « TCDn »
		}
		if err != nil {
			return fmt.Errorf("tableau croisé %s : %w", p.Config.Name, err)
		}
	}
	for _, ch := range s.Charts {
		if _, err := m.CreateChart(ctx, sheet, ch.Row, ch.Col, ch.Config); err != nil {
			return fmt.Errorf("graphique « %s » : %w", ch.Config.Title, err)
		}
	}
	for _, c := range s.Controls {
		if _, err := m.CreateButton(ctx, sheet, c.Row, c.Col, c.Config); err != nil {
			return fmt.Errorf("contrôle « %s » : %w", c.Config.Label, err)
		}
	}
	return nil
}

func setRules(ctx context.Context, m *canvas.Model, r RuleSpec) error {
	all, err := m.AllTables(ctx)
	if err != nil {
		return err
	}
	for _, t := range all {
		if t.Kind != "table" || !strings.EqualFold(t.Name, r.Table) {
			continue
		}
		for i, c := range t.Columns {
			if strings.EqualFold(c, r.Column) {
				if err := m.SetRules(ctx, t.Name, i, r.Rules); err != nil {
					return fmt.Errorf("règles de %s[%s] : %w", r.Table, r.Column, err)
				}
				return nil
			}
		}
		return fmt.Errorf("règles : colonne %s[%s] introuvable", r.Table, r.Column)
	}
	return fmt.Errorf("règles : tableau %s introuvable", r.Table)
}

// addPage ajoute un onglet Document ou une page HTML, nommé, à la fin.
func addPage(ctx context.Context, m *canvas.Model, s SheetSpec) (int64, error) {
	var id int64
	var err error
	if s.Kind() == canvas.KindDoc {
		id, err = m.AddDoc(ctx, 0)
	} else {
		id, err = m.AddPage(ctx, 0)
	}
	if err != nil {
		return 0, err
	}
	if err := m.RenameSheet(ctx, id, s.Name); err != nil {
		return 0, err
	}
	src := s.HTML
	if s.Kind() == canvas.KindDoc {
		if src, err = docDelta(ctx, m, id, s.Doc); err != nil {
			return 0, fmt.Errorf("document « %s » : %w", s.Name, err)
		}
	}
	return id, m.SetPage(ctx, id, src)
}

// docDelta écrit le Delta Quill d'un onglet Document ; ses contrôles sont
// créés sur l'onglet sheet.
func docDelta(ctx context.Context, m *canvas.Model, sheet int64, blocks []DocBlock) (string, error) {
	var ops []map[string]any
	text := func(s string) {
		for _, r := range inlineRuns(s) {
			op := map[string]any{"insert": r.text}
			if r.attrs != nil {
				op["attributes"] = r.attrs
			}
			ops = append(ops, op)
		}
	}
	for i, b := range blocks {
		if !docKinds[b.Kind] {
			return "", fmt.Errorf("bloc %d : genre « %s » inconnu (h1, h2, h3, p, bullet, number, quote, chart, pivot)", i+1, b.Kind)
		}
		switch b.Kind {
		case "chart":
			if b.Chart == nil {
				return "", fmt.Errorf("bloc %d : graphique sans configuration", i+1)
			}
			if err := checkChart(m.ChartData(ctx, *b.Chart)); err != nil {
				return "", fmt.Errorf("bloc %d : graphique « %s » : %w", i+1, b.Chart.Title, err)
			}
			ops = append(ops, map[string]any{"insert": map[string]any{"lattisChart": b.Chart}})
			continue
		case "pivot":
			if b.Pivot == nil {
				return "", fmt.Errorf("bloc %d : tableau croisé sans configuration", i+1)
			}
			if info := m.PivotData(ctx, *b.Pivot); info.Error != "" {
				return "", fmt.Errorf("bloc %d : tableau croisé : %s", i+1, info.Error)
			}
			ops = append(ops, map[string]any{"insert": map[string]any{"lattisPivot": b.Pivot}})
			continue
		}
		text(b.Text)
		if b.Control != nil {
			id, err := m.CreateButton(ctx, sheet, 0, 0, *b.Control)
			if err != nil {
				return "", fmt.Errorf("bloc %d : %w", i+1, err)
			}
			if b.Text != "" {
				ops = append(ops, map[string]any{"insert": " "})
			}
			ops = append(ops, map[string]any{"insert": map[string]any{canvas.DocControlBlot: map[string]any{"id": id}}})
		}
		if b.Button != nil {
			ops = append(ops, map[string]any{"insert": " "}, map[string]any{"insert": map[string]any{"lattisButton": b.Button}})
		}
		nl := map[string]any{"insert": "\n"}
		switch b.Kind {
		case "h1", "h2", "h3":
			nl["attributes"] = map[string]any{"header": int(b.Kind[1] - '0')}
		case "bullet":
			nl["attributes"] = map[string]any{"list": "bullet"}
		case "number":
			nl["attributes"] = map[string]any{"list": "ordered"}
		case "quote":
			nl["attributes"] = map[string]any{"blockquote": true}
		}
		ops = append(ops, nl)
	}
	if n := len(ops); n == 0 || ops[n-1]["insert"] != "\n" {
		ops = append(ops, map[string]any{"insert": "\n"})
	}
	var w strings.Builder
	enc := json.NewEncoder(&w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]any{"ops": ops}); err != nil {
		return "", err
	}
	return strings.TrimSpace(w.String()), nil
}

type run struct {
	text  string
	attrs map[string]any
}

// inlineRuns découpe un texte en morceaux **gras** et *italique*.
func inlineRuns(s string) []run {
	var out []run
	for s != "" {
		k := strings.Index(s, "*")
		if k < 0 {
			out = append(out, run{text: s})
			break
		}
		mark, attr := "*", "italic"
		if strings.HasPrefix(s[k:], "**") {
			mark, attr = "**", "bold"
		}
		end := strings.Index(s[k+len(mark):], mark)
		if end < 0 {
			out = append(out, run{text: s})
			break
		}
		if k > 0 {
			out = append(out, run{text: s[:k]})
		}
		inner := s[k+len(mark) : k+len(mark)+end]
		out = append(out, run{text: inner, attrs: map[string]any{attr: true}})
		s = s[k+2*len(mark)+end:]
	}
	return out
}

// checkChart rend l'erreur d'un graphique d'un document (les graphiques
// d'une feuille sont vérifiés à leur création).
func checkChart(info canvas.ChartInfo) error {
	if info.Error != "" {
		return errors.New(info.Error)
	}
	return nil
}
