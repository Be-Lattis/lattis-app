package transfer

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Vue normalisée d'un fichier .xlsx, pour comparer un fichier et ce que
// Lattis en réécrit (essais aller-retour, roundtrip_test.go) : valeurs,
// formules, formats et styles des cellules, mise en page de chaque feuille,
// noms définis, objets.

type xcellDump struct {
	v   string // valeur (texte, nombre tel qu'écrit, booléen, erreur)
	f   string // formule (sans =), "" sans
	fmt string // code de format de nombre ("" : Standard)
	st  string // police, remplissage, bordures, alignement
}

type xsheetDump struct {
	name   string
	cells  map[string]xcellDump
	merges []string
	cols   []string // min-max:largeur (arrondie), masquée
	pane   string
	cf     []string // plage:type
	dv     []string // plage:type
	links  []string // cellule
	notes  int
	tables []string // nom:plage
	charts int
	images int
}

type xbookDump struct {
	sheets []*xsheetDump
	names  map[string]string // noms définis (sans _xlnm)
	pivots int
}

func (b *xbookDump) sheet(name string) *xsheetDump {
	for _, s := range b.sheets {
		if s.name == name {
			return s
		}
	}
	return nil
}

func dumpXLSX(t *testing.T, p string) *xbookDump {
	t.Helper()
	zr, err := zip.OpenReader(p)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[strings.TrimPrefix(f.Name, "/")] = f
	}
	read := func(name string, v any) bool {
		f := files[strings.TrimPrefix(name, "/")]
		if f == nil {
			return false
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		if err := xml.NewDecoder(r).Decode(v); err != nil && err != io.EOF {
			t.Fatalf("%s : %v", name, err)
		}
		return true
	}
	type rel struct {
		ID     string `xml:"Id,attr"`
		Type   string `xml:"Type,attr"`
		Target string `xml:"Target,attr"`
	}
	rels := func(part string) []rel {
		var rs struct {
			R []rel `xml:"Relationship"`
		}
		dir, base := path.Split(part)
		read(dir+"_rels/"+base+".rels", &rs)
		for i := range rs.R {
			if !strings.HasPrefix(rs.R[i].Target, "/") && !strings.Contains(rs.R[i].Target, "://") && !strings.HasPrefix(rs.R[i].Target, "mailto:") {
				rs.R[i].Target = path.Clean(dir + rs.R[i].Target)
			} else {
				rs.R[i].Target = strings.TrimPrefix(rs.R[i].Target, "/")
			}
		}
		return rs.R
	}
	out := &xbookDump{names: map[string]string{}}
	for name := range files {
		if strings.HasPrefix(name, "xl/pivotTables/pivotTable") && strings.HasSuffix(name, ".xml") {
			out.pivots++
		}
	}
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
		Names []struct {
			Name  string `xml:"name,attr"`
			Value string `xml:",chardata"`
		} `xml:"definedNames>definedName"`
	}
	read("xl/workbook.xml", &wb)
	for _, n := range wb.Names {
		if !strings.HasPrefix(n.Name, "_xlnm.") {
			out.names[n.Name] = strings.ReplaceAll(n.Value, "'", "")
		}
	}
	wbRels := map[string]string{}
	var sstPath, stylesPath string
	for _, r := range rels("xl/workbook.xml") {
		wbRels[r.ID] = r.Target
		switch {
		case strings.HasSuffix(r.Type, "/sharedStrings"):
			sstPath = r.Target
		case strings.HasSuffix(r.Type, "/styles"):
			stylesPath = r.Target
		}
	}
	// Chaînes partagées.
	var sst struct {
		SI []struct {
			T string `xml:"t"`
			R []struct {
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"si"`
	}
	read(sstPath, &sst)
	strs := make([]string, len(sst.SI))
	for i, si := range sst.SI {
		strs[i] = si.T
		for _, r := range si.R {
			strs[i] += r.T
		}
	}
	// Styles.
	var styles struct {
		NumFmts []struct {
			ID   int    `xml:"numFmtId,attr"`
			Code string `xml:"formatCode,attr"`
		} `xml:"numFmts>numFmt"`
		Fonts []struct {
			B     *struct{} `xml:"b"`
			I     *struct{} `xml:"i"`
			U     *struct{} `xml:"u"`
			Color struct {
				RGB string `xml:"rgb,attr"`
			} `xml:"color"`
		} `xml:"fonts>font"`
		Fills []struct {
			Fg struct {
				RGB string `xml:"rgb,attr"`
			} `xml:"patternFill>fgColor"`
		} `xml:"fills>fill"`
		Borders []struct {
			Left struct {
				Style string `xml:"style,attr"`
			} `xml:"left"`
			Right struct {
				Style string `xml:"style,attr"`
			} `xml:"right"`
			Top struct {
				Style string `xml:"style,attr"`
			} `xml:"top"`
			Bottom struct {
				Style string `xml:"style,attr"`
			} `xml:"bottom"`
		} `xml:"borders>border"`
		Xfs []struct {
			NumFmt int `xml:"numFmtId,attr"`
			Font   int `xml:"fontId,attr"`
			Fill   int `xml:"fillId,attr"`
			Border int `xml:"borderId,attr"`
			Align  struct {
				H    string `xml:"horizontal,attr"`
				Wrap string `xml:"wrapText,attr"`
			} `xml:"alignment"`
		} `xml:"cellXfs>xf"`
	}
	read(stylesPath, &styles)
	codes := map[int]string{0: "", 14: "date", 10: "0.00%", 9: "0%", 4: "#,##0.00", 2: "0.00", 1: "0", 3: "#,##0", 22: "date-time"}
	for _, n := range styles.NumFmts {
		codes[n.ID] = n.Code
	}
	styleOf := func(s string) (string, string) {
		i, err := strconv.Atoi(s)
		if err != nil || i < 0 || i >= len(styles.Xfs) {
			return "", ""
		}
		x := styles.Xfs[i]
		code, ok := codes[x.NumFmt]
		if !ok {
			code = "#" + strconv.Itoa(x.NumFmt)
		}
		var parts []string
		if x.Font < len(styles.Fonts) {
			f := styles.Fonts[x.Font]
			if f.B != nil {
				parts = append(parts, "b")
			}
			if f.I != nil {
				parts = append(parts, "i")
			}
			if f.U != nil {
				parts = append(parts, "u")
			}
			if c := strings.ToUpper(f.Color.RGB); c != "" && c != "FF000000" {
				parts = append(parts, "fg="+c[len(c)-6:])
			}
		}
		if x.Fill < len(styles.Fills) {
			if c := strings.ToUpper(styles.Fills[x.Fill].Fg.RGB); c != "" {
				parts = append(parts, "bg="+c[len(c)-6:])
			}
		}
		if x.Border < len(styles.Borders) {
			b := styles.Borders[x.Border]
			if s := b.Left.Style + b.Right.Style + b.Top.Style + b.Bottom.Style; s != "" {
				parts = append(parts, "bd="+b.Left.Style+"/"+b.Right.Style+"/"+b.Top.Style+"/"+b.Bottom.Style)
			}
		}
		if x.Align.H != "" && x.Align.H != "general" {
			parts = append(parts, "al="+x.Align.H)
		}
		if x.Align.Wrap == "1" || x.Align.Wrap == "true" {
			parts = append(parts, "wrap")
		}
		return code, strings.Join(parts, " ")
	}
	for _, s := range wb.Sheets {
		part := wbRels[s.ID]
		sd := &xsheetDump{name: s.Name, cells: map[string]xcellDump{}}
		out.sheets = append(out.sheets, sd)
		var ws struct {
			Views []struct {
				Pane *struct {
					X     string `xml:"xSplit,attr"`
					Y     string `xml:"ySplit,attr"`
					State string `xml:"state,attr"`
				} `xml:"pane"`
			} `xml:"sheetViews>sheetView"`
			Cols []struct {
				Min    string `xml:"min,attr"`
				Max    string `xml:"max,attr"`
				Width  string `xml:"width,attr"`
				Hidden string `xml:"hidden,attr"`
			} `xml:"cols>col"`
			Rows []struct {
				C []struct {
					R  string `xml:"r,attr"`
					S  string `xml:"s,attr"`
					T  string `xml:"t,attr"`
					F  string `xml:"f"`
					V  string `xml:"v"`
					IS struct {
						T string `xml:"t"`
					} `xml:"is"`
				} `xml:"c"`
			} `xml:"sheetData>row"`
			Merges []struct {
				Ref string `xml:"ref,attr"`
			} `xml:"mergeCells>mergeCell"`
			CF []struct {
				Sqref string `xml:"sqref,attr"`
				Rules []struct {
					Type string `xml:"type,attr"`
					Op   string `xml:"operator,attr"`
				} `xml:"cfRule"`
			} `xml:"conditionalFormatting"`
			DV []struct {
				Sqref string `xml:"sqref,attr"`
				Type  string `xml:"type,attr"`
			} `xml:"dataValidations>dataValidation"`
			Links []struct {
				Ref string `xml:"ref,attr"`
			} `xml:"hyperlinks>hyperlink"`
		}
		read(part, &ws)
		for _, v := range ws.Views {
			if v.Pane != nil {
				sd.pane = fmt.Sprintf("x%s y%s %s", v.Pane.X, v.Pane.Y, v.Pane.State)
			}
		}
		for _, c := range ws.Cols {
			w, _ := strconv.ParseFloat(c.Width, 64)
			sd.cols = append(sd.cols, fmt.Sprintf("%s-%s:%.0f%s", c.Min, c.Max, w, map[bool]string{true: " masquée"}[c.Hidden == "1"]))
		}
		for _, r := range ws.Rows {
			for _, c := range r.C {
				v := c.V
				switch c.T {
				case "s":
					if i, err := strconv.Atoi(c.V); err == nil && i < len(strs) {
						v = strs[i]
					}
				case "inlineStr":
					v = c.IS.T
				case "b":
					v = map[string]string{"1": "TRUE", "0": "FALSE"}[c.V]
				}
				code, st := styleOf(c.S)
				if v == "" && c.F == "" && code == "" && st == "" {
					continue
				}
				sd.cells[c.R] = xcellDump{v: v, f: c.F, fmt: code, st: st}
			}
		}
		for _, m := range ws.Merges {
			sd.merges = append(sd.merges, m.Ref)
		}
		for _, c := range ws.CF {
			for _, r := range c.Rules {
				sd.cf = append(sd.cf, c.Sqref+":"+r.Type+" "+r.Op)
			}
		}
		for _, d := range ws.DV {
			sd.dv = append(sd.dv, d.Sqref+":"+d.Type)
		}
		for _, l := range ws.Links {
			sd.links = append(sd.links, l.Ref)
		}
		for _, r := range rels(part) {
			switch {
			case strings.HasSuffix(r.Type, "/comments"):
				var cm struct {
					C []struct{} `xml:"commentList>comment"`
				}
				read(r.Target, &cm)
				sd.notes = len(cm.C)
			case strings.HasSuffix(r.Type, "/table"):
				var tb struct {
					Name  string `xml:"displayName,attr"`
					Ref   string `xml:"ref,attr"`
					Style struct {
						Name string `xml:"name,attr"`
					} `xml:"tableStyleInfo"`
				}
				read(r.Target, &tb)
				sd.tables = append(sd.tables, tb.Name+":"+tb.Ref+":"+tb.Style.Name)
			case strings.HasSuffix(r.Type, "/drawing"):
				for _, dr := range rels(r.Target) {
					switch {
					case strings.HasSuffix(dr.Type, "/chart"):
						sd.charts++
					case strings.HasSuffix(dr.Type, "/image"):
						sd.images++
					}
				}
			}
		}
		for _, l := range [][]string{sd.merges, sd.cols, sd.cf, sd.dv, sd.links, sd.tables} {
			sort.Strings(l)
		}
	}
	return out
}

// normFormula ramène une formule à une forme comparable : sans $, sans
// préfixe _xlfn., sans apostrophes autour des noms de feuille, sans espaces.
func normFormula(f string) string {
	f = strings.NewReplacer("$", "", "_xlfn.", "", "'", "", " ", "").Replace(f)
	return strings.ToUpper(f)
}

// sameNumber : deux valeurs écrites égales (nombres à 1e-9 près).
func sameValue(a, b string) bool {
	if a == b {
		return true
	}
	x, err1 := strconv.ParseFloat(a, 64)
	y, err2 := strconv.ParseFloat(b, 64)
	if err1 != nil || err2 != nil {
		return false
	}
	d := x - y
	return d < 1e-9 && d > -1e-9
}

// fidelity compare deux classeurs et range chaque différence par sujet :
// identique, réécrit (même sens, autre écriture) ou perdu.
type fidelity struct {
	same, rewritten, lost map[string][]string
}

func (f *fidelity) add(m map[string][]string, topic, what string) {
	m[topic] = append(m[topic], what)
}

func compareXLSX(a, b *xbookDump) *fidelity {
	fd := &fidelity{same: map[string][]string{}, rewritten: map[string][]string{}, lost: map[string][]string{}}
	for _, sa := range a.sheets {
		sb := b.sheet(sa.name)
		if sb == nil {
			fd.add(fd.lost, "feuilles", sa.name)
			continue
		}
		fd.add(fd.same, "feuilles", sa.name)
		refs := make([]string, 0, len(sa.cells))
		for r := range sa.cells {
			refs = append(refs, r)
		}
		sort.Strings(refs)
		for _, r := range refs {
			ca, cb := sa.cells[r], sb.cells[r]
			at := sa.name + "!" + r
			if ca.v != "" || cb.v != "" {
				if sameValue(ca.v, cb.v) {
					fd.add(fd.same, "valeurs", at)
				} else {
					fd.add(fd.lost, "valeurs", fmt.Sprintf("%s : %q → %q", at, ca.v, cb.v))
				}
			}
			switch {
			case ca.f == "":
			case cb.f == "":
				fd.add(fd.lost, "formules", fmt.Sprintf("%s : %s → valeur", at, ca.f))
			case ca.f == cb.f || normFormula(ca.f) == normFormula(cb.f):
				fd.add(fd.same, "formules", at)
			default:
				fd.add(fd.rewritten, "formules", fmt.Sprintf("%s : %s → %s", at, ca.f, cb.f))
			}
			if ca.fmt != cb.fmt {
				fd.add(fd.lost, "formats de nombre", fmt.Sprintf("%s : %q → %q", at, ca.fmt, cb.fmt))
			} else if ca.fmt != "" {
				fd.add(fd.same, "formats de nombre", at)
			}
			if ca.st != cb.st {
				fd.add(fd.lost, "styles", fmt.Sprintf("%s : %q → %q", at, ca.st, cb.st))
			} else if ca.st != "" {
				fd.add(fd.same, "styles", at)
			}
		}
		for r, cb := range sb.cells {
			if _, ok := sa.cells[r]; !ok && (cb.v != "" || cb.f != "") {
				fd.add(fd.lost, "valeurs", fmt.Sprintf("%s!%s : ajoutée %q", sa.name, r, cb.v))
			}
		}
		list := func(topic string, x, y []string) {
			if slices.Equal(x, y) {
				if len(x) > 0 {
					fd.add(fd.same, topic, sa.name+" "+strings.Join(x, ", "))
				}
				return
			}
			fd.add(fd.lost, topic, fmt.Sprintf("%s : %v → %v", sa.name, x, y))
		}
		list("fusions", sa.merges, sb.merges)
		list("largeurs de colonnes", sa.cols, sb.cols)
		list("volets figés", []string{sa.pane}, []string{sb.pane})
		list("mise en forme conditionnelle", sa.cf, sb.cf)
		list("validations", sa.dv, sb.dv)
		list("liens hypertexte", sa.links, sb.links)
		list("tableaux Excel", sa.tables, sb.tables)
		list("notes", []string{strconv.Itoa(sa.notes)}, []string{strconv.Itoa(sb.notes)})
		list("graphiques", []string{strconv.Itoa(sa.charts)}, []string{strconv.Itoa(sb.charts)})
		list("images", []string{strconv.Itoa(sa.images)}, []string{strconv.Itoa(sb.images)})
	}
	for n, v := range a.names {
		if b.names[n] == v {
			fd.add(fd.same, "noms définis", n)
		} else {
			fd.add(fd.lost, "noms définis", fmt.Sprintf("%s=%s → %q", n, v, b.names[n]))
		}
	}
	if a.pivots == b.pivots {
		fd.add(fd.same, "tableaux croisés", strconv.Itoa(a.pivots))
	} else {
		fd.add(fd.lost, "tableaux croisés", fmt.Sprintf("%d → %d", a.pivots, b.pivots))
	}
	return fd
}

// report écrit le tableau de fidélité (journal de l'essai).
func (f *fidelity) report() string {
	topics := map[string]bool{}
	for _, m := range []map[string][]string{f.same, f.rewritten, f.lost} {
		for k := range m {
			topics[k] = true
		}
	}
	var keys []string
	for k := range topics {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%-30s identiques %3d  réécrits %3d  perdus %3d\n", k, len(f.same[k]), len(f.rewritten[k]), len(f.lost[k]))
		for _, x := range f.rewritten[k] {
			fmt.Fprintf(&b, "    ~ %s\n", x)
		}
		for _, x := range f.lost[k] {
			fmt.Fprintf(&b, "    ✗ %s\n", x)
		}
	}
	return b.String()
}
