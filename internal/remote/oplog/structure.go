package oplog

import (
	"context"
	"database/sql"
	"slices"
	"sync"

	"github.com/lalamefine/lattis/internal/command"
)

// Structure des tableaux publiés d'un classeur collaboratif (spécification
// de l'application §6.4, fonctions du serveur §9.3) : à la fin de chaque
// transaction du bus, la forme des tableaux publiés (nom, colonnes saisies
// et leurs noms et types, clé) est comparée à celle d'avant, quel que soit
// le chemin de la modification (grille, API, scripts, MCP, assistant,
// annulation). Les changements sont confiés à la synchronisation
// (SetStructure), qui les fait sur le serveur avant que la transaction
// locale ne soit validée : un refus du serveur défait toute la transaction
// locale. L'annulation d'un changement de structure est une transaction
// comme une autre : elle envoie l'opération inverse.

// Changements de structure (Change.Action).
const (
	DropTable    = "drop_table"
	RenameTable  = "rename_table"
	DropColumn   = "drop_column"
	RenameColumn = "rename_column"
	RetypeColumn = "retype_column"
	AddColumn    = "add_column"
	SetKey       = "set_key"
)

// Change est un changement de structure d'un tableau publié, en
// identifiants locaux.
type Change struct {
	Table  int64
	Action string
	// Column : colonne visée (colonnes) ; Name : nouveau nom (tableau,
	// colonne ajoutée ou renommée), ancien nom pour une suppression ; Type :
	// nouveau type local (grid.Type) d'une colonne ajoutée ou convertie.
	Column int64
	Name   string
	Type   string
	// Prev : colonne ajoutée, colonnes saisies qui la précèdent, de la plus
	// proche à la plus lointaine (la première publiée donne sa place sur le
	// serveur).
	Prev []int64
	// Key : nouvelle colonne clé (0 : plus de clé).
	Key int64
}

// StructureFunc fait sur le serveur les changements de structure d'une
// transaction du bus (kind : command.KindDo, KindUndo, KindRedo), dans la
// transaction locale tx, avant sa validation ; une erreur la défait.
type StructureFunc func(ctx context.Context, tx *sql.Tx, kind string, changes []Change) error

// structure : partie du Recorder pour la structure (SetStructure).
type structure struct {
	mu     sync.Mutex
	fn     StructureFunc
	notice string
	// watch voit chaque instruction des étapes du bus (SetWatch).
	watch func(q string) error
}

// SetWatch fait voir à fn chaque instruction exécutée par une étape du bus
// (action, annulation, rétablissement), dans sa transaction ; une erreur
// défait toute la transaction. Travail à plusieurs sur un dossier partagé
// (internal/coedit) : ce qui ne se décrit pas en opérations sur les lignes
// des tableaux y est repéré. nil : rien.
func (r *Recorder) SetWatch(fn func(q string) error) {
	r.st.mu.Lock()
	r.st.watch = fn
	r.st.mu.Unlock()
}

func (r *Recorder) watchFn() func(q string) error {
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	return r.st.watch
}

// SetStructure confie les changements de structure des tableaux publiés à
// fn (synchronisation d'un classeur collaboratif) ; nil : sans
// synchronisation, ils passent par Gate (refusés dans un classeur
// collaboratif, gardés en local dans un classeur figé).
func (r *Recorder) SetStructure(fn StructureFunc) {
	r.st.mu.Lock()
	r.st.fn = fn
	r.st.mu.Unlock()
}

func (r *Recorder) structureFn() StructureFunc {
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	return r.st.fn
}

// TakeNotice rend (et oublie) le message laissé par la dernière
// annulation (tableau rétabli sur ce poste seulement), "" sinon.
func (r *Recorder) TakeNotice() string {
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	n := r.st.notice
	r.st.notice = ""
	return n
}

func (r *Recorder) setNotice(msg string) {
	r.st.mu.Lock()
	r.st.notice = msg
	r.st.mu.Unlock()
}

// structural : opération « meta » de structure relevée par la comparaison
// des données (tableau ou colonne), confiée à la synchronisation quand
// elle est en place.
func structural(op command.Op) bool {
	return op.Kind == command.OpMeta && op.Meta != nil && (op.Meta.Object == "table" || op.Meta.Object == "column")
}

// tshape est la forme d'un tableau : nom, clé, colonnes dans l'ordre.
type tshape struct {
	name string
	key  int64
	cols []cshape
}

type cshape struct {
	id        int64
	name, typ string
	formula   bool
}

// shapes lit la forme des tableaux ids (absents : supprimés).
func shapes(ctx context.Context, db snapshot, ids []int64) (map[int64]*tshape, error) {
	list := idList(ids)
	out := map[int64]*tshape{}
	rs, err := db.QueryContext(ctx, `SELECT id, name, coalesce(key_col, 0) FROM _lattis_tables WHERE id IN (SELECT value FROM json_each(?))`, list)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var id int64
		t := &tshape{}
		if err := rs.Scan(&id, &t.name, &t.key); err != nil {
			rs.Close()
			return nil, err
		}
		out[id] = t
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	rs, err = db.QueryContext(ctx, `SELECT tbl, id, name, type, coalesce(formula, '') <> '' FROM _lattis_columns WHERE tbl IN (SELECT value FROM json_each(?)) ORDER BY tbl, pos`, list)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var tbl int64
		var c cshape
		if err := rs.Scan(&tbl, &c.id, &c.name, &c.typ, &c.formula); err != nil {
			return nil, err
		}
		if t := out[tbl]; t != nil {
			t.cols = append(t.cols, c)
		}
	}
	return out, rs.Err()
}

