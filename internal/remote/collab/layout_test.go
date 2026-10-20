package collab_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Mise en page en direct (remote/workbook/livelayout.go, décision 0018) :
// cellules libres, largeurs, feuilles… envoyées par les concepteurs,
// reçues par tous.

func (c *client) syncLayout(force bool) collab.Report {
	c.t.Helper()
	r, err := c.e.SyncLayout(context.Background(), force)
	must(c.t, err)
	return r
}

// free lit une cellule libre (raw, valeur affichée) d'une feuille.
func free(t *testing.T, c *client, sheet int64, row, col int) (raw, value string) {
	t.Helper()
	l, err := c.d.Model.Layout(context.Background(), sheet)
	must(t, err)
	for _, x := range l.Cells {
		if x.Row == row && x.Col == col {
			if x.Value == "" {
				return x.Raw, x.Raw // texte : affiché tel quel
			}
			return x.Raw, x.Value
		}
	}
	return "", ""
}

func wantFree(t *testing.T, what string, c *client, sheet int64, row, col int, raw, value string) {
	t.Helper()
	if r, v := free(t, c, sheet, row, col); r != raw || v != value {
		t.Fatalf("%s : cellule (%d, %d) %q = %q, attendu %q = %q", what, row, col, r, v, raw, value)
	}
}

func width(t *testing.T, c *client, sheet int64, col int) int {
	t.Helper()
	var w int
	err := c.d.DB().Read().QueryRow(`SELECT width FROM _lattis_col_widths WHERE sheet = ? AND col = ?`, sheet, col).Scan(&w)
	if err == sql.ErrNoRows {
		return 0
	}
	must(t, err)
	return w
}

// designer fait de Bob un concepteur (droits relus).
func designer(t *testing.T, f *workbooktest.Fake, id string, c *client) {
	t.Helper()
	must(t, f.Grant(id, c.name, "designer"))
	_, err := workbook.RefreshCapabilities(context.Background(), f.As(c.name), c.d.Share)
	must(t, err)
}

// Le cas signalé : « test » saisi dans une cellule libre arrive chez les
// autres, formule recalculée, et chez qui ouvre le classeur ensuite.
func TestLayoutFreeCellLive(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	_, err := alice.d.SetCell(ctx, 1, 10, 5, "test")
	must(t, err)
	_, err = alice.d.SetCell(ctx, 1, 11, 5, "=2*21")
	must(t, err)
	if !alice.e.LayoutDirty() {
		t.Fatal("mise en page modifiée non relevée")
	}
	alice.syncLayout(false)
	if alice.e.LayoutDirty() {
		t.Fatal("mise en page envoyée encore à envoyer")
	}
	rep := bob.syncLayout(true)
	if !rep.Changed() {
		t.Fatal("réception : grille pas à relire")
	}
	wantFree(t, "Bob", bob, 1, 10, 5, "test", "test")
	wantFree(t, "Bob", bob, 1, 11, 5, "=2*21", "42")
	// Pas d'écho : rien ne revient chez Alice, rien de plus chez Bob.
	if rep := alice.syncLayout(true); rep.Changed() {
		t.Fatal("écho chez Alice")
	}
	if rep := bob.syncLayout(true); rep.Changed() {
		t.Fatal("reçu deux fois")
	}
	// Alice efface la cellule : effacée chez Bob.
	_, err = alice.d.SetCell(ctx, 1, 10, 5, "")
	must(t, err)
	alice.syncLayout(false)
	bob.syncLayout(true)
	wantFree(t, "Bob, cellule effacée", bob, 1, 10, 5, "", "")
	// Un nouveau membre l'ouvre avec la formule.
	must(t, f.Grant(id, "carol", "reader"))
	carol := newClient(t, "carol", openAs(t, f, id, "carol"), f.As("carol"))
	wantFree(t, "Carol", carol, 1, 11, 5, "=2*21", "42")
}

