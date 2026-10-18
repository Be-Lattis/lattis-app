package canvas

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
)

// dumpData est le dump sans les corbeilles de requêtes, qui restent en place
// après une annulation (rétablir s'en sert).
func dumpData(t *testing.T, d *fixture) string {
	t.Helper()
	var keep []string
	skip := false
	for _, l := range strings.Split(dump(t, d), "\n") {
		// Le SQL d'une table créée par CREATE TABLE ... AS tient sur plusieurs lignes.
		if strings.HasPrefix(l, "## ") || strings.Contains(l, " | ") && !strings.HasPrefix(l, " ") {
			skip = strings.Contains(l, sqlTrash)
		}
		if !skip {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

func sqlFixture(t *testing.T) *fixture {
	d := open(t)
	ctx := context.Background()
	for _, s := range []struct {
		row, col int
		raw      string
	}{
		{0, 0, "Client"}, {0, 1, "Montant"}, {0, 2, "Payé"},
		{1, 0, "Durand"}, {1, 1, "120"}, {1, 2, "vrai"},
		{2, 0, "Martin"}, {2, 1, "80,5"}, {2, 2, "faux"},
		{3, 0, "Durand"}, {3, 1, "40"}, {3, 2, "faux"},
		// Un deuxième tableau juste sous la ligne grisée du premier.
		{5, 0, "Produit"}, {6, 0, "Vis"},
	} {
		if _, err := d.SetCell(ctx, 1, s.row, s.col, s.raw); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

// Chaque requête s'annule et se rétablit exactement, y compris quand son
// résultat dépend du hasard, et les tableaux qui grandissent décalent ceux
// du dessous.
func TestExecSQLUndoRedo(t *testing.T) {
	ctx := context.Background()
	for i, tc := range []struct {
		q     string
		n     int64
		check string
	}{
		{`UPDATE Feuille1 SET Montant = Montant * 2 WHERE Client = 'Durand'`, 2, "Durand|240"},
		{`INSERT INTO Feuille1 (Client, Montant) VALUES ('Petit', 1), ('Grand', 2)`, 2, "Grand|2"},
		{`DELETE FROM Feuille1 WHERE Montant < 100`, 2, ""},
		{`UPDATE Feuille1 SET Montant = abs(random() % 1000)`, 3, ""},
		{`UPDATE t_1 SET c_1 = upper(c_1)`, 3, "DURAND"},
		{`INSERT INTO Feuille1 (Client) SELECT Produit FROM Tableau1`, 1, "Vis"},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc.q)
			d := sqlFixture(t)
			before := dumpData(t, d)
			n, err := d.ExecSQL(ctx, tc.q)
			if err != nil {
				t.Fatal(err)
			}
			if n != tc.n {
				t.Errorf("%d lignes touchées, attendu %d", n, tc.n)
			}
			check(t, d, "après la requête")
			if v := view(t, d); !strings.Contains(v, tc.check) {
				t.Errorf("vue :\n%s", v)
			}
			after := dumpData(t, d)
			if _, err := d.bus.Undo(ctx); err != nil {
				t.Fatal(err)
			}
			check(t, d, "après Ctrl+Z")
			if got := dumpData(t, d); got != before {
				t.Fatalf("Ctrl+Z :\n%s", diff(before, got))
			}
			if _, err := d.bus.Redo(ctx); err != nil {
				t.Fatal(err)
			}
			check(t, d, "après Ctrl+Y")
			if got := dumpData(t, d); got != after {
				t.Fatalf("Ctrl+Y :\n%s", diff(after, got))
			}
		})
	}
}

func TestExecSQLMovesTableBelow(t *testing.T) {
	d := sqlFixture(t)
	if _, err := d.ExecSQL(context.Background(), `INSERT INTO Feuille1 (Client) VALUES ('a'), ('b'), ('c')`); err != nil {
		t.Fatal(err)
	}
	check(t, d, "après l'ajout")
	v := view(t, d)
	if !strings.Contains(v, "c\n\n[Produit]") {
		t.Fatalf("le tableau du dessous n'a pas été décalé :\n%s", v)
	}
}

func TestExecSQLRefused(t *testing.T) {
	ctx := context.Background()
	d := sqlFixture(t)
	before := dumpData(t, d)
	for q, want := range map[string]string{
		`CREATE TABLE x (a)`:                              "structure",
		`DROP TABLE t_1`:                                  "structure",
		`DELETE FROM _lattis_tables`:                      "internes",
		`UPDATE Feuille1 SET Inconnue = 1`:                "Inconnue",
		`UPDATE Feuille1 SET Montant = 1; DROP TABLE t_2`: "structure",
	} {
		_, err := d.ExecSQL(ctx, q)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s : erreur %v, attendu %q", q, err, want)
		}
	}
	if _, err := d.ExecSQL(ctx, `CREATE TABLE x (a)`); !errors.Is(err, ErrStructure) {
		t.Errorf("ErrStructure attendu : %v", err)
	}
	if got := dumpData(t, d); got != before {
		t.Fatalf("une requête refusée a laissé des traces :\n%s", diff(before, got))
	}
}

// Une corbeille dont la commande a été annulée puis abandonnée par une
// nouvelle action est supprimée à la requête suivante.
func TestExecSQLOrphanTrash(t *testing.T) {
	ctx := context.Background()
	d := sqlFixture(t)
	count := func() int {
		var n int
		if err := d.db.Read().QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name LIKE '_lattis_sqltrash_%'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if _, err := d.ExecSQL(ctx, `UPDATE Feuille1 SET Montant = 1`); err != nil {
		t.Fatal(err)
	}
	if count() != 3 {
		t.Fatalf("%d tables de corbeille", count())
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 10, 10, "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ExecSQL(ctx, `UPDATE Feuille1 SET Montant = 2`); err != nil {
		t.Fatal(err)
	}
	if count() != 3 {
		t.Fatalf("%d tables de corbeille, l'ancienne n'a pas été supprimée", count())
	}
}

// Une valeur écrite en SQL remplace une saisie invalide.
func TestExecSQLClearsInvalid(t *testing.T) {
	ctx := context.Background()
	d := sqlFixture(t)
	if err := d.FixType(ctx, "Feuille1", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 1, "beaucoup"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ExecSQL(ctx, `UPDATE Feuille1 SET Montant = 5 WHERE Client = 'Durand'`); err != nil {
		t.Fatal(err)
	}
	check(t, d, "après la requête")
	var n int
	if err := d.db.Read().QueryRow(`SELECT count(*) FROM _lattis_invalid`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("%d saisies invalides restantes (%v)", n, err)
	}
}

// Une valeur hors type dans une requête élargit la colonne, comme une saisie.
func TestExecSQLWidens(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nom", "Qté"}, {"a", "1"}, {"b", "2"}}); err != nil {
		t.Fatal(err)
	}
	before := dumpData(t, d)
	if _, err := d.ExecSQL(ctx, `INSERT INTO "Feuille1"("Nom", "Qté") VALUES ('c', 2.5)`); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	if typ := l.Tables[0].Columns[1].Type; typ != "decimal" || l.Tables[0].Rows != 3 {
		t.Fatalf("élargissement : %s, %d lignes", typ, l.Tables[0].Rows)
	}
	if _, err := d.ExecSQL(ctx, `UPDATE "Feuille1" SET "Qté" = 'beaucoup' WHERE "Nom" = 'a'`); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, 1)
	if typ := l.Tables[0].Columns[1].Type; typ != "text" {
		t.Fatalf("texte : %s", typ)
	}
	check(t, d, "élargissement SQL")
	d.bus.Undo(ctx)
	d.bus.Undo(ctx)
	if got := dumpData(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
	if _, err := d.Editor("x").Insert(ctx, "Inconnu", map[string]string{"a": "1"}); err == nil || !strings.Contains(err.Error(), "introuvable") {
		t.Fatalf("tableau inconnu : %v", err)
	}
}
