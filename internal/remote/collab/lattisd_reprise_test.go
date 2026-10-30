package collab_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Coupures et reprises de bout en bout, contre lattisd et PostgreSQL
// (LATTIS_LATTISD_URL, voir lattisd_test.go) : le réseau du poste de Bob
// est coupé pour de bon (connexions fermées, nouvelles refusées) pendant
// qu'Alice continue de travailler.

// ldPair : Alice (propriétaire) et Bob (éditeur, réseau coupable) suivent
// un classeur publié par Alice.
type ldPair struct {
	e          *ldEnv
	alice, bob *station
	wb         string
	// offline : passages hors ligne de Bob vus par le suivi.
	offline atomic.Int32
	mu      sync.Mutex
	notices []string
}

func ldPairSetup(t *testing.T, bobTTL time.Duration) (*ldPair, context.Context) {
	t.Helper()
	old := collab.Debounce
	collab.Debounce = 20 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	e := ldSetup(t)
	p := &ldPair{e: e}
	ar, ai := e.user(t, "alice")
	br, bi := e.user(t, "bob")
	p.alice = e.station(t, "alice", ar, ai, stationOpts{})
	p.wb = ldPublish(t, e, p.alice, 5)
	e.exec(t, `SELECT lattis.add_member($1::uuid, 'editor', $2::uuid)`, p.wb, bi)
	p.bob = e.station(t, "bob", br, bi, stationOpts{cuttable: true, ttl: bobTTL})
	p.bob.open(t, e, p.wb)
	ctx, cancel := context.WithCancel(context.Background())
	p.alice.follow(ctx, p.alice.fsrv, 50*time.Millisecond, nil)
	p.bob.follow(ctx, p.bob.fsrv, 50*time.Millisecond, func(ev collab.Event) {
		if ev.State && !p.bob.d.Share.Online() {
			p.offline.Add(1)
		}
		if len(ev.Notices) > 0 {
			p.mu.Lock()
			p.notices = append(p.notices, ev.Notices...)
			p.mu.Unlock()
		}
	})
	// Après les follow : le suivi s'arrête avant leurs attentes de fin.
	t.Cleanup(cancel)
	waitOnline(t, p.alice.client, p.bob.client)
	return p, ctx
}

func (p *ldPair) allNotices() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return strings.Join(p.notices, " ; ")
}

