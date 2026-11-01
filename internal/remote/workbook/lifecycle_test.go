package workbook_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/demo"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
	"github.com/lalamefine/lattis/internal/transfer"
)

var lcClosed sync.Map

func lcNew(t *testing.T) *doc.Document {
	t.Helper()
	d, err := doc.New(context.Background())
	must(t, err)
	t.Cleanup(func() {
		if _, ok := lcClosed.Load(d); !ok {
			d.Close()
		}
	})
	return d
}

func lcClose(t *testing.T, d *doc.Document) {
	t.Helper()
	lcClosed.Store(d, true)
	must(t, d.Close())
}

// Cycle complet d'un classeur publié, du côté de ce que voit l'utilisateur :
// tableaux (lignes, colonnes, place, valeurs affichées), cellules libres et
// feuilles, avant la publication et après chaque façon de le rouvrir (fichier,
// serveur, rechargement, copie locale), puis après synchronisation, partage
// automatique et « retour d'une longue coupure ». Tableaux non cochés compris.

// snapshot : ce que l'utilisateur voit d'un classeur.
type lcTable struct {
	Sheet string
	Pos   string
	Cols  []string
	Rows  int
	Data  string
}

type lcSnap struct {
	Tables map[string]lcTable
	Sheets []string
	Cells  map[string]string
	Shown  map[string]int // tableaux rendus par Layout, par feuille
}

func lcSnapshot(t *testing.T, d *doc.Document) lcSnap {
	t.Helper()
	ctx := context.Background()
	s := lcSnap{Tables: map[string]lcTable{}, Cells: map[string]string{}, Shown: map[string]int{}}
	db := d.DB().Read()
	rs, err := db.Query(`SELECT t.id, t.name, sh.name, t.row, t.col FROM _lattis_tables t JOIN _lattis_sheets sh ON sh.id = t.sheet`)
	must(t, err)
	type x struct {
		id       int64
		n, s     string
		row, col int
	}
	var l []x
	for rs.Next() {
		var a x
		must(t, rs.Scan(&a.id, &a.n, &a.s, &a.row, &a.col))
		l = append(l, a)
	}
	rs.Close()
	for _, a := range l {
		tb := lcTable{Sheet: a.s, Pos: fmt.Sprintf("%d,%d", a.row, a.col)}
		rc, err := db.Query(`SELECT id, name, type, coalesce(formula,'') FROM _lattis_columns WHERE tbl = ? ORDER BY pos`, a.id)
		must(t, err)
		var sel []string
		for rc.Next() {
			var id int64
			var n, ty, f string
			must(t, rc.Scan(&id, &n, &ty, &f))
			tb.Cols = append(tb.Cols, n+":"+ty+":"+f)
			sel = append(sel, fmt.Sprintf("coalesce(cast(c_%d as text),'∅')", id))
		}
		rc.Close()
		must(t, db.QueryRow(fmt.Sprintf(`SELECT count(*) FROM t_%d`, a.id)).Scan(&tb.Rows))
		if len(sel) > 0 && tb.Rows <= 5000 {
			var sb strings.Builder
			must(t, d.ExportTable(ctx, a.n, func([]canvas.ExportColumn) error { return nil }, func(v []any) error {
				fmt.Fprintln(&sb, v...)
				return nil
			}))
			tb.Data = sb.String()
		}
		s.Tables[a.n] = tb
	}
	rs, err = db.Query(`SELECT name FROM _lattis_sheets ORDER BY pos`)
	must(t, err)
	var ids []int64
	for rs.Next() {
		var n string
		must(t, rs.Scan(&n))
		s.Sheets = append(s.Sheets, n)
	}
	rs.Close()
	rs, err = db.Query(`SELECT id FROM _lattis_sheets ORDER BY pos`)
	must(t, err)
	for rs.Next() {
		var id int64
		must(t, rs.Scan(&id))
		ids = append(ids, id)
	}
	rs.Close()
	rs, err = db.Query(`SELECT sh.name, c.row, c.col, c.raw FROM _lattis_cells c JOIN _lattis_sheets sh ON sh.id = c.sheet`)
	must(t, err)
	for rs.Next() {
		var n, raw string
		var r, c int
		must(t, rs.Scan(&n, &r, &c, &raw))
		s.Cells[fmt.Sprintf("%s!%d,%d", n, r, c)] = raw
	}
	rs.Close()
	for _, id := range ids {
		f, err := d.LayoutFirst(ctx, id)
		must(t, err)
		lay, err := d.Layout(ctx, id)
		must(t, err)
		if len(f.Tables) != len(lay.Tables) {
			t.Errorf("LayoutFirst %d tableaux, Layout %d (feuille %d)", len(f.Tables), len(lay.Tables), id)
		}
		for _, tb := range lay.Tables {
			s.Shown[tb.Name]++
		}
	}
	return s
}

