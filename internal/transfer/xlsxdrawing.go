package transfer

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Images, formes et zones de texte d'une feuille dans un fichier .xlsx :
// une partie de dessin (xl/drawings/drawingN.xml) par feuille qui en a,
// les images dans xl/media. À l'écriture, chaque objet est ancré à sa
// cellule (oneCellAnchor : cellule, décalage et taille, comme dans Lattis) ;
// à la lecture, les ancrages à une ou deux cellules et absolus sont repris
// (images, formes rectangle, rectangle arrondi, ellipse, triangle, ligne,
// flèche et zones de texte, avec leur texte et ses couleurs ; les autres
// formes deviennent des rectangles, les couleurs du thème prennent celles
// par défaut).

const emuPerPx = 9525

// xpic est une image à écrire.
type xpic struct {
	data []byte
	typ  string
}

// sheetObjects lit les objets d'une feuille et leurs images (export).
func sheetObjects(ctx context.Context, m *canvas.Model, objs []canvas.ObjectInfo) map[string]xpic {
	pics := map[string]xpic{}
	for _, o := range objs {
		if h := o.Config.Image; o.Config.Kind == canvas.ObjImage && h != "" {
			if _, ok := pics[h]; ok {
				continue
			}
			if data, typ, err := m.ImageData(ctx, h); err == nil {
				pics[h] = xpic{data, typ}
			}
		}
	}
	return pics
}

var picExt = map[string]string{"image/png": "png", "image/jpeg": "jpeg", "image/gif": "gif", "image/webp": "webp"}

// xlsxMedia : images du classeur (nom de la partie par empreinte).
type xlsxMedia struct {
	names map[string]string
	files []struct {
		name string
		data []byte
	}
	exts map[string]string // extension → type
}

func (md *xlsxMedia) add(hash string, p xpic) string {
	if n, ok := md.names[hash]; ok {
		return n
	}
	ext := picExt[p.typ]
	if ext == "" {
		return ""
	}
	n := fmt.Sprintf("image%d.%s", len(md.files)+1, ext)
	md.names[hash] = n
	md.files = append(md.files, struct {
		name string
		data []byte
	}{n, p.data})
	md.exts[ext] = p.typ
	return n
}

