package canvas

import (
	"context"
	"fmt"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/store"
)

// cachedHints dit ce que la mise en page du tableau trouverait encore en
// cache, sans rien calculer : la suggestion de clé (contrôle de la clé),
// la numérotation automatique et le nombre de lignes.
func cachedHints(t *testing.T, d *fixture, table string) (hint, num, rows bool) {
	t.Helper()
	err := d.read(context.Background(), func(x *tx) error {
		var id int64
		if err := x.row(`SELECT id FROM _lattis_tables WHERE name = ?`, table).Scan(&id); err != nil {
			return err
		}
		name := fmt.Sprintf("t_%d", id)
		_, rows = peekOn[int](x, "rows:"+name, []string{store.NoMeta, store.CountOf(name)})
		tb, err := x.tableByName(table)
		if err != nil {
			return err
		}
		// Comme un grand tableau : rien n'est calculé hors du cache.
		tb.Rows = quickRows + 1
		_, pending, err := x.keyHintIn(tb, true)
		if err != nil {
			return err
		}
		_, later, err := x.autoNumberedIn(tb, true)
		hint, num = !pending, !later
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return hint, num, rows
}

// Le contrôle de la clé n'est refait que si le nombre de lignes change ou
// si la colonne contrôlée change (demande de Louis du 22/10) : pas après
// l'ajout, la suppression ou la conversion d'une autre colonne, ni leur
// annulation, ni un tri, une saisie ailleurs ou une mise en forme. (Une
// requête de l'onglet SQL passe par des vues et des déclencheurs : le
// Store n'en sait pas les colonnes, elle compte pour tout le tableau.)
func TestKeyHintCacheKept(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"N°", "Nom", "Prix", "Ville"},
		{"3", "Ana", "10", "Lyon"},
		{"7", "Bob", "20", "Paris"},
		{"12", "Cy", "30", "Nice"},
	}); err != nil {
		t.Fatal(err)
	}
	const tbl = "Feuille1"
	prime := func() {
		t.Helper()
		if _, err := d.Layout(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if !keyHintOf(t, d) {
			t.Fatal("valeurs uniques : pas de suggestion")
		}
		if h, n, r := cachedHints(t, d, tbl); !h || !n || !r {
			t.Fatalf("après la mise en page : en cache %v %v %v", h, n, r)
		}
	}
	type step struct {
		what string
		do   func() error
		// order : l'ordre des lignes change (la numérotation est refaite).
		order bool
	}
	undo := func() error { _, err := d.bus.Undo(ctx); return err }
	for _, s := range []step{
		{what: "insertion d'une colonne", do: func() error { return d.InsertColumn(ctx, tbl, 2, "Remise") }},
		{what: "suppression de cette colonne", do: func() error { return d.DeleteColumn(ctx, tbl, 2) }},
		{what: "suppression d'une colonne pleine", do: func() error { return d.DeleteColumn(ctx, tbl, 2) }},
		{what: "annulation de la suppression", do: undo},
		{what: "suppression de deux colonnes", do: func() error {
			return d.DeleteColumnSpans(ctx, []ColumnSpan{{Table: tbl, From: 2, To: 3}})
		}},
		{what: "annulation de la suppression de deux colonnes", do: undo},
		{what: "type d'une autre colonne", do: func() error { return d.SetType(ctx, tbl, 2, grid.Text) }},
		{what: "annulation du type", do: undo},
		{what: "saisie dans une autre colonne", do: func() error { _, err := d.SetCell(ctx, 1, 1, 1, "Anna"); return err }},
		{what: "renommage d'une colonne", do: func() error { _, err := d.SetCell(ctx, 1, 0, 3, "Lieu"); return err }},
		{what: "mise en forme", do: func() error { return d.ApplyStyle(ctx, 1, 1, 0, 3, 3, Style{"b": true}) }},
		{what: "largeur de colonne", do: func() error { return d.SetColumnWidth(ctx, 1, 0, 140) }},
		{what: "tri", do: func() error { return d.SortTable(ctx, tbl, 1, true) }, order: true},
		{what: "annulation du tri", do: undo, order: true},
		// En dernier : la feuille a ensuite deux tableaux (keyHintOf).
		{what: "autre tableau", do: func() error {
			_, err := d.Paste(ctx, 1, 0, 8, [][]string{{"Code", "Qté"}, {"A", "1"}, {"B", "2"}})
			return err
		}},
	} {
		prime()
		if err := s.do(); err != nil {
			t.Fatalf("%s : %v", s.what, err)
		}
		h, n, r := cachedHints(t, d, tbl)
		if !h {
			t.Errorf("%s : contrôle de la clé à refaire", s.what)
		}
		if !r {
			t.Errorf("%s : nombre de lignes à recompter", s.what)
		}
		if n == s.order {
			t.Errorf("%s : numérotation en cache %v", s.what, n)
		}
	}
}

