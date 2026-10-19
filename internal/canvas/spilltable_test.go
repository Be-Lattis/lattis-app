package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// spillSource pose le tableau Suividesnids (NID suivi, Etat) en A1 de la
// feuille 1.
func spillSource(t *testing.T, d *fixture) {
	t.Helper()
	ctx := context.Background()
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"NID suivi", "Etat"}, {"N1", "ok"}, {"N2", "vide"}, {"N1", "ok"}, {"N3", "ok"}})
	must(t, err)
	must(t, d.RenameTable(ctx, "Feuille1", "Suividesnids"))
}

// tableText lit un tableau : en-têtes puis lignes, valeurs séparées par
// « | », lignes par « / ».
func tableText(t *testing.T, d *fixture, name string) string {
	t.Helper()
	var out []string
	var cols []ExportColumn
	err := d.ExportTable(context.Background(), name, func(c []ExportColumn) error {
		cols = c
		var h []string
		for _, c := range c {
			h = append(h, c.Name+":"+string(c.Type))
		}
		out = append(out, strings.Join(h, "|"))
		return nil
	}, func(vals []any) error {
		var r []string
		for i, v := range vals {
			r = append(r, formulaText(v, cols[i].Type))
		}
		out = append(out, strings.Join(r, "|"))
		return nil
	})
	must(t, err)
	return strings.Join(out, " / ")
}

func tableInfo(t *testing.T, d *fixture, name string) TableInfo {
	t.Helper()
	l, err := d.Layout(context.Background(), 1)
	must(t, err)
	for _, tb := range l.Tables {
		if tb.Name == name {
			return tb
		}
	}
	t.Fatalf("tableau %s introuvable", name)
	return TableInfo{}
}

// « Convertir en tableau (valeurs) » : le titre « Nids » au-dessus de
// =UNIQUE(…) devient l'en-tête, les valeurs restent en place ; plus de
// lien ; une seule étape d'annulation rend la formule et le titre.
func TestConvertSpillValues(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	spillSource(t, d)
	mustSet(t, d, 1, 0, 5, "Nids")
	mustSet(t, d, 1, 1, 5, "=UNIQUE(Suividesnids[NID suivi])")
	name, err := d.ConvertSpill(ctx, 1, 1, 5, false)
	must(t, err)
	if name != "Nids" {
		t.Fatalf("nom : %q", name)
	}
	if got, want := tableText(t, d, "Nids"), "Nids:text / N1 / N2 / N3"; got != want {
		t.Fatalf("tableau :\n got %s\nwant %s", got, want)
	}
	ti := tableInfo(t, d, "Nids")
	if ti.Row != 0 || ti.Col != 5 || ti.QueryID != 0 {
		t.Fatalf("place ou lien : %+v", ti)
	}
	check(t, d, "conversion")
	// Valeurs figées : la source change, pas le tableau.
	mustSet(t, d, 1, 5, 0, "N4")
	if got := tableText(t, d, "Nids"); strings.Contains(got, "N4") {
		t.Fatalf("tableau de valeurs recalculé : %s", got)
	}
	_, err = d.bus.Undo(ctx)
	must(t, err)
	_, err = d.bus.Undo(ctx)
	must(t, err)
	if c := cellOn(t, d, 1, 1, 5); c.Raw != "=UNIQUE(Suividesnids[NID suivi])" || len(c.Spill) != 3 {
		t.Fatalf("formule rendue : %+v", c)
	}
	if c := cellOn(t, d, 1, 0, 5); c.Raw != "Nids" {
		t.Fatalf("titre rendu : %+v", c)
	}
	check(t, d, "annulation")
}

