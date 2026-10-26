package canvas

import (
	"math"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// Grouper un champ d'un tableau croisé (« Grouper… » de son menu dans le
// volet), comme un tableur :
//
//   - une date par années, trimestres, mois et jours, plusieurs niveaux à la
//     fois : le champ garde le plus fin (Date : les mois), les plus larges
//     deviennent des champs à part (Années, Trimestres), placés devant lui ;
//     chaque niveau est indépendant (T1 regroupe les premiers trimestres de
//     toutes les années, sauf avec Années devant) ;
//   - un nombre par intervalles (début, fin, pas) : « 0-99 », « 100-199 »,
//     « <0 », « >1000 ».
//
// Le groupe est calculé en SQL, ligne par ligne : une clé numérique (année,
// numéro de mois, rang de l'intervalle) dans la source du TCD, par
// laquelle il est rangé (2024 avant 2025, janvier avant février), et son
// étiquette, par laquelle il est regroupé et filtré.

// PivotGroup est un champ groupé.
type PivotGroup struct {
	Field string `json:"field"`
	// By : niveaux d'une date, du plus large au plus fin : years, quarters,
	// months, days ; vide : intervalles de nombres (Start, End, Step).
	By []string `json:"by,omitempty"`
	// Names : noms des champs ajoutés pour les niveaux plus larges que le
	// dernier (By sans son dernier) ; à défaut, « Années »…
	Names []string `json:"names,omitempty"`
	// Intervalles : depuis Start (0 sans), par pas de Step ; les nombres
	// sous Start ou au-dessus de End forment « <Start » et « >End ».
	Start *float64 `json:"start,omitempty"`
	End   *float64 `json:"end,omitempty"`
	Step  float64  `json:"step,omitempty"`
}

// dateLevels : les niveaux d'une date, du plus large au plus fin.
var dateLevels = []string{"years", "quarters", "months", "days"}

// groupNames donne le nom de chaque niveau d'un groupe de dates (le dernier
// est le champ lui-même).
func groupNames(g PivotGroup, lang string) []string {
	out := make([]string, len(g.By))
	for i, by := range g.By {
		switch {
		case i == len(g.By)-1:
			out[i] = g.Field
		case i < len(g.Names) && strings.TrimSpace(g.Names[i]) != "":
			out[i] = strings.TrimSpace(g.Names[i])
		default:
			out[i] = msg.Format(pivotLang(lang), msg.ID("pivot.group."+by), nil)
		}
	}
	return out
}

// groupField trouve un champ groupé : le champ lui-même (son niveau le plus
// fin, ou ses intervalles) ou un niveau ajouté (Années). ok faux : name
// n'est pas un champ groupé.
func (t *tx) groupField(p *pivotPlan, name string, base func(string) (*Column, error), known map[string]*Column) (*Column, bool, error) {
	for _, g := range p.cfg.Groups {
		level, found := "", false
		if len(g.By) == 0 {
			found = strings.EqualFold(g.Field, name)
		} else {
			for i, n := range groupNames(g, p.cfg.Lang) {
				if strings.EqualFold(n, name) {
					level, found = g.By[i], true
					break
				}
			}
		}
		if !found {
			continue
		}
		key := "g:" + strings.ToLower(name)
		if c, ok := known[key]; ok {
			return c, true, nil
		}
		b, err := base(g.Field)
		if err != nil {
			return nil, true, err
		}
		c, err := p.groupColumn(g, level, b, name)
		if err != nil {
			return nil, true, err
		}
		known[key] = c
		return c, true, nil
	}
	return nil, false, nil
}

// groupColumn ajoute à la source la clé du groupe (niveau level d'une date,
// ou rang de l'intervalle d'un nombre) et rend sa colonne.
func (p *pivotPlan) groupColumn(g PivotGroup, level string, b *Column, name string) (*Column, error) {
	x := p.exprOf(b)
	var key, label string
	if len(g.By) > 0 {
		for _, by := range g.By {
			if indexOf(dateLevels, by) < 0 {
				return nil, msg.Wrap(ErrPivot, "pivot.groupLevel", msg.Args{"field": g.Field})
			}
		}
		key, label = dateGroupSQL(level, pivotDateSQL(x), p.cfg.Lang)
	} else {
		var err error
		if key, label, err = numberGroupSQL(g, x, b.Type == grid.Integer, p.cfg.Lang); err != nil {
			return nil, err
		}
	}
	id := p.addExtra(key)
	if p.labels == nil {
		p.labels = map[int64]string{}
	}
	p.labels[id] = strings.ReplaceAll(label, "{k}", colSQL(id))
	return &Column{ID: id, Name: name, Type: grid.Decimal}, nil
}

func indexOf(list []string, s string) int {
	for i, x := range list {
		if x == s {
			return i
		}
	}
	return -1
}

// pivotDateSQL : la date ISO (« 2024-03-15… ») d'une valeur : texte d'une
// colonne Date, ou numéro de série d'une formule (1 = 1er janvier 1900) ;
// NULL pour autre chose.
func pivotDateSQL(x string) string {
	return "(CASE WHEN typeof(" + x + ") IN ('integer', 'real') THEN date(" + x + " + 2415018.5) WHEN " + x +
		" GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]*' THEN " + x + " END)"
}

// dateGroupSQL : clé et étiquette ({k} : la clé) d'un niveau de date.
func dateGroupSQL(level, d, lang string) (key, label string) {
	month := "CAST(substr(" + d + ", 6, 2) AS INTEGER)"
	switch level {
	case "years":
		return "CAST(substr(" + d + ", 1, 4) AS INTEGER)", "CAST({k} AS TEXT)"
	case "quarters":
		q := msg.Format(pivotLang(lang), "pivot.group.quarter", nil)
		return "(" + month + " + 2) / 3", "replace(" + sqlLiteral(q) + ", '{n}', {k})"
	case "days":
		return month + " * 100 + CAST(substr(" + d + ", 9, 2) AS INTEGER)",
			"printf('%02d', {k} % 100) || '-' || " + monthNamesSQL("{k} / 100", lang)
	}
	return month, monthNamesSQL("{k}", lang)
}

// monthNamesSQL : le nom abrégé du mois k (1 à 12).
func monthNamesSQL(k, lang string) string {
	names := strings.Split(msg.Format(pivotLang(lang), "pivot.group.monthNames", nil), ",")
	var b strings.Builder
	b.WriteString("CASE " + k)
	for i, n := range names {
		b.WriteString(" WHEN " + strconv.Itoa(i+1) + " THEN " + sqlLiteral(strings.TrimSpace(n)))
	}
	b.WriteString(" END")
	return b.String()
}

// Clés des nombres hors bornes (sous Start, au-dessus de End).
const (
	groupBelow = "-1000000000000000000"
	groupAbove = "1000000000000000000"
)

// numberGroupSQL : clé (rang de l'intervalle) et étiquette d'un nombre
// groupé par intervalles.
func numberGroupSQL(g PivotGroup, x string, integer bool, lang string) (key, label string, err error) {
	if g.Step <= 0 || math.IsInf(g.Step, 0) || math.IsNaN(g.Step) || (g.Start != nil && g.End != nil && *g.End < *g.Start) {
		return "", "", msg.Wrap(ErrPivot, "pivot.groupStep", msg.Args{"field": g.Field})
	}
	s0 := 0.0
	if g.Start != nil {
		s0 = *g.Start
	}
	num := func(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }
	v := "((" + x + " - " + num(s0) + ") * 1.0 / " + num(g.Step) + ")"
	floor := "(CAST(" + v + " AS INTEGER) - (" + v + " < CAST(" + v + " AS INTEGER)))"
	var b strings.Builder
	b.WriteString("CASE WHEN typeof(" + x + ") NOT IN ('integer', 'real') THEN NULL")
	if g.Start != nil {
		b.WriteString(" WHEN " + x + " < " + num(*g.Start) + " THEN " + groupBelow)
	}
	if g.End != nil {
		b.WriteString(" WHEN " + x + " > " + num(*g.End) + " THEN " + groupAbove)
	}
	b.WriteString(" ELSE " + floor + " END")
	key = b.String()
	sep := "."
	if l := pivotLang(lang); l == msg.FR || l == msg.DE || l == msg.ES {
		sep = ","
	}
	shown := func(f float64) string { return strings.Replace(num(f), ".", sep, 1) }
	whole := integer && s0 == math.Trunc(s0) && g.Step == math.Trunc(g.Step)
	var lo, hi string
	if whole {
		lo = "CAST(" + num(s0) + " + {k} * " + num(g.Step) + " AS INTEGER)"
		hi = "CAST(" + num(s0) + " + {k} * " + num(g.Step) + " + " + num(g.Step) + " - 1 AS INTEGER)"
	} else {
		dp := strconv.Itoa(max(decimals(s0), decimals(g.Step)))
		lo = "replace(printf('%." + dp + "f', " + num(s0) + " + {k} * " + num(g.Step) + "), '.', " + sqlLiteral(sep) + ")"
		hi = "replace(printf('%." + dp + "f', " + num(s0) + " + ({k} + 1) * " + num(g.Step) + "), '.', " + sqlLiteral(sep) + ")"
	}
	label = "CASE WHEN {k} IS NULL THEN NULL"
	if g.Start != nil {
		label += " WHEN {k} = " + groupBelow + " THEN " + sqlLiteral("<"+shown(*g.Start))
	}
	if g.End != nil {
		label += " WHEN {k} = " + groupAbove + " THEN " + sqlLiteral(">"+shown(*g.End))
	}
	label += " ELSE " + lo + " || '-' || " + hi + " END"
	return key, label, nil
}

// decimals : chiffres après la virgule d'un nombre (6 au plus).
func decimals(f float64) int {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return min(len(s)-i-1, 6)
	}
	return 0
}

// periodSQL : la période (« 2024 », « 2024-T2 », « 2024-03 »,
// « 2024-03-15 ») d'une date ISO d, au niveau level (chronologie).
func periodSQL(level, d string) string {
	switch level {
	case "years":
		return "substr(" + d + ", 1, 4)"
	case "quarters":
		return "(substr(" + d + ", 1, 4) || '-T' || ((CAST(substr(" + d + ", 6, 2) AS INTEGER) + 2) / 3))"
	case "days":
		return "substr(" + d + ", 1, 10)"
	}
	return "substr(" + d + ", 1, 7)"
}
