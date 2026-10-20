package query

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/store"
)

// Les tables propres à Lattis ne paraissent pas dans la liste des tables
// d'une source (classeur .lattis ouvert comme base, Serveur Lattis).
func TestInternalTablesHidden(t *testing.T) {
	for _, c := range []struct {
		t    DBTable
		want bool
	}{
		{DBTable{Schema: "wb_test_2cc356", Name: "lattis_meta_columns"}, true},
		{DBTable{Name: "_lattis_cells"}, true},
		{DBTable{Schema: "lattis", Name: "workbooks"}, true},
		{DBTable{Schema: "wb_test_2cc356", Name: "Ventes"}, false},
		{DBTable{Schema: "donnees", Name: "lattis"}, false},
	} {
		if internalTable(c.t) != c.want {
			t.Errorf("%+v : %v", c.t, !c.want)
		}
	}
	path := filepath.Join(t.TempDir(), "b.sqlite")
	db, err := store.OpenDB("file:"+path, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`CREATE TABLE Ventes (x)`, `CREATE TABLE _lattis_cells (x)`, `CREATE TABLE lattis_meta_tables (x)`} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	ts, err := FileTables(context.Background(), "sqlite", path)
	if err != nil || len(ts) != 1 || ts[0].Name != "Ventes" {
		t.Fatalf("tables : %+v, %v", ts, err)
	}
}
