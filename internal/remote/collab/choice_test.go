package collab_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Onglets à choix d'un classeur collaboratif (« Suivi | 2026 ▾ ») : la
// catégorie est publiée avec la mise en page, la valeur choisie reste
// propre à chaque poste (remote/workbook/choice.go).

// setupChoice publie depuis le poste d'Alice un tableau Suivi (Année,
// Site, Nids) sous un onglet à choix par Année réglé sur 2025, avec une
// cellule par valeur (A11 : objectif de l'année) ; Bob (éditeur) l'ouvre
// depuis le serveur.
func setupChoice(t *testing.T, strip bool) (*workbooktest.Fake, *client, *client, string) {
	t.Helper()
	useRoot(t)
	ctx := context.Background()
	f := workbooktest.New()
	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	_, err = d.Paste(ctx, 1, 0, 0, [][]string{
		{"Année", "Site", "Nids"},
		{"2024", "Étang", "3"},
		{"2025", "Étang", "5"},
		{"2025", "Bois", "2"},
		{"2026", "Bois", "4"},
	})
	must(t, err)
	var first string
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE sheet = 1 AND row = 0 AND col = 0`).Scan(&first))
	must(t, d.RenameTable(ctx, first, "Suivi"))
	must(t, d.Model.SetSheetChoice(ctx, 1, "Suivi", "Année", "2025"))
	must(t, d.DB().Write(ctx, func(tx *sql.Tx) error {
		for _, c := range [][2]string{{"2024", "Objectif 4"}, {"2025", "Objectif 5"}, {"2026", "Objectif 6"}} {
			if _, err := tx.Exec(`INSERT INTO _lattis_choice_cells (sheet, row, col, value, raw) VALUES (1, 10, 0, ?, ?)`, c[0], c[1]); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (1, 10, 0, 'Objectif 5')`)
		return err
	}))
	var tbl int64
	must(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = 'Suivi'`).Scan(&tbl))
	info, err := workbook.Publish(ctx, f.As("alice"), d.DB(), workbook.Request{Server: server, User: "alice", Title: "Nids", Mode: workbook.Collaborative, Tables: []int64{tbl}})
	must(t, err)
	if strip {
		// Classeur publié avant que les catégories le soient : ni la
		// colonne choice ni les cellules par valeur dans la mise en page,
		// chez Alice (mise en page reçue) comme sur le serveur.
		old := stripChoices(t, f.Meta(info.WorkbookID, "layout"))
		must(t, f.As("alice").Write(ctx, func(w workbook.Writer) error {
			return w.SaveMeta(ctx, info.WorkbookID, "layout", old)
		}))
		must(t, d.DB().Write(ctx, func(tx *sql.Tx) error {
			_, err := tx.Exec(`UPDATE _lattis_info SET value = ? WHERE key = 'remote_base'`, string(old))
			return err
		}))
	}
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	alice := newClient(t, "alice", d, f.As("alice"))
	must(t, f.Grant(info.WorkbookID, "bob", "editor"))
	bob := newClient(t, "bob", openChoice(t, f, info.WorkbookID, "bob"), f.As("bob"))
	return f, alice, bob, info.WorkbookID
}

func openChoice(t *testing.T, f *workbooktest.Fake, id, user string) *doc.Document {
	t.Helper()
	ctx := context.Background()
	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	_, err = workbook.Load(ctx, f.As(user), d.DB(), workbook.Source{Server: server, User: user, WorkbookID: id, Title: "Nids"}, true)
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	return d
}

// stripChoices retire d'une mise en page la colonne choice et les cellules
// par valeur (format d'avant).
func stripChoices(t *testing.T, raw []byte) []byte {
	t.Helper()
	var l map[string]any
	must(t, json.Unmarshal(raw, &l))
	tables := l["tables"].(map[string]any)
	delete(tables, "_lattis_choice_cells")
	sheets := tables["_lattis_sheets"].(map[string]any)
	cols := sheets["columns"].([]any)
	at := slices.Index(cols, any("choice"))
	if at < 0 {
		t.Fatal("mise en page publiée sans la catégorie de l'onglet à choix")
	}
	sheets["columns"] = slices.Delete(cols, at, at+1)
	for i, row := range sheets["rows"].([]any) {
		r := row.([]any)
		sheets["rows"].([]any)[i] = slices.Delete(r, at, at+1)
	}
	out, err := json.Marshal(l)
	must(t, err)
	return out
}

// tab rend la catégorie et la valeur de l'onglet de la feuille 1 ("" :
// aucun onglet à choix).
func tab(t *testing.T, c *client) (name, value string) {
	t.Helper()
	ch, err := c.d.Model.SheetChoice(context.Background(), 1)
	must(t, err)
	if ch == nil {
		return "", ""
	}
	if ch.All {
		return ch.Name, "*"
	}
	return ch.Name, ch.Value
}

// cell lit une cellule libre de la feuille 1.
func cell(t *testing.T, c *client, row, col int) string {
	t.Helper()
	var raw string
	err := c.d.DB().Read().QueryRow(`SELECT raw FROM _lattis_cells WHERE sheet = 1 AND row = ? AND col = ?`, row, col).Scan(&raw)
	if err == sql.ErrNoRows {
		return ""
	}
	must(t, err)
	return raw
}

func wantTab(t *testing.T, what string, c *client, name, value string) {
	t.Helper()
	if n, v := tab(t, c); n != name || v != value {
		t.Fatalf("%s : onglet %q · %q, attendu %q · %q", what, n, v, name, value)
	}
}

func TestChoiceTabShared(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setupChoice(t, false)
	// Publiée sans la valeur choisie d'Alice.
	var lay struct {
		Tables map[string]workbook.LayoutRows `json:"tables"`
	}
	must(t, json.Unmarshal(f.Meta(id, "layout"), &lay))
	sheets := lay.Tables["_lattis_sheets"]
	if at := slices.Index(sheets.Columns, "choice"); at < 0 || sheets.Rows[0][at] == nil {
		t.Fatalf("catégorie absente de la mise en page : %+v", sheets)
	} else if s := sheets.Rows[0][at].(string); s != `{"tbl":1,"col":1,"name":"Année"}` {
		t.Fatalf("catégorie publiée : %s", s)
	}
	if n := len(lay.Tables["_lattis_choice_cells"].Rows); n != 3 {
		t.Fatalf("cellules par valeur publiées : %d", n)
	}

	// Bob l'ouvre depuis le lien : l'onglet à choix, sur la dernière année,
	// avec la cellule de cette année.
	wantTab(t, "Bob à l'ouverture", bob, "Année", "2026")
	if v := cell(t, bob, 10, 0); v != "Objectif 6" {
		t.Fatalf("cellule par valeur chez Bob : %q", v)
	}
	wantTab(t, "Alice", alice, "Année", "2025")

	// Son choix lui reste : réception de la mise en page, rechargement.
	must(t, bob.d.Model.ChooseSheetValue(ctx, 1, "2024", false))
	_, err := bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	wantTab(t, "Bob après réception", bob, "Année", "2024")
	_, err = workbook.Load(ctx, f.As("bob"), bob.d.DB(), workbook.Source{Server: server, User: "bob", WorkbookID: id}, false)
	must(t, err)
	must(t, bob.d.ShareChanged(ctx))
	wantTab(t, "Bob après rechargement", bob, "Année", "2024")
	if v := cell(t, bob, 10, 0); v != "Objectif 4" {
		t.Fatalf("cellule par valeur chez Bob après rechargement : %q", v)
	}

	// Alice change la catégorie après la publication : Bob la reçoit.
	must(t, alice.d.Model.SetSheetChoice(ctx, 1, "Suivi", "Site", "Étang"))
	must(t, alice.e.PublishChoices(ctx, false))
	rep, err := bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if !rep.Changed() {
		t.Fatal("réception de la catégorie : grille pas à relire")
	}
	wantTab(t, "Bob, nouvelle catégorie", bob, "Site", "Bois")
	wantTab(t, "Alice", alice, "Site", "Étang")
	// Rien ne repart de chez Alice ni ne revient chez Bob.
	must(t, alice.e.PublishChoices(ctx, true))
	rep, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if rep.Choices {
		t.Fatal("catégorie reçue deux fois")
	}

	// Bob (éditeur) ne change pas la catégorie : la mise en page est aux
	// concepteurs (décision 0018) ; il garde le choix de la valeur.
	if err := bob.d.Model.SetSheetChoice(ctx, 1, "Suivi", "Année", "2025"); !errors.Is(err, workbook.ErrLayout) {
		t.Fatalf("catégorie posée par un éditeur : %v", err)
	}
	must(t, bob.d.Model.ChooseSheetValue(ctx, 1, "Étang", false))
	wantTab(t, "Bob, sa valeur", bob, "Site", "Étang")

	// Alice retire l'onglet à choix : il disparaît chez Bob.
	must(t, alice.d.Model.ClearSheetChoice(ctx, 1))
	must(t, alice.e.PublishChoices(ctx, false))
	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	wantTab(t, "Bob, onglet retiré", bob, "", "")

	// Un nouveau membre l'ouvre sans onglet à choix.
	must(t, f.Grant(id, "carol", "reader"))
	carol := newClient(t, "carol", openChoice(t, f, id, "carol"), f.As("carol"))
	wantTab(t, "Carol", carol, "", "")
}

// Classeur publié avant que la catégorie le soit (le cas signalé : ouvert
// depuis le lien, plus de tri par année).
func TestChoiceTabOldLayout(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setupChoice(t, true)
	// Comme avant : rien chez Bob ; son propre choix lui reste.
	wantTab(t, "Bob, mise en page d'avant", bob, "", "")
	// Catégorie posée par Bob quand les éditeurs le pouvaient encore (avant
	// la décision 0018) : hors du journal, comme dans son cache.
	must(t, bob.d.DB().Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE _lattis_sheets SET choice = '{"tbl":1,"col":1,"name":"Année","value":"2024"}' WHERE id = 1`)
		return err
	}))
	_, err := bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	_, err = workbook.Load(ctx, f.As("bob"), bob.d.DB(), workbook.Source{Server: server, User: "bob", WorkbookID: id}, false)
	must(t, err)
	must(t, bob.d.ShareChanged(ctx))
	wantTab(t, "Bob, son choix", bob, "Année", "2024")

	// Alice se connecte : sa catégorie part (droit de conception) ; Bob la
	// reçoit en gardant son année, Carol l'ouvre sur la dernière.
	must(t, alice.e.PublishChoices(ctx, true))
	rep, err := bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	if !rep.Choices {
		t.Fatal("catégorie d'Alice pas reçue par Bob")
	}
	wantTab(t, "Bob après Alice", bob, "Année", "2024")
	if v := cell(t, bob, 10, 0); v != "Objectif 4" {
		t.Fatalf("cellule par valeur chez Bob : %q", v)
	}
	must(t, f.Grant(id, "carol", "reader"))
	carol := newClient(t, "carol", openChoice(t, f, id, "carol"), f.As("carol"))
	wantTab(t, "Carol", carol, "Année", "2026")
	if v := cell(t, carol, 10, 0); v != "Objectif 6" {
		t.Fatalf("cellule par valeur chez Carol : %q", v)
	}
}

