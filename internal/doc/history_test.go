package doc

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/archive"
)

// L'historique d'annulation et les corbeilles ne partent jamais dans le
// fichier : une colonne supprimée et des lignes effacées par une requête SQL
// ne se relisent pas en dézippant le classeur. Dans la session, Ctrl+Z
// marche encore après l'enregistrement.
func TestHistoryNotSaved(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "rh.lattis")
	d, err := New(ctx)
	must(t, err)
	set(t, d, 0, 0, "Nom")
	set(t, d, 0, 1, "Salaire")
	set(t, d, 0, 2, "Ville")
	set(t, d, 1, 0, "Alice")
	set(t, d, 1, 1, "SalaireSecret4711")
	set(t, d, 1, 2, "Lyon")
	set(t, d, 2, 0, "LigneEffacee0815")
	set(t, d, 2, 1, "1")
	set(t, d, 2, 2, "Paris")
	l, err := d.Layout(ctx, 1)
	must(t, err)
	if len(l.Tables) != 1 {
		t.Fatalf("tableau attendu : %+v", l.Tables)
	}
	name := l.Tables[0].Name
	must(t, d.DeleteColumn(ctx, name, 1))
	if _, err := d.ExecSQL(ctx, `DELETE FROM `+name+` WHERE Nom = 'LigneEffacee0815'`); err != nil {
		t.Fatal(err)
	}
	must(t, d.Save(ctx, path))
	saved := view(t, d)
	if strings.Contains(saved, "Secret") || strings.Contains(saved, "Effacee") {
		t.Fatalf("suppressions sans effet :\n%s", saved)
	}

	// Dans la session, l'historique est intact : annuler, rétablir.
	_, err = d.Bus.Undo(ctx)
	must(t, err)
	if !strings.Contains(view(t, d), "LigneEffacee0815") {
		t.Fatal("annuler la requête après l'enregistrement : ligne non rendue")
	}
	_, err = d.Bus.Redo(ctx)
	must(t, err)
	if dirty, _ := d.Dirty(ctx); dirty {
		t.Fatal("revenu à l'état enregistré mais toujours modifié")
	}
	// Un deuxième enregistrement (sans changement) ne perd rien non plus.
	must(t, d.Save(ctx, ""))
	must(t, d.Close())

	// Le fichier : ni journal, ni corbeille, ni les valeurs supprimées.
	raw := dbBytes(t, path)
	for _, s := range []string{"SalaireSecret4711", "LigneEffacee0815", "_lattis_trash", "_lattis_sqltrash"} {
		if bytes.Contains(raw, []byte(s)) {
			t.Errorf("%q encore présent dans le fichier", s)
		}
	}

	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	if got := view(t, d); got != saved {
		t.Fatalf("contenu rouvert :\n%s\nattendu\n%s", got, saved)
	}
	var journal, trash int
	must(t, d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_journal`).Scan(&journal))
	must(t, d.DB().Read().QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name LIKE '\_lattis\_trash%' ESCAPE '\' OR name LIKE '\_lattis\_sqltrash%' ESCAPE '\'`).Scan(&trash))
	if journal != 0 || trash != 0 {
		t.Fatalf("rouvert : %d commande(s) au journal, %d corbeille(s)", journal, trash)
	}
	if st, _ := d.Bus.State(ctx); st.Undo != "" || st.Redo != "" {
		t.Fatalf("historique rouvert : %+v", st)
	}
	checkOK(t, d)
	// Le classeur rouvert se modifie et s'annule normalement.
	set(t, d, 3, 0, "Bob")
	_, err = d.Bus.Undo(ctx)
	must(t, err)
	if got := view(t, d); got != saved {
		t.Fatalf("après annulation :\n%s\nattendu\n%s", got, saved)
	}
}

// dbBytes rend la base telle qu'enregistrée dans l'archive.
func dbBytes(t *testing.T, path string) []byte {
	t.Helper()
	zr, err := archive.OpenZip(path)
	must(t, err)
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == "database.sqlite" {
			r, err := f.Open()
			must(t, err)
			defer r.Close()
			b, err := io.ReadAll(r)
			must(t, err)
			return b
		}
	}
	t.Fatal("base absente de l'archive")
	return nil
}
