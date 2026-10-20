package update

import "strings"

// Kind dit comment l'application est installée, donc comment elle se met
// à jour.
type Kind string

const (
	// KindUser : installeur par utilisateur (%LOCALAPPDATA%\Programs,
	// HKCU) : mise à jour silencieuse, sans droits d'administration.
	KindUser Kind = "user"
	// KindMachine : installeur par machine (Program Files, HKLM) : la mise
	// à jour demande les droits d'administration (invite UAC).
	KindMachine Kind = "machine"
	// KindPortable : l'exécutable seul : remplacé sur place, si son dossier
	// est inscriptible.
	KindPortable Kind = "portable"
	// KindMSIX : paquet MSIX (Store, App Installer) : mis à jour par
	// Windows, l'application ne cherche rien elle-même.
	KindMSIX Kind = "msix"
	// KindOther : macOS, Linux : la nouvelle version est signalée, la page
	// de la version s'ouvre pour la télécharger.
	KindOther Kind = "other"
)

// Fichiers Windows publiés avec chaque version (.github/workflows/release.yml,
// scripts/release-manifest.sh).
const (
	AssetUserSetup    = "Lattis-windows-x64-setup.exe"
	AssetMachineSetup = "Lattis-windows-x64-setup-machine.exe"
	AssetPortable     = "Lattis-windows-x64-portable.exe"
)

// AssetFor : le fichier de la version qui met à jour une installation de
// ce genre ; faux si l'application ne se met pas à jour elle-même.
func AssetFor(k Kind) (string, bool) {
	switch k {
	case KindUser:
		return AssetUserSetup, true
	case KindMachine:
		return AssetMachineSetup, true
	case KindPortable:
		return AssetPortable, true
	}
	return "", false
}

// Automatic dit si l'application sait installer elle-même la mise à jour.
func (k Kind) Automatic() bool {
	_, ok := AssetFor(k)
	return ok
}

// NeedsElevation : l'installation demandera les droits d'administration.
func (k Kind) NeedsElevation() bool { return k == KindMachine }

// Installs : dossiers d'installation lus dans le Registre (clé de
// désinstallation Lattis, HKCU et HKLM) ; vides s'il n'y en a pas.
type Installs struct {
	User, Machine string
}

// kindFor décide du genre d'installation de l'exécutable exe (Windows) :
// celle dont le dossier est le sien, sinon portable.
func kindFor(exe string, in Installs, packaged bool) Kind {
	if packaged {
		return KindMSIX
	}
	dir := parentDir(exe)
	if sameDir(dir, in.User) {
		return KindUser
	}
	if sameDir(dir, in.Machine) {
		return KindMachine
	}
	return KindPortable
}

// sameDir compare deux dossiers Windows (casse et barre finale ignorées).
func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	clean := func(s string) string {
		s = strings.Trim(strings.TrimSpace(s), `"`)
		s = strings.ReplaceAll(s, "/", `\`)
		return strings.ToLower(strings.TrimRight(s, `\`))
	}
	return clean(a) == clean(b)
}

// parentDir : dossier d'un chemin Windows (lu aussi sous un autre système,
// pour les tests).
func parentDir(p string) string {
	if i := strings.LastIndexAny(p, `\/`); i >= 0 {
		return p[:i]
	}
	return ""
}

// installDir : dossier d'installation d'une clé de désinstallation
// (InstallLocation, sinon le dossier de UninstallString, écrit par les
// installeurs plus anciens).
func installDir(location, uninstall string) string {
	if s := strings.Trim(strings.TrimSpace(location), `"`); s != "" {
		return s
	}
	u := strings.TrimSpace(uninstall)
	if strings.HasPrefix(u, `"`) {
		if i := strings.Index(u[1:], `"`); i >= 0 {
			u = u[1 : i+1]
		}
	} else if i := strings.Index(strings.ToLower(u), ".exe"); i >= 0 {
		u = u[:i+4]
	}
	if u == "" {
		return ""
	}
	u = strings.ReplaceAll(u, "/", `\`)
	if i := strings.LastIndex(u, `\`); i >= 0 {
		return u[:i]
	}
	return ""
}
