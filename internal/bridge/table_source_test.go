package bridge

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// sourceHost relève les bilans des envois faits à l'enregistrement.
type sourceHost struct {
	NoHost
	got chan TableSourceNotice
}

func (h *sourceHost) Emit(event string, data any) bool {
	if event == TableSourceEvent {
		h.got <- data.(TableSourceNotice)
	}
	return true
}

// sourceFixture : un classeur local avec le tableau Produit / Prix / Note,
// et un serveur de test vu par alice (API SQL et envoi HTTP).
func sourceFixture(t *testing.T) (*workbooktest.Fake, *DocumentService, *sourceHost, string) {
	t.Helper()
	workRoot(t)
	isolateRecent(t)
	ctx := context.Background()
	fake := workbooktest.New()
	old, oldPush, oldPoll := shareServer, sharePusher, sourcePoll
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		return fake.As("alice"), auth.User{PGRole: "alice"}, "Serveur de test", nil
	}
	sharePusher = func(ctx context.Context, server string) (workbook.Pusher, error) { return fake.Pusher("alice"), nil }
	sourcePoll = 5 * time.Millisecond
	t.Cleanup(func() { shareServer, sharePusher, sourcePoll = old, oldPush, oldPoll })
	s := &DocumentService{}
	h := &sourceHost{got: make(chan TableSourceNotice, 8)}
	UseHost(h, nil, s)
	t.Cleanup(func() { s.CloseDocument(ctx) })
	_, err := s.New(ctx)
	mustShare(t, err)
	_, err = s.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix", "Note"}, {"Café", "4,5", "privé"}, {"Thé", "2", "privé"}})
	mustShare(t, err)
	l, err := s.Layout(ctx, 1)
	mustShare(t, err)
	if len(l.Tables) != 1 {
		t.Fatalf("tableaux : %+v", l.Tables)
	}
	return fake, s, h, l.Tables[0].Name
}

