package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/grid"
)

// Tableaux de la grille gardés en cache par le pont (DocumentService.tables)
// après une écriture qui change ce qu'ils lisent : nombre de lignes, ordre,
// filtre, choix de l'onglet, colonnes. Une grille gardée à tort lit des
// lignes en trop ou en moins, ou refuse les blocs suivants avec
// « grid: hors du tableau » (barre d'état : « Lecture du tableau
// « BDD_nids » impossible »).

// checkGrid lit, comme la grille, chaque tableau de chaque feuille par
// blocs de 200 lignes et compare à une lecture neuve du même tableau :
// mêmes lignes, dans le même ordre, autant que la mise en page en annonce.
func checkGrid(t *testing.T, s *DocumentService, ctx context.Context, when string) {
	t.Helper()
	d, err := s.current()
	if err != nil {
		t.Fatal(err)
	}
	l, err := s.Layout(ctx, 1)
	if err != nil {
		t.Fatalf("%s : %v", when, err)
	}
	for _, sh := range l.Sheets {
		ls, err := s.Layout(ctx, sh.ID)
		if err != nil {
			t.Errorf("%s, feuille %s : %v", when, sh.Name, err)
			continue
		}
		for _, tb := range ls.Tables {
			var got []int64
			var cells []string
			for first := 0; first < tb.Rows; first += 200 {
				w, err := s.Window(ctx, sh.ID, tb.ID, first, 200)
				if err != nil {
					t.Errorf("%s, %s.%s lignes %d+ (%d affichées sur %d) : %v", when, sh.Name, tb.Name, first, tb.Rows, tb.Total, err)
					break
				}
				got = append(got, w.IDs...)
				for _, r := range w.Cells {
					cells = append(cells, fmt.Sprint(r))
				}
			}
			fresh, err := api.LoadGrid(ctx, d, tb)
			if err != nil {
				t.Fatal(err)
			}
			var want []int64
			var wcells []string
			for first := 0; first < tb.Rows; first += grid.MaxWindow {
				w, err := fresh.Window(ctx, first, grid.MaxWindow)
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, w.IDs...)
				for _, r := range w.Cells {
					wcells = append(wcells, fmt.Sprint(r))
				}
			}
			if len(want) != tb.Rows {
				t.Errorf("%s, %s.%s : mise en page %d lignes, lecture neuve %d", when, sh.Name, tb.Name, tb.Rows, len(want))
			}
			if !slices.Equal(got, want) || !slices.Equal(cells, wcells) {
				t.Errorf("%s, %s.%s : la grille lit %d lignes (%v…), une lecture neuve %d (%v…)", when, sh.Name, tb.Name, len(got), head(cells), len(want), head(wcells))
			}
		}
	}
}

func head(s []string) []string { return s[:min(len(s), 3)] }

// nids prépare le classeur du bug : « Base de données des nids » (tableau
// BDD_nids, clé ID numérotée d'office, 450 lignes sur deux années) et
// « Feuille12 ». Le tableau est lu une première fois (mis en cache).
func nids(t *testing.T) (s *DocumentService, ctx context.Context, base, f12 int64, bdd canvas.TableInfo) {
	t.Helper()
	ctx = context.Background()
	s = &DocumentService{}
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	base = 1
	if _, err := s.RenameSheet(ctx, base, "Base de données des nids"); err != nil {
		t.Fatal(err)
	}
	block := [][]string{{"ID", "Année", "Nid"}}
	for r := range 450 {
		block = append(block, []string{fmt.Sprint(r + 1), fmt.Sprint(2024 + r%2), fmt.Sprintf("N%03d", r)})
	}
	if _, err := s.Paste(ctx, base, 0, 0, block); err != nil {
		t.Fatal(err)
	}
	l, _ := s.Layout(ctx, base)
	if _, err := s.RenameTable(ctx, l.Tables[0].Name, "BDD_nids"); err != nil {
		t.Fatal(err)
	}
	var err error
	if f12, err = s.AddSheet(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RenameSheet(ctx, f12, "Feuille12"); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "départ")
	return s, ctx, base, f12, tableOf(t, s, ctx, base, "BDD_nids")
}

func tableOf(t *testing.T, s *DocumentService, ctx context.Context, sheet int64, name string) canvas.TableInfo {
	t.Helper()
	l, err := s.Layout(ctx, sheet)
	if err != nil {
		t.Fatal(err)
	}
	for _, tb := range l.Tables {
		if tb.Name == name {
			return tb
		}
	}
	t.Fatalf("tableau %s absent de la feuille %d", name, sheet)
	return canvas.TableInfo{}
}

