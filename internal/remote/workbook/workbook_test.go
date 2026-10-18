package workbook_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

const server = "https://lattis.example.test"

func useRoot(t *testing.T) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "work")
	old := archive.Root
	archive.Root = func() (string, error) { return root, nil }
	t.Cleanup(func() { archive.Root = old })
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newDoc(t *testing.T) *doc.Document {
	t.Helper()
	d, err := doc.New(context.Background())
	must(t, err)
	t.Cleanup(func() { d.Close() })
	return d
}

func paste(t *testing.T, d *doc.Document, sheet int64, row, col int, block [][]string) {
	t.Helper()
	_, err := d.Paste(context.Background(), sheet, row, col, block)
	must(t, err)
}

// tableID donne l'identifiant d'un tableau par son nom.
func tableID(t *testing.T, d *doc.Document, name string) int64 {
	t.Helper()
	var id int64
	must(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = ?`, name).Scan(&id))
	return id
}

// column lit une colonne d'un tableau, dans l'ordre d'affichage.
func column(t *testing.T, d *doc.Document, table, col string) []string {
	t.Helper()
	var tbl, cid int64
	must(t, d.DB().Read().QueryRow(`SELECT t.id, c.id FROM _lattis_tables t JOIN _lattis_columns c ON c.tbl = t.id WHERE t.name = ? AND c.name = ?`, table, col).Scan(&tbl, &cid))
	rs, err := d.DB().Read().Query(`SELECT coalesce(CAST(c_` + itoa(cid) + ` AS TEXT), '') FROM t_` + itoa(tbl) + ` ORDER BY _pos, id`)
	must(t, err)
	defer rs.Close()
	var out []string
	for rs.Next() {
		var s string
		must(t, rs.Scan(&s))
		out = append(out, s)
	}
	return out
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

type (
	canvasTable = canvas.TableInfo
	canvasCell  = canvas.CellInfo
)

// author prépare le classeur d'un auteur : un tableau Ventes (à publier),
// un tableau Notes (privé), une cellule libre et un script.
func author(t *testing.T) *doc.Document {
	t.Helper()
	ctx := context.Background()
	d := newDoc(t)
	paste(t, d, 1, 0, 0, [][]string{
		{"Produit", "Quantité", "Prix", "Livré le", "Payé"},
		{"Café", "3", "4,5", "01/02/2026", "vrai"},
		{"Thé", "10", "2", "15/03/2026", "faux"},
		{"Chocolat", "", "3,25", "", "vrai"},
	})
	must(t, d.RenameTable(ctx, tableAt(t, d, 0, 0), "Ventes"))
	paste(t, d, 1, 0, 8, [][]string{{"Note", "Valeur"}, {"a", "1"}})
	must(t, d.RenameTable(ctx, tableAt(t, d, 0, 8), "Notes"))
	_, err := d.SetCell(ctx, 1, 8, 0, "Titre du rapport")
	must(t, err)
	_, err = d.SaveScript(ctx, 0, "bonjour", `print("bonjour")`)
	must(t, err)
	return d
}

// tableAt donne le nom du tableau ancré en (row, col) de la feuille 1.
func tableAt(t *testing.T, d *doc.Document, row, col int) string {
	t.Helper()
	var n string
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE sheet = 1 AND row = ? AND col = ?`, row, col).Scan(&n))
	return n
}

func publish(t *testing.T, d *doc.Document, srv workbook.Server, mode string, tables ...string) *workbook.Info {
	t.Helper()
	ctx := context.Background()
	var ids []int64
	for _, n := range tables {
		ids = append(ids, tableID(t, d, n))
	}
	info, err := workbook.Publish(ctx, srv, d.DB(), workbook.Request{Server: server, User: "alice", Title: "Ventes 2026", Mode: mode, Tables: ids})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	return info
}

func open(t *testing.T, srv workbook.Server, id, user string) *doc.Document {
	t.Helper()
	ctx := context.Background()
	d := newDoc(t)
	_, err := workbook.Load(ctx, srv, d.DB(), workbook.Source{Server: server, User: user, WorkbookID: id, Title: "Ventes 2026"}, true)
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	return d
}

