package transfer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/work"
)

// Export SQL (Fichier › Exporter › SQL) : un script qui recrée les tableaux
// du classeur dans une base quelconque, CREATE TABLE puis INSERT par lots.
// Tableaux de Lattis (ceux chargés par une requête compris) et tableaux
// croisés, en valeurs ; les cellules libres ne sont pas exportées (pas de
// forme de table). Chaque tableau est lu deux fois, sans rien garder en
// mémoire : une passe mesure les colonnes (longueur des textes, étendue des
// entiers, décimales, dates avec heure), l'autre écrit les lignes.
//
// Types : entier → INTEGER (BIGINT au-delà de 32 bits) ; décimal →
// NUMERIC(p,s) quand les valeurs s'écrivent avec au plus 10 décimales
// (montants, mesures : la base les garde exactes), DOUBLE PRECISION sinon
// (résultats de calculs à 17 chiffres) ; texte → VARCHAR(n), n arrondi au
// palier au-dessus de la plus longue valeur (10, 20, 50, 100, 255…, 4000),
// TEXT au-delà ; date → DATE, ou TIMESTAMP si une valeur a une heure ;
// booléen → BOOLEAN. La colonne clé donne la PRIMARY KEY ; Lattis ne
// connaît ni autre contrainte d'unicité, ni NOT NULL, ni clé étrangère
// sûre : il n'en écrit pas. Une colonne calculée donne ses valeurs, sa
// formule en commentaire.
//
// Le dialecte standard s'en tient au SQL de la norme (guillemets doubles,
// DATE '…', TRUE/FALSE, pas de transaction) ; les autres l'ajustent à leur
// base et encadrent le script d'une transaction.

// SQLDialect : base visée par le script.
type SQLDialect string

const (
	SQLStandard SQLDialect = "standard"
	SQLPostgres SQLDialect = "postgresql"
	SQLMySQL    SQLDialect = "mysql"
	SQLServer   SQLDialect = "sqlserver"
	SQLSQLite   SQLDialect = "sqlite"
)

// SQLOptions : dialecte et portée de l'export. Sheet : une feuille (0 : tout
// le classeur) ; Table : un seul tableau (ou tableau croisé) par son nom.
type SQLOptions struct {
	Dialect string `json:"dialect"` // SQLDialect ; "" : standard
	Sheet   int64  `json:"sheet"`
	Table   string `json:"table"`
	// Source : nom du classeur, cité en tête du script.
	Source string `json:"source"`
	// Batch : lignes par INSERT (500 par défaut).
	Batch int `json:"-"`
	// Now : date du script (essais) ; zéro : maintenant.
	Now time.Time `json:"-"`
}

// SQLReport dit ce que contient le script : tableaux, lignes, et valeurs
// écrites NULL faute de mieux (saisies hors type, erreurs de formule).
type SQLReport struct {
	Tables int   `json:"tables"`
	Rows   int64 `json:"rows"`
	Nulls  int64 `json:"nulls"`
}

// ExportSQLFile écrit le script dans un fichier (via un fichier temporaire).
func ExportSQLFile(ctx context.Context, m *canvas.Model, path string, o SQLOptions) (rep SQLReport, err error) {
	err = writeAtomic(path, func(w io.Writer) error {
		var e error
		rep, e = ExportSQL(ctx, m, w, o)
		return e
	})
	return rep, err
}

