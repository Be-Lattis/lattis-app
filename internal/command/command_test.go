package command

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"path/filepath"
	"testing"

	"github.com/lalamefine/slitable/internal/store"
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
