package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// Session dans une autre langue : formules et bornes passent à la forme
// gardée (français), adresses A1 ($ compris) intactes, et reviennent.
func TestDVSessionLanguage(t *testing.T) {
	u := ui{loc: formula.EN, dates: grid.DateStyleFor("en")}
	p, err := u.dvIn(Style{"dv": map[string]any{"type": "custom", "formula": "=COUNTIF($A$2:$A$9,A2)=1"}})
	must(t, err)
	if f := p["dv"].(map[string]any)["formula"]; f != "=NB.SI($A$2:$A$9; A2)=1" {
		t.Fatalf("formule gardée : %v", f)
	}
	p, err = u.dvIn(Style{"dv": map[string]any{"type": "decimal", "op": "between", "min": "2.5", "max": "=$B$1*1.5"}})
	must(t, err)
	m := p["dv"].(map[string]any)
	if m["min"] != "2,5" || m["max"] != "=$B$1*1,5" {
		t.Fatalf("bornes gardées : %v", m)
	}
	r := u.dvOut(DVRule{Type: "custom", Formula: "=NB.SI($A$2:$A$9;A2)=1", Min: "2,5"})
	if r.Formula != "=COUNTIF($A$2:$A$9, A2)=1" || r.Min != "2.5" {
		t.Fatalf("relue : %+v", r)
	}
}

// typing : une saisie de la grille (validation vérifiée).
var typing = WithEntryCheck(context.Background(), false)

// refusal : le style du refus de validation d'une saisie ("" : acceptée),
// et son texte.
func refusal(t *testing.T, d *fixture, ctx context.Context, sheet int64, row, col int, raw string) (string, string) {
	t.Helper()
	_, err := d.SetCell(ctx, sheet, row, col, raw)
	if err == nil {
		return "", ""
	}
	var e *msg.Error
	if !errors.As(err, &e) || e.ID != "validation.refusedEntry" {
		t.Fatalf("saisie %q : %v", raw, err)
	}
	return e.Args["style"].(string), e.Args["text"].(string)
}

func freeRawAt(t *testing.T, d *fixture, sheet int64, row, col int) string {
	t.Helper()
	var s string
	must(t, d.read(context.Background(), func(tx *tx) error {
		return tx.row(`SELECT coalesce((SELECT raw FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?), '')`, sheet, row, col).Scan(&s)
	}))
	return s
}

