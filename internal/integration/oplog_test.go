package integration

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/remote/oplog/oplogtest"
)

// Journal logique d'un classeur partagé (docs/collaboration/journal-logique.md) :
// toute commande qui touche un tableau publié produit des opérations, et
// ces opérations, rejouées par un « serveur » (oplogtest.Mirror, écritures
// conditionnées aux valeurs vues), donnent exactement le contenu local.

// shared suit les tableaux publiés d'un classeur et leur miroir.
type shared struct {
	h *host
	m *oplogtest.Mirror
}

// publish publie le tableau nommé et ouvre le miroir.
func publish(h *host, tables ...string) *shared {
	h.t.Helper()
	ctx := context.Background()
	for _, name := range tables {
		var id int64
		if err := h.d.DB().Read().QueryRowContext(ctx, `SELECT id FROM _lattis_tables WHERE name = ?`, name).Scan(&id); err != nil {
			h.t.Fatalf("tableau %s : %v", name, err)
		}
		if err := oplog.Publish(ctx, h.d.DB(), id); err != nil {
			h.t.Fatal(err)
		}
	}
	s := &shared{h: h, m: oplogtest.New()}
	// La publication copie le tableau sur le serveur (COPY) : le miroir part
	// de l'état local.
	s.seed()
	return s
}

// seed remplit le miroir avec le contenu des tableaux publiés.
func (s *shared) seed() {
	ctx := context.Background()
	db := s.h.d.DB()
	rs, err := db.Read().QueryContext(ctx, `SELECT tbl FROM _lattis_remote_tables WHERE published = 1`)
	if err != nil {
		s.h.t.Fatal(err)
	}
	var ids []int64
	for rs.Next() {
		var id int64
		rs.Scan(&id)
		ids = append(ids, id)
	}
	rs.Close()
	for _, id := range ids {
		t := map[string]*oplogtest.Row{}
		s.m.Tables[id] = t
		cols := []int64{}
		crs, err := db.Read().QueryContext(ctx, `SELECT id FROM _lattis_columns WHERE tbl = ? AND (formula IS NULL OR formula = '')`, id)
		if err != nil {
			s.h.t.Fatal(err)
		}
		for crs.Next() {
			var c int64
			crs.Scan(&c)
			cols = append(cols, c)
		}
		crs.Close()
		sel := "r.lattis_id, t._pos"
		for _, c := range cols {
			sel += ", t.c_" + strconv.FormatInt(c, 10)
		}
		rrs, err := db.Read().QueryContext(ctx, `SELECT `+sel+` FROM t_`+strconv.FormatInt(id, 10)+` AS t JOIN _lattis_row_ids AS r ON r.tbl = ? AND r.rid = t.id`, id)
		if err != nil {
			s.h.t.Fatal(err)
		}
		for rrs.Next() {
			var uid, pos string
			vals := make([]any, len(cols))
			dest := []any{&uid, &pos}
			for i := range vals {
				dest = append(dest, &vals[i])
			}
			if err := rrs.Scan(dest...); err != nil {
				s.h.t.Fatal(err)
			}
			r := &oplogtest.Row{Pos: pos, Vals: map[int64]string{}}
			for i, v := range vals {
				if n, ok := oplogtest.Norm(v); ok {
					r.Vals[cols[i]] = n
				}
			}
			t[uid] = r
		}
		rrs.Close()
	}
}

// check rejoue la file d'envoi et compare le miroir au classeur.
func (s *shared) check(what string) {
	s.h.t.Helper()
	ctx := context.Background()
	if err := s.m.Sync(ctx, s.h.d.DB()); err != nil {
		s.h.t.Fatalf("%s : file d'envoi : %v", what, err)
	}
	if err := s.m.Check(ctx, s.h.d.DB()); err != nil {
		s.h.t.Fatalf("%s : %v", what, err)
	}
}

// data décrit le contenu des tableaux publiés (toutes leurs colonnes).
func (s *shared) data() string {
	s.h.t.Helper()
	ctx := context.Background()
	db := s.h.d.DB()
	var b strings.Builder
	rs, err := db.Read().QueryContext(ctx, `SELECT tbl FROM _lattis_remote_tables WHERE published = 1 ORDER BY tbl`)
	if err != nil {
		s.h.t.Fatal(err)
	}
	var ids []int64
	for rs.Next() {
		var id int64
		rs.Scan(&id)
		ids = append(ids, id)
	}
	rs.Close()
	for _, id := range ids {
		name := "t_" + strconv.FormatInt(id, 10)
		var n int
		db.Read().QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name = ?`, name).Scan(&n)
		if n == 0 {
			fmt.Fprintf(&b, "%s absent\n", name)
			continue
		}
		rrs, err := db.Read().QueryContext(ctx, `SELECT * FROM `+name+` ORDER BY id`)
		if err != nil {
			s.h.t.Fatal(err)
		}
		cols, _ := rrs.Columns()
		fmt.Fprintf(&b, "%s %v\n", name, cols)
		for rrs.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			rrs.Scan(ptrs...)
			for i, v := range vals {
				if bs, ok := v.([]byte); ok {
					vals[i] = string(bs)
				}
			}
			fmt.Fprintf(&b, "  %v\n", vals)
		}
		rrs.Close()
	}
	return b.String()
}

func (s *shared) outboxLen() int {
	var n int
	s.h.d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_outbox`).Scan(&n)
	return n
}

