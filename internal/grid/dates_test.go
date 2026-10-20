package grid

import (
	"testing"
	"time"
)

// Dates courtes et mois en lettres (demande de Louis, 19/10/2026).
func TestParseDateShort(t *testing.T) {
	y := time.Now().Year()
	ok := map[string]string{
		"19/10/2026":     "2026-10-19",
		"19/10/26":       "2026-10-19",
		"1/2/26":         "2026-02-01",
		"19-10-2026":     "2026-10-19",
		"19.10.2026":     "2026-10-19",
		"19.10.26":       "2026-10-19",
		"5/3/45":         "1945-03-05",
		"2026-10-19":     "2026-10-19",
		"2026/10/19":     "2026-10-19",
		"19 oct. 2026":   "2026-10-19",
		"1er janvier 26": "2026-01-01",
		"19 Octobre":     time.Date(y, 10, 19, 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
		"19/10":          time.Date(y, 10, 19, 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
		"Oct 19, 2026":   "2026-10-19",
		"19 août 2026":   "2026-08-19",
	}
	for in, want := range ok {
		d, good := ParseDate(in)
		if !good || d.Format("2006-01-02") != want {
			t.Errorf("%q : %v %v, attendu %s", in, d, good, want)
		}
	}
	for _, in := range []string{"31/02/2026", "19.10", "1,5", "abc", "19/13/2026", "123/4/2026", "19 truc 2026", "2026-10"} {
		if d, good := ParseDate(in); good {
			t.Errorf("%q lu comme %v", in, d)
		}
	}
	if _, good := ParseFullDate("19/10"); good {
		t.Error("ParseFullDate sans année")
	}
	if _, good := ParseFullDate("19/10/26"); !good {
		t.Error("ParseFullDate 19/10/26")
	}
	if d, good := ParseDateIn("10/19/26", DateStyleFor("en")); !good || d.Format("2006-01-02") != "2026-10-19" {
		t.Errorf("anglais : %v %v", d, good)
	}
}
