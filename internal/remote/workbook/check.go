package workbook

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/store"
)

// Vérifications avant publication (§5.3, point 2) : noms valides en
// PostgreSQL, types figés, saisies invalides à corriger ou à accepter comme
// texte.

// Check est le résultat des vérifications, tableau par tableau.
type Check struct {
	Tables []TableCheck `json:"tables"`
	// lang : langue des problèmes décrits (InspectIn).
	lang msg.Lang
}

// TableCheck décrit un tableau à publier.
type TableCheck struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Sheet string `json:"sheet"`
	Rows  int    `json:"rows"`
	// Published : déjà publié (republication : coché d'avance).
	Published bool          `json:"published"`
	Columns   []ColumnCheck `json:"columns"`
	// Problems : ce qui empêche de publier ce tableau (noms).
	Problems []string `json:"problems"`
	// Invalid : saisies invalides du tableau (à corriger, ou colonne à
	// accepter comme texte).
	Invalid int `json:"invalid"`
	// Query : nom de la requête qui charge le tableau ("" : tableau saisi).
	// Publié dans un classeur collaboratif, il s'actualise depuis le poste
	// qui a la connexion (lignes remplacées pour tous), ou devient une
	// source du Data Lake : choix de l'auteur à la publication.
	Query string `json:"query,omitempty"`
}

// ColumnCheck décrit une colonne d'un tableau à publier.
type ColumnCheck struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	// Remote : type sur le serveur (vide : colonne calculée, non publiée).
	Remote string `json:"remote"`
	// Inferred : type encore « élargissable », figé par la publication.
	Inferred bool `json:"inferred"`
	Computed bool `json:"computed"`
	// Invalid : nombre de saisies invalides ; Samples : quelques-unes.
	Invalid int      `json:"invalid"`
	Samples []string `json:"samples"`
}

// Blocking dit ce qui empêche de publier ces tableaux (vide : rien).
func (c Check) Blocking(ids []int64) []string {
	want := map[int64]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []string
	seen := map[string]string{}
	for _, t := range c.Tables {
		if !want[t.ID] {
			continue
		}
		for _, p := range t.Problems {
			out = append(out, msg.Format(c.lang, "check.tableProblem", msg.Args{"table": t.Name, "problem": p}))
		}
		if t.Invalid > 0 {
			out = append(out, msg.Format(c.lang, "check.invalidEntries", msg.Args{"table": t.Name, "n": t.Invalid}))
		}
		k := strings.ToLower(strings.TrimSpace(t.Name))
		if other, ok := seen[k]; ok {
			out = append(out, msg.Format(c.lang, "check.sameName", msg.Args{"table": other, "other": t.Name}))
		}
		seen[k] = t.Name
		delete(want, t.ID)
	}
	for id := range want {
		out = append(out, msg.Format(c.lang, "check.tableNotFound", msg.Args{"id": id}))
	}
	return out
}

// NameProblem dit pourquoi un nom de tableau ou de colonne n'est pas
// valable sur le serveur (lattis.check_name : 1 à 63 octets, sans espaces
// autour, préfixe lattis_ réservé), vide s'il l'est ; en français.
func NameProblem(name string) string { return nameProblem(msg.FR, name) }

// nameProblem : NameProblem dans la langue l.
func nameProblem(l msg.Lang, name string) string {
	var id msg.ID
	switch {
	case strings.TrimSpace(name) == "":
		id = "check.nameEmpty"
	case strings.TrimSpace(name) != name:
		id = "check.nameSpaces"
	case len(name) > 63:
		id = "check.nameTooLong"
	case strings.HasPrefix(strings.ToLower(name), "lattis_"):
		id = "check.namePrefix"
	case strings.ContainsRune(name, 0):
		id = "check.nameNul"
	default:
		return ""
	}
	return msg.Format(l, id, msg.Args{"n": len(name)})
}

// Inspect fait les vérifications sur tous les tableaux du classeur ; les
// problèmes sont décrits en français (InspectIn : dans une langue).
func Inspect(ctx context.Context, db *store.Store) (Check, error) {
	return InspectIn(ctx, db, msg.FR)
}

