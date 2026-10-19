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

// sourceHost relève les comptes rendus des envois faits à l'enregistrement.
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

// sourceFixture : un classeur local avec le tableau Produit / Prix, et un
// serveur de test vu par alice.
func sourceFixture(t *testing.T) (*workbooktest.Fake, *DocumentService, *sourceHost, string) {
	t.Helper()
	workRoot(t)
	isolateRecent(t)
	ctx := context.Background()
	fake := workbooktest.New()
	old := shareServer
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		return fake.As("alice"), auth.User{PGRole: "alice"}, "Serveur de test", nil
	}
	t.Cleanup(func() { shareServer = old })
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

	// Le tableau seul, sans la colonne Note, clé Produit.
	link, err := s.CreateTableSource(ctx, TableSourceRequest{Server: dataServer, Table: table, Name: "Prix du jour",
		Columns: []string{"Produit", "Prix"}, Keys: []string{"Produit"}, OnSave: true, Inherit: true})
	mustShare(t, err)
	if link.LastError != "" || link.LastResult == nil || link.LastResult.Inserted != 2 || link.SQLName != "prix_du_jour" {
		t.Fatalf("source créée : %+v", link)
	}
	cols, rows := fake.SourceRows(link.Pipeline)
	if !slices.Equal(cols, []string{"Produit", "Prix"}) || len(rows) != 2 || rows[0][0] != "Café" || rows[0][1] != 4.5 {
		t.Fatalf("copie du serveur : %v %v", cols, rows)
	}
	// Le classeur n'est pas publié : aucun classeur sur le serveur, pas de partage.
	if list, _ := fake.As("alice").Workbooks(ctx); len(list) != 0 {
		t.Fatalf("classeur publié par erreur : %+v", list)
	}
	if st := s.ShareState(); st.Shared {
		t.Fatalf("partage : %+v", st)
	}
	cat, err := s.DataCatalog(ctx, dataServer)
	mustShare(t, err)
	if len(cat.Datasets) != 1 || cat.Datasets[0].Source == nil || cat.Datasets[0].Source.Kind != "pipeline" || cat.Datasets[0].Source.PipelineID != link.Pipeline {
		t.Fatalf("catalogue : %+v", cat.Datasets)
	}
	if st, _ := s.State(ctx); !st.Dirty {
		t.Fatal("le lien est gardé dans le classeur : il doit être à enregistrer")
	}
	if info, _ := s.DataShareInfo(ctx, table); info.Source == nil || info.Source.Pipeline != link.Pipeline {
		t.Fatalf("source connue de la boîte : %+v", info)
	}

	// Mettre à jour : une ligne modifiée, une ajoutée, une supprimée.
	_, err = s.SetCell(ctx, 1, 2, 1, "3")
	mustShare(t, err)
	link, err = s.PushTableSource(ctx, table)
	mustShare(t, err)
	if r := link.LastResult; r.Updated != 1 || r.Inserted != 0 || r.Deleted != 0 {
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
	if len(links) != 1 || links[0].TableName != table || !links[0].OnSave || links[0].Pipeline != link.Pipeline {
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

	// Modifier : Prix n'est plus envoyé ; plus d'envoi à l'enregistrement.
	link, err = s.UpdateTableSource(ctx, TableSourceRequest{Table: table, Name: "Produits", Columns: []string{"Produit"}, Keys: []string{"Produit"}, Inherit: true})
	mustShare(t, err)
	if cols, _ := fake.SourceRows(link.Pipeline); !slices.Equal(cols, []string{"Produit"}) || link.OnSave {
		t.Fatalf("après modification : %v %+v", cols, link)
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

func TestTableSourceRefusals(t *testing.T) {
	fake, s, _, table := sourceFixture(t)
	ctx := context.Background()

	// Serveur sans le contrat : la voie est fermée, avec la raison.
	fake.NoTableSources = true
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
	fake.NoTableSources = false

	// Clé hors des colonnes envoyées.
	if _, err := s.CreateTableSource(ctx, TableSourceRequest{Server: dataServer, Table: table, Name: "x", Columns: []string{"Prix"}, Keys: []string{"Produit"}}); err == nil {
		t.Fatal("clé non envoyée acceptée")
	}

	// Clé en double : la source est créée, l'envoi refusé en entier.
	_, err = s.SetCell(ctx, 1, 2, 0, "Café")
	mustShare(t, err)
	link, err := s.CreateTableSource(ctx, TableSourceRequest{Server: dataServer, Table: table, Name: "Doublons", Columns: []string{"Produit", "Prix"}, Keys: []string{"Produit"}})
	mustShare(t, err)
	if !strings.Contains(link.LastError, "clé en double") || link.LastPush != "" {
		t.Fatalf("envoi refusé : %+v", link)
	}
	if _, rows := fake.SourceRows(link.Pipeline); len(rows) != 0 {
		t.Fatalf("lignes envoyées malgré le refus : %v", rows)
	}
	if _, err := s.PushTableSource(ctx, table); err == nil {
		t.Fatal("envoi d'une clé en double accepté")
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
	if got, ok := sourceValue("date", "2024-02-29"); !ok || got.(time.Time).Format("2006-01-02") != "2024-02-29" {
		t.Errorf("date : %v %v", got, ok)
	}
	if got := workbook.PushJSON(time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)); got != "2024-02-29" {
		t.Errorf("PushJSON : %v", got)
	}
}
