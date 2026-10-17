package demo

import (
	"fmt"
	"strings"

	"github.com/lalamefine/slitable/internal/order"
)

// Génération en SQL des très gros presets (10M lignes) : les lignes ne
// passent pas par Go. Une CTE récursive numérote les lignes, chaque valeur
// est une expression SQL déterministe de son numéro n et de la graine, et
// canvas.Import les insère d'un seul INSERT … SELECT (index d'ordre créé
// après le remplissage). Les données suivent les mêmes thèmes que la
// génération en Go, avec un tirage différent.

// sqlGen écrit les expressions d'un tirage pseudo-aléatoire en SQL. Chaque
// appel à r prend un nouveau « sel » : deux colonnes ne tirent pas la même
// valeur, et une valeur relue plus loin (colonne dérivée) se recalcule à
// l'identique.
type sqlGen struct {
	seed uint64
	salt int
}

// r : entier pseudo-aléatoire dans [0, 2^31 - 1[ (deux tours de générateur
// congruentiel, sans dépasser les entiers 64 bits de SQLite).
func (g *sqlGen) r() string {
	g.salt++
	off := g.seed%100_000*7919 + uint64(g.salt)*104_729
	return fmt.Sprintf("((((n + %d) * 1103515245 + 12345) %% 2147483648) * 48271 %% 2147483647)", off)
}

// u : décimal dans [0, 1[.
func (g *sqlGen) u() string { return "(" + g.r() + " / 2147483647.0)" }

// fast : décimal dans [0, 1[ en une multiplication et un modulo, pour les
// dizaines de colonnes de capteurs (moins aléatoire que u, bien assez pour
// un bruit de mesure).
func (g *sqlGen) fast() string {
	g.salt++
	mul := 7919 + 104_729*(g.salt%97) + int(g.seed%1000)*31
	return fmt.Sprintf("(((n * %d + %d) %% 10007) / 10007.0)", mul, g.salt*131)
}

// noise : bruit centré d'écart type sd (uniforme, avec fast).
func (g *sqlGen) noise(sd float64) string {
	return fmt.Sprintf("((%s - 0.5) * %g)", g.fast(), sd*3.4641016)
}

// intIn : entier dans [lo, hi].
func (g *sqlGen) intIn(lo, hi int) string { return fmt.Sprintf("(%d + %s %% %d)", lo, g.r(), hi-lo+1) }

// chance : vrai (1) avec la probabilité p.
func (g *sqlGen) chance(p float64) string { return fmt.Sprintf("(%s < %d)", g.r(), int(p*2147483647)) }

// skewed : indice dans [0, k[, les premiers plus souvent (comme Gen.Skewed).
func (g *sqlGen) skewed(k int) string {
	u := g.u()
	return fmt.Sprintf("min(%d, CAST(%s * %s * %d AS INTEGER))", k-1, u, u, k)
}

// normal : loi normale approchée (somme de quatre uniformes).
func (g *sqlGen) normal(mean, sd float64) string {
	return fmt.Sprintf("(%g + %g * ((%s + %s + %s + %s - 2.0) * 1.7320508))", mean, sd, g.u(), g.u(), g.u(), g.u())
}

// pick : un élément d'une liste de textes, à l'indice idx.
func pick(idx string, list []string) string {
	var b strings.Builder
	b.WriteString("CASE " + idx)
	for i, s := range list {
		fmt.Fprintf(&b, " WHEN %d THEN %s", i, sqlText(s))
	}
	b.WriteString(" END")
	return b.String()
}

func pickNum(idx string, list []float64) string {
	var b strings.Builder
	b.WriteString("CASE " + idx)
	for i, v := range list {
		fmt.Fprintf(&b, " WHEN %d THEN %g", i, v)
	}
	b.WriteString(" END")
	return b.String()
}

