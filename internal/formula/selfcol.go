package formula

import (
	"fmt"
	"strings"

	"github.com/ncruces/go-sqlite3"
)

// Colonnes qui lisent leurs propres lignes voisines : un total cumulé
// =[Qté]+[Cumul][-1] dépend de la valeur de la même colonne à la ligne
// précédente. Ce n'est pas une référence circulaire (chaque ligne ne dépend
// que des lignes d'avant), mais la formule ne se recopie pas en ligne sans
// fin : la colonne est calculée une fois, ligne après ligne dans l'ordre du
// tableau (requête récursive), et chaque ligne lit ensuite sa valeur.

// recStep : compilation du pas de la requête récursive. Sur la ligne alias,
// [Col][-k] (ou [Col][+k] en remontant) se lit dans la colonne vk de la
// ligne précédente du parcours (rec).
type recStep struct {
	col   *Column
	alias string
	rec   string
	dir   int // -1 : lignes précédentes ([Col][-k]) ; +1 : lignes suivantes
}

// selfOffsets cherche dans la formule de la colonne col (du tableau t) les
// références [col][±k] à ses propres lignes voisines : rend le plus grand k
// et le sens ; faux s'il n'y en a pas, ou si elles vont dans les deux sens
// (la boucle reste alors une référence circulaire).
func selfOffsets(n Node, t *Table, col *Column) (maxK, dir int, ok bool) {
	mixed := false
	var walk func(Node)
	walk = func(n Node) {
		switch n := n.(type) {
		case Unary:
			walk(n.X)
		case Binary:
			walk(n.L)
			walk(n.R)
		case Call:
			if (n.Fn == "PREVIOUS" || n.Fn == "NEXT") && len(n.Args) > 0 {
				// PRECEDENT([Solde]) : [Solde][-1], avec sa valeur par défaut.
				if r, isRef := n.Args[0].(Ref); isRef && (r.Table == "" || strings.EqualFold(r.Table, t.Name)) {
					if i, found := t.col(r.Col); found && &t.Cols[i] == col {
						k := -1
						if n.Fn == "NEXT" {
							k = 1
						}
						n = Call{Fn: "ITEM", Args: []Node{Ref{Col: r.Col}, Unary{Op: map[int]string{-1: "-", 1: "+"}[k], X: NumLit{V: 1}}}}
						walk(n)
						return
					}
				}
			}
			if k, isRel := relativeOffset(n); isRel && k != 0 {
				if r, isRef := n.Args[0].(Ref); isRef && (r.Table == "" || strings.EqualFold(r.Table, t.Name)) {
					if i, found := t.col(r.Col); found && &t.Cols[i] == col {
						d := 1
						if k < 0 {
							d, k = -1, -k
						}
						if dir != 0 && dir != d {
							mixed = true
						}
						dir, maxK = d, max(maxK, k)
						return
					}
				}
			}
			for _, a := range n.Args {
				walk(a)
			}
		}
	}
	walk(n)
	return maxK, dir, maxK > 0 && !mixed
}