// changes compare la forme des tableaux publiés avant et après.
func changes(ids []int64, before, after map[int64]*tshape) []Change {
	var out []Change
	for _, id := range ids {
		b, a := before[id], after[id]
		if b == nil {
			continue
		}
		if a == nil {
			out = append(out, Change{Table: id, Action: DropTable, Name: b.name})
			continue
		}
		if a.name != b.name {
			out = append(out, Change{Table: id, Action: RenameTable, Name: a.name})
		}
		bcols := map[int64]cshape{}
		for _, c := range b.cols {
			bcols[c.id] = c
		}
		acols := map[int64]bool{}
		for _, c := range a.cols {
			acols[c.id] = true
		}
		for _, c := range b.cols {
			if !c.formula && !acols[c.id] {
				out = append(out, Change{Table: id, Action: DropColumn, Column: c.id, Name: c.name})
			}
		}
		for i, c := range a.cols {
			if c.formula {
				continue
			}
			bc, had := bcols[c.id]
			switch {
			case !had:
				var prev []int64
				for k := i - 1; k >= 0; k-- {
					if !a.cols[k].formula {
						prev = append(prev, a.cols[k].id)
					}
				}
				out = append(out, Change{Table: id, Action: AddColumn, Column: c.id, Name: c.name, Type: c.typ, Prev: prev})
			case bc.formula:
				// Colonne calculée devenue saisie : elle reste propre au poste
				// (les colonnes calculées ne sont pas publiées).
			default:
				if bc.name != c.name {
					out = append(out, Change{Table: id, Action: RenameColumn, Column: c.id, Name: c.name})
				}
				if bc.typ != c.typ {
					out = append(out, Change{Table: id, Action: RetypeColumn, Column: c.id, Type: c.typ})
				}
			}
		}
		if a.key != b.key {
			out = append(out, Change{Table: id, Action: SetKey, Key: a.key})
		}
	}
	return out
}

// ErrImplicitDrop refuse qu'un tableau partagé disparaisse autrement que
// par sa suppression voulue (actualisation d'une requête qui le remplace,
// déplacement, fusion, conversion en plage, feuille supprimée) : ce serait
// le supprimer du serveur pour tous.
type ErrImplicitDrop struct{ Table string }

func (e ErrImplicitDrop) Error() string {
	return "Le tableau partagé « " + e.Table + " » ne peut pas être remplacé ni retiré par cette action : supprimez-le d'abord avec « Supprimer le tableau », qui le retire aussi du serveur"
}

// dropWanted dit si la transaction a voulu supprimer le tableau tbl
// (canvas.Model.DeleteTables enregistre une opération « table drop »).
func dropWanted(ops []command.Op, tbl int64) bool {
	return slices.ContainsFunc(ops, func(op command.Op) bool {
		return op.Kind == command.OpMeta && op.Table == tbl && op.Meta != nil && op.Meta.Object == "table" && op.Meta.Action == "drop"
	})
}

// structure compare la forme des tableaux publiés avant et après la
// transaction et confie les changements à fn ; ops : opérations
// enregistrées (Tx.Record).
func (s *session) structure(fn StructureFunc, kind string, ops []command.Op) error {
	ids := make([]int64, 0, len(s.pub))
	for id := range s.pub {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	before, err := s.r.db.Read().BeginTx(s.ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer before.Rollback()
	b, err := shapes(s.ctx, before, ids)
	if err != nil {
		return err
	}
	a, err := shapes(s.ctx, s.tx, ids)
	if err != nil {
		return err
	}
	ch := changes(ids, b, a)
	if len(ch) == 0 {
		return nil
	}
	for _, c := range ch {
		if c.Action == DropTable && !dropWanted(ops, c.Table) {
			return ErrImplicitDrop{Table: c.Name}
		}
	}
	return fn(s.ctx, s.tx, kind, ch)
}

// restored note un tableau supprimé du serveur (sa ligne de
// _lattis_remote_tables garde son nom, sans identifiant) qu'une annulation
// rétablit : il ne revient que sur ce poste, comme tableau local.
func (s *session) restored() error {
	rs, err := s.tx.QueryContext(s.ctx, `SELECT tbl, remote_name FROM _lattis_remote_tables WHERE published = 0 AND remote_id IS NULL AND remote_name IS NOT NULL`)
	if err != nil {
		return err
	}
	type gone struct {
		id   int64
		name string
	}
	var all []gone
	for rs.Next() {
		var g gone
		if err := rs.Scan(&g.id, &g.name); err != nil {
			rs.Close()
			return err
		}
		all = append(all, g)
	}
	rs.Close()
	if err := rs.Err(); err != nil || len(all) == 0 {
		return err
	}
	for _, g := range all {
		var now, was int
		q := `SELECT count(*) FROM main.sqlite_master WHERE type = 'table' AND name = ?`
		if err := s.tx.QueryRowContext(s.ctx, q, tableName(g.id)).Scan(&now); err != nil {
			return err
		}
		if now == 0 {
			continue
		}
		if err := s.r.db.Read().QueryRowContext(s.ctx, q, tableName(g.id)).Scan(&was); err != nil {
			return err
		}
		if was == 0 {
			s.r.setNotice("Le tableau « " + g.name + " » est rétabli sur ce poste seulement : il a été supprimé du serveur et n'est plus partagé")
		}
	}
	return nil
}
