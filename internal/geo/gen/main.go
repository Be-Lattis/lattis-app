// Command gen fabrique internal/geo/communes.bin.gz, le fichier des contours
// des communes françaises embarqué dans l'application (go generate dans
// internal/geo).
//
// Sources (Licence Ouverte / Etalab 2.0, données IGN ADMIN EXPRESS et INSEE) :
//
//   - contours des communes, simplifiés à 50 m par Etalab :
//     https://etalab-datasets.geo.data.gouv.fr/contours-administratifs/2026/geojson/communes-50m.geojson.gz
//     (millésime 2026, fichier du 2026-05-04, téléchargé le 2026-10-18) ;
//   - codes postaux, départements et régions, API Découpage administratif :
//     https://geo.api.gouv.fr/communes, /departements et /regions.
//
// Les téléchargements vont dans un dossier de cache hors du dépôt (-cache) et
// ne sont jamais versionnés. Le traitement :
//
//  1. découpe les contours en arcs entre nœuds (sommets partagés par trois
//     arêtes ou plus) : la frontière commune à deux communes n'est stockée et
//     simplifiée qu'une fois, sans trou ni recouvrement entre voisines ;
//  2. simplifie chaque arc par Douglas-Peucker (-tol, en mètres) ;
//  3. quantifie les coordonnées (-q, en cent-millièmes de degré) et les écrit
//     en différences successives (varints), le tout compressé en gzip.
//
// Format (tous les entiers en varint, zz = zigzag) :
//
//	"LGEO" version(1) source(str)
//	nRégions × (code str, nom str)
//	nDépartements × (code str, nom str, régionIdx+1 ; 0 : sans région)
//	nArcs × (nPoints, x0 zz, y0 zz, (dx zz, dy zz)…)   unités : 1e-5 degré
//	nCommunes × (code str, nom str, départementIdx, nCP × str, nArcs × (arc+1) zz (négatif : arc parcouru à l'envers))
//
// Un anneau de commune est la suite de ses arcs ; les anneaux d'une même
// commune se suivent. Le test d'appartenance (pair-impair) n'a besoin que des
// segments, pas de l'ordre.
package main

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	contoursURL = "https://etalab-datasets.geo.data.gouv.fr/contours-administratifs/2026/geojson/communes-50m.geojson.gz"
	contoursTag = "Etalab contours-administratifs 2026 communes-50m (2026-05-04)"
	communesURL = "https://geo.api.gouv.fr/communes?fields=code,nom,codesPostaux,codeDepartement,codeRegion&format=json"
	deptsURL    = "https://geo.api.gouv.fr/departements"
	regionsURL  = "https://geo.api.gouv.fr/regions"
	fetched     = "2026-10-18" // date des téléchargements
)

type pt struct{ x, y int32 } // 1e-5 degré

func main() {
	cache := flag.String("cache", filepath.Join(os.TempDir(), "lattis-geo"), "dossier des téléchargements (hors du dépôt)")
	out := flag.String("out", "communes.bin.gz", "fichier produit")
	tol := flag.Float64("tol", 40, "tolérance de Douglas-Peucker, en mètres")
	q := flag.Int("q", 10, "pas de quantification, en 1e-5 degré (10 ≈ 1 m)")
	flag.Parse()
	if err := os.MkdirAll(*cache, 0o755); err != nil {
		log.Fatal(err)
	}
	geo := fetch(*cache, "communes-50m.geojson.gz", contoursURL)
	meta := fetch(*cache, "communes.json", communesURL)
	depts := fetch(*cache, "departements.json", deptsURL)
	regions := fetch(*cache, "regions.json", regionsURL)

	feats := readGeoJSON(geo)
	log.Printf("%d communes lues", len(feats))
	b := build(feats, *tol, int32(*q))
	data := encode(b, readMeta(meta, depts, regions))
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s : %d octets (%d arcs, %d points)", *out, len(data), len(b.arcs), b.points)
}

// fetch télécharge url dans le cache s'il n'y est pas déjà.
func fetch(dir, name, url string) string {
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	log.Printf("téléchargement de %s", url)
	cl := http.Client{Timeout: 5 * time.Minute}
	resp, err := cl.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatalf("%s : %s", url, resp.Status)
	}
	f, err := os.Create(p + ".part")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		log.Fatal(err)
	}
	f.Close()
	if err := os.Rename(p+".part", p); err != nil {
		log.Fatal(err)
	}
	return p
}

