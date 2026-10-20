package grid

import (
	"testing"
	"time"
)

// Dates tapées et affichées selon la langue ; gardées à la française.
func TestDateStyle(t *testing.T) {
	for _, c := range []struct {
		lang, typed, kept string
	}{
		{"fr", "19/10/2026", "19/10/2026"},
		{"es", "19/10/2026", "19/10/2026"},
		{"de", "19.10.2026", "19.10.2026"}, // lu tel quel par ParseDate
		{"en", "10/19/2026", "19/10/2026"},
		{"en", "10/19", "19/10"},
		{"en", "1/2/26 9:30", "2/1/26 9:30"},
		{"en", "19/10/2026", "19/10/2026"}, // pas un mois : lu jour puis mois
		{"en", "2/30/2026", "2/30/2026"},   // date impossible : inchangée
		{"en", "Oct 19, 2026", "Oct 19, 2026"},
		{"zh", "2026/10/19", "2026/10/19"},
		{"zh", "10/19", "19/10"},
		{"en", "abc", "abc"},
	} {
		got := DateStyleFor(c.lang).Input(c.typed)
		if got != c.kept {
			t.Errorf("%s %q : %q, attendu %q", c.lang, c.typed, got, c.kept)
		}
	}
	for _, c := range []struct{ lang, kept, shown string }{
		{"fr", "19/10/2026", "19/10/2026"},
		{"en", "19/10/2026", "10/19/2026"},
		{"en", "19/10/2026 09:30", "10/19/2026 09:30"},
		{"de", "19/10/2026", "19.10.2026"},
		{"es", "19/10/2026", "19/10/2026"},
		{"zh", "19/10/2026", "2026/10/19"},
		{"en", "2026-10-19", "2026-10-19"},
		{"en", "Total", "Total"},
	} {
		if got := DateStyleFor(c.lang).Show(c.kept); got != c.shown {
			t.Errorf("%s %q : %q, attendu %q", c.lang, c.kept, got, c.shown)
		}
	}
	d := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	for lang, want := range map[string]string{"fr": "05/01/2026", "en": "01/05/2026", "de": "05.01.2026", "zh": "2026/01/05"} {
		if got := DateStyleFor(lang).Format(d); got != want {
			t.Errorf("%s : %s, attendu %s", lang, got, want)
		}
	}
	// Relue après Input, une date anglaise est la bonne.
	if d, ok := ParseDate(DateStyleFor("en").Input("10/19/2026")); !ok || d.Format("2006-01-02") != "2026-10-19" {
		t.Errorf("relecture : %v %v", d, ok)
	}
}
