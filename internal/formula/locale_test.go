package formula

import "testing"

// La langue de l'interface choisit séparateurs et noms ; le stockage (EN)
// n'en dépend pas.
func TestForLanguage(t *testing.T) {
	const en = `=IF(SUM(T[A], 1.5)>2, TRUE, "x")`
	for _, c := range []struct {
		lang, want string
	}{
		{"fr", `=SI(SOMME(T[A]; 1,5)>2; VRAI; "x")`},
		{"", `=SI(SOMME(T[A]; 1,5)>2; VRAI; "x")`},
		{"en", en},
		{"zh", en},
		// Noms allemands et espagnols à venir : anglais, séparateurs à la française.
		{"de", `=IF(SUM(T[A]; 1,5)>2; TRUE; "x")`},
		{"ES", `=IF(SUM(T[A]; 1,5)>2; TRUE; "x")`},
	} {
		loc := ForLanguage(c.lang)
		n, err := Parse(en, EN)
		if err != nil {
			t.Fatal(err)
		}
		got := Format(n, loc)
		if got != c.want {
			t.Errorf("%q : %s, attendu %s", c.lang, got, c.want)
			continue
		}
		// Relu dans sa locale, la formule redonne le même texte anglais.
		back, err := Parse(got, loc)
		if err != nil {
			t.Errorf("%q : relecture : %v", c.lang, err)
			continue
		}
		if s := Format(back, EN); s != en {
			t.Errorf("%q : stockage %s, attendu %s", c.lang, s, en)
		}
	}
	if ForLanguage("fr") != FR || (Locale{}) != FR {
		t.Error("le français doit rester la valeur zéro")
	}
}

// Constantes matricielles : colonnes « . » et lignes « ; » sans point
// décimal, « , » et « ; » avec.
func TestForLanguageArray(t *testing.T) {
	n, err := Parse(`SUM({1,2;3,4})`, EN)
	if err != nil {
		t.Fatal(err)
	}
	if got := Format(n, ForLanguage("de")); got != `=SUM({1.2;3.4})` {
		t.Errorf("de : %s", got)
	}
	if _, err := Parse(`SUM({1.2;3.4})`, ForLanguage("es")); err != nil {
		t.Errorf("es : %v", err)
	}
}
