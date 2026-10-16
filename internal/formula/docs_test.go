package formula

import (
	"strings"
	"testing"
)

// TestFunctionDocs : chaque fonction du catalogue a sa documentation
// (description, arguments conformes à son nombre d'arguments, exemple qui
// s'analyse en français).
func TestFunctionDocs(t *testing.T) {
	for _, f := range funcs {
		d, ok := docs[f.EN]
		if !ok {
			t.Errorf("%s : pas de documentation", f.EN)
			continue
		}
		if d.cat == "" || len(d.desc) < 15 || !strings.HasSuffix(d.desc, ".") {
			t.Errorf("%s : catégorie ou description manquante (%q)", f.EN, d.desc)
		}
		required, opt := 0, false
		for i, x := range d.args {
			if x.Name == "" || x.Desc == "" || strings.ContainsAny(x.Name, " ;[]") {
				t.Errorf("%s : argument %d mal documenté (%q)", f.EN, i, x.Name)
			}
			if x.Optional {
				opt = true
			} else if opt && i < len(d.args)-d.repeat {
				t.Errorf("%s : argument obligatoire %s après un argument facultatif", f.EN, x.Name)
			} else {
				required++
			}
		}
		if d.repeat > 0 {
			if f.Max >= 0 || d.repeat > len(d.args) {
				t.Errorf("%s : groupe répété incohérent avec Max=%d", f.EN, f.Max)
			}
			for _, x := range d.args[len(d.args)-d.repeat:] {
				if !strings.HasSuffix(x.Name, "1") {
					t.Errorf("%s : l'argument répété %s doit finir par 1", f.EN, x.Name)
				}
			}
		} else if f.Max != len(d.args) && f.EN != "LET" { // LET : paires répétées avant le calcul
			t.Errorf("%s : %d arguments documentés, Max=%d", f.EN, len(d.args), f.Max)
		}
		if required != f.Min && f.EN != "LET" {
			t.Errorf("%s : %d arguments obligatoires documentés, Min=%d", f.EN, required, f.Min)
		}
		if !strings.HasPrefix(d.example, "=") {
			t.Errorf("%s : exemple %q sans =", f.EN, d.example)
		} else if _, err := Parse(d.example, FR); err != nil {
			t.Errorf("%s : exemple %q : %v", f.EN, d.example, err)
		}
		if strings.Contains(d.example, "[@") {
			t.Errorf("%s : exemple %q avec @ (nomenclature sans @)", f.EN, d.example)
		}
		if f.EN != "HLOOKUP" && !strings.Contains(strings.ToUpper(d.example), f.FR+"(") {
			t.Errorf("%s : l'exemple %q n'utilise pas %s", f.EN, d.example, f.FR)
		}
	}
	for en := range docs {
		if _, ok := funcIndex[en]; !ok {
			t.Errorf("documentation de %s, fonction inconnue", en)
		}
	}
}

func TestSignature(t *testing.T) {
	cases := map[string]string{
		"SUMIF":  "SOMME.SI(plage; critère; [plage_somme])",
		"SUM":    "SOMME(valeur1; [valeur2]; …)",
		"SUMIFS": "SOMME.SI.ENS(plage_somme; plage_critères1; critère1; [plage_critères2; critère2]; …)",
		"PI":     "PI()",
	}
	for en, want := range cases {
		if got := docs[en].signature(funcIndex[en].FR); got != want {
			t.Errorf("%s : %q, attendu %q", en, got, want)
		}
	}
	for _, f := range Functions(FR) {
		if f.Name == "SOMME.SI" && (f.Description == "" || len(f.Args) != 3 || f.Example == "" || f.Category != "Agrégats") {
			t.Errorf("SOMME.SI incomplète : %+v", f)
		}
	}
}