// colIs attend que la colonne name du poste c vaille want.
func colIs(t *testing.T, c *client, name string, want ...string) {
	t.Helper()
	waitFor(t, c.name+" "+name+" = "+strings.Join(want, ","), func() bool {
		got := c.col(name)
		if len(got) < len(want) {
			return false
		}
		for i := range want {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	})
}

// pending : modifications de c pas encore parties.
func pending(c *client) int {
	n, _, _, _, err := c.e.Counts(context.Background())
	if err != nil {
		return -1
	}
	return n
}

// TestLattisdRelayFollow : deux postes suivent par le relais de lattisd ;
// une saisie d'Alice arrive chez Bob, et inversement.
func TestLattisdRelayFollow(t *testing.T) {
	p, _ := ldPairSetup(t, 0)
	p.alice.mustSet(0, 1, "42")
	p.alice.e.Kick()
	colIs(t, p.bob.client, "Quantité", "42")
	p.bob.mustSet(1, 1, "43")
	p.bob.e.Kick()
	colIs(t, p.alice.client, "Quantité", "42", "43")
}

// TestLattisdLongOffline : coupure plus longue que la durée du jeton
// (jeton gardé périmé, refusé par PostgreSQL et par le relais) : à la
// reprise, le jeton est renouvelé, la file part, ce qu'Alice a fait
// entre-temps arrive.
func TestLattisdLongOffline(t *testing.T) {
	p, _ := ldPairSetup(t, 0)
	p.bob.cut()
	waitFor(t, "Bob hors ligne", func() bool { return !p.bob.d.Share.Online() })
	p.bob.mustSet(0, 1, "70")
	p.alice.mustSet(1, 1, "71")
	p.alice.e.Kick()
	waitFor(t, "file d'Alice partie", func() bool { return pending(p.alice.client) == 0 })
	if pending(p.bob.client) != 1 {
		t.Fatalf("file de Bob hors ligne : %d", pending(p.bob.client))
	}
	mints := p.bob.tok.mints.Load()
	p.bob.tok.stale()
	p.bob.mend()
	colIs(t, p.alice.client, "Quantité", "70", "71")
	colIs(t, p.bob.client, "Quantité", "70", "71")
	waitFor(t, "Bob en ligne", func() bool { return p.bob.d.Share.Online() })
	if p.bob.tok.mints.Load() == mints {
		t.Fatal("jeton périmé jamais renouvelé")
	}
}

// TestLattisdTokenExpiresWhileFollowing : lattisd ferme le flux à
// l'expiration du jeton (« expired ») ; le poste se réabonne avec un jeton
// neuf sans passer hors ligne, et ne manque rien.
func TestLattisdTokenExpiresWhileFollowing(t *testing.T) {
	p, _ := ldPairSetup(t, 3*time.Second)
	before := p.offline.Load()
	time.Sleep(7 * time.Second) // deux expirations au moins
	p.alice.mustSet(2, 1, "80")
	p.alice.e.Kick()
	colIs(t, p.bob.client, "Quantité", "0", "1", "80")
	if n := p.offline.Load() - before; n != 0 {
		t.Fatalf("Bob passé %d fois hors ligne à l'expiration du jeton du flux", n)
	}
	if p.bob.tok.mints.Load() < 3 {
		t.Fatalf("jetons : %d", p.bob.tok.mints.Load())
	}
}

// TestLattisdStructureChangedMeanwhile : Bob saisit hors ligne dans Prix
// et Quantité ; Alice renomme Prix en Tarif et supprime Quantité. À la
// reprise, Bob n'est pas bloqué : sa valeur de Prix arrive dans Tarif,
// celle de Quantité est écartée (avec un message), la file est vide.
func TestLattisdStructureChangedMeanwhile(t *testing.T) {
	p, ctx := ldPairSetup(t, 0)
	p.bob.cut()
	waitFor(t, "Bob hors ligne", func() bool { return !p.bob.d.Share.Online() })
	p.bob.mustSet(0, 2, "9,5")
	p.bob.mustSet(1, 1, "55")
	must(t, p.alice.header(2, "Tarif"))
	must(t, p.alice.d.DeleteColumn(ctx, "Ventes", 1))
	p.alice.e.Kick()
	waitFor(t, "file d'Alice partie", func() bool { return pending(p.alice.client) == 0 })
	p.bob.mend()
	waitFor(t, "file de Bob partie", func() bool { return pending(p.bob.client) == 0 })
	waitFor(t, "structure reçue par Bob", func() bool {
		return strings.Join(colsOf(p.bob.client, "Ventes"), ",") == "Produit:text,Tarif:decimal"
	})
	colIs(t, p.alice.client, "Tarif", "9.5")
	colIs(t, p.bob.client, "Tarif", "9.5")
	waitFor(t, "Bob en ligne", func() bool { return p.bob.d.Share.Online() })
}

// TestLattisdDeletedMeanwhile : le classeur est supprimé pendant la
// coupure de Bob. À la reprise : supprimé noté, suivi arrêté sans nouvel
// essai, la file reste dans le classeur.
func TestLattisdDeletedMeanwhile(t *testing.T) {
	p, ctx := ldPairSetup(t, 0)
	p.bob.cut()
	waitFor(t, "Bob hors ligne", func() bool { return !p.bob.d.Share.Online() })
	p.bob.mustSet(0, 1, "90")
	must(t, p.alice.srv.RemoveWorkbook(ctx, p.wb))
	p.bob.mend()
	waitGone(t, p.bob.client, workbook.GoneDeleted)
	if pending(p.bob.client) != 1 {
		t.Fatalf("file de Bob : %d", pending(p.bob.client))
	}
	if st := p.bob.d.Share.State(); st.Offline != "" {
		t.Fatalf("état : %+v", st)
	}
}

// TestLattisdRightsReducedMeanwhile : Bob passe de éditeur à lecteur
// pendant sa coupure. À la reprise, sa saisie hors ligne est refusée
// (message, valeur du serveur remise), le classeur reste suivi en ligne.
func TestLattisdRightsReducedMeanwhile(t *testing.T) {
	p, ctx := ldPairSetup(t, 0)
	p.bob.cut()
	waitFor(t, "Bob hors ligne", func() bool { return !p.bob.d.Share.Online() })
	p.bob.mustSet(0, 1, "91")
	as := p.alice.srv
	must(t, as.RevokeRole(ctx, roleID(t, as, p.wb, "editor"), p.bob.id))
	must(t, as.GrantRole(ctx, roleID(t, as, p.wb, "reader"), p.bob.id))
	p.bob.mend()
	waitFor(t, "file de Bob vidée", func() bool { return pending(p.bob.client) == 0 })
	colIs(t, p.bob.client, "Quantité", "0")
	colIs(t, p.alice.client, "Quantité", "0")
	waitFor(t, "Bob en ligne", func() bool { return p.bob.d.Share.Online() })
	if p.bob.d.Share.Gone() != "" {
		t.Fatalf("état : %+v", p.bob.d.Share.State())
	}
	waitFor(t, "message des droits changés", func() bool { return strings.Contains(p.allNotices(), "droits") })
	if st := p.bob.d.Share.State(); len(st.Roles) != 1 || st.Roles[0] != "reader" {
		t.Fatalf("rôles : %+v", st)
	}
}

// TestLattisdAccessRemovedMeanwhile : Bob perd tout accès pendant sa
// coupure : accès retiré noté (ni hors ligne, ni boucle), file gardée.
func TestLattisdAccessRemovedMeanwhile(t *testing.T) {
	p, ctx := ldPairSetup(t, 0)
	p.bob.cut()
	waitFor(t, "Bob hors ligne", func() bool { return !p.bob.d.Share.Online() })
	p.bob.mustSet(0, 1, "92")
	as := p.alice.srv
	must(t, as.RevokeRole(ctx, roleID(t, as, p.wb, "editor"), p.bob.id))
	p.bob.mend()
	waitGone(t, p.bob.client, workbook.GoneRevoked)
	if pending(p.bob.client) != 1 {
		t.Fatalf("file de Bob : %d", pending(p.bob.client))
	}
}
