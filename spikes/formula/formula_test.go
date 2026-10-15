package formula

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"slices"
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
