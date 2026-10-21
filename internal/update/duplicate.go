package update

import (
	"bytes"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/lalamefine/lattis/internal/msg"
)

// Deux installations de Lattis sur le poste (pour l'utilisateur et pour
// tous les utilisateurs : deux entrées « Lattis » dans Applications
// installées). L'installeur n'en laisse plus deux (build/windows/nsis,
// lattis_scope.nsh), mais celles d'avant restent : au lancement, la barre
// d'état propose de retirer l'autre, par son désinstalleur.

// uninstallKey : clé de désinstallation écrite par les installeurs NSIS
// (build/windows/nsis/wails_tools.nsh, UNINST_KEY), sous HKCU pour une
// installation pour l'utilisateur, sous HKLM pour tous les utilisateurs.
const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\LattisLattis`

// Entry : une installation inscrite au Registre (clé de désinstallation).
type Entry struct {
	Dir         string // dossier (InstallLocation, sinon celui de UninstallString)
	Version     string // DisplayVersion
	Uninstaller string // désinstalleur (UninstallString, sinon Dir\uninstall.exe)
}

// entryFor : l'installation d'une clé de désinstallation, d'après ses
// valeurs InstallLocation, UninstallString et DisplayVersion.
func entryFor(location, uninstall, version string) Entry {
	e := Entry{Dir: installDir(location, uninstall), Version: strings.TrimSpace(version)}
	if e.Dir == "" {
		return Entry{}
	}
	e.Uninstaller = uninstallerPath(uninstall)
	if e.Uninstaller == "" {
		e.Uninstaller = strings.TrimRight(e.Dir, `\/`) + `\uninstall.exe`
	}
	return e
}

// uninstallerPath : l'exécutable d'une valeur UninstallString
// (« "C:\…\uninstall.exe" /x » ou sans guillemets).
func uninstallerPath(uninstall string) string {
	u := strings.TrimSpace(uninstall)
	if strings.HasPrefix(u, `"`) {
		if i := strings.Index(u[1:], `"`); i >= 0 {
			return u[1 : i+1]
		}
		return ""
	}
	if i := strings.Index(strings.ToLower(u), ".exe"); i >= 0 {
		return u[:i+4]
	}
	return ""
}

// Duplicate : l'autre installation de Lattis, à retirer (barre d'état).
type Duplicate struct {
	// Scope : sa portée, KindUser ou KindMachine.
	Scope   Kind   `json:"scope"`
	Dir     string `json:"dir"`
	Version string `json:"version,omitempty"`
	// Elevation : son retrait passe par l'invite UAC (pour tous).
	Elevation bool `json:"elevation,omitempty"`
	// Removing : retrait en cours ; Error : son échec, dans la langue de
	// la session.
	Removing bool   `json:"removing,omitempty"`
	Error    string `json:"error,omitempty"`

	uninstaller string
	err         error
}

// findDuplicate : l'autre installation quand les deux portées en ont une
// (dans deux dossiers) : celle qui n'est pas le dossier de exe ; exe dans
// aucune des deux (version portable, reste d'installation) : la plus
// ancienne, celle pour tous à version égale. nil : une seule installation.
func findDuplicate(exe string, user, machine Entry) *Duplicate {
	if user.Dir == "" || machine.Dir == "" || sameDir(user.Dir, machine.Dir) {
		return nil
	}
	dir := parentDir(exe)
	other, scope := machine, KindMachine
	switch {
	case sameDir(dir, user.Dir):
	case sameDir(dir, machine.Dir):
		other, scope = user, KindUser
	default:
		uv, uok := ParseVersion(user.Version)
		mv, mok := ParseVersion(machine.Version)
		if uok && mok && Newer(mv, uv) {
			other, scope = user, KindUser
		}
	}
	return &Duplicate{
		Scope:       scope,
		Dir:         other.Dir,
		Version:     other.Version,
		Elevation:   scope == KindMachine,
		uninstaller: other.Uninstaller,
	}
}

// keepDataSince : première version dont le désinstalleur connaît
// /KEEPDATA. Ceux d'avant effacent sans condition les données WebView2 de
// l'utilisateur (%APPDATA%\lattis.exe : historique SQL, préférences de
// l'interface), communes à toutes ses installations, donc à celle-ci.
const keepDataSince = "0.3.6-beta"

// removal : comment retirer l'autre installation.
type removal struct {
	// File, Args : la commande lancée (son désinstalleur silencieux, ou
	// reg.exe pour une clé pour tous orpheline) ; Elevate : par l'invite
	// UAC (verbe runas).
	File, Args string
	Elevate    bool
	// DeleteUserKey : clé HKCU orpheline (pas de désinstalleur), supprimée
	// directement.
	DeleteUserKey bool
	// OwnUser : installation pour l'utilisateur dont le désinstalleur
	// effacerait les données WebView2 de celle-ci : retirée par
	// l'application elle-même (fichiers connus, clé, raccourcis, classe
	// de fichiers).
	OwnUser bool
}

