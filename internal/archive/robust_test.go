package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// savedRows enregistre un classeur d'un tableau t de n lignes (assez pour
// occuper des dizaines de pages) et donne son chemin.
func savedRows(t *testing.T, n int) string {
	t.Helper()
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT) STRICT`)
	exec1(t, d, fmt.Sprintf(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < %d)
		INSERT INTO t (v) SELECT 'ligne ' || i || ' ' || hex(randomblob(40)) FROM n`, n))
	path := filepath.Join(t.TempDir(), "classeur.lattis")
	if err := d.Save(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// entries lit les entrées d'une archive.
func entries(t *testing.T, path string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out := map[string][]byte{}
	for _, f := range zr.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		out[f.Name] = b
	}
	return out
}

// writeZip écrit une archive des entrées données (dans cet ordre : nom,
// contenu), la base sans compression comme Lattis.
func writeEntries(t *testing.T, path string, kv ...any) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for i := 0; i < len(kv); i += 2 {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: kv[i].(string), Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		w.Write(kv[i+1].([]byte))
	}
	zw.Close()
	f.Close()
}

func digest(t *testing.T, path string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}

// clean vérifie qu'un échec d'ouverture ne laisse rien dans le dossier de
// travail (ni répertoire, ni session à « récupérer »).
func clean(t *testing.T, root string) {
	t.Helper()
	left, _ := os.ReadDir(root)
	if len(left) != 0 {
		t.Fatalf("dossier de travail non nettoyé : %v", left)
	}
}

// TestOpenBadFiles : chaque fichier illisible ou étranger donne une erreur
// claire, sans panique, sans répertoire de travail laissé, sans toucher au
// fichier.
func TestOpenBadFiles(t *testing.T) {
	root := useRoot(t)
	good := savedRows(t, 3000)
	e := entries(t, good)
	db, man := e[dbName], e[manifestName]
	dir := t.TempDir()

	cases := []struct {
		name    string
		build   func(p string)
		kind    error  // nil : s'ouvre
		reason  string // extrait du message
		salvage bool
	}{
		{"vide", func(p string) { os.WriteFile(p, nil, 0o644) }, ErrFormat, "vide", false},
		{"texte", func(p string) { os.WriteFile(p, []byte("bonjour, ceci n'est pas un classeur"), 0o644) }, ErrFormat, "format de fichier inconnu", false},
		{"aléatoire", func(p string) {
			b := make([]byte, 5000)
			r := rand.New(rand.NewPCG(1, 2))
			for i := range b {
				b[i] = byte(r.Uint32())
			}
			b[0] = 0x7f // ni « PK » ni en-tête SQLite
			os.WriteFile(p, b, 0o644)
		}, ErrFormat, "format de fichier inconnu", false},
		{"base SQLite nue", func(p string) { os.WriteFile(p, db, 0o644) }, ErrFormat, "base SQLite", false},
		{"dossier", func(p string) { os.Mkdir(p, 0o755) }, ErrFormat, "dossier", false},
		{"ZIP tronqué", func(p string) {
			b, _ := os.ReadFile(good)
			os.WriteFile(p, b[:len(b)*2/3], 0o644)
		}, ErrCorrupt, "incomplète", true},
		{"ZIP vide", func(p string) { writeEntries(t, p) }, ErrFormat, "pas de base", false},
		{"sans base", func(p string) { writeEntries(t, p, manifestName, man, "autre.txt", []byte("x")) }, ErrFormat, "pas de base", false},
		{"base illisible", func(p string) { writeEntries(t, p, manifestName, man, dbName, []byte("pas une base")) }, ErrCorrupt, "en-tête invalide", false},
		{"base tronquée", func(p string) { writeEntries(t, p, manifestName, man, dbName, db[:len(db)/2]) }, ErrCorrupt, "tronquée", true},
		{"somme de contrôle fausse", func(p string) {
			b, _ := os.ReadFile(good)
			i := bytes.Index(b, []byte("ligne 1500 "))
			b[i] ^= 0xff
			os.WriteFile(p, b, 0o644)
		}, ErrCorrupt, "altérée", true},
		{"format plus récent", func(p string) {
			writeEntries(t, p, manifestName, []byte("format_version: 99\napplication: Lattis\n"), dbName, db)
		}, ErrFormat, "plus récente", false},
		{"sans manifeste", func(p string) { writeEntries(t, p, dbName, db) }, nil, "", false},
		{"manifeste illisible", func(p string) { writeEntries(t, p, manifestName, []byte("\x00\x01:::"), dbName, db) }, nil, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "_")+".lattis")
			c.build(p)
			var before [32]byte
			if st, _ := os.Stat(p); !st.IsDir() {
				before = digest(t, p)
			}
			d, err := Open(p)
			if c.kind == nil {
				if err != nil {
					t.Fatalf("refusé : %v", err)
				}
				d.Close()
				clean(t, root)
				return
			}
			var oe *OpenError
			if !errors.As(err, &oe) || !errors.Is(err, c.kind) {
				t.Fatalf("erreur %v (%T), attendu %v", err, err, c.kind)
			}
			if !strings.Contains(err.Error(), c.reason) || oe.Salvage != c.salvage {
				t.Fatalf("message %q (récupérable %v), attendu %q (%v)", err, oe.Salvage, c.reason, c.salvage)
			}
			clean(t, root)
			if st, _ := os.Stat(p); !st.IsDir() && digest(t, p) != before {
				t.Fatal("fichier modifié par l'ouverture")
			}
		})
	}
	if _, err := Open(filepath.Join(dir, "absent.lattis")); err == nil || !strings.Contains(err.Error(), "introuvable") {
		t.Fatalf("fichier absent : %v", err)
	}
}

