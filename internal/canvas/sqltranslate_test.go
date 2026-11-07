package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// Traductions de l'onglet SQL (frontend/src/lib/sql/translate.cases.json,
// vérifiées côté TypeScript par translate.cases.test.ts) exécutées sur un
// vrai classeur aux mêmes noms internes : une requête traduite doit tourner
// dans SQLite, sauf les erreurs propres à la requête de l'utilisateur
// (sqlite : extrait attendu du message traduit en français).

type translateCases struct {
	Objects []struct {
		Name    string `json:"name"`
		SQLName string `json:"sqlName"`
		Columns []struct {
			Name string `json:"name"`
			SQL  string `json:"sql"`
		} `json:"columns"`
	} `json:"objects"`
	Cases []struct {
		Name   string `json:"name"`
		SQL    string `json:"sql"`
		Out    string `json:"out"`
		Before string `json:"before"`
		SQLite string `json:"sqlite"`
	} `json:"cases"`
}

// translateWorkbook : Suividesnids et BDD_nids (la requête signalée),
// Clients et « Ventes2026 » ; chaque tableau numérote ses colonnes à partir
// de 1 (c_1 est dans les quatre).
func translateWorkbook(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	blocks := []struct {
		name string
		row  int
		data [][]string
	}{
		{"Suividesnids", 0, [][]string{
			{"Année", "NID suivi", "N° DE NID", "N° DE NID.N° DE NID", "Observateur", "Date visite"},
			{"2024", "oui", "N1", "N1", "Paul", "01/03/2024"},
			{"2024", "non", "N2", "N2", "Anne", "15/04/2024"},
			{"2025", "oui", "N1", "N1", "Paul", "02/05/2025"},
		}},
		{"BDD_nids", 6, [][]string{
			{"N° DE NID", "Coordonnées.Latitude", "Coordonnées.Longitude", "Commune", "Statut"},
			{"N1", "45,75", "4,85", "Lyon", "actif"},
			{"N2", "48,85", "2,35", "Paris", "détruit"},
		}},
		{"Clients", 11, [][]string{
			{"Code", "Nom client", "Ville"},
			{"1", "Durand", "Paris"},
			{"2", "Martin", "Lyon"},
			{"3", "Petit", "Nice"},
		}},
		{"Ventes2026", 17, [][]string{
			{"Code", "Montant", "Date", "Vendeur"},
			{"1", "120", "15/01/2026", "Paul"},
			{"2", "80,5", "02/02/2026", "Anne"},
			{"1", "40", "20/02/2026", "Paul"},
		}},
	}
	for _, b := range blocks {
		if _, err := d.Paste(ctx, 1, b.row, 0, b.data); err != nil {
			t.Fatal(err)
		}
	}
	l, err := d.Layout(ctx, 1)
	must(t, err)
	if len(l.Tables) != len(blocks) {
		t.Fatalf("%d tableaux", len(l.Tables))
	}
	for i, b := range blocks {
		must(t, d.RenameTable(ctx, l.Tables[i].Name, b.name))
	}
	return d
}