// lcCompare dit ce qui manque ou diffère dans after par rapport à before,
// pour les tableaux want (nil : tous).
func lcCompare(t *testing.T, what string, before, after lcSnap, want map[string]bool) {
	t.Helper()
	for n, b := range before.Tables {
		if want != nil && !want[n] {
			continue
		}
		a, ok := after.Tables[n]
		if !ok {
			t.Errorf("%s : PERDU %s (%d lignes)", what, n, b.Rows)
			continue
		}
		if a.Rows != b.Rows {
			t.Errorf("%s : LIGNES %s %d → %d", what, n, b.Rows, a.Rows)
		}
		if !slices.Equal(a.Cols, b.Cols) {
			t.Errorf("%s : COLONNES %s\n  %v\n→ %v", what, n, b.Cols, a.Cols)
		}
		if a.Sheet != b.Sheet || a.Pos != b.Pos {
			t.Errorf("%s : PLACE %s %s %s → %s %s", what, n, b.Sheet, b.Pos, a.Sheet, a.Pos)
		}
		if a.Data != b.Data {
			t.Errorf("%s : DONNÉES %s\n  %.300s\n→ %.300s", what, n, b.Data, a.Data)
		}
		if after.Shown[n] != 1 {
			t.Errorf("%s : %s rendu %d fois par Layout", what, n, after.Shown[n])
		}
	}
	for k, v := range before.Cells {
		if after.Cells[k] != v {
			t.Errorf("%s : CELLULE %s %q → %q", what, k, v, after.Cells[k])
		}
	}
	for _, s := range before.Sheets {
		if !slices.Contains(after.Sheets, s) {
			t.Errorf("%s : FEUILLE perdue %s", what, s)
		}
	}
}

func lcNames(s lcSnap) string {
	var o []string
	for n, t := range s.Tables {
		o = append(o, fmt.Sprintf("%s(%d)", n, t.Rows))
	}
	sort.Strings(o)
	return strings.Join(o, " ")
}

