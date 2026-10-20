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

func TestResetSettingsRefusesUnknownKey(t *testing.T) {
	s := &DocumentService{}
	for _, p := range []string{"connections", "ui.themes", "ai.apiKey"} {
		if _, err := s.ResetSettings(context.Background(), []string{"features.htmlTabs", p}); err == nil {
			t.Fatalf("%s accepté", p)
		}
	}
}

func TestSetSettingRefusesUnknownKey(t *testing.T) {
	s := &DocumentService{}
	if _, err := s.SetSetting(context.Background(), "configUrl", "https://exemple.fr/"); err == nil {
		t.Fatal("clé hors de la liste acceptée")
	}
}
