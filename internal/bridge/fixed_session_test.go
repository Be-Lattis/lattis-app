package bridge

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Jetons d'un moteur lancé par lattisd : lus à l'adresse donnée avec le
// secret, gardés jusqu'à l'approche de leur expiration, redemandés après un
// refus ; un autre serveur est refusé.
func TestFixedSession(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer s3cret" {
			http.Error(w, "non", http.StatusUnauthorized)
			return
		}
		calls.Add(1)
		w.Write([]byte(`{"access_token":"tok","pg_role":"lea@acme.fr","expires_in":900,"user_id":"u1","email":"lea@acme.fr","display_name":"Léa Martin"}`))
	}))
	defer srv.Close()
	f := &FixedSession{Server: "https://lattis.acme.fr", Fetch: TokenURL(srv.URL, "s3cret", nil)}
	ctx := context.Background()
	for range 2 {
		if tok, err := f.Token(ctx, "https://lattis.acme.fr/"); err != nil || tok != "tok" {
			t.Fatalf("Token = %q, %v", tok, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("%d demande(s) de jeton, attendu 1", calls.Load())
	}
	f.Invalidate("")
	f.Token(ctx, "https://lattis.acme.fr")
	if calls.Load() != 2 {
		t.Fatal("jeton refusé non redemandé")
	}
	if _, err := f.Token(ctx, "https://autre.fr"); !errors.Is(err, ErrOtherServer) {
		t.Fatalf("autre serveur : %v", err)
	}
	if st := f.state(); !st.Enabled || len(st.Servers) != 1 || st.Servers[0].User == nil || st.Servers[0].User.PGRole != "lea@acme.fr" || st.Servers[0].User.DisplayName != "Léa Martin" || st.Servers[0].User.Email != "lea@acme.fr" || st.Servers[0].User.ID != "u1" {
		t.Fatalf("état : %+v", st)
	}
	bad := &FixedSession{Server: "https://lattis.acme.fr", Fetch: TokenURL(srv.URL, "faux", nil)}
	if _, err := bad.Token(ctx, "https://lattis.acme.fr"); err == nil {
		t.Fatal("secret refusé accepté")
	}
}
