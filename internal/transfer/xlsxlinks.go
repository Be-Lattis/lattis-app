package transfer

import (
	"bufio"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Liens hypertexte des cellules dans un fichier .xlsx : balises
// <hyperlink> de la feuille, après les cellules ; l'adresse d'un lien vers
// l'extérieur est une relation de la feuille (TargetMode="External"), celle
// d'un lien dans le classeur un attribut location (« 'Feuil 2'!A1 »).
// Dans Lattis, le lien est la propriété ln de la cellule (canvas/links.go).

// xlsxLink est une balise <hyperlink> lue : plage, relation, emplacement.
type xlsxLink struct {
	ref, id, loc string
}

// maxLinkCells borne les cellules d'une plage liée importée.
const maxLinkCells = 10_000

// sheetLinks donne les liens d'une feuille lue (scan), adresses vérifiées
// (les schémas refusés, comme file: ou javascript:, sont laissés).
func (x *xlsxFile) sheetLinks(sh xlsxSheet) []canvas.ImportLink {
	x.mu.Lock()
	links := x.links[sh.name]
	x.mu.Unlock()
	if len(links) == 0 {
		return nil
	}
	targets := x.linkTargets(sh.file.Name)
	var out []canvas.ImportLink
	for _, l := range links {
		target := targets[l.id]
		switch {
		case target != "" && l.loc != "":
			target += "#" + l.loc
		case target == "" && l.loc != "":
			target = "#" + l.loc
		}
		target, err := canvas.NormalizeLink(target)
		if err != nil || target == "" {
			continue
		}
		r0, c0, r1, c1, ok := parseRange(l.ref)
		if !ok || (r1-r0+1)*(c1-c0+1) > maxLinkCells {
			continue
		}
		for r := r0; r <= r1; r++ {
			for c := c0; c <= c1; c++ {
				out = append(out, canvas.ImportLink{Row: r, Col: c, Target: target})
			}
		}
	}
	return out
}

// parseRange lit « B2 » ou « B2:D5 ».
func parseRange(s string) (r0, c0, r1, c1 int, ok bool) {
	a, b, two := strings.Cut(s, ":")
	if r0, c0, ok = parseCellRef(a); !ok {
		return
	}
	r1, c1 = r0, c0
	if two {
		if r1, c1, ok = parseCellRef(b); !ok {
			return
		}
	}
	return min(r0, r1), min(c0, c1), max(r0, r1), max(c0, c1), true
}

// linkTargets lit les relations hypertexte d'une feuille, adresses telles
// quelles (rels les rendrait relatives au ZIP).
func (x *xlsxFile) linkTargets(part string) map[string]string {
	dir, base := path.Split(part)
	out := map[string]string{}
	f := x.find(dir + "_rels/" + base + ".rels")
	if f == nil {
		return out
	}
	var rs struct {
		Rels []struct {
			ID     string `xml:"Id,attr"`
			Type   string `xml:"Type,attr"`
			Target string `xml:"Target,attr"`
			Mode   string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if x.decode(f, &rs) != nil {
		return out
	}
	for _, r := range rs.Rels {
		if path.Base(r.Type) == "hyperlink" && strings.EqualFold(r.Mode, "External") {
			out[r.ID] = r.Target
		}
	}
	return out
}

// xlsxHyperlink est un lien à écrire : adresse extérieure (relation) ou
// emplacement dans le classeur.
type xlsxHyperlink struct {
	ref, url, loc string
}

// excelLink traduit l'adresse d'un lien de Lattis pour une cellule de la
// feuille sheet (nom exporté) : url (page web, e-mail) ou loc (emplacement
// dans le classeur). b nil : liens dans le classeur non traduits.
func excelLink(ln, sheet string, b *book) (url, loc string) {
	il, ok := canvas.ParseInternalLink(ln)
	if !ok {
		return ln, ""
	}
	if b == nil {
		return "", ""
	}
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	place := func(sh string, r0, c0, r1, c1 int) string {
		ref := colName(c0) + strconv.Itoa(r0+1)
		if r1 != r0 || c1 != c0 {
			ref += ":" + colName(c1) + strconv.Itoa(r1+1)
		}
		return q(sh) + "!" + ref
	}
	if il.Name != "" {
		if p := b.tables[strings.ToLower(il.Name)]; p != nil {
			return "", place(p.sheet, p.row, p.col, p.lastRow(), p.col+max(len(p.cols), 1)-1)
		}
		if p, ok := b.names[strings.ToLower(il.Name)]; ok {
			return "", place(p.sheet, p.row, p.col, p.row, p.col)
		}
		return "", il.Name
	}
	sh := sheet
	if il.Sheet != "" {
		if n, ok := b.sheets[strings.ToLower(il.Sheet)]; ok {
			sh = n
		} else {
			sh = il.Sheet
		}
	}
	return "", q(sh) + "!" + il.Ref
}

// linkLook : l'allure d'un lien dans le fichier (bleu souligné, comme à
// l'écran), sous la mise en forme propre de la cellule.
var linkLook = canvas.Style{"fg": "#0563C1", "u": true}

// tableLink : le lien d'une cellule de tableau (son style, sinon celui de
// sa colonne).
func tableLink(own, col canvas.Style) string {
	if v, ok := own["ln"]; ok {
		s, _ := v.(string)
		return s
	}
	s, _ := col["ln"].(string)
	return s
}

// writeHyperlinks écrit la balise <hyperlinks> ; les liens extérieurs
// prennent les relations rId<first>, rId<first+1>… et sont rendus dans
// cet ordre.
func writeHyperlinks(bw *bufio.Writer, links []xlsxHyperlink, first int) []string {
	if len(links) == 0 {
		return nil
	}
	var urls []string
	bw.WriteString(`<hyperlinks>`)
	for _, l := range links {
		if l.url != "" {
			urls = append(urls, l.url)
			fmt.Fprintf(bw, `<hyperlink ref="%s" r:id="rId%d"/>`, l.ref, first+len(urls)-1)
		} else {
			fmt.Fprintf(bw, `<hyperlink ref="%s" location="%s"/>`, l.ref, xmlEscape(l.loc))
		}
	}
	bw.WriteString(`</hyperlinks>`)
	return urls
}