// TestOpenMutations : troncatures et octets altérés au hasard (à la manière
// d'un fuzzing) : jamais de panique, jamais de répertoire laissé.
func TestOpenMutations(t *testing.T) {
	root := useRoot(t)
	good, err := os.ReadFile(savedRows(t, 2000))
	if err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(7, 11))
	p := filepath.Join(t.TempDir(), "mutant.lattis")
	for i := range 60 {
		b := bytes.Clone(good)
		if i%2 == 0 {
			b = b[:r.IntN(len(b))]
		} else {
			for range 1 + r.IntN(20) {
				b[r.IntN(len(b))] ^= byte(1 + r.IntN(255))
			}
		}
		openMutant(t, p, b)
		clean(t, root)
	}
}

func openMutant(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Open(p)
	if err == nil {
		// Ouvert (l'altération ne touche ni l'archive ni l'en-tête) : la
		// lecture peut échouer, mais proprement.
		d.DB.Read().QueryRow(`SELECT count(*) FROM t`).Scan(new(int))
		d.Close()
		return
	}
	var oe *OpenError
	if !errors.As(err, &oe) {
		t.Fatalf("erreur sans message pour l'utilisateur : %v", err)
	}
}

// FuzzOpen : go test -fuzz=FuzzOpen ./internal/archive.
func FuzzOpen(f *testing.F) {
	root := filepath.Join(f.TempDir(), "work")
	old := Root
	Root = func() (string, error) { return root, nil }
	defer func() { Root = old }()
	f.Add([]byte{})
	f.Add([]byte("PK\x03\x04"))
	f.Add([]byte("PK\x05\x06" + strings.Repeat("\x00", 18)))
	f.Add([]byte(sqliteMagic))
	p := filepath.Join(f.TempDir(), "fuzz.lattis")
	f.Fuzz(func(t *testing.T, b []byte) {
		openMutant(t, p, b)
		left, _ := os.ReadDir(root)
		if len(left) != 0 {
			t.Fatalf("dossier de travail non nettoyé : %v", left)
		}
	})
}

