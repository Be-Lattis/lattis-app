package urlscheme

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const keyPath = `Software\Classes\lattis`

// hkcu : la clé du schéma pour l'utilisateur courant.
type hkcu struct{}

func (hkcu) Command() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, keyPath+`\shell\open\command`, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer k.Close()
	s, _, err := k.GetStringValue("")
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	return s, err
}

func (hkcu) Register(exe string) error {
	set := func(sub, name, value string) error {
		path := keyPath
		if sub != "" {
			path += `\` + sub
		}
		k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.SetStringValue(name, value)
	}
	return errors.Join(
		set("", "", "URL:Lattis"),
		set("", "URL Protocol", ""),
		set("DefaultIcon", "", exe+",0"),
		set(`shell\open\command`, "", Command(exe)),
	)
}

// packaged dit si le processus tourne sous une identité de paquet (MSIX) :
// GetCurrentPackageFullName rend APPMODEL_ERROR_NO_PACKAGE (15700) sinon.
func packaged() bool {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentPackageFullName")
	if proc.Find() != nil {
		return false // avant Windows 8 : pas de paquets
	}
	var n uint32
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&n)), 0)
	const noPackage = 15700
	return r != noPackage
}

// Ensure applique la décision au Registre de l'utilisateur courant.
func Ensure(dev, disabled bool) (Decision, error) {
	exe, err := os.Executable()
	if err != nil {
		exe = ""
	}
	in := Inputs{Exe: exe, Dev: dev, Disabled: disabled}
	if !disabled {
		in.Packaged = packaged()
	}
	return Apply(hkcu{}, in, func(p string) bool {
		st, err := os.Stat(p)
		return err == nil && !st.IsDir()
	})
}
