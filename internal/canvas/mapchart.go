package canvas

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/lalamefine/lattis/internal/formula"
)

// MapGuess : colonnes de coordonnées repérées dans un tableau, proposées à
// une carte en attente. Mode : « pair » (Lat et Lon), « single » (Coord) ou
// vide (rien de sûr). Name : colonne de texte pour le nom des points.
type MapGuess struct {
	Mode  string `json:"mode"`
	Lat   string `json:"lat,omitempty"`
	Lon   string `json:"lon,omitempty"`
	Coord string `json:"coord,omitempty"`
	Name  string `json:"name,omitempty"`
}

// mapSample : lignes lues pour repérer les coordonnées.
const mapSample = 50

// mapData lit les points d'une carte : séries latitude, longitude et
// valeur (facultative), quel que soit le mode. Les coordonnées illisibles
// ou hors bornes sont comptées et ignorées ; une ligne sans coordonnées
// (cellules vides) est ignorée sans être comptée. Les colonnes repérées
// accompagnent toujours la carte : le volet les propose quand on change de
// mode de coordonnées.
func (t *tx) mapData(cfg ChartConfig, tb Table) (chartResult, error) {
	exprs, args, _, err := t.columnSQL(tb)
	if err != nil {
		return chartResult{}, err
	}
	g, err := t.mapGuess(tb, exprs, args)
	if err != nil {
		return chartResult{}, err
	}
	if cfg.MapDraft {
		return chartResult{labels: []string{}, series: []ChartSeries{}, guess: g}, nil
	}
	find := func(name string) (int, error) {
		for i, c := range tb.Cols {
			if strings.EqualFold(c.Name, name) {
				return i, nil
			}
		}
		return 0, fmt.Errorf("%w : colonne « %s » introuvable dans %s", ErrChart, name, tb.Name)
	}
	single := cfg.MapCoords == "single"
	nc := 2
	if single {
		nc = 1
	}
	var cols []int
	for _, s := range cfg.Series {
		i, err := find(s)
		if err != nil {
			return chartResult{}, err
		}
		cols = append(cols, i)
	}
	if len(cols) > nc+1 {
		cols = cols[:nc+1]
	}
	sel := []string{"NULL"}
	if cfg.X != "" {
		xi, err := find(cfg.X)
		if err != nil {
			return chartResult{}, err
		}
		sel[0] = exprs[xi]
	}
	for _, i := range cols {
		sel = append(sel, exprs[i])
	}
	q := `SELECT ` + strings.Join(sel, ", ") + ` FROM ` + tb.sql() + ` ORDER BY _pos LIMIT ?` + fmt.Sprint(len(args)+1)
	rs, err := t.query(q, append(args, MaxChartPoints)...)
	if err != nil {
		return chartResult{}, err
	}
	defer rs.Close()
	latName, lonName := tb.Cols[cols[0]].Name, tb.Cols[cols[0]].Name
	if !single {
		lonName = tb.Cols[cols[1]].Name
	}
	out := chartResult{guess: g, labels: []string{}, series: []ChartSeries{{Name: latName, Values: []*float64{}}, {Name: lonName, Values: []*float64{}}}}
	hasVal := len(cols) > nc
	if hasVal {
		out.series = append(out.series, ChartSeries{Name: tb.Cols[cols[nc]].Name, Values: []*float64{}})
	}
	for rs.Next() {
		row := make([]any, len(sel))
		ptrs := make([]any, len(row))
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return chartResult{}, err
		}
		vals := make([]formula.Value, len(row))
		for i, r := range row {
			vals[i] = formula.FromResult(r)
		}
		var lat, lon float64
		var ok bool
		if single {
			if mapBlank(vals[1]) {
				continue
			}
			lat, lon, ok = mapCoordValue(vals[1])
		} else {
			if mapBlank(vals[1]) && mapBlank(vals[2]) {
				continue
			}
			var ok1, ok2 bool
			lat, ok1 = mapAxisValue(vals[1], true)
			lon, ok2 = mapAxisValue(vals[2], false)
			ok = ok1 && ok2
		}
		if !ok {
			out.invalid++
			continue
		}
		label := ""
		if cfg.X != "" && vals[0].Kind != formula.Empty {
			label = vals[0].String()
		}
		out.labels = append(out.labels, label)
		a, o := lat, lon
		out.series[0].Values = append(out.series[0].Values, &a)
		out.series[1].Values = append(out.series[1].Values, &o)
		if hasVal {
			v := vals[nc+1]
			if v.Kind == formula.Number {
				f := v.Num
				out.series[2].Values = append(out.series[2].Values, &f)
			} else {
				out.series[2].Values = append(out.series[2].Values, nil)
			}
		}
	}
	return out, rs.Err()
}

