package canvas

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/script"
)

// Référence SQL de l'onglet SQL (frontend/src/lib/sql/ref/entries.json) :
// chaque exemple tourne sur le classeur d'exemple qu'elle suppose
// (Commandes : Client, Montant, Payé, Échéance ; Clients : Client, Ville ;
// TCD1, tableau croisé de Commandes ; Cell[H1] = 100), par le chemin des
// requêtes de l'utilisateur : adresses de cellules, tableaux croisés et
// colonnes calculées, noms affichés, autorisateur. Une écriture passe par
// ExecSQL et Ctrl+Z rend le classeur tel qu'il était.

type sqlRefEntry struct {
	ID   string `json:"id"`
	SQL  string `json:"sql"`
	Kind string `json:"kind"` // "" lecture, "write", "rename"
}

func sqlRefWorkbook(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"Client", "Montant", "Payé", "Échéance"},
		{"Durand", "120", "vrai", "15/01/2026"},
		{"Martin", "80,5", "faux", "02/02/2026"},
		{"Durand", "40", "faux", "20/02/2026"},
		{"Petit", "0", "vrai", "03/03/2026"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Paste(ctx, 1, 7, 0, [][]string{
		{"Client", "Ville"},
		{"Durand", "Paris"},
		{"Martin", "Lyon"},
		{"Grand", "Nice"},
	}); err != nil {
		t.Fatal(err)
	}
	mustSet(t, d, 1, 0, 7, "100")
	l, err := d.Layout(ctx, 1)
	must(t, err)
	if len(l.Tables) != 2 {
		t.Fatalf("%d tableaux", len(l.Tables))
	}
	must(t, d.RenameTable(ctx, l.Tables[0].Name, "Commandes"))
	must(t, d.RenameTable(ctx, l.Tables[1].Name, "Clients"))
	_, err = d.CreatePivot(ctx, PivotConfig{Source: "Commandes", RowFields: []string{"Client"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}})
	must(t, err)
	return d
}

// statementSplit sépare les instructions comme l'éditeur : « ; » ou ligne vide.
var statementSplit = regexp.MustCompile(`;\s*|\n\s*\n`)

func TestSQLReferenceExamples(t *testing.T) {
	raw, err := os.ReadFile("../../frontend/src/lib/sql/ref/entries.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []sqlRefEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) < 40 {
		t.Fatalf("%d entrées", len(entries))
	}
	ctx := context.Background()
	d := sqlRefWorkbook(t)
	read := d.db.Read()
	for _, e := range entries {
		t.Run(e.ID, func(t *testing.T) {
			switch e.Kind {
			case "rename":
				// Traduit par l'onglet SQL en renommage du classeur (rename.ts).
				m := regexp.MustCompile(`(?i)^ALTER TABLE (\S+) RENAME TO (\S+)$`).FindStringSubmatch(e.SQL)
				if m == nil {
					t.Fatalf("renommage illisible : %s", e.SQL)
				}
				must(t, d.RenameTable(ctx, m[1], m[2]))
				if _, err := d.bus.Undo(ctx); err != nil {
					t.Fatal(err)
				}
			case "write":
				before := dumpData(t, d)
				q, err := d.CellSQL(ctx, e.SQL, 1)
				must(t, err)
				n, err := d.ExecSQL(ctx, q)
				if err != nil {
					t.Fatalf("%v\n%s", err, e.SQL)
				}
				if n == 0 {
					t.Errorf("aucune ligne touchée : %s", e.SQL)
				}
				if _, err := d.bus.Undo(ctx); err != nil {
					t.Fatal(err)
				}
				if got := dumpData(t, d); got != before {
					t.Fatalf("Ctrl+Z ne rend pas le classeur :\n%s", diff(before, got))
				}
			default:
				for _, q := range statementSplit.Split(e.SQL, -1) {
					if strings.TrimSpace(q) == "" || strings.HasPrefix(strings.TrimSpace(q), "--") && !strings.Contains(q, "\n") {
						continue
					}
					q, err := d.CellSQL(ctx, q, 1)
					must(t, err)
					if q, err = d.ReadCTE(ctx, q); err != nil {
						t.Fatal(err)
					}
					if q, err = script.Rewrite(ctx, read, q); err != nil {
						t.Fatal(err)
					}
					res, err := grid.Query(ctx, read, q, 1000)
					if err != nil {
						t.Fatalf("%v\n%s", err, e.SQL)
					}
					if len(res.Rows) == 0 {
						t.Errorf("aucune ligne : %s", e.SQL)
					}
				}
			}
		})
	}
}
