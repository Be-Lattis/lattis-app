package canvas

import (
	"context"
	"strings"
	"testing"
)

// tableKey rend le rang de la clé du premier tableau de la feuille 1 (-1 sans
// clé) et celui de sa colonne numérotée d'office.
func tableKey(t *testing.T, d *fixture) (key, auto int) {
	t.Helper()
	ts := tablesOf(t, d)
	if len(ts) == 0 {
		t.Fatal("aucun tableau")
	}
	return ts[0].Key, ts[0].AutoNumber
}

// firstCol rend les valeurs de la première colonne du premier tableau, dans
// l'ordre des lignes.
func firstCol(t *testing.T, d *fixture) string {
	t.Helper()
	var out []string
	err := d.read(context.Background(), func(x *tx) error {
		tbs, err := x.tables(1)
		if err != nil {
			return err
		}
		tb := tbs[0]
		rs, err := x.query(`SELECT coalesce(CAST(` + colSQL(tb.Cols[0].ID) + ` AS TEXT), '') FROM ` + tb.sql() + ` ORDER BY _pos`)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var s string
			if err := rs.Scan(&s); err != nil {
				return err
			}
			out = append(out, s)
		}
		return rs.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(out, ",")
}

// Un tableau collé dont la première colonne s'appelle ID (entiers uniques)
// reçoit sa clé dans la même étape ; Ctrl+Z en deux temps comme avant.
func TestAutoKeyIDHeader(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	before := dump(t, d)
	ev, err := d.Paste(ctx, 1, 0, 0, [][]string{{"id", "Nom"}, {"5", "Ana"}, {"9", "Bob"}})
	if err != nil {
		t.Fatal(err)
	}
	if ev.AutoKey != "id" || !ev.Split {
		t.Fatalf("événement : %+v", ev)
	}
	if k, a := tableKey(t, d); k != 0 || a != 0 {
		t.Fatalf("clé %d, numérotée %d", k, a)
	}
	check(t, d, "clé posée")
	// Premier Ctrl+Z : cellules libres (sans clé ni tableau) ; second : rien.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, d, "création défaite")
	if got := layoutOf(t, d); got != "A1=id B1=Nom A2=5 B2=Ana A3=9 B3=Bob" {
		t.Fatalf("création défaite : %s", got)
	}
	undoOnce(t, d, before, "collage")
	// Rétablir : la saisie, puis le tableau et sa clé.
	for range 2 {
		if _, err := d.bus.Redo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if k, _ := tableKey(t, d); k != 0 {
		t.Fatalf("rétabli : clé %d", k)
	}
	check(t, d, "rétabli")
}

// Tapé au clavier : le tableau naît avec sa clé, le premier Ctrl+Z laisse
// les cellules libres, le second annule la dernière saisie.
func TestAutoKeyTyped(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	typeIn(t, d, 0, 0, [][]string{{"ID", "Nom"}})
	beforeLast := dump(t, d)
	// La 3e cellule du carré 2 × 2 fait naître le tableau.
	ev, err := d.SetCell(ctx, 1, 1, 0, "1")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Created == "" || ev.AutoKey != "ID" || !ev.Split {
		t.Fatalf("événement : %+v", ev)
	}
	if k, _ := tableKey(t, d); k != 0 {
		t.Fatalf("clé %d", k)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, d, "création défaite")
	if got := layoutOf(t, d); got != "A1=ID B1=Nom A2=1" {
		t.Fatalf("création défaite : %s", got)
	}
	undoOnce(t, d, beforeLast, "saisie")
}

// 1, 2, 3… dans la première colonne : la clé, quel que soit l'en-tête.
// Autres cas : pas de clé.
func TestAutoKeySequence(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		rows [][]string
		key  bool
	}{
		{[][]string{{"N°", "Nom"}, {"1", "Ana"}, {"2", "Bob"}, {"3", "Cy"}}, true},
		{[][]string{{"ID", "Nom"}, {"1", "Ana"}}, true},
		{[][]string{{"Rang", "Nom"}, {"2", "Ana"}, {"3", "Bob"}}, false}, // ne part pas de 1
		{[][]string{{"Rang", "Nom"}, {"1", "Ana"}, {"3", "Bob"}}, false}, // un trou
		{[][]string{{"Rang", "Nom"}, {"1", "Ana"}, {"0", "Bob"}}, false}, // pas croissant
		{[][]string{{"Rang", "Nom"}, {"1", "Ana"}}, false},               // un 1 seul
		{[][]string{{"ID", "Nom"}, {"4", "Ana"}, {"4", "Bob"}}, false},   // doublon
		{[][]string{{"ID", "Nom"}, {"A1", "Ana"}, {"A2", "Bob"}}, false}, // texte
		{[][]string{{"ID", "Nom"}, {"1,5", "Ana"}, {"2", "Bob"}}, false}, // décimal
		{[][]string{{"Nom", "ID"}, {"Ana", "1"}, {"Bob", "2"}}, false},   // pas la première colonne
		{[][]string{{"Code", "Nom"}, {"1", "Ana"}, {"2", "Bob"}}, true},  // 1, 2
		{[][]string{{"Identifiant", "Nom"}, {"7", "Ana"}, {"8", "Bob"}}, false},
	} {
		d := open(t)
		if _, err := d.Paste(ctx, 1, 0, 0, c.rows); err != nil {
			t.Fatal(err)
		}
		if k, _ := tableKey(t, d); (k == 0) != c.key {
			t.Errorf("%v : clé %d", c.rows, k)
		}
	}
}

// Mettre sous forme de tableau : même règle, dite dans la barre d'état.
func TestAutoKeyFormatAsTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"ID", "Nom"}, {"1", "Ana"}, {"2", "Bob"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.ConvertToRange(ctx, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	d.TakeNotice()
	if _, err := d.FormatAsTable(ctx, 1, 1, 0); err != nil {
		t.Fatal(err)
	}
	if k, _ := tableKey(t, d); k != 0 {
		t.Fatalf("clé %d", k)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "Clé posée sur ID (Ctrl+Z pour annuler)") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "mise sous forme de tableau")
}