type feature struct {
	code, nom, dept string
	rings           [][]pt
}

func readGeoJSON(path string) []feature {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(bufio.NewReader(f))
	if err != nil {
		log.Fatal(err)
	}
	var fc struct {
		Features []struct {
			Properties struct {
				Code, Nom, Departement string
			}
			Geometry struct {
				Type        string
				Coordinates json.RawMessage
			}
		}
	}
	if err := json.NewDecoder(zr).Decode(&fc); err != nil {
		log.Fatal(err)
	}
	conv := func(r [][2]float64) []pt {
		o := make([]pt, 0, len(r))
		for _, c := range r {
			p := pt{int32(math.Round(c[0] * 1e5)), int32(math.Round(c[1] * 1e5))}
			if len(o) > 0 && o[len(o)-1] == p {
				continue
			}
			o = append(o, p)
		}
		if len(o) > 1 && o[0] == o[len(o)-1] {
			o = o[:len(o)-1] // anneau ouvert : le dernier point rejoint le premier
		}
		return o
	}
	var out []feature
	for _, x := range fc.Features {
		ft := feature{code: x.Properties.Code, nom: x.Properties.Nom, dept: x.Properties.Departement}
		var polys [][][][2]float64
		switch x.Geometry.Type {
		case "Polygon":
			var p [][][2]float64
			if err := json.Unmarshal(x.Geometry.Coordinates, &p); err != nil {
				log.Fatal(err)
			}
			polys = [][][][2]float64{p}
		case "MultiPolygon":
			if err := json.Unmarshal(x.Geometry.Coordinates, &polys); err != nil {
				log.Fatal(err)
			}
		default:
			log.Printf("%s : géométrie %q ignorée", ft.code, x.Geometry.Type)
			continue
		}
		for _, p := range polys {
			for _, r := range p {
				if c := conv(r); len(c) >= 3 {
					ft.rings = append(ft.rings, c)
				}
			}
		}
		if len(ft.rings) > 0 {
			out = append(out, ft)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].code < out[j].code })
	return out
}

// arcRef : arc parcouru dans un sens ou dans l'autre.
type arcRef struct {
	arc int
	rev bool
}

type built struct {
	feats  []feature
	arcs   [][]pt
	refs   [][]arcRef // par commune, anneaux mis bout à bout
	points int
}

func less(a, b pt) bool { return a.x < b.x || a.x == b.x && a.y < b.y }

func build(feats []feature, tolM float64, q int32) built {
	// Degré de chaque sommet dans le graphe des arêtes (sans doublons).
	type edge struct{ a, b pt }
	norm := func(a, b pt) edge {
		if less(b, a) {
			a, b = b, a
		}
		return edge{a, b}
	}
	edges := map[edge]bool{}
	for _, f := range feats {
		for _, r := range f.rings {
			for i := range r {
				edges[norm(r[i], r[(i+1)%len(r)])] = true
			}
		}
	}
	deg := map[pt]int{}
	for e := range edges {
		deg[e.a]++
		deg[e.b]++
	}
	edges = nil
	isNode := func(p pt) bool { return deg[p] > 2 }

	b := built{feats: feats}
	arcIdx := map[string]int{} // arc canonique (clé : points) → index
	key := func(s []pt) string {
		var sb strings.Builder
		for _, p := range s {
			fmt.Fprintf(&sb, "%d,%d;", p.x, p.y)
		}
		return sb.String()
	}
	addArc := func(s []pt) arcRef {
		// Sens canonique : celui des deux qui commence par le plus petit
		// point (puis le plus petit second point si l'arc est fermé).
		rev := false
		n := len(s)
		switch {
		case less(s[n-1], s[0]):
			rev = true
		case s[n-1] == s[0] && n > 2 && less(s[n-2], s[1]):
			rev = true
		}
		c := s
		if rev {
			c = make([]pt, n)
			for i := range s {
				c[i] = s[n-1-i]
			}
		}
		k := key(c)
		if i, ok := arcIdx[k]; ok {
			return arcRef{i, rev}
		}
		i := len(b.arcs)
		arcIdx[k] = i
		b.arcs = append(b.arcs, c)
		return arcRef{i, rev}
	}
	for _, f := range feats {
		var refs []arcRef
		for _, r := range f.rings {
			n := len(r)
			// Premier nœud de l'anneau ; à défaut (île, enclave), le plus
			// petit point, choisi de la même façon des deux côtés.
			start := -1
			for i, p := range r {
				if isNode(p) {
					start = i
					break
				}
			}
			if start < 0 {
				start = 0
				for i, p := range r {
					if less(p, r[start]) {
						start = i
					}
				}
			}
			cur := []pt{r[start]}
			for k := 1; k <= n; k++ {
				p := r[(start+k)%n]
				cur = append(cur, p)
				if k == n || isNode(p) {
					refs = append(refs, addArc(cur))
					cur = []pt{p}
				}
			}
		}
		b.refs = append(b.refs, refs)
	}
	log.Printf("%d arcs avant simplification", len(b.arcs))
	for i, a := range b.arcs {
		s := simplify(a, tolM)
		for j := range s {
			s[j] = pt{roundTo(s[j].x, q), roundTo(s[j].y, q)}
		}
		b.arcs[i] = dedup(s)
		b.points += len(b.arcs[i])
	}
	return b
}

