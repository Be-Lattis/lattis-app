package script

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/command"
	"github.com/lalamefine/slitable/internal/meta"
	"github.com/lalamefine/slitable/internal/store"
)

type fixture struct {
	*canvas.Model
	db  *store.Store
	bus *command.Bus
}

func open(t *testing.T) *fixture {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "doc.sqlite"), canvas.Register)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := meta.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	bus := command.New(db)
	return &fixture{canvas.New(db, bus), db, bus}
}

// fill saisit des lignes à partir de (row, col), comme dans la grille.
func (f *fixture) fill(t *testing.T, row, col int, lines ...string) {
	t.Helper()
	for i, l := range lines {
		for j, v := range strings.Split(l, "|") {
			if _, err := f.SetCell(context.Background(), 1, row+i, col+j, v); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func (f *fixture) run(t *testing.T, src string) Result {
	t.Helper()
	return Run(context.Background(), Env{Read: f.db.Read(), Edit: f.Editor("Script")}, src, DefaultLimits)
}

func (f *fixture) mustRun(t *testing.T, src string) []string {
	t.Helper()
	res := f.run(t, src)
	if res.Err != nil {
		t.Fatalf("%v\n%s", res.Err, src)
	}
	return res.Output
}

func (f *fixture) check(t *testing.T) {
	t.Helper()
	problems, err := f.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

func (f *fixture) view(t *testing.T) string {
	t.Helper()
	v, err := f.View(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func commandes(t *testing.T) *fixture {
	f := open(t)
	f.fill(t, 0, 0,
		"Client|Montant|Payé",
		"Durand|120|vrai",
		"Martin|80,5|faux",
		"Durand|40|faux",
	)
	return f
}

func TestQueryByDisplayNames(t *testing.T) {
	f := commandes(t)
	out := f.mustRun(t, `
		local r = db.query([[SELECT Client, sum(Montant) AS total FROM "Feuille1"
		                     GROUP BY Client ORDER BY Client]])
		for _, l in ipairs(r) do print(l.Client, l.total) end
		print(db.value('SELECT count(*) FROM "Feuille1" WHERE Payé'))
		-- Les booléens arrivent en booléens Lua (0 serait vrai en Lua).
		for l in db.rows('SELECT Client, Payé FROM "Feuille1" WHERE Montant > ?', 50) do
			print(l.Client, l["Payé"], type(l["Payé"]))  -- accents : Lua 5.1 demande l["…"]
		end
		-- Une requête qui commence déjà par WITH garde ses propres expressions.
		print(db.value([[WITH gros AS (SELECT * FROM "Feuille1" WHERE Montant >= 100)
		                SELECT Client FROM gros]]))
	`)
	want := []string{"Durand\t160", "Martin\t80.5", "1", "Durand\ttrue\tboolean", "Martin\tfalse\tboolean", "Durand"}
	if strings.Join(out, "\n") != strings.Join(want, "\n") {
		t.Fatalf("sortie :\n%s\nattendu :\n%s", strings.Join(out, "\n"), strings.Join(want, "\n"))
	}
}

func TestTables(t *testing.T) {
	f := commandes(t)
	out := f.mustRun(t, `
		for _, t in ipairs(db.tables()) do
			local cols = {}
			for _, c in ipairs(t.columns) do cols[#cols+1] = c.name .. ":" .. c.type end
			print(t.sheet, t.name, table.concat(cols, ","))
		end`)
	if got := strings.Join(out, "\n"); got != "Feuille 1\tFeuille1\tClient:text,Montant:decimal,Payé:boolean" {
		t.Fatal(got)
	}
}

// Les écritures passent par le canevas : le tableau du dessous se décale,
// les types s'élargissent, et Ctrl+Z annule tout le script d'un coup.
func TestWritesAreOneUndoStep(t *testing.T) {
	f := commandes(t)
	f.fill(t, 5, 0, "Produit|Prix", "Vis|2")
	before := f.view(t)

	out := f.mustRun(t, `
		local id = db.insert("Feuille1", {Client = "Petit", Montant = 15, ["Payé"] = true})
		db.insert("Feuille1", {Client = "Grand", Montant = 1000.25})
		for l in db.rows('SELECT _id FROM "Feuille1" WHERE Client = ?', "Durand") do
			db.update("Feuille1", l._id, {Montant = "beaucoup"})   -- élargit au texte
		end
		db.delete("Feuille1", db.value('SELECT _id FROM "Feuille1" WHERE Client = ?', "Martin"))
		print(id, db.value('SELECT count(*) FROM "Feuille1"'))
	`)
	if out[0] != "4\t4" {
		t.Fatalf("sortie %q", out)
	}
	f.check(t)
	after := f.view(t)
	if !strings.Contains(after, "Petit") || !strings.Contains(after, "beaucoup") || strings.Contains(after, "Martin") {
		t.Fatalf("vue après le script :\n%s", after)
	}

	if _, err := f.bus.Undo(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.check(t)
	if got := f.view(t); got != before {
		t.Fatalf("après Ctrl+Z :\n%s\nattendu :\n%s", got, before)
	}
}

func TestWriteErrors(t *testing.T) {
	f := commandes(t)
	for src, want := range map[string]string{
		`db.insert("Inconnu", {a = 1})`:                           "Inconnu",
		`db.insert("Feuille1", {Prix = 1})`:                      `pas de colonne "Prix"`,
		`db.update("Feuille1", 99, {Client = "x"})`:              "pas de ligne 99",
		`db.update("Feuille1", 1, {Client = {}})`:                "non prise en charge",
		`db.query("SELECT nimporte FROM \"Feuille1\"")`:          "nimporte",
		`db.query("SELECT 1 FROM t WHERE x = ?", function() end)`: "paramètre SQL",
	} {
		res := f.run(t, src)
		if res.Err == nil || !strings.Contains(res.Err.Error(), want) {
			t.Errorf("%s : erreur %v, attendu %q", src, res.Err, want)
		}
	}
	res := Run(context.Background(), Env{Read: f.db.Read()}, `db.delete("Feuille1", 1)`, DefaultLimits)
	if !errors.Is(res.Err, ErrReadOnly) && (res.Err == nil || !strings.Contains(res.Err.Error(), ErrReadOnly.Error())) {
		t.Errorf("lecture seule : %v", res.Err)
	}
	f.check(t)
}

func TestSandbox(t *testing.T) {
	f := open(t)
	out := f.mustRun(t, `
		for _, name in ipairs({"io", "require", "load", "loadstring", "dofile", "debug", "_printregs", "collectgarbage"}) do
			assert(_G[name] == nil, name)
		end
		assert(os.execute == nil and os.remove == nil and os.getenv == nil and os.exit == nil)
		assert(string.dump == nil and ("x").dump == nil)
		assert(not pcall(string.rep, "x", 1e9))
		print(type(os.time()), os.date("%Y", 0) ~= nil, ("ab"):rep(2))
	`)
	if out[0] != "number\ttrue\tabab" {
		t.Fatal(out)
	}
}

func TestStop(t *testing.T) {
	f := open(t)
	lim := DefaultLimits
	lim.Timeout = 200 * time.Millisecond
	res := Run(context.Background(), Env{Read: f.db.Read()}, `while true do end`, lim)
	if !errors.Is(res.Err, ErrTimeout) {
		t.Fatalf("délai : %v", res.Err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	res = Run(ctx, Env{Read: f.db.Read()}, `local n = 0 while true do n = n + 1 end`, DefaultLimits)
	if !errors.Is(res.Err, ErrStopped) || res.Duration > time.Second {
		t.Fatalf("Arrêter : %v en %v", res.Err, res.Duration)
	}
}

func TestErrorLine(t *testing.T) {
	f := open(t)
	res := f.run(t, "local a = 1\nerror('raté')")
	if res.Err == nil || !strings.Contains(res.Err.Error(), "script:2:") {
		t.Fatalf("%v", res.Err)
	}
	if err := Check("if then"); err == nil {
		t.Fatal("erreur de syntaxe non détectée")
	}
}

func TestRewrite(t *testing.T) {
	c := &catalog{with: `"T"("_id") AS (SELECT id FROM t_1)`}
	for in, want := range map[string]string{
		"SELECT 1":                       `WITH "T"("_id") AS (SELECT id FROM t_1) SELECT 1`,
		"  -- note\n/* x */ select 1":    `WITH "T"("_id") AS (SELECT id FROM t_1) select 1`,
		"WITH a AS (SELECT 1) SELECT *":  `WITH "T"("_id") AS (SELECT id FROM t_1), a AS (SELECT 1) SELECT *`,
		"with recursive n(i) AS (...) x": `WITH RECURSIVE "T"("_id") AS (SELECT id FROM t_1), n(i) AS (...) x`,
		"PRAGMA table_info(t_1)":         "PRAGMA table_info(t_1)",
	} {
		if got := c.rewrite(in); got != want {
			t.Errorf("%q\n→ %q\nattendu %q", in, got, want)
		}
	}
}
