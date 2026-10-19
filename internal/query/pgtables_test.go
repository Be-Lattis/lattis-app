package query

import (
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// La requête du navigateur cache les schémas système et ne garde que ce
// que l'utilisateur peut lire ; le schéma lattis n'est caché que pour un
// Serveur Lattis (une base PostgreSQL quelconque peut avoir un schéma
// de ce nom à elle).
func TestPGTablesSQL(t *testing.T) {
	pg, lattis := pgTablesSQL(nil, "public"), pgTablesSQL([]string{"lattis"}, "donnees")
	for _, q := range []string{pg, lattis} {
		for _, want := range []string{
			"left(n.nspname, 3) <> 'pg_'",
			"'information_schema'",
			"has_schema_privilege(n.oid, 'USAGE')",
			"has_table_privilege(c.oid, 'SELECT')",
			"LIMIT 2000",
		} {
			if !strings.Contains(q, want) {
				t.Errorf("%q absent de :\n%s", want, q)
			}
		}
	}
	if strings.Contains(pg, "'lattis'") {
		t.Errorf("schéma lattis caché d'une base PostgreSQL :\n%s", pg)
	}
	if !strings.Contains(lattis, "NOT IN ('information_schema', 'lattis')") {
		t.Errorf("schéma lattis visible sur un Serveur Lattis :\n%s", lattis)
	}
	if !strings.Contains(pg, "n.nspname = 'public' DESC") || !strings.Contains(lattis, "n.nspname = 'donnees' DESC") {
		t.Error("ordre des schémas")
	}
	if q := pgTablesSQL([]string{"a'b"}, "c'd"); !strings.Contains(q, "'a''b'") || !strings.Contains(q, "'c''d'") {
		t.Errorf("apostrophes non doublées :\n%s", q)
	}
}

// Contre le schéma du serveur (LATTIS_PG_TEST=1, voir
// workbooktest.PGSetup) : un utilisateur ne voit ni lattis.* ni les
// catalogues, seulement les tables qu'il peut lire (donnees, classeurs).
func TestPostgresLattisTables(t *testing.T) {
	env := workbooktest.PGSetup(t)
	role, _ := env.User(t, "alice")
	u := pgx.Identifier{role}.Sanitize()
	for _, q := range []string{
		`CREATE TABLE donnees.essai_lisible (x int)`,
		`GRANT SELECT ON donnees.essai_lisible TO ` + u,
		`CREATE TABLE donnees.essai_cache (x int)`,
		`CREATE SCHEMA wb_essai`,
		`CREATE TABLE wb_essai.ventes (x int)`,
		`CREATE VIEW wb_essai.vue AS SELECT x FROM wb_essai.ventes`,
		`GRANT USAGE ON SCHEMA wb_essai TO ` + u,
		`GRANT SELECT ON wb_essai.ventes, wb_essai.vue TO ` + u,
		`CREATE SCHEMA wb_autre`,
		`CREATE TABLE wb_autre.secret (x int)`,
		`GRANT SELECT ON wb_autre.secret TO ` + u, // sans USAGE sur le schéma
	} {
		env.Exec(t, q)
	}

	cfg, err := pgx.ParseConfig(env.Super)
	if err != nil {
		t.Fatal(err)
	}
	db := stdlib.OpenDB(*cfg)
	defer db.Close()
	db.SetMaxOpenConns(1)
	list := func(d *dialect) []string {
		t.Helper()
		ts, err := listTables(ctx, db, d)
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, x := range ts {
			s := x.Schema + "." + x.Name
			if x.View {
				s += " (vue)"
			}
			if strings.HasPrefix(s, "pg_") || strings.HasPrefix(s, "information_schema.") {
				t.Errorf("%s : table système listée : %s", d.label, s)
			}
			out = append(out, s)
		}
		return out
	}
	internal := func(s string) bool { return strings.HasPrefix(s, "lattis.") }

	// Superutilisateur : tout est lisible ; lattis.* reste caché sur un
	// Serveur Lattis, pas sur une base PostgreSQL quelconque.
	if got := list(dialects[KindLattis]); slices.ContainsFunc(got, internal) {
		t.Errorf("tables internes listées : %v", got)
	}
	if got := list(dialects[KindPostgres]); !slices.ContainsFunc(got, internal) {
		t.Errorf("schéma lattis caché d'une base PostgreSQL : %v", got)
	}

	if _, err := db.ExecContext(ctx, `SET SESSION AUTHORIZATION `+u); err != nil {
		t.Fatal(err)
	}
	want := []string{"donnees.essai_lisible", "wb_essai.ventes", "wb_essai.vue (vue)"}
	for _, d := range []*dialect{dialects[KindLattis], dialects[KindPostgres]} {
		got := list(d)
		var mine []string
		for _, s := range got {
			if strings.HasPrefix(s, "donnees.essai") || strings.HasPrefix(s, "wb_") {
				mine = append(mine, s)
			}
			if internal(s) {
				t.Errorf("%s : table interne listée : %s", d.label, s)
			}
		}
		if !slices.Equal(mine, want) {
			t.Errorf("%s : tables = %v, attendu %v (liste : %v)", d.label, mine, want, got)
		}
	}
}
