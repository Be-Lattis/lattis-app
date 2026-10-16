// Package transfer importe et exporte des données : fichiers CSV (lecture et
// écriture) et classeurs Excel .xlsx (lecture des valeurs et des formules
// simples).
//
// Le paquet ne fait que lire et écrire des fichiers : il décide des
// tableaux, des types et des formules, puis remet le tout au modèle des
// feuilles (canvas.Model.Import) qui crée feuilles et tableaux en une seule
// étape d'annulation. Les fichiers sont lus deux fois (types, puis valeurs)
// plutôt que gardés en mémoire : un CSV d'un million de lignes passe.
package transfer

import (
	"strconv"
	"strings"
	"time"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/formula"
	"github.com/lalamefine/slitable/internal/grid"
)

// value est une valeur lue dans un fichier, avec son type naturel.
type value struct {
	t grid.Type // "" : vide
	i int64     // entier, booléen (0/1)
	f float64   // décimal
	s string    // texte, date aaaa-mm-jj
}

func textValue(s string) value { return value{t: grid.Text, s: s} }

// parseText lit une valeur de CSV : nombres (virgule ou point décimal,
// espaces de milliers), booléens, dates jj/mm/aaaa ou aaaa-mm-jj ; le reste
// est du texte, gardé tel quel.
//
// Des chiffres avec un zéro en tête (« 01234 ») ou plus de 15 chiffres
// restent du texte : codes postaux, références, numéros de compte, que la
// conversion en nombre abîmerait.
func parseText(raw string) value {
	s := strings.TrimSpace(raw)
	if s == "" {
		return value{}
	}
	switch strings.ToLower(s) {
	case "vrai", "true":
		return value{t: grid.Boolean, i: 1}
	case "faux", "false":
		return value{t: grid.Boolean, i: 0}
	}
	if keepsText(s) {
		return textValue(raw)
	}
	// Les essais passent par strconv et time directement : les erreurs de
	// grid.Parse (fmt.Errorf) coûtaient la moitié d'un import.
	if numeric(s) {
		n := stripSeparators(s)
		if i, err := strconv.ParseInt(n, 10, 64); err == nil {
			return value{t: grid.Integer, i: i}
		}
		if f, err := strconv.ParseFloat(strings.Replace(n, ",", ".", 1), 64); err == nil {
			return value{t: grid.Decimal, f: f}
		}
	}
	if len(s) <= 10 && strings.ContainsAny(s, "/-") {
		for _, layout := range []string{"02/01/2006", "2/1/2006", "2006-01-02"} {
			if d, err := time.Parse(layout, s); err == nil {
				return value{t: grid.Date, s: d.Format("2006-01-02")}
			}
		}
	}
	return textValue(raw)
}

// stripSeparators retire les espaces de milliers (comme grid.Parse).
func stripSeparators(s string) string {
	if !strings.ContainsAny(s, " \u00a0\u202f") {
		return s
	}
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\u00a0' || r == '\u202f' {
			return -1
		}
		return r
	}, s)
}

// numeric écarte ce que strconv.ParseFloat accepte en plus des nombres
// écrits à la main (« inf », « NaN », hexadécimal).
func numeric(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune("0123456789+-.,eE \u00a0\u202f", r) {
			return false
		}
	}
	return true
}

// keepsText repère les suites de chiffres qu'il ne faut pas lire comme des
// nombres.
func keepsText(s string) bool {
	digits := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	if digits > 15 {
		return true
	}
	t := strings.TrimLeft(s, "+-")
	return len(t) > 1 && t[0] == '0' && t[1] >= '0' && t[1] <= '9'
}

// join donne le type d'une colonne qui contient les deux types (voir
// canvas.Join : booléen → entier → décimal → texte, date → texte).
func join(a, b grid.Type) grid.Type { return canvas.Join(a, b) }

// final donne le type d'une colonne à partir du type joint de ses valeurs.
func final(t grid.Type) grid.Type {
	if t == "" {
		return grid.Text
	}
	return t
}

// cast convertit une valeur au type de stockage de sa colonne (voir
// canvas.ImportTable.Rows).
func cast(v value, to grid.Type) any {
	if v.t == "" {
		return nil
	}
	switch to {
	case grid.Integer, grid.Boolean:
		return v.i
	case grid.Decimal:
		if v.t == grid.Decimal {
			return v.f
		}
		return float64(v.i)
	case grid.Date:
		return v.s
	}
	return display(v)
}

// display écrit une valeur comme la grille l'affiche (locale française).
func display(v value) string {
	switch v.t {
	case grid.Integer:
		return strconv.FormatInt(v.i, 10)
	case grid.Decimal:
		return formula.FormatNumber(v.f)
	case grid.Boolean:
		if v.i != 0 {
			return "VRAI"
		}
		return "FAUX"
	case grid.Date:
		if d, err := time.Parse("2006-01-02", v.s); err == nil {
			return d.Format("02/01/2006")
		}
	}
	return v.s
}

// columnNames complète et rend uniques des en-têtes : un en-tête vide
// devient « Colonne N », un doublon « Nom 2 ».
func columnNames(headers []string, width int) []string {
	out := make([]string, width)
	seen := map[string]bool{}
	for i := range width {
		h := ""
		if i < len(headers) {
			h = strings.Join(strings.Fields(headers[i]), " ")
		}
		if h == "" {
			h = "Colonne " + strconv.Itoa(i+1)
		}
		name := h
		for n := 2; seen[strings.ToLower(name)]; n++ {
			name = h + " " + strconv.Itoa(n)
		}
		seen[strings.ToLower(name)] = true
		out[i] = name
	}
	return out
}

// tableName choisit le nom d'un tableau : celui de la feuille s'il est posé
// en haut à gauche (comme à la saisie), sinon TableauN ; jamais un nom pris.
func tableName(sheet string, row, col int, taken map[string]bool) string {
	if row == 0 && col == 0 {
		if n := identifier(sheet); n != "" && !taken[n] {
			taken[n] = true
			return n
		}
	}
	for i := 1; ; i++ {
		n := "Tableau" + strconv.Itoa(i)
		if !taken[n] {
			taken[n] = true
			return n
		}
	}
}

// identifier garde les lettres, chiffres (pas en tête) et _ d'un nom, comme
// le nommage des tableaux du modèle.
func identifier(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '_' || isLetter(r):
			b.WriteRune(r)
		case r >= '0' && r <= '9' && b.Len() > 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}
