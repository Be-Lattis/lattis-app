package transfer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/store"
)

type fixture struct {
	*canvas.Model
	bus *command.Bus
	db  *store.Store
}

func open(t *testing.T) *fixture {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "doc.sqlite"), canvas.Register)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := meta.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	bus := command.New(db)
	return &fixture{canvas.New(db, bus), bus, db}
}

func check(t *testing.T, d *fixture, when string) {
	t.Helper()
	problems, err := d.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Errorf("%s : %s", when, p)
	}
}

func write(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// layoutOf donne la mise en page de la dernière feuille.
func layoutOf(t *testing.T, d *fixture) canvas.Layout {
	t.Helper()
	ctx := context.Background()
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	last := l.Sheets[len(l.Sheets)-1].ID
	if l, err = d.Layout(ctx, last); err != nil {
		t.Fatal(err)
	}
	return l
}

func exportCSV(t *testing.T, d *fixture, table string, style CSVStyle) string {
	t.Helper()
	var b bytes.Buffer
	if err := ExportCSV(context.Background(), d.Model, table, &b, style); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestCSVRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	src := "\ufeffRéf;Client;Montant;Date;Payé;Note\r\n" +
		"01234;Dupont;12,5;15/01/2024;vrai;\"ligne 1\nligne 2\"\r\n" +
		"01235;Martin;1 000;2024-02-01;faux;\"avec ; et \"\"guillemets\"\"\"\r\n" +
		"01236;Durand;;;;\r\n"
	res, err := ImportCSV(ctx, d.Model, write(t, "Ventes 2024.csv", []byte(src)), CSVOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tables) != 1 || res.Tables[0] != "Ventes2024" {
		t.Fatalf("tableaux : %v", res.Tables)
	}
	check(t, d, "après import")
	l := layoutOf(t, d)
	if l.Sheets[len(l.Sheets)-1].Name != "Ventes 2024" || len(l.Tables) != 1 {
		t.Fatalf("mise en page : %+v", l)
	}
	tb := l.Tables[0]
	var got []string
	for _, c := range tb.Columns {
		got = append(got, c.Name+":"+string(c.Type))
	}
	want := "Réf:text Client:text Montant:decimal Date:date Payé:boolean Note:text"
	if strings.Join(got, " ") != want || tb.Rows != 3 || tb.Row != 0 || tb.Col != 0 {
		t.Fatalf("colonnes %v, %d lignes, ancre (%d, %d)", got, tb.Rows, tb.Row, tb.Col)
	}

	// Valeurs normalisées : saisies telles que la grille les affiche.
	norm := "\ufeffRéf;Client;Montant;Date;Payé;Note\r\n" +
		"01234;Dupont;12,5;15/01/2024;VRAI;\"ligne 1\r\nligne 2\"\r\n" +
		"01235;Martin;1000;01/02/2024;FAUX;\"avec ; et \"\"guillemets\"\"\"\r\n" +
		"01236;Durand;;;;\r\n"
	out := exportCSV(t, d, "Ventes2024", ExcelFR)
	if out != norm {
		t.Errorf("export ExcelFR :\n%q\nattendu :\n%q", out, norm)
	}
	std := exportCSV(t, d, "Ventes2024", Standard)
	if !strings.Contains(std, "01234,Dupont,12.5,2024-01-15,true,\"ligne 1\r\nligne 2\"\r\n") {
		t.Errorf("export standard :\n%s", std)
	}

	// Une seule étape d'annulation, réversible dans les deux sens.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, d, "après annulation")
	if l, _ := d.Layout(ctx, 1); len(l.Sheets) != 1 {
		t.Fatalf("feuilles après annulation : %v", l.Sheets)
	}
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, d, "après rétablissement")
	if out := exportCSV(t, d, "Ventes2024", ExcelFR); out != norm {
		t.Errorf("après rétablissement :\n%q", out)
	}

	// Annuler puis faire autre chose : la table de chargement est purgée au
	// prochain import.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 5, 5, "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportCSV(ctx, d.Model, write(t, "b.csv", []byte("a,b\n1,2\n")), CSVOptions{}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.db.Read().QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name LIKE '_lattis_trash_import_%'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d table(s) de chargement orphelines", n)
	}
	check(t, d, "second import")
}

