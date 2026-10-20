package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// speak met la session dans une langue, comme le pont (applyLocale).
func speak(d *fixture, lang string) {
	d.SetLocale(formula.ForLanguage(lang))
	d.SetLanguage(msg.Lang(lang))
}

// freeCell : texte brut et valeur d'une cellule libre, lus dans ctx.
func freeCell(t *testing.T, ctx context.Context, d *fixture, row, col int) (string, string) {
	t.Helper()
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range l.Cells {
		if c.Row == row && c.Col == col {
			return c.Raw, c.Value
		}
	}
	return "", ""
}

// zhValue : une valeur affichée en anglais, en chinois (dates aaaa/mm/jj).
func zhValue(en string) string {
	if en == "10/19/2026" {
		return "2026/10/19"
	}
	return en
}

// Une cellule libre saisie dans une langue est gardée en français et se
// relit dans chaque langue, avec le même résultat.
func TestSessionFreeCells(t *testing.T) {
	ctx := context.Background()
	stored := StorageForm(ctx)
	d := open(t)
	speak(d, "en")
	for _, c := range []struct {
		row      int
		typed    string
		kept     string
		en, frV  string
		de, zh   string
		enV, deV string
	}{
		{0, "=SUM(1.5,2,TRUE)", "=SOMME(1,5; 2; VRAI)", "=SUM(1.5, 2, TRUE)", "4,5", "=SUM(1,5; 2; TRUE)", "=SUM(1.5, 2, TRUE)", "4.5", "4,5"},
		{1, "=IF(1>2,\"a;b\",#N/A)", "=SI(1>2; \"a;b\"; #N/A)", "=IF(1>2, \"a;b\", #N/A)", "#N/A", "=IF(1>2; \"a;b\"; #N/A)", "=IF(1>2, \"a;b\", #N/A)", "#N/A", "#N/A"},
		{2, "=1/0", "=1/0", "=1/0", "#DIV/0!", "=1/0", "=1/0", "#DIV/0!", "#DIV/0!"},
		{3, "=VALUE(\"x\")", "=CNUM(\"x\")", "=VALUE(\"x\")", "#VALEUR!", "=VALUE(\"x\")", "=VALUE(\"x\")", "#VALUE!", "#VALUE!"},
		{4, "1,234.5", "1234,5", "1234.5", "", "1234,5", "1234.5", "", ""},
		{5, "10/19/2026", "19/10/2026", "10/19/2026", "", "19.10.2026", "2026/10/19", "", ""},
		{6, "true", "VRAI", "TRUE", "", "TRUE", "TRUE", "", ""},
		{7, "Total", "Total", "Total", "", "Total", "Total", "", ""},
		{8, "=DATE(2026,10,19)", "=DATE(2026; 10; 19)", "=DATE(2026, 10, 19)", "19/10/2026", "=DATE(2026; 10; 19)", "=DATE(2026, 10, 19)", "10/19/2026", "19.10.2026"},
	} {
		speak(d, "en")
		if _, err := d.SetCell(ctx, 1, c.row, 0, c.typed); err != nil {
			t.Fatalf("%q : %v", c.typed, err)
		}
		if raw, _ := freeCell(t, stored, d, c.row, 0); raw != c.kept {
			t.Errorf("%q gardé %q, attendu %q", c.typed, raw, c.kept)
		}
		for _, l := range []struct{ lang, raw, value string }{
			{"en", c.en, c.enV}, {"fr", c.kept, c.frV}, {"de", c.de, c.deV}, {"zh", c.zh, zhValue(c.enV)},
		} {
			speak(d, l.lang)
			raw, value := freeCell(t, ctx, d, c.row, 0)
			if raw != l.raw || value != l.value {
				t.Errorf("%q en %s : %q = %q, attendu %q = %q", c.typed, l.lang, raw, value, l.raw, l.value)
			}
		}
	}
	// Retapée telle qu'elle s'affiche, une cellule ne change pas.
	for _, lang := range []string{"en", "de", "zh", "fr"} {
		speak(d, lang)
		for row := 0; row <= 8; row++ {
			before, _ := freeCell(t, stored, d, row, 0)
			shown, _ := freeCell(t, ctx, d, row, 0)
			if _, err := d.SetCell(ctx, 1, row, 0, shown); err != nil {
				t.Fatalf("%s %q : %v", lang, shown, err)
			}
			if after, _ := freeCell(t, stored, d, row, 0); after != before {
				t.Errorf("%s : %q retapé devient %q (gardé %q)", lang, shown, after, before)
			}
		}
	}
}

