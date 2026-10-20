package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/lalamefine/lattis/internal/msg"
)

// Langue de la session : transmise par l'interface, elle traduit les
// *msg.Error rendus (cause de l'erreur d'appel) ; les autres erreurs
// gardent leur texte.
func TestSessionLanguage(t *testing.T) {
	defer msg.SetCurrent(msg.Current())
	s := &DocumentService{}
	ctx := context.Background()
	if err := s.SetLanguage(ctx, "klingon"); err == nil {
		t.Fatal("langue inconnue acceptée")
	}
	if err := s.SetLanguage(ctx, "de"); err != nil {
		t.Fatal(err)
	}
	if s.sessionLang() != msg.DE || msg.Current() != msg.DE {
		t.Fatalf("langue : %s, bureau %s", s.sessionLang(), msg.Current())
	}
	// Texte allemand de l'erreur enveloppée ; l'identifiant et les paramètres suivent.
	err := fmt.Errorf("renommer : %w", msg.New("table.nameTaken", msg.Args{"name": "Ventes"}))
	var c ErrorCause
	if b := MarshalError(err); b == nil || json.Unmarshal(b, &c) != nil || c.ID != "table.nameTaken" || c.Args["name"] != "Ventes" || c.Message != msg.Text(msg.DE, err) || c.Message == err.Error() {
		t.Fatalf("cause : %s", MarshalError(err))
	}
	if MarshalError(errors.New("brut")) != nil {
		t.Fatal("erreur ordinaire : traitement par défaut de Wails attendu")
	}
	text, cause := ErrorTranslator(s)(errNoDoc)
	if text != msg.Format(msg.DE, "doc.none", nil) || text == "aucun classeur ouvert" || cause.(ErrorCause).ID != "doc.none" {
		t.Fatalf("web : %q, %v", text, cause)
	}
	if text, cause := ErrorTranslator(s)(errors.New("brut")); text != "brut" || cause != nil {
		t.Fatalf("web, erreur ordinaire : %q, %v", text, cause)
	}
}

// Le bureau donne la langue du système pour le réglage « auto ».
func TestPlatformLocale(t *testing.T) {
	s := &DocumentService{}
	if p := s.Platform(); p.Web || p.Locale != msg.SystemLocale() {
		t.Fatalf("bureau : %+v", p)
	}
	UseHost(webLike{}, nil, s)
	if p := s.Platform(); p.Locale != "" {
		t.Fatalf("web : %+v", p)
	}
}
