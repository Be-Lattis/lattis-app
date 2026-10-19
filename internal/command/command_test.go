package command

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/store"
)

func open(t *testing.T, path string) (*store.Store, *Bus) {
	t.Helper()
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(Schema + `CREATE TABLE IF NOT EXISTS t (id INTEGER PRIMARY KEY, v ANY) STRICT;`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, New(db)
}

func value(t *testing.T, db *store.Store, id int64) any {
	t.Helper()
	var v any
	err := db.Read().QueryRow(`SELECT v FROM t WHERE id = ?`, id).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "absent"
	}
	if err != nil {
		t.Fatal(err)
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

func insert(id int64, v any) func(*Tx) error {
	return func(t *Tx) error {
		return t.Apply([]Stmt{S(`INSERT INTO t (id, v) VALUES (?, ?)`, id, v)}, []Stmt{S(`DELETE FROM t WHERE id = ?`, id)})
	}
}

func TestUndoRedoSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "j.sqlite")
	db, bus := open(t, path)
	big := int64(math.MaxInt64 - 7)
	for i, v := range []any{big, 1.5, "texte", []byte{0xE5, 'B', 1}, nil} {
		if _, err := bus.Do(ctx, "saisie", insert(int64(i+1), v)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Réouverture : l'historique est intact, rétablir rejoue les paramètres
	// avec leur type exact.
	db, bus = open(t, path)
	defer db.Close()
	st, _ := bus.State(ctx)
	if st.Undo != "saisie" || st.Redo != "saisie" {
		t.Fatalf("état %+v", st)
	}
	for range 5 {
		if _, err := bus.Undo(ctx); err != nil && !errors.Is(err, ErrNothing) {
			t.Fatal(err)
		}
	}
	if v := value(t, db, 1); v != "absent" {
		t.Fatalf("tout annulé, reste %v", v)
	}
	for range 5 {
		if _, err := bus.Redo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := bus.Redo(ctx); !errors.Is(err, ErrNothing) {
		t.Fatalf("rétablir de trop : %v", err)
	}
	if v := value(t, db, 1); v != big {
		t.Fatalf("entier : %v", v)
	}
	if v := value(t, db, 2); v != 1.5 {
		t.Fatalf("réel : %v", v)
	}
	if v := value(t, db, 4); v != string([]byte{0xE5, 'B', 1}) {
		t.Fatalf("BLOB : %v", v)
	}
	if v := value(t, db, 5); v != nil {
		t.Fatalf("NULL : %v", v)
	}
}

func TestNewActionClearsRedoAndFailedActionLeavesNothing(t *testing.T) {
	ctx := context.Background()
	db, bus := open(t, filepath.Join(t.TempDir(), "j.sqlite"))
	defer db.Close()
	bus.Do(ctx, "a", insert(1, "a"))
	bus.Do(ctx, "b", insert(2, "b"))
	bus.Undo(ctx)
	bus.Do(ctx, "c", insert(3, "c"))
	if _, err := bus.Redo(ctx); !errors.Is(err, ErrNothing) {
		t.Fatalf("b devrait être oublié : %v", err)
	}
	// Une action qui échoue au milieu n'applique rien et n'entre pas dans l'historique.
	_, err := bus.Do(ctx, "échec", func(t *Tx) error {
		if err := insert(4, "d")(t); err != nil {
			return err
		}
		return insert(4, "doublon")(t)
	})
	if err == nil || value(t, db, 4) != "absent" {
		t.Fatalf("action ratée appliquée : %v", err)
	}
	if st, _ := bus.State(ctx); st.Undo != "c" {
		t.Fatalf("historique : %+v", st)
	}
}

func TestAppendGroupsAndDepthPurges(t *testing.T) {
	ctx := context.Background()
	db, bus := open(t, filepath.Join(t.TempDir(), "j.sqlite"))
	defer db.Close()
	// Un script : trois petites transactions, une seule étape d'annulation.
	seq, _ := bus.Append(ctx, 0, "script", insert(1, "x"))
	seq, _ = bus.Append(ctx, seq, "script", insert(2, "y"))
	bus.Append(ctx, seq, "script", insert(3, "z"))
	bus.Undo(ctx)
	for id := int64(1); id <= 3; id++ {
		if v := value(t, db, id); v != "absent" {
			t.Fatalf("le script n'est pas annulé d'un coup : %d = %v", id, v)
		}
	}

	// Au-delà de Depth, la plus ancienne commande sort et sa purge s'exécute.
	bus.Depth = 2
	purge := func(id int64) func(*Tx) error {
		return func(t *Tx) error {
			return t.ApplyStep(Step{
				Do:    []Stmt{S(`INSERT INTO t (id, v) VALUES (?, 'p')`, id)},
				Undo:  []Stmt{S(`DELETE FROM t WHERE id = ?`, id)},
				Purge: []Stmt{S(`UPDATE t SET v = 'purgé' WHERE id = ?`, id)},
			})
		}
	}
	for id := int64(10); id < 13; id++ {
		bus.Do(ctx, "p", purge(id))
	}
	if v := value(t, db, 10); v != "purgé" {
		t.Fatalf("purge non exécutée : %v", v)
	}
	if v := value(t, db, 11); v != "p" {
		t.Fatalf("purge trop tôt : %v", v)
	}
}

func TestGroup(t *testing.T) {
	ctx := context.Background()
	db, bus := open(t, filepath.Join(t.TempDir(), "g.sqlite"))
	defer db.Close()
	gctx := WithGroup(ctx, "Script")
	for i := 1; i <= 3; i++ {
		if _, err := bus.Do(gctx, "Action", func(tx *Tx) error {
			return tx.Apply([]Stmt{S(`INSERT INTO t (v) VALUES (?)`, i)}, []Stmt{S(`DELETE FROM t WHERE v = ?`, i)})
		}); err != nil {
			t.Fatal(err)
		}
	}
	st, err := bus.State(ctx)
	if err != nil || st.Undo != "Script" {
		t.Fatalf("état %+v, %v", st, err)
	}
	if _, err := bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	db.Read().QueryRow(`SELECT count(*) FROM t`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d lignes après une seule annulation", n)
	}
}

// Detach coupe une action en deux étapes : l'action suivie de T, puis T
// défaite ; l'état final ne change pas. Le premier Ctrl+Z laisse T appliqué,
// le second annule l'action ; rétablir refait l'un puis l'autre.
func TestDetach(t *testing.T) {
	ctx := context.Background()
	db, bus := open(t, filepath.Join(t.TempDir(), "d.sqlite"))
	defer db.Close()
	var undone []string
	bus.Undone = func(name string) { undone = append(undone, name) }
	if _, err := bus.Do(ctx, "Saisie", func(tx *Tx) error {
		if err := insert(1, "tableau")(tx); err != nil {
			return err
		}
		mark := tx.Mark()
		// T : la valeur devient « libre ».
		q := `UPDATE t SET v = ? WHERE id = 1`
		if err := tx.Apply([]Stmt{S(q, "libre")}, []Stmt{S(q, "tableau")}); err != nil {
			return err
		}
		return tx.Detach(mark, "Création")
	}); err != nil {
		t.Fatal(err)
	}
	if v := value(t, db, 1); v != "tableau" {
		t.Fatalf("après l'action : %v", v)
	}
	if st, _ := bus.State(ctx); st.Undo != "Création" {
		t.Fatalf("état %+v", st)
	}
	steps := []struct {
		redo bool
		want any
	}{{false, "libre"}, {false, "absent"}, {true, "libre"}, {true, "tableau"}, {false, "libre"}}
	for i, s := range steps {
		var err error
		if s.redo {
			_, err = bus.Redo(ctx)
		} else {
			_, err = bus.Undo(ctx)
		}
		if err != nil {
			t.Fatal(err)
		}
		if v := value(t, db, 1); v != s.want {
			t.Fatalf("étape %d : %v, attendu %v", i, v, s.want)
		}
	}
	if len(undone) != 3 || undone[0] != "Création" || undone[1] != "Saisie" {
		t.Fatalf("annulations signalées : %v", undone)
	}
	// Dans un groupe (script), pas de coupe.
	gctx := WithGroup(ctx, "Script")
	if _, err := bus.Do(gctx, "Saisie", func(tx *Tx) error {
		if err := insert(2, "tableau")(tx); err != nil {
			return err
		}
		if tx.CanDetach() {
			t.Error("coupe permise dans un groupe")
		}
		return tx.Detach(0, "Création")
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if v := value(t, db, 2); v != "absent" {
		t.Fatalf("groupe : %v", v)
	}
}

// Profondeur réglée (limits.undoDepth, audit 6, M14) : lue à chaque
// commande quand le bus n'a pas la sienne.
func TestDepthSetting(t *testing.T) {
	ctx := context.Background()
	db, bus := open(t, filepath.Join(t.TempDir(), "d.sqlite"))
	defer db.Close()
	old := DepthSetting
	defer func() { DepthSetting = old }()
	if bus.depth() != DefaultDepth {
		t.Fatalf("profondeur par défaut : %d", bus.depth())
	}
	DepthSetting = func() int { return 2 }
	for i := 1; i <= 4; i++ {
		if _, err := bus.Do(ctx, "Action", func(tx *Tx) error {
			return tx.Apply([]Stmt{S(`INSERT INTO t (v) VALUES (?)`, i)}, []Stmt{S(`DELETE FROM t WHERE v = ?`, i)})
		}); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	for {
		if _, err := bus.Undo(ctx); err != nil {
			break
		}
		n++
	}
	if n != 2 {
		t.Fatalf("%d annulations possibles, attendu 2", n)
	}
	bus.Depth = 7
	if bus.depth() != 7 {
		t.Fatal("profondeur propre au bus ignorée")
	}
}
