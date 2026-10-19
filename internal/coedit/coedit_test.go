package coedit_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/coedit"
	"github.com/lalamefine/lattis/internal/doc"
)

// Délais raccourcis : les essais font les tours à la main (Sync).
var fast = coedit.Timing{Poll: 10 * time.Millisecond, Watch: 10 * time.Millisecond, Beat: 10 * time.Millisecond, Stale: 300 * time.Millisecond, JoinWait: 5 * time.Second, Compact: time.Millisecond}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// station est un poste : son répertoire de travail local, son classeur et
// sa session.
type station struct {
	t    *testing.T
	name string
	root string
	d    *doc.Document
	e    *coedit.Engine
}

// useRoot donne à chaque poste son propre dossier de répertoires de
// travail (archive.Root est lu à l'ouverture seulement).
func useRoot(t *testing.T, root string) {
	old := archive.Root
	archive.Root = func() (string, error) { return root, nil }
	t.Cleanup(func() { archive.Root = old })
}

// workbook crée « Ventes.lattis » dans un dossier « partagé » : un tableau
// Ventes de trois lignes.
func workbook(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	shared := t.TempDir()
	useRoot(t, filepath.Join(t.TempDir(), "maker"))
	d, err := doc.New(ctx)
	must(t, err)
	_, err = d.Paste(ctx, 1, 0, 0, [][]string{
		{"Produit", "Quantité", "Prix"},
		{"Café", "3", "4,5"},
		{"Thé", "10", "2"},
		{"Chocolat", "1", "3,25"},
	})
	must(t, err)
	var first string
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables`).Scan(&first))
	must(t, d.RenameTable(ctx, first, "Ventes"))
	path := filepath.Join(shared, "Ventes.lattis")
	must(t, d.Save(ctx, path))
	must(t, d.Close())
	return path
}

// open ouvre le classeur sur un nouveau poste, seul (pas de demande).
func open(t *testing.T, name, path string) *station {
	t.Helper()
	s := &station{t: t, name: name, root: filepath.Join(t.TempDir(), name)}
	useRoot(t, s.root)
	d, err := doc.OpenAs(context.Background(), path, doc.OpenNormal)
	must(t, err)
	s.d = d
	e, err := coedit.New(d, coedit.Options{Timing: fast})
	must(t, err)
	s.e = e
	t.Cleanup(s.close)
	return s
}

// join fait rejoindre un nouveau poste : sa demande, pendant que les
// postes déjà là font leurs tours.
func join(t *testing.T, name, path string, others ...*station) *station {
	t.Helper()
	ctx := context.Background()
	s := &station{t: t, name: name, root: filepath.Join(t.TempDir(), name)}
	type res struct {
		tk  *coedit.Ticket
		err error
	}
	ch := make(chan res, 1)
	go func() {
		tk, err := coedit.Join(ctx, path, fast)
		ch <- res{tk, err}
	}()
	var r res
wait:
	for {
		select {
		case r = <-ch:
			break wait
		case <-time.After(5 * time.Millisecond):
			for _, o := range others {
				o.e.Sync(ctx)
			}
		}
	}
	must(t, r.err)
	useRoot(t, s.root)
	d, err := doc.OpenAs(ctx, path, doc.OpenCoEdit)
	must(t, err)
	s.d = d
	e, err := coedit.Start(ctx, d, r.tk, coedit.Options{Timing: fast})
	must(t, err)
	s.e = e
	t.Cleanup(s.close)
	return s
}

func (s *station) close() {
	if s.e != nil {
		s.e.Close()
		s.e = nil
	}
	if s.d != nil {
		s.d.Close()
		s.d = nil
	}
}

// crash arrête le poste sans rien retirer (processus tué) : sa présence et
// son journal restent.
func (s *station) crash() {
	s.e = nil
	s.d.Unlock()
}

// set saisit raw dans la cellule (ligne de données n, colonne i) de Ventes.
func (s *station) set(n, i int, raw string) {
	s.t.Helper()
	_, err := s.d.SetCell(context.Background(), 1, 1+n, i, raw)
	must(s.t, err)
}

func (s *station) sync() coedit.Event { return s.e.Sync(context.Background()) }

// rows rend les lignes de Ventes dans l'ordre, valeurs jointes par « | ».
func (s *station) rows() []string {
	s.t.Helper()
	var tbl int64
	must(s.t, s.d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = 'Ventes'`).Scan(&tbl))
	rs, err := s.d.DB().Read().Query(fmt.Sprintf(`SELECT c_1, c_2, c_3 FROM t_%d ORDER BY _pos, id`, tbl))
	must(s.t, err)
	defer rs.Close()
	var out []string
	for rs.Next() {
		var a, b, c sql.NullString
		must(s.t, rs.Scan(&a, &b, &c))
		out = append(out, a.String+"|"+b.String+"|"+c.String)
	}
	return out
}

