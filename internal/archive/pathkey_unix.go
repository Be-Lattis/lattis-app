//go:build !windows

package archive

import "path/filepath"

// canonical donne le chemin d'un fichier ou d'un dossier existant, liens
// symboliques résolus.
func canonical(path string) (string, error) { return filepath.EvalSymlinks(path) }
