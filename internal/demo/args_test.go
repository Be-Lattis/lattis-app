package demo

import "testing"

func TestParseArgs(t *testing.T) {
	for _, c := range []struct {
		args   []string
		preset string
		seed   uint64
		err    bool
	}{
		{nil, "", 0, false},
		{[]string{"--demo"}, "sml", 0, false},
		{[]string{"--demo", "1kx100"}, "1Kx100", 0, false},
		{[]string{"--demo=med", "--seed", "42"}, "med", 42, false},
		{[]string{"-demo", "--seed=7"}, "sml", 7, false},
		{[]string{"--demo", "géant"}, "", 0, true},
		{[]string{"--seed", "x"}, "", 0, true},
	} {
		p, s, err := ParseArgs(c.args)
		if p != c.preset || s != c.seed || (err != nil) != c.err {
			t.Errorf("%v : %q %d %v", c.args, p, s, err)
		}
	}
}
