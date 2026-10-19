package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// commandes pose un tableau aux types variés : texte, entier, décimal,
// booléen, date.
func commandes(h *host) {
	h.t.Helper()
	h.paste(1, 1,
		[]any{"Client", "Nombre", "Montant HT", "Payé", "Échéance"},
		[]any{"Durand", 3, 120.5, true, "01/10/2026"},
		[]any{"Martin", 1, 80, false, "20/10/2026"},
		[]any{"L'Atelier", 2, 40.25, false, "05/09/2026"})
	h.onlyTable("Commandes")
}

// Les types saisis dans la grille arrivent en SQL avec les bons types et
// les noms affichés (accents, espaces, apostrophes).
func TestSQLSeesGridTypes(t *testing.T) {
	h := newHost(t)
	commandes(h)
	tb := h.table("Commandes")
	types := []string{}
	for _, c := range tb.Columns {
		types = append(types, c.Type)
	}
	if got := strings.Join(types, ","); got != "text,integer,decimal,boolean,date" {
		t.Fatalf("types inférés %s", got)
	}
	for sql, want := range map[string]string{
		`SELECT sum("Montant HT") FROM Commandes`:                         "240.75",
		`SELECT sum(Nombre) FROM Commandes WHERE "Payé"`:                  "3",
		`SELECT count(*) FROM Commandes WHERE NOT "Payé"`:                 "2",
		`SELECT Client FROM Commandes WHERE "Échéance" < '2026-10-01'`:    "L'Atelier",
		`SELECT "Échéance" FROM Commandes WHERE Client = 'Durand'`:        "2026-10-01",
		`SELECT typeof(Nombre) || typeof("Montant HT") FROM Commandes`:    "integerreal",
		`SELECT max(_id) FROM Commandes`:                                  "3",
		`WITH x AS (SELECT Client FROM Commandes) SELECT count(*) FROM x`: "3",
	} {
		if got := h.value(sql); got != want {
			t.Errorf("%s = %q, attendu %q", sql, got, want)
		}
	}
	if got := h.value(`SELECT Client FROM Commandes WHERE Client = ?`, "L'Atelier"); got != "L'Atelier" {
		t.Fatalf("paramètre avec apostrophe : %q", got)
	}
}

// Les écritures SQL se voient dans la grille, élargissent les types comme
// une saisie, et s'annulent exactement.
func TestSQLWritesAndUndo(t *testing.T) {
	h := newHost(t)
	commandes(h)
	before := h.snapshot()

	n := h.do("db.execute", "sql", `UPDATE Commandes SET "Payé" = 1, Nombre = Nombre * 10 WHERE Client <> 'Durand'`)
	if asText(n) != "2" {
		t.Fatalf("lignes touchées %v", n)
	}
	h.wantColumn("Commandes", "Nombre", "3", "10", "20")
	h.wantColumn("Commandes", "Payé", "1", "1", "1") // booléens : 1 et 0 en SQL
	h.do("db.execute", "sql", `INSERT INTO Commandes (Client, Nombre) VALUES ('Petit', 'beaucoup')`)
	if got := h.table("Commandes").Columns[1].Type; got != "text" {
		t.Fatalf("Nombre après un texte : %s (élargissement attendu)", got)
	}
	if got := h.table("Commandes").Rows; got != 4 {
		t.Fatalf("lignes %d", got)
	}
	h.do("db.execute", "sql", `DELETE FROM Commandes WHERE Client = 'Martin'`)
	h.wantColumn("Commandes", "Client", "Durand", "L'Atelier", "Petit")

	for i := 0; i < 3; i++ {
		h.undo()
	}
	sameSnapshot(t, "après annulation des requêtes", before, h.snapshot())
}

// Ce que le SQL refuse : structure, tables internes, plusieurs requêtes.
func TestSQLRefusals(t *testing.T) {
	h := newHost(t)
	commandes(h)
	before := h.snapshot()
	for _, q := range []string{
		`DROP TABLE Commandes`,
		`CREATE TABLE x (a)`,
		`ALTER TABLE Commandes ADD COLUMN y`,
		`DELETE FROM _lattis_sheets`,
		`UPDATE _lattis_tables SET name = 'Pirate'`,
		`INSERT INTO _lattis_journal (name, steps) VALUES ('x', '[]')`,
		`UPDATE Commandes SET Inconnue = 1`,
		`UPDATE sqlite_master SET sql = 'x'`,
	} {
		if _, err := h.try("db.execute", "sql", q); err == nil {
			t.Errorf("accepté : %s", q)
		}
	}
	for _, q := range []string{
		`DELETE FROM Commandes`,
		`SELECT load_extension('x')`,
	} {
		if _, err := h.try("db.query", "sql", q); err == nil {
			t.Errorf("db.query accepte : %s", q)
		}
	}
	sameSnapshot(t, "après des requêtes refusées", before, h.snapshot())

	// Plusieurs requêtes d'un coup forment une seule étape d'annulation.
	h.do("db.execute", "sql", `UPDATE Commandes SET Nombre = 1; DELETE FROM Commandes WHERE Client = 'Martin'`)
	h.wantColumn("Commandes", "Nombre", "1", "1")
	h.undo()
	sameSnapshot(t, "après annulation de deux requêtes", before, h.snapshot())
}

