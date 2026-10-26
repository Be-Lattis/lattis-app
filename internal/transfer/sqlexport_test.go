package transfer

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ncruces/go-sqlite3/driver"

	"github.com/lalamefine/lattis/internal/canvas"
)

// sqlFixture : un tableau de tous les types (clé entière, grand entier,
// décimal, texte piégé, date, date et heure, booléen, colonne calculée) et
// un tableau croisé.
func sqlFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"N°", "Grand", "Prix", "Libellé \"court\"", "Date", "Heure", "Payé"},
		{"1", "5000000000", "1,5", "L'été à Paris", "15/01/2026", "15/01/2026 14:30", "VRAI"},
		{"2", "7", "2,25", "a\\b ; c\nd", "16/01/2026", "16/01/2026 08:05:09", "FAUX"},
		{"3", "-3", "10", "", "", "", ""},
	})
	must(err)
	must(d.RenameSheet(ctx, 1, "Données"))
	must(d.RenameTable(ctx, "Feuille1", "Ventes_été"))
	must(d.SetKey(ctx, "Ventes_été", 0))
	_, err = d.SetCell(ctx, 1, 0, 7, "Total")
	must(err)
	_, err = d.SetCell(ctx, 1, 1, 7, "=[Prix]*2")
	must(err)
	_, err = d.CreatePivot(ctx, canvas.PivotConfig{Source: "Ventes_été", Rows: "Payé", Value: "Prix", Agg: "sum"})
	must(err)
	return d
}

func exportSQLText(t *testing.T, d *fixture, o SQLOptions) (string, SQLReport) {
	t.Helper()
	o.Now = time.Date(2026, 10, 26, 14, 30, 0, 0, time.UTC)
	o.Source = "Essai.lattis"
	var b bytes.Buffer
	rep, err := ExportSQL(context.Background(), d.Model, &b, o)
	if err != nil {
		t.Fatal(err)
	}
	return b.String(), rep
}

// Le script de chaque dialecte : types, littéraux, guillemets des noms.
func TestExportSQLDialects(t *testing.T) {
	d := sqlFixture(t)
	want := map[SQLDialect][]string{
		SQLStandard: {
			"-- Généré par Lattis le 2026-10-26 14:30",
			"-- Classeur : Essai.lattis",
			`CREATE TABLE "Ventes_été" (`,
			`  "N°" INTEGER NOT NULL,`,
			`  "Grand" BIGINT,`,
			`  "Prix" NUMERIC(15,2),`,
			`  "Libellé ""court""" VARCHAR(20),`,
			`  "Date" DATE,`,
			`  "Heure" TIMESTAMP,`,
			`  "Payé" BOOLEAN,`,
			`  "Total" NUMERIC(14,1), -- formule Lattis : =[Prix]*2`,
			`  PRIMARY KEY ("N°")`,
			`INSERT INTO "Ventes_été" ("N°", "Grand", "Prix", "Libellé ""court""", "Date", "Heure", "Payé", "Total") VALUES`,
			`(1, 5000000000, 1.5, 'L''été à Paris', DATE '2026-01-15', TIMESTAMP '2026-01-15 14:30:00', TRUE, 3)`,
			"(2, 7, 2.25, 'a\\b ; c\nd', DATE '2026-01-16', TIMESTAMP '2026-01-16 08:05:09', FALSE, 4.5)",
			`(3, -3, 10, NULL, NULL, NULL, NULL, 20);`,
			`-- Tableau croisé TCD`,
		},
		SQLPostgres: {
			"BEGIN;", `  "Libellé ""court""" TEXT,`, `  "Prix" NUMERIC(15,2),`, "TRUE, 3)", "COMMIT;",
		},
		SQLMySQL: {
			"SET NAMES utf8mb4;", "START TRANSACTION;", "CREATE TABLE `Ventes_été` (", "  `Payé` TINYINT(1),", "  `Heure` DATETIME,",
			"  `Prix` DECIMAL(15,2),", "'a\\\\b ; c\nd'", "DATE '2026-01-15', TIMESTAMP '2026-01-15 14:30:00', 1, 3)", "COMMIT;",
		},
		SQLServer: {
			"BEGIN TRANSACTION;", "CREATE TABLE [Ventes_été] (", "  [Libellé \"court\"] NVARCHAR(20),", "  [Payé] BIT,",
			"  [Heure] DATETIME2(0),", "N'L''été à Paris', '2026-01-15', '2026-01-15 14:30:00', 1, 3)", "COMMIT TRANSACTION;",
		},
		SQLSQLite: {
			"BEGIN TRANSACTION;", `  "Prix" REAL,`, `  "Libellé ""court""" TEXT,`, "'2026-01-15', '2026-01-15 14:30:00', 1, 3)", "COMMIT;",
		},
	}
	for dia, parts := range want {
		s, rep := exportSQLText(t, d, SQLOptions{Dialect: string(dia)})
		for _, p := range parts {
			if !strings.Contains(s, p) {
				t.Errorf("%s : %q absent de :\n%s", dia, p, s)
			}
		}
		if rep.Tables != 2 || rep.Rows != 3+3 {
			t.Errorf("%s : compte rendu %+v", dia, rep)
		}
	}
	if s, _ := exportSQLText(t, d, SQLOptions{}); strings.Contains(s, "BEGIN") || strings.Contains(s, "COMMIT") {
		t.Error("standard : pas de transaction attendue")
	}
	if _, err := ExportSQL(context.Background(), d.Model, &bytes.Buffer{}, SQLOptions{Dialect: "oracle"}); err == nil {
		t.Error("dialecte inconnu accepté")
	}
	if _, err := ExportSQL(context.Background(), d.Model, &bytes.Buffer{}, SQLOptions{Table: "Absent"}); err == nil {
		t.Error("tableau absent accepté")
	}
	// Portée : un seul tableau.
	if s, rep := exportSQLText(t, d, SQLOptions{Table: "ventes_été"}); rep.Tables != 1 || strings.Contains(s, "TCD") {
		t.Errorf("un seul tableau : %+v\n%s", rep, s)
	}
}

