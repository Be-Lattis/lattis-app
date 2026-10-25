// Package series reconnaît les suites de noms (onglets « Janvier »,
// « Février »… ; « 2023 », « 2024 » ; « Lot 1 », « Lot 2 » ; « Paris »,
// « Lyon »…) et en tire une catégorie typée : nom de colonne proposé, type
// et valeur de chaque nom. Sert au regroupement des onglets semblables
// (canvas.SimilarSheets, docs/conception/regrouper-onglets-semblables.md,
// §2.3 et §3).
package series

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/lalamefine/lattis/internal/geo"
	"github.com/lalamefine/lattis/internal/grid"
)

// Kind est la sorte de suite reconnue.
type Kind string

const (
	MonthYear Kind = "month_year" // Janv 2025, 2025-01, 01/2025
	Month     Kind = "month"      // Janvier, Févr., 01…12
	Year      Kind = "year"       // 2023, 2024
	Quarter   Kind = "quarter"    // T1 2025, 2025-Q1
	Week      Kind = "week"       // S01 2025, Semaine 2
	Date      Kind = "date"       // 01/03/2024, 2024-03-01
	Number    Kind = "number"     // Lot 1, Lot 2, Lot 4
	Letter    Kind = "letter"     // Annexe A, Annexe B
	Default   Kind = "default"    // Feuille1, Feuille2 (noms par défaut)
	City      Kind = "city"       // communes reconnues
	Shape     Kind = "shape"      // même forme : Agence Nord, Agence Sud
	Other     Kind = "other"      // rien de reconnu
)

// Poids des signaux (§2.3) : fort, fort atténué (noms par défaut), moyen.
const (
	Strong = 40
	Weak   = 25
	Medium = 20
)

// Series est la catégorie tirée d'une liste de noms.
type Series struct {
	Kind Kind `json:"kind"`
	// Score : poids du signal (Strong, Weak, Medium ou 0).
	Score int `json:"score"`
	// Common : partie commune des noms, sans séparateurs au bord (« Lot »,
	// « Ventes ») ; vide s'il n'y en a pas.
	Common string `json:"common"`
	// Column : nom de colonne proposé (Mois, Année, Lot…) ; Default : le
	// nom est « Catégorie », à renommer.
	Column string `json:"column"`
	// Type : type de la colonne (date, integer ou text).
	Type grid.Type `json:"type"`
	// Format : motif d'affichage d'une date (« mmmm aaaa »), sinon vide.
	Format string `json:"format,omitempty"`
	// Values : une valeur par nom, telle qu'elle se stocke (date ISO
	// aaaa-mm-jj, entier en chiffres, texte).
	Values []string `json:"values"`
	// Labels : la même valeur, telle qu'on la saisit (date jj/mm/aaaa) : pour
	// une liste déroulante.
	Labels []string `json:"labels"`
}