// Deux concepteurs : une largeur d'un côté, une cellule de l'autre, tout
// est gardé ; une même cellule des deux côtés : la dernière écriture
// l'emporte, conflit compté.
func TestLayoutMergeAndConflict(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	designer(t, f, id, bob)
	must(t, bob.d.Model.SetColumnWidth(ctx, 1, 6, 150))
	_, err := alice.d.SetCell(ctx, 1, 12, 0, "d'Alice")
	must(t, err)
	alice.syncLayout(false)
	bob.syncLayout(false) // fusion avec celle d'Alice
	alice.syncLayout(true)
	for _, c := range []*client{alice, bob} {
		wantFree(t, c.name, c, 1, 12, 0, "d'Alice", "d'Alice")
		if w := width(t, c, 1, 6); w != 150 {
			t.Fatalf("%s : largeur %d", c.name, w)
		}
	}
	// Même cellule des deux côtés.
	_, err = alice.d.SetCell(ctx, 1, 13, 0, "Alice")
	must(t, err)
	_, err = bob.d.SetCell(ctx, 1, 13, 0, "Bob")
	must(t, err)
	alice.syncLayout(false)
	rep := bob.syncLayout(false)
	if len(rep.Notices) == 0 || bob.d.Share.State().LayoutConflicts != 1 {
		t.Fatalf("conflit : %+v, état %+v", rep, bob.d.Share.State())
	}
	alice.syncLayout(true)
	wantFree(t, "Alice", alice, 1, 13, 0, "Bob", "Bob")
	wantFree(t, "Bob", bob, 1, 13, 0, "Bob", "Bob")
}

// Feuilles ajoutées, renommées, colorées : suivies ; deux feuilles
// ajoutées au même moment sous le même identifiant restent deux.
func TestLayoutSheets(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	designer(t, f, id, bob)
	s, err := alice.d.Model.AddSheet(ctx)
	must(t, err)
	must(t, alice.d.Model.RenameSheet(ctx, s, "Budget"))
	_, err = alice.d.SetCell(ctx, s, 0, 0, "Total")
	must(t, err)
	must(t, alice.d.Model.RenameSheet(ctx, 1, "Ventes"))
	// Bob ajoute la sienne au même moment (même identifiant).
	sb, err := bob.d.Model.AddSheet(ctx)
	must(t, err)
	must(t, bob.d.Model.RenameSheet(ctx, sb, "Notes"))
	_, err = bob.d.SetCell(ctx, sb, 0, 0, "Note de Bob")
	must(t, err)
	if sb != s {
		t.Fatalf("identifiants : %d et %d", s, sb)
	}
	alice.syncLayout(false)
	bob.syncLayout(false)
	alice.syncLayout(true)
	bob.syncLayout(true)
	names := func(c *client) map[string]string {
		out := map[string]string{}
		rs, err := c.d.DB().Read().Query(`SELECT s.name, coalesce((SELECT raw FROM _lattis_cells WHERE sheet = s.id AND row = 0 AND col = 0), '') FROM _lattis_sheets s`)
		must(t, err)
		defer rs.Close()
		for rs.Next() {
			var n, v string
			must(t, rs.Scan(&n, &v))
			out[n] = v
		}
		return out
	}
	for _, c := range []*client{alice, bob} {
		got := names(c)
		if got["Budget"] != "Total" || got["Notes"] != "Note de Bob" || len(got) != 3 {
			t.Fatalf("%s : feuilles %v", c.name, got)
		}
		if _, ok := got["Ventes"]; !ok {
			t.Fatalf("%s : feuille 1 pas renommée : %v", c.name, got)
		}
	}
	// Un éditeur reçoit tout aussi.
	must(t, f.Grant(id, "carol", "editor"))
	carol := newClient(t, "carol", openAs(t, f, id, "carol"), f.As("carol"))
	if got := names(carol); got["Notes"] != "Note de Bob" {
		t.Fatalf("Carol : %v", got)
	}
	// Feuille supprimée par Alice : chez Bob aussi.
	var budget int64
	must(t, bob.d.DB().Read().QueryRow(`SELECT id FROM _lattis_sheets WHERE name = 'Budget'`).Scan(&budget))
	must(t, alice.d.Model.DeleteSheet(ctx, s))
	alice.syncLayout(false)
	bob.syncLayout(true)
	if got := names(bob); len(got) != 2 {
		t.Fatalf("Bob après suppression : %v", got)
	}
}

