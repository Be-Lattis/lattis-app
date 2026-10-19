package canvas

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Cellules affichées sans être saisies : un tableau croisé (en-tête,
// étiquettes, valeurs, totaux, filtres de rapport au-dessus) et le
// déversement d'un tableau dynamique occupent des cellules de la feuille.
// Une formule qui les cite (Cell[C5], SOMME(Cell[C3:C10]),
// Feuille!Cell[C5]), une requête SQL ou une cellule nommée lit la valeur
// affichée, comme Excel : elles entrent dans la table des noms
// (nameValuesWith), sous leur clé formula.CellKey et leur alias
// « feuille.cell_a1 ».

// pivotArea est la place d'un tableau croisé calculé sur sa feuille.
type pivotArea struct {
	sheet     int64
	sheetName string // en minuscules
	row, col  int    // coin haut gauche : premier filtre de rapport, sinon l'en-tête
	filters   []PivotFilter
	top       int        // ligne de l'en-tête
	cells     [][]string // en-tête, lignes, total (tels qu'affichés)
}

// value donne la valeur affichée en (r, c), ok faux hors du TCD. Filtres de
// rapport (champ, valeur retenue), en-tête et étiquettes des lignes
// (première colonne, à plat comme en arborescence) sont du texte ; les
// valeurs et totaux des nombres (« 12,5 % » vaut 0,125).
func (a pivotArea) value(r, c int) (formula.Value, bool) {
	if k := r - a.row; k >= 0 && k < len(a.filters) {
		switch c - a.col {
		case 0:
			return formula.Str(a.filters[k].Field), true
		case 1:
			return formula.Str(pivotFilterText(a.filters[k].Values)), true
		}
		return formula.Value{}, false
	}
	i, j := r-a.top, c-a.col
	if i < 0 || i >= len(a.cells) || j < 0 || j >= len(a.cells[i]) {
		return formula.Value{}, false
	}
	s := a.cells[i][j]
	switch {
	case s == "":
		return formula.Value{}, true
	case i == 0 || j == 0:
		return formula.Str(s), true
	}
	return pivotNumber(s), true
}

// pivotAreas donne les tableaux croisés calculés de toutes les feuilles, une
// fois par génération (chaque calcul est lui-même en cache tant que sa
// source ne change pas). Un TCD ne lit que les colonnes saisies de son
// tableau source (une colonne calculée ne sert pas de champ) : il ne dépend
// d'aucune formule et ne peut pas former de référence circulaire.
func (t *tx) pivotAreas() ([]pivotArea, error) {
	return cached(t, "pivotareas", t.pivotAreasRaw)
}

func (t *tx) pivotAreasRaw() ([]pivotArea, error) {
	type placed struct {
		id  int64
		a   pivotArea
		cfg PivotConfig
	}
	rs, err := t.query(`SELECT p.id, p.sheet, s.name, p.row, p.col, p.config FROM _lattis_pivots p
		JOIN _lattis_sheets s ON s.id = p.sheet ORDER BY p.id`)
	if err != nil {
		return nil, err
	}
	var list []placed
	for rs.Next() {
		var p placed
		var raw string
		if err := rs.Scan(&p.id, &p.a.sheet, &p.a.sheetName, &p.a.row, &p.a.col, &raw); err != nil {
			rs.Close()
			return nil, err
		}
		if json.Unmarshal([]byte(raw), &p.cfg) != nil {
			continue // configuration illisible : rien d'affiché
		}
		// Même configuration que l'affichage (pivots) : même cache.
		p.cfg = p.cfg.normalized()
		if p.cfg.Name == "" {
			p.cfg.Name = fmt.Sprintf("TCD%d", p.id)
		}
		p.a.sheetName = strings.ToLower(p.a.sheetName)
		list = append(list, p)
	}
	rs.Close()
	for i := range list {
		if err := t.pivotChoice(list[i].a.sheet, &list[i].cfg); err != nil {
			return nil, err
		}
	}
	var out []pivotArea
	for _, p := range list {
		cells, _, err := t.pivotCells(p.cfg)
		if err != nil {
			continue // TCD en erreur : seul le message s'affiche
		}
		a := p.a
		a.cells, a.filters, a.top = cells, p.cfg.Filters, a.row
		if len(a.filters) > 0 {
			a.top += len(a.filters) + 1 // une ligne vide sous les filtres
		}
		out = append(out, a)
	}
	return out, nil
}

// pivotValueAt donne la valeur affichée par un TCD en (row, col) de la
// feuille, ok faux si aucun ne la recouvre.
func pivotValueAt(areas []pivotArea, sheet int64, row, col int) (formula.Value, bool) {
	for _, a := range areas {
		if a.sheet == sheet {
			if v, ok := a.value(row, col); ok {
				return v, true
			}
		}
	}
	return formula.Value{}, false
}

// pivotValues range dans values la valeur des cellules de TCD que citent
// les formules (clés et alias « feuille.cell_a1 ») ; une cellule saisie à la
// même place, calculée ensuite, l'emporte.
func pivotValues(values map[string]formula.Value, ns *formula.Names, schema *formula.Schema, areas []pivotArea) {
	ids := map[string]int64{}
	for _, a := range areas {
		ids[a.sheetName] = a.sheet
	}
	put := func(d string) {
		if _, ok := ns.Formulas[d]; ok {
			return
		}
		if _, done := values[d]; done {
			return
		}
		sheet, r, c, ok := cellOfKey(d, ids)
		if !ok {
			return
		}
		if v, ok := pivotValueAt(areas, sheet, r, c); ok && !v.IsEmpty() {
			values[d] = v
		}
	}
	for _, n := range ns.Formulas {
		deps, _ := formula.Deps(n)
		for _, d := range deps {
			put(d)
		}
	}
	// Colonnes calculées des tableaux.
	for _, ft := range schema.Tables {
		for _, c := range ft.Cols {
			if c.Formula != nil {
				deps, _ := formula.Deps(formula.QualifyCells(c.Formula, ft.Sheet))
				for _, d := range deps {
					put(d)
				}
			}
		}
	}
}

// pivotFilterText est la valeur affichée d'un filtre de rapport (voir
// pivotFilterText dans Sheet.svelte).
func pivotFilterText(values []string) string {
	switch {
	case len(values) == 0:
		return "(Tous)"
	case len(values) == 1 && values[0] == "":
		return "(Vides)"
	case len(values) == 1:
		return values[0]
	}
	return "(Plusieurs éléments)"
}

// pivotNumber relit une valeur de TCD telle qu'affichée : nombre (« 1234,5 »),
// pourcentage (« 12,5 % » vaut 0,125), sinon texte.
func pivotNumber(s string) formula.Value {
	if f, ok := freePercent(s); ok {
		return formula.Num(f)
	}
	if v, err := grid.Parse(grid.Decimal, s); err == nil {
		if f, ok := v.(float64); ok {
			return formula.Num(f)
		}
	}
	return formula.Str(s)
}

// spillAnchor est une cellule libre dont la formule est un tableau dynamique.
type spillAnchor struct {
	key       string
	node      formula.Node // formule rattachée à sa feuille
	sheet     int64
	sheetName string // en minuscules
	row, col  int
	filled    map[[2]int]bool // cellules saisies de sa feuille
}

// spillState garde, le temps d'un calcul des noms, les tableaux de chaque
// feuille (un déversement ne les recouvre pas).
type spillState struct {
	tables map[int64][]Table
}

// spillDeps fait calculer après chaque tableau dynamique les formules qui
// lisent une cellule vide qu'il peut recouvrir (à sa droite et en dessous) :
// son étendue n'est connue qu'une fois calculé. Les dépendances ajoutées
// sont rendues par nom.
func spillDeps(ns *formula.Names, spills map[string]spillAnchor) map[string][]string {
	bySheet := map[int64][]spillAnchor{}
	ids := map[string]int64{}
	for _, a := range spills {
		bySheet[a.sheet] = append(bySheet[a.sheet], a)
		ids[a.sheetName] = a.sheet
	}
	added := map[string][]string{}
	for key, n := range ns.Formulas {
		deps, _ := formula.Deps(n)
		var seen map[string]bool
		for _, d := range append(deps, ns.Extra[key]...) {
			if _, ok := ns.Formulas[d]; ok {
				continue // cellule saisie, nom : déjà ordonné
			}
			sheet, r, c, ok := cellOfKey(d, ids)
			if !ok {
				continue
			}
			for _, a := range bySheet[sheet] {
				if a.key == key || a.row > r || a.col > c || seen[a.key] {
					continue
				}
				if seen == nil {
					seen = map[string]bool{}
				}
				seen[a.key] = true
				added[key] = append(added[key], a.key)
			}
		}
	}
	for key, extra := range added {
		ns.Extra[key] = append(slices.Clip(ns.Extra[key]), extra...)
	}
	return added
}

// cellOfKey lit une clé de cellule : « cell_a1#3 » (feuille 3) ou l'alias
// « feuille.cell_a1 » (feuilles connues de ids seulement).
func cellOfKey(key string, ids map[string]int64) (sheet int64, row, col int, ok bool) {
	addr := key
	if k := strings.LastIndex(key, "#"); k > 0 {
		var id int64
		if _, err := fmt.Sscan(key[k+1:], &id); err != nil {
			return 0, 0, 0, false
		}
		sheet, addr = id, key[:k]
	} else if k := strings.LastIndex(key, "."); k > 0 {
		id, known := ids[key[:k]]
		if !known {
			return 0, 0, 0, false
		}
		sheet, addr = id, key[k+1:]
	} else {
		return 0, 0, 0, false
	}
	row, col, ok = formula.CellRef(addr)
	return sheet, row, col, ok
}

// orderWithSpills ordonne les noms avec les dépendances des déversements
// (added) ; celles-ci, prudentes, peuvent former une fausse référence
// circulaire : les noms qui ne sont circulaires qu'à cause d'elles les
// perdent (base : dépendances sans les déversements).
func orderWithSpills(ns *formula.Names, base map[string][]string, added map[string][]string) (order, cyclic []string) {
	order, cyclic = ns.Order()
	if len(cyclic) == 0 || len(added) == 0 {
		return order, cyclic
	}
	withSpills := ns.Extra
	ns.Extra = base
	_, baseCyclic := ns.Order()
	ns.Extra = withSpills
	inBase := map[string]bool{}
	for _, n := range baseCyclic {
		inBase[n] = true
	}
	for _, n := range cyclic {
		if !inBase[n] && added[n] != nil {
			ns.Extra[n] = base[n]
		}
	}
	return ns.Order()
}

// spillValues calcule un tableau dynamique dans le calcul des noms : l'ancre
// vaut la première valeur et chaque cellule recouverte la sienne ; un
// déversement bloqué (cellule saisie ou tableau sur le chemin) ou vide ne
// recouvre rien et l'ancre vaut une erreur.
func (t *tx) spillValues(values map[string]formula.Value, a spillAnchor, schema *formula.Schema, st *spillState) {
	rows, e := t.spillRows(a.node, schema, values)
	if e.IsErr() {
		values[a.key] = e
		return
	}
	if len(rows) == 0 || len(rows[0]) == 0 {
		values[a.key] = formula.Err(formula.ErrValue)
		return
	}
	r1, c1 := a.row+len(rows)-1, a.col+len(rows[0])-1
	for p := range a.filled {
		if p != [2]int{a.row, a.col} && p[0] >= a.row && p[0] <= r1 && p[1] >= a.col && p[1] <= c1 {
			values[a.key] = formula.Err(formula.ErrValue)
			return
		}
	}
	tables, ok := st.tables[a.sheet]
	if !ok {
		tables, _ = t.tables(a.sheet)
		st.tables[a.sheet] = tables
	}
	for _, tb := range tables {
		if tb.Row <= r1 && tb.Bottom() >= a.row && tb.Col <= c1 && tb.Right() >= a.col {
			values[a.key] = formula.Err(formula.ErrValue)
			return
		}
	}
	for i, row := range rows {
		for j, v := range row {
			values[formula.CellKey(a.sheet, a.row+i, a.col+j)] = v
			values[a.sheetName+"."+strings.ToLower(formula.CellName(a.row+i, a.col+j))] = v
		}
	}
}
