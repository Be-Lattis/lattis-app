package workbook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// PushClient contre un faux lattisd : CSV et jeton envoyés, bilan (200),
// écriture en cours (202), jeton refusé puis rafraîchi (401), refus (409,
// 413, 404 : message du serveur).
func TestPushClient(t *testing.T) {
	var tokens []string
	var body, ctype, wait string
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		tokens = append(tokens, tok)
		if tok == "périmé" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token", "error_description": "jeton refusé"})
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/push/p1") {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found", "error_description": "source poussée inconnue"})
			return
		}
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"pipeline_id":"p1","pending":false,"last_run":{"status":"ok","rows_read":2,"inserted":2,"finished_at":"2026-10-19T10:00:00Z"}}`)
			return
		}
		b, _ := io.ReadAll(r.Body)
		body, ctype, wait = string(b), r.Header.Get("Content-Type"), r.URL.Query().Get("wait")
		switch status {
		case http.StatusOK:
			_, _ = io.WriteString(w, `{"status":"warning","rows_read":2,"inserted":1,"rejected":1,"rejects":[{"row":3,"column":"Prix","value":"x","error":"nombre attendu"}],"trigger":"push"}`)
		case http.StatusAccepted:
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"status":"running"}`)
		default:
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "conflict", "error_description": "un envoi attend déjà d'être traité"})
		}
	}))
	defer srv.Close()
	tok := "périmé"
	c := PushClient{Issuer: srv.URL + "/", Token: func(context.Context) (string, error) { return tok, nil }, Invalidate: func() { tok = "neuf" }}
	ctx := context.Background()

	run, err := c.Push(ctx, "p1", []byte("Produit;Prix\nCafé;4.5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "warning" || run.Rejected != 1 || run.Rejects[0].Column != "Prix" || body != "Produit;Prix\nCafé;4.5\n" || !strings.HasPrefix(ctype, "text/csv") || wait != "60" {
		t.Fatalf("envoi : %+v %q %q %q", run, body, ctype, wait)
	}
	if strings.Join(tokens, ",") != "périmé,neuf" {
		t.Fatalf("jetons : %v", tokens)
	}

	status = http.StatusAccepted
	if run, err = c.Push(ctx, "p1", []byte("a\n")); err != nil || run.Status != "running" {
		t.Fatalf("202 : %+v %v", run, err)
	}
	st, err := c.PushStatus(ctx, "p1")
	if err != nil || st.LastRun == nil || st.LastRun.Inserted != 2 || st.Pending {
		t.Fatalf("état : %+v %v", st, err)
	}

	for code, want := range map[int]string{http.StatusConflict: "réessayez", http.StatusRequestEntityTooLarge: "trop gros"} {
		status = code
		_, err := c.Push(ctx, "p1", []byte("a\n"))
		var pe *PushError
		if !errors.As(err, &pe) || pe.Status != code || !strings.Contains(err.Error(), want) {
			t.Fatalf("%d : %v", code, err)
		}
	}
	if _, err := c.Push(ctx, "inconnue", []byte("a\n")); !PushNotFound(err) || err.Error() != "source poussée inconnue" {
		t.Fatalf("404 : %v", err)
	}
}