// Hors ligne : la mise en page attend, puis part à la reconnexion
// (décision 0014).
func TestLayoutOffline(t *testing.T) {
	ctx := context.Background()
	_, alice, bob, _ := setup(t)
	alice.d.Share.SetOnline(false, "coupure")
	_, err := alice.d.SetCell(ctx, 1, 10, 0, "hors ligne")
	must(t, err)
	must(t, alice.d.Model.SetColumnWidth(ctx, 1, 0, 200))
	alice.syncLayout(false)
	alice.e.FlushLayout(ctx)
	if !alice.d.Share.LayoutUnsent() {
		t.Fatal("hors ligne : mise en page non notée à envoyer")
	}
	bob.syncLayout(true)
	wantFree(t, "Bob pendant la coupure", bob, 1, 10, 0, "", "")
	alice.d.Share.SetOnline(true, "")
	alice.syncLayout(true) // connexion
	bob.syncLayout(true)
	wantFree(t, "Bob après la reconnexion", bob, 1, 10, 0, "hors ligne", "hors ligne")
	if w := width(t, bob, 1, 0); w != 200 {
		t.Fatalf("largeur chez Bob : %d", w)
	}
}

// Thème du classeur et mise en forme d'une cellule libre.
func TestLayoutThemeAndStyle(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	must(t, alice.d.DB().Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO _lattis_info (key, value) VALUES ('theme', '{"name":"Marine","table":"#123456","source":"#654321"}') ON CONFLICT (key) DO UPDATE SET value = excluded.value`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO _lattis_cell_styles (sheet, row, col, style) VALUES (1, 10, 0, '{"b":true}')`)
		return err
	}))
	alice.syncLayout(false)
	var lay struct {
		Theme json.RawMessage `json:"theme"`
	}
	must(t, json.Unmarshal(f.Meta(id, "layout"), &lay))
	if len(lay.Theme) == 0 {
		t.Fatal("thème pas envoyé")
	}
	bob.syncLayout(true)
	var style, theme string
	must(t, bob.d.DB().Read().QueryRow(`SELECT style FROM _lattis_cell_styles WHERE sheet = 1 AND row = 10 AND col = 0`).Scan(&style))
	must(t, bob.d.DB().Read().QueryRow(`SELECT value FROM _lattis_info WHERE key = 'theme'`).Scan(&theme))
	if style != `{"b":true}` || theme == "" {
		t.Fatalf("Bob : style %q, thème %q", style, theme)
	}
}

// Suivi : la cellule d'Alice part LayoutDelay après la saisie et arrive
// chez Bob sur « *meta ».
func TestLayoutFollowed(t *testing.T) {
	oldD, oldL := collab.Debounce, collab.LayoutDelay
	collab.Debounce, collab.LayoutDelay = 20*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { collab.Debounce, collab.LayoutDelay = oldD, oldL })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f, alice, bob, _ := setup(t)
	go bob.e.Follow(ctx, f.As("bob"), time.Hour, nil)
	go alice.e.Follow(ctx, f.As("alice"), time.Hour, nil)
	waitOnline(t, bob, alice)
	_, err := alice.d.SetCell(ctx, 1, 10, 5, "test")
	must(t, err)
	alice.e.Kick()
	waitFor(t, "cellule d'Alice chez Bob", func() bool {
		r, _ := free(t, bob, 1, 10, 5)
		return r == "test"
	})
}
