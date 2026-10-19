package canvas

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Lecture des coordonnées géographiques écrites à la main (graphique
// « Carte »). ParseCoord lit une cellule qui porte latitude et longitude ;
// ParseCoordAxis une cellule qui n'en porte qu'une (mode deux colonnes).
// Formes acceptées : décimal (« 48.8566, 2.3522 », « 48,8566;2,3522 »,
// « 48.8566 2.3522 »), degrés-minutes-secondes (« 48°51'24"N 2°21'03"E »,
// ′ ″, espaces, O ou W pour l'ouest), degrés-minutes décimales
// (« 48°51.4'N 2°21.05'E »), signes ou lettres N/S/E/O/W avant ou après,
// WKT « POINT(2.3522 48.8566) » (longitude d'abord) et « geo:48.8566,2.3522 ».

var (
	coordWKT     = regexp.MustCompile(`(?i)^(?:SRID=\d+\s*;\s*)?POINT\s*(?:ZM|Z|M)?\s*\(\s*([-+]?\d+(?:\.\d+)?)\s+([-+]?\d+(?:\.\d+)?)(?:\s+[-+]?\d+(?:\.\d+)?){0,2}\s*\)$`)
	coordDotDec  = regexp.MustCompile(`\d\.\d`)
	coordCommaIn = regexp.MustCompile(`\d,\d`)
)

// ParseCoord lit une latitude et une longitude dans un texte ; ok est faux
// si le texte est illisible ou hors bornes (|lat| > 90, |lon| > 180).
func ParseCoord(s string) (lat, lon float64, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	if len(s) > 4 && strings.EqualFold(s[:4], "geo:") {
		body := s[4:]
		if i := strings.IndexAny(body, ";?"); i >= 0 {
			body = body[:i]
		}
		parts := strings.Split(body, ",")
		if len(parts) < 2 || len(parts) > 3 {
			return 0, 0, false
		}
		a, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		b, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return coordCheck(a, b)
	}
	if m := coordWKT.FindStringSubmatch(s); m != nil {
		x, _ := strconv.ParseFloat(m[1], 64)
		y, _ := strconv.ParseFloat(m[2], 64)
		return coordCheck(y, x)
	}
	hasDot := coordDotDec.MatchString(s)
	// Virgule décimale : avec un séparateur franc (; | tabulation), ou sans
	// point décimal quand deux nombres en portent une (« 48,85 2,35 »).
	hard := strings.ContainsAny(s, ";|\t")
	toks, good := coordTokens(s, hard || (!hasDot && len(coordCommaIn.FindAllStringIndex(s, -1)) >= 2))
	if !good {
		return 0, 0, false
	}
	groups, good := coordGroups(toks)
	if !good || len(groups) != 2 {
		return 0, 0, false
	}
	a, ka, ok1 := coordAxis(groups[0])
	b, kb, ok2 := coordAxis(groups[1])
	if !ok1 || !ok2 || (ka != 0 && ka == kb) {
		return 0, 0, false
	}
	if ka == 'E' || kb == 'N' {
		a, b = b, a
	}
	return coordCheck(a, b)
}

// ParseCoordAxis lit une seule coordonnée (latitude si lat, longitude
// sinon) : nombre décimal (virgule ou point), ou degrés-minutes-secondes
// avec une lettre facultative, qui doit alors convenir à l'axe.
func ParseCoordAxis(s string, lat bool) (float64, bool) {
	v, k, ok := coordAxisText(s)
	if !ok || (lat && k == 'E') || (!lat && k == 'N') {
		return 0, false
	}
	if lat && math.Abs(v) > 90 || math.Abs(v) > 180 {
		return 0, false
	}
	return v, true
}

// coordAxisText lit une coordonnée seule, avec la sorte que dit sa lettre
// ('N' latitude, 'E' longitude, 0 sans lettre).
func coordAxisText(s string) (float64, byte, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	toks, good := coordTokens(s, !coordDotDec.MatchString(s))
	if !good {
		return 0, 0, false
	}
	var g []coordTok
	for _, t := range toks {
		if t.kind == '|' {
			return 0, 0, false
		}
		g = append(g, t)
	}
	// Lettre au milieu (« 48 N 2 ») : refusée.
	for i, t := range g {
		if t.kind == 'h' && i != 0 && i != len(g)-1 {
			return 0, 0, false
		}
	}
	return coordAxis(g)
}

func coordCheck(lat, lon float64) (float64, float64, bool) {
	if math.IsNaN(lat) || math.IsNaN(lon) || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
		return 0, 0, false
	}
	return lat, lon, true
}

