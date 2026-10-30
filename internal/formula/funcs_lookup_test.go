package formula

import (
	"errors"
	"testing"
)

// TestConvertLookupModes : RECHERCHEV approchée, RECHERCHEV et RECHERCHEH
// sur des cellules libres, RECHERCHEX avec ses modes. Les valeurs suivent
// les exemples des définitions du tableur (barème, plus grande valeur
// inférieure ou égale ; recherche depuis la fin).
func TestConvertLookupModes(t *testing.T) {
	f := fixture(t)
	res := fakeResolver{"Baremes!A2:B9": {"Baremes", 0}, "Etapes!A2:B9": {"Etapes", 0}}
	for _, c := range []struct{ src, conv, want string }{
		// Approchée sur un tableau : la plus grande valeur inférieure ou égale.
		{`=RECHERCHEV(250;Baremes!A2:B9;2)`, `=INDEX(Baremes[Taux]; EQUIV(250; Baremes[Seuil]; 1))`, "0,05"},
		{`=RECHERCHEV(500;Baremes!A2:B9;2;VRAI)`, `=INDEX(Baremes[Taux]; EQUIV(500; Baremes[Seuil]; 1))`, "0,1"},
		{`=RECHERCHEV(-1;Baremes!A2:B9;2)`, `=INDEX(Baremes[Taux]; EQUIV(-1; Baremes[Seuil]; 1))`, "#N/A"},
		// Cellules libres : A1:B3 vaut 1, 2, "x" et 10, 20, 30.
		{`=RECHERCHEV(2;Cell[A1:B3];2;FAUX)`, `=INDEX(Cell[B1:B3]; EQUIV(2; Cell[A1:A3]; 0))`, "20"},
		{`=RECHERCHEV(2,5;Cell[A1:B2];2)`, `=INDEX(Cell[B1:B2]; EQUIV(2,5; Cell[A1:A2]; 1))`, "20"},
		{`=RECHERCHEH(10;Cell[A1:B3];3;FAUX)`, `=INDEX(Cell[A3:B3]; EQUIV(10; Cell[A1:B1]; 0))`, "30"},
		{`=RECHERCHEH(1;Cell[A1:B3];2;FAUX)`, `=INDEX(Cell[A2:B2]; EQUIV(1; Cell[A1:B1]; 0))`, "2"},
		{`=RECHERCHEX(3;Cell[A1:A3];Cell[B1:B3];"?")`, `=SI.NON.DISP(INDEX(Cell[B1:B3]; EQUIVX(3; Cell[A1:A3])); "?")`, "?"},
		// Modes de RECHERCHEX.
		{`=RECHERCHEX(250;Baremes[Seuil];Baremes[Taux];;-1)`, `=INDEX(Baremes[Taux]; EQUIVX(250; Baremes[Seuil]; -1))`, "0,05"},
		{`=RECHERCHEX(250;Baremes[Seuil];Baremes[Taux];;1)`, `=INDEX(Baremes[Taux]; EQUIVX(250; Baremes[Seuil]; 1))`, "0,1"},
		{`=RECHERCHEX(25;Etapes[Valeur];Etapes[Nom];"aucun";1)`, `=SI.NON.DISP(INDEX(Etapes[Nom]; EQUIVX(25; Etapes[Valeur]; 1)); "aucun")`, "C"},
		{`=RECHERCHEX(99;Etapes[Valeur];Etapes[Nom];"aucun";1)`, `=SI.NON.DISP(INDEX(Etapes[Nom]; EQUIVX(99; Etapes[Valeur]; 1)); "aucun")`, "aucun"},
		{`=RECHERCHEX("b*";Etapes[Nom];Etapes[Valeur];;2)`, `=INDEX(Etapes[Valeur]; EQUIVX("b*"; Etapes[Nom]; 2))`, "20"},
		{`=RECHERCHEX(VRAI;Ventes[Payé];Ventes[Produit];;0;-1)`, `=INDEX(Ventes[Produit]; EQUIVX(VRAI; Ventes[Payé]; 0; -1))`, "Écran"},
	} {
		n := mustParse(t, c.src, FR)
		out, keys, err := Convert(n, f.schema, res, c.src)
		if err != nil || len(keys) != 0 {
			t.Errorf("%s : %v, clés %v", c.src, err, keys)
			continue
		}
		if got := Format(out, FR); got != c.conv {
			t.Errorf("%s : %s, attendu %s", c.src, got, c.conv)
		}
		if got := f.evalGo(out, 0).String(); got != c.want {
			t.Errorf("%s : Go donne %s, attendu %s", c.src, got, c.want)
		}
		if got := f.evalSQL(t, out, 0).String(); got != c.want {
			t.Errorf("%s : SQL donne %s, attendu %s", c.src, got, c.want)
		}
	}
	// Une plage A1 de cellules libres reste une plage A1 : ConvertA1 la
	// situera ensuite (Cell[…], ou un tableau qu'elle recouvre).
	src := `=RECHERCHEH(5;Feuil2!$B$2:$F$4;3;FAUX)`
	out, _, err := Convert(mustParse(t, src, FR), f.schema, res, src)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := Format(out, FR), `=INDEX(Feuil2!$B$2:$F$4; EQUIV(5; Feuil2!$B$2:$F$4; 0))`; got == want {
		t.Fatalf("plages non découpées : %s", got)
	}
	call := out.(Call)
	if r, ok := call.Args[0].(Range); !ok || r.Text != "Feuil2!B4:F4" {
		t.Fatalf("ligne renvoyée : %#v", call.Args[0])
	}
	if r, ok := call.Args[1].(Call).Args[1].(Range); !ok || r.Text != "Feuil2!B2:F2" {
		t.Fatalf("ligne cherchée : %#v", call.Args[1])
	}
	var ce *ErrConvert
	if _, _, err := Convert(mustParse(t, `=RECHERCHEH(1;Cell[A1:B3];4;FAUX)`, FR), f.schema, res, ""); !errors.As(err, &ce) {
		t.Fatalf("ligne hors plage : %v", err)
	}
}