// lcRich : plusieurs tableaux par feuille, cellules libres, colonne calculée,
// tableau tout en formules, requête, onglet à choix, regroupement d'onglets.
func lcRich(t *testing.T) *doc.Document {
	t.Helper()
	ctx := context.Background()
	d := lcNew(t)
	paste(t, d, 1, 0, 0, [][]string{{"Produit", "Quantité", "Prix", "Livré le", "Payé"}, {"Café", "3", "4,5", "01/02/2026", "vrai"}, {"Thé", "10", "2", "15/03/2026", "faux"}, {"Chocolat", "", "3,25", "", "vrai"}})
	must(t, d.RenameTable(ctx, tableAt(t, d, 0, 0), "Ventes"))
	paste(t, d, 1, 0, 8, [][]string{{"Note", "Valeur"}, {"a", "1"}, {"b", "2"}})
	must(t, d.RenameTable(ctx, tableAt(t, d, 0, 8), "Notes"))
	paste(t, d, 1, 10, 0, [][]string{{"Clé", "Lib"}, {"1", "un"}, {"2", "deux"}})
	must(t, d.RenameTable(ctx, tableAt(t, d, 10, 0), "Bas"))
	for _, c := range []struct {
		r, c int
		v    string
	}{{8, 0, "Titre"}, {20, 3, "=1+1"}, {7, 8, "libre"}} {
		_, err := d.SetCell(ctx, 1, c.r, c.c, c.v)
		must(t, err)
	}
	// Feuille importée : colonne calculée, tableau tout en formules.
	imp := func(yield func([]any) error) error {
		for i := range 4 {
			if err := yield([]any{int64(i + 1), float64(i) + 0.5, nil}); err != nil {
				return err
			}
		}
		return nil
	}
	_, err := d.Import(ctx, "imp", []canvas.ImportSheet{{Name: "Calculs", Tables: []canvas.ImportTable{
		{Name: "Calc", Row: 0, Col: 0, Headers: []string{"A", "B", "C"}, Types: []grid.Type{grid.Integer, grid.Decimal, grid.Decimal}, Formulas: []string{"", "", "=[A]+[B]"}, Count: 4, Rows: imp},
		{Name: "Calc2", Row: 0, Col: 6, Headers: []string{"N", "D", "E"}, Types: []grid.Type{grid.Integer, grid.Decimal, grid.Decimal}, Formulas: []string{"", "", "=[N]*2"}, Count: 4, Rows: imp},
	}, Cells: []canvas.ImportCell{{Row: 8, Col: 0, Raw: "Calculs libres"}}}})
	must(t, err)
	// Requête.
	_, err = d.QueryToSheet(ctx, `SELECT 1 AS Produit, 'x' AS Prix UNION ALL SELECT 2, 'y'`, "Req", nil)
	must(t, err)
	// Onglets semblables regroupés (onglet à choix).
	s2, err := d.AddSheet(ctx)
	must(t, err)
	paste(t, d, s2, 0, 0, [][]string{{"Jour", "Total"}, {"1", "10"}, {"2", "20"}})
	s3, err := d.AddSheet(ctx)
	must(t, err)
	paste(t, d, s3, 0, 0, [][]string{{"Jour", "Total"}, {"3", "30"}, {"4", "40"}})
	must(t, d.RenameSheet(ctx, s2, "Janvier"))
	must(t, d.RenameSheet(ctx, s3, "Février"))
	var n2, n3 string
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE sheet = ?`, s2).Scan(&n2))
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE sheet = ?`, s3).Scan(&n3))
	_, err = d.MergeSheets(ctx, canvas.MergePlan{Tables: []string{n2, n3}, Slicer: true, Name: "Mois", Sheet: "Mois"})
	must(t, err)
	// Onglet à choix posé à la main.
	s4, err := d.AddSheet(ctx)
	must(t, err)
	paste(t, d, s4, 0, 0, [][]string{{"Ville", "Pop"}, {"Lyon", "1"}, {"Nice", "2"}, {"Lyon", "3"}})
	var n4 string
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE sheet = ?`, s4).Scan(&n4))
	must(t, d.RenameTable(ctx, n4, "Villes"))
	must(t, d.SetSheetChoice(ctx, s4, "Villes", "Ville", "Lyon"))
	return d
}

func lcIDs(t *testing.T, d *doc.Document, except ...string) []int64 {
	t.Helper()
	check, err := workbook.Inspect(context.Background(), d.DB())
	must(t, err)
	var ids []int64
	for _, tb := range check.Tables {
		if len(tb.Problems) == 0 && tb.Invalid == 0 && !slices.Contains(except, tb.Name) {
			ids = append(ids, tb.ID)
		} else {
			t.Logf("non coché : %s %v inv=%d", tb.Name, tb.Problems, tb.Invalid)
		}
	}
	return ids
}

type lcBackend struct {
	name  string
	as    func(string) workbook.Server
	alice string
}

func lcBackends(t *testing.T) []lcBackend {
	f := workbooktest.New()
	out := []lcBackend{{"fake", func(u string) workbook.Server { return f.As(u) }, "alice"}}
	return out
}

func lcLifecycle(t *testing.T, b lcBackend, build func(*testing.T) *doc.Document, mode string, except ...string) {
	ctx := context.Background()
	srv := b.as(b.alice)
	d := build(t)
	all := lcSnapshot(t, d)
	t.Logf("avant : %s ; feuilles %v", lcNames(all), all.Sheets)
	ids := lcIDs(t, d, except...)
	want := map[string]bool{}
	check, _ := workbook.Inspect(ctx, d.DB())
	for _, tb := range check.Tables {
		if slices.Contains(ids, tb.ID) {
			want[tb.Name] = true
		}
	}
	info, err := workbook.Publish(ctx, srv, d.DB(), workbook.Request{Server: server, User: b.alice, Title: "Matrice " + time.Now().Format("150405.000"), Mode: mode, Tables: ids})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	lcCompare(t, "auteur après publication", all, lcSnapshot(t, d), nil)

	// Fichier local : enregistré, rouvert.
	path := filepath.Join(t.TempDir(), "orig.lattis")
	must(t, d.Save(ctx, path))
	lcClose(t, d)
	f, err := doc.Open(ctx, path)
	must(t, err)
	t.Cleanup(func() { f.Close() })
	lcCompare(t, "fichier rouvert", all, lcSnapshot(t, f), nil)
	f.Share.SetOnline(true, "")

	sync := func(what string, x *doc.Document, s lcSnap, want map[string]bool) {
		t.Helper()
		if mode != workbook.Collaborative {
			return
		}
		e := collab.New(x.DB(), x.Model, x.Share)
		e.SetServer(srv)
		x.Oplog.SetStructure(e.Structure)
		x.SetIndexer(e)
		defer func() { x.Oplog.SetStructure(nil); x.SetIndexer(nil) }()
		for _, long := range []bool{false, true} {
			if long {
				must(t, x.DB().Write(ctx, func(tx *sql.Tx) error {
					_, err := tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES ('layout_synced', ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, time.Now().Add(-72*time.Hour).UTC().Format(time.RFC3339))
					return err
				}))
			}
			r, err := e.SyncLayout(ctx, true)
			must(t, err)
			r2, err := e.Sync(ctx)
			must(t, err)
			_, _ = r, r2
			lcCompare(t, fmt.Sprintf("%s, sync long=%v", what, long), s, lcSnapshot(t, x), want)
		}
		rep, err := e.AutoShare(ctx)
		must(t, err)
		_ = rep
		lcCompare(t, what+", partage auto", s, lcSnapshot(t, x), want)
	}
	sync("fichier", f, all, nil)

	// Serveur, neuf.
	r := lcNew(t)
	_, err = workbook.Load(ctx, srv, r.DB(), workbook.Source{Server: server, User: b.alice, WorkbookID: info.WorkbookID}, true)
	must(t, err)
	must(t, r.ShareChanged(ctx))
	r.Share.SetOnline(true, "")
	after := lcSnapshot(t, r)
	t.Logf("serveur neuf : %s", lcNames(after))
	lcCompare(t, "serveur neuf", all, after, want)
	sync("serveur neuf", r, all, want)

	// Rechargement sur place (ShareReload), avec un tableau local ajouté.
	var first int64
	must(t, r.DB().Read().QueryRow(`SELECT id FROM _lattis_sheets ORDER BY pos LIMIT 1`).Scan(&first))
	paste(t, r, first, 4000, 0, [][]string{{"Perso", "V"}, {"x", "1"}})
	withPerso := lcSnapshot(t, r)
	if mode == workbook.Frozen {
		_, err = workbook.Load(ctx, srv, r.DB(), workbook.Source{Server: server, User: b.alice, WorkbookID: info.WorkbookID}, false)
		must(t, err)
		must(t, r.ShareChanged(ctx))
		lcCompare(t, "rechargé", withPerso, lcSnapshot(t, r), nil)
	}

	// Cache : enregistré, rouvert, rechargé (figé) ou synchronisé (direct).
	cp := filepath.Join(t.TempDir(), "cache.lattis")
	must(t, r.Save(ctx, cp))
	lcClose(t, r)
	c, err := doc.Open(ctx, cp)
	must(t, err)
	t.Cleanup(func() { c.Close() })
	lcCompare(t, "cache rouvert", withPerso, lcSnapshot(t, c), nil)
	c.Share.SetOnline(true, "")
	if mode == workbook.Frozen {
		_, err = workbook.Load(ctx, srv, c.DB(), workbook.Source{Server: server, User: b.alice, WorkbookID: info.WorkbookID}, false)
		must(t, err)
		must(t, c.ShareChanged(ctx))
		lcCompare(t, "cache rechargé", withPerso, lcSnapshot(t, c), nil)
	} else {
		sync("cache", c, withPerso, nil)
	}
}

func lcXlsx(t *testing.T) *doc.Document {
	t.Helper()
	ctx := context.Background()
	src := lcRich(t)
	p := filepath.Join(t.TempDir(), "x.xlsx")
	_, err := transfer.ExportXLSXFile(ctx, src.Model, p)
	must(t, err)
	d := lcNew(t)
	res, err := transfer.ImportXLSX(ctx, d.Model, p)
	must(t, err)
	t.Logf("import : %v", res.Warnings)
	return d
}

func lcDemo(name string) func(*testing.T) *doc.Document {
	return func(t *testing.T) *doc.Document {
		d, _, err := demo.New(context.Background(), name, demo.Options{})
		must(t, err)
		t.Cleanup(func() {
			if _, ok := lcClosed.Load(d); !ok {
				d.Close()
			}
		})
		return d
	}
}

func TestPublishReopenMatrix(t *testing.T) {
	books := map[string]func(*testing.T) *doc.Document{"riche": lcRich, "xlsx": lcXlsx}
	run := func(t *testing.T, b lcBackend, more ...string) {
		for _, name := range more {
			books[name] = lcDemo(name)
		}
		for n, build := range books {
			for _, mode := range []string{workbook.Frozen, workbook.Collaborative} {
				t.Run(n+"/"+mode, func(t *testing.T) {
					useRoot(t)
					lcLifecycle(t, b, build, mode, "Notes")
				})
			}
		}
	}
	t.Run("fake", func(t *testing.T) { run(t, lcBackends(t)[0]) })
	t.Run("pg", func(t *testing.T) {
		env := pgSetup(t)
		alice, _ := env.user(t, "alice")
		run(t, lcBackend{"pg", func(u string) workbook.Server { return env.as(u) }, alice}, "med", "refs")
	})
}

// Tableaux créés après la publication d'un classeur collaboratif (collés, sur
// une nouvelle feuille, import .xlsx, résultat de requête) : tous sur le serveur.
func TestTablesAfterPublishReachServer(t *testing.T) {
	env := pgSetup(t)
	useRoot(t)
	ctx := context.Background()
	alice, _ := env.user(t, "alice")
	srv := env.as(alice)
	d := lcNew(t)
	paste(t, d, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Café", "4,5"}})
	must(t, d.RenameTable(ctx, tableAt(t, d, 0, 0), "Ventes"))
	info, err := workbook.Publish(ctx, srv, d.DB(), workbook.Request{Server: server, User: alice, Title: "Après", Mode: workbook.Collaborative, Tables: []int64{tableID(t, d, "Ventes")}})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	e := collab.New(d.DB(), d.Model, d.Share)
	e.SetServer(srv)
	d.Oplog.SetStructure(e.Structure)
	d.SetIndexer(e)

	paste(t, d, 1, 0, 6, [][]string{{"Colle", "V"}, {"a", "1"}})
	must(t, d.RenameTable(ctx, tableAt(t, d, 0, 6), "Colle"))
	s2, err := d.AddSheet(ctx)
	must(t, err)
	paste(t, d, s2, 0, 0, [][]string{{"Autre", "V"}, {"b", "2"}})
	src := lcNew(t)
	paste(t, src, 1, 0, 0, [][]string{{"Imp", "V"}, {"c", "3"}, {"d", "4"}})
	p := filepath.Join(t.TempDir(), "i.xlsx")
	_, err = transfer.ExportXLSXFile(ctx, src.Model, p)
	must(t, err)
	_, err = transfer.ImportXLSX(ctx, d.Model, p)
	must(t, err)
	_, err = d.QueryToSheet(ctx, `SELECT 1 AS q, 'x' AS r`, "Requete", nil)
	must(t, err)
	before := lcSnapshot(t, d)
	t.Logf("auteur : %s", lcNames(before))
	for range 3 {
		r, err := e.AutoShare(ctx)
		t.Logf("partage auto : %v / %v / err %v", r.Notices, r.Shared, err)
		_, err = e.Sync(ctx)
		must(t, err)
		_, err = e.SyncLayout(ctx, true)
		must(t, err)
	}
	r := lcNew(t)
	_, err = workbook.Load(ctx, srv, r.DB(), workbook.Source{Server: server, User: alice, WorkbookID: info.WorkbookID}, true)
	must(t, err)
	after := lcSnapshot(t, r)
	t.Logf("serveur : %s ; feuilles %v", lcNames(after), after.Sheets)
	for n, b := range before.Tables {
		if _, ok := after.Tables[n]; !ok {
			t.Errorf("PAS SUR LE SERVEUR : %s (%d lignes, feuille %s)", n, b.Rows, b.Sheet)
		}
	}
	lcCompare(t, "serveur", before, after, nil)
}

// Un texte local avec des octets hors UTF-8 ou un caractère nul ne bloque plus
// toute la publication (« invalid byte sequence for encoding UTF8 »).
func TestPublishDirtyText(t *testing.T) {
	env := pgSetup(t)
	useRoot(t)
	ctx := context.Background()
	alice, _ := env.user(t, "alice")
	d := lcNew(t)
	paste(t, d, 1, 0, 0, [][]string{{"Titre", "N"}, {"a", "1"}, {"b", "2"}, {"c", "3"}})
	must(t, d.RenameTable(ctx, tableAt(t, d, 0, 0), "Livres"))
	tbl := tableID(t, d, "Livres")
	var col int64
	must(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_columns WHERE tbl = ? AND name = 'Titre'`, tbl).Scan(&col))
	must(t, d.DB().Write(ctx, func(tx *sql.Tx) error {
		q := fmt.Sprintf(`UPDATE t_%d SET c_%d = CAST(? AS TEXT) WHERE id = ?`, tbl, col)
		if _, err := tx.ExecContext(ctx, q, []byte{0xa9, 't', 'o', 'i', 'l', 'e'}, 1); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, q, []byte{'a', 0, 'b'}, 2)
		return err
	}))
	info, err := workbook.Publish(ctx, env.as(alice), d.DB(), workbook.Request{Server: server, User: alice, Title: "Texte", Mode: workbook.Frozen, Tables: []int64{tbl}})
	must(t, err)
	r := lcNew(t)
	_, err = workbook.Load(ctx, env.as(alice), r.DB(), workbook.Source{Server: server, User: alice, WorkbookID: info.WorkbookID}, true)
	must(t, err)
	if got := column(t, r, "Livres", "Titre"); !slices.Equal(got, []string{"\uFFFDtoile", "ab", "c"}) {
		t.Fatalf("textes : %q", got)
	}
}

// Préréglage big de la démonstration (100 000 lignes) : relancer à la main,
// LATTIS_LC_BIG=1 (il échouait sur « Livres » : 0xa9).
func TestPublishBigPreset(t *testing.T) {
	if os.Getenv("LATTIS_LC_BIG") == "" {
		t.Skip("LATTIS_LC_BIG=1")
	}
	env := pgSetup(t)
	useRoot(t)
	alice, _ := env.user(t, "alice")
	lcLifecycle(t, lcBackend{"pg", func(u string) workbook.Server { return env.as(u) }, alice}, lcDemo("big"), workbook.Frozen)
}
