package oplog_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/remote/oplog/oplogtest"
	"github.com/lalamefine/lattis/internal/store"
)

func TestMain(m *testing.M) {
	store.SetSQLLog(false)
	os.Exit(m.Run())
}

var S = command.S

type fixture struct {
	t    *testing.T
	path string
	db   *store.Store
	bus  *command.Bus
	m    *oplogtest.Mirror
	rec  *oplog.Recorder
}

// open crée un classeur avec le tableau 1 (t_1 : c_1 texte, c_2 réel), trois
// lignes, publié ou non.
func open(t *testing.T, publish bool) *fixture {
	t.Helper()
	f := &fixture{t: t, path: filepath.Join(t.TempDir(), "doc.sqlite"), m: oplogtest.New()}
	f.reopen()
	err := f.db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`
INSERT INTO _lattis_tables (id, sheet, name, row, col) VALUES (1, 1, 'Ventes', 0, 0);
INSERT INTO _lattis_columns (tbl, id, name, type, pos) VALUES (1, 1, 'Produit', 'text', 0), (1, 2, 'Prix', 'decimal', 1);
CREATE TABLE t_1 (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 TEXT, c_2 REAL) STRICT;
INSERT INTO t_1 VALUES (1, 'a', 'Café', 3.5), (2, 'b', 'Thé', NULL), (3, 'c', 'Miel', 7);`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if publish {
		if err := oplog.Publish(context.Background(), f.db, 1); err != nil {
			t.Fatal(err)
		}
		f.m.Tables[1] = map[string]*oplogtest.Row{}
		for rid, vals := range map[int64][2]string{1: {"a", "s:Café|n:3.5"}, 2: {"b", "s:Thé"}, 3: {"c", "s:Miel|n:7"}} {
			uid, err := oplog.RowID(context.Background(), f.db, 1, rid)
			if err != nil || len(uid) != 36 || uid[14] != '7' {
				t.Fatalf("identifiant de la ligne %d : %q %v", rid, uid, err)
			}
			r := &oplogtest.Row{Pos: vals[0], Vals: map[int64]string{}}
			for i, v := range strings.Split(vals[1], "|") {
				r.Vals[int64(i+1)] = v
			}
			f.m.Tables[1][uid] = r
		}
	}
	return f
}

func (f *fixture) reopen() {
	f.t.Helper()
	if f.db != nil {
		f.db.Close()
	}
	db, err := store.Open(f.path)
	if err != nil {
		f.t.Fatal(err)
	}
	if f.db == nil {
		f.t.Cleanup(func() { f.db.Close() })
	}
	if err := meta.Migrate(context.Background(), db); err != nil {
		f.t.Fatal(err)
	}
	f.db, f.bus = db, command.New(db)
	f.rec = oplog.New(db)
	f.bus.Recorder = f.rec
}

// do applique des instructions (et leurs inverses) en une commande.
func (f *fixture) do(do, undo []command.Stmt) int64 {
	f.t.Helper()
	seq, err := f.bus.Do(context.Background(), "test", func(tx *command.Tx) error { return tx.Apply(do, undo) })
	if err != nil {
		f.t.Fatal(err)
	}
	return seq
}

// last donne les opérations des entrées de la file ajoutées depuis n.
func (f *fixture) since(n int) ([]oplog.Entry, []command.Op) {
	f.t.Helper()
	es, err := oplog.Entries(context.Background(), f.db)
	if err != nil {
		f.t.Fatal(err)
	}
	var ops []command.Op
	for _, e := range es[n:] {
		o, err := command.DecodeOps(e.Ops)
		if err != nil {
			f.t.Fatal(err)
		}
		ops = append(ops, o...)
	}
	return es[n:], ops
}

func (f *fixture) count() int {
	es, _ := oplog.Entries(context.Background(), f.db)
	return len(es)
}

func (f *fixture) check(what string) {
	f.t.Helper()
	if err := f.m.Sync(context.Background(), f.db); err != nil {
		f.t.Fatalf("%s : %v", what, err)
	}
	if err := f.m.Check(context.Background(), f.db); err != nil {
		f.t.Fatalf("%s : %v", what, err)
	}
}

func kinds(ops []command.Op) string {
	var out []string
	for _, op := range ops {
		s := op.Kind
		if op.Kind == command.OpMeta {
			s += ":" + op.Meta.Object + "." + op.Meta.Action
		}
		out = append(out, s)
	}
	return strings.Join(out, " ")
}

