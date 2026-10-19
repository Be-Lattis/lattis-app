//go:build production

package main

import "io/fs"

// devFrontend : l'option --dev n'existe pas dans la version livrée.
func devFrontend(args []string) ([]string, fs.FS) { return args, nil }
