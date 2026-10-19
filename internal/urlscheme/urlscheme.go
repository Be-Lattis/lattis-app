// Package urlscheme déclare le schéma lattis:// pour l'utilisateur sous
// Windows quand personne ne l'a fait : l'installeur NSIS le pose
// (build/windows/nsis/wails_tools.nsh), le paquet MSIX le déclare dans son
// manifeste, mais une version lancée depuis les sources (go run .) ou copiée
// à la main n'a rien : « Ouvrir dans Lattis » sur la page web de Lattis
// Server ne fait alors rien.
//
// Au lancement, Ensure lit HKCU\Software\Classes\lattis\shell\open\command
// et décide (Decide, sans effet de bord) :
//
//   - absente, vide ou illisible : déclarée vers cet exécutable ;
//   - vers un exécutable qui n'existe plus : déclarée vers celui-ci ;
//   - vers un exécutable présent : laissée (la version installée l'emporte),
//     sauf dans une version de développement (sans l'étiquette production),
//     qui la reprend pour elle ;
//   - jamais sous le paquet MSIX, qui déclare son propre schéma.
//
// Une version de développement se déclare elle-même, pas un binaire bâti à
// côté (bin/lattis.exe) : instance.Forward ne transmet un lien qu'à
// l'instance du même exécutable, et c'est l'instance de go run qu'on veut
// joindre. go run bâtit dans un dossier temporaire effacé à la sortie : le
// lien ne marche que tant que cette instance tourne, et le lancement suivant
// (développement ou version installée) corrige la clé.
package urlscheme

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Action : ce que Ensure fait de la clé.
type Action int

const (
	Keep     Action = iota // la clé reste telle quelle
	Register               // la clé est (re)posée vers Decision.Target
	Skip                   // rien n'est lu ni écrit (paquet MSIX, option)
)

func (a Action) String() string {
	switch a {
	case Register:
		return "déclaré"
	case Skip:
		return "ignoré"
	}
	return "inchangé"
}

// Inputs : ce que Decide sait de la situation.
type Inputs struct {
	Command  string // valeur par défaut de shell\open\command ("" : absente)
	Exe      string // cet exécutable (os.Executable)
	Dev      bool   // version de développement (sans l'étiquette production)
	Packaged bool   // lancé depuis le paquet MSIX
	Disabled bool   // --no-register-protocol
}

// Decision : l'action, l'exécutable visé et la raison, pour le journal.
type Decision struct {
	Action Action
	Target string
	Reason string
}

// Decide choisit l'action ; exists dit si un fichier existe.
func Decide(in Inputs, exists func(string) bool) Decision {
	switch {
	case in.Disabled:
		return Decision{Skip, "", "désactivé par --no-register-protocol"}
	case in.Packaged:
		return Decision{Skip, "", "paquet MSIX : le schéma est déclaré par le manifeste"}
	case in.Exe == "":
		return Decision{Skip, "", "exécutable inconnu"}
	}
	cur := CommandExe(in.Command)
	switch {
	case cur == "":
		return Decision{Register, in.Exe, "schéma lattis:// absent"}
	case samePath(cur, in.Exe):
		if !strings.Contains(in.Command, "%1") {
			return Decision{Register, in.Exe, "commande sans %1 pour cet exécutable"}
		}
		return Decision{Keep, cur, "déjà déclaré pour cet exécutable"}
	case !exists(cur):
		return Decision{Register, in.Exe, "l'exécutable déclaré n'existe plus : " + cur}
	case in.Dev:
		return Decision{Register, in.Exe, "version de développement : remplace " + cur}
	}
	return Decision{Keep, cur, "déclaré pour un autre exécutable présent : " + cur}
}

// Command : la commande posée pour exe, comme l'installeur NSIS.
func Command(exe string) string { return `"` + exe + `" "%1"` }

// CommandExe extrait l'exécutable d'une commande (« "C:\…\lattis.exe" "%1" »
// ou sans guillemets « C:\…\lattis.exe %1 »).
func CommandExe(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	if cmd[0] == '"' {
		end := strings.IndexByte(cmd[1:], '"')
		if end < 0 {
			return strings.TrimSpace(cmd[1:])
		}
		return cmd[1 : 1+end]
	}
	// Sans guillemets : jusqu'à « .exe », le chemin peut contenir des espaces.
	if i := strings.Index(strings.ToLower(cmd), ".exe"); i >= 0 {
		return cmd[:i+4]
	}
	if i := strings.IndexByte(cmd, ' '); i >= 0 {
		return cmd[:i]
	}
	return cmd
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// Registry : la clé HKCU\Software\Classes\lattis (registry_windows.go).
type Registry interface {
	// Command lit shell\open\command ("" et nil si la clé manque).
	Command() (string, error)
	// Register pose la clé pour exe : « URL:Lattis », « URL Protocol »,
	// DefaultIcon et shell\open\command.
	Register(exe string) error
}

// Apply lit la clé, décide et écrit si besoin. in.Command est rempli ici.
func Apply(r Registry, in Inputs, exists func(string) bool) (Decision, error) {
	if !in.Disabled && !in.Packaged {
		cmd, err := r.Command()
		if err != nil {
			// Illisible : on la repose, comme absente.
			cmd = ""
		}
		in.Command = cmd
	}
	d := Decide(in, exists)
	if d.Action == Register {
		if err := r.Register(d.Target); err != nil {
			return d, fmt.Errorf("déclaration du schéma lattis:// : %w", err)
		}
	}
	return d, nil
}

// StripFlag retire --no-register-protocol des arguments et dit s'il y était.
func StripFlag(args []string) ([]string, bool) {
	out := args[:0:0]
	found := false
	for _, a := range args {
		if a == "--no-register-protocol" || a == "-no-register-protocol" {
			found = true
			continue
		}
		out = append(out, a)
	}
	return out, found
}