// standardForSQLite : SQLite ne lit pas DATE '…' ni TIMESTAMP '…' (les
// autres bases, si) ; le reste du script standard passe tel quel.
var typedLiteral = regexp.MustCompile(`(DATE|TIMESTAMP) '`)

// Le script se rejoue dans une base SQLite neuve, en standard comme en
// dialecte SQLite : mêmes lignes, mêmes valeurs.
func TestExportSQLRunsInSQLite(t *testing.T) {
	d := sqlFixture(t)
	for _, dia := range []SQLDialect{SQLStandard, SQLSQLite} {
		s, _ := exportSQLText(t, d, SQLOptions{Dialect: string(dia)})
		if dia == SQLStandard {
			s = typedLiteral.ReplaceAllString(s, "'")
		}
		db, err := driver.Open(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s : %v\n%s", dia, err, s)
		}
		checkSQLRows(t, string(dia), db, `SELECT "N°", "Grand", "Prix", "Libellé ""court""", "Date", "Heure", "Payé", "Total" FROM "Ventes_été" ORDER BY 1`, []string{
			"1|5000000000|1.5|L'été à Paris|2026-01-15|2026-01-15 14:30:00|1|3",
			"2|7|2.25|a\\b ; c\nd|2026-01-16|2026-01-16 08:05:09|0|4.5",
			"3|-3|10|<nil>|<nil>|<nil>|<nil>|20",
		})
		checkSQLRows(t, string(dia), db, `SELECT "Payé", "Somme de Prix" FROM "TCD1"`, []string{"VRAI|1.5", "FAUX|2.25", "(vide)|10"})
		db.Close()
	}
}