func TestUnpublishedTableWritesNothing(t *testing.T) {
	f := open(t, false)
	f.do([]command.Stmt{S(`UPDATE t_1 SET c_2 = 1`)}, []command.Stmt{S(`UPDATE t_1 SET c_2 = NULL`)})
	f.bus.Undo(context.Background())
	if n := f.count(); n != 0 {
		t.Fatalf("%d entrées pour un tableau non publié", n)
	}
	var n int
	f.db.Read().QueryRow(`SELECT count(*) FROM _lattis_row_ids`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d identifiants pour un tableau non publié", n)
	}
}

func TestDataOps(t *testing.T) {
	f := open(t, true)
	u1, _ := oplog.RowID(context.Background(), f.db, 1, 1)
	seq := f.do([]command.Stmt{
		S(`UPDATE t_1 SET c_2 = 4 WHERE id = 1`),
		S(`UPDATE t_1 SET c_1 = c_1 WHERE id = 3`), // sans changement : rien
		S(`INSERT INTO t_1 VALUES (4, 'd', 'Sel', 2)`),
		S(`DELETE FROM t_1 WHERE id = 2`),
		S(`UPDATE t_1 SET _pos = 'e' WHERE id = 1`),
		S(`INSERT INTO t_1 VALUES (5, 'f', 'Riz', NULL)`),
		S(`DELETE FROM t_1 WHERE id = 5`), // ajoutée puis supprimée : rien
	}, nil)
	es, ops := f.since(0)
	if len(es) != 1 || es[0].Seq != seq || es[0].Kind != command.KindDo || es[0].Batch != es[0].ID || es[0].State != "pending" {
		t.Fatalf("entrée : %+v", es)
	}
	if got := kinds(ops); got != "set move delete insert" {
		t.Fatalf("opérations : %s", got)
	}
	if ops[0].Row != u1 || ops[0].Col != 2 || ops[0].Old != json.Number("3.5") || ops[0].New != json.Number("4") {
		t.Fatalf("set : %+v", ops[0])
	}
	if ops[1].Old != "a" || ops[1].Pos != "e" {
		t.Fatalf("move : %+v", ops[1])
	}
	if v := ops[3].New.(command.Values); v[1] != "Sel" || v[2] != json.Number("2") || ops[3].Pos != "d" || ops[3].Row == "" {
		t.Fatalf("insert : %+v", ops[3])
	}
	if u, _ := oplog.RowID(context.Background(), f.db, 1, 2); u != "" {
		t.Fatalf("ligne supprimée : identifiant %q gardé", u)
	}
	f.check("commande")
}

