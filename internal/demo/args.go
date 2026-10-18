package demo

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrHelp : l'aide est demandée (-h, --help) ; elle s'affiche sans ouvrir
// la fenêtre.
var ErrHelp = errors.New("aide demandée")

// ParseArgs lit les options de la ligne de commande :
//
//	--demo [preset]   ouvre un classeur de fausses données (med par défaut),
//	                  ou celui décrit par un fichier .yaml (chemin absolu rendu)
//	--seed n          graine du générateur (1 par défaut)
//	-h, --help        aide (ErrHelp)
//
// Une option ou un argument inconnu est une erreur (sauf -psn_…, ajouté par
// macOS au lancement depuis le Finder).
func ParseArgs(args []string) (preset string, seed uint64, err error) {
	for i := 0; i < len(args); i++ {
		name, value, inline := strings.Cut(args[i], "=")
		next := func() (string, bool) {
			if inline {
				return value, true
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				return args[i], true
			}
			return "", false
		}
		switch name {
		case "--demo", "-demo":
			v, ok := next()
			if !ok {
				v = DefaultPreset
			}
			p, err := Resolve(v)
			if err != nil {
				return "", 0, err
			}
			// Un fichier de plan est lu tout de suite : ses erreurs s'affichent
			// avant que la fenêtre s'ouvre.
			if _, err := p.Load(); err != nil {
				return "", 0, err
			}
			preset = p.Name
		case "--seed", "-seed":
			v, ok := next()
			if seed, err = strconv.ParseUint(v, 10, 64); !ok || err != nil {
				return "", 0, fmt.Errorf("--seed attend un entier positif, pas « %s »", v)
			}
		case "-h", "--help", "-help", "/?":
			return "", 0, ErrHelp
		default:
			switch {
			case strings.HasPrefix(name, "-psn_"):
			case strings.HasPrefix(name, "-"):
				return "", 0, fmt.Errorf("option inconnue : %s", args[i])
			default:
				return "", 0, fmt.Errorf("argument inattendu : %s", args[i])
			}
		}
	}
	return preset, seed, nil
}

// Usage décrit la ligne de commande : options, sous-commande et presets.
func Usage() string {
	var b strings.Builder
	b.WriteString(`Lattis : tableur relationnel local-first.

Usage :
  lattis [classeur.lattis]     ouvre l'application, et ce classeur s'il est donné
  lattis --demo [preset]       ouvre un classeur de démonstration (fausses données)
  lattis --demo plan.yaml      ouvre le classeur de démonstration décrit par ce fichier
  lattis mcp                   relais stdio vers le serveur MCP de l'application ouverte
                                 (pour les clients MCP qui lancent un programme)
  lattis -h | --help           affiche cette aide

Options :
  --demo [preset]   classeur de démonstration ; preset par défaut : ` + DefaultPreset + `
  --demo plan.yaml  classeur de démonstration décrit en YAML (voir plus bas)
  --seed n          graine du générateur de la démonstration (1 par défaut ; l'emporte
                    sur le seed d'un fichier YAML)
  --sql, --no-sql   écrit (ou non) dans la console chaque requête SQL jouée contre le
                    SQLite local, avec sa durée ; actif par défaut avec go run / wails dev
  -h, --help        affiche cette aide, sans ouvrir la fenêtre

Presets de --demo :
`)
	for _, p := range Presets() {
		fmt.Fprintf(&b, "  %-9s %s\n", p.Name, p.Description)
	}
	b.WriteString(`
Fichier YAML de --demo (format complet : docs/demo.md ; exemple : docs/demo/exemple.yaml) :
  seed: 7                            # graine (facultative)
  sheets:                            # onglets, dans l'ordre
    - name: Ventes
      tables:
        - name: Commandes
          rows: 500                  # lignes générées
          key: N° commande           # colonne clé (REFERENCE cherche par elle)
          columns:
            - {name: N° commande, gen: sequence, start: 1001}
            - {name: Date, gen: date, from: 2025-01-01, to: 2025-12-31, ordered: true}
            - {name: Ville, gen: city, skew: true}
            - {name: Région, gen: region, of: Ville}
            - {name: Quantité, gen: int, min: 1, max: 9}
            - {name: Prix, gen: decimal, min: 5, max: 400}
            - {name: Montant, formula: "=[Quantité]*[Prix]"}
      cells: [{cell: J1, value: Total}, {cell: K1, value: "=SOMME(Commandes[Montant])"}]
      charts: [{cell: J3, source: Commandes, kind: bar, x: Région, series: [Montant]}]
  Générateurs (gen) : sequence, code, int, decimal, date, bool, pick et cycle (values), constant,
    firstname, lastname, person, company, email, phone, city, postcode, region, latitude,
    longitude, population (of : colonne ville), product, category, price (of : colonne produit).
  Un onglet peut aussi porter pivots, controls, rules, styles, widths, ou être une requête
  (query : sql avec {{Tableau}} et {{Tableau[Colonne]}}, ou source et steps), un document
  (doc : h1, p, bullet, chart, pivot, button, control) ou une page HTML (html) ; scripts et
  events (écouteurs d'évènements) se déclarent à la racine.

Variables d'environnement :
  LATTIS_BENCH=fichier    mesure le défilement de la grille et écrit le rapport dans fichier
  LATTIS_SQL=1 ou 0       force le journal SQL (comme --sql / --no-sql)
`)
	return b.String()
}
