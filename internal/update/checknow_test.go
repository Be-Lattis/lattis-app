package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/msg"
)

func releaseHits(gh *fakeGitHub) int {
	gh.mu.Lock()
	defer gh.mu.Unlock()
	return gh.hits["/repos/"+Repo+"/releases"]
}

// « Rechercher maintenant » part même si la précédente recherche date de
// moins de 12 h, et rend l'état avec la nouvelle version.
func TestCheckNowIgnoresDelay(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.publish("v0.3.3-beta", true, true, map[string][]byte{AssetSetup: []byte("x")})
	u := newTestUpdater(t, gh, "0.3.2-beta", KindUser)
	u.Init()
	u.mu.Lock()
	u.st.LastCheck = time.Now().UTC().Add(-2 * time.Hour)
	u.mu.Unlock()
	if u.due() {
		t.Fatal("recherche de moins de 12 h : pas due")
	}
	s := u.CheckNow(context.Background(), msg.FR)
	if releaseHits(gh) != 1 {
		t.Fatalf("CheckNow n'a pas cherché")
	}
	if s.Phase != PhaseAvailable || s.Version != "0.3.3-beta" || s.Current != "0.3.2-beta" || s.LastError != "" || s.Checking || s.Unavailable != "" {
		t.Errorf("état : %+v", s)
	}
	if last, err := time.Parse(time.RFC3339, s.LastCheck); err != nil || time.Since(last) > time.Minute {
		t.Errorf("dernière recherche : %q", s.LastCheck)
	}
}

// Un échec est gardé dans l'état (langue de la session) sans repousser la
// recherche suivante ; une recherche réussie l'efface.
func TestCheckErrorKept(t *testing.T) {
	limited := true
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limited {
			w.Header().Set("X-RateLimit-Remaining", "0")
			http.Error(w, "rate limit", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()
	u := &Updater{Current: "0.3.2-beta", Kind: KindUser, Client: &Client{HTTP: srv.Client(), API: srv.URL}}
	u.Init()
	s := u.CheckNow(context.Background(), msg.FR)
	if s.LastError != msg.Format(msg.FR, "update.rateLimited", nil) || s.LastCheck == "" {
		t.Errorf("échec non gardé : %+v", s)
	}
	if en := u.Status(msg.EN); en.LastError != msg.Format(msg.EN, "update.rateLimited", nil) {
		t.Errorf("échec en anglais : %q", en.LastError)
	}
	if !u.due() {
		t.Error("un échec ne repousse pas la recherche suivante")
	}
	limited = false
	if s := u.CheckNow(context.Background(), msg.FR); s.LastError != "" || s.Phase != PhaseIdle {
		t.Errorf("après une recherche réussie : %+v", s)
	}
}

// Serveur injoignable (hors ligne, proxy) : un message simple, traduit.
func TestCheckUnreachable(t *testing.T) {
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	api := srv.URL
	srv.Close()
	u := &Updater{Current: "0.3.2-beta", Kind: KindUser, Client: &Client{API: api}}
	u.Init()
	if s := u.CheckNow(context.Background(), msg.EN); s.LastError != msg.Format(msg.EN, "update.unreachable", nil) {
		t.Errorf("injoignable : %q", s.LastError)
	}
}

// Recherche impossible : la raison est dans l'état, rien n'est demandé.
func TestCheckNowUnavailable(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.publish("v0.3.3-beta", true, true, map[string][]byte{AssetSetup: []byte("x")})
	off := func() bool { return false }
	yes := func() bool { return true }
	cases := []struct {
		name    string
		current string
		kind    Kind
		enabled func() bool
		locked  func() bool
		want    Unavailable
	}{
		{"dev", "dev", KindPortable, nil, nil, UnavailableDev},
		{"msix", "0.3.2-beta", KindMSIX, nil, nil, UnavailableMSIX},
		{"stratégie", "0.3.2-beta", KindUser, off, yes, UnavailablePolicy},
		{"désactivée", "0.3.2-beta", KindUser, off, nil, UnavailableOff},
		{"possible", "0.3.2-beta", KindUser, yes, off, ""},
	}
	for _, c := range cases {
		u := newTestUpdater(t, gh, c.current, c.kind)
		u.Enabled, u.Locked = c.enabled, c.locked
		u.Init()
		before := releaseHits(gh)
		s := u.CheckNow(context.Background(), msg.FR)
		if s.Unavailable != c.want || s.Current != c.current {
			t.Errorf("%s : %+v", c.name, s)
		}
		if searched := releaseHits(gh) > before; searched != (c.want == "") {
			t.Errorf("%s : recherche faite = %v", c.name, searched)
		}
	}
}
