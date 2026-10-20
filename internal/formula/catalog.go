package formula

import "github.com/lalamefine/lattis/internal/msg"

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

// BareFunction indique si un nom désigne une fonction même écrit sans
// parenthèses : l'agrégat de GROUPER.PAR (SOMME, MOYENNE…).
func BareFunction(name string) bool {
	_, ok := groupAgg(Name{Name: name})
	return ok
}

// Functions liste les fonctions connues, triées par nom, pour l'aide à la
// saisie (autocomplétion, signature), nombre d'arguments en français (voir
// Catalog).
func Functions(loc Locale) []FunctionInfo { return Catalog(loc, msg.FR) }
