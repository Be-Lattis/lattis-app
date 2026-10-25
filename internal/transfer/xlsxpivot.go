package transfer

import (
	"path"
	"regexp"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Tableaux croisés d'un fichier .xlsx : quand la source de leur cache est
// une plage ou un tableau du classeur qui devient un tableau de Lattis, ils
// deviennent des tableaux croisés de Lattis à la même place (champs des
// lignes, premier champ des colonnes, valeurs en somme, nombre, moyenne,
// minimum ou maximum, éventuellement en pourcentage, filtres de rapport) ;
// leurs cellules ne sont alors pas lues comme des valeurs. Sinon ils
// restent importés en valeurs. À l'export, un tableau croisé s'écrit en
// valeurs, avec sa mise en forme.

type xPivotData struct {
	fld  int
	agg  string
	name string
	show string
}

type xPivot struct {
	sheet    int // feuille du tableau croisé
	loc      rect
	pages    int // filtres de rapport (lignes au-dessus)
	srcSheet string
	srcRef   rect
	srcName  string
	fields   []string   // champs du cache
	items    [][]string // valeurs partagées de chaque champ
	fldItems [][]int    // éléments de chaque champ du tableau (rang dans items)
	rows     []int
	cols     []int
	data     []xPivotData
	filters  [][2]int // champ, élément (-1 : tous)
	table    *island
	ok       bool
}

var pivotAggs = map[string]string{"": "sum", "sum": "sum", "count": "count", "countNums": "count", "average": "avg", "min": "min", "max": "max"}

var pivotShows = map[string]string{"": "", "normal": "", "percentOfTotal": "pctTotal", "percentOfRow": "pctRow", "percentOfCol": "pctCol"}

// readPivots lit les définitions des tableaux croisés des feuilles.
func (x *xlsxFile) readPivots() []*xPivot {
	var out []*xPivot
	for si, sh := range x.sheets {
		if sh.file == nil {
			continue
		}
		byID, _, err := x.rels(sh.file.Name)
		if err != nil {
			continue
		}
		for _, p := range byID {
			if !strings.Contains(strings.ToLower(p), "pivottables/") {
				continue
			}
			if pv := x.readPivot(p); pv != nil {
				pv.sheet = si
				out = append(out, pv)
			}
		}
	}
	return out
}

func (x *xlsxFile) readPivot(p string) *xPivot {
	f := x.find(p)
	if f == nil {
		return nil
	}
	var def struct {
		Loc struct {
			Ref   string `xml:"ref,attr"`
			Pages int    `xml:"rowPageCount,attr"`
		} `xml:"location"`
		Fields []struct {
			Items []struct {
				X *int   `xml:"x,attr"`
				T string `xml:"t,attr"`
			} `xml:"items>item"`
		} `xml:"pivotFields>pivotField"`
		Rows []struct {
			X int `xml:"x,attr"`
		} `xml:"rowFields>field"`
		Cols []struct {
			X int `xml:"x,attr"`
		} `xml:"colFields>field"`
		Pages []struct {
			Fld  int  `xml:"fld,attr"`
			Item *int `xml:"item,attr"`
		} `xml:"pageFields>pageField"`
		Data []struct {
			Fld  int    `xml:"fld,attr"`
			Agg  string `xml:"subtotal,attr"`
			Name string `xml:"name,attr"`
			Show string `xml:"showDataAs,attr"`
		} `xml:"dataFields>dataField"`
	}
	if x.decode(f, &def) != nil {
		return nil
	}
	loc, ok := rangeRect(def.Loc.Ref)
	if !ok {
		return nil
	}
	pv := &xPivot{loc: loc, pages: len(def.Pages)}
	// Les champs sont donnés par leur rang (-2 : les valeurs).
	for _, r := range def.Rows {
		pv.rows = append(pv.rows, r.X)
	}
	for _, c := range def.Cols {
		pv.cols = append(pv.cols, c.X)
	}
	for _, d := range def.Data {
		pv.data = append(pv.data, xPivotData{fld: d.Fld, agg: d.Agg, name: d.Name, show: d.Show})
	}
	for _, pg := range def.Pages {
		it := -1
		if pg.Item != nil {
			it = *pg.Item
		}
		pv.filters = append(pv.filters, [2]int{pg.Fld, it})
	}
	for _, fd := range def.Fields {
		var xs []int
		for _, it := range fd.Items {
			if it.T == "" && it.X != nil {
				xs = append(xs, *it.X)
			}
		}
		pv.fldItems = append(pv.fldItems, xs)
	}
	// Cache : source et champs.
	byID, byType, _ := x.rels(p)
	cp := byType["pivotCacheDefinition"]
	if cp == "" {
		for _, t := range byID {
			if strings.Contains(strings.ToLower(path.Base(t)), "pivotcachedefinition") {
				cp = t
			}
		}
	}
	cf := x.find(cp)
	if cf == nil {
		return pv
	}
	var cache struct {
		Src struct {
			Type string `xml:"type,attr"`
			WS   *struct {
				Ref   string `xml:"ref,attr"`
				Sheet string `xml:"sheet,attr"`
				Name  string `xml:"name,attr"`
			} `xml:"worksheetSource"`
		} `xml:"cacheSource"`
		Fields []struct {
			Name  string `xml:"name,attr"`
			Items struct {
				Inner []struct {
					V string `xml:"v,attr"`
				} `xml:",any"`
			} `xml:"sharedItems"`
		} `xml:"cacheFields>cacheField"`
	}
	if x.decode(cf, &cache) != nil || cache.Src.WS == nil {
		return pv
	}
	ws := cache.Src.WS
	pv.srcSheet, pv.srcName = ws.Sheet, ws.Name
	if ws.Ref != "" {
		if r, ok := rangeRect(ws.Ref); ok {
			pv.srcRef = r
		}
	}
	for _, c := range cache.Fields {
		pv.fields = append(pv.fields, c.Name)
		var vals []string
		for _, it := range c.Items.Inner {
			vals = append(vals, it.V)
		}
		pv.items = append(pv.items, vals)
	}
	return pv
}

// sourceIsland trouve le tableau importé d'où le cache tire ses lignes.
func (x *xlsxFile) sourceIsland(pv *xPivot) *island {
	sheet, ref := pv.srcSheet, pv.srcRef
	if pv.srcName != "" {
		// Un tableau d'Excel (son nom) ou un nom défini.
		for _, is := range x.allIslands() {
			if strings.EqualFold(is.name, identifier(pv.srcName)) {
				return is
			}
		}
		v, ok := x.names[strings.ToLower(pv.srcName)]
		if !ok {
			return nil
		}
		v = strings.ReplaceAll(strings.TrimPrefix(v, "="), "$", "")
		k := strings.LastIndex(v, "!")
		if k < 0 {
			return nil
		}
		r, ok := rangeRect(v[k+1:])
		if !ok {
			return nil
		}
		sheet, ref = v[:k], r
	}
	if sheet == "" || len(pv.fields) == 0 {
		return nil
	}
	for _, is := range x.rv.bySheet[sheetKey(sheet)] {
		if is.table() && is.r0 == ref.r0 && is.c0 <= ref.c0 && is.c1 >= ref.c0+len(pv.fields)-1 {
			return is
		}
	}
	return nil
}

func (x *xlsxFile) allIslands() []*island {
	var out []*island
	for _, l := range x.rv.bySheet {
		out = append(out, l...)
	}
	return out
}

// placePivots repère les tableaux croisés repris comme tels et met leur
// zone de côté ; rend les feuilles à relire.
func (x *xlsxFile) placePivots() []int {
	x.pivotDefs = x.readPivots()
	seen := map[int]bool{}
	var redo []int
	for _, pv := range x.pivotDefs {
		if x.sourceIsland(pv) == nil || len(pv.data) == 0 && len(pv.rows) == 0 {
			continue
		}
		pv.ok = true
		for _, d := range pv.data {
			if _, ok := pivotAggs[d.agg]; !ok {
				pv.ok = false // produit, écart type… : en valeurs
			}
			if _, ok := pivotShows[d.show]; !ok {
				pv.ok = false
			}
		}
		if !pv.ok {
			continue
		}
		z := pv.loc
		if pv.pages > 0 {
			z.r0 = max(0, z.r0-pv.pages-1)
			z.c1 = max(z.c1, z.c0+1)
		}
		x.sheets[pv.sheet].skip = append(x.sheets[pv.sheet].skip, z)
		if !seen[pv.sheet] {
			seen[pv.sheet] = true
			redo = append(redo, pv.sheet)
		}
	}
	return redo
}

var defaultCaption = regexp.MustCompile(`(?i)^(sum|count|average|max|min|somme|nombre|moyenne|nb) (of|de|d') ?`)

// sheetPivots donne les tableaux croisés d'une feuille, une fois les en-têtes
// des tableaux connus.
func (x *xlsxFile) sheetPivots(si int) []canvas.ImportPivot {
	var out []canvas.ImportPivot
	for _, pv := range x.pivotDefs {
		if pv.sheet != si || !pv.ok {
			continue
		}
		is := x.sourceIsland(pv)
		if is == nil {
			continue
		}
		c0 := pv.srcRef.c0
		if pv.srcName != "" {
			c0 = is.c0
		}
		field := func(i int) string {
			if j := c0 + i - is.c0; i >= 0 && j >= 0 && j < len(is.headers) {
				return is.headers[j]
			}
			return ""
		}
		var cfg canvas.PivotConfig
		cfg.Source = is.name
		for _, r := range pv.rows {
			if f := field(r); f != "" {
				cfg.RowFields = append(cfg.RowFields, f)
			}
		}
		for _, c := range pv.cols {
			if f := field(c); f != "" && cfg.Cols == "" {
				cfg.Cols = f
			}
		}
		for _, d := range pv.data {
			v := canvas.PivotValue{Field: field(d.fld), Agg: pivotAggs[d.agg], Show: pivotShows[d.show]}
			if d.name != "" && !defaultCaption.MatchString(d.name) {
				v.Label = d.name
			}
			cfg.Values = append(cfg.Values, v)
		}
		for _, fl := range pv.filters {
			f := canvas.PivotFilter{Field: field(fl[0])}
			if f.Field == "" {
				continue
			}
			if it := fl[1]; it >= 0 && fl[0] < len(pv.fldItems) && it < len(pv.fldItems[fl[0]]) {
				if k := pv.fldItems[fl[0]][it]; fl[0] < len(pv.items) && k >= 0 && k < len(pv.items[fl[0]]) {
					f.Values = []string{display(parseText(pv.items[fl[0]][k]))}
				}
			}
			cfg.Filters = append(cfg.Filters, f)
		}
		row := pv.loc.r0
		if len(cfg.Filters) > 0 {
			row = max(0, row-len(cfg.Filters)-1)
		}
		out = append(out, canvas.ImportPivot{Row: row, Col: pv.loc.c0, Config: cfg})
		x.imp.pivots++
	}
	return out
}
