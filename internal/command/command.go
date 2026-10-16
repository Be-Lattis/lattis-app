// Package command est le bus de commandes réversibles d'un document.
//
// Toute modification (données, schéma, mise en forme, script) passe par
// Bus.Do : chaque opération élémentaire exécute ses instructions SQL et
// enregistre en même temps les instructions inverses (Tx.Apply). Une action
// de l'utilisateur forme une commande, c'est-à-dire une étape d'annulation.
//
// Les commandes sont persistées dans _slitable_journal, dans la même
// transaction que les données : l'historique d'annulation survit à un
// enregistrement, à une réouverture et à un crash, et il est toujours
// cohérent avec la base. Le rétablissement rejoue les instructions
// d'origine ; les identifiants qu'elles utilisent sont fixés à la
// construction, ce qui rend le rejeu déterministe.
package command

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/lalamefine/slitable/internal/store"
)

// Schema crée la table du journal.
const Schema = `
CREATE TABLE IF NOT EXISTS _slitable_journal (
	seq    INTEGER PRIMARY KEY,
	name   TEXT NOT NULL,
	steps  TEXT NOT NULL,           -- JSON : opérations et inverses
	undone INTEGER NOT NULL DEFAULT 0
) STRICT;
`

// DefaultDepth est le nombre de commandes gardées dans l'historique.
const DefaultDepth = 200

// ErrNothing signale qu'il n'y a rien à annuler ou à rétablir.
var ErrNothing = errors.New("rien à annuler ou à rétablir")

// Stmt est une instruction SQL avec ses paramètres.
type Stmt struct {
	Q    string
	Args []any
}

// S construit une instruction.
func S(q string, args ...any) Stmt { return Stmt{q, args} }

// Step est une opération élémentaire. Purge s'exécute quand la commande sort
// de l'historique sans avoir été annulée (par exemple pour supprimer la
// corbeille d'une colonne supprimée).
type Step struct {
	Do, Undo, Purge []Stmt
}

// Tx est la transaction d'une commande en cours de construction.
type Tx struct {
	*sql.Tx
	Ctx   context.Context
	steps []Step
}

// Apply exécute do et enregistre l'opération avec son inverse.
func (t *Tx) Apply(do, undo []Stmt) error { return t.ApplyStep(Step{Do: do, Undo: undo}) }

// ApplyStep exécute s.Do et enregistre l'opération.
func (t *Tx) ApplyStep(s Step) error {
	if err := run(t.Ctx, t.Tx, s.Do); err != nil {
		return err
	}
	t.steps = append(t.steps, s)
	return nil
}

// Query et Row lisent dans la transaction (elles voient ses écritures).
func (t *Tx) Query(q string, args ...any) (*sql.Rows, error) {
	return t.QueryContext(t.Ctx, q, args...)
}
func (t *Tx) Row(q string, args ...any) *sql.Row { return t.QueryRowContext(t.Ctx, q, args...) }

func run(ctx context.Context, tx *sql.Tx, stmts []Stmt) error {
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s.Q, s.Args...); err != nil {
			return fmt.Errorf("%s : %w", s.Q, err)
		}
	}
	return nil
}

// Bus applique, annule et rétablit les commandes d'un document.
type Bus struct {
	db    *store.Store
	Depth int
	// Guard, s'il est fixé, encadre chaque annulation et rétablissement :
	// appelé dans la transaction avant de rejouer les instructions, il rend
	// une vérification appelée après ; une erreur de l'une ou l'autre défait
	// tout (protection des feuilles : voir canvas.Model).
	Guard func(ctx context.Context, tx *sql.Tx) (check func() error, err error)
}

// guarded rejoue fn entre les deux temps de Guard.
func (b *Bus) guarded(ctx context.Context, tx *sql.Tx, verb, name string, fn func() error) error {
	if b.Guard == nil {
		return fn()
	}
	check, err := b.Guard(ctx, tx)
	if err != nil {
		return err
	}
	if err := fn(); err != nil {
		return err
	}
	if err := check(); err != nil {
		return fmt.Errorf("impossible %s « %s » : %w", verb, name, err)
	}
	return nil
}