// ExportSQL écrit le script SQL des tableaux du classeur.
func ExportSQL(ctx context.Context, m *canvas.Model, w io.Writer, o SQLOptions) (SQLReport, error) {
	if o.Dialect == "" {
		o.Dialect = string(SQLStandard)
	}
	d, ok := sqlDialects[SQLDialect(o.Dialect)]
	if !ok {
		return SQLReport{}, msg.New("transfer.sqlDialect", msg.Args{"name": o.Dialect})
	}
	if o.Batch <= 0 {
		o.Batch = 500
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	work.Op(ctx, "exportSql", "")
	srcs, err := sqlSources(ctx, m, o)
	if err != nil {
		return SQLReport{}, err
	}
	var total int64
	for _, s := range srcs {
		total += 2 * s.total
	}
	work.Loop(ctx, "export", total)
	x := &sqlWriter{w: bufio.NewWriterSize(w, 256<<10), d: d, ctx: ctx, total: total, batch: o.Batch, l: msg.Current()}
	x.comment(msg.Format(x.l, "transfer.sqlGenerated", msg.Args{"date": o.Now.Format("2006-01-02 15:04")}))
	if o.Source != "" {
		x.comment(msg.Format(x.l, "transfer.sqlWorkbook", msg.Args{"name": o.Source}))
	}
	x.comment(msg.Format(x.l, "transfer.sqlDialectLine", msg.Args{"name": d.label}))
	for _, p := range d.preamble {
		x.line(p)
	}
	if d.begin != "" {
		x.line(d.begin)
	}
	used := map[string]bool{}
	for _, s := range srcs {
		if err := x.source(s, used); err != nil {
			return SQLReport{}, err
		}
	}
	if d.commit != "" {
		x.line("")
		x.line(d.commit)
	}
	if err := x.w.Flush(); err != nil {
		return SQLReport{}, err
	}
	return SQLReport{Tables: len(srcs), Rows: x.rows, Nulls: x.nulls}, nil
}

// sqlSource est un tableau à écrire : nom, colonnes et lecture des lignes.
type sqlSource struct {
	name  string
	sheet string
	pivot bool
	total int64 // lignes (avancement)
	// cols : colonnes ; celles d'un tableau sont posées à la première
	// lecture (en-tête de canvas.Model.ExportTable).
	cols []*sqlCol
	read func(ctx context.Context, each func([]sval) error) error
}

type sqlCol struct {
	name, ident string
	typ         grid.Type // type de Lattis ; "" : déduit des valeurs
	formula     string    // colonne calculée
	key         bool
	kind        byte // genre écrit (voir sval), fixé après la première passe
	st          sqlStats
}

// sqlStats : mesures d'une colonne (première passe).
type sqlStats struct {
	seen       uint8 // genres de valeurs vus (bits de sqlKinds)
	nulls      int64
	minI, maxI int64
	intDigits  int
	scale      int
	maxLen     int // en unités UTF-16 (NVARCHAR de SQL Server)
}

// sval est une valeur à écrire. k : 'i' entier, 'f' décimal, 'b' booléen
// (i : 0 ou 1) ; s : texte affiché d'un nombre ou d'un booléen de tableau
// croisé ; 'd' date (s : aaaa-mm-jj), 't' date et heure (s :
// aaaa-mm-jj hh:mm:ss), 's' texte, 'x' valeur perdue (hors type, erreur),
// 0 vide.
type sval struct {
	k byte
	i int64
	f float64
	s string
}

const sqlKinds = "ifbdts"

func kindBit(k byte) uint8 { return 1 << strings.IndexByte(sqlKinds, k) }

// sqlSources liste les tableaux et tableaux croisés de la portée choisie.
func sqlSources(ctx context.Context, m *canvas.Model, o SQLOptions) ([]*sqlSource, error) {
	first, err := m.Layout(ctx, 0)
	if err != nil {
		return nil, err
	}
	var out []*sqlSource
	for _, sh := range first.Sheets {
		if o.Sheet != 0 && sh.ID != o.Sheet {
			continue
		}
		l, err := m.Layout(ctx, sh.ID)
		if err != nil {
			return nil, err
		}
		for _, tb := range l.Tables {
			if o.Table == "" || strings.EqualFold(tb.Name, o.Table) {
				out = append(out, tableSource(m, sh.Name, tb))
			}
		}
		for _, p := range l.Pivots {
			if o.Table != "" && !strings.EqualFold(p.Config.Name, o.Table) {
				continue
			}
			if s := pivotSource(sh.Name, p); s != nil {
				out = append(out, s)
			}
		}
	}
	if o.Table != "" && len(out) == 0 {
		return nil, msg.New("transfer.sqlNoTable", msg.Args{"table": o.Table})
	}
	return out, nil
}

// tableSource : un tableau de Lattis, lu par canvas.Model.ExportTable.
func tableSource(m *canvas.Model, sheet string, tb canvas.TableInfo) *sqlSource {
	s := &sqlSource{name: tb.Name, sheet: sheet, total: int64(tb.Total)}
	formulas := map[string]string{}
	for _, c := range tb.Columns {
		if c.Formula != "" {
			formulas[c.Name] = c.Formula
		}
	}
	keyName := ""
	if tb.Key >= 0 && tb.Key < len(tb.Columns) {
		keyName = tb.Columns[tb.Key].Name
	}
	s.read = func(ctx context.Context, each func([]sval) error) error {
		var cols []canvas.ExportColumn
		var vals []sval
		return m.ExportTable(ctx, tb.Name, func(cs []canvas.ExportColumn) error {
			cols = cs
			vals = make([]sval, len(cs))
			if s.cols == nil {
				s.cols = make([]*sqlCol, len(cs))
				for i, c := range cs {
					col := &sqlCol{name: c.Name, typ: c.Type, key: c.Name == keyName && !c.Computed}
					if c.Computed {
						col.typ, col.formula = "", formulas[c.Name]
					}
					s.cols[i] = col
				}
			} else if len(s.cols) != len(cs) {
				return msg.New("export.tableChanged", msg.Args{"table": tb.Name})
			}
			return nil
		}, func(row []any) error {
			for i, v := range row {
				vals[i] = tableSval(v, cols[i].Type, cols[i].Computed)
			}
			return each(vals)
		})
	}
	return s
}

// pivotSource : un tableau croisé, en valeurs. Arborescence (plusieurs
// champs en lignes) : une colonne par champ, les lignes de détail seules ;
// sinon les lignes affichées, sans celle du total général.
func pivotSource(sheet string, p canvas.PivotInfo) *sqlSource {
	if len(p.Cells) < 1 || p.Error != "" || p.Pending {
		return nil
	}
	head := p.Cells[0]
	tree := len(p.Rows) > 0
	var names []string
	var data [][]string
	if tree {
		depth := 0
		for _, r := range p.Rows {
			if !r.Group && !r.Subtotal && !r.Total {
				depth = max(depth, len(r.Path))
			}
		}
		for i := range depth {
			if i < len(p.Config.RowFields) {
				names = append(names, p.Config.RowFields[i])
			} else {
				names = append(names, fmt.Sprintf("%s %d", head[0], i+1))
			}
		}
		if len(head) > 1 {
			names = append(names, head[1:]...)
		}
		for i, row := range p.Cells[1:] {
			if i >= len(p.Rows) {
				break
			}
			r := p.Rows[i]
			if r.Group || r.Subtotal || r.Total || len(r.Path) != depth {
				continue
			}
			line := append(append([]string{}, r.Path...), row[min(1, len(row)):]...)
			data = append(data, line)
		}
	} else {
		names = head
		rows := p.Cells[1:]
		if !p.Config.NoGrandTotal && len(rows) > 0 {
			rows = rows[:len(rows)-1] // total général
		}
		data = rows
	}
	s := &sqlSource{name: p.Config.Name, sheet: sheet, pivot: true, total: int64(len(data))}
	for _, n := range names {
		s.cols = append(s.cols, &sqlCol{name: n})
	}
	s.read = func(ctx context.Context, each func([]sval) error) error {
		vals := make([]sval, len(names))
		for _, row := range data {
			for j := range vals {
				vals[j] = sval{}
				if j < len(row) {
					vals[j] = textSval(row[j])
				}
			}
			if err := each(vals); err != nil {
				return err
			}
		}
		return nil
	}
	return s
}

// tableSval convertit une valeur lue par canvas.Model.ExportTable.
func tableSval(v any, t grid.Type, computed bool) sval {
	switch v := v.(type) {
	case nil:
		return sval{}
	case int64:
		if t == grid.Boolean && !computed {
			return sval{k: 'b', i: v}
		}
		return sval{k: 'i', i: v}
	case float64:
		return numSval(v, computed)
	case string:
		if t == grid.Date && !computed {
			return dateSval(v)
		}
		return sval{k: 's', s: v}
	case canvas.Invalid:
		return sval{k: 'x'}
	case formula.Value:
		switch v.Kind {
		case formula.Number:
			return numSval(v.Num, true)
		case formula.Date:
			if math.IsNaN(v.Num) || v.Num < 0 {
				return sval{k: 'x'}
			}
			return dateSval(formula.ISODate(v.Num))
		case formula.Bool:
			if v.Bool {
				return sval{k: 'b', i: 1}
			}
			return sval{k: 'b'}
		case formula.Text:
			return sval{k: 's', s: v.Str}
		case formula.Error:
			return sval{k: 'x'}
		}
		return sval{}
	}
	return sval{k: 's', s: fmt.Sprint(v)}
}

// numSval : un décimal ; un résultat de formule entier s'écrit en entier.
func numSval(f float64, integral bool) sval {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return sval{k: 'x'}
	}
	if integral && f == math.Trunc(f) && math.Abs(f) < 1<<53 {
		return sval{k: 'i', i: int64(f)}
	}
	return sval{k: 'f', f: f}
}

