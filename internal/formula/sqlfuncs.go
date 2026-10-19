package formula

import (
	"encoding/binary"
	"math"
	"strings"

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
	case Date:
		return binary.LittleEndian.AppendUint64([]byte{blobMark, 'D'}, math.Float64bits(v.Num))
	}
	return nil
}

func decodeBlob(b []byte) (Value, bool) {
	if len(b) == 10 && b[0] == blobMark && b[1] == 'D' {
		return DateSerial(math.Float64frombits(binary.LittleEndian.Uint64(b[2:]))), true
	}
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

// encodeAgg écrit l'état partiel d'un agrégat (slt_part → slt_final).
func encodeAgg(a Agg) []byte {
	b := []byte{blobMark, 'P', byte(a.Err)}
	f := func(x float64) { b = binary.LittleEndian.AppendUint64(b, math.Float64bits(x)) }
	u := func(x int) { b = binary.LittleEndian.AppendUint64(b, uint64(x)) }
	f(a.Sum)
	f(a.Comp)
	u(a.N)
	u(a.Nonempty)
	u(a.Blank)
	u(a.Dates)
	f(a.Min)
	f(a.Max)
	f(a.Prod)
	u(len(a.Vals))
	for _, v := range a.Vals {
		f(v)
	}
	u(len(a.Items))
	for _, s := range a.Items {
		u(len(s))
		b = append(b, s...)
	}
	return b
}

func decodeAgg(b []byte) (Agg, bool) {
	if len(b) < 3 || b[0] != blobMark || b[1] != 'P' {
		return Agg{}, false
	}
	a := Agg{Err: ErrCode(b[2])}
	rest := b[3:]
	ok := true
	next := func() uint64 {
		if len(rest) < 8 {
			ok = false
			return 0
		}
		x := binary.LittleEndian.Uint64(rest)
		rest = rest[8:]
		return x
	}
	f := func() float64 { return math.Float64frombits(next()) }
	u := func() int { return int(next()) }
	a.Sum, a.Comp = f(), f()
	a.N, a.Nonempty, a.Blank, a.Dates = u(), u(), u(), u()
	a.Min, a.Max, a.Prod = f(), f(), f()
	for n := u(); ok && n > 0; n-- {
		a.Vals = append(a.Vals, f())
	}
	for n := u(); ok && n > 0; n-- {
		l := u()
		if !ok || l > len(rest) {
			return Agg{}, false
		}
		a.Items = append(a.Items, string(rest[:l]))
		rest = rest[l:]
	}
	return a, ok && len(rest) == 0
}

// encodeValue enveloppe une valeur quelconque (résultat de slt_scan) pour la
// distinguer de l'absence de ligne (NULL).
func encodeValue(v Value) []byte {
	b := []byte{blobMark, 'W', byte(v.Kind)}
	switch v.Kind {
	case Number, Date:
		b = binary.LittleEndian.AppendUint64(b, math.Float64bits(v.Num))
	case Text:
		b = append(b, v.Str...)
	case Bool:
		b = append(b, byte(boolInt(v.Bool)))
	case Error:
		b = append(b, byte(v.Err))
	}
	return b
}

func decodeValue(b []byte) (Value, bool) {
	if len(b) < 3 || b[0] != blobMark || b[1] != 'W' {
		return Value{}, false
	}
	p := b[3:]
	switch k := Kind(b[2]); k {
	case Number, Date:
		if len(p) != 8 {
			return Value{}, false
		}
		return Value{Kind: k, Num: math.Float64frombits(binary.LittleEndian.Uint64(p))}, true
	case Text:
		return Str(string(p)), true
	case Bool:
		return Boolean(len(p) == 1 && p[0] == 1), len(p) == 1
	case Error:
		return Err(ErrCode(p[0])), len(p) == 1
	case Empty:
		return Value{}, true
	}
	return Value{}, false
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
	const det = sqlite3.DETERMINISTIC | sqlite3.INNOCUOUS
	if err := registerGrouped(c); err != nil {
		return err
	}
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
		case v.Kind == Date && arg[1].Text() == "t":
			ctx.ResultText(ISODate(v.Num))
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
	// slt_tsplitn(args…) : les rangs des valeurs de FRACTIONNER.TEXTE, en
	// tableau JSON pour json_each ; slt_tsplit(i, args…) : la valeur n° i.
	if err := c.CreateFunction("slt_tsplitn", -1, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		n := tsplitCount(args(arg))
		ctx.ResultText("[" + strings.Repeat("0,", n-1) + "0]")
	}); err != nil {
		return err
	}
	if err := c.CreateFunction("slt_tsplit", -1, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		if len(arg) == 0 {
			result(ctx, Err(ErrValue))
			return
		}
		result(ctx, tsplitAt(arg[0].Int(), args(arg[1:])))
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
	// slt_final('SUM', a, b, ...) : combine états partiels et valeurs directes
	// (précédés des arguments ordinaires de JOINDRE.TEXTE ou de VAN).
	if err := c.CreateFunction("slt_final", -1, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		acc := aggOf(arg[0].Text())
		lead := args(arg[1:min(1+aggLead[acc.Fn], len(arg))])
		for _, a := range arg[1+len(lead):] {
			if a.Type() == sqlite3.BLOB {
				if p, ok := decodeAgg(a.RawBlob()); ok {
					acc.Merge(p)
					continue
				}
			}
			acc.AddValue(fromArg(a))
		}
		result(ctx, acc.Final(lead))
	}); err != nil {
		return err
	}
	// slt_date : colonne date stockée en texte ISO → date de formule.
	if err := un("slt_date", func(v Value) Value {
		if v.Kind == Text {
			if f, ok := ParseDate(v.Str); ok {
				return DateSerial(f)
			}
		}
		return v
	}); err != nil {
		return err
	}
	// slt_pick(trouvé, valeur, sinon) : PRECEDENT et SUIVANT.
	if err := c.CreateFunction("slt_pick", 3, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		if arg[0].Int() != 0 {
			result(ctx, fromArg(arg[1]))
		} else {
			result(ctx, fromArg(arg[2]))
		}
	}); err != nil {
		return err
	}
	// slt_exists(trouvé, clé) : EXISTE.
	if err := c.CreateFunction("slt_exists", 2, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		if key := fromArg(arg[1]); key.IsErr() {
			result(ctx, key)
			return
		}
		result(ctx, Boolean(arg[0].Int() != 0))
	}); err != nil {
		return err
	}
	// slt_rand(graine, ligne, rang) : ALEA, stable entre deux F9.
	if err := c.CreateFunction("slt_rand", 3, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		ctx.ResultFloat(randAt(uint64(arg[0].Int64()), arg[1].Int64(), arg[2].Int()))
	}); err != nil {
		return err
	}
	// slt_true(x) : condition de FILTRE en entier SQL (une erreur ne garde
	// pas la ligne).
	if err := c.CreateFunction("slt_true", 1, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		b, e := Truth(fromArg(arg[0]))
		ctx.ResultBool(b && !e.IsErr())
	}); err != nil {
		return err
	}
	// slt_sortkey(x) : clé d'ordre et d'égalité des tableaux dynamiques.
	if err := c.CreateFunction("slt_sortkey", 1, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		if k := sortKey(fromArg(arg[0])); k != nil {
			ctx.ResultBlob(k)
		} else {
			ctx.ResultNull()
		}
	}); err != nil {
		return err
	}
	// slt_desc(sens) : tri décroissant demandé (-1).
	if err := c.CreateFunction("slt_desc", 1, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		d, _ := spillOrder(fromArg(arg[0]))
		ctx.ResultBool(d)
	}); err != nil {
		return err
	}
	// slt_seqn(lignes, colonnes) : nombre de lignes de SEQUENCE, -1 s'il ne
	// convient pas.
	if err := c.CreateFunction("slt_seqn", 2, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		ctx.ResultInt(seqRows(fromArg(arg[0]), arg[1].Int()))
	}); err != nil {
		return err
	}
	// slt_seq(lignes, début, pas, i) : valeur n° i de SEQUENCE (#VALEUR! si
	// le nombre de lignes ne convient pas).
	if err := c.CreateFunction("slt_seq", 4, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		if arg[0].Int() < 0 {
			result(ctx, Err(ErrValue))
			return
		}
		result(ctx, seqValue(fromArg(arg[1]), fromArg(arg[2]), arg[3].Int()))
	}); err != nil {
		return err
	}
	// slt_take(rang, total, n, exclure) : lignes gardées par PRENDRE / EXCLURE.
	if err := c.CreateFunction("slt_take", 4, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		n, e := spillCount(fromArg(arg[2]))
		if e.IsErr() {
			ctx.ResultInt(0)
			return
		}
		lo, hi := takeRange(arg[3].Int() == 1, n, arg[1].Int())
		r := arg[0].Int() - 1
		ctx.ResultBool(r >= lo && r < hi)
	}); err != nil {
		return err
	}
	// slt_scan('SUMIF', nparams, params..., colonnes...) : fonction de
	// parcours (scan.go) ; rend le résultat enveloppé (encodeValue).
	if err := c.CreateWindowFunction("slt_scan", -1, det, func() sqlite3.AggregateFunction { return &scanAgg{} }); err != nil {
		return err
	}
	// slt_scanfin('SUMIF', nparams, params..., résultat de slt_scan) : le
	// résultat, ou celui d'un parcours sans ligne (tableau vide).
	return c.CreateFunction("slt_scanfin", -1, det, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		last := arg[len(arg)-1]
		if last.Type() == sqlite3.BLOB {
			if v, ok := decodeValue(last.RawBlob()); ok {
				result(ctx, v)
				return
			}
		}
		name, rule := splitKey(arg[0].Text())
		def, ok := scans[name]
		if !ok {
			result(ctx, Err(ErrName))
			return
		}
		np := arg[1].Int()
		result(ctx, newScanner(def, args(arg[2:2+np]), rule).result())
	})
}

type scanAgg struct{ s scanner }

func (a *scanAgg) Step(ctx sqlite3.Context, arg ...sqlite3.Value) {
	np := arg[1].Int()
	if a.s == nil {
		name, rule := splitKey(arg[0].Text())
		def, ok := scans[name]
		if !ok {
			return
		}
		a.s = newScanner(def, args(arg[2:2+np]), rule)
	}
	a.s.add(args(arg[2+np:]))
}

func (a *scanAgg) Value(ctx sqlite3.Context) {
	if a.s == nil {
		ctx.ResultNull()
		return
	}
	ctx.ResultBlob(encodeValue(a.s.result()))
}

type partAgg struct{ acc Agg }

func (p *partAgg) Step(ctx sqlite3.Context, arg ...sqlite3.Value) {
	if p.acc.Fn == "" {
		p.acc = aggOf(arg[0].Text())
	}
	p.acc.AddRange(fromArg(arg[1]))
}

func (p *partAgg) Value(ctx sqlite3.Context) { ctx.ResultBlob(encodeAgg(p.acc)) }
