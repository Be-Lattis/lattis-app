package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Création d'office défaite par Ctrl+Z (demande de Louis).
//
// Quand une saisie ou un collage fait naître un tableau, le premier Ctrl+Z
// ne défait que la création : les cellules restent libres, avec la
// dernière valeur saisie ou collée, et la zone est marquée « plage » (elle
// ne refait pas de tableau d'elle-même). Le second Ctrl+Z annule la saisie.
// L'action s'enregistre en deux étapes du journal (command.Tx.Detach) : la
// saisie suivie de la conversion en plage, puis la conversion défaite.

// birthStep est le début du nom de l'étape qui crée le tableau.
const birthStep = "Création "

// bornTable : tableau créé d'office par l'action, et son étendue d'alors.
type bornTable struct {
	id                      int64
	row, col, bottom, right int
}

// noteBorn retient un tableau créé d'office.
func (t *tx) noteBorn(tb Table) {
	t.born = append(t.born, bornTable{tb.ID, tb.Row, tb.Col, tb.Bottom(), tb.Right()})
}

// detachBirth prépare le Ctrl+Z en deux temps à la fin d'une saisie ou d'un
// collage qui a créé des tableaux restés tels quels (un tableau ensuite
// fusionné ou agrandi par le rattachement n'est pas concerné).
func (t *tx) detachBirth() error {
	if !t.undoBirth || len(t.born) == 0 || !t.CanDetach() {
		return nil
	}
	var tables []Table
	seen := map[int64]bool{}
	for _, b := range t.born {
		if seen[b.id] {
			continue
		}
		seen[b.id] = true
		tb, err := t.table(b.id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if tb.Row != b.row || tb.Col != b.col || tb.Bottom() != b.bottom || tb.Right() != b.right ||
			(tb.Rows+1)*len(tb.Cols) > MaxRangeCells {
			continue
		}
		tables = append(tables, tb)
	}
	if len(tables) == 0 {
		return nil
	}
	mark, notes := t.Mark(), len(t.notes)
	names := make([]string, len(tables))
	for i, tb := range tables {
		names[i] = "« " + tb.Name + " »"
		if err := t.toRange(tb); err != nil {
			return err
		}
	}
	t.notes = t.notes[:notes]
	name := birthStep + "du tableau " + names[0]
	if len(names) > 1 {
		name = birthStep + "des tableaux " + strings.Join(names, ", ")
	}
	return t.Detach(mark, name)
}

// splitBirth dit si l'action qui vient de créer un tableau s'annule en deux
// temps (Event.Split).
func (m *Model) splitBirth(ctx context.Context, ev Event, err error) bool {
	if err != nil || ev.Created == "" {
		return false
	}
	st, err := m.bus.State(ctx)
	return err == nil && strings.HasPrefix(st.Undo, birthStep)
}

// undone dit, dans la barre d'état, qu'un Ctrl+Z vient de défaire la
// création d'un tableau (le suivant annulera la saisie).
func (m *Model) undone(name string) {
	what, ok := strings.CutPrefix(name, birthStep)
	if !ok || !strings.HasPrefix(what, "du tableau ") && !strings.HasPrefix(what, "des tableaux ") {
		return
	}
	next := "l'action"
	if st, err := m.bus.State(context.Background()); err == nil {
		switch st.Undo {
		case "Saisie":
			next = "la saisie"
		case "Coller":
			next = "le collage"
		case "":
		default:
			next = "« " + st.Undo + " »"
		}
	}
	msg := fmt.Sprintf("Tableau %s défait : les cellules restent libres (Ctrl+Z de nouveau annule %s)", strings.TrimPrefix(what, "du tableau "), next)
	if tables, ok := strings.CutPrefix(what, "des tableaux "); ok {
		msg = fmt.Sprintf("Tableaux %s défaits : les cellules restent libres (Ctrl+Z de nouveau annule %s)", tables, next)
	}
	m.note([]string{msg})
}
