package canvas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// handQuery charge une requête « Q » (en-têtes headers) en A1 de la feuille 1
// et rend la fonction qui l'actualise avec d'autres lignes (avertissements
// rendus).
func handQuery(t *testing.T, d *fixture, headers []string, rows [][]any) func(rows [][]any) []string {
	t.Helper()
	ctx := context.Background()
	id, err := d.SaveQuery(ctx, QueryDef{Name: "Q", Source: `{"kind":"table","table":"X"}`})
	must(t, err)
	load := func(rows [][]any) []string {
		t.Helper()
		it := resultTable("", headers, rows)
		it.At = &QueryAnchor{Sheet: 1}
		l, err := d.LoadQuery(ctx, id, it)
		must(t, err)
		check(t, d, "actualisation")
		return l.Warnings
	}
	load(rows)
	return load
}

// handRows lit le tableau Q : une ligne par ligne du tableau, valeurs
// séparées par « | ».
func handRows(t *testing.T, d *fixture) string {
	t.Helper()
	var out []string
	var cols []ExportColumn
	err := d.ExportTable(context.Background(), "Q", func(c []ExportColumn) error {
		cols = c
		var h []string
		for _, c := range c {
			h = append(h, c.Name)
		}
		out = append(out, strings.Join(h, "|"))
		return nil
	}, func(vals []any) error {
		var r []string
		for i, v := range vals {
			r = append(r, fmt.Sprint(formulaText(v, cols[i].Type)))
		}
		out = append(out, strings.Join(r, "|"))
		return nil
	})
	must(t, err)
	return strings.Join(out, " / ")
}

func formulaText(v any, typ grid.Type) string {
	switch v := v.(type) {
	case nil:
		return ""
	case float64:
		return formula.FormatNumber(v)
	}
	return fmt.Sprint(v)
}

// handState lit le tableau Q et l'état de ses colonnes saisies (sans les
// corbeilles, qui restent après une annulation).
func handState(t *testing.T, d *fixture) string {
	t.Helper()
	out := []string{handRows(t, d)}
	err := d.read(context.Background(), func(tx *tx) error {
		for _, q := range []string{
			`SELECT owner || ' ' || key FROM _lattis_hand ORDER BY owner`,
			`SELECT owner || ' ' || h FROM _lattis_hand_cols ORDER BY owner, h`,
			`SELECT owner || ' ' || k || ' ' || h || ' ' || coalesce(v, '') FROM _lattis_hand_kept ORDER BY owner, k, h`,
		} {
			rs, err := tx.query(q)
			if err != nil {
				return err
			}
			for rs.Next() {
				var s string
				if err := rs.Scan(&s); err != nil {
					rs.Close()
					return err
				}
				out = append(out, s)
			}
			rs.Close()
		}
		return nil
	})
	must(t, err)
	return strings.Join(out, "\n")
}

func handInfoOf(t *testing.T, d *fixture) TableInfo {
	t.Helper()
	l, err := d.Layout(context.Background(), 1)
	must(t, err)
	for _, tb := range l.Tables {
		if tb.Name == "Q" {
			return tb
		}
	}
	t.Fatal("tableau Q introuvable")
	return TableInfo{}
}

