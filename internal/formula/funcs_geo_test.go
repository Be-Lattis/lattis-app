package formula

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/store"
)

func TestCommune(t *testing.T) {
	cases := map[string]string{
		`=COMMUNE(48,853; 2,3499)`:                      "Paris",
		`=COMMUNE("48,853"; "2.3499"; "code")`:          "75056",
		`=COMMUNE(48,853; 2,3499; "Code_Postal")`:       "75001, 75002, 75003, 75004, 75005, 75006, 75007, 75008, 75009, 75010, 75011, 75012, 75013, 75014, 75015, 75016, 75017, 75018, 75019, 75020, 75116",
		`=COMMUNE(48,853; 2,3499; "département")`:       "Paris",
		`=COMMUNE(48,853; 2,3499; "region")`:            "Île-de-France",
		`=COMMUNE(45,7578; 4,832)`:                      "Lyon",
		`=COMMUNE(43,2951; 5,374; "cp")`:                "13001, 13002, 13003, 13004, 13005, 13006, 13007, 13008, 13009, 13010, 13011, 13012, 13013, 13014, 13015, 13016",
		`=COMMUNE(41,9192; 8,7386; "code_departement")`: "2A",
		`=COMMUNE(44,3842; 4,9906; "departement")`:      "Vaucluse",
		`=COMMUNE(18,068; -63,083; "region")`:           "#N/A", // Saint-Martin : collectivité sans région
		`=COMMUNE(46; -5)`:                              "#N/A", // en mer
		`=COMMUNE(50,8466; 4,3528)`:                     "#N/A", // Bruxelles
		`=COMMUNE("abc"; 2)`:                            "#VALEUR!",
		`=COMMUNE(VRAI; 2)`:                             "#VALEUR!",
		`=COMMUNE(95; 2)`:                               "#VALEUR!",
		`=COMMUNE(48,853; 2,3499; "pays")`:              "#VALEUR!",
		`=COMMUNE(48,853; 2,3499; 3)`:                   "#VALEUR!",
		`=COMMUNE(1/0; 2)`:                              "#DIV/0!",
		`=COMMUNE(""; 2)`:                               "#N/A",
		`=MUNICIPALITY(48,853; 2,3499; "insee")`:        "75056",
	}
	for src, want := range cases {
		if got := Eval(mustParse(t, src, FR), &Env{}).String(); got != want {
			t.Errorf("%s : %q, attendu %q", src, got, want)
		}
	}
}

// TestCommuneColonne : colonne calculée COMMUNE([Lat]; [Lon]) sur 100 000
// lignes, en SQL compilé (slt_fn) et dans l'évaluateur Go ; les deux doivent
// donner le même résultat, ligne à ligne.
func TestCommuneColonne(t *testing.T) {
	n := 100_000
	if testing.Short() {
		n = 10_000
	}
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "g.sqlite"), Register)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tbl := &Table{Name: "Points", SQL: "t_g", Cols: []Column{
		{Name: "Lat", SQL: "c_1", Type: grid.Decimal},
		{Name: "Lon", SQL: "c_2", Type: grid.Decimal},
		{Name: "Texte", SQL: "c_3", Type: grid.Text},
	}}
	schema := &Schema{Tables: []*Table{tbl}}
	err = db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE t_g (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 REAL, c_2 REAL, c_3 TEXT) STRICT`); err != nil {
			return err
		}
		st, err := tx.Prepare(`INSERT INTO t_g VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		r := rand.New(rand.NewPCG(1, 2))
		for i := 1; i <= n; i++ {
			// France métropolitaine et alentours (terre, mer, pays voisins).
			lat, lon := 42.3+r.Float64()*8.8, -4.8+r.Float64()*13
			var lt, ln any = lat, lon
			if i%10 == 0 {
				lt = nil // ligne sans position
			}
			txt := strings.Replace(fmt.Sprintf("%.5f", lat), ".", ",", 1)
			if _, err := st.Exec(i, fmt.Sprintf("%08d", i), lt, ln, txt); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	formula := mustParse(t, `=COMMUNE([Lat]; [Lon]; "code")`, FR)
	tbl.Cols = append(tbl.Cols, Column{Name: "Commune", Formula: formula})

	c := Compile(formula, schema, nil, tbl, "o")
	if !strings.Contains(c.SQL, "slt_fn(") {
		t.Fatalf("SQL inattendu : %s", c.SQL)
	}
	t0 := time.Now()
	rs, err := db.Read().Query(`SELECT `+c.SQL+` FROM t_g AS o ORDER BY _pos`, c.Args...)
	if err != nil {
		t.Fatal(err)
	}
	var sqlVals []Value
	for rs.Next() {
		var v any
		if err := rs.Scan(&v); err != nil {
			t.Fatal(err)
		}
		sqlVals = append(sqlVals, FromResult(v))
	}
	rs.Close()
	dSQL := time.Since(t0)

	data, err := LoadData(ctx, db.Read(), schema)
	if err != nil {
		t.Fatal(err)
	}
	t0 = time.Now()
	env := &Env{Schema: schema, Data: data, Table: tbl}
	found, na := 0, 0
	for i := range n {
		env.Row = i
		g := Eval(formula, env)
		if s := sqlVals[i]; g != s {
			t.Fatalf("ligne %d : Go %v, SQL %v", i+1, g, s)
		}
		switch {
		case g.Kind == Text:
			found++
		case g.IsErr() && g.Err == ErrNA:
			na++
		default:
			t.Fatalf("ligne %d : %v", i+1, g)
		}
	}
	dGo := time.Since(t0)
	if found < n/3 || na < n/10 {
		t.Errorf("%d communes trouvées, %d #N/A sur %d lignes", found, na, n)
	}
	// Coordonnée en texte à virgule : même résultat que le nombre.
	txt := mustParse(t, `=COMMUNE([Texte]; [Lon]; "code")=COMMUNE([Lat]; [Lon]; "code")`, FR)
	for i := range 1000 {
		env.Row = i
		if i%10 == 9 {
			continue
		}
		if v := Eval(txt, env); v != Boolean(true) && !v.IsErr() {
			t.Fatalf("ligne %d : texte et nombre diffèrent", i+1)
		}
	}
	t.Logf("%d lignes : SQL %v, Go %v (%d communes, %d #N/A)", n, dSQL, dGo, found, na)
}