// New crée le bus. Le schéma du journal doit exister (voir Schema).
func New(db *store.Store) *Bus { return &Bus{db: db, Depth: DefaultDepth} }

// Do exécute une action. Si fn échoue, rien n'est appliqué. Une action qui
// ne change rien n'entre pas dans l'historique. Renvoie le numéro de la
// commande (0 si rien n'a changé).
func (b *Bus) Do(ctx context.Context, name string, fn func(*Tx) error) (int64, error) {
	if g := groupOf(ctx); g != nil {
		return g.append(ctx, b, fn)
	}
	var seq int64
	err := b.db.Write(ctx, func(sqlTx *sql.Tx) error {
		t := &Tx{Tx: sqlTx, Ctx: ctx}
		if err := fn(t); err != nil {
			return err
		}
		if len(t.steps) == 0 {
			return nil
		}
		// Une nouvelle action efface ce qui pouvait être rétabli.
		if _, err := sqlTx.ExecContext(ctx, `DELETE FROM _slitable_journal WHERE undone = 1`); err != nil {
			return err
		}
		data, err := encodeSteps(t.steps)
		if err != nil {
			return err
		}
		r, err := sqlTx.ExecContext(ctx, `INSERT INTO _slitable_journal (name, steps) VALUES (?, ?)`, name, data)
		if err != nil {
			return err
		}
		seq, _ = r.LastInsertId()
		return b.trim(ctx, sqlTx)
	})
	return seq, err
}

// Append ajoute des opérations à la dernière commande seq, qui doit être en
// tête de l'historique : un script qui écrit par petites transactions reste
// une seule étape d'annulation. Si une autre action s'est intercalée, une
// nouvelle commande est créée sous le même nom.
func (b *Bus) Append(ctx context.Context, seq int64, name string, fn func(*Tx) error) (int64, error) {
	if g := groupOf(ctx); g != nil {
		return g.append(ctx, b, fn)
	}
	return b.appendTo(ctx, seq, name, fn)
}

// Group réunit en une seule étape d'annulation, nommée Name, toutes les
// actions faites avec le contexte qui le porte (WithGroup) : un script ou un
// bouton qui crée une feuille, renomme un tableau et écrit des lignes
// s'annule d'un coup. Chaque action reste une courte transaction ; si une
// action de l'utilisateur s'intercale, la suite forme une nouvelle étape du
// même nom (voir Append).
type Group struct {
	Name string
	mu   sync.Mutex
	seq  int64
}

type groupKey struct{}

// WithGroup ouvre un groupe d'actions nommé name.
func WithGroup(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, groupKey{}, &Group{Name: name})
}

func groupOf(ctx context.Context) *Group {
	g, _ := ctx.Value(groupKey{}).(*Group)
	return g
}

func (g *Group) append(ctx context.Context, b *Bus, fn func(*Tx) error) (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	seq, err := b.appendTo(ctx, g.seq, g.Name, fn)
	if err == nil && seq != 0 {
		g.seq = seq
	}
	return seq, err
}

func (b *Bus) appendTo(ctx context.Context, seq int64, name string, fn func(*Tx) error) (int64, error) {
	out := seq
	err := b.db.Write(ctx, func(sqlTx *sql.Tx) error {
		t := &Tx{Tx: sqlTx, Ctx: ctx}
		if err := fn(t); err != nil {
			return err
		}
		if len(t.steps) == 0 {
			return nil
		}
		var last int64
		var data string
		err := sqlTx.QueryRowContext(ctx, `SELECT seq, steps FROM _slitable_journal WHERE undone = 0 ORDER BY seq DESC LIMIT 1`).Scan(&last, &data)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if seq == 0 || last != seq {
			if _, err := sqlTx.ExecContext(ctx, `DELETE FROM _slitable_journal WHERE undone = 1`); err != nil {
				return err
			}
			enc, err := encodeSteps(t.steps)
			if err != nil {
				return err
			}
			r, err := sqlTx.ExecContext(ctx, `INSERT INTO _slitable_journal (name, steps) VALUES (?, ?)`, name, enc)
			if err != nil {
				return err
			}
			out, _ = r.LastInsertId()
			return b.trim(ctx, sqlTx)
		}
		steps, err := decodeSteps(data)
		if err != nil {
			return err
		}
		enc, err := encodeSteps(append(steps, t.steps...))
		if err != nil {
			return err
		}
		_, err = sqlTx.ExecContext(ctx, `UPDATE _slitable_journal SET steps = ? WHERE seq = ?`, enc, seq)
		return err
	})
	return out, err
}

