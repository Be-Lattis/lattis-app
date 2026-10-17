package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestControlConfigCheck(t *testing.T) {
	for _, tc := range []struct {
		cfg ButtonConfig
		bad string // fragment de l'erreur attendue ; vide : valide
	}{
		{ButtonConfig{Label: "OK"}, ""},
		{ButtonConfig{}, "libellé vide"},
		{ButtonConfig{Kind: "checkbox", Label: "Fait", Link: "b3"}, ""},
		{ButtonConfig{Kind: "select", Options: []string{" ", ""}, Link: "B3"}, "aucun choix"},
		{ButtonConfig{Kind: "input", Link: "C1"}, ""},
		{ButtonConfig{Kind: "textarea", Link: "C1", Label: "Notes…"}, ""},
		{ButtonConfig{Kind: "file", Link: "A1"}, ""},
		{ButtonConfig{Kind: "file"}, "cellule liée invalide"},
		{ButtonConfig{Kind: "input", Link: "1A"}, "cellule liée invalide"},
		{ButtonConfig{Kind: "radio", Link: "A1"}, "inconnu"},
		{ButtonConfig{Label: "OK", Name: strings.Repeat("x", 65)}, "nom trop long"},
		{ButtonConfig{Label: "OK", Listeners: []Listener{{Event: "Click", Script: "S", Function: "f"}}}, ""},
		{ButtonConfig{Label: "OK", Listeners: []Listener{{Event: "change", Script: "S", Function: "f"}}}, "« click »"},
		{ButtonConfig{Kind: "input", Link: "A1", Listeners: []Listener{{Event: "click", Script: "S", Function: "f"}}}, "« change »"},
		{ButtonConfig{Kind: "input", Link: "A1", Listeners: []Listener{{Event: "change", Script: "S", Function: "a b"}}}, "sans fonction"},
		{ButtonConfig{Kind: "input", Link: "A1", Listeners: []Listener{{Event: "change", Function: "f"}}}, "sans script"},
	} {
		cfg := tc.cfg
		err := cfg.check()
		switch {
		case tc.bad == "" && err != nil:
			t.Errorf("%+v : %v", tc.cfg, err)
		case tc.bad != "" && (err == nil || !strings.Contains(err.Error(), tc.bad)):
			t.Errorf("%+v : erreur %v, attendu « %s »", tc.cfg, err, tc.bad)
		}
	}
	cfg := ButtonConfig{Label: "OK", Listeners: []Listener{{Event: " CLICK ", Script: " S ", Function: " f "}}}
	if err := cfg.check(); err != nil || cfg.Listeners[0] != (Listener{Event: "click", Script: "S", Function: "f"}) {
		t.Fatalf("écouteur normalisé : %+v %v", cfg.Listeners, err)
	}
}

func TestNextControlName(t *testing.T) {
	if n := NextControlName("select", nil); n != "Liste1" {
		t.Fatal(n)
	}
	if n := NextControlName("", []string{"bouton1", "Bouton2", "Liste3"}); n != "Bouton3" {
		t.Fatal(n)
	}
	if n := NextControlName("file", []string{"Fichier2"}); n != "Fichier1" {
		t.Fatal(n)
	}
	if n := NextControlName("textarea", nil); n != "Zone1" {
		t.Fatal(n)
	}
}

func TestControlNames(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	create := func(sheet int64, cfg ButtonConfig) ButtonInfo {
		t.Helper()
		id, err := d.CreateButton(ctx, sheet, 0, 0, cfg)
		if err != nil {
			t.Fatal(err)
		}
		b, err := d.Button(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	b1 := create(1, ButtonConfig{Label: "A"})
	b2 := create(1, ButtonConfig{Label: "B"})
	l1 := create(1, ButtonConfig{Kind: "select", Options: []string{"x"}, Link: "B1"})
	if b1.Config.Name != "Bouton1" || b2.Config.Name != "Bouton2" || l1.Config.Name != "Liste1" || l1.Sheet != 1 {
		t.Fatalf("noms : %q %q %q", b1.Config.Name, b2.Config.Name, l1.Config.Name)
	}
	// Unicité dans le classeur, casse ignorée, à la création comme à la modification.
	if _, err := d.CreateButton(ctx, 1, 0, 0, ButtonConfig{Label: "C", Name: "bouton1"}); !errors.Is(err, ErrButton) || !strings.Contains(err.Error(), "déjà pris") {
		t.Fatalf("nom en double accepté : %v", err)
	}
	cfg := b2.Config
	cfg.Name = "LISTE1"
	if err := d.SetButton(ctx, b2.ID, cfg); err == nil {
		t.Fatal("renommage en double accepté")
	}
	cfg.Name = "Valider"
	if err := d.SetButton(ctx, b2.ID, cfg); err != nil {
		t.Fatal(err)
	}
	// Son propre nom reste permis.
	if err := d.SetButton(ctx, b2.ID, cfg); err != nil {
		t.Fatal(err)
	}
	// Une copie de feuille renomme ses contrôles.
	id, err := d.CopySheet(ctx, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	all, err := d.Controls(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var copies []string
	for _, b := range all {
		if seen[strings.ToLower(b.Config.Name)] {
			t.Fatalf("nom en double après copie : %s", b.Config.Name)
		}
		seen[strings.ToLower(b.Config.Name)] = true
		if b.Sheet == id {
			copies = append(copies, b.Config.Name)
		}
	}
	if strings.Join(copies, ",") != "Bouton2,Bouton3,Liste2" {
		t.Fatalf("contrôles copiés : %v", copies)
	}
}
