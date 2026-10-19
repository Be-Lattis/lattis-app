// Package formula est le langage de formules (validé au spike 1.3, voir
// docs/decisions/0001-jalon-m1.md) :
// un parseur FR/EN, un AST, et deux backends qui partagent la même
// sémantique « à la Excel » : un évaluateur Go (cellules nommées) et une
// compilation en SQL (colonnes calculées, agrégats), où les opérateurs sont
// des fonctions SQL écrites en Go qui appellent le code de ce fichier.
package formula

import (
	"math"
	"strconv"
	"strings"
)

// Kind est le type d'une valeur de formule.
type Kind uint8

const (
	Empty Kind = iota
	Number
	Text
	Bool
	Error
	Date // nombre de jours depuis le 30/12/1899 (numéro de série d'Excel), heure en fraction
)

// ErrCode est une erreur typée, comme dans Excel.
type ErrCode uint8

const (
	ErrDiv0 ErrCode = iota + 1
	ErrValue
	ErrNA
	ErrRef
	ErrName
	ErrNum
	// ErrCirc : référence circulaire (une formule qui dépend de sa propre
	// valeur). Excel affiche 0 et un avertissement ; Lattis l'affiche dans
	// la cellule, avec une explication. Exportée en #REF! vers Excel.
	ErrCirc
)

var errNames = map[ErrCode]string{
	ErrDiv0:  "#DIV/0!",
	ErrValue: "#VALEUR!",
	ErrNA:    "#N/A",
	ErrRef:   "#REF!",
	ErrName:  "#NOM?",
	ErrNum:   "#NOMBRE!",
	ErrCirc:  "#CIRC!",
}

// Noms anglais acceptés à la saisie.
var errAliases = map[string]ErrCode{"#VALUE!": ErrValue, "#NAME?": ErrName, "#NUM!": ErrNum}

func (e ErrCode) String() string { return errNames[e] }

// Value est une valeur de formule.
type Value struct {
	Kind Kind
	Num  float64
	Str  string
	Bool bool
	Err  ErrCode
}

func Num(f float64) Value        { return Value{Kind: Number, Num: f} }
func Str(s string) Value         { return Value{Kind: Text, Str: s} }
func Boolean(b bool) Value       { return Value{Kind: Bool, Bool: b} }
func Err(e ErrCode) Value        { return Value{Kind: Error, Err: e} }
func DateSerial(f float64) Value { return Value{Kind: Date, Num: f} }
func (v Value) IsErr() bool      { return v.Kind == Error }
func (v Value) IsEmpty() bool    { return v.Kind == Empty }

// String affiche la valeur comme la grille (locale française).
func (v Value) String() string {
	switch v.Kind {
	case Number:
		return FormatNumber(v.Num)
	case Date:
		return FormatDate(v.Num)
	case Text:
		return v.Str
	case Bool:
		if v.Bool {
			return "VRAI"
		}
		return "FAUX"
	case Error:
		return v.Err.String()
	}
	return ""
}

// FormatNumber écrit un nombre avec 15 chiffres significatifs au plus, comme
// Excel (0,1 + 0,2 affiche 0,3), virgule décimale.
func FormatNumber(f float64) string {
	s := strconv.FormatFloat(f, 'g', 15, 64)
	if !strings.ContainsAny(s, "e") {
		if r, err := strconv.ParseFloat(s, 64); err == nil {
			s = strconv.FormatFloat(r, 'f', -1, 64)
		}
	}
	return strings.Replace(s, ".", ",", 1)
}

// parseNumber lit un nombre écrit dans du texte (« 3 », « 3,5 », « 3.5 »,
// « 1 234,5 »).
func parseNumber(s string) (float64, bool) {
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == ' ' || r == ' ' {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	return f, err == nil
}

// ToNumber convertit une valeur pour un calcul : vide = 0, VRAI = 1, texte
// numérique accepté, autre texte = #VALEUR!.
func ToNumber(v Value) (float64, Value) {
	switch v.Kind {
	case Number, Date:
		return v.Num, Value{}
	case Empty:
		return 0, Value{}
	case Bool:
		if v.Bool {
			return 1, Value{}
		}
		return 0, Value{}
	case Text:
		if f, ok := parseNumber(v.Str); ok {
			return f, Value{}
		}
		return 0, Err(ErrValue)
	}
	return 0, v
}

// ToText convertit une valeur pour une concaténation.
func ToText(v Value) (string, Value) {
	if v.IsErr() {
		return "", v
	}
	return v.String(), Value{}
}

// Truth donne la valeur logique d'une condition.
func Truth(v Value) (bool, Value) {
	switch v.Kind {
	case Bool:
		return v.Bool, Value{}
	case Number, Date:
		return v.Num != 0, Value{}
	case Empty:
		return false, Value{}
	case Text:
		switch strings.ToUpper(v.Str) {
		case "VRAI", "TRUE":
			return true, Value{}
		case "FAUX", "FALSE":
			return false, Value{}
		}
		return false, Err(ErrValue)
	}
	return false, v
}

func number(f float64) Value {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return Err(ErrNum)
	}
	return Num(f)
}

