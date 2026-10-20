package auth

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// Nom choisi par l'utilisateur : préféré partout (état, messages), gardé
// dans l'index, pas remplacé par une nouvelle lecture de la découverte ;
// vide ou égal au nom du serveur : retour au nom du serveur.
func TestRename(t *testing.T) {
	f := newFakeLattisd(t)
	managed := newFakeLattisd(t)
	store := memStore{}
	index := filepath.Join(t.TempDir(), "serveurs.json")
	m := newManager(t, f, store, index, nil)
	policy := func() Policy {
		return Policy{AllowUserServers: true, Managed: []Managed{{URL: managed.url(), Name: "Géré"}}}
	}
	m.opts.Policy = policy
	ctx := context.Background()
	if _, err := m.Discover(ctx, f.url()); err != nil {
		t.Fatal(err)
	}
	st, err := m.Rename(f.url(), "  Serveur   du labo ")
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "Serveur du labo" || st.Label != "Serveur du labo" || st.ServerName != "Serveur Acme" {
		t.Fatalf("renommé : %+v", st)
	}
	if st, _ := m.Discover(ctx, f.url()); st.Name != "Serveur du labo" || st.ServerName != "Serveur Acme" {
		t.Fatalf("après une nouvelle découverte : %+v", st)
	}
	if m.Label(f.url()+"/") != "Serveur du labo" || m.Label(managed.url()) != "" {
		t.Fatalf("Label = %q", m.Label(f.url()))
	}
	// Messages : le nom choisi.
	if _, _, err := m.Session(ctx, f.url()); err == nil || !strings.Contains(err.Error(), "Serveur du labo") {
		t.Fatalf("connexion demandée : %v", err)
	}
	if _, err := m.Login(ctx, f.url(), "inconnu"); err == nil || !strings.Contains(err.Error(), "Serveur du labo") {
		t.Fatalf("fournisseur absent : %v", err)
	}
	// Serveur de l'organisation, jamais lu : renommable aussi.
	if st, err := m.Rename(managed.url(), "Mon serveur"); err != nil || st.Name != "Mon serveur" || st.ServerName != "Géré" || !st.Managed {
		t.Fatalf("serveur géré : %+v %v", st, err)
	}
	// Index relu : noms choisis gardés, nom du serveur à part.
	m2 := newManager(t, f, store, index, nil)
	m2.opts.Policy = policy
	got := map[string]ServerState{}
	for _, s := range m2.Servers() {
		got[s.URL] = s
	}
	if s := got[f.url()]; s.Name != "Serveur du labo" || s.ServerName != "Serveur Acme" {
		t.Fatalf("index relu : %+v", got)
	}
	if s := got[managed.url()]; s.Name != "Mon serveur" {
		t.Fatalf("index relu (géré) : %+v", got)
	}
	// Trop long : refusé ; vide ou nom du serveur : retour au nom du serveur.
	if _, err := m2.Rename(f.url(), strings.Repeat("é", MaxLabel+1)); err == nil {
		t.Fatal("nom trop long accepté")
	}
	if _, err := m2.Rename(f.url(), strings.Repeat("é", MaxLabel)); err != nil {
		t.Fatal(err)
	}
	if st, _ := m2.Rename(f.url(), " "); st.Name != "Serveur Acme" || st.Label != "" {
		t.Fatalf("nom effacé : %+v", st)
	}
	if st, _ := m2.Rename(managed.url(), "Géré"); st.Label != "" || st.Name != "Géré" {
		t.Fatalf("nom du serveur : %+v", st)
	}
}
