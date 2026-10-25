package transfer

import (
	"archive/zip"
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// ImportXLSX ajoute au classeur les feuilles d'un classeur Excel .xlsx.
//
// Chaque feuille est découpée en îlots (une ligne ou une colonne vide
// termine un tableau) : un îlot de deux lignes ou plus devient un tableau à
// la même place, sa première ligne donnant les en-têtes ; un îlot d'une
// ligne reste en cellules libres. Les types sont inférés sur toutes les
// valeurs ; les dates sont reconnues à leur format.
//
// Une colonne dont toutes les cellules portent la même formule recopiée
// devient une colonne calculée (voir xlsxformula.go). Sinon, et chaque fois
// qu'une formule ne peut pas être reprise, les valeurs calculées par Excel
// sont importées et un avertissement l'explique.
func ImportXLSX(ctx context.Context, m *canvas.Model, path string) (canvas.ImportResult, error) {
	x, err := openXLSX(path)
	if err != nil {
		return canvas.ImportResult{}, err
	}
	defer x.Close()
	x.lang = m.Language()
	taken, err := m.TableNames(ctx)
	if err != nil {
		return canvas.ImportResult{}, err
	}
	sheets, warnings, err := x.plan(ctx, taken)
	if err != nil {
		return canvas.ImportResult{}, err
	}
	x.pics = x.readDrawings(sheets)
	x.readCharts(sheets)
	visibleSheet(sheets)
	res, err := m.Import(ctx, "Importer "+filepath.Base(path), sheets)
	res.Warnings = slices.Concat(x.lost(), warnings, res.Warnings)
	return res, err
}

// lost dit ce que le classeur contient et que l'import ne reprend pas :
// graphiques, tableaux croisés (repris en valeurs), macros, commentaires,
// images, mise en forme conditionnelle et validations.
func (x *xlsxFile) lost() []string {
	var charts, pivots, notes, images int
	macros := false
	for _, f := range x.zr.File {
		name := strings.ToLower(f.Name)
		switch {
		case strings.HasPrefix(name, "xl/charts/chart") && strings.HasSuffix(name, ".xml"):
			charts++
		case strings.HasPrefix(name, "xl/pivottables/pivottable") && strings.HasSuffix(name, ".xml"):
			pivots++
		case name == "xl/vbaproject.bin":
			macros = true
		case strings.HasPrefix(name, "xl/media/"):
			images++
		case strings.HasPrefix(name, "xl/comments") && strings.HasSuffix(name, ".xml"):
			notes += countTags(f, "comment")
		}
	}
	// Repris : seuls les autres sont perdus.
	charts -= x.imp.charts
	pivots -= x.imp.pivots
	notes -= x.imp.notes
	var out []string
	say := func(id msg.ID, args msg.Args) { out = append(out, msg.Format(x.lang, id, args)) }
	if charts > 0 {
		say("import.xlsx.charts", msg.Args{"n": charts})
	}
	if pivots > 0 {
		say("import.xlsx.pivots", msg.Args{"n": pivots})
	}
	if macros {
		say("import.xlsx.macros", nil)
	}
	if notes > 0 {
		say("import.xlsx.comments", msg.Args{"n": notes})
	}
	// Images reprises (parties de dessin) : seules les autres sont perdues.
	if images -= x.pics; images > 0 {
		say("import.xlsx.images", msg.Args{"n": images})
	}
	for _, sh := range x.sheets {
		if n := x.imp.rules[sh.name]; n > 0 {
			say("import.xlsx.rules", msg.Args{"sheet": sh.name, "n": n})
		}
		if n := x.imp.lists[sh.name]; n > 0 {
			say("import.xlsx.validations", msg.Args{"sheet": sh.name, "n": n})
		}
	}

	return out
}

// countTags compte les balises name d'une partie XML (0 si illisible).
func countTags(f *zip.File, name string) int {
	r, err := f.Open()
	if err != nil {
		return 0
	}
	defer r.Close()
	d := newXMLScanner(r)
	n := 0
	for d.next() == nil {
		if d.kind == xmlStart && string(d.name) == name {
			n++
		}
	}
	return n
}

type colState struct {
	t        grid.Type
	formulas int
	key      string
	first    []piece
	firstRow int
	mixed    bool
	why      error // formule illisible
	errs     int   // valeurs en erreur (#N/A…)
}

type freeCell struct {
	row, col int
	v        value
	ps       []piece
	perr     error
	formula  bool
}

type islandState struct {
	tb    *tableBuild // mise en forme (tableau)
	is    *island

	cols  []colState
	cells []freeCell
}

// plan lit le classeur (deux passages par feuille : îlots, puis en-têtes,
// types et formules) et prépare l'import ; les valeurs des tableaux sont
// relues au moment de l'import.
func (x *xlsxFile) plan(ctx context.Context, taken map[string]bool) ([]canvas.ImportSheet, []string, error) {
	rv := &resolver{bySheet: map[string][]*island{}}
	x.rv = rv
	states := make([][]*islandState, len(x.sheets))
	builds := make([]*sheetBuild, len(x.sheets))
	pass1 := func(si int) error {
		sh := x.sheets[si]
		states[si] = nil
		delete(rv.bySheet, strings.ToLower(sh.name))
		b := newIslands()
		err := x.scan(ctx, sh, func(c *rawCell) error {
			if !c.empty(x.sst) && !sh.skipped(c.row, c.col) {
				b.add(c.row, c.col)
			}
			return nil
		})
		if err != nil {
			return err
		}
		rects := b.rects()
		maxCol := 0
		for _, r := range rects {
			maxCol = max(maxCol, r.c1)
		}
		sb := newSheetBuild(x, x.sheetFormat(sh, maxCol+10))
		builds[si] = sb
		xnames := x.excelTables(sh)
		for _, r := range rects {
			is := &island{rect: r, sheet: sh.name}
			st := &islandState{is: is}
			if is.table() {
				// Un tableau d'Excel à la même place garde son nom.
				if n := identifier(xnames[r]); n != "" && !taken[n] && !canvas.A1Like(n) {
					taken[n] = true
					is.name = n
				} else {
					is.name = tableName(sh.name, r.r0, r.c0, taken)
				}
				is.headers = make([]string, r.c1-r.c0+1)
				st.cols = make([]colState, len(is.headers))
				st.tb = sb.addTable(is)
			}
			states[si] = append(states[si], st)
			rv.bySheet[strings.ToLower(sh.name)] = append(rv.bySheet[strings.ToLower(sh.name)], is)
		}
		return nil
	}
	for si := range x.sheets {
		if err := pass1(si); err != nil {
			return nil, nil, err
		}
	}
	// Tableaux croisés dont la source est un tableau du classeur : leur zone
	// n'est pas lue comme des cellules (xlsxpivot.go) ; leurs feuilles sont
	// relues.
	for _, si := range x.placePivots() {
		if err := pass1(si); err != nil {
			return nil, nil, err
		}
	}


	for si, sh := range x.sheets {
		shared := map[string]struct {
			ps  []piece
			err error
		}{}
		active := states[si]
		sb := builds[si]
		err := x.scan(ctx, sh, func(c *rawCell) error {
			if sh.skipped(c.row, c.col) {
				return nil
			}
			var st *islandState
			for len(active) > 0 && active[0].is.r1 < c.row {
				active = active[1:] // îlots terminés (triés par première ligne)
			}
			for _, s := range active {
				if s.is.r0 > c.row {
					break
				}
				if s.is.contains(c.row, c.col) {
					st = s
					break
				}
			}
			if x.st != nil {
				var tb *tableBuild
				if st != nil {
					tb = st.tb
				}
				sb.cell(tb, c.row, c.col, c.s)
			}
			if st == nil || c.empty(x.sst) {
				return nil
			}

			var ps []piece
			var perr error
			if c.hasF {
				switch {
				case c.ft == "shared" && c.f == "":
					m, ok := shared[c.si]
					ps, perr = m.ps, m.err
					if !ok {
						perr = unsupported("formule partagée sans modèle")
					}
				default:
					ps, perr = splitFormula(c.f, c.row, c.col)
					if c.ft == "shared" {
						shared[c.si] = struct {
							ps  []piece
							err error
						}{ps, perr}
					}
				}
			}
			v, isErr := x.value(c)
			is := st.is
			if !is.table() {
				if isErr {
					v.t = grid.Text
				}
				st.cells = append(st.cells, freeCell{row: c.row, col: c.col, v: v, ps: ps, perr: perr, formula: c.hasF})
				return nil
			}
			i := c.col - is.c0
			if c.row == is.r0 {
				is.headers[i] = display(v)
				return nil
			}
			cs := &st.cols[i]
			if isErr {
				cs.errs++
			} else {
				cs.t = join(cs.t, v.t)
			}
			if c.hasF {
				cs.formulas++
				switch {
				case perr != nil:
					cs.mixed, cs.why = true, perr
				case cs.formulas == 1:
					cs.key, cs.first, cs.firstRow = key(ps), ps, c.row
				case key(ps) != cs.key:
					cs.mixed = true
				}
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}

	// Tous les en-têtes sont connus : les formules peuvent être résolues,
	// y compris vers une autre feuille.
	for _, sts := range states {
		for _, st := range sts {
			if st.is.table() {
				st.is.headers = columnNames(st.is.headers, len(st.is.headers))
			}
		}
	}
	var warnings []string
	out := make([]canvas.ImportSheet, len(x.sheets))
	for si, sh := range x.sheets {
		out[si].Name = sh.name
		if sh.page != nil && !sh.page.IsZero() && sh.page.Check() == nil {
			out[si].Page = sh.page
		}
		out[si].Links = x.sheetLinks(sh)
		warnings = append(warnings, x.decorate(sh, builds[si], rv)...)
		out[si].Format = builds[si].f
		out[si].Pivots = x.sheetPivots(si)
		for _, st := range states[si] {
			is := st.is
			if !is.table() {
				for _, fc := range st.cells {
					cell := canvas.ImportCell{Row: fc.row, Col: fc.col, Raw: display(fc.v)}
					if fc.formula {
						f, err := fc.perr, error(nil)
						if f == nil {
							var src string
							if src, err = rv.rewrite(fc.ps, where{sheet: sh.name, row: fc.row, col: fc.col}); err == nil {
								cell.Raw, cell.Fallback = src, display(fc.v)
							}
						} else {
							err = f
						}
						if err != nil {
							warnings = append(warnings, msg.Format(x.lang, "import.xlsx.cellFormula", msg.Args{"sheet": sh.name, "cell": cellName(fc.row, fc.col), "err": err}))
						}
					}
					out[si].Cells = append(out[si].Cells, cell)
				}
				continue
			}
			rows := is.r1 - is.r0
			t := canvas.ImportTable{
				Name:     is.name,
				Row:      is.r0,
				Col:      is.c0,
				Headers:  is.headers,
				Types:    make([]grid.Type, len(is.headers)),
				Formulas: make([]string, len(is.headers)),
				Count:    rows,
				Format:   st.tb.tf,
			}
			errs := 0
			for i, cs := range st.cols {
				t.Types[i] = final(cs.t)
				errs += cs.errs
				loc := msg.Args{"table": is.name, "column": is.headers[i]}
				switch {
				case cs.formulas == 0:
				case cs.why != nil:
					loc["err"] = cs.why
					warnings = append(warnings, msg.Format(x.lang, "import.xlsx.columnFormula", loc))
				case cs.mixed || cs.formulas < rows:
					warnings = append(warnings, msg.Format(x.lang, "import.xlsx.columnFormulas", loc))
				default:
					src, err := rv.rewrite(cs.first, where{sheet: sh.name, row: cs.firstRow, col: is.c0 + i, table: is})
					if err != nil {
						loc["err"] = err
						warnings = append(warnings, msg.Format(x.lang, "import.xlsx.columnFormula", loc))
					} else {
						t.Formulas[i] = src
					}
				}
			}
			if errs > 0 {
				warnings = append(warnings, msg.Format(x.lang, "import.xlsx.errorValues", msg.Args{"table": is.name, "n": errs, "na": "#N/A"}))
			}
			t.Rows = x.rows(ctx, sh, is, t.Types)
			out[si].Tables = append(out[si].Tables, t)
		}
	}
	return out, warnings, nil
}

var errStop = errors.New("fin de l'îlot")

// rows relit une feuille et produit les lignes de données d'un tableau.
func (x *xlsxFile) rows(ctx context.Context, sh xlsxSheet, is *island, types []grid.Type) func(func([]any) error) error {
	return func(yield func([]any) error) error {
		w := is.c1 - is.c0 + 1
		buf, blank := make([]any, w), make([]any, w)
		next, cur := is.r0+1, -1
		flushTo := func(r int) error {
			for ; next < r && next <= is.r1; next++ {
				row := blank
				if next == cur {
					row = buf
				}
				if err := yield(row); err != nil {
					return err
				}
			}
			return nil
		}
		err := x.scan(ctx, sh, func(c *rawCell) error {
			switch {
			case c.row < next:
				return nil
			case c.row > is.r1:
				return errStop
			case c.col < is.c0 || c.col > is.c1:
				return nil
			}
			if c.row != cur {
				if err := flushTo(c.row); err != nil {
					return err
				}
				clear(buf)
				cur = c.row
			}
			if v, isErr := x.value(c); !isErr {
				i := c.col - is.c0
				buf[i] = castTo(v, types[i])
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStop) {
			return err
		}
		return flushTo(is.r1 + 1)
	}
}
