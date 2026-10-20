package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
)

// PageSetupView est la mise en page d'une feuille (Affichage › Mise en page),
// lignes et colonnes à partir de 1, plages en adresses A1, marges en cm.
type PageSetupView struct {
	Paper       string         `json:"paper"`
	Orientation string         `json:"orientation"`
	Margins     canvas.Margins `json:"margins"`
	Scale       int            `json:"scale"`
	FitWide     int            `json:"fit_wide"`
	FitTall     int            `json:"fit_tall"`
	PrintArea   string         `json:"print_area"`
	TitleRows   string         `json:"title_rows"`
	TitleCols   string         `json:"title_cols"`
	RowBreaks   []int          `json:"row_breaks"`
	ColBreaks   []int          `json:"col_breaks"`
	Gridlines   bool           `json:"gridlines"`
	Headings    bool           `json:"headings"`
	CenterH     bool           `json:"center_h"`
	CenterV     bool           `json:"center_v"`
	Order       string         `json:"order"`
	BlackWhite  bool           `json:"black_white"`
	FirstPage   int            `json:"first_page"`
	Header      HeaderFooter   `json:"header"`
	Footer      HeaderFooter   `json:"footer"`
}

// HeaderFooter est un en-tête ou un pied de page en trois parties.
type HeaderFooter struct {
	Left   string `json:"left"`
	Center string `json:"center"`
	Right  string `json:"right"`
}

// pageSetupPatch : champs à changer (absents : inchangés).
type pageSetupPatch struct {
	Paper       *string          `json:"paper"`
	Orientation *string          `json:"orientation"`
	Margins     *json.RawMessage `json:"margins"`
	Scale       *int             `json:"scale"`
	FitWide     *int             `json:"fit_wide"`
	FitTall     *int             `json:"fit_tall"`
	PrintArea   *string          `json:"print_area"`
	TitleRows   *string          `json:"title_rows"`
	TitleCols   *string          `json:"title_cols"`
	RowBreaks   *[]int           `json:"row_breaks"`
	ColBreaks   *[]int           `json:"col_breaks"`
	Gridlines   *bool            `json:"gridlines"`
	Headings    *bool            `json:"headings"`
	CenterH     *bool            `json:"center_h"`
	CenterV     *bool            `json:"center_v"`
	Order       *string          `json:"order"`
	BlackWhite  *bool            `json:"black_white"`
	FirstPage   *int             `json:"first_page"`
	Header      *HeaderFooter    `json:"header"`
	Footer      *HeaderFooter    `json:"footer"`
}

func describePageSetup(p canvas.PageSetup) PageSetupView {
	v := PageSetupView{Paper: p.Paper, Orientation: "portrait", Margins: p.PageMargins(), Scale: p.Scale, FitWide: p.FitWide, FitTall: p.FitTall,
		RowBreaks: []int{}, ColBreaks: []int{}, Gridlines: p.Gridlines, Headings: p.Headings, CenterH: p.CenterH, CenterV: p.CenterV,
		Order: "down", BlackWhite: p.BlackWhite, FirstPage: p.FirstPage,
		Header: HeaderFooter(p.Header), Footer: HeaderFooter(p.Footer)}
	if v.Paper == "" {
		v.Paper = "A4"
	}
	if p.Landscape {
		v.Orientation = "landscape"
	}
	if v.Scale == 0 {
		v.Scale = 100
	}
	if p.AcrossFirst {
		v.Order = "across"
	}
	if p.Area != nil {
		v.PrintArea = p.Area.A1()
	}
	if p.TitleRows != nil {
		v.TitleRows = p.TitleRows.RowsA1()
	}
	if p.TitleCols != nil {
		v.TitleCols = p.TitleCols.ColsA1()
	}
	for _, r := range p.RowBreaks {
		v.RowBreaks = append(v.RowBreaks, r+1)
	}
	for _, c := range p.ColBreaks {
		v.ColBreaks = append(v.ColBreaks, c+1)
	}
	return v
}

