package update

import "testing"

func TestParseVersion(t *testing.T) {
	for _, s := range []string{"0.2.0-beta", "v0.2.0-beta", "1.2.3", "v1.2.3-beta.2", "1.0.0-rc.1+build.5", "10.20.30"} {
		if _, ok := ParseVersion(s); !ok {
			t.Errorf("%q : version attendue", s)
		}
	}
	for _, s := range []string{"dev", "", "1.2", "1.2.3.4", "v1.2.x", "01.2.3", "1.2.3-", "1.2.3-beta..1", "1.2.3-bêta"} {
		if _, ok := ParseVersion(s); ok {
			t.Errorf("%q : pas une version", s)
		}
	}
	v, _ := ParseVersion("v0.3.0-beta.2")
	if v.String() != "0.3.0-beta.2" || !v.Prerelease() {
		t.Errorf("v0.3.0-beta.2 → %s", v)
	}
}

func TestCompareSemVer(t *testing.T) {
	// Ordre croissant (SemVer 2.0, § 11).
	order := []string{
		"0.1.0-beta.1", "0.1.0-beta.2", "0.1.0-beta.10", "0.1.0", "0.2.0-alpha",
		"0.2.0-beta", "0.2.0-beta.1", "0.2.0-beta.2", "0.2.0-rc.1", "0.2.0", "0.2.1-beta", "0.10.0", "1.0.0",
	}
	for i := range order {
		for j := range order {
			a, _ := ParseVersion(order[i])
			b, _ := ParseVersion(order[j])
			want := sign(i - j)
			if got := Compare(a, b); got != want {
				t.Errorf("Compare(%s, %s) = %d, attendu %d", order[i], order[j], got, want)
			}
		}
	}
	a, _ := ParseVersion("v1.2.3")
	b, _ := ParseVersion("1.2.3+abc")
	if Compare(a, b) != 0 || Newer(a, b) {
		t.Error("v1.2.3 et 1.2.3+abc sont la même version")
	}
}
