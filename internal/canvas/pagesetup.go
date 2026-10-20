package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lalamefine/lattis/internal/msg"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Mise en page d'une feuille (onglet Mise en page, comme Excel) : papier,
// orientation, marges, échelle ou ajustement en pages, zone d'impression,
// titres à répéter, sauts de page manuels, options d'impression, en-tête et
// pied de page. Une ligne JSON de _lattis_page_setup par feuille (aucune :
// réglages par défaut), qui part et revient avec la feuille
// (sheetPositioned) ; les lignes et colonnes suivent les insertions et
// suppressions (shiftPageSetup). L'impression (transfer.PrintHTML) et les
// exports .xlsx / .ods s'en servent.

// PageSetup est la mise en page d'une feuille. Lignes et colonnes à partir
// de 0 ; marges en centimètres.
type PageSetup struct {
	// Paper : A4 (défaut), A3, A5, B5, Letter, Legal, Tabloid, Executive.
	Paper     string   `json:"paper,omitempty"`
	Landscape bool     `json:"landscape,omitempty"`
	Margins   *Margins `json:"margins,omitempty"` // nil : Normales
	// Scale : échelle en pour cent (10 à 400, 0 : 100) ; ignorée quand
	// FitWide ou FitTall ajuste la feuille à un nombre de pages.
	Scale   int `json:"scale,omitempty"`
	FitWide int `json:"fitWide,omitempty"` // pages en largeur (0 : automatique)
	FitTall int `json:"fitTall,omitempty"` // pages en hauteur (0 : automatique)
	// Area : zone d'impression (nil : la plage utilisée).
	Area *Rect `json:"area,omitempty"`
	// TitleRows, TitleCols : lignes à répéter en haut, colonnes à répéter à
	// gauche de chaque page (premier et dernier indice).
	TitleRows *Span `json:"titleRows,omitempty"`
	TitleCols *Span `json:"titleCols,omitempty"`
	// RowBreaks, ColBreaks : sauts de page manuels, avant ces lignes ou colonnes.
	RowBreaks []int `json:"rowBreaks,omitempty"`
	ColBreaks []int `json:"colBreaks,omitempty"`
	// Gridlines, Headings : imprimer le quadrillage, les en-têtes de lignes
	// et de colonnes.
	Gridlines bool `json:"gridlines,omitempty"`
	Headings  bool `json:"headings,omitempty"`
	CenterH   bool `json:"centerH,omitempty"`
	CenterV   bool `json:"centerV,omitempty"`
	// AcrossFirst : ordre des pages « à droite, puis vers le bas » (défaut :
	// vers le bas, puis à droite).
	AcrossFirst bool `json:"acrossFirst,omitempty"`
	BlackWhite  bool `json:"blackWhite,omitempty"`
	// FirstPage : numéro de la première page (0 : automatique, 1).
	FirstPage int          `json:"firstPage,omitempty"`
	Header    HeaderFooter `json:"header"`
	Footer    HeaderFooter `json:"footer"`
}

// Margins sont les marges d'une page, en centimètres ; Header et Footer :
// distance de l'en-tête au haut de la page, du pied de page au bas.
type Margins struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
	Header float64 `json:"header"`
	Footer float64 `json:"footer"`
}

// Span est une suite de lignes ou de colonnes (bornes comprises).
type Span struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// HeaderFooter est un en-tête ou un pied de page en trois parties. Codes :
// &[Page], &[Pages], &[Date], &[Heure], &[Fichier], &[Chemin], &[Onglet]
// (ou &[Feuille]).
type HeaderFooter struct {
	Left   string `json:"left,omitempty"`
	Center string `json:"center,omitempty"`
	Right  string `json:"right,omitempty"`
}

// Empty dit si l'en-tête (ou le pied de page) est vide.
func (h HeaderFooter) Empty() bool { return h.Left == "" && h.Center == "" && h.Right == "" }

