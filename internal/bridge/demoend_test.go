package bridge

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/msg"
)

func TestDemoGuard(t *testing.T) {
	now := time.Date(2026, 10, 21, 14, 0, 0, 0, time.UTC)
	demoNow = func() time.Time { return now }
	t.Cleanup(func() {
		demoNow = time.Now
		SetDemoDeadline(time.Time{}, 0)
	})
	ended := func(err error) bool {
		var m msg.Messager
		return errors.As(err, &m) && m.Msg().ID == "demo.ended"
	}

	// Sans fin annoncée : tout est permis, Platform n'en dit rien.
	if DemoEnded() || DemoGuard("DocumentService.SetCell") != nil {
		t.Fatal("sans fin annoncée : modification refusée")
	}
	var p Platform
	demoPlatform(&p)
	if p.DemoDeadline != 0 || p.DemoWarnBefore != 0 || p.Now != 0 {
		t.Fatalf("Platform sans fin = %+v", p)
	}

	// Avant l'heure : permis ; Platform donne la fin, le préavis et l'heure.
	SetDemoDeadline(now.Add(90*time.Second), time.Minute)
	if DemoEnded() || DemoGuard("DocumentService.SetCell") != nil {
		t.Fatal("avant la fin : modification refusée")
	}
	demoPlatform(&p)
	if p.DemoDeadline != now.Add(90*time.Second).UnixMilli() || p.DemoWarnBefore != 60000 || p.Now != now.UnixMilli() {
		t.Fatalf("Platform = %+v", p)
	}

	// À l'heure : modifications refusées, lecture et téléchargement permis.
	now = now.Add(90 * time.Second)
	if !DemoEnded() {
		t.Fatal("fin passée non vue")
	}
	for _, m := range []string{"DocumentService.SetCell", "DocumentService.AddSheet", "DocumentService.Paste", "DocumentService.SetZoom", "GridService.SetCell", "DocumentService.Inconnue"} {
		if !ended(DemoGuard(m)) {
			t.Errorf("%s permis après la fin", m)
		}
	}
	for _, m := range []string{"DocumentService.Save", "DocumentService.Download", "DocumentService.State", "DocumentService.Layout", "DocumentService.Platform", "GridService.Window"} {
		if err := DemoGuard(m); err != nil {
			t.Errorf("%s refusé après la fin : %v", m, err)
		}
	}
}

// Les méthodes permises après la fin de la démo existent (une faute de
// frappe laisserait la méthode voulue refusée).
func TestDemoAfterEndMethods(t *testing.T) {
	svcs := map[string]reflect.Type{
		"DocumentService": reflect.TypeFor[*DocumentService](),
		"GridService":     reflect.TypeFor[*GridService](),
	}
	for name := range demoAfterEnd {
		svc, m, _ := strings.Cut(name, ".")
		if _, ok := svcs[svc].MethodByName(m); !ok {
			t.Errorf("%s : méthode inconnue", name)
		}
	}
}
