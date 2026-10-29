package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/api"
)

// digestHost : Ventes (30 lignes, clé N°, colonne de référence vers
// Clients, colonne calculée), Clients (sans clé, Code unique), un tableau
// croisé, un graphique, une cellule nommée et une feuille vide.
func digestHost(t *testing.T) *host {
	t.Helper()
	h := newHost(t)
	ctx := context.Background()
	call := func(name string, args map[string]any) {
		t.Helper()
		if _, err := api.Invoke(ctx, h, name, args); err != nil {
			t.Fatalf("%s : %v", name, err)
		}
	}
	call("columns.insert", map[string]any{"table": "Ventes", "name": "TTC"})
	call("cells.set", map[string]any{"row": 2, "col": 3, "value": "=ARRONDI([Montant]*1,2; 2)"})
	call("columns.insert", map[string]any{"table": "Ventes", "name": "N°", "position": 1})
	for i := range 30 {
		call("cells.set", map[string]any{"row": 2 + i, "col": 1, "value": float64(100 + i)})
	}
	call("tables.set_key", map[string]any{"table": "Ventes", "column": "N°"})
	call("cells.paste", map[string]any{"row": 1, "col": 10, "values": []any{
		[]any{"Code", "Nom"}, []any{"C1", "Zorglub"}, []any{"C2", "Quaxine"}, []any{"C3", "Zorglub"},
	}})
	res, _ := api.Invoke(ctx, h, "tables.list", nil)
	for _, t2 := range res.([]api.TableEntry) {
		if t2.Name != "Ventes" {
			call("tables.rename", map[string]any{"table": t2.Name, "name": "Clients"})
		}
	}
	call("pivots.create", map[string]any{"config": map[string]any{"name": "ParClient", "source": "Ventes", "rowFields": []any{"Client"}, "values": []any{map[string]any{"field": "Montant", "agg": "sum"}}}})
	call("charts.create", map[string]any{"row": 2, "col": 15, "sheet": "Feuille1", "config": map[string]any{"source": "Ventes", "kind": "bar", "x": "Client", "series": []any{"Montant"}, "title": "Montant par client"}})
	call("cells.name", map[string]any{"row": 40, "col": 2, "name": "Taux", "sheet": "Feuille1"})
	call("sheets.add", map[string]any{"name": "Notes"})
	return h
}

func TestDigest(t *testing.T) {
	h := digestHost(t)
	ctx := context.Background()
	s := StructureIn(ctx, h, SchemaOnly, "fr")
	for _, want := range []string{
		structureHead, "Ventes · feuille Feuille1 · 30 ligne(s) · clé N°",
		"N° ent. clé", "Client txt 7", "Montant ent. unique",
		"TTC =ARRONDI([Montant]*1,2; 2) → nombre",
		"Clients · feuille Feuille1 · 3 ligne(s) · sans clé (clé possible : Code)", "Code txt unique", "Nom txt 2",
		"Tableau croisé ParClient · feuille", "← Ventes · lignes Client · valeurs somme Montant",
		"Graphique « Montant par client » · feuille Feuille1 ← Ventes · bar · x Client · séries Montant",
		"Cellules nommées : Taux (Feuille1!B40)", "Feuilles sans tableau : Notes (vide)", "Repères",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("digest sans %q :\n%s", want, s)
		}
	}
	// Aucune valeur de cellule, même en partage complet (le digest ne lit
	// que la structure et des comptes).
	for _, mode := range []string{SchemaOnly, Sample, Full} {
		s := StructureIn(ctx, h, mode, "fr")
		for _, v := range []string{"Zorglub", "Quaxine", "C1", "100"} {
			if strings.Contains(s, v) {
				t.Errorf("%s : valeur %q dans le digest :\n%s", mode, v, s)
			}
		}
	}
	// Formules dans l'écriture de l'interface ; erreurs comptées.
	if _, err := api.Invoke(ctx, h, "cells.set", map[string]any{"row": 2, "col": 4, "value": "=SI([Montant]>100;1/0;1)"}); err != nil {
		t.Fatal(err)
	}
	en := StructureIn(ctx, h, SchemaOnly, "en")
	if !strings.Contains(en, "TTC =IF([Montant]>100, 1/0, 1) → nombre, erreurs : 19 #DIV/0!") {
		t.Errorf("digest en anglais :\n%s", en)
	}
	// La mise à jour garde le digest, sous son propre titre.
	if u := StructureUpdate(s); !strings.HasPrefix(u, structureChanged) || !strings.Contains(u, "clé N°") {
		t.Errorf("mise à jour : %s", u)
	}
}

