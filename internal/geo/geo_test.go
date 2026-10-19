package geo

import (
	"math/rand"
	"runtime"
	"testing"
	"time"
)

func TestLookup(t *testing.T) {
	cases := []struct {
		what     string
		lat, lon float64
		code     string // "" : pas de commune
		name     string
	}{
		{"Paris, Notre-Dame", 48.8530, 2.3499, "75056", "Paris"},
		{"Lyon, place Bellecour", 45.7578, 4.8320, "69123", "Lyon"},
		{"Marseille, Vieux-Port", 43.2951, 5.3740, "13055", "Marseille"},
		{"Ajaccio", 41.9192, 8.7386, "2A004", "Ajaccio"},
		{"Saint-Denis de La Réunion", -20.8821, 55.4507, "97411", "Saint-Denis"},
		{"Nice, plage de la promenade des Anglais", 43.6935, 7.2600, "06088", "Nice"},
		{"Biarritz, Grande Plage", 43.4844, -1.5586, "64122", "Biarritz"},
		// Enclave des Papes : Valréas (Vaucluse) entourée de la Drôme, et
		// Grignan juste à côté, côté Drôme.
		{"Valréas (enclave)", 44.3842, 4.9906, "84138", "Valréas"},
		{"Grignan", 44.4197, 4.9086, "26146", "Grignan"},
		// Limite Paris / Boulogne-Billancourt, de part et d'autre du
		// boulevard périphérique à la porte de Saint-Cloud.
		{"Paris, porte de Saint-Cloud", 48.8395, 2.2590, "75056", "Paris"},
		{"Boulogne-Billancourt, centre", 48.8352, 2.2410, "92012", "Boulogne-Billancourt"},
		{"golfe de Gascogne (en mer)", 46.0, -5.0, "", ""},
		{"Manche, au large", 50.2, -1.0, "", ""},
		{"Bruxelles", 50.8466, 4.3528, "", ""},
		{"Madrid", 40.4168, -3.7038, "", ""},
		{"Philipsburg (Sint Maarten, partie néerlandaise)", 18.0237, -63.0458, "", ""},
		{"Marigot (Saint-Martin, partie française)", 18.0680, -63.0830, "97801", "Saint-Martin"},
		{"pôle Nord", 90, 0, "", ""},
	}
	for _, c := range cases {
		got, ok := Lookup(c.lat, c.lon)
		switch {
		case c.code == "" && ok:
			t.Errorf("%s : %s (%s), attendu aucune commune", c.what, got.Name, got.Code)
		case c.code != "" && !ok:
			t.Errorf("%s : aucune commune, attendu %s", c.what, c.code)
		case c.code != "" && (got.Code != c.code || got.Name != c.name):
			t.Errorf("%s : %s (%s), attendu %s (%s)", c.what, got.Name, got.Code, c.name, c.code)
		}
	}
	p, _ := Lookup(48.8530, 2.3499)
	if p.DeptCode != "75" || p.DeptName != "Paris" || p.RegionCode != "11" || p.RegionName != "Île-de-France" || len(p.PostalCodes) < 20 || p.PostalCodes[0] != "75001" {
		t.Errorf("Paris : %+v", p)
	}
	if n := Count(); n < 34000 {
		t.Errorf("%d communes chargées", n)
	}
	if Source() == "" {
		t.Error("source vide")
	}
}

// TestLookupSpeed mesure le chargement et 100 000 recherches en France
// métropolitaine (points tirés au hasard, terre et mer).
func TestLookupSpeed(t *testing.T) {
	var before runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	t0 := time.Now()
	b, err := load(data)
	if err != nil {
		t.Fatal(err)
	}
	load := time.Since(t0)
	var after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(b)
	r := rand.New(rand.NewSource(1))
	const n = 100_000
	t0 = time.Now()
	found := 0
	for range n {
		if _, ok := Lookup(42.5+r.Float64()*8.5, -4.5+r.Float64()*12.5); ok {
			found++
		}
	}
	d := time.Since(t0)
	t.Logf("chargement %v (%.1f Mo en mémoire), %d recherches en %v (%v chacune, %d trouvées)",
		load, float64(after.HeapAlloc-before.HeapAlloc)/1e6, n, d, d/n, found)
}
