// Package script exécute les scripts Lua d'un classeur (onglet SCRIPT).
//
// Un script lit le classeur en SQL avec les noms affichés des tableaux et
// des colonnes (db.query, db.rows, db.value), et l'écrit ligne par ligne
// (db.insert, db.update, db.delete). Les écritures passent par le modèle du
// canevas, comme une saisie dans la grille : typage, mise en page des
// tableaux voisins et annulation restent cohérents, et tout ce qu'un script
// a écrit s'annule en une seule étape. La référence de l'API est dans
// docs/scripting.md.
//
// Chaque écriture est une courte transaction de la file d'écriture : la
// grille reste utilisable pendant qu'un script tourne. Un script arrêté ou en
// erreur garde ce qu'il a déjà écrit ; Ctrl+Z l'efface d'un coup.
package script

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// Editor écrit dans les tableaux ; *canvas.Editor le réalise. vals associe
// un nom de colonne à une saisie brute (vide : effacer la cellule).
type Editor interface {
	Insert(ctx context.Context, table string, vals map[string]string) (int64, error)
	Update(ctx context.Context, table string, id int64, vals map[string]string) error
	Delete(ctx context.Context, table string, id int64) error
	// Exec exécute une requête d'écriture (INSERT, UPDATE, DELETE) avec les
	// noms affichés, annulable, et renvoie le nombre de lignes touchées.
	Exec(ctx context.Context, q string, args ...any) (int64, error)
}

// Env relie un script à son classeur.
type Env struct {
	Read *sql.DB // pool de lecture (store.Store.Read)
	Edit Editor  // nil : script en lecture seule
	// Print reçoit chaque ligne de print au fil de l'exécution (facultatif).
	Print func(string)
	// API : commandes de l'application (feuilles, sélection, boutons…) ;
	// nil : seulement db et print.
	API API
	// Entry : fonction globale appelée après le chargement du script (bouton
	// d'action lié à une fonction) ; vide : le script seul.
	Entry string
	// Event : argument de l'appel de Entry (évènement d'un contrôle :
	// {control, event, value, old}), converti comme du JSON ; il est aussi
	// la variable globale event du script. nil : appel sans argument.
	Event any
}

// Limits borne un script.
type Limits struct {
	Timeout  time.Duration // durée maximale (0 : aucune)
	MaxRows  int           // lignes au plus par db.query
	MaxHeap  uint64        // croissance du tas tolérée, en octets
	MaxPrint int           // lignes de sortie conservées dans Result
}

// DefaultLimits sont les bornes par défaut.
var DefaultLimits = Limits{Timeout: 30 * time.Second, MaxRows: 100_000, MaxHeap: 512 << 20, MaxPrint: 1000}

// Result est l'issue d'un script.
type Result struct {
	Output   []string      `json:"output"`
	Writes   int           `json:"writes"` // lignes ajoutées, modifiées ou supprimées
	Duration time.Duration `json:"duration"`
	Err      error         `json:"-"`
}

// Raisons d'arrêt.
var (
	ErrTimeout  = errors.New("script arrêté : durée maximale dépassée")
	ErrStopped  = errors.New("script arrêté par l'utilisateur")
	ErrMemory   = errors.New("script arrêté : trop de mémoire utilisée")
	ErrReadOnly = errors.New("ce script ne peut pas modifier le classeur")
)