// drawingXML écrit la partie de dessin d'une feuille et ses relations ;
// "" si la feuille n'a pas d'objet exportable.
func drawingXML(s *xsheet, md *xlsxMedia) (string, string) {
	if len(s.objects) == 0 && (s.extra == nil || len(s.extra.charts) == 0) {
		return "", ""
	}
	var b, rels strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	n, rid := 0, 0
	for _, o := range s.objects {
		c := o.Config
		if o.Row >= ExcelRows || o.Col >= ExcelCols {
			continue
		}
		embed := ""
		if c.Kind == canvas.ObjImage {
			p, ok := s.pics[c.Image]
			if !ok {
				continue
			}
			name := md.add(c.Image, p)
			if name == "" {
				continue
			}
			rid++
			embed = fmt.Sprintf("rId%d", rid)
			fmt.Fprintf(&rels, `<Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/%s"/>`, embed, name)
		}
		n++
		cx, cy := int64(c.Width)*emuPerPx, int64(c.Height)*emuPerPx
		fmt.Fprintf(&b, `<xdr:oneCellAnchor><xdr:from><xdr:col>%d</xdr:col><xdr:colOff>%d</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>%d</xdr:rowOff></xdr:from><xdr:ext cx="%d" cy="%d"/>`,
			o.Col, int64(c.OffX)*emuPerPx, o.Row, int64(c.OffY)*emuPerPx, cx, cy)
		xfrm := func(flip string) string {
			return fmt.Sprintf(`<a:xfrm%s><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, flip, cx, cy)
		}
		if embed != "" {
			fmt.Fprintf(&b, `<xdr:pic><xdr:nvPicPr><xdr:cNvPr id="%d" name="Image %d" descr="%s"/><xdr:cNvPicPr><a:picLocks noChangeAspect="1"/></xdr:cNvPicPr></xdr:nvPicPr><xdr:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></xdr:blipFill><xdr:spPr>%s<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></xdr:spPr></xdr:pic>`,
				n+1, n, xmlEscape(c.Alt), embed, xfrm(""))
		} else {
			b.WriteString(shapeXML(c, n, xfrm))
		}
		b.WriteString(`<xdr:clientData/></xdr:oneCellAnchor>`)
	}
	n, _ = chartFrames(&b, &rels, s, n, rid) // graphiques (xlsxchartwrite.go)
	if n == 0 {
		return "", ""
	}
	b.WriteString(`</xdr:wsDr>`)

	rels.WriteString(`</Relationships>`)
	return b.String(), rels.String()
}

var prstOf = map[string]string{canvas.ObjRect: "rect", canvas.ObjText: "rect", canvas.ObjRoundRect: "roundRect", canvas.ObjEllipse: "ellipse", canvas.ObjTriangle: "triangle", canvas.ObjLine: "line", canvas.ObjArrow: "line"}

func hexOf(c string) string { return strings.ToUpper(strings.TrimPrefix(c, "#")) }

// shapeXML écrit une forme (xdr:sp) avec son texte.
func shapeXML(c canvas.ObjectConfig, n int, xfrm func(string) string) string {
	var b strings.Builder
	name := fmt.Sprintf("Forme %d", n)
	txBox := ""
	if c.Kind == canvas.ObjText {
		name, txBox = fmt.Sprintf("Zone de texte %d", n), ` txBox="1"`
	}
	fmt.Fprintf(&b, `<xdr:sp macro="" textlink=""><xdr:nvSpPr><xdr:cNvPr id="%d" name="%s"/><xdr:cNvSpPr%s/></xdr:nvSpPr><xdr:spPr>`, n+1, name, txBox)
	flip := ""
	if c.FlipH {
		flip += ` flipH="1"`
	}
	if c.FlipV {
		flip += ` flipV="1"`
	}
	b.WriteString(xfrm(flip))
	fmt.Fprintf(&b, `<a:prstGeom prst="%s"><a:avLst/></a:prstGeom>`, prstOf[c.Kind])
	if c.Fill != "" {
		alpha := ""
		if c.Transparency > 0 {
			alpha = fmt.Sprintf(`<a:alpha val="%d"/>`, (100-c.Transparency)*1000)
		}
		fmt.Fprintf(&b, `<a:solidFill><a:srgbClr val="%s">%s</a:srgbClr></a:solidFill>`, hexOf(c.Fill), alpha)
	} else if !canvas.IsLine(c.Kind) {
		b.WriteString(`<a:noFill/>`)
	}
	if c.Line != "" && c.LineWidth > 0 {
		fmt.Fprintf(&b, `<a:ln w="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, int64(c.LineWidth*emuPerPx), hexOf(c.Line))
		if c.Kind == canvas.ObjArrow {
			b.WriteString(`<a:tailEnd type="triangle"/>`)
		}
		b.WriteString(`</a:ln>`)
	} else {
		b.WriteString(`<a:ln><a:noFill/></a:ln>`)
	}
	b.WriteString(`</xdr:spPr>`)
	if c.Text != "" && !canvas.IsLine(c.Kind) {
		anchor := map[string]string{"top": "t", "middle": "ctr", "bottom": "b"}[c.VAlign]
		if anchor == "" {
			anchor = "t"
		}
		algn := map[string]string{"left": "l", "center": "ctr", "right": "r"}[c.Align]
		if algn == "" {
			algn = "l"
		}
		size := c.Size
		if size == 0 {
			size = 11
		}
		rpr := fmt.Sprintf(` lang="fr-FR" sz="%d"`, int(size*100))
		if c.Bold {
			rpr += ` b="1"`
		}
		if c.Italic {
			rpr += ` i="1"`
		}
		color := "000000"
		if c.Color != "" {
			color = hexOf(c.Color)
		}
		fmt.Fprintf(&b, `<xdr:txBody><a:bodyPr vertOverflow="clip" wrap="square" rtlCol="0" anchor="%s"/><a:lstStyle/>`, anchor)
		for _, line := range strings.Split(c.Text, "\n") {
			fmt.Fprintf(&b, `<a:p><a:pPr algn="%s"/><a:r><a:rPr%s><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:rPr><a:t>%s</a:t></a:r></a:p>`, algn, rpr, color, xmlEscape(line))
		}
		b.WriteString(`</xdr:txBody>`)
	}
	b.WriteString(`</xdr:sp>`)
	return b.String()
}

// --- Lecture ------------------------------------------------------------------------

type xdrMarker struct {
	Col    int64 `xml:"col"`
	ColOff int64 `xml:"colOff"`
	Row    int64 `xml:"row"`
	RowOff int64 `xml:"rowOff"`
}

type xdrExt struct {
	Cx int64 `xml:"cx,attr"`
	Cy int64 `xml:"cy,attr"`
}

type xdrColor struct {
	Srgb *struct {
		Val   string `xml:"val,attr"`
		Alpha *struct {
			Val int `xml:"val,attr"`
		} `xml:"alpha"`
	} `xml:"srgbClr"`
}

type xdrEnd struct {
	Type string `xml:"type,attr"`
}

type xdrSpPr struct {
	Xfrm struct {
		FlipH string  `xml:"flipH,attr"`
		FlipV string  `xml:"flipV,attr"`
		Ext   *xdrExt `xml:"ext"`
	} `xml:"xfrm"`
	Geom struct {
		Prst string `xml:"prst,attr"`
	} `xml:"prstGeom"`
	Fill  *xdrColor `xml:"solidFill"`
	NoFil *struct{} `xml:"noFill"`
	Ln    *struct {
		W     int64     `xml:"w,attr"`
		Fill  *xdrColor `xml:"solidFill"`
		NoFil *struct{} `xml:"noFill"`
		Tail  *xdrEnd   `xml:"tailEnd"`
		Head  *xdrEnd   `xml:"headEnd"`
	} `xml:"ln"`
}

type xdrShape struct {
	Nv struct {
		Sp struct {
			TxBox string `xml:"txBox,attr"`
		} `xml:"cNvSpPr"`
	} `xml:"nvSpPr"`
	SpPr xdrSpPr `xml:"spPr"`
	Body *struct {
		BodyPr struct {
			Anchor string `xml:"anchor,attr"`
		} `xml:"bodyPr"`
		Paras []struct {
			PPr struct {
				Algn string `xml:"algn,attr"`
			} `xml:"pPr"`
			Runs []struct {
				RPr struct {
					Sz   int       `xml:"sz,attr"`
					B    string    `xml:"b,attr"`
					I    string    `xml:"i,attr"`
					Fill *xdrColor `xml:"solidFill"`
				} `xml:"rPr"`
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"p"`
	} `xml:"txBody"`
}

type xdrAnchor struct {
	XMLName xml.Name
	From    *xdrMarker `xml:"from"`
	Ext     *xdrExt    `xml:"ext"`
	Pos     *struct {
		X int64 `xml:"x,attr"`
		Y int64 `xml:"y,attr"`
	} `xml:"pos"`
	Pic *struct {
		Nv struct {
			C struct {
				Descr string `xml:"descr,attr"`
			} `xml:"cNvPr"`
		} `xml:"nvPicPr"`
		Fill struct {
			Blip struct {
				Embed string `xml:"embed,attr"`
			} `xml:"blip"`
		} `xml:"blipFill"`
		SpPr xdrSpPr `xml:"spPr"`
	} `xml:"pic"`
	Sp    *xdrShape `xml:"sp"`
	CxnSp *xdrShape `xml:"cxnSp"`
}

type xdrDrawing struct {
	Anchors []xdrAnchor `xml:",any"`
}

func colorOf(c *xdrColor) string {
	if c == nil || c.Srgb == nil || len(c.Srgb.Val) != 6 {
		return ""
	}
	if _, err := strconv.ParseUint(c.Srgb.Val, 16, 32); err != nil {
		return ""
	}
	return "#" + strings.ToLower(c.Srgb.Val)
}

// readDrawings lit les images et formes de chaque feuille (ses parties de
// dessin) ; rend le nombre d'images lues.
func (x *xlsxFile) readDrawings(sheets []canvas.ImportSheet) int {
	pics := 0
	for i, sh := range x.sheets {
		if i >= len(sheets) || sh.file == nil {
			continue
		}
		_, byType, err := x.rels(sh.file.Name)
		if err != nil || byType["drawing"] == "" {
			continue
		}
		dp := byType["drawing"]
		f := x.find(dp)
		if f == nil {
			continue
		}
		var d xdrDrawing
		if x.decode(f, &d) != nil {
			continue
		}
		media, _, _ := x.rels(dp)
		for _, a := range d.Anchors {
			o, ok := x.anchorObject(a, media)
			if ok {
				if o.Data != nil {
					pics++
				}
				sheets[i].Objects = append(sheets[i].Objects, o)
			}
		}
	}
	return pics
}

const maxMedia = canvas.MaxImageBytes

func (x *xlsxFile) anchorObject(a xdrAnchor, media map[string]string) (canvas.ImportObject, bool) {
	var o canvas.ImportObject
	switch {
	case a.From != nil:
		o.Row, o.Col = int(a.From.Row), int(a.From.Col)
		o.Config.OffX, o.Config.OffY = int(a.From.ColOff/emuPerPx), int(a.From.RowOff/emuPerPx)
	case a.Pos != nil:
		// Ancrage absolu : approché sur une grille de 64 × 20 pixels.
		px, py := int(a.Pos.X/emuPerPx), int(a.Pos.Y/emuPerPx)
		o.Col, o.Config.OffX, o.Row, o.Config.OffY = px/64, px%64, py/20, py%20
	default:
		return o, false
	}
	size := func(sp *xdrSpPr) {
		e := a.Ext
		if sp != nil && sp.Xfrm.Ext != nil && (sp.Xfrm.Ext.Cx > 0 || sp.Xfrm.Ext.Cy > 0) {
			e = sp.Xfrm.Ext
		}
		if e != nil {
			o.Config.Width, o.Config.Height = int(e.Cx/emuPerPx), int(e.Cy/emuPerPx)
		}
	}
	switch {
	case a.Pic != nil:
		target := media[a.Pic.Fill.Blip.Embed]
		f := x.find(target)
		if f == nil || f.UncompressedSize64 > maxMedia {
			return o, false
		}
		r, err := f.Open()
		if err != nil {
			return o, false
		}
		data, err := io.ReadAll(io.LimitReader(r, maxMedia+1))
		r.Close()
		if err != nil {
			return o, false
		}
		o.Data = data
		o.Config.Kind = canvas.ObjImage
		o.Config.Alt = a.Pic.Nv.C.Descr
		if o.Config.Alt == "" {
			o.Config.Alt = path.Base(target)
		}
		size(&a.Pic.SpPr)
		return o, true
	case a.Sp != nil || a.CxnSp != nil:
		sp := a.Sp
		if sp == nil {
			sp = a.CxnSp
		}
		size(&sp.SpPr)
		readShape(&o.Config, sp, a.CxnSp != nil)
		return o, true
	}
	return o, false
}

// readShape reprend la forme, ses couleurs et son texte.
func readShape(c *canvas.ObjectConfig, sp *xdrShape, connector bool) {
	pr := sp.SpPr
	switch pr.Geom.Prst {
	case "roundRect":
		c.Kind = canvas.ObjRoundRect
	case "ellipse":
		c.Kind = canvas.ObjEllipse
	case "triangle":
		c.Kind = canvas.ObjTriangle
	case "line", "straightConnector1":
		c.Kind = canvas.ObjLine
	default:
		c.Kind = canvas.ObjRect
		if connector {
			c.Kind = canvas.ObjLine
		}
	}
	if sp.Nv.Sp.TxBox == "1" {
		c.Kind = canvas.ObjText
	}
	if canvas.IsLine(c.Kind) {
		c.FlipH, c.FlipV = pr.Xfrm.FlipH == "1", pr.Xfrm.FlipV == "1"
		if ln := pr.Ln; ln != nil && (ln.Tail != nil && ln.Tail.Type != "" && ln.Tail.Type != "none") {
			c.Kind = canvas.ObjArrow
		} else if ln != nil && ln.Head != nil && ln.Head.Type != "" && ln.Head.Type != "none" {
			// Pointe au début : la flèche est retournée.
			c.Kind, c.FlipH, c.FlipV = canvas.ObjArrow, !c.FlipH, !c.FlipV
		}
	}
	// Couleurs données : gardées ; sinon (thème) celles par défaut.
	if pr.Fill != nil {
		c.Fill = colorOf(pr.Fill)
		if a := pr.Fill.Srgb; a != nil && a.Alpha != nil && a.Alpha.Val < 100000 {
			c.Transparency = 100 - a.Alpha.Val/1000
		}
	}
	if ln := pr.Ln; ln != nil {
		if ln.Fill != nil {
			c.Line = colorOf(ln.Fill)
		}
		if ln.W > 0 {
			c.LineWidth = float64(ln.W) / emuPerPx
		}
	}
	if pr.NoFil != nil && c.Fill == "" && !canvas.IsLine(c.Kind) {
		c.Fill = ""
		if c.Line == "" {
			c.Line = "#7f8790" // ni fond ni contour : un trait discret pour la voir
		}
	}
	if sp.Body == nil || canvas.IsLine(c.Kind) {
		return
	}
	var lines []string
	for _, p := range sp.Body.Paras {
		var t strings.Builder
		for _, r := range p.Runs {
			t.WriteString(r.T)
			if r.RPr.Sz > 0 && c.Size == 0 {
				c.Size = float64(r.RPr.Sz) / 100
			}
			if r.RPr.B == "1" {
				c.Bold = true
			}
			if r.RPr.I == "1" {
				c.Italic = true
			}
			if col := colorOf(r.RPr.Fill); col != "" && c.Color == "" {
				c.Color = col
			}
		}
		lines = append(lines, t.String())
		if c.Align == "" {
			c.Align = map[string]string{"l": "left", "ctr": "center", "r": "right"}[p.PPr.Algn]
		}
	}
	c.Text = strings.TrimRight(strings.Join(lines, "\n"), "\n")
	c.VAlign = map[string]string{"t": "top", "ctr": "middle", "b": "bottom"}[sp.Body.BodyPr.Anchor]
	if c.Size < 4 || c.Size > 200 {
		c.Size = 0
	}
}
