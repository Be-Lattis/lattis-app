package update

import (
	"errors"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/lalamefine/lattis/internal/msg"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// uninstallKey : clé de désinstallation écrite par les installeurs NSIS
// (build/windows/nsis/wails_tools.nsh, UNINST_KEY), sous HKCU pour une
// installation pour l'utilisateur, sous HKLM pour tous les utilisateurs.
const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\LattisLattis`

// DetectKind : genre d'installation de l'exécutable exe.
func DetectKind(exe string) Kind {
	return kindFor(exe, Installs{User: readInstall(registry.CURRENT_USER), Machine: readInstall(registry.LOCAL_MACHINE)}, packaged())
}

func readInstall(root registry.Key) string {
	k, err := registry.OpenKey(root, uninstallKey, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return ""
	}
	defer k.Close()
	loc, _, _ := k.GetStringValue("InstallLocation")
	un, _, _ := k.GetStringValue("UninstallString")
	return installDir(loc, un)
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