// Run exécute un script. Annuler ctx joue le rôle du bouton Arrêter.
func Run(ctx context.Context, env Env, src string, lim Limits) Result {
	start := time.Now()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	if lim.Timeout > 0 {
		c := &clock{since: start}
		ctx = context.WithValue(ctx, clockKey{}, c)
		go watchClock(ctx, cancel, c, lim.Timeout)
	}
	if lim.MaxHeap > 0 {
		go watchHeap(ctx, cancel, lim.MaxHeap)
	}

	L := newState(ctx)
	defer L.Close()
	r := &runner{ctx: ctx, env: env, lim: lim}
	defer r.closeRows()
	r.install(L)

	var arg lua.LValue = lua.LNil
	if env.Event != nil {
		arg = jsonToLua(L, env.Event)
		L.SetGlobal("event", arg)
	}
	err := r.load(L, src)
	if err == nil {
		err = L.PCall(0, 0, nil)
	}
	if err == nil && env.Entry != "" {
		fn, ok := L.GetGlobal(env.Entry).(*lua.LFunction)
		if !ok {
			err = fmt.Errorf("le script ne définit pas de fonction %s", env.Entry)
		} else {
			L.Push(fn)
			n := 0
			if env.Event != nil {
				L.Push(arg)
				n = 1
			}
			err = L.PCall(n, 0, nil)
		}
	}
	res := Result{Output: r.output, Writes: r.writes, Duration: time.Since(start)}
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			if errors.Is(cause, context.Canceled) {
				cause = ErrStopped
			}
			err = cause
		}
		res.Err = err
	}
	return res
}

// Check compile un script sans l'exécuter (erreurs de syntaxe).
func Check(src string) error {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	_, err := L.Load(strings.NewReader(src), "script")
	return err
}

type runner struct {
	ctx    context.Context
	env    Env
	lim    Limits
	cat    *catalog // nil : à recharger (après une écriture)
	output []string
	writes int
	open   []*sql.Rows
}

func (r *runner) load(L *lua.LState, src string) error {
	fn, err := L.Load(strings.NewReader(src), "script")
	if err != nil {
		return err
	}
	L.Push(fn)
	return nil
}

func (r *runner) install(L *lua.LState) {
	L.SetGlobal("print", L.NewFunction(r.print))
	db := L.NewTable()
	for name, fn := range map[string]lua.LGFunction{
		"query":   r.query,
		"rows":    r.rows,
		"value":   r.value,
		"tables":  r.tables,
		"insert":  r.insert,
		"update":  r.update,
		"delete":  r.delete,
		"execute": r.execute,
	} {
		db.RawSetString(name, L.NewFunction(fn))
	}
	L.SetGlobal("db", db)
	r.installAPI(L)
}

func (r *runner) print(L *lua.LState) int {
	parts := make([]string, L.GetTop())
	for i := range parts {
		parts[i] = L.ToStringMeta(L.Get(i + 1)).String()
	}
	line := strings.Join(parts, "\t")
	if r.env.Print != nil {
		r.env.Print(line)
	}
	if len(r.output) < r.lim.MaxPrint {
		r.output = append(r.output, line)
	}
	return 0
}

func (r *runner) catalog(L *lua.LState) *catalog {
	if r.cat == nil {
		cat, err := loadCatalog(r.ctx, r.env.Read)
		if err != nil {
			L.RaiseError("lecture des tableaux : %v", err)
		}
		r.cat = cat
	}
	return r.cat
}

func (r *runner) closeRows() {
	for _, rs := range r.open {
		rs.Close()
	}
	r.open = nil
}

// --- Lecture -----------------------------------------------------------------

// exec lance une requête de lecture, avec les noms affichés.
func (r *runner) exec(L *lua.LState, fn string) (*sql.Rows, []string, map[string]bool) {
	q := L.CheckString(1)
	cat := r.catalog(L)
	rs, err := r.env.Read.QueryContext(r.ctx, cat.rewrite(q), args(L, 2)...)
	if err != nil {
		L.RaiseError("db.%s : %s", fn, sqlError(err))
	}
	cols, err := rs.Columns()
	if err != nil {
		rs.Close()
		L.RaiseError("db.%s : %v", fn, err)
	}
	return rs, cols, cat.bools
}

// scanRow lit la ligne courante en table Lua { colonne = valeur }.
func scanRow(L *lua.LState, rs *sql.Rows, cols []string, bools map[string]bool) (*lua.LTable, error) {
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rs.Scan(ptrs...); err != nil {
		return nil, err
	}
	row := L.CreateTable(0, len(cols))
	for i, c := range cols {
		row.RawSetString(c, toLua(vals[i], bools[c]))
	}
	return row, nil
}

