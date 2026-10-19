// Package geo retrouve la commune française qui contient un point GPS, hors
// ligne : les contours simplifiés des communes (métropole et outre-mer) sont
// embarqués dans l'exécutable (communes.bin.gz, fabriqué par ./gen) et chargés
// au premier appel.
//
// La réponse est exacte à la simplification près (contours à ~50 m, puis
// Douglas-Peucker à 40 m) : un point à quelques dizaines de mètres d'une
// limite peut tomber chez la voisine. Un point juste hors de tout contour
// (côte, jetée, arrondis) prend la commune la plus proche jusqu'à
// MaxDistance ; au-delà (en mer, à l'étranger), pas de commune.
package geo

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
)

//go:generate go run ./gen

//go:embed communes.bin.gz
var data []byte

// Attribution : mention exigée par la Licence Ouverte, à afficher avec les
// résultats (aide de la fonction COMMUNE, documentation).
const Attribution = "Contient des données IGN et INSEE (contours des communes ADMIN EXPRESS, " +
	"diffusés par Etalab) et des codes postaux de La Poste via geo.api.gouv.fr, " +
	"Licence Ouverte / Etalab 2.0."

// MaxDistance : distance (mètres) jusqu'à laquelle un point hors de tout
// contour prend la commune la plus proche.
const MaxDistance = 1000.0

// Commune : ce que l'on sait de la commune trouvée.
type Commune struct {
	Name        string   // « Paris »
	Code        string   // code INSEE (COG), « 75056 »
	PostalCodes []string // codes postaux, triés (plusieurs pour les grandes villes)
	DeptCode    string   // « 75 », « 2A », « 971 »
	DeptName    string
	RegionCode  string // vide pour les collectivités d'outre-mer sans région
	RegionName  string
}

type entry struct {
	info                   *Commune
	arcs                   []int32 // index d'arcs (le sens ne compte pas)
	minX, minY, maxX, maxY int32
}

type base struct {
	source   string
	pts      []int32 // x, y entrelacés, en 1e-5 degré
	arcStart []int32 // arc i : points arcStart[i]..arcStart[i+1] (en paires)
	ents     []entry
	grid     map[int64][]int32
}

// cell : taille d'une case de l'index, en 1e-5 degré (0,05°, ≈ 5 km).
const cell = 5000

func cellKey(cx, cy int32) int64 { return int64(cx)<<32 | int64(uint32(cy)) }

func cellOf(v int32) int32 {
	if v < 0 {
		return (v+1)/cell - 1
	}
	return v / cell
}

var (
	once    sync.Once
	loaded  *base
	loadErr error
)

func get() (*base, error) {
	once.Do(func() { loaded, loadErr = load(data) })
	return loaded, loadErr
}

// Source décrit le millésime des données embarquées.
func Source() string {
	b, err := get()
	if err != nil {
		return ""
	}
	return b.source
}

// Count : nombre de communes embarquées.
func Count() int {
	b, err := get()
	if err != nil {
		return 0
	}
	return len(b.ents)
}

// Lookup renvoie la commune qui contient le point (degrés décimaux, WGS84).
func Lookup(lat, lon float64) (*Commune, bool) {
	b, err := get()
	if err != nil || math.IsNaN(lat) || math.IsNaN(lon) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return nil, false
	}
	return b.lookup(lat, lon)
}

func (b *base) lookup(lat, lon float64) (*Commune, bool) {
	px, py := lon*1e5, lat*1e5
	ix, iy := int32(math.Floor(px)), int32(math.Floor(py))
	for _, i := range b.grid[cellKey(cellOf(ix), cellOf(iy))] {
		e := &b.ents[i]
		if ix < e.minX-1 || ix > e.maxX || iy < e.minY-1 || iy > e.maxY {
			continue
		}
		if b.inside(e, px, py) {
			return e.info, true
		}
	}
	return b.nearest(lat, px, py)
}

// inside : règle pair-impair sur tous les segments de la commune (les trous
// sont des anneaux comme les autres).
func (b *base) inside(e *entry, px, py float64) bool {
	in := false
	for _, a := range e.arcs {
		s, t := b.arcStart[a], b.arcStart[a+1]
		p := b.pts[2*s : 2*t]
		for k := 2; k < len(p); k += 2 {
			ax, ay := float64(p[k-2]), float64(p[k-1])
			bx, by := float64(p[k]), float64(p[k+1])
			if (ay > py) != (by > py) && px < ax+(py-ay)*(bx-ax)/(by-ay) {
				in = !in
			}
		}
	}
	return in
}

// nearest : commune dont la limite est la plus proche, à MaxDistance au plus.
func (b *base) nearest(lat, px, py float64) (*Commune, bool) {
	ky := 1.1119492664 // mètres par 1e-5 degré de latitude
	kx := ky * math.Cos(lat*math.Pi/180)
	if kx < 0.01 {
		return nil, false
	}
	rx, ry := MaxDistance/kx, MaxDistance/ky // rayon en 1e-5 degré
	best, found := MaxDistance, -1
	seen := map[int32]bool{}
	for cx := cellOf(int32(math.Floor(px - rx))); cx <= cellOf(int32(math.Floor(px+rx))); cx++ {
		for cy := cellOf(int32(math.Floor(py - ry))); cy <= cellOf(int32(math.Floor(py+ry))); cy++ {
			for _, i := range b.grid[cellKey(cx, cy)] {
				if seen[i] {
					continue
				}
				seen[i] = true
				e := &b.ents[i]
				if px < float64(e.minX)-rx || px > float64(e.maxX)+rx || py < float64(e.minY)-ry || py > float64(e.maxY)+ry {
					continue
				}
				if d := b.distance(e, px, py, kx, ky, best); d <= best {
					best, found = d, int(i)
				}
			}
		}
	}
	if found < 0 {
		return nil, false
	}
	return b.ents[found].info, true
}

