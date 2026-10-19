package transfer

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

func isLetter(r rune) bool { return unicode.IsLetter(r) }

// --- Lecture ----------------------------------------------------------------

// CSVOptions règle la lecture d'un CSV ; les valeurs nulles sont détectées.
type CSVOptions struct {
	Comma rune // séparateur ; 0 : détecté parmi ; , tabulation |
}

// ImportCSV ajoute au classeur une feuille nommée d'après le fichier, avec
// un tableau en haut à gauche : la première ligne donne les en-têtes, les
// types sont inférés sur toutes les valeurs. L'encodage (UTF-8, UTF-16 avec
// BOM, sinon Windows-1252) et le séparateur sont détectés.
func ImportCSV(ctx context.Context, m *canvas.Model, path string, opt CSVOptions) (canvas.ImportResult, error) {
	sheet, err := ReadCSV(ctx, path, opt)
	if err != nil {
		return canvas.ImportResult{}, err
	}
	taken, err := m.TableNames(ctx)
	if err != nil {
		return canvas.ImportResult{}, err
	}
	sheet.Tables[0].Name = tableName(sheet.Name, 0, 0, taken)
	return m.Import(ctx, "Importer "+filepath.Base(path), []canvas.ImportSheet{sheet})
}

// ReadCSV lit un CSV (premier passage : en-têtes, largeur, types) et
// prépare la feuille ; les valeurs sont relues au moment de l'import. Le
// nom du tableau reste à choisir.
func ReadCSV(ctx context.Context, path string, opt CSVOptions) (canvas.ImportSheet, error) {
	open := func() (*csv.Reader, io.Closer, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, nil, err
		}
		r, comma, err := decode(f)
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		if opt.Comma != 0 {
			comma = opt.Comma
		}
		cr := csv.NewReader(r)
		cr.Comma = comma
		cr.LazyQuotes = true
		cr.FieldsPerRecord = -1
		cr.ReuseRecord = true
		return cr, f, nil
	}
	cr, f, err := open()
	if err != nil {
		return canvas.ImportSheet{}, err
	}
	defer f.Close()
	var headers []string
	var types []grid.Type
	count := 0
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return canvas.ImportSheet{}, csvError(path, err)
		}
		if headers == nil {
			headers = append([]string{}, rec...)
			for i := range headers {
				headers[i] = strings.TrimPrefix(headers[i], "\ufeff")
			}
			continue
		}
		for len(types) < len(rec) {
			types = append(types, "")
		}
		for i, raw := range rec {
			types[i] = join(types[i], parseText(raw).t)
		}
		count++
		if count%8192 == 0 {
			if err := ctx.Err(); err != nil {
				return canvas.ImportSheet{}, err
			}
		}
	}
	if headers == nil {
		return canvas.ImportSheet{}, fmt.Errorf("%s : fichier vide", filepath.Base(path))
	}
	width := max(len(headers), len(types))
	for len(types) < width {
		types = append(types, "")
	}
	for i := range types {
		types[i] = final(types[i])
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	t := canvas.ImportTable{
		Headers: columnNames(headers, width),
		Types:   types,
		Count:   count,
		Rows: func(yield func([]any) error) error {
			cr, f, err := open()
			if err != nil {
				return err
			}
			defer f.Close()
			row := make([]any, width)
			first := true
			for {
				rec, err := cr.Read()
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return csvError(path, err)
				}
				if first {
					first = false
					continue
				}
				for i := range row {
					row[i] = nil
					if i < len(rec) {
						if v := parseText(rec[i]); v.t != "" {
							row[i] = castTo(v, types[i])
						}
					}
				}
				if err := yield(row); err != nil {
					return err
				}
			}
		},
	}
	return canvas.ImportSheet{Name: name, Tables: []canvas.ImportTable{t}}, nil
}

// castTo convertit une valeur de CSV au type de sa colonne ; une colonne de
// texte garde la saisie telle quelle.
func castTo(v value, to grid.Type) any {
	if to == grid.Text && v.t != grid.Text && v.t != "" {
		return display(v)
	}
	return cast(v, to)
}

func csvError(path string, err error) error {
	var pe *csv.ParseError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s, ligne %d : %v", filepath.Base(path), pe.Line, pe.Err)
	}
	return err
}