// Marges prédéfinies d'Excel (en pouces, converties en centimètres).
var (
	MarginsNormal = Margins{Top: 0.75 * 2.54, Bottom: 0.75 * 2.54, Left: 0.7 * 2.54, Right: 0.7 * 2.54, Header: 0.3 * 2.54, Footer: 0.3 * 2.54}
	MarginsWide   = Margins{Top: 2.54, Bottom: 2.54, Left: 2.54, Right: 2.54, Header: 1.27, Footer: 1.27}
	MarginsNarrow = Margins{Top: 0.75 * 2.54, Bottom: 0.75 * 2.54, Left: 0.25 * 2.54, Right: 0.25 * 2.54, Header: 0.3 * 2.54, Footer: 0.3 * 2.54}
)

// PaperSizes : formats de papier connus, en millimètres (portrait).
var PaperSizes = map[string][2]float64{
	"A3":        {297, 420},
	"A4":        {210, 297},
	"A5":        {148, 210},
	"B5":        {176, 250},
	"Letter":    {215.9, 279.4},
	"Legal":     {215.9, 355.6},
	"Tabloid":   {279.4, 431.8},
	"Executive": {184.15, 266.7},
}

// PaperMM donne la largeur et la hauteur de la page, orientation comprise.
func (p PageSetup) PaperMM() (w, h float64) {
	s, ok := PaperSizes[p.Paper]
	if !ok {
		s = PaperSizes["A4"]
	}
	if p.Landscape {
		return s[1], s[0]
	}
	return s[0], s[1]
}

// PageMargins donne les marges (Normales par défaut).
func (p PageSetup) PageMargins() Margins {
	if p.Margins == nil {
		return MarginsNormal
	}
	return *p.Margins
}

// Fit dit si la feuille est ajustée à un nombre de pages.
func (p PageSetup) Fit() bool { return p.FitWide > 0 || p.FitTall > 0 }

// IsZero dit si la mise en page est celle par défaut.
func (p PageSetup) IsZero() bool {
	b, _ := json.Marshal(p)
	z, _ := json.Marshal(PageSetup{})
	return string(b) == string(z)
}

// Check vérifie la mise en page et la met en forme normale (sauts triés,
// sans doublon, zone et titres dans l'ordre).
func (p *PageSetup) Check() error {
	if p.Paper != "" {
		if _, ok := PaperSizes[p.Paper]; !ok {
			return msg.New("pageSetup.unknownPaper", msg.Args{"paper": p.Paper})
		}
		if p.Paper == "A4" {
			p.Paper = ""
		}
	}
	if p.Scale == 100 {
		p.Scale = 0
	}
	if p.Scale != 0 && (p.Scale < 10 || p.Scale > 400) {
		return msg.New("pageSetup.scaleRange", nil)
	}
	if p.FitWide < 0 || p.FitTall < 0 || p.FitWide > 999 || p.FitTall > 999 {
		return fmt.Errorf("ajustement : entre 0 (automatique) et 999 pages")
	}
	if p.FirstPage < 0 {
		return msg.New("pageSetup.firstPagePositive", nil)
	}
	if m := p.Margins; m != nil {
		for _, v := range []float64{m.Top, m.Bottom, m.Left, m.Right, m.Header, m.Footer} {
			if v < 0 || v > 50 || math.IsNaN(v) {
				return fmt.Errorf("marges : entre 0 et 50 cm")
			}
		}
		w, h := p.PaperMM()
		if (m.Left+m.Right)*10 >= w-10 || (m.Top+m.Bottom)*10 >= h-10 {
			return msg.New("pageSetup.marginsTooLarge", nil)
		}
	}
	if a := p.Area; a != nil {
		a.R0, a.R1 = min(a.R0, a.R1), max(a.R0, a.R1)
		a.C0, a.C1 = min(a.C0, a.C1), max(a.C0, a.C1)
		if a.R0 < 0 || a.C0 < 0 {
			return msg.New("pageSetup.printAreaOutOfSheet", nil)
		}
	}
	for _, s := range []*Span{p.TitleRows, p.TitleCols} {
		if s != nil {
			s.From, s.To = min(s.From, s.To), max(s.From, s.To)
			if s.From < 0 {
				return msg.New("pageSetup.titlesOutOfSheet", nil)
			}
		}
	}
	norm := func(b []int) []int {
		b = slices.DeleteFunc(slices.Clone(b), func(i int) bool { return i <= 0 })
		slices.Sort(b)
		b = slices.Compact(b)
		if len(b) == 0 {
			return nil
		}
		return b
	}
	p.RowBreaks, p.ColBreaks = norm(p.RowBreaks), norm(p.ColBreaks)
	return nil
}

