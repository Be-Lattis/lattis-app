package api

import "github.com/lalamefine/lattis/internal/msg"

// Textes des commandes dans les autres langues que le français, pour la
// référence de l'onglet Script (docs/i18n.md, § 4) : refdoc_en.go,
// refdoc_de.go, refdoc_es.go et refdoc_zh.go. Le français (les champs de
// Command) reste la référence ; une langue donne, par nom de commande, le
// résumé, les précisions et la forme du résultat, et la traduction de
// chaque description de paramètre, par son texte français (une description
// partagée par plusieurs commandes se traduit une fois). Un texte absent
// reste en français ; TestReferenceTranslated vérifie qu'il n'en manque
// aucun. Le serveur MCP et l'assistant gardent les textes français.
//
// Les formules citées restent en français : les scripts écrivent les
// formules sous la forme gardée par le classeur (docs/i18n.md, § 5).

// cmdDoc : résumé (S), précisions (D) et forme du résultat (R) d'une
// commande dans une langue.
type cmdDoc struct{ S, D, R string }

// refPhrases : les morceaux de phrase qui assemblent les précisions.
type refPhrases struct {
	params string // « Paramètres — »
	colon  string // entre un paramètre et sa description
	sep    string // entre deux paramètres
	named  string // appel par une table d'arguments nommés (%s : commande, premier paramètre)
	undo   string // commande annulable
	stop   string // point final de la liste des paramètres
	space  string // entre deux phrases des précisions
}

// apiDoc : les textes des commandes dans une langue.
type apiDoc struct {
	phrases refPhrases
	cmds    map[string]cmdDoc
	params  map[string]string // description française → traduction
}

var apiDocFR = apiDoc{phrases: refPhrases{
	params: "Paramètres — ",
	colon:  " : ",
	sep:    " ; ",
	named:  "Arguments dans l'ordre, ou une table d'arguments nommés : %s{%s = …}.",
	undo:   "Annulable : tout ce que le script modifie s'annule d'un coup (Ctrl+Z).",
	stop:   ".",
	space:  " ",
}}

// apiDocs : les textes de chaque langue autre que le français.
var apiDocs = map[msg.Lang]*apiDoc{msg.EN: &apiDocEN, msg.DE: &apiDocDE, msg.ES: &apiDocES, msg.ZH: &apiDocZH}

// apiDocIn : les textes de la langue l (le français pour une langue sans
// textes).
func apiDocIn(l msg.Lang) *apiDoc {
	if d := apiDocs[l]; d != nil {
		return d
	}
	return &apiDocFR
}
