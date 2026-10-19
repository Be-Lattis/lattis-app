package archive

import (
	"archive/zip"
	"bytes"
	stdflate "compress/flate"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// sample : données à moitié compressibles, sur plusieurs blocs.
func sample(n int) []byte {
	r := rand.New(rand.NewPCG(3, 5))
	b := make([]byte, 0, n)
	for len(b) < n {
		if r.IntN(2) == 0 {
			b = fmt.Appendf(b, "ligne %d de la table ", len(b))
		} else {
			b = append(b, byte(r.Uint32()))
		}
	}
	return b[:n]
}

// Les flux écrits par blocs en parallèle se relisent avec des décodeurs
// ordinaires (bibliothèque standard pour Deflate), quelle que soit la
// taille : vide, sous un bloc, juste un bloc, plusieurs blocs.
func TestParallelStreams(t *testing.T) {
	for _, n := range []int{0, 1, 1000, deflateBlock, zstdBlock, 3*zstdBlock + 12345} {
		data := sample(n)
		for _, c := range []struct {
			name string
			new  func(io.Writer) (io.WriteCloser, error)
			read func(io.Reader) io.Reader
		}{
			{"deflate", newDeflate, func(r io.Reader) io.Reader { return stdflate.NewReader(r) }},
			{"zstd", newZstd, func(r io.Reader) io.Reader {
				d, err := zstd.NewReader(r)
				if err != nil {
					t.Fatal(err)
				}
				return d
			}},
		} {
			var buf bytes.Buffer
			w, _ := c.new(&buf)
			// Écritures de tailles diverses, à cheval sur les blocs.
			for rest := data; len(rest) > 0; {
				k := min(len(rest), 1+len(rest)%70001)
				if _, err := w.Write(rest[:k]); err != nil {
					t.Fatal(err)
				}
				rest = rest[k:]
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(c.read(&buf))
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("%s, %d octets : relu %d octets, %v", c.name, n, len(got), err)
			}
		}
	}
}

// Une erreur d'écriture de la destination remonte à Write ou à Close.
func TestParallelWriteError(t *testing.T) {
	w, _ := newZstd(failWriter{})
	w.Write(sample(3 * zstdBlock))
	if err := w.Close(); err == nil {
		t.Fatal("erreur d'écriture perdue")
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("disque plein") }

// Enregistrer et rouvrir avec chaque méthode ; la base garde ses données.
func TestSaveMethods(t *testing.T) {
	for _, m := range []uint16{Zstd, zip.Deflate, zip.Store} {
		t.Run(fmt.Sprint(m), func(t *testing.T) {
			old := Method
			Method = m
			defer func() { Method = old }()
			path := savedRows(t, 3000)
			zr, err := zip.OpenReader(path)
			if err != nil {
				t.Fatal(err)
			}
			var got uint16 = 0xffff
			var size, comp uint64
			for _, f := range zr.File {
				if f.Name == dbName {
					got, size, comp = f.Method, f.UncompressedSize64, f.CompressedSize64
				}
			}
			zr.Close()
			if got != m || (m != zip.Store && comp*2 > size) {
				t.Fatalf("méthode %d, %d octets pour %d", got, comp, size)
			}
			useRoot(t)
			d, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if n := countRows(t, d); n != 3000 {
				t.Fatalf("%d lignes", n)
			}
		})
	}
}

// Un classeur d'avant la compression (base en Store, écrit par la
// bibliothèque standard) s'ouvre toujours ; enregistré, il est compressé.
func TestOpenStoredArchive(t *testing.T) {
	useRoot(t)
	e := entries(t, savedRows(t, 3000))
	p := filepath.Join(t.TempDir(), "ancien.lattis")
	writeEntries(t, p, manifestName, []byte("format_version: 1\napplication: Lattis\n"), dbName, e[dbName])
	d, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, d); n != 3000 {
		t.Fatalf("%d lignes", n)
	}
	if err := d.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	d.Close()
	zr, err := zip.OpenReader(p)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == dbName && f.Method != Zstd {
			t.Fatalf("réenregistré en méthode %d", f.Method)
		}
	}
}

// Bombe ZIP en Zstandard : 1 Go de zéros tient en quelques dizaines de Ko ;
// refusée sans rien écrire.
func TestOpenZstdBomb(t *testing.T) {
	root := useRoot(t)
	path := filepath.Join(t.TempDir(), "bombe.lattis")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := newZipWriter(f)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: dbName, Method: Zstd})
	if err != nil {
		t.Fatal(err)
	}
	head := append([]byte(sqliteMagic), make([]byte, 100-len(sqliteMagic))...)
	head[16], head[17] = 0x10, 0
	w.Write(head)
	zero := make([]byte, 1<<20)
	for range 1024 {
		w.Write(zero)
	}
	zw.Close()
	f.Close()
	if st, _ := os.Stat(path); st.Size() > 1<<20 {
		t.Fatalf("archive de test trop grosse : %d", st.Size())
	}
	_, err = Open(path)
	var oe *OpenError
	if !errors.As(err, &oe) || !errors.Is(err, ErrFormat) || !strings.Contains(oe.Reason, "compressée de façon anormale") {
		t.Fatalf("bombe ZIP : %v", err)
	}
	clean(t, root)
}

// Une base faite de blobs nuls (le pire cas mesuré : 814 fois en
// Zstandard, 181 fois en Deflate), au-delà de ratioFloor, n'est pas prise
// pour une bombe.
func TestZeroBlobNotBomb(t *testing.T) {
	if testing.Short() {
		t.Skip("70 Mo")
	}
	for _, m := range []uint16{Zstd, zip.Deflate} {
		t.Run(fmt.Sprint(m), func(t *testing.T) {
			old := Method
			Method = m
			defer func() { Method = old }()
			useRoot(t)
			d, err := New()
			if err != nil {
				t.Fatal(err)
			}
			exec1(t, d, `CREATE TABLE t (id INTEGER PRIMARY KEY, v BLOB) STRICT`)
			exec1(t, d, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 700)
				INSERT INTO t (v) SELECT zeroblob(100000) FROM n`)
			path := filepath.Join(t.TempDir(), "blobs.lattis")
			if err := d.Save(context.Background(), path); err != nil {
				t.Fatal(err)
			}
			d.Close()
			d, err = Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if n := countRows(t, d); n != 700 {
				t.Fatalf("%d lignes", n)
			}
		})
	}
}
