package formula

import (
	"math"
	"strings"

	"github.com/lalamefine/lattis/internal/geo"
)

// COMMUNE(latitude; longitude; [info]) : commune française qui contient un
// point GPS, hors ligne (contours embarqués, voir internal/geo). Fonction
// scalaire : la même implémentation sert à l'évaluateur Go et, par slt_fn, au
// SQL compilé.
func fnMunicipality(a []Value) Value {
	lat, e := coordArg(a[0], 90)
	if e.IsErr() {
		return e
	}
	lon, e := coordArg(a[1], 180)
	if e.IsErr() {
		return e
	}
	info := "nom"
	if len(a) > 2 && !a[2].IsEmpty() {
		if a[2].IsErr() {
			return a[2]
		}
		if a[2].Kind != Text {
			return Err(ErrValue)
		}
		info = normInfo(a[2].Str)
	}
	pick, ok := communeInfo[info]
	if !ok {
		return Err(ErrValue)
	}
	if lat.IsEmpty() || lon.IsEmpty() {
		return Err(ErrNA) // position inconnue (cellule vide)
	}
	c, found := geo.Lookup(lat.Num, lon.Num)
	if !found {
		return Err(ErrNA)
	}
	if s := pick(c); s != "" {
		return Str(s)
	}
	return Err(ErrNA) // information absente (code postal, région d'une COM)
}

// coordArg lit une coordonnée en degrés décimaux : nombre, ou texte « 48,85 »
// / « 48.85 ». Vide : renvoyé tel quel (#N/A plus loin).
func coordArg(v Value, limit float64) (Value, Value) {
	switch v.Kind {
	case Error:
		return Value{}, v
	case Empty:
		return v, Value{}
	case Number:
	case Text:
		if strings.TrimSpace(v.Str) == "" {
			return Value{}, Value{}
		}
		f, ok := parseNumber(v.Str)
		if !ok {
			return Value{}, Err(ErrValue)
		}
		v = Num(f)
	default: // booléen, date
		return Value{}, Err(ErrValue)
	}
	if math.IsNaN(v.Num) || math.Abs(v.Num) > limit {
		return Value{}, Err(ErrValue)
	}
	return v, Value{}
}

// normInfo : minuscules, sans accents ni séparateurs (« Code_Postal » →
// « codepostal », « département » → « departement »).
func normInfo(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch r {
		case ' ', '_', '-', '.':
			continue
		case 'é', 'è', 'ê':
			r = 'e'
		}
		b.WriteRune(r)
	}
	return b.String()
}

var communeInfo = map[string]func(*geo.Commune) string{
	"nom":             func(c *geo.Commune) string { return c.Name },
	"name":            func(c *geo.Commune) string { return c.Name },
	"code":            func(c *geo.Commune) string { return c.Code },
	"insee":           func(c *geo.Commune) string { return c.Code },
	"codeinsee":       func(c *geo.Commune) string { return c.Code },
	"cp":              func(c *geo.Commune) string { return strings.Join(c.PostalCodes, ", ") },
	"codepostal":      func(c *geo.Commune) string { return strings.Join(c.PostalCodes, ", ") },
	"postcode":        func(c *geo.Commune) string { return strings.Join(c.PostalCodes, ", ") },
	"departement":     func(c *geo.Commune) string { return c.DeptName },
	"department":      func(c *geo.Commune) string { return c.DeptName },
	"codedepartement": func(c *geo.Commune) string { return c.DeptCode },
	"dep":             func(c *geo.Commune) string { return c.DeptCode },
	"region":          func(c *geo.Commune) string { return c.RegionName },
	"coderegion":      func(c *geo.Commune) string { return c.RegionCode },
}