// trim fait sortir de l'historique les commandes au-delà de Depth.
func (b *Bus) trim(ctx context.Context, tx *sql.Tx) error {
	rs, err := tx.QueryContext(ctx, `SELECT seq, steps FROM _slitable_journal WHERE undone = 0 ORDER BY seq DESC LIMIT -1 OFFSET ?`, b.Depth)
	if err != nil {
		return err
	}
	type old struct {
		seq   int64
		steps string
	}
	var olds []old
	for rs.Next() {
		var o old
		if err := rs.Scan(&o.seq, &o.steps); err != nil {
			rs.Close()
			return err
		}
		olds = append(olds, o)
	}
	rs.Close()
	for _, o := range olds {
		steps, err := decodeSteps(o.steps)
		if err != nil {
			return err
		}
		for _, s := range steps {
			if err := run(ctx, tx, s.Purge); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM _slitable_journal WHERE seq = ?`, o.seq); err != nil {
			return err
		}
	}
	return nil
}

// Undo annule la dernière action et renvoie son nom.
func (b *Bus) Undo(ctx context.Context) (string, error) {
	var name string
	err := b.db.Write(ctx, func(tx *sql.Tx) error {
		var seq int64
		var data string
		err := tx.QueryRowContext(ctx, `SELECT seq, name, steps FROM _slitable_journal WHERE undone = 0 ORDER BY seq DESC LIMIT 1`).Scan(&seq, &name, &data)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNothing
		}
		if err != nil {
			return err
		}
		steps, err := decodeSteps(data)
		if err != nil {
			return err
		}
		err = b.guarded(ctx, tx, "d'annuler", name, func() error {
			for i := len(steps) - 1; i >= 0; i-- {
				if err := run(ctx, tx, steps[i].Undo); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE _slitable_journal SET undone = 1 WHERE seq = ?`, seq)
		return err
	})
	return name, err
}

