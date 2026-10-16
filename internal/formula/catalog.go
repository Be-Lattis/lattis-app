package formula

import "sort"

// FunctionInfo décrit une fonction pour l'aide à la saisie.
type FunctionInfo struct {
	Name  string `json:"name"`  // nom dans la langue demandée
	Other string `json:"other"` // nom dans l'autre langue
	Arity string `json:"arity"` // « 2 à 3 arguments »
}

// Functions liste les fonctions connues, triées par nom, pour l'aide à la
// saisie (autocomplétion, signature).
func Functions(loc Locale) []FunctionInfo {
	seen := map[string]bool{}
	var out []FunctionInfo
	for _, f := range funcIndex {
		if seen[f.EN] {
			continue
		}
		seen[f.EN] = true
		other := EN
		if loc == EN {
			other = FR
		}
		out = append(out, FunctionInfo{Name: f.name(loc), Other: f.name(other), Arity: f.arity()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
