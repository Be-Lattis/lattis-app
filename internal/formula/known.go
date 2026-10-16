package formula

// Known indique si une fonction (nom français ou anglais) fait partie du
// langage. L'import d'un classeur Excel s'en sert pour garder les valeurs
// d'une formule plutôt que d'afficher #NOM? partout.
func Known(name string) bool {
	_, ok := lookupFunc(name)
	return ok
}