// TestGridChoiceFromPivotSource : « Filtrer l'onglet par ce champ » sur un
// TCD de Feuille12 dont la source est BDD_nids (autre feuille) ; la grille,
// restée sur Feuille12, demande encore des lignes de BDD_nids (blocs en
// route au changement de feuille) avec l'identifiant de Feuille12.
func TestGridChoiceFromPivotSource(t *testing.T) {
	s, ctx, base, f12, bdd := nids(t)
	if _, err := s.PlacePivot(ctx, f12, 0, 0, canvas.PivotConfig{Source: "BDD_nids", RowFields: []string{"Année"}, Values: []canvas.PivotValue{{Field: "Nid", Agg: "count"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChooseSheetBy(ctx, f12, "BDD_nids", "Année", ""); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "Feuille12 filtrée par Année")
	if _, err := s.ChooseSheetValue(ctx, f12, "2025", false); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "Feuille12 sur 2025")
	// Lignes ajoutées à la source, lues depuis Feuille12.
	if _, err := s.SetCell(ctx, base, bdd.Row+1+bdd.Rows, 2, "N999"); err != nil {
		t.Fatal(err)
	}
	for first := 0; first < bdd.Rows+1; first += 200 {
		if _, err := s.Window(ctx, f12, bdd.ID, first, 200); err != nil {
			t.Fatalf("BDD_nids lu depuis Feuille12, lignes %d+ : %v", first, err)
		}
	}
	checkGrid(t, s, ctx, "ligne ajoutée à la source")
}

// TestGridOtherSheetTable : un bloc d'un tableau d'une autre feuille demandé
// avec la feuille affichée (changement d'onglet pendant la lecture), après
// une écriture qui vide le cache.
func TestGridOtherSheetTable(t *testing.T) {
	s, ctx, _, f12, bdd := nids(t)
	if _, err := s.SetZoom(ctx, f12, 120); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChooseSheetBy(ctx, 1, "BDD_nids", "Année", "2024"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Window(ctx, f12, bdd.ID, 200, 200); err != nil {
		t.Fatalf("BDD_nids lu avec la feuille Feuille12 : %v", err)
	}
}

// TestGridSupprDropsRows : Suppr sur des lignes entières d'un tableau qui
// suit l'onglet à choix les supprime (transfer.go, dropEmptiedRows).
func TestGridSupprDropsRows(t *testing.T) {
	s, ctx, base, _, _ := nids(t)
	if _, err := s.ChooseSheetBy(ctx, base, "BDD_nids", "Année", "2025"); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "onglet sur 2025")
	bdd := tableOf(t, s, ctx, base, "BDD_nids")
	if _, err := s.ClearRange(ctx, base, bdd.Row+1, 1, bdd.Row+30, 2, false); err != nil {
		t.Fatal(err)
	}
	if got := tableOf(t, s, ctx, base, "BDD_nids").Rows; got != bdd.Rows-30 {
		t.Fatalf("Suppr : %d lignes, attendu %d", got, bdd.Rows-30)
	}
	checkGrid(t, s, ctx, "après Suppr")
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "Suppr annulé")
}

// TestGridAutoIncrement : une ligne tapée sous un tableau filtré par
// l'onglet reçoit le numéro suivant (autokey.go) et la valeur choisie.
func TestGridAutoIncrement(t *testing.T) {
	s, ctx, base, _, _ := nids(t)
	if _, err := s.ChooseSheetBy(ctx, base, "BDD_nids", "Année", "2024"); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "onglet sur 2024")
	for k := range 30 {
		bdd := tableOf(t, s, ctx, base, "BDD_nids")
		if _, err := s.SetCell(ctx, base, bdd.Row+1+bdd.Rows, 2, fmt.Sprintf("Nouveau %d", k)); err != nil {
			t.Fatal(err)
		}
		checkGrid(t, s, ctx, fmt.Sprintf("ligne ajoutée %d", k))
	}
	if got := tableOf(t, s, ctx, base, "BDD_nids").Rows; got != 255 {
		t.Fatalf("lignes de 2024 : %d", got)
	}
	if _, err := s.InsertRow(ctx, "BDD_nids", 3); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "ligne insérée")
}