// settle fait des tours sur tous les postes jusqu'à ce qu'ils aient les
// mêmes lignes.
func settle(t *testing.T, all ...*station) {
	t.Helper()
	for range 50 {
		for _, s := range all {
			s.sync()
		}
		same := true
		ref := strings.Join(all[0].rows(), "\n")
		for _, s := range all[1:] {
			if strings.Join(s.rows(), "\n") != ref {
				same = false
			}
		}
		if same {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	for _, s := range all {
		t.Logf("%s : %v", s.name, s.rows())
	}
	t.Fatal("les postes ne convergent pas")
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lignes :\n%s\nattendu :\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func conflicts(s *station) int {
	var n int
	s.d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_conflicts`).Scan(&n)
	return n
}

func TestTwoStations(t *testing.T) {
	path := workbook(t)
	alice := open(t, "alice", path)
	// Alice modifie avant que Bob arrive : enregistré à son arrivée.
	alice.set(0, 1, "4")
	bob := join(t, "bob", path, alice)
	eq(t, bob.rows(), []string{"Café|4|4.5", "Thé|10|2", "Chocolat|1|3.25"})
	if dirty, _ := alice.d.Dirty(context.Background()); dirty {
		t.Fatal("Alice aurait dû enregistrer à l'arrivée de Bob")
	}
	if st := alice.e.State(); !st.Active || len(st.Peers) != 1 {
		t.Fatalf("Alice : %+v", st)
	}

	// Cellules différentes, lignes ajoutées et supprimées des deux côtés.
	alice.set(0, 2, "5")
	bob.set(1, 1, "11")
	bob.set(3, 0, "Biscuit") // sous le tableau : une ligne de plus
	must(t, alice.d.DeleteRow(context.Background(), "Ventes", 2))
	settle(t, alice, bob)
	eq(t, alice.rows(), []string{"Café|4|5", "Thé|11|2", "Biscuit||"})

	// Ligne ajoutée par Bob, modifiée aussitôt par Alice.
	alice.set(2, 1, "7")
	settle(t, alice, bob)
	eq(t, bob.rows(), []string{"Café|4|5", "Thé|11|2", "Biscuit|7|"})
	if conflicts(alice)+conflicts(bob) != 0 {
		t.Fatal("aucun conflit attendu")
	}
}

func TestSameCellConflict(t *testing.T) {
	path := workbook(t)
	alice := open(t, "alice", path)
	bob := join(t, "bob", path, alice)
	// Saisies concurrentes : aucun des deux n'a vu l'autre.
	alice.set(0, 1, "100")
	bob.set(0, 1, "200")
	alice.sync() // envoi d'Alice
	bob.sync()   // envoi de Bob, puis réception d'Alice
	alice.sync() // réception de Bob
	settle(t, alice, bob)
	got := alice.rows()[0]
	if got != "Café|100|4.5" && got != "Café|200|4.5" {
		t.Fatalf("valeur retenue : %s", got)
	}
	// Le perdant a un conflit, le gagnant non.
	loser, winner := alice, bob
	if got == "Café|100|4.5" {
		loser, winner = bob, alice
	}
	if conflicts(loser) != 1 || conflicts(winner) != 0 {
		t.Fatalf("conflits : perdant %d, gagnant %d", conflicts(loser), conflicts(winner))
	}
	cs, err := loser.e.Conflicts(context.Background())
	must(t, err)
	if len(cs) != 1 || cs[0].Author == "" {
		t.Fatalf("conflit : %+v", cs)
	}
	// « Garder la mienne » : elle repart et l'emporte partout.
	mine := "Café|100|4.5"
	if loser == bob {
		mine = "Café|200|4.5"
	}
	must(t, loser.e.Resolve(context.Background(), cs[0].ID, true))
	settle(t, alice, bob)
	eq(t, winner.rows()[:1], []string{mine})
	if conflicts(loser) != 0 {
		t.Fatal("conflit réglé")
	}
}

func TestStructureRefusedTogether(t *testing.T) {
	path := workbook(t)
	alice := open(t, "alice", path)
	bob := join(t, "bob", path, alice)
	alice.sync()
	ctx := context.Background()
	err := alice.d.RenameTable(ctx, "Ventes", "Ventes2")
	var te coedit.TogetherError
	if !errors.As(err, &te) {
		t.Fatalf("renommer à deux : %v", err)
	}
	if _, err := alice.d.SetCell(ctx, 1, 10, 10, "libre"); !errors.As(err, &te) {
		t.Fatalf("cellule libre à deux : %v", err)
	}
	// Bob part : Alice, seule, peut tout faire.
	bob.close()
	alice.sync()
	if st := alice.e.State(); len(st.Peers) != 0 {
		t.Fatalf("Bob devrait être parti : %+v", st)
	}
	must(t, alice.d.RenameTable(ctx, "Ventes", "Ventes2"))
	must(t, alice.d.RenameTable(ctx, "Ventes2", "Ventes"))

	// Carol arrive : Alice enregistre d'abord (changements hors journal).
	carol := join(t, "carol", path, alice)
	if dirty, _ := alice.d.Dirty(ctx); dirty {
		t.Fatal("Alice aurait dû enregistrer à l'arrivée de Carol")
	}
	settle(t, alice, carol)
}

func TestSaveThenContinue(t *testing.T) {
	path := workbook(t)
	ctx := context.Background()
	alice := open(t, "alice", path)
	bob := join(t, "bob", path, alice)
	alice.set(0, 1, "30")
	settle(t, alice, bob)
	must(t, alice.d.Save(ctx, ""))
	m, err := archive.ReadManifest(path)
	must(t, err)
	if len(m.Journal) == 0 {
		t.Fatal("positions des journaux absentes du manifeste")
	}
	// Bob continue, puis enregistre par-dessus le fichier d'Alice : il
	// contient tout ce qu'elle avait.
	bob.set(1, 1, "12")
	settle(t, alice, bob)
	must(t, bob.d.Save(ctx, ""))
	alice.set(2, 1, "2")
	settle(t, alice, bob)
	must(t, alice.d.Save(ctx, ""))

	// Carol rejoint après ces enregistrements : fichier + fin des journaux.
	alice.set(0, 2, "9")
	alice.sync()
	carol := join(t, "carol", path, alice, bob)
	settle(t, alice, bob, carol)
	eq(t, carol.rows(), []string{"Café|30|9", "Thé|12|2", "Chocolat|2|3.25"})

	// Tous partent : le dossier disparaît ; le fichier rouvert a ce qui a
	// été enregistré.
	must(t, carol.d.Save(ctx, ""))
	alice.close()
	bob.close()
	carol.close()
	if _, err := os.Stat(coedit.DirOf(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dossier du travail à plusieurs resté : %v", err)
	}
	dave := open(t, "dave", path)
	eq(t, dave.rows(), []string{"Café|30|9", "Thé|12|2", "Chocolat|2|3.25"})
}

func TestUnknownSaveAsks(t *testing.T) {
	path := workbook(t)
	ctx := context.Background()
	alice := open(t, "alice", path)
	bob := join(t, "bob", path, alice)
	alice.set(0, 1, "8")
	alice.sync()
	must(t, alice.d.Save(ctx, ""))
	// Bob n'a pas encore lu la ligne d'Alice : Known fait un tour, puis
	// l'enregistrement passe, avec les deux modifications.
	bob.set(1, 1, "9")
	must(t, bob.d.Save(ctx, ""))
	eq(t, bob.rows()[:2], []string{"Café|8|4.5", "Thé|9|2"})
	carol := open(t, "carol", path)
	eq(t, carol.rows()[:2], []string{"Café|8|4.5", "Thé|9|2"})
}

func TestCrashedStation(t *testing.T) {
	path := workbook(t)
	ctx := context.Background()
	alice := open(t, "alice", path)
	bob := join(t, "bob", path, alice)
	bob.set(0, 1, "50")
	bob.sync()
	settle(t, alice, bob)
	bobSid := bob.e.Session()
	bob.crash()
	// Le dernier battement de Bob peut n'être vu qu'au tour suivant : le
	// départ se constate Stale après, à l'horloge d'Alice.
	alice.sync()
	time.Sleep(fast.Stale + 50*time.Millisecond)
	alice.sync()
	if st := alice.e.State(); len(st.Peers) != 0 {
		t.Fatalf("Bob planté devrait être parti : %+v", st)
	}
	// Seule : la structure se change.
	must(t, alice.d.RenameTable(ctx, "Ventes", "Ventes2"))
	must(t, alice.d.RenameTable(ctx, "Ventes2", "Ventes"))
	// Enregistré : le journal de Bob est couvert, il disparaît au
	// compactage.
	must(t, alice.d.Save(ctx, ""))
	alice.sync()
	alice.sync()
	entries, _ := os.ReadDir(coedit.DirOf(path))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), bobSid) {
			t.Fatalf("reste de Bob : %s", e.Name())
		}
	}
}

func TestPartialLine(t *testing.T) {
	path := workbook(t)
	alice := open(t, "alice", path)
	bob := join(t, "bob", path, alice)
	settle(t, alice, bob)
	// Un troisième journal, écrit à la main en deux fois.
	var tbl int64
	must(t, alice.d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = 'Ventes'`).Scan(&tbl))
	var row string
	must(t, alice.d.DB().Read().QueryRow(fmt.Sprintf(`SELECT r.lattis_id FROM _lattis_row_ids r JOIN t_%d t ON t.id = r.rid WHERE r.tbl = %d ORDER BY t._pos LIMIT 1`, tbl, tbl)).Scan(&row))
	line, _ := json.Marshal(map[string]any{"v": 1, "seq": 1, "lc": 99, "at": "2026-10-19T10:00:00Z", "by": "Zoé", "kind": "do",
		"ops": []map[string]any{{"k": "set", "t": tbl, "r": row, "c": 2, "n": 77}}})
	jp := filepath.Join(coedit.DirOf(path), "PC-zoe-1-abcd-1.jsonl")
	half := len(line) / 2
	must(t, os.WriteFile(jp, line[:half], 0o644))
	alice.sync()
	if alice.rows()[0] != "Café|3|4.5" {
		t.Fatalf("ligne incomplète appliquée : %v", alice.rows())
	}
	f, err := os.OpenFile(jp, os.O_APPEND|os.O_WRONLY, 0o644)
	must(t, err)
	f.Write(append(line[half:], '\n'))
	f.Close()
	settle(t, alice, bob)
	eq(t, alice.rows()[:1], []string{"Café|77|4.5"})
}

func TestThreeStationsCausal(t *testing.T) {
	path := workbook(t)
	alice := open(t, "alice", path)
	bob := join(t, "bob", path, alice)
	carol := join(t, "carol", path, alice, bob)
	// Bob ajoute une ligne ; Carol la reçoit et la modifie ; Alice lit le
	// journal de Carol avant celui de Bob : la modification attend l'ajout.
	bob.set(3, 0, "Biscuit")
	bob.sync()
	carol.sync()
	carol.set(3, 1, "6")
	carol.sync()
	if got := alice.rows(); len(got) != 3 {
		t.Fatalf("Alice : %v", got)
	}
	settle(t, alice, bob, carol)
	eq(t, alice.rows(), []string{"Café|3|4.5", "Thé|10|2", "Chocolat|1|3.25", "Biscuit|6|"})
	names := []string{}
	for _, p := range alice.e.State().Peers {
		names = append(names, p.Machine)
	}
	sort.Strings(names)
	if len(names) != 2 {
		t.Fatalf("présences : %v", names)
	}
}

func TestUndoKeepsColleagueValue(t *testing.T) {
	path := workbook(t)
	ctx := context.Background()
	alice := open(t, "alice", path)
	bob := join(t, "bob", path, alice)
	alice.set(0, 1, "40")
	settle(t, alice, bob)
	bob.set(0, 1, "41")
	settle(t, alice, bob)
	// Alice annule sa saisie : Bob a changé la cellule depuis, elle garde
	// la valeur de Bob.
	_, err := alice.d.Bus.Undo(ctx)
	must(t, err)
	settle(t, alice, bob)
	eq(t, bob.rows()[:1], []string{"Café|41|4.5"})
}

func TestRunLoops(t *testing.T) {
	path := workbook(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	alice := open(t, "alice", path)
	go alice.e.Run(ctx)
	bob := join(t, "bob", path)
	go bob.e.Run(ctx)
	done := make(chan struct{})
	for _, s := range []*station{alice, bob} {
		go func() {
			defer func() { done <- struct{}{} }()
			col := 1
			if s == bob {
				col = 2
			}
			for i := range 15 {
				if _, err := s.d.SetCell(ctx, 1, 1+i%3, col, fmt.Sprint(i)); err != nil {
					t.Error(err)
					return
				}
				s.e.Kick()
			}
		}()
	}
	<-done
	<-done
	deadline := time.Now().Add(5 * time.Second)
	for strings.Join(alice.rows(), "\n") != strings.Join(bob.rows(), "\n") {
		if time.Now().After(deadline) {
			t.Fatalf("pas de convergence : %v / %v", alice.rows(), bob.rows())
		}
		time.Sleep(10 * time.Millisecond)
	}
	eq(t, alice.rows(), []string{"Café|12|12", "Thé|13|13", "Chocolat|14|14"})
}

func TestBarrier(t *testing.T) {
	path := workbook(t)
	ctx := context.Background()
	alice := open(t, "alice", path)
	bob := join(t, "bob", path, alice)
	settle(t, alice, bob)
	// Bob ne bat plus (réseau coupé) : Alice se croit seule et change la
	// structure.
	alice.sync()
	time.Sleep(fast.Stale + 50*time.Millisecond)
	alice.sync()
	must(t, alice.d.RenameTable(ctx, "Ventes", "Ventes2"))
	must(t, alice.d.RenameTable(ctx, "Ventes2", "Ventes"))
	alice.set(0, 1, "5")
	alice.sync()
	must(t, alice.d.Save(ctx, ""))
	// Bob revient : la barrière lui dit de rouvrir ; enregistrer par-dessus
	// redevient une question.
	bob.sync()
	if st := bob.e.State(); st.Stale == "" {
		t.Fatalf("barrière non vue : %+v", st)
	}
	bob.set(1, 1, "6")
	var changed *archive.ChangedError
	if err := bob.d.Save(ctx, ""); !errors.As(err, &changed) {
		t.Fatalf("enregistrement après une barrière : %v", err)
	}
}
