package canvas

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/store"
)

// handPivot pose une source (NID, Ville) en feuille 1 et un TCD « NID en
// lignes, nombre de lignes » sur une nouvelle feuille (en-tête en B2).
func handPivot(t *testing.T, d *fixture) (sheet, id int64, base PivotConfig) {
	t.Helper()
	ctx := context.Background()
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"NID", "Ville"},
		{"A", "Paris"},
		{"B", "Lyon"},
		{"A", "Nice"},
		{"C", "Lyon"},
	}); err != nil {
		t.Fatal(err)
	}
	base = PivotConfig{Source: "Feuille1", RowFields: []string{"NID"}, Values: []PivotValue{{Agg: "count"}}}
	sheet, err := d.CreatePivot(ctx, base)
	must(t, err)
	l, err := d.Layout(ctx, sheet)
	must(t, err)
	return sheet, l.Pivots[0].ID, base
}

func onePivot(t *testing.T, d *fixture, sheet int64) PivotInfo {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	must(t, err)
	if len(l.Pivots) != 1 || l.Pivots[0].Error != "" {
		t.Fatalf("tableau croisé : %+v", l.Pivots)
	}
	return l.Pivots[0]
}

// cellsText écrit les cellules d'un TCD ligne par ligne : « a b | c d ».
func cellsText(p PivotInfo) string {
	var out []string
	for _, r := range p.Cells {
		out = append(out, strings.Join(r, " "))
	}
	return strings.Join(out, " | ")
}

func freeValue(t *testing.T, d *fixture, sheet int64, row, col int) string {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	must(t, err)
	for _, c := range l.Cells {
		if c.Row == row && c.Col == col {
			if c.Value != "" {
				return c.Value
			}
			return c.Raw
		}
	}
	return ""
}

