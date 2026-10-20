package query

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/ncruces/go-sqlite3/driver"

	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// lattisFile : une base SQLite dont la table personnes a les colonnes
// système d'un tableau du Serveur Lattis (lattis.system_columns()), et la
// vue donnee celles qu'une donnée du Data Lake garde.
func lattisFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "serveur.db")
	db, err := driver.Open("file:" + p)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE personnes (lattis_id TEXT PRIMARY KEY, lattis_pos TEXT, lattis_xid INTEGER, lattis_author TEXT,
			lattis_updated_by TEXT, lattis_updated_at TEXT, lattis_updated_device TEXT, lattis_other_xid INTEGER, Nom TEXT, age INTEGER)`,
		`INSERT INTO personnes VALUES
			('a1', 'a', 10, 'louis@acme.fr', 'pipeline+deed58ac1d94@lattis.invalid', '2026-10-20 14:03:12.123456', 'poste', NULL, 'louis', 20),
			('a2', 'b', 11, 'louis@acme.fr', 'fabrice@acme.fr', '2026-10-21 09:00:00', '', 10, 'fabrice', 15)`,
		`CREATE VIEW donnee AS SELECT lattis_id, lattis_pos, lattis_xid, lattis_updated_at, Nom FROM personnes`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

// Un tableau du Serveur Lattis lu comme source : colonnes de l'utilisateur,
// puis « Mis à jour par » et « Mis à jour le », à l'aperçu comme au
// chargement ; les noms sont fixés à la première actualisation.
func TestSystemColumnsSource(t *testing.T) {
	p := lattisFile(t)
	d := newDoc(t)
	src := Source{Kind: SourceSQLite, Path: p, SQL: "SELECT * FROM personnes ORDER BY lattis_pos"}

	pv := previewOf(t, env(d), src)
	if pv.Error != "" {
		t.Fatal(pv.Error)
	}
	if got := fmt.Sprint(pv.Columns); got != "[{Nom text} {age integer} {Mis à jour par text} {Mis à jour le text}]" {
		t.Fatalf("colonnes %s", got)
	}
	want := "[[louis 20 pipeline+deed58ac1d94@lattis.invalid 2026-10-20 14:03:12] [fabrice 15 fabrice@acme.fr 2026-10-21 09:00:00]]"
	if got := fmt.Sprint(pv.Rows); got != want {
		t.Fatalf("lignes %s", got)
	}

	// Chargement, avec une étape qui cite une colonne ajoutée.
	id := save(t, d, Query{Name: "Personnes", Source: src, Steps: []Step{{Op: OpSort, Sort: []SortKey{{Column: "Mis à jour le", Desc: true}}}}})
	l, err := Refresh(ctx, env(d), id)
	if err != nil {
		t.Fatal(err)
	}
	table := "[[Nom:text age:integer Mis à jour par:text Mis à jour le:text] [fabrice 15 fabrice@acme.fr 2026-10-21 09:00:00] [louis 20 pipeline+deed58ac1d94@lattis.invalid 2026-10-20 14:03:12]]"
	if got := fmt.Sprint(dump(t, d, l.Table)); got != table {
		t.Fatalf("tableau %s", got)
	}
	def, err := d.QueryByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	q, _ := Decode(def)
	if q.Source.Audit == nil || *q.Source.Audit != (AuditNames{By: "Mis à jour par", At: "Mis à jour le"}) {
		t.Fatalf("noms non gardés : %+v", q.Source.Audit)
	}
	// Une seule étape d'annulation pour l'actualisation et les noms gardés.
	if name, err := d.Bus.Undo(ctx); err != nil || name != "Actualiser « Personnes »" {
		t.Fatalf("annulation %q, %v", name, err)
	}
	if def, _ = d.QueryByID(ctx, id); def.Table != "" {
		t.Fatalf("tableau resté : %+v", def)
	}
	if q, _ = Decode(def); q.Source.Audit != nil {
		t.Fatalf("noms restés : %+v", q.Source.Audit)
	}
	if l, err = Refresh(ctx, env(d), id); err != nil {
		t.Fatal(err)
	}

	// Langue changée : la requête garde ses noms, une nouvelle prend ceux
	// de la session.
	d.Model.SetLanguage(msg.EN)
	t.Cleanup(func() { d.Model.SetLanguage(msg.FR) })
	if l, err = Refresh(ctx, env(d), id); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(dump(t, d, l.Table)); got != table {
		t.Fatalf("après changement de langue : %s", got)
	}
	pv = previewOf(t, env(d), src)
	if got := fmt.Sprint(pv.Columns); got != "[{Nom text} {age integer} {Updated by text} {Updated on text}]" {
		t.Fatalf("colonnes en anglais %s", got)
	}
	d.Model.SetLanguage(msg.FR)

	// Colonnes système gardées à la demande.
	keep := src
	keep.SystemColumns = true
	if pv = previewOf(t, env(d), keep); len(pv.Columns) != 10 || pv.Columns[0].Name != "lattis_id" {
		t.Fatalf("colonnes système gardées : %v", pv.Columns)
	}

	// Donnée du Data Lake (sans lattis_updated_by) : seulement « Mis à
	// jour le ».
	pv = previewOf(t, env(d), Source{Kind: SourceSQLite, Path: p, SQL: "SELECT * FROM donnee ORDER BY lattis_pos"})
	if got := fmt.Sprint(pv.Columns, pv.Rows); got != "[{Nom text} {Mis à jour le text}] [[louis 2026-10-20 14:03:12] [fabrice 2026-10-21 09:00:00]]" {
		t.Fatalf("donnée %s", got)
	}

	// Une requête qui choisit ses colonnes n'est pas touchée.
	pv = previewOf(t, env(d), Source{Kind: SourceSQLite, Path: p, SQL: "SELECT Nom, lattis_updated_by FROM personnes ORDER BY lattis_pos"})
	if got := fmt.Sprint(pv.Columns); got != "[{Nom text} {lattis_updated_by text}]" {
		t.Fatalf("colonnes choisies %s", got)
	}
}

func TestSystemMap(t *testing.T) {
	cols := func(names ...string) []Column {
		var out []Column
		for _, n := range names {
			out = append(out, Column{Name: n})
		}
		return out
	}
	for _, c := range []struct {
		cols []string
		want string
	}{
		{[]string{"id", "Nom"}, "<nil>"},
		{[]string{"lattis_id", "Nom"}, "<nil>"},
		{[]string{"LATTIS_ID", "lattis_xid", "Nom", "lattis_updated_by"}, "&{[2] 3 -1}"},
		{[]string{"lattis_id", "lattis_updated_at", "Nom", "lattis_quelconque"}, "&{[2] -1 1}"},
	} {
		if got := fmt.Sprint(systemMap(cols(c.cols...))); got != c.want {
			t.Errorf("%v : %s, attendu %s", c.cols, got, c.want)
		}
	}
	q := Query{Source: Source{Kind: SourceLattis}, Steps: []Step{{Op: OpMerge, Other: &Source{Kind: SourcePostgres}}, {Op: OpAppend, Other: &Source{Kind: SourceCSV}}}}
	if !freezeAudit(&q, msg.DE) || q.Source.Audit.By != "Aktualisiert von" || q.Steps[0].Other.Audit == nil || q.Steps[1].Other.Audit != nil {
		t.Fatalf("freezeAudit %+v", q)
	}
	if freezeAudit(&q, msg.FR) {
		t.Fatal("noms refixés")
	}
}

// Contre le schéma du serveur (LATTIS_PG_TEST=1) : un tableau créé avec
// lattis.system_columns(), lu par une connexion Serveur Lattis.
func TestPostgresSystemColumns(t *testing.T) {
	pg := workbooktest.PGSetup(t)
	pg.Exec(t, `CREATE SCHEMA wb_essai`)
	pg.Exec(t, `DO $$ BEGIN EXECUTE format('CREATE TABLE wb_essai.personnes (%s, "Nom" text, age int8)', lattis.system_columns()); END $$`)
	pg.Exec(t, `INSERT INTO wb_essai.personnes (lattis_id, lattis_pos, "Nom", age, lattis_updated_by, lattis_updated_at)
		VALUES (gen_random_uuid(), 'a', 'louis', 20, 'pipeline+deed58ac1d94@lattis.invalid', '2026-10-20 14:03:12.5+00'),
		       (gen_random_uuid(), 'b', 'fabrice', 15, 'fabrice@acme.fr', '2026-10-21 09:00:00+00')`)
	cfg, err := pgx.ParseConfig(pg.Super)
	if err != nil {
		t.Fatal(err)
	}
	db := stdlib.OpenDB(*cfg)
	defer db.Close()

	d := newDoc(t)
	s := newSession(t)
	r := &run{s: s, env: env(d), ctx: ctx, fresh: true}
	src := Source{Kind: SourceLattis, SQL: TableSQL(KindLattis, "wb_essai", "personnes") + ` ORDER BY lattis_pos`}
	l, err := r.readInto(db, dialects[KindLattis], src, "essai")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(l.cols); got != "[{Nom text} {age integer} {Mis à jour par text} {Mis à jour le text}]" {
		t.Fatalf("colonnes %s", got)
	}
	rs, err := s.db.Query(`SELECT * FROM ` + qi(l.table))
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var got []string
	for rs.Next() {
		var nom, by, at string
		var age int64
		if err := rs.Scan(&nom, &age, &by, &at); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprint(nom, " ", age, " ", by, " ", len(at)))
	}
	if fmt.Sprint(got) != "[louis 20 pipeline+deed58ac1d94@lattis.invalid 19 fabrice 15 fabrice@acme.fr 19]" {
		t.Fatalf("lignes %v", got)
	}
	if !r.system {
		t.Fatal("lecture non signalée")
	}
}