// Une requête ne doit pas sortir du classeur : le bac à sable des scripts
// promet « aucun accès aux fichiers », et un classeur reçu peut contenir un
// bouton qui lance un script.
func TestSQLCannotTouchFiles(t *testing.T) {
	h := newHost(t)
	commandes(h)
	dir := t.TempDir()
	attached := filepath.Join(dir, "attache.db")
	if _, err := h.try("db.execute", "sql", `ATTACH DATABASE '`+attached+`' AS dehors`); err == nil {
		h.try("db.execute", "sql", `CREATE TABLE dehors.x (a)`)
	}
	byScript := filepath.Join(dir, "script.db")
	res, _ := h.try("scripts.run", "source", `pcall(db.execute, "ATTACH DATABASE '`+strings.ReplaceAll(byScript, `\`, `\\`)+`' AS p")`)
	_ = res
	byQuery := filepath.Join(dir, "lecture.db")
	if _, err := h.try("db.query", "sql", `ATTACH DATABASE '`+byQuery+`' AS l`); err == nil {
		t.Error("db.query accepte ATTACH DATABASE")
	}
	for _, f := range []string{attached, byScript, byQuery} {
		if fi, err := os.Stat(f); err == nil {
			t.Errorf("ATTACH DATABASE accepté : une requête (ou un script, ou un bouton d'un classeur reçu) crée ou modifie un fichier SQLite n'importe où sur le disque (%s, %d octets)", filepath.Base(f), fi.Size())
		}
	}
	// Les tables internes ne se lisent pas non plus (accords, journal…),
	// ni depuis un script.
	if _, err := h.try("db.query", "sql", `SELECT * FROM _lattis_info`); err == nil {
		t.Error("db.query lit _lattis_info")
	}
	if res := h.lua(`print(pcall(db.value, "SELECT count(*) FROM _lattis_journal"))`); !strings.HasPrefix(strings.Join(res.Output, ""), "false") {
		t.Errorf("un script lit _lattis_journal : %+v", res)
	}
	if _, err := h.try("db.execute", "sql", `PRAGMA query_only = 0`); err == nil {
		t.Error("db.execute accepte un PRAGMA")
	}
	if _, err := h.try("db.execute", "sql", `VACUUM INTO '`+filepath.Join(dir, "copie.db")+`'`); err == nil {
		t.Fatal("VACUUM INTO accepté")
	}
}

// Jointure entre deux tableaux de deux feuilles, et lecture d'une colonne
// calculée.
func TestSQLJoinsAndComputedColumns(t *testing.T) {
	h := newHost(t)
	commandes(h)
	h.do("sheets.add", "name", "Réf")
	h.do("cells.paste", "sheet", "Réf", "row", 1, "col", 1, "values", norm([][]any{{"Client", "Ville", "Code"}, {"Durand", "Lyon", 69}, {"Martin", "Nantes", 44}}))
	ref := h.sheet("Réf").Tables[0].Name
	h.do("tables.rename", "table", ref, "name", "Clients")
	if got := h.value(`SELECT group_concat(c.Ville, ',') FROM Commandes o JOIN Clients c ON c.Client = o.Client`); got != "Lyon,Nantes" {
		t.Fatalf("jointure : %q", got)
	}
	h.set(1, 6, "TTC")
	h.set(2, 6, `=[Montant HT]*1,2`)
	h.wantColumn("Commandes", "TTC", "144.6", "96", "48.3")
	if got := h.value(`SELECT sum(TTC) FROM Commandes`); got != "288.9" {
		t.Fatalf("une colonne calculée est vide en SQL : SELECT sum(TTC) donne « %s »", got)
	}
	// Un script la lit aussi.
	if res := h.lua(`print(db.value("SELECT sum(TTC) FROM Commandes"))`); res.Error != "" || strings.Join(res.Output, "") != "288.9" {
		t.Fatalf("script : %+v", res)
	}
	// Onglet SQL : la requête arrive traduite en noms internes (t_1, c_6).
	ctx := context.Background()
	all, err := h.d.AllTables(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var q string
	for _, tn := range all {
		if tn.Name == "Commandes" {
			q = `SELECT sum(` + tn.ColumnSQL[5] + `) FROM ` + tn.SQL
		}
	}
	if q, err = h.d.ReadCTE(ctx, q); err != nil {
		t.Fatal(err)
	}
	r, err := grid.Query(ctx, h.d.DB().Read(), q, 10)
	if err != nil || len(r.Rows) != 1 || fmt.Sprint(r.Rows[0][0]) != "288.9" {
		t.Fatalf("noms internes : %v %v", r.Rows, err)
	}
	// Formule qui parcourt son propre tableau, et colonne calculée lue dans
	// une jointure et dans une sous-requête.
	h.set(1, 7, "Part")
	h.set(2, 7, `=[TTC]/SOMME(Commandes[TTC])`)
	if got := h.value(`SELECT round(sum(Part), 6) FROM Commandes`); got != "1" {
		t.Fatalf("part du total : %q", got)
	}
	if got := h.value(`SELECT group_concat(c.Ville || ':' || o.TTC, ',') FROM Commandes o JOIN Clients c ON c.Client = o.Client WHERE o.TTC > (SELECT min(TTC) FROM Commandes)`); got != "Lyon:144.6,Nantes:96.0" {
		t.Fatalf("jointure sur une colonne calculée : %q", got)
	}
	// Une écriture filtre aussi sur une colonne calculée.
	if n := h.do("db.execute", "sql", `DELETE FROM Commandes WHERE TTC < 50`); text(n) != "1" {
		t.Fatalf("DELETE … WHERE TTC < 50 : %v lignes", n)
	}
	h.undo()
	// Une colonne calculée ne s'écrit pas : UPDATE la laisse à sa formule.
	h.do("db.execute", "sql", `UPDATE Commandes SET TTC = 0`)
	h.wantColumn("Commandes", "TTC", "144.6", "96", "48.3")
}

// Les changements de structure sont refusés avec un message clair, et les
// erreurs de SQLite arrivent en français.
func TestSQLStructureAndFrenchErrors(t *testing.T) {
	h := newHost(t)
	commandes(h)
	for _, q := range []string{`DROP TABLE Commandes`, `ALTER TABLE Commandes ADD COLUMN x`, `CREATE TABLE x (a)`} {
		h.fails("se font depuis la grille", "db.query", "sql", q)
		h.fails("se font depuis la grille", "db.execute", "sql", q)
	}
	if got := h.value(`SELECT count(*) FROM Commandes`); got != "3" {
		t.Fatalf("Commandes touché : %s lignes", got)
	}
	h.fails("Tableau inconnu : « Inconnue »", "db.query", "sql", `SELECT * FROM Inconnue`)
	h.fails("Colonne inconnue : « Prixx »", "db.query", "sql", `SELECT Prixx FROM Commandes`)
	h.fails("Erreur de syntaxe près de « SELEC »", "db.query", "sql", `SELEC 1`)
	h.fails("Tableau inconnu", "db.execute", "sql", `DELETE FROM Inconnue`)
}

// Le filtre ne cache des lignes qu'à l'affichage : le SQL les voit toutes,
// et tables.rows seulement les lignes affichées.
func TestFilterHidesOnlyDisplay(t *testing.T) {
	h := newHost(t)
	commandes(h)
	h.do("columns.set_filter", "table", "Commandes", "column", "Payé", "values", []any{"FAUX"})
	shown := h.rows("Commandes")
	if shown.Total != len(shown.Rows) {
		t.Fatalf("total %d pour %d lignes", shown.Total, len(shown.Rows))
	}
	if got := h.value(`SELECT count(*) FROM Commandes`); got != "3" {
		t.Fatalf("SQL voit %s lignes", got)
	}
	if len(shown.Rows) != 2 {
		t.Fatalf("filtre : %d lignes affichées (%v)", len(shown.Rows), shown.Rows)
	}
	if tb := h.table("Commandes"); tb.Hidden != 1 {
		t.Fatalf("masquées : %d", tb.Hidden)
	}
	// Une formule sur la colonne compte toutes les lignes, comme SOMME dans Excel.
	h.set(8, 1, "=SOMME(Commandes[Nombre])")
	h.wantVal(8, 1, "6")
	h.do("columns.set_filter", "table", "Commandes", "column", "Montant HT", "conds", []any{map[string]any{"op": "gt", "value": "50"}})
	if got := len(h.rows("Commandes").Rows); got != 1 {
		t.Fatalf("deux filtres : %d lignes", got)
	}
	h.do("tables.clear_filters", "table", "Commandes")
	if got := len(h.rows("Commandes").Rows); got != 3 {
		t.Fatalf("sans filtre : %d lignes", got)
	}
}