// Colonnes saisies à droite d'un tableau chargé (décision 0008) : jointes
// par la colonne « id » du résultat, gardées à l'actualisation, mises de
// côté quand leur ligne disparaît et rattachées quand elle revient ; tout
// s'annule et se rétablit.
func TestHandColumns(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	headers := []string{"id", "Produit", "Montant"}
	refresh := handQuery(t, d, headers, [][]any{{int64(1), "Pommes", int64(120)}, {int64(2), "Poires", int64(80)}, {int64(3), "Prunes", int64(50)}})

	// Un nom à droite de l'en-tête : colonne saisie (en-tête vert).
	mustSet(t, d, 1, 0, 3, "Note")
	ti := handInfoOf(t, d)
	if !ti.Columns[3].Hand || ti.Columns[0].Hand || ti.HandKey != "" {
		t.Fatalf("colonne saisie : %+v", ti)
	}
	// Valeurs : la clé « id » est prise d'office.
	mustSet(t, d, 1, 1, 3, "bon")
	mustSet(t, d, 1, 3, 3, "à revoir")
	if ti = handInfoOf(t, d); ti.HandKey != "id" {
		t.Fatalf("clé : %q", ti.HandKey)
	}
	// Les colonnes chargées restent en lecture seule.
	if _, err := d.SetCell(ctx, 1, 1, 1, "Coings"); !errors.Is(err, ErrQueryTable) {
		t.Fatalf("colonne chargée modifiée : %v", err)
	}
	// Une formule de la feuille voit la colonne saisie.
	mustSet(t, d, 1, 6, 0, "=NBVAL(Q[Note])")
	if v := cellOn(t, d, 1, 6, 0).Value; v != "2" {
		t.Fatalf("NBVAL(Q[Note]) : %q", v)
	}

	// Lignes dans un autre ordre, Poires partie, Kiwis arrivé.
	refresh([][]any{{int64(3), "Prunes", int64(55)}, {int64(1), "Pommes", int64(130)}, {int64(4), "Kiwis", int64(10)}})
	if got, want := handRows(t, d), "id|Produit|Montant|Note / 3|Prunes|55|à revoir / 1|Pommes|130|bon / 4|Kiwis|10|"; got != want {
		t.Fatalf("après actualisation :\n got %s\nwant %s", got, want)
	}
	// Pommes disparaît : sa note est mise de côté…
	before := handState(t, d)
	w := refresh([][]any{{int64(3), "Prunes", int64(55)}, {int64(4), "Kiwis", int64(10)}})
	if len(w) != 1 || !strings.Contains(w[0], "mises de côté") {
		t.Fatalf("avertissements : %q", w)
	}
	if ti = handInfoOf(t, d); ti.HandKept != 1 {
		t.Fatalf("mises de côté : %d", ti.HandKept)
	}
	cols, kept, err := d.HandKept(ctx, "Q")
	must(t, err)
	if fmt.Sprint(cols, kept) != "[Note] [[1 bon]]" {
		t.Fatalf("liste des valeurs de côté : %v %v", cols, kept)
	}
	// … et revient avec elle.
	after := handState(t, d)
	refresh([][]any{{int64(1), "Pommes", int64(140)}, {int64(3), "Prunes", int64(55)}})
	if got, want := handRows(t, d), "id|Produit|Montant|Note / 1|Pommes|140|bon / 3|Prunes|55|à revoir"; got != want {
		t.Fatalf("ligne revenue :\n got %s\nwant %s", got, want)
	}
	if ti = handInfoOf(t, d); ti.HandKept != 0 {
		t.Fatalf("encore de côté : %d", ti.HandKept)
	}
	// Annuler, rétablir.
	reloaded := handState(t, d)
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := handState(t, d); got != after {
		t.Fatalf("annulation :\n%s", diff(after, got))
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := handState(t, d); got != before {
		t.Fatalf("deuxième annulation :\n%s", diff(before, got))
	}
	for range 2 {
		if _, err := d.bus.Redo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := handState(t, d); got != reloaded {
		t.Fatalf("rétablissement :\n%s", diff(reloaded, got))
	}
	check(t, d, "rétablissement")

	// Clé en double : les valeurs de cette clé sont mises de côté.
	w = refresh([][]any{{int64(1), "Pommes", int64(140)}, {int64(1), "Pommes bis", int64(1)}, {int64(3), "Prunes", int64(55)}})
	if got, want := handRows(t, d), "id|Produit|Montant|Note / 1|Pommes|140| / 1|Pommes bis|1| / 3|Prunes|55|à revoir"; got != want || len(w) != 1 || !strings.Contains(w[0], "en double") {
		t.Fatalf("clé en double :\n got %s %q\nwant %s", got, w, want)
	}
	refresh([][]any{{int64(1), "Pommes", int64(140)}, {int64(3), "Prunes", int64(55)}})
	if got := handRows(t, d); !strings.Contains(got, "Pommes|140|bon") {
		t.Fatalf("doublon résolu : %s", got)
	}

	// Colonne saisie renommée : ses valeurs la suivent.
	mustSet(t, d, 1, 0, 3, "Avis")
	refresh([][]any{{int64(3), "Prunes", int64(55)}, {int64(1), "Pommes", int64(140)}})
	if got, want := handRows(t, d), "id|Produit|Montant|Avis / 3|Prunes|55|à revoir / 1|Pommes|140|bon"; got != want {
		t.Fatalf("renommée :\n got %s\nwant %s", got, want)
	}
	// La requête rend maintenant une colonne du même nom : la colonne
	// saisie devient « Avis 2 ».
	refresh2 := func(rows [][]any) []string {
		it := resultTable("", []string{"id", "Produit", "Montant", "Avis"}, rows)
		l, err := d.LoadQuery(ctx, ti.QueryID, it)
		must(t, err)
		return l.Warnings
	}
	w = refresh2([][]any{{int64(1), "Pommes", int64(140), "ok"}})
	if got, want := handRows(t, d), "id|Produit|Montant|Avis|Avis 2 / 1|Pommes|140|ok|bon"; got != want || len(w) == 0 {
		t.Fatalf("nom pris :\n got %s %q\nwant %s", got, w, want)
	}
	// Colonne saisie supprimée : oubliée à l'actualisation suivante, avec
	// ses valeurs mises de côté (Prunes).
	must(t, d.DeleteColumn(ctx, "Q", 4))
	refresh([][]any{{int64(1), "Pommes", int64(140)}})
	if got := handRows(t, d); got != "id|Produit|Montant / 1|Pommes|140" {
		t.Fatalf("colonne supprimée : %s", got)
	}
	if ti = handInfoOf(t, d); ti.HandKept != 0 {
		t.Fatalf("valeurs d'une colonne supprimée gardées : %d", ti.HandKept)
	}
	check(t, d, "fin")
}

// Sans colonne « id » ni clé : la première valeur saisie demande la clé de
// jointure (colonnes possibles, ou le rang) ; la réponse (WithKeyChoice
// « hand:… ») la pose dans la même étape.
func TestHandKeyQuestion(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	refresh := handQuery(t, d, []string{"Produit", "Rayon"}, [][]any{{"Pommes", "Fruits"}, {"Poires", "Fruits"}})
	mustSet(t, d, 1, 0, 2, "Note")
	before := handState(t, d)
	_, err := d.SetCell(ctx, 1, 1, 2, "bon")
	var q ErrHandKey
	if !errors.As(err, &q) || fmt.Sprint(q.Columns) != "[Produit]" || !strings.HasPrefix(err.Error(), "jointure à choisir : « Q »") || !strings.Contains(err.Error(), "Colonnes possibles : « Produit ».") {
		t.Fatalf("question : %v", err)
	}
	if got := handState(t, d); got != before {
		t.Fatalf("question : rien ne doit changer\n%s", diff(before, got))
	}
	// Une colonne avec doublons est refusée.
	if _, err := d.SetCell(WithKeyChoice(ctx, "hand:Rayon"), 1, 1, 2, "bon"); err == nil || !strings.Contains(err.Error(), "double") {
		t.Fatalf("clé en double acceptée : %v", err)
	}
	if _, err := d.SetCell(WithKeyChoice(ctx, "hand:Produit"), 1, 1, 2, "bon"); err != nil {
		t.Fatal(err)
	}
	if ti := handInfoOf(t, d); ti.HandKey != "Produit" {
		t.Fatalf("clé : %q", ti.HandKey)
	}
	refresh([][]any{{"Kiwis", "Exotiques"}, {"Pommes", "Fruits"}})
	if got, want := handRows(t, d), "Produit|Rayon|Note / Kiwis|Exotiques| / Pommes|Fruits|bon"; got != want {
		t.Fatalf("jointure par Produit :\n got %s\nwant %s", got, want)
	}

	// Rang des lignes : la valeur reste sur la première ligne, quelle
	// qu'elle soit ; une ligne en moins met de côté ce qu'elle portait.
	n, err := d.SetHandKey(ctx, "Q", "#")
	must(t, err)
	if n != 0 {
		t.Fatalf("oubliées : %d", n)
	}
	mustSet(t, d, 1, 1, 2, "premier")
	mustSet(t, d, 1, 2, 2, "second")
	w := refresh([][]any{{"Poires", "Fruits"}})
	if got, want := handRows(t, d), "Produit|Rayon|Note / Poires|Fruits|premier"; got != want || len(w) != 1 {
		t.Fatalf("par rang :\n got %s %q\nwant %s", got, w, want)
	}
	if n, err = d.ForgetHandKept(ctx, "Q"); err != nil || n != 1 {
		t.Fatalf("oublier : %d %v", n, err)
	}
	if ti := handInfoOf(t, d); ti.HandKept != 0 {
		t.Fatalf("encore de côté : %d", ti.HandKept)
	}
	check(t, d, "fin")
}

// Résultat d'un UNION de tableaux annuels : les mêmes id d'une année à
// l'autre. Aucune colonne seule n'identifie les lignes : la question propose
// des paires, et « id + Année » joint les valeurs saisies.
func TestHandCompositeKey(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	headers := []string{"id", "Espèce", "Année"}
	refresh := handQuery(t, d, headers, [][]any{{int64(1), "Mésange", int64(2021)}, {int64(1), "Mésange", int64(2022)}, {int64(2), "Merle", int64(2021)}})
	mustSet(t, d, 1, 0, 3, "Remarque")
	_, err := d.SetCell(ctx, 1, 2, 3, "nid détruit")
	var q ErrHandKey
	if !errors.As(err, &q) || fmt.Sprint(q.Columns) != "[id + Année Espèce + Année]" {
		t.Fatalf("question : %v", err)
	}
	if _, err := d.SetCell(WithKeyChoice(ctx, "hand:id + Année"), 1, 2, 3, "nid détruit"); err != nil {
		t.Fatal(err)
	}
	if ti := handInfoOf(t, d); ti.HandKey != "id + Année" {
		t.Fatalf("clé : %q", ti.HandKey)
	}
	refresh([][]any{{int64(1), "Mésange", int64(2022)}, {int64(2), "Merle", int64(2021)}, {int64(1), "Mésange", int64(2021)}})
	if got, want := handRows(t, d), "id|Espèce|Année|Remarque / 1|Mésange|2022|nid détruit / 2|Merle|2021| / 1|Mésange|2021|"; got != want {
		t.Fatalf("jointure id + Année :\n got %s\nwant %s", got, want)
	}
	// Une paire avec doublons est refusée.
	if _, err := d.SetHandKey(ctx, "Q", "id + Espèce"); err == nil || !strings.Contains(err.Error(), "double") {
		t.Fatalf("paire en double acceptée : %v", err)
	}
}

// Une colonne calculée ajoutée à droite d'un tableau chargé reste calculée
// à l'actualisation et ne demande pas de clé.
func TestHandComputed(t *testing.T) {
	d := open(t)
	refresh := handQuery(t, d, []string{"Produit", "Montant"}, [][]any{{"Pommes", int64(120)}, {"Poires", int64(80)}})
	mustSet(t, d, 1, 0, 2, "Double")
	mustSet(t, d, 1, 1, 2, "=[Montant]*2")
	refresh([][]any{{"Kiwis", int64(5)}})
	if got, want := handRows(t, d), "Produit|Montant|Double / Kiwis|5|10"; got != want {
		t.Fatalf("calculée :\n got %s\nwant %s", got, want)
	}
	if ti := handInfoOf(t, d); !ti.Columns[2].Hand || ti.HandKey != "" {
		t.Fatalf("calculée : %+v", ti)
	}
}
