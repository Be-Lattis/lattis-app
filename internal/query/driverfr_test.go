package query

import (
	"errors"
	"strings"
	"testing"
)

func TestDriverFrench(t *testing.T) {
	cases := []struct{ in, want string }{
		{`dial tcp: lookup db.exemple.fr: no such host`, "serveur introuvable"},
		{`dial tcp 10.0.0.1:5432: connect: connection refused`, "le serveur refuse la connexion"},
		{`dial tcp 10.0.0.1:5432: i/o timeout`, "serveur injoignable"},
		{`failed to connect to host=x user=u database=ventes: server error (FATAL: database "ventes" does not exist (SQLSTATE 3D000))`, "la base « ventes » n'existe pas"},
		{`Error 1049 (42000): Unknown database 'compta'`, "la base « compta » n'existe pas"},
		{`mssql: login error: Cannot open database "RH" requested by the login. The login failed.`, "la base « RH » n'existe pas"},
		{`tls: failed to verify certificate: x509: certificate signed by unknown authority`, "échec de la connexion chiffrée"},
	}
	for _, c := range cases {
		src := errors.New(c.in)
		got := driverFrench(src)
		if !strings.HasPrefix(got.Error(), c.want) || !strings.Contains(got.Error(), c.in) {
			t.Errorf("%q : %q", c.in, got)
		}
		if !errors.Is(got, src) {
			t.Errorf("%q : l'erreur d'origine doit rester accessible", c.in)
		}
	}
	other := errors.New("quelque chose d'autre")
	if driverFrench(other) != other {
		t.Error("une erreur inconnue doit rester telle quelle")
	}
	if driverFrench(nil) != nil {
		t.Error("nil doit rester nil")
	}
}
