package bridge

import (
	"context"
	"strings"
	"testing"
)

// Pages web : aucune case dans les paramètres (réglage à la main,
// features.htmlPages ; l'ancien features.htmlTabs n'y est plus).
func TestSettingsWithoutHtmlPages(t *testing.T) {
	s := &DocumentService{}
	for _, st := range s.Settings(context.Background()) {
		if strings.HasPrefix(strings.ToLower(st.Path), "features.html") {
			t.Fatalf("réglage des pages web dans les paramètres : %+v", st)
		}
	}
	if _, err := s.SetSetting(context.Background(), "features.htmlPages", true); err == nil {
		t.Fatal("features.htmlPages modifiable depuis les paramètres")
	}
}

func TestResetSettingsRefusesUnknownKey(t *testing.T) {
	s := &DocumentService{}
	for _, p := range []string{"connections", "ui.themes", "ai.apiKey"} {
		if _, err := s.ResetSettings(context.Background(), []string{"features.coEditing", p}); err == nil {
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
