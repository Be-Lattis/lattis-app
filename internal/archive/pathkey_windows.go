//go:build windows

package archive

import (
	"strings"

	"golang.org/x/sys/windows"
)

// volumeNameDOS : VOLUME_NAME_DOS | FILE_NAME_NORMALIZED (chemin « C:\… »).
const volumeNameDOS = 0

// canonical donne le chemin final d'un fichier ou d'un dossier existant :
// liens et jonctions résolus, lecteur réseau ramené à \\serveur\partage.
func canonical(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	h, err := windows.CreateFile(p, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 512)
	for {
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), volumeNameDOS)
		if err != nil {
			return "", err
		}
		if int(n) < len(buf) {
			s := windows.UTF16ToString(buf[:n])
			switch {
			case strings.HasPrefix(s, `\\?\UNC\`):
				s = `\\` + s[len(`\\?\UNC\`):]
			case strings.HasPrefix(s, `\\?\`):
				s = s[len(`\\?\`):]
			}
			return s, nil
		}
		buf = make([]uint16, n+1)
	}
}