// Les tableaux croisés dont le champ en colonnes porte des valeurs : sans
// partage des données, le digest n'en cite pas les colonnes.
func TestDigestSharing(t *testing.T) {
	h := valuesHost(t)
	ctx := context.Background()
	s := StructureIn(ctx, h, SchemaOnly, "fr")
	if !strings.Contains(s, "Tableau croisé TCD1") || !strings.Contains(s, "colonnes Nom") {
		t.Fatalf("digest :\n%s", s)
	}
	for _, v := range secrets {
		if strings.Contains(s, v) {
			t.Errorf("valeur %q dans le digest :\n%s", v, s)
		}
	}
	if full := StructureIn(ctx, h, Full, "fr"); !strings.Contains(full, "Zorglub") {
		t.Errorf("partage complet : colonnes du TCD attendues :\n%s", full)
	}
}

func TestDigestBudget(t *testing.T) {
	h := newHost(t)
	ctx := context.Background()
	// 30 tableaux de 30 colonnes : bien au-delà du budget.
	for k := range 30 {
		head := []any{}
		line := []any{}
		for c := range 30 {
			head = append(head, fmt.Sprintf("Colonne%02d", c))
			line = append(line, float64(c+k))
		}
		if _, err := api.Invoke(ctx, h, "cells.paste", map[string]any{"row": 1, "col": 1, "values": []any{head, line, line}, "sheet": mustSheet(t, h, fmt.Sprintf("F%02d", k))}); err != nil {
			t.Fatal(err)
		}
	}
	s := StructureIn(ctx, h, SchemaOnly, "fr")
	if len(s) > DigestBudget+200 {
		t.Fatalf("digest de %d caractères (budget %d)", len(s), DigestBudget)
	}
	if !strings.Contains(s, "tables_list") || !strings.Contains(s, "Ventes · feuille Feuille1 · 30 ligne(s)") {
		t.Errorf("digest borné :\n%s", s)
	}
	// Les petits tableaux passent à une ligne avant de disparaître.
	if !strings.Contains(s, "Colonne00 ent., Colonne01 ent.") {
		t.Errorf("format court attendu :\n%s", s)
	}
}

// Au-delà de ProfileRows lignes, les valeurs distinctes se comptent sur un
// échantillon, et le digest le dit.
func TestDigestSample(t *testing.T) {
	h := newHost(t)
	old, oldS := ProfileRows, ProfileSample
	ProfileRows, ProfileSample = 10, 5
	t.Cleanup(func() { ProfileRows, ProfileSample = old, oldS })
	s := StructureIn(context.Background(), h, SchemaOnly, "fr")
	if !strings.Contains(s, "valeurs distinctes sur les 5 premières lignes") || !strings.Contains(s, "Client txt ≥5") || strings.Contains(s, "(clé possible") {
		t.Errorf("échantillon :\n%s", s)
	}
}

func mustSheet(t *testing.T, h *host, name string) string {
	t.Helper()
	if _, err := api.Invoke(context.Background(), h, "sheets.add", map[string]any{"name": name}); err != nil {
		t.Fatal(err)
	}
	return name
}

