package transfer

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
)

// Lecture d'un classeur .xlsx (Office Open XML) avec la seule bibliothèque
// standard : le ZIP, le classeur (feuilles), les chaînes partagées, les
// styles (pour reconnaître les dates) et les feuilles, lues en flux.

type xlsxSheet struct {
	name string
	file *zip.File
}

type xlsxFile struct {
	zr       *zip.ReadCloser
	sheets   []xlsxSheet
	sst      []string
	dates    []dateKind // par style de cellule (attribut s)
	date1904 bool
	// extras : par feuille lue jusqu'au bout, règles de mise en forme
	// conditionnelle et validations (non importées).
	mu     sync.Mutex
	extras map[string][2]int
}

type dateKind uint8

const (
	notDate dateKind = iota
	dateOnly
	timeOnly
	dateTime
)

// ErrNotXLSX signale un fichier qui n'est pas un classeur .xlsx lisible.
var ErrNotXLSX = errors.New("ce fichier n'est pas un classeur Excel .xlsx")

func openXLSX(name string) (*xlsxFile, error) {
	zr, err := zip.OpenReader(name)
	if err != nil {
		return nil, fmt.Errorf("%w (%v)", ErrNotXLSX, err)
	}
	x := &xlsxFile{zr: zr}
	if err := x.load(); err != nil {
		zr.Close()
		return nil, err
	}
	return x, nil
}

func (x *xlsxFile) Close() error { return x.zr.Close() }

func (x *xlsxFile) find(name string) *zip.File {
	name = strings.TrimPrefix(name, "/")
	for _, f := range x.zr.File {
		if strings.EqualFold(f.Name, name) {
			return f
		}
	}
	return nil
}

func (x *xlsxFile) decode(f *zip.File, v any) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	return xml.NewDecoder(r).Decode(v)
}

type xmlRels struct {
	Rels []struct {
		ID     string `xml:"Id,attr"`
		Type   string `xml:"Type,attr"`
		Target string `xml:"Target,attr"`
	} `xml:"Relationship"`
}

// rels lit les relations d'une partie (dir/_rels/base.rels) ; les cibles
// sont rendues absolues dans le ZIP.
func (x *xlsxFile) rels(part string) (map[string]string, map[string]string, error) {
	dir, base := path.Split(part)
	f := x.find(dir + "_rels/" + base + ".rels")
	byID, byType := map[string]string{}, map[string]string{}
	if f == nil {
		return byID, byType, nil
	}
	var rs xmlRels
	if err := x.decode(f, &rs); err != nil {
		return nil, nil, err
	}
	for _, r := range rs.Rels {
		t := r.Target
		if strings.HasPrefix(t, "/") {
			t = strings.TrimPrefix(t, "/")
		} else {
			t = path.Join(dir, t)
		}
		byID[r.ID] = t
		byType[path.Base(r.Type)] = t
	}
	return byID, byType, nil
}

