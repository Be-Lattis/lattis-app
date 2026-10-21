package update

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lalamefine/lattis/internal/msg"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// DetectKind : genre d'installation de l'exécutable exe.
func DetectKind(exe string) Kind {
	user, machine := systemInstalls()
	return kindFor(exe, Installs{User: user.Dir, Machine: machine.Dir}, packaged())
}

// systemInstalls : les installations inscrites, pour l'utilisateur (HKCU)
// et pour tous (HKLM).
func systemInstalls() (user, machine Entry) {
	return readEntry(registry.CURRENT_USER), readEntry(registry.LOCAL_MACHINE)
}

func readEntry(root registry.Key) Entry {
	k, err := registry.OpenKey(root, uninstallKey, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return Entry{}
	}
	defer k.Close()
	loc, _, _ := k.GetStringValue("InstallLocation")
	un, _, _ := k.GetStringValue("UninstallString")
	ver, _, _ := k.GetStringValue("DisplayVersion")
	return entryFor(loc, un, ver)
}

// packaged : le processus tourne sous une identité de paquet (MSIX).
func packaged() bool {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentPackageFullName")
	if proc.Find() != nil {
		return false
	}
	var n uint32
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&n)), 0)
	const noPackage = 15700 // APPMODEL_ERROR_NO_PACKAGE
	return r != noPackage
}

// VerifySignature vérifie la signature Authenticode d'un fichier
// (WinVerifyTrust, sans contrôle de révocation en ligne).
func VerifySignature(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	file := &windows.WinTrustFileInfo{Size: uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})), FilePath: p}
	data := &windows.WinTrustData{
		Size:                            uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:                        windows.WTD_UI_NONE,
		RevocationChecks:                windows.WTD_REVOKE_NONE,
		UnionChoice:                     windows.WTD_CHOICE_FILE,
		StateAction:                     windows.WTD_STATEACTION_VERIFY,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(file),
	}
	verr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	_ = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
	return verr
}

// SelfSigned : l'exécutable en cours porte une signature Authenticode
// valide ; ses mises à jour doivent alors être signées aussi.
func SelfSigned(exe string) bool { return exe != "" && VerifySignature(exe) == nil }

// apply installe la mise à jour p (voir Updater.ApplyOnExit).
func apply(p Pending, exe string, relaunch bool) error {
	switch p.Kind {
	case KindUser, KindMachine:
		return startInstaller(p, installerArgs(p.Kind, p.Dir, relaunch))
	case KindPortable:
		if err := replaceExe(exe, p.File); err != nil {
			return msg.New("update.replaceFailed", msg.Args{"err": err.Error()})
		}
		if relaunch {
			cmd := exec.Command(exe)
			cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS}
			if err := cmd.Start(); err != nil {
				return msg.New("update.launchFailed", msg.Args{"err": err.Error()})
			}
			return cmd.Process.Release()
		}
		return nil
	}
	return msg.New("update.unsupported", nil)
}

// launchInstaller : « Installer manuellement » : l'installeur de p avec son
// interface (voir Updater.InstallManually).
func launchInstaller(p Pending) error {
	if p.Kind != KindUser && p.Kind != KindMachine {
		return msg.New("update.unsupported", nil)
	}
	return startInstaller(p, manualArgs(p.Kind, p.Dir))
}

// startInstaller lance l'installeur p.File avec les paramètres args.
func startInstaller(p Pending, args string) error {
	if p.Kind == KindMachine {
		// Pour tous les utilisateurs (/AllUsers) : « runas » fait poser la
		// question de l'UAC ici (l'installeur s'élèverait sinon lui-même) ;
		// relancée par l'installeur (élevé), l'application l'est par
		// l'Explorateur, sans les droits d'administration.
		verb, _ := windows.UTF16PtrFromString("runas")
		file, _ := windows.UTF16PtrFromString(p.File)
		params, _ := windows.UTF16PtrFromString(args)
		err := windows.ShellExecute(0, verb, file, params, nil, windows.SW_SHOWNORMAL)
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return msg.New("update.elevationRefused", nil)
		}
		if err != nil {
			return msg.New("update.launchFailed", msg.Args{"err": err.Error()})
		}
		return nil
	}
	// Pour l'utilisateur (/CurrentUser) : sans élévation, l'installeur
	// attend la fin de l'application, s'installe et la relance (/RELAUNCH).
	// Processus détaché : il survit à la fin de Lattis (vérifié).
	cmd := exec.Command(p.File)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       syscall.EscapeArg(p.File) + " " + args,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	if err := cmd.Start(); err != nil {
		return msg.New("update.launchFailed", msg.Args{"err": err.Error()})
	}
	return cmd.Process.Release()
}

