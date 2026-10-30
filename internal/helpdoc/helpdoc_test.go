package helpdoc

import (
	"testing"

	"github.com/lalamefine/lattis/internal/msg"
)

func TestSearch(t *testing.T) {
	if len(Pages()) < 5 {
		t.Fatalf("pages : %d", len(Pages()))
	}
	hits := Search("groupes d'en-tetes", 3)
	if len(hits) == 0 || hits[0].Page != "tableaux" {
		t.Fatalf("recherche : %+v", hits)
	}
	if _, ok := Get("sql"); !ok {
		t.Fatal("page sql absente")
	}
	if Search("", 3) != nil {
		t.Fatal("recherche vide")
	}
}

// TestSearchIn : chaque langue cherche dans ses pages, en français à défaut.
func TestSearchIn(t *testing.T) {
	for _, l := range msg.Langs {
		if n := len(PagesIn(l)); n != len(Pages()) {
			t.Errorf("%s : %d pages, %d en français", l, n, len(Pages()))
		}
	}
	if p, ok := GetIn(msg.EN, "prise-en-main"); !ok || p.Title != "Getting started" {
		t.Errorf("page anglaise : %q", p.Title)
	}
	if hits := SearchIn(msg.EN, "custom sort", 3); len(hits) == 0 || hits[0].Page != "tableaux" {
		t.Errorf("recherche en anglais : %+v", hits)
	}
	if hits := SearchIn(msg.ZH, "编写查询", 3); len(hits) == 0 || hits[0].Page != "sql" {
		t.Errorf("recherche en chinois : %+v", hits)
	}
	// Mots français dans une session anglaise : l'aide française.
	if hits := SearchIn(msg.EN, "groupes d'en-tetes", 3); len(hits) == 0 || hits[0].Page != "tableaux" {
		t.Errorf("repli sur le français : %+v", hits)
	}
	if p, ok := GetIn("xx", "sql"); !ok || p.Title != "Requêtes SQL" {
		t.Errorf("langue inconnue : %q", p.Title)
	}
}