// removalFor décide du retrait de d ; exists dit si un fichier existe,
// sysDir est le dossier System32.
func removalFor(d Duplicate, exists func(string) bool, sysDir string) removal {
	if d.uninstaller == "" || !exists(d.uninstaller) {
		if d.Scope == KindUser {
			return removal{DeleteUserKey: true}
		}
		return removal{
			File:    strings.TrimRight(sysDir, `\`) + `\reg.exe`,
			Args:    `delete "HKLM\` + uninstallKey + `" /f /reg:64`,
			Elevate: true,
		}
	}
	if d.Scope == KindUser {
		v, ok := ParseVersion(d.Version)
		since, _ := ParseVersion(keepDataSince)
		if !ok || Newer(since, v) {
			return removal{OwnUser: true}
		}
		return removal{File: d.uninstaller, Args: "/S /KEEPDATA"}
	}
	return removal{File: d.uninstaller, Args: "/S /KEEPDATA", Elevate: true}
}

// ownUserFiles : fichiers d'une installation pour l'utilisateur retirés
// par l'application (removal.OwnUser) : ceux que l'installeur y met, et les
// restes d'une mise à jour ; jamais autre chose.
func ownUserFiles(dir string) []string {
	dir = strings.TrimRight(dir, `\/`)
	return []string{
		dir + `\lattis.exe`,
		dir + `\lattis.exe.old`,
		dir + `\lattis.exe.new`,
		dir + `\uninstall.exe`,
	}
}

// lnkMentions : le raccourci (contenu d'un .lnk) cite le dossier dir, en
// ANSI ou en UTF-16 (cible, dossier de travail), casse ignorée.
func lnkMentions(lnk []byte, dir string) bool {
	dir = strings.ToLower(strings.TrimRight(dir, `\/`)) + `\`
	if dir == `\` {
		return false
	}
	low := bytes.ToLower(lnk)
	if bytes.Contains(low, []byte(dir)) {
		return true
	}
	u := utf16.Encode([]rune(dir))
	b := make([]byte, 0, 2*len(u))
	for _, c := range u {
		b = append(b, byte(c), byte(c>>8))
	}
	// Minuscules ASCII seulement dans le raccourci : assez pour les
	// chemins Windows (lettres accentuées comparées telles quelles).
	return bytes.Contains(lowerUTF16(lnk), b)
}

func lowerUTF16(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	for i := 0; i+1 < len(out); i += 2 {
		if out[i+1] == 0 && out[i] >= 'A' && out[i] <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return out
}

// Retrait de l'autre installation : attente de sa disparition du Registre
// (son désinstalleur se recopie et rend la main tout de suite).
var (
	dupPoll    = time.Second
	dupTimeout = 5 * time.Minute
)

// detectDuplicate cherche une seconde installation (Init).
func (u *Updater) detectDuplicate() {
	if u.Installs == nil || u.Kind == KindMSIX {
		return
	}
	if _, ok := ParseVersion(u.Current); !ok {
		return // compilation de développement
	}
	user, machine := u.Installs()
	u.dup = findDuplicate(u.Exe, user, machine)
}

// RemoveDuplicate : « Retirer l'ancienne » : lance le retrait de l'autre
// installation (invite UAC si elle est pour tous), puis attend qu'elle ait
// quitté le Registre ; l'état suit par OnChange.
func (u *Updater) RemoveDuplicate() error {
	u.mu.Lock()
	d := u.dup
	if d == nil || d.Removing {
		u.mu.Unlock()
		return nil
	}
	d.Removing, d.err = true, nil
	work := *d
	u.mu.Unlock()
	u.changed()

	var err error
	if u.Remove == nil {
		err = msg.New("update.unsupported", nil)
	} else {
		err = u.Remove(work)
	}
	if err != nil {
		u.duplicateDone(err)
		return err
	}
	go u.waitRemoved(work)
	return nil
}

// waitRemoved attend que l'installation d ne soit plus inscrite.
func (u *Updater) waitRemoved(d Duplicate) {
	deadline := time.Now().Add(dupTimeout)
	for {
		user, machine := u.Installs()
		e := machine
		if d.Scope == KindUser {
			e = user
		}
		if !sameDir(e.Dir, d.Dir) {
			u.duplicateDone(nil)
			return
		}
		if time.Now().After(deadline) {
			u.duplicateDone(msg.New("update.duplicate.notRemoved", nil))
			return
		}
		time.Sleep(dupPoll)
	}
}

// duplicateDone : fin du retrait (err nil : réussi), l'état est relu.
func (u *Updater) duplicateDone(err error) {
	u.mu.Lock()
	if err == nil {
		u.detectDuplicate()
	} else if u.dup != nil {
		u.dup.Removing, u.dup.err = false, err
	}
	after := u.AfterRemove
	u.mu.Unlock()
	if err == nil && after != nil {
		after()
	}
	u.changed()
}

// duplicateStatus : l'autre installation, pour Status (nil : aucune).
func (u *Updater) duplicateStatus(l msg.Lang) *Duplicate {
	if u.dup == nil {
		return nil
	}
	d := *u.dup
	if d.err != nil {
		d.Error = msg.Text(l, d.err)
	}
	return &d
}
