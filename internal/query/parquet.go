package query

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/deprecated"
	"github.com/parquet-go/parquet-go/format"

	"github.com/lalamefine/lattis/internal/grid"
)

// Fichiers Parquet : lecteur parquet-go (Go pur). Chaque colonne feuille
// du schéma donne une colonne (« parent.enfant » pour une structure) ;
// une colonne répétée (liste) donne le texte de ses valeurs séparées par
// des virgules. Types : entiers, décimaux (DECIMAL compris), booléens,
// dates ; horodatages et heures en texte ISO, comme PostgreSQL.

// pqLeaf décrit une colonne feuille d'un fichier Parquet.
type pqLeaf struct {
	name     string
	typ      grid.Type
	repeated bool
	conv     func(v parquet.Value) any
}

// parquetName simplifie le chemin d'une colonne : les niveaux techniques
// des listes et des dictionnaires (list.element, key_value…) disparaissent.
func parquetName(path []string) string {
	var out []string
	for i, p := range path {
		if i > 0 && (p == "list" || p == "element" || p == "item" || p == "array" || p == "bag" || p == "key_value") {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, ".")
}

// parquetLeaf prépare la conversion d'une colonne d'après son type.
func parquetLeaf(path []string, leaf parquet.LeafColumn) pqLeaf {
	l := pqLeaf{name: parquetName(path), repeated: leaf.MaxRepetitionLevel > 0}
	t := leaf.Node.Type()
	lt := t.LogicalType()
	var ct deprecated.ConvertedType = -1
	if c := t.ConvertedType(); c != nil {
		ct = *c
	}
	text := func(v parquet.Value) any {
		b := v.ByteArray()
		if utf8.Valid(b) {
			return string(b)
		}
		return fmt.Sprintf("%x", b)
	}
	scaleOf := func() int32 {
		if lt != nil {
			if d, ok := lt.Value.(*format.DecimalType); ok {
				return d.Scale
			}
		}
		return -1
	}
	var unit time.Duration
	if lt != nil {
		switch x := lt.Value.(type) {
		case *format.TimestampType:
			if x.Unit.Value != nil {
				unit = x.Unit.Value.Duration()
			}
		}
	}
	switch {
	case ct == deprecated.TimestampMillis:
		unit = time.Millisecond
	case ct == deprecated.TimestampMicros:
		unit = time.Microsecond
	}
	scale := scaleOf()
	isDate := ct == deprecated.Date
	isTime := ct == deprecated.TimeMillis || ct == deprecated.TimeMicros
	if lt != nil {
		switch lt.Value.(type) {
		case *format.DateType:
			isDate = true
		case *format.TimeType:
			isTime = true
		}
	}
	switch t.Kind() {
	case parquet.Boolean:
		l.typ = grid.Boolean
		l.conv = func(v parquet.Value) any {
			if v.Boolean() {
				return int64(1)
			}
			return int64(0)
		}
	case parquet.Int32, parquet.Int64:
		get := func(v parquet.Value) int64 {
			if v.Kind() == parquet.Int32 {
				return int64(v.Int32())
			}
			return v.Int64()
		}
		switch {
		case isDate:
			l.typ = grid.Date
			l.conv = func(v parquet.Value) any {
				return time.Unix(0, 0).UTC().AddDate(0, 0, int(get(v))).Format("2006-01-02")
			}
		case unit != 0:
			l.typ = grid.Text
			l.conv = func(v parquet.Value) any {
				return time.Unix(0, 0).UTC().Add(time.Duration(get(v)) * unit).Format("2006-01-02 15:04:05.999999")
			}
		case isTime:
			l.typ = grid.Text
			mul := time.Millisecond
			if t.Kind() == parquet.Int64 {
				mul = time.Microsecond
			}
			l.conv = func(v parquet.Value) any {
				return time.Unix(0, 0).UTC().Add(time.Duration(get(v)) * mul).Format("15:04:05.999999")
			}
		case scale > 0:
			l.typ = grid.Decimal
			l.conv = func(v parquet.Value) any { return float64(get(v)) / math.Pow10(int(scale)) }
		default:
			l.typ = grid.Integer
			l.conv = func(v parquet.Value) any { return get(v) }
		}
	case parquet.Float:
		l.typ = grid.Decimal
		l.conv = func(v parquet.Value) any { return float64(v.Float()) }
	case parquet.Double:
		l.typ = grid.Decimal
		l.conv = func(v parquet.Value) any { return v.Double() }
	case parquet.Int96:
		// Horodatage historique (Impala, Spark) : nanosecondes du jour puis
		// jour julien.
		l.typ = grid.Text
		l.conv = func(v parquet.Value) any {
			x := v.Int96()
			nanos := int64(x[0]) | int64(x[1])<<32
			day := int64(x[2])
			return time.Unix((day-2440588)*86400, nanos).UTC().Format("2006-01-02 15:04:05.999999")
		}
	default: // ByteArray, FixedLenByteArray
		switch {
		case scale >= 0:
			l.typ = grid.Decimal
			l.conv = func(v parquet.Value) any {
				n := new(big.Int).SetBytes(v.ByteArray())
				if b := v.ByteArray(); len(b) > 0 && b[0]&0x80 != 0 {
					n.Sub(n, new(big.Int).Lsh(big.NewInt(1), uint(len(b)*8)))
				}
				f, _ := new(big.Float).Quo(new(big.Float).SetInt(n), new(big.Float).SetFloat64(math.Pow10(int(scale)))).Float64()
				return f
			}
		default:
			if lt != nil {
				if _, ok := lt.Value.(*format.UUIDType); ok {
					l.typ = grid.Text
					l.conv = func(v parquet.Value) any {
						b := v.ByteArray()
						if len(b) != 16 {
							return text(v)
						}
						return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
					}
					break
				}
			}
			l.typ = grid.Text
			l.conv = text
		}
	}
	if l.repeated {
		l.typ = grid.Text
	}
	return l
}

func (r *run) loadParquet(src Source) (loaded, error) {
	if src.Path == "" {
		return loaded{}, errors.New("fichier Parquet non choisi")
	}
	f, err := os.Open(src.Path)
	if err != nil {
		return loaded{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return loaded{}, err
	}
	pf, err := parquet.OpenFile(f, st.Size())
	if err != nil {
		return loaded{}, fmt.Errorf("%s : fichier Parquet illisible : %w", baseName(src.Path), err)
	}
	schema := pf.Schema()
	paths := schema.Columns()
	leaves := make([]pqLeaf, len(paths))
	cols := make([]Column, len(paths))
	for i, p := range paths {
		leaf, ok := schema.Lookup(p...)
		if !ok {
			return loaded{}, fmt.Errorf("colonne %s introuvable dans le schéma", strings.Join(p, "."))
		}
		leaves[i] = parquetLeaf(p, leaf)
		cols[i] = Column{Name: leaves[i].name, Type: leaves[i].typ}
	}
	s, err := r.begin(cols)
	if err != nil {
		return loaded{}, err
	}
	err = readParquetRows(pf, leaves, s.insert)
	return s.finish(err)
}

// readParquetRows lit les lignes d'un fichier, converties colonne par
// colonne.
func readParquetRows(pf *parquet.File, leaves []pqLeaf, each func([]any) error) (err error) {
	defer func() {
		// Un fichier corrompu peut faire paniquer le décodeur.
		if p := recover(); p != nil {
			err = fmt.Errorf("fichier Parquet illisible : %v", p)
		}
	}()
	rd := parquet.NewReader(pf)
	defer rd.Close()
	buf := make([]parquet.Row, 256)
	out := make([]any, len(leaves))
	lists := make([][]string, len(leaves))
	for {
		n, rerr := rd.ReadRows(buf)
		for _, row := range buf[:n] {
			for i := range out {
				out[i] = nil
				lists[i] = lists[i][:0]
			}
			for _, v := range row {
				c := v.Column()
				if c < 0 || c >= len(leaves) || v.IsNull() {
					continue
				}
				x := leaves[c].conv(v)
				if leaves[c].repeated {
					lists[c] = append(lists[c], fmt.Sprint(x))
					continue
				}
				out[c] = x
			}
			for i, l := range lists {
				if leaves[i].repeated && len(l) > 0 {
					out[i] = strings.Join(l, ", ")
				}
			}
			if err := each(out); err != nil {
				return err
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}
