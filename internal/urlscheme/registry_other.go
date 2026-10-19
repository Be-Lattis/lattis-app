//go:build !windows

package urlscheme

// Ensure : hors Windows, le schéma est déclaré par le .desktop (Linux) ou
// l'Info.plist (macOS) du paquet ; rien à faire au lancement.
func Ensure(dev, disabled bool) (Decision, error) {
	return Decision{Action: Skip, Reason: "hors Windows"}, nil
}
