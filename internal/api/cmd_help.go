package api

import (
	"github.com/lalamefine/lattis/internal/helpdoc"
	"github.com/lalamefine/lattis/internal/msg"
)

// Aide de l'application, cherchable : la même que le volet d'aide, pour les
// clients MCP, l'assistant et les scripts.
func init() {
	register(&Command{
		Name:    "help.search",
		Summary: "Cherche dans l'aide de Lattis (les pages du volet d'aide, dans la langue de la session, le français à défaut) : rend les sections qui parlent des mots donnés, ou une page entière, ou la liste des pages.",
		Details: "query : mots à chercher (par exemple « en-têtes sur deux lignes », « sortir l'en-tête », « clé », « graphique filtre ») ; page : identifiant d'une page à lire en entier (voir la liste rendue sans paramètre). Sans query ni page : la liste des pages. Les procédures décrites (menus, clics) ont souvent une commande équivalente : tables.two_level_headers, tables.header_to_title, tables.set_key…",
		Returns: "{ {page, title, text}, ... } ou {id, title, text} ou { {id, title}, ... }",
		Params: []Param{
			{Name: "query", Type: String, Desc: "mots à chercher", Optional: true},
			{Name: "page", Type: String, Desc: "identifiant d'une page (prise-en-main, formules, sql…)", Optional: true},
		},
		Example: `for _, s in ipairs(help.search("en-têtes sur deux lignes")) do
  print(s.page, s.title)
end`,
		Run: func(c *Call) (any, error) {
			page, err := c.Str("page")
			if err != nil {
				return nil, err
			}
			if page != "" {
				p, ok := helpdoc.GetIn(c.lang(), page)
				if !ok {
					return nil, msg.New("api.unknownHelpPage", msg.Args{"page": page})
				}
				return p, nil
			}
			q, err := c.Str("query")
			if err != nil {
				return nil, err
			}
			if q == "" {
				return helpdoc.PagesIn(c.lang()), nil
			}
			return helpdoc.SearchIn(c.lang(), q, 6), nil
		},
	})
}

// lang : la langue de la session pour l'aide, celle du classeur ouvert
// (sinon celle du poste) ; l'aide française complète les pages qui manquent.
func (c *Call) lang() msg.Lang {
	if d, err := c.Doc(); err == nil && d != nil {
		return d.Language()
	}
	return msg.Current()
}
