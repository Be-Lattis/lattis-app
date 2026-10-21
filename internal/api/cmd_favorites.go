package api

import (
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
)

// Requêtes favorites de l'onglet SQL (canvas/favorites.go), enregistrées
// dans le classeur avec un nom : une page web les exécute par leur nom
// (lattis.query("Top villes"), frontend/src/lib/page/bridge.ts), comme
// db.query, en lecture seule. Ni les scripts Lua (db.query leur suffit) ni
// le serveur MCP (partage des données de l'assistant : internal/sharing).

// favoriteByName cherche une requête favorite par son nom (la casse et les
// espaces autour ne comptent pas).
func (c *Call) favoriteByName(name string) (canvas.Favorite, error) {
	d, err := c.Doc()
	if err != nil {
		return canvas.Favorite{}, err
	}
	name = strings.TrimSpace(name)
	fs, err := d.Favorites(c)
	if err != nil {
		return canvas.Favorite{}, err
	}
	for _, f := range fs {
		if f.Name == name {
			return f, nil
		}
	}
	for _, f := range fs {
		if strings.EqualFold(strings.TrimSpace(f.Name), name) {
			return f, nil
		}
	}
	return canvas.Favorite{}, msg.New("api.favoriteNotFound", msg.Args{"name": name})
}

func init() {
	register(
		&Command{
			Name:    "favorites.list",
			Summary: "Liste les requêtes favorites de l'onglet SQL, enregistrées dans le classeur : nom et texte SQL.",
			Returns: "{ {id, name, sql}, ... }",
			NoLua:   true,
			NoMCP:   true,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				return d.Favorites(c)
			},
		},
		&Command{
			Name:    "favorites.run",
			Summary: "Exécute une requête favorite de l'onglet SQL, par son nom, en lecture seule (comme db.query) ; renvoie au plus 10 000 lignes.",
			Details: "Les ? de la requête prennent les valeurs de params. Une requête favorite qui modifie le classeur (INSERT, UPDATE, DELETE) est refusée.",
			Returns: "{columns, rows, truncated}",
			Params: []Param{
				{Name: "name", Type: String, Desc: "nom de la requête favorite"},
				{Name: "params", Type: Array, Desc: "valeurs des ?", Optional: true},
			},
			NoLua: true,
			NoMCP: true,
			Run: func(c *Call) (any, error) {
				name, _ := c.Str("name")
				f, err := c.favoriteByName(name)
				if err != nil {
					return nil, err
				}
				var params []any
				if err := c.Decode("params", &params); err != nil {
					return nil, err
				}
				if err := ReadOnlySQL(f.SQL); err != nil {
					return nil, err
				}
				return c.readSQL(f.SQL, params)
			},
		},
	)
}