// Décision 0008, partie TCD : un nom tapé à droite de l'en-tête d'un TCD
// ajoute une colonne saisie ; ses valeurs suivent les étiquettes de lignes
// (tri, filtre, source changée), sont mises de côté quand l'étiquette
// disparaît et reviennent avec elle ; une formule vaut pour toute la colonne.
func TestPivotHandColumns(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	sheet, id, base := handPivot(t, d)
	p := onePivot(t, d, sheet)
	if p.Row != 1 || p.Col != 1 || len(p.Cells[0]) != 2 {
		t.Fatalf("TCD : %+v", p)
	}
	count := p.Cells[0][1] // « Nombre »
	// Nom à droite de l'en-tête (D2) : une colonne saisie.
	mustSet(t, d, sheet, 1, 3, "test")
	p = onePivot(t, d, sheet)
	if len(p.Hand) != 1 || p.Hand[0].Name != "test" || len(p.Formats) > 0 && len(p.Formats) != 3 {
		t.Fatalf("colonne saisie : %+v", p)
	}
	want := "NID " + count + " test | A 2  | B 1  | C 1  | Total 4 "
	if got := cellsText(p); got != want {
		t.Fatalf("cellules : %q, voulu %q", got, want)
	}
	check(t, d, "création")
	// Valeurs par ligne ; pas sur le total.
	mustSet(t, d, sheet, 2, 3, "12,5")
	mustSet(t, d, sheet, 3, 3, "lyon!")
	if _, err := d.SetCell(ctx, sheet, 5, 3, "x"); err == nil {
		t.Fatal("saisie sur la ligne de total acceptée")
	}
	if got := cellsText(onePivot(t, d, sheet)); got != "NID "+count+" test | A 2 12,5 | B 1 lyon! | C 1  | Total 4 " {
		t.Fatalf("valeurs : %q", got)
	}
	// Annuler la dernière saisie.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := onePivot(t, d, sheet).Cells[2][2]; got != "" {
		t.Fatalf("annulation : %q", got)
	}
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	// Colonne à formule : la valeur de la ligne ([Nombre], [test]), totaux compris.
	mustSet(t, d, sheet, 1, 4, "double")
	mustSet(t, d, sheet, 2, 4, "=["+count+"]*2")
	p = onePivot(t, d, sheet)
	if got := cellsText(p); got != "NID "+count+" test double | A 2 12,5 4 | B 1 lyon! 2 | C 1  2 | Total 4  8" {
		t.Fatalf("formule : %q", got)
	}
	if p.Hand[1].Formula != "=["+count+"]*2" {
		t.Fatalf("formule affichée : %+v", p.Hand)
	}
	if _, err := d.SetCell(ctx, sheet, 3, 4, "3"); err == nil {
		t.Fatal("valeur dans une colonne à formule acceptée")
	}
	mustSet(t, d, sheet, 4, 4, "=[test]*2+SOMME(TCD1["+count+"])")
	if got := onePivot(t, d, sheet).Cells[1][3]; got != "29" {
		t.Fatalf("formule sur une colonne saisie et la colonne du TCD : %q", got)
	}
	mustSet(t, d, sheet, 4, 4, "=["+count+"]*2")
	check(t, d, "formule")
	// Une cellule libre lit les colonnes saisies par leur adresse.
	mustSet(t, d, sheet, 10, 0, "=D3*2")
	mustSet(t, d, sheet, 11, 0, "=E6+1")
	if got := freeValue(t, d, sheet, 10, 0); got != "25" {
		t.Fatalf("=D3*2 : %q", got)
	}
	if got := freeValue(t, d, sheet, 11, 0); got != "9" {
		t.Fatalf("=E6+1 : %q", got)
	}
	// Une formule de colonne qui lit une cellule libre.
	mustSet(t, d, sheet, 12, 0, "100")
	mustSet(t, d, sheet, 2, 4, "=["+count+"]+A13")
	if got := onePivot(t, d, sheet).Cells[1][3]; got != "102" {
		t.Fatalf("formule avec adresse : %q", got)
	}
	mustSet(t, d, sheet, 2, 4, "=["+count+"]*2")
	// Tri : les valeurs suivent leurs étiquettes.
	sorted := base
	sorted.Sort = map[string]string{"NID": "desc"}
	must(t, d.SetPivot(ctx, id, sorted))
	if got := cellsText(onePivot(t, d, sheet)); got != "NID "+count+" test double | C 1  2 | B 1 lyon! 2 | A 2 12,5 4 | Total 4  8" {
		t.Fatalf("tri : %q", got)
	}
	// Filtre : B masqué, sa valeur mise de côté ; elle revient avec lui.
	filtered := base
	filtered.Items = []PivotFilter{{Field: "NID", Values: []string{"A", "C"}}}
	must(t, d.SetPivot(ctx, id, filtered))
	p = onePivot(t, d, sheet)
	if got := cellsText(p); got != "NID "+count+" test double | A 2 12,5 4 | C 1  2 | Total 3  6" || p.HandKept != 1 {
		t.Fatalf("filtre : %q (%d de côté)", got, p.HandKept)
	}
	must(t, d.SetPivot(ctx, id, base))
	if p = onePivot(t, d, sheet); p.Cells[2][2] != "lyon!" || p.HandKept != 0 {
		t.Fatalf("filtre retiré : %q", cellsText(p))
	}
	// Source changée : B disparaît (mis de côté), oublié, rendu par Ctrl+Z.
	mustSet(t, d, 1, 2, 0, "D")
	p = onePivot(t, d, sheet)
	if got := cellsText(p); got != "NID "+count+" test double | A 2 12,5 4 | D 1  2 | C 1  2 | Total 4  8" || p.HandKept != 1 {
		t.Fatalf("source changée : %q (%d de côté)", got, p.HandKept)
	}
	n, err := d.ForgetHandKept(ctx, "TCD1")
	if err != nil || n != 1 {
		t.Fatalf("oubli : %d, %v", n, err)
	}
	if p = onePivot(t, d, sheet); p.HandKept != 0 {
		t.Fatalf("oubli : %d de côté", p.HandKept)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if p = onePivot(t, d, sheet); p.HandKept != 1 {
		t.Fatalf("oubli annulé : %d de côté", p.HandKept)
	}
	mustSet(t, d, 1, 2, 0, "B")
	if p = onePivot(t, d, sheet); p.Cells[2][2] != "lyon!" || p.HandKept != 0 {
		t.Fatalf("ligne revenue : %q", cellsText(p))
	}
	// Vue SQL du TCD : les colonnes saisies à droite.
	q, err := d.PivotCTE(ctx, `SELECT * FROM "TCD1"`)
	must(t, err)
	var rows []string
	must(t, d.read(ctx, func(tx *tx) error {
		rs, err := tx.query(q)
		if err != nil {
			return fmt.Errorf("%s : %w", q, err)
		}
		defer rs.Close()
		cols, _ := rs.Columns()
		rows = append(rows, strings.Join(cols, "|"))
		for rs.Next() {
			v := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range v {
				ptrs[i] = &v[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				return err
			}
			rows = append(rows, strings.TrimSpace(fmt.Sprintln(v...)))
		}
		return rs.Err()
	}))
	if got := strings.Join(rows, ", "); got != "NID|"+count+"|test|double, A 2 12.5 4, B 1 lyon! 2, C 1 <nil> 2" {
		t.Fatalf("vue SQL : %s", got)
	}
	// Le TCD s'élargit (une valeur de plus) : les colonnes saisies suivent.
	wide := base
	wide.Values = append(wide.Values, PivotValue{Field: "Ville", Agg: "countDistinct"})
	must(t, d.SetPivot(ctx, id, wide))
	p = onePivot(t, d, sheet)
	if h := p.Cells[0]; len(h) != 5 || h[3] != "test" || h[4] != "double" || p.Cells[1][3] != "12,5" {
		t.Fatalf("TCD élargi : %q", cellsText(p))
	}
	if got := freeValue(t, d, sheet, 10, 0); got != "#VALEUR!" && got != "4" {
		// D3 est désormais la nouvelle colonne du TCD (2 villes pour A) :
		// une adresse désigne une cellule, pas une colonne.
		t.Logf("=D3*2 après élargissement : %q", got)
	}
	// Nouvelle colonne : à droite des colonnes saisies (G2).
	mustSet(t, d, sheet, 1, 6, "troisième")
	if p = onePivot(t, d, sheet); len(p.Hand) != 3 || p.Cells[0][5] != "troisième" {
		t.Fatalf("troisième colonne : %q", cellsText(p))
	}
	must(t, d.SetPivot(ctx, id, base))
	// Renommer ; un nom pris est refusé.
	mustSet(t, d, sheet, 1, 3, "note")
	if p = onePivot(t, d, sheet); p.Hand[0].Name != "note" || p.Cells[1][2] != "12,5" {
		t.Fatalf("renommage : %q", cellsText(p))
	}
	if _, err := d.SetCell(ctx, sheet, 1, 3, "double"); err == nil {
		t.Fatal("nom en double accepté")
	}
	// Plus de champ en lignes : tout est mis de côté, avec une note ; tout revient.
	none := base
	none.RowFields = nil
	must(t, d.SetPivot(ctx, id, none))
	if p = onePivot(t, d, sheet); p.HandNote == "" || p.HandKept != 2 {
		t.Fatalf("sans champ en lignes : %+v", p)
	}
	must(t, d.SetPivot(ctx, id, base))
	if p = onePivot(t, d, sheet); p.Cells[1][2] != "12,5" || p.Cells[2][2] != "lyon!" || p.HandNote != "" {
		t.Fatalf("champ revenu : %q", cellsText(p))
	}
	check(t, d, "champs")
	// Suppr : une valeur ; l'en-tête d'une colonne qui a des valeurs reste ;
	// toute la colonne la retire.
	must(t, d.ClearRange(ctx, sheet, 2, 2, 2, 3, false))
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	must(t, d.ClearRange(ctx, sheet, 2, 3, 2, 3, false))
	if p = onePivot(t, d, sheet); p.Cells[1][2] != "" || p.Cells[2][2] != "lyon!" {
		t.Fatalf("Suppr d'une valeur : %q", cellsText(p))
	}
	must(t, d.ClearRange(ctx, sheet, 1, 3, 1, 3, false))
	if p = onePivot(t, d, sheet); len(p.Hand) != 3 {
		t.Fatalf("en-tête avec des valeurs : %+v", p.Hand)
	}
	must(t, d.ClearRange(ctx, sheet, 1, 3, 5, 3, false))
	if p = onePivot(t, d, sheet); len(p.Hand) != 2 || p.Hand[0].Name != "double" {
		t.Fatalf("colonne retirée : %+v", p.Hand)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if p = onePivot(t, d, sheet); len(p.Hand) != 3 || p.Cells[2][2] != "lyon!" {
		t.Fatalf("retrait annulé : %q", cellsText(p))
	}
	// TCD supprimé : ses valeurs avec lui ; Ctrl+Z rend tout.
	must(t, d.DeletePivot(ctx, id))
	var left int
	must(t, d.read(ctx, func(tx *tx) error {
		return tx.row(`SELECT count(*) FROM _lattis_hand_kept WHERE owner = ?`, pivotOwner(id)).Scan(&left)
	}))
	if left != 0 {
		t.Fatalf("valeurs restées après suppression : %d", left)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if p = onePivot(t, d, sheet); p.Cells[2][2] != "lyon!" {
		t.Fatalf("suppression annulée : %q", cellsText(p))
	}
	check(t, d, "fin")
}

// Le nom déjà tapé à droite d'un TCD, avec des valeurs dessous (cellules
// libres d'avant) : la colonne les reprend sur les lignes de détail.
func TestPivotHandAdoptsFreeCells(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	sheet, _, _ := handPivot(t, d)
	// Tapées avant : sous l'en-tête à droite, une valeur sur la ligne de B.
	must(t, d.read(ctx, func(*tx) error { return nil }))
	mustSet(t, d, sheet, 3, 3, "vu")
	mustSet(t, d, sheet, 1, 3, "test")
	p := onePivot(t, d, sheet)
	if len(p.Hand) != 1 || p.Cells[2][2] != "vu" {
		t.Fatalf("reprise : %q", cellsText(p))
	}
	for _, c := range mustLayout(t, d, sheet).Cells {
		if c.Col == 3 {
			t.Fatalf("cellule libre restée : %+v", c)
		}
	}
}

func mustLayout(t *testing.T, d *fixture, sheet int64) Layout {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	must(t, err)
	return l
}

// Plusieurs champs en lignes : une valeur par ligne de détail (chemin des
// étiquettes) ; pas de saisie sur un groupe.
func TestPivotHandCompact(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	sheet, id, base := handPivot(t, d)
	two := base
	two.RowFields = []string{"NID", "Ville"}
	must(t, d.SetPivot(ctx, id, two))
	p := onePivot(t, d, sheet)
	// A, Paris, Nice, B, Lyon, C, Lyon, Total
	if len(p.Cells) != 9 || p.Cells[2][0] != "Paris" {
		t.Fatalf("arborescence : %q", cellsText(p))
	}
	mustSet(t, d, sheet, 1, 3, "test")
	mustSet(t, d, sheet, 3, 3, "x") // A › Paris
	if _, err := d.SetCell(ctx, sheet, 2, 3, "g"); err == nil {
		t.Fatal("saisie sur un groupe acceptée")
	}
	mustSet(t, d, sheet, 2, 3, "=[NID]&\"-\"&[Ville]")
	p = onePivot(t, d, sheet)
	if p.Cells[2][2] != "A-Paris" || p.Cells[1][2] != "A-" {
		t.Fatalf("formule en arborescence : %q", cellsText(p))
	}
	mustSet(t, d, sheet, 2, 3, "")
	// Les valeurs saisies reviennent quand la formule s'efface.
	if p = onePivot(t, d, sheet); p.Cells[2][2] != "x" || p.Hand[0].Formula != "" {
		t.Fatalf("formule effacée : %q", cellsText(p))
	}
	// Groupe plié : la valeur n'est pas mise de côté.
	folded := two
	folded.Collapsed = [][]string{{"A"}}
	must(t, d.SetPivot(ctx, id, folded))
	if p = onePivot(t, d, sheet); p.HandKept != 0 {
		t.Fatalf("groupe plié : %d de côté", p.HandKept)
	}
	// Retour à un seul champ : clé différente, valeur de côté ; elle revient.
	must(t, d.SetPivot(ctx, id, base))
	if p = onePivot(t, d, sheet); p.HandKept != 1 {
		t.Fatalf("un seul champ : %d de côté", p.HandKept)
	}
	must(t, d.SetPivot(ctx, id, two))
	if p = onePivot(t, d, sheet); p.Cells[2][2] != "x" {
		t.Fatalf("deux champs revenus : %q", cellsText(p))
	}
}

// Enregistrer, rouvrir : colonnes, valeurs et formules restent.
func TestPivotHandReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "doc.sqlite")
	openAt := func() (*fixture, func()) {
		db, err := store.Open(path, Register)
		must(t, err)
		must(t, meta.Migrate(ctx, db))
		bus := command.New(db)
		return &fixture{New(db, bus), bus}, func() { db.Close() }
	}
	d, done := openAt()
	sheet, _, _ := handPivot(t, d)
	mustSet(t, d, sheet, 1, 3, "test")
	mustSet(t, d, sheet, 2, 3, "oui")
	mustSet(t, d, sheet, 1, 4, "f")
	mustSet(t, d, sheet, 2, 4, "=[test]&\"!\"")
	want := cellsText(onePivot(t, d, sheet))
	done()
	d, done = openAt()
	defer done()
	if got := cellsText(onePivot(t, d, sheet)); got != want || !strings.Contains(got, "A 2 oui oui!") {
		t.Fatalf("rouvert : %q, voulu %q", got, want)
	}
}

// Les colonnes saisies font partie du TCD : une colonne ajoutée ou un TCD
// élargi écarte le tableau voisin ; déplacé, le TCD les emporte avec leurs
// valeurs ; copié, ses valeurs sont copiées.
func TestPivotHandFootprint(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	sheet, id, base := handPivot(t, d)
	// Un tableau à une colonne vide du TCD (B..C, tableau en E).
	if _, err := d.Paste(ctx, sheet, 1, 4, [][]string{{"X", "Y"}, {"1", "2"}}); err != nil {
		t.Fatal(err)
	}
	tableCol := func() int {
		t.Helper()
		for _, tb := range mustLayout(t, d, sheet).Tables {
			return tb.Col
		}
		t.Fatal("tableau disparu")
		return 0
	}
	if c := tableCol(); c != 4 {
		t.Fatalf("tableau en %d", c)
	}
	mustSet(t, d, sheet, 1, 3, "test")
	if c := tableCol(); c != 5 {
		t.Fatalf("après la colonne saisie, tableau en %d (voulu 5)", c)
	}
	mustSet(t, d, sheet, 2, 3, "oui")
	wide := base
	wide.Values = append(wide.Values, PivotValue{Field: "Ville", Agg: "countDistinct"})
	must(t, d.SetPivot(ctx, id, wide))
	if c := tableCol(); c != 6 {
		t.Fatalf("TCD élargi, tableau en %d (voulu 6)", c)
	}
	// Déplacé : les valeurs suivent.
	must(t, d.MovePivot(ctx, id, sheet, 20, 1))
	if p := onePivot(t, d, sheet); p.Row != 20 || p.Cells[1][3] != "oui" {
		t.Fatalf("déplacé : %+v", p)
	}
	// Copié : la copie a ses colonnes et ses valeurs.
	nid, err := d.CopyPivot(ctx, id, 1, 20, 0)
	must(t, err)
	found := false
	for _, p := range mustLayout(t, d, 1).Pivots {
		if p.ID == nid {
			found = true
			if len(p.Hand) != 1 || p.Cells[1][3] != "oui" {
				t.Fatalf("copie : %q", cellsText(p))
			}
		}
	}
	if !found {
		t.Fatal("copie introuvable")
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, d, "copie annulée")
}
