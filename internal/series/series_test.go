package series

import (
	"slices"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

func TestRecognize(t *testing.T) {
	cases := []struct {
		names  []string
		kind   Kind
		column string
		typ    grid.Type
		values []string
	}{
		{[]string{"Janvier", "Février", "Mars"}, Month, "Mois", grid.Text, []string{"Janvier", "Février", "Mars"}},
		{[]string{"janv.", "févr.", "mars", "avr."}, Month, "Mois", grid.Text, []string{"Janvier", "Février", "Mars", "Avril"}},
		{[]string{"Jan", "Feb", "Mar"}, Month, "Mois", grid.Text, []string{"Janvier", "Février", "Mars"}},
		{[]string{"01", "02", "03"}, Month, "Mois", grid.Text, []string{"Janvier", "Février", "Mars"}},
		{[]string{"Janv 2025", "Févr 2025", "Mars 2025"}, MonthYear, "Mois", grid.Date, []string{"2025-01-01", "2025-02-01", "2025-03-01"}},
		{[]string{"janv-25", "févr-25"}, MonthYear, "Mois", grid.Date, []string{"2025-01-01", "2025-02-01"}},
		{[]string{"2025-01", "2025-02", "2025-03"}, MonthYear, "Mois", grid.Date, []string{"2025-01-01", "2025-02-01", "2025-03-01"}},
		{[]string{"01/2025", "02/2025"}, MonthYear, "Mois", grid.Date, []string{"2025-01-01", "2025-02-01"}},
		{[]string{"2021", "2022", "2023", "2025"}, Year, "Année", grid.Integer, []string{"2021", "2022", "2023", "2025"}},
		{[]string{"Ventes 2024", "Ventes 2025"}, Year, "Année", grid.Integer, []string{"2024", "2025"}},
		{[]string{"T1 2025", "T2 2025", "T3 2025"}, Quarter, "Trimestre", grid.Text, []string{"2025-T1", "2025-T2", "2025-T3"}},
		{[]string{"Stock 2024-03-01", "Stock 2024-04-01"}, Date, "Date", grid.Date, []string{"2024-03-01", "2024-04-01"}},
		{[]string{"Lot 1", "Lot 2", "Lot 4"}, Number, "Lot", grid.Integer, []string{"1", "2", "4"}},
		{[]string{"Relevé-01", "Relevé-02", "Relevé-03"}, Number, "Relevé", grid.Integer, []string{"1", "2", "3"}},
		{[]string{"1", "2", "3"}, Number, "Numéro", grid.Integer, []string{"1", "2", "3"}},
		{[]string{"Annexe A", "Annexe B", "Annexe C"}, Letter, "Annexe", grid.Text, []string{"A", "B", "C"}},
		{[]string{"A", "B", "C"}, Letter, "Lettre", grid.Text, []string{"A", "B", "C"}},
		{[]string{"Feuille1", "Feuille2", "Feuille3"}, Default, "Numéro", grid.Integer, []string{"1", "2", "3"}},
		{[]string{"Agence Nord", "Agence Sud"}, Shape, "Agence", grid.Text, []string{"Nord", "Sud"}},
		{[]string{"Paris", "Lyon", "Marseille"}, City, "Ville", grid.Text, []string{"Paris", "Lyon", "Marseille"}},
		{[]string{"Clients", "Produits", "Notes"}, Other, "Catégorie", grid.Text, []string{"Clients", "Produits", "Notes"}},
		// Chiffres dans le désordre : pas une suite.
		{[]string{"Lot 3", "Lot 1", "Lot 2"}, Shape, "Lot", grid.Text, []string{"3", "1", "2"}},
	}
	for _, c := range cases {
		s := Recognize(c.names)
		if s.Kind != c.kind || s.Column != c.column || s.Type != c.typ || !slices.Equal(s.Values, c.values) {
			t.Errorf("%v : %s %q %s %v, attendu %s %q %s %v", c.names, s.Kind, s.Column, s.Type, s.Values, c.kind, c.column, c.typ, c.values)
		}
	}
}

func TestScores(t *testing.T) {
	if s := Recognize([]string{"2023", "2024"}); s.Score != Strong {
		t.Errorf("années : %d", s.Score)
	}
	if s := Recognize([]string{"Feuille1", "Feuille2"}); s.Score != Weak {
		t.Errorf("noms par défaut : %d", s.Score)
	}
	if s := Recognize([]string{"Agence Nord", "Agence Sud"}); s.Score != Medium {
		t.Errorf("même forme : %d", s.Score)
	}
	if s := Recognize([]string{"Clients", "Produits"}); s.Score != 0 {
		t.Errorf("rien : %d", s.Score)
	}
	if s := Recognize([]string{"Janvier 2025", "Mars 2025", "Février 2025"}); Latest(s) != 1 {
		t.Errorf("le plus récent : %d", Latest(s))
	}
}

// Les noms d'onglets se reconnaissent dans toutes les langues ; ce que
// Lattis écrit (nom de la colonne, noms des mois) suit la langue de
// l'interface (Localized).
func TestLocalized(t *testing.T) {
	for _, names := range [][]string{{"January", "February", "March"}, {"Januar", "Februar", "März"}, {"Enero", "Febrero", "Marzo"}, {"Janvier", "Février", "Mars"}} {
		s := Recognize(names)
		if s.Kind != Month || !slices.Equal(s.Values, []string{"Janvier", "Février", "Mars"}) {
			t.Errorf("%v : %s %v", names, s.Kind, s.Values)
		}
		en := Localized(s, "en")
		if en.Column != "Month" || !slices.Equal(en.Values, []string{"January", "February", "March"}) || !slices.Equal(en.Labels, en.Values) {
			t.Errorf("%v en anglais : %q %v %v", names, en.Column, en.Values, en.Labels)
		}
	}
	if s := Localized(Recognize([]string{"April", "May", "June"}), "de"); s.Column != "Monat" || !slices.Equal(s.Values, []string{"April", "Mai", "Juni"}) {
		t.Errorf("allemand : %q %v", s.Column, s.Values)
	}
	if s := Localized(Recognize([]string{"Lot 1", "Lot 2"}), "en"); s.Column != "Lot" {
		t.Errorf("partie commune traduite : %q", s.Column)
	}
	if s := Localized(Recognize([]string{"Sheet1", "Sheet2"}), "en"); s.Kind != Default || s.Column != "Number" {
		t.Errorf("noms par défaut : %s %q", s.Kind, s.Column)
	}
	if s := Localized(Recognize([]string{"Janvier", "Février"}), "fr"); s.Column != "Mois" || s.Values[0] != "Janvier" {
		t.Errorf("français : %q %v", s.Column, s.Values)
	}
}
