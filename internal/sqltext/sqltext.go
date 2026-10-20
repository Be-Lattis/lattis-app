// Package sqltext prépare les requêtes SQL de l'utilisateur (onglet SQL,
// scripts, serveur MCP) et rend leurs erreurs lisibles : messages de SQLite
// traduits en français, changements de structure refusés avec un message
// clair, définitions ajoutées en tête de la clause WITH.
package sqltext

import (
	"github.com/lalamefine/lattis/internal/msg"
	"regexp"
	"strings"
)

// ErrStructure signale une requête qui changerait la structure de la base :
// les tableaux et leurs colonnes se créent, se renomment et se suppriment
// depuis la grille (ou par les commandes tables.* et columns.*).
var ErrStructure = msg.New("sql.structure", nil)

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

// frError est une erreur SQL traduite (m) ou seulement débarrassée du
// préfixe du pilote (text, m vide) ; elle garde l'erreur d'origine
// (errors.Is, errors.As).
type frError struct {
	m    msg.Message
	text string
	err  error
}

func (e *frError) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e *frError) Msg() msg.Message {
	if e.m.ID == "" {
		return msg.M("sql.raw", msg.Args{"text": e.text})
	}
	return e.m
}

func (e *frError) Unwrap() error { return e.err }

// engine : préfixe technique des erreurs du pilote (« sqlite3: SQL logic
// error: »), retiré du message.
var engine = regexp.MustCompile(`^(?:sqlite3:\s*)?(?:(?:SQL logic error|constraint failed|datatype mismatch|generic error|SQL error or missing database|abort due to constraint violation):\s*)?`)

// rule : un message de SQLite et son message (id) ; params nomme les
// groupes de l'expression ; le message d'origine est le paramètre detail
// (détail technique utile entre parenthèses).
type rule struct {
	re     *regexp.Regexp
	id     msg.ID
	params []string
}

var rules = []rule{
	{regexp.MustCompile(`^no such table: (.+)$`), "sql.sqlite.noSuchTable", []string{"name"}},
	{regexp.MustCompile(`^no such column: (.+)$`), "sql.sqlite.noSuchColumn", []string{"name"}},
	{regexp.MustCompile(`^no such function: (.+)$`), "sql.sqlite.noSuchFunction", []string{"name"}},
	{regexp.MustCompile(`^near "(.+)": syntax error$`), "sql.sqlite.syntaxNear", []string{"token"}},
	{regexp.MustCompile(`^unrecognized token: "?(.+?)"?$`), "sql.sqlite.unrecognizedToken", []string{"token"}},
	{regexp.MustCompile(`^incomplete input$`), "sql.sqlite.incomplete", nil},
	{regexp.MustCompile(`^ambiguous column name: (.+)$`), "sql.sqlite.ambiguousColumn", []string{"name"}},
	{regexp.MustCompile(`^wrong number of arguments to function (.+)$`), "sql.sqlite.argumentCount", []string{"function"}},
	{regexp.MustCompile(`^misuse of aggregate(?: function)?:? (.+)$`), "sql.sqlite.aggregateMisuse", []string{"function"}},
	{regexp.MustCompile(`^misuse of window function (.+)$`), "sql.sqlite.windowMisuse", []string{"function"}},
	{regexp.MustCompile(`^no tables specified$`), "sql.sqlite.noTables", nil},
	{regexp.MustCompile(`^SELECTs to the left and right of (\w+(?: ALL)?) do not have the same number of result columns$`), "sql.sqlite.compoundColumns", []string{"operator"}},
	{regexp.MustCompile(`^sub-select returns (\d+) columns - expected 1$`), "sql.sqlite.subselectColumns", []string{"n"}},
	{regexp.MustCompile(`^row value misused$`), "sql.sqlite.rowValue", nil},
	{regexp.MustCompile(`^(\d+)(?:st|nd|rd|th) ORDER BY term out of range - should be between 1 and (\d+)$`), "sql.sqlite.orderByRange", []string{"term", "max"}},
	{regexp.MustCompile(`^(\d+)(?:st|nd|rd|th) GROUP BY term out of range - should be between 1 and (\d+)$`), "sql.sqlite.groupByRange", []string{"term", "max"}},
	{regexp.MustCompile(`^table (.+) has (\d+) columns but (\d+) values were supplied$`), "sql.sqlite.valuesCount", []string{"table", "columns", "values"}},
	{regexp.MustCompile(`^(\d+) values for (\d+) columns$`), "sql.sqlite.valuesForColumns", []string{"values", "columns"}},
	{regexp.MustCompile(`^(?:UNIQUE constraint failed|PRIMARY KEY must be unique): (.+)$`), "sql.sqlite.unique", []string{"column"}},
	{regexp.MustCompile(`^NOT NULL constraint failed: (.+)$`), "sql.sqlite.notNull", []string{"column"}},
	{regexp.MustCompile(`^attempt to write a readonly database$`), "sql.sqlite.readOnly", nil},
	{regexp.MustCompile(`^(?:interrupted|context deadline exceeded|context canceled)$`), "sql.sqlite.interrupted", nil},
	{regexp.MustCompile(`^database is locked$`), "sql.sqlite.locked", nil},
	{regexp.MustCompile(`^circular reference: (.+)$`), "sql.sqlite.circular", []string{"name"}},
	{regexp.MustCompile(`^too many terms in compound SELECT$`), "sql.sqlite.tooManyTerms", nil},
	{regexp.MustCompile(`^DISTINCT aggregates must have exactly one argument$`), "sql.sqlite.distinctArgument", nil},
	{regexp.MustCompile(`^cannot use window function in (.+)$`), "sql.sqlite.windowForbidden", []string{"clause"}},
	{regexp.MustCompile(`^a GROUP BY clause is required before HAVING$`), "sql.sqlite.havingNeedsGroupBy", nil},
	{regexp.MustCompile(`^(?:aggregate functions are not allowed in the GROUP BY clause)$`), "sql.sqlite.aggregateInGroupBy", nil},
	{regexp.MustCompile(`^(?:LIMIT|OFFSET) clause should come after (\w+) not before$`), "sql.sqlite.limitPlace", []string{"clause"}},
	{regexp.MustCompile(`^datatype mismatch$`), "sql.sqlite.datatypeMismatch", nil},
	// Autorisateur des requêtes de l'utilisateur (store.UserConn).
	{regexp.MustCompile(`^(?:authorization denied:\s*)?not authorized$`), "sql.sqlite.notAuthorized", nil},
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
	text := engine.ReplaceAllString(raw, "")
	for _, r := range rules {
		if m := r.re.FindStringSubmatch(text); m != nil {
			args := msg.Args{"detail": text}
			for i, p := range r.params {
				args[p] = m[i+1]
			}
			return &frError{m: msg.M(r.id, args), err: err}
		}
	}
	if text == raw {
		return err
	}
	return &frError{text: text, err: err}
}
