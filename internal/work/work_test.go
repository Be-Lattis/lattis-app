package work

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type sink struct {
	mu    sync.Mutex
	steps []Step
}

func (s *sink) emit(st Step) {
	s.mu.Lock()
	s.steps = append(s.steps, st)
	s.mu.Unlock()
}

func (s *sink) all() []Step {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Step(nil), s.steps...)
}

func setTimes(t *testing.T, delay, every time.Duration) {
	d, e := Delay, Every
	Delay, Every = delay, every
	t.Cleanup(func() { Delay, Every = d, e })
}

// TestShortOperationSilent : une opération plus courte que Delay n'émet
// rien, pas même sa fin.
func TestShortOperationSilent(t *testing.T) {
	setTimes(t, time.Hour, 0)
	var s sink
	ctx, tr := Start(context.Background(), s.emit, false)
	Op(ctx, "sort", "Prix")
	Phase(ctx, "rank")
	tr.End()
	if got := s.all(); len(got) != 0 {
		t.Fatalf("émis : %+v", got)
	}
}

// TestStepsAndEnd : après Delay, l'état courant part, puis chaque
// changement ; la fin arrive en dernier ; une opération intérieure écrit
// dans celle du dessus.
func TestStepsAndEnd(t *testing.T) {
	setTimes(t, 0, 0)
	var s sink
	ctx, tr := Start(context.Background(), s.emit, true)
	time.Sleep(20 * time.Millisecond) // première émission (minuterie)
	Op(ctx, "deleteColumn", "Prix")
	inner, none := Start(ctx, s.emit, true)
	if none != nil || From(inner) != tr {
		t.Fatal("une opération intérieure ne doit pas commencer de suivi")
	}
	Op(inner, "write", "") // ignoré : l'action a déjà un nom
	Statement(inner, "INSERT INTO _lattis_trash_1_2_3 (id, v) SELECT id, c_2 FROM t_1")
	Statement(inner, "UPDATE _lattis_columns SET pos = pos - 1") // sans étape
	Loop(ctx, "cells", 10)
	Statement(ctx, "UPDATE t_1 SET c_2 = ? WHERE id = ?") // tenue par Loop
	Progress(ctx, 4, 10)
	tr.End()
	tr.End()
	got := s.all()
	last := got[len(got)-1]
	if !last.End || last.Op != "deleteColumn" || last.Name != "Prix" || !last.Cancel {
		t.Fatalf("fin : %+v", last)
	}
	var phases []string
	for _, st := range got {
		if st.Phase != "" && (len(phases) == 0 || phases[len(phases)-1] != st.Phase) {
			phases = append(phases, st.Phase)
		}
	}
	if want := []string{"keep", "cells"}; len(phases) != 2 || phases[0] != want[0] || phases[1] != want[1] {
		t.Fatalf("étapes : %v (%+v)", phases, got)
	}
	if p := got[len(got)-2]; p.Done != 4 || p.Total != 10 {
		t.Fatalf("avancement : %+v", p)
	}
}

// TestEveryThrottles : des changements rapprochés ne partent pas tous ; le
// dernier part quand même.
func TestEveryThrottles(t *testing.T) {
	setTimes(t, 0, 50*time.Millisecond)
	var s sink
	ctx, tr := Start(context.Background(), s.emit, false)
	time.Sleep(10 * time.Millisecond)
	for i := range 1000 {
		Progress(ctx, int64(i+1), 1000)
	}
	time.Sleep(120 * time.Millisecond)
	got := s.all()
	if len(got) > 5 {
		t.Fatalf("%d émissions", len(got))
	}
	if p := got[len(got)-1]; p.Done != 1000 {
		t.Fatalf("dernier état perdu : %+v", p)
	}
	tr.End()
}

// TestCancel : seule une opération annulable s'interrompt ; son contexte
// porte la cause ErrCancelled.
func TestCancel(t *testing.T) {
	ctx, tr := Start(context.Background(), func(Step) {}, false)
	if tr.Cancel() || ctx.Err() != nil {
		t.Fatal("opération non annulable interrompue")
	}
	tr.End()
	ctx, tr = Start(context.Background(), func(Step) {}, true)
	if !tr.Cancel() || !tr.Cancelled() || !errors.Is(context.Cause(ctx), ErrCancelled) {
		t.Fatal("annulation")
	}
	tr.End()
	var none *Tracker
	if none.Cancel() || none.Cancelled() || none.ID() != 0 {
		t.Fatal("suivi nil")
	}
	none.End()
}

func TestClassify(t *testing.T) {
	for q, want := range map[string]string{
		"CREATE TABLE _lattis_trash_1 (id INTEGER PRIMARY KEY, v)":     "",
		"CREATE TABLE _lattis_trash_1_inv AS SELECT rid, raw FROM x":   "keep",
		"INSERT INTO t_3_rebuild (id, _pos) SELECT id, _pos FROM t_3":  "rewrite",
		"ALTER TABLE t_3 DROP COLUMN c_2":                              "rewrite",
		"ALTER TABLE t_3 ADD COLUMN c_9 TEXT":                          "schema",
		"INSERT INTO _lattis_rank_3_1 SELECT id, printf(...) FROM t_3": "rank",
		"UPDATE t_3 SET _pos = r.k FROM r":                             "order",
		"UPDATE t_3 SET c_1 = ?":                                       "update",
		"CREATE INDEX t_3_pos ON t_3(_pos)":                            "index",
		"DROP TABLE _lattis_trash_1":                                   "drop",
		"SELECT 1":                                                     "",
	} {
		if got := Classify(q); got != want {
			t.Errorf("%s : %q, attendu %q", q, got, want)
		}
	}
}