// TestGridFollowToggle : « Suivre l'année » décoché puis recoché (lot 4)
// sur un second tableau de la feuille qui a la colonne Année.
func TestGridFollowToggle(t *testing.T) {
	s, ctx, base, _, _ := nids(t)
	block := [][]string{{"Année", "Visite"}}
	for r := range 300 {
		block = append(block, []string{fmt.Sprint(2024 + r%2), fmt.Sprintf("V%d", r)})
	}
	if _, err := s.Paste(ctx, base, 0, 6, block); err != nil {
		t.Fatal(err)
	}
	l, _ := s.Layout(ctx, base)
	if _, err := s.RenameTable(ctx, l.Tables[1].Name, "Visites"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ChooseSheetBy(ctx, base, "BDD_nids", "Année", "2025"); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "onglet sur 2025")
	if _, err := s.SetTableFollow(ctx, "Visites", false); err != nil {
		t.Fatal(err)
	}
	if got := tableOf(t, s, ctx, base, "Visites").Rows; got != 300 {
		t.Fatalf("ne suit plus : %d lignes", got)
	}
	checkGrid(t, s, ctx, "ne suit plus")
	if _, err := s.SetTableFollow(ctx, "Visites", true); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "suit de nouveau")
	if _, err := s.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "annulé")
}

// TestGridCustomSort : tri personnalisé (table d'ordre temporaire) d'un
// tableau filtré par l'onglet.
func TestGridCustomSort(t *testing.T) {
	s, ctx, base, _, _ := nids(t)
	if _, err := s.ChooseSheetBy(ctx, base, "BDD_nids", "Année", "2025"); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "onglet sur 2025")
	if _, err := s.SortTableCustom(ctx, "BDD_nids", 2, canvas.CustomOrder{Values: []string{"N449", "N101", "N003"}}); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "tri personnalisé")
	if _, err := s.SortTable(ctx, "BDD_nids", 0, true); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "tri décroissant")
	if _, err := s.ChooseSheetValue(ctx, base, "", true); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "(Toutes)")
}

// TestGridWriteElsewhere : écritures faites sans passer par une méthode du
// pont qui prévient la grille (script, serveur MCP, relevé d'un serveur) :
// la grille relit quand même ce qui a changé.
func TestGridWriteElsewhere(t *testing.T) {
	s, ctx, base, _, _ := nids(t)
	d, _ := s.current()
	if err := d.SetSheetChoice(ctx, base, "BDD_nids", "Année", "2024"); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "onglet sur 2024")
	bdd := tableOf(t, s, ctx, base, "BDD_nids")
	if _, err := d.Paste(ctx, base, bdd.Row+1+bdd.Rows, 2, [][]string{{"P1"}, {"P2"}}); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "collé sous le tableau")
	if err := d.SortTable(ctx, "BDD_nids", 2, true); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "trié")
	if err := d.ClearRange(ctx, base, bdd.Row+1, 1, bdd.Row+5, 2, false); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "Suppr")
	if err := d.ChooseSheetValue(ctx, base, "2025", false); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "onglet sur 2025")
	if _, err := d.Bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "annulé")
	// Au-delà de la fin : relu une fois, puis refusé (détail dans le journal).
	logPath := filepath.Join(t.TempDir(), "erreurs.log")
	old := errorLogFile
	errorLogFile = func() (string, error) { return logPath, nil }
	defer func() { errorLogFile = old }()
	bdd = tableOf(t, s, ctx, base, "BDD_nids")
	if _, err := s.Window(ctx, base, bdd.ID, bdd.Rows, 200); !errors.Is(err, grid.ErrRange) {
		t.Fatalf("au-delà de la fin : %v", err)
	}
	if b, err := os.ReadFile(logPath); err != nil || !strings.Contains(string(b), "BDD_nids") {
		t.Fatalf("journal : %q %v", b, err)
	}
}

// TestGridEditFilteredValue : une saisie qui change la valeur suivie (la
// ligne quitte l'onglet) n'est qu'une mise à jour de valeurs du tableau,
// mais change les lignes affichées.
func TestGridEditFilteredValue(t *testing.T) {
	s, ctx, base, _, _ := nids(t)
	if _, err := s.ChooseSheetBy(ctx, base, "BDD_nids", "Année", "2025"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetFilter(ctx, "BDD_nids", 2, &canvas.ColumnFilter{Conds: []canvas.FilterCond{{Op: "begins", Value: "N"}}}); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "filtré")
	bdd := tableOf(t, s, ctx, base, "BDD_nids")
	if _, err := s.SetCell(ctx, base, bdd.Row+1, 2, "X"); err != nil {
		t.Fatal(err)
	}
	checkGrid(t, s, ctx, "ligne sortie du filtre")
}
