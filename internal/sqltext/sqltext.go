// Package sqltext prépare les requêtes SQL de l'utilisateur (onglet SQL,
// scripts, serveur MCP) et rend leurs erreurs lisibles : messages de SQLite
// traduits en français, changements de structure refusés avec un message
// clair, définitions ajoutées en tête de la clause WITH.
package sqltext

import (
	"errors"
	"regexp"
	"strings"
)

// ErrStructure signale une requête qui changerait la structure de la base :
// les tableaux et leurs colonnes se créent, se renomment et se suppriment
// depuis la grille (ou par les commandes tables.* et columns.*).
var ErrStructure = errors.New("Les changements de structure (DROP, ALTER, CREATE…) se font depuis la grille : seules les requêtes SELECT, INSERT, UPDATE et DELETE s'exécutent ici")

// structureWords : premiers mots d'une instruction qui change la structure.
var structureWords = map[string]bool{"create": true, "drop": true, "alter": true, "reindex": true}

// CheckStructure refuse une requête qui commence par CREATE, DROP, ALTER ou
// REINDEX (après d'éventuels commentaires).
func CheckStructure(q string) error {
	w, _ := Word(q, SkipBlank(q, 0))
	if structureWords[w] {
		return ErrStructure
	}
	return nil
}

// SkipBlank saute les espaces et les commentaires à partir de i.
func SkipBlank(q string, i int) int {
	for i < len(q) {
		switch {
		case q[i] == ' ' || q[i] == '\t' || q[i] == '\n' || q[i] == '\r' || q[i] == ';':
			i++
		case strings.HasPrefix(q[i:], "--"):
			if j := strings.IndexByte(q[i:], '\n'); j >= 0 {
				i += j + 1
			} else {
				return len(q)
			}
		case strings.HasPrefix(q[i:], "/*"):
			if j := strings.Index(q[i+2:], "*/"); j >= 0 {
				i += j + 4
			} else {
				return len(q)
			}
		default:
			return i
		}
	}
	return i
}

// Word lit un mot-clé (lettres ASCII) à partir de i, en minuscules, et rend
// la position qui le suit.
func Word(q string, i int) (string, int) {
	j := i
	for j < len(q) && (q[j] >= 'a' && q[j] <= 'z' || q[j] >= 'A' && q[j] <= 'Z') {
		j++
	}
	return strings.ToLower(q[i:j]), j
}

// PrependWith ajoute des expressions de table (« nom AS (SELECT …) ») en
// tête de la requête : dans sa clause WITH si elle en a une (RECURSIVE
// compris), dans une nouvelle sinon. Une requête qui n'est ni SELECT, ni
// VALUES, ni WITH est rendue telle quelle.
func PrependWith(q string, defs []string) string {
	if len(defs) == 0 {
		return q
	}
	with := strings.Join(defs, ", ")
	start := SkipBlank(q, 0)
	kw, end := Word(q, start)
	switch kw {
	case "select", "values":
		return "WITH " + with + " " + q[start:]
	case "with":
		rest := SkipBlank(q, end)
		head := "WITH "
		if kw2, end2 := Word(q, rest); kw2 == "recursive" {
			head, rest = "WITH RECURSIVE ", SkipBlank(q, end2)
		}
		return head + with + ", " + q[rest:]
	}
	return q
}

// frError est une erreur SQL traduite ; elle garde l'erreur d'origine
// (errors.Is, errors.As).
type frError struct {
	msg string
	err error
}

func (e *frError) Error() string { return e.msg }
func (e *frError) Unwrap() error { return e.err }

// engine : préfixe technique des erreurs du pilote (« sqlite3: SQL logic
// error: »), retiré du message.
var engine = regexp.MustCompile(`^(?:sqlite3:\s*)?(?:(?:SQL logic error|constraint failed|datatype mismatch|generic error|SQL error or missing database|abort due to constraint violation):\s*)?`)

type rule struct {
	re  *regexp.Regexp
	out string // remplacement ($1, $2…) ; le détail d'origine suit entre parenthèses si keep
	// keep : le message d'origine suit entre parenthèses (détail technique utile).
	keep bool
}

