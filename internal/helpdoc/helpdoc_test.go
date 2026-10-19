package helpdoc

import "testing"

func TestSearch(t *testing.T) {
	if len(Pages()) < 5 {
		t.Fatalf("pages : %d", len(Pages()))
	}
	hits := Search("groupes d'en-tetes", 3)
	if len(hits) == 0 || hits[0].Page != "prise-en-main" {
		t.Fatalf("recherche : %+v", hits)
	}
	if _, ok := Get("sql"); !ok {
		t.Fatal("page sql absente")
	}
	if Search("", 3) != nil {
		t.Fatal("recherche vide")
	}
}
