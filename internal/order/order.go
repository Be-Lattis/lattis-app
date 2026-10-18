// Package order fournit les clés d'ordre des lignes d'un tableau.
//
// Une table SQL n'a pas d'ordre propre : chaque ligne porte une clé texte
// (colonne _pos) dont l'ordre lexicographique donne l'ordre d'affichage. Les
// clés sont des fractions en base 62 (0,k1k2k3...) : entre deux clés il en
// existe toujours une troisième, ce qui permet d'insérer une ligne n'importe où
// sans renuméroter les autres (« fractional indexing »).
//
// Invariant : une clé n'est jamais vide et ne se termine jamais par '0', sinon
// "a" et "a0" désigneraient la même fraction et rien ne pourrait s'insérer
// entre elles.
package order

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
)

// Digits est l'alphabet des clés, dans l'ordre ASCII croissant.
const Digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

const base = len(Digits)

// ErrOrder signale des bornes invalides (a >= b ou clé mal formée).
var ErrOrder = errors.New("order: bornes invalides")

// Between renvoie une clé strictement comprise entre a et b. Une borne vide
// est ouverte : Between("", b) insère avant b, Between(a, "") après a, et
// Between("", "") donne une première clé.
func Between(a, b string) (string, error) {
	if !valid(a) || !valid(b) || (a != "" && b != "" && a >= b) {
		return "", ErrOrder
	}
	switch {
	case b == "" && a != "":
		return after(a), nil
	case a == "" && b != "":
		return before(b), nil
	}
	return midpoint(a, b), nil
}

// Distinct renvoie une clé strictement comprise entre a et b qui se
// termine par suffix (une clé valable, voir Suffix). Deux postes qui
// insèrent au même endroit en même temps obtiendraient la même clé avec
// Between ; avec deux suffixes différents, leurs clés diffèrent et l'ordre
// est le même partout (classeur collaboratif).
func Distinct(a, b, suffix string) (string, error) {
	if suffix == "" || !valid(suffix) {
		return "", ErrOrder
	}
	// m + suffix reste sous b dès que m n'est pas un préfixe de b ; sinon
	// on cherche entre a et m (une borne strictement plus courte).
	hi := b
	for {
		m, err := Between(a, hi)
		if err != nil {
			return "", err
		}
		if hi == "" || !strings.HasPrefix(hi, m) {
			return m + suffix, nil
		}
		hi = m
	}
}

// SuffixLen : longueur des suffixes de Suffix (62^4, près de 15 millions).
const SuffixLen = 4

// Suffix tire un suffixe aléatoire pour Distinct.
func Suffix() string {
	b := make([]byte, SuffixLen)
	for i := range b {
		b[i] = Digits[rand.IntN(base)]
	}
	b[len(b)-1] = Digits[1+rand.IntN(base-1)] // jamais de 0 final
	return string(b)
}

// after donne une clé juste après a, pour ajouter en fin : la clé s'allonge
// d'un caractère seulement quand le préfixe de « z » s'allonge, et chaque
// niveau offre 62^(k+1) places. Couper au milieu (midpoint) ajouterait un
// caractère toutes les six insertions environ.
func after(a string) string {
	k := 0
	for k < len(a) && a[k] == 'z' {
		k++
	}
	rest := []byte(a[k:])
	w := max(len(rest), k+1)
	for len(rest) < w {
		rest = append(rest, '0')
	}
	// Incrémente rest (w chiffres) : rest[0] != 'z', pas de débordement.
	for i := w - 1; i >= 0; i-- {
		if d := digit(rest[i]); d < base-1 {
			rest[i] = Digits[d+1]
			break
		}
		rest[i] = '0'
	}
	return strings.TrimRight(a[:k]+string(rest), "0")
}

// before donne une clé juste avant b, pour insérer en tête (symétrique de
// after, sur le préfixe de « 0 »).
func before(b string) string {
	k := 0
	for k < len(b) && b[k] == '0' {
		k++
	}
	rest := []byte(b[k:])
	w := max(len(rest), k+2)
	for len(rest) < w {
		rest = append(rest, '0')
	}
	// Décrémente rest (w chiffres) : rest[0] != '0', pas de débordement.
	for i := w - 1; i >= 0; i-- {
		if d := digit(rest[i]); d > 0 {
			rest[i] = Digits[d-1]
			break
		}
		rest[i] = Digits[base-1]
	}
	return strings.TrimRight(b[:k]+string(rest), "0")
}

