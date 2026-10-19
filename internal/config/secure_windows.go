package config

import (
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// writeRights : droits qui permettent de modifier un fichier, ou d'ajouter
// un fichier dans un dossier.
const writeRights = windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.WRITE_DAC |
	windows.WRITE_OWNER | windows.GENERIC_WRITE | windows.GENERIC_ALL | windows.DELETE

// broadSids : groupes auxquels appartient tout utilisateur ordinaire.
var broadSids = []windows.WELL_KNOWN_SID_TYPE{
	windows.WinWorldSid,
	windows.WinBuiltinUsersSid,
	windows.WinAuthenticatedUserSid,
	windows.WinInteractiveSid,
}

// secureFile : aucun droit d'écriture accordé à Tout le monde, Utilisateurs,
// Utilisateurs authentifiés ou INTERACTIF, ni sur le fichier ni sur son
// dossier. Par défaut, %ProgramData% laisse les utilisateurs y créer des
// fichiers : le déploiement doit restreindre %ProgramData%\Lattis.
var secureFile = func(path string) (bool, error) {
	for _, p := range []string{path, filepath.Dir(path)} {
		ok, err := noBroadWrite(p)
		if err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}

func noBroadWrite(path string) (bool, error) {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false, err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return false, err
	}
	if dacl == nil {
		// DACL nulle : tout le monde a tous les droits.
		return false, nil
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return false, err
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask&writeRights == 0 {
			continue
		}
		// Une entrée seulement héritée par les enfants ne s'applique pas ici.
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		for _, t := range broadSids {
			if sid.IsWellKnown(t) {
				return false, nil
			}
		}
	}
	return true, nil
}
