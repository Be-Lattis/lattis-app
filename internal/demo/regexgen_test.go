package demo

import (
	"regexp"
	"testing"
)

func TestRegexGen(t *testing.T) {
	g := NewGen(7, "regex")
	for _, p := range []string{`[A-Z]{3}-\d{4}`, `FR\d{2} ?\d{4}`, `(?i)abc|x[0-9a-f]+`, `[^a-z]{2,5}`, `\w+@exemple\.fr`, `.{3}`, `^ID-\d+$`, `(ab)*c?`} {
		re, err := compileRegexGen(p)
		if err != nil {
			t.Fatal(err)
		}
		check := regexp.MustCompile(`^(?:` + p + `)$`)
		for range 200 {
			if s := g.Regex(re); !check.MatchString(s) {
				t.Fatalf("%s : %q ne correspond pas", p, s)
			}
		}
	}
	if _, err := compileRegexGen(`[A-`); err == nil {
		t.Fatal("expression invalide acceptée")
	}
}
