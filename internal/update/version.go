// Package update : mise à jour de l'application de bureau
// (docs/conception/mise-a-jour-automatique.md, « Décisions de Louis »).
//
// L'application cherche sur GitHub (Be-Lattis/lattis-app) une version plus
// récente que la sienne, la télécharge à la demande de l'utilisateur, la
// vérifie (empreinte SHA-256 de latest.json ou de SHA256SUMS.txt, taille,
// signature Authenticode si l'application elle-même est signée), puis
// l'installe quand l'application se ferme : installeur par utilisateur
// (silencieux, sans droits d'administration), installeur par machine
// (invite UAC), ou remplacement de l'exécutable portable.
//
// Aucune télémétrie : des GET sans cookie ni identifiant, dont l'en-tête
// User-Agent dit seulement « Lattis/<version> (<système>) ». Rien en
// édition web, ni dans une compilation de développement (version « dev »).
package update

import (
	"strconv"
	"strings"
)

// Version est une version sémantique (MAJEUR.MINEUR.CORRECTIF, suffixe de
// pré-version facultatif : 0.3.0-beta, 1.2.0-beta.2, 1.2.0-rc.1).
type Version struct {
	Major, Minor, Patch int
	// Pre : identifiants de la pré-version (« beta », « 2 »), vide pour une
	// version publiée.
	Pre []string
}

// ParseVersion lit une version, avec ou sans « v » (v0.3.0-beta) ; les
// métadonnées de construction (+…) sont ignorées. Faux si ce n'est pas une
// version (« dev »).
func ParseVersion(s string) (Version, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	core, pre, hasPre := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var n [3]int
	for i, p := range parts {
		if p == "" || len(p) > 1 && p[0] == '0' {
			return Version{}, false
		}
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 {
			return Version{}, false
		}
		n[i] = x
	}
	v := Version{Major: n[0], Minor: n[1], Patch: n[2]}
	if hasPre {
		if pre == "" {
			return Version{}, false
		}
		for _, id := range strings.Split(pre, ".") {
			if id == "" {
				return Version{}, false
			}
			for _, r := range id {
				if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-') {
					return Version{}, false
				}
			}
			v.Pre = append(v.Pre, id)
		}
	}
	return v, true
}

// Prerelease dit si c'est une pré-version (canal bêta).
func (v Version) Prerelease() bool { return len(v.Pre) > 0 }

// String écrit la version sans « v ».
func (v Version) String() string {
	s := strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
	if len(v.Pre) > 0 {
		s += "-" + strings.Join(v.Pre, ".")
	}
	return s
}

// Compare rend -1, 0 ou 1 selon que a précède, égale ou suit b, dans
// l'ordre de SemVer 2.0 : 0.2.0-beta < 0.2.0-beta.1 < 0.2.0-beta.2 <
// 0.2.0-rc.1 < 0.2.0 < 0.2.1-beta.
func Compare(a, b Version) int {
	for _, d := range [3][2]int{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if d[0] != d[1] {
			return sign(d[0] - d[1])
		}
	}
	switch {
	case len(a.Pre) == 0 && len(b.Pre) == 0:
		return 0
	case len(a.Pre) == 0:
		return 1 // la version publiée suit ses pré-versions
	case len(b.Pre) == 0:
		return -1
	}
	for i := 0; i < len(a.Pre) && i < len(b.Pre); i++ {
		x, y := a.Pre[i], b.Pre[i]
		xn, xNum := numeric(x)
		yn, yNum := numeric(y)
		switch {
		case xNum && yNum:
			if xn != yn {
				return sign(xn - yn)
			}
		case xNum:
			return -1 // un identifiant numérique précède un texte
		case yNum:
			return 1
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		}
	}
	return sign(len(a.Pre) - len(b.Pre))
}

// Newer dit si candidate est strictement plus récente que current.
func Newer(candidate, current Version) bool { return Compare(candidate, current) > 0 }

func numeric(s string) (int, bool) {
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
