package bridge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/grid"
)

// TestMesureDemarrage mesure, pour des classeurs de 0, 1 M et 10 M de
// lignes (10 colonnes), un démarrage à froid du moteur dans un processus
// neuf : initialisation des paquets Go, ouverture, première vue (LayoutFirst
// et premier bloc), mise en page complète, et la mémoire (tas Go, mémoire
// réservée, pic du processus). Le moteur est le même pour le bureau et la
// version web (engine).
//
//	LATTIS_MESURE_DEMARRAGE=0,1000000,10000000 go test ./internal/bridge -run TestMesureDemarrage -v -timeout 60m
//
// LATTIS_MESURE_DOSSIER garde les classeurs d'une fois sur l'autre.
func TestMesureDemarrage(t *testing.T) {
	if p := os.Getenv("LATTIS_MESURE_DEMARRAGE_ENFANT"); p != "" {
		demarrageEnfant(t, p)
		return
	}
	list := os.Getenv("LATTIS_MESURE_DEMARRAGE")
	if list == "" {
		t.Skip("LATTIS_MESURE_DEMARRAGE=<lignes>,… pour lancer la mesure")
	}
	dir := os.Getenv("LATTIS_MESURE_DOSSIER")
	if dir == "" {
		dir = t.TempDir()
	}
	for _, f := range strings.Split(list, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, fmt.Sprintf("demarrage-%d.lattis", n))
		if _, err := os.Stat(path); err != nil {
			start := time.Now()
			demarrageClasseur(t, path, n)
			t.Logf("%d lignes : classeur fait en %v", n, time.Since(start).Round(time.Millisecond))
		}
		st, _ := os.Stat(path)
		// Deux démarrages : le premier lit le fichier du disque, le second
		// le trouve dans le cache du système.
		for i := range 2 {
			cmd := exec.Command(os.Args[0], "-test.run=^TestMesureDemarrage$", "-test.v")
			cmd.Env = append(os.Environ(), "LATTIS_MESURE_DEMARRAGE_ENFANT="+path, "GODEBUG=inittrace=1")
			start := time.Now()
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s\n%s", err, out)
			}
			total := time.Since(start)
			init := 0.0
			for _, m := range initLine.FindAllStringSubmatch(string(out), -1) {
				f, _ := strconv.ParseFloat(m[1], 64)
				init += f
			}
			var lines []string
			for _, l := range strings.Split(string(out), "\n") {
				if i := strings.Index(l, "demarrage: "); i >= 0 {
					lines = append(lines, strings.TrimSpace(l[i+len("demarrage: "):]))
				}
			}
			t.Logf("%d lignes (%d Mo), démarrage %d : processus %v, init des paquets %.0f ms ; %s", n, st.Size()>>20, i+1,
				total.Round(time.Millisecond), init, strings.Join(lines, " ; "))
		}
	}
}

var initLine = regexp.MustCompile(`init [^ ]+ @[0-9.]+ ms, ([0-9.]+) ms clock`)

// demarrageClasseur fait un classeur de n lignes × 10 colonnes (comme
// TestMesureVue) et une deuxième feuille.
func demarrageClasseur(t *testing.T, path string, n int) {
	ctx := context.Background()
	s := &DocumentService{}
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	defer s.CloseDocument(ctx)
	d, err := s.current()
	if err != nil {
		t.Fatal(err)
	}
	if n > 0 {
		headers := make([]string, 10)
		types := make([]grid.Type, 10)
		for i := range headers {
			headers[i] = fmt.Sprintf("Col%d", i+1)
			types[i] = grid.Text
			if i%2 == 1 {
				types[i] = grid.Integer
			}
		}
		types[0] = grid.Integer
		if _, err := d.Import(ctx, "Importer", []canvas.ImportSheet{{Name: "Données", Tables: []canvas.ImportTable{{
			Name: "Gros", Headers: headers, Types: types, Count: n,
			Rows: func(yield func([]any) error) error {
				row := make([]any, 10)
				for i := range n {
					for c := range row {
						if types[c] == grid.Integer {
							row[c] = int64(i*(c+1)) % 100003
						} else {
							row[c] = "Texte " + strconv.Itoa(i%7919+c)
						}
					}
					row[0] = int64(i + 1)
					if err := yield(row); err != nil {
						return err
					}
				}
				return nil
			},
		}}}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
}

// demarrageEnfant : le démarrage mesuré, dans le processus neuf.
func demarrageEnfant(t *testing.T, path string) {
	ctx := context.Background()
	s := &DocumentService{}
	var out []string
	step := func(name string, fn func() error) {
		start := time.Now()
		if err := fn(); err != nil {
			t.Fatalf("%s : %v", name, err)
		}
		out = append(out, fmt.Sprintf("%s %.0f ms", name, float64(time.Since(start).Microseconds())/1000))
	}
	start := time.Now()
	step("Open", func() error { _, err := s.Open(ctx, path); return err })
	var sheet, table int64
	step("LayoutFirst", func() error {
		l, err := s.LayoutFirst(ctx, 0)
		for _, sh := range l.Sheets {
			if sh.Name == "Données" {
				sheet = sh.ID
			}
		}
		if err == nil && sheet != 0 && sheet != l.Sheet {
			l, err = s.LayoutFirst(ctx, sheet)
		}
		if err == nil && len(l.Tables) > 0 {
			table = l.Tables[0].ID
		}
		return err
	})
	if table != 0 {
		step("bloc 0+200", func() error { _, err := s.Window(ctx, sheet, table, 0, 200); return err })
	}
	out = append(out, fmt.Sprintf("première vue %.0f ms", float64(time.Since(start).Microseconds())/1000))
	step("Layout complet", func() error { _, err := s.Layout(ctx, sheet); return err })
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	out = append(out, fmt.Sprintf("tas Go %d Mo, réservé %d Mo, pic du processus %d Mo", m.HeapInuse>>20, m.Sys>>20, peakRSS()>>20))
	s.CloseDocument(ctx)
	fmt.Println("demarrage: " + strings.Join(out, ", "))
}