// selfColumn précalcule une colonne qui lit ses propres lignes voisines, et
// rend le numéro du précalcul (lu par slt_seqof) ; faux pour une autre
// colonne, ou sans précalcul possible.
func (c *compiler) selfColumn(t *Table, col *Column) (float64, bool) {
	if c.hoist == nil || t == nil || col.Formula == nil {
		return 0, false
	}
	if seq, ok := c.seqs[col]; ok {
		return seq, seq >= 0 // -1 : précalcul en cours (boucle)
	}
	k, dir, ok := selfOffsets(col.Formula, t, col)
	if !ok {
		return 0, false
	}
	sub := c.sub() // crée c.seqs si besoin
	c.seqs[col] = -1
	defer func() {
		if c.seqs[col] < 0 {
			delete(c.seqs, col)
		}
	}()
	sub.visiting[col] = true
	cur, rec := sub.newAlias(), sub.newAlias()
	sub.rec = &recStep{col: col, alias: cur, rec: rec, dir: dir}
	val := sub.expr(col.Formula, scope{table: t, alias: cur})
	// Parcours par _pos (texte) : la ligne suivante est le plus petit _pos
	// au-delà du précédent (le plus grand en deçà en remontant). Le point de
	// départ est avant toutes les lignes : un nombre (avant tout texte) en
	// descendant, un blob (après tout texte) en remontant.
	start, next := "0", fmt.Sprintf("(SELECT min(_pos) FROM %s WHERE _pos > %s.p)", t.SQL, rec)
	if dir > 0 {
		start, next = "x''", fmt.Sprintf("(SELECT max(_pos) FROM %s WHERE _pos < %s.p)", t.SQL, rec)
	}
	vs, nulls, shift := make([]string, k), make([]string, k), []string{val}
	for i := range vs {
		vs[i] = fmt.Sprintf("v%d", i+1)
		nulls[i] = "NULL"
		if i > 0 {
			shift = append(shift, fmt.Sprintf("%s.v%d", rec, i))
		}
	}
	sql := fmt.Sprintf("(WITH RECURSIVE %[1]s(p, id, %[2]s) AS (SELECT %[3]s, NULL, %[4]s UNION ALL SELECT %[5]s._pos, %[5]s.id, %[6]s FROM %[1]s JOIN %[7]s AS %[5]s ON %[5]s._pos = %[8]s) SELECT slt_seqidx(id, v1) FROM %[1]s WHERE id IS NOT NULL)",
		rec, strings.Join(vs, ", "), start, strings.Join(nulls, ", "), cur, strings.Join(shift, ", "), t.SQL, next)
	id := c.hoist(Compiled{sql, sub.args})
	if id.Kind != Number {
		return 0, false
	}
	c.seqs[col] = id.Num
	return id.Num, true
}

// recRef donne, dans le pas de la requête récursive, la valeur de [col][k]
// sur la ligne courante ("" si la référence n'est pas celle du pas).
func (c *compiler) recRef(t *Table, col *Column, k int, sc scope) string {
	r := c.rec
	if r == nil || r.col != col || sc.alias != r.alias || sc.table != t {
		return ""
	}
	if k < 0 && r.dir < 0 {
		return fmt.Sprintf("%s.v%d", r.rec, -k)
	}
	if k > 0 && r.dir > 0 {
		return fmt.Sprintf("%s.v%d", r.rec, k)
	}
	return ""
}

// recPrevious compile, dans le pas de la requête récursive, PRECEDENT([col])
// (SUIVANT en remontant) : la valeur de la ligne d'avant, ou la valeur par
// défaut au bord du tableau.
func (c *compiler) recPrevious(n Call, sc scope) (string, bool) {
	r := c.rec
	if r == nil || sc.table == nil || sc.alias != r.alias {
		return "", false
	}
	ref, ok := n.Args[0].(Ref)
	if !ok || ref.Table != "" && !strings.EqualFold(ref.Table, sc.table.Name) {
		return "", false
	}
	i, ok := sc.table.col(ref.Col)
	k := -1
	if n.Fn == "NEXT" {
		k = 1
	}
	if !ok || c.recRef(sc.table, &sc.table.Cols[i], k, sc) == "" {
		return "", false
	}
	def := "NULL"
	if len(n.Args) > 1 {
		def = c.expr(n.Args[1], sc)
	}
	return fmt.Sprintf("(CASE WHEN %[1]s.id IS NULL THEN %[2]s ELSE %[1]s.v1 END)", r.rec, def), true
}

// seqIndex : valeur de chaque ligne (par identifiant) d'une colonne
// précalculée.
type seqIndex map[int64]Value

type seqAgg struct{ x seqIndex }

func (a *seqAgg) Step(ctx sqlite3.Context, arg ...sqlite3.Value) {
	if a.x == nil {
		a.x = seqIndex{}
	}
	a.x[arg[0].Int64()] = fromArg(arg[1])
}

func (a *seqAgg) Value(ctx sqlite3.Context) {
	if a.x == nil {
		a.x = seqIndex{} // tableau vide
	}
	ctx.ResultInt64(register(a.x))
}

func registerSeq(c *sqlite3.Conn) error {
	if err := c.CreateWindowFunction("slt_seqidx", 2, sqlite3.INNOCUOUS, func() sqlite3.AggregateFunction { return &seqAgg{} }); err != nil {
		return err
	}
	// slt_seqof(numéro, identifiant de ligne) : la valeur précalculée.
	return c.CreateFunction("slt_seqof", 2, sqlite3.INNOCUOUS, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		x, _ := lookupPrecomputed(arg[0].Int64()).(seqIndex)
		v, ok := x[arg[1].Int64()]
		if !ok {
			ctx.ResultNull()
			return
		}
		result(ctx, v)
	})
}
