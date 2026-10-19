package command

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOpsRoundTrip(t *testing.T) {
	ops := []Op{
		{Kind: OpSet, Table: 3, Row: "u1", Col: 2, Old: int64(1000), New: 1200.5},
		{Kind: OpSet, Table: 3, Row: "u1", Col: 3, Old: nil, New: "texte"},
		{Kind: OpInsert, Table: 3, Row: "u2", New: Values{1: "Café", 2: int64(math.MaxInt64 - 7)}, Pos: "aV"},
		{Kind: OpDelete, Table: 3, Row: "u3", Old: Values{1: "Thé"}},
		{Kind: OpMove, Table: 3, Row: "u2", Old: "aV", Pos: "b"},
		{Kind: OpMeta, Table: 3, Meta: &MetaChange{Object: "column", Action: "add", ID: 4, After: "text"}},
		{Kind: OpSet, Table: 3, Row: "u1", Col: 2, Old: math.Inf(1), New: 0.0},
	}
	data, err := EncodeOps(ops)
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeOps(data)
	if err != nil || len(back) != len(ops) {
		t.Fatalf("%v %d", err, len(back))
	}
	if back[0].Old != json.Number("1000") || back[0].New != json.Number("1200.5") || back[1].Old != nil || back[1].New != "texte" {
		t.Fatalf("set : %+v %+v", back[0], back[1])
	}
	ins, ok := back[2].New.(Values)
	if !ok || ins[1] != "Café" || ins[2] != json.Number("9223372036854775800") || back[2].Pos != "aV" {
		t.Fatalf("insert : %#v", back[2])
	}
	if del, ok := back[3].Old.(Values); !ok || del[1] != "Thé" {
		t.Fatalf("delete : %#v", back[3])
	}
	if back[4].Old != "aV" || back[4].Pos != "b" {
		t.Fatalf("move : %#v", back[4])
	}
	if back[5].Meta == nil || back[5].Meta.Action != "add" || back[5].Meta.ID != 4 {
		t.Fatalf("meta : %#v", back[5])
	}
	if back[6].Old != "Infinity" || back[6].New != json.Number("0") {
		t.Fatalf("infini : %#v", back[6])
	}
}

func TestOpInverse(t *testing.T) {
	cases := []struct{ op, inv Op }{
		{Op{Kind: OpSet, Row: "u", Col: 1, Old: 1, New: 2}, Op{Kind: OpSet, Row: "u", Col: 1, Old: 2, New: 1}},
		{Op{Kind: OpInsert, Row: "u", New: Values{1: "a"}, Pos: "p"}, Op{Kind: OpDelete, Row: "u", Old: Values{1: "a"}, Pos: "p"}},
		{Op{Kind: OpDelete, Row: "u", Old: Values{1: "a"}}, Op{Kind: OpInsert, Row: "u", New: Values{1: "a"}}},
		{Op{Kind: OpMove, Row: "u", Old: "a", Pos: "b"}, Op{Kind: OpMove, Row: "u", Old: "b", Pos: "a"}},
		{Op{Kind: OpMeta, Meta: &MetaChange{Object: "column", Action: "add", ID: 2, After: "text"}}, Op{Kind: OpMeta, Meta: &MetaChange{Object: "column", Action: "remove", ID: 2, Before: "text"}}},
	}
	for _, c := range cases {
		if got := c.op.Inverse(); !reflect.DeepEqual(got, c.inv) {
			t.Errorf("inverse de %+v : %+v, attendu %+v", c.op, got, c.inv)
		}
	}
}

// recorder note ce que le bus lui signale.
type recorder struct {
	stmts   []string
	commits []Commit
}

func (r *recorder) Begin(ctx context.Context, tx *sql.Tx) (Capture, error) { return r, nil }
func (r *recorder) Statement(q string) error                               { r.stmts = append(r.stmts, q); return nil }
func (r *recorder) Finish(c Commit) error                                  { r.commits = append(r.commits, c); return nil }

// TestRecorderSeesEveryTransaction : Do, Append, Undo et Redo passent par le
// Recorder, avec leurs instructions ; les opérations enregistrées (Record)
// sont gardées dans le journal et inversées par l'annulation.
func TestRecorderSeesEveryTransaction(t *testing.T) {
	ctx := context.Background()
	db, bus := open(t, filepath.Join(t.TempDir(), "r.sqlite"))
	t.Cleanup(func() { db.Close() })
	r := &recorder{}
	bus.Recorder = r
	meta := Op{Kind: OpMeta, Table: 1, Meta: &MetaChange{Object: "style", Action: "set", Before: "a", After: "b"}}
	seq, err := bus.Do(ctx, "saisie", func(tx *Tx) error {
		tx.Record(meta)
		return insert(1, "x")(tx)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bus.Append(ctx, seq, "saisie", insert(2, "y")); err != nil {
		t.Fatal(err)
	}
	if _, err := bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		kind string
		new  bool
		ops  int
	}{{KindDo, true, 1}, {KindDo, false, 0}, {KindUndo, false, 1}, {KindRedo, false, 1}}
	if len(r.commits) != len(want) {
		t.Fatalf("%d transactions vues : %+v", len(r.commits), r.commits)
	}
	for i, w := range want {
		c := r.commits[i]
		if c.Seq != seq || c.Kind != w.kind || c.New != w.new || len(c.Ops) != w.ops {
			t.Fatalf("transaction %d : %+v, attendu %+v", i, c, w)
		}
	}
	if m := r.commits[2].Ops[0].Meta; m.Before != "b" || m.After != "a" {
		t.Fatalf("annulation : %+v", m)
	}
	if m := r.commits[3].Ops[0].Meta; m.Before != "a" || m.After != "b" {
		t.Fatalf("rétablissement : %+v", m)
	}
	// Instructions : 2 faites, 2 annulées, 2 rétablies.
	if len(r.stmts) != 6 {
		t.Fatalf("instructions vues : %q", r.stmts)
	}
}
