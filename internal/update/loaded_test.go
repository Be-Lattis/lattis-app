package update

import (
	"context"
	"testing"
	"time"
)

// Interface chargée : la recherche part aussitôt, même si la précédente
// date de moins de 12 h, sans attendre le délai de secours.
func TestLoadedChecksAtOnce(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.publish("v0.3.0-beta", true, true, map[string][]byte{AssetSetup: []byte("x")})
	u := newTestUpdater(t, gh, "0.2.0-beta", KindUser)
	u.Init()
	u.mu.Lock()
	u.st.LastCheck = time.Now().UTC() // cherché il y a un instant
	u.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go u.Run(ctx, time.Hour)
	u.Loaded()
	u.Loaded() // deux fois : sans effet
	deadline := time.After(5 * time.Second)
	for {
		gh.mu.Lock()
		n := gh.hits["/repos/"+Repo+"/releases"]
		gh.mu.Unlock()
		if n == 1 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("pas de recherche après Loaded (%d)", n)
		case <-time.After(20 * time.Millisecond):
		}
	}
}
