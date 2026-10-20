package collab_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// « Ajouter au classeur partagé » : un tableau créé en local après la
// publication d'un classeur collaboratif entre dans le classeur du
// serveur avec ses lignes, les autres postes le reçoivent, puis il se
// synchronise comme les autres.

// newLocalTable colle un tableau Stock sous Ventes (ligne row) et rend son
// identifiant local.
func newLocalTable(t *testing.T, c *client, name string, row int) int64 {
	t.Helper()
	ctx := context.Background()
	_, err := c.d.Paste(ctx, 1, row, 0, [][]string{
		{"Article", "Stock"},
		{"Vis", "120"},
		{"Écrou", "75"},
	})
	must(t, err)
	var first string
	must(t, c.d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE sheet = 1 AND row = ? AND col = 0`, row).Scan(&first))
	must(t, c.d.RenameTable(ctx, first, name))
	return tableOf(t, c, name)
}

// values lit une colonne d'un tableau local dans l'ordre d'affichage.
func values(t *testing.T, c *client, table, col string) []string {
	t.Helper()
	tbl, cid := tableOf(t, c, table), colID(t, c, table, col)
	rs, err := c.d.DB().Read().Query(`SELECT coalesce(CAST(c_` + strconv.FormatInt(cid, 10) + ` AS TEXT), '') FROM t_` + strconv.FormatInt(tbl, 10) + ` ORDER BY _pos, id`)
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

func alive(t *testing.T, c *client, tbl int64) bool {
	t.Helper()
	var n int
	must(t, c.d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_tables WHERE id = ?`, tbl).Scan(&n))
	return n > 0
}

func anchorOf(t *testing.T, c *client, table string) (sheet, row int64) {
	t.Helper()
	must(t, c.d.DB().Read().QueryRow(`SELECT sheet, row FROM _lattis_tables WHERE name = ?`, table).Scan(&sheet, &row))
	return
}

