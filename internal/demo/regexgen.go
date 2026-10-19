package demo

import (
	"fmt"
	"regexp/syntax"
	"strings"
	"unicode"
)

// Générateur « regex » (demande de Louis) : une chaîne au hasard qui
// correspond à l'expression donnée, par exemple [A-Z]{3}-\d{4}. Les
// répétitions sans borne (*, +, {n,}) s'arrêtent à maxRepeat de plus que le
// minimum ; une classe niée ou très large (\D, [^a], .) tire ses caractères
// parmi les caractères imprimables courants qu'elle contient.

const maxRepeat = 8

// printable : lettres, chiffres, ponctuation ASCII, espace et lettres
// accentuées courantes du français.
var printable = func() []rune {
	var rs []rune
	for r := rune(0x20); r <= 0x7e; r++ {
		rs = append(rs, r)
	}
	return append(rs, []rune("àâäçéèêëîïôöùûüÿœæÀÂÇÉÈÊËÎÏÔÙÛÜŒÆ")...)
}()

// compileRegexGen analyse l'expression ; l'erreur est en français.
func compileRegexGen(pattern string) (*syntax.Regexp, error) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, fmt.Errorf("expression régulière invalide : %v", err)
	}
	return re.Simplify(), nil
}

// Regex rend une chaîne au hasard qui correspond à re.
func (g *Gen) Regex(re *syntax.Regexp) string {
	var b strings.Builder
	g.regex(&b, re)
	return b.String()
}

func (g *Gen) regex(b *strings.Builder, re *syntax.Regexp) {
	switch re.Op {
	case syntax.OpLiteral:
		for _, r := range re.Rune {
			if re.Flags&syntax.FoldCase != 0 && g.Chance(0.5) {
				r = unicode.SimpleFold(r)
			}
			b.WriteRune(r)
		}
	case syntax.OpCharClass:
		b.WriteRune(g.classRune(re.Rune))
	case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
		b.WriteRune(printable[g.Int(0, len(printable)-1)])
	case syntax.OpCapture:
		g.regex(b, re.Sub[0])
	case syntax.OpConcat:
		for _, s := range re.Sub {
			g.regex(b, s)
		}
	case syntax.OpAlternate:
		g.regex(b, re.Sub[g.Int(0, len(re.Sub)-1)])
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		lo, hi := re.Min, re.Max
		switch re.Op {
		case syntax.OpStar:
			lo, hi = 0, -1
		case syntax.OpPlus:
			lo, hi = 1, -1
		case syntax.OpQuest:
			lo, hi = 0, 1
		}
		if hi < 0 {
			hi = lo + maxRepeat
		}
		for range g.Int(lo, hi) {
			g.regex(b, re.Sub[0])
		}
	}
	// OpEmptyMatch, ancres (^ $ \b…) : rien à écrire.
}

// classRune tire un caractère d'une classe (paires de bornes) : parmi les
// caractères imprimables qu'elle contient si elle est large, sinon
// uniformément dans ses intervalles.
func (g *Gen) classRune(ranges []rune) rune {
	size := 0
	for i := 0; i+1 < len(ranges); i += 2 {
		size += int(ranges[i+1]-ranges[i]) + 1
	}
	in := func(r rune) bool {
		for i := 0; i+1 < len(ranges); i += 2 {
			if r >= ranges[i] && r <= ranges[i+1] {
				return true
			}
		}
		return false
	}
	if size > 256 {
		var pool []rune
		for _, r := range printable {
			if in(r) {
				pool = append(pool, r)
			}
		}
		if len(pool) > 0 {
			return pool[g.Int(0, len(pool)-1)]
		}
	}
	if size == 0 {
		return '?'
	}
	k := g.Int(0, size-1)
	for i := 0; i+1 < len(ranges); i += 2 {
		n := int(ranges[i+1]-ranges[i]) + 1
		if k < n {
			return ranges[i] + rune(k)
		}
		k -= n
	}
	return ranges[0]
}
