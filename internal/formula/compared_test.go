package formula

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/store"
)

// TestComparedPrecomputed : une colonne calculée dont le critère compare la
// ligne aux autres (">"&[@N]) donne, avec les cumuls précalculés, les mêmes
// valeurs que le parcours complet, ligne par ligne : nombres, dates, textes
// (dont des textes qui écrivent un nombre), booléens, vides et erreurs.
func TestComparedPrecomputed(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "c.sqlite"), Register)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mix := &Table{Name: "Mix", SQL: "t_m", Cols: []Column{
		{Name: "N", SQL: "c_1", Type: grid.Decimal},
		{Name: "K", SQL: "c_2", Type: grid.Text},
		{Name: "G", SQL: "c_3", Type: grid.Text},
		{Name: "D", SQL: "c_4", Type: grid.Date},
		{Name: "B", SQL: "c_5", Type: grid.Boolean},
		{Name: "V", SQL: "c_6", Type: grid.Decimal},
		{Name: "E", Formula: mustParse(t, "=[@V]/[@N]", FR)}, // #DIV/0! quand N vaut 0
	}}
	schema := &Schema{Tables: []*Table{mix}}
	r := rand.New(rand.NewPCG(1, 2))
	pick := func(vals ...string) string { return vals[r.IntN(len(vals))] }
	var rows []string
	for i := 1; i <= 80; i++ {
		rows = append(rows, fmt.Sprintf("(%d, '%04d', %s, %s, %s, %s, %s, %s)", i, i,
			pick("NULL", "0", "1", "2.5", "-3", "7", "7", "10", "12.25"),
			pick("NULL", "'abc'", "'ABD'", "'10'", "'2,5'", "'Zed'", "''", "'7'"),
			pick("'a'", "'b'", "'A'", "'c'"),
			pick("NULL", "'2026-01-15'", "'2026-02-28'", "'2025-12-31'", "'2026-01-15'"),
			pick("NULL", "0", "1"),
			pick("NULL", "1", "2.5", "100", "-4", "0.1")))
	}
	err = db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE t_m (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 REAL, c_2 TEXT, c_3 TEXT, c_4 TEXT, c_5 INTEGER, c_6 REAL) STRICT;
			INSERT INTO t_m VALUES ` + strings.Join(rows, ", "))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	hoist := func(c Compiled) Value {
		calls++
		var v any
		if err := db.Read().QueryRow(`SELECT `+c.SQL, c.Args...).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return FromResult(v)
	}
	column := func(c Compiled) []string {
		rs, err := db.Read().Query(`SELECT `+c.SQL+` FROM t_m AS o ORDER BY o._pos`, c.Args...)
		if err != nil {
			t.Fatal(err)
		}
		defer rs.Close()
		var out []string
		for rs.Next() {
			var v any
			if err := rs.Scan(&v); err != nil {
				t.Fatal(err)
			}
			out = append(out, FromResult(v).String())
		}
		return out
	}
	for src, precomputed := range map[string]bool{
		`=SOMME.SI(Mix[N];">"&[@N];Mix[V])`:                             true,
		`=NB.SI(Mix[N];"<="&[@N])`:                                      true,
		`=NB.SI(Mix[N];"<"&[@N])`:                                       true,
		`=NB.SI(Mix[N];">="&[@N])`:                                      true,
		`=NB.SI(Mix[K];">"&[@K])`:                                       true,
		`=NB.SI(Mix[K];"<="&[@K])`:                                      true,
		`=NB.SI(Mix[K];"<"&[@N])`:                                       true,
		`=NB.SI(Mix[N];">"&[@K])`:                                       true,
		`=NB.SI(Mix[B];">"&[@B])`:                                       true,
		`=NB.SI(Mix[D];"<"&[@D])`:                                       true,
		`=MOYENNE.SI(Mix[N];">="&[@N];Mix[V])`:                          true,
		`=SOMME.SI(Mix[N];">"&[@N]&"";Mix[V])`:                          true,
		`=SOMME.SI(Mix[N];">"&[@N];Mix[E])`:                             true,
		`=SOMME.SI(Mix[N];"<"&[@N];Mix[K])`:                             true,
		`=MAX.SI.ENS(Mix[V];Mix[G];[@G];Mix[D];"<"&[@D])`:               true,
		`=MIN.SI.ENS(Mix[V];Mix[G];[@G];Mix[N];">"&[@N])`:               true,
		`=SOMME.SI.ENS(Mix[V];Mix[G];[@G];Mix[D];"<="&[@D])`:            true,
		`=NB.SI.ENS(Mix[G];[@G];Mix[B];[@B];Mix[N];">"&[@N])`:           true,
		`=SOMME.SI.ENS(Mix[V];Mix[N];">"&[@N];Mix[G];"a")`:              true,
		`=SOMME.SI.ENS(Mix[V];Mix[N];">"&[@N];Mix[G];1/0)`:              true,
		`=MOYENNE.SI.ENS(Mix[V];Mix[N];"<"&[@V];Mix[K];[@K])`:           true,
		`=SOMME.SI.ENS(Mix[V];Mix[N];">"&[@N];Mix[N];"<"&[@V])`:         false, // deux comparaisons
		`=NB.SI(Mix[N];"<>"&[@N])`:                                      true,  // égalités : totaux, puis parcours
		`=SOMME.SI(Mix[G];[@G];Mix[V])`:                                 true,
		`=NB.SI.ENS(Mix[G];[@G];Mix[V];1/0)`:                            true,
		`=SOMME.SI(Mix[N];SI([@B];">";"<")&[@N];Mix[V])`:                true, // parcours complet par ligne
		`=NB.SI(Mix[K];">"&SI(ESTVIDE([@K]);"";[@K]))`:                  true,
		`=NB.SI(Mix[N];">"&[@E])`:                                       true,
		`=SOMME.SI.ENS(Mix[V];Mix[G];[@G];Mix[K];[@K];Mix[N];"<"&[@N])`: true,
	} {
		calls = 0
		n := mustParse(t, src, FR)
		fast := CompileHoisted(n, schema, nil, mix, "o", hoist)
		if (calls > 0) != precomputed {
			t.Errorf("%s : précalculé %d fois, attendu %v", src, calls, precomputed)
		}
		slow := Compile(n, schema, nil, mix, "o")
		got, want := column(fast), column(slow)
		if strings.Join(got, "|") != strings.Join(want, "|") {
			for i := range got {
				if got[i] != want[i] {
					t.Errorf("%s ligne %d : précalcul %s, parcours %s", src, i+1, got[i], want[i])
				}
			}
		}
	}
}