// Plusieurs colonnes (FILTRE d'un tableau entier) : autant de colonnes,
// en-têtes naturels posés sur la ligne vide au-dessus de l'ancre ; ancre
// en ligne 1 : l'en-tête prend sa ligne, les valeurs descendent.
func TestConvertSpillColumns(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	spillSource(t, d)
	mustSet(t, d, 1, 3, 5, `=FILTRE(Suividesnids; Suividesnids[Etat]="ok")`)
	name, err := d.ConvertSpill(ctx, 1, 3, 5, false)
	must(t, err)
	if got, want := tableText(t, d, name), "NID suivi:text|Etat:text / N1|ok / N1|ok / N3|ok"; got != want {
		t.Fatalf("tableau :\n got %s\nwant %s", got, want)
	}
	if ti := tableInfo(t, d, name); ti.Row != 2 || ti.Col != 5 {
		t.Fatalf("place : %+v", ti)
	}
	check(t, d, "plusieurs colonnes")

	d = open(t)
	spillSource(t, d)
	mustSet(t, d, 1, 0, 5, "=SEQUENCE(3)")
	mustSet(t, d, 1, 3, 5, "après")
	name, err = d.ConvertSpill(ctx, 1, 0, 5, false)
	must(t, err)
	if got, want := tableText(t, d, name), "Valeur:integer / 1 / 2 / 3"; got != want {
		t.Fatalf("SEQUENCE :\n got %s\nwant %s", got, want)
	}
	if ti := tableInfo(t, d, name); ti.Row != 0 || ti.Col != 5 {
		t.Fatalf("place : %+v", ti)
	}
	// La cellule collée sous le déversement s'est écartée.
	if c := cellOn(t, d, 1, 3, 5); c.Raw == "après" {
		t.Fatalf("cellule recouverte : %+v", c)
	}
	check(t, d, "en-tête sur l'ancre")

	// Pas de formule, ou pas un tableau dynamique : refusé.
	mustSet(t, d, 1, 10, 0, "=1+1")
	if _, err := d.ConvertSpill(ctx, 1, 10, 0, true); !errors.Is(err, ErrSpill) {
		t.Fatalf("formule simple : %v", err)
	}
	if _, err := d.ConvertSpill(ctx, 1, 11, 0, false); !errors.Is(err, ErrSpill) {
		t.Fatalf("cellule vide : %v", err)
	}
}

