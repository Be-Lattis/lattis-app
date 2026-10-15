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
	return midpoint(a, b), nil
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
