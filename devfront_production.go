//go:build production

package main

// devFrontend : l'option --dev n'existe pas dans la version livrée.
func devFrontend(args []string) ([]string, func()) { return args, func() {} }