// Une formule illisible dans la langue de la session est refusée avec la
// position dans le texte tapé, dans cette langue.
func TestSessionSyntaxError(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	speak(d, "en")
	_, err := d.SetCell(ctx, 1, 0, 0, "=SUM(1;2)")
	var fe ErrFormula
	if !errors.As(err, &fe) {
		t.Fatalf("erreur de formule attendue : %v", err)
	}
	if got := msg.Text(msg.EN, err); !strings.Contains(got, "position 6") || strings.Contains(got, "caractère") {
		t.Errorf("texte anglais : %q", got)
	}
	if got := msg.Text(msg.FR, err); !strings.Contains(got, "position 6 : il manque « , »") {
		t.Errorf("texte français : %q", got)
	}
}

// Colonne calculée, règles, champ calculé et catégorie calculée saisis en
// anglais : gardés comme toujours, relus dans chaque langue.
func TestSessionTableFormulas(t *testing.T) {
	ctx := context.Background()
	stored := StorageForm(ctx)
	d := formulaSheet(t)
	speak(d, "en")
	// Date tapée dans une colonne Date : mois puis jour en anglais ; un
	// texte dans une colonne Texte reste tel que tapé.
	if _, err := d.SetCell(ctx, 1, 1, 0, "10/19/2026"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 2, 1, "1.5"); err != nil {
		t.Fatal(err)
	}
	var day, text string
	if err := d.read(ctx, func(tx *tx) error {
		tb, err := tx.tableByName("Feuille1")
		if err != nil {
			return err
		}
		if err := tx.row(`SELECT ` + colSQL(tb.Cols[0].ID) + ` FROM ` + tb.sql() + ` ORDER BY rowid LIMIT 1`).Scan(&day); err != nil {
			return err
		}
		return tx.row(`SELECT ` + colSQL(tb.Cols[1].ID) + ` FROM ` + tb.sql() + ` ORDER BY rowid LIMIT 1 OFFSET 1`).Scan(&text)
	}); err != nil || day != "2026-10-19" || text != "1.5" {
		t.Fatalf("valeurs gardées : %q %q %v", day, text, err)
	}
	// Colonne calculée (D, à droite du tableau).
	if _, err := d.SetCell(ctx, 1, 0, 3, "Double"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 3, "=[Montant]*1.5"); err != nil {
		t.Fatal(err)
	}
	column := func(c context.Context) (string, string) {
		l, err := d.Layout(c, 1)
		if err != nil {
			t.Fatal(err)
		}
		for _, tb := range l.Tables {
			for _, col := range tb.Columns {
				if col.Name == "Double" {
					return col.Formula, col.Rules
				}
			}
		}
		t.Fatal("colonne introuvable")
		return "", ""
	}
	var en string
	if err := d.read(ctx, func(t *tx) error {
		return t.row(`SELECT formula FROM _lattis_columns WHERE name = 'Double'`).Scan(&en)
	}); err != nil || en != "=[Montant]*1.5" {
		t.Fatalf("colonne gardée : %q %v", en, err)
	}
	// Règles : une formule et une comparaison à une valeur.
	rules := []Rule{{Op: "formula", Value: "=AND([Montant]>1.5,TRUE)", Style: Style{"b": true}}, {Op: "between", Value: "1.5", To: "1,234.5", Style: Style{"i": true}}}
	if err := d.SetRules(ctx, "Feuille1", 3, rules); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ lang, formula, rule, between string }{
		{"en", "=[Montant]*1.5", "=AND([Montant]>1.5, TRUE)", `"value":"1.5","to":"1234.5"`},
		{"fr", "=[Montant]*1,5", "=ET([Montant]>1,5; VRAI)", `"value":"1,5","to":"1234,5"`},
		{"de", "=[Montant]*1,5", "=AND([Montant]>1,5; TRUE)", `"value":"1,5","to":"1234,5"`},
	} {
		speak(d, c.lang)
		f, r := column(ctx)
		if f != c.formula || !strings.Contains(r, `"value":"`+c.rule+`"`) || !strings.Contains(r, c.between) {
			t.Errorf("%s : %q %s", c.lang, f, r)
		}
	}
	if _, r := column(stored); !strings.Contains(r, `"value":"=ET([Montant]>1,5; VRAI)"`) || !strings.Contains(r, `"value":"1,5","to":"1234,5"`) {
		t.Errorf("règles gardées : %s", r)
	}
	// Champ calculé d'un tableau croisé.
	speak(d, "en")
	cfg := PivotConfig{Source: "Feuille1", Calc: []PivotCalcField{{Name: "Year", Text: "=YEAR([Date])"}}, RowFields: []string{"Year"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}
	id, err := d.CreatePivotAt(ctx, 1, 10, 0, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetPivotCalcField(ctx, id, "", "Big", `=IF([Montant]>6.5,"yes","no")`); err != nil {
		t.Fatal(err)
	}
	for lang, want := range map[string]string{"en": `=YEAR([Date]) =IF([Montant]>6.5, "yes", "no")`, "fr": `=ANNEE([Date]) =SI([Montant]>6,5; "yes"; "no")`, "de": `=YEAR([Date]) =IF([Montant]>6,5; "yes"; "no")`} {
		speak(d, lang)
		l, err := d.Layout(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, p := range l.Pivots {
			if p.ID == id {
				for _, f := range p.Config.Calc {
					got = append(got, f.Text)
				}
			}
		}
		if strings.Join(got, " ") != want {
			t.Errorf("%s : %q", lang, got)
		}
	}
	// Catégorie calculée de l'onglet.
	speak(d, "en")
	if p, err := d.PreviewChoiceFormula(ctx, 1, "=YEAR([Date])+0.5"); err != nil || p.Error != "" || len(p.Values) == 0 {
		t.Fatalf("aperçu : %+v %v", p, err)
	}
	if err := d.SetSheetChoiceFormula(ctx, 1, "Year", "=YEAR([Date])+0.5", ""); err != nil {
		t.Fatal(err)
	}
	speak(d, "fr")
	if ch, err := d.SheetChoice(ctx, 1); err != nil || ch == nil || ch.Formula != "=ANNEE([Date])+0,5" {
		t.Fatalf("catégorie : %+v %v", ch, err)
	}
}

// L'API (scripts, assistant) lit et écrit la forme gardée, quelle que soit
// la langue de la session.
func TestSessionStorageForm(t *testing.T) {
	ctx := StorageForm(context.Background())
	d := open(t)
	speak(d, "en")
	if _, err := d.SetCell(ctx, 1, 0, 0, "=SOMME(1,5; 2)"); err != nil {
		t.Fatal(err)
	}
	if raw, value := freeCell(t, ctx, d, 0, 0); raw != "=SOMME(1,5; 2)" || value != "3,5" {
		t.Errorf("forme gardée : %q = %q", raw, value)
	}
	if raw, value := freeCell(t, context.Background(), d, 0, 0); raw != "=SUM(1.5, 2)" || value != "3.5" {
		t.Errorf("session anglaise : %q = %q", raw, value)
	}
}
