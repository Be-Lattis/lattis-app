package geo

import (
	"strings"
	"sync"
)

// Noms des communes, mis à plat (voir foldName), construits au premier appel
// d'IsCommune.
var (
	namesOnce sync.Once
	names     map[string]bool
)

var nameFold = strings.NewReplacer("à", "a", "â", "a", "ä", "a", "ç", "c", "é", "e", "è", "e", "ê", "e", "ë", "e", "î", "i", "ï", "i", "ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ÿ", "y", "œ", "oe", "æ", "ae", "-", " ", "'", " ", "’", " ")

// foldName met un nom de commune à plat : minuscules, sans accents, tirets
// et apostrophes en espaces (« Saint-Étienne » = « saint etienne »).
func foldName(s string) string {
	s = nameFold.Replace(strings.ToLower(strings.TrimSpace(s)))
	return strings.Join(strings.Fields(s), " ")
}

// IsCommune dit si name est le nom d'une commune française, sans tenir
// compte de la casse, des accents ni des tirets. Le premier appel charge les
// communes (comme Lookup) : à réserver aux noms qui ne sont rien d'autre.
func IsCommune(name string) bool {
	namesOnce.Do(func() {
		b, err := get()
		if err != nil {
			return
		}
		names = make(map[string]bool, len(b.ents))
		for _, e := range b.ents {
			names[foldName(e.info.Name)] = true
		}
	})
	f := foldName(name)
	return f != "" && names[f]
}