// Chaque type et chaque opérateur, dans une cellule libre isolée (B2).
func TestDVTypesAndOperators(t *testing.T) {
	d := open(t)
	cases := []struct {
		rule     map[string]any
		ok, fail []string
	}{
		{map[string]any{"type": "whole", "op": "between", "min": "1", "max": "10"}, []string{"1", "10", "5"}, []string{"0", "11", "2,5", "abc"}},
		{map[string]any{"type": "whole", "op": "notbetween", "min": "1", "max": "10"}, []string{"0", "11"}, []string{"5"}},
		{map[string]any{"type": "whole", "op": "=", "min": "3"}, []string{"3"}, []string{"4"}},
		{map[string]any{"type": "whole", "op": "<>", "min": "3"}, []string{"4"}, []string{"3"}},
		{map[string]any{"type": "decimal", "op": ">", "min": "2,5"}, []string{"2,6", "3"}, []string{"2,5", "x"}},
		{map[string]any{"type": "decimal", "op": "<", "min": "2,5"}, []string{"2,4"}, []string{"2,5"}},
		{map[string]any{"type": "decimal", "op": ">=", "min": 2.5}, []string{"2,5"}, []string{"2,4"}},
		{map[string]any{"type": "decimal", "op": "<=", "min": "2,5"}, []string{"2,5", "-1"}, []string{"2,6"}},
		{map[string]any{"type": "date", "op": "between", "min": "01/01/2026", "max": "31/12/2026"}, []string{"15/06/2026", "01/01/2026"}, []string{"15/06/2027", "texte"}},
		{map[string]any{"type": "date", "op": ">", "min": "=AUJOURDHUI()-1"}, []string{"01/01/2099"}, []string{"01/01/2000"}},
		{map[string]any{"type": "time", "op": "between", "min": "08:00", "max": "18:00"}, []string{"12:30", "08:00", "18:00"}, []string{"19:00", "07:59"}},
		{map[string]any{"type": "length", "op": "<=", "min": "3"}, []string{"abc", "ab"}, []string{"abcd"}},
		{map[string]any{"type": "length", "op": "between", "min": "2", "max": "4"}, []string{"ab", "1234"}, []string{"a", "abcde"}},
		{map[string]any{"type": "list", "list": []any{"Oui", "Non"}}, []string{"oui", "Non"}, []string{"Peut-être"}},
		{map[string]any{"type": "custom", "formula": "=ESTNUM(B2)", "at": "B2"}, []string{"12"}, []string{"douze"}},
	}
	for _, c := range cases {
		must(t, d.ApplyStyle(context.Background(), 1, 1, 1, 1, 1, Style{"dv": c.rule}))
		for _, v := range c.ok {
			if st, text := refusal(t, d, typing, 1, 1, 1, v); st != "" {
				t.Errorf("%v : %q refusé (%s)", c.rule, v, text)
			}
		}
		for _, v := range c.fail {
			before := freeRawAt(t, d, 1, 1, 1)
			st, text := refusal(t, d, typing, 1, 1, 1, v)
			if st != "stop" {
				t.Errorf("%v : %q accepté", c.rule, v)
				continue
			}
			if text == "" || !strings.Contains(text, "Attendu") {
				t.Errorf("%v : message %q", c.rule, text)
			}
			if got := freeRawAt(t, d, 1, 1, 1); got != before {
				t.Errorf("%v : refus non annulé (%q au lieu de %q)", c.rule, got, before)
			}
		}
	}
	// Hors saisie de la grille (script, collage) : rien n'est vérifié.
	if _, err := d.SetCell(context.Background(), 1, 1, 1, "douze"); err != nil {
		t.Fatal(err)
	}
}

// Ignorer si vide : coché par défaut ; décoché, une saisie vide est refusée.
func TestDVBlank(t *testing.T) {
	d := open(t)
	must(t, d.ApplyStyle(context.Background(), 1, 1, 1, 1, 1, Style{"dv": map[string]any{"type": "whole", "op": ">", "min": "0"}}))
	_, err := d.SetCell(context.Background(), 1, 1, 1, "5")
	must(t, err)
	if st, _ := refusal(t, d, typing, 1, 1, 1, ""); st != "" {
		t.Fatal("vide refusé alors que « Ignorer si vide » est coché")
	}
	must(t, d.ApplyStyle(context.Background(), 1, 1, 1, 1, 1, Style{"dv": map[string]any{"type": "whole", "op": ">", "min": "0", "blank": false}}))
	_, err = d.SetCell(context.Background(), 1, 1, 1, "5")
	must(t, err)
	st, text := refusal(t, d, typing, 1, 1, 1, "")
	if st != "stop" || !strings.Contains(text, "vide") {
		t.Fatalf("vide accepté : %q %q", st, text)
	}
	// Borne en référence à une cellule vide : ignorée quand coché.
	must(t, d.ApplyStyle(context.Background(), 1, 3, 3, 3, 3, Style{"dv": map[string]any{"type": "whole", "op": ">", "min": "=F10"}}))
	if st, _ := refusal(t, d, typing, 1, 3, 3, "-4"); st != "" {
		t.Fatal("borne vide : refusé")
	}
	_, err = d.SetCell(context.Background(), 1, 9, 5, "0")
	must(t, err)
	if st, _ := refusal(t, d, typing, 1, 3, 3, "-4"); st != "stop" {
		t.Fatal("borne lue en F10 : accepté")
	}
}

