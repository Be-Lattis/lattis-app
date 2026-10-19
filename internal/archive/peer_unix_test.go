//go:build !windows

package archive

// hidden : rien n'est caché hors de Windows (voir hide).
func hidden(string) bool { return true }