func loadTranslateCases(t *testing.T) translateCases {
	t.Helper()
	raw, err := os.ReadFile("../../frontend/src/lib/sql/translate.cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var tc translateCases
	if err := json.Unmarshal(raw, &tc); err != nil {
		t.Fatal(err)
	}
	if len(tc.Cases) < 50 {
		t.Fatalf("%d cas", len(tc.Cases))
	}
	return tc
}

func TestSQLTranslateCases(t *testing.T) {
	ctx := context.Background()
	d := translateWorkbook(t)
	tc := loadTranslateCases(t)

	// Les noms internes du classeur sont ceux du fichier de cas.
	all, err := d.AllTables(ctx)
	must(t, err)
	byName := map[string]TableNames{}
	for _, tn := range all {
		byName[tn.Name] = tn
	}
	for _, o := range tc.Objects {
		tn, ok := byName[o.Name]
		if !ok || tn.SQL != o.SQLName || len(tn.Columns) != len(o.Columns) {
			t.Fatalf("tableau %s : %+v", o.Name, tn)
		}
		for i, c := range o.Columns {
			if tn.Columns[i] != c.Name || tn.ColumnSQL[i] != c.SQL {
				t.Fatalf("%s colonne %d : %s/%s, attendu %s/%s", o.Name, i, tn.Columns[i], tn.ColumnSQL[i], c.Name, c.SQL)
			}
		}
	}

	read := d.db.Read()
	for _, c := range tc.Cases {
		if c.Out == "" {
			continue // refusée avant SQLite
		}
		t.Run(c.Name, func(t *testing.T) {
			q, err := d.CellSQL(ctx, c.Out, 1)
			must(t, err)
			if q, err = d.ReadCTE(ctx, q); err != nil {
				t.Fatal(err)
			}
			_, err = grid.Query(ctx, read, q, 1000)
			if c.SQLite == "" {
				if err != nil {
					t.Fatalf("%v\n%s", err, c.Out)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.SQLite) {
				t.Fatalf("erreur %v, attendu « %s »\n%s", err, c.SQLite, c.Out)
			}
		})
	}
}

// La requête signalée tourne : une ligne (COUNT sans GROUP BY) ; la condition
// ON "N° DE NID.N° DE NID" (un texte, faux pour SQLite) ne rattache aucun nid.
// Écrite avec la jointure voulue, elle donne les coordonnées.
func TestSQLTranslateReported(t *testing.T) {
	ctx := context.Background()
	d := translateWorkbook(t)
	tc := loadTranslateCases(t)
	out := map[string]string{}
	for _, c := range tc.Cases {
		out[c.Name] = c.Out
	}
	res, err := grid.Query(ctx, d.db.Read(), out["requête signalée"], 1000)
	must(t, err)
	if len(res.Rows) != 1 || fmt.Sprint(res.Rows[0][2:]) != "[3 <nil> <nil>]" {
		t.Fatalf("%v %v", res.Columns, res.Rows)
	}
	if got := strings.Join(res.Columns, "|"); got != `Année|NID suivi|COUNT("NID suivi")|Coordonnées.Latitude|Coordonnées.Longitude` {
		t.Fatalf("en-têtes %s", got)
	}
	res, err = grid.Query(ctx, d.db.Read(), out["requête signalée, jointure explicite"]+" ORDER BY 1, 2", 1000)
	must(t, err)
	if got := fmt.Sprint(res.Rows); got != "[[2024 non 1 48.85 2.35] [2024 oui 1 45.75 4.85] [2025 oui 1 45.75 4.85]]" {
		t.Fatalf("%s", got)
	}
}

// Une écriture refusée par SQLite (clé en double) cite les noms affichés, pas t_3.c_1.
func TestExecSQLErrorShownNames(t *testing.T) {
	ctx := context.Background()
	d := translateWorkbook(t)
	must(t, d.SetKey(ctx, "Clients", 0))
	_, err := d.ExecSQL(ctx, `UPDATE Clients SET Code = 1 WHERE Code = 2`)
	if err == nil {
		t.Fatal("clé en double acceptée")
	}
	if strings.Contains(err.Error(), "t_3") || strings.Contains(err.Error(), "c_1") {
		t.Fatalf("nom interne dans %q", err)
	}
	t.Log(err)
}

// Cas corrigés (before) : l'ancienne traduction échouait dans SQLite (nom
// interne c_N commun à deux tableaux : « ambiguous column name ») ou rendait
// un autre résultat (colonne rattachée au mauvais tableau).
func TestSQLTranslateBeforeFix(t *testing.T) {
	ctx := context.Background()
	d := translateWorkbook(t)
	tc := loadTranslateCases(t)
	read := d.db.Read()
	n := 0
	for _, c := range tc.Cases {
		if c.Before == "" {
			continue
		}
		n++
		t.Run(c.Name, func(t *testing.T) {
			old, err := grid.Query(ctx, read, c.Before, 1000)
			if err != nil {
				if !strings.Contains(err.Error(), "ambiguous column name: c_") && !strings.Contains(err.Error(), "« c_") {
					t.Fatalf("ancienne erreur inattendue : %v", err)
				}
				return
			}
			res, err := grid.Query(ctx, read, c.Out, 1000)
			must(t, err)
			if fmt.Sprint(old.Rows) == fmt.Sprint(res.Rows) {
				t.Fatalf("l'ancienne traduction donnait déjà le même résultat\n%s", c.Before)
			}
		})
	}
	if n == 0 {
		t.Fatal("aucun cas corrigé")
	}
}