// « Convertir en tableau lié » : une requête « formule » charge le
// tableau ; il suit sa source dans la même étape d'annulation qu'une
// modification de celle-ci ; une colonne saisie à sa droite garde ses
// valeurs par clé ; un renommage du tableau source suit dans la formule ;
// l'annulation rend la formule.
func TestConvertSpillLinked(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	spillSource(t, d)
	mustSet(t, d, 1, 0, 5, "Nids")
	mustSet(t, d, 1, 1, 5, "=UNIQUE(Suividesnids[NID suivi])")
	name, err := d.ConvertSpill(ctx, 1, 1, 5, true)
	must(t, err)
	if got, want := tableText(t, d, name), "Nids:text / N1 / N2 / N3"; got != want {
		t.Fatalf("tableau lié :\n got %s\nwant %s", got, want)
	}
	ti := tableInfo(t, d, name)
	if ti.QueryID == 0 || ti.Row != 0 || ti.Col != 5 {
		t.Fatalf("lien : %+v", ti)
	}
	q, err := d.QueryByID(ctx, ti.QueryID)
	must(t, err)
	if !strings.Contains(q.Source, `"kind":"formula"`) || !strings.Contains(q.Source, "UNIQUE(Suividesnids[NID suivi])") {
		t.Fatalf("source : %s", q.Source)
	}
	check(t, d, "tableau lié")
	// Colonnes chargées en lecture seule.
	if _, err := d.SetCell(ctx, 1, 1, 5, "X"); !errors.Is(err, ErrQueryTable) {
		t.Fatalf("colonne chargée modifiée : %v", err)
	}

	// Une colonne saisie à droite : ses valeurs suivent la clé « Nids ».
	mustSet(t, d, 1, 0, 6, "Note")
	mustSet(t, d, 1, 1, 6, "premier")
	mustSet(t, d, 1, 3, 6, "troisième")
	if got, want := tableText(t, d, name), "Nids:text|Note:text / N1|premier / N2| / N3|troisième"; got != want {
		t.Fatalf("colonne saisie :\n got %s\nwant %s", got, want)
	}

	// La source change : N0 en tête, N2 disparaît. Le tableau suit aussitôt.
	before := tableText(t, d, name)
	mustSet(t, d, 1, 1, 0, "N0")
	mustSet(t, d, 1, 2, 0, "N3")
	if got, want := tableText(t, d, name), "Nids:text|Note:text / N0| / N3|troisième / N1|premier"; got != want {
		t.Fatalf("après changement de la source :\n got %s\nwant %s", got, want)
	}
	check(t, d, "recalcul")
	// Une étape d'annulation : la saisie et le recalcul ensemble.
	_, err = d.bus.Undo(ctx)
	must(t, err)
	if got, want := tableText(t, d, name), "Nids:text|Note:text / N0| / N2| / N1|premier / N3|troisième"; got != want {
		t.Fatalf("annulation d'une saisie :\n got %s\nwant %s", got, want)
	}
	_, err = d.bus.Undo(ctx)
	must(t, err)
	if got := tableText(t, d, name); got != before {
		t.Fatalf("deuxième annulation :\n got %s\nwant %s", got, before)
	}
	_, err = d.bus.Redo(ctx)
	must(t, err)
	_, err = d.bus.Redo(ctx)
	must(t, err)
	if got := tableText(t, d, name); !strings.HasPrefix(got, "Nids:text|Note:text / N0| / N3|troisième") {
		t.Fatalf("rétablissement : %s", got)
	}
	check(t, d, "rétablissement")

	// Une ligne ajoutée à la source.
	mustSet(t, d, 1, 5, 0, "N9")
	if got := tableText(t, d, name); !strings.HasSuffix(got, "N9|") {
		t.Fatalf("ligne ajoutée : %s", got)
	}

	// Le tableau source renommé : la formule suit, le lien reste vivant.
	must(t, d.RenameTable(ctx, "Suividesnids", "Suivi"))
	q, err = d.QueryByID(ctx, ti.QueryID)
	must(t, err)
	if !strings.Contains(q.Source, "UNIQUE(Suivi[NID suivi])") {
		t.Fatalf("formule après renommage : %s", q.Source)
	}
	mustSet(t, d, 1, 5, 0, "N8")
	if got := tableText(t, d, name); !strings.Contains(got, "N8|") {
		t.Fatalf("après renommage : %s", got)
	}

	// Tout annuler jusqu'à la conversion : la formule revient.
	for {
		if _, err := d.bus.Undo(ctx); err != nil {
			break
		}
		if c := cellOn(t, d, 1, 1, 5); c.Raw == "=UNIQUE(Suividesnids[NID suivi])" {
			break
		}
	}
	if c := cellOn(t, d, 1, 1, 5); c.Raw != "=UNIQUE(Suividesnids[NID suivi])" || len(c.Spill) != 3 {
		t.Fatalf("formule rendue : %+v", c)
	}
	qs, err := d.Queries(ctx)
	must(t, err)
	if len(qs) != 0 {
		t.Fatalf("requête restée : %+v", qs)
	}
	check(t, d, "annulation de la conversion")
}

// Un tableau lié lu par une autre formule liée : le recalcul se propage.
func TestConvertSpillChained(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	spillSource(t, d)
	mustSet(t, d, 1, 0, 5, "Nids")
	mustSet(t, d, 1, 1, 5, "=UNIQUE(Suividesnids[NID suivi])")
	_, err := d.ConvertSpill(ctx, 1, 1, 5, true)
	must(t, err)
	mustSet(t, d, 1, 0, 8, "Tri")
	mustSet(t, d, 1, 1, 8, "=TRIER(Nids[Nids]; 1; -1)")
	name, err := d.ConvertSpill(ctx, 1, 1, 8, true)
	must(t, err)
	if got, want := tableText(t, d, name), "Tri:text / N3 / N2 / N1"; got != want {
		t.Fatalf("second tableau :\n got %s\nwant %s", got, want)
	}
	mustSet(t, d, 1, 5, 0, "N7")
	if got, want := tableText(t, d, name), "Tri:text / N7 / N3 / N2 / N1"; got != want {
		t.Fatalf("propagation :\n got %s\nwant %s", got, want)
	}
	check(t, d, "propagation")
}
