package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSize(t *testing.T) {
	for n, want := range map[uint64]string{
		0:          "0 o",
		512:        "512 o",
		1536:       "1,5 Ko",
		3 << 20:    "3,0 Mo",
		2252 << 20: "2,2 Go",
		5 << 40:    "5,0 To",
		5000 << 40: "5000,0 To",
	} {
		if got := Size(n); got != want {
			t.Errorf("Size(%d) = %q, attendu %q", n, got, want)
		}
	}
}

// fakeFree fixe l'espace libre vu par l'extraction et l'enregistrement.
func fakeFree(t *testing.T, free uint64) {
	t.Helper()
	old := diskFree
	diskFree = func(string) (uint64, error) { return free, nil }
	t.Cleanup(func() { diskFree = old })
}

// Bombe ZIP : une base dégonflée de 200 Mo (zéros) tient dans quelques
// centaines de Ko ; elle est refusée sans rien écrire.
func TestOpenZipBomb(t *testing.T) {
	root := useRoot(t)
	path := filepath.Join(t.TempDir(), "bombe.lattis")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: dbName, Method: zip.Deflate})
	if err != nil {
		t.Fatal(err)
	}
	head := append([]byte(sqliteMagic), make([]byte, 100-len(sqliteMagic))...)
	head[16], head[17] = 0x10, 0 // pages de 4096 octets
	w.Write(head)
	zero := make([]byte, 1<<20)
	for range 200 {
		w.Write(zero)
	}
	zw.Close()
	f.Close()
	if st, _ := os.Stat(path); st.Size() > 2<<20 {
		t.Fatalf("archive de test trop grosse : %d", st.Size())
	}
	_, err = Open(path)
	var oe *OpenError
	if !errors.As(err, &oe) || !errors.Is(err, ErrFormat) || !strings.Contains(oe.Reason, "compressée de façon anormale") {
		t.Fatalf("bombe ZIP : %v", err)
	}
	clean(t, root)
}

// Taille annoncée par l'archive bien au-delà des pages déclarées par la
// base : refusée.
func TestOpenOversizedDB(t *testing.T) {
	root := useRoot(t)
	src := entries(t, savedRows(t, 50))
	db := src[dbName]
	pages := int(db[28])<<24 | int(db[29])<<16 | int(db[30])<<8 | int(db[31])
	if pages == 0 {
		t.Fatal("en-tête sans nombre de pages")
	}
	grown := append(bytes.Clone(db), make([]byte, 1<<20)...)
	path := filepath.Join(t.TempDir(), "gonflee.lattis")
	writeEntries(t, path, manifestName, src[manifestName], dbName, grown)
	_, err := Open(path)
	var oe *OpenError
	if !errors.As(err, &oe) || !strings.Contains(oe.Reason, "alors que la base en déclare") {
		t.Fatalf("base gonflée : %v", err)
	}
	clean(t, root)
}

// Disque plein : l'ouverture est refusée avant l'extraction, avec la place
// qu'il faudrait.
func TestOpenNoSpace(t *testing.T) {
	root := useRoot(t)
	path := savedRows(t, 50)
	fakeFree(t, 10<<20)
	_, err := Open(path)
	var se *SpaceError
	if !errors.As(err, &se) || se.Saving || !strings.Contains(err.Error(), "espace disque insuffisant pour ouvrir") {
		t.Fatalf("disque plein : %v", err)
	}
	clean(t, root)
}

func TestFreeBytes(t *testing.T) {
	n, err := freeBytes(t.TempDir())
	if err != nil || n == 0 {
		t.Fatalf("espace libre : %d, %v", n, err)
	}
}