// Spread renvoie n clés croissantes, régulièrement espacées et de longueur au
// plus égale, pour charger un tableau en masse en laissant de la place entre
// deux lignes pour de futures insertions.
func Spread(n int) []string {
	if n <= 0 {
		return nil
	}
	// Largeur w telle que 62^w dépasse largement n : un chiffre de marge
	// laisse au moins 61 insertions entre deux clés avant d'allonger les clés.
	width, capacity := 1, uint64(base)
	for capacity <= uint64(n)*uint64(base) {
		width++
		capacity *= uint64(base)
	}
	step := capacity / uint64(n+1)
	keys := make([]string, n)
	buf := make([]byte, width)
	for i := range n {
		v := step * uint64(i+1)
		for k := width - 1; k >= 0; k-- {
			buf[k] = Digits[v%uint64(base)]
			v /= uint64(base)
		}
		keys[i] = strings.TrimRight(string(buf), "0")
	}
	return keys
}

func valid(k string) bool {
	if k == "" {
		return true
	}
	if k[len(k)-1] == '0' {
		return false
	}
	for i := range len(k) {
		if digit(k[i]) < 0 {
			return false
		}
	}
	return true
}

func digit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 36
	}
	return -1
}

// midpoint suppose a < b (b vide = 1) et renvoie la clé la plus courte
// possible entre les deux.
func midpoint(a, b string) string {
	if b != "" {
		// Préfixe commun, a étant complété par des '0'.
		n := 0
		for n < len(b) && at(a, n) == b[n] {
			n++
		}
		if n > 0 {
			rest := ""
			if n < len(a) {
				rest = a[n:]
			}
			return b[:n] + midpoint(rest, b[n:])
		}
	}
	da := 0
	if a != "" {
		da = digit(a[0])
	}
	db := base
	if b != "" {
		db = digit(b[0])
	}
	if db-da > 1 {
		return string(Digits[(da+db+1)/2])
	}
	// Chiffres consécutifs : on garde le premier chiffre de b s'il est suivi
	// d'autre chose (b[:1] est alors strictement entre a et b), sinon on
	// descend d'un rang sous a.
	if len(b) > 1 {
		return b[:1]
	}
	rest := ""
	if len(a) > 1 {
		rest = a[1:]
	}
	return string(Digits[da]) + midpoint(rest, "")
}

func at(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return '0'
}

// SpreadSQL est Spread écrit en SQL : l'expression donne la clé de la
// ligne n (1 à count) quand n est une expression entière (une colonne), pour
// remplir un tableau par INSERT … SELECT sans passer par Go.
func SpreadSQL(count int, n string) string {
	if count <= 0 {
		return "NULL"
	}
	width, capacity := 1, uint64(base)
	for capacity <= uint64(count)*uint64(base) {
		width++
		capacity *= uint64(base)
	}
	step := capacity / uint64(count+1)
	parts := make([]string, width)
	div := uint64(1)
	for k := width - 1; k >= 0; k-- {
		parts[k] = fmt.Sprintf("substr('%s', ((%s) * %d / %d) %% %d + 1, 1)", Digits, n, step, div, base)
		div *= uint64(base)
	}
	return "rtrim(" + strings.Join(parts, " || ") + ", '0')"
}

// FixedSQL écrit en SQL les width chiffres en base 62 de l'entier expr,
// zéros de tête compris : des préfixes de même largeur, dans l'ordre des
// entiers. Préfixe + clé (Spread) donne des clés valables, dans l'ordre
// (préfixe d'abord) : de quoi recopier un bloc de lignes à la suite.
func FixedSQL(width int, expr string) string {
	parts := make([]string, width)
	div := uint64(1)
	for k := width - 1; k >= 0; k-- {
		parts[k] = fmt.Sprintf("substr('%s', ((%s) / %d) %% %d + 1, 1)", Digits, expr, div, base)
		div *= uint64(base)
	}
	return strings.Join(parts, " || ")
}

// FixedWidth donne la largeur de FixedSQL pour les entiers de 0 à n - 1.
func FixedWidth(n int) int {
	w, c := 1, base
	for c < n {
		w++
		c *= base
	}
	return w
}