// Suivi du serveur : la catégorie changée par Alice part au tour qui suit
// (Kick) et arrive chez Bob sur « *meta ».
func TestChoiceTabFollowed(t *testing.T) {
	old := collab.Debounce
	collab.Debounce = 20 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f, alice, bob, _ := setupChoice(t, false)
	go bob.e.Follow(ctx, f.As("bob"), time.Hour, nil)
	go alice.e.Follow(ctx, f.As("alice"), time.Hour, nil)
	waitOnline(t, bob, alice)
	must(t, alice.d.Model.SetSheetChoice(ctx, 1, "Suivi", "Site", "Étang"))
	alice.e.Kick()
	waitFor(t, "catégorie d'Alice chez Bob", func() bool {
		n, _ := tab(t, bob)
		return n == "Site"
	})
	wantTab(t, "Bob", bob, "Site", "Bois")
}

// Catégorie calculée par une formule : publiée avec sa formule ; Bob la
// reçoit et garde sa valeur tant que la formule ne change pas.
func TestChoiceTabFormulaShared(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setupChoice(t, false)
	must(t, alice.d.Model.SetSheetChoiceFormula(ctx, 1, "Taille", `=SI([Nids]>3;"Gros";"Petit")`, "Gros"))
	must(t, alice.e.PublishChoices(ctx, false))
	var lay struct {
		Tables map[string]workbook.LayoutRows `json:"tables"`
	}
	must(t, json.Unmarshal(f.Meta(id, "layout"), &lay))
	sheets := lay.Tables["_lattis_sheets"]
	at := slices.Index(sheets.Columns, "choice")
	s, _ := sheets.Rows[0][at].(string)
	var def map[string]any
	must(t, json.Unmarshal([]byte(s), &def))
	if len(def) != 2 || def["name"] != "Taille" || def["formula"] != `=IF([Nids]>3, "Gros", "Petit")` {
		t.Fatalf("catégorie publiée : %s", s)
	}
	_, err := bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	wantTab(t, "Bob, catégorie calculée", bob, "Taille", "Petit")
	ch, err := bob.d.Model.SheetChoice(ctx, 1)
	must(t, err)
	if ch.Formula != `=SI([Nids]>3; "Gros"; "Petit")` {
		t.Fatalf("formule chez Bob : %q", ch.Formula)
	}
	l, err := bob.d.Model.Layout(ctx, 1)
	must(t, err)
	if tb := l.Tables[0]; !tb.ByFormula || tb.Rows != 2 || tb.Total != 4 {
		t.Fatalf("tableau chez Bob : %+v", tb)
	}
	// Sa valeur lui reste à la réception suivante ; une autre formule est
	// une autre catégorie (dernière valeur).
	must(t, bob.d.Model.ChooseSheetValue(ctx, 1, "Gros", false))
	must(t, alice.e.PublishChoices(ctx, true))
	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	wantTab(t, "Bob, sa valeur", bob, "Taille", "Gros")
	must(t, alice.d.Model.SetSheetChoiceFormula(ctx, 1, "Taille", `=SI([Nids]>4;"Gros";"Petit")`, "Gros"))
	must(t, alice.e.PublishChoices(ctx, false))
	_, err = bob.e.ReceiveStructure(ctx, nil, nil)
	must(t, err)
	wantTab(t, "Bob, autre formule", bob, "Taille", "Petit")
}