// InspectIn : Inspect, problèmes décrits dans la langue l.
func InspectIn(ctx context.Context, db *store.Store, l msg.Lang) (Check, error) {
	r := db.Read()
	out := Check{Tables: []TableCheck{}, lang: l}
	rs, err := r.QueryContext(ctx, `SELECT t.id, t.name, s.name, coalesce((SELECT published FROM _lattis_remote_tables p WHERE p.tbl = t.id), 0),
		coalesce((SELECT q.name FROM _lattis_queries q WHERE q.target = t.id), '')
		FROM _lattis_tables t JOIN _lattis_sheets s ON s.id = t.sheet ORDER BY s.pos, s.id, t.row, t.col`)
	if err != nil {
		return out, err
	}
	for rs.Next() {
		var t TableCheck
		if err := rs.Scan(&t.ID, &t.Name, &t.Sheet, &t.Published, &t.Query); err != nil {
			rs.Close()
			return out, err
		}
		out.Tables = append(out.Tables, t)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return out, err
	}
	for i := range out.Tables {
		if err := inspectTable(ctx, r, l, &out.Tables[i]); err != nil {
			return out, err
		}
	}
	return out, nil
}

func inspectTable(ctx context.Context, r *sql.DB, l msg.Lang, t *TableCheck) error {
	t.Problems = []string{}
	if p := nameProblem(l, t.Name); p != "" {
		t.Problems = append(t.Problems, p)
	}
	if err := r.QueryRowContext(ctx, `SELECT count(*) FROM t_`+fmt.Sprint(t.ID)).Scan(&t.Rows); err != nil {
		return err
	}
	rs, err := r.QueryContext(ctx, `SELECT id, name, type, fixed, coalesce(formula, '') FROM _lattis_columns WHERE tbl = ? ORDER BY pos`, t.ID)
	if err != nil {
		return err
	}
	var ids []int64
	names := map[string]string{}
	stored := 0
	for rs.Next() {
		var c ColumnCheck
		var id int64
		var fixed bool
		var formula string
		if err := rs.Scan(&id, &c.Name, &c.Type, &fixed, &formula); err != nil {
			rs.Close()
			return err
		}
		c.Index = len(t.Columns)
		c.Computed = formula != ""
		c.Samples = []string{}
		if !c.Computed {
			c.Remote = RemoteType(grid.Type(c.Type))
			c.Inferred = !fixed
			stored++
			if p := nameProblem(l, c.Name); p != "" {
				t.Problems = append(t.Problems, msg.Format(l, "check.columnProblem", msg.Args{"column": c.Name, "problem": p}))
			}
			k := strings.ToLower(c.Name)
			if other, ok := names[k]; ok {
				t.Problems = append(t.Problems, msg.Format(l, "check.sameColumnName", msg.Args{"column": other, "other": c.Name}))
			}
			names[k] = c.Name
		}
		ids = append(ids, id)
		t.Columns = append(t.Columns, c)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	if stored == 0 {
		t.Problems = append(t.Problems, msg.Format(l, "check.noStoredColumn", nil))
	}
	for i, id := range ids {
		c := &t.Columns[i]
		if c.Computed {
			continue
		}
		// Dates avec heure (« 2026-10-23 14:30 ») : publiées en date et
		// heure (values.go).
		if c.Type == string(grid.Date) {
			var err error
			if c.Remote, err = DateRemote(ctx, r, t.ID, id); err != nil {
				return err
			}
		}
		if err := r.QueryRowContext(ctx, `SELECT count(*) FROM _lattis_invalid WHERE tbl = ? AND col = ?`, t.ID, id).Scan(&c.Invalid); err != nil {
			return err
		}
		if c.Invalid == 0 {
			continue
		}
		t.Invalid += c.Invalid
		srs, err := r.QueryContext(ctx, `SELECT DISTINCT raw FROM _lattis_invalid WHERE tbl = ? AND col = ? ORDER BY raw LIMIT 5`, t.ID, id)
		if err != nil {
			return err
		}
		for srs.Next() {
			var s string
			if err := srs.Scan(&s); err != nil {
				srs.Close()
				return err
			}
			c.Samples = append(c.Samples, s)
		}
		srs.Close()
	}
	return nil
}
