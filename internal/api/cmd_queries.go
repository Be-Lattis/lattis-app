package api

import (
	"errors"
	"fmt"

	"github.com/lalamefine/slitable/internal/query"
)

// Requêtes d'import (équivalent de Power Query, voir
// docs/decisions/0003-sources-externes-power-query.md).

const queryHelp = `source : {kind, ...} avec kind "csv" (path, delimiter), "xlsx" (path, sheet, index à partir de 0), "json" (path, recordPath : chemin du tableau d'enregistrements, ex. data.items), "parquet" (path), "sqlite" (path, sql), "table" (table), "query" (query : autre requête), "postgres", "mysql" ou "sqlserver" (connection, sql en lecture seule), "web" (url, headers = {{name, value}}, recordPath, connection : connexion Web facultative qui porte le jeton). steps : liste d'étapes {op, ...} : filter {conditions = {{column, cmp, value}}, any}, rename {column, name}, type {column, type}, remove/select {columns}, split {column, delimiter, parts}, replace {column, value, with}, transform {column, fn = "trim"|"upper"|"lower"}, addcolumn {name, expr (SQLite, [Colonne]), type}, index {name, start}, merge {other = source, keys, otherKeys, join = "left"|"inner"|"full"|"right"|"anti", columns}, append {other}, group {columns, aggs = {{name, op, column}}}, sort {sort = {{column, desc}}}, distinct {columns}, top {count}, unpivot {columns, name, valueName}, pivot {column, valueColumn, agg}. cmp : "=", "<>", "<", "<=", ">", ">=", "contains", "notcontains", "begins", "ends", "empty", "notempty".`

var pQuery = Param{Name: "query", Type: Ref, Desc: "requête (nom ou identifiant)"}

func (c *Call) queryDef() (query.Query, error) {
	d, err := c.Doc()
	if err != nil {
		return query.Query{}, err
	}
	name, _ := c.Str("query")
	def, err := d.QueryByName(c, name)
	if err != nil {
		return query.Query{}, err
	}
	return query.Decode(def)
}

func refreshErr(err error) error {
	var pe *query.ErrPassword
	if errors.As(err, &pe) {
		return fmt.Errorf("%w : saisissez-le une fois depuis l'interface (Données › Requêtes et connexions)", err)
	}
	return err
}

func init() {
	register(
		&Command{
			Name:    "queries.list",
			Summary: "Liste les requêtes d'import du classeur, avec leur source, leurs étapes et le tableau chargé.",
			Returns: "{ {id, name, source, steps, table, sheet, refreshed}, ... }",
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				defs, err := d.Queries(c)
				if err != nil {
					return nil, err
				}
				out := []query.Query{}
				for _, def := range defs {
					q, err := query.Decode(def)
					if err != nil {
						return nil, err
					}
					out = append(out, q)
				}
				return out, nil
			},
		},
		&Command{
			Name:    "queries.save",
			Label:   "Enregistrer la requête",
			Summary: "Crée une requête d'import (ou remplace la source et les étapes de celle qui porte ce nom) et renvoie son identifiant. Elle n'est chargée dans une feuille qu'à l'actualisation.",
			Details: queryHelp,
			Params: []Param{
				{Name: "name", Type: String, Desc: "nom de la requête"},
				{Name: "source", Type: Object, Desc: "source des données"},
				{Name: "steps", Type: Array, Items: Object, Desc: "étapes de transformation", Optional: true},
			},
			Write:   true,
			Example: `queries.save("Grosses ventes", {kind = "table", table = "Ventes"}, {{op = "filter", conditions = {{column = "Montant", cmp = ">", value = "100"}}}})`,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				name, _ := c.Str("name")
				q := query.Query{Name: name, Steps: []query.Step{}}
				if err := c.Decode("source", &q.Source); err != nil {
					return nil, err
				}
				if c.Has("steps") {
					if err := c.Decode("steps", &q.Steps); err != nil {
						return nil, err
					}
				}
				if old, err := d.QueryByName(c, name); err == nil {
					q.ID = old.ID
				}
				def, err := query.Encode(q)
				if err != nil {
					return nil, err
				}
				return d.SaveQuery(c, def)
			},
		},
		&Command{
			Name:    "queries.preview",
			Summary: "Calcule l'aperçu d'une requête (sur les 1 000 premières lignes de chaque source) après ses n premières étapes (toutes par défaut).",
			Params:  []Param{pQuery, {Name: "steps", Type: Integer, Desc: "nombre d'étapes appliquées", Optional: true}},
			Returns: "{columns, rows, count, sample, error, errorStep}",
			Run: func(c *Call) (any, error) {
				q, err := c.queryDef()
				if err != nil {
					return nil, err
				}
				upto, err := c.Int("steps", -1)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				s, err := query.NewSession()
				if err != nil {
					return nil, err
				}
				defer s.Close()
				pv := s.Preview(c, query.Env{Model: d.Model}, q, upto, true)
				pv.SQL = ""
				return pv, nil
			},
		},
		&Command{
			Name:    "queries.refresh",
			Label:   "Actualiser la requête",
			Summary: "Actualise une requête : relit sa source, rejoue ses étapes et remplace son tableau (créé sur une nouvelle feuille la première fois).",
			Params:  []Param{pQuery},
			Write:   true,
			Returns: "{sheet, table, rows, warnings}",
			Run: func(c *Call) (any, error) {
				q, err := c.queryDef()
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				l, err := query.Refresh(c, query.Env{Model: d.Model}, q.ID)
				return l, refreshErr(err)
			},
		},
		&Command{
			Name:    "queries.refresh_all",
			Label:   "Actualiser tout",
			Summary: "Actualise toutes les requêtes déjà chargées dans une feuille.",
			Write:   true,
			Returns: "{ {sheet, table, rows, warnings}, ... }",
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				s, err := query.NewSession()
				if err != nil {
					return nil, err
				}
				defer s.Close()
				ls, err := s.RefreshAll(c, query.Env{Model: d.Model})
				return ls, refreshErr(err)
			},
		},
		&Command{
			Name:    "queries.delete",
			Label:   "Supprimer la requête",
			Summary: "Supprime une requête d'import ; son tableau reste sur la feuille.",
			Params:  []Param{pQuery},
			Write:   true,
			Run: func(c *Call) (any, error) {
				q, err := c.queryDef()
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.DeleteQuery(c, q.ID)
			},
		},
		&Command{
			Name:    "connections.list",
			Summary: "Liste les connexions aux sources externes du classeur (sans mot de passe : il n'est jamais enregistré dans le classeur).",
			Returns: "{ {id, name, kind, config = {host, port, database, user, sslmode, instance, url, header}}, ... } (kind : postgres, mysql, sqlserver ou web)",
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				defs, err := d.Connections(c)
				if err != nil {
					return nil, err
				}
				out := []query.Connection{}
				for _, def := range defs {
					if cn, err := query.DecodeConnection(def); err == nil {
						out = append(out, cn)
					}
				}
				return out, nil
			},
		},
	)
}