// removeOther lance le retrait de l'autre installation d (removalFor) ;
// l'attente de sa disparition du Registre est faite par l'Updater.
func removeOther(d Duplicate) error {
	sys, _ := windows.GetSystemDirectory()
	r := removalFor(d, fileExists, sys)
	failed := func(err error) error {
		return msg.New("update.duplicate.failed", msg.Args{"err": err.Error()})
	}
	switch {
	case r.DeleteUserKey:
		if err := registry.DeleteKey(registry.CURRENT_USER, uninstallKey); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return failed(err)
		}
		return nil
	case r.OwnUser:
		return removeUserInstall(d.Dir)
	case r.Elevate:
		// Pour tous : désinstalleur (ou reg.exe) avec l'invite UAC.
		verb, _ := windows.UTF16PtrFromString("runas")
		file, _ := windows.UTF16PtrFromString(r.File)
		params, _ := windows.UTF16PtrFromString(r.Args)
		err := windows.ShellExecute(0, verb, file, params, nil, windows.SW_HIDE)
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return msg.New("update.duplicate.refused", nil)
		}
		if err != nil {
			return failed(err)
		}
		return nil
	}
	cmd := exec.Command(r.File, strings.Fields(r.Args)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		return failed(err)
	}
	return cmd.Process.Release()
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// removeUserInstall retire une installation pour l'utilisateur sans son
// désinstalleur (removal.OwnUser) : ses fichiers connus et son dossier s'il
// est vide, ses raccourcis (menu Démarrer, Bureau) qui la désignent, sa
// classe de fichiers .lattis si elle l'ouvre, sa clé de désinstallation ;
// jamais les données de l'utilisateur. Les liens lattis:// sont repris
// ensuite par cette installation (Updater.AfterRemove).
func removeUserInstall(dir string) error {
	for _, f := range ownUserFiles(dir) {
		if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			// lattis.exe encore ouvert : rien d'autre n'est touché.
			return msg.New("update.duplicate.failed", msg.Args{"err": err.Error()})
		}
	}
	_ = os.Remove(dir) // vide seulement
	for _, id := range []*windows.KNOWNFOLDERID{windows.FOLDERID_Programs, windows.FOLDERID_Desktop} {
		folder, err := windows.KnownFolderPath(id, 0)
		if err != nil {
			continue
		}
		lnk := filepath.Join(folder, "Lattis.lnk")
		if b, err := os.ReadFile(lnk); err == nil && lnkMentions(b, dir) {
			_ = os.Remove(lnk)
		}
	}
	const class = `Software\Classes\Lattis.Classeur`
	if k, err := registry.OpenKey(registry.CURRENT_USER, class+`\shell\open\command`, registry.QUERY_VALUE); err == nil {
		cmd, _, _ := k.GetStringValue("")
		k.Close()
		if strings.Contains(strings.ToLower(cmd), strings.ToLower(strings.TrimRight(dir, `\`))+`\`) {
			deleteTree(registry.CURRENT_USER, class)
		}
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, uninstallKey); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return msg.New("update.duplicate.failed", msg.Args{"err": err.Error()})
	}
	return nil
}

var procRegDeleteTree = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegDeleteTreeW")

// deleteTree supprime une clé et ses sous-clés.
func deleteTree(root registry.Key, path string) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil || procRegDeleteTree.Find() != nil {
		return
	}
	_, _, _ = procRegDeleteTree.Call(uintptr(root), uintptr(unsafe.Pointer(p)))
}
