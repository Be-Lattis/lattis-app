package formula

import (
	"encoding/binary"
	"math"

	"github.com/ncruces/go-sqlite3"
)

// Les valeurs qui n'ont pas d'équivalent SQLite (booléen, erreur typée,
// agrégat partiel) voyagent en SQL comme de petits BLOB marqués. Les
// tableaux utilisateur ne contiennent jamais de BLOB : pas d'ambiguïté.
const blobMark = 0xE5

func encodeBlob(v Value) []byte {
	switch v.Kind {
	case Bool:
		b := byte(0)
		if v.Bool {
			b = 1
		}
		return []byte{blobMark, 'B', b}
	case Error:
		return []byte{blobMark, 'E', byte(v.Err)}
	}
	return nil
}

func decodeBlob(b []byte) (Value, bool) {
	if len(b) == 3 && b[0] == blobMark {
		switch b[1] {
		case 'B':
			return Boolean(b[2] == 1), true
		case 'E':
			return Err(ErrCode(b[2])), true
		}
	}
	return Value{}, false
}

func encodeAgg(a Agg) []byte {
	b := make([]byte, 3, 3+5*8)
	b[0], b[1], b[2] = blobMark, 'P', byte(a.Err)
	b = binary.LittleEndian.AppendUint64(b, math.Float64bits(a.Sum))
	b = binary.LittleEndian.AppendUint64(b, uint64(a.N))
	b = binary.LittleEndian.AppendUint64(b, uint64(a.Nonempty))
	b = binary.LittleEndian.AppendUint64(b, math.Float64bits(a.Min))
	b = binary.LittleEndian.AppendUint64(b, math.Float64bits(a.Max))
	return b
}

func decodeAgg(b []byte) (Agg, bool) {
	if len(b) != 43 || b[0] != blobMark || b[1] != 'P' {
		return Agg{}, false
	}
	u := func(i int) uint64 { return binary.LittleEndian.Uint64(b[3+8*i:]) }
	return Agg{
		Err: ErrCode(b[2]), Sum: math.Float64frombits(u(0)), N: int(u(1)), Nonempty: int(u(2)),
		Min: math.Float64frombits(u(3)), Max: math.Float64frombits(u(4)),
	}, true
}

// fromArg lit un argument de fonction SQL.
func fromArg(v sqlite3.Value) Value {
	switch v.Type() {
	case sqlite3.INTEGER:
		return Num(float64(v.Int64()))
	case sqlite3.FLOAT:
		return Num(v.Float())
	case sqlite3.TEXT:
		return Str(v.Text())
	case sqlite3.BLOB:
		if val, ok := decodeBlob(v.RawBlob()); ok {
			return val
		}
		return Err(ErrValue)
	}
	return Value{}
}

func result(ctx sqlite3.Context, v Value) {
	switch v.Kind {
	case Number:
		ctx.ResultFloat(v.Num)
	case Text:
		ctx.ResultText(v.Str)
	case Empty:
		ctx.ResultNull()
	default:
		ctx.ResultBlob(encodeBlob(v))
	}
}

// ToSQL convertit une valeur en paramètre de requête.
func ToSQL(v Value) any {
	switch v.Kind {
	case Number:
		return v.Num
	case Text:
		return v.Str
	case Empty:
		return nil
	}
	return encodeBlob(v)
}

// FromResult lit une colonne de résultat d'une expression compilée.
func FromResult(v any) Value { return FromSQL(v, "") }

func args(arg []sqlite3.Value) []Value {
	out := make([]Value, len(arg))
	for i, a := range arg {
		out[i] = fromArg(a)
	}
	return out
}

