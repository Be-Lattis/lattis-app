package demo

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseArgs lit les options de démonstration :
//
//	--demo [preset]   ouvre un classeur de fausses données (med par défaut)
//	--seed n          graine du générateur (1 par défaut)
//
// Les autres arguments sont ignorés (laissés à Wails).
func ParseArgs(args []string) (preset string, seed uint64, err error) {
	for i := 0; i < len(args); i++ {
		name, value, inline := strings.Cut(args[i], "=")
		next := func() (string, bool) {
			if inline {
				return value, true
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				return args[i], true
			}
			return "", false
		}
		switch name {
		case "--demo", "-demo":
			v, ok := next()
			if !ok {
				v = DefaultPreset
			}
			p, found := Lookup(v)
			if !found {
				return "", 0, UnknownPresetError{Name: v}
			}
			preset = p.Name
		case "--seed", "-seed":
			v, ok := next()
			if seed, err = strconv.ParseUint(v, 10, 64); !ok || err != nil {
				return "", 0, fmt.Errorf("--seed attend un entier positif, pas « %s »", v)
			}
		}
	}
	return preset, seed, nil
}

// Usage décrit les options et les presets.
func Usage() string {
	var b strings.Builder
	b.WriteString("Usage : slitable --demo [preset] [--seed n]\nPresets :\n")
	for _, p := range Presets() {
		fmt.Fprintf(&b, "  %-9s %s\n", p.Name, p.Description)
	}
	return b.String()
}
