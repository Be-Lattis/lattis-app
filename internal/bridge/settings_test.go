package bridge

import (
	"context"
	"testing"
)

func TestSettingsWebPagesOffByDefault(t *testing.T) {
	s := &DocumentService{}
	list := s.Settings(context.Background())
	for _, st := range list {
		if st.Path == "features.webPages" {
			if st.Value != false || st.Locked {
				t.Fatalf("features.webPages = %+v", st)
			}
			return
		}
	}
	t.Fatalf("features.webPages absent : %+v", list)
}

func TestSetSettingRefusesUnknownKey(t *testing.T) {
	s := &DocumentService{}
	if _, err := s.SetSetting(context.Background(), "configUrl", "https://exemple.fr/"); err == nil {
		t.Fatal("clé hors de la liste acceptée")
	}
}