func roundTo(v, q int32) int32 {
	if q <= 1 {
		return v
	}
	return int32(math.Round(float64(v)/float64(q))) * q
}

func dedup(s []pt) []pt {
	o := s[:1]
	for _, p := range s[1:] {
		if p != o[len(o)-1] {
			o = append(o, p)
		}
	}
	if len(o) == 1 && len(s) > 1 {
		o = append(o, o[0]) // arc réduit à un point : garde ses deux bouts
	}
	return o
}

// simplify : Douglas-Peucker en mètres (projection équirectangulaire locale).
// Les extrémités sont gardées ; un arc fermé est coupé en son point le plus
// éloigné du départ pour rester un anneau.
func simplify(s []pt, tolM float64) []pt {
	if len(s) <= 2 || tolM <= 0 {
		return append([]pt(nil), s...)
	}
	lat := float64(s[0].y) / 1e5 * math.Pi / 180
	kx := math.Cos(lat) * 1.11195 // mètres par 1e-5 degré
	const ky = 1.11195
	xy := func(p pt) (float64, float64) { return float64(p.x) * kx, float64(p.y) * ky }
	keep := make([]bool, len(s))
	keep[0], keep[len(s)-1] = true, true
	var rec func(i, j int)
	rec = func(i, j int) {
		if j <= i+1 {
			return
		}
		ax, ay := xy(s[i])
		bx, by := xy(s[j])
		best, bi := -1.0, -1
		for k := i + 1; k < j; k++ {
			px, py := xy(s[k])
			d := segDist(px, py, ax, ay, bx, by)
			if d > best {
				best, bi = d, k
			}
		}
		if best > tolM {
			keep[bi] = true
			rec(i, bi)
			rec(bi, j)
		}
	}
	if s[0] == s[len(s)-1] {
		// Arc fermé : le point le plus éloigné du départ, puis chaque moitié ;
		// on garde aussi un troisième point pour que l'anneau ne s'aplatisse pas.
		ax, ay := xy(s[0])
		far, fi := -1.0, 1
		for k := 1; k < len(s)-1; k++ {
			px, py := xy(s[k])
			if d := math.Hypot(px-ax, py-ay); d > far {
				far, fi = d, k
			}
		}
		keep[fi] = true
		rec(0, fi)
		rec(fi, len(s)-1)
		n := 0
		for _, k := range keep {
			if k {
				n++
			}
		}
		if n < 4 { // départ, opposé, retour : ajoute le point le plus loin de la corde
			bx, by := xy(s[fi])
			best, bi := -1.0, -1
			for k := 1; k < len(s)-1; k++ {
				if k == fi {
					continue
				}
				px, py := xy(s[k])
				if d := segDist(px, py, ax, ay, bx, by); d > best {
					best, bi = d, k
				}
			}
			if bi > 0 {
				keep[bi] = true
			}
		}
	} else {
		rec(0, len(s)-1)
	}
	var o []pt
	for k, p := range s {
		if keep[k] {
			o = append(o, p)
		}
	}
	return o
}

func segDist(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	l := dx*dx + dy*dy
	if l == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / l
	t = max(0, min(1, t))
	return math.Hypot(px-ax-t*dx, py-ay-t*dy)
}

