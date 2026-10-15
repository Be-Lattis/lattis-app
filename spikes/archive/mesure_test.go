package archive

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/lalamefine/slitable/internal/grid"
)

// TestMesureSauvegarde mesure l'enregistrement et la réouverture d'un
// classeur de SLITABLE_MESURE lignes (tableau de démonstration, 20 colonnes).
func TestMesureSauvegarde(t *testing.T) {
	rows, _ := strconv.Atoi(os.Getenv("SLITABLE_MESURE"))
	if rows <= 0 {
		t.Skip("SLITABLE_MESURE=<nombre de lignes> pour lancer la mesure")
	}
	useRoot(t)
	ctx := context.Background()
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := grid.CreateDemo(ctx, d.DB, rows, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gros.slitable")
	start := time.Now()
	if err := d.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(path)
	t.Logf("enregistrement (VACUUM INTO + ZIP sans compression) : %v, archive %d Mo", time.Since(start).Round(time.Millisecond), st.Size()/1e6)
	d.Close()
	start = time.Now()
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ouverture (extraction) : %v", time.Since(start).Round(time.Millisecond))
	d.Close()
}
