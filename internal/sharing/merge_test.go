package sharing

import (
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Sans partage des données, le compte rendu d'un regroupement d'onglets ne
// donne ni la valeur de clé en double ni le texte des formules.
func TestMaskMerge(t *testing.T) {
	r := canvas.MergeResult{
		KeyNote: "Pas de clé : F-001 est dans Janvier et dans Février",
		Notes:   []string{"Pas de clé : F-001 est dans Janvier et dans Février"},
		Notice:  "2 onglets regroupés dans « Ventes » (4 lignes), anciens onglets masqués ; Pas de clé : F-001 est dans Janvier et dans Février ; une formule à reprendre",
		Pending: []canvas.PendingRef{{Kind: "cell", Where: "Synthèse A1", Formula: "=SOMME(Janvier[Montant])"}},
	}
	got := Filter("tables.merge_sheets", r, SchemaOnly).(canvas.MergeResult)
	all := got.KeyNote + got.Notice + strings.Join(got.Notes, " ") + got.Pending[0].Formula
	if strings.Contains(all, "F-001") || strings.Contains(all, "SOMME") || !strings.HasSuffix(got.Notice, "une formule à reprendre") || got.Pending[0].Where != "Synthèse A1" {
		t.Fatalf("masqué : %+v", got)
	}
	if full := Filter("tables.merge_sheets", r, Full).(canvas.MergeResult); full.KeyNote != r.KeyNote {
		t.Fatal("partage complet masqué")
	}
}
