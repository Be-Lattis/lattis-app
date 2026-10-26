package query

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGuardAddresses(t *testing.T) {
	g := &Guard{Hosts: []string{"interne.exemple:8080"}, resolve: func(_ context.Context, host string) ([]net.IP, error) {
		switch host {
		case "public.exemple":
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		case "mixte.exemple":
			return []net.IP{net.ParseIP("93.184.216.34"), net.ParseIP("10.0.0.5")}, nil
		}
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}}
	for host, ok := range map[string]bool{
		"169.254.169.254":      false, // métadonnées des nuages
		"127.0.0.1:5432":       false,
		"[::1]:80":             false,
		"10.1.2.3":             false,
		"192.168.1.10":         false,
		"172.16.0.1":           false,
		"100.64.0.1":           false,
		"0.0.0.0":              false,
		"[fe80::1]":            false,
		"localhost":            false,
		"mixte.exemple":        false,
		"public.exemple":       true,
		"93.184.216.34:443":    true,
		"interne.exemple:8080": true, // liste de l'administrateur
		"interne.exemple:22":   false,
	} {
		err := g.Check(ctx, Access{Host: host})
		if (err == nil) != ok || (err != nil && !errors.Is(err, ErrDenied)) {
			t.Errorf("%s : %v", host, err)
		}
	}
}

func TestGuardFiles(t *testing.T) {
	dir := t.TempDir()
	g := &Guard{Dir: dir}
	for p, ok := range map[string]bool{
		filepath.Join(dir, "envoi.csv"):               true,
		filepath.Join(dir, "sous", "envoi.csv"):       true,
		filepath.Join(dir, "..", "ailleurs.csv"):      false,
		filepath.Join(filepath.Dir(dir), "autre.csv"): false,
		"envoi.csv": false, // relatif : répertoire courant du serveur
	} {
		if err := g.CheckPath(p); (err == nil) != ok {
			t.Errorf("%s : %v", p, err)
		}
	}
	if err := (&Guard{}).CheckPath(filepath.Join(dir, "envoi.csv")); err == nil {
		t.Error("sans répertoire : fichier accepté")
	}
}

// Version web : une source ne joint pas une adresse interne du serveur, ni
// directement ni par une résolution qui change entre la vérification et la
// connexion (audit 6, I5) ; une source fichier ne lit que Dir.
func TestGuardedSources(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		fmt.Fprint(w, `[{"a": 1}]`)
	}))
	defer srv.Close()
	d := newDoc(t)
	e := env(d)
	preview := func(g *Guard, src Source) Preview {
		s := newSession(t)
		return s.Preview(WithGuard(ctx, g), e, Query{Name: "Q", Source: src}, -1, true)
	}

	web := Source{Kind: SourceWeb, URL: srv.URL + "/data"}
	if pv := preview(&Guard{}, web); !strings.Contains(pv.Error, "adresse interne") || hits.Load() != 0 {
		t.Fatalf("bouclage : %q, %d requêtes", pv.Error, hits.Load())
	}
	host := strings.TrimPrefix(srv.URL, "http://")
	if pv := preview(&Guard{Hosts: []string{host}}, web); pv.Error != "" || hits.Load() != 1 {
		t.Fatalf("hôte permis : %q", pv.Error)
	}

	// Rebinding : publique à la vérification, bouclage à la connexion.
	_, port, _ := net.SplitHostPort(host)
	var n atomic.Int32
	g := &Guard{resolve: func(context.Context, string) ([]net.IP, error) {
		if n.Add(1) == 1 {
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		}
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}}
	if pv := preview(g, Source{Kind: SourceWeb, URL: "http://changeant.exemple:" + port + "/data"}); !strings.Contains(pv.Error, "adresse interne") || hits.Load() != 1 {
		t.Fatalf("rebinding : %q, %d requêtes", pv.Error, hits.Load())
	}

	dir := t.TempDir()
	inside := writeFile(t, "a.csv", "x\n1\n")
	if pv := preview(&Guard{Dir: dir}, Source{Kind: SourceCSV, Path: inside}); !strings.Contains(pv.Error, "fichiers envoyés") {
		t.Fatalf("fichier hors du répertoire : %q", pv.Error)
	}
	if pv := preview(&Guard{Dir: filepath.Dir(inside)}, Source{Kind: SourceCSV, Path: inside}); pv.Error != "" {
		t.Fatalf("fichier envoyé : %q", pv.Error)
	}
	if _, err := FileTables(WithGuard(ctx, &Guard{Dir: dir}), KindSQLite, inside); !errors.Is(err, ErrDenied) {
		t.Fatalf("navigateur de fichier : %v", err)
	}
}

// Bases de données : le pilote résout le nom à nouveau après Check ; la
// connexion passe par Guard.DialContext, qui refuse l'adresse interne
// obtenue (audit de sécurité du 26/10, M4). Ici, le nom est public pour
// Check (resolve) et « localhost » pour le pilote.
func TestGuardDatabaseRebinding(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var hits atomic.Int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			hits.Add(1)
			c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	g := &Guard{resolve: func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}}
	ctx := WithGuard(context.Background(), g)
	for _, kind := range []string{KindPostgres, KindSQLServer} {
		c := Connection{Name: kind, Kind: kind, Config: DBConfig{Host: "localhost", Port: port, Database: "x", User: "u", SSLMode: "disable"}}
		if _, _, err := connectDB(ctx, NewSecrets(false, ""), c); err == nil || !strings.Contains(err.Error(), "adresse interne") {
			t.Fatalf("%s : %v", kind, err)
		}
		if err := TestConnection(ctx, NewSecrets(false, ""), c, "secret", false); err == nil || !strings.Contains(err.Error(), "adresse interne") {
			t.Fatalf("%s avec mot de passe : %v", kind, err)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("%d connexions vers le bouclage", hits.Load())
	}
}
