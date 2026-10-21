package update

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/lalamefine/lattis/internal/msg"
)

// InstallResultName : trace laissée par l'installeur Windows lancé pour une
// mise à jour (/UPDATE), dans le dossier des téléchargements
// (%LOCALAPPDATA%\Lattis\updates, build/windows/nsis/project.nsi,
// lattis.writeResult) : une ligne clé=valeur par champ, en UTF-16LE sans
// BOM (FileWriteUTF16LE) :
//
//	version=0.3.3
//	status=ok | failed | locked | elevation
//	code=<code de sortie>
//	dir=<dossier d'installation>
const InstallResultName = "install-result.txt"

// Statuts écrits par l'installeur ; resultNone : aucune trace.
const (
	resultOK        = "ok"
	resultFailed    = "failed"
	resultLocked    = "locked"
	resultElevation = "elevation"
	resultNone      = "none"
)

type installResult struct {
	Version, Status, Code, Dir string
}

// parseInstallResult lit le contenu du fichier de trace : UTF-16LE (avec
// ou sans BOM ; le premier caractère, « v », est ASCII) ou UTF-8.
func parseInstallResult(b []byte) installResult {
	var text string
	if len(b) >= 2 && (b[0] == 0xFF && b[1] == 0xFE || b[0] != 0 && b[1] == 0) {
		if b[0] == 0xFF && b[1] == 0xFE {
			b = b[2:]
		}
		u := make([]uint16, len(b)/2)
		for i := range u {
			u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
		}
		text = string(utf16.Decode(u))
	} else {
		text = string(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")))
	}
	var r installResult
	for _, line := range strings.Split(text, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "version":
			r.Version = v
		case "status":
			r.Status = v
		case "code":
			r.Code = v
		case "dir":
			r.Dir = v
		}
	}
	return r
}

// sameRelease : la version de l'installeur (« 0.3.3 », sans pré-version,
// INFO_PRODUCTVERSION) est celle de la mise à jour (« 0.3.3-beta »).
func sameRelease(installer, pending string) bool {
	a, _, _ := strings.Cut(strings.TrimPrefix(installer, "v"), "-")
	b, _, _ := strings.Cut(strings.TrimPrefix(pending, "v"), "-")
	return a != "" && a == b
}

// readInstallResult : la trace de l'installeur de la version version, lancé
// à after (une trace plus ancienne, ou d'une autre version, est ignorée).
func readInstallResult(path, version string, after time.Time) (installResult, bool) {
	if path == "" {
		return installResult{}, false
	}
	fi, err := os.Stat(path)
	if err != nil || fi.ModTime().Before(after.Add(-time.Minute)) {
		return installResult{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return installResult{}, false
	}
	r := parseInstallResult(b)
	if r.Status == "" || !sameRelease(r.Version, version) {
		return installResult{}, false
	}
	return r, true
}

func (u *Updater) resultPath() string {
	if u.Dir == "" {
		return ""
	}
	return filepath.Join(u.Dir, InstallResultName)
}

// judgeAttempts : au lancement, la mise à jour p est toujours en attente
// (version courante plus ancienne) : les essais lancés n'ont pas abouti.
// Le dernier prend la raison laissée par l'installeur.
func (u *Updater) judgeAttempts(p *Pending) {
	last := len(p.Attempts) - 1
	for i := range p.Attempts {
		a := &p.Attempts[i]
		if a.Outcome != OutcomeLaunched && a.Outcome != OutcomeManual {
			continue
		}
		a.Outcome = OutcomeFailed
		if p.Kind == KindPortable {
			continue
		}
		a.Result = resultNone
		if i != last {
			continue
		}
		if r, ok := readInstallResult(u.resultPath(), p.Version, a.At); ok {
			a.Result, a.Code, a.InstDir = r.Status, r.Code, r.Dir
		}
	}
}

// attemptReason : pourquoi l'essai a échoué, dans la langue l.
func attemptReason(l msg.Lang, a *Attempt) string {
	if a == nil {
		return ""
	}
	if a.Error != "" {
		return a.Error
	}
	var e error
	switch a.Result {
	case "":
		e = msg.New("update.install.notApplied", nil)
	case resultNone:
		e = msg.New("update.install.noTrace", nil)
	case resultOK:
		e = msg.New("update.install.otherDir", msg.Args{"dir": a.InstDir})
	case resultLocked:
		e = msg.New("update.install.locked", nil)
	case resultElevation:
		e = msg.New("update.install.elevation", nil)
	default:
		code := a.Code
		if code == "" {
			code = "?"
		}
		e = msg.New("update.install.failed", msg.Args{"code": code})
	}
	return msg.Text(l, e)
}