// dateSval lit une date stockée (grid.StoreDate) : aaaa-mm-jj, suivie de
// l'heure.
func dateSval(s string) sval {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return sval{k: 'd', s: t.Format("2006-01-02")}
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return sval{k: 't', s: t.Format("2006-01-02 15:04:05")}
		}
	}
	return sval{k: 'x'}
}

// textSval : une cellule de tableau croisé (texte affiché).
func textSval(s string) sval {
	if s == "" {
		return sval{}
	}
	if _, ok := excelErrors[s]; ok {
		return sval{k: 'x'}
	}
	// Nombre et booléen gardent leur texte affiché (s) pour une colonne de
	// texte (VRAI, FAUX).
	v := parseText(s)
	switch v.t {
	case grid.Integer:
		return sval{k: 'i', i: v.i, s: s}
	case grid.Decimal:
		x := numSval(v.f, false)
		x.s = s
		return x
	case grid.Boolean:
		return sval{k: 'b', i: v.i, s: s}
	case grid.Date:
		return dateSval(v.s)
	case "":
		return sval{}
	}
	return sval{k: 's', s: v.s}
}

// text : la valeur dans une colonne de texte.
func (v sval) text() string {
	if v.s != "" {
		return v.s
	}
	switch v.k {
	case 'i':
		return strconv.FormatInt(v.i, 10)
	case 'f':
		return sqlNumber(v.f)
	case 'b':
		if v.i != 0 {
			return "TRUE"
		}
		return "FALSE"
	}
	return v.s
}

