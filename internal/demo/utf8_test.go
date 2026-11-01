package demo

import (
	"testing"
	"unicode/utf8"
)

// Le thème Livres met une majuscule à des mots qui commencent par un accent :
// le texte reste de l'UTF-8 valide (la publication refusait « 0xa9 »).
func TestLivresValidUTF8(t *testing.T) {
	cols := Livres.Columns(len(Livres.Base))
	err := Rows(cols, 2000, 1, "livres", func(row []any) error {
		for i, v := range row {
			if s, ok := v.(string); ok && !utf8.ValidString(s) {
				t.Fatalf("colonne %s : %q", cols[i].Name, s)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
