package formula

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type fakeResolver map[string][2]any

func (r fakeResolver) Resolve(s string) (string, int, bool) {
	v, ok := r[s]
	if !ok {
		return "", 0, false
	}
	return v[0].(string), v[1].(int), true
}

func TestConvertVLOOKUP(t *testing.T) {
	f := fixture(t)
	f.schema.Tables[1].Key = "" // Clients sans clé : la conversion la propose
	res := fakeResolver{"Clients!$A$2:$C$100": {"Clients", 0}, "Clients!B2:C100": {"Clients", 1}}

	src := `=SIERREUR(RECHERCHEV([@Client];Clients!$A$2:$C$100;3;FAUX);"?")`
	n := mustParse(t, src, FR)
	out, keys, err := Convert(n, f.schema, res, src)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := Format(out, FR), `=SIERREUR(REFERENCE(Clients; Ville; [@Client]); "?")`; got != want {
		t.Fatalf("conversion : %s, attendu %s", got, want)
	}
	if origin := out.(Call).Args[0].(Call).Origin; origin != src {
		t.Fatalf("origine perdue : %q", origin)
	}
	if !reflect.DeepEqual(keys, []KeyNeed{{"Clients", "Id"}}) {
		t.Fatalf("clés : %v", keys)
	}
	err = f.db.Write(context.Background(), func(tx *sql.Tx) error { return EnsureKey(context.Background(), tx, f.schema, keys[0]) })
	if err != nil {
		t.Fatal(err)
	}
	if got := f.evalGo(out, 1).String(); got != "Paris" {
		t.Fatalf("Go : %s", got)
	}
	if got := f.evalSQL(t, out, 1).String(); got != "Paris" {
		t.Fatalf("SQL : %s", got)
	}

	for _, bad := range []string{
		`=RECHERCHEV(1;Clients!$A$2:$C$100;2)`,      // approchée par défaut
		`=RECHERCHEV(1;Clients!$A$2:$C$100;2;VRAI)`, // approchée explicite
		`=RECHERCHEV(1;Feuil9!A1:B2;2;FAUX)`,        // plage hors tableau
		`=RECHERCHEV(1;Clients!$A$2:$C$100;7;FAUX)`, // colonne hors plage
		`=RECHERCHEV(1;Clients!B2:C100;2;FAUX)`,     // autre clé que celle du tableau
	} {
		var ce *ErrConvert
		if _, _, err := Convert(mustParse(t, bad, FR), f.schema, res, bad); !errors.As(err, &ce) {
			t.Errorf("%s : erreur de conversion attendue, obtenu %v", bad, err)
		}
	}
}

func TestEnsureKeyRejectsDuplicates(t *testing.T) {
	f := fixture(t)
	err := f.db.Write(context.Background(), func(tx *sql.Tx) error {
		return EnsureKey(context.Background(), tx, f.schema, KeyNeed{"Ventes", "Payé"})
	})
	if !errors.Is(err, ErrNotUnique) {
		t.Fatalf("doublons acceptés : %v", err)
	}
	// Une fois la clé posée, un doublon est refusé à la saisie.
	err = f.db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO t_c VALUES (9, 'z', 2, 'Doublon', 'Brest')`)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "UNIQUE") {
		t.Fatalf("doublon de clé accepté : %v", err)
	}
}

func TestNamesGraph(t *testing.T) {
	f := fixture(t)
	if got := f.names.Affected([]string{"ventes"}, nil); !slices.Equal(got, []string{"ca", "ca_ttc"}) {
		t.Fatalf("Affected(Ventes) = %v", got)
	}
	if got := f.names.Affected(nil, []string{"TVA"}); !slices.Equal(got, []string{"ca_ttc", "tva"}) && !slices.Equal(got, []string{"tva", "ca_ttc"}) {
		t.Fatalf("Affected(TVA) = %v", got)
	}
	if v := f.names.Values["boucle2"]; v.String() != "#REF!" {
		t.Fatalf("cycle : %v", v)
	}
}

func TestParseErrors(t *testing.T) {
	for _, src := range []string{`=1+`, `=SOMME(1;2`, `="abc`, `=[@Prix`, `=SI()`, `=1 2`, `=#FOO!`} {
		var se *SyntaxError
		if _, err := Parse(src, FR); !errors.As(err, &se) {
			t.Errorf("%s : erreur de syntaxe attendue, obtenu %v", src, err)
		}
	}
	// Un nom de fonction inconnu reste dans l'AST (affiché #NOM? à l'évaluation).
	if _, err := Parse(`=DECALER(1;2)`, FR); err != nil {
		t.Errorf("fonction inconnue refusée à l'analyse : %v", err)
	}
}