// Register déclare les fonctions slt_* sur une connexion. Elles appellent
// le même code Go que l'évaluateur : la sémantique est définie à un seul
// endroit.
func Register(c *sqlite3.Conn) error {
	const det = sqlite3.DETERMINISTIC
	bin := func(name string, f func(a, b Value) Value) error {
		return c.CreateFunction(name, 2, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
			result(ctx, f(fromArg(arg[0]), fromArg(arg[1])))
		})
	}
	for name, op := range map[string]byte{"slt_add": '+', "slt_sub": '-', "slt_mul": '*', "slt_div": '/', "slt_pow": '^'} {
		if err := bin(name, func(a, b Value) Value { return Arith(op, a, b) }); err != nil {
			return err
		}
	}
	for name, op := range map[string]string{"slt_eq": "=", "slt_ne": "<>", "slt_lt": "<", "slt_le": "<=", "slt_gt": ">", "slt_ge": ">="} {
		if err := bin(name, func(a, b Value) Value { return Compare(op, a, b) }); err != nil {
			return err
		}
	}
	if err := bin("slt_cat", Concat); err != nil {
		return err
	}
	un := func(name string, f func(Value) Value) error {
		return c.CreateFunction(name, 1, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
			result(ctx, f(fromArg(arg[0])))
		})
	}
	if err := un("slt_neg", Neg); err != nil {
		return err
	}
	if err := un("slt_pct", Percent); err != nil {
		return err
	}
	if err := un("slt_plus", func(v Value) Value {
		if _, e := ToNumber(v); e.IsErr() {
			return e
		}
		return v
	}); err != nil {
		return err
	}
	// slt_bool : colonne booléenne stockée en 0/1 → booléen de formule.
	if err := un("slt_bool", func(v Value) Value {
		if v.Kind == Number {
			return Boolean(v.Num != 0)
		}
		return v
	}); err != nil {
		return err
	}
	// slt_err(code) : erreur littérale.
	if err := un("slt_err", func(v Value) Value { return Err(ErrCode(v.Num)) }); err != nil {
		return err
	}
	// slt_key(clé, 'n'|'t') : valeur de recherche comparable à une colonne clé
	// numérique ou texte. Une clé d'un autre type ne trouve rien, comme dans
	// Excel ("1" ne trouve pas 1), alors que SQLite convertirait.
	if err := c.CreateFunction("slt_key", 2, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		v := fromArg(arg[0])
		switch {
		case v.Kind == Number && arg[1].Text() == "n":
			ctx.ResultFloat(v.Num)
		case v.Kind == Text && arg[1].Text() == "t":
			ctx.ResultText(v.Str)
		default:
			ctx.ResultNull()
		}
	}); err != nil {
		return err
	}
	// slt_ref(trouvé, valeur, clé) : résultat de REFERENCE.
	if err := c.CreateFunction("slt_ref", 3, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		key := fromArg(arg[2])
		switch {
		case key.IsErr():
			result(ctx, key)
		case arg[0].Int() == 0:
			result(ctx, Err(ErrNA))
		default:
			result(ctx, fromArg(arg[1]))
		}
	}); err != nil {
		return err
	}
	// slt_fn('IF', ...) : fonctions scalaires du catalogue.
	if err := c.CreateFunction("slt_fn", -1, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		fn, ok := lookupFunc(arg[0].Text())
		if !ok || fn.impl == nil {
			result(ctx, Err(ErrName))
			return
		}
		result(ctx, fn.impl(args(arg[1:])))
	}); err != nil {
		return err
	}
	// slt_part('SUM', x) : agrégat sur une colonne, renvoyé en état partiel.
	if err := c.CreateWindowFunction("slt_part", 2, det, func() sqlite3.AggregateFunction { return &partAgg{} }); err != nil {
		return err
	}
	// slt_final('SUM', a, b, ...) : combine états partiels et valeurs directes.
	return c.CreateFunction("slt_final", -1, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		acc := Agg{Fn: arg[0].Text()}
		for _, a := range arg[1:] {
			if a.Type() == sqlite3.BLOB {
				if p, ok := decodeAgg(a.RawBlob()); ok {
					acc.Merge(p)
					continue
				}
			}
			acc.AddValue(fromArg(a))
		}
		result(ctx, acc.Result())
	})
}

type partAgg struct{ acc Agg }

func (p *partAgg) Step(ctx sqlite3.Context, arg ...sqlite3.Value) {
	if p.acc.Fn == "" {
		p.acc.Fn = arg[0].Text()
	}
	p.acc.AddRange(fromArg(arg[1]))
}

func (p *partAgg) Value(ctx sqlite3.Context) { ctx.ResultBlob(encodeAgg(p.acc)) }