// Redo rétablit la dernière action annulée et renvoie son nom.
func (b *Bus) Redo(ctx context.Context) (string, error) {
	var name string
	err := b.db.Write(ctx, func(tx *sql.Tx) error {
		var seq int64
		var data string
		err := tx.QueryRowContext(ctx, `SELECT seq, name, steps FROM _slitable_journal WHERE undone = 1 ORDER BY seq LIMIT 1`).Scan(&seq, &name, &data)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNothing
		}
		if err != nil {
			return err
		}
		steps, err := decodeSteps(data)
		if err != nil {
			return err
		}
		err = b.guarded(ctx, tx, "de rétablir", name, func() error {
			for _, s := range steps {
				if err := run(ctx, tx, s.Do); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE _slitable_journal SET undone = 0 WHERE seq = ?`, seq)
		return err
	})
	return name, err
}

// State décrit l'historique pour l'interface (libellés des menus Annuler et
// Rétablir).
type State struct {
	Undo string `json:"undo"` // nom de l'action à annuler, vide s'il n'y en a pas
	Redo string `json:"redo"`
}

// State lit l'état de l'historique.
func (b *Bus) State(ctx context.Context) (State, error) {
	var s State
	err := b.db.Read().QueryRowContext(ctx, `SELECT
		coalesce((SELECT name FROM _slitable_journal WHERE undone = 0 ORDER BY seq DESC LIMIT 1), ''),
		coalesce((SELECT name FROM _slitable_journal WHERE undone = 1 ORDER BY seq LIMIT 1), '')`).Scan(&s.Undo, &s.Redo)
	return s, err
}

// --- Encodage du journal ------------------------------------------------------

// Les paramètres gardent leur type exact (entier, réel, texte, BLOB, NULL),
// ce que JSON seul ne permet pas.
type jsonArg struct {
	T string `json:"t"`
	V any    `json:"v,omitempty"`
}

type jsonStmt struct {
	Q string    `json:"q"`
	A []jsonArg `json:"a,omitempty"`
}

type jsonStep struct {
	D []jsonStmt `json:"d"`
	U []jsonStmt `json:"u"`
	P []jsonStmt `json:"p,omitempty"`
}

func encodeStmts(ss []Stmt) ([]jsonStmt, error) {
	out := make([]jsonStmt, len(ss))
	for i, s := range ss {
		out[i].Q = s.Q
		for _, a := range s.Args {
			var ja jsonArg
			switch v := a.(type) {
			case nil:
				ja = jsonArg{T: "n"}
			case int:
				ja = jsonArg{"i", int64(v)}
			case int64:
				ja = jsonArg{"i", v}
			case bool:
				ja = jsonArg{"i", map[bool]int64{true: 1, false: 0}[v]}
			case float64:
				ja = jsonArg{"f", v}
			case string:
				ja = jsonArg{"s", v}
			case []byte:
				ja = jsonArg{"b", base64.StdEncoding.EncodeToString(v)}
			default:
				return nil, fmt.Errorf("journal : paramètre de type %T non pris en charge", a)
			}
			out[i].A = append(out[i].A, ja)
		}
	}
	return out, nil
}

func decodeStmts(js []jsonStmt) ([]Stmt, error) {
	out := make([]Stmt, len(js))
	for i, s := range js {
		out[i].Q = s.Q
		for _, a := range s.A {
			switch a.T {
			case "n":
				out[i].Args = append(out[i].Args, nil)
			case "i":
				n, _ := a.V.(json.Number)
				v, err := n.Int64()
				if err != nil {
					return nil, err
				}
				out[i].Args = append(out[i].Args, v)
			case "f":
				n, _ := a.V.(json.Number)
				v, err := n.Float64()
				if err != nil {
					return nil, err
				}
				out[i].Args = append(out[i].Args, v)
			case "s":
				s, _ := a.V.(string)
				out[i].Args = append(out[i].Args, s)
			case "b":
				s, _ := a.V.(string)
				b, err := base64.StdEncoding.DecodeString(s)
				if err != nil {
					return nil, err
				}
				out[i].Args = append(out[i].Args, b)
			default:
				return nil, fmt.Errorf("journal : type de paramètre %q inconnu", a.T)
			}
		}
	}
	return out, nil
}

func encodeSteps(steps []Step) (string, error) {
	js := make([]jsonStep, len(steps))
	for i, s := range steps {
		var err error
		if js[i].D, err = encodeStmts(s.Do); err != nil {
			return "", err
		}
		if js[i].U, err = encodeStmts(s.Undo); err != nil {
			return "", err
		}
		if js[i].P, err = encodeStmts(s.Purge); err != nil {
			return "", err
		}
	}
	b, err := json.Marshal(js)
	return string(b), err
}

func decodeSteps(data string) ([]Step, error) {
	var js []jsonStep
	dec := json.NewDecoder(strings.NewReader(data))
	dec.UseNumber() // entiers exacts au-delà de 2^53
	if err := dec.Decode(&js); err != nil {
		return nil, err
	}
	out := make([]Step, len(js))
	for i, s := range js {
		var err error
		if out[i].Do, err = decodeStmts(s.D); err != nil {
			return nil, err
		}
		if out[i].Undo, err = decodeStmts(s.U); err != nil {
			return nil, err
		}
		if out[i].Purge, err = decodeStmts(s.P); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// InGroup dit si ctx porte un groupe d'actions.
func InGroup(ctx context.Context) bool { return groupOf(ctx) != nil }