func TestFormatLocales(t *testing.T) {
	n := mustParse(t, `=SI([@Prix]>1,5;SOMME(Ventes[Montant]);"a""b")`, FR)
	if got, want := Format(n, EN), `=IF([@Prix]>1.5, SUM(Ventes[Montant]), "a""b")`; got != want {
		t.Fatalf("EN : %s, attendu %s", got, want)
	}
	if got, want := Format(n, FR), `=SI([@Prix]>1,5; SOMME(Ventes[Montant]); "a""b")`; got != want {
		t.Fatalf("FR : %s, attendu %s", got, want)
	}
}

func TestConvertXLOOKUPAndIndexMatch(t *testing.T) {
	f := fixture(t)
	res := fakeResolver{"Clients!A2:A100": {"Clients", 0}, "Clients!C2:C100": {"Clients", 2}}
	for src, want := range map[string]string{
		`=RECHERCHEX([@Client];Clients[Id];Clients[Ville])`:           `=REFERENCE(Clients; Ville; [@Client])`,
		`=RECHERCHEX([@Client];Clients[Id];Clients[Ville];"?")`:       `=SI.NON.DISP(REFERENCE(Clients; Ville; [@Client]); "?")`,
		`=RECHERCHEX([@Client];Clients!A2:A100;Clients!C2:C100)`:      `=REFERENCE(Clients; Ville; [@Client])`,
		`=INDEX(Clients[Nom];EQUIV([@Client];Clients[Id];0))`:         `=REFERENCE(Clients; Nom; [@Client])`,
		`=INDEX(Clients[Nom];EQUIV([@Client];Clients[Id]))`:           `=INDEX(Clients[Nom]; EQUIV([@Client]; Clients[Id]))`,
		`=SIERREUR(INDEX(Clients[Nom];EQUIV(2;Clients[Id];FAUX));"")`: `=SIERREUR(REFERENCE(Clients; Nom; 2); "")`,
	} {
		out, keys, err := Convert(mustParse(t, src, FR), f.schema, res, src)
		if err != nil {
			t.Errorf("%s : %v", src, err)
			continue
		}
		if got := Format(out, FR); got != want {
			t.Errorf("%s : %s, attendu %s", src, got, want)
		}
		if strings.HasPrefix(want, "=INDEX") != (len(keys) == 0) {
			t.Errorf("%s : clés %v", src, keys)
		}
		if got := f.evalGo(out, 1).String(); strings.HasPrefix(want, "=REFERENCE(Clients; Ville") && got != "Paris" {
			t.Errorf("%s : Go donne %s", src, got)
		}
	}
	for _, bad := range []string{
		`=RECHERCHEX(1;Clients[Id];Ventes[Prix])`,      // deux tableaux
		`=RECHERCHEX(1;Clients[Nom];Clients[Ville])`,   // autre clé que celle du tableau
		`=RECHERCHEX(1;Clients[Id];Clients[Nom];"";1)`, // correspondance approchée
		`=RECHERCHEH(1;Clients;2;FAUX)`,
	} {
		var ce *ErrConvert
		if _, _, err := Convert(mustParse(t, bad, FR), f.schema, res, bad); !errors.As(err, &ce) {
			t.Errorf("%s : erreur de conversion attendue, obtenu %v", bad, err)
		}
	}
}

func TestKeyTargetsAndSuggestion(t *testing.T) {
	n := mustParse(t, `=SI(EXISTE(Clients;1);REFERENCE.APPROCHEE(Baremes;Taux;[@Prix]);REFERENCE(Produits;Nom;1))`, FR)
	if got := KeyTargets(n); !slices.Equal(got, []string{"Clients", "Baremes", "Produits"}) {
		t.Fatalf("KeyTargets = %v", got)
	}
	if Suggestion("decaler") == "" || Suggestion("SOMME") != "" {
		t.Fatal("suggestions")
	}
}

// TestCatalogueNames : chaque nom (français ou anglais) désigne une seule
// fonction, et le nom anglais est la forme canonique.
func TestCatalogueNames(t *testing.T) {
	seen := map[string]string{}
	for _, f := range funcs {
		for _, name := range []string{f.EN, f.FR} {
			if other, ok := seen[name]; ok && other != f.EN {
				t.Errorf("%s désigne %s et %s", name, other, f.EN)
			}
			seen[name] = f.EN
		}
		if (f.kind == scalar) != (f.impl != nil) && f.EN != "RANDBETWEEN" {
			t.Errorf("%s : implémentation incohérente avec son genre", f.EN)
		}
		if _, ok := scans[f.EN]; ok != (f.kind == scan) && f.EN != "REFERENCE.APPROX" {
			t.Errorf("%s : déclaré en parcours sans scanner, ou l'inverse", f.EN)
		}
	}
	t.Logf("%d fonctions", len(funcs))
}