// Une ligne ajoutée à un tableau dont la clé est un entier reçoit max + 1 :
// saisie dessous, insertion au milieu (pas de renumérotation), collage de
// lignes sans la colonne ; une valeur tapée dans la colonne l'emporte.
func TestAutoNumberKey(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"ID", "Nom"}, {"1", "Ana"}, {"2", "Bob"}, {"3", "Cy"}}); err != nil {
		t.Fatal(err)
	}
	if _, a := tableKey(t, d); a != 0 {
		t.Fatalf("colonne numérotée : %d", a)
	}
	// Saisie sous le tableau, dans la 2e colonne.
	before := dump(t, d)
	mustSet(t, d, 1, 4, 1, "Dan")
	if got := firstCol(t, d); got != "1,2,3,4" {
		t.Fatalf("saisie dessous : %s", got)
	}
	check(t, d, "saisie dessous")
	undoOnce(t, d, before, "saisie dessous")
	mustSet(t, d, 1, 4, 1, "Dan")
	// Insertion au milieu : max + 1, les autres gardent leur numéro.
	if err := d.InsertRow(ctx, "Feuille1", 1); err != nil {
		t.Fatal(err)
	}
	if got := firstCol(t, d); got != "1,5,2,3,4" {
		t.Fatalf("insertion : %s", got)
	}
	// Valeur tapée dans la colonne clé : gardée, la suite repart d'elle.
	mustSet(t, d, 1, 6, 0, "10")
	if got := firstCol(t, d); got != "1,5,2,3,4,10" {
		t.Fatalf("valeur tapée : %s", got)
	}
	// Collage de deux lignes sous le tableau, sans la colonne ID.
	before = dump(t, d)
	if _, err := d.Paste(ctx, 1, 7, 1, [][]string{{"Eve"}, {"Fred"}}); err != nil {
		t.Fatal(err)
	}
	if got := firstCol(t, d); got != "1,5,2,3,4,10,11,12" {
		t.Fatalf("collage : %s", got)
	}
	check(t, d, "collage")
	undoOnce(t, d, before, "collage")
	// Ligne ajoutée par l'API (fin du tableau).
	if err := d.InsertRow(ctx, "Feuille1", 6); err != nil {
		t.Fatal(err)
	}
	if got := firstCol(t, d); got != "1,5,2,3,4,10,11" {
		t.Fatalf("ajout à la fin : %s", got)
	}
	// Sans clé : plus de numérotation (suite irrégulière).
	if err := d.ClearKey(ctx, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	if _, a := tableKey(t, d); a != -1 {
		t.Fatalf("sans clé, colonne numérotée %d", a)
	}
	mustSet(t, d, 1, 8, 1, "Gus")
	if got := firstCol(t, d); got != "1,5,2,3,4,10,11," {
		t.Fatalf("sans clé : %s", got)
	}
}

// Première colonne en suite régulière d'entiers sans être la clé (10, 20,
// 30) : une ligne ajoutée à la fin reçoit dernier + pas ; au milieu, rien.
func TestAutoNumberSequence(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Rang", "Nom"}, {"10", "Ana"}, {"20", "Bob"}, {"30", "Cy"}}); err != nil {
		t.Fatal(err)
	}
	if k, a := tableKey(t, d); k != -1 || a != 0 {
		t.Fatalf("clé %d, numérotée %d", k, a)
	}
	before := dump(t, d)
	mustSet(t, d, 1, 4, 1, "Dan")
	if got := firstCol(t, d); got != "10,20,30,40" {
		t.Fatalf("saisie dessous : %s", got)
	}
	undoOnce(t, d, before, "saisie dessous")
	if err := d.InsertRow(ctx, "Feuille1", 1); err != nil {
		t.Fatal(err)
	}
	if got := firstCol(t, d); got != "10,,20,30" {
		t.Fatalf("insertion au milieu : %s", got)
	}
	if _, a := tableKey(t, d); a != -1 {
		t.Fatalf("suite rompue, colonne numérotée %d", a)
	}
}