// Formule personnalisée dans des cellules libres : écrite pour la cellule
// active (at), ses adresses relatives suivent chaque cellule (valeurs
// uniques dans A2:A9, une ligne sur deux pour rester des cellules isolées).
func TestDVCustomFreeCells(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	must(t, d.ApplyStyle(ctx, 1, 1, 0, 8, 0, Style{"dv": map[string]any{"type": "custom", "formula": "=NB.SI($A$2:$A$9;A2)=1", "at": "A2",
		"error": map[string]any{"style": "warning", "title": "Doublon", "msg": "Déjà saisi"}}}))
	if st, _ := refusal(t, d, typing, 1, 1, 0, "x"); st != "" {
		t.Fatal("première valeur refusée")
	}
	st, text := refusal(t, d, typing, 1, 3, 0, "x")
	if st != "warning" || text != "Déjà saisi" {
		t.Fatalf("doublon : %q %q", st, text)
	}
	if got := freeRawAt(t, d, 1, 3, 0); got != "" {
		t.Fatalf("doublon écrit : %q", got)
	}
	// Oui à l'avertissement : la valeur passe.
	if st, _ := refusal(t, d, WithEntryCheck(ctx, true), 1, 3, 0, "x"); st != "" {
		t.Fatal("avertissement accepté puis refusé")
	}
	if got := freeRawAt(t, d, 1, 3, 0); got != "x" {
		t.Fatalf("valeur acceptée : %q", got)
	}
	// Le dialogue montre la formule pour la cellule active.
	r, err := d.ValidationAt(ctx, 1, 5, 0)
	must(t, err)
	if r.Formula != "=NB.SI($A$2:$A$9;A6)=1" || r.At != "A6" || r.Error.Style != "warning" {
		t.Fatalf("validation lue : %+v", r)
	}
	// Entourer les données non valides : les deux « x ».
	cells, err := d.InvalidCells(ctx, 1)
	must(t, err)
	if len(cells) != 2 || cells[0] != (CellPos{1, 0}) || cells[1] != (CellPos{3, 0}) {
		t.Fatalf("cellules non valides : %v", cells)
	}
}

// Styles d'alerte : Informations prévient et accepte avec OK.
func TestDVErrorStyles(t *testing.T) {
	d := open(t)
	for _, style := range []string{"stop", "warning", "info"} {
		must(t, d.ApplyStyle(context.Background(), 1, 1, 1, 1, 1, Style{"dv": map[string]any{"type": "whole", "op": "<", "min": "5", "error": map[string]any{"style": style}}}))
		if st, _ := refusal(t, d, typing, 1, 1, 1, "9"); st != style {
			t.Fatalf("style %s : %q", style, st)
		}
	}
	// Tout avec un message de saisie seul : gardé ; sans rien : retiré.
	must(t, d.ApplyStyle(context.Background(), 1, 1, 1, 1, 1, Style{"dv": map[string]any{"type": "any", "input": map[string]any{"title": "Aide", "msg": "Un nombre"}}}))
	if r, _ := ParseDV(freeDV(t, d, 1, 1, 1)); r.Type != "any" || r.Input == nil || r.Input.Msg != "Un nombre" {
		t.Fatalf("message de saisie : %+v", freeDV(t, d, 1, 1, 1))
	}
	if st, _ := refusal(t, d, typing, 1, 1, 1, "9"); st != "" {
		t.Fatal("Tout : refusé")
	}
	must(t, d.ApplyStyle(context.Background(), 1, 1, 1, 1, 1, Style{"dv": map[string]any{"type": "any"}}))
	if v := freeDV(t, d, 1, 1, 1); v != nil {
		t.Fatalf("Tout sans message : %v", v)
	}
	// Une règle incomplète est refusée à la pose.
	for _, bad := range []map[string]any{
		{"type": "whole", "op": "between", "min": "1"},
		{"type": "whole", "min": "abc"},
		{"type": "custom"},
		{"type": "nope"},
	} {
		if err := d.ApplyStyle(context.Background(), 1, 1, 1, 1, 1, Style{"dv": bad}); err == nil {
			t.Errorf("règle acceptée : %v", bad)
		}
	}
}

