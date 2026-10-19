package formula

import "testing"

// TestSheetChoice : ONGLET.CHOIX() rend la valeur choisie de la feuille de
// la formule (formule libre : Schema.Sheet ; colonne calculée : la feuille
// de son tableau), typée, sur les deux backends ; vide pour « (Toutes) »,
// #N/A sans choix.
func TestSheetChoice(t *testing.T) {
	f := fixture(t)
	f.schema.Choices = map[int64]Value{2: Num(2025), 3: {}}
	f.ventes.Sheet = 2
	cases := []struct {
		src   string
		sheet int64
		row   int
		want  string
	}{
		{"=ONGLET.CHOIX()", 2, 0, "2025"},
		{"=ONGLET.CHOIX()+1", 2, 0, "2026"},
		{"=ESTVIDE(ONGLET.CHOIX())", 3, 0, "VRAI"},
		{"=ONGLET.CHOIX()", 1, 0, "#N/A"},
		{"=ONGLET.CHOIX()", 0, 0, "#N/A"},
		// Colonne calculée de Ventes (feuille 2), quelle que soit la feuille affichée.
		{"=ONGLET.CHOIX()", 1, 1, "2025"},
		{"=SHEETCHOICE()", 2, 0, "2025"},
	}
	for _, c := range cases {
		f.schema.Sheet = c.sheet
		loc := FR
		if c.src == "=SHEETCHOICE()" {
			loc = EN
		}
		n := mustParse(t, c.src, loc)
		if got := f.evalGo(n, c.row).String(); got != c.want {
			t.Errorf("%s (feuille %d) : Go donne %q, attendu %q", c.src, c.sheet, got, c.want)
		}
		if got := f.evalSQL(t, n, c.row).String(); got != c.want {
			t.Errorf("%s (feuille %d) : SQL donne %q, attendu %q", c.src, c.sheet, got, c.want)
		}
	}
	if !External("=ONGLET.CHOIX()", "Ventes") {
		t.Error("ONGLET.CHOIX dépend de l'onglet, pas du seul tableau")
	}
}