func checkSQLRows(t *testing.T, what string, db *sql.DB, q string, want []string) {
	t.Helper()
	rs, err := db.Query(q)
	if err != nil {
		t.Fatalf("%s : %v", what, err)
	}
	defer rs.Close()
	cols, _ := rs.Columns()
	var got []string
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		parts := make([]string, len(vals))
		for i, v := range vals {
			switch v := v.(type) {
			case []byte:
				parts[i] = string(v)
			case time.Time:
				if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 {
					parts[i] = v.Format("2006-01-02")
				} else {
					parts[i] = v.Format("2006-01-02 15:04:05")
				}
			case bool:
				parts[i] = "0"
				if v {
					parts[i] = "1"
				}
			default:
				parts[i] = fmt.Sprint(v)
			}
		}
		got = append(got, strings.Join(parts, "|"))
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s : lignes\n%s\nattendu\n%s", what, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Lots de 500 lignes, une saisie hors type écrite NULL, une clé avec des
// vides sans PRIMARY KEY.
func TestExportSQLBatches(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	rows := [][]string{{"Id", "Nom"}}
	for i := range 1203 {
		rows = append(rows, []string{fmt.Sprint(i + 1), fmt.Sprintf("n%d", i)})
	}
	if _, err := d.Paste(ctx, 1, 0, 0, rows); err != nil {
		t.Fatal(err)
	}
	// Saisie hors type dans une colonne d'entiers, gardée en erreur.
	if err := d.FixType(ctx, "Feuille1", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 5, 0, "abc"); err != nil {
		t.Fatal(err)
	}
	s, rep := exportSQLText(t, d, SQLOptions{Dialect: string(SQLSQLite)})
	if n := strings.Count(s, "INSERT INTO"); n != 3 {
		t.Errorf("%d INSERT, 3 attendus", n)
	}
	if rep.Rows != 1203 {
		t.Errorf("lignes : %+v", rep)
	}
	db, err := driver.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(s); err != nil {
		t.Fatal(err)
	}
	checkSQLRows(t, "lots", db, `SELECT COUNT(*), SUM("Id") FROM "Feuille1"`, []string{"1203|724201"})
	if rep.Nulls != 1 || !strings.Contains(s, "-- 1 valeur hors type") {
		t.Errorf("saisie hors type : %+v", rep)
	}
}

// Contre PostgreSQL (LATTIS_TEST_PG_URL) : le script standard et le
// dialecte PostgreSQL se rejouent dans un schéma neuf.
func TestExportSQLPostgres(t *testing.T) {
	url := os.Getenv("LATTIS_TEST_PG_URL")
	if url == "" {
		t.Skip("LATTIS_TEST_PG_URL absent")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Skip("PostgreSQL injoignable : ", err)
	}
	defer conn.Close(ctx)
	d := sqlFixture(t)
	for _, dia := range []SQLDialect{SQLStandard, SQLPostgres} {
		schema := fmt.Sprintf("lattis_sql_%s_%d", dia, time.Now().UnixNano())
		if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
		if _, err := conn.Exec(ctx, "SET search_path TO "+schema); err != nil {
			t.Fatal(err)
		}
		s, _ := exportSQLText(t, d, SQLOptions{Dialect: string(dia)})
		if _, err := conn.Exec(ctx, s); err != nil {
			t.Fatalf("%s : %v\n%s", dia, err, s)
		}
		rs, err := conn.Query(ctx, `SELECT "N°", "Grand", "Prix"::text, "Libellé ""court""", "Date"::text, "Heure"::text, "Payé", "Total"::text FROM "Ventes_été" ORDER BY 1`, pgx.QueryExecModeSimpleProtocol)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for rs.Next() {
			v, err := rs.Values()
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, fmt.Sprint(v))
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			t.Fatalf("%s : %v", dia, err)
		}
		want := []string{
			"[1 5000000000 1.50 L'été à Paris 2026-01-15 2026-01-15 14:30:00 true 3.0]",
			"[2 7 2.25 a\\b ; c\nd 2026-01-16 2026-01-16 08:05:09 false 4.5]",
			"[3 -3 10.00 <nil> <nil> <nil> <nil> 20.0]",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s :\n%s", dia, strings.Join(got, "\n"))
		}
		var n int
		if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM "TCD1"`, pgx.QueryExecModeSimpleProtocol).Scan(&n); err != nil || n != 3 {
			t.Errorf("%s : TCD %d %v", dia, n, err)
		}
	}
}
