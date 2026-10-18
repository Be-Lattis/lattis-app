package workbook_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Colonnes système de la migration 0008 (lattis_updated_device,
// lattis_other_xid) : lues dans les deltas (to_jsonb), jamais prises pour
// des colonnes de l'utilisateur.
func TestChangeIgnoresSystemColumns(t *testing.T) {
	raw := `{"lattis_id": "0190a1b2-0000-7000-8000-000000000001", "lattis_pos": "a0", "lattis_xid": "1234",
		"lattis_author": "alice", "lattis_updated_by": "bob", "lattis_updated_at": "2026-10-18T10:00:00+00:00",
		"lattis_updated_device": "poste-1", "lattis_other_xid": "1200", "Produit": "Café", "Quantité": 3}`
	var row map[string]any
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&row); err != nil {
		t.Fatal(err)
	}
	rt := workbook.RemoteTable{Name: "Ventes", Columns: []workbook.RemoteColumn{{Name: "Produit", Type: "text"}, {Name: "Quantité", Type: "integer"}}}
	c := workbook.ChangeFromJSON(row, rt)
	if c.ID == "" || c.Author != "alice" || c.UpdatedBy != "bob" || c.Pos != "a0" {
		t.Fatalf("colonnes système : %+v", c)
	}
	if len(c.Values) != 2 || c.Values["Produit"] != "Café" {
		t.Fatalf("valeurs : %v", c.Values)
	}
}