// decode détecte l'encodage et le séparateur d'un CSV et renvoie le texte
// en UTF-8.
func decode(f io.Reader) (io.Reader, rune, error) {
	br := bufio.NewReaderSize(f, 64<<10)
	sample, err := br.Peek(64 << 10)
	if err != nil && err != io.EOF && !errors.Is(err, bufio.ErrBufferFull) {
		return nil, 0, err
	}
	var r io.Reader = br
	switch {
	case len(sample) >= 3 && sample[0] == 0xEF && sample[1] == 0xBB && sample[2] == 0xBF:
		br.Discard(3)
		sample = sample[3:]
	case len(sample) >= 2 && (sample[0] == 0xFF && sample[1] == 0xFE || sample[0] == 0xFE && sample[1] == 0xFF):
		big := sample[0] == 0xFE
		br.Discard(2)
		r = &utf16Reader{r: br, big: big}
		sample = decodeUTF16(sample[2:], big)
	case !validPrefix(sample):
		r = &cp1252Reader{r: br}
		sample = []byte(fromCP1252(sample))
	}
	return r, sniffComma(string(sample)), nil
}

// validPrefix vérifie l'UTF-8 d'un échantillon dont le dernier caractère
// peut être coupé.
func validPrefix(b []byte) bool {
	for i := 0; i < 3 && len(b) > 0 && !utf8.Valid(b); i++ {
		b = b[:len(b)-1]
	}
	return utf8.Valid(b)
}

