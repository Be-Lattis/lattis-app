package grid

import (
	"errors"
	"strconv"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		typ  Type
		raw  string
		want any
		err  bool
	}{
		{Integer, "42", int64(42), false},
		{Integer, " 1 234 ", int64(1234), false},
		{Integer, "1 234", int64(1234), false},
		{Integer, "12a", nil, true},
		{Decimal, "3,5", 3.5, false},
		{Decimal, "1 234,25", 1234.25, false},
		{Decimal, "2.5", 2.5, false},
		{Decimal, "abc", nil, true},
		{Decimal, "inf", nil, true},
		{Decimal, "NaN", nil, true},
		{Decimal, "1e3", 1000.0, false},
		{Decimal, "12,5 €", 12.5, false},
		{Decimal, "(3,5)", -3.5, false},
		{Integer, "12 €", int64(12), false},
		{Integer, "(12)", int64(-12), false},
		{Integer, "€", nil, true},
		{Integer, "()", nil, true},
		{Boolean, "VRAI", int64(1), false},
		{Boolean, "faux", int64(0), false},
		{Boolean, "peut-être", nil, true},
		{Date, "15/10/2026", "2026-10-15", false},
		{Date, "5/3/2026", "2026-03-05", false},
		{Date, "2026-10-15", "2026-10-15", false},
		{Date, "32/01/2026", nil, true},
		{Date, "16/10", strconv.Itoa(time.Now().Year()) + "-10-16", false},
		{Date, "5/3", strconv.Itoa(time.Now().Year()) + "-03-05", false},
		{Date, "16/13", nil, true},
		{Text, "  bonjour ", "  bonjour ", false},
		{Integer, "", nil, false},
		{Text, "   ", nil, false},
	}
	for _, tt := range tests {
		got, err := Parse(tt.typ, tt.raw)
		if tt.err {
			if !errors.Is(err, ErrValue) {
				t.Errorf("Parse(%s, %q) : erreur attendue, obtenu %v, %v", tt.typ, tt.raw, got, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("Parse(%s, %q) = %v, %v ; attendu %v", tt.typ, tt.raw, got, err, tt.want)
		}
	}
}
