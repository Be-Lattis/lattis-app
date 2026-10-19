package formula

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestLetSyntax : une forme fautive de LET est refusée à la saisie, avec un
// message, comme dans Excel.
func TestLetSyntax(t *testing.T) {
	for src, msg := range map[string]string{
		`=LET(x;1;y;2)`:       "impair",
		`=LET(A1;1;2)`:        "nom n° 1",
		`=LET(Cell[B2];1;2)`:  "nom n° 1",
		`=LET(x;1;SOMME;2;3)`: "nom n° 2",
		`=LET(texte;1;2)`:     "nom de fonction", // TEXTE
		`=LET(taux;1;2)`:      "nom de fonction", // TAUX, comme dans Excel
		`=LET(1;1;2)`:         "nom n° 1",
		`=LET("x";1;2)`:       "nom n° 1",
		`=LET(;1;2)`:          "manquant",
	} {
		var se *SyntaxError
		_, err := Parse(src, FR)
		if !errors.As(err, &se) || !strings.Contains(se.Msg, msg) {
			t.Errorf("%s : erreur « %s » attendue, obtenu %v", src, msg, err)
		}
	}
	for _, src := range []string{`=LET(x;1;x)`, `=LET(prix_ht;1;tva;0,2;prix_ht*(1+tva))`, `=LET(x;{1;2};SOMME(x))`} {
		if _, err := Parse(src, FR); err != nil {
			t.Errorf("%s : %v", src, err)
		}
	}
	if _, err := Parse(`=LET(x, 1, y, x+1, x*y)`, EN); err != nil {
		t.Errorf("LET en anglais : %v", err)
	}
}

// TestLetRandOnce : un nom vaut une seule valeur, même tirée au hasard, et
// les deux backends tirent les mêmes nombres.
func TestLetRandOnce(t *testing.T) {
	f := fixture(t)
	for _, src := range []string{`=LET(x;ALEA();x)`, `=LET(x;ALEA();y;ALEA();x+y*2)`, `=ALEA()+LET(x;ALEA();x*3)+ALEA()`, `=LET(x;LET(y;ALEA();y+1);x*x)`} {
		n := mustParse(t, src, FR)
		for _, row := range []int{0, 2} {
			if g, s := f.evalGo(n, row).String(), f.evalSQL(t, n, row).String(); g != s {
				t.Errorf("%s ligne %d : Go %s, SQL %s", src, row, g, s)
			}
		}
	}
	// Le tirage nommé est celui qu'ALEA donnerait seul à cette place.
	a, b := f.evalGo(mustParse(t, `=ALEA()`, FR), 1), f.evalGo(mustParse(t, `=LET(x;ALEA();x)`, FR), 1)
	if a != b {
		t.Errorf("LET(x;ALEA();x) = %v, ALEA() = %v", b, a)
	}
}

// TestArrayLit : constantes matricielles, en français et en anglais.
func TestArrayLit(t *testing.T) {
	n := mustParse(t, `={1.2;-3,5."a";VRAI.#N/A}`, FR)
	want := ArrayLit{Rows: [][]Node{{NumLit{1}, NumLit{2}}, {NumLit{-3.5}, StrLit{"a"}}, {BoolLit{true}, ErrLit{ErrNA}}}}
	if !reflect.DeepEqual(n, want) {
		t.Fatalf("FR : %#v", n)
	}
	if got := Format(n, EN); got != `={1,2;-3.5,"a";TRUE,#N/A}` {
		t.Errorf("EN : %s", got)
	}
	if got := Format(n, FR); got != `={1.2;-3,5."a";VRAI.#N/A}` {
		t.Errorf("FR : %s", got)
	}
	if back := mustParse(t, Format(n, EN), EN); !reflect.DeepEqual(back, want) {
		t.Errorf("EN relu : %#v", back)
	}
	if back := mustParse(t, `={1\2}`, FR); !reflect.DeepEqual(back, ArrayLit{Rows: [][]Node{{NumLit{1}, NumLit{2}}}}) {
		t.Errorf(`\ entre colonnes : %#v`, back)
	}
	for _, src := range []string{`={1;2.3}`, `={A1}`, `={1`, `={1+2}`, `={}`, `={1;;2}`, `=1.5`} {
		var se *SyntaxError
		if _, err := Parse(src, FR); !errors.As(err, &se) {
			t.Errorf("%s : erreur de syntaxe attendue, obtenu %v", src, err)
		}
	}
}

// TestTextSplitShape : sans précalcul, le moteur SQL ne connaît la forme de
// FRACTIONNER.TEXTE que pour des littéraux et des cellules ; avec (hoist,
// comme dans le classeur), aussi pour une valeur de tableau.
func TestTextSplitShape(t *testing.T) {
	f := fixture(t)
	hoist := func(c Compiled) Value {
		var v any
		if err := f.db.Read().QueryRow(`SELECT `+c.SQL, c.Args...).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return FromResult(v)
	}
	names := map[string]Value{"cell_a1": Str("x;y")}
	for src, want := range map[string]string{
		`=FRACTIONNER.TEXTE(Ventes[Produit][2];"u")`:                           "So|ris",
		`=FRACTIONNER.TEXTE(REFERENCE(Clients[Nom];3)&"-"&Cell[A1];{"-";";"})`: "Durand|x|y",
	} {
		n := mustParse(t, src, FR)
		if _, e := CompileSpill(n, f.schema, names); !e.IsErr() {
			t.Errorf("%s : sans précalcul, #VALEUR! attendu", src)
		}
		sp, e := CompileSpillHoisted(n, f.schema, names, hoist)
		if e.IsErr() {
			t.Fatalf("%s : %v", src, e)
		}
		raw := make([]any, len(sp.Columns))
		ptrs := make([]any, len(raw))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := f.db.Read().QueryRow(sp.SQL, sp.Args...).Scan(ptrs...); err != nil {
			t.Fatalf("%s : %s : %v", src, sp.SQL, err)
		}
		var got []string
		for _, v := range raw {
			got = append(got, FromResult(v).String())
		}
		if strings.Join(got, "|") != want {
			t.Errorf("%s : %v, attendu %s", src, got, want)
		}
		// Go : la même valeur, sans précalcul.
		_, rows, e := EvalSpill(n, &Env{Schema: f.schema, Data: f.data, Names: names})
		if e.IsErr() || len(rows) != 1 {
			t.Fatalf("%s : Go %v %v", src, rows, e)
		}
	}
	// Valeur de la ligne : pas de forme, donc pas de déversement.
	env := &Env{Schema: f.schema, Data: f.data, Table: f.ventes}
	if _, _, e := EvalSpill(mustParse(t, `=FRACTIONNER.TEXTE([Produit];"v")`, FR), env); !e.IsErr() {
		t.Error("FRACTIONNER.TEXTE([Produit]) se déverse")
	}
}

// TestTextSplitDeps : le texte de FRACTIONNER.TEXTE est une valeur (ses
// cellules sont des dépendances), pas un tableau source.
func TestTextSplitDeps(t *testing.T) {
	names, tables := Deps(mustParse(t, `=FRACTIONNER.TEXTE(Cell[A1]; Cell[B1])`, FR))
	if len(tables) != 0 || len(names) != 2 {
		t.Errorf("dépendances : %v %v", names, tables)
	}
	names, tables = Deps(mustParse(t, `=SEQUENCE(Cell[A1])`, FR))
	if len(tables) != 0 || len(names) != 1 {
		t.Errorf("SEQUENCE : %v %v", names, tables)
	}
}