// db.query(sql, ...) : tableau de lignes { colonne = valeur }, borné à MaxRows.
func (r *runner) query(L *lua.LState) int {
	rs, cols, bools := r.exec(L, "query")
	defer rs.Close()
	out := L.NewTable()
	n := 0
	for rs.Next() {
		if r.lim.MaxRows > 0 && n == r.lim.MaxRows {
			L.RaiseError("db.query : plus de %d lignes ; ajoutez un LIMIT ou parcourez-les avec db.rows", r.lim.MaxRows)
		}
		row, err := scanRow(L, rs, cols, bools)
		if err != nil {
			L.RaiseError("db.query : %v", err)
		}
		out.Append(row)
		n++
	}
	if err := rs.Err(); err != nil {
		L.RaiseError("db.query : %s", sqlError(err))
	}
	L.Push(out)
	return 1
}

// db.rows(sql, ...) : itérateur pour « for ligne in db.rows(...) do », sans
// limite de lignes (elles ne sont pas toutes gardées en mémoire).
func (r *runner) rows(L *lua.LState) int {
	rs, cols, bools := r.exec(L, "rows")
	r.open = append(r.open, rs)
	done := false
	L.Push(L.NewFunction(func(L *lua.LState) int {
		if done || !rs.Next() {
			if !done {
				done = true
				err := rs.Err()
				rs.Close()
				if err != nil {
					L.RaiseError("db.rows : %s", sqlError(err))
				}
			}
			L.Push(lua.LNil)
			return 1
		}
		row, err := scanRow(L, rs, cols, bools)
		if err != nil {
			L.RaiseError("db.rows : %v", err)
		}
		L.Push(row)
		return 1
	}))
	return 1
}

// db.value(sql, ...) : première colonne de la première ligne, ou nil.
func (r *runner) value(L *lua.LState) int {
	rs, cols, bools := r.exec(L, "value")
	defer rs.Close()
	if !rs.Next() {
		if err := rs.Err(); err != nil {
			L.RaiseError("db.value : %s", sqlError(err))
		}
		L.Push(lua.LNil)
		return 1
	}
	row, err := scanRow(L, rs, cols, bools)
	if err != nil {
		L.RaiseError("db.value : %v", err)
	}
	L.Push(row.RawGetString(cols[0]))
	return 1
}

// db.tables() : { {name, sheet, columns = { {name, type, fixed} } } }.
func (r *runner) tables(L *lua.LState) int {
	out := L.NewTable()
	for _, t := range r.catalog(L).tables {
		tb := L.NewTable()
		tb.RawSetString("name", lua.LString(t.Name))
		tb.RawSetString("sheet", lua.LString(t.Sheet))
		cols := L.NewTable()
		for _, c := range t.Columns {
			ct := L.NewTable()
			ct.RawSetString("name", lua.LString(c.Name))
			ct.RawSetString("type", lua.LString(c.Type))
			ct.RawSetString("fixed", lua.LBool(c.Fixed))
			cols.Append(ct)
		}
		tb.RawSetString("columns", cols)
		out.Append(tb)
	}
	L.Push(out)
	return 1
}

// --- Écriture ----------------------------------------------------------------

func (r *runner) editor(L *lua.LState, fn string) Editor {
	if r.env.Edit == nil {
		L.RaiseError("db.%s : %v", fn, ErrReadOnly)
	}
	return r.env.Edit
}

// written invalide le catalogue : une écriture peut changer le type d'une
// colonne.
func (r *runner) written() {
	r.writes++
	r.cat = nil
}

// db.insert(tableau, { colonne = valeur }) : ajoute une ligne, renvoie sa clé.
func (r *runner) insert(L *lua.LState) int {
	ed := r.editor(L, "insert")
	table, vals := L.CheckString(1), raws(L, 2, "insert")
	id, err := ed.Insert(r.ctx, table, vals)
	if err != nil {
		L.RaiseError("db.insert : %v", err)
	}
	r.written()
	L.Push(lua.LNumber(id))
	return 1
}

// db.update(tableau, clé, { colonne = valeur }) : modifie une ligne.
func (r *runner) update(L *lua.LState) int {
	ed := r.editor(L, "update")
	table, id, vals := L.CheckString(1), key(L, 2, "update"), raws(L, 3, "update")
	if err := ed.Update(r.ctx, table, id, vals); err != nil {
		L.RaiseError("db.update : %v", err)
	}
	r.written()
	return 0
}