// Dans un tableau : la formule posée avec les adresses de la cellule active
// devient [@Colonne] (chaque ligne), le renommage d'une colonne la suit.
func TestDVTableColumn(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Début", "Fin"}, {"01/01/2026", "10/01/2026"}, {"05/01/2026", "06/01/2026"}, {"02/02/2026", "03/02/2026"}})
	must(t, err)
	must(t, d.RenameTable(ctx, "Feuille1", "Séjours"))
	// Toute la colonne Fin (lignes 1 à 3), cellule active B3 (2e ligne).
	must(t, d.ApplyStyle(ctx, 1, 1, 1, 3, 1, Style{"dv": map[string]any{"type": "date", "op": ">=", "min": "=A3", "at": "B3"}}))
	var style string
	must(t, d.read(ctx, func(tx *tx) error {
		return tx.row(`SELECT style FROM _lattis_styles WHERE rid = 0`).Scan(&style)
	}))
	if !strings.Contains(style, `"min":"=[@Début]"`) || strings.Contains(style, `"at"`) {
		t.Fatalf("style de la colonne : %s", style)
	}
	if st, _ := refusal(t, d, typing, 1, 3, 1, "01/02/2026"); st != "stop" {
		t.Fatal("fin avant le début acceptée")
	}
	if st, text := refusal(t, d, typing, 1, 3, 1, "20/02/2026"); st != "" {
		t.Fatalf("fin après le début refusée : %s", text)
	}
	// Formule personnalisée [@Colonne], tapée directement.
	must(t, d.ApplyStyle(ctx, 1, 1, 0, 3, 0, Style{"dv": map[string]any{"type": "custom", "formula": "=[@Début]<=[@Fin]"}}))
	if st, _ := refusal(t, d, typing, 1, 2, 0, "07/01/2026"); st != "stop" {
		t.Fatal("début après la fin accepté")
	}
	// Renommer la colonne Début : les formules suivent.
	_, err = d.SetCell(ctx, 1, 0, 0, "Arrivée")
	must(t, err)
	must(t, d.read(ctx, func(tx *tx) error {
		return tx.row(`SELECT group_concat(style, '|') FROM _lattis_styles WHERE rid = 0`).Scan(&style)
	}))
	if !strings.Contains(style, `"min":"=[@Arrivée]"`) || !strings.Contains(style, `[@Arrivée]<=[@Fin]`) {
		t.Fatalf("après renommage : %s", style)
	}
	if st, _ := refusal(t, d, typing, 1, 2, 0, "07/01/2026"); st != "stop" {
		t.Fatal("après renommage : début après la fin accepté")
	}
	// Une valeur collée hors validation se voit avec Entourer.
	_, err = d.SetCell(ctx, 1, 1, 1, "01/12/2025")
	must(t, err)
	cells, err := d.InvalidCells(ctx, 1)
	must(t, err)
	// La fin avant le début : les deux cellules de la ligne sont fausses.
	if len(cells) != 2 || cells[0] != (CellPos{1, 0}) || cells[1] != (CellPos{1, 1}) {
		t.Fatalf("cellules non valides : %v", cells)
	}
}

// Une liste tirée d'une plage sous forme objet (alerte, messages) suit le
// renommage du tableau source sans perdre ses autres réglages.
func TestDVObjectSourceRename(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "N"}, {"FR", "1"}, {"IT", "2"}})
	must(t, err)
	must(t, d.RenameTable(ctx, "Feuille1", "Ref"))
	must(t, d.ApplyStyle(ctx, 1, 10, 0, 10, 0, Style{"dv": map[string]any{"type": "list", "src": "=Ref[Pays]", "error": map[string]any{"style": "info"}, "input": map[string]any{"msg": "Un pays"}}}))
	must(t, d.RenameTable(ctx, "Ref", "Pays"))
	r, ok := ParseDV(freeDV(t, d, 1, 10, 0))
	if !ok || r.Src != "Pays[Pays]" || r.Error == nil || r.Error.Style != "info" || r.Input == nil || r.Input.Msg != "Un pays" {
		t.Fatalf("après renommage : %+v", freeDV(t, d, 1, 10, 0))
	}
	if src, ok := DVSource(freeDV(t, d, 1, 10, 0)); !ok || src != "Pays[Pays]" {
		t.Fatalf("DVSource : %q", src)
	}
	if st, _ := refusal(t, d, typing, 1, 10, 0, "DE"); st != "info" {
		t.Fatalf("hors liste : %q", st)
	}
	if st, _ := refusal(t, d, typing, 1, 10, 0, "it"); st != "" {
		t.Fatal("valeur de la liste refusée")
	}
}
