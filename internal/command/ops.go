package command

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"math"
)

// --- Journal logique ----------------------------------------------------------
//
// En plus des instructions SQL (Step), une commande peut porter une
// description logique de ce qu'elle change, indépendante de SQLite : c'est ce
// qu'un classeur partagé envoie au Serveur Lattis (voir
// docs/collaboration/journal-logique.md). Les opérations sur les données
// (set, insert, delete, move) sont relevées automatiquement par le
// Recorder (internal/remote/oplog) ; Tx.Record sert aux opérations que la
// base ne voit pas comme des lignes (mise en page, structure : « meta »).

// Genres d'opérations logiques.
const (
	OpSet    = "set"    // cellule (Table, Row, Col) : Old → New
	OpInsert = "insert" // ligne Row ajoutée en Pos, valeurs New (Values)
	OpDelete = "delete" // ligne Row supprimée, valeurs vues Old (Values)
	OpMove   = "move"   // ligne Row déplacée : clé d'ordre Old → Pos
	OpMeta   = "meta"   // structure ou mise en page (Meta)
)

// Op est une opération logique. Table et Col sont les identifiants locaux
// (le Serveur les connaît par _lattis_remote_tables), Row l'identifiant
// partagé de la ligne (lattis_id, UUIDv7). Old est la valeur vue avant
// l'opération (la condition de l'écriture sur le serveur), New la valeur
// écrite. Pour insert et delete, New et Old sont des Values (colonne →
// valeur, colonnes vides omises).
//
// Encodage JSON compact (une étape peut compter un million d'opérations) :
// {"k":"set","t":3,"r":"0192…","c":2,"o":1000,"n":1200}.
type Op struct {
	Kind  string      `json:"k"`
	Table int64       `json:"t,omitempty"`
	Row   string      `json:"r,omitempty"`
	Col   int64       `json:"c,omitempty"`
	Old   any         `json:"o,omitempty"`
	New   any         `json:"n,omitempty"`
	Pos   string      `json:"p,omitempty"`
	Meta  *MetaChange `json:"m,omitempty"`
}

// Values donne les valeurs d'une ligne, par identifiant de colonne.
type Values map[int64]any

// MetaChange décrit un changement de structure ou de mise en page :
// Object (« table », « column », « style »…), Action (« create », « drop »,
// « add », « retype »…), ID de l'objet (colonne…), états avant et après.
type MetaChange struct {
	Object string `json:"object"`
	Action string `json:"action"`
	ID     int64  `json:"id,omitempty"`
	Before any    `json:"before,omitempty"`
	After  any    `json:"after,omitempty"`
}

// Inverse rend le changement inverse (avant et après échangés).
func (m MetaChange) Inverse() MetaChange {
	inv := map[string]string{"create": "drop", "drop": "create", "add": "remove", "remove": "add"}
	a := m.Action
	if r, ok := inv[a]; ok {
		a = r
	}
	return MetaChange{Object: m.Object, Action: a, ID: m.ID, Before: m.After, After: m.Before}
}

// Inverse rend l'opération qui défait op, conditionnée à ce que op a écrit
// (annulation compensatoire : « la cellule vaut toujours ce que j'y ai
// mis »).
func (op Op) Inverse() Op {
	out := op
	switch op.Kind {
	case OpSet:
		out.Old, out.New = op.New, op.Old
	case OpInsert:
		out.Kind, out.Old, out.New = OpDelete, op.New, nil
	case OpDelete:
		out.Kind, out.New, out.Old = OpInsert, op.Old, nil
	case OpMove:
		old, _ := op.Old.(string)
		out.Old, out.Pos = op.Pos, old
	case OpMeta:
		if op.Meta != nil {
			m := op.Meta.Inverse()
			out.Meta = &m
		}
	}
	return out
}

// UnmarshalJSON garde les nombres exacts (json.Number) et rend Values pour
// les lignes insérées ou supprimées.
func (op *Op) UnmarshalJSON(b []byte) error {
	var aux struct {
		Kind  string          `json:"k"`
		Table int64           `json:"t"`
		Row   string          `json:"r"`
		Col   int64           `json:"c"`
		Old   json.RawMessage `json:"o"`
		New   json.RawMessage `json:"n"`
		Pos   string          `json:"p"`
		Meta  *MetaChange     `json:"m"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber() // y compris dans Meta
	if err := dec.Decode(&aux); err != nil {
		return err
	}
	*op = Op{Kind: aux.Kind, Table: aux.Table, Row: aux.Row, Col: aux.Col, Pos: aux.Pos, Meta: aux.Meta}
	rows := aux.Kind == OpInsert || aux.Kind == OpDelete
	var err error
	if op.Old, err = decodeValue(aux.Old, rows); err != nil {
		return err
	}
	op.New, err = decodeValue(aux.New, rows)
	return err
}

func decodeValue(raw json.RawMessage, row bool) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if row {
		var v Values
		err := dec.Decode(&v)
		return v, err
	}
	var v any
	err := dec.Decode(&v)
	return v, err
}

// EncodeOps encode des opérations (JSON). Les réels infinis, que JSON ne
// sait pas écrire, deviennent les textes « Infinity » et « -Infinity ».
func EncodeOps(ops []Op) (string, error) {
	for i := range ops {
		ops[i].Old, ops[i].New = jsonSafe(ops[i].Old), jsonSafe(ops[i].New)
	}
	b, err := json.Marshal(ops)
	return string(b), err
}

// DecodeOps relit des opérations encodées par EncodeOps.
func DecodeOps(data string) ([]Op, error) {
	var ops []Op
	err := json.Unmarshal([]byte(data), &ops)
	return ops, err
}

func jsonSafe(v any) any {
	switch x := v.(type) {
	case float64:
		if math.IsInf(x, 1) {
			return "Infinity"
		}
		if math.IsInf(x, -1) {
			return "-Infinity"
		}
	case Values:
		for k, e := range x {
			x[k] = jsonSafe(e)
		}
	}
	return v
}

// Record attache des opérations logiques à la commande en cours, à côté
// des instructions SQL de Apply. Elles partent dans la file d'envoi avec
// les opérations relevées sur les données (voir Recorder).
func (t *Tx) Record(ops ...Op) { t.ops = append(t.ops, ops...) }

// Genres de transactions vues par un Recorder.
const (
	KindDo   = "do"   // nouvelle commande ou suite d'une commande (Append)
	KindUndo = "undo" // annulation
	KindRedo = "redo" // rétablissement
)

// Commit décrit, pour Capture.Finish, la transaction qui se termine.
type Commit struct {
	Seq  int64  // commande du journal (0 : hors journal)
	Kind string // KindDo, KindUndo, KindRedo
	New  bool   // Seq vient d'être créée (un numéro peut resservir après des annulations)
	Ops  []Op   // opérations enregistrées par Tx.Record
}

// Recorder relève les opérations logiques des écritures du bus (voir
// internal/remote/oplog). Begin est appelé au début de chaque transaction
// du bus (Do, Append, Undo, Redo), dans la transaction.
type Recorder interface {
	Begin(ctx context.Context, tx *sql.Tx) (Capture, error)
}

// Capture suit une transaction du bus. Statement est appelé après chaque
// instruction exécutée par une étape (pour suivre la structure : tables
// reconstruites ou renommées) ; Finish à la fin, avant la validation, dans
// la même transaction.
type Capture interface {
	Statement(q string) error
	Finish(c Commit) error
}