func (x *xlsxFile) load() error {
	_, top, err := x.rels("")
	if err != nil {
		return err
	}
	wbPath := top["officeDocument"]
	if wbPath == "" {
		wbPath = "xl/workbook.xml"
	}
	wbFile := x.find(wbPath)
	if wbFile == nil {
		return ErrNotXLSX
	}
	var wb struct {
		Pr struct {
			Date1904 string `xml:"date1904,attr"`
		} `xml:"workbookPr"`
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"id,attr"` // r:id
		} `xml:"sheets>sheet"`
	}
	if err := x.decode(wbFile, &wb); err != nil {
		return fmt.Errorf("%w (classeur illisible : %v)", ErrNotXLSX, err)
	}
	x.date1904 = wb.Pr.Date1904 == "1" || wb.Pr.Date1904 == "true"
	byID, byType, err := x.rels(wbPath)
	if err != nil {
		return err
	}
	for _, s := range wb.Sheets {
		if f := x.find(byID[s.ID]); f != nil {
			x.sheets = append(x.sheets, xlsxSheet{name: s.Name, file: f})
		}
	}
	if p := byType["sharedStrings"]; p != "" {
		if f := x.find(p); f != nil {
			if x.sst, err = readSST(f); err != nil {
				return err
			}
		}
	}
	if p := byType["styles"]; p != "" {
		if f := x.find(p); f != nil {
			if x.dates, err = x.readStyles(f); err != nil {
				return err
			}
		}
	}
	return nil
}

// readSST lit les chaînes partagées, texte enrichi compris (les indications
// phonétiques rPh sont ignorées).
func readSST(f *zip.File) ([]string, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	d := xml.NewDecoder(r)
	var out []string
	var b strings.Builder
	inSI, inT, skip := false, false, 0
	for {
		tok, err := d.RawToken()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "si":
				inSI = true
				b.Reset()
			case "rPh":
				skip++
			case "t":
				inT = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "si":
				inSI = false
				out = append(out, b.String())
			case "rPh":
				skip--
			case "t":
				inT = false
			}
		case xml.CharData:
			if inSI && inT && skip == 0 {
				b.Write(t)
			}
		}
	}
}

// readStyles repère les styles de cellule dont le format est une date ou
// une heure.
func (x *xlsxFile) readStyles(f *zip.File) ([]dateKind, error) {
	var st struct {
		NumFmts []struct {
			ID   int    `xml:"numFmtId,attr"`
			Code string `xml:"formatCode,attr"`
		} `xml:"numFmts>numFmt"`
		Xfs []struct {
			NumFmt int `xml:"numFmtId,attr"`
		} `xml:"cellXfs>xf"`
	}
	if err := x.decode(f, &st); err != nil {
		return nil, err
	}
	custom := map[int]string{}
	for _, n := range st.NumFmts {
		custom[n.ID] = n.Code
	}
	out := make([]dateKind, len(st.Xfs))
	for i, xf := range st.Xfs {
		if code, ok := custom[xf.NumFmt]; ok {
			out[i] = formatKind(code)
		} else {
			out[i] = builtinKind(xf.NumFmt)
		}
	}
	return out, nil
}

func builtinKind(id int) dateKind {
	switch {
	case id >= 14 && id <= 17, id >= 27 && id <= 31, id >= 34 && id <= 36, id >= 50 && id <= 58:
		return dateOnly
	case id >= 18 && id <= 21, id >= 32 && id <= 33, id >= 45 && id <= 47:
		return timeOnly
	case id == 22:
		return dateTime
	}
	return notDate
}

// formatKind lit un format personnalisé : jours, mois et années en font une
// date, heures et secondes une heure (texte entre guillemets, caractères
// échappés et couleurs [Rouge] ignorés).
func formatKind(code string) dateKind {
	if i := strings.IndexByte(code, ';'); i >= 0 {
		code = code[:i] // la section des nombres positifs suffit
	}
	var b strings.Builder
	for i := 0; i < len(code); i++ {
		c := code[i]
		switch c {
		case '"':
			j := strings.IndexByte(code[i+1:], '"')
			if j < 0 {
				i = len(code)
			} else {
				i += j + 1
			}
		case '\\', '_', '*':
			i++
		case '[':
			j := strings.IndexByte(code[i:], ']')
			if j < 0 {
				i = len(code)
				break
			}
			inner := strings.ToLower(code[i+1 : i+j])
			if inner != "" && strings.Trim(inner, "hms") == "" {
				b.WriteString(inner) // durée écoulée [h]:mm
			}
			i += j
		default:
			b.WriteByte(c | 0x20)
		}
	}
	s := b.String()
	tm := strings.ContainsAny(s, "hs")
	date := strings.ContainsAny(s, "dy") || strings.Contains(s, "mmm") && !tm
	switch {
	case date && tm:
		return dateTime
	case date:
		return dateOnly
	case tm:
		return timeOnly
	}
	return notDate
}

// rawCell est une cellule telle qu'écrite dans la feuille.
type rawCell struct {
	row, col int    // à partir de 0
	t        string // type : s, str, inlineStr, b, e, d, n
	s        int    // style
	v        string
	hasV     bool
	is       string
	f        string // formule (sans =)
	hasF     bool
	ft       string // shared, array
	si       string // groupe de formule partagée
}

func (c *rawCell) empty(sst []string) bool {
	if c.hasF {
		return false
	}
	switch c.t {
	case "inlineStr":
		return c.is == ""
	case "s":
		i, err := strconv.Atoi(c.v)
		return err != nil || i < 0 || i >= len(sst) || sst[i] == ""
	}
	return c.v == ""
}

// scan parcourt les cellules d'une feuille dans l'ordre du fichier (lignes
// croissantes, colonnes croissantes).
func (x *xlsxFile) scan(ctx context.Context, sh xlsxSheet, fn func(*rawCell) error) error {
	r, err := sh.file.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	d := newXMLScanner(r)
	var c rawCell
	row, col := -1, -1
	var buf strings.Builder
	capture, inIS, skip := false, false, 0
	n, rules, lists := 0, 0, 0
	for {
		if err := d.next(); err != nil {
			if err == io.EOF {
				x.mu.Lock()
				if x.extras == nil {
					x.extras = map[string][2]int{}
				}
				x.extras[sh.name] = [2]int{rules, lists}
				x.mu.Unlock()
				return nil
			}
			return fmt.Errorf("feuille %s : %w", sh.name, err)
		}
		switch d.kind {
		case xmlStart:
			switch string(d.name) {
			case "row":
				row++
				col = -1
				if v, err := strconv.Atoi(string(d.attr("r"))); err == nil {
					row = v - 1
				}
			case "c":
				col++
				c = rawCell{row: row, col: col, t: string(d.attr("t"))}
				if r, cc, ok := parseCellRef(string(d.attr("r"))); ok {
					c.row, c.col = r, cc
					row, col = r, cc
				}
				if s := d.attr("s"); s != nil {
					c.s, _ = strconv.Atoi(string(s))
				}
			case "v":
				buf.Reset()
				capture = true
			case "f":
				c.hasF = true
				c.ft, c.si = string(d.attr("t")), string(d.attr("si"))
				buf.Reset()
				capture = true
			case "is":
				inIS = true
				buf.Reset()
			case "rPh":
				skip++
			case "t":
				capture = inIS && skip == 0
			case "cfRule":
				rules++
			case "dataValidation":
				lists++
			}
		case xmlEnd:
			switch string(d.name) {
			case "v":
				c.v, c.hasV, capture = buf.String(), true, false
			case "f":
				c.f, capture = buf.String(), false
			case "t":
				capture = false
			case "is":
				inIS = false
				c.is = buf.String()
			case "rPh":
				skip--
			case "c":
				if n++; n%8192 == 0 {
					if err := ctx.Err(); err != nil {
						return err
					}
				}
				if err := fn(&c); err != nil {
					return err
				}
			}
		case xmlText:
			if capture {
				buf.Write(d.text)
			}
		}
	}
}

// parseCellRef lit « B12 » (colonne 1, ligne 11), « $B$12 » aussi.
func parseCellRef(s string) (row, col int, ok bool) {
	i := 0
	if i < len(s) && s[i] == '$' {
		i++
	}
	c := 0
	j := i
	for i < len(s) && (s[i]|0x20) >= 'a' && (s[i]|0x20) <= 'z' {
		c = c*26 + int(s[i]|0x20-'a'+1)
		i++
	}
	if i == j || i-j > 3 || c > 16384 {
		return 0, 0, false
	}
	if i < len(s) && s[i] == '$' {
		i++
	}
	r, err := strconv.Atoi(s[i:])
	if err != nil || r < 1 || r > 1048576 || strings.ContainsAny(s[i:], "+-") {
		return 0, 0, false
	}
	return r - 1, c - 1, true
}

// value donne la valeur d'une cellule (le résultat mis en cache pour une
// formule). Une erreur (#N/A…) est un texte marqué err.
func (x *xlsxFile) value(c *rawCell) (v value, isErr bool) {
	switch c.t {
	case "s":
		i, err := strconv.Atoi(c.v)
		if err != nil || i < 0 || i >= len(x.sst) || x.sst[i] == "" {
			return value{}, false
		}
		return textValue(x.sst[i]), false
	case "inlineStr":
		if c.is == "" {
			return value{}, false
		}
		return textValue(c.is), false
	case "str":
		if c.v == "" {
			return value{}, false
		}
		return textValue(c.v), false
	case "b":
		if !c.hasV {
			return value{}, false
		}
		if c.v == "1" || c.v == "true" {
			return value{t: grid.Boolean, i: 1}, false
		}
		return value{t: grid.Boolean, i: 0}, false
	case "e":
		return textValue(c.v), true
	case "d":
		if d, err := time.Parse("2006-01-02T15:04:05", strings.TrimSuffix(c.v, "Z")); err == nil {
			return x.dateValue(d, dateTime), false
		}
		if d, err := time.Parse("2006-01-02", c.v); err == nil {
			return value{t: grid.Date, s: d.Format("2006-01-02")}, false
		}
		return textValue(c.v), false
	}
	if c.v == "" {
		return value{}, false
	}
	f, err := strconv.ParseFloat(c.v, 64)
	if err != nil {
		return textValue(c.v), false
	}
	if c.s >= 0 && c.s < len(x.dates) && x.dates[c.s] != notDate && f >= 0 && f < 2958466 {
		return x.dateValue(x.serial(f), x.dates[c.s]), false
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return value{t: grid.Integer, i: int64(f)}, false
	}
	return value{t: grid.Decimal, f: f}, false
}

// serial convertit un numéro de série Excel en date et heure.
func (x *xlsxFile) serial(f float64) time.Time {
	days := math.Floor(f)
	secs := math.Round((f - days) * 86400)
	var base time.Time
	switch {
	case x.date1904:
		base = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	case days < 60:
		// Excel compte un 29 février 1900 qui n'a pas existé.
		base = time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	default:
		base = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	}
	return base.AddDate(0, 0, int(days)).Add(time.Duration(secs) * time.Second)
}

// dateValue : une date sans heure devient une date ; une heure, ou une date
// avec heure, reste du texte (Lattis n'a pas encore de type heure).
func (x *xlsxFile) dateValue(d time.Time, k dateKind) value {
	midnight := d.Hour() == 0 && d.Minute() == 0 && d.Second() == 0
	switch {
	case k == timeOnly:
		return textValue(d.Format("15:04:05"))
	case midnight:
		return value{t: grid.Date, s: d.Format("2006-01-02")}
	}
	return textValue(d.Format("02/01/2006 15:04:05"))
}