// db.delete(tableau, clé) : supprime une ligne.
func (r *runner) delete(L *lua.LState) int {
	ed := r.editor(L, "delete")
	table, id := L.CheckString(1), key(L, 2, "delete")
	if err := ed.Delete(r.ctx, table, id); err != nil {
		L.RaiseError("db.delete : %v", err)
	}
	r.written()
	return 0
}

// db.execute(sql, ...) : requête d'écriture, renvoie le nombre de lignes touchées.
func (r *runner) execute(L *lua.LState) int {
	ed := r.editor(L, "execute")
	n, err := ed.Exec(r.ctx, L.CheckString(1), args(L, 2)...)
	if err != nil {
		L.RaiseError("db.execute : %v", err)
	}
	r.writes += int(n)
	r.cat = nil
	L.Push(lua.LNumber(n))
	return 1
}

// --- Conversions -------------------------------------------------------------

// args convertit les paramètres Lua d'une requête, à partir de l'indice from.
func args(L *lua.LState, from int) []any {
	var out []any
	for i := from; i <= L.GetTop(); i++ {
		switch v := L.Get(i).(type) {
		case lua.LNumber:
			f := float64(v)
			if f == math.Trunc(f) && math.Abs(f) < 1<<53 {
				out = append(out, int64(f))
			} else {
				out = append(out, f)
			}
		case lua.LString:
			out = append(out, string(v))
		case lua.LBool:
			if v {
				out = append(out, int64(1))
			} else {
				out = append(out, int64(0))
			}
		case *lua.LNilType:
			out = append(out, nil)
		default:
			L.ArgError(i, "paramètre SQL : nombre, texte, booléen ou nil attendu, "+v.Type().String()+" reçu")
		}
	}
	return out
}

// raws convertit une table { colonne = valeur } en saisies brutes, écrites
// comme l'utilisateur les taperait.
func raws(L *lua.LState, i int, fn string) map[string]string {
	t := L.CheckTable(i)
	out := map[string]string{}
	t.ForEach(func(k, v lua.LValue) {
		name, ok := k.(lua.LString)
		if !ok {
			L.RaiseError("db.%s : les clés doivent être des noms de colonnes", fn)
		}
		switch v := v.(type) {
		case lua.LNumber:
			out[string(name)] = number(float64(v))
		case lua.LString:
			out[string(name)] = string(v)
		case lua.LBool:
			if v {
				out[string(name)] = "vrai"
			} else {
				out[string(name)] = "faux"
			}
		default:
			L.RaiseError("db.%s : colonne %q, valeur %s non prise en charge", fn, string(name), v.Type().String())
		}
	})
	if len(out) == 0 && fn != "insert" {
		L.RaiseError("db.%s : aucune colonne à écrire", fn)
	}
	return out
}

func number(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1<<53 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func key(L *lua.LState, i int, fn string) int64 {
	f := float64(L.CheckNumber(i))
	if f != math.Trunc(f) {
		L.RaiseError("db.%s : clé de ligne entière attendue", fn)
	}
	return int64(f)
}

// toLua convertit une valeur SQL ; asBool : la colonne porte le nom d'une
// colonne booléenne, 0 et 1 deviennent false et true (en Lua, 0 est vrai).
func toLua(v any, asBool bool) lua.LValue {
	switch v := v.(type) {
	case int64:
		if asBool && (v == 0 || v == 1) {
			return lua.LBool(v == 1)
		}
		return lua.LNumber(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []byte:
		return lua.LString(v)
	case bool:
		return lua.LBool(v)
	case time.Time:
		return lua.LString(v.Format("2006-01-02"))
	}
	return lua.LNil
}

// sqlError retire de l'erreur la clause WITH ajoutée par rewrite, que
// l'utilisateur n'a pas écrite.
func sqlError(err error) string {
	s := err.Error()
	if i := strings.Index(s, "WITH \""); i >= 0 {
		s = s[:i]
	}
	return strings.TrimRight(fmt.Sprint(s), " :")
}