func countRows(t *testing.T, d *Doc) int {
	t.Helper()
	var n int
	if err := d.DB.Read().QueryRow(`SELECT count(*) FROM t`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// noSchema : Salvage sans schéma courant (les tables viennent du fichier).
func noSchema(string) error { return nil }

// TestSalvageTruncated : une archive tronquée rend les lignes de la partie
// conservée, dans un nouveau document jamais enregistré.
func TestSalvageTruncated(t *testing.T) {
	useRoot(t)
	good := savedRows(t, 3000)
	b, _ := os.ReadFile(good)
	p := filepath.Join(t.TempDir(), "tronqué.lattis")
	os.WriteFile(p, b[:len(b)*2/3], 0o644)
	before := digest(t, p)
	var oe *OpenError
	if _, err := Open(p); !errors.As(err, &oe) || !oe.Salvage {
		t.Fatalf("ouverture : %v", err)
	}
	d, rep, err := Salvage(p, noSchema)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	n := countRows(t, d)
	if n < 1000 || n >= 3000 || rep.Rows < int64(n) || len(rep.Damaged) != 1 || rep.Damaged[0] != "t" || d.Path != "" {
		t.Fatalf("%d lignes récupérées, rapport %+v, chemin %q", n, rep, d.Path)
	}
	var v string
	d.DB.Read().QueryRow(`SELECT v FROM t WHERE id = 1`).Scan(&v)
	if !strings.HasPrefix(v, "ligne 1 ") {
		t.Fatalf("première ligne : %q", v)
	}
	if digest(t, p) != before {
		t.Fatal("fichier d'origine modifié")
	}
}

// TestSalvageDamagedPages : des pages altérées au milieu de la base ; les
// lignes avant et après sont récupérées.
func TestSalvageDamagedPages(t *testing.T) {
	useRoot(t)
	good := savedRows(t, 3000)
	e := entries(t, good)
	db := bytes.Clone(e[dbName])
	pageSize, pages, _ := dbHeader(db)
	// Quelques pages de feuilles au milieu de la table.
	for pg := pages / 2; pg < pages/2+3; pg++ {
		copy(db[pg*pageSize:], bytes.Repeat([]byte{0xA5}, int(pageSize)))
	}
	p := filepath.Join(t.TempDir(), "abîmé.lattis")
	writeEntries(t, p, manifestName, e[manifestName], dbName, db)
	d, rep, err := Salvage(p, noSchema)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	n := countRows(t, d)
	var hi int
	d.DB.Read().QueryRow(`SELECT max(id) FROM t`).Scan(&hi)
	if n < 2500 || n >= 3000 || hi != 3000 || len(rep.Damaged) != 1 {
		t.Fatalf("%d lignes récupérées (dernière %d), rapport %+v", n, hi, rep)
	}
}

func TestSalvageNothing(t *testing.T) {
	root := useRoot(t)
	p := filepath.Join(t.TempDir(), "rien.lattis")
	os.WriteFile(p, []byte("PK\x03\x04 pas grand-chose"), 0o644)
	if _, _, err := Salvage(p, noSchema); !errors.Is(err, errNothing) {
		t.Fatalf("récupération de rien : %v", err)
	}
	clean(t, root)
}

// TestSaveInterrupted : un enregistrement qui échoue juste avant le
// renommage laisse l'ancien fichier intact et aucun fichier temporaire.
func TestSaveInterrupted(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	path := savedRows(t, 10)
	before := digest(t, path)
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	exec1(t, d, `INSERT INTO t (v) VALUES ('nouvelle')`)
	old := beforeRename
	beforeRename = func(string) error { return errors.New("disque débranché") }
	err = d.Save(ctx, "")
	beforeRename = old
	if err == nil || !strings.Contains(err.Error(), "n'a pas été modifié") {
		t.Fatalf("enregistrement interrompu : %v", err)
	}
	if digest(t, path) != before {
		t.Fatal("ancien fichier modifié")
	}
	if tmp, _ := filepath.Glob(filepath.Join(filepath.Dir(path), tempPrefix+"*")); len(tmp) != 0 {
		t.Fatalf("fichiers temporaires laissés : %v", tmp)
	}
	// Dossier de destination absent : erreur, rien d'écrit.
	if err := d.Save(ctx, filepath.Join(t.TempDir(), "absent", "x.lattis")); err == nil {
		t.Fatal("enregistrement dans un dossier absent accepté")
	}
	if d.Path != path {
		t.Fatalf("chemin changé après un échec : %s", d.Path)
	}
	if err := d.Save(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if digest(t, path) == before {
		t.Fatal("enregistrement sans effet")
	}
}

// TestSaveKilled : le processus est tué pendant l'enregistrement (fichier
// temporaire écrit, pas encore renommé). L'ancien fichier est intact et
// s'ouvre, les modifications sont récupérables, le fichier temporaire est
// nettoyé au prochain enregistrement.
func TestSaveKilled(t *testing.T) {
	if os.Getenv("LATTIS_SAVEKILL_CHILD") != "" {
		saveKillChild()
		return
	}
	root := useRoot(t)
	ctx := context.Background()
	path := savedRows(t, 10)
	before := digest(t, path)
	cmd := exec.Command(os.Args[0], "-test.run=^TestSaveKilled$")
	cmd.Env = append(os.Environ(), "LATTIS_SAVEKILL_CHILD="+path, "LATTIS_SAVEKILL_ROOT="+root)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("l'enfant devait mourir avec le code 3 : %v\n%s", err, out)
	}
	if digest(t, path) != before {
		t.Fatal("ancien fichier modifié par un enregistrement interrompu")
	}
	tmp, _ := filepath.Glob(filepath.Join(filepath.Dir(path), tempPrefix+"*"))
	if len(tmp) != 1 {
		t.Fatalf("fichier temporaire attendu : %v", tmp)
	}
	var rec *RecoverableError
	if _, err := Open(path); !errors.As(err, &rec) {
		t.Fatalf("Open après l'arrêt : %v", err)
	}
	d, err := Recover(rec.Orphan)
	if err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, d); n != 11 {
		t.Fatalf("%d lignes récupérées, attendu 11", n)
	}
	old := staleTemp
	staleTemp = 0
	defer func() { staleTemp = old }()
	time.Sleep(10 * time.Millisecond)
	if err := d.Save(ctx, ""); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if tmp, _ := filepath.Glob(filepath.Join(filepath.Dir(path), tempPrefix+"*")); len(tmp) != 0 {
		t.Fatalf("fichier temporaire non nettoyé : %v", tmp)
	}
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if n := countRows(t, d); n != 11 {
		t.Fatalf("%d lignes après enregistrement, attendu 11", n)
	}
}

func saveKillChild() {
	Root = func() (string, error) { return os.Getenv("LATTIS_SAVEKILL_ROOT"), nil }
	d, err := Open(os.Getenv("LATTIS_SAVEKILL_CHILD"))
	if err != nil {
		panic(err)
	}
	err = d.DB.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO t (v) VALUES ('non enregistré')`)
		return err
	})
	if err != nil {
		panic(err)
	}
	beforeRename = func(string) error { os.Exit(3); return nil }
	d.Save(context.Background(), "")
	os.Exit(4)
}

// TestExtractInterrupted : un répertoire de travail sans base (extraction
// interrompue) n'est pas proposé en récupération, il est nettoyé, et
// l'archive se rouvre normalement.
func TestExtractInterrupted(t *testing.T) {
	root := useRoot(t)
	path := savedRows(t, 10)
	dir, _ := workDir(path)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, dbName+".part"), []byte("SQLite format 3\x00 à moitié"), 0o644)
	if o, err := Orphans(); err != nil || len(o) != 0 {
		t.Fatalf("orphelins : %+v %v", o, err)
	}
	clean(t, root)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, dbName+".part"), []byte("x"), 0o644)
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if n := countRows(t, d); n != 10 {
		t.Fatalf("%d lignes", n)
	}
}