type meta struct {
	cp      map[string][]string // code INSEE → codes postaux
	deptReg map[string]string   // département → région
	depts   [][2]string         // code, nom
	regions [][2]string
}

func readMeta(communes, depts, regions string) meta {
	read := func(p string, v any) {
		b, err := os.ReadFile(p)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal(b, v); err != nil {
			log.Fatal(err)
		}
	}
	var cs []struct {
		Code         string
		CodesPostaux []string
	}
	var ds []struct{ Code, Nom, CodeRegion string }
	var rs []struct{ Code, Nom string }
	read(communes, &cs)
	read(depts, &ds)
	read(regions, &rs)
	m := meta{cp: map[string][]string{}, deptReg: map[string]string{}}
	for _, c := range cs {
		cp := append([]string(nil), c.CodesPostaux...)
		sort.Strings(cp)
		m.cp[c.Code] = cp
	}
	for _, d := range ds {
		m.depts = append(m.depts, [2]string{d.Code, d.Nom})
		m.deptReg[d.Code] = d.CodeRegion
	}
	for _, r := range rs {
		m.regions = append(m.regions, [2]string{r.Code, r.Nom})
	}
	// Collectivités d'outre-mer présentes dans les contours mais absentes de
	// /departements : sans région.
	for _, d := range [][2]string{{"975", "Saint-Pierre-et-Miquelon"}, {"977", "Saint-Barthélemy"}, {"978", "Saint-Martin"},
		{"984", "Terres australes et antarctiques françaises"}, {"986", "Wallis-et-Futuna"}, {"987", "Polynésie française"},
		{"988", "Nouvelle-Calédonie"}, {"989", "Île de Clipperton"}} {
		if _, ok := m.deptReg[d[0]]; !ok {
			m.depts = append(m.depts, d)
			m.deptReg[d[0]] = ""
		}
	}
	return m
}

func encode(b built, m meta) []byte {
	var buf []byte
	uv := func(v uint64) { buf = binary.AppendUvarint(buf, v) }
	zz := func(v int64) { buf = binary.AppendVarint(buf, v) }
	str := func(s string) { uv(uint64(len(s))); buf = append(buf, s...) }
	buf = append(buf, "LGEO"...)
	uv(1)
	str(contoursTag + " ; geo.api.gouv.fr (" + fetched + ")")
	regIdx := map[string]int{}
	uv(uint64(len(m.regions)))
	for i, r := range m.regions {
		regIdx[r[0]] = i
		str(r[0])
		str(r[1])
	}
	deptIdx := map[string]int{}
	uv(uint64(len(m.depts)))
	for i, d := range m.depts {
		deptIdx[d[0]] = i
		str(d[0])
		str(d[1])
		ri, ok := regIdx[m.deptReg[d[0]]]
		if !ok && m.deptReg[d[0]] != "" {
			log.Fatalf("département %s : région %q inconnue", d[0], m.deptReg[d[0]])
		}
		if ok {
			uv(uint64(ri + 1)) // 0 : sans région
		} else {
			uv(0)
		}
	}
	uv(uint64(len(b.arcs)))
	for _, a := range b.arcs {
		uv(uint64(len(a)))
		var px, py int32
		for _, p := range a {
			zz(int64(p.x - px))
			zz(int64(p.y - py))
			px, py = p.x, p.y
		}
	}
	uv(uint64(len(b.feats)))
	missingCP := 0
	for i, f := range b.feats {
		str(f.code)
		str(f.nom)
		di, ok := deptIdx[f.dept]
		if !ok {
			log.Fatalf("%s : département %q inconnu", f.code, f.dept)
		}
		uv(uint64(di))
		cp, ok := m.cp[f.code]
		if !ok {
			missingCP++
		}
		uv(uint64(len(cp)))
		for _, c := range cp {
			str(c)
		}
		uv(uint64(len(b.refs[i])))
		for _, r := range b.refs[i] {
			v := int64(r.arc + 1)
			if r.rev {
				v = -v
			}
			zz(v)
		}
	}
	if missingCP > 0 {
		log.Printf("%d communes sans code postal connu", missingCP)
	}
	var out strings.Builder
	zw, _ := gzip.NewWriterLevel(&out, gzip.BestCompression)
	zw.Write(buf)
	zw.Close()
	log.Printf("brut %d octets", len(buf))
	return []byte(out.String())
}
