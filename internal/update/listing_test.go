package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// GitHub répond 504 à une liste trop longue (notes de version longues) :
// la recherche redemande moins de versions au lieu d'échouer.
func TestReleasesRetriesSmallerOn5xx(t *testing.T) {
	var asked []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := r.URL.Query().Get("per_page")
		asked = append(asked, n)
		if n != "3" {
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}
		_ = json.NewEncoder(w).Encode([]Release{{Tag: "v0.5.5-beta", Prerelease: true}})
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), API: srv.URL, Version: "0.5.4-beta"}
	list, err := c.Releases(context.Background())
	if err != nil || len(list) != 1 || list[0].Tag != "v0.5.5-beta" {
		t.Fatalf("Releases = %v, %v", list, err)
	}
	if strings.Join(asked, ",") != "10,3" {
		t.Fatalf("per_page demandés : %v", asked)
	}
}

// Une erreur qui n'est pas du serveur (404, limite) n'est pas réessayée.
func TestReleasesNoRetryOn4xx(t *testing.T) {
	hits := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), API: srv.URL, Version: "0.5.4-beta"}
	if _, err := c.Releases(context.Background()); err == nil || hits != 1 {
		t.Fatalf("err = %v, appels = %d", err, hits)
	}
}