// Fold met un nom à plat : minuscules, sans accents, espaces resserrés.
func Fold(s string) string {
	s = accents.Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

var accents = strings.NewReplacer("à", "a", "â", "a", "ä", "a", "ç", "c", "é", "e", "è", "e", "ê", "e", "ë", "e", "î", "i", "ï", "i", "ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ÿ", "y", "œ", "oe", "æ", "ae", "’", "'")

// MonthNames : noms des mois, en français, avec leur majuscule.
var MonthNames = []string{"Janvier", "Février", "Mars", "Avril", "Mai", "Juin", "Juillet", "Août", "Septembre", "Octobre", "Novembre", "Décembre"}

var enMonths = []string{"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"}

// Noms des mois en allemand et en espagnol (à plat, voir Fold), reconnus
// comme le français et l'anglais.
var (
	deMonths = []string{"januar", "februar", "marz", "april", "mai", "juni", "juli", "august", "september", "oktober", "november", "dezember"}
	esMonths = []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
)

// monthNamesIn : les noms des mois écrits dans la langue de l'interface
// (Localized) ; le français pour une autre langue.
var monthNamesIn = map[string][]string{
	"en": {"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	"de": {"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"},
	"es": {"Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio", "Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre"},
	"zh": {"一月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "十一月", "十二月"},
}

// columnsIn : les noms de colonne proposés (en français dans Series.Column)
// dans les autres langues.
var columnsIn = map[string]map[string]string{
	"en": {"Mois": "Month", "Année": "Year", "Trimestre": "Quarter", "Semaine": "Week", "Date": "Date", "Numéro": "Number", "Lettre": "Letter", "Ville": "City", "Catégorie": "Category"},
	"de": {"Mois": "Monat", "Année": "Jahr", "Trimestre": "Quartal", "Semaine": "Woche", "Date": "Datum", "Numéro": "Nummer", "Lettre": "Buchstabe", "Ville": "Stadt", "Catégorie": "Kategorie"},
	"es": {"Mois": "Mes", "Année": "Año", "Trimestre": "Trimestre", "Semaine": "Semana", "Date": "Fecha", "Numéro": "Número", "Lettre": "Letra", "Ville": "Ciudad", "Catégorie": "Categoría"},
	"zh": {"Mois": "月份", "Année": "年份", "Trimestre": "季度", "Semaine": "周", "Date": "日期", "Numéro": "编号", "Lettre": "字母", "Ville": "城市", "Catégorie": "类别"},
}

// Localized rend la catégorie s (reconnue par Recognize, en français) dans
// la langue lang de l'interface : le nom de colonne proposé (Month, Year…)
// et, pour une suite de mois, les noms des mois (January…), valeurs comme
// étiquettes. Les noms d'onglets restent reconnus dans toutes les langues ;
// seul ce que Lattis écrit change. lang vide ou "fr" : s tel quel.
func Localized(s Series, lang string) Series {
	cols, ok := columnsIn[lang]
	if !ok {
		return s
	}
	// Une suite de nombres ou de lettres, une forme commune, prennent la
	// partie commune des noms (texte de l'utilisateur) : seul leur nom par
	// défaut (Numéro, Lettre) se traduit.
	switch s.Kind {
	case Number, Letter, Shape:
		if s.Column == "Numéro" || s.Column == "Lettre" {
			s.Column = cols[s.Column]
		}
	default:
		if c, ok := cols[s.Column]; ok {
			s.Column = c
		}
	}
	if s.Kind == Month {
		names := monthNamesIn[lang]
		tr := func(vals []string) []string {
			out := slices.Clone(vals)
			for i, v := range out {
				if k := slices.Index(MonthNames, v); k >= 0 {
					out[i] = names[k]
				}
			}
			return out
		}
		s.Values, s.Labels = tr(s.Values), tr(s.Labels)
	}
	return s
}

// monthWord lit un nom de mois écrit en toutes lettres ou abrégé (français,
// anglais, allemand ou espagnol : « janv. », « fev », « Sept », « March ») ; ambigu ou trop
// court (« ju », « jui ») : refusé.
func monthWord(s string) (int, bool) {
	s = strings.TrimSuffix(Fold(s), ".")
	if len([]rune(s)) < 3 {
		return 0, false
	}
	found := 0
	for i := range 12 {
		if strings.HasPrefix(Fold(MonthNames[i]), s) || strings.HasPrefix(enMonths[i], s) || strings.HasPrefix(deMonths[i], s) || strings.HasPrefix(esMonths[i], s) {
			if found != 0 && found != i+1 {
				return 0, false
			}
			found = i + 1
		}
	}
	// « sept » et « sep » : septembre ; « dec » : décembre (accents ôtés).
	return found, found != 0
}

// tokens coupe un nom aux séparateurs (espace, tiret, barre, point, _) et
// entre lettres et chiffres (« janv25 » → janv, 25 ; « T1 » → T, 1).
func tokens(s string) []string {
	var out []string
	var cur []rune
	kind := 0 // 1 lettre, 2 chiffre
	flush := func() {
		if len(cur) > 0 {
			out = append(out, string(cur))
		}
		cur, kind = nil, 0
	}
	for _, r := range s {
		k := 0
		switch {
		case unicode.IsLetter(r):
			k = 1
		case unicode.IsDigit(r):
			k = 2
		}
		if k == 0 {
			if r != '.' || kind != 1 {
				flush()
				continue
			}
			// « janv. » : le point reste au mot (abréviation).
			cur = append(cur, r)
			continue
		}
		if kind != 0 && k != kind {
			flush()
		}
		cur = append(cur, r)
		kind = k
	}
	flush()
	return out
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// year lit une année sur quatre chiffres (1900 à 2100), ou sur deux si short.
func year(s string, short bool) (int, bool) {
	if !digits(s) {
		return 0, false
	}
	n, _ := strconv.Atoi(s)
	switch {
	case len(s) == 4 && n >= 1900 && n <= 2100:
		return n, true
	case short && len(s) == 2:
		return 2000 + n, true
	}
	return 0, false
}

// item est un nom reconnu : sa sorte et sa valeur (clé de tri, valeur
// stockée, valeur saisie).
type item struct {
	kind  Kind
	order int
	value string
	label string
}

func dateItem(t time.Time, kind Kind) item {
	return item{kind, t.Year()*10000 + int(t.Month())*100 + t.Day(), t.Format("2006-01-02"), t.Format("02/01/2006")}
}

// parse reconnaît un nom seul ; numericMonths : « 01 » à « 12 » sont des
// mois (tous les noms sont sur deux chiffres, sans partie commune).
func parse(s string, numericMonths bool) []item {
	s = strings.TrimSpace(s)
	var out []item
	// Date complète.
	for _, v := range []string{s, strings.NewReplacer("-", "/", ".", "/").Replace(s)} {
		if strings.Count(v, "/") == 2 || strings.Count(v, "-") == 2 {
			if d, ok := grid.ParseDate(v); ok {
				out = append(out, dateItem(d, Date))
				break
			}
		}
	}
	tk := tokens(s)
	switch len(tk) {
	case 1:
		w := tk[0]
		if y, ok := year(w, false); ok {
			out = append(out, item{Year, y, strconv.Itoa(y), strconv.Itoa(y)})
		}
		if m, ok := monthWord(w); ok && !digits(w) {
			out = append(out, item{Month, m, MonthNames[m-1], MonthNames[m-1]})
		}
		if numericMonths && len(w) == 2 && digits(w) {
			if m, _ := strconv.Atoi(w); m >= 1 && m <= 12 {
				out = append(out, item{Month, m, MonthNames[m-1], MonthNames[m-1]})
			}
		}
		if digits(w) && len(w) <= 9 {
			n, _ := strconv.Atoi(w)
			out = append(out, item{Number, n, strconv.Itoa(n), strconv.Itoa(n)})
		}
		if r := []rune(w); len(r) == 1 && unicode.IsLetter(r[0]) {
			u := unicode.ToUpper(r[0])
			out = append(out, item{Letter, int(u), string(u), string(u)})
		}
	case 2, 3:
		a, b := tk[0], tk[len(tk)-1]
		if len(tk) == 3 {
			// « T1 2025 », « 2025 T1 », « S 01 2025 » : un mot, un nombre et
			// une année.
			if it, ok := periodWithYear(tk); ok {
				out = append(out, it)
			}
			break
		}
		// Mois et année, dans un ordre ou l'autre.
		for _, p := range [][2]string{{a, b}, {b, a}} {
			m, okm := monthWord(p[0])
			if digits(p[0]) {
				n, _ := strconv.Atoi(p[0])
				m, okm = n, n >= 1 && n <= 12 && len(p[0]) <= 2
			}
			y, oky := year(p[1], !digits(p[0]))
			if okm && oky {
				t := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
				it := dateItem(t, MonthYear)
				out = append(out, it)
				break
			}
		}
		// Trimestre ou semaine sans année : « T1 », « S12 ».
		if it, ok := period(a, b, 0); ok {
			out = append(out, it)
		}
	}
	return out
}

// period lit un trimestre (T1, Q1, 1T) ou une semaine (S12, W12, Sem 12) ;
// y : année (0 : sans).
func period(a, b string, y int) (item, bool) {
	word, num := a, b
	if digits(a) && !digits(b) {
		word, num = b, a
	}
	if !digits(num) || len(num) > 2 {
		return item{}, false
	}
	n, _ := strconv.Atoi(num)
	switch w := Fold(word); w {
	case "t", "q", "trim", "trimestre":
		if n < 1 || n > 4 {
			return item{}, false
		}
		v := fmt.Sprintf("T%d", n)
		if y > 0 {
			v = fmt.Sprintf("%d-T%d", y, n)
		}
		return item{Quarter, y*100 + n, v, v}, true
	case "s", "w", "sem", "semaine", "week":
		if n < 1 || n > 53 {
			return item{}, false
		}
		v := fmt.Sprintf("S%02d", n)
		if y > 0 {
			v = fmt.Sprintf("%d-S%02d", y, n)
		}
		return item{Week, y*100 + n, v, v}, true
	}
	return item{}, false
}

// periodWithYear : trois morceaux, dont une année et un trimestre ou une
// semaine.
func periodWithYear(tk []string) (item, bool) {
	for yi := range tk {
		y, ok := year(tk[yi], yi > 0)
		if !ok {
			continue
		}
		var rest []string
		for i, t := range tk {
			if i != yi {
				rest = append(rest, t)
			}
		}
		if it, ok := period(rest[0], rest[1], y); ok {
			return it, true
		}
	}
	return item{}, false
}

// commonParts donne la partie commune au début et à la fin des noms, sans
// couper une suite de chiffres ou de lettres (« 2021 », « 2025 » n'ont pas
// de partie commune « 202 »).
func commonParts(names []string) (prefix, suffix string) {
	rs := make([][]rune, len(names))
	for i, n := range names {
		rs[i] = []rune(n)
	}
	minLen := len(rs[0])
	for _, r := range rs {
		minLen = min(minLen, len(r))
	}
	p := 0
	for p < minLen {
		c := rs[0][p]
		same := true
		for _, r := range rs[1:] {
			if r[p] != c {
				same = false
				break
			}
		}
		if !same {
			break
		}
		p++
	}
	class := func(r rune) int {
		switch {
		case unicode.IsDigit(r):
			return 2
		case unicode.IsLetter(r):
			return 1
		}
		return 0
	}
	// Recule tant que la coupure tombe au milieu d'un mot ou d'un nombre.
	split := func(p int) bool {
		if p == 0 {
			return true
		}
		for _, r := range rs {
			if p < len(r) && class(r[p-1]) != 0 && class(r[p-1]) == class(r[p]) {
				return false
			}
		}
		return true
	}
	for p > 0 && !split(p) {
		p--
	}
	s := 0
	for s < minLen-p {
		c := rs[0][len(rs[0])-1-s]
		same := true
		for _, r := range rs[1:] {
			if r[len(r)-1-s] != c {
				same = false
				break
			}
		}
		if !same {
			break
		}
		s++
	}
	splitEnd := func(s int) bool {
		if s == 0 {
			return true
		}
		for _, r := range rs {
			k := len(r) - s
			if k > 0 && class(r[k-1]) != 0 && class(r[k-1]) == class(r[k]) {
				return false
			}
		}
		return true
	}
	for s > 0 && !splitEnd(s) {
		s--
	}
	return string(rs[0][:p]), string(rs[0][len(rs[0])-s:])
}

// trimSep retire les séparateurs au bord d'une partie commune.
func trimSep(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// defaultNames : préfixes des noms d'onglets par défaut (Feuille1, Sheet1,
// Tabelle1, Hoja1).
var defaultNames = map[string]bool{"feuille": true, "feuil": true, "sheet": true, "tabelle": true, "hoja": true, "page": true}

// Recognize reconnaît la suite que forment les noms (dans l'ordre des
// onglets). Il faut au moins deux noms, tous différents.
func Recognize(names []string) Series {
	other := func() Series {
		vals := make([]string, len(names))
		for i, n := range names {
			vals[i] = strings.TrimSpace(n)
		}
		return Series{Kind: Other, Column: "Catégorie", Type: grid.Text, Values: vals, Labels: vals}
	}
	if len(names) < 2 {
		return other()
	}
	seen := map[string]bool{}
	for _, n := range names {
		f := Fold(n)
		if f == "" || seen[f] {
			return other()
		}
		seen[f] = true
	}
	prefix, suffix := commonParts(names)
	// La partie commune se retire entière, coupée au dernier espace, ou pas
	// du tout (« Janv 2025 » garde son année, « Stock 2024-03-01 » ne perd
	// que « Stock ») : la sorte la plus précise l'emporte.
	cuts := func(s string, end bool) []string {
		out := []string{"", s}
		if end {
			if k := strings.IndexFunc(s, unicode.IsSpace); k > 0 && k < len(s)-1 {
				out = append(out, s[k:])
			}
		} else if k := strings.LastIndexFunc(s, unicode.IsSpace); k > 0 && k < len(s)-1 {
			out = append(out, s[:k+1])
		}
		return out
	}
	// Une seule sorte pour tous les noms, dans l'ordre de préférence.
	order := []Kind{Date, MonthYear, Quarter, Week, Year, Month, Number, Letter}
	best, bestRank := Series{}, len(order)
	for _, pre := range cuts(prefix, false) {
		for _, suf := range cuts(suffix, true) {
			common := trimSep(pre + " " + suf)
			if trimSep(pre) != "" && trimSep(suf) != "" {
				common = trimSep(pre) + " " + trimSep(suf)
			}
			// Une lettre seule n'est pas une partie commune (« T » de « T1 2025 »).
			if (pre != "" || suf != "") && len([]rune(common)) < 2 {
				continue
			}
			parts := make([]string, len(names))
			empty := false
			for i, n := range names {
				r := []rune(n)
				parts[i] = strings.TrimSpace(string(r[len([]rune(pre)) : len(r)-len([]rune(suf))]))
				empty = empty || parts[i] == ""
			}
			if empty {
				continue
			}
			twoDigits := common == ""
			for _, p := range parts {
				if len(p) != 2 || !digits(p) {
					twoDigits = false
				}
			}
			parsed := make([][]item, len(parts))
			for i, p := range parts {
				parsed[i] = parse(p, twoDigits)
			}
			for rank, k := range order[:bestRank] {
				items := make([]item, len(parts))
				ok := true
				for i := range parts {
					found := false
					for _, it := range parsed[i] {
						if it.kind == k {
							items[i], found = it, true
							break
						}
					}
					if !found {
						ok = false
						break
					}
				}
				// Une suite de chiffres ou de lettres doit aller dans un sens.
				if !ok || !distinct(items) || (k == Number || k == Letter) && !monotonic(items) {
					continue
				}
				best, bestRank = build(k, items, common), rank
				break
			}
		}
	}
	if bestRank < len(order) {
		return best
	}
	common := trimSep(prefix + " " + suffix)
	if trimSep(prefix) != "" && trimSep(suffix) != "" {
		common = trimSep(prefix) + " " + trimSep(suffix)
	}
	parts := make([]string, len(names))
	for i, n := range names {
		r := []rune(n)
		parts[i] = strings.TrimSpace(string(r[len([]rune(prefix)) : len(r)-len([]rune(suffix))]))
	}
	// Communes : seulement pour des noms propres sans chiffres.
	if properNouns(names) {
		all := true
		for _, n := range names {
			if !geo.IsCommune(n) {
				all = false
				break
			}
		}
		if all {
			vals := make([]string, len(names))
			for i, n := range names {
				vals[i] = strings.TrimSpace(n)
			}
			return Series{Kind: City, Score: Medium, Column: "Ville", Type: grid.Text, Values: vals, Labels: vals}
		}
	}
	if len([]rune(common)) >= 2 && !slices.Contains(parts, "") {
		return Series{Kind: Shape, Score: Medium, Common: common, Column: capital(common), Type: grid.Text, Values: parts, Labels: parts}
	}
	return other()
}

func capital(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func properNouns(names []string) bool {
	for _, n := range names {
		r := []rune(strings.TrimSpace(n))
		if len(r) < 2 || !unicode.IsUpper(r[0]) {
			return false
		}
		for _, c := range r {
			if unicode.IsDigit(c) {
				return false
			}
		}
	}
	return true
}

func distinct(items []item) bool {
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it.value] {
			return false
		}
		seen[it.value] = true
	}
	return true
}

func monotonic(items []item) bool {
	up, down := true, true
	for i := 1; i < len(items); i++ {
		up = up && items[i].order > items[i-1].order
		down = down && items[i].order < items[i-1].order
	}
	return up || down
}

func build(k Kind, items []item, common string) Series {
	s := Series{Kind: k, Score: Strong, Common: common}
	for _, it := range items {
		s.Values = append(s.Values, it.value)
		s.Labels = append(s.Labels, it.label)
	}
	switch k {
	case MonthYear:
		s.Column, s.Type, s.Format = "Mois", grid.Date, "mmmm aaaa"
	case Month:
		s.Column, s.Type = "Mois", grid.Text
	case Year:
		s.Column, s.Type = "Année", grid.Integer
	case Quarter:
		s.Column, s.Type = "Trimestre", grid.Text
	case Week:
		s.Column, s.Type = "Semaine", grid.Text
	case Date:
		s.Column, s.Type = "Date", grid.Date
	case Number:
		s.Type = grid.Integer
		s.Column = capital(common)
		if defaultNames[Fold(common)] {
			s.Kind, s.Score, s.Column = Default, Weak, "Numéro"
		}
		if s.Column == "" {
			s.Column = "Numéro"
		}
	case Letter:
		s.Type = grid.Text
		s.Column = capital(common)
		if s.Column == "" {
			s.Column = "Lettre"
		}
	}
	return s
}

// Latest donne le rang de la valeur la plus récente (plus grande date ou
// année ; sinon le dernier nom) : l'onglet à choix s'y règle (§7, étape 8).
func Latest(s Series) int {
	best := len(s.Values) - 1
	if s.Type == grid.Integer || s.Type == grid.Date {
		vals := append([]string(nil), s.Values...)
		idx := make([]int, len(vals))
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool {
			va, vb := vals[idx[a]], vals[idx[b]]
			if s.Type == grid.Integer {
				na, _ := strconv.Atoi(va)
				nb, _ := strconv.Atoi(vb)
				return na < nb
			}
			return va < vb
		})
		best = idx[len(idx)-1]
	}
	return best
}
