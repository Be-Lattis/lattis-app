package query

import (
	"github.com/lalamefine/lattis/internal/msg"
	"regexp"
	"strings"
)

// Erreurs courantes des pilotes PostgreSQL, MySQL et SQL Server, en
// français (les messages de SQLite le sont par sqltext.French). Le refus
// d'authentification a son propre chemin (ErrPassword).
var driverRules = []struct {
	re *regexp.Regexp
	id msg.ID
}{
	{regexp.MustCompile(`(?i)no such host|server misbehaving|name resolution`), "connection.driver.hostNotFound"},
	{regexp.MustCompile(`(?i)connection refused|actively refused|connexion refusée`), "connection.driver.refused"},
	{regexp.MustCompile(`(?i)i/o timeout|deadline exceeded|timed out|network is unreachable|no route to host`), "connection.driver.unreachable"},
	{regexp.MustCompile(`(?i)database "([^"]+)" does not exist`), "connection.driver.noDatabase"},
	{regexp.MustCompile(`(?i)unknown database '([^']+)'`), "connection.driver.noDatabase"},
	{regexp.MustCompile(`(?i)cannot open database "([^"]+)"`), "connection.driver.cannotOpenDatabase"},
	{regexp.MustCompile(`(?i)\b(?:tls|ssl|x509)\b|certificate`), "connection.driver.tls"},
	{regexp.MustCompile(`(?i)too many connections|remaining connection slots`), "connection.driver.tooMany"},
}

// driverFrench rend lisible une erreur de connexion d'un pilote (message
// traduisible) ; le texte d'origine reste entre parenthèses pour le
// support. Une erreur inconnue garde son texte.
func driverFrench(err error) error {
	if err == nil {
		return nil
	}
	text := err.Error()
	for _, r := range driverRules {
		if m := r.re.FindStringSubmatch(text); m != nil {
			args := msg.Args{"detail": strings.TrimSpace(text)}
			if len(m) > 1 {
				args["database"] = m[1]
			}
			return msg.Wrap(err, r.id, args)
		}
	}
	return err
}
