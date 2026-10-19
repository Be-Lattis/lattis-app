package formula

import (
	"context"
	"database/sql"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/store"
)

// TestMesureColonnesCalculees mesure, sur le tableau de démonstration, le
// coût de colonnes calculées dans les fenêtres de la grille et dans un agrégat
// sur tout le tableau. Lancé seulement avec LATTIS_MESURE=<lignes>.
func TestMesureColonnesCalculees(t *testing.T) {
	rows, _ := strconv.Atoi(os.Getenv("LATTIS_MESURE"))
	if rows <= 0 {
		t.Skip("LATTIS_MESURE=<nombre de lignes> pour lancer la mesure")
	}
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "m.sqlite"), Register)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := grid.CreateDemo(ctx, db, rows, nil); err != nil {
		t.Fatal(err)
	}

	cmd := &Table{Name: "Commandes", SQL: "t_1"}
	for i, c := range grid.DemoColumns {
		cmd.Cols = append(cmd.Cols, Column{Name: c.Name, SQL: "c_" + strconv.Itoa(i), Type: c.Type})
	}
	vendeurs := &Table{Name: "Vendeurs", SQL: "t_2", Cols: []Column{
		{Name: "Nom", SQL: "c_1", Type: grid.Text},
		{Name: "Région", SQL: "c_2", Type: grid.Text},
	}}
	schema := &Schema{Tables: []*Table{cmd, vendeurs}}
	err = db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE t_2 (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 TEXT, c_2 TEXT) STRICT;
			INSERT INTO t_2 VALUES (1,'a','Amélie','Nord'),(2,'b','Bruno','Sud'),(3,'c','Céline','Est'),
			(4,'d','Damien','Ouest'),(5,'e','Élodie','Nord'),(6,'f','Fabien','Sud')`); err != nil {
			return err
		}
		return EnsureKey(ctx, tx, schema, KeyNeed{"Vendeurs", "Nom"})
	})
	if err != nil {
		t.Fatal(err)
	}
	computed := map[string]string{
		"Calc HT": `=[@Quantité]*[@Prix unitaire]*(1-[@Remise])`,
		"Statut":  `=SI([@Payé];"OK";"À relancer")`,
		"Part":    `=[@Montant HT]/SOMME(Commandes[Montant HT])`,
		"Région":  `=REFERENCE(Vendeurs;Région;[@Vendeur])`,
	}
	names := []string{"Calc HT", "Statut", "Part", "Région"}
	for _, n := range names {
		cmd.Cols = append(cmd.Cols, Column{Name: n, Formula: mustParse(t, computed[n], FR)})
	}

	// Ancres comme dans internal/grid : une clé d'ordre toutes les 1000 lignes.
	var anchors []string
	rs, err := db.Read().Query(`SELECT _pos FROM t_1 ORDER BY _pos`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; rs.Next(); i++ {
		if i%1000 == 0 {
			var p string
			rs.Scan(&p)
			anchors = append(anchors, p)
		}
	}
	rs.Close()

	stored := make([]string, len(grid.DemoColumns))
	for i := range stored {
		stored[i] = "o.c_" + strconv.Itoa(i)
	}
	window := func(extra []string, args []any) (p50, p95 time.Duration) {
		q := `SELECT ` + strings.Join(append(append([]string{}, stored...), extra...), ", ") +
			` FROM t_1 AS o WHERE o._pos >= :A ORDER BY o._pos LIMIT 200 OFFSET :B`
		var ds []time.Duration
		for range 200 {
			first := rand.IntN(rows - 200)
			start := time.Now()
			r, err := db.Read().Query(q, append(append([]any{}, args...), sql.Named("A", anchors[first/1000]), sql.Named("B", first%1000))...)
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for r.Next() {
				n++
			}
			if err := r.Err(); err != nil {
				t.Fatal(err)
			}
			r.Close()
			ds = append(ds, time.Since(start))
		}
		slices.Sort(ds)
		return ds[len(ds)/2], ds[len(ds)*95/100]
	}
	ms := func(d time.Duration) string {
		return strconv.FormatFloat(float64(d.Microseconds())/1000, 'f', 2, 64) + " ms"
	}

	p50, p95 := window(nil, nil)
	t.Logf("fenêtre de 200 lignes, 20 colonnes stockées : p50 %s / p95 %s", ms(p50), ms(p95))

	// Les quatre colonnes calculées compilées dans le SELECT de la fenêtre.
	var extra []string
	var args []any
	// Agrégats sans dépendance à la ligne calculés une fois (cache).
	hoisted := 0
	hoistStart := time.Now()
	hoist := func(c Compiled) Value {
		hoisted++
		var v any
		if err := db.Read().QueryRow("SELECT "+c.SQL, c.Args...).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return FromResult(v)
	}
	c := &compiler{schema: schema, names: map[string]Value{}, visiting: map[*Column]bool{}, hoist: hoist}
	for _, n := range names {
		i, _ := cmd.col(n)
		extra = append(extra, c.column(cmd, &cmd.Cols[i], "o"))
	}
	args = c.args
	t.Logf("%d agrégat(s) calculé(s) une fois à la compilation : %v", hoisted, time.Since(hoistStart).Round(time.Millisecond))
	p50, p95 = window(extra, args)
	t.Logf("fenêtre + 4 colonnes calculées (dont REFERENCE et un agrégat) : p50 %s / p95 %s", ms(p50), ms(p95))

	// Agrégat sur tout le tableau : fonctions slt_* contre SQL natif.
	agg := Compile(mustParse(t, `=SOMME(Commandes[Calc HT])`, FR), schema, nil, nil, "")
	start := time.Now()
	var v any
	if err := db.Read().QueryRow(`SELECT `+agg.SQL, agg.Args...).Scan(&v); err != nil {
		t.Fatal(err)
	}
	t.Logf("SOMME(Commandes[Calc HT]) sur %d lignes, compilée : %v (= %s)", rows, time.Since(start).Round(time.Millisecond), FromResult(v))
	start = time.Now()
	if err := db.Read().QueryRow(`SELECT total(c_9 * c_10 * (1 - c_11)) FROM t_1`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	t.Logf("même somme en SQL natif : %v", time.Since(start).Round(time.Millisecond))

	start = time.Now()
	agg = Compile(mustParse(t, `=SOMME(Commandes[Part])`, FR), schema, nil, nil, "")
	if err := db.Read().QueryRow(`SELECT `+agg.SQL, agg.Args...).Scan(&v); err != nil {
		t.Fatal(err)
	}
	t.Logf("SOMME(Commandes[Part]) (agrégat dans une colonne calculée, sous-requête non corrélée) : %v (= %s)", time.Since(start).Round(time.Millisecond), FromResult(v))

	// Fonctions de la bibliothèque sur tout le tableau (agrégats sortis de la
	// requête de la grille : calculés une fois, invalidés quand le tableau change).
	for _, src := range []string{
		`=SOMME.SI(Commandes[Catégorie];"Info*";Commandes[Montant HT])`,
		`=NB.SI.ENS(Commandes[Payé];VRAI;Commandes[Quantité];">5")`,
		`=SOMMEPROD(Commandes[Quantité];Commandes[Prix unitaire])`,
		`=MEDIANE(Commandes[Montant HT])`,
		`=MAX(Commandes[Date commande])`,
		`=NB(FILTRE(Commandes[Montant HT];Commandes[Remise]>0))`,
		`=EQUIV("Bruno";Commandes[Vendeur];0)`,
	} {
		start := time.Now()
		agg := Compile(mustParse(t, src, FR), schema, nil, nil, "")
		if err := db.Read().QueryRow(`SELECT `+agg.SQL, agg.Args...).Scan(&v); err != nil {
			t.Fatalf("%s : %v", src, err)
		}
		t.Logf("%s : %v (= %s)", src, time.Since(start).Round(time.Millisecond), FromResult(v))
	}
	sp, e := CompileSpill(mustParse(t, `=GROUPER.PAR(Commandes[Vendeur];Commandes[Montant HT];SOMME)`, FR), schema, nil)
	if e.IsErr() {
		t.Fatal(e)
	}
	start = time.Now()
	r, err := db.Read().Query(sp.SQL, sp.Args...)
	if err != nil {
		t.Fatal(err)
	}
	groups := 0
	for r.Next() {
		groups++
	}
	r.Close()
	t.Logf("GROUPER.PAR(Commandes[Vendeur];Commandes[Montant HT];SOMME) : %v (%d groupes)", time.Since(start).Round(time.Millisecond), groups)

	// Colonne calculée qui parcourt le tableau à chaque ligne (NB.SI sur la
	// valeur de la ligne) : coût d'une fenêtre de 200 lignes.
	for _, src := range []string{
		`=NB.SI(Commandes[Vendeur];[@Vendeur])`,
		`=NB.SI(Commandes[Client];[@Client])`,
		`=SOMME.SI(Commandes[N° commande];[@N° commande];Commandes[Quantité])`,
		`=NB.SI.ENS(Commandes[Vendeur];[@Vendeur];Commandes[Payé];VRAI)`,
		`=RANG([@Montant HT])`,
		`=CUMUL([@Quantité])`,
	} {
		start = time.Now()
		c = &compiler{schema: schema, names: map[string]Value{}, visiting: map[*Column]bool{}, hoist: hoist}
		perRow := c.expr(mustParse(t, src, FR), scope{table: cmd, alias: "o"})
		t.Logf("%s : précalcul en %v", src, time.Since(start).Round(time.Millisecond))
		start = time.Now()
		r, err = db.Read().Query(`SELECT `+perRow+` FROM t_1 AS o ORDER BY o._pos LIMIT 200`, c.args...)
		if err != nil {
			t.Fatal(err)
		}
		for r.Next() {
		}
		r.Close()
		t.Logf("fenêtre de 200 lignes avec %s : %v", src, time.Since(start).Round(time.Millisecond))
	}
}