func TestAddTableToSharedWorkbook(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	tbl := newLocalTable(t, alice, "Stock", 10)
	if s := alice.e.CanAdd(tbl); s != "" {
		t.Fatalf("CanAdd : %q", s)
	}
	name, err := alice.e.AddTable(ctx, tbl)
	must(t, err)
	if name != "Stock" {
		t.Fatalf("nom : %q", name)
	}
	if _, _, _, tables := f.Book(id); strings.Join(tables, ",") != "Ventes,Stock" {
		t.Fatalf("tableaux du serveur : %v", tables)
	}
	if rows := f.Rows(id, "Stock"); len(rows) != 2 || rows[0][2] != "Vis" {
		t.Fatalf("lignes du serveur : %v", rows)
	}
	if alice.d.Share.TableShare(tbl) != "published" {
		t.Fatal("Stock pas partagé chez Alice")
	}
	if s := alice.e.CanAdd(tbl); s == "" {
		t.Fatal("un tableau partagé ne s'ajoute pas deux fois")
	}
	if lay := string(f.Meta(id, "layout")); !strings.Contains(lay, `"Stock"`) {
		t.Fatalf("mise en page du serveur sans Stock : %s", lay)
	}
	// Les lignes d'Alice ne lui reviennent pas (écho écarté), rien en attente.
	r := alice.receive()
	if r.Applied.Updated+r.Applied.Inserted != 0 {
		t.Fatalf("réception d'Alice : %+v", r.Applied)
	}

	// Bob le reçoit (comme sur « *meta »), au même endroit, avec ses lignes.
	rep, err := bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if !strings.Contains(strings.Join(rep.Notices, " ; "), "Tableau « Stock » ajouté au classeur partagé") {
		t.Fatalf("messages de Bob : %v", rep.Notices)
	}
	eq(t, "Bob Article", values(t, bob, "Stock", "Article"), []string{"Vis", "Écrou"})
	eq(t, "Bob Stock", values(t, bob, "Stock", "Stock"), []string{"120", "75"})
	if s, row := anchorOf(t, bob, "Stock"); s != 1 || row != 10 {
		t.Fatalf("place chez Bob : feuille %d ligne %d", s, row)
	}
	if bob.d.Share.TableShare(tableOf(t, bob, "Stock")) != "published" {
		t.Fatal("Stock pas partagé chez Bob")
	}
	if lv, _ := bob.d.Share.TableAccess(tableOf(t, bob, "Stock")); lv != workbook.LevelEditor {
		t.Fatalf("niveau de Bob : %q", lv)
	}
	// Reçu une seconde fois : rien de plus.
	rep, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if strings.Contains(strings.Join(rep.Notices, " ; "), "ajouté") {
		t.Fatalf("tableau reçu deux fois : %v", rep.Notices)
	}

	// Modifications dans les deux sens.
	_, err = alice.d.SetCell(ctx, 1, 11, 1, "130")
	must(t, err)
	alice.send()
	bob.receive()
	eq(t, "Bob après Alice", values(t, bob, "Stock", "Stock"), []string{"130", "75"})
	_, err = bob.d.SetCell(ctx, 1, 12, 1, "80")
	must(t, err)
	_, err = bob.d.SetCell(ctx, 1, 13, 0, "Boulon")
	must(t, err)
	bob.send()
	alice.receive()
	eq(t, "Alice après Bob", values(t, alice, "Stock", "Stock"), []string{"130", "80", ""})
	eq(t, "Alice articles", values(t, alice, "Stock", "Article"), []string{"Vis", "Écrou", "Boulon"})

	// Un nouveau membre l'ouvre depuis le serveur, à sa place.
	must(t, f.Grant(id, "carol", "reader"))
	carol := newClient(t, "carol", openAs(t, f, id, "carol"), f.As("carol"))
	eq(t, "Carol articles", values(t, carol, "Stock", "Article"), []string{"Vis", "Écrou", "Boulon"})
	if _, row := anchorOf(t, carol, "Stock"); row != 10 {
		t.Fatalf("place chez Carol : ligne %d", row)
	}

	// La création du tableau ne s'annule plus : ce serait le supprimer pour
	// tous (oplog.ErrImplicitDrop).
	for {
		var un string
		un, err = alice.d.Bus.Undo(ctx)
		t.Logf("annulé %q : %v", un, err)
		if err != nil {
			break
		}
		if !alive(t, alice, tbl) {
			t.Fatal("annulation : le tableau partagé a disparu")
		}
	}
	var drop oplog.ErrImplicitDrop
	if !errors.As(err, &drop) || !alive(t, alice, tbl) {
		t.Fatalf("annulation de la création : %v", err)
	}
}

