//go:build !windows

package console

// Attach : rien à faire hors de Windows (la sortie standard est déjà
// celle du terminal).
func Attach() {}