// distance (mètres) du point à la limite de la commune, ou une valeur
// supérieure à limit quand elle est plus loin.
func (b *base) distance(e *entry, px, py, kx, ky, limit float64) float64 {
	best := math.Inf(1)
	for _, a := range e.arcs {
		s, t := b.arcStart[a], b.arcStart[a+1]
		p := b.pts[2*s : 2*t]
		for k := 2; k < len(p); k += 2 {
			ax, ay := (float64(p[k-2])-px)*kx, (float64(p[k-1])-py)*ky
			bx, by := (float64(p[k])-px)*kx, (float64(p[k+1])-py)*ky
			if math.Min(ax, bx) > limit || math.Max(ax, bx) < -limit || math.Min(ay, by) > limit || math.Max(ay, by) < -limit {
				continue
			}
			dx, dy := bx-ax, by-ay
			l := dx*dx + dy*dy
			tt := 0.0
			if l > 0 {
				tt = max(0, min(1, -(ax*dx+ay*dy)/l))
			}
			if d := math.Hypot(ax+tt*dx, ay+tt*dy); d < best {
				best = d
			}
		}
	}
	return best
}

var errFormat = errors.New("geo : fichier des communes illisible")

func load(gz []byte) (*base, error) {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	r := reader{b: raw}
	if len(raw) < 4 || string(raw[:4]) != "LGEO" {
		return nil, errFormat
	}
	r.b = raw[4:]
	if v := r.uv(); v != 1 {
		return nil, fmt.Errorf("geo : version %d du fichier des communes inconnue", v)
	}
	b := &base{source: r.str(), grid: map[int64][]int32{}}
	type region struct{ code, name string }
	regions := make([]region, r.count())
	for i := range regions {
		regions[i] = region{r.str(), r.str()}
	}
	type dept struct {
		code, name string
		reg        int
	}
	depts := make([]dept, r.count())
	for i := range depts {
		depts[i] = dept{r.str(), r.str(), int(r.uv())}
		if depts[i].reg > len(regions) {
			return nil, errFormat
		}
	}
	nArcs := r.count()
	b.arcStart = make([]int32, nArcs+1)
	b.pts = make([]int32, 0, 2*1_200_000)
	for i := range nArcs {
		n := r.count()
		var x, y int32
		for range n {
			x += int32(r.zz())
			y += int32(r.zz())
			b.pts = append(b.pts, x, y)
		}
		b.arcStart[i+1] = int32(len(b.pts) / 2)
	}
	nCom := r.count()
	infos := make([]Commune, nCom)
	b.ents = make([]entry, nCom)
	for i := range nCom {
		c := &infos[i]
		c.Code, c.Name = r.str(), r.str()
		di := int(r.uv())
		if di >= len(depts) {
			return nil, errFormat
		}
		d := depts[di]
		c.DeptCode, c.DeptName = d.code, d.name
		if d.reg > 0 {
			c.RegionCode, c.RegionName = regions[d.reg-1].code, regions[d.reg-1].name
		}
		if n := r.count(); n > 0 {
			c.PostalCodes = make([]string, n)
			for k := range c.PostalCodes {
				c.PostalCodes[k] = r.str()
			}
		}
		e := &b.ents[i]
		e.info = c
		e.arcs = make([]int32, r.count())
		e.minX, e.minY, e.maxX, e.maxY = math.MaxInt32, math.MaxInt32, math.MinInt32, math.MinInt32
		for k := range e.arcs {
			a := r.zz()
			if a < 0 {
				a = -a
			}
			if a < 1 || int(a) > nArcs {
				return nil, errFormat
			}
			e.arcs[k] = int32(a - 1)
			for p := b.arcStart[a-1]; p < b.arcStart[a]; p++ {
				x, y := b.pts[2*p], b.pts[2*p+1]
				e.minX, e.maxX = min(e.minX, x), max(e.maxX, x)
				e.minY, e.maxY = min(e.minY, y), max(e.maxY, y)
			}
		}
		for cx := cellOf(e.minX); cx <= cellOf(e.maxX); cx++ {
			for cy := cellOf(e.minY); cy <= cellOf(e.maxY); cy++ {
				k := cellKey(cx, cy)
				b.grid[k] = append(b.grid[k], int32(i))
			}
		}
	}
	if r.err {
		return nil, errFormat
	}
	return b, nil
}

type reader struct {
	b   []byte
	err bool
}

func (r *reader) uv() uint64 {
	v, n := binary.Uvarint(r.b)
	if n <= 0 {
		r.err, r.b = true, nil
		return 0
	}
	r.b = r.b[n:]
	return v
}

func (r *reader) zz() int64 {
	v, n := binary.Varint(r.b)
	if n <= 0 {
		r.err, r.b = true, nil
		return 0
	}
	r.b = r.b[n:]
	return v
}

// count lit un effectif, borné par ce qui reste à lire (fichier abîmé).
func (r *reader) count() int {
	v := r.uv()
	if v > uint64(len(r.b)) {
		r.err, r.b = true, nil
		return 0
	}
	return int(v)
}

func (r *reader) str() string {
	n := r.count()
	s := string(r.b[:n])
	r.b = r.b[n:]
	return s
}
