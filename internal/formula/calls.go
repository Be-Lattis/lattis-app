package formula

import (
	"strings"
	"time"
)

// Code commun aux deux backends pour les fonctions qui parcourent des
// tableaux.

// nowSerial donne l'instant de AUJOURDHUI et MAINTENANT : fixé par le
// document à l'ouverture et sur F9 (Schema.Now), l'heure courante sinon.
func (s *Schema) nowSerial() float64 {
	if s != nil && !s.Now.IsZero() {
		return TimeSerial(s.Now)
	}
	return TimeSerial(time.Now())
}

func (s *Schema) seed() uint64 {
	if s == nil {
		return 0
	}
	return s.Seed
}

// scanCall décrit l'exécution d'une fonction de parcours.
type scanCall struct {
	def    scanDef
	name   string
	params []Node
	cols   []Node
	err    ErrCode
}

// scanOf répartit les arguments d'une fonction de parcours entre paramètres
// et colonnes. current est le tableau de la ligne courante (ou nil).
func scanOf(n Call, s *Schema, current *Table) scanCall {
	args := n.Args
	switch n.Fn {
	case "RANK":
		// RANG([@Score]) et RANG([@Score]; ordre) : rang dans la colonne
		// elle-même. Sinon, forme d'Excel : RANG(valeur; colonne; [ordre]).
		if r, ok := args[0].(Ref); ok && r.Row && (len(args) == 1 || len(args) == 2 && !hasColumnRef(args[1])) {
			col := Ref{Table: r.Table, Col: r.Col}
			args = append([]Node{args[0], col}, args[1:]...)
		}
	case "REFERENCE.APPROX":
		t, col, err := refTarget(s, n)
		if err.IsErr() {
			return scanCall{err: err.Err}
		}
		args = []Node{args[2], Ref{Table: t.Name, Col: t.Key}, Ref{Table: t.Name, Col: t.Cols[col].Name}}
	}
	def := scans[n.Fn]
	rs, ok := def.roles(len(args))
	if !ok {
		return scanCall{err: ErrValue}
	}
	sc := scanCall{def: def, name: n.Fn}
	for i, a := range args {
		if rs[i] == param {
			sc.params = append(sc.params, a)
		} else {
			sc.cols = append(sc.cols, a)
		}
	}
	return sc
}

// hasColumnRef dit si une expression contient une colonne entière.
func hasColumnRef(n Node) bool {
	found := false
	walkArray(n, func(r Ref) { found = true })
	return found
}

// walkArray appelle f sur chaque colonne entière (Tableau[Col] ou [Col]) lue
// ligne à ligne par une expression. Les agrégats et parcours imbriqués lisent
// leurs propres colonnes : seuls leurs paramètres sont visités.
func walkArray(n Node, f func(Ref)) {
	walkCols(n, func(x Node) {
		if r, ok := x.(Ref); ok {
			f(r)
		}
	})
}

// arrayTable donne le tableau parcouru par des expressions sur colonnes (nil
// si elles n'en lisent aucune). Des colonnes de tableaux différents ne
// peuvent pas être lues ensemble : #VALEUR!.
func arrayTable(nodes []Node, s *Schema, current *Table) (*Table, ErrCode) {
	var t *Table
	var e ErrCode
	for _, n := range nodes {
		walkArray(n, func(r Ref) {
			rt := current
			if r.Table != "" && s != nil {
				var ok bool
				if rt, ok = s.Table(r.Table); !ok {
					return // tableau inconnu : #REF! à l'évaluation
				}
			}
			if rt == nil {
				return
			}
			if t != nil && t != rt {
				e = ErrValue
			}
			t = rt
		})
	}
	if e != 0 {
		return nil, e
	}
	return t, 0
}

// existsTarget lit EXISTE(Tableau; clé) : le tableau et sa colonne clé.
func existsTarget(s *Schema, n Call) (*Table, int, Value) {
	tn, ok := nameOf(n.Args[0])
	if !ok {
		return nil, 0, Err(ErrValue)
	}
	t, ok := s.Table(tn)
	if !ok || t.Key == "" {
		return nil, 0, Err(ErrRef)
	}
	k, ok := t.col(t.Key)
	if !ok {
		return nil, 0, Err(ErrRef)
	}
	return t, k, Value{}
}

// subtotalOf lit le code de SOUS.TOTAL, qui doit être écrit en toutes
// lettres (SOUS.TOTAL(9; Ventes[Montant])).
func subtotalOf(n Call) (string, Value) {
	lit, ok := n.Args[0].(NumLit)
	if !ok {
		return "", Err(ErrValue)
	}
	fn, ok := subtotalFn(lit.V)
	if !ok {
		return "", Err(ErrValue)
	}
	return fn, Value{}
}

// KeyTargets donne les tableaux où une formule cherche une ligne
// (REFERENCE, REFERENCE.APPROCHEE, EXISTE) : par leur clé, ou sans clé par
// le numéro de ligne (voir rankReference).
func KeyTargets(n Node) []string {
	var out []string
	var walk func(Node)
	walk = func(n Node) {
		switch n := n.(type) {
		case Unary:
			walk(n.X)
		case Binary:
			walk(n.L)
			walk(n.R)
		case Call:
			switch strings.ToUpper(n.Fn) {
			case "REFERENCE", "REFERENCE.APPROX", "EXISTS":
				if len(n.Args) > 0 {
					if name, ok := nameOf(n.Args[0]); ok {
						out = append(out, name)
					}
				}
			}
			for _, a := range n.Args {
				walk(a)
			}
		}
	}
	walk(n)
	return out
}