func TestPublishFrozenAndOpen(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	a := author(t)
	info := publish(t, a, fake.As("alice"), workbook.Frozen, "Ventes")

	if info.Version != 1 || info.Mode != workbook.Frozen || !info.CanDesign() || info.Channel == "" {
		t.Fatalf("info : %+v", info)
	}
	state, _, version, tables := fake.Book(info.WorkbookID)
	if state != "published" || version != 1 || !slices.Equal(tables, []string{"Ventes"}) {
		t.Fatalf("serveur : %s v%d %v", state, version, tables)
	}
	rows := fake.Rows(info.WorkbookID, "Ventes")
	if len(rows) != 3 || rows[0][2] != "Café" || rows[0][3] != int64(3) || rows[0][4] != 4.5 || rows[0][6] != true || rows[2][3] != nil {
		t.Fatalf("lignes publiées : %v", rows)
	}
	if d, ok := rows[1][5].(time.Time); !ok || d.Format("2006-01-02") != "2026-03-15" {
		t.Fatalf("date publiée : %v", rows[1][5])
	}
	// Clés d'ordre réparties, identifiants partagés posés.
	var n int
	must(t, a.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_row_ids WHERE tbl = ?`, tableID(t, a, "Ventes")).Scan(&n))
	if n != 3 {
		t.Fatalf("%d identifiants partagés", n)
	}
	if rows[0][0] == rows[1][0] || rows[0][1].(string) >= rows[1][1].(string) {
		t.Fatalf("lattis_id / lattis_pos : %v", rows)
	}
	// Types figés (§9).
	must(t, a.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_columns WHERE tbl = ? AND fixed = 0`, tableID(t, a, "Ventes")).Scan(&n))
	if n != 0 {
		t.Fatalf("%d colonnes encore élargissables", n)
	}
	// La mise en page publiée ne dit rien du tableau privé.
	if lay := string(fake.Meta(info.WorkbookID, "layout")); !strings.Contains(lay, "Ventes") || strings.Contains(lay, "Notes") || !strings.Contains(lay, "Titre du rapport") {
		t.Fatalf("mise en page publiée : %s", lay)
	}
	if fake.Calls["publish_script"] != 1 || fake.Calls["finish_publish"] != 1 {
		t.Fatalf("appels : %v", fake.Calls)
	}
	if a.Share.TableShare(tableID(t, a, "Notes")) != "private" || a.Share.TableShare(tableID(t, a, "Ventes")) != "published" {
		t.Fatal("partage des tableaux de l'auteur")
	}
	// L'auteur d'un classeur figé modifie sa copie (prochaine version) ;
	// rien ne part.
	_, err := a.SetCell(ctx, 1, 1, 1, "4")
	must(t, err)
	must(t, a.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_outbox`).Scan(&n))
	if n != 0 {
		t.Fatalf("%d entrées dans la file d'un classeur figé", n)
	}

	// Un lecteur ouvre le classeur.
	must(t, fake.Grant(info.WorkbookID, "bob", "reader"))
	list, err := fake.As("bob").Workbooks(ctx)
	must(t, err)
	if len(list) != 1 || list[0].Title != "Ventes 2026" || list[0].Roles[0] != "reader" {
		t.Fatalf("liste : %+v", list)
	}
	r := open(t, fake.As("bob"), info.WorkbookID, "bob")
	if got := column(t, r, "Ventes", "Produit"); !slices.Equal(got, []string{"Café", "Thé", "Chocolat"}) {
		t.Fatalf("produits : %v", got)
	}
	if got := column(t, r, "Ventes", "Prix"); !slices.Equal(got, []string{"4.5", "2.0", "3.25"}) {
		t.Fatalf("prix : %v", got)
	}
	if got := column(t, r, "Ventes", "Livré le"); !slices.Equal(got, []string{"2026-02-01", "2026-03-15", ""}) {
		t.Fatalf("dates : %v", got)
	}
	if got := column(t, r, "Ventes", "Payé"); !slices.Equal(got, []string{"1", "0", "1"}) {
		t.Fatalf("booléens : %v", got)
	}
	var cnt int
	must(t, r.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_tables WHERE name = 'Notes'`).Scan(&cnt))
	if cnt != 0 {
		t.Fatal("le tableau privé de l'auteur est arrivé chez le lecteur")
	}
	l, err := r.Layout(ctx, 1)
	must(t, err)
	if len(l.Tables) != 1 || l.Tables[0].Shared != "published" || l.Tables[0].ReadOnly != "Classeur publié en lecture seule" {
		t.Fatalf("tableau du lecteur : %+v", l.Tables)
	}
	if !slices.ContainsFunc(l.Cells, func(c canvasCell) bool { return c.Raw == "Titre du rapport" }) {
		t.Fatalf("cellule libre de l'auteur absente : %+v", l.Cells)
	}
	scripts, err := r.Scripts(ctx)
	must(t, err)
	if len(scripts) != 1 || scripts[0].Name != "bonjour" {
		t.Fatalf("scripts : %+v", scripts)
	}
	st := r.Share.State()
	if !st.Shared || st.CanDesign || st.ReadOnly != "Classeur publié en lecture seule" || st.Version != 1 {
		t.Fatalf("état : %+v", st)
	}

	// Lecture seule partout : saisie, en-tête, SQL, structure.
	for name, fn := range map[string]func() error{
		"saisie":     func() error { _, err := r.SetCell(ctx, 1, 1, 0, "Thé vert"); return err },
		"en-tête":    func() error { _, err := r.SetCell(ctx, 1, 0, 0, "Article"); return err },
		"sous":       func() error { _, err := r.SetCell(ctx, 1, 4, 0, "Biscuit"); return err },
		"SQL UPDATE": func() error { _, err := r.ExecSQL(ctx, `UPDATE Ventes SET Prix = 0`); return err },
		"SQL DELETE": func() error { _, err := r.ExecSQL(ctx, `DELETE FROM Ventes`); return err },
		"renommer":   func() error { return r.RenameTable(ctx, "Ventes", "V") },
		"type":       func() error { return r.SetType(ctx, "Ventes", 0, grid.Integer) },
		"supprimer":  func() error { return r.DeleteTables(ctx, []string{"Ventes"}) },
		"ligne":      func() error { return r.InsertRow(ctx, "Ventes", 1) },
	} {
		if err := fn(); err == nil || !strings.Contains(err.Error(), "Classeur publié en lecture seule") {
			t.Errorf("%s : %v", name, err)
		}
	}
	if got := column(t, r, "Ventes", "Prix"); !slices.Equal(got, []string{"4.5", "2.0", "3.25"}) {
		t.Fatalf("prix modifiés : %v", got)
	}
	// Permis en local : tri, cellules libres, tableaux privés.
	must(t, r.SortTable(ctx, "Ventes", 2, true))
	if got := column(t, r, "Ventes", "Prix"); !slices.Equal(got, []string{"4.5", "3.25", "2.0"}) {
		t.Fatalf("tri local : %v", got)
	}
	_, err = r.SetCell(ctx, 1, 10, 0, "=SOMME(Ventes[Prix])")
	must(t, err)
	paste(t, r, 1, 0, 8, [][]string{{"Perso", "Montant"}, {"x", "5"}})
	must(t, r.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_outbox`).Scan(&cnt))
	if cnt != 0 {
		t.Fatalf("%d entrées dans la file d'un lecteur", cnt)
	}
}

func TestRepublishKeepsLocalAdditions(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	a := author(t)
	info := publish(t, a, fake.As("alice"), workbook.Frozen, "Ventes")
	must(t, fake.Grant(info.WorkbookID, "bob", "reader"))
	r := open(t, fake.As("bob"), info.WorkbookID, "bob")

	// Le lecteur ajoute un tableau privé (même identifiant que le prochain
	// tableau de l'auteur), une cellule libre et une feuille.
	paste(t, r, 1, 20, 0, [][]string{{"Perso", "Montant"}, {"x", "5"}, {"y", "7"}})
	perso := tableID(t, r, tableAt(t, r, 20, 0))
	if perso == tableID(t, r, "Ventes") {
		t.Fatal("même tableau")
	}
	_, err := r.SetCell(ctx, 1, 30, 3, "note perso")
	must(t, err)
	sheet, err := r.AddSheet(ctx)
	must(t, err)
	_, err = r.SetCell(ctx, sheet, 0, 0, "feuille perso")
	must(t, err)

	// L'auteur change une valeur, ajoute un tableau publié, republie.
	var notify []string
	lctx, stop := context.WithCancel(ctx)
	ready := make(chan struct{})
	got := make(chan string, 4)
	go fake.As("bob").Listen(lctx, info.Channel, func() { close(ready) }, func(p string) { got <- p })
	<-ready
	_, err = a.SetCell(ctx, 1, 1, 2, "5")
	must(t, err)
	paste(t, a, 1, 12, 0, [][]string{{"Stock", "Reste"}, {"Café", "8"}})
	stock := tableID(t, a, tableAt(t, a, 12, 0))
	if stock != perso {
		t.Logf("identifiants : stock %d, perso %d", stock, perso)
	}
	must(t, a.RenameTable(ctx, nameOf(t, a, stock), "Stock"))
	info2, err := workbook.Republish(ctx, fake.As("alice"), a.DB(), []int64{tableID(t, a, "Ventes"), tableID(t, a, "Stock")})
	must(t, err)
	if info2.Version != 2 {
		t.Fatalf("version %d", info2.Version)
	}
	select {
	case p := <-got:
		notify = append(notify, p)
	case <-time.After(5 * time.Second):
		t.Fatal("pas de notification")
	}
	stop()
	if notify[0] != "*version:2" {
		t.Fatalf("notification : %v", notify)
	}
	if !r.Share.SetNewVersion(2) || r.Share.State().NewVersion != 2 {
		t.Fatal("nouvelle version non notée")
	}

	// Rechargement chez le lecteur.
	_, err = workbook.Load(ctx, fake.As("bob"), r.DB(), workbook.Source{Server: server, User: "bob", WorkbookID: info.WorkbookID}, false)
	must(t, err)
	must(t, r.ShareChanged(ctx))
	if st := r.Share.State(); st.Version != 2 || st.NewVersion != 0 || st.Title != "Ventes 2026" {
		t.Fatalf("état : %+v", st)
	}
	if got := column(t, r, "Ventes", "Prix"); !slices.Equal(got, []string{"5.0", "2.0", "3.25"}) {
		t.Fatalf("prix rechargés : %v", got)
	}
	if got := column(t, r, "Stock", "Reste"); !slices.Equal(got, []string{"8"}) {
		t.Fatalf("nouveau tableau : %v", got)
	}
	// Ajouts locaux gardés, le tableau privé renuméroté s'il le fallait.
	var name string
	must(t, r.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE id NOT IN (SELECT tbl FROM _lattis_remote_tables WHERE published = 1)`).Scan(&name))
	if got := column(t, r, name, "Montant"); !slices.Equal(got, []string{"5", "7"}) {
		t.Fatalf("tableau privé : %v", got)
	}
	if r.Share.TableShare(tableID(t, r, name)) != "private" {
		t.Fatal("tableau privé devenu publié")
	}
	l, err := r.Layout(ctx, 1)
	must(t, err)
	if !slices.ContainsFunc(l.Cells, func(c canvasCell) bool { return c.Raw == "note perso" }) {
		t.Fatal("cellule libre locale perdue")
	}
	var sheets []string
	for _, s := range l.Sheets {
		sheets = append(sheets, s.Name)
	}
	if len(sheets) != 2 {
		t.Fatalf("feuilles : %v", sheets)
	}
	var free string
	must(t, r.DB().Read().QueryRow(`SELECT raw FROM _lattis_cells WHERE sheet = (SELECT id FROM _lattis_sheets WHERE name = ?) `, sheets[1]).Scan(&free))
	if free != "feuille perso" {
		t.Fatalf("feuille locale : %q", free)
	}
	// Historique vidé (rien ne s'annule à travers un rechargement), classeur
	// toujours cohérent.
	st, err := r.Bus.State(ctx)
	must(t, err)
	if st.Undo != "" {
		t.Fatalf("annulation encore proposée : %q", st.Undo)
	}
	problems, err := r.Check(ctx)
	must(t, err)
	for _, p := range problems {
		t.Error(p)
	}
}