// step exécute une action : si elle change un tableau publié, elle doit
// produire des opérations, et le miroir doit suivre ; de même pour son
// annulation et son rétablissement.
func (s *shared) step(what string, fn func()) {
	s.h.t.Helper()
	for _, phase := range []string{"", " (annulé)", " (rétabli)"} {
		before, n := s.data(), s.outboxLen()
		switch phase {
		case "":
			fn()
		case " (annulé)":
			s.h.undo()
		default:
			s.h.redo()
		}
		if after := s.data(); after != before && s.outboxLen() == n {
			s.h.t.Fatalf("%s%s : tableau publié modifié sans opération logique\navant :\n%s\naprès :\n%s", what, phase, before, after)
		}
		s.check(what + phase)
	}
}

// TestOplogCoversCommands : chaque commande qui écrit dans un tableau
// (inventaire de docs/collaboration/journal-logique.md) produit les
// opérations logiques qui décrivent son effet, et son annulation et son
// rétablissement aussi.
func TestOplogCoversCommands(t *testing.T) {
	h := newHost(t)
	ctx := context.Background()
	ventes(h)
	s := publish(h, "Ventes")
	s.check("publication")
	sheet := h.sheet("").ID

	s.step("saisie", func() { h.set(2, 2, 9.5) })
	s.step("saisie d'un texte", func() { h.set(3, 1, "Thé vert") })
	s.step("effacement d'une cellule", func() { h.set(3, 1, "") })
	s.step("saisie sous le tableau", func() { h.set(6, 1, "Miel") })
	s.step("collage", func() { h.paste(2, 1, []any{"Café", 3}, []any{"Thé", 4}) })
	s.step("collage qui agrandit", func() { h.paste(7, 1, []any{"Sel", 1, 2}, []any{"Riz", 2, 3}) })
	s.step("insertion de ligne", func() { h.do("tables.insert_row", "table", "Ventes", "row", 2) })
	s.step("suppression de ligne", func() { h.do("tables.delete_row", "table", "Ventes", "row", 3) })
	s.step("tri", func() { h.do("tables.sort", "table", "Ventes", "column", "Prix", "descending", true) })
	s.step("doublon", func() { h.paste(10, 1, []any{"Sel", 1, 2}) })
	s.step("dédoublonnage", func() { h.do("tables.remove_duplicates", "table", "Ventes") })
	s.step("remplacement", func() { h.do("sheets.replace", "text", "Riz", "by", "Orge") })
	s.step("effacement d'une plage", func() { h.do("cells.clear", "row", 2, "col", 2, "row2", 3, "col2", 3) })
	s.step("SQL INSERT", func() {
		h.do("db.execute", "sql", `INSERT INTO Ventes (Produit, Prix, "Qté") VALUES (?, ?, ?)`, "params", []any{"Sucre", 2, 7})
	})
	s.step("SQL UPDATE", func() { h.do("db.execute", "sql", `UPDATE Ventes SET Prix = coalesce(Prix, 0) * 2`) })
	s.step("SQL DELETE", func() { h.do("db.execute", "sql", `DELETE FROM Ventes WHERE Produit = 'Sucre'`) })
	s.step("script", func() {
		h.lua(`for l in db.rows("SELECT _id, Prix FROM Ventes WHERE Prix IS NOT NULL") do db.update("Ventes", l._id, {Prix = l.Prix + 1}) end
local id = db.insert("Ventes", {Produit = "Lait", Prix = 1})
db.delete("Ventes", id)
db.insert("Ventes", {Produit = "Beurre", Prix = 3})`)
	})
	s.step("insertion de lignes de la feuille", func() {
		if err := h.d.InsertSheetRows(ctx, sheet, 2, 2); err != nil {
			t.Fatal(err)
		}
	})
	s.step("suppression de lignes de la feuille", func() {
		if err := h.d.DeleteSheetRows(ctx, sheet, 2, 3); err != nil {
			t.Fatal(err)
		}
	})
	s.step("déplacement de cellules", func() {
		_, err := h.d.Transfer(ctx, sheet, canvas.Rect{R0: 1, C0: 1, R1: 2, C1: 1}, sheet, canvas.Rect{R0: 3, C0: 1, R1: 4, C1: 1}, true, nil)
		if err != nil {
			t.Fatal(err)
		}
	})
	s.step("copie de cellules", func() {
		_, err := h.d.Transfer(ctx, sheet, canvas.Rect{R0: 3, C0: 0, R1: 3, C1: 0}, sheet, canvas.Rect{R0: 1, C0: 0, R1: 2, C1: 0}, false, nil)
		if err != nil {
			t.Fatal(err)
		}
	})
	s.step("effacement par la feuille", func() {
		if err := h.d.ClearRange(ctx, sheet, 1, 0, 1, 2, false); err != nil {
			t.Fatal(err)
		}
	})
	s.step("conversion de texte en colonnes", func() {
		h.set(2, 1, "a-b")
		h.lua(`tables.text_to_columns("Ventes", "Produit", {delims = "-"})`)
	})
	s.step("insertion de colonne", func() { h.do("columns.insert", "table", "Ventes", "name", "Note", "position", 2) })
	s.step("saisie dans la nouvelle colonne", func() { h.set(2, 2, "bon") })
	s.step("suppression de colonne", func() { h.do("columns.delete", "table", "Ventes", "column", "Note") })
	s.step("changement de type", func() { h.do("columns.set_type", "table", "Ventes", "column", "Qté", "type", "text") })
	s.step("colonne collée à droite", func() { h.set(1, 7, "Remise") })
	s.step("suppression du tableau", func() { h.do("tables.delete", "tables", []any{"Ventes"}) })
	h.undo()
	s.check("tableau rétabli")
	s.step("conversion en plage", func() {
		if err := h.d.ConvertToRange(ctx, "Ventes"); err != nil {
			t.Fatal(err)
		}
	})
	// Écritures sur un tableau non publié : rien dans la file.
	n := s.outboxLen()
	h.paste(20, 1, []any{"Ville", "Pays"}, []any{"Lyon", "France"}, []any{"Oslo", "Norvège"})
	h.set(21, 1, "Paris")
	h.undo()
	h.undo()
	if s.outboxLen() != n {
		t.Fatalf("un tableau non publié a écrit dans la file d'envoi")
	}
	for k, v := range s.m.Ops {
		t.Logf("%s : %d", k, v)
	}
	for _, k := range []string{command.OpSet, command.OpInsert, command.OpDelete, command.OpMove, command.OpMeta} {
		if s.m.Ops[k] == 0 {
			t.Errorf("aucune opération %s", k)
		}
	}
}

