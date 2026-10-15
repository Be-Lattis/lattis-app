package transfer

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/grid"
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
	taken, err := m.TableNames(ctx)
	if err != nil {
		return canvas.ImportResult{}, err
	}
	sheets, warnings, err := x.plan(ctx, taken)
	if err != nil {
		return canvas.ImportResult{}, err
	}
	res, err := m.Import(ctx, "Importer "+filepath.Base(path), sheets)
	res.Warnings = append(warnings, res.Warnings...)
	return res, err
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
	is    *island
	cols  []colState
	cells []freeCell
}

// plan lit le classeur (deux passages par feuille : îlots, puis en-têtes,
// types et formules) et prépare l'import ; les valeurs des tableaux sont
// relues au moment de l'import.
func (x *xlsxFile) plan(ctx context.Context, taken map[string]bool) ([]canvas.ImportSheet, []string, error) {
	rv := &resolver{bySheet: map[string][]*island{}}
	states := make([][]*islandState, len(x.sheets))
	for si, sh := range x.sheets {
		b := newIslands()
		err := x.scan(ctx, sh, func(c *rawCell) error {
			if !c.empty(x.sst) {
				b.add(c.row, c.col)
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
		for _, r := range b.rects() {
			is := &island{rect: r, sheet: sh.name}
			st := &islandState{is: is}
			if is.table() {
				is.name = tableName(sh.name, r.r0, r.c0, taken)
				is.headers = make([]string, r.c1-r.c0+1)
				st.cols = make([]colState, len(is.headers))
			}
			states[si] = append(states[si], st)
			rv.bySheet[strings.ToLower(sh.name)] = append(rv.bySheet[strings.ToLower(sh.name)], is)
		}
	}

	for si, sh := range x.sheets {
		shared := map[string]struct {
			ps  []piece
			err error
		}{}
		active := states[si]
		err := x.scan(ctx, sh, func(c *rawCell) error {
			if c.empty(x.sst) {
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
			if st == nil {
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
							warnings = append(warnings, fmt.Sprintf("Feuille « %s », cellule %s : %v ; valeur importée sans la formule", sh.name, cellName(fc.row, fc.col), err))
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
			}
			errs := 0
			for i, cs := range st.cols {
				t.Types[i] = final(cs.t)
				errs += cs.errs
				loc := fmt.Sprintf("Tableau %s, colonne « %s »", is.name, is.headers[i])
				switch {
				case cs.formulas == 0:
				case cs.why != nil:
					warnings = append(warnings, fmt.Sprintf("%s : %v ; valeurs importées sans la formule", loc, cs.why))
				case cs.mixed || cs.formulas < rows:
					warnings = append(warnings, fmt.Sprintf("%s : %v ; valeurs importées sans les formules", loc, errInconsistent))
				default:
					src, err := rv.rewrite(cs.first, where{sheet: sh.name, row: cs.firstRow, col: is.c0 + i, table: is})
					if err != nil {
						warnings = append(warnings, fmt.Sprintf("%s : %v ; valeurs importées sans la formule", loc, err))
					} else {
						t.Formulas[i] = src
					}
				}
			}
			if errs > 0 {
				warnings = append(warnings, fmt.Sprintf("Tableau %s : %d valeur(s) en erreur dans Excel (#N/A…) importée(s) vide(s)", is.name, errs))
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
