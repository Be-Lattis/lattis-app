//go:build !windows

package msg

// systemLocale : rien de plus que les variables d'environnement. Une
// application lancée du Finder n'a souvent pas LANG : l'interface prend
// alors la langue de la page (navigator.language) et la transmet
// (DocumentService.SetLanguage).
func systemLocale() string { return "" }
