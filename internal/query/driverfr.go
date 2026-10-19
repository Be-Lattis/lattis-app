package query

import (
	"regexp"
	"strings"
)

// Erreurs courantes des pilotes PostgreSQL, MySQL et SQL Server, en
// français (les messages de SQLite le sont par sqltext.French). Le refus
// d'authentification a son propre chemin (ErrPassword).
var driverRules = []struct {
	re  *regexp.Regexp
	out string
}{
	{regexp.MustCompile(`(?i)no such host|server misbehaving|name resolution`), "serveur introuvable : vérifiez le nom du serveur"},
	{regexp.MustCompile(`(?i)connection refused|actively refused|connexion refusée`), "le serveur refuse la connexion : vérifiez le port, et que la base accepte les connexions de ce poste"},
	{regexp.MustCompile(`(?i)i/o timeout|deadline exceeded|timed out|network is unreachable|no route to host`), "serveur injoignable : délai dépassé (réseau, pare-feu ou VPN)"},
	{regexp.MustCompile(`(?i)database "([^"]+)" does not exist`), "la base « $1 » n'existe pas sur ce serveur"},
	{regexp.MustCompile(`(?i)unknown database '([^']+)'`), "la base « $1 » n'existe pas sur ce serveur"},
	{regexp.MustCompile(`(?i)cannot open database "([^"]+)"`), "la base « $1 » n'existe pas sur ce serveur, ou ce compte n'y a pas accès"},
	{regexp.MustCompile(`(?i)\b(?:tls|ssl|x509)\b|certificate`), "échec de la connexion chiffrée (TLS) : vérifiez le mode SSL de la connexion et le certificat du serveur"},
	{regexp.MustCompile(`(?i)too many connections|remaining connection slots`), "le serveur a atteint son nombre maximal de connexions : réessayez plus tard"},
}

// driverFrench traduit le message d'une erreur de connexion d'un pilote ;
// le texte d'origine reste entre parenthèses pour le support. Une erreur
// inconnue garde son texte.
func driverFrench(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	for _, r := range driverRules {
		if m := r.re.FindStringSubmatchIndex(msg); m != nil {
			out := string(r.re.ExpandString(nil, r.out, msg, m))
			return &driverError{msg: out + " (" + strings.TrimSpace(msg) + ")", err: err}
		}
	}
	return err
}

type driverError struct {
	msg string
	err error
}

func (e *driverError) Error() string { return e.msg }
func (e *driverError) Unwrap() error { return e.err }