// sniffComma choisit le séparateur qui découpe les premières lignes en un
// même nombre de champs (hors guillemets).
func sniffComma(sample string) rune {
	lines := splitLines(sample, 30)
	best, bestScore := ';', 0.0
	for _, c := range []rune{';', ',', '\t', '|'} {
		freq := map[int]int{}
		for _, l := range lines {
			freq[countOutsideQuotes(l, c)]++
		}
		mode, n := 0, 0
		for k, v := range freq {
			if k > 0 && (v > n || v == n && k > mode) {
				mode, n = k, v
			}
		}
		if mode == 0 {
			continue
		}
		score := float64(n)/float64(len(lines))*1000 + float64(mode)
		if score > bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

// splitLines découpe en lignes logiques (un saut de ligne entre guillemets
// ne compte pas) ; la dernière, peut-être coupée, est écartée.
func splitLines(s string, limit int) []string {
	var out []string
	in := false
	start := 0
	for i, r := range s {
		switch {
		case r == '"':
			in = !in
		case r == '\n' && !in:
			if l := strings.TrimRight(s[start:i], "\r"); l != "" {
				out = append(out, l)
			}
			start = i + 1
			if len(out) == limit {
				return out
			}
		}
	}
	if len(out) == 0 && start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func countOutsideQuotes(l string, c rune) int {
	n := 0
	in := false
	for _, r := range l {
		switch {
		case r == '"':
			in = !in
		case r == c && !in:
			n++
		}
	}
	return n
}

// cp1252 donne les caractères 0x80 à 0x9F de Windows-1252 (le reste est
// identique à Latin-1).
var cp1252 = [32]rune{
	'€', 0x81, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0x8D, 'Ž', 0x8F,
	0x90, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0x9D, 'ž', 'Ÿ',
}

func cp1252Rune(b byte) rune {
	if b >= 0x80 && b < 0xA0 {
		return cp1252[b-0x80]
	}
	return rune(b)
}

func fromCP1252(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		sb.WriteRune(cp1252Rune(c))
	}
	return sb.String()
}

type cp1252Reader struct {
	r   *bufio.Reader
	buf []byte
}

func (c *cp1252Reader) Read(p []byte) (int, error) {
	for len(c.buf) < len(p) {
		b, err := c.r.ReadByte()
		if err != nil {
			if len(c.buf) > 0 {
				break
			}
			return 0, err
		}
		c.buf = utf8.AppendRune(c.buf, cp1252Rune(b))
	}
	n := copy(p, c.buf)
	c.buf = c.buf[n:]
	return n, nil
}

func decodeUTF16(b []byte, big bool) []byte {
	units := make([]uint16, len(b)/2)
	for i := range units {
		if big {
			units[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		} else {
			units[i] = uint16(b[2*i+1])<<8 | uint16(b[2*i])
		}
	}
	return []byte(string(utf16.Decode(units)))
}

type utf16Reader struct {
	r   *bufio.Reader
	big bool
	buf []byte
}

func (u *utf16Reader) unit() (uint16, error) {
	var b [2]byte
	if _, err := io.ReadFull(u.r, b[:]); err != nil {
		if err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
		return 0, err
	}
	if u.big {
		return uint16(b[0])<<8 | uint16(b[1]), nil
	}
	return uint16(b[1])<<8 | uint16(b[0]), nil
}

func (u *utf16Reader) Read(p []byte) (int, error) {
	for len(u.buf) < len(p) {
		a, err := u.unit()
		if err != nil {
			if len(u.buf) > 0 {
				break
			}
			return 0, err
		}
		r := rune(a)
		if utf16.IsSurrogate(r) {
			b, err := u.unit()
			if err != nil {
				r = utf8.RuneError
			} else {
				r = utf16.DecodeRune(r, rune(b))
			}
		}
		u.buf = utf8.AppendRune(u.buf, r)
	}
	n := copy(p, u.buf)
	u.buf = u.buf[n:]
	return n, nil
}

// --- Écriture ---------------------------------------------------------------

// CSVStyle choisit les conventions d'un CSV écrit.
type CSVStyle int

const (
	// ExcelFR : ce qu'Excel en français ouvre d'un double clic : séparateur
	// ;, virgule décimale, dates jj/mm/aaaa, VRAI/FAUX, UTF-8 avec BOM.
	ExcelFR CSVStyle = iota
	// Standard : RFC 4180 : séparateur virgule, point décimal, dates
	// aaaa-mm-jj, true/false, UTF-8 sans BOM.
	Standard
)

// ExportCSV écrit un tableau en CSV : une ligne d'en-têtes, puis les
// lignes dans l'ordre d'affichage. Les colonnes calculées donnent leur
// valeur, les saisies en erreur leur texte d'origine.
func ExportCSV(ctx context.Context, m *canvas.Model, table string, w io.Writer, style CSVStyle) error {
	bw := bufio.NewWriterSize(w, 64<<10)
	if style == ExcelFR {
		bw.WriteString("\ufeff")
	}
	cw := csv.NewWriter(bw)
	cw.UseCRLF = true
	if style == ExcelFR {
		cw.Comma = ';'
	}
	var cols []canvas.ExportColumn
	var rec []string
	err := m.ExportTable(ctx, table, func(c []canvas.ExportColumn) error {
		cols = c
		rec = make([]string, len(c))
		for i, col := range c {
			rec[i] = col.Name
		}
		return cw.Write(rec)
	}, func(row []any) error {
		for i, v := range row {
			rec[i] = exportText(v, cols[i].Type, style)
		}
		return cw.Write(rec)
	})
	if err != nil {
		return err
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	return bw.Flush()
}

// ExportCSVFile écrit le CSV dans un fichier temporaire à côté de la cible,
// puis le renomme : un export interrompu ne laisse pas un fichier tronqué.
func ExportCSVFile(ctx context.Context, m *canvas.Model, table, path string, style CSVStyle) error {
	return writeAtomic(path, func(w io.Writer) error { return ExportCSV(ctx, m, table, w, style) })
}

func writeAtomic(path string, fn func(io.Writer) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := fn(tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// exportText écrit une valeur lue par canvas.Model.ExportTable.
func exportText(v any, t grid.Type, style CSVStyle) string {
	num := func(f float64) string {
		if style == ExcelFR {
			return formula.FormatNumber(f)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	boolean := func(b bool) string {
		switch {
		case style == Standard && b:
			return "true"
		case style == Standard:
			return "false"
		case b:
			return "VRAI"
		}
		return "FAUX"
	}
	switch v := v.(type) {
	case nil:
		return ""
	case canvas.Invalid:
		return string(v)
	case formula.Value:
		switch v.Kind {
		case formula.Number:
			return num(v.Num)
		case formula.Bool:
			return boolean(v.Bool)
		case formula.Empty:
			return ""
		}
		return v.String()
	case int64:
		if t == grid.Boolean {
			return boolean(v != 0)
		}
		return strconv.FormatInt(v, 10)
	case float64:
		return num(v)
	case string:
		if t == grid.Date && style == ExcelFR {
			if d, err := time.Parse("2006-01-02", v); err == nil {
				return d.Format("02/01/2006")
			}
		}
		return v
	}
	return fmt.Sprint(v)
}