// … et il est refait quand il le faut : lignes ajoutées ou supprimées,
// valeurs de la colonne contrôlée (saisie, collage, recherche, effacement,
// onglet SQL, annulation), colonne contrôlée supprimée ou convertie.
func TestKeyHintCacheDropped(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"N°", "Nom", "Prix"},
		{"3", "Ana", "10"},
		{"7", "Bob", "20"},
		{"12", "Cy", "30"},
	}); err != nil {
		t.Fatal(err)
	}
	const tbl = "Feuille1"
	undo := func() error { _, err := d.bus.Undo(ctx); return err }
	for _, s := range []struct {
		what string
		do   func() error
		// rows : le nombre de lignes change.
		rows bool
	}{
		{what: "saisie dans la première colonne", do: func() error { _, err := d.SetCell(ctx, 1, 1, 0, "4"); return err }},
		{what: "annulation de la saisie", do: undo},
		{what: "collage dans la première colonne", do: func() error {
			_, err := d.Paste(ctx, 1, 2, 0, [][]string{{"8"}})
			return err
		}},
		{what: "remplacement", do: func() error { _, err := d.Replace(ctx, 1, "12", "13"); return err }},
		{what: "effacement", do: func() error { return d.ClearRange(ctx, 1, 3, 0, 3, 0, false) }},
		{what: "annulation de l'effacement", do: undo},
		{what: "onglet SQL : mise à jour de la colonne", do: func() error {
			_, err := d.ExecSQL(ctx, `UPDATE "`+tbl+`" SET "N°" = "N°" + 100 WHERE "Nom" = 'Ana'`)
			return err
		}},
		{what: "onglet SQL : ajout d'une ligne", do: func() error {
			_, err := d.ExecSQL(ctx, `INSERT INTO "`+tbl+`" ("N°", "Nom") VALUES (99, 'Dan')`)
			return err
		}, rows: true},
		{what: "onglet SQL : suppression d'une ligne", do: func() error {
			_, err := d.ExecSQL(ctx, `DELETE FROM "`+tbl+`" WHERE "N°" = 99`)
			return err
		}, rows: true},
		{what: "insertion d'une ligne", do: func() error { return d.InsertRow(ctx, tbl, 1) }, rows: true},
		{what: "suppression d'une ligne", do: func() error { return d.DeleteRow(ctx, tbl, 1) }, rows: true},
		{what: "annulation de la suppression de ligne", do: undo, rows: true},
		{what: "type de la première colonne", do: func() error { return d.SetType(ctx, tbl, 0, grid.Text) }},
		{what: "annulation du type", do: undo},
		{what: "suppression de la première colonne", do: func() error { return d.DeleteColumn(ctx, tbl, 0) }},
		{what: "annulation de la suppression", do: undo},
	} {
		if _, err := d.Layout(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if h, _, _ := cachedHints(t, d, tbl); !h {
			t.Fatalf("avant « %s » : contrôle de la clé pas en cache", s.what)
		}
		if err := s.do(); err != nil {
			t.Fatalf("%s : %v", s.what, err)
		}
		h, _, r := cachedHints(t, d, tbl)
		if h {
			t.Errorf("%s : contrôle de la clé gardé", s.what)
		}
		if s.rows && r {
			t.Errorf("%s : nombre de lignes gardé", s.what)
		}
	}
}