// TestOplogRandomEdits : les suites d'actions tirées au hasard de
// TestRandomEditsKeepInvariants, sur un tableau publié ; après chaque
// action, la file d'envoi rejouée donne le contenu local, y compris après
// réouverture et en annulant tout.
func TestOplogRandomEdits(t *testing.T) {
	seeds, ops := int64(6), 60
	if testing.Short() {
		seeds, ops = 2, 30
	}
	for seed := int64(1); seed <= seeds; seed++ {
		t.Run(fmt.Sprint("graine ", seed), func(t *testing.T) { randomSessionOn(t, seed, ops, true) })
	}
}

// TestOplogUndoIsConditioned : l'annulation d'une saisie est conditionnée à
// la valeur saisie ; si la valeur locale a changé entre-temps sans passer
// par le journal (modification reçue d'un collègue), la cellule n'est pas
// annulée et garde la valeur du collègue (§7), le reste de l'étape l'est.
func TestOplogUndoIsConditioned(t *testing.T) {
	h := newHost(t)
	ctx := context.Background()
	ventes(h)
	s := publish(h, "Ventes")
	h.set(2, 2, 10)
	s.check("saisie")
	// Un collègue écrit 11 dans la même cellule : appliqué hors journal.
	err := h.d.DB().Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE t_1 SET c_2 = 11 WHERE id = 1`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := oplog.Entries(ctx, h.d.DB())
	if err != nil {
		t.Fatal(err)
	}
	h.undo()
	if n := h.d.Oplog.TakeSkipped(); n != 1 {
		t.Fatalf("cellules gardées : %d", n)
	}
	var v float64
	if err := h.d.DB().Read().QueryRow(`SELECT c_2 FROM t_1 WHERE id = 1`).Scan(&v); err != nil || v != 11 {
		t.Fatalf("cellule du collègue : %v (%v)", v, err)
	}
	if es, _ := oplog.Entries(ctx, h.d.DB()); len(es) != len(before) {
		t.Fatalf("rien ne part pour une cellule gardée : %+v", es[len(before):])
	}
	// Sans écriture d'un collègue : condition « la cellule vaut toujours ce
	// que j'y ai mis ».
	h.set(2, 2, 12)
	h.undo()
	es, err := oplog.Entries(ctx, h.d.DB())
	if err != nil {
		t.Fatal(err)
	}
	last := es[len(es)-1]
	ops, err := command.DecodeOps(last.Ops)
	if err != nil || last.Kind != command.KindUndo || len(ops) != 1 {
		t.Fatalf("annulation : %+v %v", last, err)
	}
	op := ops[0]
	if op.Kind != command.OpSet || fmt.Sprint(op.Old) != "12" || fmt.Sprint(op.New) != "11" {
		t.Fatalf("annulation : %+v, attendu set 12 → 11", op)
	}
}
