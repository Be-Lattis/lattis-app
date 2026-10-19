package canvas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/formula"
)

// lookupFixture : Clients (N°, Client) en A1, Commandes (Ligne, ID) en E1,
// et l'en-tête d'une colonne à calculer en G1.
func lookupFixture(t *testing.T, clients [][]string) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, append([][]string{{"N°", "Client"}}, clients...))
	must(t, err)
	_, err = d.Paste(ctx, 1, 0, 4, [][]string{{"Ligne", "ID"}, {"2", "100001"}})
	must(t, err)
	mustSet(t, d, 1, 0, 6, "Calcul")
	return d
}

var threeClients = [][]string{{"100001", "Anna Simon"}, {"100002", "Chloé Faure"}, {"100003", "Théo Simon"}}

// REFERENCE dans un tableau sans clé ne pose pas de clé en silence : la
// saisie demande si la recherche désigne les lignes par leur numéro ou par
// la première colonne, qui deviendrait la clé (choix de Louis, 19/10/2026).
func TestReferenceAsksForKey(t *testing.T) {
	ctx := context.Background()
	d := lookupFixture(t, threeClients)
	before := dump(t, d)
	_, err := d.SetCell(ctx, 1, 1, 6, `=REFERENCE(Feuille1[Client]; [Ligne])`)
	if err != (ErrKeyChoice{Table: "Feuille1", Column: "N°"}) {
		t.Fatalf("question attendue : %v", err)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("saisie refusée mais classeur modifié :\n%s", diff(before, got))
	}
	// Numéro de ligne : pas de clé, la formule lit la 2e ligne ; le tableau
	// ne propose plus de clé et les formules suivantes ne redemandent rien.
	if _, err := d.SetCell(WithKeyChoice(ctx, KeyIndex), 1, 1, 6, `=REFERENCE(Feuille1[Client]; [Ligne])`); err != nil {
		t.Fatal(err)
	}
	if ts := tablesOf(t, d); ts[0].Key != -1 || ts[0].KeyHint {
		t.Fatalf("clé %d, proposée %v", ts[0].Key, ts[0].KeyHint)
	}
	if c := computedOf(t, d, 2, 2); c != "Chloé Faure" {
		t.Fatalf("par numéro de ligne : %q", c)
	}
	mustSet(t, d, 1, 6, 0, `=REFERENCE(Feuille1[Client]; 3)`)
	if c := cellOn(t, d, 1, 6, 0).Value; c != "Théo Simon" {
		t.Fatalf("cellule libre, par numéro : %q", c)
	}
	mustSet(t, d, 1, 6, 2, `=EXISTE(Feuille1; 4)`)
	if c := cellOn(t, d, 1, 6, 2).Value; c != "FAUX" {
		t.Fatalf("EXISTE par numéro : %q", c)
	}
	d.bus.Undo(ctx)
	d.bus.Undo(ctx)
	undoOnce(t, d, before, "formule par numéro de ligne")
}

// Clé choisie : la première colonne devient la clé et la formule est
// enregistrée dans la même étape d'annulation.
func TestReferenceKeyColumn(t *testing.T) {
	ctx := context.Background()
	d := lookupFixture(t, threeClients)
	before := dump(t, d)
	ev, err := d.SetCell(WithKeyChoice(ctx, KeyColumn), 1, 1, 6, `=REFERENCE(Feuille1[Client]; [ID])`)
	if err != nil || ev.Keyed != "Feuille1.N°" {
		t.Fatalf("SetCell = %+v, %v", ev, err)
	}
	if k := tablesOf(t, d)[0].Key; k != 0 {
		t.Fatalf("clé : %d", k)
	}
	if c := computedOf(t, d, 2, 2); c != "Anna Simon" {
		t.Fatalf("par clé : %q", c)
	}
	undoOnce(t, d, before, "formule et clé")
}

// Tableau qui a déjà une clé : rien n'est demandé.
func TestReferenceKeyedTable(t *testing.T) {
	ctx := context.Background()
	d := lookupFixture(t, threeClients)
	must(t, d.SetKey(ctx, "Feuille1", 0))
	before := dump(t, d)
	mustSet(t, d, 1, 1, 6, `=RECHERCHEV([@ID]; Feuille1; 2; FAUX)`)
	if c := computedOf(t, d, 2, 2); c != "Anna Simon" {
		t.Fatalf("par clé : %q", c)
	}
	undoOnce(t, d, before, "formule sur un tableau à clé")
}

// RECHERCHEV dans un tableau aux doublons : la colonne ne peut pas devenir
// la clé (la question le dit) ; par numéro de ligne, elle devient
// INDEX/EQUIV, la première ligne trouvée.
func TestVlookupByRowNumber(t *testing.T) {
	ctx := context.Background()
	d := lookupFixture(t, [][]string{{"100001", "Anna Simon"}, {"100001", "Chloé Faure"}, {"100003", "Théo Simon"}})
	before := dump(t, d)
	const src = `=RECHERCHEV([@ID]; Feuille1; 2; FAUX)`
	_, err := d.SetCell(ctx, 1, 1, 6, src)
	if err != (ErrKeyChoice{Table: "Feuille1", Column: "N°", Why: "1 valeur(s) en double"}) {
		t.Fatalf("question attendue : %v", err)
	}
	if _, err := d.SetCell(WithKeyChoice(ctx, KeyColumn), 1, 1, 6, src); !errors.As(err, &ErrKey{}) {
		t.Fatalf("clé sur une colonne aux doublons : %v", err)
	}
	if _, err := d.SetCell(WithKeyChoice(ctx, KeyIndex), 1, 1, 6, src); err != nil {
		t.Fatal(err)
	}
	ts := tablesOf(t, d)
	if f := ts[1].Columns[2].Formula; f != "=INDEX(Feuille1[Client]; EQUIV([ID]; Feuille1[N°]; 0))" {
		t.Fatalf("formule : %q", f)
	}
	if c := computedOf(t, d, 2, 2); c != "Anna Simon" {
		t.Fatalf("première ligne trouvée : %q", c)
	}
	undoOnce(t, d, before, "RECHERCHEV par numéro de ligne")
}

// computedOf lit la première ligne d'une colonne calculée (lecture par fenêtres).
func computedOf(t *testing.T, d *fixture, table int64, col int) string {
	t.Helper()
	exprs, args, _, err := d.ColumnExprs(context.Background(), table)
	if err != nil {
		t.Fatal(err)
	}
	// Toutes les expressions ensemble : elles partagent leurs paramètres.
	got := make([]any, len(exprs))
	ptrs := make([]any, len(exprs))
	for i := range got {
		ptrs[i] = &got[i]
	}
	err = d.read(context.Background(), func(x *tx) error {
		return x.row(`SELECT `+strings.Join(exprs, ", ")+fmt.Sprintf(` FROM t_%d ORDER BY _pos LIMIT 1`, table), args...).Scan(ptrs...)
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(formula.FromResult(got[col]))
}