// sqlNumber écrit un décimal : le plus court qui le relit exactement, sans
// exposant pour les grandeurs ordinaires.
func sqlNumber(f float64) string {
	if a := math.Abs(f); a != 0 && (a >= 1e21 || a < 1e-6) {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// add mesure une valeur (première passe).
func (c *sqlCol) add(v sval) {
	st := &c.st
	if v.k == 0 || v.k == 'x' {
		st.nulls++
		return
	}
	first := st.seen&kindBit('i') == 0
	st.seen |= kindBit(v.k)
	switch v.k {
	case 'i':
		if first || v.i < st.minI {
			st.minI = v.i
		}
		if first || v.i > st.maxI {
			st.maxI = v.i
		}
		st.intDigits = max(st.intDigits, len(strings.TrimPrefix(strconv.FormatInt(v.i, 10), "-")))
	case 'f':
		s := strings.TrimPrefix(strconv.FormatFloat(v.f, 'f', -1, 64), "-")
		ip, fp, _ := strings.Cut(s, ".")
		st.intDigits = max(st.intDigits, len(strings.TrimLeft(ip, "0")))
		st.scale = max(st.scale, len(fp))
	}
	// Longueur en texte : colonne de texte, ou dont le genre se déduit.
	if v.k == 's' || c.typ == "" {
		n := len(v.text())
		if !isASCII(v.text()) {
			n = utf16Len(v.text())
		}
		st.maxLen = max(st.maxLen, n)
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// resolve fixe le genre de la colonne : celui de son type de Lattis, ou
// celui des valeurs (colonne calculée, tableau croisé) ; des genres mêlés
// donnent du texte.
func (c *sqlCol) resolve() {
	seen := c.st.seen
	has := func(k byte) bool { return seen&kindBit(k) != 0 }
	switch c.typ {
	case grid.Integer:
		c.kind = 'i'
		return
	case grid.Decimal:
		c.kind = 'f'
		return
	case grid.Boolean:
		c.kind = 'b'
		return
	case grid.Date:
		c.kind = 'd'
		if has('t') {
			c.kind = 't'
		}
		return
	case grid.Text:
		c.kind = 's'
		return
	}
	num, dates := kindBit('i')|kindBit('f'), kindBit('d')|kindBit('t')
	switch {
	case seen == 0:
		c.kind = 's'
	case seen == kindBit('i'):
		c.kind = 'i'
	case seen&^num == 0:
		c.kind = 'f'
	case seen == kindBit('b'):
		c.kind = 'b'
	case seen == kindBit('d'):
		c.kind = 'd'
	case seen&^dates == 0:
		c.kind = 't'
	default:
		c.kind = 's'
	}
}

// sqlDialect : ce qui change d'une base à l'autre.
type sqlDialect struct {
	label         string
	open, close   string // guillemets des noms
	trueLit       string
	falseLit      string
	typedDates    bool // DATE '…', TIMESTAMP '…'
	nchar         bool // N'…' (textes Unicode de SQL Server)
	backslash     bool // \ à doubler dans les textes (MySQL)
	preamble      []string
	begin, commit string
	types         func(c *sqlCol) string
}

var sqlDialects = map[SQLDialect]sqlDialect{
	SQLStandard: {label: "SQL standard", open: `"`, close: `"`, trueLit: "TRUE", falseLit: "FALSE", typedDates: true, types: stdTypes},
	SQLPostgres: {label: "PostgreSQL", open: `"`, close: `"`, trueLit: "TRUE", falseLit: "FALSE", typedDates: true,
		preamble: []string{"SET client_encoding = 'UTF8';"}, begin: "BEGIN;", commit: "COMMIT;", types: pgTypes},
	SQLMySQL: {label: "MySQL", open: "`", close: "`", trueLit: "1", falseLit: "0", typedDates: true, backslash: true,
		preamble: []string{"SET NAMES utf8mb4;"}, begin: "START TRANSACTION;", commit: "COMMIT;", types: mysqlTypes},
	SQLServer: {label: "SQL Server", open: "[", close: "]", trueLit: "1", falseLit: "0", nchar: true,
		preamble: []string{"SET NOCOUNT ON;"}, begin: "BEGIN TRANSACTION;", commit: "COMMIT TRANSACTION;", types: mssqlTypes},
	SQLSQLite: {label: "SQLite", open: `"`, close: `"`, trueLit: "1", falseLit: "0",
		begin: "BEGIN TRANSACTION;", commit: "COMMIT;", types: sqliteTypes},
}

// varcharSize arrondit une longueur au palier au-dessus ; 0 : au-delà de
// limit.
func varcharSize(n, limit int) int {
	for _, p := range []int{10, 20, 50, 100, 255, 500, 1000, 2000, 4000} {
		if p > limit {
			break
		}
		if n <= p {
			return p
		}
	}
	return 0
}

// numeric : précision et échelle d'un NUMERIC(p,s), ok faux s'il faut un
// flottant (trop de décimales ou de chiffres, ou aucune valeur).
func (c *sqlCol) numeric() (p, s int, ok bool) {
	st := c.st
	if st.seen&kindBit('f') == 0 && st.seen&kindBit('i') == 0 {
		return 0, 0, false
	}
	if st.scale > 10 || st.intDigits+st.scale > 38 {
		return 0, 0, false
	}
	// Marge pour les valeurs à venir : au moins 13 chiffres avant la virgule.
	s = st.scale
	p = min(max(st.intDigits, 13)+s, 38)
	return p, s, true
}

func (c *sqlCol) bigint() bool {
	return c.st.seen&kindBit('i') != 0 && (c.st.minI < math.MinInt32 || c.st.maxI > math.MaxInt32)
}

func stdTypes(c *sqlCol) string {
	switch c.kind {
	case 'i':
		if c.bigint() {
			return "BIGINT"
		}
		return "INTEGER"
	case 'f':
		if p, s, ok := c.numeric(); ok {
			return fmt.Sprintf("NUMERIC(%d,%d)", p, s)
		}
		return "DOUBLE PRECISION"
	case 'b':
		return "BOOLEAN"
	case 'd':
		return "DATE"
	case 't':
		return "TIMESTAMP"
	}
	if n := varcharSize(max(c.st.maxLen, 1), 4000); n > 0 {
		return fmt.Sprintf("VARCHAR(%d)", n)
	}
	return "TEXT"
}

func pgTypes(c *sqlCol) string {
	if c.kind == 's' {
		return "TEXT"
	}
	return stdTypes(c)
}

func mysqlTypes(c *sqlCol) string {
	switch c.kind {
	case 'f':
		if p, s, ok := c.numeric(); ok {
			return fmt.Sprintf("DECIMAL(%d,%d)", p, s)
		}
		return "DOUBLE"
	case 'b':
		return "TINYINT(1)"
	case 't':
		return "DATETIME"
	case 's':
		// Une clé ne peut pas être un TEXT (index de 3 072 octets au plus).
		limit := 255
		if c.key {
			limit = 500
		}
		switch n := c.st.maxLen; {
		case varcharSize(max(n, 1), limit) > 0:
			return fmt.Sprintf("VARCHAR(%d)", varcharSize(max(n, 1), limit))
		case n <= 16383:
			return "TEXT"
		case n <= 4194303:
			return "MEDIUMTEXT"
		}
		return "LONGTEXT"
	}
	return stdTypes(c)
}

func mssqlTypes(c *sqlCol) string {
	switch c.kind {
	case 'f':
		if p, s, ok := c.numeric(); ok {
			return fmt.Sprintf("DECIMAL(%d,%d)", p, s)
		}
		return "FLOAT"
	case 'b':
		return "BIT"
	case 't':
		return "DATETIME2(0)"
	case 's':
		if n := varcharSize(max(c.st.maxLen, 1), 4000); n > 0 {
			return fmt.Sprintf("NVARCHAR(%d)", n)
		}
		return "NVARCHAR(MAX)"
	}
	return stdTypes(c)
}

func sqliteTypes(c *sqlCol) string {
	switch c.kind {
	case 'i':
		return "INTEGER"
	case 'f':
		return "REAL"
	case 's':
		return "TEXT"
	}
	return stdTypes(c)
}

// sqlWriter écrit le script.
type sqlWriter struct {
	w           *bufio.Writer
	d           sqlDialect
	ctx         context.Context
	l           msg.Lang
	batch       int
	total, done int64
	rows, nulls int64
}

func (x *sqlWriter) line(s string) {
	x.w.WriteString(s)
	x.w.WriteString("\n")
}

// comment écrit un commentaire d'une ligne.
func (x *sqlWriter) comment(s string) { x.line("-- " + oneLine(s)) }

func oneLine(s string) string {
	return strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)
}

func (x *sqlWriter) quote(name string) string {
	return x.d.open + strings.ReplaceAll(name, x.d.close, x.d.close+x.d.close) + x.d.close
}

func (x *sqlWriter) step() {
	x.done++
	if x.done&1023 == 0 {
		work.Progress(x.ctx, x.done, x.total)
	}
}

// source écrit un tableau : mesure, CREATE TABLE, puis les INSERT.
func (x *sqlWriter) source(s *sqlSource, used map[string]bool) error {
	err := s.read(x.ctx, func(vals []sval) error {
		for i, v := range vals {
			s.cols[i].add(v)
		}
		x.step()
		return nil
	})
	if err != nil {
		return err
	}
	if s.cols == nil {
		return nil
	}
	ident := sqlUnique(s.name, used)
	names := map[string]bool{}
	for _, c := range s.cols {
		c.ident = sqlUnique(c.name, names)
		c.resolve()
	}
	x.line("")
	what := "transfer.sqlTable"
	if s.pivot {
		what = "transfer.sqlPivot"
	}
	x.comment(msg.Format(x.l, msg.ID(what), msg.Args{"name": s.name, "sheet": s.sheet}))
	x.line("CREATE TABLE " + x.quote(ident) + " (")
	var key *sqlCol
	for _, c := range s.cols {
		if c.key {
			if c.st.nulls == 0 {
				key = c
			} else {
				x.comment(msg.Format(x.l, "transfer.sqlNoKey", msg.Args{"column": c.name}))
			}
		}
	}
	for i, c := range s.cols {
		def := "  " + x.quote(c.ident) + " " + x.d.types(c)
		if c == key {
			def += " NOT NULL"
		}
		if i < len(s.cols)-1 || key != nil {
			def += ","
		}
		if c.formula != "" {
			def += " -- " + oneLine(msg.Format(x.l, "transfer.sqlFormula", msg.Args{"formula": c.formula}))
		}
		x.line(def)
	}
	if key != nil {
		x.line("  PRIMARY KEY (" + x.quote(key.ident) + ")")
	}
	x.line(");")
	cols := make([]string, len(s.cols))
	for i, c := range s.cols {
		cols[i] = x.quote(c.ident)
	}
	insert := "INSERT INTO " + x.quote(ident) + " (" + strings.Join(cols, ", ") + ") VALUES"
	n := 0
	var lost int64
	err = s.read(x.ctx, func(vals []sval) error {
		if n%x.batch == 0 {
			if n > 0 {
				x.w.WriteString(";\n")
			}
			x.w.WriteString(insert)
			x.w.WriteString("\n(")
		} else {
			x.w.WriteString(",\n(")
		}
		for i, v := range vals {
			if i > 0 {
				x.w.WriteString(", ")
			}
			if !x.literal(s.cols[i], v) && v.k != 0 {
				lost++
			}
		}
		x.w.WriteByte(')')
		n++
		x.step()
		return nil
	})
	if err != nil {
		return err
	}
	if n > 0 {
		x.w.WriteString(";\n")
	}
	if lost > 0 {
		x.comment(msg.Format(x.l, "transfer.sqlNulls", msg.Args{"n": lost}))
	}
	x.rows += int64(n)
	x.nulls += lost
	return nil
}

// literal écrit une valeur dans le genre de sa colonne ; faux : NULL.
func (x *sqlWriter) literal(c *sqlCol, v sval) bool {
	w := x.w
	switch {
	case v.k == 0 || v.k == 'x':
	case c.kind == 's':
		x.str(v.text())
		return true
	case c.kind == 'i' && v.k == 'i':
		w.WriteString(strconv.FormatInt(v.i, 10))
		return true
	case c.kind == 'f' && v.k == 'i':
		w.WriteString(strconv.FormatInt(v.i, 10))
		return true
	case c.kind == 'f' && v.k == 'f':
		w.WriteString(sqlNumber(v.f))
		return true
	case c.kind == 'b' && v.k == 'b':
		if v.i != 0 {
			w.WriteString(x.d.trueLit)
		} else {
			w.WriteString(x.d.falseLit)
		}
		return true
	case c.kind == 'd' && v.k == 'd':
		x.date("DATE", v.s)
		return true
	case c.kind == 't' && (v.k == 't' || v.k == 'd'):
		s := v.s
		if v.k == 'd' {
			s += " 00:00:00"
		}
		x.date("TIMESTAMP", s)
		return true
	}
	w.WriteString("NULL")
	return false
}

func (x *sqlWriter) date(kw, s string) {
	if x.d.typedDates {
		x.w.WriteString(kw)
		x.w.WriteByte(' ')
	}
	x.w.WriteByte('\'')
	x.w.WriteString(s)
	x.w.WriteByte('\'')
}

// str écrit un texte entre apostrophes (doublées) ; le caractère nul, que
// les bases refusent, est retiré.
func (x *sqlWriter) str(s string) {
	w := x.w
	if x.d.nchar {
		w.WriteByte('N')
	}
	w.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		switch b := s[i]; b {
		case '\'':
			w.WriteString("''")
		case '\\':
			if x.d.backslash {
				w.WriteString(`\\`)
			} else {
				w.WriteByte(b)
			}
		case 0:
		default:
			w.WriteByte(b)
		}
	}
	w.WriteByte('\'')
}

// sqlUnique donne un nom SQL : 63 octets au plus (PostgreSQL ; MySQL en
// prend 64), sans caractère nul, distinct (sans la casse) de ceux de used.
func sqlUnique(name string, used map[string]bool) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\x00", ""))
	if name == "" {
		name = "col"
	}
	base := cutBytes(name, 63)
	out := base
	for i := 2; used[strings.ToLower(out)]; i++ {
		suf := "_" + strconv.Itoa(i)
		out = cutBytes(base, 63-len(suf)) + suf
	}
	used[strings.ToLower(out)] = true
	return out
}

// cutBytes coupe s à n octets au plus, sans couper un caractère.
func cutBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