func TestCSVEncodingsAndSeparators(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		data  []byte
		types string
		first string
	}{
		{"virgule", []byte("nom,prix,qté\nA,1.5,2\nB,2,3\n"), "text decimal integer", "A"},
		{"tabulation", []byte("nom\tprix\nA\t1,5\n"), "text decimal", "A"},
		{"windows-1252", []byte("nom;ville\nL\xe9a;Orl\xe9ans\n\x80;x\n"), "text text", "Léa"},
		{"utf-16", append([]byte{0xFF, 0xFE}, utf16le("a;b\r\nété;1\r\n")...), "text integer", "été"},
		{"une colonne", []byte("nom\nA\nB\n"), "text", "A"},
		{"lignes inégales", []byte("a;b\n1\n2;3;4\n"), "integer integer integer", "1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := open(t)
			if _, err := ImportCSV(ctx, d.Model, write(t, "x.csv", c.data), CSVOptions{}); err != nil {
				t.Fatal(err)
			}
			check(t, d, c.name)
			tb := layoutOf(t, d).Tables[0]
			var types []string
			for _, col := range tb.Columns {
				types = append(types, string(col.Type))
			}
			if strings.Join(types, " ") != c.types {
				t.Errorf("types %v, attendu %s", types, c.types)
			}
			out := exportCSV(t, d, tb.Name, Standard)
			if line := strings.SplitN(out, "\r\n", 3)[1]; !strings.HasPrefix(line, c.first) {
				t.Errorf("première ligne %q, attendu %q…", line, c.first)
			}
		})
	}
}

func utf16le(s string) []byte {
	var b []byte
	for _, r := range s {
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}

func TestParseText(t *testing.T) {
	for raw, want := range map[string]grid.Type{
		"12": grid.Integer, "-3": grid.Integer, "1 234": grid.Integer, "0": grid.Integer,
		"12,5": grid.Decimal, "0.5": grid.Decimal, "1e3": grid.Decimal,
		"VRAI": grid.Boolean, "false": grid.Boolean,
		"15/01/2024": grid.Date, "2024-01-15": grid.Date,
		"007": grid.Text, "1234567890123456": grid.Text, "inf": grid.Text, "NaN": grid.Text, "0x10": grid.Text,
		"": "", "  ": "",
	} {
		if got := parseText(raw).t; got != want {
			t.Errorf("%q : %q, attendu %q", raw, got, want)
		}
	}
}

// Dates avec heure et texte « null » dans un CSV (demande de Louis,
// 23/10/2026) : une colonne de dates avec heure est une colonne Date, null
// une case vide (NULL), réécrite vide à l'export.
func TestCSVDateTimeAndNull(t *testing.T) {
	for raw, want := range map[string]string{
		"23/10/2026 14:30":    "2026-10-23 14:30",
		"14:30 23/10/2026":    "2026-10-23 14:30",
		"23/10/2026 14:30:15": "2026-10-23 14:30:15",
		"23.10.2026 14:30":    "2026-10-23 14:30",
		"2026-10-23 14:30":    "2026-10-23 14:30",
		"2026/10/23 14:30":    "2026-10-23 14:30",
		"2026-10-23T14:30:00": "2026-10-23 14:30",
		"23/10/2026 2:30 PM":  "2026-10-23 14:30",
	} {
		if v := parseText(raw); v.t != grid.Date || v.s != want {
			t.Errorf("%q : %q %q, attendu %q", raw, v.t, v.s, want)
		}
	}
	for _, raw := range []string{"1.2.3", "14:30", "v1.2 14:30", "23/10/2026 25:00"} {
		if v := parseText(raw); v.t != grid.Text {
			t.Errorf("%q : %q", raw, v.t)
		}
	}
	for _, raw := range []string{"null", "NULL", " Null "} {
		if v := parseText(raw); v.t != "" {
			t.Errorf("%q : %q", raw, v.t)
		}
	}
	ctx := context.Background()
	path := write(t, "rdv.csv", []byte("Quand;Qui;N\r\n23/10/2026 14:30;Léa;1\r\n14:30:15 24/10/2026;null;NULL\r\nnull;Paul;3\r\n"))
	d := open(t)
	if _, err := ImportCSV(ctx, d.Model, path, CSVOptions{}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "après import")
	var types []string
	for _, c := range layoutOf(t, d).Tables[0].Columns {
		types = append(types, c.Name+":"+string(c.Type))
	}
	if got := strings.Join(types, " "); got != "Quand:date Qui:text N:integer" {
		t.Errorf("colonnes : %s", got)
	}
	want := "\ufeffQuand;Qui;N\r\n23/10/2026 14:30;Léa;1\r\n24/10/2026 14:30:15;;\r\n;Paul;3\r\n"
	if out := exportCSV(t, d, "rdv", ExcelFR); out != want {
		t.Errorf("export :\n%q\nattendu :\n%q", out, want)
	}
}
