package formula

import "sort"

// FunctionInfo décrit une fonction pour l'aide à la saisie (infobulle de
// signature, autocomplétion, dialogue « Insérer une fonction »).
type FunctionInfo struct {
	Name        string   `json:"name"`        // nom dans la langue demandée
	Other       string   `json:"other"`       // nom dans l'autre langue
	Arity       string   `json:"arity"`       // « 2 à 3 arguments »
	Category    string   `json:"category"`    // « Agrégats »
	Description string   `json:"description"` // une phrase
	Signature   string   `json:"signature"`   // SOMME.SI(plage; critère; [plage_somme])
	Args        []ArgDoc `json:"args"`        // arguments, dans l'ordre
	Repeat      int      `json:"repeat"`      // nombre de derniers arguments qui se répètent
	Example     string   `json:"example"`     // formule d'exemple, en français
}

// Functions liste les fonctions connues, triées par nom, pour l'aide à la
// saisie (autocomplétion, signature).
func Functions(loc Locale) []FunctionInfo {
	seen := map[string]bool{}
	var out []FunctionInfo
	for _, f := range funcIndex {
		if seen[f.EN] || refused[f.EN] {
			continue
		}
		seen[f.EN] = true
		other := EN
		if loc == EN {
			other = FR
		}
		d := docs[f.EN]
		as := make([]ArgDoc, 0, len(d.args))
		for _, a := range d.args {
			a.Kind = argKind(a)
			as = append(as, a)
		}
		out = append(out, FunctionInfo{
			Name: f.name(loc), Other: f.name(other), Arity: f.arity(),
			Category: d.cat, Description: d.desc, Signature: d.signature(f.name(loc)),
			Args: as, Repeat: d.repeat, Example: d.example,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
