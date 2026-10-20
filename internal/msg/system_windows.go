package msg

import "golang.org/x/sys/windows"

// systemLocale : première langue d'affichage préférée de l'utilisateur.
func systemLocale() string {
	l, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(l) == 0 {
		return ""
	}
	return l[0]
}