// Bilan d'une écriture : « ok », et pour une formule les erreurs par code,
// la première cellule en erreur et la cause.
func TestWriteReport(t *testing.T) {
	h := newHost(t)
	ctx := context.Background()
	exec := execRaw(h, SchemaOnly)
	run := func(in string) (string, bool) {
		t.Helper()
		return exec(ctx, "cells_set", json.RawMessage(in))
	}
	if res, isErr := exec(ctx, "columns_insert", json.RawMessage(`{"table":"Ventes","name":"Test"}`)); isErr || res != "ok" {
		t.Fatalf("columns_insert : %s", res)
	}
	cases := []struct{ in, want string }{
		{`{"row":2,"col":3,"value":"=FOOBAR(1)"}`, "colonne calculée « Test » =FOOBAR(1) : 30 #NOM? sur 30 ligne(s), la première en C2 ; fonction inconnue FOOBAR"},
		{`{"row":2,"col":3,"value":"=[Montnat]*2"}`, "30 #REF! sur 30 ligne(s), la première en C2 ; colonne inconnue [Montnat] dans Ventes (proche : Montant)"},
		{`{"row":2,"col":3,"value":"=SI([Montant]>100;1/0;1)"}`, "19 #DIV/0! sur 30 ligne(s), la première en C13"},
		{`{"row":2,"col":3,"value":"=ARRONDI([Montant]*1,2; 2)"}`, "ok\nVérification : colonne calculée « Test » =ARRONDI([Montant]*1,2; 2) : aucune erreur (résultat : nombre)"},
		{`{"row":40,"col":8,"value":"=FOOBAR(2)"}`, "cellule H40 : #NOM? (fonction inconnue FOOBAR"},
		{`{"row":41,"col":8,"value":"=1+1"}`, "1 cellule(s) avec formule, sans erreur"},
		{`{"row":42,"col":8,"value":"texte"}`, "ok"},
	}
	for _, c := range cases {
		res, isErr := run(c.in)
		if isErr || !strings.Contains(res, c.want) {
			t.Errorf("%s : %v %s", c.in, isErr, res)
		}
	}
	if res, _ := run(`{"row":42,"col":8,"value":"autre"}`); res != "ok" {
		t.Errorf("bilan vide : %s", res)
	}
	// Collage qui crée un tableau : les formules y restent du texte, le bilan le dit.
	res, isErr := exec(ctx, "cells_paste", json.RawMessage(`{"row":1,"col":20,"values":[["A","B"],[1,"=[A]/0"],[2,"=[A]/0"]]}`))
	if isErr || !strings.Contains(res, "la colonne « B » de Tableau1 garde la formule comme du texte") || !strings.Contains(res, "(U2)") {
		t.Errorf("collage : %s", res)
	}
}

// Langue de l'interface : formules écrites et lues comme dans la grille ;
// en anglais, une formule française reste acceptée ; en français, une
// formule anglaise est refusée avec l'écriture attendue.
func TestFormulaLanguage(t *testing.T) {
	h := newHost(t)
	ctx := context.Background()
	if _, isErr := execRaw(h, SchemaOnly)(ctx, "columns_insert", json.RawMessage(`{"table":"Ventes","name":"TTC"}`)); isErr {
		t.Fatal("columns_insert")
	}
	en := execRawIn(h, SchemaOnly, "en")
	fr := execRawIn(h, SchemaOnly, "fr")
	res, isErr := en(ctx, "cells_set", json.RawMessage(`{"row":2,"col":3,"value":"=ROUND([Montant]*1.2, 2)"}`))
	if isErr || !strings.Contains(res, "=ROUND([Montant]*1.2, 2) : aucune erreur") {
		t.Fatalf("anglais : %s", res)
	}
	if res, _ := en(ctx, "tables_get", json.RawMessage(`{"table":"Ventes"}`)); !strings.Contains(res, `"formula":"=ROUND([Montant]*1.2, 2)"`) {
		t.Errorf("lecture en anglais : %s", res)
	}
	if res, _ := fr(ctx, "tables_get", json.RawMessage(`{"table":"Ventes"}`)); !strings.Contains(res, `"formula":"=ARRONDI([Montant]*1,2; 2)"`) {
		t.Errorf("lecture en français : %s", res)
	}
	// Formule française dans une session anglaise : acceptée.
	if res, isErr := en(ctx, "cells_set", json.RawMessage(`{"row":2,"col":3,"value":"=ARRONDI([Montant]*1,5; 1)"}`)); isErr || !strings.Contains(res, "=ROUND([Montant]*1.5, 1)") {
		t.Errorf("français en session anglaise : %s", res)
	}
	// Illisible dans les deux : l'écriture de la session.
	if res, isErr := en(ctx, "cells_set", json.RawMessage(`{"row":2,"col":3,"value":"=ROUND([Montant]*1.2; 2"}`)); !isErr || !strings.Contains(res, "« , »") || !strings.Contains(res, "point (1.2)") {
		t.Errorf("erreur en anglais : %s", res)
	}
	for _, in := range []string{`=SOMME([Montant], 2)`, `=[Montant]*1.2`} {
		b, _ := json.Marshal(map[string]any{"row": 2, "col": 3, "value": in})
		if res, isErr := fr(ctx, "cells_set", b); !isErr || !strings.Contains(res, "« ; »") || !strings.Contains(res, "virgule (1,2)") {
			t.Errorf("%s en français : %s", in, res)
		}
	}
	// Recherche de fonctions dans l'écriture de la session.
	if res, _ := en(ctx, functionsTool, json.RawMessage(`{"query":"sumif"}`)); !strings.HasPrefix(res, `[{"description"`) || !strings.Contains(res, `"name":"SUMIF"`) || !strings.Contains(res, "SUMIF(plage, critère") {
		t.Errorf("fonctions en anglais : %s", res)
	}
	res, _ = fr(ctx, functionsTool, json.RawMessage(`{"query":"somme.si"}`))
	var fns []struct{ Name, Signature string }
	if json.Unmarshal([]byte(res), &fns) != nil || len(fns) == 0 || fns[0].Name != "SOMME.SI" || len(fns) > MaxFunctions {
		t.Errorf("fonctions : %s", res)
	}
	if res, _ := fr(ctx, functionsTool, json.RawMessage(`{}`)); len(res) > 8000 || !strings.Contains(res, "Agrégats : ") || !strings.Contains(res, "SOMME.SI") {
		t.Errorf("liste des fonctions (%d car.) : %.300s", len(res), res)
	}
	// Consigne : langue de la réponse et écriture des formules.
	sysEN, sysFR := SystemFor(SchemaOnly, false, "en"), SystemFor(SchemaOnly, false, "fr")
	for _, want := range []string{"langue de l'interface : anglais", "=SUMIF(Commandes[IdClient], [IdClient], Commandes[Montant HT])", "=ROUND([Montant HT]*1.2, 2)", "point (1.2)"} {
		if !strings.Contains(sysEN, want) {
			t.Errorf("consigne en anglais sans %q", want)
		}
	}
	for _, want := range []string{"langue de l'interface : français", "=SOMME.SI(Commandes[IdClient]; [IdClient]; Commandes[Montant HT])", "virgule (1,2)", "Trois phrases au plus", "Vérifier avant de conclure", "strftime"} {
		if !strings.Contains(sysFR, want) {
			t.Errorf("consigne en français sans %q", want)
		}
	}
	if SystemWith(SchemaOnly, false) != sysFR {
		t.Error("SystemWith : consigne française")
	}
	if c := Compose("Bonjour", View{Today: "2026-10-29"}, nil); !strings.Contains(c, "aujourd'hui 2026-10-29") {
		t.Errorf("date du jour : %s", c)
	}
}

