package bridge

import (
	"context"
	"testing"
)

func TestSettingsHtmlTabsOffByDefault(t *testing.T) {
	s := &DocumentService{}
	list := s.Settings(context.Background())
	for _, st := range list {
		if st.Path == "features.htmlTabs" {
			if st.Value != false || st.Locked {
				t.Fatalf("features.htmlTabs = %+v", st)
			}
			return
		}
	}
	t.Fatalf("features.htmlTabs absent : %+v", list)
}

func TestSetSettingRefusesUnknownKey(t *testing.T) {
	s := &DocumentService{}
	if _, err := s.SetSetting(context.Background(), "configUrl", "https://exemple.fr/"); err == nil {
		t.Fatal("clé hors de la liste acceptée")
	}
}