// TestUndoRedo : l'annulation et le rétablissement envoient les opérations
// inverses, conditionnées à ce que la commande avait écrit.
func TestUndoRedo(t *testing.T) {
	ctx := context.Background()
	f := open(t, true)
	f.do([]command.Stmt{S(`UPDATE t_1 SET c_2 = 10 WHERE id = 1`), S(`DELETE FROM t_1 WHERE id = 2`)},
		[]command.Stmt{S(`UPDATE t_1 SET c_2 = 3.5 WHERE id = 1`), S(`INSERT INTO t_1 VALUES (2, 'b', 'Thé', NULL)`)})
	f.check("commande")
	if _, err := f.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	es, ops := f.since(1)
	if len(es) != 1 || es[0].Kind != command.KindUndo || kinds(ops) != "set insert" {
		t.Fatalf("annulation : %+v %s", es, kinds(ops))
	}
	f.check("annulation")
	if _, err := f.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	f.check("rétablissement")
	if _, err := f.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	f.check("deuxième annulation")

	// Une valeur reçue d'un collègue (hors journal) entre la saisie et son
	// annulation : la cellule n'est pas annulée, elle garde la valeur du
	// collègue (§7) ; le reste de l'étape l'est.
	f.do([]command.Stmt{S(`UPDATE t_1 SET c_2 = 20 WHERE id = 3`), S(`UPDATE t_1 SET c_1 = 'Lait' WHERE id = 3`)},
		[]command.Stmt{S(`UPDATE t_1 SET c_2 = 7 WHERE id = 3`), S(`UPDATE t_1 SET c_1 = 'Sucre' WHERE id = 3`)})
	n := f.count()
	err := f.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE t_1 SET c_2 = 21 WHERE id = 3`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	_, ops = f.since(n)
	if len(ops) != 1 || ops[0].Col != 1 || ops[0].Old != "Lait" || ops[0].New != "Sucre" {
		t.Fatalf("annulation après une modification reçue : %+v", ops)
	}
	if k := f.rec.TakeSkipped(); k != 1 {
		t.Fatalf("cellules gardées : %d", k)
	}
	var v int64
	if err := f.db.Read().QueryRow(`SELECT c_2 FROM t_1 WHERE id = 3`).Scan(&v); err != nil || v != 21 {
		t.Fatalf("cellule du collègue : %d (%v)", v, err)
	}
	// Une annulation sans écriture d'un collègue garde la condition « la
	// cellule vaut toujours ce que j'y ai mis ».
	f.do([]command.Stmt{S(`UPDATE t_1 SET c_2 = 30 WHERE id = 3`)}, []command.Stmt{S(`UPDATE t_1 SET c_2 = 21 WHERE id = 3`)})
	n = f.count()
	if _, err := f.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ops = f.since(n); len(ops) != 1 || ops[0].Old != json.Number("30") || ops[0].New != json.Number("21") {
		t.Fatalf("annulation conditionnée : %+v", ops)
	}
	// L'écriture hors journal n'est pas renvoyée par la commande suivante.
	n = f.count()
	f.do([]command.Stmt{S(`UPDATE t_1 SET c_1 = 'Lait' WHERE id = 1`)}, nil)
	if _, ops = f.since(n); kinds(ops) != "set" || ops[0].Col != 1 {
		t.Fatalf("après une écriture hors journal : %+v", ops)
	}
}

// TestRebuild : un tableau reconstruit (nouvelle table, copie, DROP,
// RENAME) est comparé en entier ; un type changé est une opération de
// structure, pas une suite de valeurs.
func TestRebuild(t *testing.T) {
	f := open(t, true)
	rebuild := func(typ, expr string) []command.Stmt {
		return []command.Stmt{
			S(`CREATE TABLE t_1_rebuild (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 TEXT, c_2 ` + typ + `) STRICT`),
			S(`INSERT INTO t_1_rebuild SELECT id, _pos, c_1, ` + expr + ` FROM t_1`),
			S(`DROP TABLE t_1`),
			S(`ALTER TABLE t_1_rebuild RENAME TO t_1`),
		}
	}
	f.do(append(rebuild("REAL", "c_2"), S(`UPDATE t_1 SET c_1 = 'Sucre' WHERE id = 2`)), nil)
	if _, ops := f.since(0); kinds(ops) != "set" {
		t.Fatalf("reconstruction et saisie : %s", kinds(ops))
	}
	f.check("reconstruction")
	n := f.count()
	f.do(append(rebuild("TEXT", "CAST(c_2 AS TEXT)"), S(`UPDATE _lattis_columns SET type = 'text' WHERE tbl = 1 AND id = 2`)), nil)
	if _, ops := f.since(n); kinds(ops) != "meta:column.retype" {
		t.Fatalf("changement de type : %s", kinds(ops))
	}
	n = f.count()
	f.do([]command.Stmt{S(`ALTER TABLE t_1 ADD COLUMN c_3 INTEGER`), S(`INSERT INTO _lattis_columns (tbl, id, name, type, pos) VALUES (1, 3, 'Qté', 'integer', 2)`), S(`UPDATE t_1 SET c_3 = 1`)}, nil)
	if _, ops := f.since(n); kinds(ops) != "meta:column.add set set set" {
		t.Fatalf("colonne ajoutée : %s", kinds(ops))
	}
	n = f.count()
	f.do([]command.Stmt{S(`ALTER TABLE t_1 DROP COLUMN c_1`), S(`DELETE FROM _lattis_columns WHERE tbl = 1 AND id = 1`)}, nil)
	if _, ops := f.since(n); kinds(ops) != "meta:column.remove" {
		t.Fatalf("colonne supprimée : %s", kinds(ops))
	}
	n = f.count()
	f.do([]command.Stmt{S(`ALTER TABLE t_1 RENAME TO _lattis_trash_1`)}, []command.Stmt{S(`ALTER TABLE _lattis_trash_1 RENAME TO t_1`)})
	if _, ops := f.since(n); kinds(ops) != "meta:table.drop" {
		t.Fatalf("tableau supprimé : %s", kinds(ops))
	}
	n = f.count()
	if _, err := f.bus.Undo(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ops := f.since(n); kinds(ops) != "meta:table.create insert insert insert" {
		t.Fatalf("tableau rétabli : %s", kinds(ops))
	}
}

// TestRecordedMeta : les opérations enregistrées par Tx.Record sur un
// tableau publié partent avec la commande ; l'annulation les inverse.
func TestRecordedMeta(t *testing.T) {
	ctx := context.Background()
	f := open(t, true)
	op := command.Op{Kind: command.OpMeta, Table: 1, Meta: &command.MetaChange{Object: "width", Action: "set", ID: 2, Before: 80, After: 120}}
	other := command.Op{Kind: command.OpMeta, Table: 9, Meta: &command.MetaChange{Object: "width", Action: "set"}}
	_, err := f.bus.Do(ctx, "largeur", func(tx *command.Tx) error {
		tx.Record(op, other)
		return tx.Apply([]command.Stmt{S(`UPDATE _lattis_tables SET row = 1 WHERE id = 1`)}, []command.Stmt{S(`UPDATE _lattis_tables SET row = 0 WHERE id = 1`)})
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ops := f.since(0); len(ops) != 1 || ops[0].Meta.After != json.Number("120") {
		t.Fatalf("commande : %+v", ops)
	}
	if _, err := f.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ops := f.since(1); len(ops) != 1 || ops[0].Meta.After != json.Number("80") {
		t.Fatalf("annulation : %+v", ops)
	}
}

// TestLargeStep : une grande étape est rangée en plusieurs entrées de la
// même transaction (batch) ; une ligne au-delà des clés d'avant n'est pas
// relue dans l'instantané d'avant.
func TestLargeStep(t *testing.T) {
	f := open(t, true)
	n := 12000
	f.do([]command.Stmt{S(`WITH RECURSIVE s(i) AS (SELECT 10 UNION ALL SELECT i + 1 FROM s WHERE i < ?) INSERT INTO t_1 SELECT i, printf('z%08d', i), 'x', i FROM s`, n+9)}, nil)
	es, ops := f.since(0)
	if len(es) != 3 || len(ops) != n {
		t.Fatalf("%d entrées, %d opérations", len(es), len(ops))
	}
	for _, e := range es {
		if e.Batch != es[0].ID {
			t.Fatalf("entrées de lots différents : %+v", es)
		}
	}
	f.check("grande étape")
	if _, err := f.bus.Undo(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.check("grande étape annulée")
}

// TestSeqReuse : un numéro de commande qui resert (commande annulée puis
// remplacée) ne rattache plus les entrées de l'ancienne commande.
func TestSeqReuse(t *testing.T) {
	ctx := context.Background()
	f := open(t, true)
	s1 := f.do([]command.Stmt{S(`UPDATE t_1 SET c_2 = 1 WHERE id = 1`)}, []command.Stmt{S(`UPDATE t_1 SET c_2 = 3.5 WHERE id = 1`)})
	f.bus.Undo(ctx)
	s2 := f.do([]command.Stmt{S(`UPDATE t_1 SET c_2 = 2 WHERE id = 3`)}, []command.Stmt{S(`UPDATE t_1 SET c_2 = 7 WHERE id = 3`)})
	if s1 != s2 {
		t.Skipf("numéro non réutilisé (%d, %d)", s1, s2)
	}
	es, _ := f.since(0)
	if es[0].Seq != 0 || es[1].Seq != 0 || es[2].Seq != s2 {
		t.Fatalf("file : %+v", es)
	}
	if _, err := f.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	f.check("annulation de la nouvelle commande")
}

// TestReopen : les déclencheurs de relevé sont temporaires (connexion
// d'écriture) ; ils sont reposés après une réouverture, et retirés quand le
// tableau n'est plus publié.
func TestReopen(t *testing.T) {
	ctx := context.Background()
	f := open(t, true)
	f.reopen()
	f.do([]command.Stmt{S(`UPDATE t_1 SET c_1 = 'Sel' WHERE id = 1`)}, nil)
	if f.count() != 1 {
		t.Fatal("écriture non relevée après réouverture")
	}
	f.check("réouverture")
	if err := oplog.Unpublish(ctx, f.db, 1); err != nil {
		t.Fatal(err)
	}
	f.do([]command.Stmt{S(`UPDATE t_1 SET c_1 = 'Riz' WHERE id = 1`)}, nil)
	if f.count() != 1 {
		t.Fatal("écriture relevée sur un tableau dépublié")
	}
	var trg int
	f.db.Write(ctx, func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM temp.sqlite_master WHERE type = 'trigger'`).Scan(&trg)
	})
	if trg != 0 {
		t.Fatalf("%d déclencheurs restent", trg)
	}
	if ok, _ := oplog.Published(ctx, f.db, 1); ok {
		t.Fatal("toujours publié")
	}
}
