package bridge

import (
	"context"
	"errors"
	"testing"
)

// webLike est un hôte web sans scripting.
type webLike struct{ NoHost }

func (webLike) Platform() Platform { return Platform{Web: true} }

func TestScriptingOptIn(t *testing.T) {
	s := &DocumentService{}
	UseHost(webLike{}, nil, s)
	if p := s.Platform(); !p.Web || p.Scripting {
		t.Fatalf("Platform = %+v", p)
	}
	ctx := context.Background()
	if _, err := s.RunScript(ctx, "x", "print(1)"); !errors.Is(err, errNoScripting) {
		t.Fatalf("RunScript sans scripting : %v", err)
	}
	if _, err := s.AddPage(ctx, 0); !errors.Is(err, errNoScripting) {
		t.Fatalf("AddPage sans scripting : %v", err)
	}
	if _, err := s.SetMCPEnabled(true); !errors.Is(err, errNoScripting) {
		t.Fatalf("SetMCPEnabled sans scripting : %v", err)
	}
	if s.listening("open") {
		t.Fatal("écouteurs Lua actifs sans scripting")
	}
	if r, err := s.FireControl(ctx, 1, "click", "", ""); err != nil || r.Error != "" {
		t.Fatalf("FireControl sans scripting : %+v, %v", r, err)
	}
}

func TestWithExt(t *testing.T) {
	for in, want := range map[string]string{"a": "a.csv", "a.CSV": "a.CSV", "": ""} {
		if got := withExt(in, ".csv"); got != want {
			t.Errorf("withExt(%q) = %q", in, got)
		}
	}
}
