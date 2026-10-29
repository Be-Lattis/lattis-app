package formula

import (
	"sort"
	"strings"
)

// Search cherche des fonctions dans un catalogue (Catalog) par nom, autre
// nom, catégorie ou description, sans tenir compte des majuscules ni des
// accents : le nom exact d'abord, puis les noms qui commencent par la
// recherche, ceux qui la contiennent, la catégorie, enfin la description.
// category, si non vide, ne garde que les fonctions de cette catégorie.
// Au plus max fonctions (max <= 0 : toutes).
func Search(list []FunctionInfo, query, category string, max int) []FunctionInfo {
	q := Fold(strings.TrimSpace(query))
	cat := Fold(strings.TrimSpace(category))
	type hit struct {
		f     FunctionInfo
		score int
	}
	var hits []hit
	for _, f := range list {
		if cat != "" && !strings.Contains(Fold(f.Category), cat) {
			continue
		}
		score := 1
		if q != "" {
			name, other := Fold(f.Name), Fold(f.Other)
			switch {
			case name == q || other == q:
				score = 100
			case strings.HasPrefix(name, q) || strings.HasPrefix(other, q):
				score = 50
			case strings.Contains(name, q) || strings.Contains(other, q):
				score = 30
			case strings.Contains(Fold(f.Category), q):
				score = 20
			case strings.Contains(Fold(f.Description), q):
				score = 10
			default:
				score = 0
				// Plusieurs mots : chacun dans le nom, la catégorie ou la
				// description.
				words := strings.Fields(q)
				if len(words) > 1 {
					text := name + " " + other + " " + Fold(f.Category) + " " + Fold(f.Description)
					score = 5
					for _, w := range words {
						if !strings.Contains(text, w) {
							score = 0
							break
						}
					}
				}
			}
		}
		if score > 0 {
			hits = append(hits, hit{f, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].f.Name < hits[j].f.Name
	})
	out := []FunctionInfo{}
	for _, h := range hits {
		if max > 0 && len(out) == max {
			break
		}
		out = append(out, h.f)
	}
	return out
}

var searchFold = strings.NewReplacer(
	"é", "e", "è", "e", "ê", "e", "ë", "e", "à", "a", "â", "a", "ä", "a", "á", "a",
	"î", "i", "ï", "i", "í", "i", "ô", "o", "ö", "o", "ó", "o", "û", "u", "ü", "u", "ù", "u", "ú", "u",
	"ç", "c", "ñ", "n", "ß", "ss",
)

// Fold : s en minuscules, sans accents (recherches, noms proches).
func Fold(s string) string { return searchFold.Replace(strings.ToLower(s)) }