// Erreurs : noms proches, noms d'outils tels que le modèle les voit.
func TestErrorHints(t *testing.T) {
	h := newHost(t)
	ctx := context.Background()
	exec := execRaw(h, SchemaOnly)
	if res, isErr := exec(ctx, showTool, json.RawMessage(`{"sql":"SELECT * FROM \"Vents\""}`)); !isErr || !strings.Contains(res, "vouliez-vous Ventes") {
		t.Errorf("tableau inconnu : %s", res)
	}
	if res, isErr := exec(ctx, "charts_create", json.RawMessage(`{"row":1,"col":10,"config":{"source":"Ventes","kind":"bar","x":"Clientt","series":["Montant"]}}`)); !isErr || !strings.Contains(res, "vouliez-vous Client") {
		t.Errorf("colonne inconnue : %s", res)
	}
	if res, isErr := exec(ctx, "tables_get", json.RawMessage(`{"table":"Zzzz"}`)); !isErr || !strings.Contains(res, "tableaux du classeur : Ventes") {
		t.Errorf("tableau sans nom proche : %s", res)
	}
	// Une recherche dans un tableau sans clé : le message cite les outils
	// avec « _ ».
	if _, err := api.Invoke(ctx, h, "cells.paste", map[string]any{"row": 1, "col": 10, "values": []any{[]any{"Nom", "Ville"}, []any{"C1", "Lyon"}, []any{"C2", "Nice"}}}); err != nil {
		t.Fatal(err)
	}
	res, _ := api.Invoke(ctx, h, "tables.list", nil)
	for _, t2 := range res.([]api.TableEntry) {
		if t2.Name != "Ventes" {
			api.Invoke(ctx, h, "tables.rename", map[string]any{"table": t2.Name, "name": "Clients"})
		}
	}
	exec(ctx, "columns_insert", json.RawMessage(`{"table":"Ventes","name":"Ville"}`))
	out, isErr := exec(ctx, "cells_set", json.RawMessage(`{"row":2,"col":3,"value":"=REFERENCE(Clients[Ville]; [Client])"}`))
	if !isErr || !strings.Contains(out, "tables_set_key") || strings.Contains(out, "tables.set_key") {
		t.Errorf("noms d'outils : %s", out)
	}
	if got := toolNamer.Replace("tables.set_key puis tables.set_sort_lists et db.query"); got != "tables_set_key puis tables_set_sort_lists et db_query" {
		t.Errorf("noms d'outils : %s", got)
	}
}