func sqlText(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// sqlDay : date ISO d'un jour compté depuis le 1er janvier 2020.
func sqlDay(expr string) string { return "date(2458849.5 + " + expr + ")" }

// numbers : CTE récursive des numéros de ligne 1 à rows.
func numbers(rows int) string {
	return fmt.Sprintf("WITH RECURSIVE s(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM s WHERE n < %d)", rows)
}

// Blocs : le tirage complet (coûteux en SQL) ne se fait que sur deux petits
// blocs de lignes, de tailles premières entre elles ; la ligne n combine la
// ligne n mod blockA du premier et n mod blockB du second : les combinaisons
// ne se répètent qu'au bout de blockA × blockB lignes, et chaque ligne ne
// coûte plus que deux recherches et quelques opérations. Le remplissage
// devient l'essentiel du temps.
const (
	blockA = 1009
	blockB = 997
)

// block écrit la CTE matérialisée d'un bloc : colonne i (0 à size - 1) puis
// x1…xk. from est la source des lignes (sous-requête sur z, qui numérote n).
func block(name string, size int, from string, exprs []string) string {
	cols := make([]string, len(exprs))
	for i, e := range exprs {
		cols[i] = fmt.Sprintf("%s AS x%d", e, i+1)
	}
	return fmt.Sprintf("%s AS MATERIALIZED (SELECT n - 1 AS i, %s FROM %s WHERE n <= %d)", name, strings.Join(cols, ", "), from, size)
}

// blockQuery assemble la requête : numéros s (rows lignes) et z (lignes des
// blocs), blocs a et b, puis colonnes v1…vk de la ligne n.
func blockQuery(rows int, a, b string, exprs []string) string {
	cols := make([]string, len(exprs))
	for i, e := range exprs {
		cols[i] = fmt.Sprintf("%s AS v%d", e, i+1)
	}
	return fmt.Sprintf("WITH RECURSIVE s(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM s WHERE n < %d), "+
		"z(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM z WHERE n < %d), %s, %s "+
		"SELECT s.n AS n, %s FROM s JOIN a ON a.i = s.n %% %d JOIN b ON b.i = s.n %% %d",
		rows, max(blockA, blockB), a, b, strings.Join(cols, ", "), blockA, blockB)
}

// SQLRows écrit la requête qui génère rows lignes des cols premières
// colonnes du thème (Commandes : 10 colonnes au plus ; Relevés : toutes) ;
// ok est faux pour un thème sans version SQL.
func SQLRows(th Theme, cols, rows int, seed uint64) (string, bool) {
	g := &sqlGen{seed: seed}
	switch th.Name {
	case Commandes.Name:
		if cols > 10 {
			return "", false
		}
		names := make([]string, len(produits))
		cats := make([]string, len(produits))
		prix := make([]float64, len(produits))
		for i, p := range produits {
			names[i], cats[i], prix[i] = p.Nom, p.Categorie, p.Prix
		}
		vn := make([]string, len(villes))
		for i, v := range villes {
			vn[i] = v.Nom
		}
		// Bloc a : date, client, ville ; bloc b : la ligne de commande.
		a := block("a", blockA, fmt.Sprintf("(SELECT n, %s AS d, %s AS vi FROM z)", g.intIn(0, today-1), g.skewed(len(villes))), []string{
			sqlDay("d"),
			pick(g.intIn(0, len(prenoms)-1), prenoms) + " || ' ' || " + pick(g.intIn(0, len(noms)-1), noms),
			pick("vi", vn),
		})
		pu := "round(" + pickNum("p", prix) + " * f, 2)"
		b := block("b", blockB, fmt.Sprintf("(SELECT n, %s AS p, %s AS q, %s AS f FROM z)", g.skewed(len(produits)), "(1 + "+g.skewed(10)+")", "(0.92 + 0.16 * "+g.u()+")"), []string{
			pick("p", names),
			pick("p", cats),
			"q",
			pu,
			"round(q * " + pu + ", 2)",
			g.chance(0.88),
		})
		exprs := []string{"100000 + s.n", "a.x1", "a.x2", "a.x3", "b.x1", "b.x2", "b.x3", "b.x4", "b.x5", "b.x6"}
		return blockQuery(rows, a, b, exprs[:cols]), true
	case Releves.Name:
		// Bloc a : un relevé complet (station, jour, heure, température…) ;
		// bloc b : les écarts des capteurs supplémentaires.
		base := fmt.Sprintf("(SELECT n, %s AS st, %s AS d, %s AS h FROM z)", g.intIn(0, len(stations)-1), g.intIn(3*365, today-1), g.intIn(0, 23))
		temp := fmt.Sprintf("round(12 - 8 * cos(2 * pi() * ((d %% 365) - 15) / 365.0) - 4 * cos(2 * pi() * (h - 3) / 24.0) + %s, 1)", g.normal(0, 2.5))
		vent := "CAST(abs(" + g.normal(12, 12) + ") AS INTEGER)"
		pluie := fmt.Sprintf("CASE WHEN %s THEN 0.0 ELSE round(-3 * ln(1 - 0.999 * %s), 1) END", g.chance(0.78), g.u())
		c := fmt.Sprintf("(SELECT x.*, %s AS t, %s AS vent, %s AS pluie FROM %s AS x)", temp, vent, pluie, base)
		hum := fmt.Sprintf("CAST(max(15, min(100, 95 - 1.8 * t + %s)) AS INTEGER)", g.normal(0, 8))
		e := fmt.Sprintf("(SELECT y.*, %s AS hum FROM %s AS y)", hum, c)
		a := block("a", blockA, e, []string{
			pick("st", stations),
			sqlDay("d"),
			"h",
			"t",
			"hum",
			"round(" + g.normal(1015, 8) + ", 1)",
			"vent",
			"pluie",
			fmt.Sprintf("CAST(max(0, sin(pi() * (h - 6) / 14.0)) * (200 + 700 * %s) AS INTEGER)", g.u()),
			"(vent > 50 OR pluie > 12)",
		})
		exprs := []string{"a.x1", "a.x2", "a.x3", "a.x4", "a.x5", "a.x6", "a.x7", "a.x8", "a.x9", "a.x10"}
		var deltas []string
		for k := 0; len(exprs) < cols; k++ {
			x := fmt.Sprintf("b.x%d", len(deltas)+1)
			switch k % 3 {
			case 0:
				deltas = append(deltas, "round("+g.normal(0, 0.6)+", 1)")
				exprs = append(exprs, "round(a.x4 + "+x+", 1)")
			case 1:
				deltas = append(deltas, "CAST(round("+g.normal(0, 3)+") AS INTEGER)")
				exprs = append(exprs, "max(0, min(100, a.x5 + "+x+"))")
			default:
				deltas = append(deltas, g.chance(0.97))
				exprs = append(exprs, x)
			}
		}
		if len(deltas) == 0 {
			deltas = []string{"0"}
		}
		b := block("b", blockB, "z", deltas)
		return blockQuery(rows, a, b, exprs[:cols]), true
	}
	return "", false
}

// copyBlock : lignes tirées une fois (choix de Louis : 5 000 lignes
// aléatoires suffisent), puis recopiées à la suite jusqu'au total ;
// copyChunk : lignes écrites par requête (la progression avance entre deux).
const (
	copyBlock = 5_000
	copyChunk = 250_000
)

// CopiedRows écrit la requête des très gros presets : copyBlock lignes
// tirées par SQLRows, recopiées autant de fois qu'il faut par un produit
// croisé avec les numéros de copie j. Les clés d'ordre (colonne k) sont le
// numéro de copie en base 62 suivi de la clé de la ligne dans le bloc : pas
// de calcul de clé par ligne, et des clés déjà croissantes. Les colonnes
// qui numérotent (N° commande) restent uniques.
//
// first et last (0 à rows - 1) bornent les lignes produites : un morceau
// de la génération (multiple de copyBlock pour first, sauf la fin).
func CopiedRows(th Theme, cols, rows int, seed uint64, first, last int) (string, bool) {
	size := min(rows, copyBlock)
	inner, ok := SQLRows(th, cols, size, seed)
	if !ok {
		return "", false
	}
	copies := (rows + size - 1) / size
	exprs := make([]string, cols)
	for i := range exprs {
		exprs[i] = fmt.Sprintf("b.v%d AS v%d", i+1, i+1)
	}
	n := fmt.Sprintf("(c.j * %d + b.n)", size)
	if th.Name == Commandes.Name {
		exprs[0] = "100000 + " + n + " AS v1"
	}
	return fmt.Sprintf("WITH RECURSIVE c(j) AS (SELECT %d UNION ALL SELECT j + 1 FROM c WHERE j < %d), "+
		"b AS MATERIALIZED (SELECT q.*, %s AS sk FROM (%s) AS q) "+
		"SELECT %s AS n, %s || b.sk AS k, %s FROM c CROSS JOIN b WHERE %s BETWEEN %d AND %d",
		first/size, last/size, order.SpreadSQL(size, "q.n"), inner, n, order.FixedSQL(order.FixedWidth(copies), "c.j"), strings.Join(exprs, ", "), n, first+1, last+1), true
}