func nameOf(t *testing.T, d *doc.Document, id int64) string {
	t.Helper()
	var n string
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE id = ?`, id).Scan(&n))
	return n
}

func TestOfflineIsReadOnly(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	a := author(t)
	info := publish(t, a, fake.As("alice"), workbook.Frozen, "Ventes")
	path := filepath.Join(t.TempDir(), "Ventes.lattis")
	must(t, a.Save(ctx, path))

	// Le manifeste dit le partage.
	m, err := archive.ReadManifest(path)
	must(t, err)
	if m.Remote == nil || m.Remote.Server != server || m.Remote.WorkbookID != info.WorkbookID || m.Remote.Mode != "frozen" || m.Remote.Version != 1 || len(m.Remote.Since) != 1 {
		t.Fatalf("manifeste : %+v", m.Remote)
	}

	// Rouvert sans serveur : hors ligne, lecture seule, même pour l'auteur.
	d, err := doc.Open(ctx, path)
	must(t, err)
	defer d.Close()
	st := d.Share.State()
	if !st.Shared || st.Online || st.ReadOnly != "Hors ligne, lecture seule" {
		t.Fatalf("état : %+v", st)
	}
	if _, err := d.SetCell(ctx, 1, 1, 0, "x"); !errors.Is(err, workbook.ErrOffline) && (err == nil || !strings.Contains(err.Error(), "Hors ligne")) {
		t.Fatalf("saisie hors ligne : %v", err)
	}
	if _, err := d.ExecSQL(ctx, `UPDATE Ventes SET Prix = 1`); err == nil || !strings.Contains(err.Error(), "Hors ligne, lecture seule") {
		t.Fatalf("SQL hors ligne : %v", err)
	}
	// Tableau privé : modifiable.
	_, err = d.SetCell(ctx, 1, 1, 8, "b")
	must(t, err)
	// De retour en ligne, l'auteur modifie sa copie.
	d.Share.SetOnline(true, "")
	_, err = d.SetCell(ctx, 1, 1, 0, "Café moulu")
	must(t, err)
}

func TestChecksBeforePublishing(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	d := newDoc(t)
	paste(t, d, 1, 0, 0, [][]string{{"lattis_id", "Montant"}, {"a", "1"}, {"b", "2"}})
	tbl := tableID(t, d, tableAt(t, d, 0, 0))
	must(t, d.RenameTable(ctx, nameOf(t, d, tbl), "Comptes"))
	// Une saisie invalide dans une colonne au type fixé.
	must(t, d.SetType(ctx, "Comptes", 1, grid.Integer))
	_, err := d.SetCell(ctx, 1, 1, 1, "beaucoup")
	must(t, err)

	check, err := workbook.Inspect(ctx, d.DB())
	must(t, err)
	b := check.Blocking([]int64{tbl})
	if len(b) != 2 || !strings.Contains(b[0], "lattis_ est réservé") || !strings.Contains(b[1], "1 saisie(s) invalide(s)") {
		t.Fatalf("blocages : %q", b)
	}
	c := check.Tables[0].Columns[1]
	if c.Invalid != 1 || c.Samples[0] != "beaucoup" || c.Remote != "integer" || c.Inferred {
		t.Fatalf("colonne : %+v", c)
	}
	_, err = workbook.Publish(ctx, fake.As("alice"), d.DB(), workbook.Request{Server: server, Title: "Comptes", Mode: workbook.Frozen, Tables: []int64{tbl}})
	if err == nil || !strings.Contains(err.Error(), "publication impossible") {
		t.Fatalf("publication : %v", err)
	}
	if fake.Calls["publish"] != 0 {
		t.Fatal("le serveur a été appelé")
	}
	// Corrigé : nom de colonne, saisie acceptée comme texte.
	_, err = d.SetCell(ctx, 1, 0, 0, "Compte")
	must(t, err)
	must(t, d.AcceptAsText(ctx, "Comptes", 1))
	if got := column(t, d, "Comptes", "Montant"); !slices.Equal(got, []string{"beaucoup", "2"}) {
		t.Fatalf("saisie acceptée comme texte : %v", got)
	}
	check, err = workbook.Inspect(ctx, d.DB())
	must(t, err)
	if b := check.Blocking([]int64{tbl}); len(b) != 0 {
		t.Fatalf("blocages : %q", b)
	}
	// Annuler rend la saisie invalide.
	_, err = d.Bus.Undo(ctx)
	must(t, err)
	if check, _ = workbook.Inspect(ctx, d.DB()); check.Tables[0].Invalid != 1 {
		t.Fatalf("annulation : %+v", check.Tables[0])
	}
	_, err = d.Bus.Redo(ctx)
	must(t, err)
	if !check.Tables[0].Columns[0].Inferred {
		t.Fatal("colonne inférée non signalée")
	}
	for _, tc := range []struct{ name, want string }{
		{"Ventes", ""}, {" Ventes", "espaces"}, {strings.Repeat("é", 32), "trop long"}, {"Lattis_x", "réservé"}, {"", "vide"},
	} {
		if got := workbook.NameProblem(tc.name); (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
			t.Errorf("%q : %q", tc.name, got)
		}
	}
}

func TestPublishFailureLeavesNothing(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	a := author(t)
	fake.SetDown(true)
	_, err := workbook.Publish(ctx, fake.As("alice"), a.DB(), workbook.Request{Server: server, Title: "X", Mode: workbook.Frozen, Tables: []int64{tableID(t, a, "Ventes")}})
	if !errors.Is(err, workbooktest.ErrDown) {
		t.Fatalf("serveur coupé : %v", err)
	}
	i, err := workbook.ReadInfo(ctx, a.DB().Read())
	must(t, err)
	if i != nil {
		t.Fatal("classeur noté publié")
	}
	fake.SetDown(false)
	// Échec en fin de publication : tout est défait côté serveur, rien n'est
	// noté en local.
	fake.Fail = "finish_publish"
	_, err = workbook.Publish(ctx, fake.As("alice"), a.DB(), workbook.Request{Server: server, Title: "X", Mode: workbook.Frozen, Tables: []int64{tableID(t, a, "Ventes")}})
	if err == nil || !strings.Contains(err.Error(), "échec simulé") {
		t.Fatalf("échec : %v", err)
	}
	fake.Fail = ""
	list, _ := fake.As("alice").Workbooks(ctx)
	if len(list) != 0 {
		t.Fatalf("classeur créé : %+v", list)
	}
	if i, _ := workbook.ReadInfo(ctx, a.DB().Read()); i != nil {
		t.Fatal("classeur noté publié")
	}
	// Publier deux fois : non (Republier).
	publish(t, a, fake.As("alice"), workbook.Frozen, "Ventes")
	_, err = workbook.Publish(ctx, fake.As("alice"), a.DB(), workbook.Request{Server: server, Title: "X", Mode: workbook.Frozen, Tables: []int64{tableID(t, a, "Ventes")}})
	if err == nil || !strings.Contains(err.Error(), "Republier") {
		t.Fatalf("deuxième publication : %v", err)
	}
	// Un lecteur ne republie pas.
	list, _ = fake.As("alice").Workbooks(ctx)
	must(t, fake.Grant(list[0].ID, "bob", "reader"))
	r := open(t, fake.As("bob"), list[0].ID, "bob")
	if _, err := workbook.Republish(ctx, fake.As("bob"), r.DB(), nil); err == nil || !strings.Contains(err.Error(), "propriétaires") {
		t.Fatalf("republication par un lecteur : %v", err)
	}
}
