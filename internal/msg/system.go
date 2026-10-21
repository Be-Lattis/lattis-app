package msg

import (
	"os"
	"strings"
)

// SystemLocale : code de langue de l'utilisateur du système (« fr-FR »),
// "" s'il est inconnu. Windows : langue d'affichage préférée ; ailleurs,
// les variables LC_ALL, LC_MESSAGES puis LANG (« C » et « POSIX » ne
// nomment aucune langue) ; LATTIS_SYSTEM_LOCALE avant tout.
func SystemLocale() string {
	// LATTIS_SYSTEM_LOCALE l'emporte : essais (rédigés en français) sur une
	// machine d'intégration en anglais, poste de démonstration.
	if v := strings.TrimSpace(os.Getenv("LATTIS_SYSTEM_LOCALE")); v != "" {
		return v
	}
	if s := systemLocale(); s != "" {
		return s
	}
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" && v != "C" && v != "POSIX" && !strings.HasPrefix(v, "C.") {
			return v
		}
	}
	return ""
}
