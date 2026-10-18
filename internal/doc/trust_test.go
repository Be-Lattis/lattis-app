package doc

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/trust"
)

// otherMachine simule un autre poste : un magasin d'accords vide.
func otherMachine(t *testing.T, legacy ...string) *trust.Store {
	t.Helper()
	s := trust.New(filepath.Join(t.TempDir(), "accords.json"), func() []string { return legacy })
	t.Cleanup(trust.Use(s))
	return s
}

// Les accords réseau sont rangés sur le poste, par classeur : un classeur
// reçu (autre poste) n'en a aucun, et les hôtes qu'une version précédente
// enregistrait dans le fichier ne sont pas repris.
func TestHostsStayOnMachine(t *testing.T) {
	useRoot(t)
	otherMachine(t)
	ctx := context.Background()
	d, err := New(ctx)
	must(t, err)
	must(t, d.AllowHTTPHost(ctx, "api.example.com"))
	if dirty, _ := d.Dirty(ctx); dirty {
		t.Fatal("l'accord modifie le classeur")
	}
	// Accord enregistré dans le fichier par une version précédente (ou par
	// l'auteur d'un classeur piégé).
	must(t, d.DB().Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO _lattis_http_hosts (host) VALUES ('pirate.example.com')`)
		return err
	}))
	path := filepath.Join(t.TempDir(), "reseau.lattis")
	must(t, d.Save(ctx, path))
	must(t, d.Close())

	d, err = Open(ctx, path)
	must(t, err)
	hosts, _ := d.HTTPHosts(ctx)
	if len(hosts) != 1 || hosts[0] != "api.example.com" {
		t.Fatalf("même poste : %v", hosts)
	}
	if ok, _, _ := d.HTTPHostAllowed(ctx, "pirate.example.com"); ok {
		t.Fatal("hôte du fichier accordé")
	}
	must(t, d.Close())

	// Reçu sur un autre poste : rien n'est accordé.
	otherMachine(t)
	d, err = Open(ctx, path)
	must(t, err)
	hosts, _ = d.HTTPHosts(ctx)
	if len(hosts) != 0 {
		t.Fatalf("classeur reçu : %v", hosts)
	}
	must(t, d.Close())

	// Classeur de ce poste d'avant la mise à jour (récent) : ses hôtes et
	// ses scripts sont repris une fois.
	otherMachine(t, path)
	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	if ok, _, _ := d.HTTPHostAllowed(ctx, "pirate.example.com"); !ok {
		t.Fatal("hôtes d'un ancien classeur de ce poste non repris")
	}
}

// Un script enregistré sur ce poste y est autorisé ; le même classeur ouvert
// ailleurs ne l'est pas, sauf « autoriser tous les scripts du classeur ».
func TestScriptVersionsAllowed(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	here := otherMachine(t)
	d, err := New(ctx)
	must(t, err)
	_, err = d.SaveScript(ctx, 0, "Calcul", `print(1)`)
	must(t, err)
	if !here.ScriptAllowed(trust.Hash(`print(1)`)) {
		t.Fatal("script écrit ici non autorisé")
	}
	path := filepath.Join(t.TempDir(), "scripts.lattis")
	must(t, d.Save(ctx, path))
	must(t, d.Close())

	there := otherMachine(t)
	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	if there.ScriptAllowed(trust.Hash(`print(1)`)) {
		t.Fatal("script reçu autorisé d'office")
	}
	must(t, d.AllowAllScripts(ctx))
	if !there.ScriptAllowed(trust.Hash(`print(1)`)) {
		t.Fatal("AllowAllScripts sans effet")
	}
}
