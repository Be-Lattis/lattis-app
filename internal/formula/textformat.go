package formula

import "github.com/lalamefine/lattis/internal/numfmt"

// fnText : TEXTE(valeur; format) avec les codes de format d'un tableur, par
// le moteur des formats de cellule (internal/numfmt) :
//
//   - nombres : 0 (chiffre obligatoire), # (chiffre facultatif), ? (espace),
//     virgule ou point décimal, espace ou « #,## » pour les milliers, %,
//     E+00 (scientifique), # ?/? (fraction) ; sections
//     positif;négatif;zéro;texte, conditions [>100] ; texte entre
//     guillemets ;
//   - dates : jj/dd, jjj, jjjj, mm, mmm, mmmm, aa/yy, aaaa/yyyy, hh, mm
//     (minutes après hh ou avant ss), ss, AM/PM, [h] (durée).
//
// Le résultat suit la locale française (virgule décimale, espace pour les
// milliers, noms des jours et des mois en français).
func fnText(a []Value) Value {
	v := a[0]
	f, e := ToText(a[1])
	if e.IsErr() {
		return e
	}
	if v.IsErr() {
		return v
	}
	if v.Kind == Bool {
		return Str(v.String())
	}
	code := numfmt.Parse(f)
	x, e := ToNumber(v)
	if e.IsErr() {
		// Un texte non numérique : la section du texte s'il y en a une, sinon
		// le texte tel quel, comme dans un tableur.
		return Str(numfmt.Plain(code.Text(v.String()).Text))
	}
	r := code.Number(x, numfmt.FR)
	if !r.OK {
		return Err(ErrValue)
	}
	return Str(numfmt.Plain(r.Text))
}