func TestTableSourceWithoutPublishing(t *testing.T) {
	fake, s, h, table := sourceFixture(t)
	ctx := context.Background()

	// Classeur local : la boîte propose les deux voies.
	info, err := s.DataShareInfo(ctx, table)
	mustShare(t, err)
	if info.Choice != "workbook" || info.Reason != "" || info.Source != nil {
		t.Fatalf("choix : %+v", info)
	}
	ti, err := s.TableSourceInfo(ctx, table, dataServer)
	mustShare(t, err)
	if ti.Reason != "" || len(ti.Fields) != 3 || ti.Link != nil {
		t.Fatalf("source à créer : %+v", ti)
	}

	// Le tableau seul, sans la colonne Note, mise à jour par la clé Produit.
	link, err := s.CreateTableSource(ctx, TableSourceRequest{Server: dataServer, Table: table, Name: "Prix du jour",
		Columns: []string{"Produit", "Prix"}, Mode: workbook.PushUpsert, Keys: []string{"Produit"}, OnSave: true, Inherit: true})
	mustShare(t, err)
	if link.LastError != "" || link.LastResult == nil || link.LastResult.Inserted != 2 || link.SQLName != "prix_du_jour" || link.WorkbookID == "" {
		t.Fatalf("source créée : %+v", link)
	}
	if !strings.HasPrefix(fake.LastPush, "Produit;Prix\nCafé;4.5\n") || strings.Contains(fake.LastPush, "privé") {
		t.Fatalf("CSV envoyé : %q", fake.LastPush)
	}
	cols, rows := fake.SourceRows(link.Pipeline)
	if !slices.Equal(cols, []string{"Produit:text", "Prix:decimal"}) || len(rows) != 2 || rows[0][0] != "Café" || rows[0][1] != 4.5 {
		t.Fatalf("copie du serveur : %v %v", cols, rows)
	}
	// Le classeur n'est pas publié : aucun classeur visible, pas de partage.
	if list, _ := fake.As("alice").Workbooks(ctx); len(list) != 0 {
		t.Fatalf("classeur publié par erreur : %+v", list)
	}
	if st := s.ShareState(); st.Shared {
		t.Fatalf("partage : %+v", st)
	}
	cat, err := s.DataCatalog(ctx, dataServer)
	mustShare(t, err)
	if len(cat.Datasets) != 1 || cat.Datasets[0].Source == nil || cat.Datasets[0].Source.WorkbookID != link.WorkbookID {
		t.Fatalf("catalogue : %+v", cat.Datasets)
	}
	if st, _ := s.State(ctx); !st.Dirty {
		t.Fatal("le lien est gardé dans le classeur : il doit être à enregistrer")
	}
	if info, _ := s.DataShareInfo(ctx, table); info.Source == nil || info.Source.Pipeline != link.Pipeline {
		t.Fatalf("source connue de la boîte : %+v", info)
	}

	// Mettre à jour : une ligne modifiée.
	_, err = s.SetCell(ctx, 1, 2, 1, "3")
	mustShare(t, err)
	link, err = s.PushTableSource(ctx, table)
	mustShare(t, err)
	if r := link.LastResult; r.Updated != 1 || r.Inserted != 0 || r.RowsRead != 2 {
		t.Fatalf("envoi : %+v", r)
	}
	if got := PushSummary(table, link.LastResult, 0); !strings.Contains(got, "1 modifiée") {
		t.Fatal(got)
	}

	// Enregistré puis rouvert : le lien suit le fichier ; l'enregistrement envoie.
	path := filepath.Join(t.TempDir(), "prix.lattis")
	d, err := s.current()
	mustShare(t, err)
	mustShare(t, d.Save(ctx, path))
	mustShare(t, s.CloseDocument(ctx))
	_, err = s.Open(ctx, path)
	mustShare(t, err)
	links, err := s.TableSources(ctx)
	mustShare(t, err)
	if len(links) != 1 || links[0].TableName != table || !links[0].OnSave || links[0].Pipeline != link.Pipeline || !slices.Equal(links[0].Columns, []string{"Produit", "Prix"}) {
		t.Fatalf("liens relus : %+v", links)
	}
	_, err = s.SetCell(ctx, 1, 3, 0, "Chocolat")
	mustShare(t, err)
	_, err = s.SetCell(ctx, 1, 3, 1, "5")
	mustShare(t, err)
	_, err = s.Save(ctx, false)
	mustShare(t, err)
	select {
	case n := <-h.got:
		if n.Failed || !strings.Contains(n.Message, "1 ajoutée") {
			t.Fatalf("envoi à l'enregistrement : %+v", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pas d'envoi à l'enregistrement")
	}
	if _, rows := fake.SourceRows(link.Pipeline); len(rows) != 3 {
		t.Fatalf("après l'enregistrement : %v", rows)
	}

	// Modifier : Note ajoutée, plus d'envoi à l'enregistrement.
	link, err = s.UpdateTableSource(ctx, TableSourceRequest{Table: table, Name: "Produits", Columns: []string{"Produit", "Prix", "Note"}, Mode: workbook.PushUpsert, Keys: []string{"Produit"}, Inherit: true})
	mustShare(t, err)
	if cols, _ := fake.SourceRows(link.Pipeline); !slices.Equal(cols, []string{"Produit:text", "Prix:decimal", "Note:text"}) || link.OnSave {
		t.Fatalf("colonne ajoutée : %v %+v", cols, link)
	}
	// Catalogue : source poussée, avec son classeur d'origine.
	if cat, _ := s.DataCatalog(ctx, dataServer); cat.Datasets[0].Source.Kind != "source" || cat.Datasets[0].Source.PipelineID != link.Pipeline ||
		cat.Datasets[0].Source.Origin == nil || cat.Datasets[0].Source.Origin.Table != table {
		t.Fatalf("origine au catalogue : %+v", cat.Datasets[0].Source)
	}
	// Mode changé (migration 0028) : remplacement, la clé tombe.
	link, err = s.UpdateTableSource(ctx, TableSourceRequest{Table: table, Name: "Produits", Columns: []string{"Produit", "Prix"}, Mode: workbook.PushReplace, Inherit: true})
	mustShare(t, err)
	if link.Mode != workbook.PushReplace || len(link.Keys) != 0 || link.LastError != "" || link.LastResult.Deleted != 3 {
		t.Fatalf("mode changé : %+v %+v", link, link.LastResult)
	}
	if list, _ := s.PushedSources(ctx, dataServer); len(list) != 1 || list[0].Mode != workbook.PushReplace || list[0].Origin.Table != table {
		t.Fatalf("mes sources poussées : %+v", list)
	}
	// Suspendue : rien n'est envoyé, ni à la demande ni à l'enregistrement ; reprise.
	link, err = s.UpdateTableSource(ctx, TableSourceRequest{Table: table, Name: "Produits", Columns: []string{"Produit", "Prix"}, Mode: workbook.PushReplace, Suspended: true, Inherit: true})
	mustShare(t, err)
	if !link.Suspended {
		t.Fatalf("suspension : %+v", link)
	}
	if _, err := s.PushTableSource(ctx, table); err == nil || !strings.Contains(err.Error(), "suspendue") {
		t.Fatalf("envoi d'une source suspendue : %v", err)
	}
	link, err = s.UpdateTableSource(ctx, TableSourceRequest{Table: table, Name: "Produits", Columns: []string{"Produit", "Prix"}, Mode: workbook.PushReplace, Inherit: true})
	mustShare(t, err)
	if link.Suspended || link.LastError != "" {
		t.Fatalf("reprise : %+v", link)
	}

	// Retirer : la donnée quitte le catalogue, le lien est oublié.
	mustShare(t, s.DropTableSource(ctx, table))
	if cat, _ := s.DataCatalog(ctx, dataServer); len(cat.Datasets) != 0 {
		t.Fatalf("catalogue après retrait : %+v", cat.Datasets)
	}
	if links, _ := s.TableSources(ctx); len(links) != 0 {
		t.Fatalf("liens après retrait : %+v", links)
	}
	if info, _ := s.DataShareInfo(ctx, table); info.Choice != "workbook" {
		t.Fatalf("après retrait : %+v", info)
	}
}

// Colonnes renommées, converties, supprimées en local : le serveur suit
// avant l'envoi suivant (fonctions de structure).
func TestTableSourceColumnsFollow(t *testing.T) {
	fake, s, _, table := sourceFixture(t)
	ctx := context.Background()
	link, err := s.CreateTableSource(ctx, TableSourceRequest{Server: dataServer, Table: table, Name: "Prix", Columns: []string{"Produit", "Prix", "Note"}, Inherit: true})
	mustShare(t, err)
	if link.LastError != "" || link.Mode != workbook.PushReplace {
		t.Fatalf("source : %+v", link)
	}
	d, err := s.current()
	mustShare(t, err)
	var tbl int64
	mustShare(t, d.DB().Read().QueryRowContext(ctx, `SELECT id FROM _lattis_tables WHERE name = ?`, table).Scan(&tbl))
	_, err = s.Call(ctx, "columns.rename", map[string]any{"table": table, "column": "Prix", "name": "Tarif"})
	mustShare(t, err)
	_, err = s.Call(ctx, "columns.delete", map[string]any{"table": table, "column": "Note"})
	mustShare(t, err)
	link, err = s.PushTableSource(ctx, table)
	mustShare(t, err)
	cols, rows := fake.SourceRows(link.Pipeline)
	if !slices.Equal(cols, []string{"Produit:text", "Tarif:decimal"}) || len(rows) != 2 || rows[1][1] != 2.0 {
		t.Fatalf("après renommage et suppression : %v %v (%+v)", cols, rows, link)
	}
	if !slices.Equal(link.Columns, []string{"Produit", "Tarif"}) {
		t.Fatalf("lien : %+v", link)
	}
	if fake.Calls["add_column"] != 0 {
		t.Fatalf("colonne ajoutée sans raison : %v", fake.Calls)
	}
}

func TestTableSourceRefusals(t *testing.T) {
	fake, s, _, table := sourceFixture(t)
	ctx := context.Background()

	// Serveur sans les sources poussées : la voie est fermée, avec la raison.
	fake.NoPushedSources = true
	ti, err := s.TableSourceInfo(ctx, table, dataServer)
	mustShare(t, err)
	if ti.Reason != unsupportedMessage {
		t.Fatalf("serveur ancien : %+v", ti)
	}
	if _, err := s.CreateTableSource(ctx, TableSourceRequest{Server: dataServer, Table: table, Name: "x", Columns: []string{"Produit"}}); err == nil {
		t.Fatal("création sur un serveur sans le contrat")
	}
	if links, _ := s.TableSources(ctx); len(links) != 0 {
		t.Fatalf("lien gardé après un refus : %+v", links)
	}
	fake.NoPushedSources = false

	// Clé hors des colonnes envoyées, ou mise à jour par clé sans clé.
	if _, err := s.CreateTableSource(ctx, TableSourceRequest{Server: dataServer, Table: table, Name: "x", Columns: []string{"Prix"}, Mode: workbook.PushUpsert, Keys: []string{"Produit"}}); err == nil {
		t.Fatal("clé non envoyée acceptée")
	}
	if _, err := s.CreateTableSource(ctx, TableSourceRequest{Server: dataServer, Table: table, Name: "x", Columns: []string{"Prix"}, Mode: workbook.PushUpsert}); err == nil {
		t.Fatal("upsert sans clé accepté")
	}

	// Source en ajout à la suite.
	link, err := s.CreateTableSource(ctx, TableSourceRequest{Server: dataServer, Table: table, Name: "Ajouts", Columns: []string{"Produit", "Prix"}, Mode: workbook.PushAppend})
	mustShare(t, err)
	if link.LastError != "" || link.LastResult.Inserted != 2 {
		t.Fatalf("source : %+v", link)
	}

	// 409 : un envoi précédent pas fini ; le message est gardé.
	fake.PushBusy = true
	if _, err := s.PushTableSource(ctx, table); err == nil || !strings.Contains(err.Error(), "réessayez") {
		t.Fatalf("409 : %v", err)
	}
	if links, _ := s.TableSources(ctx); links[0].LastError == "" {
		t.Fatalf("échec non noté : %+v", links[0])
	}
	fake.PushBusy = false

	// 413 : trop gros.
	fake.MaxPush = 5
	if _, err := s.PushTableSource(ctx, table); err == nil || !strings.Contains(err.Error(), "trop gros") {
		t.Fatalf("413 : %v", err)
	}
	fake.MaxPush = 0

	// 202 : l'écriture dure, le bilan est relu par son exécution (?run=), « pending » d'abord.
	fake.PushSlow = true
	link, err = s.PushTableSource(ctx, table)
	mustShare(t, err)
	if link.LastError != "" || link.LastResult == nil || link.LastResult.Inserted != 2 || fake.Calls["pushed_run"] < 2 || fake.Calls["push_status"] != 0 {
		t.Fatalf("202 : %+v %v", link, fake.Calls)
	}
	// Serveur d'avant 0028 : pas d'identifiant d'exécution, suivi par l'état
	// de la source ; mode non modifiable.
	fake.Before28 = true
	link, err = s.PushTableSource(ctx, table)
	mustShare(t, err)
	if link.LastError != "" || link.LastResult.Inserted != 2 || fake.Calls["push_status"] == 0 {
		t.Fatalf("202 sans 0028 : %+v %v", link, fake.Calls)
	}
	if _, err := s.UpdateTableSource(ctx, TableSourceRequest{Table: table, Name: "Ajouts", Columns: []string{"Produit", "Prix"}, Mode: workbook.PushReplace}); err == nil || !strings.Contains(err.Error(), "ne permet pas encore") {
		t.Fatalf("mode sans 0028 : %v", err)
	}
	fake.Before28, fake.PushSlow = false, false
	// Ajout à la suite : premier envoi et deux envois en 202 (les refusés n'ont rien écrit).
	if _, rows := fake.SourceRows(link.Pipeline); len(rows) != 6 {
		t.Fatalf("ajouts : %d lignes", len(rows))
	}
}

func TestPushSummaryRejects(t *testing.T) {
	r := &workbook.PushRun{Status: "warning", RowsRead: 3, Inserted: 2, Rejected: 1, Rejects: []workbook.PushReject{{Row: 3, Column: "Prix", Value: "abc", Error: "nombre attendu"}}}
	got := PushSummary("T", r, 0)
	if got != "Source « T » mise à jour : 3 lignes lues (2 ajoutées, 1 refusée) ; ligne 3, Prix « abc » : nombre attendu" {
		t.Fatal(got)
	}
}

func TestSourceValue(t *testing.T) {
	for _, c := range []struct {
		typ  string
		in   any
		want any
		ok   bool
	}{
		{"decimal", 4.5, 4.5, true},
		{"integer", int64(3), int64(3), true},
		{"boolean", int64(1), true, true},
		{"text", "x", "x", true},
		{"decimal", formula.Num(2), 2.0, true},
		{"text", formula.Boolean(true), "VRAI", true},
		{"text", formula.DateSerial(45000), "2023-03-15", true},
		{"decimal", formula.Err(formula.ErrDiv0), nil, false},
		{"integer", "abc", nil, false},
		{"text", nil, nil, true},
	} {
		got, ok := sourceValue(c.typ, c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("sourceValue(%s, %v) = %v %v, attendu %v %v", c.typ, c.in, got, ok, c.want, c.ok)
		}
	}
	got, ok := sourceValue("date", "2024-02-29")
	if !ok || csvText(got) != "2024-02-29" {
		t.Errorf("date : %v %v", got, ok)
	}
	for v, want := range map[any]string{nil: "", true: "true", int64(12): "12", 0.1: "0.1", 1e21: "1000000000000000000000"} {
		if got := csvText(v); got != want {
			t.Errorf("csvText(%v) = %q, attendu %q", v, got, want)
		}
	}
}
