package grid

import "testing"

// Dates avec heure tapées dans chaque langue, dans les deux ordres, avec
// ou sans secondes, AM/PM en anglais (demande de Louis, 23/10/2026).
func TestParseDateTimeLanguages(t *testing.T) {
	for _, c := range []struct{ lang, typed, kept string }{
		{"fr", "23/10/2026 14:30", "2026-10-23 14:30"},
		{"fr", "14:30 23/10/2026", "2026-10-23 14:30"},
		{"fr", "23/10/2026 14:30:15", "2026-10-23 14:30:15"},
		{"fr", "14:30:15 23/10/2026", "2026-10-23 14:30:15"},
		{"fr", "23/10/26 9:05", "2026-10-23 09:05"},
		{"fr", "23 oct. 2026 14:30", "2026-10-23 14:30"},
		{"fr", "23/10/2026 00:00", "2026-10-23 00:00"},
		{"es", "23/10/2026 14:30", "2026-10-23 14:30"},
		{"es", "14:30 23/10/2026", "2026-10-23 14:30"},
		{"es", "23-10-2026 14:30:15", "2026-10-23 14:30:15"},
		{"de", "23.10.2026 14:30", "2026-10-23 14:30"},
		{"de", "14:30 23.10.2026", "2026-10-23 14:30"},
		{"de", "23.10.2026 14:30:15", "2026-10-23 14:30:15"},
		{"en", "10/23/2026 2:30 PM", "2026-10-23 14:30"},
		{"en", "10/23/2026 2:30 pm", "2026-10-23 14:30"},
		{"en", "10/23/2026 12:05 AM", "2026-10-23 00:05"},
		{"en", "10/23/2026 12:05 PM", "2026-10-23 12:05"},
		{"en", "2:30 PM 10/23/2026", "2026-10-23 14:30"},
		{"en", "10/23/2026 14:30", "2026-10-23 14:30"},
		{"en", "10/23/2026 14:30:15", "2026-10-23 14:30:15"},
		{"en", "14:30 10/23/2026", "2026-10-23 14:30"},
		{"en", "10/23/2026 2:30:15 a.m.", "2026-10-23 02:30:15"},
		{"en", "Oct 23, 2026 2:30 PM", "2026-10-23 14:30"},
		{"zh", "2026/10/23 14:30", "2026-10-23 14:30"},
		{"zh", "2026-10-23 14:30:15", "2026-10-23 14:30:15"},
		{"zh", "14:30 2026/10/23", "2026-10-23 14:30"},
		{"fr", "2026-10-23T14:30:00", "2026-10-23 14:30"},
		{"en", "2026-10-23T14:30:15Z", "2026-10-23 14:30:15"},
		{"de", "2026-10-23 14:30", "2026-10-23 14:30"},
		// Sans heure : une date, comme avant.
		{"fr", "23/10/2026", "2026-10-23"},
		{"en", "10/23/2026", "2026-10-23"},
		{"de", "23.10.2026", "2026-10-23"},
		{"zh", "2026/10/23", "2026-10-23"},
	} {
		in := DateStyleFor(c.lang).Input(c.typed)
		v, err := Parse(Date, in)
		if err != nil || v != c.kept {
			t.Errorf("%s %q (lu %q) : %v %v, attendu %q", c.lang, c.typed, in, v, err, c.kept)
		}
	}
}

// Ce qui ressemble de loin à une date avec heure reste du texte.
func TestParseDateTimeRefused(t *testing.T) {
	for _, s := range []string{
		"12 3", "1.2.3", "1.2.3 4:5", "v1.2 14:30", "14:30", "14:30:15", "2:30 PM",
		"23/10/2026 25:00", "23/10/2026 14:60", "23/10/2026 14:30:61", "23/10/2026 13:00 PM",
		"23/10/2026 0:30 AM", "31/02/2026 14:30", "19.10 14:30", "14:30 abc", "abc 14:30",
		"12:30 12:45", "23/10/2026 14h30", "23/10/2026  14:30 15:00", "Salle 12 14:30",
	} {
		if d, _, ok := ParseDateTime(s); ok {
			t.Errorf("%q lu comme %v", s, d)
		}
		if _, err := Parse(Date, s); err == nil {
			t.Errorf("%q accepté dans une colonne Date", s)
		}
	}
}

// Formes gardées : colonne Date (ISO), cellule libre (jj/mm/aaaa) et
// affichage à la française.
func TestStoredDates(t *testing.T) {
	d, withTime, ok := ParseDateTime("23/10/2026 14:30:15")
	if !ok || !withTime {
		t.Fatal("lecture")
	}
	if s := StoreDate(d, true); s != "2026-10-23 14:30:15" {
		t.Error(s)
	}
	if s := FreeDate(d, true); s != "23/10/2026 14:30:15" {
		t.Error(s)
	}
	if s := FreeDate(d, false); s != "23/10/2026" {
		t.Error(s)
	}
	for in, want := range map[string]string{
		"2026-10-23":          "23/10/2026",
		"2026-10-23 14:30":    "23/10/2026 14:30",
		"2026-10-23 14:30:15": "23/10/2026 14:30:15",
		"Total":               "Total",
	} {
		if got := ShowStored(in); got != want {
			t.Errorf("%q : %q, attendu %q", in, got, want)
		}
	}
	if StoredHasTime("2026-10-23") || !StoredHasTime("2026-10-23 14:30") || StoredHasTime("abcdefghijklm") {
		t.Error("StoredHasTime")
	}
	// Le texte d'une cellule libre se montre dans chaque langue.
	for lang, want := range map[string]string{"fr": "23/10/2026 14:30", "en": "10/23/2026 14:30", "de": "23.10.2026 14:30", "es": "23/10/2026 14:30", "zh": "2026/10/23 14:30"} {
		if got := DateStyleFor(lang).Show("23/10/2026 14:30"); got != want {
			t.Errorf("%s : %q, attendu %q", lang, got, want)
		}
	}
}

// Le texte « null » vaut une case vide (demande de Louis, 23/10/2026).
func TestNullText(t *testing.T) {
	for _, s := range []string{"null", "NULL", " Null ", "nUlL"} {
		if !IsNull(s) {
			t.Errorf("%q", s)
		}
		for _, typ := range []Type{Text, Integer, Decimal, Boolean, Date} {
			if v, err := Parse(typ, s); v != nil || err != nil {
				t.Errorf("%s %q : %v %v", typ, s, v, err)
			}
		}
	}
	for _, s := range []string{"nul", "nullable", "'null", "null null", ""} {
		if IsNull(s) {
			t.Errorf("%q", s)
		}
	}
	if v, _ := Parse(Text, "nullable"); v != "nullable" {
		t.Errorf("nullable : %v", v)
	}
}
