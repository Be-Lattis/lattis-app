package demo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/trust"
)

// La vitrine med : onglets dans l'ordre, document et page, requêtes
// chargées, scripts autorisés sur ce poste, écouteur, contrôles, règles.
func TestMedVitrine(t *testing.T) {
	ctx := context.Background()
	d, _ := build(t, "med", 0)
	l, err := d.Layout(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range l.Sheets {
		names = append(names, s.Name+":"+s.Kind)
	}
	want := "À découvrir:doc Ventes: Villes: Synthèse: Ventes mensuelles: Impayés: Tableau de bord:html Clients: Employés: Analyse:"
	if got := strings.Join(names, " "); got != want {
		t.Fatalf("onglets :\n got %s\nwant %s", got, want)
	}
	src, err := d.Page(ctx, l.Sheets[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`"lattisChart"`, `"lattisPivot"`, `"lattisButton"`, `"lattisControl":{"id":`, `"header":1`, `"list":"bullet"`} {
		if !strings.Contains(src, s) {
			t.Errorf("document sans %s", s)
		}
	}
	qs, err := d.Queries(ctx)
	if err != nil || len(qs) != 2 {
		t.Fatalf("requêtes %v %v", qs, err)
	}
	for _, q := range qs {
		if q.Table == "" || q.Refreshed == "" {
			t.Errorf("requête %s non chargée", q.Name)
		}
	}
	scripts, _ := d.Scripts(ctx)
	if len(scripts) != 1 || !trust.Default().ScriptAllowed(trust.Hash(scripts[0].Source)) {
		t.Errorf("script de la démonstration non autorisé : %v", scripts)
	}
	if ls, _ := d.WorkbookListeners(ctx); len(ls) != 1 || ls[0].Event != "change" {
		t.Errorf("écouteurs %v", ls)
	}
	if ok, _ := d.SessionHost(MapTileHost); !ok {
		t.Error("fonds de carte non autorisés pour la session")
	}
	ctl, _ := d.Controls(ctx)
	if len(ctl) != 4 {
		t.Errorf("%d contrôles", len(ctl))
	}
	// Indicateurs de la feuille Ventes, pilotés par la liste.
	sl, _ := d.Layout(ctx, l.Sheets[1].ID)
	vals := map[string]string{}
	for _, c := range sl.Cells {
		vals[c.Raw] = c.Value
	}
	if v := vals["=NB.SI(Commandes[Région]; Cell[R1])"]; v == "" || v == "0" {
		t.Errorf("commandes de la région : %q (%v)", v, vals)
	}
	// Une seule étape d'annulation : elle rend le classeur neuf.
	if name, err := d.Bus.Undo(ctx); err != nil || name != "Données de démonstration" {
		t.Fatalf("annulation : %q %v", name, err)
	}
	if l, _ := d.Layout(ctx, 0); len(l.Sheets) != 1 {
		t.Errorf("%d onglets après annulation", len(l.Sheets))
	}
}

// Le fichier d'exemple de la documentation se lit et se fabrique sans
// erreur, et la graine de --seed l'emporte sur la sienne.
func TestExemple(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "demo", "exemple.yaml")
	d, res := build(t, path, 0)
	check(t, d)
	got := strings.Join(shape(t, d), " ")
	for _, w := range []string{"Ateliers/Ateliers:6x8", "Ateliers/graphique", "Réparations/Réparations:400x11", "Réparations/TCD", "Par mois/ParMois:12x4", "Par mois/graphique", "Par type/ParType:6x3"} {
		if !strings.Contains(got, w) {
			t.Errorf("exemple : %s absent de %s", w, got)
		}
	}
	if len(res.Sheets) != 6 {
		t.Errorf("%d onglets", len(res.Sheets))
	}
	sl, _ := d.Layout(context.Background(), res.Sheets[2])
	found := false
	for _, c := range sl.Cells {
		if strings.HasPrefix(c.Raw, "=NB.SI") {
			found = c.Value != "" && c.Value != "0"
		}
	}
	if !found {
		t.Errorf("cellules libres de Réparations : %+v", sl.Cells)
	}
	plan, err := LoadFile(path)
	if err != nil || plan.Seed != 7 {
		t.Fatalf("graine %d, %v", plan.Seed, err)
	}
	dump := func(seed uint64) string {
		d, _ := build(t, path, seed)
		var b strings.Builder
		d.ExportTable(context.Background(), "Réparations", func([]canvas.ExportColumn) error { return nil }, func(row []any) error {
			fmt.Fprintln(&b, row...)
			return nil
		})
		return b.String()
	}
	if dump(0) != dump(7) || dump(0) == dump(8) {
		t.Error("graine : celle du fichier par défaut, --seed l'emporte")
	}
}

// L'exemple de l'aide (lattis --help) est un plan valide.
func TestUsageExemple(t *testing.T) {
	u := Usage()
	from := strings.Index(u, "  seed: 7")
	to := strings.Index(u, "  Générateurs")
	if from < 0 || to < from {
		t.Fatal("exemple YAML absent de l'aide")
	}
	var lines []string
	for _, l := range strings.Split(u[from:to], "\n") {
		lines = append(lines, strings.TrimPrefix(l, "  "))
	}
	p, err := ParsePlan("aide", []byte(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	d, err := doc.New(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := BuildPlan(context.Background(), d.Model, p, Options{}); err != nil {
		t.Fatal(err)
	}
	check(t, d)
}

// Les erreurs d'un plan YAML nomment le fichier et la ligne.
func TestYAMLErreurs(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"", "plan.yaml : fichier vide"},
		{"sheets: [", "plan.yaml:1 : YAML illisible"},
		{"feuilles: []\n", "plan.yaml:1 : clé « feuilles » inconnue"},
		{"sheets: []\n", "aucun onglet"},
		{"sheets:\n  - name: A\n    tables:\n      - name: T\n        rows: 3\n        columns:\n          - {name: X, gen: hasard}\n", "plan.yaml:7 : générateur « hasard » inconnu"},
		{"sheets:\n  - name: A\n    tables:\n      - name: T\n        rows: dix\n", "plan.yaml:5 : valeur « dix » invalide ici (nombre attendu)"},
		{"sheets:\n  - name: A\n    tables:\n      - name: T\n        rows: 3\n        key: Y\n        columns:\n          - {name: X, gen: int}\n", "plan.yaml:6 : clé : colonne « Y » introuvable"},
		{"sheets:\n  - name: A\n    tables:\n      - name: T\n        rows: 3\n        columns:\n          - {name: X, gen: region, of: Ville}\n", "plan.yaml:7 : colonne « Ville » introuvable"},
		{"sheets:\n  - name: A\n    charts:\n      - {cell: B2, source: T, kind: bar, couleur: rouge}\n", "plan.yaml:4 : clé « couleur » inconnue"},
		{"sheets:\n  - name: A\n    charts:\n      - {cell: 2B, source: T}\n", "plan.yaml:4 : adresse « 2B » invalide"},
		{"sheets:\n  - name: A\n    doc:\n      - titre: Bonjour\n", "plan.yaml:4 : bloc « titre » inconnu"},
		{"sheets:\n  - name: A\n  - name: a\n", "plan.yaml:3 : onglet « a » en double"},
		{"sheets:\n  - name: A\n    html: <p>x</p>\n    query: {sql: SELECT 1}\n", "un seul contenu"},
	} {
		_, err := ParsePlan("plan.yaml", []byte(c.src))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q :\n got %v\nwant %s", c.src, err, c.want)
		}
	}
	// Une erreur à la fabrication (tableau introuvable) arrête --demo.
	dir := t.TempDir()
	p := filepath.Join(dir, "plan.yml")
	os.WriteFile(p, []byte("sheets:\n  - name: A\n    query: {sql: 'SELECT * FROM {{Absent}}'}\n"), 0o644)
	if _, _, err := New(context.Background(), p, Options{}); err == nil || !strings.Contains(err.Error(), "{{Absent}} : tableau introuvable") {
		t.Errorf("tableau absent : %v", err)
	}
	if _, err := Resolve(filepath.Join(dir, "absent.yaml")); err == nil || !strings.Contains(err.Error(), "introuvable") {
		t.Errorf("fichier absent : %v", err)
	}
}