var rules = []rule{
	{regexp.MustCompile(`^no such table: (.+)$`), "Tableau inconnu : « $1 »", false},
	{regexp.MustCompile(`^no such column: (.+)$`), "Colonne inconnue : « $1 »", false},
	{regexp.MustCompile(`^no such function: (.+)$`), "Fonction inconnue : « $1 »", false},
	{regexp.MustCompile(`^near "(.+)": syntax error$`), "Erreur de syntaxe près de « $1 »", false},
	{regexp.MustCompile(`^unrecognized token: "?(.+?)"?$`), "Erreur de syntaxe : « $1 » n'est pas reconnu", false},
	{regexp.MustCompile(`^incomplete input$`), "Requête incomplète : il manque la fin (parenthèse, mot-clé ou valeur)", false},
	{regexp.MustCompile(`^ambiguous column name: (.+)$`), "Colonne ambiguë : « $1 » est dans plusieurs tableaux de la requête ; préciser le tableau (Tableau.\"$1\")", false},
	{regexp.MustCompile(`^wrong number of arguments to function (.+)$`), "Nombre d'arguments incorrect pour la fonction $1", false},
	{regexp.MustCompile(`^misuse of aggregate(?: function)?:? (.+)$`), "Fonction d'agrégat mal employée : $1 (dans WHERE, utiliser HAVING)", false},
	{regexp.MustCompile(`^misuse of window function (.+)$`), "Fonction de fenêtre mal employée : $1", false},
	{regexp.MustCompile(`^no tables specified$`), "Aucun tableau indiqué : ajouter FROM", false},
	{regexp.MustCompile(`^SELECTs to the left and right of (\w+(?: ALL)?) do not have the same number of result columns$`), "Les SELECT de part et d'autre de $1 n'ont pas le même nombre de colonnes", false},
	{regexp.MustCompile(`^sub-select returns (\d+) columns - expected 1$`), "La sous-requête rend $1 colonnes au lieu d'une", false},
	{regexp.MustCompile(`^row value misused$`), "Liste de valeurs entre parenthèses mal employée", true},
	{regexp.MustCompile(`^(\d+)(?:st|nd|rd|th) ORDER BY term out of range - should be between 1 and (\d+)$`), "Le terme n° $1 de ORDER BY est hors limites : entre 1 et $2", false},
	{regexp.MustCompile(`^(\d+)(?:st|nd|rd|th) GROUP BY term out of range - should be between 1 and (\d+)$`), "Le terme n° $1 de GROUP BY est hors limites : entre 1 et $2", false},
	{regexp.MustCompile(`^table (.+) has (\d+) columns but (\d+) values were supplied$`), "Le tableau « $1 » a $2 colonnes, mais $3 valeurs sont données", false},
	{regexp.MustCompile(`^(\d+) values for (\d+) columns$`), "$1 valeurs pour $2 colonnes", false},
	{regexp.MustCompile(`^(?:UNIQUE constraint failed|PRIMARY KEY must be unique): (.+)$`), "Valeur en double : une autre ligne a déjà cette clé", true},
	{regexp.MustCompile(`^NOT NULL constraint failed: (.+)$`), "Valeur obligatoire manquante", true},
	{regexp.MustCompile(`^attempt to write a readonly database$`), "Cette requête ne peut pas écrire : seules INSERT, UPDATE et DELETE modifient les tableaux", false},
	{regexp.MustCompile(`^(?:interrupted|context deadline exceeded|context canceled)$`), "Requête interrompue : trop longue, ou arrêtée", false},
	{regexp.MustCompile(`^database is locked$`), "Le classeur est occupé : réessayer dans un instant", false},
	{regexp.MustCompile(`^circular reference: (.+)$`), "Référence circulaire : « $1 »", false},
	{regexp.MustCompile(`^too many terms in compound SELECT$`), "Trop de SELECT réunis (UNION…)", false},
	{regexp.MustCompile(`^DISTINCT aggregates must have exactly one argument$`), "Un agrégat DISTINCT prend un seul argument", false},
	{regexp.MustCompile(`^cannot use window function in (.+)$`), "Fonction de fenêtre interdite dans $1", true},
	{regexp.MustCompile(`^a GROUP BY clause is required before HAVING$`), "HAVING demande un GROUP BY", false},
	{regexp.MustCompile(`^(?:aggregate functions are not allowed in the GROUP BY clause)$`), "Les fonctions d'agrégat ne vont pas dans GROUP BY", false},
	{regexp.MustCompile(`^(?:LIMIT|OFFSET) clause should come after (\w+) not before$`), "LIMIT se place après $1", true},
	{regexp.MustCompile(`^datatype mismatch$`), "Type de valeur incompatible", true},
}

// French traduit en français une erreur de SQLite (no such table, no such
// column, syntax error…) ; le détail d'origine reste entre parenthèses
// quand il aide. Une erreur déjà française, ou inconnue, garde son texte
// sans le préfixe technique du pilote. nil reste nil.
func French(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*frError); ok {
		return err
	}
	raw := err.Error()
	msg := engine.ReplaceAllString(raw, "")
	for _, r := range rules {
		if m := r.re.FindStringSubmatchIndex(msg); m != nil {
			out := string(r.re.ExpandString(nil, r.out, msg, m))
			if r.keep {
				out += " (" + msg + ")"
			}
			return &frError{msg: out, err: err}
		}
	}
	if msg == raw {
		return err
	}
	return &frError{msg: msg, err: err}
}
