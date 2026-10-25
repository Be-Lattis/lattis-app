package canvas

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"math"
	"strings"
)

// PrintObject est un objet flottant (image, forme, zone de texte) prêt à
// être dessiné par un rendu d'impression ou d'export : sa place (cellule
// d'ancrage, décalage et taille en pixels CSS, comme la grille) et un
// fragment HTML autonome qui le dessine, de Width × Height pixels.
//
// Le rendu place le fragment en position absolue au point
// (gauche de la colonne Col + OffX, haut de la ligne Row + OffY) de la
// feuille, à son échelle ; les objets viennent dans l'ordre de la liste
// (du fond vers le premier plan). Un objet qui chevauche deux pages est
// découpé par le rendu (overflow: hidden sur la page).
type PrintObject struct {
	ID     int64  `json:"id"`
	Kind   string `json:"kind"`
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	OffX   int    `json:"offX"`
	OffY   int    `json:"offY"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	HTML   string `json:"html"`
}

// PrintObjects donne les objets flottants d'une feuille pour l'impression,
// du fond vers le premier plan (voir PrintObject). Les images sont
// incluses en data: (PNG, JPEG, GIF, WebP : jamais de SVG, voir
// imageTypes) ; une image absente du classeur donne un cadre vide.
func (m *Model) PrintObjects(ctx context.Context, sheet int64) ([]PrintObject, error) {
	var out []PrintObject
	err := m.read(ctx, func(t *tx) error {
		objs, err := t.objects(sheet)
		if err != nil {
			return err
		}
		out = make([]PrintObject, 0, len(objs))
		for _, o := range objs {
			src := ""
			if o.Config.Kind == ObjImage {
				var data []byte
				var typ string
				if t.row(`SELECT data, type FROM _lattis_images WHERE hash = ?`, o.Config.Image).Scan(&data, &typ) == nil && imageTypes[typ] {
					src = "data:" + typ + ";base64," + base64.StdEncoding.EncodeToString(data)
				}
			}
			c := o.Config
			out = append(out, PrintObject{ID: o.ID, Kind: c.Kind, Row: o.Row, Col: o.Col, OffX: c.OffX, OffY: c.OffY, Width: c.Width, Height: c.Height, HTML: ObjectHTML(c, src)})
		}
		return nil
	})
	return out, err
}

// ObjectHTML dessine un objet en HTML autonome (SVG pour les formes, texte
// par-dessus), de cfg.Width × cfg.Height pixels. src : adresse de l'image
// (data:…) d'un objet image.
func ObjectHTML(cfg ObjectConfig, src string) string {
	w, h := cfg.Width, cfg.Height
	var b strings.Builder
	fmt.Fprintf(&b, `<div style="position:relative;width:%dpx;height:%dpx;overflow:visible">`, w, h)
	if cfg.Kind == ObjImage {
		if src != "" {
			fmt.Fprintf(&b, `<img src="%s" alt="%s" style="display:block;width:%dpx;height:%dpx">`, html.EscapeString(src), html.EscapeString(cfg.Alt), w, h)
		} else {
			fmt.Fprintf(&b, `<div style="width:%dpx;height:%dpx;box-sizing:border-box;border:1px dashed #9ca3af"></div>`, w, h)
		}
		b.WriteString(`</div>`)
		return b.String()
	}
	b.WriteString(ShapeSVG(cfg))
	if cfg.Text != "" && !IsLine(cfg.Kind) {
		just := map[string]string{"left": "flex-start", "right": "flex-end"}[cfg.Align]
		if just == "" {
			just = "center"
			if cfg.Kind == ObjText && cfg.Align == "" {
				just = "flex-start"
			}
		}
		items := map[string]string{"top": "flex-start", "bottom": "flex-end", "middle": "center"}[cfg.VAlign]
		if items == "" {
			items = "flex-start"
			if cfg.Kind != ObjText {
				items = "center"
			}
		}
		size := cfg.Size
		if size == 0 {
			size = 11
		}
		color := cfg.Color
		if color == "" {
			color = "#000000"
		}
		style := fmt.Sprintf("position:absolute;inset:0;display:flex;justify-content:%s;align-items:%s;padding:4px 6px;box-sizing:border-box;overflow:hidden;white-space:pre-wrap;word-break:break-word;font-size:%gpt;color:%s;text-align:%s", just, items, size, color, textAlign(cfg))
		if cfg.Bold {
			style += ";font-weight:bold"
		}
		if cfg.Italic {
			style += ";font-style:italic"
		}
		fmt.Fprintf(&b, `<div style="%s"><span>%s</span></div>`, style, html.EscapeString(cfg.Text))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func textAlign(cfg ObjectConfig) string {
	if cfg.Align == "" {
		if cfg.Kind == ObjText {
			return "left"
		}
		return "center"
	}
	return cfg.Align
}

// ShapeSVG dessine une forme (ni image ni texte) en SVG de sa taille.
func ShapeSVG(cfg ObjectConfig) string {
	w, h := float64(cfg.Width), float64(cfg.Height)
	lw := cfg.LineWidth
	stroke := `stroke="none"`
	if cfg.Line != "" && lw > 0 {
		stroke = fmt.Sprintf(`stroke="%s" stroke-width="%g"`, cfg.Line, lw)
	} else {
		lw = 0
	}
	fill := `fill="none"`
	if cfg.Fill != "" {
		fill = fmt.Sprintf(`fill="%s" fill-opacity="%g"`, cfg.Fill, 1-float64(cfg.Transparency)/100)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%g" height="%g" viewBox="0 0 %g %g" style="position:absolute;left:0;top:0;overflow:visible">`, w, h, w, h)
	i := lw / 2
	iw, ih := math.Max(0, w-lw), math.Max(0, h-lw)
	switch cfg.Kind {
	case ObjRect, ObjText:
		fmt.Fprintf(&b, `<rect x="%g" y="%g" width="%g" height="%g" %s %s/>`, i, i, iw, ih, fill, stroke)
	case ObjRoundRect:
		r := math.Min(iw, ih) * 0.16
		fmt.Fprintf(&b, `<rect x="%g" y="%g" width="%g" height="%g" rx="%g" %s %s/>`, i, i, iw, ih, r, fill, stroke)
	case ObjEllipse:
		fmt.Fprintf(&b, `<ellipse cx="%g" cy="%g" rx="%g" ry="%g" %s %s/>`, w/2, h/2, iw/2, ih/2, fill, stroke)
	case ObjTriangle:
		fmt.Fprintf(&b, `<polygon points="%g,%g %g,%g %g,%g" stroke-linejoin="round" %s %s/>`, w/2, i, w-i, h-i, i, h-i, fill, stroke)
	case ObjLine, ObjArrow:
		x1, y1, x2, y2 := LineEnds(cfg)
		fmt.Fprintf(&b, `<line x1="%g" y1="%g" x2="%g" y2="%g" stroke-linecap="round" %s/>`, x1, y1, x2, y2, stroke)
		if cfg.Kind == ObjArrow && cfg.Line != "" {
			fmt.Fprintf(&b, `<polygon points="%s" fill="%s"/>`, arrowHead(x1, y1, x2, y2, cfg.LineWidth), cfg.Line)
		}
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// LineEnds donne les extrémités d'une ligne ou d'une flèche dans sa boîte
// (la pointe d'une flèche au second point).
func LineEnds(cfg ObjectConfig) (x1, y1, x2, y2 float64) {
	w, h := float64(cfg.Width), float64(cfg.Height)
	x1, x2 = 0, w
	if cfg.FlipH {
		x1, x2 = w, 0
	}
	y1, y2 = 0, h
	if cfg.FlipV {
		y1, y2 = h, 0
	}
	return
}

// arrowHead : triangle de la pointe d'une flèche (x1, y1) → (x2, y2).
func arrowHead(x1, y1, x2, y2, lw float64) string {
	l := math.Hypot(x2-x1, y2-y1)
	if l == 0 {
		return ""
	}
	size := math.Max(8, lw*4)
	ux, uy := (x2-x1)/l, (y2-y1)/l
	bx, by := x2-ux*size, y2-uy*size
	px, py := -uy*size/2, ux*size/2
	return fmt.Sprintf("%g,%g %g,%g %g,%g", round2(x2), round2(y2), round2(bx+px), round2(by+py), round2(bx-px), round2(by-py))
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
