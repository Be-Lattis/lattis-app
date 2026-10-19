package formula

import "testing"

// TestTextNumber : nombres écrits dans du texte, à la française et à
// l'étrangère (décision 0013).
func TestTextNumber(t *testing.T) {
	type want struct {
		fr, en float64
		ok     bool
	}
	same := func(f float64) want { return want{f, f, true} }
	no := want{}
	for s, w := range map[string]want{
		"12":           same(12),
		" 12 ":         same(12),
		"1 234,5":      same(1234.5),
		"1 234,5":      same(1234.5),
		"1 234,5":      same(1234.5),
		"12 €":         same(12),
		"€12":          same(12),
		"$12":          same(12),
		"12 $":         same(12),
		"£3.5":         same(3.5),
		"12 CHF":       same(12),
		"EUR 12":       same(12),
		"12 usd":       same(12),
		"15 %":         same(0.15),
		"15%":          same(0.15),
		"-12":          same(-12),
		"+12":          same(12),
		"12-":          same(-12),
		"(12)":         same(-12),
		"(1 234,5 €)":  same(-1234.5),
		"-1 234,5 EUR": same(-1234.5),
		"3,5":          same(3.5),
		"3.5":          same(3.5),
		"0.25":         same(0.25),
		"0,250":        same(0.25),
		".5":           same(0.5),
		",5":           same(0.5),
		"1234,5678":    same(1234.5678),
		"1,234.56":     same(1234.56),
		"1.234,56":     same(1234.56),
		"1'234.5":      same(1234.5),
		"1’234,5":      same(1234.5),
		"1,234,567":    same(1234567),
		"1.234.567":    same(1234567),
		"1,234,567.89": same(1234567.89),
		"1.234.567,89": same(1234567.89),
		"1,5e3":        same(1500),
		"1.5E-2":       same(0.015),
		"2e3":          same(2000),
		// Ambigu : un séparateur suivi d'exactement trois chiffres.
		"1,234":  {1.234, 1234, true},
		"1.234":  {1234, 1.234, true},
		"12,345": {12.345, 12345, true},
		"-1.234": {-1234, -1.234, true},
		// Non convertibles.
		"":           no,
		"abc":        no,
		"12abc":      no,
		"1,2,3":      no,
		"1.23.456":   no,
		"1,234.5,6":  no,
		"1.2,3.4":    no,
		"--12":       no,
		"(-12)":      no,
		"%":          no,
		"€":          no,
		"inf":        no,
		"NaN":        no,
		"0x10":       no,
		"12/03/2024": no,
		"1e":         no,
		"e5":         no,
	} {
		for _, c := range []struct {
			loc  Locale
			want float64
		}{{FR, w.fr}, {EN, w.en}} {
			got, ok := TextNumber(s, c.loc)
			if ok != w.ok || ok && got != c.want {
				t.Errorf("TextNumber(%q, %v) = %v, %v ; attendu %v, %v", s, c.loc, got, ok, c.want, w.ok)
			}
		}
	}
}

// TestAggTextColumn : la règle des colonnes de texte, fonction par fonction,
// sans passer par le parseur.
func TestAggTextColumn(t *testing.T) {
	col := []Value{Str("12"), Str("1 234,5"), Str("12 €"), Str("15 %"), Str("abc"), {}, Str(""), Str("  ")}
	for fn, want := range map[string]string{
		"SUM":     "1258,65",
		"AVERAGE": "251,73",
		"COUNT":   "4",
		"COUNTA":  "7",
		"MIN":     "0",
		"MAX":     "1234,5",
		"PRODUCT": "0",
		"MEDIAN":  "12",
		"VAR":     "301859,577",
	} {
		for _, cells := range []bool{false, true} {
			a := newAgg(fn, numRule{cells: cells})
			for _, v := range col {
				a.AddRange(v)
			}
			got := a.Result().String()
			if cells {
				// Cellules libres : textes ignorés, comme dans Excel.
				switch fn {
				case "SUM", "COUNT", "PRODUCT":
					want = "0"
				case "AVERAGE", "MEDIAN", "VAR":
					want = map[string]string{"AVERAGE": "#DIV/0!", "MEDIAN": "#NOMBRE!", "VAR": "#DIV/0!"}[fn]
				case "MIN", "MAX":
					want = "0"
				}
			}
			if got != want {
				t.Errorf("%s (cellules %v) = %s, attendu %s", fn, cells, got, want)
			}
		}
	}
	// Langue anglaise : « 1,234 » vaut 1234.
	a := newAgg("SUM", numRule{loc: EN})
	a.AddRange(Str("1,234"))
	a.AddRange(Str("1.5"))
	if got := a.Result().String(); got != "1235,5" {
		t.Errorf("SUM anglais = %s, attendu 1235,5", got)
	}
	// La règle voyage dans le nom transmis à SQLite.
	for _, r := range []numRule{{}, {loc: EN}, {cells: true}, {loc: EN, cells: true}} {
		if name, back := splitKey(r.key("SUMIF")); name != "SUMIF" || back != r {
			t.Errorf("splitKey(%q) = %q, %+v", r.key("SUMIF"), name, back)
		}
	}
}
