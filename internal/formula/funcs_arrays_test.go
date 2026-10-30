package formula

import "testing"

// TestArrayShape : ASSEMB.V, ASSEMB.H, CHOISIRCOLS, CHOISIRLIGNES,
// VERS.COL, VERS.LIGNE et le 3e argument de PRENDRE / EXCLURE, sur les deux
// moteurs (en-têtes et lignes déversées). Cell[A1:B3] vaut 1, 10 / 2, 20 /
// "x", 30 ; Etapes s'affiche A 10, B 20, C 30.
func TestArrayShape(t *testing.T) {
	f := fixture(t)
	for _, c := range []struct{ src, want string }{
		{`=ASSEMB.V(Etapes[Nom];Clients[Nom])`, "Nom / A / B / C / Dupont / Martin / Durand"},
		{`=ASSEMB.V(Etapes;Clients[Ville])`, "Nom|Valeur / A|10 / B|20 / C|30 / Paris|#N/A / Lyon|#N/A / Nice|#N/A"},
		{`=ASSEMB.V({1.2};{3})`, "Valeur1|Valeur2 / 1|2 / 3|#N/A"},
		{`=ASSEMB.H(Etapes[Nom];Cell[B1:B2])`, "Nom|Valeur / A|10 / B|20 / C|#N/A"},
		{`=ASSEMB.H(Clients[Nom];Clients[Nom])`, "Nom|Nom2 / Dupont|Dupont / Martin|Martin / Durand|Durand"},
		{`=CHOISIRCOLS(Clients;3;1)`, "Ville|Id / Paris|1 / Lyon|2 / Nice|3"},
		{`=CHOISIRCOLS(Clients;-1)`, "Ville / Paris / Lyon / Nice"},
		{`=CHOISIRCOLS(Clients;4)`, "#VALEUR!"},
		{`=CHOISIRCOLS(Clients;Cell[A1])`, "#VALEUR!"},
		{`=FILTRE(CHOISIRCOLS(Clients;2);Clients[Id]>1)`, "Nom / Martin / Durand"},
		{`=CHOISIRLIGNES(Clients[Nom];1;-1)`, "Nom / Dupont / Durand"},
		{`=CHOISIRLIGNES(Clients;Cell[A2])`, "Id|Nom|Ville / 2|Martin|Lyon"},
		{`=CHOISIRLIGNES(Clients[Nom];5)`, "Nom / #VALEUR!"},
		{`=VERS.COL(Cell[A1:B3])`, "Valeur / 1 / 10 / 2 / 20 / x / 30"},
		{`=VERS.COL(Cell[A1:B3];0;VRAI)`, "Valeur / 1 / 2 / x / 10 / 20 / 30"},
		{`=VERS.COL(Cell[A3:B4];1)`, "Valeur / x / 30"},
		{`=VERS.COL({1;#N/A;3};2)`, "Valeur / 1 / 3"},
		{`=VERS.COL(Clients;0;1)`, "Valeur / 1 / 2 / 3 / Dupont / Martin / Durand / Paris / Lyon / Nice"},
		{`=VERS.COL(Cell[A1:B3];4)`, "#VALEUR!"},
		{`=VERS.LIGNE(Cell[A1:A3])`, "Valeur1|Valeur2|Valeur3 / 1|2|x"},
		{`=VERS.LIGNE(Cell[A1:B2];0;VRAI)`, "Valeur1|Valeur2|Valeur3|Valeur4 / 1|2|10|20"},
		{`=VERS.LIGNE(Clients[Nom])`, "#VALEUR!"},
		{`=PRENDRE(Clients;2;-1)`, "Ville / Paris / Lyon"},
		{`=PRENDRE(Clients;;2)`, "Id|Nom / 1|Dupont / 2|Martin / 3|Durand"},
		{`=EXCLURE(Clients;1;1)`, "Nom|Ville / Martin|Lyon / Durand|Nice"},
		{`=PRENDRE(Clients;1;0)`, "#VALEUR!"},
		{`=PRENDRE(Clients;1;Cell[A1])`, "#VALEUR!"},
	} {
		n := mustParse(t, c.src, FR)
		if got := f.spillGo(n); got != c.want {
			t.Errorf("%s : Go donne %q, attendu %q", c.src, got, c.want)
		}
		if got := f.spillSQL(t, n); got != c.want {
			t.Errorf("%s : SQL donne %q, attendu %q", c.src, got, c.want)
		}
	}
}

// TestStackDeps : chaque tableau nommé d'ASSEMB.V est une dépendance et suit
// un renommage.
func TestStackDeps(t *testing.T) {
	n := mustParse(t, `=ASSEMB.V(Ventes;Clients;Etapes[Nom])`, FR)
	_, tables := Deps(n)
	want := map[string]bool{"ventes": true, "clients": true, "etapes": true}
	for _, tb := range tables {
		delete(want, tb)
	}
	if len(want) != 0 {
		t.Fatalf("dépendances %v : manque %v", tables, want)
	}
	got, ok := RenameRefs(`=ASSEMB.V(Ventes;Clients;Etapes[Nom])`, FR, Renames{Tables: map[string]string{"clients": "Acheteurs", "etapes": "Phases"}}, "")
	if !ok || got != `=ASSEMB.V(Ventes;Acheteurs;Phases[Nom])` {
		t.Fatalf("renommage : %q", got)
	}
	out, ok := RetargetTables(n, map[string]string{"clients": "Clients2"})
	if s := Format(out, FR); !ok || s != `=ASSEMB.V(Ventes; Clients2; Etapes[Nom])` {
		t.Fatalf("copie de feuille : %q", s)
	}
}