// coordTok : nombre ('n'), unité ('u' : d, m ou s), signe ('-' : neg),
// lettre d'hémisphère ('h' : N, S, E ou W), séparateur ('|').
type coordTok struct {
	kind byte
	num  float64
	unit byte
	hemi byte
	neg  bool
}

var coordWords = map[string]byte{
	"n": 'N', "nord": 'N', "north": 'N',
	"s": 'S', "sud": 'S', "south": 'S',
	"e": 'E', "est": 'E', "east": 'E',
	"o": 'W', "w": 'W', "ouest": 'W', "west": 'W',
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// coordTokens découpe le texte ; commaDec : la virgule entre deux chiffres
// est décimale (sinon c'est un séparateur).
func coordTokens(s string, commaDec bool) ([]coordTok, bool) {
	rs := []rune(s)
	var out []coordTok
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case r == '\t' || r == ';' || r == '|' || r == '/' || r == ',':
			out = append(out, coordTok{kind: '|'})
			i++
		case unicode.IsSpace(r):
			i++
		case isDigit(r) || (r == '.' && i+1 < len(rs) && isDigit(rs[i+1])):
			j := i
			dec := false
			var b strings.Builder
			for j < len(rs) {
				c := rs[j]
				if isDigit(c) {
					b.WriteRune(c)
					j++
					continue
				}
				if !dec && (c == '.' || (c == ',' && commaDec)) && j+1 < len(rs) && isDigit(rs[j+1]) {
					dec = true
					b.WriteByte('.')
					j++
					continue
				}
				break
			}
			f, err := strconv.ParseFloat(b.String(), 64)
			if err != nil {
				return nil, false
			}
			out = append(out, coordTok{kind: 'n', num: f})
			i = j
		case r == '-' || r == '−' || r == '–':
			out = append(out, coordTok{kind: '-', neg: true})
			i++
		case r == '+':
			out = append(out, coordTok{kind: '-'})
			i++
		case r == '°' || r == 'º' || r == '˚':
			out = append(out, coordTok{kind: 'u', unit: 'd'})
			i++
		case r == '\'' || r == '′' || r == '’' || r == '‘' || r == '´':
			if i+1 < len(rs) && (rs[i+1] == '\'' || rs[i+1] == '′' || rs[i+1] == '’') {
				out = append(out, coordTok{kind: 'u', unit: 's'})
				i += 2
			} else {
				out = append(out, coordTok{kind: 'u', unit: 'm'})
				i++
			}
		case r == '"' || r == '″' || r == '”' || r == '“':
			out = append(out, coordTok{kind: 'u', unit: 's'})
			i++
		case unicode.IsLetter(r):
			j := i
			for j < len(rs) && unicode.IsLetter(rs[j]) {
				j++
			}
			h, ok := coordWords[strings.ToLower(string(rs[i:j]))]
			if !ok {
				return nil, false
			}
			out = append(out, coordTok{kind: 'h', hemi: h})
			i = j
		default:
			return nil, false
		}
	}
	return out, true
}