// PageSetup lit la mise en page d'une feuille.
func (m *Model) PageSetup(ctx context.Context, sheet int64) (PageSetup, error) {
	var p PageSetup
	err := m.read(ctx, func(t *tx) (err error) {
		p, err = t.pageSetup(sheet)
		return err
	})
	return p, err
}

func (t *tx) pageSetup(sheet int64) (PageSetup, error) {
	var p PageSetup
	var raw string
	err := t.row(`SELECT setup FROM _lattis_page_setup WHERE sheet = ?`, sheet).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	return p, json.Unmarshal([]byte(raw), &p)
}

// SetPageSetup remplace la mise en page d'une feuille (une étape
// d'annulation « Mise en page »).
func (m *Model) SetPageSetup(ctx context.Context, sheet int64, p PageSetup) error {
	return m.SetPageSetupAs(ctx, sheet, p, "Mise en page")
}

// SetPageSetupAs fait SetPageSetup sous un autre nom d'étape (« Zone
// d'impression », « Orientation »…).
func (m *Model) SetPageSetupAs(ctx context.Context, sheet int64, p PageSetup, label string) error {
	if err := p.Check(); err != nil {
		return fmt.Errorf("%w : %v", ErrSheet, err)
	}
	return m.Do(ctx, label, func(t *tx) error {
		// Permise sur une feuille protégée (sans effet sur son contenu), pas
		// quand sa structure l'est.
		if err := t.guardStructure(sheet); err != nil {
			return err
		}
		var n int
		if err := t.row(`SELECT count(*) FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return msg.Wrap(ErrSheet, "sheet.notFound", nil)
		}
		return t.setPageSetup(sheet, p)
	})
}

// setPageSetup écrit la mise en page (réversible) ; rien si elle ne change pas.
func (t *tx) setPageSetup(sheet int64, p PageSetup) error {
	old, err := t.snapshot(`_lattis_page_setup`, sheet)
	if err != nil {
		return err
	}
	do := []stmt{st(`DELETE FROM _lattis_page_setup WHERE sheet = ?`, sheet)}
	if !p.IsZero() {
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}
		if len(old) == 1 && len(old[0].Args) == 2 && old[0].Args[1] == string(b) {
			return nil
		}
		do = append(do, st(`INSERT INTO _lattis_page_setup (sheet, setup) VALUES (?, ?)`, sheet, string(b)))
	} else if len(old) == 0 {
		return nil
	}
	return t.apply(do, append([]stmt{st(`DELETE FROM _lattis_page_setup WHERE sheet = ?`, sheet)}, old...))
}

// shiftPageSetup fait suivre la zone d'impression, les titres et les sauts
// de page à une insertion ou une suppression de lignes (ou de colonnes) :
// move donne la nouvelle place d'un indice, ou false s'il est supprimé.
func (t *tx) shiftPageSetup(sheet int64, a axis, move func(int) (int, bool)) error {
	p, err := t.pageSetup(sheet)
	if err != nil || p.IsZero() {
		return err
	}
	// Une plage va de sa première à sa dernière ligne restante ;
	// supprimée en entier, elle disparaît.
	span := func(lo, hi int) (int, int, bool) {
		for ; lo <= hi; lo++ {
			if j, ok := move(lo); ok {
				for ; hi >= lo; hi-- {
					if k, ok := move(hi); ok {
						return j, k, true
					}
				}
			}
		}
		return 0, 0, false
	}
	if a == rowsAxis {
		if p.Area != nil {
			if lo, hi, ok := span(p.Area.R0, p.Area.R1); ok {
				p.Area.R0, p.Area.R1 = lo, hi
			} else {
				p.Area = nil
			}
		}
		if s := p.TitleRows; s != nil {
			if lo, hi, ok := span(s.From, s.To); ok {
				s.From, s.To = lo, hi
			} else {
				p.TitleRows = nil
			}
		}
		p.RowBreaks = moveBreaks(p.RowBreaks, move)
	} else {
		if p.Area != nil {
			if lo, hi, ok := span(p.Area.C0, p.Area.C1); ok {
				p.Area.C0, p.Area.C1 = lo, hi
			} else {
				p.Area = nil
			}
		}
		if s := p.TitleCols; s != nil {
			if lo, hi, ok := span(s.From, s.To); ok {
				s.From, s.To = lo, hi
			} else {
				p.TitleCols = nil
			}
		}
		p.ColBreaks = moveBreaks(p.ColBreaks, move)
	}
	if err := p.Check(); err != nil {
		return err
	}
	return t.setPageSetup(sheet, p)
}

func moveBreaks(b []int, move func(int) (int, bool)) []int {
	var out []int
	for _, i := range b {
		if j, ok := move(i); ok {
			out = append(out, j)
		}
	}
	return out
}

// --- Adresses A1 de la mise en page (API, import et export) ----------------

// ParseA1Range lit « A1:F20 » ($ permis) : lignes et colonnes à partir de 0.
func ParseA1Range(s string) (Rect, error) {
	s = strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(s)), "$", "")
	a, b, two := strings.Cut(s, ":")
	if !two {
		b = a
	}
	r0, c0, ok0 := parseA1Cell(a)
	r1, c1, ok1 := parseA1Cell(b)
	if !ok0 || !ok1 {
		return Rect{}, fmt.Errorf("plage %q : adresse A1 attendue (A1:F20)", s)
	}
	return Rect{R0: min(r0, r1), C0: min(c0, c1), R1: max(r0, r1), C1: max(c0, c1)}, nil
}

func parseA1Cell(s string) (row, col int, ok bool) {
	i := 0
	for i < len(s) && s[i] >= 'A' && s[i] <= 'Z' {
		col = col*26 + int(s[i]-'A'+1)
		i++
	}
	if i == 0 || i > 4 || i == len(s) {
		return 0, 0, false
	}
	n, err := strconv.Atoi(s[i:])
	if err != nil || n < 1 {
		return 0, 0, false
	}
	return n - 1, col - 1, true
}

// ParseLineSpan lit des lignes (« 1:2 », « 3 ») ou des colonnes (« A:B »,
// « C ») entières, à partir de 0.
func ParseLineSpan(s string, rows bool) (Span, error) {
	s = strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(s)), "$", "")
	a, b, two := strings.Cut(s, ":")
	if !two {
		b = a
	}
	one := func(x string) (int, bool) {
		if rows {
			n, err := strconv.Atoi(x)
			return n - 1, err == nil && n >= 1
		}
		c := 0
		for i := 0; i < len(x); i++ {
			if x[i] < 'A' || x[i] > 'Z' || i >= 4 {
				return 0, false
			}
			c = c*26 + int(x[i]-'A'+1)
		}
		return c - 1, x != ""
	}
	i, ok0 := one(a)
	j, ok1 := one(b)
	if !ok0 || !ok1 {
		if rows {
			return Span{}, msg.New("pageSetup.rowsFormat", msg.Args{"value": fmt.Sprintf("%q", s)})
		}
		return Span{}, msg.New("pageSetup.columnsFormat", msg.Args{"value": fmt.Sprintf("%q", s)})
	}
	return Span{From: min(i, j), To: max(i, j)}, nil
}

// ColLetters donne les lettres d'une colonne (0 → A).
func ColLetters(c int) string {
	s := ""
	for c++; c > 0; c = (c - 1) / 26 {
		s = string(rune('A'+(c-1)%26)) + s
	}
	return s
}

// A1 écrit une plage « A1:F20 ».
func (r Rect) A1() string {
	return ColLetters(r.C0) + strconv.Itoa(r.R0+1) + ":" + ColLetters(r.C1) + strconv.Itoa(r.R1+1)
}

// RowsA1 écrit des lignes entières « 1:2 », ColsA1 des colonnes « A:B ».
func (s Span) RowsA1() string { return strconv.Itoa(s.From+1) + ":" + strconv.Itoa(s.To+1) }
func (s Span) ColsA1() string { return ColLetters(s.From) + ":" + ColLetters(s.To) }