func TestAggRoundTrip(t *testing.T) {
	a := Agg{Fn: "TEXTJOIN"}
	for _, v := range []Value{Str("a"), Value{}, Num(1.5), DateSerial(46023)} {
		a.AddRange(v)
	}
	b, ok := decodeAgg(encodeAgg(a))
	if !ok || !reflect.DeepEqual(a, Agg{Fn: a.Fn, Items: b.Items, Vals: b.Vals, Sum: b.Sum, Comp: b.Comp, N: b.N, Nonempty: b.Nonempty, Blank: b.Blank, Dates: b.Dates, Min: b.Min, Max: b.Max, Prod: b.Prod}) {
		t.Fatalf("aller-retour : %+v → %+v", a, b)
	}
	if got := b.TextResult(Str("|"), Boolean(false)).String(); got != "a||1,5|01/01/2026" {
		t.Fatalf("TEXTJOIN : %s", got)
	}
	for _, v := range []Value{Num(1), Str("x"), Boolean(true), Err(ErrNA), DateSerial(2), Value{}} {
		if back, ok := decodeValue(encodeValue(v)); !ok || back != v {
			t.Errorf("encodeValue(%v) → %v", v, back)
		}
	}
}

// TestSommeCompensee : la somme d'un million de parts vaut exactement 1.
func TestSommeCompensee(t *testing.T) {
	a := Agg{Fn: "SUM"}
	for range 1_000_000 {
		a.AddRange(Num(1.0 / 1_000_000))
	}
	if got := a.Result().String(); got != "1" {
		t.Fatalf("somme : %s", got)
	}
}

// TestHoistScans : un NB.SI ou un EQUIV qui ne dépend pas de la ligne est
// calculé une fois ; s'il lit la ligne, il reste dans la requête.
func TestHoistScans(t *testing.T) {
	f := fixture(t)
	calls := 0
	hoist := func(c Compiled) Value {
		calls++
		var v any
		if err := f.db.Read().QueryRow(`SELECT `+c.SQL, c.Args...).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return FromResult(v)
	}
	for src, hoisted := range map[string]bool{
		`=[@Qté]/NB.SI(Ventes[Payé];VRAI)`:                         true,
		`=[@Qté]*SOMMEPROD(Ventes[Prix];Ventes[Qté])`:              true,
		`=NB.SI(Ventes[Client];[@Client])`:                         true, // totaux par valeur
		`=RANG([@Qté])`:                                            true, // valeurs triées
		`=RANG([@Qté];1)`:                                          true,
		`=CUMUL([@Montant])`:                                       true, // cumuls précalculés
		`=PRECEDENT([@Qté];0)+[@Qté]`:                              false,
		`=EQUIV(3;[Qté];0)`:                                        true,
		`=SOMME.SI(Ventes[Payé];[@Payé];Ventes[Qté])`:              true,
		`=NB.SI.ENS(Ventes[Note];[@Note];Ventes[Payé];VRAI)`:       true,
		`=NB.SI(Ventes[Note];[@Client]&"")`:                        true,
		`=MOYENNE.SI(Ventes[Produit];[@Produit]&"*";Ventes[Prix])`: true,
	} {
		calls = 0
		c := CompileHoisted(mustParse(t, src, FR), f.schema, nil, f.ventes, "o", hoist)
		if (calls > 0) != hoisted {
			t.Errorf("%s : calculé à part %d fois, attendu %v", src, calls, hoisted)
		}
		for row := 1; row <= 4; row++ {
			var v any
			if err := f.db.Read().QueryRow(`SELECT `+c.SQL+` FROM t_v AS o WHERE o.id = `+strconv.Itoa(row), c.Args...).Scan(&v); err != nil {
				t.Fatalf("%s : %v", src, err)
			}
			if got, want := FromResult(v).String(), f.evalGo(mustParse(t, src, FR), row).String(); got != want {
				t.Errorf("%s ligne %d : SQL %s, Go %s", src, row, got, want)
			}
		}
	}
}

// TestRandStable : ALEA ne change pas d'une lecture à l'autre, mais change
// d'une ligne à l'autre et quand la graine change (F9).
func TestRandStable(t *testing.T) {
	f := fixture(t)
	n := mustParse(t, `=ALEA()`, FR)
	a, b := f.evalSQL(t, n, 1), f.evalSQL(t, n, 1)
	if a != b || a == f.evalSQL(t, n, 2) {
		t.Fatalf("ALEA instable ou constant : %v %v %v", a, b, f.evalSQL(t, n, 2))
	}
	f.schema.Seed++
	if f.evalSQL(t, n, 1) == a {
		t.Fatal("ALEA ne change pas avec la graine")
	}
}

