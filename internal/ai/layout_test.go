package ai

import (
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/api"
)

// Sans partage des données, sheets_get est permis mais ne rend que la
// disposition : positions et nature des cellules, pas leur contenu.
func TestLayoutOnly(t *testing.T) {
	if commands(SchemaOnly)["sheets_get"] == nil {
		t.Fatal("sheets_get absent en schema-only")
	}
	v := layoutOnly(api.SheetView{
		Cells:  []api.CellView{{Row: 1, Col: 1, Raw: "Salaire de Léa"}, {Row: 2, Col: 1, Raw: "=SOMME(A1:A3)", Value: "42"}, {Row: 3, Col: 1, Raw: "1 200,5"}},
		Pivots: []api.ObjectView{{Name: "TCD", Row: 5, Col: 1, Config: map[string]any{"filters": []string{"Léa"}}}},
	}).(api.SheetView)
	got := []string{v.Cells[0].Raw, v.Cells[1].Raw, v.Cells[2].Raw}
	if strings.Join(got, "|") != "(texte)|(formule)|(nombre)" || v.Cells[1].Value != "" || v.Pivots[0].Config != nil || v.Pivots[0].Row != 5 {
		t.Fatalf("disposition : %+v", v)
	}
}
