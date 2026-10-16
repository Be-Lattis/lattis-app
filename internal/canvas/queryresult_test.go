package canvas

import (
	"context"
	"fmt"
	"testing"
)

func TestQueryToSheet(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes", "Date"}, {"FR", "10", "15/01/2026"}, {"IT", "5", ""}, {"FR", "7", "16/01/2026"}}); err != nil {
		t.Fatal(err)
	}
	q := `SELECT c_1, sum(c_2), max(c_3), avg(c_2), c_1 FROM t_1 GROUP BY c_1 ORDER BY c_1`
	res, err := d.QueryToSheet(ctx, q, "", []string{"Pays", "Total", "Dernière", "Moyenne", "Pays"})
	if err != nil {
		t.Fatal(err)
	}
	check(t, d, "résultat vers une feuille")
	if len(res.Sheets) != 1 || len(res.Tables) != 1 || res.Tables[0] != "Requête1" {
		t.Fatalf("résultat : %+v", res)
	}
	l, _ := d.Layout(ctx, res.Sheets[0])
	if l.Sheets[1].Name != "Requête1" {
		t.Fatalf("onglets : %+v", l.Sheets)
	}
	tb := l.Tables[0]
	var cols []string
	for _, c := range tb.Columns {
		cols = append(cols, c.Name+":"+string(c.Type))
	}
	if got := fmt.Sprint(cols); got != "[Pays:text Total:integer Dernière:date Moyenne:decimal Pays 2:text]" || tb.Rows != 2 {
		t.Fatalf("tableau : %s, %d lignes", got, tb.Rows)
	}
	// Un deuxième envoi prend le nom suivant ; un nom donné est repris.
	res2, err := d.QueryToSheet(ctx, q, "", nil)
	if err != nil || res2.Tables[0] != "Requête2" {
		t.Fatalf("deuxième envoi : %+v %v", res2, err)
	}
	res3, err := d.QueryToSheet(ctx, q, "Ventes par pays", nil)
	if err != nil || res3.Tables[0] != "Ventesparpays" {
		t.Fatalf("envoi nommé : %+v %v", res3, err)
	}
	if _, err := d.QueryToSheet(ctx, "", "", nil); err == nil {
		t.Fatal("requête vide acceptée")
	}
	// L'annulation retire feuilles et tableaux (les tables de chargement
	// restent, pour rétablir, comme pour un import).
	for range 3 {
		d.bus.Undo(ctx)
	}
	check(t, d, "annulation")
	if l, _ := d.Layout(ctx, 1); len(l.Sheets) != 1 || len(l.Tables) != 1 {
		t.Fatalf("après annulation : %+v", l.Sheets)
	}
	if names, _ := d.TableNames(ctx); len(names) != 1 {
		t.Fatalf("tableaux après annulation : %v", names)
	}
}