// coordGroups partage les jetons en coordonnées : d'après les lettres
// (avant ou après chaque coordonnée), sinon d'après un séparateur, sinon
// d'après les symboles de degré, sinon d'après le nombre de valeurs.
func coordGroups(toks []coordTok) ([][]coordTok, bool) {
	var hemis []int
	for i, t := range toks {
		if t.kind == 'h' {
			hemis = append(hemis, i)
		}
	}
	noSep := func(g []coordTok) []coordTok {
		var out []coordTok
		for _, t := range g {
			if t.kind != '|' {
				out = append(out, t)
			}
		}
		return out
	}
	trimSep := func(g []coordTok) ([]coordTok, bool) {
		for len(g) > 0 && g[0].kind == '|' {
			g = g[1:]
		}
		for len(g) > 0 && g[len(g)-1].kind == '|' {
			g = g[:len(g)-1]
		}
		for _, t := range g {
			if t.kind == '|' {
				return nil, false
			}
		}
		return g, len(g) > 0
	}
	switch len(hemis) {
	case 2:
		lead, _ := trimSep(toks[:hemis[0]+1])
		var parts [][]coordTok
		if len(lead) > 0 && lead[0].kind == 'h' {
			parts = [][]coordTok{toks[:hemis[1]], toks[hemis[1]:]}
		} else {
			parts = [][]coordTok{toks[:hemis[0]+1], toks[hemis[0]+1:]}
		}
		var out [][]coordTok
		for _, p := range parts {
			g, ok := trimSep(p)
			if !ok {
				return nil, false
			}
			out = append(out, g)
		}
		// Lettres d'un même côté de chaque coordonnée.
		if (out[0][0].kind == 'h') != (out[1][0].kind == 'h') {
			return nil, false
		}
		return out, true
	case 0:
	default:
		return nil, false
	}
	// Séparateur franc.
	var out [][]coordTok
	var cur []coordTok
	seps := false
	for _, t := range toks {
		if t.kind == '|' {
			seps = true
			if len(cur) > 0 {
				out = append(out, cur)
			}
			cur = nil
			continue
		}
		cur = append(cur, t)
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	if seps {
		return out, true
	}
	toks = noSep(toks)
	// Symboles de degré : chaque nombre suivi de ° ouvre une coordonnée
	// (avec son signe éventuel).
	degs := 0
	for i, t := range toks {
		if t.kind == 'u' && t.unit == 'd' && i > 0 && toks[i-1].kind == 'n' {
			degs++
		}
	}
	if degs == 2 {
		out = nil
		cur = nil
		for i, t := range toks {
			starts := t.kind == 'n' && i+1 < len(toks) && toks[i+1].kind == 'u' && toks[i+1].unit == 'd'
			if t.kind == '-' && i+2 < len(toks) && toks[i+1].kind == 'n' && toks[i+2].kind == 'u' && toks[i+2].unit == 'd' {
				starts = true
			}
			if starts && i > 0 && toks[i-1].kind == '-' {
				starts = false // déjà ouvert par le signe
			}
			if starts && len(cur) > 0 {
				out = append(out, cur)
				cur = nil
			}
			cur = append(cur, t)
		}
		if len(cur) > 0 {
			out = append(out, cur)
		}
		return out, true
	}
	if degs != 0 {
		return nil, false
	}
	// Nombres seuls : 2 (décimal), 4 (degrés minutes) ou 6 (DMS).
	var nums []int
	for i, t := range toks {
		switch t.kind {
		case 'n':
			nums = append(nums, i)
		case 'u':
			return nil, false
		}
	}
	if len(nums) != 2 && len(nums) != 4 && len(nums) != 6 {
		return nil, false
	}
	cut := nums[len(nums)/2]
	if cut > 0 && toks[cut-1].kind == '-' {
		cut--
	}
	return [][]coordTok{toks[:cut], toks[cut:]}, true
}

// coordAxis calcule une coordonnée : signe ou lettre, puis degrés, minutes
// et secondes (repérés par leur symbole, sinon par leur rang). kind vaut
// 'N' (latitude), 'E' (longitude) ou 0 (sans lettre).
func coordAxis(g []coordTok) (float64, byte, bool) {
	var hemi byte
	if len(g) > 0 && g[0].kind == 'h' {
		hemi, g = g[0].hemi, g[1:]
	} else if len(g) > 0 && g[len(g)-1].kind == 'h' {
		hemi, g = g[len(g)-1].hemi, g[:len(g)-1]
	}
	neg := false
	if len(g) > 0 && g[0].kind == '-' {
		neg, g = g[0].neg, g[1:]
	}
	var parts [3]float64
	var have [3]bool
	slot := -1
	for i := 0; i < len(g); i++ {
		t := g[i]
		if t.kind != 'n' {
			return 0, 0, false
		}
		s := slot + 1
		if i+1 < len(g) && g[i+1].kind == 'u' {
			s = map[byte]int{'d': 0, 'm': 1, 's': 2}[g[i+1].unit]
			i++
		}
		if s <= slot || s > 2 {
			return 0, 0, false
		}
		slot = s
		parts[s], have[s] = t.num, true
	}
	if !have[0] {
		return 0, 0, false
	}
	frac := func(f float64) bool { return f != math.Trunc(f) }
	if have[1] || have[2] {
		if frac(parts[0]) || parts[1] >= 60 || parts[2] >= 60 {
			return 0, 0, false
		}
		if have[2] && frac(parts[1]) {
			return 0, 0, false
		}
	}
	v := parts[0] + parts[1]/60 + parts[2]/3600
	if neg || hemi == 'S' || hemi == 'W' {
		v = -v
	}
	switch hemi {
	case 'N', 'S':
		return v, 'N', true
	case 'E', 'W':
		return v, 'E', true
	}
	return v, 0, true
}
