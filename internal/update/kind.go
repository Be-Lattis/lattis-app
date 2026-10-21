package update

import "strings"

// Kind dit comment l'application est installée, donc comment elle se met
// à jour.
type Kind string

const (
	// KindUser : installé pour l'utilisateur (%LOCALAPPDATA%\Programs,
	// HKCU) : mise à jour silencieuse, sans droits d'administration.
	KindUser Kind = "user"
	// KindMachine : installé pour tous les utilisateurs (Program Files,
	// HKLM) : la mise à jour demande les droits d'administration (invite UAC).
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
	// AssetSetup : l'installeur, pour l'utilisateur ou pour tous les
	// utilisateurs du poste (build/windows/nsis/lattis_scope.nsh).
	AssetSetup = "Lattis-windows-x64-setup.exe"
	// AssetLegacyMachineSetup : le même fichier sous le nom de l'ancien
	// installeur par machine. La 0.3.0-beta installée pour tous les
	// utilisateurs cherche sa mise à jour sous ce nom (dans les fichiers de
	// la release puis dans latest.json) et la lance sans /AllUsers : la
	// release le publie encore, et l'installeur prend alors la portée de
	// l'installation du dossier /D=. Plus utilisé par l'application.
	AssetLegacyMachineSetup = "Lattis-windows-x64-setup-machine.exe"
	AssetPortable           = "Lattis-windows-x64-portable.exe"
)

// AssetFor : le fichier de la version qui met à jour une installation de
// ce genre ; faux si l'application ne se met pas à jour elle-même.
func AssetFor(k Kind) (string, bool) {
	switch k {
	case KindUser, KindMachine:
		return AssetSetup, true
	case KindPortable:
		return AssetPortable, true
	}
	return "", false
}

// installerArgs : paramètres de l'installeur silencieux qui met à jour une
// installation de ce genre dans le dossier dir : la portée explicite
// (/CurrentUser : jamais d'élévation ; /AllUsers : lancé élevé), /UPDATE,
// /RELAUNCH pour rouvrir Lattis, et /D= en dernier, sans guillemets même
// avec des espaces (convention NSIS).
func installerArgs(k Kind, dir string, relaunch bool) string {
	args := "/S /CurrentUser"
	if k == KindMachine {
		args = "/S /AllUsers"
	}
	args += " /UPDATE"
	if relaunch {
		args += " /RELAUNCH"
	}
	if dir != "" {
		args += " /D=" + dir
	}
	return args
}

// manualArgs : paramètres de l'installeur lancé avec son interface
// (« Installer manuellement ») : ceux de la mise à jour, sans /S, avec
// /RELAUNCH.
func manualArgs(k Kind, dir string) string {
	return strings.TrimPrefix(installerArgs(k, dir, true), "/S ")
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