func TestAddTableFollowed(t *testing.T) {
	old := collab.Debounce
	collab.Debounce = 20 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f, alice, bob, _ := setup(t)
	go bob.e.Follow(ctx, f.As("bob"), time.Hour, nil)
	go alice.e.Follow(ctx, f.As("alice"), time.Hour, nil)
	waitOnline(t, bob, alice)
	// Une ligne occupe la place chez Bob : le tableau reçu va dessous
	// (cellule d'avant la décision 0018 : un éditeur n'en saisit plus).
	must(t, bob.d.DB().Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (1, 11, 1, 'note de Bob')`)
		return err
	}))
	tbl := newLocalTable(t, alice, "Stock", 10)
	_, err := alice.e.AddTable(ctx, tbl)
	must(t, err)
	waitFor(t, "Stock chez Bob", func() bool { return hasTable(bob, "Stock") })
	if _, row := anchorOf(t, bob, "Stock"); row <= 11 {
		t.Fatalf("Stock recouvre la cellule de Bob : ligne %d", row)
	}
	_, err = alice.d.SetCell(ctx, 1, 11, 1, "121")
	must(t, err)
	alice.e.Kick()
	waitFor(t, "valeur d'Alice chez Bob", func() bool { return values(t, bob, "Stock", "Stock")[0] == "121" })
}

func TestAddTableRefused(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)

	// Éditeur, pas concepteur : il ne crée pas de tableau sur la feuille
	// (cellules libres et mise en page aux concepteurs, décision 0018), ni
	// n'en ajoute au classeur partagé.
	if _, err := bob.d.Paste(ctx, 1, 10, 0, [][]string{{"Article", "Stock"}, {"Vis", "120"}}); !errors.Is(err, workbook.ErrLayout) {
		t.Fatalf("tableau créé par un éditeur : %v", err)
	}
	tb := tableID(t, bob.d)
	if s := bob.e.CanAdd(tb); s == "" {
		t.Fatal("CanAdd d'un éditeur")
	}
	if _, err := bob.e.AddTable(ctx, tb); !errors.Is(err, workbook.ErrAddDesign) {
		t.Fatalf("éditeur : %v", err)
	}

	// Hors ligne : refusé.
	tbl := newLocalTable(t, alice, "Stock", 10)
	alice.d.Share.SetOnline(false, "coupure")
	if _, err := alice.e.AddTable(ctx, tbl); !errors.Is(err, collab.ErrAddOffline) {
		t.Fatalf("hors ligne : %v", err)
	}
	alice.d.Share.SetOnline(true, "")

	// Refus du serveur : rien ne change, ni ici ni là-bas.
	f.Fail = "create_table"
	if _, err := alice.e.AddTable(ctx, tbl); err == nil {
		t.Fatal("échec du serveur non signalé")
	}
	f.Fail = ""
	if alice.d.Share.TableShare(tbl) == "published" {
		t.Fatal("Stock partagé malgré le refus")
	}
	if _, _, _, tables := f.Book(id); strings.Join(tables, ",") != "Ventes" {
		t.Fatalf("tableaux du serveur : %v", tables)
	}
	// Refus du serveur avec son texte : Bob, devenu concepteur, a ajouté
	// un tableau Stock juste avant (pas encore reçu par Alice).
	must(t, f.Grant(id, "bob", "designer"))
	_, err := workbook.RefreshCapabilities(ctx, f.As("bob"), bob.d.Share)
	must(t, err)
	_, err = bob.e.AddTable(ctx, newLocalTable(t, bob, "Notes", 10))
	must(t, err)
	must(t, bob.d.RenameTable(ctx, "Notes", "Stock"))
	bob.send()
	_, err = alice.e.AddTable(ctx, tbl)
	var refused *collab.RefusedError
	if !errors.As(err, &refused) || !strings.Contains(refused.Error(), "Refusé par le serveur") || !strings.Contains(refused.Error(), "existe déjà") {
		t.Fatalf("nom pris : %v", err)
	}
	if alice.d.Share.TableShare(tbl) == "published" {
		t.Fatal("Stock partagé malgré le refus")
	}
	// Renommé, il passe ; Alice reçoit aussi celui de Bob (identifiant de
	// mise en page différent du sien).
	must(t, alice.d.RenameTable(ctx, "Stock", "Stock2"))
	_, err = alice.e.AddTable(ctx, tbl)
	must(t, err)
	_, err = alice.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if !hasTable(alice, "Stock") || !hasTable(alice, "Stock2") {
		t.Fatal("tableaux d'Alice après réception")
	}
	eq(t, "Stock de Bob chez Alice", values(t, alice, "Stock", "Article"), []string{"Vis", "Écrou"})
	lay, err := decodeIDs(f.Meta(id, "layout"))
	must(t, err)
	if len(lay) != 3 || lay[0] == lay[1] || lay[1] == lay[2] || lay[0] == lay[2] {
		t.Fatalf("identifiants de la mise en page : %v", lay)
	}
}

// decodeIDs lit les identifiants des tableaux d'une mise en page publiée.
func decodeIDs(raw []byte) ([]int64, error) {
	var l struct {
		Tables map[string]struct {
			Columns []string `json:"columns"`
			Rows    [][]any  `json:"rows"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, err
	}
	r := l.Tables["_lattis_tables"]
	at := slices.Index(r.Columns, "id")
	var out []int64
	for _, row := range r.Rows {
		f, _ := row[at].(float64)
		out = append(out, int64(f))
	}
	return out, nil
}