// TestSpill : un tableau dynamique donne les mêmes en-têtes et lignes sur les
// deux backends.
func TestSpill(t *testing.T) {
	f := fixture(t)
	render := func(cols []string, rows [][]Value) string {
		out := []string{strings.Join(cols, "|")}
		for _, r := range rows {
			var cells []string
			for _, v := range r {
				cells = append(cells, v.String())
			}
			out = append(out, strings.Join(cells, "|"))
		}
		return strings.Join(out, " / ")
	}
	for src, want := range map[string]string{
		`=FILTRE(Clients;Clients[Ville]<>"Lyon")`:                  `Id|Nom|Ville / 1|Dupont|Paris / 3|Durand|Nice`,
		`=TRIER(Clients;3)`:                                        `Id|Nom|Ville / 2|Martin|Lyon / 3|Durand|Nice / 1|Dupont|Paris`,
		`=TRIER(Ventes[Prix];1;-1)`:                                `Prix / 199,99 / 25,5 / 10 / `,
		`=UNIQUE(Ventes[Payé])`:                                    `Payé / VRAI / FAUX / `,
		`=UNIQUE(Ventes[Payé];FAUX;VRAI)`:                          `Payé / FAUX / `,
		`=GROUPER.PAR(Ventes[Payé];Ventes[Montant];SOMME)`:         `Payé|Montant / FAUX|30 / VRAI|250,99 / |0`,
		`=PRENDRE(TRIERPAR(Ventes[Produit];Ventes[Montant];-1);2)`: `Produit / Écran / Clavier`,
		`=FILTRE(Ventes[Date];Ventes[Date]>=DATE(2026;1;1))`:       `Date / 15/01/2026 / 28/02/2026`,
		`=EXCLURE(FILTRE(Etapes;Etapes[Valeur]>0);-2)`:             `Nom|Valeur / A|10`,
	} {
		n := mustParse(t, src, FR)
		cols, rows, e := EvalSpill(n, &Env{Schema: f.schema, Data: f.data})
		if e.IsErr() {
			t.Errorf("%s : Go %v", src, e)
		} else if got := render(cols, rows); got != want {
			t.Errorf("%s : Go %q, attendu %q", src, got, want)
		}
		sp, e := CompileSpill(n, f.schema, nil)
		if e.IsErr() {
			t.Errorf("%s : SQL %v", src, e)
			continue
		}
		rs, err := f.db.Read().Query(sp.SQL, sp.Args...)
		if err != nil {
			t.Fatalf("%s : %s : %v", src, sp.SQL, err)
		}
		var sqlRows [][]Value
		for rs.Next() {
			raw := make([]any, len(sp.Columns))
			ptrs := make([]any, len(raw))
			for i := range raw {
				ptrs[i] = &raw[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			row := make([]Value, len(raw))
			for i, v := range raw {
				row[i] = FromResult(v)
			}
			sqlRows = append(sqlRows, row)
		}
		rs.Close()
		if got := render(sp.Columns, sqlRows); got != want {
			t.Errorf("%s : SQL %q, attendu %q", src, got, want)
		}
	}
	if !IsSpill(mustParse(t, `=TRIER(Clients)`, FR)) || IsSpill(mustParse(t, `=SOMME(1)`, FR)) {
		t.Error("IsSpill")
	}
	sp, e := CompileSpill(mustParse(t, `=REQUETE("SELECT 1 AS a, 'x' AS b")`, FR), f.schema, nil)
	if e.IsErr() || sp.Columns != nil {
		t.Fatalf("REQUETE : %v %v", e, sp)
	}
	var a any
	var b string
	if err := f.db.Read().QueryRow(sp.SQL).Scan(&a, &b); err != nil || b != "x" {
		t.Fatalf("REQUETE : %v %q", err, b)
	}
}

// TestFonctionsDocumentees : chaque fonction figure dans docs/fonctions.md.
func TestFonctionsDocumentees(t *testing.T) {
	doc, err := os.ReadFile("../../docs/fonctions.md")
	if err != nil {
		t.Fatal(err)
	}
	words := map[string]bool{}
	for _, w := range strings.FieldsFunc(string(doc), func(r rune) bool { return r == ' ' || r == ',' || r == '|' || r == '\n' || r == '`' || r == '(' }) {
		words[w] = true
	}
	for _, f := range funcs {
		if !words[f.FR] || !words[f.EN] && f.EN != f.FR && !strings.Contains(string(doc), f.EN) {
			t.Errorf("%s / %s absente de docs/fonctions.md", f.FR, f.EN)
		}
	}
}

func TestFunctionsCatalog(t *testing.T) {
	fr := Functions(FR)
	if len(fr) < 20 {
		t.Fatalf("%d fonctions", len(fr))
	}
	found := false
	for _, f := range fr {
		if f.Name == "SI" && f.Other == "IF" && f.Arity == "de 2 à 3 arguments" {
			found = true
		}
	}
	if !found {
		t.Fatal("SI absent ou mal décrit")
	}
}