// apply pose les champs donnés sur p.
func (x pageSetupPatch) apply(p *canvas.PageSetup) error {
	if x.Paper != nil {
		p.Paper = *x.Paper
		for k := range canvas.PaperSizes {
			if strings.EqualFold(k, p.Paper) {
				p.Paper = k
			}
		}
	}
	if x.Orientation != nil {
		switch *x.Orientation {
		case "portrait":
			p.Landscape = false
		case "landscape", "paysage":
			p.Landscape = true
		default:
			return msg.New("api.orientation", nil)
		}
	}
	if x.Margins != nil {
		var name string
		if json.Unmarshal(*x.Margins, &name) == nil {
			switch name {
			case "normal":
				p.Margins = nil
			case "wide":
				m := canvas.MarginsWide
				p.Margins = &m
			case "narrow":
				m := canvas.MarginsNarrow
				p.Margins = &m
			default:
				return msg.New("api.marginsExpected", nil)
			}
		} else {
			m := p.PageMargins()
			if err := json.Unmarshal(*x.Margins, &m); err != nil {
				return fmt.Errorf("margins : %v", err)
			}
			p.Margins = &m
		}
	}
	setInt := func(dst *int, v *int) {
		if v != nil {
			*dst = *v
		}
	}
	setInt(&p.Scale, x.Scale)
	setInt(&p.FitWide, x.FitWide)
	setInt(&p.FitTall, x.FitTall)
	setInt(&p.FirstPage, x.FirstPage)
	if x.PrintArea != nil {
		p.Area = nil
		if strings.TrimSpace(*x.PrintArea) != "" {
			r, err := canvas.ParseA1Range(*x.PrintArea)
			if err != nil {
				return fmt.Errorf("print_area : %v", err)
			}
			p.Area = &r
		}
	}
	for _, t := range []struct {
		v    *string
		dst  **canvas.Span
		rows bool
	}{{x.TitleRows, &p.TitleRows, true}, {x.TitleCols, &p.TitleCols, false}} {
		if t.v == nil {
			continue
		}
		*t.dst = nil
		if strings.TrimSpace(*t.v) != "" {
			s, err := canvas.ParseLineSpan(*t.v, t.rows)
			if err != nil {
				return err
			}
			*t.dst = &s
		}
	}
	breaks := func(v *[]int) []int {
		var out []int
		for _, i := range *v {
			out = append(out, i-1)
		}
		return out
	}
	if x.RowBreaks != nil {
		p.RowBreaks = breaks(x.RowBreaks)
	}
	if x.ColBreaks != nil {
		p.ColBreaks = breaks(x.ColBreaks)
	}
	for _, b := range []struct {
		dst *bool
		v   *bool
	}{{&p.Gridlines, x.Gridlines}, {&p.Headings, x.Headings}, {&p.CenterH, x.CenterH}, {&p.CenterV, x.CenterV}, {&p.BlackWhite, x.BlackWhite}} {
		if b.v != nil {
			*b.dst = *b.v
		}
	}
	if x.Order != nil {
		switch *x.Order {
		case "down":
			p.AcrossFirst = false
		case "across":
			p.AcrossFirst = true
		default:
			return msg.New("api.pageOrder", nil)
		}
	}
	if x.Header != nil {
		p.Header = canvas.HeaderFooter(*x.Header)
	}
	if x.Footer != nil {
		p.Footer = canvas.HeaderFooter(*x.Footer)
	}
	return nil
}

func init() {
	register(
		&Command{
			Name:    "sheets.page_setup",
			Summary: "Lit la mise en page d'une feuille (Affichage › Mise en page) : papier, orientation, marges, échelle, zone d'impression, titres, sauts de page, en-tête et pied de page.",
			Details: "Lignes et colonnes à partir de 1, marges en centimètres. scale vaut 100 par défaut ; fit_wide et fit_tall (0 : automatique) ajustent la feuille à un nombre de pages en largeur et en hauteur. row_breaks : lignes qui commencent une page (saut manuel avant elles). order : \"down\" (vers le bas, puis à droite) ou \"across\". En-tête et pied de page : codes &[Page], &[Pages], &[Date], &[Heure], &[Fichier], &[Chemin], &[Onglet].",
			Returns: "{paper, orientation, margins = {top, bottom, left, right, header, footer}, scale, fit_wide, fit_tall, print_area, title_rows, title_cols, row_breaks, col_breaks, gridlines, headings, center_h, center_v, order, black_white, first_page, header = {left, center, right}, footer}",
			Params:  []Param{pSheet},
			Example: `local p = sheets.page_setup()
print(p.orientation, p.print_area)`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				p, err := d.PageSetup(c, s.ID)
				return describePageSetup(p), err
			},
		},
		&Command{
			Name:    "sheets.set_page_setup",
			Label:   "Mise en page",
			Summary: "Change la mise en page d'une feuille : seuls les champs donnés changent (mêmes noms que sheets.page_setup).",
			Details: "paper : A4, A3, A5, B5, Letter, Legal, Tabloid, Executive. orientation : \"portrait\" ou \"landscape\". margins : \"normal\", \"wide\", \"narrow\" ou une table en cm. print_area : \"A1:F40\" (\"\" : toute la plage utilisée). title_rows : \"1:2\" ; title_cols : \"A:B\" (\"\" : aucun). reset = true repart des réglages par défaut. Fichier › Imprimer / PDF et les exports .xlsx et .ods s'en servent.",
			Params:  []Param{{Name: "setup", Type: Object, Desc: "champs à changer"}, {Name: "reset", Type: Boolean, Desc: "repartir des réglages par défaut", Optional: true}, pSheet},
			Write:   true,
			Example: `sheets.set_page_setup({orientation = "landscape", fit_wide = 1, fit_tall = 0,
  title_rows = "1:1", footer = {center = "Page &[Page] sur &[Pages]"}})`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				raw, err := json.Marshal(c.Raw("setup"))
				if err != nil {
					return nil, err
				}
				var x pageSetupPatch
				dec := json.NewDecoder(bytes.NewReader(raw))
				dec.DisallowUnknownFields()
				if err := dec.Decode(&x); err != nil {
					return nil, fmt.Errorf("setup : %v", err)
				}
				reset, err := c.Bool("reset")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				var p canvas.PageSetup
				if !reset {
					if p, err = d.PageSetup(c, s.ID); err != nil {
						return nil, err
					}
				}
				if err := x.apply(&p); err != nil {
					return nil, err
				}
				if err := d.SetPageSetup(c, s.ID, p); err != nil {
					return nil, err
				}
				p, err = d.PageSetup(c, s.ID)
				return describePageSetup(p), err
			},
		},
	)
}