func mapBlank(v formula.Value) bool {
	return v.Kind == formula.Empty || v.Kind == formula.Text && strings.TrimSpace(v.Str) == ""
}

// mapAxisValue lit une latitude (lat) ou une longitude : nombre, ou texte.
func mapAxisValue(v formula.Value, lat bool) (float64, bool) {
	switch v.Kind {
	case formula.Number:
		if lat && (v.Num < -90 || v.Num > 90) || v.Num < -180 || v.Num > 180 {
			return 0, false
		}
		return v.Num, true
	case formula.Text:
		return ParseCoordAxis(v.Str, lat)
	}
	return 0, false
}

// mapCoordValue lit une latitude et une longitude dans une seule cellule.
func mapCoordValue(v formula.Value) (float64, float64, bool) {
	if v.Kind != formula.Text {
		return 0, 0, false
	}
	return ParseCoord(v.Str)
}

// mapWords : mots d'un nom de colonne, en minuscules.
func mapWords(name string) []string {
	return strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return !unicode.IsLetter(r) })
}

func mapNamed(name string, words ...string) bool {
	for _, w := range mapWords(name) {
		for _, x := range words {
			if w == x {
				return true
			}
		}
	}
	return false
}

// mapGuess repère les coordonnées d'un tableau, d'après le nom des colonnes
// (lat, latitude ; lon, lng, long, longitude) et leur contenu (premières
// lignes) : deux colonnes nommées et lisibles, sinon une colonne dont les
// cellules sont des coordonnées complètes, sinon deux colonnes dont les
// lettres disent N/S et E/O.
func (t *tx) mapGuess(tb Table, exprs []string, args []any) (*MapGuess, error) {
	g := &MapGuess{}
	if len(tb.Cols) == 0 {
		return g, nil
	}
	q := `SELECT ` + strings.Join(exprs, ", ") + ` FROM ` + tb.sql() + ` ORDER BY _pos LIMIT ?` + fmt.Sprint(len(args)+1)
	rs, err := t.query(q, append(args, mapSample)...)
	if err != nil {
		return nil, err
	}
	cols := make([][]formula.Value, len(tb.Cols))
	for rs.Next() {
		row := make([]any, len(tb.Cols))
		ptrs := make([]any, len(row))
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			rs.Close()
			return nil, err
		}
		for i, r := range row {
			if v := formula.FromResult(r); !mapBlank(v) {
				cols[i] = append(cols[i], v)
			}
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	// Part lisible des cellules non vides : plus de la moitié.
	readable := func(vs []formula.Value, ok func(formula.Value) bool) bool {
		n := 0
		for _, v := range vs {
			if ok(v) {
				n++
			}
		}
		return n*2 > len(vs)
	}
	names := make([]string, len(tb.Cols))
	for i, c := range tb.Cols {
		names[i] = c.Name
	}
	axisOK := func(lat bool) func(formula.Value) bool {
		return func(v formula.Value) bool { _, ok := mapAxisValue(v, lat); return ok }
	}
	pick := func(match func(i int) bool) string {
		for i := range names {
			if match(i) {
				return names[i]
			}
		}
		return ""
	}
	lat := pick(func(i int) bool { return mapNamed(names[i], "lat", "latitude") && readable(cols[i], axisOK(true)) })
	lon := pick(func(i int) bool {
		return names[i] != lat && mapNamed(names[i], "lon", "lng", "long", "longitude") && readable(cols[i], axisOK(false))
	})
	coord := pick(func(i int) bool {
		return readable(cols[i], func(v formula.Value) bool { _, _, ok := mapCoordValue(v); return ok })
	})
	// Lettres d'hémisphère dans les cellules : N/S pour la latitude, E/O pour la longitude.
	lettered := func(kind byte) func(i int) bool {
		return func(i int) bool {
			return readable(cols[i], func(v formula.Value) bool {
				if v.Kind != formula.Text {
					return false
				}
				_, k, ok := coordAxisText(v.Str)
				return ok && k == kind
			})
		}
	}
	switch {
	case lat != "" && lon != "":
		g.Mode, g.Lat, g.Lon = "pair", lat, lon
		g.Coord = coord
	case coord != "":
		g.Mode, g.Coord = "single", coord
	default:
		if a, o := pick(lettered('N')), pick(lettered('E')); a != "" && o != "" {
			g.Mode, g.Lat, g.Lon = "pair", a, o
		}
	}
	g.Name = pick(func(i int) bool {
		n := names[i]
		if n == g.Lat || n == g.Lon || n == g.Coord || len(cols[i]) == 0 {
			return false
		}
		for _, v := range cols[i] {
			if v.Kind != formula.Text {
				return false
			}
		}
		return true
	})
	return g, nil
}
