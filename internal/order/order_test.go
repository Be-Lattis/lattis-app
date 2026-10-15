package order

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestBetweenBasics(t *testing.T) {
	cases := []struct{ a, b string }{
		{"", ""}, {"", "V"}, {"V", ""}, {"V", "W"}, {"V", "V1"}, {"V1", "W"},
		{"1", "2"}, {"", "1"}, {"z", ""}, {"zz", ""}, {"", "01"}, {"V", "Vz"},
		{"Vzz", "W"}, {"a1", "a11"},
	}
	for _, c := range cases {
		k, err := Between(c.a, c.b)
		if err != nil {
			t.Fatalf("Between(%q, %q): %v", c.a, c.b, err)
		}
		if !valid(k) || k == "" {
			t.Fatalf("Between(%q, %q) = %q : clé invalide", c.a, c.b, k)
		}
		if (c.a != "" && k <= c.a) || (c.b != "" && k >= c.b) {
			t.Fatalf("Between(%q, %q) = %q : hors bornes", c.a, c.b, k)
		}
	}
}

func TestBetweenRejectsBadBounds(t *testing.T) {
	for _, c := range []struct{ a, b string }{{"W", "V"}, {"V", "V"}, {"V0", ""}, {"", "é"}} {
		if _, err := Between(c.a, c.b); err == nil {
			t.Errorf("Between(%q, %q) aurait dû échouer", c.a, c.b)
		}
	}
}

// Insertions aléatoires en tête, en queue et au milieu : la liste doit rester
// triée et sans doublon, et les clés rester courtes.
func TestRandomInsertions(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	keys := []string{}
	for range 20_000 {
		i := r.IntN(len(keys) + 1)
		var a, b string
		if i > 0 {
			a = keys[i-1]
		}
		if i < len(keys) {
			b = keys[i]
		}
		k, err := Between(a, b)
		if err != nil {
			t.Fatalf("Between(%q, %q): %v", a, b, err)
		}
		keys = slices.Insert(keys, i, k)
	}
	if !slices.IsSorted(keys) {
		t.Fatal("clés non triées")
	}
	for i := 1; i < len(keys); i++ {
		if keys[i] == keys[i-1] {
			t.Fatalf("doublon %q", keys[i])
		}
	}
	longest := 0
	for _, k := range keys {
		longest = max(longest, len(k))
	}
	if longest > 12 {
		t.Errorf("clé la plus longue : %d caractères", longest)
	}
}

// Le pire cas : toujours insérer au même endroit allonge les clés d'environ
// un caractère toutes les 6 insertions (log2(62)).
func TestRepeatedInsertSamePlace(t *testing.T) {
	a, b := "V", "W"
	for range 600 {
		k, err := Between(a, b)
		if err != nil {
			t.Fatal(err)
		}
		b = k
	}
	if len(b) > 110 {
		t.Errorf("clé de %d caractères après 600 insertions au même endroit", len(b))
	}
}

func TestSpread(t *testing.T) {
	for _, n := range []int{1, 2, 61, 62, 1000, 1_000_000} {
		keys := Spread(n)
		if len(keys) != n {
			t.Fatalf("Spread(%d) : %d clés", n, len(keys))
		}
		if !slices.IsSorted(keys) {
			t.Fatalf("Spread(%d) non trié", n)
		}
		for i, k := range keys {
			if !valid(k) || k == "" {
				t.Fatalf("Spread(%d)[%d] = %q invalide", n, i, k)
			}
			if i > 0 && k == keys[i-1] {
				t.Fatalf("Spread(%d) doublon %q", n, k)
			}
		}
		// On peut toujours insérer entre deux clés voisines.
		if n > 1 {
			if _, err := Between(keys[0], keys[1]); err != nil {
				t.Fatal(err)
			}
		}
	}
}
