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
//	--demo [preset]   ouvre un classeur de fausses données (med par défaut)
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
			p, found := Lookup(v)
			if !found {
				return "", 0, UnknownPresetError{Name: v}
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
  lattis                       ouvre l'application
  lattis --demo [preset]       ouvre un classeur de démonstration (fausses données)
  lattis mcp                   relais stdio vers le serveur MCP de l'application ouverte
                                 (pour les clients MCP qui lancent un programme)
  lattis -h | --help           affiche cette aide

Options :
  --demo [preset]   classeur de démonstration ; preset par défaut : ` + DefaultPreset + `
  --seed n          graine du générateur de la démonstration (1 par défaut)
  --sql, --no-sql   écrit (ou non) dans la console chaque requête SQL jouée contre le
                    SQLite local, avec sa durée ; actif par défaut avec go run / wails dev
  -h, --help        affiche cette aide, sans ouvrir la fenêtre

Presets de --demo :
`)
	for _, p := range Presets() {
		fmt.Fprintf(&b, "  %-9s %s\n", p.Name, p.Description)
	}
	b.WriteString(`
Variables d'environnement :
  LATTIS_BENCH=fichier    mesure le défilement de la grille et écrit le rapport dans fichier
  LATTIS_SQL=1 ou 0       force le journal SQL (comme --sql / --no-sql)
`)
	return b.String()
}