// Arith applique un opérateur arithmétique (+ - * / ^).
func Arith(op byte, a, b Value) Value {
	x, e := ToNumber(a)
	if e.IsErr() {
		return e
	}
	y, e := ToNumber(b)
	if e.IsErr() {
		return e
	}
	// Arithmétique des dates, comme Excel : date ± jours donne une date, et
	// date - date un nombre de jours.
	date := a.Kind == Date && b.Kind != Date || op == '+' && b.Kind == Date && a.Kind != Date
	switch op {
	case '+':
		if date {
			return dateNumber(x + y)
		}
		return number(x + y)
	case '-':
		if date {
			return dateNumber(x - y)
		}
		return number(x - y)
	case '*':
		return number(x * y)
	case '/':
		if y == 0 {
			return Err(ErrDiv0)
		}
		return number(x / y)
	case '^':
		return number(math.Pow(x, y))
	}
	return Err(ErrValue)
}

// Neg et Percent sont les opérateurs unaires.
func Neg(a Value) Value {
	x, e := ToNumber(a)
	if e.IsErr() {
		return e
	}
	return Num(-x)
}

func Percent(a Value) Value {
	x, e := ToNumber(a)
	if e.IsErr() {
		return e
	}
	return Num(x / 100)
}

// Concat est l'opérateur &.
func Concat(a, b Value) Value {
	x, e := ToText(a)
	if e.IsErr() {
		return e
	}
	y, e := ToText(b)
	if e.IsErr() {
		return e
	}
	return Str(x + y)
}

// order classe les types comme Excel : nombres < textes < booléens.
func order(k Kind) int {
	switch k {
	case Number, Date:
		return 0
	case Text:
		return 1
	case Bool:
		return 2
	}
	return -1
}

// cmp compare deux valeurs sans erreur. Une cellule vide vaut 0, "" ou FAUX
// selon ce à quoi on la compare. Le texte est comparé sans tenir compte de
// la casse.
func cmp(a, b Value) int {
	if a.Kind == Empty {
		a = zeroLike(b)
	}
	if b.Kind == Empty {
		b = zeroLike(a)
	}
	if a.Kind != b.Kind && order(a.Kind) != order(b.Kind) {
		return order(a.Kind) - order(b.Kind)
	}
	switch a.Kind {
	case Number, Date:
		switch {
		case a.Num < b.Num:
			return -1
		case a.Num > b.Num:
			return 1
		}
		return 0
	case Text:
		return strings.Compare(strings.ToLower(a.Str), strings.ToLower(b.Str))
	case Bool:
		switch {
		case a.Bool == b.Bool:
			return 0
		case !a.Bool:
			return -1
		}
		return 1
	}
	return 0
}

func zeroLike(v Value) Value {
	switch v.Kind {
	case Text:
		return Str("")
	case Bool:
		return Boolean(false)
	}
	return Num(0)
}

// Compare applique un opérateur de comparaison (=, <>, <, <=, >, >=).
func Compare(op string, a, b Value) Value {
	if a.IsErr() {
		return a
	}
	if b.IsErr() {
		return b
	}
	c := cmp(a, b)
	switch op {
	case "=":
		return Boolean(c == 0)
	case "<>":
		return Boolean(c != 0)
	case "<":
		return Boolean(c < 0)
	case "<=":
		return Boolean(c <= 0)
	case ">":
		return Boolean(c > 0)
	case ">=":
		return Boolean(c >= 0)
	}
	return Err(ErrValue)
}

// Equal sert aux recherches (REFERENCE) : même règle que l'opérateur =.
func Equal(a, b Value) bool {
	return !a.IsErr() && !b.IsErr() && cmp(a, b) == 0
}
